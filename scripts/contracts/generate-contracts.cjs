const fs = require('node:fs');
const path = require('node:path');
const {
  extractFrontendCalls,
  normalizeComparablePath,
  walkFiles,
} = require('./extract-contracts.cjs');

const repoRoot = path.resolve(__dirname, '..', '..');

function toRepoPath(absolutePath) {
  return path.relative(repoRoot, absolutePath).split(path.sep).join('/');
}

function compareEntries(left, right) {
  return (
    left.method.localeCompare(right.method) ||
    left.path.localeCompare(right.path) ||
    left.source.localeCompare(right.source) ||
    left.line - right.line
  );
}

function routeKey(method, routePath) {
  return `${method}:${normalizeComparablePath(routePath)}`;
}

function findCompatibilityGaps(routes, calls) {
  const routeKeys = new Set(
    routes.map((route) => routeKey(route.method, route.path)),
  );
  const gaps = new Map();

  for (const call of calls) {
    if (routeKeys.has(routeKey(call.method, call.path))) continue;

    const key = routeKey(call.method, call.path);
    const existing = gaps.get(key) || {
      method: call.method,
      path: call.path,
      callSites: [],
    };
    existing.callSites.push({
      source: call.source,
      line: call.line,
      expression: call.expression,
    });
    gaps.set(key, existing);
  }

  return [...gaps.values()].sort(
    (left, right) =>
      left.method.localeCompare(right.method) ||
      left.path.localeCompare(right.path),
  );
}

function writeJson(filePath, value) {
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, `${JSON.stringify(value, null, 2)}\n`, 'utf8');
}

function generateContracts(
  outputRoot,
  baselineRoot = path.join(repoRoot, 'contracts'),
) {
  const frontendRoot = path.join(repoRoot, 'flowai-studio-frontend', 'src');
  const frontendFiles = walkFiles(
    frontendRoot,
    (filePath) => /\.(ts|tsx)$/.test(filePath) && !filePath.endsWith('.d.ts'),
  ).sort();

  const routeManifest = JSON.parse(
    fs.readFileSync(path.join(baselineRoot, 'http', 'routes.json'), 'utf8'),
  );
  const routes = routeManifest.routes;

  const calls = frontendFiles
    .flatMap((filePath) =>
      extractFrontendCalls(
        fs.readFileSync(filePath, 'utf8'),
        toRepoPath(filePath),
      ),
    )
    .sort(compareEntries);

  const gaps = findCompatibilityGaps(routes, calls);

  writeJson(path.join(outputRoot, 'http', 'routes.json'), routeManifest);
  writeJson(path.join(outputRoot, 'http', 'frontend-calls.json'), {
    schemaVersion: '1.0',
    source: 'current worktree React TypeScript sources',
    count: calls.length,
    calls,
  });
  writeJson(path.join(outputRoot, 'http', 'compatibility-gaps.json'), {
    schemaVersion: '1.0',
    source: 'frontend calls not present in the frozen public API baseline',
    count: gaps.length,
    gaps,
  });

  return { routes, calls, gaps };
}

if (require.main === module) {
  const result = generateContracts(path.join(repoRoot, 'contracts'));
  process.stdout.write(
    `Generated ${result.routes.length} routes, ${result.calls.length} frontend calls, and ${result.gaps.length} compatibility gaps.\n`,
  );
}

module.exports = {
  findCompatibilityGaps,
  generateContracts,
};
