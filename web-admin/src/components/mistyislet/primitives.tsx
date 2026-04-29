import type { ReactNode } from "react"
import { CheckCircle2Icon, type LucideIcon } from "lucide-react"

import { cn } from "@/lib/utils"

export type ActivityTone = "success" | "warning" | "danger" | "info"

export function toneClassName(tone: ActivityTone) {
  switch (tone) {
    case "success":
      return "bg-[#35a853]"
    case "warning":
      return "bg-[#d98b06]"
    case "danger":
      return "bg-[#d93025]"
    default:
      return "bg-[#1863dc]"
  }
}

export function KpiCard({
  label,
  value,
  note,
  icon: Icon,
  tone,
  loading,
}: {
  label: string
  value: string
  note: string
  icon: LucideIcon
  tone: ActivityTone
  loading: boolean
}) {
  return (
    <div className="min-h-[146px] rounded-[6px] border border-[#e1e3e8] bg-white p-5">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <p className="text-sm font-medium text-[#6f717c]">{label}</p>
          <p className="mt-3 text-[34px] font-bold leading-10 text-[#17171c]">{loading ? "--" : value}</p>
        </div>
        <div className="flex size-10 shrink-0 items-center justify-center rounded-[14px] bg-[#fbfbfc] text-[#6f717c] ring-1 ring-[#eceef2]">
          <Icon className="size-5" />
        </div>
      </div>
      <div className="mt-5 flex items-center gap-2 text-xs font-medium text-[#6f717c]">
        <span className={cn("size-2 rounded-full", toneClassName(tone))} />
        <span>{note}</span>
      </div>
    </div>
  )
}

export function PageBreadcrumbs({ items }: { items: string[] }) {
  return (
    <div className="flex flex-wrap items-center gap-2 text-sm font-medium">
      {items.map((item, index) => (
        <span key={`${item}-${index}`} className="flex items-center gap-2">
          <span className={index === items.length - 1 ? "text-[#6f717c]" : "text-[#4f55ff] underline underline-offset-2"}>
            {item}
          </span>
          {index < items.length - 1 ? <span className="text-[#9a9ca7]">›</span> : null}
        </span>
      ))}
    </div>
  )
}

export function PageFrame({
  breadcrumbs,
  title,
  count,
  description,
  actions,
  children,
}: {
  breadcrumbs: string[]
  title: string
  count?: string | number
  description: string
  actions?: ReactNode
  children: ReactNode
}) {
  return (
    <div>
      <div className="flex flex-col gap-5 md:flex-row md:items-start md:justify-between">
        <div>
          <PageBreadcrumbs items={breadcrumbs} />
          <h1 className="mt-5 text-[34px] font-bold leading-[42px] text-[#17171c]">
            {title}
            {count !== undefined ? <span className="ml-2 text-[#6f717c]">{count}</span> : null}
          </h1>
          <p className="mt-2 text-sm text-[#6f717c]">{description}</p>
        </div>
        {actions ? <div className="flex shrink-0 flex-wrap gap-3">{actions}</div> : null}
      </div>
      <div className="mt-8 space-y-6">{children}</div>
    </div>
  )
}

