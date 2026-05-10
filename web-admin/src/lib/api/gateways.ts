import { request, requestItems, requestText, encodePathSegment } from "./core"
import type { Controller, Reader, Terminal, GatewayCommandAck } from "./locks"
import { listControllers, listReaders, listTerminals, bindControllerLock, unbindControllerLock, publishControllerConfig, rebootController } from "./locks"

export type GatewayDevice = {
  id: string
  gateway_id: string
  serial_number: string
  kind: string
  source: "mistypass_procured" | "legacy_integration"
  protocol?: "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble"
  rs485_config?: {
    baud_rate: number
    parity: "none" | "even" | "odd"
    stop_bits: number
    device_address: number
    timeout_ms: number
  }
  rs485_health?: {
    retry_count: number
    timeout_count: number
    collision_count: number
    consecutive_timeouts: number
    last_error?: string
    last_report_at: string
  }
  status: "online" | "offline"
  last_seen_at: string
}

export type Gateway = {
  id: string
  tenant_id: string
  serial_number: string
  building_id: string
  device_capacity: number
  devices?: GatewayDevice[]
  status: GatewayStatus
  last_seen_at: string
  bound_door_ids?: string[]
}

export type GatewayStatus = "online" | "offline" | "disabled" | "revoked" | string

export type GatewayCertificateRevocation = {
  id: string
  tenant_id?: string
  gateway_id?: string
  serial_number: string
  reason?: string
  revoked_by?: string
  revoked_at?: string
  source: "runtime" | "environment" | string
}

export type GatewaySerialInventoryProductType =
  | "gateway"
  | "reader"
  | "controller"
  | "relay"
  | "sensor"

export type GatewaySerialInventoryStatus = "available" | "consumed" | "frozen" | "scrapped"

export type GatewaySerialInventoryItem = {
  id: string
  tenant_id: string
  serial_number: string
  product_type: GatewaySerialInventoryProductType
  status: GatewaySerialInventoryStatus
  batch_code?: string
  source?: string
  consumed_gateway_id?: string
  consumed_at?: string
  created_at: string
  updated_at: string
}

export type GatewayCheckpointTrend = {
  report_total: number
  acked_delta: number
  direction: "up" | "down" | "flat"
  first_report_at?: string
  last_report_at?: string
}

export type GatewayCheckpointSummaryItem = {
  gateway_id: string
  tenant_id: string
  building_id: string
  queue: string
  checkpoint_id: string
  last_request_id: string
  acked_count: number
  event_total: number
  access_event_total: number
  device_event_total: number
  lag_count: number
  last_occurred_at?: string
  updated_at: string
  time_window_trend: GatewayCheckpointTrend
}

export type GatewayCheckpointSummaryResponse = {
  items: GatewayCheckpointSummaryItem[]
  totals: {
    queues: number
    event_total: number
    acked_total: number
    lag_total: number
  }
  time_window_trend: {
    window_minutes: number
    since: string
    until: string
    report_total: number
    gateway_total: number
    queue_total: number
    acked_delta_total: number
    direction: "up" | "down" | "flat"
    last_report_at?: string
  }
}

// --- Gateway device helpers ---

const gatewayDeviceProtocols = new Set<NonNullable<GatewayDevice["protocol"]>>([
  "wiegand_26",
  "wiegand_34",
  "osdp_v2",
  "rs485",
  "ble",
])

function normalizeReferenceGatewayDeviceProtocol(protocol: string | undefined): GatewayDevice["protocol"] | undefined {
  const normalized = protocol?.trim().toLowerCase().replace(/-/g, "_")
  if (!normalized) {
    return undefined
  }
  return gatewayDeviceProtocols.has(normalized as NonNullable<GatewayDevice["protocol"]>)
    ? (normalized as GatewayDevice["protocol"])
    : undefined
}

function referenceGatewayDeviceStatus(status: string | undefined): GatewayDevice["status"] {
  return status === "offline" ? "offline" : "online"
}

function referenceControllerCapacity(controller: Controller, deviceCount: number) {
  const parsedCapacity = controller.description?.match(/\b(\d+)\s*[- ]?device\b/i)?.[1]
  const capacity = parsedCapacity ? Number.parseInt(parsedCapacity, 10) : 0
  return Math.max(Number.isFinite(capacity) ? capacity : 0, deviceCount, controller.lock_ids?.length ?? 0, 4)
}

