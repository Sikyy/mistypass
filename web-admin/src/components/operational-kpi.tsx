import type { ComponentProps, ReactNode } from "react"
import { ArrowDownRightIcon, ArrowRightIcon, ArrowUpRightIcon, TriangleAlertIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { cn } from "@/lib/utils"

type OperationalKPITrendDirection = "up" | "down" | "flat" | "success" | "warning" | "danger"

type OperationalKPITrend = {
  direction?: OperationalKPITrendDirection
  label: ReactNode
}

type OperationalKPIProps = {
  className?: string
  description?: ReactNode
  icon?: ReactNode
  label: ReactNode
  note?: ReactNode
  trend?: OperationalKPITrend
  value: ReactNode
}

const trendVariant: Record<OperationalKPITrendDirection, ComponentProps<typeof Badge>["variant"]> = {
  up: "success",
  down: "warning",
  flat: "outline",
  success: "success",
  warning: "warning",
  danger: "danger",
}

function TrendIcon({ direction }: { direction: OperationalKPITrendDirection }) {
  if (direction === "up" || direction === "success") {
    return <ArrowUpRightIcon className="size-3" />
  }
  if (direction === "down" || direction === "danger") {
    return <ArrowDownRightIcon className="size-3" />
  }
  if (direction === "warning") {
    return <TriangleAlertIcon className="size-3" />
  }
  return <ArrowRightIcon className="size-3" />
}

export function OperationalKPI({
  className,
  description,
  icon,
  label,
  note,
  trend,
  value,
}: OperationalKPIProps) {
  const direction = trend?.direction ?? "flat"

  return (
    <Card variant="task" className={cn("min-w-0", className)}>
      <CardHeader className="gap-3 sm:flex sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <CardDescription>{label}</CardDescription>
          <CardTitle className="mt-1 flex min-w-0 items-center gap-2 text-2xl">
            {icon ? <span className="shrink-0 text-[#62636a]">{icon}</span> : null}
            <span className="min-w-0 truncate">{value}</span>
          </CardTitle>
        </div>
        {trend ? (
          <Badge variant={trendVariant[direction]} className="w-fit">
            <TrendIcon direction={direction} />
            {trend.label}
          </Badge>
        ) : null}
      </CardHeader>
      {(description || note) ? (
        <CardContent className="space-y-1 text-xs leading-5 text-[#62636a]">
          {description ? <p>{description}</p> : null}
          {note ? <p>{note}</p> : null}
        </CardContent>
      ) : null}
    </Card>
  )
}
