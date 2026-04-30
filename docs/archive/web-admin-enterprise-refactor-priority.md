# Web Admin 企业态改造路线图（截至 2026-04-17）

当前能力状态：

- `CONTRACT_READY`：路线与接口契约口径已对齐当前主干。
- `PROD_READY`：页面能力状态已对齐当前前端实现（截至 2026-04-17）。

## 0. 读法与分级

- 紧急度：
  - `S0` 必须优先完成，直接影响企业客户是否能顺畅使用管理后台。
  - `S1` 高优先，需与 S0 并行推进，影响后续 1-2 个迭代节奏。
  - `S2` 中优先，在主路径稳定后集中收口。
  - `S3` 依赖外部条件或产品定稿，当前不阻塞主交付。
- 状态定义：
  - `已完成`：该分项已形成稳定前端能力，路由、页面和文档口径一致。
  - `进行中`：已有页面或壳层改造，但主路径仍有缺口。
  - `未完成`：方向已确认，但页面和交互尚未真正落地。
- 进度口径：
  - `0-100%` 代表“信息架构 + 页面实现 + 角色守卫 + 基础验证”的综合完成度。
  - 不代表视觉细节、后续运营增强或端到端业务验收已经全部完成。
- 进度条读法：
  - 示例：`██████░░░░ 60%`

## 1. 高优先级总览（按优先级排序）

| 编号 | 事项 | 紧急度 | 当前状态 | 进度 | 下一里程碑 |
|---|---|---|---|---:|---|
| F1 | 角色态工作台与租户去暴露 | S0 | 已完成 | 100% | 2026-04-16：角色边界回归签字与空状态核验完成 |
| F3 | MistyPass 发放中心重构 | S0 | 已完成 | 100% | 2026-04-16：转入增强项评估（高级运行拆页 / 文案收口） |
| F2 | 企业目录、HRIS、SSO 与审批入口 | S1 | 进行中 | 99% | 2026-04-17：继续把同步来源结果、审批积压和目录异常收口成更完整的企业主路径 |
| F4 | 网关、事件、告警的企业态体验 | S1 | 已完成 | 100% | 2026-04-16：多角色场景行为复核与边界签字完成 |
| F5 | 中文排版、按钮密度与仪表盘视觉收口 | S1 | 已完成 | 100% | 2026-04-17：中文排版/密度/长字段省略规范一次性收口完成 |
| F6 | Access 信息架构拆分（员工/用户组/策略/访客） | S1 | 已完成 | 100% | 2026-04-17：三域独立页面壳 + `/:section` 兼容重定向收口完成 |
| F7 | 构建回归、页面验证与性能治理 | S2 | 已完成 | 100% | 2026-04-17：分支级守护 `build + smoke + role-boundary + browser e2e(108/108) + doc-marker` 通过 |
| F0 | 前端改造方向与优先级冻结 | S0 | 已完成 | 100% | 已完成 |

综合进度：`99%`（`进行中`）

状态说明：

- `已完成`：方向、优先级和核心角色模型已固定，不再继续摇摆在“物业多租户后台”和“企业自营后台”之间。
- `进行中`：主壳层、企业页、发放页、网关/事件页已经开始角色化，但仍有部分页面未完全收口。
- `未完成`：外部 API 语义校验与回执映射增强事项仍待恢复。

## 2. 大项拆解（已完成 / 进行中 / 未完成）

### F0. 前端改造方向与优先级冻结（S0，已完成 100%）

进度条：`██████████ 100%`

已完成子项：

- [x] 确认不再以 `super_admin + 多租户物业运营` 作为唯一产品叙事。
- [x] 确认企业自营登录后默认隐藏租户入口、租户筛选和跨租户汇总。
- [x] 确认 `Wallet` 不是一级心智，统一改为“凭证发放 / MistyPass 发放”。
- [x] 确认企业管理员的主路径优先级为“员工目录 -> 用户组 -> 权限 -> 发放”。
- [x] 建立本路线图文档，后续按与后端一致的口径维护进度。

未完成子项：

- [ ] 无。

### F1. 角色态工作台与租户去暴露（S0，已完成 100%）

进度条：`██████████ 100%`

已完成子项：

- [x] 接入 `/api/v1/me`，建立当前用户上下文。
- [x] 导航和路由按角色生成，企业管理员不再看到 `租户` 顶级入口。
- [x] `Dashboard`、`Spaces`、`Access`、`Wallet`、`Alarms` 已开始按企业态隐藏租户列和租户切换。
- [x] 企业登录后默认锁定当前 `tenant_id`，不再暴露跨租户视图。
- [x] `网关` 和 `事件` 已从“仅平台可用”放开为企业态可用。
- [x] `building_admin` 已可进入 `空间` 页面，并按 `building_ids` 仅看到负责楼宇范围。
- [x] `Dashboard` 与 `告警` 已补楼宇管理员视角文案和范围过滤。
- [x] `事件` 与 `网关` 已补 `building_ids` 范围过滤，楼宇管理员不再看到非本楼宇数据。
- [x] 已修复 `building_admin` 在 `building_ids` 为空时退回全量视图的问题；`Dashboard`、`Spaces`、`Events`、`Alarms`、`Gateways` 现在都会稳定收口为空范围。
- [x] 关键页面已补楼宇范围缺失提示、只读/可操作边界说明和更明确的空状态，不再把“未分配范围”和“真实系统报错”混在一起。
- [x] `Gateways` 已统一只读角色提示文案，明确“按钮禁用或缺失属于权限边界，不是系统异常”，并同步收口库存与命令区提示口径。
- [x] 已清理一批平台叙事残留文案（`Dashboard` 的“企业运营工作台”、`Alarms` 的“告警运营”），改为企业侧处置叙事。
- [x] 已继续清理残余“运营”叙事文案：`Login`、`Events`、`Wallet`（高级运行说明）、`Tenants` 卡片以及导航总览描述均已改为“工作台/值守/排障”口径。
- [x] 角色标签已从“运营人员”调整为“值守人员”，与页面权限边界表达保持一致。
- [x] `Wallet` 已补统一只读边界提示（页面级只读说明 + 关键动作区“只读（权限边界）”标签），明确“按钮禁用或缺失属于权限边界，不是系统异常”。
- [x] 新增角色边界回归脚本 `docs/testing/curl-web-admin-role-boundary-smoke.zsh`，覆盖路由守卫、角色标识、`building_admin` 空范围提示以及只读边界提示契约。
- [x] 企业登录 `SSO` 概览文案已清理“物业多租户”残留叙事，统一为平台级/企业级边界表达。
- [x] 已完成本轮角色边界签字留档：`build + smoke + role-boundary + browser e2e(108/108)` 全通过，签字记录见 `docs/testing/artifacts/web-admin-role-boundary-signoff-20260416.md`。

进行中子项：

- [ ] 无。

未完成子项：

- [ ] 无。

### F2. 企业目录、HRIS、SSO 与审批入口（S1，进行中 99%）

进度条：`█████████░ 99%`

已完成子项：

