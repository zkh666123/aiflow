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

test('manages only workspace-owned hidden native processes', () => {
  const loader = requiredFile('scripts/native/load-env.ps1');
  const start = requiredFile('scripts/native/start-services.ps1');
  const stop = requiredFile('scripts/native/stop-services.ps1');
  const check = requiredFile('scripts/native/check-services.ps1');
  requiredFile('scripts/native/check-grpc.py');

  assert.match(loader, /\.env\.native/);
  assert.match(loader, /Set-Item\s+-Path\s+"Env:/);
  assert.doesNotMatch(loader, /Write-Host.*FLOWAI_GRPC_TOKEN/i);

  assert.match(start, /Start-Process/);
  assert.match(start, /-WindowStyle\s+Hidden/);
  assert.match(start, /-PassThru/);
  assert.match(start, /\.runtime/);
  assert.match(start, /ConvertTo-Json/);
  assert.match(start, /flowai-control-plane\.exe/);
  assert.match(start, /\.Equals\(\$root/);
  assert.match(start, /if \(\$Arguments\.Count -gt 0\)/);
  assert.doesNotMatch(start, /\bgo\s+run\b/);

  assert.match(stop, /Get-Process\s+-Id/);
  assert.match(stop, /\.Path/);
  assert.match(stop, /Stop-Process\s+-Id/);
  assert.doesNotMatch(stop, /Get-Process\s+-Name|taskkill|Stop-Process\s+-Name/i);

  assert.match(check, /check-grpc\.py/);
  assert.match(check, /Invoke-RestMethod/);
  assert.match(check, /127\.0\.0\.1/);
});

test('keeps the frontend proxy switchable between legacy and Go', () => {
  const config = requiredFile('flowai-studio-frontend/vite.config.ts');

  assert.match(config, /FLOWAI_BACKEND_TARGET/);
  assert.match(config, /127\.0\.0\.1:3000/);
  assert.match(config, /127\.0\.0\.1:3001/);
});
