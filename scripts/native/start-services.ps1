[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Split-Path -Parent (Split-Path -Parent $PSScriptRoot)))
$runtimeDirectory = [IO.Path]::GetFullPath((Join-Path $root '.runtime'))
$rootPrefix = $root.TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar

if (-not $runtimeDirectory.StartsWith($rootPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Runtime directory must stay inside the workspace.'
}

New-Item -ItemType Directory -Force -Path $runtimeDirectory | Out-Null

function Assert-WorkspacePath {
    param([Parameter(Mandatory)][string]$Path)

    $resolved = [IO.Path]::GetFullPath((Resolve-Path -LiteralPath $Path).Path)
    $isWorkspaceRoot = $resolved.Equals($root, [StringComparison]::OrdinalIgnoreCase)
    if (-not $isWorkspaceRoot -and -not $resolved.StartsWith($rootPrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Path is outside the workspace: $resolved"
    }
    return $resolved
}

function Wait-TcpPort {
    param(
        [Parameter(Mandatory)][string]$Address,
        [int]$TimeoutSeconds = 30
    )

    $hostName, $portText = $Address.Split(':', 2)
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        $client = [Net.Sockets.TcpClient]::new()
        try {
            $attempt = $client.BeginConnect($hostName, [int]$portText, $null, $null)
            if ($attempt.AsyncWaitHandle.WaitOne(250) -and $client.Connected) {
                $client.EndConnect($attempt)
                return
            }
        }
        catch {
        }
        finally {
            $client.Dispose()
        }
        Start-Sleep -Milliseconds 150
    }
    throw "Timed out waiting for $Address"
}

function Start-WorkspaceProcess {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][string]$Executable,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory)][string]$WorkingDirectory
    )

    $pidPath = Join-Path $runtimeDirectory "$Name.json"
    if (Test-Path -LiteralPath $pidPath) {
        $existing = Get-Content -Raw -Encoding utf8 -LiteralPath $pidPath | ConvertFrom-Json
        $existingProcess = Get-Process -Id ([int]$existing.pid) -ErrorAction SilentlyContinue
        if ($null -ne $existingProcess) {
            throw "$Name is already running with PID $($existing.pid)"
        }
        Remove-Item -LiteralPath $pidPath -Force
    }

    $resolvedExecutable = Assert-WorkspacePath $Executable
    $resolvedWorkingDirectory = Assert-WorkspacePath $WorkingDirectory
    $stdoutPath = Join-Path $runtimeDirectory "$Name.stdout.log"
    $stderrPath = Join-Path $runtimeDirectory "$Name.stderr.log"
    if ($Arguments.Count -gt 0) {
        $process = Start-Process `
            -FilePath $resolvedExecutable `
            -ArgumentList $Arguments `
            -WorkingDirectory $resolvedWorkingDirectory `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdoutPath `
            -RedirectStandardError $stderrPath `
            -PassThru
    }
    else {
        $process = Start-Process `
            -FilePath $resolvedExecutable `
            -WorkingDirectory $resolvedWorkingDirectory `
            -WindowStyle Hidden `
            -RedirectStandardOutput $stdoutPath `
            -RedirectStandardError $stderrPath `
            -PassThru
    }

    [ordered]@{
        name = $Name
        pid = $process.Id
        executable = $resolvedExecutable
        workingDirectory = $resolvedWorkingDirectory
        arguments = $Arguments
        startedAt = $process.StartTime.ToUniversalTime().ToString('O')
    } | ConvertTo-Json | Set-Content -Encoding utf8 -LiteralPath $pidPath
}

function Start-WSLKeepAlive {
    $name = 'wsl-keepalive'
    $pidPath = Join-Path $runtimeDirectory "$name.json"
    if (Test-Path -LiteralPath $pidPath) {
        $existing = Get-Content -Raw -Encoding utf8 -LiteralPath $pidPath | ConvertFrom-Json
        $existingProcess = Get-Process -Id ([int]$existing.pid) -ErrorAction SilentlyContinue
        if ($null -ne $existingProcess) {
            return
        }
        Remove-Item -LiteralPath $pidPath -Force
    }

    $wslExecutable = [IO.Path]::GetFullPath((Get-Command wsl.exe -ErrorAction Stop).Source)
    $stdoutPath = Join-Path $runtimeDirectory "$name.stdout.log"
    $stderrPath = Join-Path $runtimeDirectory "$name.stderr.log"
    $process = Start-Process `
        -FilePath $wslExecutable `
        -ArgumentList @('-d', 'Ubuntu-24.04', '--', 'sleep', 'infinity') `
        -WindowStyle Hidden `
        -RedirectStandardOutput $stdoutPath `
        -RedirectStandardError $stderrPath `
        -PassThru

    [ordered]@{
        name = $name
        pid = $process.Id
        executable = $wslExecutable
        arguments = @('-d', 'Ubuntu-24.04', '--', 'sleep', 'infinity')
        startedAt = $process.StartTime.ToUniversalTime().ToString('O')
    } | ConvertTo-Json | Set-Content -Encoding utf8 -LiteralPath $pidPath
}

try {
    Start-WSLKeepAlive
    & (Join-Path $PSScriptRoot 'initialize-database.ps1')
    . (Join-Path $PSScriptRoot 'load-env.ps1')
    & (Join-Path $PSScriptRoot 'check-environment.ps1')

    & uv sync --project (Join-Path $root 'proto\python')
    & uv sync --project (Join-Path $root 'flowai-studio-sandbox')
    & uv sync --project (Join-Path $root 'flowai-studio-ai-runtime')

    $controlPlane = Join-Path $runtimeDirectory 'flowai-control-plane.exe'
    $previousGoMaxProcs = $env:GOMAXPROCS
    try {
        $env:GOMAXPROCS = '2'
        Push-Location (Join-Path $root 'flowai-studio-control-plane')
        & go build -p 1 -o $controlPlane ./cmd/api
        if ($LASTEXITCODE -ne 0) {
            throw 'Go control plane build failed.'
        }
    }
    finally {
        Pop-Location
        $env:GOMAXPROCS = $previousGoMaxProcs
    }

    Start-WorkspaceProcess `
        -Name 'sandbox' `
        -Executable (Join-Path $root 'flowai-studio-sandbox\.venv\Scripts\python.exe') `
        -Arguments @('-m', 'aiflow_sandbox.server') `
        -WorkingDirectory $root
    Wait-TcpPort $env:FLOWAI_SANDBOX_GRPC_ADDR

    Start-WorkspaceProcess `
        -Name 'ai-runtime' `
        -Executable (Join-Path $root 'flowai-studio-ai-runtime\.venv\Scripts\python.exe') `
        -Arguments @('-m', 'aiflow_runtime.server') `
        -WorkingDirectory $root
    Wait-TcpPort $env:FLOWAI_AI_GRPC_ADDR

    Start-WorkspaceProcess `
        -Name 'control-plane' `
        -Executable $controlPlane `
        -WorkingDirectory (Join-Path $root 'flowai-studio-control-plane')
    Wait-TcpPort $env:FLOWAI_HTTP_ADDR

    & (Join-Path $PSScriptRoot 'check-services.ps1')
}
catch {
    & (Join-Path $PSScriptRoot 'stop-services.ps1')
    throw
}
