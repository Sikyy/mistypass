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
  // tenant_id goes on the query string (backend reads it via resolveTenantID); the form body carries only the artifact fields.
  return requestFormData<GatewayFirmware>(`/api/v1/gateways/firmware${firmwareQuery(tenantID)}`, fd, token)
}

// --- Rollout types ---

export type RolloutTarget = { kind: "all" | "building" | "gateways"; building_id?: string; gateway_ids?: string[] }
export type RolloutPhase = { percentage: number; requires_approval: boolean }
export type RolloutSchedule = { start_at?: string; window_start?: string; window_end?: string; timezone?: string }
export type GatewayRollout = {
  id: string; tenant_id: string; firmware_id: string; firmware_version: string
  target: RolloutTarget; phases: RolloutPhase[]; failure_threshold_pct: number
  state: string; current_phase: number; schedule?: RolloutSchedule
  created_by?: string; updated_by?: string; created_at: string; updated_at: string
}
export type RolloutGatewayStatus = { gateway_id: string; phase: number; ota_status: string; current_firmware_version?: string }
export type CreateRolloutInput = { firmware_id: string; target: RolloutTarget; phases: RolloutPhase[]; failure_threshold_pct?: number; schedule?: RolloutSchedule }
export type RolloutActionName = "approve" | "pause" | "resume" | "abort"

// --- Rollout API functions ---

export async function createRollout(token: string | undefined, tenantID: string | undefined, input: CreateRolloutInput): Promise<GatewayRollout> {
  return request<GatewayRollout>(`/api/v1/gateways/rollouts${firmwareQuery(tenantID)}`, { method: "POST", body: JSON.stringify(input) }, token)
}
export async function listRollouts(token: string | undefined, tenantID?: string): Promise<GatewayRollout[]> {
  return requestItems<GatewayRollout>(`/api/v1/gateways/rollouts${firmwareQuery(tenantID)}`, token)
}
export async function getRolloutDetail(token: string | undefined, tenantID: string | undefined, id: string): Promise<{ rollout: GatewayRollout; gateways: RolloutGatewayStatus[] }> {
  return request<{ rollout: GatewayRollout; gateways: RolloutGatewayStatus[] }>(`/api/v1/gateways/rollouts/${encodeURIComponent(id)}${firmwareQuery(tenantID)}`, { method: "GET" }, token)
}
export async function rolloutAction(token: string | undefined, tenantID: string | undefined, id: string, action: RolloutActionName): Promise<GatewayRollout> {
  return request<GatewayRollout>(`/api/v1/gateways/rollouts/${encodeURIComponent(id)}/${action}${firmwareQuery(tenantID)}`, { method: "POST" }, token)
}
