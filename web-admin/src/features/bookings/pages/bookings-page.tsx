import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  CalendarCheckIcon,
  DoorOpenIcon,
  LayoutGridIcon,
  LogInIcon,
  LogOutIcon,
  PencilIcon,
  PhoneIcon,
  PlusIcon,
  StarIcon,
  Trash2Icon,
  XCircleIcon,
} from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import { MistyisletEmptyTableRow, MistyisletSearchField } from "@/components/mistyislet/data-display"
import { PageFrame, StatusDot } from "@/components/mistyislet/primitives"
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
  checkInBooking,
  checkOutBooking,
  createBookableSpace,
  createBooking,
  deleteBookableSpace,
  deleteBooking,
  getBookableSpaceStatus,
  listBookableSpaces,
  listBookings,
  updateBookableSpace,
  updateBooking,
  type BookableSpace,
  type BookableSpaceStatus,
  type Booking,
  type CurrentUser,
} from "@/lib/api"
import { formatDateTime } from "@/lib/format"
import { getViewerTenantID } from "@/lib/viewer"

// --- Helpers ---

const SPACE_TYPE_OPTIONS: { value: BookableSpace["space_type"]; label: string }[] = [
  { value: "meeting_room", label: "Meeting Room" },
  { value: "prayer_room", label: "Prayer Room" },
  { value: "phone_booth", label: "Phone Booth" },
  { value: "custom", label: "Custom" },
]

const CAPACITY_MODE_OPTIONS: { value: BookableSpace["capacity_mode"]; label: string }[] = [
  { value: "single_occupancy", label: "Single Occupancy" },
  { value: "limited_capacity", label: "Limited Capacity" },
  { value: "unlimited", label: "Unlimited" },
]

function spaceTypeIcon(type: BookableSpace["space_type"]) {
  switch (type) {
    case "meeting_room":
      return DoorOpenIcon
    case "prayer_room":
      return StarIcon
    case "phone_booth":
      return PhoneIcon
    case "custom":
      return LayoutGridIcon
  }
}

function spaceTypeLabel(type: BookableSpace["space_type"]): string {
  return SPACE_TYPE_OPTIONS.find((o) => o.value === type)?.label ?? type
}

function capacityModeLabel(mode: BookableSpace["capacity_mode"]): string {
  return CAPACITY_MODE_OPTIONS.find((o) => o.value === mode)?.label ?? mode
}

function bookingStatusTone(status: Booking["status"]): "info" | "success" | "warning" | "danger" {
  switch (status) {
    case "confirmed":
      return "info"
    case "checked_in":
      return "success"
    case "completed":
      return "warning"
    case "cancelled":
      return "danger"
    case "no_show":
      return "warning"
    default:
      return "info"
  }
}

function bookingStatusLabel(status: Booking["status"]): string {
  switch (status) {
    case "confirmed":
      return "Confirmed"
    case "checked_in":
      return "Checked In"
    case "completed":
      return "Completed"
    case "cancelled":
      return "Cancelled"
    case "no_show":
      return "No Show"
    default:
      return status
  }
}

// --- Initial form state ---

type SpaceFormState = {
  name: string
  description: string
  space_type: BookableSpace["space_type"]
  capacity_mode: BookableSpace["capacity_mode"]
  max_capacity: number
  lock_id: string
  requires_booking: boolean
  enabled: boolean
}

const INITIAL_SPACE_FORM: SpaceFormState = {
  name: "",
  description: "",
  space_type: "meeting_room",
  capacity_mode: "single_occupancy",
  max_capacity: 1,
  lock_id: "",
  requires_booking: true,
  enabled: true,
}

type BookingFormState = {
  space_id: string
  title: string
  start_time: string
  end_time: string
}

const INITIAL_BOOKING_FORM: BookingFormState = {
  space_id: "",
  title: "",
  start_time: "",
  end_time: "",
}

// --- Main component ---

type BookingsPageProps = {
  token: string
  viewer: CurrentUser
}

