import { expect, test, type Page, type Route } from "@playwright/test"

const viewer = {
  id: "user-tenant-admin",
  email: "tenant.admin@sudirman.co",
  role: "tenant_admin",
  tenant_id: "tenant-sudirman",
  building_ids: ["building-1"],
}

const platformViewer = {
  id: "user-platform-admin",
  email: "superadmin@mistypass.local",
  role: "super_admin",
  tenant_id: "",
  building_ids: [],
}

const now = "2026-04-22T10:00:00Z"

function parseRelativeURL(href: string | null) {
  return new URL(href || "/", "http://localhost:4173")
}

async function fulfillJson(route: Route, payload: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(payload),
  })
}

async function seedAuthenticatedSession(page: Page, user = viewer) {
  await page.addInitScript((user) => {
    window.sessionStorage.setItem("mistypass_admin_access_token", "e2e-token")
    window.sessionStorage.setItem("mistypass_admin_refresh_token", "e2e-refresh")
    window.sessionStorage.setItem("mistypass_admin_csrf_token", "e2e-csrf")
    window.localStorage.setItem("i18nextLng", "zh-CN")
    window.localStorage.setItem("mistypass_viewer_email", user.email)
  }, user)
}

async function mockEnterpriseWorkerAlertFlow(
  page: Page,
  options?: {
    extraWebhookReceipts?: Array<Record<string, unknown>>
    extraDLQEntries?: Array<Record<string, unknown>>
  }
) {
  let syncRequests = [
    {
      request_id: "sync-req-pending-1",
      tenant_id: viewer.tenant_id,
      connector_id: "connector-talenta",
      raw_payload_ref: "hris_webhook_receipt:receipt-talenta-1",
      result: {
        job: {
          id: "syn-pending-1",
          tenant_id: viewer.tenant_id,
          source: "hris_talenta",
          status: "completed",
          total: 1,
          created: 1,
          updated: 0,
          deactivated: 0,
          rejected: 0,
          actor: "enterprise.sync.worker",
          started_at: "2026-04-22T09:00:00Z",
          ended_at: "2026-04-22T09:01:00Z",
        },
        items: [],
      },
      access_applied: false,
      access_created: 0,
      access_updated: 0,
      access_rejected: 0,
      access_attempt_count: 1,
      last_access_error: "access service throttled",
      last_access_attempt_at: "2026-04-22T09:40:00Z",
      created_at: "2026-04-22T09:10:00Z",
    },
    {
      request_id: "sync-req-applied-1",
      tenant_id: viewer.tenant_id,
      connector_id: "connector-talenta",
      raw_payload_ref: "hris_pull_run:pull-req-001",
      result: {
        job: {
          id: "syn-applied-1",
          tenant_id: viewer.tenant_id,
          source: "hris_talenta",
          status: "completed",
          total: 1,
          created: 1,
          updated: 0,
          deactivated: 0,
          rejected: 0,
          actor: "enterprise.sync.worker",
          started_at: "2026-04-22T08:00:00Z",
          ended_at: "2026-04-22T08:01:00Z",
        },
        items: [],
      },
      access_applied: true,
      access_created: 1,
      access_updated: 0,
      access_rejected: 0,
      access_attempt_count: 1,
      last_access_attempt_at: "2026-04-22T08:01:00Z",
      created_at: "2026-04-22T08:00:00Z",
    },
  ]

  let dlqEntries = [
    {
      id: "dlq-talenta-1",
      tenant_id: viewer.tenant_id,
      connector_id: "connector-talenta",
      vendor: "talenta",
      receipt_id: "receipt-talenta-1",
      request_id: "wh-req-001",
      event_type: "talenta.employee.detail.created",
      failure_stage: "merge",
      error: "employee merge target missing",
      status: "dlq",
      replay_count: 2,
      replay_state: "cooldown",
      next_retry_at: "2026-04-22T10:07:00Z",
      remaining_attempts: 1,
      cooldown_remaining_seconds: 420,
      stale_in_flight: false,
      last_replay_at: "2026-04-22T09:15:00Z",
      updated_at: "2026-04-22T09:20:00Z",
      created_at: "2026-04-22T09:00:00Z",
    },
    {
      id: "dlq-talenta-2",
      tenant_id: viewer.tenant_id,
      connector_id: "connector-talenta",
      vendor: "talenta",
      receipt_id: "receipt-talenta-3",
      request_id: "wh-req-003",
      event_type: "talenta.employee.transfer.cancelled",
      failure_stage: "normalize",
      error: "employee replay queued",
      status: "dlq",
      replay_count: 0,
      replay_state: "ready",
      remaining_attempts: 3,
      cooldown_remaining_seconds: 0,
      stale_in_flight: false,
      updated_at: "2026-04-22T09:42:00Z",
      created_at: "2026-04-22T09:42:00Z",
    },
  ]

  let webhookReceipts = [
    {
      id: "receipt-talenta-1",
      tenant_id: viewer.tenant_id,
      connector_id: "connector-talenta",
      vendor: "talenta",
      event_type: "talenta.employee.detail.created",
      request_id: "wh-req-001",
      status: "failed",
      attempt_count: 2,
      last_error: "employee merge target missing",
      received_at: "2026-04-22T09:00:00Z",
      last_attempt_at: "2026-04-22T09:18:00Z",
      processed_at: "2026-04-22T09:18:00Z",
      queue_state: "cooldown",
      next_retry_at: "2026-04-22T10:05:00Z",
      remaining_attempts: 1,
      cooldown_remaining_seconds: 300,
      stale_in_flight: false,
    },
    {
      id: "receipt-talenta-2",
      tenant_id: viewer.tenant_id,
      connector_id: "connector-talenta",
      vendor: "talenta",
      event_type: "talenta.employee.detail.updated",
      request_id: "wh-req-002",
      status: "processing",
      attempt_count: 1,
      received_at: "2026-04-22T09:25:00Z",
      last_attempt_at: "2026-04-22T09:26:00Z",
      queue_state: "in_flight",
      processing_deadline_at: "2026-04-22T10:06:00Z",
      remaining_attempts: 2,
      cooldown_remaining_seconds: 0,
      stale_in_flight: false,
    },
    {
      id: "receipt-talenta-3",
      tenant_id: viewer.tenant_id,
      connector_id: "connector-talenta",
      vendor: "talenta",
      event_type: "talenta.employee.transfer.cancelled",
      request_id: "wh-req-003",
      status: "received",
      attempt_count: 0,
      received_at: "2026-04-22T09:40:00Z",
      queue_state: "ready",
      remaining_attempts: 3,
      cooldown_remaining_seconds: 0,
      stale_in_flight: false,
    },
    {
      id: "receipt-talenta-4",
      tenant_id: viewer.tenant_id,
      connector_id: "connector-talenta",
      vendor: "talenta",
      event_type: "talenta.employee.resignation.cancelled",
      request_id: "wh-req-004",
      status: "failed",
      attempt_count: 5,
      last_error: "attempt budget exhausted before handoff confirmation",
      received_at: "2026-04-22T08:50:00Z",
      last_attempt_at: "2026-04-22T09:10:00Z",
      processed_at: "2026-04-22T09:10:00Z",
      queue_state: "attempt_limit",
      remaining_attempts: 0,
      cooldown_remaining_seconds: 0,
      stale_in_flight: false,
    },
  ]

  if (options?.extraWebhookReceipts?.length) {
    webhookReceipts = [...webhookReceipts, ...(options.extraWebhookReceipts as typeof webhookReceipts)]
  }

  let hrisWebhookExecutions = [
    {
      id: "exec-receipt-talenta-1",
      tenant_id: viewer.tenant_id,
      kind: "receipt_process",
      target_id: "receipt-talenta-1",
      receipt_id: "receipt-talenta-1",
      connector_id: "connector-talenta",
      vendor: "talenta",
      request_id: "wh-req-001",
      event_type: "talenta.employee.detail.created",
      failure_stage: "merge",
      execution_mode: "queued",
      dispatch_mode: "worker_tick",
      status: "failed",
      target_status: "failed",
      requested_by: "enterprise.sync.worker",
      audit_source: "hris_webhook_receipt_worker",
      queue_state: "terminal",
      attempt_count: 2,
      requeue_count: 1,
      last_error: "employee merge target missing",
      queued_at: "2026-04-22T09:15:00Z",
      started_at: "2026-04-22T09:16:00Z",
      finished_at: "2026-04-22T09:18:00Z",
      updated_at: "2026-04-22T09:18:00Z",
    },
    {
      id: "exec-dlq-talenta-1",
      tenant_id: viewer.tenant_id,
      kind: "dlq_replay",
      target_id: "dlq-talenta-1",
      receipt_id: "receipt-talenta-1",
      connector_id: "connector-talenta",
      vendor: "talenta",
      request_id: "wh-req-001",
      event_type: "talenta.employee.detail.created",
      failure_stage: "merge",
      execution_mode: "queued",
      dispatch_mode: "worker_task_channel",
      status: "queued",
      target_status: "dlq",
      requested_by: "tenant.admin@sudirman.co",
      audit_source: "enterprise_alerts",
      replay_require_worker: true,
      queue_state: "cooldown",
      next_retry_at: "2026-04-22T10:07:00Z",
      cooldown_remaining_seconds: 420,
      stale_in_flight: false,
      attempt_count: 1,
      requeue_count: 1,
      queued_at: "2026-04-22T09:30:00Z",
      updated_at: "2026-04-22T09:30:00Z",
    },
  ]

  if (options?.extraDLQEntries?.length) {
    dlqEntries = [...dlqEntries, ...(options.extraDLQEntries as typeof dlqEntries)]
  }

  const matchesSearch = (item: string[], query: string) => {
    const normalizedQuery = query.trim().toLowerCase()
    if (!normalizedQuery) {
      return true
    }
    return item.map((value) => value.toLowerCase()).some((value) => value.includes(normalizedQuery))
  }

  const buildWebhookReceiptQueueCounts = (items: typeof webhookReceipts) => ({
    all: items.length,
    ready: items.filter((item) => item.queue_state === "ready").length,
    cooldown: items.filter((item) => item.queue_state === "cooldown").length,
    in_flight: items.filter((item) => item.queue_state === "in_flight").length,
    attempt_limit: items.filter((item) => item.queue_state === "attempt_limit").length,
    terminal: items.filter((item) => item.queue_state === "terminal").length,
  })

  const buildDLQReplayCounts = (items: typeof dlqEntries) => ({
    all: items.length,
    ready: items.filter((item) => item.replay_state === "ready").length,
    cooldown: items.filter((item) => item.replay_state === "cooldown").length,
    in_flight: items.filter((item) => item.replay_state === "in_flight").length,
    attempt_limit: items.filter((item) => item.replay_state === "attempt_limit").length,
    terminal: items.filter((item) => item.replay_state === "terminal").length,
  })

  const buildHRISWebhookExecutionStatusCounts = (items: typeof hrisWebhookExecutions) => ({
    all: items.length,
    queued: items.filter((item) => item.status === "queued").length,
    running: items.filter((item) => item.status === "running").length,
    succeeded: items.filter((item) => item.status === "succeeded").length,
    failed: items.filter((item) => item.status === "failed").length,
  })

  const buildHRISWebhookExecutionQueueCounts = (items: typeof hrisWebhookExecutions) => ({
    all: items.length,
    ready: items.filter((item) => item.queue_state === "ready").length,
    cooldown: items.filter((item) => item.queue_state === "cooldown").length,
    in_flight: items.filter((item) => item.queue_state === "in_flight").length,
    attempt_limit: items.filter((item) => item.queue_state === "attempt_limit").length,
    terminal: items.filter((item) => item.queue_state === "terminal").length,
  })

  const listWebhookReceiptPayload = (url: URL) => {
    const tenantID = url.searchParams.get("tenant_id")?.trim() || ""
    const connectorID = url.searchParams.get("connector_id")?.trim() || ""
    const status = url.searchParams.get("status")?.trim() || ""
    const queueState = url.searchParams.get("queue_state")?.trim() || ""
    const query = url.searchParams.get("q")?.trim() || ""
    const offset = Number(url.searchParams.get("offset") || "0")
    const limit = Number(url.searchParams.get("limit") || "50")

    const baseItems = webhookReceipts.filter((item) => {
      if (tenantID && item.tenant_id !== tenantID) {
        return false
      }
      if (connectorID && item.connector_id !== connectorID) {
        return false
      }
      if (status && item.status !== status) {
        return false
      }
      return matchesSearch(
        [
          item.id,
          item.tenant_id,
          item.connector_id,
          item.vendor,
          item.event_type || "",
          item.request_id || "",
          item.status,
          item.queue_state,
          item.last_error || "",
          String(item.attempt_count || 0),
        ],
        query
      )
    })
    const filteredItems = baseItems.filter((item) => !queueState || item.queue_state === queueState)
    const safeOffset = Number.isFinite(offset) && offset > 0 ? offset : 0
    const safeLimit = Number.isFinite(limit) && limit > 0 ? limit : 50
    const pagedItems = filteredItems.slice(safeOffset, safeOffset + safeLimit)
    return {
      items: pagedItems,
      total: filteredItems.length,
      offset: safeOffset,
      limit: safeLimit,
      next_offset: safeOffset + safeLimit < filteredItems.length ? safeOffset + safeLimit : undefined,
      has_more: safeOffset + safeLimit < filteredItems.length,
      queue_counts: buildWebhookReceiptQueueCounts(baseItems),
    }
  }

  const listDLQPayload = (url: URL) => {
    const tenantID = url.searchParams.get("tenant_id")?.trim() || ""
    const connectorID = url.searchParams.get("connector_id")?.trim() || ""
    const status = url.searchParams.get("status")?.trim() || ""
    const replayState = url.searchParams.get("replay_state")?.trim() || ""
    const query = url.searchParams.get("q")?.trim() || ""
    const offset = Number(url.searchParams.get("offset") || "0")
    const limit = Number(url.searchParams.get("limit") || "50")

    const baseItems = dlqEntries.filter((item) => {
      if (tenantID && item.tenant_id !== tenantID) {
        return false
      }
      if (connectorID && item.connector_id !== connectorID) {
        return false
      }
      if (status && item.status !== status) {
        return false
      }
      return matchesSearch(
        [
          item.id,
          item.tenant_id,
          item.connector_id || "",
          item.vendor || "",
          item.receipt_id || "",
          item.request_id || "",
          item.event_type || "",
          item.failure_stage || "",
          item.error || "",
          item.status || "",
          item.replay_state || "",
          item.raw_payload_ref || "",
          String(item.replay_count || 0),
        ],
        query
      )
    })
    const filteredItems = baseItems.filter((item) => !replayState || item.replay_state === replayState)
    const safeOffset = Number.isFinite(offset) && offset > 0 ? offset : 0
    const safeLimit = Number.isFinite(limit) && limit > 0 ? limit : 50
    const pagedItems = filteredItems.slice(safeOffset, safeOffset + safeLimit)
    return {
      items: pagedItems,
      total: filteredItems.length,
      offset: safeOffset,
      limit: safeLimit,
      next_offset: safeOffset + safeLimit < filteredItems.length ? safeOffset + safeLimit : undefined,
      has_more: safeOffset + safeLimit < filteredItems.length,
      replay_counts: buildDLQReplayCounts(baseItems),
    }
  }

  const listHRISWebhookExecutionPayload = (url: URL) => {
    const tenantID = url.searchParams.get("tenant_id")?.trim() || ""
    const connectorID = url.searchParams.get("connector_id")?.trim() || ""
    const kind = url.searchParams.get("kind")?.trim() || ""
    const status = url.searchParams.get("status")?.trim() || ""
    const queueState = url.searchParams.get("queue_state")?.trim() || ""
    const replayScope = url.searchParams.get("replay_scope")?.trim() || ""
    const executionMode = url.searchParams.get("execution_mode")?.trim() || ""
    const dispatchMode = url.searchParams.get("dispatch_mode")?.trim() || ""
    const targetStatus = url.searchParams.get("target_status")?.trim() || ""
    const query = url.searchParams.get("q")?.trim() || ""
    const offset = Number(url.searchParams.get("offset") || "0")
    const limit = Number(url.searchParams.get("limit") || "50")

    const baseItems = hrisWebhookExecutions.filter((item) => {
      if (tenantID && item.tenant_id !== tenantID) {
        return false
      }
      if (connectorID && item.connector_id !== connectorID) {
        return false
      }
      return matchesSearch(
        [
          item.id,
          item.tenant_id,
          item.kind,
          item.target_id,
          item.receipt_id || "",
          item.connector_id || "",
          item.vendor || "",
          item.request_id || "",
          item.event_type || "",
          item.failure_stage || "",
          item.execution_mode || "",
          item.dispatch_mode || "",
          item.status,
          item.target_status || "",
          item.queue_state || "",
          item.last_error || "",
        ],
        query
      )
    })
    const filteredItems = baseItems.filter((item) => {
      if (kind && item.kind !== kind) {
        return false
      }
      if (status && item.status !== status) {
        return false
      }
      if (queueState && item.queue_state !== queueState) {
        return false
      }
      if (replayScope === "replayed" && !item.replay_source_execution_id) {
        return false
      }
      if (replayScope === "worker_required" && !item.replay_require_worker) {
        return false
      }
      if (executionMode && item.execution_mode !== executionMode) {
        return false
      }
      if (dispatchMode && item.dispatch_mode !== dispatchMode) {
        return false
      }
      if (targetStatus && item.target_status !== targetStatus) {
        return false
      }
      return true
    })
    const safeOffset = Number.isFinite(offset) && offset > 0 ? offset : 0
    const safeLimit = Number.isFinite(limit) && limit > 0 ? limit : 50
    const pagedItems = filteredItems.slice(safeOffset, safeOffset + safeLimit)
    return {
      items: pagedItems,
      total: filteredItems.length,
      offset: safeOffset,
      limit: safeLimit,
      next_offset: safeOffset + safeLimit < filteredItems.length ? safeOffset + safeLimit : undefined,
      has_more: safeOffset + safeLimit < filteredItems.length,
      status_counts: buildHRISWebhookExecutionStatusCounts(baseItems),
      queue_counts: buildHRISWebhookExecutionQueueCounts(baseItems),
    }
  }

  function processMockWebhookReceipts(receiptIDs: string[], executionMode: "inline" | "queued" = "inline") {
    const normalizedReceiptIDs = Array.from(new Set(receiptIDs.map((item) => item.trim()).filter(Boolean)))
    const processedAt = "2026-04-22T10:03:00Z"
    const processingDeadlineAt = "2026-04-22T10:13:00Z"
    let queued = 0
    let processed = 0
    let skipped = 0
    let failed = 0
    let dlq = 0
    const items = [] as Array<{
      receipt_id: string
      status: string
      reason?: string
      error?: string
      item?: (typeof webhookReceipts)[number]
    }>

    webhookReceipts = webhookReceipts.map((item) => {
      if (!normalizedReceiptIDs.includes(item.id)) {
        return item
      }
      if (item.queue_state !== "ready") {
        skipped += 1
        items.push({
          receipt_id: item.id,
          status: "skipped",
          reason: item.queue_state,
          item,
        })
        return item
      }
      if (executionMode === "queued") {
        queued += 1
        const updatedItem = {
          ...item,
          status: "processing",
          attempt_count: (item.attempt_count || 0) + 1,
          last_attempt_at: processedAt,
          processed_at: undefined,
          queue_state: "in_flight",
          next_retry_at: undefined,
          processing_deadline_at: processingDeadlineAt,
          remaining_attempts: Math.max(0, (item.remaining_attempts || 0) - 1),
          cooldown_remaining_seconds: 0,
          stale_in_flight: false,
          last_error: undefined,
        }
        items.push({
          receipt_id: item.id,
          status: "queued",
          item: updatedItem,
        })
        return updatedItem
      }
      processed += 1
      const updatedItem = {
        ...item,
        status: "processed",
        attempt_count: (item.attempt_count || 0) + 1,
        last_attempt_at: processedAt,
        processed_at: processedAt,
        queue_state: "terminal",
        next_retry_at: undefined,
        processing_deadline_at: undefined,
        remaining_attempts: 0,
        cooldown_remaining_seconds: 0,
        stale_in_flight: false,
        last_error: undefined,
      }
      items.push({
        receipt_id: item.id,
        status: "processed",
        item: updatedItem,
      })
      return updatedItem
    })

    return {
      processedAt,
      queued,
      processed,
      skipped,
      failed,
      dlq,
      items,
    }
  }

  let workerAlertSubscription = {
    tenant_id: viewer.tenant_id,
    enabled: true,
    worker_alert_threshold: 3,
    window_seconds: 900,
    cooldown_seconds: 900,
    channels: {
      email: true,
      whatsapp: false,
    },
    receiver_groups: ["security"],
    updated_at: now,
  }

  let workerAlertNotifications = [
    {
      id: "worker-notification-1",
      tenant_id: viewer.tenant_id,
      worker_action: "talenta_pull",
      worker_kind: "pull",
      worker_label: "Talenta Pull Worker",
      fingerprint: "talenta_pull|connector-talenta",
      count: 4,
      threshold: 3,
      failed: 4,
      processed: 8,
      applied: 4,
      connector_id: "connector-talenta",
      vendor: "talenta",
      request_id: "pull-req-001",
      idempotency_key: "tenant-sudirman:talenta_pull:connector-talenta:3",
      attempt: 1,
      channels: ["email"],
      receiver_groups: ["security"],
      status: "failed",
      reason: "provider_timeout",
      retryable: true,
      provider: "mock",
      provider_error: "upstream 503 timeout",
      next_retry_at: "2026-04-22T09:59:00Z",
      triggered_at: "2026-04-22T09:56:00Z",
      channel_results: [
        {
          channel: "email",
          status: "failed",
          reason: "provider_timeout",
          provider: "mock",
          provider_error: "upstream 503 timeout",
          retryable: true,
          receivers: ["security@mistypass.local"],
        },
      ],
    },
    {
      id: "worker-notification-2",
      tenant_id: viewer.tenant_id,
      worker_action: "talenta_processing",
      worker_kind: "webhook",
      worker_label: "Talenta Processing Worker",
      fingerprint: "talenta_processing|connector-talenta|merge",
      count: 3,
      threshold: 2,
      failed: 0,
      processed: 3,
      applied: 3,
      connector_id: "connector-talenta",
      vendor: "talenta",
      event_type: "talenta.employee.detail.created",
      request_id: "wh-req-001",
      failure_stage: "merge",
      idempotency_key: "tenant-sudirman:talenta_processing:connector-talenta-merge:2",
      attempt: 1,
      channels: ["email", "whatsapp"],
      receiver_groups: ["security", "ops"],
      status: "sent",
      retryable: false,
      source_notification_id: "worker-notification-0",
      triggered_at: "2026-04-22T09:30:00Z",
      channel_results: [
        {
          channel: "email",
          status: "sent",
          retryable: false,
          receivers: ["security@mistypass.local"],
        },
        {
          channel: "whatsapp",
          status: "sent",
          retryable: false,
          receivers: ["+10000000000"],
        },
      ],
    },
  ]

  const matchesWorkerAlertNotificationQuery = (item: (typeof workerAlertNotifications)[number], query: string) => {
    const normalizedQuery = query.trim().toLowerCase()
    if (!normalizedQuery) {
      return true
    }
    const values = [
      item.id,
      item.worker_action,
      item.worker_kind,
      item.worker_label,
      item.fingerprint,
      item.connector_id || "",
      item.vendor || "",
      item.event_type || "",
      item.request_id || "",
      item.failure_stage || "",
      item.status,
      item.reason || "",
      item.idempotency_key || "",
      item.provider || "",
      item.provider_error || "",
      item.source_notification_id || "",
      item.next_retry_at || "",
      item.triggered_at,
      ...(item.channels || []),
      ...(item.receiver_groups || []),
    ]
      .join(" ")
      .toLowerCase()
    if (values.includes(normalizedQuery)) {
      return true
    }
    return (item.channel_results || []).some((result) =>
      [
        result.channel,
        result.status,
        result.reason || "",
        result.provider || "",
        result.provider_error || "",
        (result.receivers || []).join(" "),
      ]
        .join(" ")
        .toLowerCase()
        .includes(normalizedQuery)
    )
  }

  const isWorkerAlertNotificationDueNow = (item: (typeof workerAlertNotifications)[number]) => {
    if (item.status !== "failed" || !item.retryable || !item.next_retry_at) {
      return false
    }
    return new Date(item.next_retry_at).getTime() <= new Date(now).getTime()
  }

  const buildWorkerAlertNotificationFilterCounts = (items: typeof workerAlertNotifications) => ({
    all: items.length,
    failed: items.filter((item) => item.status === "failed").length,
    retryable: items.filter((item) => item.retryable).length,
    suppressed: items.filter((item) => item.status === "skipped" && item.reason === "manual_suppressed").length,
    due_now: items.filter((item) => isWorkerAlertNotificationDueNow(item)).length,
  })

  const buildWorkerAlertNotificationStatusCounts = (items: typeof workerAlertNotifications) => ({
    sent: items.filter((item) => item.status === "sent").length,
    failed: items.filter((item) => item.status === "failed").length,
    skipped: items.filter((item) => item.status === "skipped").length,
  })

  const listWorkerAlertNotificationPayload = (url: URL) => {
    const query = url.searchParams.get("q")?.trim() || ""
    const status = url.searchParams.get("status")?.trim() || ""
    const reason = url.searchParams.get("reason")?.trim() || ""
    const retryable = url.searchParams.get("retryable")
    const dueNow = url.searchParams.get("due_now")
    const offset = Number(url.searchParams.get("offset") || "0")
    const limit = Number(url.searchParams.get("limit") || "50")

    const baseItems = workerAlertNotifications.filter((item) => matchesWorkerAlertNotificationQuery(item, query))
    const filteredItems = baseItems.filter((item) => {
      if (status && item.status !== status) {
        return false
      }
      if (reason && (item.reason || "") !== reason) {
        return false
      }
      if (retryable !== null && String(item.retryable) !== retryable) {
        return false
      }
      if (dueNow !== null && String(isWorkerAlertNotificationDueNow(item)) !== dueNow) {
        return false
      }
      return true
    })

    const safeOffset = Number.isFinite(offset) && offset > 0 ? offset : 0
    const safeLimit = Number.isFinite(limit) && limit > 0 ? limit : 50
    const pagedItems = filteredItems.slice(safeOffset, safeOffset + safeLimit)

    return {
      items: pagedItems,
      total: filteredItems.length,
      offset: safeOffset,
      limit: safeLimit,
      next_offset: safeOffset + safeLimit < filteredItems.length ? safeOffset + safeLimit : undefined,
      has_more: safeOffset + safeLimit < filteredItems.length,
      filter_counts: buildWorkerAlertNotificationFilterCounts(baseItems),
      status_counts: buildWorkerAlertNotificationStatusCounts(filteredItems),
    }
  }

  const buildWorkerAlertNotificationCSV = (url: URL) => {
    const payload = listWorkerAlertNotificationPayload(url)
    const escape = (value: unknown) => `"${String(value ?? "").replace(/"/g, '""')}"`
    const rows = [
      [
        "id",
        "tenant_id",
        "triggered_at",
        "worker_action",
        "worker_kind",
        "worker_label",
        "fingerprint",
        "connector_id",
        "vendor",
        "event_type",
        "request_id",
        "failure_stage",
        "mode",
        "count",
        "threshold",
        "failed",
        "processed",
        "applied",
        "status",
        "reason",
        "attempt",
        "retryable",
        "next_retry_at",
        "channels",
        "receiver_groups",
        "provider",
        "provider_error",
        "source_notification_id",
        "idempotency_key",
        "channel_results",
      ],
      ...payload.items.map((item) => [
        item.id,
        item.tenant_id,
        item.triggered_at,
        item.worker_action,
        item.worker_kind,
        item.worker_label,
        item.fingerprint,
        item.connector_id || "",
        item.vendor || "",
        item.event_type || "",
        item.request_id || "",
        item.failure_stage || "",
        item.mode || "",
        item.count,
        item.threshold,
        item.failed,
        item.processed,
        item.applied,
        item.status,
        item.reason || "",
        item.attempt ?? "",
        item.retryable,
        item.next_retry_at || "",
        (item.channels || []).join("|"),
        (item.receiver_groups || []).join("|"),
        item.provider || "",
        item.provider_error || "",
        item.source_notification_id || "",
        item.idempotency_key || "",
        (item.channel_results || [])
          .map((result) =>
            [
              `${result.channel}:${result.status}`,
              result.reason ? `reason=${result.reason}` : "",
              result.provider ? `provider=${result.provider}` : "",
              result.provider_error ? `error=${result.provider_error}` : "",
              result.receivers && result.receivers.length > 0 ? `receivers=${result.receivers.join("|")}` : "",
            ]
              .filter(Boolean)
              .join(" / ")
          )
          .join(" || "),
      ]),
    ]
    return rows.map((row) => row.map((value) => escape(value)).join(",")).join("\n")
  }

  await page.route("**/api/v1/**", async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname
    const method = request.method().toUpperCase()

    if (path === "/api/v1/me" && method === "GET") {
      await fulfillJson(route, viewer)
      return
    }

    if (path === "/api/v1/tenants" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            id: viewer.tenant_id,
            name: "Sudirman HQ",
            type: "company",
            status: "active",
            created_at: now,
          },
        ],
      })
      return
    }

    if (path === "/api/v1/enterprise/employees" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            id: "emp-1",
            tenant_id: viewer.tenant_id,
            external_id: "E1001",
            email: "alice@sudirman.co",
            full_name: "Alice Zhang",
            department: "Operations",
            job_title: "Admin",
            location: "Tower A",
            access_role: "employee",
            building_id: "building-1",
            status: "active",
            source: "hris_talenta",
            last_synced_at: now,
          },
        ],
      })
      return
    }

    if (path === "/api/v1/enterprise/hris-connectors" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/hris-secrets" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-jobs" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-requests" && method === "GET") {
      await fulfillJson(route, { items: syncRequests })
      return
    }

    if (path === "/api/v1/enterprise/sync-requests/reconcile-pending" && method === "POST") {
      const reconciledAt = "2026-04-22T10:01:00Z"
      const pendingItem = syncRequests.find((item) => item.request_id === "sync-req-pending-1")
      if (!pendingItem) {
        await fulfillJson(route, {
          processed: 0,
          applied: 0,
          failed: 0,
          skipped_by_attempt_limit: 0,
          skipped_by_cooldown: 0,
          items: [],
        })
        return
      }
      const updatedItem = {
        ...pendingItem,
        access_applied: true,
        access_created: 1,
        access_attempt_count: pendingItem.access_attempt_count + 1,
        last_access_error: "",
        last_access_attempt_at: reconciledAt,
      }
      syncRequests = syncRequests.map((item) => (item.request_id === updatedItem.request_id ? updatedItem : item))
      await fulfillJson(route, {
        processed: 1,
        applied: 1,
        failed: 0,
        skipped_by_attempt_limit: 0,
        skipped_by_cooldown: 0,
        items: [
          {
            request_id: updatedItem.request_id,
            job_id: updatedItem.result.job.id,
            access_applied: true,
            access_created: 1,
            access_updated: 0,
            access_rejected: 0,
            access_attempt_count: updatedItem.access_attempt_count,
            last_access_attempt_at: reconciledAt,
          },
        ],
      })
      return
    }

    if (path === "/api/v1/enterprise/jit-provision-approvals" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alert-subscription" && method === "GET") {
      await fulfillJson(route, workerAlertSubscription)
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alert-subscription" && method === "PUT") {
      const payload = (request.postDataJSON() ?? {}) as {
        tenant_id?: string
        enabled?: boolean
        worker_alert_threshold?: number
        window_seconds?: number
        cooldown_seconds?: number
        channels?: {
          email?: boolean
          whatsapp?: boolean
        }
        receiver_groups?: string[]
      }
      const nextReceiverGroups =
        payload.receiver_groups === undefined
          ? workerAlertSubscription.receiver_groups
          : Array.from(new Set(payload.receiver_groups.map((value) => value.trim()).filter(Boolean)))
      workerAlertSubscription = {
        tenant_id: payload.tenant_id?.trim() || workerAlertSubscription.tenant_id,
        enabled: typeof payload.enabled === "boolean" ? payload.enabled : workerAlertSubscription.enabled,
        worker_alert_threshold:
          typeof payload.worker_alert_threshold === "number" && payload.worker_alert_threshold > 0
            ? payload.worker_alert_threshold
            : workerAlertSubscription.worker_alert_threshold,
        window_seconds:
          typeof payload.window_seconds === "number" && payload.window_seconds > 0
            ? payload.window_seconds
            : workerAlertSubscription.window_seconds,
        cooldown_seconds:
          typeof payload.cooldown_seconds === "number" && payload.cooldown_seconds >= 0
            ? payload.cooldown_seconds
            : workerAlertSubscription.cooldown_seconds,
        channels: {
          email:
            typeof payload.channels?.email === "boolean"
              ? payload.channels.email
              : workerAlertSubscription.channels.email,
          whatsapp:
            typeof payload.channels?.whatsapp === "boolean"
              ? payload.channels.whatsapp
              : workerAlertSubscription.channels.whatsapp,
        },
        receiver_groups: nextReceiverGroups.length > 0 ? nextReceiverGroups : ["security"],
        updated_at: "2026-04-22T10:03:00Z",
      }
      await fulfillJson(route, workerAlertSubscription)
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/summary" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            tenant_id: viewer.tenant_id,
            worker_action: "talenta_pull",
            worker_kind: "pull",
            worker_label: "Talenta Pull Worker",
            count: 5,
            first_seen_at: "2026-04-22T08:00:00Z",
            last_seen_at: "2026-04-22T09:55:00Z",
            last_failed: 4,
            last_threshold: 3,
            last_processed: 10,
            last_applied: 6,
            last_skipped_by_attempt_limit: 0,
            last_skipped_by_cooldown: 1,
          },
          {
            tenant_id: viewer.tenant_id,
            worker_action: "talenta_replay",
            worker_kind: "webhook",
            worker_label: "Talenta Replay Worker",
            count: 0,
            first_seen_at: "2026-04-22T07:30:00Z",
            last_seen_at: "2026-04-22T09:10:00Z",
            last_failed: 0,
            last_threshold: 3,
            last_processed: 2,
            last_applied: 2,
            last_skipped_by_attempt_limit: 0,
            last_skipped_by_cooldown: 0,
          },
          {
            tenant_id: viewer.tenant_id,
            worker_action: "gadjian_pull",
            worker_kind: "pull",
            worker_label: "Gadjian Pull Worker",
            count: 2,
            first_seen_at: "2026-04-22T07:00:00Z",
            last_seen_at: "2026-04-22T09:00:00Z",
            last_failed: 1,
            last_threshold: 3,
            last_processed: 4,
            last_applied: 3,
            last_skipped_by_attempt_limit: 0,
            last_skipped_by_cooldown: 0,
          },
          {
            tenant_id: viewer.tenant_id,
            worker_action: "talenta_processing",
            worker_kind: "webhook",
            worker_label: "Talenta Processing Worker",
            count: 3,
            first_seen_at: "2026-04-22T07:20:00Z",
            last_seen_at: "2026-04-22T09:20:00Z",
            last_failed: 3,
            last_threshold: 2,
            last_processed: 4,
            last_applied: 0,
            last_skipped_by_attempt_limit: 0,
            last_skipped_by_cooldown: 0,
          },
        ],
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            id: "worker-alert-pull-1",
            tenant_id: viewer.tenant_id,
            actor: "enterprise.sync.worker",
            role: "system",
            action: "talenta_pull",
            worker_action: "talenta_pull",
            worker_kind: "pull",
            worker_label: "Talenta Pull Worker",
            source: "enterprise_sync_worker",
            at: "2026-04-22T09:55:00Z",
            failed: 4,
            threshold: 3,
            processed: 10,
            applied: 6,
            skipped_by_attempt_limit: 0,
            skipped_by_cooldown: 1,
            connector_id: "connector-talenta",
            vendor: "talenta",
            request_id: "pull-req-001",
            mode: "incremental",
            raw_target: "failed=4 threshold=3 processed=10 synced=6 connector_id=connector-talenta vendor=talenta",
          },
          {
            id: "worker-alert-processing-1",
            tenant_id: viewer.tenant_id,
            actor: "enterprise.sync.worker",
            role: "system",
            action: "talenta_processing",
            worker_action: "talenta_processing",
            worker_kind: "webhook",
            worker_label: "Talenta Processing Worker",
            source: "enterprise_sync_worker",
            at: "2026-04-22T09:20:00Z",
            failed: 3,
            threshold: 2,
            processed: 4,
            applied: 0,
            skipped_by_attempt_limit: 0,
            skipped_by_cooldown: 0,
            connector_id: "connector-talenta",
            vendor: "talenta",
            event_type: "talenta.employee.detail.created",
            request_id: "wh-req-001",
            failure_stage: "merge",
            raw_target:
              "failed=3 threshold=2 processed=4 applied=0 connector_id=connector-talenta vendor=talenta event_type=talenta.employee.detail.created request_id=wh-req-001 failure_stage=merge",
          },
        ],
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/notifications/export-csv" && method === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "text/csv; charset=utf-8",
        body: buildWorkerAlertNotificationCSV(url),
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/notifications" && method === "GET") {
      await fulfillJson(route, listWorkerAlertNotificationPayload(url))
      return
    }

    if (path === "/api/v1/enterprise/hris-webhook-receipts" && method === "GET") {
      await fulfillJson(route, listWebhookReceiptPayload(url))
      return
    }

    if (path === "/api/v1/enterprise/hris-webhook-executions" && method === "GET") {
      await fulfillJson(route, listHRISWebhookExecutionPayload(url))
      return
    }

    const executionDetailMatch = path.match(/^\/api\/v1\/enterprise\/hris-webhook-executions\/([^/]+)$/)
    if (executionDetailMatch && method === "GET") {
      const executionID = executionDetailMatch[1]
      const item = hrisWebhookExecutions.find((execution) => execution.id === executionID)
      if (!item) {
        await fulfillJson(route, { error: "execution not found" }, 404)
        return
      }
      await fulfillJson(route, { item })
      return
    }

    const receiptProcessMatch = path.match(/^\/api\/v1\/enterprise\/hris-webhook-receipts\/([^/]+)\/process$/)
    if (receiptProcessMatch && method === "POST") {
      const receiptID = receiptProcessMatch[1]
      const payload = (request.postDataJSON() ?? {}) as {
        execution_mode?: "inline" | "queued" | string
      }
      const executionMode = payload.execution_mode === "queued" ? "queued" : "inline"
      const result = processMockWebhookReceipts([receiptID], executionMode)
      const matched = result.items.find((item) => item.receipt_id === receiptID)?.item
      if (!matched) {
        await route.fulfill({
          status: 404,
          contentType: "application/json; charset=utf-8",
          body: JSON.stringify({ error: "receipt not found" }),
        })
        return
      }
      await fulfillJson(route, {
        item: matched,
        execution_mode: executionMode,
      })
      return
    }

    if (path === "/api/v1/enterprise/hris-webhook-receipts/process-batch" && method === "POST") {
      const payload = (request.postDataJSON() ?? {}) as {
        receipt_ids?: string[]
        execution_mode?: "inline" | "queued" | string
      }
      const executionMode = payload.execution_mode === "queued" ? "queued" : "inline"
      const result = processMockWebhookReceipts(payload.receipt_ids || [], executionMode)

      await fulfillJson(route, {
        tenant_id: viewer.tenant_id,
        total_receipts: Array.from(new Set((payload.receipt_ids || []).map((item) => item.trim()).filter(Boolean))).length,
        queued: result.queued,
        processed: result.processed,
        skipped: result.skipped,
        failed: result.failed,
        dlq: result.dlq,
        execution_mode: executionMode,
        items: result.items,
        updated_at: result.processedAt,
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/notifications/auto-retry" && method === "POST") {
      const retriedItems: typeof workerAlertNotifications = []
      let retried = 0
      let failed = 0
      let skipped = 0
      let suppressed = 0
      const seenFingerprints = new Set<string>()

      workerAlertNotifications = workerAlertNotifications.map((item) => {
        const dueNow = item.status === "failed" && item.retryable && Boolean(item.next_retry_at)
        if (!dueNow) {
          return item
        }
        if (seenFingerprints.has(item.fingerprint)) {
          suppressed += 1
          return item
        }
        seenFingerprints.add(item.fingerprint)
        retried += 1
        const updatedItem = {
          ...item,
          status: "sent",
          reason: "",
          retryable: false,
          provider_error: "",
          next_retry_at: undefined,
          attempt: (item.attempt || 0) + 1,
          triggered_at: "2026-04-22T10:06:00Z",
          channel_results: [
            {
              channel: "email",
              status: "sent",
              retryable: false,
              receivers: ["security@mistypass.local"],
            },
          ],
        }
        retriedItems.push(updatedItem)
        return updatedItem
      })

      await fulfillJson(route, {
        tenant_id: viewer.tenant_id,
        total_notifications: retried + failed + skipped + suppressed,
        retried,
        failed,
        skipped,
        suppressed,
        items: retriedItems,
        updated_at: "2026-04-22T10:06:00Z",
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/notifications/restore-batch" && method === "POST") {
      const payload = (request.postDataJSON() ?? {}) as {
        notification_ids?: string[]
      }
      const notificationIDs = Array.from(new Set((payload.notification_ids ?? []).map((item) => item.trim()).filter(Boolean)))
      const restoredItems: typeof workerAlertNotifications = []
      let restored = 0
      let skipped = 0

      workerAlertNotifications = workerAlertNotifications.map((item) => {
        if (!notificationIDs.includes(item.id)) {
          return item
        }
        if (item.status !== "skipped" || item.reason !== "manual_suppressed") {
          skipped += 1
          return item
        }
        restored += 1
        const updatedItem = {
          ...item,
          status: "failed",
          reason: "manual_suppressed_restored",
          retryable: true,
          next_retry_at: "2026-04-22T10:06:30Z",
          attempt: (item.attempt || 0) + 1,
          triggered_at: "2026-04-22T10:06:30Z",
          channel_results: [
            {
              channel: "email",
              status: "failed",
              reason: "manual_suppressed_restored",
              retryable: true,
              receivers: ["security@mistypass.local"],
            },
          ],
        }
        restoredItems.push(updatedItem)
        return updatedItem
      })

      await fulfillJson(route, {
        tenant_id: viewer.tenant_id,
        total_notifications: notificationIDs.length,
        restored,
        skipped,
        items: restoredItems,
        updated_at: "2026-04-22T10:06:30Z",
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/notifications/suppress-batch" && method === "POST") {
      const payload = (request.postDataJSON() ?? {}) as {
        notification_ids?: string[]
      }
      const notificationIDs = Array.from(new Set((payload.notification_ids ?? []).map((item) => item.trim()).filter(Boolean)))
      const suppressedItems: typeof workerAlertNotifications = []
      let suppressed = 0
      let skipped = 0
      workerAlertNotifications = workerAlertNotifications.map((item) => {
        if (!notificationIDs.includes(item.id)) {
          return item
        }
        if (item.status !== "failed") {
          skipped += 1
          return item
        }
        suppressed += 1
        const updatedItem = {
          ...item,
          status: "skipped",
          reason: "manual_suppressed",
          retryable: false,
          provider_error: "",
          next_retry_at: undefined,
          attempt: (item.attempt || 0) + 1,
          triggered_at: "2026-04-22T10:05:30Z",
          channel_results: [
            {
              channel: "email",
              status: "skipped",
              reason: "manual_suppressed",
              retryable: false,
            },
          ],
        }
        suppressedItems.push(updatedItem)
        return updatedItem
      })
      await fulfillJson(route, {
        tenant_id: viewer.tenant_id,
        total_notifications: notificationIDs.length,
        suppressed,
        skipped,
        items: suppressedItems,
        updated_at: "2026-04-22T10:05:30Z",
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/notifications/retry-batch" && method === "POST") {
      const payload = (request.postDataJSON() ?? {}) as {
        notification_ids?: string[]
      }
      const notificationIDs = Array.from(new Set((payload.notification_ids ?? []).map((item) => item.trim()).filter(Boolean)))
      const seenFingerprints = new Set<string>()
      const retriedItems: typeof workerAlertNotifications = []
      let retried = 0
      let skipped = 0
      let suppressed = 0
      workerAlertNotifications = workerAlertNotifications.map((item) => {
        if (!notificationIDs.includes(item.id)) {
          return item
        }
        if (seenFingerprints.has(item.fingerprint)) {
          suppressed += 1
          return item
        }
        seenFingerprints.add(item.fingerprint)
        if (item.status !== "failed" || !item.retryable) {
          skipped += 1
          return item
        }
        retried += 1
        const updatedItem = {
          ...item,
          status: "sent",
          reason: "",
          retryable: false,
          provider_error: "",
          next_retry_at: undefined,
          attempt: (item.attempt || 0) + 1,
          triggered_at: "2026-04-22T10:05:00Z",
          channel_results: [
            {
              channel: "email",
              status: "sent",
              retryable: false,
            },
          ],
        }
        retriedItems.push(updatedItem)
        return updatedItem
      })
      await fulfillJson(route, {
        tenant_id: viewer.tenant_id,
        total_notifications: notificationIDs.length,
        retried,
        skipped,
        failed: 0,
        suppressed,
        items: retriedItems,
        updated_at: "2026-04-22T10:05:00Z",
      })
      return
    }

    if (
      path.startsWith("/api/v1/enterprise/sync-worker-alerts/notifications/") &&
      path.endsWith("/retry") &&
      method === "POST"
    ) {
      const segments = path.split("/")
      const notificationID = segments[segments.length - 2]
      const currentItem = workerAlertNotifications.find((item) => item.id === notificationID)
      if (!currentItem) {
        await fulfillJson(route, { error: "worker alert notification not found" }, 404)
        return
      }
      const retriedAt = "2026-04-22T10:04:00Z"
      const updatedItem = {
        ...currentItem,
        status: "sent",
        reason: "",
        retryable: false,
        provider_error: "",
        next_retry_at: undefined,
        attempt: (currentItem.attempt || 0) + 1,
        triggered_at: retriedAt,
        channel_results: [
          {
            channel: "email",
            status: "sent",
            retryable: false,
          },
        ],
      }
      workerAlertNotifications = workerAlertNotifications.map((item) =>
        item.id === notificationID ? updatedItem : item
      )
      await fulfillJson(route, updatedItem)
      return
    }

    if (path === "/api/v1/enterprise/hris-webhook-dlq" && method === "GET") {
      await fulfillJson(route, listDLQPayload(url))
      return
    }

    if (path === "/api/v1/enterprise/hris-webhook-dlq/replay-batch" && method === "POST") {
      const payload = (request.postDataJSON() ?? {}) as {
        entry_ids?: string[]
        execution_mode?: "inline" | "queued" | string
      }
      const executionMode = payload.execution_mode === "queued" ? "queued" : "inline"
      const entryIDs = Array.from(new Set((payload.entry_ids || []).map((item) => item.trim()).filter(Boolean)))
      const replayedAt = "2026-04-22T10:02:30Z"
      const processingDeadlineAt = "2026-04-22T10:12:30Z"
      let queued = 0
      let replayed = 0
      let skipped = 0
      let failed = 0
      const items = [] as Array<{
        entry_id: string
        status: string
        reason?: string
        error?: string
        item?: (typeof dlqEntries)[number]
      }>

      dlqEntries = dlqEntries.map((item) => {
        if (!entryIDs.includes(item.id)) {
          return item
        }
        if (item.status === "resolved") {
          skipped += 1
          items.push({
            entry_id: item.id,
            status: "skipped",
            reason: "not_replayable",
            error: "hris dlq entry cannot be replayed",
          })
          return item
        }
        if (executionMode === "queued") {
          queued += 1
          const updatedItem = {
            ...item,
            status: "replaying",
            replay_state: "in_flight",
            next_retry_at: undefined,
            processing_deadline_at: processingDeadlineAt,
            remaining_attempts: Math.max(0, (item.remaining_attempts || 0) - 1),
            cooldown_remaining_seconds: 0,
            stale_in_flight: false,
            replay_count: (item.replay_count || 0) + 1,
            last_replay_at: replayedAt,
            resolved_at: undefined,
            updated_at: replayedAt,
          }
          items.push({
            entry_id: item.id,
            status: "queued",
            item: updatedItem,
          })
          return updatedItem
        }
        replayed += 1
        const updatedItem = {
          ...item,
          status: "resolved",
          replay_state: "terminal",
          next_retry_at: undefined,
          processing_deadline_at: undefined,
          remaining_attempts: 0,
          cooldown_remaining_seconds: 0,
          stale_in_flight: false,
          replay_count: (item.replay_count || 0) + 1,
          last_replay_at: replayedAt,
          resolved_at: replayedAt,
          updated_at: replayedAt,
        }
        items.push({
          entry_id: item.id,
          status: "replayed",
          item: updatedItem,
        })
        return updatedItem
      })

      const processedCount = queued + replayed + skipped
      if (processedCount < entryIDs.length) {
        failed = entryIDs.length - processedCount
      }

      await fulfillJson(route, {
        tenant_id: viewer.tenant_id,
        total_entries: entryIDs.length,
        queued,
        replayed,
        skipped,
        failed,
        execution_mode: executionMode,
        items,
        updated_at: replayedAt,
      })
      return
    }

    if (path.startsWith("/api/v1/enterprise/hris-webhook-dlq/") && path.endsWith("/replay") && method === "POST") {
      const segments = path.split("/")
      const entryID = segments[segments.length - 2]
      const replayedAt = "2026-04-22T10:02:00Z"
      const processingDeadlineAt = "2026-04-22T10:12:00Z"
      const executionMode = url.searchParams.get("execution_mode") === "queued" ? "queued" : "inline"
      const currentItem = dlqEntries.find((item) => item.id === entryID)
      if (!currentItem) {
        await fulfillJson(route, { error: "hris dlq entry not found" }, 404)
        return
      }
      const updatedItem =
        executionMode === "queued"
          ? {
              ...currentItem,
              status: "replaying",
              replay_state: "in_flight",
              next_retry_at: undefined,
              processing_deadline_at: processingDeadlineAt,
              remaining_attempts: Math.max(0, (currentItem.remaining_attempts || 0) - 1),
              cooldown_remaining_seconds: 0,
              stale_in_flight: false,
              replay_count: (currentItem.replay_count || 0) + 1,
              last_replay_at: replayedAt,
              resolved_at: undefined,
              updated_at: replayedAt,
            }
          : {
              ...currentItem,
              status: "resolved",
              replay_state: "terminal",
              next_retry_at: undefined,
              processing_deadline_at: undefined,
              remaining_attempts: 0,
              cooldown_remaining_seconds: 0,
              stale_in_flight: false,
              replay_count: (currentItem.replay_count || 0) + 1,
              last_replay_at: replayedAt,
              resolved_at: replayedAt,
              updated_at: replayedAt,
            }
      dlqEntries = dlqEntries.map((item) => (item.id === entryID ? updatedItem : item))
      await fulfillJson(route, { item: updatedItem, execution_mode: executionMode })
      return
    }

    if (path === "/api/v1/enterprise/hris-pull-states" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            tenant_id: viewer.tenant_id,
            connector_id: "connector-talenta",
            vendor: "talenta",
            status: "failed",
            last_request_id: "pull-req-001",
            last_mode: "incremental",
            last_failure_at: "2026-04-22T09:55:00Z",
            last_success_at: "2026-04-22T08:10:00Z",
            last_error: "429 upstream throttled",
            consecutive_failures: 2,
            created_at: "2026-04-22T07:00:00Z",
            updated_at: "2026-04-22T09:55:00Z",
          },
        ],
      })
      return
    }

    if (path === "/api/v1/groups" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/access-policies" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/cards" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/idp-config" && method === "GET") {
      await fulfillJson(route, {
        id: "idp-1",
        tenant_id: viewer.tenant_id,
        provider: "okta",
        issuer_url: "https://idp.example.com",
        client_id: "client-1",
        scopes: ["openid", "profile", "email"],
        status: "active",
        sync_mode: "jit",
        updated_by: viewer.email,
        created_at: now,
        updated_at: now,
      })
      return
    }

    if (path === "/api/v1/enterprise/scim/config" && method === "GET") {
      await fulfillJson(route, { endpoint: "", token_status: "inactive", supported_operations: [], setup_steps: [] })
      return
    }

    if (path === "/api/v1/enterprise/scim/logs" && method === "GET") {
      await fulfillJson(route, { items: [], total: 0 })
      return
    }

    await fulfillJson(route, { error: `unmocked route: ${method} ${path}` }, 500)
  })
}

test("enterprise sync renders worker alert summary and review links", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto(
    "/enterprise?sync_focus_hint=worker_alert&worker_filter_hint=hot&worker_query_hint=talenta&worker_action=talenta_pull&worker_alert_label=Talenta%20Pull%20Worker&worker_kind=pull&worker_review_status_hint=handled&worker_review_stage_hint=alerts#sync"
  )
  await page.waitForLoadState("networkidle")

  await expect(page.getByTestId("enterprise-sync-worker-alerts-card")).toBeVisible()
  await expect(page.getByText("Worker 告警处置")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-worker-alert-focus")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-worker-review")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-worker-alert-query")).toHaveValue("talenta")
  await expect(page.getByTestId("enterprise-sync-worker-alert-item")).toHaveCount(1)

  const hotItem = page.getByTestId("enterprise-sync-worker-alert-item").first()
  await expect(hotItem).toContainText("Talenta Pull Worker")
  await expect(hotItem).toContainText("高风险")
  await expect(hotItem.getByTestId("enterprise-sync-worker-alert-guidance")).toBeVisible()
  await expect(hotItem.getByTestId("enterprise-sync-worker-alert-guidance-title")).toHaveText(
    "Talenta Pull Sync：正在等待冷却"
  )
  await expect(hotItem.getByTestId("enterprise-sync-worker-alert-guidance-badge")).toHaveText("稍后重试")
  await expect(hotItem.getByTestId("enterprise-sync-worker-alert-guidance-summary")).toContainText(
    "Talenta 最近一次处理 10 条，成功应用 6 条，失败 4 条"
  )
  await expect(hotItem.getByTestId("enterprise-sync-worker-alert-guidance-suggestion-0")).toHaveText(
    "当前已有 retry cooldown，先等待下一轮 worker，再看是否持续告警。"
  )
  await expect(page.getByText("Gadjian Pull Worker")).toHaveCount(0)

  const reviewAlertsLink = parseRelativeURL(
    await page.getByTestId("enterprise-sync-worker-review-alerts-link").getAttribute("href")
  )
  expect(reviewAlertsLink.hash).toBe("#alerts")
  expect(reviewAlertsLink.searchParams.get("alerts_view_hint")).toBe("directory_exceptions")
  expect(reviewAlertsLink.searchParams.get("worker_action")).toBe("talenta_pull")
  expect(reviewAlertsLink.searchParams.get("worker_alert_label")).toBe("Talenta Pull Worker")
  expect(reviewAlertsLink.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(reviewAlertsLink.searchParams.get("worker_kind")).toBe("pull")
  expect(reviewAlertsLink.searchParams.get("worker_query_hint")).toBe("talenta")
  expect(reviewAlertsLink.searchParams.get("worker_review_status_hint")).toBeNull()
  expect(reviewAlertsLink.searchParams.get("worker_review_stage_hint")).toBeNull()

  const clearReviewLink = parseRelativeURL(
    await page.getByRole("link", { name: "清空复核状态" }).getAttribute("href")
  )
  expect(clearReviewLink.hash).toBe("#sync")
  expect(clearReviewLink.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(clearReviewLink.searchParams.get("worker_action")).toBe("talenta_pull")
  expect(clearReviewLink.searchParams.get("worker_alert_label")).toBe("Talenta Pull Worker")
  expect(clearReviewLink.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(clearReviewLink.searchParams.get("worker_kind")).toBe("pull")
  expect(clearReviewLink.searchParams.get("worker_query_hint")).toBe("talenta")
  expect(clearReviewLink.searchParams.get("worker_review_status_hint")).toBeNull()
  expect(clearReviewLink.searchParams.get("worker_review_stage_hint")).toBeNull()

  const alertDetailsLink = parseRelativeURL(await hotItem.getByRole("link", { name: "前往告警" }).getAttribute("href"))
  expect(alertDetailsLink.hash).toBe("#alerts")
  expect(alertDetailsLink.searchParams.get("worker_action")).toBe("talenta_pull")
  expect(alertDetailsLink.searchParams.get("worker_alert_level")).toBe("hot")
  expect(alertDetailsLink.searchParams.get("worker_alert_tenant_id")).toBe(viewer.tenant_id)

  await page.getByTestId("enterprise-sync-worker-alert-filter-stable").click()
  await expect(page.getByTestId("enterprise-sync-worker-alert-empty")).toBeVisible()
  await page.getByTestId("enterprise-sync-worker-alert-scope").getByRole("button", { name: "清空" }).click()
  await page.getByTestId("enterprise-sync-worker-alert-filter-hot").click()
  await expect(page.getByTestId("enterprise-sync-worker-alert-item")).toHaveCount(2)
  const processingItem = page
    .getByTestId("enterprise-sync-worker-alert-item")
    .filter({ hasText: "Talenta Processing Worker" })
    .first()
  await expect(processingItem).toBeVisible()
  await expect(processingItem.getByTestId("enterprise-sync-worker-alert-guidance-title")).toHaveText(
    "Talenta Webhook 处理：字段映射或合并失败"
  )
  await expect(processingItem.getByTestId("enterprise-sync-worker-alert-guidance-badge")).toHaveText("优先处理")

  await page.getByTestId("enterprise-sync-worker-alert-filter-stable").click()
  await expect(page.getByTestId("enterprise-sync-worker-alert-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-sync-worker-alert-item").first()).toContainText("Talenta Replay Worker")
  await expect(page.getByTestId("enterprise-sync-worker-alert-item").first()).toContainText("稳定")
})

test("enterprise worker alert link bridges sync and alerts workspaces", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#sync")
  await page.waitForLoadState("networkidle")

  const hotItem = page
    .getByTestId("enterprise-sync-worker-alert-item")
    .filter({ hasText: "Talenta Pull Worker" })
    .first()
  await expect(hotItem).toBeVisible()
  await hotItem.getByRole("link", { name: "前往告警" }).click()

  await expect(page).toHaveURL(/#alerts$/)
  await expect(page.getByTestId("enterprise-alerts-tab-directory-exceptions")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-sync-worker-card")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-exceptions")).toBeVisible()
  await expect(page.getByTestId("enterprise-worker-alerts")).toBeVisible()
  await expect(page.getByTestId("enterprise-hris-receipts")).toBeVisible()
  await expect(page.getByTestId("enterprise-hris-dlq")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-worker-alert-scope")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-directory-query")).toHaveValue(viewer.tenant_id)
  await expect(page.getByTestId("enterprise-alerts-worker-alert-item")).toHaveCount(1)

  const alertsWorkerItem = page.getByTestId("enterprise-alerts-worker-alert-item").first()
  await expect(alertsWorkerItem).toContainText("Talenta Pull Worker")
  await expect(alertsWorkerItem.getByTestId("enterprise-alerts-worker-alert-guidance")).toBeVisible()
  await expect(alertsWorkerItem.getByTestId("enterprise-alerts-worker-alert-guidance-title")).toHaveText(
    "Talenta Pull Sync：正在等待冷却"
  )
  await expect(page.getByTestId("enterprise-alerts-worker-event-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-worker-event-item").first()).toContainText("connector-talenta")
  await expect(page.getByTestId("enterprise-alerts-pull-state-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-pull-state-item").first()).toContainText("pull-req-001")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-empty")).toBeVisible()

  const batchToDirectoryLink = parseRelativeURL(
    await page.getByTestId("enterprise-alerts-worker-batch-to-directory").getAttribute("href")
  )
  expect(batchToDirectoryLink.pathname).toBe("/access/directory")
  expect(batchToDirectoryLink.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(batchToDirectoryLink.searchParams.get("worker_query_hint")).toBe(viewer.tenant_id)
  expect(batchToDirectoryLink.searchParams.get("worker_action")).toBe("talenta_pull")
  expect(batchToDirectoryLink.searchParams.get("worker_alert_label")).toBe("Talenta Pull Worker")
  expect(batchToDirectoryLink.searchParams.get("worker_kind")).toBe("pull")

  const batchToPoliciesLink = parseRelativeURL(
    await page.getByTestId("enterprise-alerts-worker-batch-to-policies").getAttribute("href")
  )
  expect(batchToPoliciesLink.pathname).toBe("/access/policies")
  expect(batchToPoliciesLink.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(batchToPoliciesLink.searchParams.get("worker_query_hint")).toBe(viewer.tenant_id)
  expect(batchToPoliciesLink.searchParams.get("worker_action")).toBe("talenta_pull")
  expect(batchToPoliciesLink.searchParams.get("worker_alert_label")).toBe("Talenta Pull Worker")
  expect(batchToPoliciesLink.searchParams.get("worker_kind")).toBe("pull")

  const batchToSyncLink = parseRelativeURL(
    await page.getByTestId("enterprise-alerts-worker-batch-to-sync").getAttribute("href")
  )
  expect(batchToSyncLink.hash).toBe("#sync")
  expect(batchToSyncLink.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(batchToSyncLink.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(batchToSyncLink.searchParams.get("worker_query_hint")).toBe(viewer.tenant_id)
  expect(batchToSyncLink.searchParams.get("worker_action")).toBe("talenta_pull")
  expect(batchToSyncLink.searchParams.get("worker_alert_label")).toBe("Talenta Pull Worker")
  expect(batchToSyncLink.searchParams.get("worker_kind")).toBe("pull")

  await page.getByTestId("enterprise-alerts-worker-batch-to-sync").click()
  await expect(page).toHaveURL(/#sync$/)
  await expect(page.getByTestId("enterprise-sync-worker-alert-focus")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-worker-alert-query")).toHaveValue(viewer.tenant_id)
  await expect(page.getByTestId("enterprise-sync-worker-alert-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-sync-worker-alert-item").first()).toContainText("Talenta Pull Worker")
})

test("enterprise alerts can replay DLQ entry and refresh state", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  await page.getByTestId("enterprise-alerts-hris-dlq-filter-ready").click()
  const dlqItem = page.getByTestId("enterprise-alerts-hris-dlq-item").first()
  await expect(dlqItem).toBeVisible()
  await expect(dlqItem).toContainText("employee replay queued")
  await expect(dlqItem).toContainText("重放 0 次")
  await expect(dlqItem).toContainText("ready")

  await dlqItem.getByTestId("enterprise-alerts-hris-dlq-replay").click()

  await expect(page.getByText("DLQ 已加入后台重放队列：dlq-talenta-2 / status replaying")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-empty")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-filter-ready")).toContainText("0")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-filter-in_flight")).toContainText("1")

  await page.getByTestId("enterprise-alerts-hris-dlq-filter-all").click()
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-item").nth(1)).toContainText("replaying")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-item").nth(1)).toContainText("in_flight")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-item").nth(1)).toContainText("重放 1 次")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-replay")).toHaveCount(0)
})

test("enterprise alerts renders Talenta webhook receipt queue runtime and jump link", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const receiptItems = page.getByTestId("enterprise-alerts-webhook-receipt-item")
  await expect(receiptItems).toHaveCount(4)

  const cooldownReceipt = receiptItems.filter({ hasText: "talenta.employee.detail.created" }).first()
  await expect(cooldownReceipt).toContainText("connector-talenta")
  await expect(cooldownReceipt).toContainText("failed")
  await expect(cooldownReceipt).toContainText("cooldown")
  await expect(cooldownReceipt).toContainText("employee merge target missing")

  const inflightReceipt = receiptItems.filter({ hasText: "talenta.employee.detail.updated" }).first()
  await expect(inflightReceipt).toContainText("processing")
  await expect(inflightReceipt).toContainText("in_flight")

  const readyReceipt = receiptItems.filter({ hasText: "talenta.employee.transfer.cancelled" }).first()
  await expect(readyReceipt).toContainText("received")
  await expect(readyReceipt).toContainText("ready")

  const attemptLimitReceipt = receiptItems.filter({ hasText: "talenta.employee.resignation.cancelled" }).first()
  await expect(attemptLimitReceipt).toContainText("attempt_limit")
  await expect(attemptLimitReceipt).toContainText("attempt budget exhausted before handoff confirmation")

  const receiptLink = parseRelativeURL(
    await cooldownReceipt.getByTestId("enterprise-alerts-webhook-receipt-to-sync").getAttribute("href")
  )
  expect(receiptLink.hash).toBe("#sync")
  expect(receiptLink.searchParams.get("worker_action")).toBe("talenta_processing")
  expect(receiptLink.searchParams.get("worker_alert_label")).toBe("Talenta Processing Worker")
  expect(receiptLink.searchParams.get("worker_kind")).toBe("webhook")
  expect(receiptLink.searchParams.get("worker_connector_id")).toBe("connector-talenta")
  expect(receiptLink.searchParams.get("worker_event_type")).toBe("talenta.employee.detail.created")
  expect(receiptLink.searchParams.get("worker_queue_state")).toBe("cooldown")
  expect(receiptLink.searchParams.get("worker_request_id")).toBe("wh-req-001")
  expect(receiptLink.searchParams.get("worker_status")).toBe("failed")
  expect(receiptLink.searchParams.get("worker_vendor")).toBe("talenta")

  await cooldownReceipt.getByTestId("enterprise-alerts-webhook-receipt-to-sync").click()

  await expect(page).toHaveURL(/#sync$/)
  await expect(page.getByTestId("enterprise-sync-worker-alert-focus")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("connector-talenta")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("wh-req-001")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("failed")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("cooldown")
})

test("enterprise alerts can load more Talenta receipt and DLQ backlog", async ({ page }) => {
  const extraWebhookReceipts = Array.from({ length: 9 }, (_, index) => ({
    id: `receipt-talenta-extra-${index + 5}`,
    tenant_id: viewer.tenant_id,
    connector_id: "connector-talenta",
    vendor: "talenta",
    event_type: `talenta.employee.extra.${index + 5}`,
    request_id: `wh-extra-${index + 5}`,
    status: index % 2 === 0 ? "received" : "failed",
    attempt_count: index % 2 === 0 ? 0 : 1,
    last_error: index % 2 === 0 ? undefined : `extra failure ${index + 5}`,
    received_at: `2026-04-22T07:${String(index).padStart(2, "0")}:00Z`,
    last_attempt_at: index % 2 === 0 ? undefined : `2026-04-22T08:${String(index).padStart(2, "0")}:00Z`,
    queue_state: index % 2 === 0 ? "ready" : "cooldown",
    next_retry_at: index % 2 === 0 ? undefined : `2026-04-22T10:${String(index).padStart(2, "0")}:00Z`,
    remaining_attempts: index % 2 === 0 ? 3 : 1,
    cooldown_remaining_seconds: index % 2 === 0 ? 0 : 120,
    stale_in_flight: false,
  }))
  const extraDLQEntries = Array.from({ length: 11 }, (_, index) => ({
    id: `dlq-talenta-extra-${index + 3}`,
    tenant_id: viewer.tenant_id,
    connector_id: "connector-talenta",
    vendor: "talenta",
    receipt_id: `receipt-talenta-extra-${index + 20}`,
    request_id: `dlq-extra-${index + 3}`,
    event_type: `talenta.employee.dlq.${index + 3}`,
    failure_stage: index % 2 === 0 ? "normalize" : "merge",
    error: `extra dlq ${index + 3}`,
    status: "dlq",
    replay_count: index % 2,
    replay_state: index % 2 === 0 ? "ready" : "cooldown",
    next_retry_at: index % 2 === 0 ? undefined : `2026-04-22T10:${String(index).padStart(2, "0")}:30Z`,
    remaining_attempts: index % 2 === 0 ? 2 : 1,
    cooldown_remaining_seconds: index % 2 === 0 ? 0 : 180,
    stale_in_flight: false,
    updated_at: `2026-04-22T09:${String(index).padStart(2, "0")}:00Z`,
    created_at: `2026-04-22T08:${String(index).padStart(2, "0")}:00Z`,
  }))

  await mockEnterpriseWorkerAlertFlow(page, {
    extraWebhookReceipts,
    extraDLQEntries,
  })
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const receiptItems = page.getByTestId("enterprise-alerts-webhook-receipt-item")
  const dlqItems = page.getByTestId("enterprise-alerts-hris-dlq-item")

  await expect(receiptItems).toHaveCount(12)
  await expect(dlqItems).toHaveCount(12)
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-pagination-summary")).toContainText("12")
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-pagination-summary")).toContainText("13")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-pagination-summary")).toContainText("12")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-pagination-summary")).toContainText("13")

  const receiptLoadMoreResponsePromise = page.waitForResponse((response) => {
    const responseURL = new URL(response.url())
    return (
      responseURL.pathname === "/api/v1/enterprise/hris-webhook-receipts" &&
      responseURL.searchParams.get("offset") === "12" &&
      responseURL.searchParams.get("limit") === "12"
    )
  })
  await page.getByTestId("enterprise-alerts-webhook-receipt-load-more").click()
  expect((await receiptLoadMoreResponsePromise).ok()).toBeTruthy()
  await expect(receiptItems).toHaveCount(13)
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-load-more")).toHaveCount(0)

  const dlqLoadMoreResponsePromise = page.waitForResponse((response) => {
    const responseURL = new URL(response.url())
    return (
      responseURL.pathname === "/api/v1/enterprise/hris-webhook-dlq" &&
      responseURL.searchParams.get("offset") === "12" &&
      responseURL.searchParams.get("limit") === "12"
    )
  })
  await page.getByTestId("enterprise-alerts-hris-dlq-load-more").click()
  expect((await dlqLoadMoreResponsePromise).ok()).toBeTruthy()
  await expect(dlqItems).toHaveCount(13)
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-load-more")).toHaveCount(0)
})

