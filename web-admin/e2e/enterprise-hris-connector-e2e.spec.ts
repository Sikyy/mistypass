import { expect, test, type Page, type Route } from "@playwright/test"

const viewer = {
  id: "user-tenant-admin",
  email: "tenant.admin@sudirman.co",
  role: "tenant_admin",
  tenant_id: "tenant-sudirman",
  building_ids: ["building-1"],
}

const now = "2026-04-22T10:00:00Z"

async function fulfillJson(route: Route, payload: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(payload),
  })
}

async function seedAuthenticatedSession(page: Page) {
  await page.addInitScript((user) => {
    window.sessionStorage.setItem("mistypass_admin_access_token", "e2e-token")
    window.sessionStorage.setItem("mistypass_admin_refresh_token", "e2e-refresh")
    window.sessionStorage.setItem("mistypass_admin_csrf_token", "e2e-csrf")
    window.localStorage.setItem("i18nextLng", "zh-CN")
    window.localStorage.setItem("mistypass_viewer_email", user.email)
  }, viewer)
}

async function configureTalentaConnectorWithExistingRefs(page: Page) {
  const connectorForm = page.getByRole("button", { name: "创建 Talenta Connector" }).locator("xpath=ancestor::form[1]")
  await connectorForm.locator('[role="combobox"]:visible').nth(2).click()
  await page.getByRole("option", { name: "使用已有 Ref" }).click()
  await connectorForm.locator('[role="combobox"]:visible').nth(3).click()
  await page.getByRole("option", { name: "hris/talenta/credential" }).click()
  await connectorForm.locator('[role="combobox"]:visible').nth(4).click()
  await page.getByRole("option", { name: "使用已有 Ref" }).click()
  await connectorForm.locator('[role="combobox"]:visible').nth(5).click()
  await page.getByRole("option", { name: "hris/talenta/webhook_secret" }).click()
  return connectorForm
}

test("enterprise sync should create Talenta connector with existing refs and refresh metadata", async ({ page }) => {
  let submittedPayload: Record<string, unknown> | null = null
  let connectors: Array<Record<string, unknown>> = []
  let secrets: Array<Record<string, unknown>> = [
    {
      ref: `vault://${viewer.tenant_id}/hris/talenta/credential`,
      tenant_id: viewer.tenant_id,
      name: "hris/talenta/credential",
      kind: "connector_credential",
      created_at: now,
      updated_at: now,
    },
    {
      ref: `vault://${viewer.tenant_id}/hris/talenta/webhook_secret`,
      tenant_id: viewer.tenant_id,
      name: "hris/talenta/webhook_secret",
      kind: "webhook_secret",
      created_at: now,
      updated_at: now,
    },
  ]

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
      await fulfillJson(route, { items: connectors })
      return
    }

    if (path === "/api/v1/enterprise/hris-secrets" && method === "GET") {
      await fulfillJson(route, { items: secrets })
      return
    }

    if (path === "/api/v1/enterprise/hris-connectors" && method === "POST") {
      submittedPayload = (request.postDataJSON() as Record<string, unknown>) ?? {}
      connectors = [
        {
          id: "hrc_talenta_sudirman",
          tenant_id: viewer.tenant_id,
          vendor: "talenta",
          status: "active",
          sync_strategy: "hybrid",
          credential_ref: `vault://${viewer.tenant_id}/hris/talenta/credential`,
          webhook_secret_ref: `vault://${viewer.tenant_id}/hris/talenta/webhook_secret`,
          created_at: now,
          updated_at: now,
        },
      ]
      await fulfillJson(route, connectors[0], 201)
      return
    }

    if (path === "/api/v1/enterprise/sync-jobs" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-requests" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/jit-provision-approvals" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/summary" && method === "GET") {
      await fulfillJson(route, { items: [] })
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
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/access-policies" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/wallet/passes" && method === "GET") {
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

    await fulfillJson(route, { error: `unmocked route: ${method} ${path}` }, 500)
  })

  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#sync")
  await page.waitForLoadState("networkidle")

  await expect(page.getByText("Talenta Connector 配置")).toBeVisible()
  const connectorForm = await configureTalentaConnectorWithExistingRefs(page)
  await expect(connectorForm.getByText("Talenta 默认支持增量拉取")).toBeVisible()
  await expect(connectorForm.getByText("当前使用已有 credential ref。Talenta 默认增量仍然生效")).toBeVisible()
  await expect(connectorForm.locator('[role="switch"]')).toBeDisabled()
  await page.getByRole("button", { name: "创建 Talenta Connector" }).click()
  await expect.poll(() => submittedPayload !== null).toBe(true)

  await expect(page.getByRole("button", { name: "更新 Talenta Connector" })).toBeVisible()
  await expect(
    page.locator('input[value="http://localhost:8080/api/v1/enterprise/hris-webhook/hrc_talenta_sudirman"]')
  ).toBeVisible()
  await expect(page.getByText(`vault://${viewer.tenant_id}/hris/talenta/credential`).first()).toBeVisible()
  await expect(page.getByText(`vault://${viewer.tenant_id}/hris/talenta/webhook_secret`).first()).toBeVisible()

  expect(submittedPayload).toMatchObject({
    tenant_id: viewer.tenant_id,
    vendor: "talenta",
    status: "active",
    sync_strategy: "hybrid",
    updated_by: viewer.email,
    credential_ref: `vault://${viewer.tenant_id}/hris/talenta/credential`,
    webhook_secret_ref: `vault://${viewer.tenant_id}/hris/talenta/webhook_secret`,
  })
  expect(submittedPayload?.credential_value).toBeUndefined()
  expect(submittedPayload?.webhook_secret_value).toBeUndefined()
})

