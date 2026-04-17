import { expect, test, type Page, type Route } from "@playwright/test"

type MockViewer = {
  id: string
  email: string
  role: string
  tenant_id: string
  building_ids: string[]
}

function buildLoginResponse(viewer: MockViewer) {
  return {
    access_token: "e2e-token",
    refresh_token: "e2e-refresh",
    expires_in: 3600,
    user: viewer,
  }
}

async function fulfillJson(route: Route, payload: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(payload),
  })
}

async function setupApiMocks(page: Page, viewer: MockViewer) {
  const loginResponse = buildLoginResponse(viewer)
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

    if (path === "/api/v1/gateway/events/checkpoint/summary" && method === "GET") {
      await fulfillJson(route, {
        items: [],
        totals: {
          queues: 0,
          event_total: 0,
          acked_total: 0,
          lag_total: 0,
        },
        time_window_trend: {
          window_minutes: 60,
          since: "2026-04-16T00:00:00Z",
          until: "2026-04-16T00:00:00Z",
          report_total: 0,
          gateway_total: 0,
          queue_total: 0,
          acked_delta_total: 0,
          direction: "flat",
          last_report_at: "2026-04-16T00:00:00Z",
        },
      })
      return
    }

    if (method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    await fulfillJson(route, {})
  })
}

async function login(page: Page, email: string) {
  await page.goto("/login")
  await page.getByLabel("邮箱").fill(email)
  await page.getByLabel("密码").fill("admin123")
  await page.getByRole("button", { name: "登录" }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

test("building_admin without building scope should show empty-scope boundary hints across pages", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-building-admin-empty-scope",
    email: "building.admin.empty@sudirman.co",
    role: "building_admin",
    tenant_id: "tenant-sudirman",
    building_ids: [],
  }
  await setupApiMocks(page, viewer)
  await login(page, viewer.email)

  await expect(
    page.getByText("当前楼宇管理员尚未分配 `building_ids` 范围。仪表盘只保留空态指标，不展示任何楼宇级运行数据。")
  ).toBeVisible()

  await page.goto("/spaces")
  await expect(
    page.getByText(
      "当前楼宇管理员尚未分配 `building_ids` 范围。此页不会展示任何楼宇、楼层、区域或门点数据，也不会开放新增操作。"
    )
  ).toBeVisible()

  await page.goto("/events")
  await expect(
    page.getByText("当前楼宇管理员尚未分配 `building_ids` 范围。此页不会展示任何事件记录，避免误暴露非本楼宇数据。")
  ).toBeVisible()

  await page.goto("/alarms")
  await expect(
    page.getByText("当前楼宇管理员尚未分配 `building_ids` 范围。此页不会展示任何告警记录，避免误处置非本楼宇事件。")
  ).toBeVisible()

  await page.goto("/gateways")
  await expect(
    page.getByText(
      "当前楼宇管理员尚未分配 `building_ids` 范围。此页不会展示任何网关、门点或边缘设备数据，避免误操作非本楼宇设备。"
    )
  ).toBeVisible()
})

test("building_admin should be redirected by enterprise/access/wallet route guards", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-building-admin-guard",
    email: "building.admin.guard@sudirman.co",
    role: "building_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(page, viewer)
  await login(page, viewer.email)

  await page.goto("/enterprise")
  await expect(page).toHaveURL(/\/dashboard$/)

  await page.goto("/access/directory")
  await expect(page).toHaveURL(/\/dashboard$/)

  await page.goto("/wallet")
  await expect(page).toHaveURL(/\/dashboard$/)
})

test("operator should see read-only boundary hints on gateways page", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-operator-readonly",
    email: "operator.readonly@sudirman.co",
    role: "operator",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(page, viewer)
  await login(page, viewer.email)

  await page.goto("/gateways")
  await expect(
    page.getByText("当前角色为只读库存视图，仅支持导出当前筛选结果。按钮禁用或缺失属于权限边界，不是系统异常。")
  ).toBeVisible()
  await expect(
    page.getByText("当前角色无网关写权限，仅可查看状态。按钮禁用或缺失属于权限边界，不是系统异常。")
  ).toBeVisible()
})
