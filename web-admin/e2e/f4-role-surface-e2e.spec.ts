import { expect, test, type Page, type Route } from "@playwright/test"

type MockViewer = {
  id: string
  email: string
  role: string
  tenant_id: string
  building_ids: string[]
}

type MockAccessEvent = {
  id: string
  tenant_id: string
  building_id: string
  area_id: string
  type: string
  actor: string
  door_id: string
  gateway_id: string
  result: string
  at: string
}

type MockDeviceEvent = {
  id: string
  tenant_id: string
  building_id: string
  type: string
  gateway_id: string
  detail: string
  result: string
  at: string
}

type MockAlarm = {
  id: string
  tenant_id: string
  building_id: string
  area_id: string
  door_id: string
  type: string
  severity: string
  location: string
  status: string
  created_at: string
}

type MockGateway = {
  id: string
  tenant_id: string
  serial_number: string
  building_id: string
  device_capacity: number
  devices: Array<{
    id: string
    gateway_id: string
    serial_number: string
    kind: string
    source: string
    status: string
    last_seen_at: string
  }>
  status: string
  last_seen_at: string
  bound_door_ids: string[]
}

type MockDoor = {
  id: string
  tenant_id: string
  building_id: string
  floor_id: string
  area_id: string
  name: string
  gateway_id: string
  kind: string
  status: string
  created_at: string
}

type MockUserGroup = {
  id: string
  tenant_id: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

type MockSerialInventory = {
  id: string
  tenant_id: string
  serial_number: string
  product_type: string
  status: string
  created_at: string
  updated_at: string
}

type MockScenario = {
  accessEvents: MockAccessEvent[]
  deviceEvents: MockDeviceEvent[]
  alarms: MockAlarm[]
  gateways: MockGateway[]
  doors: MockDoor[]
  userGroups: MockUserGroup[]
  serialInventory: MockSerialInventory[]
}

type MockCounters = {
  alarmStatusCalls: number
  alarmStatusUpdates: number
  bindDoorCalls: number
  bindDoorUpdates: number
}

type MockErrorConfig = {
  alarmStatusErrorIDs?: string[]
  bindDoorErrorGatewayIDs?: string[]
  publishConfigErrorGatewayIDs?: string[]
  rebootErrorGatewayIDs?: string[]
  exportInventoryError?: boolean
}

const now = "2026-04-16T00:00:00Z"

function buildLoginResponse(viewer: MockViewer) {
  return {
    access_token: "e2e-token",
    refresh_token: "e2e-refresh",
    expires_in: 3600,
    user: viewer,
  }
}

function buildScenario(overrides: Partial<MockScenario> = {}): MockScenario {
  return {
    accessEvents: [],
    deviceEvents: [],
    alarms: [],
    gateways: [],
    doors: [],
    userGroups: [],
    serialInventory: [],
    ...overrides,
  }
}

function buildGatewayCommandAck(gatewayID: string, command: string) {
  return {
    task_id: `${command}-task-1`,
    gateway_id: gatewayID,
    command,
    status: "queued",
    created_at: now,
  }
}

async function fulfillJson(route: Route, payload: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(payload),
  })
}

