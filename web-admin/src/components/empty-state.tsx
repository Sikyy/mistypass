import type { ReactNode } from "react"
import { InboxIcon } from "lucide-react"

import { MistyIslandMark } from "@/components/brand/misty-island-mark"
import { cn } from "@/lib/utils"

type EmptyStateProps = {
  action?: ReactNode
  className?: string
  description?: ReactNode
  illustration?: ReactNode
  title: ReactNode
}

export function EmptyState({
  action,
  className,
  description,
  illustration,
  title,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex min-h-40 flex-col items-center justify-center rounded-card border border-dashed border-card-task-border bg-[#fafafa] px-4 py-8 text-center text-sm text-[#62636a]",
        className
      )}
    >
      <div className="mb-4 flex size-14 items-center justify-center rounded-2xl border border-card-task-border bg-[#17171c] text-white">
        {illustration ?? (
          <MistyIslandMark
            className="size-11"
            markClassName="h-8 w-11"
            title=""
          />
        )}
      </div>
      <p className="max-w-md text-base font-medium text-content-heading">{title}</p>
      {description ? <p className="mt-2 max-w-md leading-6">{description}</p> : null}
      {action ? <div className="mt-4 flex flex-wrap items-center justify-center gap-2">{action}</div> : null}
    </div>
  )
}

export function EmptyStateIcon() {
  return <InboxIcon className="size-6" />
}
