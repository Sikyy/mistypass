import { expect, test, type Page, type Route } from "@playwright/test"

type MockViewer = {
  id: string
  email: string
  role: string
  tenant_id: string
  building_ids: string[]
}

const now = "2026-04-28T10:00:00Z"
const tenantID = "tenant_demo_jakarta"

const viewer = {
  id: "usr_place_admin_sudirman_001",
  email: "place.admin.sudirman@mistypass.local",
  role: "building_admin",
  tenant_id: tenantID,
  building_ids: [],
} satisfies MockViewer

const assignedPlace = {
  id: "building_demo_001",
  tenant_id: tenantID,
  name: "Sudirman Hub",
  address: "Jl. Jend. Sudirman, Jakarta",
  region: "Jakarta",
  created_at: now,
}

const unassignedPlace = {
  id: "building_demo_002",
  tenant_id: tenantID,
  name: "Kuningan Tower",
  address: "Kuningan, Jakarta",
  region: "Jakarta",
  created_at: now,
}

const assignedFloor = {
  id: "floor_demo_001",
  tenant_id: tenantID,
  building_id: assignedPlace.id,
  name: "Lobby",
  created_at: now,
}

const unassignedFloor = {
  id: "floor_demo_003",
  tenant_id: tenantID,
  building_id: unassignedPlace.id,
  name: "Kuningan 1F",
  created_at: now,
}

const assignedArea = {
  id: "area_demo_001",
  tenant_id: tenantID,
  building_id: assignedPlace.id,
  floor_id: assignedFloor.id,
  name: "Reception",
  created_at: now,
}

const unassignedArea = {
  id: "area_demo_003",
  tenant_id: tenantID,
  building_id: unassignedPlace.id,
  floor_id: unassignedFloor.id,
  name: "Server Wing",
  created_at: now,
}

const assignedLock = {
  id: "door_jkt_001",
  tenant_id: tenantID,
  building_id: assignedPlace.id,
  floor_id: assignedFloor.id,
  area_id: assignedArea.id,
  name: "Sudirman Lobby",
  gateway_id: "controller_jkt_001",
  kind: "turnstile",
  status: "online",
  created_at: now,
}

