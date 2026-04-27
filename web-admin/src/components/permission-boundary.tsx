import type { ReactNode } from "react"
import { ShieldOffIcon } from "lucide-react"

import { EmptyState } from "@/components/empty-state"

type PermissionBoundaryProps = {
  action?: ReactNode
  allowed: boolean
  children: ReactNode
  description?: ReactNode
  title: ReactNode
}

export function PermissionBoundary({
  action,
  allowed,
  children,
  description,
  title,
}: PermissionBoundaryProps) {
  if (allowed) {
    return <>{children}</>
  }

  return (
    <EmptyState
      action={action}
      description={description}
      illustration={<ShieldOffIcon className="size-6" />}
      title={title}
    />
  )
}