- [x] 新增企业页，统一承接员工目录、HRIS / SCIM / CSV / 手动导入入口。
- [x] 企业页已接入 SSO 概览、同步任务、JIT 审批和 worker 异常摘要。
- [x] `Access` 页已补“去企业页导入员工”入口，避免用户组页面成为死胡同。
- [x] 企业态和平台态已能共享同一套企业集成信息，但按角色裁剪入口。
- [x] 企业页顶部入口已与页签联动，点击“HRIS / SCIM / 企业 SSO / 审批与异常”会真正切到对应工作区。
- [x] 企业页与 `Access / 凭证发放` 之间已补连续动作入口，用户不再需要自己回忆下一步该去哪。
- [x] 企业页去“凭证发放”的入口已带员工发放场景参数，员工同步完成后可直接落到员工发放面板。
- [x] 企业页已新增“主路径进度 / 当前建议动作”工作区，直接按员工目录、用户组、策略、发放的真实状态给出下一步。
- [x] 企业页已接入用户组、权限策略和已发放凭证计数，不再只停留在目录与同步概览。
- [x] HRIS / CSV / 企业 SSO / 审批与异常入口已改成可操作的快捷入口，会真实切到对应工作区或同步来源。
- [x] 企业页已新增“关键提醒”卡片，会根据 SSO 缺失、同步异常、审批积压和 worker 告警直接提示当前阻塞项，并给出可执行入口。
- [x] 企业页已新增“企业登录落地工作区”，把 SSO 状态、目录来源和审批积压放进同一工作区，不再只剩配置概览。
- [x] 企业页“审批与异常”页签已新增阻塞项处理台，可直接处理企业登录缺口、同步失败和审批积压。
- [x] 企业页“导入与同步”页签已补“同步结果与下一步”工作区，会直接把最近同步结果推向员工目录、用户组、策略或异常处理。
- [x] 同步来源卡片已变成可操作入口，管理员可直接在 HRIS / SCIM / CSV / 手动同步之间切换当前来源。
- [x] 同步任务提交成功后已补后续建议，不再只提示任务已提交。
- [x] 企业页已补“同步异常分类”，把 rejected、停用和空目录风险分开提示，避免所有同步问题都挤在一个失败概念里。
- [x] 企业页已补“同步处理闭环”，把异常分类继续推进到“先去哪里处理、处理后怎么回到目录主路径”的步骤卡。
- [x] 企业页“审批与异常”页签已补“处理完成后回流动作”，异常处理完成后会直接把管理员引回目录、策略或发放主路径。
- [x] 企业页“企业登录”页签已补“完成后的下一步”，企业登录配置完成后会直接回流到审批、目录、策略或发放主路径。
- [x] 企业页 `employees / sync / idp / alerts` 四个工作区已抽成独立组件，主页面聚焦状态编排与跨工作区回流逻辑。
- [x] 企业页“导入与同步”已新增“来源专项反馈”，会按 `HRIS / SCIM / CSV / 手动同步` 分别给出结果状态、校验要点和回流动作，不再只给统一后续提示。
- [x] 企业页“导入与同步”已新增“同步来源状态总览”，可并排查看四类来源最近结果与下一步动作，避免只围绕当前选中来源做局部判断。
- [x] 企业页“导入与同步”已新增“目录到策略主流程连通检查”，会直接显示同步结果、目录、用户组、策略、发放五步状态，并给出当前唯一下一步动作。
- [x] 企业页“同步来源状态总览”已补来源级结果明细（创建/更新/停用/rejected）、最近完成时间和来源校验要点，导入反馈不再只停留在统一描述。
- [x] 企业页“目录到策略主流程连通检查”五个步骤已补直达动作按钮，可按步骤直接回流到目录、策略、发放或异常处理。
- [x] 企业页“审批与异常”已新增“落地与回流动作”三卡，把同步结果、审批积压和目录异常拆成聚焦处理入口，不再只停留在统一阻塞提示。
- [x] 三类落地卡已补“处理后回流”动作，异常处理完成后可直接回流到员工目录、权限策略或凭证发放主路径。
- [x] 企业页“审批与异常”台账已补细粒度筛选：JIT 审批支持状态 + 外部同步状态双维筛选；同步任务支持状态 + 来源双维筛选；worker 告警支持告警级别筛选。
- [x] 审批、同步任务和 worker 告警记录已补逐条处理动作与回流动作，管理员可按记录快速定位处理并回到主路径。
- [x] 企业页“审批积压闭环清单”已按待审批、回写失败、回写进行中三类拆分入口，支持一键定位对应筛选并回流主路径。
- [x] 企业页“目录异常闭环清单”已按未完成、rejected、停用影响三类拆分入口，支持一键定位对应筛选并回流主路径。
- [x] 企业页 JIT 审批台账已接入真实处理动作：`pending` 记录可直接批准/拒绝，非 `pending` 且未回写成功记录可直接标记 `synced`，处理后自动刷新企业页数据。
- [x] 企业页已新增 JIT 审批 review 与外部回写更新的前端 API 封装，并在处理动作执行中补提交中状态，避免重复提交。
- [x] 企业页 JIT 审批台账已补批量处理动作：可对当前筛选结果批量批准/拒绝 `pending`，并可批量标记外部回写 `synced`。
- [x] 批量处理已补成功/失败统计与一次性数据刷新，避免逐条提交后多次重复刷新造成的操作抖动。
- [x] 企业页“审批与异常”已新增落地页导航（总览 / 审批积压 / 目录异常），可按处理目标切换独立落地页，避免所有处理动作混在同一视图。
- [x] 企业页“审批积压闭环清单”已把批量动作沉淀到落地页：支持就地批量批准/拒绝 `pending`，并可按“回写失败/进行中”就地批量标记 `synced`。
- [x] 企业页“目录异常闭环清单”已补 worker 告警堆积维度，可直接定位并承接目录异常落地页闭环处理。
- [x] 企业页到 `Access / 凭证发放` 的主路径链接已统一带 `from/flow/stage/tenant_id` 上下文参数，跨页回流不再丢失组织和阶段语义。
- [x] `Access` 页已支持承接企业页上下文：会按 `tenant_id` 回填组织，并在 `directory/policies/grants` 域切换时保留查询参数，减少跳转后二次定位。
- [x] `Access` 页已按 `stage` 补自动定位与草稿预填：`directory` 可预填用户组草稿，`policies` 可预填策略草稿，`issuance` 可定位授权域并给出发放承接提示。
- [x] `Wallet` 页已支持承接企业页上下文：会按 `tenant_id` 自动切换组织并刷新数据，发放场景保留 `scenario`，同时展示来源承接说明。
- [x] 企业页跨页上下文已补记录级对象提示（`group_member_* / target_*`）；`Access` 可承接成员线索并预填用户组草稿，`Wallet` 可承接发放对象并自动落位到单发目标、检索和投递收件人草稿。
- [x] 企业页“同步异常分类 / 同步处理闭环 / 来源状态总览”已补“停用对象清理”定向入口；`Access` 已支持按 `remediation_hint + group_member_status` 优先定位包含该成员的现有用户组，停用对象修复不再只停留在域级跳转。
- [x] `Access` 三域内部跳转已按企业流转自动更新 `stage`，并持续携带同一成员线索（`group_member_* / target_*`）；策略域会优先按该成员所在用户组套用草稿，发放域继续承接同对象提示。
- [x] `Access` 策略台账已补关键词筛选与承接预填（同对象线索可直接定位到策略列表）；`Wallet` 承接查询已补 `target_name` 兜底，发放台账定位不再依赖仅有 `target_id/email`。
- [x] 同对象已可直达处理动作：`Access` 在策略域命中后会优先直达“编辑策略”，`Wallet` 会按同对象自动预选外部投递目标，支持直接补发或重发失败通道。
- [x] 多对象批量处理动作已补首轮闭环：`Access` 可按当前策略筛选结果批量设为草稿/启用（单次最多 20 条），`Wallet` 可按同对象线索批量重发失败通道（单次最多 20 条）。
- [x] `Wallet` 已补“失败回执 -> 对象生命周期动作”联动：可按失败对象一键写入批量补发草稿、批量修复可恢复状态，并继续批量重发失败通道。
- [x] `Access` 策略批量复核完成后已补页内回流提示与“去凭证发放继续处理”入口，跨页衔接不再只靠顶部通用导航。
- [x] `Access` 批量启用策略后已补对象级发放预填（`target_ids`），`Wallet` 已支持承接并写入批量发放对象草稿，同时补“回企业页审批与异常”直达入口。
- [x] `Wallet` 已补对象级预填命中率面板（总数/已命中/未命中），并支持“一键仅保留未命中对象继续批量发放 / 恢复全部预填对象 / 回企业页审批与异常”闭环动作。
- [x] `Wallet` 已补未命中对象来源定位（目录在职 / 目录停用 / 仅在用户组 / 目录缺失）与分流动作，可直接“仅保留可补发对象”“回目录复核来源”“回企业页审批与异常”继续处理。
- [x] `Wallet -> Enterprise` 已补“同步异常一键联动”筛选线索：回流到审批与异常时可自动定位到目录异常落地页并预置同步状态/来源筛选，减少二次筛选操作。
- [x] `Wallet -> Enterprise` 已补“记录级线索关键词承接”：回流到审批与异常时可自动预填审批关键词（邮箱/对象 ID）与目录异常关键词（同步任务 ID），支持在企业页直接筛到对应台账记录。
- [x] 企业页目录异常台账已补“按本任务回流”动作：可从单条同步任务直接回流到目录/策略/发放，并携带 `sync_job_id/sync_source/sync_status/sync_category` 线索；`Access/Wallet` 已承接同步记录线索并补摘要提示。
- [x] 企业页目录异常台账已补“按本告警回流”动作：可从单条 worker 告警直接回流到目录/策略/发放，并携带 `worker_alert_* / worker_filter_hint / worker_query_hint` 线索；`Access/Wallet` 已承接 worker 告警线索并补摘要提示。
- [x] 企业页已补 `sync` 工作区的 worker 告警记录定位面板：支持承接 `sync_focus_hint + worker_*_hint`，在导入与同步页直接复核告警并回流到审批与异常、目录、策略与发放。
- [x] `Access / Wallet` 已补“处理完成后回导入与同步复核”入口，并携带 `worker_review_status_hint/worker_review_stage_hint`；企业页 `sync` 工作区已承接回流状态并补“二次复核 + 清除状态”动作。
- [x] 企业页 `sync` 工作区已补“连续主流程分段状态”卡：把“同步结果 -> 用户组使用 -> 权限下发”拆成分段状态并给出跨页承接动作；`Access` 已承接 `segment_hint/segment_status_hint` 并在来源摘要中补分段提示。
- [x] 企业页 `sync` 工作区已补“策略下发 -> 发放执行与回执”分段承接；`Wallet` 已承接 `segment_hint/segment_status_hint` 并在发放摘要中补分段提示。
- [x] `Wallet` 已补“回执失败分流 -> 重发/状态修复 -> 回企业页复核”同屏闭环状态卡，并新增 `segment_hint=receipt_recovery` 回企业复核承接入口。
- [x] 企业页“审批与异常”已补“回执失败复核结论回流”卡，支持按结论回发放页继续重发/状态修复/收口，并通过 `receipt_recovery_action_hint` 在 `Wallet` 侧落位处理动作。
- [x] 企业页 `sync` 工作区“连续主流程分段状态”已补统一收口提示与优先动作排序：先给唯一优先动作，再按分段显示优先级，减少并行跳转造成的断点。
- [x] 企业页 `sync` 工作区已对齐“同步来源状态总览 / 同步结果与下一步 / 连续主流程分段状态”动作口径：来源卡和下一步卡默认跟随分段优先动作，仅在来源阻塞时保留来源级优先处理。
- [x] 企业页 `sync` 工作区已对齐“同步处理闭环 / 目录到策略主流程连通检查”动作口径：未完成步骤默认跟随分段优先动作，并保留步骤级兜底入口。

