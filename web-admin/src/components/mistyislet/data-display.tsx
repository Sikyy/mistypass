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
    <div className={cn("flex h-10 flex-1 items-center gap-3 rounded-[6px] border border-[#d9dbe3] bg-white px-4", className)}>
      <SearchIcon className="size-4 shrink-0 text-[#6f717c]" />
      <input
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-full min-w-0 flex-1 bg-transparent text-sm text-[#2f3037] outline-none placeholder:text-[#9a9ca7]"
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
      className={cn("flex h-10 items-center justify-between rounded-[6px] border border-[#d9dbe3] bg-white px-4 text-sm font-semibold text-[#2f3037]", className)}
    >
      {label}
      <ChevronDownIcon className="size-4 text-[#6f717c]" />
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
      <td colSpan={colSpan} className="px-6 py-12 text-center text-sm text-[#6f717c]">
        {children}
      </td>
    </tr>
  )
}

export function MistyisletTablePagination() {
  return (
    <div className="grid grid-cols-3 border-t border-[#eceef2] px-8 py-5 text-sm text-[#6f717c]">
      <span>Previous Page</span>
      <span className="text-center text-[#17171c]">Page 1 of 1</span>
      <span className="text-right">Next Page</span>
    </div>
  )
}
