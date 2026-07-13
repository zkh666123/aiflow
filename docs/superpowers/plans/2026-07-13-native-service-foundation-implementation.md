# Native Service Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the tested native foundation for the Go control plane, Python AI runtime, CPython WASI sandbox manager, authenticated gRPC contracts, and isolated PostgreSQL schemas without Docker.

**Architecture:** Go and both Python services run as native Windows processes. PostgreSQL 16 with pgvector and Redis 7 run in WSL and are reached through loopback. Buf owns the internal `aiflow.v1` contracts, Go is the only public HTTP service, Python listeners are authenticated and loopback-only, and the sandbox refuses user code until a checksum-verified CPython 3.13 WASI artifact is present.

**Tech Stack:** Go 1.26, Gin, pgx, sqlc, goose, gRPC-Go, Python 3.13, uv, grpcio, Pydantic v2, SQLAlchemy 2, Alembic, wasmtime-py, PostgreSQL 16, pgvector 0.8.5, Redis 7, Buf, PowerShell.

---

## Scope And File Map

This phase creates the following ownership units:

- `toolchain/native-tools.json`: pinned command-line tool versions and generated-code plugin versions.
- `scripts/native/`: environment checks, database bootstrap, process lifecycle, and native integration checks.
- `proto/aiflow/v1/`: source-of-truth protobuf contracts for Go-to-AI and AI-to-sandbox calls.
- `proto/python/`: generated Python protobuf package shared by the AI runtime and sandbox.
- `flowai-studio-control-plane/`: Go module, generated Go protobuf code, database migrations/queries, and `/api/health`.
- `flowai-studio-ai-runtime/`: Python package, authenticated gRPC server, database migration, and model health service.
- `flowai-studio-sandbox/`: authenticated loopback gRPC service and fail-closed WASI runtime boundary.

The existing NestJS and React trees remain available as compatibility evidence. This phase must not modify or delete them.

### Fixed Local Ports

| Service | Address | Exposure |
| --- | --- | --- |
| Legacy NestJS | `127.0.0.1:3000` | Temporary compatibility oracle |
| Go control plane | `127.0.0.1:3001` | Frontend-facing migration target |
| Python AI runtime gRPC | `127.0.0.1:50051` | Authenticated loopback only |
| Python sandbox gRPC | `127.0.0.1:50052` | Authenticated loopback only |
| WSL PostgreSQL | `127.0.0.1:5432` | Native backend clients |
| WSL Redis | `127.0.0.1:6379` | Native backend clients |

## Task 1: Pin And Install The Native Toolchain

**Files:**
- Create: `toolchain/native-tools.json`
- Create: `scripts/native/environment-contracts.test.cjs`
- Create: `scripts/native/check-environment.ps1`
- Modify: `.gitignore`

- [ ] **Step 1: Write the failing toolchain contract test**

The Node test must assert exact, non-floating versions for Buf, sqlc, goose, all four Buf plugins, Go, and Python. It must also assert that `check-environment.ps1` checks WSL PostgreSQL, pgvector, and Redis and contains no Docker command.

```javascript
test('pins the native toolchain without Docker', () => {
  const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
  assert.equal(manifest.tools.buf, 'v1.71.0');
  assert.equal(manifest.tools.sqlc, 'v1.31.1');
  assert.equal(manifest.tools.goose, 'v3.27.2');
  assert.equal(manifest.runtimes.go, '1.26');
  assert.equal(manifest.runtimes.python, '3.13');
  assert.doesNotMatch(JSON.stringify(manifest), /latest/i);
  assert.match(script, /pg_isready/);
  assert.match(script, /redis-cli/);
  assert.match(script, /pg_extension/);
  assert.doesNotMatch(script, /docker/i);
});
```

- [ ] **Step 2: Run the test and confirm the missing manifest failure**

Run: `node --test scripts/native/environment-contracts.test.cjs`

Expected: FAIL because `toolchain/native-tools.json` does not exist.

- [ ] **Step 3: Add the pinned manifest and environment checker**

Pin these tool versions:

