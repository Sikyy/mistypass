import {
  createEventSet,
  listAccessEvents,
  listAccessPolicies,
  listAccessUsers,
  listAreas,
  listCardAssignments,
  listCards,
  listControllers,
  listDoorGroups,
  listFloors,
  listGateways,
  listGroupLocks,
  listGroupZones,
  listGroups,
  listLocks,
  listPlaces,
  listRoleAssignments,
  listRoles,
  listReaders,
  listShares,
  listTeamMemberships,
  listTeams,
  listTerminals,
  listTemporaryAccess,
  listWalletPasses,
  type AccessEvent,
  type AccessPolicy,
  type AccessUser,
  type Area,
  type Building,
  type Card,
  type CardAssignment,
  type Controller,
  type CurrentUser,
  type Door,
  type DoorGroup,
  type EventSet,
  type EventSetEvent,
  type Floor,
  type Gateway,
  type GatewayDevice,
  type GroupLock,
  type GroupZone,
  type Role,
  type RoleAssignment,
  type Reader,
  type Share,
  type Team,
  type TeamMembership,
  type Terminal,
  type TemporaryAccess,
  type UserGroup,
  type WalletPassInstance,
} from "@/lib/api"
import { getViewerBuildingIDs, getViewerTenantID, isPlatformViewer } from "@/lib/viewer"

export type KisiResourceTone = "success" | "warning" | "danger" | "info"

export type KisiPlaceResource = {
  id: string
  name: string
  address: string
  region: string
  doorCount: number
  gatewayCount: number
  offlineCount: number
  statusLabel: string
  tone: KisiResourceTone
}

export type KisiFloorResource = {
  id: string
  placeId: string
  name: string
  description: string
  doorCount: number
  hardwareCount: number
  statusLabel: string
  tone: KisiResourceTone
}

export type KisiZoneResource = {
  id: string
  placeId: string
  floorId: string
  floorName: string
  name: string
  description: string
  doorCount: number
}

export type KisiDoorResource = {
  id: string
  placeId: string
  floorId: string
  floorName: string
  areaName: string
  name: string
  kind: string
  status: "online" | "offline"
  gatewayId: string
  gatewaySerial: string
  deviceCount: number
  description: string
}

export type KisiHardwareResource = {
  id: string
  placeId: string
  gatewayId: string
  name: string
  type: string
  statusLabel: string
  tone: KisiResourceTone
  location: string
  lastSeenLabel: string
  doorNames: string[]
}

export type KisiEventResource = {
  id: string
  placeId: string
  doorId: string
  object: string
  action: string
  user: string
  timeLabel: string
  statusLabel: string
  tone: KisiResourceTone
  details: string
  at: string
}

export type KisiUserResource = {
  id: string
  placeId: string
  name: string
  email: string
  role: string
  roleValue?: string
  rawStatus?: string
  statusLabel: string
  tone: KisiResourceTone
  accessDateLabel: string
  sourceLabel: string
  groupIds?: string[]
}

export type KisiCredentialResource = {
  id: string
  user: string
  userID: string
  type: string
  statusLabel: string
  tone: KisiResourceTone
  issuedLabel: string
  expiresLabel: string
}

export type KisiGroupResource = {
  id: string
  tenantID?: string
  placeId: string
  name: string
  kind: string
  memberCount: number
  targetLabel: string
  description: string
  doorIds?: string[]
  zoneIds?: string[]
  zoneFloorIds?: string[]
  loginEnabled?: boolean
  geofenceRestrictionEnabled?: boolean
  geofenceRestrictionRadius?: number
  primaryDeviceRestrictionEnabled?: boolean
  managedDeviceRestrictionEnabled?: boolean
  readerRestrictionEnabled?: boolean
  timeRestrictionEnabled?: boolean
  tapToAccessRestrictionEnabled?: boolean
  timeRestrictionTimeZone?: string
  statusLabel: string
  tone: KisiResourceTone
}

export type KisiTeamResource = {
  id: string
  placeId: string
  name: string
  scopeLabel: string
  sourceLabel: string
  description: string
  memberCount: number
  accessRightCount: number
  statusLabel: string
  tone: KisiResourceTone
}

export type KisiTeamMembershipResource = {
  id: string
  teamId: string
  placeId: string
  memberType: "User" | "Guest"
  name: string
  email: string
  sourceLabel: string
  statusLabel: string
  tone: KisiResourceTone
}

export type KisiAccessRightResource = {
  id: string
  sourceType?: "role_assignment" | "share"
  placeId: string
  name: string
  subjectType: "Group" | "Place" | "Team" | "User" | "Access Link"
  targetLabel: string
  ruleLabel: string
  statusLabel: string
  tone: KisiResourceTone
  reviewedAt?: string
  reviewedBy?: string
  needsReview?: boolean
}

export type KisiResourceSummary = {
  places: KisiPlaceResource[]
  floors: KisiFloorResource[]
  zones: KisiZoneResource[]
  doors: KisiDoorResource[]
  hardware: KisiHardwareResource[]
  events: KisiEventResource[]
  users: KisiUserResource[]
  credentials: KisiCredentialResource[]
  groups: KisiGroupResource[]
  teams: KisiTeamResource[]
  teamMemberships: KisiTeamMembershipResource[]
  accessRights: KisiAccessRightResource[]
  partial: boolean
}

export type KisiPlaceContext = {
  place: KisiPlaceResource | null
  floors: KisiFloorResource[]
  zones: KisiZoneResource[]
  doors: KisiDoorResource[]
  hardware: KisiHardwareResource[]
  events: KisiEventResource[]
  users: KisiUserResource[]
  groups: KisiGroupResource[]
  teams: KisiTeamResource[]
  teamMemberships: KisiTeamMembershipResource[]
  accessRights: KisiAccessRightResource[]
}

const fallbackPlaces: KisiPlaceResource[] = [
  {
    id: "sudirman-hub",
    name: "Sudirman Hub",
    address: "Jl. Jend. Sudirman, Jakarta",
    region: "Jakarta",
    doorCount: 12,
    gatewayCount: 3,
    offlineCount: 0,
    statusLabel: "All online",
    tone: "success",
  },
  {
    id: "kuningan-tower",
    name: "Kuningan Tower",
    address: "Kuningan, Jakarta",
    region: "Jakarta",
    doorCount: 6,
    gatewayCount: 2,
    offlineCount: 1,
    statusLabel: "1 offline",
    tone: "warning",
  },
  {
    id: "serang-plant",
    name: "Serang Plant",
    address: "Serang Industrial Estate",
    region: "Banten",
    doorCount: 6,
    gatewayCount: 2,
    offlineCount: 0,
    statusLabel: "All online",
    tone: "success",
  },
]

const fallbackFloors: KisiFloorResource[] = [
  {
    id: "sudirman-1f",
    placeId: "sudirman-hub",
    name: "1st Floor",
    description: "Lobby, reception, visitor turnstiles",
    doorCount: 4,
    hardwareCount: 3,
    statusLabel: "Online",
    tone: "success",
  },
  {
    id: "sudirman-3f",
    placeId: "sudirman-hub",
    name: "3rd Floor",
    description: "Office access and meeting rooms",
    doorCount: 5,
    hardwareCount: 2,
    statusLabel: "Online",
    tone: "success",
  },
  {
    id: "sudirman-parking",
    placeId: "sudirman-hub",
    name: "Parking",
    description: "Garage and loading bay",
    doorCount: 3,
    hardwareCount: 2,
    statusLabel: "Review",
    tone: "warning",
  },
]

const fallbackZones: KisiZoneResource[] = [
  {
    id: "zone-sudirman-lobby",
    placeId: "sudirman-hub",
    floorId: "sudirman-1f",
    floorName: "1st Floor",
    name: "Lobby",
    description: "Reception and visitor turnstiles",
    doorCount: 4,
  },
  {
    id: "zone-sudirman-garage",
    placeId: "sudirman-hub",
    floorId: "sudirman-parking",
    floorName: "Parking",
    name: "Garage",
    description: "Vehicle and loading bay access",
    doorCount: 3,
  },
]

