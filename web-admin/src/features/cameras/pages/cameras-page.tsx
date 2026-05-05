import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  CameraIcon,
  CheckCircle2Icon,
  ImageIcon,
  Loader2Icon,
  PlusIcon,
  RefreshCwIcon,
  Trash2Icon,
  WifiIcon,
  XCircleIcon,
} from "lucide-react"

import { ConfirmActionDialog } from "@/components/mistyislet/actions"
import { PageFrame, StatusDot } from "@/components/mistyislet/primitives"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  captureSnapshot,
  createCamera,
  deleteCamera,
  listCameras,
  listCameraSnapshots,
  testCameraConnection,
  updateCamera,
  type Camera,
  type CameraCreateRequest,
  type CameraProvider,
  type CameraSnapshot,
  type CurrentUser,
} from "@/lib/api"

// --- Constants ---

const CAMERA_PROVIDERS: { value: CameraProvider; label: string }[] = [
  { value: "hikvision", label: "Hikvision" },
  { value: "dahua", label: "Dahua" },
  { value: "onvif", label: "ONVIF" },
  { value: "zkteco", label: "ZKTeco" },
  { value: "vivotek", label: "VIVOTEK" },
]

function providerLabel(provider: string): string {
  return CAMERA_PROVIDERS.find((p) => p.value === provider)?.label ?? provider
}

function providerBadgeClass(provider: string): string {
  switch (provider) {
    case "hikvision": return "bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300"
    case "dahua": return "bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-300"
    case "onvif": return "bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-300"
    case "zkteco": return "bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300"
    case "vivotek": return "bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-300"
    default: return "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300"
  }
}

// --- Page Props ---

type CamerasPageProps = {
  token: string
  viewer: CurrentUser
}

