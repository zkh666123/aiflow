const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { normalizeComparablePath, walkFiles } = require('./extract-contracts.cjs');
const { generateContracts } = require('./generate-contracts.cjs');

const repoRoot = path.resolve(__dirname, '..', '..');
const generatedFiles = [
  'http/routes.json',
  'http/frontend-calls.json',
  'http/compatibility-gaps.json',
];

function readJson(filePath) {
  return JSON.parse(fs.readFileSync(filePath, 'utf8'));
}

function normalizedText(filePath) {
  return fs.readFileSync(filePath, 'utf8').replace(/\r\n/g, '\n');
}

function pathMatches(pattern, actualPath) {
  const normalizedPattern = normalizeComparablePath(pattern);
  const normalizedActual = normalizeComparablePath(actualPath);

  if (normalizedPattern.endsWith('/**')) {
    const prefix = normalizedPattern.slice(0, -3);
    return (
      normalizedActual === prefix || normalizedActual.startsWith(`${prefix}/`)
    );
  }

  return normalizedPattern === normalizedActual;
}

function gapMatchesRule(gap, rule) {
  return (
    (rule.method === '*' || rule.method === gap.method) &&
    pathMatches(rule.path, gap.path)
  );
}

function validateKnownGaps(gaps, rules) {
  for (const gap of gaps) {
    if (!rules.some((rule) => gapMatchesRule(gap, rule))) {
      throw new Error(
        `undocumented compatibility gap: ${gap.method} ${gap.path}`,
      );
    }
  }

  for (const rule of rules) {
    if (!gaps.some((gap) => gapMatchesRule(gap, rule))) {
      throw new Error(`stale compatibility gap rule: ${rule.method} ${rule.path}`);
    }
  }
}

function validateSequence(events, name) {
  if (events.length === 0 || events[0].type !== 'workflow_start') {
    throw new Error(`${name} must start with workflow_start`);
  }

  const startCount = events.filter(
    (event) => event.type === 'workflow_start',
  ).length;
  if (startCount !== 1) {
    throw new Error(`${name} must contain exactly one workflow_start`);
  }

  const terminals = events
    .map((event, index) =>
      ['done', 'error'].includes(event.type) ? index : -1,
    )
    .filter((index) => index >= 0);
  if (terminals.length !== 1) {
    throw new Error(`${name} must contain exactly one terminal event`);
  }
  if (terminals[0] !== events.length - 1) {
    throw new Error(`${name} contains events after termination`);
  }
}

function validateHandReviewedContracts(contractsRoot) {
  for (const filePath of walkFiles(
    contractsRoot,
    (candidate) => candidate.endsWith('.json'),
  )) {
    readJson(filePath);
  }

  const nodeSchema = readJson(
    path.join(contractsRoot, 'workflow', 'node-types.schema.json'),
  );
  const workflowNodeTypes = nodeSchema.$defs.nodeType.enum;
  if (workflowNodeTypes.length !== 8 || nodeSchema.oneOf.length !== 8) {
    throw new Error('workflow contract must define exactly eight node types');
  }
  if (workflowNodeTypes.includes('user-input')) {
    throw new Error('user-input must not be a canonical workflow node type');
  }

  const dslSchema = readJson(
    path.join(contractsRoot, 'workflow', 'workflow-dsl.schema.json'),
  );
  if (
    dslSchema.properties.version.const !== '1.0' ||
    dslSchema.properties.kind.const !== 'Workflow' ||
    dslSchema.$defs.legacyUserInputNode.properties.type.const !== 'user-input'
  ) {
    throw new Error('workflow DSL compatibility contract is invalid');
  }

  const sseSchema = readJson(
    path.join(contractsRoot, 'sse', 'events.schema.json'),
  );
  const sseEventTypes = sseSchema.$defs.eventType.enum;
  if (sseEventTypes.length !== 6) {
    throw new Error('SSE contract must define exactly six event types');
  }

  validateSequence(
    readJson(path.join(contractsRoot, 'sse', 'valid-success-sequence.json')),
    'success sequence',
  );
  validateSequence(
    readJson(path.join(contractsRoot, 'sse', 'valid-error-sequence.json')),
    'error sequence',
  );

  const responseSchema = readJson(
    path.join(contractsRoot, 'http', 'response-envelope.schema.json'),
  );
  const requiredEnvelopeFields = [
    'success',
    'code',
    'message',
    'data',
    'timestamp',
  ];
  if (
    JSON.stringify(responseSchema.required) !==
    JSON.stringify(requiredEnvelopeFields)
  ) {
    throw new Error('API response envelope fields have drifted');
  }

  return {
    workflowNodeTypes: workflowNodeTypes.length,
    sseEventTypes: sseEventTypes.length,
  };
}

function checkContracts(contractsRoot = path.join(repoRoot, 'contracts')) {
  const generatedRoot = fs.mkdtempSync(
    path.join(os.tmpdir(), 'aiflow-generated-contracts-'),
  );

  try {
    const generated = generateContracts(generatedRoot, contractsRoot);

    for (const relativePath of generatedFiles) {
      const committedPath = path.join(contractsRoot, relativePath);
      const regeneratedPath = path.join(generatedRoot, relativePath);
      if (normalizedText(committedPath) !== normalizedText(regeneratedPath)) {
        throw new Error(`manifest drift: ${relativePath}`);
      }
    }

    const knownGaps = readJson(
      path.join(contractsRoot, 'http', 'known-gaps.json'),
    ).gaps;
    validateKnownGaps(generated.gaps, knownGaps);

    const reviewed = validateHandReviewedContracts(contractsRoot);

    return {
      routes: generated.routes.length,
      frontendCalls: generated.calls.length,
      compatibilityGaps: generated.gaps.length,
      knownGapRules: knownGaps.length,
      workflowNodeTypes: reviewed.workflowNodeTypes,
      sseEventTypes: reviewed.sseEventTypes,
    };
  } finally {
    fs.rmSync(generatedRoot, { recursive: true, force: true });
  }
}

if (require.main === module) {
  const result = checkContracts();
  process.stdout.write(
    `Contract check passed: ${result.routes} routes, ${result.frontendCalls} frontend calls, ${result.compatibilityGaps} documented gaps, ${result.workflowNodeTypes} node types, ${result.sseEventTypes} SSE event types.\n`,
  );
}

module.exports = {
  checkContracts,
  gapMatchesRule,
  pathMatches,
  validateKnownGaps,
};
