# 强制 + 验证的 OTA 固件签名 — 设计文档

> 日期：2026-06-07
> 状态：设计已确认，待出实施计划
> 范围：gateway-agent 二进制自更新 + 离线 Ed25519 签名 + agent 侧强制验签

---

## 1. 背景与现状

记忆/路线图曾把"OTA 固件签名"标为"依赖硬件"（NEXT-ROADMAP H2），经 2026-06-07 核对代码，**这是误判 —— 它是纯软件任务**，不依赖任何硬件采购或第三方账号，且现有 Orange Pi Zero3（局域网 192.168.1.49，已在运行 gateway-agent）就具备端到端测试条件。

### 已具备的服务端基础

| 能力 | 位置 |
|---|---|
| 创建 OTA 任务（已带 `firmware_sha256` + `firmware_signature` 字段） | `api/internal/http/router.go:995`、`api/internal/modules/gateway/service.go:1709` |
| Ed25519 签名 / SHA256 **格式**校验 | `api/internal/modules/gateway/service.go:2663`（`isValidEd25519SignatureHex`，注释明确 64 字节 = 128 hex 字符） |
| config pull 把 `pending_ota_tasks`（含 URL+SHA256+签名）下发给 agent | `api/internal/http/router_handlers_gateway.go:81` |
| OTA 状态回报端点 `/gateway/ota/report` | `api/internal/http/router.go:854` |
| agent 已有"固定/pin"模式可参照 | `api/cmd/gateway-agent/main.go:48`（`--tls-pin-sha256`）、mTLS 设备证书 `mtls.go` |

### 两个待补缺口（= 本设计要解决的"强制 + 验证"）

1. **强制（mandatory）未做**：`CreateOTATask` 中 sha256 与 signature 当前是**可选**的 —— `api/internal/modules/gateway/service.go:1734` 的逻辑为"签名非空才校验"，空签名照样放行，因此现在能发布未签名固件。
2. **验证（verify-before-flash）未做**：gateway-agent **没有任何 OTA 代码**。`pullConfig()`（`api/cmd/gateway-agent/agent.go:373`）虽然已收到 `pending_ota_tasks`，但直接忽略 —— 没有下载、没有 SHA256 校验、没有 Ed25519 验签、没有刷写、没有回报。（agent 现有的 `VerifyBLESignature` 是 BLE 认证用的 ECDSA，与 OTA 的 Ed25519 无关。）

此外：尚无签名密钥对，agent 内也尚未固定任何 OTA 公钥。

---

## 2. 目标与非目标

### 目标
- 任何下发到网关的固件必须带**有效 Ed25519 签名**；agent 在替换自身二进制**之前**用**固定的公钥**验签，验不过一律拒绝、绝不刷写。
- 服务端在创建 OTA 任务时**强制要求** sha256 + 签名存在且格式合法。
- 提供离线签名工具，私钥全程不离开操作者本地机器。
- 全程经 `/gateway/ota/report` 上报状态，失败可观测、可回滚。

### 非目标（YAGNI，本期排除）
- KMS / HSM 密钥托管（本期用离线密钥文件）。
- A/B 双分区 / 独立 updater 进程（本期用"最小自更新"机制 A）。
- 服务端验签（信任锚在 agent 的固定公钥；况且离线签名模式下固件字节不在服务端，服务端也算不出 sha256）。
- MCU（如 ESP32）固件 / 整机 OS 镜像更新。

---

## 3. 信任模型（核心原则）

**信任锚 = agent 内固定的公钥（compile-time 常量或启动 flag）。**

服务器 / Mac mini 只负责存储和分发 OTA 任务，**不持有私钥、不是信任锚**。因此即使分发服务器被攻破，攻击者也拿不到私钥，伪造不出能通过 agent 验签的固件 —— 这正是 OTA 签名的全部意义。

⚠️ 由此推出两条不可违背的约束：
1. **私钥绝不进入 API 进程能读到的地方**（见 §4 密钥托管 = 离线签名）。
2. **验签公钥必须 pin 在 agent 内**，绝不跟固件一起从同一服务端动态获取 —— 否则攻击者控制服务端后自带一对密钥+签名即可绕过，签名形同虚设。

---

## 4. 已确认的关键决策

| 决策点 | 选定方案 | 理由 |
|---|---|---|
| **签名私钥托管** | **离线签名** —— 私钥只在操作者本地/一台不对外的机器，手动用 CLI 签固件，签名填进 OTA 任务。API/Mac mini 永不接触私钥。 | 最强隔离、几乎零额外设施；Mac mini 即使被攻破也伪造不了固件。 |
| **更新对象** | **gateway-agent 二进制自更新** | MVP 架构里 BLE 走 Orange Pi + USB dongle，无独立 MCU；唯一可更新的软件就是 agent 自身。纯软件、现有硬件可端到端测。 |
| **apply / 回滚机制** | **机制 A：最小自更新** —— 下载→验签→原子 rename→systemd 重启→健康确认；失败用 `.bak` + ExecStartPre 守护回滚。 | 零额外设施、纯软件、现在就能在 Orange Pi 上跑通。A/B 与独立 updater 是将来上规模（无法 SSH 兜底）时的加固方向。 |

