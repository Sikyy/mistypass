# MistyPass Hardware Integration Guide

Current-generation hardware chain and next-generation hardware selection plan.

**Version:** 1.1
**Date:** 2026-04-30

---

## Table of Contents

0. [Live Test Record (2026-04-30)](#0-live-test-record-2026-04-30)

1. [Current Hardware Architecture](#1-current-hardware-architecture)
2. [Current-Gen Wiring Diagram](#2-current-gen-wiring-diagram)
3. [Component Details (Current Gen)](#3-component-details-current-gen)
4. [Reader Integration (Missing Piece)](#4-reader-integration-missing-piece)
5. [Development Testing Without Hardware](#5-development-testing-without-hardware)
6. [Next-Gen Hardware Selection Plan](#6-next-gen-hardware-selection-plan)

---

## 1. Current Hardware Architecture

---

## 0. Live Test Record (2026-04-30)

End-to-end test using ACS WalletMate II + macOS development machine. No lock hardware — relay in dry-run mode.

### 0.1 Test Environment

| Component | Detail |
|-----------|--------|
| Gateway Host | MacBook Air M1, macOS 15.7.4 |
| NFC Reader | ACS WalletMate II (ACR1552U-MW), USB, Serial: RR650-000225 |
| Cloud API | `go run ./cmd/api` on localhost:8080 |
| Gateway Agent | `go run ./cmd/gateway-agent` on same machine |
| Relay | DryRunRelay (no physical lock) |
| Test Card | Contactless bank card (Visa/Mastercard payWave) |

### 0.2 System Startup

**Terminal 1 — Cloud API:**
```bash
GATEWAY_BOOTSTRAP_TOKEN=mistypass-dev-bootstrap-local-only-20260424 \
ENABLE_DEMO_USERS=true \
go run ./cmd/api
```
Result: `mistypass api listening addr=:8080`

**Terminal 2 — Gateway Agent:**
```bash
go run ./cmd/gateway-agent \
  -api http://localhost:8080 \
  -gateway gw_demo_001 \
  -tenant tenant_demo_jakarta \
  -token "mistypass-dev-bootstrap-local-only-20260424" \
  -token-file /tmp/mistypass-dev-token \
  -reader-lock-id door_jkt_001
```

Result:
```
config pulled  version=authz-f8c88936...  access_rules=2
PC/SC readers detected (3):
  [0] ACS WalletMate II Dual Reader 02    ← Apple VAS interface
  [1] ACS WalletMate II Dual Reader 01    ← Google Smart Tap interface
  [2] ACS WalletMate II Dual Reader       ← Generic NFC interface (ISO 14443)
Reader:   PC/SC NFC → door_jkt_001
```

### 0.3 Reader Interface Discovery

WalletMate II exposes 3 logical PC/SC interfaces via a single USB connection:

| Interface | Name | Purpose | GET UID Response |
|-----------|------|---------|-----------------|
| [0] | `ACS WalletMate II Dual Reader 02` | Apple VAS (ECP 2.0) | `SW=6E00` (CLA not supported) |
| [1] | `ACS WalletMate II Dual Reader 01` | Google Smart Tap | `SW=6E00` (CLA not supported) |
| [2] | `ACS WalletMate II Dual Reader` | Generic NFC (ISO 14443) | `SW=9000` (success) + UID |

The `GET UID` APDU command (`FF CA 00 00 00`) only works on interface [2] (generic NFC).
Interfaces [0] and [1] require vendor-specific APDU commands for Apple VAS / Google Smart Tap protocols, and will be integrated after obtaining the respective API credentials from Apple and Google.

The `reader.go` driver automatically tries all 3 interfaces and uses the first one that returns a valid UID.

### 0.4 Card Tap Test

**Test 1 — Bank card (Visa payWave):**
```
NFC card detected  uid=23E25A3C  lock_id=door_jkt_001
ACCESS DENIED  (credential_not_found — UID not registered)
```

**Test 2 — Same bank card, second tap:**
```
NFC card detected  uid=A362DABC  lock_id=door_jkt_001
ACCESS DENIED  (credential_not_found — UID not registered)
```

**Finding: Bank cards use random UID.** Modern Visa/Mastercard contactless cards generate a different random UID (NFCID1) on each activation to prevent tracking. This is a privacy feature defined in EMV Contactless specification.

**Conclusion:** Bank cards cannot be used for UID-based door access. Fixed-UID cards required:
- MIFARE Classic / DESFire (door access cards)
- NTAG213/215/216 (NFC stickers)
- Transit cards (public transport)

### 0.5 Credential Registration & Verification (Cloud API)

Registration was tested via API calls with the first UID (`23E25A3C`, before discovering the random-UID issue):

**Step 1 — Register physical card to inventory:**
```bash
POST /api/v1/wallet/physical-card-inventory
Body: {"tenant_id":"tenant_demo_jakarta","uid":"23E25A3C","card_number":"SIKY-001"}
Response: 201 Created, id=wpci_9584c2253f08, status=available
```

**Step 2 — Issue pass to user:**
```bash
POST /api/v1/wallet/passes/issue
Body: {"tenant_id":"tenant_demo_jakarta","template_id":"wpt_employee_demo",
       "target_type":"user","target_id":"usr_1001"}
Response: 201 Created, id=wps_258340d66a01, status=issued
```

**Step 3 — Bind card to pass via physical card task:**
```bash
POST /api/v1/wallet/physical-card-tasks
Body: {"tenant_id":"tenant_demo_jakarta","pass_id":"wps_258340d66a01",
       "task_type":"issue","card_number":"SIKY-001","inventory_id":"wpci_9584c2253f08"}
Response: 201 Created, task queued
```

**Step 4 — Activate pass:**
```bash
PATCH /api/v1/wallet/passes/wps_258340d66a01/activate?tenant_id=tenant_demo_jakarta
Response: 200 OK, status=active, activated_at=2026-04-30T12:50:23Z
```

**Step 5 — Verify credential:**
```bash
POST /api/v1/verify-credential
Body: {"gateway_id":"gw_demo_001","lock_id":"door_jkt_001",
       "tenant_id":"tenant_demo_jakarta",
       "credential_type":"nfc_uid","credential_data":"23E25A3C"}
Response:
{
  "decision": "allow",
  "reason": "access_granted",
  "user_id": "usr_1001",
  "user_name": "Andri Pratama",
  "user_email": "andri.pratama@mistypass.local",
  "group_name": "Common Office Access",
  "lock_id": "door_jkt_001"
}
```

### 0.6 Full Chain Verified

```
Card Tap → WalletMate II → PC/SC (USB) → reader.go → HandleCredentialPresented
  → VerifyCredential (local cache) → match access rule → Relay.Unlock (DryRun)
  → Queue event → Push to Cloud (every 10s)

Card Registration:
  Admin API → physical-card-inventory → issue pass → physical-card-task
  → activate pass → config pull (30s) → Gateway cache updated
```

| Link | Verified | Result |
|------|----------|--------|
| WalletMate II USB detection | Pass | 3 interfaces detected |
| NFC card UID read (ISO 14443) | Pass | UID read successfully |
| Apple VAS / Google Smart Tap | Not tested | Requires API credentials |
| Cloud API credential verification | Pass | allow/deny correct |
| Gateway local cache decision | Pass | Rules synced in 30s |
| Relay unlock (DryRun) | Pass | Logs "relay ON/OFF" |
| Event upload to Cloud | Pass | Events pushed every 10s |
| Credential revocation propagation | Not tested | Expected: deny within 30s |
| Random UID detection | Discovered | Bank cards incompatible |

### 0.7 Security Observations from Live Test

1. **Random UID cards (bank cards) are correctly rejected** — each tap produces a different UID, so the credential will never match a registered entry. This is a secure-by-default behavior.

2. **Apple VAS / Google Smart Tap interfaces return `SW=6E00`** — the reader correctly refuses generic APDU on wallet-specific interfaces. These require vendor-specific initialization (Apple VAS merchant key, Google Smart Tap collector ID) which will be configured via the ACS Configuration Tool after obtaining API access.

3. **WalletMate II Secure Element** — the reader stores Apple VAS and Google Smart Tap private keys in an on-device Secure Element. This means wallet credential decryption happens on the reader, not on the gateway. The gateway only receives the decrypted pass identifier.

4. **PC/SC context lifecycle** — current implementation creates/releases a PC/SC context on every poll cycle (300ms). For production, this should be optimized to maintain a persistent context and use `SCardGetStatusChange` for event-driven card detection instead of polling.

5. **3-second debounce** — prevents the same card from triggering multiple unlock events. This is important for physical relay control (prevents relay flutter).

```
                         Internet (HTTPS/TLS)
                              |
                              |
                    +-------------------+
                    |   Cloud API       |
                    |   (Go Server)     |
                    +-------------------+
                              |
                              | HTTPS + Device Token
                              | + TLS Certificate Pinning
                              |
              +===============================+
              |       Gateway Agent           |
              |   (Orange Pi Zero3 / RPi)     |
              |                               |
              |  +----------+  +-----------+  |
              |  | Config   |  | Relay     |  |
              |  | Cache    |  | Driver    |  |
              |  | (rules)  |  | (GPIO/    |  |
              |  |          |  |  RS485)   |  |
              |  +----------+  +-----------+  |
              |       |              |        |
              +===============================+
                      |              |
           +----------+         +---+---+
           |                    |       |
     +-----------+        +--------+  +--------+
     | NFC/RFID  |        | Relay  |  | 电锁   |
     | Reader    |        | Module |  | (门)   |
     | (读卡器)  |        | (继电器)|  |        |
     +-----------+        +--------+  +--------+
           |                    |          |
     用户刷卡              12V/24V DC    开门/关门
```

**当前已实现的链路：**

| 链路 | 状态 | 代码 |
|------|------|------|
| Cloud API ↔ Gateway Agent | 已实现 | `agent.go` (config pull, heartbeat, event push) |
| Gateway Agent → Relay → 电锁 | 已实现 | `relay.go` (GPIO + RS485 Modbus) |
| NFC Reader → Gateway Agent | **已实现** | `reader.go` (PC/SC, 支持 WalletMate II) |
| stdin 模拟读卡 | 已实现 (测试用) | `input.go` |

---

## 2. Current-Gen Wiring Diagram

### 方案 A: GPIO 直接控制继电器 (最简)

```
Orange Pi Zero3                    继电器模块                    电锁
+-------------+                   +----------+                +------+
|         PC9 |---[GPIO 73]------>| IN       |                |      |
|         (H) |                   |          |---[COM]------->| +    |
|         3.3V|---[VCC]---------->| VCC      |---[NO]-------->| -    |
|         GND |---[GND]---------->| GND      |                |      |
+-------------+                   +----------+                +------+
                                       |                        |
                                  [12V/24V DC 电源]              |
                                       +------------------------+
```

**接线说明：**
- GPIO 73 (PC9) → 继电器 IN 引脚 (低电平触发)
- Orange Pi 3.3V → 继电器 VCC
- Orange Pi GND → 继电器 GND
- 继电器 COM → 电锁正极
- 继电器 NO (常开) → 电源正极
- 电源负极 → 电锁负极

**代码配置：**
```bash
./gateway-agent -relay-gpio 73 -unlock-duration 5s
```

**对应代码 (`relay.go:40-94`)：**
```
初始化: export GPIO → 设置 direction=out → 拉高 (继电器断开)
开门:   拉低 GPIO → 继电器吸合 → 电锁通电
关门:   延时后拉高 GPIO → 继电器断开 → 电锁断电 (弹簧复位)
```

### 方案 B: RS485 Modbus 控制继电器 (多门/远距离)

```
Orange Pi Zero3        USB转RS485          RS485 继电器模块       电锁
+-------------+       +---------+         +-------------+      +------+
|         USB |------>| USB     |         |             |      |      |
|             |       | RS485   |---A+--->| A+          |      |      |
|             |       | 转换器  |---B---->| B-    COM   |----->| +    |
+-------------+       +---------+         |       NO    |----->| -    |
                                          +-------------+      +------+
                                               |                  |
                                          [12V/24V DC 电源]        |
                                               +------------------+
```

**代码配置：**
```bash
./gateway-agent -relay-rs485 /dev/ttyUSB0 -unlock-duration 5s
```

**Modbus 命令 (`relay.go:112-142`)：**
```
开门: [0x01, 0x05, 0x00, 0x00, 0xFF, 0x00, 0x8C, 0x3A]
      地址=0x01  功能=写单线圈  寄存器=0x0000  值=ON  CRC16
关门: [0x01, 0x05, 0x00, 0x00, 0x00, 0x00, 0xCD, 0xCA]
      地址=0x01  功能=写单线圈  寄存器=0x0000  值=OFF  CRC16
```

---

## 3. Component Details (Current Gen)

### 3.1 Gateway 主板

| 参数 | Orange Pi Zero3 | Raspberry Pi 4B |
|------|----------------|-----------------|
| SoC | Allwinner H618 (Cortex-A53) | BCM2711 (Cortex-A72) |
| RAM | 1/2/4 GB | 2/4/8 GB |
| GPIO | 26-pin header | 40-pin header |
| 网络 | GbE + WiFi | GbE + WiFi |
| USB | 1x USB 3.0, 2x USB 2.0 | 2x USB 3.0, 2x USB 2.0 |
| 尺寸 | 65x30mm | 85x56mm |
| 功耗 | ~3W | ~5W |
| 价格 | ~$20 | ~$55 |
| TPM 支持 | 无 | 无 (需外接模块) |
| Secure Boot | 不支持原生 | 不支持原生 |

**代码中的引用：**
- GPIO 73 = PC9 引脚 (Orange Pi Zero3 header)
- 交叉编译: `GOOS=linux GOARCH=arm64 go build -o gateway-agent ./cmd/gateway-agent`

### 3.2 继电器模块

**GPIO 方案 — 单路继电器：**

| 参数 | 值 |
|------|-----|
| 工作电压 | 3.3V / 5V |
| 触发方式 | 低电平触发 (active-low) |
| 负载能力 | 10A 250VAC / 10A 30VDC |
| 推荐型号 | 单路光耦隔离继电器模块 |

**RS485 方案 — Modbus 继电器：**

| 参数 | 值 |
|------|-----|
| 通信协议 | Modbus RTU |
| 波特率 | 9600 (默认) |
| 地址 | 0x01 (可配置) |
| 推荐型号 | LC-tech RS485 1路/2路/4路继电器 |
| 通信距离 | 最远 1200m (RS485 标准) |
| USB 转换器 | CH340 / FT232 USB-RS485 |

### 3.3 电锁类型

| 类型 | 工作电压 | 工作方式 | 适用场景 |
|------|---------|---------|---------|
| 电磁锁 (磁力锁) | 12V DC | 通电上锁，断电开门 (fail-safe) | 消防要求场景 |
| 电插锁 | 12V DC | 通电上锁，断电开门 | 玻璃门/木门 |
| 电控锁 | 12V DC | 通电开门，断电上锁 (fail-secure) | 一般门禁 |
| 电子锁体 | 12V/24V DC | 电机驱动 | 防盗门 |

**注意：** 继电器接线方式取决于电锁类型：
- Fail-safe (断电开门): 使用 NC (常闭) 端
- Fail-secure (断电锁门): 使用 NO (常开) 端
- 当前代码默认使用 NO (常开) 端，适合 fail-secure 电锁

---

## 4. Reader Integration (Missing Piece)

当前 Gateway Agent 通过 stdin 模拟刷卡。生产环境需要集成真实的 NFC/RFID 读卡器。

### 4.1 集成方案对比

| 方案 | 协议 | 优势 | 劣势 | 推荐场景 |
|------|------|------|------|---------|
| **USB NFC Reader** | USB HID (keyboard emulation) | 即插即用，无需驱动 | 只能模拟键盘输入 | 快速原型/开发测试 |
| **USB NFC Reader** | USB Serial (ACR122U 等) | 灵活控制，读取完整 UID | 需要 libnfc/PC-SC 驱动 | 开发版推荐 |
| **Wiegand Reader** | Wiegand 26/34 bit | 行业标准，兼容性好 | 需要 GPIO 接线 | 传统门禁集成 |
| **OSDP Reader** | RS485 (OSDP v2) | 加密通信，双向，现代标准 | 实现复杂 | 下一代产品 |

### 4.2 已集成: ACS WalletMate II (ACR1552U-MW)

Gateway Agent 已内置 PC/SC 驱动 (`reader.go`)，支持 ACS WalletMate II 及所有 PC/SC 兼容读卡器。

```
WalletMate II (USB)                Gateway (Orange Pi / Mac)
+------------------+              +----------------------------+
|  [NFC 感应区]    |--- USB ----->|  USB Port                  |
|                  |              |                            |
|  ISO 14443 A/B   |              |  gateway-agent             |
|  Apple VAS       |              |    └── reader.go (PC/SC)   |
|  Google Smart Tap |              |         │                  |
+------------------+              |         ▼                  |
                                  |  HandleCredentialPresented |
                                  |    └── VerifyCredential    |
                                  |         └── Relay.Unlock   |
                                  +----------------------------+
```

**启动命令：**
```bash
./gateway-agent \
  -api https://your-cloud-api.com \
  -gateway gw_001 \
  -tenant tenant_001 \
  -token "bootstrap-token" \
  -reader-lock-id door_jkt_001 \
  -reader-poll 300ms
```

**工作流程：**
1. `reader.go` 通过 PC/SC 接口连接读卡器
2. 每 300ms 轮询卡片状态
3. 检测到卡片时发送 GET UID APDU (`FF CA 00 00 00`)
4. 读取 NFC UID (4/7/10 字节)，转为大写 hex
5. 调用 `agent.HandleCredentialPresented("nfc_uid", uid, lockID)`
6. 3 秒去抖动（同一张卡不重复触发）

**WalletMate II 额外能力（未来集成）：**

| 能力 | 协议 | 当前状态 | 用途 |
|------|------|---------|------|
| 读取 NFC UID | ISO 14443 GET UID APDU | **已实现** | 物理 NFC 卡门禁 |
| Apple Wallet 门禁 | Apple VAS (ECP 2.0 Access Control) | 待实现 | iPhone/Apple Watch 开门 |
| Google Wallet 门禁 | Google Smart Tap | 待实现 | Android 手机开门 |
| MIFARE 扇区读取 | MIFARE Classic/DESFire APDU | 待实现 | 加密卡数据验证 |

**代码：** `api/cmd/gateway-agent/reader.go`，依赖 `github.com/ebfe/scard` (PC/SC Go 绑定)

### 4.3 Wiegand Reader 集成 (未来)

```
Wiegand Reader                    Orange Pi GPIO
+-----------+                     +-------------+
| D0 (绿线) |-------------------->| GPIO Pin A  |
| D1 (白线) |-------------------->| GPIO Pin B  |
| GND       |-------------------->| GND         |
| VCC (12V) |<---- 12V 电源 ------|             |
+-----------+                     +-------------+
```

需要新增 Wiegand GPIO 监听代码：
- D0/D1 两线协议，脉冲宽度 ~50μs
- 26-bit: 1 parity + 8 facility + 16 card + 1 parity
- 34-bit: 1 parity + 16 facility + 16 card + 1 parity

---

## 5. Development Testing Without Hardware

### 5.1 完全软件模拟 (当前可用)

```bash
# 终端 1: Cloud API
cd api && go run ./cmd/api

# 终端 2: Gateway Agent (干跑模式)
cd api && go run ./cmd/gateway-agent \
  -api http://localhost:8081 \
  -gateway gw_demo_001 \
  -tenant tenant_demo_jakarta \
  -token "mistypass-dev-bootstrap-local-only-20260424" \
  -token-file /tmp/mistypass-dev-token

# 终端 2 输入模拟刷卡:
card> nfc_uid UID-1001 door_jkt_001
# → ACCESS GRANTED — DRY RUN: relay ON for 5s

card> nfc_uid UNKNOWN-UID door_jkt_001
# → ACCESS DENIED

card> rules
# → 显示所有缓存的访问规则
```

### 5.2 Gateway Simulator (NATS 集成测试)

```bash
# 终端 3: Gateway Simulator (模拟 NATS 命令接收)
cd api && go run ./cmd/gateway-simulator \
  -nats nats://localhost:4222 \
  -gateway gw_demo_001
# → 监听 Cloud 发来的 unlock/lockdown 命令
```

### 5.3 最小硬件测试 (无电锁)

只需 Orange Pi + 继电器模块 + LED，不需要电锁：

```
Orange Pi --- GPIO 73 ---> 继电器 IN
                           继电器 COM ---> LED 正极
                           继电器 NO ----> 3.3V
                           GND ----------> LED 负极 (串联电阻)
```

```bash
# Orange Pi 上运行
./gateway-agent \
  -api https://your-cloud-api.com \
  -gateway gw_test_001 \
  -tenant tenant_demo_jakarta \
  -token "your-bootstrap-token" \
  -relay-gpio 73

card> nfc_uid UID-1001 door_jkt_001
# → LED 亮 5 秒后灭 (模拟开门/关门)
```

---

## 6. Next-Gen Hardware Selection Plan

### 6.1 Selection Criteria

| 维度 | 当前 (Gen 1) | 下一代 (Gen 2) 要求 | 原因 |
|------|-------------|-------------------|------|
| **SoC** | Allwinner H618 (无安全特性) | 需要 ARM TrustZone 或 TPM 2.0 | Secure Boot, 密钥保护 |
| **Secure Boot** | 不支持 | 必须支持 | 防止固件篡改 |
| **密钥存储** | 文件系统 (0600) | Secure Enclave / TPM | 设备令牌和 mTLS 私钥不可导出 |
| **网络** | WiFi + GbE | GbE + 4G/LTE 备份 | 网络冗余 |
| **GPIO** | 通用 header | 专用 RS485 + Wiegand | 减少接线，提高可靠性 |
| **防篡改** | 无 | 外壳开盖检测 + 加速计 | 物理攻击告警 |
| **功耗** | ~3W | <5W (含 4G) | PoE 或电池备份 |
| **认证** | 无 | FCC/CE | 商业部署必需 |

### 6.2 Candidate SoC/Platform

| 平台 | SoC | TrustZone | TPM | Secure Boot | 价格 | 评估 |
|------|-----|-----------|-----|-------------|------|------|
| **NXP i.MX 8M Mini** | Cortex-A53 | Yes | 外接 | HAB (签名引导) | $25-35 | 推荐：成熟的安全引导链 |
| **STM32MP157** | Cortex-A7 + M4 | Yes | 外接 | OP-TEE 支持 | $15-20 | 性价比高，双核可做实时 IO |
| **Raspberry Pi CM4** | BCM2711 | 有限 | 外接 | 不原生支持 | $35-45 | 生态好但安全特性弱 |
| **Rockchip RK3568** | Cortex-A55 | Yes | 外接 | 支持 | $20-30 | 国产替代，性价比高 |
| **MediaTek Genio 350** | Cortex-A53 | Yes | 内置 SE | 支持 | $20-25 | IoT 专用，集成度高 |

**推荐：NXP i.MX 8M Mini** — 原因：
- HAB (High Assurance Boot) 是业界最成熟的签名引导方案
- 大量门禁/IoT 行业参考设计
- 长期供货承诺 (15+ 年)
- 丰富的 Linux BSP 和安全文档

### 6.3 Next-Gen Reader Strategy

| 阶段 | Reader | 协议 | 安全性 |
|------|--------|------|--------|
| Gen 1 (现在) | USB NFC (ACR122U) 或 HID keyboard | USB HID / Serial | 低 (明文 UID) |
| Gen 1.5 | Wiegand 读卡器 (HID iCLASS 等) | Wiegand 26/34 | 中 (无加密通道) |
| Gen 2 | OSDP v2 读卡器 | RS485 + AES-128 加密 | 高 (加密+双向通信) |
| Gen 3 | 自研读卡器 + BLE | OSDP v2 + BLE 5.0 | 最高 (challenge-response) |

### 6.4 Next-Gen System Diagram

```
                         Internet (HTTPS/TLS + cert pinning)
                              |
                    +-------------------+
                    |   Cloud API       |
                    +-------------------+
                              |
                              | HTTPS + mTLS (client certificate in TPM)
                              |
              +========================================+
              |         Gateway Agent (Gen 2)          |
              |        NXP i.MX 8M Mini                |
              |                                        |
              |  +----------+  +---------+  +-------+  |
              |  | TrustZone|  | OSDP v2 |  | 4G    |  |
              |  | Secure   |  | Driver  |  | Backup|  |
              |  | Storage  |  | (RS485) |  |       |  |
              |  +----------+  +---------+  +-------+  |
              |       |              |                  |
              |  +----------+  +---------+             |
              |  | Tamper   |  | Wiegand |             |
              |  | Detect   |  | Legacy  |             |
              |  | (GPIO)   |  | (GPIO)  |             |
              |  +----------+  +---------+             |
              +========================================+
                      |              |
              +-------+-------+     |
              |               |     |
        +-----------+  +-----------+  +--------+
        | OSDP v2   |  | Wiegand   |  | 电锁   |
        | Reader    |  | Reader    |  |        |
        | (加密通信) |  | (兼容旧设备)|  |        |
        +-----------+  +-----------+  +--------+
```

### 6.5 Implementation Timeline

```
Gen 1 (现在 - Q3 2026)
  ├── Cloud API + Web Admin: 已完成
  ├── Gateway Agent (DryRun + GPIO + RS485): 已完成
  ├── 安全链路 (TLS pinning, device token, cache TTL): 已完成
  ├── PC/SC NFC Reader 驱动 (WalletMate II): 已完成
  └── 待做: 接入继电器 + 电锁 → 完成 MVP 全链路演示

Gen 1.5 (Q3 - Q4 2026)
  ├── 集成 Wiegand 读卡器 (GPIO 驱动)
  ├── Google Wallet Corporate Badge API 对接
  ├── Apple Wallet 生产签名
  └── 首批客户部署

Gen 2 (2027 H1)
  ├── 切换到 NXP i.MX 8M Mini
  ├── 实现 Secure Boot (HAB)
  ├── mTLS (私钥存入 TrustZone)
  ├── OSDP v2 读卡器驱动
  ├── 防篡改检测 (外壳传感器)
  └── FCC/CE 认证

Gen 3 (2027 H2+)
  ├── 自研读卡器 PCB
  ├── BLE challenge-response
  ├── NFC 动态凭证 (非静态 UID)
  └── PoE 供电 + 电池备份
```
