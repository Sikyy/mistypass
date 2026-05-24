import { afterEach, describe, expect, it, vi } from "vitest"

import {
  APIError,
  batchUpdateWalletPhysicalCardInventoryStatus,
  bindGatewayDoor,
  cleanupWalletDLQJobs,
  createBuilding,
  createDoor,
  createTemporaryAccess,
  createUserGroup,
  getWalletGoogleConfig,
  getWalletJobSummary,
  listAccessEvents,
  listBuildings,
  listDoorGroups,
  listDoors,
  listGatewayCertificateRevocations,
  listGateways,
  listTemporaryAccess,
  listUserGroups,
  listWalletPasses,
  normalizeOffsetListResponse,
  parseAPIErrorDetails,
  publishGatewayConfig,
  rebootGateway,
  processWalletJobs,
  requeueWalletDLQJob,
  requeueWalletDLQJobs,
  restoreGatewayCertificateSerial,
  revokeGatewayCertificateSerial,
  upsertWalletGoogleConfig,
  updateWalletPhysicalCardInventoryStatus,
  updateGatewayStatus,
  updateUserGroup,
  validateWalletGoogleConfig,
} from "./api"

describe("api list response normalization", () => {
  it("normalizes reference wrapper pagination metadata", () => {
    expect(
      normalizeOffsetListResponse([
        { id: "role_organization_admin" },
        { id: "role_place_admin" },
      ])
    ).toEqual({
      items: [{ id: "role_organization_admin" }, { id: "role_place_admin" }],
      total: 2,
      offset: 0,
      limit: 2,
      next_offset: undefined,
      has_more: false,
    })

    expect(
      normalizeOffsetListResponse({
        items: [{ id: "role_place_admin" }],
        pagination: {
          offset: 1,
          limit: 1,
          total: 3,
          has_more: true,
        },
      })
    ).toEqual({
      items: [{ id: "role_place_admin" }],
      total: 3,
      offset: 1,
      limit: 1,
      next_offset: 2,
      has_more: true,
    })
  })

  it("keeps supporting legacy top-level offset responses", () => {
    expect(
      normalizeOffsetListResponse({
        items: [{ id: "notification_001" }],
        total: 4,
        offset: 2,
        limit: 1,
        next_offset: 3,
        has_more: true,
      })
    ).toEqual({
      items: [{ id: "notification_001" }],
      total: 4,
      offset: 2,
      limit: 1,
      next_offset: 3,
      has_more: true,
    })
  })
})

describe("api error normalization", () => {
  it("parses structured error payloads without losing the legacy message", async () => {
    const details = await parseAPIErrorDetails(
      new Response(
        JSON.stringify({
          error: "group link token is invalid",
          message: "group link token is invalid",
          code: "not_found",
          status: "404",
        }),
        {
          status: 404,
          statusText: "Not Found",
          headers: {
            "Content-Type": "application/json",
          },
        }
      )
    )

    expect(details).toEqual({
      message: "group link token is invalid",
      code: "not_found",
      responseStatus: "404",
    })
  })

  it("falls back to HTTP status text for non-json errors", async () => {
    await expect(
      parseAPIErrorDetails(
        new Response("upstream unavailable", {
          status: 502,
          statusText: "Bad Gateway",
        })
      )
    ).resolves.toEqual({
      message: "502 Bad Gateway",
    })
  })

  it("carries structured error code on APIError", () => {
    const error = new APIError(422, "apple wallet cards must be enrolled by resident users", {
      code: "unprocessable_entity",
      responseStatus: "422",
    })

    expect(error.message).toBe("apple wallet cards must be enrolled by resident users")
    expect(error.status).toBe(422)
    expect(error.code).toBe("unprocessable_entity")
    expect(error.responseStatus).toBe("422")
  })
})

describe("legacy-named group helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("routes listUserGroups through the reference groups endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ items: [{ id: "group_service", name: "Service" }] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    )
    vi.stubGlobal("fetch", fetchMock)

    await expect(listUserGroups("token")).resolves.toEqual([{ id: "group_service", name: "Service" }])

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/groups"),
      expect.objectContaining({
        method: "GET",
      })
    )
    expect(fetchMock.mock.calls[0][0]).not.toContain("/api/v1/user-groups")
  })

  it("wraps create and update user group payloads for reference groups", async () => {
    const fetchMock = vi.fn().mockImplementation(() =>
      Promise.resolve(
        new Response(JSON.stringify({ id: "group_service", name: "Service" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        })
      )
    )
    vi.stubGlobal("fetch", fetchMock)

    await createUserGroup("token", {
      tenant_id: "tenant_demo",
      name: "Service",
      members: ["usr_001"],
    })
    await updateUserGroup("token", "group_service", {
      name: "Service Updated",
      members: ["usr_002"],
    })

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/groups"))
    expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({
      group: {
        tenant_id: "tenant_demo",
        name: "Service",
        members: ["usr_001"],
      },
    })
    expect(fetchMock.mock.calls[1][0]).toEqual(expect.stringContaining("/api/v1/groups/group_service"))
    expect(JSON.parse(fetchMock.mock.calls[1][1]?.body as string)).toEqual({
      group: {
        name: "Service Updated",
        members: ["usr_002"],
      },
    })
  })
})

