import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CalendarClockIcon, PlusIcon, Trash2Icon } from "lucide-react"

import { ConfirmActionDialog } from "@/components/mistyislet/actions"
import { PageFrame, StatusDot, ToggleSwitch } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  createAlarmSchedule,
  deleteAlarmSchedule,
  getAlarmScheduleCalendar,
  listAlarmSchedules,
  type AlarmSchedule,
  type CurrentUser,
} from "@/lib/api"

const DAY_LABELS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const
const HOURS = Array.from({ length: 24 }, (_, i) => i)
const COLORS = ["bg-brand/60", "bg-[#35a853]/60", "bg-[#d98b06]/60", "bg-[#d93025]/60",
  "bg-[#1863dc]/60", "bg-[#9333ea]/60", "bg-[#0891b2]/60", "bg-[#e11d48]/60"]
const parseHour = (t: string) => parseInt(t.split(":")[0], 10)
const INPUT_CLS = "h-10 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring focus:ring-2 focus:ring-brand-ring/20"

type AlarmSchedulePageProps = { token: string; viewer: CurrentUser }

export function AlarmSchedulePage({ token, viewer }: AlarmSchedulePageProps) {
  const queryClient = useQueryClient()
  const tenantID = viewer.tenant_id
  const [showCreate, setShowCreate] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [mutationError, setMutationError] = useState("")
  const [form, setForm] = useState({
    name: "", days_of_week: [] as number[], start_time: "09:00", end_time: "17:00", timezone: "UTC",
  })

  const schedulesQuery = useQuery({
    queryKey: ["alarm-schedules", tenantID],
    queryFn: () => listAlarmSchedules(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const calendarQuery = useQuery({
    queryKey: ["alarm-schedule-calendar", tenantID],
    queryFn: () => getAlarmScheduleCalendar(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const createMutation = useMutation({
    mutationFn: () =>
      createAlarmSchedule(token, { tenant_id: tenantID, ...form, enabled: true, alarm_types: [] }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alarm-schedules"] })
      queryClient.invalidateQueries({ queryKey: ["alarm-schedule-calendar"] })
      setShowCreate(false)
      setForm({ name: "", days_of_week: [], start_time: "09:00", end_time: "17:00", timezone: "UTC" })
      setMutationError("")
    },
    onError: (err) => setMutationError(err instanceof Error ? err.message : "Failed to create schedule"),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteAlarmSchedule(token, id, tenantID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alarm-schedules"] })
      queryClient.invalidateQueries({ queryKey: ["alarm-schedule-calendar"] })
      setConfirmDelete(null)
      setMutationError("")
    },
    onError: (err) => setMutationError(err instanceof Error ? err.message : "Failed to delete schedule"),
  })

  const schedules = schedulesQuery.data?.items ?? []
  const calendarEntries = calendarQuery.data?.entries ?? []
  const colorMap = new Map<string, string>()
  schedules.forEach((s, i) => colorMap.set(s.id, COLORS[i % COLORS.length]))

  function toggleDay(day: number) {
    setForm((f) => ({
      ...f,
      days_of_week: f.days_of_week.includes(day)
        ? f.days_of_week.filter((d) => d !== day)
        : [...f.days_of_week, day].sort(),
    }))
  }
  return (
    <PageFrame
      breadcrumbs={["Dashboard", "Alarm Schedules"]}
      title="Alarm Schedules"
      description="Configure weekly alarm monitoring windows."
      actions={
        <Button
          className="h-11 rounded-[6px] bg-brand px-6 text-white hover:bg-brand-hover"
          onClick={() => setShowCreate(true)}
        >
          <PlusIcon className="mr-2 size-4" />
          New Schedule
        </Button>
      }
    >
      {mutationError && (
        <div className="rounded-[6px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {mutationError}
        </div>
      )}

      {/* Weekly Calendar Grid */}
      <div className="overflow-hidden rounded-[6px] border border-line-subtle bg-white">
        <div className="border-b border-line-subtle px-5 py-3">
          <h2 className="flex items-center gap-2 text-sm font-semibold text-content-heading">
            <CalendarClockIcon className="size-4 text-content-subtle" />
            Weekly Calendar
          </h2>
        </div>
        <div className="overflow-x-auto p-4">
          <div className="grid min-w-[600px] grid-cols-[60px_repeat(7,1fr)] gap-px">
            {/* Header row */}
            <div />
            {DAY_LABELS.map((day) => (
              <div key={day} className="py-1 text-center text-xs font-semibold text-content-subtle">{day}</div>
            ))}
            {/* Hour rows */}
            {HOURS.map((hour) => (
              <div key={hour} className="contents">
                <div className="flex items-center justify-end pr-2 text-[10px] text-content-muted">{String(hour).padStart(2, "0")}:00</div>
                {DAY_LABELS.map((_, dayIdx) => {
                  const active = calendarEntries.filter((e) => e.day_of_week === dayIdx && parseHour(e.start_time) <= hour && parseHour(e.end_time) > hour)
                  return (
                    <div key={dayIdx} className="relative h-5 rounded-sm border border-[#f0f0f3] bg-[#fafafa]">
                      {active.map((e) => (
                        <div key={e.schedule_id} className={`absolute inset-0 rounded-sm ${colorMap.get(e.schedule_id) ?? "bg-brand/40"}`} title={e.name} />
                      ))}
                    </div>
                  )
                })}
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Create Form */}
      {showCreate && (
        <div className="rounded-[6px] border border-line-subtle bg-white p-6">
          <h3 className="mb-4 text-lg font-semibold text-content-heading">New Alarm Schedule</h3>
          <div className="grid gap-4 md:grid-cols-2">
            {([
              ["name", "Name *", "text"], ["timezone", "Timezone", "text"],
              ["start_time", "Start Time", "time"], ["end_time", "End Time", "time"],
            ] as const).map(([key, label, type]) => (
              <label key={key} className="block">
                <span className="mb-1 block text-xs font-semibold text-content-subtle">{label}</span>
                <input
                  type={type}
                  value={form[key]}
                  onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
                  className={INPUT_CLS}
                />
              </label>
            ))}
          </div>
          <div className="mt-4">
            <span className="mb-2 block text-xs font-semibold text-content-subtle">Days of Week *</span>
            <div className="flex flex-wrap gap-2">
              {DAY_LABELS.map((day, idx) => (
                <button
                  key={day}
                  type="button"
                  onClick={() => toggleDay(idx)}
                  className={`rounded-[6px] border px-3 py-1.5 text-sm font-medium transition-colors ${
                    form.days_of_week.includes(idx)
                      ? "border-brand bg-brand text-white"
                      : "border-line-default bg-white text-content-subtle hover:border-brand-ring"
                  }`}
                >
                  {day}
                </button>
              ))}
            </div>
          </div>
          <div className="mt-4 flex gap-2">
            <Button
              className="h-10 rounded-[6px] bg-brand px-6 text-white hover:bg-brand-hover"
              disabled={createMutation.isPending || !form.name.trim() || form.days_of_week.length === 0}
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

      {/* Schedule List */}
      <div>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-content-subtle">
          Schedules ({schedules.length})
        </h2>
        {schedules.length === 0 ? (
          <p className="text-sm text-content-muted">No alarm schedules configured.</p>
        ) : (
          <div className="overflow-hidden rounded-[6px] border border-line-subtle bg-white">
            {schedules.map((s) => (
              <ScheduleRow key={s.id} schedule={s} color={colorMap.get(s.id)} onDelete={setConfirmDelete} />
            ))}
          </div>
        )}
      </div>

      <ConfirmActionDialog
        open={confirmDelete !== null}
        title="Delete alarm schedule"
        description="This schedule will be permanently removed."
        confirmLabel="Delete"
        onConfirm={() => confirmDelete && deleteMutation.mutate(confirmDelete)}
        onOpenChange={(open) => { if (!open) setConfirmDelete(null) }}
        pending={deleteMutation.isPending}
        destructive
      />
    </PageFrame>
  )
}

function ScheduleRow({ schedule, color, onDelete }: { schedule: AlarmSchedule; color?: string; onDelete: (id: string) => void }) {
  const days = schedule.days_of_week.map((d) => DAY_LABELS[d]).join(", ")
  return (
    <div className="flex items-center gap-4 border-b border-line-subtle px-5 py-4 last:border-b-0">
      <span className={`size-3 shrink-0 rounded-full ${color ?? "bg-brand/60"}`} />
      <div className="min-w-0 flex-1">
        <span className="font-semibold text-content-heading">{schedule.name}</span>
        <p className="mt-0.5 text-sm text-content-subtle">
          {days} &middot; {schedule.start_time} &ndash; {schedule.end_time} ({schedule.timezone})
        </p>
      </div>
      <ToggleSwitch enabled={schedule.enabled} />
      <StatusDot tone={schedule.enabled ? "success" : "warning"} label={schedule.enabled ? "Active" : "Paused"} />
      <button
        type="button"
        title="Delete"
        className="flex size-8 items-center justify-center rounded-[6px] text-content-subtle hover:bg-surface-page"
        onClick={() => onDelete(schedule.id)}
      >
        <Trash2Icon className="size-4" />
      </button>
    </div>
  )
}