进行中子项：

- [ ] `挂起待处理` 需要补外部 API 验证：核对真实 HRIS / SCIM 上游返回的 rejected 原因、停用语义和字段映射，用于确定企业页“同步结果与下一步”的失败分类和修复建议。

未完成子项：

- [ ] 本地闭环已基本收口，剩余 F2 关键缺口为外部 API 验证后的失败分类与修复建议固化。

### F3. MistyPass 发放中心重构（S0，已完成 100%）

进度条：`██████████ 100%`

已完成子项：

- [x] 顶级导航已从 `Wallet` 改为“凭证发放”。
- [x] 企业态隐藏跨租户聚合和风险排行，企业管理员默认只看当前组织发放状态。
- [x] 发放页已新增“员工发放 / 访客与临时证 / 批量补发 / 异常处理”的上层导览。
- [x] 发放页已接入模板列表、模板创建、模板启停。
- [x] 发放页已接入单个发放、批量发放、最近批量回执和已发放凭证状态表。
- [x] 已支持在发放页内直接暂停、恢复、吊销凭证，不再只看指标。
- [x] 发放页已拆成“发放操作 / 高级运行”双页签，首次进入默认聚焦主发放流程。
- [x] 已发放凭证已补搜索、状态/对象/模板筛选和批量暂停、恢复、吊销能力。
- [x] 已新增“员工移动凭证 / 员工实体卡联动 / 访客二维码 / 临时证”四类统一场景预设，可一键回填模板名称、`class_id`、`style_config` 和推荐失效时间。
- [x] 模板列表、单发/批发表单和已发放凭证台账已补场景标签，用户可直接区分员工移动凭证、实体卡联动、访客二维码和临时证。
- [x] 发放页已新增“交付与回执工作区”，把保存链接、交付方式、通道提示和同类台账入口集中在主流程内。
- [x] `Enterprise / Access -> 凭证发放` 已支持带场景参数直达员工发放或访客 / 临时证场景，不再要求用户手动切模板。
- [x] 已接入 `GET /wallet/passes/{passID}` 与 `GET /wallet/passes/{passID}/save-link` 的前端消费，支持按需刷新保存链接。
- [x] 发放页已支持二维码预览、SVG 下载和保存链接复制，二维码与保存链接不再只是静态文本。
- [x] 已新增 `GET /wallet/physical-card-tasks`、`POST /wallet/physical-card-tasks`、`PATCH /wallet/physical-card-tasks/{taskID}/status`，实体卡任务已有独立后端契约与状态持久化。
- [x] 发放页已新增“实体卡任务 / 最近实体卡进度”工作区，可直接创建制卡、补卡、挂失任务并在同页推进状态。
- [x] 实体卡任务已与数字凭证状态联动：挂失确认后可自动暂停员工凭证，补卡发放完成后可恢复凭证状态。
- [x] 已新增 `GET /wallet/deliveries`、`POST /wallet/deliveries/dispatch`、`POST /wallet/deliveries/{notificationID}/retry`，Email / WhatsApp 投递回执已有独立后端契约与重发入口。
- [x] 发放页已新增“发送外部投递 / 最近外部投递回执”工作区，可在同页选择凭证、填写接收方并查看每个通道的发送结果。
- [x] 外部投递回执已补失败原因、接收方明细和“重发失败通道”动作，不再只停留在保存链接复制层面。

后续增强项：

- [ ] 继续收口文案，让用户始终感知“发放 MistyPass”，而不是感知底层通道或队列实现。
- [ ] 继续把场景预设、模板、单发、批发、交付回执和已发放凭证之间的推荐动作串成更顺滑的连续流程。
- [ ] 高级运行视图仍在同页，后续可继续评估是否拆成高级页。

### F4. 网关、事件、告警的企业态体验（S1，已完成 100%）

进度条：`██████████ 100%`

已完成子项：

- [x] `事件` 页已支持企业态，不再强制显示租户筛选和租户列。
- [x] `网关` 页已按角色放开查看，并隐藏不适合企业用户的租户操作。
- [x] `网关库存`、`批量状态变更`、`注册` 等能力已按角色控制可写范围。
- [x] `告警` 页企业态已开始隐藏租户列与跨租户噪音信息。
- [x] `告警` 页已按 `building_ids` 过滤楼宇管理员范围，并补空状态提示。
- [x] `空间` 页已支持楼宇管理员维护楼层、区域和门点，但不开放新建楼宇。
- [x] `事件` 页已按 `building_ids` 过滤楼宇管理员范围，并补楼宇视角空状态提示。
- [x] `网关` 页已按 `building_ids` 过滤楼宇管理员范围，并将“注册网关”和“运维操作”权限拆开。
- [x] `Dashboard`、`Spaces`、`Events`、`Alarms`、`Gateways` 已补“未分配楼宇范围”专用提示，避免楼宇管理员误判为空数据或误看全量数据。
- [x] `Spaces` 页已补门点台账空状态与禁用态，`Events / Alarms / Gateways` 页已补范围说明，企业态行为更一致。
- [x] `Alarms` 页已补“关键词 + 状态 + 等级”统一筛选与重置动作，并对齐“空范围 / 筛选无结果 / 当前范围无数据”三段空状态口径。
- [x] `Events` 已补“事件类型筛选 + 关键词筛选 + 重置筛选”完整闭环，`Gateways` 已补“状态筛选 + 关键词筛选 + 重置网关筛选”，并对齐“空范围 / 筛选无结果 / 当前范围无数据”分层空状态口径。
- [x] 已完成多角色行为签字留档：`f4-role-surface` 关键场景（`building_admin` 可写边界、`tenant_admin` 筛选重置、`operator` 只读边界）在浏览器执行级回归通过，签字记录见 `docs/testing/artifacts/web-admin-role-boundary-signoff-20260416.md`。

