import { Clock4Icon, ShieldCheckIcon, TicketIcon, UsersRoundIcon } from "lucide-react"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type AccessDomainMetricsCardsProps = {
  loading: boolean
  policyCount: number
  activeEmployeeCount: number
  groupCount: number
  grantCount: number
  visitorGrantCount: number
  expiredGrantCount: number
}

export function AccessDomainMetricsCards({
  loading,
  policyCount,
  activeEmployeeCount,
  groupCount,
  grantCount,
  visitorGrantCount,
  expiredGrantCount,
}: AccessDomainMetricsCardsProps) {
  return (
    <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <Card>
        <CardHeader className="pb-2">
          <CardDescription>策略数</CardDescription>
          <CardTitle className="flex items-center gap-2 text-2xl">
            {loading ? "--" : policyCount} <ShieldCheckIcon className="size-4 text-muted-foreground" />
          </CardTitle>
        </CardHeader>
        <CardContent className="mp-kpi-note">独立管理楼宇、区域、门点权限规则。</CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardDescription>员工与用户组</CardDescription>
          <CardTitle className="flex items-center gap-2 text-2xl">
            {loading ? "--" : activeEmployeeCount} <UsersRoundIcon className="size-4 text-muted-foreground" />
          </CardTitle>
        </CardHeader>
        <CardContent className="mp-kpi-note">
          {loading ? "正在加载目录状态..." : `${groupCount} 个用户组，按组织目录维护成员。`}
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardDescription>临时与访客授权</CardDescription>
          <CardTitle className="flex items-center gap-2 text-2xl">
            {loading ? "--" : grantCount} <Clock4Icon className="size-4 text-muted-foreground" />
          </CardTitle>
        </CardHeader>
        <CardContent className="mp-kpi-note">
          {loading ? "正在统计授权..." : `访客 ${visitorGrantCount} 条，已到期 ${expiredGrantCount} 条。`}
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="pb-2">
          <CardDescription>下发方式</CardDescription>
          <CardTitle className="flex items-center gap-2 text-2xl">
            2 <TicketIcon className="size-4 text-muted-foreground" />
          </CardTitle>
        </CardHeader>
        <CardContent className="mp-kpi-note">MistyPass 移动凭证 / 邮件二维码。</CardContent>
      </Card>
    </div>
  )
}
