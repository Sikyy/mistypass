import { Fragment, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronDownIcon, MapPinPlusIcon } from "lucide-react"

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
import type { CurrentUser } from "@/lib/api"
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
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live event resources are unavailable. Showing reference data.
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="border-b border-[#eceef2] px-5 py-4">
          <h2 className="text-base font-semibold text-[#17171c]">{placeScoped ? t("kisi.eventHistory.unlockHistory") : t("kisi.eventHistory.title")}</h2>
        </div>
        <div className="flex items-center gap-2 border-b border-[#eceef2] bg-[#fbfbfc] px-6 py-4 text-xs text-[#6f717c]">
          <MapPinPlusIcon className="size-4" />
          {t("kisi.eventHistory.timezone")} <span className="text-[#4f55ff]">Change</span>
        </div>
        <div className="flex flex-col gap-3 border-b border-[#eceef2] px-6 py-4 md:flex-row md:items-center">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="outline" className="h-10 justify-between font-normal md:w-36">
                {dateFilter === "all" ? t("kisi.eventHistory.allTime") : dateFilter === "today" ? t("kisi.eventHistory.today") : t("kisi.eventHistory.last7Days")}
                <ChevronDownIcon className="ml-2 size-4 text-[#6f717c]" />
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
                <ChevronDownIcon className="ml-2 size-4 text-[#6f717c]" />
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
              <tr className="border-b border-[#eceef2] text-[#2f3037]">
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
                    <tr className={cn("border-b border-[#eceef2] last:border-0", expanded && "bg-[#e7e5df]")}>
                      <td className="px-6 py-4">
                        <button type="button" onClick={() => setExpandedRow(expanded ? "" : row.id)} aria-label={`Toggle ${row.object} details`}>
                          <ChevronDownIcon className={cn("size-4 text-[#2f3037] transition-transform", !expanded && "-rotate-90")} />
                        </button>
                      </td>
                      <td className="px-4 py-4 text-[#4f55ff]">{row.object}</td>
                      <td className="px-4 py-4 text-[#2f3037]">{row.action}</td>
                      <td className="px-4 py-4 text-[#6f717c]">{row.user}</td>
                      <td className="px-4 py-4 text-[#2f3037]">{row.timeLabel}</td>
                      <td className="px-4 py-4">
                        <StatusDot tone={row.tone} label={row.statusLabel} />
                      </td>
                    </tr>
                    {expanded ? (
                      <tr className="border-b border-[#eceef2] bg-[#f7f7f8]">
                        <td className="px-6 py-4" />
                        <td colSpan={5} className="px-4 py-4 text-sm leading-6 text-[#6f717c]">
                          <span className="font-semibold text-[#2f3037]">{t("kisi.eventHistory.details")}</span>
                          <p className="mt-1">{row.details}</p>
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
