const assert = require('node:assert/strict');
const { existsSync, readFileSync } = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const root = path.resolve(__dirname, '..', '..');
const manifestPath = path.join(root, 'toolchain', 'native-tools.json');
const checkerPath = path.join(root, 'scripts', 'native', 'check-environment.ps1');
const installerPath = path.join(root, 'scripts', 'native', 'install-tools.ps1');

test('pins the native toolchain without Docker', () => {
  assert.ok(existsSync(manifestPath), 'toolchain/native-tools.json must exist');
  assert.ok(existsSync(checkerPath), 'scripts/native/check-environment.ps1 must exist');
  assert.ok(existsSync(installerPath), 'scripts/native/install-tools.ps1 must exist');

  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
  const script = readFileSync(checkerPath, 'utf8');
  const installer = readFileSync(installerPath, 'utf8');

  assert.deepEqual(manifest.runtimes, {
    go: '1.26',
    python: '3.13',
    postgresql: '16',
    pgvector: '0.8.5',
    redis: '7',
  });
  assert.deepEqual(manifest.tools, {
    buf: 'v1.71.0',
    sqlc: 'v1.31.1',
    goose: 'v3.27.2',
  });
  assert.deepEqual(manifest.bufPlugins, {
    'protocolbuffers/go': 'v1.36.10',
    'grpc/go': 'v1.5.1',
    'protocolbuffers/python': 'v31.1',
    'protocolbuffers/pyi': 'v31.1',
    'grpc/python': 'v1.74.0',
  });
  assert.deepEqual(manifest.artifacts, {
    buf: {
      url: 'https://github.com/bufbuild/buf/releases/download/v1.71.0/buf-Windows-x86_64.exe',
      sha256: 'b003ead3eebe7920a4e6f748fdf5b666e4763637a7fb1837c975ac9c5d21d558',
    },
    sqlc: {
      url: 'https://github.com/sqlc-dev/sqlc/releases/download/v1.31.1/sqlc_1.31.1_windows_amd64.zip',
      sha256: '352711fa7dcb05dcdfefca0ad71b2c9a74fd090f8d7fc609419de4cbc725429f',
    },
    goose: {
      url: 'https://github.com/pressly/goose/releases/download/v3.27.2/goose_windows_x86_64.exe',
      sha256: '0a802cb50e2b3ee7950bc73a3c31e8f1c186e252731b6e2b1c110dbdc3623646',
    },
  });

  assert.doesNotMatch(JSON.stringify(manifest), /latest/i);
  assert.match(script, /pg_isready/);
  assert.match(script, /redis-cli/);
  assert.match(script, /pg_extension/);
  assert.match(script, /buf\s+--version/);
  assert.match(script, /sqlc\s+version/);
  assert.match(script, /goose\s+-version/);
  assert.doesNotMatch(script, /docker/i);
  assert.match(installer, /Get-FileHash/);
  assert.match(installer, /Expand-Archive/);
  assert.doesNotMatch(installer, /go\s+install/);
});