async function setupApiMocks(
  page: Page,
  viewer: MockViewer,
  seedScenario: MockScenario,
  errorConfig: MockErrorConfig = {}
): Promise<MockCounters> {
  const loginResponse = buildLoginResponse(viewer)
  const scenario: MockScenario = {
    accessEvents: [...seedScenario.accessEvents],
    deviceEvents: [...seedScenario.deviceEvents],
    alarms: seedScenario.alarms.map((item) => ({ ...item })),
    gateways: seedScenario.gateways.map((item) => ({
      ...item,
      devices: [...item.devices],
      bound_door_ids: [...item.bound_door_ids],
    })),
    doors: [...seedScenario.doors],
    userGroups: [...seedScenario.userGroups],
    serialInventory: [...seedScenario.serialInventory],
  }
  const counters: MockCounters = {
    alarmStatusCalls: 0,
    alarmStatusUpdates: 0,
    bindDoorCalls: 0,
    bindDoorUpdates: 0,
  }
  const alarmStatusErrorIDs = new Set(errorConfig.alarmStatusErrorIDs ?? [])
  const bindDoorErrorGatewayIDs = new Set(errorConfig.bindDoorErrorGatewayIDs ?? [])
  const publishConfigErrorGatewayIDs = new Set(errorConfig.publishConfigErrorGatewayIDs ?? [])
  const rebootErrorGatewayIDs = new Set(errorConfig.rebootErrorGatewayIDs ?? [])

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

    if (path === "/api/v1/events/access" && method === "GET") {
      await fulfillJson(route, { items: scenario.accessEvents })
      return
    }

    if (path === "/api/v1/events/device" && method === "GET") {
      await fulfillJson(route, { items: scenario.deviceEvents })
      return
    }

    if (path === "/api/v1/alarms" && method === "GET") {
      await fulfillJson(route, { items: scenario.alarms })
      return
    }

    if (path.startsWith("/api/v1/alarms/") && path.endsWith("/status") && method === "PATCH") {
      const alarmID = path.replace("/api/v1/alarms/", "").replace("/status", "")
      const payload = request.postDataJSON() as { status?: string }
      const nextStatus = payload.status || "acknowledged"
      counters.alarmStatusCalls += 1
      if (alarmStatusErrorIDs.has(alarmID)) {
        await fulfillJson(route, { error: `mock alarm status update failed: ${alarmID}` }, 500)
        return
      }
      counters.alarmStatusUpdates += 1
      const index = scenario.alarms.findIndex((item) => item.id === alarmID)
      if (index < 0) {
        await fulfillJson(route, { error: "mock alarm not found" }, 404)
        return
      }
      scenario.alarms[index] = {
        ...scenario.alarms[index],
        status: nextStatus,
      }
      await fulfillJson(route, scenario.alarms[index])
      return
    }

    if (path === "/api/v1/user-groups" && method === "GET") {
      await fulfillJson(route, { items: scenario.userGroups })
      return
    }

    if (path === "/api/v1/gateways" && method === "GET") {
      await fulfillJson(route, { items: scenario.gateways })
      return
    }

    if (path === "/api/v1/doors" && method === "GET") {
      await fulfillJson(route, { items: scenario.doors })
      return
    }

    if (path === "/api/v1/gateways/serial-inventory" && method === "GET") {
      await fulfillJson(route, { items: scenario.serialInventory })
      return
    }

    if (
      (path === "/api/v1/gateways/events/checkpoint/summary" || path === "/api/v1/gateway/events/checkpoint/summary") &&
      method === "GET"
    ) {
      await fulfillJson(route, {
        items: [],
        totals: {
          queues: 0,
          event_total: 0,
          acked_total: 0,
          lag_total: 0,
        },
        time_window_trend: {
          window_minutes: Number(url.searchParams.get("trend_window_minutes") || 60),
          since: now,
          until: now,
          report_total: 0,
          gateway_total: 0,
          queue_total: 0,
          acked_delta_total: 0,
          direction: "flat",
          last_report_at: now,
        },
      })
      return
    }

    if (path.startsWith("/api/v1/gateways/") && path.endsWith("/bind-door") && method === "POST") {
      const gatewayID = path.replace("/api/v1/gateways/", "").replace("/bind-door", "")
      const payload = request.postDataJSON() as { door_id?: string }
      const doorID = payload.door_id?.trim() || ""
      counters.bindDoorCalls += 1
      if (bindDoorErrorGatewayIDs.has(gatewayID)) {
        await fulfillJson(route, { error: `mock bind door failed: ${gatewayID}` }, 500)
        return
      }
      const index = scenario.gateways.findIndex((item) => item.id === gatewayID)
      if (index < 0) {
        await fulfillJson(route, { error: "mock gateway not found" }, 404)
        return
      }
      counters.bindDoorUpdates += 1
      const nextBound = new Set(scenario.gateways[index].bound_door_ids)
      if (doorID) {
        nextBound.add(doorID)
      }
      scenario.gateways[index] = {
        ...scenario.gateways[index],
        bound_door_ids: Array.from(nextBound),
      }
      await fulfillJson(route, scenario.gateways[index])
      return
    }

    if (path.startsWith("/api/v1/gateways/serial-inventory/export-csv") && method === "GET") {
      if (errorConfig.exportInventoryError) {
        await fulfillJson(route, { error: "mock export inventory csv failed" }, 500)
        return
      }
      await route.fulfill({
        status: 200,
        contentType: "text/csv; charset=utf-8",
        body: "serial_number,product_type,status\nINV-001,gateway,available\n",
      })
      return
    }

    if (path.startsWith("/api/v1/gateways/") && path.endsWith("/config/publish") && method === "POST") {
      const gatewayID = path.replace("/api/v1/gateways/", "").replace("/config/publish", "")
      if (publishConfigErrorGatewayIDs.has(gatewayID)) {
        await fulfillJson(route, { error: `mock publish config failed: ${gatewayID}` }, 500)
        return
      }
      await fulfillJson(route, buildGatewayCommandAck(gatewayID, "publish_config"))
      return
    }

    if (path.startsWith("/api/v1/gateways/") && path.endsWith("/reboot") && method === "POST") {
      const gatewayID = path.replace("/api/v1/gateways/", "").replace("/reboot", "")
      if (rebootErrorGatewayIDs.has(gatewayID)) {
        await fulfillJson(route, { error: `mock reboot failed: ${gatewayID}` }, 500)
        return
      }
      await fulfillJson(route, buildGatewayCommandAck(gatewayID, "reboot_gateway"))
      return
    }

    if (path === "/api/v1/tenants" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    await fulfillJson(route, {})
  })

  return counters
}

