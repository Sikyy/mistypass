import { useMemo } from "react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

import { type EnterpriseWorkflowOverviewStep } from "@/components/enterprise/enterprise-workflow-overview"
import {
  type EnterpriseHRISConnector,
  type EnterpriseHRISSecret,
  type EnterpriseSyncJob,
  type EnterpriseSyncWorkerAlertSummaryItem,
  type EnterpriseEmployee,
} from "@/lib/api"

// ── Types ───────────────────────────────────────────────────────────────────

type EnterpriseSection = "employees" | "sync" | "idp" | "scim" | "alerts"
type EnterpriseWorkflowState = "completed" | "pending" | "blocked"

type SyncSourceOption = {
  value: string
  label: string
  description: string
}

// ── Helpers (kept local to the module) ──────────────────────────────────────

function buildSyncSourceOptions(t: TFunction): SyncSourceOption[] {
  return [
    {
      value: "hris",
      label: t("enterprisePage.syncSources.hris.label"),
      description: t("enterprisePage.syncSources.hris.description"),
    },
    {
      value: "scim",
      label: t("enterprisePage.syncSources.scim.label"),
      description: t("enterprisePage.syncSources.scim.description"),
    },
    {
      value: "csv_import",
      label: t("enterprisePage.syncSources.csvImport.label"),
      description: t("enterprisePage.syncSources.csvImport.description"),
    },
    {
      value: "manual_sync",
      label: t("enterprisePage.syncSources.manual.label"),
      description: t("enterprisePage.syncSources.manual.description"),
    },
  ]
}

