# Deprecation Policy（对外 API 弃用策略）

当前能力状态：

- `CONTRACT_READY`：已形成统一的弃用标注、迁移窗口与下线口径。
- `PROD_READY`：策略字段与当前 changelog/release 模板一致，可直接用于发布评审与对外通知。

## 1. 适用范围

适用于所有对外合同变更中的“弃用但暂未移除”场景，包括：

- 字段弃用（request/response）
- 端点弃用
- 参数语义弃用（例如默认值即将调整）

## 2. 弃用状态定义

1. `deprecated`：已发布替代方案，旧能力仍可用。
2. `sunset_scheduled`：已确认下线时间窗口，开始倒计时。
3. `removed`：旧能力已移除，不再提供兼容。

说明：

- 对外文档中，`deprecated` 与 `sunset_scheduled` 必须同时给出替代方案与迁移步骤。
- `removed` 只在到达下线窗口后写入 changelog，不允许提前标注为已移除。

## 3. 必填元数据

每条弃用项至少包含以下字段：

- `deprecated_since`：首次标注弃用的版本（例如 `2026-04-15-r9`）
- `sunset_at`：计划下线日期（建议 `YYYY-MM-DD`）
- `replacement`：替代端点/字段
- `migration_note`：迁移动作摘要（1-3 条）
- `owner`：责任人或责任小组

## 4. 推荐时间窗口

1. `MINOR` 级兼容调整：建议至少 `30` 天迁移窗口。
2. 影响高频主链路的变更：建议至少 `60` 天迁移窗口。
3. `MAJOR` 级移除：除紧急安全修复外，建议至少 `90` 天并提供双轨观察期。

## 5. 文档标注模板（复制即用）

```markdown
## Deprecation Notice
- Status: deprecated
- Deprecated Since: 2026-04-15-r9
- Sunset At: 2026-06-30
- Replacement: `POST /api/v1/example/new-endpoint`
- Migration Note:
  1. 将请求字段 `legacy_x` 替换为 `new_x`
  2. 保留旧调用链路 60 天
  3. 完成回归后移除旧参数
```

## 6. 发布与下线执行清单

1. 在对应 Guide/Reference 页面新增弃用标注。
2. 在 `changelog-and-migration.md` 追加变更记录。
3. 在 `release-process-template.md` 的评审模板中记录风险与回滚口径。
4. 对受影响端点执行最小回归脚本并留档。
5. 到达 `sunset_at` 后再执行“已移除”标注与行为变更。

## 7. 回滚与例外

- 若迁移期间错误率异常升高，可延后 `sunset_at` 并在 changelog 记录原因。
- 安全漏洞类紧急处置可缩短窗口，但必须补充安全公告与替代路径。

## 8. 相关文档

- `docs/wiki/external-api/changelog-and-migration.md`
- `docs/wiki/external-api/release-process-template.md`
- `docs/wiki/external-api/errors-and-reliability.md`
