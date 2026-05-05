import { expect, test, type Page, type Route } from "@playwright/test"

const viewer = {
  id: "user-tenant-admin",
  email: "tenant.admin@sudirman.co",
  role: "tenant_admin",
  tenant_id: "tenant-sudirman",
  building_ids: ["building-1"],
}

const initialTime = "2026-04-22T10:00:00Z"
const updatedTime = "2026-04-22T11:30:00Z"

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

test("enterprise sync should update Talenta connector with hybrid incremental credential payload", async ({ page }) => {
  let patchPayload: Record<string, unknown> | null = null
  let connectors: Array<Record<string, unknown>> = [
    {
      id: "hrc_talenta_sudirman",
      tenant_id: viewer.tenant_id,
      vendor: "talenta",
      status: "active",
      sync_strategy: "hybrid",
      webhook_secret_ref: `vault://${viewer.tenant_id}/hris/talenta/webhook_secret`,
      created_at: initialTime,
      updated_at: initialTime,
    },
  ]
  const secrets: Array<Record<string, unknown>> = [
    {
      ref: `vault://${viewer.tenant_id}/hris/talenta/credential`,
      tenant_id: viewer.tenant_id,
      name: "hris/talenta/credential",
      kind: "connector_credential",
      created_at: initialTime,
      updated_at: initialTime,
    },
    {
      ref: `vault://${viewer.tenant_id}/hris/talenta/webhook_secret`,
      tenant_id: viewer.tenant_id,
      name: "hris/talenta/webhook_secret",
      kind: "webhook_secret",
      created_at: initialTime,
      updated_at: initialTime,
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
            created_at: initialTime,
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
            last_synced_at: initialTime,
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

    if (path === "/api/v1/enterprise/hris-connectors/hrc_talenta_sudirman" && method === "PATCH") {
      patchPayload = (request.postDataJSON() as Record<string, unknown>) ?? {}
      connectors = [
        {
          ...connectors[0],
          status: patchPayload.status ?? "active",
          sync_strategy: patchPayload.sync_strategy ?? "hybrid",
          credential_ref: `vault://${viewer.tenant_id}/hris/talenta/credential`,
          webhook_secret_ref: `vault://${viewer.tenant_id}/hris/talenta/webhook_secret`,
          updated_at: updatedTime,
        },
      ]
      await fulfillJson(route, connectors[0])
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
        created_at: initialTime,
        updated_at: initialTime,
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

  await seedAuthenticatedSession(page)
  await page.goto("/enterprise#sync")
  await page.waitForLoadState("networkidle")

  const connectorForm = page.getByRole("button", { name: "更新 Talenta Connector" }).locator("xpath=ancestor::form[1]")
  await expect(connectorForm.getByRole("button", { name: "更新 Talenta Connector" })).toBeVisible()
  await expect(connectorForm.getByText("Talenta 默认支持增量拉取")).toBeVisible()
  await expect(connectorForm.getByText("系统会继续使用 Talenta 默认增量契约")).toBeVisible()
  await connectorForm.getByLabel("Talenta Client ID").fill("mekari-client-id-002")
  await connectorForm.getByLabel("Talenta Client Secret").fill("mekari-client-secret-002")
  await connectorForm.getByLabel("拉取基地址").fill("https://api.mekari.example")
  await connectorForm.getByLabel("员工列表路径").fill("/v2/talenta/v2/employee")
  await connectorForm.getByLabel("分页大小").fill("50")

  await connectorForm.locator('[role="switch"]').click()
  await expect(connectorForm.getByText("只填写需要覆盖的字段即可")).toBeVisible()
  await connectorForm.getByLabel("时间格式覆盖项").fill("unix")

  await connectorForm.getByRole("button", { name: "更新 Talenta Connector" }).click()
  await expect.poll(() => patchPayload !== null).toBe(true)

  await expect(page.getByText(`最近更新时间：${updatedTime}。`)).toBeVisible()
  await expect(
    page.locator('input[value="http://localhost:8080/api/v1/enterprise/hris-webhook/hrc_talenta_sudirman"]')
  ).toBeVisible()

  expect(patchPayload).toMatchObject({
    tenant_id: viewer.tenant_id,
    status: "active",
    sync_strategy: "hybrid",
    webhook_secret_ref: `vault://${viewer.tenant_id}/hris/talenta/webhook_secret`,
    updated_by: viewer.email,
  })
  expect(patchPayload?.webhook_secret_value).toBeUndefined()
  expect(patchPayload?.credential_ref).toBeUndefined()

  const credentialValue = patchPayload?.credential_value
  expect(typeof credentialValue).toBe("string")
  const parsedCredentialValue = JSON.parse(String(credentialValue))
  expect(parsedCredentialValue).toMatchObject({
    client_id: "mekari-client-id-002",
    client_secret: "mekari-client-secret-002",
    base_url: "https://api.mekari.example",
    employee_path: "/v2/talenta/v2/employee",
    page_limit: 50,
    timestamp_format: "unix",
  })
  expect(parsedCredentialValue.updated_after_param).toBeUndefined()
  expect(parsedCredentialValue.updated_before_param).toBeUndefined()
})
