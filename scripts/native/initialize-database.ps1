[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$envPath = Join-Path $root '.env.native'
$controlMigrations = Join-Path $root 'flowai-studio-control-plane\db\migrations'
$aiProject = Join-Path $root 'flowai-studio-ai-runtime'
$alembicConfig = Join-Path $aiProject 'alembic.ini'

function New-Secret {
    $bytes = [byte[]]::new(32)
    $generator = [Security.Cryptography.RandomNumberGenerator]::Create()
    try {
        $generator.GetBytes($bytes)
    }
    finally {
        $generator.Dispose()
    }
    return [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function Read-EnvFile {
    param([Parameter(Mandatory)][string]$Path)

    $values = [ordered]@{}
    foreach ($line in Get-Content -Encoding utf8 -LiteralPath $Path) {
        if ([string]::IsNullOrWhiteSpace($line) -or $line.TrimStart().StartsWith('#')) {
            continue
        }
        $name, $value = $line.Split('=', 2)
        $values[$name] = $value
    }
    return $values
}

function ConvertTo-SqlLiteral {
    param([Parameter(Mandatory)][string]$Value)
    return "'" + $Value.Replace("'", "''") + "'"
}

function Invoke-AdminPsql {
    param(
        [Parameter(Mandatory)][string]$Database,
        [Parameter(Mandatory)][string]$Sql
    )

    $previousEncoding = $OutputEncoding
    $previousErrorPreference = $ErrorActionPreference
    try {
        $OutputEncoding = [Text.UTF8Encoding]::new($false)
        $ErrorActionPreference = 'Continue'
        $output = ($Sql | & wsl.exe -u postgres -- psql -X -v ON_ERROR_STOP=1 -d $Database 2>&1 | Out-String).Trim()
        if ($LASTEXITCODE -ne 0) {
            throw "PostgreSQL bootstrap failed for database $Database`: $output"
        }
    }
    finally {
        $OutputEncoding = $previousEncoding
        $ErrorActionPreference = $previousErrorPreference
    }
}

if (-not (Test-Path -LiteralPath $envPath)) {
    $controlPassword = New-Secret
    $controlMigrationPassword = New-Secret
    $aiPassword = New-Secret
    $aiMigrationPassword = New-Secret
    $grpcToken = New-Secret

    $settings = [ordered]@{
        FLOWAI_HTTP_ADDR = '127.0.0.1:3001'
        FLOWAI_AI_GRPC_ADDR = '127.0.0.1:50051'
        FLOWAI_SANDBOX_GRPC_ADDR = '127.0.0.1:50052'
        FLOWAI_GRPC_TOKEN = $grpcToken
        FLOWAI_CONTROL_DATABASE_URL = "postgres://flowai_control:$([Uri]::EscapeDataString($controlPassword))@127.0.0.1:5432/flowai_studio?sslmode=disable&search_path=control"
        FLOWAI_CONTROL_MIGRATION_DATABASE_URL = "postgres://flowai_control_migrator:$([Uri]::EscapeDataString($controlMigrationPassword))@127.0.0.1:5432/flowai_studio?sslmode=disable&search_path=control"
        FLOWAI_AI_DATABASE_URL = "postgresql+psycopg://flowai_ai:$([Uri]::EscapeDataString($aiPassword))@127.0.0.1:5432/flowai_studio?sslmode=disable"
        FLOWAI_AI_MIGRATION_DATABASE_URL = "postgresql+psycopg://flowai_ai_migrator:$([Uri]::EscapeDataString($aiMigrationPassword))@127.0.0.1:5432/flowai_studio?sslmode=disable"
        FLOWAI_REDIS_URL = 'redis://127.0.0.1:6379/0'
    }

    $settings.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value)" } | Set-Content -Encoding utf8 -LiteralPath $envPath
    Write-Host "Created ignored local configuration at $envPath"
}

$settings = Read-EnvFile $envPath
$required = @(
    'FLOWAI_GRPC_TOKEN',
    'FLOWAI_CONTROL_DATABASE_URL',
    'FLOWAI_CONTROL_MIGRATION_DATABASE_URL',
    'FLOWAI_AI_DATABASE_URL',
    'FLOWAI_AI_MIGRATION_DATABASE_URL',
    'FLOWAI_REDIS_URL'
)
foreach ($name in $required) {
    if ([string]::IsNullOrWhiteSpace($settings[$name])) {
        throw "$name is missing from $envPath"
    }
}

function Get-PasswordFromUrl {
    param([Parameter(Mandatory)][string]$Url)
    $uri = [Uri]$Url
    $parts = $uri.UserInfo.Split(':', 2)
    return [Uri]::UnescapeDataString($parts[1])
}

$controlPasswordSql = ConvertTo-SqlLiteral (Get-PasswordFromUrl $settings.FLOWAI_CONTROL_DATABASE_URL)
$controlMigrationPasswordSql = ConvertTo-SqlLiteral (Get-PasswordFromUrl $settings.FLOWAI_CONTROL_MIGRATION_DATABASE_URL)
$aiPasswordSql = ConvertTo-SqlLiteral (Get-PasswordFromUrl $settings.FLOWAI_AI_DATABASE_URL)
$aiMigrationPasswordSql = ConvertTo-SqlLiteral (Get-PasswordFromUrl $settings.FLOWAI_AI_MIGRATION_DATABASE_URL)

$clusterSql = @'
DO $flowai$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'flowai_control_migrator') THEN
    CREATE ROLE flowai_control_migrator LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'flowai_control') THEN
    CREATE ROLE flowai_control LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'flowai_ai_migrator') THEN
    CREATE ROLE flowai_ai_migrator LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'flowai_ai') THEN
    CREATE ROLE flowai_ai LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
  END IF;
