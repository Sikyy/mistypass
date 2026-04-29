# 门禁通信协议架构评估报告

> 日期：2026-04-30
> 范围：印尼市场（政府、学校、机构、工厂、写字楼）
> 目标：确定 Mistyislet 产品的 Gateway ↔ Cloud 通信协议整体架构

---

## 1. 行业主流厂商通信架构分析

### 1.1 Kisi（Cloud-native 门禁，美国）

| 维度 | 实现方式 |
|---|---|
| Cloud 通信 | **TLS 1.2 持久连接**（基于 Electric Imp IoT 平台的自定义二进制协议），mutual auth + ephemeral key exchange + PKI 链验证 |
| 端口策略 | **TCP 31314**（主） → **TCP 993**（fallback，利用 IMAP 端口通常开放） → **TCP 443**（终极 fallback） |
| 连接方向 | **仅出站**（Controller 主动连 Cloud，不需要入站端口/防火墙打洞） |
| 本地通信 | **AES 加密 UDP**（端口 62435），Controller ↔ Reader 本地局域网，签名 + AES/HMAC 加密 |
| 离线能力 | Controller 本地缓存策略，离线可判定放行；设备自动同步本地存储到云端 |
| 远程开门 | **实时**（持久 TLS 连接允许 Cloud 即时下推命令，不是 polling） |
| OTA 更新 | 通过端口 443/80 下载，RSA 签名（HSM 托管密钥）+ AES 加密，每两周更新，停机 <10 秒 |
| 设备密钥 | per-device keys + AES-GCM-AEAD |
| 托管 | Google Cloud Platform，区域故障转移，热备数据库 |

**架构特点**：
- **不是 HTTPS polling，是 TLS 持久连接** — 类似 MQTT/WebSocket，但使用 Electric Imp 的私有协议
- 端口 fallback 策略巧妙：31314 → 993（IMAP 端口通常开放）→ 443（HTTPS 端口几乎必开）
- Controller 和 Reader 在同一局域网通过 AES-UDP 通信，Cloud 失联时走本地
- 去中心化设计：Reader 和 Controller 不需要直接布线连接，只需在同一局域网

### 1.2 Brivo（企业级云门禁，美国）

| 维度 | 实现方式 |
|---|---|
| Cloud 通信 | **HTTPS** + Ethernet/WiFi，蜂窝网络备份 |
| 硬件 | ACS6000 系列 IP 控制面板，每面板最多 30 个读卡器 |
| 架构 | 集中式控制面板 → 低压线缆 → 读卡器（传统布线） |
| API | REST API + Webhooks |

**架构特点**：传统面板+IP 化。面板通过以太网/WiFi 连云。

### 1.3 Salto（欧洲，无线锁为主）

| 维度 | 实现方式 |
|---|---|
| 通信 | **SVN（Salto Virtual Network）数据卡上协议** — 权限写在卡上，锁读卡获取权限 |
| 网络 | 锁不联网，权限通过"更新点"（在线锁或编程器）写入卡 |
| 离线 | 天然离线 — 锁靠电池，卡携带权限 |

**架构特点**：完全去中心化。锁不需要网络连接。

### 1.4 HID Mercury + Genetec Synergis（企业级传统）

| 维度 | 实现方式 |
|---|---|
| Controller ↔ Reader | **OSDP v2**（双向加密）或 Wiegand（兼容旧设备） |
| Controller ↔ Server | **TCP/IP**（私有协议或 HTTPS） |
| 云化 | Genetec Synergis Cloud Link 做网关桥接 |

**架构特点**：传统控制器 + OSDP/Wiegand 读卡器 + 服务端管理。

### 1.5 学术/开源 IoT 门禁方案

多项 IEEE 论文和开源项目使用 **MQTT** 做 ESP32/NodeMCU 门锁与云端的通信：
- ESP32 + MQTT + BLE + RFID（HiveMQ/EMQX 作为 broker）
- 适合小型单门/少量门场景
- 资源消耗低，嵌入式适配好

---

## 2. 印尼市场场景适用性分析

### 2.1 市场概况

- 印尼门禁市场 2024 年 **2.37 亿美元**，预计 2033 年达 **5.408 亿美元**（CAGR 9.6%）
- 移动渗透率 112%，2.66 亿移动用户 → 移动开门有基础
- 政府推 SPBE（电子政务系统），数字基础设施快速建设

### 2.2 各场景网络环境与需求

