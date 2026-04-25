import type { TFunction } from "i18next"

import { type WalletPassInstance, type WalletPassTemplate, type WalletPhysicalCardTask } from "@/lib/api"

export type WalletScenarioKind = "employee_mobile" | "employee_physical" | "visitor_qr" | "visitor_temporary"

type WalletScenarioCounters = Record<WalletScenarioKind, number>

export type WalletIssuanceScenarioPreset = {
  id: WalletScenarioKind
  titleKey: string
  descriptionKey: string
  templateNameKey: string
  passType: "employee" | "visitor"
  classID: string
  styleConfig: Record<string, string>
  targetType: "user" | "visitor"
  recommendedExecutionMode: "inline" | "queued"
  defaultExpiresInHours?: number
}

export const walletIssuanceScenarioPresets: WalletIssuanceScenarioPreset[] = [
  {
    id: "employee_mobile",
    titleKey: "walletPage.scenarios.employeeMobile.title",
    descriptionKey: "walletPage.scenarios.employeeMobile.description",
    templateNameKey: "walletPage.scenarios.employeeMobile.templateName",
    passType: "employee",
    classID: "employee-mobile",
    styleConfig: {
      brand_color: "#0f766e",
      delivery_method: "wallet",
      dispatch_channels: "email,whatsapp",
      access_medium: "mobile_pass",
    },
    targetType: "user",
    recommendedExecutionMode: "queued",
  },
  {
    id: "employee_physical",
    titleKey: "walletPage.scenarios.employeePhysical.title",
    descriptionKey: "walletPage.scenarios.employeePhysical.description",
    templateNameKey: "walletPage.scenarios.employeePhysical.templateName",
    passType: "employee",
    classID: "employee-physical-card",
    styleConfig: {
      brand_color: "#155e75",
      delivery_method: "wallet",
      dispatch_channels: "email,whatsapp",
      access_medium: "physical_card",
      card_workflow: "enabled",
    },
    targetType: "user",
    recommendedExecutionMode: "queued",
  },
  {
    id: "visitor_qr",
    titleKey: "walletPage.scenarios.visitorQr.title",
    descriptionKey: "walletPage.scenarios.visitorQr.description",
    templateNameKey: "walletPage.scenarios.visitorQr.templateName",
    passType: "visitor",
    classID: "visitor-qr",
    styleConfig: {
      brand_color: "#1d4ed8",
      delivery_method: "email_qr",
      dispatch_channels: "email,whatsapp",
      access_medium: "qr_code",
      suggested_validity: "24h",
    },
    targetType: "visitor",
    recommendedExecutionMode: "inline",
    defaultExpiresInHours: 24,
  },
  {
    id: "visitor_temporary",
    titleKey: "walletPage.scenarios.visitorTemporary.title",
    descriptionKey: "walletPage.scenarios.visitorTemporary.description",
    templateNameKey: "walletPage.scenarios.visitorTemporary.templateName",
    passType: "visitor",
    classID: "visitor-temporary",
    styleConfig: {
      brand_color: "#b45309",
      delivery_method: "email_qr",
      dispatch_channels: "email",
      access_medium: "temporary_pass",
      suggested_validity: "8h",
    },
    targetType: "visitor",
    recommendedExecutionMode: "inline",
    defaultExpiresInHours: 8,
  },
]

export const walletIssuanceScenarioPresetByID = new Map(
  walletIssuanceScenarioPresets.map((item) => [item.id, item] as const)
)

export function parsePositiveInt(raw: string): number | undefined {
  const value = Number.parseInt(raw.trim(), 10)
  if (!Number.isFinite(value) || value <= 0) {
    return undefined
  }
  return value
}

export function parseNonNegativeInt(raw: string): number | undefined {
  const value = Number.parseInt(raw.trim(), 10)
  if (!Number.isFinite(value) || value < 0) {
    return undefined
  }
  return value
}

export function parseReceiverGroups(raw: string): string[] {
  const groups = raw
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
  return Array.from(new Set(groups))
}

