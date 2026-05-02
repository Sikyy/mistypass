export type EnterpriseSection = "employees" | "sync" | "idp" | "alerts"
export type AlertLandingView = "overview" | "approval_backlog" | "directory_exceptions"
export type AlertSegmentHint = "receipt_recovery"
export type AlertSegmentStatus = "pending" | "attention" | "ready"
export type NotificationHistoryFilter = "all" | "failed" | "retryable" | "suppressed" | "due_now"
export type HRISWebhookExecutionKindFilter = "all" | "receipt_process" | "dlq_replay"
export type HRISWebhookExecutionStatusFilter = "all" | "queued" | "running" | "succeeded" | "failed"
export type HRISWebhookExecutionQueueStateFilter = "all" | "ready" | "cooldown" | "in_flight" | "attempt_limit" | "terminal"
export type HRISWebhookExecutionReplayScopeFilter = "all" | "replayed" | "worker_required"
export type HRISWebhookExecutionModeFilter = "all" | "inline" | "queued"
export type HRISWebhookExecutionDispatchFilter = "all" | "worker_tick" | "worker_task_channel" | "goroutine_fallback"

export type EnterpriseAttentionItem = {
  actionLabel: string
  description: string
  onClick: () => void
  title: string
}

export type EnterpriseLandingAction = {
  alertsView?: AlertLandingView
  kind: "section" | "route"
  label: string
  section?: EnterpriseSection
  to?: string
}

export type EnterpriseLandingCard = {
  action: EnterpriseLandingAction
  description: string
  returnAction?: EnterpriseLandingAction
  returnHint?: string
  statusLabel: string
  statusVariant: "outline" | "secondary" | "destructive"
  title: string
}

export type EnterpriseRecoveryAction = {
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
