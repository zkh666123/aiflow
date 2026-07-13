const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const root = path.resolve(__dirname, '..', '..');
const migrationPath = path.join(
  root,
  'flowai-studio-control-plane',
  'db',
  'migrations',
  '00002_identity_access.sql',
);
const schemaPath = path.join(
  root,
  'flowai-studio-control-plane',
  'db',
  'schema',
  'control.sql',
);
const queryDirectory = path.join(
  root,
  'flowai-studio-control-plane',
  'db',
  'query',
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

test('defines the complete constrained identity and access schema', () => {
  const migration = readFileSync(migrationPath, 'utf8');
  const schema = readFileSync(schemaPath, 'utf8');

  for (const table of expectedTables) {
    const create = new RegExp(`CREATE TABLE control\\.${table}\\s*\\(`);
    assert.match(migration, create, `migration must create control.${table}`);
    assert.match(schema, create, `sqlc schema must declare control.${table}`);
  }

  assert.match(migration, /global_role IN \('admin', 'member'\)/);
  assert.match(migration, /role IN \('owner', 'admin', 'editor', 'viewer'\)/);
  assert.match(migration, /permission IN \('full_access', 'can_edit', 'can_view'\)/);
  assert.match(migration, /key_digest bytea NOT NULL UNIQUE/);
  assert.match(migration, /scopes jsonb NOT NULL/);
  assert.match(migration, /embed_config jsonb/);
  assert.match(
    migration,
    /-- \+goose StatementBegin\s+CREATE OR REPLACE FUNCTION[\s\S]+?-- \+goose StatementEnd/,
    'Goose must treat the PL/pgSQL trigger function as one statement',
  );
  assert.match(migration, /-- \+goose Down/);
  assert.match(migration, /DROP TABLE control\.users/);
});

test('defines typed queries for every identity and access aggregate', () => {
  const expectedQueries = {
    'users.sql': [
      'CreateUser',
      'GetUserByUsername',
      'GetUserByID',
      'UpdateUserProfile',
      'GetUserGlobalRole',
    ],
    'applications.sql': [
      'CreateApplication',
      'ListApplicationsForUser',
      'GetApplicationByID',
      'UpdateApplication',
      'DeleteApplication',
      'SetApplicationStatus',
      'ListApplicationAccessForUser',
    ],
    'teams.sql': [
      'CreateTeam',
      'CreateTeamMember',
      'ListTeamsForUser',
      'GetTeamByID',
      'GetTeamMembership',
      'CreateTeamApplication',
      'UpdateTeamApplicationPermission',
    ],
    'api_keys.sql': [
      'CreateAPIKey',
      'ListAPIKeys',
      'GetAPIKeyByID',
      'GetAPIKeyByDigest',
      'SetAPIKeyActive',
      'DeleteAPIKey',
    ],
    'shares.sql': [
      'CreateAppShare',
      'GetAppShareByApplicationID',
      'GetPublicAppShareByLink',
      'UpdateAppShare',
      'DeleteAppShare',
    ],
  };

  for (const [file, queryNames] of Object.entries(expectedQueries)) {
    const sql = readFileSync(path.join(queryDirectory, file), 'utf8');
    for (const queryName of queryNames) {
      assert.match(sql, new RegExp(`-- name: ${queryName} `));
    }
    assert.doesNotMatch(sql, /\$\{[^}]+\}/, `${file} must not interpolate SQL`);
  }
});
