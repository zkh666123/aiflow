[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$manifestPath = Join-Path $root 'toolchain\native-tools.json'
$manifest = Get-Content -Raw -Encoding utf8 $manifestPath | ConvertFrom-Json

function Invoke-CheckedCommand {
    param(
        [Parameter(Mandatory)]
        [string]$Name,

        [Parameter(Mandatory)]
        [scriptblock]$Command,

        [Parameter(Mandatory)]
        [string]$ExpectedPattern
    )

    $output = (& $Command 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "$Name check failed with exit code $LASTEXITCODE`: $output"
    }
    if ($output -notmatch $ExpectedPattern) {
        throw "$Name check returned an unexpected value: $output"
    }

    [pscustomobject]@{
        Check = $Name
        Value = $output
    }
}

$checks = @(
    Invoke-CheckedCommand 'Go' { go version } "go$([regex]::Escape($manifest.runtimes.go))\."
    Invoke-CheckedCommand 'Python' { py -3.13 --version } "Python $([regex]::Escape($manifest.runtimes.python))\."
    Invoke-CheckedCommand 'uv' { uv --version } '^uv\s+\d+\.'
    Invoke-CheckedCommand 'Buf' { buf --version } "^v?$([regex]::Escape($manifest.tools.buf.TrimStart('v')))($|\s)"
    Invoke-CheckedCommand 'sqlc' { sqlc version } "$([regex]::Escape($manifest.tools.sqlc))"
    Invoke-CheckedCommand 'goose' { goose -version } "$([regex]::Escape($manifest.tools.goose))"
    Invoke-CheckedCommand 'PostgreSQL' { wsl.exe -- pg_isready -h 127.0.0.1 -p 5432 } 'accepting connections'
    Invoke-CheckedCommand 'Redis' { wsl.exe -- redis-cli -h 127.0.0.1 -p 6379 ping } '^PONG$'
    Invoke-CheckedCommand 'pgvector' { wsl.exe -u postgres -- psql -X -Atc "SELECT extversion FROM pg_extension WHERE extname='vector';" } "^$([regex]::Escape($manifest.runtimes.pgvector))($|\r?\n)"
)

$checks | Format-Table -AutoSize
