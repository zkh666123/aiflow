# FlowAI Studio

FlowAI Studio 是基于 React 与 Python 的可视化 AI 工作流编排平台。前端通过统一 `/api/**` 和 SSE 接口访问 FastAPI 后端；后端负责应用、团队、RBAC、工作流 DAG、版本、Trace、模型 Provider、Agent、RAG、MCP、Token 用量和运行控制。

## 技术栈

- 前端：React 18、TypeScript、Vite、Zustand、Ant Design、React Flow。
- 后端：Python 3.13、FastAPI、Pydantic v2、SQLAlchemy 2、Alembic、LangGraph。
- 数据：PostgreSQL 16、pgvector、Redis。
- 沙箱：独立 Python gRPC 进程，通过受限 WASI CPython 执行代码；Web 进程不直接执行用户命令。

本地环境不使用 Docker。PostgreSQL/pgvector 与 Redis 运行在 WSL，FastAPI、前端和沙箱运行在 Windows。

## 目录

```text
flowai-studio-frontend/   React 前端
flowai-studio-backend/    唯一公开 FastAPI 后端
flowai-studio-sandbox/    独立 Python 代码沙箱
proto/                    沙箱 gRPC 契约和 Python 生成代码
contracts/                HTTP、工作流 DSL 与 SSE 兼容基线
scripts/native/           Windows 原生启动、停止和检查脚本
```

## 环境准备

需要安装 Python 3.13、uv 和 WSL。WSL 内需要 PostgreSQL 16、pgvector 与 Redis。

首次初始化：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/initialize-database.ps1
```

脚本会创建忽略提交的 `.env.native`、两个 PostgreSQL schema，并执行全部 Alembic migration。

## 启动

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/start-services.ps1
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/check-services.ps1
```

服务地址：

- FastAPI：`http://127.0.0.1:3001`
- OpenAPI：`http://127.0.0.1:3001/api/docs`
- Sandbox gRPC：`127.0.0.1:50052`
- 前端：`http://127.0.0.1:5173`

启动前端：

```powershell
cd flowai-studio-frontend
npm install
npm run dev
```

停止后端和沙箱：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/native/stop-services.ps1
```

## 环境变量

核心变量由初始化脚本写入 `.env.native`：

- `FLOWAI_HTTP_ADDR`
- `FLOWAI_DATABASE_URL`
- `FLOWAI_MIGRATION_DATABASE_URL`
- `FLOWAI_REDIS_URL`
- `FLOWAI_JWT_SECRET`
- `FLOWAI_API_KEY_HMAC_SECRET`
- `FLOWAI_SANDBOX_GRPC_ADDR`
- `FLOWAI_GRPC_TOKEN`

模型 Provider 可按需配置 `OPENAI_API_KEY`、`ANTHROPIC_API_KEY`、`GEMINI_API_KEY`、`DASHSCOPE_API_KEY` 和 `OLLAMA_BASE_URL`。

## 工作流

平台固定支持八类节点：Start、UserInput、LLM、RAG、Agent、Skill、Condition、Output。平台 DAG 由 Python 调度器执行，LangGraph 只用于 Agent 内部流程。SSE 事件保持 `workflow_start`、`node_status`、`agent_trace`、`heartbeat`、`done`、`error`。

## 开源协议

小圆项目，禁止商业用途。
