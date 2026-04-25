# 印尼 HRIS 厂商与字段映射清单

当前能力状态：

- `CONTRACT_READY`：Talenta 单厂商字段映射、事件边界与交付路径已收口，可作为当前 connector 配置、联调与验收基线；其他厂商仅保留调研基线。
- `FOUNDATION_IN_PROGRESS`：厂商优先级、字段映射与 Talenta 公开 schema 基线已整理；`Talenta` 单厂商主线现已收口，`webhook/HMAC/normalizer/pull/hybrid/async receipt/DLQ/worker alerts/runtime UI` 均已落地，仅公开文档未提供的 `leave/time-off` 事件继续单独跟踪；其他厂商仍处于 discovery / adapter 规划阶段，且按当前交付优先级暂时挂起。

## 1. 适用范围

- 用于回答三个问题：
  - 先接哪些印尼 HRIS
  - 每家大概值不值得优先做
  - MistyPass 需要从 HRIS 拿哪些字段和事件
- 本文不尝试给出“正式市场份额”，只给出 `公开客户规模 proxy`。
- 推荐架构和系统边界见 [indonesia-hris-integration-architecture.md](./indonesia-hris-integration-architecture.md)。

## 2. 厂商短名单与优先级

| 厂商 | 公开规模口径 | 公开集成能力 | 推荐优先级 | 更适合的客户 |
|---|---|---|---|---|
| `Mekari Talenta` | `35,000+` businesses | `HMAC`、API、webhook、Marketplace | `P0` | 本地 SMB 到大型企业 |
| `Gadjian` | `10,000+` businesses in Indonesia | Pricing 页公开 `Open API Access` | `P0/P1` | 本地 SMB、payroll-heavy 客户 |
| `GreatDay HR` | `3,000+` companies、`1,000,000+` users | 公开 `Open API`，涵盖 employee/attendance/leave/overtime 等 | `P1` | 连锁、多门店、多班次 |
| `SunFish HR` | `2,000+` companies | `open API`、`xDBC`、`batch file` | `P1/P2` | 大中型、集团、复杂组织 |
| `LinovHR` | `500+` companies | `OpenAPI` + `sandbox` | `P1` | 需要快速 PoC 的中型客户 |

## 3. 公开规模 proxy

### 3.1 先说结论

- 没有查到可信、公开、可审计的“印尼 HRIS vendor market share”第三方报告可直接引用。
- 所以下表只能作为 `connector 优先级参考`，不能当财务或融资材料使用。

### 3.2 计算方法

`normalized_public_scale = 官方公开客户数 / 五家公开客户数总和`

注意：

- 指标口径不完全一致，有的是 `businesses`，有的是 `companies`，有的是区域客户数。
- `GreatDay HR` 和 `SunFish` 属于同一集团技术谱系，存在区域与客户口径重叠的可能。
- 因此下面是 `公开规模 proxy`，不是 `真实印尼市占率`。

| 厂商 | 公开数值 | 归一化 proxy |
|---|---:|---:|
| `Talenta` | `35,000` | `69.3%` |
| `Gadjian` | `10,000` | `19.8%` |
| `GreatDay HR` | `3,000` | `5.9%` |
| `SunFish HR` | `2,000` | `4.0%` |
| `LinovHR` | `500` | `1.0%` |

实操建议：

- 如果你要决定先做哪三家，按 `Talenta -> Gadjian -> GreatDay` 排序更稳。
- 如果你的客单价更偏 enterprise，改为 `Talenta -> SunFish -> GreatDay`。

## 4. MistyPass 最小字段模型

