import { afterEach, describe, expect, it, vi } from "vitest"
import { computeSha256Hex, formatBytes, isSignatureHex, truncateHex } from "./firmware-utils"

afterEach(() => vi.restoreAllMocks())

describe("firmware-utils", () => {
  it("formatBytes humanizes sizes", () => {
    expect(formatBytes(0)).toBe("0 B")
    expect(formatBytes(1023)).toBe("1023 B")
    expect(formatBytes(1024)).toBe("1.0 KB")
    expect(formatBytes(1024 * 1024 * 5)).toBe("5.0 MB")
    expect(formatBytes(-1)).toBe("—")
  })

  it("truncateHex shortens long hex with ellipsis", () => {
    expect(truncateHex("abcd")).toBe("abcd")
    expect(truncateHex("a".repeat(64))).toContain("…")
    expect(truncateHex("a".repeat(64)).length).toBeLessThan(64)
  })

  it("isSignatureHex requires exactly 128 hex chars", () => {
    expect(isSignatureHex("b".repeat(128))).toBe(true)
    expect(isSignatureHex("  " + "B".repeat(128) + "  ")).toBe(true)
    expect(isSignatureHex("b".repeat(127))).toBe(false)
    expect(isSignatureHex("z".repeat(128))).toBe(false)
  })

  it("computeSha256Hex hex-encodes the platform digest", async () => {
    const digest = new Uint8Array(32).fill(0xab).buffer
    const subtle = { digest: vi.fn().mockResolvedValue(digest) }
    vi.stubGlobal("crypto", { subtle })
    const file = new File([new Uint8Array([1, 2, 3])], "fw")
    const hex = await computeSha256Hex(file)
    expect(hex).toBe("ab".repeat(32))
    expect(subtle.digest).toHaveBeenCalledWith("SHA-256", expect.anything())
    vi.unstubAllGlobals()
  })
})
