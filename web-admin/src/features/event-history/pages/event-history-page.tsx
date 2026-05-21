import { Fragment, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { CameraIcon, ChevronDownIcon, ImageIcon, MapPinPlusIcon } from "lucide-react"

import { MistyisletEmptyTableRow, MistyisletSearchField } from "@/components/mistyislet/data-display"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Button } from "@/components/ui/button"
import { PageFrame, StatusDot } from "@/components/mistyislet/primitives"
import { selectMistyisletPlaceContext } from "@/features/mistyislet-shell/resource-data"
import { useMistyisletResourceSummary } from "@/features/mistyislet-shell/use-resource-summary"
import { listEventSnapshots, type CameraSnapshot, type CurrentUser } from "@/lib/api"
import { cn } from "@/lib/utils"

export function EventHistoryAdaptedPage({
  token,
  viewer,
  placeID,
  placeScoped = false,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
  placeScoped?: boolean
}) {
  const { t } = useTranslation()
  const resourceQuery = useMistyisletResourceSummary(token, viewer)
  const placeContext = useMemo(
    () => selectMistyisletPlaceContext(resourceQuery.summary, placeID),
    [placeID, resourceQuery.summary]
  )
  const rows = placeScoped ? placeContext.events : resourceQuery.summary.events
  const [expandedRow, setExpandedRow] = useState("")
  const [query, setQuery] = useState("")
  const [dateFilter, setDateFilter] = useState("all")
  const [actionFilter, setActionFilter] = useState("all")

  const actionTypes = useMemo(() => {
    const types = new Set<string>()
    for (const row of rows) {
      if (row.action) types.add(row.action)
    }
    return Array.from(types).sort()
  }, [rows])

  const visibleRows = rows.filter((row) => {
    const normalizedQuery = query.trim().toLowerCase()
    if (normalizedQuery !== "" && ![row.object, row.action, row.user, row.timeLabel, row.statusLabel, row.details].join(" ").toLowerCase().includes(normalizedQuery)) {
      return false
    }
    if (actionFilter !== "all" && row.action !== actionFilter) {
      return false
    }
    if (dateFilter === "today") {
      const today = new Date().toISOString().split("T")[0]
      if (!row.timeLabel?.startsWith(today)) return false
    } else if (dateFilter === "7days") {
      const weekAgo = new Date(Date.now() - 7 * 86400000)
      const rowDate = new Date(row.timeLabel ?? "")
      if (rowDate < weekAgo) return false
    }
    return true
  })

  useEffect(() => {
    if (expandedRow && rows.length > 0 && !rows.some((row) => row.id === expandedRow)) {
      setExpandedRow(rows[0]?.id ?? "")
    }
  }, [expandedRow, rows])

  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Unlock History"] : ["Home", "Event History"]}
      title={placeScoped ? t("kisi.eventHistory.unlockHistory") : t("kisi.eventHistory.title")}
      count={resourceQuery.isPending ? "--" : rows.length}
      description={placeScoped ? t("kisi.eventHistory.unlockDesc") : t("kisi.eventHistory.description")}
    >
      {resourceQuery.usingFallback ? (
        <div className="mp-alert-warning">
          Live event resources are unavailable. Showing reference data.
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-line-default bg-white">
        <div className="border-b border-line-subtle px-5 py-4">
          <h2 className="text-base font-semibold text-content-heading">{placeScoped ? t("kisi.eventHistory.unlockHistory") : t("kisi.eventHistory.title")}</h2>
        </div>
        <div className="flex items-center gap-2 border-b border-line-subtle bg-surface-page px-6 py-4 text-xs text-content-subtle">
          <MapPinPlusIcon className="size-4" />
          {t("kisi.eventHistory.timezone")} <span className="text-brand">{t("kisi.accessLink.change")}</span>
        </div>
        <div className="flex flex-col gap-3 border-b border-line-subtle px-6 py-4 md:flex-row md:items-center">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" className="h-10 justify-between font-normal md:w-36">
                {dateFilter === "all" ? t("kisi.eventHistory.allTime") : dateFilter === "today" ? t("kisi.eventHistory.today") : t("kisi.eventHistory.last7Days")}
                <ChevronDownIcon className="ml-2 size-4 text-content-subtle" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-36">
              <DropdownMenuItem className="cursor-pointer" onSelect={() => setDateFilter("all")}>{t("kisi.eventHistory.allTime")}</DropdownMenuItem>
              <DropdownMenuItem className="cursor-pointer" onSelect={() => setDateFilter("today")}>{t("kisi.eventHistory.today")}</DropdownMenuItem>
              <DropdownMenuItem className="cursor-pointer" onSelect={() => setDateFilter("7days")}>{t("kisi.eventHistory.last7Days")}</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" className="h-10 justify-between font-normal md:w-40">
                {actionFilter === "all" ? t("kisi.eventHistory.allActions") : actionFilter}
                <ChevronDownIcon className="ml-2 size-4 text-content-subtle" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="max-h-60 w-40 overflow-y-auto">
              <DropdownMenuItem className="cursor-pointer" onSelect={() => setActionFilter("all")}>{t("kisi.eventHistory.allActions")}</DropdownMenuItem>
              {actionTypes.map((action) => (
                <DropdownMenuItem key={action} className="cursor-pointer" onSelect={() => setActionFilter(action)}>{action}</DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          <MistyisletSearchField value={query} onChange={setQuery} placeholder={t("kisi.eventHistory.searchPlaceholder")} />
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead>
              <tr className="border-b border-line-subtle text-content-body">
                <th className="w-12 px-6 py-4" />
                <th className="px-4 py-4 font-semibold">{t("kisi.eventHistory.object")}</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.eventHistory.action")}</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.eventHistory.user")}</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.eventHistory.time")}</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.eventHistory.status")}</th>
              </tr>
            </thead>
            <tbody>
              {visibleRows.map((row) => {
                const expanded = expandedRow === row.id
                return (
                  <Fragment key={row.id}>
                    <tr className={cn("border-b border-line-subtle last:border-0", expanded && "bg-surface-active")}>
                      <td className="px-6 py-4">
                        <button type="button" onClick={() => setExpandedRow(expanded ? "" : row.id)} aria-label={`Toggle ${row.object} details`}>
                          <ChevronDownIcon className={cn("size-4 text-content-body transition-transform", !expanded && "-rotate-90")} />
                        </button>
                      </td>
                      <td className="px-4 py-4 text-brand">{row.object}</td>
                      <td className="px-4 py-4 text-content-body">{row.action}</td>
                      <td className="px-4 py-4 text-content-subtle">{row.user}</td>
                      <td className="px-4 py-4 text-content-body">{row.timeLabel}</td>
                      <td className="px-4 py-4">
                        <StatusDot tone={row.tone} label={row.statusLabel} />
                      </td>
                    </tr>
                    {expanded ? (
                      <tr className="border-b border-line-subtle bg-[#f7f7f8]">
                        <td className="px-6 py-4" />
                        <td colSpan={5} className="px-4 py-4 text-sm leading-6 text-content-subtle">
                          <span className="font-semibold text-content-body">{t("kisi.eventHistory.details")}</span>
                          <p className="mt-1">{row.details}</p>
                          <EventSnapshotStrip token={token} eventID={row.id} />
                        </td>
                      </tr>
                    ) : null}
                  </Fragment>
                )
              })}
              {visibleRows.length === 0 ? (
                <MistyisletEmptyTableRow colSpan={6}>{t("kisi.eventHistory.noMatch")}</MistyisletEmptyTableRow>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </PageFrame>
  )
}

// --- Event Snapshot Strip ---

function EventSnapshotStrip({ token, eventID }: { token: string; eventID: string }) {
  const { t } = useTranslation()
  const snapshotQuery = useQuery({
    queryKey: ["event-snapshots", eventID],
    queryFn: () => listEventSnapshots(token, eventID),
    enabled: Boolean(token && eventID),
    staleTime: 60_000,
  })

  if (snapshotQuery.isLoading) {
    return (
      <div className="mt-3 flex items-center gap-2 text-xs text-content-muted">
        <CameraIcon className="size-3.5" />
        {t("kisi.eventHistory.loadingSnapshots")}
      </div>
    )
  }

  const snapshots = snapshotQuery.data ?? []
  if (snapshots.length === 0) return null

  return (
    <div className="mt-3">
      <div className="mb-2 flex items-center gap-1.5 text-xs font-semibold text-content-body">
        <ImageIcon className="size-3.5" />
        {t("kisi.eventHistory.snapshotsLabel", { count: snapshots.length })}
      </div>
      <div className="flex gap-2 overflow-x-auto pb-1">
        {snapshots.map((snap) => (
          <SnapshotThumb key={snap.id} snapshot={snap} />
        ))}
      </div>
    </div>
  )
}

function SnapshotThumb({ snapshot }: { snapshot: CameraSnapshot }) {
  const [imgError, setImgError] = useState(false)
  const time = new Date(snapshot.captured_at).toLocaleTimeString()

  if (imgError || !snapshot.signed_url) {
    return (
      <div className="flex h-20 w-28 flex-shrink-0 flex-col items-center justify-center rounded-[4px] border border-line-subtle bg-surface-page text-xs text-content-muted">
        <CameraIcon className="mb-1 size-4" />
        {time}
      </div>
    )
  }

  return (
    <a
      href={snapshot.signed_url}
      target="_blank"
      rel="noopener noreferrer"
      className="group relative flex-shrink-0 overflow-hidden rounded-[4px] border border-line-subtle transition-shadow hover:shadow-md"
    >
      <img
        src={snapshot.signed_url}
        alt={`Snapshot ${time}`}
        className="h-20 w-28 object-cover"
        onError={() => setImgError(true)}
      />
      <span className="absolute bottom-0 left-0 right-0 bg-black/50 px-1.5 py-0.5 text-[10px] text-white">
        {time}
      </span>
    </a>
  )
}
