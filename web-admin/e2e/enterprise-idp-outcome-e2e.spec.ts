import { expect, test, type Page, type Route } from "@playwright/test"

const viewer = {
  id: "user-tenant-admin",
  email: "tenant.admin@sudirman.co",
  role: "tenant_admin",
  tenant_id: "tenant-sudirman",
  building_ids: ["building-1"],
}

const loginResponse = {
  access_token: "e2e-token",
  refresh_token: "e2e-refresh",
  expires_in: 3600,
  user: viewer,
}

const now = "2026-04-16T00:00:00Z"

type EnterpriseMockScenario = {
  approvals: Array<{
    id: string
    tenant_id: string
    email: string
    external_id?: string
    provider?: string
    employment_status?: string
    status: string
    reason?: string
    external_sync_status?: string
    external_sync_ref?: string
    external_sync_attempt_count?: number
    external_sync_last_error?: string
    external_sync_updated_at?: string
    reviewed_by?: string
    reviewed_at?: string
    created_at: string
    updated_at: string
  }>
  approvalReviewErrorIDs: string[]
  approvalExternalSyncErrorIDs: string[]
  employees: Array<{
    id: string
    tenant_id: string
    external_id: string
    email: string
    full_name: string
    department: string
    job_title: string
    location: string
    access_role: string
    building_id: string
    status: string
    source: string
    last_synced_at: string
  }>
  idpConfig: {
    id: string
    tenant_id: string
    provider: string
    issuer_url: string
    client_id: string
    scopes: string[]
    status: string
    sync_mode: string
    updated_by: string
    created_at: string
    updated_at: string
  }
  policies: Array<{
    id: string
    tenant_id: string
    name: string
    scope_type: string
    schedule: string
    members: number
    status: string
    updated_at: string
  }>
  syncJobs: Array<{
    id: string
    tenant_id: string
    source: string
    status: string
    total: number
    created: number
    updated: number
    deactivated: number
    rejected: number
    actor: string
    started_at: string
    ended_at: string
  }>
  userGroups: Array<{
    id: string
    tenant_id: string
    name: string
    description: string
    created_at: string
    updated_at: string
  }>
  walletPasses: Array<{
    id: string
    tenant_id: string
    provider: string
    template_id: string
    target_type: string
    target_id: string
    object_id: string
    status: string
    save_link: string
    issued_at: string
    created_by: string
    updated_by: string
    created_at: string
    updated_at: string
  }>
  walletTemplates: Array<{
    id: string
    tenant_id: string
    provider: string
    pass_type: string
    class_id: string
    name: string
    status: string
    created_at: string
    updated_at: string
  }>
  walletDeliveries: Array<{
    id: string
    tenant_id: string
    pass_id: string
    template_id: string
    target_type: string
    target_id: string
    status: string
    reason?: string
    attempt?: number
    retryable: boolean
    triggered_at: string
    source_notification_id?: string
  }>
  walletDeliveryRetryErrorIDs: string[]
  walletPassActivateErrorIDs: string[]
  walletMetrics: {
    tenant_id: string
    max_retry: number
    dlq_alert_threshold: number
    summary: {
      tenant_id: string
      max_retry: number
      total: number
      pending: number
      processing: number
      success: number
      failed: number
      dlq: number
      retryable_failed: number
      non_retryable_failed: number
      updated_at: string
    }
    window: {
      window_seconds: number
      since: string
      until: string
      created: number
      updated: number
      pending: number
      processing: number
      success: number
      failed: number
      dlq: number
    }
    alerts?: Array<{
      type: string
      count: number
      threshold: number
      error_code?: string
    }>
    updated_at: string
  }
  walletMetricsTrend: {
    tenant_id: string
    max_retry: number
    dlq_alert_threshold: number
    window_seconds: number
    bucket_seconds: number
    bucket_count: number
    since: string
    until: string
    summary: {
      tenant_id: string
      max_retry: number
      total: number
      pending: number
      processing: number
      success: number
      failed: number
      dlq: number
      retryable_failed: number
      non_retryable_failed: number
      updated_at: string
    }
    buckets: Array<{
      index: number
      start: string
      end: string
      created: number
      updated: number
      pending: number
      processing: number
      success: number
      failed: number
      dlq: number
    }>
    alerts?: Array<{
      type: string
      count: number
      threshold: number
      error_code?: string
    }>
    updated_at: string
  }
  walletAlertSubscription: {
    tenant_id: string
    enabled: boolean
    dlq_alert_threshold: number
    window_seconds: number
    cooldown_seconds: number
    channels: {
      email: boolean
      whatsapp: boolean
    }
    receiver_groups?: string[]
    updated_at: string
  }
  walletAlertNotifications: Array<{
    id: string
    tenant_id: string
    type: string
    count: number
    threshold: number
    status: string
    retryable: boolean
    triggered_at: string
  }>
  walletArchives: Array<{
    id: string
    tenant_id: string
    limit: number
    older_than_seconds: number
    actor: string
    removed: number
    remaining_dlq: number
    at: string
  }>
  walletPhysicalTasks: Array<{
    id: string
    updated_at: string
  }>
  walletBatchIssueError: string
  walletBatchIssueFailedTargetIDs: string[]
  workerAlerts: Array<{
    tenant_id: string
    count: number
    first_seen_at: string
    last_seen_at: string
    last_failed: number
    last_threshold: number
    last_processed: number
    last_applied: number
    last_skipped_by_attempt_limit: number
    last_skipped_by_cooldown: number
  }>
}

function buildScenario(overrides: Partial<EnterpriseMockScenario> = {}): EnterpriseMockScenario {
  return {
    approvals: [],
    approvalReviewErrorIDs: [],
    approvalExternalSyncErrorIDs: [],
    employees: [
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
        source: "hris",
        last_synced_at: now,
      },
    ],
    idpConfig: {
      id: "idp-1",
      tenant_id: viewer.tenant_id,
      provider: "okta",
      issuer_url: "https://idp.example.com",
      client_id: "client-1",
      scopes: ["openid", "profile", "email"],
      status: "active",
      sync_mode: "jit",
      updated_by: "ops@sudirman.co",
      created_at: now,
      updated_at: now,
    },
    policies: [
      {
        id: "policy-1",
        tenant_id: viewer.tenant_id,
        name: "办公区通行策略",
        scope_type: "building",
        schedule: "workdays",
        members: 12,
        status: "active",
        updated_at: now,
      },
    ],
    syncJobs: [
      {
        id: "job-1",
        tenant_id: viewer.tenant_id,
        source: "hris",
        status: "completed",
        total: 1,
        created: 1,
        updated: 0,
        deactivated: 0,
        rejected: 0,
        actor: "ops@sudirman.co",
        started_at: now,
        ended_at: now,
      },
    ],
    userGroups: [
      {
        id: "group-1",
        tenant_id: viewer.tenant_id,
        name: "HQ Admin",
        description: "企业登录承接首批用户组",
        created_at: now,
        updated_at: now,
      },
    ],
    walletPasses: [
      {
        id: "pass-1",
        tenant_id: viewer.tenant_id,
        provider: "google_wallet",
        template_id: "tpl-employee",
        target_type: "user",
        target_id: "emp-1",
        object_id: "obj-1",
        status: "issued",
        save_link: "https://wallet.example.com/pass-1",
        issued_at: now,
        created_by: viewer.email,
        updated_by: viewer.email,
        created_at: now,
        updated_at: now,
      },
    ],
    walletTemplates: [
      {
        id: "tpl-employee",
        tenant_id: viewer.tenant_id,
        provider: "google_wallet",
        pass_type: "employee",
        class_id: "employee-mobile",
        name: "总部员工移动凭证",
        status: "active",
        created_at: now,
        updated_at: now,
      },
    ],
    walletDeliveries: [],
    walletDeliveryRetryErrorIDs: [],
    walletPassActivateErrorIDs: [],
    walletMetrics: {
      tenant_id: viewer.tenant_id,
      max_retry: 3,
      dlq_alert_threshold: 20,
      summary: {
        tenant_id: viewer.tenant_id,
        max_retry: 3,
        total: 1,
        pending: 0,
        processing: 0,
        success: 1,
        failed: 0,
        dlq: 0,
        retryable_failed: 0,
        non_retryable_failed: 0,
        updated_at: now,
      },
      window: {
        window_seconds: 900,
        since: now,
        until: now,
        created: 1,
        updated: 1,
        pending: 0,
        processing: 0,
        success: 1,
        failed: 0,
        dlq: 0,
      },
      alerts: [],
      updated_at: now,
    },
    walletMetricsTrend: {
      tenant_id: viewer.tenant_id,
      max_retry: 3,
      dlq_alert_threshold: 20,
      window_seconds: 900,
      bucket_seconds: 75,
      bucket_count: 12,
      since: now,
      until: now,
      summary: {
        tenant_id: viewer.tenant_id,
        max_retry: 3,
        total: 1,
        pending: 0,
        processing: 0,
        success: 1,
        failed: 0,
        dlq: 0,
        retryable_failed: 0,
        non_retryable_failed: 0,
        updated_at: now,
      },
      buckets: [
        {
          index: 0,
          start: now,
          end: now,
          created: 1,
          updated: 1,
          pending: 0,
          processing: 0,
          success: 1,
          failed: 0,
          dlq: 0,
        },
      ],
      alerts: [],
      updated_at: now,
    },
    walletAlertSubscription: {
      tenant_id: viewer.tenant_id,
      enabled: true,
      dlq_alert_threshold: 20,
      window_seconds: 900,
      cooldown_seconds: 900,
      channels: {
        email: true,
        whatsapp: false,
      },
      receiver_groups: ["security"],
      updated_at: now,
    },
    walletAlertNotifications: [],
    walletArchives: [],
    walletPhysicalTasks: [],
    walletBatchIssueError: "",
    walletBatchIssueFailedTargetIDs: [],
    workerAlerts: [],
    ...overrides,
  }
}

async function fulfillJson(route: Route, payload: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(payload),
  })
}

