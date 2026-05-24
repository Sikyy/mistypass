import { useEffect, useMemo, useState } from "react"
import { useLocation } from "react-router"
import { useTranslation } from "react-i18next"

import {
  listWalletPassDeliveries,
  listWalletPasses,
  listWalletPhysicalCardTasks,
  listWalletTemplates,
  type CurrentUser,
  type WalletPassInstance,
} from "@/lib/api"
import { canAccessAccessPage, canAccessEnterprisePage, canManageIssuance } from "@/lib/viewer"
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
  deliveryHint,
  deliveryMethodLabel,
  dispatchChannelLabels,
  enterpriseWalletStageLabel,
  formatDateTime,
  formatDurationSeconds,
  formatTimeLabel,
  getTemplateAccessMedium,
  getTemplateDeliveryMethod,
  inferPassScenario,
  inferTemplateScenario,
  nextPhysicalCardTaskActions,
  parseTargetIDs,
  passStatusLabel,
  passStatusVariant,
  passTypeLabel,
  physicalCardTaskStatusLabel,
  physicalCardTaskStatusVariant,
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
  deliveryNotificationStatusVariant,
  deliveryNotificationStatusLabel,
  type WalletScenarioKind,
} from "./wallet-page-utils"

import { useWalletTenants } from "../hooks/use-wallet-tenants"
import { useWalletTemplates } from "../hooks/use-wallet-templates"
import { useWalletPasses } from "../hooks/use-wallet-passes"
import { useWalletDelivery } from "../hooks/use-wallet-delivery"
import { useWalletAlerts } from "../hooks/use-wallet-alerts"
import { useWalletPhysicalCards } from "../hooks/use-wallet-physical-cards"
import { useWalletGoogleConfig } from "../hooks/use-wallet-google-config"

type WalletPageProps = {
  token: string
  viewer: CurrentUser
}

