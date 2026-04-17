import { FormEvent, useEffect, useMemo, useState } from "react"
import { ArrowUpRightIcon, ShieldCheckIcon, UsersRoundIcon } from "lucide-react"
import { Link, useLocation, useNavigate } from "react-router-dom"
import { useQuery } from "@tanstack/react-query"

import { EnterpriseAlertsWorkspace } from "@/components/enterprise/enterprise-alerts-workspace"
import { EnterpriseEmployeesWorkspace } from "@/components/enterprise/enterprise-employees-workspace"
import { EnterpriseIDPWorkspace } from "@/components/enterprise/enterprise-idp-workspace"
import { EnterpriseSyncWorkspace } from "@/components/enterprise/enterprise-sync-workspace"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  getEnterpriseIDPConfig,
  listAccessPolicies,
  listEnterpriseEmployees,
  listEnterpriseJITProvisionApprovals,
  listEnterpriseSyncJobs,
  listEnterpriseSyncWorkerAlertSummary,
  listUserGroups,
  listTenants,
  listWalletPasses,
  reviewEnterpriseJITProvisionApproval,
  syncEnterpriseEmployees,
  updateEnterpriseJITProvisionApprovalExternalSync,
  type CurrentUser,
  type AccessPolicy,
  type EmployeeSyncInput,
  type EnterpriseEmployee,
  type EnterpriseIDPConfig,
  type EnterpriseJITProvisionApproval,
  type EnterpriseSyncJob,
  type EnterpriseSyncWorkerAlertSummaryItem,
  type Tenant,
  type UserGroup,
  type WalletPassInstance,
} from "@/lib/api"
import { canManageEnterprise, getViewerTenantID, isPlatformViewer } from "@/lib/viewer"

type EnterprisePageProps = {
  token: string
  viewer: CurrentUser
}

type EnterpriseSection = "employees" | "sync" | "idp" | "alerts"
type AlertLandingView = "overview" | "approval_backlog" | "directory_exceptions"
type AlertSegmentHint = "receipt_recovery"
type AlertSegmentStatusHint = "pending" | "attention" | "ready"
type SyncFocusHint = "worker_alert"
type WorkerReviewStatusHint = "handled"
type WorkerReviewStageHint = "alerts" | "directory" | "policies" | "issuance" | "sync"
type EnterpriseWorkflowState = "completed" | "pending" | "blocked"

const syncSourceOptions = [
  {
    value: "hris",
    label: "HRIS / 员工管理系统",
    description: "适合飞书人事、钉钉、BambooHR、Workday 等上游系统。",
  },
  {
    value: "scim",
    label: "SCIM / 目录同步",
    description: "适合持续增量同步、自动停用和目录治理。",
  },
  {
    value: "csv_import",
    label: "CSV / Excel 导入",
    description: "适合部署初期快速迁移或管理员批量导入。",
  },
  {
    value: "manual_sync",
    label: "手动同步",
    description: "适合临时人员、特殊岗位和小规模修补。",
  },
]

const sampleSyncPayload = `[
  {
    "external_id": "emp-001",
    "email": "employee@company.local",
    "full_name": "张晨曦",
    "department": "Finance",
    "job_title": "Finance Manager",
    "location": "HQ / 18F",
    "status": "active"
  }
]`

const syncSourceCheckpointsBySource: Record<string, string[]> = {
  hris: [
    "建议校验部门、岗位、楼宇字段映射是否稳定。",
    "建议确认上游主键与员工邮箱是否一一对应，避免后续重复对象。",
  ],
  scim: [
    "建议校验 external_id 与邮箱回写是否稳定，避免增量冲突。",
    "建议确认停用语义是否符合组织离职流程，避免下游权限残留。",
  ],
  csv_import: [
    "建议校验 CSV 字段头、必填列和日期格式，避免批量 rejected。",
    "建议确认重复邮箱与重复 external_id 的处理口径。",
  ],
  manual_sync: [
    "建议确认临时补录对象是否已补齐部门、岗位和楼宇字段。",
    "建议将临时对象及时回流到稳定用户组，避免后续策略缺对象。",
  ],
}

function formatDateTime(value?: string) {
  if (!value) {
    return "-"
  }
  const timestamp = new Date(value)
  if (Number.isNaN(timestamp.getTime())) {
    return value
  }
  return timestamp.toLocaleString("zh-CN")
}

function syncSourceLabel(source?: string) {
  if (!source) {
    return "-"
  }
  return syncSourceOptions.find((item) => item.value === source)?.label || source
}

function buildEnterpriseFlowLink(
  path: string,
  tenantID: string,
  stage: "directory" | "policies" | "issuance",
  hints?: Record<string, string>
) {
  const [pathname, rawQuery = ""] = path.split("?")
  const params = new URLSearchParams(rawQuery)
  params.set("from", "enterprise")
  params.set("flow", "sync_to_access")
  params.set("stage", stage)
  if (tenantID.trim()) {
    params.set("tenant_id", tenantID.trim())
  }
  if (hints) {
    Object.entries(hints).forEach(([key, value]) => {
      if (!key.trim() || !value.trim()) {
        return
      }
      params.set(key.trim(), value.trim())
    })
  }
  const nextQuery = params.toString()
  return nextQuery ? `${pathname}?${nextQuery}` : pathname
}

function syncSourceFailureHint(source: string, rejected: number) {
  if (source === "scim") {
    return `当前有 ${rejected} 条记录需要复核，建议先去“审批与异常”核对 SCIM 标识映射、停用语义和 rejected 原因。`
  }
  if (source === "csv_import") {
    return `当前有 ${rejected} 条记录需要复核，建议先去“审批与异常”核对 CSV 字段头、必填项和重复主键。`
  }
  if (source === "manual_sync") {
    return `当前有 ${rejected} 条记录需要复核，建议先去“审批与异常”处理临时补录冲突并复核字段映射。`
  }
  return `当前有 ${rejected} 条记录需要复核，建议先去“审批与异常”核对 HRIS 字段映射和 rejected 原因。`
}

function syncSourceSuccessHint(source: string, deactivated: number) {
  if (source === "scim" && deactivated > 0) {
    return `目录已更新，并同步停用了 ${deactivated} 名员工；建议先去“员工与用户组”复核停用对象的下游权限。`
  }
  if (source === "csv_import") {
    return "目录已更新，建议下一步去“员工与用户组”做首轮分组，并补齐策略对象。"
  }
  if (source === "manual_sync") {
    return "目录已更新，建议下一步去“员工与用户组”把临时对象纳入稳定分组。"
  }
  return "目录已更新，建议下一步去“员工与用户组”整理发放对象。"
}

function statusBadgeVariant(status?: string) {
  switch (status) {
    case "active":
    case "approved":
    case "completed":
      return "outline"
    case "rejected":
    case "failed":
      return "destructive"
    default:
      return "secondary"
  }
}

function workflowStateVariant(state: EnterpriseWorkflowState) {
  switch (state) {
    case "completed":
      return "outline"
    case "blocked":
      return "secondary"
    default:
      return "secondary"
  }
}

function workflowStateLabel(state: EnterpriseWorkflowState) {
  switch (state) {
    case "completed":
      return "已完成"
    case "blocked":
      return "待前置"
    default:
      return "待处理"
  }
}

function resolveEnterpriseSection(value?: string): EnterpriseSection {
  switch (value) {
    case "employees":
    case "sync":
    case "idp":
    case "alerts":
      return value
    default:
      return "employees"
  }
}

type EnterprisePageData = {
  employees: EnterpriseEmployee[]
  syncJobs: EnterpriseSyncJob[]
  approvals: EnterpriseJITProvisionApproval[]
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[]
  idpConfig: EnterpriseIDPConfig | null
  userGroups: UserGroup[]
  policies: AccessPolicy[]
  issuedPasses: WalletPassInstance[]
}

