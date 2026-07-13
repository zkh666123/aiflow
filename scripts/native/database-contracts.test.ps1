[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$initializer = Join-Path $PSScriptRoot 'initialize-database.ps1'
$envPath = Join-Path $root '.env.native'

if (-not (Test-Path -LiteralPath $initializer)) {
    throw 'scripts/native/initialize-database.ps1 must exist'
}

function Read-EnvFile {
    param([Parameter(Mandatory)][string]$Path)

    $values = @{}
    foreach ($line in Get-Content -Encoding utf8 -LiteralPath $Path) {
        if ([string]::IsNullOrWhiteSpace($line) -or $line.TrimStart().StartsWith('#')) {
            continue
        }
        $name, $value = $line.Split('=', 2)
        $values[$name] = $value
    }
    return $values
}

function ConvertFrom-DatabaseUrl {
    param([Parameter(Mandatory)][string]$Url)

    $uri = [Uri]$Url
    $user, $password = $uri.UserInfo.Split(':', 2)
    return [pscustomobject]@{
        User = [Uri]::UnescapeDataString($user)
        Password = [Uri]::UnescapeDataString($password)
        Database = $uri.AbsolutePath.TrimStart('/')
    }
}

function Invoke-RoleSql {
    param(
        [Parameter(Mandatory)][string]$Url,
        [Parameter(Mandatory)][string]$Sql,
        [switch]$ExpectFailure
    )

    $connection = ConvertFrom-DatabaseUrl $Url
    $previousPassword = $env:PGPASSWORD
    $previousWslEnv = $env:WSLENV
    $previousErrorPreference = $ErrorActionPreference
    try {
        $env:PGPASSWORD = $connection.Password
        $env:WSLENV = if ([string]::IsNullOrWhiteSpace($previousWslEnv)) {
            'PGPASSWORD'
        } elseif ($previousWslEnv -match '(^|:)PGPASSWORD($|:)') {
            $previousWslEnv
        } else {
            "$previousWslEnv`:PGPASSWORD"
        }

        $ErrorActionPreference = 'Continue'
        $output = (& wsl.exe -- psql -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 5432 -U $connection.User -d $connection.Database -Atc $Sql 2>&1 | Out-String).Trim()
        $exitCode = $LASTEXITCODE
    }
    finally {
        $env:PGPASSWORD = $previousPassword
        $env:WSLENV = $previousWslEnv
        $ErrorActionPreference = $previousErrorPreference
    }

    if ($ExpectFailure) {
        if ($exitCode -eq 0) {
            throw "Expected SQL to fail for $($connection.User): $Sql"
        }
        return
    }

    if ($exitCode -ne 0) {
        throw "SQL failed for $($connection.User): $output"
    }
    return $output
}

& $initializer
& $initializer

if (-not (Test-Path -LiteralPath $envPath)) {
    throw '.env.native was not created'
}

$settings = Read-EnvFile $envPath
$required = @(
    'FLOWAI_CONTROL_DATABASE_URL',
    'FLOWAI_CONTROL_MIGRATION_DATABASE_URL',
    'FLOWAI_AI_DATABASE_URL',
    'FLOWAI_AI_MIGRATION_DATABASE_URL'
)
foreach ($name in $required) {
    if ([string]::IsNullOrWhiteSpace($settings[$name])) {
        throw "$name is missing from .env.native"
    }
}

$adminSql = @'
SELECT current_database();
SELECT extversion FROM pg_extension WHERE extname = 'vector';
SELECT string_agg(nspname, ',' ORDER BY nspname) FROM pg_namespace WHERE nspname IN ('ai', 'control');
SELECT count(*) FROM pg_roles
WHERE rolname IN ('flowai_control', 'flowai_control_migrator', 'flowai_ai', 'flowai_ai_migrator')
  AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication;
'@
$adminOutput = (& wsl.exe -u postgres -- psql -X -v ON_ERROR_STOP=1 -d flowai_studio -Atc $adminSql 2>&1 | Out-String).Trim()
if ($LASTEXITCODE -ne 0) {
    throw "Admin verification failed: $adminOutput"
}
$adminLines = $adminOutput -split "`r?`n"
if ($adminLines[0] -ne 'flowai_studio') { throw 'flowai_studio database is missing' }
if ([version]$adminLines[1] -lt [version]'0.8.5') { throw 'pgvector is older than 0.8.5' }
if ($adminLines[2] -ne 'ai,control') { throw 'control and ai schemas are missing' }
if ($adminLines[3] -ne '4') { throw 'runtime and migrator roles are not least-privilege roles' }

$controlUrl = $settings.FLOWAI_CONTROL_DATABASE_URL
$controlMigrationUrl = $settings.FLOWAI_CONTROL_MIGRATION_DATABASE_URL
$aiUrl = $settings.FLOWAI_AI_DATABASE_URL
$aiMigrationUrl = $settings.FLOWAI_AI_MIGRATION_DATABASE_URL

Invoke-RoleSql $controlUrl "BEGIN; INSERT INTO control.schema_metadata(key, value) VALUES ('contract-test', '1'); UPDATE control.schema_metadata SET value = '2' WHERE key = 'contract-test'; DELETE FROM control.schema_metadata WHERE key = 'contract-test'; ROLLBACK;" | Out-Null
Invoke-RoleSql $aiUrl "BEGIN; INSERT INTO ai.schema_metadata(key, value) VALUES ('contract-test', '1'); UPDATE ai.schema_metadata SET value = '2' WHERE key = 'contract-test'; DELETE FROM ai.schema_metadata WHERE key = 'contract-test'; ROLLBACK;" | Out-Null

Invoke-RoleSql $controlUrl 'CREATE TABLE control.runtime_must_not_create(id integer);' -ExpectFailure
Invoke-RoleSql $aiUrl 'CREATE TABLE ai.runtime_must_not_create(id integer);' -ExpectFailure
Invoke-RoleSql $controlUrl 'SELECT count(*) FROM ai.schema_metadata;' -ExpectFailure
Invoke-RoleSql $aiUrl 'SELECT count(*) FROM control.schema_metadata;' -ExpectFailure
Invoke-RoleSql $controlMigrationUrl 'CREATE TABLE ai.control_migrator_must_not_create(id integer);' -ExpectFailure
Invoke-RoleSql $aiMigrationUrl 'CREATE TABLE control.ai_migrator_must_not_create(id integer);' -ExpectFailure

$gooseTable = Invoke-RoleSql $controlMigrationUrl "SELECT n.nspname || '.' || c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'control' AND c.relname = 'goose_db_version';"
$alembicTable = Invoke-RoleSql $aiMigrationUrl "SELECT n.nspname || '.' || c.relname FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'ai' AND c.relname = 'alembic_version';"
if ($gooseTable -ne 'control.goose_db_version') { throw 'Goose version table is not in control schema' }
if ($alembicTable -ne 'ai.alembic_version') { throw 'Alembic version table is not in ai schema' }

Write-Host 'Database contracts passed: pgvector, idempotency, role isolation, DML grants, and migration ownership.'