async function setupApiMocks(page: Page, scenario: EnterpriseMockScenario) {
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const path = url.pathname
    const method = request.method().toUpperCase()

    if (path === "/api/v1/auth/login" && method === "POST") {
      await fulfillJson(route, loginResponse)
      return
    }

    if (path === "/api/v1/me" && method === "GET") {
      await fulfillJson(route, viewer)
      return
    }

    if (path === "/api/v1/enterprise/idp-config" && method === "GET") {
      await fulfillJson(route, scenario.idpConfig)
      return
    }

    if (path === "/api/v1/enterprise/employees" && method === "GET") {
      await fulfillJson(route, { items: scenario.employees })
      return
    }

    if (path === "/api/v1/enterprise/sync-jobs" && method === "GET") {
      await fulfillJson(route, { items: scenario.syncJobs })
      return
    }

    if (path === "/api/v1/enterprise/sync-requests" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/jit-provision-approvals" && method === "GET") {
      await fulfillJson(route, { items: scenario.approvals })
      return
    }

    const reviewMatch = path.match(/^\/api\/v1\/enterprise\/jit-provision-approvals\/([^/]+)\/review$/)
    if (reviewMatch && method === "POST") {
      const approvalID = decodeURIComponent(reviewMatch[1] ?? "")
      const approvalReviewErrorIDSet = new Set(
        scenario.approvalReviewErrorIDs.map((item) => item.trim()).filter(Boolean)
      )
      if (approvalReviewErrorIDSet.has(approvalID)) {
        await fulfillJson(route, { error: `mock approval review failed: ${approvalID}` }, 500)
        return
      }
      let decision: "approved" | "rejected" = "approved"
      try {
        const payload = request.postDataJSON() as { decision?: string }
        if (payload?.decision === "rejected") {
          decision = "rejected"
        }
      } catch {
        decision = "approved"
      }

      scenario.approvals = scenario.approvals.map((item) =>
        item.id === approvalID
          ? {
              ...item,
              status: decision,
              updated_at: now,
            }
          : item
      )

      const updated = scenario.approvals.find((item) => item.id === approvalID)
      await fulfillJson(route, {
        item:
          updated ??
          {
            id: approvalID,
            tenant_id: viewer.tenant_id,
            email: `${approvalID}@example.com`,
            status: decision,
            created_at: now,
            updated_at: now,
          },
      })
      return
    }

    const externalSyncMatch = path.match(/^\/api\/v1\/enterprise\/jit-provision-approvals\/([^/]+)\/external-sync$/)
    if (externalSyncMatch && method === "POST") {
      const approvalID = decodeURIComponent(externalSyncMatch[1] ?? "")
      const approvalExternalSyncErrorIDSet = new Set(
        scenario.approvalExternalSyncErrorIDs.map((item) => item.trim()).filter(Boolean)
      )
      if (approvalExternalSyncErrorIDSet.has(approvalID)) {
        await fulfillJson(route, { error: `mock approval external sync failed: ${approvalID}` }, 500)
        return
      }
      let status: "synced" | "failed" = "synced"
      let externalSyncRef = ""
      let externalSyncLastError = ""
      try {
        const payload = request.postDataJSON() as {
          status?: string
          external_sync_ref?: string
          last_error?: string
        }
        if (payload?.status === "failed") {
          status = "failed"
        }
        externalSyncRef = payload?.external_sync_ref?.trim() || ""
        externalSyncLastError = payload?.last_error?.trim() || ""
      } catch {
        status = "synced"
      }

      scenario.approvals = scenario.approvals.map((item) =>
        item.id === approvalID
          ? {
              ...item,
              external_sync_status: status,
              external_sync_ref: externalSyncRef || item.external_sync_ref || `mock-external-sync-${approvalID}`,
              external_sync_last_error:
                status === "failed" ? externalSyncLastError || "mock external sync failed" : "",
              external_sync_attempt_count: (item.external_sync_attempt_count ?? 0) + 1,
              external_sync_updated_at: now,
              updated_at: now,
            }
          : item
      )

      const updated = scenario.approvals.find((item) => item.id === approvalID)
      await fulfillJson(route, {
        item:
          updated ??
          {
            id: approvalID,
            tenant_id: viewer.tenant_id,
            email: `${approvalID}@example.com`,
            status: "approved",
            external_sync_status: status,
            external_sync_ref: externalSyncRef || `mock-external-sync-${approvalID}`,
            external_sync_last_error: status === "failed" ? externalSyncLastError || "mock external sync failed" : "",
            external_sync_attempt_count: 1,
            external_sync_updated_at: now,
            created_at: now,
            updated_at: now,
          },
      })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/summary" && method === "GET") {
      await fulfillJson(route, { items: scenario.workerAlerts })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/notifications" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/hris-webhook-receipts" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/hris-webhook-dlq" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/hris-pull-states" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/user-groups" && method === "GET") {
      await fulfillJson(route, { items: scenario.userGroups })
      return
    }

    if (path === "/api/v1/access-policies" && method === "GET") {
      await fulfillJson(route, { items: scenario.policies })
      return
    }

    if (path === "/api/v1/wallet/passes" && method === "GET") {
      await fulfillJson(route, { items: scenario.walletPasses })
      return
    }

    if (path === "/api/v1/wallet/templates" && method === "GET") {
      await fulfillJson(route, { items: scenario.walletTemplates })
      return
    }

    if (path === "/api/v1/wallet/deliveries" && method === "GET") {
      await fulfillJson(route, { items: scenario.walletDeliveries })
      return
    }

    if (path === "/api/v1/wallet/passes/issue-batch" && method === "POST") {
      if (scenario.walletBatchIssueError.trim()) {
        await fulfillJson(route, { error: scenario.walletBatchIssueError.trim() }, 500)
        return
      }

      let payload: {
        tenant_id?: string
        template_id?: string
        target_type?: "user" | "visitor" | string
        target_ids?: string[]
        execution_mode?: "inline" | "queued" | string
      } = {}
      try {
        payload = request.postDataJSON() as {
          tenant_id?: string
          template_id?: string
          target_type?: "user" | "visitor" | string
          target_ids?: string[]
          execution_mode?: "inline" | "queued" | string
        }
      } catch {
        payload = {}
      }

      const targetIDs = Array.isArray(payload.target_ids)
        ? payload.target_ids.map((item) => item.trim()).filter(Boolean)
        : []
      const failedTargetIDSet = new Set(
        scenario.walletBatchIssueFailedTargetIDs.map((item) => item.trim()).filter(Boolean)
      )
      const targetType = payload.target_type === "visitor" ? "visitor" : "user"
      const templateID = payload.template_id?.trim() || scenario.walletTemplates[0]?.id || "tpl-employee"
      const executionMode = payload.execution_mode === "inline" ? "inline" : "queued"
      const batchID = `batch-${Date.now()}`
      const createdItems = targetIDs.map((targetID, index) => {
        const failed = failedTargetIDSet.has(targetID)
        const status = failed ? "failed" : "success"
        const itemID = `job-${targetID}-${index}`
        return {
          id: itemID,
          tenant_id: payload.tenant_id?.trim() || viewer.tenant_id,
          provider: "google_wallet",
          batch_id: batchID,
          template_id: templateID,
          target_type: targetType,
          target_id: targetID,
          pass_id: failed ? "" : `pass-issued-${targetID}`,
          status,
          retry_count: 0,
          error_code: failed ? "mock_issue_failed" : "",
          error_message: failed ? "mock issue failed" : "",
          created_at: now,
          updated_at: now,
        }
      })

      const createdPasses = createdItems
        .filter((item) => item.status === "success")
        .map((item) => ({
          id: item.pass_id || `pass-issued-${item.target_id}`,
          tenant_id: item.tenant_id,
          provider: "google_wallet",
          template_id: item.template_id,
          target_type: item.target_type,
          target_id: item.target_id,
          object_id: `obj-issued-${item.target_id}`,
          status: "issued",
          save_link: `https://wallet.example.com/${item.pass_id || `pass-issued-${item.target_id}`}`,
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        }))

      if (createdPasses.length > 0) {
        scenario.walletPasses = [...createdPasses, ...scenario.walletPasses]
      }

      await fulfillJson(route, {
        items: createdItems,
        execution_mode: executionMode,
      })
      return
    }

    const deliveryRetryMatch = path.match(/^\/api\/v1\/wallet\/deliveries\/([^/]+)\/retry$/)
    if (deliveryRetryMatch && method === "POST") {
      const notificationID = decodeURIComponent(deliveryRetryMatch[1] ?? "")
      const deliveryRetryErrorIDSet = new Set(
        scenario.walletDeliveryRetryErrorIDs.map((item) => item.trim()).filter(Boolean)
      )
      if (deliveryRetryErrorIDSet.has(notificationID)) {
        await fulfillJson(route, { error: `mock delivery retry failed: ${notificationID}` }, 500)
        return
      }
      scenario.walletDeliveries = scenario.walletDeliveries.map((item) =>
        item.id === notificationID
          ? {
              ...item,
              status: "sent",
              retryable: false,
              reason: "",
              attempt: (item.attempt ?? 1) + 1,
              source_notification_id: item.id,
              triggered_at: now,
            }
          : item
      )
      const updated = scenario.walletDeliveries.find((item) => item.id === notificationID)
      await fulfillJson(
        route,
        updated ?? {
          id: notificationID,
          tenant_id: viewer.tenant_id,
          pass_id: "pass-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "fallback-target",
          status: "sent",
          retryable: false,
          attempt: 2,
          triggered_at: now,
        }
      )
      return
    }

    const passActivateMatch = path.match(/^\/api\/v1\/wallet\/passes\/([^/]+)\/activate$/)
    if (passActivateMatch && method === "PATCH") {
      const passID = decodeURIComponent(passActivateMatch[1] ?? "")
      const passActivateErrorIDSet = new Set(
        scenario.walletPassActivateErrorIDs.map((item) => item.trim()).filter(Boolean)
      )
      if (passActivateErrorIDSet.has(passID)) {
        await fulfillJson(route, { error: `mock pass activate failed: ${passID}` }, 500)
        return
      }
      scenario.walletPasses = scenario.walletPasses.map((item) =>
        item.id === passID
          ? {
              ...item,
              status: "active",
              updated_at: now,
              activated_at: now,
            }
          : item
      )
      const updated = scenario.walletPasses.find((item) => item.id === passID)
      await fulfillJson(
        route,
        updated ?? {
          id: passID,
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "fallback-target",
          object_id: "obj-fallback",
          status: "active",
          save_link: "https://wallet.example.com/fallback",
          issued_at: now,
          activated_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        }
      )
      return
    }

    if (path === "/api/v1/wallet/jobs/metrics" && method === "GET") {
      await fulfillJson(route, scenario.walletMetrics)
      return
    }

    if (path === "/api/v1/wallet/jobs/metrics/trend" && method === "GET") {
      await fulfillJson(route, scenario.walletMetricsTrend)
      return
    }

    if (path === "/api/v1/wallet/jobs/dlq/cleanup/archives" && method === "GET") {
      await fulfillJson(route, { items: scenario.walletArchives })
      return
    }

    if (path === "/api/v1/wallet/jobs/alert-notifications" && method === "GET") {
      await fulfillJson(route, { items: scenario.walletAlertNotifications })
      return
    }

    if (path === "/api/v1/wallet/jobs/alert-subscription" && method === "GET") {
      await fulfillJson(route, scenario.walletAlertSubscription)
      return
    }

    if (path === "/api/v1/wallet/physical-card-tasks" && method === "GET") {
      await fulfillJson(route, { items: scenario.walletPhysicalTasks })
      return
    }

    if (method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    await fulfillJson(route, {})
  })
}

async function login(page: Page) {
  await page.goto("/login")
  await page.getByRole("button", { name: "中文" }).click()
  await page.getByLabel("邮箱").fill(viewer.email)
  await page.getByLabel("密码").fill("admin123")
  await page.getByRole("button", { name: "登录" }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

test("enterprise idp outcome should go to alerts when approvals are pending", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-1",
          tenant_id: viewer.tenant_id,
          email: "pending.user@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise#idp")
  const outcomeCard = page.getByTestId("enterprise-idp-outcome")
  await expect(outcomeCard).toBeVisible()

  const outcomeAction = outcomeCard.getByTestId("enterprise-idp-outcome-action")
  await expect(outcomeAction).toBeVisible()
  await outcomeAction.click()

  await expect(page).toHaveURL(/\/enterprise#alerts$/)
})

test("enterprise idp outcome should go to wallet when directory/policy are ready", async ({ page }) => {
  await setupApiMocks(page, buildScenario())

  await login(page)
  await page.goto("/enterprise#idp")
  const outcomeCard = page.getByTestId("enterprise-idp-outcome")
  await expect(outcomeCard).toBeVisible()

  const outcomeAction = outcomeCard.getByRole("link", { name: "去凭证发放" })
  await expect(outcomeAction).toBeVisible()
  await outcomeAction.click()
  await expect(page).toHaveURL(/\/wallet\?/)

  const nextURL = new URL(page.url())
  expect(nextURL.pathname).toBe("/wallet")
  expect(nextURL.searchParams.get("scenario")).toBe("employee_mobile")
  expect(nextURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(nextURL.searchParams.get("stage")).toBe("issuance")
  expect(nextURL.searchParams.get("tenant_id")).toBe(viewer.tenant_id)
})

test("enterprise alerts receipt recovery backflow link should keep context hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-pending-1",
          tenant_id: viewer.tenant_id,
          email: "receipt.pending@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?segment_hint=receipt_recovery&segment_status_hint=attention&approval_query_hint=target-42#alerts")

  const recoveryCard = page.getByTestId("enterprise-alerts-receipt-recovery")
  await expect(recoveryCard).toBeVisible()

  const retryBackflowLink = page.getByTestId("enterprise-alerts-receipt-retry-link")
  await expect(retryBackflowLink).toBeVisible()

  const href = await retryBackflowLink.getAttribute("href")
  expect(href).toBeTruthy()

  const nextURL = new URL(href ?? "", "http://localhost")
  expect(nextURL.pathname).toBe("/wallet")
  expect(nextURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("attention")
  expect(nextURL.searchParams.get("receipt_recovery_action_hint")).toBe("retry_delivery")
  expect(nextURL.searchParams.get("target_hint")).toBe("target-42")
  expect(nextURL.searchParams.get("target_id")).toBe("target-42")
})

test("enterprise alerts receipt recovery repair and closed links should keep context hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-pending-2",
          tenant_id: viewer.tenant_id,
          email: "receipt.pending.repair@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?segment_hint=receipt_recovery&segment_status_hint=attention&approval_query_hint=target-99#alerts")

  const repairLink = page.getByRole("link", { name: "结论：继续状态修复" })
  await expect(repairLink).toBeVisible()
  const repairHref = await repairLink.getAttribute("href")
  expect(repairHref).toBeTruthy()
  const repairURL = new URL(repairHref ?? "", "http://localhost")
  expect(repairURL.pathname).toBe("/wallet")
  expect(repairURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(repairURL.searchParams.get("segment_status_hint")).toBe("attention")
  expect(repairURL.searchParams.get("receipt_recovery_action_hint")).toBe("repair_pass_status")
  expect(repairURL.searchParams.get("target_hint")).toBe("target-99")
  expect(repairURL.searchParams.get("target_id")).toBe("target-99")

  const closedLink = page.getByRole("link", { name: "结论：复核已收口" })
  await expect(closedLink).toBeVisible()
  const closedHref = await closedLink.getAttribute("href")
  expect(closedHref).toBeTruthy()
  const closedURL = new URL(closedHref ?? "", "http://localhost")
  expect(closedURL.pathname).toBe("/wallet")
  expect(closedURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(closedURL.searchParams.get("segment_status_hint")).toBe("ready")
  expect(closedURL.searchParams.get("receipt_recovery_action_hint")).toBe("review_closed")
  expect(closedURL.searchParams.get("target_hint")).toBe("target-99")
  expect(closedURL.searchParams.get("target_id")).toBe("target-99")
})

test("enterprise alerts receipt recovery explicit ready hint should keep badge while retry link follows live blockers", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-recovery-conflict-1",
          tenant_id: viewer.tenant_id,
          email: "recovery.conflict.1@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?segment_hint=receipt_recovery&segment_status_hint=ready&approval_query_hint=target-ready-hint#alerts")

  const recoveryCard = page.getByTestId("enterprise-alerts-receipt-recovery")
  await expect(recoveryCard).toBeVisible()
  await expect(recoveryCard.getByText("已承接")).toBeVisible()
  await expect(recoveryCard.getByText("1 个待处理项")).toBeVisible()

  await recoveryCard.getByRole("link", { name: "结论：继续重发失败通道" }).click()
  await expect(page).toHaveURL(/\/wallet\?/)
  const nextURL = new URL(page.url())
  expect(nextURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("attention")
  expect(nextURL.searchParams.get("target_hint")).toBe("target-ready-hint")
})

test("enterprise alerts receipt recovery explicit pending hint should fallback retry link to ready when blockers are cleared", async ({
  page,
}) => {
  await setupApiMocks(page, buildScenario())

  await login(page)
  await page.goto("/enterprise?segment_hint=receipt_recovery&segment_status_hint=pending&approval_query_hint=target-pending-hint#alerts")

  const recoveryCard = page.getByTestId("enterprise-alerts-receipt-recovery")
  await expect(recoveryCard).toBeVisible()
  await expect(recoveryCard.getByText("待补齐")).toBeVisible()
  await expect(recoveryCard.getByText("可回发放页收口")).toBeVisible()

  await recoveryCard.getByRole("link", { name: "结论：继续重发失败通道" }).click()
  await expect(page).toHaveURL(/\/wallet\?/)
  const nextURL = new URL(page.url())
  expect(nextURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("ready")
  expect(nextURL.searchParams.get("target_id")).toBe("target-pending-hint")
})

test("enterprise sync worker review link should keep review context hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 3,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 8,
          last_threshold: 4,
          last_processed: 11,
          last_applied: 6,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/enterprise?sync_focus_hint=worker_alert&worker_filter_hint=hot&worker_query_hint=tenant-sudirman&worker_review_stage_hint=issuance&worker_review_status_hint=handled#sync"
  )

  const workerReview = page.getByTestId("enterprise-sync-worker-review")
  await expect(workerReview).toBeVisible()
  await expect(workerReview).toContainText("已从凭证发放回流到导入与同步")

  const reviewAlertsLink = page.getByTestId("enterprise-sync-worker-review-alerts-link")
  await expect(reviewAlertsLink).toBeVisible()

  const href = await reviewAlertsLink.getAttribute("href")
  expect(href).toBeTruthy()

  const nextURL = new URL(href ?? "", "http://localhost")
  expect(nextURL.pathname).toBe("/enterprise")
  expect(nextURL.hash).toBe("#alerts")
  expect(nextURL.searchParams.get("alerts_view_hint")).toBe("directory_exceptions")
  expect(nextURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(nextURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_review_stage_hint")).toBeNull()
  expect(nextURL.searchParams.get("worker_review_status_hint")).toBeNull()
})

test("wallet receipt recovery action hint should show retry recommendation summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-recovery-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-1",
          object_id: "obj-recovery-1",
          status: "issued",
          save_link: "https://wallet.example.com/pass-recovery-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-recovery-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-recovery-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-1",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&segment_hint=receipt_recovery&segment_status_hint=attention&receipt_recovery_action_hint=retry_delivery"
  )

  await expect(page.getByText("来源：企业页复核结论。建议继续重发失败通道：当前可批量重发 1 条，可修复状态 1 张。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" }).first()).toBeVisible()
  await expect(page.getByRole("button", { name: "批量状态修复（1）" }).first()).toBeVisible()
})

test("wallet receipt recovery action hint should show repair recommendation summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-recovery-repair-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-repair-1",
          object_id: "obj-recovery-repair-1",
          status: "issued",
          save_link: "https://wallet.example.com/pass-recovery-repair-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-recovery-repair-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-recovery-repair-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-repair-1",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&segment_hint=receipt_recovery&segment_status_hint=attention&receipt_recovery_action_hint=repair_pass_status"
  )

  await expect(page.getByText("来源：企业页复核结论。建议继续状态修复：当前可修复状态 1 张，可批量重发 1 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量状态修复（1）" }).first()).toBeVisible()
})

test("wallet receipt recovery action hint should show review closed summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-recovery-closed-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-closed-1",
          object_id: "obj-recovery-closed-1",
          status: "suspended",
          save_link: "https://wallet.example.com/pass-recovery-closed-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-recovery-closed-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-recovery-closed-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-closed-1",
          status: "failed",
          reason: "receiver unavailable",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&segment_hint=receipt_recovery&segment_status_hint=attention&receipt_recovery_action_hint=review_closed"
  )

  await expect(
    page.getByText("来源：企业页复核结论。复核已收口：当前失败回执 1 条，若仍需处理可继续重发或状态修复。")
  ).toBeVisible()
})

