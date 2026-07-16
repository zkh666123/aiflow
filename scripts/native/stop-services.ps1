[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Split-Path -Parent (Split-Path -Parent $PSScriptRoot)))
$runtimeDirectory = [IO.Path]::GetFullPath((Join-Path $root '.runtime'))
$rootPrefix = $root.TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar

if (-not $runtimeDirectory.StartsWith($rootPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Runtime directory must stay inside the workspace.'
}
if (-not (Test-Path -LiteralPath $runtimeDirectory)) {
    return
}

foreach ($name in @('backend', 'control-plane', 'ai-runtime', 'sandbox')) {
    $pidPath = Join-Path $runtimeDirectory "$name.json"
    if (-not (Test-Path -LiteralPath $pidPath)) {
        continue
    }

    $record = Get-Content -Raw -Encoding utf8 -LiteralPath $pidPath | ConvertFrom-Json
    $expectedExecutable = [IO.Path]::GetFullPath([string]$record.executable)
    if (-not $expectedExecutable.StartsWith($rootPrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing to stop $name because its recorded executable is outside the workspace."
    }

    $process = Get-Process -Id ([int]$record.pid) -ErrorAction SilentlyContinue
    if ($null -ne $process) {
        $actualExecutable = [IO.Path]::GetFullPath($process.Path)
        if (-not $actualExecutable.Equals($expectedExecutable, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing to stop PID $($record.pid) because its executable identity changed."
        }
        $recordedStart = [DateTime]::Parse([string]$record.startedAt).ToUniversalTime()
        $actualStart = $process.StartTime.ToUniversalTime()
        if ([Math]::Abs(($actualStart - $recordedStart).TotalSeconds) -gt 2) {
            throw "Refusing to stop PID $($record.pid) because its start time identity changed."
        }
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit(5000) | Out-Null
    }

    Remove-Item -LiteralPath $pidPath -Force
}

$keepAlivePath = Join-Path $runtimeDirectory 'wsl-keepalive.json'
if (Test-Path -LiteralPath $keepAlivePath) {
    $record = Get-Content -Raw -Encoding utf8 -LiteralPath $keepAlivePath | ConvertFrom-Json
    $expectedExecutable = [IO.Path]::GetFullPath((Get-Command wsl.exe -ErrorAction Stop).Source)
    $recordedExecutable = [IO.Path]::GetFullPath([string]$record.executable)
    if (-not $recordedExecutable.Equals($expectedExecutable, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Refusing to stop WSL keepalive because its executable identity changed.'
    }

    $process = Get-Process -Id ([int]$record.pid) -ErrorAction SilentlyContinue
    if ($null -ne $process) {
        $actualExecutable = [IO.Path]::GetFullPath($process.Path)
        if (-not $actualExecutable.Equals($expectedExecutable, [StringComparison]::OrdinalIgnoreCase)) {
            throw 'Refusing to stop WSL keepalive because its process identity changed.'
        }
        $recordedStart = [DateTime]::Parse([string]$record.startedAt).ToUniversalTime()
        $actualStart = $process.StartTime.ToUniversalTime()
        if ([Math]::Abs(($actualStart - $recordedStart).TotalSeconds) -gt 2) {
            throw 'Refusing to stop WSL keepalive because its start time identity changed.'
        }
        Stop-Process -Id $process.Id -Force
        $process.WaitForExit(5000) | Out-Null
    }

    Remove-Item -LiteralPath $keepAlivePath -Force
}