const fallbackDoors: KisiDoorResource[] = [
  {
    id: "door-garage",
    placeId: "sudirman-hub",
    floorId: "sudirman-parking",
    floorName: "Parking",
    areaName: "Garage",
    name: "Garage",
    kind: "parking gate",
    status: "online",
    gatewayId: "gw-sudirman-a",
    gatewaySerial: "MP-GW-JK",
    deviceCount: 2,
    description: "Secure parking entry with reader unlocks",
  },
  {
    id: "door-11f",
    placeId: "sudirman-hub",
    floorId: "sudirman-3f",
    floorName: "3rd Floor",
    areaName: "Office",
    name: "11th Floor Entry",
    kind: "office",
    status: "online",
    gatewayId: "gw-sudirman-a",
    gatewaySerial: "MP-GW-JK",
    deviceCount: 1,
    description: "Primary office reader",
  },
  {
    id: "door-lobby",
    placeId: "sudirman-hub",
    floorId: "sudirman-1f",
    floorName: "1st Floor",
    areaName: "Lobby",
    name: "Lobby Turnstile",
    kind: "turnstile",
    status: "offline",
    gatewayId: "gw-sudirman-b",
    gatewaySerial: "MP-GW-LB",
    deviceCount: 1,
    description: "Visitor and staff turnstile",
  },
]

const fallbackHardware: KisiHardwareResource[] = [
  {
    id: "gw-sudirman-a",
    placeId: "sudirman-hub",
    gatewayId: "gw-sudirman-a",
    name: "MP-GW-JK",
    type: "Gateway",
    statusLabel: "Online",
    tone: "success",
    location: "Sudirman Hub",
    lastSeenLabel: "2 minutes ago",
    doorNames: ["Garage", "11th Floor Entry"],
  },
  {
    id: "reader-sudirman-11f",
    placeId: "sudirman-hub",
    gatewayId: "gw-sudirman-a",
    name: "Reader Pro 11F",
    type: "Reader",
    statusLabel: "Online",
    tone: "success",
    location: "11th Floor Entry",
    lastSeenLabel: "2 minutes ago",
    doorNames: ["11th Floor Entry"],
  },
  {
    id: "controller-sudirman-garage",
    placeId: "sudirman-hub",
    gatewayId: "gw-sudirman-b",
    name: "Garage Controller",
    type: "Controller",
    statusLabel: "Needs review",
    tone: "warning",
    location: "Garage",
    lastSeenLabel: "18 minutes ago",
    doorNames: ["Garage"],
  },
]

const fallbackEvents: KisiEventResource[] = [
  {
    id: "event-1",
    placeId: "sudirman-hub",
    doorId: "door-11f",
    object: "11th Floor Entry",
    action: "Unlock failed",
    user: "unknown_device",
    timeLabel: "12:02 PM",
    statusLabel: "Review",
    tone: "warning",
    details: "Location could not be determined",
    at: new Date().toISOString(),
  },
  {
    id: "event-2",
    placeId: "sudirman-hub",
    doorId: "door-garage",
    object: "Garage",
    action: "Unlock successful",
    user: "indra.saputra",
    timeLabel: "11:20 AM",
    statusLabel: "Success",
    tone: "success",
    details: "Mobile credential accepted",
    at: new Date().toISOString(),
  },
  {
    id: "event-3",
    placeId: "sudirman-hub",
    doorId: "door-lobby",
    object: "Lobby Turnstile",
    action: "Door forced open",
    user: "system",
    timeLabel: "10:19 AM",
    statusLabel: "Open",
    tone: "danger",
    details: "Alarm opened for security review",
    at: new Date().toISOString(),
  },
]

const fallbackUsers: KisiUserResource[] = [
  {
    id: "user-andra",
    placeId: "sudirman-hub",
    name: "Andra Saputra",
    email: "andra.saputra@tenant.local",
    role: "Admin",
    statusLabel: "Active",
    tone: "success",
    accessDateLabel: "Apr 13, 2026",
    sourceLabel: "Manual",
  },
  {
    id: "user-dina",
    placeId: "sudirman-hub",
    name: "Dina Wijaya",
    email: "dina.wijaya@tenant.local",
    role: "Member",
    statusLabel: "Active",
    tone: "success",
    accessDateLabel: "Unconfirmed user",
    sourceLabel: "Directory",
  },
  {
    id: "user-pending",
    placeId: "sudirman-hub",
    name: "Unconfirmed user",
    email: "pending.invite@tenant.local",
    role: "Member",
    statusLabel: "Pending",
    tone: "warning",
    accessDateLabel: "Unconfirmed user",
    sourceLabel: "Invite",
  },
  {
    id: "user-rina",
    placeId: "sudirman-hub",
    name: "Rina Hartono",
    email: "rina.hartono@tenant.local",
    role: "Place Admin",
    statusLabel: "Suspended",
    tone: "danger",
    accessDateLabel: "Mar 28, 2026",
    sourceLabel: "Manual",
  },
]

const fallbackCredentials: KisiCredentialResource[] = [
  {
    id: "credential-andra-mobile",
    user: "Andra Saputra",
    userID: "user-andra",
    type: "Mobile",
    statusLabel: "Active",
    tone: "success",
    issuedLabel: "Apr 26, 2026",
    expiresLabel: "No expiry",
  },
  {
    id: "credential-dina-link",
    user: "Dina Wijaya",
    userID: "user-dina",
    type: "Access Link",
    statusLabel: "Pending",
    tone: "warning",
    issuedLabel: "Apr 25, 2026",
    expiresLabel: "No expiry",
  },
  {
    id: "credential-rina-mobile",
    user: "Rina Hartono",
    userID: "user-rina",
    type: "Mobile",
    statusLabel: "Revoked",
    tone: "danger",
    issuedLabel: "Apr 18, 2026",
    expiresLabel: "Revoked",
  },
]

const fallbackGroups: KisiGroupResource[] = [
  {
    id: "group-service-personnel",
    placeId: "sudirman-hub",
    name: "Service Personnel",
    kind: "Door group",
    memberCount: 18,
    targetLabel: "Garage, 11th Floor Entry",
    description: "People and access resources used by service teams.",
    zoneIds: ["zone-sudirman-garage"],
    zoneFloorIds: ["sudirman-parking"],
    statusLabel: "Enabled",
    tone: "success",
  },
  {
    id: "group-lobby-staff",
    placeId: "sudirman-hub",
    name: "Lobby Staff",
    kind: "Door group",
    memberCount: 12,
    targetLabel: "Lobby Turnstile",
    description: "Front desk and visitor handling access.",
    zoneIds: ["zone-sudirman-lobby"],
    zoneFloorIds: ["sudirman-1f"],
    statusLabel: "Enabled",
    tone: "success",
  },
  {
    id: "group-vendor-temporary",
    placeId: "kuningan-tower",
    name: "Vendor Temporary",
    kind: "User group",
    memberCount: 6,
    targetLabel: "Temporary assignments",
    description: "Short-term vendor access awaiting review.",
    statusLabel: "Review",
    tone: "warning",
  },
]

const fallbackTeams: KisiTeamResource[] = [
  {
    id: "team-engineering",
    placeId: "sudirman-hub",
    name: "Engineering Team",
    scopeLabel: "Sudirman Hub",
    sourceLabel: "SCIM group: engineering",
    description: "Office engineering staff managed through directory sync.",
    memberCount: 2,
    accessRightCount: 1,
    statusLabel: "Enabled",
    tone: "success",
  },
  {
    id: "team-operations",
    placeId: "sudirman-hub",
    name: "Operations Team",
    scopeLabel: "Sudirman Hub",
    sourceLabel: "Manual",
    description: "Building operations and facility support.",
    memberCount: 1,
    accessRightCount: 1,
    statusLabel: "Enabled",
    tone: "success",
  },
]

const fallbackTeamMemberships: KisiTeamMembershipResource[] = [
  {
    id: "team-engineering-andra",
    teamId: "team-engineering",
    placeId: "sudirman-hub",
    memberType: "User",
    name: "Andra Saputra",
    email: "andra.saputra@tenant.local",
    sourceLabel: "SCIM",
    statusLabel: "Active",
    tone: "success",
  },
  {
    id: "team-engineering-dina",
    teamId: "team-engineering",
    placeId: "sudirman-hub",
    memberType: "User",
    name: "Dina Wijaya",
    email: "dina.wijaya@tenant.local",
    sourceLabel: "SCIM",
    statusLabel: "Active",
    tone: "success",
  },
]