export function WalletPage({ token, viewer }: WalletPageProps) {
  const { t } = useTranslation()
  const location = useLocation()
  const writable = canManageIssuance(viewer)
  const enterpriseWorkspaceAccessible = canAccessEnterprisePage(viewer)
  const accessWorkspaceAccessible = canAccessAccessPage(viewer)
  const readOnlyBoundaryHint = t("walletPage.hints.readOnlyBoundary")

  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState("")
  const [issuanceSummary, setIssuanceSummary] = useState("")
  const [incomingScenarioApplied, setIncomingScenarioApplied] = useState("")
  const [enterpriseFlowSearchApplied, setEnterpriseFlowSearchApplied] = useState("")
  const [enterpriseFlowDirectActionApplied, setEnterpriseFlowDirectActionApplied] = useState("")

  // --- Tenants hook ---
  const tenants = useWalletTenants({ token, viewer })

  // --- Alerts hook ---
  const alerts = useWalletAlerts({ token, tenantID: tenants.tenantID })

  // --- Google Wallet provider config hook ---
  const googleConfig = useWalletGoogleConfig({ token, tenantID: tenants.tenantID })

  // Unified loadWalletOps orchestrator — coordinates across hooks
  async function loadWalletOps(nextTenantID: string) {
    const [templateItems, passItems, deliveryItems, physicalTaskItems] =
      await Promise.all([
        alerts.loadMetricsAndAlerts(nextTenantID).then(() => listWalletTemplates(token, nextTenantID)),
        listWalletPasses(token, nextTenantID),
        listWalletPassDeliveries(token, { tenant_id: nextTenantID }),
        listWalletPhysicalCardTasks(token, nextTenantID),
      ])

    tpl.setTemplates(
      [...templateItems].sort((a, b) => {
        if (a.status !== b.status) {
          return a.status === "active" ? -1 : 1
        }
        return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
      })
    )
    passes.setPasses(
      [...passItems].sort(
        (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
      )
    )
    delivery.setDeliveryNotifications(
      [...deliveryItems].sort(
        (a, b) => new Date(b.triggered_at).getTime() - new Date(a.triggered_at).getTime()
      )
    )
    physicalCards.setPhysicalCardTasks(
      [...physicalTaskItems].sort(
        (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
      )
    )

    await googleConfig.loadGoogleConfig(nextTenantID)
    await tenants.loadEnterpriseData(nextTenantID)
  }

  // --- Templates hook ---
  const tpl = useWalletTemplates({ token, tenantID: tenants.tenantID, loadWalletOps })

  // --- Passes hook ---
  const passes = useWalletPasses({
    token,
    tenantID: tenants.tenantID,
    writable,
    templates: tpl.templates,
    templateByID: tpl.templateByID,
    resolveTargetType: tpl.resolveTargetType,
    loadWalletOps,
    setGlobalIssuanceSummary: setIssuanceSummary,
    setGlobalError: setError,
  })

  // --- Delivery hook ---
  const delivery = useWalletDelivery({
    token,
    tenantID: tenants.tenantID,
    passes: passes.passes,
    passByID: passes.passByID,
    templateByID: tpl.templateByID,
    deliverablePasses: passes.deliverablePasses,
    activeEmployeeTemplate: tpl.activeEmployeeTemplate,
    activeVisitorTemplate: tpl.activeVisitorTemplate,
    enterpriseFlowContext: tenants.enterpriseFlowContext,
    passQuery: passes.passQuery,
    loadWalletOps,
    setGlobalIssuanceSummary: setIssuanceSummary,
    setGlobalError: setError,
    setBatchTemplateID: passes.setBatchTemplateID,
    setBatchTargetIDs: passes.setBatchTargetIDs,
    setBatchExecutionMode: passes.setBatchExecutionMode,
    setPassTargetTypeFilter: passes.setPassTargetTypeFilter,
    setPassTemplateFilter: passes.setPassTemplateFilter,
  })

  // --- Physical cards hook ---
  const physicalCards = useWalletPhysicalCards({
    token,
    tenantID: tenants.tenantID,
    passes: passes.passes,
    templateByID: tpl.templateByID,
    employeeCardEligiblePasses: passes.employeeCardEligiblePasses,
    loadWalletOps,
    setGlobalIssuanceSummary: setIssuanceSummary,
    setGlobalError: setError,
  })

  // --- Enterprise flow memos that depend on multiple hooks ---
  const enterpriseBatchTargetStats = useMemo(() => {
    const targetIDs = parseTargetIDs(tenants.enterpriseFlowContext?.targetIDs || "")
    if (targetIDs.length === 0) {
      return {
        targetIDs: [] as string[],
        matchedIDs: [] as string[],
        missingIDs: [] as string[],
        hitRate: 0,
      }
    }

    const passTargetIDSet = new Set(
      passes.passes
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
  }, [tenants.enterpriseFlowContext?.targetIDs, passes.passes])

  const enterpriseMissingTargetBreakdown = useMemo(() => {
    const employeeByID = new Map(
      tenants.enterpriseEmployees
        .map((item) => [item.id.trim().toLowerCase(), item] as const)
        .filter(([id]) => Boolean(id))
    )
    const groupNamesByMemberID = new Map<string, string[]>()
    tenants.enterpriseUserGroups.forEach((group) => {
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
  }, [enterpriseBatchTargetStats.missingIDs, tenants.enterpriseEmployees, tenants.enterpriseUserGroups])

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
      return { approvalQuery: "", targetHint: "" }
    }
    const approvalQuery = firstNeedsAlerts.approvalHint?.trim() || firstNeedsAlerts.targetID.trim()
    const targetHint = firstNeedsAlerts.employeeName.trim() || approvalQuery
    return { approvalQuery, targetHint }
  }, [enterpriseMissingTargetBreakdown.rows])

  const accessDirectoryReviewLink = useMemo(() => {
    const query = new URLSearchParams()
    const hintedTenantID = (tenants.enterpriseFlowContext?.tenantID || tenants.tenantID).trim()
    if (hintedTenantID) {
      query.set("tenant_id", hintedTenantID)
    }
    query.set("from", "enterprise")
    query.set("flow", tenants.enterpriseFlowContext?.flow || "sync_to_access")
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
  }, [enterpriseBatchTargetStats.missingIDs, tenants.enterpriseFlowContext, enterpriseMissingTargetBreakdown.rows, tenants.tenantID])

  const enterpriseAlertsIssueLink = useMemo(() => {
    const query = new URLSearchParams()
    const hintedTenantID = (tenants.enterpriseFlowContext?.tenantID || tenants.tenantID).trim()
    if (hintedTenantID) {
      query.set("tenant_id", hintedTenantID)
    }
    query.set("from", "enterprise")
    query.set("flow", tenants.enterpriseFlowContext?.flow || "sync_to_access")
    query.set("stage", "issuance")
    if (enterpriseMissingTargetBreakdown.needsAlertsCount > 0) {
      query.set("alerts_view_hint", "directory_exceptions")
    }
    const explicitTargetQueryHint = resolveEnterpriseTargetQuery(tenants.enterpriseFlowContext)
    const approvalQueryHint = explicitTargetQueryHint || enterpriseAlertsTargetHint.approvalQuery || ""
    if (approvalQueryHint.trim()) {
      query.set("approval_query_hint", approvalQueryHint.trim())
    }
    const targetHint = explicitTargetQueryHint || enterpriseAlertsTargetHint.targetHint || approvalQueryHint
    if (targetHint.trim()) {
      query.set("target_hint", targetHint.trim())
    }
    const explicitWorkerFilter = tenants.enterpriseFlowContext?.workerFilterHint?.trim() || ""
    const workerFilterHint =
      explicitWorkerFilter === "all" ||
      explicitWorkerFilter === "alerting" ||
      explicitWorkerFilter === "hot" ||
      explicitWorkerFilter === "stable"
        ? explicitWorkerFilter
        : tenants.enterpriseFlowContext?.workerAlertLevel === "hot" || tenants.enterpriseFlowContext?.workerAlertLevel === "alerting"
          ? tenants.enterpriseFlowContext.workerAlertLevel
          : ""
    const workerQueryHint =
      tenants.enterpriseFlowContext?.workerQueryHint?.trim() || tenants.enterpriseFlowContext?.workerAlertTenantID?.trim() || ""
    if (workerFilterHint) {
      query.set("alerts_view_hint", "directory_exceptions")
      query.set("worker_filter_hint", workerFilterHint)
    }
    if (workerQueryHint) {
      query.set("worker_query_hint", workerQueryHint)
    }
    if (tenants.enterpriseFlowContext?.workerAlertLevel?.trim()) {
      query.set("worker_alert_level", tenants.enterpriseFlowContext.workerAlertLevel.trim())
    }
    if (tenants.enterpriseFlowContext?.workerAlertTenantID?.trim()) {
      query.set("worker_alert_tenant_id", tenants.enterpriseFlowContext.workerAlertTenantID.trim())
    }
    if (tenants.enterpriseFlowContext?.workerAlertLastSeen?.trim()) {
      query.set("worker_alert_last_seen", tenants.enterpriseFlowContext.workerAlertLastSeen.trim())
    }
    if (tenants.enterpriseFlowContext?.workerAlertFailed?.trim()) {
      query.set("worker_alert_failed", tenants.enterpriseFlowContext.workerAlertFailed.trim())
    }
    if (tenants.enterpriseFlowContext?.workerAlertThreshold?.trim()) {
      query.set("worker_alert_threshold", tenants.enterpriseFlowContext.workerAlertThreshold.trim())
    }
    const explicitSyncCategory = tenants.enterpriseFlowContext?.syncCategory?.trim() || ""
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
      if (tenants.enterpriseFlowContext?.syncSource?.trim()) {
        query.set("sync_source_hint", tenants.enterpriseFlowContext.syncSource.trim())
      }
      if (tenants.enterpriseFlowContext?.syncJobID?.trim()) {
        query.set("sync_query_hint", tenants.enterpriseFlowContext.syncJobID.trim())
      }
    } else if (tenants.enterpriseLatestSyncJob) {
      const normalizedStatus = (tenants.enterpriseLatestSyncJob.status || "").trim().toLowerCase()
      let syncStatusHint: "attention" | "rejected" | "deactivated" | "healthy" = "healthy"
      if (normalizedStatus !== "completed") {
        syncStatusHint = "attention"
      } else if (tenants.enterpriseLatestSyncJob.rejected > 0) {
        syncStatusHint = "rejected"
      } else if (tenants.enterpriseLatestSyncJob.deactivated > 0) {
        syncStatusHint = "deactivated"
      }
      query.set("sync_status_hint", syncStatusHint)
      if (tenants.enterpriseLatestSyncJob.source.trim()) {
        query.set("sync_source_hint", tenants.enterpriseLatestSyncJob.source.trim())
      }
      if (tenants.enterpriseLatestSyncJob.id.trim()) {
        query.set("sync_query_hint", tenants.enterpriseLatestSyncJob.id.trim())
      }
    }
    const nextQuery = query.toString()
    return nextQuery ? `/enterprise?${nextQuery}#alerts` : "/enterprise#alerts"
  }, [
    enterpriseAlertsTargetHint.approvalQuery,
    enterpriseAlertsTargetHint.targetHint,
    tenants.enterpriseFlowContext,
    tenants.enterpriseLatestSyncJob,
    enterpriseMissingTargetBreakdown.needsAlertsCount,
    tenants.tenantID,
  ])

  const enterpriseSyncWorkerReviewLink = useMemo(() => {
    const query = new URLSearchParams()
    const hintedTenantID = (tenants.enterpriseFlowContext?.tenantID || tenants.tenantID).trim()
    if (hintedTenantID) {
      query.set("tenant_id", hintedTenantID)
    }
    query.set("from", "enterprise")
    query.set("flow", tenants.enterpriseFlowContext?.flow || "sync_to_access")
    query.set("stage", "issuance")
    query.set("sync_focus_hint", "worker_alert")
    query.set("worker_review_status_hint", "handled")
    query.set("worker_review_stage_hint", "issuance")
    const explicitWorkerFilter = tenants.enterpriseFlowContext?.workerFilterHint?.trim() || ""
    const workerFilterHint =
      explicitWorkerFilter === "all" ||
      explicitWorkerFilter === "alerting" ||
      explicitWorkerFilter === "hot" ||
      explicitWorkerFilter === "stable"
        ? explicitWorkerFilter
        : tenants.enterpriseFlowContext?.workerAlertLevel === "hot" ||
            tenants.enterpriseFlowContext?.workerAlertLevel === "alerting" ||
            tenants.enterpriseFlowContext?.workerAlertLevel === "stable"
          ? tenants.enterpriseFlowContext.workerAlertLevel
          : ""
    if (workerFilterHint) {
      query.set("worker_filter_hint", workerFilterHint)
    }
    const workerQueryHint =
      tenants.enterpriseFlowContext?.workerQueryHint?.trim() || tenants.enterpriseFlowContext?.workerAlertTenantID?.trim() || ""
    if (workerQueryHint) {
      query.set("worker_query_hint", workerQueryHint)
    }
    if (tenants.enterpriseFlowContext?.workerAlertLevel?.trim()) {
      query.set("worker_alert_level", tenants.enterpriseFlowContext.workerAlertLevel.trim())
    }
    if (tenants.enterpriseFlowContext?.workerAlertTenantID?.trim()) {
      query.set("worker_alert_tenant_id", tenants.enterpriseFlowContext.workerAlertTenantID.trim())
    }
    if (tenants.enterpriseFlowContext?.workerAlertLastSeen?.trim()) {
      query.set("worker_alert_last_seen", tenants.enterpriseFlowContext.workerAlertLastSeen.trim())
    }
    if (tenants.enterpriseFlowContext?.workerAlertFailed?.trim()) {
      query.set("worker_alert_failed", tenants.enterpriseFlowContext.workerAlertFailed.trim())
    }
    if (tenants.enterpriseFlowContext?.workerAlertThreshold?.trim()) {
      query.set("worker_alert_threshold", tenants.enterpriseFlowContext.workerAlertThreshold.trim())
    }
    const nextQuery = query.toString()
    return nextQuery ? `/enterprise?${nextQuery}#sync` : "/enterprise#sync"
  }, [tenants.enterpriseFlowContext, tenants.tenantID])

  const enterpriseReceiptRecoveryReviewLink = useMemo(
    () =>
      withRouteHints(enterpriseAlertsIssueLink, {
        alerts_view_hint: "directory_exceptions",
        segment_hint: "receipt_recovery",
        segment_status_hint: delivery.receiptRecoveryFlowStatus,
      }),
    [enterpriseAlertsIssueLink, delivery.receiptRecoveryFlowStatus]
  )

  // --- Bootstrap effect ---
  useEffect(() => {
    if (tenants.platformViewer && tenants.tenantsQuery.isPending) {
      return
    }

    async function bootstrap() {
      setLoading(true)
      setError("")
      try {
        const tenantItems = tenants.platformViewer ? tenants.tenantsQuery.data ?? [] : []
        tenants.setTenants(tenantItems)

        const nextTenantID = tenants.platformViewer ? tenantItems[0]?.id ?? "" : tenants.viewerTenantID
        tenants.setTenantID(nextTenantID)
        if (!nextTenantID) {
          alerts.resetMetricsAndAlerts()
          googleConfig.resetGoogleConfig()
          tpl.setTemplates([])
          passes.setPasses([])
          tenants.resetEnterpriseData()
          delivery.setDeliveryNotifications([])
          physicalCards.setPhysicalCardTasks([])
          return
        }
        await loadWalletOps(nextTenantID)
        if (tenants.platformViewer) {
          await alerts.loadWalletTenantAggregates(tenantItems)
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : t("walletPage.errors.loadOpsFailed")
        setError(message)
      } finally {
        setLoading(false)
      }
    }

    void bootstrap()
  }, [tenants.platformViewer, tenants.tenantsQuery.data, tenants.tenantsQuery.isPending, token, tenants.viewerTenantID])

  // --- Enterprise flow search effect ---
  useEffect(() => {
    if (loading) {
      return
    }
    if (!tenants.enterpriseFlowContext) {
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

    const incomingTenantID = tenants.enterpriseFlowContext.tenantID
    const canApplyTenant = Boolean(
      incomingTenantID &&
        (tenants.platformViewer
          ? tenants.tenants.some((item) => item.id === incomingTenantID)
          : incomingTenantID === tenants.viewerTenantID)
    )
    if (canApplyTenant && incomingTenantID !== tenants.tenantID) {
      tenants.setTenantID(incomingTenantID)
      void applyFilters(incomingTenantID)
    }

    const tenantLabel = canApplyTenant ? tenants.tenantByID.get(incomingTenantID)?.name || incomingTenantID : ""
    const flowLabel = tenants.enterpriseFlowContext.flow ? `${tenants.enterpriseFlowContext.flow} / ` : ""
    const batchTargetIDsHint = enterpriseBatchTargetStats.targetIDs
    if (tenants.enterpriseFlowContext.targetHint === "user" || tenants.enterpriseFlowContext.targetHint === "visitor") {
      passes.setPassTargetTypeFilter(tenants.enterpriseFlowContext.targetHint)
    }
    const targetIDHint = tenants.enterpriseFlowContext.targetID || tenants.enterpriseFlowContext.targetEmail
    const targetQueryHint = resolveEnterpriseTargetQuery(tenants.enterpriseFlowContext)
    if (targetIDHint && !passes.singleTargetID.trim()) {
      passes.setSingleTargetID(targetIDHint)
    }
    if (targetQueryHint && !passes.passQuery.trim()) {
      passes.setPassQuery(targetQueryHint)
    }
    if (batchTargetIDsHint.length > 0 && !passes.batchTargetIDs.trim()) {
      passes.setBatchTargetIDs(batchTargetIDsHint.join("\n"))
    }
    if (tenants.enterpriseFlowContext.targetEmail && !delivery.deliveryEmailRecipients.trim()) {
      delivery.setDeliveryEmailRecipients(tenants.enterpriseFlowContext.targetEmail)
    }
    if (tenants.enterpriseFlowContext.templateHint === "employee" || tenants.enterpriseFlowContext.templateHint === "visitor") {
      tpl.setTemplatePassType(tenants.enterpriseFlowContext.templateHint)
      if (!tpl.templateName.trim()) {
        tpl.setTemplateName(
          tenants.enterpriseFlowContext.templateHint === "employee"
            ? t("walletPage.scenarios.employeeMobile.templateName")
            : t("walletPage.scenarios.visitorQr.templateName")
        )
      }
    }
    const targetLabel = tenants.enterpriseFlowContext.targetName || tenants.enterpriseFlowContext.targetEmail || tenants.enterpriseFlowContext.targetID
    const syncRecordLabel = tenants.enterpriseFlowContext.syncJobID
      ? t("walletPage.summaries.enterprise.syncRecordLabel", {
          source: tenants.enterpriseFlowContext.syncSource || t("walletPage.summaries.enterprise.syncSourceDefault"),
          jobID: tenants.enterpriseFlowContext.syncJobID,
          statusSuffix: tenants.enterpriseFlowContext.syncStatus
            ? t("walletPage.summaries.enterprise.syncStatusSuffix", { status: tenants.enterpriseFlowContext.syncStatus })
            : "",
        })
      : ""
    const workerAlertLabel =
      tenants.enterpriseFlowContext.workerAlertLevel || tenants.enterpriseFlowContext.workerFilterHint
        ? t("walletPage.summaries.enterprise.workerAlertLabel", {
            tenant: tenants.enterpriseFlowContext.workerAlertTenantID || tenants.tenantID || t("walletPage.summaries.enterprise.currentTenant"),
            level: tenants.enterpriseFlowContext.workerAlertLevel || tenants.enterpriseFlowContext.workerFilterHint,
            thresholdSuffix:
              tenants.enterpriseFlowContext.workerAlertFailed && tenants.enterpriseFlowContext.workerAlertThreshold
                ? t("walletPage.summaries.enterprise.workerAlertThresholdSuffix", {
                    failed: tenants.enterpriseFlowContext.workerAlertFailed,
                    threshold: tenants.enterpriseFlowContext.workerAlertThreshold,
                  })
                : "",
          })
        : ""
    const receiptRecoveryActionLabel =
      tenants.enterpriseFlowContext.segmentHint === "receipt_recovery"
        ? receiptRecoveryActionHintLabel(t, resolveReceiptRecoveryActionHint(tenants.enterpriseFlowContext.receiptRecoveryActionHint))
        : ""
    const issuanceSummarySegments = [
      t("walletPage.summaries.enterprise.flowAcceptedPrefix", {
        flowLabel,
        stageLabel: enterpriseWalletStageLabel(t, tenants.enterpriseFlowContext.stage),
      }),
      tenantLabel ? t("walletPage.summaries.enterprise.organizationSegment", { tenant: tenantLabel }) : "",
      tenants.enterpriseFlowSegmentDescriptor
        ? t("walletPage.summaries.enterprise.segmentHintSegment", { segment: tenants.enterpriseFlowSegmentDescriptor })
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
    passes.batchTargetIDs,
    delivery.deliveryEmailRecipients,
    enterpriseBatchTargetStats,
    enterpriseFlowDirectActionApplied,
    tenants.enterpriseFlowContext,
    tenants.enterpriseFlowSegmentDescriptor,
    enterpriseFlowSearchApplied,
    loading,
    location.search,
    passes.passQuery,
    tenants.platformViewer,
    passes.singleTargetID,
    tenants.tenantByID,
    tenants.tenantID,
    tenants.tenants,
    tpl.templateName,
    tenants.viewerTenantID,
  ])

  // --- Enterprise flow direct action effect ---
  useEffect(() => {
    if (loading || refreshing) {
      return
    }
    if (!tenants.enterpriseFlowContext) {
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

    const incomingTenantID = tenants.enterpriseFlowContext.tenantID.trim()
    const canApplyTenant = Boolean(
      incomingTenantID &&
        (tenants.platformViewer
          ? tenants.tenants.some((item) => item.id === incomingTenantID)
          : incomingTenantID === tenants.viewerTenantID)
    )
    if (canApplyTenant && tenants.tenantID !== incomingTenantID) {
      return
    }

    const targetQuery = resolveEnterpriseTargetQuery(tenants.enterpriseFlowContext)
    const receiptRecoveryFlow = tenants.enterpriseFlowContext.segmentHint.trim() === "receipt_recovery"
    const receiptRecoveryActionHint = resolveReceiptRecoveryActionHint(tenants.enterpriseFlowContext.receiptRecoveryActionHint)

    const matchedPasses =
      targetQuery.length > 0
        ? passes.passes.filter((item) => {
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
      if (!delivery.deliveryPassID.trim() || !matchedPasses.some((item) => item.id === delivery.deliveryPassID)) {
        delivery.setDeliveryPassID(preferredPass.id)
      }
    }

    if (receiptRecoveryFlow) {
      if (receiptRecoveryActionHint === "retry_delivery") {
        setIssuanceSummary(
          t("walletPage.summaries.enterprise.receiptRecoveryRetryDelivery", {
            retryableCount: delivery.batchRetryableDeliveryNotifications.length,
            repairableCount: delivery.repairableRetryableDeliveryPasses.length,
          })
        )
      } else if (receiptRecoveryActionHint === "repair_pass_status") {
        setIssuanceSummary(
          t("walletPage.summaries.enterprise.receiptRecoveryRepairStatus", {
            repairableCount: delivery.repairableRetryableDeliveryPasses.length,
            retryableCount: delivery.batchRetryableDeliveryNotifications.length,
          })
        )
      } else if (receiptRecoveryActionHint === "review_closed") {
        setIssuanceSummary(
          t("walletPage.summaries.enterprise.receiptRecoveryReviewClosed", {
            failedCount: delivery.failedDeliveryNotifications.length,
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
    delivery.batchRetryableDeliveryNotifications.length,
    delivery.deliveryPassID,
    tenants.enterpriseFlowContext,
    enterpriseFlowDirectActionApplied,
    enterpriseFlowSearchApplied,
    delivery.failedDeliveryNotifications.length,
    loading,
    location.search,
    passes.passes,
    tenants.platformViewer,
    delivery.repairableRetryableDeliveryPasses.length,
    refreshing,
    tenants.tenantID,
    tenants.tenants,
    tenants.viewerTenantID,
  ])

  // --- Template/pass default selection sync effects ---
  useEffect(() => {
    const fallbackTemplateID = tpl.pickDefaultTemplateID(tpl.templates)
    if (!tpl.templates.some((item) => item.id === passes.singleTemplateID)) {
      passes.setSingleTemplateID(fallbackTemplateID)
    }
    if (!tpl.templates.some((item) => item.id === passes.batchTemplateID)) {
      passes.setBatchTemplateID(fallbackTemplateID)
    }
  }, [passes.batchTemplateID, passes.singleTemplateID, tpl.templates])

  useEffect(() => {
    const fallbackPassID = passes.deliverablePasses[0]?.id ?? ""
    if (!passes.deliverablePasses.some((item) => item.id === delivery.deliveryPassID)) {
      delivery.setDeliveryPassID(fallbackPassID)
    }
  }, [passes.deliverablePasses, delivery.deliveryPassID])

  useEffect(() => {
    const fallbackPassID = passes.employeeCardEligiblePasses[0]?.id ?? ""
    if (!passes.employeeCardEligiblePasses.some((item) => item.id === physicalCards.physicalTaskPassID)) {
      physicalCards.setPhysicalTaskPassID(fallbackPassID)
    }
  }, [passes.employeeCardEligiblePasses, physicalCards.physicalTaskPassID])

  // --- Scenario preset effect ---
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

  // --- Page-level orchestration functions ---
  async function applyFilters(nextTenantID?: string) {
    const effectiveTenantID = (nextTenantID ?? tenants.tenantID).trim()
    if (!effectiveTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }
    setRefreshing(true)
    setError("")
    try {
      await Promise.all([
        loadWalletOps(effectiveTenantID),
        ...(tenants.platformViewer ? [alerts.loadWalletTenantAggregates(tenants.tenants)] : []),
      ])
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.refreshOpsFailed")
      setError(message)
    } finally {
      setRefreshing(false)
    }
  }

  function onTenantChange(value: string) {
    tenants.setTenantID(value)
    void applyFilters(value)
  }

  function focusPassScenario(scenarioID: WalletScenarioKind) {
    const preset = walletIssuanceScenarioPresetByID.get(scenarioID)
    if (!preset) {
      return
    }
    const activeTemplate = tpl.activeTemplateByScenario.get(scenarioID)
    passes.setPassQuery("")
    passes.setPassStatusFilter("all")
    passes.setPassTargetTypeFilter(preset.targetType)
    passes.setPassTemplateFilter(activeTemplate?.id ?? "all")
    passes.setSelectedPassIDs([])
    setIssuanceSummary(t("walletPage.summaries.scenarioLedgerFocused", { scenario: t(preset.titleKey) }))
  }

  function applyScenarioPreset(scenarioID: WalletScenarioKind) {
    const preset = walletIssuanceScenarioPresetByID.get(scenarioID)
    if (!preset) {
      return
    }
    const activeTemplate = tpl.activeTemplateByScenario.get(scenarioID)
    tpl.setTemplateName(t(preset.templateNameKey))
    tpl.setTemplatePassType(preset.passType)
    tpl.setTemplateClassID(preset.classID)
    tpl.setTemplateStyleConfig(stringifyStyleConfig(preset.styleConfig))
    tpl.setTemplateStatus(tpl.defaultTemplateStatus)
    passes.setBatchExecutionMode(preset.recommendedExecutionMode)
    passes.setSingleTargetID("")
    passes.setBatchTargetIDs("")
    passes.setSelectedPassIDs([])
    passes.setPassQuery("")
    passes.setPassStatusFilter("all")
    passes.setPassTargetTypeFilter(preset.targetType)
    passes.setPassTemplateFilter(activeTemplate?.id ?? "all")
    if (activeTemplate) {
      passes.setSingleTemplateID(activeTemplate.id)
      passes.setBatchTemplateID(activeTemplate.id)
    } else {
      passes.setSingleTemplateID("")
      passes.setBatchTemplateID("")
    }
    if (typeof preset.defaultExpiresInHours === "number") {
      const expiresAt = buildRelativeDateTimeInput(preset.defaultExpiresInHours)
      passes.setSingleExpiresAt(expiresAt)
      passes.setBatchExpiresAt(expiresAt)
    } else {
      passes.setSingleExpiresAt("")
      passes.setBatchExpiresAt("")
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

  function keepMissingEnterpriseTargetsInBatchDraft() {
    if (enterpriseBatchTargetStats.missingIDs.length === 0) {
      setIssuanceSummary(t("walletPage.summaries.enterprise.allTargetsMatched"))
      return
    }
    passes.setBatchTargetIDs(enterpriseBatchTargetStats.missingIDs.join("\n"))
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
    passes.setBatchTargetIDs(issueReadyEnterpriseMissingTargetIDs.join("\n"))
    setError("")
    setIssuanceSummary(
      t("walletPage.summaries.enterprise.issueReadyTargetsSeeded", { count: issueReadyEnterpriseMissingTargetIDs.length })
    )
  }

  function restoreEnterpriseTargetsToBatchDraft() {
    if (enterpriseBatchTargetStats.targetIDs.length === 0) {
      return
    }
    passes.setBatchTargetIDs(enterpriseBatchTargetStats.targetIDs.join("\n"))
    setError("")
    setIssuanceSummary(
      t("walletPage.summaries.enterprise.allPrefilledTargetsRestored", { count: enterpriseBatchTargetStats.targetIDs.length })
    )
  }

  // --- Merged summaries/errors from hooks ---
  const effectiveIssuanceSummary = googleConfig.summary || issuanceSummary || tpl.issuanceSummary || passes.issuanceSummary
  const effectiveError = error || tenants.queryError || tpl.error || passes.error || alerts.error || googleConfig.error

  return (
    <div className="space-y-6">
      <WalletPageOverview writable={writable} readOnlyBoundaryHint={readOnlyBoundaryHint} />

      <WalletAlertConfigCard
        platformViewer={tenants.platformViewer}
        tenantID={tenants.tenantID}
        onTenantChange={onTenantChange}
        tenants={tenants.tenants}
        windowSeconds={alerts.windowSeconds}
        onWindowSecondsChange={alerts.setWindowSeconds}
        maxRetry={alerts.maxRetry}
        onMaxRetryChange={alerts.setMaxRetry}
        alertThreshold={alerts.alertThreshold}
        onAlertThresholdChange={alerts.setAlertThreshold}
        archiveLimit={alerts.archiveLimit}
        onArchiveLimitChange={alerts.setArchiveLimit}
        trendBucketCount={alerts.trendBucketCount}
        onTrendBucketCountChange={alerts.setTrendBucketCount}
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
            templateScenarioCounts={tpl.templateScenarioCounts}
            passScenarioCounts={passes.passScenarioCounts}
            activeTemplateNameByScenario={new Map(
              Array.from(tpl.activeTemplateByScenario.entries()).map(([scenarioID, template]) => [scenarioID, template.name])
            )}
            onApplyScenarioPreset={(scenarioID) => applyScenarioPreset(scenarioID as WalletScenarioKind)}
            activeEmployeeTemplateName={tpl.activeEmployeeTemplate?.name ?? ""}
            employeePassCount={passes.employeePassCount}
            showDirectorySyncLink={enterpriseWorkspaceAccessible}
            showAccessGrantLinks={accessWorkspaceAccessible}
            onUseEmployeeTemplate={() => {
              if (!tpl.activeEmployeeTemplate) {
                return
              }
              passes.setSingleTemplateID(tpl.activeEmployeeTemplate.id)
              passes.setBatchTemplateID(tpl.activeEmployeeTemplate.id)
              passes.setPassTargetTypeFilter("user")
              passes.setPassTemplateFilter(tpl.activeEmployeeTemplate.id)
              passes.setPassStatusFilter("all")
              passes.setPassQuery("")
            }}
            activeVisitorTemplateName={tpl.activeVisitorTemplate?.name ?? ""}
            visitorPassCount={passes.visitorPassCount}
            onUseVisitorTemplate={() => {
              if (!tpl.activeVisitorTemplate) {
                return
              }
              passes.setSingleTemplateID(tpl.activeVisitorTemplate.id)
              passes.setBatchTemplateID(tpl.activeVisitorTemplate.id)
              passes.setPassTargetTypeFilter("visitor")
              passes.setPassTemplateFilter(tpl.activeVisitorTemplate.id)
              passes.setPassStatusFilter("all")
              passes.setPassQuery("")
            }}
            suspendedPassCount={passes.suspendedPassCount}
            revocablePassCount={passes.revocablePassCount}
            onViewSuspended={() => {
              passes.setPassStatusFilter("suspended")
              passes.setPassTargetTypeFilter("all")
              passes.setPassTemplateFilter("all")
              passes.setPassQuery("")
            }}
            onViewAllStatuses={() => {
              passes.setPassStatusFilter("all")
              passes.setPassTargetTypeFilter("all")
              passes.setPassTemplateFilter("all")
              passes.setPassQuery("")
            }}
          />

          <div className="grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(0,0.95fr)]">
            <WalletTemplateManagerCard
              writable={writable}
              readOnlyBoundaryHint={readOnlyBoundaryHint}
              creatingTemplate={tpl.creatingTemplate}
              loading={loading}
              refreshing={refreshing}
              templateName={tpl.templateName}
              onTemplateNameChange={tpl.setTemplateName}
              templateClassID={tpl.templateClassID}
              onTemplateClassIDChange={tpl.setTemplateClassID}
              templatePassType={tpl.templatePassType}
              onTemplatePassTypeChange={tpl.setTemplatePassType}
              templateStatus={tpl.templateStatus}
              onTemplateStatusChange={tpl.setTemplateStatus}
              templateStyleConfig={tpl.templateStyleConfig}
              onTemplateStyleConfigChange={tpl.setTemplateStyleConfig}
              onSubmitTemplate={tpl.submitTemplate}
              issuanceSummary={effectiveIssuanceSummary}
              templates={tpl.templates}
              onSetDefaultTemplate={(templateID) => {
                passes.setSingleTemplateID(templateID)
                passes.setBatchTemplateID(templateID)
                passes.setPassTemplateFilter(templateID)
              }}
              onToggleTemplateStatus={(template) => {
                void tpl.toggleTemplateStatus(template)
              }}
              updatingTemplateID={tpl.updatingTemplateID}
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
              enterpriseSyncIssueHint={tenants.enterpriseSyncIssueHint}
              issueReadyEnterpriseMissingTargetIDs={issueReadyEnterpriseMissingTargetIDs}
              onKeepIssueReadyEnterpriseTargets={keepIssueReadyEnterpriseTargetsInBatchDraft}
              onKeepMissingEnterpriseTargets={keepMissingEnterpriseTargetsInBatchDraft}
              onRestoreEnterpriseTargets={restoreEnterpriseTargetsToBatchDraft}
              accessDirectoryReviewLink={accessDirectoryReviewLink}
              canOpenAccessReview={accessWorkspaceAccessible}
              enterpriseAlertsIssueLink={enterpriseAlertsIssueLink}
              canOpenEnterpriseReview={enterpriseWorkspaceAccessible}
              hasWorkerAlertFlowHints={tenants.hasWorkerAlertFlowHints}
              enterpriseSyncWorkerReviewLink={enterpriseSyncWorkerReviewLink}
              onSubmitSingleIssue={passes.submitSingleIssue}
              targetTypeLabel={(type) => targetTypeLabel(t, type)}
              singleTargetType={passes.singleTargetType}
              singleTemplateID={passes.singleTemplateID}
              onSingleTemplateIDChange={passes.setSingleTemplateID}
              templates={tpl.templates}
              singleTargetID={passes.singleTargetID}
              onSingleTargetIDChange={passes.setSingleTargetID}
              singleExpiresAt={passes.singleExpiresAt}
              onSingleExpiresAtChange={passes.setSingleExpiresAt}
              selectedSingleTemplate={passes.selectedSingleTemplate}
              getTemplateScenarioLabel={(template) => walletScenarioLabel(t, inferTemplateScenario(template))}
              getTemplateScenarioHint={(template) => walletScenarioHint(t, inferTemplateScenario(template))}
              issuingSingle={passes.issuingSingle}
              onSubmitBatchIssue={passes.submitBatchIssue}
              batchTargetType={passes.batchTargetType}
              batchTemplateID={passes.batchTemplateID}
              onBatchTemplateIDChange={passes.setBatchTemplateID}
              batchExpiresAt={passes.batchExpiresAt}
              onBatchExpiresAtChange={passes.setBatchExpiresAt}
              batchExecutionMode={passes.batchExecutionMode}
              onBatchExecutionModeChange={passes.setBatchExecutionMode}
              batchTargetIDs={passes.batchTargetIDs}
              onBatchTargetIDsChange={passes.setBatchTargetIDs}
              selectedBatchTemplate={passes.selectedBatchTemplate}
              issuingBatch={passes.issuingBatch}
              lastIssuedJobs={passes.lastIssuedJobs}
              formatDateTime={formatDateTime}
            />
          </div>

          <WalletDeliveryWorkspacePanels
            scenarioPresets={walletIssuanceScenarioPresets}
            activeTemplateByScenario={tpl.activeTemplateByScenario}
            passScenarioCounts={passes.passScenarioCounts}
            saveLinkScenarioCounts={passes.saveLinkScenarioCounts}
            deliveryDeskPasses={passes.deliveryDeskPasses}
            templateByID={tpl.templateByID}
            resolvingSaveLinkPassID={passes.resolvingSaveLinkPassID}
            onFocusPassScenario={(scenarioID) => focusPassScenario(scenarioID as WalletScenarioKind)}
            onOpenPassQrDialog={passes.openPassQrDialog}
            onCopySaveLink={passes.copySaveLink}
            onRefreshPassSaveLink={passes.refreshPassSaveLink}
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
              dispatchingDelivery={delivery.dispatchingDelivery}
              resolvingSaveLinkPassID={passes.resolvingSaveLinkPassID}
              readOnlyBoundaryHint={readOnlyBoundaryHint}
              deliveryPassID={delivery.deliveryPassID}
              deliveryEmailEnabled={delivery.deliveryEmailEnabled}
              deliveryWhatsAppEnabled={delivery.deliveryWhatsAppEnabled}
              deliverablePasses={passes.deliverablePasses}
              selectedDeliveryPass={delivery.selectedDeliveryPass}
              selectedDeliveryTemplate={delivery.selectedDeliveryTemplate}
              templateByID={tpl.templateByID}
              passDeliveryForm={delivery.passDeliveryForm}
              deliveryEmailRecipientsField={delivery.deliveryEmailRecipientsField}
              deliveryWhatsAppRecipientsField={delivery.deliveryWhatsAppRecipientsField}
              passDeliveryFormError={delivery.passDeliveryFormError}
              onDeliveryPassIDChange={delivery.setDeliveryPassID}
              onDeliveryEmailEnabledChange={delivery.setDeliveryEmailEnabled}
              onDeliveryWhatsAppEnabledChange={delivery.setDeliveryWhatsAppEnabled}
              onDeliveryEmailRecipientsChange={delivery.setDeliveryEmailRecipients}
              onDeliveryWhatsAppRecipientsChange={delivery.setDeliveryWhatsAppRecipients}
              onSubmit={delivery.onSubmitPassDeliveryForm}
              onOpenPassQrDialog={passes.openPassQrDialog}
              onCopySaveLink={passes.copySaveLink}
              onRefreshPassSaveLink={passes.refreshPassSaveLink}
              passStatusVariant={passStatusVariant}
              passStatusLabel={(status) => passStatusLabel(t, status)}
              walletScenarioLabel={(pass, template) => walletScenarioLabel(t, inferPassScenario(pass, template))}
            />

            <WalletDeliveryReceiptsCard
              writable={writable}
              loading={loading}
              refreshing={refreshing}
              batchRetryingDelivery={delivery.batchRetryingDelivery}
              repairingRetryablePasses={delivery.repairingRetryablePasses}
              retryingDeliveryNotificationID={delivery.retryingDeliveryNotificationID}
              receiptRecoveryFlowStatus={delivery.receiptRecoveryFlowStatus}
              receiptSplitStatus={delivery.receiptSplitStatus}
              receiptRemediationStatus={delivery.receiptRemediationStatus}
              receiptReviewStatus={delivery.receiptReviewStatus}
              failedDeliveryNotificationsCount={delivery.failedDeliveryNotifications.length}
              retryableDeliveryNotificationsCount={delivery.retryableDeliveryNotifications.length}
              nonRetryableFailedDeliveryNotificationsCount={delivery.nonRetryableFailedDeliveryNotifications.length}
              batchRetryableDeliveryNotificationsCount={delivery.batchRetryableDeliveryNotifications.length}
              repairableRetryableDeliveryPassesCount={delivery.repairableRetryableDeliveryPasses.length}
              reissueTargetIDsByRetryableDeliveryCount={delivery.reissueTargetIDsByRetryableDelivery.length}
              recentDeliveryNotifications={delivery.recentDeliveryNotifications}
              deliveryRetryQuery={delivery.deliveryRetryQuery}
              enterpriseReceiptRecoveryReviewLink={enterpriseReceiptRecoveryReviewLink}
              enterpriseAlertsIssueLink={enterpriseAlertsIssueLink}
              hasWorkerAlertFlowHints={tenants.hasWorkerAlertFlowHints}
              enterpriseSyncWorkerReviewLink={enterpriseSyncWorkerReviewLink}
              passByID={passes.passByID}
              templateByID={tpl.templateByID}
              receiptRecoveryStatusVariant={receiptRecoveryStatusVariant}
              receiptRecoveryStatusLabel={(status) => receiptRecoveryStatusLabel(t, status)}
              deliveryNotificationStatusVariant={deliveryNotificationStatusVariant}
              deliveryNotificationStatusLabel={(status) => deliveryNotificationStatusLabel(t, status)}
              formatDateTime={formatDateTime}
              onRetryBatch={() => {
                void delivery.retryDeliveryNotificationBatch()
              }}
              onRepairBatch={() => {
                void delivery.repairRetryableDeliveryPassStatusBatch()
              }}
              onSeedBatchReissue={delivery.seedBatchReissueFromRetryableDelivery}
              onOpenPassQrDialog={passes.openPassQrDialog}
              onCopySaveLink={passes.copySaveLink}
              onRetryDeliveryNotification={(notificationID) => {
                void delivery.retryDeliveryNotification(notificationID)
              }}
            />
          </div>

          <WalletIssuedPassesCard
            templates={tpl.templates}
            passTable={passes.passTable}
            passQuery={passes.passQuery}
            passStatusFilter={passes.passStatusFilter}
            passTargetTypeFilter={passes.passTargetTypeFilter}
            passTemplateFilter={passes.passTemplateFilter}
            filteredPassCount={passes.filteredPasses.length}
            selectedFilteredPassCount={passes.selectedFilteredPassCount}
            passesCount={passes.passes.length}
            hasPassFilters={passes.hasPassFilters}
            writable={writable}
            loading={loading}
            batchUpdatingPassAction={passes.batchUpdatingPassAction}
            passPage={passes.passPage}
            passPageSize={passes.passPageSize}
            onPassQueryChange={passes.setPassQuery}
            onPassStatusFilterChange={passes.setPassStatusFilter}
            onPassTargetTypeFilterChange={passes.setPassTargetTypeFilter}
            onPassTemplateFilterChange={passes.setPassTemplateFilter}
            onPassPageChange={passes.setPassPage}
            onPassPageSizeChange={passes.setPassPageSize}
            onClearFilters={() => {
              passes.setPassQuery("")
              passes.setPassStatusFilter("all")
              passes.setPassTargetTypeFilter("all")
              passes.setPassTemplateFilter("all")
              passes.setPassPage(1)
            }}
            onClearSelection={() => passes.setSelectedPassIDs([])}
            onUpdateSelectedPasses={(action) => {
              void passes.updateSelectedPasses(action)
            }}
          />
      </div>

        <WalletAdvancedWorkspace
          googleConfigCardProps={{
            writable,
            readOnlyBoundaryHint,
            tenantID: tenants.tenantID,
            config: googleConfig.config,
            validation: googleConfig.validation,
            loading,
            refreshing,
            loadingConfig: googleConfig.loadingConfig,
            savingConfig: googleConfig.savingConfig,
            validatingConfig: googleConfig.validatingConfig,
            error: googleConfig.error,
            formatDateTime,
            onSaveConfig: (payload) => {
              void googleConfig.saveGoogleConfig(payload)
            },
            onValidateConfig: (payload) => {
              void googleConfig.validateGoogleConfig(payload)
            },
          }}
          physicalCardTasksSectionProps={{
            writable,
            loading,
            refreshing,
            creatingPhysicalCardTask: physicalCards.creatingPhysicalCardTask,
            updatingPhysicalCardTaskID: physicalCards.updatingPhysicalCardTaskID,
            readOnlyBoundaryHint,
            physicalTaskPassID: physicalCards.physicalTaskPassID,
            physicalTaskType: physicalCards.physicalTaskType,
            employeeCardEligiblePasses: passes.employeeCardEligiblePasses,
            selectedPhysicalTaskPass: physicalCards.selectedPhysicalTaskPass,
            selectedPhysicalTaskTemplate: physicalCards.selectedPhysicalTaskTemplate,
            recentPhysicalCardTasks: physicalCards.recentPhysicalCardTasks,
            passByID: passes.passByID,
            templateByID: tpl.templateByID,
            physicalCardTaskForm: physicalCards.physicalCardTaskForm,
            physicalTaskCardNumberField: physicalCards.physicalTaskCardNumberField,
            physicalTaskNoteField: physicalCards.physicalTaskNoteField,
            physicalCardTaskFormError: physicalCards.physicalCardTaskFormError,
            onPhysicalTaskPassIDChange: physicalCards.setPhysicalTaskPassID,
            onPhysicalTaskTypeChange: physicalCards.setPhysicalTaskType,
            onPhysicalTaskCardNumberChange: physicalCards.setPhysicalTaskCardNumber,
            onPhysicalTaskNoteChange: physicalCards.setPhysicalTaskNote,
            onSubmit: physicalCards.onSubmitPhysicalCardTaskForm,
            onFocusEmployeePhysicalScenario: () => focusPassScenario("employee_physical"),
            onOpenPassQrDialog: passes.openPassQrDialog,
            onAdvancePhysicalCardTask: physicalCards.advancePhysicalCardTask,
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
            subscriptionEnabled: alerts.subscriptionEnabled,
            onSubscriptionEnabledChange: alerts.setSubscriptionEnabled,
            subscriptionEmailEnabled: alerts.subscriptionEmailEnabled,
            onSubscriptionEmailEnabledChange: alerts.setSubscriptionEmailEnabled,
            subscriptionWhatsAppEnabled: alerts.subscriptionWhatsAppEnabled,
            onSubscriptionWhatsAppEnabledChange: alerts.setSubscriptionWhatsAppEnabled,
            subscriptionThreshold: alerts.subscriptionThreshold,
            onSubscriptionThresholdChange: alerts.setSubscriptionThreshold,
            subscriptionWindowSeconds: alerts.subscriptionWindowSeconds,
            onSubscriptionWindowSecondsChange: alerts.setSubscriptionWindowSeconds,
            subscriptionCooldownSeconds: alerts.subscriptionCooldownSeconds,
            onSubscriptionCooldownSecondsChange: alerts.setSubscriptionCooldownSeconds,
            subscriptionReceiverGroups: alerts.subscriptionReceiverGroups,
            onSubscriptionReceiverGroupsChange: alerts.setSubscriptionReceiverGroups,
            subscription: alerts.subscription,
            formatDateTime,
            dispatchSummary: alerts.dispatchSummary,
            loading,
            refreshing,
            dispatchingAlerts: alerts.dispatchingAlerts,
            savingSubscription: alerts.savingSubscription,
            onDispatchAlertsNow: () => {
              void alerts.dispatchAlertsNow()
            },
            onSaveAlertSubscription: () => {
              void alerts.saveAlertSubscription()
            },
          }}
          riskOverviewPanelsProps={{
            loading,
            platformViewer: tenants.platformViewer,
            effectiveError,
            aggregateWarning: alerts.aggregateWarning,
            metrics: alerts.metrics,
            alertItems: alerts.alertItems,
            formatDurationSeconds,
            aggregateStats: alerts.aggregateStats,
            tenantAggregates: alerts.tenantAggregates,
            formatDateTime,
          }}
          alertTrendPanelsProps={{
            loading,
            metrics: alerts.metrics,
            metricsTrend: alerts.metricsTrend,
            trendPeakUpdated: alerts.trendPeakUpdated,
            formatDurationSeconds,
            formatTimeLabel,
            alertItems: alerts.alertItems,
            windowErrorCodeRows: alerts.windowErrorCodeRows,
            formatDateTime,
          }}
          alertNotificationRecordsCardProps={{
            loading,
            refreshing,
            writable,
            alertNotifications: alerts.alertNotifications,
            retryingAlertNotificationID: alerts.retryingAlertNotificationID,
            onRetryAlertNotification: (notificationID) => {
              void alerts.retryAlertNotification(notificationID)
            },
            formatDateTime,
          }}
          dlqGovernanceCardProps={{
            writable,
            readOnlyBoundaryHint,
            loading,
            refreshing,
            jobSummary: alerts.jobSummary,
            processLimit: alerts.processLimit,
            onProcessLimitChange: alerts.setProcessLimit,
            processWorkerCount: alerts.processWorkerCount,
            onProcessWorkerCountChange: alerts.setProcessWorkerCount,
            processMaxRetry: alerts.processMaxRetry,
            onProcessMaxRetryChange: alerts.setProcessMaxRetry,
            dlqLimit: alerts.dlqLimit,
            onDLQLimitChange: alerts.setDLQLimit,
            dlqErrorCode: alerts.dlqErrorCode,
            onDLQErrorCodeChange: alerts.setDLQErrorCode,
            dlqTargetIDOverride: alerts.dlqTargetIDOverride,
            onDLQTargetIDOverrideChange: alerts.setDLQTargetIDOverride,
            dlqOlderThanSeconds: alerts.dlqOlderThanSeconds,
            onDLQOlderThanSecondsChange: alerts.setDLQOlderThanSeconds,
            processingJobs: alerts.processingJobs,
            requeueingDLQ: alerts.requeueingDLQ,
            cleaningDLQ: alerts.cleaningDLQ,
            governanceSummary: alerts.governanceSummary,
            onProcessPendingJobs: () => {
              void alerts.processPendingJobs()
            },
            onRequeueDLQBatch: () => {
              void alerts.requeueDLQBatch()
            },
            onCleanupDLQBatch: () => {
              void alerts.cleanupDLQBatch()
            },
            formatDateTime,
          }}
          dlqCleanupArchivesCardProps={{
            loading,
            archives: alerts.archives,
            formatDateTime,
            formatDurationSeconds,
          }}
        />

      <WalletPassQrDialog
        open={passes.qrDialogOpen}
        pass={passes.qrDialogPass}
        template={passes.qrDialogTemplate}
        loading={passes.qrDialogLoading}
        previewURL={passes.qrDialogPreviewURL}
        saveLink={passes.qrDialogSaveLink}
        resolvingSaveLinkPassID={passes.resolvingSaveLinkPassID}
        onOpenChange={(open) => {
          passes.setQrDialogOpen(open)
          if (!open) {
            passes.setQrDialogPass(null)
            passes.setQrDialogSaveLink("")
            passes.setQrDialogSVG("")
            passes.setQrDialogLoading(false)
          }
        }}
        onDownloadSvg={passes.downloadQrSVG}
        onCopyLink={() => {
          if (passes.qrDialogPass) {
            void passes.copySaveLink({ ...passes.qrDialogPass, save_link: passes.qrDialogSaveLink })
          }
        }}
        onRefreshLink={() => {
          if (passes.qrDialogPass) {
            void passes.refreshPassSaveLink(passes.qrDialogPass)
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
