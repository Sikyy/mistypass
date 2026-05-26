const HEADER_NAMES = [
  "message-id",
  "in-reply-to",
  "references",
  "subject",
  "date",
  "from",
  "to",
  "cc",
  "reply-to",
  "x-cloudflare-email-message-id",
  "cf-email-message-id",
]

export default {
  async fetch(_request, _env) {
    return new Response("mistypass-email-inbound-worker ok\n", {
      headers: { "content-type": "text/plain; charset=utf-8" },
    })
  },

  async email(message, env, ctx) {
    const config = resolveConfig(env)
    if (!config.ok) {
      message.setReject(config.error)
      return
    }

    if (!recipientAllowed(message.to, config.allowedRecipients)) {
      message.setReject("recipient not allowed")
      return
    }
    if (!senderAllowed(message.from, config.allowedSenders)) {
      message.setReject("sender not allowed")
      return
    }

    const payload = buildPayload(message, config)
    const webhookPromise = postWebhook(config, payload)

    if (config.rejectOnWebhookFailure) {
      const result = await webhookPromise
      if (!result.ok) {
        message.setReject(`MistyPass webhook failed: ${result.status}`)
        return
      }
    } else {
      ctx.waitUntil(logWebhookFailure(webhookPromise))
    }

    if (config.forwardTo && message.canBeForwarded) {
      await message.forward(config.forwardTo)
    }
  },
}

function resolveConfig(env) {
  const apiBaseURL = trimTrailingSlash(env.MISTYPASS_API_BASE_URL)
  const tenantID = stringValue(env.TENANT_ID)
  const webhookSecret = stringValue(env.EMAIL_INBOUND_WEBHOOK_SECRET)

  if (!apiBaseURL) return { ok: false, error: "MISTYPASS_API_BASE_URL is required" }
  if (!tenantID) return { ok: false, error: "TENANT_ID is required" }
  if (!webhookSecret) return { ok: false, error: "EMAIL_INBOUND_WEBHOOK_SECRET is required" }

  return {
    ok: true,
    apiBaseURL,
    tenantID,
    webhookSecret,
    provider: stringValue(env.PROVIDER) || "cloudflare_email_worker",
    defaultEventType: stringValue(env.DEFAULT_EVENT_TYPE) || "inbound_message",
    forwardTo: stringValue(env.FORWARD_TO),
    rejectOnWebhookFailure: parseBool(env.REJECT_ON_WEBHOOK_FAILURE),
    allowedRecipients: parseList(env.ALLOWED_RECIPIENTS),
    allowedSenders: parseList(env.ALLOWED_SENDERS),
  }
}

function buildPayload(message, config) {
  const headers = selectedHeaders(message.headers)
  const messageID = headers["message-id"] || `cloudflare-${crypto.randomUUID()}`
  const inReplyTo = headers["in-reply-to"] || ""
  const eventType = inReplyTo ? "reply" : config.defaultEventType

  return {
    tenant_id: config.tenantID,
    provider: config.provider,
    event_type: eventType,
    message_id: messageID,
    provider_delivery_id:
      headers["cf-email-message-id"] || headers["x-cloudflare-email-message-id"] || "",
    from: message.from,
    to: [message.to],
    subject: headers.subject || "",
    received_at: new Date().toISOString(),
    metadata: compactMetadata({
      in_reply_to: inReplyTo,
      references: headers.references || "",
      raw_size: String(message.rawSize || 0),
      envelope_to: message.to,
      forwarded_to: config.forwardTo || "",
    }),
    raw_payload: {
      envelope_from: message.from,
      envelope_to: message.to,
      headers,
      raw_size: message.rawSize || 0,
    },
  }
}

async function postWebhook(config, payload) {
  const body = JSON.stringify(payload)
  const timestamp = String(Math.floor(Date.now() / 1000))
  const signature = await signPayload(config.webhookSecret, timestamp, body)
  try {
    const response = await fetch(`${config.apiBaseURL}/api/v1/webhooks/email/inbound`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        "x-mistypass-email-timestamp": timestamp,
        "x-mistypass-email-signature": signature,
      },
      body,
    })

    return { ok: response.ok, status: response.status }
  } catch (error) {
    return { ok: false, status: `network_error:${error.message}` }
  }
}

async function signPayload(secret, timestamp, body) {
  const encoder = new TextEncoder()
  const key = await crypto.subtle.importKey(
    "raw",
    encoder.encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  )
  const digest = await crypto.subtle.sign("HMAC", key, encoder.encode(`${timestamp}.${body}`))
  return `sha256=${toHex(digest)}`
}

function selectedHeaders(headers) {
  const selected = {}
  for (const name of HEADER_NAMES) {
    const value = headers.get(name)
    if (value) selected[name] = value
  }
  return selected
}

function compactMetadata(metadata) {
  const compacted = {}
  for (const [key, value] of Object.entries(metadata)) {
    if (value !== "") compacted[key] = value
  }
  return compacted
}

async function logWebhookFailure(webhookPromise) {
  const result = await webhookPromise
  if (!result.ok) {
    console.error(`MistyPass inbound email webhook failed: ${result.status}`)
  }
}

function recipientAllowed(to, allowedRecipients) {
  return allowedRecipients.length === 0 || allowedRecipients.includes(to.toLowerCase())
}

function senderAllowed(from, allowedSenders) {
  return allowedSenders.length === 0 || allowedSenders.includes(from.toLowerCase())
}

function parseList(value) {
  return stringValue(value)
    .split(",")
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)
}

function parseBool(value) {
  return ["1", "true", "yes", "on"].includes(stringValue(value).toLowerCase())
}

function stringValue(value) {
  return typeof value === "string" ? value.trim() : ""
}

function trimTrailingSlash(value) {
  return stringValue(value).replace(/\/+$/, "")
}

function toHex(buffer) {
  return Array.from(new Uint8Array(buffer))
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("")
}
