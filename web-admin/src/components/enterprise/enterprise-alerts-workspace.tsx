import { useEffect, useMemo, useState } from "react"
import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { TabsContent } from "@/components/ui/tabs"
import {
  type EnterpriseJITProvisionApproval,
  type EnterpriseSyncJob,
  type EnterpriseSyncWorkerAlertSummaryItem,
} from "@/lib/api"

type EnterpriseSection = "employees" | "sync" | "idp" | "alerts"
type AlertLandingView = "overview" | "approval_backlog" | "directory_exceptions"
type AlertSegmentHint = "receipt_recovery"
type AlertSegmentStatus = "pending" | "attention" | "ready"

type EnterpriseAttentionItem = {
  actionLabel: string
  description: string
  onClick: () => void
  title: string
}

type EnterpriseLandingAction = {
  alertsView?: AlertLandingView
  kind: "section" | "route"
  label: string
  section?: EnterpriseSection
  to?: string
}

type EnterpriseLandingCard = {
  action: EnterpriseLandingAction
  description: string
  returnAction?: EnterpriseLandingAction
  returnHint?: string
  statusLabel: string
  statusVariant: "outline" | "secondary" | "destructive"
  title: string
}

type EnterpriseRecoveryAction = {
  blockerCount: number
  description: string
  nextAction: {
    description: string
    kind: "section" | "route"
    label: string
    section?: EnterpriseSection
    title: string
    to?: string
  }
  title: string
}

type EnterpriseAlertsWorkspaceProps = {
  alertRecoveryAction: EnterpriseRecoveryAction
  approvals: EnterpriseJITProvisionApproval[]
  approvalActionID?: string | null
  approvalActionBusy?: boolean
  attentionItems: EnterpriseAttentionItem[]
  directoryLink: string
  formatDateTime: (value?: string) => string
  goToSection: (section: EnterpriseSection) => void
  landingCards: EnterpriseLandingCard[]
  loading: boolean
  onBatchReviewApprovals?: (approvalIDs: string[], decision: "approved" | "rejected") => Promise<void>
  onBatchUpdateApprovalExternalSync?: (approvalIDs: string[], status: "synced" | "failed") => Promise<void>
  onReviewApproval?: (approvalID: string, decision: "approved" | "rejected") => Promise<void>
  onUpdateApprovalExternalSync?: (approvalID: string, status: "synced" | "failed") => Promise<void>
  syncLink: string
  initialFilterContextKey?: string
  initialLandingView?: AlertLandingView
  initialApprovalQuery?: string
  initialDirectoryQuery?: string
  initialSegmentHint?: AlertSegmentHint
  initialSegmentStatus?: AlertSegmentStatus
  initialWorkerFilter?: "all" | "alerting" | "hot" | "stable"
  initialSyncSourceFilter?: string
  initialSyncStatusFilter?: "all" | "attention" | "rejected" | "deactivated" | "healthy"
  policiesLink: string
  selectedTenantName?: string
  statusBadgeVariant: (status?: string) => "outline" | "secondary" | "destructive"
  syncJobs: EnterpriseSyncJob[]
  walletLink: string
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[]
}

