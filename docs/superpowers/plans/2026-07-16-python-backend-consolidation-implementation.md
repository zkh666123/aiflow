# FlowAI Studio 全 Python 后端实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用一个公开 FastAPI 后端和一个独立 Python 沙箱进程替换 Go 控制面与 NestJS 后端，并保持现有 112 条公开路由、HTTP 包装、八节点 DSL 和 SSE 契约。

**Architecture:** `flowai-studio-ai-runtime` 先扩展为 FastAPI 单体，直接拥有 `control` 与 `ai` schema，并通过模块调用执行工作流与 AI 能力。完成前保留 NestJS 和 Go 作为行为参照；完成后将 Python 项目移动到 `flowai-studio-backend`，删除 Go、Prisma、内部 AI gRPC 和 Node 后端依赖。沙箱继续作为 loopback-only Python 进程。

**Tech Stack:** Python 3.13、FastAPI、Pydantic v2、SQLAlchemy 2、Alembic、PostgreSQL 16/pgvector、Redis、LangGraph、MCP Python SDK、uv、pytest。

**Verification order override:** 用户要求所有测试和质量检查在全部代码任务完成后统一执行。Task 1-8 只实施代码和提交，不运行 pytest、契约测试、lint、类型检查、构建或 E2E；Task 9 统一验证并修复。

---

### Task 1: FastAPI 公共后端与统一数据库基线

**Files:**
- Modify: `flowai-studio-ai-runtime/pyproject.toml`
- Modify: `flowai-studio-ai-runtime/src/aiflow_runtime/config.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/app.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/envelope.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/errors.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/infrastructure/database.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/infrastructure/redis.py`
- Create: `flowai-studio-ai-runtime/alembic/versions/0002_control_schema.py`
- Modify: `flowai-studio-ai-runtime/alembic/env.py`

- [ ] **Step 1: Add the public HTTP dependencies**

Add pinned `uvicorn`, `pyjwt`, `pwdlib[argon2]`, `httpx`, `python-multipart`, `orjson`, `langgraph`, `rank-bm25`, `pypdf`, `python-docx`, and `mcp` dependencies. Keep the sandbox gRPC client dependency until Task 8.

- [ ] **Step 2: Define runtime settings**

Expose `FLOWAI_HTTP_ADDR`, `FLOWAI_DATABASE_URL`, `FLOWAI_REDIS_URL`, `FLOWAI_FRONTEND_URL`, `FLOWAI_JWT_SECRET`, `FLOWAI_API_KEY_HMAC_SECRET`, provider keys, and sandbox address. The HTTP default is `127.0.0.1:3001`; public HTTP and sandbox addresses must validate independently.

- [ ] **Step 3: Add shared infrastructure**

Create one async SQLAlchemy engine/session factory and one Redis client during FastAPI lifespan. Request handlers obtain them through FastAPI dependencies and never create per-request connection pools.

- [ ] **Step 4: Add compatible HTTP behavior**

Implement request IDs, CORS for the configured frontend, strict Pydantic request bodies, panic/exception recovery, and the frozen envelope:

```python
{"success": True, "code": "SUCCESS", "message": "...", "data": value, "timestamp": iso8601}
```

- [ ] **Step 5: Move the control schema to Alembic**

Create `control.users`, `applications`, `teams`, `team_members`, `team_applications`, `api_keys`, and `app_shares` with the same UUIDs, constraints, indexes, and timestamps as Goose migration `00002_identity_access.sql`. Alembic becomes the only migration runner.

- [ ] **Step 6: Commit the foundation**

Commit only Task 1 files with `feat: establish Python public backend foundation`.

### Task 2: 用户、应用、团队、RBAC、API Key 与分享

