import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { z } from "zod"

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
import { Textarea } from "@/components/ui/textarea"
import { type WalletIssueJob, type WalletPassTemplate } from "@/lib/api"

const batchExecutionModeValues = ["inline", "queued"] as const
const singleIssueSchema = z.object({
  single_template_id: z.string().trim().min(1, "Please select an issuance template first"),
  single_target_id: z
    .string()
    .trim()
    .min(1, "Please enter an employee or visitor ID")
    .max(128, "Employee or visitor ID must be at most 128 characters"),
  single_expires_at: z
    .string()
    .optional()
    .or(z.literal(""))
    .refine((value) => !value || !Number.isNaN(new Date(value).getTime()), "Invalid expiration time format"),
})
const batchIssueSchema = z.object({
  batch_template_id: z.string().trim().min(1, "Please select a batch issuance template first"),
  batch_expires_at: z
    .string()
    .optional()
    .or(z.literal(""))
    .refine((value) => !value || !Number.isNaN(new Date(value).getTime()), "Invalid expiration time format"),
  batch_execution_mode: z.enum(batchExecutionModeValues),
  batch_target_ids: z
    .string()
    .trim()
    .min(1, "Please enter at least one employee or visitor ID")
    .max(200000, "Batch target content is too long, please split and resubmit")
    .refine((value) => parseBatchTargetIDs(value).length > 0, "Please enter at least one employee or visitor ID"),
})

type SingleIssueFormValues = z.infer<typeof singleIssueSchema>
type BatchIssueFormValues = z.infer<typeof batchIssueSchema>

