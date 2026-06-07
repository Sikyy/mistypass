# OTA Rollout 调度(start_at + 维护窗口)— 设计文档

> 日期：2026-06-07
> 状态：设计已确认,待出实施计划
> 上层目标:**全面对标 Kisi OTA**。本文是子项 **#4**(#1 版本可见性、#2 固件仓库、#3 灰度发布已完成)。
> 纯后端;扩展 #3;管理 UI 统一放 #5。

---

## 1. 背景与范围

#3 给了受控灰度 rollout(分阶段 + 失败门 + 审批)。#4 加**调度**:rollout 可定时启动(`start_at`)并只在**每日维护窗口**内推进阶段(如门禁只在凌晨 02:00–05:00 更新)。

**范围 = 仅调度。** 决定:门禁控制器**静默自动更新有风险**(绕过 #3 的金丝雀/失败门),故 #4 只做调度(安全、最对标 Kisi);"网关自动跟通道更新"留后续。

**延续 #3:反应式 + 懒评估,不新增后台 worker。** config/pull(目标网关每 ~30s 轮询)= 天然 tick:窗口一开,目标网关下次 pull(30s 内)即启动 rollout。

### 复用的现有基建(已核实)
- #3 rollout 状态机 + `advanceRolloutLocked`/`startRolloutPhaseLocked`/`evaluateRolloutPhaseLocked`/`rolloutTargetGatewaysLocked`(`gateway/rollout.go`)。
- config/pull(`gatewayBootstrapConfigPull`,`router_handlers_gateway.go:55/85-98`)已是反应式钩子(记录版本、组装 pending tasks)。
- **时区库:** `time/tzdata` 和 `LoadLocation` 当前**都未用**→ 依赖 OS tzdata。最小容器里 `LoadLocation` 可能失败,故本项需入口引入 `_ "time/tzdata"`(嵌入,~450KB)。

---

## 2. 已确认的关键决策
| 决策 | 选定 |
|---|---|
| 调度原语 | `start_at`(一次性,不早于 T 启动)+ **每日维护窗口**(本地时段 + IANA 时区,支持跨午夜如 22:00→05:00) |
| 窗口门控粒度 | **只门控阶段启动**(创建该阶段 cohort 的 OTA 任务);已派发任务网关照常 apply(不撤回) |
| 触发机制 | 反应式:config/pull(目标网关)+ OTA 报告 + 管理端 GET。**不新增 worker** |
| 状态 | 新增 `scheduled`(当前阶段任务尚未创建,等窗口);其余沿用 #3 |

---

## 3. 数据模型 — Schedule
`gateway/rollout.go`,`GatewayRollout` 加 `Schedule *RolloutSchedule json:"schedule,omitempty"`(为空 = 无调度,行为同 #3):
```go
type RolloutSchedule struct {
	StartAt     *time.Time `json:"start_at,omitempty"`     // 不早于此绝对时间(UTC)启动；可空=立即
	WindowStart string     `json:"window_start,omitempty"` // "HH:MM" 本地24h；可空
	WindowEnd   string     `json:"window_end,omitempty"`   // "HH:MM" 本地；< start 表示跨午夜
	Timezone    string     `json:"timezone,omitempty"`     // IANA，如 "Asia/Jakarta"；空=UTC
}
```
新增状态常量 `rolloutStateScheduled = "scheduled"`。

---

## 4. `scheduleOpenLocked(sch *RolloutSchedule, now time.Time) bool`
判断"现在能否启动一个阶段":
1. `sch == nil` → **true**(无调度,立即;= #3)。
2. `sch.StartAt != nil && now.Before(*sch.StartAt)` → false(还没到启动时间)。
3. 窗口已设(`WindowStart != "" && WindowEnd != ""`):
   - `loc` = `LoadLocation(Timezone)`(空 → UTC);`hhmm` = `now.In(loc).Format("15:04")`。
   - 正常(`WindowStart <= WindowEnd`):`WindowStart <= hhmm && hhmm < WindowEnd`。
   - 跨午夜(`WindowStart > WindowEnd`):`hhmm >= WindowStart || hhmm < WindowEnd`。
   - (零填充 "HH:MM" 字符串比较 = 时序比较。)
4. 窗口未设 → 仅看 StartAt(步骤 2 过了即 true)。

> Timezone 在 CreateRollout 时已校验 `LoadLocation` 成功,故这里失败兜底按 UTC(不 panic)。

---

## 5. 状态机整合(核心:门控"启动阶段")

把 #3 里每处"启动一个阶段"经由新 helper:
```go
// tryStartPhaseLocked: 窗口开则建该阶段任务转 active；否则停在 scheduled 等窗口。Caller 持 s.mu。
tryStartPhaseLocked(r, phase, all, now):
    if scheduleOpenLocked(r.Schedule, now):
        startRolloutPhaseLocked(r, phase, all, now)   // 建 cohort 任务，CurrentPhase=phase，state=active
    else:
        r.CurrentPhase = phase; r.State = scheduled; r.UpdatedAt = now   // 不建任务，等窗口
```

- **CreateRollout**:原 #3 直接 `startRolloutPhaseLocked(0)` → 改为 `tryStartPhaseLocked(0)`。有调度且窗口关 → `scheduled`;否则 → `active`(建 phase0)。
- **advanceRolloutLocked** 扩展循环,同时处理 `scheduled` 与 `active`:
  ```
  loop:
    state==scheduled:
       scheduleOpen? 否 → break(继续等)；是 → startRolloutPhaseLocked(CurrentPhase)→active→continue
    state==active:
       评估当前阶段(同 #3:终态/失败率/stall)
       未终态 → break
       失败率≥阈值 → paused → break
       最后阶段 → completed → break
       下一阶段 RequiresApproval → awaiting_approval → break
       否则 → tryStartPhaseLocked(CurrentPhase+1)   // 窗口关则落 scheduled（下一轮 scheduled 分支判定后 break）
    其它 state → break
  ```
  循环终止:active-advance 时 CurrentPhase 单调增(上界 len(Phases));scheduled 分支窗口关即 break。

- **语义**:`scheduled` = `CurrentPhase` 这阶段任务**尚未创建**;`active` = 已创建运行中。窗口关的阶段间也回 `scheduled`。

---

## 6. config/pull 反应式触发
新服务方法 `EvaluateGatewayScheduledRollouts(tenantID, gatewayID string) error`(持锁):遍历该租户 `state==scheduled` 且 `target` 含 `gatewayID` 的 rollout,各 `advanceRolloutLocked`(窗口开则启动);若有变化 `persistLocked`。
- `router_handlers_gateway.go` 的 `gatewayBootstrapConfigPull` 在记录版本后调用它(忽略错误,best-effort)。
- 任一目标网关在窗口内 pull → 启动该 rollout(其 cohort 任务建好,待对应网关下次 pull 取走)。

---

## 7. 端点(无新增,schedule 并入 create)
- `POST /gateways/rollouts` 请求体加可选 `schedule {start_at, window_start, window_end, timezone}`,透传 `CreateRolloutInput.Schedule`。
- 详情/列表已返回整个 `GatewayRollout`(含 `schedule` + `state`),无需改。
- 控制:`abort` 可作用于 `scheduled`(→ failed,取消未启动的调度);`pause`(仅 active)/`resume`/`approve` 不作用于 `scheduled`(返回 409)。
- **重要(与 #3 advance-first 的交互):** #3 的 `pause`/`abort` 会先 `advanceRolloutLocked` 刷新状态。但 #4 后 `advanceRolloutLocked` 在窗口开时会**启动** scheduled rollout——若 abort/pause 无条件 advance-first,可能在窗口刚开的瞬间把 scheduled 启动(建了 phase0 任务)再 fail/pause,产生孤儿任务。故 **pause/abort 的 advance-first 仅当 `state==active` 时执行**;`scheduled` 不 advance:pause→409、abort→直接置 failed。

---

## 8. 校验 / 边界(CreateRollout)
| 情况 | 行为 |
|---|---|
| Timezone 非空且 `LoadLocation` 失败 | 400 `ErrRolloutScheduleInvalid` |
| window_start/end 只设其一 | 400(要么都设要么都空) |
| window_start/end 非 "HH:MM"(00-23:00-59) | 400 |
| (其余沿用 #3:firmware/target/phases/threshold) | 同 #3 |
| 入口缺 `time/tzdata` 且 OS 无 tz | LoadLocation 失败 → 调度不可用(本项含加 tzdata) |

---

## 9. 测试
- **scheduleOpenLocked**:nil→开;StartAt 前→关/后→开;窗口内→开/外→关;跨午夜(23:30 在 22:00–05:00 内、12:00 外);时区(注入 UTC vs Asia/Jakarta 同一 UTC 时刻不同判定);窗口未设只看 StartAt。
- **CreateRollout 带调度**:窗口关 → state=scheduled 且**无 phase0 任务**;窗口开/无调度 → active 且有任务。
- **advance**:scheduled + 窗口开(注入 StartAt 过去/窗口全天)→ 启动 phase0;阶段间窗口关 → 回 scheduled(注入 PhaseStartedAt + 关窗口)。
- **EvaluateGatewayScheduledRollouts**:scheduled rollout + 目标网关 → 触发启动;非目标网关 → 不动。
- **校验**:坏时区/半截窗口/坏 HH:MM → 400。
- **abort scheduled** → failed。
- HTTP:create 带 schedule → 201 且 state 正确。

---

## 10. 改动文件
**修改**
- `api/internal/modules/gateway/rollout.go` — `RolloutSchedule` + `scheduled` 常量 + `scheduleOpenLocked` + `tryStartPhaseLocked` + `advanceRolloutLocked` 扩展 + `CreateRollout` 收 schedule + 校验 + `EvaluateGatewayScheduledRollouts` + 错误 `ErrRolloutScheduleInvalid`
- `api/internal/modules/gateway/rollout_test.go`
- `api/internal/http/routes_gateway_rollout.go` — create 请求体加 `schedule`
- `api/internal/http/routes_gateway_rollout_test.go`
- `api/internal/http/router_handlers_gateway.go` — config/pull 调 `EvaluateGatewayScheduledRollouts`
- `api/internal/http/routes_gateway_bootstrap_test.go`
- `api/cmd/api/main.go` — 加 `_ "time/tzdata"`(确保 LoadLocation 全环境可用)

> 持久化:`Schedule` 是 `GatewayRollout` 内嵌字段,随既有 rollout 持久化(`cloneGatewayRollouts` 浅拷贝含指针——`Schedule` 指针在记录创建后不可变,浅拷贝安全;若审查认为需深拷贝,实现时加)。

---

## 11. 不做(YAGNI / 留后续)
网关静默自更、cron 表达式、每阶段不同窗口、窗口门控在途任务(只门控阶段启动)、UI(#5)、调度的"下次窗口时间"计算展示(可后续)。

## 12. 工作量
约 1 天(纯后端;调度判定 + 状态机门控 + config/pull 触发 + 校验 + tzdata)。
