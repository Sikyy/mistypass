import { useMemo } from "react"
import { useLocation, useNavigate } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

import { EnterpriseAlertsWorkspace } from "@/components/enterprise/enterprise-alerts-workspace"
import { EnterpriseEmployeesWorkspace } from "@/components/enterprise/enterprise-employees-workspace"
import { EnterprisePageHeader } from "@/components/enterprise/enterprise-page-header"
import { EnterpriseIDPWorkspace } from "@/components/enterprise/enterprise-idp-workspace"
import { EnterpriseSCIMWorkspace } from "@/components/enterprise/enterprise-scim-workspace"
import { EnterprisePageOverview } from "@/components/enterprise/enterprise-page-overview"
import { EnterpriseSyncWorkspace } from "@/components/enterprise/enterprise-sync-workspace"
import { EnterpriseWorkflowOverview } from "@/components/enterprise/enterprise-workflow-overview"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  API_BASE_URL,
  getEnterpriseHRISWebhookExecution,
  getEnterpriseIDPConfig,
  getEnterpriseSyncWorkerAlertSubscription,
  listAccessPolicies,
  listEnterpriseEmployees,
  listEnterpriseHRISConnectors,
  listEnterpriseHRISPullStates,
  listEnterpriseHRISSecrets,
  listEnterpriseJITProvisionApprovals,
  listEnterpriseSyncRequests,
  listEnterpriseSyncJobs,
  listEnterpriseSyncWorkerAlerts,
  listEnterpriseSyncWorkerAlertSummary,
  listUserGroups,
  listWalletPasses,
  type CurrentUser,
  type AccessPolicy,
  type EnterpriseEmployee,
  type EnterpriseHRISConnector,
  type EnterpriseHRISPullState,
  type EnterpriseHRISSecret,
  type EnterpriseIDPConfig,
  type EnterpriseJITProvisionApproval,
  type EnterpriseSyncRequestRecord,
  type EnterpriseSyncJob,
  type EnterpriseSyncWorkerAlertSubscription,
  type EnterpriseSyncWorkerAlertItem,
  type EnterpriseSyncWorkerAlertSummaryItem,
  type UserGroup,
  type WalletPassInstance,
} from "@/lib/api"
import { canManageEnterprise, isPlatformViewer } from "@/lib/viewer"

import { useEnterpriseTenants } from "../hooks/use-enterprise-tenants"
import { useEnterpriseEmployees } from "../hooks/use-enterprise-employees"
import { useEnterpriseSync } from "../hooks/use-enterprise-sync"
import { useEnterpriseAlerts } from "../hooks/use-enterprise-alerts"
import { useEnterpriseWorkflow, statusBadgeVariant } from "../hooks/use-enterprise-workflow"

// ── Types ───────────────────────────────────────────────────────────────────

type EnterprisePageProps = {
  token: string
  viewer: CurrentUser
}

type EnterpriseSection = "employees" | "sync" | "idp" | "scim" | "alerts"
type AlertLandingView = "overview" | "approval_backlog" | "directory_exceptions"
type AlertSegmentHint = "receipt_recovery"
type AlertSegmentStatusHint = "pending" | "attention" | "ready"
type SyncFocusHint = "worker_alert"
type WorkerReviewStatusHint = "handled"
type WorkerReviewStageHint = "alerts" | "directory" | "policies" | "issuance" | "sync"
type HRISWebhookExecutionKindFilter = "all" | "receipt_process" | "dlq_replay"
type HRISWebhookExecutionStatusFilter = "all" | "queued" | "running" | "succeeded" | "failed"
type HRISWebhookExecutionReplayScopeFilter = "all" | "replayed" | "worker_required"
type HRISWebhookExecutionModeFilter = "all" | "inline" | "queued"
type HRISWebhookExecutionQueueStateFilter = "all" | "ready" | "cooldown" | "in_flight" | "attempt_limit" | "terminal"

// ── Helper functions (non-hook, kept in page) ───────────────────────────────

function getEnterpriseSectionsForViewer(viewer: CurrentUser): EnterpriseSection[] {
  if (isPlatformViewer(viewer)) {
    return ["sync", "alerts", "employees", "idp", "scim"]
  }
  return ["employees", "idp", "scim", "sync", "alerts"]
}

function resolveEnterpriseSection(value: string | undefined, fallback: EnterpriseSection): EnterpriseSection {
  switch (value) {
    case "employees":
    case "sync":
    case "idp":
    case "scim":
    case "alerts":
      return value
    default:
      return fallback
  }
}

function enterpriseSectionLabel(section: EnterpriseSection, t: TFunction) {
  switch (section) {
    case "employees":
      return t("enterprisePage.labels.employees")
    case "sync":
      return t("enterprisePage.labels.sync")
    case "idp":
      return t("enterprisePage.labels.idp")
    case "scim":
      return "SCIM"
    case "alerts":
      return t("enterprisePage.labels.alerts")
  }
}

function defaultEnterpriseSyncWorkerAlertSubscription(tenantID: string): EnterpriseSyncWorkerAlertSubscription {
  return {
    tenant_id: tenantID.trim(),
    enabled: true,
    worker_alert_threshold: 3,
    window_seconds: 900,
    cooldown_seconds: 900,
    channels: {
      email: true,
      whatsapp: false,
    },
    receiver_groups: ["security"],
    updated_at: new Date().toISOString(),
  }
}

