import {
  type ComponentProps,
  type FormEvent,
  useEffect,
  useMemo,
  useState,
} from "react"
import {
  CheckCircle2Icon,
  KeyRoundIcon,
  Loader2Icon,
  SaveIcon,
  ShieldCheckIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  type WalletGoogleConfig,
  type WalletGoogleConfigStatus,
  type WalletGoogleConfigValidation,
} from "@/lib/api"

type BadgeVariant = ComponentProps<typeof Badge>["variant"]
type GoogleConfigDraft = {
  issuer_id: string
  service_account_email: string
  key_ref: string
  status: WalletGoogleConfigStatus
}

type WalletGoogleConfigCardProps = {
  writable: boolean
  readOnlyBoundaryHint: string
  tenantID: string
  config: WalletGoogleConfig | null
  validation: WalletGoogleConfigValidation | null
  loading: boolean
  refreshing: boolean
  loadingConfig: boolean
  savingConfig: boolean
  validatingConfig: boolean
  error: string
  formatDateTime: (value?: string) => string
  onSaveConfig: (payload: GoogleConfigDraft) => unknown
  onValidateConfig: (payload: Omit<GoogleConfigDraft, "status">) => unknown
}

const emptyDraft: GoogleConfigDraft = {
  issuer_id: "",
  service_account_email: "",
  key_ref: "",
  status: "inactive",
}

function statusVariant(status?: string): BadgeVariant {
  return status === "active" ? "success" : "outline"
}

function validationVariant(status?: string): BadgeVariant {
  if (status === "ok") {
    return "success"
  }
  if (status === "warn") {
    return "warning"
  }
  if (status === "error") {
    return "destructive"
  }
  return "outline"
}