function buildReferenceGatewayDevices(
  controllerID: string,
  readers: Reader[],
  terminals: Terminal[],
  readersByID: Map<string, Reader>
): GatewayDevice[] {
  const readerDevices = readers
    .filter((reader) => reader.controller_id === controllerID)
    .map((reader) => ({
      id: reader.id,
      gateway_id: controllerID,
      serial_number: reader.device_id || reader.token || reader.name || reader.id,
      kind: "reader",
      source: "mistypass_procured",
      protocol: normalizeReferenceGatewayDeviceProtocol(reader.protocol),
      status: referenceGatewayDeviceStatus(reader.status),
      last_seen_at: reader.last_seen_at,
    }) satisfies GatewayDevice)

  const terminalDevices = terminals
    .filter((terminal) => {
      if (terminal.controller_id === controllerID) {
        return true
      }
      return terminal.reader_id ? readersByID.get(terminal.reader_id)?.controller_id === controllerID : false
    })
    .map((terminal) => ({
      id: terminal.id,
      gateway_id: controllerID,
      serial_number: terminal.name || terminal.id,
      kind: "terminal",
      source: "mistypass_procured",
      status: referenceGatewayDeviceStatus(terminal.status),
      last_seen_at: terminal.last_seen_at ?? terminal.updated_at,
    }) satisfies GatewayDevice)

  return [...readerDevices, ...terminalDevices]
}

export function mapReferenceHardwareToGateways(controllers: Controller[], readers: Reader[], terminals: Terminal[]): Gateway[] {
  const readersByID = new Map(readers.map((reader) => [reader.id, reader]))
  return controllers.map((controller) => {
    const devices = buildReferenceGatewayDevices(controller.id, readers, terminals, readersByID)
    return {
      id: controller.id,
      tenant_id: controller.tenant_id,
      serial_number: controller.device_id || controller.token || controller.name || controller.id,
      building_id: controller.place_id,
      device_capacity: referenceControllerCapacity(controller, devices.length),
      devices,
      status: controller.status,
      last_seen_at: controller.last_seen_at,
      bound_door_ids: controller.lock_ids ?? [],
    } satisfies Gateway
  })
}

export async function listGateways(token: string | undefined): Promise<Gateway[]> {
  try {
    const [controllers, readers, terminals] = await Promise.all([
      listControllers(token, { sort: "name" }),
      listReaders(token, { sort: "name" }),
      listTerminals(token, { sort: "name" }),
    ])
    return mapReferenceHardwareToGateways(controllers, readers, terminals)
  } catch {
    return requestItems<Gateway>("/api/v1/gateways", token)
  }
}

// --- Serial Inventory ---

function buildGatewaySerialInventoryPath(
  tenantID?: string,
  options?: {
    product_type?: GatewaySerialInventoryProductType
    status?: GatewaySerialInventoryStatus
  }
): string {
  const query = new URLSearchParams()
  if (tenantID?.trim()) {
    query.set("tenant_id", tenantID.trim())
  }
  if (options?.product_type?.trim()) {
    query.set("product_type", options.product_type.trim())
  }
  if (options?.status?.trim()) {
    query.set("status", options.status.trim())
  }
  const suffix = query.toString()
  return suffix ? `/api/v1/gateways/serial-inventory?${suffix}` : "/api/v1/gateways/serial-inventory"
}

export async function listGatewaySerialInventory(
  token: string | undefined,
  tenantID?: string,
  options?: {
    product_type?: GatewaySerialInventoryProductType
    status?: GatewaySerialInventoryStatus
  }
): Promise<GatewaySerialInventoryItem[]> {
  return requestItems<GatewaySerialInventoryItem>(buildGatewaySerialInventoryPath(tenantID, options), token)
}

