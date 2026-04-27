import { describe, expect, it } from "vitest"
import type * as React from "react"

import { ActionInbox } from "@/components/action-inbox"
import { DetailDrawer } from "@/components/detail-drawer"
import { EmptyState } from "@/components/empty-state"
import { OperationalKPI } from "@/components/operational-kpi"
import { PermissionBoundary } from "@/components/permission-boundary"
import { ScopeLockedField } from "@/components/scope-locked-field"

describe("business components", () => {
  it("renders locked scope fields as task surfaces", () => {
    const field = ScopeLockedField({ label: "Scope", value: "Building A" }) as React.ReactElement<{ className: string }>

    expect(field.props.className).toContain("rounded-card")
    expect(field.props.className).toContain("bg-card-task")
  })

  it("uses task cards for operational KPI and action inbox surfaces", () => {
    const kpi = OperationalKPI({ label: "Open", value: 3 }) as React.ReactElement<{ variant: string }>
    const inbox = ActionInbox({
      emptyState: { title: "Empty" },
      items: [],
      title: "Queue",
    }) as React.ReactElement<{ variant: string }>

    expect(kpi.props.variant).toBe("task")
    expect(inbox.props.variant).toBe("task")
  })

  it("keeps DetailDrawer wired through the sheet root", () => {
    const drawer = DetailDrawer({
      children: "Body",
      onOpenChange: () => undefined,
      open: true,
      title: "Details",
    }) as React.ReactElement<{ open: boolean }>

    expect(drawer.props.open).toBe(true)
  })

  it("renders a permission fallback when access is denied", () => {
    const fallback = PermissionBoundary({
      allowed: false,
      children: "Allowed",
      title: "Denied",
    }) as React.ReactElement

    expect(fallback.type).toBe(EmptyState)
  })
})