export function CamerasPage({ token, viewer }: CamerasPageProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const tenantID = viewer.tenant_id

  const [showCreate, setShowCreate] = useState(false)
  const [editCamera, setEditCamera] = useState<Camera | null>(null)
  const [snapshotCamera, setSnapshotCamera] = useState<Camera | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [actionFeedback, setActionFeedback] = useState<{ cameraID: string; type: "success" | "error"; message: string } | null>(null)

  // --- Queries ---

  const camerasQuery = useQuery({
    queryKey: ["cameras", tenantID],
    queryFn: () => listCameras(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const cameras = camerasQuery.data ?? []
  const activeCount = cameras.filter((c) => c.status === "active").length

  // --- Create form ---

  const emptyForm: CameraCreateRequest = {
    tenant_id: tenantID,
    place_id: "",
    name: "",
    provider: "hikvision",
    host: "",
    port: 80,
    channel: 1,
    use_tls: false,
    username: "admin",
    password: "",
  }

  const [form, setForm] = useState(emptyForm)
  const [editForm, setEditForm] = useState<{ name: string; host: string; port: number; channel: number; username: string; password: string; status: string }>({
    name: "", host: "", port: 80, channel: 1, username: "", password: "", status: "active",
  })
  const [mutationError, setMutationError] = useState("")

  // --- Mutations ---

  const createMutation = useMutation({
    mutationFn: () => createCamera(token, { ...form, tenant_id: tenantID }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cameras"] })
      setShowCreate(false)
      setForm(emptyForm)
      setMutationError("")
    },
    onError: (error) => setMutationError(error instanceof Error ? error.message : t("cameras.error.createFailed")),
  })

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editCamera) throw new Error("No camera selected")
      return updateCamera(token, editCamera.id, {
        name: editForm.name || undefined,
        host: editForm.host || undefined,
        port: editForm.port || undefined,
        channel: editForm.channel || undefined,
        username: editForm.username || undefined,
        password: editForm.password || undefined,
        status: editForm.status || undefined,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cameras"] })
      setEditCamera(null)
      setMutationError("")
    },
    onError: (error) => setMutationError(error instanceof Error ? error.message : t("cameras.error.updateFailed")),
  })

  const deleteMutation = useMutation({
    mutationFn: (cameraID: string) => deleteCamera(token, cameraID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cameras"] })
      setConfirmDelete(null)
    },
    onError: (error) => setMutationError(error instanceof Error ? error.message : t("cameras.error.deleteFailed")),
  })

  const testMutation = useMutation({
    mutationFn: (cameraID: string) => testCameraConnection(token, cameraID),
    onSuccess: (_data, cameraID) => {
      setActionFeedback({ cameraID, type: "success", message: t("cameras.testSuccess") })
      setTimeout(() => setActionFeedback(null), 3000)
    },
    onError: (error, cameraID) => {
      setActionFeedback({ cameraID, type: "error", message: error instanceof Error ? error.message : t("cameras.testFailed") })
      setTimeout(() => setActionFeedback(null), 5000)
    },
  })

  const snapshotMutation = useMutation({
    mutationFn: (cameraID: string) => captureSnapshot(token, cameraID),
    onSuccess: (_data, cameraID) => {
      queryClient.invalidateQueries({ queryKey: ["cameras"] })
      queryClient.invalidateQueries({ queryKey: ["camera-snapshots", cameraID] })
      setActionFeedback({ cameraID, type: "success", message: t("cameras.snapshotCaptured") })
      setTimeout(() => setActionFeedback(null), 3000)
    },
    onError: (error, cameraID) => {
      setActionFeedback({ cameraID, type: "error", message: error instanceof Error ? error.message : t("cameras.snapshotFailed") })
      setTimeout(() => setActionFeedback(null), 5000)
    },
  })

  function openEdit(camera: Camera) {
    setEditForm({
      name: camera.name,
      host: camera.host,
      port: camera.port,
      channel: camera.channel,
      username: "",
      password: "",
      status: camera.status,
    })
    setEditCamera(camera)
    setMutationError("")
  }

  // --- Render ---

  return (
    <PageFrame
      breadcrumbs={[t("cameras.breadcrumbHome"), t("cameras.title")]}
      title={t("cameras.title")}
      description={t("cameras.description")}
      actions={
        <Button size="sm" onClick={() => { setShowCreate(true); setMutationError("") }}>
          <PlusIcon className="mr-1 h-4 w-4" />
          {t("cameras.addCamera")}
        </Button>
      }
    >
      {/* Summary cards */}
      <div className="mb-6 grid gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t("cameras.totalCameras")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{cameras.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t("cameras.activeCameras")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">{activeCount}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">{t("cameras.providers")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{new Set(cameras.map((c) => c.provider)).size}</div>
          </CardContent>
        </Card>
      </div>

      {/* Camera list */}
      {camerasQuery.isLoading ? (
        <div className="flex items-center justify-center py-12 text-muted-foreground">
          <Loader2Icon className="mr-2 h-5 w-5 animate-spin" />
          {t("cameras.loading")}
        </div>
      ) : cameras.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12 text-center">
            <CameraIcon className="mb-3 h-10 w-10 text-muted-foreground/50" />
            <p className="text-sm text-muted-foreground">{t("cameras.empty")}</p>
            <Button size="sm" variant="outline" className="mt-4" onClick={() => setShowCreate(true)}>
              <PlusIcon className="mr-1 h-4 w-4" />
              {t("cameras.addCamera")}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {cameras.map((camera) => (
            <Card key={camera.id} className="relative">
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between">
                  <div className="space-y-1">
                    <CardTitle className="text-base">{camera.name}</CardTitle>
                    <p className="text-xs text-muted-foreground">{camera.host}:{camera.port}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant="secondary" className={providerBadgeClass(camera.provider)}>
                      {providerLabel(camera.provider)}
                    </Badge>
                    <StatusDot tone={camera.status === "active" ? "success" : "warning"} label={camera.status === "active" ? t("cameras.statusActive") : t("cameras.statusInactive")} />
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex items-center gap-4 text-xs text-muted-foreground">
                  <span>{t("cameras.channel")}: {camera.channel}</span>
                  {camera.door_id && <span>{t("cameras.doorLinked")}</span>}
                  {camera.last_snapshot_at && (
                    <span>{t("cameras.lastSnapshot")}: {new Date(camera.last_snapshot_at).toLocaleString()}</span>
                  )}
                </div>

                {/* Feedback for this camera */}
                {actionFeedback?.cameraID === camera.id && (
                  <div className={`flex items-center gap-1 rounded px-2 py-1 text-xs ${
                    actionFeedback.type === "success"
                      ? "bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300"
                      : "bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300"
                  }`}>
                    {actionFeedback.type === "success" ? <CheckCircle2Icon className="h-3 w-3" /> : <XCircleIcon className="h-3 w-3" />}
                    {actionFeedback.message}
                  </div>
                )}

                {/* Actions */}
                <div className="flex flex-wrap gap-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => testMutation.mutate(camera.id)}
                    disabled={testMutation.isPending}
                  >
                    <WifiIcon className="mr-1 h-3 w-3" />
                    {t("cameras.test")}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => snapshotMutation.mutate(camera.id)}
                    disabled={snapshotMutation.isPending}
                  >
                    <ImageIcon className="mr-1 h-3 w-3" />
                    {t("cameras.snapshot")}
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setSnapshotCamera(camera)}
                  >
                    <RefreshCwIcon className="mr-1 h-3 w-3" />
                    {t("cameras.history")}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => openEdit(camera)}>
                    {t("cameras.edit")}
                  </Button>
                  <Button size="sm" variant="ghost" className="text-destructive" onClick={() => setConfirmDelete(camera.id)}>
                    <Trash2Icon className="h-3 w-3" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Create Sheet */}
      <Sheet open={showCreate} onOpenChange={setShowCreate}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{t("cameras.addCamera")}</SheetTitle>
            <SheetDescription>{t("cameras.addCameraDesc")}</SheetDescription>
          </SheetHeader>
          <div className="mt-4 space-y-4">
            <div>
              <Label>{t("cameras.form.name")}</Label>
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder={t("cameras.form.namePlaceholder")} />
            </div>
            <div>
              <Label>{t("cameras.form.provider")}</Label>
              <Select value={form.provider} onValueChange={(v) => setForm({ ...form, provider: v })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {CAMERA_PROVIDERS.map((p) => (
                    <SelectItem key={p.value} value={p.value}>{p.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid grid-cols-3 gap-2">
              <div className="col-span-2">
                <Label>{t("cameras.form.host")}</Label>
                <Input value={form.host} onChange={(e) => setForm({ ...form, host: e.target.value })} placeholder="192.168.1.100" />
              </div>
              <div>
                <Label>{t("cameras.form.port")}</Label>
                <Input type="number" value={form.port} onChange={(e) => setForm({ ...form, port: Number(e.target.value) || 80 })} />
              </div>
            </div>
            <div>
              <Label>{t("cameras.form.channel")}</Label>
              <Input type="number" value={form.channel} onChange={(e) => setForm({ ...form, channel: Number(e.target.value) || 1 })} />
            </div>
            <div>
              <Label>{t("cameras.form.username")}</Label>
              <Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
            </div>
            <div>
              <Label>{t("cameras.form.password")}</Label>
              <Input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            </div>
            <div>
              <Label>{t("cameras.form.doorId")}</Label>
              <Input value={form.door_id ?? ""} onChange={(e) => setForm({ ...form, door_id: e.target.value || undefined })} placeholder={t("cameras.form.doorIdPlaceholder")} />
            </div>
            {mutationError && <p className="text-sm text-destructive">{mutationError}</p>}
            <Button className="w-full" onClick={() => createMutation.mutate()} disabled={createMutation.isPending || !form.name || !form.host}>
              {createMutation.isPending && <Loader2Icon className="mr-1 h-4 w-4 animate-spin" />}
              {t("cameras.form.create")}
            </Button>
          </div>
        </SheetContent>
      </Sheet>

      {/* Edit Sheet */}
      <Sheet open={editCamera !== null} onOpenChange={(open) => { if (!open) setEditCamera(null) }}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{t("cameras.editCamera")}</SheetTitle>
            <SheetDescription>{editCamera?.id}</SheetDescription>
          </SheetHeader>
          <div className="mt-4 space-y-4">
            <div>
              <Label>{t("cameras.form.name")}</Label>
              <Input value={editForm.name} onChange={(e) => setEditForm({ ...editForm, name: e.target.value })} />
            </div>
            <div className="grid grid-cols-3 gap-2">
              <div className="col-span-2">
                <Label>{t("cameras.form.host")}</Label>
                <Input value={editForm.host} onChange={(e) => setEditForm({ ...editForm, host: e.target.value })} />
              </div>
              <div>
                <Label>{t("cameras.form.port")}</Label>
                <Input type="number" value={editForm.port} onChange={(e) => setEditForm({ ...editForm, port: Number(e.target.value) || 80 })} />
              </div>
            </div>
            <div>
              <Label>{t("cameras.form.channel")}</Label>
              <Input type="number" value={editForm.channel} onChange={(e) => setEditForm({ ...editForm, channel: Number(e.target.value) || 1 })} />
            </div>
            <div>
              <Label>{t("cameras.form.username")}</Label>
              <Input value={editForm.username} onChange={(e) => setEditForm({ ...editForm, username: e.target.value })} placeholder={t("cameras.form.leaveBlank")} />
            </div>
            <div>
              <Label>{t("cameras.form.password")}</Label>
              <Input type="password" value={editForm.password} onChange={(e) => setEditForm({ ...editForm, password: e.target.value })} placeholder={t("cameras.form.leaveBlank")} />
            </div>
            <div>
              <Label>{t("cameras.form.status")}</Label>
              <Select value={editForm.status} onValueChange={(v) => setEditForm({ ...editForm, status: v })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="active">{t("cameras.statusActive")}</SelectItem>
                  <SelectItem value="inactive">{t("cameras.statusInactive")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            {mutationError && <p className="text-sm text-destructive">{mutationError}</p>}
            <Button className="w-full" onClick={() => updateMutation.mutate()} disabled={updateMutation.isPending}>
              {updateMutation.isPending && <Loader2Icon className="mr-1 h-4 w-4 animate-spin" />}
              {t("cameras.form.save")}
            </Button>
          </div>
        </SheetContent>
      </Sheet>

      {/* Snapshots Sheet */}
      <Sheet open={snapshotCamera !== null} onOpenChange={(open) => { if (!open) setSnapshotCamera(null) }}>
        <SheetContent className="sm:max-w-lg">
          <SheetHeader>
            <SheetTitle>{t("cameras.snapshotHistory")}</SheetTitle>
            <SheetDescription>{snapshotCamera?.name}</SheetDescription>
          </SheetHeader>
          {snapshotCamera && <SnapshotList token={token} cameraID={snapshotCamera.id} />}
        </SheetContent>
      </Sheet>

      {/* Delete Confirm */}
      <ConfirmActionDialog
        open={confirmDelete !== null}
        onOpenChange={(open) => { if (!open) setConfirmDelete(null) }}
        title={t("cameras.deleteTitle")}
        description={t("cameras.deleteDesc")}
        confirmLabel={t("cameras.deleteConfirm")}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={() => { if (confirmDelete) deleteMutation.mutate(confirmDelete) }}
      />
    </PageFrame>
  )
}

// --- Snapshot List Component ---

function SnapshotList({ token, cameraID }: { token: string; cameraID: string }) {
  const { t } = useTranslation()

  const snapshotsQuery = useQuery({
    queryKey: ["camera-snapshots", cameraID],
    queryFn: () => listCameraSnapshots(token, cameraID),
    enabled: Boolean(token && cameraID),
  })

  const snapshots: CameraSnapshot[] = snapshotsQuery.data ?? []

  if (snapshotsQuery.isLoading) {
    return (
      <div className="flex items-center justify-center py-8 text-muted-foreground">
        <Loader2Icon className="mr-2 h-4 w-4 animate-spin" />
        {t("cameras.loading")}
      </div>
    )
  }

  if (snapshots.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-center text-muted-foreground">
        <ImageIcon className="mb-2 h-8 w-8 opacity-50" />
        <p className="text-sm">{t("cameras.noSnapshots")}</p>
      </div>
    )
  }

  return (
    <div className="mt-4 space-y-3">
      {snapshots.map((snap) => (
        <Card key={snap.id}>
          <CardContent className="p-3">
            <div className="flex items-center justify-between text-xs">
              <div className="space-y-1">
                <p className="font-medium">{new Date(snap.captured_at).toLocaleString()}</p>
                <div className="flex items-center gap-2 text-muted-foreground">
                  <Badge variant="outline" className="text-[10px]">{snap.event_type || "manual"}</Badge>
                  <span>{(snap.size_bytes / 1024).toFixed(1)} KB</span>
                </div>
              </div>
              {snap.signed_url && (
                <a href={snap.signed_url} target="_blank" rel="noopener noreferrer">
                  <Button size="sm" variant="outline">
                    <ImageIcon className="mr-1 h-3 w-3" />
                    {t("cameras.view")}
                  </Button>
                </a>
              )}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
