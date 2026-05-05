import { zodResolver } from "@hookform/resolvers/zod"
import { type ColumnDef, type SortingState, type VisibilityState, getCoreRowModel, getFilteredRowModel, getPaginationRowModel, getSortedRowModel, useReactTable } from "@tanstack/react-table"
import { useEffect, useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"
import {
  ArrowUpDownIcon,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { TableCellText } from "@/components/ui/table"
import { useLocation } from "react-router"
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
import { canAccessAccessPage, canAccessEnterprisePage, canManageIssuance, getViewerTenantID, isPlatformViewer } from "@/lib/viewer"
import { WalletAdvancedWorkspace } from "@/components/wallet/wallet-advanced-workspace"
import { WalletAlertConfigCard } from "@/components/wallet/wallet-alert-config-card"
import { WalletDeliveryReceiptsCard } from "@/components/wallet/wallet-delivery-receipts-card"
import { WalletDeliveryWorkspacePanels } from "@/components/wallet/wallet-delivery-workspace-panels"
import { WalletIssueJobQueueCard } from "@/components/wallet/wallet-issue-job-queue-card"
import { WalletIssuedPassesCard } from "@/components/wallet/wallet-issued-passes-card"
import { WalletOperationsOverviewCards } from "@/components/wallet/wallet-operations-overview-cards"
import { WalletPassQrDialog } from "@/components/wallet/wallet-pass-qr-dialog"
import { WalletPageOverview } from "@/components/wallet/wallet-page-overview"
import { WalletSendDeliveryCard } from "@/components/wallet/wallet-send-delivery-card"
import { WalletTemplateManagerCard } from "@/components/wallet/wallet-template-manager-card"
import {
  accessMediumLabel,
  buildRelativeDateTimeInput,
  createWalletScenarioCounters,
  deliveryHint,
  deliveryMethodLabel,
  deliveryNotificationStatusLabel,
  deliveryNotificationStatusVariant,
  dispatchChannelLabels,
  enterpriseWalletSegmentLabel,
  enterpriseWalletSegmentStatusLabel,
  enterpriseWalletStageLabel,
  formatDateTime,
  formatDurationSeconds,
  formatTimeLabel,
  getTemplateAccessMedium,
  getTemplateDeliveryMethod,
  inferPassScenario,
  inferTemplateScenario,
  nextPhysicalCardTaskActions,
  normalizeDateTimeInput,
  parseNonNegativeInt,
  parsePositiveInt,
  parseReceiverGroups,
  parseReceiverValues,
  parseStyleConfig,
  parseTargetIDs,
  passStatusLabel,
  passStatusVariant,
  passTypeLabel,
  physicalCardTaskStatusLabel,
  physicalCardTaskStatusVariant,
  physicalCardTaskTypeLabel,
  receiptRecoveryActionHintLabel,
  receiptRecoveryStatusLabel,
  receiptRecoveryStatusVariant,
  resolveEnterpriseTargetQuery,
  resolveReceiptRecoveryActionHint,
  stringifyStyleConfig,
  targetTypeLabel,
  templateStatusVariant,
  walletIssuanceScenarioPresetByID,
  walletIssuanceScenarioPresets,
  walletScenarioHint,
  walletScenarioLabel,
  withRouteHints,
  type ReceiptRecoveryStatus,
  type WalletScenarioKind,
} from "./wallet-page-utils"
import QRCode from "qrcode"
import { z } from "zod"

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
const physicalCardTaskTypeValues = ["issue", "reissue", "loss_report"] as const

function createPassDeliverySchema(t: TFunction) {
  return z
    .object({
      delivery_pass_id: z.string().trim().min(1, t("walletPage.validation.delivery.passRequired")),
      delivery_email_enabled: z.boolean(),
      delivery_whatsapp_enabled: z.boolean(),
      delivery_email_recipients: z.string().max(50000, t("walletPage.validation.delivery.emailRecipientsTooLong")),
      delivery_whatsapp_recipients: z
        .string()
        .max(50000, t("walletPage.validation.delivery.whatsAppRecipientsTooLong")),
    })
    .superRefine((values, context) => {
      if (!values.delivery_email_enabled && !values.delivery_whatsapp_enabled) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["delivery_email_enabled"],
          message: t("walletPage.validation.delivery.channelRequired"),
        })
      }
    })
}

function createPhysicalCardTaskSchema(t: TFunction) {
  return z.object({
    physical_task_pass_id: z.string().trim().min(1, t("walletPage.validation.physicalTask.passRequired")),
    physical_task_type: z.enum(physicalCardTaskTypeValues),
    physical_task_card_number: z
      .string()
      .trim()
      .max(128, t("walletPage.validation.physicalTask.cardNumberTooLong"))
      .optional()
      .or(z.literal("")),
    physical_task_note: z
      .string()
      .trim()
      .max(500, t("walletPage.validation.physicalTask.noteTooLong"))
      .optional()
      .or(z.literal("")),
  })
}

type PassDeliveryFormValues = z.infer<ReturnType<typeof createPassDeliverySchema>>
type PhysicalCardTaskFormValues = z.infer<ReturnType<typeof createPhysicalCardTaskSchema>>

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

