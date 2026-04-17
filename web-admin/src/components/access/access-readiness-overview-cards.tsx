import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

import { AccessReadinessCard } from "./access-readiness-card"

type AccessReadinessOverviewCardsProps = {
  activeEmployeeCount: number
  groupCount: number
  policyCount: number
  buildingCount: number
  areaCount: number
  doorCount: number
  visitorGrantCount: number
  expiredGrantCount: number
  directoryReady: boolean
  policyReady: boolean
  topologyReady: boolean
  issuanceReady: boolean
  hasEmployees: boolean
  enterpriseSyncLink: string
  directorySectionLink: string
  spacesLink: string
  policiesSectionLink: string
  walletEmployeeLink: string
  grantsSectionLink: string
}

export function AccessReadinessOverviewCards({
  activeEmployeeCount,
  groupCount,
  policyCount,
  buildingCount,
  areaCount,
  doorCount,
  visitorGrantCount,
  expiredGrantCount,
  directoryReady,
  policyReady,
  topologyReady,
  issuanceReady,
  hasEmployees,
  enterpriseSyncLink,
  directorySectionLink,
  spacesLink,
  policiesSectionLink,
  walletEmployeeLink,
  grantsSectionLink,
}: AccessReadinessOverviewCardsProps) {
  return (
    <div className="grid gap-4 xl:grid-cols-3">
      <AccessReadinessCard
        title="1. 目录准备度"
        description="先确认员工目录和用户组是否已经具备策略与发放的上游数据。"
        status={
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={directoryReady ? "outline" : "secondary"}>{directoryReady ? "已就绪" : "待补齐"}</Badge>
            <span className="text-sm text-muted-foreground">在职员工 {activeEmployeeCount} 名 / 用户组 {groupCount} 个</span>
          </div>
        }
        detail={
          directoryReady
            ? "目录和用户组已具备基础条件，可以继续配置访问策略。"
            : !hasEmployees
              ? "当前还没有企业员工目录，建议先去企业页接入 HRIS、SCIM、CSV 或手动同步。"
              : "已有员工目录，但还没有稳定的用户组，建议先完成用户组整理。"
        }
        actions={
          <Button asChild size="sm" variant="outline">
            <Link to={!hasEmployees ? enterpriseSyncLink : directorySectionLink}>{!hasEmployees ? "去导入员工" : "去员工与用户组"}</Link>
          </Button>
        }
      />

      <AccessReadinessCard
        title="2. 策略准备度"
        description="把权限独立成规则层，并确认楼宇拓扑是否足够支撑楼宇、区域、门点授权。"
        status={
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={policyReady ? "outline" : "secondary"}>{policyReady ? "已就绪" : "待补齐"}</Badge>
            <span className="text-sm text-muted-foreground">
              策略 {policyCount} 条 / 楼宇 {buildingCount} / 区域 {areaCount} / 门点 {doorCount}
            </span>
          </div>
        }
        detail={
          !directoryReady
            ? "建议先把目录和用户组准备完整，再开始配置策略。"
            : !topologyReady
              ? "当前楼宇拓扑还不完整，建议先去空间页补齐楼宇、区域和门点。"
              : policyReady
                ? "策略规则已经成型，可以继续处理访客与临时授权，或进入发放中心。"
                : "目录和拓扑都已具备，可以开始建立首批访问策略。"
        }
        actions={
          <Button asChild size="sm" variant="outline">
            <Link to={!topologyReady ? spacesLink : policiesSectionLink}>{!topologyReady ? "去空间拓扑" : "去权限策略"}</Link>
          </Button>
        }
      />

      <AccessReadinessCard
        title="3. 发放与授权准备度"
        description="临时授权留在这里处理，长期员工和批量发放则统一进入凭证发放中心。"
        status={
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={issuanceReady ? "outline" : "secondary"}>{issuanceReady ? "可进入发放" : "待补齐前置"}</Badge>
            <span className="text-sm text-muted-foreground">访客授权 {visitorGrantCount} 条 / 已到期 {expiredGrantCount} 条</span>
          </div>
        }
        detail={
          issuanceReady
            ? "长期员工、批量补发和状态维护建议去“凭证发放”，短期访客和临时证继续留在当前页处理。"
            : "在目录和策略都就绪前，不建议直接跳到长期发放；先把组织基础数据准备完整。"
        }
        actions={
          <Button asChild size="sm" variant="outline">
            <Link to={issuanceReady ? walletEmployeeLink : grantsSectionLink}>{issuanceReady ? "去凭证发放" : "去临时授权"}</Link>
          </Button>
        }
      />
    </div>
  )
}
