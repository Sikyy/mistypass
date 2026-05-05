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

function buildEmptyWalletMetrics(tenantID: string) {
  const now = "2026-04-16T00:00:00Z"
  return {
    tenant_id: tenantID,
    max_retry: 3,
    dlq_alert_threshold: 20,
    summary: {
      tenant_id: tenantID,
      max_retry: 3,
      total: 0,
      pending: 0,
      processing: 0,
      success: 0,
      failed: 0,
      dlq: 0,
      retryable_failed: 0,
      non_retryable_failed: 0,
      error_code_breakdown: {},
      updated_at: now,
    },
    window: {
      window_seconds: 900,
      since: now,
      until: now,
      created: 0,
      updated: 0,
      pending: 0,
      processing: 0,
      success: 0,
      failed: 0,
      dlq: 0,
      error_code_breakdown: {},
    },
    alerts: [],
    updated_at: now,
  }
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

    if (path === "/api/v1/enterprise/scim/config" && method === "GET") {
      await fulfillJson(route, { endpoint: "", token_status: "inactive", supported_operations: [], setup_steps: [] })
      return
    }

    if (path === "/api/v1/enterprise/scim/logs" && method === "GET") {
      await fulfillJson(route, { items: [], total: 0 })
      return
    }

    if (path === "/api/v1/enterprise/idp-config" && method === "GET") {
      await fulfillJson(route, { message: "not found" }, 404)
      return
    }

    if (path === "/api/v1/wallet/jobs/metrics" && method === "GET") {
      const tenantID = url.searchParams.get("tenant_id") || viewer.tenant_id
      await fulfillJson(route, buildEmptyWalletMetrics(tenantID))
      return
    }

    if (path === "/api/v1/wallet/jobs/metrics/trend" && method === "GET") {
      const tenantID = url.searchParams.get("tenant_id") || viewer.tenant_id
      await fulfillJson(route, {
        ...buildEmptyWalletMetrics(tenantID),
        window_seconds: 900,
        bucket_seconds: 75,
        bucket_count: 12,
        since: "2026-04-16T00:00:00Z",
        until: "2026-04-16T00:00:00Z",
        buckets: [],
      })
      return
    }

    if (path === "/api/v1/wallet/jobs/alert-subscription" && method === "GET") {
      await fulfillJson(route, {
        tenant_id: url.searchParams.get("tenant_id") || viewer.tenant_id,
        enabled: false,
        dlq_alert_threshold: 20,
        window_seconds: 900,
        cooldown_seconds: 900,
        channels: {
          email: true,
          whatsapp: false,
        },
        receiver_groups: ["security"],
        updated_at: "2026-04-16T00:00:00Z",
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
  await expect(page).toHaveURL(/\/home$/)
}

test("super_admin should see grouped navigation with correct section titles", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-super-admin-nav-groups",
    email: "super.admin.nav@sudirman.co",
    role: "super_admin",
    tenant_id: "",
    building_ids: [],
  }
  await setupApiMocks(page, viewer)
  await login(page, viewer.email)

  const nav = page.locator("nav").first()
  await expect(nav.getByText("人员与权限", { exact: true })).toBeVisible()
  await expect(nav.getByText("场所", { exact: true }).first()).toBeVisible()
  await expect(nav.getByText("事件与报表", { exact: true })).toBeVisible()
  await expect(nav.getByText("组织设置", { exact: true })).toBeVisible()
})

test("enterprise entry should use platform health title and sync default for super_admin", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-super-admin-enterprise-entry",
    email: "super.admin.enterprise@sudirman.co",
    role: "super_admin",
    tenant_id: "",
    building_ids: [],
  }
  await setupApiMocks(page, viewer)
  await login(page, viewer.email)

  await page.goto("/enterprise")
  await expect(page.getByRole("heading", { name: "企业健康" })).toBeVisible()
  await expect(page.getByRole("tab", { name: "同步" })).toHaveAttribute("data-state", "active")
})