---

## 5. 组件设计（4 个单元）

### 5.1 离线签名 CLI — 新增 `api/cmd/ota-sign/`
单一职责：生成密钥、对固件签名。私钥永不离开本地。

子命令：
- `ota-sign gen-key --out-priv priv.pem --out-pub pub.hex`
  生成 Ed25519 密钥对。私钥写为 PKCS#8 PEM（操作者离线保管）；公钥写为 hex（用于填进 agent 的 `--ota-pubkey`）。
- `ota-sign sign --key priv.pem --version <ver> --in <binary>`
  计算固件 SHA256 → 构造规范化消息（见 §6）→ Ed25519 签名 → 打印 `sha256`、`signature`，以及**可直接粘贴到 create-task API 的 JSON 请求体**。

依赖：仅 Go 标准库 `crypto/ed25519`、`crypto/sha256`。

### 5.2 服务端强制校验 — 改 `api/internal/modules/gateway/service.go`
`CreateOTATask`（`:1709`）：把 `firmware_sha256` 与 `firmware_signature` 从可选改为**必填**：
- 空 sha256 → 返回新增错误 `ErrGatewayOTAFirmwareSHA256Required`。
- 空 signature → 返回新增错误 `ErrGatewayOTAFirmwareSignatureRequired`。
- 非空但格式非法 → 沿用现有 `ErrGatewayOTAFirmwareSHA256Invalid` / `ErrGatewayOTAFirmwareSignatureInvalid`。
- HTTP 层（`routes_gateway_management.go`）把新错误映射为 400。

服务端**不验签**（不持有固件字节，也不是信任锚），只保证"存在 + 格式合法"后存储并分发。

### 5.3 agent 固定公钥 — 改 `api/cmd/gateway-agent/main.go`
- 新增 flag `--ota-pubkey`：Ed25519 公钥，hex 编码；**支持逗号分隔多个**（当前 + 下一把），以便将来轮换密钥。可选编译期默认常量。
- **未配置任何公钥 → OTA 子系统关闭**：agent 记一条 warning 并**忽略**所有 `pending_ota_tasks`（fail-closed，绝不刷写未验证固件）。

### 5.4 agent OTA 执行器 — 新增 `api/cmd/gateway-agent/ota.go`
在 `pullConfig()` 拿到 `pending_ota_tasks` 后被调用，负责下载/验签/替换/重启编排与状态上报（流程见 §7）。

---

## 6. 签名内容（防调包 + 防降级）

签名对象是一段**规范化消息**，把"版本"和"字节"绑死：

```
message = "mistypass-ota-v1\n" + firmware_version + "\n" + lowercase_hex(sha256(binary))
signature = Ed25519_sign(privkey, message)        // 64 字节，hex 编码（128 字符）
```

- agent 验签时用**任务里的 `firmware_version`** + **自己下载后算出的 sha256** 重建同一段消息，再对固定公钥 `ed25519.Verify`。
  → 攻击者既改不了字节（sha256 变），也无法把一份旧固件冒充成更高版本（version 在签名内）。
- **域分隔前缀** `mistypass-ota-v1` 防止签名被挪作他用，并为未来协议升级留版本位。
- **防降级**：agent 拒绝 `task.firmware_version ≤ 当前运行版本`。需要 agent 带编译期版本号（`-ldflags "-X main.version=<ver>"`）；若当前无此变量，本期补上。

---

## 7. agent 自更新流程 + 回滚（机制 A）

```
pullConfig 收到 pending_ota_task（version 比自己新、且未在防降级/格式检查中被拒）
 ├─ report downloading → 下载 firmware_url 到二进制同分区的临时文件（保证 rename 原子）
 ├─ report verifying  → 比对 sha256 + 重建消息并 Ed25519 验签（对固定公钥列表任一把通过即可）
 │     ✗ 下载失败 / sha256 不符 / 验签失败 / 版本降级 → report failed；旧二进制原封不动，继续运行
 ├─ report applying   → chmod +x；备份当前二进制 → <bin>.bak；原子 rename(tmp → bin)
 │     写 ota-pending 标记文件（{new_version, task_id, bak_path, confirmed:false}）；exit(0)
 └─ systemd（Restart=always）拉起新版二进制
       ├─ ExecStartPre 守护脚本（mistypass-ota-guard.sh，每次启动先跑）：
       │     读标记 → 若存在且 confirmed=false → attempts++
       │       · attempts ≥ 3 → 用 .bak 还原二进制、清标记（自动回滚兜底，覆盖"新版根本起不来"）
       ├─ 新版 agent 启动 → 见到 ota-pending 标记 → 跑健康检查（首次 pullConfig 成功，≤60s）
       │     ✓ 健康 → 标记 confirmed=true / 删除标记 + 删 .bak → report success
       │     ✗ 超时不健康 → 用 .bak 还原 → 删标记 → exit → systemd 拉起旧版 → report rolled_back
```