```json
{
  "runtimes": { "go": "1.26", "python": "3.13", "postgresql": "16", "pgvector": "0.8.5", "redis": "7" },
  "tools": { "buf": "v1.71.0", "sqlc": "v1.31.1", "goose": "v3.27.2" },
  "bufPlugins": {
    "protocolbuffers/go": "v1.36.10",
    "grpc/go": "v1.5.1",
    "protocolbuffers/python": "v31.1",
    "grpc/python": "v1.74.0"
  }
}
```

`check-environment.ps1` must return non-zero unless all of the following are true:

```powershell
go version
py -3.13 --version
uv --version
buf --version
sqlc version
goose -version
wsl.exe -- pg_isready -h 127.0.0.1 -p 5432
wsl.exe -- redis-cli -h 127.0.0.1 -p 6379 ping
wsl.exe -u postgres -- psql -X -Atc "SELECT extversion FROM pg_extension WHERE extname='vector';"
```

Add `.runtime/`, `flowai-studio-sandbox/runtime/`, and `*.wasm` to `.gitignore`; keep `.env.example` files tracked and all real `.env*` files ignored.

- [ ] **Step 4: Install and verify the pinned Go tools**

Run:

```powershell
go install github.com/bufbuild/buf/cmd/buf@v1.71.0
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
go install github.com/pressly/goose/v3/cmd/goose@v3.27.2
node --test scripts/native/environment-contracts.test.cjs
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/check-environment.ps1
```

Expected: the test passes and the environment checker reports Go 1.26, Python 3.13, PostgreSQL 16, pgvector 0.8.5, Redis 7, and the three pinned tools.

- [ ] **Step 5: Commit the toolchain increment**

Commit only Task 1 files with message `chore: pin native migration toolchain`.

## Task 2: Define And Validate The Protobuf Contracts

**Files:**
- Create: `buf.yaml`
- Create: `buf.gen.yaml`
- Create: `proto/aiflow/v1/common.proto`
- Create: `proto/aiflow/v1/execution.proto`
- Create: `proto/aiflow/v1/documents.proto`
- Create: `proto/aiflow/v1/retrieval.proto`
- Create: `proto/aiflow/v1/mcp.proto`
- Create: `proto/aiflow/v1/models.proto`
- Create: `proto/aiflow/v1/sandbox.proto`
- Create: `scripts/proto/proto-contracts.test.cjs`

- [ ] **Step 1: Write the failing protobuf invariant test**

The test must inspect source files and assert:

```javascript
assert.deepEqual(serviceMethods, {
  ExecutionService: ['ExecuteNode:server_streaming'],
  DocumentService: ['IngestDocument:client_streaming'],
  RetrievalService: ['Retrieve:unary'],
  McpService: ['ManageMcp:unary'],
  ModelService: ['ListModels:unary', 'HealthCheck:unary'],
  SandboxService: ['ExecutePython:unary', 'HealthCheck:unary'],
});
```

It must also require `RequestContext` fields for request ID, trace ID, caller, idempotency key, and deadline; require zero-valued `*_UNSPECIFIED` enum members; reject `google.protobuf.Any`; and verify that every service request carries `RequestContext` directly or through its first upload frame.

- [ ] **Step 2: Run the test and confirm missing contract failures**

Run: `node --test scripts/proto/proto-contracts.test.cjs`

Expected: FAIL because the protobuf files do not exist.

- [ ] **Step 3: Add the Buf module and typed contracts**

Use package `aiflow.v1`, this Go package option, and no JavaScript generation:

```proto
option go_package = "github.com/gulugulu33/aiflow-studio/flowai-studio-control-plane/internal/gen/aiflow/v1;aiflowv1";
```

Contract rules:

- `ExecuteNodeRequest` supports only `LLM`, `RAG`, `AGENT`, and `SKILL` node specs through `oneof`.
- `ExecuteNodeEvent` has a monotonically increasing `sequence`, timestamp, node ID, and typed payloads for start, token, trace, output, usage, error, and done.
- `IngestDocumentRequest` uses `oneof` metadata/chunk; metadata must be the first frame and chunks contain bytes plus sequence.
- `RetrieveRequest` uses an explicit `VECTOR`, `KEYWORD`, or `HYBRID` mode and optional reranker policy.
- `ManageMcpRequest` and response use typed `oneof` variants for configure, connect, disconnect, discover, and call.
- Model records expose provider, capabilities, context window, health, input/output cost, and configuration state.
- `ExecutePythonRequest` contains code and explicit limits; the response contains stable status, stdout, stderr, duration, and failure code.

