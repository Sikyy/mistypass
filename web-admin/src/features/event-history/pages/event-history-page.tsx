import { Fragment, useEffect, useMemo, useState } from "react"
import { ChevronDownIcon, MapPinPlusIcon } from "lucide-react"

import { KisiEmptyTableRow, KisiFilterButton, KisiSearchField } from "@/components/kisi/data-display"
import { PageFrame, StatusDot } from "@/components/kisi/primitives"
import { selectKisiPlaceContext } from "@/features/kisi-shell/resource-data"
import { useKisiResourceSummary } from "@/features/kisi-shell/use-resource-summary"
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
  const resourceQuery = useKisiResourceSummary(token, viewer)
  const placeContext = useMemo(
    () => selectKisiPlaceContext(resourceQuery.summary, placeID),
    [placeID, resourceQuery.summary]
  )
  const rows = placeScoped ? placeContext.events : resourceQuery.summary.events
  const [expandedRow, setExpandedRow] = useState("")
  const [query, setQuery] = useState("")
  const visibleRows = rows.filter((row) => {
    const normalizedQuery = query.trim().toLowerCase()
    if (normalizedQuery === "") {
      return true
    }
    return [row.object, row.action, row.user, row.timeLabel, row.statusLabel, row.details]
      .join(" ")
      .toLowerCase()
      .includes(normalizedQuery)
  })

  useEffect(() => {
    if (!expandedRow && rows[0]) {
      setExpandedRow(rows[0].id)
      return
    }
    if (expandedRow && rows.length > 0 && !rows.some((row) => row.id === expandedRow)) {
      setExpandedRow(rows[0].id)
    }
  }, [expandedRow, rows])

  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Unlock History"] : ["Home", "Event History"]}
      title={placeScoped ? "Unlock History" : "Event History"}
      count={resourceQuery.isPending ? "--" : rows.length}
      description={placeScoped ? "View and filter place unlock events" : "View and filter previous events"}
    >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live event resources are unavailable. Showing reference data.
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="border-b border-[#eceef2] px-5 py-4">
          <h2 className="text-base font-semibold text-[#17171c]">{placeScoped ? "Unlock History" : "Event History"}</h2>
        </div>
        <div className="flex items-center gap-2 border-b border-[#eceef2] bg-[#fbfbfc] px-6 py-4 text-xs text-[#6f717c]">
          <MapPinPlusIcon className="size-4" />
          The timezone is Indonesia (Jakarta). <span className="text-[#4f55ff]">Change</span>
        </div>
        <div className="flex flex-col gap-3 border-b border-[#eceef2] px-6 py-4 md:flex-row md:items-center">
          <KisiFilterButton label="Today" className="font-normal md:w-36" />
          <KisiFilterButton label="All Actions" className="font-normal md:w-40" />
          <KisiSearchField value={query} onChange={setQuery} placeholder="Search events..." />
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead>
              <tr className="border-b border-[#eceef2] text-[#2f3037]">
                <th className="w-12 px-6 py-4" />
                <th className="px-4 py-4 font-semibold">Object</th>
                <th className="px-4 py-4 font-semibold">Action</th>
                <th className="px-4 py-4 font-semibold">User</th>
                <th className="px-4 py-4 font-semibold">Time</th>
                <th className="px-4 py-4 font-semibold">Status</th>
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
                          <span className="font-semibold text-[#2f3037]">Details</span>
                          <p className="mt-1">{row.details}</p>
                        </td>
                      </tr>
                    ) : null}
                  </Fragment>
                )
              })}
              {visibleRows.length === 0 ? (
                <KisiEmptyTableRow colSpan={6}>No events match this search.</KisiEmptyTableRow>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>
    </PageFrame>
  )
}
