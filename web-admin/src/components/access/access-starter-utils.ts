import {
  formatDateTimeLocalInput,
  inferPolicyStarterName,
  inferPolicyStarterSchedule,
  inferPolicyStarterScope,
  inferPositionTemplateByGroupName,
  scopeSummary,
  type DeliveryMethod,
  type ScopeType,
} from "@/components/access/access-page-utils"
import type { AccessPolicy, Area, Building, Door, UserGroup } from "@/lib/api"

export type PolicyStarter = {
  id: string
  groupName: string
  title: string
  description: string
  name: string
  scopeType: ScopeType
  buildingID: string
  areaID: string
  doorID: string
  schedule: string
  members: number
  reviewNote: string
}

export type GrantStarter = {
  id: string
  title: string
  description: string
  scopeType: ScopeType
  buildingID: string
  areaID: string
  doorID: string
  deliveryMethod: DeliveryMethod
  passType: string
  validUntil: string
  reviewNote: string
}

function resolveTopologySeed(buildings: Building[], areas: Area[], doors: Door[]) {
  const firstBuilding = buildings[0]
  const firstArea = areas.find((item) => item.building_id === firstBuilding?.id) ?? areas[0]
  const firstDoor =
    doors.find((item) => item.area_id === firstArea?.id) ??
    doors.find((item) => item.building_id === firstBuilding?.id) ??
    doors[0]
  return { firstArea, firstBuilding, firstDoor }
}

function hasIncompleteTopology(scopeType: ScopeType, buildingID: string, areaID: string, doorID: string) {
  return (
    (scopeType === "building" && !buildingID) ||
    (scopeType === "area" && !(buildingID && areaID)) ||
    (scopeType === "door" && !(buildingID && areaID && doorID))
  )
}

function scopeSummaryByID(
  scopeType: ScopeType,
  buildingID: string,
  areaID: string,
  doorID: string,
  buildingByID: Map<string, Building>,
  areaByID: Map<string, Area>,
  doorByID: Map<string, Door>
) {
  return scopeSummary(scopeType, buildingByID.get(buildingID)?.name, areaByID.get(areaID)?.name, doorByID.get(doorID)?.name)
}

export function buildPolicyStarters({
  areaByID,
  areas,
  buildingByID,
  buildings,
  doorByID,
  doors,
  tenantGroups,
  tenantPolicies,
}: {
  areaByID: Map<string, Area>
  areas: Area[]
  buildingByID: Map<string, Building>
  buildings: Building[]
  doorByID: Map<string, Door>
  doors: Door[]
  tenantGroups: UserGroup[]
  tenantPolicies: AccessPolicy[]
}) {
  const { firstArea, firstBuilding, firstDoor } = resolveTopologySeed(buildings, areas, doors)

  return tenantGroups
    .map((group) => {
      const template = inferPositionTemplateByGroupName(group.name)
      if (!template) {
        return null
      }

      const buildingID = firstBuilding?.id ?? ""
      const areaID = firstArea?.id ?? ""
      const doorID = firstDoor?.id ?? ""
      const scopeType = inferPolicyStarterScope(template.accessRole, buildingID, areaID, doorID)
      const targetBuildingID = scopeType === "all" ? "" : buildingID
      const targetAreaID = scopeType === "area" || scopeType === "door" ? areaID : ""
      const targetDoorID = scopeType === "door" ? doorID : ""
      const memberCount = (group.members ?? []).length
      const starterName = inferPolicyStarterName(group.name, template.accessRole)

      const policyAlreadyExists = tenantPolicies.some((policy) => {
        const haystack = `${policy.name} ${policy.scope_type} ${policy.building_id ?? ""} ${policy.area_id ?? ""} ${policy.door_id ?? ""}`
          .trim()
          .toLowerCase()
        return haystack.includes(group.name.trim().toLowerCase()) || haystack.includes(starterName.trim().toLowerCase())
      })

      if (policyAlreadyExists) {
        return null
      }

      const scopedSummary = scopeSummaryByID(
        scopeType,
        targetBuildingID,
        targetAreaID,
        targetDoorID,
        buildingByID,
        areaByID,
        doorByID
      )
      const reviewNote = hasIncompleteTopology(scopeType, targetBuildingID, targetAreaID, targetDoorID)
        ? "当前楼宇拓扑还不完整，先生成了保守草稿；保存前建议先回到空间页补齐楼宇、区域和门点。"
        : `建议范围：${scopedSummary}`

      return {
        id: group.id,
        groupName: group.name,
        title: template.defaultGroup,
        description: `${group.name} 已具备 ${memberCount} 名成员，可直接先套一条 ${scopedSummary} 策略草稿。`,
        name: starterName,
        scopeType,
        buildingID: targetBuildingID,
        areaID: targetAreaID,
        doorID: targetDoorID,
        schedule: inferPolicyStarterSchedule(template.accessRole),
        members: memberCount,
        reviewNote,
      } satisfies PolicyStarter
    })
    .filter((item): item is PolicyStarter => Boolean(item))
}