`buf.gen.yaml` must pin the four plugin versions from `toolchain/native-tools.json`, write Go code under `flowai-studio-control-plane/internal/gen`, and write Python code/stubs under `proto/python/src`.

- [ ] **Step 4: Run source and Buf validation**

Run:

```powershell
node --test scripts/proto/proto-contracts.test.cjs
buf lint
buf build
```

Expected: all commands pass with no lint findings.

- [ ] **Step 5: Commit the protobuf source increment**

Commit only Task 2 files with message `feat: define internal grpc contracts`.

## Task 3: Generate Shared Go And Python Stubs

**Files:**
- Create: `proto/python/pyproject.toml`
- Create: `proto/python/src/aiflow/__init__.py`
- Create: `proto/python/src/aiflow/v1/__init__.py`
- Generate: `proto/python/src/aiflow/v1/*_pb2.py`
- Generate: `proto/python/src/aiflow/v1/*_pb2.pyi`
- Generate: `proto/python/src/aiflow/v1/*_pb2_grpc.py`
- Generate: `flowai-studio-control-plane/internal/gen/aiflow/v1/*.pb.go`
- Create: `scripts/proto/generated-contracts.test.cjs`

- [ ] **Step 1: Write the failing generated-code test**

Assert that every source proto has Go and Python outputs, generated headers are present, no generated file is outside the two approved directories, and Python gRPC imports resolve after adding `proto/python/src` to `PYTHONPATH`.

- [ ] **Step 2: Run the test and confirm generated files are missing**

Run: `node --test scripts/proto/generated-contracts.test.cjs`

Expected: FAIL because generated files do not exist.

- [ ] **Step 3: Generate and package the stubs**

Run:

```powershell
buf generate
$env:PYTHONPATH = (Resolve-Path 'proto/python/src')
py -3.13 -c "from aiflow.v1 import execution_pb2, execution_pb2_grpc, sandbox_pb2; print('python proto imports ok')"
```

`proto/python/pyproject.toml` must define package `flowai-proto`, require Python 3.13, and pin compatible `protobuf==6.31.1` and `grpcio==1.74.0` runtimes.

- [ ] **Step 4: Verify deterministic generation**

Run `buf generate` a second time, then run:

```powershell
node --test scripts/proto/generated-contracts.test.cjs
git diff --exit-code -- proto/python/src flowai-studio-control-plane/internal/gen
```

Expected: generated tests pass and the second generation creates no diff.

- [ ] **Step 5: Commit generated contracts**

Commit only Task 3 files with message `chore: generate grpc language bindings`.

## Task 4: Bootstrap PostgreSQL Roles And Owned Schemas

**Files:**
- Create: `.env.native.example`
- Create: `scripts/native/initialize-database.ps1`
- Create: `scripts/native/database-contracts.test.ps1`
- Create: `flowai-studio-control-plane/db/migrations/00001_control_schema.sql`
- Create: `flowai-studio-control-plane/db/query/health.sql`
- Create: `flowai-studio-control-plane/sqlc.yaml`
- Create: `flowai-studio-ai-runtime/alembic.ini`
- Create: `flowai-studio-ai-runtime/alembic/env.py`
- Create: `flowai-studio-ai-runtime/alembic/versions/0001_ai_schema.py`

- [ ] **Step 1: Write the failing database ownership test**

The PowerShell test must run the bootstrap twice and prove:

- Database `flowai_studio` exists with extension `vector` version `0.8.5` or newer.
- Schemas `control` and `ai` exist.
- `flowai_control` can create no object in `ai` and `flowai_ai` can create no object in `control`.
- Runtime roles can select/insert/update/delete tables in their own schema after migrations.
- Runtime roles cannot create or alter tables.
- Goose and Alembic version state is stored in the owned schema.

