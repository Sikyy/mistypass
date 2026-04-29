import { useEffect, useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { CalendarClockIcon, CheckIcon, EyeIcon, PencilIcon, PlusIcon, Trash2Icon } from "lucide-react"

import { ConfirmActionDialog, RowActionsMenu } from "@/components/kisi/actions"
import { KisiEmptyTableRow, KisiSearchField, KisiTablePagination } from "@/components/kisi/data-display"
import { PageFrame, StatusDot } from "@/components/kisi/primitives"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { selectKisiPlaceContext } from "@/features/kisi-shell/resource-data"
import { useKisiResourceSummary } from "@/features/kisi-shell/use-resource-summary"
import {
  createRoleAssignment,
  createShare,
  deleteRoleAssignment,
  deleteShare,
  getRoleAssignment,
  getShare,
  listAccessRightsScheduleTemplates,
  listRoles,
  previewAccessRightsImpact,
  reviewAccessRights,
  updateRoleAssignment,
  updateAccessRightsSchedule,
  updateShare,
  type AccessRightsImpactPreview,
  type AccessRightsScheduleTemplate,
  type CurrentUser,
  type Role,
  type RoleAssignment,
  type Share,
  type TimeWindow,
} from "@/lib/api"
import { cn } from "@/lib/utils"
import { getViewerTenantID } from "@/lib/viewer"

type AccessRightRow = {
  id: string
  sourceType: "role_assignment" | "share"
  name: string
  type: string
  target: string
  rule: string
  status: string
  tone: "success" | "warning" | "danger" | "info"
  reviewedAt?: string
  reviewedBy?: string
  needsReview?: boolean
}

type ShareAccessMode = "role_assignment" | "share"
type AccessRightDetail = RoleAssignment | Share
type ScheduleFilter = "all" | "enabled" | "scheduled" | "expired" | "needs_review"

function defaultValidUntil() {
  const date = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000)
  return date.toISOString().replace(/\.\d{3}Z$/, "Z")
}

function defaultRoleForScope(roles: Role[], scope: RoleAssignment["applies_to_type"]) {
  return roles.find((role) => role.applies_to === scope)?.id ?? (scope === "Group" ? "role_group_access" : scope === "Place" ? "role_place_admin" : "role_organization_admin")
}

function applyScheduleTemplate(
  templateID: string,
  templates: AccessRightsScheduleTemplate[],
  setValidFromValue: (value: string) => void,
  setValidUntilValue: (value: string) => void,
  setTimeWindows?: (value: TimeWindow[]) => void
) {
  const template = templates.find((item) => item.id === templateID)
  if (!template) {
    return
  }
  setValidFromValue(template.valid_from ?? "")
  setValidUntilValue(template.valid_until ?? "")
  if (setTimeWindows) {
    setTimeWindows(template.time_windows ?? [])
  }
}

function formatTimeWindows(windows: TimeWindow[]): string {
  if (!windows.length) return "No time restriction"
  return windows.map((tw) => {
    const days = tw.day_of_week_set === "weekday" ? "Mon–Fri" : tw.day_of_week_set === "weekend" ? "Sat–Sun" : tw.day_of_week_set === "all" ? "Every day" : tw.day_of_week_set
    return `${days} ${tw.start_time}–${tw.end_time}`
  }).join(", ")
}

function accessRightNeedsReview(row: AccessRightRow) {
  if (row.reviewedAt?.trim()) {
    return false
  }
  if (typeof row.needsReview === "boolean") {
    return row.needsReview
  }
  const normalizedStatus = row.status.trim().toLowerCase()
  return normalizedStatus !== "" && normalizedStatus !== "enabled" && normalizedStatus !== "active"
}

function accessRightMatchesSchedule(row: AccessRightRow, scheduleFilter: ScheduleFilter) {
  const normalizedStatus = row.status.trim().toLowerCase()
  if (scheduleFilter === "all") {
    return true
  }
  if (scheduleFilter === "needs_review") {
    return accessRightNeedsReview(row)
  }
  return normalizedStatus === scheduleFilter
}

function rowSelectionKey(row: AccessRightRow) {
  return `${row.sourceType}:${row.id}`
}

function accessRightPreviewTone(status: string, needsReview: boolean): AccessRightRow["tone"] {
  const normalizedStatus = status.trim().toLowerCase()
  if (normalizedStatus === "expired" || normalizedStatus === "revoked" || normalizedStatus === "disabled" || normalizedStatus === "inactive") {
    return "danger"
  }
  if (needsReview || normalizedStatus === "scheduled" || normalizedStatus === "pending") {
    return "warning"
  }
  return "success"
}

