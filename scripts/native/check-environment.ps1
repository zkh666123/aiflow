[CmdletBinding()]
param()
$ErrorActionPreference='Stop'
function Check([string]$Name,[scriptblock]$Command,[string]$Pattern){$output=(& $Command 2>&1|Out-String).Trim();if($LASTEXITCODE-ne 0-or$output-notmatch$Pattern){throw "$Name check failed: $output"};[pscustomobject]@{Check=$Name;Value=$output}}
@(
 Check 'Python' { py -3.13 --version } '^Python 3\.13\.'
 Check 'uv' { uv --version } '^uv\s+'
 Check 'PostgreSQL' { wsl.exe -- pg_isready -h 127.0.0.1 -p 5432 } 'accepting connections'
 Check 'Redis' { wsl.exe -- redis-cli -h 127.0.0.1 -p 6379 ping } '^PONG$'
 Check 'pgvector' { wsl.exe -u postgres -- psql -X -Atc "SELECT extversion FROM pg_extension WHERE extname='vector';" } '^0\.'
)|Format-Table -AutoSize
