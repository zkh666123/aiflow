[CmdletBinding()]
param()
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Split-Path -Parent (Split-Path -Parent $PSScriptRoot)))
. (Join-Path $PSScriptRoot 'load-env.ps1')
& wsl.exe -- pg_isready -h 127.0.0.1 -p 5432|Out-Null;if($LASTEXITCODE-ne 0){throw 'PostgreSQL is unavailable.'}
if((& wsl.exe -- redis-cli -h 127.0.0.1 -p 6379 ping).Trim()-ne'PONG'){throw 'Redis is unavailable.'}
$python=Join-Path $root 'flowai-studio-backend\.venv\Scripts\python.exe'
$sandbox=(& $python (Join-Path $PSScriptRoot 'check-grpc.py') sandbox --address $env:FLOWAI_SANDBOX_GRPC_ADDR|ConvertFrom-Json)
$backend=Invoke-RestMethod -Uri 'http://127.0.0.1:3001/api/health' -TimeoutSec 15
if($sandbox.state-notin@('HEALTH_STATE_HEALTHY','HEALTH_STATE_NOT_READY')){throw "Sandbox health is $($sandbox.state)"}
if(-not$backend.success){throw 'Python backend health envelope reported failure.'}
[ordered]@{postgres='healthy';redis='healthy';sandbox=$sandbox.state;backend=$backend.data.status}|ConvertTo-Json
