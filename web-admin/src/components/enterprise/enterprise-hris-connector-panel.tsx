import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect, useMemo, useState } from "react"
import type { TFunction } from "i18next"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { type EnterpriseHRISConnector, type EnterpriseHRISSecret } from "@/lib/api"

export type TalentaConnectorSaveInput = {
  credential_ref?: string
  credential_value?: string
  status: EnterpriseHRISConnector["status"]
  sync_strategy: EnterpriseHRISConnector["sync_strategy"]
  webhook_secret_ref?: string
  webhook_secret_value?: string
}

type EnterpriseHRISConnectorPanelProps = {
  apiBaseURL: string
  connectors: EnterpriseHRISConnector[]
  loading: boolean
  onSaveTalentaConnector: (input: TalentaConnectorSaveInput) => Promise<void>
  secrets: EnterpriseHRISSecret[]
  selectedTenantName?: string
  writable: boolean
}

function createTalentaConnectorSchema(t: TFunction) {
  return z
    .object({
      status: z.enum(["active", "inactive"]),
      sync_strategy: z.enum(["webhook", "pull", "hybrid"]),
      credential_mode: z.enum(["existing_ref", "inline_secret"]),
      credential_ref: z.string().trim().optional().or(z.literal("")),
      client_id: z.string().trim().optional().or(z.literal("")),
      client_secret: z.string().trim().optional().or(z.literal("")),
      base_url: z.string().trim().optional().or(z.literal("")),
      employee_path: z.string().trim().optional().or(z.literal("")),
      page_limit: z.string().trim().optional().or(z.literal("")),
      enable_incremental: z.boolean(),
      updated_after_param: z.string().trim().optional().or(z.literal("")),
      updated_before_param: z.string().trim().optional().or(z.literal("")),
      timestamp_format: z.string().trim().optional().or(z.literal("")),
      webhook_secret_mode: z.enum(["existing_ref", "inline_secret"]),
      webhook_secret_ref: z.string().trim().optional().or(z.literal("")),
      webhook_secret_value: z.string().trim().optional().or(z.literal("")),
    })
    .superRefine((value, context) => {
      if (value.credential_mode === "existing_ref" && !value.credential_ref?.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("enterpriseSyncWorkspace.hrisConnector.validation.credentialRefRequired"),
          path: ["credential_ref"],
        })
      }
      if (value.credential_mode === "inline_secret" && !value.client_id?.trim()) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("enterpriseSyncWorkspace.hrisConnector.validation.clientIDRequired"),
          path: ["client_id"],
        })
      }
      if (
        value.credential_mode === "inline_secret" &&
        value.sync_strategy !== "webhook" &&
        !value.client_secret?.trim()
      ) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          message: t("enterpriseSyncWorkspace.hrisConnector.validation.clientSecretRequired"),
          path: ["client_secret"],
        })
      }
      if (value.page_limit?.trim()) {
        const nextPageLimit = Number.parseInt(value.page_limit.trim(), 10)
        if (!Number.isFinite(nextPageLimit) || nextPageLimit <= 0) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            message: t("enterpriseSyncWorkspace.hrisConnector.validation.pageLimitPositive"),
            path: ["page_limit"],
          })
        }
      }
      if (value.sync_strategy !== "pull") {
        if (value.webhook_secret_mode === "existing_ref" && !value.webhook_secret_ref?.trim()) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            message: t("enterpriseSyncWorkspace.hrisConnector.validation.webhookSecretRefRequired"),
            path: ["webhook_secret_ref"],
          })
        }
        if (value.webhook_secret_mode === "inline_secret" && !value.webhook_secret_value?.trim()) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            message: t("enterpriseSyncWorkspace.hrisConnector.validation.webhookSecretValueRequired"),
            path: ["webhook_secret_value"],
          })
        }
      }
    })
}

type TalentaConnectorSchema = ReturnType<typeof createTalentaConnectorSchema>
type TalentaConnectorFormValues = z.infer<TalentaConnectorSchema>

function buildWebhookURL(apiBaseURL: string, connectorID: string) {
  const normalizedBaseURL = apiBaseURL.trim().replace(/\/+$/, "")
  if (!normalizedBaseURL || !connectorID.trim()) {
    return ""
  }
  return `${normalizedBaseURL}/api/v1/enterprise/hris-webhook/${connectorID.trim()}`
}

function buildDefaultTalentaValues(connector: EnterpriseHRISConnector | null): TalentaConnectorFormValues {
  return {
    status: connector?.status ?? "active",
    sync_strategy: connector?.sync_strategy ?? "hybrid",
    credential_mode: connector?.credential_ref ? "existing_ref" : "inline_secret",
    credential_ref: connector?.credential_ref ?? "",
    client_id: "",
    client_secret: "",
    base_url: "",
    employee_path: "",
    page_limit: "",
    enable_incremental: false,
    updated_after_param: "",
    updated_before_param: "",
    timestamp_format: "",
    webhook_secret_mode: connector?.webhook_secret_ref ? "existing_ref" : "inline_secret",
    webhook_secret_ref: connector?.webhook_secret_ref ?? "",
    webhook_secret_value: "",
  }
}