test("enterprise alerts can apply receipt runtime scope from URL hints", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto(
    "/enterprise?alerts_view_hint=directory_exceptions&worker_action=talenta_processing&worker_alert_label=Talenta%20Processing%20Worker&worker_kind=webhook&worker_status=received&worker_queue_state=ready#alerts"
  )
  await page.waitForLoadState("networkidle")

  await expect(page.getByTestId("enterprise-alerts-worker-alert-scope")).toContainText("Talenta Processing Worker")
  await expect(page.getByTestId("enterprise-alerts-worker-alert-scope")).toContainText("received")
  await expect(page.getByTestId("enterprise-alerts-worker-alert-scope")).toContainText("ready")
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-item").first()).toContainText(
    "talenta.employee.transfer.cancelled"
  )
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-item").first()).toContainText("received")
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-item").first()).toContainText("ready")
})

test("enterprise alerts can filter Talenta webhook receipts with runtime shortcuts", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const receiptItems = page.getByTestId("enterprise-alerts-webhook-receipt-item")
  await expect(receiptItems).toHaveCount(4)

  await page.getByTestId("enterprise-alerts-webhook-receipt-filter-cooldown").click()
  await expect(receiptItems).toHaveCount(1)
  await expect(receiptItems.first()).toContainText("talenta.employee.detail.created")
  await expect(page.getByTestId("enterprise-alerts-worker-alert-scope")).toContainText("cooldown")
  await expect(receiptItems.first().getByTestId("enterprise-alerts-webhook-receipt-runtime-budget")).toContainText(
    "剩余尝试 1"
  )
  await expect(receiptItems.first().getByTestId("enterprise-alerts-webhook-receipt-runtime-cooldown")).toContainText(
    "冷却剩余 300 秒"
  )

  await page.getByTestId("enterprise-alerts-webhook-receipt-filter-attempt_limit").click()
  await expect(receiptItems).toHaveCount(1)
  await expect(receiptItems.first()).toContainText("talenta.employee.resignation.cancelled")
  await expect(receiptItems.first()).toContainText("attempt_limit")

  await page.getByTestId("enterprise-alerts-webhook-receipt-filter-ready").click()
  await expect(receiptItems).toHaveCount(1)
  await expect(receiptItems.first()).toContainText("talenta.employee.transfer.cancelled")
  await expect(receiptItems.first()).toContainText("ready")
  await expect(receiptItems.first().getByTestId("enterprise-alerts-webhook-receipt-runtime-budget")).toContainText(
    "剩余尝试 3"
  )

  await page.getByTestId("enterprise-alerts-webhook-receipt-filter-all").click()
  await expect(receiptItems).toHaveCount(4)
})

