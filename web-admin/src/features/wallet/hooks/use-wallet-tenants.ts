import { useEffect, useMemo, useState } from "react"
import { useLocation } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  listTenants,
  listEnterpriseEmployees,
  listEnterpriseSyncJobs,
  listUserGroups,
  type CurrentUser,
  type Tenant,
  type EnterpriseEmployee,
  type EnterpriseSyncJob,
  type UserGroup,
} from "@/lib/api"
import { getViewerTenantID, isPlatformViewer } from "@/lib/viewer"
import {
  enterpriseWalletSegmentLabel,
  enterpriseWalletSegmentStatusLabel,
  parseTargetIDs,
  resolveEnterpriseTargetQuery,
  type WalletScenarioKind,
} from "../pages/wallet-page-utils"

type UseWalletTenantsParams = {
  token: string
  viewer: CurrentUser
}

export function useWalletTenants({ token, viewer }: UseWalletTenantsParams) {
  const { t } = useTranslation()
  const location = useLocation()
  const platformViewer = isPlatformViewer(viewer)
  const viewerTenantID = getViewerTenantID(viewer)

  const [tenants, setTenants] = useState<Tenant[]>([])
  const [tenantID, setTenantID] = useState("")

  const tenantsQuery = useQuery({
    queryKey: ["wallet-tenants", platformViewer ? "platform" : "tenant"],
    queryFn: () => (platformViewer ? listTenants(token) : Promise.resolve([])),
    staleTime: 30 * 1000,
  })
  const queryError = tenantsQuery.error instanceof Error ? tenantsQuery.error.message : ""

  const [enterpriseEmployees, setEnterpriseEmployees] = useState<EnterpriseEmployee[]>([])
  const [enterpriseUserGroups, setEnterpriseUserGroups] = useState<UserGroup[]>([])
  const [enterpriseSyncJobs, setEnterpriseSyncJobs] = useState<EnterpriseSyncJob[]>([])

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

  const enterpriseLatestSyncJob = useMemo(() => {
    const sorted = [...enterpriseSyncJobs].sort((left, right) => {
      return new Date(right.ended_at || right.started_at).getTime() - new Date(left.ended_at || left.started_at).getTime()
    })
    return sorted[0] ?? null
  }, [enterpriseSyncJobs])

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

  const hasWorkerAlertFlowHints = Boolean(
    enterpriseFlowContext &&
      (
        enterpriseFlowContext.workerAlertLevel ||
        enterpriseFlowContext.workerAlertTenantID ||
        enterpriseFlowContext.workerFilterHint ||
        enterpriseFlowContext.workerQueryHint
      ).trim()
  )

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

  async function loadEnterpriseData(nextTenantID: string) {
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

  function resetEnterpriseData() {
    setEnterpriseEmployees([])
    setEnterpriseUserGroups([])
    setEnterpriseSyncJobs([])
  }

  return {
    platformViewer,
    viewerTenantID,
    tenants,
    setTenants,
    tenantID,
    setTenantID,
    tenantsQuery,
    queryError,
    tenantByID,
    enterpriseFlowContext,
    enterpriseFlowSegmentDescriptor,
    enterpriseEmployees,
    enterpriseUserGroups,
    enterpriseSyncJobs,
    enterpriseLatestSyncJob,
    enterpriseSyncIssueHint,
    hasWorkerAlertFlowHints,
    loadEnterpriseData,
    resetEnterpriseData,
  }
}

export type EnterpriseFlowContext = NonNullable<ReturnType<typeof useWalletTenants>["enterpriseFlowContext"]>
