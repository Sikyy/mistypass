import { useEffect, useMemo, useState, type FormEvent } from "react"
import { RefreshCwIcon } from "lucide-react"
import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { TabsContent } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { type EnterpriseSyncJob, type EnterpriseSyncWorkerAlertSummaryItem } from "@/lib/api"

// smoke-contract markers: /access/directory /access/policies /wallet?scenario=employee_mobile

type EnterpriseSection = "employees" | "sync" | "idp" | "alerts"
type EnterpriseWorkflowState = "completed" | "pending" | "blocked"
type SyncFocusHint = "worker_alert"
type WorkerReviewStatusHint = "handled"
type WorkerReviewStageHint = "alerts" | "directory" | "policies" | "issuance" | "sync"
type MainflowSegmentHint = "directory_usage" | "policy_delivery" | "issuance_receipt"
type MainflowSegmentStatus = "pending" | "attention" | "ready"
type MainflowPriorityAction = {
  description: string
  kind: "section" | "route"
  label: string
  section?: EnterpriseSection
  title: string
  to?: string
}

type EnterpriseSyncAction = {
  description: string
  kind: "section" | "route"
  label: string
  section?: EnterpriseSection
  title: string
  to?: string
}

type EnterpriseSyncIssueCard = {
  description: string
  kind: "section" | "route"
  label: string
  section?: EnterpriseSection
  title: string
  to?: string
}

type EnterpriseSyncRemediationStep = {
  description: string
  kind: "section" | "route"
  label: string
  section?: EnterpriseSection
  state: EnterpriseWorkflowState
  title: string
  to?: string
}

type EnterpriseSyncSourceOption = {
  description: string
  label: string
  value: string
}

type EnterpriseSyncFeedbackAction = {
  kind: "section" | "route"
  label: string
  section?: EnterpriseSection
  to?: string
}

type EnterpriseSyncSourceFeedback = {
  action: EnterpriseSyncFeedbackAction
  checkpoints: string[]
  description: string
  statusLabel: string
  statusVariant: "outline" | "secondary" | "destructive"
  title: string
}

type EnterpriseSyncSourceStatusCard = {
  action: EnterpriseSyncFeedbackAction
  checkpoints: string[]
  description: string
  latestEndedAt: string
  metrics: string
  source: string
  statusLabel: string
  statusVariant: "outline" | "secondary" | "destructive"
  title: string
}

type EnterpriseSyncWorkspaceProps = {
  activeEmployeeCount: number
  alertsLink: string
  directoryLink: string
  failedSyncJobCount: number
  formatDateTime: (value?: string) => string
  goToSection: (section: EnterpriseSection) => void
  initialFilterContextKey?: string
  initialFocusHint?: SyncFocusHint
  initialWorkerFilter?: "all" | "alerting" | "hot" | "stable"
  initialWorkerQuery?: string
  initialWorkerReviewStage?: WorkerReviewStageHint
  initialWorkerReviewStatus?: WorkerReviewStatusHint
  latestSyncJob: EnterpriseSyncJob | null
  loading: boolean
  onSyncEmployees: (event: FormEvent<HTMLFormElement>) => void
  onSyncPayloadChange: (value: string) => void
  onSyncRequestIDChange: (value: string) => void
  onSyncSourceChange: (value: string) => void
  sampleSyncPayload: string
  sortedSyncJobs: EnterpriseSyncJob[]
  statusBadgeVariant: (status?: string) => "outline" | "secondary" | "destructive"
  syncIssueCards: EnterpriseSyncIssueCard[]
  syncOutcomeAction: EnterpriseSyncAction
  syncPayload: string
  syncRemediationSteps: EnterpriseSyncRemediationStep[]
  syncRequestID: string
  syncSource: string
  syncSourceLabel: (source?: string) => string
  syncSourceOptions: EnterpriseSyncSourceOption[]
  syncing: boolean
  tenantGroupsCount: number
  tenantPoliciesCount: number
  issuedPassCount: number
  policiesLink: string
  selectedTenantName?: string
  syncLink: string
  syncSourceFeedback: EnterpriseSyncSourceFeedback
  syncSourceStatusCards: EnterpriseSyncSourceStatusCard[]
  workerAlerts: EnterpriseSyncWorkerAlertSummaryItem[]
  walletLink: string
  workflowStateLabel: (state: EnterpriseWorkflowState) => string
  workflowStateVariant: (state: EnterpriseWorkflowState) => "outline" | "secondary"
  writable: boolean
}

