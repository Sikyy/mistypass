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
    <div className="min-h-[146px] rounded-[6px] border border-line-default bg-white p-5">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <p className="text-sm font-medium text-content-subtle">{label}</p>
          <p className="mt-3 text-[34px] font-bold leading-10 text-content-heading">{loading ? "--" : value}</p>
        </div>
        <div className="flex size-10 shrink-0 items-center justify-center rounded-[14px] bg-surface-page text-content-subtle ring-1 ring-[#eceef2]">
          <Icon className="size-5" />
        </div>
      </div>
      <div className="mt-5 flex items-center gap-2 text-xs font-medium text-content-subtle">
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
          <span className={index === items.length - 1 ? "text-content-subtle" : "text-brand underline underline-offset-2"}>
            {item}
          </span>
          {index < items.length - 1 ? <span className="text-content-muted">›</span> : null}
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
          <h1 className="mt-5 text-[34px] font-bold leading-[42px] text-content-heading">
            {title}
            {count !== undefined ? <span className="ml-2 text-content-subtle">{count}</span> : null}
          </h1>
          <p className="mt-2 text-sm text-content-subtle">{description}</p>
        </div>
        {actions ? <div className="flex shrink-0 flex-wrap gap-3">{actions}</div> : null}
      </div>
      <div className="mt-8 space-y-6">{children}</div>
    </div>
  )
}

export function InfoBanner({ children }: { children: ReactNode }) {
  return (
    <div className="flex gap-3 rounded-[6px] border border-brand-ring bg-brand-subtle px-5 py-4 text-sm leading-6 text-content-body">
      <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-brand text-xs font-bold text-white">
        i
      </span>
      <p>{children}</p>
    </div>
  )
}

export function StatusDot({ tone, label }: { tone: ActivityTone; label: string }) {
  return (
    <span className="inline-flex items-center gap-2 text-sm text-content-subtle">
      <span className={cn("size-2 rounded-full", toneClassName(tone))} />
      {label}
    </span>
  )
}

export function EnabledCheck({ label = "Enabled" }: { label?: string }) {
  return (
    <span className="inline-flex items-center gap-2 text-sm text-content-subtle">
      <CheckCircle2Icon className="size-5 fill-[#35a853] text-white" />
      {label ? <span>{label}</span> : null}
    </span>
  )
}

export function ToggleSwitch({ enabled, onToggle, label }: { enabled: boolean; onToggle?: () => void; label?: string }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={enabled}
      aria-label={label}
      onClick={onToggle}
      className={cn(
        "relative inline-flex h-5 w-10 shrink-0 rounded-full transition-colors",
        enabled ? "bg-brand-ring" : "bg-[#c4c6cc]",
        onToggle && "cursor-pointer"
      )}
    >
      <span
        className={cn(
          "absolute top-0.5 size-4 rounded-full shadow-sm transition-all",
          enabled ? "right-0.5 bg-brand" : "left-0.5 bg-white"
        )}
      />
    </button>
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
      <span className="mb-2 block text-xs font-semibold text-content-subtle">{label}</span>
      <div
        className={cn(
          "flex min-h-12 items-center rounded-[6px] border border-line-default px-4 text-sm",
          muted ? "text-content-muted" : "text-content-body"
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
    <div className="flex flex-col gap-4 border-b border-line-subtle px-7 py-5 md:flex-row md:items-center md:justify-between">
      <div>
        <h2 className="text-lg font-semibold text-content-heading">{title}</h2>
        {description ? <p className="mt-1 text-sm text-content-subtle">{description}</p> : null}
      </div>
      {action}
    </div>
  )
}

export function SettingToggleRows({
  rows,
  onToggle,
}: {
  rows: Array<[string, boolean, string, LucideIcon?]>
  onToggle?: (index: number) => void
}) {
  return (
    <div className="divide-y divide-line-subtle">
      {rows.map(([title, enabled, description, Icon], index) => (
        <div key={title} className="flex gap-5 px-7 py-5">
          {Icon ? (
            <div className={cn("flex size-10 shrink-0 items-center justify-center rounded-[6px]", enabled ? "bg-brand-subtle text-brand" : "bg-surface-sunken text-content-subtle")}>
              <Icon className="size-5" />
            </div>
          ) : null}
          <div className="min-w-0">
            <h3 className="font-semibold text-content-heading">{title}</h3>
            <p className="mt-1 text-sm leading-6 text-content-subtle">{description}</p>
          </div>
          <div className="ml-auto pt-1">
            <ToggleSwitch enabled={enabled} onToggle={onToggle ? () => onToggle(index) : undefined} />
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
    <section className="grid rounded-[6px] border border-line-default bg-white lg:grid-cols-[280px_1fr]">
      <div className="border-b border-line-subtle p-5 lg:border-b-0 lg:border-r lg:p-7">
        {tabs.map((item) => (
          <button
            key={item}
            type="button"
            onClick={() => onTabChange?.(item)}
            className={cn(
              "mb-2 flex h-12 w-full items-center rounded-[6px] px-5 text-left text-base font-semibold",
              item === active ? "bg-brand text-white" : "text-content-body hover:bg-brand-subtle hover:text-brand-hover"
            )}
          >
            {item}
          </button>
        ))}
      </div>
      <div className="min-w-0 overflow-x-auto">
        {children}
        {footer ? <div className="flex justify-end border-t border-line-subtle bg-surface-page p-5">{footer}</div> : null}
      </div>
    </section>
  )
}
