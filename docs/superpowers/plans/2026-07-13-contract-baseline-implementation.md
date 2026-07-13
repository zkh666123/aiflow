# Contract Baseline and Compatibility Harness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create an executable, machine-readable compatibility baseline for the current NestJS controllers, React API calls, eight workflow node types, workflow DSL, response envelope, and SSE event order.

**Architecture:** A dependency-light Node.js harness uses the TypeScript compiler API already installed by the legacy backend to parse source files structurally. Generated JSON manifests capture current facts; hand-reviewed JSON Schemas and invariant tests define canonical migration behavior where the current sources conflict.

**Tech Stack:** Node.js 18+, TypeScript compiler API, Node built-in `node:test`, JSON Schema draft 2020-12, and direct PowerShell-compatible Node commands.

---

### Task 1: Add TypeScript AST Extraction Library

**Files:**
- Create: `scripts/contracts/extract-contracts.cjs`
- Test: `scripts/contracts/extract-contracts.test.cjs`

- [x] **Step 1: Write failing AST extraction tests**

Create `scripts/contracts/extract-contracts.test.cjs` with focused source fixtures:

```javascript
const test = require('node:test');
const assert = require('node:assert/strict');
const {
  extractControllerRoutes,
  extractFrontendCalls,
  normalizeComparablePath,
} = require('./extract-contracts.cjs');

test('extracts controller method, full path, guard, and line', () => {
  const source = `
    @Controller('widgets')
    @UseGuards(JwtAuthGuard)
    export class WidgetController {
      @Get(':id')
      findOne() {}
    }
  `;

  assert.deepEqual(extractControllerRoutes(source, 'widget.controller.ts'), [{
    method: 'GET',
    path: '/api/widgets/:id',
    guards: ['JwtAuthGuard'],
    source: 'widget.controller.ts',
    line: 5,
  }]);
});

test('extracts request and fetch calls with template parameters', () => {
  const source = `
    request.get('/apps');
    request.patch(\`/apps/\${id}\`, data);
    fetch(\`/api/workflows/\${workflowId}/run/stream\`, { method: 'POST' });
  `;

  assert.deepEqual(extractFrontendCalls(source, 'slice.ts'), [
    { method: 'GET', path: '/api/apps', expression: "'/apps'", source: 'slice.ts', line: 2 },
    { method: 'PATCH', path: '/api/apps/:id', expression: '`/apps/${id}`', source: 'slice.ts', line: 3 },
    { method: 'POST', path: '/api/workflows/:workflowId/run/stream', expression: '`/api/workflows/${workflowId}/run/stream`', source: 'slice.ts', line: 4 },
  ]);
});

test('normalizes parameter names for route comparison', () => {
  assert.equal(
    normalizeComparablePath('/api/apps/:appId/share'),
    '/api/apps/:param/share',
  );
});
```

- [x] **Step 2: Run the tests and verify they fail**

Run:

```powershell
node --test scripts/contracts/extract-contracts.test.cjs
```

Expected: FAIL because `extract-contracts.cjs` does not exist.

- [x] **Step 3: Implement the extraction library**

Create `scripts/contracts/extract-contracts.cjs` with these exported functions:

```javascript
const fs = require('node:fs');
const path = require('node:path');
const { createRequire } = require('node:module');

const repoRoot = path.resolve(__dirname, '..', '..');
const backendRequire = createRequire(
  path.join(repoRoot, 'flowai-studio-backend', 'package.json'),
);
const ts = backendRequire('typescript');

function getDecorators(node) {
  return ts.canHaveDecorators(node) ? (ts.getDecorators(node) || []) : [];
}

function getDecoratorCall(node, name) {
  return getDecorators(node)
    .map((decorator) => decorator.expression)
    .find((expression) =>
      ts.isCallExpression(expression) &&
      ts.isIdentifier(expression.expression) &&
      expression.expression.text === name,
    );
}

function firstArgumentText(call) {
  if (!call || call.arguments.length === 0) return '';
  const argument = call.arguments[0];
  return ts.isStringLiteralLike(argument) ? argument.text : argument.getText();
}

function extractGuardNames(node) {
  const call = getDecoratorCall(node, 'UseGuards');
  return call ? call.arguments.map((argument) => argument.getText()) : [];
}

