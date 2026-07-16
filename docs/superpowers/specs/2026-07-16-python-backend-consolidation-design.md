# FlowAI Studio 全 Python 后端收敛设计

## 1. 目标

将尚未完成的“Go 控制面 + Python AI 执行面”迁移收敛为全 Python 后端。React 前端只访问 FastAPI；PostgreSQL/pgvector、Redis 和现有公开 HTTP/SSE 契约保持不变；不保留旧数据，也不使用 Docker。

## 2. 已选择方案

采用一个公开 FastAPI 应用加一个独立 Python 沙箱进程：

```text
React -> FastAPI -> PostgreSQL/pgvector
                 -> Redis
                 -> Python sandbox worker
                 -> Model/MCP providers
```

FastAPI 进程同时承担用户、团队、应用、权限、工作流、版本、执行、Trace、RAG、Agent、MCP 和 Token 统计。沙箱进程只执行不可信 Python 代码，不承担业务 API。

不选择以下方案：

- 继续 Go + Python：保留两套语言、部署和内部协议，剩余实现成本最高。
- Python 微服务化：当前没有多实例或独立扩缩容需求，会增加进程、鉴权和故障处理成本。
- Python 单体保留控制面到 AI Runtime 的 gRPC：同进程模块调用即可，gRPC 只在独立沙箱确有隔离价值时保留。

## 3. 代码与进程边界

`flowai-studio-ai-runtime/` 扩展为唯一公开后端，后续更名为 `flowai-studio-backend/`。内部按领域组织：

- `api/`：FastAPI 路由、JWT/API Key 认证、响应包装、SSE。
- `identity/`：用户、团队、应用、分享、RBAC。
- `workflow/`：DSL、版本、DAG 调度、执行状态、Trace。
- `ai/`：Provider、Agent、RAG、文档、MCP、Token 用量。
- `infrastructure/`：SQLAlchemy、Redis、缓存、限流和配置。

`flowai-studio-sandbox/` 保持独立进程和 loopback-only 地址。Web 主进程不得直接 `exec`、`eval` 或执行用户命令。

## 4. 数据边界

Python 独占 PostgreSQL 的 `control` 与 `ai` schema。现有 `control` 表结构直接转为 Alembic 基线，不编写 Goose/Prisma 数据迁移程序。新增工作流、版本、执行、Trace、知识库、文档、分块、MCP 和 Token 表由 Alembic 创建。

业务事务不得跨 Redis 和 PostgreSQL 伪装成原子事务。PostgreSQL 保存最终状态，Redis 只保存缓存、运行中状态、取消标志、限流令牌和短期锁。

## 5. 公开兼容性

- 保持现有 `/api/**` 路径、字段命名和 HTTP 状态码。
- 保持 `{success, code, message, data, timestamp}` 响应包装。
- 保持 `workflow_start`、`node_status`、`agent_trace`、`heartbeat`、`done`、`error` SSE 事件及终止事件唯一性。
- 固定八类节点：Start、UserInput、LLM、RAG、Agent、Skill、Condition、Output。
- 旧 NestJS 在切换验收前只作为行为参照，不接收新功能。

## 6. 工作流执行

平台 DAG 由 Python 自研调度器执行，继续使用拓扑入度、汇合节点等待、条件分支递归剪枝、超时、重试、取消和心跳。LangGraph 只用于 Agent 内部 ReAct 与 Supervisor/Worker，不替代平台 DAG。

Start、UserInput、Condition、Output 在工作流模块内执行；LLM、RAG、Agent、Skill 调用同进程 AI 模块。代码类 Skill 通过沙箱进程运行。

## 7. 错误和运行状态

HTTP 异常统一映射到冻结的错误包装。SSE 建立后不再改变 HTTP 状态码，运行错误使用唯一 `error` 终止事件。客户端断开、显式取消和节点超时写入 Redis 取消标志，并传递到 Provider、MCP 和沙箱调用。

## 8. 迁移顺序

1. 将 `control` schema 和现有 35 条 Go 路由迁移到 FastAPI。
2. 实现工作流、模板、版本、Trace、DAG 和 SSE 纵向链路。
3. 实现 Provider、RAG、Agent、MCP、文档和 Token 用量。
4. 接入 Python 沙箱，切换前端默认后端。
5. 删除 Go 控制面、Go 工具链、内部 AI gRPC 和旧 NestJS/Prisma。
6. 所有代码任务完成后，统一执行契约、单元、集成、前端 E2E、lint、类型检查和运行验收。

## 9. 完成标准

- 前端无需删除功能即可通过 FastAPI 完成核心流程。
- 默认启动脚本不构建或启动 Go/NestJS，不依赖 Docker。
- 仓库最终后端语言只有 Python，前端保持 React/TypeScript。
- 所有冻结公开路由、响应包装、工作流 DSL 和 SSE 契约通过统一验收。
- PostgreSQL/pgvector、Redis、FastAPI 和沙箱可由本机启动脚本拉起并报告健康状态。
