import { afterEach, describe, expect, it, vi } from "vitest"
import { getFirmwareSummary, listFirmware, uploadFirmware } from "./ota"

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
})