async function login(page: Page, email: string) {
  await page.goto("/login")
  await page.getByRole("button", { name: "中文" }).click()
  await page.getByLabel("邮箱").fill(email)
  await page.getByLabel("密码").fill("admin123")
  await page.getByRole("button", { name: "登录" }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

test("building_admin scoped pages should only show owned building records", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-building-admin-scoped",
    email: "building.admin.scoped@sudirman.co",
    role: "building_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(
    page,
    viewer,
    buildScenario({
      accessEvents: [
        {
          id: "evt-access-b1",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          area_id: "area-1",
          type: "access_granted",
          actor: "alice@sudirman.co",
          door_id: "door-1",
          gateway_id: "gw-scope-1",
          result: "success",
          at: now,
        },
        {
          id: "evt-access-b2",
          tenant_id: viewer.tenant_id,
          building_id: "building-2",
          area_id: "area-2",
          type: "access_denied",
          actor: "bob@sudirman.co",
          door_id: "door-2",
          gateway_id: "gw-scope-2",
          result: "denied",
          at: now,
        },
      ],
      deviceEvents: [
        {
          id: "evt-device-b1",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          type: "heartbeat",
          gateway_id: "gw-scope-1",
          detail: "ok",
          result: "warning",
          at: now,
        },
        {
          id: "evt-device-b2",
          tenant_id: viewer.tenant_id,
          building_id: "building-2",
          type: "heartbeat",
          gateway_id: "gw-scope-2",
          detail: "lag",
          result: "warning",
          at: now,
        },
      ],
      alarms: [
        {
          id: "alarm-b1",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          area_id: "area-1",
          door_id: "door-1",
          type: "door_forced_open",
          severity: "high",
          location: "Tower A / L1",
          status: "open",
          created_at: now,
        },
        {
          id: "alarm-b2",
          tenant_id: viewer.tenant_id,
          building_id: "building-2",
          area_id: "area-2",
          door_id: "door-2",
          type: "door_forced_open",
          severity: "high",
          location: "Tower B / L3",
          status: "open",
          created_at: now,
        },
      ],
      gateways: [
        {
          id: "gw-scope-1",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-B1-001",
          building_id: "building-1",
          device_capacity: 4,
          devices: [],
          status: "online",
          last_seen_at: now,
          bound_door_ids: [],
        },
        {
          id: "gw-scope-2",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-B2-001",
          building_id: "building-2",
          device_capacity: 4,
          devices: [],
          status: "online",
          last_seen_at: now,
          bound_door_ids: [],
        },
      ],
      doors: [
        {
          id: "door-1",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          floor_id: "floor-1",
          area_id: "area-1",
          name: "L1 Main Door",
          gateway_id: "gw-scope-1",
          kind: "office",
          status: "online",
          created_at: now,
        },
        {
          id: "door-2",
          tenant_id: viewer.tenant_id,
          building_id: "building-2",
          floor_id: "floor-2",
          area_id: "area-2",
          name: "L2 Main Door",
          gateway_id: "gw-scope-2",
          kind: "office",
          status: "online",
          created_at: now,
        },
      ],
    })
  )
  await login(page, viewer.email)

  await page.goto("/events")
  await expect(page.getByText("evt-access-b1")).toBeVisible()
  await expect(page.getByText("evt-device-b1")).toBeVisible()
  await expect(page.getByText("evt-access-b2")).toHaveCount(0)
  await expect(page.getByText("evt-device-b2")).toHaveCount(0)

  await page.goto("/alarms")
  await expect(page.getByText("alarm-b1")).toBeVisible()
  await expect(page.getByText("alarm-b2")).toHaveCount(0)

  await page.goto("/gateways")
  await expect(page.getByRole("cell", { name: "gw-scope-1" })).toBeVisible()
  await expect(page.getByRole("cell", { name: "gw-scope-2" })).toHaveCount(0)
})