export function WalletGoogleConfigCard({
  writable,
  readOnlyBoundaryHint,
  tenantID,
  config,
  validation,
  loading,
  refreshing,
  loadingConfig,
  savingConfig,
  validatingConfig,
  error,
  formatDateTime,
  onSaveConfig,
  onValidateConfig,
}: WalletGoogleConfigCardProps) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState<GoogleConfigDraft>(emptyDraft)
  const readOnlyDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : undefined
  const busy =
    loading || refreshing || loadingConfig || savingConfig || validatingConfig
  const saveDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : busy
      ? t("walletPage.disabledReasons.busy")
      : undefined
  const validationSummary = useMemo(() => {
    const items = validation?.items ?? []
    return {
      ok: items.filter((item) => item.status === "ok").length,
      warn: items.filter((item) => item.status === "warn").length,
      error: items.filter((item) => item.status === "error").length,
    }
  }, [validation?.items])

  useEffect(() => {
    if (!config) {
      setDraft(emptyDraft)
      return
    }
    setDraft({
      issuer_id: config.issuer_id,
      service_account_email: config.service_account_email,
      key_ref: config.key_ref,
      status: config.status === "active" ? "active" : "inactive",
    })
  }, [config])

  function submitConfig(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    onSaveConfig(draft)
  }

  function validateConfig() {
    onValidateConfig({
      issuer_id: draft.issuer_id,
      service_account_email: draft.service_account_email,
      key_ref: draft.key_ref,
    })
  }

  return (
    <Card data-testid="wallet-google-config-card">
      <CardHeader>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <CardTitle className="inline-flex items-center gap-2 text-base">
              <KeyRoundIcon className="size-4" />
              {t("walletPage.components.googleConfig.title")}
            </CardTitle>
            <CardDescription>
              {t("walletPage.components.googleConfig.description")}
            </CardDescription>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={config ? "secondary" : "outline"}>
              {config
                ? t("walletPage.components.googleConfig.configured")
                : t("walletPage.components.googleConfig.notConfigured")}
            </Badge>
            <Badge variant={statusVariant(config?.status)}>
              {config?.status === "active"
                ? t("walletPage.components.googleConfig.statusActive")
                : t("walletPage.components.googleConfig.statusInactive")}
            </Badge>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <form className="space-y-4" onSubmit={submitConfig}>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1.1fr)_160px]">
            <div className="space-y-2">
              <Label htmlFor="wallet-google-issuer-id">
                {t("walletPage.components.googleConfig.issuerID")}
              </Label>
              <Input
                id="wallet-google-issuer-id"
                value={draft.issuer_id}
                disabled={!writable || loadingConfig}
                title={readOnlyDisabledReason}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    issuer_id: event.target.value,
                  }))
                }
                placeholder="issuer_id"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="wallet-google-service-account-email">
                {t("walletPage.components.googleConfig.serviceAccountEmail")}
              </Label>
              <Input
                id="wallet-google-service-account-email"
                value={draft.service_account_email}
                disabled={!writable || loadingConfig}
                title={readOnlyDisabledReason}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    service_account_email: event.target.value,
                  }))
                }
                placeholder="wallet-issuer@project.iam.gserviceaccount.com"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="wallet-google-key-ref">
                {t("walletPage.components.googleConfig.keyRef")}
              </Label>
              <Input
                id="wallet-google-key-ref"
                value={draft.key_ref}
                disabled={!writable || loadingConfig}
                title={readOnlyDisabledReason}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    key_ref: event.target.value,
                  }))
                }
                placeholder="kms://wallet/google/issuer"
              />
            </div>
            <div className="space-y-2">
              <Label>{t("walletPage.components.googleConfig.status")}</Label>
              <Select
                value={draft.status}
                disabled={!writable || loadingConfig}
                onValueChange={(value) =>
                  setDraft((current) => ({
                    ...current,
                    status: value as WalletGoogleConfigStatus,
                  }))
                }
              >
                <SelectTrigger title={readOnlyDisabledReason}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="inactive">
                    {t("walletPage.components.googleConfig.statusInactive")}
                  </SelectItem>
                  <SelectItem value="active">
                    {t("walletPage.components.googleConfig.statusActive")}
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
            <div className="space-y-1">
              <p className="mp-kpi-note">
                {t("walletPage.components.googleConfig.tenant", {
                  tenantID: tenantID || "-",
                })}
              </p>
              <p className="mp-kpi-note">
                {t("walletPage.components.googleConfig.lastUpdated", {
                  time: formatDateTime(config?.updated_at),
                })}
              </p>
              {!writable ? (
                <p className="mp-kpi-note">
                  {t("walletPage.components.googleConfig.readOnlyHint")}
                  {readOnlyBoundaryHint}
                </p>
              ) : null}
              {error ? (
                <p className="text-destructive text-xs">{error}</p>
              ) : null}
            </div>
            <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
              <Button
                type="button"
                variant="outline"
                className="w-full gap-2 sm:w-auto"
                disabled={!writable || busy}
                title={saveDisabledReason}
                onClick={validateConfig}
              >
                {validatingConfig ? (
                  <Loader2Icon className="size-4 animate-spin" />
                ) : (
                  <ShieldCheckIcon className="size-4" />
                )}
                {validatingConfig
                  ? t("walletPage.components.googleConfig.validating")
                  : t("walletPage.components.googleConfig.validate")}
              </Button>
              <Button
                type="submit"
                className="w-full gap-2 sm:w-auto"
                disabled={!writable || busy}
                title={saveDisabledReason}
              >
                {savingConfig ? (
                  <Loader2Icon className="size-4 animate-spin" />
                ) : (
                  <SaveIcon className="size-4" />
                )}
                {savingConfig
                  ? t("walletPage.components.googleConfig.saving")
                  : t("walletPage.components.googleConfig.save")}
              </Button>
            </div>
          </div>
        </form>

        <div className="bg-muted/20 rounded-lg border px-3 py-3">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <p className="inline-flex items-center gap-2 text-sm font-medium">
              <CheckCircle2Icon className="size-4" />
              {t("walletPage.components.googleConfig.validationTitle")}
            </p>
            {validation ? (
              <div className="flex flex-wrap gap-2">
                <Badge variant={validation.valid ? "success" : "destructive"}>
                  {validation.valid
                    ? t("walletPage.components.googleConfig.validationPassed")
                    : t("walletPage.components.googleConfig.validationFailed")}
                </Badge>
                <Badge variant="outline">OK {validationSummary.ok}</Badge>
                <Badge variant="warning">Warn {validationSummary.warn}</Badge>
                <Badge variant="destructive">
                  Error {validationSummary.error}
                </Badge>
              </div>
            ) : null}
          </div>
          {validation ? (
            <div className="mt-3 space-y-2">
              <p className="mp-kpi-note">
                {t("walletPage.components.googleConfig.validationCheckedAt", {
                  time: formatDateTime(validation.checked_at),
                })}
              </p>
              <div className="grid gap-2 md:grid-cols-2">
                {validation.items.map((item) => (
                  <div
                    key={`${item.field}-${item.status}-${item.message}`}
                    className="bg-background rounded-md border px-3 py-2"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-sm font-medium">
                        {item.field}
                      </span>
                      <Badge variant={validationVariant(item.status)}>
                        {item.status}
                      </Badge>
                    </div>
                    <p className="text-muted-foreground mt-1 text-xs">
                      {item.message}
                    </p>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <p className="text-muted-foreground mt-2 text-xs">
              {loadingConfig
                ? t("walletPage.components.googleConfig.loading")
                : t("walletPage.components.googleConfig.noValidation")}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
