import { expect, test, type Page, type Route } from "@playwright/test"

const tenantID = "tenant-sudirman"
const now = "2026-04-28T10:00:00Z"

const viewer = {
  id: "user-tenant-admin",
  email: "tenant.admin@sudirman.co",
  role: "tenant_admin",
  tenant_id: tenantID,
  building_ids: ["building-1"],
}

type IntegrationRecord = {
  id: string
  resource_type: "Integration"
  tenant_id: string
  type: "hris" | "identity_provider"
  provider: string
  name: string
  description: string
  status: string
  configured: boolean
  sync_mode: string
  source_id: string
  last_sync_at?: string
  created_at: string
  updated_at: string
}

async function fulfillJson(route: Route, payload: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(payload),
  })
}

async function seedAuthenticatedSession(page: Page) {
  await page.addInitScript(() => {
    window.localStorage.setItem("i18nextLng", "en-US")
    window.sessionStorage.setItem("mistypass_admin_access_token", "e2e-token")
    window.sessionStorage.setItem("mistypass_admin_refresh_token", "e2e-refresh")
  })
}

function integrationRecord(patch: Partial<IntegrationRecord> & Pick<IntegrationRecord, "id" | "provider" | "source_id">): IntegrationRecord {
  const provider = patch.provider
  return {
    id: patch.id,
    resource_type: "Integration",
    tenant_id: patch.tenant_id ?? tenantID,
    type: patch.type ?? "hris",
    provider,
    name: patch.name ?? `${provider.toUpperCase()} HRIS`,
    description: patch.description ?? "Directory and employee access sync",
    status: patch.status ?? "active",
    configured: patch.configured ?? true,
    sync_mode: patch.sync_mode ?? "hybrid",
    source_id: patch.source_id,
    last_sync_at: patch.last_sync_at,
    created_at: patch.created_at ?? now,
    updated_at: patch.updated_at ?? now,
  }
}

async function setupApiMocks(page: Page) {
  const requests: Array<{ method: string; path: string; body: unknown }> = []
  let integrations: IntegrationRecord[] = [
    integrationRecord({
      id: "integration_hrc_talenta",
      provider: "talenta",
      source_id: "hrc_talenta",
      last_sync_at: "2026-04-28T09:45:00Z",
    }),
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

    if (path === "/api/v1/auth/refresh" && method === "POST") {
      await fulfillJson(route, {
        access_token: "e2e-token",
        refresh_token: "e2e-refresh",
        expires_in: 3600,
        user: viewer,
      })
      return
    }

    if (path === "/api/v1/integrations" && method === "GET") {
      await fulfillJson(route, { items: integrations })
      return
    }

    if (path === "/api/v1/integrations" && method === "POST") {
      const body = request.postDataJSON()
      requests.push({ method, path, body })
      const payload = body.integration
      const created = integrationRecord({
        id: "integration_hrc_sunfish",
        provider: payload.provider,
        source_id: "hrc_sunfish",
        status: payload.status,
        sync_mode: payload.sync_mode,
        configured: Boolean(payload.credential_ref || payload.webhook_secret_ref),
      })
      integrations = [created, ...integrations]
      await fulfillJson(route, created, 201)
      return
    }

    const integrationDetailMatch = path.match(/^\/api\/v1\/integrations\/([^/]+)$/)
    if (integrationDetailMatch && method === "GET") {
      const integration = integrations.find((item) => item.id === integrationDetailMatch[1])
      await fulfillJson(route, integration ?? { error: "integration not found" }, integration ? 200 : 404)
      return
    }

    if (integrationDetailMatch && method === "PATCH") {
      const integrationID = integrationDetailMatch[1]
      const body = request.postDataJSON()
      requests.push({ method, path, body })
      const payload = body.integration
      integrations = integrations.map((item) =>
        item.id === integrationID
          ? {
              ...item,
              status: payload.status ?? item.status,
              sync_mode: payload.sync_mode ?? item.sync_mode,
              configured: item.configured || Boolean(payload.credential_ref || payload.webhook_secret_ref),
              updated_at: now,
            }
          : item
      )
      await fulfillJson(route, integrations.find((item) => item.id === integrationID))
      return
    }

    if (integrationDetailMatch && method === "DELETE") {
      const integrationID = integrationDetailMatch[1]
      requests.push({ method, path, body: null })
      integrations = integrations.map((item) =>
        item.id === integrationID
          ? {
              ...item,
              status: "inactive",
              updated_at: now,
            }
          : item
      )
      await route.fulfill({ status: 204, body: "" })
      return
    }

    await fulfillJson(route, method === "GET" ? { items: [] } : {})
  })

  return requests
}

test("organization integrations support add, edit, and disable write flow", async ({ page }) => {
  await seedAuthenticatedSession(page)
  const requests = await setupApiMocks(page)

  await page.goto("/organization/integrations")
  await expect(page.getByRole("heading", { name: "TALENTA HRIS" })).toBeVisible()

  await page.getByRole("button", { name: "Add Integration" }).click()
  const addDialog = page.getByRole("dialog", { name: "Add Integration" })
  await expect(addDialog).toBeVisible()
  await addDialog.locator("select").nth(1).selectOption("sunfish")
  await addDialog.locator("select").nth(3).selectOption("webhook")
  await addDialog.getByPlaceholder("vault://tenant/hris/provider/api_key").fill("vault://tenant-sudirman/hris/sunfish/api_key")
  await addDialog.getByPlaceholder("vault://tenant/hris/provider/webhook_secret").fill("vault://tenant-sudirman/hris/sunfish/webhook_secret")
  await addDialog.getByRole("button", { name: "Add Integration" }).click()

  await expect(page.getByRole("heading", { name: "SUNFISH HRIS" })).toBeVisible()
  expect(requests[0]).toMatchObject({
    method: "POST",
    path: "/api/v1/integrations",
    body: {
      integration: {
        tenant_id: tenantID,
        type: "hris",
        provider: "sunfish",
        status: "active",
        sync_mode: "webhook",
        credential_ref: "vault://tenant-sudirman/hris/sunfish/api_key",
        webhook_secret_ref: "vault://tenant-sudirman/hris/sunfish/webhook_secret",
        actor: viewer.email,
      },
    },
  })

  await page.getByRole("button", { name: "Actions for SUNFISH HRIS" }).click()
  await page.getByRole("menuitem", { name: "Edit" }).click()
  const editDialog = page.getByRole("dialog", { name: "Edit Integration" })
  await expect(editDialog).toBeVisible()
  await editDialog.locator("select").nth(3).selectOption("pull")
  await editDialog.getByRole("button", { name: "Save Integration" }).click()

  await expect(page.getByText("pull", { exact: true })).toBeVisible()
  expect(requests[1]).toMatchObject({
    method: "PATCH",
    path: "/api/v1/integrations/integration_hrc_sunfish",
    body: {
      integration: {
        tenant_id: tenantID,
        type: "hris",
        provider: "sunfish",
        status: "active",
        sync_mode: "pull",
        actor: viewer.email,
      },
    },
  })

  await page.getByRole("button", { name: "Actions for SUNFISH HRIS" }).click()
  await page.getByRole("menuitem", { name: "Disable" }).click()
  const disableDialog = page.getByRole("dialog", { name: "Disable integration" })
  await expect(disableDialog).toBeVisible()
  await disableDialog.getByRole("button", { name: "Disable integration" }).click()

  await expect(page.getByText("Inactive", { exact: true }).first()).toBeVisible()
  expect(requests[2]).toMatchObject({
    method: "DELETE",
    path: "/api/v1/integrations/integration_hrc_sunfish",
  })
})