function normalizeConnectorBadgeVariant(status?: string): "outline" | "secondary" | "destructive" {
  switch (status) {
    case "active":
      return "outline"
    case "inactive":
      return "secondary"
    default:
      return "destructive"
  }
}

function secretKindLabel(secret: EnterpriseHRISSecret, t: TFunction) {
  switch (secret.kind) {
    case "connector_credential":
      return t("enterpriseSyncWorkspace.hrisConnector.secretKinds.credential")
    case "webhook_secret":
      return t("enterpriseSyncWorkspace.hrisConnector.secretKinds.webhook")
    default:
      return secret.kind
  }
}

type TalentaConnectorSaveErrorGuidance = {
  badgeLabel: string
  badgeVariant: "outline" | "secondary" | "destructive"
  summary: string
  suggestions: string[]
  title: string
}

function classifyTalentaConnectorSaveError({
  credentialMode,
  enableIncremental,
  message,
  showPullFields,
  webhookSecretMode,
  t,
}: {
  credentialMode: "existing_ref" | "inline_secret"
  enableIncremental: boolean
  message: string
  showPullFields: boolean
  webhookSecretMode: "existing_ref" | "inline_secret"
  t: TFunction
}): TalentaConnectorSaveErrorGuidance | null {
  const normalizedMessage = message.trim().toLowerCase()
  if (!normalizedMessage) {
    return null
  }

  const includesAny = (terms: string[]) => terms.some((term) => normalizedMessage.includes(term))
  const fixBadgeLabel = t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.labels.fixConfig")
  const retryBadgeLabel = t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.labels.retryLater")
  const reviewBadgeLabel = t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.labels.review")

  const transientIssue = includesAny([
    "429",
    "502",
    "503",
    "504",
    "connection refused",
    "connection reset",
    "deadline exceeded",
    "eof",
    "rate limit",
    "temporarily unavailable",
    "temporary unavailable",
    "timed out",
    "timeout",
    "too many requests",
    "unavailable",
    "upstream",
  ])
  if (transientIssue) {
    return {
      badgeLabel: retryBadgeLabel,
      badgeVariant: "secondary",
      title: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.titles.transient"),
      summary: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.summaries.transient"),
      suggestions: [
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.retryLater"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.avoidRepeatedSubmit"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.captureRawError"),
      ],
    }
  }

  const credentialRefIssue =
    credentialMode === "existing_ref" &&
    includesAny(["credential_ref", "credential ref", "tenant vault", "vault://"]) &&
    includesAny(["empty", "invalid", "missing", "not found", "required", "unknown"])
  if (credentialRefIssue) {
    return {
      badgeLabel: fixBadgeLabel,
      badgeVariant: "destructive",
      title: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.titles.credentialRef"),
      summary: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.summaries.credentialRef"),
      suggestions: [
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.checkCredentialRef"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.useInlineSecret"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.captureRawError"),
      ],
    }
  }

  const webhookRefIssue =
    webhookSecretMode === "existing_ref" &&
    includesAny(["webhook", "webhook_secret_ref", "webhook secret ref", "tenant vault", "vault://"]) &&
    includesAny(["empty", "invalid", "missing", "not found", "required", "unknown"])
  if (webhookRefIssue) {
    return {
      badgeLabel: fixBadgeLabel,
      badgeVariant: "destructive",
      title: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.titles.webhookRef"),
      summary: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.summaries.webhookRef"),
      suggestions: [
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.checkWebhookRef"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.useInlineSecret"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.captureRawError"),
      ],
    }
  }

  const webhookIssue = includesAny(["digest", "hmac", "signature", "webhook"])
  if (webhookIssue) {
    return {
      badgeLabel: fixBadgeLabel,
      badgeVariant: "destructive",
      title: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.titles.webhook"),
      summary: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.summaries.webhook"),
      suggestions: [
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.verifyWebhookSecret"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.confirmSyncStrategy"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.captureRawError"),
      ],
    }
  }

  const credentialIssue = includesAny([
    "401",
    "403",
    "auth",
    "client id",
    "client secret",
    "client_id",
    "client_secret",
    "credential",
    "forbidden",
    "token",
    "unauthorized",
  ])
  if (credentialIssue) {
    return {
      badgeLabel: fixBadgeLabel,
      badgeVariant: "destructive",
      title: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.titles.credentials"),
      summary: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.summaries.credentials"),
      suggestions: [
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.verifyTalentaCredentials"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.confirmSyncStrategy"),
        t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.captureRawError"),
      ],
    }
  }

  const pullIssue =
    showPullFields &&
    includesAny([
      "400",
      "422",
      "bad request",
      "base_url",
      "employee_path",
      "invalid",
      "page limit",
      "page_limit",
      "parse",
      "schema",
      "updated_after",
      "updated_before",
      "validation",
    ])
  if (pullIssue) {
    const suggestions = [
      t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.verifyPullFields"),
      enableIncremental
        ? t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.verifyIncrementalFields")
        : t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.confirmSyncStrategy"),
      t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.captureRawError"),
    ]
    return {
      badgeLabel: fixBadgeLabel,
      badgeVariant: "destructive",
      title: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.titles.pullConfig"),
      summary: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.summaries.pullConfig"),
      suggestions,
    }
  }

  return {
    badgeLabel: reviewBadgeLabel,
    badgeVariant: "outline",
    title: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.titles.generic"),
    summary: t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.summaries.generic"),
    suggestions: [
      t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.captureRawError"),
      t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actions.confirmSyncStrategy"),
    ],
  }
}

