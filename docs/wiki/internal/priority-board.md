# 优先级看板（内部开发）

> 本页为 `docs/development-status-roadmap.md` 的精简视图，用于日常排期同步。
> 详细子项拆解、已完成/未完成清单与风险说明请查阅 roadmap 原文。

当前能力状态：

- `CONTRACT_READY`：本页口径与 roadmap 一致。
- `PROD_READY`：路线按 PRD v0.2 冻结为"Cloud Access SaaS + Edge Controller（2D/4D）+ Reader 兼容层"。

## 1. 当前高优先事项

### 后端（Cloud/API + Edge）

| 优先级 | 编号 | 事项 | 紧急度 | 进度 | 当前推进焦点 |
|---:|---|---|---|---:|---|
| 1 | R2 | PostgreSQL 增量写入与稳定回放 | `S0` | 99% | 持续积累 nightly 数据并完成 `>=7` 天签字 |
| 2 | R1 | 网关离线可运行闭环 | `S0` | 98% | `queue_hint/retry_subset/checkpoint` 合同回归守护 |
| 3 | R8 | Controller MVP 台架验证（2D/4D） | `S1` | 58% | 单门 I/O 闭环 + `Wiegand/OSDP` 参数基线 + 实体证据留档 |
| 4 | R6 | 协议层强化（legacy→OSDP 升级路径） | `S1` | 68% | `WG-Branch-A` A2/A3 本地放行与门态链路收口 |
| 5 | R9 | Encore 渐进迁移试点 | `S1` | 89% | 决策已冻结（保持 Go+Chi），详见 `docs/architecture/encore-decision-report-2026-04-19.md` |
| 6 | R3 | Wallet 队列/重试/DLQ/可观测 | `S1` | 97% | 非 Meta 真通道路径持续守护（mock/resend） |

### 前端（Web Admin）

| 优先级 | 编号 | 事项 | 紧急度 | 进度 | 当前推进焦点 |
|---:|---|---|---|---:|---|
| 1 | F7 | 构建回归、页面验证与性能守护 | `S1` | 100% | 分支级集中验证：`build + smoke + role-boundary + browser e2e + doc-marker` |
| 2 | F1/F4/F5/F6 | 企业态主路径已收口能力守卫 | `S1` | 100% | 回归守卫，避免角色边界和信息架构回退 |

### 外部依赖项（不纳入执行顺序）

- `R3` Meta 企业号真实通道联调
- `R7` Google Wallet API（LEI 条件）
- `F2` HRIS/SCIM 上游返回语义校验

## 2. 推进顺序（无外部依赖）

1. 后端 S0 主链路：`R2 + R1`
2. 边端主链路：`R8 + R6`
3. 前端守护：`F7`

## 3. 建议脚本（按变更域）

### 前端

- `cd web-admin && npm run build`
- Playwright E2E：`web-admin/e2e/*.spec.ts`

### 后端

- `cd api && go test ./...`
- `docs/testing/curl-gateway-*.zsh`
- `docs/testing/curl-pg-replay-*.zsh`
- `docs/testing/curl-wallet-job-*.zsh`

## 4. 每日同步最小清单

1. 查看 `docs/development-status-roadmap.md` 与 `docs/web-admin-enterprise-refactor-priority.md` 是否有状态变更。
2. 运行与当前变更域匹配的 `docs/testing/*.zsh` 与构建命令。
3. 若接口契约变化，同步更新：
   - `docs/testing/admin-ui-test-and-api-map.md`
   - `docs/wiki/internal/*`
   - `docs/wiki/external-api/*`（若对外合同受影响）

## 5. 快速命令

```bash
# 文档状态标识守卫
./docs/testing/check-doc-capability-markers.zsh

# 后端单测
cd api && go test ./...

# 前端构建
cd web-admin && npm run build
```