const fallbackAccessRights: KisiAccessRightResource[] = [
  {
    id: "access-service-personnel",
    placeId: "sudirman-hub",
    name: "Service Personnel",
    subjectType: "Group",
    targetLabel: "Garage, 11th Floor Entry",
    ruleLabel: "Weekdays 08:00-18:00",
    statusLabel: "Enabled",
    tone: "success",
  },
  {
    id: "access-lobby-staff",
    placeId: "sudirman-hub",
    name: "Lobby Staff",
    subjectType: "Group",
    targetLabel: "Lobby Turnstile",
    ruleLabel: "Always",
    statusLabel: "Enabled",
    tone: "success",
  },
  {
    id: "access-cleaning-vendor",
    placeId: "sudirman-hub",
    name: "Cleaning vendor",
    subjectType: "Access Link",
    targetLabel: "Service Personnel",
    ruleLabel: "Expires Apr 30, 2026",
    statusLabel: "Review",
    tone: "warning",
  },
]

export const fallbackKisiResourceSummary: KisiResourceSummary = {
  places: fallbackPlaces,
  floors: fallbackFloors,
  zones: fallbackZones,
  doors: fallbackDoors,
  hardware: fallbackHardware,
  events: fallbackEvents,
  users: fallbackUsers,
  credentials: fallbackCredentials,
  groups: fallbackGroups,
  teams: fallbackTeams,
  teamMemberships: fallbackTeamMemberships,
  accessRights: fallbackAccessRights,
  partial: false,
}

function fulfilledValue<T>(result: PromiseSettledResult<T>, fallback: T): T {
  return result.status === "fulfilled" ? result.value : fallback
}

function isRejected<T>(result: PromiseSettledResult<T>) {
  return result.status === "rejected"
}

function fulfilledResult<T>(value: T): PromiseFulfilledResult<T> {
  return { status: "fulfilled", value }
}

async function settleFallback<T>(enabled: boolean, load: () => Promise<T>, fallback: T): Promise<PromiseSettledResult<T>> {
  if (!enabled) {
    return fulfilledResult(fallback)
  }
  try {
    return fulfilledResult(await load())
  } catch (reason) {
    return { status: "rejected", reason }
  }
}

function slugify(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
}

function titleize(value: string) {
  return value
    .replace(/[_-]+/g, " ")
    .split(" ")
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ")
}

function formatCount(count: number, singular: string) {
  return `${count} ${singular}${count === 1 ? "" : "s"}`
}

function formatTimeLabel(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return "--:--"
  }
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  }).format(parsed)
}

function formatDateLabel(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return "Unconfirmed user"
  }
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  }).format(parsed)
}

function formatRelativeSeen(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return "Unknown"
  }
  const diffMs = Date.now() - parsed.getTime()
  const diffMinutes = Math.max(0, Math.round(diffMs / 60000))
  if (diffMinutes < 1) {
    return "Just now"
  }
  if (diffMinutes < 60) {
    return `${diffMinutes} minute${diffMinutes === 1 ? "" : "s"} ago`
  }
  const diffHours = Math.round(diffMinutes / 60)
  if (diffHours < 24) {
    return `${diffHours} hour${diffHours === 1 ? "" : "s"} ago`
  }
  const diffDays = Math.round(diffHours / 24)
  return `${diffDays} day${diffDays === 1 ? "" : "s"} ago`
}

function formatActor(actor: string) {
  if (!actor.trim()) {
    return "system"
  }
  return actor.includes("@") ? actor.split("@")[0] : actor
}

function formatAccessAction(event: AccessEvent) {
  if (event.result === "success") {
    return "Unlock successful"
  }
  if (event.result === "denied") {
    return "Unlock failed"
  }
  if (event.type.trim()) {
    return titleize(event.type)
  }
  return "Access warning"
}

function eventTone(event: AccessEvent): KisiResourceTone {
  if (event.result === "success") {
    return "success"
  }
  if (event.type.toLowerCase().includes("forced")) {
    return "danger"
  }
  return "warning"
}

function eventStatusLabel(tone: KisiResourceTone) {
  if (tone === "success") {
    return "Success"
  }
  if (tone === "danger") {
    return "Open"
  }
  return "Review"
}

function eventSetTone(event: EventSetEvent): KisiResourceTone {
  if (event.success || event.result === "success") {
    return "success"
  }
  if (event.result === "denied" || event.type.toLowerCase().includes("denied")) {
    return "warning"
  }
  if (event.result === "warning") {
    return "warning"
  }
  return "info"
}

function formatEventSetAction(event: EventSetEvent) {
  if (event.type === "access_granted") {
    return "Unlock successful"
  }
  if (event.type === "access_denied") {
    return "Unlock failed"
  }
  return titleize(event.type || "Event")
}

function userTone(status: string): KisiResourceTone {
  const normalized = status.trim().toLowerCase()
  if (normalized === "active" || normalized === "enabled") {
    return "success"
  }
  if (normalized === "pending" || normalized === "invited" || normalized === "unconfirmed") {
    return "warning"
  }
  return "danger"
}

function userStatusLabel(status: string) {
  const normalized = status.trim().toLowerCase()
  if (normalized === "active" || normalized === "enabled") {
    return "Active"
  }
  if (normalized === "pending" || normalized === "invited" || normalized === "unconfirmed") {
    return "Pending"
  }
  if (normalized === "suspended" || normalized === "disabled" || normalized === "inactive") {
    return "Suspended"
  }
  return titleize(status || "Unknown")
}

function credentialTone(status: string): KisiResourceTone {
  const normalized = status.trim().toLowerCase()
  if (normalized === "active") {
    return "success"
  }
  if (normalized === "issued" || normalized === "pending" || normalized === "suspended") {
    return "warning"
  }
  return "danger"
}

function credentialStatusLabel(status: string) {
  const normalized = status.trim().toLowerCase()
  if (normalized === "active") {
    return "Active"
  }
  if (normalized === "issued" || normalized === "pending") {
    return "Pending"
  }
  if (normalized === "suspended") {
    return "Suspended"
  }
  if (normalized === "revoked") {
    return "Revoked"
  }
  return titleize(status || "Unknown")
}

function credentialType(pass: WalletPassInstance) {
  if (pass.target_type === "visitor") {
    return "Access Link"
  }
  return pass.provider ? `${titleize(pass.provider)} Mobile` : "Mobile"
}

function cardCredentialType(card: Card) {
  if (card.credential_kind === "google_wallet" || card.provider === "google") {
    return "Google Wallet"
  }
  if (card.credential_kind === "apple_wallet" || card.provider === "apple") {
    return "Apple Wallet"
  }
  if (card.credential_kind === "physical_card" || card.uid || card.card_number) {
    return card.provider && card.provider !== "physical_card" ? `${titleize(card.provider)} Card` : "Physical Card"
  }
  return card.provider ? `${titleize(card.provider)} Credential` : "Credential"
}

function credentialStatusLabelFromCard(status: Card["status"]) {
  if (status === "activated") {
    return "Active"
  }
  if (status === "unassigned") {
    return "Pending"
  }
  if (status === "revoked") {
    return "Revoked"
  }
  return "Suspended"
}

function credentialToneFromCard(status: Card["status"]): KisiResourceTone {
  if (status === "activated") {
    return "success"
  }
  if (status === "unassigned") {
    return "warning"
  }
  return "danger"
}

function productUserRoleLabel(role: string) {
  const normalized = role.trim().toLowerCase()
  if (normalized === "super_admin" || normalized === "tenant_admin") {
    return "Organization Admin"
  }
  if (normalized === "building_admin" || normalized === "operator") {
    return "Place Admin"
  }
  return titleize(role || "Member")
}

function compactListSummary(values: string[], fallback: string) {
  const unique = Array.from(new Set(values.map((value) => value.trim()).filter(Boolean)))
  if (unique.length === 0) {
    return fallback
  }
  if (unique.length <= 2) {
    return unique.join(", ")
  }
  return `${unique.slice(0, 2).join(", ")} +${unique.length - 2}`
}

function deviceType(device: GatewayDevice) {
  const normalized = device.kind.toLowerCase()
  if (normalized.includes("reader")) {
    return "Reader"
  }
  if (normalized.includes("controller")) {
    return "Controller"
  }
  if (normalized.includes("relay")) {
    return "Relay"
  }
  if (normalized.includes("sensor")) {
    return "Sensor"
  }
  return titleize(device.kind || "Device")
}

function filterBuildingsByViewerScope(items: Building[], viewer: CurrentUser) {
  const buildingIDs = new Set(getViewerBuildingIDs(viewer))
  if ((viewer.role !== "building_admin" && viewer.role !== "operator") || buildingIDs.size === 0) {
    return items
  }
  return items.filter((item) => buildingIDs.has(item.id))
}

