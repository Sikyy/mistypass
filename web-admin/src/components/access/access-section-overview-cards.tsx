import { Link } from "react-router"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

type AccessSection = "directory" | "policies" | "grants"

type AccessSectionOverviewItem = {
  value: AccessSection
  title: string
  description: string
  metric: string
  helper: string
}

type AccessSectionOverviewCardsProps = {
  sections: AccessSectionOverviewItem[]
  activeSection: AccessSection
  onGoToSection: (next: AccessSection) => void
  showDirectoryImportAction: boolean
  enterpriseHomeLink: string
}

export function AccessSectionOverviewCards({
  sections,
  activeSection,
  onGoToSection,
  showDirectoryImportAction,
  enterpriseHomeLink,
}: AccessSectionOverviewCardsProps) {
  const { t } = useTranslation()

  return (
    <div className="grid gap-4 xl:grid-cols-3">
      {sections.map((section) => (
        <Card
          key={section.value}
          className={activeSection === section.value ? "border-primary/40 bg-primary/5" : undefined}
        >
          <CardHeader className="pb-3">
            <CardTitle className="text-base">{section.title}</CardTitle>
            <CardDescription>{section.description}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-sm font-medium">{section.metric}</p>
            <p className="mp-kpi-note">{section.helper}</p>
            <div className="flex items-center gap-2">
              <Button
                variant={activeSection === section.value ? "default" : "outline"}
                size="sm"
                onClick={() => onGoToSection(section.value)}
              >
                {activeSection === section.value
                  ? t("accessPage.components.sectionOverview.currentSection", { defaultValue: "Current section" })
                  : t("accessPage.components.sectionOverview.enterSection", { defaultValue: "Enter section" })}
              </Button>
              {section.value === "directory" && showDirectoryImportAction ? (
                <Button asChild variant="outline" size="sm">
                  <Link to={enterpriseHomeLink}>
                    {t("accessPage.components.sectionOverview.goImportEmployees", { defaultValue: "Import employees in Enterprise" })}
                  </Link>
                </Button>
              ) : null}
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
