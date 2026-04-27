import type { ComponentProps, ReactNode } from "react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { EmptyState } from "@/components/empty-state"
import { cn } from "@/lib/utils"

type ActionInboxButtonVariant = ComponentProps<typeof Button>["variant"]
type ActionInboxBadgeVariant = ComponentProps<typeof Badge>["variant"]

type ActionInboxAction = {
  disabled?: boolean
  label: ReactNode
  onClick?: () => void
  variant?: ActionInboxButtonVariant
}

type ActionInboxItem = {
  actions?: ActionInboxAction[]
  content?: ReactNode
  description?: ReactNode
  id: string
  meta?: ReactNode
  statusLabel?: ReactNode
  statusVariant?: ActionInboxBadgeVariant
  title: ReactNode
}

type ActionInboxEmptyState = {
  action?: ReactNode
  description?: ReactNode
  title: ReactNode
}

type ActionInboxProps = {
  className?: string
  description?: ReactNode
  emptyState: ActionInboxEmptyState
  items: ActionInboxItem[]
  summary?: ReactNode
  title: ReactNode
}

export function ActionInbox({
  className,
  description,
  emptyState,
  items,
  summary,
  title,
}: ActionInboxProps) {
  return (
    <Card variant="task" className={className}>
      <CardHeader className="gap-3 sm:flex sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <CardTitle>{title}</CardTitle>
          {description ? <CardDescription>{description}</CardDescription> : null}
        </div>
        {summary ? <div className="shrink-0">{summary}</div> : null}
      </CardHeader>
      <CardContent className="space-y-3">
        {items.length === 0 ? (
          <EmptyState
            action={emptyState.action}
            description={emptyState.description}
            title={emptyState.title}
          />
        ) : (
          items.map((item) => (
            <div
              key={item.id}
              className="rounded-card border border-card-task-border bg-[#fafafa] px-4 py-3 text-sm text-[#212121]"
            >
              <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div className="min-w-0 space-y-1">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <p className="min-w-0 font-medium text-[#17171c]">{item.title}</p>
                    {item.statusLabel ? (
                      <Badge variant={item.statusVariant ?? "outline"}>{item.statusLabel}</Badge>
                    ) : null}
                  </div>
                  {item.description ? <p className="leading-6 text-[#62636a]">{item.description}</p> : null}
                  {item.meta ? <div className="text-xs leading-5 text-[#62636a]">{item.meta}</div> : null}
                </div>
                {item.actions?.length ? (
                  <div className="flex shrink-0 flex-wrap gap-2">
                    {item.actions.map((action, index) => {
                      const variant = action.variant ?? "interaction"

                      return (
                        <Button
                          key={index}
                          size="sm"
                          variant={variant}
                          disabled={action.disabled}
                          onClick={action.onClick}
                          className={cn(variant === "interaction" && "text-[#212121]")}
                        >
                          {action.label}
                        </Button>
                      )
                    })}
                  </div>
                ) : null}
              </div>
              {item.content ? <div className="mt-3">{item.content}</div> : null}
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}

export type { ActionInboxAction, ActionInboxEmptyState, ActionInboxItem }
