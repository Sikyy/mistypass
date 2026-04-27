import { zodResolver } from "@hookform/resolvers/zod"
import { useMemo } from "react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
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
import { type WalletPassTemplate } from "@/lib/api"

const templatePassTypeValues = ["employee", "visitor"] as const
const templateStatusValues = ["active", "inactive"] as const
function buildWalletTemplateSchema(t: (key: string) => string) {
  return z.object({
    template_name: z
      .string()
      .trim()
      .min(1, t("walletPage.components.templateManager.validation.templateNameRequired"))
      .max(128, t("walletPage.components.templateManager.validation.templateNameMax")),
    template_class_id: z
      .string()
      .trim()
      .max(128, t("walletPage.components.templateManager.validation.classIDMax"))
      .optional()
      .or(z.literal("")),
    template_pass_type: z.enum(templatePassTypeValues),
    template_status: z.enum(templateStatusValues),
    template_style_config: z
      .string()
      .max(20000, t("walletPage.components.templateManager.validation.styleConfigMax"))
      .optional()
      .or(z.literal("")),
  })
}

type WalletTemplateFormValues = z.infer<ReturnType<typeof buildWalletTemplateSchema>>

export type WalletTemplateSubmitPayload = {
  name: string
  classID: string
  passType: "employee" | "visitor"
  status: "active" | "inactive"
  styleConfig: string
}

type WalletTemplateManagerCardProps = {
  writable: boolean
  readOnlyBoundaryHint: string
  creatingTemplate: boolean
  loading: boolean
  refreshing: boolean
  templateName: string
  onTemplateNameChange: (value: string) => void
  templateClassID: string
  onTemplateClassIDChange: (value: string) => void
  templatePassType: "employee" | "visitor"
  onTemplatePassTypeChange: (value: "employee" | "visitor") => void
  templateStatus: "active" | "inactive"
  onTemplateStatusChange: (value: "active" | "inactive") => void
  templateStyleConfig: string
  onTemplateStyleConfigChange: (value: string) => void
  onSubmitTemplate: (payload: WalletTemplateSubmitPayload) => Promise<boolean>
  issuanceSummary: string
  templates: WalletPassTemplate[]
  onSetDefaultTemplate: (templateID: string) => void
  onToggleTemplateStatus: (template: WalletPassTemplate) => void
  updatingTemplateID: string
  templateStatusVariant: (status: string) => "outline" | "secondary"
  passTypeLabel: (type: string) => string
  getTemplateScenarioLabel: (template: WalletPassTemplate) => string
  formatDateTime: (value?: string) => string
}