test("wallet receipt recovery flow without action hint should show matched-target guidance", async ({ page }) => {
  await setupApiMocks(page, buildScenario())

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&segment_hint=receipt_recovery&segment_status_hint=attention&target_id=emp-1"
  )

  await expect(page.getByText("来源：企业页。已命中 1 条目标凭证，已直达外部投递对象，可直接补发或重发失败通道。")).toBeVisible()
})

test("wallet receipt recovery flow without action hint should show fallback guidance when no target hints", async ({ page }) => {
  await setupApiMocks(page, buildScenario())

  await login(page)
  await page.goto("/wallet?from=enterprise&flow=sync_to_access&stage=issuance&segment_hint=receipt_recovery&segment_status_hint=attention")

  await expect(page.getByText("来源：企业页。已回流到回执失败恢复闭环，请在“最近外部投递回执”继续重发或状态修复。")).toBeVisible()
})

test("wallet receipt recovery review link should carry enterprise segment hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-recovery-link-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-link-1",
          object_id: "obj-recovery-link-1",
          status: "issued",
          save_link: "https://wallet.example.com/pass-recovery-link-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-recovery-link-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-recovery-link-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-link-1",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&segment_hint=receipt_recovery&target_id=emp-recovery-link-1"
  )

  const reviewLink = page.getByRole("link", { name: "回企业页复核回执失败" }).first()
  await expect(reviewLink).toBeVisible()
  const href = await reviewLink.getAttribute("href")
  expect(href).toBeTruthy()
  const nextURL = new URL(href ?? "", "http://localhost")
  expect(nextURL.pathname).toBe("/enterprise")
  expect(nextURL.hash).toBe("#alerts")
  expect(nextURL.searchParams.get("from")).toBe("enterprise")
  expect(nextURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(nextURL.searchParams.get("tenant_id")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("alerts_view_hint")).toBe("directory_exceptions")
  expect(nextURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("attention")
  expect(nextURL.searchParams.get("approval_query_hint")).toBe("emp-recovery-link-1")
  expect(nextURL.searchParams.get("target_hint")).toBe("emp-recovery-link-1")
})

test("enterprise receipt recovery roundtrip should keep target hints across repeated backflow", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-recovery-roundtrip-1",
          tenant_id: viewer.tenant_id,
          email: "recovery.roundtrip@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
      walletPasses: [
        {
          id: "pass-recovery-roundtrip-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-roundtrip",
          object_id: "obj-recovery-roundtrip",
          status: "issued",
          save_link: "https://wallet.example.com/pass-recovery-roundtrip-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-recovery-roundtrip-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-recovery-roundtrip-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-roundtrip",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/enterprise?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&segment_hint=receipt_recovery&segment_status_hint=attention&approval_query_hint=emp-recovery-roundtrip&target_hint=emp-recovery-roundtrip#alerts"
  )

  const retryBackflowLink = page.getByTestId("enterprise-alerts-receipt-retry-link")
  await expect(retryBackflowLink).toBeVisible()
  await retryBackflowLink.click()

  await expect(page).toHaveURL(/\/wallet\?.*receipt_recovery_action_hint=retry_delivery/)
  const firstWalletURL = new URL(page.url())
  expect(firstWalletURL.searchParams.get("target_id")).toBe("emp-recovery-roundtrip")
  expect(firstWalletURL.searchParams.get("target_hint")).toBe("emp-recovery-roundtrip")

  const reviewLink = page.getByRole("link", { name: "回企业页复核回执失败" }).first()
  await expect(reviewLink).toBeVisible()
  await reviewLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  const backToEnterpriseURL = new URL(page.url())
  expect(backToEnterpriseURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(backToEnterpriseURL.searchParams.get("segment_status_hint")).toBe("attention")
  expect(backToEnterpriseURL.searchParams.get("target_hint")).toBe("emp-recovery-roundtrip")
  expect(backToEnterpriseURL.searchParams.get("approval_query_hint")).toBe("emp-recovery-roundtrip")

  const retryBackflowAgain = page.getByTestId("enterprise-alerts-receipt-retry-link")
  await expect(retryBackflowAgain).toBeVisible()
  await retryBackflowAgain.click()

  await expect(page).toHaveURL(/\/wallet\?.*receipt_recovery_action_hint=retry_delivery/)
  const secondWalletURL = new URL(page.url())
  expect(secondWalletURL.searchParams.get("from")).toBe("enterprise")
  expect(secondWalletURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(secondWalletURL.searchParams.get("stage")).toBe("issuance")
  expect(secondWalletURL.searchParams.get("tenant_id")).toBe("tenant-sudirman")
  expect(secondWalletURL.searchParams.get("target_id")).toBe("emp-recovery-roundtrip")
  expect(secondWalletURL.searchParams.get("target_hint")).toBe("emp-recovery-roundtrip")
})

test("wallet receipt recovery missing-target roundtrip should keep explicit target hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [],
      walletDeliveries: [],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&segment_hint=receipt_recovery&segment_status_hint=attention&target_id=emp-recovery-missing&target_hint=emp-recovery-missing"
  )

  await expect(page.getByText("来源：企业页。未找到该对象的既有凭证，已预填单发对象，可直接创建补发。")).toBeVisible()

  const reviewLink = page.getByRole("link", { name: "回企业页复核回执失败" }).first()
  await expect(reviewLink).toBeVisible()
  await reviewLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  const enterpriseURL = new URL(page.url())
  expect(enterpriseURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(enterpriseURL.searchParams.get("segment_status_hint")).toBe("pending")
  expect(enterpriseURL.searchParams.get("target_hint")).toBe("emp-recovery-missing")
  expect(enterpriseURL.searchParams.get("approval_query_hint")).toBe("emp-recovery-missing")

  const retryBackflowLink = page.getByTestId("enterprise-alerts-receipt-retry-link")
  await expect(retryBackflowLink).toBeVisible()
  await retryBackflowLink.click()

  await expect(page).toHaveURL(/\/wallet\?.*receipt_recovery_action_hint=retry_delivery/)
  const walletRetryURL = new URL(page.url())
  expect(walletRetryURL.searchParams.get("target_id")).toBe("emp-recovery-missing")
  expect(walletRetryURL.searchParams.get("target_hint")).toBe("emp-recovery-missing")
})

test("wallet receipt recovery retry then close roundtrip should keep ready status and target hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-recovery-ready-roundtrip-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-ready-roundtrip",
          object_id: "obj-recovery-ready-roundtrip",
          status: "issued",
          save_link: "https://wallet.example.com/pass-recovery-ready-roundtrip-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-recovery-ready-roundtrip-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-recovery-ready-roundtrip-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-ready-roundtrip",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&segment_hint=receipt_recovery&segment_status_hint=attention&target_id=emp-recovery-ready-roundtrip&target_hint=emp-recovery-ready-roundtrip"
  )

  const retryButtons = page.getByRole("button", { name: "批量重发失败通道（1）" })
  await expect(retryButtons).toHaveCount(2)
  await retryButtons.first().click()
  await expect(page.getByText("已批量重发 1 条失败通道，成功 1 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（0）" })).toHaveCount(2)

  const reviewLink = page.getByRole("link", { name: "回企业页复核回执失败" }).first()
  await expect(reviewLink).toBeVisible()
  await reviewLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  const enterpriseURL = new URL(page.url())
  expect(enterpriseURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(enterpriseURL.searchParams.get("segment_status_hint")).toBe("ready")
  expect(enterpriseURL.searchParams.get("target_hint")).toBe("emp-recovery-ready-roundtrip")
  expect(enterpriseURL.searchParams.get("approval_query_hint")).toBe("emp-recovery-ready-roundtrip")

  const closeBackflowLink = page.getByRole("link", { name: "结论：复核已收口" })
  await expect(closeBackflowLink).toBeVisible()
  await closeBackflowLink.click()

  await expect(page).toHaveURL(/\/wallet\?.*receipt_recovery_action_hint=review_closed/)
  const walletClosedURL = new URL(page.url())
  expect(walletClosedURL.searchParams.get("segment_status_hint")).toBe("ready")
  expect(walletClosedURL.searchParams.get("target_id")).toBe("emp-recovery-ready-roundtrip")
  expect(walletClosedURL.searchParams.get("target_hint")).toBe("emp-recovery-ready-roundtrip")
  await expect(
    page.getByText("来源：企业页复核结论。复核已收口：当前失败回执 0 条，若仍需处理可继续重发或状态修复。")
  ).toBeVisible()
})

test("wallet receipt recovery repair roundtrip should keep attention status and target hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-recovery-repair-roundtrip-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-repair-roundtrip",
          object_id: "obj-recovery-repair-roundtrip",
          status: "suspended",
          save_link: "https://wallet.example.com/pass-recovery-repair-roundtrip-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-recovery-repair-roundtrip-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-recovery-repair-roundtrip-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-recovery-repair-roundtrip",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&segment_hint=receipt_recovery&segment_status_hint=attention&target_id=emp-recovery-repair-roundtrip&target_hint=emp-recovery-repair-roundtrip"
  )

  const repairButtons = page.getByRole("button", { name: "批量状态修复（1）" })
  await expect(repairButtons).toHaveCount(2)
  await repairButtons.first().click()
  await expect(page.getByText("已按失败回执批量修复 1 张凭证状态，成功 1 张。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量状态修复（0）" })).toHaveCount(2)
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)

  const reviewLink = page.getByRole("link", { name: "回企业页复核回执失败" }).first()
  await expect(reviewLink).toBeVisible()
  await reviewLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  const enterpriseURL = new URL(page.url())
  expect(enterpriseURL.searchParams.get("segment_hint")).toBe("receipt_recovery")
  expect(enterpriseURL.searchParams.get("segment_status_hint")).toBe("attention")
  expect(enterpriseURL.searchParams.get("target_hint")).toBe("emp-recovery-repair-roundtrip")
  expect(enterpriseURL.searchParams.get("approval_query_hint")).toBe("emp-recovery-repair-roundtrip")

  const repairBackflowLink = page.getByRole("link", { name: "结论：继续状态修复" })
  await expect(repairBackflowLink).toBeVisible()
  await repairBackflowLink.click()

  await expect(page).toHaveURL(/\/wallet\?.*receipt_recovery_action_hint=repair_pass_status/)
  const walletRepairURL = new URL(page.url())
  expect(walletRepairURL.searchParams.get("segment_status_hint")).toBe("ready")
  expect(walletRepairURL.searchParams.get("target_id")).toBe("emp-recovery-repair-roundtrip")
  expect(walletRepairURL.searchParams.get("target_hint")).toBe("emp-recovery-repair-roundtrip")
  await expect(page.getByText("来源：企业页复核结论。建议继续状态修复：")).toBeVisible()
})

test("enterprise alerts batch approval should update pending count and show success summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-pending-a",
          tenant_id: viewer.tenant_id,
          email: "pending.a@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-pending-b",
          tenant_id: viewer.tenant_id,
          email: "pending.b@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const batchApproveButton = page.getByRole("button", { name: "批量批准 pending（2）" })
  await expect(batchApproveButton).toBeVisible()
  await batchApproveButton.click()

  await expect(page.getByText("批量批准完成：成功 2 条，失败 0 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量批准 pending（0）" })).toBeVisible()
  await expect(page.getByRole("button", { name: /^pending（\d+）$/ })).toHaveCount(0)
  await expect(page.getByRole("button", { name: /^approved（2）$/ })).toBeVisible()
})

test("enterprise alerts batch approval should keep failed pending records and show partial failure summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-pending-partial-a",
          tenant_id: viewer.tenant_id,
          email: "pending.partial.a@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-pending-partial-b",
          tenant_id: viewer.tenant_id,
          email: "pending.partial.b@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
      approvalReviewErrorIDs: ["approval-pending-partial-b"],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const batchApproveButton = page.getByRole("button", { name: "批量批准 pending（2）" })
  await expect(batchApproveButton).toBeVisible()
  await batchApproveButton.click()

  await expect(page.getByText("批量批准完成：成功 1 条，失败 1 条。")).toBeVisible()
  await expect(page.getByText("部分审批记录处理失败，请在台账中复核失败项后重试。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量批准 pending（1）" })).toBeVisible()
  await expect(page.getByRole("button", { name: /^pending（1）$/ })).toBeVisible()
  await expect(page.getByRole("button", { name: /^approved（1）$/ })).toBeVisible()
})

test("enterprise alerts single approval action should keep pending state when review api fails", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-single-review-fail",
          tenant_id: viewer.tenant_id,
          email: "single.review.fail@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-single-review-keep",
          tenant_id: viewer.tenant_id,
          email: "single.review.keep@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
      approvalReviewErrorIDs: ["approval-single-review-fail"],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const targetRow = page.locator("tr", { hasText: "single.review.fail@sudirman.co" })
  await expect(targetRow).toBeVisible()
  await targetRow.getByRole("button", { name: "批准" }).click()

  await expect(page.getByText("mock approval review failed: approval-single-review-fail")).toBeVisible()
  await expect(targetRow.getByRole("button", { name: "批准" })).toBeVisible()
  await expect(page.getByRole("button", { name: "批量批准 pending（2）" })).toBeVisible()
})

test("enterprise alerts single approval action should update counts when review api succeeds", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-single-approve-ok",
          tenant_id: viewer.tenant_id,
          email: "single.approve.ok@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-single-approve-keep",
          tenant_id: viewer.tenant_id,
          email: "single.approve.keep@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const targetRow = page.locator("tr", { hasText: "single.approve.ok@sudirman.co" })
  await expect(targetRow).toBeVisible()
  await targetRow.getByRole("button", { name: "批准" }).click()

  await expect(page.getByText("审批 approval-single-approve-ok 已批准，并已刷新当前企业目录状态。")).toBeVisible()
  await expect(page.getByRole("button", { name: /^pending（1）$/ })).toBeVisible()
  await expect(page.getByRole("button", { name: /^approved（1）$/ })).toBeVisible()
  await expect(targetRow.getByText("approved")).toBeVisible()
})

test("enterprise alerts batch reject should update pending count and show success summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-reject-a",
          tenant_id: viewer.tenant_id,
          email: "reject.a@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-reject-b",
          tenant_id: viewer.tenant_id,
          email: "reject.b@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const batchRejectButton = page.getByRole("button", { name: "批量拒绝 pending（2）" })
  await expect(batchRejectButton).toBeVisible()
  await batchRejectButton.click()

  await expect(page.getByText("批量拒绝完成：成功 2 条，失败 0 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量拒绝 pending（0）" })).toBeVisible()
  await expect(page.getByRole("button", { name: /^pending（\d+）$/ })).toHaveCount(0)
  await expect(page.getByRole("button", { name: /^rejected（2）$/ })).toBeVisible()
})

test("enterprise alerts single reject action should keep pending state when review api fails", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-single-reject-fail",
          tenant_id: viewer.tenant_id,
          email: "single.reject.fail@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-single-reject-keep",
          tenant_id: viewer.tenant_id,
          email: "single.reject.keep@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
      approvalReviewErrorIDs: ["approval-single-reject-fail"],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const targetRow = page.locator("tr", { hasText: "single.reject.fail@sudirman.co" })
  await expect(targetRow).toBeVisible()
  await targetRow.getByRole("button", { name: "拒绝" }).click()

  await expect(page.getByText("mock approval review failed: approval-single-reject-fail")).toBeVisible()
  await expect(targetRow.getByRole("button", { name: "拒绝" })).toBeVisible()
  await expect(page.getByRole("button", { name: "批量拒绝 pending（2）" })).toBeVisible()
})