export function parseTargetIDs(raw: string): string[] {
  return Array.from(
    new Set(
      raw
        .split(/[\n,;]+/g)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

export function parseReceiverValues(raw: string): string[] {
  return Array.from(
    new Set(
      raw
        .split(/[\n,;]+/g)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

export function parseStyleConfig(raw: string): Record<string, string> | undefined {
  const value = raw.trim()
  if (!value) {
    return undefined
  }

  if (value.startsWith("{")) {
    try {
      const payload = JSON.parse(value) as Record<string, unknown>
      const entries: Array<readonly [string, string]> = []
      for (const [key, item] of Object.entries(payload)) {
        if (!key.trim() || typeof item !== "string" || !item.trim()) {
          continue
        }
        entries.push([key.trim(), item.trim()] as const)
      }
      return entries.length > 0 ? Object.fromEntries(entries) : undefined
    } catch {
      return undefined
    }
  }

  const config: Record<string, string> = {}
  for (const line of value.split(/\n+/)) {
    const separatorIndex = line.includes("=") ? line.indexOf("=") : line.indexOf(":")
    if (separatorIndex <= 0) {
      continue
    }
    const key = line.slice(0, separatorIndex).trim()
    const item = line.slice(separatorIndex + 1).trim()
    if (key && item) {
      config[key] = item
    }
  }
  return Object.keys(config).length > 0 ? config : undefined
}

export function normalizeDateTimeInput(raw: string): string | undefined {
  const value = raw.trim()
  if (!value) {
    return undefined
  }
  const timestamp = new Date(value)
  return Number.isNaN(timestamp.getTime()) ? value : timestamp.toISOString()
}

export function formatDateTime(value?: string) {
  if (!value) {
    return "-"
  }
  const timestamp = new Date(value)
  if (Number.isNaN(timestamp.getTime())) {
    return value
  }
  return timestamp.toLocaleString()
}

export function formatDurationSeconds(seconds?: number) {
  if (!seconds || seconds <= 0) {
    return "-"
  }
  if (seconds % 3600 === 0) {
    return `${seconds / 3600}h`
  }
  if (seconds % 60 === 0) {
    return `${seconds / 60}m`
  }
  return `${seconds}s`
}

export function formatTimeLabel(value?: string) {
  if (!value) {
    return "-"
  }
  const timestamp = new Date(value)
  if (Number.isNaN(timestamp.getTime())) {
    return value
  }
  return timestamp.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
}

function formatDateTimeLocalInput(date: Date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, "0")
  const day = String(date.getDate()).padStart(2, "0")
  const hour = String(date.getHours()).padStart(2, "0")
  const minute = String(date.getMinutes()).padStart(2, "0")
  return `${year}-${month}-${day}T${hour}:${minute}`
}

export function buildRelativeDateTimeInput(hoursFromNow: number) {
  const next = new Date()
  next.setHours(next.getHours() + hoursFromNow)
  return formatDateTimeLocalInput(next)
}

export function stringifyStyleConfig(config: Record<string, string>) {
  return Object.entries(config)
    .map(([key, value]) => `${key}=${value}`)
    .join("\n")
}

export function templateStatusVariant(status: string) {
  return status === "active" ? "outline" : "secondary"
}

export function passStatusVariant(status: string) {
  switch (status) {
    case "active":
      return "outline"
    case "revoked":
      return "destructive"
    default:
      return "secondary"
  }
}

export function passStatusLabel(t: TFunction, status: string) {
  switch (status) {
    case "issued":
      return t("walletPage.status.pass.issued")
    case "active":
      return t("walletPage.status.pass.active")
    case "suspended":
      return t("walletPage.status.pass.suspended")
    case "revoked":
      return t("walletPage.status.pass.revoked")
    default:
      return status
  }
}

export function deliveryNotificationStatusVariant(status: string) {
  switch (status) {
    case "sent":
      return "secondary"
    case "failed":
      return "destructive"
    default:
      return "outline"
  }
}

export function deliveryNotificationStatusLabel(t: TFunction, status: string) {
  switch (status) {
    case "sent":
      return t("walletPage.status.delivery.sent")
    case "failed":
      return t("walletPage.status.delivery.failed")
    case "skipped":
      return t("walletPage.status.delivery.skipped")
    default:
      return status
  }
}

export function targetTypeLabel(t: TFunction, type: string) {
  switch (type) {
    case "user":
      return t("walletPage.labels.targetType.user")
    case "visitor":
      return t("walletPage.labels.targetType.visitor")
    default:
      return type
  }
}

export function passTypeLabel(t: TFunction, type: string) {
  switch (type) {
    case "employee":
      return t("walletPage.labels.passType.employee")
    case "visitor":
      return t("walletPage.labels.passType.visitor")
    default:
      return type
  }
}

export function physicalCardTaskTypeLabel(t: TFunction, type: string) {
  switch (type) {
    case "issue":
      return t("walletPage.labels.physicalTaskType.issue")
    case "reissue":
      return t("walletPage.labels.physicalTaskType.reissue")
    case "loss_report":
      return t("walletPage.labels.physicalTaskType.lossReport")
    default:
      return type
  }
}

export function physicalCardTaskStatusVariant(status: string) {
  switch (status) {
    case "issued":
    case "reported_lost":
      return "outline"
    case "cancelled":
      return "destructive"
    default:
      return "secondary"
  }
}

export function physicalCardTaskStatusLabel(t: TFunction, status: string) {
  switch (status) {
    case "queued":
      return t("walletPage.status.physicalTask.queued")
    case "printing":
      return t("walletPage.status.physicalTask.printing")
    case "ready":
      return t("walletPage.status.physicalTask.ready")
    case "issued":
      return t("walletPage.status.physicalTask.issued")
    case "reported_lost":
      return t("walletPage.status.physicalTask.reportedLost")
    case "cancelled":
      return t("walletPage.status.physicalTask.cancelled")
    default:
      return status
  }
}

export function nextPhysicalCardTaskActions(t: TFunction, task: WalletPhysicalCardTask) {
  switch (task.task_type) {
    case "issue":
    case "reissue":
      switch (task.status) {
        case "queued":
          return [
            { status: "printing", label: t("walletPage.actions.physicalTask.startPrinting") },
            { status: "ready", label: t("walletPage.actions.physicalTask.ready") },
            { status: "issued", label: t("walletPage.actions.physicalTask.issueDirectly") },
            { status: "cancelled", label: t("walletPage.actions.physicalTask.cancel") },
          ] as const
        case "printing":
          return [
            { status: "ready", label: t("walletPage.actions.physicalTask.ready") },
            { status: "issued", label: t("walletPage.actions.physicalTask.issueDirectly") },
            { status: "cancelled", label: t("walletPage.actions.physicalTask.cancel") },
          ] as const
        case "ready":
          return [
            { status: "issued", label: t("walletPage.actions.physicalTask.markIssued") },
            { status: "cancelled", label: t("walletPage.actions.physicalTask.cancel") },
          ] as const
        default:
          return [] as const
      }
    case "loss_report":
      if (task.status === "queued") {
        return [
          { status: "reported_lost", label: t("walletPage.actions.physicalTask.confirmLoss") },
          { status: "cancelled", label: t("walletPage.actions.physicalTask.cancel") },
        ] as const
      }
      return [] as const
    default:
      return [] as const
  }
}

export function createWalletScenarioCounters(): WalletScenarioCounters {
  return {
    employee_mobile: 0,
    employee_physical: 0,
    visitor_qr: 0,
    visitor_temporary: 0,
  }
}

function getTemplateScenarioSearchText(template: Pick<WalletPassTemplate, "pass_type" | "name" | "class_id" | "style_config">) {
  return [
    template.pass_type,
    template.name,
    template.class_id,
    ...Object.keys(template.style_config ?? {}),
    ...Object.values(template.style_config ?? {}),
  ]
    .join(" ")
    .toLowerCase()
}

export function inferTemplateScenario(
  template: Pick<WalletPassTemplate, "pass_type" | "name" | "class_id" | "style_config">
): WalletScenarioKind {
  const haystack = getTemplateScenarioSearchText(template)
  if (template.pass_type === "employee") {
    if (
      haystack.includes("physical") ||
      haystack.includes("\u5b9e\u4f53") ||
      haystack.includes("card_workflow")
    ) {
      return "employee_physical"
    }
    return "employee_mobile"
  }
  if (haystack.includes("temporary") || haystack.includes("\u4e34\u65f6")) {
    return "visitor_temporary"
  }
  return "visitor_qr"
}

export function inferPassScenario(pass: WalletPassInstance, template?: WalletPassTemplate): WalletScenarioKind {
  if (template) {
    return inferTemplateScenario(template)
  }
  return pass.target_type === "visitor" ? "visitor_qr" : "employee_mobile"
}

export function walletScenarioLabel(t: TFunction, kind: WalletScenarioKind) {
  const preset = walletIssuanceScenarioPresetByID.get(kind)
  return preset ? t(preset.titleKey) : kind
}

export function walletScenarioHint(t: TFunction, kind: WalletScenarioKind) {
  switch (kind) {
    case "employee_mobile":
      return t("walletPage.hints.scenario.employeeMobile")
    case "employee_physical":
      return t("walletPage.hints.scenario.employeePhysical")
    case "visitor_qr":
      return t("walletPage.hints.scenario.visitorQr")
    case "visitor_temporary":
      return t("walletPage.hints.scenario.visitorTemporary")
    default:
      return ""
  }
}

export function deliveryMethodLabel(t: TFunction, method?: string) {
  switch (method) {
    case "email_qr":
      return t("walletPage.labels.deliveryMethod.emailQr")
    case "wallet":
      return t("walletPage.labels.deliveryMethod.walletLink")
    default:
      return method || "-"
  }
}

export function accessMediumLabel(t: TFunction, medium?: string) {
  switch (medium) {
    case "mobile_pass":
      return t("walletPage.labels.accessMedium.mobilePass")
    case "physical_card":
      return t("walletPage.labels.accessMedium.physicalCard")
    case "qr_code":
      return t("walletPage.labels.accessMedium.qrCode")
    case "temporary_pass":
      return t("walletPage.labels.accessMedium.temporaryPass")
    default:
      return medium || "-"
  }
}

function getTemplateStyleValue(template: WalletPassTemplate | undefined, key: string) {
  return template?.style_config?.[key]?.trim() ?? ""
}

export function getTemplateDeliveryMethod(template: WalletPassTemplate | undefined) {
  const configured = getTemplateStyleValue(template, "delivery_method")
  if (configured) {
    return configured
  }
  if (!template) {
    return ""
  }
  return template.pass_type === "visitor" ? "email_qr" : "wallet"
}

export function getTemplateAccessMedium(template: WalletPassTemplate | undefined) {
  const configured = getTemplateStyleValue(template, "access_medium")
  if (configured) {
    return configured
  }
  if (!template) {
    return ""
  }
  switch (inferTemplateScenario(template)) {
    case "employee_physical":
      return "physical_card"
    case "visitor_qr":
      return "qr_code"
    case "visitor_temporary":
      return "temporary_pass"
    default:
      return "mobile_pass"
  }
}

function getTemplateDispatchChannels(template: WalletPassTemplate | undefined) {
  const configured = getTemplateStyleValue(template, "dispatch_channels")
  if (configured) {
    return Array.from(
      new Set(
        configured
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean)
      )
    )
  }
  if (!template) {
    return []
  }
  return getTemplateDeliveryMethod(template) === "email_qr" ? ["email"] : ["email", "whatsapp"]
}

export function dispatchChannelLabels(t: TFunction, template: WalletPassTemplate | undefined) {
  const channels = getTemplateDispatchChannels(template)
  if (channels.length === 0) {
    return "-"
  }
  return channels
    .map((item) => {
      switch (item) {
        case "email":
          return "Email"
        case "whatsapp":
          return "WhatsApp"
        default:
          return item
      }
    })
    .join(" / ")
}

export function deliveryHint(t: TFunction, pass: WalletPassInstance, template: WalletPassTemplate | undefined) {
  const scenario = inferPassScenario(pass, template)
  if (scenario === "employee_physical") {
    return t("walletPage.hints.delivery.employeePhysical")
  }
  if (pass.save_link) {
    return t("walletPage.hints.delivery.hasSaveLink")
  }
  if (scenario === "visitor_temporary") {
    return t("walletPage.hints.delivery.visitorTemporary")
  }
  return t("walletPage.hints.delivery.pending")
}

export function enterpriseWalletStageLabel(t: TFunction, stage?: string): string {
  switch ((stage || "").trim()) {
    case "issuance":
      return t("walletPage.enterprise.stage.issuance")
    case "policies":
      return t("walletPage.enterprise.stage.policies")
    case "directory":
      return t("walletPage.enterprise.stage.directory")
    default:
      return t("walletPage.enterprise.stage.default")
  }
}

export function enterpriseWalletSegmentLabel(t: TFunction, segmentHint?: string): string {
  switch ((segmentHint || "").trim()) {
    case "directory_usage":
      return t("walletPage.enterprise.segment.directoryUsage")
    case "policy_delivery":
      return t("walletPage.enterprise.segment.policyDelivery")
    case "issuance_receipt":
      return t("walletPage.enterprise.segment.issuanceReceipt")
    case "receipt_recovery":
      return t("walletPage.enterprise.segment.receiptRecovery")
    default:
      return ""
  }
}

export function enterpriseWalletSegmentStatusLabel(t: TFunction, statusHint?: string): string {
  switch ((statusHint || "").trim()) {
    case "ready":
      return t("walletPage.enterprise.segmentStatus.ready")
    case "attention":
      return t("walletPage.enterprise.segmentStatus.attention")
    case "pending":
      return t("walletPage.enterprise.segmentStatus.pending")
    default:
      return ""
  }
}

export type ReceiptRecoveryActionHint = "retry_delivery" | "repair_pass_status" | "review_closed"

export function resolveReceiptRecoveryActionHint(value?: string): ReceiptRecoveryActionHint | "" {
  const normalized = (value || "").trim()
  if (normalized === "retry_delivery" || normalized === "repair_pass_status" || normalized === "review_closed") {
    return normalized
  }
  return ""
}

export function receiptRecoveryActionHintLabel(t: TFunction, actionHint: ReceiptRecoveryActionHint | ""): string {
  switch (actionHint) {
    case "retry_delivery":
      return t("walletPage.enterprise.receiptRecoveryAction.retryDelivery")
    case "repair_pass_status":
      return t("walletPage.enterprise.receiptRecoveryAction.repairPassStatus")
    case "review_closed":
      return t("walletPage.enterprise.receiptRecoveryAction.reviewClosed")
    default:
      return ""
  }
}

export type EnterpriseFlowTargetHints = {
  targetID: string
  targetEmail: string
  targetName: string
  targetHint: string
}

function normalizeEnterpriseTargetHintAsQuery(targetHint?: string): string {
  const normalized = (targetHint || "").trim()
  if (!normalized || normalized === "user" || normalized === "visitor") {
    return ""
  }
  return normalized
}

export function resolveEnterpriseTargetQuery(hints?: EnterpriseFlowTargetHints | null): string {
  if (!hints) {
    return ""
  }
  return (
    hints.targetID ||
    hints.targetEmail ||
    hints.targetName ||
    normalizeEnterpriseTargetHintAsQuery(hints.targetHint)
  ).trim()
}

export type ReceiptRecoveryStatus = "pending" | "attention" | "ready"

export function receiptRecoveryStatusLabel(t: TFunction, status: ReceiptRecoveryStatus): string {
  switch (status) {
    case "ready":
      return t("walletPage.enterprise.receiptRecoveryStatus.ready")
    case "attention":
      return t("walletPage.enterprise.receiptRecoveryStatus.attention")
    case "pending":
    default:
      return t("walletPage.enterprise.receiptRecoveryStatus.pending")
  }
}

export function receiptRecoveryStatusVariant(status: ReceiptRecoveryStatus): "outline" | "secondary" | "destructive" {
  switch (status) {
    case "ready":
      return "outline"
    case "attention":
      return "secondary"
    case "pending":
    default:
      return "destructive"
  }
}

export function withRouteHints(baseLink: string, hints: Record<string, string>) {
  const [pathPart, hashPart] = baseLink.split("#")
  const [pathname, rawQuery = ""] = pathPart.split("?")
  const query = new URLSearchParams(rawQuery)
  Object.entries(hints).forEach(([key, value]) => {
    const normalizedKey = key.trim()
    if (!normalizedKey) {
      return
    }
    const normalizedValue = (value || "").trim()
    if (!normalizedValue) {
      query.delete(normalizedKey)
      return
    }
    query.set(normalizedKey, normalizedValue)
  })
  const nextQuery = query.toString()
  const nextPath = nextQuery ? `${pathname}?${nextQuery}` : pathname
  return hashPart ? `${nextPath}#${hashPart}` : nextPath
}
