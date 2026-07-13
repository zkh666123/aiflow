[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$manifestPath = Join-Path $root 'toolchain\native-tools.json'
$manifest = Get-Content -Raw -Encoding utf8 $manifestPath | ConvertFrom-Json
$goPath = (& go env GOPATH 2>&1 | Out-String).Trim()

if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($goPath)) {
    throw 'Unable to resolve GOPATH.'
}

$binDirectory = Join-Path $goPath 'bin'
New-Item -ItemType Directory -Force -Path $binDirectory | Out-Null

$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$tempDirectory = Join-Path $tempRoot ("flowai-native-tools-{0}" -f [guid]::NewGuid().ToString('N'))
$tempDirectory = [IO.Path]::GetFullPath($tempDirectory)

if (-not $tempDirectory.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to use temporary directory outside $tempRoot"
}

New-Item -ItemType Directory -Path $tempDirectory | Out-Null

function Get-VerifiedArtifact {
    param(
        [Parameter(Mandatory)]
        [string]$Name,

        [Parameter(Mandatory)]
        [string]$Url,

        [Parameter(Mandatory)]
        [string]$Sha256,

        [Parameter(Mandatory)]
        [string]$FileName
    )

    $destination = Join-Path $tempDirectory $FileName
    Write-Host "Downloading $Name..."
    Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $destination

    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $destination).Hash.ToLowerInvariant()
    if ($actual -ne $Sha256.ToLowerInvariant()) {
        throw "$Name SHA-256 mismatch. Expected $Sha256, got $actual."
    }

    return $destination
}

try {
    $bufSource = Get-VerifiedArtifact 'Buf' $manifest.artifacts.buf.url $manifest.artifacts.buf.sha256 'buf.exe'
    Move-Item -Force -LiteralPath $bufSource -Destination (Join-Path $binDirectory 'buf.exe')

    $sqlcArchive = Get-VerifiedArtifact 'sqlc' $manifest.artifacts.sqlc.url $manifest.artifacts.sqlc.sha256 'sqlc.zip'
    $sqlcDirectory = Join-Path $tempDirectory 'sqlc'
    Expand-Archive -LiteralPath $sqlcArchive -DestinationPath $sqlcDirectory
    $sqlcSource = Get-ChildItem -LiteralPath $sqlcDirectory -Recurse -Filter 'sqlc.exe' | Select-Object -First 1
    if ($null -eq $sqlcSource) {
        throw 'The verified sqlc archive did not contain sqlc.exe.'
    }
    Move-Item -Force -LiteralPath $sqlcSource.FullName -Destination (Join-Path $binDirectory 'sqlc.exe')

    $gooseSource = Get-VerifiedArtifact 'goose' $manifest.artifacts.goose.url $manifest.artifacts.goose.sha256 'goose.exe'
    Move-Item -Force -LiteralPath $gooseSource -Destination (Join-Path $binDirectory 'goose.exe')

    Write-Host "Installed native tools in $binDirectory"
}
finally {
    if (Test-Path -LiteralPath $tempDirectory) {
        $resolvedTemp = [IO.Path]::GetFullPath($tempDirectory)
        if (-not $resolvedTemp.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing to remove temporary directory outside $tempRoot"
        }
        Remove-Item -LiteralPath $resolvedTemp -Recurse -Force
    }
}
