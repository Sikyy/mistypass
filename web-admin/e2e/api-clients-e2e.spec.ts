import { expect, test, type Page, type Route } from "@playwright/test"

const tenantID = "tenant_demo_jakarta"
const now = "2026-05-24T10:00:00Z"

const viewer = {
  id: "user-tenant-admin-api-clients",
  email: "tenant.admin.api-clients@mistypass.local",
  role: "tenant_admin",
  tenant_id: tenantID,
  building_ids: ["building-1"],
}

type OAuth2ClientRecord = {
  id: string
  tenant_id: string
  name: string
  client_secret?: string
  redirect_uris: string[]
  scopes: string[]
  enabled: boolean
  created_at: string
  updated_at: string
}

type RequestLog = {
  method: string
  path: string
  body: unknown
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

function oauth2Client(patch: Partial<OAuth2ClientRecord> & Pick<OAuth2ClientRecord, "id" | "name">): OAuth2ClientRecord {
  return {
    id: patch.id,
    tenant_id: patch.tenant_id ?? tenantID,
    name: patch.name,
    client_secret: patch.client_secret,
    redirect_uris: patch.redirect_uris ?? ["https://mobile.example.com/oauth/callback"],
    scopes: patch.scopes ?? ["read"],
    enabled: patch.enabled ?? true,
    created_at: patch.created_at ?? now,
    updated_at: patch.updated_at ?? now,
  }
}

async function setupApiMocks(page: Page) {
  const requests: RequestLog[] = []
  let clients: OAuth2ClientRecord[] = [
    oauth2Client({
      id: "oac_existing_mobile",
      name: "Mobile App",
      scopes: ["read"],
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

    if (path === "/api/v1/oauth2/clients" && method === "GET") {
      await fulfillJson(route, { items: clients, total: clients.length })
      return
    }

    if (path === "/api/v1/oauth2/clients" && method === "POST") {
      const body = request.postDataJSON()
      requests.push({ method, path, body })
      const created = oauth2Client({
        id: "oac_created_ops",
        name: body.name,
        client_secret: "e2e-created-secret",
        redirect_uris: body.redirect_uris,
        scopes: body.scopes,
        enabled: true,
      })
      clients = [created, ...clients]
      await fulfillJson(route, created, 201)
      return
    }

    const clientMatch = path.match(/^\/api\/v1\/oauth2\/clients\/([^/]+)$/)
    if (clientMatch && method === "PATCH") {
      const clientID = clientMatch[1]
      const body = request.postDataJSON()
      requests.push({ method, path, body })
      clients = clients.map((client) =>
        client.id === clientID
          ? {
              ...client,
              name: body.name ?? client.name,
              redirect_uris: body.redirect_uris ?? client.redirect_uris,
              scopes: body.scopes ?? client.scopes,
              enabled: body.enabled ?? client.enabled,
              client_secret: undefined,
              updated_at: now,
            }
          : client
      )
      const updated = clients.find((client) => client.id === clientID)
      await fulfillJson(route, updated ?? { error: "client not found" }, updated ? 200 : 404)
      return
    }

    if (clientMatch && method === "DELETE") {
      const clientID = clientMatch[1]
      requests.push({ method, path, body: null })
      clients = clients.filter((client) => client.id !== clientID)
      await route.fulfill({ status: 204, body: "" })
      return
    }

    if (method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    await fulfillJson(route, {})
  })

  return requests
}

test("api clients page supports create, edit, disable, and delete flow", async ({ page }) => {
  await seedAuthenticatedSession(page)
  const requests = await setupApiMocks(page)

  await page.goto("/developer/api-clients")
  await expect(page.getByRole("heading", { name: "API Clients" })).toBeVisible()
  await expect(page.getByText("Mobile App")).toBeVisible()

  await page.getByRole("button", { name: "New Client" }).click()
  const createSheet = page.getByRole("dialog", { name: "New API Client" })
  await expect(createSheet).toBeVisible()
  await createSheet.getByLabel("Name").fill("Operations Portal")
  await createSheet.getByLabel("Redirect URIs").fill("https://ops.example.com/callback\nhttp://localhost:5173/oauth/callback")
  await createSheet.getByRole("button", { name: "write" }).click()
  await createSheet.getByRole("button", { name: "Create Client" }).click()

  await expect(page.getByText("e2e-created-secret")).toBeVisible()
  await createSheet.getByRole("button", { name: "Close" }).click()
  await expect(page.getByText("Operations Portal")).toBeVisible()
  expect(requests[0]).toMatchObject({
    method: "POST",
    path: "/api/v1/oauth2/clients",
    body: {
      name: "Operations Portal",
      redirect_uris: ["https://ops.example.com/callback", "http://localhost:5173/oauth/callback"],
      scopes: ["read", "write"],
    },
  })

  await page.getByRole("button", { name: "Actions for Operations Portal" }).click()
  await page.getByRole("menuitem", { name: "Edit" }).click()
  const editSheet = page.getByRole("dialog", { name: "Edit API Client" })
  await expect(editSheet).toBeVisible()
  await editSheet.getByLabel("Name").fill("Operations Portal Disabled")
  await editSheet.getByLabel("Redirect URIs").fill("https://ops.example.com/updated-callback")
  await editSheet.getByRole("button", { name: "write" }).click()
  await editSheet.getByRole("switch", { name: "Enabled" }).click()
  await editSheet.getByRole("button", { name: "Save Changes" }).click()

  await expect(page.getByText("Operations Portal Disabled")).toBeVisible()
  await expect(page.getByText("Disabled", { exact: true }).first()).toBeVisible()
  expect(requests[1]).toMatchObject({
    method: "PATCH",
    path: "/api/v1/oauth2/clients/oac_created_ops",
    body: {
      name: "Operations Portal Disabled",
      redirect_uris: ["https://ops.example.com/updated-callback"],
      scopes: ["read"],
      enabled: false,
    },
  })

  await page.getByRole("button", { name: "Actions for Operations Portal Disabled" }).click()
  await page.getByRole("menuitem", { name: "Delete" }).click()
  const deleteDialog = page.getByRole("dialog", { name: "Delete API client" })
  await expect(deleteDialog).toBeVisible()
  await deleteDialog.getByRole("button", { name: "Delete" }).click()

  await expect(page.getByText("Operations Portal Disabled")).toHaveCount(0)
  expect(requests[2]).toMatchObject({
    method: "DELETE",
    path: "/api/v1/oauth2/clients/oac_created_ops",
  })
})
