import { type FormEvent } from "react"
import { Link } from "react-router-dom"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"

type AccessGroupFormEmployee = {
  email: string
  fullName: string
  id: string
}

type AccessGroupFormProps = {
  filteredEmployees: AccessGroupFormEmployee[]
  groupDescription: string
  groupMemberQuery: string
  groupName: string
  isEditing: boolean
  onDescriptionChange: (value: string) => void
  onMemberQueryChange: (value: string) => void
  onNameChange: (value: string) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  onToggleMember: (employeeID: string) => void
  selectedMemberIDs: string[]
}

export function AccessGroupForm({
  filteredEmployees,
  groupDescription,
  groupMemberQuery,
  groupName,
  isEditing,
  onDescriptionChange,
  onMemberQueryChange,
  onNameChange,
  onSubmit,
  onToggleMember,
  selectedMemberIDs,
}: AccessGroupFormProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{isEditing ? "编辑用户组" : "新建用户组"}</CardTitle>
        <CardDescription>先用企业目录确定成员，再把用户组作为策略与发放的基础对象。</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-3" onSubmit={onSubmit}>
          <Input value={groupName} onChange={(event) => onNameChange(event.target.value)} placeholder="用户组名称" />
          <Input value={groupDescription} onChange={(event) => onDescriptionChange(event.target.value)} placeholder="描述" />
          <Input
            value={groupMemberQuery}
            onChange={(event) => onMemberQueryChange(event.target.value)}
            placeholder="从企业员工库搜索成员（姓名/邮箱/部门）"
          />
          <div className="max-h-48 space-y-1 overflow-auto rounded-md border bg-muted/20 p-2">
            {filteredEmployees.length === 0 ? (
              <div className="space-y-2 px-2 py-3">
                <p className="mp-kpi-note">
                  当前组织员工库暂无可选成员，请先去企业页接入 HRIS、SCIM、CSV 或手动同步。
                </p>
                <Button asChild variant="outline" size="sm">
                  <Link to="/enterprise">去企业页导入员工</Link>
                </Button>
              </div>
            ) : null}
            {filteredEmployees.map((employee) => (
              <label
                key={employee.id}
                className="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 hover:bg-muted/40"
              >
                <input
                  type="checkbox"
                  checked={selectedMemberIDs.includes(employee.id)}
                  onChange={() => onToggleMember(employee.id)}
                />
                <span className="min-w-0 text-sm">
                  <span className="font-medium">{employee.fullName}</span>
                  <span className="ml-1 text-xs text-muted-foreground">({employee.email})</span>
                </span>
              </label>
            ))}
          </div>
          <p className="mp-kpi-note">已选择成员：{selectedMemberIDs.length}</p>
          <Button type="submit" className="w-full">
            {isEditing ? "更新用户组" : "创建用户组"}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}