test("building_admin should update alarm workflow and write notification logs", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-building-admin-alarm-write",
    email: "building.admin.alarm.write@sudirman.co",
    role: "building_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  const counters = await setupApiMocks(
    page,
    viewer,
    buildScenario({
      alarms: [
        {
          id: "alarm-scope-1",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          area_id: "area-1",
          door_id: "door-1",
          type: "door_forced_open",
          severity: "high",
          location: "Tower A / L1",
          status: "open",
          created_at: now,
        },
      ],
      userGroups: [
        {
          id: "ug-1",
          tenant_id: viewer.tenant_id,
          name: "值守组",
          description: "Security",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )
  await login(page, viewer.email)

  await page.goto("/alarms")
  await expect(page.getByRole("heading", { name: "Open" })).toBeVisible()
  await expect(page.getByRole("heading", { name: "Investigating" })).toBeVisible()
  await expect(page.getByRole("heading", { name: "Mitigated" })).toBeVisible()
  await expect(page.getByRole("heading", { name: "Closed" })).toBeVisible()
  const card = page.getByTestId("alarm-card").filter({ hasText: "alarm-scope-1" })
  await card.getByRole("button", { name: "确认" }).click()

  await expect(card.getByText("已确认")).toBeVisible()
  await page.getByRole("tab", { name: "通知配置" }).click()
  await expect(page.getByText("暂无通知记录，更新任一告警状态后会自动写入。")).toHaveCount(0)
  await expect(page.getByRole("cell", { name: "alarm-scope-1" }).first()).toBeVisible()
  await expect(page.getByRole("cell", { name: "邮件" }).first()).toBeVisible()
  expect(counters.alarmStatusUpdates).toBe(1)
})

test("building_admin gateways should keep write ops but hide register entry", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-building-admin-gateway-write",
    email: "building.admin.gateway.write@sudirman.co",
    role: "building_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  const counters = await setupApiMocks(
    page,
    viewer,
    buildScenario({
      gateways: [
        {
          id: "gw-scope-1",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-B1-001",
          building_id: "building-1",
          device_capacity: 4,
          devices: [],
          status: "online",
          last_seen_at: now,
          bound_door_ids: [],
        },
      ],
      doors: [
        {
          id: "door-1",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          floor_id: "floor-1",
          area_id: "area-1",
          name: "L1 Main Door",
          gateway_id: "gw-scope-1",
          kind: "office",
          status: "online",
          created_at: now,
        },
      ],
    })
  )
  await login(page, viewer.email)

  await page.goto("/gateways")
  await expect(page.getByText("注册网关")).toHaveCount(0)
  await expect(page.getByRole("button", { name: "绑定门点" })).toBeEnabled()
  await expect(page.getByRole("button", { name: "发布配置" })).toBeEnabled()

  await page.getByRole("button", { name: "绑定门点" }).click()
  await expect(page.getByText("门点 door-1 已绑定到 gw-scope-1")).toBeVisible()
  expect(counters.bindDoorUpdates).toBe(1)
})

