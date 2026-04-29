import { useEffect, useState } from "react"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import {
  AlertCircleIcon,
  BellIcon,
  ChevronDownIcon,
  CreditCardIcon,
  DoorOpenIcon,
  ServerIcon,
  Trash2Icon,
  UsersIcon,
} from "lucide-react"

import {
  FormField,
  PageFrame,
  PanelHeader,
  SettingsPanel,
  SettingToggleRows,
} from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useMistyisletPlaceContext } from "@/features/mistyislet-shell/use-resource-summary"
import {
  cancelPlaceLockdown,
  deletePlace,
  lockDownPlace,
  updatePlace,
  type CurrentUser,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

export function PlaceSettingsAdaptedPage({
  token,
  viewer,
  placeID,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
}) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [activeTab, setActiveTab] = useState("General")
  const [placeName, setPlaceName] = useState("")
  const [address, setAddress] = useState("")
  const [region, setRegion] = useState("")
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [deleteConfirmText, setDeleteConfirmText] = useState("")
  const [actionNotice, setActionNotice] = useState("")
  const [actionError, setActionError] = useState("")
  const resourceQuery = useMistyisletPlaceContext(token, viewer, placeID)
  const { place } = resourceQuery.context
  const tenantID = getViewerTenantID(viewer)
  const canMutate = Boolean(tenantID && place && !resourceQuery.usingFallback)
  const canDeletePlace = canMutate && (viewer.role === "super_admin" || viewer.role === "tenant_admin")
  const deleteConfirmMatches = Boolean(place && deleteConfirmText.trim() === place.name)

  useEffect(() => {
    if (!place) {
      setPlaceName("")
      setAddress("")
      setRegion("")
      return
    }
    setPlaceName(place.name)
    setAddress(place.address)
    setRegion(place.region)
  }, [place])

  async function refreshPlace() {
    await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
  }

  const updatePlaceMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !place) {
        throw new Error("place is required")
      }
      return updatePlace(token, place.id, {
        tenant_id: tenantID,
        name: placeName.trim(),
        address: address.trim(),
        region: region.trim(),
      })
    },
    onSuccess: async () => {
      setActionNotice("Place saved.")
      setActionError("")
      await refreshPlace()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Place save failed")
    },
  })

  const placeActionMutation = useMutation({
    mutationFn: (action: "lock_down" | "cancel_lockdown") => {
      if (!tenantID || !place) {
        throw new Error("place is required")
      }
      return action === "lock_down" ? lockDownPlace(token, place.id, tenantID) : cancelPlaceLockdown(token, place.id, tenantID)
    },
    onSuccess: async (result) => {
      setActionNotice(result.action === "lock_down" ? "Place lockdown accepted." : "Place lockdown cancel accepted.")
      setActionError("")
      await refreshPlace()
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Place action failed")
    },
  })

  const deletePlaceMutation = useMutation({
    mutationFn: () => {
      if (!tenantID || !place) {
        throw new Error("place is required")
      }
      return deletePlace(token, place.id, tenantID)
    },
    onSuccess: async () => {
      setDeleteConfirmOpen(false)
      setDeleteConfirmText("")
      setActionError("")
      await refreshPlace()
      navigate("/places", { replace: true })
    },
    onError: (error) => {
      setActionNotice("")
      setActionError(error instanceof Error ? error.message : "Place delete failed")
    },
  })

  return (
    <>
      <PageFrame
        breadcrumbs={["Home", "Places", place?.name ?? "Assigned Place", "Place Settings"]}
        title={place?.name ?? "Place Settings"}
        description="Configure this place's identity, timezone, and access behavior"
      >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live place settings are unavailable. Showing reference data.
        </div>
      ) : null}
      {actionNotice ? (
        <div className="rounded-[6px] border border-[#b9dfc7] bg-[#f1fff5] px-5 py-4 text-sm text-[#1f6b3a]">
          {actionNotice}
        </div>
      ) : null}
      {actionError ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          {actionError}
        </div>
      ) : null}
      <SettingsPanel
        tabs={["General", "Access", "Schedules", "Notifications", "Advanced"]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <Button
            disabled={!canMutate || updatePlaceMutation.isPending || !placeName.trim()}
            onClick={() => updatePlaceMutation.mutate()}
            className="h-10 rounded-[8px] bg-[#4f55ff] px-8 text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
          >
            {updatePlaceMutation.isPending ? "Saving..." : "Save"}
          </Button>
        }
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title="General" description="Place name, address, and timezone." />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Place name</span>
                <input
                  value={placeName}
                  disabled={!place}
                  onChange={(event) => setPlaceName(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                />
              </label>
              <FormField label="Timezone" value="Asia/Jakarta" trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Address</span>
                <input
                  value={address}
                  disabled={!place}
                  onChange={(event) => setAddress(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Region</span>
                <input
                  value={region}
                  disabled={!place}
                  onChange={(event) => setRegion(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037] disabled:bg-[#f5f6f8]"
                />
              </label>
              <FormField label="Default floor" value="1st Floor" />
            </div>
          </>
        ) : null}

        {activeTab === "Access" ? (
          <>
            <PanelHeader title="Access" description="Place-wide access defaults." />
            <SettingToggleRows
              rows={[
                ["Allow mobile unlocks", true, "Users with valid Access Rights can unlock from mobile apps.", CreditCardIcon],
                ["Require reader unlock for sensitive doors", true, "Apply reader-required mode to high-security groups.", DoorOpenIcon],
                ["Suspend new users by default", false, "Require admin approval before newly synced users can unlock.", UsersIcon],
              ]}
            />
          </>
        ) : null}

        {activeTab === "Schedules" ? (
          <>
            <PanelHeader title="Schedules" description="Reusable schedules for door and group restrictions." />
            <div className="divide-y divide-[#eceef2]">
              {[
                ["Business Hours", "Mon-Fri 08:00-18:00", "Default group schedule"],
                ["After Hours", "Mon-Fri 18:00-22:00", "Facilities only"],
                ["Weekend Maintenance", "Sat-Sun 09:00-16:00", "Temporary vendors"],
              ].map((row) => (
                <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_220px_1fr] md:items-center">
                  <span className="font-semibold text-[#17171c]">{row[0]}</span>
                  <span className="text-sm text-[#2f3037]">{row[1]}</span>
                  <span className="text-sm text-[#6f717c]">{row[2]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {activeTab === "Notifications" ? (
          <>
            <PanelHeader title="Notifications" description="Place admin alert routing." />
            <SettingToggleRows
              rows={[
                ["Door forced open", true, "Notify place admins and security channel immediately.", BellIcon],
                ["Device offline", true, "Notify after five minutes without heartbeat.", ServerIcon],
                ["Access denied spike", true, "Create a review item when denial thresholds are exceeded.", AlertCircleIcon],
              ]}
            />
          </>
        ) : null}

        {activeTab === "Advanced" ? (
          <>
            <PanelHeader title="Advanced" description="Dangerous actions and data export controls." />
            <div className="space-y-4 p-7">
              {[
                ["Export place audit log", "Generate CSV for unlocks, denials, and admin changes."],
                ["Rotate device secrets", "Force all gateways and readers to refresh credentials."],
                ["Lock down place", "Send a lockdown command to all locks in this place."],
                ["Cancel lockdown", "Cancel the current place-wide lockdown command."],
              ].map((row, index) => (
                <div key={row[0]} className="flex gap-5 rounded-[6px] border border-[#eceef2] p-5">
                  <div>
                    <h3 className="font-semibold text-[#17171c]">{row[0]}</h3>
                    <p className="mt-1 text-sm text-[#6f717c]">{row[1]}</p>
                  </div>
                  <Button
                    variant="outline"
                    disabled={index > 1 && (!canMutate || placeActionMutation.isPending)}
                    onClick={() => {
                      if (index === 2) {
                        placeActionMutation.mutate("lock_down")
                      } else if (index === 3) {
                        placeActionMutation.mutate("cancel_lockdown")
                      }
                    }}
                    className={cn(
                      "ml-auto h-10 rounded-[6px] bg-white px-5",
                      index === 2
                        ? "border-[#f1b7b2] text-[#d93025] hover:border-[#f1b7b2] hover:bg-[#fff5f5] hover:text-[#9f1d1d]"
                        : "border-[#8589ff] text-[#4f55ff]"
                    )}
                  >
                    {index === 0 ? "Export" : index === 1 ? "Rotate" : index === 2 ? "Lockdown" : "Cancel"}
                  </Button>
                </div>
              ))}
              <div className="flex gap-5 rounded-[6px] border border-[#f1b7b2] bg-[#fffafa] p-5">
                <div>
                  <h3 className="font-semibold text-[#17171c]">Delete place</h3>
                  <p className="mt-1 text-sm text-[#6f717c]">Remove this place and return to the organization place list.</p>
                </div>
                <Button
                  variant="outline"
                  disabled={!canDeletePlace || deletePlaceMutation.isPending}
                  onClick={() => {
                    setActionNotice("")
                    setActionError("")
                    setDeleteConfirmOpen(true)
                  }}
                  className="ml-auto h-10 rounded-[6px] border-[#f1b7b2] bg-white px-5 text-[#d93025] hover:border-[#f1b7b2] hover:bg-[#fff5f5] hover:text-[#9f1d1d] disabled:border-[#d9dbe3] disabled:text-[#8d909b]"
                >
                  <Trash2Icon className="mr-1.5 size-4" />
                  Delete
                </Button>
              </div>
            </div>
          </>
        ) : null}
      </SettingsPanel>
      </PageFrame>

      <Dialog
        open={deleteConfirmOpen}
        onOpenChange={(open) => {
          setDeleteConfirmOpen(open)
          if (!open) {
            setDeleteConfirmText("")
          }
        }}
      >
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Delete place</DialogTitle>
            <DialogDescription>
              This removes {place?.name ?? "this place"} from the organization. Type the place name to confirm.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <label className="block">
              <span className="mb-2 block text-xs font-semibold uppercase text-[#6f717c]">Place name</span>
              <input
                value={deleteConfirmText}
                onChange={(event) => setDeleteConfirmText(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] px-3 text-sm text-[#2f3037]"
              />
            </label>
            {actionError ? (
              <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-4 py-3 text-sm text-[#8a5a00]">
                {actionError}
              </div>
            ) : null}
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="outline" className="h-10 rounded-[6px]">
                Cancel
              </Button>
            </DialogClose>
            <Button
              type="button"
              disabled={!canDeletePlace || !deleteConfirmMatches || deletePlaceMutation.isPending}
              onClick={() => deletePlaceMutation.mutate()}
              className="h-10 rounded-[6px] bg-[#d93025] px-6 text-white hover:bg-[#b42318] disabled:bg-[#f0c7c4]"
            >
              {deletePlaceMutation.isPending ? "Deleting..." : "Delete place"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
