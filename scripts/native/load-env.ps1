[CmdletBinding()]
param(
    [string]$Path
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
if ([string]::IsNullOrWhiteSpace($Path)) {
    $Path = Join-Path $root '.env.native'
}

if (-not (Test-Path -LiteralPath $Path)) {
    throw "$Path does not exist. Run scripts/native/initialize-database.ps1 first."
}

foreach ($line in Get-Content -Encoding utf8 -LiteralPath $Path) {
    if ([string]::IsNullOrWhiteSpace($line) -or $line.TrimStart().StartsWith('#')) {
        continue
    }
    if ($line -notmatch '^([^=]+)=(.*)$') {
        throw "Invalid environment line in $Path"
    }
    $name = $matches[1]
    $value = $matches[2]
    Set-Item -Path "Env:$name" -Value $value
}