test("operator gateways should keep read-only controls disabled while export stays available", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-operator-gateway-readonly",
    email: "operator.gateway.readonly@sudirman.co",
    role: "operator",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(
    page,
    viewer,
    buildScenario({
      gateways: [
        {
          id: "gw-operator-1",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-OP-001",
          building_id: "building-1",
          device_capacity: 4,
          devices: [],
          status: "online",
          last_seen_at: now,
          bound_door_ids: [],
        },
      ],
      doors: [
        {
          id: "door-1",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          floor_id: "floor-1",
          area_id: "area-1",
          name: "L1 Main Door",
          gateway_id: "gw-operator-1",
          kind: "office",
          status: "online",
          created_at: now,
        },
      ],
      serialInventory: [
        {
          id: "inv-1",
          tenant_id: viewer.tenant_id,
          serial_number: "INV-001",
          product_type: "gateway",
          status: "available",
          created_at: now,
          updated_at: now,
        },
      ],
    })
  )
  await login(page, viewer.email)

  await page.goto("/gateways")
  await expect(page.getByText("注册网关")).toHaveCount(0)
  await expect(page.getByRole("button", { name: "绑定门点" })).toBeDisabled()
  await expect(page.getByRole("button", { name: "发布配置" })).toBeDisabled()
  await expect(page.getByRole("button", { name: "重启" })).toBeDisabled()
  await expect(page.getByRole("button", { name: "导出当前筛选 CSV" })).toBeEnabled()
  await expect(page.getByRole("button", { name: "入库" })).toHaveCount(0)
})

test("tenant_admin empty-state copy should stay consistent across events and alarms", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-tenant-admin-empty-state",
    email: "tenant.admin.empty.state@sudirman.co",
    role: "tenant_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(
    page,
    viewer,
    buildScenario({
      accessEvents: [],
      deviceEvents: [],
      alarms: [],
    })
  )
  await login(page, viewer.email)

  await page.goto("/events")
  await expect(page.getByText("当前范围内暂无事件。")).toBeVisible()

  await page.goto("/alarms")
  await expect(page.getByText("当前范围内暂无告警。")).toBeVisible()
})

test("tenant_admin events filter should support type switch, empty result hint and reset", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-tenant-admin-events-filter",
    email: "tenant.admin.events.filter@sudirman.co",
    role: "tenant_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(
    page,
    viewer,
    buildScenario({
      accessEvents: [
        {
          id: "evt-filter-access",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          area_id: "area-1",
          type: "access_granted",
          actor: "alice@sudirman.co",
          door_id: "door-1",
          gateway_id: "gw-filter-1",
          result: "success",
          at: now,
        },
      ],
      deviceEvents: [
        {
          id: "evt-filter-gateway",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          type: "heartbeat",
          gateway_id: "gw-filter-1",
          detail: "lag",
          result: "warning",
          at: now,
        },
      ],
    })
  )
  await login(page, viewer.email)

  await page.goto("/events")
  await expect(page.getByRole("combobox", { name: "时间范围" })).toContainText("最近 24 小时")
  await expect(page.getByText("evt-filter-access")).toBeVisible()
  await expect(page.getByText("evt-filter-gateway")).toBeVisible()

  await page.getByRole("combobox", { name: "事件类型" }).click()
  await page.getByRole("option", { name: "网关事件（gateway_event）" }).click()
  await expect(page.getByText("evt-filter-gateway")).toBeVisible()
  await expect(page.getByText("evt-filter-access")).toHaveCount(0)

  await page.getByPlaceholder("按事件ID/楼宇/区域/网关/门点搜索").fill("NO-MATCH")
  await expect(page.getByText("当前筛选条件下没有匹配的事件。")).toBeVisible()

  await page.getByRole("button", { name: "重置筛选" }).click()
  await expect(page.getByText("evt-filter-access")).toBeVisible()
  await expect(page.getByText("evt-filter-gateway")).toBeVisible()

  await page.getByTestId("event-row").filter({ hasText: "evt-filter-gateway" }).click()
  const detailDrawer = page.getByRole("dialog", { name: "evt-filter-gateway" })
  await expect(detailDrawer).toBeVisible()
  await expect(detailDrawer.getByText("原始 JSON", { exact: true })).toBeVisible()
  await expect(detailDrawer.getByText(/"id": "evt-filter-gateway"/)).toBeVisible()
})

