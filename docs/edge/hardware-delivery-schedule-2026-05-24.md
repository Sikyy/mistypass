# 硬件交付排期：Edge Controller / Reader / Door Loop

> 日期：2026-05-24
> 能力状态：CONTRACT_READY
> 参考现状：`docs/wiki/internal/priority-board.md` 中 R8 为 58%，R6 为 68%；`docs/MVP-ROADMAP.md` 中 M5 仍为待推进。

## 1. 目标

这份排期只围绕一件事：让 Mistyislet 能稳定完成“云端授权 -> 边缘控制器 -> 读头/门锁 -> 事件回传”的真实硬件闭环。

不把 Apple Wallet / Google Wallet 真实发卡、Meta WhatsApp 真实通道、摄像头高级能力放进 P0。它们重要，但不能阻塞第一扇真实门打开。

## 2. 当前判断

| 模块 | 当前判断 | 风险 |
|---|---|---|
| Cloud API / Gateway routes | 主链路基本具备，gateway checkpoint/OTA/config/verify 有 API | 需要实机回归证据 |
| Wiegand input | 设计与计划文档已存在 | 需要真 GPIO / reader 台架验证 |
| OSDP / RS485 | 有设计/代码痕迹，优先级高于扩展功能 | 需要 RS485 真机参数和异常清单 |
| Door relay / lock | MVP 需要从模拟转真实 I/O | 电压、继电器、锁体接线安全 |
| Web Admin hardware UI | 基础硬件管理存在 | 缺少“实机验证状态”和“配置下发结果”的运营视图 |
| iOS / Android | 开门和 BLE/NFC 相关能力已有 | 真机 smoke 暂放，先做契约和后端稳定 |

## 3. 推荐硬件范围

### P0 台架

- Edge controller：现有 Orange Pi Zero 3 或等价 Linux SBC。
- Relay：1 路继电器模块，先接灯/蜂鸣器模拟门锁，再接锁。
- Reader：Wiegand 26/34 读头。
- RS485：USB to RS485，用于 OSDP 预研。
- Credential：Mifare/IC 卡若干，至少包含白名单、黑名单、未知卡。
- 电源与安全：独立 12V 门锁电源、3.3V/5V level shift、万用表。

### P1 试点

- 2D/4D Controller 候选板。
- OSDP reader。
- 门磁 / REX / emergency input。
- PoE 或 UPS 供电方案。

### 暂不纳入 P0

- 自研 PCB。
- 量产外壳。
- Tamper sensor。
- 摄像头联动开门。
- 多门大规模离线授权压测。

## 4. 排期

### W0：准备与基线冻结（2026-05-24 至 2026-05-25）

目标：把台架材料、端口、接线、安全边界固定下来。

2026-05-24 更新：W0 台架资源 ID、接线假设、安全门禁和冒烟脚本已冻结到 [Hardware Bench W0 Freeze](hardware-bench-w0-freeze-2026-05-24.md)。拿到硬件后按该文档补齐实测证据，再进入 W1。

任务：

- 确认现有硬件清单和照片归档。
- 冻结第一轮 `gateway_id`, `reader_id`, `lock_id`, `tenant_id`, `place_id`。
- 在 Web Admin 建好对应 Place / Door / Gateway / Reader / Access Rule。
- 写一份接线表：GPIO、Wiegand D0/D1、RS485 A/B、relay NO/NC/COM。
- 确认 Cloud API 当前生产地址 `https://api.mistyislet.com/healthz` 可用。

验收：

- 文档里能找到每个硬件序列号、接线图、API 资源 ID。
- 不接锁体时，继电器动作可用灯/蜂鸣器验证。

### W1：单门 I/O 闭环（2026-05-26 至 2026-05-31）

目标：先让 API 命令真实触发边缘输出。

任务：

- Gateway agent 能拉取配置并绑定本地 lock/relay。
- `POST /api/v1/locks/{id}/unlock` 或 app unlock 进入 gateway command。
- Gateway 执行 relay pulse，回传 command ack 和 access event。
- Web Admin 看到最后在线时间、最后命令、最后事件。

验收：

- 连续 30 次远程开门，成功率 100%。
- 断网 2 分钟后恢复，checkpoint 不丢事件。
- event 中能关联 user / credential / door / gateway。

### W2：Wiegand 读头闭环（2026-06-01 至 2026-06-07）

目标：刷实体卡能触发后端授权判断和开门。

任务：