test("enterprise sync should classify Talenta connector credential ref save failure", async ({ page }) => {
  let submitCount = 0
  const connectors: Array<Record<string, unknown>> = []
  const secrets: Array<Record<string, unknown>> = [
    {
      ref: `vault://${viewer.tenant_id}/hris/talenta/credential`,
      tenant_id: viewer.tenant_id,
      name: "hris/talenta/credential",
      kind: "connector_credential",
      created_at: now,
      updated_at: now,
    },
    {
      ref: `vault://${viewer.tenant_id}/hris/talenta/webhook_secret`,
      tenant_id: viewer.tenant_id,
      name: "hris/talenta/webhook_secret",
      kind: "webhook_secret",
      created_at: now,
      updated_at: now,
    },
  ]

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
      await fulfillJson(route, { items: connectors })
      return
    }

    if (path === "/api/v1/enterprise/hris-secrets" && method === "GET") {
      await fulfillJson(route, { items: secrets })
      return
    }

    if (path === "/api/v1/enterprise/hris-connectors" && method === "POST") {
      submitCount += 1
      await fulfillJson(route, { error: "credential_ref not found in tenant vault" }, 400)
      return
    }

    if (path === "/api/v1/enterprise/sync-jobs" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-requests" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/jit-provision-approvals" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/summary" && method === "GET") {
      await fulfillJson(route, { items: [] })
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
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/access-policies" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/wallet/passes" && method === "GET") {
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

    await fulfillJson(route, { error: `unmocked route: ${method} ${path}` }, 500)
  })

  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#sync")
  await page.waitForLoadState("networkidle")

  await configureTalentaConnectorWithExistingRefs(page)
  await page.getByRole("button", { name: "创建 Talenta Connector" }).click()

  await expect.poll(() => submitCount).toBe(1)
  await expect(page.getByTestId("enterprise-talenta-save-error-guidance")).toBeVisible()
  await expect(page.getByTestId("enterprise-talenta-save-error-title")).toHaveText("检查凭证 Ref")
  await expect(page.getByTestId("enterprise-talenta-save-error-badge")).toHaveText("需修正配置")
  await expect(page.getByTestId("enterprise-talenta-save-error-raw")).toHaveText("credential_ref not found in tenant vault")
  await expect(page.getByTestId("enterprise-talenta-save-error-suggestion-0")).toContainText("Credential Ref")
  await expect(page.getByRole("button", { name: "创建 Talenta Connector" })).toBeVisible()
  await expect(page.getByRole("button", { name: "更新 Talenta Connector" })).toHaveCount(0)
})

test("enterprise sync should classify Talenta connector transient save failure with retry guidance", async ({ page }) => {
  let submitCount = 0
  const connectors: Array<Record<string, unknown>> = []
  const secrets: Array<Record<string, unknown>> = [
    {
      ref: `vault://${viewer.tenant_id}/hris/talenta/credential`,
      tenant_id: viewer.tenant_id,
      name: "hris/talenta/credential",
      kind: "connector_credential",
      created_at: now,
      updated_at: now,
    },
    {
      ref: `vault://${viewer.tenant_id}/hris/talenta/webhook_secret`,
      tenant_id: viewer.tenant_id,
      name: "hris/talenta/webhook_secret",
      kind: "webhook_secret",
      created_at: now,
      updated_at: now,
    },
  ]

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
      await fulfillJson(route, { items: connectors })
      return
    }

    if (path === "/api/v1/enterprise/hris-secrets" && method === "GET") {
      await fulfillJson(route, { items: secrets })
      return
    }

    if (path === "/api/v1/enterprise/hris-connectors" && method === "POST") {
      submitCount += 1
      await fulfillJson(route, { error: "upstream Talenta API 429 too many requests" }, 503)
      return
    }

    if (path === "/api/v1/enterprise/sync-jobs" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-requests" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/jit-provision-approvals" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/enterprise/sync-worker-alerts/summary" && method === "GET") {
      await fulfillJson(route, { items: [] })
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
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/access-policies" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/wallet/passes" && method === "GET") {
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

    await fulfillJson(route, { error: `unmocked route: ${method} ${path}` }, 500)
  })

  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#sync")
  await page.waitForLoadState("networkidle")

  await configureTalentaConnectorWithExistingRefs(page)
  await page.getByRole("button", { name: "创建 Talenta Connector" }).click()

  await expect.poll(() => submitCount).toBe(1)
  await expect(page.getByTestId("enterprise-talenta-save-error-guidance")).toBeVisible()
  await expect(page.getByTestId("enterprise-talenta-save-error-title")).toHaveText("稍后重试当前保存")
  await expect(page.getByTestId("enterprise-talenta-save-error-badge")).toHaveText("可稍后重试")
  await expect(page.getByTestId("enterprise-talenta-save-error-raw")).toHaveText(
    "upstream Talenta API 429 too many requests"
  )
  await expect(page.getByTestId("enterprise-talenta-save-error-suggestion-0")).toContainText("等待 Talenta")
  await expect(page.getByRole("button", { name: "创建 Talenta Connector" })).toBeVisible()
  await expect(page.getByRole("button", { name: "更新 Talenta Connector" })).toHaveCount(0)
})
