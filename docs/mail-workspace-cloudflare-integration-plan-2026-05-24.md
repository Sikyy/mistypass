# Mistyislet 邮件接入方案：Workspace / Cloudflare-first / Resend fallback

> 日期：2026-05-24
> 能力状态：CONTRACT_READY
> 范围：后台报表发送、Wallet / Enterprise 告警、邀请邮件、未来收信回执。

## 1. 结论

如果你现在已经有 Google Workspace，推荐保留 Workspace 负责真人邮箱，把应用系统邮件交给 Cloudflare 独立完成：Cloudflare DNS / DMARC 管域名认证，Cloudflare Email Service 负责事务发信，Cloudflare Email Routing / Workers 负责收信、转发和 webhook。Resend 不再作为 P0 依赖，只保留为可选 fallback；不建议把第一阶段的应用发信直接压到 Workspace SMTP。

本项目当前代码已经具备统一发送抽象、Resend fallback 和 Cloudflare Email Service 主通道：

- 统一发送抽象：`api/internal/mail` 提供 `Provider` / `Message` / `Receipt`，当前已有 Resend 与 Cloudflare Email Service provider。
- 报表发送：`api/internal/http/routes_report_schedule.go` 复用统一 mail provider，并继续兼容 `USER_INVITATION_RESEND_API_KEY` / `USER_INVITATION_RESEND_ENDPOINT`。
- Web Admin：Report Schedules 页面已补 “Send now” 行操作和 provider status 状态条，provider 未启用/未配置会直接显示。
- Report schedule 回归：`docs/testing/curl-report-schedule-resend.zsh` 已覆盖 provider status、send now、Gotenberg PDF 生成、Resend PDF 附件、metadata、idempotency key 与 `report_schedule_sent` 审计。
- Wallet / Enterprise 告警：Wallet email sender 已通过统一 Resend provider 发送，Enterprise worker alerts 复用 Wallet 多通道 dispatch；`spaceemail` 仍映射到 `resend` 兼容模式。
- 入站/回执入口：`POST /api/v1/webhooks/email/inbound` 已补 HMAC 验签、state store 落库、受保护列表查询和 `email_inbound_event_received` 审计；`docs/testing/curl-email-inbound-webhook.zsh` 已接入 API Smoke。
- 当前缺口不是“没有邮件能力”，而是缺少 Cloudflare Email Service 生产账号/DNS/API token smoke、Cloudflare Worker 转发和生产回执关联。

## 2. 三种方案对比

| 方案 | 适合做什么 | 不适合做什么 | 推荐度 |
|---|---|---|---|
| Google Workspace SMTP / Gmail API | 少量内部通知、人工邮箱、临时 MVP | 高频事务邮件、报表附件批量发送、可观测回执 | 低 |
| Cloudflare Email Service | 应用事务邮件、报表附件、系统通知、告警 | 直接替代完整 mailbox，或绕过后端审计链 | 高 |
| Cloudflare Email Routing / Workers | 收信路由、别名、Worker 入站解析、回执/退信 webhook | 大批量附件处理、复杂业务落库 | 高 |
| Resend / Postmark / SES | fallback 发信、供应商对照 smoke | 真人邮箱收发 | 中 |

官方参考：

- Cloudflare Email Routing: https://developers.cloudflare.com/email-routing/
- Cloudflare Email Workers: https://developers.cloudflare.com/email-routing/email-workers/
- Cloudflare Email Service: https://developers.cloudflare.com/email-service/
- Google Workspace SMTP relay: https://support.google.com/a/answer/2956491
- Gmail API sending guide: https://developers.google.com/workspace/gmail/api/guides/sending

## 3. 推荐落地路径

### Phase E0：Cloudflare 域名与发信基线（0.5 天）

目标：让 `mistyislet.com` 的人用邮箱和系统发件身份分清楚。

建议：

