import type { ReactNode } from "react"

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { cn } from "@/lib/utils"

type DetailDrawerProps = {
  bodyClassName?: string
  children: ReactNode
  contentClassName?: string
  description?: ReactNode
  footer?: ReactNode
  headerClassName?: string
  onOpenChange: (open: boolean) => void
  open: boolean
  side?: "top" | "right" | "bottom" | "left"
  title: ReactNode
}

export function DetailDrawer({
  bodyClassName,
  children,
  contentClassName,
  description,
  footer,
  headerClassName,
  onOpenChange,
  open,
  side = "right",
  title,
}: DetailDrawerProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side={side} className={cn("w-full overflow-y-auto sm:max-w-xl", contentClassName)}>
        <SheetHeader className={headerClassName}>
          <SheetTitle>{title}</SheetTitle>
          {description ? <SheetDescription>{description}</SheetDescription> : null}
        </SheetHeader>
        <div className={cn("space-y-4 px-4 pb-4", bodyClassName)}>{children}</div>
        {footer ? <SheetFooter>{footer}</SheetFooter> : null}
      </SheetContent>
    </Sheet>
  )
}