function gatewayDoorNames(gateway: Gateway, doorsByID: Map<string, Door>, doorsByGatewayID: Map<string, Door[]>) {
  const boundDoorIDs = gateway.bound_door_ids ?? []
  const names = boundDoorIDs
    .map((doorID) => doorsByID.get(doorID)?.name)
    .filter((name): name is string => Boolean(name))
  if (names.length > 0) {
    return names
  }
  return (doorsByGatewayID.get(gateway.id) ?? []).map((door) => door.name)
}

export function buildKisiHardwareFromReferenceResources({
  controllers,
  readers,
  terminals,
  gateways = [],
  doors,
  places,
  useGatewayFallback = false,
}: {
  controllers: Controller[]
  readers: Reader[]
  terminals: Terminal[]
  gateways?: Gateway[]
  doors: Door[]
  places: Array<Pick<KisiPlaceResource, "id" | "name">>
  useGatewayFallback?: boolean
}) {
  const doorsByID = new Map(doors.map((door) => [door.id, door]))
  const doorsByGatewayID = new Map<string, Door[]>()
  doors.forEach((door) => {
    if (!door.gateway_id) {
      return
    }
    const current = doorsByGatewayID.get(door.gateway_id) ?? []
    current.push(door)
    doorsByGatewayID.set(door.gateway_id, current)
  })

  const placeByID = new Map(places.map((place) => [place.id, place]))
  const doorNamesForLockIDs = (lockIDs: string[]) => lockIDs.map((lockID) => doorsByID.get(lockID)?.name).filter((name): name is string => Boolean(name))
  const referenceHardwareResources = [
    ...controllers.map((controller) => {
      const place = placeByID.get(controller.place_id)
      const doorNames = doorNamesForLockIDs(controller.lock_ids ?? [])
      const tone: KisiResourceTone = controller.status === "online" ? "success" : "warning"
      return {
        id: controller.id,
        placeId: controller.place_id,
        gatewayId: controller.id,
        name: controller.name,
        type: "Controller",
        statusLabel: controller.status === "online" ? "Online" : "Needs review",
        tone,
        location: place?.name ?? "Unassigned place",
        lastSeenLabel: formatRelativeSeen(controller.last_seen_at),
        doorNames,
      } satisfies KisiHardwareResource
    }),
    ...readers.map((reader) => {
      const place = placeByID.get(reader.place_id)
      const doorNames = doorNamesForLockIDs(reader.lock_ids ?? [])
      const tone: KisiResourceTone = reader.status === "online" ? "success" : "warning"
      return {
        id: reader.id,
        placeId: reader.place_id,
        gatewayId: reader.controller_id,
        name: reader.name,
        type: "Reader",
        statusLabel: reader.status === "online" ? "Online" : "Needs review",
        tone,
        location: doorNames[0] ?? place?.name ?? "Unassigned place",
        lastSeenLabel: formatRelativeSeen(reader.last_seen_at),
        doorNames,
      } satisfies KisiHardwareResource
    }),
    ...terminals.map((terminal) => {
      const place = placeByID.get(terminal.place_id)
      const tone: KisiResourceTone = terminal.status === "offline" ? "warning" : "success"
      return {
        id: terminal.id,
        placeId: terminal.place_id,
        gatewayId: terminal.controller_id ?? terminal.id,
        name: terminal.name,
        type: "Terminal",
        statusLabel: terminal.status === "offline" ? "Needs review" : "Online",
        tone,
        location: terminal.place?.name ?? place?.name ?? "Unassigned place",
        lastSeenLabel: formatRelativeSeen(terminal.last_seen_at ?? terminal.updated_at),
        doorNames: [],
      } satisfies KisiHardwareResource
    }),
  ]

  if (referenceHardwareResources.length > 0 || !useGatewayFallback) {
    return referenceHardwareResources
  }

  return gateways.flatMap((gateway) => {
    const place = placeByID.get(gateway.building_id)
    const doorNames = gatewayDoorNames(gateway, doorsByID, doorsByGatewayID)
    const gatewayTone: KisiResourceTone = gateway.status === "online" ? "success" : "warning"
    const gatewayRow: KisiHardwareResource = {
      id: gateway.id,
      placeId: gateway.building_id,
      gatewayId: gateway.id,
      name: gateway.serial_number,
      type: "Gateway",
      statusLabel: gateway.status === "online" ? "Online" : "Needs review",
      tone: gatewayTone,
      location: place?.name ?? "Unassigned place",
      lastSeenLabel: formatRelativeSeen(gateway.last_seen_at),
      doorNames,
    }

    const deviceRows = (gateway.devices ?? []).map((device) => {
      const deviceTone: KisiResourceTone = device.status === "online" ? "success" : "warning"
      return {
        id: device.id,
        placeId: gateway.building_id,
        gatewayId: gateway.id,
        name: device.serial_number,
        type: deviceType(device),
        statusLabel: device.status === "online" ? "Online" : "Needs review",
        tone: deviceTone,
        location: doorNames[0] ?? place?.name ?? "Unassigned place",
        lastSeenLabel: formatRelativeSeen(device.last_seen_at),
        doorNames,
      } satisfies KisiHardwareResource
    })

    return [gatewayRow, ...deviceRows]
  })
}

function placeIDForAccessScope(
  item: Pick<AccessPolicy | TemporaryAccess, "scope_type" | "building_id" | "area_id" | "door_id">,
  areasByID: Map<string, Area>,
  doorsByID: Map<string, Door>
) {
  if (item.scope_type === "building") {
    return item.building_id ?? ""
  }
  if (item.scope_type === "area") {
    return item.area_id ? areasByID.get(item.area_id)?.building_id ?? "" : ""
  }
  if (item.scope_type === "door") {
    return item.door_id ? doorsByID.get(item.door_id)?.building_id ?? "" : ""
  }
  return ""
}

function targetLabelForAccessScope(
  item: Pick<AccessPolicy | TemporaryAccess, "scope_type" | "building_id" | "area_id" | "door_id">,
  placesByID: Map<string, KisiPlaceResource>,
  areasByID: Map<string, Area>,
  doorsByID: Map<string, Door>
) {
  if (item.scope_type === "building") {
    return item.building_id ? placesByID.get(item.building_id)?.name ?? `Place ${item.building_id.slice(0, 8)}` : "Unassigned place"
  }
  if (item.scope_type === "area") {
    return item.area_id ? areasByID.get(item.area_id)?.name ?? `Area ${item.area_id.slice(0, 8)}` : "Unassigned area"
  }
  if (item.scope_type === "door") {
    return item.door_id ? doorsByID.get(item.door_id)?.name ?? `Door ${item.door_id.slice(0, 8)}` : "Unassigned door"
  }
  return "All organization places"
}

function policyStatusLabel(status: AccessPolicy["status"]) {
  if (status === "active") {
    return "Enabled"
  }
  if (status === "draft") {
    return "Review"
  }
  return "Disabled"
}

function policyTone(status: AccessPolicy["status"]): KisiResourceTone {
  if (status === "active") {
    return "success"
  }
  if (status === "draft") {
    return "warning"
  }
  return "danger"
}

function temporaryAccessStatus(item: TemporaryAccess) {
  const expiresAt = new Date(item.valid_until)
  if (!Number.isNaN(expiresAt.getTime()) && expiresAt.getTime() < Date.now()) {
    return { statusLabel: "Expired", tone: "danger" as KisiResourceTone }
  }
  return { statusLabel: "Enabled", tone: "success" as KisiResourceTone }
}

function formatAccessDateLabel(value?: string, fallback = "No expiry") {
  if (!value?.trim()) {
    return fallback
  }
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return fallback
  }
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  }).format(parsed)
}

function roleName(roleID: string, rolesByID: Map<string, Role>) {
  return rolesByID.get(roleID)?.name ?? titleize(roleID.replace(/^role_/, "") || "Access role")
}