- [ ] **Step 2: Run the test and confirm bootstrap files are missing**

Run: `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/database-contracts.test.ps1`

Expected: FAIL because `initialize-database.ps1` does not exist.

- [ ] **Step 3: Implement idempotent secret and role bootstrap**

The script generates `.env.native` on first run with cryptographically random values for:

```text
FLOWAI_GRPC_TOKEN
FLOWAI_CONTROL_DATABASE_URL
FLOWAI_CONTROL_MIGRATION_DATABASE_URL
FLOWAI_AI_DATABASE_URL
FLOWAI_AI_MIGRATION_DATABASE_URL
FLOWAI_REDIS_URL
```

It sends role/password SQL to `wsl.exe -u postgres -- psql` over stdin so passwords do not appear in command arguments or logs. Use separate migrator and runtime roles for `control` and `ai`. Create the database and extension before running goose, sqlc generation, and Alembic.

- [ ] **Step 4: Apply migrations and verify ownership**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/initialize-database.ps1
sqlc generate -f flowai-studio-control-plane/sqlc.yaml
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/database-contracts.test.ps1
```

Expected: the bootstrap is idempotent and every positive/negative privilege assertion passes.

- [ ] **Step 5: Commit database foundation**

Commit only Task 4 files, excluding `.env.native`, with message `feat: bootstrap isolated postgres schemas`.

## Task 5: Build The Authenticated Python AI Runtime Skeleton

**Files:**
- Create: `flowai-studio-ai-runtime/pyproject.toml`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/config.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/grpc/auth.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/grpc/model_service.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/server.py`
- Create: `flowai-studio-ai-runtime/tests/test_config.py`
- Create: `flowai-studio-ai-runtime/tests/test_grpc_auth.py`
- Create: `flowai-studio-ai-runtime/tests/test_model_service.py`
- Generate: `flowai-studio-ai-runtime/uv.lock`

- [ ] **Step 1: Write failing config, auth, and health tests**

Tests must require loopback binding, reject missing/blank service tokens, return `UNAUTHENTICATED` for absent or wrong metadata, use `hmac.compare_digest`, and return database/Redis/provider health without exposing connection strings or exceptions.

- [ ] **Step 2: Run tests and confirm imports fail**

Run: `uv run --project flowai-studio-ai-runtime pytest flowai-studio-ai-runtime/tests -q`

Expected: FAIL because `aiflow_runtime` modules do not exist.

- [ ] **Step 3: Implement the minimum server**

The server must:

```python
server = grpc.aio.server(interceptors=[ServiceTokenInterceptor(settings.grpc_token)])
server.add_insecure_port(settings.grpc_address)  # address validated as loopback
```

Register every public AI service. Only `ModelService.ListModels` and `HealthCheck` return implemented responses in this phase; other RPCs return gRPC `UNIMPLEMENTED` without starting background work. Health checks use bounded deadlines and stable status fields.

- [ ] **Step 4: Lock dependencies and run tests**

Pin FastAPI, Pydantic v2, pydantic-settings, SQLAlchemy 2, psycopg 3, Alembic, grpcio 1.74.0, protobuf 6.31.1, redis, and pytest through `uv.lock`.

Run:

```powershell
uv sync --project flowai-studio-ai-runtime
uv run --project flowai-studio-ai-runtime pytest flowai-studio-ai-runtime/tests -q
uv run --project flowai-studio-ai-runtime python -m compileall -q flowai-studio-ai-runtime/src
```

Expected: all tests pass and compileall exits zero.

- [ ] **Step 5: Commit the AI runtime skeleton**

Commit only Task 5 files with message `feat: add authenticated ai runtime skeleton`.

## Task 6: Build The Fail-Closed WASI Sandbox Skeleton

