import { type ComponentProps } from "react"
import { Controller, type UseFormRegisterReturn, type UseFormReturn } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"

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
import { type WalletPassInstance, type WalletPassTemplate, type WalletPhysicalCardTask } from "@/lib/api"

type PhysicalTaskType = "issue" | "reissue" | "loss_report"
type BadgeVariant = ComponentProps<typeof Badge>["variant"]

type WalletPhysicalCardTasksSectionProps = {
  writable: boolean
  loading: boolean
  refreshing: boolean
  creatingPhysicalCardTask: boolean
  updatingPhysicalCardTaskID: string
  readOnlyBoundaryHint: string
  physicalTaskPassID: string
  physicalTaskType: PhysicalTaskType
  employeeCardEligiblePasses: WalletPassInstance[]
  selectedPhysicalTaskPass: WalletPassInstance | null
  selectedPhysicalTaskTemplate?: WalletPassTemplate
  recentPhysicalCardTasks: WalletPhysicalCardTask[]
  passByID: Map<string, WalletPassInstance>
  templateByID: Map<string, WalletPassTemplate>
  physicalCardTaskForm: UseFormReturn<any>
  physicalTaskCardNumberField: UseFormRegisterReturn
  physicalTaskNoteField: UseFormRegisterReturn
  physicalCardTaskFormError: string
  onPhysicalTaskPassIDChange: (value: string) => void
  onPhysicalTaskTypeChange: (value: PhysicalTaskType) => void
  onPhysicalTaskCardNumberChange: (value: string) => void
  onPhysicalTaskNoteChange: (value: string) => void
  onSubmit: (values: any) => unknown
  onFocusEmployeePhysicalScenario: () => void
  onOpenPassQrDialog: (pass: WalletPassInstance) => unknown
  onAdvancePhysicalCardTask: (task: WalletPhysicalCardTask, status: string) => unknown
  passStatusVariant: (status: string) => BadgeVariant
  passStatusLabel: (status: string) => string
  walletScenarioLabel: (pass: WalletPassInstance, template?: WalletPassTemplate) => string
  physicalTaskStatusVariant: (status: string) => BadgeVariant
  physicalTaskStatusLabel: (status: string) => string
  nextPhysicalTaskActions: (task: WalletPhysicalCardTask) => ReadonlyArray<{ status: string; label: string }>
  formatDateTime: (value?: string) => string
}

