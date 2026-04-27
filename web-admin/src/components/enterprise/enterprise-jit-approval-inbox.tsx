import type { ReactNode } from "react"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type EnterpriseJITApprovalInboxProps = {
  children: ReactNode
  description: ReactNode
  title: ReactNode
}

export function EnterpriseJITApprovalInbox({
  children,
  description,
  title,
}: EnterpriseJITApprovalInboxProps) {
  return (
    <Card data-testid="enterprise-jit-approval-inbox">
      <CardHeader>
        <CardTitle className="text-base">{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">{children}</CardContent>
    </Card>
  )
}
