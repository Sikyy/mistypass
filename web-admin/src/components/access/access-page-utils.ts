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

export function policyStatusLabel(status: string) {
  switch (status) {
    case "active":
      return "启用"
    case "inactive":
      return "停用"
    case "draft":
      return "草稿"
    default:
      return status
  }
}

export function deliveryLabel(method: string) {
  switch (method) {
    case "wallet":
      return "MistyPass 移动凭证"
    case "email_qr":
      return "邮件二维码凭证"
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
    return "时间格式异常"
  }
  const deltaMs = expiresAt.getTime() - now
  if (deltaMs <= 0) {
    return "已到期"
  }
  const totalSeconds = Math.floor(deltaMs / 1000)
  const days = Math.floor(totalSeconds / 86400)
  const hours = Math.floor((totalSeconds % 86400) / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  if (days > 0) {
    return `${days}天 ${hours}时 ${minutes}分`
  }
  return `${hours}时 ${minutes}分 ${seconds}秒`
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
      return "已到期"
    case "expiring_soon":
      return "24 小时内到期"
    default:
      return "当前有效"
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
    return "全部区域"
  }
  if (scopeType === "building") {
    return `楼宇：${buildingName || "-"}`
  }
  if (scopeType === "area") {
    return `楼宇：${buildingName || "-"} / 区域：${areaName || "-"}`
  }
  if (scopeType === "door") {
    return `楼宇：${buildingName || "-"} / 区域：${areaName || "-"} / 门点：${doorName || "-"}`
  }
  return scopeType
}

export const positionTemplateSpec: PositionTemplateSpec[] = [
  {
    position: "Security / Satpam / Guard",
    defaultGroup: "Factory Security",
    accessRole: "operator",
    permissionPreset: "告警处置 + 门禁巡检 + 关键门点优先通行",
  },
  {
    position: "Facility / Engineering / Building",
    defaultGroup: "Building Operations",
    accessRole: "building_admin",
    permissionPreset: "楼宇级配置发布 + 网关运维 + 公共设备区访问",
  },
  {
    position: "IT / Identity / Admin",
    defaultGroup: "Tenant Platform Admin",
    accessRole: "tenant_admin",
    permissionPreset: "租户级策略与用户组管理 + 全部区域授权",
  },
  {
    position: "General Employee",
    defaultGroup: "Common Office Access",
    accessRole: "resident",
    permissionPreset: "办公区/公共区默认通行，无需逐人分配",
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
      return `${groupName} 全域管理策略`
    case "building_admin":
      return `${groupName} 楼宇运维策略`
    case "operator":
      return `${groupName} 巡检与关键门点策略`
    default:
      return `${groupName} 办公通行策略`
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
      return "同步结果已承接到员工与用户组"
    case "policies":
      return "用户组结果已承接到权限策略"
    case "issuance":
      return "权限策略结果已承接到凭证发放"
    default:
      return "已承接企业页主路径"
  }
}

export function enterpriseFlowSegmentLabel(segmentHint?: string): string {
  switch ((segmentHint || "").trim()) {
    case "directory_usage":
      return "同步结果到用户组使用"
    case "policy_delivery":
      return "用户组使用到权限下发"
    default:
      return ""
  }
}

export function enterpriseFlowSegmentStatusLabel(statusHint?: string): string {
  switch ((statusHint || "").trim()) {
    case "ready":
      return "已承接"
    case "attention":
      return "待收口"
    case "pending":
      return "待补齐"
    default:
      return ""
  }
}

export function validateScope(scopeType: ScopeType, buildingID: string, areaID: string, doorID: string): string {
  if (scopeType === "all") {
    return ""
  }
  if (scopeType === "building" && !buildingID) {
    return "选择楼宇后才能保存楼宇范围授权"
  }
  if (scopeType === "area" && (!buildingID || !areaID)) {
    return "区域范围必须同时选择楼宇和区域"
  }
  if (scopeType === "door" && (!buildingID || !areaID || !doorID)) {
    return "门点范围必须同时选择楼宇、区域和门点"
  }
  return ""
}
