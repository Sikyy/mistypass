import type { ReactNode } from "react"
import { MoreHorizontalIcon, type LucideIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"

export type RowActionItem = {
  id: string
  label: string
  icon?: LucideIcon
  disabled?: boolean
  destructive?: boolean
  hidden?: boolean
  onSelect: () => void
}

export function RowActionsMenu({
  label = "Row actions",
  items,
  align = "end",
  className,
}: {
  label?: string
  items: RowActionItem[]
  align?: "start" | "center" | "end"
  className?: string
}) {
  const visibleItems = items.filter((item) => !item.hidden)
  if (visibleItems.length === 0) {
    return null
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label={label}
          className={cn("rounded-[6px] text-content-subtle hover:bg-brand-subtle hover:text-[#3439cc]", className)}
        >
          <MoreHorizontalIcon className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align={align}
        side="bottom"
        sideOffset={6}
        avoidCollisions={false}
        className="w-44 rounded-[6px] border-line-default bg-white p-1 text-content-body shadow-lg"
      >
        {visibleItems.map((item) => {
          const Icon = item.icon
          return (
            <DropdownMenuItem
              key={item.id}
              variant={item.destructive ? "destructive" : "default"}
              disabled={item.disabled}
              onSelect={() => {
                item.onSelect()
              }}
              className={cn(
                "cursor-pointer rounded-[5px] px-2.5 py-2 text-sm text-content-body focus:bg-brand-subtle focus:text-[#3439cc]",
                item.destructive &&
                  "text-[#bd2f2f] data-[variant=destructive]:text-[#bd2f2f] data-[variant=destructive]:focus:bg-[#fff5f5] data-[variant=destructive]:focus:text-[#9f1d1d]"
              )}
            >
              {Icon ? <Icon className="size-4" /> : null}
              <span>{item.label}</span>
            </DropdownMenuItem>
          )
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export function ConfirmActionDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  confirmLabel,
  cancelLabel = "Cancel",
  pending = false,
  disabled = false,
  destructive = false,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: ReactNode
  children?: ReactNode
  confirmLabel: string
  cancelLabel?: string
  pending?: boolean
  disabled?: boolean
  destructive?: boolean
  onConfirm: () => void
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md rounded-[6px] border-line-default bg-white p-0 text-content-body">
        <DialogHeader className="px-6 pt-6">
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription className="leading-6">{description}</DialogDescription> : null}
        </DialogHeader>
        {children ? <div className="px-6 text-sm text-content-body">{children}</div> : null}
        <DialogFooter className="mx-0 mb-0 mt-2 rounded-b-[6px] border-t border-line-subtle bg-surface-page px-6 py-4">
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() => onOpenChange(false)}
            className="rounded-[6px]"
          >
            {cancelLabel}
          </Button>
          <Button
            type="button"
            variant={destructive ? "destructive" : "default"}
            disabled={disabled || pending}
            onClick={onConfirm}
            className="rounded-[6px]"
          >
            {pending ? "Working..." : confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