test("enterprise alerts single reject action should update counts when review api succeeds", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-single-reject-ok",
          tenant_id: viewer.tenant_id,
          email: "single.reject.ok@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-single-reject-keep",
          tenant_id: viewer.tenant_id,
          email: "single.reject.keep2@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const targetRow = page.locator("tr", { hasText: "single.reject.ok@sudirman.co" })
  await expect(targetRow).toBeVisible()
  await targetRow.getByRole("button", { name: "拒绝" }).click()

  await expect(page.getByText("审批 approval-single-reject-ok 已拒绝，并已刷新当前企业目录状态。")).toBeVisible()
  await expect(page.getByRole("button", { name: /^pending（1）$/ })).toBeVisible()
  await expect(page.getByRole("button", { name: /^rejected（1）$/ })).toBeVisible()
  await expect(targetRow.getByText("rejected")).toBeVisible()
})

test("enterprise alerts batch reject should preserve pending records and show all-failed summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-reject-fail-a",
          tenant_id: viewer.tenant_id,
          email: "reject.fail.a@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-reject-fail-b",
          tenant_id: viewer.tenant_id,
          email: "reject.fail.b@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
      ],
      approvalReviewErrorIDs: ["approval-reject-fail-a", "approval-reject-fail-b"],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const batchRejectButton = page.getByRole("button", { name: "批量拒绝 pending（2）" })
  await expect(batchRejectButton).toBeVisible()
  await batchRejectButton.click()

  await expect(page.getByText("批量拒绝完成：成功 0 条，失败 2 条。")).toBeVisible()
  await expect(page.getByText("部分审批记录处理失败，请在台账中复核失败项后重试。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量拒绝 pending（2）" })).toBeVisible()
  await expect(page.getByRole("button", { name: /^pending（2）$/ })).toBeVisible()
})

test("enterprise alerts batch approval should only process filtered pending subset", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-filter-approve-focus",
          tenant_id: viewer.tenant_id,
          email: "focus.batch.approve@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-filter-approve-keep",
          tenant_id: viewer.tenant_id,
          email: "keep.batch.approve@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-filter-approve-ready",
          tenant_id: viewer.tenant_id,
          email: "ready.batch.approve@sudirman.co",
          status: "approved",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const approvalQueryInput = page.getByPlaceholder("按邮箱 / external_id / 审批ID筛选")
  await approvalQueryInput.fill("focus.batch.approve")

  const batchApproveButton = page.getByRole("button", { name: "批量批准 pending（1）" })
  await expect(batchApproveButton).toBeVisible()
  await batchApproveButton.click()

  await expect(page.getByText("批量批准完成：成功 1 条，失败 0 条。")).toBeVisible()
  await page.getByRole("button", { name: "清空" }).click()
  await expect(page.getByRole("button", { name: /^pending（1）$/ })).toBeVisible()
  await expect(page.getByRole("button", { name: /^approved（2）$/ })).toBeVisible()
})

test("enterprise alerts batch reject should only process filtered pending subset", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-filter-reject-focus",
          tenant_id: viewer.tenant_id,
          email: "focus.batch.reject@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-filter-reject-keep",
          tenant_id: viewer.tenant_id,
          email: "keep.batch.reject@sudirman.co",
          status: "pending",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-filter-reject-ready",
          tenant_id: viewer.tenant_id,
          email: "ready.batch.reject@sudirman.co",
          status: "rejected",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const approvalQueryInput = page.getByPlaceholder("按邮箱 / external_id / 审批ID筛选")
  await approvalQueryInput.fill("focus.batch.reject")

  const batchRejectButton = page.getByRole("button", { name: "批量拒绝 pending（1）" })
  await expect(batchRejectButton).toBeVisible()
  await batchRejectButton.click()

  await expect(page.getByText("批量拒绝完成：成功 1 条，失败 0 条。")).toBeVisible()
  await page.getByRole("button", { name: "清空" }).click()
  await expect(page.getByRole("button", { name: /^pending（1）$/ })).toBeVisible()
  await expect(page.getByRole("button", { name: /^rejected（2）$/ })).toBeVisible()
})

test("enterprise alerts batch external sync mark should clear failed count and show success summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-sync-failed-a",
          tenant_id: viewer.tenant_id,
          email: "sync.failed.a@sudirman.co",
          status: "approved",
          external_sync_status: "failed",
          external_sync_last_error: "mock timeout",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-sync-failed-b",
          tenant_id: viewer.tenant_id,
          email: "sync.failed.b@sudirman.co",
          status: "rejected",
          external_sync_status: "failed",
          external_sync_last_error: "mock upstream rejected",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const batchMarkSyncedButton = page.getByRole("button", { name: "批量标记已回写（2）" }).first()
  await expect(batchMarkSyncedButton).toBeVisible()
  await batchMarkSyncedButton.click()

  await expect(page.getByText("批量更新外部回写状态完成（synced）：成功 2 条，失败 0 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "失败（0）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "成功（2）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "批量标记已回写（0）" }).first()).toBeVisible()
})

test("enterprise alerts single external sync mark should keep failed state when api fails", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-single-sync-fail",
          tenant_id: viewer.tenant_id,
          email: "single.sync.fail@sudirman.co",
          status: "approved",
          external_sync_status: "failed",
          external_sync_last_error: "mock timeout",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-single-sync-stable",
          tenant_id: viewer.tenant_id,
          email: "single.sync.stable@sudirman.co",
          status: "approved",
          external_sync_status: "synced",
          created_at: now,
          updated_at: now,
        },
      ],
      approvalExternalSyncErrorIDs: ["approval-single-sync-fail"],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const targetRow = page.locator("tr", { hasText: "single.sync.fail@sudirman.co" })
  await expect(targetRow).toBeVisible()
  await targetRow.getByRole("button", { name: "标记已回写" }).click()

  await expect(page.getByText("mock approval external sync failed: approval-single-sync-fail")).toBeVisible()
  await expect(targetRow.getByRole("button", { name: "标记已回写" })).toBeVisible()
  await expect(page.getByRole("button", { name: "失败（1）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "成功（1）" })).toBeVisible()
})

test("enterprise alerts single external sync mark should update counts when api succeeds", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-single-sync-ok",
          tenant_id: viewer.tenant_id,
          email: "single.sync.ok@sudirman.co",
          status: "approved",
          external_sync_status: "failed",
          external_sync_last_error: "mock timeout",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-single-sync-ready",
          tenant_id: viewer.tenant_id,
          email: "single.sync.ready@sudirman.co",
          status: "approved",
          external_sync_status: "synced",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const targetRow = page.locator("tr", { hasText: "single.sync.ok@sudirman.co" })
  await expect(targetRow).toBeVisible()
  await targetRow.getByRole("button", { name: "标记已回写" }).click()

  await expect(page.getByText("审批 approval-single-sync-ok 外部回写已标记为 synced。")).toBeVisible()
  await expect(page.getByRole("button", { name: "失败（0）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "成功（2）" })).toBeVisible()
  await expect(targetRow.getByRole("button", { name: "标记已回写" })).toHaveCount(0)
})

test("enterprise alerts batch external sync mark should keep failed items when partial update fails", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-sync-partial-a",
          tenant_id: viewer.tenant_id,
          email: "sync.partial.a@sudirman.co",
          status: "approved",
          external_sync_status: "failed",
          external_sync_last_error: "mock timeout",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-sync-partial-b",
          tenant_id: viewer.tenant_id,
          email: "sync.partial.b@sudirman.co",
          status: "rejected",
          external_sync_status: "failed",
          external_sync_last_error: "mock upstream rejected",
          created_at: now,
          updated_at: now,
        },
      ],
      approvalExternalSyncErrorIDs: ["approval-sync-partial-b"],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const batchMarkSyncedButton = page.getByRole("button", { name: "批量标记已回写（2）" }).first()
  await expect(batchMarkSyncedButton).toBeVisible()
  await batchMarkSyncedButton.click()

  await expect(page.getByText("批量更新外部回写状态完成（synced）：成功 1 条，失败 1 条。")).toBeVisible()
  await expect(page.getByText("部分外部回写状态更新失败，请在台账中复核失败项后重试。")).toBeVisible()
  await expect(page.getByRole("button", { name: "失败（1）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "成功（1）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "批量标记已回写（1）" }).first()).toBeVisible()
})

test("enterprise alerts batch external sync mark should clear pending count and show success summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-sync-pending-a",
          tenant_id: viewer.tenant_id,
          email: "sync.pending.a@sudirman.co",
          status: "approved",
          external_sync_status: "processing",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-sync-synced-a",
          tenant_id: viewer.tenant_id,
          email: "sync.synced.a@sudirman.co",
          status: "approved",
          external_sync_status: "synced",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const batchMarkSyncedButton = page.getByRole("button", { name: "批量标记已回写（1）" }).first()
  await expect(batchMarkSyncedButton).toBeVisible()
  await batchMarkSyncedButton.click()

  await expect(page.getByText("批量更新外部回写状态完成（synced）：成功 1 条，失败 0 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "进行中（0）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "成功（2）" })).toBeVisible()
})

test("enterprise alerts batch external sync mark should preserve pending sync states when all updates fail", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-sync-pending-fail-a",
          tenant_id: viewer.tenant_id,
          email: "sync.pending.fail.a@sudirman.co",
          status: "approved",
          external_sync_status: "processing",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-sync-pending-fail-b",
          tenant_id: viewer.tenant_id,
          email: "sync.pending.fail.b@sudirman.co",
          status: "rejected",
          external_sync_status: "processing",
          created_at: now,
          updated_at: now,
        },
      ],
      approvalExternalSyncErrorIDs: ["approval-sync-pending-fail-a", "approval-sync-pending-fail-b"],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const batchMarkSyncedButton = page.getByRole("button", { name: "批量标记已回写（2）" }).first()
  await expect(batchMarkSyncedButton).toBeVisible()
  await batchMarkSyncedButton.click()

  await expect(page.getByText("批量更新外部回写状态完成（synced）：成功 0 条，失败 2 条。")).toBeVisible()
  await expect(page.getByText("部分外部回写状态更新失败，请在台账中复核失败项后重试。")).toBeVisible()
  await expect(page.getByRole("button", { name: "进行中（2）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "批量标记已回写（2）" }).first()).toBeVisible()
})

test("enterprise alerts batch external sync mark should only process filtered markable subset", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      approvals: [
        {
          id: "approval-filter-sync-focus",
          tenant_id: viewer.tenant_id,
          email: "focus.batch.sync@sudirman.co",
          status: "approved",
          external_sync_status: "failed",
          external_sync_last_error: "mock timeout",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-filter-sync-keep",
          tenant_id: viewer.tenant_id,
          email: "keep.batch.sync@sudirman.co",
          status: "rejected",
          external_sync_status: "failed",
          external_sync_last_error: "mock upstream rejected",
          created_at: now,
          updated_at: now,
        },
        {
          id: "approval-filter-sync-ready",
          tenant_id: viewer.tenant_id,
          email: "ready.batch.sync@sudirman.co",
          status: "approved",
          external_sync_status: "synced",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?alerts_view_hint=approval_backlog#alerts")

  const approvalQueryInput = page.getByPlaceholder("按邮箱 / external_id / 审批ID筛选")
  await approvalQueryInput.fill("focus.batch.sync")

  const batchMarkSyncedButton = page.getByRole("button", { name: "批量标记已回写（1）" }).first()
  await expect(batchMarkSyncedButton).toBeVisible()
  await batchMarkSyncedButton.click()

  await expect(page.getByText("批量更新外部回写状态完成（synced）：成功 1 条，失败 0 条。")).toBeVisible()
  await page.getByRole("button", { name: "清空" }).click()
  await expect(page.getByRole("button", { name: "失败（1）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "成功（2）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "批量标记已回写（1）" }).first()).toBeVisible()
})

test("enterprise sync mainflow issuance segment link should carry wallet segment hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [],
    })
  )

  await login(page)
  await page.goto("/enterprise#sync")

  const issuanceSegmentLink = page.getByRole("link", { name: "去凭证发放承接" }).first()
  await expect(issuanceSegmentLink).toBeVisible()
  await issuanceSegmentLink.click()

  await expect(page).toHaveURL(/\/wallet\?/)
  const nextURL = new URL(page.url())
  expect(nextURL.pathname).toBe("/wallet")
  expect(nextURL.searchParams.get("from")).toBe("enterprise")
  expect(nextURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(nextURL.searchParams.get("segment_hint")).toBe("issuance_receipt")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("attention")
})

test("enterprise sync mainflow directory segment link should carry pending status when directory is empty", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      employees: [],
    })
  )

  await login(page)
  await page.goto("/enterprise#sync")

  const directorySegmentLink = page.getByRole("link", { name: "去员工与用户组承接" }).first()
  await expect(directorySegmentLink).toBeVisible()
  await directorySegmentLink.click()

  await expect(page).toHaveURL(/\/access\/directory\?/)
  const nextURL = new URL(page.url())
  expect(nextURL.pathname).toBe("/access/directory")
  expect(nextURL.searchParams.get("segment_hint")).toBe("directory_usage")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("pending")
})

test("enterprise sync mainflow directory segment link should carry access directory hints", async ({ page }) => {
  await setupApiMocks(page, buildScenario())

  await login(page)
  await page.goto("/enterprise#sync")

  const directorySegmentLink = page.getByRole("link", { name: "去员工与用户组承接" }).first()
  await expect(directorySegmentLink).toBeVisible()
  await directorySegmentLink.click()

  await expect(page).toHaveURL(/\/access\/directory\?/)
  const nextURL = new URL(page.url())
  expect(nextURL.pathname).toBe("/access/directory")
  expect(nextURL.searchParams.get("segment_hint")).toBe("directory_usage")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("ready")
  expect(nextURL.searchParams.get("sync_job_id")).toBe("job-1")
  expect(nextURL.searchParams.get("sync_source")).toBe("hris")
  expect(nextURL.searchParams.get("sync_status")).toBe("completed")
})

test("enterprise sync mainflow policy segment link should carry pending status when groups are missing", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      userGroups: [],
    })
  )

  await login(page)
  await page.goto("/enterprise#sync")

  const policySegmentLink = page.getByRole("link", { name: "去权限策略承接" }).first()
  await expect(policySegmentLink).toBeVisible()
  await policySegmentLink.click()

  await expect(page).toHaveURL(/\/access\/policies\?/)
  const nextURL = new URL(page.url())
  expect(nextURL.pathname).toBe("/access/policies")
  expect(nextURL.searchParams.get("segment_hint")).toBe("policy_delivery")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("pending")
})

test("enterprise sync mainflow policy segment link should carry access policy hints", async ({ page }) => {
  await setupApiMocks(page, buildScenario())

  await login(page)
  await page.goto("/enterprise#sync")

  const policySegmentLink = page.getByRole("link", { name: "去权限策略承接" }).first()
  await expect(policySegmentLink).toBeVisible()
  await policySegmentLink.click()

  await expect(page).toHaveURL(/\/access\/policies\?/)
  const nextURL = new URL(page.url())
  expect(nextURL.pathname).toBe("/access/policies")
  expect(nextURL.searchParams.get("segment_hint")).toBe("policy_delivery")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("ready")
  expect(nextURL.searchParams.get("sync_job_id")).toBe("job-1")
  expect(nextURL.searchParams.get("sync_source")).toBe("hris")
  expect(nextURL.searchParams.get("sync_status")).toBe("completed")
})

