import { useEffect, useState } from "react"
import i18next from "i18next"
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
  createGroupElevatorStop,
  createGroupLink,
  createGroupLock,
  createGroupTerminal,
  deleteGroup,
  deleteGroupElevatorStop,
  deleteGroupLink,
  deleteGroupLock,
  deleteGroupTerminal,
  listElevatorStops,
  listGroupElevatorStops,
  listGroupLinks,
  listGroupTerminals,
  listTerminals,
  updateGroup,
  updateGroupLink,
  type CurrentUser,
  type ElevatorStop,
  type GroupElevatorStop,
  type GroupLink,
  type GroupTerminal,
  type Terminal,
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
  const groupTabKeys = ["general", "members", "doors", "zones", "elevator_stops", "terminals", "links", "time_restrictions", "permissions"] as const
  type GroupTabKey = (typeof groupTabKeys)[number]
  const groupTabLabels: Record<GroupTabKey, string> = {
    general: t("kisi.groups.tabGeneral"),
    members: t("kisi.groups.tabMembers"),
    doors: t("kisi.groups.tabDoors"),
    zones: t("kisi.groups.tabZones"),
    elevator_stops: "Elevator Stops",
    terminals: "Terminals",
    links: t("kisi.groups.tabLinks"),
    time_restrictions: t("kisi.groups.tabTimeRestrictions"),
    permissions: t("kisi.groups.tabPermissions"),
  }
  const [activeTab, setActiveTab] = useState<GroupTabKey>("permissions")
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
  const [addElevatorStopOpen, setAddElevatorStopOpen] = useState(false)
  const [selectedElevatorStopID, setSelectedElevatorStopID] = useState("")
  const [addTerminalOpen, setAddTerminalOpen] = useState(false)
  const [selectedTerminalID, setSelectedTerminalID] = useState("")
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
  const groupElevatorStopsQuery = useQuery({
    queryKey: ["reference-group-elevator-stops", tenantID ?? "none", currentGroup?.id ?? "none"],
    queryFn: () => listGroupElevatorStops(token, tenantID ?? undefined, currentGroup?.id),
    enabled: Boolean(tenantID && currentGroup?.id && !resourceQuery.usingFallback),
    staleTime: 30 * 1000,
  })
  const groupElevatorStops: GroupElevatorStop[] = groupElevatorStopsQuery.data?.items ?? []
  const allElevatorStopsQuery = useQuery({
    queryKey: ["reference-all-elevator-stops", tenantID ?? "none"],
    queryFn: () => listElevatorStops(token, tenantID ?? undefined),
    enabled: Boolean(tenantID && !resourceQuery.usingFallback),
    staleTime: 30 * 1000,
  })
  const allElevatorStops: ElevatorStop[] = allElevatorStopsQuery.data?.items ?? []
  const assignedElevatorStopIDs = new Set(groupElevatorStops.map((item) => item.elevator_stop_id))
  const availableElevatorStops = allElevatorStops.filter((item) => !assignedElevatorStopIDs.has(item.id))
  const groupTerminalsQuery = useQuery({
    queryKey: ["reference-group-terminals", tenantID ?? "none", currentGroup?.id ?? "none"],
    queryFn: () => listGroupTerminals(token, tenantID ?? undefined, currentGroup?.id),
    enabled: Boolean(tenantID && currentGroup?.id && !resourceQuery.usingFallback),
    staleTime: 30 * 1000,
  })
  const groupTerminals: GroupTerminal[] = groupTerminalsQuery.data?.items ?? []
  const allTerminalsQuery = useQuery({
    queryKey: ["reference-all-terminals", tenantID ?? "none"],
    queryFn: () => listTerminals(token, { tenant_id: tenantID ?? undefined }),
    enabled: Boolean(tenantID && !resourceQuery.usingFallback),
    staleTime: 30 * 1000,
  })
  const allTerminals: Terminal[] = allTerminalsQuery.data ?? []
  const assignedTerminalIDs = new Set(groupTerminals.map((item) => item.terminal_id))
  const availableTerminals = allTerminals.filter((item) => !assignedTerminalIDs.has(item.id))
  const restrictions = [
    {
      key: "primary_device_restriction_enabled",
      title: i18next.t("kisi.groups.restrictions"),
      description: i18next.t("kisi.groups.permDesc"),
      enabled: Boolean(currentGroup?.primaryDeviceRestrictionEnabled),
      icon: ShieldCheckIcon,
    },
    {
      key: "login_enabled",
      title: i18next.t("common.permissions"),
      description: i18next.t("kisi.groups.permDesc"),
      enabled: currentGroup?.loginEnabled ?? true,
      icon: CreditCardIcon,
    },
    {
      key: "geofence_restriction_enabled",
      title: i18next.t("kisi.groups.restrictions"),
      description: `Require users to be near the place location${currentGroup?.geofenceRestrictionRadius ? ` within ${currentGroup.geofenceRestrictionRadius}m` : ""}.`,
      enabled: Boolean(currentGroup?.geofenceRestrictionEnabled),
      icon: MapPinPlusIcon,
    },
    {
      key: "reader_restriction_enabled",
      title: i18next.t("kisi.groups.restrictions"),
      description: i18next.t("kisi.groups.permDesc"),
      enabled: Boolean(currentGroup?.readerRestrictionEnabled),
      icon: DoorOpenIcon,
    },
    {
      key: "tap_to_access_restriction_enabled",
      title: i18next.t("kisi.groups.restrictions"),
      description: i18next.t("kisi.groups.permDesc"),
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

  const addElevatorStopMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) throw new Error("tenant_id is required")
      if (!currentGroup) throw new Error("group is required")
      const stopID = selectedElevatorStopID.trim()
      if (!stopID) throw new Error("elevator stop is required")
      return createGroupElevatorStop(token, { tenant_id: tenantID, group_id: currentGroup.id, elevator_stop_id: stopID })
    },
    onSuccess: async () => {
      setAddElevatorStopOpen(false)
      setSelectedElevatorStopID("")
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-group-elevator-stops"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Elevator stop assignment failed"),
  })

  const removeElevatorStopMutation = useMutation({
    mutationFn: (groupElevatorStopID: string) => {
      if (!tenantID) throw new Error("tenant_id is required")
      return deleteGroupElevatorStop(token, groupElevatorStopID, tenantID)
    },
    onSuccess: async () => {
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-group-elevator-stops"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Elevator stop removal failed"),
  })

  const addTerminalMutation = useMutation({
    mutationFn: () => {
      if (!tenantID) throw new Error("tenant_id is required")
      if (!currentGroup) throw new Error("group is required")
      const termID = selectedTerminalID.trim()
      if (!termID) throw new Error("terminal is required")
      return createGroupTerminal(token, { tenant_id: tenantID, group_id: currentGroup.id, terminal_id: termID })
    },
    onSuccess: async () => {
      setAddTerminalOpen(false)
      setSelectedTerminalID("")
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-group-terminals"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Terminal assignment failed"),
  })

  const removeTerminalMutation = useMutation({
    mutationFn: (groupTerminalID: string) => {
      if (!tenantID) throw new Error("tenant_id is required")
      return deleteGroupTerminal(token, groupTerminalID, tenantID)
    },
    onSuccess: async () => {
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["reference-group-terminals"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Terminal removal failed"),
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
            className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover"
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
            className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
          >
            <Trash2Icon className="mr-1.5 size-4" />
            Delete Group
          </Button>
        </>
      }
    >
      {resourceQuery.usingFallback ? (
        <div className="mp-alert-warning">
          Live group resources are unavailable. Showing reference data.
        </div>
      ) : null}
      {actionError ? (
        <div className="mp-alert-warning">
          {actionError}
        </div>
      ) : null}

      <section className="overflow-hidden rounded-[6px] border border-line-default bg-white">
        <div className="border-b border-line-subtle px-6 py-5">
          <h2 className="text-base font-semibold text-content-heading">{placeScoped ? "Place Groups" : "Organization Groups"}</h2>
          <p className="mt-1 text-sm text-content-subtle">{t("kisi.groups.pageDesc")}</p>
        </div>
        <div className="overflow-x-auto">
          <table className={cn("mp-table", "min-w-[760px]")}>
            <thead className="bg-surface-page">
              <tr className="border-b border-line-subtle">
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
                    "mp-table-row cursor-pointer",
                    currentGroup?.id === group.id ? "bg-[#f7f7ff]" : ""
                  )}
                >
                  <td className="px-6 py-4 font-semibold text-brand">{group.name}</td>
                  <td className="px-4 py-4 text-content-body">{group.kind}</td>
                  <td className="px-4 py-4 text-content-subtle">{group.targetLabel}</td>
                  <td className="px-4 py-4 text-content-body">{group.memberCount}</td>
                  <td className="px-4 py-4">
                    <StatusDot tone={group.tone} label={group.statusLabel} />
                  </td>
                </tr>
              ))}
              {groupRows.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-6 py-10 text-center text-sm text-content-subtle">
                    No groups found for this scope.
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      </section>

      <SettingsPanel
        tabs={groupTabKeys.map((k) => groupTabLabels[k])}
        active={groupTabLabels[activeTab]}
        onTabChange={(label) => {
          const key = groupTabKeys.find((k) => groupTabLabels[k] === label)
          if (key) setActiveTab(key)
        }}
        footer={
          <Button
            type="button"
            disabled={!canMutateGroups || !groupFormDirty || !editGroupName.trim() || updateGroupMutation.isPending}
            onClick={() => updateGroupMutation.mutate()}
            className="h-10 rounded-[8px] bg-brand px-8 text-white hover:bg-brand-hover disabled:bg-[#eef0f4] disabled:text-[#8d909b]"
          >
            {updateGroupMutation.isPending ? "Saving..." : "Save"}
          </Button>
        }
      >
        {activeTab === "general" ? (
          <>
            <PanelHeader title={t("common.general")} description={t("kisi.groups.generalDesc")} />
            <div className="grid gap-6 p-7 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("kisi.groups.groupName")}</span>
                <input
                  value={editGroupName}
                  disabled={!canMutateGroups}
                  onChange={(event) => setEditGroupName(event.target.value)}
                  className="h-12 w-full rounded-[6px] border border-line-default bg-white px-4 text-sm text-content-body disabled:bg-surface-page disabled:text-content-subtle"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.description")}</span>
                <input
                  value={editGroupDescription}
                  disabled={!canMutateGroups}
                  onChange={(event) => setEditGroupDescription(event.target.value)}
                  className="h-12 w-full rounded-[6px] border border-line-default bg-white px-4 text-sm text-content-body disabled:bg-surface-page disabled:text-content-subtle"
                />
              </label>
              <FormField label="Access enabled" value={currentGroup?.statusLabel ?? "Unavailable"} trailing={<ToggleSwitch enabled={currentGroup?.tone !== "danger"} />} />
              <FormField label={t("kisi.groups.defaultRole")} value="Basic" trailing={<ChevronDownIcon className="size-4 text-content-subtle" />} />
              <FormField label={t("kisi.groups.placeScope")} value={placeScoped ? placeContext.place?.name ?? "Assigned Place" : "All organization places"} />
              <FormField label={t("kisi.teams.accessRights")} value={`${currentAccessRights.length} access rights`} />
            </div>
          </>
        ) : null}

        {activeTab === "members" ? (
          <>
            <PanelHeader
              title={t("common.members")}
              description={t("kisi.groups.membersDesc")}
              action={
                <Button variant="outline" className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover">
                  Add Members
                </Button>
              }
            />
            <div className="divide-y divide-line-subtle">
              {memberRows.map((row) => (
                <div key={row.id} className="grid gap-3 px-7 py-5 md:grid-cols-[220px_150px_1fr] md:items-center">
                  <span className="font-semibold text-content-heading">{row.name}</span>
                  <span className="text-sm text-content-body">{row.role}</span>
                  <span className="text-sm text-content-subtle">{row.email}</span>
                </div>
              ))}
              {memberRows.length === 0 ? <div className="px-7 py-8 text-sm text-content-subtle">{t("kisi.groups.noMembers")}</div> : null}
            </div>
          </>
        ) : null}

        {activeTab === "doors" ? (
          <>
            <PanelHeader
              title={t("kisi.groups.doors")}
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
                  className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
                >
                  <PlusIcon className="mr-1.5 size-4" />
                  Add Doors
                </Button>
              }
            />
            <div className="divide-y divide-line-subtle">
              {doorRows.map((row) => (
                <div key={row.id} className="grid gap-3 px-7 py-5 md:grid-cols-[minmax(180px,1fr)_180px_140px_100px] md:items-center">
                  <span className="font-semibold text-brand">{row.name}</span>
                  <span className="text-sm text-content-body">{row.floorName}</span>
                  <StatusDot tone={row.status === "online" ? "success" : "warning"} label={row.status === "online" ? t("common.online") : t("common.status")} />
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    disabled={!canMutateGroupLocks || removeDoorMutation.isPending}
                    onClick={() => removeDoorMutation.mutate(row.id)}
                    className="justify-self-start rounded-[6px] text-content-subtle md:justify-self-end"
                  >
                    Remove
                  </Button>
                </div>
              ))}
              {doorRows.length === 0 ? <div className="px-7 py-8 text-sm text-content-subtle">{t("kisi.groups.noDoors")}</div> : null}
            </div>
          </>
        ) : null}

        {activeTab === "zones" ? (
          <>
            <PanelHeader title={t("kisi.groups.zones")} description={t("kisi.groups.zonesDesc")} />
            <div className="grid gap-4 p-7 md:grid-cols-2">
              {zoneRows.map((row) => (
                <div key={row.id} className="rounded-[6px] border border-line-subtle p-5">
                  <LayersIcon className="size-6 text-content-subtle" />
                  <h3 className="mt-5 font-semibold text-content-heading">{row.name}</h3>
                  <p className="mt-1 text-sm text-content-subtle">{row.floorName} · {row.description}</p>
                  <p className="mt-4 text-sm font-semibold text-content-body">{row.doorCount} doors</p>
                </div>
              ))}
              {zoneRows.length === 0 ? <div className="text-sm text-content-subtle">{t("kisi.groups.noZones")}</div> : null}
            </div>
          </>
        ) : null}

        {activeTab === "elevator_stops" ? (
          <>
            <PanelHeader
              title="Elevator Stops"
              description="Elevator floor stops this group can access."
              action={
                <Button
                  type="button"
                  variant="outline"
                  disabled={!canMutateGroups || availableElevatorStops.length === 0}
                  onClick={() => {
                    setSelectedElevatorStopID(availableElevatorStops[0]?.id ?? "")
                    setActionError("")
                    setAddElevatorStopOpen(true)
                  }}
                  className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
                >
                  <PlusIcon className="mr-1.5 size-4" />
                  Add Elevator Stop
                </Button>
              }
            />
            <div className="divide-y divide-line-subtle">
              {groupElevatorStops.map((row) => {
                const stop = allElevatorStops.find((s) => s.id === row.elevator_stop_id)
                return (
                  <div key={row.id} className="grid gap-3 px-7 py-5 md:grid-cols-[minmax(180px,1fr)_180px_140px_100px] md:items-center">
                    <span className="font-semibold text-brand">{stop?.name ?? row.elevator_stop_id}</span>
                    <span className="text-sm text-content-body">{stop?.status ?? "unknown"}</span>
                    <StatusDot tone={stop?.status === "online" ? "success" : "warning"} label={stop?.status ?? "unknown"} />
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      disabled={!canMutateGroups || removeElevatorStopMutation.isPending}
                      onClick={() => removeElevatorStopMutation.mutate(row.id)}
                      className="justify-self-start rounded-[6px] text-content-subtle md:justify-self-end"
                    >
                      Remove
                    </Button>
                  </div>
                )
              })}
              {groupElevatorStops.length === 0 ? (
                <div className="px-7 py-8 text-sm text-content-subtle">
                  {groupElevatorStopsQuery.isPending ? "Loading elevator stops..." : "No elevator stops are assigned to this group."}
                </div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "terminals" ? (
          <>
            <PanelHeader
              title="Terminals"
              description="Terminals assigned to this group."
              action={
                <Button
                  type="button"
                  variant="outline"
                  disabled={!canMutateGroups || availableTerminals.length === 0}
                  onClick={() => {
                    setSelectedTerminalID(availableTerminals[0]?.id ?? "")
                    setActionError("")
                    setAddTerminalOpen(true)
                  }}
                  className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
                >
                  <PlusIcon className="mr-1.5 size-4" />
                  Add Terminal
                </Button>
              }
            />
            <div className="divide-y divide-line-subtle">
              {groupTerminals.map((row) => {
                const terminal = allTerminals.find((t) => t.id === row.terminal_id)
                return (
                  <div key={row.id} className="grid gap-3 px-7 py-5 md:grid-cols-[minmax(180px,1fr)_180px_140px_100px] md:items-center">
                    <span className="font-semibold text-brand">{terminal?.name ?? row.terminal_id}</span>
                    <span className="text-sm text-content-body">{terminal?.description ?? ""}</span>
                    <StatusDot tone={terminal?.status === "online" ? "success" : "warning"} label={terminal?.status ?? "unknown"} />
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      disabled={!canMutateGroups || removeTerminalMutation.isPending}
                      onClick={() => removeTerminalMutation.mutate(row.id)}
                      className="justify-self-start rounded-[6px] text-content-subtle md:justify-self-end"
                    >
                      Remove
                    </Button>
                  </div>
                )
              })}
              {groupTerminals.length === 0 ? (
                <div className="px-7 py-8 text-sm text-content-subtle">
                  {groupTerminalsQuery.isPending ? "Loading terminals..." : "No terminals are assigned to this group."}
                </div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "links" ? (
          <>
            <PanelHeader
              title={t("kisi.groups.links")}
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
                  className="h-10 rounded-[6px] border-brand-ring bg-white px-6 text-brand hover:border-brand-hover hover:bg-brand-subtle hover:text-brand-hover"
                >
                  <PlusIcon className="mr-1.5 size-4" />
                  Add Link
                </Button>
              }
            />
            <div className="divide-y divide-line-subtle">
              {groupLinks.map((link) => (
                <div key={link.id} className="grid gap-3 px-7 py-5 md:grid-cols-[minmax(180px,1fr)_minmax(180px,1fr)_140px_190px] md:items-center">
                  <div className="min-w-0">
                    <span className="block truncate font-semibold text-content-heading">{link.name}</span>
                    <span className="mt-1 block truncate text-sm text-content-subtle">{link.email || link.phone || "Reusable link"}</span>
                  </div>
                  <span className="text-sm text-content-body">{link.valid_until ? `Expires ${link.valid_until}` : "No expiry"}</span>
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
                <div className="px-7 py-8 text-sm text-content-subtle">
                  {groupLinksQuery.isPending ? "Loading group links..." : "No links are assigned to this group."}
                </div>
              ) : null}
            </div>
          </>
        ) : null}

        {activeTab === "time_restrictions" ? (
          <>
            <PanelHeader title={t("kisi.groups.tabTimeRestrictions")} description={t("kisi.groups.timeDesc")} />
            <div className="p-7">
              <div className="grid min-h-[240px] grid-cols-[56px_repeat(5,minmax(90px,1fr))] overflow-hidden rounded-[6px] border border-line-subtle">
                <div className="border-r border-line-subtle bg-white pt-12 text-right text-xs font-semibold text-content-muted">
                  {["8 AM", "12 PM", "4 PM", "8 PM"].map((time) => (
                    <div key={time} className="h-11 pr-3">
                      {time}
                    </div>
                  ))}
                </div>
                {["Mon", "Tue", "Wed", "Thu", "Fri"].map((day) => (
                  <div key={day} className="border-r border-white/80 bg-surface-sunken text-center text-sm font-semibold text-content-subtle last:border-r-0">
                    <div className="border-b border-white/80 bg-white py-4">{day}</div>
                    <div className="mx-auto mt-16 w-full max-w-[118px] rounded-[5px] bg-[#17171c] p-3 text-left text-xs text-white">
                      <p className="truncate font-semibold">{t("kisi.doors.accessPermitted")}</p>
                      <p className="mt-1 text-white/70">08:00 - 18:00</p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </>
        ) : null}

        {activeTab === "permissions" ? (
          <>
            <PanelHeader title={t("common.permissions")} description={t("kisi.groups.permDesc")} />
            <div className="divide-y divide-line-subtle px-7">
              {restrictions.map(({ key, title, description, enabled, icon: Icon }) => (
                <div key={key} className="flex gap-5 py-6">
                  <div
                    className={cn(
                      "flex size-12 shrink-0 items-center justify-center rounded-[6px]",
                      enabled ? "bg-brand text-white" : "bg-surface-sunken text-content-body"
                    )}
                  >
                    <Icon className="size-6" />
                  </div>
                  <div className="min-w-0">
                    <h3 className="text-lg font-semibold text-content-heading">{title}</h3>
                    <p className="mt-1 max-w-3xl text-sm leading-6 text-content-subtle">
                      {description} <span className="text-brand underline underline-offset-2">{t("common.description")}</span>
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
        title={t("kisi.groups.deleteGroup")}
        description={
          <>
            This removes <span className="font-semibold text-content-heading">{currentGroup?.name ?? "this group"}</span> and its
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
        title={t("common.delete")}
        description={
          <>
            This removes <span className="font-semibold text-content-heading">{deleteLinkTarget?.name ?? "this link"}</span>. Existing
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
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
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
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.name")}</span>
              <input
                value={createGroupName}
                onChange={(event) => setCreateGroupName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.description")}</span>
              <textarea
                value={createGroupDescription}
                onChange={(event) => setCreateGroupDescription(event.target.value)}
                rows={3}
                className="w-full rounded-[6px] border border-line-default bg-white px-3 py-2 text-sm text-content-body"
              />
            </label>
            {placeScoped ? (
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.place")}</span>
                <input
                  value={placeContext.place?.name ?? "Assigned Place"}
                  readOnly
                  className="h-11 w-full rounded-[6px] border border-line-default bg-surface-page px-3 text-sm text-content-body"
                />
              </label>
            ) : (
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.place")}</span>
                <select
                  value={createGroupPlaceID}
                  onChange={(event) => setCreateGroupPlaceID(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
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
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canCreateGroups || !createGroupName.trim() || createGroupMutation.isPending}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover"
              >
                {createGroupMutation.isPending ? "Creating..." : "Create Group"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={addDoorsOpen} onOpenChange={setAddDoorsOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
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
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.group")}</span>
              <input
                value={currentGroup?.name ?? "No group selected"}
                readOnly
                className="h-11 w-full rounded-[6px] border border-line-default bg-surface-page px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.door")}</span>
              <select
                value={selectedDoorID}
                onChange={(event) => setSelectedDoorID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                {availableDoorRows.map((door) => (
                  <option key={door.id} value={door.id}>
                    {door.name} · {door.floorName}
                  </option>
                ))}
              </select>
            </label>
            {availableDoorRows.length === 0 ? <p className="text-sm text-warning-text">{t("kisi.groups.noDoors")}</p> : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutateGroupLocks || addDoorMutation.isPending || availableDoorRows.length === 0}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover"
              >
                {addDoorMutation.isPending ? "Adding..." : "Add Doors"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={createLinkOpen} onOpenChange={setCreateLinkOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
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
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.group")}</span>
              <input
                value={currentGroup?.name ?? "No group selected"}
                readOnly
                className="h-11 w-full rounded-[6px] border border-line-default bg-surface-page px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.name")}</span>
              <input
                value={createLinkName}
                onChange={(event) => setCreateLinkName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.email")}</span>
              <input
                value={createLinkEmail}
                onChange={(event) => setCreateLinkEmail(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.validUntil")}</span>
              <input
                value={createLinkValidUntil}
                placeholder="2026-05-01T10:00:00Z"
                onChange={(event) => setCreateLinkValidUntil(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">QR code</span>
              <select
                value={createLinkQRCodeType}
                onChange={(event) => setCreateLinkQRCodeType(event.target.value as "online" | "offline" | "")}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                <option value="online">{t("common.online")}</option>
                <option value="offline">{t("common.offline")}</option>
                <option value="">-</option>
              </select>
            </label>
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutateGroupLinks || !createLinkName.trim() || createLinkMutation.isPending}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover"
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
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
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
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.group")}</span>
              <input
                value={currentGroup?.name ?? "No group selected"}
                readOnly
                className="h-11 w-full rounded-[6px] border border-line-default bg-surface-page px-3 text-sm text-content-body"
              />
            </label>
            <button
              type="button"
              onClick={() => setEditLinkEnabled((enabled) => !enabled)}
              className="flex w-full items-center justify-between rounded-[6px] border border-line-default bg-white px-3 py-3 text-left"
            >
              <span>
                <span className="block text-sm font-semibold text-content-heading">{t("kisi.groups.linkEnabled")}</span>
                <span className="mt-1 block text-xs text-content-subtle">{t("kisi.groups.linkEnabled")}</span>
              </span>
              <ToggleSwitch enabled={editLinkEnabled} />
            </button>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.name")}</span>
              <input
                value={editLinkName}
                onChange={(event) => setEditLinkName(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.email")}</span>
              <input
                value={editLinkEmail}
                onChange={(event) => setEditLinkEmail(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.validUntil")}</span>
              <input
                value={editLinkValidUntil}
                placeholder="2026-05-01T10:00:00Z"
                onChange={(event) => setEditLinkValidUntil(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">QR code</span>
              <select
                value={editLinkQRCodeType}
                onChange={(event) => setEditLinkQRCodeType(event.target.value as "online" | "offline" | "")}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                <option value="online">{t("common.online")}</option>
                <option value="offline">{t("common.offline")}</option>
                <option value="">-</option>
              </select>
            </label>
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutateGroupLinks || !editLinkName.trim() || !editingLinkID || updateLinkMutation.isPending}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover"
              >
                {updateLinkMutation.isPending ? "Saving..." : "Save Link"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={addElevatorStopOpen} onOpenChange={setAddElevatorStopOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>Add Elevator Stop</SheetTitle>
            <SheetDescription>Assign an elevator stop to this group.</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              addElevatorStopMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.group")}</span>
              <input
                value={currentGroup?.name ?? "No group selected"}
                readOnly
                className="h-11 w-full rounded-[6px] border border-line-default bg-surface-page px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Elevator Stop</span>
              <select
                value={selectedElevatorStopID}
                onChange={(event) => setSelectedElevatorStopID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                {availableElevatorStops.map((stop) => (
                  <option key={stop.id} value={stop.id}>
                    {stop.name}
                  </option>
                ))}
              </select>
            </label>
            {availableElevatorStops.length === 0 ? <p className="text-sm text-warning-text">No available elevator stops to assign.</p> : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutateGroups || addElevatorStopMutation.isPending || availableElevatorStops.length === 0}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover"
              >
                {addElevatorStopMutation.isPending ? "Adding..." : "Add Elevator Stop"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={addTerminalOpen} onOpenChange={setAddTerminalOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[440px]">
          <SheetHeader className="border-b border-line-subtle px-6 py-5">
            <SheetTitle>Add Terminal</SheetTitle>
            <SheetDescription>Assign a terminal to this group.</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              addTerminalMutation.mutate()
            }}
          >
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">{t("common.group")}</span>
              <input
                value={currentGroup?.name ?? "No group selected"}
                readOnly
                className="h-11 w-full rounded-[6px] border border-line-default bg-surface-page px-3 text-sm text-content-body"
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-content-subtle">Terminal</span>
              <select
                value={selectedTerminalID}
                onChange={(event) => setSelectedTerminalID(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-line-default bg-white px-3 text-sm text-content-body"
              >
                {availableTerminals.map((term) => (
                  <option key={term.id} value={term.id}>
                    {term.name} {term.description ? `· ${term.description}` : ""}
                  </option>
                ))}
              </select>
            </label>
            {availableTerminals.length === 0 ? <p className="text-sm text-warning-text">No available terminals to assign.</p> : null}
            <SheetFooter className="-mx-6 mt-6 border-t border-line-subtle bg-surface-page px-6 py-4">
              <Button
                type="submit"
                disabled={!canMutateGroups || addTerminalMutation.isPending || availableTerminals.length === 0}
                className="h-10 rounded-[6px] bg-brand px-5 text-white hover:bg-brand-hover"
              >
                {addTerminalMutation.isPending ? "Adding..." : "Add Terminal"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </PageFrame>
  )
}
