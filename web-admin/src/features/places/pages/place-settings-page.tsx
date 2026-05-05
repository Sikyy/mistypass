import i18next from "i18next"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router"
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
  const { t } = useTranslation()
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
      setActionNotice(t("common.save"))
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
        breadcrumbs={[t("common.home"), "Places", place?.name ?? "Assigned Place", "Place Settings"]}
        title={place?.name ?? "Place Settings"}
        description={t("kisi.floors.description")}
      >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-warning-bg px-5 py-4 text-sm text-warning-text">
          Live place settings are unavailable. Showing reference data.
        </div>
      ) : null}
      {actionNotice ? (
        <div className="rounded-[6px] border border-[#b9dfc7] bg-[#f1fff5] px-5 py-4 text-sm text-[#1f6b3a]">
          {actionNotice}
        </div>
      ) : null}
      {actionError ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-warning-bg px-5 py-4 text-sm text-warning-text">
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
            className="h-10 rounded-[8px] bg-brand px-8 text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
          >
            {updatePlaceMutation.isPending ? "Saving..." : "Save"}
          </Button>
        }
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title={t("common.general")} description={t("kisi.floors.description")} />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-content-subtle">{t("common.name")}</span>
                <input
                  value={placeName}
                  disabled={!place}
                  onChange={(event) => setPlaceName(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-sunken"
                />
              </label>
              <FormField label={t("kisi.doors.timezone")} value="Asia/Jakarta" trailing={<ChevronDownIcon className="size-4 text-content-subtle" />} />
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-content-subtle">{t("common.description")}</span>
                <input
                  value={address}
                  disabled={!place}
                  onChange={(event) => setAddress(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-sunken"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase text-content-subtle">{t("common.place")}</span>
                <input
                  value={region}
                  disabled={!place}
                  onChange={(event) => setRegion(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body disabled:bg-surface-sunken"
                />
              </label>
              <FormField label={t("kisi.doors.floor")} value="1st Floor" />
            </div>
          </>
        ) : null}

        {activeTab === "Access" ? (
          <>
            <PanelHeader title={t("common.permissions")} description={t("kisi.floors.description")} />
            <SettingToggleRows
              rows={[
                [i18next.t("common.permissions"), true, i18next.t("kisi.floors.description"), CreditCardIcon],
                [i18next.t("common.permissions"), true, i18next.t("kisi.floors.description"), DoorOpenIcon],
                [i18next.t("common.permissions"), false, i18next.t("kisi.floors.description"), UsersIcon],
              ]}
            />
          </>
        ) : null}

        {activeTab === "Schedules" ? (
          <>
            <PanelHeader title={t("kisi.accessRights.schedule")} description={t("kisi.floors.description")} />
            <div className="divide-y divide-line-subtle">
              {[
                ["Business Hours", "Mon-Fri 08:00-18:00", "Default group schedule"],
                ["After Hours", "Mon-Fri 18:00-22:00", "Facilities only"],
                ["Weekend Maintenance", "Sat-Sun 09:00-16:00", "Temporary vendors"],
              ].map((row) => (
                <div key={row[0]} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_220px_1fr] md:items-center">
                  <span className="font-semibold text-content-heading">{row[0]}</span>
                  <span className="text-sm text-content-body">{row[1]}</span>
                  <span className="text-sm text-content-subtle">{row[2]}</span>
                </div>
              ))}
            </div>
          </>
        ) : null}

        {activeTab === "Notifications" ? (
          <>
            <PanelHeader title={t("kisi.orgSetup.notifications")} description={t("kisi.floors.description")} />
            <SettingToggleRows
              rows={[
                [i18next.t("kisi.orgSetup.notifications"), true, i18next.t("kisi.floors.description"), BellIcon],
                [i18next.t("kisi.orgSetup.notifications"), true, i18next.t("kisi.floors.description"), ServerIcon],
                ["Access denied spike", true, "Create a review item when denial thresholds are exceeded.", AlertCircleIcon],
              ]}
            />
          </>
        ) : null}

        {activeTab === "Advanced" ? (
          <>
            <PanelHeader title={t("common.settings")} description={t("kisi.floors.description")} />
            <div className="space-y-4 p-7">
              {[
                ["Export place audit log", "Generate CSV for unlocks, denials, and admin changes."],
                ["Rotate device secrets", "Force all gateways and readers to refresh credentials."],
                [i18next.t("kisi.doors.lockdown"), i18next.t("kisi.floors.description")],
                ["Cancel lockdown", "Cancel the current place-wide lockdown command."],
              ].map((row, index) => (
                <div key={row[0]} className="flex gap-5 rounded-[6px] border border-line-subtle p-5">
                  <div>
                    <h3 className="font-semibold text-content-heading">{row[0]}</h3>
                    <p className="mt-1 text-sm text-content-subtle">{row[1]}</p>
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
                  <h3 className="font-semibold text-content-heading">{t("common.delete")}</h3>
                  <p className="mt-1 text-sm text-content-subtle">{t("common.deleteConfirm")}</p>
                </div>
                <Button
                  variant="outline"
                  disabled={!canDeletePlace || deletePlaceMutation.isPending}
                  onClick={() => {
                    setActionNotice("")
                    setActionError("")
                    setDeleteConfirmOpen(true)
                  }}
                  className="ml-auto h-10 rounded-[6px] border-[#f1b7b2] bg-white px-5 text-[#d93025] hover:border-[#f1b7b2] hover:bg-[#fff5f5] hover:text-[#9f1d1d] disabled:border-line-default disabled:text-[#8d909b]"
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
            <DialogTitle>{t("common.delete")}</DialogTitle>
            <DialogDescription>
              This removes {place?.name ?? "this place"} from the organization. Type the place name to confirm.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <label className="block">
              <span className="mb-2 block text-xs font-semibold uppercase text-content-subtle">{t("common.name")}</span>
              <input
                value={deleteConfirmText}
                onChange={(event) => setDeleteConfirmText(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default px-3 text-sm text-content-body"
              />
            </label>
            {actionError ? (
              <div className="rounded-[6px] border border-[#f1c27a] bg-warning-bg px-4 py-3 text-sm text-warning-text">
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
