[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath((Split-Path -Parent (Split-Path -Parent $PSScriptRoot)))
. (Join-Path $PSScriptRoot 'load-env.ps1')

& wsl.exe -- pg_isready -h 127.0.0.1 -p 5432 | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL is unavailable.' }
if ((& wsl.exe -- redis-cli -h 127.0.0.1 -p 6379 ping).Trim() -ne 'PONG') {
    throw 'Redis is unavailable.'
}

$python = Join-Path $root 'proto\python\.venv\Scripts\python.exe'
$grpcCheck = Join-Path $PSScriptRoot 'check-grpc.py'
$ai = (& $python $grpcCheck ai --address $env:FLOWAI_AI_GRPC_ADDR --token $env:FLOWAI_GRPC_TOKEN | ConvertFrom-Json)
$sandbox = (& $python $grpcCheck sandbox --address $env:FLOWAI_SANDBOX_GRPC_ADDR --token $env:FLOWAI_GRPC_TOKEN | ConvertFrom-Json)
$go = Invoke-RestMethod -Uri 'http://127.0.0.1:3001/api/health' -TimeoutSec 15

if ($ai.state -ne 'HEALTH_STATE_HEALTHY') {
    throw "AI runtime health is $($ai.state)"
}
if ($sandbox.state -notin @('HEALTH_STATE_HEALTHY', 'HEALTH_STATE_NOT_READY')) {
    throw "Sandbox health is $($sandbox.state)"
}
if (-not $go.success) {
    throw 'Go control plane health envelope reported failure.'
}

[ordered]@{
    postgres = 'healthy'
    redis = 'healthy'
    aiRuntime = $ai.state
    sandbox = $sandbox.state
    controlPlane = $go.data.status
} | ConvertTo-Json