describe("door group extension helper", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("routes door group reads through the snake_case extension endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ items: [{ id: "dg_lobby", name: "Lobby Doors" }] }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      })
    )
    vi.stubGlobal("fetch", fetchMock)

    await expect(listDoorGroups("token", "tenant_demo")).resolves.toEqual([{ id: "dg_lobby", name: "Lobby Doors" }])

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/door_groups?tenant_id=tenant_demo"),
      expect.objectContaining({
        method: "GET",
      })
    )
    expect(fetchMock.mock.calls[0][0]).not.toContain("/api/v1/door-groups")
  })
})

describe("legacy-named space helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("routes building and door list helpers through reference endpoints", async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            items: url.includes("/api/v1/places")
              ? [{ id: "place_001", name: "Sudirman Hub" }]
              : [{ id: "lock_001", name: "Lobby Door" }],
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }
        )
      )
    )
    vi.stubGlobal("fetch", fetchMock)

    await expect(listBuildings("token", "tenant_demo")).resolves.toEqual([{ id: "place_001", name: "Sudirman Hub" }])
    await expect(listDoors("token", "tenant_demo")).resolves.toEqual([{ id: "lock_001", name: "Lobby Door" }])

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/places?tenant_id=tenant_demo"))
    expect(fetchMock.mock.calls[1][0]).toEqual(expect.stringContaining("/api/v1/locks?tenant_id=tenant_demo"))
    expect(fetchMock.mock.calls[0][0]).not.toContain("/api/v1/buildings")
    expect(fetchMock.mock.calls[1][0]).not.toContain("/api/v1/doors")
  })

  it("wraps legacy-named create helpers for reference endpoints", async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) =>
      Promise.resolve(
        new Response(JSON.stringify(url.includes("/api/v1/places") ? { id: "place_001" } : { id: "lock_001" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        })
      )
    )
    vi.stubGlobal("fetch", fetchMock)

    await createBuilding("token", {
      tenant_id: "tenant_demo",
      name: "Sudirman Hub",
      address: "Jakarta",
      region: "ID-JK",
    })
    await createDoor("token", {
      tenant_id: "tenant_demo",
      building_id: "place_001",
      floor_id: "floor_001",
      area_id: "area_001",
      name: "Lobby Door",
      gateway_id: "gateway_001",
      kind: "office",
      status: "online",
    })

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/places"))
    expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({
      place: {
        tenant_id: "tenant_demo",
        name: "Sudirman Hub",
        address: "Jakarta",
        region: "ID-JK",
      },
    })
    expect(fetchMock.mock.calls[1][0]).toEqual(expect.stringContaining("/api/v1/locks"))
    expect(JSON.parse(fetchMock.mock.calls[1][1]?.body as string)).toEqual({
      lock: {
        tenant_id: "tenant_demo",
        building_id: "place_001",
        floor_id: "floor_001",
        area_id: "area_001",
        name: "Lobby Door",
        gateway_id: "gateway_001",
        kind: "office",
        status: "online",
      },
    })
  })
})

