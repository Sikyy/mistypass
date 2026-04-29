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
      breadcrumbs={["Home", "Places", place?.name ?? "Assigned Place", "Dashboard"]}
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
      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="border-b border-[#eceef2] px-6 py-5">
          <h2 className="text-lg font-semibold text-[#17171c]">Recent Place Activity</h2>
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
              No recent place activity.
            </div>
          )}
        </div>
      </section>
    </PageFrame>
  )
}
