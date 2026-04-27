import i18n from "@/lib/i18n"
import type { EnterpriseEmployee } from "@/lib/api"

import type { AccessSection } from "./access-sections-tabs"

export type ScopeType = "all" | "building" | "area" | "door"
export type DeliveryMethod = "email_qr" | "wallet"
export type GrantLifecycleStatus = "active" | "expiring_soon" | "expired"
export type PositionTemplateSpec = {
  position: string
  defaultGroup: string
  accessRole: string
  permissionPreset: string
}

function t(key: string, defaultValue: string, options?: Record<string, unknown>) {
  return i18n.t(key, {
    defaultValue,
    ...options,
  })
}

export function policyStatusLabel(status: string) {
  switch (status) {
    case "active":
      return t("accessPage.components.utils.policyStatus.active", "Active")
    case "inactive":
      return t("accessPage.components.utils.policyStatus.inactive", "Inactive")
    case "draft":
      return t("accessPage.components.utils.policyStatus.draft", "Draft")
    default:
      return status
  }
}

export function deliveryLabel(method: string) {
  switch (method) {
    case "wallet":
      return t("accessPage.components.utils.delivery.wallet", "Mistyislet mobile pass")
    case "email_qr":
      return t("accessPage.components.utils.delivery.emailQr", "Email QR pass")
    default:
      return method
  }
}

export function parseDateTime(value: string): Date | null {
  const raw = value.trim()
  if (!raw) {
    return null
  }
  const direct = new Date(raw)
  if (!Number.isNaN(direct.getTime())) {
    return direct
  }
  const normalized = raw.includes(" ") ? raw.replace(" ", "T") : raw
  const fallback = new Date(normalized)
  if (!Number.isNaN(fallback.getTime())) {
    return fallback
  }
  return null
}

export function remainingLabel(validUntil: string, now: number): string {
	const expiresAt = parseDateTime(validUntil)
	if (!expiresAt) {
		return t("accessPage.components.utils.remaining.invalidTime", "Invalid time format")
	}
	const deltaMs = expiresAt.getTime() - now
	if (deltaMs <= 0) {
		return t("accessPage.components.utils.remaining.expired", "Expired")
	}
	const totalMinutes = Math.floor(deltaMs / 60000)
	const days = Math.floor(totalMinutes / 1440)
	const hours = Math.floor((totalMinutes % 1440) / 60)
	const minutes = totalMinutes % 60
	if (days > 0) {
		return t("accessPage.components.utils.remaining.days", "{{days}}d {{hours}}h {{minutes}}m", {
			days,
			hours,
			minutes,
		})
	}
	return t("accessPage.components.utils.remaining.hours", "{{hours}}h {{minutes}}m", {
		hours,
		minutes,
	})
}

export function isGrantExpired(validUntil: string, now: number): boolean {
  const expiresAt = parseDateTime(validUntil)
  if (!expiresAt) {
    return false
  }
  return expiresAt.getTime() <= now
}

export function getGrantLifecycleStatus(validUntil: string, now: number): GrantLifecycleStatus {
  const expiresAt = parseDateTime(validUntil)
  if (!expiresAt) {
    return "active"
  }
  const deltaMs = expiresAt.getTime() - now
  if (deltaMs <= 0) {
    return "expired"
  }
  if (deltaMs <= 24 * 60 * 60 * 1000) {
    return "expiring_soon"
  }
  return "active"
}

export function grantLifecycleLabel(status: GrantLifecycleStatus): string {
  switch (status) {
    case "expired":
      return t("accessPage.components.utils.grantLifecycle.expired", "Expired")
    case "expiring_soon":
      return t("accessPage.components.utils.grantLifecycle.expiringSoon", "Expiring within 24h")
    default:
      return t("accessPage.components.utils.grantLifecycle.active", "Active")
  }
}

export function grantLifecycleBadgeVariant(status: GrantLifecycleStatus): "outline" | "secondary" | "destructive" {
  switch (status) {
    case "expired":
      return "destructive"
    case "expiring_soon":
      return "secondary"
    default:
      return "outline"
  }
}

export function formatDateTimeLocalInput(value: Date): string {
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, "0")
  const day = String(value.getDate()).padStart(2, "0")
  const hours = String(value.getHours()).padStart(2, "0")
  const minutes = String(value.getMinutes()).padStart(2, "0")
  return `${year}-${month}-${day}T${hours}:${minutes}`
}

