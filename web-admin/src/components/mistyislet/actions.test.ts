import { createElement } from "react"
import { renderToStaticMarkup } from "react-dom/server"
import { describe, expect, it } from "vitest"

import { RowActionsMenu } from "@/components/mistyislet/actions"

describe("kisi action primitives", () => {
  it("renders an accessible row actions trigger", () => {
    const html = renderToStaticMarkup(
      createElement(RowActionsMenu, {
        label: "Actions for Main Entrance",
        items: [
          {
            id: "edit",
            label: "Edit",
            onSelect: () => undefined,
          },
        ],
      })
    )

    expect(html).toContain('aria-label="Actions for Main Entrance"')
  })

  it("does not render an empty row actions menu", () => {
    expect(renderToStaticMarkup(createElement(RowActionsMenu, { items: [] }))).toBe("")
  })
})