async function loadEnterprisePageData(token: string, tenantID: string): Promise<EnterprisePageData> {
  const [employeeItems, syncJobItems, approvalItems, workerAlertItems, groupItems, policyItems, passItems] = await Promise.all([
    listEnterpriseEmployees(token, tenantID),
    listEnterpriseSyncJobs(token, tenantID),
    listEnterpriseJITProvisionApprovals(token, {
      tenant_id: tenantID,
      limit: 12,
    }),
    listEnterpriseSyncWorkerAlertSummary(token, {
      tenant_id: tenantID,
      limit: 12,
    }),
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
    syncJobs: syncJobItems,
    approvals: approvalItems,
    workerAlerts: workerAlertItems,
    idpConfig: nextIDPConfig,
    userGroups: groupItems,
    policies: policyItems,
    issuedPasses: passItems,
  }
}

export function EnterprisePage({ token, viewer }: EnterprisePageProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const platformViewer = isPlatformViewer(viewer)
  const writable = canManageEnterprise(viewer)
  const viewerTenantID = getViewerTenantID(viewer)
  const activeSection = resolveEnterpriseSection(location.hash.replace(/^#/, ""))
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
    const approvalQuery =
      query.get("approval_query_hint")?.trim() ||
      query.get("target_hint")?.trim() ||
      query.get("target_email")?.trim() ||
      query.get("target_id")?.trim() ||
      ""
    const directoryQuery = query.get("worker_query_hint")?.trim() || query.get("sync_query_hint")?.trim() || ""
    const hasHints =
      activeSection === "alerts" &&
      Boolean(landingView || segmentHint || segmentStatus || syncStatus || syncSource || workerFilter || approvalQuery || directoryQuery)
    return {
      hasHints,
      contextKey: hasHints ? `${location.search}::${activeSection}` : "",
      approvalQuery: approvalQuery || undefined,
      directoryQuery: directoryQuery || undefined,
      landingView,
      segmentHint,
      segmentStatus,
      workerFilter,
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
    const workerQuery = query.get("worker_query_hint")?.trim() || ""
    const hasHints =
      activeSection === "sync" && Boolean(focusHint || workerFilter || workerQuery || workerReviewStatus || workerReviewStage)
    return {
      hasHints,
      contextKey: hasHints ? `${location.search}::${activeSection}` : "",
      focusHint,
      workerFilter,
      workerReviewStage,
      workerReviewStatus,
      workerQuery: workerQuery || undefined,
    }
  }, [activeSection, location.search])

  const [tenants, setTenants] = useState<Tenant[]>([])
  const [selectedTenantID, setSelectedTenantID] = useState(viewerTenantID)
  const [employees, setEmployees] = useState<EnterpriseEmployee[]>([])
  const [syncJobs, setSyncJobs] = useState<EnterpriseSyncJob[]>([])
  const [approvals, setApprovals] = useState<EnterpriseJITProvisionApproval[]>([])
  const [workerAlerts, setWorkerAlerts] = useState<EnterpriseSyncWorkerAlertSummaryItem[]>([])
  const [idpConfig, setIDPConfig] = useState<EnterpriseIDPConfig | null>(null)
  const [userGroups, setUserGroups] = useState<UserGroup[]>([])
  const [policies, setPolicies] = useState<AccessPolicy[]>([])
  const [issuedPasses, setIssuedPasses] = useState<WalletPassInstance[]>([])

  const [syncSource, setSyncSource] = useState("hris")
  const [syncRequestID, setSyncRequestID] = useState(() => `sync-${Date.now()}`)
  const [syncPayload, setSyncPayload] = useState(sampleSyncPayload)
  const [syncSummary, setSyncSummary] = useState("")
  const [approvalActionID, setApprovalActionID] = useState<string | null>(null)

  const [syncing, setSyncing] = useState(false)
  const [error, setError] = useState("")

  const tenantsQuery = useQuery({
    queryKey: ["enterprise-tenants", token, platformViewer ? "platform" : "tenant"],
    queryFn: () => (platformViewer ? listTenants(token) : Promise.resolve([])),
    staleTime: 30 * 1000,
  })
  const enterpriseDataQuery = useQuery({
    queryKey: ["enterprise-data", token, selectedTenantID.trim()],
    queryFn: () => loadEnterprisePageData(token, selectedTenantID.trim()),
    enabled: selectedTenantID.trim().length > 0,
    staleTime: 30 * 1000,
  })
  const loading =
    (platformViewer && tenantsQuery.isPending) ||
    (selectedTenantID.trim().length > 0 && enterpriseDataQuery.isPending)
  const queryError =
    (tenantsQuery.error instanceof Error && tenantsQuery.error.message) ||
    (enterpriseDataQuery.error instanceof Error && enterpriseDataQuery.error.message) ||
    ""

  const selectedTenant = useMemo(
    () => tenants.find((item) => item.id === selectedTenantID) ?? null,
    [selectedTenantID, tenants]
  )
  const tenantGroups = useMemo(
    () => userGroups.filter((item) => item.tenant_id === selectedTenantID.trim()),
    [selectedTenantID, userGroups]
  )
  const tenantPolicies = useMemo(
    () => policies.filter((item) => item.tenant_id === selectedTenantID.trim()),
    [policies, selectedTenantID]
  )
  const activeEmployeeCount = useMemo(
    () => employees.filter((item) => item.status === "active").length,
    [employees]
  )
  const pendingApprovalCount = useMemo(
    () => approvals.filter((item) => item.status === "pending").length,
    [approvals]
  )
  const failedSyncJobCount = useMemo(
    () => syncJobs.filter((item) => item.status !== "completed" || item.rejected > 0).length,
    [syncJobs]
  )
  const workerAlertCount = useMemo(
    () => workerAlerts.reduce((sum, item) => sum + item.count, 0),
    [workerAlerts]
  )
  const idpReady = Boolean(idpConfig && idpConfig.status === "active")
  const issuedPassCount = issuedPasses.length
  const primaryEmployeeHint = useMemo(
    () =>
      employees.find((item) => item.status === "active" && item.email.trim()) ??
      employees.find((item) => item.status === "active") ??
      employees[0] ??
      null,
    [employees]
  )
  const primaryEmployeeTargetHint = useMemo(() => {
    if (!primaryEmployeeHint) {
      return ""
    }
    return primaryEmployeeHint.email.trim() || primaryEmployeeHint.external_id.trim() || primaryEmployeeHint.id.trim()
  }, [primaryEmployeeHint])
  const deactivatedEmployeeHint = useMemo(
    () =>
      employees.find((item) => item.status !== "active" && item.email.trim()) ??
      employees.find((item) => item.status !== "active") ??
      null,
    [employees]
  )
  const primaryGroupHint = useMemo(() => tenantGroups[0]?.name?.trim() || "Common Office Access", [tenantGroups])
  const primaryPolicyHint = useMemo(() => {
    if (tenantPolicies[0]?.name?.trim()) {
      return tenantPolicies[0].name.trim()
    }
    return primaryGroupHint ? `${primaryGroupHint} 办公通行策略` : "首批权限策略"
  }, [primaryGroupHint, tenantPolicies])
  const directoryFlowLink = buildEnterpriseFlowLink("/access/directory", selectedTenantID, "directory", {
    group_name: primaryGroupHint,
    group_desc: "来源企业页同步承接草稿",
    group_member_email: primaryEmployeeHint?.email || "",
    group_member_id: primaryEmployeeHint?.id || "",
    group_member_name: primaryEmployeeHint?.full_name || "",
  })
  const deactivatedDirectoryFlowLink = buildEnterpriseFlowLink("/access/directory", selectedTenantID, "directory", {
    group_name: primaryGroupHint,
    group_desc: "来源企业页停用对象清理",
    group_member_email: deactivatedEmployeeHint?.email || "",
    group_member_id: deactivatedEmployeeHint?.id || "",
    group_member_name: deactivatedEmployeeHint?.full_name || "",
    group_member_status: deactivatedEmployeeHint?.status || "",
    remediation_hint: "deactivated_cleanup",
  })
  const policiesFlowLink = buildEnterpriseFlowLink("/access/policies", selectedTenantID, "policies", {
    policy_group: primaryGroupHint,
    policy_name: primaryPolicyHint,
  })
  const walletFlowLink = buildEnterpriseFlowLink("/wallet?scenario=employee_mobile", selectedTenantID, "issuance", {
    target_hint: "user",
    target_email: primaryEmployeeHint?.email || "",
    target_id: primaryEmployeeTargetHint,
    target_name: primaryEmployeeHint?.full_name || "",
    template_hint: "employee",
  })
  const enterpriseSyncFlowLink = `${buildEnterpriseFlowLink("/enterprise", selectedTenantID, "directory")}#sync`
  const enterpriseAlertsFlowLink = `${buildEnterpriseFlowLink("/enterprise", selectedTenantID, "directory")}#alerts`
  const sortedSyncJobs = useMemo(
    () =>
      [...syncJobs].sort((left, right) => {
        const leftTime = new Date(left.ended_at || left.started_at).getTime() || 0
        const rightTime = new Date(right.ended_at || right.started_at).getTime() || 0
        return rightTime - leftTime
      }),
    [syncJobs]
  )
  const latestSyncJob = sortedSyncJobs[0] ?? null
  const workflowSteps = useMemo(
    () => [
      {
        id: "sync" as const,
        title: "1. 接入员工目录",
        metric: loading ? "--" : `${activeEmployeeCount} 名在职员工 / ${syncJobs.length} 次同步`,
        state: activeEmployeeCount > 0 ? ("completed" as const) : ("pending" as const),
        helper:
          activeEmployeeCount > 0
            ? "员工目录已接通，可以开始整理用户组。"
            : "先接入 HRIS、SCIM、CSV 或手动同步，把组织对象补齐。",
        actionLabel: activeEmployeeCount > 0 ? "查看员工目录" : "去导入与同步",
      },
      {
        id: "directory" as const,
        title: "2. 整理用户组",
        metric: loading ? "--" : `${tenantGroups.length} 个用户组`,
        state:
          activeEmployeeCount === 0
            ? ("blocked" as const)
            : tenantGroups.length > 0
              ? ("completed" as const)
              : ("pending" as const),
        helper:
          activeEmployeeCount === 0
            ? "需要先有员工目录，用户组才有稳定成员来源。"
            : tenantGroups.length > 0
              ? "用户组已具备基础形态，可以继续落策略。"
              : "建议先建立按岗位、部门或场景划分的基础用户组。",
        actionLabel: "去员工与用户组",
      },
      {
        id: "policies" as const,
        title: "3. 建立权限策略",
        metric: loading ? "--" : `${tenantPolicies.length} 条策略`,
        state:
          tenantGroups.length === 0
            ? ("blocked" as const)
            : tenantPolicies.length > 0
              ? ("completed" as const)
              : ("pending" as const),
        helper:
          tenantGroups.length === 0
            ? "先把用户组整理完成，再把规则落到楼宇、区域和门点。"
            : tenantPolicies.length > 0
              ? "策略规则已具备，可进入发放中心。"
              : "把“谁能进哪里、何时能进”单独配置成策略层。",
        actionLabel: "去权限策略",
      },
      {
        id: "issuance" as const,
        title: "4. 发放 MistyPass",
        metric: loading ? "--" : `${issuedPassCount} 张已发放凭证`,
        state:
          tenantPolicies.length === 0
            ? ("blocked" as const)
            : issuedPassCount > 0
              ? ("completed" as const)
              : ("pending" as const),
        helper:
          tenantPolicies.length === 0
            ? "先让目录和策略到位，再进行长期员工发放。"
            : issuedPassCount > 0
              ? "发放主路径已跑通，可继续管理状态、补发和回执。"
              : "模板、用户组和策略就绪后，就可以进入发放中心下发 MistyPass。",
        actionLabel: issuedPassCount > 0 ? "查看发放中心" : "去凭证发放",
      },
    ],
    [activeEmployeeCount, issuedPassCount, loading, syncJobs.length, tenantGroups.length, tenantPolicies.length]
  )
  const nextWorkflowAction = useMemo(() => {
    if (activeEmployeeCount === 0) {
      return {
        title: "先接入员工目录",
        description: "企业页已经提供 HRIS、SCIM、CSV 和手动同步入口。先把员工目录接通，再去整理用户组。",
        kind: "section" as const,
        section: "sync" as const,
        label: "去导入与同步",
      }
    }
    if (tenantGroups.length === 0) {
      return {
        title: "把员工整理成用户组",
        description: `当前已有 ${activeEmployeeCount} 名在职员工，但还没有稳定的用户组。先按岗位或部门建组，再进入策略配置。`,
        kind: "route" as const,
        to: directoryFlowLink,
        label: "去员工与用户组",
      }
    }
    if (tenantPolicies.length === 0) {
      return {
        title: "建立首批权限策略",
        description: `当前已有 ${tenantGroups.length} 个用户组，但权限规则还没落地。下一步应配置楼宇、区域和门点策略。`,
        kind: "route" as const,
        to: policiesFlowLink,
        label: "去权限策略",
      }
    }
    if (issuedPassCount === 0) {
      return {
        title: "进入发放中心下发 MistyPass",
        description: "目录、用户组和权限策略都已具备，可以开始员工移动凭证发放，并继续补回执与状态维护。",
        kind: "route" as const,
        to: walletFlowLink,
        label: "去凭证发放",
      }
    }
    return {
      title: "主路径已跑通",
      description: "当前组织已经具备员工目录、用户组、权限策略和已发放凭证。下一步以状态维护、补发和异常处理为主。",
      kind: "route" as const,
      to: walletFlowLink,
      label: "查看发放中心",
    }
  }, [activeEmployeeCount, directoryFlowLink, issuedPassCount, policiesFlowLink, tenantGroups.length, tenantPolicies.length, walletFlowLink])
  const workflowDirectAction = useMemo(() => {
    if (nextWorkflowAction.kind === "section") {
      return {
        kind: "section" as const,
        section: nextWorkflowAction.section,
        label: nextWorkflowAction.label,
      }
    }
    return {
      kind: "route" as const,
      to: nextWorkflowAction.to,
      label: nextWorkflowAction.label,
    }
  }, [nextWorkflowAction])
  const workflowReturnAction = useMemo(() => {
    if (nextWorkflowAction.kind === "section") {
      return {
        kind: "section" as const,
        section: nextWorkflowAction.section,
        label: `处理后${nextWorkflowAction.label}`,
      }
    }
    return {
      kind: "route" as const,
      to: nextWorkflowAction.to,
      label: `处理后${nextWorkflowAction.label}`,
    }
  }, [nextWorkflowAction])
  const alertLandingCards = useMemo(() => {
    const resolvePostSyncAction = () => {
      if (activeEmployeeCount === 0) {
        return {
          kind: "section" as const,
          section: "employees" as const,
          label: "去员工目录复核",
        }
      }
      if (tenantGroups.length === 0) {
        return {
          kind: "route" as const,
          to: directoryFlowLink,
          label: "去员工与用户组",
        }
      }
      if (tenantPolicies.length === 0) {
        return {
          kind: "route" as const,
          to: policiesFlowLink,
          label: "去权限策略",
        }
      }
      if (issuedPassCount === 0) {
        return {
          kind: "route" as const,
          to: walletFlowLink,
          label: "去凭证发放",
        }
      }
      return {
        kind: "route" as const,
        to: walletFlowLink,
        label: "查看发放中心",
      }
    }

    const postSyncAction = resolvePostSyncAction()
    const postSyncReturnAction =
      postSyncAction.kind === "section"
        ? {
            kind: "section" as const,
            section: postSyncAction.section,
            label: `处理后${postSyncAction.label}`,
          }
        : {
            kind: "route" as const,
            to: postSyncAction.to,
            label: `处理后${postSyncAction.label}`,
          }

    const syncLandingCard = !latestSyncJob
      ? {
          title: "同步结果落地",
          statusLabel: "待导入",
          statusVariant: "secondary" as const,
          description: "当前还没有同步结果。先提交首批目录，再把人员继续回流到用户组、策略与发放。",
          action: {
            kind: "section" as const,
            section: "sync" as const,
            label: "去导入与同步",
          },
          returnHint: "完成导入后立即回到主路径，避免停在任务提交提示。",
        }
      : latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0
        ? {
            title: "同步结果落地",
            statusLabel: "待复核",
            statusVariant: "destructive" as const,
            description: `${syncSourceLabel(latestSyncJob.source)} 最近结果 ${latestSyncJob.status} / rejected ${latestSyncJob.rejected}，需要先清理异常。`,
            action: {
              kind: "section" as const,
              section: "alerts" as const,
              label: "去审批与异常",
            },
            returnAction: postSyncReturnAction,
            returnHint: "同步异常处理完成后，直接回流到目录、策略或发放。",
          }
        : {
            title: "同步结果落地",
            statusLabel:
              activeEmployeeCount === 0
                ? "待确认"
                : tenantGroups.length === 0 || tenantPolicies.length === 0 || issuedPassCount === 0
                  ? "待回流"
                  : "已闭环",
            statusVariant:
              activeEmployeeCount === 0
                ? ("secondary" as const)
                : tenantGroups.length === 0 || tenantPolicies.length === 0 || issuedPassCount === 0
                  ? ("secondary" as const)
                  : ("outline" as const),
            description:
              activeEmployeeCount === 0
                ? "最近同步已完成，但员工目录仍为空，需要先复核目录字段映射。"
                : tenantGroups.length === 0
                  ? `目录已有 ${activeEmployeeCount} 名在职员工，下一步应先整理用户组。`
                  : tenantPolicies.length === 0
                    ? `当前已有 ${tenantGroups.length} 个用户组，下一步应继续落地权限策略。`
                    : issuedPassCount === 0
                      ? "目录与策略链路已具备，下一步应进入发放中心开始员工发放。"
                      : `目录、策略与发放链路已连通（已发放 ${issuedPassCount} 张）。`,
            action: postSyncAction,
            returnHint:
              tenantGroups.length === 0 || tenantPolicies.length === 0 || issuedPassCount === 0
                ? "建议按当前动作继续推进主路径，不要停在同步结果页。"
                : "主路径已跑通，后续以状态维护与补发为主。",
          }

    const approvalLandingCard =
      pendingApprovalCount > 0
        ? {
            title: "审批积压落地",
            statusLabel: `${pendingApprovalCount} 条待审批`,
            statusVariant: "secondary" as const,
            description: "待审批项可能阻塞企业登录后的自动开户与同步回写，建议先在本页完成审批清理。",
            action: {
              kind: "section" as const,
              section: "alerts" as const,
              label: "去处理审批",
            },
            returnAction: workflowReturnAction,
            returnHint: "审批处理完成后，立即回流到主路径下一步动作。",
          }
        : {
            title: "审批积压落地",
            statusLabel: "已清空",
            statusVariant: "outline" as const,
            description: "当前没有审批积压，可直接继续目录、策略或发放主路径。",
            action: workflowDirectAction,
          }

    const directoryExceptionCard =
      latestSyncJob && latestSyncJob.deactivated > 0
        ? {
            title: "目录异常落地",
            statusLabel: "待清理",
            statusVariant: "secondary" as const,
            description: `最近一次同步停用了 ${latestSyncJob.deactivated} 名员工，需复核这些对象是否仍在用户组、策略或发放对象里。`,
            action: {
              kind: "route" as const,
              to: directoryFlowLink,
              label: "去员工与用户组复核",
            },
            returnAction: workflowReturnAction,
            returnHint: "目录清理完成后，继续回流策略或发放动作。",
          }
        : latestSyncJob && latestSyncJob.status === "completed" && activeEmployeeCount === 0
          ? {
              title: "目录异常落地",
              statusLabel: "需复核",
              statusVariant: "destructive" as const,
              description: "最近同步已完成但目录仍为空。建议先核对目录字段映射和上游启用状态。",
              action: {
                kind: "section" as const,
                section: "employees" as const,
                label: "去员工目录复核",
              },
              returnAction: {
                kind: "section" as const,
                section: "sync" as const,
                label: "复核后回到导入与同步",
              },
            }
          : {
              title: "目录异常落地",
              statusLabel: "目录稳定",
              statusVariant: "outline" as const,
              description: "当前未发现明显目录异常，可继续按主路径推进后续动作。",
              action: {
                kind: "route" as const,
                to: directoryFlowLink,
                label: "查看员工与用户组",
              },
            }

    return [syncLandingCard, approvalLandingCard, directoryExceptionCard]
  }, [
    activeEmployeeCount,
    issuedPassCount,
    latestSyncJob,
    pendingApprovalCount,
    tenantGroups.length,
    tenantPolicies.length,
    directoryFlowLink,
    policiesFlowLink,
    walletFlowLink,
    workflowDirectAction,
    workflowReturnAction,
  ])
  const attentionItems = useMemo(() => {
    const items: Array<{
      title: string
      description: string
      actionLabel: string
      onClick: () => void
    }> = []

    if (!idpReady) {
      items.push({
        title: "企业登录尚未就绪",
        description: "当前组织还没有可用的企业登录配置。需要先补 IdP 配置，避免企业自身登录继续缺位。",
        actionLabel: "去企业登录",
        onClick: () => goToSection("idp"),
      })
    }
    if (failedSyncJobCount > 0) {
      items.push({
        title: "同步结果需要复核",
        description: `最近有 ${failedSyncJobCount} 条同步任务未完全成功，建议先检查 rejected 或失败记录，避免目录数据带病进入用户组。`,
        actionLabel: "去审批与异常",
        onClick: () => goToSection("alerts"),
      })
    }
    if (pendingApprovalCount > 0) {
      items.push({
        title: "存在待处理审批",
        description: `当前有 ${pendingApprovalCount} 条待处理审批，可能阻塞企业登录后的自动开户与同步。`,
        actionLabel: "去处理审批",
        onClick: () => goToSection("alerts"),
      })
    }
    if (workerAlertCount > 0) {
      items.push({
        title: "同步 worker 有异常",
        description: `最近累计 ${workerAlertCount} 条 worker 告警，建议先排查重试、阈值或上游同步质量。`,
        actionLabel: "查看异常",
        onClick: () => goToSection("alerts"),
      })
    }

    return items.slice(0, 4)
  }, [failedSyncJobCount, idpReady, pendingApprovalCount, workerAlertCount])
  const alertRecoveryAction = useMemo(() => {
    const blockerCount = failedSyncJobCount + pendingApprovalCount + workerAlertCount + (idpReady ? 0 : 1)
    return {
      blockerCount,
      title:
        blockerCount > 0
          ? "处理完当前阻塞项后，继续回到主路径"
          : "当前没有明显阻塞项，可直接回到主路径",
      description:
        blockerCount > 0
          ? `当前仍有 ${blockerCount} 个阻塞维度需要关注。完成这些处理后，建议直接承接到下一步业务动作，避免停在异常页。`
          : "企业登录、同步和审批主路径当前基本通畅，可以继续回到目录、策略或发放推进业务。",
      nextAction: nextWorkflowAction,
    }
  }, [failedSyncJobCount, idpReady, nextWorkflowAction, pendingApprovalCount, workerAlertCount])
  const idpOutcomeAction = useMemo(() => {
    if (!idpReady) {
      return {
        title: "先补企业登录配置",
        description: "当前组织还没有可用的企业登录配置。先补齐 IdP，再继续处理审批积压和目录回流。",
        kind: "section" as const,
        section: "idp" as const,
        label: "去企业登录",
      }
    }
    if (pendingApprovalCount > 0 || failedSyncJobCount > 0 || workerAlertCount > 0) {
      return {
        title: "企业登录已具备基础条件，先清审批与异常",
        description: `当前仍有 ${pendingApprovalCount} 条待审批、${failedSyncJobCount} 条待复核或 ${workerAlertCount} 条 worker 告警，建议先回到审批与异常做收口。`,
        kind: "section" as const,
        section: "alerts" as const,
        label: "去审批与异常",
      }
    }
    if (activeEmployeeCount === 0) {
      return {
        title: "回到目录来源确认同步结果",
        description: "企业登录已就绪，但当前还没有有效员工目录。建议回到导入与同步，确认目录来源和首批同步结果。",
        kind: "section" as const,
        section: "sync" as const,
        label: "去导入与同步",
      }
    }
    if (tenantGroups.length === 0) {
      return {
        title: "把企业登录接入后的员工整理成用户组",
        description: "员工目录已经具备，下一步应先整理用户组，避免策略和发放继续缺少对象。",
        kind: "route" as const,
        to: directoryFlowLink,
        label: "去员工与用户组",
      }
    }
    if (tenantPolicies.length === 0) {
      return {
        title: "继续去权限策略落规则",
        description: "企业登录、员工目录和用户组都已具备，下一步应把楼宇、区域和门点规则落地。",
        kind: "route" as const,
        to: policiesFlowLink,
        label: "去权限策略",
      }
    }
    return {
      title: "企业登录链路已闭环，回到发放中心",
      description: "企业登录、目录、用户组和策略都已具备，可以回到发放中心继续员工 MistyPass 发放和状态维护。",
      kind: "route" as const,
      to: walletFlowLink,
      label: "去凭证发放",
    }
  }, [
    activeEmployeeCount,
    directoryFlowLink,
    failedSyncJobCount,
    idpReady,
    pendingApprovalCount,
    policiesFlowLink,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
    workerAlertCount,
  ])
  const syncOutcomeAction = useMemo(() => {
    if (!latestSyncJob) {
      return {
        title: "先完成第一批员工导入",
        description: "当前还没有同步结果。先选定一个同步来源并导入员工目录，再继续整理用户组。",
        kind: "section" as const,
        section: "sync" as const,
        label: "继续导入员工",
      }
    }
    if (latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0) {
      return {
        title: "先复核同步失败和 rejected 记录",
        description: `最近一次 ${syncSourceLabel(latestSyncJob.source)} 同步仍有异常，建议先处理 ${latestSyncJob.rejected} 条 rejected 记录和失败项，再让目录进入下游。`,
        kind: "section" as const,
        section: "alerts" as const,
        label: "去审批与异常",
      }
    }
    if (activeEmployeeCount === 0) {
      return {
        title: "回到员工目录确认结果",
        description: "同步任务看起来已完成，但当前员工目录仍为空。建议先确认同步来源配置和载荷是否正确。",
        kind: "section" as const,
        section: "employees" as const,
        label: "查看员工目录",
      }
    }
    if (tenantGroups.length === 0) {
      return {
        title: "把新增员工整理成用户组",
        description: `目录已接通，当前已有 ${activeEmployeeCount} 名在职员工。下一步应先建立用户组，避免策略和发放继续缺对象。`,
        kind: "route" as const,
        to: directoryFlowLink,
        label: "去员工与用户组",
      }
    }
    if (tenantPolicies.length === 0) {
      return {
        title: "开始生成首批权限策略",
        description: `当前已有 ${tenantGroups.length} 个用户组，可以直接去权限页生成策略草稿，并把规则落到楼宇、区域和门点。`,
        kind: "route" as const,
        to: policiesFlowLink,
        label: "去权限策略",
      }
    }
    return {
      title: "同步主路径已跑通，进入发放中心",
      description: "目录、用户组和策略都已具备，可以开始员工 MistyPass 发放，并继续状态维护和补发。",
      kind: "route" as const,
      to: walletFlowLink,
      label: "去凭证发放",
    }
  }, [activeEmployeeCount, directoryFlowLink, latestSyncJob, policiesFlowLink, tenantGroups.length, tenantPolicies.length, walletFlowLink])
  const syncIssueCards = useMemo(() => {
    if (!latestSyncJob) {
      return []
    }

    const items: Array<{
      title: string
      description: string
      label: string
      kind: "section" | "route"
      section?: EnterpriseSection
      to?: string
    }> = []

    if (latestSyncJob.status !== "completed") {
      items.push({
        title: "最近一次同步尚未完全成功",
        description: `${syncSourceLabel(latestSyncJob.source)} 最近结果为 ${latestSyncJob.status}，建议先去审批与异常查看失败项和 worker 告警。`,
        label: "去审批与异常",
        kind: "section",
        section: "alerts",
      })
    }
    if (latestSyncJob.rejected > 0) {
      items.push({
        title: "存在 rejected 记录",
        description: `最近一次同步有 ${latestSyncJob.rejected} 条记录未被接纳，目录数据进入下游前需要先复核字段和权限映射。`,
        label: "去审批与异常",
        kind: "section",
        section: "alerts",
      })
    }
    if (latestSyncJob.deactivated > 0) {
      items.push({
        title: "有员工被停用",
        description: `最近一次同步停用了 ${latestSyncJob.deactivated} 名员工，建议尽快复核这些人是否仍出现在用户组、策略或发放对象中。`,
        label: "去停用对象清理",
        kind: "route",
        to: deactivatedDirectoryFlowLink,
      })
    }
    if (latestSyncJob.status === "completed" && activeEmployeeCount === 0) {
      items.push({
        title: "同步完成但目录仍为空",
        description: "同步任务显示完成，但当前员工目录没有有效员工。建议先核对载荷、字段映射和上游启用状态。",
        label: "查看员工目录",
        kind: "section",
        section: "employees",
      })
    }

    return items.slice(0, 4)
  }, [activeEmployeeCount, deactivatedDirectoryFlowLink, latestSyncJob])
  const syncRemediationSteps = useMemo(() => {
    const steps: Array<{
      title: string
      description: string
      state: EnterpriseWorkflowState
      kind: "section" | "route"
      label: string
      section?: EnterpriseSection
      to?: string
    }> = []

    if (!latestSyncJob) {
      steps.push({
        title: "先完成首批同步",
        description: "当前还没有同步结果，先提交一批员工目录，再进入异常复核和后续整理。",
        state: "pending",
        kind: "section",
        label: "去导入与同步",
        section: "sync",
      })
      return steps
    }

    steps.push({
      title: "复核最近一次同步结果",
      description: `${syncSourceLabel(latestSyncJob.source)} 最近结果为 ${latestSyncJob.status}，先确认创建、更新、停用和 rejected 数量是否符合预期。`,
      state: "completed",
      kind: "section",
      label: "查看同步结果",
      section: "sync",
    })

    steps.push({
      title: "处理 rejected 与失败项",
      description:
        latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0
          ? `当前仍有 ${latestSyncJob.rejected} 条 rejected 或未完成项，建议先去审批与异常复核字段映射和失败记录。`
          : "最近一次同步没有 rejected 或失败项，这一步已完成。",
      state: latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0 ? "pending" : "completed",
      kind: "section",
      label: "去审批与异常",
      section: "alerts",
    })

    steps.push({
      title: "复核停用员工的下游影响",
      description:
        latestSyncJob.deactivated > 0
          ? `最近一次同步停用了 ${latestSyncJob.deactivated} 名员工，建议走“停用对象清理”入口，直接定位成员并复核其所在用户组与后续发放对象。`
          : "最近一次同步没有停用员工，这一步已完成。",
      state: latestSyncJob.deactivated > 0 ? "pending" : "completed",
      kind: "route",
      label: "去停用对象清理",
      to: deactivatedDirectoryFlowLink,
    })

    steps.push({
      title: "把目录继续推进到策略与发放",
      description:
        activeEmployeeCount === 0
          ? "同步结果还没有形成有效员工目录，先回到员工目录核对载荷和字段映射。"
          : tenantGroups.length === 0
            ? "目录已经具备，但还没有用户组。下一步应先整理用户组，再进入策略配置。"
            : tenantPolicies.length === 0
              ? "目录和用户组都已具备，可以继续去权限策略落规则。"
              : "目录、用户组和策略都已具备，可以继续进入发放中心。",
      state:
        activeEmployeeCount === 0
          ? "blocked"
          : tenantGroups.length === 0 || tenantPolicies.length === 0
            ? "pending"
            : "completed",
      kind: activeEmployeeCount === 0 ? "section" : "route",
      label:
        activeEmployeeCount === 0
          ? "查看员工目录"
          : tenantGroups.length === 0
            ? "去员工与用户组"
            : tenantPolicies.length === 0
              ? "去权限策略"
              : "去凭证发放",
      section: activeEmployeeCount === 0 ? "employees" : undefined,
      to:
        activeEmployeeCount === 0
          ? undefined
          : tenantGroups.length === 0
            ? directoryFlowLink
            : tenantPolicies.length === 0
              ? policiesFlowLink
              : walletFlowLink,
    })

    return steps
  }, [activeEmployeeCount, deactivatedDirectoryFlowLink, directoryFlowLink, latestSyncJob, policiesFlowLink, tenantGroups.length, tenantPolicies.length, walletFlowLink])
  const syncSourceFeedback = useMemo(() => {
    const checkpoints = syncSourceCheckpointsBySource[syncSource] ?? []
    const latestJobOfSource = sortedSyncJobs.find((item) => item.source === syncSource) ?? null
    const sourceLabel = syncSourceLabel(syncSource)

    if (!latestJobOfSource) {
      return {
        title: `${sourceLabel} 仍待首批结果`,
        statusLabel: "待提交",
        statusVariant: "secondary" as const,
        description: "当前来源还没有最近同步记录。建议先提交首批目录并马上复核结果，避免停在任务创建阶段。",
        checkpoints,
        action: {
          kind: "section" as const,
          section: "sync" as const,
          label: "提交同步任务",
        },
      }
    }

    if (latestJobOfSource.status !== "completed" || latestJobOfSource.rejected > 0) {
      return {
        title: `${sourceLabel} 结果待复核`,
        statusLabel: "需复核",
        statusVariant: "destructive" as const,
        description: `${sourceLabel} 最近结果为 ${latestJobOfSource.status}，rejected ${latestJobOfSource.rejected}。建议优先处理失败与 rejected，再继续下游流程。`,
        checkpoints,
        action: {
          kind: "section" as const,
          section: "alerts" as const,
          label: "去审批与异常",
        },
      }
    }

    if (syncSource === "scim" && latestJobOfSource.deactivated > 0) {
      return {
        title: "SCIM 停用结果待回流",
        statusLabel: "待回流",
        statusVariant: "secondary" as const,
        description: `最近一次 SCIM 同步停用了 ${latestJobOfSource.deactivated} 名员工。建议先回到目录域清理用户组与策略对象，再继续推进。`,
        checkpoints,
        action: {
          kind: "route" as const,
          to: deactivatedDirectoryFlowLink,
          label: "去停用对象清理",
        },
      }
    }

    if (activeEmployeeCount === 0) {
      return {
        title: `${sourceLabel} 结果待确认`,
        statusLabel: "待确认",
        statusVariant: "secondary" as const,
        description: "同步任务显示已完成，但当前员工目录仍为空。建议先回到员工目录确认载荷与字段映射。",
        checkpoints,
        action: {
          kind: "section" as const,
          section: "employees" as const,
          label: "查看员工目录",
        },
      }
    }

    if (tenantGroups.length === 0) {
      return {
        title: `${sourceLabel} 已接通，建议推进分组`,
        statusLabel: "可推进",
        statusVariant: "outline" as const,
        description: `当前已有 ${activeEmployeeCount} 名在职员工，建议直接进入用户组整理，避免策略与发放继续缺对象。`,
        checkpoints,
        action: {
          kind: "route" as const,
          to: directoryFlowLink,
          label: "去员工与用户组",
        },
      }
    }

    if (tenantPolicies.length === 0) {
      return {
        title: `${sourceLabel} 已回流到目录域`,
        statusLabel: "可推进",
        statusVariant: "outline" as const,
        description: `当前已有 ${tenantGroups.length} 个用户组，建议继续去策略域落规则，避免主流程卡在“仅有目录”。`,
        checkpoints,
        action: {
          kind: "route" as const,
          to: policiesFlowLink,
          label: "去权限策略",
        },
      }
    }

    return {
      title: `${sourceLabel} 主路径已闭环`,
      statusLabel: "已闭环",
      statusVariant: "outline" as const,
      description: "目录、用户组和策略都已具备。下一步可进入发放中心继续员工凭证下发与状态维护。",
      checkpoints,
      action: {
        kind: "route" as const,
        to: walletFlowLink,
        label: "去凭证发放",
      },
    }
  }, [activeEmployeeCount, deactivatedDirectoryFlowLink, directoryFlowLink, policiesFlowLink, sortedSyncJobs, syncSource, tenantGroups.length, tenantPolicies.length, walletFlowLink])
  const syncSourceStatusCards = useMemo(() => {
    return syncSourceOptions.map((item) => {
      const sourceLabel = item.label
      const latestJobOfSource = sortedSyncJobs.find((job) => job.source === item.value) ?? null
      const checkpoints = syncSourceCheckpointsBySource[item.value] ?? []
      const metrics = latestJobOfSource
        ? `创建 ${latestJobOfSource.created} / 更新 ${latestJobOfSource.updated} / 停用 ${latestJobOfSource.deactivated} / rejected ${latestJobOfSource.rejected}`
        : "尚无同步结果"
      const latestEndedAt = latestJobOfSource ? formatDateTime(latestJobOfSource.ended_at || latestJobOfSource.started_at) : "-"

      if (!latestJobOfSource) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: "待接入",
          statusVariant: "secondary" as const,
          description: "当前来源还没有同步记录。建议先提交首批目录再继续下游动作。",
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "section" as const,
            section: "sync" as const,
            label: "去导入与同步",
          },
        }
      }

      if (latestJobOfSource.status !== "completed" || latestJobOfSource.rejected > 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: "需复核",
          statusVariant: "destructive" as const,
          description: `最近结果 ${latestJobOfSource.status} / rejected ${latestJobOfSource.rejected}，建议先处理异常再继续推进。`,
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "section" as const,
            section: "alerts" as const,
            label: "去审批与异常",
          },
        }
      }

      if (item.value === "scim" && latestJobOfSource.deactivated > 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: "停用待清理",
          statusVariant: "secondary" as const,
          description: `最近停用了 ${latestJobOfSource.deactivated} 名员工，建议先走停用对象清理入口复核组内对象。`,
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: deactivatedDirectoryFlowLink,
            label: "去停用对象清理",
          },
        }
      }

      if (activeEmployeeCount === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: "待确认",
          statusVariant: "secondary" as const,
          description: "最近结果已完成，但目录仍为空，建议先回到员工目录复核载荷。",
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "section" as const,
            section: "employees" as const,
            label: "查看员工目录",
          },
        }
      }

      if (tenantGroups.length === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: "可推进",
          statusVariant: "outline" as const,
          description: `当前已有 ${activeEmployeeCount} 名在职员工，下一步建议先整理用户组。`,
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: directoryFlowLink,
            label: "去员工与用户组",
          },
        }
      }

      if (tenantPolicies.length === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: "可推进",
          statusVariant: "outline" as const,
          description: `当前已有 ${tenantGroups.length} 个用户组，下一步建议继续落地权限策略。`,
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: policiesFlowLink,
            label: "去权限策略",
          },
        }
      }

      if (issuedPassCount === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: "待发放",
          statusVariant: "secondary" as const,
          description: "目录、用户组和策略都已具备，下一步建议进入发放中心执行员工发放。",
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: walletFlowLink,
            label: "去凭证发放",
          },
        }
      }

      return {
        source: item.value,
        title: sourceLabel,
        statusLabel: "已闭环",
        statusVariant: "outline" as const,
        description: `当前已有 ${issuedPassCount} 张已发放凭证，可继续在发放中心做状态维护与补发。`,
        metrics,
        latestEndedAt,
        checkpoints,
        action: {
          kind: "route" as const,
          to: walletFlowLink,
          label: "查看发放中心",
        },
      }
    })
  }, [activeEmployeeCount, deactivatedDirectoryFlowLink, directoryFlowLink, issuedPassCount, policiesFlowLink, sortedSyncJobs, tenantGroups.length, tenantPolicies.length, walletFlowLink])

  useEffect(() => {
    if (!platformViewer) {
      setSelectedTenantID(viewerTenantID)
      return
    }

    const items = tenantsQuery.data ?? []
    setTenants(items)
    setSelectedTenantID((current) => current || items[0]?.id || "")
  }, [platformViewer, tenantsQuery.data, viewerTenantID])

  useEffect(() => {
    const effectiveTenantID = selectedTenantID.trim()
    if (!effectiveTenantID) {
      setEmployees([])
      setSyncJobs([])
      setApprovals([])
      setWorkerAlerts([])
      setIDPConfig(null)
      setUserGroups([])
      setPolicies([])
      setIssuedPasses([])
      return
    }

    if (!enterpriseDataQuery.data) {
      return
    }

    setEmployees(enterpriseDataQuery.data.employees)
    setSyncJobs(enterpriseDataQuery.data.syncJobs)
    setApprovals(enterpriseDataQuery.data.approvals)
    setWorkerAlerts(enterpriseDataQuery.data.workerAlerts)
    setIDPConfig(enterpriseDataQuery.data.idpConfig)
    setUserGroups(enterpriseDataQuery.data.userGroups)
    setPolicies(enterpriseDataQuery.data.policies)
    setIssuedPasses(enterpriseDataQuery.data.issuedPasses)
  }, [enterpriseDataQuery.data, selectedTenantID])

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

  async function onSyncEmployees(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!writable || !selectedTenantID.trim()) {
      return
    }

    let employeesToSync: EmployeeSyncInput[]
    try {
      const parsed = JSON.parse(syncPayload) as EmployeeSyncInput[]
      employeesToSync = Array.isArray(parsed) ? parsed : []
    } catch {
      setError("同步载荷必须是合法的 JSON 数组")
      return
    }

    if (employeesToSync.length === 0) {
      setError("请至少提供 1 条员工记录")
      return
    }

    setSyncing(true)
    setError("")
    setSyncSummary("")
    try {
      const result = await syncEnterpriseEmployees(token, {
        tenant_id: selectedTenantID.trim(),
        source: syncSource,
        actor: viewer.email,
        request_id: syncRequestID.trim() || undefined,
        employees: employeesToSync,
      })
      setSyncSummary(
        `同步任务 ${result.job.id} 已提交，创建 ${result.job.created}，更新 ${result.job.updated}，权限同步创建 ${result.access_sync.created}。`
      )
      setSyncRequestID(`sync-${Date.now()}`)
      const [employeeItems, syncJobItems] = await Promise.all([
        listEnterpriseEmployees(token, selectedTenantID.trim()),
        listEnterpriseSyncJobs(token, selectedTenantID.trim()),
      ])
      setEmployees(employeeItems)
      setSyncJobs(syncJobItems)
      const followUp =
        result.job.rejected > 0
          ? syncSourceFailureHint(syncSource, result.job.rejected)
          : employeeItems.length === 0
            ? "同步任务已提交，但当前目录仍为空，建议先回到员工目录确认结果。"
            : syncSourceSuccessHint(syncSource, result.job.deactivated)
      setSyncSummary(
        `同步任务 ${result.job.id} 已提交，创建 ${result.job.created}，更新 ${result.job.updated}，权限同步创建 ${result.access_sync.created}。${followUp}`
      )
    } catch (err) {
      const message = err instanceof Error ? err.message : "提交员工同步失败"
      setError(message)
    } finally {
      setSyncing(false)
    }
  }

  async function onReviewApproval(approvalID: string, decision: "approved" | "rejected") {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || approvalActionID) {
      return
    }

    setApprovalActionID(approvalID)
    setError("")
    try {
      await reviewEnterpriseJITProvisionApproval(token, approvalID, {
        tenant_id: tenantID,
        decision,
        reviewed_by: viewer.email,
      })
      await reloadEnterpriseData(tenantID)
      setSyncSummary(`审批 ${approvalID} 已${decision === "approved" ? "批准" : "拒绝"}，并已刷新当前企业目录状态。`)
    } catch (err) {
      const message = err instanceof Error ? err.message : "更新审批状态失败"
      setError(message)
    } finally {
      setApprovalActionID(null)
    }
  }

  async function onUpdateApprovalExternalSync(approvalID: string, status: "synced" | "failed") {
    const tenantID = selectedTenantID.trim()
    if (!writable || !tenantID || approvalActionID) {
      return
    }

    setApprovalActionID(approvalID)
    setError("")
    try {
      await updateEnterpriseJITProvisionApprovalExternalSync(token, approvalID, {
        tenant_id: tenantID,
        status,
        external_sync_ref: `web-admin-${Date.now()}`,
        last_error: status === "failed" ? "manually marked as failed from web-admin" : "",
      })
      await reloadEnterpriseData(tenantID)
      setSyncSummary(`审批 ${approvalID} 外部回写已标记为 ${status === "synced" ? "synced" : "failed"}。`)
    } catch (err) {
      const message = err instanceof Error ? err.message : "更新外部回写状态失败"
      setError(message)
    } finally {
      setApprovalActionID(null)
    }
  }

  async function onBatchReviewApprovals(approvalIDs: string[], decision: "approved" | "rejected") {
    const tenantID = selectedTenantID.trim()
    const uniqueApprovalIDs = Array.from(new Set(approvalIDs.map((item) => item.trim()).filter(Boolean)))
    if (!writable || !tenantID || approvalActionID || uniqueApprovalIDs.length === 0) {
      return
    }

    setApprovalActionID(`batch-review-${decision}-${Date.now()}`)
    setError("")
    let successCount = 0
    let failedCount = 0

    for (const approvalID of uniqueApprovalIDs) {
      try {
        await reviewEnterpriseJITProvisionApproval(token, approvalID, {
          tenant_id: tenantID,
          decision,
          reviewed_by: viewer.email,
        })
        successCount += 1
      } catch {
        failedCount += 1
      }
    }

    try {
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        `批量${decision === "approved" ? "批准" : "拒绝"}完成：成功 ${successCount} 条，失败 ${failedCount} 条。`
      )
      if (failedCount > 0) {
        setError("部分审批记录处理失败，请在台账中复核失败项后重试。")
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "批量刷新企业数据失败"
      setError(message)
    } finally {
      setApprovalActionID(null)
    }
  }

  async function onBatchUpdateApprovalExternalSync(approvalIDs: string[], status: "synced" | "failed") {
    const tenantID = selectedTenantID.trim()
    const uniqueApprovalIDs = Array.from(new Set(approvalIDs.map((item) => item.trim()).filter(Boolean)))
    if (!writable || !tenantID || approvalActionID || uniqueApprovalIDs.length === 0) {
      return
    }

    setApprovalActionID(`batch-external-sync-${status}-${Date.now()}`)
    setError("")
    let successCount = 0
    let failedCount = 0

    for (let index = 0; index < uniqueApprovalIDs.length; index += 1) {
      const approvalID = uniqueApprovalIDs[index]
      try {
        await updateEnterpriseJITProvisionApprovalExternalSync(token, approvalID, {
          tenant_id: tenantID,
          status,
          external_sync_ref: `web-admin-batch-${Date.now()}-${index + 1}`,
          last_error: status === "failed" ? "manually marked as failed from web-admin batch action" : "",
        })
        successCount += 1
      } catch {
        failedCount += 1
      }
    }

    try {
      await reloadEnterpriseData(tenantID)
      setSyncSummary(
        `批量更新外部回写状态完成（${status}）：成功 ${successCount} 条，失败 ${failedCount} 条。`
      )
      if (failedCount > 0) {
        setError("部分外部回写状态更新失败，请在台账中复核失败项后重试。")
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "批量刷新企业数据失败"
      setError(message)
    } finally {
      setApprovalActionID(null)
    }
  }

  function goToSection(section: EnterpriseSection) {
    navigate({
      pathname: location.pathname,
      hash: `#${section}`,
    })
  }
  const effectiveError = error || queryError

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div className="space-y-1">
          <p className="mp-page-eyebrow">企业目录与集成</p>
          <h1 className="mp-page-title">员工、同步与企业登录</h1>
          <p className="mp-page-description">
            先建立员工目录与企业登录，再把用户组、权限和 MistyPass 发放串成完整主路径。
          </p>
        </div>

        {platformViewer ? (
          <div className="w-full lg:w-[360px]">
            <Select value={selectedTenantID} onValueChange={setSelectedTenantID}>
              <SelectTrigger>
                <SelectValue placeholder="选择企业租户" />
              </SelectTrigger>
              <SelectContent>
                {tenants.map((item) => (
                  <SelectItem key={item.id} value={item.id}>
                    {item.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        ) : (
          <Badge variant="outline" className="w-fit rounded-full px-3 py-1">
            {selectedTenant?.name || selectedTenantID || "当前组织"}
          </Badge>
        )}
      </div>

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>员工目录</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : activeEmployeeCount}
              <UsersRoundIcon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">用户组和权限分配的上游数据源。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>最近同步任务</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : syncJobs.length}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">HRIS / SCIM / CSV / 手动同步统一收口。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>SSO 状态</CardDescription>
            <CardTitle className="text-2xl">{loading ? "--" : idpConfig?.status || "未配置"}</CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">企业登录与 JIT 同步策略。</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>待处理审批</CardDescription>
            <CardTitle className="flex items-center gap-2 text-2xl">
              {loading ? "--" : pendingApprovalCount}
              <ShieldCheckIcon className="size-4 text-muted-foreground" />
            </CardTitle>
          </CardHeader>
          <CardContent className="mp-kpi-note">JIT 账户开通和外部同步异常。</CardContent>
        </Card>
      </div>

      {effectiveError ? (
        <div className="rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {effectiveError}
        </div>
      ) : null}
      {syncSummary ? (
        <div className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700">
          {syncSummary}
        </div>
      ) : null}
      {attentionItems.length > 0 ? (
        <div className="grid gap-4 xl:grid-cols-2">
          {attentionItems.map((item) => (
            <Card key={item.title} className="border-amber-500/30 bg-amber-500/5">
              <CardHeader className="pb-2">
                <CardTitle className="text-base">{item.title}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <p className="text-sm text-muted-foreground">{item.description}</p>
                <Button size="sm" variant="outline" onClick={item.onClick}>
                  {item.actionLabel}
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>
      ) : null}

      <div className="grid gap-4 xl:grid-cols-4">
        {[
          {
            title: "HRIS / SCIM",
            description: "接企业员工系统，批量导入并持续同步员工档案。",
            actionLabel: "设为同步入口",
            onClick: () => {
              setSyncSource("hris")
              goToSection("sync")
            },
          },
          {
            title: "CSV / Excel",
            description: "为实施期和一次性迁移保留低门槛导入入口。",
            actionLabel: "切到 CSV 导入",
            onClick: () => {
              setSyncSource("csv_import")
              goToSection("sync")
            },
          },
          {
            title: "企业 SSO",
            description: "将企业登录、JIT 开通和目录来源统一到一个入口。",
            actionLabel: "查看企业登录",
            onClick: () => {
              goToSection("idp")
            },
          },
          {
            title: "审批与异常",
            description: "集中处理审批、失败同步和 worker 告警。",
            actionLabel: "查看待处理项",
            onClick: () => {
              goToSection("alerts")
            },
          },
        ].map((item) => (
          <Card key={item.title}>
            <CardHeader className="pb-2">
              <CardTitle className="text-base">{item.title}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <p className="text-sm text-muted-foreground">{item.description}</p>
              <Button variant="outline" size="sm" onClick={item.onClick}>
                {item.actionLabel}
                <ArrowUpRightIcon className="ml-1.5 size-4" />
              </Button>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">主路径进度</CardTitle>
            <CardDescription>把目录、用户组、策略和 MistyPass 发放连成一条连续工作流，而不是让管理员自己猜下一步。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {workflowSteps.map((step) => (
              <div
                key={step.id}
                className="flex flex-col gap-3 rounded-xl border bg-muted/10 px-4 py-3 lg:flex-row lg:items-center lg:justify-between"
              >
                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="font-medium">{step.title}</p>
                    <Badge variant={workflowStateVariant(step.state)}>{workflowStateLabel(step.state)}</Badge>
                  </div>
                  <p className="text-sm">{step.metric}</p>
                  <p className="mp-kpi-note">{step.helper}</p>
                </div>
                {step.id === "sync" ? (
                  <Button size="sm" variant="outline" onClick={() => goToSection(activeEmployeeCount > 0 ? "employees" : "sync")}>
                    {step.actionLabel}
                  </Button>
                ) : step.id === "directory" ? (
                  <Button asChild size="sm" variant="outline">
                    <Link to={directoryFlowLink}>{step.actionLabel}</Link>
                  </Button>
                ) : step.id === "policies" ? (
                  <Button asChild size="sm" variant="outline">
                    <Link to={policiesFlowLink}>{step.actionLabel}</Link>
                  </Button>
                ) : (
                  <Button asChild size="sm" variant="outline">
                    <Link to={walletFlowLink}>{step.actionLabel}</Link>
                  </Button>
                )}
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">当前建议动作</CardTitle>
            <CardDescription>按当前组织的真实准备度，直接给出下一步，不再固定写死三个入口。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <p className="font-medium">{nextWorkflowAction.title}</p>
              <p className="mt-1 text-sm text-muted-foreground">{nextWorkflowAction.description}</p>
            </div>

            <div className="grid gap-3 sm:grid-cols-3 xl:grid-cols-1">
              <div className="rounded-lg border bg-muted/10 px-3 py-2">
                <p className="mp-kpi-note">用户组</p>
                <p className="mt-1 text-sm font-medium">{loading ? "--" : `${tenantGroups.length} 个`}</p>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-2">
                <p className="mp-kpi-note">权限策略</p>
                <p className="mt-1 text-sm font-medium">{loading ? "--" : `${tenantPolicies.length} 条`}</p>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-2">
                <p className="mp-kpi-note">已发放凭证</p>
                <p className="mt-1 text-sm font-medium">{loading ? "--" : `${issuedPassCount} 张`}</p>
              </div>
            </div>

            {nextWorkflowAction.kind === "section" ? (
              <Button className="w-full" onClick={() => goToSection(nextWorkflowAction.section)}>
                {nextWorkflowAction.label}
              </Button>
            ) : (
              <Button asChild className="w-full">
                <Link to={nextWorkflowAction.to}>{nextWorkflowAction.label}</Link>
              </Button>
            )}

            <div className="flex flex-wrap items-center gap-2">
              <Button asChild size="sm" variant="outline">
                <Link to={directoryFlowLink}>去员工与用户组</Link>
              </Button>
              <Button asChild size="sm" variant="outline">
                <Link to={policiesFlowLink}>去权限策略</Link>
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>

      <Tabs value={activeSection} onValueChange={(value) => goToSection(value as EnterpriseSection)} className="space-y-4">
        <TabsList className="grid w-full max-w-3xl grid-cols-4">
          <TabsTrigger value="employees">员工目录</TabsTrigger>
          <TabsTrigger value="sync">导入与同步</TabsTrigger>
          <TabsTrigger value="idp">企业登录</TabsTrigger>
          <TabsTrigger value="alerts">审批与异常</TabsTrigger>
        </TabsList>

        <EnterpriseEmployeesWorkspace
          directoryLink={directoryFlowLink}
          employees={employees}
          formatDateTime={formatDateTime}
          loading={loading}
          statusBadgeVariant={statusBadgeVariant}
        />

        <EnterpriseSyncWorkspace
          activeEmployeeCount={activeEmployeeCount}
          alertsLink={enterpriseAlertsFlowLink}
          directoryLink={directoryFlowLink}
          failedSyncJobCount={failedSyncJobCount}
          formatDateTime={formatDateTime}
          goToSection={goToSection}
          initialFilterContextKey={syncRouteHints.hasHints ? syncRouteHints.contextKey : undefined}
          initialFocusHint={syncRouteHints.focusHint}
          initialWorkerFilter={syncRouteHints.workerFilter}
          initialWorkerReviewStage={syncRouteHints.workerReviewStage}
          initialWorkerReviewStatus={syncRouteHints.workerReviewStatus}
          initialWorkerQuery={syncRouteHints.workerQuery}
          latestSyncJob={latestSyncJob}
          loading={loading}
          onSyncEmployees={onSyncEmployees}
          onSyncPayloadChange={setSyncPayload}
          onSyncRequestIDChange={setSyncRequestID}
          onSyncSourceChange={setSyncSource}
          sampleSyncPayload={sampleSyncPayload}
          sortedSyncJobs={sortedSyncJobs}
          statusBadgeVariant={statusBadgeVariant}
          syncIssueCards={syncIssueCards}
          syncOutcomeAction={syncOutcomeAction}
          syncPayload={syncPayload}
          syncRemediationSteps={syncRemediationSteps}
          syncRequestID={syncRequestID}
          syncSource={syncSource}
          syncSourceFeedback={syncSourceFeedback}
          syncSourceStatusCards={syncSourceStatusCards}
          syncSourceLabel={syncSourceLabel}
          syncSourceOptions={syncSourceOptions}
          syncing={syncing}
          tenantGroupsCount={tenantGroups.length}
          tenantPoliciesCount={tenantPolicies.length}
          issuedPassCount={issuedPassCount}
          policiesLink={policiesFlowLink}
          selectedTenantName={selectedTenant?.name}
          syncLink={enterpriseSyncFlowLink}
          workflowStateLabel={workflowStateLabel}
          workflowStateVariant={workflowStateVariant}
          workerAlerts={workerAlerts}
          walletLink={walletFlowLink}
          writable={writable}
        />

        <EnterpriseIDPWorkspace
          activeEmployeeCount={activeEmployeeCount}
          directoryLink={directoryFlowLink}
          failedSyncJobCount={failedSyncJobCount}
          formatDateTime={formatDateTime}
          goToSection={goToSection}
          idpConfig={idpConfig}
          idpOutcomeAction={idpOutcomeAction}
          idpReady={idpReady}
          loading={loading}
          pendingApprovalCount={pendingApprovalCount}
          policiesLink={policiesFlowLink}
          syncJobsCount={syncJobs.length}
          workerAlertCount={workerAlertCount}
        />

        <EnterpriseAlertsWorkspace
          alertRecoveryAction={alertRecoveryAction}
          approvals={approvals}
          approvalActionID={approvalActionID}
          approvalActionBusy={approvalActionID !== null}
          attentionItems={attentionItems}
          directoryLink={directoryFlowLink}
          formatDateTime={formatDateTime}
          goToSection={goToSection}
          landingCards={alertLandingCards}
          loading={loading}
          onBatchReviewApprovals={onBatchReviewApprovals}
          onBatchUpdateApprovalExternalSync={onBatchUpdateApprovalExternalSync}
          onReviewApproval={onReviewApproval}
          onUpdateApprovalExternalSync={onUpdateApprovalExternalSync}
          syncLink={enterpriseSyncFlowLink}
          initialFilterContextKey={alertsRouteHints.hasHints ? alertsRouteHints.contextKey : undefined}
          initialLandingView={alertsRouteHints.landingView}
          initialApprovalQuery={alertsRouteHints.approvalQuery}
          initialDirectoryQuery={alertsRouteHints.directoryQuery}
          initialSegmentHint={alertsRouteHints.segmentHint}
          initialSegmentStatus={alertsRouteHints.segmentStatus}
          initialWorkerFilter={alertsRouteHints.workerFilter}
          initialSyncSourceFilter={alertsRouteHints.syncSource}
          initialSyncStatusFilter={alertsRouteHints.syncStatus}
          policiesLink={policiesFlowLink}
          selectedTenantName={selectedTenant?.name}
          statusBadgeVariant={statusBadgeVariant}
          syncJobs={syncJobs}
          walletLink={walletFlowLink}
          workerAlerts={workerAlerts}
        />
      </Tabs>
    </div>
  )
}
