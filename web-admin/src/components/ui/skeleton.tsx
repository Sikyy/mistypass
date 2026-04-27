import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const skeletonVariants = cva("animate-pulse rounded-md", {
  variants: {
    variant: {
      default: "bg-muted",
      task: "bg-[#f2f2f2]",
    },
  },
  defaultVariants: {
    variant: "default",
  },
})

function Skeleton({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<"div"> & VariantProps<typeof skeletonVariants>) {
  return (
    <div
      data-slot="skeleton"
      data-variant={variant}
      className={cn(skeletonVariants({ variant }), className)}
      {...props}
    />
  )
}

export { Skeleton, skeletonVariants }
