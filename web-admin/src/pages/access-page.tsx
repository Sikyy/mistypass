import { FormEvent, useEffect, useMemo, useState } from "react"
import { useLocation, useNavigate } from "react-router-dom"
import { useQuery } from "@tanstack/react-query"

import { AccessDomainBanner } from "@/components/access/access-domain-banner"
import { AccessDomainBannerActions } from "@/components/access/access-domain-banner-actions"
import { AccessDomainMetricsCards } from "@/components/access/access-domain-metrics-cards"
import {
  applyEnterpriseWalletContext,
  buildEnterpriseFlowSummary,
  buildEnterpriseSyncGroupDraft,
  buildEnterpriseStagePresetKey,
  buildEnterpriseSummaryTail,
  buildEnterpriseSyncRecordLabel,
  buildEnterpriseStageSearch,
  buildEnterpriseSyncWorkerReviewLink,
  buildEnterpriseWorkerAlertLabel,
  buildEnterpriseWorkerGroupDraft,
  buildEnterpriseWorkerPolicyDraftName,
  buildFlowSegmentDescriptor,
  deriveEnterpriseHintedMemberLabel,
  deriveEnterpriseRemediationLabel,
  findByGroupNameHint,
  findByNameHint,
  findHintedGroupMember,
  hasWorkerAlertFlowHints as hasEnterpriseWorkerAlertFlowHints,
  parseEnterpriseFlowContext,
  resolveEnterpriseAccessStageRoute,
} from "@/components/access/access-enterprise-flow-utils"
import { AccessGrantDetailDialog } from "@/components/access/access-grant-detail-dialog"
import { AccessGrantFilterBar } from "@/components/access/access-grant-filter-bar"
import { AccessGrantForm } from "@/components/access/access-grant-form"
import { AccessGrantLedgerTable, type AccessGrantLedgerRow } from "@/components/access/access-grant-ledger-table"
import { AccessGrantOverviewCards } from "@/components/access/access-grant-overview-cards"
import { AccessGrantStarterCard } from "@/components/access/access-grant-starter-card"
import { buildTenantGrantViewModel } from "@/components/access/access-grant-ledger-utils"
import {
  buildGroupLedgerRows,
  buildPolicyLedgerRows,
  derivePolicyLedgerMatch,
  deriveGroupLedgerEmptyState,
  derivePolicyLedgerEmptyState,
  deriveSuggestedPolicyLedgerQuery,
  filterPolicyLedgerRows,
} from "@/components/access/access-ledger-view-model-utils"
import { AccessOperationFeedback } from "@/components/access/access-operation-feedback"
import { AccessPageHeader } from "@/components/access/access-page-header"
import {
  buildAccessSectionsOverview,
  deriveNextRecommendedAction,
} from "@/components/access/access-page-recommendation-utils"
import {
  buildGrantStarters,
  buildPolicyStarters,
  type GrantStarter,
  type PolicyStarter,
} from "@/components/access/access-starter-utils"
import {
  deliveryLabel,
  enterpriseFlowStageLabel,
  getGrantLifecycleStatus,
  grantLifecycleBadgeVariant,
  grantLifecycleLabel,
  inferStarterGroupMemberIDs,
  isAccessSection,
  positionTemplateSpec,
  remainingLabel,
  resolveAccessSection,
  scopeSummary,
  sectionFromAccessPath,
  type DeliveryMethod,
  type ScopeType,
  validateScope,
} from "@/components/access/access-page-utils"
import { AccessReadinessOverviewCards } from "@/components/access/access-readiness-overview-cards"
import { AccessSectionsTabs, type AccessSection } from "@/components/access/access-sections-tabs"
import { AccessSectionOverviewCards } from "@/components/access/access-section-overview-cards"
import {
  createAccessPolicy,
  createTemporaryAccess,
  createUserGroup,
  listAccessPolicies,
  listAreas,
  listBuildings,
  listDoors,
  listEnterpriseEmployees,
  listTemporaryAccess,
  listTenants,
  listUserGroups,
  updateAccessPolicy,
  updateUserGroup,
  type CurrentUser,
  type AccessPolicy,
  type Area,
  type Building,
  type Door,
  type EnterpriseEmployee,
  type TemporaryAccess,
  type Tenant,
  type UserGroup,
} from "@/lib/api"
import { getViewerTenantID, isPlatformViewer } from "@/lib/viewer"

export type AccessPageProps = {
  token: string
  viewer: CurrentUser
  activeSectionOverride?: AccessSection
}

type AccessBaseData = {
  policies: AccessPolicy[]
  userGroups: UserGroup[]
  grants: TemporaryAccess[]
  tenants: Tenant[]
}

type AccessTopologyData = {
  buildings: Building[]
  areas: Area[]
  doors: Door[]
}

async function loadAccessBaseData(args: {
  token: string
  platformViewer: boolean
}): Promise<AccessBaseData> {
  const [policyItems, groupItems, temporaryItems, tenantItems] = await Promise.all([
    listAccessPolicies(args.token),
    listUserGroups(args.token),
    listTemporaryAccess(args.token),
    args.platformViewer ? listTenants(args.token) : Promise.resolve([]),
  ])
  return {
    policies: policyItems,
    userGroups: groupItems,
    grants: temporaryItems,
    tenants: tenantItems,
  }
}

async function loadAccessTopologyData(args: {
  token: string
  selectedTenantID: string
}): Promise<AccessTopologyData> {
  const [buildingItems, areaItems, doorItems] = await Promise.all([
    listBuildings(args.token, args.selectedTenantID),
    listAreas(args.token, args.selectedTenantID),
    listDoors(args.token, args.selectedTenantID),
  ])
  return {
    buildings: buildingItems,
    areas: areaItems,
    doors: doorItems,
  }
}

