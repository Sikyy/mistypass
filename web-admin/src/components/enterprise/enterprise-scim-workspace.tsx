import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  CheckCircleIcon,
  ClipboardCopyIcon,
  KeyIcon,
  LinkIcon,
  RefreshCwIcon,
  ShieldCheckIcon,
  Trash2Icon,
  XCircleIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { TabsContent } from "@/components/ui/tabs"
import {
  generateSCIMToken,
  getSCIMConfig,
  listSCIMProvisioningLogs,
  revokeSCIMToken,
  testSCIMEndpoint,
  type SCIMTokenResponse,
} from "@/lib/api"

type EnterpriseSCIMWorkspaceProps = {
  token: string
}

export function EnterpriseSCIMWorkspace({ token }: EnterpriseSCIMWorkspaceProps) {
  const queryClient = useQueryClient()
  const [newToken, setNewToken] = useState<SCIMTokenResponse | null>(null)
  const [copied, setCopied] = useState<string | null>(null)

  const configQuery = useQuery({
    queryKey: ["scim-config"],
    queryFn: () => getSCIMConfig(token),
    enabled: Boolean(token),
  })

  const logsQuery = useQuery({
    queryKey: ["scim-logs"],
    queryFn: () => listSCIMProvisioningLogs(token),
    enabled: Boolean(token),
  })

  const generateMutation = useMutation({
    mutationFn: (label: string) => generateSCIMToken(token, label),
    onSuccess: (data) => {
      setNewToken(data)
      queryClient.invalidateQueries({ queryKey: ["scim-config"] })
    },
  })

  const revokeMutation = useMutation({
    mutationFn: () => revokeSCIMToken(token),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["scim-config"] })
      setNewToken(null)
    },
  })

  const testMutation = useMutation({
    mutationFn: () => testSCIMEndpoint(token),
  })

  const config = configQuery.data

  function copyToClipboard(text: string, label: string) {
    navigator.clipboard.writeText(text)
    setCopied(label)
    setTimeout(() => setCopied(null), 2000)
  }

  return (
    <TabsContent value="scim">
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-lg font-semibold text-[#17171c]">SCIM 2.0 Provisioning</h2>
        <p className="mt-1 text-sm text-[#6f717c]">
          Configure SCIM to automatically sync users from your Identity Provider (Okta, Entra ID, OneLogin).
          When employees join or leave in your IdP, their access is automatically updated.
        </p>
      </div>

      {/* Connection Info */}
      {config && (
        <div className="rounded-lg border border-[#eceef2] bg-white p-5">
          <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-[#17171c]">
            <LinkIcon className="size-4" />
            Connection Details
          </h3>
          <div className="space-y-3">
            <div className="flex items-center justify-between rounded-md bg-[#f7f7f8] px-4 py-3">
              <div>
                <p className="text-xs font-medium text-[#6f717c]">SCIM Base URL</p>
                <p className="mt-0.5 font-mono text-sm text-[#17171c]">{config.scim_base_url}</p>
              </div>
              <button
                type="button"
                className="rounded p-1.5 text-[#6f717c] hover:bg-[#eceef2]"
                onClick={() => copyToClipboard(config.scim_base_url, "url")}
              >
                {copied === "url" ? <CheckCircleIcon className="size-4 text-green-600" /> : <ClipboardCopyIcon className="size-4" />}
              </button>
            </div>

            <div className="flex items-center justify-between rounded-md bg-[#f7f7f8] px-4 py-3">
              <div>
                <p className="text-xs font-medium text-[#6f717c]">Authentication</p>
                <p className="mt-0.5 text-sm text-[#17171c]">Bearer Token (HTTP Header)</p>
              </div>
              <span className="rounded bg-[#e0e1ff] px-2 py-0.5 text-xs font-medium text-[#4f55ff]">
                SCIM {config.scim_version}
              </span>
            </div>

            <div className="flex items-center justify-between rounded-md bg-[#f7f7f8] px-4 py-3">
              <div>
                <p className="text-xs font-medium text-[#6f717c]">Supported Operations</p>
                <div className="mt-1 flex flex-wrap gap-1.5">
                  {config.supported_operations.map((op) => (
                    <span key={op} className="rounded bg-white px-2 py-0.5 text-xs text-[#6f717c] border border-[#eceef2]">
                      {op}
                    </span>
                  ))}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Token Management */}
      <div className="rounded-lg border border-[#eceef2] bg-white p-5">
        <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-[#17171c]">
          <KeyIcon className="size-4" />
          Bearer Token
        </h3>

        {config?.token_configured ? (
          <div className="space-y-3">
            <div className="flex items-center justify-between rounded-md bg-green-50 px-4 py-3 border border-green-200">
              <div>
                <div className="flex items-center gap-2">
                  <CheckCircleIcon className="size-4 text-green-600" />
                  <span className="text-sm font-medium text-green-800">Token configured</span>
                </div>
                <p className="mt-1 text-xs text-green-700">
                  {config.token_label && `Label: ${config.token_label} · `}
                  Token: {config.token_masked}
                  {config.token_created_at && ` · Created: ${new Date(config.token_created_at).toLocaleDateString()}`}
                  {config.token_created_by && ` by ${config.token_created_by}`}
                </p>
              </div>
              <Button
                variant="outline"
                className="h-8 gap-1.5 border-red-200 text-red-600 hover:bg-red-50"
                onClick={() => revokeMutation.mutate()}
                disabled={revokeMutation.isPending}
              >
                <Trash2Icon className="size-3.5" />
                Revoke
              </Button>
            </div>
          </div>
        ) : (
          <div className="rounded-md bg-amber-50 border border-amber-200 px-4 py-3">
            <div className="flex items-center gap-2">
              <XCircleIcon className="size-4 text-amber-600" />
              <span className="text-sm font-medium text-amber-800">No token configured</span>
            </div>
            <p className="mt-1 text-xs text-amber-700">
              Generate a Bearer Token to enable SCIM provisioning from your IdP.
            </p>
          </div>
        )}

        {/* New token display (shown only once after generation) */}
        {newToken && (
          <div className="mt-3 rounded-md bg-blue-50 border border-blue-200 px-4 py-3">
            <p className="text-sm font-medium text-blue-800">New token generated — copy it now, it won't be shown again.</p>
            <div className="mt-2 flex items-center gap-2">
              <code className="flex-1 rounded bg-white px-3 py-2 font-mono text-xs text-[#17171c] border border-blue-200 break-all">
                {newToken.token}
              </code>
              <button
                type="button"
                className="shrink-0 rounded p-1.5 text-blue-600 hover:bg-blue-100"
                onClick={() => copyToClipboard(newToken.token, "token")}
              >
                {copied === "token" ? <CheckCircleIcon className="size-4 text-green-600" /> : <ClipboardCopyIcon className="size-4" />}
              </button>
            </div>
          </div>
        )}

        <div className="mt-3 flex gap-2">
          <Button
            className="h-9 gap-1.5 rounded-[6px] bg-[#4f55ff] px-4 text-white hover:bg-[#3439cc]"
            onClick={() => generateMutation.mutate("okta")}
            disabled={generateMutation.isPending}
          >
            <KeyIcon className="size-3.5" />
            {config?.token_configured ? "Regenerate Token" : "Generate Token"}
          </Button>
          <Button
            variant="outline"
            className="h-9 gap-1.5"
            onClick={() => testMutation.mutate()}
            disabled={testMutation.isPending || !config?.token_configured}
          >
            <ShieldCheckIcon className="size-3.5" />
            Test Connection
          </Button>
        </div>

        {testMutation.data && (
          <div className={`mt-3 rounded-md px-4 py-2 text-sm ${testMutation.data.status === "connected" ? "bg-green-50 text-green-800 border border-green-200" : "bg-red-50 text-red-800 border border-red-200"}`}>
            {testMutation.data.status === "connected" ? (
              <span className="flex items-center gap-2">
                <CheckCircleIcon className="size-4" />
                Connected — {testMutation.data.user_count} users synced
              </span>
            ) : (
              <span className="flex items-center gap-2">
                <XCircleIcon className="size-4" />
                {testMutation.data.message}
              </span>
            )}
          </div>
        )}
      </div>

      {/* Setup Guide */}
      {config && (
        <div className="rounded-lg border border-[#eceef2] bg-white p-5">
          <h3 className="mb-4 text-sm font-semibold text-[#17171c]">Setup Guide</h3>
          <ol className="space-y-3">
            {config.setup_steps.map((step) => (
              <li key={step.step} className="flex gap-3">
                <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-[#f3f4f8] text-xs font-bold text-[#6f717c]">
                  {step.step}
                </span>
                <div>
                  <p className="text-sm font-medium text-[#17171c]">{step.title}</p>
                  <p className="text-xs text-[#6f717c]">{step.description}</p>
                </div>
              </li>
            ))}
          </ol>
        </div>
      )}

      {/* Provisioning Logs */}
      <div className="rounded-lg border border-[#eceef2] bg-white p-5">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-[#17171c]">Provisioning Activity</h3>
          <button
            type="button"
            className="rounded p-1 text-[#6f717c] hover:bg-[#f3f4f8]"
            onClick={() => queryClient.invalidateQueries({ queryKey: ["scim-logs"] })}
          >
            <RefreshCwIcon className="size-4" />
          </button>
        </div>
        {logsQuery.data?.items.length === 0 ? (
          <p className="text-sm text-[#9a9ca7]">No provisioning activity yet. Events will appear here once your IdP starts syncing users.</p>
        ) : (
          <div className="max-h-64 overflow-y-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[#eceef2] text-left text-xs text-[#6f717c]">
                  <th className="pb-2 font-medium">Action</th>
                  <th className="pb-2 font-medium">Target</th>
                  <th className="pb-2 font-medium">Time</th>
                </tr>
              </thead>
              <tbody>
                {logsQuery.data?.items.map((log) => (
                  <tr key={log.id} className="border-b border-[#f3f4f8] last:border-0">
                    <td className="py-2">
                      <span className={`inline-flex items-center rounded px-1.5 py-0.5 text-xs font-medium ${log.action.includes("created") ? "bg-green-50 text-green-700" : log.action.includes("revoked") || log.action.includes("deleted") ? "bg-red-50 text-red-700" : "bg-blue-50 text-blue-700"}`}>
                        {log.action.replace(/_/g, " ")}
                      </span>
                    </td>
                    <td className="py-2 text-[#6f717c]">{log.target}</td>
                    <td className="py-2 text-xs text-[#9a9ca7]">{new Date(log.timestamp).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
    </TabsContent>
  )
}
