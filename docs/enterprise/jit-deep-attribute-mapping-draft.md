# 企业 JIT 深属性映射与撤权联动草案（2026-04-14）

当前能力状态：

- `CONTRACT_READY`：JIT 已支持目录缺失自动建档、`inactive` 阻断、本地 trusted user 角色与楼栋范围对齐。
- `SKELETON_ONLY`：SCIM/HRIS 深属性映射、组织审批流、跨系统停用撤权联动仍未收口到统一执行策略。

## 1. 目标

- 固化 JIT 场景的字段映射优先级，避免 OIDC/SAML/目录源之间语义漂移。
- 定义停用撤权联动顺序，确保“禁用优先于授权”。
- 为 R5 后续实现提供可测试的最小规则集。

## 2. 映射优先级（同一次登录）

优先级从高到低：

1. SCIM/HRIS 最新目录快照（若存在且在有效窗口内）。
2. OIDC/SAML 回调身份声明（本次登录实时数据）。
3. 本地已存员工档案（兜底，禁止回填高优先级字段）。

一致性约束：

- `external_id` 一旦绑定，不允许切换到其他员工记录。
- `tenant_id + external_id` 冲突时直接拒绝会话签发并审计。
- `email` 仅可在同一 `external_id` 上更新，不允许“跨 external_id 抢占”。

## 3. 字段映射草案

| 目标字段 | OIDC/SAML 来源 | SCIM/HRIS 来源 | 规则 |
|---|---|---|---|
| `full_name` | `name` / `display_name` | `displayName` | 高优先级覆盖低优先级；空值不覆盖非空 |
| `department` | `department` | `department.name` | 归一化小写后入库 |
| `job_title` | `job_title` / `title` | `title` | 保留原始展示值 + 归一化索引 |
| `location` | `location` | `locations[].name` | 用于 building scope 模板匹配 |
| `phone` | `phone_number` | `phoneNumbers[].value` | E.164 标准化失败则保留原值并打 warning |
| `manager_external_id` | `manager_id` | `manager.value` | 仅作关系字段，不影响登录放行 |
| `employment_status` | `status` | `active`/`status` | `inactive/terminated` 直接阻断会话 |

## 4. 权限模板联动

- 先决条件：`employment_status=active`。
- 角色映射：沿用当前 `department -> access_role` 模板，新增 `job_title` 精细化兜底。
- 范围映射：`location -> building_id`，若映射失败则降级到 tenant 范围最小权限（不授予 building_admin）。

## 5. 停用/撤权联动顺序

1. 检测到 `inactive/terminated`。
2. 立即阻断本次会话签发。
3. 撤销该用户有效 refresh token（best effort，失败重试）。
4. 将本地 trusted user 权限降级为最小权限或冻结状态。
5. 写入审计：`enterprise_jit_deprovision_applied`（含来源、旧角色、新状态、request_id）。

## 6. 回归清单（实现前置）

- `inactive` 用户即使本地已有账号也必须拒绝登录。
- `external_id` 冲突必须返回 `409` 且不改写现存档案。
- 同一用户部门/地点变更后，下次 JIT 登录角色和范围可收敛到新模板。
- 停用后 token 撤销与审计事件可查询。

## 7. 非目标

- 本草案不覆盖跨租户组织架构建模。
- 本草案不覆盖审批工作流 UI，仅定义后端联动规则。