// ── Enterprise data loader ──────────────────────────────────────────────────

type EnterprisePageData = {
  employees: EnterpriseEmployee[]
  hrisConnectors: EnterpriseHRISConnector[]
  hrisSecrets: EnterpriseHRISSecret[]
  syncJobs: EnterpriseSyncJob[]
  syncRequests: EnterpriseSyncRequestRecord[]
  workerAlertSubscription: EnterpriseSyncWorkerAlertSubscription
  approvals: EnterpriseJITProvisionApproval[]
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[]
  workerAlertEvents: EnterpriseSyncWorkerAlertItem[]
  hrisPullStates: EnterpriseHRISPullState[]
  idpConfig: EnterpriseIDPConfig | null
  userGroups: UserGroup[]
  policies: AccessPolicy[]
  issuedPasses: WalletPassInstance[]
}

async function loadEnterprisePageData(token: string, tenantID: string): Promise<EnterprisePageData> {
  const [
    employeeItems,
    connectorItems,
    secretItems,
    syncJobItems,
    syncRequestItems,
    workerAlertSubscription,
    approvalItems,
    workerAlertItems,
    workerAlertEventItems,
    hrisPullStateItems,
    groupItems,
    policyItems,
    passItems,
  ] = await Promise.all([
    listEnterpriseEmployees(token, tenantID),
    listEnterpriseHRISConnectors(token, tenantID),
    listEnterpriseHRISSecrets(token, tenantID),
    listEnterpriseSyncJobs(token, tenantID),
    listEnterpriseSyncRequests(token, {
      tenant_id: tenantID,
      limit: 50,
    }),
    getEnterpriseSyncWorkerAlertSubscription(token, {
      tenant_id: tenantID,
    }).catch(() => defaultEnterpriseSyncWorkerAlertSubscription(tenantID)),
    listEnterpriseJITProvisionApprovals(token, {
      tenant_id: tenantID,
      limit: 12,
    }),
    listEnterpriseSyncWorkerAlertSummary(token, {
      tenant_id: tenantID,
      limit: 12,
    }),
    listEnterpriseSyncWorkerAlerts(token, {
      tenant_id: tenantID,
      limit: 12,
    }),
    listEnterpriseHRISPullStates(token, tenantID),
    listUserGroups(token),
    listAccessPolicies(token),
    listWalletPasses(token, tenantID),
  ])

  let nextIDPConfig: EnterpriseIDPConfig | null = null
  try {
    nextIDPConfig = await getEnterpriseIDPConfig(token, tenantID)
  } catch (err) {
    const message = err instanceof Error ? err.message : ""
    if (!message.includes("404") && !message.toLowerCase().includes("not found")) {
      throw err
    }
  }

  return {
    employees: employeeItems,
    hrisConnectors: connectorItems,
    hrisSecrets: secretItems,
    syncJobs: syncJobItems,
    syncRequests: syncRequestItems,
    workerAlertSubscription,
    approvals: approvalItems,
    workerAlerts: workerAlertItems,
    workerAlertEvents: workerAlertEventItems,
    hrisPullStates: hrisPullStateItems,
    idpConfig: nextIDPConfig,
    userGroups: groupItems,
    policies: policyItems,
    issuedPasses: passItems,
  }
}

// ── Component ───────────────────────────────────────────────────────────────