export function EnterpriseAlertsWorkspace({
  alertRecoveryAction,
  approvals,
  approvalActionID,
  approvalActionBusy,
  attentionItems,
  directoryLink,
  formatDateTime,
  goToSection,
  landingCards,
  loading,
  onBatchReviewApprovals,
  onBatchUpdateApprovalExternalSync,
  onReviewApproval,
  onUpdateApprovalExternalSync,
  syncLink,
  initialFilterContextKey,
  initialLandingView,
  initialApprovalQuery,
  initialDirectoryQuery,
  initialSegmentHint,
  initialSegmentStatus,
  initialWorkerFilter,
  initialSyncSourceFilter,
  initialSyncStatusFilter,
  policiesLink,
  selectedTenantName,
  statusBadgeVariant,
  syncJobs,
  walletLink,
  workerAlerts,
}: EnterpriseAlertsWorkspaceProps) {
  const [landingView, setLandingView] = useState<AlertLandingView>("overview")
  const [approvalStatusFilter, setApprovalStatusFilter] = useState("all")
  const [approvalSyncFilter, setApprovalSyncFilter] = useState<"all" | "failed" | "pending" | "success" | "none">("all")
  const [approvalQuery, setApprovalQuery] = useState("")
  const [syncStatusFilter, setSyncStatusFilter] = useState<"all" | "attention" | "rejected" | "deactivated" | "healthy">("all")
  const [syncSourceFilter, setSyncSourceFilter] = useState("all")
  const [workerFilter, setWorkerFilter] = useState<"all" | "alerting" | "hot" | "stable">("all")
  const [directoryQuery, setDirectoryQuery] = useState("")
  const [appliedInitialFilterContextKey, setAppliedInitialFilterContextKey] = useState("")

  const normalizeStatus = (value?: string) => (value || "").trim().toLowerCase()

  const classifyExternalSyncStatus = (value?: string): "none" | "failed" | "pending" | "success" | "other" => {
    const normalized = normalizeStatus(value)
    if (!normalized) {
      return "none"
    }
    if (/fail|error|reject/.test(normalized)) {
      return "failed"
    }
    if (/pending|queue|processing|running/.test(normalized)) {
      return "pending"
    }
    if (/success|synced|complete|ok/.test(normalized)) {
      return "success"
    }
    return "other"
  }

  const classifySyncJob = (item: EnterpriseSyncJob): "attention" | "rejected" | "deactivated" | "healthy" => {
    const normalizedStatus = normalizeStatus(item.status)
    if (normalizedStatus !== "completed") {
      return "attention"
    }
    if (item.rejected > 0) {
      return "rejected"
    }
    if (item.deactivated > 0) {
      return "deactivated"
    }
    return "healthy"
  }

  const classifyWorkerAlert = (item: EnterpriseSyncWorkerAlertSummaryItem): "alerting" | "hot" | "stable" => {
    if (item.count === 0) {
      return "stable"
    }
    if (item.last_threshold > 0 && item.last_failed >= item.last_threshold) {
      return "hot"
    }
    return "alerting"
  }

  const nextFlowAction = useMemo<EnterpriseLandingAction>(() => {
    if (alertRecoveryAction.nextAction.kind === "section") {
      return {
        kind: "section",
        section: alertRecoveryAction.nextAction.section,
        label: alertRecoveryAction.nextAction.label,
      }
    }
    return {
      kind: "route",
      to: alertRecoveryAction.nextAction.to,
      label: alertRecoveryAction.nextAction.label,
    }
  }, [alertRecoveryAction.nextAction])

  const approvalStatusOptions = useMemo(() => {
    const dynamicStatuses = Array.from(new Set(approvals.map((item) => normalizeStatus(item.status)).filter(Boolean)))
    const preferredOrder = ["pending", "approved", "rejected"]
    const ordered = preferredOrder.filter((status) => dynamicStatuses.includes(status))
    const extra = dynamicStatuses.filter((status) => !preferredOrder.includes(status)).sort((a, b) => a.localeCompare(b))
    return ["all", ...ordered, ...extra]
  }, [approvals])

  const syncSourceOptions = useMemo(() => {
    const sources = Array.from(new Set(syncJobs.map((item) => item.source).filter(Boolean)))
    return ["all", ...sources]
  }, [syncJobs])

  useEffect(() => {
    if (!initialFilterContextKey || appliedInitialFilterContextKey === initialFilterContextKey) {
      return
    }
    if (initialLandingView) {
      setLandingView(initialLandingView)
    }
    if (initialSyncStatusFilter) {
      setSyncStatusFilter(initialSyncStatusFilter)
    }
    if (initialSyncSourceFilter && initialSyncSourceFilter !== "all") {
      const canApplySource = syncJobs.some((item) => item.source === initialSyncSourceFilter)
      setSyncSourceFilter(canApplySource ? initialSyncSourceFilter : "all")
    } else {
      setSyncSourceFilter("all")
    }
    if (initialWorkerFilter) {
      setWorkerFilter(initialWorkerFilter)
    }
    setApprovalQuery(initialApprovalQuery?.trim() || "")
    setDirectoryQuery(initialDirectoryQuery?.trim() || "")
    setAppliedInitialFilterContextKey(initialFilterContextKey)
  }, [
    appliedInitialFilterContextKey,
    initialApprovalQuery,
    initialDirectoryQuery,
    initialFilterContextKey,
    initialLandingView,
    initialWorkerFilter,
    initialSyncSourceFilter,
    initialSyncStatusFilter,
    syncJobs,
  ])

  const filteredApprovals = useMemo(() => {
    const normalizedQuery = approvalQuery.trim().toLowerCase()
    return approvals.filter((item) => {
      const normalizedStatus = normalizeStatus(item.status)
      const syncState = classifyExternalSyncStatus(item.external_sync_status)
      const statusPass = approvalStatusFilter === "all" || normalizedStatus === approvalStatusFilter
      const syncPass = approvalSyncFilter === "all" || syncState === approvalSyncFilter
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.id,
          item.email,
          item.external_id,
          item.provider,
          item.reason,
          item.external_sync_status,
          item.external_sync_ref,
          item.external_sync_last_error,
        ]
          .map((value) => (value || "").toString().toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      return statusPass && syncPass && queryPass
    })
  }, [approvalQuery, approvalStatusFilter, approvalSyncFilter, approvals])
  const batchPendingApprovalIDs = useMemo(
    () => filteredApprovals.filter((item) => normalizeStatus(item.status) === "pending").map((item) => item.id),
    [filteredApprovals]
  )
  const batchSyncMarkableApprovalIDs = useMemo(
    () =>
      filteredApprovals
        .filter(
          (item) =>
            normalizeStatus(item.status) !== "pending" && classifyExternalSyncStatus(item.external_sync_status) !== "success"
        )
        .map((item) => item.id),
    [filteredApprovals]
  )
  const allPendingApprovalIDs = useMemo(
    () => approvals.filter((item) => normalizeStatus(item.status) === "pending").map((item) => item.id),
    [approvals]
  )
  const allFailedSyncApprovalIDs = useMemo(
    () =>
      approvals
        .filter(
          (item) =>
            normalizeStatus(item.status) !== "pending" && classifyExternalSyncStatus(item.external_sync_status) === "failed"
        )
        .map((item) => item.id),
    [approvals]
  )
  const allPendingSyncApprovalIDs = useMemo(
    () =>
      approvals
        .filter(
          (item) =>
            normalizeStatus(item.status) !== "pending" && classifyExternalSyncStatus(item.external_sync_status) === "pending"
        )
        .map((item) => item.id),
    [approvals]
  )

  const filteredSyncJobs = useMemo(() => {
    const normalizedQuery = directoryQuery.trim().toLowerCase()
    return syncJobs.filter((item) => {
      const category = classifySyncJob(item)
      const statusPass = syncStatusFilter === "all" || category === syncStatusFilter
      const sourcePass = syncSourceFilter === "all" || item.source === syncSourceFilter
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.id,
          item.source,
          item.status,
          item.actor,
          String(item.total),
          String(item.created),
          String(item.updated),
          String(item.deactivated),
          String(item.rejected),
        ]
          .map((value) => value.toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      return statusPass && sourcePass && queryPass
    })
  }, [directoryQuery, syncJobs, syncSourceFilter, syncStatusFilter])

  const filteredWorkerAlerts = useMemo(() => {
    const normalizedQuery = directoryQuery.trim().toLowerCase()
    return workerAlerts.filter((item) => {
      const category = classifyWorkerAlert(item)
      const categoryPass = workerFilter === "all" || category === workerFilter
      const queryPass =
        normalizedQuery.length === 0 ||
        [
          item.tenant_id,
          String(item.count),
          String(item.last_failed),
          String(item.last_threshold),
          String(item.last_processed),
          String(item.last_applied),
        ]
          .map((value) => value.toLowerCase())
          .some((value) => value.includes(normalizedQuery))
      return categoryPass && queryPass
    })
  }, [directoryQuery, workerAlerts, workerFilter])

  const approvalCounts = useMemo(() => {
    const statusCount = (status: string) => approvals.filter((item) => normalizeStatus(item.status) === status).length
    const syncCount = (syncType: "failed" | "pending" | "success" | "none") =>
      approvals.filter((item) => classifyExternalSyncStatus(item.external_sync_status) === syncType).length
    return {
      all: approvals.length,
      pending: statusCount("pending"),
      approved: statusCount("approved"),
      rejected: statusCount("rejected"),
      syncFailed: syncCount("failed"),
      syncPending: syncCount("pending"),
      syncSuccess: syncCount("success"),
      syncNone: syncCount("none"),
    }
  }, [approvals])

  const syncJobCounts = useMemo(() => {
    return {
      all: syncJobs.length,
      attention: syncJobs.filter((item) => classifySyncJob(item) === "attention").length,
      rejected: syncJobs.filter((item) => classifySyncJob(item) === "rejected").length,
      deactivated: syncJobs.filter((item) => classifySyncJob(item) === "deactivated").length,
      healthy: syncJobs.filter((item) => classifySyncJob(item) === "healthy").length,
    }
  }, [syncJobs])

  const workerCounts = useMemo(() => {
    return {
      all: workerAlerts.length,
      alerting: workerAlerts.filter((item) => classifyWorkerAlert(item) === "alerting").length,
      hot: workerAlerts.filter((item) => classifyWorkerAlert(item) === "hot").length,
      stable: workerAlerts.filter((item) => classifyWorkerAlert(item) === "stable").length,
    }
  }, [workerAlerts])
  const approvalLandingIssueCount = approvalCounts.pending + approvalCounts.syncFailed + approvalCounts.syncPending
  const directoryLandingIssueCount =
    syncJobCounts.attention + syncJobCounts.rejected + syncJobCounts.deactivated + workerCounts.alerting + workerCounts.hot
  const receiptRecoveryFlowEnabled = initialSegmentHint === "receipt_recovery"
  const receiptRecoveryBlockerCount = approvalLandingIssueCount + directoryLandingIssueCount
  const receiptRecoverySegmentStatus: AlertSegmentStatus =
    initialSegmentStatus || (receiptRecoveryBlockerCount > 0 ? "attention" : "ready")
  const receiptRecoverySegmentStatusLabel =
    receiptRecoverySegmentStatus === "ready"
      ? "已承接"
      : receiptRecoverySegmentStatus === "attention"
        ? "待处理"
        : "待补齐"
  const receiptRecoverySegmentStatusVariant: "outline" | "secondary" | "destructive" =
    receiptRecoverySegmentStatus === "ready"
      ? "outline"
      : receiptRecoverySegmentStatus === "attention"
        ? "secondary"
        : "destructive"
  const receiptRecoveryBackflowStatus: AlertSegmentStatus = receiptRecoveryBlockerCount > 0 ? "attention" : "ready"
  const receiptRecoveryQueryHint = useMemo(() => {
    const first = approvalQuery.trim() || directoryQuery.trim()
    if (first) {
      return first
    }
    return (initialApprovalQuery || initialDirectoryQuery || "").trim()
  }, [approvalQuery, directoryQuery, initialApprovalQuery, initialDirectoryQuery])
  const receiptRecoveryBackflowLinks = useMemo(
    () => ({
      retry: withRouteHints(walletLink, {
        segment_hint: "receipt_recovery",
        segment_status_hint: receiptRecoveryBackflowStatus,
        receipt_recovery_action_hint: "retry_delivery",
        target_hint: receiptRecoveryQueryHint,
        target_id: receiptRecoveryQueryHint,
      }),
      repair: withRouteHints(walletLink, {
        segment_hint: "receipt_recovery",
        segment_status_hint: receiptRecoveryBackflowStatus,
        receipt_recovery_action_hint: "repair_pass_status",
        target_hint: receiptRecoveryQueryHint,
        target_id: receiptRecoveryQueryHint,
      }),
      closed: withRouteHints(walletLink, {
        segment_hint: "receipt_recovery",
        segment_status_hint: "ready",
        receipt_recovery_action_hint: "review_closed",
        target_hint: receiptRecoveryQueryHint,
        target_id: receiptRecoveryQueryHint,
      }),
    }),
    [receiptRecoveryBackflowStatus, receiptRecoveryQueryHint, walletLink]
  )
  const showOverviewCards = landingView === "overview"
  const showApprovalCards = landingView === "overview" || landingView === "approval_backlog"
  const showDirectoryCards = landingView === "overview" || landingView === "directory_exceptions"

  const resolveApprovalAction = (item: EnterpriseJITProvisionApproval): EnterpriseLandingAction => {
    const syncState = classifyExternalSyncStatus(item.external_sync_status)
    const normalizedStatus = normalizeStatus(item.status)

    if (syncState === "failed") {
      return {
        kind: "section",
        section: "idp",
        label: "去企业登录复核",
      }
    }
    if (normalizedStatus === "approved") {
      return {
        kind: "route",
        to: directoryLink,
        label: "回流员工目录",
      }
    }
    if (normalizedStatus === "pending") {
      return {
        kind: "section",
        section: "idp",
        label: "去企业登录复核",
      }
    }
    return nextFlowAction
  }

  const resolveSyncJobAction = (item: EnterpriseSyncJob): EnterpriseLandingAction => {
    const category = classifySyncJob(item)
    if (category === "attention") {
      return {
        kind: "section",
        section: "sync",
        label: "去导入与同步",
      }
    }
    if (category === "rejected") {
      return {
        kind: "section",
        section: "alerts",
        alertsView: "directory_exceptions",
        label: "去目录异常落地页",
      }
    }
    if (category === "deactivated") {
      return {
        kind: "route",
        to: directoryLink,
        label: "复核停用对象",
      }
    }
    return nextFlowAction
  }

  const resolveWorkerAction = (item: EnterpriseSyncWorkerAlertSummaryItem): EnterpriseLandingAction => {
    const category = classifyWorkerAlert(item)
    if (category === "hot" || category === "alerting") {
      return {
        kind: "section",
        section: "sync",
        label: "去导入与同步",
      }
    }
    return nextFlowAction
  }

  const approvalClosureItems = useMemo(() => {
    const pendingCount = approvals.filter((item) => normalizeStatus(item.status) === "pending").length
    const syncFailedCount = approvals.filter((item) => classifyExternalSyncStatus(item.external_sync_status) === "failed").length
    const syncPendingCount = approvals.filter((item) => classifyExternalSyncStatus(item.external_sync_status) === "pending").length

    return [
      {
        key: "pending",
        title: "待审批积压",
        statusLabel: `${pendingCount} 条`,
        statusVariant: pendingCount > 0 ? ("secondary" as const) : ("outline" as const),
        description: pendingCount > 0 ? "先清理待审批记录，避免企业登录后的自动开户持续堆积。" : "当前没有待审批积压。",
        onLocate: () => {
          setLandingView("approval_backlog")
          setApprovalStatusFilter("pending")
          setApprovalSyncFilter("all")
        },
        locateLabel: "定位待审批",
        batchActions: [
          {
            key: "approve_pending",
            label: `批量批准（${allPendingApprovalIDs.length}）`,
            disabled: !onBatchReviewApprovals || allPendingApprovalIDs.length === 0 || approvalActionBusy,
            onClick: () => {
              if (!onBatchReviewApprovals || allPendingApprovalIDs.length === 0) {
                return
              }
              void onBatchReviewApprovals(allPendingApprovalIDs, "approved")
            },
          },
          {
            key: "reject_pending",
            label: `批量拒绝（${allPendingApprovalIDs.length}）`,
            disabled: !onBatchReviewApprovals || allPendingApprovalIDs.length === 0 || approvalActionBusy,
            onClick: () => {
              if (!onBatchReviewApprovals || allPendingApprovalIDs.length === 0) {
                return
              }
              void onBatchReviewApprovals(allPendingApprovalIDs, "rejected")
            },
          },
        ],
      },
      {
        key: "sync_failed",
        title: "外部回写失败",
        statusLabel: `${syncFailedCount} 条`,
        statusVariant: syncFailedCount > 0 ? ("destructive" as const) : ("outline" as const),
        description: syncFailedCount > 0 ? "外部回写失败会阻塞目录和下游授权，建议优先处理失败回写。" : "当前没有外部回写失败。",
        onLocate: () => {
          setLandingView("approval_backlog")
          setApprovalStatusFilter("all")
          setApprovalSyncFilter("failed")
        },
        locateLabel: "定位回写失败",
        batchActions: [
          {
            key: "mark_failed_synced",
            label: `批量标记已回写（${allFailedSyncApprovalIDs.length}）`,
            disabled:
              !onBatchUpdateApprovalExternalSync || allFailedSyncApprovalIDs.length === 0 || approvalActionBusy,
            onClick: () => {
              if (!onBatchUpdateApprovalExternalSync || allFailedSyncApprovalIDs.length === 0) {
                return
              }
              void onBatchUpdateApprovalExternalSync(allFailedSyncApprovalIDs, "synced")
            },
          },
        ],
      },
      {
        key: "sync_pending",
        title: "外部回写进行中",
        statusLabel: `${syncPendingCount} 条`,
        statusVariant: syncPendingCount > 0 ? ("secondary" as const) : ("outline" as const),
        description: syncPendingCount > 0 ? "持续跟踪回写进行中记录，避免长时间挂起后无人处理。" : "当前没有长尾回写进行中记录。",
        onLocate: () => {
          setLandingView("approval_backlog")
          setApprovalStatusFilter("all")
          setApprovalSyncFilter("pending")
        },
        locateLabel: "定位回写进行中",
        batchActions: [
          {
            key: "mark_pending_synced",
            label: `批量标记已回写（${allPendingSyncApprovalIDs.length}）`,
            disabled:
              !onBatchUpdateApprovalExternalSync || allPendingSyncApprovalIDs.length === 0 || approvalActionBusy,
            onClick: () => {
              if (!onBatchUpdateApprovalExternalSync || allPendingSyncApprovalIDs.length === 0) {
                return
              }
              void onBatchUpdateApprovalExternalSync(allPendingSyncApprovalIDs, "synced")
            },
          },
        ],
      },
    ]
  }, [
    allFailedSyncApprovalIDs,
    allPendingApprovalIDs,
    allPendingSyncApprovalIDs,
    approvalActionBusy,
    approvals,
    onBatchReviewApprovals,
    onBatchUpdateApprovalExternalSync,
  ])

  const directoryClosureItems = useMemo(() => {
    const unfinishedCount = syncJobs.filter((item) => classifySyncJob(item) === "attention").length
    const rejectedCount = syncJobs.filter((item) => classifySyncJob(item) === "rejected").length
    const deactivatedCount = syncJobs.filter((item) => classifySyncJob(item) === "deactivated").length
    const workerIssueCount = workerAlerts.filter((item) => classifyWorkerAlert(item) !== "stable").length

    return [
      {
        key: "unfinished",
        title: "未完成同步任务",
        statusLabel: `${unfinishedCount} 条`,
        statusVariant: unfinishedCount > 0 ? ("secondary" as const) : ("outline" as const),
        description: unfinishedCount > 0 ? "先处理未完成任务，避免目录状态长期处于不确定阶段。" : "当前没有未完成同步任务。",
        onLocate: () => {
          setLandingView("directory_exceptions")
          setSyncStatusFilter("attention")
          setSyncSourceFilter("all")
        },
        locateLabel: "定位未完成任务",
      },
      {
        key: "rejected",
        title: "rejected 目录异常",
        statusLabel: `${rejectedCount} 条`,
        statusVariant: rejectedCount > 0 ? ("destructive" as const) : ("outline" as const),
        description: rejectedCount > 0 ? "先处理 rejected 记录，再继续目录到策略与发放主路径。" : "当前没有 rejected 目录异常。",
        onLocate: () => {
          setLandingView("directory_exceptions")
          setSyncStatusFilter("rejected")
          setSyncSourceFilter("all")
        },
        locateLabel: "定位 rejected",
      },
      {
        key: "deactivated",
        title: "停用影响复核",
        statusLabel: `${deactivatedCount} 条`,
        statusVariant: deactivatedCount > 0 ? ("secondary" as const) : ("outline" as const),
        description: deactivatedCount > 0 ? "复核停用对象是否仍留在用户组、策略或发放对象中。" : "当前没有停用影响待复核。",
        onLocate: () => {
          setLandingView("directory_exceptions")
          setSyncStatusFilter("deactivated")
          setSyncSourceFilter("all")
        },
        locateLabel: "定位停用影响",
      },
      {
        key: "worker",
        title: "worker 告警堆积",
        statusLabel: `${workerIssueCount} 条`,
        statusVariant: workerIssueCount > 0 ? ("destructive" as const) : ("outline" as const),
        description: workerIssueCount > 0 ? "先排查 worker 告警堆积，再继续目录主路径，避免同步质量持续下滑。" : "当前没有 worker 告警堆积。",
        onLocate: () => {
          setLandingView("directory_exceptions")
          setWorkerFilter("alerting")
        },
        locateLabel: "定位 worker 告警",
      },
    ]
  }, [syncJobs, workerAlerts])

  function goToLandingView(view: AlertLandingView) {
    setLandingView(view)
    goToSection("alerts")
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

  function buildSyncJobScopedLinks(item: EnterpriseSyncJob) {
    const category = classifySyncJob(item)
    const sourceLabel = item.source.trim() ? item.source.trim().toUpperCase() : "同步"
    const syncJobID = item.id.trim()
    const syncSource = item.source.trim()
    const syncStatus = item.status.trim()
    const remediationHint =
      category === "rejected" ? "sync_rejected_cleanup" : category === "deactivated" ? "deactivated_cleanup" : "sync_attention"
    const syncSummaryLabel = syncJobID ? `${sourceLabel} 任务 ${syncJobID}` : `${sourceLabel} 同步任务`

    return {
      directory: withRouteHints(directoryLink, {
        group_desc: `来源${syncSummaryLabel}`,
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: category === "deactivated" ? "deactivated" : "",
        group_name: `${sourceLabel} 同步复核`,
        remediation_hint: remediationHint,
        sync_category: category,
        sync_job_id: syncJobID,
        sync_source: syncSource,
        sync_status: syncStatus,
      }),
      policies: withRouteHints(policiesLink, {
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: category === "deactivated" ? "deactivated" : "",
        policy_group: `${sourceLabel} 同步复核`,
        policy_name: `${sourceLabel} 同步异常策略复核`,
        remediation_hint: remediationHint,
        sync_category: category,
        sync_job_id: syncJobID,
        sync_source: syncSource,
        sync_status: syncStatus,
      }),
      wallet: withRouteHints(walletLink, {
        sync_category: category,
        sync_job_id: syncJobID,
        sync_source: syncSource,
        sync_status: syncStatus,
        sync_query_hint: syncJobID,
        target_email: "",
        target_id: "",
        target_ids: "",
        target_name: "",
        template_hint: "employee",
      }),
    }
  }

  function buildWorkerAlertScopedLinks(item: EnterpriseSyncWorkerAlertSummaryItem) {
    const category = classifyWorkerAlert(item)
    const remediationHint = category === "hot" ? "worker_hot_alert" : category === "alerting" ? "worker_alerting" : ""
    const workerScopeLabel = selectedTenantName || item.tenant_id
    const workerSummaryLabel =
      remediationHint && item.last_seen_at
        ? `${workerScopeLabel} worker 告警（${formatDateTime(item.last_seen_at)}）`
        : `${workerScopeLabel} worker 告警`
    const workerFilterHint = category === "hot" ? "hot" : category === "alerting" ? "alerting" : "stable"

    return {
      directory: withRouteHints(directoryLink, {
        group_desc: `来源${workerSummaryLabel}`,
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: "",
        group_name: `${workerScopeLabel} Worker 告警复核`,
        remediation_hint: remediationHint,
        worker_alert_failed: String(item.last_failed),
        worker_alert_level: category,
        worker_alert_last_seen: item.last_seen_at || "",
        worker_alert_tenant_id: item.tenant_id,
        worker_alert_threshold: String(item.last_threshold),
      }),
      policies: withRouteHints(policiesLink, {
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: "",
        policy_group: `${workerScopeLabel} Worker 告警复核`,
        policy_name: `${workerScopeLabel} Worker 告警策略复核`,
        remediation_hint: remediationHint,
        worker_alert_failed: String(item.last_failed),
        worker_alert_level: category,
        worker_alert_last_seen: item.last_seen_at || "",
        worker_alert_tenant_id: item.tenant_id,
        worker_alert_threshold: String(item.last_threshold),
      }),
      wallet: withRouteHints(walletLink, {
        template_hint: "employee",
        worker_alert_failed: String(item.last_failed),
        worker_alert_level: category,
        worker_alert_last_seen: item.last_seen_at || "",
        worker_alert_tenant_id: item.tenant_id,
        worker_alert_threshold: String(item.last_threshold),
        worker_filter_hint: workerFilterHint,
        worker_query_hint: item.tenant_id,
      }),
      sync: withRouteHints(syncLink, {
        sync_focus_hint: "worker_alert",
        worker_filter_hint: workerFilterHint,
        worker_query_hint: item.tenant_id,
        worker_alert_failed: String(item.last_failed),
        worker_alert_level: category,
        worker_alert_last_seen: item.last_seen_at || "",
        worker_alert_tenant_id: item.tenant_id,
        worker_alert_threshold: String(item.last_threshold),
      }),
    }
  }

  const renderActionButton = (action: EnterpriseLandingAction, variant: "default" | "outline" = "outline") => {
    if (action.kind === "section") {
      return (
        <Button
          size="sm"
          variant={variant}
          onClick={() => {
            if (action.section === "alerts" && action.alertsView) {
              setLandingView(action.alertsView)
            }
            goToSection(action.section!)
          }}
        >
          {action.label}
        </Button>
      )
    }
    return (
      <Button asChild size="sm" variant={variant}>
        <Link to={action.to!}>{action.label}</Link>
      </Button>
    )
  }

  return (
    <TabsContent value="alerts">
      <div className="grid gap-4 xl:grid-cols-[1fr_1fr]">
        <Card className="xl:col-span-2">
          <CardHeader>
            <CardTitle className="text-base">审批与异常落地页</CardTitle>
            <CardDescription>按处理目标切换到聚焦落地页：总览、审批积压、目录异常。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <Button
                size="sm"
                variant={landingView === "overview" ? "default" : "outline"}
                onClick={() => goToLandingView("overview")}
              >
                总览
              </Button>
              <Button
                size="sm"
                variant={landingView === "approval_backlog" ? "default" : "outline"}
                onClick={() => goToLandingView("approval_backlog")}
              >
                审批积压（{approvalLandingIssueCount}）
              </Button>
              <Button
                size="sm"
                variant={landingView === "directory_exceptions" ? "default" : "outline"}
                onClick={() => goToLandingView("directory_exceptions")}
              >
                目录异常（{directoryLandingIssueCount}）
              </Button>
            </div>
            <div className="rounded-lg border bg-muted/10 px-3 py-2 text-sm text-muted-foreground">
              {landingView === "overview"
                ? "总览会并排展示落地入口、阻塞项和闭环台账，适合全局排查。"
                : landingView === "approval_backlog"
                  ? "审批积压落地页聚焦 pending / 回写失败 / 回写进行中，并提供就地批量处理。"
                  : "目录异常落地页聚焦未完成同步、rejected、停用影响与 worker 告警。"}
            </div>
          </CardContent>
        </Card>

        {receiptRecoveryFlowEnabled ? (
          <Card className="xl:col-span-2" data-testid="enterprise-alerts-receipt-recovery">
            <CardHeader>
              <CardTitle className="text-base">回执失败复核结论回流</CardTitle>
              <CardDescription>企业页完成复核后，按结论直接回发放页继续重发或状态修复，不再停在异常概览。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="rounded-xl border bg-muted/10 px-4 py-3">
                <div className="flex flex-wrap items-center gap-2">
                  <p className="font-medium">当前复核状态</p>
                  <Badge variant={receiptRecoverySegmentStatusVariant}>{receiptRecoverySegmentStatusLabel}</Badge>
                  <Badge variant={receiptRecoveryBlockerCount > 0 ? "secondary" : "outline"}>
                    {receiptRecoveryBlockerCount > 0 ? `${receiptRecoveryBlockerCount} 个待处理项` : "可回发放页收口"}
                  </Badge>
                </div>
                <p className="mt-1 text-sm text-muted-foreground">
                  {receiptRecoveryBlockerCount > 0
                    ? "审批或目录异常仍有未收口项，建议先给出复核结论并回发放页继续执行。"
                    : "审批与目录异常已基本收口，可回发放页复核重发与状态修复结果。"}
                </p>
              </div>
              {loading ? (
                <p className="mp-kpi-note">正在加载审批与目录状态，稍后可选择回流动作。</p>
              ) : (
                <>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button asChild size="sm">
                      <Link to={receiptRecoveryBackflowLinks.retry} data-testid="enterprise-alerts-receipt-retry-link">
                        结论：继续重发失败通道
                      </Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={receiptRecoveryBackflowLinks.repair}>结论：继续状态修复</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={receiptRecoveryBackflowLinks.closed}>结论：复核已收口</Link>
                    </Button>
                  </div>
                  <p className="mp-kpi-note">
                    {receiptRecoveryQueryHint
                      ? `回流时已携带对象线索“${receiptRecoveryQueryHint}”，用于发放页定位失败回执。`
                      : "当前未带对象线索，回流后将按租户级失败回执范围继续处理。"}
                  </p>
                </>
              )}
            </CardContent>
          </Card>
        ) : null}

        {showOverviewCards ? (
          <Card className="xl:col-span-2">
            <CardHeader>
              <CardTitle className="text-base">落地与回流动作</CardTitle>
              <CardDescription>把同步结果、审批积压和目录异常拆成聚焦处理卡，处理后直接回流目录、策略或发放主路径。</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 md:grid-cols-3">
              {landingCards.map((item) => (
                <div key={item.title} className="rounded-xl border bg-muted/10 px-4 py-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="font-medium">{item.title}</p>
                    <Badge variant={item.statusVariant}>{item.statusLabel}</Badge>
                  </div>
                  <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {item.action.kind === "section" ? (
                      <Button size="sm" variant="outline" onClick={() => goToSection(item.action.section!)}>
                        {item.action.label}
                      </Button>
                    ) : (
                      <Button asChild size="sm" variant="outline">
                        <Link to={item.action.to!}>{item.action.label}</Link>
                      </Button>
                    )}
                    {item.returnAction ? (
                      item.returnAction.kind === "section" ? (
                        <Button size="sm" onClick={() => goToSection(item.returnAction!.section!)}>
                          {item.returnAction.label}
                        </Button>
                      ) : (
                        <Button asChild size="sm">
                          <Link to={item.returnAction.to!}>{item.returnAction.label}</Link>
                        </Button>
                      )
                    ) : null}
                  </div>
                  {item.returnHint ? <p className="mt-2 text-xs text-muted-foreground">{item.returnHint}</p> : null}
                </div>
              ))}
            </CardContent>
          </Card>
        ) : null}

        {showOverviewCards ? (
          <Card className="xl:col-span-2">
            <CardHeader>
              <CardTitle className="text-base">阻塞项处理台</CardTitle>
              <CardDescription>把同步失败、审批积压和企业登录缺口集中在一个地方处理，不再只看静态表格。</CardDescription>
            </CardHeader>
            <CardContent>
              {attentionItems.length === 0 ? (
                <div className="rounded-xl border bg-muted/10 px-4 py-3">
                  <p className="font-medium">当前没有明显阻塞项</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    企业登录、同步和审批主路径当前基本通畅。下一步更适合回到权限策略或发放中心继续推进。
                  </p>
                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    <Button asChild size="sm" variant="outline">
                      <Link to={policiesLink}>去权限策略</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={walletLink}>去凭证发放</Link>
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="grid gap-3 md:grid-cols-2">
                  {attentionItems.map((item) => (
                    <div key={item.title} className="rounded-xl border bg-muted/10 px-4 py-3">
                      <p className="font-medium">{item.title}</p>
                      <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
                      <Button size="sm" variant="outline" className="mt-3" onClick={item.onClick}>
                        {item.actionLabel}
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        ) : null}

        {showApprovalCards ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">审批积压闭环清单</CardTitle>
              <CardDescription>按审批积压类型拆分处理入口，先定位，再回流主路径。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {approvalClosureItems.map((item) => (
                <div key={item.key} className="rounded-lg border bg-muted/15 p-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">{item.title}</p>
                    <Badge variant={item.statusVariant}>{item.statusLabel}</Badge>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">{item.description}</p>
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <Button size="sm" variant="outline" onClick={item.onLocate}>
                      {item.locateLabel}
                    </Button>
                    {renderActionButton(nextFlowAction)}
                  </div>
                  {item.batchActions?.length ? (
                    <div className="mt-2 flex flex-wrap items-center gap-2">
                      {item.batchActions.map((action) => (
                        <Button
                          key={action.key}
                          size="sm"
                          variant="outline"
                          disabled={action.disabled}
                          onClick={action.onClick}
                        >
                          {action.label}
                        </Button>
                      ))}
                    </div>
                  ) : null}
                </div>
              ))}
            </CardContent>
          </Card>
        ) : null}

        {showDirectoryCards ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">目录异常闭环清单</CardTitle>
              <CardDescription>按目录异常类型拆分处理入口，处理后直接回流目录、策略或发放。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {directoryClosureItems.map((item) => (
                <div key={item.key} className="rounded-lg border bg-muted/15 p-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-medium">{item.title}</p>
                    <Badge variant={item.statusVariant}>{item.statusLabel}</Badge>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">{item.description}</p>
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <Button size="sm" variant="outline" onClick={item.onLocate}>
                      {item.locateLabel}
                    </Button>
                    {renderActionButton(nextFlowAction)}
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>
        ) : null}

        <Card className="xl:col-span-2">
          <CardHeader>
            <CardTitle className="text-base">处理完成后回流动作</CardTitle>
            <CardDescription>异常处理不是终点。处理完当前阻塞项后，直接回到目录、策略或发放主路径继续推进。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <div className="flex flex-wrap items-center gap-2">
                <p className="font-medium">{alertRecoveryAction.title}</p>
                <Badge variant={alertRecoveryAction.blockerCount > 0 ? "secondary" : "outline"}>
                  {alertRecoveryAction.blockerCount > 0 ? `${alertRecoveryAction.blockerCount} 个阻塞维度` : "可直接回流"}
                </Badge>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">{alertRecoveryAction.description}</p>
            </div>

            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <p className="font-medium">{alertRecoveryAction.nextAction.title}</p>
              <p className="mt-1 text-sm text-muted-foreground">{alertRecoveryAction.nextAction.description}</p>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                {alertRecoveryAction.nextAction.kind === "section" ? (
                  <Button size="sm" onClick={() => goToSection(alertRecoveryAction.nextAction.section!)}>
                    {alertRecoveryAction.nextAction.label}
                  </Button>
                ) : (
                  <Button asChild size="sm">
                    <Link to={alertRecoveryAction.nextAction.to!}>{alertRecoveryAction.nextAction.label}</Link>
                  </Button>
                )}
                <Button asChild size="sm" variant="outline">
                  <Link to={directoryLink}>去员工与用户组</Link>
                </Button>
                <Button asChild size="sm" variant="outline">
                  <Link to={policiesLink}>去权限策略</Link>
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>

        {showApprovalCards ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">JIT 审批</CardTitle>
              <CardDescription>登录即开户、目录补录和外部同步的审批队列。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
            <div className="rounded-lg border bg-muted/15 p-3">
              <p className="text-sm font-medium">状态筛选</p>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                {approvalStatusOptions.map((status) => (
                  <Button
                    key={status}
                    size="sm"
                    variant={approvalStatusFilter === status ? "default" : "outline"}
                    onClick={() => setApprovalStatusFilter(status)}
                  >
                    {status === "all"
                      ? `全部（${approvalCounts.all}）`
                      : status === "pending"
                        ? `pending（${approvalCounts.pending}）`
                        : status === "approved"
                          ? `approved（${approvalCounts.approved}）`
                          : status === "rejected"
                            ? `rejected（${approvalCounts.rejected}）`
                            : status}
                  </Button>
                ))}
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                {[
                  { value: "all", label: `外部同步全部（${approvalCounts.all}）` },
                  { value: "failed", label: `失败（${approvalCounts.syncFailed}）` },
                  { value: "pending", label: `进行中（${approvalCounts.syncPending}）` },
                  { value: "success", label: `成功（${approvalCounts.syncSuccess}）` },
                  { value: "none", label: `未回写（${approvalCounts.syncNone}）` },
                ].map((item) => (
                  <Button
                    key={item.value}
                    size="sm"
                    variant={approvalSyncFilter === item.value ? "default" : "outline"}
                    onClick={() => setApprovalSyncFilter(item.value as "all" | "failed" | "pending" | "success" | "none")}
                  >
                    {item.label}
                  </Button>
                ))}
              </div>
              <div className="mt-2 flex items-center gap-2">
                <Input
                  value={approvalQuery}
                  onChange={(event) => setApprovalQuery(event.target.value)}
                  placeholder="按邮箱 / external_id / 审批ID筛选"
                  className="h-8"
                />
                {approvalQuery.trim() ? (
                  <Button size="sm" variant="outline" onClick={() => setApprovalQuery("")}>
                    清空
                  </Button>
                ) : null}
              </div>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!onBatchReviewApprovals || batchPendingApprovalIDs.length === 0 || approvalActionBusy}
                  onClick={() => {
                    if (!onBatchReviewApprovals || batchPendingApprovalIDs.length === 0) {
                      return
                    }
                    void onBatchReviewApprovals(batchPendingApprovalIDs, "approved")
                  }}
                >
                  批量批准 pending（{batchPendingApprovalIDs.length}）
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!onBatchReviewApprovals || batchPendingApprovalIDs.length === 0 || approvalActionBusy}
                  onClick={() => {
                    if (!onBatchReviewApprovals || batchPendingApprovalIDs.length === 0) {
                      return
                    }
                    void onBatchReviewApprovals(batchPendingApprovalIDs, "rejected")
                  }}
                >
                  批量拒绝 pending（{batchPendingApprovalIDs.length}）
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!onBatchUpdateApprovalExternalSync || batchSyncMarkableApprovalIDs.length === 0 || approvalActionBusy}
                  onClick={() => {
                    if (!onBatchUpdateApprovalExternalSync || batchSyncMarkableApprovalIDs.length === 0) {
                      return
                    }
                    void onBatchUpdateApprovalExternalSync(batchSyncMarkableApprovalIDs, "synced")
                  }}
                >
                  批量标记已回写（{batchSyncMarkableApprovalIDs.length}）
                </Button>
              </div>
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>邮箱</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>外部同步</TableHead>
                  <TableHead>更新时间</TableHead>
                  <TableHead>处理动作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {!loading && filteredApprovals.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="py-8 text-center text-muted-foreground">
                      当前筛选下没有审批记录。
                    </TableCell>
                  </TableRow>
                ) : null}
                {filteredApprovals.slice(0, 10).map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{item.email}</TableCell>
                    <TableCell>
                      <Badge variant={statusBadgeVariant(item.status)}>{item.status}</Badge>
                    </TableCell>
                    <TableCell>{item.external_sync_status || "-"}</TableCell>
                    <TableCell>{formatDateTime(item.updated_at)}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap items-center gap-2">
                        {normalizeStatus(item.status) === "pending" && onReviewApproval ? (
                          <>
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={approvalActionID === item.id || approvalActionBusy}
                              onClick={() => {
                                void onReviewApproval(item.id, "approved")
                              }}
                            >
                              {approvalActionID === item.id ? "处理中..." : "批准"}
                            </Button>
                            <Button
                              size="sm"
                              variant="outline"
                              disabled={approvalActionID === item.id || approvalActionBusy}
                              onClick={() => {
                                void onReviewApproval(item.id, "rejected")
                              }}
                            >
                              {approvalActionID === item.id ? "处理中..." : "拒绝"}
                            </Button>
                          </>
                        ) : null}
                        {normalizeStatus(item.status) !== "pending" &&
                        classifyExternalSyncStatus(item.external_sync_status) !== "success" &&
                        onUpdateApprovalExternalSync ? (
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={approvalActionID === item.id || approvalActionBusy}
                            onClick={() => {
                              void onUpdateApprovalExternalSync(item.id, "synced")
                            }}
                          >
                            {approvalActionID === item.id ? "处理中..." : "标记已回写"}
                          </Button>
                        ) : null}
                        {renderActionButton(resolveApprovalAction(item), "outline")}
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            </CardContent>
          </Card>
        ) : null}

        {showDirectoryCards ? (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">同步任务与异常</CardTitle>
              <CardDescription>同步任务结果和 worker 告警摘要。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
            <div className="rounded-lg border bg-muted/15 p-3 space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-sm font-medium">最近同步任务</p>
                <Badge variant="secondary">
                  {filteredSyncJobs.length} / {syncJobs.length}
                </Badge>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {[
                  { value: "all", label: `全部（${syncJobCounts.all}）` },
                  { value: "attention", label: `未完成（${syncJobCounts.attention}）` },
                  { value: "rejected", label: `rejected（${syncJobCounts.rejected}）` },
                  { value: "deactivated", label: `停用影响（${syncJobCounts.deactivated}）` },
                  { value: "healthy", label: `稳定（${syncJobCounts.healthy}）` },
                ].map((item) => (
                  <Button
                    key={item.value}
                    size="sm"
                    variant={syncStatusFilter === item.value ? "default" : "outline"}
                    onClick={() =>
                      setSyncStatusFilter(item.value as "all" | "attention" | "rejected" | "deactivated" | "healthy")
                    }
                  >
                    {item.label}
                  </Button>
                ))}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {syncSourceOptions.map((source) => (
                  <Button
                    key={source}
                    size="sm"
                    variant={syncSourceFilter === source ? "default" : "outline"}
                    onClick={() => setSyncSourceFilter(source)}
                  >
                    {source === "all" ? "来源：全部" : `来源：${source}`}
                  </Button>
                ))}
              </div>
              <div className="flex items-center gap-2">
                <Input
                  value={directoryQuery}
                  onChange={(event) => setDirectoryQuery(event.target.value)}
                  placeholder="按任务ID / 来源 / actor / 租户筛选"
                  className="h-8"
                />
                {directoryQuery.trim() ? (
                  <Button size="sm" variant="outline" onClick={() => setDirectoryQuery("")}>
                    清空
                  </Button>
                ) : null}
              </div>
              <div className="mt-3 space-y-2">
                {filteredSyncJobs.slice(0, 6).map((item) => {
                  const category = classifySyncJob(item)
                  const action = resolveSyncJobAction(item)
                  const scopedLinks = buildSyncJobScopedLinks(item)
                  return (
                    <div key={item.id} className="rounded-md border bg-background px-3 py-2 text-sm">
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <p className="font-medium">{item.source}</p>
                          <p className="mp-kpi-note">{item.id}</p>
                        </div>
                        <Badge variant={statusBadgeVariant(item.status)}>{item.status}</Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        created {item.created} / updated {item.updated} / deactivated {item.deactivated} / rejected {item.rejected}
                      </p>
                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        {renderActionButton(action, "outline")}
                        {category !== "healthy" ? (
                          <>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.directory}>按本任务去目录</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.policies}>按本任务去策略</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.wallet}>按本任务去发放</Link>
                            </Button>
                          </>
                        ) : null}
                        {category !== "healthy" ? renderActionButton(nextFlowAction) : null}
                      </div>
                    </div>
                  )
                })}
                {!loading && filteredSyncJobs.length === 0 ? (
                  <p className="text-sm text-muted-foreground">当前筛选下没有同步任务记录。</p>
                ) : null}
              </div>
            </div>

            <div className="rounded-lg border bg-muted/15 p-3 space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-sm font-medium">Worker 告警摘要</p>
                <Badge variant="secondary">
                  {filteredWorkerAlerts.length} / {workerAlerts.length}
                </Badge>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {[
                  { value: "all", label: `全部（${workerCounts.all}）` },
                  { value: "alerting", label: `告警中（${workerCounts.alerting}）` },
                  { value: "hot", label: `超阈值（${workerCounts.hot}）` },
                  { value: "stable", label: `稳定（${workerCounts.stable}）` },
                ].map((item) => (
                  <Button
                    key={item.value}
                    size="sm"
                    variant={workerFilter === item.value ? "default" : "outline"}
                    onClick={() => setWorkerFilter(item.value as "all" | "alerting" | "hot" | "stable")}
                  >
                    {item.label}
                  </Button>
                ))}
              </div>
              <div className="mt-3 space-y-2">
                {filteredWorkerAlerts.slice(0, 6).map((item) => {
                  const action = resolveWorkerAction(item)
                  const category = classifyWorkerAlert(item)
                  const scopedLinks = buildWorkerAlertScopedLinks(item)
                  return (
                    <div key={`${item.tenant_id}-${item.last_seen_at}`} className="rounded-md border bg-background px-3 py-2 text-sm">
                      <div className="flex items-center justify-between gap-3">
                        <p className="font-medium">{selectedTenantName ?? item.tenant_id}</p>
                        <Badge variant={item.count > 0 ? "destructive" : "outline"}>{item.count}</Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        last failed {item.last_failed} / threshold {item.last_threshold} / {formatDateTime(item.last_seen_at)}
                      </p>
                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        {renderActionButton(action, "outline")}
                        {category !== "stable" ? (
                          <>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.directory}>按本告警去目录</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.policies}>按本告警去策略</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.wallet}>按本告警去发放</Link>
                            </Button>
                            <Button asChild size="sm" variant="outline">
                              <Link to={scopedLinks.sync}>按本告警去导入与同步</Link>
                            </Button>
                          </>
                        ) : null}
                        {category !== "stable" ? renderActionButton(nextFlowAction) : null}
                      </div>
                    </div>
                  )
                })}
                {!loading && filteredWorkerAlerts.length === 0 ? (
                  <p className="text-sm text-muted-foreground">当前筛选下没有 worker 告警。</p>
                ) : null}
              </div>
            </div>
            </CardContent>
          </Card>
        ) : null}
      </div>
    </TabsContent>
  )
}
