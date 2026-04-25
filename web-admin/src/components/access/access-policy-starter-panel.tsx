import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"

type PolicyStarterCard = {
  id: string
  groupName: string
  memberCount: number
  name: string
  description: string
  reviewNote: string
  schedule: string
}

type AccessPolicyStarterPanelProps = {
  hasGroups: boolean
  items: PolicyStarterCard[]
  topologyReady: boolean
  onApply: (id: string) => void
}

export function AccessPolicyStarterPanel({
  hasGroups,
  items,
  topologyReady,
  onApply,
}: AccessPolicyStarterPanelProps) {
  const { t } = useTranslation()

  if (!hasGroups) {
    return (
      <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm text-muted-foreground">
        {t("accessPage.components.policyStarterPanel.emptyNoGroups", {
          defaultValue:
            "No reusable user groups yet. Build baseline groups in Employees & Groups first, then return to generate policy drafts.",
        })}
      </div>
    )
  }

  if (items.length === 0) {
    return (
      <div className="rounded-xl border bg-muted/10 px-4 py-3 text-sm text-muted-foreground">
        {t("accessPage.components.policyStarterPanel.emptyNoItems", {
          defaultValue:
            "No new suggested drafts for current groups. Common first-batch policies are mostly in place; continue with manual fine-grained rules or go to issuance center.",
        })}
      </div>
    )
  }

  return (
    <div className="grid gap-3 lg:grid-cols-2">
      {items.map((item) => (
        <div key={item.id} className="rounded-xl border bg-muted/10 px-4 py-3">
          <div className="flex flex-wrap items-center gap-2">
            <p className="font-medium">{item.groupName}</p>
            <Badge variant="secondary">
              {t("accessPage.components.policyStarterPanel.memberCount", {
                defaultValue: "{{count}} members",
                count: item.memberCount,
              })}
            </Badge>
            <Badge variant="outline">{t("accessPage.components.policyStarterPanel.draftBadge", { defaultValue: "Draft" })}</Badge>
          </div>
          <p className="mt-2 text-sm">{item.name}</p>
          <p className="mt-1 text-sm text-muted-foreground">{item.description}</p>
          <p className="mt-2 text-xs text-muted-foreground">{item.reviewNote}</p>
          <p className="mt-1 text-xs text-muted-foreground">
            {t("accessPage.components.policyStarterPanel.suggestedSchedule", {
              defaultValue: "Suggested schedule: {{schedule}}",
              schedule: item.schedule,
            })}
          </p>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Button size="sm" variant="outline" onClick={() => onApply(item.id)}>
              {t("accessPage.components.policyStarterPanel.applyButton", { defaultValue: "Apply to left form" })}
            </Button>
            {!topologyReady ? (
              <Button asChild size="sm" variant="ghost">
                <Link to="/spaces">{t("accessPage.components.policyStarterPanel.goTopology", { defaultValue: "Complete topology" })}</Link>
              </Button>
            ) : null}
          </div>
        </div>
      ))}
    </div>
  )
}
