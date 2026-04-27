import type { ReactNode } from "react"

type EnterpriseHRISReceiptsProps = {
  actions?: ReactNode
  children: ReactNode
  title: ReactNode
}

export function EnterpriseHRISReceipts({ actions, children, title }: EnterpriseHRISReceiptsProps) {
  return (
    <section
      className="rounded-md border bg-background/70 p-3"
      data-testid="enterprise-hris-receipts"
    >
      <div className="flex items-center justify-between gap-2">
        <p className="text-sm font-medium">{title}</p>
        {actions}
      </div>
      {children}
    </section>
  )
}