test("enterprise entry should use directory and SSO title with directory default for tenant_admin", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-tenant-admin-enterprise-entry",
    email: "tenant.admin.enterprise@sudirman.co",
    role: "tenant_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(page, viewer)
  await login(page, viewer.email)

  await page.goto("/enterprise")
  await expect(page.getByRole("heading", { name: "目录与登录" })).toBeVisible()
  await expect(page.getByRole("tab", { name: "员工" })).toHaveAttribute("data-state", "active")
  await expect(page.getByRole("tab", { name: "IDP" })).toBeVisible()
})

test("resident should stop at the no-permission page instead of entering app shell", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-resident-no-admin",
    email: "resident.noadmin@sudirman.co",
    role: "resident",
    tenant_id: "tenant-sudirman",
    building_ids: [],
  }
  await setupApiMocks(page, viewer)

  // Resident role renders NoPermissionPage directly (not through <Routes>),
  // so the URL may stay at /login instead of /home after navigation.
  // Avoid the login() helper which asserts toHaveURL(/\/home$/).
  await page.goto("/login")
  await page.getByLabel("邮箱").fill(viewer.email)
  await page.getByLabel("密码").fill("admin123")
  await page.getByRole("button", { name: "登录" }).click()

  await expect(page.getByText("当前账号不能进入管理后台")).toBeVisible()
  await expect(page.getByText("退出并切换账号")).toBeVisible()
  await expect(page.getByRole("link", { name: /仪表盘/ })).toHaveCount(0)
})

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

test("building_admin should stay on enterprise/access/wallet pages without redirect", async ({ page }) => {
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
  await expect(page).toHaveURL(/\/enterprise$/)

  await page.goto("/access/directory")
  await expect(page).toHaveURL(/\/access\/directory$/)

  await page.goto("/wallet")
  await expect(page).toHaveURL(/\/wallet$/)
})

test("wallet should keep advanced operations collapsed behind the daily issuance path", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-tenant-admin-wallet-advanced",
    email: "tenant.admin.wallet.advanced@sudirman.co",
    role: "tenant_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(page, viewer)
  await login(page, viewer.email)

  await page.goto("/wallet")
  await expect(page.getByTestId("wallet-operations-workspace")).toBeVisible()
  await expect(page.getByTestId("wallet-advanced-workspace")).toBeVisible()
  await expect(page.getByRole("tab", { name: "高级" })).toHaveCount(0)
  await expect(page.getByTestId("wallet-advanced-content")).toHaveCount(0)

  const toggle = page.getByTestId("wallet-advanced-toggle")
  await expect(toggle).toHaveText(/展开高级运营/)
  await expect(toggle).toHaveAttribute("aria-expanded", "false")
  await toggle.click()
  await expect(toggle).toHaveText(/收起高级运营/)
  await expect(toggle).toHaveAttribute("aria-expanded", "true")
  await expect(page.getByTestId("wallet-advanced-content")).toBeVisible()
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

test("tenant_admin gateways should keep setup registration folded by default", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-tenant-admin-gateway-density",
    email: "tenant.admin.gateway.density@sudirman.co",
    role: "tenant_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(page, viewer)
  await login(page, viewer.email)

  await page.goto("/gateways")
  await expect(page.getByText("网关配置入口")).toBeVisible()
  await expect(page.getByText("注册网关")).toHaveCount(0)

  const toggle = page.getByRole("button", { name: "展开配置" })
  await expect(toggle).toHaveAttribute("aria-expanded", "false")
  await toggle.click()
  await expect(page.getByRole("button", { name: "收起配置" })).toHaveAttribute("aria-expanded", "true")
  await expect(page.getByText("注册网关")).toBeVisible()
})

test("operator should stay on enterprise page without redirect", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-operator-enterprise-guard",
    email: "operator.enterprise.guard@sudirman.co",
    role: "operator",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(page, viewer)
  await login(page, viewer.email)

  await page.goto("/enterprise")
  await expect(page).toHaveURL(/\/enterprise$/)
})
