import { ReactNode } from "react"

import { cn } from "@/lib/utils"

type AccessDomainBannerProps = {
  title: string
  description: string
  actions?: ReactNode
  className?: string
}

export function AccessDomainBanner({
  title,
  description,
  actions,
  className,
}: AccessDomainBannerProps) {
  return (
    <div className={cn("rounded-xl border bg-muted/15 px-4 py-3", className)}>
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <p className="text-sm font-medium">{title}</p>
          <p className="mt-1 text-sm text-muted-foreground">{description}</p>
        </div>
        {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
      </div>
    </div>
  )
}
