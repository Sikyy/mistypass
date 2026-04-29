import { useMemo } from "react"
import { AlertCircleIcon, DoorOpenIcon, HistoryIcon, UsersIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { KpiCard, PageFrame, StatusDot, type ActivityTone } from "@/components/mistyislet/primitives"
import { useMistyisletPlaceContext } from "@/features/mistyislet-shell/use-resource-summary"
import type { CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"

function isSameLocalDate(value: string, expected: Date) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return false
  }
  return (
    parsed.getFullYear() === expected.getFullYear() &&
    parsed.getMonth() === expected.getMonth() &&
    parsed.getDate() === expected.getDate()
  )
}

export function PlaceDashboardAdaptedPage({
  token,
  viewer,
  placeID,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
}) {
  const { t } = useTranslation()
  const resourceQuery = useMistyisletPlaceContext(token, viewer, placeID)
  const { place, doors, events } = resourceQuery.context
  const today = new Date()
  const onlineDoors = doors.filter((door) => door.status === "online").length
  const todayEvents = events.filter((event) => isSameLocalDate(event.at, today))
  const activeUsers = new Set(events.filter((event) => event.user !== "system").map((event) => event.user)).size
  const reviewCount = place?.offlineCount ?? events.filter((event) => event.tone !== "success").length
  const cards = [
    [
      t("kisi.placeDashboard.onlineDoors"),
      `${onlineDoors}/${doors.length}`,
      place?.offlineCount ? `${place.offlineCount} resource needs review` : "All scoped doors online",
      DoorOpenIcon,
      place?.offlineCount ? "warning" : "success",
    ],
    [t("kisi.placeDashboard.todayUnlocks"), todayEvents.length.toLocaleString(), "Access events in local time", HistoryIcon, "info"],
    [t("kisi.placeDashboard.activeUsers"), activeUsers.toString(), "Users seen in recent events", UsersIcon, "success"],
    [
      t("kisi.placeDashboard.openAlerts"),
      reviewCount.toString(),
      reviewCount > 0 ? "Operational follow-up needed" : "No review backlog",
      AlertCircleIcon,
      reviewCount > 0 ? "danger" : "success",
    ],
  ] as const

  return (
    <PageFrame
      breadcrumbs={[t("common.home"), "Places", place?.name ?? "Assigned Place", "Dashboard"]}
      title={t("kisi.placeDashboard.title")}
      description={place ? t("kisi.placeDashboard.description", { place: place.name }) : t("kisi.placeDashboard.description", { place: "" })}
    >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live place resources are unavailable. Showing reference data.
        </div>
      ) : null}
      <section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {cards.map(([label, value, note, Icon, tone]) => (
          <KpiCard key={label} label={label} value={value} note={note} icon={Icon} tone={tone as ActivityTone} loading={resourceQuery.isPending} />
        ))}
      </section>
      <DailyUsageTable events={events} t={t} />
      <UnlockHeatmap events={events} t={t} />

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="border-b border-[#eceef2] px-6 py-5">
          <h2 className="text-lg font-semibold text-[#17171c]">{t("kisi.placeDashboard.recentActivity")}</h2>
        </div>
        <div className="divide-y divide-[#eceef2]">
          {events.length > 0 ? (
            events.slice(0, 6).map((event, index) => (
              <div key={event.id} className={cn("grid grid-cols-[90px_1fr_180px_120px] items-center px-6 py-4 text-sm", index === 0 && "bg-[#e7e5df]")}>
                <span className="font-medium text-[#6f717c]">{event.timeLabel}</span>
                <span className="font-semibold text-[#4f55ff]">{event.object}</span>
                <span className="text-[#2f3037]">{event.action}</span>
                <StatusDot tone={event.tone} label={event.statusLabel} />
              </div>
            ))
          ) : (
            <div className="px-6 py-12 text-center text-sm text-[#6f717c]">
              {t("kisi.placeDashboard.noActivity")}
            </div>
          )}
        </div>
      </section>
    </PageFrame>
  )
}

type EventRow = { id: string; at: string; user: string; object: string; action: string; tone: string }
type TFn = ReturnType<typeof useTranslation>["t"]

function getDayLabel(date: Date): string {
  return date.toLocaleDateString("en-US", { weekday: "long", month: "short", day: "numeric", year: "numeric" })
}

