import { useTranslation } from "react-i18next"

import { AccessDirectorySection } from "@/components/access/access-directory-section"
import { AccessGrantsSection } from "@/components/access/access-grants-section"
import { AccessPoliciesSection } from "@/components/access/access-policies-section"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

export type AccessSection = "directory" | "policies" | "grants"

type AccessSectionsTabsProps = {
  activeSection: AccessSection
  onSectionChange: (nextSection: AccessSection) => void
  directoryProps: Parameters<typeof AccessDirectorySection>[0]
  policiesProps: Parameters<typeof AccessPoliciesSection>[0]
  grantsProps: Parameters<typeof AccessGrantsSection>[0]
}

export function AccessSectionsTabs({
  activeSection,
  onSectionChange,
  directoryProps,
  policiesProps,
  grantsProps,
}: AccessSectionsTabsProps) {
  const { t } = useTranslation()

  return (
    <Tabs value={activeSection} onValueChange={(value) => onSectionChange(value as AccessSection)} className="space-y-4">
      <TabsList className="grid w-full max-w-2xl grid-cols-3">
        <TabsTrigger value="directory">
          {t("accessPage.components.sectionsTabs.directory")}
        </TabsTrigger>
        <TabsTrigger value="policies">
          {t("accessPage.components.sectionsTabs.policies")}
        </TabsTrigger>
        <TabsTrigger value="grants">
          {t("accessPage.components.sectionsTabs.grants")}
        </TabsTrigger>
      </TabsList>

      <TabsContent value="directory" className="space-y-4">
        <AccessDirectorySection {...directoryProps} />
      </TabsContent>

      <TabsContent value="policies" className="space-y-4">
        <AccessPoliciesSection {...policiesProps} />
      </TabsContent>

      <TabsContent value="grants" className="space-y-4">
        <AccessGrantsSection {...grantsProps} />
      </TabsContent>
    </Tabs>
  )
}
