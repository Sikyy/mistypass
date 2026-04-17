import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

type AccessGrantStarterCardProps = {
  title: string
  deliveryLabel: string
  passType: string
  description: string
  reviewNote: string
  validUntilLabel: string
  onApply: () => void
  showTopologyAction?: boolean
}

export function AccessGrantStarterCard({
  title,
  deliveryLabel,
  passType,
  description,
  reviewNote,
  validUntilLabel,
  onApply,
  showTopologyAction = false,
}: AccessGrantStarterCardProps) {
  return (
    <div className="rounded-xl border bg-muted/10 px-4 py-3">
      <div className="flex flex-wrap items-center gap-2">
        <p className="font-medium">{title}</p>
        <Badge variant="secondary">{deliveryLabel}</Badge>
        <Badge variant="outline">{passType}</Badge>
      </div>
      <p className="mt-2 text-sm text-muted-foreground">{description}</p>
      <p className="mt-2 text-xs text-muted-foreground">{reviewNote}</p>
      <p className="mt-1 text-xs text-muted-foreground">建议截止时间：{validUntilLabel}</p>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <Button size="sm" variant="outline" onClick={onApply}>
          套用到左侧表单
        </Button>
        {showTopologyAction ? (
          <Button asChild size="sm" variant="ghost">
            <Link to="/spaces">去补空间拓扑</Link>
          </Button>
        ) : null}
      </div>
    </div>
  )
}