END
$flowai$;

ALTER ROLE flowai_control PASSWORD __CONTROL_PASSWORD__;
ALTER ROLE flowai_control_migrator PASSWORD __CONTROL_MIGRATION_PASSWORD__;
ALTER ROLE flowai_ai PASSWORD __AI_PASSWORD__;
ALTER ROLE flowai_ai_migrator PASSWORD __AI_MIGRATION_PASSWORD__;

SELECT 'CREATE DATABASE flowai_studio'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'flowai_studio')
\gexec

REVOKE ALL ON DATABASE flowai_studio FROM PUBLIC;
GRANT CONNECT ON DATABASE flowai_studio TO flowai_control, flowai_control_migrator, flowai_ai, flowai_ai_migrator;
'@
$clusterSql = $clusterSql.Replace('__CONTROL_PASSWORD__', $controlPasswordSql)
$clusterSql = $clusterSql.Replace('__CONTROL_MIGRATION_PASSWORD__', $controlMigrationPasswordSql)
$clusterSql = $clusterSql.Replace('__AI_PASSWORD__', $aiPasswordSql)
$clusterSql = $clusterSql.Replace('__AI_MIGRATION_PASSWORD__', $aiMigrationPasswordSql)
Invoke-AdminPsql 'postgres' $clusterSql

$databaseSql = @'
SET client_min_messages TO warning;
CREATE EXTENSION IF NOT EXISTS vector;

REVOKE CREATE ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO flowai_control, flowai_control_migrator, flowai_ai, flowai_ai_migrator;

CREATE SCHEMA IF NOT EXISTS control AUTHORIZATION flowai_control_migrator;
CREATE SCHEMA IF NOT EXISTS ai AUTHORIZATION flowai_ai_migrator;
ALTER SCHEMA control OWNER TO flowai_control_migrator;
ALTER SCHEMA ai OWNER TO flowai_ai_migrator;

REVOKE ALL ON SCHEMA control FROM PUBLIC, flowai_ai, flowai_ai_migrator;
REVOKE ALL ON SCHEMA ai FROM PUBLIC, flowai_control, flowai_control_migrator;
GRANT USAGE ON SCHEMA control TO flowai_control;
GRANT USAGE ON SCHEMA ai TO flowai_ai;

ALTER DEFAULT PRIVILEGES FOR ROLE flowai_control_migrator IN SCHEMA control
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO flowai_control;
ALTER DEFAULT PRIVILEGES FOR ROLE flowai_control_migrator IN SCHEMA control
  GRANT USAGE, SELECT ON SEQUENCES TO flowai_control;
ALTER DEFAULT PRIVILEGES FOR ROLE flowai_ai_migrator IN SCHEMA ai
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO flowai_ai;
ALTER DEFAULT PRIVILEGES FOR ROLE flowai_ai_migrator IN SCHEMA ai
  GRANT USAGE, SELECT ON SEQUENCES TO flowai_ai;

ALTER ROLE flowai_control IN DATABASE flowai_studio SET search_path TO control, public;
ALTER ROLE flowai_control_migrator IN DATABASE flowai_studio SET search_path TO control, public;
ALTER ROLE flowai_ai IN DATABASE flowai_studio SET search_path TO ai, public;
ALTER ROLE flowai_ai_migrator IN DATABASE flowai_studio SET search_path TO ai, public;
'@
Invoke-AdminPsql 'flowai_studio' $databaseSql

$previousGooseDriver = $env:GOOSE_DRIVER
$previousGooseDbString = $env:GOOSE_DBSTRING
$previousAiMigrationUrl = $env:FLOWAI_AI_MIGRATION_DATABASE_URL
try {
    $env:GOOSE_DRIVER = 'postgres'
    $env:GOOSE_DBSTRING = $settings.FLOWAI_CONTROL_MIGRATION_DATABASE_URL
    & goose -no-color -table control.goose_db_version -dir $controlMigrations up
    if ($LASTEXITCODE -ne 0) {
        throw 'Goose control migrations failed.'
    }

    $env:FLOWAI_AI_MIGRATION_DATABASE_URL = $settings.FLOWAI_AI_MIGRATION_DATABASE_URL
    & uv sync --project $aiProject
    if ($LASTEXITCODE -ne 0) {
        throw 'AI runtime migration dependencies failed to sync.'
    }
    & uv run --project $aiProject alembic -c $alembicConfig upgrade head
    if ($LASTEXITCODE -ne 0) {
        throw 'Alembic AI migrations failed.'
    }
}
finally {
    $env:GOOSE_DRIVER = $previousGooseDriver
    $env:GOOSE_DBSTRING = $previousGooseDbString
    $env:FLOWAI_AI_MIGRATION_DATABASE_URL = $previousAiMigrationUrl
}

Write-Host 'FlowAI Studio database roles, schemas, extension, and migrations are ready.'
