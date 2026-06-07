# OTA 舰队批量 + 灰度发布 — 设计文档

> 日期：2026-06-07
> 状态：设计已确认,待出实施计划
> 上层目标:**全面对标 Kisi OTA**。本文是子项 **#3**(#1 版本可见性、#2 固件仓库已完成)。
> 纯后端;**所有管理 UI 统一放子项 #5**。

---

## 1. 背景

#1 给了舰队版本可见性,#2 把签名固件收进平台。但发布仍是**逐网关**手建 OTA 任务。#3 加一层 **rollout(发布)编排器**:管理员选一个固件版本 + 一组网关 + 分阶段计划,平台自动分批建 OTA 任务、监控成功率、按门槛自动推进或暂停、可标记阶段需人工审批。

### 复用的现有基建(已核实)
- 网关分组只有 `BuildingID`(无 tags/labels)→ 定向维度 = 整租户 / 某楼 / 显式网关 id 列表。
- #2 固件仓库:`firmware_id` → OTA 任务自动填 sha256/signature/version。
- per-gateway OTA 任务:`CreateOTATask`、状态 queued/dispatching/succeeded/failed;状态上报走 `UpdateOTATaskStatus`(`service.go:1929`)——**灰度反应式推进的挂钩点**。
- #1:网关 `CurrentFirmwareVersion`(每网关进度展示用)。

---

## 2. 已确认的关键决策
| 决策 | 选定 |
|---|---|
| 推进模式 | **混合**:默认自动推进 + 失败门;阶段可标记 `requires_approval` → 该阶段边界停在 `awaiting_approval` 等人工 approve |
| 推进机制 | **反应式 + 懒评估,不新增后台 worker**:OTA 报告(`UpdateOTATaskStatus`)时推进 + 管理端 GET 详情时再评估(兜 stall 超时) |
| 定向 | `all`(租户全部)/ `building`(某楼)/ `gateways`(显式 id 列表) |
| 失败处理 | 某阶段失败率 ≥ 阈值 → 自动 `paused`(需人工);**不做 rollout 级整批自动回滚**(单网关回滚已由 agent 看门狗管;已更新的保持新版,管理员定夺) |
| UI | 统一放 #5 |

---

## 3. 数据模型 — Rollout
放新文件 `api/internal/modules/gateway/rollout.go`:
```go
type RolloutTarget struct {
	Kind       string   `json:"kind"`        // "all" | "building" | "gateways"
	BuildingID string   `json:"building_id,omitempty"`
	GatewayIDs []string `json:"gateway_ids,omitempty"`
}

type RolloutPhase struct {
	Percentage       int  `json:"percentage"`        // 累进覆盖率 1-100，严格递增，最后一阶段=100
	RequiresApproval bool `json:"requires_approval"` // 进入此阶段前需人工 approve
}

type GatewayRollout struct {
	ID                  string        `json:"id"`
	TenantID            string        `json:"tenant_id"`
	FirmwareID          string        `json:"firmware_id"`
	FirmwareVersion     string        `json:"firmware_version"`     // 冗余自固件记录，便于展示
	Target              RolloutTarget `json:"target"`
	Phases              []RolloutPhase `json:"phases"`
	FailureThresholdPct int           `json:"failure_threshold_pct"` // 默认 20
	State               string        `json:"state"`                 // pending|active|awaiting_approval|paused|completed|failed
	CurrentPhase        int           `json:"current_phase"`         // 0-based，当前已创建任务的阶段
	PhaseStartedAt      time.Time     `json:"phase_started_at"`      // 当前阶段建任务时间（算 stall 超时）
	CreatedBy           string        `json:"created_by,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}
