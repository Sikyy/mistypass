import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { FileTextIcon, PlusIcon, Trash2Icon } from "lucide-react"

import { ConfirmActionDialog } from "@/components/mistyislet/actions"
import { PageFrame, StatusDot, ToggleSwitch } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  createReportSchedule,
  deleteReportSchedule,
  listReportSchedules,
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
  const [showCreate, setShowCreate] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [mutationError, setMutationError] = useState("")
  const [form, setForm] = useState({
    name: "", report_type: "access_summary", frequency: "weekly", recipients: "",
    format: "pdf", day_of_week: 1,
  })

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
        enabled: true,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["report-schedules"] })
      setShowCreate(false)
      setForm({ name: "", report_type: "access_summary", frequency: "weekly", recipients: "", format: "pdf", day_of_week: 1 })
      setMutationError("")
    },
    onError: (err) => setMutationError(err instanceof Error ? err.message : "Failed to create report schedule"),
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
  return (
    <PageFrame
      breadcrumbs={["Dashboard", "Reports"]}
      title="Report Schedules"
      description="Configure automated report delivery."
      actions={
        <Button
          className="h-11 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#3439cc]"
          onClick={() => setShowCreate(true)}
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

      {/* Create Form */}
      {showCreate && (
        <div className="rounded-[6px] border border-[#eceef2] bg-white p-6">
          <h3 className="mb-4 text-lg font-semibold text-[#17171c]">New Report Schedule</h3>
          <div className="grid gap-4 md:grid-cols-2">
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-[#6f717c]">Name *</span>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                className="h-10 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-[#6f717c]">Report Type *</span>
              <select
                value={form.report_type}
                onChange={(e) => setForm((f) => ({ ...f, report_type: e.target.value }))}
                className="h-10 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
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
                className="h-10 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
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
                className="h-10 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
              >
                {FORMATS.map((f) => (
                  <option key={f.value} value={f.value}>{f.label}</option>
                ))}
              </select>
            </label>
            <label className="block md:col-span-2">
              <span className="mb-1 block text-xs font-semibold text-[#6f717c]">Recipients (comma-separated emails) *</span>
              <input
                type="text"
                value={form.recipients}
                onChange={(e) => setForm((f) => ({ ...f, recipients: e.target.value }))}
                placeholder="admin@example.com, ops@example.com"
                className="h-10 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
              />
            </label>
          </div>
          <div className="mt-4 flex gap-2">
            <Button
              className="h-10 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#3439cc]"
              disabled={createMutation.isPending || !form.name.trim() || !form.recipients.trim()}
              onClick={() => createMutation.mutate()}
            >
              {createMutation.isPending ? "Creating..." : "Create Schedule"}
            </Button>
            <Button variant="outline" className="h-10 rounded-[6px]" onClick={() => setShowCreate(false)}>
              Cancel
            </Button>
          </div>
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
              <ReportRow key={s.id} schedule={s} onDelete={setConfirmDelete} />
            ))}
          </div>
        )}
      </div>

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

function ReportRow({ schedule, onDelete }: { schedule: ReportSchedule; onDelete: (id: string) => void }) {
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
      <span className="text-sm text-[#9a9ca7]">{schedule.last_sent_at ? new Date(schedule.last_sent_at).toLocaleDateString() : "Never"}</span>
      <div className="flex items-center gap-2">
        <ToggleSwitch enabled={schedule.enabled} />
        <StatusDot tone={schedule.enabled ? "success" : "warning"} label={schedule.enabled ? "Active" : "Paused"} />
        <button
          type="button"
          title="Delete"
          className="flex size-8 items-center justify-center rounded-[6px] text-[#6f717c] hover:bg-[#fbfbfc]"
          onClick={() => onDelete(schedule.id)}
        >
          <Trash2Icon className="size-4" />
        </button>
      </div>
    </div>
  )
}
