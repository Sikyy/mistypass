# Release Process Template（对外 API 发布流程模板）

当前能力状态：

- `CONTRACT_READY`：已形成可执行的对外 API 发布流程模板（评审、版本、回滚、留档）。
- `PROD_READY`：模板字段已对齐当前 Wiki 结构与回归口径，可直接用于内部发布评审。

## 1. 适用范围

适用于所有会影响外部集成方的 API 变更，包括：

- 新端点/新字段发布
- 参数约束调整
- 默认行为变更
- 弃用标识与下线计划

## 2. 版本与变更分级

建议版本标识：`YYYY-MM-DD-rN`（例如 `2026-04-15-r9`）。

- `PATCH`：文档示例与解释补充，不改接口行为。
- `MINOR`：向后兼容新增（端点、可选字段、可选参数）。
- `MAJOR`：潜在破坏性变更（删除、收紧校验、默认语义变化）。

## 3. 发布前检查清单（Pre-Release Checklist）

1. 合同变更已记录在 `changelog-and-migration.md`。
2. 受影响 Guide 与 Reference 已同步更新。
3. 错误语义与重试策略已在 `errors-and-reliability.md` 对齐。
4. 对应回归脚本已通过（至少覆盖变更域主链路）。
5. 回滚路径已明确（开关、旧路径、数据兼容）。
6. 涉及弃用时，已标注 `deprecated_since/sunset_at/replacement`。

## 4. 评审模板（复制即用）

```markdown
# API Release Review - <version>

## 1) Release 基本信息
- Version: <YYYY-MM-DD-rN>
- Change Type: <PATCH|MINOR|MAJOR>
- Owner: <name>
- Planned Window: <start/end>

## 2) 变更范围
- Affected Endpoints:
  - <method path>
- Contract Delta:
  - Added:
  - Changed:
  - Removed:

## 3) 文档与迁移
- Updated Docs:
  - <path>
- Migration Notes:
  - <summary>

## 4) 验证证据
- Test Scripts:
  - <script> -> PASS
- go test ./... -> PASS/NA
- Doc Marker Guard -> PASS/NA

## 5) 风险与回滚
- Risks:
  - <risk>
- Rollback Trigger:
  - <condition>
- Rollback Action:
  - <steps>

## 6) 审批
- API Owner: <approve/reject>
- QA/Integrator: <approve/reject>
- Ops/SRE: <approve/reject>
```

## 5. 发布执行步骤（Runbook）

1. 锁定版本号并更新 `changelog-and-migration.md`。
2. 合并文档与代码变更，执行：
   - `./docs/testing/check-doc-capability-markers.zsh`
   - `cd api && go test ./...`
3. 对变更域执行关键脚本并留存输出。
4. 发布窗口内观察错误率与关键端点成功率。
5. 发布后补齐变更审计记录与对外通知摘要。

## 6. 回滚模板

触发条件（示例）：

- 关键端点连续 15 分钟错误率异常升高。
- 集成方出现批量不可恢复 `4xx/5xx`。

回滚动作（示例）：

1. 关闭新行为开关或回退到上一版本路径。
2. 保持旧合同路径可用，禁止继续扩散。
3. 在 changelog 追加回滚记录（时间、原因、影响范围）。
4. 发布修复窗口与再次验证计划。

## 7. 相关文档

- `docs/wiki/external-api/changelog-and-migration.md`
- `docs/wiki/external-api/deprecation-policy.md`
- `docs/wiki/external-api/errors-and-reliability.md`
