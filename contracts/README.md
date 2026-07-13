# FlowAI Studio 迁移契约基线

本目录是 Go 控制面和 Python AI Runtime 迁移期间的可执行契约来源。它记录当前工作树中 NestJS Controller、React API 调用、工作流节点、DSL、响应包装和 SSE 行为，并对现有代码冲突给出明确裁决。

## 基线来源

契约提取以当前工作树为准，包括尚未提交但已暂存的用户修改，不只读取 `main` 历史提交。

冲突按以下优先级处理：

1. 已批准迁移设计和验收标准。
2. 前端现有功能能够正常运行所需的行为。
3. NestJS 公开 Controller 的路径、状态码和响应行为。
4. DTO 与前端 TypeScript 类型。
5. 内部实现和注释。

总体裁决见 [Go + Python 后端迁移设计](../docs/superpowers/specs/2026-07-13-go-python-backend-migration-design.md)。

## 文件分类

自动生成文件：

- `http/routes.json`：NestJS Controller 的 HTTP 方法、完整路径、Guard 和源码位置。
- `http/frontend-calls.json`：React/TypeScript 中通过 `request` 或 `fetch` 发起的 API 调用。
- `http/compatibility-gaps.json`：前端调用中找不到同方法、同规范化路径 Controller 的条目。

人工审核文件：

- `http/known-gaps.json`：对现有差异的迁移裁决。
- `http/response-envelope.schema.json`：统一成功/错误响应包装。
- `workflow/*.schema.json`：8 类节点、工作流 JSON 和 DSL 1.0。
- `workflow/fixtures/`：规范工作流示例。
- `sse/events.schema.json`：6 类 SSE 事件及载荷。
- `sse/valid-*-sequence.json`：成功与失败的规范事件顺序。

生成文件不得手工修改。源代码发生有意契约变化时，先修改或补充测试，再重新生成并审核差异。

## 当前统计

- NestJS 公开路由：112
- 前端 API 调用位置：93
- 原始兼容差异：11
- 已审核差异规则：2
- 规范工作流节点类型：8
- SSE 事件类型：6

11 条原始差异由两类问题构成：

- 前端使用 `/api/workflow/templates/**`，Controller 使用 `/api/templates/**`。Go 同时支持兼容别名和规范路径。
- 前端读取 `GET /api/apps/:appId/share`，现有 Controller 只有创建、更新和删除。Go 补齐 GET。

## 命令

运行全部契约测试：

```powershell
node --test scripts/contracts/*.test.cjs
```

根据当前源码重新生成清单：

```powershell
node scripts/contracts/generate-contracts.cjs
```

检查生成文件漂移、差异覆盖、节点/DSL 和 SSE 核心不变量：

```powershell
node scripts/contracts/check-contracts.cjs
```

标准验证顺序：

```powershell
node --test scripts/contracts/*.test.cjs
node scripts/contracts/generate-contracts.cjs
node scripts/contracts/check-contracts.cjs
```

## 旧系统构建状态

记录契约时，旧系统本身不是全绿基线：

- `npm run build`（NestJS）报告 118 个 TypeScript 错误，主要来自过期 Prisma Client、缺少 `ioredis` 安装结果和已有类型问题。
- Jest 共 17 个套件，7 个通过，10 个因编译错误未运行；实际执行的 96 个断言通过。
- React 构建因缺少图表包和 `AgentTraceStep` 定义失败。

这些错误用于解释当前状态，不能复制成新后端行为。新实现以本目录的规范契约和迁移验收标准为目标。

## 迁移使用方式

每个 Go 路由完成后，应以 `http/routes.json` 中对应条目建立兼容测试，并记录该路由的迁移所有者和通过状态。涉及 AI、RAG、Agent、Skill 的 Handler 还必须验证 Go 到 Python 的 gRPC 错误映射与取消传播。

NestJS 只能在全部路由、SSE、Docker 集成和前端 E2E 验收完成后删除。删除旧后端前，需要把 AST 提取工具对 TypeScript 的依赖迁移到独立工具锁定文件，避免继续依赖 `flowai-studio-backend/node_modules`。
