[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$envPath = Join-Path $root '.env.native'
$backend = Join-Path $root 'flowai-studio-backend'

function New-Secret {
    $bytes = [byte[]]::new(32)
    $generator = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $generator.GetBytes($bytes) }
    finally { $generator.Dispose() }
    return [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}
function Read-EnvFile([string]$Path) {
    $values = [ordered]@{}
    if (Test-Path -LiteralPath $Path) {
        foreach ($line in Get-Content -Encoding utf8 -LiteralPath $Path) {
            if ($line -and -not $line.TrimStart().StartsWith('#')) { $name, $value = $line.Split('=', 2); $values[$name] = $value }
        }
    }
    return $values
}
function Ensure([System.Collections.IDictionary]$Values, [string]$Name, [scriptblock]$Factory) {
    if (-not $Values.Contains($Name) -or [string]::IsNullOrWhiteSpace([string]$Values[$Name])) { $Values[$Name] = & $Factory }
}
function Sql-Literal([string]$Value) { return "'" + $Value.Replace("'", "''") + "'" }
function Url-Password([string]$Url) { $parts = ([Uri]$Url).UserInfo.Split(':', 2); return [Uri]::UnescapeDataString($parts[1]) }
function Admin-Psql([string]$Database, [string]$Sql) {
    $old = $OutputEncoding; $oldPreference = $ErrorActionPreference
    try {
        $OutputEncoding = [Text.UTF8Encoding]::new($false); $ErrorActionPreference = 'Continue'
        $output = ($Sql | & wsl.exe -u postgres -- psql -X -v ON_ERROR_STOP=1 -d $Database 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -ne 0) { throw $output }
    }
    finally { $OutputEncoding = $old; $ErrorActionPreference = $oldPreference }
}

$settings = Read-EnvFile $envPath
$runtimePassword = New-Secret; $migrationPassword = New-Secret
Ensure $settings 'FLOWAI_HTTP_ADDR' { '127.0.0.1:3001' }
Ensure $settings 'FLOWAI_SANDBOX_GRPC_ADDR' { '127.0.0.1:50052' }
Ensure $settings 'FLOWAI_GRPC_TOKEN' { New-Secret }
Ensure $settings 'FLOWAI_JWT_SECRET' { New-Secret }
Ensure $settings 'FLOWAI_JWT_EXPIRATION' { '168h' }
Ensure $settings 'FLOWAI_API_KEY_HMAC_SECRET' { New-Secret }
if (-not $settings.Contains('FLOWAI_API_KEY_HMAC_PREVIOUS_SECRET')) { $settings['FLOWAI_API_KEY_HMAC_PREVIOUS_SECRET'] = '' }
Ensure $settings 'FLOWAI_FRONTEND_URL' { 'http://127.0.0.1:5173' }
Ensure $settings 'FLOWAI_REDIS_URL' { 'redis://127.0.0.1:6379/0' }
if (-not $settings.Contains('FLOWAI_DATABASE_URL')) {
    $settings['FLOWAI_DATABASE_URL'] = "postgresql+psycopg://flowai_ai:$([Uri]::EscapeDataString($runtimePassword))@127.0.0.1:5432/flowai_studio?sslmode=disable"
}
if (-not $settings.Contains('FLOWAI_MIGRATION_DATABASE_URL')) {
    $settings['FLOWAI_MIGRATION_DATABASE_URL'] = "postgresql+psycopg://flowai_ai_migrator:$([Uri]::EscapeDataString($migrationPassword))@127.0.0.1:5432/flowai_studio?sslmode=disable"
}
$settings.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value)" } | Set-Content -Encoding utf8 -LiteralPath $envPath

$runtimePasswordSql = Sql-Literal (Url-Password $settings.FLOWAI_DATABASE_URL)
$migrationPasswordSql = Sql-Literal (Url-Password $settings.FLOWAI_MIGRATION_DATABASE_URL)
$cluster = @'
DO $flowai$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='flowai_ai') THEN CREATE ROLE flowai_ai LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION; END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='flowai_ai_migrator') THEN CREATE ROLE flowai_ai_migrator LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION; END IF;
END $flowai$;
ALTER ROLE flowai_ai PASSWORD __RUNTIME_PASSWORD__;
ALTER ROLE flowai_ai_migrator PASSWORD __MIGRATION_PASSWORD__;
SELECT 'CREATE DATABASE flowai_studio' WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname='flowai_studio') \gexec
REVOKE ALL ON DATABASE flowai_studio FROM PUBLIC;
GRANT CONNECT ON DATABASE flowai_studio TO flowai_ai,flowai_ai_migrator;
GRANT CREATE ON DATABASE flowai_studio TO flowai_ai_migrator;
'@
Admin-Psql 'postgres' ($cluster.Replace('__RUNTIME_PASSWORD__',$runtimePasswordSql).Replace('__MIGRATION_PASSWORD__',$migrationPasswordSql))
Admin-Psql 'flowai_studio' @'
CREATE EXTENSION IF NOT EXISTS vector;
CREATE SCHEMA IF NOT EXISTS control AUTHORIZATION flowai_ai_migrator;
CREATE SCHEMA IF NOT EXISTS ai AUTHORIZATION flowai_ai_migrator;
ALTER SCHEMA control OWNER TO flowai_ai_migrator; ALTER SCHEMA ai OWNER TO flowai_ai_migrator;
GRANT USAGE ON SCHEMA control,ai,public TO flowai_ai;
GRANT ALL ON ALL TABLES IN SCHEMA control,ai TO flowai_ai;
GRANT ALL ON ALL SEQUENCES IN SCHEMA control,ai TO flowai_ai;
ALTER DEFAULT PRIVILEGES FOR ROLE flowai_ai_migrator IN SCHEMA control GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO flowai_ai;
ALTER DEFAULT PRIVILEGES FOR ROLE flowai_ai_migrator IN SCHEMA ai GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO flowai_ai;
ALTER ROLE flowai_ai IN DATABASE flowai_studio SET search_path TO control,ai,public;
ALTER ROLE flowai_ai_migrator IN DATABASE flowai_studio SET search_path TO control,ai,public;
'@
. (Join-Path $PSScriptRoot 'load-env.ps1')
& uv sync --project $backend
if ($LASTEXITCODE -ne 0) { throw 'Python backend dependency sync failed.' }
& uv run --project $backend alembic -c (Join-Path $backend 'alembic.ini') upgrade head
if ($LASTEXITCODE -ne 0) { throw 'Alembic migrations failed.' }
Write-Host 'FlowAI Studio PostgreSQL schemas and Alembic migrations are ready.'
