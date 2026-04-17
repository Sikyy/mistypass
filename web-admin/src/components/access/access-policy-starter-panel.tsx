import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

type PolicyStarterCard = {
  id: string
  groupName: string
  memberCount: number
  name: string
  description: string
  reviewNote: string
  schedule: string
}

type AccessPolicyStarterPanelProps = {
  hasGroups: boolean
  items: PolicyStarterCard[]
  topologyReady: boolean
  onApply: (id: string) => void
}

export function AccessPolicyStarterPanel({
  hasGroups,
  items,
  topologyReady,
  onApply,
}: AccessPolicyStarterPanelProps) {
  if (!hasGroups) {
    return (
      <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm text-muted-foreground">
        当前还没有可复用的用户组。建议先回到“员工与用户组”建立基础分组，再回来生成策略草稿。
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm text-muted-foreground">
        已有用户组暂时没有新的推荐草稿，说明首批常见策略基本已经落地。你可以继续手动补充更细的规则，或直接去发放中心。
      </div>
    )
  }

  return (
    <div className="grid gap-3 lg:grid-cols-2">
      {items.map((item) => (
        <div key={item.id} className="rounded-xl border bg-muted/10 px-4 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <p className="font-medium">{item.groupName}</p>
            <Badge variant="secondary">{item.memberCount} 名成员</Badge>
            <Badge variant="outline">草稿</Badge>
          </div>
          <p className="mt-2 text-sm">{item.name}</p>
          <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
          <p className="mt-2 text-xs text-muted-foreground">{item.reviewNote}</p>
          <p className="mt-1 text-xs text-muted-foreground">建议时间计划：{item.schedule}</p>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Button size="sm" variant="outline" onClick={() => onApply(item.id)}>
              套用到左侧表单
            </Button>
            {!topologyReady ? (
              <Button asChild size="sm" variant="ghost">
                <Link to="/spaces">去补空间拓扑</Link>
              </Button>
            ) : null}
          </div>
        </div>
      ))}
    </div>
  )
}