| 场景 | 网络环境 | 防火墙限制 | 延迟需求 | 离线需求 | 规模 |
|---|---|---|---|---|---|
| **政府机构** | 专网或受管网络，DNS 过滤，严格出站限制 | **高**（可能仅开放 443/80） | 低（不需实时远程开门） | **高**（网络不稳定） | 中（10-100门） |
| **学校** | 基础 WiFi/以太网，带宽有限 | **中**（管理不严格但不稳定） | 低 | **高** | 小-中（5-50门） |
| **大型机构**（银行、国企） | 企业级网络，SOC 监控，严格端口管控 | **高**（仅 443，深度包检测） | 中 | 中 | 大（50-500门） |
| **工厂** | 工业网络/独立 VLAN，OT 与 IT 分离 | **中-高**（OT 网络隔离，IT 审批端口） | 中（考勤打卡需实时） | **高**（车间网络差） | 中-大（20-200门） |
| **写字楼** | 商业宽带，物业管理网络 | **低-中**（物业可控） | **高**（远程开门、访客体验） | 中 | 中（10-100门） |
| **产业园区** | 多栋建筑，网络层级复杂 | **中** | 高 | 中 | 大（100-1000门） |

### 2.3 关键约束

1. **防火墙穿透是核心问题**：政府/大型机构/工厂的网络通常只开放 443（HTTPS），MQTT 的 1883/8883 和 NATS 的 4222 大概率被屏蔽
2. **网络稳定性差**：印尼很多场景（学校、工厂、政府大楼）网络中断频繁，离线能力是刚需
3. **IT 运维能力有限**：客户端不太可能帮你开端口或配 NATS/MQTT broker

---

## 3. 协议方案对比

### 3.1 四种候选方案

| 维度 | A: 纯 NATS | B: 纯 MQTT | C: HTTPS pull/push | D: TLS 持久连接（Kisi 模式） |
|---|---|---|---|---|
| 实时性 | 毫秒级 | 毫秒级 | 秒级（取决 poll 间隔） | **毫秒级** |
| 防火墙穿透 | **差**（4222） | **差**（1883/8883） | **好**（443） | **最好**（31314→993→443 fallback） |
| WebSocket 穿透 | 可以（WS 443） | 可以（WSS 443） | 不需要 | 不需要（原生 TLS） |
| 离线能力 | 需自行实现 | QoS 2 + retained | 天然支持 | 天然支持 |
| 设备端复杂度 | 中（Go client 3MB） | 低（C client <100KB） | 最低（HTTP） | 中（需实现连接管理） |
| ARM 嵌入式适配 | 好（Go 交叉编译） | **最好**（C/Python） | 最好 | 好（Go/C TLS） |
| 运维部署 | 需 NATS server | 需 MQTT broker | 无额外组件 | 无额外组件 |
| 行业成熟度 | 云原生领域 | **IoT 最成熟** | 企业级标准 | **Kisi 已验证** |
| 代表厂商 | 无门禁厂商 | 学术/IoT 门锁 | Brivo | **Kisi** |

### 3.2 MQTT over WebSocket 443 方案

如果选 MQTT，可以通过 **WebSocket 443 端口** 穿透防火墙：

```
Gateway → WSS://cloud:443/mqtt → MQTT Broker (EMQX)
```

EMQX（你 docker-compose 里已有）支持 MQTT over WebSocket on port 443。这样：
- ✅ 防火墙只需开 443（和 HTTPS 一样）
- ✅ IoT 生态最成熟
- ✅ QoS 保证消息可靠
- ❌ 需要部署 EMQX broker

### 3.3 NATS over WebSocket 443 方案

NATS 也支持 WebSocket：

```
Gateway → WSS://cloud:443/nats → NATS Server
```

- ✅ 防火墙穿透
- ✅ 性能最强（百万消息/秒）
- ✅ JetStream 持久化
- ❌ IoT 嵌入式库不如 MQTT 丰富

---

## 4. 推荐架构：分层双通道

```
┌───────────────────────────────────────────────────────┐
│                    Mistyislet Cloud                     │
│                                                         │
│  ┌──────────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │ REST API     │  │ NATS     │  │ MQTT Broker      │ │
│  │ (管理/配置)   │  │ (内部总线) │  │ (EMQX, 设备通信)  │ │
│  │ :443         │  │ :4222    │  │ WSS :443/mqtt    │ │
│  └──────┬───────┘  └────┬─────┘  └────────┬─────────┘ │
│         │               │                  │           │
│         │          ┌────┴─────┐            │           │
│         │          │ 内部桥接  │            │           │
│         │          │ MQTT↔NATS│            │           │
│         │          └──────────┘            │           │
└─────────┼──────────────────────────────────┼───────────┘
          │                                  │
     ┌────┴────────────┐            ┌────────┴──────────┐
     │ 场景 A           │            │ 场景 B             │
     │ 写字楼/园区       │            │ 政府/工厂/学校      │
     │ 网络条件好        │            │ 防火墙严格          │
     │                  │            │                    │
     │ Gateway          │            │ Gateway            │
     │ ├ MQTT over WSS  │            │ ├ HTTPS pull       │
     │ │ (实时推送)      │            │ │ (定时拉配置)       │
     │ └ 本地判定优先     │            │ ├ HTTPS push       │
     │                  │            │ │ (事件批量上报)      │
     └──────────────────┘            │ └ 本地判定优先       │
                                     └────────────────────┘
```

