import { request, requestItems, withTenantQuery } from "./core"

// --- Types ---

export type Camera = {
  id: string
  tenant_id: string
  place_id: string
  door_id: string
  name: string
  provider: CameraProvider
  host: string
  port: number
  channel: number
  use_tls: boolean
  status: string
  last_snapshot_at: string
  created_at: string
  updated_at: string
}

export type CameraProvider = "onvif" | "hikvision" | "dahua" | "zkteco" | "vivotek"

export type CameraCreateRequest = {
  tenant_id: string
  place_id: string
  door_id?: string
  name: string
  provider: string
  host: string
  port?: number
  channel?: number
  use_tls?: boolean
  username: string
  password: string
}

export type CameraUpdateRequest = {
  name?: string
  door_id?: string
  host?: string
  port?: number
  channel?: number
  use_tls?: boolean
  username?: string
  password?: string
  status?: string
}

export type CameraSnapshot = {
  id: string
  camera_id: string
  event_id: string
  event_type: string
  content_type: string
  size_bytes: number
  signed_url: string
  storage_path: string
  captured_at: string
  retention_until: string
}

export type DiscoveredCamera = {
  name: string
  manufacturer: string
  model: string
  address: string
  mac_address: string
  provider: string
}

export type CameraTestResult = {
  message: string
}

export type CameraVideoLink = {
  url: string
}

// --- API functions ---

export async function listCameras(token: string | undefined, tenantID: string): Promise<Camera[]> {
  return requestItems<Camera>(withTenantQuery("/api/v1/cameras", tenantID), token)
}

export async function getCamera(token: string | undefined, cameraID: string): Promise<Camera> {
  return request<Camera>(`/api/v1/cameras/${cameraID}`, { method: "GET" }, token)
}

export async function createCamera(token: string | undefined, body: CameraCreateRequest): Promise<Camera> {
  return request<Camera>("/api/v1/cameras", { method: "POST", body: JSON.stringify(body) }, token)
}

export async function updateCamera(token: string | undefined, cameraID: string, body: CameraUpdateRequest): Promise<Camera> {
  return request<Camera>(`/api/v1/cameras/${cameraID}`, { method: "PATCH", body: JSON.stringify(body) }, token)
}

export async function deleteCamera(token: string | undefined, cameraID: string): Promise<void> {
  return request<void>(`/api/v1/cameras/${cameraID}`, { method: "DELETE" }, token)
}

export async function testCameraConnection(token: string | undefined, cameraID: string): Promise<CameraTestResult> {
  return request<CameraTestResult>(`/api/v1/cameras/${cameraID}/test`, { method: "POST" }, token)
}

export async function captureSnapshot(token: string | undefined, cameraID: string): Promise<CameraSnapshot> {
  return request<CameraSnapshot>(`/api/v1/cameras/${cameraID}/snapshot`, { method: "POST" }, token)
}

export async function listCameraSnapshots(token: string | undefined, cameraID: string): Promise<CameraSnapshot[]> {
  return requestItems<CameraSnapshot>(`/api/v1/cameras/${cameraID}/snapshots`, token)
}

export async function discoverCameras(token: string | undefined, provider: string, subnet: string): Promise<DiscoveredCamera[]> {
  const result = await request<{ items: DiscoveredCamera[] }>(
    "/api/v1/cameras/discover",
    { method: "POST", body: JSON.stringify({ provider, subnet }) },
    token
  )
  return Array.isArray(result?.items) ? result.items : []
}

export async function getCameraVideoLink(token: string | undefined, cameraID: string): Promise<string> {
  const result = await request<CameraVideoLink>(`/api/v1/cameras/${cameraID}/video_link`, { method: "GET" }, token)
  return result?.url ?? ""
}

export async function listEventSnapshots(token: string | undefined, eventID: string): Promise<CameraSnapshot[]> {
  return requestItems<CameraSnapshot>(`/api/v1/events/${eventID}/snapshots`, token)
}
