import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect, useMemo, useState } from "react"
import type { TFunction } from "i18next"
import { useMutation } from "@tanstack/react-query"
import { CheckCircleIcon, RefreshCwIcon, XCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { z } from "zod"

import {
  EnterpriseHRISConnectorPanel,
  type TalentaConnectorSaveInput,
} from "@/components/enterprise/enterprise-hris-connector-panel"
import {
  classifyEnterpriseSyncWorkerAlertLevel,
  describeEnterpriseSyncWorkerAlertGuidance,
} from "@/components/enterprise/enterprise-sync-worker-alert-guidance"
import { useAuth } from "@/context/auth-context"
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
import {
  larkSyncUsers,
  larkBotTest,
  googleWorkspaceSyncUsers,
  type LarkSyncResult,
  type GoogleWorkspaceSyncResult,
  type EnterpriseHRISConnector,
  type EnterpriseHRISSecret,
  type EnterpriseSyncJob,
  type EnterpriseSyncWorkerAlertSummaryItem,
} from "@/lib/api"

// smoke-contract markers: /access/directory /access/policies /wallet?scenario=employee_mobile

type EnterpriseSection = "employees" | "sync" | "idp" | "scim" | "alerts"
type EnterpriseWorkflowState = "completed" | "pending" | "blocked"
type SyncFocusHint = "worker_alert"
type HRISWebhookExecutionKindHint = "all" | "receipt_process" | "dlq_replay"
type HRISWebhookExecutionModeHint = "all" | "inline" | "queued"
type HRISWebhookExecutionQueueStateHint = "all" | "ready" | "cooldown" | "in_flight" | "attempt_limit" | "terminal"
type HRISWebhookExecutionReplayScopeHint = "all" | "replayed" | "worker_required"
type HRISWebhookExecutionStatusHint = "all" | "queued" | "running" | "succeeded" | "failed"
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

function formatLifecycleToken(value?: string) {
  const normalized = (value || "").trim().toLowerCase()
  if (!normalized) {
    return ""
  }
  return normalized.replace(/_/g, " ")
}

function createEnterpriseSyncSubmitSchema(t: TFunction) {
  return z.object({
    sync_source: z
      .string()
      .trim()
      .min(1, t("enterpriseSyncWorkspace.validation.syncSourceRequired"))
      .max(32, t("enterpriseSyncWorkspace.validation.syncSourceInvalid")),
    sync_request_id: z
      .string()
      .trim()
      .max(64, t("enterpriseSyncWorkspace.validation.requestIDMax"))
      .optional()
      .or(z.literal("")),
    sync_payload: z
      .string()
      .trim()
      .min(1, t("enterpriseSyncWorkspace.validation.syncPayloadRequired"))
      .superRefine((value, context) => {
        try {
          const parsed = JSON.parse(value) as unknown
          if (!Array.isArray(parsed)) {
            context.addIssue({
              code: z.ZodIssueCode.custom,
              message: t("enterpriseSyncWorkspace.validation.syncPayloadArrayRequired"),
            })
            return
          }
          if (parsed.length === 0) {
            context.addIssue({
              code: z.ZodIssueCode.custom,
              message: t("enterpriseSyncWorkspace.validation.syncPayloadMinItems"),
            })
          }
        } catch {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            message: t("enterpriseSyncWorkspace.validation.syncPayloadInvalidJSON"),
          })
        }
      }),
  })
}

function matchesSearchTokens(values: string[], query: string) {
  const normalizedValues = values.map((value) => value.toLowerCase())
  const tokens = query
    .trim()
    .toLowerCase()
    .split(/\s+/)
    .map((value) => value.trim())
    .filter(Boolean)
  if (tokens.length === 0) {
    return true
  }
  return tokens.every((token) => normalizedValues.some((value) => value.includes(token)))
}