test("enterprise sync mainflow issuance segment link should carry pending status when policies are missing", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      policies: [],
    })
  )

  await login(page)
  await page.goto("/enterprise#sync")

  const issuanceSegmentLink = page.getByRole("link", { name: "去凭证发放承接" }).first()
  await expect(issuanceSegmentLink).toBeVisible()
  await issuanceSegmentLink.click()

  await expect(page).toHaveURL(/\/wallet\?/)
  const nextURL = new URL(page.url())
  expect(nextURL.pathname).toBe("/wallet")
  expect(nextURL.searchParams.get("segment_hint")).toBe("issuance_receipt")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("pending")
})

test("enterprise sync mainflow issuance segment link should carry ready status when issuance is connected", async ({ page }) => {
  await setupApiMocks(page, buildScenario())

  await login(page)
  await page.goto("/enterprise#sync")

  const issuanceSegmentLink = page.getByRole("link", { name: "去凭证发放承接" }).first()
  await expect(issuanceSegmentLink).toBeVisible()
  await issuanceSegmentLink.click()

  await expect(page).toHaveURL(/\/wallet\?/)
  const nextURL = new URL(page.url())
  expect(nextURL.pathname).toBe("/wallet")
  expect(nextURL.searchParams.get("segment_hint")).toBe("issuance_receipt")
  expect(nextURL.searchParams.get("segment_status_hint")).toBe("ready")
  expect(nextURL.searchParams.get("sync_job_id")).toBe("job-1")
  expect(nextURL.searchParams.get("sync_source")).toBe("hris")
  expect(nextURL.searchParams.get("sync_status")).toBe("completed")
})

