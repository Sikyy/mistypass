import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { LogInIcon, LogOutIcon, PlusIcon, Trash2Icon, XCircleIcon } from "lucide-react"

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

type VisitorsPageProps = {
  token: string
  viewer: CurrentUser
}

export function VisitorsPage({ token, viewer }: VisitorsPageProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const tenantID = viewer.tenant_id
  const [showCreate, setShowCreate] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
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
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["guests"] })
      setShowCreate(false)
      setForm({ name: "", email: "", phone: "", company: "", purpose: "", host_name: "", host_email: "", host_phone: "", expected_at: "", id_document_type: "", id_document_number: "", notify_host: true })
      setMutationError("")
    },
    onError: (error) => setMutationError(error instanceof Error ? error.message : t("visitors.error.createFailed")),
  })

  const statusMutation = useMutation({
    mutationFn: ({ guestID, status }: { guestID: string; status: string }) =>
      updateGuestStatus(token, guestID, tenantID, status),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["guests"] }); setMutationError("") },
    onError: (error) => setMutationError(error instanceof Error ? error.message : t("visitors.error.updateFailed")),
  })

  const deleteMutation = useMutation({
    mutationFn: (guestID: string) => deleteGuest(token, guestID, tenantID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["guests"] })
      setConfirmDelete(null)
      setMutationError("")
    },
    onError: (error) => setMutationError(error instanceof Error ? error.message : t("visitors.error.deleteFailed")),
  })

  const guests = guestsQuery.data?.items ?? []
  const present = guests.filter((g) => g.status === "checked_in")
  const upcoming = guests.filter((g) => g.status === "expected")
  const past = guests.filter((g) => g.status === "checked_out" || g.status === "cancelled")

  const guestStatusLabel = (status: string): string => {
    switch (status) {
      case "expected": return t("visitors.expected")
      case "checked_in": return t("visitors.present")
      case "checked_out": return t("visitors.past")
      case "cancelled": return t("visitors.cancel")
      default: return status
    }
  }

  return (
    <PageFrame
      breadcrumbs={["Dashboard", t("visitors.navLabel")]}
      title={t("visitors.title")}
      description={t("visitors.description")}
      actions={
        <Button
          className="h-11 rounded-[6px] bg-brand px-6 text-white hover:bg-brand-hover"
          onClick={() => setShowCreate(true)}
        >
          <PlusIcon className="mr-2 size-4" />
          {t("visitors.registerGuest")}
        </Button>
      }
    >
      {mutationError && (
        <div className="mb-4 rounded-[6px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          {mutationError}
        </div>
      )}

      {showCreate && (
        <div className="mb-6 rounded-[6px] border border-line-subtle bg-white p-6">
          <h3 className="mb-4 text-lg font-semibold text-content-heading">{t("visitors.registerTitle")}</h3>
          <div className="grid gap-4 md:grid-cols-2">
            {([
              ["name", t("visitors.form.guestName"), "text"],
              ["phone", t("visitors.form.phone"), "tel"],
              ["email", t("visitors.form.email"), "email"],
              ["company", t("visitors.form.company"), "text"],
              ["purpose", t("visitors.form.purpose"), "text"],
              ["host_name", t("visitors.form.hostName"), "text"],
              ["host_email", t("visitors.form.hostEmail"), "email"],
              ["host_phone", t("visitors.form.hostPhone"), "tel"],
              ["expected_at", t("visitors.form.expectedAt"), "datetime-local"],
              ["id_document_number", t("visitors.form.idDocNumber"), "text"],
            ] as const).map(([key, label, type]) => (
              <label key={key} className="block">
                <span className="mb-1 block text-xs font-semibold text-content-subtle">{label}</span>
                <input
                  type={type}
                  value={form[key as keyof typeof form] as string}
                  onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))}
                  className="h-10 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body outline-none focus:border-brand-ring focus:ring-2 focus:ring-brand-ring/20"
                />
              </label>
            ))}
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">{t("visitors.form.idDocType")}</span>
              <select
                value={form.id_document_type}
                onChange={(e) => setForm((f) => ({ ...f, id_document_type: e.target.value }))}
                className="h-10 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                <option value="">{t("visitors.form.idDocNone")}</option>
                <option value="KTP">{t("visitors.form.idDocKTP")}</option>
                <option value="KITAS">{t("visitors.form.idDocKITAS")}</option>
                <option value="ITAS">{t("visitors.form.idDocITAS")}</option>
              </select>
            </label>
          </div>
          <div className="mt-4 flex items-center gap-4">
            <label className="flex items-center gap-2 text-sm text-content-body">
              <input
                type="checkbox"
                checked={form.notify_host}
                onChange={(e) => setForm((f) => ({ ...f, notify_host: e.target.checked }))}
                className="size-4 rounded border-line-default accent-brand"
              />
              {t("visitors.notifyHost")}
            </label>
          </div>
          <div className="mt-4 flex gap-2">
            <Button
              className="h-10 rounded-[6px] bg-brand px-6 text-white hover:bg-brand-hover"
              disabled={createMutation.isPending || !form.name.trim() || !form.phone.trim() || !form.host_name.trim()}
              onClick={() => createMutation.mutate()}
            >
              {createMutation.isPending ? t("visitors.creating") : t("visitors.create")}
            </Button>
            <Button variant="outline" className="h-10 rounded-[6px]" onClick={() => setShowCreate(false)}>
              {t("visitors.cancelAction")}
            </Button>
          </div>
        </div>
      )}

      {/* Present Visitors */}
      <div className="mb-6">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-content-subtle">
          {t("visitors.present")} ({present.length})
        </h2>
        {present.length === 0 ? (
          <p className="text-sm text-content-muted">{t("visitors.noPresent")}</p>
        ) : (
          <div className="overflow-hidden rounded-[6px] border border-line-subtle bg-white">
            {present.map((g) => (
              <GuestRow key={g.id} guest={g} statusLabel={guestStatusLabel} onStatus={statusMutation.mutate} onDelete={setConfirmDelete} />
            ))}
          </div>
        )}
      </div>

      {/* Upcoming */}
      <div className="mb-6">
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-content-subtle">
          {t("visitors.expected")} ({upcoming.length})
        </h2>
        {upcoming.length === 0 ? (
          <p className="text-sm text-content-muted">{t("visitors.noExpected")}</p>
        ) : (
          <div className="overflow-hidden rounded-[6px] border border-line-subtle bg-white">
            {upcoming.map((g) => (
              <GuestRow key={g.id} guest={g} statusLabel={guestStatusLabel} onStatus={statusMutation.mutate} onDelete={setConfirmDelete} />
            ))}
          </div>
        )}
      </div>

      {/* Past */}
      {past.length > 0 && (
        <div className="mb-6">
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-wider text-content-subtle">
            {t("visitors.past")} ({past.length})
          </h2>
          <div className="overflow-hidden rounded-[6px] border border-line-subtle bg-white">
            {past.map((g) => (
              <GuestRow key={g.id} guest={g} statusLabel={guestStatusLabel} onStatus={statusMutation.mutate} onDelete={setConfirmDelete} />
            ))}
          </div>
        </div>
      )}

      <ConfirmActionDialog
        open={confirmDelete !== null}
        title={t("visitors.deleteTitle")}
        description={t("visitors.deleteDesc")}
        confirmLabel={t("visitors.deleteConfirm")}
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
  statusLabel,
  onStatus,
  onDelete,
}: {
  guest: Guest
  statusLabel: (status: string) => string
  onStatus: (args: { guestID: string; status: string }) => void
  onDelete: (id: string) => void
}) {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-4 border-b border-line-subtle px-5 py-4 last:border-b-0">
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-content-heading">{guest.name}</span>
          {guest.company && <span className="text-sm text-content-subtle">({guest.company})</span>}
          {guest.id_document_type && (
            <span className="rounded bg-[#f3f4f8] px-1.5 py-0.5 text-xs font-medium text-content-subtle">
              {guest.id_document_type}
            </span>
          )}
        </div>
        <p className="mt-0.5 text-sm text-content-subtle">
          {t("visitors.host")}: {guest.host_name}
          {guest.phone ? ` · ${guest.phone}` : ""}
          {guest.purpose ? ` · ${guest.purpose}` : ""}
        </p>
        {guest.expected_at && (
          <p className="mt-0.5 text-xs text-content-muted">
            {t("visitors.expectedAt")}: {new Date(guest.expected_at).toLocaleString()}
          </p>
        )}
        {guest.checked_in_at && (
          <p className="mt-0.5 text-xs text-content-muted">
            {t("visitors.checkedInAt")}: {new Date(guest.checked_in_at).toLocaleString()}
          </p>
        )}
        {guest.host_notified_at && (
          <p className="mt-0.5 text-xs text-green-600">
            {t("visitors.hostNotified")}
          </p>
        )}
      </div>
      <StatusDot tone={guestStatusTone(guest.status)} label={statusLabel(guest.status)} />
      <div className="flex gap-1">
        {guest.status === "expected" && (
          <>
            <button
              type="button"
              title={t("visitors.checkIn")}
              className="flex size-8 items-center justify-center rounded-[6px] text-green-600 hover:bg-green-50"
              onClick={() => onStatus({ guestID: guest.id, status: "checked_in" })}
            >
              <LogInIcon className="size-4" />
            </button>
            <button
              type="button"
              title={t("visitors.cancel")}
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
            title={t("visitors.checkOut")}
            className="flex size-8 items-center justify-center rounded-[6px] text-amber-600 hover:bg-amber-50"
            onClick={() => onStatus({ guestID: guest.id, status: "checked_out" })}
          >
            <LogOutIcon className="size-4" />
          </button>
        )}
        <button
          type="button"
          title={t("visitors.deleteConfirm")}
          className="flex size-8 items-center justify-center rounded-[6px] text-content-subtle hover:bg-surface-page"
          onClick={() => onDelete(guest.id)}
        >
          <Trash2Icon className="size-4" />
        </button>
      </div>
    </div>
  )
}