export function InfoBanner({ children }: { children: ReactNode }) {
  return (
    <div className="flex gap-3 rounded-[6px] border border-[#6268ff] bg-[#f3f4ff] px-5 py-4 text-sm leading-6 text-[#2f3037]">
      <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-[#4f55ff] text-xs font-bold text-white">
        i
      </span>
      <p>{children}</p>
    </div>
  )
}

export function StatusDot({ tone, label }: { tone: ActivityTone; label: string }) {
  return (
    <span className="inline-flex items-center gap-2 text-sm text-[#6f717c]">
      <span className={cn("size-2 rounded-full", toneClassName(tone))} />
      {label}
    </span>
  )
}

export function EnabledCheck({ label = "Enabled" }: { label?: string }) {
  return (
    <span className="inline-flex items-center gap-2 text-sm text-[#6f717c]">
      <CheckCircle2Icon className="size-5 fill-[#35a853] text-white" />
      {label ? <span>{label}</span> : null}
    </span>
  )
}

export function ToggleSwitch({ enabled }: { enabled: boolean }) {
  return (
    <span
      className={cn(
        "relative inline-flex h-5 w-10 shrink-0 rounded-full transition-colors",
        enabled ? "bg-[#9ba3ff]" : "bg-[#c4c6cc]"
      )}
    >
      <span
        className={cn(
          "absolute top-0.5 size-4 rounded-full shadow-sm transition-all",
          enabled ? "right-0.5 bg-[#4f55ff]" : "left-0.5 bg-white"
        )}
      />
    </span>
  )
}

export function FormField({
  label,
  value,
  muted = false,
  trailing,
}: {
  label: string
  value: ReactNode
  muted?: boolean
  trailing?: ReactNode
}) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs font-semibold text-[#6f717c]">{label}</span>
      <div
        className={cn(
          "flex min-h-12 items-center rounded-[6px] border border-[#d9dbe3] px-4 text-sm",
          muted ? "text-[#9a9ca7]" : "text-[#2f3037]"
        )}
      >
        <span className="min-w-0 flex-1 truncate">{value}</span>
        {trailing}
      </div>
    </label>
  )
}

export function PanelHeader({
  title,
  description,
  action,
}: {
  title: string
  description?: string
  action?: ReactNode
}) {
  return (
    <div className="flex flex-col gap-4 border-b border-[#eceef2] px-7 py-5 md:flex-row md:items-center md:justify-between">
      <div>
        <h2 className="text-lg font-semibold text-[#17171c]">{title}</h2>
        {description ? <p className="mt-1 text-sm text-[#6f717c]">{description}</p> : null}
      </div>
      {action}
    </div>
  )
}

export function SettingToggleRows({
  rows,
}: {
  rows: Array<[string, boolean, string, LucideIcon?]>
}) {
  return (
    <div className="divide-y divide-[#eceef2]">
      {rows.map(([title, enabled, description, Icon]) => (
        <div key={title} className="flex gap-5 px-7 py-5">
          {Icon ? (
            <div className={cn("flex size-10 shrink-0 items-center justify-center rounded-[6px]", enabled ? "bg-[#eef0ff] text-[#4f55ff]" : "bg-[#f1f2f5] text-[#6f717c]")}>
              <Icon className="size-5" />
            </div>
          ) : null}
          <div className="min-w-0">
            <h3 className="font-semibold text-[#17171c]">{title}</h3>
            <p className="mt-1 text-sm leading-6 text-[#6f717c]">{description}</p>
          </div>
          <div className="ml-auto pt-1">
            <ToggleSwitch enabled={enabled} />
          </div>
        </div>
      ))}
    </div>
  )
}

export function SettingsPanel({
  tabs,
  active,
  children,
  footer,
  onTabChange,
}: {
  tabs: string[]
  active: string
  children: ReactNode
  footer?: ReactNode
  onTabChange?: (tab: string) => void
}) {
  return (
    <section className="grid rounded-[6px] border border-[#d9dbe3] bg-white lg:grid-cols-[280px_1fr]">
      <div className="border-b border-[#eceef2] p-5 lg:border-b-0 lg:border-r lg:p-7">
        {tabs.map((item) => (
          <button
            key={item}
            type="button"
            onClick={() => onTabChange?.(item)}
            className={cn(
              "mb-2 flex h-12 w-full items-center rounded-[6px] px-5 text-left text-base font-semibold",
              item === active ? "bg-[#4f55ff] text-white" : "text-[#2f3037] hover:bg-[#f3f4ff] hover:text-[#3439cc]"
            )}
          >
            {item}
          </button>
        ))}
      </div>
      <div className="min-w-0 overflow-x-auto">
        {children}
        {footer ? <div className="flex justify-end border-t border-[#eceef2] bg-[#fbfbfc] p-5">{footer}</div> : null}
      </div>
    </section>
  )
}