export function scopeSummary(
  scopeType: string,
  buildingName?: string,
  areaName?: string,
  doorName?: string
): string {
  if (scopeType === "all") {
    return t("accessPage.components.utils.scopeSummary.all", "All areas")
  }
  if (scopeType === "building") {
    return t("accessPage.components.utils.scopeSummary.building", "Building: {{building}}", {
      building: buildingName || "-",
    })
  }
  if (scopeType === "area") {
    return t("accessPage.components.utils.scopeSummary.area", "Building: {{building}} / Area: {{area}}", {
      building: buildingName || "-",
      area: areaName || "-",
    })
  }
  if (scopeType === "door") {
    return t(
      "accessPage.components.utils.scopeSummary.door",
      "Building: {{building}} / Area: {{area}} / Door: {{door}}",
      {
        building: buildingName || "-",
        area: areaName || "-",
        door: doorName || "-",
      }
    )
  }
  return scopeType
}

export const positionTemplateSpec: PositionTemplateSpec[] = [
  {
    position: "Security / Satpam / Guard",
    defaultGroup: "Factory Security",
    accessRole: "operator",
    permissionPreset: t(
      "accessPage.components.utils.positionTemplate.security.permissionPreset",
      "Alarm handling + gate patrol + priority access for critical doors"
    ),
  },
  {
    position: "Facility / Engineering / Building",
    defaultGroup: "Building Operations",
    accessRole: "building_admin",
    permissionPreset: t(
      "accessPage.components.utils.positionTemplate.facility.permissionPreset",
      "Building-level configuration release + gateway operations + shared facility area access"
    ),
  },
  {
    position: "IT / Identity / Admin",
    defaultGroup: "Tenant Platform Admin",
    accessRole: "tenant_admin",
    permissionPreset: t(
      "accessPage.components.utils.positionTemplate.admin.permissionPreset",
      "Tenant-level policy and group management + all-area authorization"
    ),
  },
  {
    position: "General Employee",
    defaultGroup: "Common Office Access",
    accessRole: "resident",
    permissionPreset: t(
      "accessPage.components.utils.positionTemplate.employee.permissionPreset",
      "Default office/common-area access without per-person assignment"
    ),
  },
]

export function inferStarterGroupMemberIDs(template: PositionTemplateSpec, employees: EnterpriseEmployee[]): string[] {
  const normalizedAccessRole = template.accessRole.trim().toLowerCase()
  const keywords = template.position
    .split("/")
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean)

  return employees
    .filter((employee) => employee.status === "active")
    .filter((employee) => {
      const employeeAccessRole = (employee.access_role || "").trim().toLowerCase()
      if (normalizedAccessRole === "resident") {
        return !["tenant_admin", "building_admin", "operator"].includes(employeeAccessRole)
      }
      if (employeeAccessRole === normalizedAccessRole) {
        return true
      }
      const haystack = `${employee.job_title} ${employee.department} ${employee.access_role} ${employee.location}`.toLowerCase()
      return keywords.some((keyword) => haystack.includes(keyword))
    })
    .map((employee) => employee.id)
}

export function inferPositionTemplateByGroupName(groupName: string): PositionTemplateSpec | null {
  const haystack = groupName.trim().toLowerCase()
  if (!haystack) {
    return null
  }
  for (const item of positionTemplateSpec) {
    if (item.defaultGroup.trim().toLowerCase() === haystack) {
      return item
    }
    const keywords = item.position
      .split("/")
      .map((keyword) => keyword.trim().toLowerCase())
      .filter(Boolean)
    if (keywords.some((keyword) => haystack.includes(keyword))) {
      return item
    }
  }
  return null
}

export function inferPolicyStarterScope(
  accessRole: string,
  buildingID: string,
  areaID: string,
  doorID: string
): ScopeType {
  switch (accessRole.trim().toLowerCase()) {
    case "tenant_admin":
      return "all"
    case "building_admin":
      return buildingID ? "building" : "all"
    case "operator":
      if (doorID) {
        return "door"
      }
      if (areaID) {
        return "area"
      }
      if (buildingID) {
        return "building"
      }
      return "all"
    default:
      if (areaID) {
        return "area"
      }
      if (buildingID) {
        return "building"
      }
      return "all"
  }
}

