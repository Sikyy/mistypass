import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "group/button inline-flex shrink-0 items-center justify-center rounded-lg border border-transparent bg-clip-padding text-sm leading-none font-medium tracking-[0.01em] whitespace-nowrap transition-all outline-none select-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 active:not-aria-[haspopup]:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default:
          "bg-brand text-white shadow-[0_1px_2px_rgba(35,38,120,0.18)] hover:bg-[#454bea] active:bg-[#3f44d4] disabled:bg-[#eef0f4] disabled:text-[#8d909b] disabled:shadow-none disabled:opacity-100",
        outline:
          "border-line-default bg-white text-content-body hover:border-[#8589ff] hover:bg-brand-subtle hover:text-[#3439cc] aria-expanded:border-[#8589ff] aria-expanded:bg-brand-subtle aria-expanded:text-[#3439cc] disabled:border-line-default disabled:bg-[#f7f8fb] disabled:text-[#8d909b] disabled:opacity-100",
        secondary:
          "bg-[#eef0f4] text-content-body hover:bg-[#e2e5ec] aria-expanded:bg-[#e2e5ec] aria-expanded:text-content-body disabled:bg-[#f1f2f5] disabled:text-[#8d909b] disabled:opacity-100",
        ghost:
          "text-content-body hover:bg-brand-subtle hover:text-[#3439cc] aria-expanded:bg-brand-subtle aria-expanded:text-[#3439cc] disabled:bg-transparent disabled:text-[#8d909b] disabled:opacity-100",
        interaction:
          "bg-transparent text-foreground hover:bg-transparent hover:text-interaction focus-visible:border-interaction focus-visible:ring-interaction/30 aria-expanded:text-interaction disabled:text-[#8d909b] disabled:opacity-100",
        destructive:
          "bg-[#fff5f5] text-[#bd2f2f] hover:bg-[#ffe8e8] hover:text-[#9f1d1d] focus-visible:border-[#d45b5b] focus-visible:ring-[#d45b5b]/20 disabled:bg-[#f7f8fb] disabled:text-[#8d909b] disabled:opacity-100",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default:
          "h-9 gap-1.5 px-3.5 text-[0.8125rem] has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        xs: "h-6 gap-1 rounded-[min(var(--radius-md),10px)] px-2 text-xs in-data-[slot=button-group]:rounded-lg has-data-[icon=inline-end]:pr-1.5 has-data-[icon=inline-start]:pl-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-8 gap-1.5 rounded-[min(var(--radius-md),12px)] px-3 text-[0.8125rem] in-data-[slot=button-group]:rounded-lg has-data-[icon=inline-end]:pr-2 has-data-[icon=inline-start]:pl-2 [&_svg:not([class*='size-'])]:size-3.5",
        lg: "h-10 gap-1.5 px-3.5 has-data-[icon=inline-end]:pr-3 has-data-[icon=inline-start]:pl-3",
        icon: "size-8",
        "icon-xs":
          "size-6 rounded-[min(var(--radius-md),10px)] in-data-[slot=button-group]:rounded-lg [&_svg:not([class*='size-'])]:size-3",
        "icon-sm":
          "size-7 rounded-[min(var(--radius-md),12px)] in-data-[slot=button-group]:rounded-lg",
        "icon-lg": "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

const Button = React.forwardRef<
  HTMLButtonElement,
  React.ComponentProps<"button"> &
    VariantProps<typeof buttonVariants> & {
      asChild?: boolean
    }
>(function Button({ className, variant = "default", size = "default", asChild = false, ...props }, ref) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      ref={ref}
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
})

export { Button, buttonVariants }
