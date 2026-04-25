import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { AccessDomainBanner } from "@/components/access/access-domain-banner"
import { AccessGroupForm } from "@/components/access/access-group-form"
import { AccessGroupLedgerTable } from "@/components/access/access-group-ledger-table"
import { AccessGroupStarterPanel } from "@/components/access/access-group-starter-panel"
import { AccessRoleTemplateTable } from "@/components/access/access-role-template-table"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { type UserGroup } from "@/lib/api"

type AccessDirectoryStarterItem = {
  accessRole: string
  matchedMemberCount: number
  name: string
  permissionPreset: string
  position: string
}

type AccessDirectoryEmployee = {
  email: string
  fullName: string
  id: string
}

type AccessDirectoryLedgerRow = {
  descriptionLabel: string
  group: UserGroup
  membersLabel: string
}

type AccessDirectoryRoleTemplate = {
  accessRole: string
  defaultGroup: string
  permissionPreset: string
  position: string
}

type AccessDirectorySectionProps = {
  filteredEmployees: AccessDirectoryEmployee[]
  groupDescription: string
  groupLedgerEmptyState: string
  groupLedgerRows: AccessDirectoryLedgerRow[]
  groupMemberQuery: string
  groupName: string
  isEditingGroup: boolean
  onCreateStarterGroups: () => void
  onDescriptionChange: (value: string) => void
  onEditGroup: (group: UserGroup) => void
  onMemberQueryChange: (value: string) => void
  onNameChange: (value: string) => void
  onSubmitGroup: (payload: { name: string; description: string }) => void
  onToggleMember: (employeeID: string) => void
  roleTemplateItems: AccessDirectoryRoleTemplate[]
  selectedMemberIDs: string[]
  showStarterPanel: boolean
  starterItems: AccessDirectoryStarterItem[]
}

export function AccessDirectorySection({
  filteredEmployees,
  groupDescription,
  groupLedgerEmptyState,
  groupLedgerRows,
  groupMemberQuery,
  groupName,
  isEditingGroup,
  onCreateStarterGroups,
  onDescriptionChange,
  onEditGroup,
  onMemberQueryChange,
  onNameChange,
  onSubmitGroup,
  onToggleMember,
  roleTemplateItems,
  selectedMemberIDs,
  showStarterPanel,
  starterItems,
}: AccessDirectorySectionProps) {
  const { t } = useTranslation()

  return (
    <div className="space-y-4">
      <AccessDomainBanner
        title={t("accessPage.components.directorySection.bannerTitle", { defaultValue: "Employees & user groups" })}
        description={t("accessPage.components.directorySection.bannerDescription", {
          defaultValue:
            "This section focuses on organization objects only. Employee directory comes from enterprise sync; groups carry policy and issuance targets.",
        })}
        actions={
          <>
            <Button asChild size="sm" variant="outline">
              <Link to="/enterprise#sync">{t("accessPage.components.directorySection.goEnterpriseSync", { defaultValue: "Go to enterprise sync" })}</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to="/access/policies">
                {t("accessPage.components.directorySection.goPoliciesNext", { defaultValue: "Next: access policies" })}
              </Link>
            </Button>
          </>
        }
      />

      {showStarterPanel ? (
        <AccessGroupStarterPanel items={starterItems} onCreate={onCreateStarterGroups} />
      ) : null}

      <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
        <AccessGroupForm
          filteredEmployees={filteredEmployees}
          groupDescription={groupDescription}
          groupMemberQuery={groupMemberQuery}
          groupName={groupName}
          isEditing={isEditingGroup}
          onDescriptionChange={onDescriptionChange}
          onMemberQueryChange={onMemberQueryChange}
          onNameChange={onNameChange}
          onSubmit={onSubmitGroup}
          onToggleMember={onToggleMember}
          selectedMemberIDs={selectedMemberIDs}
        />

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("accessPage.components.directorySection.groupListTitle", { defaultValue: "User group list" })}</CardTitle>
              <CardDescription>
                {t("accessPage.components.directorySection.groupListDescription", {
                  defaultValue: "Groups can be used immediately for policies, visitor grants, and batch issuance targeting.",
                })}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <AccessGroupLedgerTable rows={groupLedgerRows} emptyState={groupLedgerEmptyState} onEdit={onEditGroup} />
            </CardContent>
          </Card>

          <AccessRoleTemplateTable items={roleTemplateItems} />
        </div>
      </div>
    </div>
  )
}