export function buildGrantStarters({
  areaByID,
  areas,
  buildingByID,
  buildings,
  doorByID,
  doors,
}: {
  areaByID: Map<string, Area>
  areas: Area[]
  buildingByID: Map<string, Building>
  buildings: Building[]
  doorByID: Map<string, Door>
  doors: Door[]
}) {
  const now = new Date()
  const addHours = (hours: number) => formatDateTimeLocalInput(new Date(now.getTime() + hours * 60 * 60 * 1000))
  const addDays = (days: number) => formatDateTimeLocalInput(new Date(now.getTime() + days * 24 * 60 * 60 * 1000))
  const { firstArea, firstBuilding, firstDoor } = resolveTopologySeed(buildings, areas, doors)

  const visitorScopeType: ScopeType = firstDoor ? "door" : firstArea ? "area" : firstBuilding ? "building" : "all"
  const contractorScopeType: ScopeType = firstArea ? "area" : firstBuilding ? "building" : "all"
  const interviewScopeType: ScopeType = firstBuilding ? "building" : "all"
  const temporaryEmployeeScopeType: ScopeType = firstBuilding ? "building" : "all"

  const buildReviewNote = (scopeType: ScopeType, buildingID: string, areaID: string, doorID: string) => {
    if (hasIncompleteTopology(scopeType, buildingID, areaID, doorID)) {
      return "当前楼宇拓扑还不完整，先用了保守范围；正式授权前建议先去空间页补齐楼宇、区域和门点。"
    }
    return `建议范围：${scopeSummaryByID(scopeType, buildingID, areaID, doorID, buildingByID, areaByID, doorByID)}`
  }

  const presets: Array<Omit<GrantStarter, "reviewNote">> = [
    {
      id: "visitor_reception",
      title: "来访宾客",
      description: "适合前台来访、客户接待和短时访客，优先用二维码或邮件快速发送。",
      scopeType: visitorScopeType,
      buildingID: visitorScopeType === "all" ? "" : firstBuilding?.id ?? "",
      areaID: visitorScopeType === "area" || visitorScopeType === "door" ? firstArea?.id ?? "" : "",
      doorID: visitorScopeType === "door" ? firstDoor?.id ?? "" : "",
      deliveryMethod: "email_qr",
      passType: "visitor",
      validUntil: addHours(8),
    },
    {
      id: "contractor_maintenance",
      title: "施工 / 维护",
      description: "适合当天维修、保洁、施工进场，建议范围控制在区域级并保留时效。",
      scopeType: contractorScopeType,
      buildingID: contractorScopeType === "all" ? "" : firstBuilding?.id ?? "",
      areaID: contractorScopeType === "area" ? firstArea?.id ?? "" : "",
      doorID: "",
      deliveryMethod: "wallet",
      passType: "contractor",
      validUntil: addHours(12),
    },
    {
      id: "candidate_interview",
      title: "面试来访",
      description: "适合面试、候选人参访和短期办公楼访问，默认楼宇级范围即可。",
      scopeType: interviewScopeType,
      buildingID: interviewScopeType === "all" ? "" : firstBuilding?.id ?? "",
      areaID: "",
      doorID: "",
      deliveryMethod: "email_qr",
      passType: "candidate",
      validUntil: addHours(6),
    },
    {
      id: "temporary_employee",
      title: "临时员工补录",
      description: "适合短期派驻、试运行人员和临时员工，后续可转去凭证发放做长期承接。",
      scopeType: temporaryEmployeeScopeType,
      buildingID: firstBuilding?.id ?? "",
      areaID: "",
      doorID: "",
      deliveryMethod: "wallet",
      passType: "employee_temp",
      validUntil: addDays(7),
    },
  ]

  return presets.map((item) => ({
    ...item,
    reviewNote: buildReviewNote(item.scopeType, item.buildingID, item.areaID, item.doorID),
  }))
}
