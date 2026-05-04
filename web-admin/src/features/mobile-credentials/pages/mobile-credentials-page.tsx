import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { SmartphoneIcon, ShieldXIcon, RefreshCwIcon } from "lucide-react"
import {
  listMobileCredentials,
  revokeMobileCredential,
  revokeAllUserCredentials,
  type MobileCredential,
  type CurrentUser,
} from "@/lib/api"

type MobileCredentialsPageProps = {
  token: string
  viewer: CurrentUser
}

export function MobileCredentialsPage({ token, viewer }: MobileCredentialsPageProps) {
  const queryClient = useQueryClient()
  const tenantID = viewer.tenant_id
  const [confirmRevoke, setConfirmRevoke] = useState<string | null>(null)
  const [filterStatus, setFilterStatus] = useState<"" | "active" | "revoked">("")
  const [detailCred, setDetailCred] = useState<MobileCredential | null>(null)

  const credentialsQuery = useQuery({
    queryKey: ["mobile-credentials", tenantID],
    queryFn: () => listMobileCredentials(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const revokeMutation = useMutation({
    mutationFn: (credentialID: string) => revokeMobileCredential(token, credentialID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mobile-credentials"] })
      setConfirmRevoke(null)
    },
  })

  const items = credentialsQuery.data?.items ?? []
  const filtered = filterStatus ? items.filter((c) => c.status === filterStatus) : items

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-[#1a1c23]">Mobile Credentials</h1>
          <p className="mt-1 text-sm text-[#6f717c]">
            BLE mobile credentials registered by users (Android Keystore / iOS Secure Enclave)
          </p>
        </div>
        <div className="flex gap-2">
          <select
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value as typeof filterStatus)}
            className="rounded-md border border-[#d9dbe3] px-3 py-2 text-sm"
          >
            <option value="">All Status</option>
            <option value="active">Active</option>
            <option value="revoked">Revoked</option>
          </select>
          <button
            onClick={() => queryClient.invalidateQueries({ queryKey: ["mobile-credentials"] })}
            className="rounded-md border border-[#d9dbe3] p-2 hover:bg-[#f5f6f8]"
          >
            <RefreshCwIcon className="size-4" />
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4">
        <StatCard label="Total" value={items.length} />
        <StatCard label="Active" value={items.filter((c) => c.status === "active").length} color="green" />
        <StatCard label="Revoked" value={items.filter((c) => c.status === "revoked").length} color="red" />
      </div>

      {/* Credentials table */}
      <div className="rounded-lg border border-[#eceef2] bg-white">
        <div className="grid grid-cols-[1fr_1fr_100px_120px_100px_80px] gap-4 border-b border-[#eceef2] px-4 py-3 text-xs font-medium uppercase text-[#6f717c]">
          <div>User</div>
          <div>Device</div>
          <div>Platform</div>
          <div>Security</div>
          <div>Status</div>
          <div>Actions</div>
        </div>

        {credentialsQuery.isLoading && (
          <div className="p-8 text-center text-[#6f717c]">Loading...</div>
        )}

        {filtered.length === 0 && !credentialsQuery.isLoading && (
          <div className="p-8 text-center text-[#6f717c]">
            No mobile credentials registered yet
          </div>
        )}

        {filtered.map((cred) => (
          <div
            key={cred.id}
            className="grid cursor-pointer grid-cols-[1fr_1fr_100px_120px_100px_80px] items-center gap-4 border-b border-[#eceef2] px-4 py-3 last:border-b-0 hover:bg-[#fafbfc]"
            onClick={() => setDetailCred(cred)}
          >
            <div>
              <div className="text-sm font-medium text-[#1a1c23]">{cred.user_email}</div>
              <div className="text-xs text-[#6f717c]">{cred.user_id}</div>
            </div>
            <div>
              <div className="text-sm text-[#1a1c23]">{cred.device_model}</div>
              <div className="text-xs text-[#6f717c]">{cred.device_id.slice(0, 20)}...</div>
            </div>
            <div>
              <span className="inline-flex items-center gap-1 text-sm">
                <SmartphoneIcon className="size-3" />
                {cred.platform}
              </span>
            </div>
            <div>
              <KeystoreBadge level={cred.keystore_level} />
            </div>
            <div>
              <StatusBadge status={cred.status} />
            </div>
            <div>
              {cred.status === "active" && (
                <button
                  onClick={(e) => { e.stopPropagation(); setConfirmRevoke(cred.id) }}
                  className="rounded p-1 text-red-600 hover:bg-red-50"
                  title="Revoke credential"
                >
                  <ShieldXIcon className="size-4" />
                </button>
              )}
            </div>
          </div>
        ))}
      </div>

      {/* Credential detail dialog */}
      {detailCred && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40" onClick={() => setDetailCred(null)}>
          <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl" onClick={(e) => e.stopPropagation()}>
            <h3 className="mb-4 text-lg font-semibold text-[#17171c]">Credential Details</h3>
            <dl className="space-y-2 text-sm">
              {[
                ["ID", detailCred.id],
                ["User", detailCred.user_email],
                ["Device", detailCred.device_model],
                ["Device ID", detailCred.device_id],
                ["Platform", detailCred.platform],
                ["Keystore", detailCred.keystore_level],
                ["Status", detailCred.status],
                ["Issued", new Date(detailCred.issued_at).toLocaleString()],
                ["Expires", new Date(detailCred.expires_at).toLocaleString()],
                ["Last Used", detailCred.last_used_at ? new Date(detailCred.last_used_at).toLocaleString() : "Never"],
                ...(detailCred.revoked_at ? [["Revoked", new Date(detailCred.revoked_at).toLocaleString()]] : []),
              ].map(([label, value]) => (
                <div key={label as string} className="flex justify-between">
                  <dt className="text-[#6f717c]">{label}</dt>
                  <dd className="font-medium text-[#17171c]">{value}</dd>
                </div>
              ))}
            </dl>
            <div className="mt-4 flex justify-end gap-2">
              {detailCred.status === "active" && (
                <button
                  className="rounded-md bg-red-600 px-4 py-2 text-sm text-white hover:bg-red-700"
                  onClick={() => { setDetailCred(null); setConfirmRevoke(detailCred.id) }}
                >
                  Revoke
                </button>
              )}
              <button
                className="rounded-md border border-[#d9dbe3] px-4 py-2 text-sm"
                onClick={() => setDetailCred(null)}
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Revoke confirmation dialog */}
      {confirmRevoke && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-[400px] rounded-lg bg-white p-6 shadow-lg">
            <h3 className="text-lg font-semibold">Revoke Credential</h3>
            <p className="mt-2 text-sm text-[#6f717c]">
              This will immediately revoke the mobile credential. The user will no longer be able to
              unlock doors via BLE until they re-register.
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <button
                onClick={() => setConfirmRevoke(null)}
                className="rounded-md border border-[#d9dbe3] px-4 py-2 text-sm"
              >
                Cancel
              </button>
              <button
                onClick={() => revokeMutation.mutate(confirmRevoke)}
                disabled={revokeMutation.isPending}
                className="rounded-md bg-red-600 px-4 py-2 text-sm text-white hover:bg-red-700 disabled:opacity-50"
              >
                {revokeMutation.isPending ? "Revoking..." : "Revoke"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value, color }: { label: string; value: number; color?: string }) {
  const colorClass =
    color === "green" ? "text-green-600" : color === "red" ? "text-red-600" : "text-[#1a1c23]"
  return (
    <div className="rounded-lg border border-[#eceef2] bg-white p-4">
      <div className="text-xs font-medium uppercase text-[#6f717c]">{label}</div>
      <div className={`mt-1 text-2xl font-semibold ${colorClass}`}>{value}</div>
    </div>
  )
}

function KeystoreBadge({ level }: { level: string }) {
  const colors: Record<string, string> = {
    strongbox: "bg-green-100 text-green-800",
    tee: "bg-blue-100 text-blue-800",
    software: "bg-yellow-100 text-yellow-800",
  }
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${colors[level] ?? "bg-gray-100 text-gray-800"}`}>
      {level}
    </span>
  )
}

function StatusBadge({ status }: { status: string }) {
  const colors: Record<string, string> = {
    active: "bg-green-100 text-green-800",
    revoked: "bg-red-100 text-red-800",
    expired: "bg-gray-100 text-gray-800",
  }
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${colors[status] ?? "bg-gray-100 text-gray-800"}`}>
      {status}
    </span>
  )
}
