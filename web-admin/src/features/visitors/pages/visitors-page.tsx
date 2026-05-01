import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CheckCircleIcon, LogInIcon, LogOutIcon, PlusIcon, Trash2Icon, XCircleIcon } from "lucide-react"

import { ConfirmActionDialog } from "@/components/mistyislet/actions"
import { PageFrame, StatusDot } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  createGuest,
  deleteGuest,
  listGuests,
  updateGuestStatus,
  type CurrentUser,
  type Guest,
} from "@/lib/api"

function guestStatusTone(status: string): "success" | "warning" | "info" | "error" {
  switch (status) {
    case "checked_in": return "success"
    case "expected": return "info"
    case "checked_out": return "warning"
    case "cancelled": return "error"
    default: return "info"
  }
}

function guestStatusLabel(status: string): string {
  switch (status) {
    case "expected": return "Expected"
    case "checked_in": return "Checked In"
    case "checked_out": return "Checked Out"
    case "cancelled": return "Cancelled"
    default: return status
  }
}

type VisitorsPageProps = {
  token: string
  viewer: CurrentUser
}

export function VisitorsPage({ token, viewer }: VisitorsPageProps) {
  const queryClient = useQueryClient()
  const tenantID = viewer.tenant_id
  const [showCreate, setShowCreate] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [form, setForm] = useState({
    name: "", email: "", phone: "", company: "", purpose: "", host_name: "", host_email: "", expected_at: "",
  })

  const guestsQuery = useQuery({
    queryKey: ["guests", tenantID],
    queryFn: () => listGuests(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const [mutationError, setMutationError] = useState("")

  const createMutation = useMutation({
    mutationFn: () => createGuest(token, { tenant_id: tenantID, ...form }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["guests"] })
      setShowCreate(false)
      setForm({ name: "", email: "", phone: "", company: "", purpose: "", host_name: "", host_email: "", expected_at: "" })
      setMutationError("")
    },
    onError: (error) => setMutationError(error instanceof Error ? error.message : "Failed to create guest"),
  })

  const statusMutation = useMutation({
    mutationFn: ({ guestID, status }: { guestID: string; status: string }) =>
      updateGuestStatus(token, guestID, tenantID, status),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["guests"] }); setMutationError("") },
    onError: (error) => setMutationError(error instanceof Error ? error.message : "Failed to update status"),
  })

  const deleteMutation = useMutation({
    mutationFn: (guestID: string) => deleteGuest(token, guestID, tenantID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["guests"] })
      setConfirmDelete(null)
      setMutationError("")
    },
    onError: (error) => setMutationError(error instanceof Error ? error.message : "Failed to delete guest"),
  })

  const guests = guestsQuery.data?.items ?? []
  const present = guests.filter((g) => g.status === "checked_in")
  const upcoming = guests.filter((g) => g.status === "expected")
  const past = guests.filter((g) => g.status === "checked_out" || g.status === "cancelled")

  return (
    <PageFrame
      breadcrumbs={["Dashboard", "Visitors"]}
      title="Visitor Management"
      description="Track and manage building visitors."
      actions={
        <Button
          className="h-11 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#3439cc]"
          onClick={() => setShowCreate(true)}
        >
          <PlusIcon className="mr-2 size-4" />
          Register Guest
        </Button>
      }
    >
      {showCreate && (
        <div className="mb-6 rounded-[6px] border border-[#eceef2] bg-white p-6">
          <h3 className="mb-4 text-lg font-semibold text-[#17171c]">Register New Guest</h3>
          <div className="grid gap-4 md:grid-cols-2">
            {[
              ["name", "Guest Name *", "text"],
              ["email", "Email", "email"],
              ["phone", "Phone", "tel"],
              ["company", "Company", "text"],
              ["purpose", "Purpose of Visit", "text"],
              ["host_name", "Host Name *", "text"],
              ["host_email", "Host Email", "email"],
              ["expected_at", "Expected At", "datetime-local"],
            ].map(([key, label, type]) => (
              <label key={key} className="block">
                <span className="mb-1 block text-xs font-semibold text-[#6f717c]">{label}</span>
                <input
                  type={type}
                  value={form[key as keyof typeof form]}
                  onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
                  className="h-10 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
                />
              </label>
            ))}
          </div>
          <div className="mt-4 flex gap-2">
            <Button
              className="h-10 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#3439cc]"
              disabled={createMutation.isPending || !form.name.trim() || !form.host_name.trim()}
              onClick={() => createMutation.mutate()}
            >
              {createMutation.isPending ? "Creating..." : "Create"}
            </Button>
            <Button variant="outline" className="h-10 rounded-[6px]" onClick={() => setShowCreate(false)}>
              Cancel
            </Button>
          </div>
        </div>
      )}

      {/* Present Visitors */}
      <div className="mb-6">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-[#6f717c]">
          Present ({present.length})
        </h2>
        {present.length === 0 ? (
          <p className="text-sm text-[#9a9ca7]">No visitors currently checked in.</p>
        ) : (
          <div className="overflow-hidden rounded-[6px] border border-[#eceef2] bg-white">
            {present.map((g) => (
              <GuestRow key={g.id} guest={g} onStatus={statusMutation.mutate} onDelete={setConfirmDelete} />
            ))}
          </div>
        )}
      </div>

      {/* Upcoming */}
      <div className="mb-6">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-[#6f717c]">
          Expected ({upcoming.length})
        </h2>
        {upcoming.length === 0 ? (
          <p className="text-sm text-[#9a9ca7]">No upcoming visitors.</p>
        ) : (
          <div className="overflow-hidden rounded-[6px] border border-[#eceef2] bg-white">
            {upcoming.map((g) => (
              <GuestRow key={g.id} guest={g} onStatus={statusMutation.mutate} onDelete={setConfirmDelete} />
            ))}
          </div>
        )}
      </div>

      {/* Past */}
      {past.length > 0 && (
        <div className="mb-6">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-[#6f717c]">
            Past ({past.length})
          </h2>
          <div className="overflow-hidden rounded-[6px] border border-[#eceef2] bg-white">
            {past.map((g) => (
              <GuestRow key={g.id} guest={g} onStatus={statusMutation.mutate} onDelete={setConfirmDelete} />
            ))}
          </div>
        </div>
      )}

      <ConfirmActionDialog
        open={confirmDelete !== null}
        title="Delete guest record"
        description="This guest record will be permanently removed."
        confirmLabel="Delete"
        onConfirm={() => confirmDelete && deleteMutation.mutate(confirmDelete)}
        onOpenChange={(open) => { if (!open) setConfirmDelete(null) }}
        pending={deleteMutation.isPending}
        destructive
      />
    </PageFrame>
  )
}

function GuestRow({
  guest,
  onStatus,
  onDelete,
}: {
  guest: Guest
  onStatus: (args: { guestID: string; status: string }) => void
  onDelete: (id: string) => void
}) {
  return (
    <div className="flex items-center gap-4 border-b border-[#eceef2] px-5 py-4 last:border-b-0">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-[#17171c]">{guest.name}</span>
          {guest.company && <span className="text-sm text-[#6f717c]">({guest.company})</span>}
        </div>
        <p className="mt-0.5 text-sm text-[#6f717c]">
          Host: {guest.host_name}
          {guest.purpose ? ` · ${guest.purpose}` : ""}
        </p>
        {guest.expected_at && (
          <p className="mt-0.5 text-xs text-[#9a9ca7]">
            Expected: {new Date(guest.expected_at).toLocaleString()}
          </p>
        )}
        {guest.checked_in_at && (
          <p className="mt-0.5 text-xs text-[#9a9ca7]">
            Checked in: {new Date(guest.checked_in_at).toLocaleString()}
          </p>
        )}
      </div>
      <StatusDot tone={guestStatusTone(guest.status)} label={guestStatusLabel(guest.status)} />
      <div className="flex gap-1">
        {guest.status === "expected" && (
          <>
            <button
              type="button"
              title="Check in"
              className="flex size-8 items-center justify-center rounded-[6px] text-green-600 hover:bg-green-50"
              onClick={() => onStatus({ guestID: guest.id, status: "checked_in" })}
            >
              <LogInIcon className="size-4" />
            </button>
            <button
              type="button"
              title="Cancel"
              className="flex size-8 items-center justify-center rounded-[6px] text-red-500 hover:bg-red-50"
              onClick={() => onStatus({ guestID: guest.id, status: "cancelled" })}
            >
              <XCircleIcon className="size-4" />
            </button>
          </>
        )}
        {guest.status === "checked_in" && (
          <button
            type="button"
            title="Check out"
            className="flex size-8 items-center justify-center rounded-[6px] text-amber-600 hover:bg-amber-50"
            onClick={() => onStatus({ guestID: guest.id, status: "checked_out" })}
          >
            <LogOutIcon className="size-4" />
          </button>
        )}
        <button
          type="button"
          title="Delete"
          className="flex size-8 items-center justify-center rounded-[6px] text-[#6f717c] hover:bg-[#fbfbfc]"
          onClick={() => onDelete(guest.id)}
        >
          <Trash2Icon className="size-4" />
        </button>
      </div>
    </div>
  )
}
