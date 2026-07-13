const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const repoRoot = path.resolve(__dirname, '..', '..');
const terminalTypes = new Set(['done', 'error']);

function loadJson(relativePath) {
  return require(path.join(repoRoot, relativePath));
}

function validateSequence(events) {
  if (events.length === 0 || events[0].type !== 'workflow_start') {
    throw new Error('sequence must start with workflow_start');
  }

  const startCount = events.filter(
    (event) => event.type === 'workflow_start',
  ).length;
  if (startCount !== 1) {
    throw new Error('sequence must contain exactly one workflow_start');
  }

  const terminalIndexes = events
    .map((event, index) => (terminalTypes.has(event.type) ? index : -1))
    .filter((index) => index >= 0);
  if (terminalIndexes.length !== 1) {
    throw new Error('sequence must contain exactly one terminal event');
  }
  if (terminalIndexes[0] !== events.length - 1) {
    throw new Error('no events are allowed after the terminal event');
  }
}

test('defines the stable API response envelope', () => {
  const schema = loadJson('contracts/http/response-envelope.schema.json');
  assert.deepEqual(schema.required, [
    'success',
    'code',
    'message',
    'data',
    'timestamp',
  ]);
  assert.equal(schema.properties.timestamp.format, 'date-time');
  assert.equal(schema.properties.path.type, 'string');
});

test('defines all canonical SSE event and node status types', () => {
  const schema = loadJson('contracts/sse/events.schema.json');
  assert.deepEqual(schema.$defs.eventType.enum, [
    'workflow_start',
    'node_status',
    'agent_trace',
    'heartbeat',
    'done',
    'error',
  ]);
  assert.deepEqual(schema.$defs.nodeStatus.enum, [
    'running',
    'retrying',
    'success',
    'skipped',
    'timeout',
    'failed',
  ]);
});

test('accepts canonical success and error sequences', () => {
  validateSequence(loadJson('contracts/sse/valid-success-sequence.json'));
  validateSequence(loadJson('contracts/sse/valid-error-sequence.json'));
});

test('rejects a sequence that does not start with workflow_start', () => {
  assert.throws(
    () => validateSequence([{ type: 'node_status', data: {} }]),
    /start with workflow_start/,
  );
});

test('rejects duplicate terminal events', () => {
  assert.throws(
    () =>
      validateSequence([
        { type: 'workflow_start', data: {} },
        { type: 'done', data: {} },
        { type: 'error', data: {} },
      ]),
    /exactly one terminal event/,
  );
});

test('rejects node, trace, or heartbeat events after termination', () => {
  for (const type of ['node_status', 'agent_trace', 'heartbeat']) {
    assert.throws(
      () =>
        validateSequence([
          { type: 'workflow_start', data: {} },
          { type: 'done', data: {} },
          { type, data: {} },
        ]),
      /after the terminal event/,
    );
  }
});