test("enterprise alerts can process single ready Talenta receipt", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  await page.getByTestId("enterprise-alerts-webhook-receipt-filter-ready").click()
  const readyReceipt = page.getByTestId("enterprise-alerts-webhook-receipt-item").first()
  await expect(readyReceipt).toContainText("talenta.employee.transfer.cancelled")
  await expect(readyReceipt.getByTestId("enterprise-alerts-webhook-receipt-process")).toHaveText("处理")

  const processResponsePromise = page.waitForResponse(
    (response) =>
      response.url().includes("/api/v1/enterprise/hris-webhook-receipts/receipt-talenta-3/process") &&
      response.request().method() === "POST"
  )
  await readyReceipt.getByTestId("enterprise-alerts-webhook-receipt-process").click()
  expect((await processResponsePromise).ok()).toBeTruthy()

  await expect(page.getByText("Receipt 已加入后台处理队列：receipt-talenta-3 / status processing")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-process")).toHaveCount(0)
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-empty")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-filter-ready")).toContainText("0")
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-filter-in_flight")).toContainText("2")
})

test("enterprise alerts can batch process visible ready Talenta receipts", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  await page.getByTestId("enterprise-alerts-webhook-receipt-filter-ready").click()
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-process-visible")).toHaveText(
    "处理可见待处理项 (1)"
  )
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-batch-hint")).toContainText("1")

  const processVisibleButton = page.getByTestId("enterprise-alerts-webhook-receipt-process-visible")
  const processBatchResponsePromise = page.waitForResponse(
    (response) =>
      response.url().includes("/api/v1/enterprise/hris-webhook-receipts/process-batch") &&
      response.request().method() === "POST"
  )
  await processVisibleButton.focus()
  await processVisibleButton.press("Enter")
  const processBatchResponse = await processBatchResponsePromise
  expect(processBatchResponse.ok()).toBeTruthy()
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-batch-result-summary")).toContainText(
    "selected 1 / queued 1 / skipped 0 / failed 0"
  )
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-batch-result-item")).toContainText(
    "talenta.employee.transfer.cancelled"
  )
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-batch-result-item")).toContainText("queued")
  const disabledProcessVisibleButton = page.getByTestId("enterprise-alerts-webhook-receipt-process-visible")
  await expect(disabledProcessVisibleButton).toBeVisible()
  await expect(disabledProcessVisibleButton).toBeDisabled()
  await expect(disabledProcessVisibleButton).toHaveAttribute("title", "当前视图没有 ready 状态的 HRIS webhook receipt。")
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-empty")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-filter-ready")).toContainText("0")
  await expect(page.getByTestId("enterprise-alerts-webhook-receipt-filter-in_flight")).toContainText("2")
})

