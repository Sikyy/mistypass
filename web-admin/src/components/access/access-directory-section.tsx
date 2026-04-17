import { type FormEvent } from "react"
import { Link } from "react-router-dom"

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
  onSubmitGroup: (event: FormEvent<HTMLFormElement>) => void
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
  return (
    <div className="space-y-4">
      <AccessDomainBanner
        title="员工与用户组"
        description="这个域只负责整理组织对象。员工目录来自企业同步，用户组负责承接策略与发放对象，不再在这里混入范围策略和访客授权。"
        actions={
          <>
            <Button asChild size="sm" variant="outline">
              <Link to="/enterprise#sync">去企业同步</Link>
            </Button>
            <Button asChild size="sm" variant="outline">
              <Link to="/access/policies">下一步去权限策略</Link>
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
              <CardTitle className="text-base">用户组列表</CardTitle>
              <CardDescription>编辑后可立即用于权限策略、访客授权和批量发放对象选择。</CardDescription>
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
