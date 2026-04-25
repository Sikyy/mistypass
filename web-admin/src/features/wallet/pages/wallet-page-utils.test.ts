import { describe, expect, it } from "vitest"

import type { WalletPassTemplate, WalletPhysicalCardTask } from "@/lib/api"

import {
  getTemplateAccessMedium,
  getTemplateDeliveryMethod,
  inferTemplateScenario,
  nextPhysicalCardTaskActions,
  parseStyleConfig,
  parseTargetIDs,
} from "./wallet-page-utils"

function createTemplate(
  overrides: Partial<WalletPassTemplate> & Pick<WalletPassTemplate, "class_id" | "name" | "pass_type">
): WalletPassTemplate {
  const { class_id, name, pass_type, ...rest } = overrides
  return {
    class_id,
    created_at: "2026-04-24T00:00:00Z",
    id: "tpl_1",
    name,
    pass_type,
    provider: "mock_provider",
    status: "active",
    tenant_id: "tenant_demo",
    updated_at: "2026-04-24T00:00:00Z",
    ...rest,
  }
}

function createPhysicalTask(
  overrides: Pick<WalletPhysicalCardTask, "status" | "task_type">
): WalletPhysicalCardTask {
  return {
    created_at: "2026-04-24T00:00:00Z",
    created_by: "system",
    id: "task_1",
    pass_id: "pass_1",
    pass_status: "issued",
    status: overrides.status,
    target_id: "user_1",
    target_type: "user",
    task_type: overrides.task_type,
    template_id: "tpl_1",
    tenant_id: "tenant_demo",
    updated_at: "2026-04-24T00:00:00Z",
    updated_by: "system",
  }
}

describe("wallet page helpers", () => {
  it("parses target ids across separators and removes duplicates", () => {
    expect(parseTargetIDs(" emp-1,\nemp-2; emp-1 \n\n emp-3 ")).toEqual(["emp-1", "emp-2", "emp-3"])
  })

  it("parses style config from JSON and key-value text", () => {
    expect(parseStyleConfig('{"brand_color":"#0f766e","delivery_method":"wallet","empty":"  " }')).toEqual({
      brand_color: "#0f766e",
      delivery_method: "wallet",
    })
    expect(parseStyleConfig("brand_color=#155e75\ndelivery_method: email_qr\nignored")).toEqual({
      brand_color: "#155e75",
      delivery_method: "email_qr",
    })
    expect(parseStyleConfig("")).toBeUndefined()
  })

  it("infers scenario and fallback delivery/access metadata from template shape", () => {
    const employeePhysical = createTemplate({
      class_id: "employee-card",
      name: "Employee card workflow",
      pass_type: "employee",
      style_config: {
        card_workflow: "enabled",
      },
    })
    const visitorTemporary = createTemplate({
      class_id: "visitor-pass",
      name: "Temporary visitor pass",
      pass_type: "visitor",
      style_config: {},
    })
    const visitorQr = createTemplate({
      class_id: "visitor-qr",
      name: "Visitor QR",
      pass_type: "visitor",
      style_config: {},
    })

    expect(inferTemplateScenario(employeePhysical)).toBe("employee_physical")
    expect(inferTemplateScenario(visitorTemporary)).toBe("visitor_temporary")
    expect(getTemplateDeliveryMethod(visitorQr)).toBe("email_qr")
    expect(getTemplateAccessMedium(employeePhysical)).toBe("physical_card")
    expect(getTemplateAccessMedium(visitorQr)).toBe("qr_code")
  })

  it("derives next physical card task actions by task type and status", () => {
    const t = ((key: string) => key) as never

    expect(
      nextPhysicalCardTaskActions(
        t,
        createPhysicalTask({
          status: "queued",
          task_type: "issue",
        })
      )
    ).toEqual([
      { label: "walletPage.actions.physicalTask.startPrinting", status: "printing" },
      { label: "walletPage.actions.physicalTask.ready", status: "ready" },
      { label: "walletPage.actions.physicalTask.issueDirectly", status: "issued" },
      { label: "walletPage.actions.physicalTask.cancel", status: "cancelled" },
    ])

    expect(
      nextPhysicalCardTaskActions(
        t,
        createPhysicalTask({
          status: "queued",
          task_type: "loss_report",
        })
      )
    ).toEqual([
      { label: "walletPage.actions.physicalTask.confirmLoss", status: "reported_lost" },
      { label: "walletPage.actions.physicalTask.cancel", status: "cancelled" },
    ])
  })
})
