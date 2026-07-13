const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { generateContracts } = require('./generate-contracts.cjs');

test('generates deterministic HTTP and frontend contract manifests', () => {
  const outputRoot = fs.mkdtempSync(
    path.join(os.tmpdir(), 'aiflow-contracts-'),
  );

  try {
    const result = generateContracts(outputRoot);

    assert.equal(result.routes.length, 112);
    assert.ok(
      result.routes.some(
        (route) =>
          route.method === 'POST' &&
          route.path === '/api/workflows/:id/run/stream',
      ),
    );
    assert.ok(
      result.calls.some(
        (call) =>
          call.method === 'GET' && call.path === '/api/workflow/templates',
      ),
    );
    assert.ok(
      result.gaps.some(
        (gap) =>
          gap.method === 'GET' &&
          gap.path === '/api/apps/:appId/share',
      ),
    );
    assert.ok(
      result.gaps.some(
        (gap) =>
          gap.method === 'GET' && gap.path === '/api/workflow/templates',
      ),
    );

    const routesPath = path.join(outputRoot, 'http', 'routes.json');
    const callsPath = path.join(outputRoot, 'http', 'frontend-calls.json');
    const gapsPath = path.join(
      outputRoot,
      'http',
      'compatibility-gaps.json',
    );

    assert.ok(fs.existsSync(routesPath));
    assert.ok(fs.existsSync(callsPath));
    assert.ok(fs.existsSync(gapsPath));

    const firstRoutes = fs.readFileSync(routesPath, 'utf8');
    generateContracts(outputRoot);
    assert.equal(fs.readFileSync(routesPath, 'utf8'), firstRoutes);
    assert.match(firstRoutes, /\n$/);
  } finally {
    fs.rmSync(outputRoot, { recursive: true, force: true });
  }
});
