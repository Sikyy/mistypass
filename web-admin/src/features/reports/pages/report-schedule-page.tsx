import { useState } from "react"
import i18n from "@/lib/i18n"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { FileTextIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import { PageFrame, StatusDot, ToggleSwitch } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  createReportSchedule,
  deleteReportSchedule,
  listReportSchedules,
  updateReportSchedule,
  type CurrentUser,
  type ReportSchedule,
} from "@/lib/api"

const REPORT_TYPES = [
  { value: "access_summary", label: "Access Summary" },
  { value: "alarm_history", label: "Alarm History" },
  { value: "visitor_log", label: "Visitor Log" },
  { value: "door_usage", label: "Door Usage" },
  { value: "compliance", label: "Compliance" },
]

const FREQUENCIES = [
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "monthly", label: "Monthly" },
  { value: "quarterly", label: "Quarterly" },
]

const FORMATS = [
  { value: "pdf", label: "PDF" },
  { value: "csv", label: "CSV" },
  { value: "json", label: "JSON" },
]

type ReportSchedulePageProps = { token: string; viewer: CurrentUser }

export function ReportSchedulePage({ token, viewer }: ReportSchedulePageProps) {
  const queryClient = useQueryClient()
  const tenantID = viewer.tenant_id
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<ReportSchedule | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [mutationError, setMutationError] = useState("")
  const [form, setForm] = useState({
    name: "", report_type: "access_summary", frequency: "weekly", recipients: "",
    format: "pdf", day_of_week: 1, enabled: true,
  })

  const defaultForm = { name: "", report_type: "access_summary", frequency: "weekly", recipients: "", format: "pdf", day_of_week: 1, enabled: true }

  function openCreate() {
    setEditTarget(null)
    setForm(defaultForm)
    setMutationError("")
    setSheetOpen(true)
  }

  function openEdit(s: ReportSchedule) {
    setEditTarget(s)
    setForm({
      name: s.name,
      report_type: s.report_type,
      frequency: s.frequency,
      recipients: s.recipients.join(", "),
      format: s.format,
      day_of_week: s.day_of_week,
      enabled: s.enabled,
    })
    setMutationError("")
    setSheetOpen(true)
  }

  const schedulesQuery = useQuery({
    queryKey: ["report-schedules", tenantID],
    queryFn: () => listReportSchedules(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const createMutation = useMutation({
    mutationFn: () =>
      createReportSchedule(token, {
        tenant_id: tenantID,
        name: form.name,
        report_type: form.report_type,
        frequency: form.frequency,
        recipients: form.recipients.split(",").map((r) => r.trim()).filter(Boolean),
        format: form.format,
        day_of_week: form.day_of_week,
        enabled: form.enabled,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["report-schedules"] })
      setSheetOpen(false)
      setForm(defaultForm)
      setMutationError("")
    },
    onError: (err) => setMutationError(err instanceof Error ? err.message : "Failed to create report schedule"),
  })

  const updateMutation = useMutation({
    mutationFn: () =>
      updateReportSchedule(token, editTarget!.id, {
        tenant_id: tenantID,
        name: form.name,
        report_type: form.report_type,
        frequency: form.frequency,
        recipients: form.recipients.split(",").map((r) => r.trim()).filter(Boolean),
        format: form.format,
        day_of_week: form.day_of_week,
        enabled: form.enabled,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["report-schedules"] })
      setSheetOpen(false)
      setEditTarget(null)
      setForm(defaultForm)
      setMutationError("")
    },
    onError: (err) => setMutationError(err instanceof Error ? err.message : "Failed to update report schedule"),
  })

  const toggleMutation = useMutation({
    mutationFn: (s: ReportSchedule) =>
      updateReportSchedule(token, s.id, { tenant_id: tenantID, enabled: !s.enabled }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["report-schedules"] })
    },
    onError: (err) => setMutationError(err instanceof Error ? err.message : "Failed to toggle schedule"),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteReportSchedule(token, id, tenantID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["report-schedules"] })
      setConfirmDelete(null)
      setMutationError("")
    },
    onError: (err) => setMutationError(err instanceof Error ? err.message : "Failed to delete report schedule"),
  })

  const schedules = schedulesQuery.data?.items ?? []
  const isEditing = editTarget !== null
  const saving = createMutation.isPending || updateMutation.isPending
  return (
    <PageFrame
      breadcrumbs={["Dashboard", "Reports"]}
      title="Report Schedules"
      description="Configure automated report delivery."
      actions={
        <Button
          className="h-11 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#3439cc]"
          onClick={openCreate}
        >
          <PlusIcon className="mr-2 size-4" />
          New Report Schedule
        </Button>
      }
    >
      {mutationError && (
        <div className="rounded-[6px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {mutationError}
        </div>
      )}

      {/* Schedule Table */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-[#6f717c]">
          Scheduled Reports ({schedules.length})
        </h2>
        {schedules.length === 0 ? (
          <p className="text-sm text-[#9a9ca7]">No report schedules configured.</p>
        ) : (
          <div className="overflow-hidden rounded-[6px] border border-[#eceef2] bg-white">
            <div className="hidden border-b border-[#eceef2] px-5 py-3 md:grid md:grid-cols-[2fr_1fr_1fr_1fr_1fr_auto]">
              {["Name", "Type", "Frequency", "Recipients", "Last Sent", ""].map((h) => (
                <span key={h} className="text-xs font-semibold text-[#6f717c]">{h}</span>
              ))}
            </div>
            {schedules.map((s) => (
              <ReportRow
                key={s.id}
                schedule={s}
                onEdit={openEdit}
                onDelete={setConfirmDelete}
                onToggle={(sched) => toggleMutation.mutate(sched)}
              />
            ))}
          </div>
        )}
      </div>

      {/* Create / Edit Sheet */}
      <Sheet open={sheetOpen} onOpenChange={(open) => { if (!open) { setSheetOpen(false); setEditTarget(null) } }}>
        <SheetContent className="w-full max-w-lg overflow-y-auto">
          <SheetHeader>
            <SheetTitle>{isEditing ? "Edit Report Schedule" : "New Report Schedule"}</SheetTitle>
            <SheetDescription>
              {isEditing ? "Update the schedule settings below." : "Configure automated report delivery."}
            </SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 p-6"
            onSubmit={(e) => {
              e.preventDefault()
              if (isEditing) updateMutation.mutate()
              else createMutation.mutate()
            }}
          >
            {mutationError && (
              <div className="rounded-[6px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
                {mutationError}
              </div>
            )}
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-[#6f717c]">Name *</span>
              <input
                type="text"
                required
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-[#6f717c]">Report Type *</span>
              <select
                value={form.report_type}
                onChange={(e) => setForm((f) => ({ ...f, report_type: e.target.value }))}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
              >
                {REPORT_TYPES.map((t) => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-[#6f717c]">Frequency *</span>
              <select
                value={form.frequency}
                onChange={(e) => setForm((f) => ({ ...f, frequency: e.target.value }))}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
              >
                {FREQUENCIES.map((f) => (
                  <option key={f.value} value={f.value}>{f.label}</option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-[#6f717c]">Format</span>
              <select
                value={form.format}
                onChange={(e) => setForm((f) => ({ ...f, format: e.target.value }))}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
              >
                {FORMATS.map((f) => (
                  <option key={f.value} value={f.value}>{f.label}</option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-[#6f717c]">Recipients (comma-separated emails) *</span>
              <input
                type="text"
                required
                value={form.recipients}
                onChange={(e) => setForm((f) => ({ ...f, recipients: e.target.value }))}
                placeholder="admin@example.com, ops@example.com"
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
              />
            </label>
            <div className="flex items-center justify-between rounded-[6px] border border-[#eceef2] px-4 py-3">
              <span className="text-sm font-medium text-[#17171c]">Enabled</span>
              <ToggleSwitch enabled={form.enabled} onToggle={() => setForm((f) => ({ ...f, enabled: !f.enabled }))} />
            </div>
            <SheetFooter>
              <Button
                type="submit"
                className="h-11 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#3439cc]"
                disabled={saving || !form.name.trim() || !form.recipients.trim()}
              >
                {saving ? "Saving..." : isEditing ? "Save Changes" : "Create Schedule"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <ConfirmActionDialog
        open={confirmDelete !== null}
        title="Delete report schedule"
        description="This report schedule will be permanently removed."
        confirmLabel="Delete"
        onConfirm={() => confirmDelete && deleteMutation.mutate(confirmDelete)}
        onOpenChange={(open) => { if (!open) setConfirmDelete(null) }}
        pending={deleteMutation.isPending}
        destructive
      />
    </PageFrame>
  )
}

function ReportRow({
  schedule,
  onEdit,
  onDelete,
  onToggle,
}: {
  schedule: ReportSchedule
  onEdit: (s: ReportSchedule) => void
  onDelete: (id: string) => void
  onToggle: (s: ReportSchedule) => void
}) {
  const typeLabel = REPORT_TYPES.find((t) => t.value === schedule.report_type)?.label ?? schedule.report_type
  const freqLabel = FREQUENCIES.find((f) => f.value === schedule.frequency)?.label ?? schedule.frequency
  return (
    <div className="flex flex-col gap-2 border-b border-[#eceef2] px-5 py-4 last:border-b-0 md:grid md:grid-cols-[2fr_1fr_1fr_1fr_1fr_auto] md:items-center md:gap-4">
      <div className="flex items-center gap-2">
        <FileTextIcon className="size-4 shrink-0 text-[#6f717c]" />
        <div className="min-w-0">
          <span className="font-semibold text-[#17171c]">{schedule.name}</span>
          <span className="ml-2 text-xs uppercase text-[#9a9ca7]">{schedule.format}</span>
        </div>
      </div>
      <span className="text-sm text-[#6f717c]">{typeLabel}</span>
      <span className="text-sm text-[#6f717c]">{freqLabel}</span>
      <span className="truncate text-sm text-[#6f717c]">{schedule.recipients.join(", ")}</span>
      <span className="text-sm text-[#9a9ca7]">{schedule.last_sent_at ? new Date(schedule.last_sent_at).toLocaleDateString(i18n.language) : "Never"}</span>
      <div className="flex items-center gap-2">
        <ToggleSwitch enabled={schedule.enabled} onToggle={() => onToggle(schedule)} />
        <StatusDot tone={schedule.enabled ? "success" : "warning"} label={schedule.enabled ? "Active" : "Paused"} />
        <RowActionsMenu
          items={[
            { id: "edit", label: "Edit", icon: PencilIcon, onSelect: () => onEdit(schedule) },
            { id: "delete", label: "Delete", icon: Trash2Icon, onSelect: () => onDelete(schedule.id), destructive: true },
          ]}
        />
      </div>
    </div>
  )
}
