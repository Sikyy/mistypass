import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import type { TFunction } from "i18next"
import {
  activateWalletPass,
  dispatchWalletPassDelivery,
  retryWalletPassDelivery,
  type WalletPassDeliveryNotification,
  type WalletPassInstance,
  type WalletPassTemplate,
} from "@/lib/api"
import {
  deliveryNotificationStatusLabel,
  inferTemplateScenario,
  parseReceiverValues,
  resolveEnterpriseTargetQuery,
  walletIssuanceScenarioPresetByID,
  type ReceiptRecoveryStatus,
  type WalletScenarioKind,
} from "../pages/wallet-page-utils"
import type { EnterpriseFlowContext } from "./use-wallet-tenants"

function createPassDeliverySchema(t: TFunction) {
  return z
    .object({
      delivery_pass_id: z.string().trim().min(1, t("walletPage.validation.delivery.passRequired")),
      delivery_email_enabled: z.boolean(),
      delivery_whatsapp_enabled: z.boolean(),
      delivery_email_recipients: z.string().max(50000, t("walletPage.validation.delivery.emailRecipientsTooLong")),
      delivery_whatsapp_recipients: z
        .string()
        .max(50000, t("walletPage.validation.delivery.whatsAppRecipientsTooLong")),
    })
    .superRefine((values, context) => {
      if (!values.delivery_email_enabled && !values.delivery_whatsapp_enabled) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["delivery_email_enabled"],
          message: t("walletPage.validation.delivery.channelRequired"),
        })
      }
    })
}

type PassDeliveryFormValues = z.infer<ReturnType<typeof createPassDeliverySchema>>

const defaultBatchExecutionMode = "queued"

type UseWalletDeliveryParams = {
  token: string
  tenantID: string
  passes: WalletPassInstance[]
  passByID: Map<string, WalletPassInstance>
  templateByID: Map<string, WalletPassTemplate>
  deliverablePasses: WalletPassInstance[]
  activeEmployeeTemplate: WalletPassTemplate | null
  activeVisitorTemplate: WalletPassTemplate | null
  enterpriseFlowContext: EnterpriseFlowContext | null
  passQuery: string
  loadWalletOps: (tenantID: string) => Promise<void>
  setGlobalIssuanceSummary: (summary: string) => void
  setGlobalError: (error: string) => void
  setBatchTemplateID: (id: string) => void
  setBatchTargetIDs: (ids: string) => void
  setBatchExecutionMode: (mode: "inline" | "queued") => void
  setPassTargetTypeFilter: (filter: "all" | "user" | "visitor") => void
  setPassTemplateFilter: (filter: string) => void
}