### 4.1 核心学习：Kisi 的安全模型

Kisi 真正值得学的不是它的私有传输协议（Electric Imp 平台绑定），而是：

1. **mTLS（双向证书认证）**：每台设备有独立的客户端证书，Cloud 和设备互相验证身份
2. **仅出站连接**：设备主动连 Cloud，不需要入站端口/防火墙打洞
3. **per-device 密钥**：AES-GCM-AEAD，设备级密钥隔离
4. **本地判定优先**：门禁放行不依赖 Cloud 实时 round-trip

**不应该学的**：
- 自研 raw TLS 私有协议（需要硬件/固件/安全芯片/运维全链路配套，当前阶段不现实）
- 多端口 fallback（31314→993→443 是 Electric Imp 平台的设计，不是通用方案）

### 4.2 架构决策：443-only + mTLS

**所有 Gateway ↔ Cloud 通信走 443 端口，用标准协议 + 设备证书认证。**

```
┌──────────────────────────────────────────────────┐
│                 Mistyislet Cloud :443              │
│                                                    │
│  ┌─────────────────┐  ┌────────────────────────┐ │
│  │ HTTPS + mTLS    │  │ WSS (MQTT) + mTLS      │ │
│  │ 配置/注册/事件   │  │ 实时命令/状态/推送      │ │
│  └────────┬────────┘  └───────────┬────────────┘ │
│           │          内部          │              │
│           └──────── NATS ─────────┘              │
│                    :4222                          │
└──────────────────────────────────────────────────┘
                        │
                   全部 :443 出站
                        │
              ┌─────────┴─────────┐
              │     Gateway       │
              │  per-device cert  │
              │  本地 access rule  │
              └───────────────────┘
```

### 4.3 三层设计

| 层 | 协议 | 用途 | 端口 |
|---|---|---|---|
| **管理层** | HTTPS + mTLS | 设备注册、拉配置、事件上报、OTA、证书轮换 | 443 |
| **实时层** | WSS + mTLS（MQTT over WebSocket） | 远程开门、lockdown、在线状态推送 | 443 |
| **内部总线层** | NATS | API 微服务间通信、事件路由、开发调试 | 4222（内部，不暴露） |

**为什么 443-only**：
- 印尼任何网络（政府/工厂/学校/写字楼）都开放 443
- 不需要客户 IT 配合开额外端口
- HTTPS 和 WSS 共用 443，由 path 区分（`/api/v1/gateway/*` vs `/ws/gateway`）
- 与企业代理/WAF 兼容

### 4.4 Gateway 通信模式

| 场景 | 连接方式 | 说明 |
|---|---|---|
| **正常在线** | WSS :443 (MQTT over WebSocket) | 持久连接，实时命令推送 + 事件上报 |
| **WSS 不可用** | HTTPS :443 pull/push | 定时拉配置 + 批量上报事件 |
| **开发调试** | NATS :4222 | Gateway Simulator 直连（仅开发环境） |

Gateway 启动时优先建立 WSS 持久连接；如果 WebSocket 被代理/防火墙阻断，自动降级为 HTTPS pull/push。

### 4.5 设备认证：per-device client certificate

```
1. Gateway 首次注册：
   POST /api/v1/gateway/register (bootstrap token)
   → Cloud 签发 per-device client certificate
   → Gateway 保存证书到安全存储

2. 后续通信：
   所有 HTTPS/WSS 请求携带 client certificate (mTLS)
   Cloud 验证证书 → 解析 gateway_id + tenant_id
   → 无需额外 token 交换
```

### 4.6 与 Kisi 架构的对比

| 维度 | Kisi | Mistyislet |
|---|---|---|
| 传输协议 | Electric Imp 私有二进制协议 | **HTTPS + WSS (MQTT)**（开放标准） |
| 端口 | 31314（主）→ 993 → 443/80 | **443-only** |
| 设备认证 | PKI mTLS（Electric Imp 管理） | **PKI mTLS（自管 CA）** |
| 实时推送 | TLS 持久连接 | **WSS 持久连接**（同等实时性） |
| 本地通信 | AES-UDP :62435 | **待定** |
| 离线判定 | 本地缓存策略 | **本地 access_rules 缓存**（已实现） |
| OTA | RSA+AES :443/80 | **HTTPS :443**（待实现） |
| 平台绑定 | **绑定 Electric Imp** | **无绑定，开放标准** |