test("wallet worker review backflow link should keep worker hints and navigate to enterprise sync", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 3,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 8,
          last_threshold: 4,
          last_processed: 11,
          last_applied: 6,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&worker_filter_hint=hot&worker_query_hint=tenant-sudirman&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_alert_failed=8&worker_alert_threshold=4"
  )

  const reviewLink = page.getByRole("link", { name: "处理完成后回导入与同步复核" }).first()
  await expect(reviewLink).toBeVisible()

  const href = await reviewLink.getAttribute("href")
  expect(href).toBeTruthy()
  const nextURL = new URL(href ?? "", "http://localhost")
  expect(nextURL.pathname).toBe("/enterprise")
  expect(nextURL.hash).toBe("#sync")
  expect(nextURL.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(nextURL.searchParams.get("worker_review_status_hint")).toBe("handled")
  expect(nextURL.searchParams.get("worker_review_stage_hint")).toBe("issuance")
  expect(nextURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(nextURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_alert_level")).toBe("hot")
  expect(nextURL.searchParams.get("worker_alert_tenant_id")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_alert_failed")).toBe("8")
  expect(nextURL.searchParams.get("worker_alert_threshold")).toBe("4")

  await reviewLink.click()
  await expect(page).toHaveURL(/\/enterprise\?.*#sync$/)
  await expect(page.getByTestId("enterprise-sync-worker-review")).toBeVisible()
  await expect(page.getByText("已从凭证发放回流到导入与同步")).toBeVisible()
})

test("wallet worker review roundtrip should keep worker hints across sync and alerts hops", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 3,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 8,
          last_threshold: 4,
          last_processed: 11,
          last_applied: 6,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&worker_filter_hint=hot&worker_query_hint=tenant-sudirman&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_alert_failed=8&worker_alert_threshold=4"
  )

  const reviewLink = page.getByRole("link", { name: "处理完成后回导入与同步复核" }).first()
  await expect(reviewLink).toBeVisible()
  await reviewLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#sync$/)
  const syncReviewCard = page.getByTestId("enterprise-sync-worker-review")
  await expect(syncReviewCard).toBeVisible()
  await expect(syncReviewCard).toContainText("已从凭证发放回流到导入与同步")

  const reviewAlertsLink = page.getByTestId("enterprise-sync-worker-review-alerts-link")
  await expect(reviewAlertsLink).toBeVisible()
  const href = await reviewAlertsLink.getAttribute("href")
  expect(href).toBeTruthy()
  const alertsLinkURL = new URL(href ?? "", "http://localhost")
  expect(alertsLinkURL.pathname).toBe("/enterprise")
  expect(alertsLinkURL.hash).toBe("#alerts")
  expect(alertsLinkURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(alertsLinkURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")

  await reviewAlertsLink.click()
  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  const finalAlertsURL = new URL(page.url())
  expect(finalAlertsURL.searchParams.get("alerts_view_hint")).toBe("directory_exceptions")
  expect(finalAlertsURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(finalAlertsURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
  expect(finalAlertsURL.searchParams.get("worker_alert_level")).toBeNull()
  expect(finalAlertsURL.searchParams.get("worker_alert_tenant_id")).toBeNull()
  expect(finalAlertsURL.searchParams.get("worker_alert_failed")).toBeNull()
  expect(finalAlertsURL.searchParams.get("worker_alert_threshold")).toBeNull()
  expect(finalAlertsURL.searchParams.get("worker_review_stage_hint")).toBeNull()
  expect(finalAlertsURL.searchParams.get("worker_review_status_hint")).toBeNull()
})

test("enterprise sync worker review reset link should clear review hints and keep worker focus context", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 3,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 8,
          last_threshold: 4,
          last_processed: 11,
          last_applied: 6,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/enterprise?sync_focus_hint=worker_alert&worker_filter_hint=hot&worker_query_hint=tenant-sudirman&worker_review_stage_hint=issuance&worker_review_status_hint=handled#sync"
  )

  const reviewCard = page.getByTestId("enterprise-sync-worker-review")
  await expect(reviewCard).toBeVisible()
  const resetLink = page.getByRole("link", { name: "清除本次回流状态" })
  await expect(resetLink).toBeVisible()

  const href = await resetLink.getAttribute("href")
  expect(href).toBeTruthy()
  const resetURL = new URL(href ?? "", "http://localhost")
  expect(resetURL.pathname).toBe("/enterprise")
  expect(resetURL.hash).toBe("#sync")
  expect(resetURL.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(resetURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(resetURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
  expect(resetURL.searchParams.get("worker_review_stage_hint")).toBeNull()
  expect(resetURL.searchParams.get("worker_review_status_hint")).toBeNull()

  await resetLink.click()
  await expect(page).toHaveURL(/\/enterprise\?.*#sync$/)
  await expect(page.getByTestId("enterprise-sync-worker-review")).toHaveCount(0)
  await expect(page.getByText("已按 worker 告警线索定位到导入与同步工作区")).toBeVisible()
})

test("enterprise sync worker review alerts link should reflect updated filter and query before hop", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 3,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 8,
          last_threshold: 4,
          last_processed: 11,
          last_applied: 6,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/enterprise?sync_focus_hint=worker_alert&worker_filter_hint=hot&worker_query_hint=tenant-sudirman&worker_review_stage_hint=issuance&worker_review_status_hint=handled#sync"
  )

  await page.getByRole("button", { name: /全部（/ }).click()
  await page.getByPlaceholder("按租户 / failed / threshold 筛选").fill("tenant-updated")

  const reviewAlertsLink = page.getByTestId("enterprise-sync-worker-review-alerts-link")
  await expect(reviewAlertsLink).toBeVisible()
  const href = await reviewAlertsLink.getAttribute("href")
  expect(href).toBeTruthy()
  const nextURL = new URL(href ?? "", "http://localhost")
  expect(nextURL.pathname).toBe("/enterprise")
  expect(nextURL.hash).toBe("#alerts")
  expect(nextURL.searchParams.get("alerts_view_hint")).toBe("directory_exceptions")
  expect(nextURL.searchParams.get("worker_filter_hint")).toBeNull()
  expect(nextURL.searchParams.get("worker_query_hint")).toBe("tenant-updated")
  expect(nextURL.searchParams.get("worker_review_stage_hint")).toBeNull()
  expect(nextURL.searchParams.get("worker_review_status_hint")).toBeNull()

  await reviewAlertsLink.click()
  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  await expect(page.getByPlaceholder("按任务ID / 来源 / actor / 租户筛选")).toHaveValue("tenant-updated")
})

test("enterprise sync worker scoped alerts link after reset should keep worker metrics hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 3,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 8,
          last_threshold: 4,
          last_processed: 11,
          last_applied: 6,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/enterprise?sync_focus_hint=worker_alert&worker_filter_hint=hot&worker_query_hint=tenant-sudirman&worker_review_stage_hint=issuance&worker_review_status_hint=handled#sync"
  )

  await page.getByRole("link", { name: "清除本次回流状态" }).click()
  await expect(page.getByTestId("enterprise-sync-worker-review")).toHaveCount(0)

  const scopedAlertsLink = page.getByRole("link", { name: "去审批与异常处理" }).first()
  await expect(scopedAlertsLink).toBeVisible()
  await scopedAlertsLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  const nextURL = new URL(page.url())
  expect(nextURL.searchParams.get("alerts_view_hint")).toBe("directory_exceptions")
  expect(nextURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(nextURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_alert_level")).toBe("hot")
  expect(nextURL.searchParams.get("worker_alert_tenant_id")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_alert_failed")).toBe("8")
  expect(nextURL.searchParams.get("worker_alert_threshold")).toBe("4")
  expect(nextURL.searchParams.get("worker_review_stage_hint")).toBeNull()
  expect(nextURL.searchParams.get("worker_review_status_hint")).toBeNull()
})

test("enterprise sync worker scoped wallet link should keep hints and expose review backflow entry", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 3,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 8,
          last_threshold: 4,
          last_processed: 11,
          last_applied: 6,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/enterprise?sync_focus_hint=worker_alert&worker_filter_hint=hot&worker_query_hint=tenant-sudirman#sync")

  const scopedWalletLink = page.getByRole("link", { name: "处理后去凭证发放" }).first()
  await expect(scopedWalletLink).toBeVisible()
  await scopedWalletLink.click()

  await expect(page).toHaveURL(/\/wallet\?/)
  const walletURL = new URL(page.url())
  expect(walletURL.pathname).toBe("/wallet")
  expect(walletURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(walletURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
  expect(walletURL.searchParams.get("worker_alert_level")).toBe("hot")
  expect(walletURL.searchParams.get("worker_alert_tenant_id")).toBe("tenant-sudirman")
  expect(walletURL.searchParams.get("worker_alert_failed")).toBe("8")
  expect(walletURL.searchParams.get("worker_alert_threshold")).toBe("4")
  expect(walletURL.searchParams.get("template_hint")).toBe("employee")

  const reviewLink = page.getByRole("link", { name: "处理完成后回导入与同步复核" }).first()
  await expect(reviewLink).toBeVisible()
  const reviewHref = await reviewLink.getAttribute("href")
  expect(reviewHref).toBeTruthy()
  const reviewURL = new URL(reviewHref ?? "", "http://localhost")
  expect(reviewURL.pathname).toBe("/enterprise")
  expect(reviewURL.hash).toBe("#sync")
  expect(reviewURL.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(reviewURL.searchParams.get("worker_review_status_hint")).toBe("handled")
  expect(reviewURL.searchParams.get("worker_review_stage_hint")).toBe("issuance")
})

test("wallet alerts issue link should keep sync and worker hints then prefill enterprise alerts filters", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      syncJobs: [
        {
          id: "job-rejected-target",
          tenant_id: viewer.tenant_id,
          source: "hris",
          status: "completed",
          total: 12,
          created: 10,
          updated: 2,
          deactivated: 0,
          rejected: 2,
          actor: "ops@sudirman.co",
          started_at: now,
          ended_at: now,
        },
        {
          id: "job-other",
          tenant_id: "tenant-other",
          source: "scim",
          status: "completed",
          total: 8,
          created: 8,
          updated: 0,
          deactivated: 0,
          rejected: 0,
          actor: "ops@sudirman.co",
          started_at: now,
          ended_at: now,
        },
      ],
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 1,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 6,
          last_threshold: 4,
          last_processed: 9,
          last_applied: 3,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_id=emp-alert-target&sync_category=rejected&sync_source=hris&sync_job_id=job-rejected-target&worker_filter_hint=hot&worker_query_hint=job-rejected-target&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman"
  )

  const alertsIssueLink = page.getByRole("link", { name: "回企业页并按同步异常定位" }).first()
  await expect(alertsIssueLink).toBeVisible()

  const href = await alertsIssueLink.getAttribute("href")
  expect(href).toBeTruthy()
  const nextURL = new URL(href ?? "", "http://localhost")
  expect(nextURL.pathname).toBe("/enterprise")
  expect(nextURL.hash).toBe("#alerts")
  expect(nextURL.searchParams.get("alerts_view_hint")).toBe("directory_exceptions")
  expect(nextURL.searchParams.get("approval_query_hint")).toBe("emp-alert-target")
  expect(nextURL.searchParams.get("sync_status_hint")).toBe("rejected")
  expect(nextURL.searchParams.get("sync_source_hint")).toBe("hris")
  expect(nextURL.searchParams.get("sync_query_hint")).toBe("job-rejected-target")
  expect(nextURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(nextURL.searchParams.get("worker_query_hint")).toBe("job-rejected-target")

  await alertsIssueLink.click()
  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  await expect(page.getByText("目录异常落地页聚焦未完成同步、rejected、停用影响与 worker 告警。")).toBeVisible()
  await expect(page.getByPlaceholder("按任务ID / 来源 / actor / 租户筛选")).toHaveValue("job-rejected-target")
  await expect(page.getByText("job-rejected-target")).toBeVisible()
  await expect(page.getByText("job-other")).toHaveCount(0)
})

test("wallet repair action should preserve alerts and worker backflow hints for follow-up hops", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-repair-followup-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-followup",
          object_id: "obj-repair-followup",
          status: "suspended",
          save_link: "https://wallet.example.com/pass-repair-followup-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-repair-followup-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-repair-followup-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-followup",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
      syncJobs: [
        {
          id: "job-repair-followup",
          tenant_id: viewer.tenant_id,
          source: "hris",
          status: "completed",
          total: 10,
          created: 8,
          updated: 2,
          deactivated: 0,
          rejected: 2,
          actor: "ops@sudirman.co",
          started_at: now,
          ended_at: now,
        },
        {
          id: "job-repair-other",
          tenant_id: "tenant-other",
          source: "scim",
          status: "completed",
          total: 3,
          created: 3,
          updated: 0,
          deactivated: 0,
          rejected: 0,
          actor: "ops@sudirman.co",
          started_at: now,
          ended_at: now,
        },
      ],
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 1,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 8,
          last_threshold: 4,
          last_processed: 9,
          last_applied: 3,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&segment_hint=receipt_recovery&segment_status_hint=attention&target_id=emp-repair-followup&sync_category=rejected&sync_source=hris&sync_job_id=job-repair-followup&worker_filter_hint=hot&worker_query_hint=job-repair-followup&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_alert_failed=8&worker_alert_threshold=4"
  )

  const repairButtons = page.getByRole("button", { name: "批量状态修复（1）" })
  await expect(repairButtons).toHaveCount(2)
  await repairButtons.first().click()
  await expect(page.getByText("已按失败回执批量修复 1 张凭证状态，成功 1 张。")).toBeVisible()

  const alertsIssueLink = page.getByRole("link", { name: "回企业页并按同步异常定位" }).first()
  await expect(alertsIssueLink).toBeVisible()
  const alertsHref = await alertsIssueLink.getAttribute("href")
  expect(alertsHref).toBeTruthy()
  const alertsURL = new URL(alertsHref ?? "", "http://localhost")
  expect(alertsURL.pathname).toBe("/enterprise")
  expect(alertsURL.hash).toBe("#alerts")
  expect(alertsURL.searchParams.get("approval_query_hint")).toBe("emp-repair-followup")
  expect(alertsURL.searchParams.get("target_hint")).toBe("emp-repair-followup")
  expect(alertsURL.searchParams.get("sync_status_hint")).toBe("rejected")
  expect(alertsURL.searchParams.get("sync_source_hint")).toBe("hris")
  expect(alertsURL.searchParams.get("sync_query_hint")).toBe("job-repair-followup")
  expect(alertsURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(alertsURL.searchParams.get("worker_query_hint")).toBe("job-repair-followup")

  const reviewLink = page.getByRole("link", { name: "处理完成后回导入与同步复核" }).first()
  await expect(reviewLink).toBeVisible()
  const reviewHref = await reviewLink.getAttribute("href")
  expect(reviewHref).toBeTruthy()
  const reviewURL = new URL(reviewHref ?? "", "http://localhost")
  expect(reviewURL.pathname).toBe("/enterprise")
  expect(reviewURL.hash).toBe("#sync")
  expect(reviewURL.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(reviewURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(reviewURL.searchParams.get("worker_query_hint")).toBe("job-repair-followup")
  expect(reviewURL.searchParams.get("worker_alert_level")).toBe("hot")
  expect(reviewURL.searchParams.get("worker_alert_tenant_id")).toBe("tenant-sudirman")

  await reviewLink.click()
  await expect(page).toHaveURL(/\/enterprise\?.*#sync$/)
  await expect(page.getByTestId("enterprise-sync-worker-review")).toBeVisible()

  const reviewAlertsLink = page.getByTestId("enterprise-sync-worker-review-alerts-link")
  await expect(reviewAlertsLink).toBeVisible()
  await reviewAlertsLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  await expect(page.getByPlaceholder("按任务ID / 来源 / actor / 租户筛选")).toHaveValue("job-repair-followup")
  await expect(page.getByText("job-repair-followup")).toBeVisible()
  await expect(page.getByText("job-repair-other")).toHaveCount(0)
})

test("wallet retry partial failure should preserve alerts and worker backflow hints for follow-up hops", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-retry-followup-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-retry-followup-a",
          object_id: "obj-retry-followup-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-retry-followup-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-retry-followup-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-retry-followup-b",
          object_id: "obj-retry-followup-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-retry-followup-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-retry-followup-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-retry-followup-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-retry-followup-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-retry-followup-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-retry-followup-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-retry-followup-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
      walletDeliveryRetryErrorIDs: ["delivery-retry-followup-b"],
      syncJobs: [
        {
          id: "job-retry-followup",
          tenant_id: viewer.tenant_id,
          source: "hris",
          status: "completed",
          total: 12,
          created: 9,
          updated: 3,
          deactivated: 0,
          rejected: 2,
          actor: "ops@sudirman.co",
          started_at: now,
          ended_at: now,
        },
        {
          id: "job-retry-other",
          tenant_id: "tenant-other",
          source: "scim",
          status: "completed",
          total: 2,
          created: 2,
          updated: 0,
          deactivated: 0,
          rejected: 0,
          actor: "ops@sudirman.co",
          started_at: now,
          ended_at: now,
        },
      ],
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 1,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 7,
          last_threshold: 4,
          last_processed: 8,
          last_applied: 3,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&segment_hint=receipt_recovery&segment_status_hint=attention&target_id=emp-retry-followup&sync_category=rejected&sync_source=hris&sync_job_id=job-retry-followup&worker_filter_hint=hot&worker_query_hint=job-retry-followup&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_alert_failed=7&worker_alert_threshold=4"
  )

  const retryButtons = page.getByRole("button", { name: "批量重发失败通道（2）" })
  await expect(retryButtons).toHaveCount(2)
  await retryButtons.first().click()
  await expect(page.getByText("已批量重发 2 条失败通道，成功 1 条，失败 1 条。")).toBeVisible()

  const alertsIssueLink = page.getByRole("link", { name: "回企业页并按同步异常定位" }).first()
  await expect(alertsIssueLink).toBeVisible()
  const alertsHref = await alertsIssueLink.getAttribute("href")
  expect(alertsHref).toBeTruthy()
  const alertsURL = new URL(alertsHref ?? "", "http://localhost")
  expect(alertsURL.pathname).toBe("/enterprise")
  expect(alertsURL.hash).toBe("#alerts")
  expect(alertsURL.searchParams.get("approval_query_hint")).toBe("emp-retry-followup")
  expect(alertsURL.searchParams.get("sync_status_hint")).toBe("rejected")
  expect(alertsURL.searchParams.get("sync_source_hint")).toBe("hris")
  expect(alertsURL.searchParams.get("sync_query_hint")).toBe("job-retry-followup")
  expect(alertsURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(alertsURL.searchParams.get("worker_query_hint")).toBe("job-retry-followup")

  const reviewLink = page.getByRole("link", { name: "处理完成后回导入与同步复核" }).first()
  await expect(reviewLink).toBeVisible()
  const reviewHref = await reviewLink.getAttribute("href")
  expect(reviewHref).toBeTruthy()
  const reviewURL = new URL(reviewHref ?? "", "http://localhost")
  expect(reviewURL.pathname).toBe("/enterprise")
  expect(reviewURL.hash).toBe("#sync")
  expect(reviewURL.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(reviewURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(reviewURL.searchParams.get("worker_query_hint")).toBe("job-retry-followup")
  expect(reviewURL.searchParams.get("worker_alert_level")).toBe("hot")
  expect(reviewURL.searchParams.get("worker_alert_tenant_id")).toBe("tenant-sudirman")

  await reviewLink.click()
  await expect(page).toHaveURL(/\/enterprise\?.*#sync$/)
  await expect(page.getByTestId("enterprise-sync-worker-review")).toBeVisible()

  const reviewAlertsLink = page.getByTestId("enterprise-sync-worker-review-alerts-link")
  await expect(reviewAlertsLink).toBeVisible()
  await reviewAlertsLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  await expect(page.getByPlaceholder("按任务ID / 来源 / actor / 租户筛选")).toHaveValue("job-retry-followup")
  await expect(page.getByText("job-retry-followup")).toBeVisible()
  await expect(page.getByText("job-retry-other")).toHaveCount(0)
})

test("wallet repair partial failure should preserve alerts and worker backflow hints for follow-up hops", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-repair-partial-followup-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-partial-followup-a",
          object_id: "obj-repair-partial-followup-a",
          status: "suspended",
          save_link: "https://wallet.example.com/pass-repair-partial-followup-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-repair-partial-followup-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-partial-followup-b",
          object_id: "obj-repair-partial-followup-b",
          status: "suspended",
          save_link: "https://wallet.example.com/pass-repair-partial-followup-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-repair-partial-followup-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-repair-partial-followup-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-partial-followup-a",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-repair-partial-followup-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-repair-partial-followup-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-partial-followup-b",
          status: "failed",
          reason: "receiver unavailable",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
      walletPassActivateErrorIDs: ["pass-repair-partial-followup-b"],
      syncJobs: [
        {
          id: "job-repair-partial-followup",
          tenant_id: viewer.tenant_id,
          source: "hris",
          status: "completed",
          total: 11,
          created: 9,
          updated: 2,
          deactivated: 0,
          rejected: 1,
          actor: "ops@sudirman.co",
          started_at: now,
          ended_at: now,
        },
        {
          id: "job-repair-partial-other",
          tenant_id: "tenant-other",
          source: "scim",
          status: "completed",
          total: 2,
          created: 2,
          updated: 0,
          deactivated: 0,
          rejected: 0,
          actor: "ops@sudirman.co",
          started_at: now,
          ended_at: now,
        },
      ],
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 1,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 9,
          last_threshold: 4,
          last_processed: 8,
          last_applied: 3,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&segment_hint=receipt_recovery&segment_status_hint=attention&target_id=emp-repair-partial-followup&sync_category=rejected&sync_source=hris&sync_job_id=job-repair-partial-followup&worker_filter_hint=hot&worker_query_hint=job-repair-partial-followup&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_alert_failed=9&worker_alert_threshold=4"
  )

  const repairButtons = page.getByRole("button", { name: "批量状态修复（2）" })
  await expect(repairButtons).toHaveCount(2)
  await repairButtons.first().click()
  await expect(page.getByText("已按失败回执批量修复 2 张凭证状态，成功 1 张，失败 1 张。")).toBeVisible()

  const alertsIssueLink = page.getByRole("link", { name: "回企业页并按同步异常定位" }).first()
  await expect(alertsIssueLink).toBeVisible()
  const alertsHref = await alertsIssueLink.getAttribute("href")
  expect(alertsHref).toBeTruthy()
  const alertsURL = new URL(alertsHref ?? "", "http://localhost")
  expect(alertsURL.pathname).toBe("/enterprise")
  expect(alertsURL.hash).toBe("#alerts")
  expect(alertsURL.searchParams.get("approval_query_hint")).toBe("emp-repair-partial-followup")
  expect(alertsURL.searchParams.get("sync_status_hint")).toBe("rejected")
  expect(alertsURL.searchParams.get("sync_source_hint")).toBe("hris")
  expect(alertsURL.searchParams.get("sync_query_hint")).toBe("job-repair-partial-followup")
  expect(alertsURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(alertsURL.searchParams.get("worker_query_hint")).toBe("job-repair-partial-followup")

  const reviewLink = page.getByRole("link", { name: "处理完成后回导入与同步复核" }).first()
  await expect(reviewLink).toBeVisible()
  const reviewHref = await reviewLink.getAttribute("href")
  expect(reviewHref).toBeTruthy()
  const reviewURL = new URL(reviewHref ?? "", "http://localhost")
  expect(reviewURL.pathname).toBe("/enterprise")
  expect(reviewURL.hash).toBe("#sync")
  expect(reviewURL.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(reviewURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(reviewURL.searchParams.get("worker_query_hint")).toBe("job-repair-partial-followup")
  expect(reviewURL.searchParams.get("worker_alert_level")).toBe("hot")
  expect(reviewURL.searchParams.get("worker_alert_tenant_id")).toBe("tenant-sudirman")

  await reviewLink.click()
  await expect(page).toHaveURL(/\/enterprise\?.*#sync$/)
  await expect(page.getByTestId("enterprise-sync-worker-review")).toBeVisible()

  const reviewAlertsLink = page.getByTestId("enterprise-sync-worker-review-alerts-link")
  await expect(reviewAlertsLink).toBeVisible()
  await reviewAlertsLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  await expect(page.getByPlaceholder("按任务ID / 来源 / actor / 租户筛选")).toHaveValue("job-repair-partial-followup")
  await expect(page.getByText("job-repair-partial-followup")).toBeVisible()
  await expect(page.getByText("job-repair-partial-other")).toHaveCount(0)
})

test("wallet batch issue failure should keep enterprise alerts and worker review backflow hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-batch-fail-hit",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-batch-fail-hit",
          object_id: "obj-batch-fail-hit",
          status: "issued",
          save_link: "https://wallet.example.com/pass-batch-fail-hit",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletBatchIssueError: "mock batch issue failed: follow-up blocked by policy",
      syncJobs: [
        {
          id: "job-batch-fail-followup",
          tenant_id: viewer.tenant_id,
          source: "hris",
          status: "completed",
          total: 10,
          created: 9,
          updated: 1,
          deactivated: 0,
          rejected: 1,
          actor: "ops@sudirman.co",
          started_at: now,
          ended_at: now,
        },
      ],
      workerAlerts: [
        {
          tenant_id: viewer.tenant_id,
          count: 1,
          first_seen_at: now,
          last_seen_at: now,
          last_failed: 5,
          last_threshold: 4,
          last_processed: 8,
          last_applied: 3,
          last_skipped_by_attempt_limit: 0,
          last_skipped_by_cooldown: 0,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_ids=emp-batch-fail-hit,emp-batch-fail-missing&sync_category=rejected&sync_source=hris&sync_job_id=job-batch-fail-followup&worker_filter_hint=hot&worker_query_hint=job-batch-fail-followup&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_alert_failed=5&worker_alert_threshold=4"
  )

  await page.getByRole("button", { name: "仅保留未命中对象（1）" }).click()
  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  await expect(batchTargetInput).toHaveValue("emp-batch-fail-missing")

  await page.getByRole("button", { name: "提交批量发放" }).click()
  await expect(batchTargetInput).toHaveValue("emp-batch-fail-missing")
  await expect(page.getByText("最近批量回执")).toHaveCount(0)
  await expect(page.getByText(/^已提交 \d+ 个员工对象，执行模式为/)).toHaveCount(0)

  const alertsIssueLink = page.getByRole("link", { name: "回企业页并按同步异常定位" }).first()
  await expect(alertsIssueLink).toBeVisible()
  const alertsHref = await alertsIssueLink.getAttribute("href")
  expect(alertsHref).toBeTruthy()
  const alertsURL = new URL(alertsHref ?? "", "http://localhost")
  expect(alertsURL.pathname).toBe("/enterprise")
  expect(alertsURL.hash).toBe("#alerts")
  expect(alertsURL.searchParams.get("sync_status_hint")).toBe("rejected")
  expect(alertsURL.searchParams.get("sync_source_hint")).toBe("hris")
  expect(alertsURL.searchParams.get("sync_query_hint")).toBe("job-batch-fail-followup")
  expect(alertsURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(alertsURL.searchParams.get("worker_query_hint")).toBe("job-batch-fail-followup")

  const reviewLink = page.getByRole("link", { name: "处理完成后回导入与同步复核" }).first()
  await expect(reviewLink).toBeVisible()
  const reviewHref = await reviewLink.getAttribute("href")
  expect(reviewHref).toBeTruthy()
  const reviewURL = new URL(reviewHref ?? "", "http://localhost")
  expect(reviewURL.pathname).toBe("/enterprise")
  expect(reviewURL.hash).toBe("#sync")
  expect(reviewURL.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(reviewURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(reviewURL.searchParams.get("worker_query_hint")).toBe("job-batch-fail-followup")

  await alertsIssueLink.click()
  await expect(page).toHaveURL(/\/enterprise\?.*#alerts$/)
  await expect(page.getByPlaceholder("按任务ID / 来源 / actor / 租户筛选")).toHaveValue("job-batch-fail-followup")
  await expect(page.getByText("job-batch-fail-followup")).toBeVisible()
})

test("wallet batch retry should update retryable count and show success summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletDeliveries: [
        {
          id: "delivery-failed-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-1",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet")

  const retryButtonsBefore = page.getByRole("button", { name: "批量重发失败通道（1）" })
  await expect(retryButtonsBefore).toHaveCount(2)
  await retryButtonsBefore.first().click()

  await expect(page.getByText("已批量重发 1 条失败通道，成功 1 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(0)
  await expect(page.getByRole("button", { name: "批量重发失败通道（0）" })).toHaveCount(2)
})

test("wallet batch retry should follow enterprise target hint and only process matched subset", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-filter-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-filter-a",
          object_id: "obj-filter-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-filter-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-filter-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-filter-b",
          object_id: "obj-filter-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-filter-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-filter-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-filter-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-filter-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-filter-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-filter-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-filter-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet?from=enterprise&target_id=emp-filter-a")

  await expect(page.getByText("按对象线索“emp-filter-a”可匹配 1 条可重发失败通道。")).toBeVisible()
  const retryButtonsBefore = page.getByRole("button", { name: "批量重发失败通道（1）" })
  await expect(retryButtonsBefore).toHaveCount(2)
  await retryButtonsBefore.first().click()

  await expect(page.getByText("已批量重发 1 条失败通道，成功 1 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（0）" })).toHaveCount(2)
  await expect(page.getByRole("button", { name: /^重发失败通道$/ })).toHaveCount(1)
})

test("wallet batch retry should report partial failures and keep remaining retryable items", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-partial-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-partial-a",
          object_id: "obj-partial-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-partial-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-partial-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-partial-b",
          object_id: "obj-partial-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-partial-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-partial-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-partial-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-partial-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-partial-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-partial-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-partial-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
      walletDeliveryRetryErrorIDs: ["delivery-partial-b"],
    })
  )

  await login(page)
  await page.goto("/wallet")

  const retryButtonsBefore = page.getByRole("button", { name: "批量重发失败通道（2）" })
  await expect(retryButtonsBefore).toHaveCount(2)
  await retryButtonsBefore.first().click()

  await expect(page.getByText("已批量重发 2 条失败通道，成功 1 条，失败 1 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
  await expect(page.getByRole("button", { name: /^重发失败通道$/ })).toHaveCount(1)
})

test("wallet batch repair should update repairable count and show success summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-repair-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-1",
          object_id: "obj-repair-1",
          status: "suspended",
          save_link: "https://wallet.example.com/pass-repair-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-failed-repair-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-repair-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-1",
          status: "failed",
          reason: "receiver unavailable",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet")

  const repairButtonsBefore = page.getByRole("button", { name: "批量状态修复（1）" })
  await expect(repairButtonsBefore).toHaveCount(2)
  await repairButtonsBefore.first().click()

  await expect(page.getByText("已按失败回执批量修复 1 张凭证状态，成功 1 张。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量状态修复（1）" })).toHaveCount(0)
  await expect(page.getByRole("button", { name: "批量状态修复（0）" })).toHaveCount(2)
})

test("wallet batch repair should follow enterprise target hint and only process matched subset", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-repair-filter-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-filter-a",
          object_id: "obj-repair-filter-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-repair-filter-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-repair-filter-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-filter-b",
          object_id: "obj-repair-filter-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-repair-filter-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-repair-filter-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-repair-filter-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-filter-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-repair-filter-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-repair-filter-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-filter-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet?from=enterprise&target_id=emp-repair-filter-a")

  await expect(page.getByText("按对象线索“emp-repair-filter-a”可匹配 1 条可重发失败通道。")).toBeVisible()
  const repairButtonsBefore = page.getByRole("button", { name: "批量状态修复（1）" })
  await expect(repairButtonsBefore).toHaveCount(2)
  await repairButtonsBefore.first().click()

  await expect(page.getByText("已按失败回执批量修复 1 张凭证状态，成功 1 张。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量状态修复（0）" })).toHaveCount(2)
})

test("wallet batch repair should report partial failures and keep remaining repairable passes", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-repair-partial-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-partial-a",
          object_id: "obj-repair-partial-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-repair-partial-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-repair-partial-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-partial-b",
          object_id: "obj-repair-partial-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-repair-partial-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-repair-partial-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-repair-partial-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-partial-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-repair-partial-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-repair-partial-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-repair-partial-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
      walletPassActivateErrorIDs: ["pass-repair-partial-b"],
    })
  )

  await login(page)
  await page.goto("/wallet")

  const repairButtonsBefore = page.getByRole("button", { name: "批量状态修复（2）" })
  await expect(repairButtonsBefore).toHaveCount(2)
  await repairButtonsBefore.first().click()

  await expect(page.getByText("已按失败回执批量修复 2 张凭证状态，成功 1 张，失败 1 张。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量状态修复（1）" })).toHaveCount(2)
})

test("wallet batch repair then retry should keep counters aligned in same enterprise target context", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-chain-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-chain-a",
          object_id: "obj-chain-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-chain-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-chain-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-chain-b",
          object_id: "obj-chain-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-chain-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-chain-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-chain-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-chain-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-chain-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-chain-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-chain-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet?from=enterprise&target_id=emp-chain-a")

  await expect(page.getByText("按对象线索“emp-chain-a”可匹配 1 条可重发失败通道。")).toBeVisible()
  const repairButtonsBefore = page.getByRole("button", { name: "批量状态修复（1）" })
  await expect(repairButtonsBefore).toHaveCount(2)
  await repairButtonsBefore.first().click()

  await expect(page.getByText("已按失败回执批量修复 1 张凭证状态，成功 1 张。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量状态修复（0）" })).toHaveCount(2)
  const retryButtonsAfterRepair = page.getByRole("button", { name: "批量重发失败通道（1）" })
  await expect(retryButtonsAfterRepair).toHaveCount(2)
  await retryButtonsAfterRepair.first().click()

  await expect(page.getByText("已批量重发 1 条失败通道，成功 1 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（0）" })).toHaveCount(2)
  await expect(page.getByRole("button", { name: "批量状态修复（0）" })).toHaveCount(2)
})

