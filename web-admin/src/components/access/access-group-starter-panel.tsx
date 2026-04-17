import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type AccessGroupStarterCard = {
  accessRole: string
  matchedMemberCount: number
  name: string
  permissionPreset: string
  position: string
}

type AccessGroupStarterPanelProps = {
  items: AccessGroupStarterCard[]
  onCreate: () => void
}

export function AccessGroupStarterPanel({ items, onCreate }: AccessGroupStarterPanelProps) {
  if (items.length === 0) {
    return null
  }

  return (
    <Card>
      <CardHeader className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <div>
          <CardTitle className="text-base">快速创建基础用户组</CardTitle>
          <CardDescription>
            已有员工目录但还缺基础分组时，可按岗位模板一键生成默认用户组，减少从目录到策略的空档。
          </CardDescription>
        </div>
        <Button size="sm" variant="outline" onClick={onCreate}>
          一键创建 {items.length} 个基础组
        </Button>
      </CardHeader>
      <CardContent className="grid gap-3 lg:grid-cols-2">
        {items.map((item) => (
          <div key={item.name} className="rounded-xl border bg-muted/10 px-4 py-3">
            <div className="flex flex-wrap items-center gap-2">
              <p className="font-medium">{item.name}</p>
              <Badge variant="secondary">{item.matchedMemberCount} 名匹配员工</Badge>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">{item.permissionPreset}</p>
            <p className="mt-1 text-xs text-muted-foreground">
              匹配依据：{item.position} / role={item.accessRole}
            </p>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
