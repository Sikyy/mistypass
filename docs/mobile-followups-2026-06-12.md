# 移动端待办排期（2026-06-12）

> 来源：2026-06-10 全项目审查（docs/CODE-REVIEW-2026-06-10.md）的移动端 findings + 06-12 实测基线复核。
> 两个仓库独立于 server repo：
> - Android：`/Users/siky/code/android-MistyisletPass`，分支 `codex/android-ios-fidelity-2026-05-27`
> - iOS：`/Users/siky/code/ios-MistyisletPass`，分支 `codex/ios-staging-scheme-2026-05-25`

## 0. 已完成（任务卡核销）

| 审查项 | 状态 | 证据 |
|---|---|---|
| iOS 推送 aps-environment（P1-3） | ✅ 已修 | iOS `172608b fix: enable APNs push end-to-end`；entitlements 含 `aps-environment`；另有登录后注册时序修复会话 |
| gateway-agent 深审（任务卡） | ✅ 跑过，至少 1 个 P0 已修 | 主仓库 `82d4500 fix(gateway-agent): enforce per-lock authorization in v2 auth chain` |

## 1. 基线复核（与审查时的差异）

- **分支落后已大幅缓解**：审查时记“Android 领先 main 61 commits 未合并”，现为 **Android ahead 32 / behind 1**，iOS **ahead 9 / behind 3**（main 已通过多次 merge 追上）。仍需各开一个 PR 把分支并回 main。
- **door_ids 模型双端已就绪**：Android `CreateGuestRequest.doorIds`（AdminModels.kt:494，缺省 emptyList）、iOS `Guest.swift` 有 `doorIds`。缺的是**选门 UI** —— 调用点 Android 用缺省空、iOS 硬编码 `doorIds: []`（AdminGuestManagementView.swift:410）。
- **Android 假数据静默回退确认存在且面广**：19 个 admin 屏引用 `AdminDemoData` 且在出错时 `_error.value = null`（清空错误后回退假数据）。

## 2. 待办（按优先级）

### P1 — Android 假数据静默回退（M-1）
- **问题**：19 个 admin 屏在 API Error/Exception 时回退 `AdminDemoData` 并清空 error → 生产故障时管理台显示虚构的门/网关/用户，无任何报错。
- **改法**：登录态（生产）下不得回退假数据。统一一个 gating：仅当显式 demo/mock 环境（已有 mock/staging/prod 环境切换）才允许 `AdminDemoData`；否则把错误如实抛到 UI（error 状态 + 重试）。19 屏建议抽一个 helper 收口，不要逐屏改逻辑。
- **验收**：mock 环境行为不变；staging/prod 下断网/500 时屏幕显示错误而非假数据；至少覆盖 Gateways/Users/Events 三屏的单测或 UI 测试。
- **规模**：中（19 屏，但可收口到 1 个 helper + 逐屏替换）。

### P2 — 双端 create-guest 选门 UI（M-2）
- **问题**：后端 `door_ids`（commit 03536c5）+ 归属校验（B-2）已就绪，但两端创建访客都不让选门（Android 缺省空、iOS 硬编码 []），后端能力闲置。
- **Android**：CreateGuest 表单（AdminGuestManagementScreen.kt:~492）加门多选；门列表取该 place 的 doors（已有 my-doors / place doors API）；提交时传 `doorIds`。
- **iOS**：AdminGuestManagementView.swift:410 同样加多选并传 `doorIds`。
- **验收**：选门后创建的 guest，后端返回 door_ids 非空；选了不属于该 place 的门被后端 400（B-2 已保证，前端给出错误提示）。
- **规模**：中（双端各一个表单组件 + 门列表加载）。

### P3 — 分支并回 main（M-3）
- Android ahead 32 / iOS ahead 9。各开 PR、过 CI、合并。Android 分支历史早期有 cherry-pick 重复对，合并用 squash 或 merge 视团队习惯。
- **规模**：小（流程性，但需 CI 绿 + review）。

## 3. 与 Kisi 的移动端差距（非本批，记录）
- Wallet 凭证（Apple/Google Wallet 员工证）双端入口仍隐藏 —— 需 Apple/Google 钱包合作资质，列长期。
- MotionSense 式免操作开门（BLE ranging 自动解锁）—— 自研可行，Android 先行，单列。

## 4. 执行方式
M-1 / M-2 各挂一张 spawn_task 任务卡（指向对应仓库），可一键在新会话 + 独立 worktree 开工；M-3 走常规 PR 流程，不单独开卡。