test("enterprise alerts can filter Talenta DLQ with runtime shortcuts", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const dlqItems = page.getByTestId("enterprise-alerts-hris-dlq-item")
  await expect(dlqItems).toHaveCount(2)

  await page.getByTestId("enterprise-alerts-hris-dlq-filter-cooldown").click()
  await expect(dlqItems).toHaveCount(1)
  await expect(dlqItems.first()).toContainText("talenta.employee.detail.created")
  await expect(dlqItems.first()).toContainText("cooldown")
  await expect(page.getByTestId("enterprise-alerts-worker-alert-scope")).toContainText("cooldown")
  await expect(dlqItems.first().getByTestId("enterprise-alerts-hris-dlq-runtime-budget")).toContainText(
    "剩余重放 1"
  )
  await expect(dlqItems.first().getByTestId("enterprise-alerts-hris-dlq-runtime-cooldown")).toContainText(
    "冷却剩余 420 秒"
  )
  await expect(dlqItems.first().getByTestId("enterprise-alerts-hris-dlq-replay")).toHaveCount(0)

  await page.getByTestId("enterprise-alerts-hris-dlq-filter-ready").click()
  await expect(dlqItems).toHaveCount(1)
  await expect(dlqItems.first()).toContainText("talenta.employee.transfer.cancelled")
  await expect(dlqItems.first()).toContainText("ready")
  await expect(dlqItems.first().getByTestId("enterprise-alerts-hris-dlq-runtime-budget")).toContainText(
    "剩余重放 3"
  )
  await expect(dlqItems.first().getByTestId("enterprise-alerts-hris-dlq-replay")).toHaveText("重放")

  await page.getByTestId("enterprise-alerts-hris-dlq-filter-all").click()
  await expect(dlqItems).toHaveCount(2)
})

