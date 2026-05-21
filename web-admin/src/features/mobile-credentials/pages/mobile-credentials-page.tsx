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
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

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
          <h1 className="text-2xl font-semibold text-content-heading">Mobile Credentials</h1>
          <p className="mt-1 text-sm text-content-subtle">
            BLE mobile credentials registered by users (Android Keystore / iOS Secure Enclave)
          </p>
        </div>
        <div className="flex gap-2">
          <select
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value as typeof filterStatus)}
            className="rounded-[6px] border border-line-default bg-white px-3 py-2 text-sm text-content-body"
          >
            <option value="">All Status</option>
            <option value="active">Active</option>
            <option value="revoked">Revoked</option>
          </select>
          <Button
            variant="outline"
            size="icon"
            onClick={() => queryClient.invalidateQueries({ queryKey: ["mobile-credentials"] })}
          >
            <RefreshCwIcon className="size-4" />
          </Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4">
        <StatCard label="Total" value={items.length} />
        <StatCard label="Active" value={items.filter((c) => c.status === "active").length} color="green" />
        <StatCard label="Revoked" value={items.filter((c) => c.status === "revoked").length} color="red" />
      </div>

      {/* Credentials table */}
      <div className="rounded-[6px] border border-line-subtle bg-white">
        <div className="grid grid-cols-[1fr_1fr_100px_120px_100px_80px] gap-4 border-b border-line-subtle px-4 py-3 text-xs font-medium uppercase text-content-subtle">
          <div>User</div>
          <div>Device</div>
          <div>Platform</div>
          <div>Security</div>
          <div>Status</div>
          <div>Actions</div>
        </div>

        {credentialsQuery.isLoading && (
          <div className="p-8 text-center text-content-subtle">Loading...</div>
        )}

        {filtered.length === 0 && !credentialsQuery.isLoading && (
          <div className="p-8 text-center text-content-subtle">
            No mobile credentials registered yet
          </div>
        )}

        {filtered.map((cred) => (
          <div
            key={cred.id}
            className="grid cursor-pointer grid-cols-[1fr_1fr_100px_120px_100px_80px] items-center gap-4 border-b border-line-subtle px-4 py-3 last:border-b-0 hover:bg-surface-page"
            onClick={() => setDetailCred(cred)}
          >
            <div>
              <div className="text-sm font-medium text-content-heading">{cred.user_email}</div>
              <div className="text-xs text-content-subtle">{cred.user_id}</div>
            </div>
            <div>
              <div className="text-sm text-content-heading">{cred.device_model}</div>
              <div className="text-xs text-content-subtle">{cred.device_id.slice(0, 20)}...</div>
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
          <div className="w-full max-w-md rounded-[6px] bg-white p-6 shadow-xl" onClick={(e) => e.stopPropagation()}>
            <h3 className="mb-4 text-lg font-semibold text-content-heading">Credential Details</h3>
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
                  <dt className="text-content-subtle">{label}</dt>
                  <dd className="font-medium text-content-heading">{value}</dd>
                </div>
              ))}
            </dl>
            <div className="mt-4 flex justify-end gap-2">
              {detailCred.status === "active" && (
                <Button
                  variant="destructive"
                  onClick={() => { setDetailCred(null); setConfirmRevoke(detailCred.id) }}
                >
                  Revoke
                </Button>
              )}
              <Button variant="outline" onClick={() => setDetailCred(null)}>
                Close
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Revoke confirmation dialog */}
      {confirmRevoke && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="w-[400px] rounded-[6px] bg-white p-6 shadow-lg">
            <h3 className="text-lg font-semibold text-content-heading">Revoke Credential</h3>
            <p className="mt-2 text-sm text-content-subtle">
              This will immediately revoke the mobile credential. The user will no longer be able to
              unlock doors via BLE until they re-register.
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <Button variant="outline" onClick={() => setConfirmRevoke(null)}>
                Cancel
              </Button>
              <Button
                variant="destructive"
                onClick={() => revokeMutation.mutate(confirmRevoke)}
                disabled={revokeMutation.isPending}
              >
                {revokeMutation.isPending ? "Revoking..." : "Revoke"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function StatCard({ label, value, color }: { label: string; value: number; color?: string }) {
  return (
    <div className="rounded-[6px] border border-line-subtle bg-white p-4">
      <div className="text-xs font-medium uppercase text-content-subtle">{label}</div>
      <div className={cn(
        "mt-1 text-2xl font-semibold",
        color === "green" ? "text-success-text" : color === "red" ? "text-danger-text" : "text-content-heading"
      )}>{value}</div>
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