export function inferPolicyStarterName(groupName: string, accessRole: string): string {
  switch (accessRole.trim().toLowerCase()) {
    case "tenant_admin":
      return t("accessPage.components.utils.policyStarterName.tenantAdmin", "{{groupName}} all-area management policy", {
        groupName,
      })
    case "building_admin":
      return t("accessPage.components.utils.policyStarterName.buildingAdmin", "{{groupName}} building operations policy", {
        groupName,
      })
    case "operator":
      return t(
        "accessPage.components.utils.policyStarterName.operator",
        "{{groupName}} patrol and critical-door policy",
        {
          groupName,
        }
      )
    default:
      return t("accessPage.components.utils.policyStarterName.default", "{{groupName}} office access policy", {
        groupName,
      })
  }
}

export function inferPolicyStarterSchedule(accessRole: string): string {
  switch (accessRole.trim().toLowerCase()) {
    case "tenant_admin":
      return "Mon-Sun 00:00-23:59"
    case "building_admin":
      return "Mon-Sun 06:00-22:00"
    case "operator":
      return "Mon-Sun 00:00-23:59"
    default:
      return "Mon-Fri 07:00-20:00"
  }
}

export function resolveAccessSection(value?: string): AccessSection {
  switch (value) {
    case "directory":
    case "policies":
    case "grants":
      return value
    default:
      return "directory"
  }
}

export function isAccessSection(value?: string): value is AccessSection {
  return value === "directory" || value === "policies" || value === "grants"
}

export function sectionFromAccessPath(pathname: string): string | undefined {
  if (!pathname.startsWith("/access")) {
    return undefined
  }
  const suffix = pathname.slice("/access".length)
  if (suffix === "" || suffix === "/") {
    return undefined
  }
  if (!suffix.startsWith("/")) {
    return undefined
  }
  const segment = suffix.slice(1).split("/")[0]
  return segment || undefined
}

export function enterpriseFlowStageLabel(stage?: string): string {
  switch ((stage || "").trim()) {
    case "directory":
      return t(
        "accessPage.components.utils.enterpriseFlowStage.directory",
        "Sync result has been carried into employees and user groups"
      )
    case "policies":
      return t("accessPage.components.utils.enterpriseFlowStage.policies", "Group result has been carried into access policies")
    case "issuance":
      return t("accessPage.components.utils.enterpriseFlowStage.issuance", "Policy result has been carried into pass issuance")
    default:
      return t("accessPage.components.utils.enterpriseFlowStage.default", "Carried from enterprise main flow")
  }
}

export function enterpriseFlowSegmentLabel(segmentHint?: string): string {
  switch ((segmentHint || "").trim()) {
    case "directory_usage":
      return t("accessPage.components.utils.enterpriseFlowSegment.directoryUsage", "Sync result to group usage")
    case "policy_delivery":
      return t("accessPage.components.utils.enterpriseFlowSegment.policyDelivery", "Group usage to policy delivery")
    default:
      return ""
  }
}

export function enterpriseFlowSegmentStatusLabel(statusHint?: string): string {
  switch ((statusHint || "").trim()) {
    case "ready":
      return t("accessPage.components.utils.enterpriseFlowSegmentStatus.ready", "Ready")
    case "attention":
      return t("accessPage.components.utils.enterpriseFlowSegmentStatus.attention", "Needs closure")
    case "pending":
      return t("accessPage.components.utils.enterpriseFlowSegmentStatus.pending", "Pending")
    default:
      return ""
  }
}

export function validateScope(scopeType: ScopeType, buildingID: string, areaID: string, doorID: string): string {
  if (scopeType === "all") {
    return ""
  }
  if (scopeType === "building" && !buildingID) {
    return t(
      "accessPage.components.utils.validateScope.buildingRequired",
      "Select a building before saving building-level authorization"
    )
  }
  if (scopeType === "area" && (!buildingID || !areaID)) {
    return t("accessPage.components.utils.validateScope.areaRequired", "Area scope requires both building and area")
  }
  if (scopeType === "door" && (!buildingID || !areaID || !doorID)) {
    return t("accessPage.components.utils.validateScope.doorRequired", "Door scope requires building, area, and door")
  }
  return ""
}
