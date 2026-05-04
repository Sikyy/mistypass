import { request, encodePathSegment } from "./core"

export type SouthboundProvider = "hikvision" | "zkteco"

export type SouthboundDeviceConfig = {
  host: string
  port: number
  username: string
  password: string
}

export type SouthboundUnlockRequest = SouthboundDeviceConfig & {
  door_index: number
}

export type SouthboundSyncUser = {
  user_id: string
  name: string
  card_number?: string
  start_time?: string
  end_time?: string
}

export async function southboundUnlock(
  token: string | undefined,
  provider: SouthboundProvider,
  deviceID: string,
  payload: SouthboundUnlockRequest,
): Promise<{ status: string; device: string }> {
  return request<{ status: string; device: string }>(
    `/api/v1/gateway/southbound/${encodePathSegment(provider)}/${encodePathSegment(deviceID)}/unlock`,
    { method: "POST", body: JSON.stringify(payload) },
    token,
  )
}

export async function southboundSyncUsers(
  token: string | undefined,
  provider: SouthboundProvider,
  deviceID: string,
  payload: SouthboundDeviceConfig & { users: SouthboundSyncUser[] },
): Promise<{ status: string; user_count: number }> {
  return request<{ status: string; user_count: number }>(
    `/api/v1/gateway/southbound/${encodePathSegment(provider)}/${encodePathSegment(deviceID)}/sync-users`,
    { method: "POST", body: JSON.stringify(payload) },
    token,
  )
}

export async function southboundTestConnection(
  token: string | undefined,
  provider: SouthboundProvider,
  payload: SouthboundDeviceConfig,
): Promise<{ reachable: boolean; error?: string; provider: string }> {
  return request<{ reachable: boolean; error?: string; provider: string }>(
    `/api/v1/gateway/southbound/${encodePathSegment(provider)}/test`,
    { method: "POST", body: JSON.stringify(payload) },
    token,
  )
}
