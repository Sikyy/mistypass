import type { ReactNode } from "react"

type EnterpriseSyncExceptionsProps = {
  actions?: ReactNode
  children: ReactNode
  description?: ReactNode
  title: ReactNode
}

export function EnterpriseSyncExceptions({
  actions,
  children,
  description,
  title,
}: EnterpriseSyncExceptionsProps) {
  return (
    <section
      className="space-y-3 rounded-lg border bg-muted/15 p-3"
      data-testid="enterprise-sync-exceptions"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="text-sm font-medium">{title}</p>
          {description ? <p className="mt-1 text-xs text-muted-foreground">{description}</p> : null}
        </div>
        {actions}
      </div>
      {children}
    </section>
  )
}