**Files:**
- Create: `flowai-studio-sandbox/pyproject.toml`
- Create: `flowai-studio-sandbox/src/aiflow_sandbox/config.py`
- Create: `flowai-studio-sandbox/src/aiflow_sandbox/grpc/auth.py`
- Create: `flowai-studio-sandbox/src/aiflow_sandbox/grpc/service.py`
- Create: `flowai-studio-sandbox/src/aiflow_sandbox/wasi/artifact.py`
- Create: `flowai-studio-sandbox/src/aiflow_sandbox/server.py`
- Create: `flowai-studio-sandbox/tests/test_artifact.py`
- Create: `flowai-studio-sandbox/tests/test_service.py`
- Create: `flowai-studio-sandbox/tests/test_host_safety.py`
- Generate: `flowai-studio-sandbox/uv.lock`

- [ ] **Step 1: Write failing fail-closed sandbox tests**

Tests must prove that a missing WASI module makes health `NOT_READY`, `ExecutePython` returns `FAILED_PRECONDITION`, wrong token returns `UNAUTHENTICATED`, a valid artifact requires an expected SHA-256 digest, and production source contains no imports or calls to `subprocess`, `os.system`, `eval`, `exec`, `shell=True`, or native Python code execution.

- [ ] **Step 2: Run tests and confirm imports fail**

Run: `uv run --project flowai-studio-sandbox pytest flowai-studio-sandbox/tests -q`

Expected: FAIL because the sandbox package does not exist.

- [ ] **Step 3: Implement only the safe boundary**

The skeleton loads no guest code. It validates the configured `python.wasm` path and digest, binds only to `127.0.0.1:50052`, authenticates every RPC, and rejects execution until the later sandbox implementation phase supplies the verified CPython WASI artifact and resource-limited Wasmtime runner.

- [ ] **Step 4: Lock dependencies and run tests**

Pin Pydantic v2, pydantic-settings, grpcio 1.74.0, protobuf 6.31.1, wasmtime-py, and pytest through `uv.lock`.

Run:

```powershell
uv sync --project flowai-studio-sandbox
uv run --project flowai-studio-sandbox pytest flowai-studio-sandbox/tests -q
uv run --project flowai-studio-sandbox python -m compileall -q flowai-studio-sandbox/src
```

Expected: all tests pass and the host-safety source scan finds no forbidden execution path.

- [ ] **Step 5: Commit the sandbox skeleton**

Commit only Task 6 files with message `feat: add fail-closed wasi sandbox skeleton`.

## Task 7: Build The Go Control Plane Health Slice

**Files:**
- Create: `flowai-studio-control-plane/go.mod`
- Create: `flowai-studio-control-plane/cmd/api/main.go`
- Create: `flowai-studio-control-plane/internal/config/config.go`
- Create: `flowai-studio-control-plane/internal/grpcclient/ai.go`
- Create: `flowai-studio-control-plane/internal/httpapi/envelope.go`
- Create: `flowai-studio-control-plane/internal/httpapi/health.go`
- Create: `flowai-studio-control-plane/internal/httpapi/router.go`
- Create: `flowai-studio-control-plane/internal/httpapi/health_test.go`
- Create: `flowai-studio-control-plane/internal/config/config_test.go`

- [ ] **Step 1: Write failing config and HTTP compatibility tests**

Tests must require loopback AI and sandbox addresses, reject missing gRPC tokens, and prove `GET /api/health` returns the frozen response envelope with `database`, `redis`, `pgvector`, and `aiRuntime` checks. A failed dependency yields HTTP 200 with business status `degraded`, matching the legacy endpoint behavior.

- [ ] **Step 2: Run tests and confirm missing package failures**

Run: `go test ./...` from `flowai-studio-control-plane`.

Expected: FAIL because the packages do not exist.

- [ ] **Step 3: Implement the minimum Go service**

Use dependency interfaces in the health handler so tests use real handler behavior with deterministic fakes. Production wiring uses pgx, go-redis, and an authenticated gRPC client. The response remains:

```json
{
  "success": true,
  "code": "SUCCESS",
  "message": "success",
  "data": { "status": "healthy", "timestamp": "RFC3339", "checks": {} },
  "timestamp": "RFC3339"
}
```

The server binds to `127.0.0.1:3001` by default, applies request IDs and panic recovery, and shuts down on Ctrl+C with a bounded timeout.