test("enterprise alerts can batch replay visible Talenta DLQ entries", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  await expect(page.getByTestId("enterprise-alerts-hris-dlq-item")).toHaveCount(2)
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-replay-visible")).toHaveText("重放可见项 (1)")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-batch-hint")).toContainText("1")

  await page.getByTestId("enterprise-alerts-hris-dlq-replay-visible").click()

  await expect(
    page.getByText("DLQ 批量重放已排队：selected 1 / queued 1 / skipped 0 / failed 0")
  ).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-batch-result-summary")).toContainText(
    "selected 1 / queued 1 / skipped 0 / failed 0"
  )
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-batch-result-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-batch-result-item").first()).toContainText(
    "talenta.employee.transfer.cancelled"
  )
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-batch-result-item").first()).toContainText("queued")
  const disabledReplayVisibleButton = page.getByTestId("enterprise-alerts-hris-dlq-replay-visible")
  await expect(disabledReplayVisibleButton).toBeVisible()
  await expect(disabledReplayVisibleButton).toBeDisabled()
  await expect(disabledReplayVisibleButton).toHaveAttribute("title", "当前视图没有 ready 状态的 HRIS DLQ 条目。")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-replay")).toHaveCount(0)
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-filter-ready")).toContainText("0")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-filter-in_flight")).toContainText("1")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-item").nth(1)).toContainText("replaying")
  await expect(page.getByTestId("enterprise-alerts-hris-dlq-item").nth(1)).toContainText("in_flight")
})

