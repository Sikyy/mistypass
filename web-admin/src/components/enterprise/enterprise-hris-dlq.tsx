import type { ReactNode } from "react"

type EnterpriseHRISDLQProps = {
  actions?: ReactNode
  children: ReactNode
  title: ReactNode
}

export function EnterpriseHRISDLQ({ actions, children, title }: EnterpriseHRISDLQProps) {
  return (
    <section
      className="rounded-md border bg-background/70 p-3"
      data-testid="enterprise-hris-dlq"
    >
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm font-medium">{title}</p>
        {actions}
      </div>
      {children}
    </section>
  )
}