type EnterpriseSyncSubmitSchema = ReturnType<typeof createEnterpriseSyncSubmitSchema>
type EnterpriseSyncSubmitFormValues = z.infer<EnterpriseSyncSubmitSchema>

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
  apiBaseURL: string
  alertsLink: string
  directoryLink: string
  failedSyncJobCount: number
  formatDateTime: (value?: string) => string
  goToSection: (section: EnterpriseSection) => void
  hrisConnectors: EnterpriseHRISConnector[]
  hrisSecrets: EnterpriseHRISSecret[]
  initialFilterContextKey?: string
  initialFocusHint?: SyncFocusHint
  initialWorkerAction?: string
  initialWorkerConnectorID?: string
  initialWorkerFailureStage?: string
  initialWorkerFilter?: "all" | "alerting" | "hot" | "stable"
  initialWorkerKind?: string
  initialWorkerLabel?: string
  initialWorkerMode?: string
  initialWorkerQueueState?: string
  initialWorkerQuery?: string
  initialWorkerReplayState?: string
  initialWorkerRequestID?: string
  initialWorkerReviewStage?: WorkerReviewStageHint
  initialWorkerReviewStatus?: WorkerReviewStatusHint
  initialWorkerStatus?: string
  initialWorkerVendor?: string
  initialExecutionID?: string
  initialExecutionKind?: HRISWebhookExecutionKindHint
  initialExecutionMode?: HRISWebhookExecutionModeHint
  initialExecutionQueueState?: HRISWebhookExecutionQueueStateHint
  initialExecutionReplayScope?: HRISWebhookExecutionReplayScopeHint
  initialExecutionStatus?: HRISWebhookExecutionStatusHint
  latestSyncJob: EnterpriseSyncJob | null
  loading: boolean
  onSaveTalentaConnector: (input: TalentaConnectorSaveInput) => Promise<void>
  onSyncEmployees: (payload: { source: string; requestID: string; payload: string }) => void
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
  apiBaseURL,
  alertsLink,
  directoryLink,
  failedSyncJobCount,
  formatDateTime,
  goToSection,
  hrisConnectors,
  hrisSecrets,
  initialFilterContextKey,
  initialFocusHint,
  initialWorkerAction,
  initialWorkerConnectorID,
  initialWorkerFailureStage,
  initialWorkerFilter,
  initialWorkerKind,
  initialWorkerLabel,
  initialWorkerMode,
  initialWorkerQueueState,
  initialWorkerQuery,
  initialWorkerReplayState,
  initialWorkerRequestID,
  initialWorkerReviewStage,
  initialWorkerReviewStatus,
  initialWorkerStatus,
  initialWorkerVendor,
  initialExecutionID,
  initialExecutionKind,
  initialExecutionMode,
  initialExecutionQueueState,
  initialExecutionReplayScope,
  initialExecutionStatus,
  latestSyncJob,
  loading,
  onSaveTalentaConnector,
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
  const { t } = useTranslation()
  const latestSyncReady = Boolean(latestSyncJob && latestSyncJob.status === "completed" && latestSyncJob.rejected === 0)
  const hasSyncFailure = Boolean(latestSyncJob && (latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0))
  const [focusHint, setFocusHint] = useState<SyncFocusHint | "">("")
  const [workerActionScope, setWorkerActionScope] = useState("")
  const [workerConnectorScope, setWorkerConnectorScope] = useState("")
  const [workerFailureStageScope, setWorkerFailureStageScope] = useState("")
  const [workerFilter, setWorkerFilter] = useState<"all" | "alerting" | "hot" | "stable">("all")
  const [workerKindScope, setWorkerKindScope] = useState("")
  const [workerLabelScope, setWorkerLabelScope] = useState("")
  const [workerModeScope, setWorkerModeScope] = useState("")
  const [workerQueueStateScope, setWorkerQueueStateScope] = useState("")
  const [workerQuery, setWorkerQuery] = useState("")
  const [workerReplayStateScope, setWorkerReplayStateScope] = useState("")
  const [workerRequestScope, setWorkerRequestScope] = useState("")
  const [workerReviewStage, setWorkerReviewStage] = useState<WorkerReviewStageHint | "">("")
  const [workerReviewStatus, setWorkerReviewStatus] = useState<WorkerReviewStatusHint | "">("")
  const [workerStatusScope, setWorkerStatusScope] = useState("")
  const [workerVendorScope, setWorkerVendorScope] = useState("")
  const [executionIDScope, setExecutionIDScope] = useState("")
  const [executionKindScope, setExecutionKindScope] = useState("")
  const [executionModeScope, setExecutionModeScope] = useState("")
  const [executionQueueStateScope, setExecutionQueueStateScope] = useState("")
  const [executionReplayScopeScope, setExecutionReplayScopeScope] = useState("")
  const [executionStatusScope, setExecutionStatusScope] = useState("")
  const [appliedInitialFilterContextKey, setAppliedInitialFilterContextKey] = useState("")
  const syncSubmitSchema = useMemo(() => createEnterpriseSyncSubmitSchema(t), [t])
  const syncSubmitForm = useForm<EnterpriseSyncSubmitFormValues>({
    resolver: zodResolver(syncSubmitSchema),
    values: {
      sync_source: syncSource,
      sync_request_id: syncRequestID,
      sync_payload: syncPayload,
    },
  })
  const syncRequestIDField = syncSubmitForm.register("sync_request_id")
  const syncPayloadField = syncSubmitForm.register("sync_payload")
  const syncSubmitFormError =
    syncSubmitForm.formState.errors.sync_source?.message ||
    syncSubmitForm.formState.errors.sync_request_id?.message ||
    syncSubmitForm.formState.errors.sync_payload?.message ||
    ""

  function onSubmitSyncEmployees(values: EnterpriseSyncSubmitFormValues) {
    onSyncEmployees({
      source: values.sync_source,
      requestID: values.sync_request_id || "",
      payload: values.sync_payload,
    })
  }

  useEffect(() => {
    if (!initialFilterContextKey || appliedInitialFilterContextKey === initialFilterContextKey) {
      return
    }
    setFocusHint(initialFocusHint || "")
    setWorkerActionScope(initialWorkerAction?.trim() || "")
    setWorkerConnectorScope(initialWorkerConnectorID?.trim() || "")
    setWorkerFailureStageScope(initialWorkerFailureStage?.trim() || "")
    setWorkerFilter(initialWorkerFilter || "all")
    setWorkerKindScope(initialWorkerKind?.trim() || "")
    setWorkerLabelScope(initialWorkerLabel?.trim() || "")
    setWorkerModeScope(initialWorkerMode?.trim() || "")
    setWorkerQueueStateScope(initialWorkerQueueState?.trim() || "")
    setWorkerQuery(initialWorkerQuery?.trim() || "")
    setWorkerReplayStateScope(initialWorkerReplayState?.trim() || "")
    setWorkerRequestScope(initialWorkerRequestID?.trim() || "")
    setWorkerReviewStatus(initialWorkerReviewStatus || "")
    setWorkerReviewStage(initialWorkerReviewStage || "")
    setWorkerStatusScope(initialWorkerStatus?.trim() || "")
    setWorkerVendorScope(initialWorkerVendor?.trim() || "")
    setExecutionIDScope(initialExecutionID?.trim() || "")
    setExecutionKindScope(initialExecutionKind?.trim() || "")
    setExecutionModeScope(initialExecutionMode?.trim() || "")
    setExecutionQueueStateScope(initialExecutionQueueState?.trim() || "")
    setExecutionReplayScopeScope(initialExecutionReplayScope?.trim() || "")
    setExecutionStatusScope(initialExecutionStatus?.trim() || "")
    setAppliedInitialFilterContextKey(initialFilterContextKey)
  }, [
    appliedInitialFilterContextKey,
    initialExecutionID,
    initialExecutionKind,
    initialExecutionMode,
    initialExecutionQueueState,
    initialExecutionReplayScope,
    initialExecutionStatus,
    initialFilterContextKey,
    initialFocusHint,
    initialWorkerAction,
    initialWorkerConnectorID,
    initialWorkerFailureStage,
    initialWorkerFilter,
    initialWorkerKind,
    initialWorkerLabel,
    initialWorkerMode,
    initialWorkerQueueState,
    initialWorkerRequestID,
    initialWorkerReplayState,
    initialWorkerReviewStage,
    initialWorkerReviewStatus,
    initialWorkerQuery,
    initialWorkerStatus,
    initialWorkerVendor,
  ])

  const workerCounts = useMemo(() => {
    return {
      all: workerAlerts.length,
      alerting: workerAlerts.filter((item) => classifyEnterpriseSyncWorkerAlertLevel(item) === "alerting").length,
      hot: workerAlerts.filter((item) => classifyEnterpriseSyncWorkerAlertLevel(item) === "hot").length,
      stable: workerAlerts.filter((item) => classifyEnterpriseSyncWorkerAlertLevel(item) === "stable").length,
    }
  }, [workerAlerts])

  const filteredWorkerAlerts = useMemo(() => {
    const normalizedActionScope = workerActionScope.trim().toLowerCase()
    const normalizedKindScope = workerKindScope.trim().toLowerCase()
    const normalizedLabelScope = workerLabelScope.trim().toLowerCase()
    const normalizedQuery = workerQuery.trim()
    return workerAlerts.filter((item) => {
      const category = classifyEnterpriseSyncWorkerAlertLevel(item)
      const categoryPass = workerFilter === "all" || category === workerFilter
      const actionPass =
        normalizedActionScope.length === 0 || (item.worker_action || "").trim().toLowerCase() === normalizedActionScope
      const kindPass =
        normalizedKindScope.length === 0 || (item.worker_kind || "").trim().toLowerCase() === normalizedKindScope
      const labelPass =
        normalizedLabelScope.length === 0 || (item.worker_label || "").trim().toLowerCase() === normalizedLabelScope
      const queryPass = matchesSearchTokens(
        [
          item.tenant_id,
          item.worker_action || "",
          item.worker_kind || "",
          item.worker_label || "",
          String(item.count),
          String(item.last_failed),
          String(item.last_threshold),
          String(item.last_processed),
          String(item.last_applied),
        ],
        normalizedQuery
      )
      return categoryPass && actionPass && kindPass && labelPass && queryPass
    })
  }, [workerActionScope, workerAlerts, workerFilter, workerKindScope, workerLabelScope, workerQuery])

  const workerReviewStageLabel = useMemo(() => {
    switch (workerReviewStage) {
      case "alerts":
        return t("enterpriseSyncWorkspace.workerReview.stage.alerts")
      case "directory":
        return t("enterpriseSyncWorkspace.workerReview.stage.directory")
      case "policies":
        return t("enterpriseSyncWorkspace.workerReview.stage.policies")
      case "issuance":
        return t("enterpriseSyncWorkspace.workerReview.stage.issuance")
      case "sync":
        return t("enterpriseSyncWorkspace.workerReview.stage.sync")
      default:
        return t("enterpriseSyncWorkspace.workerReview.stage.current")
    }
  }, [t, workerReviewStage])

  const mainflowSegmentLabel = (hint: MainflowSegmentHint) => {
    switch (hint) {
      case "directory_usage":
        return t("enterpriseSyncWorkspace.mainflow.segment.directoryUsage")
      case "policy_delivery":
        return t("enterpriseSyncWorkspace.mainflow.segment.policyDelivery")
      case "issuance_receipt":
        return t("enterpriseSyncWorkspace.mainflow.segment.issuanceReceipt")
      default:
        return hint
    }
  }

  const mainflowSegmentStatusLabel = (status: MainflowSegmentStatus) => {
    switch (status) {
      case "ready":
        return t("enterpriseSyncWorkspace.mainflow.status.ready")
      case "attention":
        return t("enterpriseSyncWorkspace.mainflow.status.attention")
      case "pending":
      default:
        return t("enterpriseSyncWorkspace.mainflow.status.pending")
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

  const executionRouteHints = {
    execution_id: executionIDScope,
    execution_kind: executionKindScope,
    execution_mode: executionModeScope,
    execution_queue_state: executionQueueStateScope,
    execution_replay_scope: executionReplayScopeScope,
    execution_status: executionStatusScope,
  }

  function buildWorkerAlertScopedLinks(item: EnterpriseSyncWorkerAlertSummaryItem) {
    const category = classifyEnterpriseSyncWorkerAlertLevel(item)
    const workerFilterHint = category === "hot" ? "hot" : category === "alerting" ? "alerting" : "stable"
    const remediationHint = category === "hot" ? "worker_hot_alert" : category === "alerting" ? "worker_alerting" : ""
    const workerScopeLabel = selectedTenantName || item.tenant_id

    const workerHints = {
      worker_connector_id: workerConnectorScope,
      worker_failure_stage: workerFailureStageScope,
      remediation_hint: remediationHint,
      worker_action: item.worker_action || "",
      worker_alert_failed: String(item.last_failed),
      worker_alert_label: item.worker_label || "",
      worker_alert_last_seen: item.last_seen_at || "",
      worker_alert_level: category,
      worker_alert_tenant_id: item.tenant_id,
      worker_alert_threshold: String(item.last_threshold),
      worker_kind: item.worker_kind || "",
      worker_mode: workerModeScope,
      worker_queue_state: workerQueueStateScope,
      worker_filter_hint: workerFilterHint,
      worker_query_hint: item.tenant_id,
      worker_replay_state: workerReplayStateScope,
      worker_request_id: workerRequestScope,
      worker_status: workerStatusScope,
      worker_vendor: workerVendorScope,
    }

    return {
      alerts: withRouteHints(alertsLink, {
        alerts_view_hint: "directory_exceptions",
        ...workerHints,
        ...executionRouteHints,
      }),
      directory: withRouteHints(directoryLink, {
        group_desc: t("enterpriseSyncWorkspace.hints.workerGroupDesc", { scope: workerScopeLabel }),
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: "",
        group_name: t("enterpriseSyncWorkspace.hints.workerGroupName", { scope: workerScopeLabel }),
        ...workerHints,
      }),
      policies: withRouteHints(policiesLink, {
        group_member_email: "",
        group_member_id: "",
        group_member_name: "",
        group_member_status: "",
        policy_group: t("enterpriseSyncWorkspace.hints.workerPolicyGroup", { scope: workerScopeLabel }),
        policy_name: t("enterpriseSyncWorkspace.hints.workerPolicyName", { scope: workerScopeLabel }),
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
      title: t("enterpriseSyncWorkspace.pipeline.syncResult.title"),
      ready: latestSyncReady,
      helper: latestSyncJob
        ? t("enterpriseSyncWorkspace.pipeline.syncResult.helperWithJob", {
            source: syncSourceLabel(latestSyncJob.source),
            status: latestSyncJob.status,
            rejected: latestSyncJob.rejected,
          })
        : t("enterpriseSyncWorkspace.pipeline.syncResult.helperNoJob"),
      action: !latestSyncJob
        ? {
            kind: "section",
            section: "sync",
            label: t("enterpriseSyncWorkspace.pipeline.syncResult.actionGoSync"),
          }
        : latestSyncReady
          ? {
              kind: "section",
              section: "sync",
              label: t("enterpriseSyncWorkspace.pipeline.syncResult.actionView"),
            }
          : {
              kind: "section",
              section: "alerts",
              label: t("enterpriseSyncWorkspace.pipeline.syncResult.actionGoAlerts"),
            },
    },
    {
      title: t("enterpriseSyncWorkspace.pipeline.employeeDirectory.title"),
      ready: activeEmployeeCount > 0,
      helper:
        activeEmployeeCount > 0
          ? t("enterpriseSyncWorkspace.pipeline.employeeDirectory.helperReady", { count: activeEmployeeCount })
          : t("enterpriseSyncWorkspace.pipeline.employeeDirectory.helperEmpty"),
      action: {
        kind: "section",
        section: "employees",
        label:
          activeEmployeeCount > 0
            ? t("enterpriseSyncWorkspace.pipeline.employeeDirectory.actionView")
            : t("enterpriseSyncWorkspace.pipeline.employeeDirectory.actionReview"),
      },
    },
    {
      title: t("enterpriseSyncWorkspace.pipeline.userGroups.title"),
      ready: tenantGroupsCount > 0,
      helper:
        tenantGroupsCount > 0
          ? t("enterpriseSyncWorkspace.pipeline.userGroups.helperReady", { count: tenantGroupsCount })
          : t("enterpriseSyncWorkspace.pipeline.userGroups.helperEmpty"),
      action: {
        kind: "route",
        to: directoryLink,
        label:
          tenantGroupsCount > 0
            ? t("enterpriseSyncWorkspace.pipeline.userGroups.actionView")
            : t("enterpriseSyncWorkspace.pipeline.userGroups.actionGo"),
      },
    },
    {
      title: t("enterpriseSyncWorkspace.pipeline.policies.title"),
      ready: tenantPoliciesCount > 0,
      helper:
        tenantPoliciesCount > 0
          ? t("enterpriseSyncWorkspace.pipeline.policies.helperReady", { count: tenantPoliciesCount })
          : t("enterpriseSyncWorkspace.pipeline.policies.helperEmpty"),
      action: {
        kind: "route",
        to: policiesLink,
        label:
          tenantPoliciesCount > 0
            ? t("enterpriseSyncWorkspace.pipeline.policies.actionView")
            : t("enterpriseSyncWorkspace.pipeline.policies.actionGo"),
      },
    },
    {
      title: t("enterpriseSyncWorkspace.pipeline.issuance.title"),
      ready: issuedPassCount > 0,
      helper:
        issuedPassCount > 0
          ? t("enterpriseSyncWorkspace.pipeline.issuance.helperReady", { count: issuedPassCount })
          : t("enterpriseSyncWorkspace.pipeline.issuance.helperEmpty"),
      action: {
        kind: "route",
        to: walletLink,
        label:
          issuedPassCount > 0
            ? t("enterpriseSyncWorkspace.pipeline.issuance.actionView")
            : t("enterpriseSyncWorkspace.pipeline.issuance.actionGo"),
      },
    },
  ]
  const pendingSteps = pipelineSteps.filter((item) => !item.ready).length
  const pipelineAction =
    !latestSyncJob
      ? {
          kind: "section" as const,
          section: "sync" as const,
          label: t("enterpriseSyncWorkspace.pipeline.primaryAction.goSync"),
        }
      : !latestSyncReady
        ? {
            kind: "section" as const,
            section: "alerts" as const,
            label: t("enterpriseSyncWorkspace.pipeline.primaryAction.goAlerts"),
          }
        : activeEmployeeCount === 0
          ? {
              kind: "section" as const,
              section: "employees" as const,
              label: t("enterpriseSyncWorkspace.pipeline.primaryAction.viewEmployees"),
            }
          : tenantGroupsCount === 0
            ? {
                kind: "route" as const,
                to: directoryLink,
                label: t("enterpriseSyncWorkspace.pipeline.primaryAction.goDirectory"),
              }
            : tenantPoliciesCount === 0
              ? {
                  kind: "route" as const,
                  to: policiesLink,
                  label: t("enterpriseSyncWorkspace.pipeline.primaryAction.goPolicies"),
                }
              : issuedPassCount === 0
                ? {
                    kind: "route" as const,
                    to: walletLink,
                    label: t("enterpriseSyncWorkspace.pipeline.primaryAction.goIssuance"),
                  }
                : {
                    kind: "route" as const,
                    to: walletLink,
                    label: t("enterpriseSyncWorkspace.pipeline.primaryAction.viewIssuance"),
                  }
  const workerReviewAlertsLink = withRouteHints(alertsLink, {
    alerts_view_hint: "directory_exceptions",
    worker_action: workerActionScope,
    worker_alert_label: workerLabelScope,
    worker_connector_id: workerConnectorScope,
    worker_failure_stage: workerFailureStageScope,
    worker_filter_hint: workerFilter === "all" ? "" : workerFilter,
    worker_kind: workerKindScope,
    worker_mode: workerModeScope,
    worker_queue_state: workerQueueStateScope,
    worker_query_hint: workerQuery,
    worker_replay_state: workerReplayStateScope,
    worker_request_id: workerRequestScope,
    worker_review_stage_hint: "",
    worker_review_status_hint: "",
    worker_status: workerStatusScope,
    worker_vendor: workerVendorScope,
    ...executionRouteHints,
  })
  const workerReviewResetLink = withRouteHints(syncLink, {
    sync_focus_hint: "worker_alert",
    worker_action: workerActionScope,
    worker_alert_label: workerLabelScope,
    worker_connector_id: workerConnectorScope,
    worker_failure_stage: workerFailureStageScope,
    worker_filter_hint: workerFilter === "all" ? "" : workerFilter,
    worker_kind: workerKindScope,
    worker_mode: workerModeScope,
    worker_queue_state: workerQueueStateScope,
    worker_query_hint: workerQuery,
    worker_replay_state: workerReplayStateScope,
    worker_request_id: workerRequestScope,
    worker_review_stage_hint: "",
    worker_review_status_hint: "",
    worker_status: workerStatusScope,
    worker_vendor: workerVendorScope,
    ...executionRouteHints,
  })
  const executionReviewAlertsLink = withRouteHints(alertsLink, {
    alerts_view_hint: "directory_exceptions",
    worker_action: workerActionScope,
    worker_alert_label: workerLabelScope,
    worker_connector_id: workerConnectorScope,
    worker_failure_stage: workerFailureStageScope,
    worker_filter_hint: workerFilter === "all" ? "" : workerFilter,
    worker_kind: workerKindScope,
    worker_mode: workerModeScope,
    worker_queue_state: workerQueueStateScope,
    worker_query_hint: workerQuery,
    worker_replay_state: workerReplayStateScope,
    worker_request_id: workerRequestScope,
    worker_status: workerStatusScope,
    worker_vendor: workerVendorScope,
    ...executionRouteHints,
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
  const hasExecutionContext =
    Boolean(
      executionIDScope ||
        executionKindScope ||
        executionModeScope ||
        executionQueueStateScope ||
        executionReplayScopeScope ||
        executionStatusScope
    )
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
          ? t("enterpriseSyncWorkspace.mainflow.directoryUsage.metricPending")
          : t("enterpriseSyncWorkspace.mainflow.directoryUsage.metricReady", {
              employees: activeEmployeeCount,
              groups: tenantGroupsCount,
            }),
      actionLabel: t("enterpriseSyncWorkspace.mainflow.directoryUsage.action"),
      description:
        directoryUsageStatus === "ready"
          ? t("enterpriseSyncWorkspace.mainflow.directoryUsage.descriptionReady")
          : directoryUsageStatus === "attention"
            ? t("enterpriseSyncWorkspace.mainflow.directoryUsage.descriptionAttention")
            : t("enterpriseSyncWorkspace.mainflow.directoryUsage.descriptionPending"),
      to: withRouteHints(directoryLink, {
        group_desc: syncReferenceSource
          ? t("enterpriseSyncWorkspace.hints.segmentGroupDescWithSource", {
              source: syncReferenceSource.toUpperCase(),
            })
          : t("enterpriseSyncWorkspace.hints.segmentGroupDesc"),
        group_name: selectedTenantName
          ? t("enterpriseSyncWorkspace.hints.segmentGroupNameWithTenant", { tenant: selectedTenantName })
          : t("enterpriseSyncWorkspace.hints.segmentGroupName"),
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
          ? t("enterpriseSyncWorkspace.mainflow.policyDelivery.metricPending")
          : t("enterpriseSyncWorkspace.mainflow.policyDelivery.metricReady", {
              groups: tenantGroupsCount,
              policies: tenantPoliciesCount,
            }),
      actionLabel: t("enterpriseSyncWorkspace.mainflow.policyDelivery.action"),
      description:
        policyDeliveryStatus === "ready"
          ? t("enterpriseSyncWorkspace.mainflow.policyDelivery.descriptionReady")
          : policyDeliveryStatus === "attention"
            ? t("enterpriseSyncWorkspace.mainflow.policyDelivery.descriptionAttention")
            : t("enterpriseSyncWorkspace.mainflow.policyDelivery.descriptionPending"),
      to: withRouteHints(policiesLink, {
        policy_group: selectedTenantName
          ? t("enterpriseSyncWorkspace.hints.segmentGroupNameWithTenant", { tenant: selectedTenantName })
          : t("enterpriseSyncWorkspace.hints.segmentGroupName"),
        policy_name: syncReferenceSource
          ? t("enterpriseSyncWorkspace.hints.segmentPolicyNameWithSource", {
              source: syncReferenceSource.toUpperCase(),
            })
          : t("enterpriseSyncWorkspace.hints.segmentPolicyName"),
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
          ? t("enterpriseSyncWorkspace.mainflow.issuanceReceipt.metricPending")
          : t("enterpriseSyncWorkspace.mainflow.issuanceReceipt.metricReady", {
              policies: tenantPoliciesCount,
              issued: issuedPassCount,
            }),
      actionLabel: t("enterpriseSyncWorkspace.mainflow.issuanceReceipt.action"),
      description:
        issuanceReceiptStatus === "ready"
          ? t("enterpriseSyncWorkspace.mainflow.issuanceReceipt.descriptionReady")
          : issuanceReceiptStatus === "attention"
            ? t("enterpriseSyncWorkspace.mainflow.issuanceReceipt.descriptionAttention")
            : t("enterpriseSyncWorkspace.mainflow.issuanceReceipt.descriptionPending"),
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
        title: t("enterpriseSyncWorkspace.mainflow.priority.failure.title"),
        description: t("enterpriseSyncWorkspace.mainflow.priority.failure.description"),
        kind: "section",
        section: "alerts",
        label: t("enterpriseSyncWorkspace.mainflow.priority.failure.action"),
      }
    : unresolvedMainflowSegments.length === 0
      ? {
          title: t("enterpriseSyncWorkspace.mainflow.priority.done.title"),
          description: t("enterpriseSyncWorkspace.mainflow.priority.done.description"),
          kind: "route",
          to: walletLink,
          label: t("enterpriseSyncWorkspace.mainflow.priority.done.action"),
        }
      : (() => {
          const first = unresolvedMainflowSegments[0]
          return {
            title: t("enterpriseSyncWorkspace.mainflow.priority.pending.title"),
            description: t("enterpriseSyncWorkspace.mainflow.priority.pending.description", {
              unresolved: unresolvedMainflowLabel,
              first: mainflowSegmentLabel(first.hint),
            }),
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
            <CardTitle className="text-base">{t("enterpriseSyncWorkspace.quickImport.title")}</CardTitle>
            <CardDescription>{t("enterpriseSyncWorkspace.quickImport.description")}</CardDescription>
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
                    {syncSource === item.value
                      ? t("enterpriseSyncWorkspace.quickImport.currentSource")
                      : t("enterpriseSyncWorkspace.quickImport.setAsCurrent")}
                  </Button>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("enterpriseSyncWorkspace.submit.title")}</CardTitle>
            <CardDescription>
              {t("enterpriseSyncWorkspace.submit.description")}
            </CardDescription>
          </CardHeader>
          <CardContent>
            <form className="space-y-3" onSubmit={syncSubmitForm.handleSubmit(onSubmitSyncEmployees)}>
              <div className="grid gap-3 md:grid-cols-2">
                <Controller
                  control={syncSubmitForm.control}
                  name="sync_source"
                  render={({ field }) => (
                    <Select
                      value={field.value}
                      onValueChange={(value) => {
                        field.onChange(value)
                        onSyncSourceChange(value)
                      }}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={t("enterpriseSyncWorkspace.submit.selectSourcePlaceholder")} />
                      </SelectTrigger>
                      <SelectContent>
                        {syncSourceOptions.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                />
                <Input
                  {...syncRequestIDField}
                  onChange={(event) => {
                    syncRequestIDField.onChange(event)
                    onSyncRequestIDChange(event.target.value)
                  }}
                  placeholder="request_id"
                />
              </div>
              <Textarea
                {...syncPayloadField}
                onChange={(event) => {
                  syncPayloadField.onChange(event)
                  onSyncPayloadChange(event.target.value)
                }}
                rows={14}
                placeholder={sampleSyncPayload}
                disabled={!writable}
              />
              {!writable ? (
                <p className="mp-kpi-note">
                  {t("enterpriseSyncWorkspace.submit.readonlyHint")}
                </p>
              ) : null}
              <div className="flex justify-end">
                <Button type="submit" disabled={syncing || !writable || syncSubmitForm.formState.isSubmitting}>
                  <RefreshCwIcon className={`mr-1.5 size-4 ${syncing ? "animate-spin" : ""}`} />
                  {syncing
                    ? t("enterpriseSyncWorkspace.submit.submitting")
                    : t("enterpriseSyncWorkspace.submit.submit")}
                </Button>
              </div>
              {syncSubmitFormError ? <p className="text-sm text-destructive">{syncSubmitFormError}</p> : null}
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

      <EnterpriseHRISConnectorPanel
        apiBaseURL={apiBaseURL}
        connectors={hrisConnectors}
        loading={loading}
        onSaveTalentaConnector={onSaveTalentaConnector}
        secrets={hrisSecrets}
        selectedTenantName={selectedTenantName}
        writable={writable}
      />

      <LarkSyncSection writable={writable} />

      <GoogleWorkspaceSyncSection writable={writable} />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("enterpriseSyncWorkspace.sourceOverview.title")}</CardTitle>
          <CardDescription>{t("enterpriseSyncWorkspace.sourceOverview.description")}</CardDescription>
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
              <p className="mt-1 text-xs text-muted-foreground">
                {t("enterpriseSyncWorkspace.sourceOverview.latestEndedAt", { value: item.latestEndedAt })}
              </p>
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
                      {fallbackAction
                        ? renderNavAction(fallbackAction, {
                            label: t("enterpriseSyncWorkspace.sourceOverview.fallbackAction"),
                            variant: "outline",
                          })
                        : null}
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
          <CardTitle className="text-base">{t("enterpriseSyncWorkspace.mainflow.title")}</CardTitle>
          <CardDescription>{t("enterpriseSyncWorkspace.mainflow.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="rounded-xl border bg-muted/10 px-4 py-3">
            <div className="flex flex-wrap items-center gap-2">
              <p className="font-medium">{mainflowPriorityAction.title}</p>
              <Badge variant={unresolvedMainflowSegments.length > 0 ? "secondary" : "outline"}>
                {unresolvedMainflowSegments.length > 0
                  ? t("enterpriseSyncWorkspace.mainflow.unresolvedCount", {
                      count: unresolvedMainflowSegments.length,
                    })
                  : t("enterpriseSyncWorkspace.mainflow.connected")}
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
                    {unresolvedRank > 0 ? (
                      <Badge variant="secondary">
                        {t("enterpriseSyncWorkspace.mainflow.priorityRank", { rank: unresolvedRank })}
                      </Badge>
                    ) : null}
                  </div>
                  <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
                  <p className="mt-1 text-xs text-muted-foreground">{item.metric}</p>
                  <div className="mt-3">
                    <Button asChild size="sm" variant="outline">
                      <Link to={item.to} data-testid={`enterprise-sync-mainflow-${item.hint}-link`}>
                        {item.actionLabel}
                      </Link>
                    </Button>
                  </div>
                </div>
              )
            })}
          </div>
        </CardContent>
      </Card>

      <Card data-testid="enterprise-sync-worker-alerts-card">
        <CardHeader>
          <CardTitle className="text-base">{t("enterpriseSyncWorkspace.workerAlerts.title")}</CardTitle>
          <CardDescription>{t("enterpriseSyncWorkspace.workerAlerts.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-wrap items-center gap-2">
            {[
              { value: "all", label: t("enterpriseSyncWorkspace.workerAlerts.filters.all", { count: workerCounts.all }) },
              {
                value: "alerting",
                label: t("enterpriseSyncWorkspace.workerAlerts.filters.alerting", {
                  count: workerCounts.alerting,
                }),
              },
              { value: "hot", label: t("enterpriseSyncWorkspace.workerAlerts.filters.hot", { count: workerCounts.hot }) },
              {
                value: "stable",
                label: t("enterpriseSyncWorkspace.workerAlerts.filters.stable", {
                  count: workerCounts.stable,
                }),
              },
            ].map((item) => (
              <Button
                key={item.value}
                data-testid={`enterprise-sync-worker-alert-filter-${item.value}`}
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
              data-testid="enterprise-sync-worker-alert-query"
              value={workerQuery}
              onChange={(event) => setWorkerQuery(event.target.value)}
              placeholder={t("enterpriseSyncWorkspace.workerAlerts.queryPlaceholder")}
              className="h-8"
            />
            {workerQuery.trim() ? (
              <Button size="sm" variant="outline" onClick={() => setWorkerQuery("")}>
                {t("enterpriseSyncWorkspace.workerAlerts.clear")}
              </Button>
            ) : null}
          </div>
          {workerActionScope ||
          workerConnectorScope ||
          workerFailureStageScope ||
          workerKindScope ||
          workerLabelScope ||
          workerModeScope ||
          workerQueueStateScope ||
          executionIDScope ||
          executionKindScope ||
          executionModeScope ||
          executionQueueStateScope ||
          executionReplayScopeScope ||
          executionStatusScope ||
          workerReplayStateScope ||
          workerRequestScope ||
          workerStatusScope ||
          workerVendorScope ? (
            <div className="flex flex-wrap items-center gap-2" data-testid="enterprise-sync-worker-alert-scope">
              {workerLabelScope ? <Badge variant="secondary">{workerLabelScope}</Badge> : null}
              {workerActionScope ? <Badge variant="secondary">{workerActionScope}</Badge> : null}
              {workerKindScope ? <Badge variant="outline">{workerKindScope}</Badge> : null}
              {workerConnectorScope ? <Badge variant="outline">{workerConnectorScope}</Badge> : null}
              {workerRequestScope ? <Badge variant="outline">{workerRequestScope}</Badge> : null}
              {workerFailureStageScope ? <Badge variant="outline">{workerFailureStageScope}</Badge> : null}
              {workerVendorScope ? <Badge variant="outline">{workerVendorScope}</Badge> : null}
              {workerModeScope ? <Badge variant="outline">{workerModeScope}</Badge> : null}
              {workerStatusScope ? <Badge variant="outline">{workerStatusScope}</Badge> : null}
              {workerQueueStateScope ? <Badge variant="outline">{workerQueueStateScope}</Badge> : null}
              {workerReplayStateScope ? <Badge variant="outline">{workerReplayStateScope}</Badge> : null}
              {executionIDScope ? <Badge variant="secondary">{executionIDScope}</Badge> : null}
              {executionKindScope ? <Badge variant="outline">{formatLifecycleToken(executionKindScope)}</Badge> : null}
              {executionModeScope ? <Badge variant="outline">{formatLifecycleToken(executionModeScope)}</Badge> : null}
              {executionStatusScope ? <Badge variant="outline">{formatLifecycleToken(executionStatusScope)}</Badge> : null}
              {executionQueueStateScope ? (
                <Badge variant="outline">{formatLifecycleToken(executionQueueStateScope)}</Badge>
              ) : null}
              {executionReplayScopeScope ? (
                <Badge variant="outline">{formatLifecycleToken(executionReplayScopeScope)}</Badge>
              ) : null}
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  setWorkerActionScope("")
                  setWorkerConnectorScope("")
                  setWorkerFailureStageScope("")
                  setWorkerKindScope("")
                  setWorkerLabelScope("")
                  setWorkerModeScope("")
                  setWorkerQueueStateScope("")
                  setWorkerReplayStateScope("")
                  setWorkerRequestScope("")
                  setWorkerStatusScope("")
                  setWorkerVendorScope("")
                  setExecutionIDScope("")
                  setExecutionKindScope("")
                  setExecutionModeScope("")
                  setExecutionQueueStateScope("")
                  setExecutionReplayScopeScope("")
                  setExecutionStatusScope("")
                }}
              >
                {t("enterpriseSyncWorkspace.workerAlerts.clear")}
              </Button>
            </div>
          ) : null}
          {focusHint === "worker_alert" ? (
            <div
              className="rounded-lg border bg-muted/10 px-3 py-2 text-xs text-muted-foreground"
              data-testid="enterprise-sync-worker-alert-focus"
            >
              {t("enterpriseSyncWorkspace.workerAlerts.focusHint")}
            </div>
          ) : null}
          {hasExecutionContext ? (
            <div
              className="rounded-lg border bg-muted/10 px-3 py-2"
              data-testid="enterprise-sync-execution-context"
            >
              <p className="text-xs text-muted-foreground">
                {t("enterpriseSyncWorkspace.executionContext.focusHint")}
              </p>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Button asChild size="sm" variant="outline">
                  <Link to={executionReviewAlertsLink} data-testid="enterprise-sync-execution-context-alerts-link">
                    {t("enterpriseSyncWorkspace.executionContext.returnToAlerts")}
                  </Link>
                </Button>
              </div>
            </div>
          ) : null}
          {workerReviewStatus === "handled" ? (
            <div
              className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-2"
              data-testid="enterprise-sync-worker-review"
            >
              <p className="text-xs text-emerald-700">
                {t("enterpriseSyncWorkspace.workerAlerts.reviewHandled", { stage: workerReviewStageLabel })}
              </p>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Button asChild size="sm" variant="outline">
                  <Link to={workerReviewAlertsLink} data-testid="enterprise-sync-worker-review-alerts-link">
                    {t("enterpriseSyncWorkspace.workerAlerts.goSecondaryReview")}
                  </Link>
                </Button>
                <Button asChild size="sm" variant="outline">
                  <Link to={workerReviewResetLink}>{t("enterpriseSyncWorkspace.workerAlerts.clearReviewState")}</Link>
                </Button>
              </div>
            </div>
          ) : null}
          <div className="space-y-2">
            {filteredWorkerAlerts.slice(0, 4).map((item) => {
              const category = classifyEnterpriseSyncWorkerAlertLevel(item)
              const guidance = describeEnterpriseSyncWorkerAlertGuidance(item, t)
              const scopedLinks = buildWorkerAlertScopedLinks(item)
              const workerTitle = item.worker_label?.trim() || item.worker_action?.trim() || "Worker Alert"
              return (
                <div
                  key={`${item.tenant_id}-${item.worker_action || "worker"}-${item.last_seen_at}`}
                  className="rounded-xl border bg-muted/10 px-4 py-3"
                  data-testid="enterprise-sync-worker-alert-item"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <p className="font-medium">{workerTitle}</p>
                      <p className="text-xs text-muted-foreground">{selectedTenantName ?? item.tenant_id}</p>
                    </div>
                    <Badge variant={category === "hot" ? "destructive" : category === "alerting" ? "secondary" : "outline"}>
                      {category === "hot"
                        ? t("enterpriseSyncWorkspace.workerAlerts.category.hot")
                        : category === "alerting"
                          ? t("enterpriseSyncWorkspace.workerAlerts.category.alerting")
                          : t("enterpriseSyncWorkspace.workerAlerts.category.stable")}
                    </Badge>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">
                    failed {item.last_failed} / threshold {item.last_threshold} / last seen {formatDateTime(item.last_seen_at)}
                  </p>
                  {guidance ? (
                    <div
                      className="mt-3 rounded-lg border border-muted-foreground/15 bg-background/80 px-3 py-3"
                      data-testid="enterprise-sync-worker-alert-guidance"
                    >
                      <div className="flex flex-wrap items-center gap-2">
                        <p
                          className="text-sm font-medium"
                          data-testid="enterprise-sync-worker-alert-guidance-title"
                        >
                          {guidance.title}
                        </p>
                        <Badge
                          variant={guidance.badgeVariant}
                          data-testid="enterprise-sync-worker-alert-guidance-badge"
                        >
                          {guidance.badgeLabel}
                        </Badge>
                      </div>
                      <p
                        className="mt-1 text-xs text-muted-foreground"
                        data-testid="enterprise-sync-worker-alert-guidance-summary"
                      >
                        {guidance.summary}
                      </p>
                      <div className="mt-2 space-y-2">
                        <p className="mp-kpi-note">
                          {t("enterpriseSyncWorkspace.workerAlerts.guidance.actionsTitle")}
                        </p>
                        <ul className="list-disc space-y-1 pl-5 text-xs text-muted-foreground">
                          {guidance.suggestions.map((suggestion, index) => (
                            <li
                              key={`${workerTitle}-${index}-${suggestion}`}
                              data-testid={`enterprise-sync-worker-alert-guidance-suggestion-${index}`}
                            >
                              {suggestion}
                            </li>
                          ))}
                        </ul>
                      </div>
                    </div>
                  ) : null}
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <Button asChild size="sm" variant="outline">
                      <Link to={scopedLinks.alerts}>{t("enterpriseSyncWorkspace.workerAlerts.goAlerts")}</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={scopedLinks.directory}>{t("enterpriseSyncWorkspace.workerAlerts.goDirectory")}</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={scopedLinks.policies}>{t("enterpriseSyncWorkspace.workerAlerts.goPolicies")}</Link>
                    </Button>
                    <Button asChild size="sm" variant="outline">
                      <Link to={scopedLinks.wallet} data-testid="enterprise-sync-worker-alert-wallet-link">
                        {t("enterpriseSyncWorkspace.workerAlerts.goWallet")}
                      </Link>
                    </Button>
                  </div>
                </div>
              )
            })}
            {!loading && filteredWorkerAlerts.length === 0 ? (
              <div
                className="rounded-xl border border-dashed px-4 py-8 text-center text-sm text-muted-foreground"
                data-testid="enterprise-sync-worker-alert-empty"
              >
                {t("enterpriseSyncWorkspace.workerAlerts.empty")}
              </div>
            ) : null}
          </div>
        </CardContent>
      </Card>

      <div className="grid gap-4 xl:grid-cols-[1.05fr_0.95fr]">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("enterpriseSyncWorkspace.outcome.title")}</CardTitle>
            <CardDescription>{t("enterpriseSyncWorkspace.outcome.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <p className="font-medium">
                  {latestSyncJob
                    ? t("enterpriseSyncWorkspace.outcome.latestResultWithSource", {
                        source: syncSourceLabel(latestSyncJob.source),
                      })
                    : t("enterpriseSyncWorkspace.outcome.noResult")}
                </p>
                {latestSyncJob ? (
                  <Badge variant={statusBadgeVariant(latestSyncJob.status)}>{latestSyncJob.status}</Badge>
                ) : (
                  <Badge variant="secondary">{t("enterpriseSyncWorkspace.outcome.pendingImport")}</Badge>
                )}
              </div>
              <p className="mt-2 text-sm text-muted-foreground">
                {latestSyncJob
                  ? t("enterpriseSyncWorkspace.outcome.latestResultSummary", {
                      created: latestSyncJob.created,
                      updated: latestSyncJob.updated,
                      deactivated: latestSyncJob.deactivated,
                      rejected: latestSyncJob.rejected,
                    })
                  : t("enterpriseSyncWorkspace.outcome.noResultHint")}
              </p>
              {latestSyncJob ? (
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("enterpriseSyncWorkspace.outcome.startedEnded", {
                    startedAt: formatDateTime(latestSyncJob.started_at),
                    endedAt: formatDateTime(latestSyncJob.ended_at),
                  })}
                </p>
              ) : null}
            </div>

            <div className="grid gap-3 sm:grid-cols-3">
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.outcome.kpi.directoryResult")}</p>
                <p className="mt-1 text-sm font-medium">
                  {loading
                    ? "--"
                    : t("enterpriseSyncWorkspace.outcome.kpi.activeEmployees", { count: activeEmployeeCount })}
                </p>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.outcome.kpi.pendingReview")}</p>
                <p className="mt-1 text-sm font-medium">
                  {loading
                    ? "--"
                    : t("enterpriseSyncWorkspace.outcome.kpi.syncFailures", { count: failedSyncJobCount })}
                </p>
              </div>
              <div className="rounded-lg border bg-muted/10 px-3 py-3">
                <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.outcome.kpi.followup")}</p>
                <p className="mt-1 text-sm font-medium">
                  {loading
                    ? "--"
                    : t("enterpriseSyncWorkspace.outcome.kpi.groupsPolicies", {
                        groups: tenantGroupsCount,
                        policies: tenantPoliciesCount,
                      })}
                </p>
              </div>
            </div>

            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <p className="font-medium">{syncOutcomeAction.title}</p>
              <p className="mt-1 text-sm text-muted-foreground">{syncOutcomeAction.description}</p>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                {renderNavAction(mainflowPriorityNavAction, { variant: "default" })}
                {syncOutcomeFallbackAction
                  ? renderNavAction(syncOutcomeFallbackAction, {
                      label: t("enterpriseSyncWorkspace.outcome.fallbackBySyncResult"),
                      variant: "outline",
                    })
                  : null}
              </div>
              <p className="mt-2 text-xs text-muted-foreground">{t("enterpriseSyncWorkspace.outcome.priorityAligned")}</p>
            </div>

            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <div className="flex flex-wrap items-center gap-2">
                <p className="font-medium">{t("enterpriseSyncWorkspace.pipelineCheck.title")}</p>
                <Badge variant={hasSyncFailure ? "destructive" : pendingSteps > 0 ? "secondary" : "outline"}>
                  {hasSyncFailure
                    ? t("enterpriseSyncWorkspace.pipelineCheck.badgeFailure")
                    : pendingSteps > 0
                      ? t("enterpriseSyncWorkspace.pipelineCheck.badgePending", { count: pendingSteps })
                      : t("enterpriseSyncWorkspace.pipelineCheck.badgeReady")}
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
                        <Badge variant={item.ready ? "outline" : "secondary"}>
                          {item.ready
                            ? t("enterpriseSyncWorkspace.pipelineCheck.stepReady")
                            : t("enterpriseSyncWorkspace.pipelineCheck.stepPending")}
                        </Badge>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">{item.helper}</p>
                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        {renderNavAction(primaryAction, { variant: item.ready ? "outline" : "default" })}
                        {fallbackAction
                          ? renderNavAction(fallbackAction, {
                              label: t("enterpriseSyncWorkspace.pipelineCheck.fallbackByStep"),
                              variant: "outline",
                            })
                          : null}
                      </div>
                    </div>
                  )
                })}
              </div>

              <div className="mt-3 flex flex-wrap items-center gap-2">
                {renderNavAction(mainflowPriorityNavAction, { variant: "default" })}
                {pipelineFallbackAction
                  ? renderNavAction(pipelineFallbackAction, {
                      label: t("enterpriseSyncWorkspace.pipelineCheck.fallbackByConnectivity"),
                      variant: "outline",
                    })
                  : null}
              </div>
              <p className="mt-2 text-xs text-muted-foreground">{t("enterpriseSyncWorkspace.pipelineCheck.priorityAligned")}</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("enterpriseSyncWorkspace.recentResults.title")}</CardTitle>
            <CardDescription>{t("enterpriseSyncWorkspace.recentResults.description")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {!loading && sortedSyncJobs.length === 0 ? (
              <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm text-muted-foreground">
                {t("enterpriseSyncWorkspace.recentResults.empty")}
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
                  {t("enterpriseSyncWorkspace.recentResults.rowSummary", {
                    created: item.created,
                    updated: item.updated,
                    deactivated: item.deactivated,
                    rejected: item.rejected,
                  })}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {t("enterpriseSyncWorkspace.recentResults.rowActorAt", {
                    actor: item.actor || "-",
                    at: formatDateTime(item.ended_at || item.started_at),
                  })}
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
              <CardTitle className="text-base">{t("enterpriseSyncWorkspace.issueCards.title")}</CardTitle>
              <CardDescription>{t("enterpriseSyncWorkspace.issueCards.description")}</CardDescription>
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
              <CardTitle className="text-base">{t("enterpriseSyncWorkspace.remediation.title")}</CardTitle>
              <CardDescription>{t("enterpriseSyncWorkspace.remediation.description")}</CardDescription>
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
                      {fallbackAction
                        ? renderNavAction(fallbackAction, {
                            label: t("enterpriseSyncWorkspace.remediation.fallbackByStep"),
                            variant: "outline",
                          })
                        : null}
                    </div>
                  </div>
                )
              })}
              <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.remediation.priorityHint")}</p>
            </CardContent>
          </Card>
        </div>
      ) : null}
    </TabsContent>
  )
}

// ---------------------------------------------------------------------------
// Lark Directory Sync Section
// ---------------------------------------------------------------------------

function LarkSyncSection({ writable }: { writable: boolean }) {
  const { token } = useAuth()
  const [appId, setAppId] = useState("")
  const [appSecret, setAppSecret] = useState("")
  const [webhookUrl, setWebhookUrl] = useState("")

  const syncMutation = useMutation({
    mutationFn: () => larkSyncUsers(token ?? undefined, appId, appSecret),
  })

  const botTestMutation = useMutation({
    mutationFn: () => larkBotTest(token ?? undefined, webhookUrl),
  })

  const syncResult: LarkSyncResult | undefined = syncMutation.data

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Lark Directory Sync</CardTitle>
          <CardDescription>
            Sync employee directory from Lark (Feishu). Provide your Lark app credentials to pull users.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div>
            <label className="mb-1 block text-sm font-medium">App ID</label>
            <Input
              value={appId}
              onChange={(e) => setAppId(e.target.value)}
              placeholder="cli_xxxxx"
              disabled={!writable}
            />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium">App Secret</label>
            <Input
              type="password"
              value={appSecret}
              onChange={(e) => setAppSecret(e.target.value)}
              placeholder="Lark app secret"
              disabled={!writable}
            />
          </div>
          <div className="flex justify-end">
            <Button
              onClick={() => syncMutation.mutate()}
              disabled={!writable || syncMutation.isPending || !appId.trim() || !appSecret.trim()}
            >
              <RefreshCwIcon className={`mr-1.5 size-4 ${syncMutation.isPending ? "animate-spin" : ""}`} />
              {syncMutation.isPending ? "Syncing..." : "Sync Now"}
            </Button>
          </div>
          {syncMutation.isError ? (
            <div className="flex items-center gap-2 rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-800">
              <XCircleIcon className="size-4" />
              {(syncMutation.error as Error).message || "Sync failed"}
            </div>
          ) : null}
          {syncResult ? (
            <div className="rounded-md border border-green-200 bg-green-50 px-4 py-2 text-sm text-green-800">
              <span className="flex items-center gap-2">
                <CheckCircleIcon className="size-4" />
                Status: {syncResult.status} &mdash; {syncResult.user_count} users synced
              </span>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Lark Bot Test</CardTitle>
          <CardDescription>
            Test your Lark webhook bot to verify alert delivery.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div>
            <label className="mb-1 block text-sm font-medium">Webhook URL</label>
            <Input
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
              placeholder="https://open.larksuite.com/open-apis/bot/v2/hook/..."
              disabled={!writable}
            />
          </div>
          <div className="flex justify-end">
            <Button
              variant="outline"
              onClick={() => botTestMutation.mutate()}
              disabled={!writable || botTestMutation.isPending || !webhookUrl.trim()}
            >
              {botTestMutation.isPending ? "Testing..." : "Test Bot"}
            </Button>
          </div>
          {botTestMutation.isError ? (
            <div className="flex items-center gap-2 rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-800">
              <XCircleIcon className="size-4" />
              {(botTestMutation.error as Error).message || "Test failed"}
            </div>
          ) : null}
          {botTestMutation.data ? (
            <div
              className={`flex items-center gap-2 rounded-md px-4 py-2 text-sm ${
                botTestMutation.data.status === "ok"
                  ? "border border-green-200 bg-green-50 text-green-800"
                  : "border border-red-200 bg-red-50 text-red-800"
              }`}
            >
              {botTestMutation.data.status === "ok" ? (
                <>
                  <CheckCircleIcon className="size-4" />
                  Bot connected successfully
                </>
              ) : (
                <>
                  <XCircleIcon className="size-4" />
                  {botTestMutation.data.error || botTestMutation.data.message || "Unexpected response"}
                </>
              )}
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Google Workspace Sync Section
// ---------------------------------------------------------------------------

function GoogleWorkspaceSyncSection({ writable }: { writable: boolean }) {
  const { token } = useAuth()
  const [serviceAccountJson, setServiceAccountJson] = useState("")
  const [delegatedAdmin, setDelegatedAdmin] = useState("")
  const [domain, setDomain] = useState("")

  const syncMutation = useMutation({
    mutationFn: () =>
      googleWorkspaceSyncUsers(token ?? undefined, serviceAccountJson, delegatedAdmin, domain),
  })

  const syncResult: GoogleWorkspaceSyncResult | undefined = syncMutation.data

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Google Workspace Sync</CardTitle>
        <CardDescription>
          Sync users from Google Workspace using a service account. Users are matched by email domain.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div>
          <label className="mb-1 block text-sm font-medium">Service Account JSON</label>
          <Textarea
            value={serviceAccountJson}
            onChange={(e) => setServiceAccountJson(e.target.value)}
            placeholder='{"type":"service_account","project_id":"...","private_key":"..."}'
            rows={5}
            disabled={!writable}
          />
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <div>
            <label className="mb-1 block text-sm font-medium">Delegated Admin</label>
            <Input
              value={delegatedAdmin}
              onChange={(e) => setDelegatedAdmin(e.target.value)}
              placeholder="admin@company.com"
              disabled={!writable}
            />
          </div>
          <div>
            <label className="mb-1 block text-sm font-medium">Domain</label>
            <Input
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="company.com"
              disabled={!writable}
            />
          </div>
        </div>
        <div className="flex justify-end">
          <Button
            onClick={() => syncMutation.mutate()}
            disabled={
              !writable ||
              syncMutation.isPending ||
              !serviceAccountJson.trim() ||
              !delegatedAdmin.trim() ||
              !domain.trim()
            }
          >
            <RefreshCwIcon className={`mr-1.5 size-4 ${syncMutation.isPending ? "animate-spin" : ""}`} />
            {syncMutation.isPending ? "Syncing..." : "Sync Now"}
          </Button>
        </div>
        {syncMutation.isError ? (
          <div className="flex items-center gap-2 rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-800">
            <XCircleIcon className="size-4" />
            {(syncMutation.error as Error).message || "Sync failed"}
          </div>
        ) : null}
        {syncResult ? (
          <div className="rounded-md border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-800">
            <span className="flex items-center gap-2">
              <CheckCircleIcon className="size-4" />
              Status: {syncResult.status}
            </span>
            <div className="mt-2 grid grid-cols-2 gap-2 text-xs sm:grid-cols-4">
              <div className="rounded bg-white/60 px-2 py-1">
                <span className="font-medium">Created:</span> {syncResult.created}
              </div>
              <div className="rounded bg-white/60 px-2 py-1">
                <span className="font-medium">Deactivated:</span> {syncResult.deactivated}
              </div>
              <div className="rounded bg-white/60 px-2 py-1">
                <span className="font-medium">Skipped:</span> {syncResult.skipped}
              </div>
              <div className="rounded bg-white/60 px-2 py-1">
                <span className="font-medium">Errors:</span> {syncResult.errors}
              </div>
            </div>
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}
