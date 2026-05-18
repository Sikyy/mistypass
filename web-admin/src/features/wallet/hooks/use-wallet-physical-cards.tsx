import { useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import type { TFunction } from "i18next"
import {
  createWalletPhysicalCardTask,
  updateWalletPhysicalCardTaskStatus,
  type WalletPassInstance,
  type WalletPassTemplate,
  type WalletPhysicalCardTask,
} from "@/lib/api"
import {
  physicalCardTaskStatusLabel,
  physicalCardTaskTypeLabel,
} from "../pages/wallet-page-utils"

const physicalCardTaskTypeValues = ["issue", "reissue", "loss_report"] as const

function createPhysicalCardTaskSchema(t: TFunction) {
  return z.object({
    physical_task_pass_id: z.string().trim().min(1, t("walletPage.validation.physicalTask.passRequired")),
    physical_task_type: z.enum(physicalCardTaskTypeValues),
    physical_task_card_number: z
      .string()
      .trim()
      .max(128, t("walletPage.validation.physicalTask.cardNumberTooLong"))
      .optional()
      .or(z.literal("")),
    physical_task_note: z
      .string()
      .trim()
      .max(500, t("walletPage.validation.physicalTask.noteTooLong"))
      .optional()
      .or(z.literal("")),
  })
}

type PhysicalCardTaskFormValues = z.infer<ReturnType<typeof createPhysicalCardTaskSchema>>

type UseWalletPhysicalCardsParams = {
  token: string
  tenantID: string
  passes: WalletPassInstance[]
  templateByID: Map<string, WalletPassTemplate>
  employeeCardEligiblePasses: WalletPassInstance[]
  loadWalletOps: (tenantID: string) => Promise<void>
  setGlobalIssuanceSummary: (summary: string) => void
  setGlobalError: (error: string) => void
}

export function useWalletPhysicalCards({
  token,
  tenantID,
  passes,
  templateByID,
  employeeCardEligiblePasses,
  loadWalletOps,
  setGlobalIssuanceSummary,
  setGlobalError,
}: UseWalletPhysicalCardsParams) {
  const { t, i18n } = useTranslation()

  const [physicalCardTasks, setPhysicalCardTasks] = useState<WalletPhysicalCardTask[]>([])
  const [physicalTaskPassID, setPhysicalTaskPassID] = useState("")
  const [physicalTaskType, setPhysicalTaskType] = useState<"issue" | "reissue" | "loss_report">("issue")
  const [physicalTaskCardNumber, setPhysicalTaskCardNumber] = useState("")
  const [physicalTaskNote, setPhysicalTaskNote] = useState("")

  const [creatingPhysicalCardTask, setCreatingPhysicalCardTask] = useState(false)
  const [updatingPhysicalCardTaskID, setUpdatingPhysicalCardTaskID] = useState("")

  const physicalCardTaskSchema = useMemo(() => createPhysicalCardTaskSchema(t), [t, i18n.language])
  const physicalCardTaskForm = useForm<PhysicalCardTaskFormValues>({
    resolver: zodResolver(physicalCardTaskSchema),
    values: {
      physical_task_pass_id: physicalTaskPassID,
      physical_task_type: physicalTaskType,
      physical_task_card_number: physicalTaskCardNumber,
      physical_task_note: physicalTaskNote,
    },
  })
  const physicalTaskCardNumberField = physicalCardTaskForm.register("physical_task_card_number")
  const physicalTaskNoteField = physicalCardTaskForm.register("physical_task_note")
  const physicalCardTaskFormError =
    physicalCardTaskForm.formState.errors.physical_task_pass_id?.message ||
    physicalCardTaskForm.formState.errors.physical_task_type?.message ||
    physicalCardTaskForm.formState.errors.physical_task_card_number?.message ||
    physicalCardTaskForm.formState.errors.physical_task_note?.message ||
    ""

  const selectedPhysicalTaskPass = useMemo(
    () => passes.find((item) => item.id === physicalTaskPassID) ?? null,
    [passes, physicalTaskPassID]
  )
  const selectedPhysicalTaskTemplate = useMemo(
    () => (selectedPhysicalTaskPass ? templateByID.get(selectedPhysicalTaskPass.template_id) : undefined),
    [selectedPhysicalTaskPass, templateByID]
  )
  const recentPhysicalCardTasks = useMemo(() => physicalCardTasks.slice(0, 6), [physicalCardTasks])

  async function submitPhysicalCardTask(payload: {
    passID: string
    taskType: "issue" | "reissue" | "loss_report"
    cardNumber: string
    note: string
  }): Promise<boolean> {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setGlobalError(t("walletPage.errors.tenantRequired"))
      return false
    }
    if (!payload.passID.trim()) {
      setGlobalError(t("walletPage.errors.physicalTaskPassRequired"))
      return false
    }

    setCreatingPhysicalCardTask(true)
    setGlobalIssuanceSummary("")
    setGlobalError("")
    try {
      const created = await createWalletPhysicalCardTask(token, {
        tenant_id: nextTenantID,
        pass_id: payload.passID,
        task_type: payload.taskType,
        card_number: payload.cardNumber.trim() || undefined,
        note: payload.note.trim() || undefined,
        actor: "web_admin.wallet.physical_card.create",
      })
      setGlobalIssuanceSummary(
        t("walletPage.summaries.physicalTaskCreated", {
          targetID: created.target_id,
          taskType: physicalCardTaskTypeLabel(t, created.task_type),
          status: physicalCardTaskStatusLabel(t, created.status),
        })
      )
      setPhysicalTaskCardNumber("")
      setPhysicalTaskNote("")
      await loadWalletOps(nextTenantID)
      return true
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.createPhysicalTaskFailed")
      setGlobalError(message)
      return false
    } finally {
      setCreatingPhysicalCardTask(false)
    }
  }

  async function onSubmitPhysicalCardTaskForm(values: PhysicalCardTaskFormValues) {
    await submitPhysicalCardTask({
      passID: values.physical_task_pass_id.trim(),
      taskType: values.physical_task_type,
      cardNumber: values.physical_task_card_number || "",
      note: values.physical_task_note || "",
    })
  }

  async function advancePhysicalCardTask(task: WalletPhysicalCardTask, status: string) {
    const nextTenantID = tenantID.trim()
    if (!nextTenantID) {
      setGlobalError(t("walletPage.errors.tenantRequired"))
      return
    }

    setUpdatingPhysicalCardTaskID(task.id)
    setGlobalIssuanceSummary("")
    setGlobalError("")
    try {
      const updated = await updateWalletPhysicalCardTaskStatus(token, task.id, {
        tenant_id: nextTenantID,
        status,
        card_number: task.card_number,
        note: task.note,
        actor: `web_admin.wallet.physical_card.${status}`,
      })
      setGlobalIssuanceSummary(
        t("walletPage.summaries.physicalTaskUpdated", {
          targetID: updated.target_id,
          taskType: physicalCardTaskTypeLabel(t, updated.task_type),
          status: physicalCardTaskStatusLabel(t, updated.status),
        })
      )
      await loadWalletOps(nextTenantID)
    } catch (err) {
      const message = err instanceof Error ? err.message : t("walletPage.errors.updatePhysicalTaskFailed")
      setGlobalError(message)
    } finally {
      setUpdatingPhysicalCardTaskID("")
    }
  }

  return {
    physicalCardTasks,
    setPhysicalCardTasks,
    physicalTaskPassID,
    setPhysicalTaskPassID,
    physicalTaskType,
    setPhysicalTaskType,
    physicalTaskCardNumber,
    setPhysicalTaskCardNumber,
    physicalTaskNote,
    setPhysicalTaskNote,
    creatingPhysicalCardTask,
    updatingPhysicalCardTaskID,
    physicalCardTaskForm,
    physicalTaskCardNumberField,
    physicalTaskNoteField,
    physicalCardTaskFormError,
    selectedPhysicalTaskPass,
    selectedPhysicalTaskTemplate,
    recentPhysicalCardTasks,
    onSubmitPhysicalCardTaskForm,
    advancePhysicalCardTask,
  }
}