const unassignedLock = {
  id: "door_jkt_014",
  tenant_id: tenantID,
  building_id: unassignedPlace.id,
  floor_id: unassignedFloor.id,
  area_id: unassignedArea.id,
  name: "Kuningan Server Room",
  gateway_id: "controller_jkt_014",
  kind: "server-room",
  status: "online",
  created_at: now,
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

async function setupApiMocks(page: Page) {
  const mutationRequests: string[] = []

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

    if (path === "/api/v1/buildings" && method === "GET") {
      await fulfillJson(route, { items: [assignedPlace] })
      return
    }

    if (path === "/api/v1/places" && method === "GET") {
      await fulfillJson(route, { items: [assignedPlace] })
      return
    }

    if (path === `/api/v1/places/${unassignedPlace.id}` && method === "GET") {
      await fulfillJson(route, { error: "forbidden" }, 403)
      return
    }

    if (path === "/api/v1/floors" && method === "GET") {
      await fulfillJson(route, { items: [assignedFloor, unassignedFloor] })
      return
    }

    if (path === "/api/v1/areas" && method === "GET") {
      await fulfillJson(route, { items: [assignedArea, unassignedArea] })
      return
    }

    if (path === "/api/v1/locks" && method === "GET") {
      await fulfillJson(route, { items: [assignedLock, unassignedLock] })
      return
    }

    if (path === `/api/v1/locks/${assignedLock.id}` && method === "GET") {
      await fulfillJson(route, assignedLock)
      return
    }

    if (path === `/api/v1/locks/${unassignedLock.id}` && method === "GET") {
      await fulfillJson(route, { error: "forbidden" }, 403)
      return
    }

    if (path === "/api/v1/controllers" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            id: "controller_jkt_001",
            resource_type: "Controller",
            tenant_id: tenantID,
            place_id: assignedPlace.id,
            name: "Sudirman Controller",
            device_id: "MP-GW-JKT-001",
            token: "MP-GW-JKT-001",
            status: "online",
            configured: true,
            lock_ids: [assignedLock.id],
            last_seen_at: now,
            created_at: now,
            updated_at: now,
          },
          {
            id: "controller_jkt_014",
            resource_type: "Controller",
            tenant_id: tenantID,
            place_id: unassignedPlace.id,
            name: "Kuningan Controller",
            device_id: "MP-GW-JKT-014",
            token: "MP-GW-JKT-014",
            status: "online",
            configured: true,
            lock_ids: [unassignedLock.id],
            last_seen_at: now,
            created_at: now,
            updated_at: now,
          },
        ],
      })
      return
    }

    if (path === "/api/v1/event_sets" && method === "POST") {
      await fulfillJson(route, {
        id: "event_set_e2e",
        created_at: now,
        status: "finished",
        events: [
          {
            uuid: "evt_sudirman_001",
            type: "access_granted",
            actor_name: "Sudirman User",
            object_type: "Lock",
            object_id: assignedLock.id,
            object_name: assignedLock.name,
            place_id: assignedPlace.id,
            lock_id: assignedLock.id,
            success: true,
            result: "granted",
            detail: "mobile unlock",
            created_at: now,
          },
          {
            uuid: "evt_kuningan_001",
            type: "access_granted",
            actor_name: "Kuningan User",
            object_type: "Lock",
            object_id: unassignedLock.id,
            object_name: unassignedLock.name,
            place_id: unassignedPlace.id,
            lock_id: unassignedLock.id,
            success: true,
            result: "granted",
            detail: "mobile unlock",
            created_at: now,
          },
        ],
      })
      return
    }

    if (path === "/api/v1/users" && method === "GET") {
      await fulfillJson(route, {
        items: [
          {
            id: "usr_sudirman_001",
            tenant_id: tenantID,
            building_id: assignedPlace.id,
            name: "Sudirman User",
            email: "user.sudirman@example.test",
            role: "resident",
            status: "active",
            created_at: now,
          },
          {
            id: "usr_kuningan_001",
            tenant_id: tenantID,
            building_id: unassignedPlace.id,
            name: "Kuningan User",
            email: "user.kuningan@example.test",
            role: "resident",
            status: "active",
            created_at: now,
          },
        ],
      })
      return
    }

    if (
      method === "GET" &&
      [
        "/api/v1/readers",
        "/api/v1/terminals",
        "/api/v1/gateways",
        "/api/v1/cards",
        "/api/v1/card_assignments",
        "/api/v1/wallet/passes",
        "/api/v1/groups",
        "/api/v1/door-groups",
        "/api/v1/group_locks",
        "/api/v1/group_zones",
        "/api/v1/teams",
        "/api/v1/team_memberships",
        "/api/v1/roles",
        "/api/v1/role_assignments",
        "/api/v1/shares",
        "/api/v1/access-policies",
        "/api/v1/temporary-access",
      ].includes(path)
    ) {
      await fulfillJson(route, { items: [] })
      return
    }

    if (path === "/api/v1/events/access" && method === "GET") {
      await fulfillJson(route, { items: [] })
      return
    }

    if (method !== "GET" && path !== "/api/v1/event_sets") {
      mutationRequests.push(`${method} ${path}`)
    }

    await fulfillJson(route, method === "GET" ? { items: [] } : {})
  })

  return mutationRequests
}

test("place admin direct URLs cannot switch context to an unassigned place", async ({ page }) => {
  await seedAuthenticatedSession(page)
  const mutationRequests = await setupApiMocks(page)

  await page.goto(`/places/${assignedPlace.id}/doors`)
  await expect(page.getByRole("heading", { name: assignedLock.name })).toBeVisible()
  await expect(page.getByRole("button", { name: "Add Door" })).toBeEnabled()
  await expect(page.getByText(unassignedLock.name)).toHaveCount(0)

  await page.goto(`/places/${unassignedPlace.id}/doors`)
  await expect(page.getByText("No doors found for this place.")).toBeVisible()
  await expect(page.getByText(assignedLock.name)).toHaveCount(0)
  await expect(page.getByText(unassignedLock.name)).toHaveCount(0)
  await expect(page.getByRole("button", { name: "Add Door" })).toBeDisabled()
  await expect(page.getByRole("button", { name: "Unlock", exact: true })).toBeDisabled()
  await expect(page.getByRole("button", { name: "Delete Door" })).toBeDisabled()

  await page.goto(`/places/${unassignedPlace.id}/settings`)
  await expect(page.getByRole("heading", { name: "Place Settings" })).toBeVisible()
  await expect(page.getByRole("button", { name: "Save" })).toBeDisabled()
  await page.getByRole("button", { name: "Advanced" }).click()
  await expect(page.getByRole("button", { name: "Lockdown" })).toBeDisabled()

  expect(mutationRequests).toEqual([])
})