export async function importGatewaySerialInventory(
  token: string | undefined,
  payload: {
    tenant_id: string
    items: Array<{
      serial_number: string
      product_type: GatewaySerialInventoryProductType
      batch_code?: string
      source?: string
    }>
  }
): Promise<GatewaySerialInventoryItem[]> {
  const response = await request<{ items: GatewaySerialInventoryItem[] }>(
    "/api/v1/gateways/serial-inventory/import",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.items
}

export async function importGatewaySerialInventoryCSV(
  token: string | undefined,
  payload: {
    tenant_id: string
    csv_content: string
  }
): Promise<GatewaySerialInventoryItem[]> {
  const response = await request<{ items: GatewaySerialInventoryItem[] }>(
    "/api/v1/gateways/serial-inventory/import-csv",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.items
}

export async function batchUpdateGatewaySerialInventoryStatus(
  token: string | undefined,
  payload: {
    tenant_id: string
    status: GatewaySerialInventoryStatus
    serial_numbers: string[]
    consumed_gateway_id?: string
  }
): Promise<GatewaySerialInventoryItem[]> {
  const response = await request<{ items: GatewaySerialInventoryItem[] }>(
    "/api/v1/gateways/serial-inventory/batch-status",
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
  return response.items
}

export async function updateGatewaySerialInventoryStatus(
  token: string | undefined,
  serialNumber: string,
  payload: {
    tenant_id: string
    status: GatewaySerialInventoryStatus
    consumed_gateway_id?: string
  }
): Promise<GatewaySerialInventoryItem> {
  return request<GatewaySerialInventoryItem>(
    `/api/v1/gateways/serial-inventory/${encodeURIComponent(serialNumber)}/status`,
    {
      method: "PATCH",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function exportGatewaySerialInventoryCSV(
  token: string | undefined,
  tenantID?: string,
  options?: {
    product_type?: GatewaySerialInventoryProductType
    status?: GatewaySerialInventoryStatus
  }
): Promise<string> {
  const base = buildGatewaySerialInventoryPath(tenantID, options)
  const path = base.replace("/api/v1/gateways/serial-inventory", "/api/v1/gateways/serial-inventory/export-csv")
  return requestText(path, token)
}

// --- Gateway Registration, Binding, Commands ---

export async function registerGateway(
  token: string | undefined,
  payload: {
    serial_number: string
    tenant_id: string
    building_id?: string
    device_capacity?: number
  }
): Promise<Gateway> {
  return request<Gateway>(
    "/api/v1/gateways/register",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function updateGatewayStatus(
  token: string | undefined,
  gatewayID: string,
  payload: {
    tenant_id?: string
    status: "online" | "offline" | "disabled" | "revoked"
  }
): Promise<Gateway> {
  const query = new URLSearchParams()
  if (payload.tenant_id?.trim()) {
    query.set("tenant_id", payload.tenant_id.trim())
  }
  const suffix = query.toString()
  return request<Gateway>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/status${suffix ? `?${suffix}` : ""}`,
    {
      method: "PATCH",
      body: JSON.stringify({ status: payload.status }),
    },
    token
  )
}

export async function listGatewayCertificateRevocations(
  token: string | undefined,
  tenantID?: string
): Promise<GatewayCertificateRevocation[]> {
  const query = new URLSearchParams()
  if (tenantID?.trim()) {
    query.set("tenant_id", tenantID.trim())
  }
  const suffix = query.toString()
  return requestItems<GatewayCertificateRevocation>(
    suffix ? `/api/v1/gateways/cert-revocations?${suffix}` : "/api/v1/gateways/cert-revocations",
    token
  )
}

export async function revokeGatewayCertificateSerial(
  token: string | undefined,
  payload: {
    tenant_id?: string
    gateway_id?: string
    serial_number: string
    reason?: string
  }
): Promise<GatewayCertificateRevocation> {
  return request<GatewayCertificateRevocation>(
    "/api/v1/gateways/cert-revocations",
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function restoreGatewayCertificateSerial(
  token: string | undefined,
  serialNumber: string,
  tenantID?: string
): Promise<GatewayCertificateRevocation> {
  return request<GatewayCertificateRevocation>(
    `/api/v1/gateways/cert-revocations/${encodePathSegment(serialNumber)}`,
    {
      method: "DELETE",
      body: JSON.stringify({ tenant_id: tenantID?.trim() || undefined }),
    },
    token
  )
}

async function gatewayFromReferenceControllerMutation(
  token: string | undefined,
  controller: Controller,
  previousGateway?: Gateway
): Promise<Gateway> {
  try {
    const gateways = await listGateways(token)
    const refreshed = gateways.find((gateway) => gateway.id === controller.id)
    if (refreshed) {
      return refreshed
    }
  } catch {
    // Keep the mutation result usable when the follow-up read is unavailable.
  }

  const mapped = mapReferenceHardwareToGateways([controller], [], [])[0]
  return previousGateway
    ? {
        ...mapped,
        devices: previousGateway.devices,
      }
    : mapped
}

async function findGatewaySnapshot(token: string | undefined, gatewayID: string): Promise<Gateway | undefined> {
  try {
    const gateways = await listGateways(token)
    return gateways.find((gateway) => gateway.id === gatewayID)
  } catch {
    return undefined
  }
}

export async function bindGatewayDoor(
  token: string | undefined,
  gatewayID: string,
  doorID: string
): Promise<Gateway> {
  try {
    const previousGateway = await findGatewaySnapshot(token, gatewayID)
    const updatedController = await bindControllerLock(token, gatewayID, doorID)
    return gatewayFromReferenceControllerMutation(token, updatedController, previousGateway)
  } catch {
    return request<Gateway>(
      `/api/v1/gateways/${encodePathSegment(gatewayID)}/bind-door`,
      {
        method: "POST",
        body: JSON.stringify({ door_id: doorID }),
      },
      token
    )
  }
}

export async function unbindGatewayDoor(
  token: string | undefined,
  gatewayID: string,
  doorID: string
): Promise<Gateway> {
  const previousGateway = await findGatewaySnapshot(token, gatewayID)
  try {
    await unbindControllerLock(token, gatewayID, doorID)
  } catch {
    return request<Gateway>(
      `/api/v1/gateways/${encodePathSegment(gatewayID)}/unbind-door`,
      {
        method: "POST",
        body: JSON.stringify({ door_id: doorID }),
      },
      token
    )
  }

  const refreshedGateway = await findGatewaySnapshot(token, gatewayID)
  if (refreshedGateway) {
    return refreshedGateway
  }
  if (previousGateway) {
    return {
      ...previousGateway,
      bound_door_ids: (previousGateway.bound_door_ids ?? []).filter((boundDoorID) => boundDoorID !== doorID),
    }
  }
  return {
    id: gatewayID,
    tenant_id: "",
    serial_number: gatewayID,
    building_id: "",
    device_capacity: 4,
    status: "online",
    last_seen_at: "",
    bound_door_ids: [],
  }
}

export async function registerGatewayDevice(
  token: string | undefined,
  gatewayID: string,
  payload: {
    serial_number: string
    kind?: "reader" | "door_controller" | "relay" | "sensor" | "legacy_reader" | "legacy_controller"
    source?: "mistypass_procured" | "legacy_integration"
    protocol?: "wiegand_26" | "wiegand_34" | "osdp_v2" | "rs485" | "ble"
    rs485_config?: {
      baud_rate?: number
      parity?: "none" | "even" | "odd"
      stop_bits?: number
      device_address?: number
      timeout_ms?: number
    }
    status?: "online" | "offline"
  }
): Promise<Gateway> {
  return request<Gateway>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/devices`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function reportGatewayDeviceRS485Telemetry(
  token: string | undefined,
  gatewayID: string,
  deviceID: string,
  payload: {
    retries?: number
    timeouts?: number
    collisions?: number
    last_error?: string
    reset_consecutive_timeouts?: boolean
  }
): Promise<{
  gateway_id: string
  device_id: string
  device: GatewayDevice
  alerted: boolean
}> {
  return request<{
    gateway_id: string
    device_id: string
    device: GatewayDevice
    alerted: boolean
  }>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/devices/${encodePathSegment(deviceID)}/rs485/telemetry`,
    {
      method: "POST",
      body: JSON.stringify(payload),
    },
    token
  )
}

export async function probeGatewayLegacyDevices(
  token: string | undefined,
  gatewayID: string
): Promise<string[]> {
  const payload = await request<{ items: string[] }>(
    `/api/v1/gateways/${encodePathSegment(gatewayID)}/devices/probe-legacy`,
    { method: "POST" },
    token
  )
  return payload.items
}

export async function publishGatewayConfig(
  token: string | undefined,
  gatewayID: string,
  version: string
): Promise<GatewayCommandAck> {
  try {
    return await publishControllerConfig(token, gatewayID, version)
  } catch {
    return request<GatewayCommandAck>(
      `/api/v1/gateways/${encodePathSegment(gatewayID)}/config/publish`,
      {
        method: "POST",
        body: JSON.stringify({ version }),
      },
      token
    )
  }
}

export async function rebootGateway(token: string | undefined, gatewayID: string): Promise<GatewayCommandAck> {
  try {
    return await rebootController(token, gatewayID)
  } catch {
    return request<GatewayCommandAck>(
      `/api/v1/gateways/${encodePathSegment(gatewayID)}/reboot`,
      {
        method: "POST",
      },
      token
    )
  }
}

export async function listGatewayEventCheckpointSummary(
  token: string | undefined,
  options?: {
    tenant_id?: string
    gateway_id?: string
    queue?: string
    limit?: number
    trend_window_minutes?: number
  }
): Promise<GatewayCheckpointSummaryResponse> {
  const query = new URLSearchParams()
  if (options?.tenant_id?.trim()) {
    query.set("tenant_id", options.tenant_id.trim())
  }
  if (options?.gateway_id?.trim()) {
    query.set("gateway_id", options.gateway_id.trim())
  }
  if (options?.queue?.trim()) {
    query.set("queue", options.queue.trim())
  }
  if (typeof options?.limit === "number" && Number.isFinite(options.limit) && options.limit >= 0) {
    query.set("limit", String(Math.floor(options.limit)))
  }
  if (
    typeof options?.trend_window_minutes === "number" &&
    Number.isFinite(options.trend_window_minutes) &&
    options.trend_window_minutes > 0
  ) {
    query.set("trend_window_minutes", String(Math.floor(options.trend_window_minutes)))
  }
  const suffix = query.toString()
  const path = suffix
    ? `/api/v1/gateways/events/checkpoint/summary?${suffix}`
    : "/api/v1/gateways/events/checkpoint/summary"
  return request<GatewayCheckpointSummaryResponse>(path, { method: "GET" }, token)
}
