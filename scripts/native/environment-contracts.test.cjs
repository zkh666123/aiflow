const assert = require('node:assert/strict');
const { existsSync, readFileSync } = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const root = path.resolve(__dirname, '..', '..');

function requiredFile(relativePath) {
  const filePath = path.join(root, relativePath);
  assert.ok(existsSync(filePath), `${relativePath} must exist`);
  return readFileSync(filePath, 'utf8');
}

test('defines the native all-Python environment without Docker or Go tools', () => {
  const checker = requiredFile('scripts/native/check-environment.ps1');
  const initializer = requiredFile('scripts/native/initialize-database.ps1');
  const starter = requiredFile('scripts/native/start-services.ps1');
  const stopper = requiredFile('scripts/native/stop-services.ps1');
  const example = requiredFile('.env.native.example');
  const backendProject = requiredFile('flowai-studio-backend/pyproject.toml');
  const sandboxProject = requiredFile('flowai-studio-sandbox/pyproject.toml');

  assert.match(checker, /py\s+-3\.13\s+--version/);
  assert.match(checker, /uv\s+--version/);
  assert.match(checker, /pg_isready/);
  assert.match(checker, /redis-cli/);
  assert.match(checker, /pg_extension/);
  assert.doesNotMatch(checker, /docker|\bgo\b|buf|sqlc|goose/i);

  assert.match(initializer, /FLOWAI_DATABASE_URL/);
  assert.match(initializer, /FLOWAI_MIGRATION_DATABASE_URL/);
  assert.match(initializer, /FLOWAI_JWT_SECRET/);
  assert.match(initializer, /FLOWAI_API_KEY_HMAC_SECRET/);
  assert.match(initializer, /alembic[\s\S]+upgrade head/);
  assert.match(starter, /flowai-studio-backend\\\.venv\\Scripts\\python\.exe/);
  assert.match(starter, /flowai-studio-sandbox\\\.venv\\Scripts\\python\.exe/);
  assert.doesNotMatch(starter, /control-plane|ai-runtime|docker|\bgo\.exe\b/i);
  assert.doesNotMatch(stopper, /control-plane|ai-runtime/);

  assert.match(example, /FLOWAI_DATABASE_URL=/);
  assert.match(example, /FLOWAI_MIGRATION_DATABASE_URL=/);
  assert.doesNotMatch(example, /FLOWAI_CONTROL_|FLOWAI_AI_GRPC_ADDR/);
  assert.match(backendProject, /requires-python = ">=3\.13,<3\.14"/);
  assert.match(sandboxProject, /requires-python = ">=3\.13,<3\.14"/);

  assert.equal(existsSync(path.join(root, 'docker-compose.yml')), false);
  assert.equal(existsSync(path.join(root, 'flowai-studio-control-plane')), false);
  assert.equal(existsSync(path.join(root, 'toolchain', 'native-tools.json')), false);
});