- Wiegand D0/D1 电压确认，必要时加 level shifter。
- 26/34 bit 解码实测，记录 facility code / card number。
- Gateway 调 `/api/v1/gateway/verify-credential`。
- allow -> relay unlock，deny -> 不动作并写失败事件。
- 加入 unknown card、expired schedule、revoked card 三类负例。

验收：

- 白名单卡 20 次刷卡全部开门。
- 黑名单/未知/过期卡全部拒绝。
- Web Admin 事件列表能区分 allow/deny 和失败原因。

### W3：OSDP / RS485 基线（2026-06-08 至 2026-06-14）

目标：形成 OSDP 可行性结论，不要求一次性量产。

任务：

- RS485 接线和终端电阻确认。
- 读头地址、波特率、secure channel 支持矩阵。
- OSDP poll / LED / buzzer / output command 最小闭环。
- 与 Wiegand 做同一张卡的行为对照。

验收：

- OSDP 至少完成一条稳定读卡或控制路径。
- 输出参数基线：address、baud、parity、timeout、retry。
- 明确 OSDP v2 secure channel 是 P1 还是 P2。

### W4：离线运行与恢复（2026-06-15 至 2026-06-21）

目标：网络不稳定时，门禁仍然可控、事件可追。

任务：

- Gateway 缓存 access rules。
- 离线刷卡 allow/deny。
- 离线事件本地队列。
- 恢复联网后 batch upload + checkpoint。
- 规则更新后的 cache version 生效验证。

验收：

- 断网 30 分钟内，白名单卡可开门，黑名单卡拒绝。
- 恢复后事件顺序和去重正确。
- 规则撤销后 gateway 在下一次配置同步后拒绝旧卡。

### W5：双端 smoke 与运营视图（2026-06-22 至 2026-06-28）

目标：管理员和用户视角都能解释硬件状态。

任务：

- iOS / Android 使用生产或 staging API 跑登录、门点、开门 smoke。
- Web Admin Hardware 页显示 gateway online、reader status、last config applied、last checkpoint。
- Report hardware PDF 能展示真实硬件状态。
- 编写现场排障手册：离线、读卡失败、继电器不动作、权限拒绝。

验收：

- iOS / Android 各完成 10 次远程开门。
- Admin 能定位失败原因，不需要看服务器日志。
- hardware PDF 使用真实台架数据。

### W6-W7：小范围试点准备（2026-06-29 至 2026-07-12）

目标：从台架走向一扇真实非关键门。

任务：

- 门锁电源和消防/逃生安全审查。
- 安装位置、布线、弱电箱空间确认。
- 监控告警：gateway offline、reader offline、door forced/open too long。
- 试点回滚方案：保留原有钥匙/卡系统。
- 试点 checklist 签字。

验收：

- 一扇非关键门完成 3 天试运行。
- 手动回滚不超过 10 分钟。
- 每天导出事件和异常报告。

## 5. 推荐执行顺序

| 优先级 | 工作包 | 推荐时间 |
|---:|---|---|
| P0 | W0 + W1 单门输出闭环 | 立即开始 |
| P0 | W2 Wiegand 刷卡闭环 | W1 通过后马上做 |
| P1 | W3 OSDP / RS485 基线 | Wiegand 有稳定结果后做 |
| P1 | W4 离线运行 | 第一轮硬件可控后做 |
| P1 | W5 双端 smoke 与运营视图 | 离线恢复前后都可以并行 |
| P2 | W6-W7 小范围试点 | 台架证据完整后做 |

## 6. 阻塞项

| 阻塞 | 影响 | 处理 |
|---|---|---|
| 没有真实 reader/lock | 不能完成 P0 验收 | 先用 relay + 灯验证输出，但 Wiegand 仍需读头 |
| 不确定 GPIO 电平 | 可能损坏 SBC | 上电前用万用表确认，必要时加 level shifter |
| 没有固定 resource ID | 事件和配置难以追踪 | W0 冻结 ID 并写入台架文档 |
| Mobile OpenAPI 缺失 | 三端 smoke 依赖手写路径 | 与 API 审计 Batch A 并行推进 |
| 邮件 provider 未配置 | 报表/告警不能真实通知 | 按邮件方案 E0 接 Cloudflare Email Service，Resend 仅作 fallback |

## 7. 产出物清单

- [W0 台架冻结文档](hardware-bench-w0-freeze-2026-05-24.md)。
- 台架接线图和照片。
- 硬件资源 ID 表。
- Wiegand / OSDP 参数基线。
- API smoke 记录。
- iOS / Android smoke 记录。
- Web Admin Hardware 截图。
- hardware PDF 示例。
- 试点回滚手册。