export function EnterpriseHRISConnectorPanel({
  apiBaseURL,
  connectors,
  loading,
  onSaveTalentaConnector,
  secrets,
  selectedTenantName,
  writable,
}: EnterpriseHRISConnectorPanelProps) {
  const { t } = useTranslation()
  const [saveError, setSaveError] = useState("")
  const [saving, setSaving] = useState(false)
  const schema = useMemo(() => createTalentaConnectorSchema(t), [t])
  const talentaConnector = useMemo(
    () => connectors.find((item) => item.vendor === "talenta") ?? null,
    [connectors]
  )
  const credentialSecrets = useMemo(
    () => secrets.filter((item) => item.kind === "connector_credential"),
    [secrets]
  )
  const webhookSecrets = useMemo(
    () => secrets.filter((item) => item.kind === "webhook_secret"),
    [secrets]
  )
  const otherConnectors = useMemo(
    () => connectors.filter((item) => item.vendor !== "talenta"),
    [connectors]
  )
  const defaultValues = useMemo(() => buildDefaultTalentaValues(talentaConnector), [talentaConnector])
  const form = useForm<TalentaConnectorFormValues>({
    resolver: zodResolver(schema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
    setSaveError("")
  }, [defaultValues, form])

  const syncStrategy = form.watch("sync_strategy")
  const credentialMode = form.watch("credential_mode")
  const webhookSecretMode = form.watch("webhook_secret_mode")
  const enableIncremental = form.watch("enable_incremental")
  const showPullFields = syncStrategy !== "webhook"
  const showWebhookFields = syncStrategy !== "pull"
  const saveErrorGuidance = useMemo(
    () =>
      classifyTalentaConnectorSaveError({
        credentialMode,
        enableIncremental,
        message: saveError,
        showPullFields,
        webhookSecretMode,
        t,
      }),
    [credentialMode, enableIncremental, saveError, showPullFields, webhookSecretMode, t]
  )
  const webhookURL = talentaConnector ? buildWebhookURL(apiBaseURL, talentaConnector.id) : ""
  const latestSecrets = useMemo(() => {
    return [...secrets]
      .sort((left, right) => {
        const leftTime = new Date(left.updated_at).getTime() || 0
        const rightTime = new Date(right.updated_at).getTime() || 0
        return rightTime - leftTime
      })
      .slice(0, 4)
  }, [secrets])

  const formError =
    form.formState.errors.status?.message ||
    form.formState.errors.sync_strategy?.message ||
    form.formState.errors.credential_ref?.message ||
    form.formState.errors.client_id?.message ||
    form.formState.errors.client_secret?.message ||
    form.formState.errors.page_limit?.message ||
    form.formState.errors.updated_after_param?.message ||
    form.formState.errors.webhook_secret_ref?.message ||
    form.formState.errors.webhook_secret_value?.message
  const formDisabledReason = !writable
    ? t("enterpriseSyncWorkspace.hrisConnector.form.disabledReadOnly")
    : loading
      ? t("enterpriseSyncWorkspace.hrisConnector.form.disabledLoading")
      : saving
        ? t("enterpriseSyncWorkspace.hrisConnector.form.disabledSaving")
        : ""
  const credentialRefDisabledReason =
    formDisabledReason ||
    (credentialSecrets.length === 0
      ? t("enterpriseSyncWorkspace.hrisConnector.form.disabledNoCredentialRefs")
      : "")
  const webhookSecretRefDisabledReason =
    formDisabledReason ||
    (webhookSecrets.length === 0
      ? t("enterpriseSyncWorkspace.hrisConnector.form.disabledNoWebhookSecretRefs")
      : "")
  const incrementalDisabledReason =
    formDisabledReason ||
    (credentialMode === "existing_ref"
      ? t("enterpriseSyncWorkspace.hrisConnector.form.disabledExistingCredentialRef")
      : "")
  const submitDisabledReason =
    formDisabledReason ||
    (form.formState.isSubmitting ? t("enterpriseSyncWorkspace.hrisConnector.form.disabledSaving") : "")

  async function onSubmit(values: TalentaConnectorFormValues) {
    const input: TalentaConnectorSaveInput = {
      status: values.status,
      sync_strategy: values.sync_strategy,
    }
    const credentialRef = values.credential_ref?.trim() || ""
    const clientID = values.client_id?.trim() || ""
    const clientSecret = values.client_secret?.trim() || ""
    const baseURL = values.base_url?.trim() || ""
    const employeePath = values.employee_path?.trim() || ""
    const pageLimit = values.page_limit?.trim() || ""
    const updatedAfterParam = values.updated_after_param?.trim() || ""
    const updatedBeforeParam = values.updated_before_param?.trim() || ""
    const timestampFormat = values.timestamp_format?.trim() || ""
    const webhookSecretRef = values.webhook_secret_ref?.trim() || ""
    const webhookSecretValue = values.webhook_secret_value?.trim() || ""

    if (values.credential_mode === "existing_ref") {
      input.credential_ref = credentialRef
    } else {
      const credentialPayload: Record<string, string | number> = {
        client_id: clientID,
      }
      if (clientSecret) {
        credentialPayload.client_secret = clientSecret
      }
      if (baseURL) {
        credentialPayload.base_url = baseURL
      }
      if (employeePath) {
        credentialPayload.employee_path = employeePath
      }
      if (pageLimit) {
        credentialPayload.page_limit = Number.parseInt(pageLimit, 10)
      }
      if (values.enable_incremental) {
        if (updatedAfterParam) {
          credentialPayload.updated_after_param = updatedAfterParam
        }
        if (updatedBeforeParam) {
          credentialPayload.updated_before_param = updatedBeforeParam
        }
        if (timestampFormat) {
          credentialPayload.timestamp_format = timestampFormat
        }
      }
      input.credential_value = JSON.stringify(credentialPayload)
    }

    if (showWebhookFields) {
      if (values.webhook_secret_mode === "existing_ref") {
        input.webhook_secret_ref = webhookSecretRef
      } else {
        input.webhook_secret_value = webhookSecretValue
      }
    }

    setSaving(true)
    setSaveError("")
    try {
      await onSaveTalentaConnector(input)
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : t("enterpriseSyncWorkspace.hrisConnector.messages.saveFailed")
      setSaveError(message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("enterpriseSyncWorkspace.hrisConnector.summary.title")}</CardTitle>
          <CardDescription>
            {t("enterpriseSyncWorkspace.hrisConnector.summary.description", {
              tenant: selectedTenantName || t("enterpriseSyncWorkspace.hrisConnector.summary.defaultTenant"),
            })}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="rounded-xl border bg-muted/10 px-4 py-3">
            <div className="flex flex-wrap items-center gap-2">
              <p className="font-medium">{t("enterpriseSyncWorkspace.hrisConnector.summary.talentaTitle")}</p>
              <Badge variant={normalizeConnectorBadgeVariant(talentaConnector?.status)}>
                {talentaConnector
                  ? t(`enterpriseSyncWorkspace.hrisConnector.status.${talentaConnector.status}`)
                  : t("enterpriseSyncWorkspace.hrisConnector.status.notConfigured")}
              </Badge>
              {talentaConnector ? (
                <Badge variant="secondary">
                  {t(`enterpriseSyncWorkspace.hrisConnector.syncStrategy.${talentaConnector.sync_strategy}`)}
                </Badge>
              ) : null}
            </div>
            <p className="mt-1 text-sm text-muted-foreground">
              {talentaConnector
                ? t("enterpriseSyncWorkspace.hrisConnector.summary.talentaConfigured", {
                    updatedAt: talentaConnector.updated_at,
                  })
                : t("enterpriseSyncWorkspace.hrisConnector.summary.talentaPending")}
            </p>
            <div className="mt-3 space-y-2 text-sm">
              <div className="rounded-lg border bg-background px-3 py-2">
                <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.hrisConnector.summary.webhookURL")}</p>
                <p className="mt-1 break-all font-mono text-xs">
                  {webhookURL || t("enterpriseSyncWorkspace.hrisConnector.summary.webhookURLPending")}
                </p>
              </div>
              <div className="grid gap-2 md:grid-cols-2">
                <div className="rounded-lg border bg-background px-3 py-2">
                  <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.hrisConnector.summary.credentialRef")}</p>
                  <p className="mt-1 break-all font-mono text-xs">
                    {talentaConnector?.credential_ref || t("enterpriseSyncWorkspace.hrisConnector.summary.none")}
                  </p>
                </div>
                <div className="rounded-lg border bg-background px-3 py-2">
                  <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.hrisConnector.summary.webhookSecretRef")}</p>
                  <p className="mt-1 break-all font-mono text-xs">
                    {talentaConnector?.webhook_secret_ref || t("enterpriseSyncWorkspace.hrisConnector.summary.none")}
                  </p>
                </div>
              </div>
              <div className="grid gap-2 md:grid-cols-2">
                <div className="rounded-lg border bg-background px-3 py-2">
                  <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.hrisConnector.summary.lastSyncAt")}</p>
                  <p className="mt-1 text-sm">
                    {talentaConnector?.last_sync_at || t("enterpriseSyncWorkspace.hrisConnector.summary.notSynced")}
                  </p>
                </div>
                <div className="rounded-lg border bg-background px-3 py-2">
                  <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.hrisConnector.summary.secretInventory")}</p>
                  <p className="mt-1 text-sm">
                    {t("enterpriseSyncWorkspace.hrisConnector.summary.secretInventoryValue", {
                      credentials: credentialSecrets.length,
                      webhooks: webhookSecrets.length,
                    })}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <div className="rounded-xl border bg-muted/10 px-4 py-3">
            <div className="flex flex-wrap items-center gap-2">
              <p className="font-medium">{t("enterpriseSyncWorkspace.hrisConnector.summary.secretRefsTitle")}</p>
              <Badge variant="outline">{t("enterpriseSyncWorkspace.hrisConnector.summary.secretRefsCount", { count: secrets.length })}</Badge>
            </div>
            <div className="mt-3 space-y-2">
              {latestSecrets.length > 0 ? (
                latestSecrets.map((item) => (
                  <div key={item.ref} className="rounded-lg border bg-background px-3 py-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="font-medium">{item.name}</p>
                      <Badge variant="secondary">{secretKindLabel(item, t)}</Badge>
                    </div>
                    <p className="mt-1 break-all font-mono text-xs text-muted-foreground">{item.ref}</p>
                  </div>
                ))
              ) : (
                <p className="text-sm text-muted-foreground">
                  {t("enterpriseSyncWorkspace.hrisConnector.summary.secretRefsEmpty")}
                </p>
              )}
            </div>
          </div>

          {otherConnectors.length > 0 ? (
            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <p className="font-medium">{t("enterpriseSyncWorkspace.hrisConnector.summary.otherConnectorsTitle")}</p>
              <div className="mt-3 space-y-2">
                {otherConnectors.map((item) => (
                  <div key={item.id} className="rounded-lg border bg-background px-3 py-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="font-medium">{item.vendor}</p>
                      <Badge variant={normalizeConnectorBadgeVariant(item.status)}>
                        {t(`enterpriseSyncWorkspace.hrisConnector.status.${item.status}`)}
                      </Badge>
                    </div>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {t(`enterpriseSyncWorkspace.hrisConnector.syncStrategy.${item.sync_strategy}`)}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("enterpriseSyncWorkspace.hrisConnector.form.title")}</CardTitle>
          <CardDescription>{t("enterpriseSyncWorkspace.hrisConnector.form.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-2">
                <Label>{t("enterpriseSyncWorkspace.hrisConnector.form.statusLabel")}</Label>
                <Controller
                  control={form.control}
                  name="status"
                  render={({ field }) => (
                    <Select value={field.value} onValueChange={field.onChange}>
                      <SelectTrigger disabled={Boolean(formDisabledReason)} title={formDisabledReason || undefined}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="active">{t("enterpriseSyncWorkspace.hrisConnector.status.active")}</SelectItem>
                        <SelectItem value="inactive">{t("enterpriseSyncWorkspace.hrisConnector.status.inactive")}</SelectItem>
                      </SelectContent>
                    </Select>
                  )}
                />
              </div>
              <div className="space-y-2">
                <Label>{t("enterpriseSyncWorkspace.hrisConnector.form.syncStrategyLabel")}</Label>
                <Controller
                  control={form.control}
                  name="sync_strategy"
                  render={({ field }) => (
                    <Select value={field.value} onValueChange={field.onChange}>
                      <SelectTrigger disabled={Boolean(formDisabledReason)} title={formDisabledReason || undefined}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="hybrid">{t("enterpriseSyncWorkspace.hrisConnector.syncStrategy.hybrid")}</SelectItem>
                        <SelectItem value="webhook">{t("enterpriseSyncWorkspace.hrisConnector.syncStrategy.webhook")}</SelectItem>
                        <SelectItem value="pull">{t("enterpriseSyncWorkspace.hrisConnector.syncStrategy.pull")}</SelectItem>
                      </SelectContent>
                    </Select>
                  )}
                />
              </div>
            </div>

            <div className="rounded-xl border bg-muted/10 px-4 py-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="font-medium">{t("enterpriseSyncWorkspace.hrisConnector.form.credentialTitle")}</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {t("enterpriseSyncWorkspace.hrisConnector.form.credentialDescription")}
                  </p>
                </div>
                <Controller
                  control={form.control}
                  name="credential_mode"
                  render={({ field }) => (
                    <Select value={field.value} onValueChange={field.onChange}>
                      <SelectTrigger className="w-[220px]" disabled={Boolean(formDisabledReason)} title={formDisabledReason || undefined}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="inline_secret">
                          {t("enterpriseSyncWorkspace.hrisConnector.form.secretMode.inline")}
                        </SelectItem>
                        <SelectItem value="existing_ref">
                          {t("enterpriseSyncWorkspace.hrisConnector.form.secretMode.existing")}
                        </SelectItem>
                      </SelectContent>
                    </Select>
                  )}
                />
              </div>

              {credentialMode === "existing_ref" ? (
                <div className="mt-3 space-y-2">
                  <Label>{t("enterpriseSyncWorkspace.hrisConnector.form.credentialRefLabel")}</Label>
                  <Controller
                    control={form.control}
                    name="credential_ref"
                    render={({ field }) => (
                      <Select value={field.value || undefined} onValueChange={field.onChange}>
                        <SelectTrigger disabled={Boolean(credentialRefDisabledReason)} title={credentialRefDisabledReason || undefined}>
                          <SelectValue
                            placeholder={t("enterpriseSyncWorkspace.hrisConnector.form.credentialRefPlaceholder")}
                          />
                        </SelectTrigger>
                        <SelectContent>
                          {credentialSecrets.map((item) => (
                            <SelectItem key={item.ref} value={item.ref}>
                              {item.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    )}
                  />
                  <p className="mp-kpi-note">
                    {credentialSecrets.length > 0
                      ? t("enterpriseSyncWorkspace.hrisConnector.form.credentialRefHint")
                      : t("enterpriseSyncWorkspace.hrisConnector.form.secretRefsEmpty")}
                  </p>
                </div>
              ) : (
                <div className="mt-3 grid gap-3 md:grid-cols-2">
                  <div className="space-y-2">
                    <Label htmlFor="talenta-client-id">
                      {t("enterpriseSyncWorkspace.hrisConnector.form.clientIDLabel")}
                    </Label>
                    <Controller
                      control={form.control}
                      name="client_id"
                      render={({ field }) => (
                        <Input
                          id="talenta-client-id"
                          name={field.name}
                          value={field.value ?? ""}
                          onBlur={field.onBlur}
                          onChange={field.onChange}
                          placeholder="mekari-client-id"
                          disabled={Boolean(formDisabledReason)}
                          title={formDisabledReason || undefined}
                        />
                      )}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="talenta-client-secret">
                      {t("enterpriseSyncWorkspace.hrisConnector.form.clientSecretLabel")}
                    </Label>
                    <Controller
                      control={form.control}
                      name="client_secret"
                      render={({ field }) => (
                        <Input
                          id="talenta-client-secret"
                          name={field.name}
                          value={field.value ?? ""}
                          onBlur={field.onBlur}
                          onChange={field.onChange}
                          type="password"
                          placeholder={
                            syncStrategy === "webhook"
                              ? t("enterpriseSyncWorkspace.hrisConnector.form.clientSecretOptional")
                              : "mekari-client-secret"
                          }
                          disabled={Boolean(formDisabledReason)}
                          title={formDisabledReason || undefined}
                        />
                      )}
                    />
                  </div>
                  {showPullFields ? (
                    <>
                      <div className="space-y-2">
                        <Label htmlFor="talenta-base-url">
                          {t("enterpriseSyncWorkspace.hrisConnector.form.baseURLLabel")}
                        </Label>
                        <Controller
                          control={form.control}
                          name="base_url"
                          render={({ field }) => (
                            <Input
                              id="talenta-base-url"
                              name={field.name}
                              value={field.value ?? ""}
                              onBlur={field.onBlur}
                              onChange={field.onChange}
                              placeholder="https://api.mekari.com"
                              disabled={Boolean(formDisabledReason)}
                              title={formDisabledReason || undefined}
                            />
                          )}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="talenta-employee-path">
                          {t("enterpriseSyncWorkspace.hrisConnector.form.employeePathLabel")}
                        </Label>
                        <Controller
                          control={form.control}
                          name="employee_path"
                          render={({ field }) => (
                            <Input
                              id="talenta-employee-path"
                              name={field.name}
                              value={field.value ?? ""}
                              onBlur={field.onBlur}
                              onChange={field.onChange}
                              placeholder="/v2/talenta/v2/employee"
                              disabled={Boolean(formDisabledReason)}
                              title={formDisabledReason || undefined}
                            />
                          )}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="talenta-page-limit">
                          {t("enterpriseSyncWorkspace.hrisConnector.form.pageLimitLabel")}
                        </Label>
                        <Controller
                          control={form.control}
                          name="page_limit"
                          render={({ field }) => (
                            <Input
                              id="talenta-page-limit"
                              name={field.name}
                              value={field.value ?? ""}
                              onBlur={field.onBlur}
                              onChange={field.onChange}
                              inputMode="numeric"
                              placeholder="20"
                              disabled={Boolean(formDisabledReason)}
                              title={formDisabledReason || undefined}
                            />
                          )}
                        />
                      </div>
                    </>
                  ) : null}
                </div>
              )}
            </div>

            {showPullFields ? (
              <div className="rounded-xl border bg-muted/10 px-4 py-3">
                <div className="flex items-center justify-between gap-3">
                  <div>
                    <p className="font-medium">{t("enterpriseSyncWorkspace.hrisConnector.form.incrementalTitle")}</p>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {t("enterpriseSyncWorkspace.hrisConnector.form.incrementalDescription")}
                    </p>
                  </div>
                  <Controller
                    control={form.control}
                    name="enable_incremental"
                    render={({ field }) => (
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={Boolean(incrementalDisabledReason)}
                        title={incrementalDisabledReason || undefined}
                      />
                    )}
                  />
                </div>
                <p className="mt-3 text-sm text-muted-foreground">
                  {credentialMode === "existing_ref"
                    ? t("enterpriseSyncWorkspace.hrisConnector.form.incrementalRefHint")
                    : enableIncremental
                      ? t("enterpriseSyncWorkspace.hrisConnector.form.incrementalEnabledHint")
                      : t("enterpriseSyncWorkspace.hrisConnector.form.incrementalDisabledHint")}
                </p>
                {enableIncremental ? (
                  <div className="mt-3 grid gap-3 md:grid-cols-2">
                    <div className="space-y-2">
                      <Label htmlFor="talenta-updated-after">
                        {t("enterpriseSyncWorkspace.hrisConnector.form.updatedAfterLabel")}
                      </Label>
                      <Controller
                        control={form.control}
                        name="updated_after_param"
                        render={({ field }) => (
                          <Input
                            id="talenta-updated-after"
                            name={field.name}
                            value={field.value ?? ""}
                            onBlur={field.onBlur}
                            onChange={field.onChange}
                            placeholder="updated_after"
                            disabled={Boolean(incrementalDisabledReason)}
                            title={incrementalDisabledReason || undefined}
                          />
                        )}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label htmlFor="talenta-updated-before">
                        {t("enterpriseSyncWorkspace.hrisConnector.form.updatedBeforeLabel")}
                      </Label>
                      <Controller
                        control={form.control}
                        name="updated_before_param"
                        render={({ field }) => (
                          <Input
                            id="talenta-updated-before"
                            name={field.name}
                            value={field.value ?? ""}
                            onBlur={field.onBlur}
                            onChange={field.onChange}
                            placeholder="updated_before"
                            disabled={Boolean(incrementalDisabledReason)}
                            title={incrementalDisabledReason || undefined}
                          />
                        )}
                      />
                    </div>
                    <div className="space-y-2 md:col-span-2">
                      <Label htmlFor="talenta-timestamp-format">
                        {t("enterpriseSyncWorkspace.hrisConnector.form.timestampFormatLabel")}
                      </Label>
                      <Controller
                        control={form.control}
                        name="timestamp_format"
                        render={({ field }) => (
                          <Input
                            id="talenta-timestamp-format"
                            name={field.name}
                            value={field.value ?? ""}
                            onBlur={field.onBlur}
                            onChange={field.onChange}
                            placeholder={t("enterpriseSyncWorkspace.hrisConnector.form.timestampFormatPlaceholder")}
                            disabled={Boolean(incrementalDisabledReason)}
                            title={incrementalDisabledReason || undefined}
                          />
                        )}
                      />
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}

            {showWebhookFields ? (
              <div className="rounded-xl border bg-muted/10 px-4 py-3">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <p className="font-medium">{t("enterpriseSyncWorkspace.hrisConnector.form.webhookTitle")}</p>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {t("enterpriseSyncWorkspace.hrisConnector.form.webhookDescription")}
                    </p>
                  </div>
                  <Controller
                    control={form.control}
                    name="webhook_secret_mode"
                    render={({ field }) => (
                      <Select value={field.value} onValueChange={field.onChange}>
                        <SelectTrigger className="w-[220px]" disabled={Boolean(formDisabledReason)} title={formDisabledReason || undefined}>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="inline_secret">
                            {t("enterpriseSyncWorkspace.hrisConnector.form.secretMode.inline")}
                          </SelectItem>
                          <SelectItem value="existing_ref">
                            {t("enterpriseSyncWorkspace.hrisConnector.form.secretMode.existing")}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                    )}
                  />
                </div>
                <div className="mt-3 space-y-2">
                  <Label>{t("enterpriseSyncWorkspace.hrisConnector.form.webhookURLLabel")}</Label>
                  <Input
                    readOnly
                    value={webhookURL || t("enterpriseSyncWorkspace.hrisConnector.form.webhookURLPending")}
                    disabled
                  />
                </div>
                {webhookSecretMode === "existing_ref" ? (
                  <div className="mt-3 space-y-2">
                    <Label>{t("enterpriseSyncWorkspace.hrisConnector.form.webhookSecretRefLabel")}</Label>
                    <Controller
                      control={form.control}
                      name="webhook_secret_ref"
                      render={({ field }) => (
                        <Select value={field.value || undefined} onValueChange={field.onChange}>
                          <SelectTrigger disabled={Boolean(webhookSecretRefDisabledReason)} title={webhookSecretRefDisabledReason || undefined}>
                            <SelectValue
                              placeholder={t("enterpriseSyncWorkspace.hrisConnector.form.webhookSecretRefPlaceholder")}
                            />
                          </SelectTrigger>
                          <SelectContent>
                            {webhookSecrets.map((item) => (
                              <SelectItem key={item.ref} value={item.ref}>
                                {item.name}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      )}
                    />
                  </div>
                ) : (
                  <div className="mt-3 space-y-2">
                    <Label htmlFor="talenta-webhook-secret">
                      {t("enterpriseSyncWorkspace.hrisConnector.form.webhookSecretValueLabel")}
                    </Label>
                    <Controller
                      control={form.control}
                      name="webhook_secret_value"
                      render={({ field }) => (
                        <Input
                          id="talenta-webhook-secret"
                          name={field.name}
                          value={field.value ?? ""}
                          onBlur={field.onBlur}
                          onChange={field.onChange}
                          type="password"
                          placeholder="mekari-webhook-secret"
                          disabled={Boolean(formDisabledReason)}
                          title={formDisabledReason || undefined}
                        />
                      )}
                    />
                    <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.hrisConnector.form.webhookSecretHint")}</p>
                  </div>
                )}
              </div>
            ) : null}

            {!writable ? (
              <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.hrisConnector.form.readonlyHint")}</p>
            ) : null}
            {formError ? <p className="text-sm text-destructive">{formError}</p> : null}
            {submitDisabledReason ? (
              <p className="mp-kpi-note">{submitDisabledReason}</p>
            ) : null}
            {saveErrorGuidance ? (
              <div
                className="rounded-xl border border-destructive/25 bg-destructive/5 px-4 py-3 space-y-3"
                data-testid="enterprise-talenta-save-error-guidance"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <p className="font-medium" data-testid="enterprise-talenta-save-error-title">
                    {saveErrorGuidance.title}
                  </p>
                  <Badge
                    variant={saveErrorGuidance.badgeVariant}
                    data-testid="enterprise-talenta-save-error-badge"
                  >
                    {saveErrorGuidance.badgeLabel}
                  </Badge>
                </div>
                <p className="text-sm text-muted-foreground">{saveErrorGuidance.summary}</p>
                <div className="rounded-lg border bg-background/80 px-3 py-2">
                  <p className="mp-kpi-note">{t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.rawError")}</p>
                  <p className="mt-1 text-sm text-destructive" data-testid="enterprise-talenta-save-error-raw">
                    {saveError}
                  </p>
                </div>
                <div className="space-y-2">
                  <p className="mp-kpi-note">
                    {t("enterpriseSyncWorkspace.hrisConnector.messages.errorGuidance.actionsTitle")}
                  </p>
                  <ul className="list-disc space-y-1 pl-5 text-sm text-muted-foreground">
                    {saveErrorGuidance.suggestions.map((suggestion, index) => (
                      <li
                        key={`${index}-${suggestion}`}
                        data-testid={`enterprise-talenta-save-error-suggestion-${index}`}
                      >
                        {suggestion}
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            ) : null}
            <div className="flex justify-end">
              <Button type="submit" disabled={Boolean(submitDisabledReason)} title={submitDisabledReason || undefined}>
                {saving
                  ? t("enterpriseSyncWorkspace.hrisConnector.form.saving")
                  : talentaConnector
                    ? t("enterpriseSyncWorkspace.hrisConnector.form.update")
                    : t("enterpriseSyncWorkspace.hrisConnector.form.create")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