function joinApiPath(...parts) {
  const joined = parts.filter(Boolean).join('/').replace(/\/{2,}/g, '/');
  return joined.startsWith('/') ? joined : `/${joined}`;
}

function extractControllerRoutes(source, sourceName) {
  const file = ts.createSourceFile(sourceName, source, ts.ScriptTarget.Latest, true);
  const routes = [];
  const verbs = ['Get', 'Post', 'Put', 'Patch', 'Delete', 'Sse', 'Options', 'Head'];

  file.forEachChild((node) => {
    if (!ts.isClassDeclaration(node)) return;
    const controller = getDecoratorCall(node, 'Controller');
    if (!controller) return;
    const prefix = firstArgumentText(controller);
    const classGuards = extractGuardNames(node);

    for (const member of node.members) {
      if (!ts.isMethodDeclaration(member)) continue;
      for (const verb of verbs) {
        const routeDecorator = getDecoratorCall(member, verb);
        if (!routeDecorator) continue;
        const position = file.getLineAndCharacterOfPosition(member.getStart(file));
        routes.push({
          method: verb.toUpperCase(),
          path: joinApiPath('/api', prefix, firstArgumentText(routeDecorator)),
          guards: [...new Set([...classGuards, ...extractGuardNames(member)])],
          source: sourceName,
          line: position.line + 1,
        });
      }
    }
  });

  return routes;
}

function pathFromExpression(expression) {
  if (ts.isStringLiteralLike(expression)) return expression.text;
  if (!ts.isTemplateExpression(expression)) return null;
  let result = expression.head.text;
  for (const span of expression.templateSpans) {
    result += `:${span.expression.getText()}` + span.literal.text;
  }
  return result;
}

function extractFrontendCalls(source, sourceName) {
  const file = ts.createSourceFile(sourceName, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const calls = [];

  function visit(node) {
    if (ts.isCallExpression(node) && node.arguments.length > 0) {
      let method = null;
      if (ts.isIdentifier(node.expression) && node.expression.text === 'fetch') {
        method = 'GET';
        const options = node.arguments[1];
        if (options && ts.isObjectLiteralExpression(options)) {
          const property = options.properties.find((item) =>
            ts.isPropertyAssignment(item) && item.name.getText() === 'method',
          );
          if (property && ts.isPropertyAssignment(property) && ts.isStringLiteralLike(property.initializer)) {
            method = property.initializer.text.toUpperCase();
          }
        }
      } else if (ts.isPropertyAccessExpression(node.expression)) {
        const owner = node.expression.expression.getText(file);
        const verb = node.expression.name.text.toUpperCase();
        if (owner === 'request' && ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].includes(verb)) {
          method = verb;
        }
      }

      if (method) {
        const extracted = pathFromExpression(node.arguments[0]);
        if (extracted && extracted.startsWith('/')) {
          const position = file.getLineAndCharacterOfPosition(node.getStart(file));
          calls.push({
            method,
            path: extracted.startsWith('/api/') ? extracted : joinApiPath('/api', extracted),
            expression: node.arguments[0].getText(file),
            source: sourceName,
            line: position.line + 1,
          });
        }
      }
    }
    ts.forEachChild(node, visit);
  }

  visit(file);
  return calls;
}

function normalizeComparablePath(value) {
  return value.replace(/:[^/]+/g, ':param').replace(/\/{2,}/g, '/');
}

function walkFiles(directory, predicate, output = []) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) walkFiles(absolute, predicate, output);
    else if (entry.isFile() && predicate(absolute)) output.push(absolute);
  }
  return output;
}

