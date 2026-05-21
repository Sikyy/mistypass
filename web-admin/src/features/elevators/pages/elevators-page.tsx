import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Building2Icon, PlusIcon, ShieldAlertIcon, ShieldOffIcon, Trash2Icon } from "lucide-react"

import { ConfirmActionDialog } from "@/components/mistyislet/actions"
import { PageFrame, StatusDot } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  cancelElevatorStopLockdown,
  createElevator,
  createElevatorStop,
  deleteElevator,
  deleteElevatorStop,
  listElevators,
  listElevatorStops,
  lockDownElevatorStop,
  type CurrentUser,
  type Elevator,
  type ElevatorStop,
} from "@/lib/api"
import { getViewerTenantID } from "@/lib/viewer"

export function ElevatorsAdaptedPage({ token, viewer, placeID }: { token: string; viewer: CurrentUser; placeID?: string }) {
  const queryClient = useQueryClient()
  const tenantID = getViewerTenantID(viewer)
  const [showCreate, setShowCreate] = useState(false)
  const [elevatorName, setElevatorName] = useState("")
  const [showCreateStop, setShowCreateStop] = useState<string | null>(null)
  const [stopName, setStopName] = useState("")
  const [confirmDelete, setConfirmDelete] = useState<{ type: "elevator" | "stop"; id: string } | null>(null)

  const elevatorsQuery = useQuery({
    queryKey: ["elevators", tenantID],
    queryFn: () => listElevators(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const stopsQuery = useQuery({
    queryKey: ["elevator-stops", tenantID],
    queryFn: () => listElevatorStops(token, tenantID),
    enabled: Boolean(token && tenantID),
  })

  const createMutation = useMutation({
    mutationFn: () => createElevator(token, { tenant_id: tenantID, place_id: placeID ?? "", name: elevatorName }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["elevators"] }); setShowCreate(false); setElevatorName("") },
    onError: (error) => alert(error instanceof Error ? error.message : "Failed to create elevator"),
  })

  const createStopMutation = useMutation({
    mutationFn: () => {
      if (!showCreateStop) return Promise.reject(new Error("No elevator selected"))
      return createElevatorStop(token, { tenant_id: tenantID, elevator_id: showCreateStop, name: stopName })
    },
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["elevator-stops"] }); setShowCreateStop(null); setStopName("") },
    onError: (error) => alert(error instanceof Error ? error.message : "Failed to create stop"),
  })

  const deleteMutation = useMutation({
    mutationFn: (target: { type: string; id: string }) =>
      target.type === "elevator" ? deleteElevator(token, target.id, tenantID) : deleteElevatorStop(token, target.id, tenantID),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ["elevators"] }); queryClient.invalidateQueries({ queryKey: ["elevator-stops"] }); setConfirmDelete(null) },
    onError: (error) => alert(error instanceof Error ? error.message : "Failed to delete"),
  })

  const lockdownMutation = useMutation({
    mutationFn: ({ id, action }: { id: string; action: "lock_down" | "cancel" }) =>
      action === "lock_down" ? lockDownElevatorStop(token, id, tenantID) : cancelElevatorStopLockdown(token, id, tenantID),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["elevator-stops"] }),
    onError: (error) => alert(error instanceof Error ? error.message : "Lockdown operation failed"),
  })

  const elevators = elevatorsQuery.data?.items ?? []
  const stops = stopsQuery.data?.items ?? []

  return (
    <PageFrame
      breadcrumbs={["Dashboard", "Places", "Elevators"]}
      title="Elevators"
      description="Manage elevator access and floor stops."
      actions={
        <Button className="h-11 rounded-[6px] bg-brand px-6 text-white hover:bg-brand-hover" onClick={() => setShowCreate(true)}>
          <PlusIcon className="mr-2 size-4" /> Add Elevator
        </Button>
      }
    >
      {showCreate && (
        <div className="mb-6 rounded-[6px] border border-line-subtle bg-white p-6">
          <h3 className="mb-3 font-semibold text-content-heading">New Elevator</h3>
          <div className="flex gap-3">
            <input
              type="text" placeholder="Elevator name" value={elevatorName}
              onChange={(e) => setElevatorName(e.target.value)}
              className="h-10 flex-1 rounded-[6px] border border-line-default bg-white px-3 text-sm outline-none focus:border-brand-ring"
            />
            <Button className="h-10 bg-brand px-5 text-white" disabled={!elevatorName.trim()} onClick={() => createMutation.mutate()}>Create</Button>
            <Button variant="outline" className="h-10" onClick={() => setShowCreate(false)}>Cancel</Button>
          </div>
        </div>
      )}

      {elevators.length === 0 ? (
        <div className="rounded-[6px] border border-line-default bg-white px-6 py-12 text-center">
          <Building2Icon className="mx-auto size-8 text-content-muted" />
          <p className="mt-3 text-sm font-semibold text-content-heading">No elevators</p>
          <p className="mt-1 text-sm text-content-subtle">Add an elevator to manage floor access.</p>
        </div>
      ) : (
        elevators.map((elevator) => {
          const elevatorStops = stops.filter((s) => s.elevator_id === elevator.id)
          return (
            <div key={elevator.id} className="mb-4 rounded-[6px] border border-line-subtle bg-white">
              <div className="flex items-center gap-4 border-b border-line-subtle px-5 py-4">
                <Building2Icon className="size-5 text-brand" />
                <div className="flex-1">
                  <h3 className="font-semibold text-content-heading">{elevator.name}</h3>
                  {elevator.description && <p className="text-sm text-content-subtle">{elevator.description}</p>}
                </div>
                <span className="text-sm text-content-muted">{elevatorStops.length} stops</span>
                <Button variant="outline" size="sm" onClick={() => { setShowCreateStop(elevator.id); setStopName("") }}>
                  <PlusIcon className="mr-1 size-3" /> Stop
                </Button>
                <button type="button" className="size-8 rounded-[6px] text-content-subtle hover:bg-surface-page" onClick={() => setConfirmDelete({ type: "elevator", id: elevator.id })}>
                  <Trash2Icon className="mx-auto size-4" />
                </button>
              </div>
              {showCreateStop === elevator.id && (
                <div className="flex gap-3 border-b border-line-subtle px-5 py-3 bg-surface-page">
                  <input type="text" placeholder="Stop name (e.g. Floor 3)" value={stopName} onChange={(e) => setStopName(e.target.value)}
                    className="h-9 flex-1 rounded-[6px] border border-line-default bg-white px-3 text-sm outline-none focus:border-brand-ring" />
                  <Button size="sm" className="bg-brand text-white" disabled={!stopName.trim()} onClick={() => createStopMutation.mutate()}>Add</Button>
                  <Button size="sm" variant="outline" onClick={() => setShowCreateStop(null)}>Cancel</Button>
                </div>
              )}
              {elevatorStops.map((stop) => (
                <div key={stop.id} className="flex items-center gap-4 border-b border-line-subtle px-5 py-3 last:border-b-0">
                  <span className="flex-1 text-sm font-medium text-content-heading">{stop.name}</span>
                  <StatusDot tone={stop.status === "locked_down" ? "danger" : "success"} label={stop.status === "locked_down" ? "Locked Down" : "Active"} />
                  {stop.status === "locked_down" ? (
                    <button type="button" title="Cancel lockdown" className="size-7 rounded text-green-600 hover:bg-green-50"
                      onClick={() => lockdownMutation.mutate({ id: stop.id, action: "cancel" })}>
                      <ShieldOffIcon className="mx-auto size-4" />
                    </button>
                  ) : (
                    <button type="button" title="Lock down" className="size-7 rounded text-red-500 hover:bg-red-50"
                      onClick={() => lockdownMutation.mutate({ id: stop.id, action: "lock_down" })}>
                      <ShieldAlertIcon className="mx-auto size-4" />
                    </button>
                  )}
                  <button type="button" className="size-7 rounded text-content-subtle hover:bg-surface-page"
                    onClick={() => setConfirmDelete({ type: "stop", id: stop.id })}>
                    <Trash2Icon className="mx-auto size-4" />
                  </button>
                </div>
              ))}
            </div>
          )
        })
      )}

      <ConfirmActionDialog
        open={confirmDelete !== null}
        title={confirmDelete?.type === "elevator" ? "Delete elevator" : "Delete elevator stop"}
        description="This action cannot be undone."
        confirmLabel="Delete"
        onConfirm={() => confirmDelete && deleteMutation.mutate(confirmDelete)}
        onOpenChange={(open) => { if (!open) setConfirmDelete(null) }}
        pending={deleteMutation.isPending}
        destructive
      />
    </PageFrame>
  )
}
