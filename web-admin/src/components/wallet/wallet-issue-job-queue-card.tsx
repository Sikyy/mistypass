import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect, useMemo } from "react"
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
function buildSingleIssueSchema(t: (key: string) => string) {
  return z.object({
    single_template_id: z.string().trim().min(1, t("walletPage.components.issueQueue.validation.selectTemplateRequired")),
    single_target_id: z
      .string()
      .trim()
      .min(1, t("walletPage.components.issueQueue.validation.targetIDRequired"))
      .max(128, t("walletPage.components.issueQueue.validation.targetIDMax")),
    single_expires_at: z
      .string()
      .optional()
      .or(z.literal(""))
      .refine((value) => !value || !Number.isNaN(new Date(value).getTime()), t("walletPage.components.issueQueue.validation.expirationInvalid")),
  })
}

function buildBatchIssueSchema(t: (key: string) => string) {
  return z.object({
    batch_template_id: z.string().trim().min(1, t("walletPage.components.issueQueue.validation.selectBatchTemplateRequired")),
    batch_expires_at: z
      .string()
      .optional()
      .or(z.literal(""))
      .refine((value) => !value || !Number.isNaN(new Date(value).getTime()), t("walletPage.components.issueQueue.validation.expirationInvalid")),
    batch_execution_mode: z.enum(batchExecutionModeValues),
    batch_target_ids: z
      .string()
      .trim()
      .min(1, t("walletPage.components.issueQueue.validation.batchTargetsRequired"))
      .max(200000, t("walletPage.components.issueQueue.validation.batchTargetsMax"))
      .refine((value) => parseBatchTargetIDs(value).length > 0, t("walletPage.components.issueQueue.validation.batchTargetsRequired")),
  })
}

