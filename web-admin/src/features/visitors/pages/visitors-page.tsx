import { useState } from "react"
import i18n from "@/lib/i18n"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CheckCircleIcon, ClipboardCopyIcon, LogInIcon, LogOutIcon, PlusIcon, QrCodeIcon, Trash2Icon, XCircleIcon } from "lucide-react"

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

function guestStatusTone(status: string): "success" | "warning" | "info" | "danger" {
  switch (status) {
    case "checked_in": return "success"
    case "expected": return "info"
    case "checked_out": return "warning"
    case "cancelled": return "danger"
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
  const [qrGuest, setQrGuest] = useState<Guest | null>(null)
  const [form, setForm] = useState({
    name: "", email: "", phone: "", company: "", purpose: "",
    host_name: "", host_email: "", host_phone: "", expected_at: "",
    id_document_type: "", id_document_number: "", notify_host: true,
  })

  const guestsQuery = useQuery({
    queryKey: ["guests", tenantID],
    queryFn: () => listGuests(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const [mutationError, setMutationError] = useState("")

  const createMutation = useMutation({
    mutationFn: () => createGuest(token, { tenant_id: tenantID, ...form, id_document_type: (form.id_document_type || undefined) as Guest["id_document_type"] }),
    onSuccess: (guest) => {
      queryClient.invalidateQueries({ queryKey: ["guests"] })
      setShowCreate(false)
      setForm({ name: "", email: "", phone: "", company: "", purpose: "", host_name: "", host_email: "", host_phone: "", expected_at: "", id_document_type: "", id_document_number: "", notify_host: true })
      setMutationError("")
      if (guest.access_token) {
        setQrGuest(guest)
      }
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
              ["phone", "Phone *", "tel"],
              ["email", "Email", "email"],
              ["company", "Company", "text"],
              ["purpose", "Purpose of Visit", "text"],
              ["host_name", "Host Name *", "text"],
              ["host_email", "Host Email", "email"],
              ["host_phone", "Host Phone (WhatsApp)", "tel"],
              ["expected_at", "Expected At", "datetime-local"],
              ["id_document_number", "ID Document Number", "text"],
            ].map(([key, label, type]) => (
              <label key={key} className="block">
                <span className="mb-1 block text-xs font-semibold text-[#6f717c]">{label}</span>
                <input
                  type={type}
                  value={form[key as keyof typeof form] as string}
                  onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
                  className="h-10 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] outline-none focus:border-[#8589ff] focus:ring-2 focus:ring-[#8589ff]/20"
                />
              </label>
            ))}
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-[#6f717c]">ID Document Type</span>
              <select
                value={form.id_document_type}
                onChange={(e) => setForm((f) => ({ ...f, id_document_type: e.target.value }))}
                className="h-10 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                <option value="">None</option>
                <option value="KTP">KTP (ID Card)</option>
                <option value="KITAS">KITAS (Temporary Stay Permit)</option>
                <option value="ITAS">ITAS (Temporary Residence Permit)</option>
              </select>
            </label>
          </div>
          <div className="mt-4 flex items-center gap-4">
            <label className="flex items-center gap-2 text-sm text-[#2f3037]">
              <input
                type="checkbox"
                checked={form.notify_host}
                onChange={(e) => setForm((f) => ({ ...f, notify_host: e.target.checked }))}
                className="size-4 rounded border-[#d9dbe3] accent-[#4f55ff]"
              />
              Notify host via WhatsApp
            </label>
          </div>
          <div className="mt-4 flex gap-2">
            <Button
              className="h-10 rounded-[6px] bg-[#4f55ff] px-6 text-white hover:bg-[#3439cc]"
              disabled={createMutation.isPending || !form.name.trim() || !form.phone.trim() || !form.host_name.trim()}
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
              <GuestRow key={g.id} guest={g} onStatus={statusMutation.mutate} onDelete={setConfirmDelete} onShowQR={setQrGuest} />
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
              <GuestRow key={g.id} guest={g} onStatus={statusMutation.mutate} onDelete={setConfirmDelete} onShowQR={setQrGuest} />
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
              <GuestRow key={g.id} guest={g} onStatus={statusMutation.mutate} onDelete={setConfirmDelete} onShowQR={setQrGuest} />
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

      {/* QR Access Token Dialog */}
      {qrGuest && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={() => setQrGuest(null)}>
          <div className="w-full max-w-sm rounded-lg bg-white p-6 shadow-xl" onClick={(e) => e.stopPropagation()}>
            <div className="mb-4 flex items-center gap-2">
              <QrCodeIcon className="size-5 text-[#4f55ff]" />
              <h3 className="text-lg font-semibold text-[#17171c]">Visitor QR Access</h3>
            </div>
            <p className="mb-2 text-sm text-[#6f717c]">
              Share this token with <strong>{qrGuest.name}</strong> for door access.
            </p>
            <div className="mb-4 flex items-center gap-2 rounded-[6px] bg-[#f3f4f8] p-3">
              <code className="flex-1 break-all text-sm font-mono text-[#17171c]">{qrGuest.access_token}</code>
              <button
                type="button"
                title="Copy token"
                className="shrink-0 rounded p-1 text-[#6f717c] hover:bg-[#eceef2]"
                onClick={() => navigator.clipboard.writeText(qrGuest.access_token ?? "")}
              >
                <ClipboardCopyIcon className="size-4" />
              </button>
            </div>
            {qrGuest.access_token_expires_at && (
              <p className="mb-4 text-xs text-[#9a9ca7]">
                Expires: {new Date(qrGuest.access_token_expires_at).toLocaleString(i18n.language)}
              </p>
            )}
            {qrGuest.door_ids && qrGuest.door_ids.length > 0 && (
              <p className="mb-4 text-xs text-[#9a9ca7]">
                Access: {qrGuest.door_ids.length} door(s)
              </p>
            )}
            <div className="flex justify-end">
              <Button className="h-9 rounded-[6px] bg-[#4f55ff] px-4 text-white hover:bg-[#3439cc]" onClick={() => setQrGuest(null)}>
                Done
              </Button>
            </div>
          </div>
        </div>
      )}
    </PageFrame>
  )
}

function GuestRow({
  guest,
  onStatus,
  onDelete,
  onShowQR,
}: {
  guest: Guest
  onStatus: (args: { guestID: string; status: string }) => void
  onDelete: (id: string) => void
  onShowQR: (guest: Guest) => void
}) {
  return (
    <div className="flex items-center gap-4 border-b border-[#eceef2] px-5 py-4 last:border-b-0">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-[#17171c]">{guest.name}</span>
          {guest.company && <span className="text-sm text-[#6f717c]">({guest.company})</span>}
          {guest.id_document_type && (
            <span className="rounded bg-[#f3f4f8] px-1.5 py-0.5 text-xs font-medium text-[#6f717c]">
              {guest.id_document_type}
            </span>
          )}
        </div>
        <p className="mt-0.5 text-sm text-[#6f717c]">
          Host: {guest.host_name}
          {guest.phone ? ` · ${guest.phone}` : ""}
          {guest.purpose ? ` · ${guest.purpose}` : ""}
        </p>
        {guest.expected_at && (
          <p className="mt-0.5 text-xs text-[#9a9ca7]">
            Expected: {new Date(guest.expected_at).toLocaleString(i18n.language)}
          </p>
        )}
        {guest.checked_in_at && (
          <p className="mt-0.5 text-xs text-[#9a9ca7]">
            Checked in: {new Date(guest.checked_in_at).toLocaleString(i18n.language)}
          </p>
        )}
        {guest.host_notified_at && (
          <p className="mt-0.5 text-xs text-green-600">
            Host notified via WhatsApp
          </p>
        )}
        {guest.access_token && guest.access_token_expires_at && new Date(guest.access_token_expires_at) > new Date() && (
          <p className="mt-0.5 flex items-center gap-1 text-xs text-[#4f55ff]">
            <QrCodeIcon className="size-3" />
            QR active until {new Date(guest.access_token_expires_at).toLocaleString(i18n.language)}
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
        {guest.access_token && guest.access_token_expires_at && new Date(guest.access_token_expires_at) > new Date() && (
          <button
            type="button"
            title="Show QR token"
            className="flex size-8 items-center justify-center rounded-[6px] text-[#4f55ff] hover:bg-[#f0f0ff]"
            onClick={() => onShowQR(guest)}
          >
            <QrCodeIcon className="size-4" />
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
