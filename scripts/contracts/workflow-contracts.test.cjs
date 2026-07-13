const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const repoRoot = path.resolve(__dirname, '..', '..');

function loadJson(relativePath) {
  return require(path.join(repoRoot, relativePath));
}

function validateWorkflowFixture(workflow) {
  const errors = [];
  const nodeIds = new Set();
  const nodeTypes = new Map();

  for (const node of workflow.nodes) {
    if (nodeIds.has(node.id)) errors.push(`duplicate node id: ${node.id}`);
    nodeIds.add(node.id);
    nodeTypes.set(node.id, node.type);
  }

  const inDegree = new Map([...nodeIds].map((nodeId) => [nodeId, 0]));
  const adjacency = new Map([...nodeIds].map((nodeId) => [nodeId, []]));

  for (const edge of workflow.edges) {
    if (!nodeIds.has(edge.source)) {
      errors.push(`unknown edge source: ${edge.source}`);
      continue;
    }
    if (!nodeIds.has(edge.target)) {
      errors.push(`unknown edge target: ${edge.target}`);
      continue;
    }
    if (
      nodeTypes.get(edge.source) === 'condition' &&
      !['true', 'false'].includes(edge.sourceHandle)
    ) {
      errors.push(`invalid condition handle: ${edge.sourceHandle}`);
    }
    adjacency.get(edge.source).push(edge.target);
    inDegree.set(edge.target, inDegree.get(edge.target) + 1);
  }

  const queue = [...inDegree.entries()]
    .filter(([, degree]) => degree === 0)
    .map(([nodeId]) => nodeId);
  let visited = 0;
  while (queue.length > 0) {
    const nodeId = queue.shift();
    visited += 1;
    for (const target of adjacency.get(nodeId)) {
      const degree = inDegree.get(target) - 1;
      inDegree.set(target, degree);
      if (degree === 0) queue.push(target);
    }
  }
  if (visited !== nodeIds.size) errors.push('workflow contains a cycle');

  return errors;
}

test('defines exactly eight canonical workflow node types', () => {
  const schema = loadJson('contracts/workflow/node-types.schema.json');
  assert.deepEqual(schema.$defs.nodeType.enum, [
    'start',
    'userInput',
    'llm',
    'rag',
    'skill',
    'condition',
    'output',
    'agent',
  ]);
  assert.equal(schema.oneOf.length, 8);
  assert.ok(!schema.$defs.nodeType.enum.includes('user-input'));
});

test('keeps user-input only as a DSL import compatibility alias', () => {
  const schema = loadJson('contracts/workflow/workflow-dsl.schema.json');
  assert.equal(schema.properties.version.const, '1.0');
  assert.equal(schema.properties.kind.const, 'Workflow');
  assert.equal(
    schema.$defs.legacyUserInputNode.properties.type.const,
    'user-input',
  );
});

test('minimal workflow fixture has valid references, handles, and topology', () => {
  const fixture = loadJson(
    'contracts/workflow/fixtures/minimal-workflow.json',
  );
  assert.deepEqual(validateWorkflowFixture(fixture), []);
});

test('fixture validator rejects invalid condition handles and cycles', () => {
  const invalid = {
    id: 'invalid',
    name: 'Invalid',
    nodes: [
      {
        id: 'condition-1',
        type: 'condition',
        position: { x: 0, y: 0 },
        data: { label: 'Condition', conditions: [] },
      },
      {
        id: 'output-1',
        type: 'output',
        position: { x: 100, y: 0 },
        data: { label: 'Output', outputValue: '' },
      },
    ],
    edges: [
      {
        id: 'edge-1',
        source: 'condition-1',
        target: 'output-1',
        sourceHandle: 'maybe',
      },
      {
        id: 'edge-2',
        source: 'output-1',
        target: 'condition-1',
      },
    ],
  };

  assert.deepEqual(validateWorkflowFixture(invalid), [
    'invalid condition handle: maybe',
    'workflow contains a cycle',
  ]);
});
