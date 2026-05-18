import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import {
  createWalletTemplate,
  updateWalletTemplateStatus,
  type WalletPassTemplate,
} from "@/lib/api"
import {
  createWalletScenarioCounters,
  inferTemplateScenario,
  parseStyleConfig,
  passTypeLabel,
  type WalletScenarioKind,
} from "../pages/wallet-page-utils"

const defaultTemplateStatus: "active" | "inactive" = "active"
const defaultTemplatePassType = "employee"

type UseWalletTemplatesParams = {
  token: string
  tenantID: string
  loadWalletOps: (tenantID: string) => Promise<void>
}

export function useWalletTemplates({ token, tenantID, loadWalletOps }: UseWalletTemplatesParams) {
  const { t } = useTranslation()

  const [templates, setTemplates] = useState<WalletPassTemplate[]>([])
  const [templateName, setTemplateName] = useState("")
  const [templatePassType, setTemplatePassType] = useState<"employee" | "visitor">(defaultTemplatePassType)
  const [templateClassID, setTemplateClassID] = useState("")
  const [templateStyleConfig, setTemplateStyleConfig] = useState("")
  const [templateStatus, setTemplateStatus] = useState<"active" | "inactive">(defaultTemplateStatus)

  const [creatingTemplate, setCreatingTemplate] = useState(false)
  const [updatingTemplateID, setUpdatingTemplateID] = useState("")
  const [issuanceSummary, setIssuanceSummary] = useState("")
  const [error, setError] = useState("")

  const templateByID = useMemo(() => new Map(templates.map((item) => [item.id, item])), [templates])

  const activeEmployeeTemplate = useMemo(
    () => templates.find((item) => item.pass_type === "employee" && item.status === "active") ?? null,
    [templates]
  )
  const activeVisitorTemplate = useMemo(
    () => templates.find((item) => item.pass_type === "visitor" && item.status === "active") ?? null,
    [templates]
  )

  const activeTemplateByScenario = useMemo(() => {
    const next = new Map<WalletScenarioKind, WalletPassTemplate>()
    for (const item of templates) {
      if (item.status !== "active") {
        continue
      }
      const scenario = inferTemplateScenario(item)
      if (!next.has(scenario)) {
        next.set(scenario, item)
      }
    }
    return next
  }, [templates])

  const templateScenarioCounts = useMemo(() => {
    const next = createWalletScenarioCounters()
    templates.forEach((item) => {
      next[inferTemplateScenario(item)] += 1
    })
    return next
  }, [templates])

  function pickDefaultTemplateID(items: WalletPassTemplate[]): string {
    return items.find((item) => item.status === "active")?.id ?? items[0]?.id ?? ""
  }

  function resolveTargetType(templateID: string): "user" | "visitor" {
    return templates.find((item) => item.id === templateID)?.pass_type === "visitor" ? "visitor" : "user"
  }

  async function submitTemplate(payload: {
    name: string
    classID: string
    passType: "employee" | "visitor"
    status: "active" | "inactive"
    styleConfig: string
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.name.trim()) {
      setError(t("walletPage.errors.templateNameRequired"))
      return false
    }

    setCreatingTemplate(true)
    setIssuanceSummary("")
    setError("")
    try {
      const created = await createWalletTemplate(token, {
        tenant_id: nextTenantID,
        name: payload.name.trim(),
        pass_type: payload.passType,
        class_id: payload.classID.trim() || undefined,
        style_config: parseStyleConfig(payload.styleConfig),
        status: payload.status,
        actor: "web_admin.wallet.template",
      })
      setIssuanceSummary(
        t("walletPage.summaries.templateCreated", { templateName: created.name, passType: passTypeLabel(t, created.pass_type) })
      )
      setTemplateName("")
      setTemplateClassID("")
      setTemplateStyleConfig("")
      setTemplateStatus(defaultTemplateStatus)
      setTemplatePassType(defaultTemplatePassType)
      await loadWalletOps(nextTenantID)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.createTemplateFailed")
      setError(message)
      return false
    } finally {
      setCreatingTemplate(false)
    }
  }

  async function toggleTemplateStatus(template: WalletPassTemplate) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setError(t("walletPage.errors.tenantRequired"))
      return
    }

    setUpdatingTemplateID(template.id)
    setIssuanceSummary("")
    setError("")
    try {
      const nextStatus = template.status === "active" ? "inactive" : "active"
      const updated = await updateWalletTemplateStatus(token, template.id, {
        tenant_id: nextTenantID,
        status: nextStatus,
        actor: "web_admin.wallet.template.status",
      })
      setIssuanceSummary(
        t("walletPage.summaries.templateStatusUpdated", {
          templateName: updated.name,
          status: updated.status === "active" ? t("walletPage.labels.templateStatus.enabled") : t("walletPage.labels.templateStatus.disabled"),
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.updateTemplateStatusFailed")
      setError(message)
    } finally {
      setUpdatingTemplateID("")
    }
  }

  return {
    templates,
    setTemplates,
    templateName,
    setTemplateName,
    templatePassType,
    setTemplatePassType,
    templateClassID,
    setTemplateClassID,
    templateStyleConfig,
    setTemplateStyleConfig,
    templateStatus,
    setTemplateStatus,
    templateByID,
    activeEmployeeTemplate,
    activeVisitorTemplate,
    activeTemplateByScenario,
    templateScenarioCounts,
    creatingTemplate,
    updatingTemplateID,
    issuanceSummary,
    setIssuanceSummary,
    error,
    setError,
    pickDefaultTemplateID,
    resolveTargetType,
    submitTemplate,
    toggleTemplateStatus,
    defaultTemplateStatus,
    defaultTemplatePassType,
  }
}
