import type { CurrentUser, UserRole } from "@/lib/api"
import i18n from "@/lib/i18n"

export type ViewerCapability =
  | "platform.tenants.manage"
  | "platform.audit.view"
  | "tenant.directory.manage"
  | "tenant.access.manage"
  | "tenant.credentials.manage"
  | "site.spaces.manage"
  | "site.gateways.manage"
  | "ops.gateways.view"
  | "ops.gatewayInventory.view"
  | "ops.gatewayInventory.edit"
  | "ops.events.view"
  | "ops.alarms.handle"
  | "ops.wallet.view"
  | "enterprise.sync.health.view"

const ROLE_CAPABILITIES: Record<UserRole, readonly ViewerCapability[]> = {
  super_admin: [
    "platform.tenants.manage",
    "platform.audit.view",
    "tenant.directory.manage",
    "tenant.access.manage",
    "tenant.credentials.manage",
    "site.spaces.manage",
    "site.gateways.manage",
    "ops.gateways.view",
    "ops.gatewayInventory.view",
    "ops.gatewayInventory.edit",
    "ops.events.view",
    "ops.alarms.handle",
    "ops.wallet.view",
    "enterprise.sync.health.view",
  ],
  tenant_admin: [
    "tenant.directory.manage",
    "tenant.access.manage",
    "tenant.credentials.manage",
    "site.spaces.manage",
    "site.gateways.manage",
    "ops.gateways.view",
    "ops.gatewayInventory.view",
    "ops.gatewayInventory.edit",
    "ops.events.view",
    "ops.alarms.handle",
    "ops.wallet.view",
    "enterprise.sync.health.view",
  ],
  operator: [
    "ops.gateways.view",
    "ops.gatewayInventory.view",
    "ops.events.view",
    "ops.alarms.handle",
    "ops.wallet.view",
  ],
  building_admin: [
    "site.spaces.manage",
    "site.gateways.manage",
    "ops.gateways.view",
    "ops.events.view",
    "ops.alarms.handle",
  ],
  resident: [],
}

const ROLE_CAPABILITY_SETS: Record<UserRole, ReadonlySet<ViewerCapability>> = {
  super_admin: new Set(ROLE_CAPABILITIES.super_admin),
  tenant_admin: new Set(ROLE_CAPABILITIES.tenant_admin),
  operator: new Set(ROLE_CAPABILITIES.operator),
  building_admin: new Set(ROLE_CAPABILITIES.building_admin),
  resident: new Set(ROLE_CAPABILITIES.resident),
}

export function hasViewerCapability(viewer: CurrentUser, capability: ViewerCapability): boolean {
  return ROLE_CAPABILITY_SETS[viewer.role]?.has(capability) ?? false
}

export function getViewerCapabilities(viewer: CurrentUser): readonly ViewerCapability[] {
  return ROLE_CAPABILITIES[viewer.role] ?? []
}

export function isPlatformViewer(viewer: CurrentUser): boolean {
  return viewer.role === "super_admin"
}

export function isBuildingAdmin(viewer: CurrentUser): boolean {
  return viewer.role === "building_admin"
}

export function isTenantScopedViewer(viewer: CurrentUser): boolean {
  return !isPlatformViewer(viewer)
}

export function getViewerTenantID(viewer: CurrentUser): string {
  return viewer.tenant_id?.trim() ?? ""
}

export function getViewerBuildingIDs(viewer: CurrentUser): string[] {
  return Array.from(
    new Set(
      (viewer.building_ids ?? [])
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

export function canAccessEnterprisePage(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "tenant.directory.manage")
}

export function canManageEnterprise(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "tenant.directory.manage")
}

export function canAccessSpacesPage(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "site.spaces.manage")
}

export function canCreateBuildings(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin"].includes(viewer.role)
}

export function canManageScopedSpaces(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "site.spaces.manage")
}

export function canAccessAccessPage(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "tenant.access.manage")
}

export function canAccessIssuancePage(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "tenant.credentials.manage") || hasViewerCapability(viewer, "ops.wallet.view")
}

export function canManageIssuance(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "tenant.credentials.manage")
}

export function canAccessAlarmsPage(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "ops.alarms.handle")
}

export function canAccessAuditPage(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "platform.audit.view")
}

export function canAccessGatewaysPage(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "ops.gateways.view")
}

export function canManageGateways(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "site.gateways.manage")
}

export function canRegisterGateways(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin"].includes(viewer.role)
}

export function canAccessGatewayInventory(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "ops.gatewayInventory.view")
}

export function canEditGatewayInventory(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "ops.gatewayInventory.edit")
}

export function canAccessEventsPage(viewer: CurrentUser): boolean {
  return hasViewerCapability(viewer, "ops.events.view")
}

export function getViewerRoleLabel(viewer: CurrentUser): string {
  switch (viewer.role) {
    case "super_admin":
      return i18n.t("viewer.role.superAdmin", { defaultValue: "Organization Admin" })
    case "tenant_admin":
      return i18n.t("viewer.role.tenantAdmin", { defaultValue: "Organization Admin" })
    case "operator":
      return i18n.t("viewer.role.operator", { defaultValue: "Place Admin" })
    case "building_admin":
      return i18n.t("viewer.role.buildingAdmin", { defaultValue: "Place Admin" })
    case "resident":
      return i18n.t("viewer.role.resident", { defaultValue: "Unsupported account" })
    default:
      return viewer.role
  }
}
