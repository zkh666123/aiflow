# FlowAI Studio 契约基线

本目录保存全 Python 后端必须兼容的公开 HTTP、工作流 DSL、响应包装和 SSE 契约。

## 契约来源

- `http/routes.json`：切换到 Python 前冻结的 112 条公开 API 基线。FastAPI 可以增加兼容别名，但不能缺少这些路由。
- `http/frontend-calls.json`：从当前 React/TypeScript 源码提取的 API 调用。
- `http/compatibility-gaps.json`：前端使用、但不在冻结基线中的调用。
- `http/known-gaps.json`：已经由 Python 后端补齐的兼容决策。
- `http/response-envelope.schema.json`：统一响应 `{success, code, message, data, timestamp}`。
- `workflow/`：8 类节点、工作流 JSON 和 DSL 1.0。
- `sse/`：`workflow_start`、`node_status`、`agent_trace`、`heartbeat`、`done`、`error` 事件及有效顺序。

冻结路由由 `flowai-studio-backend/tests/test_contracts.py` 对照实际 FastAPI 路由验证。Node 契约工具只重新提取前端调用，不再依赖已删除的 NestJS 源码。

## 命令

```powershell
node --test scripts/contracts/*.test.cjs
node scripts/contracts/generate-contracts.cjs
node scripts/contracts/check-contracts.cjs
```

本项目的公开后端只有 `flowai-studio-backend/`。本地运行使用 Windows Python 进程以及 WSL 中的 PostgreSQL/pgvector、Redis，不使用 Docker。