设计要点：
- **回滚逻辑的兜底在 ExecStartPre 守护脚本里**（它总在 agent 之前运行，即使 agent 完全崩溃也能执行还原），因此机制 A 的自动回滚也能覆盖"新版根本起不来"。agent 自身只负责"健康确认 + 报成功"和"起得来但不健康时的主动回滚"。
- Linux 允许 `rename` 覆盖**正在运行**的二进制（旧 inode 保留到进程退出），安全。临时文件必须落在与目标二进制**同一文件系统**，否则 rename 跨分区报 EXDEV。
- 依赖 systemd unit 含 `Restart=always`（长期运行的 agent 本就该有；部署时确认/补齐）。
- SSH 手动兜底仅用于守护脚本本身也失效的病态情况（机器在局域网，可达）。

---

## 8. 错误处理（一律 fail-closed）

| 情况 | 行为 |
|---|---|
| 建任务时缺 / 坏 sha256 或签名 | 400，任务不创建 |
| agent 未配置 OTA 公钥 | OTA 关闭，忽略所有任务 |
| 下载失败 / sha256 不符 / 验签失败 / 版本降级 | **替换前**中止，`report failed`，继续运行旧版 |
| 新版起来但健康检查超时 | 自动 `.bak` 还原，`report rolled_back` |
| 新版根本起不来 | ExecStartPre 守护按 attempts 阈值还原 `.bak`；极端情况下 SSH 兜底 |
| 状态上报时云端不可达 | 上报入队/重试（复用 agent 现有事件队列语义），不阻塞回滚 |

---

## 9. 安全考量

- **私钥隔离**：私钥只在离线机器，API/分发服务器不可读（§3、§4）。
- **公钥固定**：验签公钥 pin 在 agent，不动态获取（§3）。
- **绑定版本+字节**：规范化消息防调包/防版本冒充（§6）。
- **防降级**：拒绝 ≤ 当前版本（§6），避免重放旧的合法签名固件。
- **密钥轮换**：`--ota-pubkey` 接受多公钥，可"先加新公钥、再切签名私钥、最后撤旧公钥"平滑轮换。
- **fail-closed**：任何不确定状态都不刷写、保旧版（§8）。

---

## 10. 测试策略

### 单元测试（主体无需硬件）
- `ota-sign`：签→验往返；篡改固件字节 / 改 version → 验签失败。
- 服务端 `CreateOTATask`：空 sha256 / 空签名 → 必填错误；合法 → 通过；格式非法 → 既有错误。
- agent 验签：正确签名通过；错误公钥 / 篡改字节 / 版本降级 → 拒绝，**且二进制未被触碰**。
- agent apply / 回滚：临时文件 → 备份 → rename 原子性（用临时目录）；`.bak` 还原正确。

### 真机集成测试（Orange Pi，可选但具备条件）
- 发布一个真签名的 agent 更新 → 观察自更新 + `report success`。
- 故意发布一个坏二进制（起不来）→ 观察 ExecStartPre 守护自动回滚 + `report rolled_back`。

---

## 11. 改动文件清单

**新增**
- `api/cmd/ota-sign/main.go` — 离线签名 + 生成密钥 CLI
- `api/cmd/gateway-agent/ota.go` — OTA 执行器（下载/验签/替换/回滚/上报）
- `api/cmd/gateway-agent/ota_test.go` — 执行器与验签单测
- `mistypass-ota-guard.sh` — ExecStartPre 回滚守护脚本（与 systemd unit 同处，具体路径实施期定）+ systemd unit 片段
- `docs/ota-signing-runbook.md` — 签名操作手册 + 密钥托管 + 轮换 + systemd 配置

**修改**
- `api/internal/modules/gateway/service.go` — `CreateOTATask` 改强制；新增两个 `...Required` 错误
- `api/internal/modules/gateway/service_test.go` — 强制校验测试
- `api/internal/http/routes_gateway_management.go` — 新错误映射为 400
- `api/cmd/gateway-agent/main.go` — 新增 `--ota-pubkey` flag；接入版本号 ldflags
- `api/cmd/gateway-agent/agent.go` — `pullConfig` 接入 OTA 执行器；启动时健康确认/回滚

---

## 12. 待实施期确认的小项（非阻塞）
- gateway-agent 当前是否已有编译期版本变量；无则补 `-ldflags -X main.version`。
- 生产/staging 的 systemd unit 是否含 `Restart=always`；无则补。
- agent ARM64 构建产物路径与 firmware 托管位置（可复用现有 Upload 签名 URL 或任意静态托管）。

---

## 13. 工作量估计
约 1–1.5 天。其中纯密码学（签名/验签）零风险；唯一需要在真机上仔细测的是"原子替换 + 失败回滚"，避免刷一半把网关刷砖。