进行中子项：

- [ ] 无。

未完成子项：

- [ ] 无。

### F5. 中文排版、按钮密度与仪表盘视觉收口（S1，已完成 100%）

进度条：`██████████ 100%`

已完成子项：

- [x] 已补第一轮 CJK 字体回退策略。
- [x] 已修正一部分按钮高度、标题 tracking 和信息密度问题。
- [x] 发放页、企业页等关键路径文案已开始按中文产品语境收口。
- [x] UI 原子组件密度基线已统一：`button/tabs/card/table` 的字号、行高、内边距与说明文字行距统一收口，中文场景下不再沿用英文紧凑密度。
- [x] 页面标题层级已统一：新增并落地 `mp-page-eyebrow/mp-page-title/mp-page-description` 到 `Dashboard/Enterprise/Access/Wallet/Gateways/Events/Alarms/Spaces/Tenants/Audit` 与 `TenantDetail`。
- [x] 长字段省略规范已落地：`TableCellText` 已接入 `Events/Alarms/Gateways/Wallet` 与 `Access` 三域台账（用户组/策略/授权），统一 `max-width + truncate + title` 行为。

进行中子项：

- [ ] 无。

未完成子项：

- [ ] 无。

### F6. Access 信息架构拆分（员工 / 用户组 / 策略 / 访客）（S1，已完成 100%）

进度条：`██████████ 100%`

已完成子项：

- [x] 已确认旧版 `Access` 页职责过多，不再继续在单页堆积功能。
- [x] 已确认后续目标结构为“员工与用户组 / 权限策略 / 临时与访客授权”。
- [x] 已新增 `/access/directory`、`/access/policies`、`/access/grants` 三个域路径骨架。
- [x] `Access` 页顶部已补三域卡片和推荐顺序，明确“目录 -> 策略 -> 临时授权”的工作流。
- [x] 用户组域已和企业目录导入入口形成更直接的联动。
- [x] `Access` 已补“目录准备度 / 策略准备度 / 发放与授权准备度”三张准备度卡片，用户能直接看到当前卡在哪一步。
- [x] 三个域已补域内说明卡和下一步动作入口，减少在企业页、权限页和发放页之间来回猜路径。
- [x] 用户组列表、策略列表、授权台账已补空状态文案，明确告诉用户是该去导入员工、补空间拓扑，还是去凭证发放。
- [x] `Access` 去“凭证发放”的入口已按场景分流，目录/策略域默认去员工发放，访客授权域默认去临时证场景。
- [x] `Access` 顶部“推荐顺序”已改成动态“当前建议动作”，会按真实准备度在导入员工、整理用户组、补空间拓扑、建策略和去发放之间切换。
- [x] 侧栏权限导航文案已从“用户组与授权”收口为“目录、策略与授权”，与三域结构保持一致。
- [x] 员工与用户组域已补“快速创建基础用户组”，可按岗位模板一键生成默认用户组，减少从已同步员工到可建策略之间的空档。
- [x] `Access` 已补成功反馈提示，创建用户组、策略、临时授权和快速建组后不再只有静默列表变化。
- [x] 权限策略域已补“快速生成策略草稿”，可直接从已有用户组回填建议范围、时间计划和成员数，减少从用户组到首批策略之间的断层。
- [x] 策略草稿会明确提示建议范围与需要复核的拓扑缺口，用户不再只能从空白表单开始。
- [x] `Access` 三域的顶部说明卡与准备度卡已开始抽成共享组件，避免继续把同类结构堆在单页里。
- [x] 临时与访客授权域已补授权台账筛选，可按日期、下发方式、对象类型和有效状态快速复核短期授权。
- [x] 临时与访客授权域已补“快速授权场景”，可直接套用来访宾客、施工维护、面试来访和临时员工补录的建议范围、方式和失效时间。
- [x] 短期授权不再只能从空白表单开始，`grants` 域已经开始具备面向任务的场景入口。
- [x] `grants` 域的“快速授权场景”和“台账筛选条”已开始抽成独立组件，进一步降低单页堆叠度。
- [x] 授权台账已补“24 小时内到期”状态线，支持在筛选和台账中直接识别即将失效的短期授权。
- [x] `grants` 域顶部统计卡已可直接触发状态或对象类型筛选，前台和安保可更快切到“当前有效 / 即将到期 / 已到期 / 访客授权”视图。
- [x] `grants` 域的授权统计卡和授权台账表格已继续抽成独立组件，页面主文件开始从授权工作台细节里解耦。
- [x] `grants` 域的创建授权表单和授权详情弹窗已继续抽成独立组件，页面主文件进一步收口到数据准备和动作绑定。
- [x] `grants` 域已进一步抽成独立 section 组件，`Access` 主文件开始从整段授权工作台里解耦，而不只是拆散场景卡、表单、筛选和台账。
- [x] `policies` 域的“快速生成策略草稿”面板和“策略列表”台账已继续抽成独立组件，页面主文件开始从策略推荐和策略台账细节里解耦。
- [x] `policies` 域的新建 / 编辑策略表单已继续抽成独立组件，页面主文件进一步收口到策略数据准备和动作绑定。
- [x] `policies` 域已进一步抽成独立 section 组件，`Access` 主文件开始从整段策略工作台里解耦，而不只是拆散推荐、表单和台账。
- [x] `directory` 域的“快速创建基础用户组”面板、用户组表单和用户组台账已继续抽成独立组件，页面主文件开始从目录工作台细节里解耦。
- [x] `directory` 域的“岗位自动分组与权限模板”也已继续抽成独立组件，目录域主文件基本完成从说明与表格细节里的解耦。
- [x] `directory` 域已进一步抽成独立 section 组件，`Access` 主文件开始从整段目录工作台里解耦，而不只是拆散单个表单和表格。
- [x] `Access` 三域已补显式子路径（`/access/directory`、`/access/policies`、`/access/grants`），并保留 `/:section` 兼容入口；页面已改为按 `pathname` 解析当前域，降低对参数路由的耦合。
- [x] `grants` 域“去凭证发放”已改为上下文承接链接：保留 `enterprise` 回流参数、组织线索与对象 hint，并新增“按当前筛选继续发放”动作，短期授权到发放衔接不再只靠固定 URL。
- [x] `Access` 顶部“指标卡 / 三域入口卡 / 三张准备度卡”已继续抽成独立共享组件（`AccessDomainMetricsCards`、`AccessSectionOverviewCards`、`AccessReadinessOverviewCards`），主页面进一步收口到域路由与动作编排。
- [x] `Access` 顶部“标题与租户选择区”已抽成独立组件 `AccessPageHeader`，`access-page` 主文件进一步减少头部 UI 细节耦合，继续向“域编排页”收口。
- [x] `Access` 三域 `Tabs + section` 主体已抽成 `AccessSectionsTabs`，`access-page` 进一步收口为“状态准备 + 动作绑定 + 组件编排”。
- [x] `Access` 操作反馈提示（error/summary）已抽成 `AccessOperationFeedback`，页面主文件不再内联重复反馈块。
- [x] `Access` 主页已把三域 props 绑定预组装为 `directory/policies/grants` 常量，并拆出 `resetGrantFilters/applyGrantStarterByID`，减少 `JSX` 内联业务绑定噪声。
- [x] `AccessDomainBanner` 按钮组已抽成 `AccessDomainBannerActions`，主页面继续收口到“推荐动作编排”而非按钮细节拼装。
- [x] `Access` 页通用格式化/范围校验/路径解析/策略模板推导函数已抽成 `access-page-utils.ts`，主页面进一步收口到状态与业务流程编排。
- [x] `Access` enterprise 承接参数解析、stage 链接拼装、worker review 回流链接与 wallet 上下文注入逻辑已抽成 `access-enterprise-flow-utils.ts`，主页面减少 URL 参数拼装重复。
- [x] `Access` 推荐动作与三域总览文案已抽成 `access-page-recommendation-utils.ts`，`access-page` 主文件进一步收口为状态装配与域编排。
- [x] `Access` 策略草稿与授权场景生成逻辑已抽成 `access-starter-utils.ts`，主页面进一步去除大段 starter 推导细节。
- [x] `Access` 授权台账的租户/日期筛选、状态计数与 pass type 聚合已抽成 `access-grant-ledger-utils.ts`，主页面减少重复过滤链条。
- [x] `Access` 目录/策略台账视图模型（成员展示、策略列表、策略搜索建议、空态判定）已抽成 `access-ledger-view-model-utils.ts`，主页面进一步减少列表拼装和文案分支。
- [x] `Access` enterprise 分段预置中的同步/worker/成员线索标签与策略命中推导已收口到工具层（`access-enterprise-flow-utils.ts` + `access-ledger-view-model-utils.ts`），`access-page` 进一步减重到 `1739` 行。
- [x] `Access` enterprise 分段预置里的 summary 尾缀（同步记录/worker 告警）已统一收口到 `buildEnterpriseSummaryTail`，`access-page` 进一步减重到 `1736` 行。
- [x] `Access` enterprise 分段预置里的 name/group hint 精确匹配、stageKey 生成与 summary 前缀拼装已统一收口到工具层（`findByNameHint/findByGroupNameHint/buildEnterpriseStagePresetKey/buildEnterpriseFlowSummary`），`access-page` 继续去除重复分支与重复字符串模板。
- [x] `Access` enterprise 分段预置里的 stage 路由映射与 sync/worker 草稿命名已统一收口到工具层（`resolveEnterpriseAccessStageRoute/buildEnterpriseSyncGroupDraft/buildEnterpriseWorkerGroupDraft/buildEnterpriseWorkerPolicyDraftName`），`access-page` 继续减少重复导航与命名模板分支。
- [x] `Access` enterprise 分段预置里的成员线索回填动作与搜索承接摘要已继续收口（`applyHintedGroupMemberQuery` + `buildEnterpriseFlowSummary` 复用），`access-page` 进一步减重到 `1723` 行。
- [x] `Access` 三域路由已收口为独立页面壳：`/access/directory`、`/access/policies`、`/access/grants` 分别挂载 `AccessDirectoryPage/AccessPoliciesPage/AccessGrantsPage`，`/access/:section` 降级为兼容重定向页 `AccessLegacySectionRedirectPage`；三域页面与兼容入口解耦完成。

