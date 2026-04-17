import type { AccessSection } from "./access-sections-tabs"

export type AccessSectionOverviewItem = {
  value: AccessSection
  title: string
  description: string
  metric: string
  helper: string
}

export type AccessRecommendedAction = {
  title: string
  description: string
  to: string
  label: string
}

export function buildAccessSectionsOverview({
  activeEmployeeCount,
  employeeCount,
  expiredGrantCount,
  grantCount,
  groupCount,
  loading,
  policyCount,
  visitorGrantCount,
}: {
  activeEmployeeCount: number
  employeeCount: number
  expiredGrantCount: number
  grantCount: number
  groupCount: number
  loading: boolean
  policyCount: number
  visitorGrantCount: number
}): AccessSectionOverviewItem[] {
  return [
    {
      value: "directory",
      title: "员工与用户组",
      description: "先接员工目录，再把员工归到用户组，后续策略和发放都依赖这里。",
      metric: loading ? "--" : `${activeEmployeeCount} 名在职员工 / ${groupCount} 个用户组`,
      helper:
        employeeCount > 0
          ? "目录已接通，可直接维护用户组成员。"
          : "员工库为空时，先去企业页接 HRIS、SCIM、CSV 或手动导入。",
    },
    {
      value: "policies",
      title: "权限策略",
      description: "把访问权限落到楼宇、区域、门点，并为不同用户组准备策略模板。",
      metric: loading ? "--" : `${policyCount} 条策略`,
      helper: "策略是权限的规则层，不再和员工导入、临时授权混在一起。",
    },
    {
      value: "grants",
      title: "临时与访客授权",
      description: "处理短期授权、访客通行和邮件二维码 / MistyPass 的临时发放。",
      metric: loading ? "--" : `${grantCount} 条授权 / ${visitorGrantCount} 条访客授权`,
      helper: expiredGrantCount > 0 ? `有 ${expiredGrantCount} 条授权已到期，建议优先复核。` : "当前授权时效正常。",
    },
  ]
}

export function deriveNextRecommendedAction({
  directorySectionLink,
  employeeCount,
  enterpriseSyncLink,
  policyReady,
  policiesSectionLink,
  spacesLink,
  topologyReady,
  walletEmployeeLink,
  groupCount,
}: {
  directorySectionLink: string
  employeeCount: number
  enterpriseSyncLink: string
  groupCount: number
  policiesSectionLink: string
  policyReady: boolean
  spacesLink: string
  topologyReady: boolean
  walletEmployeeLink: string
}): AccessRecommendedAction {
  if (employeeCount === 0) {
    return {
      title: "先接入员工目录",
      description: "员工库还没接通，用户组、策略和长期发放都会缺上游数据。",
      to: enterpriseSyncLink,
      label: "去导入员工",
    }
  }
  if (groupCount === 0) {
    return {
      title: "先整理用户组",
      description: "当前已有员工目录，但还没有稳定的用户组承接策略和发放对象。",
      to: directorySectionLink,
      label: "去员工与用户组",
    }
  }
  if (!topologyReady) {
    return {
      title: "补齐空间拓扑",
      description: "策略要精确落到楼宇、区域和门点，当前拓扑还不完整。",
      to: spacesLink,
      label: "去空间拓扑",
    }
  }
  if (!policyReady) {
    return {
      title: "建立首批权限策略",
      description: "用户组和拓扑已具备，可以开始落楼宇、区域、门点的访问规则。",
      to: policiesSectionLink,
      label: "去权限策略",
    }
  }
  return {
    title: "进入凭证发放",
    description: "目录、用户组和权限策略都已具备，可以去发放中心完成长期员工发放与状态维护。",
    to: walletEmployeeLink,
    label: "去凭证发放",
  }
}
