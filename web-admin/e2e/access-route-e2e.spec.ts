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

async function fulfillJson(route: Route, payload: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(payload),
  })
}

async function setupApiMocks(page: Page) {
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

    if (method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    await fulfillJson(route, {})
  })
}

test.beforeEach(async ({ page }) => {
  await setupApiMocks(page)
})

test("login and navigate access domains by browser interactions", async ({ page }) => {
  await page.goto("/login")

  await page.getByLabel("邮箱").fill("tenant.admin@sudirman.co")
  await page.getByLabel("密码").fill("admin123")
  await page.getByRole("button", { name: "登录" }).click()
  await expect(page).toHaveURL(/\/dashboard$/)

  await page.getByRole("link", { name: /权限/ }).click()
  await expect(page).toHaveURL(/\/access\/directory$/)

  await page.getByRole("tab", { name: "权限策略" }).click()
  await expect(page).toHaveURL(/\/access\/policies$/)

  await page.getByRole("tab", { name: "临时与访客授权" }).click()
  await expect(page).toHaveURL(/\/access\/grants$/)
})

test("invalid access section should redirect to /access/directory and keep query", async ({ page }) => {
  await page.goto("/login")
  await page.getByLabel("邮箱").fill("tenant.admin@sudirman.co")
  await page.getByLabel("密码").fill("admin123")
  await page.getByRole("button", { name: "登录" }).click()
  await expect(page).toHaveURL(/\/dashboard$/)

  await page.goto("/access/unknown?from=enterprise&flow=sync_to_access")
  await expect(page).toHaveURL(/\/access\/directory\?from=enterprise&flow=sync_to_access$/)
})
