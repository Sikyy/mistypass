# 内部开发 Wiki（Internal）

当前能力状态：

- `CONTRACT_READY`：内部 Wiki 结构已覆盖“系统总览、模块职责、数据与状态、开发与交付流程”。
- `PROD_READY`：内容已按当前主干代码组织（`api/internal/modules/*`、`web-admin/src/pages/*`）同步。

## 1. 阅读顺序（建议）

1. `docs/wiki/internal/system-overview.md`
2. `docs/wiki/internal/module-reference.md`
3. `docs/wiki/internal/priority-board.md`
4. `docs/wiki/internal/dev-workflow.md`

## 2. 你能在这里找到什么

- 项目拆分方式：Cloud API / Admin UI / 状态存储 / 回归脚本。
- 每个模块的职责边界、关键接口、依赖关系与测试入口。
- 新功能落地时，应该改哪里、如何验证、如何更新文档。

## 3. 不在这里的内容

- 详细协议草案、企业接入设计、专项实验报告等仍在专题目录：
  - `docs/enterprise/*`
  - `docs/architecture/*`
  - `docs/testing/*`