describe("legacy-named hardware helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("routes gateway reads through reference hardware endpoints", async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      const items = url.includes("/api/v1/controllers")
        ? [
            {
              id: "controller_001",
              resource_type: "Controller",
              tenant_id: "tenant_demo",
              place_id: "place_001",
              name: "Controller A",
              description: "4-device controller",
              device_id: "MP-GW-001",
              token: "MP-GW-001",
              status: "online",
              configured: true,
              lock_ids: ["lock_001"],
              last_seen_at: "2026-04-28T10:00:00Z",
              created_at: "2026-04-28T10:00:00Z",
              updated_at: "2026-04-28T10:00:00Z",
            },
          ]
        : url.includes("/api/v1/readers")
          ? [
              {
                id: "reader_001",
                resource_type: "Reader",
                tenant_id: "tenant_demo",
                place_id: "place_001",
                controller_id: "controller_001",
                name: "Reader A",
                device_id: "RD-001",
                token: "RD-001",
                model: "reader-pro-3.0",
                protocol: "osdp_v2",
                status: "offline",
                configured: true,
                lock_ids: ["lock_001"],
                last_seen_at: "2026-04-28T10:01:00Z",
                created_at: "2026-04-28T10:01:00Z",
                updated_at: "2026-04-28T10:01:00Z",
              },
            ]
          : url.includes("/api/v1/terminals")
            ? [
                {
                  id: "terminal_001",
                  resource_type: "Terminal",
                  tenant_id: "tenant_demo",
                  created_at: "2026-04-28T10:02:00Z",
                  updated_at: "2026-04-28T10:02:00Z",
                  name: "Terminal A",
                  description: "Reader terminal",
                  place_id: "place_001",
                  controller_id: "controller_001",
                  reader_id: "reader_001",
                  status: "online",
                  last_seen_at: "2026-04-28T10:02:00Z",
                },
              ]
            : []

      return Promise.resolve(
        new Response(JSON.stringify({ items }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        })
      )
    })
    vi.stubGlobal("fetch", fetchMock)

    await expect(listGateways("token")).resolves.toEqual([
      {
        id: "controller_001",
        tenant_id: "tenant_demo",
        serial_number: "MP-GW-001",
        building_id: "place_001",
        device_capacity: 4,
        status: "online",
        last_seen_at: "2026-04-28T10:00:00Z",
        bound_door_ids: ["lock_001"],
        devices: [
          {
            id: "reader_001",
            gateway_id: "controller_001",
            serial_number: "RD-001",
            kind: "reader",
            source: "mistypass_procured",
            protocol: "osdp_v2",
            status: "offline",
            last_seen_at: "2026-04-28T10:01:00Z",
          },
          {
            id: "terminal_001",
            gateway_id: "controller_001",
            serial_number: "Terminal A",
            kind: "terminal",
            source: "mistypass_procured",
            status: "online",
            last_seen_at: "2026-04-28T10:02:00Z",
          },
        ],
      },
    ])

    expect(fetchMock.mock.calls.map((call) => call[0])).toEqual([
      expect.stringContaining("/api/v1/controllers?sort=name"),
      expect.stringContaining("/api/v1/readers?sort=name"),
      expect.stringContaining("/api/v1/terminals?sort=name"),
    ])
    expect(fetchMock.mock.calls.map((call) => call[0]).join("\n")).not.toContain("/api/v1/gateways")
  })

  it("falls back to legacy gateways when reference hardware reads fail", async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      if (url.includes("/api/v1/controllers")) {
        return Promise.resolve(
          new Response(JSON.stringify({ error: "controllers unavailable" }), {
            status: 503,
            headers: { "Content-Type": "application/json" },
          })
        )
      }
      if (url.includes("/api/v1/gateways")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [
                {
                  id: "gateway_legacy_001",
                  tenant_id: "tenant_demo",
                  serial_number: "MP-GW-LEGACY-001",
                  building_id: "place_001",
                  device_capacity: 4,
                  status: "online",
                  last_seen_at: "2026-04-28T10:00:00Z",
                },
              ],
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }
          )
        )
      }
      return Promise.resolve(
        new Response(JSON.stringify({ items: [] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        })
      )
    })
    vi.stubGlobal("fetch", fetchMock)

    await expect(listGateways("token")).resolves.toEqual([
      expect.objectContaining({
        id: "gateway_legacy_001",
        serial_number: "MP-GW-LEGACY-001",
      }),
    ])
    expect(fetchMock.mock.calls.map((call) => call[0]).join("\n")).toContain("/api/v1/gateways")
  })

  it("routes legacy gateway bind and command helpers through reference controller actions", async () => {
    let bound = false
    const controllerPayload = () => ({
      id: "controller_001",
      resource_type: "Controller",
      tenant_id: "tenant_demo",
      place_id: "place_001",
      name: "Controller A",
      description: "4-device controller",
      device_id: "MP-GW-001",
      token: "MP-GW-001",
      status: "online",
      configured: true,
      lock_ids: bound ? ["lock_001"] : [],
      last_seen_at: "2026-04-28T10:00:00Z",
      created_at: "2026-04-28T10:00:00Z",
      updated_at: "2026-04-28T10:00:00Z",
    })
    const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/v1/controllers/controller_001/locks") && init?.method === "POST") {
        bound = true
        return Promise.resolve(
          new Response(JSON.stringify(controllerPayload()), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          })
        )
      }
      if (url.includes("/api/v1/controllers/controller_001/config/publish")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              task_id: "task_publish_001",
              gateway_id: "controller_001",
              command: "publish_config",
              status: "queued",
              created_at: "2026-04-28T10:03:00Z",
            }),
            {
              status: 202,
              headers: { "Content-Type": "application/json" },
            }
          )
        )
      }
      if (url.includes("/api/v1/controllers/controller_001/reboot")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              task_id: "task_reboot_001",
              gateway_id: "controller_001",
              command: "reboot",
              status: "queued",
              created_at: "2026-04-28T10:04:00Z",
            }),
            {
              status: 202,
              headers: { "Content-Type": "application/json" },
            }
          )
        )
      }
      if (url.includes("/api/v1/controllers")) {
        return Promise.resolve(
          new Response(JSON.stringify({ items: [controllerPayload()] }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          })
        )
      }
      if (url.includes("/api/v1/readers") || url.includes("/api/v1/terminals")) {
        return Promise.resolve(
          new Response(JSON.stringify({ items: [] }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          })
        )
      }
      return Promise.resolve(
        new Response(JSON.stringify({ items: [] }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        })
      )
    })
    vi.stubGlobal("fetch", fetchMock)

    await expect(bindGatewayDoor("token", "controller_001", "lock_001")).resolves.toEqual(
      expect.objectContaining({
        id: "controller_001",
        bound_door_ids: ["lock_001"],
      })
    )
    await expect(publishGatewayConfig("token", "controller_001", "v1")).resolves.toMatchObject({
      task_id: "task_publish_001",
      gateway_id: "controller_001",
    })
    await expect(rebootGateway("token", "controller_001")).resolves.toMatchObject({
      task_id: "task_reboot_001",
      gateway_id: "controller_001",
    })

    const calledURLs = fetchMock.mock.calls.map((call) => call[0]).join("\n")
    expect(calledURLs).toContain("/api/v1/controllers/controller_001/locks")
    expect(calledURLs).toContain("/api/v1/controllers/controller_001/config/publish")
    expect(calledURLs).toContain("/api/v1/controllers/controller_001/reboot")
    expect(calledURLs).not.toContain("/api/v1/gateways/controller_001/bind-door")
    expect(calledURLs).not.toContain("/api/v1/gateways/controller_001/config/publish")
    expect(calledURLs).not.toContain("/api/v1/gateways/controller_001/reboot")
  })

  it("routes gateway status and certificate revocation helpers through security endpoints", async () => {
    const fetchMock = vi.fn().mockImplementation((url: string, init?: RequestInit) => {
      if (url.includes("/api/v1/gateways/controller_001/status")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: "controller_001",
              tenant_id: "tenant_demo",
              serial_number: "MP-GW-001",
              building_id: "place_001",
              device_capacity: 4,
              status: "revoked",
              last_seen_at: "2026-05-11T10:00:00Z",
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }
          )
        )
      }
      if (url.includes("/api/v1/gateways/cert-revocations/abc123")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: "gw_cert_rev_abc123",
              tenant_id: "tenant_demo",
              serial_number: "abc123",
              source: "runtime",
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }
          )
        )
      }
      if (url.includes("/api/v1/gateways/cert-revocations") && init?.method === "POST") {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: "gw_cert_rev_abc123",
              tenant_id: "tenant_demo",
              gateway_id: "controller_001",
              serial_number: "abc123",
              source: "runtime",
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }
          )
        )
      }
      if (url.includes("/api/v1/gateways/cert-revocations")) {
        return Promise.resolve(
          new Response(
            JSON.stringify({
              items: [
                {
                  id: "gw_cert_rev_abc123",
                  tenant_id: "tenant_demo",
                  gateway_id: "controller_001",
                  serial_number: "abc123",
                  source: "runtime",
                },
              ],
            }),
            {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }
          )
        )
      }
      return Promise.resolve(new Response(JSON.stringify({ items: [] }), { status: 200 }))
    })
    vi.stubGlobal("fetch", fetchMock)

    await expect(updateGatewayStatus("token", "controller_001", { tenant_id: "tenant_demo", status: "revoked" })).resolves.toMatchObject({
      id: "controller_001",
      status: "revoked",
    })
    await expect(listGatewayCertificateRevocations("token", "tenant_demo")).resolves.toEqual([
      expect.objectContaining({ serial_number: "abc123", source: "runtime" }),
    ])
    await expect(
      revokeGatewayCertificateSerial("token", {
        tenant_id: "tenant_demo",
        gateway_id: "controller_001",
        serial_number: "00:AB:C1:23",
        reason: "incident",
      })
    ).resolves.toMatchObject({ serial_number: "abc123" })
    await expect(restoreGatewayCertificateSerial("token", "abc123", "tenant_demo")).resolves.toMatchObject({
      serial_number: "abc123",
    })

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/gateways/controller_001/status?tenant_id=tenant_demo"))
    expect(fetchMock.mock.calls[0][1]?.method).toBe("PATCH")
    expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({ status: "revoked" })
    expect(fetchMock.mock.calls[2][0]).toEqual(expect.stringContaining("/api/v1/gateways/cert-revocations"))
    expect(fetchMock.mock.calls[2][1]?.method).toBe("POST")
    expect(JSON.parse(fetchMock.mock.calls[2][1]?.body as string)).toEqual({
      tenant_id: "tenant_demo",
      gateway_id: "controller_001",
      serial_number: "00:AB:C1:23",
      reason: "incident",
    })
    expect(fetchMock.mock.calls[3][0]).toEqual(expect.stringContaining("/api/v1/gateways/cert-revocations/abc123"))
    expect(fetchMock.mock.calls[3][1]?.method).toBe("DELETE")
  })
})