test("enterprise alerts renders worker notification history and retries failed notification", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const notificationHistory = page.getByTestId("enterprise-alerts-worker-notification-history")
  await expect(notificationHistory).toBeVisible()
  await expect(notificationHistory.getByTestId("enterprise-alerts-worker-notification-row")).toHaveCount(2)
  await expect(notificationHistory).toContainText("Talenta Pull Worker")
  await expect(notificationHistory).toContainText("provider_timeout")

  const failedRow = notificationHistory
    .getByTestId("enterprise-alerts-worker-notification-row")
    .filter({ hasText: "Talenta Pull Worker" })
    .first()
  await expect(failedRow).toContainText("failed (provider_timeout)")
  await expect(failedRow).toContainText("1")

  await failedRow.getByRole("button", { name: /Retry|重试/i }).click()

  await expect(notificationHistory.getByTestId("enterprise-alerts-worker-notification-row")).toHaveCount(2)
  await expect(
    notificationHistory
      .getByTestId("enterprise-alerts-worker-notification-row")
      .filter({ hasText: "Talenta Pull Worker" })
      .first()
  ).toContainText("sent")
  await expect(
    notificationHistory
      .getByTestId("enterprise-alerts-worker-notification-row")
      .filter({ hasText: "Talenta Pull Worker" })
      .first()
  ).toContainText("2")
  await expect(
    notificationHistory
      .getByTestId("enterprise-alerts-worker-notification-row")
      .filter({ hasText: "Talenta Pull Worker" })
      .first()
      .getByRole("button", { name: /Retry|重试/i })
  ).toHaveCount(0)
})

test("enterprise alerts can expand worker notification details", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const failedRow = page
    .getByTestId("enterprise-alerts-worker-notification-row")
    .filter({ hasText: "Talenta Pull Worker" })
    .first()
  await expect(failedRow).toBeVisible()

  await failedRow.getByRole("button", { name: /Details|详情/i }).click()

  const detailsPanel = page.getByTestId("enterprise-alerts-worker-notification-details-panel-worker-notification-1")
  await expect(detailsPanel).toBeVisible()
  await expect(detailsPanel).toContainText("talenta_pull|connector-talenta")
  await expect(detailsPanel).toContainText("tenant-sudirman:talenta_pull:connector-talenta:3")
  await expect(detailsPanel).toContainText("pull-req-001")
  await expect(detailsPanel).toContainText("2026")
  await expect(detailsPanel).toContainText("upstream 503 timeout")
  await expect(detailsPanel).toContainText("email:failed")
  await expect(detailsPanel).toContainText("reason=provider_timeout")
})

