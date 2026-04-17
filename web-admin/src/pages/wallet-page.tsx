import { FormEvent, useEffect, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import {
  AlertTriangleIcon,
  MailIcon,
  MessageCircleIcon,
  RefreshCwIcon,
  ShieldAlertIcon,
  TrendingUpIcon,
  Trash2Icon,
  WalletCardsIcon,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableCellText,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Link, useLocation } from "react-router-dom"
import {
  activateWalletPass,
  dispatchWalletPassDelivery,
  createWalletPhysicalCardTask,
  createWalletTemplate,
  dispatchWalletJobAlerts,
  getWalletPass,
  getWalletJobAlertSubscription,
  getWalletJobMetrics,
  getWalletJobMetricsTrend,
  getWalletPassSaveLink,
  issueWalletPass,
  issueWalletPassBatch,
  listEnterpriseEmployees,
  listEnterpriseSyncJobs,
  listWalletJobAlertNotifications,
  listWalletPassDeliveries,
  listWalletPhysicalCardTasks,
  listWalletPasses,
  listWalletTemplates,
  listTenants,
  listUserGroups,
  listWalletDLQCleanupArchives,
  revokeWalletPass,
  retryWalletJobAlertNotification,
  retryWalletPassDelivery,
  suspendWalletPass,
  updateWalletTemplateStatus,
  updateWalletPhysicalCardTaskStatus,
  upsertWalletJobAlertSubscription,
  type CurrentUser,
  type Tenant,
  type EnterpriseEmployee,
  type EnterpriseSyncJob,
  type UserGroup,
  type WalletJobAlertDispatchResult,
  type WalletJobAlertNotification,
  type WalletJobAlertSubscription,
  type WalletDLQCleanupArchive,
  type WalletIssueJob,
  type WalletJobMetrics,
  type WalletJobMetricsTrend,
  type WalletPassDeliveryNotification,
  type WalletPhysicalCardTask,
  type WalletPassInstance,
  type WalletPassTemplate,
} from "@/lib/api"
import { canManageIssuance, getViewerTenantID, isPlatformViewer } from "@/lib/viewer"
import QRCode from "qrcode"

type WalletPageProps = {
  token: string
  viewer: CurrentUser
}

const defaultWindowSeconds = "900"
const defaultArchiveLimit = "20"
const defaultTrendBucketCount = "12"
const defaultSubscriptionCooldownSeconds = "900"
const defaultTemplateStatus = "active"
const defaultBatchExecutionMode = "queued"
const defaultTemplatePassType = "employee"

type WalletTenantAggregateRow = {
  tenantID: string
  tenantName: string
  total: number
  failed: number
  dlq: number
  retryableFailed: number
  alertCount: number
  updatedAt: string
}

type WalletScenarioKind = "employee_mobile" | "employee_physical" | "visitor_qr" | "visitor_temporary"

type WalletScenarioCounters = Record<WalletScenarioKind, number>

type WalletIssuanceScenarioPreset = {
  id: WalletScenarioKind
  title: string
  description: string
  templateName: string
  passType: "employee" | "visitor"
  classID: string
  styleConfig: Record<string, string>
  targetType: "user" | "visitor"
  recommendedExecutionMode: "inline" | "queued"
  defaultExpiresInHours?: number
}

const walletIssuanceScenarioPresets: WalletIssuanceScenarioPreset[] = [
  {
    id: "employee_mobile",
    title: "员工移动凭证",
    description: "面向在职员工的长期 MistyPass，默认用邮件或 WhatsApp 分发保存链接。",
    templateName: "总部员工移动凭证",
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
    title: "员工实体卡联动",
    description: "用于实体卡制作、补卡、挂失和数字凭证联动，员工仍在同一页完成管理。",
    templateName: "总部员工实体卡联动",
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
    title: "访客二维码",
    description: "访客预约、前台来访和邮件二维码都归到同一套访客模板下发。",
    templateName: "访客二维码凭证",
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
    title: "临时证",
    description: "施工、保洁、运维等短时通行场景，默认带明确失效时间和二维码投递。",
    templateName: "临时运维通行证",
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

const walletIssuanceScenarioPresetByID = new Map(
  walletIssuanceScenarioPresets.map((item) => [item.id, item] as const)
)

function parsePositiveInt(raw: string): number | undefined {
  const value = Number.parseInt(raw.trim(), 10)
  if (!Number.isFinite(value) || value <= 0) {
    return undefined
  }
  return value
}

function parseNonNegativeInt(raw: string): number | undefined {
  const value = Number.parseInt(raw.trim(), 10)
  if (!Number.isFinite(value) || value < 0) {
    return undefined
  }
  return value
}

function parseReceiverGroups(raw: string): string[] {
  const groups = raw
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean)
  return Array.from(new Set(groups))
}

function parseTargetIDs(raw: string): string[] {
  return Array.from(
    new Set(
      raw
        .split(/[\n,;]+/g)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

function parseReceiverValues(raw: string): string[] {
  return Array.from(
    new Set(
      raw
        .split(/[\n,;]+/g)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

function parseStyleConfig(raw: string): Record<string, string> | undefined {
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

function normalizeDateTimeInput(raw: string): string | undefined {
  const value = raw.trim()
  if (!value) {
    return undefined
  }
  const timestamp = new Date(value)
  return Number.isNaN(timestamp.getTime()) ? value : timestamp.toISOString()
}

function formatDateTime(value?: string) {
  if (!value) {
    return "-"
  }
  const timestamp = new Date(value)
  if (Number.isNaN(timestamp.getTime())) {
    return value
  }
  return timestamp.toLocaleString()
}

function formatDurationSeconds(seconds?: number) {
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

function formatTimeLabel(value?: string) {
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

function buildRelativeDateTimeInput(hoursFromNow: number) {
  const next = new Date()
  next.setHours(next.getHours() + hoursFromNow)
  return formatDateTimeLocalInput(next)
}

function stringifyStyleConfig(config: Record<string, string>) {
  return Object.entries(config)
    .map(([key, value]) => `${key}=${value}`)
    .join("\n")
}

function templateStatusVariant(status: string) {
  return status === "active" ? "outline" : "secondary"
}

function passStatusVariant(status: string) {
  switch (status) {
    case "active":
      return "outline"
    case "revoked":
      return "destructive"
    default:
      return "secondary"
  }
}

function passStatusLabel(status: string) {
  switch (status) {
    case "issued":
      return "已发放"
    case "active":
      return "已激活"
    case "suspended":
      return "已暂停"
    case "revoked":
      return "已吊销"
    default:
      return status
  }
}

function deliveryNotificationStatusVariant(status: string) {
  switch (status) {
    case "sent":
      return "secondary"
    case "failed":
      return "destructive"
    default:
      return "outline"
  }
}

function deliveryNotificationStatusLabel(status: string) {
  switch (status) {
    case "sent":
      return "已发送"
    case "failed":
      return "发送失败"
    case "skipped":
      return "已跳过"
    default:
      return status
  }
}

function targetTypeLabel(type: string) {
  switch (type) {
    case "user":
      return "员工"
    case "visitor":
      return "访客"
    default:
      return type
  }
}

function passTypeLabel(type: string) {
  switch (type) {
    case "employee":
      return "员工"
    case "visitor":
      return "访客"
    default:
      return type
  }
}

function physicalCardTaskTypeLabel(type: string) {
  switch (type) {
    case "issue":
      return "制卡"
    case "reissue":
      return "补卡"
    case "loss_report":
      return "挂失"
    default:
      return type
  }
}

function physicalCardTaskStatusVariant(status: string) {
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

function physicalCardTaskStatusLabel(status: string) {
  switch (status) {
    case "queued":
      return "待处理"
    case "printing":
      return "制卡中"
    case "ready":
      return "待发卡"
    case "issued":
      return "已发卡"
    case "reported_lost":
      return "已挂失"
    case "cancelled":
      return "已取消"
    default:
      return status
  }
}

function nextPhysicalCardTaskActions(task: WalletPhysicalCardTask) {
  switch (task.task_type) {
    case "issue":
    case "reissue":
      switch (task.status) {
        case "queued":
          return [
            { status: "printing", label: "开始制卡" },
            { status: "ready", label: "备卡完成" },
            { status: "issued", label: "直接发卡" },
            { status: "cancelled", label: "取消任务" },
          ] as const
        case "printing":
          return [
            { status: "ready", label: "备卡完成" },
            { status: "issued", label: "直接发卡" },
            { status: "cancelled", label: "取消任务" },
          ] as const
        case "ready":
          return [
            { status: "issued", label: "标记已发卡" },
            { status: "cancelled", label: "取消任务" },
          ] as const
        default:
          return [] as const
      }
    case "loss_report":
      if (task.status === "queued") {
        return [
          { status: "reported_lost", label: "确认挂失" },
          { status: "cancelled", label: "取消任务" },
        ] as const
      }
      return [] as const
    default:
      return [] as const
  }
}

function createWalletScenarioCounters(): WalletScenarioCounters {
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

function inferTemplateScenario(
  template: Pick<WalletPassTemplate, "pass_type" | "name" | "class_id" | "style_config">
): WalletScenarioKind {
  const haystack = getTemplateScenarioSearchText(template)
  if (template.pass_type === "employee") {
    if (haystack.includes("physical") || haystack.includes("实体") || haystack.includes("card_workflow")) {
      return "employee_physical"
    }
    return "employee_mobile"
  }
  if (haystack.includes("temporary") || haystack.includes("临时")) {
    return "visitor_temporary"
  }
  return "visitor_qr"
}

function inferPassScenario(pass: WalletPassInstance, template?: WalletPassTemplate): WalletScenarioKind {
  if (template) {
    return inferTemplateScenario(template)
  }
  return pass.target_type === "visitor" ? "visitor_qr" : "employee_mobile"
}

function walletScenarioLabel(kind: WalletScenarioKind) {
  return walletIssuanceScenarioPresetByID.get(kind)?.title ?? kind
}

function walletScenarioHint(kind: WalletScenarioKind) {
  switch (kind) {
    case "employee_mobile":
      return "长期员工发放，推荐保留默认有效期并批量开通。"
    case "employee_physical":
      return "适合实体卡制作、挂失补卡和数字凭证状态联动。"
    case "visitor_qr":
      return "适合预约来访、邮件二维码和短期访客通行。"
    case "visitor_temporary":
      return "适合有明确结束时间的运维、施工或保洁临时证。"
    default:
      return ""
  }
}

function deliveryMethodLabel(method?: string) {
  switch (method) {
    case "email_qr":
      return "邮件二维码"
    case "wallet":
      return "MistyPass 保存链接"
    default:
      return method || "-"
  }
}

function accessMediumLabel(medium?: string) {
  switch (medium) {
    case "mobile_pass":
      return "移动凭证"
    case "physical_card":
      return "实体卡联动"
    case "qr_code":
      return "二维码"
    case "temporary_pass":
      return "临时证"
    default:
      return medium || "-"
  }
}

function getTemplateStyleValue(template: WalletPassTemplate | undefined, key: string) {
  return template?.style_config?.[key]?.trim() ?? ""
}

function getTemplateDeliveryMethod(template: WalletPassTemplate | undefined) {
  const configured = getTemplateStyleValue(template, "delivery_method")
  if (configured) {
    return configured
  }
  if (!template) {
    return ""
  }
  return template.pass_type === "visitor" ? "email_qr" : "wallet"
}

function getTemplateAccessMedium(template: WalletPassTemplate | undefined) {
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

function dispatchChannelLabels(template: WalletPassTemplate | undefined) {
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

function deliveryHint(pass: WalletPassInstance, template: WalletPassTemplate | undefined) {
  const scenario = inferPassScenario(pass, template)
  if (scenario === "employee_physical") {
    return "适合制卡、补卡、挂失和数字凭证状态联动。"
  }
  if (pass.save_link) {
    return "已生成保存链接，可直接发送给终端用户。"
  }
  if (scenario === "visitor_temporary") {
    return "临时证建议补齐失效时间后再交付，避免长期有效。"
  }
  return "等待保存链接或渠道交付结果。"
}

function enterpriseWalletStageLabel(stage?: string): string {
  switch ((stage || "").trim()) {
    case "issuance":
      return "已承接到员工凭证发放"
    case "policies":
      return "已承接策略结果，可继续发放验证"
    case "directory":
      return "已承接目录结果，可继续发放验证"
    default:
      return "已承接企业页主路径"
  }
}

function enterpriseWalletSegmentLabel(segmentHint?: string): string {
  switch ((segmentHint || "").trim()) {
    case "directory_usage":
      return "同步结果到用户组使用"
    case "policy_delivery":
      return "用户组使用到权限下发"
    case "issuance_receipt":
      return "策略下发到发放执行与回执"
    case "receipt_recovery":
      return "回执失败分流到重发修复"
    default:
      return ""
  }
}

function enterpriseWalletSegmentStatusLabel(statusHint?: string): string {
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

type ReceiptRecoveryActionHint = "retry_delivery" | "repair_pass_status" | "review_closed"

function resolveReceiptRecoveryActionHint(value?: string): ReceiptRecoveryActionHint | "" {
  const normalized = (value || "").trim()
  if (normalized === "retry_delivery" || normalized === "repair_pass_status" || normalized === "review_closed") {
    return normalized
  }
  return ""
}

function receiptRecoveryActionHintLabel(actionHint: ReceiptRecoveryActionHint | ""): string {
  switch (actionHint) {
    case "retry_delivery":
      return "继续重发失败通道"
    case "repair_pass_status":
      return "继续状态修复"
    case "review_closed":
      return "复核已收口"
    default:
      return ""
  }
}

type EnterpriseFlowTargetHints = {
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

function resolveEnterpriseTargetQuery(hints?: EnterpriseFlowTargetHints | null): string {
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

type ReceiptRecoveryStatus = "pending" | "attention" | "ready"

function receiptRecoveryStatusLabel(status: ReceiptRecoveryStatus): string {
  switch (status) {
    case "ready":
      return "已承接"
    case "attention":
      return "待处理"
    case "pending":
    default:
      return "待补齐"
  }
}

function receiptRecoveryStatusVariant(status: ReceiptRecoveryStatus): "outline" | "secondary" | "destructive" {
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

function withRouteHints(baseLink: string, hints: Record<string, string>) {
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

export function WalletPage({ token, viewer }: WalletPageProps) {
  const location = useLocation()
  const platformViewer = isPlatformViewer(viewer)
  const writable = canManageIssuance(viewer)
  const readOnlyBoundaryHint = "按钮禁用或缺失属于权限边界，不是系统异常。"
  const viewerTenantID = getViewerTenantID(viewer)
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [tenantID, setTenantID] = useState("")
  const [windowSeconds, setWindowSeconds] = useState(defaultWindowSeconds)
  const [maxRetry, setMaxRetry] = useState("")
  const [alertThreshold, setAlertThreshold] = useState("")
  const [archiveLimit, setArchiveLimit] = useState(defaultArchiveLimit)
  const [trendBucketCount, setTrendBucketCount] = useState(defaultTrendBucketCount)

  const [metrics, setMetrics] = useState<WalletJobMetrics | null>(null)
  const [metricsTrend, setMetricsTrend] = useState<WalletJobMetricsTrend | null>(null)
  const [archives, setArchives] = useState<WalletDLQCleanupArchive[]>([])
  const [alertNotifications, setAlertNotifications] = useState<WalletJobAlertNotification[]>([])
  const [subscription, setSubscription] = useState<WalletJobAlertSubscription | null>(null)
  const [tenantAggregates, setTenantAggregates] = useState<WalletTenantAggregateRow[]>([])
  const [templates, setTemplates] = useState<WalletPassTemplate[]>([])
  const [passes, setPasses] = useState<WalletPassInstance[]>([])
  const [enterpriseEmployees, setEnterpriseEmployees] = useState<EnterpriseEmployee[]>([])
  const [enterpriseUserGroups, setEnterpriseUserGroups] = useState<UserGroup[]>([])
  const [enterpriseSyncJobs, setEnterpriseSyncJobs] = useState<EnterpriseSyncJob[]>([])
  const [deliveryNotifications, setDeliveryNotifications] = useState<WalletPassDeliveryNotification[]>([])
  const [physicalCardTasks, setPhysicalCardTasks] = useState<WalletPhysicalCardTask[]>([])
  const [lastIssuedJobs, setLastIssuedJobs] = useState<WalletIssueJob[]>([])
  const [subscriptionEnabled, setSubscriptionEnabled] = useState(true)
  const [subscriptionEmailEnabled, setSubscriptionEmailEnabled] = useState(true)
  const [subscriptionWhatsAppEnabled, setSubscriptionWhatsAppEnabled] = useState(false)
  const [subscriptionThreshold, setSubscriptionThreshold] = useState("")
  const [subscriptionWindowSeconds, setSubscriptionWindowSeconds] = useState(defaultWindowSeconds)
  const [subscriptionCooldownSeconds, setSubscriptionCooldownSeconds] = useState(defaultSubscriptionCooldownSeconds)
  const [subscriptionReceiverGroups, setSubscriptionReceiverGroups] = useState("security")
  const [templateName, setTemplateName] = useState("")
  const [templatePassType, setTemplatePassType] = useState<"employee" | "visitor">(defaultTemplatePassType)
  const [templateClassID, setTemplateClassID] = useState("")
  const [templateStyleConfig, setTemplateStyleConfig] = useState("")
  const [templateStatus, setTemplateStatus] = useState<"active" | "inactive">(defaultTemplateStatus)
  const [singleTemplateID, setSingleTemplateID] = useState("")
  const [singleTargetID, setSingleTargetID] = useState("")
  const [singleExpiresAt, setSingleExpiresAt] = useState("")
  const [batchTemplateID, setBatchTemplateID] = useState("")
  const [batchTargetIDs, setBatchTargetIDs] = useState("")
  const [batchExpiresAt, setBatchExpiresAt] = useState("")
  const [batchExecutionMode, setBatchExecutionMode] = useState<"inline" | "queued">(defaultBatchExecutionMode)
  const [deliveryPassID, setDeliveryPassID] = useState("")
  const [deliveryEmailEnabled, setDeliveryEmailEnabled] = useState(true)
  const [deliveryWhatsAppEnabled, setDeliveryWhatsAppEnabled] = useState(false)
  const [deliveryEmailRecipients, setDeliveryEmailRecipients] = useState("")
  const [deliveryWhatsAppRecipients, setDeliveryWhatsAppRecipients] = useState("")
  const [physicalTaskPassID, setPhysicalTaskPassID] = useState("")
  const [physicalTaskType, setPhysicalTaskType] = useState<"issue" | "reissue" | "loss_report">("issue")
  const [physicalTaskCardNumber, setPhysicalTaskCardNumber] = useState("")
  const [physicalTaskNote, setPhysicalTaskNote] = useState("")
  const [passQuery, setPassQuery] = useState("")
  const [passStatusFilter, setPassStatusFilter] = useState<"all" | "issued" | "active" | "suspended" | "revoked">("all")
  const [passTargetTypeFilter, setPassTargetTypeFilter] = useState<"all" | "user" | "visitor">("all")
  const [passTemplateFilter, setPassTemplateFilter] = useState("all")
  const [selectedPassIDs, setSelectedPassIDs] = useState<string[]>([])

  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [savingSubscription, setSavingSubscription] = useState(false)
  const [dispatchingAlerts, setDispatchingAlerts] = useState(false)
  const [creatingTemplate, setCreatingTemplate] = useState(false)
  const [issuingSingle, setIssuingSingle] = useState(false)
  const [issuingBatch, setIssuingBatch] = useState(false)
  const [dispatchingDelivery, setDispatchingDelivery] = useState(false)
  const [creatingPhysicalCardTask, setCreatingPhysicalCardTask] = useState(false)
  const [updatingTemplateID, setUpdatingTemplateID] = useState("")
  const [updatingPassID, setUpdatingPassID] = useState("")
  const [updatingPhysicalCardTaskID, setUpdatingPhysicalCardTaskID] = useState("")
  const [batchUpdatingPassAction, setBatchUpdatingPassAction] = useState<"" | "activate" | "suspend" | "revoke">("")
  const [retryingDeliveryNotificationID, setRetryingDeliveryNotificationID] = useState("")
  const [batchRetryingDelivery, setBatchRetryingDelivery] = useState(false)
  const [repairingRetryablePasses, setRepairingRetryablePasses] = useState(false)
  const [retryingAlertNotificationID, setRetryingAlertNotificationID] = useState("")
  const [dispatchSummary, setDispatchSummary] = useState("")
  const [issuanceSummary, setIssuanceSummary] = useState("")
  const [error, setError] = useState("")
  const [aggregateWarning, setAggregateWarning] = useState("")
  const [incomingScenarioApplied, setIncomingScenarioApplied] = useState("")
  const [enterpriseFlowSearchApplied, setEnterpriseFlowSearchApplied] = useState("")
  const [enterpriseFlowDirectActionApplied, setEnterpriseFlowDirectActionApplied] = useState("")
  const [resolvingSaveLinkPassID, setResolvingSaveLinkPassID] = useState("")
  const [qrDialogOpen, setQrDialogOpen] = useState(false)
  const [qrDialogPass, setQrDialogPass] = useState<WalletPassInstance | null>(null)
  const [qrDialogSaveLink, setQrDialogSaveLink] = useState("")
  const [qrDialogSVG, setQrDialogSVG] = useState("")
  const [qrDialogLoading, setQrDialogLoading] = useState(false)
  const tenantsQuery = useQuery({
    queryKey: ["wallet-tenants", token, platformViewer ? "platform" : "tenant"],
    queryFn: () => (platformViewer ? listTenants(token) : Promise.resolve([])),
    staleTime: 30 * 1000,
  })
  const queryError = tenantsQuery.error instanceof Error ? tenantsQuery.error.message : ""
  const enterpriseFlowContext = useMemo(() => {
    const query = new URLSearchParams(location.search)
    if (query.get("from")?.trim() !== "enterprise") {
      return null
    }
    return {
      flow: query.get("flow")?.trim() || "",
      syncCategory: query.get("sync_category")?.trim() || "",
      syncJobID: query.get("sync_job_id")?.trim() || "",
      syncSource: query.get("sync_source")?.trim() || "",
      syncStatus: query.get("sync_status")?.trim() || "",
      segmentHint: query.get("segment_hint")?.trim() || "",
      segmentStatusHint: query.get("segment_status_hint")?.trim() || "",
      receiptRecoveryActionHint: query.get("receipt_recovery_action_hint")?.trim() || "",
      stage: query.get("stage")?.trim() || "",
      tenantID: query.get("tenant_id")?.trim() || "",
      targetHint: query.get("target_hint")?.trim() || "",
      targetEmail: query.get("target_email")?.trim() || "",
      targetID: query.get("target_id")?.trim() || "",
      targetIDs: query.get("target_ids")?.trim() || "",
      targetName: query.get("target_name")?.trim() || "",
      templateHint: query.get("template_hint")?.trim() || "",
      workerAlertFailed: query.get("worker_alert_failed")?.trim() || "",
      workerAlertLastSeen: query.get("worker_alert_last_seen")?.trim() || "",
      workerAlertLevel: query.get("worker_alert_level")?.trim() || "",
      workerAlertTenantID: query.get("worker_alert_tenant_id")?.trim() || "",
      workerAlertThreshold: query.get("worker_alert_threshold")?.trim() || "",
      workerFilterHint: query.get("worker_filter_hint")?.trim() || "",
      workerQueryHint: query.get("worker_query_hint")?.trim() || "",
      workerReviewStageHint: query.get("worker_review_stage_hint")?.trim() || "",
      workerReviewStatusHint: query.get("worker_review_status_hint")?.trim() || "",
    }
  }, [location.search])
  const tenantByID = useMemo(() => new Map(tenants.map((item) => [item.id, item])), [tenants])
  const enterpriseBatchTargetStats = useMemo(() => {
    const targetIDs = parseTargetIDs(enterpriseFlowContext?.targetIDs || "")
    if (targetIDs.length === 0) {
      return {
        targetIDs: [] as string[],
        matchedIDs: [] as string[],
        missingIDs: [] as string[],
        hitRate: 0,
      }
    }

    const passTargetIDSet = new Set(
      passes
        .map((item) => item.target_id.trim().toLowerCase())
        .filter(Boolean)
    )
    const matchedIDs: string[] = []
    const missingIDs: string[] = []
    targetIDs.forEach((item) => {
      if (passTargetIDSet.has(item.trim().toLowerCase())) {
        matchedIDs.push(item)
      } else {
        missingIDs.push(item)
      }
    })

    return {
      targetIDs,
      matchedIDs,
      missingIDs,
      hitRate: Math.round((matchedIDs.length / targetIDs.length) * 100),
    }
  }, [enterpriseFlowContext?.targetIDs, passes])
  const enterpriseLatestSyncJob = useMemo(() => {
    const sorted = [...enterpriseSyncJobs].sort((left, right) => {
      return new Date(right.ended_at || right.started_at).getTime() - new Date(left.ended_at || left.started_at).getTime()
    })
    return sorted[0] ?? null
  }, [enterpriseSyncJobs])
  const enterpriseMissingTargetBreakdown = useMemo(() => {
    const employeeByID = new Map(
      enterpriseEmployees
        .map((item) => [item.id.trim().toLowerCase(), item] as const)
        .filter(([id]) => Boolean(id))
    )
    const groupNamesByMemberID = new Map<string, string[]>()
    enterpriseUserGroups.forEach((group) => {
      const members = group.members ?? []
      members.forEach((memberID) => {
        const key = memberID.trim().toLowerCase()
        if (!key) {
          return
        }
        const next = groupNamesByMemberID.get(key) ?? []
        if (!next.includes(group.name)) {
          next.push(group.name)
        }
        groupNamesByMemberID.set(key, next)
      })
    })

    const rows = enterpriseBatchTargetStats.missingIDs.map((targetID) => {
      const key = targetID.trim().toLowerCase()
      const employee = employeeByID.get(key)
      const groups = groupNamesByMemberID.get(key) ?? []
      const groupLabel = groups.slice(0, 2).join(" / ")
      const hasMoreGroups = groups.length > 2
      if (employee) {
        const status = employee.status.trim().toLowerCase()
        if (status === "active") {
          return {
            targetID,
            category: "issue_ready" as const,
            sourceLabel: employee.source || "-",
            groupLabel: groupLabel || "-",
            reason: "目录在职，建议继续批量补发。",
            employeeName: employee.full_name || "",
            approvalHint: employee.email || employee.external_id || targetID,
          }
        }
        return {
          targetID,
          category: "needs_alerts" as const,
          sourceLabel: employee.source || "-",
          groupLabel: groupLabel || "-",
          reason: `目录状态为 ${employee.status}，建议先回企业页处理目录异常。`,
          employeeName: employee.full_name || "",
          approvalHint: employee.email || employee.external_id || targetID,
        }
      }
      if (groups.length > 0) {
        return {
          targetID,
          category: "needs_directory" as const,
          sourceLabel: "-",
          groupLabel: hasMoreGroups ? `${groupLabel} ...` : groupLabel,
          reason: "对象仅存在于用户组，目录未找到该对象，建议先回目录复核成员来源。",
          employeeName: "",
          approvalHint: targetID,
        }
      }
      return {
        targetID,
        category: "needs_alerts" as const,
        sourceLabel: "-",
        groupLabel: "-",
        reason: "目录未找到该对象，建议先回企业页核对同步异常来源。",
        employeeName: "",
        approvalHint: targetID,
      }
    })

    return {
      rows,
      issueReadyCount: rows.filter((item) => item.category === "issue_ready").length,
      needsDirectoryCount: rows.filter((item) => item.category === "needs_directory").length,
      needsAlertsCount: rows.filter((item) => item.category === "needs_alerts").length,
    }
  }, [enterpriseBatchTargetStats.missingIDs, enterpriseEmployees, enterpriseUserGroups])
  const issueReadyEnterpriseMissingTargetIDs = useMemo(
    () =>
      enterpriseMissingTargetBreakdown.rows
        .filter((item) => item.category === "issue_ready")
        .map((item) => item.targetID)
        .slice(0, 20),
    [enterpriseMissingTargetBreakdown.rows]
  )
  const enterpriseAlertsTargetHint = useMemo(() => {
    const firstNeedsAlerts = enterpriseMissingTargetBreakdown.rows.find((item) => item.category === "needs_alerts")
    if (!firstNeedsAlerts) {
      return {
        approvalQuery: "",
        targetHint: "",
      }
    }
    const approvalQuery = firstNeedsAlerts.approvalHint?.trim() || firstNeedsAlerts.targetID.trim()
    const targetHint = firstNeedsAlerts.employeeName.trim() || approvalQuery
    return {
      approvalQuery,
      targetHint,
    }
  }, [enterpriseMissingTargetBreakdown.rows])
  const accessDirectoryReviewLink = useMemo(() => {
    const query = new URLSearchParams()
    const hintedTenantID = (enterpriseFlowContext?.tenantID || tenantID).trim()
    if (hintedTenantID) {
      query.set("tenant_id", hintedTenantID)
    }
    query.set("from", "enterprise")
    query.set("flow", enterpriseFlowContext?.flow || "sync_to_access")
    query.set("stage", "directory")
    const firstDirectoryTarget =
      enterpriseMissingTargetBreakdown.rows.find((item) => item.category === "needs_directory")?.targetID ??
      enterpriseBatchTargetStats.missingIDs[0] ??
      ""
    if (firstDirectoryTarget) {
      query.set("target_id", firstDirectoryTarget)
      query.set("group_member_id", firstDirectoryTarget)
    }
    return `/access/directory?${query.toString()}`
  }, [enterpriseBatchTargetStats.missingIDs, enterpriseFlowContext, enterpriseMissingTargetBreakdown.rows, tenantID])
  const enterpriseSyncIssueHint = useMemo(() => {
    if (!enterpriseLatestSyncJob) {
      return ""
    }
    if (enterpriseLatestSyncJob.status === "completed" && enterpriseLatestSyncJob.rejected <= 0) {
      return ""
    }
    return `最近同步来源 ${enterpriseLatestSyncJob.source} 状态 ${enterpriseLatestSyncJob.status}，rejected ${enterpriseLatestSyncJob.rejected}。`
  }, [enterpriseLatestSyncJob])
  const enterpriseAlertsIssueLink = useMemo(() => {
    const query = new URLSearchParams()
    const hintedTenantID = (enterpriseFlowContext?.tenantID || tenantID).trim()
    if (hintedTenantID) {
      query.set("tenant_id", hintedTenantID)
    }
    query.set("from", "enterprise")
    query.set("flow", enterpriseFlowContext?.flow || "sync_to_access")
    query.set("stage", "issuance")
    if (enterpriseMissingTargetBreakdown.needsAlertsCount > 0) {
      query.set("alerts_view_hint", "directory_exceptions")
    }
    const explicitTargetQueryHint = resolveEnterpriseTargetQuery(enterpriseFlowContext)
    const approvalQueryHint = explicitTargetQueryHint || enterpriseAlertsTargetHint.approvalQuery || ""
    if (approvalQueryHint.trim()) {
      query.set("approval_query_hint", approvalQueryHint.trim())
    }
    const targetHint = explicitTargetQueryHint || enterpriseAlertsTargetHint.targetHint || approvalQueryHint
    if (targetHint.trim()) {
      query.set("target_hint", targetHint.trim())
    }
    const explicitWorkerFilter = enterpriseFlowContext?.workerFilterHint?.trim() || ""
    const workerFilterHint =
      explicitWorkerFilter === "all" ||
      explicitWorkerFilter === "alerting" ||
      explicitWorkerFilter === "hot" ||
      explicitWorkerFilter === "stable"
        ? explicitWorkerFilter
        : enterpriseFlowContext?.workerAlertLevel === "hot" || enterpriseFlowContext?.workerAlertLevel === "alerting"
          ? enterpriseFlowContext.workerAlertLevel
          : ""
    const workerQueryHint =
      enterpriseFlowContext?.workerQueryHint?.trim() || enterpriseFlowContext?.workerAlertTenantID?.trim() || ""
    if (workerFilterHint) {
      query.set("alerts_view_hint", "directory_exceptions")
      query.set("worker_filter_hint", workerFilterHint)
    }
    if (workerQueryHint) {
      query.set("worker_query_hint", workerQueryHint)
    }
    if (enterpriseFlowContext?.workerAlertLevel?.trim()) {
      query.set("worker_alert_level", enterpriseFlowContext.workerAlertLevel.trim())
    }
    if (enterpriseFlowContext?.workerAlertTenantID?.trim()) {
      query.set("worker_alert_tenant_id", enterpriseFlowContext.workerAlertTenantID.trim())
    }
    if (enterpriseFlowContext?.workerAlertLastSeen?.trim()) {
      query.set("worker_alert_last_seen", enterpriseFlowContext.workerAlertLastSeen.trim())
    }
    if (enterpriseFlowContext?.workerAlertFailed?.trim()) {
      query.set("worker_alert_failed", enterpriseFlowContext.workerAlertFailed.trim())
    }
    if (enterpriseFlowContext?.workerAlertThreshold?.trim()) {
      query.set("worker_alert_threshold", enterpriseFlowContext.workerAlertThreshold.trim())
    }
    const explicitSyncCategory = enterpriseFlowContext?.syncCategory?.trim() || ""
    const explicitSyncStatusHint =
      explicitSyncCategory === "all" ||
      explicitSyncCategory === "attention" ||
      explicitSyncCategory === "rejected" ||
      explicitSyncCategory === "deactivated" ||
      explicitSyncCategory === "healthy"
        ? explicitSyncCategory
        : ""
    if (explicitSyncStatusHint) {
      query.set("sync_status_hint", explicitSyncStatusHint)
      if (enterpriseFlowContext?.syncSource?.trim()) {
        query.set("sync_source_hint", enterpriseFlowContext.syncSource.trim())
      }
      if (enterpriseFlowContext?.syncJobID?.trim()) {
        query.set("sync_query_hint", enterpriseFlowContext.syncJobID.trim())
      }
    } else if (enterpriseLatestSyncJob) {
      const normalizedStatus = (enterpriseLatestSyncJob.status || "").trim().toLowerCase()
      let syncStatusHint: "attention" | "rejected" | "deactivated" | "healthy" = "healthy"
      if (normalizedStatus !== "completed") {
        syncStatusHint = "attention"
      } else if (enterpriseLatestSyncJob.rejected > 0) {
        syncStatusHint = "rejected"
      } else if (enterpriseLatestSyncJob.deactivated > 0) {
        syncStatusHint = "deactivated"
      }
      query.set("sync_status_hint", syncStatusHint)
      if (enterpriseLatestSyncJob.source.trim()) {
        query.set("sync_source_hint", enterpriseLatestSyncJob.source.trim())
      }
      if (enterpriseLatestSyncJob.id.trim()) {
        query.set("sync_query_hint", enterpriseLatestSyncJob.id.trim())
      }
    }
    const nextQuery = query.toString()
    return nextQuery ? `/enterprise?${nextQuery}#alerts` : "/enterprise#alerts"
  }, [
    enterpriseAlertsTargetHint.approvalQuery,
    enterpriseAlertsTargetHint.targetHint,
    enterpriseFlowContext,
    enterpriseLatestSyncJob,
    enterpriseMissingTargetBreakdown.needsAlertsCount,
    tenantID,
  ])
  const hasWorkerAlertFlowHints = Boolean(
    enterpriseFlowContext &&
      (
        enterpriseFlowContext.workerAlertLevel ||
        enterpriseFlowContext.workerAlertTenantID ||
        enterpriseFlowContext.workerFilterHint ||
        enterpriseFlowContext.workerQueryHint
      ).trim()
  )
  const enterpriseSyncWorkerReviewLink = useMemo(() => {
    const query = new URLSearchParams()
    const hintedTenantID = (enterpriseFlowContext?.tenantID || tenantID).trim()
    if (hintedTenantID) {
      query.set("tenant_id", hintedTenantID)
    }
    query.set("from", "enterprise")
    query.set("flow", enterpriseFlowContext?.flow || "sync_to_access")
    query.set("stage", "issuance")
    query.set("sync_focus_hint", "worker_alert")
    query.set("worker_review_status_hint", "handled")
    query.set("worker_review_stage_hint", "issuance")
    const explicitWorkerFilter = enterpriseFlowContext?.workerFilterHint?.trim() || ""
    const workerFilterHint =
      explicitWorkerFilter === "all" ||
      explicitWorkerFilter === "alerting" ||
      explicitWorkerFilter === "hot" ||
      explicitWorkerFilter === "stable"
        ? explicitWorkerFilter
        : enterpriseFlowContext?.workerAlertLevel === "hot" ||
            enterpriseFlowContext?.workerAlertLevel === "alerting" ||
            enterpriseFlowContext?.workerAlertLevel === "stable"
          ? enterpriseFlowContext.workerAlertLevel
          : ""
    if (workerFilterHint) {
      query.set("worker_filter_hint", workerFilterHint)
    }
    const workerQueryHint =
      enterpriseFlowContext?.workerQueryHint?.trim() || enterpriseFlowContext?.workerAlertTenantID?.trim() || ""
    if (workerQueryHint) {
      query.set("worker_query_hint", workerQueryHint)
    }
    if (enterpriseFlowContext?.workerAlertLevel?.trim()) {
      query.set("worker_alert_level", enterpriseFlowContext.workerAlertLevel.trim())
    }
    if (enterpriseFlowContext?.workerAlertTenantID?.trim()) {
      query.set("worker_alert_tenant_id", enterpriseFlowContext.workerAlertTenantID.trim())
    }
    if (enterpriseFlowContext?.workerAlertLastSeen?.trim()) {
      query.set("worker_alert_last_seen", enterpriseFlowContext.workerAlertLastSeen.trim())
    }
    if (enterpriseFlowContext?.workerAlertFailed?.trim()) {
      query.set("worker_alert_failed", enterpriseFlowContext.workerAlertFailed.trim())
    }
    if (enterpriseFlowContext?.workerAlertThreshold?.trim()) {
      query.set("worker_alert_threshold", enterpriseFlowContext.workerAlertThreshold.trim())
    }
    const nextQuery = query.toString()
    return nextQuery ? `/enterprise?${nextQuery}#sync` : "/enterprise#sync"
  }, [enterpriseFlowContext, tenantID])
  const enterpriseFlowSegmentDescriptor = useMemo(() => {
    if (!enterpriseFlowContext) {
      return ""
    }
    const segmentLabel = enterpriseWalletSegmentLabel(enterpriseFlowContext.segmentHint)
    if (!segmentLabel) {
      return ""
    }
    const statusLabel = enterpriseWalletSegmentStatusLabel(enterpriseFlowContext.segmentStatusHint)
    return statusLabel ? `${segmentLabel} / ${statusLabel}` : segmentLabel
  }, [enterpriseFlowContext])

  const windowErrorCodeRows = useMemo(() => {
    const items = Object.entries(metrics?.window.error_code_breakdown ?? {})
    return items.sort((a, b) => b[1] - a[1]).slice(0, 5)
  }, [metrics?.window.error_code_breakdown])
  const trendPeakUpdated = useMemo(() => {
    const peak = Math.max(...(metricsTrend?.buckets.map((item) => item.updated) ?? [0]))
    return peak > 0 ? peak : 1
  }, [metricsTrend?.buckets])
  const aggregateStats = useMemo(() => {
    const totals = tenantAggregates.reduce(
      (acc, row) => {
        acc.total += row.total
        acc.failed += row.failed
        acc.dlq += row.dlq
        acc.alertTenants += row.alertCount > 0 ? 1 : 0
        return acc
      },
      { total: 0, failed: 0, dlq: 0, alertTenants: 0 }
    )
    return totals
  }, [tenantAggregates])
  const selectedSingleTemplate = useMemo(
    () => templates.find((item) => item.id === singleTemplateID) ?? null,
    [singleTemplateID, templates]
  )
  const selectedBatchTemplate = useMemo(
    () => templates.find((item) => item.id === batchTemplateID) ?? null,
    [batchTemplateID, templates]
  )
  const activeEmployeeTemplate = useMemo(
    () => templates.find((item) => item.pass_type === "employee" && item.status === "active") ?? null,
    [templates]
  )
  const activeVisitorTemplate = useMemo(
    () => templates.find((item) => item.pass_type === "visitor" && item.status === "active") ?? null,
    [templates]
  )
  const templateByID = useMemo(() => new Map(templates.map((item) => [item.id, item])), [templates])
  const filteredPasses = useMemo(() => {
    const q = passQuery.trim().toLowerCase()
    return passes.filter((item) => {
      if (passStatusFilter !== "all" && item.status !== passStatusFilter) {
        return false
      }
      if (passTargetTypeFilter !== "all" && item.target_type !== passTargetTypeFilter) {
        return false
      }
      if (passTemplateFilter !== "all" && item.template_id !== passTemplateFilter) {
        return false
      }
      if (!q) {
        return true
      }
      const templateName = templateByID.get(item.template_id)?.name ?? item.template_id
      return (
        item.id.toLowerCase().includes(q) ||
        item.target_id.toLowerCase().includes(q) ||
        item.object_id.toLowerCase().includes(q) ||
        item.status.toLowerCase().includes(q) ||
        item.target_type.toLowerCase().includes(q) ||
        templateName.toLowerCase().includes(q)
      )
    })
  }, [passQuery, passStatusFilter, passTargetTypeFilter, passTemplateFilter, passes, templateByID])
  const selectedPassIDSet = useMemo(() => new Set(selectedPassIDs), [selectedPassIDs])
  const selectedVisiblePassCount = useMemo(() => {
    return filteredPasses.reduce((sum, item) => (selectedPassIDSet.has(item.id) ? sum + 1 : sum), 0)
  }, [filteredPasses, selectedPassIDSet])
  const allVisiblePassesSelected = filteredPasses.length > 0 && selectedVisiblePassCount === filteredPasses.length
  const hasPassFilters =
    passQuery.trim().length > 0 ||
    passStatusFilter !== "all" ||
    passTargetTypeFilter !== "all" ||
    passTemplateFilter !== "all"
  const employeePassCount = useMemo(() => passes.filter((item) => item.target_type === "user").length, [passes])
  const visitorPassCount = useMemo(() => passes.filter((item) => item.target_type === "visitor").length, [passes])
  const suspendedPassCount = useMemo(() => passes.filter((item) => item.status === "suspended").length, [passes])
  const revocablePassCount = useMemo(() => passes.filter((item) => item.status !== "revoked").length, [passes])
  const activeTemplateByScenario = useMemo(() => {
    const next = new Map<WalletScenarioKind, WalletPassTemplate>()
    for (const item of templates) {
      if (item.status !== "active") {
        continue
      }
      const scenario = inferTemplateScenario(item)
      if (!next.has(scenario)) {
        next.set(scenario, item)
      }
    }
    return next
  }, [templates])
  const templateScenarioCounts = useMemo(() => {
    const next = createWalletScenarioCounters()
    templates.forEach((item) => {
      next[inferTemplateScenario(item)] += 1
    })
    return next
  }, [templates])
  const passScenarioCounts = useMemo(() => {
    const next = createWalletScenarioCounters()
    passes.forEach((item) => {
      next[inferPassScenario(item, templateByID.get(item.template_id))] += 1
    })
    return next
  }, [passes, templateByID])
  const saveLinkScenarioCounts = useMemo(() => {
    const next = createWalletScenarioCounters()
    passes.forEach((item) => {
      if (!item.save_link) {
        return
      }
      next[inferPassScenario(item, templateByID.get(item.template_id))] += 1
    })
    return next
  }, [passes, templateByID])
  const deliveryDeskPasses = useMemo(() => {
    return filteredPasses
      .filter((item) => {
        const scenario = inferPassScenario(item, templateByID.get(item.template_id))
        return item.save_link || scenario === "employee_physical"
      })
      .slice(0, 6)
  }, [filteredPasses, templateByID])
  const deliverablePasses = useMemo(() => {
    return passes.filter((item) => item.save_link)
  }, [passes])
  const selectedDeliveryPass = useMemo(
    () => passes.find((item) => item.id === deliveryPassID) ?? null,
    [deliveryPassID, passes]
  )
  const selectedDeliveryTemplate = useMemo(
    () => (selectedDeliveryPass ? templateByID.get(selectedDeliveryPass.template_id) : undefined),
    [selectedDeliveryPass, templateByID]
  )
  const recentDeliveryNotifications = useMemo(() => deliveryNotifications.slice(0, 6), [deliveryNotifications])
  const failedDeliveryNotifications = useMemo(
    () => deliveryNotifications.filter((item) => item.status === "failed"),
    [deliveryNotifications]
  )
  const nonRetryableFailedDeliveryNotifications = useMemo(
    () => failedDeliveryNotifications.filter((item) => !item.retryable),
    [failedDeliveryNotifications]
  )
  const deliveryRetryQuery = useMemo(() => {
    const targetHint = resolveEnterpriseTargetQuery(enterpriseFlowContext)
    return targetHint || passQuery.trim()
  }, [enterpriseFlowContext, passQuery])
  const retryableDeliveryNotifications = useMemo(() => {
    const q = deliveryRetryQuery.trim().toLowerCase()
    return deliveryNotifications
      .filter((item) => item.status === "failed" && item.retryable)
      .filter((item) => {
        if (!q) {
          return true
        }
        return (
          item.target_id.toLowerCase().includes(q) ||
          item.pass_id.toLowerCase().includes(q) ||
          item.id.toLowerCase().includes(q) ||
          (item.reason || "").toLowerCase().includes(q)
        )
      })
  }, [deliveryNotifications, deliveryRetryQuery])
  const batchRetryableDeliveryNotifications = useMemo(
    () => retryableDeliveryNotifications.slice(0, 20),
    [retryableDeliveryNotifications]
  )
  const retryableDeliveryPasses = useMemo(() => {
    if (batchRetryableDeliveryNotifications.length === 0) {
      return []
    }
    const retryablePassIDSet = new Set(batchRetryableDeliveryNotifications.map((item) => item.pass_id))
    return passes.filter((item) => retryablePassIDSet.has(item.id))
  }, [batchRetryableDeliveryNotifications, passes])
  const repairableRetryableDeliveryPasses = useMemo(
    () => retryableDeliveryPasses.filter((item) => item.status === "issued" || item.status === "suspended").slice(0, 20),
    [retryableDeliveryPasses]
  )
  const reissueTargetIDsByRetryableDelivery = useMemo(
    () =>
      Array.from(
        new Set(batchRetryableDeliveryNotifications.map((item) => item.target_id.trim()).filter(Boolean))
      ).slice(0, 20),
    [batchRetryableDeliveryNotifications]
  )
  const reissueTemplateByRetryableDelivery = useMemo(() => {
    const matchedTemplates: WalletPassTemplate[] = []
    const matchedTemplateIDSet = new Set<string>()
    retryableDeliveryPasses.forEach((item) => {
      const template = templateByID.get(item.template_id)
      if (!template || matchedTemplateIDSet.has(template.id)) {
        return
      }
      matchedTemplateIDSet.add(template.id)
      matchedTemplates.push(template)
    })
    const matchedActiveTemplate = matchedTemplates.find((item) => item.status === "active")
    if (matchedActiveTemplate) {
      return matchedActiveTemplate
    }
    const targetTypeHint = batchRetryableDeliveryNotifications[0]?.target_type
    if (targetTypeHint === "visitor" && activeVisitorTemplate) {
      return activeVisitorTemplate
    }
    if (targetTypeHint === "user" && activeEmployeeTemplate) {
      return activeEmployeeTemplate
    }
    return activeEmployeeTemplate || activeVisitorTemplate || matchedTemplates[0] || null
  }, [
    activeEmployeeTemplate,
    activeVisitorTemplate,
    batchRetryableDeliveryNotifications,
    retryableDeliveryPasses,
    templateByID,
  ])
  const receiptRecoveryFlowStatus: ReceiptRecoveryStatus =
    deliveryNotifications.length === 0 ? "pending" : failedDeliveryNotifications.length > 0 ? "attention" : "ready"
  const receiptSplitStatus: ReceiptRecoveryStatus =
    deliveryNotifications.length === 0 ? "pending" : failedDeliveryNotifications.length > 0 ? "attention" : "ready"
  const receiptRemediationStatus: ReceiptRecoveryStatus =
    failedDeliveryNotifications.length === 0 ? (deliveryNotifications.length === 0 ? "pending" : "ready") : "attention"
  const receiptReviewStatus: ReceiptRecoveryStatus =
    failedDeliveryNotifications.length === 0 ? (deliveryNotifications.length === 0 ? "pending" : "ready") : "attention"
  const enterpriseReceiptRecoveryReviewLink = useMemo(
    () =>
      withRouteHints(enterpriseAlertsIssueLink, {
        alerts_view_hint: "directory_exceptions",
        segment_hint: "receipt_recovery",
        segment_status_hint: receiptRecoveryFlowStatus,
      }),
    [enterpriseAlertsIssueLink, receiptRecoveryFlowStatus]
  )
  const employeeCardEligiblePasses = useMemo(() => {
    return passes.filter((item) => item.target_type === "user")
  }, [passes])
  const selectedPhysicalTaskPass = useMemo(
    () => passes.find((item) => item.id === physicalTaskPassID) ?? null,
    [passes, physicalTaskPassID]
  )
  const selectedPhysicalTaskTemplate = useMemo(
    () => (selectedPhysicalTaskPass ? templateByID.get(selectedPhysicalTaskPass.template_id) : undefined),
    [selectedPhysicalTaskPass, templateByID]
  )
  const recentPhysicalCardTasks = useMemo(() => physicalCardTasks.slice(0, 6), [physicalCardTasks])
  const qrDialogTemplate = useMemo(
    () => (qrDialogPass ? templateByID.get(qrDialogPass.template_id) : undefined),
    [qrDialogPass, templateByID]
  )

  function buildMetricsQueryOptions() {
    return {
      window_seconds: parsePositiveInt(windowSeconds),
      max_retry: parseNonNegativeInt(maxRetry),
      dlq_alert_threshold: parsePositiveInt(alertThreshold),
    }
  }

  function buildMetricsTrendQueryOptions() {
    return {
      window_seconds: parsePositiveInt(windowSeconds),
      bucket_count: parsePositiveInt(trendBucketCount),
      max_retry: parseNonNegativeInt(maxRetry),
      dlq_alert_threshold: parsePositiveInt(alertThreshold),
    }
  }

  function syncSubscriptionDraft(next: WalletJobAlertSubscription) {
    setSubscriptionEnabled(next.enabled)
    setSubscriptionEmailEnabled(next.channels.email)
    setSubscriptionWhatsAppEnabled(next.channels.whatsapp)
    setSubscriptionThreshold(String(next.dlq_alert_threshold))
    setSubscriptionWindowSeconds(String(next.window_seconds))
    setSubscriptionCooldownSeconds(String(next.cooldown_seconds))
    setSubscriptionReceiverGroups((next.receiver_groups ?? ["security"]).join(", "))
  }

  function resolveTargetType(templateID: string): "user" | "visitor" {
    return templates.find((item) => item.id === templateID)?.pass_type === "visitor" ? "visitor" : "user"
  }

  function pickDefaultTemplateID(items: WalletPassTemplate[]): string {
    return items.find((item) => item.status === "active")?.id ?? items[0]?.id ?? ""
  }

  useEffect(() => {
    if (platformViewer && tenantsQuery.isPending) {
      return
    }

    async function bootstrap() {
      setLoading(true)
      setError("")
      try {
        const tenantItems = platformViewer ? tenantsQuery.data ?? [] : []
        setTenants(tenantItems)

        const nextTenantID = platformViewer ? tenantItems[0]?.id ?? "" : viewerTenantID
        setTenantID(nextTenantID)
        if (!nextTenantID) {
          setMetrics(null)
          setMetricsTrend(null)
          setArchives([])
          setAlertNotifications([])
          setSubscription(null)
          setTenantAggregates([])
          setTemplates([])
          setPasses([])
          setEnterpriseEmployees([])
          setEnterpriseUserGroups([])
          setEnterpriseSyncJobs([])
          setDeliveryNotifications([])
          setPhysicalCardTasks([])
          setLastIssuedJobs([])
          return
        }
        await loadWalletOps(nextTenantID)
        if (platformViewer) {
          await loadWalletTenantAggregates(tenantItems)
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : "加载凭证发放数据失败"
        setError(message)
      } finally {
        setLoading(false)
      }
    }

    void bootstrap()
  }, [platformViewer, tenantsQuery.data, tenantsQuery.isPending, token, viewerTenantID])

  useEffect(() => {
    if (loading) {
      return
    }
    if (!enterpriseFlowContext) {
      if (enterpriseFlowSearchApplied) {
        setEnterpriseFlowSearchApplied("")
      }
      if (enterpriseFlowDirectActionApplied) {
        setEnterpriseFlowDirectActionApplied("")
      }
      return
    }
    if (enterpriseFlowSearchApplied === location.search) {
      return
    }

    const incomingTenantID = enterpriseFlowContext.tenantID
    const canApplyTenant = Boolean(
      incomingTenantID &&
        (platformViewer
          ? tenants.some((item) => item.id === incomingTenantID)
          : incomingTenantID === viewerTenantID)
    )
    if (canApplyTenant && incomingTenantID !== tenantID) {
      setTenantID(incomingTenantID)
      void applyFilters(incomingTenantID)
    }

    const tenantLabel = canApplyTenant ? tenantByID.get(incomingTenantID)?.name || incomingTenantID : ""
    const flowLabel = enterpriseFlowContext.flow ? `${enterpriseFlowContext.flow} / ` : ""
    const batchTargetIDsHint = enterpriseBatchTargetStats.targetIDs
    if (enterpriseFlowContext.targetHint === "user" || enterpriseFlowContext.targetHint === "visitor") {
      setPassTargetTypeFilter(enterpriseFlowContext.targetHint)
    }
    const targetIDHint = enterpriseFlowContext.targetID || enterpriseFlowContext.targetEmail
    const targetQueryHint = resolveEnterpriseTargetQuery(enterpriseFlowContext)
    if (targetIDHint && !singleTargetID.trim()) {
      setSingleTargetID(targetIDHint)
    }
    if (targetQueryHint && !passQuery.trim()) {
      setPassQuery(targetQueryHint)
    }
    if (batchTargetIDsHint.length > 0 && !batchTargetIDs.trim()) {
      setBatchTargetIDs(batchTargetIDsHint.join("\n"))
    }
    if (enterpriseFlowContext.targetEmail && !deliveryEmailRecipients.trim()) {
      setDeliveryEmailRecipients(enterpriseFlowContext.targetEmail)
    }
    if (enterpriseFlowContext.templateHint === "employee" || enterpriseFlowContext.templateHint === "visitor") {
      setTemplatePassType(enterpriseFlowContext.templateHint)
      if (!templateName.trim()) {
        setTemplateName(
          enterpriseFlowContext.templateHint === "employee" ? "总部员工移动凭证" : "访客二维码凭证"
        )
      }
    }
    const targetLabel = enterpriseFlowContext.targetName || enterpriseFlowContext.targetEmail || enterpriseFlowContext.targetID
    const syncRecordLabel = enterpriseFlowContext.syncJobID
      ? `${enterpriseFlowContext.syncSource || "同步"} 任务 ${enterpriseFlowContext.syncJobID}${
          enterpriseFlowContext.syncStatus ? `（${enterpriseFlowContext.syncStatus}）` : ""
        }`
      : ""
    const workerAlertLabel =
      enterpriseFlowContext.workerAlertLevel || enterpriseFlowContext.workerFilterHint
        ? `${enterpriseFlowContext.workerAlertTenantID || tenantID || "当前租户"} worker ${
            enterpriseFlowContext.workerAlertLevel || enterpriseFlowContext.workerFilterHint
          } 告警${
            enterpriseFlowContext.workerAlertFailed && enterpriseFlowContext.workerAlertThreshold
              ? `（failed ${enterpriseFlowContext.workerAlertFailed} / threshold ${enterpriseFlowContext.workerAlertThreshold}）`
              : ""
          }`
        : ""
    const receiptRecoveryActionLabel =
      enterpriseFlowContext.segmentHint === "receipt_recovery"
        ? receiptRecoveryActionHintLabel(resolveReceiptRecoveryActionHint(enterpriseFlowContext.receiptRecoveryActionHint))
        : ""
    setIssuanceSummary(
      `来源：企业页。已承接 ${flowLabel}${enterpriseWalletStageLabel(enterpriseFlowContext.stage)}${
        tenantLabel ? `（组织：${tenantLabel}）` : ""
      }${enterpriseFlowSegmentDescriptor ? `（分段提示：${enterpriseFlowSegmentDescriptor}）` : ""}${
        receiptRecoveryActionLabel ? `（复核结论：${receiptRecoveryActionLabel}）` : ""
      }${
        targetLabel ? `，并定位发放对象“${targetLabel}”` : ""
      }${
        batchTargetIDsHint.length > 0
          ? `，并预填批量发放对象 ${batchTargetIDsHint.length} 个（已命中 ${enterpriseBatchTargetStats.matchedIDs.length} / 未命中 ${enterpriseBatchTargetStats.missingIDs.length}）`
          : ""
      }${syncRecordLabel ? `，同步记录为“${syncRecordLabel}”` : ""}${
        workerAlertLabel ? `，worker 告警为“${workerAlertLabel}”` : ""
      }。`
    )
    setEnterpriseFlowSearchApplied(location.search)
  }, [
    batchTargetIDs,
    deliveryEmailRecipients,
    enterpriseBatchTargetStats,
    enterpriseFlowDirectActionApplied,
    enterpriseFlowContext,
    enterpriseFlowSegmentDescriptor,
    enterpriseFlowSearchApplied,
    loading,
    location.search,
    passQuery,
    platformViewer,
    singleTargetID,
    tenantByID,
    tenantID,
    tenants,
    templateName,
    viewerTenantID,
  ])

  useEffect(() => {
    if (loading || refreshing) {
      return
    }
    if (!enterpriseFlowContext) {
      if (enterpriseFlowDirectActionApplied) {
        setEnterpriseFlowDirectActionApplied("")
      }
      return
    }
    if (enterpriseFlowSearchApplied !== location.search) {
      return
    }
    if (enterpriseFlowDirectActionApplied === location.search) {
      return
    }

    const incomingTenantID = enterpriseFlowContext.tenantID.trim()
    const canApplyTenant = Boolean(
      incomingTenantID &&
        (platformViewer
          ? tenants.some((item) => item.id === incomingTenantID)
          : incomingTenantID === viewerTenantID)
    )
    if (canApplyTenant && tenantID !== incomingTenantID) {
      return
    }

    const targetQuery = resolveEnterpriseTargetQuery(enterpriseFlowContext)
    const receiptRecoveryFlow = enterpriseFlowContext.segmentHint.trim() === "receipt_recovery"
    const receiptRecoveryActionHint = resolveReceiptRecoveryActionHint(enterpriseFlowContext.receiptRecoveryActionHint)

    const matchedPasses =
      targetQuery.length > 0
        ? passes.filter((item) => {
            const q = targetQuery.toLowerCase()
            return (
              item.target_id.toLowerCase().includes(q) ||
              item.id.toLowerCase().includes(q) ||
              item.object_id.toLowerCase().includes(q)
            )
          })
        : []
    if (matchedPasses.length > 0) {
      const preferredPass = matchedPasses.find((item) => item.save_link) ?? matchedPasses[0]
      if (!deliveryPassID.trim() || !matchedPasses.some((item) => item.id === deliveryPassID)) {
        setDeliveryPassID(preferredPass.id)
      }
    }

    if (receiptRecoveryFlow) {
      if (receiptRecoveryActionHint === "retry_delivery") {
        setIssuanceSummary(
          `来源：企业页复核结论。建议继续重发失败通道：当前可批量重发 ${batchRetryableDeliveryNotifications.length} 条，可修复状态 ${repairableRetryableDeliveryPasses.length} 张。`
        )
      } else if (receiptRecoveryActionHint === "repair_pass_status") {
        setIssuanceSummary(
          `来源：企业页复核结论。建议继续状态修复：当前可修复状态 ${repairableRetryableDeliveryPasses.length} 张，可批量重发 ${batchRetryableDeliveryNotifications.length} 条。`
        )
      } else if (receiptRecoveryActionHint === "review_closed") {
        setIssuanceSummary(
          `来源：企业页复核结论。复核已收口：当前失败回执 ${failedDeliveryNotifications.length} 条，若仍需处理可继续重发或状态修复。`
        )
      } else if (targetQuery && matchedPasses.length > 0) {
        setIssuanceSummary(
          `来源：企业页。已命中 ${matchedPasses.length} 条目标凭证，已直达外部投递对象，可直接补发或重发失败通道。`
        )
      } else if (targetQuery) {
        setIssuanceSummary("来源：企业页。未找到该对象的既有凭证，已预填单发对象，可直接创建补发。")
      } else {
        setIssuanceSummary("来源：企业页。已回流到回执失败恢复闭环，请在“最近外部投递回执”继续重发或状态修复。")
      }
      setEnterpriseFlowDirectActionApplied(location.search)
      return
    }

    if (!targetQuery) {
      setEnterpriseFlowDirectActionApplied(location.search)
      return
    }

    if (matchedPasses.length > 0) {
      setIssuanceSummary(
        `来源：企业页。已命中 ${matchedPasses.length} 条目标凭证，已直达外部投递对象，可直接补发或重发失败通道。`
      )
    } else {
      setIssuanceSummary("来源：企业页。未找到该对象的既有凭证，已预填单发对象，可直接创建补发。")
    }
    setEnterpriseFlowDirectActionApplied(location.search)
  }, [
    batchRetryableDeliveryNotifications.length,
    deliveryPassID,
    enterpriseFlowContext,
    enterpriseFlowDirectActionApplied,
    enterpriseFlowSearchApplied,
    failedDeliveryNotifications.length,
    loading,
    location.search,
    passes,
    platformViewer,
    repairableRetryableDeliveryPasses.length,
    refreshing,
    tenantID,
    tenants,
    viewerTenantID,
  ])

  useEffect(() => {
    const fallbackTemplateID = pickDefaultTemplateID(templates)
    if (!templates.some((item) => item.id === singleTemplateID)) {
      setSingleTemplateID(fallbackTemplateID)
    }
    if (!templates.some((item) => item.id === batchTemplateID)) {
      setBatchTemplateID(fallbackTemplateID)
    }
  }, [batchTemplateID, singleTemplateID, templates])

  useEffect(() => {
    const fallbackPassID = deliverablePasses[0]?.id ?? ""
    if (!deliverablePasses.some((item) => item.id === deliveryPassID)) {
      setDeliveryPassID(fallbackPassID)
    }
  }, [deliverablePasses, deliveryPassID])

  useEffect(() => {
    const fallbackPassID = employeeCardEligiblePasses[0]?.id ?? ""
    if (!employeeCardEligiblePasses.some((item) => item.id === physicalTaskPassID)) {
      setPhysicalTaskPassID(fallbackPassID)
    }
  }, [employeeCardEligiblePasses, physicalTaskPassID])

  useEffect(() => {
    const visiblePassIDSet = new Set(passes.map((item) => item.id))
    setSelectedPassIDs((current) => current.filter((item) => visiblePassIDSet.has(item)))
    if (passTemplateFilter !== "all" && !templates.some((item) => item.id === passTemplateFilter)) {
      setPassTemplateFilter("all")
    }
  }, [passTemplateFilter, passes, templates])

  useEffect(() => {
    if (loading) {
      return
    }
    const rawScenario = new URLSearchParams(location.search).get("scenario")
    const nextScenario =
      rawScenario && walletIssuanceScenarioPresetByID.has(rawScenario as WalletScenarioKind)
        ? (rawScenario as WalletScenarioKind)
        : ""
    if (!nextScenario) {
      if (incomingScenarioApplied) {
        setIncomingScenarioApplied("")
      }
      return
    }
    if (incomingScenarioApplied === nextScenario) {
      return
    }
    applyScenarioPreset(nextScenario)
    setIncomingScenarioApplied(nextScenario)
  }, [incomingScenarioApplied, loading, location.search])

  async function loadWalletOps(nextTenantID: string) {
    const metricsQuery = buildMetricsQueryOptions()
    const trendQuery = buildMetricsTrendQueryOptions()
    const nextArchiveLimit = parsePositiveInt(archiveLimit) ?? 20

    const [metricsData, trendData, archiveItems, notificationItems, subscriptionData, templateItems, passItems, deliveryItems, physicalTaskItems] =
      await Promise.all([
        getWalletJobMetrics(token, {
          tenant_id: nextTenantID,
          ...metricsQuery,
        }),
        getWalletJobMetricsTrend(token, {
          tenant_id: nextTenantID,
          ...trendQuery,
        }),
        listWalletDLQCleanupArchives(token, {
          tenant_id: nextTenantID,
          limit: nextArchiveLimit,
        }),
        listWalletJobAlertNotifications(token, {
          tenant_id: nextTenantID,
          limit: 30,
        }),
        getWalletJobAlertSubscription(token, {
          tenant_id: nextTenantID,
        }),
        listWalletTemplates(token, nextTenantID),
        listWalletPasses(token, nextTenantID),
        listWalletPassDeliveries(token, {
          tenant_id: nextTenantID,
        }),
        listWalletPhysicalCardTasks(token, nextTenantID),
      ])

    setMetrics(metricsData)
    setMetricsTrend(trendData)
    setArchives(archiveItems)
    setAlertNotifications(notificationItems)
    setSubscription(subscriptionData)
    syncSubscriptionDraft(subscriptionData)
    setTemplates(
      [...templateItems].sort((a, b) => {
        if (a.status !== b.status) {
          return a.status === "active" ? -1 : 1
        }
        return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
      })
    )
    setPasses(
      [...passItems].sort(
        (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
      )
    )
    setDeliveryNotifications(
      [...deliveryItems].sort(
        (a, b) => new Date(b.triggered_at).getTime() - new Date(a.triggered_at).getTime()
      )
    )
    setPhysicalCardTasks(
      [...physicalTaskItems].sort(
        (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
      )
    )

    const [employeesResult, groupsResult, syncJobsResult] = await Promise.allSettled([
      listEnterpriseEmployees(token, nextTenantID),
      listUserGroups(token),
      listEnterpriseSyncJobs(token, nextTenantID),
    ])
    setEnterpriseEmployees(employeesResult.status === "fulfilled" ? employeesResult.value : [])
    setEnterpriseUserGroups(
      groupsResult.status === "fulfilled"
        ? groupsResult.value.filter((item) => item.tenant_id === nextTenantID)
        : []
    )
    setEnterpriseSyncJobs(syncJobsResult.status === "fulfilled" ? syncJobsResult.value : [])
  }

  async function loadWalletTenantAggregates(tenantItems: Tenant[]) {
    if (tenantItems.length === 0) {
      setTenantAggregates([])
      setAggregateWarning("")
      return
    }

    const metricsQuery = buildMetricsQueryOptions()
    const settled = await Promise.allSettled(
      tenantItems.map(async (tenant) => {
        const data = await getWalletJobMetrics(token, {
          tenant_id: tenant.id,
          ...metricsQuery,
        })
        return {
          tenantID: tenant.id,
          tenantName: tenant.name,
          total: data.summary.total,
          failed: data.summary.failed,
          dlq: data.summary.dlq,
          retryableFailed: data.summary.retryable_failed,
          alertCount: data.alerts?.length ?? 0,
          updatedAt: data.updated_at,
        } satisfies WalletTenantAggregateRow
      })
    )

    const rows: WalletTenantAggregateRow[] = []
    let failedCount = 0
    for (const item of settled) {
      if (item.status === "fulfilled") {
        rows.push(item.value)
        continue
      }
      failedCount++
    }
    rows.sort((a, b) => {
      if (a.dlq !== b.dlq) {
        return b.dlq - a.dlq
      }
      if (a.failed !== b.failed) {
        return b.failed - a.failed
      }
      return a.tenantName.localeCompare(b.tenantName)
    })
    setTenantAggregates(rows)
    if (failedCount > 0) {
      setAggregateWarning(`跨租户聚合有 ${failedCount} 个租户数据拉取失败`)
    } else {
      setAggregateWarning("")
    }
  }

  async function saveAlertSubscription() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }

    setSavingSubscription(true)
    setError("")
    try {
      const updated = await upsertWalletJobAlertSubscription(token, {
        tenant_id: nextTenantID,
        enabled: subscriptionEnabled,
        dlq_alert_threshold: parsePositiveInt(subscriptionThreshold) ?? subscription?.dlq_alert_threshold ?? 20,
        window_seconds: parsePositiveInt(subscriptionWindowSeconds) ?? subscription?.window_seconds ?? 900,
        cooldown_seconds: parseNonNegativeInt(subscriptionCooldownSeconds) ?? subscription?.cooldown_seconds ?? 900,
        channels: {
          email: subscriptionEmailEnabled,
          whatsapp: subscriptionWhatsAppEnabled,
        },
        receiver_groups: parseReceiverGroups(subscriptionReceiverGroups),
        actor: "web_admin.wallet",
      })
      setSubscription(updated)
      syncSubscriptionDraft(updated)
    } catch (err) {
      const message = err instanceof Error ? err.message : "保存发放告警订阅策略失败"
      setError(message)
    } finally {
      setSavingSubscription(false)
    }
  }

  async function submitTemplate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }
    if (!templateName.trim()) {
      setError("请输入模板名称")
      return
    }

    setCreatingTemplate(true)
    setIssuanceSummary("")
    setError("")
    try {
      const created = await createWalletTemplate(token, {
        tenant_id: nextTenantID,
        name: templateName.trim(),
        pass_type: templatePassType,
        class_id: templateClassID.trim() || undefined,
        style_config: parseStyleConfig(templateStyleConfig),
        status: templateStatus,
        actor: "web_admin.wallet.template",
      })
      setIssuanceSummary(`已创建模板“${created.name}”，可立即用于${passTypeLabel(created.pass_type)}发放。`)
      setTemplateName("")
      setTemplateClassID("")
      setTemplateStyleConfig("")
      setTemplateStatus(defaultTemplateStatus)
      setTemplatePassType(defaultTemplatePassType)
      await loadWalletOps(nextTenantID)
      if (!singleTemplateID) {
        setSingleTemplateID(created.id)
      }
      if (!batchTemplateID) {
        setBatchTemplateID(created.id)
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "创建发放模板失败"
      setError(message)
    } finally {
      setCreatingTemplate(false)
    }
  }

  async function toggleTemplateStatus(template: WalletPassTemplate) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }

    setUpdatingTemplateID(template.id)
    setIssuanceSummary("")
    setError("")
    try {
      const nextStatus = template.status === "active" ? "inactive" : "active"
      const updated = await updateWalletTemplateStatus(token, template.id, {
        tenant_id: nextTenantID,
        status: nextStatus,
        actor: "web_admin.wallet.template.status",
      })
      setIssuanceSummary(`模板“${updated.name}”已切换为${updated.status === "active" ? "启用" : "停用"}状态。`)
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "更新发放模板状态失败"
      setError(message)
    } finally {
      setUpdatingTemplateID("")
    }
  }

  async function submitSingleIssue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }
    if (!singleTemplateID.trim()) {
      setError("请先选择发放模板")
      return
    }
    if (!singleTargetID.trim()) {
      setError("请输入员工或访客 ID")
      return
    }

    setIssuingSingle(true)
    setIssuanceSummary("")
    setError("")
    try {
      const targetType = resolveTargetType(singleTemplateID)
      const pass = await issueWalletPass(token, {
        tenant_id: nextTenantID,
        template_id: singleTemplateID,
        target_type: targetType,
        target_id: singleTargetID.trim(),
        expires_at: normalizeDateTimeInput(singleExpiresAt),
        actor: "web_admin.wallet.issue.single",
      })
      setIssuanceSummary(
        `${targetTypeLabel(pass.target_type)} ${pass.target_id} 的凭证已发放，当前状态为${passStatusLabel(pass.status)}。`
      )
      setSingleTargetID("")
      setSingleExpiresAt("")
      setLastIssuedJobs([])
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "单次发放凭证失败"
      setError(message)
    } finally {
      setIssuingSingle(false)
    }
  }

  async function submitBatchIssue(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }
    if (!batchTemplateID.trim()) {
      setError("请先选择批量发放模板")
      return
    }

    const targetIDs = parseTargetIDs(batchTargetIDs)
    if (targetIDs.length === 0) {
      setError("请至少输入一个员工或访客 ID")
      return
    }

    setIssuingBatch(true)
    setIssuanceSummary("")
    setError("")
    try {
      const targetType = resolveTargetType(batchTemplateID)
      const result = await issueWalletPassBatch(token, {
        tenant_id: nextTenantID,
        template_id: batchTemplateID,
        target_type: targetType,
        target_ids: targetIDs,
        expires_at: normalizeDateTimeInput(batchExpiresAt),
        execution_mode: batchExecutionMode,
        actor: "web_admin.wallet.issue.batch",
      })
      setLastIssuedJobs(result.items)
      setIssuanceSummary(
        `已提交 ${targetIDs.length} 个${targetTypeLabel(targetType)}对象，执行模式为 ${result.execution_mode}。`
      )
      setBatchTargetIDs("")
      setBatchExpiresAt("")
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "批量发放凭证失败"
      setError(message)
    } finally {
      setIssuingBatch(false)
    }
  }

  async function submitPassDelivery(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }
    if (!deliveryPassID.trim()) {
      setError("请选择需要发送的凭证")
      return
    }

    const channels: string[] = []
    if (deliveryEmailEnabled) {
      channels.push("email")
    }
    if (deliveryWhatsAppEnabled) {
      channels.push("whatsapp")
    }
    if (channels.length === 0) {
      setError("请至少选择一个投递通道")
      return
    }

    setDispatchingDelivery(true)
    setIssuanceSummary("")
    setError("")
    try {
      const created = await dispatchWalletPassDelivery(token, {
        tenant_id: nextTenantID,
        pass_id: deliveryPassID,
        channels,
        email_recipients: deliveryEmailEnabled ? parseReceiverValues(deliveryEmailRecipients) : [],
        whatsapp_recipients: deliveryWhatsAppEnabled ? parseReceiverValues(deliveryWhatsAppRecipients) : [],
        actor: "web_admin.wallet.delivery.dispatch",
      })
      setIssuanceSummary(
        `${created.target_id} 的外部投递已提交，当前结果为${deliveryNotificationStatusLabel(created.status)}${created.reason ? `（${created.reason}）` : ""}。`
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "发送外部投递失败"
      setError(message)
    } finally {
      setDispatchingDelivery(false)
    }
  }

  async function retryDeliveryNotification(notificationID: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }

    setRetryingDeliveryNotificationID(notificationID)
    setIssuanceSummary("")
    setError("")
    try {
      const retried = await retryWalletPassDelivery(token, {
        tenant_id: nextTenantID,
        notification_id: notificationID,
        actor: "web_admin.wallet.delivery.retry",
      })
      setIssuanceSummary(
        `${retried.target_id} 的失败通道已重发，当前结果为${deliveryNotificationStatusLabel(retried.status)}${retried.reason ? `（${retried.reason}）` : ""}。`
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "重发外部投递失败"
      setError(message)
    } finally {
      setRetryingDeliveryNotificationID("")
    }
  }

  async function retryDeliveryNotificationBatch() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }
    if (batchRetryableDeliveryNotifications.length === 0) {
      setIssuanceSummary("当前没有可批量重发的失败通道。")
      return
    }

    setBatchRetryingDelivery(true)
    setError("")
    setIssuanceSummary("")
    try {
      const settled = await Promise.allSettled(
        batchRetryableDeliveryNotifications.map((item) =>
          retryWalletPassDelivery(token, {
            tenant_id: nextTenantID,
            notification_id: item.id,
            actor: "web_admin.wallet.delivery.retry.batch",
          })
        )
      )
      const successCount = settled.filter((item) => item.status === "fulfilled").length
      const failedCount = settled.length - successCount
      setIssuanceSummary(
        `已批量重发 ${settled.length} 条失败通道，成功 ${successCount} 条${failedCount > 0 ? `，失败 ${failedCount} 条` : ""}。`
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "批量重发失败通道失败"
      setError(message)
    } finally {
      setBatchRetryingDelivery(false)
    }
  }

  function seedBatchReissueFromRetryableDelivery() {
    if (reissueTargetIDsByRetryableDelivery.length === 0) {
      setIssuanceSummary("当前没有可写入批量补发草稿的失败对象。")
      return
    }
    if (!reissueTemplateByRetryableDelivery) {
      setError("当前缺少可用模板，请先创建并启用员工或访客模板后再补发。")
      return
    }

    const scenarioPreset = walletIssuanceScenarioPresetByID.get(inferTemplateScenario(reissueTemplateByRetryableDelivery))
    const templateTargetType = reissueTemplateByRetryableDelivery.pass_type === "visitor" ? "visitor" : "user"
    setBatchTemplateID(reissueTemplateByRetryableDelivery.id)
    setBatchTargetIDs(reissueTargetIDsByRetryableDelivery.join("\n"))
    setBatchExecutionMode(scenarioPreset?.recommendedExecutionMode ?? defaultBatchExecutionMode)
    setPassTargetTypeFilter(templateTargetType)
    setPassTemplateFilter(reissueTemplateByRetryableDelivery.id)
    setError("")
    setIssuanceSummary(
      `已将 ${reissueTargetIDsByRetryableDelivery.length} 个失败对象写入批量补发草稿，并预选模板“${reissueTemplateByRetryableDelivery.name}”。${
        reissueTemplateByRetryableDelivery.status === "active" ? "可直接提交批量发放。" : "当前模板未启用，建议先启用模板后再提交。"
      }`
    )
  }

  async function repairRetryableDeliveryPassStatusBatch() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }
    if (repairableRetryableDeliveryPasses.length === 0) {
      setIssuanceSummary("当前没有需要修复状态的失败对象凭证。")
      return
    }

    setRepairingRetryablePasses(true)
    setError("")
    setIssuanceSummary("")
    try {
      const settled = await Promise.allSettled(
        repairableRetryableDeliveryPasses.map((item) =>
          activateWalletPass(token, item.id, {
            tenant_id: nextTenantID,
            actor: "web_admin.wallet.pass.batch.repair_from_delivery",
          })
        )
      )
      const successCount = settled.filter((item) => item.status === "fulfilled").length
      const failedCount = settled.length - successCount
      setIssuanceSummary(
        `已按失败回执批量修复 ${settled.length} 张凭证状态，成功 ${successCount} 张${
          failedCount > 0 ? `，失败 ${failedCount} 张` : ""
        }。`
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "批量修复失败对象凭证状态失败"
      setError(message)
    } finally {
      setRepairingRetryablePasses(false)
    }
  }

  function keepMissingEnterpriseTargetsInBatchDraft() {
    if (enterpriseBatchTargetStats.missingIDs.length === 0) {
      setIssuanceSummary("来源：企业页。当前预填对象已全部命中既有凭证，可直接做状态修复、重发或回企业页核对异常。")
      return
    }
    setBatchTargetIDs(enterpriseBatchTargetStats.missingIDs.join("\n"))
    setError("")
    setIssuanceSummary(
      `来源：企业页。已将未命中的 ${enterpriseBatchTargetStats.missingIDs.length} 个对象写入批量发放草稿，可直接继续补发。`
    )
  }

  function keepIssueReadyEnterpriseTargetsInBatchDraft() {
    if (issueReadyEnterpriseMissingTargetIDs.length === 0) {
      setIssuanceSummary("来源：企业页。当前未命中对象里没有可直接补发对象，请先回目录或审批与异常处理后再重试。")
      return
    }
    setBatchTargetIDs(issueReadyEnterpriseMissingTargetIDs.join("\n"))
    setError("")
    setIssuanceSummary(
      `来源：企业页。已筛出 ${issueReadyEnterpriseMissingTargetIDs.length} 个可直接补发对象并写入批量发放草稿。`
    )
  }

  function restoreEnterpriseTargetsToBatchDraft() {
    if (enterpriseBatchTargetStats.targetIDs.length === 0) {
      return
    }
    setBatchTargetIDs(enterpriseBatchTargetStats.targetIDs.join("\n"))
    setError("")
    setIssuanceSummary(
      `来源：企业页。已恢复全部 ${enterpriseBatchTargetStats.targetIDs.length} 个预填对象到批量发放草稿。`
    )
  }

  async function submitPhysicalCardTask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }
    if (!physicalTaskPassID.trim()) {
      setError("请选择要联动的员工凭证")
      return
    }

    setCreatingPhysicalCardTask(true)
    setIssuanceSummary("")
    setError("")
    try {
      const created = await createWalletPhysicalCardTask(token, {
        tenant_id: nextTenantID,
        pass_id: physicalTaskPassID,
        task_type: physicalTaskType,
        card_number: physicalTaskCardNumber.trim() || undefined,
        note: physicalTaskNote.trim() || undefined,
        actor: "web_admin.wallet.physical_card.create",
      })
      setIssuanceSummary(
        `${created.target_id} 的${physicalCardTaskTypeLabel(created.task_type)}任务已创建，当前状态为${physicalCardTaskStatusLabel(created.status)}。`
      )
      setPhysicalTaskCardNumber("")
      setPhysicalTaskNote("")
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "创建实体卡任务失败"
      setError(message)
    } finally {
      setCreatingPhysicalCardTask(false)
    }
  }

  async function advancePhysicalCardTask(task: WalletPhysicalCardTask, status: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }

    setUpdatingPhysicalCardTaskID(task.id)
    setIssuanceSummary("")
    setError("")
    try {
      const updated = await updateWalletPhysicalCardTaskStatus(token, task.id, {
        tenant_id: nextTenantID,
        status,
        card_number: task.card_number,
        note: task.note,
        actor: `web_admin.wallet.physical_card.${status}`,
      })
      setIssuanceSummary(
        `${updated.target_id} 的${physicalCardTaskTypeLabel(updated.task_type)}任务已更新为${physicalCardTaskStatusLabel(updated.status)}。`
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "更新实体卡任务失败"
      setError(message)
    } finally {
      setUpdatingPhysicalCardTaskID("")
    }
  }

  async function updatePassStatus(pass: WalletPassInstance, action: "activate" | "suspend" | "revoke") {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }

    setUpdatingPassID(pass.id)
    setIssuanceSummary("")
    setError("")
    try {
      const payload = {
        tenant_id: nextTenantID,
        actor: `web_admin.wallet.pass.${action}`,
      }
      const updated =
        action === "activate"
          ? await activateWalletPass(token, pass.id, payload)
          : action === "suspend"
            ? await suspendWalletPass(token, pass.id, payload)
            : await revokeWalletPass(token, pass.id, payload)
      setIssuanceSummary(
        `${targetTypeLabel(updated.target_type)} ${updated.target_id} 的凭证已更新为${passStatusLabel(updated.status)}。`
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "更新凭证状态失败"
      setError(message)
    } finally {
      setUpdatingPassID("")
    }
  }

  function canApplyPassAction(pass: WalletPassInstance, action: "activate" | "suspend" | "revoke") {
    switch (action) {
      case "activate":
        return pass.status === "issued" || pass.status === "suspended"
      case "suspend":
        return pass.status === "issued" || pass.status === "active"
      case "revoke":
        return pass.status !== "revoked"
      default:
        return false
    }
  }

  function onSelectPass(passID: string, checked: boolean) {
    setSelectedPassIDs((current) => {
      if (checked) {
        if (current.includes(passID)) {
          return current
        }
        return [...current, passID]
      }
      return current.filter((item) => item !== passID)
    })
  }

  function onSelectAllVisiblePasses(checked: boolean) {
    const visiblePassIDs = filteredPasses.map((item) => item.id)
    if (visiblePassIDs.length === 0) {
      return
    }
    setSelectedPassIDs((current) => {
      if (!checked) {
        const removable = new Set(visiblePassIDs)
        return current.filter((item) => !removable.has(item))
      }
      const merged = new Set(current)
      visiblePassIDs.forEach((item) => merged.add(item))
      return Array.from(merged)
    })
  }

  async function updateSelectedPasses(action: "activate" | "suspend" | "revoke") {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }

    const targetPasses = filteredPasses.filter(
      (item) => selectedPassIDSet.has(item.id) && canApplyPassAction(item, action)
    )
    if (targetPasses.length === 0) {
      setError("当前没有可执行该操作的已选凭证")
      return
    }

    setBatchUpdatingPassAction(action)
    setIssuanceSummary("")
    setError("")
    try {
      const settled = await Promise.allSettled(
        targetPasses.map((pass) => {
          const payload = {
            tenant_id: nextTenantID,
            actor: `web_admin.wallet.pass.batch.${action}`,
          }
          if (action === "activate") {
            return activateWalletPass(token, pass.id, payload)
          }
          if (action === "suspend") {
            return suspendWalletPass(token, pass.id, payload)
          }
          return revokeWalletPass(token, pass.id, payload)
        })
      )
      const succeeded = settled.filter((item) => item.status === "fulfilled").length
      const failed = settled.length - succeeded
      setSelectedPassIDs([])
      setIssuanceSummary(`批量${action === "activate" ? "激活" : action === "suspend" ? "暂停" : "吊销"}完成：成功 ${succeeded}，失败 ${failed}。`)
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "批量更新凭证状态失败"
      setError(message)
    } finally {
      setBatchUpdatingPassAction("")
    }
  }

  async function dispatchAlertsNow() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }

    setDispatchingAlerts(true)
    setDispatchSummary("")
    setError("")
    try {
      const result: WalletJobAlertDispatchResult = await dispatchWalletJobAlerts(token, {
        tenant_id: nextTenantID,
        window_seconds: parsePositiveInt(windowSeconds),
        max_retry: parseNonNegativeInt(maxRetry),
        dlq_alert_threshold: parsePositiveInt(alertThreshold),
        actor: "web_admin.wallet.dispatch",
      })
      setDispatchSummary(
        `本次评估 ${result.total_alerts} 条告警，发送 ${result.dispatched}，跳过 ${result.skipped}，失败 ${result.failed}`
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "触发发放告警发送失败"
      setError(message)
    } finally {
      setDispatchingAlerts(false)
    }
  }

  async function retryAlertNotification(notificationID: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError("请选择租户")
      return
    }

    setRetryingAlertNotificationID(notificationID)
    setError("")
    try {
      const result = await retryWalletJobAlertNotification(token, {
        tenant_id: nextTenantID,
        notification_id: notificationID,
        actor: "web_admin.wallet.dispatch.retry",
      })
      setDispatchSummary(`重试结果：${result.status}${result.reason ? ` (${result.reason})` : ""}`)
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : "重试发放告警发送失败"
      setError(message)
    } finally {
      setRetryingAlertNotificationID("")
    }
  }

  async function applyFilters(nextTenantID?: string) {
    const effectiveTenantID = (nextTenantID ?? tenantID).trim()
    if (!effectiveTenantID) {
      setError("请选择租户")
      return
    }
    setRefreshing(true)
    setError("")
    try {
      await Promise.all([
        loadWalletOps(effectiveTenantID),
        ...(platformViewer ? [loadWalletTenantAggregates(tenants)] : []),
      ])
    } catch (err) {
      const message = err instanceof Error ? err.message : "刷新凭证发放数据失败"
      setError(message)
    } finally {
      setRefreshing(false)
    }
  }

  function onTenantChange(value: string) {
    setTenantID(value)
    void applyFilters(value)
  }

  function focusPassScenario(scenarioID: WalletScenarioKind) {
    const preset = walletIssuanceScenarioPresetByID.get(scenarioID)
    if (!preset) {
      return
    }
    const activeTemplate = activeTemplateByScenario.get(scenarioID)
    setPassQuery("")
    setPassStatusFilter("all")
    setPassTargetTypeFilter(preset.targetType)
    setPassTemplateFilter(activeTemplate?.id ?? "all")
    setSelectedPassIDs([])
    setIssuanceSummary(`已切到“${preset.title}”的交付台账。`)
  }

  function applyScenarioPreset(scenarioID: WalletScenarioKind) {
    const preset = walletIssuanceScenarioPresetByID.get(scenarioID)
    if (!preset) {
      return
    }
    const activeTemplate = activeTemplateByScenario.get(scenarioID)
    setTemplateName(preset.templateName)
    setTemplatePassType(preset.passType)
    setTemplateClassID(preset.classID)
    setTemplateStyleConfig(stringifyStyleConfig(preset.styleConfig))
    setTemplateStatus(defaultTemplateStatus)
    setBatchExecutionMode(preset.recommendedExecutionMode)
    setSingleTargetID("")
    setBatchTargetIDs("")
    setSelectedPassIDs([])
    setPassQuery("")
    setPassStatusFilter("all")
    setPassTargetTypeFilter(preset.targetType)
    setPassTemplateFilter(activeTemplate?.id ?? "all")
    if (activeTemplate) {
      setSingleTemplateID(activeTemplate.id)
      setBatchTemplateID(activeTemplate.id)
    } else {
      setSingleTemplateID("")
      setBatchTemplateID("")
    }
    if (typeof preset.defaultExpiresInHours === "number") {
      const expiresAt = buildRelativeDateTimeInput(preset.defaultExpiresInHours)
      setSingleExpiresAt(expiresAt)
      setBatchExpiresAt(expiresAt)
    } else {
      setSingleExpiresAt("")
      setBatchExpiresAt("")
    }
    setIssuanceSummary(
      activeTemplate
        ? `已切换到“${preset.title}”场景，模板表单已写入推荐字段，并已对准现有模板“${activeTemplate.name}”。`
        : `已套用“${preset.title}”场景预设，请确认模板名称、class_id 和 style_config 后创建模板。`
    )
  }

  async function copySaveLink(pass: WalletPassInstance) {
    if (!pass.save_link) {
      return
    }
    if (typeof navigator === "undefined" || !navigator.clipboard?.writeText) {
      setError("当前环境不支持复制保存链接，请直接打开链接。")
      return
    }
    try {
      await navigator.clipboard.writeText(pass.save_link)
      setIssuanceSummary(`已复制 ${pass.target_id} 的保存链接，可直接发送给用户。`)
      setError("")
    } catch (err) {
      const message = err instanceof Error ? err.message : "复制保存链接失败"
      setError(message)
    }
  }

  function patchPassRecord(passID: string, updater: (current: WalletPassInstance) => WalletPassInstance) {
    setPasses((current) => current.map((item) => (item.id === passID ? updater(item) : item)))
  }

  async function refreshPassRecord(pass: WalletPassInstance) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      throw new Error("请选择租户")
    }
    const latest = await getWalletPass(token, pass.id, nextTenantID)
    patchPassRecord(pass.id, () => latest)
    return latest
  }

  async function resolvePassSaveLink(pass: WalletPassInstance) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      throw new Error("请选择租户")
    }
    if (pass.save_link) {
      return pass.save_link
    }
    const latest = await refreshPassRecord(pass)
    if (latest.save_link) {
      return latest.save_link
    }
    const link = await getWalletPassSaveLink(token, pass.id, nextTenantID)
    patchPassRecord(pass.id, (current) => ({ ...current, save_link: link }))
    return link
  }

  async function refreshPassSaveLink(pass: WalletPassInstance) {
    setResolvingSaveLinkPassID(pass.id)
    setError("")
    try {
      const link = await resolvePassSaveLink(pass)
      setIssuanceSummary(`已刷新 ${pass.target_id} 的保存链接，可继续复制、预览二维码或发送给用户。`)
      if (qrDialogPass?.id === pass.id) {
        setQrDialogSaveLink(link)
        const svg = await QRCode.toString(link, {
          type: "svg",
          width: 280,
          margin: 1,
          color: { dark: "#0f172a", light: "#ffffff" },
        })
        setQrDialogSVG(svg)
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "刷新保存链接失败"
      setError(message)
    } finally {
      setResolvingSaveLinkPassID("")
    }
  }

  async function openPassQrDialog(pass: WalletPassInstance) {
    setQrDialogOpen(true)
    setQrDialogPass(pass)
    setQrDialogSaveLink("")
    setQrDialogSVG("")
    setQrDialogLoading(true)
    setError("")
    try {
      let latest = pass
      if (!latest.save_link) {
        latest = await refreshPassRecord(pass)
      }
      const link = latest.save_link || (await getWalletPassSaveLink(token, pass.id, tenantID.trim()))
      if (!latest.save_link) {
        latest = { ...latest, save_link: link }
        patchPassRecord(pass.id, () => latest)
      }
      const svg = await QRCode.toString(link, {
        type: "svg",
        width: 280,
        margin: 1,
        color: { dark: "#0f172a", light: "#ffffff" },
      })
      setQrDialogPass(latest)
      setQrDialogSaveLink(link)
      setQrDialogSVG(svg)
    } catch (err) {
      const message = err instanceof Error ? err.message : "生成二维码失败"
      setQrDialogOpen(false)
      setQrDialogPass(null)
      setError(message)
    } finally {
      setQrDialogLoading(false)
    }
  }

  function downloadQrSVG() {
    if (!qrDialogSVG || !qrDialogPass) {
      return
    }
    const blob = new Blob([qrDialogSVG], { type: "image/svg+xml;charset=utf-8" })
    const objectURL = URL.createObjectURL(blob)
    const anchor = document.createElement("a")
    anchor.href = objectURL
    anchor.download = `${qrDialogPass.target_id || qrDialogPass.id}-mistypass-qr.svg`
    document.body.append(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(objectURL)
  }

  const alertItems = metrics?.alerts ?? []
  const singleTargetType = selectedSingleTemplate?.pass_type === "visitor" ? "visitor" : "user"
  const batchTargetType = selectedBatchTemplate?.pass_type === "visitor" ? "visitor" : "user"
  const effectiveError = error || queryError

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-1">
        <p className="mp-page-eyebrow">凭证发放</p>
        <h1 className="mp-page-title">MistyPass 发放与状态</h1>
        <p className="mp-page-description">
          统一查看 MistyPass 的发放状态、阈值告警和异常记录；移动凭证、二维码、临时证等渠道都收口在这一页。
        </p>
      </div>

      {!writable ? (
        <div className="rounded-lg border bg-muted/15 px-4 py-3 text-sm text-muted-foreground">
          当前角色为只读视图，可查看发放状态、投递回执和告警趋势，但不能执行新建、重发、状态更新和策略保存。{readOnlyBoundaryHint}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {[
          {
            title: "员工发放",
            description: "面向常驻员工的长期凭证，默认走统一的组织发放策略。",
            icon: WalletCardsIcon,
          },
          {
            title: "访客与临时证",
            description: "短时访问、二维码、临时证和补发都沿用同一套状态与审计。",
            icon: MessageCircleIcon,
          },
          {
            title: "批量补发",
            description: "失败重试、批量重发和状态刷新都在这里收口，而不是分散到渠道页。",
            icon: RefreshCwIcon,
          },
          {
            title: "异常处理",
            description: "阈值告警、通知记录和清理能力保留在同页，但下沉到运行视图。",
            icon: ShieldAlertIcon,
          },
        ].map((item) => (
          <Card key={item.title}>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center justify-between text-base">
                {item.title}
                <item.icon className="size-4 text-muted-foreground" />
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">{item.description}</CardContent>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">运行窗口与告警参数</CardTitle>
          <CardDescription>
            {platformViewer
              ? "支持按租户切换并覆盖默认窗口/阈值参数；这里配置的是运行窗口，不是发放方式。"
              : "当前组织的发放运行参数与状态窗口；这里配置的是运行窗口，不是发放方式。"}
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2 xl:grid-cols-7">
          {platformViewer ? (
            <Select value={tenantID} onValueChange={onTenantChange}>
              <SelectTrigger>
                <SelectValue placeholder="租户" />
              </SelectTrigger>
              <SelectContent>
                {tenants.map((item) => (
                  <SelectItem key={item.id} value={item.id}>
                    {item.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <Input value={tenantID} readOnly />
          )}

          <Input
            value={windowSeconds}
            onChange={(event) => setWindowSeconds(event.target.value)}
            placeholder="window_seconds"
          />
          <Input
            value={maxRetry}
            onChange={(event) => setMaxRetry(event.target.value)}
            placeholder="max_retry（留空=默认）"
          />
          <Input
            value={alertThreshold}
            onChange={(event) => setAlertThreshold(event.target.value)}
            placeholder="dlq_alert_threshold（留空=默认）"
          />
          <Input
            value={archiveLimit}
            onChange={(event) => setArchiveLimit(event.target.value)}
            placeholder="archive limit"
          />
          <Input
            value={trendBucketCount}
            onChange={(event) => setTrendBucketCount(event.target.value)}
            placeholder="trend bucket_count"
          />

          <Button onClick={() => void applyFilters()} disabled={loading || refreshing}>
            <RefreshCwIcon className={`mr-1.5 size-4 ${refreshing ? "animate-spin" : ""}`} />
            刷新
          </Button>
        </CardContent>
      </Card>

      <Tabs defaultValue="operations" className="space-y-4">
        <div className="rounded-xl border bg-muted/15 p-1">
          <TabsList className="grid h-auto w-full grid-cols-2 bg-transparent">
            <TabsTrigger value="operations" className="py-2.5">
              发放操作
            </TabsTrigger>
            <TabsTrigger value="advanced" className="py-2.5">
              高级运行
            </TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="operations" className="space-y-4">
          <div className="rounded-xl border bg-muted/15 px-4 py-3">
            <p className="text-sm font-medium">围绕 MistyPass 发放的主操作台</p>
            <p className="mt-1 text-sm text-muted-foreground">
              先维护模板，再完成员工、访客和批量发放，最后在凭证列表里统一暂停、恢复和吊销。用户只需要关心
              “发放 MistyPass”，不需要再理解 Wallet、蓝牙、实体卡或二维码的底层差异。
            </p>
          </div>

          <div className="grid gap-4 xl:grid-cols-4">
            {walletIssuanceScenarioPresets.map((item) => {
              const activeTemplate = activeTemplateByScenario.get(item.id)
              return (
                <Card key={item.id}>
                  <CardHeader className="pb-3">
                    <div className="flex items-start justify-between gap-3">
                      <div className="space-y-1">
                        <CardTitle className="text-base">{item.title}</CardTitle>
                        <CardDescription>{item.description}</CardDescription>
                      </div>
                      <Badge variant="outline">{item.passType === "employee" ? "员工" : "访客"}</Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-3">
                    <div className="rounded-lg border bg-muted/10 px-3 py-2 text-xs text-muted-foreground">
                      模板 {templateScenarioCounts[item.id]} 个 · 已发放 {passScenarioCounts[item.id]} 张
                    </div>
                    <p className="text-sm text-muted-foreground">
                      {activeTemplate
                        ? `已启用模板：${activeTemplate.name}。可直接切到这一场景开始单发、批发或状态维护。`
                        : "当前还没有启用模板，建议先套用预设，再创建模板后开始发放。"}
                    </p>
                    <div className="flex flex-wrap items-center gap-2">
                      <Button size="sm" variant="outline" onClick={() => applyScenarioPreset(item.id)}>
                        {activeTemplate ? "切到此场景" : "套用预设"}
                      </Button>
                      {item.id === "visitor_temporary" ? (
                        <Button asChild size="sm" variant="outline">
                          <Link to="/access/grants">临时授权台账</Link>
                        </Button>
                      ) : null}
                    </div>
                  </CardContent>
                </Card>
              )
            })}
          </div>

          <div className="grid gap-4 xl:grid-cols-3">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">员工长期发放</CardTitle>
                <CardDescription>面向在职员工的常驻凭证，适合入职开通、补发和批量激活。</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  {activeEmployeeTemplate
                    ? `已就绪模板：${activeEmployeeTemplate.name}，当前共有 ${employeePassCount} 张员工凭证。`
                    : "还没有启用中的员工模板，建议先创建员工模板后再开始单发或批发。"}
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!activeEmployeeTemplate}
                    onClick={() => {
                      if (!activeEmployeeTemplate) {
                        return
                      }
                      setSingleTemplateID(activeEmployeeTemplate.id)
                      setBatchTemplateID(activeEmployeeTemplate.id)
                      setPassTargetTypeFilter("user")
                      setPassTemplateFilter(activeEmployeeTemplate.id)
                      setPassStatusFilter("all")
                      setPassQuery("")
                    }}
                  >
                    {activeEmployeeTemplate ? "使用员工模板" : "先创建员工模板"}
                  </Button>
                  <Button asChild size="sm" variant="outline">
                    <Link to="/enterprise#sync">去同步员工目录</Link>
                  </Button>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">访客与临时证</CardTitle>
                <CardDescription>短期访问、访客二维码和临时证都统一从访客模板或临时授权入口处理。</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  {activeVisitorTemplate
                    ? `已就绪模板：${activeVisitorTemplate.name}，当前共有 ${visitorPassCount} 张访客 / 临时证记录。`
                    : "还没有启用中的访客模板；若是一次性或短期访问，也可以直接走临时授权流程。"}
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!activeVisitorTemplate}
                    onClick={() => {
                      if (!activeVisitorTemplate) {
                        return
                      }
                      setSingleTemplateID(activeVisitorTemplate.id)
                      setBatchTemplateID(activeVisitorTemplate.id)
                      setPassTargetTypeFilter("visitor")
                      setPassTemplateFilter(activeVisitorTemplate.id)
                      setPassStatusFilter("all")
                      setPassQuery("")
                    }}
                  >
                    {activeVisitorTemplate ? "使用访客模板" : "先创建访客模板"}
                  </Button>
                  <Button asChild size="sm" variant="outline">
                    <Link to="/access/grants">去临时授权</Link>
                  </Button>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">补发与状态维护</CardTitle>
                <CardDescription>挂失、暂停、恢复和吊销都留在同一处完成，不再分散到渠道页。</CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  当前有 {suspendedPassCount} 张已暂停凭证，仍可维护状态的凭证共 {revocablePassCount} 张。
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setPassStatusFilter("suspended")
                      setPassTargetTypeFilter("all")
                      setPassTemplateFilter("all")
                      setPassQuery("")
                    }}
                  >
                    查看已暂停
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => {
                      setPassStatusFilter("all")
                      setPassTargetTypeFilter("all")
                      setPassTemplateFilter("all")
                      setPassQuery("")
                    }}
                  >
                    查看全部状态
                  </Button>
                </div>
              </CardContent>
            </Card>
          </div>

          <div className="grid gap-4 xl:grid-cols-[1.05fr_0.95fr]">
	        <Card>
	          <CardHeader>
	            <CardTitle className="text-base">发放模板</CardTitle>
	            <CardDescription>
	              模板定义发给谁、属于哪种发放场景，以及默认样式。可先点上方场景卡自动带入推荐名称、class_id 和 style_config。
	            </CardDescription>
	          </CardHeader>
	          <CardContent className="space-y-4">
            <form className="space-y-3 rounded-xl border bg-muted/15 p-4" onSubmit={submitTemplate}>
              <div className="grid gap-3 md:grid-cols-2">
                <Input
                  value={templateName}
                  disabled={!writable}
                  onChange={(event) => setTemplateName(event.target.value)}
                  placeholder="模板名称，例如：总部员工长期凭证"
                />
                <Input
                  value={templateClassID}
                  disabled={!writable}
                  onChange={(event) => setTemplateClassID(event.target.value)}
                  placeholder="class_id（可选）"
                />
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                <Select
                  value={templatePassType}
                  disabled={!writable}
                  onValueChange={(value) => setTemplatePassType(value as "employee" | "visitor")}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="模板类型" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="employee">员工凭证模板</SelectItem>
                    <SelectItem value="visitor">访客 / 临时证模板</SelectItem>
                  </SelectContent>
                </Select>

                <Select
                  value={templateStatus}
                  disabled={!writable}
                  onValueChange={(value) => setTemplateStatus(value as "active" | "inactive")}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="模板状态" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="active">启用</SelectItem>
                    <SelectItem value="inactive">停用</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <Textarea
                value={templateStyleConfig}
                disabled={!writable}
                onChange={(event) => setTemplateStyleConfig(event.target.value)}
                placeholder={"style_config（可选，支持每行 key=value）\nbrand_color=#0f766e\nlogo_variant=light"}
                rows={4}
              />

              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="mp-kpi-note">
                  {writable
                    ? "建议先维护 1 个员工模板和 1 个访客模板，再开始单发或批发。"
                    : `当前角色为只读，可查看模板与发放状态，但不能新建或修改模板。${readOnlyBoundaryHint}`}
                </p>
                <Button type="submit" disabled={!writable || creatingTemplate || loading || refreshing}>
                  {creatingTemplate ? "创建中..." : "新建模板"}
                </Button>
              </div>
            </form>

            {issuanceSummary ? (
              <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700">
                {issuanceSummary}
              </div>
            ) : null}

            <div className="space-y-3">
              {loading ? (
                <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
                  正在加载模板...
                </div>
              ) : null}
              {!loading && templates.length === 0 ? (
                <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
                  暂无模板，请先创建员工或访客模板。
                </div>
              ) : null}
              {!loading &&
                templates.map((item) => (
                  <div
                    key={item.id}
                    className="flex flex-col gap-3 rounded-xl border bg-card/80 px-4 py-3 lg:flex-row lg:items-center lg:justify-between"
                  >
                    <div className="space-y-2">
	                      <div className="flex flex-wrap items-center gap-2">
	                        <p className="font-medium">{item.name}</p>
	                        <Badge variant={templateStatusVariant(item.status)}>
	                          {item.status === "active" ? "启用中" : "已停用"}
	                        </Badge>
	                        <Badge variant="outline">{passTypeLabel(item.pass_type)}</Badge>
	                        <Badge variant="secondary">{walletScenarioLabel(inferTemplateScenario(item))}</Badge>
	                      </div>
	                      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
	                        <span>class_id: {item.class_id || "-"}</span>
                        <span>样式项: {Object.keys(item.style_config ?? {}).length}</span>
                        <span>更新时间: {formatDateTime(item.updated_at)}</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          setSingleTemplateID(item.id)
                          setBatchTemplateID(item.id)
                          setPassTemplateFilter(item.id)
                        }}
                      >
                        设为默认
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => void toggleTemplateStatus(item)}
                        disabled={!writable || updatingTemplateID === item.id}
                      >
                        {updatingTemplateID === item.id
                          ? "处理中..."
                          : item.status === "active"
                            ? "停用"
                            : "启用"}
                      </Button>
                    </div>
                  </div>
                ))}
            </div>
          </CardContent>
        </Card>

        <Card>
	          <CardHeader>
	            <CardTitle className="text-base">立即发放</CardTitle>
	            <CardDescription>
	              先选模板，再发给员工或访客。模板场景会自动决定这是员工移动凭证、实体卡联动、访客二维码还是临时证。
	            </CardDescription>
	          </CardHeader>
	          <CardContent className="space-y-4">
            {enterpriseBatchTargetStats.targetIDs.length > 0 ? (
              <div className="rounded-xl border bg-muted/10 px-4 py-3">
                <div className="flex flex-col gap-2">
                  <div className="space-y-0.5">
                    <p className="text-sm font-medium">
                      企业回流对象预填命中率 {enterpriseBatchTargetStats.hitRate}%
                    </p>
                    <p className="mp-kpi-note">
                      预填 {enterpriseBatchTargetStats.targetIDs.length} 个对象，已命中既有凭证 {enterpriseBatchTargetStats.matchedIDs.length} 个，未命中 {enterpriseBatchTargetStats.missingIDs.length} 个。
                    </p>
                    {enterpriseBatchTargetStats.missingIDs.length > 0 ? (
                      <p className="mp-kpi-note">
                        未命中对象中：可直接补发 {enterpriseMissingTargetBreakdown.issueReadyCount} 个，需回目录复核 {enterpriseMissingTargetBreakdown.needsDirectoryCount} 个，需回审批与异常处理 {enterpriseMissingTargetBreakdown.needsAlertsCount} 个。
                      </p>
                    ) : null}
                    {enterpriseSyncIssueHint ? (
                      <p className="text-xs text-amber-700">{enterpriseSyncIssueHint}</p>
                    ) : null}
                  </div>
                  {enterpriseMissingTargetBreakdown.rows.length > 0 ? (
                    <div className="rounded-lg border bg-background px-3 py-2">
                      <p className="text-xs font-medium">未命中对象定位（最多展示 3 条）</p>
                      <div className="mt-1 space-y-1">
                        {enterpriseMissingTargetBreakdown.rows.slice(0, 3).map((item) => (
                          <p key={item.targetID} className="mp-kpi-note">
                            {item.targetID}
                            {item.employeeName ? `（${item.employeeName}）` : ""} · {item.reason}
                            {item.groupLabel !== "-" ? ` · 用户组 ${item.groupLabel}` : ""}
                            {item.sourceLabel !== "-" ? ` · 来源 ${item.sourceLabel}` : ""}
                          </p>
                        ))}
                      </div>
                    </div>
                  ) : null}
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={!writable || loading || refreshing || issueReadyEnterpriseMissingTargetIDs.length === 0}
                      onClick={keepIssueReadyEnterpriseTargetsInBatchDraft}
                    >
                      {`仅保留可直接补发对象（${issueReadyEnterpriseMissingTargetIDs.length}）`}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={!writable || loading || refreshing || enterpriseBatchTargetStats.missingIDs.length === 0}
                      onClick={keepMissingEnterpriseTargetsInBatchDraft}
                    >
                      {`仅保留未命中对象（${enterpriseBatchTargetStats.missingIDs.length}）`}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={!writable || loading || refreshing}
                      onClick={restoreEnterpriseTargetsToBatchDraft}
                    >
                      {`恢复全部预填对象（${enterpriseBatchTargetStats.targetIDs.length}）`}
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={accessDirectoryReviewLink}>回目录复核对象来源</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={enterpriseAlertsIssueLink}>回企业页并按同步异常定位</Link>
                    </Button>
                    {hasWorkerAlertFlowHints ? (
                      <Button asChild size="sm" variant="outline">
                        <Link to={enterpriseSyncWorkerReviewLink}>处理完成后回导入与同步复核</Link>
                      </Button>
                    ) : null}
                  </div>
                </div>
              </div>
            ) : null}
            <div className="grid gap-4 xl:grid-cols-2">
              <form className="space-y-3 rounded-xl border bg-muted/15 p-4" onSubmit={submitSingleIssue}>
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium">单个发放</p>
                  <Badge variant="outline">{targetTypeLabel(singleTargetType)}</Badge>
                </div>
                <Select value={singleTemplateID} onValueChange={setSingleTemplateID}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择发放模板" />
                  </SelectTrigger>
                  <SelectContent>
                    {templates.map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Input
                  value={singleTargetID}
                  disabled={!writable}
                  onChange={(event) => setSingleTargetID(event.target.value)}
                  placeholder={singleTargetType === "visitor" ? "访客 ID，例如 visitor-001" : "员工 ID，例如 user-001"}
                />
                <Input
                  type="datetime-local"
                  value={singleExpiresAt}
                  disabled={!writable}
                  onChange={(event) => setSingleExpiresAt(event.target.value)}
                />
	                <div className="space-y-2">
	                  <p className="mp-kpi-note">
	                    {selectedSingleTemplate
	                      ? `当前模板场景：${walletScenarioLabel(inferTemplateScenario(selectedSingleTemplate))}。${walletScenarioHint(inferTemplateScenario(selectedSingleTemplate))} 留空到期时间表示按默认策略持续有效。`
	                      : "请先选择一个模板。"}
	                  </p>
	                  <Button
                    type="submit"
                    className="w-full"
                    disabled={!writable || issuingSingle || !singleTemplateID || loading || refreshing}
                  >
                    {issuingSingle ? "发放中..." : "发放 1 张凭证"}
                  </Button>
                </div>
              </form>

              <form className="space-y-3 rounded-xl border bg-muted/15 p-4" onSubmit={submitBatchIssue}>
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium">批量发放</p>
                  <Badge variant="outline">{targetTypeLabel(batchTargetType)}</Badge>
                </div>
                <Select value={batchTemplateID} onValueChange={setBatchTemplateID}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择批量模板" />
                  </SelectTrigger>
                  <SelectContent>
                    {templates.map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <div className="grid gap-3 md:grid-cols-2">
                  <Input
                    type="datetime-local"
                    value={batchExpiresAt}
                    disabled={!writable}
                    onChange={(event) => setBatchExpiresAt(event.target.value)}
                  />
                  <Select
                    value={batchExecutionMode}
                    disabled={!writable}
                    onValueChange={(value) => setBatchExecutionMode(value as "inline" | "queued")}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="执行模式" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="queued">排队执行</SelectItem>
                      <SelectItem value="inline">立即执行</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                <Textarea
                  value={batchTargetIDs}
                  disabled={!writable}
                  onChange={(event) => setBatchTargetIDs(event.target.value)}
                  placeholder="输入多个员工或访客 ID，支持换行、逗号、分号分隔"
                  rows={6}
                />
	                <div className="space-y-2">
	                  <p className="mp-kpi-note">
	                    {selectedBatchTemplate
	                      ? `当前模板场景：${walletScenarioLabel(inferTemplateScenario(selectedBatchTemplate))}。${walletScenarioHint(inferTemplateScenario(selectedBatchTemplate))}`
	                      : `当前将按 ${targetTypeLabel(batchTargetType)} 模板发放；适合补发、入职批量开通和临时证批量下发。`}
	                  </p>
	                  <Button
                    type="submit"
                    className="w-full"
                    disabled={!writable || issuingBatch || !batchTemplateID || loading || refreshing}
                  >
                    {issuingBatch ? "提交中..." : "提交批量发放"}
                  </Button>
                </div>
              </form>
            </div>

            {lastIssuedJobs.length > 0 ? (
              <div className="rounded-xl border bg-muted/10 p-4" data-testid="wallet-recent-batch-receipts">
                <div className="mb-3 flex items-center justify-between gap-2">
                  <p className="text-sm font-medium">最近批量回执</p>
                  <Badge variant="outline">{lastIssuedJobs.length} 条</Badge>
                </div>
                <div className="space-y-2">
                  {lastIssuedJobs.slice(0, 5).map((item) => (
                    <div
                      key={item.id}
                      data-testid={`wallet-recent-batch-receipt-${item.id}`}
                      className="flex flex-col gap-1 rounded-lg border bg-background px-3 py-2 text-sm lg:flex-row lg:items-center lg:justify-between"
                    >
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium" data-testid="wallet-recent-batch-target">
                          {item.target_id}
                        </span>
                        <Badge
                          variant={item.status === "success" ? "secondary" : "outline"}
                          data-testid="wallet-recent-batch-status"
                        >
                          {item.status}
                        </Badge>
                        {item.error_code ? (
                          <Badge variant="destructive" data-testid="wallet-recent-batch-error">
                            {item.error_code}
                          </Badge>
                        ) : null}
                      </div>
                      <p className="mp-kpi-note">
                        retry {item.retry_count} · {formatDateTime(item.updated_at)}
                      </p>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
          </CardContent>
        </Card>
      </div>

          <div className="grid gap-4 xl:grid-cols-[0.92fr_1.08fr]">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">交付与回执工作区</CardTitle>
                <CardDescription>
                  把实体卡联动、二维码交付、临时证发放和保存链接处理集中在这里，避免发放后还要回到别的页面查回执。
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {walletIssuanceScenarioPresets.map((item) => {
                  const activeTemplate = activeTemplateByScenario.get(item.id)
                  return (
                    <div
                      key={item.id}
                      className="flex flex-col gap-3 rounded-xl border bg-muted/10 px-4 py-3 lg:flex-row lg:items-center lg:justify-between"
                    >
                      <div className="space-y-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="font-medium">{item.title}</p>
                          <Badge variant="secondary">{passScenarioCounts[item.id]} 张</Badge>
                          <Badge variant="outline">保存链接 {saveLinkScenarioCounts[item.id]} 张</Badge>
                        </div>
                        <p className="text-sm text-muted-foreground">
                          {activeTemplate
                            ? `当前模板：${activeTemplate.name} · ${deliveryMethodLabel(getTemplateDeliveryMethod(activeTemplate))} · ${accessMediumLabel(getTemplateAccessMedium(activeTemplate))}`
                            : "当前还没有启用模板，可先回到上方套用预设并创建模板。"}
                        </p>
                        <p className="mp-kpi-note">
                          交付通道：{dispatchChannelLabels(activeTemplate)}
                        </p>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        <Button size="sm" variant="outline" onClick={() => focusPassScenario(item.id)}>
                          查看台账
                        </Button>
                        {item.id === "visitor_temporary" ? (
                          <Button asChild size="sm" variant="outline">
                            <Link to="/access/grants">去临时授权</Link>
                          </Button>
                        ) : null}
                      </div>
                    </div>
                  )
                })}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">最近交付与回执</CardTitle>
                <CardDescription>
                  最近有保存链接或需要实体卡联动的凭证都会出现在这里，便于复制链接、打开交付页或回到对应场景继续处理。
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {deliveryDeskPasses.length === 0 ? (
                  <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
                    还没有可交付的凭证。先完成发放，或切到访客二维码 / 临时证场景生成保存链接。
                  </div>
                ) : (
                  deliveryDeskPasses.map((item) => {
                    const itemTemplate = templateByID.get(item.template_id)
                    const scenario = inferPassScenario(item, itemTemplate)
                    return (
                      <div
                        key={item.id}
                        className="flex flex-col gap-3 rounded-xl border bg-card/80 px-4 py-3 lg:flex-row lg:items-center lg:justify-between"
                      >
                        <div className="space-y-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <p className="font-medium">{item.target_id}</p>
                            <Badge variant={passStatusVariant(item.status)}>{passStatusLabel(item.status)}</Badge>
                            <Badge variant="secondary">{walletScenarioLabel(scenario)}</Badge>
                          </div>
                          <p className="text-sm text-muted-foreground">{itemTemplate?.name ?? item.template_id}</p>
                          <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                            <span>{deliveryMethodLabel(getTemplateDeliveryMethod(itemTemplate))}</span>
                            <span>{accessMediumLabel(getTemplateAccessMedium(itemTemplate))}</span>
                            <span>{dispatchChannelLabels(itemTemplate)}</span>
                          </div>
                          <p className="mp-kpi-note">{deliveryHint(item, itemTemplate)}</p>
                        </div>
                        <div className="flex flex-wrap items-center gap-2">
                          <Button size="sm" variant="outline" onClick={() => focusPassScenario(scenario)}>
                            查看同类台账
                          </Button>
                          {item.save_link ? (
                            <>
                              <Button size="sm" variant="outline" onClick={() => void openPassQrDialog(item)}>
                                查看二维码
                              </Button>
                              <Button asChild size="sm" variant="outline">
                                <a href={item.save_link} rel="noreferrer" target="_blank">
                                  打开链接
                                </a>
                              </Button>
                              <Button size="sm" variant="outline" onClick={() => void copySaveLink(item)}>
                                复制链接
                              </Button>
                            </>
                          ) : (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => void refreshPassSaveLink(item)}
                              disabled={resolvingSaveLinkPassID === item.id}
                            >
                              {resolvingSaveLinkPassID === item.id ? "刷新中..." : "刷新链接"}
                            </Button>
                          )}
                        </div>
                      </div>
                    )
                  })
                )}
              </CardContent>
            </Card>
          </div>

          <div className="grid gap-4 xl:grid-cols-[0.98fr_1.02fr]">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">发送外部投递</CardTitle>
                <CardDescription>
                  把保存链接通过 Email 或 WhatsApp 发给员工、访客，并在同页查看每次发送的回执结果。
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <form className="space-y-4" onSubmit={submitPassDelivery}>
                  <Select value={deliveryPassID} onValueChange={setDeliveryPassID}>
                    <SelectTrigger>
                      <SelectValue placeholder="选择需要发送的凭证" />
                    </SelectTrigger>
                    <SelectContent>
                      {deliverablePasses.map((item) => {
                        const itemTemplate = templateByID.get(item.template_id)
                        return (
                          <SelectItem key={item.id} value={item.id}>
                            {item.target_id} · {itemTemplate?.name ?? item.template_id}
                          </SelectItem>
                        )
                      })}
                    </SelectContent>
                  </Select>

                  {selectedDeliveryPass ? (
                    <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium">{selectedDeliveryPass.target_id}</span>
                        <Badge variant={passStatusVariant(selectedDeliveryPass.status)}>
                          {passStatusLabel(selectedDeliveryPass.status)}
                        </Badge>
                        <Badge variant="secondary">
                          {walletScenarioLabel(inferPassScenario(selectedDeliveryPass, selectedDeliveryTemplate))}
                        </Badge>
                      </div>
                      <p className="mt-1 text-sm text-muted-foreground">
                        {selectedDeliveryTemplate?.name ?? selectedDeliveryPass.template_id}
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {selectedDeliveryPass.save_link
                          ? "当前凭证已具备保存链接，可直接发送到外部通道。"
                          : "当前凭证尚未拿到保存链接，建议先刷新链接后再发送。"}
                      </p>
                      <div className="mt-3 flex flex-wrap items-center gap-2">
                        {selectedDeliveryPass.save_link ? (
                          <>
                            <Button size="sm" type="button" variant="outline" onClick={() => void openPassQrDialog(selectedDeliveryPass)}>
                              查看二维码
                            </Button>
                            <Button size="sm" type="button" variant="outline" onClick={() => void copySaveLink(selectedDeliveryPass)}>
                              复制链接
                            </Button>
                          </>
                        ) : (
                          <Button
                            size="sm"
                            type="button"
                            variant="outline"
                            onClick={() => void refreshPassSaveLink(selectedDeliveryPass)}
                            disabled={resolvingSaveLinkPassID === selectedDeliveryPass.id}
                          >
                            {resolvingSaveLinkPassID === selectedDeliveryPass.id ? "刷新中..." : "刷新链接"}
                          </Button>
                        )}
                      </div>
                    </div>
                  ) : (
                    <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
                      {deliverablePasses.length === 0
                        ? "当前没有可发送的保存链接。先完成发放或刷新链接，再回来发送外部投递。"
                        : "请选择一张凭证后发送外部投递。"}
                    </div>
                  )}

                  <div className="grid gap-3 md:grid-cols-2">
                    <div className="space-y-3 rounded-xl border bg-muted/10 p-4">
                      <div className="flex items-center justify-between gap-3">
                        <div className="space-y-0.5">
                          <p className="inline-flex items-center gap-1 text-sm font-medium">
                            <MailIcon className="size-3.5" />
                            Email
                          </p>
                          <p className="mp-kpi-note">用于员工入职通知、访客邮件和补发链接。</p>
                        </div>
                        <Switch
                          checked={deliveryEmailEnabled}
                          disabled={!writable}
                          onCheckedChange={(checked) => setDeliveryEmailEnabled(checked)}
                        />
                      </div>
                      <Textarea
                        rows={4}
                        disabled={!writable || !deliveryEmailEnabled}
                        value={deliveryEmailRecipients}
                        onChange={(event) => setDeliveryEmailRecipients(event.target.value)}
                        placeholder="输入一个或多个邮箱，支持换行、逗号、分号分隔"
                      />
                    </div>

                    <div className="space-y-3 rounded-xl border bg-muted/10 p-4">
                      <div className="flex items-center justify-between gap-3">
                        <div className="space-y-0.5">
                          <p className="inline-flex items-center gap-1 text-sm font-medium">
                            <MessageCircleIcon className="size-3.5" />
                            WhatsApp
                          </p>
                          <p className="mp-kpi-note">用于即时发送保存链接或访客到场提醒。</p>
                        </div>
                        <Switch
                          checked={deliveryWhatsAppEnabled}
                          disabled={!writable}
                          onCheckedChange={(checked) => setDeliveryWhatsAppEnabled(checked)}
                        />
                      </div>
                      <Textarea
                        rows={4}
                        disabled={!writable || !deliveryWhatsAppEnabled}
                        value={deliveryWhatsAppRecipients}
                        onChange={(event) => setDeliveryWhatsAppRecipients(event.target.value)}
                        placeholder="输入一个或多个手机号，支持换行、逗号、分号分隔"
                      />
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      type="submit"
                      disabled={!writable || dispatchingDelivery || loading || refreshing || !deliveryPassID}
                    >
                      {dispatchingDelivery ? "发送中..." : "发送外部投递"}
                    </Button>
                    {!writable ? (
                      <span className="mp-kpi-note">只读角色只能查看发送回执。{readOnlyBoundaryHint}</span>
                    ) : null}
                  </div>
                </form>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">最近外部投递回执</CardTitle>
                <CardDescription>
                  展示 Email / WhatsApp 的最新发送结果、失败原因和每个通道的明细，失败且可重试时可直接重发。
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="rounded-xl border bg-muted/10 px-4 py-3 space-y-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">回执失败处理闭环状态</p>
                    <Badge variant={receiptRecoveryStatusVariant(receiptRecoveryFlowStatus)}>
                      {receiptRecoveryStatusLabel(receiptRecoveryFlowStatus)}
                    </Badge>
                  </div>
                  <div className="grid gap-2 md:grid-cols-3">
                    <div className="rounded-lg border bg-background px-3 py-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="text-sm font-medium">1. 回执失败分流</p>
                        <Badge variant={receiptRecoveryStatusVariant(receiptSplitStatus)}>
                          {receiptRecoveryStatusLabel(receiptSplitStatus)}
                        </Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        失败回执 {failedDeliveryNotifications.length} 条，其中可重发 {retryableDeliveryNotifications.length} 条，不可重发{" "}
                        {nonRetryableFailedDeliveryNotifications.length} 条。
                      </p>
                    </div>
                    <div className="rounded-lg border bg-background px-3 py-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="text-sm font-medium">2. 重发与状态修复</p>
                        <Badge variant={receiptRecoveryStatusVariant(receiptRemediationStatus)}>
                          {receiptRecoveryStatusLabel(receiptRemediationStatus)}
                        </Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        可批量重发 {batchRetryableDeliveryNotifications.length} 条，可修复状态 {repairableRetryableDeliveryPasses.length} 张，可写入补发草稿{" "}
                        {reissueTargetIDsByRetryableDelivery.length} 个。
                      </p>
                    </div>
                    <div className="rounded-lg border bg-background px-3 py-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="text-sm font-medium">3. 回企业页复核</p>
                        <Badge variant={receiptRecoveryStatusVariant(receiptReviewStatus)}>
                          {receiptRecoveryStatusLabel(receiptReviewStatus)}
                        </Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        重发或修复后，回企业页审批与异常复核是否仍存在同步异常和目录阻塞。
                      </p>
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={
                        !writable ||
                        batchRetryingDelivery ||
                        repairingRetryablePasses ||
                        loading ||
                        refreshing ||
                        batchRetryableDeliveryNotifications.length === 0
                      }
                      onClick={() => void retryDeliveryNotificationBatch()}
                    >
                      {batchRetryingDelivery ? "批量重发中..." : `批量重发失败通道（${batchRetryableDeliveryNotifications.length}）`}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={
                        !writable ||
                        batchRetryingDelivery ||
                        repairingRetryablePasses ||
                        loading ||
                        refreshing ||
                        repairableRetryableDeliveryPasses.length === 0
                      }
                      onClick={() => void repairRetryableDeliveryPassStatusBatch()}
                    >
                      {repairingRetryablePasses ? "状态修复中..." : `批量状态修复（${repairableRetryableDeliveryPasses.length}）`}
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={enterpriseReceiptRecoveryReviewLink}>回企业页复核回执失败</Link>
                    </Button>
                  </div>
                </div>
                <div className="rounded-xl border bg-muted/10 px-4 py-3">
                  <p className="mp-kpi-note">
                    {deliveryRetryQuery.trim()
                      ? `按对象线索“${deliveryRetryQuery.trim()}”可匹配 ${retryableDeliveryNotifications.length} 条可重发失败通道。`
                      : `当前可重发失败通道 ${retryableDeliveryNotifications.length} 条。`}
                    {retryableDeliveryNotifications.length > batchRetryableDeliveryNotifications.length
                      ? ` 单次最多处理 ${batchRetryableDeliveryNotifications.length} 条。`
                      : ""}
                    {` 可修复状态凭证 ${repairableRetryableDeliveryPasses.length} 张，可写入批量补发对象 ${reissueTargetIDsByRetryableDelivery.length} 个。`}
                  </p>
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={
                        !writable ||
                        batchRetryingDelivery ||
                        repairingRetryablePasses ||
                        loading ||
                        refreshing ||
                        batchRetryableDeliveryNotifications.length === 0
                      }
                      onClick={() => void retryDeliveryNotificationBatch()}
                    >
                      {batchRetryingDelivery
                        ? "批量重发中..."
                        : `批量重发失败通道（${batchRetryableDeliveryNotifications.length}）`}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={
                        !writable ||
                        batchRetryingDelivery ||
                        repairingRetryablePasses ||
                        loading ||
                        refreshing ||
                        repairableRetryableDeliveryPasses.length === 0
                      }
                      onClick={() => void repairRetryableDeliveryPassStatusBatch()}
                    >
                      {repairingRetryablePasses
                        ? "状态修复中..."
                        : `批量状态修复（${repairableRetryableDeliveryPasses.length}）`}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={
                        !writable ||
                        batchRetryingDelivery ||
                        repairingRetryablePasses ||
                        loading ||
                        refreshing ||
                        reissueTargetIDsByRetryableDelivery.length === 0
                      }
                      onClick={seedBatchReissueFromRetryableDelivery}
                    >
                      {`写入批量补发草稿（${reissueTargetIDsByRetryableDelivery.length}）`}
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={enterpriseAlertsIssueLink}>回企业页并按同步异常定位</Link>
                    </Button>
                    {hasWorkerAlertFlowHints ? (
                      <Button asChild size="sm" variant="outline">
                        <Link to={enterpriseSyncWorkerReviewLink}>处理完成后回导入与同步复核</Link>
                      </Button>
                    ) : null}
                  </div>
                </div>

                {recentDeliveryNotifications.length === 0 ? (
                  <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
                    还没有外部投递回执。选择一张凭证并发送后，这里会展示每次发送结果。
                  </div>
                ) : (
                  recentDeliveryNotifications.map((item) => {
                    const itemPass = passes.find((pass) => pass.id === item.pass_id)
                    const itemTemplate = templateByID.get(item.template_id)
                    return (
                      <div
                        key={item.id}
                        className="rounded-xl border bg-card/80 px-4 py-3"
                      >
                        <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                          <div className="space-y-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <p className="font-medium">{item.target_id}</p>
                              <Badge variant={deliveryNotificationStatusVariant(item.status)}>
                                {deliveryNotificationStatusLabel(item.status)}
                              </Badge>
                              <Badge variant="outline">attempt {item.attempt ?? 1}</Badge>
                              {item.reason ? <Badge variant="outline">{item.reason}</Badge> : null}
                            </div>
                            <p className="text-sm text-muted-foreground">{itemTemplate?.name ?? item.template_id}</p>
                            <p className="mp-kpi-note">
                              {formatDateTime(item.triggered_at)}
                              {item.source_notification_id ? ` · 重发自 ${item.source_notification_id}` : ""}
                            </p>
                            {item.channel_results && item.channel_results.length > 0 ? (
                              <div className="flex flex-col gap-1 pt-1 text-xs text-muted-foreground">
                                {item.channel_results.map((result) => (
                                  <p key={`${item.id}-${result.channel}`}>
                                    {result.channel} · {result.status}
                                    {result.reason ? ` (${result.reason})` : ""}
                                    {result.receivers && result.receivers.length > 0 ? ` · ${result.receivers.join(", ")}` : ""}
                                  </p>
                                ))}
                              </div>
                            ) : null}
                          </div>
                          <div className="flex flex-wrap items-center gap-2">
                            {itemPass?.save_link ? (
                              <>
                                <Button size="sm" variant="outline" onClick={() => void openPassQrDialog(itemPass)}>
                                  查看二维码
                                </Button>
                                <Button size="sm" variant="outline" onClick={() => void copySaveLink(itemPass)}>
                                  复制链接
                                </Button>
                              </>
                            ) : null}
                            {item.retryable ? (
                              <Button
                                size="sm"
                                variant="outline"
                                disabled={!writable || retryingDeliveryNotificationID === item.id}
                                onClick={() => void retryDeliveryNotification(item.id)}
                              >
                                {retryingDeliveryNotificationID === item.id ? "重发中..." : "重发失败通道"}
                              </Button>
                            ) : null}
                            {!writable ? <span className="mp-kpi-note">只读（权限边界）</span> : null}
                          </div>
                        </div>
                      </div>
                    )
                  })
                )}
              </CardContent>
            </Card>
          </div>

          <div className="grid gap-4 xl:grid-cols-[0.98fr_1.02fr]">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">实体卡任务</CardTitle>
                <CardDescription>
                  把制卡、补卡、挂失放进同一条实体卡工作流。挂失确认后会暂停数字凭证，补卡发放完成后可恢复员工凭证状态。
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <form className="space-y-4" onSubmit={submitPhysicalCardTask}>
                  <div className="grid gap-3 md:grid-cols-[minmax(0,1.2fr)_180px]">
                    <Select value={physicalTaskPassID} onValueChange={setPhysicalTaskPassID}>
                      <SelectTrigger>
                        <SelectValue placeholder="选择员工凭证" />
                      </SelectTrigger>
                      <SelectContent>
                        {employeeCardEligiblePasses.map((item) => {
                          const itemTemplate = templateByID.get(item.template_id)
                          return (
                            <SelectItem key={item.id} value={item.id}>
                              {item.target_id} · {itemTemplate?.name ?? item.template_id}
                            </SelectItem>
                          )
                        })}
                      </SelectContent>
                    </Select>
                    <Select
                      value={physicalTaskType}
                      onValueChange={(value) => setPhysicalTaskType(value as "issue" | "reissue" | "loss_report")}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="任务类型" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="issue">制卡</SelectItem>
                        <SelectItem value="reissue">补卡</SelectItem>
                        <SelectItem value="loss_report">挂失</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="grid gap-3 md:grid-cols-[220px_minmax(0,1fr)]">
                    <Input
                      value={physicalTaskCardNumber}
                      onChange={(event) => setPhysicalTaskCardNumber(event.target.value)}
                      placeholder="卡号，可选"
                    />
                    <Textarea
                      rows={3}
                      value={physicalTaskNote}
                      onChange={(event) => setPhysicalTaskNote(event.target.value)}
                      placeholder="补充制卡说明、交付备注或挂失原因"
                    />
                  </div>

                  {selectedPhysicalTaskPass ? (
                    <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium">{selectedPhysicalTaskPass.target_id}</span>
                        <Badge variant={passStatusVariant(selectedPhysicalTaskPass.status)}>
                          {passStatusLabel(selectedPhysicalTaskPass.status)}
                        </Badge>
                        <Badge variant="secondary">
                          {walletScenarioLabel(inferPassScenario(selectedPhysicalTaskPass, selectedPhysicalTaskTemplate))}
                        </Badge>
                      </div>
                      <p className="mt-1 text-sm text-muted-foreground">
                        {selectedPhysicalTaskTemplate?.name ?? selectedPhysicalTaskPass.template_id}
                      </p>
                      <p className="mt-1 text-xs text-muted-foreground">
                        {physicalTaskType === "loss_report"
                          ? "确认挂失后会自动把该员工的数字凭证暂停，避免实体卡遗失后仍可通行。"
                          : "适合前台制卡、补卡和卡号绑定，任务完成后可回到同类台账继续交付。"}
                      </p>
                    </div>
                  ) : (
                    <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
                      {employeeCardEligiblePasses.length === 0
                        ? "当前没有可联动的员工凭证。先去员工场景发放 MistyPass，再回来创建实体卡任务。"
                        : "请选择一个员工凭证后创建实体卡任务。"}
                    </div>
                  )}

                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      type="submit"
                      disabled={!writable || creatingPhysicalCardTask || loading || refreshing || !physicalTaskPassID}
                    >
                      {creatingPhysicalCardTask ? "创建中..." : "创建实体卡任务"}
                    </Button>
                    {!writable ? (
                      <span className="mp-kpi-note">只读角色只能查看任务进度。{readOnlyBoundaryHint}</span>
                    ) : null}
                    {employeeCardEligiblePasses.length === 0 ? (
                      <Button asChild size="sm" variant="outline">
                        <Link to="/wallet?scenario=employee_mobile">先发员工凭证</Link>
                      </Button>
                    ) : null}
                  </div>
                </form>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">最近实体卡进度</CardTitle>
                <CardDescription>
                  按最近更新时间显示制卡、补卡和挂失任务，支持在当前页面直接推进状态并查看联动后的数字凭证状态。
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {recentPhysicalCardTasks.length === 0 ? (
                  <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
                    还没有实体卡任务。需要制卡、补卡或挂失时，可在左侧直接创建。
                  </div>
                ) : (
                  recentPhysicalCardTasks.map((task) => {
                    const itemPass = passes.find((item) => item.id === task.pass_id)
                    const itemTemplate = templateByID.get(task.template_id)
                    const taskActions = nextPhysicalCardTaskActions(task)
                    return (
                      <div
                        key={task.id}
                        className="rounded-xl border bg-card/80 px-4 py-3"
                      >
                        <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                          <div className="space-y-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <p className="font-medium">{task.target_id}</p>
                              <Badge variant="secondary">{physicalCardTaskTypeLabel(task.task_type)}</Badge>
                              <Badge variant={physicalCardTaskStatusVariant(task.status)}>
                                {physicalCardTaskStatusLabel(task.status)}
                              </Badge>
                              <Badge variant={passStatusVariant(task.pass_status)}>
                                数字凭证 {passStatusLabel(task.pass_status)}
                              </Badge>
                            </div>
                            <p className="text-sm text-muted-foreground">{itemTemplate?.name ?? task.template_id}</p>
                            <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                              <span>任务 ID {task.id}</span>
                              <span>更新时间 {formatDateTime(task.updated_at)}</span>
                              {task.card_number ? <span>卡号 {task.card_number}</span> : null}
                            </div>
                            {task.note ? <p className="mp-kpi-note">{task.note}</p> : null}
                          </div>
                          <div className="flex flex-wrap items-center gap-2">
                            <Button size="sm" variant="outline" onClick={() => focusPassScenario("employee_physical")}>
                              查看同类台账
                            </Button>
                            {itemPass?.save_link ? (
                              <Button size="sm" variant="outline" onClick={() => void openPassQrDialog(itemPass)}>
                                查看二维码
                              </Button>
                            ) : null}
                            {taskActions.map((action) => (
                              <Button
                                key={action.status}
                                size="sm"
                                variant="outline"
                                disabled={!writable || updatingPhysicalCardTaskID === task.id}
                                onClick={() => void advancePhysicalCardTask(task, action.status)}
                              >
                                {updatingPhysicalCardTaskID === task.id ? "处理中..." : action.label}
                              </Button>
                            ))}
                            {!writable ? <span className="mp-kpi-note">只读（权限边界）</span> : null}
                          </div>
                        </div>
                      </div>
                    )
                  })
                )}
              </CardContent>
            </Card>
          </div>

	          <Card>
	            <CardHeader>
	              <CardTitle className="text-base">已发放凭证</CardTitle>
	              <CardDescription>
	                统一查看员工移动凭证、实体卡联动、访客二维码和临时证状态；支持按对象、模板和状态筛选，并在同一页完成批量停用、恢复和吊销。
	              </CardDescription>
	            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 xl:grid-cols-[minmax(0,1.4fr)_180px_180px_220px]">
                <Input
                  value={passQuery}
                  onChange={(event) => setPassQuery(event.target.value)}
                  placeholder="搜索员工/访客 ID、模板名、对象 ID 或状态"
                />
                <Select
                  value={passStatusFilter}
                  onValueChange={(value) =>
                    setPassStatusFilter(value as "all" | "issued" | "active" | "suspended" | "revoked")
                  }
                >
                  <SelectTrigger>
                    <SelectValue placeholder="状态" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">全部状态</SelectItem>
                    <SelectItem value="issued">已发放</SelectItem>
                    <SelectItem value="active">生效中</SelectItem>
                    <SelectItem value="suspended">已暂停</SelectItem>
                    <SelectItem value="revoked">已吊销</SelectItem>
                  </SelectContent>
                </Select>
                <Select
                  value={passTargetTypeFilter}
                  onValueChange={(value) => setPassTargetTypeFilter(value as "all" | "user" | "visitor")}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="对象类型" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">全部对象</SelectItem>
                    <SelectItem value="user">员工</SelectItem>
                    <SelectItem value="visitor">访客 / 临时证</SelectItem>
                  </SelectContent>
                </Select>
                <Select value={passTemplateFilter} onValueChange={setPassTemplateFilter}>
                  <SelectTrigger>
                    <SelectValue placeholder="模板" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">全部模板</SelectItem>
                    {templates.map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border bg-muted/10 px-4 py-3">
                <div className="space-y-1">
                  <p className="text-sm font-medium">当前命中 {filteredPasses.length} 张凭证</p>
                  <p className="mp-kpi-note">
                    {selectedVisiblePassCount > 0
                      ? `已勾选 ${selectedVisiblePassCount} 张，可直接批量暂停、恢复或吊销。`
                      : "先筛选需要处理的员工、访客或模板，再在下方统一做失效和状态维护。"}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {hasPassFilters ? (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        setPassQuery("")
                        setPassStatusFilter("all")
                        setPassTargetTypeFilter("all")
                        setPassTemplateFilter("all")
                      }}
                    >
                      清空筛选
                    </Button>
                  ) : null}
                  {selectedVisiblePassCount > 0 ? (
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => setSelectedPassIDs([])}
                      disabled={!writable || batchUpdatingPassAction.length > 0}
                    >
                      取消勾选
                    </Button>
                  ) : null}
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void updateSelectedPasses("suspend")}
                    disabled={!writable || selectedVisiblePassCount === 0 || batchUpdatingPassAction.length > 0}
                  >
                    {batchUpdatingPassAction === "suspend" ? "批量暂停中..." : "批量暂停"}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void updateSelectedPasses("activate")}
                    disabled={!writable || selectedVisiblePassCount === 0 || batchUpdatingPassAction.length > 0}
                  >
                    {batchUpdatingPassAction === "activate" ? "批量恢复中..." : "批量恢复"}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => void updateSelectedPasses("revoke")}
                    disabled={!writable || selectedVisiblePassCount === 0 || batchUpdatingPassAction.length > 0}
                  >
                    {batchUpdatingPassAction === "revoke" ? "批量吊销中..." : "批量吊销"}
                  </Button>
                </div>
              </div>

              <Table>
                <TableHeader>
                  <TableRow>
                    {writable ? (
                      <TableHead className="w-12">
                        <input
                          aria-label="select all visible wallet passes"
                          type="checkbox"
                          className="size-4 rounded border"
                          disabled={filteredPasses.length === 0 || batchUpdatingPassAction.length > 0}
                          checked={allVisiblePassesSelected}
                          onChange={(event) => onSelectAllVisiblePasses(event.target.checked)}
                        />
                      </TableHead>
                    ) : null}
                    <TableHead>模板</TableHead>
                    <TableHead>对象</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>有效期</TableHead>
                    <TableHead>保存链接</TableHead>
                    <TableHead>更新时间</TableHead>
                    <TableHead>操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    <TableRow>
                      <TableCell colSpan={writable ? 8 : 7} className="py-10 text-center text-muted-foreground">
                        正在加载已发放凭证...
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {!loading && filteredPasses.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={writable ? 8 : 7} className="py-8 text-center text-muted-foreground">
                        {passes.length === 0 ? "暂无凭证记录，请先完成模板配置或发放操作。" : "当前筛选条件下没有匹配的凭证。"}
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {!loading &&
                    filteredPasses.map((item) => {
                      const itemTemplate = templateByID.get(item.template_id)
                      const canSuspend = canApplyPassAction(item, "suspend")
                      const canActivate = canApplyPassAction(item, "activate")
                      const canRevoke = canApplyPassAction(item, "revoke")

                      return (
                        <TableRow key={item.id}>
                          {writable ? (
                            <TableCell>
                              <input
                                aria-label={`select wallet pass ${item.id}`}
                                type="checkbox"
                                className="size-4 rounded border"
                                checked={selectedPassIDSet.has(item.id)}
                                disabled={batchUpdatingPassAction.length > 0}
                                onChange={(event) => onSelectPass(item.id, event.target.checked)}
                              />
                            </TableCell>
                          ) : null}
	                          <TableCell>
	                            <div className="space-y-1">
	                              <p className="font-medium">{itemTemplate?.name ?? item.template_id}</p>
	                              <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
	                                <span>{itemTemplate ? passTypeLabel(itemTemplate.pass_type) : targetTypeLabel(item.target_type)}</span>
	                                <Badge variant="secondary">
	                                  {walletScenarioLabel(inferPassScenario(item, itemTemplate))}
	                                </Badge>
	                                {itemTemplate?.status === "inactive" ? <Badge variant="outline">模板已停用</Badge> : null}
	                              </div>
	                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="space-y-1">
                              <p>{item.target_id}</p>
                              <p className="mp-kpi-note">
                                {targetTypeLabel(item.target_type)} · object {item.object_id}
                              </p>
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant={passStatusVariant(item.status)}>{passStatusLabel(item.status)}</Badge>
                          </TableCell>
                          <TableCell className="mp-kpi-note">
                            {item.expires_at ? formatDateTime(item.expires_at) : "按默认策略"}
                          </TableCell>
	                          <TableCell className="text-xs">
	                            {item.save_link ? (
                                <div className="flex flex-col items-start gap-1">
                                  <a
                                    className="text-primary underline-offset-2 hover:underline"
                                    href={item.save_link}
                                    rel="noreferrer"
                                    target="_blank"
                                  >
                                    打开保存链接
                                  </a>
                                  <button
                                    className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                                    type="button"
                                    onClick={() => void copySaveLink(item)}
                                  >
                                    复制链接
                                  </button>
                                  <button
                                    className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                                    type="button"
                                    onClick={() => void openPassQrDialog(item)}
                                  >
                                    查看二维码
                                  </button>
                                </div>
		                            ) : (
		                              <button
                                  className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                                  type="button"
                                  onClick={() => void refreshPassSaveLink(item)}
                                  disabled={resolvingSaveLinkPassID === item.id}
                                >
                                  {resolvingSaveLinkPassID === item.id ? "刷新中..." : "刷新链接"}
                                </button>
		                            )}
	                          </TableCell>
                          <TableCell className="mp-kpi-note">{formatDateTime(item.updated_at)}</TableCell>
                          <TableCell>
                            <div className="flex flex-wrap gap-2">
                              {canSuspend ? (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  disabled={!writable || updatingPassID === item.id || batchUpdatingPassAction.length > 0}
                                  onClick={() => void updatePassStatus(item, "suspend")}
                                >
                                  暂停
                                </Button>
                              ) : null}
                              {canActivate ? (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  disabled={!writable || updatingPassID === item.id || batchUpdatingPassAction.length > 0}
                                  onClick={() => void updatePassStatus(item, "activate")}
                                >
                                  激活
                                </Button>
                              ) : null}
                              {canRevoke ? (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  disabled={!writable || updatingPassID === item.id || batchUpdatingPassAction.length > 0}
                                  onClick={() => void updatePassStatus(item, "revoke")}
                                >
                                  吊销
                                </Button>
                              ) : null}
                              {!writable ? <span className="mp-kpi-note">只读（权限边界）</span> : null}
                            </div>
                          </TableCell>
                        </TableRow>
                      )
                    })}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="advanced" className="space-y-4">
          <div className="rounded-xl border bg-muted/15 px-4 py-3">
            <p className="text-sm font-medium">高级运行视图</p>
            <p className="mt-1 text-sm text-muted-foreground">
              这里保留告警订阅、趋势、错误原因、通知记录和清理归档，供平台管理员、企业管理员和排障场景使用；普通发放主路径已收口在“发放操作”页签内。
            </p>
          </div>

          <Card>
        <CardHeader>
          <CardTitle className="text-base">发放告警订阅</CardTitle>
          <CardDescription>
            当前组织的发放告警订阅配置；平台管理员可切换租户，企业管理员可直接维护本组织策略。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-3 md:grid-cols-3">
            <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
              <div className="space-y-0.5">
                <p className="text-sm font-medium">启用订阅</p>
                <p className="mp-kpi-note">关闭后仅保留指标看板。</p>
              </div>
              <Switch
                checked={subscriptionEnabled}
                disabled={!writable}
                onCheckedChange={(checked) => {
                  setSubscriptionEnabled(checked)
                }}
              />
            </div>
            <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
              <div className="space-y-0.5">
                <p className="inline-flex items-center gap-1 text-sm font-medium">
                  <MailIcon className="size-3.5" />
                  Email
                </p>
                <p className="mp-kpi-note">邮件通知通道。</p>
              </div>
              <Switch
                checked={subscriptionEmailEnabled}
                disabled={!writable}
                onCheckedChange={(checked) => {
                  setSubscriptionEmailEnabled(checked)
                }}
              />
            </div>
            <div className="flex items-center justify-between rounded-lg border bg-muted/20 px-3 py-2">
              <div className="space-y-0.5">
                <p className="inline-flex items-center gap-1 text-sm font-medium">
                  <MessageCircleIcon className="size-3.5" />
                  WhatsApp
                </p>
                <p className="mp-kpi-note">即时通知通道。</p>
              </div>
              <Switch
                checked={subscriptionWhatsAppEnabled}
                disabled={!writable}
                onCheckedChange={(checked) => {
                  setSubscriptionWhatsAppEnabled(checked)
                }}
              />
            </div>
          </div>

          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <Input
              value={subscriptionThreshold}
              disabled={!writable}
              onChange={(event) => setSubscriptionThreshold(event.target.value)}
              placeholder="dlq_alert_threshold"
            />
            <Input
              value={subscriptionWindowSeconds}
              disabled={!writable}
              onChange={(event) => setSubscriptionWindowSeconds(event.target.value)}
              placeholder="window_seconds"
            />
            <Input
              value={subscriptionCooldownSeconds}
              disabled={!writable}
              onChange={(event) => setSubscriptionCooldownSeconds(event.target.value)}
              placeholder="cooldown_seconds"
            />
            <Input
              value={subscriptionReceiverGroups}
              disabled={!writable}
              onChange={(event) => setSubscriptionReceiverGroups(event.target.value)}
              placeholder="receiver_groups（逗号分隔）"
            />
          </div>

          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="space-y-1">
              <p className="mp-kpi-note">
                最近更新：{formatDateTime(subscription?.updated_at)}
              </p>
              {!writable ? (
                <p className="mp-kpi-note">
                  当前角色为只读，可查看发放状态但不能修改订阅策略。{readOnlyBoundaryHint}
                </p>
              ) : null}
              {dispatchSummary ? (
                <p className="text-xs text-emerald-700">{dispatchSummary}</p>
              ) : null}
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                onClick={() => void dispatchAlertsNow()}
                disabled={loading || refreshing || dispatchingAlerts || !writable}
              >
                {dispatchingAlerts ? "发送中..." : "立即评估并通知"}
              </Button>
              <Button
                onClick={() => void saveAlertSubscription()}
                disabled={loading || refreshing || savingSubscription || !writable}
              >
                {savingSubscription ? "保存中..." : "保存订阅策略"}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {effectiveError ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {effectiveError}
        </div>
      ) : null}
      {platformViewer && aggregateWarning ? (
        <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-700">
          {aggregateWarning}
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>待处理异常</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (metrics?.summary.dlq ?? 0)}
              <ShieldAlertIcon className="size-4 text-amber-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            命中阈值：{loading ? "--" : alertItems.length}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>可自动重试异常</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : (metrics?.summary.retryable_failed ?? 0)}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">`status=failed` 且可自动重试。</CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>窗口新增发放</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : (metrics?.window.created ?? 0)}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            员工、访客与临时证统一统计，最近 {loading ? "--" : formatDurationSeconds(metrics?.window.window_seconds)}。
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>发放状态变更</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : (metrics?.window.updated ?? 0)}
              <WalletCardsIcon className="size-4 text-cyan-500" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">
            成功 {loading ? "--" : (metrics?.window.success ?? 0)} / 失败{" "}
            {loading ? "--" : (metrics?.window.failed ?? 0)} / DLQ {loading ? "--" : (metrics?.window.dlq ?? 0)}
          </CardContent>
        </Card>
      </div>

      {platformViewer ? (
        <>
          <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>跨租户任务总量</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.total}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">当前查询窗口对应的租户级汇总。</CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>跨租户失败</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.failed}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">全部租户 `status=failed` 聚合。</CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>跨租户 DLQ</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.dlq}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">全部租户 `status=dlq` 聚合。</CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardDescription>命中阈值租户</CardDescription>
                <CardTitle className="text-2xl">{loading ? "--" : aggregateStats.alertTenants}</CardTitle>
              </CardHeader>
              <CardContent className="mp-kpi-note">`alerts.length` 大于 0 的租户数。</CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">跨租户发放风险排行</CardTitle>
              <CardDescription>按异常积压和失败量倒序，优先定位高风险租户。</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>租户</TableHead>
                    <TableHead>任务总量</TableHead>
                    <TableHead>失败</TableHead>
                    <TableHead>DLQ</TableHead>
                    <TableHead>可重试失败</TableHead>
                    <TableHead>告警</TableHead>
                    <TableHead>更新时间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {loading ? (
                    <TableRow>
                      <TableCell colSpan={7} className="py-10 text-center text-muted-foreground">
                        正在加载跨租户聚合...
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {!loading && tenantAggregates.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={7} className="py-8 text-center text-muted-foreground">
                        暂无跨租户聚合数据。
                      </TableCell>
                    </TableRow>
                  ) : null}
                  {!loading &&
                    tenantAggregates.map((row) => (
                      <TableRow key={row.tenantID}>
                        <TableCell className="font-medium">
                          <TableCellText className="max-w-[14rem]">{row.tenantName}</TableCellText>
                        </TableCell>
                        <TableCell>{row.total}</TableCell>
                        <TableCell>{row.failed}</TableCell>
                        <TableCell>{row.dlq}</TableCell>
                        <TableCell>{row.retryableFailed}</TableCell>
                        <TableCell>
                          <Badge variant={row.alertCount > 0 ? "destructive" : "outline"}>{row.alertCount}</Badge>
                        </TableCell>
                        <TableCell className="mp-kpi-note">{formatDateTime(row.updatedAt)}</TableCell>
                      </TableRow>
                    ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle className="inline-flex items-center gap-2 text-base">
            <TrendingUpIcon className="size-4 text-cyan-500" />
            发放运行趋势
          </CardTitle>
          <CardDescription>
            窗口 {loading ? "--" : formatDurationSeconds(metricsTrend?.window_seconds)}，每桶{" "}
            {loading ? "--" : formatDurationSeconds(metricsTrend?.bucket_seconds)}，共{" "}
            {loading ? "--" : (metricsTrend?.bucket_count ?? 0)} 桶。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {loading ? (
            <div className="py-8 text-center text-sm text-muted-foreground">正在加载趋势数据...</div>
          ) : null}
          {!loading && (!metricsTrend || metricsTrend.buckets.length === 0) ? (
            <div className="py-8 text-center text-sm text-muted-foreground">窗口内暂无趋势数据。</div>
          ) : null}
          {!loading &&
            (metricsTrend?.buckets ?? []).map((item) => {
              const successWidth = item.success > 0 ? Math.max(6, Math.round((item.success / trendPeakUpdated) * 100)) : 0
              const failedWidth = item.failed > 0 ? Math.max(6, Math.round((item.failed / trendPeakUpdated) * 100)) : 0
              const dlqWidth = item.dlq > 0 ? Math.max(6, Math.round((item.dlq / trendPeakUpdated) * 100)) : 0
              return (
                <div key={`${item.index}-${item.start}`} className="rounded-lg border bg-muted/20 px-3 py-2">
                  <div className="mb-2 flex items-center justify-between text-xs text-muted-foreground">
                    <span>
                      {formatTimeLabel(item.start)} - {formatTimeLabel(item.end)}
                    </span>
                    <span>
                      updated {item.updated} / created {item.created}
                    </span>
                  </div>
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-2 text-xs">
                      <span className="w-10 text-muted-foreground">success</span>
                      <div className="h-2 flex-1 rounded bg-emerald-500/15">
                        <div className="h-2 rounded bg-emerald-500" style={{ width: `${successWidth}%` }} />
                      </div>
                      <span className="w-6 text-right">{item.success}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="w-10 text-muted-foreground">failed</span>
                      <div className="h-2 flex-1 rounded bg-amber-500/15">
                        <div className="h-2 rounded bg-amber-500" style={{ width: `${failedWidth}%` }} />
                      </div>
                      <span className="w-6 text-right">{item.failed}</span>
                    </div>
                    <div className="flex items-center gap-2 text-xs">
                      <span className="w-10 text-muted-foreground">dlq</span>
                      <div className="h-2 flex-1 rounded bg-red-500/15">
                        <div className="h-2 rounded bg-red-500" style={{ width: `${dlqWidth}%` }} />
                      </div>
                      <span className="w-6 text-right">{item.dlq}</span>
                    </div>
                  </div>
                </div>
              )
            })}
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[1fr_1fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">发放异常阈值</CardTitle>
            <CardDescription>
              当前阈值：{loading ? "--" : (metrics?.dlq_alert_threshold ?? 0)}（`dlq_alert_threshold`）
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>类型</TableHead>
                  <TableHead>错误码</TableHead>
                  <TableHead>计数</TableHead>
                  <TableHead>阈值</TableHead>
                  <TableHead>状态</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-10 text-center text-muted-foreground">
                      正在加载告警...
                    </TableCell>
                  </TableRow>
                ) : null}
                {!loading && alertItems.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                      暂无命中阈值的告警。
                    </TableCell>
                  </TableRow>
                ) : null}
                {!loading &&
                  alertItems.map((item, index) => (
                    <TableRow key={`${item.type}-${item.error_code ?? "unknown"}-${index}`}>
                      <TableCell>{item.type}</TableCell>
                      <TableCell>{item.error_code ?? "-"}</TableCell>
                      <TableCell>{item.count}</TableCell>
                      <TableCell>{item.threshold}</TableCell>
                      <TableCell>
                        <Badge variant={item.count >= item.threshold ? "destructive" : "outline"}>
                          {item.count >= item.threshold ? "超阈值" : "正常"}
                        </Badge>
                      </TableCell>
                    </TableRow>
                  ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">失败原因分布（Top 5）</CardTitle>
            <CardDescription>
              统计区间：{loading ? "--" : formatDateTime(metrics?.window.since)} -{" "}
              {loading ? "--" : formatDateTime(metrics?.window.until)}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {loading ? (
              <div className="py-8 text-center text-sm text-muted-foreground">正在加载错误码分布...</div>
            ) : null}
            {!loading && windowErrorCodeRows.length === 0 ? (
              <div className="py-8 text-center text-sm text-muted-foreground">窗口内暂无错误码。</div>
            ) : null}
            {!loading &&
              windowErrorCodeRows.map(([code, count]) => (
                <div
                  key={code}
                  className="flex items-center justify-between rounded-lg border bg-muted/25 px-3 py-2 text-sm"
                >
                  <span className="font-medium">{code}</span>
                  <Badge variant="outline">{count}</Badge>
                </div>
              ))}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">异常通知记录</CardTitle>
          <CardDescription>最近 {alertNotifications.length} 条发送结果（包含冷却跳过）。</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>时间</TableHead>
                <TableHead>错误码</TableHead>
                <TableHead>计数 / 阈值</TableHead>
                <TableHead>尝试</TableHead>
                <TableHead>通道</TableHead>
                <TableHead>接收组</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={8} className="py-10 text-center text-muted-foreground">
                    正在加载发送记录...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && alertNotifications.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} className="py-8 text-center text-muted-foreground">
                    暂无发送记录，点击“立即评估并发送”后生成。
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                alertNotifications.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="mp-kpi-note">{formatDateTime(item.triggered_at)}</TableCell>
                    <TableCell>{item.error_code || "-"}</TableCell>
                    <TableCell>
                      {item.count} / {item.threshold}
                    </TableCell>
                    <TableCell>{item.attempt ?? "-"}</TableCell>
                    <TableCell className="mp-kpi-note">
                      {item.channels && item.channels.length > 0 ? item.channels.join(", ") : "-"}
                    </TableCell>
                    <TableCell className="mp-kpi-note">
                      {item.receiver_groups && item.receiver_groups.length > 0
                        ? item.receiver_groups.join(", ")
                        : "-"}
                    </TableCell>
                    <TableCell>
                      <div className="space-y-1">
                        <Badge variant={item.status === "sent" ? "secondary" : "outline"}>
                          {item.status}
                          {item.reason ? ` (${item.reason})` : ""}
                        </Badge>
                        {item.channel_results && item.channel_results.length > 0 ? (
                          <p className="mp-kpi-note">
                            {item.channel_results
                              .map((result) =>
                                result.reason
                                  ? `${result.channel}:${result.status}(${result.reason})`
                                  : `${result.channel}:${result.status}`
                              )
                              .join(" | ")}
                          </p>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>
                      {item.status === "failed" && item.retryable ? (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => void retryAlertNotification(item.id)}
                          disabled={retryingAlertNotificationID === item.id || loading || refreshing || !writable}
                        >
                          {retryingAlertNotificationID === item.id ? "重试中..." : "重试"}
                        </Button>
                      ) : (
                        <span className="mp-kpi-note">-</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">高级清理归档</CardTitle>
          <CardDescription>最近 {archives.length} 条异常清理记录（按时间倒序）。</CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>时间</TableHead>
                <TableHead>执行人</TableHead>
                <TableHead>筛选条件</TableHead>
                <TableHead>清理结果</TableHead>
                <TableHead>处理任务</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-10 text-center text-muted-foreground">
                    正在加载归档记录...
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading && archives.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                    暂无清理归档记录。
                  </TableCell>
                </TableRow>
              ) : null}
              {!loading &&
                archives.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="mp-kpi-note">{formatDateTime(item.at)}</TableCell>
                    <TableCell>{item.actor || "-"}</TableCell>
                    <TableCell>
                      <div className="flex flex-col gap-1 text-xs">
                        <span>error_code: {item.error_code || "*"}</span>
                        <span>older_than: {formatDurationSeconds(item.older_than_seconds)}</span>
                        <span>limit: {item.limit}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <Badge variant={item.removed > 0 ? "secondary" : "outline"}>
                          <Trash2Icon className="mr-1 size-3" />
                          removed {item.removed}
                        </Badge>
                        <Badge variant="outline">remaining {item.remaining_dlq}</Badge>
                      </div>
                    </TableCell>
                    <TableCell className="mp-kpi-note">
                      {item.processed_jobs && item.processed_jobs.length > 0 ? (
                        item.processed_jobs.slice(0, 3).join(", ")
                      ) : (
                        <span className="inline-flex items-center gap-1">
                          <AlertTriangleIcon className="size-3" />
                          无
                        </span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
            </TableBody>
          </Table>
        </CardContent>
	      </Card>
	        </TabsContent>
	      </Tabs>

      <Dialog
        open={qrDialogOpen}
        onOpenChange={(open) => {
          setQrDialogOpen(open)
          if (!open) {
            setQrDialogPass(null)
            setQrDialogSaveLink("")
            setQrDialogSVG("")
            setQrDialogLoading(false)
          }
        }}
      >
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>凭证二维码</DialogTitle>
            <DialogDescription>
              {qrDialogPass
                ? `${qrDialogPass.target_id} · ${walletScenarioLabel(inferPassScenario(qrDialogPass, qrDialogTemplate))}`
                : "预览保存链接对应的二维码，可直接下载 SVG 或复制链接。"}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            {qrDialogPass ? (
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant={passStatusVariant(qrDialogPass.status)}>{passStatusLabel(qrDialogPass.status)}</Badge>
                <Badge variant="secondary">{walletScenarioLabel(inferPassScenario(qrDialogPass, qrDialogTemplate))}</Badge>
                <Badge variant="outline">{deliveryMethodLabel(getTemplateDeliveryMethod(qrDialogTemplate))}</Badge>
                <Badge variant="outline">{accessMediumLabel(getTemplateAccessMedium(qrDialogTemplate))}</Badge>
              </div>
            ) : null}

            <div className="flex min-h-84 items-center justify-center rounded-xl border bg-muted/10 p-4">
              {qrDialogLoading ? (
                <p className="text-sm text-muted-foreground">正在生成二维码...</p>
              ) : qrDialogSVG ? (
                <div
                  className="rounded-lg border bg-white p-3 shadow-sm"
                  dangerouslySetInnerHTML={{ __html: qrDialogSVG }}
                />
              ) : (
                <p className="text-sm text-muted-foreground">当前没有可预览的二维码。</p>
              )}
            </div>

            <div className="rounded-lg border bg-muted/10 px-3 py-2 text-xs text-muted-foreground break-all">
              {qrDialogSaveLink || "当前还没有保存链接，可先尝试刷新链接。"}
            </div>

            <div className="flex flex-wrap items-center gap-2">
              {qrDialogSaveLink ? (
                <>
                  <Button size="sm" variant="outline" onClick={() => downloadQrSVG()}>
                    下载 SVG
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() =>
                      qrDialogPass ? void copySaveLink({ ...qrDialogPass, save_link: qrDialogSaveLink }) : undefined
                    }
                  >
                    复制链接
                  </Button>
                  <Button asChild size="sm" variant="outline">
                    <a href={qrDialogSaveLink} rel="noreferrer" target="_blank">
                      打开链接
                    </a>
                  </Button>
                </>
              ) : null}
              {qrDialogPass ? (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => void refreshPassSaveLink(qrDialogPass)}
                  disabled={resolvingSaveLinkPassID === qrDialogPass.id}
                >
                  {resolvingSaveLinkPassID === qrDialogPass.id ? "刷新中..." : "刷新链接"}
                </Button>
              ) : null}
            </div>
          </div>
        </DialogContent>
      </Dialog>
	    </div>
	  )
	}