export function WalletPage({ token, viewer }: WalletPageProps) {
  const { t, i18n } = useTranslation()
  const location = useLocation()
  const platformViewer = isPlatformViewer(viewer)
  const writable = canManageIssuance(viewer)
  const enterpriseWorkspaceAccessible = canAccessEnterprisePage(viewer)
  const accessWorkspaceAccessible = canAccessAccessPage(viewer)
  const readOnlyBoundaryHint = t("walletPage.hints.readOnlyBoundary")
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
  const [passPage, setPassPage] = useState(1)
  const [passPageSize, setPassPageSize] = useState(25)
  const [passSorting, setPassSorting] = useState<SortingState>([])
  const [passColumnVisibility, setPassColumnVisibility] = useState<VisibilityState>({})

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
  const [qrDialogPreviewURL, setQrDialogPreviewURL] = useState("")
  const [qrDialogLoading, setQrDialogLoading] = useState(false)
  const tenantsQuery = useQuery({
    queryKey: ["wallet-tenants", platformViewer ? "platform" : "tenant"],
    queryFn: () => (platformViewer ? listTenants(token) : Promise.resolve([])),
    staleTime: 30 * 1000,
  })
  const queryError = tenantsQuery.error instanceof Error ? tenantsQuery.error.message : ""
  const passDeliverySchema = useMemo(() => createPassDeliverySchema(t), [t, i18n.language])
  const physicalCardTaskSchema = useMemo(() => createPhysicalCardTaskSchema(t), [t, i18n.language])
  const passDeliveryForm = useForm<PassDeliveryFormValues>({
    resolver: zodResolver(passDeliverySchema),
    values: {
      delivery_pass_id: deliveryPassID,
      delivery_email_enabled: deliveryEmailEnabled,
      delivery_whatsapp_enabled: deliveryWhatsAppEnabled,
      delivery_email_recipients: deliveryEmailRecipients,
      delivery_whatsapp_recipients: deliveryWhatsAppRecipients,
    },
  })
  const physicalCardTaskForm = useForm<PhysicalCardTaskFormValues>({
    resolver: zodResolver(physicalCardTaskSchema),
    values: {
      physical_task_pass_id: physicalTaskPassID,
      physical_task_type: physicalTaskType,
      physical_task_card_number: physicalTaskCardNumber,
      physical_task_note: physicalTaskNote,
    },
  })
  const deliveryEmailRecipientsField = passDeliveryForm.register("delivery_email_recipients")
  const deliveryWhatsAppRecipientsField = passDeliveryForm.register("delivery_whatsapp_recipients")
  const physicalTaskCardNumberField = physicalCardTaskForm.register("physical_task_card_number")
  const physicalTaskNoteField = physicalCardTaskForm.register("physical_task_note")
  const passDeliveryFormError =
    passDeliveryForm.formState.errors.delivery_pass_id?.message ||
    passDeliveryForm.formState.errors.delivery_email_enabled?.message ||
    passDeliveryForm.formState.errors.delivery_whatsapp_enabled?.message ||
    passDeliveryForm.formState.errors.delivery_email_recipients?.message ||
    passDeliveryForm.formState.errors.delivery_whatsapp_recipients?.message ||
    ""
  const physicalCardTaskFormError =
    physicalCardTaskForm.formState.errors.physical_task_pass_id?.message ||
    physicalCardTaskForm.formState.errors.physical_task_type?.message ||
    physicalCardTaskForm.formState.errors.physical_task_card_number?.message ||
    physicalCardTaskForm.formState.errors.physical_task_note?.message ||
    ""
  useEffect(() => {
    if (!qrDialogSVG) {
      setQrDialogPreviewURL("")
      return
    }

    const blob = new Blob([qrDialogSVG], { type: "image/svg+xml;charset=utf-8" })
    const objectURL = URL.createObjectURL(blob)
    setQrDialogPreviewURL(objectURL)

    return () => {
      URL.revokeObjectURL(objectURL)
    }
  }, [qrDialogSVG])
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
            reason: t("walletPage.enterprise.missingTargetReason.issueReady"),
            employeeName: employee.full_name || "",
            approvalHint: employee.email || employee.external_id || targetID,
          }
        }
        return {
          targetID,
          category: "needs_alerts" as const,
          sourceLabel: employee.source || "-",
          groupLabel: groupLabel || "-",
          reason: t("walletPage.enterprise.missingTargetReason.statusNeedsAlerts", { status: employee.status }),
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
          reason: t("walletPage.enterprise.missingTargetReason.needsDirectory"),
          employeeName: "",
          approvalHint: targetID,
        }
      }
      return {
        targetID,
        category: "needs_alerts" as const,
        sourceLabel: "-",
        groupLabel: "-",
        reason: t("walletPage.enterprise.missingTargetReason.needsAlerts"),
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
    return t("walletPage.enterprise.latestSyncIssueHint", {
      source: enterpriseLatestSyncJob.source,
      status: enterpriseLatestSyncJob.status,
      rejected: enterpriseLatestSyncJob.rejected,
    })
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
    const segmentLabel = enterpriseWalletSegmentLabel(t, enterpriseFlowContext.segmentHint)
    if (!segmentLabel) {
      return ""
    }
    const statusLabel = enterpriseWalletSegmentStatusLabel(t, enterpriseFlowContext.segmentStatusHint)
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
  const passByID = useMemo(() => new Map(passes.map((item) => [item.id, item])), [passes])
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
  const passMaxPage = Math.max(1, Math.ceil(filteredPasses.length / passPageSize))
  const selectedPassIDSet = useMemo(() => new Set(selectedPassIDs), [selectedPassIDs])
  const passColumns = useMemo<ColumnDef<WalletPassInstance>[]>(
    () => {
      const definition: ColumnDef<WalletPassInstance>[] = [
        {
          id: "template",
          accessorFn: (row) => templateByID.get(row.template_id)?.name ?? row.template_id,
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.template")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => {
            const itemTemplate = templateByID.get(row.original.template_id)
            return (
              <div className="space-y-1">
                <p className="font-medium">{itemTemplate?.name ?? row.original.template_id}</p>
                <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                  <span>{itemTemplate ? passTypeLabel(t, itemTemplate.pass_type) : targetTypeLabel(t, row.original.target_type)}</span>
                  <Badge variant="secondary">
                    {walletScenarioLabel(t, inferPassScenario(row.original, itemTemplate))}
                  </Badge>
                  {itemTemplate?.status === "inactive" ? (
                    <Badge variant="outline">{t("walletPage.table.templateInactive")}</Badge>
                  ) : null}
                </div>
              </div>
            )
          },
        },
        {
          id: "target",
          accessorFn: (row) => row.target_id,
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.target")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <div className="space-y-1">
              <p>{row.original.target_id}</p>
              <p className="mp-kpi-note">
                {targetTypeLabel(t, row.original.target_type)} · object {row.original.object_id}
              </p>
            </div>
          ),
        },
        {
          id: "status",
          accessorKey: "status",
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.status")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <Badge variant={passStatusVariant(row.original.status)}>{passStatusLabel(t, row.original.status)}</Badge>,
        },
        {
          id: "expires_at",
          accessorFn: (row) => row.expires_at || "",
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.expiresAt")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <span className="mp-kpi-note">
              {row.original.expires_at ? formatDateTime(row.original.expires_at) : t("walletPage.table.expiresAtDefaultPolicy")}
            </span>
          ),
        },
        {
          id: "save_link",
          accessorFn: (row) => row.save_link || "",
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.saveLink")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => (
            <div className="text-xs">
              {row.original.save_link ? (
                <div className="flex flex-col items-start gap-1">
                  <a
                    className="text-primary underline-offset-2 hover:underline"
                    href={row.original.save_link}
                    rel="noreferrer"
                    target="_blank"
                  >
                    {t("walletPage.actions.openSaveLink")}
                  </a>
                  <button
                    className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                    type="button"
                    onClick={() => void copySaveLink(row.original)}
                  >
                    {t("walletPage.actions.copyLink")}
                  </button>
                  <button
                    className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                    type="button"
                    onClick={() => void openPassQrDialog(row.original)}
                  >
                    {t("walletPage.actions.viewQrCode")}
                  </button>
                </div>
              ) : (
                <button
                  className="text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
                  type="button"
                  onClick={() => void refreshPassSaveLink(row.original)}
                  disabled={resolvingSaveLinkPassID === row.original.id}
                >
                  {resolvingSaveLinkPassID === row.original.id
                    ? t("walletPage.actions.refreshing")
                    : t("walletPage.actions.refreshLink")}
                </button>
              )}
            </div>
          ),
        },
        {
          id: "updated_at",
          accessorKey: "updated_at",
          header: ({ column }) => (
              <Button variant="ghost" className="-ml-2 h-8 px-2" onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}>
              {t("walletPage.table.columns.updatedAt")}
              <ArrowUpDownIcon className="ml-1.5 size-3.5" />
            </Button>
          ),
          cell: ({ row }) => <span className="mp-kpi-note">{formatDateTime(row.original.updated_at)}</span>,
        },
        {
          id: "actions",
          header: () => t("walletPage.table.columns.actions"),
          enableSorting: false,
          enableHiding: false,
          cell: ({ row }) => {
            const canSuspend = canApplyPassAction(row.original, "suspend")
            const canActivate = canApplyPassAction(row.original, "activate")
            const canRevoke = canApplyPassAction(row.original, "revoke")
            return (
              <div className="flex flex-wrap gap-2">
                {canSuspend ? (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!writable || updatingPassID === row.original.id || batchUpdatingPassAction.length > 0}
                    onClick={() => void updatePassStatus(row.original, "suspend")}
                  >
                    {t("walletPage.actions.suspend")}
                  </Button>
                ) : null}
                {canActivate ? (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!writable || updatingPassID === row.original.id || batchUpdatingPassAction.length > 0}
                    onClick={() => void updatePassStatus(row.original, "activate")}
                  >
                    {t("walletPage.actions.activate")}
                  </Button>
                ) : null}
                {canRevoke ? (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!writable || updatingPassID === row.original.id || batchUpdatingPassAction.length > 0}
                    onClick={() => void updatePassStatus(row.original, "revoke")}
                  >
                    {t("walletPage.actions.revoke")}
                  </Button>
                ) : null}
                {!writable ? <span className="mp-kpi-note">{t("walletPage.hints.readOnlyBoundaryOnly")}</span> : null}
              </div>
            )
          },
        },
      ]
      if (writable) {
        definition.unshift({
          id: "select",
          header: ({ table }) => {
            const pageRows = table.getRowModel().rows
            const allSelected =
              pageRows.length > 0 && pageRows.every((row) => selectedPassIDSet.has(row.original.id))
            return (
              <input
                aria-label="select all visible wallet passes"
                type="checkbox"
                className="size-4 rounded border"
                disabled={pageRows.length === 0 || batchUpdatingPassAction.length > 0}
                checked={allSelected}
                onChange={(event) => {
                  const visiblePassIDs = pageRows.map((row) => row.original.id)
                  if (visiblePassIDs.length === 0) {
                    return
                  }
                  setSelectedPassIDs((current) => {
                    if (!event.target.checked) {
                      const removable = new Set(visiblePassIDs)
                      return current.filter((item) => !removable.has(item))
                    }
                    const merged = new Set(current)
                    visiblePassIDs.forEach((item) => merged.add(item))
                    return Array.from(merged)
                  })
                }}
              />
            )
          },
          enableSorting: false,
          enableHiding: false,
          cell: ({ row }) => (
            <input
              aria-label={`select wallet pass ${row.original.id}`}
              type="checkbox"
              className="size-4 rounded border"
              checked={selectedPassIDSet.has(row.original.id)}
              disabled={batchUpdatingPassAction.length > 0}
              onChange={(event) => onSelectPass(row.original.id, event.target.checked)}
            />
          ),
        })
      }
      return definition
    },
    [
      batchUpdatingPassAction.length,
      resolvingSaveLinkPassID,
      selectedPassIDSet,
      templateByID,
      updatingPassID,
      writable,
    ]
  )
  const passTable = useReactTable({
    columns: passColumns,
    data: filteredPasses,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getRowId: (row) => row.id,
    state: {
      columnVisibility: passColumnVisibility,
      pagination: {
        pageIndex: Math.max(0, passPage - 1),
        pageSize: passPageSize,
      },
      sorting: passSorting,
    },
    onColumnVisibilityChange: setPassColumnVisibility,
    onSortingChange: setPassSorting,
  })
  const selectedFilteredPassCount = useMemo(() => {
    return filteredPasses.reduce((sum, item) => (selectedPassIDSet.has(item.id) ? sum + 1 : sum), 0)
  }, [filteredPasses, selectedPassIDSet])
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
        const message = err instanceof Error ? err.message : t("walletPage.errors.loadOpsFailed")
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
          enterpriseFlowContext.templateHint === "employee"
            ? t("walletPage.scenarios.employeeMobile.templateName")
            : t("walletPage.scenarios.visitorQr.templateName")
        )
      }
    }
    const targetLabel = enterpriseFlowContext.targetName || enterpriseFlowContext.targetEmail || enterpriseFlowContext.targetID
    const syncRecordLabel = enterpriseFlowContext.syncJobID
      ? t("walletPage.summaries.enterprise.syncRecordLabel", {
          source: enterpriseFlowContext.syncSource || t("walletPage.summaries.enterprise.syncSourceDefault"),
          jobID: enterpriseFlowContext.syncJobID,
          statusSuffix: enterpriseFlowContext.syncStatus
            ? t("walletPage.summaries.enterprise.syncStatusSuffix", { status: enterpriseFlowContext.syncStatus })
            : "",
        })
      : ""
    const workerAlertLabel =
      enterpriseFlowContext.workerAlertLevel || enterpriseFlowContext.workerFilterHint
        ? t("walletPage.summaries.enterprise.workerAlertLabel", {
            tenant: enterpriseFlowContext.workerAlertTenantID || tenantID || t("walletPage.summaries.enterprise.currentTenant"),
            level: enterpriseFlowContext.workerAlertLevel || enterpriseFlowContext.workerFilterHint,
            thresholdSuffix:
              enterpriseFlowContext.workerAlertFailed && enterpriseFlowContext.workerAlertThreshold
                ? t("walletPage.summaries.enterprise.workerAlertThresholdSuffix", {
                    failed: enterpriseFlowContext.workerAlertFailed,
                    threshold: enterpriseFlowContext.workerAlertThreshold,
                  })
                : "",
          })
        : ""
    const receiptRecoveryActionLabel =
      enterpriseFlowContext.segmentHint === "receipt_recovery"
        ? receiptRecoveryActionHintLabel(t, resolveReceiptRecoveryActionHint(enterpriseFlowContext.receiptRecoveryActionHint))
        : ""
    const issuanceSummarySegments = [
      t("walletPage.summaries.enterprise.flowAcceptedPrefix", {
        flowLabel,
        stageLabel: enterpriseWalletStageLabel(t, enterpriseFlowContext.stage),
      }),
      tenantLabel ? t("walletPage.summaries.enterprise.organizationSegment", { tenant: tenantLabel }) : "",
      enterpriseFlowSegmentDescriptor
        ? t("walletPage.summaries.enterprise.segmentHintSegment", { segment: enterpriseFlowSegmentDescriptor })
        : "",
      receiptRecoveryActionLabel
        ? t("walletPage.summaries.enterprise.recoveryConclusionSegment", { action: receiptRecoveryActionLabel })
        : "",
      targetLabel ? t("walletPage.summaries.enterprise.targetLocatedSegment", { target: targetLabel }) : "",
      batchTargetIDsHint.length > 0
        ? t("walletPage.summaries.enterprise.batchTargetsPrefilledSegment", {
            count: batchTargetIDsHint.length,
            matched: enterpriseBatchTargetStats.matchedIDs.length,
            missing: enterpriseBatchTargetStats.missingIDs.length,
          })
        : "",
      syncRecordLabel ? t("walletPage.summaries.enterprise.syncRecordSegment", { syncRecord: syncRecordLabel }) : "",
      workerAlertLabel ? t("walletPage.summaries.enterprise.workerAlertSegment", { workerAlert: workerAlertLabel }) : "",
    ]
    setIssuanceSummary(issuanceSummarySegments.join("") + t("walletPage.summaries.enterprise.sentenceEnd"))
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
          t("walletPage.summaries.enterprise.receiptRecoveryRetryDelivery", {
            retryableCount: batchRetryableDeliveryNotifications.length,
            repairableCount: repairableRetryableDeliveryPasses.length,
          })
        )
      } else if (receiptRecoveryActionHint === "repair_pass_status") {
        setIssuanceSummary(
          t("walletPage.summaries.enterprise.receiptRecoveryRepairStatus", {
            repairableCount: repairableRetryableDeliveryPasses.length,
            retryableCount: batchRetryableDeliveryNotifications.length,
          })
        )
      } else if (receiptRecoveryActionHint === "review_closed") {
        setIssuanceSummary(
          t("walletPage.summaries.enterprise.receiptRecoveryReviewClosed", {
            failedCount: failedDeliveryNotifications.length,
          })
        )
      } else if (targetQuery && matchedPasses.length > 0) {
        setIssuanceSummary(
          t("walletPage.summaries.enterprise.matchedPassesDirectDelivery", { count: matchedPasses.length })
        )
      } else if (targetQuery) {
        setIssuanceSummary(t("walletPage.summaries.enterprise.noExistingPassPrefilledSingle"))
      } else {
        setIssuanceSummary(t("walletPage.summaries.enterprise.backToReceiptRecoveryLoop"))
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
        t("walletPage.summaries.enterprise.matchedPassesDirectDelivery", { count: matchedPasses.length })
      )
    } else {
      setIssuanceSummary(t("walletPage.summaries.enterprise.noExistingPassPrefilledSingle"))
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
    setPassPage(1)
  }, [passPageSize, passQuery, passStatusFilter, passTargetTypeFilter, passTemplateFilter])

  useEffect(() => {
    if (passPage > passMaxPage) {
      setPassPage(passMaxPage)
    }
  }, [passMaxPage, passPage])

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
      setAggregateWarning(t("walletPage.warnings.aggregateTenantLoadFailed", { failedCount }))
    } else {
      setAggregateWarning("")
    }
  }

  async function saveAlertSubscription() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
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
      const message = err instanceof Error ? err.message : t("walletPage.errors.saveAlertSubscriptionFailed")
      setError(message)
    } finally {
      setSavingSubscription(false)
    }
  }

  async function submitTemplate(payload: {
    name: string
    classID: string
    passType: "employee" | "visitor"
    status: "active" | "inactive"
    styleConfig: string
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.name.trim()) {
      setError(t("walletPage.errors.templateNameRequired"))
      return false
    }

    setCreatingTemplate(true)
    setIssuanceSummary("")
    setError("")
    try {
      const created = await createWalletTemplate(token, {
        tenant_id: nextTenantID,
        name: payload.name.trim(),
        pass_type: payload.passType,
        class_id: payload.classID.trim() || undefined,
        style_config: parseStyleConfig(payload.styleConfig),
        status: payload.status,
        actor: "web_admin.wallet.template",
      })
      setIssuanceSummary(
        t("walletPage.summaries.templateCreated", { templateName: created.name, passType: passTypeLabel(t, created.pass_type) })
      )
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
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.createTemplateFailed")
      setError(message)
      return false
    } finally {
      setCreatingTemplate(false)
    }
  }

  async function toggleTemplateStatus(template: WalletPassTemplate) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
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
      setIssuanceSummary(
        t("walletPage.summaries.templateStatusUpdated", {
          templateName: updated.name,
          status: updated.status === "active" ? t("walletPage.labels.templateStatus.enabled") : t("walletPage.labels.templateStatus.disabled"),
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.updateTemplateStatusFailed")
      setError(message)
    } finally {
      setUpdatingTemplateID("")
    }
  }

  async function submitSingleIssue(payload: {
    templateID: string
    targetID: string
    expiresAt: string
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.templateID.trim()) {
      setError(t("walletPage.errors.issueTemplateRequired"))
      return false
    }
    if (!payload.targetID.trim()) {
      setError(t("walletPage.errors.issueTargetRequired"))
      return false
    }

    setIssuingSingle(true)
    setIssuanceSummary("")
    setError("")
    try {
      const targetType = resolveTargetType(payload.templateID)
      const pass = await issueWalletPass(token, {
        tenant_id: nextTenantID,
        template_id: payload.templateID,
        target_type: targetType,
        target_id: payload.targetID.trim(),
        expires_at: normalizeDateTimeInput(payload.expiresAt),
        actor: "web_admin.wallet.issue.single",
      })
      setIssuanceSummary(
        t("walletPage.summaries.singleIssueSubmitted", {
          targetType: targetTypeLabel(t, pass.target_type),
          targetID: pass.target_id,
          status: passStatusLabel(t, pass.status),
        })
      )
      setSingleTargetID("")
      setSingleExpiresAt("")
      setLastIssuedJobs([])
      await loadWalletOps(nextTenantID)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.singleIssueFailed")
      setError(message)
      return false
    } finally {
      setIssuingSingle(false)
    }
  }

  async function submitBatchIssue(payload: {
    templateID: string
    targetIDs: string[]
    expiresAt: string
    executionMode: "inline" | "queued"
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.templateID.trim()) {
      setError(t("walletPage.errors.batchIssueTemplateRequired"))
      return false
    }

    const targetIDs = payload.targetIDs
    if (targetIDs.length === 0) {
      setError(t("walletPage.errors.batchIssueTargetsRequired"))
      return false
    }

    setIssuingBatch(true)
    setIssuanceSummary("")
    setError("")
    try {
      const targetType = resolveTargetType(payload.templateID)
      const result = await issueWalletPassBatch(token, {
        tenant_id: nextTenantID,
        template_id: payload.templateID,
        target_type: targetType,
        target_ids: targetIDs,
        expires_at: normalizeDateTimeInput(payload.expiresAt),
        execution_mode: payload.executionMode,
        actor: "web_admin.wallet.issue.batch",
      })
      setLastIssuedJobs(result.items)
      setIssuanceSummary(
        t("walletPage.summaries.batchIssueSubmitted", {
          count: targetIDs.length,
          targetType: targetTypeLabel(t, targetType),
          executionMode: result.execution_mode,
        })
      )
      setBatchTargetIDs("")
      setBatchExpiresAt("")
      await loadWalletOps(nextTenantID)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.batchIssueFailed")
      setError(message)
      return false
    } finally {
      setIssuingBatch(false)
    }
  }

  async function submitPassDelivery(payload: {
    passID: string
    emailEnabled: boolean
    whatsAppEnabled: boolean
    emailRecipients: string
    whatsAppRecipients: string
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.passID.trim()) {
      setError(t("walletPage.errors.deliveryPassRequired"))
      return false
    }

    const channels: string[] = []
    if (payload.emailEnabled) {
      channels.push("email")
    }
    if (payload.whatsAppEnabled) {
      channels.push("whatsapp")
    }
    if (channels.length === 0) {
      setError(t("walletPage.errors.deliveryChannelRequired"))
      return false
    }

    setDispatchingDelivery(true)
    setIssuanceSummary("")
    setError("")
    try {
      const created = await dispatchWalletPassDelivery(token, {
        tenant_id: nextTenantID,
        pass_id: payload.passID,
        channels,
        email_recipients: payload.emailEnabled ? parseReceiverValues(payload.emailRecipients) : [],
        whatsapp_recipients: payload.whatsAppEnabled ? parseReceiverValues(payload.whatsAppRecipients) : [],
        actor: "web_admin.wallet.delivery.dispatch",
      })
      setIssuanceSummary(
        t("walletPage.summaries.deliverySubmitted", {
          targetID: created.target_id,
          status: deliveryNotificationStatusLabel(t, created.status),
          reasonSuffix: created.reason ? t("walletPage.summaries.reasonSuffix", { reason: created.reason }) : "",
        })
      )
      await loadWalletOps(nextTenantID)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.deliverySubmitFailed")
      setError(message)
      return false
    } finally {
      setDispatchingDelivery(false)
    }
  }

  async function onSubmitPassDeliveryForm(values: PassDeliveryFormValues) {
    await submitPassDelivery({
      passID: values.delivery_pass_id.trim(),
      emailEnabled: values.delivery_email_enabled,
      whatsAppEnabled: values.delivery_whatsapp_enabled,
      emailRecipients: values.delivery_email_recipients,
      whatsAppRecipients: values.delivery_whatsapp_recipients,
    })
  }

  async function retryDeliveryNotification(notificationID: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
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
        t("walletPage.summaries.deliveryRetrySubmitted", {
          targetID: retried.target_id,
          status: deliveryNotificationStatusLabel(t, retried.status),
          reasonSuffix: retried.reason ? t("walletPage.summaries.reasonSuffix", { reason: retried.reason }) : "",
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.deliveryRetryFailed")
      setError(message)
    } finally {
      setRetryingDeliveryNotificationID("")
    }
  }

  async function retryDeliveryNotificationBatch() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }
    if (batchRetryableDeliveryNotifications.length === 0) {
      setIssuanceSummary(t("walletPage.summaries.noRetryableDeliveryChannels"))
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
        t("walletPage.summaries.batchRetryDeliverySubmitted", {
          total: settled.length,
          successCount,
          failedSuffix: failedCount > 0 ? t("walletPage.summaries.batchRetryDeliveryFailedSuffix", { failedCount }) : "",
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.batchRetryDeliveryFailed")
      setError(message)
    } finally {
      setBatchRetryingDelivery(false)
    }
  }

  function seedBatchReissueFromRetryableDelivery() {
    if (reissueTargetIDsByRetryableDelivery.length === 0) {
      setIssuanceSummary(t("walletPage.summaries.noReissueTargets"))
      return
    }
    if (!reissueTemplateByRetryableDelivery) {
      setError(t("walletPage.errors.reissueTemplateMissing"))
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
      t("walletPage.summaries.reissueDraftSeeded", {
        count: reissueTargetIDsByRetryableDelivery.length,
        templateName: reissueTemplateByRetryableDelivery.name,
        statusHint:
          reissueTemplateByRetryableDelivery.status === "active"
            ? t("walletPage.summaries.reissueTemplateReady")
            : t("walletPage.summaries.reissueTemplateInactive"),
      })
    )
  }

  async function repairRetryableDeliveryPassStatusBatch() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }
    if (repairableRetryableDeliveryPasses.length === 0) {
      setIssuanceSummary(t("walletPage.summaries.noRepairableDeliveryPasses"))
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
        t("walletPage.summaries.batchRepairPassStatusSubmitted", {
          total: settled.length,
          successCount,
          failedSuffix: failedCount > 0 ? t("walletPage.summaries.batchRepairPassStatusFailedSuffix", { failedCount }) : "",
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.batchRepairPassStatusFailed")
      setError(message)
    } finally {
      setRepairingRetryablePasses(false)
    }
  }

  function keepMissingEnterpriseTargetsInBatchDraft() {
    if (enterpriseBatchTargetStats.missingIDs.length === 0) {
      setIssuanceSummary(t("walletPage.summaries.enterprise.allTargetsMatched"))
      return
    }
    setBatchTargetIDs(enterpriseBatchTargetStats.missingIDs.join("\n"))
    setError("")
    setIssuanceSummary(
      t("walletPage.summaries.enterprise.missingTargetsSeeded", { count: enterpriseBatchTargetStats.missingIDs.length })
    )
  }

  function keepIssueReadyEnterpriseTargetsInBatchDraft() {
    if (issueReadyEnterpriseMissingTargetIDs.length === 0) {
      setIssuanceSummary(t("walletPage.summaries.enterprise.noIssueReadyTargets"))
      return
    }
    setBatchTargetIDs(issueReadyEnterpriseMissingTargetIDs.join("\n"))
    setError("")
    setIssuanceSummary(
      t("walletPage.summaries.enterprise.issueReadyTargetsSeeded", { count: issueReadyEnterpriseMissingTargetIDs.length })
    )
  }

  function restoreEnterpriseTargetsToBatchDraft() {
    if (enterpriseBatchTargetStats.targetIDs.length === 0) {
      return
    }
    setBatchTargetIDs(enterpriseBatchTargetStats.targetIDs.join("\n"))
    setError("")
    setIssuanceSummary(
      t("walletPage.summaries.enterprise.allPrefilledTargetsRestored", { count: enterpriseBatchTargetStats.targetIDs.length })
    )
  }

  async function submitPhysicalCardTask(payload: {
    passID: string
    taskType: "issue" | "reissue" | "loss_report"
    cardNumber: string
    note: string
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.passID.trim()) {
      setError(t("walletPage.errors.physicalTaskPassRequired"))
      return false
    }

    setCreatingPhysicalCardTask(true)
    setIssuanceSummary("")
    setError("")
    try {
      const created = await createWalletPhysicalCardTask(token, {
        tenant_id: nextTenantID,
        pass_id: payload.passID,
        task_type: payload.taskType,
        card_number: payload.cardNumber.trim() || undefined,
        note: payload.note.trim() || undefined,
        actor: "web_admin.wallet.physical_card.create",
      })
      setIssuanceSummary(
        t("walletPage.summaries.physicalTaskCreated", {
          targetID: created.target_id,
          taskType: physicalCardTaskTypeLabel(t, created.task_type),
          status: physicalCardTaskStatusLabel(t, created.status),
        })
      )
      setPhysicalTaskCardNumber("")
      setPhysicalTaskNote("")
      await loadWalletOps(nextTenantID)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.createPhysicalTaskFailed")
      setError(message)
      return false
    } finally {
      setCreatingPhysicalCardTask(false)
    }
  }

  async function onSubmitPhysicalCardTaskForm(values: PhysicalCardTaskFormValues) {
    await submitPhysicalCardTask({
      passID: values.physical_task_pass_id.trim(),
      taskType: values.physical_task_type,
      cardNumber: values.physical_task_card_number || "",
      note: values.physical_task_note || "",
    })
  }

  async function advancePhysicalCardTask(task: WalletPhysicalCardTask, status: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
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
        t("walletPage.summaries.physicalTaskUpdated", {
          targetID: updated.target_id,
          taskType: physicalCardTaskTypeLabel(t, updated.task_type),
          status: physicalCardTaskStatusLabel(t, updated.status),
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.updatePhysicalTaskFailed")
      setError(message)
    } finally {
      setUpdatingPhysicalCardTaskID("")
    }
  }

  async function updatePassStatus(pass: WalletPassInstance, action: "activate" | "suspend" | "revoke") {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
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
        t("walletPage.summaries.passStatusUpdated", {
          targetType: targetTypeLabel(t, updated.target_type),
          targetID: updated.target_id,
          status: passStatusLabel(t, updated.status),
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.updatePassStatusFailed")
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

  async function updateSelectedPasses(action: "activate" | "suspend" | "revoke") {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    const targetPasses = filteredPasses.filter(
      (item) => selectedPassIDSet.has(item.id) && canApplyPassAction(item, action)
    )
    if (targetPasses.length === 0) {
      setError(t("walletPage.errors.noActionableSelectedPasses"))
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
      setIssuanceSummary(
        t("walletPage.summaries.batchPassActionCompleted", {
          action:
            action === "activate"
              ? t("walletPage.actions.activate")
              : action === "suspend"
                ? t("walletPage.actions.suspend")
                : t("walletPage.actions.revoke"),
          succeeded,
          failed,
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.batchUpdatePassStatusFailed")
      setError(message)
    } finally {
      setBatchUpdatingPassAction("")
    }
  }

  async function dispatchAlertsNow() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
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
        t("walletPage.summaries.alertDispatchEvaluated", {
          totalAlerts: result.total_alerts,
          dispatched: result.dispatched,
          skipped: result.skipped,
          failed: result.failed,
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.dispatchAlertsFailed")
      setError(message)
    } finally {
      setDispatchingAlerts(false)
    }
  }

  async function retryAlertNotification(notificationID: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
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
      setDispatchSummary(
        t("walletPage.summaries.alertRetryResult", {
          status: result.status,
          reasonSuffix: result.reason ? ` (${result.reason})` : "",
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.retryAlertNotificationFailed")
      setError(message)
    } finally {
      setRetryingAlertNotificationID("")
    }
  }

  async function applyFilters(nextTenantID?: string) {
    const effectiveTenantID = (nextTenantID ?? tenantID).trim()
    if (!effectiveTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
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
      const message = err instanceof Error ? err.message : t("walletPage.errors.refreshOpsFailed")
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
    setIssuanceSummary(t("walletPage.summaries.scenarioLedgerFocused", { scenario: t(preset.titleKey) }))
  }

  function applyScenarioPreset(scenarioID: WalletScenarioKind) {
    const preset = walletIssuanceScenarioPresetByID.get(scenarioID)
    if (!preset) {
      return
    }
    const activeTemplate = activeTemplateByScenario.get(scenarioID)
    setTemplateName(t(preset.templateNameKey))
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
        ? t("walletPage.summaries.scenarioPresetAppliedWithTemplate", {
            scenario: t(preset.titleKey),
            templateName: activeTemplate.name,
          })
        : t("walletPage.summaries.scenarioPresetAppliedWithoutTemplate", { scenario: t(preset.titleKey) })
    )
  }

  async function copySaveLink(pass: WalletPassInstance) {
    if (!pass.save_link) {
      return
    }
    if (typeof navigator === "undefined" || !navigator.clipboard?.writeText) {
      setError(t("walletPage.errors.clipboardUnsupported"))
      return
    }
    try {
      await navigator.clipboard.writeText(pass.save_link)
      setIssuanceSummary(t("walletPage.summaries.saveLinkCopied", { targetID: pass.target_id }))
      setError("")
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.copySaveLinkFailed")
      setError(message)
    }
  }

  function patchPassRecord(passID: string, updater: (current: WalletPassInstance) => WalletPassInstance) {
    setPasses((current) => current.map((item) => (item.id === passID ? updater(item) : item)))
  }

  async function refreshPassRecord(pass: WalletPassInstance) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      throw new Error(t("walletPage.errors.tenantRequired"))
    }
    const latest = await getWalletPass(token, pass.id, nextTenantID)
    patchPassRecord(pass.id, () => latest)
    return latest
  }

  async function resolvePassSaveLink(pass: WalletPassInstance) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      throw new Error(t("walletPage.errors.tenantRequired"))
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
      setIssuanceSummary(t("walletPage.summaries.saveLinkRefreshed", { targetID: pass.target_id }))
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
      const message = err instanceof Error ? err.message : t("walletPage.errors.refreshSaveLinkFailed")
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
      const message = err instanceof Error ? err.message : t("walletPage.errors.generateQrCodeFailed")
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
      <WalletPageOverview writable={writable} readOnlyBoundaryHint={readOnlyBoundaryHint} />

      <WalletAlertConfigCard
        platformViewer={platformViewer}
        tenantID={tenantID}
        onTenantChange={onTenantChange}
        tenants={tenants}
        windowSeconds={windowSeconds}
        onWindowSecondsChange={setWindowSeconds}
        maxRetry={maxRetry}
        onMaxRetryChange={setMaxRetry}
        alertThreshold={alertThreshold}
        onAlertThresholdChange={setAlertThreshold}
        archiveLimit={archiveLimit}
        onArchiveLimitChange={setArchiveLimit}
        trendBucketCount={trendBucketCount}
        onTrendBucketCountChange={setTrendBucketCount}
        loading={loading}
        refreshing={refreshing}
        onApplyFilters={() => {
          void applyFilters()
        }}
      />

      <div className="space-y-4" data-testid="wallet-operations-workspace">
          <div className="rounded-xl border bg-muted/15 px-4 py-3">
            <p className="text-sm font-medium">{t("walletPage.operations.title")}</p>
            <p className="mt-1 text-sm text-muted-foreground">
              {t("walletPage.operations.description")}
            </p>
          </div>

          <WalletOperationsOverviewCards
            scenarioPresets={walletIssuanceScenarioPresets}
            templateScenarioCounts={templateScenarioCounts}
            passScenarioCounts={passScenarioCounts}
            activeTemplateNameByScenario={new Map(
              Array.from(activeTemplateByScenario.entries()).map(([scenarioID, template]) => [scenarioID, template.name])
            )}
            onApplyScenarioPreset={(scenarioID) => applyScenarioPreset(scenarioID as WalletScenarioKind)}
            activeEmployeeTemplateName={activeEmployeeTemplate?.name ?? ""}
            employeePassCount={employeePassCount}
            showDirectorySyncLink={enterpriseWorkspaceAccessible}
            showAccessGrantLinks={accessWorkspaceAccessible}
            onUseEmployeeTemplate={() => {
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
            activeVisitorTemplateName={activeVisitorTemplate?.name ?? ""}
            visitorPassCount={visitorPassCount}
            onUseVisitorTemplate={() => {
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
            suspendedPassCount={suspendedPassCount}
            revocablePassCount={revocablePassCount}
            onViewSuspended={() => {
              setPassStatusFilter("suspended")
              setPassTargetTypeFilter("all")
              setPassTemplateFilter("all")
              setPassQuery("")
            }}
            onViewAllStatuses={() => {
              setPassStatusFilter("all")
              setPassTargetTypeFilter("all")
              setPassTemplateFilter("all")
              setPassQuery("")
            }}
          />

          <div className="grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)]">
            <WalletTemplateManagerCard
              writable={writable}
              readOnlyBoundaryHint={readOnlyBoundaryHint}
              creatingTemplate={creatingTemplate}
              loading={loading}
              refreshing={refreshing}
              templateName={templateName}
              onTemplateNameChange={setTemplateName}
              templateClassID={templateClassID}
              onTemplateClassIDChange={setTemplateClassID}
              templatePassType={templatePassType}
              onTemplatePassTypeChange={setTemplatePassType}
              templateStatus={templateStatus}
              onTemplateStatusChange={setTemplateStatus}
              templateStyleConfig={templateStyleConfig}
              onTemplateStyleConfigChange={setTemplateStyleConfig}
              onSubmitTemplate={submitTemplate}
              issuanceSummary={issuanceSummary}
              templates={templates}
              onSetDefaultTemplate={(templateID) => {
                setSingleTemplateID(templateID)
                setBatchTemplateID(templateID)
                setPassTemplateFilter(templateID)
              }}
              onToggleTemplateStatus={(template) => {
                void toggleTemplateStatus(template)
              }}
              updatingTemplateID={updatingTemplateID}
              templateStatusVariant={templateStatusVariant}
              passTypeLabel={(type) => passTypeLabel(t, type)}
              getTemplateScenarioLabel={(template) => walletScenarioLabel(t, inferTemplateScenario(template))}
              formatDateTime={formatDateTime}
            />
            <WalletIssueJobQueueCard
              writable={writable}
              loading={loading}
              refreshing={refreshing}
              enterpriseBatchTargetStats={enterpriseBatchTargetStats}
              enterpriseMissingTargetBreakdown={enterpriseMissingTargetBreakdown}
              enterpriseSyncIssueHint={enterpriseSyncIssueHint}
              issueReadyEnterpriseMissingTargetIDs={issueReadyEnterpriseMissingTargetIDs}
              onKeepIssueReadyEnterpriseTargets={keepIssueReadyEnterpriseTargetsInBatchDraft}
              onKeepMissingEnterpriseTargets={keepMissingEnterpriseTargetsInBatchDraft}
              onRestoreEnterpriseTargets={restoreEnterpriseTargetsToBatchDraft}
              accessDirectoryReviewLink={accessDirectoryReviewLink}
              canOpenAccessReview={accessWorkspaceAccessible}
              enterpriseAlertsIssueLink={enterpriseAlertsIssueLink}
              canOpenEnterpriseReview={enterpriseWorkspaceAccessible}
              hasWorkerAlertFlowHints={hasWorkerAlertFlowHints}
              enterpriseSyncWorkerReviewLink={enterpriseSyncWorkerReviewLink}
              onSubmitSingleIssue={submitSingleIssue}
              targetTypeLabel={(type) => targetTypeLabel(t, type)}
              singleTargetType={singleTargetType}
              singleTemplateID={singleTemplateID}
              onSingleTemplateIDChange={setSingleTemplateID}
              templates={templates}
              singleTargetID={singleTargetID}
              onSingleTargetIDChange={setSingleTargetID}
              singleExpiresAt={singleExpiresAt}
              onSingleExpiresAtChange={setSingleExpiresAt}
              selectedSingleTemplate={selectedSingleTemplate}
              getTemplateScenarioLabel={(template) => walletScenarioLabel(t, inferTemplateScenario(template))}
              getTemplateScenarioHint={(template) => walletScenarioHint(t, inferTemplateScenario(template))}
              issuingSingle={issuingSingle}
              onSubmitBatchIssue={submitBatchIssue}
              batchTargetType={batchTargetType}
              batchTemplateID={batchTemplateID}
              onBatchTemplateIDChange={setBatchTemplateID}
              batchExpiresAt={batchExpiresAt}
              onBatchExpiresAtChange={setBatchExpiresAt}
              batchExecutionMode={batchExecutionMode}
              onBatchExecutionModeChange={setBatchExecutionMode}
              batchTargetIDs={batchTargetIDs}
              onBatchTargetIDsChange={setBatchTargetIDs}
              selectedBatchTemplate={selectedBatchTemplate}
              issuingBatch={issuingBatch}
              lastIssuedJobs={lastIssuedJobs}
              formatDateTime={formatDateTime}
            />
          </div>

          <WalletDeliveryWorkspacePanels
            scenarioPresets={walletIssuanceScenarioPresets}
            activeTemplateByScenario={activeTemplateByScenario}
            passScenarioCounts={passScenarioCounts}
            saveLinkScenarioCounts={saveLinkScenarioCounts}
            deliveryDeskPasses={deliveryDeskPasses}
            templateByID={templateByID}
            resolvingSaveLinkPassID={resolvingSaveLinkPassID}
            onFocusPassScenario={(scenarioID) => focusPassScenario(scenarioID as WalletScenarioKind)}
            onOpenPassQrDialog={openPassQrDialog}
            onCopySaveLink={copySaveLink}
            onRefreshPassSaveLink={refreshPassSaveLink}
            passStatusVariant={passStatusVariant}
            passStatusLabel={(status) => passStatusLabel(t, status)}
            walletScenarioLabel={(pass, template) => walletScenarioLabel(t, inferPassScenario(pass, template))}
            inferScenarioID={(pass, template) => inferPassScenario(pass, template)}
            deliveryMethodLabel={(template) => deliveryMethodLabel(t, getTemplateDeliveryMethod(template))}
            accessMediumLabel={(template) => accessMediumLabel(t, getTemplateAccessMedium(template))}
            dispatchChannelLabels={(template) => dispatchChannelLabels(t, template)}
            deliveryHint={(pass, template) => deliveryHint(t, pass, template)}
          />

          <div className="grid gap-4 xl:grid-cols-[minmax(0,0.98fr)_minmax(0,1.02fr)]">
            <WalletSendDeliveryCard
              writable={writable}
              loading={loading}
              refreshing={refreshing}
              dispatchingDelivery={dispatchingDelivery}
              resolvingSaveLinkPassID={resolvingSaveLinkPassID}
              readOnlyBoundaryHint={readOnlyBoundaryHint}
              deliveryPassID={deliveryPassID}
              deliveryEmailEnabled={deliveryEmailEnabled}
              deliveryWhatsAppEnabled={deliveryWhatsAppEnabled}
              deliverablePasses={deliverablePasses}
              selectedDeliveryPass={selectedDeliveryPass}
              selectedDeliveryTemplate={selectedDeliveryTemplate}
              templateByID={templateByID}
              passDeliveryForm={passDeliveryForm}
              deliveryEmailRecipientsField={deliveryEmailRecipientsField}
              deliveryWhatsAppRecipientsField={deliveryWhatsAppRecipientsField}
              passDeliveryFormError={passDeliveryFormError}
              onDeliveryPassIDChange={setDeliveryPassID}
              onDeliveryEmailEnabledChange={setDeliveryEmailEnabled}
              onDeliveryWhatsAppEnabledChange={setDeliveryWhatsAppEnabled}
              onDeliveryEmailRecipientsChange={setDeliveryEmailRecipients}
              onDeliveryWhatsAppRecipientsChange={setDeliveryWhatsAppRecipients}
              onSubmit={onSubmitPassDeliveryForm}
              onOpenPassQrDialog={openPassQrDialog}
              onCopySaveLink={copySaveLink}
              onRefreshPassSaveLink={refreshPassSaveLink}
              passStatusVariant={passStatusVariant}
              passStatusLabel={(status) => passStatusLabel(t, status)}
              walletScenarioLabel={(pass, template) => walletScenarioLabel(t, inferPassScenario(pass, template))}
            />

            <WalletDeliveryReceiptsCard
              writable={writable}
              loading={loading}
              refreshing={refreshing}
              batchRetryingDelivery={batchRetryingDelivery}
              repairingRetryablePasses={repairingRetryablePasses}
              retryingDeliveryNotificationID={retryingDeliveryNotificationID}
              receiptRecoveryFlowStatus={receiptRecoveryFlowStatus}
              receiptSplitStatus={receiptSplitStatus}
              receiptRemediationStatus={receiptRemediationStatus}
              receiptReviewStatus={receiptReviewStatus}
              failedDeliveryNotificationsCount={failedDeliveryNotifications.length}
              retryableDeliveryNotificationsCount={retryableDeliveryNotifications.length}
              nonRetryableFailedDeliveryNotificationsCount={nonRetryableFailedDeliveryNotifications.length}
              batchRetryableDeliveryNotificationsCount={batchRetryableDeliveryNotifications.length}
              repairableRetryableDeliveryPassesCount={repairableRetryableDeliveryPasses.length}
              reissueTargetIDsByRetryableDeliveryCount={reissueTargetIDsByRetryableDelivery.length}
              recentDeliveryNotifications={recentDeliveryNotifications}
              deliveryRetryQuery={deliveryRetryQuery}
              enterpriseReceiptRecoveryReviewLink={enterpriseReceiptRecoveryReviewLink}
              enterpriseAlertsIssueLink={enterpriseAlertsIssueLink}
              hasWorkerAlertFlowHints={hasWorkerAlertFlowHints}
              enterpriseSyncWorkerReviewLink={enterpriseSyncWorkerReviewLink}
              passByID={passByID}
              templateByID={templateByID}
              receiptRecoveryStatusVariant={receiptRecoveryStatusVariant}
              receiptRecoveryStatusLabel={(status) => receiptRecoveryStatusLabel(t, status)}
              deliveryNotificationStatusVariant={deliveryNotificationStatusVariant}
              deliveryNotificationStatusLabel={(status) => deliveryNotificationStatusLabel(t, status)}
              formatDateTime={formatDateTime}
              onRetryBatch={() => {
                void retryDeliveryNotificationBatch()
              }}
              onRepairBatch={() => {
                void repairRetryableDeliveryPassStatusBatch()
              }}
              onSeedBatchReissue={seedBatchReissueFromRetryableDelivery}
              onOpenPassQrDialog={openPassQrDialog}
              onCopySaveLink={copySaveLink}
              onRetryDeliveryNotification={(notificationID) => {
                void retryDeliveryNotification(notificationID)
              }}
            />
          </div>

          <WalletIssuedPassesCard
            templates={templates}
            passTable={passTable}
            passQuery={passQuery}
            passStatusFilter={passStatusFilter}
            passTargetTypeFilter={passTargetTypeFilter}
            passTemplateFilter={passTemplateFilter}
            filteredPassCount={filteredPasses.length}
            selectedFilteredPassCount={selectedFilteredPassCount}
            passesCount={passes.length}
            hasPassFilters={hasPassFilters}
            writable={writable}
            loading={loading}
            batchUpdatingPassAction={batchUpdatingPassAction}
            passPage={passPage}
            passPageSize={passPageSize}
            onPassQueryChange={setPassQuery}
            onPassStatusFilterChange={setPassStatusFilter}
            onPassTargetTypeFilterChange={setPassTargetTypeFilter}
            onPassTemplateFilterChange={setPassTemplateFilter}
            onPassPageChange={setPassPage}
            onPassPageSizeChange={setPassPageSize}
            onClearFilters={() => {
              setPassQuery("")
              setPassStatusFilter("all")
              setPassTargetTypeFilter("all")
              setPassTemplateFilter("all")
              setPassPage(1)
            }}
            onClearSelection={() => setSelectedPassIDs([])}
            onUpdateSelectedPasses={(action) => {
              void updateSelectedPasses(action)
            }}
          />
      </div>

        <WalletAdvancedWorkspace
          physicalCardTasksSectionProps={{
            writable,
            loading,
            refreshing,
            creatingPhysicalCardTask,
            updatingPhysicalCardTaskID,
            readOnlyBoundaryHint,
            physicalTaskPassID,
            physicalTaskType,
            employeeCardEligiblePasses,
            selectedPhysicalTaskPass,
            selectedPhysicalTaskTemplate,
            recentPhysicalCardTasks,
            passByID,
            templateByID,
            physicalCardTaskForm,
            physicalTaskCardNumberField,
            physicalTaskNoteField,
            physicalCardTaskFormError,
            onPhysicalTaskPassIDChange: setPhysicalTaskPassID,
            onPhysicalTaskTypeChange: setPhysicalTaskType,
            onPhysicalTaskCardNumberChange: setPhysicalTaskCardNumber,
            onPhysicalTaskNoteChange: setPhysicalTaskNote,
            onSubmit: onSubmitPhysicalCardTaskForm,
            onFocusEmployeePhysicalScenario: () => focusPassScenario("employee_physical"),
            onOpenPassQrDialog: openPassQrDialog,
            onAdvancePhysicalCardTask: advancePhysicalCardTask,
            passStatusVariant,
            passStatusLabel: (status) => passStatusLabel(t, status),
            walletScenarioLabel: (pass, template) => walletScenarioLabel(t, inferPassScenario(pass, template)),
            physicalTaskStatusVariant: physicalCardTaskStatusVariant,
            physicalTaskStatusLabel: (status) => physicalCardTaskStatusLabel(t, status),
            nextPhysicalTaskActions: (task) => nextPhysicalCardTaskActions(t, task),
            formatDateTime,
          }}
          alertSubscriptionCardProps={{
            writable,
            readOnlyBoundaryHint,
            subscriptionEnabled,
            onSubscriptionEnabledChange: setSubscriptionEnabled,
            subscriptionEmailEnabled,
            onSubscriptionEmailEnabledChange: setSubscriptionEmailEnabled,
            subscriptionWhatsAppEnabled,
            onSubscriptionWhatsAppEnabledChange: setSubscriptionWhatsAppEnabled,
            subscriptionThreshold,
            onSubscriptionThresholdChange: setSubscriptionThreshold,
            subscriptionWindowSeconds,
            onSubscriptionWindowSecondsChange: setSubscriptionWindowSeconds,
            subscriptionCooldownSeconds,
            onSubscriptionCooldownSecondsChange: setSubscriptionCooldownSeconds,
            subscriptionReceiverGroups,
            onSubscriptionReceiverGroupsChange: setSubscriptionReceiverGroups,
            subscription,
            formatDateTime,
            dispatchSummary,
            loading,
            refreshing,
            dispatchingAlerts,
            savingSubscription,
            onDispatchAlertsNow: () => {
              void dispatchAlertsNow()
            },
            onSaveAlertSubscription: () => {
              void saveAlertSubscription()
            },
          }}
          riskOverviewPanelsProps={{
            loading,
            platformViewer,
            effectiveError,
            aggregateWarning,
            metrics,
            alertItems,
            formatDurationSeconds,
            aggregateStats,
            tenantAggregates,
            formatDateTime,
          }}
          alertTrendPanelsProps={{
            loading,
            metrics,
            metricsTrend,
            trendPeakUpdated,
            formatDurationSeconds,
            formatTimeLabel,
            alertItems,
            windowErrorCodeRows,
            formatDateTime,
          }}
          alertNotificationRecordsCardProps={{
            loading,
            refreshing,
            writable,
            alertNotifications,
            retryingAlertNotificationID,
            onRetryAlertNotification: (notificationID) => {
              void retryAlertNotification(notificationID)
            },
            formatDateTime,
          }}
          dlqCleanupArchivesCardProps={{
            loading,
            archives,
            formatDateTime,
            formatDurationSeconds,
          }}
        />

      <WalletPassQrDialog
        open={qrDialogOpen}
        pass={qrDialogPass}
        template={qrDialogTemplate}
        loading={qrDialogLoading}
        previewURL={qrDialogPreviewURL}
        saveLink={qrDialogSaveLink}
        resolvingSaveLinkPassID={resolvingSaveLinkPassID}
        onOpenChange={(open) => {
          setQrDialogOpen(open)
          if (!open) {
            setQrDialogPass(null)
            setQrDialogSaveLink("")
            setQrDialogSVG("")
            setQrDialogLoading(false)
          }
        }}
        onDownloadSvg={downloadQrSVG}
        onCopyLink={() => {
          if (qrDialogPass) {
            void copySaveLink({ ...qrDialogPass, save_link: qrDialogSaveLink })
          }
        }}
        onRefreshLink={() => {
          if (qrDialogPass) {
            void refreshPassSaveLink(qrDialogPass)
          }
        }}
        passStatusVariant={passStatusVariant}
        passStatusLabel={(status) => passStatusLabel(t, status)}
        scenarioLabel={(pass, template) => walletScenarioLabel(t, inferPassScenario(pass, template))}
        deliveryMethodLabel={(template) => deliveryMethodLabel(t, getTemplateDeliveryMethod(template))}
        accessMediumLabel={(template) => accessMediumLabel(t, getTemplateAccessMedium(template))}
      />
	    </div>
	  )
	}
