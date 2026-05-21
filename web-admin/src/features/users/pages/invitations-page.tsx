import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Link } from "react-router"
import { MailXIcon, RefreshCwIcon } from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import { MistyisletEmptyTableRow, MistyisletSearchField } from "@/components/mistyislet/data-display"
import { PageFrame, StatusDot } from "@/components/mistyislet/primitives"
import { cn } from "@/lib/utils"
import {
  cancelInvitation,
  listInvitations,
  resendInvitation,
  type CurrentUser,
  type UserInvitationDelivery,
} from "@/lib/api"
import { getViewerTenantID } from "@/lib/viewer"

type ActivityTone = "success" | "warning" | "danger" | "info"

function invitationStatusTone(status: string): ActivityTone {
  switch (status) {
    case "sent":
      return "success"
    case "queued":
      return "info"
    case "failed":
      return "danger"
    case "cancelled":
      return "warning"
    default:
      return "info"
  }
}

export function InvitationsAdaptedPage({
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
  const [statusFilter, setStatusFilter] = useState("")
  const [page, setPage] = useState(0)
  const [confirmCancel, setConfirmCancel] = useState<UserInvitationDelivery | null>(null)
  const [confirmResend, setConfirmResend] = useState<UserInvitationDelivery | null>(null)
  const [actionNotice, setActionNotice] = useState("")
  const pageSize = 20

  const invitationsQuery = useQuery({
    queryKey: ["invitations", tenantID, statusFilter],
    queryFn: () => listInvitations(token, { tenant_id: tenantID, status: statusFilter || undefined }),
    enabled: !!token,
  })

  const cancelMutation = useMutation({
    mutationFn: (deliveryID: string) => cancelInvitation(token, deliveryID, tenantID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["invitations"] })
      setActionNotice(t("kisi.invitations.cancelled"))
      setConfirmCancel(null)
    },
  })

  const resendMutation = useMutation({
    mutationFn: (deliveryID: string) => resendInvitation(token, deliveryID, tenantID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["invitations"] })
      setActionNotice(t("kisi.invitations.resent"))
      setConfirmResend(null)
    },
  })

  const allItems = invitationsQuery.data ?? []
  const filtered = search
    ? allItems.filter(
        (d) =>
          d.email.toLowerCase().includes(search.toLowerCase()) ||
          d.user_id.toLowerCase().includes(search.toLowerCase())
      )
    : allItems
  const totalPages = Math.ceil(filtered.length / pageSize)
  const pagedItems = filtered.slice(page * pageSize, (page + 1) * pageSize)

  const statusFilters = ["", "queued", "sent", "failed", "cancelled"]

  return (
    <PageFrame
      breadcrumbs={["Home", t("kisi.invitations.title")]}
      title={t("kisi.invitations.title")}
      count={filtered.length}
      description={t("kisi.invitations.description")}
    >
      {actionNotice && (
        <div className={cn("mp-alert-success", "mx-0 mb-4 py-3")}>
          {actionNotice}
        </div>
      )}

      <section className="rounded-[6px] border border-line-default bg-white">
        <div className="flex items-center gap-3 border-b border-line-subtle px-6 py-4">
          <MistyisletSearchField value={search} onChange={setSearch} placeholder="Search by email or user ID" />
          <div className="ml-auto flex gap-2">
            {statusFilters.map((s) => (
              <button
                key={s || "all"}
                onClick={() => { setStatusFilter(s); setPage(0) }}
                className={`rounded-[6px] px-3 py-1.5 text-xs font-medium transition-colors ${
                  statusFilter === s
                    ? "bg-brand text-white"
                    : "bg-surface-sunken text-content-subtle hover:bg-line-subtle"
                }`}
              >
                {s === "" ? t("common.all") : t(`kisi.invitations.${s === "cancelled" ? "cancelled_status" : s}`)}
              </button>
            ))}
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-line-subtle bg-surface-page text-left text-xs font-semibold text-content-subtle">
                <th className="px-6 py-3">{t("common.email")}</th>
                <th className="px-6 py-3">{t("common.user")}</th>
                <th className="px-6 py-3">{t("common.status")}</th>
                <th className="px-6 py-3">{t("common.method")}</th>
                <th className="px-6 py-3">{t("common.date")}</th>
                <th className="w-12 px-6 py-3" />
              </tr>
            </thead>
            <tbody>
              {pagedItems.length === 0 ? (
                <MistyisletEmptyTableRow colSpan={6}>No invitations found.</MistyisletEmptyTableRow>
              ) : (
                pagedItems.map((d) => (
                  <tr key={d.id} className="border-b border-line-subtle hover:bg-surface-page">
                    <td className="px-6 py-3 font-medium text-content-heading">{d.email}</td>
                    <td className="px-6 py-3 text-content-subtle">
                      <Link to={`/users/${d.user_id}`} className="text-brand hover:underline">
                        {d.user_id}
                      </Link>
                    </td>
                    <td className="px-6 py-3">
                      <StatusDot tone={invitationStatusTone(d.status)} label={d.status} />
                    </td>
                    <td className="px-6 py-3 text-content-subtle">{d.delivery_method}</td>
                    <td className="px-6 py-3 text-content-subtle">
                      {d.delivered_at
                        ? new Date(d.delivered_at).toLocaleString()
                        : new Date(d.queued_at).toLocaleString()}
                    </td>
                    <td className="px-6 py-3">
                      <RowActionsMenu
                        items={[
                          ...(d.status === "queued"
                            ? [{ id: "cancel", label: t("kisi.invitations.cancel"), icon: MailXIcon, onSelect: () => setConfirmCancel(d) }]
                            : []),
                          { id: "resend", label: t("kisi.invitations.resend"), icon: RefreshCwIcon, onSelect: () => setConfirmResend(d) },
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

      <ConfirmActionDialog
        open={!!confirmCancel}
        onOpenChange={(open) => { if (!open) setConfirmCancel(null) }}
        title={t("kisi.invitations.cancel")}
        description={t("kisi.invitations.cancelConfirm", { email: confirmCancel?.email })}
        confirmLabel={t("kisi.invitations.cancel")}
        onConfirm={() => confirmCancel && cancelMutation.mutate(confirmCancel.id)}
        pending={cancelMutation.isPending}
      />

      <ConfirmActionDialog
        open={!!confirmResend}
        onOpenChange={(open) => { if (!open) setConfirmResend(null) }}
        title={t("kisi.invitations.resend")}
        description={t("kisi.invitations.resendConfirm", { email: confirmResend?.email })}
        confirmLabel={t("kisi.invitations.resend")}
        onConfirm={() => confirmResend && resendMutation.mutate(confirmResend.id)}
        pending={resendMutation.isPending}
      />
    </PageFrame>
  )
}
