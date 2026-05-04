import type { ReactNode } from "react"
import { ChevronDownIcon, SearchIcon } from "lucide-react"

import { cn } from "@/lib/utils"

export function MistyisletSearchField({
  value,
  onChange,
  placeholder,
  className,
}: {
  value: string
  onChange: (value: string) => void
  placeholder: string
  className?: string
}) {
  return (
    <div className={cn("flex h-10 flex-1 items-center gap-3 rounded-[6px] border border-line-default bg-white px-4", className)}>
      <SearchIcon className="size-4 shrink-0 text-content-subtle" />
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-full min-w-0 flex-1 bg-transparent text-sm text-content-body outline-none placeholder:text-content-muted"
        placeholder={placeholder}
      />
    </div>
  )
}

export function MistyisletFilterButton({
  label,
  className,
}: {
  label: string
  className?: string
}) {
  return (
    <button
      type="button"
      className={cn("flex h-10 items-center justify-between rounded-[6px] border border-line-default bg-white px-4 text-sm font-semibold text-content-body", className)}
    >
      {label}
      <ChevronDownIcon className="size-4 text-content-subtle" />
    </button>
  )
}

export function MistyisletEmptyTableRow({
  colSpan,
  children,
}: {
  colSpan: number
  children: ReactNode
}) {
  return (
    <tr>
      <td colSpan={colSpan} className="px-6 py-12 text-center text-sm text-content-subtle">
        {children}
      </td>
    </tr>
  )
}

export function MistyisletTablePagination() {
  return (
    <div className="grid grid-cols-3 border-t border-line-subtle px-8 py-5 text-sm text-content-subtle">
      <span>Previous Page</span>
      <span className="text-center text-content-heading">Page 1 of 1</span>
      <span className="text-right">Next Page</span>
    </div>
  )
}
