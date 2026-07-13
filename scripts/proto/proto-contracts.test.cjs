const assert = require('node:assert/strict');
const { existsSync, readFileSync } = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const root = path.resolve(__dirname, '..', '..');
const protoRoot = path.join(root, 'proto', 'aiflow', 'v1');
const protoFiles = [
  'common.proto',
  'execution.proto',
  'documents.proto',
  'retrieval.proto',
  'mcp.proto',
  'models.proto',
  'sandbox.proto',
];

const expectedServices = {
  ExecutionService: ['ExecuteNode:server_streaming'],
  DocumentService: ['IngestDocument:client_streaming'],
  RetrievalService: ['Retrieve:unary'],
  McpService: ['ManageMcp:unary'],
  ModelService: ['ListModels:unary', 'HealthCheck:unary'],
  SandboxService: ['ExecutePython:unary', 'HealthCheck:unary'],
};

function messageBody(source, name) {
  const match = source.match(new RegExp(`message\\s+${name}\\s*\\{([\\s\\S]*?)\\n\\}`, 'm'));
  assert.ok(match, `message ${name} must exist`);
  return match[1];
}

test('defines the complete typed aiflow.v1 service surface', () => {
  const bufPath = path.join(root, 'buf.yaml');
  const generatePath = path.join(root, 'buf.gen.yaml');
  assert.ok(existsSync(bufPath), 'buf.yaml must exist');
  assert.ok(existsSync(generatePath), 'buf.gen.yaml must exist');

  const sources = new Map();
  for (const file of protoFiles) {
    const filePath = path.join(protoRoot, file);
    assert.ok(existsSync(filePath), `${file} must exist`);
    const source = readFileSync(filePath, 'utf8');
    assert.match(source, /^syntax = "proto3";/m, `${file} must use proto3`);
    assert.match(source, /^package aiflow\.v1;/m, `${file} must use aiflow.v1`);
    assert.match(
      source,
      /option go_package = "github\.com\/gulugulu33\/aiflow-studio\/flowai-studio-control-plane\/internal\/gen\/aiflow\/v1;aiflowv1";/,
      `${file} must use the canonical Go package`,
    );
    assert.doesNotMatch(source, /google\.protobuf\.Any|\bAny\s+\w+\s*=/);
    sources.set(file, source);
  }

  const allSources = [...sources.values()].join('\n');
  const actualServices = {};
  for (const serviceMatch of allSources.matchAll(/service\s+(\w+)\s*\{([\s\S]*?)\n\}/g)) {
    const methods = [];
    for (const method of serviceMatch[2].matchAll(
      /rpc\s+(\w+)\s*\(\s*(stream\s+)?[\w.]+\s*\)\s*returns\s*\(\s*(stream\s+)?[\w.]+\s*\)/g,
    )) {
      const mode = method[2] ? 'client_streaming' : method[3] ? 'server_streaming' : 'unary';
      methods.push(`${method[1]}:${mode}`);
    }
    actualServices[serviceMatch[1]] = methods;
  }
  assert.deepEqual(actualServices, expectedServices);

  const common = sources.get('common.proto');
  const context = messageBody(common, 'RequestContext');
  assert.match(context, /string request_id = 1;/);
  assert.match(context, /string trace_id = 2;/);
  assert.match(context, /string caller = 3;/);
  assert.match(context, /string idempotency_key = 4;/);
  assert.match(context, /google\.protobuf\.Timestamp deadline = 5;/);

  const requestMessages = {
    'execution.proto': ['ExecuteNodeRequest'],
    'documents.proto': ['IngestDocumentMetadata'],
    'retrieval.proto': ['RetrieveRequest'],
    'mcp.proto': ['ManageMcpRequest'],
    'models.proto': ['ListModelsRequest', 'ModelServiceHealthCheckRequest'],
    'sandbox.proto': ['ExecutePythonRequest', 'SandboxServiceHealthCheckRequest'],
  };
  for (const [file, messages] of Object.entries(requestMessages)) {
    for (const message of messages) {
      assert.match(messageBody(sources.get(file), message), /RequestContext context = 1;/);
    }
  }

  for (const enumMatch of allSources.matchAll(/enum\s+(\w+)\s*\{([\s\S]*?)\n\}/g)) {
    const firstValue = enumMatch[2].match(/\b([A-Z][A-Z0-9_]*)\s*=\s*0\s*;/);
    assert.ok(firstValue, `enum ${enumMatch[1]} must define a zero value`);
    assert.match(firstValue[1], /_UNSPECIFIED$/, `enum ${enumMatch[1]} zero value must be *_UNSPECIFIED`);
  }
});

test('pins deterministic Go and Python Buf generation', () => {
  const manifest = JSON.parse(
    readFileSync(path.join(root, 'toolchain', 'native-tools.json'), 'utf8'),
  );
  const config = readFileSync(path.join(root, 'buf.gen.yaml'), 'utf8');

  for (const [plugin, version] of Object.entries(manifest.bufPlugins)) {
    assert.match(config, new RegExp(`buf\\.build/${plugin}:${version.replaceAll('.', '\\.')}\\b`));
  }
  assert.match(config, /out: flowai-studio-control-plane\/internal\/gen/);
  assert.match(config, /out: proto\/python\/src/);
  assert.doesNotMatch(config, /javascript|typescript|js_out|ts_proto/i);
});
