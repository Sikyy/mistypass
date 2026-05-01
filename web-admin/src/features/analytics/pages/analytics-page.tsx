import { useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { BarChart3Icon, DownloadIcon, ShieldAlertIcon, DoorOpenIcon } from "lucide-react"

import { PageFrame, KpiCard, StatusDot } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  getAccessSummary,
  getDoorActivity,
  getAlarmMetrics,
  exportAnalytics,
  type CurrentUser,
} from "@/lib/api"

const todayISO = () => new Date().toISOString().slice(0, 10)
const weekAgoISO = () => { const d = new Date(); d.setDate(d.getDate() - 7); return d.toISOString().slice(0, 10) }

export function AnalyticsPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const [start, setStart] = useState(weekAgoISO)
  const [end, setEnd] = useState(todayISO)
  const [exporting, setExporting] = useState(false)

  const summaryQuery = useQuery({
    queryKey: ["analytics-access-summary", start, end],
    queryFn: () => getAccessSummary(token, { start, end }),
  })

  const doorQuery = useQuery({
    queryKey: ["analytics-door-activity", start, end],
    queryFn: () => getDoorActivity(token, { start, end }),
  })

  const alarmQuery = useQuery({
    queryKey: ["analytics-alarm-metrics", start, end],
    queryFn: () => getAlarmMetrics(token, { start, end }),
  })

  const summary = summaryQuery.data
  const doors = doorQuery.data ?? []
  const alarms = alarmQuery.data

  async function handleExport() {
    setExporting(true)
    try {
      const blob = await exportAnalytics(token, { start, end, format: "pdf" })
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = `analytics-${start}-${end}.pdf`
      a.click()
      URL.revokeObjectURL(url)
    } finally {
      setExporting(false)
    }
  }

  const maxHourlyCount = doors.reduce(
    (max, d) => Math.max(max, ...d.hourly_counts),
    1
  )

  return (
    <PageFrame
      breadcrumbs={["Home", "Analytics"]}
      title="Analytics"
      description="Access events, door activity, and alarm metrics across your properties."
      actions={
        <div className="flex flex-wrap items-center gap-3">
          <input
            type="date"
            value={start}
            onChange={(e) => setStart(e.target.value)}
            className="h-10 rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
          />
          <span className="text-sm text-[#6f717c]">to</span>
          <input
            type="date"
            value={end}
            onChange={(e) => setEnd(e.target.value)}
            className="h-10 rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
          />
          <Button
            onClick={handleExport}
            disabled={exporting}
            className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
          >
            <DownloadIcon className="mr-2 size-4" />
            {exporting ? "Exporting..." : "Export PDF"}
          </Button>
        </div>
      }
    >
      {/* Access Summary */}
      <section>
        <h2 className="mb-4 text-lg font-semibold text-[#17171c]">Access Summary</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <KpiCard label="Total Events" value={String(summary?.total_events ?? 0)} note="All access events in range" icon={BarChart3Icon} tone="info" loading={summaryQuery.isPending} />
          <KpiCard label="Granted" value={String(summary?.granted ?? 0)} note="Successful access grants" icon={DoorOpenIcon} tone="success" loading={summaryQuery.isPending} />
          <KpiCard label="Denied" value={String(summary?.denied ?? 0)} note="Rejected access attempts" icon={ShieldAlertIcon} tone="danger" loading={summaryQuery.isPending} />
          <KpiCard label="Peak Hour" value={summary?.peak_hour ?? "--"} note="Busiest hour of the day" icon={BarChart3Icon} tone="warning" loading={summaryQuery.isPending} />
        </div>
      </section>

      {/* Door Activity */}
      <section>
        <h2 className="mb-4 text-lg font-semibold text-[#17171c]">Door Activity</h2>
        <div className="rounded-[6px] border border-[#e1e3e8] bg-white">
          {doorQuery.isPending ? (
            <p className="px-5 py-8 text-center text-sm text-[#6f717c]">Loading door activity...</p>
          ) : doors.length === 0 ? (
            <p className="px-5 py-8 text-center text-sm text-[#6f717c]">No door activity for this period.</p>
          ) : (
            <div className="divide-y divide-[#eceef2]">
              {doors.slice(0, 10).map((door) => (
                <div key={door.door_id} className="flex items-center gap-5 px-5 py-4">
                  <div className="w-40 min-w-0 shrink-0">
                    <p className="truncate text-sm font-medium text-[#17171c]">{door.door_name}</p>
                    <p className="text-xs text-[#6f717c]">{door.access_count} events</p>
                  </div>
                  <div className="flex flex-1 items-end gap-px">
                    {door.hourly_counts.map((count, i) => (
                      <div
                        key={i}
                        className="flex-1 rounded-t bg-[#4f55ff]"
                        style={{ height: `${Math.max((count / maxHourlyCount) * 48, 2)}px` }}
                        title={`${String(i).padStart(2, "0")}:00 - ${count} events`}
                      />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      {/* Alarm Metrics */}
      <section>
        <h2 className="mb-4 text-lg font-semibold text-[#17171c]">Alarm Metrics</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div className="rounded-[6px] border border-[#e1e3e8] bg-white p-5">
            <p className="text-sm font-medium text-[#6f717c]">Total Alarms</p>
            <p className="mt-2 text-2xl font-bold text-[#17171c]">
              {alarmQuery.isPending ? "--" : alarms?.total ?? 0}
            </p>
          </div>
          <div className="rounded-[6px] border border-[#e1e3e8] bg-white p-5">
            <p className="text-sm font-medium text-[#6f717c]">Open / Resolved</p>
            <div className="mt-2 flex items-center gap-4">
              <StatusDot tone="danger" label={`${alarms?.open ?? 0} open`} />
              <StatusDot tone="success" label={`${alarms?.resolved ?? 0} resolved`} />
            </div>
          </div>
          <div className="rounded-[6px] border border-[#e1e3e8] bg-white p-5">
            <p className="mb-3 text-sm font-medium text-[#6f717c]">By Severity</p>
            <div className="space-y-2">
              {alarmQuery.isPending ? (
                <p className="text-sm text-[#6f717c]">Loading...</p>
              ) : (
                (alarms?.by_severity ?? []).map((s) => (
                  <div key={s.severity} className="flex items-center justify-between text-sm">
                    <span className="capitalize text-[#2f3037]">{s.severity}</span>
                    <span className="font-semibold text-[#17171c]">{s.count}</span>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </section>
    </PageFrame>
  )
}