export function EnterprisePage({ token, viewer }: EnterprisePageProps) {
  const { t } = useTranslation()
  const location = useLocation()
  const navigate = useNavigate()
  const writable = canManageEnterprise(viewer)
  const visibleSections = useMemo(() => getEnterpriseSectionsForViewer(viewer), [viewer])
  const defaultSection = visibleSections[0] ?? "employees"
  const requestedSection = resolveEnterpriseSection(location.hash.replace(/^#/, ""), defaultSection)
  const activeSection = visibleSections.includes(requestedSection) ? requestedSection : defaultSection

  // ── Route hints (kept in page — tightly coupled to router) ────────────

  const enterpriseRouteHints = useMemo(() => {
    const query = new URLSearchParams(location.search)
    return {
      executionID: query.get("execution_id")?.trim() || "",
      tenantID: query.get("tenant_id")?.trim() || "",
    }
  }, [location.search])

  const alertsRouteHints = useMemo(() => {
    const query = new URLSearchParams(location.search)
    const rawLandingView = query.get("alerts_view_hint")?.trim() || ""
    const landingView: AlertLandingView | undefined =
      rawLandingView === "overview" || rawLandingView === "approval_backlog" || rawLandingView === "directory_exceptions"
        ? (rawLandingView as AlertLandingView)
        : undefined
    const rawSegmentHint = query.get("segment_hint")?.trim() || ""
    const segmentHint: AlertSegmentHint | undefined = rawSegmentHint === "receipt_recovery" ? "receipt_recovery" : undefined
    const rawSegmentStatus = query.get("segment_status_hint")?.trim() || ""
    const segmentStatus: AlertSegmentStatusHint | undefined = ["pending", "attention", "ready"].includes(rawSegmentStatus)
      ? (rawSegmentStatus as AlertSegmentStatusHint)
      : undefined
    const rawSyncStatus = query.get("sync_status_hint")?.trim() || ""
    const syncStatus:
      | "all"
      | "attention"
      | "rejected"
      | "deactivated"
      | "healthy"
      | undefined = ["all", "attention", "rejected", "deactivated", "healthy"].includes(rawSyncStatus)
      ? (rawSyncStatus as "all" | "attention" | "rejected" | "deactivated" | "healthy")
      : undefined
    const syncSource = query.get("sync_source_hint")?.trim() || ""
    const rawWorkerFilter = query.get("worker_filter_hint")?.trim() || ""
    const workerFilter: "all" | "alerting" | "hot" | "stable" | undefined = ["all", "alerting", "hot", "stable"].includes(
      rawWorkerFilter
    )
      ? (rawWorkerFilter as "all" | "alerting" | "hot" | "stable")
      : undefined
    const workerAction = query.get("worker_action")?.trim() || ""
    const workerLabel = query.get("worker_alert_label")?.trim() || ""
    const workerKind = query.get("worker_kind")?.trim() || ""
    const workerQueueState = query.get("worker_queue_state")?.trim() || ""
    const workerReplayState = query.get("worker_replay_state")?.trim() || ""
    const workerStatus = query.get("worker_status")?.trim() || ""
    const rawExecutionKind = query.get("execution_kind")?.trim() || ""
    const executionKind: HRISWebhookExecutionKindFilter | undefined = ["receipt_process", "dlq_replay"].includes(
      rawExecutionKind
    )
      ? (rawExecutionKind as HRISWebhookExecutionKindFilter)
      : undefined
    const rawExecutionStatus = query.get("execution_status")?.trim() || ""
    const executionStatus: HRISWebhookExecutionStatusFilter | undefined = ["queued", "running", "succeeded", "failed"].includes(
      rawExecutionStatus
    )
      ? (rawExecutionStatus as HRISWebhookExecutionStatusFilter)
      : undefined
    const rawExecutionQueueState = query.get("execution_queue_state")?.trim() || ""
    const executionQueueState: HRISWebhookExecutionQueueStateFilter | undefined = [
      "ready",
      "cooldown",
      "in_flight",
      "attempt_limit",
      "terminal",
    ].includes(rawExecutionQueueState)
      ? (rawExecutionQueueState as HRISWebhookExecutionQueueStateFilter)
      : undefined
    const rawExecutionReplayScope = query.get("execution_replay_scope")?.trim() || ""
    const executionReplayScope: HRISWebhookExecutionReplayScopeFilter | undefined = [
      "replayed",
      "worker_required",
    ].includes(rawExecutionReplayScope)
      ? (rawExecutionReplayScope as HRISWebhookExecutionReplayScopeFilter)
      : undefined
    const rawExecutionMode = query.get("execution_mode")?.trim() || ""
    const executionMode: HRISWebhookExecutionModeFilter | undefined = ["inline", "queued"].includes(rawExecutionMode)
      ? (rawExecutionMode as HRISWebhookExecutionModeFilter)
      : undefined
    const approvalQuery =
      query.get("approval_query_hint")?.trim() ||
      query.get("target_hint")?.trim() ||
      query.get("target_email")?.trim() ||
      query.get("target_id")?.trim() ||
      ""
    const directoryQuery = query.get("worker_query_hint")?.trim() || query.get("sync_query_hint")?.trim() || ""
    const hasHints =
      activeSection === "alerts" &&
      Boolean(
        landingView ||
          segmentHint ||
          segmentStatus ||
          syncStatus ||
          syncSource ||
          workerFilter ||
          workerAction ||
          workerLabel ||
          workerKind ||
          workerQueueState ||
          workerReplayState ||
          workerStatus ||
          executionKind ||
          executionStatus ||
          executionQueueState ||
          executionReplayScope ||
          executionMode ||
          approvalQuery ||
          directoryQuery
      )
    return {
      hasHints,
      contextKey: hasHints ? `${location.search}::${activeSection}` : "",
      approvalQuery: approvalQuery || undefined,
      directoryQuery: directoryQuery || undefined,
      landingView,
      segmentHint,
      segmentStatus,
      workerAction: workerAction || undefined,
      workerFilter,
      workerKind: workerKind || undefined,
      workerLabel: workerLabel || undefined,
      workerQueueState: workerQueueState || undefined,
      workerReplayState: workerReplayState || undefined,
      workerStatus: workerStatus || undefined,
      executionKind,
      executionMode,
      executionQueueState,
      executionReplayScope,
      executionStatus,
      syncSource: syncSource || undefined,
      syncStatus,
    }
  }, [activeSection, location.search])

  const syncRouteHints = useMemo(() => {
    const query = new URLSearchParams(location.search)
    const rawFocusHint = query.get("sync_focus_hint")?.trim() || ""
    const focusHint: SyncFocusHint | undefined = rawFocusHint === "worker_alert" ? "worker_alert" : undefined
    const rawWorkerReviewStatus = query.get("worker_review_status_hint")?.trim() || ""
    const workerReviewStatus: WorkerReviewStatusHint | undefined =
      rawWorkerReviewStatus === "handled" ? "handled" : undefined
    const rawWorkerReviewStage = query.get("worker_review_stage_hint")?.trim() || ""
    const workerReviewStage: WorkerReviewStageHint | undefined = [
      "alerts",
      "directory",
      "policies",
      "issuance",
      "sync",
    ].includes(rawWorkerReviewStage)
      ? (rawWorkerReviewStage as WorkerReviewStageHint)
      : undefined
    const rawWorkerFilter = query.get("worker_filter_hint")?.trim() || ""
    const workerFilter: "all" | "alerting" | "hot" | "stable" | undefined = ["all", "alerting", "hot", "stable"].includes(
      rawWorkerFilter
    )
      ? (rawWorkerFilter as "all" | "alerting" | "hot" | "stable")
      : undefined
    const workerAction = query.get("worker_action")?.trim() || ""
    const workerLabel = query.get("worker_alert_label")?.trim() || ""
    const workerKind = query.get("worker_kind")?.trim() || ""
    const workerConnectorID = query.get("worker_connector_id")?.trim() || ""
    const workerFailureStage = query.get("worker_failure_stage")?.trim() || ""
    const workerMode = query.get("worker_mode")?.trim() || ""
    const workerQueueState = query.get("worker_queue_state")?.trim() || ""
    const workerQuery = query.get("worker_query_hint")?.trim() || ""
    const workerReplayState = query.get("worker_replay_state")?.trim() || ""
    const workerRequestID = query.get("worker_request_id")?.trim() || ""
    const workerStatus = query.get("worker_status")?.trim() || ""
    const workerVendor = query.get("worker_vendor")?.trim() || ""
    const executionID = query.get("execution_id")?.trim() || ""
    const rawExecutionKind = query.get("execution_kind")?.trim() || ""
    const executionKind: HRISWebhookExecutionKindFilter | undefined = ["receipt_process", "dlq_replay"].includes(
      rawExecutionKind
    )
      ? (rawExecutionKind as HRISWebhookExecutionKindFilter)
      : undefined
    const rawExecutionMode = query.get("execution_mode")?.trim() || ""
    const executionMode: HRISWebhookExecutionModeFilter | undefined = ["inline", "queued"].includes(rawExecutionMode)
      ? (rawExecutionMode as HRISWebhookExecutionModeFilter)
      : undefined
    const rawExecutionQueueState = query.get("execution_queue_state")?.trim() || ""
    const executionQueueState: HRISWebhookExecutionQueueStateFilter | undefined = [
      "ready",
      "cooldown",
      "in_flight",
      "attempt_limit",
      "terminal",
    ].includes(rawExecutionQueueState)
      ? (rawExecutionQueueState as HRISWebhookExecutionQueueStateFilter)
      : undefined
    const rawExecutionReplayScope = query.get("execution_replay_scope")?.trim() || ""
    const executionReplayScope: HRISWebhookExecutionReplayScopeFilter | undefined = [
      "replayed",
      "worker_required",
    ].includes(rawExecutionReplayScope)
      ? (rawExecutionReplayScope as HRISWebhookExecutionReplayScopeFilter)
      : undefined
    const rawExecutionStatus = query.get("execution_status")?.trim() || ""
    const executionStatus: HRISWebhookExecutionStatusFilter | undefined = ["queued", "running", "succeeded", "failed"].includes(
      rawExecutionStatus
    )
      ? (rawExecutionStatus as HRISWebhookExecutionStatusFilter)
      : undefined
    const hasHints =
      activeSection === "sync" &&
      Boolean(
        focusHint ||
          workerFilter ||
          workerQuery ||
          workerAction ||
          workerLabel ||
          workerKind ||
          workerConnectorID ||
          workerFailureStage ||
          workerMode ||
          workerQueueState ||
          workerRequestID ||
          workerReplayState ||
          workerReviewStatus ||
          workerReviewStage ||
          workerStatus ||
          workerVendor ||
          executionID ||
          executionKind ||
          executionMode ||
          executionQueueState ||
          executionReplayScope ||
          executionStatus
      )
    return {
      hasHints,
      contextKey: hasHints ? `${location.search}::${activeSection}` : "",
      executionID: executionID || undefined,
      executionKind: executionKind || undefined,
      executionMode: executionMode || undefined,
      executionQueueState: executionQueueState || undefined,
      executionReplayScope: executionReplayScope || undefined,
      executionStatus: executionStatus || undefined,
      focusHint,
      workerAction: workerAction || undefined,
      workerConnectorID: workerConnectorID || undefined,
      workerFailureStage: workerFailureStage || undefined,
      workerFilter,
      workerKind: workerKind || undefined,
      workerLabel: workerLabel || undefined,
      workerMode: workerMode || undefined,
      workerQueueState: workerQueueState || undefined,
      workerRequestID: workerRequestID || undefined,
      workerReplayState: workerReplayState || undefined,
      workerReviewStage,
      workerReviewStatus,
      workerQuery: workerQuery || undefined,
      workerStatus: workerStatus || undefined,
      workerVendor: workerVendor || undefined,
    }
  }, [activeSection, location.search])

  // ── Navigation ────────────────────────────────────────────────────────

  function goToSection(section: EnterpriseSection) {
    navigate({
      pathname: location.pathname,
      search: location.search,
      hash: `#${section}`,
    })
  }

  function onSelectHRISWebhookExecution(executionID: string | null) {
    const nextExecutionID = executionID?.trim() || ""
    const query = new URLSearchParams(location.search)
    if (nextExecutionID) {
      query.set("execution_id", nextExecutionID)
      if (selectedTenantID.trim()) {
        query.set("tenant_id", selectedTenantID.trim())
      }
      const currentAlertsViewHint = query.get("alerts_view_hint")?.trim() || ""
      if (currentAlertsViewHint !== "overview" && currentAlertsViewHint !== "directory_exceptions") {
        query.set("alerts_view_hint", "directory_exceptions")
      }
    } else {
      query.delete("execution_id")
    }
    const nextSearch = query.toString()
    navigate({
      pathname: location.pathname,
      search: nextSearch ? `?${nextSearch}` : "",
      hash: "#alerts",
    })
  }

  // ── Tenant hook ───────────────────────────────────────────────────────

  const tenantHook = useEnterpriseTenants({
    token,
    viewer,
    enterpriseRouteHintTenantID: enterpriseRouteHints.tenantID,
  })
  const {
    tenants,
    selectedTenantID,
    setSelectedTenantID,
    selectedTenant,
    platformViewer,
    tenantsLoading,
    tenantsError,
  } = tenantHook

  // ── Enterprise data query ─────────────────────────────────────────────

  const enterpriseDataQuery = useQuery({
    queryKey: ["enterprise-data", selectedTenantID.trim()],
    queryFn: () => loadEnterprisePageData(token, selectedTenantID.trim()),
    enabled: selectedTenantID.trim().length > 0,
    staleTime: 30 * 1000,
  })

  const loading =
    tenantsLoading ||
    (selectedTenantID.trim().length > 0 && enterpriseDataQuery.isPending)
  const queryError =
    tenantsError ||
    (enterpriseDataQuery.error instanceof Error && enterpriseDataQuery.error.message) ||
    ""

  async function reloadEnterpriseData(tenantID: string) {
    const effectiveTenantID = tenantID.trim()
    if (!effectiveTenantID) {
      return
    }
    if (effectiveTenantID !== selectedTenantID.trim()) {
      setSelectedTenantID(effectiveTenantID)
      return
    }
    await enterpriseDataQuery.refetch()
  }

  // ── Employee hook ─────────────────────────────────────────────────────

  const employeeHook = useEnterpriseEmployees({
    selectedTenantID,
    enterpriseData: enterpriseDataQuery.data,
  })
  const {
    employees,
    setEmployees,
    hrisConnectors,
    setHRISConnectors,
    hrisSecrets,
    hrisPullStates,
    idpConfig,
    tenantGroups,
    tenantPolicies,
    activeEmployeeCount,
    idpReady,
    issuedPassCount,
    primaryEmployeeHint,
    primaryEmployeeTargetHint,
    deactivatedEmployeeHint,
    primaryGroupHint,
  } = employeeHook

  // ── Sync hook ─────────────────────────────────────────────────────────

  const syncHook = useEnterpriseSync({
    token,
    viewer,
    selectedTenantID,
    selectedTenantName: selectedTenant?.name,
    enterpriseData: enterpriseDataQuery.data,
    enterpriseRouteHintExecutionID: enterpriseRouteHints.executionID,
    reloadEnterpriseData,
    setEmployees,
    setHRISConnectors,
  })

  // ── Alerts hook ───────────────────────────────────────────────────────

  const alertsHook = useEnterpriseAlerts({
    token,
    viewer,
    selectedTenantID,
    selectedTenantName: selectedTenant?.name,
    enterpriseData: enterpriseDataQuery.data,
    reloadEnterpriseData,
    setSyncSummary: syncHook.setSyncSummary,
    setError: syncHook.setError,
  })

  // ── Workflow hook ─────────────────────────────────────────────────────

  const workflowHook = useEnterpriseWorkflow({
    selectedTenantID,
    loading,
    activeEmployeeCount,
    idpReady,
    issuedPassCount,
    failedSyncJobCount: syncHook.failedSyncJobCount,
    pendingApprovalCount: alertsHook.pendingApprovalCount,
    workerAlertCount: alertsHook.workerAlertCount,
    tenantGroups,
    tenantPolicies,
    syncJobs: syncHook.syncJobs,
    sortedSyncJobs: syncHook.sortedSyncJobs,
    latestSyncJob: syncHook.latestSyncJob,
    syncSource: syncHook.syncSource,
    primaryEmployeeHint,
    primaryEmployeeTargetHint,
    deactivatedEmployeeHint,
    primaryGroupHint,
    goToSection,
    setSyncSource: syncHook.setSyncSource,
  })

  // ── Derived ───────────────────────────────────────────────────────────

  const effectiveError = syncHook.error || queryError

  // ── Render ────────────────────────────────────────────────────────────

  return (
    <div className="space-y-6">
      <EnterprisePageHeader
        platformViewer={platformViewer}
        selectedTenantID={selectedTenantID}
        selectedTenantName={selectedTenant?.name}
        tenants={tenants}
        onTenantChange={setSelectedTenantID}
      />

      <EnterprisePageOverview
        loading={loading}
        activeEmployeeCount={activeEmployeeCount}
        syncJobsCount={syncHook.syncJobs.length}
        idpStatusLabel={idpConfig?.status || t("enterprisePage.status.notConfigured")}
        pendingApprovalCount={alertsHook.pendingApprovalCount}
        effectiveError={effectiveError}
        syncSummary={syncHook.syncSummary}
        attentionItems={workflowHook.attentionItems}
        quickActions={workflowHook.quickActions}
      />

      <EnterpriseWorkflowOverview
        loading={loading}
        tenantGroupsCount={tenantGroups.length}
        tenantPoliciesCount={tenantPolicies.length}
        issuedPassCount={issuedPassCount}
        workflowSteps={workflowHook.workflowStepsOverview}
        nextWorkflowAction={workflowHook.nextWorkflowAction}
        directoryFlowLink={workflowHook.directoryFlowLink}
        policiesFlowLink={workflowHook.policiesFlowLink}
        onGoToSection={goToSection}
      />

      <Tabs value={activeSection} onValueChange={(value) => goToSection(value as EnterpriseSection)} className="space-y-4">
        <TabsList
          className="grid w-full max-w-3xl"
          style={{ gridTemplateColumns: `repeat(${visibleSections.length}, minmax(0, 1fr))` }}
        >
          {visibleSections.map((section) => (
            <TabsTrigger key={section} value={section}>
              {enterpriseSectionLabel(section, t)}
            </TabsTrigger>
          ))}
        </TabsList>

        <EnterpriseEmployeesWorkspace
          directoryLink={workflowHook.directoryFlowLink}
          employees={employees}
          formatDateTime={workflowHook.formatDateTimeValue}
          loading={loading}
          statusBadgeVariant={statusBadgeVariant}
        />

        <EnterpriseSyncWorkspace
          activeEmployeeCount={activeEmployeeCount}
          apiBaseURL={API_BASE_URL}
          alertsLink={workflowHook.enterpriseAlertsFlowLink}
          directoryLink={workflowHook.directoryFlowLink}
          failedSyncJobCount={syncHook.failedSyncJobCount}
          formatDateTime={workflowHook.formatDateTimeValue}
          goToSection={goToSection}
          hrisConnectors={hrisConnectors}
          hrisSecrets={hrisSecrets}
          initialFilterContextKey={syncRouteHints.hasHints ? syncRouteHints.contextKey : undefined}
          initialFocusHint={syncRouteHints.focusHint}
          initialWorkerFilter={syncRouteHints.workerFilter}
          initialWorkerAction={syncRouteHints.workerAction}
          initialWorkerConnectorID={syncRouteHints.workerConnectorID}
          initialWorkerFailureStage={syncRouteHints.workerFailureStage}
          initialWorkerKind={syncRouteHints.workerKind}
          initialWorkerLabel={syncRouteHints.workerLabel}
          initialWorkerMode={syncRouteHints.workerMode}
          initialWorkerQueueState={syncRouteHints.workerQueueState}
          initialWorkerRequestID={syncRouteHints.workerRequestID}
          initialWorkerReplayState={syncRouteHints.workerReplayState}
          initialWorkerReviewStage={syncRouteHints.workerReviewStage}
          initialWorkerReviewStatus={syncRouteHints.workerReviewStatus}
          initialWorkerQuery={syncRouteHints.workerQuery}
          initialWorkerStatus={syncRouteHints.workerStatus}
          initialWorkerVendor={syncRouteHints.workerVendor}
          initialExecutionID={syncRouteHints.executionID}
          initialExecutionKind={syncRouteHints.executionKind}
          initialExecutionMode={syncRouteHints.executionMode}
          initialExecutionQueueState={syncRouteHints.executionQueueState}
          initialExecutionReplayScope={syncRouteHints.executionReplayScope}
          initialExecutionStatus={syncRouteHints.executionStatus}
          latestSyncJob={syncHook.latestSyncJob}
          loading={loading}
          onSaveTalentaConnector={syncHook.onSaveTalentaConnector}
          onSyncEmployees={syncHook.onSyncEmployees}
          onSyncPayloadChange={syncHook.setSyncPayload}
          onSyncRequestIDChange={syncHook.setSyncRequestID}
          onSyncSourceChange={syncHook.setSyncSource}
          sampleSyncPayload={syncHook.sampleSyncPayload}
          sortedSyncJobs={syncHook.sortedSyncJobs}
          statusBadgeVariant={statusBadgeVariant}
          syncIssueCards={workflowHook.syncIssueCards}
          syncOutcomeAction={workflowHook.syncOutcomeAction}
          syncPayload={syncHook.syncPayload}
          syncRemediationSteps={workflowHook.syncRemediationSteps}
          syncRequestID={syncHook.syncRequestID}
          syncSource={syncHook.syncSource}
          syncSourceFeedback={workflowHook.syncSourceFeedback}
          syncSourceStatusCards={workflowHook.syncSourceStatusCards}
          syncSourceLabel={workflowHook.syncSourceLabelValue}
          syncSourceOptions={workflowHook.syncSourceOptions}
          syncing={syncHook.syncing}
          tenantGroupsCount={tenantGroups.length}
          tenantPoliciesCount={tenantPolicies.length}
          issuedPassCount={issuedPassCount}
          policiesLink={workflowHook.policiesFlowLink}
          selectedTenantName={selectedTenant?.name}
          syncLink={workflowHook.enterpriseSyncFlowLink}
          workflowStateLabel={workflowHook.workflowStateLabel}
          workflowStateVariant={workflowHook.workflowStateVariant}
          workerAlerts={alertsHook.workerAlerts}
          walletLink={workflowHook.walletFlowLink}
          writable={writable}
        />

        <EnterpriseIDPWorkspace
          activeEmployeeCount={activeEmployeeCount}
          directoryLink={workflowHook.directoryFlowLink}
          failedSyncJobCount={syncHook.failedSyncJobCount}
          formatDateTime={workflowHook.formatDateTimeValue}
          goToSection={goToSection}
          idpConfig={idpConfig}
          idpOutcomeAction={workflowHook.idpOutcomeAction}
          idpReady={idpReady}
          loading={loading}
          pendingApprovalCount={alertsHook.pendingApprovalCount}
          policiesLink={workflowHook.policiesFlowLink}
          syncJobsCount={syncHook.syncJobs.length}
          workerAlertCount={alertsHook.workerAlertCount}
        />

        <EnterpriseSCIMWorkspace token={token} />

        <EnterpriseAlertsWorkspace
          alertRecoveryAction={workflowHook.alertRecoveryAction}
          approvals={alertsHook.approvals}
          approvalActionID={alertsHook.approvalActionID}
          approvalActionBusy={alertsHook.approvalActionID !== null}
          attentionItems={workflowHook.attentionItems}
          directoryLink={workflowHook.directoryFlowLink}
          dlqActionBusy={syncHook.dlqActionID !== null}
          dlqActionID={syncHook.dlqActionID}
          formatDateTime={workflowHook.formatDateTimeValue}
          goToSection={goToSection}
          landingCards={workflowHook.alertLandingCards}
          loading={loading}
          onProcessHRISWebhookReceipt={syncHook.onProcessHRISWebhookReceipt}
          onBatchProcessHRISWebhookReceipts={syncHook.onBatchProcessHRISWebhookReceipts}
          onBatchReviewApprovals={alertsHook.onBatchReviewApprovals}
          onBatchUpdateApprovalExternalSync={alertsHook.onBatchUpdateApprovalExternalSync}
          onBatchReplayHRISWebhookDLQ={syncHook.onBatchReplayHRISWebhookDLQ}
          onReconcilePendingSyncRequests={syncHook.onReconcilePendingSyncRequests}
          onReplayHRISWebhookDLQ={syncHook.onReplayHRISWebhookDLQ}
          onReviewApproval={alertsHook.onReviewApproval}
          onUpdateApprovalExternalSync={alertsHook.onUpdateApprovalExternalSync}
          syncLink={workflowHook.enterpriseSyncFlowLink}
          initialFilterContextKey={alertsRouteHints.hasHints ? alertsRouteHints.contextKey : undefined}
          initialLandingView={alertsRouteHints.landingView}
          initialApprovalQuery={alertsRouteHints.approvalQuery}
          initialDirectoryQuery={alertsRouteHints.directoryQuery}
          initialSegmentHint={alertsRouteHints.segmentHint}
          initialSegmentStatus={alertsRouteHints.segmentStatus}
          initialWorkerAction={alertsRouteHints.workerAction}
          initialWorkerFilter={alertsRouteHints.workerFilter}
          initialWorkerKind={alertsRouteHints.workerKind}
          initialWorkerLabel={alertsRouteHints.workerLabel}
          initialWorkerQueueState={alertsRouteHints.workerQueueState}
          initialWorkerReplayState={alertsRouteHints.workerReplayState}
          initialExecutionKind={alertsRouteHints.executionKind}
          initialExecutionMode={alertsRouteHints.executionMode}
          initialExecutionQueueState={alertsRouteHints.executionQueueState}
          initialExecutionReplayScope={alertsRouteHints.executionReplayScope}
          initialExecutionStatus={alertsRouteHints.executionStatus}
          initialSyncSourceFilter={alertsRouteHints.syncSource}
          initialSyncStatusFilter={alertsRouteHints.syncStatus}
          initialWorkerStatus={alertsRouteHints.workerStatus}
          policiesLink={workflowHook.policiesFlowLink}
          latestWebhookDLQBatchReplayResult={syncHook.latestWebhookDLQBatchReplayResult}
          latestWebhookReceiptBatchProcessResult={syncHook.latestWebhookReceiptBatchProcessResult}
          receiptActionID={syncHook.receiptActionID}
          receiptActionBusy={syncHook.receiptActionID !== null}
          selectedTenantName={selectedTenant?.name}
          statusBadgeVariant={statusBadgeVariant}
          syncRequestActionBusy={syncHook.syncRequestActionID !== null}
          syncRequests={syncHook.syncRequests}
          syncJobs={syncHook.syncJobs}
          writable={writable}
          workerAlertSubscription={alertsHook.workerAlertSubscription}
          workerAlertDispatching={alertsHook.workerAlertDispatchActionID !== null}
          workerAlertSubscriptionSaving={alertsHook.workerAlertSubscriptionActionID !== null}
          walletLink={workflowHook.walletFlowLink}
          workerAlerts={alertsHook.workerAlerts}
          workerAlertEvents={alertsHook.workerAlertEvents}
          workerAlertNotifications={alertsHook.workerAlertNotifications}
          workerAlertNotificationTotal={alertsHook.workerAlertNotificationTotal}
          workerAlertNotificationFilterCounts={alertsHook.workerAlertNotificationFilterCounts}
          workerAlertNotificationStatusCounts={alertsHook.workerAlertNotificationStatusCounts}
          workerAlertNotificationLoading={alertsHook.workerAlertNotificationListLoading}
          workerAlertNotificationLoadingMore={alertsHook.workerAlertNotificationListLoadingMore}
          workerAlertNotificationHasMore={alertsHook.workerAlertNotificationHasMore}
          exportingWorkerAlertNotifications={alertsHook.workerAlertNotificationExporting}
          onWorkerAlertNotificationHistoryViewChange={alertsHook.onWorkerAlertNotificationHistoryViewChange}
          onLoadMoreWorkerAlertNotifications={alertsHook.onLoadMoreWorkerAlertNotifications}
          onExportWorkerAlertNotifications={alertsHook.onExportWorkerAlertNotifications}
          hrisWebhookExecutions={syncHook.hrisWebhookExecutions}
          hrisWebhookExecutionTotal={syncHook.hrisWebhookExecutionTotal}
          hrisWebhookExecutionStatusCounts={syncHook.hrisWebhookExecutionStatusCounts}
          hrisWebhookExecutionQueueCounts={syncHook.hrisWebhookExecutionQueueCounts}
          hrisWebhookExecutionLoading={syncHook.hrisWebhookExecutionListLoading}
          hrisWebhookExecutionLoadingMore={syncHook.hrisWebhookExecutionListLoadingMore}
          hrisWebhookExecutionHasMore={syncHook.hrisWebhookExecutionHasMore}
          selectedTenantID={selectedTenantID}
          selectedHRISWebhookExecutionID={syncHook.selectedHRISWebhookExecutionID}
          selectedHRISWebhookExecution={syncHook.selectedHRISWebhookExecutionItem}
          selectedHRISWebhookExecutionLoading={syncHook.selectedHRISWebhookExecutionLoading}
          selectedHRISWebhookExecutionError={syncHook.selectedHRISWebhookExecutionError}
          executionActionID={syncHook.executionActionID}
          onHRISWebhookExecutionHistoryViewChange={syncHook.onHRISWebhookExecutionHistoryViewChange}
          onLoadMoreHRISWebhookExecutions={syncHook.onLoadMoreHRISWebhookExecutions}
          onReplayHRISWebhookExecution={syncHook.onReplayHRISWebhookExecution}
          onSelectHRISWebhookExecution={onSelectHRISWebhookExecution}
          retryingWorkerAlertNotificationID={alertsHook.workerAlertNotificationActionID}
          retryingWorkerAlertNotificationBatch={alertsHook.workerAlertNotificationBatchActionID !== null}
          restoringWorkerAlertNotificationBatch={alertsHook.workerAlertNotificationRestoreBatchActionID !== null}
          suppressingWorkerAlertNotificationBatch={alertsHook.workerAlertNotificationSuppressBatchActionID !== null}
          autoRetryingWorkerAlertNotifications={alertsHook.workerAlertNotificationAutoRetryActionID !== null}
          hrisWebhookReceipts={syncHook.hrisWebhookReceipts}
          hrisWebhookReceiptTotal={syncHook.hrisWebhookReceiptTotal}
          hrisWebhookReceiptQueueCounts={syncHook.hrisWebhookReceiptQueueCounts}
          hrisWebhookReceiptLoading={syncHook.hrisWebhookReceiptListLoading}
          hrisWebhookReceiptLoadingMore={syncHook.hrisWebhookReceiptListLoadingMore}
          hrisWebhookReceiptHasMore={syncHook.hrisWebhookReceiptHasMore}
          onLoadMoreHRISWebhookReceipts={syncHook.onLoadMoreHRISWebhookReceipts}
          hrisWebhookDLQEntries={syncHook.hrisWebhookDLQEntries}
          hrisWebhookDLQTotal={syncHook.hrisWebhookDLQTotal}
          hrisWebhookDLQReplayCounts={syncHook.hrisWebhookDLQReplayCounts}
          hrisWebhookDLQLoading={syncHook.hrisWebhookDLQListLoading}
          hrisWebhookDLQLoadingMore={syncHook.hrisWebhookDLQListLoadingMore}
          hrisWebhookDLQHasMore={syncHook.hrisWebhookDLQHasMore}
          onLoadMoreHRISWebhookDLQ={syncHook.onLoadMoreHRISWebhookDLQ}
          hrisPullStates={hrisPullStates}
          onDispatchWorkerAlerts={alertsHook.onDispatchWorkerAlerts}
          onRetryWorkerAlertNotification={alertsHook.onRetryWorkerAlertNotification}
          onAutoRetryWorkerAlertNotifications={alertsHook.onAutoRetryWorkerAlertNotifications}
          onBatchRetryWorkerAlertNotifications={alertsHook.onBatchRetryWorkerAlertNotifications}
          onBatchRestoreWorkerAlertNotifications={alertsHook.onBatchRestoreWorkerAlertNotifications}
          onBatchSuppressWorkerAlertNotifications={alertsHook.onBatchSuppressWorkerAlertNotifications}
          onSaveWorkerAlertSubscription={alertsHook.onSaveWorkerAlertSubscription}
        />
      </Tabs>
    </div>
  )
}