export function WalletTemplateManagerCard({
  writable,
  readOnlyBoundaryHint,
  creatingTemplate,
  loading,
  refreshing,
  templateName,
  onTemplateNameChange,
  templateClassID,
  onTemplateClassIDChange,
  templatePassType,
  onTemplatePassTypeChange,
  templateStatus,
  onTemplateStatusChange,
  templateStyleConfig,
  onTemplateStyleConfigChange,
  onSubmitTemplate,
  issuanceSummary,
  templates,
  onSetDefaultTemplate,
  onToggleTemplateStatus,
  updatingTemplateID,
  templateStatusVariant,
  passTypeLabel,
  getTemplateScenarioLabel,
  formatDateTime,
}: WalletTemplateManagerCardProps) {
  const { t } = useTranslation()
  const walletTemplateSchema = useMemo(() => buildWalletTemplateSchema(t), [t])
  const templateForm = useForm<WalletTemplateFormValues>({
    resolver: zodResolver(walletTemplateSchema),
    values: {
      template_name: templateName,
      template_class_id: templateClassID,
      template_pass_type: templatePassType,
      template_status: templateStatus,
      template_style_config: templateStyleConfig,
    },
  })
  const templateNameField = templateForm.register("template_name")
  const templateClassIDField = templateForm.register("template_class_id")
  const templateStyleConfigField = templateForm.register("template_style_config")
  const templateFormError =
    templateForm.formState.errors.template_name?.message ||
    templateForm.formState.errors.template_class_id?.message ||
    templateForm.formState.errors.template_pass_type?.message ||
    templateForm.formState.errors.template_status?.message ||
    templateForm.formState.errors.template_style_config?.message ||
    ""
  const readOnlyDisabledReason = !writable ? t("walletPage.disabledReasons.readOnly") : undefined
  const templateSubmitDisabledReason = !writable
    ? t("walletPage.disabledReasons.readOnly")
    : creatingTemplate || templateForm.formState.isSubmitting
      ? t("walletPage.disabledReasons.busy")
      : loading || refreshing
        ? t("walletPage.disabledReasons.loading")
        : undefined

  async function onSubmitTemplateForm(values: WalletTemplateFormValues) {
    await onSubmitTemplate({
      name: values.template_name.trim(),
      classID: values.template_class_id?.trim() || "",
      passType: values.template_pass_type,
      status: values.template_status,
      styleConfig: values.template_style_config || "",
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("walletPage.components.templateManager.title")}</CardTitle>
        <CardDescription>
          {t("walletPage.components.templateManager.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <form className="space-y-3 rounded-xl border bg-muted/15 p-4" onSubmit={templateForm.handleSubmit(onSubmitTemplateForm)}>
          <div className="grid gap-3 md:grid-cols-2">
            <Input
              {...templateNameField}
              disabled={!writable}
              title={readOnlyDisabledReason}
              onChange={(event) => {
                templateNameField.onChange(event)
                onTemplateNameChange(event.target.value)
              }}
              placeholder={t("walletPage.components.templateManager.templateName")}
            />
            <Input
              {...templateClassIDField}
              disabled={!writable}
              title={readOnlyDisabledReason}
              onChange={(event) => {
                templateClassIDField.onChange(event)
                onTemplateClassIDChange(event.target.value)
              }}
              placeholder={t("walletPage.components.templateManager.classID")}
            />
          </div>

          <div className="grid gap-3 md:grid-cols-2">
            <Controller
              control={templateForm.control}
              name="template_pass_type"
              render={({ field }) => (
                <Select
                  value={field.value}
                  disabled={!writable}
                  onValueChange={(value: "employee" | "visitor") => {
                    field.onChange(value)
                    onTemplatePassTypeChange(value)
                  }}
                >
                  <SelectTrigger className="w-full min-w-0" title={readOnlyDisabledReason}>
                    <SelectValue placeholder={t("walletPage.components.templateManager.templateType")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="employee">{t("walletPage.components.templateManager.employeeTemplate")}</SelectItem>
                    <SelectItem value="visitor">{t("walletPage.components.templateManager.visitorTemplate")}</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />

            <Controller
              control={templateForm.control}
              name="template_status"
              render={({ field }) => (
                <Select
                  value={field.value}
                  disabled={!writable}
                  onValueChange={(value: "active" | "inactive") => {
                    field.onChange(value)
                    onTemplateStatusChange(value)
                  }}
                >
                  <SelectTrigger className="w-full min-w-0" title={readOnlyDisabledReason}>
                    <SelectValue placeholder={t("walletPage.components.templateManager.templateStatus")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="active">{t("walletPage.components.templateManager.statusActive")}</SelectItem>
                    <SelectItem value="inactive">{t("walletPage.components.templateManager.statusInactive")}</SelectItem>
                  </SelectContent>
                </Select>
              )}
            />
          </div>

          <Textarea
            {...templateStyleConfigField}
            disabled={!writable}
            title={readOnlyDisabledReason}
            onChange={(event) => {
              templateStyleConfigField.onChange(event)
              onTemplateStyleConfigChange(event.target.value)
            }}
            placeholder={t("walletPage.components.templateManager.styleConfigPlaceholder")}
            rows={4}
          />

          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="mp-kpi-note">
              {writable
                ? t("walletPage.components.templateManager.writableHint")
                : `${t("walletPage.components.templateManager.readOnlyHint")}${readOnlyBoundaryHint}`}
            </p>
            <Button
              type="submit"
              disabled={!writable || creatingTemplate || loading || refreshing || templateForm.formState.isSubmitting}
              title={templateSubmitDisabledReason}
            >
              {creatingTemplate
                ? t("walletPage.components.templateManager.creating")
                : t("walletPage.components.templateManager.createTemplate")}
            </Button>
          </div>
          {templateFormError ? <p className="text-sm text-destructive">{templateFormError}</p> : null}
        </form>

        {issuanceSummary ? (
          <div className="rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-700">
            {issuanceSummary}
          </div>
        ) : null}

        <div className="space-y-3">
          {loading ? (
            <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
              {t("walletPage.components.templateManager.loading")}
            </div>
          ) : null}
          {!loading && templates.length === 0 ? (
            <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
              {t("walletPage.components.templateManager.empty")}
            </div>
          ) : null}
          {!loading &&
            templates.map((item) => (
              <div
                key={item.id}
                className="flex flex-col gap-3 rounded-xl border bg-card/80 px-4 py-3 lg:flex-row lg:items-center lg:justify-between"
              >
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="font-medium">{item.name}</p>
                    <Badge variant={templateStatusVariant(item.status)}>
                      {item.status === "active"
                        ? t("walletPage.components.templateManager.statusActiveBadge")
                        : t("walletPage.components.templateManager.statusInactiveBadge")}
                    </Badge>
                    <Badge variant="outline">{passTypeLabel(item.pass_type)}</Badge>
                    <Badge variant="secondary">{getTemplateScenarioLabel(item)}</Badge>
                  </div>
                  <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                    <span>class_id: {item.class_id || "-"}</span>
                    <span>
                      {t("walletPage.components.templateManager.styleItems", {
                        count: Object.keys(item.style_config ?? {}).length,
                      })}
                    </span>
                    <span>
                      {t("walletPage.components.templateManager.updatedAt", {
                        time: formatDateTime(item.updated_at),
                      })}
                    </span>
                  </div>
                </div>
                <div className="flex w-full flex-wrap items-center gap-2 lg:w-auto">
                  <Button
                    size="sm"
                    variant="outline"
                    className="w-full sm:w-auto"
                    onClick={() => {
                      onSetDefaultTemplate(item.id)
                    }}
                  >
                    {t("walletPage.components.templateManager.setDefault")}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="w-full sm:w-auto"
                    onClick={() => {
                      onToggleTemplateStatus(item)
                    }}
                    disabled={!writable || updatingTemplateID === item.id}
                    title={
                      !writable
                        ? t("walletPage.disabledReasons.readOnly")
                        : updatingTemplateID === item.id
                          ? t("walletPage.disabledReasons.busy")
                          : undefined
                    }
                  >
                    {updatingTemplateID === item.id
                      ? t("walletPage.components.templateManager.processing")
                      : item.status === "active"
                        ? t("walletPage.components.templateManager.disable")
                        : t("walletPage.components.templateManager.enable")}
                  </Button>
                </div>
              </div>
            ))}
        </div>
      </CardContent>
    </Card>
  )
}