```
每网关进度从其 OTA 任务派生(OTA 任务加 `RolloutID` + `RolloutPhase int` 字段)。

### Stall 超时
常量 `rolloutStallWindow = 1h`。当前阶段 cohort 里某网关在 `now - PhaseStartedAt > rolloutStallWindow` 仍无终态任务 → 计为 **failed(timed_out)**,进失败率。

---

## 4. 状态机
```
create → pending →(start)→ active
  active(phase i):给 cohort(i) 网关建引用 firmware_id 的 OTA 任务(带 RolloutID/RolloutPhase=i),PhaseStartedAt=now
  评估触发 = OTA 报告(UpdateOTATaskStatus) | 管理端 GET 详情:
    cohort(i) 全部达终态(succeeded/failed/stall-timed-out):
      失败率(failed/cohort_size)*100 ≥ FailureThresholdPct → paused
      否则:
        无下一阶段 → completed
        下一阶段.RequiresApproval → awaiting_approval(CurrentPhase 仍=i,等 approve)
        否则 → 建 cohort(i+1) 任务,CurrentPhase=i+1,PhaseStartedAt=now,保持 active
  admin:
    pause(**仅 active** → paused;暂停不再推进/建任务)
    resume(paused → active:当前阶段已达终态则**强制推进下一阶段**[=管理员覆盖失败门],否则继续等上报)
    approve(awaiting_approval → 建下一 cohort,active)
    abort(任意非终态 → failed,不再建任务)
  注:awaiting_approval 是故意的审批门,只能 approve 或 abort(不能 pause,否则 resume 会绕过审批)。
```
终态:`completed` / `failed`。`completed`/`failed` 拒绝 approve/resume/pause/abort(返回 409)。

---

## 5. 组件(单元,纯后端)

### 5.1 Rollout store(`gateway/rollout.go`)
- `CreateRollout(in CreateRolloutInput) (GatewayRollout, error)` — 校验:firmware_id 存在(同租户,复用 `findFirmwareLocked`)、target 非空且解析出 ≥1 网关、phases 非空且百分比严格递增/末=100/各 1-100、阈值 0-100(0 取默认 20)。建记录(state=pending),**立即 start**(建 phase 0 任务,state=active)。
- `GetRollout(tenantID, id) (GatewayRollout, error)`、`ListRollouts(tenantID) []GatewayRollout`。
- 状态转移:`ApproveRollout`/`PauseRollout`/`ResumeRollout`/`AbortRollout(tenantID, id, actor)`。
- 随 stateSnapshot 持久化(新 slice `rollouts` + snapshot 字段 + clone)。

### 5.2 Cohort 计算(`rollout.go`)
- `rolloutTargetGatewaysLocked(tenantID, target) []Gateway` — all=该租户全部;building=该租户 BuildingID 匹配;gateways=显式 id 且属该租户。**按 Gateway.ID 排序**(确定性 cohort 切分)。
- `cohortForPhase(all []Gateway, phases, phaseIdx) []Gateway` — 累进:`cum(i)=ceil(phases[i].Percentage*N/100)`,`cohort(i)=all[cum(i-1):cum(i)]`(cum(-1)=0)。

### 5.3 推进引擎(`rollout.go`)`advanceRolloutLocked(r *GatewayRollout)`
评估当前阶段终态 + 失败率 + stall → 转移 state / 建下一 cohort 任务。挂在 `UpdateOTATaskStatus`(报告后,若 task.RolloutID!="" 取 rollout 评估)与 GET 详情前。**Caller 持 s.mu。**

### 5.4 OTA 任务带 rollout 归属(`service.go`)
- `GatewayOTATask` 加 `RolloutID string json:",omitempty"` + `RolloutPhase int json:",omitempty"`。
- 内部 `appendRolloutOTATaskLocked(gw, fw, rolloutID, phase, now)` — 构造带 rollout 字段 + firmware sha/sig 的任务并 prepend(不走 CreateOTATask 的逐个校验,固件已在建 rollout 时校验)。

### 5.5 HTTP 端点(`routes_gateway_rollout.go`)
| 方法 路径 | 角色 | 作用 |
|---|---|---|
| `POST /gateways/rollouts` | write | 建 + start |
| `GET /gateways/rollouts` | write+operator | 列表 |
| `GET /gateways/rollouts/{id}` | write+operator | 详情(评估后返回 state + **每网关进度**) |
| `POST /gateways/rollouts/{id}/approve` | write | awaiting_approval → 进下一阶段 |
| `POST .../pause` `.../resume` `.../abort` | write | 状态控制 |

write = super_admin/tenant_admin/building_admin。详情的每网关进度 = {gateway_id, phase, ota_status(queued/dispatching/succeeded/failed/timed_out/pending), current_firmware_version}。

---

## 6. 数据流
```
admin POST /rollouts {firmware_id,target,phases,threshold} → 校验 → 建+start → phase0 cohort 建 OTA 任务
网关 pull→apply→report(succeeded/failed) → UpdateOTATaskStatus → advanceRollout:
  phase 全终态→评估→进下一 phase / awaiting_approval / paused / completed
