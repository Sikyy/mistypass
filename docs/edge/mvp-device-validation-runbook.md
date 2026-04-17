# Edge Controller MVP 实体联调 Runbook（2026-04-17）

## 1. 目标

- 目标：验证 `Controller 本地判定优先` 链路，覆盖单门 I/O 闭环与 Cloud 异步补传合同。
- 范围：MVP 台架验证，仅覆盖“门控可运行 + 协议兼容 + 事件可补传”，不包含量产级 EMC/ESD/浪涌认证。

能力状态：

- `CONTRACT_READY`：Cloud API 与关键回归脚本已可执行。
- `PROD_READY`：路线已冻结为 `Cloud Access SaaS + Edge Controller + Reader 兼容层`。
- `BLOCKED_EXTERNAL`：仅用于外部 API/资质事项；当前本 runbook 主项无外部阻塞。
- 状态标识口径见：`docs/architecture/capability-status-markers.md`

## 2. 前置条件

- 台架硬件到位：Orange Pi Zero 3（1G 套件）、2 路光耦、FT232RNL RS485、杜邦线。
- 单门闭环物料到位：锁控继电器板、12V/24V 门锁电源、门磁、REX、防拆开关。
- 网络可达：Edge 主机可访问 API（本地或测试环境）。
- API 服务可用：`GET /healthz` 返回 `200`。
- 测试账号可用：`superadmin@mistypass.local / admin123`。
- 可选：WalletMate II 仅作为实验输入源，不是执行本 runbook 的前置必需。

## 3. 推荐执行顺序

0. 一键包装（推荐）  
   `docs/testing/run-edge-mvp-validation.zsh`
1. Legacy 接入基线  
   `docs/testing/curl-gateway-legacy-wiegand-poc.zsh`
2. 单门 I/O 事务链路（R8 A3）  
   `docs/testing/curl-gateway-door-io-loop.zsh`
3. 协议与设备注册基线  
   `docs/testing/curl-gateway-serial-protocol.zsh`
4. 幂等回放基线  
   `docs/testing/curl-gateway-event-idempotency.zsh`
5. retry subset 混合场景  
   `docs/testing/curl-gateway-event-retry-subset-mixed.zsh`
6. checkpoint 保护场景  
   `docs/testing/curl-gateway-event-checkpoint-partial.zsh`
7. 执行器仿真对照（Cloud 侧）  
   `docs/testing/curl-gateway-edge-queue-executor-sim.zsh`
8. 本地 I/O 实测（手工）
   - 继电器触发 -> 门磁状态变化 -> REX/防拆输入 -> 事件上报连续性。

## 4. 执行命令模板

```bash
# 示例：把 API 指向测试环境端口
export API_BASE_URL="http://<api-host>:8080"
export LOGIN_EMAIL="superadmin@mistypass.local"
export LOGIN_PASSWORD="admin123"

/bin/zsh docs/testing/run-edge-mvp-validation.zsh

# 如需逐条执行：
/bin/zsh docs/testing/curl-gateway-legacy-wiegand-poc.zsh
/bin/zsh docs/testing/curl-gateway-door-io-loop.zsh
/bin/zsh docs/testing/curl-gateway-serial-protocol.zsh
/bin/zsh docs/testing/curl-gateway-event-idempotency.zsh
/bin/zsh docs/testing/curl-gateway-event-retry-subset-mixed.zsh
/bin/zsh docs/testing/curl-gateway-event-checkpoint-partial.zsh
/bin/zsh docs/testing/curl-gateway-edge-queue-executor-sim.zsh
```

## 5. 证据留存模板

每条脚本至少留存：

- 执行时间（本地时区）
- 设备连接拓扑照片/编号
- 脚本 PASS/FAIL
- 关键断言（`dedup/retry_subset/checkpoint`）
- 失败日志路径

本地 I/O 实测需额外留存：

- 接线图版本（relay/door_contact/rex/tamper）
- 每次输入触发对应事件（含时间戳）
- 30 分钟稳定性结论

建议记录表：

| 项目 | 值 |
|---|---|
| 执行日期 | `<YYYY-MM-DD>` |
| 执行人 | `<name>` |
| 设备序列号 | `<controller host / rs485 adapter / optional reader>` |
| API 地址 | `<http://host:port>` |
| 脚本 | `<script path>` |
| 结果 | `PASS / FAIL` |
| 关键断言 | `<dedup / retry_subset / checkpoint / io-loop>` |
| 日志附件 | `<path or link>` |
| 备注 | `<异常与结论>` |

## 6. 通过标准（MVP）

- 关键脚本全部 PASS。
- 重放事件不产生重复污染（`deduplicated=true` 且累计量不异常增长）。
- checkpoint 回退与越界均触发预期 `409` 保护。
- 单门 I/O 闭环连续 30 分钟无持续错误。
- 断网后本地判定与事件排队可持续，恢复后补传成功。

## 7. 异常处理与回退

- 设备未识别：先确认供电与线序，再检查串口/USB 枚举（如 `/dev/ttyUSB0`）。
- Wiegand/OSDP 参数不匹配：按设备手册修正后复测，并记录参数变更。
- API 不可达：先确认 `healthz` 与鉴权配置。
- 若实体链路失败但 Cloud 合同脚本通过：记录为台架接线/设备侧缺陷，保留日志并进入硬件排障分支，不阻塞 Cloud 主链路。
