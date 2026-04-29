import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  ChevronDownIcon,
  CreditCardIcon,
  DoorOpenIcon,
  LayersIcon,
  MapPinPlusIcon,
  PencilIcon,
  PlusIcon,
  ShieldCheckIcon,
  Trash2Icon,
} from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/mistyislet/actions"
import { FormField, PageFrame, PanelHeader, SettingsPanel, StatusDot, ToggleSwitch } from "@/components/mistyislet/primitives"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { selectMistyisletPlaceContext } from "@/features/mistyislet-shell/resource-data"
import { useMistyisletResourceSummary } from "@/features/mistyislet-shell/use-resource-summary"
import {
  createGroup,
  createGroupLink,
  createGroupLock,
  deleteGroup,
  deleteGroupLink,
  deleteGroupLock,
  listGroupLinks,
  updateGroup,
  updateGroupLink,
  type CurrentUser,
  type GroupLink,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

export function GroupsAdaptedPage({
  token,
  viewer,
  placeID,
  placeScoped = false,
}: {
  token: string
  viewer: CurrentUser
  placeID?: string
  placeScoped?: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [activeTab, setActiveTab] = useState("Permissions")
  const [selectedGroupID, setSelectedGroupID] = useState("")
  const [deleteGroupConfirmOpen, setDeleteGroupConfirmOpen] = useState(false)
  const [createGroupOpen, setCreateGroupOpen] = useState(false)
  const [createGroupName, setCreateGroupName] = useState("")
  const [createGroupDescription, setCreateGroupDescription] = useState("")
  const [createGroupPlaceID, setCreateGroupPlaceID] = useState("")
  const [editGroupName, setEditGroupName] = useState("")
  const [editGroupDescription, setEditGroupDescription] = useState("")
  const [addDoorsOpen, setAddDoorsOpen] = useState(false)
  const [selectedDoorID, setSelectedDoorID] = useState("")
  const [createLinkOpen, setCreateLinkOpen] = useState(false)
  const [createLinkName, setCreateLinkName] = useState("")
  const [createLinkEmail, setCreateLinkEmail] = useState("")
  const [createLinkValidUntil, setCreateLinkValidUntil] = useState("")
  const [createLinkQRCodeType, setCreateLinkQRCodeType] = useState<"online" | "offline" | "">("online")
  const [editLinkOpen, setEditLinkOpen] = useState(false)
  const [editingLinkID, setEditingLinkID] = useState("")
  const [editLinkName, setEditLinkName] = useState("")
  const [editLinkEmail, setEditLinkEmail] = useState("")
  const [editLinkValidUntil, setEditLinkValidUntil] = useState("")
  const [editLinkQRCodeType, setEditLinkQRCodeType] = useState<"online" | "offline" | "">("online")
  const [editLinkEnabled, setEditLinkEnabled] = useState(true)
  const [deleteLinkTarget, setDeleteLinkTarget] = useState<GroupLink | null>(null)
  const [actionError, setActionError] = useState("")
  const resourceQuery = useMistyisletResourceSummary(token, viewer)
  const placeContext = selectMistyisletPlaceContext(resourceQuery.summary, placeID)
  const groupRows = placeScoped ? placeContext.groups : resourceQuery.summary.groups
  const currentGroup = groupRows.find((group) => group.id === selectedGroupID) ?? groupRows[0]
  const currentAccessRights = (placeScoped ? placeContext.accessRights : resourceQuery.summary.accessRights)
    .filter((item) => !currentGroup || item.name === currentGroup.name || item.subjectType === "Group")
    .slice(0, 5)
  const memberRows = (placeScoped ? placeContext.users : resourceQuery.summary.users).slice(0, 5)
  const allDoorRows = placeScoped ? placeContext.doors : resourceQuery.summary.doors
  const currentDoorIDs = new Set(currentGroup?.doorIds ?? [])
  const doorRows = allDoorRows.filter((row) => currentDoorIDs.has(row.id))
  const availableDoorRows = allDoorRows.filter((row) => !currentDoorIDs.has(row.id))
  const currentZoneIDs = new Set(currentGroup?.zoneIds ?? [])
  const allZoneRows = placeScoped ? placeContext.zones : resourceQuery.summary.zones
  const zoneRows = allZoneRows.filter((row) => currentZoneIDs.has(row.id))
  const tenantID = getViewerTenantID(viewer)
  const placeRows = placeScoped && placeContext.place ? [placeContext.place] : resourceQuery.summary.places
  const canCreateGroups = Boolean(tenantID && !resourceQuery.usingFallback)
  const canMutateGroups = Boolean(tenantID && currentGroup?.kind === "User group" && !resourceQuery.usingFallback)
  const canMutateGroupLocks = Boolean(tenantID && currentGroup?.kind === "Door group" && !resourceQuery.usingFallback)
  const canMutateGroupLinks = Boolean(tenantID && currentGroup?.kind === "User group" && !resourceQuery.usingFallback)
  const groupFormDirty =
    Boolean(currentGroup) &&
    (editGroupName.trim() !== (currentGroup?.name ?? "") || editGroupDescription.trim() !== (currentGroup?.description ?? ""))
  const groupLinksQuery = useQuery({
    queryKey: ["reference-group-links", tenantID ?? "none", currentGroup?.id ?? "none"],
    queryFn: () => listGroupLinks(token, { tenant_id: tenantID, group_id: currentGroup?.id }),
    enabled: Boolean(tenantID && currentGroup?.id && currentGroup?.kind === "User group" && !resourceQuery.usingFallback),
    staleTime: 30 * 1000,
  })
  const groupLinks = groupLinksQuery.data ?? []
  const restrictions = [
    {
      key: "primary_device_restriction_enabled",
      title: "Primary Device Restriction",
      description: "Only allow unlocks from primary smartphones.",
      enabled: Boolean(currentGroup?.primaryDeviceRestrictionEnabled),
      icon: ShieldCheckIcon,
    },
    {
      key: "login_enabled",
      title: "Allow App Access",
      description: "If enabled, users may access using Mistyislet mobile and web apps.",
      enabled: currentGroup?.loginEnabled ?? true,
      icon: CreditCardIcon,
    },
    {
      key: "geofence_restriction_enabled",
      title: "Geofence Restriction",
      description: `Require users to be near the place location${currentGroup?.geofenceRestrictionRadius ? ` within ${currentGroup.geofenceRestrictionRadius}m` : ""}.`,
      enabled: Boolean(currentGroup?.geofenceRestrictionEnabled),
      icon: MapPinPlusIcon,
    },
    {
      key: "reader_restriction_enabled",
      title: "Reader Restriction",
      description: "Require unlocks at the reader for this group's doors.",
      enabled: Boolean(currentGroup?.readerRestrictionEnabled),
      icon: DoorOpenIcon,
    },
    {
      key: "tap_to_access_restriction_enabled",
      title: "Tap To Access Restriction",
      description: "Require reader unlock instead of remote app unlock.",
      enabled: Boolean(currentGroup?.tapToAccessRestrictionEnabled),
      icon: DoorOpenIcon,
    },
  ] as const

  useEffect(() => {
    setEditGroupName(currentGroup?.name ?? "")
    setEditGroupDescription(currentGroup?.description ?? "")
  }, [currentGroup?.id, currentGroup?.name, currentGroup?.description])

  async function refreshGroups() {
    await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
  }

  const createGroupMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      const name = createGroupName.trim()
      if (!name) {
        throw new Error("group name is required")
      }
      const selectedPlaceID = placeScoped ? placeContext.place?.id ?? placeID ?? "" : createGroupPlaceID.trim()
      return createGroup(token, {
        tenant_id: tenantID,
        place_id: selectedPlaceID || undefined,
        name,
        description: createGroupDescription.trim(),
      })
    },
    onSuccess: async (created) => {
      setSelectedGroupID(created.id)
      setCreateGroupOpen(false)
      setCreateGroupName("")
      setCreateGroupDescription("")
      setCreateGroupPlaceID("")
      setActionError("")
      await refreshGroups()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Group create failed"),
  })

  const updateGroupMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!currentGroup) {
        throw new Error("group is required")
      }
      const name = editGroupName.trim()
      if (!name) {
        throw new Error("group name is required")
      }
      return updateGroup(token, currentGroup.id, {
        tenant_id: tenantID,
        place_id: currentGroup.placeId || undefined,
        name,
        description: editGroupDescription.trim(),
      })
    },
    onSuccess: async () => {
      setActionError("")
      await refreshGroups()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Group update failed"),
  })

  const deleteGroupMutation = useMutation({
    mutationFn: (groupID: string) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!groupID) {
        throw new Error("group is required")
      }
      return deleteGroup(token, groupID, tenantID)
    },
    onSuccess: async () => {
      setSelectedGroupID("")
      setDeleteGroupConfirmOpen(false)
      setActionError("")
      await refreshGroups()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Group delete failed"),
  })

  const addDoorMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!currentGroup) {
        throw new Error("group is required")
      }
      const doorID = selectedDoorID.trim()
      if (!doorID) {
        throw new Error("door is required")
      }
      const door = allDoorRows.find((item) => item.id === doorID)
      return createGroupLock(token, {
        tenant_id: tenantID,
        group_id: currentGroup.id,
        lock_id: doorID,
        place_id: door?.placeId,
      })
    },
    onSuccess: async () => {
      setAddDoorsOpen(false)
      setSelectedDoorID("")
      setActionError("")
      await refreshGroups()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Door assignment failed"),
  })

  const removeDoorMutation = useMutation({
    mutationFn: (doorID: string) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!currentGroup) {
        throw new Error("group is required")
      }
      return deleteGroupLock(token, `${currentGroup.id}:${doorID}`, tenantID)
    },
    onSuccess: async () => {
      setActionError("")
      await refreshGroups()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Door removal failed"),
  })

  const updateRestrictionMutation = useMutation({
    mutationFn: (patch: Record<string, boolean | number | string | undefined>) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!currentGroup) {
        throw new Error("group is required")
      }
      return updateGroup(token, currentGroup.id, {
        tenant_id: tenantID,
        place_id: currentGroup.placeId || undefined,
        name: currentGroup.name,
        description: currentGroup.description,
        ...patch,
      })
    },
    onSuccess: async () => {
      setActionError("")
      await refreshGroups()
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Restriction update failed"),
  })

  const createLinkMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!currentGroup) {
        throw new Error("group is required")
      }
      const name = createLinkName.trim()
      if (!name) {
        throw new Error("link name is required")
      }
      return createGroupLink(token, {
        tenant_id: tenantID,
        group_id: currentGroup.id,
        name,
        email: createLinkEmail.trim() || undefined,
        quick_response_code_type: createLinkQRCodeType,
        valid_until: createLinkValidUntil.trim() || undefined,
        link_enabled: true,
      })
    },
    onSuccess: async () => {
      setCreateLinkOpen(false)
      setCreateLinkName("")
      setCreateLinkEmail("")
      setCreateLinkValidUntil("")
      setCreateLinkQRCodeType("online")
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-group-links"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Group link create failed"),
  })

  const updateLinkMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!editingLinkID) {
        throw new Error("group link is required")
      }
      const name = editLinkName.trim()
      if (!name) {
        throw new Error("link name is required")
      }
      return updateGroupLink(token, editingLinkID, {
        tenant_id: tenantID,
        name,
        email: editLinkEmail.trim(),
        quick_response_code_type: editLinkQRCodeType,
        valid_until: editLinkValidUntil.trim(),
        link_enabled: editLinkEnabled,
      })
    },
    onSuccess: async () => {
      setEditLinkOpen(false)
      setEditingLinkID("")
      setEditLinkName("")
      setEditLinkEmail("")
      setEditLinkValidUntil("")
      setEditLinkQRCodeType("online")
      setEditLinkEnabled(true)
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-group-links"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Group link update failed"),
  })

  const deleteLinkMutation = useMutation({
    mutationFn: (groupLinkID: string) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      return deleteGroupLink(token, groupLinkID, tenantID)
    },
    onSuccess: async () => {
      setDeleteLinkTarget(null)
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-group-links"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Group link delete failed"),
  })

  function openEditLink(link: GroupLink) {
    setEditingLinkID(link.id)
    setEditLinkName(link.name)
    setEditLinkEmail(link.email ?? "")
    setEditLinkValidUntil(link.valid_until ?? "")
    setEditLinkQRCodeType(
      link.quick_response_code_type === "offline" || link.quick_response_code_type === "online"
        ? link.quick_response_code_type
        : ""
    )
    setEditLinkEnabled(link.link_enabled)
    setActionError("")
    setEditLinkOpen(true)
  }

  return (
    <PageFrame
      breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Groups"] : ["Home", "Groups"]}
      title={currentGroup?.name ?? (placeScoped ? "Place Groups" : "Groups")}
      count={resourceQuery.isPending ? "--" : groupRows.length}
      description={t("kisi.groups.pageDesc")}
      actions={
        <>
          <Button
            type="button"
            disabled={!canCreateGroups}
            onClick={() => {
              setCreateGroupPlaceID(placeScoped ? placeContext.place?.id ?? placeID ?? "" : placeRows[0]?.id ?? "")
              setCreateGroupName("")
              setCreateGroupDescription("")
              setActionError("")
              setCreateGroupOpen(true)
            }}
            className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            Add Group
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={!canMutateGroups || !currentGroup || deleteGroupMutation.isPending}
            onClick={() => {
              setActionError("")
              setDeleteGroupConfirmOpen(true)
            }}
            className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
          >
            <Trash2Icon className="mr-1.5 size-4" />
            Delete Group
          </Button>
        </>
      }
    >
      {resourceQuery.usingFallback ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          Live group resources are unavailable. Showing reference data.
        </div>
      ) : null}
      {actionError ? (
        <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
          {actionError}
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
        <div className="border-b border-[#eceef2] px-6 py-5">
          <h2 className="text-base font-semibold text-[#17171c]">{placeScoped ? "Place Groups" : "Organization Groups"}</h2>
          <p className="mt-1 text-sm text-[#6f717c]">{t("kisi.groups.pageDesc")}</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead className="bg-[#fbfbfc]">
              <tr className="border-b border-[#eceef2]">
                <th className="px-6 py-4 font-semibold">{t("common.name")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.type")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.target")}</th>
                <th className="px-4 py-4 font-semibold">{t("kisi.groups.membersDoors")}</th>
                <th className="px-4 py-4 font-semibold">{t("common.status")}</th>
              </tr>
            </thead>
            <tbody>
              {groupRows.map((group) => (
                <tr
                  key={group.id}
                  aria-selected={currentGroup?.id === group.id}
                  onClick={() => setSelectedGroupID(group.id)}
                  className={cn(
                    "cursor-pointer border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]",
                    currentGroup?.id === group.id ? "bg-[#f7f7ff]" : ""
                  )}
                >
                  <td className="px-6 py-4 font-semibold text-[#4f55ff]">{group.name}</td>
                  <td className="px-4 py-4 text-[#2f3037]">{group.kind}</td>
                  <td className="px-4 py-4 text-[#6f717c]">{group.targetLabel}</td>
                  <td className="px-4 py-4 text-[#2f3037]">{group.memberCount}</td>
                  <td className="px-4 py-4">
                    <StatusDot tone={group.tone} label={group.statusLabel} />
                  </td>
                </tr>
              ))}
              {groupRows.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-6 py-10 text-center text-sm text-[#6f717c]">
                    No groups found for this scope.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <SettingsPanel
        tabs={[t("kisi.groups.tabGeneral"), t("kisi.groups.tabMembers"), t("kisi.groups.tabDoors"), t("kisi.groups.tabZones"), t("kisi.groups.tabLinks"), t("kisi.groups.tabTimeRestrictions"), t("kisi.groups.tabPermissions")]}
        active={activeTab}
        onTabChange={setActiveTab}
        footer={
          <Button
            type="button"
            disabled={!canMutateGroups || !groupFormDirty || !editGroupName.trim() || updateGroupMutation.isPending}
            onClick={() => updateGroupMutation.mutate()}
            className="h-10 rounded-[8px] bg-[#4f55ff] px-8 text-white hover:bg-[#454bea] disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
          >
            {updateGroupMutation.isPending ? "Saving..." : "Save"}
          </Button>
        }
      >
        {activeTab === "General" ? (
          <>
            <PanelHeader title={t("common.general")} description={t("kisi.groups.generalDesc")} />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("kisi.groups.groupName")}</span>
                <input
                  value={editGroupName}
                  disabled={!canMutateGroups}
                  onChange={(event) => setEditGroupName(event.target.value)}
                  className="h-12 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-4 text-sm text-[#2f3037] disabled:bg-[#fbfbfc] disabled:text-[#6f717c]"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.description")}</span>
                <input
                  value={editGroupDescription}
                  disabled={!canMutateGroups}
                  onChange={(event) => setEditGroupDescription(event.target.value)}
                  className="h-12 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-4 text-sm text-[#2f3037] disabled:bg-[#fbfbfc] disabled:text-[#6f717c]"
                />
              </label>
              <FormField label="Access enabled" value={currentGroup?.statusLabel ?? "Unavailable"} trailing={<ToggleSwitch enabled={currentGroup?.tone !== "danger"} />} />
              <FormField label="Default role" value="Basic" trailing={<ChevronDownIcon className="size-4 text-[#6f717c]" />} />
              <FormField label="Place scope" value={placeScoped ? placeContext.place?.name ?? "Assigned Place" : "All organization places"} />
              <FormField label="Assigned rights" value={`${currentAccessRights.length} access rights`} />
            </div>
          </>
        ) : null}

        {activeTab === "Members" ? (
          <>
            <PanelHeader
              title="Members"
              description={t("kisi.groups.membersDesc")}
              action={
                <Button variant="outline" className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]">
                  Add Members
                </Button>
              }
            />
            <div className="divide-y divide-[#eceef2]">
              {memberRows.map((row) => (
                <div key={row.id} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_150px_1fr] md:items-center">
                  <span className="font-semibold text-[#17171c]">{row.name}</span>
                  <span className="text-sm text-[#2f3037]">{row.role}</span>
                  <span className="text-sm text-[#6f717c]">{row.email}</span>
                </div>
              ))}
              {memberRows.length === 0 ? <div className="px-7 py-8 text-sm text-[#6f717c]">{t("kisi.groups.noMembers")}</div> : null}
            </div>
          </>
        ) : null}

        {activeTab === "Doors" ? (
          <>
            <PanelHeader
              title="Doors"
              description={t("kisi.groups.doorsDesc")}
              action={
                <Button
                  type="button"
                  variant="outline"
                  disabled={!canMutateGroupLocks || availableDoorRows.length === 0}
                  onClick={() => {
                    setSelectedDoorID(availableDoorRows[0]?.id ?? "")
                    setActionError("")
                    setAddDoorsOpen(true)
                  }}
                  className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                >
                  <PlusIcon className="mr-1.5 size-4" />
                  Add Doors
                </Button>
              }
            />
            <div className="divide-y divide-[#eceef2]">
              {doorRows.map((row) => (
                <div key={row.id} className="grid gap-3 px-7 py-5 md:grid-cols-[minmax(180px,1fr)_180px_140px_100px] md:items-center">
                  <span className="font-semibold text-[#4f55ff]">{row.name}</span>
                  <span className="text-sm text-[#2f3037]">{row.floorName}</span>
                  <StatusDot tone={row.status === "online" ? "success" : "warning"} label={row.status === "online" ? "Online" : "Review"} />
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    disabled={!canMutateGroupLocks || removeDoorMutation.isPending}
                    onClick={() => removeDoorMutation.mutate(row.id)}
                    className="justify-self-start rounded-[6px] text-[#6f717c] md:justify-self-end"
                  >
                    Remove
                  </Button>
                </div>
              ))}
              {doorRows.length === 0 ? <div className="px-7 py-8 text-sm text-[#6f717c]">{t("kisi.groups.noDoors")}</div> : null}
            </div>
          </>
        ) : null}

        {activeTab === "Zones" ? (
          <>
            <PanelHeader title={t("kisi.groups.zones")} description={t("kisi.groups.zonesDesc")} />
            <div className="grid gap-4 p-7 md:grid-cols-2">
              {zoneRows.map((row) => (
                <div key={row.id} className="rounded-[6px] border border-[#eceef2] p-5">
                  <LayersIcon className="size-6 text-[#6f717c]" />
                  <h3 className="mt-5 font-semibold text-[#17171c]">{row.name}</h3>
                  <p className="mt-1 text-sm text-[#6f717c]">{row.floorName} · {row.description}</p>
                  <p className="mt-4 text-sm font-semibold text-[#2f3037]">{row.doorCount} doors</p>
                </div>
              ))}
              {zoneRows.length === 0 ? <div className="text-sm text-[#6f717c]">{t("kisi.groups.noZones")}</div> : null}
            </div>
          </>
        ) : null}

        {activeTab === "Links" ? (
          <>
            <PanelHeader
              title="Links"
              description={t("kisi.groups.linksDesc")}
              action={
                <Button
                  type="button"
                  variant="outline"
                  disabled={!canMutateGroupLinks}
                  onClick={() => {
                    setCreateLinkName(currentGroup ? `${currentGroup.name} access link` : "")
                    setCreateLinkEmail("")
                    setCreateLinkValidUntil("")
                    setCreateLinkQRCodeType("online")
                    setActionError("")
                    setCreateLinkOpen(true)
                  }}
                  className="h-10 rounded-[6px] border-[#8589ff] bg-white px-6 text-[#4f55ff] hover:border-[#6f74ff] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
                >
                  <PlusIcon className="mr-1.5 size-4" />
                  Add Link
                </Button>
              }
            />
            <div className="divide-y divide-[#eceef2]">
              {groupLinks.map((link) => (
                <div key={link.id} className="grid gap-3 px-7 py-5 md:grid-cols-[minmax(180px,1fr)_minmax(180px,1fr)_140px_190px] md:items-center">
                  <div className="min-w-0">
                    <span className="block truncate font-semibold text-[#17171c]">{link.name}</span>
                    <span className="mt-1 block truncate text-sm text-[#6f717c]">{link.email || link.phone || "Reusable link"}</span>
                  </div>
                  <span className="text-sm text-[#2f3037]">{link.valid_until ? `Expires ${link.valid_until}` : "No expiry"}</span>
                  <StatusDot tone={link.link_enabled ? "success" : "warning"} label={link.link_enabled ? "Enabled" : "Disabled"} />
                  <div className="justify-self-start md:justify-self-end">
                    <RowActionsMenu
                      label={`Actions for ${link.name}`}
                      items={[
                        {
                          id: "edit",
                          label: "Edit",
                          icon: PencilIcon,
                          disabled: !canMutateGroupLinks || updateLinkMutation.isPending,
                          onSelect: () => openEditLink(link),
                        },
                        {
                          id: "delete",
                          label: "Delete",
                          icon: Trash2Icon,
                          destructive: true,
                          disabled: !canMutateGroupLinks || deleteLinkMutation.isPending,
                          onSelect: () => {
                            setActionError("")
                            setDeleteLinkTarget(link)
                          },
                        },
                      ]}
                    />
                  </div>
                </div>
              ))}
              {groupLinks.length === 0 ? (
                <div className="px-7 py-8 text-sm text-[#6f717c]">
                  {groupLinksQuery.isPending ? "Loading group links..." : "No links are assigned to this group."}
                </div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "Time Restrictions" ? (
          <>
            <PanelHeader title={t("kisi.groups.tabTimeRestrictions")} description={t("kisi.groups.timeDesc")} />
            <div className="p-7">
              <div className="grid min-h-[240px] grid-cols-[56px_repeat(5,minmax(90px,1fr))] overflow-hidden rounded-[6px] border border-[#eceef2]">
                <div className="border-r border-[#eceef2] bg-white pt-12 text-right text-xs font-semibold text-[#9a9ca7]">
                  {["8 AM", "12 PM", "4 PM", "8 PM"].map((time) => (
                    <div key={time} className="h-11 pr-3">
                      {time}
                    </div>
                  ))}
                </div>
                {["Mon", "Tue", "Wed", "Thu", "Fri"].map((day) => (
                  <div key={day} className="border-r border-white/80 bg-[#f1f2f5] text-center text-sm font-semibold text-[#6f717c] last:border-r-0">
                    <div className="border-b border-white/80 bg-white py-4">{day}</div>
                    <div className="mx-auto mt-16 w-full max-w-[118px] rounded-[5px] bg-[#202443] p-3 text-left text-xs text-white">
                      <p className="truncate font-semibold">{t("kisi.doors.accessPermitted")}</p>
                      <p className="mt-1 text-white/70">08:00 - 18:00</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </>
        ) : null}

        {activeTab === "Permissions" ? (
          <>
            <PanelHeader title={t("common.permissions")} description={t("kisi.groups.permDesc")} />
            <div className="divide-y divide-[#eceef2] px-7">
              {restrictions.map(({ key, title, description, enabled, icon: Icon }) => (
                <div key={key} className="flex gap-5 py-6">
                  <div
                    className={cn(
                      "flex size-12 shrink-0 items-center justify-center rounded-[6px]",
                      enabled ? "bg-[#4f55ff] text-white" : "bg-[#f1f2f5] text-[#2f3037]"
                    )}
                  >
                    <Icon className="size-6" />
                  </div>
                  <div className="min-w-0">
                    <h3 className="text-lg font-semibold text-[#17171c]">{title}</h3>
                    <p className="mt-1 max-w-3xl text-sm leading-6 text-[#6f717c]">
                      {description} <span className="text-[#4f55ff] underline underline-offset-2">{t("common.description")}</span>
                    </p>
                  </div>
                  <div className="ml-auto pt-2">
                    <button
                      type="button"
                      disabled={!canMutateGroups || updateRestrictionMutation.isPending}
                      onClick={() => updateRestrictionMutation.mutate({ [key]: !enabled })}
                      className="rounded-full disabled:opacity-60"
                      aria-pressed={enabled}
                    >
                      <ToggleSwitch enabled={enabled} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </>
        ) : null}
      </SettingsPanel>

      <ConfirmActionDialog
        open={deleteGroupConfirmOpen}
        onOpenChange={(open) => {
          if (!deleteGroupMutation.isPending) {
            setDeleteGroupConfirmOpen(open)
          }
        }}
        title="Delete group"
        description={
          <>
            This removes <span className="font-semibold text-[#17171c]">{currentGroup?.name ?? "this group"}</span> and its
            access-resource bindings from this workspace.
          </>
        }
        confirmLabel="Delete group"
        pending={deleteGroupMutation.isPending}
        disabled={!canMutateGroups || !currentGroup}
        destructive
        onConfirm={() => {
          if (currentGroup) {
            deleteGroupMutation.mutate(currentGroup.id)
          }
        }}
      />

      <ConfirmActionDialog
        open={Boolean(deleteLinkTarget)}
        onOpenChange={(open) => {
          if (!deleteLinkMutation.isPending && !open) {
            setDeleteLinkTarget(null)
          }
        }}
        title="Delete access link"
        description={
          <>
            This removes <span className="font-semibold text-[#17171c]">{deleteLinkTarget?.name ?? "this link"}</span>. Existing
            shared URLs and QR tokens for this link will stop working.
          </>
        }
        confirmLabel="Delete link"
        pending={deleteLinkMutation.isPending}
        disabled={!canMutateGroupLinks || !deleteLinkTarget}
        destructive
        onConfirm={() => {
          if (deleteLinkTarget) {
            deleteLinkMutation.mutate(deleteLinkTarget.id)
          }
        }}
      />

      <Sheet open={createGroupOpen} onOpenChange={setCreateGroupOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>{t("kisi.groups.addGroup")}</SheetTitle>
            <SheetDescription>{t("kisi.groups.createGroupDesc")}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createGroupMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.name")}</span>
              <input
                value={createGroupName}
                onChange={(event) => setCreateGroupName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.description")}</span>
              <textarea
                value={createGroupDescription}
                onChange={(event) => setCreateGroupDescription(event.target.value)}
                rows={3}
                className="w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 py-2 text-sm text-[#2f3037]"
              />
            </label>
            {placeScoped ? (
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.place")}</span>
                <input
                  value={placeContext.place?.name ?? "Assigned Place"}
                  readOnly
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-[#fbfbfc] px-3 text-sm text-[#2f3037]"
                />
              </label>
            ) : (
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.place")}</span>
                <select
                  value={createGroupPlaceID}
                  onChange={(event) => setCreateGroupPlaceID(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                >
                  <option value="">{t("kisi.groups.orgWide")}</option>
                  {placeRows.map((place) => (
                    <option key={place.id} value={place.id}>
                      {place.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={!canCreateGroups || !createGroupName.trim() || createGroupMutation.isPending}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
              >
                {createGroupMutation.isPending ? "Creating..." : "Create Group"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={addDoorsOpen} onOpenChange={setAddDoorsOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>{t("kisi.groups.addDoors")}</SheetTitle>
            <SheetDescription>{t("kisi.groups.addDoorsDesc")}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              addDoorMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.group")}</span>
              <input
                value={currentGroup?.name ?? "No group selected"}
                readOnly
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-[#fbfbfc] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.door")}</span>
              <select
                value={selectedDoorID}
                onChange={(event) => setSelectedDoorID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                {availableDoorRows.map((door) => (
                  <option key={door.id} value={door.id}>
                    {door.name} · {door.floorName}
                  </option>
                ))}
              </select>
            </label>
            {availableDoorRows.length === 0 ? <p className="text-sm text-[#8a5a00]">{t("kisi.groups.noDoors")}</p> : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutateGroupLocks || addDoorMutation.isPending || availableDoorRows.length === 0}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
              >
                {addDoorMutation.isPending ? "Adding..." : "Add Doors"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={createLinkOpen} onOpenChange={setCreateLinkOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>{t("kisi.groups.addLink")}</SheetTitle>
            <SheetDescription>{t("kisi.groups.addLinkDesc")}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createLinkMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.group")}</span>
              <input
                value={currentGroup?.name ?? "No group selected"}
                readOnly
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-[#fbfbfc] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.name")}</span>
              <input
                value={createLinkName}
                onChange={(event) => setCreateLinkName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.email")}</span>
              <input
                value={createLinkEmail}
                onChange={(event) => setCreateLinkEmail(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.validUntil")}</span>
              <input
                value={createLinkValidUntil}
                placeholder="2026-05-01T10:00:00Z"
                onChange={(event) => setCreateLinkValidUntil(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">QR code</span>
              <select
                value={createLinkQRCodeType}
                onChange={(event) => setCreateLinkQRCodeType(event.target.value as "online" | "offline" | "")}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                <option value="online">{t("common.online")}</option>
                <option value="offline">{t("common.offline")}</option>
                <option value="">-</option>
              </select>
            </label>
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutateGroupLinks || !createLinkName.trim() || createLinkMutation.isPending}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
              >
                {createLinkMutation.isPending ? "Creating..." : "Create Link"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet
        open={editLinkOpen}
        onOpenChange={(open) => {
          setEditLinkOpen(open)
          if (!open) {
            setEditingLinkID("")
          }
        }}
      >
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>{t("kisi.groups.editLink")}</SheetTitle>
            <SheetDescription>{t("kisi.groups.editLinkDesc")}</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              updateLinkMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.group")}</span>
              <input
                value={currentGroup?.name ?? "No group selected"}
                readOnly
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-[#fbfbfc] px-3 text-sm text-[#2f3037]"
              />
            </label>
            <button
              type="button"
              onClick={() => setEditLinkEnabled((enabled) => !enabled)}
              className="flex w-full items-center justify-between rounded-[6px] border border-[#d9dbe3] bg-white px-3 py-3 text-left"
            >
              <span>
                <span className="block text-sm font-semibold text-[#17171c]">{t("kisi.groups.linkEnabled")}</span>
                <span className="mt-1 block text-xs text-[#6f717c]">{t("kisi.groups.linkEnabled")}</span>
              </span>
              <ToggleSwitch enabled={editLinkEnabled} />
            </button>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.name")}</span>
              <input
                value={editLinkName}
                onChange={(event) => setEditLinkName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.email")}</span>
              <input
                value={editLinkEmail}
                onChange={(event) => setEditLinkEmail(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{t("common.validUntil")}</span>
              <input
                value={editLinkValidUntil}
                placeholder="2026-05-01T10:00:00Z"
                onChange={(event) => setEditLinkValidUntil(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">QR code</span>
              <select
                value={editLinkQRCodeType}
                onChange={(event) => setEditLinkQRCodeType(event.target.value as "online" | "offline" | "")}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
              >
                <option value="online">{t("common.online")}</option>
                <option value="offline">{t("common.offline")}</option>
                <option value="">-</option>
              </select>
            </label>
            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutateGroupLinks || !editLinkName.trim() || !editingLinkID || updateLinkMutation.isPending}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
              >
                {updateLinkMutation.isPending ? "Saving..." : "Save Link"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </PageFrame>
  )
}
