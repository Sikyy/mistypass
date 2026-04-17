# 开发与交付流程（内部）

当前能力状态：

- `PROD_READY`：流程已对齐当前 CI 与回归脚本，适合作为团队默认开发路径。
- `CONTRACT_READY`：新需求可按模板增量推进，不需要先重构主干。

## 1. 本地开发最小路径

1. 启动 API

```bash
cd api
go run ./cmd/api
```

2. 启动管理台

```bash
cd web-admin
npm install
npm run dev
```

3. PostgreSQL 模式（推荐）

```bash
cd api
DATABASE_URL='postgres://user:pass@localhost:5432/postgres?sslmode=disable' go run ./cmd/api
```

## 2. 需求落地建议顺序

0. 先确认当前优先级事项：`docs/wiki/internal/priority-board.md`。
1. 先定义合同边界（请求/响应、错误码、审计 action）。
2. 在 module service 实现领域逻辑。
3. 在 router 完成参数校验和错误映射。
4. 补单测（`go test ./...`）。
5. 补回归脚本（`docs/testing/*.zsh`）。
6. 更新文档（roadmap + API map + README/wiki）。

## 3. 质量门（合入前）

- 必跑：
  - `go test ./...`
  - `docs/testing/check-doc-capability-markers.zsh`
- 变更涉及模块时，追加对应脚本：
  - 网关：`curl-gateway-event-*.zsh`
  - 企业：`curl-enterprise-*.zsh`
  - Wallet：`curl-wallet-job-*.zsh`
  - 回放：`curl-pg-replay-*.zsh`

## 4. 常见风险与处理

- 风险：只改 router，不改 module，逻辑散落。
  - 处理：保持 router 薄层，复杂逻辑下沉 module。
- 风险：仅改代码不补脚本，回归盲区扩大。
  - 处理：每个合同变更至少补一条脚本或路由级测试。
- 风险：文档状态落后于代码。
  - 处理：合并前同步 roadmap 与 API map。

## 5. 版本演进建议

- 对外合同优先“向后兼容 + 增量字段”。
- 高风险主链路（gateway/checkpoint/replay/enterprise callback）避免大规模重写。
- 新能力优先在 control-plane 侧分边界增量接入。