describe("legacy temporary access helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("routes temporary access reads through reference shares", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [
            {
              id: "tmp_001",
              tenant_id: "tenant_demo",
              email: "guest@example.com",
              role_id: "role_group_access",
              place_id: "building_001",
              area_id: "area_001",
              lock_id: "door_001",
              valid_until: "2026-05-01T10:00:00Z",
              status: "active",
              delivery_method: "email_qr",
              grantee_name: "Guest Visitor",
              grantee_phone: "+628110000001",
              mobile_model: "iPhone 15",
              pass_type: "visitor",
              authorized_at: "2026-04-28T10:00:00Z",
              created_at: "2026-04-28T10:00:00Z",
            },
          ],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }
      )
    )
    vi.stubGlobal("fetch", fetchMock)

    await expect(listTemporaryAccess("token")).resolves.toEqual([
      {
        id: "tmp_001",
        tenant_id: "tenant_demo",
        scope_type: "door",
        building_id: "building_001",
        area_id: "area_001",
        door_id: "door_001",
        delivery_method: "email_qr",
        grantee_name: "Guest Visitor",
        grantee_phone: "+628110000001",
        grantee_email: "guest@example.com",
        mobile_model: "iPhone 15",
        pass_type: "visitor",
        valid_from: undefined,
        valid_until: "2026-05-01T10:00:00Z",
        authorized_by_id: undefined,
        authorized_by_email: undefined,
        authorized_by_role: undefined,
        authorized_at: "2026-04-28T10:00:00Z",
        reviewed_at: undefined,
        reviewed_by: undefined,
        created_at: "2026-04-28T10:00:00Z",
      },
    ])

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/shares"))
    expect(fetchMock.mock.calls[0][0]).not.toContain("/api/v1/temporary-access")
  })

  it("routes temporary access creation through reference shares", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "tmp_001",
          tenant_id: "tenant_demo",
          email: "guest@example.com",
          role_id: "role_group_access",
          place_id: "building_001",
          area_id: "area_001",
          lock_id: "door_001",
          valid_until: "2026-05-01T10:00:00Z",
          status: "active",
          delivery_method: "wallet",
          grantee_name: "Guest Visitor",
          grantee_phone: "+628110000001",
          mobile_model: "Pixel 8",
          pass_type: "visitor",
          created_at: "2026-04-28T10:00:00Z",
        }),
        {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }
      )
    )
    vi.stubGlobal("fetch", fetchMock)

    await expect(
      createTemporaryAccess("token", {
        tenant_id: "tenant_demo",
        scope_type: "door",
        building_id: "building_001",
        area_id: "area_001",
        door_id: "door_001",
        delivery_method: "wallet",
        grantee_name: "Guest Visitor",
        grantee_phone: "+628110000001",
        grantee_email: "guest@example.com",
        mobile_model: "Pixel 8",
        pass_type: "visitor",
        valid_until: "2026-05-01T10:00:00Z",
      })
    ).resolves.toMatchObject({
      id: "tmp_001",
      scope_type: "door",
      grantee_email: "guest@example.com",
      grantee_phone: "+628110000001",
    })

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/shares"))
    expect(fetchMock.mock.calls[0][0]).not.toContain("/api/v1/temporary-access")
    expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({
      share: {
        tenant_id: "tenant_demo",
        email: "guest@example.com",
        place_id: "building_001",
        area_id: "area_001",
        lock_id: "door_001",
        delivery_method: "wallet",
        grantee_name: "Guest Visitor",
        grantee_phone: "+628110000001",
        mobile_model: "Pixel 8",
        pass_type: "visitor",
        valid_until: "2026-05-01T10:00:00Z",
      },
    })
  })
})

