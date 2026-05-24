import { PlayIcon, RotateCcwIcon, Trash2Icon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { type WalletJobSummary } from "@/lib/api"

type WalletDlqGovernanceCardProps = {
  writable: boolean
  readOnlyBoundaryHint: string
  loading: boolean
  refreshing: boolean
  jobSummary: WalletJobSummary | null
  processLimit: string
  onProcessLimitChange: (value: string) => void
  processWorkerCount: string
  onProcessWorkerCountChange: (value: string) => void
  processMaxRetry: string
  onProcessMaxRetryChange: (value: string) => void
  dlqLimit: string
  onDLQLimitChange: (value: string) => void
  dlqErrorCode: string
  onDLQErrorCodeChange: (value: string) => void
  dlqTargetIDOverride: string
  onDLQTargetIDOverrideChange: (value: string) => void
  dlqOlderThanSeconds: string
  onDLQOlderThanSecondsChange: (value: string) => void
  processingJobs: boolean
  requeueingDLQ: boolean
  cleaningDLQ: boolean
  governanceSummary: string
  onProcessPendingJobs: () => void
  onRequeueDLQBatch: () => void
  onCleanupDLQBatch: () => void
  formatDateTime: (value?: string) => string
}

export function WalletDlqGovernanceCard({
  writable,
  readOnlyBoundaryHint,
  loading,
  refreshing,
  jobSummary,
  processLimit,
  onProcessLimitChange,
  processWorkerCount,
  onProcessWorkerCountChange,
  processMaxRetry,
  onProcessMaxRetryChange,
  dlqLimit,
  onDLQLimitChange,
  dlqErrorCode,
  onDLQErrorCodeChange,
  dlqTargetIDOverride,
  onDLQTargetIDOverrideChange,
  dlqOlderThanSeconds,
  onDLQOlderThanSecondsChange,
  processingJobs,
  requeueingDLQ,
  cleaningDLQ,
  governanceSummary,
  onProcessPendingJobs,
  onRequeueDLQBatch,
  onCleanupDLQBatch,
  formatDateTime,
}: WalletDlqGovernanceCardProps) {
  const { t } = useTranslation()
  const busy = loading || refreshing || processingJobs || requeueingDLQ || cleaningDLQ
  const readOnlyDisabledReason = !writable ? t("walletPage.disabledReasons.readOnly") : undefined
  const actionDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : busy
      ? t("walletPage.disabledReasons.busy")
      : undefined
  const confirmAction = (message: string, action: () => void) => {
    if (typeof window !== "undefined" && !window.confirm(message)) {
      return
    }
    action()
  }

  return (
    <Card data-testid="wallet-dlq-governance-card">
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.components.dlqGovernance.title")}</CardTitle>
        <CardDescription>{t("walletPage.components.dlqGovernance.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-2 md:grid-cols-3 xl:grid-cols-6">
          <Badge variant="outline">{t("walletPage.components.dlqGovernance.summary.pending", { count: jobSummary?.pending ?? 0 })}</Badge>
          <Badge variant="outline">{t("walletPage.components.dlqGovernance.summary.processing", { count: jobSummary?.processing ?? 0 })}</Badge>
          <Badge variant="secondary">{t("walletPage.components.dlqGovernance.summary.failed", { count: jobSummary?.failed ?? 0 })}</Badge>
          <Badge variant={(jobSummary?.dlq ?? 0) > 0 ? "warning" : "outline"}>
            {t("walletPage.components.dlqGovernance.summary.dlq", { count: jobSummary?.dlq ?? 0 })}
          </Badge>
          <Badge variant="outline">{t("walletPage.components.dlqGovernance.summary.retryable", { count: jobSummary?.retryable_failed ?? 0 })}</Badge>
          <Badge variant={(jobSummary?.non_retryable_failed ?? 0) > 0 ? "destructive" : "outline"}>
            {t("walletPage.components.dlqGovernance.summary.nonRetryable", { count: jobSummary?.non_retryable_failed ?? 0 })}
          </Badge>
        </div>

        <div className="grid gap-3 xl:grid-cols-3">
          <div className="rounded-lg border bg-muted/20 p-3">
            <div className="mb-3 flex items-center gap-2">
              <PlayIcon className="size-4" />
              <p className="text-sm font-medium">{t("walletPage.components.dlqGovernance.processTitle")}</p>
            </div>
            <div className="grid gap-3">
              <div className="grid gap-2">
                <Label htmlFor="wallet-process-limit">{t("walletPage.components.dlqGovernance.limit")}</Label>
                <Input
                  id="wallet-process-limit"
                  value={processLimit}
                  disabled={!writable}
                  title={readOnlyDisabledReason}
                  onChange={(event) => onProcessLimitChange(event.target.value)}
                  placeholder="20"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="wallet-process-workers">{t("walletPage.components.dlqGovernance.workerCount")}</Label>
                <Input
                  id="wallet-process-workers"
                  value={processWorkerCount}
                  disabled={!writable}
                  title={readOnlyDisabledReason}
                  onChange={(event) => onProcessWorkerCountChange(event.target.value)}
                  placeholder="2"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="wallet-process-max-retry">{t("walletPage.components.dlqGovernance.maxRetry")}</Label>
                <Input
                  id="wallet-process-max-retry"
                  value={processMaxRetry}
                  disabled={!writable}
                  title={readOnlyDisabledReason}
                  onChange={(event) => onProcessMaxRetryChange(event.target.value)}
                  placeholder={t("walletPage.components.dlqGovernance.defaultMaxRetry")}
                />
              </div>
              <Button
                className="w-full gap-2"
                disabled={!writable || busy}
                title={actionDisabledReason}
                onClick={() => confirmAction(t("walletPage.components.dlqGovernance.confirmProcess"), onProcessPendingJobs)}
              >
                <PlayIcon className={`size-4 ${processingJobs ? "animate-pulse" : ""}`} />
                {processingJobs
                  ? t("walletPage.components.dlqGovernance.processing")
                  : t("walletPage.components.dlqGovernance.process")}
              </Button>
            </div>
          </div>

          <div className="rounded-lg border bg-muted/20 p-3">
            <div className="mb-3 flex items-center gap-2">
              <RotateCcwIcon className="size-4" />
              <p className="text-sm font-medium">{t("walletPage.components.dlqGovernance.requeueTitle")}</p>
            </div>
            <div className="grid gap-3">
              <div className="grid gap-2">
                <Label htmlFor="wallet-dlq-limit">{t("walletPage.components.dlqGovernance.limit")}</Label>
                <Input
                  id="wallet-dlq-limit"
                  value={dlqLimit}
                  disabled={!writable}
                  title={readOnlyDisabledReason}
                  onChange={(event) => onDLQLimitChange(event.target.value)}
                  placeholder="20"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="wallet-dlq-error-code">{t("walletPage.components.dlqGovernance.errorCode")}</Label>
                <Input
                  id="wallet-dlq-error-code"
                  value={dlqErrorCode}
                  disabled={!writable}
                  title={readOnlyDisabledReason}
                  onChange={(event) => onDLQErrorCodeChange(event.target.value)}
                  placeholder="template_inactive"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="wallet-dlq-target-override">{t("walletPage.components.dlqGovernance.targetOverride")}</Label>
                <Input
                  id="wallet-dlq-target-override"
                  value={dlqTargetIDOverride}
                  disabled={!writable}
                  title={readOnlyDisabledReason}
                  onChange={(event) => onDLQTargetIDOverrideChange(event.target.value)}
                  placeholder={t("walletPage.components.dlqGovernance.optional")}
                />
              </div>
              <Button
                variant="outline"
                className="w-full gap-2"
                disabled={!writable || busy}
                title={actionDisabledReason}
                onClick={() => confirmAction(t("walletPage.components.dlqGovernance.confirmRequeue"), onRequeueDLQBatch)}
              >
                <RotateCcwIcon className={`size-4 ${requeueingDLQ ? "animate-spin" : ""}`} />
                {requeueingDLQ
                  ? t("walletPage.components.dlqGovernance.requeueing")
                  : t("walletPage.components.dlqGovernance.requeue")}
              </Button>
            </div>
          </div>

          <div className="rounded-lg border bg-muted/20 p-3">
            <div className="mb-3 flex items-center gap-2">
              <Trash2Icon className="size-4" />
              <p className="text-sm font-medium">{t("walletPage.components.dlqGovernance.cleanupTitle")}</p>
            </div>
            <div className="grid gap-3">
              <div className="grid gap-2">
                <Label htmlFor="wallet-dlq-cleanup-limit">{t("walletPage.components.dlqGovernance.limit")}</Label>
                <Input
                  id="wallet-dlq-cleanup-limit"
                  value={dlqLimit}
                  disabled={!writable}
                  title={readOnlyDisabledReason}
                  onChange={(event) => onDLQLimitChange(event.target.value)}
                  placeholder="20"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="wallet-dlq-cleanup-error-code">{t("walletPage.components.dlqGovernance.errorCode")}</Label>
                <Input
                  id="wallet-dlq-cleanup-error-code"
                  value={dlqErrorCode}
                  disabled={!writable}
                  title={readOnlyDisabledReason}
                  onChange={(event) => onDLQErrorCodeChange(event.target.value)}
                  placeholder="template_inactive"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="wallet-dlq-older-than">{t("walletPage.components.dlqGovernance.olderThan")}</Label>
                <Input
                  id="wallet-dlq-older-than"
                  value={dlqOlderThanSeconds}
                  disabled={!writable}
                  title={readOnlyDisabledReason}
                  onChange={(event) => onDLQOlderThanSecondsChange(event.target.value)}
                  placeholder="86400"
                />
              </div>
              <Button
                variant="destructive"
                className="w-full gap-2"
                disabled={!writable || busy}
                title={actionDisabledReason}
                onClick={() => confirmAction(t("walletPage.components.dlqGovernance.confirmCleanup"), onCleanupDLQBatch)}
              >
                <Trash2Icon className={`size-4 ${cleaningDLQ ? "animate-pulse" : ""}`} />
                {cleaningDLQ
                  ? t("walletPage.components.dlqGovernance.cleaning")
                  : t("walletPage.components.dlqGovernance.cleanup")}
              </Button>
            </div>
          </div>
        </div>

        <div className="flex flex-col gap-1 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <span>{t("walletPage.components.dlqGovernance.lastUpdated", { time: formatDateTime(jobSummary?.updated_at) })}</span>
          {!writable ? <span>{t("walletPage.components.dlqGovernance.readOnlyHint")}{readOnlyBoundaryHint}</span> : null}
        </div>
        {governanceSummary ? <p className="text-xs text-emerald-700">{governanceSummary}</p> : null}
      </CardContent>
    </Card>
  )
}