function accessValidityStatus(validFrom?: string, validUntil?: string, rawStatus?: string) {
  const normalizedStatus = rawStatus?.trim().toLowerCase()
  if (normalizedStatus === "expired" || normalizedStatus === "revoked" || normalizedStatus === "disabled" || normalizedStatus === "inactive") {
    return { statusLabel: titleize(normalizedStatus), tone: "danger" as KisiResourceTone }
  }
  if (normalizedStatus === "pending" || normalizedStatus === "scheduled") {
    return { statusLabel: titleize(normalizedStatus), tone: "warning" as KisiResourceTone }
  }

  const startsAt = validFrom?.trim() ? new Date(validFrom).getTime() : Number.NaN
  if (!Number.isNaN(startsAt) && startsAt > Date.now()) {
    return { statusLabel: "Scheduled", tone: "warning" as KisiResourceTone }
  }

  const expiresAt = validUntil?.trim() ? new Date(validUntil).getTime() : Number.NaN
  if (!Number.isNaN(expiresAt) && expiresAt < Date.now()) {
    return { statusLabel: "Expired", tone: "danger" as KisiResourceTone }
  }

  return { statusLabel: "Enabled", tone: "success" as KisiResourceTone }
}

function accessRightNeedsReview(statusLabel: string, reviewedAt?: string) {
  const normalizedStatus = statusLabel.trim().toLowerCase()
  return !reviewedAt?.trim() && normalizedStatus !== "" && normalizedStatus !== "enabled" && normalizedStatus !== "active"
}

function roleAssignmentSubjectType(item: RoleAssignment): KisiAccessRightResource["subjectType"] {
  if (item.assignee_type === "Team") {
    return "Team"
  }
  if (item.applies_to_type === "Group") {
    return "Group"
  }
  if (item.applies_to_type === "Place") {
    return "Place"
  }
  return "User"
}

function roleAssignmentPlaceID(item: RoleAssignment, groupsByID: Map<string, KisiGroupResource>) {
  if (item.applies_to_type === "Place") {
    return item.applies_to_id
  }
  if (item.applies_to_type === "Group") {
    return groupsByID.get(item.applies_to_id)?.placeId ?? ""
  }
  return ""
}

function scopeLabelForRoleAssignment(
  item: RoleAssignment,
  placesByID: Map<string, KisiPlaceResource>,
  groupsByID: Map<string, KisiGroupResource>
) {
  if (item.applies_to_type === "Organization") {
    return "All organization places"
  }
  if (item.applies_to_type === "Place") {
    return placesByID.get(item.applies_to_id)?.name ?? `Place ${item.applies_to_id.slice(0, 8)}`
  }
  return groupsByID.get(item.applies_to_id)?.name ?? `Group ${item.applies_to_id.slice(0, 8)}`
}

function assigneeLabelForRoleAssignment(
  item: RoleAssignment,
  usersByID: Map<string, KisiUserResource>,
  teamsByID: Map<string, KisiTeamResource>
) {
  if (item.assignee_type === "User") {
    return usersByID.get(item.assignee_id)?.name ?? (item.assignee_email ? formatActor(item.assignee_email) : `User ${item.assignee_id.slice(0, 8)}`)
  }
  if (item.assignee_type === "Team") {
    return teamsByID.get(item.assignee_id)?.name ?? item.assignee_email ?? `Team ${item.assignee_id.slice(0, 8)}`
  }
  return item.assignee_email || `Guest ${item.assignee_id.slice(0, 8)}`
}

function roleAssignmentRuleLabel(item: RoleAssignment, rolesByID: Map<string, Role>) {
  const base = roleName(item.role_id, rolesByID)
  if (item.valid_until?.trim()) {
    return `${base} until ${formatAccessDateLabel(item.valid_until)}`
  }
  if (item.valid_from?.trim()) {
    return `${base} from ${formatAccessDateLabel(item.valid_from)}`
  }
  return base
}

function sharePlaceID(
  item: Share,
  doorsByID: Map<string, Pick<Door, "building_id" | "name">>,
  groupsByID: Map<string, KisiGroupResource>
) {
  if (item.place_id?.trim()) {
    return item.place_id
  }
  if (item.lock_id?.trim()) {
    return doorsByID.get(item.lock_id)?.building_id ?? ""
  }
  if (item.group_id?.trim()) {
    return groupsByID.get(item.group_id)?.placeId ?? ""
  }
  return ""
}

function targetLabelForShare(
  item: Share,
  rolesByID: Map<string, Role>,
  placesByID: Map<string, KisiPlaceResource>,
  groupsByID: Map<string, KisiGroupResource>,
  doorsByID: Map<string, Pick<Door, "building_id" | "name">>
) {
  if (item.group_id?.trim()) {
    return groupsByID.get(item.group_id)?.name ?? `Group ${item.group_id.slice(0, 8)}`
  }
  if (item.lock_id?.trim()) {
    return doorsByID.get(item.lock_id)?.name ?? `Door ${item.lock_id.slice(0, 8)}`
  }
  if (item.place_id?.trim()) {
    return placesByID.get(item.place_id)?.name ?? `Place ${item.place_id.slice(0, 8)}`
  }
  return roleName(item.role_id, rolesByID)
}

export function buildKisiAccessRightsFromReferenceResources({
  roleAssignments,
  shares,
  roles,
  places,
  groups,
  teams = [],
  users,
  doors,
}: {
  roleAssignments: RoleAssignment[]
  shares: Share[]
  roles: Role[]
  places: KisiPlaceResource[]
  groups: KisiGroupResource[]
  teams?: KisiTeamResource[]
  users: KisiUserResource[]
  doors: Array<Pick<Door, "id" | "building_id" | "name">>
}) {
  const rolesByID = new Map(roles.map((role) => [role.id, role]))
  const placesByID = new Map(places.map((place) => [place.id, place]))
  const groupsByID = new Map(groups.map((group) => [group.id, group]))
  const teamsByID = new Map(teams.map((team) => [team.id, team]))
  const usersByID = new Map(users.map((user) => [user.id, user]))
  const doorsByID = new Map(doors.map((door) => [door.id, door]))

  const assignmentRights = roleAssignments.map((assignment) => {
    const subjectType = roleAssignmentSubjectType(assignment)
    const scopeLabel = scopeLabelForRoleAssignment(assignment, placesByID, groupsByID)
    const assigneeLabel = assigneeLabelForRoleAssignment(assignment, usersByID, teamsByID)
    const status = accessValidityStatus(assignment.valid_from, assignment.valid_until)

    return {
      id: assignment.id,
      sourceType: "role_assignment",
      placeId: roleAssignmentPlaceID(assignment, groupsByID),
      name: subjectType === "Group" || subjectType === "Place" ? scopeLabel : assigneeLabel,
      subjectType,
      targetLabel: subjectType === "Group" || subjectType === "Place" ? assigneeLabel : scopeLabel,
      ruleLabel: roleAssignmentRuleLabel(assignment, rolesByID),
      statusLabel: status.statusLabel,
      tone: status.tone,
      reviewedAt: assignment.reviewed_at,
      reviewedBy: assignment.reviewed_by,
      needsReview: accessRightNeedsReview(status.statusLabel, assignment.reviewed_at),
    } satisfies KisiAccessRightResource
  })

  const shareRights = shares.map((share) => {
    const status = accessValidityStatus(share.valid_from, share.valid_until, share.status)

    return {
      id: share.id,
      sourceType: "share",
      placeId: sharePlaceID(share, doorsByID, groupsByID),
      name: share.grantee_name || share.email || "Access link",
      subjectType: "Access Link",
      targetLabel: targetLabelForShare(share, rolesByID, placesByID, groupsByID, doorsByID),
      ruleLabel: share.valid_until?.trim() ? `Expires ${formatAccessDateLabel(share.valid_until)}` : roleName(share.role_id, rolesByID),
      statusLabel: status.statusLabel,
      tone: status.tone,
      reviewedAt: share.reviewed_at,
      reviewedBy: share.reviewed_by,
      needsReview: accessRightNeedsReview(status.statusLabel, share.reviewed_at),
    } satisfies KisiAccessRightResource
  })

  return [...assignmentRights, ...shareRights].sort((left, right) => left.name.localeCompare(right.name))
}