describe("legacy access event helper", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("routes access event reads through reference event sets", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          id: "event_set_demo",
          created_at: "2026-04-28T10:00:00Z",
          status: "finished",
          events: [
            {
              uuid: "evt_001",
              tenant_id: "tenant_demo",
              type: "access_granted",
              actor_id: "usr_001",
              actor_name: "Ari",
              object_type: "Lock",
              object_id: "door_001",
              place_id: "building_001",
              area_id: "area_001",
              lock_id: "door_001",
              gateway_id: "gw_001",
              success: true,
              result: "success",
              created_at: "2026-04-28T10:00:00Z",
            },
            {
              uuid: "evt_002",
              type: "access_denied",
              actor_email: "guest@example.com",
              object_type: "Lock",
              object_id: "door_002",
              place_id: "building_001",
              success: false,
              result: "denied",
              created_at: "2026-04-28T09:00:00Z",
            },
          ],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }
      )
    )
    vi.stubGlobal("fetch", fetchMock)

    await expect(listAccessEvents("token", { page: 1, limit: 1 })).resolves.toEqual([
      {
        id: "evt_001",
        tenant_id: "tenant_demo",
        building_id: "building_001",
        area_id: "area_001",
        type: "access_granted",
        actor: "Ari",
        door_id: "door_001",
        gateway_id: "gw_001",
        result: "success",
        at: "2026-04-28T10:00:00Z",
      },
    ])

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/event_sets"))
    expect(fetchMock.mock.calls[0][0]).not.toContain("/api/v1/events/access")
    expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({
      event_set: {
        interval: undefined,
        place_id: undefined,
        event_place_id: undefined,
        event_type: undefined,
        event_uuid: undefined,
        event_success: undefined,
        event_object_id: undefined,
        event_object_type: "Lock",
      },
    })
  })
})