test("tenant_admin gateways filter should support status switch, empty result hint and reset", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-tenant-admin-gateway-filter",
    email: "tenant.admin.gateway.filter@sudirman.co",
    role: "tenant_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(
    page,
    viewer,
    buildScenario({
      gateways: [
        {
          id: "gw-filter-online",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-FILTER-ONLINE",
          building_id: "building-1",
          device_capacity: 4,
          devices: [],
          status: "online",
          last_seen_at: now,
          bound_door_ids: [],
        },
        {
          id: "gw-filter-offline",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-FILTER-OFFLINE",
          building_id: "building-1",
          device_capacity: 4,
          devices: [],
          status: "offline",
          last_seen_at: now,
          bound_door_ids: [],
        },
      ],
    })
  )
  await login(page, viewer.email)

  await page.goto("/gateways")
  await expect(page.getByRole("cell", { name: /^gw-filter-online$/ })).toBeVisible()
  await expect(page.getByRole("cell", { name: /^gw-filter-offline$/ })).toBeVisible()

  await page.getByLabel("网关状态筛选").click()
  await page.getByRole("option", { name: "离线" }).click()
  await expect(page.getByRole("cell", { name: /^gw-filter-offline$/ })).toBeVisible()
  await expect(page.getByRole("cell", { name: /^gw-filter-online$/ })).toHaveCount(0)

  await page.getByPlaceholder("搜索网关记录").fill("NO-MATCH")
  await expect(page.getByText("当前筛选条件下没有匹配的网关。")).toBeVisible()

  await page.getByRole("button", { name: "重置网关筛选" }).click()
  await expect(page.getByRole("cell", { name: /^gw-filter-online$/ })).toBeVisible()
  await expect(page.getByRole("cell", { name: /^gw-filter-offline$/ })).toBeVisible()
})

test("tenant_admin alarm filters should support keyword, severity, status and reset", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-tenant-admin-alarm-filter",
    email: "tenant.admin.alarm.filter@sudirman.co",
    role: "tenant_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(
    page,
    viewer,
    buildScenario({
      alarms: [
        {
          id: "alarm-filter-open",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          area_id: "area-1",
          door_id: "door-1",
          type: "door_forced_open",
          severity: "high",
          location: "Tower A / L1",
          status: "open",
          created_at: now,
        },
        {
          id: "alarm-filter-resolved",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          area_id: "area-2",
          door_id: "door-2",
          type: "door_forced_open",
          severity: "low",
          location: "Tower B / L2",
          status: "resolved",
          created_at: now,
        },
      ],
    })
  )
  await login(page, viewer.email)

  await page.goto("/alarms")
  await expect(page.getByText("alarm-filter-open")).toBeVisible()
  await expect(page.getByText("alarm-filter-resolved")).toBeVisible()

  await page.getByPlaceholder("按告警ID/楼宇/区域/门点/类型搜索").fill("Tower A")
  await expect(page.getByText("alarm-filter-open")).toBeVisible()
  await expect(page.getByText("alarm-filter-resolved")).toHaveCount(0)

  await page.getByLabel("告警等级筛选").click()
  await page.getByRole("option", { name: "高" }).click()
  await expect(page.getByText("alarm-filter-open")).toBeVisible()

  await page.getByLabel("告警状态筛选").click()
  await page.getByRole("option", { name: "已关闭" }).click()
  await expect(page.getByText("当前筛选条件下没有匹配的告警。")).toBeVisible()

  await page.getByRole("button", { name: "重置筛选" }).click()
  await expect(page.getByText("alarm-filter-open")).toBeVisible()
  await expect(page.getByText("alarm-filter-resolved")).toBeVisible()
})

test("building_admin alarm status update failure should show api error and keep notification log empty", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-building-admin-alarm-failure",
    email: "building.admin.alarm.failure@sudirman.co",
    role: "building_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  const counters = await setupApiMocks(
    page,
    viewer,
    buildScenario({
      alarms: [
        {
          id: "alarm-scope-fail",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          area_id: "area-1",
          door_id: "door-1",
          type: "door_forced_open",
          severity: "high",
          location: "Tower A / L1",
          status: "open",
          created_at: now,
        },
      ],
      userGroups: [
        {
          id: "ug-1",
          tenant_id: viewer.tenant_id,
          name: "值守组",
          description: "Security",
          created_at: now,
          updated_at: now,
        },
      ],
    }),
    {
      alarmStatusErrorIDs: ["alarm-scope-fail"],
    }
  )
  await login(page, viewer.email)

  await page.goto("/alarms")
  const card = page.getByTestId("alarm-card").filter({ hasText: "alarm-scope-fail" })
  await card.getByRole("button", { name: "确认" }).click()

  await expect(page.getByText("mock alarm status update failed: alarm-scope-fail")).toBeVisible()
  await page.getByRole("tab", { name: "通知配置" }).click()
  await expect(page.getByText("暂无通知记录，更新任一告警状态后会自动写入。")).toBeVisible()
  expect(counters.alarmStatusCalls).toBe(1)
  expect(counters.alarmStatusUpdates).toBe(0)
})