- Google Workspace 管理真人邮箱：`hello@mistyislet.com`、`support@mistyislet.com`、`admin@mistyislet.com`。
- 应用发信使用专用身份：`reports@mistyislet.com`、`no-reply@mistyislet.com` 或子域 `mail.mistyislet.com`。
- DNS 统一放 Cloudflare：SPF、DKIM、DMARC 都在 Cloudflare 配置；Cloudflare Email Service onboarding 会生成发送认证记录。
- 生产 API 环境变量先接 Cloudflare Email Service：
  - `REPORT_EMAIL_ENABLED=true`
  - `MAIL_PROVIDER=cloudflare`
  - `CLOUDFLARE_ACCOUNT_ID=...`
  - `CLOUDFLARE_EMAIL_API_TOKEN=...`
  - `CLOUDFLARE_EMAIL_ENDPOINT=https://api.cloudflare.com/client/v4/accounts/{account_id}/email/sending/send`
  - `REPORT_EMAIL_FROM=reports@mistyislet.com`
  - `USER_INVITATION_EMAIL_PROVIDER=cloudflare`
  - `USER_INVITATION_EMAIL_FROM=no-reply@mistyislet.com`
  - `WALLET_ALERT_EMAIL_PROVIDER=cloudflare`
  - `WALLET_ALERT_EMAIL_FROM=no-reply@mistyislet.com`
  - `EMAIL_INBOUND_WEBHOOK_SECRET=...`

验收：

- `POST /api/v1/report-schedules/{id}/send?tenant_id=...` mock smoke 已覆盖 PDF 附件、provider metadata 与审计；Cloudflare DNS/API token 接入后再跑一次真实收件验收。
- Wallet alert dispatch smoke 已通过 CI mock；Cloudflare DNS/API token 接入后再跑一次真实收件验收。
- DMARC aggregate 记录能看到通过率。
- 若 report PDF 可能超过 Cloudflare 普通发信 5 MiB 限制，改为邮件内安全下载链接，附件只用于小文件。

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
- `cloudflare`：已实现，使用 Cloudflare Email Service REST API；report schedule、Wallet alert 与 invitation email 可共用。
- `resend`：保留为 fallback，不阻塞 Cloudflare-first 路径。

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
- 后端落库：`/api/v1/webhooks/email/inbound` 已先落入 `module_email_inbound` state store，并提供 `/api/v1/webhooks/email/inbound/events` 查询；后续如需强查询和报表关联，再迁入专用表。

验收：

- 本地/CI mock 已验证签名 webhook、事件列表和 `email_inbound_event_received` 审计。
- Cloudflare Worker 接入后，回复报表邮件在 Admin 审计页可见 inbound event。
- 退信能关联到 report schedule / wallet delivery notification。

## 4. 不建议直接用 Workspace SMTP 的原因

Google Workspace 更适合作为企业办公邮箱，而不是高频事务邮件供应商。直接接 SMTP relay 会遇到这些问题：

- 发件额度、风控、附件限制和账号安全策略会影响业务邮件。
- OAuth / SMTP relay 配置比 API provider 更难做多环境和可观测。
- 回执、退信、送达事件通常不如事务邮件服务自然。
- 一旦报表 PDF、Wallet 通知、邀请邮件都走同一 Workspace 账号，后续排查会混在真人邮箱日志里。

所以最省事的组合是：

`Google Workspace = 人用邮箱`，`Cloudflare Email Service = 系统发信`，`Cloudflare Email Routing/Workers = 入站收信与 webhook`，`Resend = fallback`。

## 5. 当前待办

| 优先级 | 事项 | 说明 |
|---:|---|---|
| P0 | 配置 Cloudflare Email Service 生产发信 DNS | 完成 Email Sending onboarding，确认 SPF/DKIM/DMARC 与 from 域名 |
| P0 | 在 mac mini `.env` 接入 Cloudflare Email token | 从 `deploy/env/macmini-staging.example.env` 复制完整模板，只在 macmini 本机填真实 token；`docker-compose.yml` 已透传 Cloudflare/report/wallet env 到 API 容器 |
| P1 | 生产真实 Cloudflare smoke | 用 `reports@mistyislet.com` / `no-reply@mistyislet.com` 验证 report PDF 与 Wallet alert 真收件 |
| P1 | 接 Cloudflare Email Worker 转发 | 后端 `/webhooks/email/inbound` 已就绪，下一步部署 Worker 和 allowlist/HMAC |
| P1 | 邮件回执关联业务对象 | 把 `report_schedule_id` / `wallet_delivery_id` / provider delivery id 关联到对应发送记录 |
| P2 | Resend fallback smoke | 仅在 Cloudflare 账号额度、beta 限制或附件限制不满足时启用 |