type SingleIssueFormValues = z.infer<ReturnType<typeof buildSingleIssueSchema>>
type BatchIssueFormValues = z.infer<ReturnType<typeof buildBatchIssueSchema>>

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
  const singleIssueSchema = useMemo(() => buildSingleIssueSchema(t), [t])
  const batchIssueSchema = useMemo(() => buildBatchIssueSchema(t), [t])
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

  useEffect(() => {
    batchIssueForm.setValue("batch_target_ids", batchTargetIDs, {
      shouldDirty: false,
      shouldTouch: false,
      shouldValidate: false,
    })
  }, [batchIssueForm, batchTargetIDs])
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
  const formFieldDisabledReason = !writable ? t("walletPage.disabledReasons.readOnly") : ""
  const flowActionDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : loading || refreshing
      ? t("walletPage.disabledReasons.loading")
      : ""
  const keepIssueReadyDisabledReason =
    flowActionDisabledReason ||
    (issueReadyEnterpriseMissingTargetIDs.length === 0 ? t("walletPage.disabledReasons.noIssueReadyTargets") : "")
  const keepMissingDisabledReason =
    flowActionDisabledReason ||
    (enterpriseBatchTargetStats.missingIDs.length === 0 ? t("walletPage.disabledReasons.noMissingTargets") : "")
  const restorePrefilledDisabledReason =
    flowActionDisabledReason ||
    (enterpriseBatchTargetStats.targetIDs.length === 0 ? t("walletPage.disabledReasons.noPrefilledTargets") : "")
  const singleIssueDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : issuingSingle || singleIssueForm.formState.isSubmitting
      ? t("walletPage.disabledReasons.issuing")
      : loading || refreshing
        ? t("walletPage.disabledReasons.loading")
        : !singleTemplateID
          ? t("walletPage.disabledReasons.selectTemplate")
          : !singleTargetID.trim()
            ? t("walletPage.disabledReasons.enterTargetID")
            : ""
  const batchIssueDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : issuingBatch || batchIssueForm.formState.isSubmitting
      ? t("walletPage.disabledReasons.issuing")
      : loading || refreshing
        ? t("walletPage.disabledReasons.loading")
        : !batchTemplateID
          ? t("walletPage.disabledReasons.selectTemplate")
          : parseBatchTargetIDs(batchTargetIDs).length === 0
            ? t("walletPage.disabledReasons.enterBatchTargets")
            : ""

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
        <CardTitle className="text-base">{t("walletPage.components.issueQueue.title")}</CardTitle>
        <CardDescription>
          {t("walletPage.components.issueQueue.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {enterpriseBatchTargetStats.targetIDs.length > 0 ? (
          <div className="rounded-xl border bg-muted/10 px-4 py-3">
            <div className="flex flex-col gap-2">
              <div className="space-y-0.5">
                <p className="text-sm font-medium">
                  {t("walletPage.components.issueQueue.enterpriseHitRate", {
                    rate: enterpriseBatchTargetStats.hitRate,
                  })}
                </p>
                <p className="mp-kpi-note">
                  {t("walletPage.components.issueQueue.enterprisePrefillSummary", {
                    targets: enterpriseBatchTargetStats.targetIDs.length,
                    matched: enterpriseBatchTargetStats.matchedIDs.length,
                    missing: enterpriseBatchTargetStats.missingIDs.length,
                  })}
                </p>
                {enterpriseBatchTargetStats.missingIDs.length > 0 ? (
                  <p className="mp-kpi-note">
                    {t("walletPage.components.issueQueue.enterpriseMissingBreakdown", {
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
                    {t("walletPage.components.issueQueue.enterpriseMissingDetailsTitle")}
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
                  title={keepIssueReadyDisabledReason || undefined}
                  onClick={onKeepIssueReadyEnterpriseTargets}
                >
                  {t("walletPage.components.issueQueue.keepIssueReadyOnly", {
                    count: issueReadyEnterpriseMissingTargetIDs.length,
                  })}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!writable || loading || refreshing || enterpriseBatchTargetStats.missingIDs.length === 0}
                  title={keepMissingDisabledReason || undefined}
                  onClick={onKeepMissingEnterpriseTargets}
                >
                  {t("walletPage.components.issueQueue.keepMissingOnly", {
                    count: enterpriseBatchTargetStats.missingIDs.length,
                  })}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={!writable || loading || refreshing}
                  title={restorePrefilledDisabledReason || undefined}
                  onClick={onRestoreEnterpriseTargets}
                >
                  {t("walletPage.components.issueQueue.restoreAllPrefilled", {
                    count: enterpriseBatchTargetStats.targetIDs.length,
                  })}
                </Button>
                {canOpenAccessReview ? (
                  <Button asChild size="sm" variant="outline">
                    <Link to={accessDirectoryReviewLink}>
                      {t("walletPage.components.issueQueue.goDirectoryReview")}
                    </Link>
                  </Button>
                ) : null}
                {canOpenEnterpriseReview ? (
                  <Button asChild size="sm" variant="outline">
                    <Link to={enterpriseAlertsIssueLink}>
                      {t("walletPage.components.issueQueue.goEnterpriseSyncIssue")}
                    </Link>
                  </Button>
                ) : null}
                {canOpenEnterpriseReview && hasWorkerAlertFlowHints ? (
                  <Button asChild size="sm" variant="outline">
                    <Link to={enterpriseSyncWorkerReviewLink}>
                      {t("walletPage.components.issueQueue.backToSyncReview")}
                    </Link>
                  </Button>
                ) : null}
                {flowActionDisabledReason ? (
                  <p className="w-full basis-full text-xs text-muted-foreground">{flowActionDisabledReason}</p>
                ) : null}
              </div>
            </div>
          </div>
        ) : null}
        <div className="grid gap-4 xl:grid-cols-2">
          <form className="space-y-3 rounded-xl border bg-muted/15 p-4" onSubmit={singleIssueForm.handleSubmit(onSubmitSingleIssueForm)}>
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium">{t("walletPage.components.issueQueue.singleIssue")}</p>
              <Badge variant="outline">{targetTypeLabel(singleTargetType)}</Badge>
            </div>
            <Controller
              control={singleIssueForm.control}
              name="single_template_id"
              render={({ field }) => (
                <Select
                  value={field.value}
                  disabled={!writable}
                  onValueChange={(value) => {
                    field.onChange(value)
                    onSingleTemplateIDChange(value)
                  }}
                >
                  <SelectTrigger title={formFieldDisabledReason || undefined}>
                    <SelectValue placeholder={t("walletPage.components.issueQueue.selectTemplate")} />
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
              title={formFieldDisabledReason || undefined}
              onChange={(event) => {
                singleTargetIDField.onChange(event)
                onSingleTargetIDChange(event.target.value)
              }}
              placeholder={
                singleTargetType === "visitor"
                  ? t("walletPage.components.issueQueue.visitorIDPlaceholder")
                  : t("walletPage.components.issueQueue.employeeIDPlaceholder")
              }
            />
            <Input
              {...singleExpiresAtField}
              type="datetime-local"
              disabled={!writable}
              title={formFieldDisabledReason || undefined}
              onChange={(event) => {
                singleExpiresAtField.onChange(event)
                onSingleExpiresAtChange(event.target.value)
              }}
            />
            <div className="space-y-2">
              <p className="mp-kpi-note">
                {selectedSingleTemplate
                  ? t("walletPage.components.issueQueue.singleTemplateHint", {
                      scenario: getTemplateScenarioLabel(selectedSingleTemplate),
                      hint: getTemplateScenarioHint(selectedSingleTemplate),
                    })
                  : t("walletPage.components.issueQueue.selectTemplateFirst")}
              </p>
              <Button
                type="submit"
                className="w-full"
                disabled={Boolean(singleIssueDisabledReason)}
                title={singleIssueDisabledReason || undefined}
              >
                {issuingSingle
                  ? t("walletPage.components.issueQueue.issuingSingle")
                  : t("walletPage.components.issueQueue.issueOnePass")}
              </Button>
              {singleIssueDisabledReason ? (
                <p className="text-xs text-muted-foreground">{singleIssueDisabledReason}</p>
              ) : null}
            </div>
            {singleIssueFormError ? <p className="text-sm text-destructive">{singleIssueFormError}</p> : null}
          </form>

          <form className="space-y-3 rounded-xl border bg-muted/15 p-4" onSubmit={batchIssueForm.handleSubmit(onSubmitBatchIssueForm)}>
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium">{t("walletPage.components.issueQueue.batchIssue")}</p>
              <Badge variant="outline">{targetTypeLabel(batchTargetType)}</Badge>
            </div>
            <Controller
              control={batchIssueForm.control}
              name="batch_template_id"
              render={({ field }) => (
                <Select
                  value={field.value}
                  disabled={!writable}
                  onValueChange={(value) => {
                    field.onChange(value)
                    onBatchTemplateIDChange(value)
                  }}
                >
                  <SelectTrigger title={formFieldDisabledReason || undefined}>
                    <SelectValue placeholder={t("walletPage.components.issueQueue.selectBatchTemplate")} />
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
                title={formFieldDisabledReason || undefined}
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
                    <SelectTrigger title={formFieldDisabledReason || undefined}>
                      <SelectValue placeholder={t("walletPage.components.issueQueue.executionMode")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="queued">{t("walletPage.components.issueQueue.executionQueued")}</SelectItem>
                      <SelectItem value="inline">{t("walletPage.components.issueQueue.executionInline")}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
            <Textarea
              {...batchTargetIDsField}
              value={batchTargetIDs}
              disabled={!writable}
              title={formFieldDisabledReason || undefined}
              onChange={(event) => {
                batchTargetIDsField.onChange(event)
                onBatchTargetIDsChange(event.target.value)
              }}
              placeholder={t("walletPage.components.issueQueue.batchTargetsPlaceholder")}
              rows={6}
            />
            <div className="space-y-2">
              <p className="mp-kpi-note">
                {selectedBatchTemplate
                  ? t("walletPage.components.issueQueue.batchTemplateHint", {
                      scenario: getTemplateScenarioLabel(selectedBatchTemplate),
                      hint: getTemplateScenarioHint(selectedBatchTemplate),
                    })
                  : t("walletPage.components.issueQueue.batchDefaultHint", {
                      targetType: targetTypeLabel(batchTargetType),
                    })}
              </p>
              <Button
                type="submit"
                className="w-full"
                disabled={Boolean(batchIssueDisabledReason)}
                title={batchIssueDisabledReason || undefined}
              >
                {issuingBatch
                  ? t("walletPage.components.issueQueue.submittingBatch")
                  : t("walletPage.components.issueQueue.submitBatchIssue")}
              </Button>
              {batchIssueDisabledReason ? (
                <p className="text-xs text-muted-foreground">{batchIssueDisabledReason}</p>
              ) : null}
            </div>
            {batchIssueFormError ? <p className="text-sm text-destructive">{batchIssueFormError}</p> : null}
          </form>
        </div>

        {lastIssuedJobs.length > 0 ? (
          <div className="rounded-xl border bg-muted/10 p-4" data-testid="wallet-recent-batch-receipts">
            <div className="mb-3 flex items-center justify-between gap-2">
              <p className="text-sm font-medium">{t("walletPage.components.issueQueue.recentBatchReceipts")}</p>
              <Badge variant="outline">
                {t("walletPage.components.issueQueue.receiptCount", {
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