describe("legacy wallet pass helper", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("routes wallet pass reads through reference cards", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          items: [
            {
              id: "pass_001",
              resource_type: "Card",
              tenant_id: "tenant_demo",
              status: "activated",
              token: "wallet_object_001",
              provider: "google",
              credential_kind: "google_wallet",
              template_id: "tpl_employee",
              user_id: "usr_001",
              assignee_type: "User",
              assignee_id: "usr_001",
              save_link: "https://wallet.example/save/pass_001",
              issued_at: "2026-04-28T10:00:00Z",
              expires_at: "2026-05-28T10:00:00Z",
              created_at: "2026-04-28T10:00:00Z",
              updated_at: "2026-04-28T10:30:00Z",
            },
            {
              id: "pass_002",
              resource_type: "Card",
              tenant_id: "tenant_demo",
              status: "deactivated",
              token: "wallet_object_002",
              provider: "apple",
              credential_kind: "apple_wallet",
              template_id: "tpl_visitor",
              assignee_type: "Guest",
              assignee_id: "guest_001",
              save_link: "",
              issued_at: "2026-04-28T09:00:00Z",
              created_at: "2026-04-28T09:00:00Z",
              updated_at: "2026-04-28T09:30:00Z",
            },
          ],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }
      )
    )
    vi.stubGlobal("fetch", fetchMock)

    await expect(listWalletPasses("token", "tenant_demo", { page: 1, limit: 1 })).resolves.toEqual([
      {
        id: "pass_001",
        tenant_id: "tenant_demo",
        provider: "google",
        template_id: "tpl_employee",
        target_type: "user",
        target_id: "usr_001",
        object_id: "wallet_object_001",
        status: "active",
        save_link: "https://wallet.example/save/pass_001",
        expires_at: "2026-05-28T10:00:00Z",
        issued_at: "2026-04-28T10:00:00Z",
        created_by: "",
        updated_by: "",
        created_at: "2026-04-28T10:00:00Z",
        updated_at: "2026-04-28T10:30:00Z",
      },
    ])

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/cards?tenant_id=tenant_demo"))
    expect(fetchMock.mock.calls[0][0]).not.toContain("/api/v1/wallet/passes")
  })
})