test("enterprise alerts filters due worker notifications and exports visible csv", async ({ page }) => {
  await page.addInitScript(() => {
    const originalCreateObjectURL = URL.createObjectURL.bind(URL)
    const target = window as Window & {
      __notificationHistoryExports: Array<{
        type: string
        text: string
      }>
    }
    target.__notificationHistoryExports = []
    URL.createObjectURL = (object: Blob | MediaSource) => {
      if (object instanceof Blob) {
        void object.text().then((text) => {
          target.__notificationHistoryExports.push({
            type: object.type,
            text,
          })
        })
      }
      return originalCreateObjectURL(object)
    }
  })
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const notificationHistory = page.getByTestId("enterprise-alerts-worker-notification-history")
  await expect(notificationHistory).toBeVisible()

  await page.getByTestId("enterprise-alerts-worker-notification-filter-due_now").click()
  await expect(notificationHistory.getByTestId("enterprise-alerts-worker-notification-row")).toHaveCount(1)
  await expect(notificationHistory).toContainText("Talenta Pull Worker")

  await page.getByTestId("enterprise-alerts-worker-notification-query").fill("provider_timeout")
  await expect(notificationHistory.getByTestId("enterprise-alerts-worker-notification-row")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-worker-notification-export-visible")).toHaveText(
    "导出筛选结果（1）"
  )

  await page.getByTestId("enterprise-alerts-worker-notification-export-visible").click()

  await expect
    .poll(async () =>
      page.evaluate(() => {
        const target = window as Window & {
          __notificationHistoryExports?: Array<{
            type: string
            text: string
          }>
        }
        return target.__notificationHistoryExports?.length || 0
      })
    )
    .toBe(1)

  const exported = await page.evaluate(() => {
    const target = window as Window & {
      __notificationHistoryExports?: Array<{
        type: string
        text: string
      }>
    }
    return target.__notificationHistoryExports?.[0] || null
  })

  expect(exported).not.toBeNull()
  expect(exported?.type).toContain("text/csv")
  expect(exported?.text).toContain('"worker-notification-1"')
  expect(exported?.text).toContain('"provider_timeout"')
  expect(exported?.text).not.toContain('"worker-notification-2"')
})

test("enterprise alerts auto retries due worker notifications", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const notificationHistory = page.getByTestId("enterprise-alerts-worker-notification-history")
  await expect(notificationHistory).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-worker-notification-auto-retry-due")).toHaveText(
    "自动重试到期项（1）"
  )

  await page.getByTestId("enterprise-alerts-worker-notification-auto-retry-due").click()

  await expect(page.getByText("Worker 告警自动重试完成：已发送 1 / 失败 0 / 跳过 0 / 已静默 0")).toBeVisible()
  const pullRow = notificationHistory
    .getByTestId("enterprise-alerts-worker-notification-row")
    .filter({ hasText: "Talenta Pull Worker" })
    .first()
  await expect(pullRow).toContainText("sent")
  await expect(pullRow).toContainText("2")
  await expect(page.getByTestId("enterprise-alerts-worker-notification-auto-retry-due")).toHaveText(
    "自动重试到期项（0）"
  )
})

test("enterprise alerts retries visible worker notifications in batch", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const notificationHistory = page.getByTestId("enterprise-alerts-worker-notification-history")
  await expect(notificationHistory).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-worker-notification-retry-visible")).toHaveText("重试可见项（1）")

  await page.getByTestId("enterprise-alerts-worker-notification-retry-visible").click()

  const pullRow = notificationHistory
    .getByTestId("enterprise-alerts-worker-notification-row")
    .filter({ hasText: "Talenta Pull Worker" })
    .first()
  await expect(pullRow).toContainText("sent")
  await expect(pullRow).toContainText("2")
  await expect(pullRow.getByRole("button", { name: /Retry|重试/i })).toHaveCount(0)
})

test("enterprise alerts suppresses visible failed worker notifications in batch", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const notificationHistory = page.getByTestId("enterprise-alerts-worker-notification-history")
  await expect(notificationHistory).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-worker-notification-suppress-visible")).toHaveText("静默可见项（1）")

  await page.getByTestId("enterprise-alerts-worker-notification-suppress-visible").click()

  const pullRow = notificationHistory
    .getByTestId("enterprise-alerts-worker-notification-row")
    .filter({ hasText: "Talenta Pull Worker" })
    .first()
  await expect(pullRow).toContainText("skipped (manual_suppressed)")
  await expect(pullRow).toContainText("2")
  await expect(pullRow.getByRole("button", { name: /Retry|重试/i })).toHaveCount(0)
})

test("enterprise alerts restores visible manually suppressed worker notifications", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const notificationHistory = page.getByTestId("enterprise-alerts-worker-notification-history")
  await expect(notificationHistory).toBeVisible()

  await page.getByTestId("enterprise-alerts-worker-notification-suppress-visible").click()
  await expect(page.getByTestId("enterprise-alerts-worker-notification-filter-suppressed")).toHaveText("已静默 (1)")

  await page.getByTestId("enterprise-alerts-worker-notification-filter-suppressed").click()
  await expect(page.getByTestId("enterprise-alerts-worker-notification-restore-visible")).toHaveText("恢复可见项（1）")

  await page.getByTestId("enterprise-alerts-worker-notification-restore-visible").click()
  await expect(page.getByText("Worker 告警通知已恢复：恢复 1 / 跳过 0")).toBeVisible()
  await page.getByTestId("enterprise-alerts-worker-notification-filter-all").click()

  const pullRow = notificationHistory
    .getByTestId("enterprise-alerts-worker-notification-row")
    .filter({ hasText: "Talenta Pull Worker" })
    .first()
  await expect(pullRow).toContainText("failed (manual_suppressed_restored)")
  await expect(pullRow).toContainText("3")
  await expect(pullRow.getByRole("button", { name: /Retry|重试/i })).toHaveCount(1)
})

test("enterprise alerts can load and save worker alert subscription", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const subscriptionCard = page.getByTestId("enterprise-alerts-worker-subscription-card")
  const enabledSwitch = page.getByTestId("enterprise-alerts-worker-subscription-enabled")
  const emailSwitch = page.getByTestId("enterprise-alerts-worker-subscription-email")
  const whatsAppSwitch = page.getByTestId("enterprise-alerts-worker-subscription-whatsapp")
  const thresholdInput = page.getByTestId("enterprise-alerts-worker-subscription-threshold")
  const windowInput = page.getByTestId("enterprise-alerts-worker-subscription-window")
  const cooldownInput = page.getByTestId("enterprise-alerts-worker-subscription-cooldown")
  const receiverGroupsInput = page.getByTestId("enterprise-alerts-worker-subscription-receiver-groups")

  await expect(subscriptionCard).toBeVisible()
  await expect(enabledSwitch).toHaveAttribute("aria-checked", "true")
  await expect(emailSwitch).toHaveAttribute("aria-checked", "true")
  await expect(whatsAppSwitch).toHaveAttribute("aria-checked", "false")
  await expect(thresholdInput).toHaveValue("3")
  await expect(windowInput).toHaveValue("900")
  await expect(cooldownInput).toHaveValue("900")
  await expect(receiverGroupsInput).toHaveValue("security")

  await thresholdInput.fill("5")
  await windowInput.fill("600")
  await cooldownInput.fill("1200")
  await receiverGroupsInput.fill("security, ops")
  await whatsAppSwitch.click()
  await page.getByTestId("enterprise-alerts-worker-subscription-save").click()

  await expect(page.getByText("Worker 告警订阅已保存：threshold 5 / window 600 / cooldown 1200")).toBeVisible()
  await expect(whatsAppSwitch).toHaveAttribute("aria-checked", "true")
  await expect(thresholdInput).toHaveValue("5")
  await expect(windowInput).toHaveValue("600")
  await expect(cooldownInput).toHaveValue("1200")
  await expect(receiverGroupsInput).toHaveValue("security, ops")
})

test("enterprise alerts can reconcile pending sync requests and refresh state", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const syncRequestItem = page.getByTestId("enterprise-alerts-sync-request-item").first()
  await expect(syncRequestItem).toBeVisible()
  await expect(syncRequestItem).toContainText("sync-req-pending-1")
  await expect(syncRequestItem).toContainText("access service throttled")

  await page.getByTestId("enterprise-alerts-sync-request-reconcile-pending").click()

  await expect(page.getByText("待补偿请求已处理：processed 1 / applied 1 / failed 0 / attempt-limit 0 / cooldown 0")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-sync-request-empty")).toBeVisible()
})