**核心策略**：学 Kisi 的安全模型（mTLS + per-device cert + 仅出站 + 本地判定），不学它的私有传输（Electric Imp 绑定）。用 HTTPS/WSS 开放标准达到同等效果。

### 4.4 印尼各场景推荐配置

| 场景 | 推荐模式 | 原因 |
|---|---|---|
| **写字楼** | MQTT 模式 | 网络条件好，远程开门需实时 |
| **产业园区** | MQTT 模式 | 多栋建筑，实时管理需求高 |
| **工厂** | HTTPS 模式 | OT/IT 网络隔离，开端口困难 |
| **政府机构** | HTTPS 模式 | 严格防火墙，仅开 443 |
| **学校** | HTTPS 模式 | 网络不稳定，离线需求高 |
| **大型机构** | MQTT 模式 (WSS 443) | 企业级网络支持 WSS，且需实时性 |

---

## 5. 与当前实现的过渡计划

### 5.1 当前状态

- ✅ NATS 内部总线已运行（publisher/subscriber/simulator）
- ✅ Gateway Simulator 通过 NATS 收发命令
- ✅ HTTPS Gateway API 已有（register/config/events/verify）
- ❌ MQTT 设备通信层未实现
- ❌ EMQX broker 已在 docker-compose 但未接入业务

### 5.2 过渡步骤

| 阶段 | 任务 | 影响 |
|---|---|---|
| **现在** | 继续用 NATS 开发调试 + HTTPS Gateway API | 不影响现有代码 |
| **MVP 验证** | 香橙派跑 Gateway 程序 → HTTPS 模式连 Cloud | 用已有 API |
| **生产准备** | 加 MQTT over WSS 通道，EMQX 桥接到 NATS | 新增 MQTT adapter |
| **部署** | 按客户网络环境选模式（MQTT/HTTPS/NATS） | 配置级切换 |

### 5.3 代码改动量

| 组件 | 改动 | 预估 |
|---|---|---|
| EMQX 配置（WSS 443 + auth） | docker-compose + EMQX config | 0.5天 |
| MQTT → NATS 桥接（EMQX 内置或自写 adapter） | 新增 MQTT subscriber → publish to NATS | 1天 |
| Gateway 端 MQTT client | 新增模式，复用 NATS 的 command/event schema | 1天 |
| 模式切换配置 | `COMM_MODE` 环境变量 | 0.5天 |

**总计约 3 天**，且不影响现有 NATS + HTTPS 代码。

---

## 6. 结论

1. **不需要替换 NATS**——它作为内部总线继续发挥价值
2. **设备通信主协议选 MQTT over WebSocket (443)**——IoT 生态最成熟，防火墙穿透最好
3. **保留 HTTPS pull/push 作为 fallback**——政府/工厂等严格网络环境
4. **三种模式配置级切换**——一套 Gateway 固件适配所有场景
5. **MVP 阶段继续用 NATS**——等生产部署前再加 MQTT 层，代码改动小

---

## 来源

- [Kisi Security Architecture](https://www.getkisi.com/security)
- [Kisi System Architecture](https://docs.kisi.io/platform/kisi_system_architecture/)
- [Kisi Controller Pro 2](https://docs.kisi.io/access_control/hardware/controllers/kisi_controller_pro_2/)
- [NATS vs MQTT Comparison (i-flow)](https://i-flow.io/en/ressources/nats-vs-mqtt-comparison-for-the-uns-application/)
- [NATS Official Documentation](https://docs.nats.io/)
- [EMQX NATS Gateway](https://www.emqx.com/en/blog/emqx-nats-gateway)
- [HiveMQ: Why MQTT Outperforms NATS](https://www.hivemq.com/blog/building-unified-namespace-why-mqtt-outperforms-nats/)
- [Indonesia Access Control Market (Astute Analytica)](https://www.globenewswire.com/news-release/2025/02/05/3020939/0/en/Indonesia-Access-Control-Solutions-Market-to-Hit-Valuation-of-US-540-8-Million-by-2033-Astute-Analytica.html)
- [Indonesia Digital Infrastructure (CGD)](https://cgd.ibc-institute.id/building-a-connected-indonesia-through-digital-infrastructure-and-connectivity/)
- [IEEE: MQTT Smart Door Access System](https://ieeexplore.ieee.org/document/10425368)
- [Brivo vs Openpath vs Coram Comparison](https://www.coram.ai/post/brivo-vs-openpath)
- [Genetec Synergis Cloud Link + Mercury OSDP](https://synergis-cloudlink-help.genetec.com/en/EN/SSW/T_SSW_AddingOSDPv2ReadersToMercuryController.html)