export function AccessPage({ token, viewer, activeSectionOverride }: AccessPageProps) {
  const platformViewer = isPlatformViewer(viewer)
  const viewerTenantID = getViewerTenantID(viewer)
  const navigate = useNavigate()
  const location = useLocation()
  const pathnameSection = sectionFromAccessPath(location.pathname)
  const activeSection = activeSectionOverride ?? resolveAccessSection(pathnameSection)
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [selectedTenantID, setSelectedTenantID] = useState("")

  const [buildings, setBuildings] = useState<Building[]>([])
  const [areas, setAreas] = useState<Area[]>([])
  const [doors, setDoors] = useState<Door[]>([])

  const [policies, setPolicies] = useState<AccessPolicy[]>([])
  const [userGroups, setUserGroups] = useState<UserGroup[]>([])
  const [grants, setGrants] = useState<TemporaryAccess[]>([])

  const [error, setError] = useState("")
  const [summary, setSummary] = useState("")

  const [policyEditID, setPolicyEditID] = useState("")
  const [policyName, setPolicyName] = useState("")
  const [policyScopeType, setPolicyScopeType] = useState<ScopeType>("all")
  const [policyBuildingID, setPolicyBuildingID] = useState("")
  const [policyAreaID, setPolicyAreaID] = useState("")
  const [policyDoorID, setPolicyDoorID] = useState("")
  const [policySchedule, setPolicySchedule] = useState("")
  const [policyStatus, setPolicyStatus] = useState<"active" | "inactive" | "draft">("active")
  const [policyMembers, setPolicyMembers] = useState("0")
  const [policyLedgerQuery, setPolicyLedgerQuery] = useState("")
  const [batchUpdatingPolicyStatus, setBatchUpdatingPolicyStatus] = useState<"" | "active" | "draft">("")
  const [policyBatchFlowHint, setPolicyBatchFlowHint] = useState("")
  const [policyBatchTargetIDsHint, setPolicyBatchTargetIDsHint] = useState<string[]>([])

  const [groupEditID, setGroupEditID] = useState("")
  const [groupName, setGroupName] = useState("")
  const [groupDescription, setGroupDescription] = useState("")
  const [employees, setEmployees] = useState<EnterpriseEmployee[]>([])
  const [groupMemberQuery, setGroupMemberQuery] = useState("")
  const [selectedGroupMemberIDs, setSelectedGroupMemberIDs] = useState<string[]>([])

  const [grantScopeType, setGrantScopeType] = useState<ScopeType>("door")
  const [grantBuildingID, setGrantBuildingID] = useState("")
  const [grantAreaID, setGrantAreaID] = useState("")
  const [grantDoorID, setGrantDoorID] = useState("")
  const [grantMethod, setGrantMethod] = useState<DeliveryMethod>("wallet")
  const [granteeName, setGranteeName] = useState("")
  const [granteeGender, setGranteeGender] = useState("")
  const [granteePhone, setGranteePhone] = useState("")
  const [granteeEmail, setGranteeEmail] = useState("")
  const [mobileModel, setMobileModel] = useState("")
  const [passType, setPassType] = useState("employee")
  const [validUntil, setValidUntil] = useState("")
  const [grantDateFrom, setGrantDateFrom] = useState("")
  const [grantDateTo, setGrantDateTo] = useState("")
  const [grantMethodFilter, setGrantMethodFilter] = useState<"all" | DeliveryMethod>("all")
  const [grantPassTypeFilter, setGrantPassTypeFilter] = useState("all")
  const [grantStatusFilter, setGrantStatusFilter] = useState<"all" | "active" | "expiring_soon" | "expired">("all")
  const [activeGrant, setActiveGrant] = useState<TemporaryAccess | null>(null)
  const [nowTick, setNowTick] = useState(() => Date.now())
  const [enterpriseFlowSearchApplied, setEnterpriseFlowSearchApplied] = useState("")
  const [enterpriseStagePresetApplied, setEnterpriseStagePresetApplied] = useState("")
  const baseDataQuery = useQuery({
    queryKey: ["access-base-data", token, platformViewer ? "platform" : "tenant"],
    queryFn: () =>
      loadAccessBaseData({
        token,
        platformViewer,
      }),
    staleTime: 30 * 1000,
  })
  const topologyQuery = useQuery({
    queryKey: ["access-topology-data", token, selectedTenantID],
    queryFn: () =>
      loadAccessTopologyData({
        token,
        selectedTenantID,
      }),
    enabled: selectedTenantID.trim().length > 0,
    staleTime: 30 * 1000,
  })
  const employeesQuery = useQuery({
    queryKey: ["access-enterprise-employees", token, selectedTenantID],
    queryFn: () => listEnterpriseEmployees(token, selectedTenantID),
    enabled: selectedTenantID.trim().length > 0,
    staleTime: 30 * 1000,
  })
  const loading =
    baseDataQuery.isPending ||
    (selectedTenantID.trim().length > 0 && (topologyQuery.isPending || employeesQuery.isPending))
  const queryError =
    (baseDataQuery.error instanceof Error && baseDataQuery.error.message) ||
    (topologyQuery.error instanceof Error && topologyQuery.error.message) ||
    (employeesQuery.error instanceof Error && employeesQuery.error.message) ||
    ""

  useEffect(() => {
    if (!baseDataQuery.data) {
      return
    }
    setTenants(baseDataQuery.data.tenants)
    setPolicies(baseDataQuery.data.policies)
    setUserGroups(baseDataQuery.data.userGroups)
    setGrants(baseDataQuery.data.grants)
    setSelectedTenantID((current) => current || (platformViewer ? baseDataQuery.data.tenants[0]?.id || "" : viewerTenantID))
  }, [baseDataQuery.data, platformViewer, viewerTenantID])

  useEffect(() => {
    if (!selectedTenantID) {
      setBuildings([])
      setAreas([])
      setDoors([])
      return
    }
    if (!topologyQuery.data) {
      return
    }
    setBuildings(topologyQuery.data.buildings)
    setAreas(topologyQuery.data.areas)
    setDoors(topologyQuery.data.doors)
  }, [selectedTenantID, topologyQuery.data])

  useEffect(() => {
    if (!selectedTenantID) {
      setEmployees([])
      return
    }
    if (!employeesQuery.data) {
      return
    }
    setEmployees(employeesQuery.data)
  }, [employeesQuery.data, selectedTenantID])

  const tenantByID = useMemo(() => new Map(tenants.map((item) => [item.id, item])), [tenants])
  const enterpriseFlowContext = useMemo(() => parseEnterpriseFlowContext(location.search), [location.search])
  const buildingByID = useMemo(() => new Map(buildings.map((item) => [item.id, item])), [buildings])
  const areaByID = useMemo(() => new Map(areas.map((item) => [item.id, item])), [areas])
  const doorByID = useMemo(() => new Map(doors.map((item) => [item.id, item])), [doors])
  const employeeByID = useMemo(() => new Map(employees.map((item) => [item.id, item])), [employees])
  const hintedGroupMember = useMemo(
    () => findHintedGroupMember(enterpriseFlowContext, employees),
    [employees, enterpriseFlowContext]
  )
  const hintedMemberGroup = useMemo(() => {
    if (!hintedGroupMember) {
      return null
    }
    return (
      userGroups.find(
        (item) => item.tenant_id === selectedTenantID && (item.members ?? []).includes(hintedGroupMember.id)
      ) ?? null
    )
  }, [hintedGroupMember, selectedTenantID, userGroups])

  const flowMemberNameHint =
    enterpriseFlowContext?.groupMemberName || enterpriseFlowContext?.targetName || hintedGroupMember?.full_name || ""
  const flowMemberEmailHint =
    enterpriseFlowContext?.groupMemberEmail || enterpriseFlowContext?.targetEmail || hintedGroupMember?.email || ""
  const flowMemberIDHint =
    enterpriseFlowContext?.groupMemberID || enterpriseFlowContext?.targetID || hintedGroupMember?.id || ""
  const flowMemberStatusHint = enterpriseFlowContext?.groupMemberStatus || hintedGroupMember?.status || ""
  const flowRemediationHint = enterpriseFlowContext?.remediationHint || ""
  const flowGroupNameHint = enterpriseFlowContext?.groupName || hintedMemberGroup?.name || ""
  const flowPolicyGroupHint = enterpriseFlowContext?.policyGroup || flowGroupNameHint
  const flowPolicyNameHint = enterpriseFlowContext?.policyName || ""
  const flowSegmentDescriptor = useMemo(() => buildFlowSegmentDescriptor(enterpriseFlowContext), [enterpriseFlowContext])

  const directorySectionLink = `/access/directory${buildEnterpriseStageSearch({
    baseSearch: location.search,
    context: enterpriseFlowContext,
    selectedTenantID,
    stage: "directory",
    hints: {
      group_name: flowGroupNameHint,
      group_member_email: flowMemberEmailHint,
      group_member_id: flowMemberIDHint,
      group_member_name: flowMemberNameHint,
      group_member_status: flowMemberStatusHint,
      remediation_hint: flowRemediationHint,
    },
  })}`
  const policiesSectionLink = `/access/policies${buildEnterpriseStageSearch({
    baseSearch: location.search,
    context: enterpriseFlowContext,
    selectedTenantID,
    stage: "policies",
    hints: {
      policy_group: flowPolicyGroupHint,
      policy_name: flowPolicyNameHint,
      group_member_email: flowMemberEmailHint,
      group_member_id: flowMemberIDHint,
      group_member_name: flowMemberNameHint,
      group_member_status: flowMemberStatusHint,
      remediation_hint: flowRemediationHint,
    },
  })}`
  const grantsSectionLink = `/access/grants${buildEnterpriseStageSearch({
    baseSearch: location.search,
    context: enterpriseFlowContext,
    selectedTenantID,
    stage: "issuance",
    hints: {
      target_email: flowMemberEmailHint,
      target_id: flowMemberIDHint,
      target_name: flowMemberNameHint,
    },
  })}`
  const enterpriseHomeLink = `/enterprise${location.search}`
  const enterpriseSyncLink = `/enterprise${location.search}#sync`
  const hasWorkerAlertFlowHints = hasEnterpriseWorkerAlertFlowHints(enterpriseFlowContext)
  const enterpriseSyncWorkerReviewLink = useMemo(
    () =>
      buildEnterpriseSyncWorkerReviewLink({
        activeSection,
        baseSearch: location.search,
        context: enterpriseFlowContext,
        selectedTenantID,
      }),
    [activeSection, enterpriseFlowContext, location.search, selectedTenantID]
  )
  const spacesLink = `/spaces${location.search}`
  const walletEmployeeLink = useMemo(() => {
    const query = new URLSearchParams(location.search)
    query.set("scenario", "employee_mobile")
    applyEnterpriseWalletContext({
      context: enterpriseFlowContext,
      flowMemberEmailHint,
      flowMemberIDHint,
      flowMemberNameHint,
      query,
      selectedTenantID,
      targetHint: "user",
    })
    if (policyBatchTargetIDsHint.length > 0) {
      query.set("target_ids", policyBatchTargetIDsHint.join(","))
    }
    const nextQuery = query.toString()
    return nextQuery ? `/wallet?${nextQuery}` : "/wallet"
  }, [
    enterpriseFlowContext,
    flowMemberEmailHint,
    flowMemberIDHint,
    flowMemberNameHint,
    location.search,
    policyBatchTargetIDsHint,
    selectedTenantID,
  ])
  const walletVisitorLink = useMemo(() => {
    const query = new URLSearchParams(location.search)
    query.set("scenario", "visitor_temporary")
    applyEnterpriseWalletContext({
      context: enterpriseFlowContext,
      flowMemberEmailHint,
      flowMemberIDHint,
      flowMemberNameHint,
      query,
      selectedTenantID,
      targetHint: "visitor",
    })
    const nextQuery = query.toString()
    return nextQuery ? `/wallet?${nextQuery}` : "/wallet"
  }, [enterpriseFlowContext, flowMemberEmailHint, flowMemberIDHint, flowMemberNameHint, location.search, selectedTenantID])
  const filteredEmployees = useMemo(() => {
    const q = groupMemberQuery.trim().toLowerCase()
    if (!q) {
      return employees
    }
    return employees.filter((item) => {
      return (
        item.id.toLowerCase().includes(q) ||
        item.full_name.toLowerCase().includes(q) ||
        item.email.toLowerCase().includes(q) ||
        item.department.toLowerCase().includes(q) ||
        item.job_title.toLowerCase().includes(q)
      )
    })
  }, [employees, groupMemberQuery])
  const policyAreaOptions = useMemo(() => {
    if (!policyBuildingID) {
      return areas
    }
    return areas.filter((item) => item.building_id === policyBuildingID)
  }, [areas, policyBuildingID])
  const policyDoorOptions = useMemo(() => {
    return doors.filter((item) => {
      if (policyBuildingID && item.building_id !== policyBuildingID) {
        return false
      }
      if (policyAreaID && item.area_id !== policyAreaID) {
        return false
      }
      return true
    })
  }, [doors, policyAreaID, policyBuildingID])
  const grantAreaOptions = useMemo(() => {
    if (!grantBuildingID) {
      return areas
    }
    return areas.filter((item) => item.building_id === grantBuildingID)
  }, [areas, grantBuildingID])
  const grantDoorOptions = useMemo(() => {
    return doors.filter((item) => {
      if (grantBuildingID && item.building_id !== grantBuildingID) {
        return false
      }
      if (grantAreaID && item.area_id !== grantAreaID) {
        return false
      }
      return true
    })
  }, [doors, grantAreaID, grantBuildingID])

  useEffect(() => {
    const timer = window.setInterval(() => {
      setNowTick(Date.now())
    }, 1000)
    return () => {
      window.clearInterval(timer)
    }
  }, [])

  useEffect(() => {
    if (activeSection !== "policies" && (policyBatchFlowHint || policyBatchTargetIDsHint.length > 0)) {
      setPolicyBatchFlowHint("")
      setPolicyBatchTargetIDsHint([])
    }
  }, [activeSection, policyBatchFlowHint, policyBatchTargetIDsHint])

  useEffect(() => {
    if (!enterpriseFlowContext) {
      if (enterpriseFlowSearchApplied) {
        setEnterpriseFlowSearchApplied("")
      }
      return
    }
    if (enterpriseFlowSearchApplied === location.search) {
      return
    }

    const incomingTenantID = enterpriseFlowContext.tenantID
    const tenantExists = incomingTenantID ? tenants.some((item) => item.id === incomingTenantID) : false
    const canApplyTenant = Boolean(incomingTenantID && (!platformViewer || tenantExists))

    if (canApplyTenant && incomingTenantID !== selectedTenantID) {
      setSelectedTenantID(incomingTenantID)
    }

    const tenantLabel = canApplyTenant ? tenantByID.get(incomingTenantID)?.name || incomingTenantID : ""
    const flowLabel = enterpriseFlowContext.flow ? `${enterpriseFlowContext.flow} / ` : ""
    setSummary(
      buildEnterpriseFlowSummary(
        `已承接 ${flowLabel}${enterpriseFlowStageLabel(enterpriseFlowContext.stage)}${
        tenantLabel ? `（组织：${tenantLabel}）` : ""
      }${flowSegmentDescriptor ? `（分段提示：${flowSegmentDescriptor}）` : ""}。`
      )
    )
    setEnterpriseFlowSearchApplied(location.search)
  }, [
    enterpriseFlowContext,
    enterpriseFlowSearchApplied,
    location.search,
    platformViewer,
    selectedTenantID,
    flowSegmentDescriptor,
    tenantByID,
    tenants,
  ])

  useEffect(() => {
    if (policyScopeType === "all") {
      setPolicyBuildingID("")
      setPolicyAreaID("")
      setPolicyDoorID("")
      return
    }
    if (policyScopeType === "building") {
      setPolicyAreaID("")
      setPolicyDoorID("")
      return
    }
    if (policyScopeType === "area") {
      setPolicyDoorID("")
    }
  }, [policyScopeType])

  useEffect(() => {
    if (!policyBuildingID) {
      return
    }
    if (policyAreaID && !areas.some((item) => item.id === policyAreaID && item.building_id === policyBuildingID)) {
      setPolicyAreaID("")
      setPolicyDoorID("")
    }
  }, [areas, policyAreaID, policyBuildingID])

  useEffect(() => {
    if (!policyAreaID) {
      return
    }
    if (policyDoorID && !doors.some((item) => item.id === policyDoorID && item.area_id === policyAreaID)) {
      setPolicyDoorID("")
    }
  }, [doors, policyAreaID, policyDoorID])

  useEffect(() => {
    if (grantScopeType === "all") {
      setGrantBuildingID("")
      setGrantAreaID("")
      setGrantDoorID("")
      return
    }
    if (grantScopeType === "building") {
      setGrantAreaID("")
      setGrantDoorID("")
      return
    }
    if (grantScopeType === "area") {
      setGrantDoorID("")
    }
  }, [grantScopeType])

  useEffect(() => {
    if (!grantBuildingID) {
      return
    }
    if (grantAreaID && !areas.some((item) => item.id === grantAreaID && item.building_id === grantBuildingID)) {
      setGrantAreaID("")
      setGrantDoorID("")
    }
  }, [areas, grantAreaID, grantBuildingID])

  useEffect(() => {
    if (!grantAreaID) {
      return
    }
    if (grantDoorID && !doors.some((item) => item.id === grantDoorID && item.area_id === grantAreaID)) {
      setGrantDoorID("")
    }
  }, [doors, grantAreaID, grantDoorID])

  async function submitPolicy(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedTenantID || !policyName.trim()) {
      return
    }
    const scopeError = validateScope(policyScopeType, policyBuildingID, policyAreaID, policyDoorID)
    if (scopeError) {
      setError(scopeError)
      return
    }

    setError("")
    setSummary("")
    try {
      const scopedBuildingID = policyScopeType === "all" ? "" : policyBuildingID
      const scopedAreaID = policyScopeType === "area" || policyScopeType === "door" ? policyAreaID : ""
      const scopedDoorID = policyScopeType === "door" ? policyDoorID : ""
      const payload = {
        tenant_id: selectedTenantID,
        name: policyName.trim(),
        scope_type: policyScopeType,
        building_id: scopedBuildingID || undefined,
        area_id: scopedAreaID || undefined,
        door_id: scopedDoorID || undefined,
        schedule: policySchedule.trim() || undefined,
        members: Number.parseInt(policyMembers || "0", 10) || 0,
        status: policyStatus,
      }

      if (policyEditID) {
        const updated = await updateAccessPolicy(token, policyEditID, {
          name: payload.name,
          scope_type: payload.scope_type,
          building_id: payload.building_id,
          area_id: payload.area_id,
          door_id: payload.door_id,
          schedule: payload.schedule,
          members: payload.members,
          status: payload.status,
        })
        setPolicies((current) => current.map((item) => (item.id === updated.id ? updated : item)))
        setSummary(`已更新策略“${updated.name}”。`)
      } else {
        const created = await createAccessPolicy(token, payload)
        setPolicies((current) => [created, ...current])
        setSummary(`已创建策略“${created.name}”。`)
      }

      setPolicyEditID("")
      setPolicyName("")
      setPolicyScopeType("all")
      setPolicyBuildingID("")
      setPolicyAreaID("")
      setPolicyDoorID("")
      setPolicySchedule("")
      setPolicyStatus("active")
      setPolicyMembers("0")
    } catch (err) {
      const message = err instanceof Error ? err.message : "保存策略失败"
      setError(message)
    }
  }

  function editPolicy(item: AccessPolicy) {
    setPolicyEditID(item.id)
    setSelectedTenantID(item.tenant_id)
    setPolicyName(item.name)
    setPolicyScopeType((item.scope_type as ScopeType) || "all")
    setPolicyBuildingID(item.building_id ?? "")
    setPolicyAreaID(item.area_id ?? "")
    setPolicyDoorID(item.door_id ?? "")
    setPolicySchedule(item.schedule)
    setPolicyStatus((item.status as "active" | "inactive" | "draft") || "active")
    setPolicyMembers(String(item.members))
  }

  function applyPolicyStarter(starter: PolicyStarter) {
    setError("")
    setSummary(`已为“${starter.groupName}”填入策略草稿，请先复核范围和时间计划再保存。`)
    setPolicyEditID("")
    setPolicyName(starter.name)
    setPolicyScopeType(starter.scopeType)
    setPolicyBuildingID(starter.buildingID)
    setPolicyAreaID(starter.areaID)
    setPolicyDoorID(starter.doorID)
    setPolicySchedule(starter.schedule)
    setPolicyStatus("draft")
    setPolicyMembers(String(starter.members))
  }

  function applyPolicyStarterByID(starterID: string) {
    const starter = policyStarters.find((item) => item.id === starterID)
    if (!starter) {
      return
    }
    applyPolicyStarter(starter)
  }

  async function applyBatchPolicyStatus(nextStatus: "active" | "draft") {
    const targetRows = filteredPolicyLedgerRows.slice(0, 20)
    if (targetRows.length === 0) {
      setSummary("当前筛选结果没有可批量复核的策略。")
      return
    }

    setBatchUpdatingPolicyStatus(nextStatus)
    setError("")
    setSummary("")
    setPolicyBatchFlowHint("")
    setPolicyBatchTargetIDsHint([])
    try {
      const settled = await Promise.allSettled(
        targetRows.map((row) =>
          updateAccessPolicy(token, row.policy.id, {
            name: row.policy.name,
            scope_type: (row.policy.scope_type as ScopeType) || "all",
            building_id: row.policy.building_id || undefined,
            area_id: row.policy.area_id || undefined,
            door_id: row.policy.door_id || undefined,
            schedule: row.policy.schedule || undefined,
            members: row.policy.members,
            status: nextStatus,
          })
        )
      )

      const updatedPolicies = settled
        .filter((item): item is PromiseFulfilledResult<AccessPolicy> => item.status === "fulfilled")
        .map((item) => item.value)
      const updatedByID = new Map(updatedPolicies.map((item) => [item.id, item]))
      if (updatedByID.size > 0) {
        setPolicies((current) => current.map((item) => updatedByID.get(item.id) ?? item))
      }

      const failedCount = settled.length - updatedPolicies.length
      const targetLabel = nextStatus === "active" ? "启用" : "草稿"
      setSummary(
        `已批量将 ${settled.length} 条命中策略设为${targetLabel}，成功 ${updatedPolicies.length} 条${
          failedCount > 0 ? `，失败 ${failedCount} 条` : ""
        }。`
      )
      if (nextStatus === "active" && updatedPolicies.length > 0) {
        const policyNames = updatedPolicies.map((item) => item.name.trim().toLowerCase()).filter(Boolean)
        const inferredTargetIDs = Array.from(
          new Set(
            tenantGroups
              .filter((group) => {
                const groupName = group.name.trim().toLowerCase()
                if (!groupName) {
                  return false
                }
                return policyNames.some((policyName) => policyName.includes(groupName))
              })
              .flatMap((group) => group.members ?? [])
              .map((item) => item.trim())
              .filter(Boolean)
          )
        ).slice(0, 20)
        setPolicyBatchTargetIDsHint(inferredTargetIDs)
        setPolicyBatchFlowHint(
          inferredTargetIDs.length > 0
            ? `本轮已启用 ${updatedPolicies.length} 条策略，并预填 ${inferredTargetIDs.length} 个发放对象，可直接回流到凭证发放继续做批量补发、状态修复和失败通道重发。`
            : `本轮已启用 ${updatedPolicies.length} 条策略，可直接回流到凭证发放继续做批量补发、状态修复和失败通道重发。`
        )
      }
      if (failedCount > 0) {
        setError(`有 ${failedCount} 条策略更新失败，请稍后重试或逐条复核。`)
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "批量更新策略状态失败"
      setError(message)
    } finally {
      setBatchUpdatingPolicyStatus("")
    }
  }

  function toggleGroupMember(employeeID: string) {
    setSelectedGroupMemberIDs((current) => {
      if (current.includes(employeeID)) {
        return current.filter((item) => item !== employeeID)
      }
      return [...current, employeeID]
    })
  }

  async function submitGroup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedTenantID || !groupName.trim()) {
      return
    }

    setError("")
    setSummary("")
    try {
      const members = selectedGroupMemberIDs.filter(Boolean)

      if (groupEditID) {
        const updated = await updateUserGroup(token, groupEditID, {
          name: groupName.trim(),
          description: groupDescription.trim() || undefined,
          members,
        })
        setUserGroups((current) => current.map((item) => (item.id === updated.id ? updated : item)))
        setSummary(`已更新用户组“${updated.name}”。`)
      } else {
        const created = await createUserGroup(token, {
          tenant_id: selectedTenantID,
          name: groupName.trim(),
          description: groupDescription.trim() || undefined,
          members,
        })
        setUserGroups((current) => [created, ...current])
        setSummary(`已创建用户组“${created.name}”。`)
      }

      setGroupEditID("")
      setGroupName("")
      setGroupDescription("")
      setSelectedGroupMemberIDs([])
      setGroupMemberQuery("")
    } catch (err) {
      const message = err instanceof Error ? err.message : "保存用户组失败"
      setError(message)
    }
  }

  async function createStarterGroups() {
    if (!selectedTenantID || missingStarterGroupSpecs.length === 0) {
      return
    }

    setError("")
    setSummary("")
    try {
      const settled = await Promise.allSettled(
        missingStarterGroupSpecs.map((item) =>
          createUserGroup(token, {
            tenant_id: selectedTenantID,
            name: item.defaultGroup,
            description: item.permissionPreset,
            members: inferStarterGroupMemberIDs(item, employees),
          })
        )
      )

      const createdGroups = settled
        .filter((item): item is PromiseFulfilledResult<UserGroup> => item.status === "fulfilled")
        .map((item) => item.value)
      const failedCount = settled.length - createdGroups.length

      if (createdGroups.length > 0) {
        setUserGroups((current) => [...createdGroups, ...current])
      }
      if (createdGroups.length === 0) {
        setError("基础用户组创建失败，请检查当前组织权限或稍后重试。")
        return
      }

      setSummary(`已快速创建 ${createdGroups.length} 个基础用户组${failedCount > 0 ? `，失败 ${failedCount} 个` : ""}。`)
    } catch (err) {
      const message = err instanceof Error ? err.message : "快速创建基础用户组失败"
      setError(message)
    }
  }

  function editGroup(item: UserGroup) {
    setGroupEditID(item.id)
    setSelectedTenantID(item.tenant_id)
    setGroupName(item.name)
    setGroupDescription(item.description)
    setSelectedGroupMemberIDs(item.members ?? [])
  }

  async function submitGrant(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selectedTenantID || !granteeName.trim() || !granteePhone.trim() || !granteeEmail.trim() || !validUntil.trim()) {
      return
    }
    const scopeError = validateScope(grantScopeType, grantBuildingID, grantAreaID, grantDoorID)
    if (scopeError) {
      setError(scopeError)
      return
    }

    setError("")
    setSummary("")
    try {
      const scopedBuildingID = grantScopeType === "all" ? "" : grantBuildingID
      const scopedAreaID = grantScopeType === "area" || grantScopeType === "door" ? grantAreaID : ""
      const scopedDoorID = grantScopeType === "door" ? grantDoorID : ""
      const created = await createTemporaryAccess(token, {
        tenant_id: selectedTenantID,
        scope_type: grantScopeType,
        building_id: scopedBuildingID || undefined,
        area_id: scopedAreaID || undefined,
        door_id: scopedDoorID || undefined,
        delivery_method: grantMethod,
        grantee_name: granteeName.trim(),
        grantee_gender: granteeGender.trim() || undefined,
        grantee_phone: granteePhone.trim(),
        grantee_email: granteeEmail.trim(),
        mobile_model: mobileModel.trim() || undefined,
        pass_type: passType.trim() || undefined,
        valid_until: validUntil.trim(),
      })
      setGrants((current) => [created, ...current])
      setSummary(`已创建 ${created.grantee_name} 的临时授权。`)

      setGrantScopeType("door")
      setGrantBuildingID("")
      setGrantAreaID("")
      setGrantDoorID("")
      setGrantMethod("wallet")
      setGranteeName("")
      setGranteeGender("")
      setGranteePhone("")
      setGranteeEmail("")
      setMobileModel("")
      setPassType("employee")
      setValidUntil("")
    } catch (err) {
      const message = err instanceof Error ? err.message : "创建授权失败"
      setError(message)
    }
  }

  function applyGrantStarter(starter: GrantStarter) {
    setError("")
    setSummary(`已套用“${starter.title}”场景，请补齐被授权人信息后创建授权。`)
    setGrantScopeType(starter.scopeType)
    setGrantBuildingID(starter.buildingID)
    setGrantAreaID(starter.areaID)
    setGrantDoorID(starter.doorID)
    setGrantMethod(starter.deliveryMethod)
    setPassType(starter.passType)
    setValidUntil(starter.validUntil)
  }

  const tenantPolicies = useMemo(
    () => policies.filter((item) => (selectedTenantID ? item.tenant_id === selectedTenantID : true)),
    [policies, selectedTenantID]
  )
  const tenantGroups = useMemo(
    () => userGroups.filter((item) => (selectedTenantID ? item.tenant_id === selectedTenantID : true)),
    [selectedTenantID, userGroups]
  )
  const missingStarterGroupSpecs = useMemo(() => {
    const existingNames = new Set(tenantGroups.map((item) => item.name.trim().toLowerCase()).filter(Boolean))
    return positionTemplateSpec.filter((item) => !existingNames.has(item.defaultGroup.trim().toLowerCase()))
  }, [tenantGroups])
  const groupStarterCards = useMemo(
    () =>
      missingStarterGroupSpecs.map((item) => ({
        accessRole: item.accessRole,
        matchedMemberCount: inferStarterGroupMemberIDs(item, employees).length,
        name: item.defaultGroup,
        permissionPreset: item.permissionPreset,
        position: item.position,
      })),
    [employees, missingStarterGroupSpecs]
  )
  const policyStarters = useMemo(
    () =>
      buildPolicyStarters({
        areaByID,
        areas,
        buildingByID,
        buildings,
        doorByID,
        doors,
        tenantGroups,
        tenantPolicies,
      }),
    [areaByID, areas, buildingByID, buildings, doorByID, doors, tenantGroups, tenantPolicies]
  )
  const policyStarterCards = useMemo(
    () =>
      policyStarters.map((starter) => ({
        id: starter.id,
        groupName: starter.groupName,
        memberCount: starter.members,
        name: starter.name,
        description: starter.description,
        reviewNote: starter.reviewNote,
        schedule: starter.schedule,
      })),
    [policyStarters]
  )
  const grantStarters = useMemo(
    () =>
      buildGrantStarters({
        areaByID,
        areas,
        buildingByID,
        buildings,
        doorByID,
        doors,
      }),
    [areaByID, areas, buildingByID, buildings, doorByID, doors]
  )
  const tenantGrantView = useMemo(
    () =>
      buildTenantGrantViewModel({
        grantDateFrom,
        grantDateTo,
        grantMethodFilter,
        grantPassTypeFilter,
        grantStatusFilter,
        grants,
        nowTick,
        selectedTenantID,
      }),
    [
      grantDateFrom,
      grantDateTo,
      grantMethodFilter,
      grantPassTypeFilter,
      grantStatusFilter,
      grants,
      nowTick,
      selectedTenantID,
    ]
  )
  const {
    activeGrantCount,
    expiredGrantCount,
    expiringSoonGrantCount,
    filteredGrantLedger,
    grantPassTypeOptions,
    tenantGrants,
    visitorGrantCount,
  } = tenantGrantView
  const activeEmployeeCount = useMemo(
    () => employees.filter((item) => item.status === "active").length,
    [employees]
  )
  const walletFilteredGrantLink = useMemo(() => {
    if (filteredGrantLedger.length === 0) {
      return ""
    }
    const firstGrant = filteredGrantLedger[0]
    const query = new URLSearchParams(location.search)
    query.set("scenario", "visitor_temporary")
    applyEnterpriseWalletContext({
      context: enterpriseFlowContext,
      flowMemberEmailHint,
      flowMemberIDHint,
      flowMemberNameHint,
      query,
      selectedTenantID,
      targetHint: "visitor",
    })
    const effectiveTenantID = selectedTenantID.trim() || firstGrant.tenant_id.trim()
    if (effectiveTenantID) {
      query.set("tenant_id", effectiveTenantID)
    }
    if (firstGrant.grantee_email.trim()) {
      query.set("target_email", firstGrant.grantee_email.trim())
    }
    if (firstGrant.grantee_name.trim()) {
      query.set("target_name", firstGrant.grantee_name.trim())
    }
    const passTypeRaw = firstGrant.pass_type?.trim() || ""
    if (passTypeRaw) {
      query.set("grant_pass_type", passTypeRaw)
    }
    const passTypeHint = passTypeRaw.toLowerCase()
    const targetHint = passTypeHint.includes("visitor") || passTypeHint.includes("guest") ? "visitor" : "user"
    query.set("target_hint", targetHint)
    const nextQuery = query.toString()
    return nextQuery ? `/wallet?${nextQuery}` : "/wallet"
  }, [
    enterpriseFlowContext,
    filteredGrantLedger,
    flowMemberEmailHint,
    flowMemberIDHint,
    flowMemberNameHint,
    location.search,
    selectedTenantID,
  ])
  const grantFiltersActive = grantMethodFilter !== "all" || grantPassTypeFilter !== "all" || grantStatusFilter !== "all"
  const grantLedgerRows = useMemo<AccessGrantLedgerRow[]>(
    () =>
      filteredGrantLedger.map((grant) => {
        const lifecycleStatus = getGrantLifecycleStatus(grant.valid_until, nowTick)
        const remaining = remainingLabel(grant.valid_until, nowTick)
        return {
          grant,
          tenantLabel: tenantByID.get(grant.tenant_id)?.name ?? grant.tenant_id,
          scopeLabel: scopeSummary(
            grant.scope_type,
            buildingByID.get(grant.building_id || "")?.name ?? grant.building_id,
            areaByID.get(grant.area_id || "")?.name ?? grant.area_id,
            doorByID.get(grant.door_id || "")?.name ?? grant.door_id
          ),
          granteeLabel: `${grant.grantee_name}${grant.grantee_gender ? `（${grant.grantee_gender}）` : ""}`,
          deliveryLabel: deliveryLabel(grant.delivery_method),
          authorizedByRole: grant.authorized_by_role || "-",
          authorizedByEmail: grant.authorized_by_email || "-",
          authorizedAtLabel: new Date(grant.authorized_at || grant.created_at).toLocaleString("zh-CN"),
          statusLabel: grantLifecycleLabel(lifecycleStatus),
          statusVariant: grantLifecycleBadgeVariant(lifecycleStatus),
          validUntilLabel: grant.valid_until,
          remainingLabel: remaining,
          remainingVariant: remaining === "已到期" ? "destructive" : "outline",
        }
      }),
    [areaByID, buildingByID, doorByID, filteredGrantLedger, nowTick, tenantByID]
  )
  const grantLedgerEmptyState =
    tenantGrants.length === 0
      ? "暂无临时或访客授权记录。短期通行可在左侧创建，长期员工和批量发放建议前往“凭证发放”页面。"
      : "当前筛选条件下没有匹配的授权记录，可调整方式、对象类型、状态或日期范围。"
  const directoryReady = activeEmployeeCount > 0 && tenantGroups.length > 0
  const groupLedgerRows = useMemo(
    () =>
      buildGroupLedgerRows({
        employeeByID,
        tenantGroups,
      }),
    [employeeByID, tenantGroups]
  )
  const groupLedgerEmptyState = deriveGroupLedgerEmptyState(employees.length)
  const policyReady = tenantPolicies.length > 0
  const topologyReady = buildings.length > 0 && areas.length > 0 && doors.length > 0
  const issuanceReady = directoryReady && policyReady
  const policyLedgerRows = useMemo(
    () =>
      buildPolicyLedgerRows({
        areaByID,
        buildingByID,
        doorByID,
        tenantPolicies,
      }),
    [areaByID, buildingByID, doorByID, tenantPolicies]
  )
  const suggestedPolicyLedgerQuery = useMemo(
    () =>
      deriveSuggestedPolicyLedgerQuery({
        enterpriseFlowContext,
        hintedMemberGroupName: hintedMemberGroup?.name,
      }),
    [enterpriseFlowContext, hintedMemberGroup?.name]
  )
  const filteredPolicyLedgerRows = useMemo(
    () =>
      filterPolicyLedgerRows({
        policyLedgerQuery,
        policyLedgerRows,
      }),
    [policyLedgerQuery, policyLedgerRows]
  )
  const policyLedgerQueryActive = policyLedgerQuery.trim().length > 0
  const policyLedgerEmptyState = derivePolicyLedgerEmptyState({
    directoryReady,
    policyLedgerQueryActive,
    topologyReady,
  })
  const syncRecordLabel = enterpriseFlowContext ? buildEnterpriseSyncRecordLabel(enterpriseFlowContext) : ""
  const workerAlertLabel = enterpriseFlowContext
    ? buildEnterpriseWorkerAlertLabel({
        context: enterpriseFlowContext,
        selectedTenantID,
      })
    : ""
  const hintedMemberLabel = enterpriseFlowContext
    ? deriveEnterpriseHintedMemberLabel({
        context: enterpriseFlowContext,
        hintedGroupMember,
      })
    : ""
  const directoryRemediationLabel = enterpriseFlowContext
    ? deriveEnterpriseRemediationLabel({
        context: enterpriseFlowContext,
        deactivatedLabel: "停用对象清理",
        normalLabel: "成员复核",
      })
    : "成员复核"
  const policyRemediationLabel = enterpriseFlowContext
    ? deriveEnterpriseRemediationLabel({
        context: enterpriseFlowContext,
        deactivatedLabel: "停用对象清理",
        normalLabel: "成员承接",
      })
    : "成员承接"
  const enterpriseSummaryTail = buildEnterpriseSummaryTail({
    syncRecordLabel,
    workerAlertLabel,
  })
  const enterpriseSyncCompactTail = buildEnterpriseSummaryTail({
    syncLabelPrefix: "",
    syncRecordLabel,
    workerAlertLabel: "",
  })
  const enterpriseWorkerCompactTail = buildEnterpriseSummaryTail({
    syncRecordLabel: "",
    workerLabelPrefix: "",
    workerAlertLabel,
  })
  const setEnterpriseSummary = (message: string) => {
    setSummary(buildEnterpriseFlowSummary(message))
  }
  const applyHintedGroupMemberQuery = () => {
    if (hintedMemberLabel) {
      setGroupMemberQuery(hintedMemberLabel)
    }
  }

  useEffect(() => {
    if (!enterpriseFlowContext || enterpriseFlowContext.flow !== "sync_to_access") {
      if (enterpriseStagePresetApplied) {
        setEnterpriseStagePresetApplied("")
      }
      return
    }

    const stage = enterpriseFlowContext.stage
    const stageKey = buildEnterpriseStagePresetKey({
      search: location.search,
      stage,
    })
    if (enterpriseStagePresetApplied === stageKey) {
      return
    }
    const stageRoute = resolveEnterpriseAccessStageRoute(stage)
    if (stageRoute && activeSection !== stageRoute.section) {
      navigate(
        {
          pathname: stageRoute.pathname,
          search: location.search,
        },
        { replace: true }
      )
    }

    if (stage === "directory") {
      if (!groupEditID) {
        const groupNameHint = enterpriseFlowContext.groupName
        const hintedGroup = findByNameHint(tenantGroups, groupNameHint)

        if (hintedGroup) {
          editGroup(hintedGroup)
          applyHintedGroupMemberQuery()
          setEnterpriseSummary(
            `已定位到用户组“${hintedGroup.name}”${
              hintedMemberLabel ? `，并承接成员线索“${hintedMemberLabel}”` : ""
            }${enterpriseSummaryTail}，可直接进行${directoryRemediationLabel}并继续下游流程。`
          )
        } else if (hintedMemberGroup) {
          editGroup(hintedMemberGroup)
          applyHintedGroupMemberQuery()
          setEnterpriseSummary(
            `已按成员线索定位到用户组“${hintedMemberGroup.name}”${
              hintedMemberLabel ? `（${hintedMemberLabel}）` : ""
            }${enterpriseSummaryTail}，可直接进行${directoryRemediationLabel}。`
          )
        } else if (!groupName.trim() && groupNameHint) {
          setGroupName(groupNameHint)
          setGroupDescription(enterpriseFlowContext.groupDesc || "来源企业页同步承接草稿")
          setSelectedGroupMemberIDs(hintedGroupMember ? [hintedGroupMember.id] : [])
          applyHintedGroupMemberQuery()
          setEnterpriseSummary(
            `已预填“${groupNameHint}”用户组草稿${
              hintedMemberLabel ? `，并定位成员“${hintedMemberLabel}”` : ""
            }${enterpriseSummaryTail}，请先完成${directoryRemediationLabel}后保存。`
          )
        } else if (!groupName.trim() && enterpriseFlowContext.syncSource) {
          const syncDraft = buildEnterpriseSyncGroupDraft({
            syncJobID: enterpriseFlowContext.syncJobID,
            syncSource: enterpriseFlowContext.syncSource,
            syncStatus: enterpriseFlowContext.syncStatus,
          })
          setGroupName(syncDraft.name)
          setGroupDescription(syncDraft.description)
          applyHintedGroupMemberQuery()
          setEnterpriseSummary(
            `已按同步异常记录预填用户组草稿${
              enterpriseSyncCompactTail
            }，可直接进行${directoryRemediationLabel}并继续下游流程。`
          )
        } else if (!groupName.trim() && enterpriseFlowContext.workerAlertLevel) {
          const workerDraft = buildEnterpriseWorkerGroupDraft({
            selectedTenantID,
            workerAlertLabel,
            workerAlertLastSeen: enterpriseFlowContext.workerAlertLastSeen,
            workerAlertTenantID: enterpriseFlowContext.workerAlertTenantID,
          })
          setGroupName(workerDraft.name)
          setGroupDescription(workerDraft.description)
          applyHintedGroupMemberQuery()
          setEnterpriseSummary(
            `已按 worker 告警记录预填用户组草稿${
              enterpriseWorkerCompactTail
            }，请先复核目录再继续主流程。`
          )
        } else if (!groupName.trim() && missingStarterGroupSpecs.length > 0) {
          const starter = missingStarterGroupSpecs[0]
          const starterMemberIDs = inferStarterGroupMemberIDs(starter, employees)
          const mergedMemberIDs =
            hintedGroupMember && !starterMemberIDs.includes(hintedGroupMember.id)
              ? [hintedGroupMember.id, ...starterMemberIDs]
              : starterMemberIDs
          setGroupName(starter.defaultGroup)
          setGroupDescription(starter.permissionPreset)
          setSelectedGroupMemberIDs(mergedMemberIDs)
          applyHintedGroupMemberQuery()
          setEnterpriseSummary(
            `已预填“${starter.defaultGroup}”用户组草稿${
              hintedMemberLabel ? `，并承接成员线索“${hintedMemberLabel}”` : ""
            }${enterpriseSummaryTail}，请先完成${directoryRemediationLabel}后保存。`
          )
        }
      }
    }

    if (stage === "policies") {
      if (!policyLedgerQuery.trim() && suggestedPolicyLedgerQuery.trim()) {
        setPolicyLedgerQuery(suggestedPolicyLedgerQuery.trim())
      }
      if (!policyEditID) {
        const policyNameHint = enterpriseFlowContext.policyName
        const policyGroupHint =
          enterpriseFlowContext.policyGroup || hintedMemberGroup?.name || enterpriseFlowContext.groupName
        const policyMatch = derivePolicyLedgerMatch({
          fallbackQuery: suggestedPolicyLedgerQuery,
          policyLedgerQuery,
          policyLedgerRows,
        })
        const firstMatchedPolicy = policyMatch.firstMatchedPolicy
        const hintedPolicy = findByNameHint(tenantPolicies, policyNameHint)
        const hintedStarter = findByGroupNameHint(policyStarters, policyGroupHint)

        if (hintedPolicy) {
          editPolicy(hintedPolicy)
          setEnterpriseSummary(
            `已定位到策略“${hintedPolicy.name}”${
              hintedMemberLabel ? `，并承接成员线索“${hintedMemberLabel}”` : ""
            }${enterpriseSummaryTail}，可继续完成${policyRemediationLabel}。`
          )
        } else if (!policyName.trim() && firstMatchedPolicy) {
          editPolicy(firstMatchedPolicy)
          setEnterpriseSummary(
            `已按当前线索命中 ${policyMatch.matchedRows.length} 条策略，并直达编辑“${firstMatchedPolicy.name}”${
              hintedMemberLabel ? `（成员线索：${hintedMemberLabel}）` : ""
            }${enterpriseSummaryTail}，可继续完成${policyRemediationLabel}。`
          )
        } else if (!policyName.trim() && hintedStarter) {
          applyPolicyStarter(hintedStarter)
          setEnterpriseSummary(
            `已定位到策略域，并套用“${hintedStarter.groupName}”策略草稿${
              hintedMemberLabel ? `（成员线索：${hintedMemberLabel}）` : ""
            }${enterpriseSummaryTail}，请复核范围后继续${policyRemediationLabel}。`
          )
        } else if (!policyName.trim() && policyNameHint) {
          setPolicyName(policyNameHint)
          setPolicyStatus("draft")
          setEnterpriseSummary(
            `已预填策略名称“${policyNameHint}”${
              hintedMemberLabel ? `（成员线索：${hintedMemberLabel}）` : ""
            }${enterpriseSummaryTail}，请补齐范围后继续${policyRemediationLabel}。`
          )
        } else if (!policyName.trim() && policyStarters.length > 0) {
          const starter =
            (hintedMemberGroup ? findByGroupNameHint(policyStarters, hintedMemberGroup.name) : null) ?? policyStarters[0]
          applyPolicyStarter(starter)
          setEnterpriseSummary(
            `已定位到策略域，并套用“${starter.groupName}”策略草稿${
              hintedMemberLabel ? `（成员线索：${hintedMemberLabel}）` : ""
            }${enterpriseSummaryTail}，请复核范围后继续${policyRemediationLabel}。`
          )
        } else if (!policyName.trim()) {
          setPolicyStatus("draft")
          setEnterpriseSummary(
            `已定位到策略域${hintedMemberLabel ? `（成员线索：${hintedMemberLabel}）` : ""}${
              enterpriseSummaryTail
            }，请优先创建首条权限策略。`
          )
        }
        if (!policyName.trim() && enterpriseFlowContext.workerAlertLevel && !policyNameHint && !hintedStarter && !firstMatchedPolicy) {
          setPolicyName(
            buildEnterpriseWorkerPolicyDraftName({
              selectedTenantID,
              workerAlertTenantID: enterpriseFlowContext.workerAlertTenantID,
            })
          )
          setPolicyStatus("draft")
          setEnterpriseSummary(
            `已按 worker 告警记录预填策略草稿${
              enterpriseWorkerCompactTail
            }，请优先创建首条权限策略。`
          )
        }
      }
    }

    if (stage === "issuance") {
      if (enterpriseFlowContext.grantPassType) {
        setPassType(enterpriseFlowContext.grantPassType)
      }
      const targetNameHint = enterpriseFlowContext.targetName
      const targetEmailHint =
        enterpriseFlowContext.targetEmail ||
        (enterpriseFlowContext.targetID.includes("@") ? enterpriseFlowContext.targetID : "")
      const targetLabel = targetNameHint || targetEmailHint || enterpriseFlowContext.targetID
      if (!granteeName.trim() && targetNameHint) {
        setGranteeName(targetNameHint)
      }
      if (!granteeEmail.trim() && targetEmailHint) {
        setGranteeEmail(targetEmailHint)
      }
      setEnterpriseSummary(
        `${
          targetLabel ? `已承接对象“${targetLabel}”。` : ""
        }长期员工发放建议直接前往凭证发放；临时与访客授权可继续在当前域处理。`
      )
    }

    setEnterpriseStagePresetApplied(stageKey)
  }, [
    activeSection,
    employees,
    enterpriseFlowContext,
    enterpriseStagePresetApplied,
    granteeEmail,
    granteeName,
    groupEditID,
    groupName,
    hintedGroupMember,
    hintedMemberGroup,
    location.search,
    missingStarterGroupSpecs,
    navigate,
    policyEditID,
    policyLedgerRows,
    policyLedgerQuery,
    policyName,
    policyStarters,
    suggestedPolicyLedgerQuery,
    tenantGroups,
    tenantPolicies,
  ])

  useEffect(() => {
    if (!enterpriseFlowContext || !flowSegmentDescriptor) {
      return
    }
    const normalizedSummary = summary.trim()
    if (!normalizedSummary || !normalizedSummary.startsWith("来源：企业页。") || normalizedSummary.includes("分段提示：")) {
      return
    }
    const withoutTailPunctuation = normalizedSummary.replace(/[。！？!?]+$/g, "")
    setSummary(`${withoutTailPunctuation}（分段提示：${flowSegmentDescriptor}）。`)
  }, [enterpriseFlowContext, flowSegmentDescriptor, summary])

  const nextRecommendedAction = useMemo(
    () =>
      deriveNextRecommendedAction({
        directorySectionLink,
        employeeCount: employees.length,
        enterpriseSyncLink,
        groupCount: tenantGroups.length,
        policiesSectionLink,
        policyReady,
        spacesLink,
        topologyReady,
        walletEmployeeLink,
      }),
    [
      directorySectionLink,
      employees.length,
      enterpriseSyncLink,
      policiesSectionLink,
      policyReady,
      spacesLink,
      tenantGroups.length,
      topologyReady,
      walletEmployeeLink,
    ]
  )
  const accessSections = useMemo(
    () =>
      buildAccessSectionsOverview({
        activeEmployeeCount,
        employeeCount: employees.length,
        expiredGrantCount,
        grantCount: tenantGrants.length,
        groupCount: tenantGroups.length,
        loading,
        policyCount: tenantPolicies.length,
        visitorGrantCount,
      }),
    [
      activeEmployeeCount,
      employees.length,
      expiredGrantCount,
      loading,
      tenantGrants.length,
      tenantGroups.length,
      tenantPolicies.length,
      visitorGrantCount,
    ]
  )

  useEffect(() => {
    if (activeSectionOverride) {
      return
    }
    if (!pathnameSection || !isAccessSection(pathnameSection)) {
      navigate(
        {
          pathname: "/access/directory",
          search: location.search,
        },
        { replace: true }
      )
    }
  }, [activeSectionOverride, location.search, navigate, pathnameSection])

  function goToSection(next: AccessSection) {
    if (next === "directory") {
      navigate(directorySectionLink)
      return
    }
    if (next === "policies") {
      navigate(policiesSectionLink)
      return
    }
    navigate(grantsSectionLink)
  }

  function resetGrantFilters() {
    setGrantDateFrom("")
    setGrantDateTo("")
    setGrantMethodFilter("all")
    setGrantPassTypeFilter("all")
    setGrantStatusFilter("all")
  }

  function applyGrantStarterByID(starterID: string) {
    const starter = grantStarters.find((item) => item.id === starterID)
    if (!starter) {
      return
    }
    applyGrantStarter(starter)
  }

  const roleTemplateItems = positionTemplateSpec.map((item) => ({
    accessRole: item.accessRole,
    defaultGroup: item.defaultGroup,
    permissionPreset: item.permissionPreset,
    position: item.position,
  }))

  const grantStarterItems = grantStarters.map((starter) => ({
    id: starter.id,
    title: starter.title,
    deliveryLabel: deliveryLabel(starter.deliveryMethod),
    passType: starter.passType,
    description: starter.description,
    reviewNote: starter.reviewNote,
    validUntilLabel: starter.validUntil.replace("T", " "),
  }))

  const directorySectionProps = {
    filteredEmployees: filteredEmployees.map((employee) => ({
      email: employee.email,
      fullName: employee.full_name,
      id: employee.id,
    })),
    groupDescription,
    groupLedgerEmptyState,
    groupLedgerRows,
    groupMemberQuery,
    groupName,
    isEditingGroup: Boolean(groupEditID),
    onCreateStarterGroups: () => void createStarterGroups(),
    onDescriptionChange: setGroupDescription,
    onEditGroup: editGroup,
    onMemberQueryChange: setGroupMemberQuery,
    onNameChange: setGroupName,
    onSubmitGroup: submitGroup,
    onToggleMember: toggleGroupMember,
    roleTemplateItems,
    selectedMemberIDs: selectedGroupMemberIDs,
    showStarterPanel: missingStarterGroupSpecs.length > 0,
    starterItems: groupStarterCards,
  }

  const policiesSectionProps = {
    areaID: policyAreaID,
    areaOptions: policyAreaOptions,
    batchActionPending: batchUpdatingPolicyStatus,
    batchFlowHint: policyBatchFlowHint,
    buildingID: policyBuildingID,
    buildingOptions: buildings,
    doorID: policyDoorID,
    doorOptions: policyDoorOptions,
    emptyState: policyLedgerEmptyState,
    hasGroups: tenantGroups.length > 0,
    hasLedgerQuery: policyLedgerQueryActive,
    isEditing: Boolean(policyEditID),
    ledgerFilteredCount: filteredPolicyLedgerRows.length,
    ledgerQuery: policyLedgerQuery,
    ledgerRows: filteredPolicyLedgerRows,
    ledgerTotalCount: policyLedgerRows.length,
    members: policyMembers,
    name: policyName,
    onApplyStarter: applyPolicyStarterByID,
    onAreaIDChange: setPolicyAreaID,
    onBatchSetActive: () => void applyBatchPolicyStatus("active"),
    onBatchSetDraft: () => void applyBatchPolicyStatus("draft"),
    onBuildingIDChange: setPolicyBuildingID,
    onClearLedgerQuery: () => setPolicyLedgerQuery(""),
    onDoorIDChange: setPolicyDoorID,
    onEditPolicy: editPolicy,
    onLedgerQueryChange: setPolicyLedgerQuery,
    onMembersChange: setPolicyMembers,
    onNameChange: setPolicyName,
    onScheduleChange: setPolicySchedule,
    onScopeTypeChange: setPolicyScopeType,
    onStatusChange: setPolicyStatus,
    onSubmitPolicy: submitPolicy,
    schedule: policySchedule,
    spacesLink,
    scopeSummaryLabel: scopeSummary(
      policyScopeType,
      buildingByID.get(policyBuildingID)?.name,
      areaByID.get(policyAreaID)?.name,
      doorByID.get(policyDoorID)?.name
    ),
    scopeType: policyScopeType,
    starterItems: policyStarterCards,
    status: policyStatus,
    topologyReady,
    grantsLink: grantsSectionLink,
    issuanceLink: walletEmployeeLink,
  }

  const grantsSectionProps = {
    activeCount: activeGrantCount,
    activeGrant,
    areaID: grantAreaID,
    areaOptions: grantAreaOptions,
    buildingID: grantBuildingID,
    buildingOptions: buildings,
    dateFrom: grantDateFrom,
    dateTo: grantDateTo,
    deliveryMethod: grantMethod,
    doorID: grantDoorID,
    doorOptions: grantDoorOptions,
    emptyState: grantLedgerEmptyState,
    expiredCount: expiredGrantCount,
    expiringSoonCount: expiringSoonGrantCount,
    filtersActive: grantFiltersActive,
    filteredCount: filteredGrantLedger.length,
    grantRows: grantLedgerRows,
    granteeEmail,
    granteeGender,
    granteeName,
    granteePhone,
    methodFilter: grantMethodFilter,
    mobileModel,
    onActiveGrantChange: setActiveGrant,
    onAreaChange: setGrantAreaID,
    onBuildingChange: setGrantBuildingID,
    onDateFromChange: setGrantDateFrom,
    onDateToChange: setGrantDateTo,
    onDeliveryMethodChange: setGrantMethod,
    onDoorChange: setGrantDoorID,
    onGranteeEmailChange: setGranteeEmail,
    onGranteeGenderChange: setGranteeGender,
    onGranteeNameChange: setGranteeName,
    onGranteePhoneChange: setGranteePhone,
    onMethodChange: (value: string) => setGrantMethodFilter(value as "all" | DeliveryMethod),
    onMobileModelChange: setMobileModel,
    onOpenGrant: setActiveGrant,
    onPassTypeChange: setPassType,
    onPassTypeFilterChange: setGrantPassTypeFilter,
    onResetFilters: resetGrantFilters,
    onScopeTypeChange: setGrantScopeType,
    onStarterApply: applyGrantStarterByID,
    onStatusChange: setGrantStatusFilter,
    onSubmitGrant: submitGrant,
    onValidUntilChange: setValidUntil,
    passType,
    passTypeFilter: grantPassTypeFilter,
    passTypeOptions: grantPassTypeOptions,
    platformViewer,
    scopeSummaryLabel: scopeSummary(
      grantScopeType,
      buildingByID.get(grantBuildingID)?.name,
      areaByID.get(grantAreaID)?.name,
      doorByID.get(grantDoorID)?.name
    ),
    scopeType: grantScopeType,
    starters: grantStarterItems,
    statusFilter: grantStatusFilter,
    validUntil,
    visitorCount: visitorGrantCount,
    walletLink: walletVisitorLink,
    walletFilteredLink: grantFiltersActive ? walletFilteredGrantLink : "",
  }
  const effectiveError = error || queryError

  return (
    <div className="space-y-6">
      <AccessPageHeader
        platformViewer={platformViewer}
        selectedTenantID={selectedTenantID}
        tenants={tenants}
        onTenantChange={setSelectedTenantID}
      />

      <AccessDomainMetricsCards
        loading={loading}
        policyCount={tenantPolicies.length}
        activeEmployeeCount={activeEmployeeCount}
        groupCount={tenantGroups.length}
        grantCount={tenantGrants.length}
        visitorGrantCount={visitorGrantCount}
        expiredGrantCount={expiredGrantCount}
      />

      <AccessOperationFeedback error={effectiveError} summary={summary} />

      <AccessSectionOverviewCards
        sections={accessSections}
        activeSection={activeSection}
        onGoToSection={goToSection}
        showDirectoryImportAction={employees.length === 0}
        enterpriseHomeLink={enterpriseHomeLink}
      />

      <AccessDomainBanner
        title={nextRecommendedAction.title}
        description={nextRecommendedAction.description}
        actions={
          <AccessDomainBannerActions
            primaryActionLabel={nextRecommendedAction.label}
            primaryActionTo={nextRecommendedAction.to}
            enterpriseHomeLink={enterpriseHomeLink}
            walletEmployeeLink={walletEmployeeLink}
            hasWorkerAlertFlowHints={hasWorkerAlertFlowHints}
            enterpriseSyncWorkerReviewLink={enterpriseSyncWorkerReviewLink}
          />
        }
      />

      <AccessReadinessOverviewCards
        activeEmployeeCount={activeEmployeeCount}
        groupCount={tenantGroups.length}
        policyCount={tenantPolicies.length}
        buildingCount={buildings.length}
        areaCount={areas.length}
        doorCount={doors.length}
        visitorGrantCount={visitorGrantCount}
        expiredGrantCount={expiredGrantCount}
        directoryReady={directoryReady}
        policyReady={policyReady}
        topologyReady={topologyReady}
        issuanceReady={issuanceReady}
        hasEmployees={employees.length > 0}
        enterpriseSyncLink={enterpriseSyncLink}
        directorySectionLink={directorySectionLink}
        spacesLink={spacesLink}
        policiesSectionLink={policiesSectionLink}
        walletEmployeeLink={walletEmployeeLink}
        grantsSectionLink={grantsSectionLink}
      />

      <AccessSectionsTabs
        activeSection={activeSection}
        onSectionChange={goToSection}
        directoryProps={directorySectionProps}
        policiesProps={policiesSectionProps}
        grantsProps={grantsSectionProps}
      />
    </div>
  )
}