export function BookingsPage({ token, viewer }: BookingsPageProps) {
  const queryClient = useQueryClient()
  const tenantID = getViewerTenantID(viewer)

  // Space state
  const [spaceSheetOpen, setSpaceSheetOpen] = useState(false)
  const [editSpace, setEditSpace] = useState<BookableSpace | null>(null)
  const [deleteSpaceTarget, setDeleteSpaceTarget] = useState<BookableSpace | null>(null)
  const [spaceForm, setSpaceForm] = useState<SpaceFormState>(INITIAL_SPACE_FORM)
  const [selectedSpaceID, setSelectedSpaceID] = useState<string | null>(null)

  // Booking state
  const [bookingSheetOpen, setBookingSheetOpen] = useState(false)
  const [editBooking, setEditBooking] = useState<Booking | null>(null)
  const [deleteBookingTarget, setDeleteBookingTarget] = useState<Booking | null>(null)
  const [bookingForm, setBookingForm] = useState<BookingFormState>(INITIAL_BOOKING_FORM)
  const [filterSpaceID, setFilterSpaceID] = useState("")

  const [actionError, setActionError] = useState("")

  // --- Queries ---

  const spacesQuery = useQuery({
    queryKey: ["bookable-spaces", tenantID],
    queryFn: () => listBookableSpaces(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const bookingsQuery = useQuery({
    queryKey: ["bookings", tenantID, filterSpaceID],
    queryFn: () => listBookings(token, tenantID, filterSpaceID || undefined),
    enabled: Boolean(token && tenantID),
  })

  const spaceStatusQuery = useQuery({
    queryKey: ["bookable-space-status", selectedSpaceID, tenantID],
    queryFn: () => getBookableSpaceStatus(token, selectedSpaceID!, tenantID),
    enabled: Boolean(token && tenantID && selectedSpaceID),
  })

  // --- Space mutations ---

  const createSpaceMutation = useMutation({
    mutationFn: () =>
      createBookableSpace(token, {
        tenant_id: tenantID,
        name: spaceForm.name.trim(),
        description: spaceForm.description.trim() || undefined,
        space_type: spaceForm.space_type,
        capacity_mode: spaceForm.capacity_mode,
        max_capacity: spaceForm.capacity_mode === "limited_capacity" ? spaceForm.max_capacity : undefined,
        lock_id: spaceForm.lock_id.trim() || undefined,
        requires_booking: spaceForm.requires_booking,
        enabled: spaceForm.enabled,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bookable-spaces"] })
      setSpaceSheetOpen(false)
      setSpaceForm(INITIAL_SPACE_FORM)
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Failed to create space"),
  })

  const updateSpaceMutation = useMutation({
    mutationFn: () => {
      if (!editSpace) throw new Error("space is required")
      return updateBookableSpace(token, editSpace.id, {
        tenant_id: tenantID,
        name: spaceForm.name.trim(),
        description: spaceForm.description.trim() || undefined,
        space_type: spaceForm.space_type,
        capacity_mode: spaceForm.capacity_mode,
        max_capacity: spaceForm.capacity_mode === "limited_capacity" ? spaceForm.max_capacity : undefined,
        lock_id: spaceForm.lock_id.trim() || undefined,
        requires_booking: spaceForm.requires_booking,
        enabled: spaceForm.enabled,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bookable-spaces"] })
      await queryClient.invalidateQueries({ queryKey: ["bookable-space-status"] })
      setSpaceSheetOpen(false)
      setEditSpace(null)
      setSpaceForm(INITIAL_SPACE_FORM)
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Failed to update space"),
  })

  const deleteSpaceMutation = useMutation({
    mutationFn: (space: BookableSpace) => deleteBookableSpace(token, space.id, tenantID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bookable-spaces"] })
      if (deleteSpaceTarget && selectedSpaceID === deleteSpaceTarget.id) {
        setSelectedSpaceID(null)
      }
      setDeleteSpaceTarget(null)
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Failed to delete space"),
  })

  // --- Booking mutations ---

  const createBookingMutation = useMutation({
    mutationFn: () =>
      createBooking(token, {
        tenant_id: tenantID,
        space_id: bookingForm.space_id,
        title: bookingForm.title.trim() || undefined,
        start_time: new Date(bookingForm.start_time).toISOString(),
        end_time: new Date(bookingForm.end_time).toISOString(),
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bookings"] })
      await queryClient.invalidateQueries({ queryKey: ["bookable-space-status"] })
      setBookingSheetOpen(false)
      setBookingForm(INITIAL_BOOKING_FORM)
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Failed to create booking"),
  })

  const updateBookingMutation = useMutation({
    mutationFn: () => {
      if (!editBooking) throw new Error("booking is required")
      return updateBooking(token, editBooking.id, {
        tenant_id: tenantID,
        title: bookingForm.title.trim() || undefined,
        start_time: bookingForm.start_time ? new Date(bookingForm.start_time).toISOString() : undefined,
        end_time: bookingForm.end_time ? new Date(bookingForm.end_time).toISOString() : undefined,
      })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bookings"] })
      await queryClient.invalidateQueries({ queryKey: ["bookable-space-status"] })
      setBookingSheetOpen(false)
      setEditBooking(null)
      setBookingForm(INITIAL_BOOKING_FORM)
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Failed to update booking"),
  })

  const deleteBookingMutation = useMutation({
    mutationFn: (booking: Booking) => deleteBooking(token, booking.id, tenantID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bookings"] })
      await queryClient.invalidateQueries({ queryKey: ["bookable-space-status"] })
      setDeleteBookingTarget(null)
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Failed to delete booking"),
  })

  const checkInMutation = useMutation({
    mutationFn: (bookingID: string) => checkInBooking(token, bookingID, tenantID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bookings"] })
      await queryClient.invalidateQueries({ queryKey: ["bookable-space-status"] })
      await queryClient.invalidateQueries({ queryKey: ["bookable-spaces"] })
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Failed to check in"),
  })

  const checkOutMutation = useMutation({
    mutationFn: (bookingID: string) => checkOutBooking(token, bookingID, tenantID),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bookings"] })
      await queryClient.invalidateQueries({ queryKey: ["bookable-space-status"] })
      await queryClient.invalidateQueries({ queryKey: ["bookable-spaces"] })
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Failed to check out"),
  })

  const cancelBookingMutation = useMutation({
    mutationFn: (booking: Booking) =>
      updateBooking(token, booking.id, { tenant_id: tenantID }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["bookings"] })
      await queryClient.invalidateQueries({ queryKey: ["bookable-space-status"] })
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Failed to cancel booking"),
  })

  // --- Helpers ---

  const spaces = spacesQuery.data ?? []
  const bookings = bookingsQuery.data ?? []
  const canMutate = Boolean(tenantID)
  const spaceStatus: BookableSpaceStatus | undefined = spaceStatusQuery.data

  function openCreateSpace() {
    setEditSpace(null)
    setSpaceForm(INITIAL_SPACE_FORM)
    setActionError("")
    setSpaceSheetOpen(true)
  }

  function openEditSpace(space: BookableSpace) {
    setEditSpace(space)
    setSpaceForm({
      name: space.name,
      description: space.description ?? "",
      space_type: space.space_type,
      capacity_mode: space.capacity_mode,
      max_capacity: space.max_capacity,
      lock_id: space.lock_id ?? "",
      requires_booking: space.requires_booking,
      enabled: space.enabled,
    })
    setActionError("")
    setSpaceSheetOpen(true)
  }

  function openCreateBooking() {
    setEditBooking(null)
    setBookingForm(INITIAL_BOOKING_FORM)
    setActionError("")
    setBookingSheetOpen(true)
  }

  function openEditBooking(booking: Booking) {
    setEditBooking(booking)
    setBookingForm({
      space_id: booking.space_id,
      title: booking.title ?? "",
      start_time: booking.start_time ? toDatetimeLocalValue(booking.start_time) : "",
      end_time: booking.end_time ? toDatetimeLocalValue(booking.end_time) : "",
    })
    setActionError("")
    setBookingSheetOpen(true)
  }

  function spaceNameByID(id: string): string {
    return spaces.find((s) => s.id === id)?.name ?? id
  }

  return (
    <>
      <PageFrame
        breadcrumbs={["Home", "Bookings"]}
        title="Bookings"
        description="Manage bookable spaces and reservations for meeting rooms, prayer rooms, phone booths, and more."
      >
        {actionError ? (
          <div className="rounded-[6px] border border-[#f1c27a] bg-warning-bg px-5 py-4 text-sm text-warning-text">
            {actionError}
          </div>
        ) : null}

        {/* Section A: Bookable Spaces */}
        <section>
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-content-heading">Bookable Spaces</h2>
            <Button
              disabled={!canMutate}
              onClick={openCreateSpace}
              className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
            >
              <PlusIcon className="mr-1.5 size-4" />
              Create Space
            </Button>
          </div>

          <div className="overflow-hidden rounded-[6px] border border-line-default bg-white">
            <div className="overflow-x-auto">
              <table className="w-full min-w-[800px] text-left text-sm">
                <thead>
                  <tr className="border-b border-line-subtle bg-surface-page text-left text-xs font-semibold text-content-subtle">
                    <th className="px-4 py-3">Name</th>
                    <th className="px-4 py-3">Type</th>
                    <th className="px-4 py-3">Capacity Mode</th>
                    <th className="px-4 py-3">Max Capacity</th>
                    <th className="px-4 py-3">Current Occupancy</th>
                    <th className="px-4 py-3">Status</th>
                    <th className="w-12 px-4 py-3" />
                  </tr>
                </thead>
                <tbody>
                  {spaces.map((space) => {
                    const Icon = spaceTypeIcon(space.space_type)
                    return (
                      <tr
                        key={space.id}
                        className={
                          "cursor-pointer border-b border-line-subtle last:border-0 hover:bg-surface-page" +
                          (selectedSpaceID === space.id ? " bg-brand-subtle" : "")
                        }
                        onClick={() => setSelectedSpaceID(selectedSpaceID === space.id ? null : space.id)}
                      >
                        <td className="px-4 py-4 font-semibold text-content-heading">{space.name}</td>
                        <td className="px-4 py-4">
                          <span className="inline-flex items-center gap-1.5 text-content-body">
                            <Icon className="size-4 text-content-subtle" />
                            {spaceTypeLabel(space.space_type)}
                          </span>
                        </td>
                        <td className="px-4 py-4 text-content-body">{capacityModeLabel(space.capacity_mode)}</td>
                        <td className="px-4 py-4 text-content-body">{space.max_capacity}</td>
                        <td className="px-4 py-4 text-content-body">{space.current_occupancy}</td>
                        <td className="px-4 py-4">
                          <StatusDot
                            tone={space.enabled ? "success" : "danger"}
                            label={space.enabled ? "Enabled" : "Disabled"}
                          />
                        </td>
                        <td className="px-4 py-4" onClick={(e) => e.stopPropagation()}>
                          <RowActionsMenu
                            label={`Actions for ${space.name}`}
                            items={[
                              {
                                id: "edit",
                                label: "Edit",
                                icon: PencilIcon,
                                disabled: !canMutate,
                                onSelect: () => openEditSpace(space),
                              },
                              {
                                id: "delete",
                                label: "Delete",
                                icon: Trash2Icon,
                                destructive: true,
                                disabled: !canMutate,
                                onSelect: () => {
                                  setActionError("")
                                  setDeleteSpaceTarget(space)
                                },
                              },
                            ]}
                          />
                        </td>
                      </tr>
                    )
                  })}
                  {spacesQuery.isPending ? (
                    <MistyisletEmptyTableRow colSpan={7}>Loading...</MistyisletEmptyTableRow>
                  ) : null}
                  {!spacesQuery.isPending && spaces.length === 0 ? (
                    <MistyisletEmptyTableRow colSpan={7}>
                      No bookable spaces yet. Create one to get started.
                    </MistyisletEmptyTableRow>
                  ) : null}
                </tbody>
              </table>
            </div>
          </div>

          {/* Space status panel */}
          {selectedSpaceID && spaceStatus ? (
            <div className="mt-4 rounded-[6px] border border-line-subtle bg-white p-5">
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-sm font-semibold text-content-heading">
                  Status: {spaceStatus.space.name}
                </h3>
                <span className="text-sm text-content-subtle">
                  Current Occupancy: <span className="font-semibold text-content-heading">{spaceStatus.current_occupancy}</span>
                </span>
              </div>
              {spaceStatus.active_bookings.length > 0 ? (
                <div className="overflow-x-auto rounded-[6px] border border-line-subtle">
                  <table className="w-full text-left text-sm">
                    <thead>
                      <tr className="border-b border-line-subtle bg-surface-page text-xs font-semibold text-content-subtle">
                        <th className="px-4 py-2">Title</th>
                        <th className="px-4 py-2">User</th>
                        <th className="px-4 py-2">Start</th>
                        <th className="px-4 py-2">End</th>
                        <th className="px-4 py-2">Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {spaceStatus.active_bookings.map((b) => (
                        <tr key={b.id} className="border-b border-line-subtle last:border-0">
                          <td className="px-4 py-2 text-content-heading">{b.title ?? "-"}</td>
                          <td className="px-4 py-2 text-content-body">{b.user_name}</td>
                          <td className="px-4 py-2 text-content-subtle">{formatDateTime(b.start_time)}</td>
                          <td className="px-4 py-2 text-content-subtle">{formatDateTime(b.end_time)}</td>
                          <td className="px-4 py-2">
                            <StatusDot tone={bookingStatusTone(b.status)} label={bookingStatusLabel(b.status)} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p className="text-sm text-content-muted">No active bookings for this space.</p>
              )}
            </div>
          ) : null}
          {selectedSpaceID && spaceStatusQuery.isPending ? (
            <div className="mt-4 rounded-[6px] border border-line-subtle bg-white p-5 text-sm text-content-subtle">
              Loading space status...
            </div>
          ) : null}
        </section>

        {/* Section B: Bookings */}
        <section>
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-content-heading">Bookings</h2>
            <Button
              disabled={!canMutate || spaces.length === 0}
              onClick={openCreateBooking}
              className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
            >
              <PlusIcon className="mr-1.5 size-4" />
              New Booking
            </Button>
          </div>

          <div className="overflow-hidden rounded-[6px] border border-line-default bg-white">
            <div className="flex items-center gap-3 border-b border-line-subtle bg-surface-page px-6 py-4">
              <label className="flex items-center gap-2 text-sm text-content-subtle">
                <span className="font-semibold">Space:</span>
                <select
                  value={filterSpaceID}
                  onChange={(e) => setFilterSpaceID(e.target.value)}
                  className="h-10 rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
                >
                  <option value="">All spaces</option>
                  {spaces.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </select>
              </label>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full min-w-[800px] text-left text-sm">
                <thead>
                  <tr className="border-b border-line-subtle bg-surface-page text-left text-xs font-semibold text-content-subtle">
                    <th className="px-4 py-3">Title</th>
                    <th className="px-4 py-3">Space</th>
                    <th className="px-4 py-3">User</th>
                    <th className="px-4 py-3">Start Time</th>
                    <th className="px-4 py-3">End Time</th>
                    <th className="px-4 py-3">Status</th>
                    <th className="w-12 px-4 py-3" />
                  </tr>
                </thead>
                <tbody>
                  {bookings.map((booking) => (
                    <tr key={booking.id} className="border-b border-line-subtle last:border-0 hover:bg-surface-page">
                      <td className="px-4 py-4 font-semibold text-content-heading">{booking.title ?? "-"}</td>
                      <td className="px-4 py-4 text-content-body">{spaceNameByID(booking.space_id)}</td>
                      <td className="px-4 py-4 text-content-body">{booking.user_name}</td>
                      <td className="px-4 py-4 text-content-subtle">{formatDateTime(booking.start_time)}</td>
                      <td className="px-4 py-4 text-content-subtle">{formatDateTime(booking.end_time)}</td>
                      <td className="px-4 py-4">
                        <StatusDot tone={bookingStatusTone(booking.status)} label={bookingStatusLabel(booking.status)} />
                      </td>
                      <td className="px-4 py-4">
                        <RowActionsMenu
                          label={`Actions for booking ${booking.title ?? booking.id}`}
                          items={[
                            {
                              id: "check-in",
                              label: "Check In",
                              icon: LogInIcon,
                              hidden: booking.status !== "confirmed",
                              disabled: checkInMutation.isPending,
                              onSelect: () => checkInMutation.mutate(booking.id),
                            },
                            {
                              id: "check-out",
                              label: "Check Out",
                              icon: LogOutIcon,
                              hidden: booking.status !== "checked_in",
                              disabled: checkOutMutation.isPending,
                              onSelect: () => checkOutMutation.mutate(booking.id),
                            },
                            {
                              id: "edit",
                              label: "Edit",
                              icon: PencilIcon,
                              hidden: booking.status === "completed" || booking.status === "cancelled" || booking.status === "no_show",
                              disabled: !canMutate,
                              onSelect: () => openEditBooking(booking),
                            },
                            {
                              id: "cancel",
                              label: "Cancel",
                              icon: XCircleIcon,
                              destructive: true,
                              hidden: booking.status !== "confirmed" && booking.status !== "checked_in",
                              disabled: cancelBookingMutation.isPending,
                              onSelect: () => cancelBookingMutation.mutate(booking),
                            },
                            {
                              id: "delete",
                              label: "Delete",
                              icon: Trash2Icon,
                              destructive: true,
                              disabled: !canMutate,
                              onSelect: () => {
                                setActionError("")
                                setDeleteBookingTarget(booking)
                              },
                            },
                          ]}
                        />
                      </td>
                    </tr>
                  ))}
                  {bookingsQuery.isPending ? (
                    <MistyisletEmptyTableRow colSpan={7}>Loading...</MistyisletEmptyTableRow>
                  ) : null}
                  {!bookingsQuery.isPending && bookings.length === 0 ? (
                    <MistyisletEmptyTableRow colSpan={7}>
                      No bookings yet. Create one to get started.
                    </MistyisletEmptyTableRow>
                  ) : null}
                </tbody>
              </table>
            </div>
          </div>
        </section>
      </PageFrame>

      {/* Space Sheet */}
      <Sheet
        open={spaceSheetOpen}
        onOpenChange={(open) => {
          if (!open) {
            setSpaceSheetOpen(false)
            setEditSpace(null)
          }
        }}
      >
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[520px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>{editSpace ? "Edit Bookable Space" : "Create Bookable Space"}</SheetTitle>
            <SheetDescription>
              {editSpace
                ? "Update the space configuration."
                : "Set up a new bookable space for your organization."}
            </SheetDescription>
          </SheetHeader>

          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(e) => {
              e.preventDefault()
              if (editSpace) {
                updateSpaceMutation.mutate()
              } else {
                createSpaceMutation.mutate()
              }
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Name *</span>
              <input
                value={spaceForm.name}
                onChange={(e) => setSpaceForm((f) => ({ ...f, name: e.target.value }))}
                required
                placeholder="e.g. Meeting Room A"
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
              />
            </label>

            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Description</span>
              <input
                value={spaceForm.description}
                onChange={(e) => setSpaceForm((f) => ({ ...f, description: e.target.value }))}
                placeholder="Optional description"
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
              />
            </label>

            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Space Type</span>
              <select
                value={spaceForm.space_type}
                onChange={(e) => setSpaceForm((f) => ({ ...f, space_type: e.target.value as BookableSpace["space_type"] }))}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
              >
                {SPACE_TYPE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </label>

            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Capacity Mode</span>
              <select
                value={spaceForm.capacity_mode}
                onChange={(e) => setSpaceForm((f) => ({ ...f, capacity_mode: e.target.value as BookableSpace["capacity_mode"] }))}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
              >
                {CAPACITY_MODE_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
            </label>

            {spaceForm.capacity_mode === "limited_capacity" ? (
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">Max Capacity</span>
                <input
                  type="number"
                  min={1}
                  value={spaceForm.max_capacity}
                  onChange={(e) => setSpaceForm((f) => ({ ...f, max_capacity: Number(e.target.value) }))}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
                />
              </label>
            ) : null}

            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Lock ID</span>
              <input
                value={spaceForm.lock_id}
                onChange={(e) => setSpaceForm((f) => ({ ...f, lock_id: e.target.value }))}
                placeholder="Optional lock identifier"
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
              />
            </label>

            <div className="flex flex-col gap-3">
              <label className="flex items-center gap-2 text-sm text-content-body">
                <input
                  type="checkbox"
                  checked={spaceForm.requires_booking}
                  onChange={(e) => setSpaceForm((f) => ({ ...f, requires_booking: e.target.checked }))}
                  className="size-4 rounded border-line-default accent-brand"
                />
                Requires booking
              </label>
              <label className="flex items-center gap-2 text-sm text-content-body">
                <input
                  type="checkbox"
                  checked={spaceForm.enabled}
                  onChange={(e) => setSpaceForm((f) => ({ ...f, enabled: e.target.checked }))}
                  className="size-4 rounded border-line-default accent-brand"
                />
                Enabled
              </label>
            </div>

            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutate || !spaceForm.name.trim() || createSpaceMutation.isPending || updateSpaceMutation.isPending}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {createSpaceMutation.isPending || updateSpaceMutation.isPending
                  ? "Saving..."
                  : editSpace
                    ? "Save Changes"
                    : "Create Space"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      {/* Booking Sheet */}
      <Sheet
        open={bookingSheetOpen}
        onOpenChange={(open) => {
          if (!open) {
            setBookingSheetOpen(false)
            setEditBooking(null)
          }
        }}
      >
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[520px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>{editBooking ? "Edit Booking" : "New Booking"}</SheetTitle>
            <SheetDescription>
              {editBooking
                ? "Update this booking."
                : "Reserve a space for a specific time slot."}
            </SheetDescription>
          </SheetHeader>

          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(e) => {
              e.preventDefault()
              if (editBooking) {
                updateBookingMutation.mutate()
              } else {
                createBookingMutation.mutate()
              }
            }}
          >
            {!editBooking ? (
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">Space *</span>
                <select
                  value={bookingForm.space_id}
                  onChange={(e) => setBookingForm((f) => ({ ...f, space_id: e.target.value }))}
                  required
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
                >
                  <option value="">Select a space</option>
                  {spaces
                    .filter((s) => s.enabled)
                    .map((s) => (
                      <option key={s.id} value={s.id}>{s.name}</option>
                    ))}
                </select>
              </label>
            ) : null}

            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Title</span>
              <input
                value={bookingForm.title}
                onChange={(e) => setBookingForm((f) => ({ ...f, title: e.target.value }))}
                placeholder="Optional booking title"
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
              />
            </label>

            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Start Time *</span>
              <input
                type="datetime-local"
                value={bookingForm.start_time}
                onChange={(e) => setBookingForm((f) => ({ ...f, start_time: e.target.value }))}
                required
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
              />
            </label>

            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">End Time *</span>
              <input
                type="datetime-local"
                value={bookingForm.end_time}
                onChange={(e) => setBookingForm((f) => ({ ...f, end_time: e.target.value }))}
                required
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring"
              />
            </label>

            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={
                  !canMutate ||
                  (!editBooking && !bookingForm.space_id) ||
                  !bookingForm.start_time ||
                  !bookingForm.end_time ||
                  createBookingMutation.isPending ||
                  updateBookingMutation.isPending
                }
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {createBookingMutation.isPending || updateBookingMutation.isPending
                  ? "Saving..."
                  : editBooking
                    ? "Save Changes"
                    : "Create Booking"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      {/* Confirm delete space */}
      <ConfirmActionDialog
        open={Boolean(deleteSpaceTarget)}
        onOpenChange={(open) => {
          if (!deleteSpaceMutation.isPending && !open) {
            setDeleteSpaceTarget(null)
          }
        }}
        title="Delete Bookable Space"
        description={
          <>
            This will permanently delete{" "}
            <span className="font-semibold text-content-heading">{deleteSpaceTarget?.name ?? "this space"}</span>{" "}
            and all associated bookings.
          </>
        }
        confirmLabel="Delete Space"
        pending={deleteSpaceMutation.isPending}
        disabled={!canMutate || !deleteSpaceTarget}
        destructive
        onConfirm={() => {
          if (deleteSpaceTarget) {
            deleteSpaceMutation.mutate(deleteSpaceTarget)
          }
        }}
      />

      {/* Confirm delete booking */}
      <ConfirmActionDialog
        open={Boolean(deleteBookingTarget)}
        onOpenChange={(open) => {
          if (!deleteBookingMutation.isPending && !open) {
            setDeleteBookingTarget(null)
          }
        }}
        title="Delete Booking"
        description={
          <>
            This will permanently delete the booking{" "}
            <span className="font-semibold text-content-heading">{deleteBookingTarget?.title ?? deleteBookingTarget?.id ?? ""}</span>.
          </>
        }
        confirmLabel="Delete Booking"
        pending={deleteBookingMutation.isPending}
        disabled={!canMutate || !deleteBookingTarget}
        destructive
        onConfirm={() => {
          if (deleteBookingTarget) {
            deleteBookingMutation.mutate(deleteBookingTarget)
          }
        }}
      />
    </>
  )
}

// --- Utility ---

function toDatetimeLocalValue(isoString: string): string {
  const d = new Date(isoString)
  if (Number.isNaN(d.getTime())) return ""
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