export async function loadKisiResourceSummary(token: string, viewer: CurrentUser): Promise<KisiResourceSummary> {
  const tenantID = isPlatformViewer(viewer) ? undefined : getViewerTenantID(viewer)
  const [
    buildingsResult,
    floorsResult,
    areasResult,
    doorsResult,
    controllersResult,
    readersResult,
    terminalsResult,
    eventSetResult,
    usersResult,
    cardsResult,
    cardAssignmentsResult,
    userGroupsResult,
    doorGroupsResult,
    groupLocksResult,
    groupZonesResult,
    teamsResult,
    teamMembershipsResult,
    rolesResult,
    roleAssignmentsResult,
    sharesResult,
  ] = await Promise.allSettled([
    listPlaces(token, tenantID),
    listFloors(token, tenantID),
    listAreas(token, tenantID),
    listLocks(token, tenantID),
    listControllers(token, tenantID ? { tenant_id: tenantID, sort: "name" } : { sort: "name" }),
    listReaders(token, tenantID ? { tenant_id: tenantID, sort: "name" } : { sort: "name" }),
    listTerminals(token, tenantID ? { tenant_id: tenantID, sort: "name" } : { sort: "name" }),
    createEventSet(token, tenantID ? { tenant_id: tenantID } : undefined),
    listAccessUsers(token, tenantID),
    listCards(token, tenantID ? { tenant_id: tenantID } : undefined),
    listCardAssignments(token, tenantID ? { tenant_id: tenantID } : undefined),
    listGroups(token, tenantID),
    listDoorGroups(token, tenantID),
    listGroupLocks(token, tenantID ? { tenant_id: tenantID } : undefined),
    listGroupZones(token, tenantID ? { tenant_id: tenantID } : undefined),
    listTeams(token, tenantID ? { tenant_id: tenantID, sort: "name" } : { sort: "name" }),
    listTeamMemberships(token, tenantID ? { tenant_id: tenantID } : undefined),
    listRoles(token),
    listRoleAssignments(token, tenantID ? { tenant_id: tenantID } : undefined),
    listShares(token, tenantID ? { tenant_id: tenantID } : undefined),
  ])
  const useGatewayFallback = isRejected(controllersResult) || isRejected(readersResult) || isRejected(terminalsResult)
  const [gatewaysResult, eventsResult, walletPassesResult, accessPoliciesResult, temporaryAccessResult] = await Promise.all([
    settleFallback(useGatewayFallback, () => listGateways(token), [] as Gateway[]),
    settleFallback(isRejected(eventSetResult), () => listAccessEvents(token, { page: 1, limit: 120 }), [] as AccessEvent[]),
    settleFallback(isRejected(cardsResult), () => listWalletPasses(token, tenantID, { page: 1, limit: 120 }), [] as WalletPassInstance[]),
    settleFallback(isRejected(roleAssignmentsResult), () => listAccessPolicies(token), [] as AccessPolicy[]),
    settleFallback(isRejected(sharesResult), () => listTemporaryAccess(token), [] as TemporaryAccess[]),
  ])

  const buildings = filterBuildingsByViewerScope(fulfilledValue<Building[]>(buildingsResult, []), viewer)
  const buildingIDs = new Set(buildings.map((building) => building.id))
  const floors = fulfilledValue<Floor[]>(floorsResult, []).filter((floor) => buildingIDs.has(floor.building_id))
  const areas = fulfilledValue<Area[]>(areasResult, []).filter((area) => buildingIDs.has(area.building_id))
  const doors = fulfilledValue<Door[]>(doorsResult, []).filter((door) => buildingIDs.has(door.building_id))
  const controllers = fulfilledValue<Controller[]>(controllersResult, []).filter((controller) => buildingIDs.has(controller.place_id))
  const controllersByID = new Map(controllers.map((controller) => [controller.id, controller]))
  const readers = fulfilledValue<Reader[]>(readersResult, []).filter((reader) => buildingIDs.has(reader.place_id))
  const terminals = fulfilledValue<Terminal[]>(terminalsResult, []).filter((terminal) => buildingIDs.has(terminal.place_id))
  const gateways = fulfilledValue<Gateway[]>(gatewaysResult, []).filter((gateway) => buildingIDs.has(gateway.building_id))
  const eventSetEvents = fulfilledValue<EventSet>(eventSetResult, {
    id: "event_set_fallback",
    created_at: "",
    status: "failed",
    events: [],
  }).events.filter((event) => !event.place_id || buildingIDs.has(event.place_id))
  const events = (isRejected(eventSetResult) ? fulfilledValue<AccessEvent[]>(eventsResult, []) : []).filter((event) => buildingIDs.has(event.building_id))
  const users = fulfilledValue<AccessUser[]>(usersResult, []).filter((user) => !user.building_id || buildingIDs.has(user.building_id))
  const cards = fulfilledValue<Card[]>(cardsResult, [])
  const cardAssignments = fulfilledValue<CardAssignment[]>(cardAssignmentsResult, [])
  const walletPasses = isRejected(cardsResult) ? fulfilledValue<WalletPassInstance[]>(walletPassesResult, []) : []
  const userGroups = fulfilledValue<UserGroup[]>(userGroupsResult, [])
  const doorGroups = fulfilledValue<DoorGroup[]>(doorGroupsResult, [])
  const groupLocks = fulfilledValue<GroupLock[]>(groupLocksResult, [])
  const groupZones = fulfilledValue<GroupZone[]>(groupZonesResult, []).filter((zone) => buildingIDs.has(zone.place_id))
  const teams = fulfilledValue<Team[]>(teamsResult, []).filter((team) => !team.place_id || buildingIDs.has(team.place_id))
  const teamIDs = new Set(teams.map((team) => team.id))
  const teamMemberships = fulfilledValue<TeamMembership[]>(teamMembershipsResult, []).filter((membership) => teamIDs.has(membership.team_id))
  const roles = fulfilledValue<Role[]>(rolesResult, [])
  const roleAssignments = fulfilledValue<RoleAssignment[]>(roleAssignmentsResult, [])
  const shares = fulfilledValue<Share[]>(sharesResult, [])
  const accessPolicies = (isRejected(roleAssignmentsResult) ? fulfilledValue<AccessPolicy[]>(accessPoliciesResult, []) : []).filter((policy) => {
    const policyPlaceID = policy.building_id || (policy.area_id ? areas.find((area) => area.id === policy.area_id)?.building_id : "") || (policy.door_id ? doors.find((door) => door.id === policy.door_id)?.building_id : "")
    return !policyPlaceID || buildingIDs.has(policyPlaceID)
  })
  const temporaryAccess = (isRejected(sharesResult) ? fulfilledValue<TemporaryAccess[]>(temporaryAccessResult, []) : []).filter((grant) => {
    const grantPlaceID = grant.building_id || (grant.area_id ? areas.find((area) => area.id === grant.area_id)?.building_id : "") || (grant.door_id ? doors.find((door) => door.id === grant.door_id)?.building_id : "")
    return !grantPlaceID || buildingIDs.has(grantPlaceID)
  })

  const floorsByID = new Map(floors.map((floor) => [floor.id, floor]))
  const areasByID = new Map(areas.map((area) => [area.id, area]))
  const doorsByID = new Map(doors.map((door) => [door.id, door]))
  const gatewaysByID = new Map(gateways.map((gateway) => [gateway.id, gateway]))

  const places = buildings
    .map((building) => {
      const placeDoors = doors.filter((door) => door.building_id === building.id)
      const placeGateways = gateways.filter((gateway) => gateway.building_id === building.id)
      const placeControllers = controllers.filter((controller) => controller.place_id === building.id)
      const hardwareCount = placeControllers.length > 0 || !useGatewayFallback ? placeControllers.length : placeGateways.length
      const offlineCount =
        placeDoors.filter((door) => door.status === "offline").length +
        (placeControllers.length > 0 || !useGatewayFallback ? placeControllers.filter((controller) => controller.status !== "online").length : placeGateways.filter((gateway) => gateway.status !== "online").length)

      return {
        id: building.id,
        name: building.name,
        address: building.address || "No address set",
        region: building.region || "Unassigned region",
        doorCount: placeDoors.length,
        gatewayCount: hardwareCount,
        offlineCount,
        statusLabel: offlineCount > 0 ? `${offlineCount} offline` : "All online",
        tone: offlineCount > 0 ? "warning" : "success",
      } satisfies KisiPlaceResource
    })
    .sort((left, right) => left.name.localeCompare(right.name))
  const placesByID = new Map(places.map((place) => [place.id, place]))

  const floorResources = floors
    .map((floor) => {
      const floorDoors = doors.filter((door) => door.floor_id === floor.id)
      const floorGateways = gateways.filter((gateway) => gateway.building_id === floor.building_id)
      const floorControllers = controllers.filter((controller) => controller.place_id === floor.building_id)
      const areaNames = areas
        .filter((area) => area.floor_id === floor.id)
        .map((area) => area.name)
        .slice(0, 3)
      const offlineCount = floorDoors.filter((door) => door.status === "offline").length

      return {
        id: floor.id,
        placeId: floor.building_id,
        name: floor.name,
        description: areaNames.length > 0 ? areaNames.join(", ") : "No areas mapped yet",
        doorCount: floorDoors.length,
        hardwareCount: floorControllers.length > 0 || !useGatewayFallback ? floorControllers.length : floorGateways.length,
        statusLabel: offlineCount > 0 ? "Review" : "Online",
        tone: offlineCount > 0 ? "warning" : "success",
      } satisfies KisiFloorResource
    })
    .sort((left, right) => left.name.localeCompare(right.name, undefined, { numeric: true }))

  const zoneResources = areas
    .map((area) => {
      const floor = floorsByID.get(area.floor_id)
      const areaDoors = doors.filter((door) => door.area_id === area.id)
      return {
        id: area.id,
        placeId: area.building_id,
        floorId: area.floor_id,
        floorName: floor?.name ?? "Unassigned floor",
        name: area.name,
        description: floor ? `${floor.name} zone` : "Unassigned floor zone",
        doorCount: areaDoors.length,
      } satisfies KisiZoneResource
    })
    .sort((left, right) => left.name.localeCompare(right.name))

  const doorResources = doors
    .map((door) => {
      const floor = floorsByID.get(door.floor_id)
      const area = areasByID.get(door.area_id)
      const gateway = gatewaysByID.get(door.gateway_id)
      const controller = controllersByID.get(door.gateway_id)
      const devices = gateway?.devices ?? []
      const referenceReaderCount = readers.filter((reader) => reader.controller_id === door.gateway_id && (reader.lock_ids ?? []).includes(door.id)).length

      return {
        id: door.id,
        placeId: door.building_id,
        floorId: door.floor_id,
        floorName: floor?.name ?? "Unassigned floor",
        areaName: area?.name ?? "Unassigned area",
        name: door.name,
        kind: titleize(door.kind),
        status: door.status,
        gatewayId: door.gateway_id,
        gatewaySerial: controller?.name ?? gateway?.serial_number ?? "Unassigned gateway",
        deviceCount: referenceReaderCount > 0 || !useGatewayFallback ? referenceReaderCount : devices.length,
        description: `${titleize(door.kind)} door on ${floor?.name ?? "unassigned floor"}`,
      } satisfies KisiDoorResource
    })
    .sort((left, right) => left.name.localeCompare(right.name))

  const hardwareResources = buildKisiHardwareFromReferenceResources({
    controllers,
    readers,
    terminals,
    gateways,
    doors,
    places,
    useGatewayFallback,
  })

  const referenceEventResources = eventSetEvents
    .map((event) => {
      const door = event.lock_id ? doorsByID.get(event.lock_id) : undefined
      const tone = eventSetTone(event)
      return {
        id: event.uuid,
        placeId: event.place_id ?? "",
        doorId: event.lock_id ?? event.object_id ?? "",
        object: door?.name ?? event.object_name ?? event.object_id ?? "Event",
        action: formatEventSetAction(event),
        user: formatActor(event.actor_name || event.actor_email || event.actor_id || "system"),
        timeLabel: formatTimeLabel(event.created_at),
        statusLabel: eventStatusLabel(tone),
        tone,
        details: event.detail || `${event.result || "event"} ${event.object_type ? `on ${event.object_type}` : ""}`.trim(),
        at: event.created_at,
      } satisfies KisiEventResource
    })
    .sort((left, right) => new Date(right.at).getTime() - new Date(left.at).getTime())

  const fallbackEventResources = events
    .map((event) => {
      const door = doorsByID.get(event.door_id)
      const tone = eventTone(event)
      return {
        id: event.id,
        placeId: event.building_id,
        doorId: event.door_id,
        object: door?.name ?? `Door ${event.door_id.slice(0, 8)}`,
        action: formatAccessAction(event),
        user: formatActor(event.actor),
        timeLabel: formatTimeLabel(event.at),
        statusLabel: eventStatusLabel(tone),
        tone,
        details: `${event.result} via gateway ${event.gateway_id.slice(0, 8)}`,
        at: event.at,
      } satisfies KisiEventResource
    })
    .sort((left, right) => new Date(right.at).getTime() - new Date(left.at).getTime())
  const eventResources = referenceEventResources.length > 0 || !isRejected(eventSetResult) ? referenceEventResources : fallbackEventResources

  const userResources = users
    .map((user) => {
      const tone = userTone(user.status)
      return {
        id: user.id,
        placeId: user.building_id ?? "",
        name: user.name || formatActor(user.email),
        email: user.email,
        role: productUserRoleLabel(user.role),
        roleValue: user.role,
        rawStatus: user.status,
        statusLabel: userStatusLabel(user.status),
        tone,
        accessDateLabel: formatDateLabel(user.created_at),
        sourceLabel: user.sync_source ? titleize(user.sync_source) : "Manual",
        groupIds: user.group_ids ?? [],
      } satisfies KisiUserResource
    })
    .sort((left, right) => left.name.localeCompare(right.name))

  const usersByID = new Map(userResources.map((user) => [user.id, user]))
  const teamMembershipsByTeamID = new Map<string, TeamMembership[]>()
  teamMemberships.forEach((membership) => {
    const current = teamMembershipsByTeamID.get(membership.team_id) ?? []
    current.push(membership)
    teamMembershipsByTeamID.set(membership.team_id, current)
  })

  const teamResources = teams
    .map((team) => {
      const memberships = teamMembershipsByTeamID.get(team.id) ?? []
      const accessRightCount = roleAssignments.filter((assignment) => assignment.assignee_type === "Team" && assignment.assignee_id === team.id).length
      return {
        id: team.id,
        placeId: team.place_id ?? "",
        name: team.name,
        scopeLabel: team.place_id ? placesByID.get(team.place_id)?.name ?? `Place ${team.place_id.slice(0, 8)}` : "Organization",
        sourceLabel: team.source || "Manual",
        description: team.description || "Team membership used for batch Access Rights.",
        memberCount: memberships.length,
        accessRightCount,
        statusLabel: "Enabled",
        tone: "success",
      } satisfies KisiTeamResource
    })
    .sort((left, right) => left.name.localeCompare(right.name))

  const teamsByID = new Map(teamResources.map((team) => [team.id, team]))
  const teamMembershipResources = teamMemberships
    .map((membership) => {
      const user = usersByID.get(membership.member_id)
      const team = teamsByID.get(membership.team_id)
      const tone = user?.tone ?? "success"
      return {
        id: membership.id,
        teamId: membership.team_id,
        placeId: team?.placeId ?? "",
        memberType: membership.member_type,
        name: user?.name ?? membership.member_name ?? `${membership.member_type} ${membership.member_id.slice(0, 8)}`,
        email: user?.email ?? membership.member_email ?? "No email",
        sourceLabel: membership.source || "Manual",
        statusLabel: user?.statusLabel ?? "Active",
        tone,
      } satisfies KisiTeamMembershipResource
    })
    .sort((left, right) => left.name.localeCompare(right.name))

  const cardAssignmentsByCardID = new Map(cardAssignments.map((assignment) => [assignment.card_id, assignment]))
  const cardCredentials = cards
    .map((card) => {
      const assignment = cardAssignmentsByCardID.get(card.id)
      const userID = card.user_id || assignment?.user_id || assignment?.assignee_id || card.assignee_id || ""
      const user = usersByID.get(userID)
      const statusLabel = credentialStatusLabelFromCard(card.status)
      return {
        id: card.id,
        user: user?.name ?? (assignment?.assignee_type ? `${assignment.assignee_type} ${assignment.assignee_id.slice(0, 8)}` : `Card ${card.id.slice(0, 8)}`),
        userID,
        type: cardCredentialType(card),
        statusLabel,
        tone: credentialToneFromCard(card.status),
        issuedLabel: formatDateLabel(card.issued_at),
        expiresLabel: card.expires_at ? formatDateLabel(card.expires_at) : "No expiry",
      } satisfies KisiCredentialResource
    })
    .sort((left, right) => left.user.localeCompare(right.user))

  const walletCredentials = walletPasses
    .map((pass) => {
      const user = usersByID.get(pass.target_id)
      const statusLabel = credentialStatusLabel(pass.status)
      return {
        id: pass.id,
        user: user?.name ?? `${titleize(pass.target_type)} ${pass.target_id.slice(0, 8)}`,
        userID: pass.target_id,
        type: credentialType(pass),
        statusLabel,
        tone: credentialTone(pass.status),
        issuedLabel: formatDateLabel(pass.issued_at),
        expiresLabel: pass.expires_at ? formatDateLabel(pass.expires_at) : "No expiry",
      } satisfies KisiCredentialResource
    })
    .sort((left, right) => left.user.localeCompare(right.user))
  const credentials = cardCredentials.length > 0 || !isRejected(cardsResult) ? cardCredentials : walletCredentials

  const groupResources = [
    ...userGroups.map((group) => {
      const memberCount = group.members?.length ?? 0
      return {
        id: group.id,
        tenantID: group.tenant_id,
        placeId: group.building_id && buildingIDs.has(group.building_id) ? group.building_id : "",
        name: group.name,
        kind: "User group",
        memberCount,
        targetLabel: memberCount > 0 ? formatCount(memberCount, "member") : "No members",
        description: group.description || "Directory or manually managed user group.",
        loginEnabled: group.login_enabled ?? true,
        geofenceRestrictionEnabled: Boolean(group.geofence_restriction_enabled),
        geofenceRestrictionRadius: group.geofence_restriction_radius,
        primaryDeviceRestrictionEnabled: Boolean(group.primary_device_restriction_enabled),
        managedDeviceRestrictionEnabled: Boolean(group.managed_device_restriction_enabled),
        readerRestrictionEnabled: Boolean(group.reader_restriction_enabled),
        timeRestrictionEnabled: Boolean(group.time_restriction_enabled),
        tapToAccessRestrictionEnabled: Boolean(group.tap_to_access_restriction_enabled),
        timeRestrictionTimeZone: group.time_restriction_time_zone,
        statusLabel: "Enabled",
        tone: "success",
      } satisfies KisiGroupResource
    }),
    ...doorGroups.map((group) => {
      const relationDoorIDs = groupLocks.filter((lock) => lock.group_id === group.id).map((lock) => lock.lock_id)
      const doorIDs = groupLocksResult.status === "fulfilled" ? relationDoorIDs : group.door_ids ?? []
      const groupDoors = doorIDs.map((doorID) => doorsByID.get(doorID)).filter((door): door is Door => Boolean(door))
      const relationZones = groupZones.filter((zone) => zone.group_id === group.id)
      const fallbackZoneIDs = Array.from(new Set(groupDoors.map((door) => door.area_id).filter(Boolean)))
      const zoneIDs = groupZonesResult.status === "fulfilled" ? relationZones.map((zone) => zone.zone_id) : fallbackZoneIDs
      const zoneFloorIDs = groupZonesResult.status === "fulfilled" ? relationZones.map((zone) => zone.floor_id) : fallbackZoneIDs.map((zoneID) => areasByID.get(zoneID)?.floor_id ?? "").filter(Boolean)
      const placeIDs = Array.from(new Set(groupDoors.map((door) => door.building_id).filter(Boolean)))
      const placeId = placeIDs.length === 1 ? placeIDs[0] : ""
      return {
        id: group.id,
        tenantID: group.tenant_id,
        placeId,
        name: group.name,
        kind: "Door group",
        memberCount: groupDoors.length,
        targetLabel: compactListSummary(
          groupDoors.map((door) => door.name),
          "No doors"
        ),
        description: placeId ? `${placesByID.get(placeId)?.name ?? "Place"} door group.` : "Door group spanning multiple places.",
        doorIds: groupDoors.map((door) => door.id),
        zoneIds: Array.from(new Set(zoneIDs)),
        zoneFloorIds: Array.from(new Set(zoneFloorIDs)),
        statusLabel: groupDoors.length > 0 ? "Enabled" : "Review",
        tone: groupDoors.length > 0 ? "success" : "warning",
      } satisfies KisiGroupResource
    }),
  ].sort((left, right) => left.name.localeCompare(right.name))

  const referenceAccessRights = buildKisiAccessRightsFromReferenceResources({
    roleAssignments,
    shares,
    roles,
    places,
    groups: groupResources,
    teams: teamResources,
    users: userResources,
    doors,
  }).filter((right) => !right.placeId || buildingIDs.has(right.placeId))

  const fallbackAccessRights = [
    ...accessPolicies.map((policy) => {
      return {
        id: policy.id,
        placeId: placeIDForAccessScope(policy, areasByID, doorsByID),
        name: policy.name,
        subjectType: "Group",
        targetLabel: targetLabelForAccessScope(policy, placesByID, areasByID, doorsByID),
        ruleLabel: policy.schedule || "Always",
        statusLabel: policyStatusLabel(policy.status),
        tone: policyTone(policy.status),
      } satisfies KisiAccessRightResource
    }),
    ...temporaryAccess.map((grant) => {
      const status = temporaryAccessStatus(grant)
      return {
        id: grant.id,
        placeId: placeIDForAccessScope(grant, areasByID, doorsByID),
        name: grant.grantee_name || grant.grantee_email || "Access link",
        subjectType: "Access Link",
        targetLabel: targetLabelForAccessScope(grant, placesByID, areasByID, doorsByID),
        ruleLabel: `Expires ${formatDateLabel(grant.valid_until)}`,
        statusLabel: status.statusLabel,
        tone: status.tone,
      } satisfies KisiAccessRightResource
    }),
  ].sort((left, right) => left.name.localeCompare(right.name))

  const accessRights = [...referenceAccessRights, ...fallbackAccessRights].sort((left, right) => left.name.localeCompare(right.name))

  return {
    places,
    floors: floorResources,
    zones: zoneResources,
    doors: doorResources,
    hardware: hardwareResources,
    events: eventResources,
    users: userResources,
    credentials,
    groups: groupResources,
    teams: teamResources,
    teamMemberships: teamMembershipResources,
    accessRights,
    partial:
      isRejected(buildingsResult) ||
      isRejected(floorsResult) ||
      isRejected(areasResult) ||
      isRejected(doorsResult) ||
      (useGatewayFallback && isRejected(gatewaysResult)) ||
      (isRejected(eventSetResult) && isRejected(eventsResult)) ||
      isRejected(usersResult) ||
      (isRejected(cardsResult) && isRejected(walletPassesResult)) ||
      isRejected(cardAssignmentsResult) ||
      isRejected(userGroupsResult) ||
      isRejected(doorGroupsResult) ||
      isRejected(groupLocksResult) ||
      isRejected(groupZonesResult) ||
      isRejected(teamsResult) ||
      isRejected(teamMembershipsResult) ||
      (isRejected(roleAssignmentsResult) && isRejected(accessPoliciesResult)) ||
      (isRejected(sharesResult) && isRejected(temporaryAccessResult)),
  }
}