**Files:**
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/identity/models.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/identity/schemas.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/identity/auth.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/identity/rbac.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/identity/service.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/users.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/applications.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/teams.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/api_keys.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/shares.py`
- Modify: `flowai-studio-ai-runtime/src/aiflow_runtime/app.py`

- [ ] **Step 1: Port authentication**

Implement registration, login, profile read/update, Argon2 password hashing, HS256 JWT, Redis login failure lock, bearer-token dependency, and the exact `/api/users/**` status codes.

- [ ] **Step 2: Port application ownership and status changes**

Implement application create/list/get/update/delete plus publish, unpublish, archive, and unarchive using the frozen response fields and authorization rules.

- [ ] **Step 3: Port three-level RBAC**

Implement global `admin/member`, team `owner/admin/editor/viewer`, and team application `full_access/can_edit/can_view`. Centralize permission evaluation so workflow routes can reuse it.

- [ ] **Step 4: Port teams and grants**

Implement team CRUD, member add/role/remove/leave, and team application add/permission/remove routes.

- [ ] **Step 5: Port API keys and sharing**

Generate `sk-` plus 64 lowercase hex characters, return plaintext only once, store HMAC-SHA256 digest and seven-character prefix, and implement list/toggle/revoke. Implement share create/read/update/revoke, public share lookup, embed response, and idempotent share generation.

- [ ] **Step 6: Commit identity and access**

Commit only Task 2 files with `feat: migrate identity and access to Python`.

### Task 3: 工作流数据、模板、版本与 Trace API

**Files:**
- Create: `flowai-studio-ai-runtime/alembic/versions/0003_workflow_schema.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/workflow/models.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/workflow/schemas.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/workflow/service.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/workflow/dsl.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/workflows.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/templates.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/versions.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/traces.py`

- [ ] **Step 1: Create workflow persistence**

Add workflows, templates, template ratings, workflow versions, executions, traces, and spans. Store node/edge/config snapshots as JSONB and index owner/application/status/timestamps.

- [ ] **Step 2: Implement workflow and DSL routes**

Implement workflow CRUD/by-app/running routes and DSL export/import/validate using DSL version `1.0`, kind `Workflow`, eight canonical node types, and the legacy `user-input` input alias.

- [ ] **Step 3: Implement templates**

Implement `/api/templates/**` and `/api/workflow/templates/**` aliases for CRUD, categories, publish, archive, import, and rating.

- [ ] **Step 4: Implement versions and traces**

Implement create/list/get/delete/compare/rollback for versions and trace get/by-workflow/slow/stats routes. Version diff reports node, edge, variable, and metadata changes.

- [ ] **Step 5: Commit workflow metadata APIs**

Commit only Task 3 files with `feat: add Python workflow metadata APIs`.

### Task 4: DAG 调度、节点执行与 SSE

**Files:**
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/workflow/graph.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/workflow/runtime.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/workflow/events.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/workflow/executors.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/workflow/state.py`
- Modify: `flowai-studio-ai-runtime/src/aiflow_runtime/api/workflows.py`

- [ ] **Step 1: Implement graph preparation**

Validate unique nodes, valid edges, one Start node, reachable Output nodes, and acyclicity. Build adjacency, reverse adjacency, and `runtime_in_degree` maps.

- [ ] **Step 2: Implement scheduling**

Use an async ready queue, wait for all active parents at joins, recursively prune non-selected condition branches, enforce node timeout and exponential retry, and stop scheduling after cancellation.

- [ ] **Step 3: Implement local node executors**

Execute Start, UserInput, Condition, and Output locally. Route LLM, RAG, Agent, and Skill to Task 5-7 service interfaces without gRPC.

- [ ] **Step 4: Implement execution state and cancellation**

Persist durable execution/trace summaries in PostgreSQL and runtime state/cancel flags in Redis. Implement `/run`, `/run/stream`, `/cancel/:executionId`, and `/running`.

- [ ] **Step 5: Emit compatible SSE**

Emit exactly one `workflow_start`, ordered `node_status`, zero or more `agent_trace`, periodic `heartbeat`, and exactly one `done` or `error`. Disconnecting the client sets cancellation.

- [ ] **Step 6: Commit executable workflows**

Commit only Task 4 files with `feat: execute workflows with Python DAG and SSE`.

### Task 5: 模型 Provider、聊天、Agent 与 Token 用量

**Files:**
- Create: `flowai-studio-ai-runtime/alembic/versions/0004_ai_runtime_schema.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/ai/providers.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/ai/chat.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/ai/agent.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/ai/token_usage.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/ai.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/models.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/token_usage.py`

- [ ] **Step 1: Add AI persistence**

Create chat sessions/messages and token usage records. Token usage accepts in-memory events and flushes at 100 records, every 10 seconds, and during shutdown.

- [ ] **Step 2: Implement provider routing**

Normalize OpenAI-compatible, Claude, Gemini, Qwen, and Ollama configuration behind one streaming interface. Route by requested model, capability, configured key, and health; expose model list/detail/health/cost/discovery routes.

- [ ] **Step 3: Implement chat endpoints**

Implement `/api/ai/run`, `/stream-run`, `/chat`, and chat history routes. Streaming returns the legacy SSE payload shape consumed by the frontend.

- [ ] **Step 4: Implement LangGraph Agent**

Implement bounded ReAct tool loops and Supervisor/Worker delegation. Emit thinking, tool call/result, retrieval, delegation, worker result, final answer, and error trace entries.

- [ ] **Step 5: Implement token reports**

Implement usage list, cost report, and model ranking endpoints from persisted usage.

- [ ] **Step 6: Commit AI execution**

Commit only Task 5 files with `feat: add Python providers agents and usage`.

### Task 6: RAG、文档与混合检索

**Files:**
- Create: `flowai-studio-ai-runtime/alembic/versions/0005_rag_schema.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/ai/documents.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/ai/embeddings.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/ai/retrieval.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/rag.py`

- [ ] **Step 1: Create RAG persistence**

Add knowledge bases, documents, chunks, vector embeddings, FTS indexes, and ingestion status/error fields under `ai` schema.

- [ ] **Step 2: Implement knowledge base and document routes**

Implement knowledge base CRUD, multipart upload, document delete, and chunk listing. Parse TXT, Markdown, PDF, and DOCX; chunk and index in a bounded background task.

- [ ] **Step 3: Implement hybrid retrieval**

Collect pgvector and PostgreSQL FTS candidates, apply Python BM25, fuse rankings with weighted reciprocal rank fusion, and optionally rerank with Cohere or Ollama. Reranker failures return the fused result.

- [ ] **Step 4: Connect RAG workflow nodes**

Return source metadata, scores, and content through the workflow RAG executor and `/api/rag/retrieve`.

- [ ] **Step 5: Commit RAG**

Commit only Task 6 files with `feat: add Python document ingestion and hybrid RAG`.

### Task 7: MCP、Skill、沙箱与运行控制

**Files:**
- Create: `flowai-studio-ai-runtime/alembic/versions/0006_tools_schema.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/ai/mcp.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/ai/skills.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/infrastructure/limits.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/infrastructure/cache.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/mcp.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/skills.py`
- Create: `flowai-studio-ai-runtime/src/aiflow_runtime/api/operations.py`

- [ ] **Step 1: Implement MCP management**

Persist MCP server configuration, keep stdio/HTTP sessions in the Python process, cache connection/tool catalogs in Redis, and implement server CRUD/connect/disconnect/discover/call routes.

- [ ] **Step 2: Implement skills**

Implement builtin list, custom Skill CRUD, and execution. Calculator uses an AST allowlist; Python code execution calls the independent sandbox service with time/output limits.

- [ ] **Step 3: Implement rate and concurrency controls**

Use Redis Lua for token bucket and execution lease acquire/release. Implement closed/open/half-open circuit breaker state and the frozen rate-limit inspection/reset routes.

- [ ] **Step 4: Implement cache and health**

Implement bounded L1 LRU plus L2 Redis with null values, TTL jitter, and Redis mutex. Health reports PostgreSQL, pgvector, Redis, provider, and sandbox state plus cache stats.

- [ ] **Step 5: Commit tools and controls**

Commit only Task 7 files with `feat: add Python tools sandbox and runtime controls`.

### Task 8: 启动切换、目录收敛与旧后端删除

**Files:**
- Modify: `scripts/native/start-services.ps1`
- Modify: `scripts/native/stop-services.ps1`
- Modify: `scripts/native/check-environment.ps1`
- Modify: `scripts/native/check-services.ps1`
- Modify: `scripts/native/initialize-database.ps1`
- Modify: `flowai-studio-frontend/vite.config.ts`
- Modify: `README.md`
- Delete: `flowai-studio-control-plane/`
- Delete: existing NestJS/Prisma contents under `flowai-studio-backend/`
- Move: completed Python backend from `flowai-studio-ai-runtime/` to `flowai-studio-backend/`
- Remove: obsolete AI gRPC server files and control-plane protobuf generation

- [ ] **Step 1: Switch native lifecycle scripts**

Run Alembic once, start sandbox and FastAPI only, wait for ports 50052 and 3001, and remove Go/sqlc/goose/Buf requirements that no remaining runtime or sandbox interface uses.

- [ ] **Step 2: Switch frontend default backend**

Keep the frontend proxy at `/api` -> `127.0.0.1:3001`; remove references that specifically expect NestJS or the Go health payload.

- [ ] **Step 3: Remove obsolete backends**

Delete Go control plane, Go toolchain entries, Node backend source/package/Prisma migrations, and internal AI gRPC service code. Preserve the frozen contracts and sandbox protocol needed by the Python backend.

- [ ] **Step 4: Update documentation**

Document Python 3.13, uv, PostgreSQL/pgvector, Redis, native start/stop commands, directory layout, environment variables, and the all-Python architecture.

- [ ] **Step 5: Commit cutover**

Commit Task 8 with `refactor: cut over FlowAI Studio to Python backend` while explicitly excluding unrelated user-owned frontend changes unless they are required by the cutover.

### Task 9: 统一测试、质量检查、运行验收与修复

**Files:**
- Create/modify focused tests under `flowai-studio-backend/tests/`
- Modify implementation files only when a failing check proves a defect
- Update: `docs/superpowers/plans/2026-07-16-python-backend-consolidation-implementation.md`

- [ ] **Step 1: Install locked dependencies and migrate a clean database**

Run `uv sync`, Alembic upgrade, and verify PostgreSQL schemas/extensions plus Redis connectivity.

- [ ] **Step 2: Run static checks**

Run Python compile/import checks, formatter check, lint, and type checking; fix every implementation defect without unrelated cleanup.

- [ ] **Step 3: Run unit and contract tests**

Cover authentication/RBAC, API keys, graph joins/branches/cycles/retry/timeout/cancel, SSE ordering, provider routing, Agent loops, BM25/RRF, document parsing, MCP, token batching, rate controls, cache, and all 112 frozen routes/envelopes.

- [ ] **Step 4: Run native integration checks**

Start PostgreSQL/pgvector, Redis, sandbox, FastAPI, and frontend without Docker. Exercise register/login, app/team/API key/share, workflow CRUD/run/SSE/cancel, RAG upload/retrieve, Agent tool call, version rollback, Trace display, and permission denial.

- [ ] **Step 5: Run frontend E2E**

Verify login, eight-node editing, streaming execution, RAG, Agent, versions, Trace, and teams in a real browser. Confirm no request targets Go or NestJS.

- [ ] **Step 6: Audit final deletion and documentation**

Prove no Go source/module, Prisma schema/migration, NestJS package, Node backend image, internal AI gRPC endpoint, or Docker requirement remains. Confirm user-owned staged frontend work is preserved or intentionally incorporated.

- [ ] **Step 7: Commit verification fixes and completion record**

Commit focused fixes, record exact commands/results, and mark every evidence-backed checkbox complete.

