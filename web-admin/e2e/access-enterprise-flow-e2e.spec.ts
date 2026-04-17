import { expect, test, type Page, type Route } from "@playwright/test"

const viewer = {
  id: "user-tenant-admin-access-flow",
  email: "tenant.admin.access.flow@sudirman.co",
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

async function login(page: Page) {
  await page.goto("/login")
  await page.getByLabel("邮箱").fill(viewer.email)
  await page.getByLabel("密码").fill("admin123")
  await page.getByRole("button", { name: "登录" }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

test.beforeEach(async ({ page }) => {
  await setupApiMocks(page)
})

test("access enterprise flow stage=directory should redirect to directory and keep hints", async ({ page }) => {
  await login(page)
  await page.goto(
    "/access/policies?from=enterprise&flow=sync_to_access&stage=directory&tenant_id=tenant-sudirman&group_name=Ops%20Directory"
  )

  await expect(page).toHaveURL(/\/access\/directory\?/)
  const currentURL = new URL(page.url())
  expect(currentURL.searchParams.get("from")).toBe("enterprise")
  expect(currentURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(currentURL.searchParams.get("stage")).toBe("directory")
  expect(currentURL.searchParams.get("tenant_id")).toBe("tenant-sudirman")
  expect(currentURL.searchParams.get("group_name")).toBe("Ops Directory")
  await expect(page.getByText("来源：企业页。已预填“Ops Directory”用户组草稿")).toBeVisible()
})

test("access enterprise flow stage=policies should redirect and append segment descriptor", async ({ page }) => {
  await login(page)
  await page.goto(
    "/access/directory?from=enterprise&flow=sync_to_access&stage=policies&tenant_id=tenant-sudirman&segment_hint=policy_delivery&segment_status_hint=attention&policy_name=Night%20Shift%20Policy"
  )

  await expect(page).toHaveURL(/\/access\/policies\?/)
  const currentURL = new URL(page.url())
  expect(currentURL.searchParams.get("stage")).toBe("policies")
  expect(currentURL.searchParams.get("segment_hint")).toBe("policy_delivery")
  expect(currentURL.searchParams.get("segment_status_hint")).toBe("attention")
  await expect(page.getByText("来源：企业页。已预填策略名称“Night Shift Policy”")).toBeVisible()
  await expect(page.getByText("分段提示：用户组使用到权限下发 / 待收口")).toBeVisible()
})

test("access domain tabs should keep enterprise query and update stage across sections", async ({ page }) => {
  await login(page)
  await page.goto(
    "/access/directory?from=enterprise&flow=sync_to_access&stage=directory&tenant_id=tenant-sudirman&group_member_email=alice%40sudirman.co&group_member_name=Alice%20Zhang"
  )

  await page.getByRole("tab", { name: "权限策略" }).click()
  await expect(page).toHaveURL(/\/access\/policies\?/)
  let currentURL = new URL(page.url())
  expect(currentURL.searchParams.get("stage")).toBe("policies")
  expect(currentURL.searchParams.get("group_member_email")).toBe("alice@sudirman.co")
  expect(currentURL.searchParams.get("group_member_name")).toBe("Alice Zhang")

  await page.getByRole("tab", { name: "临时与访客授权" }).click()
  await expect(page).toHaveURL(/\/access\/grants\?/)
  currentURL = new URL(page.url())
  expect(currentURL.searchParams.get("stage")).toBe("issuance")
  expect(currentURL.searchParams.get("target_email")).toBe("alice@sudirman.co")
  expect(currentURL.searchParams.get("target_name")).toBe("Alice Zhang")
})

test("access worker review backflow link should carry handled status and current stage", async ({ page }) => {
  await login(page)
  await page.goto(
    "/access/policies?from=enterprise&flow=sync_to_access&stage=policies&tenant_id=tenant-sudirman&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_alert_failed=8&worker_alert_threshold=4&worker_filter_hint=hot&worker_query_hint=tenant-sudirman"
  )

  const reviewLink = page.getByRole("link", { name: "处理完成后回导入与同步复核" })
  await expect(reviewLink).toBeVisible()
  await reviewLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#sync$/)
  const nextURL = new URL(page.url())
  expect(nextURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(nextURL.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(nextURL.searchParams.get("worker_review_status_hint")).toBe("handled")
  expect(nextURL.searchParams.get("worker_review_stage_hint")).toBe("policies")
  expect(nextURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(nextURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
})

test("access enterprise flow stage=issuance should redirect to grants and keep target hints", async ({ page }) => {
  await login(page)
  await page.goto(
    "/access/directory?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_name=Bob%20Li&target_email=bob%40sudirman.co&target_id=emp-bob"
  )

  await expect(page).toHaveURL(/\/access\/grants\?/)
  const currentURL = new URL(page.url())
  expect(currentURL.searchParams.get("from")).toBe("enterprise")
  expect(currentURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(currentURL.searchParams.get("stage")).toBe("issuance")
  expect(currentURL.searchParams.get("tenant_id")).toBe("tenant-sudirman")
  expect(currentURL.searchParams.get("target_name")).toBe("Bob Li")
  expect(currentURL.searchParams.get("target_email")).toBe("bob@sudirman.co")
  expect(currentURL.searchParams.get("target_id")).toBe("emp-bob")
  await expect(page.getByText("来源：企业页。已承接对象“Bob Li”。长期员工发放建议直接前往凭证发放")).toBeVisible()
})

test("access grants wallet link should carry enterprise and worker hints", async ({ page }) => {
  await login(page)
  await page.goto(
    "/access/grants?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&group_member_email=alice%40sudirman.co&group_member_id=emp-alice&group_member_name=Alice%20Zhang&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_alert_failed=8&worker_alert_threshold=4&worker_filter_hint=hot&worker_query_hint=tenant-sudirman"
  )

  const walletLink = page.getByRole("link", { name: "去凭证发放" }).first()
  await expect(walletLink).toBeVisible()
  const href = await walletLink.getAttribute("href")
  expect(href).toBeTruthy()

  const nextURL = new URL(href ?? "", "http://localhost")
  expect(nextURL.pathname).toBe("/wallet")
  expect(nextURL.searchParams.get("from")).toBe("enterprise")
  expect(nextURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(nextURL.searchParams.get("stage")).toBe("issuance")
  expect(nextURL.searchParams.get("scenario")).toBe("employee_mobile")
  expect(nextURL.searchParams.get("tenant_id")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("target_hint")).toBe("user")
  expect(nextURL.searchParams.get("target_email")).toBe("alice@sudirman.co")
  expect(nextURL.searchParams.get("target_id")).toBe("emp-alice")
  expect(nextURL.searchParams.get("target_name")).toBe("Alice Zhang")
  expect(nextURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(nextURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_alert_level")).toBe("hot")
  expect(nextURL.searchParams.get("worker_alert_tenant_id")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_alert_failed")).toBe("8")
  expect(nextURL.searchParams.get("worker_alert_threshold")).toBe("4")
})

test("access grants domain wallet link should keep visitor scenario and enterprise hints", async ({ page }) => {
  await login(page)
  await page.goto(
    "/access/grants?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&target_name=Bob%20Li&target_email=bob%40sudirman.co&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_filter_hint=hot&worker_query_hint=tenant-sudirman"
  )

  const visitorWalletLink = page.getByRole("link", { name: "去访客/临时发放" })
  await expect(visitorWalletLink).toBeVisible()
  const href = await visitorWalletLink.getAttribute("href")
  expect(href).toBeTruthy()

  const nextURL = new URL(href ?? "", "http://localhost")
  expect(nextURL.pathname).toBe("/wallet")
  expect(nextURL.searchParams.get("scenario")).toBe("visitor_temporary")
  expect(nextURL.searchParams.get("from")).toBe("enterprise")
  expect(nextURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(nextURL.searchParams.get("stage")).toBe("issuance")
  expect(nextURL.searchParams.get("tenant_id")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("target_hint")).toBe("visitor")
  expect(nextURL.searchParams.get("target_email")).toBe("bob@sudirman.co")
  expect(nextURL.searchParams.get("target_name")).toBe("Bob Li")
  expect(nextURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(nextURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_alert_level")).toBe("hot")
  expect(nextURL.searchParams.get("worker_alert_tenant_id")).toBe("tenant-sudirman")
})

test("access grants worker review backflow link should carry handled status and issuance stage", async ({ page }) => {
  await login(page)
  await page.goto(
    "/access/grants?from=enterprise&flow=sync_to_access&stage=issuance&tenant_id=tenant-sudirman&worker_alert_level=hot&worker_alert_tenant_id=tenant-sudirman&worker_alert_failed=8&worker_alert_threshold=4&worker_filter_hint=hot&worker_query_hint=tenant-sudirman"
  )

  const reviewLink = page.getByRole("link", { name: "处理完成后回导入与同步复核" }).first()
  await expect(reviewLink).toBeVisible()
  await reviewLink.click()

  await expect(page).toHaveURL(/\/enterprise\?.*#sync$/)
  const nextURL = new URL(page.url())
  expect(nextURL.searchParams.get("flow")).toBe("sync_to_access")
  expect(nextURL.searchParams.get("sync_focus_hint")).toBe("worker_alert")
  expect(nextURL.searchParams.get("worker_review_status_hint")).toBe("handled")
  expect(nextURL.searchParams.get("worker_review_stage_hint")).toBe("issuance")
  expect(nextURL.searchParams.get("worker_filter_hint")).toBe("hot")
  expect(nextURL.searchParams.get("worker_query_hint")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_alert_level")).toBe("hot")
  expect(nextURL.searchParams.get("worker_alert_tenant_id")).toBe("tenant-sudirman")
  expect(nextURL.searchParams.get("worker_alert_failed")).toBe("8")
  expect(nextURL.searchParams.get("worker_alert_threshold")).toBe("4")
})