export function selectKisiPlaceContext(summary: KisiResourceSummary, placeID?: string | null): KisiPlaceContext {
  const requestedID = placeID?.trim()
  const requestedSlug = requestedID ? slugify(requestedID) : ""
  const place =
    summary.places.find((item) => item.id === requestedID) ??
    summary.places.find((item) => slugify(item.name) === requestedSlug) ??
    (requestedID ? null : summary.places[0]) ??
    null

  if (!place) {
    return {
      place: null,
      floors: [],
      zones: [],
      doors: [],
      hardware: [],
      events: [],
      users: [],
      groups: [],
      teams: [],
      teamMemberships: [],
      accessRights: [],
    }
  }

  return {
    place,
    floors: summary.floors.filter((floor) => floor.placeId === place.id),
    zones: summary.zones.filter((zone) => zone.placeId === place.id),
    doors: summary.doors.filter((door) => door.placeId === place.id),
    hardware: summary.hardware.filter((item) => item.placeId === place.id),
    events: summary.events.filter((event) => event.placeId === place.id),
    users: summary.users.filter((user) => user.placeId === place.id),
    groups: summary.groups.filter((group) => !group.placeId || group.placeId === place.id),
    teams: summary.teams.filter((team) => !team.placeId || team.placeId === place.id),
    teamMemberships: summary.teamMemberships.filter((membership) => !membership.placeId || membership.placeId === place.id),
    accessRights: summary.accessRights.filter((accessRight) => !accessRight.placeId || accessRight.placeId === place.id),
  }
}

export function summarizePlaceCounts(place: KisiPlaceResource) {
  return `${formatCount(place.doorCount, "door")} - ${formatCount(place.gatewayCount, "gateway")}`
}