test("enterprise alerts raw items can jump to sync with scoped worker context", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  const processingEventItem = page
    .getByTestId("enterprise-alerts-worker-event-item")
    .filter({ hasText: "Talenta Processing Worker" })
    .first()
  const processingEventLink = parseRelativeURL(
    await processingEventItem.getByTestId("enterprise-alerts-worker-event-to-sync").getAttribute("href")
  )
  expect(processingEventLink.hash).toBe("#sync")
  expect(processingEventLink.searchParams.get("worker_action")).toBe("talenta_processing")
  expect(processingEventLink.searchParams.get("worker_alert_label")).toBe("Talenta Processing Worker")
  expect(processingEventLink.searchParams.get("worker_kind")).toBe("webhook")
  expect(processingEventLink.searchParams.get("worker_connector_id")).toBe("connector-talenta")
  expect(processingEventLink.searchParams.get("worker_request_id")).toBe("wh-req-001")
  expect(processingEventLink.searchParams.get("worker_failure_stage")).toBe("merge")
  expect(processingEventLink.searchParams.get("worker_vendor")).toBe("talenta")

  const pullStateLink = parseRelativeURL(
    await page.getByTestId("enterprise-alerts-pull-state-to-sync").first().getAttribute("href")
  )
  expect(pullStateLink.hash).toBe("#sync")
  expect(pullStateLink.searchParams.get("worker_action")).toBe("talenta_pull")
  expect(pullStateLink.searchParams.get("worker_alert_label")).toBe("Talenta Pull Worker")
  expect(pullStateLink.searchParams.get("worker_connector_id")).toBe("connector-talenta")
  expect(pullStateLink.searchParams.get("worker_request_id")).toBe("pull-req-001")
  expect(pullStateLink.searchParams.get("worker_mode")).toBe("incremental")
  expect(pullStateLink.searchParams.get("worker_vendor")).toBe("talenta")

  const dlqLink = parseRelativeURL(await page.getByTestId("enterprise-alerts-hris-dlq-to-sync").first().getAttribute("href"))
  expect(dlqLink.hash).toBe("#sync")
  expect(dlqLink.searchParams.get("worker_action")).toBe("talenta_replay")
  expect(dlqLink.searchParams.get("worker_alert_label")).toBe("Talenta Replay Worker")
  expect(dlqLink.searchParams.get("worker_connector_id")).toBe("connector-talenta")
  expect(dlqLink.searchParams.get("worker_request_id")).toBe("wh-req-001")
  expect(dlqLink.searchParams.get("worker_failure_stage")).toBe("merge")
  expect(dlqLink.searchParams.get("worker_replay_state")).toBe("cooldown")
  expect(dlqLink.searchParams.get("worker_status")).toBe("dlq")
  expect(dlqLink.searchParams.get("worker_vendor")).toBe("talenta")

  await processingEventItem.getByTestId("enterprise-alerts-worker-event-to-sync").click()

  await expect(page).toHaveURL(/#sync$/)
  await expect(page.getByTestId("enterprise-sync-worker-alert-focus")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("Talenta Processing Worker")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("connector-talenta")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("wh-req-001")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("merge")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("talenta")
  await expect(page.getByTestId("enterprise-sync-worker-alert-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-sync-worker-alert-item").first()).toContainText("Talenta Processing Worker")
})

test("enterprise alerts dlq item can jump to sync with replay runtime context", async ({ page }) => {
  await mockEnterpriseWorkerAlertFlow(page)
  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  await page.getByTestId("enterprise-alerts-hris-dlq-to-sync").first().click()

  await expect(page).toHaveURL(/#sync$/)
  await expect(page.getByTestId("enterprise-sync-worker-alert-focus")).toBeVisible()
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("connector-talenta")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("wh-req-001")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("merge")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("dlq")
  await expect(page.getByTestId("enterprise-sync-worker-alert-scope")).toContainText("cooldown")
})

test("enterprise alerts should switch worker alerts with platform tenant selector", async ({ page }) => {
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname
    const method = request.method().toUpperCase()
    const tenantID = url.searchParams.get("tenant_id") || ""

    if (path === "/api/v1/me" && method === "GET") {
      await fulfillJson(route, platformViewer)
      return
    }

    if (path === "/api/v1/tenants" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            id: "tenant-sudirman",
            name: "Sudirman HQ",
            type: "company",
            status: "active",
            created_at: now,
          },
          {
            id: "tenant-bandung",
            name: "Bandung Ops",
            type: "company",
            status: "active",
            created_at: now,
          },
        ],
      })
      return
    }

    if (path === "/api/v1/enterprise/employees" && method === "GET") {
      await fulfillJson(route, {
        items:
          tenantID === "tenant-bandung"
            ? [
                {
                  id: "emp-bandung-1",
                  tenant_id: "tenant-bandung",
                  external_id: "B1001",
                  email: "bandung.ops@example.com",
                  full_name: "Bandung Operator",
                  department: "Operations",
                  job_title: "Operator",
                  location: "Bandung",
                  access_role: "employee",
                  building_id: "building-bandung",
                  status: "active",
                  source: "hris_gadjian",
                  last_synced_at: now,
                },
              ]
            : [
                {
                  id: "emp-sudirman-1",
                  tenant_id: "tenant-sudirman",
                  external_id: "S1001",
                  email: "sudirman.ops@example.com",
                  full_name: "Sudirman Operator",
                  department: "Operations",
                  job_title: "Operator",
                  location: "Jakarta",
                  access_role: "employee",
                  building_id: "building-1",
                  status: "active",
                  source: "hris_talenta",
                  last_synced_at: now,
                },
              ],
      })
      return
    }

    if (path === "/api/v1/enterprise/hris-connectors" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/hris-secrets" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-jobs" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-requests" && method === "GET") {
      await fulfillJson(route, {
        items:
          tenantID === "tenant-bandung"
            ? [
                {
                  request_id: "sync-bandung-1",
                  tenant_id: "tenant-bandung",
                  connector_id: "connector-bandung-gadjian",
                  result: {
                    job: {
                      id: "syn-bandung-1",
                      tenant_id: "tenant-bandung",
                      source: "hris_gadjian",
                      status: "completed",
                      total: 1,
                      created: 1,
                      updated: 0,
                      deactivated: 0,
                      rejected: 0,
                      actor: "enterprise.sync.worker",
                      started_at: now,
                      ended_at: now,
                    },
                    items: [],
                  },
                  access_applied: false,
                  access_created: 0,
                  access_updated: 0,
                  access_rejected: 0,
                  access_attempt_count: 1,
                  last_access_error: "bandung pending reconcile",
                  last_access_attempt_at: "2026-04-22T09:00:00Z",
                  created_at: "2026-04-22T08:55:00Z",
                },
              ]
            : [
                {
                  request_id: "sync-sudirman-1",
                  tenant_id: "tenant-sudirman",
                  connector_id: "connector-sudirman-talenta",
                  result: {
                    job: {
                      id: "syn-sudirman-1",
                      tenant_id: "tenant-sudirman",
                      source: "hris_talenta",
                      status: "completed",
                      total: 1,
                      created: 1,
                      updated: 0,
                      deactivated: 0,
                      rejected: 0,
                      actor: "enterprise.sync.worker",
                      started_at: now,
                      ended_at: now,
                    },
                    items: [],
                  },
                  access_applied: false,
                  access_created: 0,
                  access_updated: 0,
                  access_rejected: 0,
                  access_attempt_count: 2,
                  last_access_error: "sudirman pending reconcile",
                  last_access_attempt_at: "2026-04-22T09:05:00Z",
                  created_at: "2026-04-22T08:50:00Z",
                },
              ],
      })
      return
    }

    if (path === "/api/v1/enterprise/jit-provision-approvals" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/summary" && method === "GET") {
      await fulfillJson(route, {
        items:
          tenantID === "tenant-bandung"
            ? [
                {
                  tenant_id: "tenant-bandung",
                  worker_action: "enterprise_hris_pull_worker_alert",
                  worker_kind: "hris_pull",
                  worker_label: "Bandung Gadjian Pull",
                  count: 2,
                  first_seen_at: "2026-04-22T08:20:00Z",
                  last_seen_at: "2026-04-22T09:40:00Z",
                  last_failed: 2,
                  last_threshold: 1,
                  last_processed: 3,
                  last_applied: 1,
                  last_skipped_by_attempt_limit: 0,
                  last_skipped_by_cooldown: 0,
                },
              ]
            : [
                {
                  tenant_id: "tenant-sudirman",
                  worker_action: "enterprise_hris_pull_worker_alert",
                  worker_kind: "hris_pull",
                  worker_label: "Sudirman Talenta Pull",
                  count: 1,
                  first_seen_at: "2026-04-22T08:10:00Z",
                  last_seen_at: "2026-04-22T09:30:00Z",
                  last_failed: 1,
                  last_threshold: 1,
                  last_processed: 2,
                  last_applied: 1,
                  last_skipped_by_attempt_limit: 0,
                  last_skipped_by_cooldown: 0,
                },
              ],
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts" && method === "GET") {
      await fulfillJson(route, {
        items:
          tenantID === "tenant-bandung"
            ? [
                {
                  id: "bandung-pull-alert",
                  tenant_id: "tenant-bandung",
                  actor: "enterprise.sync.worker",
                  role: "system",
                  action: "enterprise_hris_pull_worker_alert",
                  worker_action: "enterprise_hris_pull_worker_alert",
                  worker_kind: "hris_pull",
                  worker_label: "Bandung Gadjian Pull",
                  source: "enterprise_sync_worker",
                  at: "2026-04-22T09:40:00Z",
                  failed: 2,
                  threshold: 1,
                  processed: 3,
                  applied: 1,
                  skipped_by_attempt_limit: 0,
                  skipped_by_cooldown: 0,
                  connector_id: "connector-bandung-gadjian",
                  vendor: "gadjian",
                  request_id: "bandung-pull-001",
                  mode: "full",
                  raw_target: "failed=2 threshold=1 processed=3 synced=1 connector_id=connector-bandung-gadjian vendor=gadjian mode=full",
                },
              ]
            : [
                {
                  id: "sudirman-pull-alert",
                  tenant_id: "tenant-sudirman",
                  actor: "enterprise.sync.worker",
                  role: "system",
                  action: "enterprise_hris_pull_worker_alert",
                  worker_action: "enterprise_hris_pull_worker_alert",
                  worker_kind: "hris_pull",
                  worker_label: "Sudirman Talenta Pull",
                  source: "enterprise_sync_worker",
                  at: "2026-04-22T09:30:00Z",
                  failed: 1,
                  threshold: 1,
                  processed: 2,
                  applied: 1,
                  skipped_by_attempt_limit: 0,
                  skipped_by_cooldown: 0,
                  connector_id: "connector-sudirman-talenta",
                  vendor: "talenta",
                  request_id: "sudirman-pull-001",
                  mode: "incremental",
                  raw_target: "failed=1 threshold=1 processed=2 synced=1 connector_id=connector-sudirman-talenta vendor=talenta mode=incremental",
                },
              ],
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/notifications" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/hris-webhook-receipts" && method === "GET") {
      const items =
        tenantID === "tenant-bandung"
          ? [
              {
                id: "receipt-bandung-1",
                tenant_id: "tenant-bandung",
                connector_id: "connector-bandung-gadjian",
                vendor: "gadjian",
                request_id: "bandung-webhook-001",
                status: "failed",
                attempt_count: 2,
                received_at: "2026-04-22T09:05:00Z",
                queue_state: "cooldown",
                next_retry_at: "2026-04-22T10:10:00Z",
                remaining_attempts: 1,
                cooldown_remaining_seconds: 600,
                stale_in_flight: false,
              },
            ]
          : [
              {
                id: "receipt-sudirman-1",
                tenant_id: "tenant-sudirman",
                connector_id: "connector-sudirman-talenta",
                vendor: "talenta",
                request_id: "sudirman-webhook-001",
                status: "failed",
                attempt_count: 1,
                received_at: "2026-04-22T09:00:00Z",
                queue_state: "ready",
                remaining_attempts: 2,
                cooldown_remaining_seconds: 0,
                stale_in_flight: false,
              },
            ]
      await fulfillJson(route, {
        items,
        total: items.length,
        offset: 0,
        limit: 50,
        has_more: false,
        queue_counts: {
          all: items.length,
          ready: items.filter((item) => item.queue_state === "ready").length,
          cooldown: items.filter((item) => item.queue_state === "cooldown").length,
          in_flight: items.filter((item) => item.queue_state === "in_flight").length,
          attempt_limit: items.filter((item) => item.queue_state === "attempt_limit").length,
          terminal: items.filter((item) => item.queue_state === "terminal").length,
        },
      })
      return
    }

    if (path === "/api/v1/enterprise/hris-webhook-dlq" && method === "GET") {
      const items =
        tenantID === "tenant-bandung"
          ? [
              {
                id: "dlq-bandung-1",
                tenant_id: "tenant-bandung",
                connector_id: "connector-bandung-gadjian",
                vendor: "gadjian",
                request_id: "bandung-webhook-001",
                event_type: "employee.updated",
                failure_stage: "merge",
                error: "bandung merge miss",
                status: "dlq",
                replay_count: 1,
                replay_state: "cooldown",
                next_retry_at: "2026-04-22T10:12:00Z",
                remaining_attempts: 2,
                cooldown_remaining_seconds: 720,
                stale_in_flight: false,
                updated_at: "2026-04-22T09:10:00Z",
                created_at: "2026-04-22T09:00:00Z",
              },
            ]
          : [
              {
                id: "dlq-sudirman-1",
                tenant_id: "tenant-sudirman",
                connector_id: "connector-sudirman-talenta",
                vendor: "talenta",
                request_id: "sudirman-webhook-001",
                event_type: "talenta.employee.detail.created",
                failure_stage: "merge",
                error: "sudirman merge miss",
                status: "dlq",
                replay_count: 2,
                replay_state: "ready",
                remaining_attempts: 1,
                cooldown_remaining_seconds: 0,
                stale_in_flight: false,
                updated_at: "2026-04-22T09:05:00Z",
                created_at: "2026-04-22T08:50:00Z",
              },
            ]
      await fulfillJson(route, {
        items,
        total: items.length,
        offset: 0,
        limit: 50,
        has_more: false,
        replay_counts: {
          all: items.length,
          ready: items.filter((item) => item.replay_state === "ready").length,
          cooldown: items.filter((item) => item.replay_state === "cooldown").length,
          in_flight: items.filter((item) => item.replay_state === "in_flight").length,
          attempt_limit: items.filter((item) => item.replay_state === "attempt_limit").length,
          terminal: items.filter((item) => item.replay_state === "terminal").length,
        },
      })
      return
    }

    if (path === "/api/v1/enterprise/hris-pull-states" && method === "GET") {
      await fulfillJson(route, {
        items:
          tenantID === "tenant-bandung"
            ? [
                {
                  tenant_id: "tenant-bandung",
                  connector_id: "connector-bandung-gadjian",
                  vendor: "gadjian",
                  status: "failed",
                  last_request_id: "bandung-pull-001",
                  last_mode: "full",
                  last_failure_at: "2026-04-22T09:40:00Z",
                  consecutive_failures: 2,
                  created_at: "2026-04-22T08:00:00Z",
                  updated_at: "2026-04-22T09:40:00Z",
                },
              ]
            : [
                {
                  tenant_id: "tenant-sudirman",
                  connector_id: "connector-sudirman-talenta",
                  vendor: "talenta",
                  status: "failed",
                  last_request_id: "sudirman-pull-001",
                  last_mode: "incremental",
                  last_failure_at: "2026-04-22T09:30:00Z",
                  consecutive_failures: 1,
                  created_at: "2026-04-22T08:00:00Z",
                  updated_at: "2026-04-22T09:30:00Z",
                },
              ],
      })
      return
    }

    if (path === "/api/v1/groups" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            id: "group-sudirman",
            tenant_id: "tenant-sudirman",
            name: "Sudirman Group",
            description: "Jakarta group",
            created_at: now,
            updated_at: now,
          },
          {
            id: "group-bandung",
            tenant_id: "tenant-bandung",
            name: "Bandung Group",
            description: "Bandung group",
            created_at: now,
            updated_at: now,
          },
        ],
      })
      return
    }

    if (path === "/api/v1/access-policies" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            id: "policy-sudirman",
            tenant_id: "tenant-sudirman",
            name: "Sudirman Policy",
            scope_type: "group",
            schedule: "weekday",
            members: 1,
            status: "active",
            updated_at: now,
          },
          {
            id: "policy-bandung",
            tenant_id: "tenant-bandung",
            name: "Bandung Policy",
            scope_type: "group",
            schedule: "weekday",
            members: 1,
            status: "active",
            updated_at: now,
          },
        ],
      })
      return
    }

    if (path === "/api/v1/cards" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/idp-config" && method === "GET") {
      await fulfillJson(route, {
        id: tenantID === "tenant-bandung" ? "idp-bandung" : "idp-sudirman",
        tenant_id: tenantID,
        provider: "okta",
        issuer_url: "https://idp.example.com",
        client_id: "client-1",
        scopes: ["openid", "profile", "email"],
        status: "active",
        sync_mode: "jit",
        updated_by: platformViewer.email,
        created_at: now,
        updated_at: now,
      })
      return
    }

    if (path === "/api/v1/enterprise/scim/config" && method === "GET") {
      await fulfillJson(route, { endpoint: "", token_status: "inactive", supported_operations: [], setup_steps: [] })
      return
    }

    if (path === "/api/v1/enterprise/scim/logs" && method === "GET") {
      await fulfillJson(route, { items: [], total: 0 })
      return
    }

    await fulfillJson(route, { error: `unmocked route: ${method} ${path}` }, 500)
  })

  await seedAuthenticatedSession(page, platformViewer)
  await page.goto("/enterprise#alerts")
  await page.waitForLoadState("networkidle")

  await expect(page.getByTestId("enterprise-tenant-select-trigger")).toBeVisible()
  await expect(page.getByTestId("enterprise-alerts-worker-alert-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-worker-alert-item").first()).toContainText("Sudirman Talenta Pull")
  await expect(page.getByTestId("enterprise-alerts-pull-state-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-pull-state-item").first()).toContainText("connector-sudirman-talenta")

  await page.getByTestId("enterprise-tenant-select-trigger").click()
  await page.getByRole("option", { name: "Bandung Ops" }).click()
  await page.waitForLoadState("networkidle")

  await expect(page.getByTestId("enterprise-alerts-worker-alert-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-worker-alert-item").first()).toContainText("Bandung Gadjian Pull")
  await expect(page.getByTestId("enterprise-alerts-pull-state-item")).toHaveCount(1)
  await expect(page.getByTestId("enterprise-alerts-pull-state-item").first()).toContainText("connector-bandung-gadjian")
  await expect(page.getByText("Sudirman Talenta Pull")).toHaveCount(0)
})
