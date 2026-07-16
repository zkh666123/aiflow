const assert = require('node:assert/strict');
const { existsSync, readFileSync } = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const root = path.resolve(__dirname, '..', '..');
const migrationPath = path.join(
  root,
  'flowai-studio-backend',
  'alembic',
  'versions',
  '0002_control_schema.py',
);
const apiKeyScopeMigrationPath = path.join(
  root,
  'flowai-studio-backend',
  'alembic',
  'versions',
  '0007_api_key_application_cascade.py',
);

const expectedTables = [
  'users',
  'applications',
  'teams',
  'team_members',
  'team_applications',
  'api_keys',
  'app_shares',
];

test('defines the complete constrained identity and access schema in Alembic', () => {
  const migration = readFileSync(migrationPath, 'utf8');

  for (const table of expectedTables) {
    assert.match(
      migration,
      new RegExp(`CREATE TABLE IF NOT EXISTS control\\.${table}\\s*\\(`),
      `Alembic must create control.${table}`,
    );
  }

  assert.match(migration, /global_role IN \('admin', 'member'\)/);
  assert.match(migration, /role IN \('owner', 'admin', 'editor', 'viewer'\)/);
  assert.match(migration, /permission IN \('full_access', 'can_edit', 'can_view'\)/);
  assert.match(migration, /key_digest bytea NOT NULL UNIQUE/);
  assert.match(migration, /scopes jsonb NOT NULL/);
  assert.match(migration, /embed_config jsonb/);
  assert.match(migration, /def downgrade\(\) -> None:/);
  assert.match(migration, /DROP TABLE IF EXISTS control\.\{table\} CASCADE/);
});

test('revokes application-scoped API keys when the application is deleted', () => {
  assert.ok(existsSync(apiKeyScopeMigrationPath), 'API key cascade migration must exist');
  const migration = readFileSync(apiKeyScopeMigrationPath, 'utf8');

  assert.match(migration, /DROP CONSTRAINT IF EXISTS api_keys_application_id_fkey/);
  assert.match(migration, /FOREIGN KEY \(application_id\)/);
  assert.match(migration, /ON DELETE CASCADE/);
  assert.match(migration, /def downgrade\(\) -> None:/);
  assert.match(migration, /ON DELETE SET NULL/);
});

test('uses Python services for API key hashing and RBAC', () => {
  const apiKeys = readFileSync(
    path.join(root, 'flowai-studio-backend', 'src', 'aiflow_runtime', 'api', 'api_keys.py'),
    'utf8',
  );
  const auth = readFileSync(
    path.join(root, 'flowai-studio-backend', 'src', 'aiflow_runtime', 'identity', 'auth.py'),
    'utf8',
  );
  const rbac = readFileSync(
    path.join(root, 'flowai-studio-backend', 'src', 'aiflow_runtime', 'identity', 'rbac.py'),
    'utf8',
  );

  assert.match(apiKeys, /key_digest/);
  assert.match(apiKeys, /key_prefix/);
  assert.match(apiKeys, /hmac\.new\(secret, raw_key\.encode\(\), hashlib\.sha256\)/);
  assert.match(auth, /PasswordHash/);
  assert.match(rbac, /full_access/);
  assert.match(rbac, /can_edit/);
  assert.match(rbac, /can_view/);
});
