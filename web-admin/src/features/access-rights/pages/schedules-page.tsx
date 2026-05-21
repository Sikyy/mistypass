import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CalendarClockIcon, PlusIcon, Trash2Icon } from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import { MistyisletEmptyTableRow, MistyisletSearchField } from "@/components/mistyislet/data-display"
import { PageFrame } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  createSchedule,
  deleteSchedule,
  listSchedules,
  updateSchedule,
  type CurrentUser,
  type Schedule,
  type TimeWindow,
} from "@/lib/api"
import { getViewerTenantID } from "@/lib/viewer"

function formatTimeWindows(windows?: TimeWindow[]): string {
  if (!windows || windows.length === 0) return "No restrictions"
  return windows
    .map((tw) => `${tw.day_of_week_set} ${tw.start_time}-${tw.end_time}`)
    .join(", ")
}

export function SchedulesAdaptedPage({
  token,
  viewer,
}: {
  token: string
  viewer: CurrentUser
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const tenantID = getViewerTenantID(viewer)
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(0)
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editSchedule, setEditSchedule] = useState<Schedule | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Schedule | null>(null)
  const [actionNotice, setActionNotice] = useState("")

  const [draftName, setDraftName] = useState("")
  const [draftDescription, setDraftDescription] = useState("")
  const [draftValidFrom, setDraftValidFrom] = useState("")
  const [draftValidUntil, setDraftValidUntil] = useState("")
  const [draftDayOfWeek, setDraftDayOfWeek] = useState("weekday")
  const [draftStartTime, setDraftStartTime] = useState("09:00")
  const [draftEndTime, setDraftEndTime] = useState("18:00")
  const [draftExceptionDates, setDraftExceptionDates] = useState("")

  const pageSize = 20

  const schedulesQuery = useQuery({
    queryKey: ["schedules", tenantID],
    queryFn: () => listSchedules(token, tenantID),
    enabled: !!token,
  })

  const createMutation = useMutation({
    mutationFn: () =>
      createSchedule(token, {
        tenant_id: tenantID,
        name: draftName,
        description: draftDescription,
        valid_from: draftValidFrom || undefined,
        valid_until: draftValidUntil || undefined,
        time_windows: [{ start_time: draftStartTime, end_time: draftEndTime, day_of_week_set: draftDayOfWeek }],
        exception_dates: draftExceptionDates ? draftExceptionDates.split(",").map((d) => d.trim()) : undefined,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      setSheetOpen(false)
      resetDraft()
      setActionNotice(t("kisi.schedules.created"))
    },
  })

  const updateMutation = useMutation({
    mutationFn: () =>
      updateSchedule(
        token,
        editSchedule!.id,
        {
          name: draftName,
          description: draftDescription,
          valid_from: draftValidFrom || undefined,
          valid_until: draftValidUntil || undefined,
          time_windows: [{ start_time: draftStartTime, end_time: draftEndTime, day_of_week_set: draftDayOfWeek }],
          exception_dates: draftExceptionDates ? draftExceptionDates.split(",").map((d) => d.trim()) : undefined,
        },
        tenantID
      ),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      setEditSchedule(null)
      setSheetOpen(false)
      resetDraft()
      setActionNotice(t("kisi.schedules.updated"))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteSchedule(token, id, tenantID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["schedules"] })
      setDeleteTarget(null)
      setActionNotice(t("kisi.schedules.deleted"))
    },
  })

  function resetDraft() {
    setDraftName("")
    setDraftDescription("")
    setDraftValidFrom("")
    setDraftValidUntil("")
    setDraftDayOfWeek("weekday")
    setDraftStartTime("09:00")
    setDraftEndTime("18:00")
    setDraftExceptionDates("")
  }

  function openEdit(s: Schedule) {
    setEditSchedule(s)
    setDraftName(s.name)
    setDraftDescription(s.description ?? "")
    setDraftValidFrom(s.valid_from ?? "")
    setDraftValidUntil(s.valid_until ?? "")
    const tw = s.time_windows?.[0]
    setDraftDayOfWeek(tw?.day_of_week_set ?? "weekday")
    setDraftStartTime(tw?.start_time ?? "09:00")
    setDraftEndTime(tw?.end_time ?? "18:00")
    setDraftExceptionDates((s.exception_dates ?? []).join(", "))
    setSheetOpen(true)
  }

  function openCreate() {
    setEditSchedule(null)
    resetDraft()
    setSheetOpen(true)
  }

  const allItems = schedulesQuery.data ?? []
  const filtered = search
    ? allItems.filter((s) => s.name.toLowerCase().includes(search.toLowerCase()))
    : allItems
  const totalPages = Math.ceil(filtered.length / pageSize)
  const pagedItems = filtered.slice(page * pageSize, (page + 1) * pageSize)

  return (
    <PageFrame
      breadcrumbs={["Home", t("kisi.schedules.title")]}
      title={t("kisi.schedules.title")}
      count={filtered.length}
      description={t("kisi.schedules.description")}
      actions={
        <Button className="h-10 rounded-[8px] px-5" onClick={openCreate}>
          <PlusIcon className="mr-1.5 size-4" />
          {t("kisi.schedules.create")}
        </Button>
      }
    >
      {actionNotice && (
        <div className={cn("mp-alert-success", "mx-0 mb-4 py-3")}>
          {actionNotice}
        </div>
      )}

      <section className="rounded-[6px] border border-line-default bg-white">
        <div className="flex items-center gap-3 border-b border-line-subtle px-6 py-4">
          <MistyisletSearchField value={search} onChange={setSearch} placeholder="Search schedules" />
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-line-subtle bg-surface-page text-left text-xs font-semibold text-content-subtle">
                <th className="px-6 py-3">{t("kisi.schedules.name")}</th>
                <th className="px-6 py-3">{t("kisi.schedules.timeWindows")}</th>
                <th className="px-6 py-3">{t("kisi.schedules.validFrom")}</th>
                <th className="px-6 py-3">{t("kisi.schedules.validUntil")}</th>
                <th className="w-12 px-6 py-3" />
              </tr>
            </thead>
            <tbody>
              {pagedItems.length === 0 ? (
                <MistyisletEmptyTableRow colSpan={5}>No schedules found.</MistyisletEmptyTableRow>
              ) : (
                pagedItems.map((s) => (
                  <tr key={s.id} className="border-b border-line-subtle hover:bg-surface-page">
                    <td className="px-6 py-3">
                      <div className="flex items-center gap-2">
                        <CalendarClockIcon className="size-4 text-content-subtle" />
                        <span className="font-medium text-content-heading">{s.name}</span>
                      </div>
                      {s.description && <p className="mt-0.5 text-xs text-content-subtle">{s.description}</p>}
                    </td>
                    <td className="px-6 py-3 text-content-subtle">{formatTimeWindows(s.time_windows)}</td>
                    <td className="px-6 py-3 text-content-subtle">{s.valid_from ? new Date(s.valid_from).toLocaleDateString() : "-"}</td>
                    <td className="px-6 py-3 text-content-subtle">{s.valid_until ? new Date(s.valid_until).toLocaleDateString() : "-"}</td>
                    <td className="px-6 py-3">
                      <RowActionsMenu
                        items={[
                          { id: "edit", label: t("kisi.schedules.edit"), icon: CalendarClockIcon, onSelect: () => openEdit(s) },
                          { id: "delete", label: t("kisi.schedules.delete"), icon: Trash2Icon, onSelect: () => setDeleteTarget(s), destructive: true },
                        ]}
                      />
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {totalPages > 1 && (
          <div className="grid grid-cols-3 border-t border-line-subtle px-8 py-5 text-sm text-content-subtle">
            <button onClick={() => setPage(Math.max(0, page - 1))} disabled={page === 0} className="text-left disabled:opacity-40">Previous Page</button>
            <span className="text-center text-content-heading">Page {page + 1} of {totalPages}</span>
            <button onClick={() => setPage(Math.min(totalPages - 1, page + 1))} disabled={page >= totalPages - 1} className="text-right disabled:opacity-40">Next Page</button>
          </div>
        )}
      </section>

      <Sheet open={sheetOpen} onOpenChange={(open) => { if (!open) { setSheetOpen(false); setEditSchedule(null) } }}>
        <SheetContent className="w-full max-w-lg overflow-y-auto">
          <SheetHeader>
            <SheetTitle>{editSchedule ? t("kisi.schedules.edit") : t("kisi.schedules.create")}</SheetTitle>
            <SheetDescription>{t("kisi.schedules.description")}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 p-6"
            onSubmit={(e) => {
              e.preventDefault()
              if (editSchedule) updateMutation.mutate()
              else createMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">{t("kisi.schedules.name")}</span>
              <input
                value={draftName}
                onChange={(e) => setDraftName(e.target.value)}
                required
                className="h-10 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm outline-none focus:border-brand-ring"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">Description</span>
              <input
                value={draftDescription}
                onChange={(e) => setDraftDescription(e.target.value)}
                className="h-10 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm outline-none focus:border-brand-ring"
              />
            </label>
            <div className="grid grid-cols-2 gap-4">
              <label className="block">
                <span className="mb-1 block text-xs font-semibold text-content-subtle">{t("kisi.schedules.validFrom")}</span>
                <input
                  type="date"
                  value={draftValidFrom ? draftValidFrom.slice(0, 10) : ""}
                  onChange={(e) => setDraftValidFrom(e.target.value ? e.target.value + "T00:00:00Z" : "")}
                  className="h-10 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm outline-none focus:border-brand-ring"
                />
              </label>
              <label className="block">
                <span className="mb-1 block text-xs font-semibold text-content-subtle">{t("kisi.schedules.validUntil")}</span>
                <input
                  type="date"
                  value={draftValidUntil ? draftValidUntil.slice(0, 10) : ""}
                  onChange={(e) => setDraftValidUntil(e.target.value ? e.target.value + "T23:59:59Z" : "")}
                  className="h-10 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm outline-none focus:border-brand-ring"
                />
              </label>
            </div>
            <div className="rounded-[6px] border border-line-subtle p-4">
              <h4 className="mb-3 text-xs font-semibold text-content-subtle">{t("kisi.schedules.timeWindows")}</h4>
              <div className="grid grid-cols-3 gap-3">
                <label className="block">
                  <span className="mb-1 block text-xs text-content-subtle">Days</span>
                  <select
                    value={draftDayOfWeek}
                    onChange={(e) => setDraftDayOfWeek(e.target.value)}
                    className="h-9 w-full rounded-[6px] border border-line-default bg-white px-2 text-sm outline-none focus:border-brand-ring"
                  >
                    <option value="all">Every day</option>
                    <option value="weekday">Weekdays</option>
                    <option value="weekend">Weekends</option>
                  </select>
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs text-content-subtle">Start</span>
                  <input
                    type="time"
                    value={draftStartTime}
                    onChange={(e) => setDraftStartTime(e.target.value)}
                    className="h-9 w-full rounded-[6px] border border-line-default bg-white px-2 text-sm outline-none focus:border-brand-ring"
                  />
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs text-content-subtle">End</span>
                  <input
                    type="time"
                    value={draftEndTime}
                    onChange={(e) => setDraftEndTime(e.target.value)}
                    className="h-9 w-full rounded-[6px] border border-line-default bg-white px-2 text-sm outline-none focus:border-brand-ring"
                  />
                </label>
              </div>
            </div>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">{t("kisi.schedules.exceptionDates")}</span>
              <input
                value={draftExceptionDates}
                onChange={(e) => setDraftExceptionDates(e.target.value)}
                placeholder="2026-08-17, 2026-12-25"
                className="h-10 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm outline-none focus:border-brand-ring"
              />
              <span className="mt-1 block text-xs text-content-subtle">Comma-separated YYYY-MM-DD dates</span>
            </label>
            <SheetFooter>
              <Button type="submit" disabled={!draftName.trim() || createMutation.isPending || updateMutation.isPending}>
                {(createMutation.isPending || updateMutation.isPending) ? "Saving..." : editSchedule ? "Save" : "Create"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <ConfirmActionDialog
        open={!!deleteTarget}
        title={t("kisi.schedules.delete")}
        description={t("kisi.schedules.deleteConfirm", { name: deleteTarget?.name })}
        confirmLabel={t("kisi.schedules.delete")}
        onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.id)}
        onOpenChange={(open) => { if (!open) setDeleteTarget(null) }}
        pending={deleteMutation.isPending}
        destructive
      />
    </PageFrame>
  )
}