function DailyUsageTable({ events, t }: { events: EventRow[]; t: TFn }) {
  const dailyData = useMemo(() => {
    const days = new Map<string, { unlocks: number; users: Set<string> }>()
    const today = new Date()
    for (let i = 6; i >= 0; i--) {
      const d = new Date(today)
      d.setDate(d.getDate() - i)
      const key = d.toISOString().split("T")[0]
      days.set(key, { unlocks: 0, users: new Set() })
    }
    for (const ev of events) {
      const key = ev.at?.split("T")[0]
      if (key && days.has(key)) {
        const entry = days.get(key)!
        entry.unlocks++
        if (ev.user && ev.user !== "system") entry.users.add(ev.user)
      }
    }
    return Array.from(days.entries()).map(([date, data]) => ({
      date,
      label: getDayLabel(new Date(date + "T00:00:00")),
      unlocks: data.unlocks,
      uniqueUsers: data.users.size,
      occupancy: Math.min(100, Math.round((data.users.size / Math.max(1, data.unlocks)) * 100)),
    }))
  }, [events])

  const maxUnlocks = Math.max(1, ...dailyData.map((d) => d.unlocks))

  return (
    <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
      <div className="border-b border-[#eceef2] px-6 py-5">
        <h2 className="text-lg font-semibold text-[#17171c]">
          <span className="mr-2 inline-block size-3 rounded-sm bg-[#4f55ff]" />
          {t("kisi.placeDashboard.dailyUsage")}
        </h2>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[600px] text-left text-sm">
          <thead>
            <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-xs text-[#6f717c]">
              <th className="px-6 py-3 font-semibold">{t("kisi.placeDashboard.date")}</th>
              <th className="px-4 py-3 text-right font-semibold">{t("kisi.placeDashboard.unlockCount")}</th>
              <th className="px-4 py-3 text-right font-semibold">{t("kisi.placeDashboard.uniqueUsersCol")}</th>
              <th className="px-4 py-3 text-right font-semibold">{t("kisi.placeDashboard.occupancy")}</th>
            </tr>
          </thead>
          <tbody>
            {dailyData.map((row) => (
              <tr key={row.date} className="border-b border-[#eceef2] last:border-0">
                <td className="px-6 py-3 text-[#2f3037]">{row.label}</td>
                <td className="px-4 py-3 text-right">
                  <span className="inline-block rounded px-2 py-0.5 text-xs font-semibold" style={{ backgroundColor: `rgba(79,85,255,${Math.max(0.15, row.unlocks / maxUnlocks)})`, color: row.unlocks > 0 ? "#fff" : "#6f717c" }}>
                    {row.unlocks}
                  </span>
                </td>
                <td className="px-4 py-3 text-right">
                  <span className="inline-block rounded bg-[#e8e9ff] px-2 py-0.5 text-xs font-semibold text-[#4f55ff]">{row.uniqueUsers}</span>
                </td>
                <td className="px-4 py-3 text-right">
                  <span className={cn("inline-block rounded px-2 py-0.5 text-xs font-semibold", row.occupancy > 50 ? "bg-[#d4f5db] text-[#1f6b3a]" : "bg-[#f5f6f8] text-[#6f717c]")}>{row.occupancy}%</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}

function UnlockHeatmap({ events, t }: { events: EventRow[]; t: TFn }) {
  const heatmapData = useMemo(() => {
    const grid: number[][] = []
    const today = new Date()
    const dayLabels: string[] = []
    for (let d = 6; d >= 0; d--) {
      const date = new Date(today)
      date.setDate(date.getDate() - d)
      dayLabels.push(date.toLocaleDateString("en-US", { weekday: "short", month: "short", day: "numeric" }))
      const row: number[] = new Array(24).fill(0)
      for (const ev of events) {
        if (!ev.at) continue
        const evDate = new Date(ev.at)
        if (evDate.toDateString() === date.toDateString() && ev.user && ev.user !== "system") {
          row[evDate.getHours()]++
        }
      }
      grid.push(row)
    }
    return { grid, dayLabels }
  }, [events])

  const maxVal = Math.max(1, ...heatmapData.grid.flat())
  const hours = Array.from({ length: 24 }, (_, i) => i)

  return (
    <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
      <div className="border-b border-[#eceef2] px-6 py-5">
        <h2 className="text-lg font-semibold text-[#17171c]">
          <span className="mr-2 inline-block size-3 rounded-sm bg-[#4f55ff]" />
          {t("kisi.placeDashboard.unlockHeatmap")}
        </h2>
      </div>
      <div className="overflow-x-auto px-6 py-4">
        <div className="min-w-[700px]">
          <div className="mb-1 flex">
            <div className="w-[130px]" />
            {hours.map((h) => (
              <div key={h} className="flex-1 text-center text-[10px] text-[#6f717c]">{h}</div>
            ))}
          </div>
          {heatmapData.grid.map((row, dayIdx) => (
            <div key={heatmapData.dayLabels[dayIdx]} className="mb-[2px] flex items-center">
              <div className="w-[130px] truncate pr-2 text-xs text-[#2f3037]">{heatmapData.dayLabels[dayIdx]}</div>
              {row.map((val, hourIdx) => (
                <div
                  key={hourIdx}
                  className="mx-[1px] flex-1 rounded-[2px]"
                  style={{
                    height: 20,
                    backgroundColor: val > 0 ? `rgba(79,85,255,${Math.max(0.12, val / maxVal)})` : "#f5f6f8",
                  }}
                  title={`${val} ${val === 1 ? "user" : "users"}`}
                >
                  {val > 0 ? <span className="flex size-full items-center justify-center text-[9px] font-bold text-white">{val}</span> : null}
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
