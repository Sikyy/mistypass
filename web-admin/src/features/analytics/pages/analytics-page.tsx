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
import { getViewerTenantID } from "@/lib/viewer"

const todayISO = () => new Date().toISOString().slice(0, 10)
const weekAgoISO = () => { const d = new Date(); d.setDate(d.getDate() - 7); return d.toISOString().slice(0, 10) }

export function AnalyticsPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const [start, setStart] = useState(weekAgoISO)
  const [end, setEnd] = useState(todayISO)
  const [exporting, setExporting] = useState(false)
  const tenantID = getViewerTenantID(viewer)

  const summaryQuery = useQuery({
    queryKey: ["analytics-access-summary", start, end],
    queryFn: () => getAccessSummary(token, tenantID, start, end),
  })

  const doorQuery = useQuery({
    queryKey: ["analytics-door-activity", start, end],
    queryFn: () => getDoorActivity(token, tenantID),
  })

  const alarmQuery = useQuery({
    queryKey: ["analytics-alarm-metrics", start, end],
    queryFn: () => getAlarmMetrics(token, tenantID, start, end),
  })

  const summary = summaryQuery.data
  const doors = doorQuery.data?.doors ?? []
  const alarms = alarmQuery.data

  async function handleExport() {
    setExporting(true)
    try {
      const blob = await exportAnalytics(token, tenantID, "access", "pdf", start, end)
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
    (max, d) => Math.max(max, ...d.hourly_distribution),
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
            className="h-10 rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
          />
          <span className="text-sm text-content-subtle">to</span>
          <input
            type="date"
            value={end}
            onChange={(e) => setEnd(e.target.value)}
            className="h-10 rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
          />
          <Button
            onClick={handleExport}
            disabled={exporting}
            className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover"
          >
            <DownloadIcon className="mr-2 size-4" />
            {exporting ? "Exporting..." : "Export PDF"}
          </Button>
        </div>
      }
    >
      {/* Access Summary */}
      <section>
        <h2 className="mb-4 text-lg font-semibold text-content-heading">Access Summary</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <KpiCard label="Total Events" value={String(summary?.total_events ?? 0)} note="All access events in range" icon={BarChart3Icon} tone="info" loading={summaryQuery.isPending} />
          <KpiCard label="Granted" value={String(summary?.by_result?.["granted"] ?? 0)} note="Successful access grants" icon={DoorOpenIcon} tone="success" loading={summaryQuery.isPending} />
          <KpiCard label="Denied" value={String(summary?.by_result?.["denied"] ?? 0)} note="Rejected access attempts" icon={ShieldAlertIcon} tone="danger" loading={summaryQuery.isPending} />
          <KpiCard label="Peak Hour" value={summary?.peak_hour != null ? String(summary.peak_hour) : "--"} note="Busiest hour of the day" icon={BarChart3Icon} tone="warning" loading={summaryQuery.isPending} />
        </div>
      </section>

      {/* Door Activity */}
      <section>
        <h2 className="mb-4 text-lg font-semibold text-content-heading">Door Activity</h2>
        <div className="rounded-[6px] border border-line-default bg-white">
          {doorQuery.isPending ? (
            <p className="px-5 py-8 text-center text-sm text-content-subtle">Loading door activity...</p>
          ) : doors.length === 0 ? (
            <p className="px-5 py-8 text-center text-sm text-content-subtle">No door activity for this period.</p>
          ) : (
            <div className="divide-y divide-line-subtle">
              {doors.slice(0, 10).map((door) => (
                <div key={door.door_id} className="flex items-center gap-5 px-5 py-4">
                  <div className="w-40 min-w-0 shrink-0">
                    <p className="truncate text-sm font-medium text-content-heading">{door.door_id}</p>
                    <p className="text-xs text-content-subtle">{door.total_access} events</p>
                  </div>
                  <div className="flex flex-1 items-end gap-px">
                    {door.hourly_distribution.map((count: number, i: number) => (
                      <div
                        key={i}
                        className="flex-1 rounded-t bg-brand"
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
        <h2 className="mb-4 text-lg font-semibold text-content-heading">Alarm Metrics</h2>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div className="rounded-[6px] border border-line-default bg-white p-5">
            <p className="text-sm font-medium text-content-subtle">Total Alarms</p>
            <p className="mt-2 text-2xl font-bold text-content-heading">
              {alarmQuery.isPending ? "--" : alarms?.total ?? 0}
            </p>
          </div>
          <div className="rounded-[6px] border border-line-default bg-white p-5">
            <p className="text-sm font-medium text-content-subtle">Open / Resolved</p>
            <div className="mt-2 flex items-center gap-4">
              <StatusDot tone="danger" label={`${alarms?.by_status?.["open"] ?? 0} open`} />
              <StatusDot tone="success" label={`${alarms?.by_status?.["resolved"] ?? 0} resolved`} />
            </div>
          </div>
          <div className="rounded-[6px] border border-line-default bg-white p-5">
            <p className="mb-3 text-sm font-medium text-content-subtle">By Severity</p>
            <div className="space-y-2">
              {alarmQuery.isPending ? (
                <p className="text-sm text-content-subtle">Loading...</p>
              ) : (
                Object.entries(alarms?.by_severity ?? {}).map(([severity, count]) => (
                  <div key={severity} className="flex items-center justify-between text-sm">
                    <span className="capitalize text-content-body">{severity}</span>
                    <span className="font-semibold text-content-heading">{count}</span>
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
