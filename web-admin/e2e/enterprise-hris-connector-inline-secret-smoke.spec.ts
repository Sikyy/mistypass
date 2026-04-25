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

test("enterprise sync smoke creates Talenta connector with inline secrets", async ({ page }) => {
  let submittedPayload: Record<string, unknown> | null = null
  let connectors: Array<Record<string, unknown>> = []
  let secrets: Array<Record<string, unknown>> = []

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
          id: "hrc_talenta_inline_secret",
          tenant_id: viewer.tenant_id,
          vendor: "talenta",
          status: "active",
          sync_strategy: "hybrid",
          credential_ref: `vault://${viewer.tenant_id}/hris/talenta/credential-inline`,
          webhook_secret_ref: `vault://${viewer.tenant_id}/hris/talenta/webhook-inline`,
          created_at: now,
          updated_at: now,
        },
      ]
      secrets = [
        {
          ref: `vault://${viewer.tenant_id}/hris/talenta/credential-inline`,
          tenant_id: viewer.tenant_id,
          name: "hris/talenta/credential-inline",
          kind: "connector_credential",
          created_at: now,
          updated_at: now,
        },
        {
          ref: `vault://${viewer.tenant_id}/hris/talenta/webhook-inline`,
          tenant_id: viewer.tenant_id,
          name: "hris/talenta/webhook-inline",
          kind: "webhook_secret",
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
  await expect(page.getByText("0 个凭证 Ref / 0 个 Webhook Ref")).toBeVisible()
  const connectorForm = page.getByRole("button", { name: "创建 Talenta Connector" }).locator("xpath=ancestor::form[1]")
  const clientIDInput = connectorForm.getByLabel("Talenta Client ID")
  const clientSecretInput = connectorForm.getByLabel("Talenta Client Secret")
  const webhookSecretInput = connectorForm.getByLabel("Webhook 验签 Secret")

  await clientIDInput.fill("mekari-client-id")
  await clientSecretInput.fill("mekari-client-secret")
  await webhookSecretInput.fill("mekari-webhook-secret")
  await expect(clientIDInput).toHaveValue("mekari-client-id")
  await expect(clientSecretInput).toHaveValue("mekari-client-secret")
  await expect(webhookSecretInput).toHaveValue("mekari-webhook-secret")
  await connectorForm.getByRole("button", { name: "创建 Talenta Connector" }).click()

  await expect.poll(() => submittedPayload !== null).toBe(true)

  const credentialValue = submittedPayload?.credential_value
  expect(typeof credentialValue).toBe("string")
  expect(JSON.parse(String(credentialValue))).toMatchObject({
    client_id: "mekari-client-id",
    client_secret: "mekari-client-secret",
  })

  expect(submittedPayload).toMatchObject({
    tenant_id: viewer.tenant_id,
    vendor: "talenta",
    status: "active",
    sync_strategy: "hybrid",
    updated_by: viewer.email,
    webhook_secret_value: "mekari-webhook-secret",
  })
  expect(submittedPayload?.credential_ref).toBeUndefined()
  expect(submittedPayload?.webhook_secret_ref).toBeUndefined()

  await expect(page.getByRole("button", { name: "更新 Talenta Connector" })).toBeVisible()
  await expect(
    page.locator('input[value="http://localhost:8080/api/v1/enterprise/hris-webhook/hrc_talenta_inline_secret"]')
  ).toBeVisible()
  await expect(page.getByText(`vault://${viewer.tenant_id}/hris/talenta/credential-inline`).first()).toBeVisible()
  await expect(page.getByText(`vault://${viewer.tenant_id}/hris/talenta/webhook-inline`).first()).toBeVisible()
})