export function AccessRightsAdaptedPage({
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
  const queryClient = useQueryClient()
  const tabs = ["Groups", "Places", "Teams", "Users", "Access Links"]
  const [activeTab, setActiveTab] = useState("Places")
  const [query, setQuery] = useState("")
  const [shareAccessOpen, setShareAccessOpen] = useState(false)
  const [shareMode, setShareMode] = useState<ShareAccessMode>("role_assignment")
  const [scopeType, setScopeType] = useState<RoleAssignment["applies_to_type"]>("Place")
  const [roleID, setRoleID] = useState("role_place_admin")
  const [scopeID, setScopeID] = useState("")
  const [assigneeType, setAssigneeType] = useState<RoleAssignment["assignee_type"]>("User")
  const [assigneeID, setAssigneeID] = useState("")
  const [assigneeEmail, setAssigneeEmail] = useState("")
  const [validFrom, setValidFrom] = useState("")
  const [validUntil, setValidUntil] = useState(defaultValidUntil)
  const [scheduleTemplateID, setScheduleTemplateID] = useState("")
  const [shareEmail, setShareEmail] = useState("")
  const [shareName, setShareName] = useState("")
  const [shareGroupID, setShareGroupID] = useState("")
  const [sharePlaceID, setSharePlaceID] = useState("")
  const [shareLockID, setShareLockID] = useState("")
  const [targetFilter, setTargetFilter] = useState("all")
  const [scheduleFilter, setScheduleFilter] = useState<ScheduleFilter>("all")
  const [editOpen, setEditOpen] = useState(false)
  const [editingRow, setEditingRow] = useState<AccessRightRow | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AccessRightRow | null>(null)
  const [editRoleID, setEditRoleID] = useState("")
  const [editValidFrom, setEditValidFrom] = useState("")
  const [editValidUntil, setEditValidUntil] = useState("")
  const [editScheduleTemplateID, setEditScheduleTemplateID] = useState("")
  const [bulkScheduleOpen, setBulkScheduleOpen] = useState(false)
  const [bulkScheduleTemplateID, setBulkScheduleTemplateID] = useState("")
  const [bulkValidFrom, setBulkValidFrom] = useState("")
  const [bulkValidUntil, setBulkValidUntil] = useState(defaultValidUntil)
  const [editShareEmail, setEditShareEmail] = useState("")
  const [editShareName, setEditShareName] = useState("")
  const [editShareGroupID, setEditShareGroupID] = useState("")
  const [editSharePlaceID, setEditSharePlaceID] = useState("")
  const [editShareLockID, setEditShareLockID] = useState("")
  const [actionError, setActionError] = useState("")
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([])
  const [impactPreviewOpen, setImpactPreviewOpen] = useState(false)
  const [impactPreview, setImpactPreview] = useState<AccessRightsImpactPreview | null>(null)
  const resourceQuery = useKisiResourceSummary(token, viewer)
  const tenantID = getViewerTenantID(viewer)
  const rolesQuery = useQuery({
    queryKey: ["reference-roles"],
    queryFn: () => listRoles(token),
    staleTime: 60 * 1000,
  })
  const scheduleTemplatesQuery = useQuery({
    queryKey: ["access-rights-schedule-templates", tenantID],
    queryFn: () => listAccessRightsScheduleTemplates(token, tenantID),
    enabled: Boolean(tenantID),
    staleTime: 60 * 1000,
  })
  const placeContext = selectKisiPlaceContext(resourceQuery.summary, placeID)
  const accessRights = placeScoped ? placeContext.accessRights : resourceQuery.summary.accessRights
  const roles = rolesQuery.data ?? []
  const scheduleTemplates = scheduleTemplatesQuery.data ?? []
  const placeRows = placeScoped && placeContext.place ? [placeContext.place] : resourceQuery.summary.places
  const groupRows = placeScoped ? placeContext.groups : resourceQuery.summary.groups
  const teamRows = placeScoped ? placeContext.teams : resourceQuery.summary.teams
  const userRows = placeScoped ? placeContext.users : resourceQuery.summary.users
  const doorRows = placeScoped ? placeContext.doors : resourceQuery.summary.doors
  const roleOptions = roles.filter((role) => role.applies_to === scopeType)
  const scopeTypeOptions: RoleAssignment["applies_to_type"][] = placeScoped ? ["Place", "Group"] : ["Organization", "Place", "Group"]
  const scopeOptions =
    scopeType === "Organization"
      ? [{ id: tenantID, name: "Organization" }]
      : scopeType === "Place"
        ? placeRows
        : groupRows
  const assigneeOptions = assigneeType === "Team" ? teamRows : assigneeType === "User" ? userRows : []
  const canMutate = Boolean(tenantID && !resourceQuery.usingFallback)
  const canSubmitRoleAssignment =
    canMutate &&
    Boolean(roleID.trim()) &&
    Boolean(scopeID.trim()) &&
    (assigneeType === "Guest" ? Boolean(assigneeEmail.trim()) : Boolean(assigneeID.trim()))
  const canSubmitShare = canMutate && Boolean(shareEmail.trim()) && Boolean(validUntil.trim()) && Boolean(shareGroupID || sharePlaceID || shareLockID)
  const accessRightRows = (subjectType: AccessRightRow["type"]) =>
    accessRights
      .filter((item) => item.subjectType === subjectType)
      .map((item) => ({
        id: item.id,
        sourceType: item.sourceType ?? (item.subjectType === "Access Link" ? "share" : "role_assignment"),
        name: item.name,
        type: item.subjectType,
        target: item.targetLabel,
        rule: item.ruleLabel,
        status: item.statusLabel,
        tone: item.tone,
        reviewedAt: item.reviewedAt,
        reviewedBy: item.reviewedBy,
        needsReview: item.needsReview,
      }) satisfies AccessRightRow)
  const rowsByTab: Record<string, AccessRightRow[]> = {
    Groups: accessRightRows("Group"),
    Places: accessRightRows("Place"),
    Teams: accessRightRows("Team"),
    Users: accessRightRows("User"),
    "Access Links": accessRightRows("Access Link"),
  }
  const activeTabRows = rowsByTab[activeTab]
  const targetOptions = Array.from(new Set(activeTabRows.map((row) => row.target).filter(Boolean))).sort((left, right) => left.localeCompare(right))
  const activeTargetFilter = targetOptions.includes(targetFilter) ? targetFilter : "all"
  const reviewCount = activeTabRows.filter(accessRightNeedsReview).length
  const scheduledCount = activeTabRows.filter((row) => row.status.trim().toLowerCase() === "scheduled").length
  const expiredCount = activeTabRows.filter((row) => row.status.trim().toLowerCase() === "expired").length
  const rows = activeTabRows.filter(
    (row) =>
      (query.trim() === "" || Object.values(row).join(" ").toLowerCase().includes(query.trim().toLowerCase())) &&
      (activeTargetFilter === "all" || row.target === activeTargetFilter) &&
      accessRightMatchesSchedule(row, scheduleFilter)
  )
  const selectedRowKeySet = new Set(selectedRowKeys)
  const selectedRows = rows.filter((row) => selectedRowKeySet.has(rowSelectionKey(row)))
  const allVisibleSelected = rows.length > 0 && selectedRows.length === rows.length
  const someVisibleSelected = selectedRows.length > 0
  const editDetailQuery = useQuery<AccessRightDetail>({
    queryKey: ["access-right-detail", editingRow?.sourceType, editingRow?.id, tenantID],
    queryFn: async (): Promise<AccessRightDetail> => {
      if (!editingRow) {
        throw new Error("access right is required")
      }
      if (editingRow.sourceType === "share") {
        return await getShare(token, editingRow.id, tenantID)
      }
      return await getRoleAssignment(token, editingRow.id, tenantID)
    },
    enabled: editOpen && Boolean(editingRow?.id),
  })
  const editRoleAssignment = editingRow?.sourceType === "role_assignment" ? (editDetailQuery.data as RoleAssignment | undefined) : undefined
  const editShare = editingRow?.sourceType === "share" ? (editDetailQuery.data as Share | undefined) : undefined
  const editRoleOptions = roles.filter((role) => role.applies_to === editRoleAssignment?.applies_to_type)
  const canSubmitEdit =
    Boolean(editingRow) &&
    !editDetailQuery.isPending &&
    (editingRow?.sourceType === "share" ? Boolean(editShareEmail.trim()) && Boolean(editValidUntil.trim()) : Boolean(editRoleID.trim()))
  const canSubmitBulkSchedule = canMutate && selectedRows.length > 0 && Boolean(bulkValidUntil.trim())

  useEffect(() => {
    if (!editDetailQuery.data || !editingRow) {
      return
    }
    if (editingRow.sourceType === "share") {
      const detail = editDetailQuery.data as Share
      setEditRoleID(detail.role_id)
      setEditValidFrom(detail.valid_from ?? "")
      setEditValidUntil(detail.valid_until ?? "")
      setEditScheduleTemplateID("")
      setEditShareEmail(detail.email ?? "")
      setEditShareName(detail.grantee_name ?? "")
      setEditShareGroupID(detail.group_id ?? "")
      setEditSharePlaceID(detail.place_id ?? "")
      setEditShareLockID(detail.lock_id ?? "")
      return
    }
    const detail = editDetailQuery.data as RoleAssignment
    setEditRoleID(detail.role_id)
    setEditValidFrom(detail.valid_from ?? "")
    setEditValidUntil(detail.valid_until ?? "")
    setEditScheduleTemplateID("")
  }, [editDetailQuery.data, editingRow])

  useEffect(() => {
    setSelectedRowKeys([])
    setImpactPreview(null)
    setImpactPreviewOpen(false)
    setBulkScheduleOpen(false)
  }, [activeTab, query, activeTargetFilter, scheduleFilter, tenantID])

  function resetShareAccessForm(mode: ShareAccessMode = "role_assignment") {
    const nextScopeType: RoleAssignment["applies_to_type"] = placeScoped ? "Place" : "Place"
    setShareMode(mode)
    setScopeType(nextScopeType)
    setRoleID(defaultRoleForScope(roles, nextScopeType))
    setScopeID((placeScoped ? placeContext.place?.id : placeRows[0]?.id) ?? "")
    setAssigneeType("User")
    setAssigneeID(userRows[0]?.id ?? "")
    setAssigneeEmail(userRows[0]?.email ?? "")
    setValidFrom("")
    setValidUntil(defaultValidUntil())
    setScheduleTemplateID("")
    setShareEmail("")
    setShareName("")
    setShareGroupID(groupRows[0]?.id ?? "")
    setSharePlaceID((placeScoped ? placeContext.place?.id : placeRows[0]?.id) ?? "")
    setShareLockID("")
    setActionError("")
  }

  function updateScheduleTemplate(templateID: string) {
    setScheduleTemplateID(templateID)
    applyScheduleTemplate(templateID, scheduleTemplates, setValidFrom, setValidUntil)
  }

  function updateEditScheduleTemplate(templateID: string) {
    setEditScheduleTemplateID(templateID)
    applyScheduleTemplate(templateID, scheduleTemplates, setEditValidFrom, setEditValidUntil)
  }

  function updateBulkScheduleTemplate(templateID: string) {
    setBulkScheduleTemplateID(templateID)
    applyScheduleTemplate(templateID, scheduleTemplates, setBulkValidFrom, setBulkValidUntil)
  }

  function openBulkScheduleSheet() {
    setBulkScheduleTemplateID("")
    setBulkValidFrom("")
    setBulkValidUntil(defaultValidUntil())
    setActionError("")
    setBulkScheduleOpen(true)
  }

  function updateScopeType(nextScopeType: RoleAssignment["applies_to_type"]) {
    setScopeType(nextScopeType)
    setRoleID(defaultRoleForScope(roles, nextScopeType))
    if (nextScopeType === "Organization") {
      setScopeID(tenantID)
      return
    }
    if (nextScopeType === "Place") {
      setScopeID((placeScoped ? placeContext.place?.id : placeRows[0]?.id) ?? "")
      return
    }
    setScopeID(groupRows[0]?.id ?? "")
  }

  function updateAssigneeType(nextAssigneeType: RoleAssignment["assignee_type"]) {
    setAssigneeType(nextAssigneeType)
    if (nextAssigneeType === "User") {
      setAssigneeID(userRows[0]?.id ?? "")
      setAssigneeEmail(userRows[0]?.email ?? "")
      return
    }
    if (nextAssigneeType === "Team") {
      setAssigneeID(teamRows[0]?.id ?? "")
      setAssigneeEmail("")
      return
    }
    setAssigneeID("")
    setAssigneeEmail("")
  }

  function updateAssigneeID(nextAssigneeID: string) {
    setAssigneeID(nextAssigneeID)
    if (assigneeType === "User") {
      setAssigneeEmail(userRows.find((user) => user.id === nextAssigneeID)?.email ?? "")
    }
  }

  function openEditSheet(row: AccessRightRow) {
    setEditingRow(row)
    setEditRoleID("")
    setEditValidFrom("")
    setEditValidUntil("")
    setEditScheduleTemplateID("")
    setEditShareEmail("")
    setEditShareName("")
    setEditShareGroupID("")
    setEditSharePlaceID("")
    setEditShareLockID("")
    setActionError("")
    setEditOpen(true)
  }

  function buildAccessRightsSelectionPayload() {
    if (!tenantID) {
      throw new Error("tenant_id is required")
    }
    const roleAssignmentIDs = selectedRows.filter((row) => row.sourceType === "role_assignment").map((row) => row.id)
    const shareIDs = selectedRows.filter((row) => row.sourceType === "share").map((row) => row.id)
    if (roleAssignmentIDs.length === 0 && shareIDs.length === 0) {
      throw new Error("select access rights to review")
    }
    return {
      tenant_id: tenantID,
      role_assignment_ids: roleAssignmentIDs,
      share_ids: shareIDs,
    }
  }

  function toggleVisibleRows(checked: boolean) {
    setSelectedRowKeys(checked ? rows.map(rowSelectionKey) : [])
  }

  function toggleSelectedRow(row: AccessRightRow, checked: boolean) {
    const key = rowSelectionKey(row)
    setSelectedRowKeys((current) => {
      if (checked) {
        return current.includes(key) ? current : [...current, key]
      }
      return current.filter((item) => item !== key)
    })
  }

  const createAccessRightMutation = useMutation({
    mutationFn: async (): Promise<RoleAssignment | Share> => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (shareMode === "share") {
        const email = shareEmail.trim()
        if (!email) {
          throw new Error("email is required")
        }
        const nextValidUntil = validUntil.trim()
        if (!nextValidUntil) {
          throw new Error("valid_until is required")
        }
        return createShare(token, {
          tenant_id: tenantID,
          email,
          grantee_name: shareName.trim() || email,
          grantee_phone: "not_provided",
          group_id: shareGroupID || undefined,
          role_id: "role_group_access",
          place_id: sharePlaceID || undefined,
          lock_id: shareLockID || undefined,
          valid_until: nextValidUntil,
          valid_from: validFrom.trim() || undefined,
          delivery_method: "email_qr",
          pass_type: "share",
        })
      }
      const nextScopeID = scopeID.trim()
      if (!nextScopeID) {
        throw new Error("scope is required")
      }
      const nextAssigneeID = assigneeID.trim() || assigneeEmail.trim()
      if (!nextAssigneeID) {
        throw new Error("assignee is required")
      }
      return createRoleAssignment(token, {
        tenant_id: tenantID,
        role_id: roleID || defaultRoleForScope(roles, scopeType),
        applies_to_type: scopeType,
        applies_to_id: nextScopeID,
        assignee_type: assigneeType,
        assignee_id: nextAssigneeID,
        assignee_email: assigneeEmail.trim() || undefined,
        valid_from: validFrom.trim() || undefined,
        valid_until: validUntil.trim() || undefined,
      })
    },
    onSuccess: async () => {
      setShareAccessOpen(false)
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Access right create failed"),
  })

  const deleteAccessRightMutation = useMutation({
    mutationFn: (row: AccessRightRow) => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (row.sourceType === "share") {
        return deleteShare(token, row.id, tenantID)
      }
      return deleteRoleAssignment(token, row.id, tenantID)
    },
    onSuccess: async () => {
      setDeleteTarget(null)
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Access right delete failed"),
  })

  const updateAccessRightMutation = useMutation({
    mutationFn: async () => {
      if (!tenantID) {
        throw new Error("tenant_id is required")
      }
      if (!editingRow) {
        throw new Error("access right is required")
      }
      if (editingRow.sourceType === "share") {
        if (!editShare) {
          throw new Error("access link detail is required")
        }
        return updateShare(token, editingRow.id, {
          tenant_id: tenantID,
          email: editShareEmail.trim(),
          grantee_name: editShareName.trim() || editShareEmail.trim(),
          group_id: editShareGroupID.trim() || undefined,
          role_id: editRoleID.trim() || editShare.role_id,
          place_id: editSharePlaceID.trim() || undefined,
          lock_id: editShareLockID.trim() || undefined,
          valid_from: editValidFrom.trim(),
          valid_until: editValidUntil.trim(),
          delivery_method: editShare.delivery_method,
          grantee_phone: "not_provided",
          pass_type: "share",
        })
      }
      if (!editRoleAssignment) {
        throw new Error("role assignment detail is required")
      }
      return updateRoleAssignment(token, editingRow.id, {
        tenant_id: tenantID,
        role_id: editRoleID.trim() || editRoleAssignment.role_id,
        applies_to_type: editRoleAssignment.applies_to_type,
        applies_to_id: editRoleAssignment.applies_to_id,
        assignee_type: editRoleAssignment.assignee_type,
        assignee_id: editRoleAssignment.assignee_id,
        assignee_email: editRoleAssignment.assignee_email || undefined,
        valid_from: editValidFrom.trim() || undefined,
        valid_until: editValidUntil.trim() || undefined,
      })
    },
    onSuccess: async () => {
      setEditOpen(false)
      setEditingRow(null)
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Access right update failed"),
  })

  const previewAccessRightsMutation = useMutation({
    mutationFn: () => previewAccessRightsImpact(token, buildAccessRightsSelectionPayload()),
    onSuccess: (preview) => {
      setImpactPreview(preview)
      setImpactPreviewOpen(true)
      setActionError("")
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Access rights preview failed"),
  })

  const updateAccessRightsScheduleMutation = useMutation({
    mutationFn: () => {
      const nextValidUntil = bulkValidUntil.trim()
      if (!nextValidUntil) {
        throw new Error("valid_until is required")
      }
      return updateAccessRightsSchedule(token, {
        ...buildAccessRightsSelectionPayload(),
        valid_from: bulkValidFrom.trim(),
        valid_until: nextValidUntil,
      })
    },
    onSuccess: async () => {
      setBulkScheduleOpen(false)
      setBulkScheduleTemplateID("")
      setBulkValidFrom("")
      setBulkValidUntil(defaultValidUntil())
      setSelectedRowKeys([])
      setImpactPreview(null)
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Access rights schedule update failed"),
  })

  const reviewAccessRightsMutation = useMutation({
    mutationFn: () => reviewAccessRights(token, buildAccessRightsSelectionPayload()),
    onSuccess: async () => {
      setImpactPreviewOpen(false)
      setImpactPreview(null)
      setSelectedRowKeys([])
      setActionError("")
      await queryClient.invalidateQueries({ queryKey: ["kisi-resource-summary"] })
    },
    onError: (error) => setActionError(error instanceof Error ? error.message : "Access rights review failed"),
  })

  return (
    <>
      <PageFrame
        breadcrumbs={placeScoped ? ["Home", "Places", placeContext.place?.name ?? "Assigned Place", "Access Rights"] : ["Home", "Access Rights"]}
        title="Access Rights"
        count={resourceQuery.isPending ? "--" : accessRights.length}
        description="Share access by connecting users, teams, groups, places, and schedules"
        actions={
          <Button
            disabled={!canMutate}
            onClick={() => {
              resetShareAccessForm("role_assignment")
              setShareAccessOpen(true)
            }}
            className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
          >
            <PlusIcon className="mr-1.5 size-4" />
            Share Access
          </Button>
        }
      >
        {resourceQuery.usingFallback ? (
          <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
            Live access-right resources are unavailable. Showing reference data.
          </div>
        ) : null}

        {actionError ? (
          <div className="rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-5 py-4 text-sm text-[#8a5a00]">
            {actionError}
          </div>
        ) : null}

        <section className="overflow-hidden rounded-[6px] border border-[#d9dbe3] bg-white">
          <div className="flex gap-8 overflow-x-auto border-b border-[#eceef2] px-6">
            {tabs.map((tab) => (
              <button
                key={tab}
                type="button"
                onClick={() => setActiveTab(tab)}
                className={cn(
                  "whitespace-nowrap py-5 text-base font-semibold",
                  activeTab === tab ? "border-b-2 border-[#4f55ff] text-[#4f55ff]" : "text-[#2f3037]"
                )}
              >
                {tab}
              </button>
            ))}
          </div>
          <div className="grid gap-3 border-b border-[#eceef2] bg-[#fbfbfc] px-6 py-4 text-sm text-[#6f717c] sm:grid-cols-3">
            <div>
              Needs review <span className="font-semibold text-[#17171c]">{reviewCount}</span>
            </div>
            <div>
              Scheduled <span className="font-semibold text-[#17171c]">{scheduledCount}</span>
            </div>
            <div>
              Expired <span className="font-semibold text-[#17171c]">{expiredCount}</span>
            </div>
          </div>
          <div className="flex flex-col gap-4 border-b border-[#eceef2] p-6 md:flex-row md:items-center">
            <KisiSearchField value={query} onChange={setQuery} placeholder={`Search ${activeTab.toLowerCase()}...`} className="h-12 bg-transparent" />
            <select
              value={activeTargetFilter}
              onChange={(event) => setTargetFilter(event.target.value)}
              className="h-12 rounded-[6px] border border-[#d9dbe3] bg-white px-4 text-sm font-semibold text-[#2f3037] md:w-56"
              aria-label="Filter access rights by target"
            >
              <option value="all">All targets</option>
              {targetOptions.map((target) => (
                <option key={target} value={target}>
                  {target}
                </option>
              ))}
            </select>
            <select
              value={scheduleFilter}
              onChange={(event) => setScheduleFilter(event.target.value as ScheduleFilter)}
              className="h-12 rounded-[6px] border border-[#d9dbe3] bg-white px-4 text-sm font-semibold text-[#2f3037] md:w-48"
              aria-label="Filter access rights by schedule"
            >
              <option value="all">All schedules</option>
              <option value="enabled">Enabled</option>
              <option value="scheduled">Scheduled</option>
              <option value="expired">Expired</option>
              <option value="needs_review">Needs review</option>
            </select>
          </div>
          {someVisibleSelected ? (
            <div className="flex flex-col gap-3 border-b border-[#eceef2] bg-[#f7f8ff] px-6 py-4 text-sm text-[#2f3037] md:flex-row md:items-center md:justify-between">
              <div>
                <span className="font-semibold text-[#17171c]">{selectedRows.length}</span> selected
                <span className="ml-3 text-[#6f717c]">
                  {selectedRows.filter(accessRightNeedsReview).length} need review
                </span>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  disabled={!canMutate || updateAccessRightsScheduleMutation.isPending}
                  onClick={openBulkScheduleSheet}
                  className="h-9 rounded-[6px] border border-[#c8cad6] bg-white px-3 text-[#2f3037] hover:bg-[#f3f4f8]"
                >
                  <CalendarClockIcon className="mr-1.5 size-4" />
                  Edit schedule
                </Button>
                <Button
                  type="button"
                  disabled={!canMutate || previewAccessRightsMutation.isPending || reviewAccessRightsMutation.isPending}
                  onClick={() => previewAccessRightsMutation.mutate()}
                  className="h-9 rounded-[6px] border border-[#c8cad6] bg-white px-3 text-[#2f3037] hover:bg-[#f3f4f8]"
                >
                  <EyeIcon className="mr-1.5 size-4" />
                  {previewAccessRightsMutation.isPending ? "Previewing..." : "Preview impact"}
                </Button>
                <Button
                  type="button"
                  disabled={!canMutate || reviewAccessRightsMutation.isPending || previewAccessRightsMutation.isPending}
                  onClick={() => reviewAccessRightsMutation.mutate()}
                  className="h-9 rounded-[6px] bg-[#4f55ff] px-3 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
                >
                  <CheckIcon className="mr-1.5 size-4" />
                  {reviewAccessRightsMutation.isPending ? "Marking..." : "Mark reviewed"}
                </Button>
              </div>
            </div>
          ) : null}
          <div className="overflow-x-auto">
            <table className="w-full min-w-[940px] text-left text-sm">
              <thead className="bg-[#fbfbfc]">
                <tr className="border-b border-[#eceef2]">
                  <th className="w-16 px-6 py-4">
                    <input
                      type="checkbox"
                      checked={allVisibleSelected}
                      disabled={rows.length === 0 || !canMutate}
                      onChange={(event) => toggleVisibleRows(event.target.checked)}
                      className="size-5 rounded-[3px] border border-[#9a9ca7] accent-[#4f55ff]"
                      aria-label="Select visible access rights"
                    />
                  </th>
                  <th className="px-4 py-4 font-semibold">Name</th>
                  <th className="px-4 py-4 font-semibold">Type</th>
                  <th className="px-4 py-4 font-semibold">Target</th>
                  <th className="px-4 py-4 font-semibold">Rule</th>
                  <th className="px-4 py-4 font-semibold">Status</th>
                  <th className="px-4 py-4 text-right font-semibold">Actions</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => (
                  <tr key={`${activeTab}-${row.id}`} className="border-b border-[#eceef2] last:border-0 hover:bg-[#fbfbfc]">
                    <td className="px-6 py-5">
                      <input
                        type="checkbox"
                        checked={selectedRowKeySet.has(rowSelectionKey(row))}
                        disabled={!canMutate}
                        onChange={(event) => toggleSelectedRow(row, event.target.checked)}
                        className="size-5 rounded-[3px] border border-[#d9dbe3] accent-[#4f55ff]"
                        aria-label={`Select ${row.name}`}
                      />
                    </td>
                    <td className="px-4 py-5 font-semibold text-[#4f55ff]">{row.name}</td>
                    <td className="px-4 py-5 text-[#2f3037]">{row.type}</td>
                    <td className="px-4 py-5 text-[#6f717c]">{row.target}</td>
                    <td className="px-4 py-5 text-[#6f717c]">{row.rule}</td>
                    <td className="px-4 py-5">
                      <div className="space-y-1">
                        <StatusDot tone={row.tone} label={row.status} />
                        {row.reviewedAt ? (
                          <div className="text-xs text-[#6f717c]">
                            Reviewed{row.reviewedBy ? ` by ${row.reviewedBy}` : ""}
                          </div>
                        ) : null}
                      </div>
                    </td>
                    <td className="px-4 py-5 text-right">
                      <RowActionsMenu
                        label={`Actions for ${row.name}`}
                        items={[
                          {
                            id: "edit",
                            label: "Review / Edit",
                            icon: PencilIcon,
                            disabled: !canMutate || updateAccessRightMutation.isPending,
                            onSelect: () => openEditSheet(row),
                          },
                          {
                            id: "delete",
                            label: "Delete",
                            icon: Trash2Icon,
                            destructive: true,
                            disabled: !canMutate || deleteAccessRightMutation.isPending,
                            onSelect: () => {
                              setActionError("")
                              setDeleteTarget(row)
                            },
                          },
                        ]}
                      />
                    </td>
                  </tr>
                ))}
                {rows.length === 0 ? (
                  <KisiEmptyTableRow colSpan={7}>No access rights match this search.</KisiEmptyTableRow>
                ) : null}
              </tbody>
            </table>
          </div>
          <KisiTablePagination />
        </section>
      </PageFrame>

      <ConfirmActionDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!deleteAccessRightMutation.isPending && !open) {
            setDeleteTarget(null)
          }
        }}
        title={deleteTarget?.sourceType === "share" ? "Delete access link" : "Delete role assignment"}
        description={
          <>
            This removes <span className="font-semibold text-[#17171c]">{deleteTarget?.name ?? "this access right"}</span> from{" "}
            {deleteTarget?.target ?? "this scope"}.
          </>
        }
        confirmLabel={deleteTarget?.sourceType === "share" ? "Delete link" : "Delete assignment"}
        pending={deleteAccessRightMutation.isPending}
        disabled={!canMutate || !deleteTarget}
        destructive
        onConfirm={() => {
          if (deleteTarget) {
            deleteAccessRightMutation.mutate(deleteTarget)
          }
        }}
      />

      <Sheet open={shareAccessOpen} onOpenChange={setShareAccessOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[500px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Share Access</SheetTitle>
            <SheetDescription>Create a role assignment or access link for this scope.</SheetDescription>
          </SheetHeader>
          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              createAccessRightMutation.mutate()
            }}
          >
            <div className="grid grid-cols-2 gap-2 rounded-[6px] bg-[#f3f4f8] p-1">
              <button
                type="button"
                disabled={createAccessRightMutation.isPending}
                onClick={() => resetShareAccessForm("role_assignment")}
                className={cn(
                  "h-10 rounded-[5px] text-sm font-semibold",
                  shareMode === "role_assignment" ? "bg-white text-[#17171c] shadow-sm" : "text-[#6f717c]"
                )}
              >
                Role Assignment
              </button>
              <button
                type="button"
                disabled={createAccessRightMutation.isPending}
                onClick={() => resetShareAccessForm("share")}
                className={cn(
                  "h-10 rounded-[5px] text-sm font-semibold",
                  shareMode === "share" ? "bg-white text-[#17171c] shadow-sm" : "text-[#6f717c]"
                )}
              >
                Access Link
              </button>
            </div>

            {shareMode === "role_assignment" ? (
              <>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Scope type</span>
                    <select
                      value={scopeType}
                      onChange={(event) => updateScopeType(event.target.value as RoleAssignment["applies_to_type"])}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    >
                      {scopeTypeOptions.map((type) => (
                        <option key={type} value={type}>
                          {type}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Role</span>
                    <select
                      value={roleID}
                      onChange={(event) => setRoleID(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    >
                      {roleOptions.length === 0 ? (
                        <option value={roleID}>{roleID}</option>
                      ) : (
                        roleOptions.map((role) => (
                          <option key={role.id} value={role.id}>
                            {role.name}
                          </option>
                        ))
                      )}
                    </select>
                  </label>
                </div>

                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Scope</span>
                  <select
                    value={scopeID}
                    onChange={(event) => setScopeID(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                  >
                    {scopeOptions.length === 0 ? (
                      <option value="">No scope available</option>
                    ) : (
                      scopeOptions.map((scope) => (
                        <option key={scope.id} value={scope.id}>
                          {scope.name}
                        </option>
                      ))
                    )}
                  </select>
                </label>

                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Assignee type</span>
                    <select
                      value={assigneeType}
                      onChange={(event) => updateAssigneeType(event.target.value as RoleAssignment["assignee_type"])}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    >
                      <option value="User">User</option>
                      <option value="Team">Team</option>
                      <option value="Guest">Guest</option>
                    </select>
                  </label>
                  {assigneeType === "Guest" ? (
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Guest email</span>
                      <input
                        value={assigneeEmail}
                        onChange={(event) => {
                          setAssigneeEmail(event.target.value)
                          setAssigneeID(event.target.value)
                        }}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      />
                    </label>
                  ) : (
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Assignee</span>
                      <select
                        value={assigneeID}
                        onChange={(event) => updateAssigneeID(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      >
                        {assigneeOptions.length === 0 ? (
                          <option value="">No assignee available</option>
                        ) : (
                          assigneeOptions.map((assignee) => (
                            <option key={assignee.id} value={assignee.id}>
                              {assignee.name}
                            </option>
                          ))
                        )}
                      </select>
                    </label>
                  )}
                </div>

                {assigneeType === "User" ? (
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Assignee email</span>
                    <input
                      value={assigneeEmail}
                      onChange={(event) => setAssigneeEmail(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    />
                  </label>
                ) : null}
              </>
            ) : (
              <>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Email</span>
                    <input
                      value={shareEmail}
                      onChange={(event) => setShareEmail(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    />
                  </label>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Name</span>
                    <input
                      value={shareName}
                      onChange={(event) => setShareName(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    />
                  </label>
                </div>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Group</span>
                  <select
                    value={shareGroupID}
                    onChange={(event) => setShareGroupID(event.target.value)}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                  >
                    <option value="">No group</option>
                    {groupRows.map((group) => (
                      <option key={group.id} value={group.id}>
                        {group.name}
                      </option>
                    ))}
                  </select>
                </label>
                <div className="grid gap-3 sm:grid-cols-2">
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Place</span>
                    <select
                      value={sharePlaceID}
                      onChange={(event) => setSharePlaceID(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    >
                      <option value="">No place</option>
                      {placeRows.map((place) => (
                        <option key={place.id} value={place.id}>
                          {place.name}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Door</span>
                    <select
                      value={shareLockID}
                      onChange={(event) => setShareLockID(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    >
                      <option value="">No door</option>
                      {doorRows.map((door) => (
                        <option key={door.id} value={door.id}>
                          {door.name}
                        </option>
                      ))}
                    </select>
                  </label>
                </div>
              </>
            )}

            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Schedule template</span>
              <select
                value={scheduleTemplateID}
                disabled={scheduleTemplatesQuery.isPending && scheduleTemplates.length === 0}
                onChange={(event) => updateScheduleTemplate(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] disabled:bg-[#f8f8fa]"
              >
                <option value="">Custom schedule</option>
                {scheduleTemplates.map((template) => (
                  <option key={template.id} value={template.id}>
                    {template.name}
                  </option>
                ))}
              </select>
            </label>

            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Valid from</span>
                <input
                  value={validFrom}
                  placeholder="Optional"
                  onChange={(event) => {
                    setScheduleTemplateID("")
                    setValidFrom(event.target.value)
                  }}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Valid until</span>
                <input
                  value={validUntil}
                  placeholder="2026-05-01T10:00:00Z"
                  onChange={(event) => {
                    setScheduleTemplateID("")
                    setValidUntil(event.target.value)
                  }}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                />
              </label>
            </div>

            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={
                  createAccessRightMutation.isPending ||
                  (shareMode === "role_assignment" ? !canSubmitRoleAssignment : !canSubmitShare)
                }
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
              >
                {createAccessRightMutation.isPending ? "Creating..." : "Share Access"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet
        open={editOpen}
        onOpenChange={(open) => {
          setEditOpen(open)
          if (!open) {
            setEditingRow(null)
          }
        }}
      >
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[520px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Edit Access Right</SheetTitle>
            <SheetDescription>Review the live reference record and update its role or schedule.</SheetDescription>
          </SheetHeader>

          {editDetailQuery.isPending ? (
            <div className="px-6 py-5 text-sm text-[#6f717c]">Loading access right...</div>
          ) : null}

          {editDetailQuery.error ? (
            <div className="mx-6 mt-5 rounded-[6px] border border-[#f1c27a] bg-[#fff8ed] px-4 py-3 text-sm text-[#8a5a00]">
              {editDetailQuery.error instanceof Error ? editDetailQuery.error.message : "Access right detail failed"}
            </div>
          ) : null}

          {editingRow && !editDetailQuery.isPending && !editDetailQuery.error ? (
            <form
              className="space-y-5 px-6 py-5"
              onSubmit={(event) => {
                event.preventDefault()
                updateAccessRightMutation.mutate()
              }}
            >
              <div className="grid gap-3 rounded-[6px] border border-[#eceef2] bg-[#fbfbfc] p-4 text-sm">
                <div className="flex items-start justify-between gap-4">
                  <span className="text-xs font-semibold text-[#6f717c]">Record</span>
                  <span className="text-right font-semibold text-[#17171c]">{editingRow.id}</span>
                </div>
                <div className="flex items-start justify-between gap-4">
                  <span className="text-xs font-semibold text-[#6f717c]">Type</span>
                  <span className="text-right text-[#2f3037]">{editingRow.type}</span>
                </div>
                <div className="flex items-start justify-between gap-4">
                  <span className="text-xs font-semibold text-[#6f717c]">Current target</span>
                  <span className="max-w-[18rem] text-right text-[#2f3037]">{editingRow.target}</span>
                </div>
              </div>

              {editingRow.sourceType === "role_assignment" && editRoleAssignment ? (
                <>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Scope type</span>
                      <input
                        value={editRoleAssignment.applies_to_type}
                        readOnly
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-[#f8f8fa] px-3 text-sm text-[#2f3037]"
                      />
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Scope ID</span>
                      <input
                        value={editRoleAssignment.applies_to_id}
                        readOnly
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-[#f8f8fa] px-3 text-sm text-[#2f3037]"
                      />
                    </label>
                  </div>

                  <div className="grid gap-3 sm:grid-cols-2">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Assignee type</span>
                      <input
                        value={editRoleAssignment.assignee_type}
                        readOnly
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-[#f8f8fa] px-3 text-sm text-[#2f3037]"
                      />
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Assignee ID</span>
                      <input
                        value={editRoleAssignment.assignee_id}
                        readOnly
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-[#f8f8fa] px-3 text-sm text-[#2f3037]"
                      />
                    </label>
                  </div>

                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Role</span>
                    <select
                      value={editRoleID}
                      onChange={(event) => setEditRoleID(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    >
                      {editRoleOptions.length === 0 ? (
                        <option value={editRoleID}>{editRoleID}</option>
                      ) : (
                        editRoleOptions.map((role) => (
                          <option key={role.id} value={role.id}>
                            {role.name}
                          </option>
                        ))
                      )}
                    </select>
                  </label>
                </>
              ) : null}

              {editingRow.sourceType === "share" && editShare ? (
                <>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Email</span>
                      <input
                        value={editShareEmail}
                        onChange={(event) => setEditShareEmail(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      />
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Name</span>
                      <input
                        value={editShareName}
                        onChange={(event) => setEditShareName(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      />
                    </label>
                  </div>

                  <label className="block">
                    <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Group</span>
                    <select
                      value={editShareGroupID}
                      onChange={(event) => setEditShareGroupID(event.target.value)}
                      className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                    >
                      <option value="">No group</option>
                      {groupRows.map((group) => (
                        <option key={group.id} value={group.id}>
                          {group.name}
                        </option>
                      ))}
                    </select>
                  </label>

                  <div className="grid gap-3 sm:grid-cols-2">
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Place</span>
                      <select
                        value={editSharePlaceID}
                        onChange={(event) => setEditSharePlaceID(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      >
                        <option value="">No place</option>
                        {placeRows.map((place) => (
                          <option key={place.id} value={place.id}>
                            {place.name}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="block">
                      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Door</span>
                      <select
                        value={editShareLockID}
                        onChange={(event) => setEditShareLockID(event.target.value)}
                        className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                      >
                        <option value="">No door</option>
                        {doorRows.map((door) => (
                          <option key={door.id} value={door.id}>
                            {door.name}
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>
                </>
              ) : null}

              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Schedule template</span>
                <select
                  value={editScheduleTemplateID}
                  disabled={scheduleTemplatesQuery.isPending && scheduleTemplates.length === 0}
                  onChange={(event) => updateEditScheduleTemplate(event.target.value)}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] disabled:bg-[#f8f8fa]"
                >
                  <option value="">Custom schedule</option>
                  {scheduleTemplates.map((template) => (
                    <option key={template.id} value={template.id}>
                      {template.name}
                    </option>
                  ))}
                </select>
              </label>

              <div className="grid gap-3 sm:grid-cols-2">
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Valid from</span>
                  <input
                    value={editValidFrom}
                    placeholder="Optional"
                    onChange={(event) => {
                      setEditScheduleTemplateID("")
                      setEditValidFrom(event.target.value)
                    }}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                  />
                </label>
                <label className="block">
                  <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Valid until</span>
                  <input
                    value={editValidUntil}
                    placeholder="2026-05-01T10:00:00Z"
                    onChange={(event) => {
                      setEditScheduleTemplateID("")
                      setEditValidUntil(event.target.value)
                    }}
                    className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                  />
                </label>
              </div>

              <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
                <Button
                  type="submit"
                  disabled={!canSubmitEdit || updateAccessRightMutation.isPending}
                  className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
                >
                  {updateAccessRightMutation.isPending ? "Saving..." : "Save Changes"}
                </Button>
              </SheetFooter>
            </form>
          ) : null}
        </SheetContent>
      </Sheet>

      <Sheet open={bulkScheduleOpen} onOpenChange={setBulkScheduleOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[500px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Edit Schedule</SheetTitle>
            <SheetDescription>Update the schedule for selected access rights.</SheetDescription>
          </SheetHeader>

          <form
            className="space-y-5 px-6 py-5"
            onSubmit={(event) => {
              event.preventDefault()
              updateAccessRightsScheduleMutation.mutate()
            }}
          >
            <div className="grid gap-3 rounded-[6px] border border-[#eceef2] bg-[#fbfbfc] p-4 text-sm">
              <div className="flex items-start justify-between gap-4">
                <span className="text-xs font-semibold text-[#6f717c]">Selected</span>
                <span className="text-right font-semibold text-[#17171c]">{selectedRows.length}</span>
              </div>
              <div className="flex items-start justify-between gap-4">
                <span className="text-xs font-semibold text-[#6f717c]">Role assignments</span>
                <span className="text-right text-[#2f3037]">{selectedRows.filter((row) => row.sourceType === "role_assignment").length}</span>
              </div>
              <div className="flex items-start justify-between gap-4">
                <span className="text-xs font-semibold text-[#6f717c]">Access links</span>
                <span className="text-right text-[#2f3037]">{selectedRows.filter((row) => row.sourceType === "share").length}</span>
              </div>
            </div>

            <label className="block">
              <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Schedule template</span>
              <select
                value={bulkScheduleTemplateID}
                disabled={scheduleTemplatesQuery.isPending && scheduleTemplates.length === 0}
                onChange={(event) => updateBulkScheduleTemplate(event.target.value)}
                className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037] disabled:bg-[#f8f8fa]"
              >
                <option value="">Custom schedule</option>
                {scheduleTemplates.map((template) => (
                  <option key={template.id} value={template.id}>
                    {template.name}
                  </option>
                ))}
              </select>
            </label>

            <div className="grid gap-3 sm:grid-cols-2">
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Valid from</span>
                <input
                  value={bulkValidFrom}
                  placeholder="Optional"
                  onChange={(event) => {
                    setBulkScheduleTemplateID("")
                    setBulkValidFrom(event.target.value)
                  }}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold text-[#6f717c]">Valid until</span>
                <input
                  value={bulkValidUntil}
                  placeholder="2026-05-01T10:00:00Z"
                  onChange={(event) => {
                    setBulkScheduleTemplateID("")
                    setBulkValidUntil(event.target.value)
                  }}
                  className="h-11 w-full rounded-[6px] border border-[#d9dbe3] bg-white px-3 text-sm text-[#2f3037]"
                />
              </label>
            </div>

            <SheetFooter className="-mx-6 mt-6 border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
              <Button
                type="submit"
                disabled={!canSubmitBulkSchedule || updateAccessRightsScheduleMutation.isPending}
                className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea] disabled:bg-[#c6c8d2]"
              >
                {updateAccessRightsScheduleMutation.isPending ? "Saving..." : "Save Schedule"}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet open={impactPreviewOpen} onOpenChange={setImpactPreviewOpen}>
        <SheetContent className="w-full overflow-y-auto bg-white sm:max-w-[560px]">
          <SheetHeader className="border-b border-[#eceef2] px-6 py-5">
            <SheetTitle>Access Impact Preview</SheetTitle>
            <SheetDescription>Review affected resources before marking selected access rights as reviewed.</SheetDescription>
          </SheetHeader>

          {impactPreview ? (
            <div className="space-y-5 px-6 py-5">
              <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
                {[
                  ["Selected", impactPreview.selected_count],
                  ["Needs review", impactPreview.needs_review_count],
                  ["Users", impactPreview.affected_users],
                  ["Teams", impactPreview.affected_teams],
                  ["Groups", impactPreview.affected_groups],
                  ["Places", impactPreview.affected_places],
                  ["Locks", impactPreview.affected_locks],
                ].map(([label, value]) => (
                  <div key={label} className="rounded-[6px] border border-[#eceef2] bg-[#fbfbfc] px-4 py-3">
                    <div className="text-xs font-semibold text-[#6f717c]">{label}</div>
                    <div className="mt-1 text-lg font-semibold text-[#17171c]">{value}</div>
                  </div>
                ))}
              </div>

              <div className="overflow-hidden rounded-[6px] border border-[#eceef2]">
                <div className="border-b border-[#eceef2] bg-[#fbfbfc] px-4 py-3 text-sm font-semibold text-[#17171c]">
                  Selected records
                </div>
                <div className="divide-y divide-[#eceef2]">
                  {impactPreview.items.map((item) => (
                    <div key={`${item.source_type}-${item.id}`} className="grid gap-2 px-4 py-3 text-sm">
                      <div className="flex items-start justify-between gap-4">
                        <div>
                          <div className="font-semibold text-[#17171c]">{item.name}</div>
                          <div className="text-xs text-[#6f717c]">{item.source_type === "share" ? "Access Link" : "Role Assignment"}</div>
                        </div>
                        <StatusDot tone={accessRightPreviewTone(item.status, item.needs_review)} label={item.status} />
                      </div>
                      <div className="text-[#6f717c]">{item.target}</div>
                      <div className="text-xs text-[#6f717c]">
                        {item.affected_users} users, {item.affected_groups} groups, {item.affected_places} places, {item.affected_locks} locks
                      </div>
                      {item.reviewed_at ? (
                        <div className="text-xs text-[#6f717c]">
                          Reviewed{item.reviewed_by ? ` by ${item.reviewed_by}` : ""}
                        </div>
                      ) : null}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          ) : (
            <div className="px-6 py-5 text-sm text-[#6f717c]">No preview loaded.</div>
          )}

          <SheetFooter className="border-t border-[#eceef2] bg-[#fbfbfc] px-6 py-4">
            <Button
              type="button"
              disabled={!canMutate || selectedRows.length === 0 || reviewAccessRightsMutation.isPending}
              onClick={() => reviewAccessRightsMutation.mutate()}
              className="h-10 rounded-[6px] bg-[#4f55ff] px-5 text-white hover:bg-[#454bea]"
            >
              <CheckIcon className="mr-1.5 size-4" />
              {reviewAccessRightsMutation.isPending ? "Marking..." : "Mark reviewed"}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </>
  )
}
