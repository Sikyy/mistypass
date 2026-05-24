import { useMemo, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { KeyRoundIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import { PageFrame, StatusDot, ToggleSwitch } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Textarea } from "@/components/ui/textarea"
import {
  createOAuth2Client,
  deleteOAuth2Client,
  listOAuth2Clients,
  updateOAuth2Client,
  type CurrentUser,
  type OAuth2Client,
} from "@/lib/api"

type APIClientsPageProps = {
  token: string
  viewer: CurrentUser
}

type ClientDraft = {
  name: string
  redirectURIs: string
  scopes: string[]
  enabled: boolean
}

const scopeOptions = ["read", "write", "admin"]

const emptyDraft: ClientDraft = {
  name: "",
  redirectURIs: "",
  scopes: ["read"],
  enabled: true,
}

function parseRedirectURIs(input: string) {
  return input
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
}

export function APIClientsPage({ token, viewer }: APIClientsPageProps) {
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<OAuth2Client | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<OAuth2Client | null>(null)
  const [draft, setDraft] = useState<ClientDraft>(emptyDraft)
  const [createdSecret, setCreatedSecret] = useState<{ clientID: string; secret: string } | null>(null)
  const [error, setError] = useState("")

  const clientsQuery = useQuery({
    queryKey: ["oauth2-clients", viewer.tenant_id],
    queryFn: () => listOAuth2Clients(token),
    enabled: Boolean(token && viewer.tenant_id),
  })

  const clients = clientsQuery.data ?? []
  const redirectURIs = useMemo(() => parseRedirectURIs(draft.redirectURIs), [draft.redirectURIs])
  const isEditing = Boolean(editTarget)

  function openCreate() {
    setEditTarget(null)
    setDraft(emptyDraft)
    setCreatedSecret(null)
    setError("")
    setSheetOpen(true)
  }

  function openEdit(client: OAuth2Client) {
    setEditTarget(client)
    setDraft({
      name: client.name,
      redirectURIs: client.redirect_uris.join("\n"),
      scopes: client.scopes.length > 0 ? client.scopes : ["read"],
      enabled: client.enabled,
    })
    setCreatedSecret(null)
    setError("")
    setSheetOpen(true)
  }

  function updateDraft(patch: Partial<ClientDraft>) {
    setDraft((current) => ({ ...current, ...patch }))
  }

  function toggleScope(scope: string) {
    setDraft((current) => {
      const exists = current.scopes.includes(scope)
      const next = exists ? current.scopes.filter((item) => item !== scope) : [...current.scopes, scope]
      return { ...current, scopes: next.length > 0 ? next : ["read"] }
    })
  }

  const createMutation = useMutation({
    mutationFn: () =>
      createOAuth2Client(token, {
        name: draft.name.trim(),
        redirect_uris: redirectURIs,
        scopes: draft.scopes,
      }),
    onSuccess: (client) => {
      queryClient.invalidateQueries({ queryKey: ["oauth2-clients"] })
      setCreatedSecret(client.client_secret ? { clientID: client.id, secret: client.client_secret } : null)
      setDraft(emptyDraft)
      setError("")
    },
    onError: (err) => setError(err instanceof Error ? err.message : "Failed to create API client"),
  })

  const updateMutation = useMutation({
    mutationFn: () =>
      updateOAuth2Client(token, editTarget!.id, {
        name: draft.name.trim(),
        redirect_uris: redirectURIs,
        scopes: draft.scopes,
        enabled: draft.enabled,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["oauth2-clients"] })
      setSheetOpen(false)
      setEditTarget(null)
      setDraft(emptyDraft)
      setError("")
    },
    onError: (err) => setError(err instanceof Error ? err.message : "Failed to update API client"),
  })

  const deleteMutation = useMutation({
    mutationFn: (client: OAuth2Client) => deleteOAuth2Client(token, client.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["oauth2-clients"] })
      setDeleteTarget(null)
      setError("")
    },
    onError: (err) => setError(err instanceof Error ? err.message : "Failed to delete API client"),
  })

  const saving = createMutation.isPending || updateMutation.isPending
  const canSubmit = draft.name.trim() !== "" && redirectURIs.length > 0

  return (
    <PageFrame
      breadcrumbs={["Home", "Organization", "API Clients"]}
      title="API Clients"
      description="Manage OAuth2 client applications."
      actions={
        <Button type="button" className="h-11 rounded-[6px] bg-brand px-6 text-white hover:bg-brand-hover" onClick={openCreate}>
          <PlusIcon className="mr-2 size-4" />
          New Client
        </Button>
      }
    >
      {error ? (
        <div className="rounded-[6px] border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</div>
      ) : null}
      {clientsQuery.error instanceof Error ? (
        <div className="rounded-[6px] border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          {clientsQuery.error.message}
        </div>
      ) : null}
      {createdSecret ? (
        <div className="rounded-[6px] border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-800">
          <span className="font-semibold">Client secret for {createdSecret.clientID}: </span>
          <code className="break-all rounded bg-white/70 px-1.5 py-0.5">{createdSecret.secret}</code>
        </div>
      ) : null}

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-content-subtle">Clients</CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-bold text-content-heading">{clientsQuery.isLoading ? "--" : clients.length}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-content-subtle">Enabled</CardTitle>
          </CardHeader>
          <CardContent className="text-3xl font-bold text-content-heading">{clients.filter((item) => item.enabled).length}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-content-subtle">Tenant</CardTitle>
          </CardHeader>
          <CardContent className="truncate text-sm font-semibold text-content-heading">{viewer.tenant_id}</CardContent>
        </Card>
      </div>

      <section className="overflow-hidden rounded-[6px] border border-line-subtle bg-white">
        <div className="hidden border-b border-line-subtle px-5 py-3 md:grid md:grid-cols-[1.4fr_1.6fr_1fr_1fr_auto]">
          {["Name", "Redirect URIs", "Scopes", "Status", ""].map((header) => (
            <span key={header} className="text-xs font-semibold text-content-subtle">{header}</span>
          ))}
        </div>
        {clients.length === 0 ? (
          <div className="px-5 py-8 text-center text-sm text-content-subtle">
            {clientsQuery.isLoading ? "Loading API clients..." : "No API clients configured."}
          </div>
        ) : (
          clients.map((client) => (
            <div key={client.id} className="grid gap-3 border-b border-line-subtle px-5 py-4 last:border-b-0 md:grid-cols-[1.4fr_1.6fr_1fr_1fr_auto] md:items-center">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <KeyRoundIcon className="size-4 shrink-0 text-content-subtle" />
                  <span className="truncate font-semibold text-content-heading">{client.name}</span>
                </div>
                <p className="mt-1 truncate text-xs text-content-muted">{client.id}</p>
              </div>
              <div className="min-w-0 text-sm text-content-subtle">
                {client.redirect_uris.map((uri) => (
                  <p key={uri} className="truncate">{uri}</p>
                ))}
              </div>
              <span className="text-sm text-content-subtle">{client.scopes.join(", ") || "read"}</span>
              <StatusDot tone={client.enabled ? "success" : "warning"} label={client.enabled ? "Enabled" : "Disabled"} />
              <RowActionsMenu
                label={`Actions for ${client.name}`}
                items={[
                  { id: "edit", label: "Edit", icon: PencilIcon, onSelect: () => openEdit(client) },
                  {
                    id: "delete",
                    label: "Delete",
                    icon: Trash2Icon,
                    destructive: true,
                    onSelect: () => setDeleteTarget(client),
                  },
                ]}
              />
            </div>
          ))
        )}
      </section>

      <Sheet
        open={sheetOpen}
        onOpenChange={(open) => {
          if (!saving) {
            setSheetOpen(open)
            if (!open) {
              setEditTarget(null)
              setError("")
            }
          }
        }}
      >
        <SheetContent className="w-full max-w-lg overflow-y-auto bg-white">
          <SheetHeader>
            <SheetTitle>{isEditing ? "Edit API Client" : "New API Client"}</SheetTitle>
            <SheetDescription>{isEditing ? editTarget?.id : "OAuth2 authorization code client"}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 p-6"
            onSubmit={(event) => {
              event.preventDefault()
              if (isEditing) updateMutation.mutate()
              else createMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">Name</span>
              <Input value={draft.name} onChange={(event) => updateDraft({ name: event.target.value })} required />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-semibold text-content-subtle">Redirect URIs</span>
              <Textarea
                value={draft.redirectURIs}
                onChange={(event) => updateDraft({ redirectURIs: event.target.value })}
                placeholder="https://example.com/oauth/callback"
                rows={4}
                required
              />
            </label>
            <div>
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Scopes</span>
              <div className="inline-flex overflow-hidden rounded-[6px] border border-line-default">
                {scopeOptions.map((scope) => {
                  const active = draft.scopes.includes(scope)
                  return (
                    <button
                      key={scope}
                      type="button"
                      className={`h-10 px-4 text-sm font-medium ${active ? "bg-brand text-white" : "bg-white text-content-subtle hover:bg-brand-subtle"}`}
                      onClick={() => toggleScope(scope)}
                    >
                      {scope}
                    </button>
                  )
                })}
              </div>
            </div>
            {isEditing ? (
              <div className="flex items-center justify-between rounded-[6px] border border-line-subtle px-4 py-3">
                <span className="text-sm font-medium text-content-heading">Enabled</span>
                <ToggleSwitch enabled={draft.enabled} label="Enabled" onToggle={() => updateDraft({ enabled: !draft.enabled })} />
              </div>
            ) : null}
            <SheetFooter>
              <Button type="submit" disabled={saving || !canSubmit}>
                {saving ? "Saving..." : isEditing ? "Save Changes" : "Create Client"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <ConfirmActionDialog
        open={Boolean(deleteTarget)}
        title="Delete API client"
        description={`Delete ${deleteTarget?.name ?? "this client"} and revoke outstanding authorization codes.`}
        confirmLabel="Delete"
        destructive
        pending={deleteMutation.isPending}
        onOpenChange={(open) => {
          if (!open && !deleteMutation.isPending) setDeleteTarget(null)
        }}
        onConfirm={() => {
          if (deleteTarget) deleteMutation.mutate(deleteTarget)
        }}
      />
    </PageFrame>
  )
}
