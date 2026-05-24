# Mistyislet 邮件接入方案：Workspace / Cloudflare / Resend

> 日期：2026-05-24
> 能力状态：CONTRACT_READY
> 范围：后台报表发送、Wallet / Enterprise 告警、邀请邮件、未来收信回执。

## 1. 结论

如果你现在已经有 Google Workspace，推荐保留 Workspace 负责真人邮箱与域名管理，把应用系统的事务邮件继续交给 Resend 这类 API 邮件服务；Cloudflare 适合承担 DNS、DMARC、邮件路由、收信转发、Workers 入站处理，也可以作为后续发送通道候选，但不建议把第一阶段的应用发信直接压到 Workspace SMTP。

本项目当前代码已经具备 Resend 路径：

- 统一发送抽象：`api/internal/mail` 提供 `Provider` / `Message` / `Receipt`，当前最小实现为 Resend。
- 报表发送：`api/internal/http/routes_report_schedule.go` 复用统一 mail provider，并继续兼容 `USER_INVITATION_RESEND_API_KEY` / `USER_INVITATION_RESEND_ENDPOINT`。
- Web Admin：Report Schedules 页面已补 “Send now” 行操作，provider 未启用/未配置会直接显示后端错误。
- Wallet / Enterprise 告警：Wallet email sender 已通过统一 Resend provider 发送，Enterprise worker alerts 复用 Wallet 多通道 dispatch；`spaceemail` 仍映射到 `resend` 兼容模式。
- 当前缺口不是“没有邮件能力”，而是缺少域名 DNS 记录清单、真实生产 key、真实收信/回执 webhook。

## 2. 三种方案对比

| 方案 | 适合做什么 | 不适合做什么 | 推荐度 |
|---|---|---|---|
| Google Workspace SMTP / Gmail API | 少量内部通知、人工邮箱、临时 MVP | 高频事务邮件、报表附件批量发送、可观测回执 | 低 |
| Resend / Postmark / SES | 应用事务邮件、附件、模板、回执、退信 | 真人邮箱收发 | 高 |
| Cloudflare Email Routing / Workers / Email Service | 收信路由、别名、Worker 入站解析、未来 Worker 发信 | 直接替代完整 mailbox，或绕过后端审计链 | 中高 |

官方参考：

- Cloudflare Email Routing: https://developers.cloudflare.com/email-routing/
- Cloudflare Email Workers: https://developers.cloudflare.com/email-routing/email-workers/
- Cloudflare Email Service: https://developers.cloudflare.com/email-service/
- Google Workspace SMTP relay: https://support.google.com/a/answer/2956491
- Gmail API sending guide: https://developers.google.com/workspace/gmail/api/guides/sending

## 3. 推荐落地路径

### Phase E0：域名与发信基线（0.5 天）

目标：让 `mistyislet.com` 的人用邮箱和系统发件身份分清楚。

建议：

- Google Workspace 管理真人邮箱：`hello@mistyislet.com`、`support@mistyislet.com`、`admin@mistyislet.com`。
- 应用发信使用专用身份：`reports@mistyislet.com`、`no-reply@mistyislet.com` 或子域 `mail.mistyislet.com`。
- DNS 统一放 Cloudflare：SPF、DKIM、DMARC 都在 Cloudflare 配置。
- 生产 API 环境变量先接 Resend：
  - `REPORT_EMAIL_ENABLED=true`
  - `USER_INVITATION_RESEND_API_KEY=...`
  - `USER_INVITATION_EMAIL_FROM=reports@mistyislet.com`
  - `WALLET_ALERT_EMAIL_PROVIDER=resend`
  - `WALLET_ALERT_EMAIL_FROM=no-reply@mistyislet.com`

验收：

- `POST /api/v1/report-schedules/{id}/send?tenant_id=...` 能收到 PDF 附件。
- Wallet alert dispatch resend smoke 通过。
- DMARC aggregate 记录能看到通过率。

### Phase E1：统一 MailProvider 抽象（已启动）

目标：避免 report schedule、wallet alert、enterprise alert 各自拼 provider。

建议新增接口：

```go
type MailProvider interface {
	Send(ctx context.Context, msg MailMessage) (MailReceipt, error)
}
```

最小实现：

- `resend`：已作为 `api/internal/mail` 的统一 provider，报表发送与 Wallet email sender 共用。
- `mock`：测试和本地开发。
- `cloudflare`：后续接 Cloudflare Email Service 时实现，不影响业务层。

需要收敛的代码面：

- `api/internal/http/routes_report_schedule.go`
- `api/internal/modules/wallet/*alert*`
- `api/internal/modules/enterprise/*alert*`
- `api/internal/config/config.go`

验收：

- 三类发送都返回统一 `provider`, `provider_delivery_id`, `channel_results`。
- provider 失败都能写审计与重试线索。

### Phase E2：Cloudflare 收信入口（1 天）

目标：处理报表回复、退信、供应商 webhook，而不是只发不收。

建议：

- Cloudflare Email Routing 接收：
  - `reports-reply@mistyislet.com`
  - `bounce@mistyislet.com`
  - `security-alerts@mistyislet.com`
- Email Worker 做最薄转发：
  - 验签 / allowlist
  - 提取 `Message-ID`, `from`, `subject`, attachments metadata
  - 调用后端 `/api/v1/webhooks/email/inbound`
- 后端落库：`email_inbound_events` 或复用 notification receipt 模型。

验收：

- 回复报表邮件后，在 Admin 审计页可见 inbound event。
- 退信能关联到 report schedule / wallet delivery notification。

## 4. 不建议直接用 Workspace SMTP 的原因

Google Workspace 更适合作为企业办公邮箱，而不是高频事务邮件供应商。直接接 SMTP relay 会遇到这些问题：

- 发件额度、风控、附件限制和账号安全策略会影响业务邮件。
- OAuth / SMTP relay 配置比 API provider 更难做多环境和可观测。
- 回执、退信、送达事件通常不如事务邮件服务自然。
- 一旦报表 PDF、Wallet 通知、邀请邮件都走同一 Workspace 账号，后续排查会混在真人邮箱日志里。

所以最省事的组合是：

`Google Workspace = 人用邮箱`，`Resend/Cloudflare Email Service = 系统发信`，`Cloudflare Email Routing/Workers = 入站收信与 webhook`。

## 5. 当前待办

| 优先级 | 事项 | 说明 |
|---:|---|---|
| P0 | 配置生产发信 DNS | SPF/DKIM/DMARC，确认 from 域名 |
| P0 | 在 mac mini `.env` 接入 Resend key | 先让 PDF report 和告警真实可发 |
| P1 | 抽 `MailProvider` 接口 | 避免后续从 Resend 换 Cloudflare 时改业务层 |
| P1 | 新增 Cloudflare inbound webhook | 解决回信、退信、供应商事件入库 |
| P2 | Cloudflare Email Service 发信适配 | 等账号侧可用性、价格和限制确认后接 |