describe("wallet google config helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("reads, saves, and validates provider config through wallet google routes", async () => {
    const config = {
      id: "wcfg_001",
      tenant_id: "tenant_demo",
      provider: "google",
      issuer_id: "issuer_001",
      service_account_email: "wallet-issuer@example.iam.gserviceaccount.com",
      key_ref: "kms://wallet/google/issuer",
      status: "active",
      created_at: "2026-05-24T10:00:00Z",
      updated_at: "2026-05-24T10:05:00Z",
    }
    const validation = {
      provider: "google",
      tenant_id: "tenant_demo",
      valid: true,
      items: [{ field: "issuer_id", status: "ok", message: "issuer_id looks good" }],
      checked_at: "2026-05-24T10:10:00Z",
    }
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(config), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(config), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(validation), { status: 200, headers: { "Content-Type": "application/json" } }))
    vi.stubGlobal("fetch", fetchMock)

    await expect(getWalletGoogleConfig("token", "tenant_demo")).resolves.toMatchObject({
      issuer_id: "issuer_001",
      status: "active",
    })
    await expect(
      upsertWalletGoogleConfig("token", {
        tenant_id: "tenant_demo",
        issuer_id: "issuer_001",
        service_account_email: "wallet-issuer@example.iam.gserviceaccount.com",
        key_ref: "kms://wallet/google/issuer",
        status: "active",
        actor: "web_admin.wallet.google_config",
      })
    ).resolves.toMatchObject({ id: "wcfg_001" })
    await expect(
      validateWalletGoogleConfig("token", {
        tenant_id: "tenant_demo",
        issuer_id: "issuer_001",
        service_account_email: "wallet-issuer@example.iam.gserviceaccount.com",
        key_ref: "kms://wallet/google/issuer",
      })
    ).resolves.toMatchObject({ valid: true })

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/wallet/google/config?tenant_id=tenant_demo"))
    expect(fetchMock.mock.calls[1][0]).toEqual(expect.stringContaining("/api/v1/wallet/google/config"))
    expect(fetchMock.mock.calls[1][1]?.method).toBe("PUT")
    expect(JSON.parse(fetchMock.mock.calls[1][1]?.body as string)).toMatchObject({
      tenant_id: "tenant_demo",
      status: "active",
      actor: "web_admin.wallet.google_config",
    })
    expect(fetchMock.mock.calls[2][0]).toEqual(expect.stringContaining("/api/v1/wallet/google/config/validate"))
    expect(fetchMock.mock.calls[2][1]?.method).toBe("POST")
  })
})

describe("wallet dlq governance helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("routes summary, process, requeue, and cleanup calls to wallet queue endpoints", async () => {
    const summary = {
      tenant_id: "tenant_demo",
      max_retry: 3,
      total: 6,
      pending: 2,
      processing: 0,
      success: 1,
      failed: 1,
      dlq: 2,
      retryable_failed: 1,
      non_retryable_failed: 1,
      updated_at: "2026-05-24T10:00:00Z",
    }
    const processResult = {
      tenant_id: "tenant_demo",
      limit: 10,
      worker_count: 2,
      max_retry: 3,
      claimed: 2,
      succeeded: 1,
      failed: 0,
      dlq: 1,
      skipped: 0,
      retried: 1,
      pending_after: 0,
      processed_job_ids: ["wjob_001", "wjob_002"],
      started_at: "2026-05-24T10:01:00Z",
      completed_at: "2026-05-24T10:01:01Z",
    }
    const requeueResult = {
      tenant_id: "tenant_demo",
      limit: 10,
      requeued: 2,
      skipped: 0,
      remaining_dlq: 0,
      processed_jobs: ["wjob_003", "wjob_004"],
      updated_at: "2026-05-24T10:02:00Z",
    }
    const cleanupResult = {
      tenant_id: "tenant_demo",
      limit: 10,
      removed: 1,
      remaining_dlq: 1,
      processed_jobs: ["wjob_005"],
      updated_at: "2026-05-24T10:03:00Z",
    }
    const singleRequeueResult = {
      id: "wjob_006",
      tenant_id: "tenant_demo",
      provider: "google",
      batch_id: "batch_001",
      template_id: "wpt_001",
      target_type: "user",
      target_id: "usr_override",
      status: "pending",
      retry_count: 0,
      created_at: "2026-05-24T10:00:00Z",
      updated_at: "2026-05-24T10:04:00Z",
    }
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response(JSON.stringify(summary), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(processResult), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(requeueResult), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(cleanupResult), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify(singleRequeueResult), { status: 200, headers: { "Content-Type": "application/json" } }))
    vi.stubGlobal("fetch", fetchMock)

    await expect(getWalletJobSummary("token", { tenant_id: "tenant_demo", max_retry: 3 })).resolves.toMatchObject({ dlq: 2 })
    await expect(
      processWalletJobs("token", {
        tenant_id: "tenant_demo",
        limit: 10,
        worker_count: 2,
        max_retry: 3,
        actor: "web_admin.wallet.queue.process",
      })
    ).resolves.toMatchObject({ claimed: 2 })
    await expect(
      requeueWalletDLQJobs("token", {
        tenant_id: "tenant_demo",
        limit: 10,
        error_code: "template_inactive",
        actor: "web_admin.wallet.dlq.requeue",
      })
    ).resolves.toMatchObject({ requeued: 2 })
    await expect(
      cleanupWalletDLQJobs("token", {
        tenant_id: "tenant_demo",
        limit: 10,
        error_code: "template_inactive",
        older_than_seconds: 86400,
        actor: "web_admin.wallet.dlq.cleanup",
      })
    ).resolves.toMatchObject({ removed: 1 })
    await expect(
      requeueWalletDLQJob("token", "wjob_006", {
        tenant_id: "tenant_demo",
        target_id: "usr_override",
        actor: "web_admin.wallet.dlq.requeue_single",
      })
    ).resolves.toMatchObject({ id: "wjob_006", status: "pending" })

    expect(fetchMock.mock.calls[0][0]).toEqual(expect.stringContaining("/api/v1/wallet/jobs/summary?tenant_id=tenant_demo&max_retry=3"))
    expect(fetchMock.mock.calls[1][0]).toEqual(expect.stringContaining("/api/v1/wallet/jobs/process"))
    expect(fetchMock.mock.calls[1][1]?.method).toBe("POST")
    expect(fetchMock.mock.calls[2][0]).toEqual(expect.stringContaining("/api/v1/wallet/jobs/dlq/requeue"))
    expect(fetchMock.mock.calls[2][1]?.method).toBe("POST")
    expect(JSON.parse(fetchMock.mock.calls[2][1]?.body as string)).toMatchObject({
      tenant_id: "tenant_demo",
      error_code: "template_inactive",
    })
    expect(fetchMock.mock.calls[3][0]).toEqual(expect.stringContaining("/api/v1/wallet/jobs/dlq/cleanup"))
    expect(fetchMock.mock.calls[3][1]?.method).toBe("POST")
    expect(JSON.parse(fetchMock.mock.calls[3][1]?.body as string)).toMatchObject({
      older_than_seconds: 86400,
      actor: "web_admin.wallet.dlq.cleanup",
    })
    expect(fetchMock.mock.calls[4][0]).toEqual(expect.stringContaining("/api/v1/wallet/jobs/wjob_006/dlq/requeue"))
    expect(fetchMock.mock.calls[4][1]?.method).toBe("POST")
    expect(JSON.parse(fetchMock.mock.calls[4][1]?.body as string)).toMatchObject({
      tenant_id: "tenant_demo",
      target_id: "usr_override",
      actor: "web_admin.wallet.dlq.requeue_single",
    })
  })
})