test("building_admin bind door failure should show api error and keep gateway unchanged", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-building-admin-bind-failure",
    email: "building.admin.bind.failure@sudirman.co",
    role: "building_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  const counters = await setupApiMocks(
    page,
    viewer,
    buildScenario({
      gateways: [
        {
          id: "gw-bind-fail",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-BIND-FAIL-001",
          building_id: "building-1",
          device_capacity: 4,
          devices: [],
          status: "online",
          last_seen_at: now,
          bound_door_ids: [],
        },
      ],
      doors: [
        {
          id: "door-bind-fail",
          tenant_id: viewer.tenant_id,
          building_id: "building-1",
          floor_id: "floor-1",
          area_id: "area-1",
          name: "L1 Bind Fail Door",
          gateway_id: "gw-bind-fail",
          kind: "office",
          status: "online",
          created_at: now,
        },
      ],
    }),
    {
      bindDoorErrorGatewayIDs: ["gw-bind-fail"],
    }
  )
  await login(page, viewer.email)

  await page.goto("/gateways")
  await page.getByRole("button", { name: "绑定门点" }).click()

  await expect(page.getByText("mock bind door failed: gw-bind-fail")).toBeVisible()
  expect(counters.bindDoorCalls).toBe(1)
  expect(counters.bindDoorUpdates).toBe(0)
})

test("building_admin publish config failure should show api error", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-building-admin-publish-failure",
    email: "building.admin.publish.failure@sudirman.co",
    role: "building_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(
    page,
    viewer,
    buildScenario({
      gateways: [
        {
          id: "gw-publish-fail",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-PUBLISH-FAIL-001",
          building_id: "building-1",
          device_capacity: 4,
          devices: [],
          status: "online",
          last_seen_at: now,
          bound_door_ids: [],
        },
      ],
    }),
    {
      publishConfigErrorGatewayIDs: ["gw-publish-fail"],
    }
  )
  await login(page, viewer.email)

  await page.goto("/gateways")
  await page.getByRole("button", { name: "发布配置" }).click()
  await expect(page.getByText("mock publish config failed: gw-publish-fail")).toBeVisible()
})

test("building_admin reboot failure should show api error", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-building-admin-reboot-failure",
    email: "building.admin.reboot.failure@sudirman.co",
    role: "building_admin",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(
    page,
    viewer,
    buildScenario({
      gateways: [
        {
          id: "gw-reboot-fail",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-REBOOT-FAIL-001",
          building_id: "building-1",
          device_capacity: 4,
          devices: [],
          status: "online",
          last_seen_at: now,
          bound_door_ids: [],
        },
      ],
    }),
    {
      rebootErrorGatewayIDs: ["gw-reboot-fail"],
    }
  )
  await login(page, viewer.email)

  await page.goto("/gateways")
  await page.getByRole("button", { name: "重启" }).click()
  await expect(page.getByRole("dialog", { name: "确认重启网关" })).toBeVisible()
  await page.getByRole("button", { name: "确认重启" }).click()
  await expect(page.getByText("mock reboot failed: gw-reboot-fail")).toBeVisible()
})

test("operator export inventory csv failure should show api error", async ({ page }) => {
  const viewer: MockViewer = {
    id: "user-operator-export-failure",
    email: "operator.export.failure@sudirman.co",
    role: "operator",
    tenant_id: "tenant-sudirman",
    building_ids: ["building-1"],
  }
  await setupApiMocks(
    page,
    viewer,
    buildScenario({
      gateways: [
        {
          id: "gw-operator-export",
          tenant_id: viewer.tenant_id,
          serial_number: "GW-EXPORT-001",
          building_id: "building-1",
          device_capacity: 4,
          devices: [],
          status: "online",
          last_seen_at: now,
          bound_door_ids: [],
        },
      ],
      serialInventory: [
        {
          id: "inv-export-1",
          tenant_id: viewer.tenant_id,
          serial_number: "INV-EXPORT-001",
          product_type: "gateway",
          status: "available",
          created_at: now,
          updated_at: now,
        },
      ],
    }),
    {
      exportInventoryError: true,
    }
  )
  await login(page, viewer.email)

  await page.goto("/gateways")
  await page.getByRole("button", { name: "导出当前筛选 CSV" }).click()
  await expect(page.getByText("mock export inventory csv failed")).toBeVisible()
})
