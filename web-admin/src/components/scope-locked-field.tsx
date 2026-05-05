import type { ReactNode } from "react"
import { LockKeyholeIcon } from "lucide-react"

import { cn } from "@/lib/utils"

type ScopeLockedFieldProps = {
  className?: string
  description?: ReactNode
  icon?: ReactNode
  label: ReactNode
  value: ReactNode
}

export function ScopeLockedField({
  className,
  description,
  icon,
  label,
  value,
}: ScopeLockedFieldProps) {
  return (
    <div
      className={cn(
        "flex min-w-0 items-start gap-3 rounded-card border border-card-task-border bg-card-task px-4 py-3 text-sm text-[#212121]",
        className
      )}
    >
      <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full border border-card-task-border bg-[#fafafa] text-[#62636a]">
        {icon ?? <LockKeyholeIcon className="size-4" />}
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-xs font-medium tracking-normal text-[#62636a]">{label}</p>
        <p className="mt-1 break-words font-medium text-content-heading">{value}</p>
        {description ? <p className="mt-1 text-xs leading-5 text-[#62636a]">{description}</p> : null}
      </div>
    </div>
  )
}
