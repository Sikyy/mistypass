# MistyPass 项目 Wiki（内部 + 外部）

当前能力状态：

- `CONTRACT_READY`：本目录已提供内部开发 Wiki 与对外 API 文档，可直接作为团队协作与对接基线。
- `PROD_READY`：已对齐当前主干代码与回归口径。

## 1. 使用方式

- 内部研发：`docs/wiki/internal/README.md`
- 对外 API 文档：`docs/wiki/external-api/README.md`

快速入口：

| 用途 | 文档 |
|------|------|
| 优先级看板 | `docs/wiki/internal/priority-board.md` |
| 模块参考手册 | `docs/wiki/internal/module-reference.md` |
| 对外 Quick Start | `docs/wiki/external-api/getting-started.md` |
| 对外 Enterprise SSO/JIT | `docs/wiki/external-api/guides-enterprise-sso-jit.md` |
| 对外 Wallet Queue Ops | `docs/wiki/external-api/guides-wallet-queue-ops.md` |
| 对外 Changelog | `docs/wiki/external-api/changelog-and-migration.md` |

## 2. 文档边界

- 本目录不替代现有专题文档（`docs/enterprise/*`、`docs/testing/*`、`docs/architecture/*`）。
- 本目录负责"索引 + 可执行说明 + 模板化规范"，让新同学能快速定位到正确文档与代码。

## 3. 维护约定

- 代码结构或接口分组有新增时，同步更新本目录对应页。
- 对外 API 文档发布前，先走内部评审，再从 `external-api` 骨架生成正式发布版本。