function parseBatchTargetIDs(raw: string): string[] {
  return Array.from(
    new Set(
      raw
        .split(/[\n,;]+/g)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

type EnterpriseMissingTargetRow = {
  targetID: string
  category: "issue_ready" | "needs_directory" | "needs_alerts"
  sourceLabel: string
  groupLabel: string
  reason: string
  employeeName: string
  approvalHint: string
}

type EnterpriseBatchTargetStats = {
  targetIDs: string[]
  matchedIDs: string[]
  missingIDs: string[]
  hitRate: number
}

type EnterpriseMissingTargetBreakdown = {
  rows: EnterpriseMissingTargetRow[]
  issueReadyCount: number
  needsDirectoryCount: number
  needsAlertsCount: number
}

type WalletIssueJobQueueCardProps = {
  writable: boolean
  loading: boolean
  refreshing: boolean
  enterpriseBatchTargetStats: EnterpriseBatchTargetStats
  enterpriseMissingTargetBreakdown: EnterpriseMissingTargetBreakdown
  enterpriseSyncIssueHint: string
  issueReadyEnterpriseMissingTargetIDs: string[]
  onKeepIssueReadyEnterpriseTargets: () => void
  onKeepMissingEnterpriseTargets: () => void
  onRestoreEnterpriseTargets: () => void
  accessDirectoryReviewLink: string
  canOpenAccessReview: boolean
  enterpriseAlertsIssueLink: string
  canOpenEnterpriseReview: boolean
  hasWorkerAlertFlowHints: boolean
  enterpriseSyncWorkerReviewLink: string
  onSubmitSingleIssue: (payload: { templateID: string; targetID: string; expiresAt: string }) => Promise<boolean>
  targetTypeLabel: (type: string) => string
  singleTargetType: "user" | "visitor"
  singleTemplateID: string
  onSingleTemplateIDChange: (value: string) => void
  templates: WalletPassTemplate[]
  singleTargetID: string
  onSingleTargetIDChange: (value: string) => void
  singleExpiresAt: string
  onSingleExpiresAtChange: (value: string) => void
  selectedSingleTemplate: WalletPassTemplate | null
  getTemplateScenarioLabel: (template: WalletPassTemplate) => string
  getTemplateScenarioHint: (template: WalletPassTemplate) => string
  issuingSingle: boolean
  onSubmitBatchIssue: (payload: {
    templateID: string
    targetIDs: string[]
    expiresAt: string
    executionMode: "inline" | "queued"
  }) => Promise<boolean>
  batchTargetType: "user" | "visitor"
  batchTemplateID: string
  onBatchTemplateIDChange: (value: string) => void
  batchExpiresAt: string
  onBatchExpiresAtChange: (value: string) => void
  batchExecutionMode: "inline" | "queued"
  onBatchExecutionModeChange: (value: "inline" | "queued") => void
  batchTargetIDs: string
  onBatchTargetIDsChange: (value: string) => void
  selectedBatchTemplate: WalletPassTemplate | null
  issuingBatch: boolean
  lastIssuedJobs: WalletIssueJob[]
  formatDateTime: (value?: string) => string
}

export function WalletIssueJobQueueCard({
  writable,
  loading,
  refreshing,
  enterpriseBatchTargetStats,
  enterpriseMissingTargetBreakdown,
  enterpriseSyncIssueHint,
  issueReadyEnterpriseMissingTargetIDs,
  onKeepIssueReadyEnterpriseTargets,
  onKeepMissingEnterpriseTargets,
  onRestoreEnterpriseTargets,
  accessDirectoryReviewLink,
  canOpenAccessReview,
  enterpriseAlertsIssueLink,
  canOpenEnterpriseReview,
  hasWorkerAlertFlowHints,
  enterpriseSyncWorkerReviewLink,
  onSubmitSingleIssue,
  targetTypeLabel,
  singleTargetType,
  singleTemplateID,
  onSingleTemplateIDChange,
  templates,
  singleTargetID,
  onSingleTargetIDChange,
  singleExpiresAt,
  onSingleExpiresAtChange,
  selectedSingleTemplate,
  getTemplateScenarioLabel,
  getTemplateScenarioHint,
  issuingSingle,
  onSubmitBatchIssue,
  batchTargetType,
  batchTemplateID,
  onBatchTemplateIDChange,
  batchExpiresAt,
  onBatchExpiresAtChange,
  batchExecutionMode,
  onBatchExecutionModeChange,
  batchTargetIDs,
  onBatchTargetIDsChange,
  selectedBatchTemplate,
  issuingBatch,
  lastIssuedJobs,
  formatDateTime,
}: WalletIssueJobQueueCardProps) {
  const { t } = useTranslation()
  const singleIssueForm = useForm<SingleIssueFormValues>({
    resolver: zodResolver(singleIssueSchema),
    values: {
      single_template_id: singleTemplateID,
      single_target_id: singleTargetID,
      single_expires_at: singleExpiresAt,
    },
  })
  const batchIssueForm = useForm<BatchIssueFormValues>({
    resolver: zodResolver(batchIssueSchema),
    values: {
      batch_template_id: batchTemplateID,
      batch_expires_at: batchExpiresAt,
      batch_execution_mode: batchExecutionMode,
      batch_target_ids: batchTargetIDs,
    },
  })
  const singleTargetIDField = singleIssueForm.register("single_target_id")
  const singleExpiresAtField = singleIssueForm.register("single_expires_at")
  const batchExpiresAtField = batchIssueForm.register("batch_expires_at")
  const batchTargetIDsField = batchIssueForm.register("batch_target_ids")
  const singleIssueFormError =
    singleIssueForm.formState.errors.single_template_id?.message ||
    singleIssueForm.formState.errors.single_target_id?.message ||
    singleIssueForm.formState.errors.single_expires_at?.message ||
    ""
  const batchIssueFormError =
    batchIssueForm.formState.errors.batch_template_id?.message ||
    batchIssueForm.formState.errors.batch_expires_at?.message ||
    batchIssueForm.formState.errors.batch_execution_mode?.message ||
    batchIssueForm.formState.errors.batch_target_ids?.message ||
    ""

  async function onSubmitSingleIssueForm(values: SingleIssueFormValues) {
    await onSubmitSingleIssue({
      templateID: values.single_template_id.trim(),
      targetID: values.single_target_id.trim(),
      expiresAt: values.single_expires_at || "",
    })
  }

  async function onSubmitBatchIssueForm(values: BatchIssueFormValues) {
    await onSubmitBatchIssue({
      templateID: values.batch_template_id.trim(),
      targetIDs: parseBatchTargetIDs(values.batch_target_ids),
      expiresAt: values.batch_expires_at || "",
      executionMode: values.batch_execution_mode,
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.components.issueQueue.title", { defaultValue: "Issue now" })}</CardTitle>
        <CardDescription>
          {t("walletPage.components.issueQueue.description", {
            defaultValue:
              "Select a template first, then issue to employee or visitor. Template scenario determines mobile pass, physical-card flow, visitor QR, or temporary pass.",
          })}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {enterpriseBatchTargetStats.targetIDs.length > 0 ? (
          <div className="rounded-xl border bg-muted/10 px-4 py-3">
            <div className="flex flex-col gap-2">
              <div className="space-y-0.5">
                <p className="text-sm font-medium">
                  {t("walletPage.components.issueQueue.enterpriseHitRate", {
                    defaultValue: "Prefilled enterprise target hit rate {{rate}}%",
                    rate: enterpriseBatchTargetStats.hitRate,
                  })}
                </p>
                <p className="mp-kpi-note">
                  {t("walletPage.components.issueQueue.enterprisePrefillSummary", {
                    defaultValue:
                      "Prefilled {{targets}} targets, matched existing passes {{matched}}, missing {{missing}}.",
                    targets: enterpriseBatchTargetStats.targetIDs.length,
                    matched: enterpriseBatchTargetStats.matchedIDs.length,
                    missing: enterpriseBatchTargetStats.missingIDs.length,
                  })}
                </p>
                {enterpriseBatchTargetStats.missingIDs.length > 0 ? (
                  <p className="mp-kpi-note">
                    {t("walletPage.components.issueQueue.enterpriseMissingBreakdown", {
                      defaultValue:
                        "Among missing targets: issue-ready {{issueReady}}, needs directory review {{needsDirectory}}, needs approvals/exception handling {{needsAlerts}}.",
                      issueReady: enterpriseMissingTargetBreakdown.issueReadyCount,
                      needsDirectory: enterpriseMissingTargetBreakdown.needsDirectoryCount,
                      needsAlerts: enterpriseMissingTargetBreakdown.needsAlertsCount,
                    })}
                  </p>
                ) : null}
                {enterpriseSyncIssueHint ? (
                  <p className="text-xs text-amber-700">{enterpriseSyncIssueHint}</p>
                ) : null}
              </div>
              {enterpriseMissingTargetBreakdown.rows.length > 0 ? (
                <div className="rounded-lg border bg-background px-3 py-2">
                  <p className="text-xs font-medium">
                    {t("walletPage.components.issueQueue.enterpriseMissingDetailsTitle", {
                      defaultValue: "Missing target details (up to 3)",
                    })}
                  </p>
                  <div className="mt-1 space-y-1">
                    {enterpriseMissingTargetBreakdown.rows.slice(0, 3).map((item) => (
                      <p key={item.targetID} className="mp-kpi-note">
                        {item.targetID}
                        {item.employeeName ? ` (${item.employeeName})` : ""} · {item.reason}
                        {item.groupLabel !== "-" ? ` · Group ${item.groupLabel}` : ""}
                        {item.sourceLabel !== "-" ? ` · Source ${item.sourceLabel}` : ""}
                      </p>
                    ))}
                  </div>
                </div>
              ) : null}
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!writable || loading || refreshing || issueReadyEnterpriseMissingTargetIDs.length === 0}
                  onClick={onKeepIssueReadyEnterpriseTargets}
                >
                  {t("walletPage.components.issueQueue.keepIssueReadyOnly", {
                    defaultValue: "Keep issue-ready targets only ({{count}})",
                    count: issueReadyEnterpriseMissingTargetIDs.length,
                  })}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!writable || loading || refreshing || enterpriseBatchTargetStats.missingIDs.length === 0}
                  onClick={onKeepMissingEnterpriseTargets}
                >
                  {t("walletPage.components.issueQueue.keepMissingOnly", {
                    defaultValue: "Keep missing targets only ({{count}})",
                    count: enterpriseBatchTargetStats.missingIDs.length,
                  })}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!writable || loading || refreshing}
                  onClick={onRestoreEnterpriseTargets}
                >
                  {t("walletPage.components.issueQueue.restoreAllPrefilled", {
                    defaultValue: "Restore all prefilled targets ({{count}})",
                    count: enterpriseBatchTargetStats.targetIDs.length,
                  })}
                </Button>
                {canOpenAccessReview ? (
                  <Button asChild size="sm" variant="outline">
                    <Link to={accessDirectoryReviewLink}>
                      {t("walletPage.components.issueQueue.goDirectoryReview", { defaultValue: "Review target source in directory" })}
                    </Link>
                  </Button>
                ) : null}
                {canOpenEnterpriseReview ? (
                  <Button asChild size="sm" variant="outline">
                    <Link to={enterpriseAlertsIssueLink}>
                      {t("walletPage.components.issueQueue.goEnterpriseSyncIssue", {
                        defaultValue: "Back to enterprise and locate by sync anomalies",
                      })}
                    </Link>
                  </Button>
                ) : null}
                {canOpenEnterpriseReview && hasWorkerAlertFlowHints ? (
                  <Button asChild size="sm" variant="outline">
                    <Link to={enterpriseSyncWorkerReviewLink}>
                      {t("walletPage.components.issueQueue.backToSyncReview", {
                        defaultValue: "Return to import & sync review after handling",
                      })}
                    </Link>
                  </Button>
                ) : null}
              </div>
            </div>
          </div>
        ) : null}
        <div className="grid gap-4 xl:grid-cols-2">
          <form className="space-y-3 rounded-xl border bg-muted/15 p-4" onSubmit={singleIssueForm.handleSubmit(onSubmitSingleIssueForm)}>
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium">{t("walletPage.components.issueQueue.singleIssue", { defaultValue: "Single issue" })}</p>
              <Badge variant="outline">{targetTypeLabel(singleTargetType)}</Badge>
            </div>
            <Controller
              control={singleIssueForm.control}
              name="single_template_id"
              render={({ field }) => (
                <Select
                  value={field.value}
                  onValueChange={(value) => {
                    field.onChange(value)
                    onSingleTemplateIDChange(value)
                  }}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t("walletPage.components.issueQueue.selectTemplate", { defaultValue: "Select issuance template" })} />
                  </SelectTrigger>
                  <SelectContent>
                    {templates.map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
            <Input
              {...singleTargetIDField}
              disabled={!writable}
              onChange={(event) => {
                singleTargetIDField.onChange(event)
                onSingleTargetIDChange(event.target.value)
              }}
              placeholder={
                singleTargetType === "visitor"
                  ? t("walletPage.components.issueQueue.visitorIDPlaceholder", { defaultValue: "Visitor ID, e.g. visitor-001" })
                  : t("walletPage.components.issueQueue.employeeIDPlaceholder", { defaultValue: "Employee ID, e.g. user-001" })
              }
            />
            <Input
              {...singleExpiresAtField}
              type="datetime-local"
              disabled={!writable}
              onChange={(event) => {
                singleExpiresAtField.onChange(event)
                onSingleExpiresAtChange(event.target.value)
              }}
            />
            <div className="space-y-2">
              <p className="mp-kpi-note">
                {selectedSingleTemplate
                  ? t("walletPage.components.issueQueue.singleTemplateHint", {
                      defaultValue:
                        "Current template scenario: {{scenario}}. {{hint}} Leave expiration empty to keep default policy.",
                      scenario: getTemplateScenarioLabel(selectedSingleTemplate),
                      hint: getTemplateScenarioHint(selectedSingleTemplate),
                    })
                  : t("walletPage.components.issueQueue.selectTemplateFirst", { defaultValue: "Please select a template first." })}
              </p>
              <Button
                type="submit"
                className="w-full"
                disabled={!writable || issuingSingle || !singleTemplateID || loading || refreshing || singleIssueForm.formState.isSubmitting}
              >
                {issuingSingle
                  ? t("walletPage.components.issueQueue.issuingSingle", { defaultValue: "Issuing..." })
                  : t("walletPage.components.issueQueue.issueOnePass", { defaultValue: "Issue 1 pass" })}
              </Button>
            </div>
            {singleIssueFormError ? <p className="text-sm text-destructive">{singleIssueFormError}</p> : null}
          </form>

          <form className="space-y-3 rounded-xl border bg-muted/15 p-4" onSubmit={batchIssueForm.handleSubmit(onSubmitBatchIssueForm)}>
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium">{t("walletPage.components.issueQueue.batchIssue", { defaultValue: "Batch issue" })}</p>
              <Badge variant="outline">{targetTypeLabel(batchTargetType)}</Badge>
            </div>
            <Controller
              control={batchIssueForm.control}
              name="batch_template_id"
              render={({ field }) => (
                <Select
                  value={field.value}
                  onValueChange={(value) => {
                    field.onChange(value)
                    onBatchTemplateIDChange(value)
                  }}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t("walletPage.components.issueQueue.selectBatchTemplate", { defaultValue: "Select batch template" })} />
                  </SelectTrigger>
                  <SelectContent>
                    {templates.map((item) => (
                      <SelectItem key={item.id} value={item.id}>
                        {item.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            />
            <div className="grid gap-3 md:grid-cols-2">
              <Input
                {...batchExpiresAtField}
                type="datetime-local"
                disabled={!writable}
                onChange={(event) => {
                  batchExpiresAtField.onChange(event)
                  onBatchExpiresAtChange(event.target.value)
                }}
              />
              <Controller
                control={batchIssueForm.control}
                name="batch_execution_mode"
                render={({ field }) => (
                  <Select
                    value={field.value}
                    disabled={!writable}
                    onValueChange={(value: "inline" | "queued") => {
                      field.onChange(value)
                      onBatchExecutionModeChange(value)
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t("walletPage.components.issueQueue.executionMode", { defaultValue: "Execution mode" })} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="queued">{t("walletPage.components.issueQueue.executionQueued", { defaultValue: "Queued" })}</SelectItem>
                      <SelectItem value="inline">{t("walletPage.components.issueQueue.executionInline", { defaultValue: "Run immediately" })}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
            <Textarea
              {...batchTargetIDsField}
              disabled={!writable}
              onChange={(event) => {
                batchTargetIDsField.onChange(event)
                onBatchTargetIDsChange(event.target.value)
              }}
              placeholder={t("walletPage.components.issueQueue.batchTargetsPlaceholder", {
                defaultValue: "Enter multiple employee/visitor IDs, separated by newline/comma/semicolon",
              })}
              rows={6}
            />
            <div className="space-y-2">
              <p className="mp-kpi-note">
                {selectedBatchTemplate
                  ? t("walletPage.components.issueQueue.batchTemplateHint", {
                      defaultValue: "Current template scenario: {{scenario}}. {{hint}}",
                      scenario: getTemplateScenarioLabel(selectedBatchTemplate),
                      hint: getTemplateScenarioHint(selectedBatchTemplate),
                    })
                  : t("walletPage.components.issueQueue.batchDefaultHint", {
                      defaultValue:
                        "Will issue using {{targetType}} templates; suitable for reissue, onboarding batch activation, and temporary batch delivery.",
                      targetType: targetTypeLabel(batchTargetType),
                    })}
              </p>
              <Button
                type="submit"
                className="w-full"
                disabled={!writable || issuingBatch || !batchTemplateID || loading || refreshing || batchIssueForm.formState.isSubmitting}
              >
                {issuingBatch
                  ? t("walletPage.components.issueQueue.submittingBatch", { defaultValue: "Submitting..." })
                  : t("walletPage.components.issueQueue.submitBatchIssue", { defaultValue: "Submit batch issue" })}
              </Button>
            </div>
            {batchIssueFormError ? <p className="text-sm text-destructive">{batchIssueFormError}</p> : null}
          </form>
        </div>

        {lastIssuedJobs.length > 0 ? (
          <div className="rounded-xl border bg-muted/10 p-4" data-testid="wallet-recent-batch-receipts">
            <div className="mb-3 flex items-center justify-between gap-2">
              <p className="text-sm font-medium">{t("walletPage.components.issueQueue.recentBatchReceipts", { defaultValue: "Recent batch receipts" })}</p>
              <Badge variant="outline">
                {t("walletPage.components.issueQueue.receiptCount", {
                  defaultValue: "{{count}} records",
                  count: lastIssuedJobs.length,
                })}
              </Badge>
            </div>
            <div className="space-y-2">
              {lastIssuedJobs.slice(0, 5).map((item) => (
                <div
                  key={item.id}
                  data-testid={`wallet-recent-batch-receipt-${item.id}`}
                  className="flex flex-col gap-1 rounded-lg border bg-background px-3 py-2 text-sm lg:flex-row lg:items-center lg:justify-between"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-medium" data-testid="wallet-recent-batch-target">
                      {item.target_id}
                    </span>
                    <Badge
                      variant={item.status === "success" ? "secondary" : "outline"}
                      data-testid="wallet-recent-batch-status"
                    >
                      {item.status}
                    </Badge>
                    {item.error_code ? (
                      <Badge variant="destructive" data-testid="wallet-recent-batch-error">
                        {item.error_code}
                      </Badge>
                    ) : null}
                  </div>
                  <p className="mp-kpi-note">
                    retry {item.retry_count} · {formatDateTime(item.updated_at)}
                  </p>
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}