export function useWalletDelivery({
  token,
  tenantID,
  passes,
  passByID,
  templateByID,
  deliverablePasses,
  activeEmployeeTemplate,
  activeVisitorTemplate,
  enterpriseFlowContext,
  passQuery,
  loadWalletOps,
  setGlobalIssuanceSummary,
  setGlobalError,
  setBatchTemplateID,
  setBatchTargetIDs,
  setBatchExecutionMode,
  setPassTargetTypeFilter,
  setPassTemplateFilter,
}: UseWalletDeliveryParams) {
  const { t, i18n } = useTranslation()

  const [deliveryNotifications, setDeliveryNotifications] = useState<WalletPassDeliveryNotification[]>([])
  const [deliveryPassID, setDeliveryPassID] = useState("")
  const [deliveryEmailEnabled, setDeliveryEmailEnabled] = useState(true)
  const [deliveryWhatsAppEnabled, setDeliveryWhatsAppEnabled] = useState(false)
  const [deliveryEmailRecipients, setDeliveryEmailRecipients] = useState("")
  const [deliveryWhatsAppRecipients, setDeliveryWhatsAppRecipients] = useState("")

  const [dispatchingDelivery, setDispatchingDelivery] = useState(false)
  const [retryingDeliveryNotificationID, setRetryingDeliveryNotificationID] = useState("")
  const [batchRetryingDelivery, setBatchRetryingDelivery] = useState(false)
  const [repairingRetryablePasses, setRepairingRetryablePasses] = useState(false)

  const passDeliverySchema = useMemo(() => createPassDeliverySchema(t), [t, i18n.language])
  const passDeliveryForm = useForm<PassDeliveryFormValues>({
    resolver: zodResolver(passDeliverySchema),
    values: {
      delivery_pass_id: deliveryPassID,
      delivery_email_enabled: deliveryEmailEnabled,
      delivery_whatsapp_enabled: deliveryWhatsAppEnabled,
      delivery_email_recipients: deliveryEmailRecipients,
      delivery_whatsapp_recipients: deliveryWhatsAppRecipients,
    },
  })
  const deliveryEmailRecipientsField = passDeliveryForm.register("delivery_email_recipients")
  const deliveryWhatsAppRecipientsField = passDeliveryForm.register("delivery_whatsapp_recipients")
  const passDeliveryFormError =
    passDeliveryForm.formState.errors.delivery_pass_id?.message ||
    passDeliveryForm.formState.errors.delivery_email_enabled?.message ||
    passDeliveryForm.formState.errors.delivery_whatsapp_enabled?.message ||
    passDeliveryForm.formState.errors.delivery_email_recipients?.message ||
    passDeliveryForm.formState.errors.delivery_whatsapp_recipients?.message ||
    ""

  const selectedDeliveryPass = useMemo(
    () => passes.find((item) => item.id === deliveryPassID) ?? null,
    [deliveryPassID, passes]
  )
  const selectedDeliveryTemplate = useMemo(
    () => (selectedDeliveryPass ? templateByID.get(selectedDeliveryPass.template_id) : undefined),
    [selectedDeliveryPass, templateByID]
  )

  const recentDeliveryNotifications = useMemo(() => deliveryNotifications.slice(0, 6), [deliveryNotifications])
  const failedDeliveryNotifications = useMemo(
    () => deliveryNotifications.filter((item) => item.status === "failed"),
    [deliveryNotifications]
  )
  const nonRetryableFailedDeliveryNotifications = useMemo(
    () => failedDeliveryNotifications.filter((item) => !item.retryable),
    [failedDeliveryNotifications]
  )

  const deliveryRetryQuery = useMemo(() => {
    const targetHint = resolveEnterpriseTargetQuery(enterpriseFlowContext)
    return targetHint || passQuery.trim()
  }, [enterpriseFlowContext, passQuery])

  const retryableDeliveryNotifications = useMemo(() => {
    const q = deliveryRetryQuery.trim().toLowerCase()
    return deliveryNotifications
      .filter((item) => item.status === "failed" && item.retryable)
      .filter((item) => {
        if (!q) {
          return true
        }
        return (
          item.target_id.toLowerCase().includes(q) ||
          item.pass_id.toLowerCase().includes(q) ||
          item.id.toLowerCase().includes(q) ||
          (item.reason || "").toLowerCase().includes(q)
        )
      })
  }, [deliveryNotifications, deliveryRetryQuery])

  const batchRetryableDeliveryNotifications = useMemo(
    () => retryableDeliveryNotifications.slice(0, 20),
    [retryableDeliveryNotifications]
  )

  const retryableDeliveryPasses = useMemo(() => {
    if (batchRetryableDeliveryNotifications.length === 0) {
      return []
    }
    const retryablePassIDSet = new Set(batchRetryableDeliveryNotifications.map((item) => item.pass_id))
    return passes.filter((item) => retryablePassIDSet.has(item.id))
  }, [batchRetryableDeliveryNotifications, passes])

  const repairableRetryableDeliveryPasses = useMemo(
    () => retryableDeliveryPasses.filter((item) => item.status === "issued" || item.status === "suspended").slice(0, 20),
    [retryableDeliveryPasses]
  )

  const reissueTargetIDsByRetryableDelivery = useMemo(
    () =>
      Array.from(
        new Set(batchRetryableDeliveryNotifications.map((item) => item.target_id.trim()).filter(Boolean))
      ).slice(0, 20),
    [batchRetryableDeliveryNotifications]
  )

  const reissueTemplateByRetryableDelivery = useMemo(() => {
    const matchedTemplates: WalletPassTemplate[] = []
    const matchedTemplateIDSet = new Set<string>()
    retryableDeliveryPasses.forEach((item) => {
      const template = templateByID.get(item.template_id)
      if (!template || matchedTemplateIDSet.has(template.id)) {
        return
      }
      matchedTemplateIDSet.add(template.id)
      matchedTemplates.push(template)
    })
    const matchedActiveTemplate = matchedTemplates.find((item) => item.status === "active")
    if (matchedActiveTemplate) {
      return matchedActiveTemplate
    }
    const targetTypeHint = batchRetryableDeliveryNotifications[0]?.target_type
    if (targetTypeHint === "visitor" && activeVisitorTemplate) {
      return activeVisitorTemplate
    }
    if (targetTypeHint === "user" && activeEmployeeTemplate) {
      return activeEmployeeTemplate
    }
    return activeEmployeeTemplate || activeVisitorTemplate || matchedTemplates[0] || null
  }, [
    activeEmployeeTemplate,
    activeVisitorTemplate,
    batchRetryableDeliveryNotifications,
    retryableDeliveryPasses,
    templateByID,
  ])

  const receiptRecoveryFlowStatus: ReceiptRecoveryStatus =
    deliveryNotifications.length === 0 ? "pending" : failedDeliveryNotifications.length > 0 ? "attention" : "ready"
  const receiptSplitStatus: ReceiptRecoveryStatus =
    deliveryNotifications.length === 0 ? "pending" : failedDeliveryNotifications.length > 0 ? "attention" : "ready"
  const receiptRemediationStatus: ReceiptRecoveryStatus =
    failedDeliveryNotifications.length === 0 ? (deliveryNotifications.length === 0 ? "pending" : "ready") : "attention"
  const receiptReviewStatus: ReceiptRecoveryStatus =
    failedDeliveryNotifications.length === 0 ? (deliveryNotifications.length === 0 ? "pending" : "ready") : "attention"

  async function submitPassDelivery(payload: {
    passID: string
    emailEnabled: boolean
    whatsAppEnabled: boolean
    emailRecipients: string
    whatsAppRecipients: string
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setGlobalError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.passID.trim()) {
      setGlobalError(t("walletPage.errors.deliveryPassRequired"))
      return false
    }

    const channels: string[] = []
    if (payload.emailEnabled) {
      channels.push("email")
    }
    if (payload.whatsAppEnabled) {
      channels.push("whatsapp")
    }
    if (channels.length === 0) {
      setGlobalError(t("walletPage.errors.deliveryChannelRequired"))
      return false
    }

    setDispatchingDelivery(true)
    setGlobalIssuanceSummary("")
    setGlobalError("")
    try {
      const created = await dispatchWalletPassDelivery(token, {
        tenant_id: nextTenantID,
        pass_id: payload.passID,
        channels,
        email_recipients: payload.emailEnabled ? parseReceiverValues(payload.emailRecipients) : [],
        whatsapp_recipients: payload.whatsAppEnabled ? parseReceiverValues(payload.whatsAppRecipients) : [],
        actor: "web_admin.wallet.delivery.dispatch",
      })
      setGlobalIssuanceSummary(
        t("walletPage.summaries.deliverySubmitted", {
          targetID: created.target_id,
          status: deliveryNotificationStatusLabel(t, created.status),
          reasonSuffix: created.reason ? t("walletPage.summaries.reasonSuffix", { reason: created.reason }) : "",
        })
      )
      await loadWalletOps(nextTenantID)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.deliverySubmitFailed")
      setGlobalError(message)
      return false
    } finally {
      setDispatchingDelivery(false)
    }
  }

  async function onSubmitPassDeliveryForm(values: PassDeliveryFormValues) {
    await submitPassDelivery({
      passID: values.delivery_pass_id.trim(),
      emailEnabled: values.delivery_email_enabled,
      whatsAppEnabled: values.delivery_whatsapp_enabled,
      emailRecipients: values.delivery_email_recipients,
      whatsAppRecipients: values.delivery_whatsapp_recipients,
    })
  }

  async function retryDeliveryNotification(notificationID: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setGlobalError(t("walletPage.errors.tenantRequired"))
      return
    }

    setRetryingDeliveryNotificationID(notificationID)
    setGlobalIssuanceSummary("")
    setGlobalError("")
    try {
      const retried = await retryWalletPassDelivery(token, {
        tenant_id: nextTenantID,
        notification_id: notificationID,
        actor: "web_admin.wallet.delivery.retry",
      })
      setGlobalIssuanceSummary(
        t("walletPage.summaries.deliveryRetrySubmitted", {
          targetID: retried.target_id,
          status: deliveryNotificationStatusLabel(t, retried.status),
          reasonSuffix: retried.reason ? t("walletPage.summaries.reasonSuffix", { reason: retried.reason }) : "",
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.deliveryRetryFailed")
      setGlobalError(message)
    } finally {
      setRetryingDeliveryNotificationID("")
    }
  }

  async function retryDeliveryNotificationBatch() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setGlobalError(t("walletPage.errors.tenantRequired"))
      return
    }
    if (batchRetryableDeliveryNotifications.length === 0) {
      setGlobalIssuanceSummary(t("walletPage.summaries.noRetryableDeliveryChannels"))
      return
    }

    setBatchRetryingDelivery(true)
    setGlobalError("")
    setGlobalIssuanceSummary("")
    try {
      const settled = await Promise.allSettled(
        batchRetryableDeliveryNotifications.map((item) =>
          retryWalletPassDelivery(token, {
            tenant_id: nextTenantID,
            notification_id: item.id,
            actor: "web_admin.wallet.delivery.retry.batch",
          })
        )
      )
      const successCount = settled.filter((item) => item.status === "fulfilled").length
      const failedCount = settled.length - successCount
      setGlobalIssuanceSummary(
        t("walletPage.summaries.batchRetryDeliverySubmitted", {
          total: settled.length,
          successCount,
          failedSuffix: failedCount > 0 ? t("walletPage.summaries.batchRetryDeliveryFailedSuffix", { failedCount }) : "",
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.batchRetryDeliveryFailed")
      setGlobalError(message)
    } finally {
      setBatchRetryingDelivery(false)
    }
  }

  function seedBatchReissueFromRetryableDelivery() {
    if (reissueTargetIDsByRetryableDelivery.length === 0) {
      setGlobalIssuanceSummary(t("walletPage.summaries.noReissueTargets"))
      return
    }
    if (!reissueTemplateByRetryableDelivery) {
      setGlobalError(t("walletPage.errors.reissueTemplateMissing"))
      return
    }

    const scenarioPreset = walletIssuanceScenarioPresetByID.get(inferTemplateScenario(reissueTemplateByRetryableDelivery))
    const templateTargetType = reissueTemplateByRetryableDelivery.pass_type === "visitor" ? "visitor" : "user"
    setBatchTemplateID(reissueTemplateByRetryableDelivery.id)
    setBatchTargetIDs(reissueTargetIDsByRetryableDelivery.join("\n"))
    setBatchExecutionMode(scenarioPreset?.recommendedExecutionMode ?? (defaultBatchExecutionMode as "inline" | "queued"))
    setPassTargetTypeFilter(templateTargetType)
    setPassTemplateFilter(reissueTemplateByRetryableDelivery.id)
    setGlobalError("")
    setGlobalIssuanceSummary(
      t("walletPage.summaries.reissueDraftSeeded", {
        count: reissueTargetIDsByRetryableDelivery.length,
        templateName: reissueTemplateByRetryableDelivery.name,
        statusHint:
          reissueTemplateByRetryableDelivery.status === "active"
            ? t("walletPage.summaries.reissueTemplateReady")
            : t("walletPage.summaries.reissueTemplateInactive"),
      })
    )
  }

  async function repairRetryableDeliveryPassStatusBatch() {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setGlobalError(t("walletPage.errors.tenantRequired"))
      return
    }
    if (repairableRetryableDeliveryPasses.length === 0) {
      setGlobalIssuanceSummary(t("walletPage.summaries.noRepairableDeliveryPasses"))
      return
    }

    setRepairingRetryablePasses(true)
    setGlobalError("")
    setGlobalIssuanceSummary("")
    try {
      const settled = await Promise.allSettled(
        repairableRetryableDeliveryPasses.map((item) =>
          activateWalletPass(token, item.id, {
            tenant_id: nextTenantID,
            actor: "web_admin.wallet.pass.batch.repair_from_delivery",
          })
        )
      )
      const successCount = settled.filter((item) => item.status === "fulfilled").length
      const failedCount = settled.length - successCount
      setGlobalIssuanceSummary(
        t("walletPage.summaries.batchRepairPassStatusSubmitted", {
          total: settled.length,
          successCount,
          failedSuffix: failedCount > 0 ? t("walletPage.summaries.batchRepairPassStatusFailedSuffix", { failedCount }) : "",
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.batchRepairPassStatusFailed")
      setGlobalError(message)
    } finally {
      setRepairingRetryablePasses(false)
    }
  }

  return {
    deliveryNotifications,
    setDeliveryNotifications,
    deliveryPassID,
    setDeliveryPassID,
    deliveryEmailEnabled,
    setDeliveryEmailEnabled,
    deliveryWhatsAppEnabled,
    setDeliveryWhatsAppEnabled,
    deliveryEmailRecipients,
    setDeliveryEmailRecipients,
    deliveryWhatsAppRecipients,
    setDeliveryWhatsAppRecipients,
    dispatchingDelivery,
    retryingDeliveryNotificationID,
    batchRetryingDelivery,
    repairingRetryablePasses,
    passDeliveryForm,
    deliveryEmailRecipientsField,
    deliveryWhatsAppRecipientsField,
    passDeliveryFormError,
    selectedDeliveryPass,
    selectedDeliveryTemplate,
    recentDeliveryNotifications,
    failedDeliveryNotifications,
    nonRetryableFailedDeliveryNotifications,
    deliveryRetryQuery,
    retryableDeliveryNotifications,
    batchRetryableDeliveryNotifications,
    retryableDeliveryPasses,
    repairableRetryableDeliveryPasses,
    reissueTargetIDsByRetryableDelivery,
    reissueTemplateByRetryableDelivery,
    receiptRecoveryFlowStatus,
    receiptSplitStatus,
    receiptRemediationStatus,
    receiptReviewStatus,
    onSubmitPassDeliveryForm,
    retryDeliveryNotification,
    retryDeliveryNotificationBatch,
    seedBatchReissueFromRetryableDelivery,
    repairRetryableDeliveryPassStatusBatch,
  }
}