- [ ] **Step 4: Generate, format, and verify Go code**

Run:

```powershell
sqlc generate -f flowai-studio-control-plane/sqlc.yaml
go mod tidy
gofmt -w flowai-studio-control-plane
go test ./...
go vet ./...
```

Run the last three commands from `flowai-studio-control-plane`. Expected: tests and vet pass.

- [ ] **Step 5: Commit the Go health slice**

Commit only Task 7 files with message `feat: add control plane health endpoint`.

## Task 8: Add Native Process Lifecycle And Integration Verification

**Files:**
- Create: `scripts/native/load-env.ps1`
- Create: `scripts/native/start-services.ps1`
- Create: `scripts/native/stop-services.ps1`
- Create: `scripts/native/check-services.ps1`
- Create: `scripts/native/process-contracts.test.cjs`
- Modify: `flowai-studio-frontend/vite.config.ts`

- [ ] **Step 1: Write failing lifecycle contract tests**

Tests must require hidden background windows, PID files under `.runtime`, resolved workspace-bound working directories, loopback-only service addresses, process identity validation before stop, and a temporary Vite route switch that can target either NestJS `3000` or Go `3001` without frontend code changes.

- [ ] **Step 2: Run tests and confirm scripts are missing**

Run: `node --test scripts/native/process-contracts.test.cjs`

Expected: FAIL because lifecycle scripts do not exist.

- [ ] **Step 3: Implement safe native lifecycle scripts**

`start-services.ps1` must check PostgreSQL/Redis first, then start sandbox, AI runtime, and Go in that order. It writes PID plus executable and command-line identity, refuses duplicate starts, and waits for health before continuing. `stop-services.ps1` stops only matching workspace processes in reverse order and never uses broad name-based termination.

- [ ] **Step 4: Run the native integration path**

Run:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/start-services.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/check-services.ps1
Invoke-RestMethod http://127.0.0.1:3001/api/health | ConvertTo-Json -Depth 8
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/stop-services.ps1
```

Expected: AI and Go report healthy, sandbox reports explicit `NOT_READY` until its checked WASI artifact exists, the public Go envelope reports that state without exposing internals, and all three native processes stop cleanly.

- [ ] **Step 5: Commit lifecycle integration**

Commit only Task 8 files with message `feat: add native service lifecycle scripts`.

## Task 9: Phase Acceptance Review

**Files:**
- Modify: `docs/superpowers/plans/2026-07-13-native-service-foundation-implementation.md`
- Modify: `README.md`

- [ ] **Step 1: Run every deterministic suite**

```powershell
node --test scripts/contracts/*.test.cjs
node scripts/contracts/check-contracts.cjs
node --test scripts/native/*.test.cjs scripts/proto/*.test.cjs
buf lint
buf build
go test ./...
go vet ./...
uv run --project flowai-studio-ai-runtime pytest flowai-studio-ai-runtime/tests -q
uv run --project flowai-studio-sandbox pytest flowai-studio-sandbox/tests -q
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/database-contracts.test.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/check-services.ps1
```

Run Go commands from `flowai-studio-control-plane`.

- [ ] **Step 2: Verify repository and security boundaries**

Run:

```powershell
rg -n "Docker|Compose|subprocess|os\.system|shell=True|eval\(|exec\(" flowai-studio-control-plane flowai-studio-ai-runtime flowai-studio-sandbox scripts/native
git diff --check
git status --short
```

Expected: Docker/Compose are absent from the new runtime path; any `eval`/`exec` strings appear only in negative source-scan tests; real secrets and `.wasm` artifacts are untracked; the user's pre-existing staged files remain staged and unchanged.

- [ ] **Step 3: Record completion and the next phase boundary**

Check off completed tasks and record exact versions, test counts, local addresses, database role assertions, and known fail-closed sandbox status. The next plan begins Go identity/app/team/RBAC compatibility routes; it must not claim Python code execution until the CPython WASI artifact and Wasmtime resource-limit tests exist.

- [ ] **Step 4: Commit phase documentation**

Commit only the plan and README changes with message `docs: record native foundation completion`.

