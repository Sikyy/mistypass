import type { ReactNode } from "react"

type EnterpriseWorkerAlertsProps = {
  actions?: ReactNode
  children: ReactNode
  title: ReactNode
}

export function EnterpriseWorkerAlerts({ actions, children, title }: EnterpriseWorkerAlertsProps) {
  return (
    <section
      className="space-y-3 rounded-lg border bg-muted/15 p-3"
      data-testid="enterprise-worker-alerts"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm font-medium">{title}</p>
        {actions}
      </div>
      {children}
    </section>
  )
}
