const assert = require('node:assert/strict');
const { existsSync, readFileSync } = require('node:fs');
const path = require('node:path');
const test = require('node:test');

const root = path.resolve(__dirname, '..', '..');
const protoRoot = path.join(root, 'proto', 'aiflow', 'v1');
const protoFiles = ['common.proto', 'sandbox.proto'];

function messageBody(source, name) {
  const match = source.match(new RegExp(`message\\s+${name}\\s*\\{([\\s\\S]*?)\\n\\}`, 'm'));
  assert.ok(match, `message ${name} must exist`);
  return match[1];
}

test('defines only the shared Python sandbox gRPC contract', () => {
  assert.ok(existsSync(path.join(root, 'buf.yaml')), 'buf.yaml must exist');
  assert.ok(existsSync(path.join(root, 'buf.gen.yaml')), 'buf.gen.yaml must exist');

  const sources = new Map();
  for (const file of protoFiles) {
    const filePath = path.join(protoRoot, file);
    assert.ok(existsSync(filePath), `${file} must exist`);
    const source = readFileSync(filePath, 'utf8');
    assert.match(source, /^syntax = "proto3";/m);
    assert.match(source, /^package aiflow\.v1;/m);
    assert.doesNotMatch(source, /go_package|google\.protobuf\.Any|\bAny\s+\w+\s*=/);
    sources.set(file, source);
  }

  const actualProtoFiles = require('node:fs')
    .readdirSync(protoRoot)
    .filter((name) => name.endsWith('.proto'))
    .sort();
  assert.deepEqual(actualProtoFiles, protoFiles);

  const common = sources.get('common.proto');
  const context = messageBody(common, 'RequestContext');
  assert.match(context, /string request_id = 1;/);
  assert.match(context, /string trace_id = 2;/);
  assert.match(context, /string caller = 3;/);
  assert.match(context, /string idempotency_key = 4;/);
  assert.match(context, /google\.protobuf\.Timestamp deadline = 5;/);

  const sandbox = sources.get('sandbox.proto');
  assert.match(sandbox, /service SandboxService/);
  assert.match(sandbox, /rpc ExecutePython\s*\([^)]*\)\s*returns\s*\([^)]*\)/);
  assert.match(sandbox, /rpc HealthCheck\s*\([^)]*\)\s*returns\s*\([^)]*\)/);
  assert.match(messageBody(sandbox, 'ExecutePythonRequest'), /RequestContext context = 1;/);
  assert.match(
    messageBody(sandbox, 'SandboxServiceHealthCheckRequest'),
    /RequestContext context = 1;/,
  );

  for (const source of sources.values()) {
    for (const enumMatch of source.matchAll(/enum\s+(\w+)\s*\{([\s\S]*?)\n\}/g)) {
      const firstValue = enumMatch[2].match(/\b([A-Z][A-Z0-9_]*)\s*=\s*0\s*;/);
      assert.ok(firstValue, `enum ${enumMatch[1]} must define a zero value`);
      assert.match(firstValue[1], /_UNSPECIFIED$/);
    }
  }
});

test('pins deterministic Python-only Buf generation', () => {
  const config = readFileSync(path.join(root, 'buf.gen.yaml'), 'utf8');
  assert.match(config, /buf\.build\/protocolbuffers\/python:v31\.1/);
  assert.match(config, /buf\.build\/protocolbuffers\/pyi:v31\.1/);
  assert.match(config, /buf\.build\/grpc\/python:v1\.74\.0/);
  assert.match(config, /out: proto\/python\/src/);
  assert.doesNotMatch(config, /protocolbuffers\/go|grpc\/go|\.pb\.go|javascript|typescript/i);
});
