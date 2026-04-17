import type { CurrentUser } from "@/lib/api"

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
  return ["super_admin", "tenant_admin", "operator"].includes(viewer.role)
}

export function canManageEnterprise(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin"].includes(viewer.role)
}

export function canAccessSpacesPage(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin", "building_admin"].includes(viewer.role)
}

export function canCreateBuildings(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin"].includes(viewer.role)
}

export function canManageScopedSpaces(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin", "building_admin"].includes(viewer.role)
}

export function canAccessAccessPage(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin"].includes(viewer.role)
}

export function canAccessIssuancePage(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin", "operator"].includes(viewer.role)
}

export function canManageIssuance(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin"].includes(viewer.role)
}

export function canAccessAlarmsPage(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin", "operator", "building_admin"].includes(viewer.role)
}

export function canAccessGatewaysPage(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin", "operator", "building_admin"].includes(viewer.role)
}

export function canManageGateways(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin", "building_admin"].includes(viewer.role)
}

export function canRegisterGateways(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin"].includes(viewer.role)
}

export function canAccessGatewayInventory(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin", "operator"].includes(viewer.role)
}

export function canEditGatewayInventory(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin"].includes(viewer.role)
}

export function canAccessEventsPage(viewer: CurrentUser): boolean {
  return ["super_admin", "tenant_admin", "operator", "building_admin"].includes(viewer.role)
}

export function getViewerRoleLabel(viewer: CurrentUser): string {
  switch (viewer.role) {
    case "super_admin":
      return "平台管理员"
    case "tenant_admin":
      return "企业管理员"
    case "operator":
      return "值守人员"
    case "building_admin":
      return "楼宇管理员"
    default:
      return viewer.role
  }
}