test("wallet batch retry then repair should keep counters aligned after retry partial failure", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-chain-retry-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-chain-retry-a",
          object_id: "obj-chain-retry-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-chain-retry-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-chain-retry-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-chain-retry-b",
          object_id: "obj-chain-retry-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-chain-retry-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-chain-retry-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-chain-retry-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-chain-retry-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-chain-retry-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-chain-retry-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-chain-retry-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
      walletDeliveryRetryErrorIDs: ["delivery-chain-retry-b"],
    })
  )

  await login(page)
  await page.goto("/wallet?from=enterprise&target_id=emp-chain-retry")

  await expect(page.getByText("按对象线索“emp-chain-retry”可匹配 2 条可重发失败通道。")).toBeVisible()
  const retryButtonsBefore = page.getByRole("button", { name: "批量重发失败通道（2）" })
  await expect(retryButtonsBefore).toHaveCount(2)
  await retryButtonsBefore.first().click()

  await expect(page.getByText("已批量重发 2 条失败通道，成功 1 条，失败 1 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
  await expect(page.getByRole("button", { name: "批量状态修复（1）" })).toHaveCount(2)

  const repairButtonsAfterRetry = page.getByRole("button", { name: "批量状态修复（1）" })
  await repairButtonsAfterRetry.first().click()
  await expect(page.getByText("已按失败回执批量修复 1 张凭证状态，成功 1 张。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量状态修复（0）" })).toHaveCount(2)
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
})

test("wallet batch retry and repair buttons should be disabled when no retryable failures", async ({ page }) => {
  await setupApiMocks(page, buildScenario())

  await login(page)
  await page.goto("/wallet")

  const retryButtons = page.getByRole("button", { name: "批量重发失败通道（0）" })
  const repairButtons = page.getByRole("button", { name: "批量状态修复（0）" })
  await expect(retryButtons).toHaveCount(2)
  await expect(repairButtons).toHaveCount(2)
  await expect(retryButtons.first()).toBeDisabled()
  await expect(retryButtons.nth(1)).toBeDisabled()
  await expect(repairButtons.first()).toBeDisabled()
  await expect(repairButtons.nth(1)).toBeDisabled()
})

test("wallet batch retry should support manual pass query split without enterprise hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-manual-retry-split-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-retry-split-a",
          object_id: "obj-manual-retry-split-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-manual-retry-split-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-manual-retry-split-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-retry-split-b",
          object_id: "obj-manual-retry-split-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-manual-retry-split-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-manual-retry-split-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-manual-retry-split-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-retry-split-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-manual-retry-split-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-manual-retry-split-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-retry-split-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet")

  const passQueryInput = page.getByPlaceholder("搜索员工/访客 ID、模板名、对象 ID 或状态")
  await passQueryInput.fill("emp-manual-retry-split-a")
  await expect(page.getByText("按对象线索“emp-manual-retry-split-a”可匹配 1 条可重发失败通道。")).toBeVisible()

  const retryButtons = page.getByRole("button", { name: "批量重发失败通道（1）" })
  await expect(retryButtons).toHaveCount(2)
  await retryButtons.first().click()

  await expect(page.getByText("已批量重发 1 条失败通道，成功 1 条。")).toBeVisible()

  await passQueryInput.fill("")
  await expect(page.getByText("当前可重发失败通道 1 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
})

test("wallet batch repair should support manual pass query split without enterprise hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-manual-repair-split-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-repair-split-a",
          object_id: "obj-manual-repair-split-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-manual-repair-split-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-manual-repair-split-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-repair-split-b",
          object_id: "obj-manual-repair-split-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-manual-repair-split-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-manual-repair-split-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-manual-repair-split-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-repair-split-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-manual-repair-split-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-manual-repair-split-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-repair-split-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet")

  const passQueryInput = page.getByPlaceholder("搜索员工/访客 ID、模板名、对象 ID 或状态")
  await passQueryInput.fill("emp-manual-repair-split-a")
  await expect(page.getByText("按对象线索“emp-manual-repair-split-a”可匹配 1 条可重发失败通道。")).toBeVisible()

  const repairButtons = page.getByRole("button", { name: "批量状态修复（1）" })
  await expect(repairButtons).toHaveCount(2)
  await repairButtons.first().click()

  await expect(page.getByText("已按失败回执批量修复 1 张凭证状态，成功 1 张。")).toBeVisible()

  await passQueryInput.fill("")
  await expect(page.getByRole("button", { name: "批量状态修复（1）" })).toHaveCount(2)
})

test("wallet seed batch draft should support manual pass query split without enterprise hints", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-manual-draft-split-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-draft-split-a",
          object_id: "obj-manual-draft-split-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-manual-draft-split-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-manual-draft-split-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-draft-split-b",
          object_id: "obj-manual-draft-split-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-manual-draft-split-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-manual-draft-split-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-manual-draft-split-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-draft-split-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-manual-draft-split-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-manual-draft-split-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-manual-draft-split-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet")

  const passQueryInput = page.getByPlaceholder("搜索员工/访客 ID、模板名、对象 ID 或状态")
  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")

  await passQueryInput.fill("emp-manual-draft-split-a")
  await expect(page.getByText("按对象线索“emp-manual-draft-split-a”可匹配 1 条可重发失败通道。")).toBeVisible()

  const seedDraftSplitButton = page.getByRole("button", { name: "写入批量补发草稿（1）" })
  await expect(seedDraftSplitButton).toBeVisible()
  await seedDraftSplitButton.click()

  await expect(page.getByText("已将 1 个失败对象写入批量补发草稿")).toBeVisible()
  await expect(batchTargetInput).toHaveValue("emp-manual-draft-split-a")

  await passQueryInput.fill("")
  await expect(page.getByText("当前可重发失败通道 2 条。")).toBeVisible()

  const seedDraftAllButton = page.getByRole("button", { name: "写入批量补发草稿（2）" })
  await expect(seedDraftAllButton).toBeVisible()
  await seedDraftAllButton.click()

  await expect(page.getByText("已将 2 个失败对象写入批量补发草稿")).toBeVisible()
  await expect(batchTargetInput).toHaveValue("emp-manual-draft-split-a\nemp-manual-draft-split-b")
})

test("wallet seed batch draft should prefill targets and show summary", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-draft-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-draft-1",
          object_id: "obj-draft-1",
          status: "suspended",
          save_link: "https://wallet.example.com/pass-draft-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-failed-draft-1",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-draft-1",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-draft-1",
          status: "failed",
          reason: "address missing",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet")

  const seedDraftButton = page.getByRole("button", { name: "写入批量补发草稿（1）" })
  await expect(seedDraftButton).toBeVisible()
  await seedDraftButton.click()

  await expect(page.getByText("已将 1 个失败对象写入批量补发草稿")).toBeVisible()
  await expect(page.getByText("可直接提交批量发放。")).toBeVisible()
  await expect(page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")).toHaveValue("emp-draft-1")
})

test("wallet enterprise batch draft restore should recover prefilled targets after missing-only filter", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-hit-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hit-1",
          object_id: "obj-hit-1",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hit-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_ids=emp-hit-1,emp-missing-1"
  )

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  await expect(page.getByRole("button", { name: "仅保留未命中对象（1）" })).toBeVisible()
  await expect(page.getByRole("button", { name: "恢复全部预填对象（2）" })).toBeVisible()
  await expect(batchTargetInput).toHaveValue("emp-hit-1\nemp-missing-1")

  await page.getByRole("button", { name: "仅保留未命中对象（1）" }).click()
  await expect(page.getByText("来源：企业页。已将未命中的 1 个对象写入批量发放草稿，可直接继续补发。")).toBeVisible()
  await expect(batchTargetInput).toHaveValue("emp-missing-1")

  await page.getByRole("button", { name: "恢复全部预填对象（2）" }).click()
  await expect(page.getByText("来源：企业页。已恢复全部 2 个预填对象到批量发放草稿。")).toBeVisible()
  await expect(batchTargetInput).toHaveValue("emp-hit-1\nemp-missing-1")
})

test("wallet enterprise missing-only draft should submit batch issue and clear draft with receipts", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-enterprise-hit-1",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-enterprise-hit-1",
          object_id: "obj-enterprise-hit-1",
          status: "issued",
          save_link: "https://wallet.example.com/pass-enterprise-hit-1",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_ids=emp-enterprise-hit-1,emp-enterprise-missing-1"
  )

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  const submitBatchButton = page.getByRole("button", { name: "提交批量发放" })
  await expect(submitBatchButton).toBeEnabled()
  await expect(batchTargetInput).toHaveValue("emp-enterprise-hit-1\nemp-enterprise-missing-1")

  await page.getByRole("button", { name: "仅保留未命中对象（1）" }).click()
  await expect(page.getByText("来源：企业页。已将未命中的 1 个对象写入批量发放草稿，可直接继续补发。")).toBeVisible()
  await expect(batchTargetInput).toHaveValue("emp-enterprise-missing-1")

  await submitBatchButton.click()
  await expect(page.getByText("已提交 1 个员工对象，执行模式为 queued。")).toBeVisible()
  await expect(page.getByText("最近批量回执")).toBeVisible()
  await expect(page.getByTestId("wallet-recent-batch-target").first()).toHaveText("emp-enterprise-missing-1")
  await expect(batchTargetInput).toHaveValue("")
})

test("wallet enterprise missing-only draft should keep draft when batch issue submit fails", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-enterprise-hit-2",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-enterprise-hit-2",
          object_id: "obj-enterprise-hit-2",
          status: "issued",
          save_link: "https://wallet.example.com/pass-enterprise-hit-2",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletBatchIssueError: "mock batch issue failed: enterprise draft submit blocked",
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_ids=emp-enterprise-hit-2,emp-enterprise-missing-2"
  )

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  const submitBatchButton = page.getByRole("button", { name: "提交批量发放" })
  await expect(submitBatchButton).toBeEnabled()

  await page.getByRole("button", { name: "仅保留未命中对象（1）" }).click()
  await expect(batchTargetInput).toHaveValue("emp-enterprise-missing-2")
  await submitBatchButton.click()

  await expect(batchTargetInput).toHaveValue("emp-enterprise-missing-2")
  await expect(page.getByText("最近批量回执")).toHaveCount(0)
  await expect(page.getByText(/^已提交 \d+ 个员工对象，执行模式为/)).toHaveCount(0)
})