module.exports = {
  extractControllerRoutes,
  extractFrontendCalls,
  normalizeComparablePath,
  walkFiles,
};
```

- [x] **Step 4: Run the extraction tests**

Run:

```powershell
node --test scripts/contracts/extract-contracts.test.cjs
```

Expected: 3 tests pass.

- [x] **Step 5: Commit the extraction library only**

Run:

```powershell
git add scripts/contracts/extract-contracts.cjs scripts/contracts/extract-contracts.test.cjs
git commit --only -m "test: add contract AST extraction" -- scripts/contracts/extract-contracts.cjs scripts/contracts/extract-contracts.test.cjs
```

### Task 2: Generate HTTP and Frontend Contract Manifests

**Files:**
- Create: `scripts/contracts/generate-contracts.cjs`
- Create: `contracts/http/routes.json`
- Create: `contracts/http/frontend-calls.json`
- Create: `contracts/http/compatibility-gaps.json`
- Create: `contracts/http/known-gaps.json`
- Test: `scripts/contracts/generate-contracts.test.cjs`

- [x] **Step 1: Write repository invariant tests**

The tests run the generator into a temporary directory and assert:

```javascript
assert.equal(result.routes.length, 112);
assert.ok(result.routes.some((route) => route.method === 'POST' && route.path === '/api/workflows/:id/run/stream'));
assert.ok(result.calls.some((call) => call.path === '/api/workflow/templates'));
assert.ok(result.gaps.some((gap) => gap.path === '/api/workflow/templates'));
```

- [x] **Step 2: Verify the generator test fails**

Run `node --test scripts/contracts/generate-contracts.test.cjs`.

Expected: FAIL because the generator does not exist.

- [x] **Step 3: Implement deterministic manifest generation**

`generate-contracts.cjs` must:

1. Scan `*.controller.ts` under `flowai-studio-backend/src`.
2. Scan `*.ts` and `*.tsx` under `flowai-studio-frontend/src`.
3. Sort routes and calls by method, path, source, and line.
4. Compare normalized method/path keys.
5. Write UTF-8 JSON with two-space indentation and a trailing newline.
6. Export `generateContracts(outputRoot)` for tests.
7. Write generated files only when invoked as the main module.

Known gaps must initially contain the two verified source conflicts:

```json
[
  {
    "method": "GET",
    "path": "/api/apps/:appId/share",
    "decision": "Go exposes GET in addition to the legacy POST/PATCH/DELETE routes because the frontend reads current share settings."
  },
  {
    "method": "*",
    "path": "/api/workflow/templates/**",
    "decision": "Go exposes compatibility aliases while retaining /api/templates/** as the canonical public routes."
  }
]
```

- [x] **Step 4: Run generator tests and generate committed manifests**

Run:

```powershell
node --test scripts/contracts/generate-contracts.test.cjs
node scripts/contracts/generate-contracts.cjs
```

Expected: tests pass and the three generated JSON files are created.

- [x] **Step 5: Commit generator and HTTP contracts only**

Use `git commit --only` with the exact Task 2 files so pre-existing staged changes remain staged.

### Task 3: Define Workflow Node and DSL Schemas

**Files:**
- Create: `contracts/workflow/node-types.schema.json`
- Create: `contracts/workflow/workflow.schema.json`
- Create: `contracts/workflow/workflow-dsl.schema.json`
- Create: `contracts/workflow/fixtures/minimal-workflow.json`
- Test: `scripts/contracts/workflow-contracts.test.cjs`

- [x] **Step 1: Write failing schema invariant tests**

Use Node assertions to verify:

- Exactly eight canonical node types exist.
- `userInput` is canonical and `user-input` is accepted only by DSL import compatibility.
- Condition edges may use only `true` and `false` source handles.
- DSL version is exactly `1.0` and kind is exactly `Workflow`.
- The minimal fixture has valid node and edge references and no cycle.

- [x] **Step 2: Add complete JSON Schemas**

`node-types.schema.json` defines shared `id`, `type`, `position`, and `data.label`, then uses `oneOf` for all eight node data shapes already present in `flowai-studio-frontend/src/types/index.ts`.

`workflow.schema.json` defines:

```json
{
  "id": "uuid or stable frontend id",
  "name": "non-empty string",
  "description": "optional string",
  "nodes": "array of node-types.schema.json entries",
  "edges": "array of unique id/source/target entries with optional sourceHandle/targetHandle/label",
  "variables": "optional object"
}
```

`workflow-dsl.schema.json` defines version `1.0`, kind `Workflow`, metadata, and spec. It accepts both `userInput` and legacy `user-input` on input, while the fixture uses `userInput`.

- [x] **Step 3: Run workflow contract tests**

Run `node --test scripts/contracts/workflow-contracts.test.cjs`.

Expected: all tests pass.

- [x] **Step 4: Commit workflow contracts only**

Commit only the Task 3 files with message `test: freeze workflow contract schemas`.

### Task 4: Define Response and SSE Contracts

**Files:**
- Create: `contracts/http/response-envelope.schema.json`
- Create: `contracts/sse/events.schema.json`
- Create: `contracts/sse/valid-success-sequence.json`
- Create: `contracts/sse/valid-error-sequence.json`
- Test: `scripts/contracts/sse-contracts.test.cjs`

- [x] **Step 1: Write failing SSE invariant tests**

Tests must reject sequences that:

- Do not start with exactly one `workflow_start`.
- Emit `node_status` or `heartbeat` after termination.
- Contain both `done` and `error`.
- Contain more than one terminal event.
- Put `agent_trace` after terminal completion.

- [x] **Step 2: Add canonical response and event schemas**

The response envelope requires `success`, `code`, `message`, `data`, and RFC 3339 `timestamp`; errors may add `path`.

The SSE schema defines `workflow_start`, `node_status`, `agent_trace`, `heartbeat`, `done`, and `error`. `node_status.status` is limited to `running`, `retrying`, `success`, `skipped`, `timeout`, and `failed`.

- [x] **Step 3: Run SSE tests**

Run `node --test scripts/contracts/sse-contracts.test.cjs`.

Expected: valid fixtures pass and invalid inline fixtures are rejected.

- [x] **Step 4: Commit response and SSE contracts only**

Commit only Task 4 files with message `test: freeze response and SSE contracts`.

### Task 5: Add One-Command Contract Verification

**Files:**
- Create: `scripts/contracts/check-contracts.cjs`
- Create: `contracts/README.md`
- Test: all files under `scripts/contracts/*.test.cjs`

- [x] **Step 1: Implement the checker**

The checker regenerates manifests in a temporary directory, compares them byte-for-byte with committed generated files, validates known gaps, runs workflow/SSE invariants, and exits non-zero on drift.

- [x] **Step 2: Document provenance and commands**

`contracts/README.md` explains:

- Current worktree is the baseline source.
- Generated versus hand-reviewed files.
- Conflict-resolution order from the design document.
- Commands to generate and check contracts.
- Why current legacy build failures are recorded but not copied into the target behavior.

- [x] **Step 3: Run the complete contract suite**

Run:

```powershell
node --test scripts/contracts/*.test.cjs
node scripts/contracts/generate-contracts.cjs
node scripts/contracts/check-contracts.cjs
```

Expected: all tests pass, generated files are unchanged, route count is 112, and only documented compatibility gaps remain.

- [x] **Step 4: Verify scope and commit**

Run:

```powershell
git diff --check
git status --short
```

Commit only Task 5 files with message `test: add contract drift gate`.

### Task 6: Phase Acceptance Review

**Files:**
- Modify: `docs/superpowers/plans/2026-07-13-contract-baseline-implementation.md`

- [x] **Step 1: Check off completed plan steps**

Update each completed checkbox without changing the approved scope.

- [x] **Step 2: Run final phase verification**

Run:

```powershell
node --test scripts/contracts/*.test.cjs
node scripts/contracts/check-contracts.cjs
git diff --check HEAD~5..HEAD
```

Expected: all contract tests pass and no whitespace errors exist in the phase commits.

- [x] **Step 3: Record the next plan boundary**

The next implementation plan is `Buf/gRPC contracts, native process bootstrap, schemas, and service skeletons`. Do not scaffold those services inside this contract-baseline phase.

## Completion Record

- Completed on 2026-07-13 on branch `codex/go-python-migration`.
- Contract tests: 17 passed, 0 failed.
- Generated baseline: 112 backend routes, 93 frontend calls, 11 compatibility gaps.
- Reviewed compatibility rules: 2, covering every current gap with no stale rule.
- Canonical workflow types: 8.
- Canonical SSE event types: 6.
- Next phase boundary: Buf/gRPC contracts, native PowerShell process bootstrap, `control`/`ai` schema initialization, and minimal Go/Python/WASI sandbox service skeletons.
