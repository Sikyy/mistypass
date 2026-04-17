import { ReactNode } from "react"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type AccessReadinessCardProps = {
  title: string
  description: string
  status: ReactNode
  detail: string
  actions?: ReactNode
}

export function AccessReadinessCard({
  title,
  description,
  status,
  detail,
  actions,
}: AccessReadinessCardProps) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {status}
        <p className="text-sm text-muted-foreground">{detail}</p>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </CardContent>
    </Card>
  )
}
