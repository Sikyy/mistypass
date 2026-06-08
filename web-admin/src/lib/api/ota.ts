import { request, requestFormData, requestItems } from "./core"

export type FirmwareVersionCount = { version: string; count: number }
export type FirmwareSummary = { total: number; reported: number; versions: FirmwareVersionCount[] }
export type GatewayFirmware = {
  id: string; tenant_id: string; version: string; channel?: string
  sha256: string; signature: string; size_bytes: number; uploaded_by?: string; created_at: string
}
export type UploadFirmwareInput = { version: string; channel?: string; sha256: string; signature: string; file: File }

function firmwareQuery(tenantID?: string, channel?: string): string {
  const params = new URLSearchParams()
  if (tenantID && tenantID.trim() !== "") params.set("tenant_id", tenantID.trim())
  if (channel && channel.trim() !== "") params.set("channel", channel.trim())
  const s = params.toString()
  return s ? `?${s}` : ""
}

export async function getFirmwareSummary(token: string | undefined, tenantID?: string): Promise<FirmwareSummary> {
  const res = await request<Partial<FirmwareSummary>>(`/api/v1/gateways/firmware-summary${firmwareQuery(tenantID)}`, { method: "GET" }, token)
  return { total: res.total ?? 0, reported: res.reported ?? 0, versions: res.versions ?? [] }
}
export async function listFirmware(token: string | undefined, tenantID?: string, channel?: string): Promise<GatewayFirmware[]> {
  return requestItems<GatewayFirmware>(`/api/v1/gateways/firmware${firmwareQuery(tenantID, channel)}`, token)
}
export async function uploadFirmware(token: string | undefined, tenantID: string | undefined, input: UploadFirmwareInput): Promise<GatewayFirmware> {
  const fd = new FormData()
  fd.set("version", input.version)
  if (input.channel && input.channel.trim() !== "") fd.set("channel", input.channel.trim())
  fd.set("sha256", input.sha256)
  fd.set("signature", input.signature)
  fd.set("file", input.file)
  return requestFormData<GatewayFirmware>(`/api/v1/gateways/firmware${firmwareQuery(tenantID)}`, fd, token)
}
