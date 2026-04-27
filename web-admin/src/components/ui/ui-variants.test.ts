import { describe, expect, it } from "vitest"
import type * as React from "react"

import { badgeVariants } from "@/components/ui/badge"
import { buttonVariants } from "@/components/ui/button"
import { cardVariants } from "@/components/ui/card"
import { skeletonVariants } from "@/components/ui/skeleton"
import { TabsTrigger } from "@/components/ui/tabs"

describe("ui design variants", () => {
  it("exposes the task card token classes", () => {
    expect(cardVariants({ variant: "task" })).toContain("rounded-card")
    expect(cardVariants({ variant: "task" })).toContain("bg-card-task")
    expect(cardVariants({ variant: "task" })).toContain("border-card-task-border")
  })

  it("exposes the interaction button token classes", () => {
    expect(buttonVariants({ variant: "interaction" })).toContain("hover:text-interaction")
    expect(buttonVariants({ variant: "interaction" })).toContain("focus-visible:border-interaction")
  })

  it("exposes semantic badge variants", () => {
    expect(badgeVariants({ variant: "success" })).toContain("text-emerald-300")
    expect(badgeVariants({ variant: "warning" })).toContain("text-amber-300")
    expect(badgeVariants({ variant: "danger" })).toContain("text-red-300")
  })

  it("uses interaction blue for line tab indicators", () => {
    const trigger = TabsTrigger({ value: "sample" }) as React.ReactElement<{ className: string }>

    expect(trigger.props.className).toContain("after:bg-interaction")
    expect(trigger.props.className).toContain("data-active:text-interaction")
  })

  it("exposes the task skeleton surface", () => {
    expect(skeletonVariants({ variant: "task" })).toContain("bg-[#f2f2f2]")
  })
})