进行中子项：

- [ ] 无。

未完成子项：

- [ ] 无。

### F7. 构建回归、页面验证与性能治理（S2，已完成 100%）

进度条：`██████████ 100%`

已完成子项：

- [x] 本轮角色态、企业页、网关/事件页和发放页改造后，`web-admin` 构建通过。
- [x] 已形成前端高优先级改造路线图，后续可以按编号持续回填。
- [x] `Access` 三域骨架改造后再次完成前端构建验证。
- [x] `building_admin` 范围过滤改造后再次完成前端构建验证。
- [x] 已将页面路由改为懒加载，按页拆出独立 chunk，主 bundle 已显著下降。
- [x] 发放页补齐双页签、筛选和批量操作后再次完成前端构建验证。
- [x] 企业页与 `Access` 主链路联动改造后再次完成前端构建验证。
- [x] `Access` 三域准备度卡、空状态和跨页 CTA 改造后再次完成前端构建验证。
- [x] `building_admin` 空范围边界修复与多页面提示收口后再次完成前端构建验证。
- [x] 发放页补齐统一场景预设、场景标签和操作联动后再次完成前端构建验证。
- [x] 发放页补齐场景直达、交付与回执工作区和复制保存链接操作后再次完成前端构建验证。
- [x] 发放页补齐二维码预览、SVG 下载与保存链接刷新后再次完成前端构建验证。
- [x] 实体卡任务后端接口与状态持久化补齐后，`api` 下 `go test ./...` 已通过。
- [x] 发放页补齐实体卡任务创建、状态推进与最近进度区后再次完成前端构建验证。
- [x] 发放页补齐外部投递派发、渠道回执明细与失败通道重发后，`api` 下 `go test ./...` 与 `web-admin` 下 `npm run build` 均已通过。
- [x] 企业页补齐主路径进度看板、快捷入口联动与跨模块计数后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 顶部动态下一步动作与导航文案收口后，`web-admin` 下 `npm run build` 已通过。
- [x] 企业页补齐关键提醒卡片、Access 补齐快速建组与成功反馈后，`web-admin` 下 `npm run build` 已通过。
- [x] 企业页补齐“企业登录落地工作区 / 阻塞项处理台”以及 Access 策略草稿推荐后，`web-admin` 下 `npm run build` 已通过。
- [x] 企业页补齐“同步结果与下一步”工作区、可操作同步来源卡片和提交后的后续建议后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 抽出共享组件并补齐授权台账筛选后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 补齐“快速授权场景”后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将授权场景卡和台账筛选条继续抽成独立组件后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 授权台账补齐“24 小时内到期”状态筛选与状态列后，`web-admin` 下 `npm run build` 已通过。
- [x] 企业页补齐“同步异常分类”、Access 授权统计卡支持直接触发筛选后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将授权统计卡和授权台账表格继续抽成独立组件后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将创建授权表单和授权详情弹窗继续抽成独立组件后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将策略草稿面板和策略台账表格继续抽成独立组件后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将新建 / 编辑策略表单继续抽成独立组件后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将目录域的快速建组面板、用户组表单和用户组台账继续抽成独立组件后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将岗位模板表格继续抽成独立组件、企业页补齐同步处理闭环后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将目录域继续抽成独立 section 组件后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将策略域继续抽成独立 section 组件后，`web-admin` 下 `npm run build` 已通过。
- [x] Access 将授权域继续抽成独立 section 组件后，`web-admin` 下 `npm run build` 已通过。
- [x] 企业页“审批与异常”补齐“处理完成后回流动作”后，`web-admin` 下 `npm run build` 已通过。
- [x] 企业页“企业登录”补齐“完成后的下一步”后，`web-admin` 下 `npm run build` 已通过。
- [x] 企业页将 `employees / sync / idp / alerts` 四个工作区抽成组件后，`web-admin` 下 `npm run build` 已通过（`enterprise-page` chunk `46.07 kB`，`access-page` chunk `59.63 kB`，主 bundle `328.91 kB`）。
- [x] 新增前端路由 smoke 脚本 `docs/testing/curl-web-admin-smoke.zsh`（默认 `build + 路由契约检查`，可选开启 `HTTP` 路由可达检查）；本轮已完成 `web-admin` 构建与脚本本地验证。
- [x] 前端路由 smoke 脚本已补“鉴权守卫 + 主流程 CTA”契约检查（`route + guard + mainflow` 三类校验），并完成本地执行验证。
- [x] 企业页“同步来源状态总览”补来源级结果明细、企业页“目录到策略主流程连通检查”补五步直达动作后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `57.77 kB`，`access-page` chunk `59.63 kB`，主 bundle `328.91 kB`）。
- [x] 企业页“审批与异常”补“落地与回流动作”三卡后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `63.25 kB`，`access-page` chunk `59.63 kB`，主 bundle `328.91 kB`）。
- [x] 企业页“审批与异常”补细粒度筛选与逐条处理动作后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `69.19 kB`，`access-page` chunk `59.63 kB`，主 bundle `328.91 kB`）。
- [x] 企业页“审批与异常”补“审批积压闭环清单 + 目录异常闭环清单”后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `73.02 kB`，`access-page` chunk `59.63 kB`，主 bundle `328.91 kB`）。
- [x] 企业页“审批与异常”补审批 review / 外部回写更新真实处理动作后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `74.98 kB`，`access-page` chunk `59.63 kB`，主 bundle `329.30 kB`）。
- [x] 企业页“审批与异常”补审批批量处理动作（批量批准/拒绝、批量标记回写）后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `77.27 kB`，`access-page` chunk `59.63 kB`，主 bundle `329.30 kB`）。
- [x] 企业页“审批与异常”补“总览 / 审批积压 / 目录异常”独立落地页切换，并将审批批量动作沉淀到闭环清单后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `80.64 kB`，`access-page` chunk `59.63 kB`，主 bundle `329.30 kB`）。
- [x] 企业页主路径已补跨页上下文参数承接（`from/flow/stage/tenant_id`）并在 `Access` 页落地组织回填与查询参数保留后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `80.21 kB`，`access-page` chunk `60.58 kB`，主 bundle `329.30 kB`）。
- [x] `Access / Wallet` 已补 `stage` 自动定位与预填（目录/策略草稿、发放承接租户回填）后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `80.21 kB`，`access-page` chunk `61.93 kB`，`wallet-page` chunk `109.97 kB`，主 bundle `329.30 kB`）。
- [x] 企业页到 `Access / Wallet` 已补记录级对象承接（成员线索 + 发放对象落位）后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `80.98 kB`，`access-page` chunk `64.44 kB`，`wallet-page` chunk `110.67 kB`，主 bundle `329.30 kB`）。
- [x] 企业页已补“停用对象清理”定向入口并在 `Access` 承接到成员级定位后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `64.94 kB`，`wallet-page` chunk `110.67 kB`，主 bundle `329.30 kB`）。
- [x] `Access` 三域跳转已补 `stage` 连续切换与同对象线索承接（目录 -> 策略 -> 发放），策略域优先按成员所在组套草稿后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `66.72 kB`，`wallet-page` chunk `110.67 kB`，主 bundle `329.30 kB`）。
- [x] `Access` 策略台账补关键词筛选与承接预填、`Wallet` 承接查询补 `target_name` 兜底后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `68.02 kB`，`wallet-page` chunk `110.69 kB`，主 bundle `329.30 kB`）。
- [x] `Access` 已补同对象命中后直达策略编辑，`Wallet` 已补同对象直达外部投递目标（补发/重发入口就绪）后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `68.51 kB`，`wallet-page` chunk `111.54 kB`，主 bundle `329.30 kB`）。
- [x] `Access` 已补按筛选结果批量复核策略状态（草稿/启用），`Wallet` 已补按对象线索批量重发失败通道后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `69.91 kB`，`wallet-page` chunk `113.12 kB`，主 bundle `329.30 kB`）。
- [x] `Wallet` 已补“失败回执 -> 批量状态修复 -> 批量补发草稿 -> 批量重发”生命周期联动动作，并修复凭证台账批量状态按钮误禁用后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `69.91 kB`，`wallet-page` chunk `115.41 kB`，主 bundle `329.30 kB`）。
- [x] `Access` 已补“策略批量复核完成后回流到凭证发放”的页内提示与直达入口后，`web-admin` 下 `npm run build` 与 `docs/testing/curl-web-admin-smoke.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `70.54 kB`，`wallet-page` chunk `115.41 kB`，主 bundle `329.30 kB`）。
- [x] `Access` 已补批量启用策略后的对象级发放预填（`target_ids`），`Wallet` 已补承接批量对象草稿与“回企业页审批与异常”入口后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `71.09 kB`，`wallet-page` chunk `115.97 kB`，主 bundle `329.30 kB`）。
- [x] `Wallet` 已补对象级预填命中率面板与“仅保留未命中对象/恢复全部预填对象”动作后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `71.09 kB`，`wallet-page` chunk `118.02 kB`，主 bundle `329.30 kB`）。
- [x] `Wallet` 已补未命中对象来源定位与分流动作（可补发/回目录/回审批与异常）后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `81.70 kB`，`access-page` chunk `71.09 kB`，`wallet-page` chunk `121.98 kB`，主 bundle `329.30 kB`）。
- [x] `Wallet` 回企业页已补“目录异常落地页 + 同步状态/来源”一键筛选联动，企业页审批与异常可自动承接筛选线索后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `82.64 kB`，`access-page` chunk `71.09 kB`，`wallet-page` chunk `122.34 kB`，主 bundle `329.30 kB`）。
- [x] 企业页“审批与异常”已补审批关键词/目录关键词筛选并承接 `Wallet` 回流 hint（`approval_query_hint` / `sync_query_hint`），可直接按对象线索和同步任务 ID 定位台账后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `84.40 kB`，`access-page` chunk `71.09 kB`，`wallet-page` chunk `122.97 kB`，主 bundle `329.30 kB`）。
- [x] 企业页目录异常台账已补“按本任务去目录 / 策略 / 发放”记录级回流动作，并在 `Access/Wallet` 承接同步记录线索摘要后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `86.12 kB`，`access-page` chunk `72.11 kB`，`wallet-page` chunk `123.60 kB`，主 bundle `329.30 kB`）。
- [x] 企业页目录异常台账已补“按本告警去目录 / 策略 / 发放”记录级回流动作，并在 `Access/Wallet` 承接 worker 告警线索摘要后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `88.05 kB`，`access-page` chunk `74.87 kB`，`wallet-page` chunk `125.16 kB`，主 bundle `329.30 kB`）。
- [x] 企业页已补“按本告警去导入与同步”记录级入口，`sync` 工作区已补 worker 告警定位与回流动作，并承接 `worker_filter_hint/worker_query_hint` 后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `94.20 kB`，`access-page` chunk `74.87 kB`，`wallet-page` chunk `125.16 kB`，主 bundle `329.30 kB`）。
- [x] `Access / Wallet` 已补“处理完成后回导入与同步复核”回流入口，企业页 `sync` 工作区已承接 `worker_review_status_hint/worker_review_stage_hint` 并补二次复核动作后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `95.90 kB`，`access-page` chunk `76.78 kB`，`wallet-page` chunk `126.86 kB`，主 bundle `329.30 kB`）。
- [x] 企业页 `sync` 工作区已补“连续主流程分段状态”卡并下钻到 `Access` 分段承接，`Access` 已承接 `segment_hint/segment_status_hint` 分段提示后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `98.68 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `126.86 kB`，主 bundle `329.30 kB`）。
- [x] 企业页 `sync` 工作区已补“策略下发 -> 发放执行与回执”分段承接并下钻到 `Wallet`，`Wallet` 已承接 `segment_hint/segment_status_hint` 分段提示后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `99.36 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `127.54 kB`，主 bundle `329.30 kB`）。
- [x] `Wallet` 已补“回执失败分流 -> 重发/状态修复 -> 回企业页复核”同屏闭环状态卡并接入 `segment_hint=receipt_recovery` 复核入口后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `99.36 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `130.80 kB`，主 bundle `329.30 kB`）。
- [x] 企业页“审批与异常”已补“回执失败复核结论回流”动作，并在 `Wallet` 承接 `receipt_recovery_action_hint` 后完成“继续重发/继续状态修复/复核已收口”定位提示，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `102.08 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `132.43 kB`，主 bundle `329.30 kB`）。
- [x] 企业页 `sync` 工作区“连续主流程分段状态”已补统一收口提示与优先动作排序后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `103.70 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `132.43 kB`，主 bundle `329.30 kB`）。
- [x] 企业页 `sync` 工作区已对齐“来源总览 + 下一步 + 分段状态”动作口径并默认跟随分段优先动作后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `103.68 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `132.43 kB`，主 bundle `329.30 kB`）。
- [x] 企业页 `sync` 工作区已对齐“同步处理闭环 + 目录到策略主流程连通检查”动作口径并默认跟随分段优先动作后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `103.92 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `132.43 kB`，主 bundle `329.30 kB`）。
- [x] F1 已补“运营叙事”残留清理（`login/events/wallet/tenants` + 导航总览 + 角色标签）后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`dashboard-page` chunk `10.05 kB`，`alarms-page` chunk `10.64 kB`，`gateways-page` chunk `37.33 kB`，`enterprise-page` chunk `103.92 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `132.43 kB`，主 bundle `329.31 kB`）。
- [x] 前端 smoke 脚本已补“登录后关键交互契约”检查（审批批量动作、回执恢复批量动作、告警通知策略、同步处理后回流动作与库存导出提示），并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 本地验证（`enterprise-page` chunk `103.92 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `132.43 kB`，主 bundle `329.31 kB`）。
- [x] `Wallet` 只读边界提示已做二次收口（页面级只读说明 + 模板/投递/实体卡/台账/告警订阅五处提示统一），`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `103.92 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.31 kB`）。
- [x] 新增 `docs/testing/curl-web-admin-role-boundary-smoke.zsh` 后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`enterprise-page` chunk `103.92 kB`，`access-page` chunk `77.63 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.31 kB`）。
- [x] `Access` 显式子路径改造后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均已通过（`curl-web-admin-smoke` 的 `route_markers=13`，`enterprise-page` chunk `103.92 kB`，`access-page` chunk `77.83 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.66 kB`）。
- [x] 新增 `docs/testing/curl-web-admin-browser-e2e.zsh` 浏览器执行级基线（Playwright：`access` 路由切换 + `enterprise` 企业登录完成后回流动作），并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh` 本地验证（`browser e2e` `4/4` 通过；`enterprise-page` chunk `104.05 kB`，`access-page` chunk `77.83 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.66 kB`）。
- [x] 浏览器执行级 e2e 已补 `enterprise` 失败态回流上下文承接：覆盖“回执失败复核结论回发放（`receipt_recovery`）”与“发放处理后回导入与同步复核（`worker_review`）”两条链路，并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 本地验证（`browser e2e` `6/6` 通过；`enterprise-page` chunk `104.26 kB`，`access-page` chunk `77.83 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.66 kB`）。
- [x] 浏览器执行级 e2e 已补“动作执行后状态变化”断言：覆盖企业页 `alerts` 批量批准 pending 后的成功反馈与状态切换（`pending -> approved`），并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh` 本地验证（`browser e2e` `7/7` 通过；`enterprise-page` chunk `104.26 kB`，`access-page` chunk `77.83 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.66 kB`）。
- [x] 浏览器执行级 e2e 已补发放侧“动作执行后状态变化”断言：覆盖 `Wallet` “批量重发失败通道”执行后的成功反馈与重发计数变化（`1 -> 0`），并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 本地验证（`browser e2e` `8/8` 通过；`enterprise-page` chunk `104.26 kB`，`access-page` chunk `77.83 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.66 kB`）。
- [x] 浏览器执行级 e2e 已补发放侧“动作执行后状态变化”断言：覆盖 `Wallet` “批量状态修复”执行后的成功反馈与修复计数变化（`1 -> 0`），并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 本地验证（`browser e2e` `9/9` 通过；`enterprise-page` chunk `104.26 kB`，`access-page` chunk `77.83 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.66 kB`）。
- [x] 浏览器执行级 e2e 已补发放侧“动作执行后状态变化”断言：覆盖 `Wallet` “写入批量补发草稿”执行后的成功反馈与草稿目标预填（`target_ids`），并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh` 本地验证（`browser e2e` `10/10` 通过；`enterprise-page` chunk `104.26 kB`，`access-page` chunk `77.83 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.66 kB`）。
- [x] 浏览器执行级 e2e 已补发放侧“动作执行后状态变化”断言：覆盖 `Wallet` “仅保留未命中对象 -> 恢复全部预填对象”执行后的草稿目标变化，并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh` 本地验证（`browser e2e` `11/11` 通过；`enterprise-page` chunk `104.26 kB`，`access-page` chunk `77.83 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.66 kB`）。
- [x] 浏览器执行级 e2e 已补发放侧“批量补发提交”动作执行后状态变化断言：覆盖成功态（提交摘要 + 最近批量回执）与失败态（草稿保留 + 无回执 + 无成功摘要），并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh` 本地验证（`browser e2e` `13/13` 通过；`enterprise-page` chunk `104.26 kB`，`access-page` chunk `77.83 kB`，`wallet-page` chunk `132.86 kB`，主 bundle `329.66 kB`）。
- [x] 浏览器执行级 e2e 已扩展并全量通过：`web-admin` 下 `docs/testing/curl-web-admin-browser-e2e.zsh` 本轮 `104/104` 通过；同时 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 全部通过（`enterprise-page` chunk `104.33 kB`，`access-page` chunk `77.57 kB`，`wallet-page` chunk `105.33 kB`，`index` chunk `43.38 kB`，`vendor-react` chunk `165.42 kB`，`vendor-ui` chunk `115.72 kB`）。
- [x] 本轮继续补 `F4 / F6` 浏览器执行级回归：新增 `Alarms` 筛选收口与 `Access grants -> Wallet` 上下文承接断言后，`web-admin` 下 `docs/testing/curl-web-admin-browser-e2e.zsh` 本轮 `106/106` 通过；同时 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均通过（`enterprise-page` chunk `104.34 kB`，`access-page` chunk `79.04 kB`，`alarms-page` chunk `12.52 kB`，`wallet-page` chunk `105.33 kB`，主 bundle `329.66 kB`）。
- [x] 本轮继续补 `F4` 浏览器执行级回归：新增 `Events` 类型筛选重置、`Gateways` 状态筛选重置与分层空状态断言后，`web-admin` 下 `docs/testing/curl-web-admin-browser-e2e.zsh` 本轮 `108/108` 通过；同时 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/check-doc-capability-markers.zsh` 均通过（`enterprise-page` chunk `104.34 kB`，`access-page` chunk `79.04 kB`，`events-page` chunk `8.13 kB`，`gateways-page` chunk `36.59 kB`，`wallet-page` chunk `105.33 kB`，主 bundle `329.66 kB`）。
- [x] 本轮完成 `F1/F4` 角色边界签字留档：`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）全部通过，签字记录见 `docs/testing/artifacts/web-admin-role-boundary-signoff-20260416.md`。
- [x] `manualChunks` 评估已收口：当前 `vendor-router / vendor-react / vendor-ui / vendor-icons / vendor-qrcode` 与按页懒加载组合稳定，构建无新增大 chunk 警告，暂不继续做更细粒度拆包。
- [x] 本轮继续补 `F6` 解耦收口：`Access` 准备度三卡抽成 `AccessReadinessOverviewCards` 后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）全部通过（`enterprise-page` chunk `104.34 kB`，`access-page` chunk `80.17 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 解耦收口：`Access` 顶部标题与租户选择区抽成 `AccessPageHeader` 后，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh` 复跑（`108/108`）全部通过（`enterprise-page` chunk `104.34 kB`，`access-page` chunk `80.33 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：新增 `AccessSectionsTabs` 与 `AccessOperationFeedback` 后，先完成 `build + smoke + role-boundary` 快速守护，再做一次分支级 `browser e2e`；`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）全部通过（`enterprise-page` chunk `104.34 kB`，`access-page` chunk `80.64 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 补 `props` 预组装与 `AccessDomainBannerActions` 组件化后，继续按“先 `build + smoke + role-boundary` 快速守护，后一次性分支级 e2e”执行；`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）全部通过（`enterprise-page` chunk `104.34 kB`，`access-page` chunk `81.01 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 通用函数抽离到 `access-page-utils.ts` 后，继续按“先 `build + smoke + role-boundary`，后一次性分支级 e2e”执行；`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）全部通过（`enterprise-page` chunk `104.34 kB`，`access-page` chunk `81.01 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 新增 `access-enterprise-flow-utils.ts`（enterprise 承接参数/回流链接/Wallet 上下文统一）与 `access-page-recommendation-utils.ts`（推荐动作/三域总览文案统一），并同步修正 `docs/testing/curl-web-admin-smoke.zsh` 的主流程标记扫描路径；期间定位并修复 `enterprise-alerts` 回执失败回流链接在加载态下 `segment_status_hint` 抖动（改为加载完成后再展示回流动作）。本轮 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh` 与 `docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）全部通过（`enterprise-page` chunk `104.50 kB`，`access-page` chunk `81.69 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 新增 `access-starter-utils.ts`（策略草稿/授权场景推导统一）与 `access-grant-ledger-utils.ts`（授权台账筛选/状态计数/对象类型聚合统一）后，继续按“先 `build + smoke + role-boundary`，后一次性分支级 e2e”执行；`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）及 `docs/testing/check-doc-capability-markers.zsh` 全部通过（`enterprise-page` chunk `104.50 kB`，`access-page` chunk `82.46 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 新增 `access-ledger-view-model-utils.ts`（目录/策略台账行映射、搜索过滤、空态判定与建议查询统一）后，继续按“先 `build + smoke + role-boundary`，后一次性分支级 e2e”执行；`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）及 `docs/testing/check-doc-capability-markers.zsh` 全部通过（`enterprise-page` chunk `104.50 kB`，`access-page` chunk `83.08 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 将 enterprise 分段预置中的重复标签与策略命中逻辑下沉到 `access-enterprise-flow-utils.ts` / `access-ledger-view-model-utils.ts`，主页面移除重复分支并继续按“先 `build + smoke + role-boundary`，后一次性分支级 e2e”执行；`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）及 `docs/testing/check-doc-capability-markers.zsh` 全部通过（`enterprise-page` chunk `104.50 kB`，`access-page` chunk `83.12 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 将 enterprise summary 尾缀拼装统一下沉到 `access-enterprise-flow-utils.ts`（`buildEnterpriseSummaryTail`），并在 `access-page` 复用 `enterpriseSummaryTail/enterpriseSyncCompactTail/enterpriseWorkerCompactTail`，继续按“先 `build + smoke + role-boundary`，后一次性分支级 e2e”执行；`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）及 `docs/testing/check-doc-capability-markers.zsh` 全部通过（`enterprise-page` chunk `104.50 kB`，`access-page` chunk `82.76 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 将 enterprise 分段预置中的 `name/group` hint 匹配、`stageKey` 生成与 summary 前缀拼装统一下沉到 `access-enterprise-flow-utils.ts`，并在 `access-page` 引入 `setEnterpriseSummary` 复用消息模板，继续按“先 `build + smoke + role-boundary`，后一次性分支级 e2e”执行；`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）及 `docs/testing/check-doc-capability-markers.zsh` 全部通过（`enterprise-page` chunk `104.50 kB`，`access-page` chunk `82.49 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 将 enterprise 分段预置中的 stage 路由映射与 sync/worker 草稿命名统一下沉到 `access-enterprise-flow-utils.ts`，并在 `access-page` 复用 `resolveEnterpriseAccessStageRoute + buildEnterpriseSyncGroupDraft + buildEnterpriseWorkerGroupDraft + buildEnterpriseWorkerPolicyDraftName` 去除重复导航与命名模板；主文件进一步降到 `1728` 行。继续按“先 `build + smoke + role-boundary`，后一次性分支级 e2e”执行，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）及 `docs/testing/check-doc-capability-markers.zsh` 全部通过（`enterprise-page` chunk `104.50 kB`，`access-page` chunk `83.03 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮继续补 `F6` 分支级解耦收口：`Access` 进一步去重 enterprise 成员线索回填动作与摘要前缀拼装（`applyHintedGroupMemberQuery` + `buildEnterpriseFlowSummary` 复用），`access-page` 主文件进一步降到 `1723` 行。继续按“先 `build + smoke + role-boundary`，后一次性分支级 e2e”执行，`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）及 `docs/testing/check-doc-capability-markers.zsh` 全部通过（`enterprise-page` chunk `104.50 kB`，`access-page` chunk `83.00 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮一次性收口 `F6`：`Access` 三域改为独立页面壳（`AccessDirectoryPage/AccessPoliciesPage/AccessGrantsPage`），并将 `/:section` 入口降级为兼容重定向页（`AccessLegacySectionRedirectPage`）；`web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）与 `docs/testing/check-doc-capability-markers.zsh` 全部通过（`enterprise-page` chunk `104.50 kB`，`access-page` chunk `78.35 kB`，`wallet-page` chunk `105.33 kB`）。
- [x] 本轮一次性收口 `F5`：完成中文排版/密度规范统一（`mp-page-eyebrow/title/description` + `mp-kpi-note` + `button/tabs/card/table` 基线 + `TableCellText` 长字段省略），并完成 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（`108/108`）与 `docs/testing/check-doc-capability-markers.zsh` 全通过（`enterprise-page` chunk `103.96 kB`，`access-page` chunk `78.47 kB`，`wallet-page` chunk `104.53 kB`）。
- [x] 本轮一次性收口 `F7` 守护：按“分支级集中验证”执行 `web-admin` 下 `npm run build`、`docs/testing/curl-web-admin-smoke.zsh`、`docs/testing/curl-web-admin-role-boundary-smoke.zsh`、`docs/testing/curl-web-admin-browser-e2e.zsh`（首轮 `107/108`，重跑 `108/108`）与 `docs/testing/check-doc-capability-markers.zsh`，全部通过（`enterprise-page` chunk `103.96 kB`，`access-page` chunk `78.47 kB`，`wallet-page` chunk `104.53 kB`）。

