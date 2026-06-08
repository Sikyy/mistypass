// Pure helpers for the firmware UI (unit-tested per web-admin convention).

export function isCryptoSubtleAvailable(): boolean {
  return typeof crypto !== "undefined" && typeof crypto.subtle !== "undefined" && typeof crypto.subtle.digest === "function"
}

export async function computeSha256Hex(file: File): Promise<string> {
  const buf = await file.arrayBuffer()
  const digest = await crypto.subtle.digest("SHA-256", buf)
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("")
}

export function isSignatureHex(value: string): boolean {
  return /^[0-9a-fA-F]{128}$/.test(value.trim())
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "—"
  const units = ["B", "KB", "MB", "GB"]
  let value = bytes
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i += 1
  }
  return `${i === 0 ? Math.round(value) : value.toFixed(1)} ${units[i]}`
}

export function truncateHex(hex: string, head = 8, tail = 6): string {
  const h = hex.trim()
  if (h.length <= head + tail + 1) return h
  return `${h.slice(0, head)}…${h.slice(-tail)}`
}