function buildSyncSourceCheckpointsBySource(t: TFunction): Record<string, string[]> {
  return {
    hris: [
      t("enterprisePage.syncCheckpoints.hris.fieldMapping"),
      t("enterprisePage.syncCheckpoints.hris.primaryKeyEmail"),
    ],
    scim: [
      t("enterprisePage.syncCheckpoints.scim.externalIdEmail"),
      t("enterprisePage.syncCheckpoints.scim.deactivationSemantics"),
    ],
    csv_import: [
      t("enterprisePage.syncCheckpoints.csvImport.headersAndRequired"),
      t("enterprisePage.syncCheckpoints.csvImport.duplicateHandling"),
    ],
    manual_sync: [
      t("enterprisePage.syncCheckpoints.manual.requiredProfileFields"),
      t("enterprisePage.syncCheckpoints.manual.backflowToStableGroups"),
    ],
  }
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

function syncSourceLabel(source: string | undefined, syncSourceOptions: SyncSourceOption[]) {
  if (!source) {
    return "-"
  }
  return syncSourceOptions.find((item) => item.value === source)?.label || source
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

function workflowStateLabel(t: TFunction, state: EnterpriseWorkflowState) {
  switch (state) {
    case "completed":
      return t("enterprisePage.workflow.state.completed")
    case "blocked":
      return t("enterprisePage.workflow.state.blocked")
    default:
      return t("enterprisePage.workflow.state.pending")
  }
}

export function formatDateTime(value?: string, locale?: string) {
  if (!value) {
    return "-"
  }
  const timestamp = new Date(value)
  if (Number.isNaN(timestamp.getTime())) {
    return value
  }
  return timestamp.toLocaleString(locale || undefined)
}

export function statusBadgeVariant(status?: string) {
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

// ── Hook params ─────────────────────────────────────────────────────────────

type UseEnterpriseWorkflowParams = {
  selectedTenantID: string
  loading: boolean
  activeEmployeeCount: number
  idpReady: boolean
  issuedPassCount: number
  failedSyncJobCount: number
  pendingApprovalCount: number
  workerAlertCount: number
  tenantGroups: { length: number; name?: string }[]
  tenantPolicies: { length: number; name?: string }[]
  syncJobs: EnterpriseSyncJob[]
  sortedSyncJobs: EnterpriseSyncJob[]
  latestSyncJob: EnterpriseSyncJob | null
  syncSource: string
  primaryEmployeeHint: EnterpriseEmployee | null
  primaryEmployeeTargetHint: string
  deactivatedEmployeeHint: EnterpriseEmployee | null
  primaryGroupHint: string
  goToSection: (section: EnterpriseSection) => void
  setSyncSource: (source: string) => void
}

export function useEnterpriseWorkflow({
  selectedTenantID,
  loading,
  activeEmployeeCount,
  idpReady,
  issuedPassCount,
  failedSyncJobCount,
  pendingApprovalCount,
  workerAlertCount,
  tenantGroups,
  tenantPolicies,
  syncJobs,
  sortedSyncJobs,
  latestSyncJob,
  syncSource,
  primaryEmployeeHint,
  primaryEmployeeTargetHint,
  deactivatedEmployeeHint,
  primaryGroupHint,
  goToSection,
  setSyncSource,
}: UseEnterpriseWorkflowParams) {
  const { t, i18n } = useTranslation()

  // ── Sync source options ───────────────────────────────────────────────

  const syncSourceOptions = useMemo(() => buildSyncSourceOptions(t), [t])
  const syncSourceCheckpointsBySource = useMemo(() => buildSyncSourceCheckpointsBySource(t), [t])
  const formatDateTimeValue = (value?: string) => formatDateTime(value, i18n.language)
  const syncSourceLabelValue = (source?: string) => syncSourceLabel(source, syncSourceOptions)

  // ── Primary policy hint ───────────────────────────────────────────────

  const primaryPolicyHint = useMemo(() => {
    if (tenantPolicies[0]?.name?.trim()) {
      return tenantPolicies[0].name.trim()
    }
    return primaryGroupHint
      ? t("enterprisePage.flowHints.officePolicyTemplate", { groupName: primaryGroupHint })
      : t("enterprisePage.flowHints.firstPolicy")
  }, [primaryGroupHint, t, tenantPolicies])

  // ── Flow links ────────────────────────────────────────────────────────

  const directoryFlowLink = buildEnterpriseFlowLink("/access/directory", selectedTenantID, "directory", {
    group_name: primaryGroupHint,
    group_desc: t("enterprisePage.flowHints.groupDescSyncDraft"),
    group_member_email: primaryEmployeeHint?.email || "",
    group_member_id: primaryEmployeeHint?.id || "",
    group_member_name: primaryEmployeeHint?.full_name || "",
  })
  const deactivatedDirectoryFlowLink = buildEnterpriseFlowLink("/access/directory", selectedTenantID, "directory", {
    group_name: primaryGroupHint,
    group_desc: t("enterprisePage.flowHints.groupDescDeactivatedCleanup"),
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

  // ── Workflow steps ────────────────────────────────────────────────────

  const workflowSteps = useMemo(
    () => [
      {
        id: "sync" as const,
        title: t("enterprisePage.workflow.steps.sync.title"),
        metric:
          loading
            ? "--"
            : t("enterprisePage.workflow.steps.sync.metric", {
                activeEmployeeCount,
                syncCount: syncJobs.length,
              }),
        state: activeEmployeeCount > 0 ? ("completed" as const) : ("pending" as const),
        helper:
          activeEmployeeCount > 0
            ? t("enterprisePage.workflow.steps.sync.helperCompleted")
            : t("enterprisePage.workflow.steps.sync.helperPending"),
        actionLabel: activeEmployeeCount > 0 ? t("enterprisePage.actions.viewEmployees") : t("enterprisePage.actions.goToSync"),
      },
      {
        id: "directory" as const,
        title: t("enterprisePage.workflow.steps.directory.title"),
        metric: loading ? "--" : t("enterprisePage.workflow.steps.directory.metric", { count: tenantGroups.length }),
        state:
          activeEmployeeCount === 0
            ? ("blocked" as const)
            : tenantGroups.length > 0
              ? ("completed" as const)
              : ("pending" as const),
        helper:
          activeEmployeeCount === 0
            ? t("enterprisePage.workflow.steps.directory.helperBlocked")
            : tenantGroups.length > 0
              ? t("enterprisePage.workflow.steps.directory.helperCompleted")
              : t("enterprisePage.workflow.steps.directory.helperPending"),
        actionLabel: t("enterprisePage.actions.goToDirectory"),
      },
      {
        id: "policies" as const,
        title: t("enterprisePage.workflow.steps.policies.title"),
        metric: loading ? "--" : t("enterprisePage.workflow.steps.policies.metric", { count: tenantPolicies.length }),
        state:
          tenantGroups.length === 0
            ? ("blocked" as const)
            : tenantPolicies.length > 0
              ? ("completed" as const)
              : ("pending" as const),
        helper:
          tenantGroups.length === 0
            ? t("enterprisePage.workflow.steps.policies.helperBlocked")
            : tenantPolicies.length > 0
              ? t("enterprisePage.workflow.steps.policies.helperCompleted")
              : t("enterprisePage.workflow.steps.policies.helperPending"),
        actionLabel: t("enterprisePage.actions.goToPolicies"),
      },
      {
        id: "issuance" as const,
        title: t("enterprisePage.workflow.steps.issuance.title"),
        metric: loading ? "--" : t("enterprisePage.workflow.steps.issuance.metric", { count: issuedPassCount }),
        state:
          tenantPolicies.length === 0
            ? ("blocked" as const)
            : issuedPassCount > 0
              ? ("completed" as const)
              : ("pending" as const),
        helper:
          tenantPolicies.length === 0
            ? t("enterprisePage.workflow.steps.issuance.helperBlocked")
            : issuedPassCount > 0
              ? t("enterprisePage.workflow.steps.issuance.helperCompleted")
              : t("enterprisePage.workflow.steps.issuance.helperPending"),
        actionLabel: issuedPassCount > 0 ? t("enterprisePage.actions.viewIssuanceCenter") : t("enterprisePage.actions.goToWallet"),
      },
    ],
    [activeEmployeeCount, issuedPassCount, loading, syncJobs.length, t, tenantGroups.length, tenantPolicies.length]
  )

  // ── Next workflow action ──────────────────────────────────────────────

  const nextWorkflowAction = useMemo(() => {
    if (activeEmployeeCount === 0) {
      return {
        title: t("enterprisePage.nextAction.connectEmployees.title"),
        description: t("enterprisePage.nextAction.connectEmployees.description"),
        kind: "section" as const,
        section: "sync" as const,
        label: t("enterprisePage.actions.goToSync"),
      }
    }
    if (tenantGroups.length === 0) {
      return {
        title: t("enterprisePage.nextAction.groupEmployees.title"),
        description: t("enterprisePage.nextAction.groupEmployees.description", { activeEmployeeCount }),
        kind: "route" as const,
        to: directoryFlowLink,
        label: t("enterprisePage.actions.goToDirectory"),
      }
    }
    if (tenantPolicies.length === 0) {
      return {
        title: t("enterprisePage.nextAction.createPolicies.title"),
        description: t("enterprisePage.nextAction.createPolicies.description", { tenantGroupsCount: tenantGroups.length }),
        kind: "route" as const,
        to: policiesFlowLink,
        label: t("enterprisePage.actions.goToPolicies"),
      }
    }
    if (issuedPassCount === 0) {
      return {
        title: t("enterprisePage.nextAction.goIssuance.title"),
        description: t("enterprisePage.nextAction.goIssuance.description"),
        kind: "route" as const,
        to: walletFlowLink,
        label: t("enterprisePage.actions.goToWallet"),
      }
    }
    return {
      title: t("enterprisePage.nextAction.closedLoop.title"),
      description: t("enterprisePage.nextAction.closedLoop.description"),
      kind: "route" as const,
      to: walletFlowLink,
      label: t("enterprisePage.actions.viewIssuanceCenter"),
    }
  }, [activeEmployeeCount, directoryFlowLink, issuedPassCount, policiesFlowLink, t, tenantGroups.length, tenantPolicies.length, walletFlowLink])

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
  }, [nextWorkflowAction, t])

  const workflowStepsOverview = useMemo<EnterpriseWorkflowOverviewStep[]>(
    () =>
      workflowSteps.map((step) => ({
        id: step.id,
        title: step.title,
        metric: step.metric,
        helper: step.helper,
        statusLabel: workflowStateLabel(t, step.state),
        statusVariant: workflowStateVariant(step.state) as EnterpriseWorkflowOverviewStep["statusVariant"],
        action:
          step.id === "sync"
            ? ({
                kind: "section" as const,
                section: activeEmployeeCount > 0 ? ("employees" as const) : ("sync" as const),
                label: step.actionLabel,
              })
            : step.id === "directory"
              ? ({
                  kind: "route" as const,
                  to: directoryFlowLink,
                  label: step.actionLabel,
                })
              : step.id === "policies"
                ? ({
                    kind: "route" as const,
                    to: policiesFlowLink,
                    label: step.actionLabel,
                  })
                : ({
                    kind: "route" as const,
                    to: walletFlowLink,
                    label: step.actionLabel,
                  }),
      })),
    [activeEmployeeCount, directoryFlowLink, policiesFlowLink, t, walletFlowLink, workflowSteps]
  )

  const workflowReturnAction = useMemo(() => {
    if (nextWorkflowAction.kind === "section") {
      return {
        kind: "section" as const,
        section: nextWorkflowAction.section,
        label: t("enterprisePage.actions.afterAction", { action: nextWorkflowAction.label }),
      }
    }
    return {
      kind: "route" as const,
      to: nextWorkflowAction.to,
      label: t("enterprisePage.actions.afterAction", { action: nextWorkflowAction.label }),
    }
  }, [nextWorkflowAction])

  // ── Alert landing cards ───────────────────────────────────────────────

  const alertLandingCards = useMemo(() => {
    const resolvePostSyncAction = () => {
      if (activeEmployeeCount === 0) {
        return {
          kind: "section" as const,
          section: "employees" as const,
          label: t("enterprisePage.actions.reviewEmployees"),
        }
      }
      if (tenantGroups.length === 0) {
        return {
          kind: "route" as const,
          to: directoryFlowLink,
          label: t("enterprisePage.actions.goToDirectory"),
        }
      }
      if (tenantPolicies.length === 0) {
        return {
          kind: "route" as const,
          to: policiesFlowLink,
          label: t("enterprisePage.actions.goToPolicies"),
        }
      }
      if (issuedPassCount === 0) {
        return {
          kind: "route" as const,
          to: walletFlowLink,
          label: t("enterprisePage.actions.goToWallet"),
        }
      }
      return {
        kind: "route" as const,
        to: walletFlowLink,
        label: t("enterprisePage.actions.viewIssuanceCenter"),
      }
    }

    const postSyncAction = resolvePostSyncAction()
    const postSyncReturnAction =
      postSyncAction.kind === "section"
        ? {
            kind: "section" as const,
            section: postSyncAction.section,
            label: t("enterprisePage.actions.afterAction", { action: postSyncAction.label }),
          }
        : {
            kind: "route" as const,
            to: postSyncAction.to,
            label: t("enterprisePage.actions.afterAction", { action: postSyncAction.label }),
          }

    const syncLandingCard = !latestSyncJob
      ? {
          title: t("enterprisePage.cards.syncOutcome.title"),
          statusLabel: t("enterprisePage.status.pendingImport"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.cards.syncOutcome.emptyDescription"),
          action: {
            kind: "section" as const,
            section: "sync" as const,
            label: t("enterprisePage.actions.goToSync"),
          },
          returnHint: t("enterprisePage.cards.syncOutcome.returnHintAfterImport"),
        }
      : latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0
        ? {
            title: t("enterprisePage.cards.syncOutcome.title"),
            statusLabel: t("enterprisePage.status.pendingReview"),
            statusVariant: "destructive" as const,
            description: t("enterprisePage.cards.syncOutcome.needsReview", {
              sourceLabel: syncSourceLabelValue(latestSyncJob.source),
              status: latestSyncJob.status,
              rejected: latestSyncJob.rejected,
            }),
            action: {
              kind: "section" as const,
              section: "alerts" as const,
              label: t("enterprisePage.actions.goToAlerts"),
            },
            returnAction: postSyncReturnAction,
            returnHint: t("enterprisePage.cards.syncOutcome.returnHintAfterAlert"),
          }
        : {
            title: t("enterprisePage.cards.syncOutcome.title"),
            statusLabel:
              activeEmployeeCount === 0
                ? t("enterprisePage.status.pendingConfirm")
                : tenantGroups.length === 0 || tenantPolicies.length === 0 || issuedPassCount === 0
                  ? t("enterprisePage.status.pendingBackflow")
                  : t("enterprisePage.status.closedLoop"),
            statusVariant:
              activeEmployeeCount === 0
                ? ("secondary" as const)
                : tenantGroups.length === 0 || tenantPolicies.length === 0 || issuedPassCount === 0
                  ? ("secondary" as const)
                  : ("outline" as const),
            description:
              activeEmployeeCount === 0
                ? t("enterprisePage.cards.syncOutcome.completedEmptyDirectory")
                : tenantGroups.length === 0
                  ? t("enterprisePage.cards.syncOutcome.groupEmployeesNext", { activeEmployeeCount })
                  : tenantPolicies.length === 0
                    ? t("enterprisePage.cards.syncOutcome.applyPoliciesNext", { tenantGroupsCount: tenantGroups.length })
                    : issuedPassCount === 0
                      ? t("enterprisePage.cards.syncOutcome.readyForIssuance")
                      : t("enterprisePage.cards.syncOutcome.connectedWithIssuedPasses", { issuedPassCount }),
            action: postSyncAction,
            returnHint:
              tenantGroups.length === 0 || tenantPolicies.length === 0 || issuedPassCount === 0
                ? t("enterprisePage.cards.syncOutcome.keepMoving")
                : t("enterprisePage.cards.syncOutcome.closedLoopHint"),
          }

    const approvalLandingCard =
      pendingApprovalCount > 0
        ? {
            title: t("enterprisePage.cards.approvalBacklog.title"),
            statusLabel: t("enterprisePage.cards.approvalBacklog.pendingCount", { count: pendingApprovalCount }),
            statusVariant: "secondary" as const,
            description: t("enterprisePage.cards.approvalBacklog.descriptionPending"),
            action: {
              kind: "section" as const,
              section: "alerts" as const,
              label: t("enterprisePage.actions.handleApprovals"),
            },
            returnAction: workflowReturnAction,
            returnHint: t("enterprisePage.cards.approvalBacklog.returnHint"),
          }
        : {
            title: t("enterprisePage.cards.approvalBacklog.title"),
            statusLabel: t("enterprisePage.status.cleared"),
            statusVariant: "outline" as const,
            description: t("enterprisePage.cards.approvalBacklog.descriptionCleared"),
            action: workflowDirectAction,
          }

    const directoryExceptionCard =
      latestSyncJob && latestSyncJob.deactivated > 0
        ? {
            title: t("enterprisePage.cards.directoryException.title"),
            statusLabel: t("enterprisePage.status.pendingCleanup"),
            statusVariant: "secondary" as const,
            description: t("enterprisePage.cards.directoryException.deactivatedEmployees", {
              deactivated: latestSyncJob.deactivated,
            }),
            action: {
              kind: "route" as const,
              to: directoryFlowLink,
              label: t("enterprisePage.actions.reviewDirectory"),
            },
            returnAction: workflowReturnAction,
            returnHint: t("enterprisePage.cards.directoryException.returnHint"),
          }
        : latestSyncJob && latestSyncJob.status === "completed" && activeEmployeeCount === 0
          ? {
              title: t("enterprisePage.cards.directoryException.title"),
              statusLabel: t("enterprisePage.status.needsReview"),
              statusVariant: "destructive" as const,
              description: t("enterprisePage.cards.directoryException.completedButEmpty"),
              action: {
                kind: "section" as const,
                section: "employees" as const,
                label: t("enterprisePage.actions.reviewEmployees"),
              },
              returnAction: {
                kind: "section" as const,
                section: "sync" as const,
                label: t("enterprisePage.actions.backToSyncAfterReview"),
              },
            }
          : {
              title: t("enterprisePage.cards.directoryException.title"),
              statusLabel: t("enterprisePage.status.directoryStable"),
              statusVariant: "outline" as const,
              description: t("enterprisePage.cards.directoryException.stableDescription"),
              action: {
                kind: "route" as const,
                to: directoryFlowLink,
                label: t("enterprisePage.actions.viewDirectory"),
              },
            }

    return [syncLandingCard, approvalLandingCard, directoryExceptionCard]
  }, [
    activeEmployeeCount,
    issuedPassCount,
    latestSyncJob,
    pendingApprovalCount,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    directoryFlowLink,
    policiesFlowLink,
    walletFlowLink,
    workflowDirectAction,
    workflowReturnAction,
  ])

  // ── Attention items ───────────────────────────────────────────────────

  const attentionItems = useMemo(() => {
    const items: Array<{
      title: string
      description: string
      actionLabel: string
      onClick: () => void
    }> = []

    if (!idpReady) {
      items.push({
        title: t("enterprisePage.attention.idpNotReady.title"),
        description: t("enterprisePage.attention.idpNotReady.description"),
        actionLabel: t("enterprisePage.actions.goToIdp"),
        onClick: () => goToSection("idp"),
      })
    }
    if (failedSyncJobCount > 0) {
      items.push({
        title: t("enterprisePage.attention.syncNeedsReview.title"),
        description: t("enterprisePage.attention.syncNeedsReview.description", { failedSyncJobCount }),
        actionLabel: t("enterprisePage.actions.goToAlerts"),
        onClick: () => goToSection("alerts"),
      })
    }
    if (pendingApprovalCount > 0) {
      items.push({
        title: t("enterprisePage.attention.pendingApprovals.title"),
        description: t("enterprisePage.attention.pendingApprovals.description", { pendingApprovalCount }),
        actionLabel: t("enterprisePage.actions.handleApprovals"),
        onClick: () => goToSection("alerts"),
      })
    }
    if (workerAlertCount > 0) {
      items.push({
        title: t("enterprisePage.attention.workerAlert.title"),
        description: t("enterprisePage.attention.workerAlert.description", { workerAlertCount }),
        actionLabel: t("enterprisePage.actions.viewExceptions"),
        onClick: () => goToSection("alerts"),
      })
    }

    return items.slice(0, 4)
  }, [failedSyncJobCount, idpReady, pendingApprovalCount, t, workerAlertCount])

  // ── Quick actions ─────────────────────────────────────────────────────

  const quickActions = useMemo(
    () => [
      {
        title: t("enterprisePage.quickActions.hris.title"),
        description: t("enterprisePage.quickActions.hris.description"),
        actionLabel: t("enterprisePage.actions.setAsSyncEntry"),
        onClick: () => {
          setSyncSource("hris")
          goToSection("sync")
        },
      },
      {
        title: t("enterprisePage.quickActions.csv.title"),
        description: t("enterprisePage.quickActions.csv.description"),
        actionLabel: t("enterprisePage.actions.switchToCsv"),
        onClick: () => {
          setSyncSource("csv_import")
          goToSection("sync")
        },
      },
      {
        title: t("enterprisePage.quickActions.idp.title"),
        description: t("enterprisePage.quickActions.idp.description"),
        actionLabel: t("enterprisePage.actions.viewEnterpriseLogin"),
        onClick: () => {
          goToSection("idp")
        },
      },
      {
        title: t("enterprisePage.labels.alerts"),
        description: t("enterprisePage.quickActions.alerts.description"),
        actionLabel: t("enterprisePage.actions.viewPendingItems"),
        onClick: () => {
          goToSection("alerts")
        },
      },
    ],
    [t]
  )

  // ── Alert recovery action ─────────────────────────────────────────────

  const alertRecoveryAction = useMemo(() => {
    const blockerCount = failedSyncJobCount + pendingApprovalCount + workerAlertCount + (idpReady ? 0 : 1)
    return {
      blockerCount,
      title:
        blockerCount > 0
          ? t("enterprisePage.alertRecovery.withBlockersTitle")
          : t("enterprisePage.alertRecovery.noBlockersTitle"),
      description:
        blockerCount > 0
          ? t("enterprisePage.alertRecovery.withBlockersDescription", { blockerCount })
          : t("enterprisePage.alertRecovery.noBlockersDescription"),
      nextAction: nextWorkflowAction,
    }
  }, [failedSyncJobCount, idpReady, nextWorkflowAction, pendingApprovalCount, t, workerAlertCount])

  // ── IDP outcome action ────────────────────────────────────────────────

  const idpOutcomeAction = useMemo(() => {
    if (!idpReady) {
      return {
        title: t("enterprisePage.idpOutcome.setupIdpFirst.title"),
        description: t("enterprisePage.idpOutcome.setupIdpFirst.description"),
        kind: "section" as const,
        section: "idp" as const,
        label: t("enterprisePage.actions.goToIdp"),
      }
    }
    if (pendingApprovalCount > 0 || failedSyncJobCount > 0 || workerAlertCount > 0) {
      return {
        title: t("enterprisePage.idpOutcome.clearAlertsFirst.title"),
        description: t("enterprisePage.idpOutcome.clearAlertsFirst.description", {
          pendingApprovalCount,
          failedSyncJobCount,
          workerAlertCount,
        }),
        kind: "section" as const,
        section: "alerts" as const,
        label: t("enterprisePage.actions.goToAlerts"),
      }
    }
    if (activeEmployeeCount === 0) {
      return {
        title: t("enterprisePage.idpOutcome.confirmSyncSource.title"),
        description: t("enterprisePage.idpOutcome.confirmSyncSource.description"),
        kind: "section" as const,
        section: "sync" as const,
        label: t("enterprisePage.actions.goToSync"),
      }
    }
    if (tenantGroups.length === 0) {
      return {
        title: t("enterprisePage.idpOutcome.groupEmployeesAfterIdp.title"),
        description: t("enterprisePage.idpOutcome.groupEmployeesAfterIdp.description"),
        kind: "route" as const,
        to: directoryFlowLink,
        label: t("enterprisePage.actions.goToDirectory"),
      }
    }
    if (tenantPolicies.length === 0) {
      return {
        title: t("enterprisePage.idpOutcome.goPolicyRules.title"),
        description: t("enterprisePage.idpOutcome.goPolicyRules.description"),
        kind: "route" as const,
        to: policiesFlowLink,
        label: t("enterprisePage.actions.goToPolicies"),
      }
    }
    return {
      title: t("enterprisePage.idpOutcome.idpClosedLoop.title"),
      description: t("enterprisePage.idpOutcome.idpClosedLoop.description"),
      kind: "route" as const,
      to: walletFlowLink,
      label: t("enterprisePage.actions.goToWallet"),
    }
  }, [
    activeEmployeeCount,
    directoryFlowLink,
    failedSyncJobCount,
    idpReady,
    pendingApprovalCount,
    policiesFlowLink,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
    workerAlertCount,
  ])

  // ── Sync outcome action ───────────────────────────────────────────────

  const syncOutcomeAction = useMemo(() => {
    if (!latestSyncJob) {
      return {
        title: t("enterprisePage.syncOutcome.firstImport.title"),
        description: t("enterprisePage.syncOutcome.firstImport.description"),
        kind: "section" as const,
        section: "sync" as const,
        label: t("enterprisePage.actions.continueImport"),
      }
    }
    if (latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0) {
      return {
        title: t("enterprisePage.syncOutcome.reviewRejected.title"),
        description: t("enterprisePage.syncOutcome.reviewRejected.description", {
          sourceLabel: syncSourceLabelValue(latestSyncJob.source),
          rejected: latestSyncJob.rejected,
        }),
        kind: "section" as const,
        section: "alerts" as const,
        label: t("enterprisePage.actions.goToAlerts"),
      }
    }
    if (activeEmployeeCount === 0) {
      return {
        title: t("enterprisePage.syncOutcome.backToEmployees.title"),
        description: t("enterprisePage.syncOutcome.backToEmployees.description"),
        kind: "section" as const,
        section: "employees" as const,
        label: t("enterprisePage.actions.viewEmployees"),
      }
    }
    if (tenantGroups.length === 0) {
      return {
        title: t("enterprisePage.syncOutcome.groupNewEmployees.title"),
        description: t("enterprisePage.syncOutcome.groupNewEmployees.description", { activeEmployeeCount }),
        kind: "route" as const,
        to: directoryFlowLink,
        label: t("enterprisePage.actions.goToDirectory"),
      }
    }
    if (tenantPolicies.length === 0) {
      return {
        title: t("enterprisePage.syncOutcome.createPolicies.title"),
        description: t("enterprisePage.syncOutcome.createPolicies.description", { tenantGroupsCount: tenantGroups.length }),
        kind: "route" as const,
        to: policiesFlowLink,
        label: t("enterprisePage.actions.goToPolicies"),
      }
    }
    return {
      title: t("enterprisePage.syncOutcome.closedLoop.title"),
      description: t("enterprisePage.syncOutcome.closedLoop.description"),
      kind: "route" as const,
      to: walletFlowLink,
      label: t("enterprisePage.actions.goToWallet"),
    }
  }, [
    activeEmployeeCount,
    directoryFlowLink,
    latestSyncJob,
    policiesFlowLink,
    syncSourceLabelValue,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
  ])

  // ── Sync issue cards ──────────────────────────────────────────────────

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
        title: t("enterprisePage.cards.latestSyncIncomplete.title"),
        description: t("enterprisePage.cards.latestSyncIncomplete.description", {
          sourceLabel: syncSourceLabelValue(latestSyncJob.source),
          status: latestSyncJob.status,
        }),
        label: t("enterprisePage.actions.goToAlerts"),
        kind: "section",
        section: "alerts",
      })
    }
    if (latestSyncJob.rejected > 0) {
      items.push({
        title: t("enterprisePage.cards.rejectedRecords.title"),
        description: t("enterprisePage.cards.rejectedRecords.description", { rejected: latestSyncJob.rejected }),
        label: t("enterprisePage.actions.goToAlerts"),
        kind: "section",
        section: "alerts",
      })
    }
    if (latestSyncJob.deactivated > 0) {
      items.push({
        title: t("enterprisePage.cards.deactivatedEmployees.title"),
        description: t("enterprisePage.cards.deactivatedEmployees.description", { deactivated: latestSyncJob.deactivated }),
        label: t("enterprisePage.actions.goToDeactivatedCleanup"),
        kind: "route",
        to: deactivatedDirectoryFlowLink,
      })
    }
    if (latestSyncJob.status === "completed" && activeEmployeeCount === 0) {
      items.push({
        title: t("enterprisePage.cards.syncCompletedEmpty.title"),
        description: t("enterprisePage.cards.syncCompletedEmpty.description"),
        label: t("enterprisePage.actions.viewEmployees"),
        kind: "section",
        section: "employees",
      })
    }

    return items.slice(0, 4)
  }, [activeEmployeeCount, deactivatedDirectoryFlowLink, latestSyncJob, syncSourceLabelValue, t])

  // ── Sync remediation steps ────────────────────────────────────────────

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
        title: t("enterprisePage.remediation.firstSync.title"),
        description: t("enterprisePage.remediation.firstSync.description"),
        state: "pending",
        kind: "section",
        label: t("enterprisePage.actions.goToSync"),
        section: "sync",
      })
      return steps
    }

    steps.push({
      title: t("enterprisePage.remediation.reviewLatest.title"),
      description: t("enterprisePage.remediation.reviewLatest.description", {
        sourceLabel: syncSourceLabelValue(latestSyncJob.source),
        status: latestSyncJob.status,
      }),
      state: "completed",
      kind: "section",
      label: t("enterprisePage.actions.viewSyncResult"),
      section: "sync",
    })

    steps.push({
      title: t("enterprisePage.remediation.handleRejected.title"),
      description:
        latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0
          ? t("enterprisePage.remediation.handleRejected.pending", { rejected: latestSyncJob.rejected })
          : t("enterprisePage.remediation.handleRejected.completed"),
      state: latestSyncJob.status !== "completed" || latestSyncJob.rejected > 0 ? "pending" : "completed",
      kind: "section",
      label: t("enterprisePage.actions.goToAlerts"),
      section: "alerts",
    })

    steps.push({
      title: t("enterprisePage.remediation.reviewDeactivatedImpact.title"),
      description:
        latestSyncJob.deactivated > 0
          ? t("enterprisePage.remediation.reviewDeactivatedImpact.pending", { deactivated: latestSyncJob.deactivated })
          : t("enterprisePage.remediation.reviewDeactivatedImpact.completed"),
      state: latestSyncJob.deactivated > 0 ? "pending" : "completed",
      kind: "route",
      label: t("enterprisePage.actions.goToDeactivatedCleanup"),
      to: deactivatedDirectoryFlowLink,
    })

    steps.push({
      title: t("enterprisePage.remediation.pushToPolicyAndIssuance.title"),
      description:
        activeEmployeeCount === 0
          ? t("enterprisePage.remediation.pushToPolicyAndIssuance.noEmployees")
          : tenantGroups.length === 0
            ? t("enterprisePage.remediation.pushToPolicyAndIssuance.noGroups")
            : tenantPolicies.length === 0
              ? t("enterprisePage.remediation.pushToPolicyAndIssuance.noPolicies")
              : t("enterprisePage.remediation.pushToPolicyAndIssuance.readyForIssuance"),
      state:
        activeEmployeeCount === 0
          ? "blocked"
          : tenantGroups.length === 0 || tenantPolicies.length === 0
            ? "pending"
            : "completed",
      kind: activeEmployeeCount === 0 ? "section" : "route",
      label:
        activeEmployeeCount === 0
          ? t("enterprisePage.actions.viewEmployees")
          : tenantGroups.length === 0
            ? t("enterprisePage.actions.goToDirectory")
            : tenantPolicies.length === 0
              ? t("enterprisePage.actions.goToPolicies")
              : t("enterprisePage.actions.goToWallet"),
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
  }, [
    activeEmployeeCount,
    deactivatedDirectoryFlowLink,
    directoryFlowLink,
    latestSyncJob,
    policiesFlowLink,
    syncSourceLabelValue,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
  ])

  // ── Sync source feedback ──────────────────────────────────────────────

  const syncSourceFeedback = useMemo(() => {
    const checkpoints = syncSourceCheckpointsBySource[syncSource] ?? []
    const latestJobOfSource = sortedSyncJobs.find((item) => item.source === syncSource) ?? null
    const sourceLabel = syncSourceLabelValue(syncSource)

    if (!latestJobOfSource) {
      return {
        title: t("enterprisePage.syncSourceFeedback.pendingFirstResult.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.pendingSubmit"),
        statusVariant: "secondary" as const,
        description: t("enterprisePage.syncSourceFeedback.pendingFirstResult.description"),
        checkpoints,
        action: {
          kind: "section" as const,
          section: "sync" as const,
          label: t("enterprisePage.actions.submitSyncJob"),
        },
      }
    }

    if (latestJobOfSource.status !== "completed" || latestJobOfSource.rejected > 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.pendingReview.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.needsReview"),
        statusVariant: "destructive" as const,
        description: t("enterprisePage.syncSourceFeedback.pendingReview.description", {
          sourceLabel,
          status: latestJobOfSource.status,
          rejected: latestJobOfSource.rejected,
        }),
        checkpoints,
        action: {
          kind: "section" as const,
          section: "alerts" as const,
          label: t("enterprisePage.actions.goToAlerts"),
        },
      }
    }

    if (syncSource === "scim" && latestJobOfSource.deactivated > 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.scimDeactivated.title"),
        statusLabel: t("enterprisePage.status.pendingBackflow"),
        statusVariant: "secondary" as const,
        description: t("enterprisePage.syncSourceFeedback.scimDeactivated.description", {
          deactivated: latestJobOfSource.deactivated,
        }),
        checkpoints,
        action: {
          kind: "route" as const,
          to: deactivatedDirectoryFlowLink,
          label: t("enterprisePage.actions.goToDeactivatedCleanup"),
        },
      }
    }

    if (activeEmployeeCount === 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.pendingConfirm.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.pendingConfirm"),
        statusVariant: "secondary" as const,
        description: t("enterprisePage.syncSourceFeedback.pendingConfirm.description"),
        checkpoints,
        action: {
          kind: "section" as const,
          section: "employees" as const,
          label: t("enterprisePage.actions.viewEmployees"),
        },
      }
    }

    if (tenantGroups.length === 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.readyForGrouping.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.readyToProceed"),
        statusVariant: "outline" as const,
        description: t("enterprisePage.syncSourceFeedback.readyForGrouping.description", { activeEmployeeCount }),
        checkpoints,
        action: {
          kind: "route" as const,
          to: directoryFlowLink,
          label: t("enterprisePage.actions.goToDirectory"),
        },
      }
    }

    if (tenantPolicies.length === 0) {
      return {
        title: t("enterprisePage.syncSourceFeedback.backflowToDirectory.title", { sourceLabel }),
        statusLabel: t("enterprisePage.status.readyToProceed"),
        statusVariant: "outline" as const,
        description: t("enterprisePage.syncSourceFeedback.backflowToDirectory.description", { tenantGroupsCount: tenantGroups.length }),
        checkpoints,
        action: {
          kind: "route" as const,
          to: policiesFlowLink,
          label: t("enterprisePage.actions.goToPolicies"),
        },
      }
    }

    return {
      title: t("enterprisePage.syncSourceFeedback.closedLoop.title", { sourceLabel }),
      statusLabel: t("enterprisePage.status.closedLoop"),
      statusVariant: "outline" as const,
      description: t("enterprisePage.syncSourceFeedback.closedLoop.description"),
      checkpoints,
      action: {
        kind: "route" as const,
        to: walletFlowLink,
        label: t("enterprisePage.actions.goToWallet"),
      },
    }
  }, [
    activeEmployeeCount,
    deactivatedDirectoryFlowLink,
    directoryFlowLink,
    policiesFlowLink,
    sortedSyncJobs,
    syncSource,
    syncSourceCheckpointsBySource,
    syncSourceLabelValue,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
  ])

  // ── Sync source status cards ──────────────────────────────────────────

  const syncSourceStatusCards = useMemo(() => {
    return syncSourceOptions.map((item) => {
      const sourceLabel = item.label
      const latestJobOfSource = sortedSyncJobs.find((job) => job.source === item.value) ?? null
      const checkpoints = syncSourceCheckpointsBySource[item.value] ?? []
      const metrics = latestJobOfSource
        ? t("enterprisePage.syncSourceStatus.metrics", {
            created: latestJobOfSource.created,
            updated: latestJobOfSource.updated,
            deactivated: latestJobOfSource.deactivated,
            rejected: latestJobOfSource.rejected,
          })
        : t("enterprisePage.syncSourceStatus.noResult")
      const latestEndedAt = latestJobOfSource
        ? formatDateTimeValue(latestJobOfSource.ended_at || latestJobOfSource.started_at)
        : "-"

      if (!latestJobOfSource) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.pendingConnect"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.syncSourceStatus.pendingConnect.description"),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "section" as const,
            section: "sync" as const,
            label: t("enterprisePage.actions.goToSync"),
          },
        }
      }

      if (latestJobOfSource.status !== "completed" || latestJobOfSource.rejected > 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.needsReview"),
          statusVariant: "destructive" as const,
          description: t("enterprisePage.syncSourceStatus.needsReview.description", {
            status: latestJobOfSource.status,
            rejected: latestJobOfSource.rejected,
          }),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "section" as const,
            section: "alerts" as const,
            label: t("enterprisePage.actions.goToAlerts"),
          },
        }
      }

      if (item.value === "scim" && latestJobOfSource.deactivated > 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.deactivatedNeedsCleanup"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.syncSourceStatus.deactivatedNeedsCleanup.description", {
            deactivated: latestJobOfSource.deactivated,
          }),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: deactivatedDirectoryFlowLink,
            label: t("enterprisePage.actions.goToDeactivatedCleanup"),
          },
        }
      }

      if (activeEmployeeCount === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.pendingConfirm"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.syncSourceStatus.pendingConfirm.description"),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "section" as const,
            section: "employees" as const,
            label: t("enterprisePage.actions.viewEmployees"),
          },
        }
      }

      if (tenantGroups.length === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.readyToProceed"),
          statusVariant: "outline" as const,
          description: t("enterprisePage.syncSourceStatus.readyForGrouping.description", { activeEmployeeCount }),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: directoryFlowLink,
            label: t("enterprisePage.actions.goToDirectory"),
          },
        }
      }

      if (tenantPolicies.length === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.readyToProceed"),
          statusVariant: "outline" as const,
          description: t("enterprisePage.syncSourceStatus.readyForPolicies.description", { tenantGroupsCount: tenantGroups.length }),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: policiesFlowLink,
            label: t("enterprisePage.actions.goToPolicies"),
          },
        }
      }

      if (issuedPassCount === 0) {
        return {
          source: item.value,
          title: sourceLabel,
          statusLabel: t("enterprisePage.status.pendingIssuance"),
          statusVariant: "secondary" as const,
          description: t("enterprisePage.syncSourceStatus.pendingIssuance.description"),
          metrics,
          latestEndedAt,
          checkpoints,
          action: {
            kind: "route" as const,
            to: walletFlowLink,
            label: t("enterprisePage.actions.goToWallet"),
          },
        }
      }

      return {
        source: item.value,
        title: sourceLabel,
        statusLabel: t("enterprisePage.status.closedLoop"),
        statusVariant: "outline" as const,
        description: t("enterprisePage.syncSourceStatus.closedLoop.description", { issuedPassCount }),
        metrics,
        latestEndedAt,
        checkpoints,
        action: {
          kind: "route" as const,
          to: walletFlowLink,
          label: t("enterprisePage.actions.viewIssuanceCenter"),
        },
      }
    })
  }, [
    activeEmployeeCount,
    deactivatedDirectoryFlowLink,
    directoryFlowLink,
    formatDateTimeValue,
    issuedPassCount,
    policiesFlowLink,
    sortedSyncJobs,
    syncSourceCheckpointsBySource,
    syncSourceOptions,
    t,
    tenantGroups.length,
    tenantPolicies.length,
    walletFlowLink,
  ])

  return {
    // Options and labels
    syncSourceOptions,
    syncSourceCheckpointsBySource,
    formatDateTimeValue,
    syncSourceLabelValue,
    workflowStateLabel: (state: EnterpriseWorkflowState) => workflowStateLabel(t, state),
    workflowStateVariant,

    // Flow links
    directoryFlowLink,
    deactivatedDirectoryFlowLink,
    policiesFlowLink,
    walletFlowLink,
    enterpriseSyncFlowLink,
    enterpriseAlertsFlowLink,

    // Workflow steps and actions
    workflowSteps,
    workflowStepsOverview,
    nextWorkflowAction,
    workflowDirectAction,
    workflowReturnAction,

    // Landing/overview cards
    alertLandingCards,
    attentionItems,
    quickActions,
    alertRecoveryAction,
    idpOutcomeAction,
    syncOutcomeAction,
    syncIssueCards,
    syncRemediationSteps,
    syncSourceFeedback,
    syncSourceStatusCards,
  }
}
