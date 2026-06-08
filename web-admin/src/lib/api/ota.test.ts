import { afterEach, describe, expect, it, vi } from "vitest"
import { getFirmwareSummary, listFirmware, uploadFirmware } from "./ota"
import { createRollout, getRolloutDetail, listRollouts, rolloutAction } from "./ota"

function mockFetchOnce(body: unknown, ok = true, status = 200) {
  const fn = vi.fn().mockResolvedValue({
    ok, status, statusText: ok ? "OK" : "Error", json: async () => body,
  } as Response)
  vi.stubGlobal("fetch", fn)
  return fn
}

afterEach(() => { vi.unstubAllGlobals(); vi.restoreAllMocks() })

describe("ota api", () => {
  it("getFirmwareSummary calls the summary endpoint with tenant_id", async () => {
    const fetchFn = mockFetchOnce({ total: 3, reported: 2, versions: [{ version: "1.4.0", count: 2 }] })
    const res = await getFirmwareSummary("tok", "tenant_demo_jakarta")
    expect(res.total).toBe(3)
    const url = String(fetchFn.mock.calls[0][0])
    expect(url).toContain("/api/v1/gateways/firmware-summary")
    expect(url).toContain("tenant_id=tenant_demo_jakarta")
  })
  it("listFirmware passes channel filter and unwraps items", async () => {
    const fetchFn = mockFetchOnce({ items: [{ id: "fw_1", version: "1.4.0", channel: "stable" }] })
    const res = await listFirmware("tok", "tenant_demo_jakarta", "stable")
    expect(res).toHaveLength(1)
    const url = String(fetchFn.mock.calls[0][0])
    expect(url).toContain("/api/v1/gateways/firmware?")
    expect(url).toContain("channel=stable")
  })
  it("uploadFirmware posts multipart FormData with the right fields and no JSON content-type", async () => {
    const fetchFn = mockFetchOnce({ id: "fw_1", version: "1.4.0" })
    const file = new File([new Uint8Array([1, 2, 3])], "gateway-agent")
    await uploadFirmware("tok", "tenant_demo_jakarta", {
      version: "1.4.0", channel: "stable", sha256: "a".repeat(64), signature: "b".repeat(128), file,
    })
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit]
    expect(String(url)).toContain("/api/v1/gateways/firmware?")
    expect(init.method).toBe("POST")
    expect(init.body).toBeInstanceOf(FormData)
    const fd = init.body as FormData
    expect(fd.get("version")).toBe("1.4.0")
    expect(fd.get("sha256")).toBe("a".repeat(64))
    expect(fd.get("signature")).toBe("b".repeat(128))
    expect(fd.get("file")).toBeInstanceOf(File)
    const headers = new Headers(init.headers)
    expect(headers.get("Content-Type")).toBeNull()
  })
  it("getFirmwareSummary defaults to zeros on an empty response", async () => {
    mockFetchOnce({})
    const res = await getFirmwareSummary("tok")
    expect(res.total).toBe(0)
    expect(res.reported).toBe(0)
    expect(res.versions).toEqual([])
  })
})

describe("rollout api", () => {
  it("createRollout POSTs the body with tenant_id query", async () => {
    const fetchFn = mockFetchOnce({ id: "rollout_1", state: "active" })
    await createRollout("tok", "tenant_demo_jakarta", {
      firmware_id: "fw_1", target: { kind: "all" }, phases: [{ percentage: 100, requires_approval: false }],
    })
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit]
    expect(String(url)).toContain("/api/v1/gateways/rollouts?")
    expect(String(url)).toContain("tenant_id=tenant_demo_jakarta")
    expect(init.method).toBe("POST")
    expect(JSON.parse(String(init.body)).firmware_id).toBe("fw_1")
  })
  it("listRollouts unwraps items", async () => {
    mockFetchOnce({ items: [{ id: "rollout_1" }] })
    expect(await listRollouts("tok", "t")).toHaveLength(1)
  })
  it("getRolloutDetail returns {rollout, gateways}", async () => {
    mockFetchOnce({ rollout: { id: "rollout_1", state: "active" }, gateways: [{ gateway_id: "gw1", phase: 0, ota_status: "queued" }] })
    const res = await getRolloutDetail("tok", "t", "rollout_1")
    expect(res.rollout.id).toBe("rollout_1")
    expect(res.gateways).toHaveLength(1)
  })
  it("rolloutAction POSTs to the action path", async () => {
    const fetchFn = mockFetchOnce({ id: "rollout_1", state: "paused" })
    await rolloutAction("tok", "t", "rollout_1", "pause")
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit]
    expect(String(url)).toContain("/api/v1/gateways/rollouts/rollout_1/pause")
    expect(init.method).toBe("POST")
  })
})