test("wallet enterprise mixed target hints should keep retry filter stable when toggling missing-only draft", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-mixed-filter-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-mixed-filter-a",
          object_id: "obj-mixed-filter-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-mixed-filter-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-mixed-filter-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-mixed-filter-b",
          object_id: "obj-mixed-filter-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-mixed-filter-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-mixed-filter-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-mixed-filter-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-mixed-filter-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-mixed-filter-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-mixed-filter-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-mixed-filter-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_id=emp-mixed-filter-a&target_ids=emp-mixed-filter-a,emp-mixed-filter-b,emp-mixed-filter-missing"
  )

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  await expect(batchTargetInput).toHaveValue("emp-mixed-filter-a\nemp-mixed-filter-b\nemp-mixed-filter-missing")
  await expect(page.getByText("按对象线索“emp-mixed-filter-a”可匹配 1 条可重发失败通道。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)

  await page.getByRole("button", { name: "仅保留未命中对象（1）" }).click()
  await expect(batchTargetInput).toHaveValue("emp-mixed-filter-missing")
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)

  await page.getByRole("button", { name: "恢复全部预填对象（3）" }).click()
  await expect(batchTargetInput).toHaveValue("emp-mixed-filter-a\nemp-mixed-filter-b\nemp-mixed-filter-missing")
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
})

test("wallet enterprise mixed target hints should support retry then missing-only batch issue in one flow", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-mixed-flow-retry",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-mixed-flow-retry",
          object_id: "obj-mixed-flow-retry",
          status: "issued",
          save_link: "https://wallet.example.com/pass-mixed-flow-retry",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-mixed-flow-hit",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-mixed-flow-hit",
          object_id: "obj-mixed-flow-hit",
          status: "issued",
          save_link: "https://wallet.example.com/pass-mixed-flow-hit",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-mixed-flow-retry",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-mixed-flow-retry",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-mixed-flow-retry",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_id=emp-mixed-flow-retry&target_ids=emp-mixed-flow-hit,emp-mixed-flow-missing"
  )

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  await expect(batchTargetInput).toHaveValue("emp-mixed-flow-hit\nemp-mixed-flow-missing")
  await expect(page.getByText("按对象线索“emp-mixed-flow-retry”可匹配 1 条可重发失败通道。")).toBeVisible()

  const retryButtonsBefore = page.getByRole("button", { name: "批量重发失败通道（1）" })
  await expect(retryButtonsBefore).toHaveCount(2)
  await retryButtonsBefore.first().click()
  await expect(page.getByText("已批量重发 1 条失败通道，成功 1 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（0）" })).toHaveCount(2)

  await page.getByRole("button", { name: "仅保留未命中对象（1）" }).click()
  await expect(batchTargetInput).toHaveValue("emp-mixed-flow-missing")

  const submitBatchButton = page.getByRole("button", { name: "提交批量发放" })
  await expect(submitBatchButton).toBeEnabled()
  await submitBatchButton.click()
  await expect(page.getByText("已提交 1 个员工对象，执行模式为 queued。")).toBeVisible()
  await expect(page.getByTestId("wallet-recent-batch-target").first()).toHaveText("emp-mixed-flow-missing")
  await expect(batchTargetInput).toHaveValue("")
})

test("wallet enterprise target hints should prioritize target_id over target_email and target_name", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-priority-id",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-priority-id",
          object_id: "obj-priority-id",
          status: "issued",
          save_link: "https://wallet.example.com/pass-priority-id",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-priority-email",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-priority-email",
          object_id: "obj-priority-email",
          status: "issued",
          save_link: "https://wallet.example.com/pass-priority-email",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-priority-name",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-priority-name",
          object_id: "obj-priority-name",
          status: "issued",
          save_link: "https://wallet.example.com/pass-priority-name",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-priority-id",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-priority-id",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-priority-id",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-priority-email",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-priority-email",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-priority-email",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-priority-name",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-priority-name",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-priority-name",
          status: "failed",
          reason: "address missing",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_id=emp-priority-id&target_email=emp-priority-email&target_name=emp-priority-name"
  )

  await expect(page.getByText("按对象线索“emp-priority-id”可匹配 1 条可重发失败通道。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
})

test("wallet enterprise target hints should fallback to target_email before target_name when target_id is absent", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-fallback-email",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-fallback-email",
          object_id: "obj-fallback-email",
          status: "issued",
          save_link: "https://wallet.example.com/pass-fallback-email",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-fallback-name",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-fallback-name",
          object_id: "obj-fallback-name",
          status: "issued",
          save_link: "https://wallet.example.com/pass-fallback-name",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-fallback-email",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-fallback-email",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-fallback-email",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-fallback-name",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-fallback-name",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-fallback-name",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_email=emp-fallback-email&target_name=emp-fallback-name&target_ids=emp-fallback-email,emp-fallback-missing"
  )

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  await expect(page.getByText("按对象线索“emp-fallback-email”可匹配 1 条可重发失败通道。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
  await page.getByRole("button", { name: "仅保留未命中对象（1）" }).click()
  await expect(batchTargetInput).toHaveValue("emp-fallback-missing")
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
})

test("wallet enterprise target hints should fallback to target_name when target_id and target_email are absent", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-fallback-name-only",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-fallback-name-only",
          object_id: "obj-fallback-name-only",
          status: "issued",
          save_link: "https://wallet.example.com/pass-fallback-name-only",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-fallback-name-other",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-fallback-name-other",
          object_id: "obj-fallback-name-other",
          status: "issued",
          save_link: "https://wallet.example.com/pass-fallback-name-other",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-fallback-name-only",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-fallback-name-only",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-fallback-name-only",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-fallback-name-other",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-fallback-name-other",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-fallback-name-other",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_name=emp-fallback-name-only&target_ids=emp-fallback-name-only,emp-fallback-name-missing"
  )

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  await expect(page.getByText("按对象线索“emp-fallback-name-only”可匹配 1 条可重发失败通道。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
  await page.getByRole("button", { name: "仅保留未命中对象（1）" }).click()
  await expect(batchTargetInput).toHaveValue("emp-fallback-name-missing")
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
})

test("wallet enterprise target hints should keep target_name priority over target_hint", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-name-priority",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-name-priority",
          object_id: "obj-name-priority",
          status: "issued",
          save_link: "https://wallet.example.com/pass-name-priority",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-hint-fallback",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-fallback",
          object_id: "obj-hint-fallback",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hint-fallback",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-name-priority",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-name-priority",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-name-priority",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-hint-fallback",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-hint-fallback",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-fallback",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_name=emp-name-priority&target_hint=emp-hint-fallback"
  )

  const passQueryInput = page.getByPlaceholder("搜索员工/访客 ID、模板名、对象 ID 或状态")
  await expect(passQueryInput).toHaveValue("emp-name-priority")
  await expect(page.getByText("按对象线索“emp-name-priority”可匹配 1 条可重发失败通道。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
})

test("wallet enterprise target hints should use target_hint when other target hints are absent", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-hint-only",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-only",
          object_id: "obj-hint-only",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hint-only",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-hint-alt",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "staff-alt-delivery",
          object_id: "obj-hint-alt",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hint-alt",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-hint-only",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-hint-only",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-only",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-hint-alt",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-hint-alt",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "staff-alt-delivery",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_hint=emp-hint-only&target_ids=emp-hint-only,emp-hint-only-missing"
  )

  const passQueryInput = page.getByPlaceholder("搜索员工/访客 ID、模板名、对象 ID 或状态")
  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  await expect(passQueryInput).toHaveValue("emp-hint-only")
  await expect(page.getByText("按对象线索“emp-hint-only”可匹配 1 条可重发失败通道。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
  await page.getByRole("button", { name: "仅保留未命中对象（1）" }).click()
  await expect(batchTargetInput).toHaveValue("emp-hint-only-missing")
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
})

test("wallet enterprise target hints should keep target_id priority when target_hint is user", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-hint-user-primary",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-user-primary",
          object_id: "obj-hint-user-primary",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hint-user-primary",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-hint-user-alt",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "staff-user-alt",
          object_id: "obj-hint-user-alt",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hint-user-alt",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-hint-user-primary",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-hint-user-primary",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-user-primary",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-hint-user-alt",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-hint-user-alt",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "staff-user-alt",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_hint=user&target_id=emp-hint-user-primary"
  )

  const passQueryInput = page.getByPlaceholder("搜索员工/访客 ID、模板名、对象 ID 或状态")
  await expect(passQueryInput).toHaveValue("emp-hint-user-primary")
  await expect(page.getByText("按对象线索“emp-hint-user-primary”可匹配 1 条可重发失败通道。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（1）" })).toHaveCount(2)
})

test("wallet enterprise target hints should not use target_hint visitor as retry query when object hints are absent", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-hint-visitor-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-visitor-a",
          object_id: "obj-hint-visitor-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hint-visitor-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-hint-visitor-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "staff-hint-visitor-b",
          object_id: "obj-hint-visitor-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hint-visitor-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-hint-visitor-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-hint-visitor-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-visitor-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-hint-visitor-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-hint-visitor-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "staff-hint-visitor-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_hint=visitor")

  await expect(page.getByText("当前可重发失败通道 2 条。")).toBeVisible()
  await expect(page.getByRole("button", { name: "批量重发失败通道（2）" })).toHaveCount(2)
})

test("wallet enterprise target hints should not treat target_hint user as object query when object hints are absent", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-hint-user-generic-a",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-generic-a",
          object_id: "obj-hint-generic-a",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hint-user-generic-a",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
        {
          id: "pass-hint-user-generic-b",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-generic-b",
          object_id: "obj-hint-generic-b",
          status: "issued",
          save_link: "https://wallet.example.com/pass-hint-user-generic-b",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-hint-user-generic-a",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-hint-user-generic-a",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-generic-a",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
        {
          id: "delivery-hint-user-generic-b",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-hint-user-generic-b",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-hint-generic-b",
          status: "failed",
          reason: "receiver busy",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto("/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_hint=user")

  await expect(page.getByText("来源：企业页。已承接")).toBeVisible()
  await expect(page.getByText("当前可重发失败通道 2 条。")).toBeVisible()
  await expect(page.getByText("未找到该对象的既有凭证，已预填单发对象，可直接创建补发。")).toHaveCount(0)
})

test("wallet manual pass query should persist after enterprise-hint actions without being overwritten", async ({
  page,
}) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletPasses: [
        {
          id: "pass-query-persist",
          tenant_id: viewer.tenant_id,
          provider: "google_wallet",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-query-persist",
          object_id: "obj-query-persist",
          status: "issued",
          save_link: "https://wallet.example.com/pass-query-persist",
          issued_at: now,
          created_by: viewer.email,
          updated_by: viewer.email,
          created_at: now,
          updated_at: now,
        },
      ],
      walletDeliveries: [
        {
          id: "delivery-query-persist",
          tenant_id: viewer.tenant_id,
          pass_id: "pass-query-persist",
          template_id: "tpl-employee",
          target_type: "user",
          target_id: "emp-query-persist",
          status: "failed",
          reason: "channel timeout",
          attempt: 1,
          retryable: true,
          triggered_at: now,
        },
      ],
    })
  )

  await login(page)
  await page.goto(
    "/wallet?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_id=emp-query-persist"
  )

  const passQueryInput = page.getByPlaceholder("搜索员工/访客 ID、模板名、对象 ID 或状态")
  await expect(passQueryInput).toHaveValue("emp-query-persist")
  await passQueryInput.fill("manual-query-overwrite-check")

  const retryButtons = page.getByRole("button", { name: "批量重发失败通道（1）" })
  await expect(retryButtons).toHaveCount(2)
  await retryButtons.first().click()
  await expect(page.getByText("已批量重发 1 条失败通道，成功 1 条。")).toBeVisible()

  await expect(passQueryInput).toHaveValue("manual-query-overwrite-check")
  await expect(page.getByRole("button", { name: "批量重发失败通道（0）" })).toHaveCount(2)
})

test("wallet batch issue submit should show summary and recent receipts", async ({ page }) => {
  await setupApiMocks(page, buildScenario())

  await login(page)
  await page.goto("/wallet")

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  const submitBatchButton = page.getByRole("button", { name: "提交批量发放" })
  await expect(submitBatchButton).toBeEnabled()

  await batchTargetInput.fill("emp-batch-1\nemp-batch-2")
  await submitBatchButton.click()

  await expect(page.getByText("已提交 2 个员工对象，执行模式为 queued。")).toBeVisible()
  await expect(page.getByText("最近批量回执")).toBeVisible()
  await expect(page.getByText("emp-batch-1", { exact: true }).first()).toBeVisible()
  await expect(page.getByText("emp-batch-2", { exact: true }).first()).toBeVisible()
})

test("wallet batch issue mixed result should show failed receipt error badge", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletBatchIssueFailedTargetIDs: ["emp-batch-mixed-failed"],
    })
  )

  await login(page)
  await page.goto("/wallet")

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  const submitBatchButton = page.getByRole("button", { name: "提交批量发放" })
  await expect(submitBatchButton).toBeEnabled()

  await batchTargetInput.fill("emp-batch-mixed-success\nemp-batch-mixed-failed")
  await submitBatchButton.click()

  await expect(page.getByText("已提交 2 个员工对象，执行模式为 queued。")).toBeVisible()
  const receiptBoard = page.getByTestId("wallet-recent-batch-receipts")
  await expect(receiptBoard).toBeVisible()

  const successRow = page.getByTestId("wallet-recent-batch-receipt-job-emp-batch-mixed-success-0")
  await expect(successRow.getByTestId("wallet-recent-batch-target")).toHaveText("emp-batch-mixed-success")
  await expect(successRow.getByTestId("wallet-recent-batch-status")).toHaveText("success")
  await expect(successRow.getByTestId("wallet-recent-batch-error")).toHaveCount(0)

  const failedRow = page.getByTestId("wallet-recent-batch-receipt-job-emp-batch-mixed-failed-1")
  await expect(failedRow.getByTestId("wallet-recent-batch-target")).toHaveText("emp-batch-mixed-failed")
  await expect(failedRow.getByTestId("wallet-recent-batch-status")).toHaveText("failed")
  await expect(failedRow.getByTestId("wallet-recent-batch-error")).toHaveText("mock_issue_failed")
})

test("wallet batch issue submit failure should show api error", async ({ page }) => {
  await setupApiMocks(
    page,
    buildScenario({
      walletBatchIssueError: "mock batch issue failed: template rate limited",
    })
  )

  await login(page)
  await page.goto("/wallet")

  const batchTargetInput = page.getByPlaceholder("输入多个员工或访客 ID，支持换行、逗号、分号分隔")
  const submitBatchButton = page.getByRole("button", { name: "提交批量发放" })
  await expect(submitBatchButton).toBeEnabled()

  await batchTargetInput.fill("emp-batch-failed-1")
  await submitBatchButton.click()

  await expect(batchTargetInput).toHaveValue("emp-batch-failed-1")
  await expect(page.getByText("最近批量回执")).toHaveCount(0)
  await expect(page.getByText(/^已提交 \d+ 个员工对象，执行模式为/)).toHaveCount(0)
})