export function EnterpriseSyncWorkspace({
  activeEmployeeCount,
  alertsLink,
  directoryLink,
  failedSyncJobCount,
  formatDateTime,
  goToSection,
  initialFilterContextKey,
  initialFocusHint,
  initialWorkerFilter,
  initialWorkerQuery,
  initialWorkerReviewStage,
  initialWorkerReviewStatus,
  latestSyncJob,
  loading,
  onSyncEmployees,
  onSyncPayloadChange,
  onSyncRequestIDChange,
  onSyncSourceChange,
  sampleSyncPayload,
  sortedSyncJobs,
  statusBadgeVariant,
  syncIssueCards,
  syncOutcomeAction,
  syncPayload,
  syncRemediationSteps,
  syncRequestID,
  syncSource,
  syncSourceLabel,
  syncSourceOptions,
  syncing,
  tenantGroupsCount,
  tenantPoliciesCount,
  issuedPassCount,
  policiesLink,
  selectedTenantName,
  syncLink,
  syncSourceFeedback,
  syncSourceStatusCards,
  workerAlerts,
  walletLink,
  workflowStateLabel,
  workflowStateVariant,
  writable,
}: EnterpriseSyncWorkspaceProps) {
  const latestSyncReady = Boolean(latestSyncJob && latestSyncJob.status === "completed" && latestSyncJob.rejected === 0)
  const hasSyncFailure = Boolean(latestSyncJob && (latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0))
  const [focusHint, setFocusHint] = useState<SyncFocusHint | "">("")
  const [workerFilter, setWorkerFilter] = useState<"all" | "alerting" | "hot" | "stable">("all")
  const [workerQuery, setWorkerQuery] = useState("")
  const [workerReviewStage, setWorkerReviewStage] = useState<WorkerReviewStageHint | "">("")
  const [workerReviewStatus, setWorkerReviewStatus] = useState<WorkerReviewStatusHint | "">("")
  const [appliedInitialFilterContextKey, setAppliedInitialFilterContextKey] = useState("")

  const classifyWorkerAlert = (item: EnterpriseSyncWorkerAlertSummaryItem): "alerting" | "hot" | "stable" => {
    if (item.count === 0) {
      return "stable"
    }
    if (item.last_threshold > 0 && item.last_failed >= item.last_threshold) {
      return "hot"
    }
    return "alerting"
  }

  useEffect(() => {
    if (!initialFilterContextKey || appliedInitialFilterContextKey === initialFilterContextKey) {
      return
    }
    setFocusHint(initialFocusHint || "")
    setWorkerFilter(initialWorkerFilter || "all")
    setWorkerQuery(initialWorkerQuery?.trim() || "")
    setWorkerReviewStatus(initialWorkerReviewStatus || "")
    setWorkerReviewStage(initialWorkerReviewStage || "")
    setAppliedInitialFilterContextKey(initialFilterContextKey)
  }, [
    appliedInitialFilterContextKey,
    initialFilterContextKey,
    initialFocusHint,
    initialWorkerFilter,
    initialWorkerReviewStage,
    initialWorkerReviewStatus,
    initialWorkerQuery,
  ])

  const workerCounts = useMemo(() => {
    return {
      all: workerAlerts.length,
      alerting: workerAlerts.filter((item) => classifyWorkerAlert(item) === "alerting").length,
      hot: workerAlerts.filter((item) => classifyWorkerAlert(item) === "hot").length,
      stable: workerAlerts.filter((item) => classifyWorkerAlert(item) === "stable").length,
    }
  }, [workerAlerts])

  const filteredWorkerAlerts = useMemo(() => {
    const normalizedQuery = workerQuery.trim().toLowerCase()
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
  }, [workerAlerts, workerFilter, workerQuery])

  const workerReviewStageLabel = useMemo(() => {
    switch (workerReviewStage) {
      case "alerts":
        return "审批与异常"
      case "directory":
        return "员工与用户组"
      case "policies":
        return "权限策略"
      case "issuance":
        return "凭证发放"
      case "sync":
        return "导入与同步"
      default:
        return "当前工作区"
    }
  }, [workerReviewStage])

  const mainflowSegmentLabel = (hint: MainflowSegmentHint) => {
    switch (hint) {
      case "directory_usage":
        return "同步结果到用户组使用"
      case "policy_delivery":
        return "用户组使用到权限下发"
      case "issuance_receipt":
        return "策略下发到发放执行与回执"
      default:
        return hint
    }
  }

  const mainflowSegmentStatusLabel = (status: MainflowSegmentStatus) => {
    switch (status) {
      case "ready":
        return "已承接"
      case "attention":
        return "待收口"
      case "pending":
      default:
        return "待补齐"
    }
  }

  const mainflowSegmentStatusVariant = (status: MainflowSegmentStatus): "outline" | "secondary" | "destructive" => {
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

  type SyncNavAction = {
    kind: "section" | "route"
    label: string
    section?: EnterpriseSection
    to?: string
  }

  const toSyncNavAction = (action: SyncNavAction): SyncNavAction => {
    return {
      kind: action.kind,
      label: action.label,
      section: action.section,
      to: action.to,
    }
  }

  const sameNavAction = (left: SyncNavAction, right: SyncNavAction): boolean => {
    if (left.kind !== right.kind) {
      return false
    }
    if (left.kind === "section") {
      return (left.section || "") === (right.section || "")
    }
    return (left.to || "") === (right.to || "")
  }

  const renderNavAction = (
    action: SyncNavAction,
    options?: {
      label?: string
      variant?: "default" | "outline"
    }
  ) => {
    const label = options?.label || action.label
    const variant = options?.variant || "outline"
    if (action.kind === "section") {
      return (
        <Button size="sm" variant={variant} onClick={() => goToSection(action.section!)}>
          {label}
        </Button>
      )
    }
    return (
      <Button asChild size="sm" variant={variant}>
        <Link to={action.to!}>{label}</Link>
      </Button>
    )
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

  function buildWorkerAlertScopedLinks(item: EnterpriseSyncWorkerAlertSummaryItem) {
    const category = classifyWorkerAlert(item)
    const workerFilterHint = category === "hot" ? "hot" : category === "alerting" ? "alerting" : "stable"
    const remediationHint = category === "hot" ? "worker_hot_alert" : category === "alerting" ? "worker_alerting" : ""
    const workerScopeLabel = selectedTenantName || item.tenant_id

    const workerHints = {
      remediation_hint: remediationHint,
      worker_alert_failed: String(item.last_failed),
      worker_alert_last_seen: item.last_seen_at || "",
      worker_alert_level: category,
      worker_alert_tenant_id: item.tenant_id,
      worker_alert_threshold: String(item.last_threshold),
      worker_filter_hint: workerFilterHint,
      worker_query_hint: item.tenant_id,
    }

    return {
      alerts: withRouteHints(alertsLink, {
        alerts_view_hint: "directory_exceptions",
        ...workerHints,
      }),
      directory: withRouteHints(directoryLink, {
        group_desc: `来源${workerScopeLabel} worker 告警`,
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: "",
        group_name: `${workerScopeLabel} Worker 告警复核`,
        ...workerHints,
      }),
      policies: withRouteHints(policiesLink, {
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: "",
        policy_group: `${workerScopeLabel} Worker 告警复核`,
        policy_name: `${workerScopeLabel} Worker 告警策略复核`,
        ...workerHints,
      }),
      wallet: withRouteHints(walletLink, {
        target_email: "",
        target_id: "",
        target_ids: "",
        target_name: "",
        template_hint: "employee",
        ...workerHints,
      }),
      sync: withRouteHints(syncLink, {
        sync_focus_hint: "worker_alert",
        ...workerHints,
      }),
    }
  }
  const pipelineSteps: Array<{
    action: EnterpriseSyncFeedbackAction
    helper: string
    ready: boolean
    title: string
  }> = [
    {
      title: "同步结果",
      ready: latestSyncReady,
      helper: latestSyncJob
        ? `${syncSourceLabel(latestSyncJob.source)} 最近结果 ${latestSyncJob.status} / rejected ${latestSyncJob.rejected}`
        : "尚未生成首批同步结果",
      action: !latestSyncJob
        ? {
            kind: "section",
            section: "sync",
            label: "去导入与同步",
          }
        : latestSyncReady
          ? {
              kind: "section",
              section: "sync",
              label: "查看同步结果",
            }
          : {
              kind: "section",
              section: "alerts",
              label: "去审批与异常",
            },
    },
    {
      title: "员工目录",
      ready: activeEmployeeCount > 0,
      helper: activeEmployeeCount > 0 ? `${activeEmployeeCount} 名在职员工` : "目录仍为空",
      action: {
        kind: "section",
        section: "employees",
        label: activeEmployeeCount > 0 ? "查看员工目录" : "去员工目录复核",
      },
    },
    {
      title: "用户组",
      ready: tenantGroupsCount > 0,
      helper: tenantGroupsCount > 0 ? `${tenantGroupsCount} 个用户组` : "尚未建立稳定用户组",
      action: {
        kind: "route",
        to: directoryLink,
        label: tenantGroupsCount > 0 ? "查看员工与用户组" : "去员工与用户组",
      },
    },
    {
      title: "权限策略",
      ready: tenantPoliciesCount > 0,
      helper: tenantPoliciesCount > 0 ? `${tenantPoliciesCount} 条策略` : "尚未落策略",
      action: {
        kind: "route",
        to: policiesLink,
        label: tenantPoliciesCount > 0 ? "查看权限策略" : "去权限策略",
      },
    },
    {
      title: "凭证发放",
      ready: issuedPassCount > 0,
      helper: issuedPassCount > 0 ? `${issuedPassCount} 张已发放凭证` : "尚未开始发放",
      action: {
        kind: "route",
        to: walletLink,
        label: issuedPassCount > 0 ? "查看发放中心" : "去凭证发放",
      },
    },
  ]
  const pendingSteps = pipelineSteps.filter((item) => !item.ready).length
  const pipelineAction =
    !latestSyncJob
      ? {
          kind: "section" as const,
          section: "sync" as const,
          label: "去导入与同步",
        }
      : !latestSyncReady
        ? {
            kind: "section" as const,
            section: "alerts" as const,
            label: "去审批与异常",
          }
        : activeEmployeeCount === 0
          ? {
              kind: "section" as const,
              section: "employees" as const,
              label: "查看员工目录",
            }
          : tenantGroupsCount === 0
            ? {
                kind: "route" as const,
                to: directoryLink,
                label: "去员工与用户组",
              }
            : tenantPoliciesCount === 0
              ? {
                  kind: "route" as const,
                  to: policiesLink,
                  label: "去权限策略",
                }
              : issuedPassCount === 0
                ? {
                    kind: "route" as const,
                    to: walletLink,
                    label: "去凭证发放",
                  }
                : {
                    kind: "route" as const,
                    to: walletLink,
                    label: "查看发放中心",
                  }
  const workerReviewAlertsLink = withRouteHints(alertsLink, {
    alerts_view_hint: "directory_exceptions",
    worker_filter_hint: workerFilter === "all" ? "" : workerFilter,
    worker_query_hint: workerQuery,
    worker_review_stage_hint: "",
    worker_review_status_hint: "",
  })
  const workerReviewResetLink = withRouteHints(syncLink, {
    sync_focus_hint: "worker_alert",
    worker_filter_hint: workerFilter === "all" ? "" : workerFilter,
    worker_query_hint: workerQuery,
    worker_review_stage_hint: "",
    worker_review_status_hint: "",
  })
  const syncReferenceSource = latestSyncJob?.source?.trim() || syncSource.trim()
  const syncReferenceJobID = latestSyncJob?.id?.trim() || ""
  const syncReferenceStatus = latestSyncJob?.status?.trim() || ""
  const directoryUsageStatus: MainflowSegmentStatus =
    activeEmployeeCount === 0 ? "pending" : tenantGroupsCount > 0 ? "ready" : "attention"
  const policyDeliveryStatus: MainflowSegmentStatus =
    tenantGroupsCount === 0 ? "pending" : tenantPoliciesCount > 0 ? "ready" : "attention"
  const issuanceReceiptStatus: MainflowSegmentStatus =
    tenantPoliciesCount === 0 ? "pending" : issuedPassCount > 0 ? "ready" : "attention"
  const mainflowSegments: Array<{
    actionLabel: string
    description: string
    hint: MainflowSegmentHint
    metric: string
    status: MainflowSegmentStatus
    to: string
  }> = [
    {
      hint: "directory_usage",
      status: directoryUsageStatus,
      metric:
        activeEmployeeCount === 0
          ? "目录尚未接通，用户组承接尚未开始。"
          : `在职员工 ${activeEmployeeCount} 名 / 用户组 ${tenantGroupsCount} 个。`,
      actionLabel: "去员工与用户组承接",
      description:
        directoryUsageStatus === "ready"
          ? "同步结果已进入用户组使用态，可继续复核策略下发。"
          : directoryUsageStatus === "attention"
            ? "已有同步结果，但用户组承接不足，需先补齐组织分组。"
            : "先完成同步并形成有效目录，再建立用户组承接关系。",
      to: withRouteHints(directoryLink, {
        group_desc: syncReferenceSource ? `来源 ${syncReferenceSource.toUpperCase()} 同步分段承接` : "来源同步分段承接",
        group_name: selectedTenantName ? `${selectedTenantName} 用户组承接` : "同步用户组承接",
        segment_hint: "directory_usage",
        segment_status_hint: directoryUsageStatus,
        sync_job_id: syncReferenceJobID,
        sync_source: syncReferenceSource,
        sync_status: syncReferenceStatus,
      }),
    },
    {
      hint: "policy_delivery",
      status: policyDeliveryStatus,
      metric:
        tenantGroupsCount === 0
          ? "尚无可承接策略下发的用户组。"
          : `用户组 ${tenantGroupsCount} 个 / 策略 ${tenantPoliciesCount} 条。`,
      actionLabel: "去权限策略承接",
      description:
        policyDeliveryStatus === "ready"
          ? "策略下发已承接用户组，可继续推进发放与回执复核。"
          : policyDeliveryStatus === "attention"
            ? "用户组已就绪，但策略下发尚未形成闭环。"
            : "先补齐用户组，再进入权限策略下发。",
      to: withRouteHints(policiesLink, {
        policy_group: selectedTenantName ? `${selectedTenantName} 用户组承接` : "同步用户组承接",
        policy_name: syncReferenceSource ? `${syncReferenceSource.toUpperCase()} 同步策略下发复核` : "同步策略下发复核",
        segment_hint: "policy_delivery",
        segment_status_hint: policyDeliveryStatus,
        sync_job_id: syncReferenceJobID,
        sync_source: syncReferenceSource,
        sync_status: syncReferenceStatus,
      }),
    },
    {
      hint: "issuance_receipt",
      status: issuanceReceiptStatus,
      metric:
        tenantPoliciesCount === 0
          ? "尚无策略下发结果，无法稳定进入发放执行。"
          : `策略 ${tenantPoliciesCount} 条 / 已发放凭证 ${issuedPassCount} 张。`,
      actionLabel: "去凭证发放承接",
      description:
        issuanceReceiptStatus === "ready"
          ? "发放执行已承接策略结果，可继续核对交付与回执状态。"
          : issuanceReceiptStatus === "attention"
            ? "策略下发已就绪，但发放执行与回执链路仍需补齐。"
            : "先补齐策略下发，再推进发放执行与回执。",
      to: withRouteHints(walletLink, {
        segment_hint: "issuance_receipt",
        segment_status_hint: issuanceReceiptStatus,
        sync_job_id: syncReferenceJobID,
        sync_source: syncReferenceSource,
        sync_status: syncReferenceStatus,
      }),
    },
  ]
  const unresolvedMainflowSegments = mainflowSegments.filter((item) => item.status !== "ready")
  const unresolvedMainflowLabel = unresolvedMainflowSegments.map((item) => mainflowSegmentLabel(item.hint)).join("、")
  const mainflowPriorityAction: MainflowPriorityAction = hasSyncFailure
    ? {
        title: "优先处理同步异常再继续主流程",
        description: "最近同步仍有失败或 rejected，建议先在审批与异常收口，再继续目录、策略和发放承接。",
        kind: "section",
        section: "alerts",
        label: "去审批与异常优先收口",
      }
    : unresolvedMainflowSegments.length === 0
      ? {
          title: "主流程分段已全部承接",
          description: "同步、用户组、策略和发放承接均已连通，可继续在发放页做回执复核与状态维护。",
          kind: "route",
          to: walletLink,
          label: "去凭证发放继续维护",
        }
      : (() => {
          const first = unresolvedMainflowSegments[0]
          return {
            title: "当前唯一优先动作",
            description: `当前待收口分段：${unresolvedMainflowLabel}。建议先处理“${mainflowSegmentLabel(first.hint)}”，避免并行跳转导致主流程再次断点。`,
            kind: "route",
            to: first.to,
            label: first.actionLabel,
          } satisfies MainflowPriorityAction
        })()
  const mainflowPriorityNavAction = toSyncNavAction(mainflowPriorityAction)
  const pipelineNavAction = toSyncNavAction(pipelineAction)
  const pipelineFallbackAction = sameNavAction(pipelineNavAction, mainflowPriorityNavAction) ? null : pipelineNavAction
  const syncOutcomeNavAction: SyncNavAction = toSyncNavAction(syncOutcomeAction)
  const syncOutcomeFallbackAction = sameNavAction(syncOutcomeNavAction, mainflowPriorityNavAction) ? null : syncOutcomeNavAction

  return (
    <TabsContent value="sync">
      <div className="grid gap-4 xl:grid-cols-[0.95fr_1.05fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">快速导入入口</CardTitle>
            <CardDescription>把 HRIS、SCIM、CSV 和手动同步做成企业页的一等能力，而不是藏在用户组提示里。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            {syncSourceOptions.map((item) => (
              <div
                key={item.value}
                className={
                  syncSource === item.value
                    ? "rounded-lg border border-primary/40 bg-primary/5 p-3"
                    : "rounded-lg border bg-muted/20 p-3"
                }
              >
                <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                  <div>
                    <p className="font-medium">{item.label}</p>
                    <p className="mt-1 text-muted-foreground">{item.description}</p>
                  </div>
                  <Button
                    size="sm"
                    variant={syncSource === item.value ? "default" : "outline"}
                    onClick={() => onSyncSourceChange(item.value)}
                  >
                    {syncSource === item.value ? "当前来源" : "设为当前来源"}
                  </Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">同步任务提交</CardTitle>
            <CardDescription>
              当前先提供 JSON 导入工作台，用于打通后端同步链路；后续再替换成正式的 HRIS / SCIM 表单。
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={onSyncEmployees}>
              <div className="grid gap-3 md:grid-cols-2">
                <Select value={syncSource} onValueChange={onSyncSourceChange}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择同步来源" />
                  </SelectTrigger>
                  <SelectContent>
                    {syncSourceOptions.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <Input
                  value={syncRequestID}
                  onChange={(event) => onSyncRequestIDChange(event.target.value)}
                  placeholder="request_id"
                />
              </div>
              <Textarea
                value={syncPayload}
                onChange={(event) => onSyncPayloadChange(event.target.value)}
                rows={14}
                placeholder={sampleSyncPayload}
                disabled={!writable}
              />
              {!writable ? (
                <p className="mp-kpi-note">
                  当前角色为只读，无法发起同步；请由企业管理员或平台管理员操作。
                </p>
              ) : null}
              <div className="flex justify-end">
                <Button type="submit" disabled={syncing || !writable}>
                  <RefreshCwIcon className={`mr-1.5 size-4 ${syncing ? "animate-spin" : ""}`} />
                  {syncing ? "提交中..." : "提交同步任务"}
                </Button>
              </div>
            </form>

            <div className="mt-4 rounded-xl border bg-muted/10 px-4 py-3">
              <div className="flex flex-wrap items-center gap-2">
                <p className="font-medium">{syncSourceFeedback.title}</p>
                <Badge variant={syncSourceFeedback.statusVariant}>{syncSourceFeedback.statusLabel}</Badge>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">{syncSourceFeedback.description}</p>

              <div className="mt-3 space-y-1.5">
                {syncSourceFeedback.checkpoints.map((item) => (
                  <p key={item} className="mp-kpi-note">
                    {item}
                  </p>
                ))}
              </div>

              <div className="mt-3">
                {renderNavAction(toSyncNavAction(syncSourceFeedback.action))}
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">同步来源状态总览</CardTitle>
          <CardDescription>并排查看各导入来源的最近结果与下一步动作，避免只围绕当前来源做局部判断。</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          {syncSourceStatusCards.map((item) => (
            <div key={item.source} className="rounded-xl border bg-muted/10 px-4 py-3">
              <div className="flex flex-wrap items-center gap-2">
                <p className="font-medium">{item.title}</p>
                <Badge variant={item.statusVariant}>{item.statusLabel}</Badge>
              </div>
              <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
              <p className="mt-2 text-xs text-muted-foreground">{item.metrics}</p>
              <p className="mt-1 text-xs text-muted-foreground">最近完成时间：{item.latestEndedAt}</p>
              <div className="mt-2 space-y-1">
                {item.checkpoints.map((checkpoint) => (
                  <p key={`${item.source}-${checkpoint}`} className="mp-kpi-note">
                    {checkpoint}
                  </p>
                ))}
              </div>
              <div className="mt-3">
                {(() => {
                  const sourceAction = toSyncNavAction(item.action)
                  const sourceActionShouldLead =
                    (sourceAction.kind === "section" &&
                      (sourceAction.section === "sync" ||
                        sourceAction.section === "alerts" ||
                        sourceAction.section === "employees")) ||
                    item.statusVariant === "destructive"
                  const primaryAction = sourceActionShouldLead ? sourceAction : mainflowPriorityNavAction
                  const fallbackAction = sameNavAction(primaryAction, sourceAction)
                    ? null
                    : sourceAction
                  return (
                    <div className="flex flex-wrap items-center gap-2">
                      {renderNavAction(primaryAction)}
                      {fallbackAction ? renderNavAction(fallbackAction, { label: "按该来源处理", variant: "outline" }) : null}
                    </div>
                  )
                })()}
              </div>
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">连续主流程分段状态</CardTitle>
          <CardDescription>把“同步结果到用户组使用再到权限下发”拆成分段状态，并给出统一收口提示与优先动作。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="rounded-xl border bg-muted/10 px-4 py-3">
            <div className="flex flex-wrap items-center gap-2">
              <p className="font-medium">{mainflowPriorityAction.title}</p>
              <Badge variant={unresolvedMainflowSegments.length > 0 ? "secondary" : "outline"}>
                {unresolvedMainflowSegments.length > 0 ? `${unresolvedMainflowSegments.length} 个分段待收口` : "分段已连通"}
              </Badge>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">{mainflowPriorityAction.description}</p>
              <div className="mt-3">
                {renderNavAction(mainflowPriorityNavAction, { variant: "default" })}
              </div>
            </div>
          <div className="grid gap-3 md:grid-cols-2">
            {mainflowSegments.map((item) => {
              const unresolvedRank =
                item.status === "ready" ? 0 : unresolvedMainflowSegments.findIndex((segment) => segment.hint === item.hint) + 1
              return (
                <div key={item.hint} className="rounded-xl border bg-muted/10 px-4 py-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="font-medium">{mainflowSegmentLabel(item.hint)}</p>
                    <Badge variant={mainflowSegmentStatusVariant(item.status)}>{mainflowSegmentStatusLabel(item.status)}</Badge>
                    {unresolvedRank > 0 ? <Badge variant="secondary">{`优先级 #${unresolvedRank}`}</Badge> : null}
                  </div>
                  <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
                  <p className="mt-1 text-xs text-muted-foreground">{item.metric}</p>
                  <div className="mt-3">
                    <Button asChild size="sm" variant="outline">
                      <Link to={item.to}>{item.actionLabel}</Link>
                    </Button>
                  </div>
                </div>
              )
            })}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Worker 告警记录定位</CardTitle>
          <CardDescription>在导入与同步工作区直接承接 worker 告警线索，并给出处理后回流到目录、策略和发放的动作。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
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
          <div className="flex items-center gap-2">
            <Input
              value={workerQuery}
              onChange={(event) => setWorkerQuery(event.target.value)}
              placeholder="按租户 / failed / threshold 筛选"
              className="h-8"
            />
            {workerQuery.trim() ? (
              <Button size="sm" variant="outline" onClick={() => setWorkerQuery("")}>
                清空
              </Button>
            ) : null}
          </div>
          {focusHint === "worker_alert" ? (
            <div className="rounded-lg border bg-muted/10 px-3 py-2 text-xs text-muted-foreground">
              已按 worker 告警线索定位到导入与同步工作区，可先确认告警级别与最近失败量，再回流目录、策略或发放。
            </div>
          ) : null}
          {workerReviewStatus === "handled" ? (
            <div
              className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-2"
              data-testid="enterprise-sync-worker-review"
            >
              <p className="text-xs text-emerald-700">
                已从{workerReviewStageLabel}回流到导入与同步。建议先去审批与异常做二次复核，再确认 worker 告警是否持续下降。
              </p>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Button asChild size="sm" variant="outline">
                  <Link to={workerReviewAlertsLink} data-testid="enterprise-sync-worker-review-alerts-link">
                    去审批与异常二次复核
                  </Link>
                </Button>
                <Button asChild size="sm" variant="outline">
                  <Link to={workerReviewResetLink}>清除本次回流状态</Link>
                </Button>
              </div>
            </div>
          ) : null}
          <div className="space-y-2">
            {filteredWorkerAlerts.slice(0, 4).map((item) => {
              const category = classifyWorkerAlert(item)
              const scopedLinks = buildWorkerAlertScopedLinks(item)
              return (
                <div key={`${item.tenant_id}-${item.last_seen_at}`} className="rounded-xl border bg-muted/10 px-4 py-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="font-medium">{selectedTenantName ?? item.tenant_id}</p>
                    <Badge variant={category === "hot" ? "destructive" : category === "alerting" ? "secondary" : "outline"}>
                      {category === "hot" ? "超阈值" : category === "alerting" ? "告警中" : "稳定"}
                    </Badge>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">
                    failed {item.last_failed} / threshold {item.last_threshold} / last seen {formatDateTime(item.last_seen_at)}
                  </p>
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <Button asChild size="sm" variant="outline">
                      <Link to={scopedLinks.alerts}>去审批与异常处理</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={scopedLinks.directory}>处理后去员工与用户组</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={scopedLinks.policies}>处理后去权限策略</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={scopedLinks.wallet}>处理后去凭证发放</Link>
                    </Button>
                  </div>
                </div>
              )
            })}
            {!loading && filteredWorkerAlerts.length === 0 ? (
              <div className="rounded-xl border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
                当前筛选下没有 worker 告警记录。
              </div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[1.05fr_0.95fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">同步结果与下一步</CardTitle>
            <CardDescription>同步完成后不该停在任务提交，而应直接进入目录复核、用户组、策略或异常处理。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <p className="font-medium">
                  {latestSyncJob ? `${syncSourceLabel(latestSyncJob.source)} 最近结果` : "还没有同步结果"}
                </p>
                {latestSyncJob ? (
                  <Badge variant={statusBadgeVariant(latestSyncJob.status)}>{latestSyncJob.status}</Badge>
                ) : (
                  <Badge variant="secondary">待导入</Badge>
                )}
              </div>
              <p className="mt-2 text-sm text-muted-foreground">
                {latestSyncJob
                  ? `最近一次同步创建 ${latestSyncJob.created}，更新 ${latestSyncJob.updated}，停用 ${latestSyncJob.deactivated}，rejected ${latestSyncJob.rejected}。`
                  : "先选择一个同步来源并提交首批员工目录，后续状态会在这里直接汇总。"}
              </p>
              {latestSyncJob ? (
                <p className="mt-1 text-xs text-muted-foreground">
                  开始于 {formatDateTime(latestSyncJob.started_at)}，结束于 {formatDateTime(latestSyncJob.ended_at)}。
                </p>
              ) : null}
            </div>

            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">目录结果</p>
                <p className="mt-1 text-sm font-medium">{loading ? "--" : `${activeEmployeeCount} 名在职员工`}</p>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">待复核</p>
                <p className="mt-1 text-sm font-medium">{loading ? "--" : `${failedSyncJobCount} 条同步异常`}</p>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">后续承接</p>
                <p className="mt-1 text-sm font-medium">
                  {loading ? "--" : `${tenantGroupsCount} 个组 / ${tenantPoliciesCount} 条策略`}
                </p>
              </div>
            </div>

            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <p className="font-medium">{syncOutcomeAction.title}</p>
              <p className="mt-1 text-sm text-muted-foreground">{syncOutcomeAction.description}</p>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                {renderNavAction(mainflowPriorityNavAction, { variant: "default" })}
                {syncOutcomeFallbackAction ? renderNavAction(syncOutcomeFallbackAction, { label: "按同步结果处理", variant: "outline" }) : null}
              </div>
              <p className="mt-2 text-xs text-muted-foreground">动作已与“连续主流程分段状态”的优先级保持一致。</p>
            </div>

            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <div className="flex flex-wrap items-center gap-2">
                <p className="font-medium">目录到策略主流程连通检查</p>
                <Badge variant={hasSyncFailure ? "destructive" : pendingSteps > 0 ? "secondary" : "outline"}>
                  {hasSyncFailure ? "同步异常待处理" : pendingSteps > 0 ? `${pendingSteps} 个步骤待补` : "主流程已连通"}
                </Badge>
              </div>

              <div className="mt-3 grid gap-2 md:grid-cols-2">
                {pipelineSteps.map((item) => {
                  const stepAction = toSyncNavAction(item.action)
                  const primaryAction = item.ready ? stepAction : mainflowPriorityNavAction
                  const fallbackAction = item.ready || sameNavAction(primaryAction, stepAction) ? null : stepAction
                  return (
                    <div key={item.title} className="rounded-lg border bg-background px-3 py-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="text-sm font-medium">{item.title}</p>
                        <Badge variant={item.ready ? "outline" : "secondary"}>{item.ready ? "已就绪" : "待补齐"}</Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">{item.helper}</p>
                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        {renderNavAction(primaryAction, { variant: item.ready ? "outline" : "default" })}
                        {fallbackAction ? renderNavAction(fallbackAction, { label: "按本步骤处理", variant: "outline" }) : null}
                      </div>
                    </div>
                  )
                })}
              </div>

              <div className="mt-3 flex flex-wrap items-center gap-2">
                {renderNavAction(mainflowPriorityNavAction, { variant: "default" })}
                {pipelineFallbackAction ? renderNavAction(pipelineFallbackAction, { label: "按连通检查处理", variant: "outline" }) : null}
              </div>
              <p className="mt-2 text-xs text-muted-foreground">连通检查动作已与分段优先动作保持一致。</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">最近同步结果</CardTitle>
            <CardDescription>直接复核最近任务的来源、增量结果和失败情况，不用跳去异常页才看得到。</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {!loading && sortedSyncJobs.length === 0 ? (
              <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm text-muted-foreground">
                当前还没有同步记录。建议先选择导入来源并提交首批员工目录。
              </div>
            ) : null}
            {sortedSyncJobs.slice(0, 4).map((item) => (
              <div key={item.id} className="rounded-xl border bg-muted/10 px-4 py-3">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <p className="font-medium">{syncSourceLabel(item.source)}</p>
                    <p className="mt-1 text-xs text-muted-foreground">{item.id}</p>
                  </div>
                  <Badge variant={statusBadgeVariant(item.status)}>{item.status}</Badge>
                </div>
                <p className="mt-2 text-sm text-muted-foreground">
                  创建 {item.created} / 更新 {item.updated} / 停用 {item.deactivated} / rejected {item.rejected}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  操作人 {item.actor || "-"} / {formatDateTime(item.ended_at || item.started_at)}
                </p>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>

      {syncIssueCards.length > 0 ? (
        <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">同步异常分类</CardTitle>
              <CardDescription>把 rejected、停用和空目录风险拆开处理，避免所有问题都挤在同一个“失败”概念里。</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 md:grid-cols-2">
              {syncIssueCards.map((item) => (
                <div key={item.title} className="rounded-xl border bg-muted/10 px-4 py-3">
                  <p className="font-medium">{item.title}</p>
                  <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
                  <div className="mt-3">
                    {item.kind === "section" ? (
                      <Button size="sm" variant="outline" onClick={() => goToSection(item.section!)}>
                        {item.label}
                      </Button>
                    ) : (
                      <Button asChild size="sm" variant="outline">
                        <Link to={item.to!}>{item.label}</Link>
                      </Button>
                    )}
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-base">同步处理闭环</CardTitle>
              <CardDescription>把“看到异常”继续推进到“下一步去哪里处理、处理后怎么回到目录主路径”。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {syncRemediationSteps.map((item, index) => {
                const stepAction = toSyncNavAction(item)
                const followPriority = item.state === "pending" || item.state === "blocked"
                const primaryAction = followPriority ? mainflowPriorityNavAction : stepAction
                const fallbackAction = followPriority && !sameNavAction(primaryAction, stepAction) ? stepAction : null
                return (
                  <div key={item.title} className="rounded-xl border bg-muted/10 px-4 py-3">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="font-medium">
                        {index + 1}. {item.title}
                      </p>
                      <Badge variant={workflowStateVariant(item.state)}>{workflowStateLabel(item.state)}</Badge>
                    </div>
                    <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
                    <div className="mt-3 flex flex-wrap items-center gap-2">
                      {renderNavAction(primaryAction, { variant: followPriority ? "default" : "outline" })}
                      {fallbackAction ? renderNavAction(fallbackAction, { label: "按本步骤处理", variant: "outline" }) : null}
                    </div>
                  </div>
                )
              })}
              <p className="mp-kpi-note">处理闭环中的未完成步骤会优先跟随分段收口动作。</p>
            </CardContent>
          </Card>
        </div>
      ) : null}
    </TabsContent>
  )
}
