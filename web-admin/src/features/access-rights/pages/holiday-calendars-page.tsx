import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CalendarDaysIcon, ChevronDownIcon, ChevronRightIcon, DownloadIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import { MistyisletEmptyTableRow, MistyisletSearchField } from "@/components/mistyislet/data-display"
import { PageFrame } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  createHolidayCalendar,
  deleteHolidayCalendar,
  listHolidayCalendarPresetCountries,
  listHolidayCalendarPresets,
  listHolidayCalendars,
  updateHolidayCalendar,
  type CurrentUser,
  type HolidayCalendar,
  type HolidayEntry,
  type HolidayPresetCountry,
} from "@/lib/api"
import { formatDate } from "@/lib/format"
import { getViewerTenantID } from "@/lib/viewer"

export function HolidayCalendarsAdaptedPage({
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
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editCalendar, setEditCalendar] = useState<HolidayCalendar | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<HolidayCalendar | null>(null)
  const [expandedID, setExpandedID] = useState<string | null>(null)
  const [actionError, setActionError] = useState("")

  const [draftName, setDraftName] = useState("")
  const [draftCountry, setDraftCountry] = useState("")
  const [draftYear, setDraftYear] = useState(new Date().getFullYear())
  const [draftEntries, setDraftEntries] = useState<HolidayEntry[]>([])
  const [newEntryDate, setNewEntryDate] = useState("")
  const [newEntryName, setNewEntryName] = useState("")
  const [newEntryDescription, setNewEntryDescription] = useState("")

  const calendarsQuery = useQuery({
    queryKey: ["holiday-calendars", tenantID],
    queryFn: () => listHolidayCalendars(token, tenantID),
    enabled: Boolean(tenantID),
    staleTime: 30 * 1000,
  })

  const countriesQuery = useQuery({
    queryKey: ["holiday-calendar-preset-countries"],
    queryFn: () => listHolidayCalendarPresetCountries(token),
    staleTime: 5 * 60 * 1000,
  })

  const presetsQuery = useQuery({
    queryKey: ["holiday-calendar-presets", draftCountry, draftYear],
    queryFn: () => listHolidayCalendarPresets(token, draftCountry, draftYear),
    enabled: Boolean(draftCountry) && sheetOpen,
    staleTime: 5 * 60 * 1000,
  })

  const createMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) throw new Error("tenant_id is required")
      return createHolidayCalendar(token, {
        tenant_id: tenantID,
        name: draftName.trim(),
        country: draftCountry || undefined,
        entries: draftEntries,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["holiday-calendars"] })
      setSheetOpen(false)
      resetDraft()
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Create failed"),
  })

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editCalendar) throw new Error("calendar is required")
      return updateHolidayCalendar(token, editCalendar.id, {
        name: draftName.trim(),
        country: draftCountry || undefined,
        entries: draftEntries,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["holiday-calendars"] })
      setSheetOpen(false)
      setEditCalendar(null)
      resetDraft()
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Update failed"),
  })

  const deleteMutation = useMutation({
    mutationFn: (calendar: HolidayCalendar) =>
      deleteHolidayCalendar(token, calendar.id, tenantID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["holiday-calendars"] })
      setDeleteTarget(null)
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Delete failed"),
  })

  function resetDraft() {
    setDraftName("")
    setDraftCountry("")
    setDraftYear(new Date().getFullYear())
    setDraftEntries([])
    setNewEntryDate("")
    setNewEntryName("")
    setNewEntryDescription("")
  }

  function openCreate() {
    setEditCalendar(null)
    resetDraft()
    setActionError("")
    setSheetOpen(true)
  }

  function openEdit(calendar: HolidayCalendar) {
    setEditCalendar(calendar)
    setDraftName(calendar.name)
    setDraftCountry(calendar.country ?? "")
    setDraftYear(new Date().getFullYear())
    setDraftEntries([...calendar.entries])
    setNewEntryDate("")
    setNewEntryName("")
    setNewEntryDescription("")
    setActionError("")
    setSheetOpen(true)
  }

  function loadPresets() {
    if (!presetsQuery.data) return
    setDraftEntries(presetsQuery.data.entries)
  }

  function addManualEntry() {
    if (!newEntryDate.trim() || !newEntryName.trim()) return
    setDraftEntries((prev) => [
      ...prev,
      {
        date: newEntryDate.trim(),
        name: newEntryName.trim(),
        description: newEntryDescription.trim() || undefined,
      },
    ])
    setNewEntryDate("")
    setNewEntryName("")
    setNewEntryDescription("")
  }

  function removeEntry(index: number) {
    setDraftEntries((prev) => prev.filter((_, i) => i !== index))
  }

  function toggleExpanded(id: string) {
    setExpandedID((prev) => (prev === id ? null : id))
  }

  const allCalendars = calendarsQuery.data ?? []
  const countries = countriesQuery.data ?? []
  const filtered = search
    ? allCalendars.filter(
        (cal) =>
          cal.name.toLowerCase().includes(search.toLowerCase()) ||
          (cal.country ?? "").toLowerCase().includes(search.toLowerCase())
      )
    : allCalendars
  const canMutate = Boolean(tenantID)
  const canSubmit = canMutate && Boolean(draftName.trim()) && draftEntries.length > 0

  return (
    <>
      <PageFrame
        breadcrumbs={["Home", "Holiday Calendars"]}
        title="Holiday Calendars"
        count={calendarsQuery.isPending ? "--" : allCalendars.length}
        description="Manage holiday calendars for access schedule exceptions. Load country presets or add entries manually."
        actions={
          <Button
            disabled={!canMutate}
            onClick={openCreate}
            className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            New Calendar
          </Button>
        }
      >
        {actionError ? (
          <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
            {actionError}
          </div>
        ) : null}

        <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
          <div className="flex items-center gap-3 border-b border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
            <MistyisletSearchField
              value={search}
              onChange={setSearch}
              placeholder="Search holiday calendars..."
            />
          </div>

          <div className="overflow-x-auto">
            <table className="w-full min-w-[700px] text-left text-sm">
              <thead>
                <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-left text-xs font-semibold text-[#6f717c]">
                  <th className="w-10 px-6 py-3" />
                  <th className="px-4 py-3">{t("common.name")}</th>
                  <th className="px-4 py-3">Country</th>
                  <th className="px-4 py-3">Entries</th>
                  <th className="px-4 py-3">Last Updated</th>
                  <th className="w-12 px-4 py-3" />
                </tr>
              </thead>
              <tbody>
                {filtered.map((calendar) => (
                  <CalendarRow
                    key={calendar.id}
                    calendar={calendar}
                    expanded={expandedID === calendar.id}
                    onToggle={() => toggleExpanded(calendar.id)}
                    canMutate={canMutate}
                    onEdit={() => openEdit(calendar)}
                    onDelete={() => {
                      setActionError("")
                      setDeleteTarget(calendar)
                    }}
                    isPending={deleteMutation.isPending || updateMutation.isPending}
                  />
                ))}
                {calendarsQuery.isPending ? (
                  <MistyisletEmptyTableRow colSpan={6}>{t("common.loading")}</MistyisletEmptyTableRow>
                ) : null}
                {!calendarsQuery.isPending && filtered.length === 0 ? (
                  <MistyisletEmptyTableRow colSpan={6}>
                    {search ? "No calendars match your search." : "No holiday calendars yet. Create one to get started."}
                  </MistyisletEmptyTableRow>
                ) : null}
              </tbody>
            </table>
          </div>
        </section>
      </PageFrame>

      <ConfirmActionDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!deleteMutation.isPending && !open) {
            setDeleteTarget(null)
          }
        }}
        title="Delete Holiday Calendar"
        description={
          <>
            This will permanently delete{" "}
            <span className="font-semibold text-[#17171c]">{deleteTarget?.name ?? "this calendar"}</span>{" "}
            and all its entries.
          </>
        }
        confirmLabel="Delete Calendar"
        pending={deleteMutation.isPending}
        disabled={!canMutate || !deleteTarget}
        destructive
        onConfirm={() => {
          if (deleteTarget) {
            deleteMutation.mutate(deleteTarget)
          }
        }}
      />

      <Sheet
        open={sheetOpen}
        onOpenChange={(open) => {
          if (!open) {
            setSheetOpen(false)
            setEditCalendar(null)
          }
        }}
      >
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[520px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>{editCalendar ? "Edit Holiday Calendar" : "New Holiday Calendar"}</SheetTitle>
            <SheetDescription>
              {editCalendar
                ? "Update the calendar name, country, and holiday entries."
                : "Create a calendar with country presets or manually add entries."}
            </SheetDescription>
          </SheetHeader>

          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              if (editCalendar) {
                updateMutation.mutate()
              } else {
                createMutation.mutate()
              }
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.name")}</span>
              <input
                value={draftName}
                onChange={(event) => setDraftName(event.target.value)}
                required
                placeholder="e.g. Indonesia 2026"
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff]"
              />
            </label>

            <div className="rounded-[6px] border border-[#eceef2] bg-[#fbfbfc] p-4">
              <h4 className="mb-3 text-xs font-semibold text-[#6f717c]">Load Country Presets</h4>
              <div className="grid grid-cols-3 gap-3">
                <label className="col-span-1 block">
                  <span className="mb-1 block text-xs text-[#6f717c]">Country</span>
                  <select
                    value={draftCountry}
                    onChange={(event) => setDraftCountry(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff]"
                  >
                    <option value="">Select country</option>
                    {countries.map((country: HolidayPresetCountry) => (
                      <option key={country.code} value={country.code}>
                        {country.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs text-[#6f717c]">Year</span>
                  <input
                    type="number"
                    value={draftYear}
                    min={2020}
                    max={2040}
                    onChange={(event) => setDraftYear(Number(event.target.value))}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff]"
                  />
                </label>
                <div className="flex items-end">
                  <Button
                    type="button"
                    disabled={!draftCountry || presetsQuery.isPending}
                    onClick={loadPresets}
                    className="h-11 w-full rounded-[6px] border border-[#c8cad6] bg-white px-3 text-sm font-semibold text-[#2f3037] hover:bg-[#f3f4f8] disabled:bg-[#f8f8fa] disabled:text-[#9a9ca7]"
                  >
                    <DownloadIcon className="mr-1.5 size-4" />
                    {presetsQuery.isPending ? "Loading..." : "Load"}
                  </Button>
                </div>
              </div>
              {presetsQuery.data ? (
                <p className="mt-2 text-xs text-[#6f717c]">
                  {presetsQuery.data.entries.length} presets available for {presetsQuery.data.country_name} {presetsQuery.data.year}
                </p>
              ) : null}
            </div>

            <div className="rounded-[6px] border border-[#eceef2] p-4">
              <h4 className="mb-3 text-xs font-semibold text-[#6f717c]">Add Entry Manually</h4>
              <div className="grid grid-cols-3 gap-3">
                <label className="block">
                  <span className="mb-1 block text-xs text-[#6f717c]">Date</span>
                  <input
                    type="date"
                    value={newEntryDate}
                    onChange={(event) => setNewEntryDate(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff]"
                  />
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs text-[#6f717c]">{t("common.name")}</span>
                  <input
                    value={newEntryName}
                    onChange={(event) => setNewEntryName(event.target.value)}
                    placeholder="Holiday name"
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff]"
                  />
                </label>
                <div className="flex items-end">
                  <Button
                    type="button"
                    disabled={!newEntryDate.trim() || !newEntryName.trim()}
                    onClick={addManualEntry}
                    className="h-11 w-full rounded-[6px] border border-[#c8cad6] bg-white px-3 text-sm font-semibold text-[#2f3037] hover:bg-[#f3f4f8] disabled:bg-[#f8f8fa] disabled:text-[#9a9ca7]"
                  >
                    <PlusIcon className="mr-1.5 size-4" />
                    Add
                  </Button>
                </div>
              </div>
              <label className="mt-3 block">
                <span className="mb-1 block text-xs text-[#6f717c]">Description (optional)</span>
                <input
                  value={newEntryDescription}
                  onChange={(event) => setNewEntryDescription(event.target.value)}
                  placeholder="Optional description"
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff]"
                />
              </label>
            </div>

            <div>
              <div className="mb-2 flex items-center justify-between">
                <h4 className="text-xs font-semibold text-[#6f717c]">
                  Entries ({draftEntries.length})
                </h4>
                {draftEntries.length > 0 ? (
                  <button
                    type="button"
                    onClick={() => setDraftEntries([])}
                    className="text-xs font-semibold text-[#e0463e] hover:underline"
                  >
                    Clear all
                  </button>
                ) : null}
              </div>
              {draftEntries.length === 0 ? (
                <div className="rounded-[6px] border border-[#eceef2] bg-[#fbfbfc] px-4 py-6 text-center text-sm text-[#6f717c]">
                  No entries yet. Load country presets or add entries manually.
                </div>
              ) : (
                <div className="max-h-[280px] overflow-y-auto rounded-[6px] border border-[#eceef2]">
                  <table className="w-full text-left text-sm">
                    <thead className="sticky top-0 bg-[#fbfbfc]">
                      <tr className="border-b border-[#eceef2] text-xs font-semibold text-[#6f717c]">
                        <th className="px-3 py-2">Date</th>
                        <th className="px-3 py-2">{t("common.name")}</th>
                        <th className="w-8 px-3 py-2" />
                      </tr>
                    </thead>
                    <tbody>
                      {draftEntries.map((entry, index) => (
                        <tr key={`${entry.date}-${index}`} className="border-b border-[#eceef2] last:border-0">
                          <td className="px-3 py-2 text-[#2f3037]">{entry.date}</td>
                          <td className="px-3 py-2 text-[#17171c]">
                            <div>{entry.name}</div>
                            {entry.description ? (
                              <div className="text-xs text-[#6f717c]">{entry.description}</div>
                            ) : null}
                          </td>
                          <td className="px-3 py-2">
                            <button
                              type="button"
                              onClick={() => removeEntry(index)}
                              className="text-[#9a9ca7] hover:text-[#e0463e]"
                              aria-label={`Remove ${entry.name}`}
                            >
                              <Trash2Icon className="size-3.5" />
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>

            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={!canSubmit || createMutation.isPending || updateMutation.isPending}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {createMutation.isPending || updateMutation.isPending
                  ? "Saving..."
                  : editCalendar
                    ? "Save Changes"
                    : "Create Calendar"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </>
  )
}

function CalendarRow({
  calendar,
  expanded,
  onToggle,
  canMutate,
  onEdit,
  onDelete,
  isPending,
}: {
  calendar: HolidayCalendar
  expanded: boolean
  onToggle: () => void
  canMutate: boolean
  onEdit: () => void
  onDelete: () => void
  isPending: boolean
}) {
  return (
    <>
      <tr className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
        <td className="px-6 py-4">
          <button
            type="button"
            onClick={onToggle}
            className="text-[#6f717c] hover:text-[#17171c]"
            aria-label={expanded ? "Collapse entries" : "Expand entries"}
          >
            {expanded ? (
              <ChevronDownIcon className="size-4" />
            ) : (
              <ChevronRightIcon className="size-4" />
            )}
          </button>
        </td>
        <td className="px-4 py-4">
          <div className="flex items-center gap-2">
            <CalendarDaysIcon className="size-4 text-[#6f717c]" />
            <span className="font-semibold text-[#17171c]">{calendar.name}</span>
          </div>
        </td>
        <td className="px-4 py-4 text-[#2f3037]">{calendar.country ?? "-"}</td>
        <td className="px-4 py-4 text-[#6f717c]">{calendar.entries.length}</td>
        <td className="px-4 py-4 text-[#6f717c]">{formatDate(calendar.updated_at)}</td>
        <td className="px-4 py-4">
          <RowActionsMenu
            label={`Actions for ${calendar.name}`}
            items={[
              {
                id: "edit",
                label: "Edit",
                icon: PencilIcon,
                disabled: !canMutate || isPending,
                onSelect: onEdit,
              },
              {
                id: "delete",
                label: "Delete",
                icon: Trash2Icon,
                destructive: true,
                disabled: !canMutate || isPending,
                onSelect: onDelete,
              },
            ]}
          />
        </td>
      </tr>
      {expanded && calendar.entries.length > 0 ? (
        <tr className="border-b border-[#eceef2]">
          <td colSpan={6} className="bg-[#fbfbfc] px-6 py-3">
            <div className="overflow-hidden rounded-[6px] border border-[#eceef2] bg-white">
              <table className="w-full text-left text-sm">
                <thead>
                  <tr className="border-b border-[#eceef2] bg-[#fbfbfc] text-xs font-semibold text-[#6f717c]">
                    <th className="px-4 py-2">Date</th>
                    <th className="px-4 py-2">Name</th>
                    <th className="px-4 py-2">Description</th>
                  </tr>
                </thead>
                <tbody>
                  {calendar.entries.map((entry, index) => (
                    <tr
                      key={`${entry.date}-${index}`}
                      className="border-b border-[#eceef2] last:border-0"
                    >
                      <td className="px-4 py-2 text-[#2f3037]">{entry.date}</td>
                      <td className="px-4 py-2 font-medium text-[#17171c]">{entry.name}</td>
                      <td className="px-4 py-2 text-[#6f717c]">{entry.description ?? "-"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </td>
        </tr>
      ) : null}
      {expanded && calendar.entries.length === 0 ? (
        <tr className="border-b border-[#eceef2]">
          <td colSpan={6} className="bg-[#fbfbfc] px-6 py-4 text-center text-sm text-[#6f717c]">
            No entries in this calendar.
          </td>
        </tr>
      ) : null}
    </>
  )
}