describe("physical card inventory governance helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("patches single and batch inventory status routes", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            id: "wpci_001",
            tenant_id: "tenant_demo",
            card_number: "CARD-001",
            status: "frozen",
            created_at: "2026-04-29T10:00:00Z",
            updated_at: "2026-04-29T10:05:00Z",
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }
        )
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            items: [
              {
                id: "wpci_001",
                tenant_id: "tenant_demo",
                card_number: "CARD-001",
                status: "scrapped",
                created_at: "2026-04-29T10:00:00Z",
                updated_at: "2026-04-29T10:10:00Z",
              },
            ],
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }
        )
      )
    vi.stubGlobal("fetch", fetchMock)

    await expect(
      updateWalletPhysicalCardInventoryStatus("token", "wpci_001", {
        tenant_id: "tenant_demo",
        status: "frozen",
        reason: "QA hold",
      })
    ).resolves.toMatchObject({ id: "wpci_001", status: "frozen" })

    await expect(
      batchUpdateWalletPhysicalCardInventoryStatus("token", {
        tenant_id: "tenant_demo",
        inventory_ids: ["wpci_001", "wpci_002"],
        status: "scrapped",
        reason: "bad stock",
      })
    ).resolves.toEqual([expect.objectContaining({ id: "wpci_001", status: "scrapped" })])

    expect(fetchMock.mock.calls[0][0]).toEqual(
      expect.stringContaining("/api/v1/wallet/physical-card-inventory/wpci_001/status")
    )
    expect(fetchMock.mock.calls[0][1]?.method).toBe("PATCH")
    expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({
      tenant_id: "tenant_demo",
      status: "frozen",
      reason: "QA hold",
    })

    expect(fetchMock.mock.calls[1][0]).toEqual(
      expect.stringContaining("/api/v1/wallet/physical-card-inventory/batch-status")
    )
    expect(fetchMock.mock.calls[1][1]?.method).toBe("PATCH")
    expect(JSON.parse(fetchMock.mock.calls[1][1]?.body as string)).toEqual({
      tenant_id: "tenant_demo",
      inventory_ids: ["wpci_001", "wpci_002"],
      status: "scrapped",
      reason: "bad stock",
    })
  })
})
