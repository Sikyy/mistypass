# MistyPass Email Inbound Worker

> Capability status: CONTRACT_READY

Cloudflare Email Routing / Email Service can route incoming domain email to this
Worker. The Worker converts the incoming email event into the MistyPass inbound
email webhook contract and signs the payload with the same
`EMAIL_INBOUND_WEBHOOK_SECRET` configured on the API.

## Required Configuration

Set these Worker variables:

| Name | Example | Purpose |
| --- | --- | --- |
| `MISTYPASS_API_BASE_URL` | `https://staging-api.mistyislet.com` | API origin for `/api/v1/webhooks/email/inbound`. |
| `TENANT_ID` | `tenant_demo_jakarta` | Tenant to associate inbound email events with. |
| `PROVIDER` | `cloudflare_email_worker` | Provider name stored by the API. |
| `DEFAULT_EVENT_TYPE` | `inbound_message` | Optional; replies with `In-Reply-To` become `reply`. |
| `FORWARD_TO` | `ops@mistyislet.com` | Optional verified forwarding mailbox. |
| `REJECT_ON_WEBHOOK_FAILURE` | `true` | Optional; reject the email if the API does not accept the webhook. |

Set this Worker secret:

```bash
cd deploy/cloudflare/email-inbound-worker
pnpm dlx wrangler@latest secret put EMAIL_INBOUND_WEBHOOK_SECRET
```

The secret value must match the Mac mini API `.env.staging` value:

```env
EMAIL_INBOUND_WEBHOOK_SECRET=...
```

## Deploy

```bash
cd deploy/cloudflare/email-inbound-worker
pnpm dlx wrangler@latest deploy
```

Then bind the Worker to a Cloudflare Email Routing address:

1. Cloudflare Dashboard -> Email Routing.
2. Create or edit a custom address, for example `reports@mistyislet.com`.
3. Set the action to send to this Worker.
4. Send a test email to the route.
5. In MistyPass, verify `GET /api/v1/webhooks/email/inbound/events`.

## Smoke

Backend-only smoke remains:

```bash
/bin/zsh docs/testing/curl-email-inbound-webhook.zsh
```

After Worker deployment, send a real email and query:

```bash
curl -sS "$API_BASE_URL/api/v1/webhooks/email/inbound/events?tenant_id=tenant_demo_jakarta&provider=cloudflare_email_worker&limit=5" \
  -H "Authorization: Bearer $TOKEN"
```