export function WalletPhysicalCardTasksSection({
  writable,
  loading,
  refreshing,
  creatingPhysicalCardTask,
  updatingPhysicalCardTaskID,
  readOnlyBoundaryHint,
  physicalTaskPassID,
  physicalTaskType,
  employeeCardEligiblePasses,
  selectedPhysicalTaskPass,
  selectedPhysicalTaskTemplate,
  recentPhysicalCardTasks,
  passByID,
  templateByID,
  physicalCardTaskForm,
  physicalTaskCardNumberField,
  physicalTaskNoteField,
  physicalCardTaskFormError,
  onPhysicalTaskPassIDChange,
  onPhysicalTaskTypeChange,
  onPhysicalTaskCardNumberChange,
  onPhysicalTaskNoteChange,
  onSubmit,
  onFocusEmployeePhysicalScenario,
  onOpenPassQrDialog,
  onAdvancePhysicalCardTask,
  passStatusVariant,
  passStatusLabel,
  walletScenarioLabel,
  physicalTaskStatusVariant,
  physicalTaskStatusLabel,
  nextPhysicalTaskActions,
  formatDateTime,
}: WalletPhysicalCardTasksSectionProps) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-4 xl:grid-cols-[0.98fr_1.02fr]">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("walletPage.cards.physicalTasks.title")}</CardTitle>
          <CardDescription>
            {t("walletPage.cards.physicalTasks.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <form className="space-y-4" onSubmit={physicalCardTaskForm.handleSubmit(onSubmit)}>
            <div className="grid gap-3 md:grid-cols-[minmax(0,1.2fr)_180px]">
              <Controller
                control={physicalCardTaskForm.control}
                name="physical_task_pass_id"
                render={({ field }) => (
                  <Select
                    value={field.value}
                    onValueChange={(value) => {
                      field.onChange(value)
                      onPhysicalTaskPassIDChange(value)
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t("walletPage.placeholders.physicalTaskPass")} />
                    </SelectTrigger>
                    <SelectContent>
                      {employeeCardEligiblePasses.map((item) => {
                        const itemTemplate = templateByID.get(item.template_id)
                        return (
                          <SelectItem key={item.id} value={item.id}>
                            {item.target_id} · {itemTemplate?.name ?? item.template_id}
                          </SelectItem>
                        )
                      })}
                    </SelectContent>
                  </Select>
                )}
              />
              <Controller
                control={physicalCardTaskForm.control}
                name="physical_task_type"
                render={({ field }) => (
                  <Select
                    value={field.value}
                    onValueChange={(value) => {
                      const nextType = value as PhysicalTaskType
                      field.onChange(nextType)
                      onPhysicalTaskTypeChange(nextType)
                    }}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder={t("walletPage.placeholders.physicalTaskType")} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="issue">{t("walletPage.labels.physicalTaskType.issue")}</SelectItem>
                      <SelectItem value="reissue">{t("walletPage.labels.physicalTaskType.reissue")}</SelectItem>
                      <SelectItem value="loss_report">{t("walletPage.labels.physicalTaskType.lossReport")}</SelectItem>
                    </SelectContent>
                  </Select>
                )}
              />
            </div>

            <div className="grid gap-3 md:grid-cols-[220px_minmax(0,1fr)]">
              <Input
                {...physicalTaskCardNumberField}
                onChange={(event) => {
                  physicalTaskCardNumberField.onChange(event)
                  onPhysicalTaskCardNumberChange(event.target.value)
                }}
                placeholder={t("walletPage.placeholders.physicalCardNumber")}
              />
              <Textarea
                {...physicalTaskNoteField}
                rows={3}
                onChange={(event) => {
                  physicalTaskNoteField.onChange(event)
                  onPhysicalTaskNoteChange(event.target.value)
                }}
                placeholder={t("walletPage.placeholders.physicalTaskNote")}
              />
            </div>

            {selectedPhysicalTaskPass ? (
              <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-medium">{selectedPhysicalTaskPass.target_id}</span>
                  <Badge variant={passStatusVariant(selectedPhysicalTaskPass.status)}>
                    {passStatusLabel(selectedPhysicalTaskPass.status)}
                  </Badge>
                  <Badge variant="secondary">
                    {walletScenarioLabel(selectedPhysicalTaskPass, selectedPhysicalTaskTemplate)}
                  </Badge>
                </div>
                <p className="mt-1 text-sm text-muted-foreground">
                  {selectedPhysicalTaskTemplate?.name ?? selectedPhysicalTaskPass.template_id}
                </p>
                <p className="mt-1 text-xs text-muted-foreground">
                  {physicalTaskType === "loss_report"
                    ? t("walletPage.cards.physicalTasks.lossReportHint")
                    : t("walletPage.cards.physicalTasks.defaultHint")}
                </p>
              </div>
            ) : (
              <div className="rounded-lg border border-dashed px-4 py-6 text-center text-sm text-muted-foreground">
                {employeeCardEligiblePasses.length === 0
                  ? t("walletPage.cards.physicalTasks.emptyNoEligiblePass")
                  : t("walletPage.cards.physicalTasks.emptySelectPass")}
              </div>
            )}

            <div className="flex flex-wrap items-center gap-2">
              <Button
                type="submit"
                disabled={
                  !writable ||
                  creatingPhysicalCardTask ||
                  loading ||
                  refreshing ||
                  !physicalTaskPassID ||
                  physicalCardTaskForm.formState.isSubmitting
                }
              >
                {creatingPhysicalCardTask ? t("walletPage.actions.creating") : t("walletPage.actions.createPhysicalTask")}
              </Button>
              {!writable ? (
                <span className="mp-kpi-note">
                  {t("walletPage.hints.readOnlyPhysicalTasksOnly")}
                  {readOnlyBoundaryHint}
                </span>
              ) : null}
              {employeeCardEligiblePasses.length === 0 ? (
                <Button asChild size="sm" variant="outline">
                  <Link to="/wallet?scenario=employee_mobile">{t("walletPage.actions.issueEmployeePassFirst")}</Link>
                </Button>
              ) : null}
            </div>
            {physicalCardTaskFormError ? (
              <p className="text-sm text-destructive">{physicalCardTaskFormError}</p>
            ) : null}
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("walletPage.cards.recentPhysicalTasks.title")}</CardTitle>
          <CardDescription>
            {t("walletPage.cards.recentPhysicalTasks.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {recentPhysicalCardTasks.length === 0 ? (
            <div className="rounded-lg border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
              {t("walletPage.cards.recentPhysicalTasks.empty")}
            </div>
          ) : (
            recentPhysicalCardTasks.map((task) => {
              const itemPass = passByID.get(task.pass_id)
              const itemTemplate = templateByID.get(task.template_id)
              const taskActions = nextPhysicalTaskActions(task)
              return (
                <div
                  key={task.id}
                  className="rounded-xl border bg-card/80 px-4 py-3"
                >
                  <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                    <div className="space-y-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <p className="font-medium">{task.target_id}</p>
                        <Badge variant="secondary">{t(`walletPage.labels.physicalTaskType.${task.task_type === "loss_report" ? "lossReport" : task.task_type}`)}</Badge>
                        <Badge variant={physicalTaskStatusVariant(task.status)}>
                          {physicalTaskStatusLabel(task.status)}
                        </Badge>
                        <Badge variant={passStatusVariant(task.pass_status)}>
                          {t("walletPage.cards.recentPhysicalTasks.passStatusLabel", {
                            status: passStatusLabel(task.pass_status),
                          })}
                        </Badge>
                      </div>
                      <p className="text-sm text-muted-foreground">{itemTemplate?.name ?? task.template_id}</p>
                      <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                        <span>{t("walletPage.cards.recentPhysicalTasks.taskID", { id: task.id })}</span>
                        <span>{t("walletPage.cards.recentPhysicalTasks.updatedAt", { time: formatDateTime(task.updated_at) })}</span>
                        {task.card_number ? (
                          <span>{t("walletPage.cards.recentPhysicalTasks.cardNumber", { cardNumber: task.card_number })}</span>
                        ) : null}
                      </div>
                      {task.note ? <p className="mp-kpi-note">{task.note}</p> : null}
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <Button size="sm" variant="outline" onClick={onFocusEmployeePhysicalScenario}>
                        {t("walletPage.actions.viewSimilarLedger")}
                      </Button>
                      {itemPass?.save_link ? (
                        <Button size="sm" variant="outline" onClick={() => void onOpenPassQrDialog(itemPass)}>
                          {t("walletPage.actions.viewQrCode")}
                        </Button>
                      ) : null}
                      {taskActions.map((action) => (
                        <Button
                          key={action.status}
                          size="sm"
                          variant="outline"
                          disabled={!writable || updatingPhysicalCardTaskID === task.id}
                          onClick={() => void onAdvancePhysicalCardTask(task, action.status)}
                        >
                          {updatingPhysicalCardTaskID === task.id ? t("walletPage.actions.processing") : action.label}
                        </Button>
                      ))}
                      {!writable ? <span className="mp-kpi-note">{t("walletPage.hints.readOnlyBoundaryOnly")}</span> : null}
                    </div>
                  </div>
                </div>
              )
            })
          )}
        </CardContent>
      </Card>
    </div>
  )
}