进行中子项：

- [ ] 无。

未完成子项：

- [ ] 无。

## 3. 当前轮次继续推进顺序

1. `F1` 角色态与租户去暴露：
   - 本轮已完成角色边界签字与空状态核验，当前转入持续守护。
2. `F4` 网关、事件、告警企业态体验：
   - 本轮已完成多角色行为签字与筛选/空状态收口，当前转入持续守护。
3. `F6` Access 结构拆分：
   - 本轮已完成三域独立页面壳与 `/:section` 兼容重定向收口，转入持续守护。
4. `F5` 中文排版与视觉密度收口：
   - 本轮已完成中文排版、密度与长字段省略规范收口，转入持续守护。
5. `F2` 企业目录与 Access 联动：
   - 本地闭环收口已完成，下一步等待外部 `HRIS / SCIM` API 验证恢复后固化失败分类与修复建议。
6. `F3` MistyPass 发放中心：
   - 主路径已完成，转入增强项评估：高级运行拆页、文案继续收口和更细的渠道运营视图。
7. `F7` 构建回归与页面验证：
   - 本轮分支级集中守护通过：`build + smoke + role-boundary + browser e2e(108/108) + doc-marker`；后续仅保留增量改动的持续守护，不再作为当前阻塞项。

验证后剩余待推进（高优先级且无外部依赖）：

- 前端：无新增高优先级阻塞项（`F1/F4/F5/F6` 已完成并转入 `F7` 增量守护）。
- 后端并行项见 `docs/development-status-roadmap.md` 的 `5.1`：`R6-WG Branch-A/B/C` 与 `R1` 合同回归守护可持续推进，均不依赖外部 API。

## 4. 挂起待处理（需外部 API 验证）

- `F2`：核对真实 `HRIS / SCIM` 上游返回的 `rejected` 原因、停用语义和字段映射，用于确定企业页“同步结果与下一步”的失败分类和修复建议。
- `Wallet/Access` 增强项：确认邮件 / 二维码投递回执字段能否反向映射到授权台账，用于决定后续是否在授权域直显投递结果。
