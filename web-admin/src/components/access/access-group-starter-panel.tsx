import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type AccessGroupStarterCard = {
  accessRole: string
  matchedMemberCount: number
  name: string
  permissionPreset: string
  position: string
}

type AccessGroupStarterPanelProps = {
  items: AccessGroupStarterCard[]
  onCreate: () => void
}

export function AccessGroupStarterPanel({ items, onCreate }: AccessGroupStarterPanelProps) {
  const { t } = useTranslation()

  if (items.length === 0) {
    return null
  }

  return (
    <Card>
      <CardHeader className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <div>
          <CardTitle className="text-base">
            {t("accessPage.components.groupStarterPanel.title", { defaultValue: "Quick-create baseline user groups" })}
          </CardTitle>
          <CardDescription>
            {t("accessPage.components.groupStarterPanel.description", {
              defaultValue:
                "When employee directory exists but baseline grouping is missing, generate default groups by role templates in one click.",
            })}
          </CardDescription>
        </div>
        <Button size="sm" variant="outline" onClick={onCreate}>
          {t("accessPage.components.groupStarterPanel.createButton", {
            defaultValue: "Create {{count}} baseline groups",
            count: items.length,
          })}
        </Button>
      </CardHeader>
      <CardContent className="grid gap-3 lg:grid-cols-2">
        {items.map((item) => (
          <div key={item.name} className="rounded-xl border bg-muted/10 px-4 py-3">
            <div className="flex flex-wrap items-center gap-2">
              <p className="font-medium">{item.name}</p>
              <Badge variant="secondary">
                {t("accessPage.components.groupStarterPanel.matchedEmployees", {
                  defaultValue: "{{count}} matched employees",
                  count: item.matchedMemberCount,
                })}
              </Badge>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">{item.permissionPreset}</p>
            <p className="mt-1 text-xs text-muted-foreground">
              {t("accessPage.components.groupStarterPanel.matchBasis", {
                defaultValue: "Match basis: {{position}} / role={{accessRole}}",
                position: item.position,
                accessRole: item.accessRole,
              })}
            </p>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
