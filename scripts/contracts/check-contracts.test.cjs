const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { checkContracts } = require('./check-contracts.cjs');

const repoRoot = path.resolve(__dirname, '..', '..');

function copyDirectory(source, target) {
  fs.mkdirSync(target, { recursive: true });
  for (const entry of fs.readdirSync(source, { withFileTypes: true })) {
    const sourcePath = path.join(source, entry.name);
    const targetPath = path.join(target, entry.name);
    if (entry.isDirectory()) copyDirectory(sourcePath, targetPath);
    else if (entry.isFile()) fs.copyFileSync(sourcePath, targetPath);
  }
}

function copyContracts() {
  const target = fs.mkdtempSync(
    path.join(os.tmpdir(), 'aiflow-contract-check-'),
  );
  copyDirectory(path.join(repoRoot, 'contracts'), target);
  return target;
}

test('accepts the committed contract baseline', () => {
  const result = checkContracts(path.join(repoRoot, 'contracts'));
  assert.deepEqual(result, {
    routes: 112,
    frontendCalls: 93,
    compatibilityGaps: 11,
    knownGapRules: 2,
    workflowNodeTypes: 8,
    sseEventTypes: 6,
  });
});

test('rejects generated manifest drift', () => {
  const contractsRoot = copyContracts();
  try {
    const routesPath = path.join(contractsRoot, 'http', 'routes.json');
    const routes = JSON.parse(fs.readFileSync(routesPath, 'utf8'));
    routes.routes.pop();
    routes.count -= 1;
    fs.writeFileSync(routesPath, `${JSON.stringify(routes, null, 2)}\n`);

    assert.throws(() => checkContracts(contractsRoot), /manifest drift/);
  } finally {
    fs.rmSync(contractsRoot, { recursive: true, force: true });
  }
});

test('rejects undocumented compatibility gaps', () => {
  const contractsRoot = copyContracts();
  try {
    const knownGapsPath = path.join(
      contractsRoot,
      'http',
      'known-gaps.json',
    );
    fs.writeFileSync(
      knownGapsPath,
      `${JSON.stringify({ schemaVersion: '1.0', gaps: [] }, null, 2)}\n`,
    );

    assert.throws(
      () => checkContracts(contractsRoot),
      /undocumented compatibility gap/,
    );
  } finally {
    fs.rmSync(contractsRoot, { recursive: true, force: true });
  }
});