> 实现状态参见 [architecture doc § 6](./indonesia-hris-integration-architecture.md#6-canonical-employee-model)

| Canonical 字段 | 必填 | 访问控制用途 | 备注 | 实现状态 |
|---|---|---|---|---|
| `external_id` | 是 | 上游主键、幂等、停用 | 优先使用 HRIS employee id | ✅ 已实现 |
| `employee_number` | 否 | 工号展示、卡号关联 | 与 `external_id` 分离 | ✅ 已实现 |
| `full_name` | 是 | 前台展示、人工核验 | | ✅ 已实现 |
| `email` | 否 | SSO、通知、域名校验 | 最好拿工作邮箱 | ✅ 已实现 |
| `phone` | 否 | 通知、人工核验 | 手机优先 | ✅ 已实现 |
| `department` | 否 | 默认权限模板 | | ✅ 已实现 |
| `job_title` | 否 | 默认权限模板 | | ✅ 已实现 |
| `manager_external_id` | 否 | 审批流或升级流程 | | ✅ 已实现 |
| `location` | 否 | 映射 `building_id` / site | 分支、branch、office 都可归一 | ✅ 已实现 |
| `employment_status` | 是 | 开通/停用/冻结 | 必须归一为 `active/inactive` | ✅ 已实现 |
| `join_date` | 否 | 生效起始 | | ✅ 已实现 |
| `resign_date` | 否 | 撤权时间 | | ✅ 已实现 |
| `shift_code` | 否 | 班次门禁 | | ✅ 已实现 |
| `schedule_window` | 否 | 时段门禁 | | ✅ 已实现 |
| `leave_status` | 否 | 特殊门区规则 | 默认不直接停用全部权限 | ✅ 已实现 |
| `cost_center` | 否 | 报表或高级分组 | | ✅ 已实现 |
| `photo_url` | 否 | 头像 / 视觉核验 | 不作为人脸模板真值 | ✅ 已实现 |

## 5. 最小事件清单

| 事件类别 | 对门禁的重要性 | 说明 |
|---|---|---|
| `employee.created` | 高 | 自动开通 |
| `employee.updated` | 高 | 更新部门、岗位、地点 |
| `employee.deleted` | 高 | 快速停用 |
| `employee.resigned` | 高 | 按离职日撤权 |
| `employee.transferred` | 高 | 调楼宇、调区域 |
| `shift.changed` | 中高 | 班次门禁 |
| `schedule.changed` | 中高 | 周期排班门禁 |
| `leave.approved` | 中 | 只在客户需要时参与访问控制 |
| `attendance.live` | 中 | 更适合审计或在场状态，不适合作为唯一开门依据 |

## 6. Talenta 精确字段映射

说明：Talenta 是目前公开资料最完整的一家，下面字段来自公开 webhook schema，可直接作为第一版 adapter 蓝图。

| Canonical 字段 | Talenta 字段 | 用法 |
|---|---|---|
| `external_id` | `employment.employee_id` | 主匹配键 |
| `full_name` | `personal.first_name + personal.last_name` | 展示名 |
| `email` | `personal.email` | 通知 / SSO |
| `phone` | `personal.mobile_phone`，回退 `personal.phone` | 联系方式 |
| `department` | `employment.organization_name` | 权限模板 |
| `job_title` | `employment.job_position` | 权限模板 |
| `manager_external_id` | `employment.approval_line_employee_id` | 审批或升级 |
| `location` | `employment.branch` | 站点映射 |
| `employment_status` | `employment.status` + `employment.employment_status` | 归一 `active/inactive` |
| `join_date` | `employment.join_date` | 生效起始 |
| `resign_date` | `employment.resign_date` | 撤权日期 |
| `cost_center` | `payroll_info.cost_center_name` | 高级分组 |
| `photo_url` | `personal.avatar` | 头像 |
| `custom_attributes` | `custom_field[]` | 厂商自定义扩展 |

Talenta 公开可用事件：

| 事件 | 公开 event name | 适合用途 |
|---|---|---|
| 新员工 | `talenta.employee.detail.created` | 开通员工与初始权限 |
| 员工资料更新 | `talenta.employee.detail.updated` | 更新主档字段与访问映射 |
| 删除员工 | `talenta.employee.detail.deleted` | 停用或软删除 |
| 调岗调组织 | `talenta.employee.transfer.approved` | 更新部门、岗位、branch |
| 调岗取消 | `talenta.employee.transfer.cancelled` | 回滚原计划中的部门、岗位、branch 变更 |
| 离职 | `talenta.employee.resignation.created` | 定时撤权 |
| 取消离职 | `talenta.employee.resignation.cancelled` | 恢复在职并清空既有撤权日期 |
| 实时考勤 | `talenta.attendance.liveattendance` | 审计、在场状态 |
| 班次调整 | `talenta.attendance.scheduler.changeshift` | 门禁时段更新 |
| 排班调整 | `talenta.attendance.scheduler.changeschedule` | 周期时段更新 |

当前实现进度：

- 已落地首版 `Talenta` normalizer，并接入 `created`、`updated`、`deleted`、`transfer.approved/cancelled`、`resignation.created/cancelled` 七类主员工事件；员工主档 payload 中的 `employee_number / join_date / resign_date / leave_status / cost_center / photo_url` 也已归一化写入 canonical model，且 `leave_status / join_date / cost_center / photo_url` 已补 inline webhook / async receipt worker / pull / hybrid 四条链路一致性回归；`changeshift / changeschedule` 的稀疏 merge 也已补 inline / async receipt worker / direct DLQ replay / direct DLQ worker，以及 `receipt worker -> DLQ -> manual replay`、`receipt worker -> DLQ -> DLQ worker`、`stale replaying recovery`、`fresh replaying skip` 四类异步恢复/跳过路径上的保留扩展字段回归。
- 最新已把 Talenta 官方 `employment-only payload` 事件也对齐到现有 merge 语义：`transfer.approved/cancelled` 与 `resignation.created/cancelled` 即便缺少 `personal.email` 也能先归一化，再 merge 既有员工，再进入 sync；其中 `resignation.cancelled` 还会显式清空既有 `resign_date`。对应 inline webhook、async receipt worker 与 `resignation.cancelled` 的 hybrid `webhook + pull worker` 回归已补齐。
- 2026-04-23 重新核对 Mekari 官方 Talenta webhook 目录后，`detail.updated`、`transfer.cancelled`、`resignation.cancelled` 已可确认有公开页面，但当前仍未发现 `leave/time-off` webhook 文档；因此 leave 事件暂不硬编码猜测 event name。
- 以当前公开 webhook 目录为边界，Talenta 单厂商范围已完成收口；剩余只在公开目录未提供的 `leave/time-off` 事件，不再阻塞 Talenta vendor 完成度判断。
- 已完成 Mekari Marketplace 路径评估：当前 Marketplace 更适合作为 add-ons 的采购 / 激活入口，而不是 MistyPass Talenta connector 的主技术通路；因此默认交付继续以 `API + webhook + pull/hybrid` 为主，仅在客户明确要求 Marketplace 安装入口时再单独推进。
- 已补入 `Talenta` webhook HMAC 验签，当前按 Mekari 官方 `Authorization + Date + Digest` 规则校验；`webhook_secret_ref` 用于签名 secret，`credential_ref` 可选用于比对 `client_id`。
- 已新增 `Talenta` pull adapter 基线，当前通过公开员工列表端点做分页拉取，并在 nightly reconcile 中把“本次全量未出现”的既有 Talenta 员工置为 `inactive`。
- 已补入首版 incremental hardening：worker 在 full reconcile 周期之间默认按 Talenta 内建 query 契约走 incremental pull（默认 `updated_after` / `updated_before` + `rfc3339`），connector credential 仅作为 override 使用。
- 已接入 `scheduler.changeshift`、`scheduler.changeschedule` 两类排班事件：
  - `changeshift` 映射 `shift_code` 与单班次 `schedule_window`
  - `changeschedule` 映射周期 `schedule_window`，仅在班次唯一时覆盖 `shift_code`
  - 由于官方 payload 属于稀疏变更事件，当前会先按 `external_id` merge 既有员工，再进入 canonical sync
- `attendance.liveattendance` 继续作为 deferred 事件保留，当前已在 webhook 路径显式落为 `skipped + deferred audit`，仅建议用于审计或在场态扩展，不直接驱动员工同步。

Talenta 接入 checklist：

- 鉴权：确认客户是否已开通 Talenta API 所需 scope。
- 凭证：按租户单独保存 `CLIENT_ID` / `CLIENT_SECRET`，不可复用跨客户凭证。
- webhook：确认是否提供签名机制、重试策略、IP allowlist。
- 拉取：当前已接入员工列表分页 pull；默认使用 Talenta 内建 incremental query 契约做 hourly incremental + daily full reconcile，若客户租户的真实 API 合同不同，再通过 credential 显式 override query 名或时间格式。
- 身份：确认 `employee_id` 是否在整个公司生命周期内稳定不变。
- 组织：确认 `organization_id`、`branch_id` 是否稳定，名称变更时 ID 是否保留。
- 市场：如果客户需要采购入口，再评估 `Mekari Marketplace` 上架，而不是反过来。

## 7. 其他厂商字段映射基线

说明：下表中 `Talenta` 以外的列采用“公开模块 / 业务对象”口径，而不是伪造的 endpoint 字段名。真正实施时，需以销售后提供的 API 文档或 sandbox schema 替换。

| Canonical 字段 | Gadjian | GreatDay HR | LinovHR | SunFish HR |
|---|---|---|---|---|
| `external_id` | Employee Database / employee code | Employee Data / employee id | Employee Management / employee id | Core HR / employee id |
| `full_name` | Employee Database / full name | Employee Data / full name | Employee Management / employee name | Core HR / employee name |
| `email` | Employee Database / work email | Employee Data / email | Employee Management / email | Core HR / work email |
| `phone` | Employee Database / phone | Employee Data / mobile phone | Employee Management / phone | Core HR / contact number |
| `department` | Organizational Structure / department | Company Data / org unit | Employee Management / department | Core HR / organization unit |
| `job_title` | Position Structure / title | Employee Data / position | Employee Management / position | Core HR / position |
| `manager_external_id` | Approval / supervisor employee code | Employee Data / supervisor | Employee Management / manager | Core HR / line manager |
| `location` | Attendance Location / branch | Company Data / branch or work site | Employee Management / branch | Core HR / site / branch |
| `employment_status` | Employee status / resign / active | Employee Data / employment status | Employee Management / active or deactivate | Core HR / employment status |
| `join_date` | Employee Database / join date | Employee Data / join date | Employee Management / hire date | Core HR / hire date |
| `resign_date` | Resignation / exit date | Employee Data / resignation date | Employee Management / deactivate date | Core HR / termination date |
| `shift_code` | Work Calendar & Shift Scheduling | Attendance / shift | Attendance & Timesheet / shift | Time & Attendance / shift |
| `schedule_window` | Attendance / work pattern | Attendance / tracking report | Attendance & Timesheet / schedule | Time & Attendance / schedule |
| `leave_status` | Leave / Time-Off Requests | Leave | Leave & Absence | Leave module |
| `cost_center` | Payroll / salary structure | Company Data / cost center if available | Payroll | Payroll / cost center |
| `photo_url` | Employee profile photo | Employee Data / avatar | Employee Management / photo | Core HR / profile photo |

## 8. Gadjian 接入策略

公开确认：

- 首页写明已服务 `10,000+` 印尼企业。
- Pricing 页公开 `Open API Access`，描述为“将其他系统中的 employee data 自动集成到 Gadjian”。
- 产品公开能力覆盖 employee database、organizational structure、attendance、shift、leave、mobile app。

建议接法：

- 主路径：`API pull + scheduled reconcile`
- 备选：如果 webhook 不开放，使用定时拉取或批量导出
- 适合优先读取：employee、department、position、attendance location、shift、leave、resignation

上线前必须确认：

- Open API 是只支持导入到 Gadjian，还是也支持读取 Gadjian 员工数据
- 鉴权模式是 API key、OAuth 还是 partner token
- 是否支持 webhook
- 是否有 `employee status` / `resign_date` / `branch` 相关字段
- 是否能按 `updated_at` 增量拉取
- 是否有 sandbox 或 demo tenant 可用于联调

接入判断：

- 如果 API 只偏“写入 Gadjian”，它对 MistyPass 作为下游门禁 SaaS 的价值会下降。
- 但 Gadjian 的本地客户覆盖很值得你优先把商务通道打通。

## 9. GreatDay HR 接入策略

公开确认：

- 官网公开 `3000+` 公司、`1,000,000+` 用户。
- IT 页面明确写有 `Open API`，可集成模块包括 `Attendance`、`Company Data`、`Employee Data`、`Leave`、`Overtime`、`Push Notification`、`Activity Report`、`Tracking Report`。
- DataOn 官方说明 GreatDay HR 是基于 SunFish 技术、面向 `up to 200 staff` 的 SME 产品。

建议接法：

- 对 MistyPass 最有价值的是：`Employee Data + Attendance + Leave + Overtime + Tracking Report`
- 如果客户依赖班次或移动考勤，GreatDay 的价值高于单纯 payroll 型 HRIS
- `Push Notification` 不应作为权限同步主机制，但可用于联动提醒

上线前必须确认：

- 是否有 webhook，还是仅支持 pull API
- `Company Data` 是否包含 branch / site / org master data
- `Tracking Report` 是否能稳定提供 location 或 schedule 信息
- 离职、停用、休假是否有标准化状态值
- Open API 文档是否自助可得，还是需销售开通

接入判断：

- 如果你的门禁产品强调 `班次门禁`、`现场考勤联动`、`多门店出勤治理`，GreatDay 应排在 Gadjian 之前。

## 10. LinovHR 接入策略

公开确认：

- Open API 页面公开 `500+` 客户。
- 提供 `Employee Management`、`Attendance & Timesheet`、`Leave & Absence`、`Payroll`、`Recruitment`、`Performance Management`。
- 明确提供 `sandbox`、`dummy data`、交互式文档，并说明审批后给 `API Key`。

建议接法：

- LinovHR 是很适合做 `connector framework` 第二家样板的厂商。
- 先把 `employee lifecycle`、`attendance`、`leave`、`deactivate` 跑通，再决定是否要接 payroll 衍生字段。

上线前必须确认：

- API rate limit 与分页规则
- API key 生命周期与轮换
- sandbox 与 production schema 是否完全一致
- 是否支持 webhook；如果不支持，增量拉取粒度如何
- attendance/schedule 是否能满足门禁时间窗计算

接入判断：

- 如果你要快速证明“同一套 canonical connector 可以接多家印尼 HRIS”，LinovHR 的公开 sandbox 条件很好。

## 11. SunFish HR 接入策略

公开确认：

- DataOn / SunFish 公开 `2,000+` 企业客户。
- 官网明确支持 `open API`、`xDBC`、`batch file integrations`，并带 configurable data mapping、pre/post processing、translation tools。
- 能力明显偏 enterprise 集成，而非纯自助式 SMB API。

建议接法：

- 先定义 MistyPass 的 canonical mapping，再把 SunFish 视作 `项目制 adapter`。
- 如果客户是集团、多实体、私有云或本地部署，SunFish 的优先级会上升。
- 对 SunFish 不要预设“所有租户都能直接用同一套 public API”；要接受每个项目略有差异。

上线前必须确认：

- 客户部署形态：cloud、private cloud、on-premise
- 可开放通路：open API、xDBC、batch file 哪个可用
- 是否能提供 employee master、org unit、site、status、termination、schedule 数据
- 是否存在客户自定义字段映射或翻译层
- 如果是客户私有环境，MistyPass 是否需要专线、IP allowlist 或代理部署

接入判断：

- SunFish 值得做，但它更像 enterprise 方案包，而不是“做完一个公开 app 就全市场通吃”。

## 12. 插件层还是 API 层

| 厂商 | 先做什么 | 原因 |
|---|---|---|
| `Talenta` | `API/webhook` 先，`Marketplace` 后 | 已有公开 API 与 webhook，Marketplace 更适合作为安装入口 |
| `Gadjian` | `API` 先 | 无公开 marketplace 证据，先打通 partner docs |
| `GreatDay HR` | `API` 先 | Open API 已公开，但更像项目或销售驱动接入 |
| `LinovHR` | `API` 先 | 有 sandbox，适合先做标准 connector |
| `SunFish` | `project integration` 先 | 通常是 open API / xDBC / batch file 三选一 |

产品结论：

- 你的核心资产应该是 `canonical model + connector runtime + policy engine`。
- 插件层只负责 `安装、授权、目录展示、销售入口`。
- 不要把 MistyPass 做成“寄生在某一家 HRIS 插件槽里”的产品。

## 13. 推荐的 connector 落地顺序

1. `Talenta`
2. `Gadjian` 或 `GreatDay HR`
3. `LinovHR`
4. `SunFish HR`

排序逻辑：

- `Talenta`：公开技术资料最成熟。
- `Gadjian`：本地市场覆盖大，值得尽早打商务通道。
- `GreatDay HR`：如果你卖给多班次、多站点客户，优先级可高于 Gadjian。
- `LinovHR`：最适合验证多厂商 connector 框架的可复用性。
- `SunFish`：enterprise 价值高，但交付周期和售前成本也更高。

## 14. 公开来源

- Mekari Talenta 产品页: <https://mekari.com/en/product/talenta/>
- Mekari HMAC 鉴权: <https://developers.mekari.com/docs/kb/hmac-authentication>
- Talenta Webhooks 总览: <https://developers.mekari.com/docs/kb/webhooks/talenta>
- Talenta 新员工事件: <https://developers.mekari.com/docs/kb/webhooks/talenta/talenta-employee-detail-created>
- Talenta 删除员工事件: <https://developers.mekari.com/docs/kb/webhooks/talenta/talenta-employee-detail-deleted>
- Talenta 调岗事件: <https://developers.mekari.com/docs/kb/webhooks/talenta/talenta-employee-transfer-approved>
- Talenta 离职事件: <https://developers.mekari.com/docs/kb/webhooks/talenta/talenta-employee-resignation-created>
- Talenta 实时考勤: <https://developers.mekari.com/docs/kb/webhooks/talenta/talenta-attendance-liveattendance>
- Talenta 班次变化: <https://developers.mekari.com/docs/kb/webhooks/talenta/talenta-attendance-scheduler-changeshift>
- Talenta 排班变化: <https://developers.mekari.com/docs/kb/webhooks/talenta/talenta-attendance-scheduler-changeschedule>
- Mekari Marketplace: <https://help-center.mekari.com/hc/en-us/articles/13899478186521-How-to-use-Marketplace-on-Mekari-Account>
- Gadjian 首页: <https://www.gadjian.com/home>
- Gadjian Pricing / Open API: <https://www.gadjian.com/en/pricing>
- GreatDay HR 首页: <https://greatdayhr.com/id-id/>
- GreatDay HR IT / Open API: <https://greatdayhr.com/en-en/role/it-support/>
- LinovHR Open API: <https://www.linovhr.com/open-api/>
- DataOn / SunFish 首页: <https://dataon.com/en-en/>
- SunFish HR 产品页: <https://dataon.com/en-en/sunfish-hr/>
- DataOn About / GreatDay 与 SunFish 关系: <https://dataon.com/en-en/about-us/>