admin GET /rollouts/{id} → 评估(兜 stall)→ 返回 state + 每网关进度
admin approve/pause/resume/abort 控制
```

---

## 7. 错误处理 / 边界
| 情况 | 行为 |
|---|---|
| firmware_id 不存在/跨租户 | 400 |
| target 解析出 0 网关 | 400 |
| phases 空 / 百分比非递增 / 末≠100 / 越界 | 400 |
| 阈值越界 | 400(0 → 默认 20) |
| 失败率 ≥ 阈值 | paused |
| cohort 网关 stall 超窗口 | 计 failed(timed_out) |
| approve 非 awaiting_approval | 409 |
| pause 非 active | 409 |
| resume 非 paused | 409 |
| abort 对已终态(completed/failed) | 409 |
| abort | failed,不再建任务(已建的不撤,agent 自己看门狗) |

---

## 8. 测试
- **cohort 计算**:all/building/gateways 解析 + 排序确定性;累进百分比切分(含 ceil 边界、单网关 fleet)。
- **状态机**:自动推进(模拟 report 全成功→进下一阶段→completed);失败门(失败率≥阈值→paused);awaiting_approval(标记阶段→停→approve→进);abort/pause/resume;completed/failed 拒绝控制(409)。
- **stall 超时**:cohort 网关不报 + 越窗 → 计 failed(注入 PhaseStartedAt 过去时间);GET 详情触发评估。
- **reactive 推进**:UpdateOTATaskStatus 报告 rollout 任务 → 触发 advance。
- **OTA 任务归属**:cohort 任务带 RolloutID/RolloutPhase。
- **端点**:建(校验各 400)、列表、详情(每网关进度)、approve/pause/resume/abort + 角色。

---

## 9. 改动文件
**新增**
- `api/internal/modules/gateway/rollout.go` — 模型 + store + cohort + 推进引擎 + 错误
- `api/internal/modules/gateway/rollout_test.go`
- `api/internal/http/routes_gateway_rollout.go` — 6 端点 + 每网关进度组装
- `api/internal/http/routes_gateway_rollout_test.go`

**修改**
- `api/internal/modules/gateway/service.go` — OTA 任务加 `RolloutID`/`RolloutPhase`;`UpdateOTATaskStatus` 末触发 `advanceRolloutLocked`;`appendRolloutOTATaskLocked`;snapshot 持久化 `rollouts`
- `api/internal/http/router.go` — 注册 6 路由

---

## 10. 不做(YAGNI / 留后续)
管理 UI(#5)、tags 定向(只有 building)、跨租户、复杂 cohort 策略(只累进百分比)、rollout 级整批自动回滚、定时/窗口调度(#4)、config/pull 触发推进(report + GET 已够,留作增强)。

## 11. 工作量
约 1.5–2 天(纯后端;状态机 + cohort + 反应式推进 + 6 端点 + 每网关进度)。
