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

### 3.1 候选方案对比

| 维度 | HTTPS :443 (pull/push) | WSS :443 (长连接) | MQTT over WSS :443 | NATS |
|---|---|---|---|---|
| 实时性 | 秒级（取决 poll 间隔） | 毫秒级 | 毫秒级 | 毫秒级 |
| 防火墙穿透 | **最好**（443 出站） | **好**（443，但 WSS 可能被 DPI 拦） | **好**（同 WSS） | **差**（需开 4222） |
| 离线能力 | 本地策略缓存 + 事件队列 + 幂等同步 | 同左（离线能力不来自传输协议） | 同左 | 同左 |
| 设备端复杂度 | **最低**（HTTP client） | 中（需 WebSocket 管理） | 中（需 MQTT + WS 库） | 中（需 NATS client） |
| 运维部署 | **无额外组件** | 无额外组件 | 需部署 MQTT broker | 需部署 NATS server |
| 生产就绪 | **最快** | 快 | 需 broker HA + ACL + 限流 + 审计 | 仅适合内部 |

**关键澄清**：
- 门禁的**离线能力**来自 Gateway 本地策略缓存、事件持久队列、幂等同步机制，**不来自 MQTT QoS 或 retained message**
- MQTT QoS 只保证 broker↔client 的投递语义，**不保证业务 exactly-once**。开门命令必须有 command_id、过期时间、ack、去重、审计
- MQTT retained message **不能用于 unlock/lockdown 命令**（命令必须短 TTL、不可 retained）

### 3.2 不推荐的方案

| 方案 | 原因 |
|---|---|
| 993 端口 fallback | 政府/银行客户将"借 IMAP 端口跑非 IMAP 协议"视为安全绕行，深度包检测也可能拦截 |
| 自研 raw TLS 私有协议 | 需硬件/固件/安全芯片/运维全链路能力，Kisi 靠 Electric Imp 平台支撑，我们没有 |
| MQTT 作为唯一主协议 | 生产级 MQTT 需设备证书、topic ACL、多租户隔离、断线重放、broker HA、限流、证书吊销等，不应作为首批客户的入门门槛 |
| NATS 暴露给现场 Gateway | NATS 是云内部总线，不适合现场设备直连 |

---

## 4. 推荐架构

### 4.1 产品路线：先赢保守场景，再增强实时体验

**HTTPS-only 先赢政府、学校、工厂；WSS 再提升写字楼、园区的实时体验。不要为了技术漂亮把首批客户部署难度拉高。**

```
┌──────────────────────────────────────────────────┐
│               Mistyislet Cloud :443               │
│                                                    │
│  ┌──────────────────┐  ┌───────────────────────┐ │
│  │ HTTPS + mTLS     │  │ WSS + mTLS            │ │
│  │ 强制基础通道      │  │ 实时增强通道（可选）    │ │
│  │ 注册/配置/事件/OTA│  │ 远程开门/lockdown/状态 │ │
│  └────────┬─────────┘  └──────────┬────────────┘ │
│           │           内部         │              │
│           └──────── NATS ─────────┘              │
│                    :4222                          │
└──────────────────────────────────────────────────┘
                        │
                   全部 :443 出站
                   per-device client cert (mTLS)
                        │
              ┌─────────┴─────────┐
              │     Gateway       │
              │  mTLS 设备证书     │
              │  本地策略缓存      │
              │  事件持久队列      │
              └───────────────────┘
```

### 4.2 分阶段落地

| 阶段 | 通道 | 覆盖客户 | 能力 |
|---|---|---|---|
| **MVP / 首批客户** | HTTPS :443 + mTLS（强制） | 政府、学校、工厂、所有场景 | 配置快照、事件队列、OTA、证书轮换、本地判定 |
| **实时增强** | + WSS :443 + mTLS（可选） | 写字楼、园区 | 远程实时开门、lockdown、在线状态推送 |
| **仅内部** | NATS :4222 | 开发调试 | Gateway Simulator、微服务间事件路由 |

WSS 实现方式可以是自定义 WebSocket，也可以是 MQTT over WebSocket — 不在第一阶段做决定。

### 4.3 设备认证：per-device client certificate (mTLS)

```
1. Gateway 首次注册：
   POST /api/v1/gateway/register (一次性 bootstrap token)
   → Cloud 签发 per-device client certificate
   → Gateway 保存证书到安全存储

2. 后续所有通信：
   HTTPS/WSS 请求携带 client certificate (mTLS)
   Cloud 验证证书 → 解析 gateway_id + tenant_id
   → 无需额外 token

3. 证书轮换：
   Cloud 在证书到期前通过 config/pull 下发新证书
   Gateway 平滑切换

4. 证书撤销：
   Gateway 被删除时，Cloud 即时撤销证书 (CRL/OCSP)
```

### 4.4 学 Kisi 的安全模型

| 从 Kisi 学到的 | 在 Mistyislet 怎么做 |
|---|---|
| mTLS per-device 证书 | 自管 CA 签发 per-device cert |
| 仅出站连接 | Gateway 主动连 Cloud :443，不开入站端口 |
| per-device 密钥隔离 | 每台设备独立证书和密钥 |
| 本地判定优先 | Gateway 缓存 access_rules，离线可判定（已实现） |
| 端口友好 | 443-only，不借用其他端口 |

| 不从 Kisi 学的 | 原因 |
|---|---|
| Electric Imp 私有协议 | 绑定平台，我们用开放标准 |
| 993 端口伪装 | 客户可能视为安全绕行 |
| raw TLS 自研协议 | 需全链路硬件/固件能力支撑 |

### 4.5 与 Kisi 架构的对比

| 维度 | Kisi | Mistyislet |
|---|---|---|
| 传输协议 | Electric Imp 私有二进制协议 | **HTTPS + WSS**（开放标准） |
| 端口 | 31314（初始连接）→ 993（fallback）→ 443/80（firmware/API） | **443-only** |
| 设备认证 | PKI mTLS（Electric Imp 管理） | **PKI mTLS（自管 CA）** |
| 实时推送 | 私有 TLS 持久连接 | **WSS 持久连接**（可选增强） |
| 强制基础通道 | 私有 TLS | **HTTPS**（任何网络都能通） |
| 离线判定 | 本地缓存策略 | **本地 access_rules 缓存**（已实现） |
| OTA | RSA+AES :443/80 | **HTTPS :443**（待实现） |
| 平台绑定 | **绑定 Electric Imp** | **无绑定** |

### 4.6 印尼各场景推荐配置

| 场景 | 阶段一（HTTPS） | 阶段二（+WSS） |
|---|---|---|
| **政府机构** | HTTPS pull/push（唯一要求：443 出站） | 按需加 WSS |
| **学校** | HTTPS pull/push | 通常不需要 WSS |
| **工厂** | HTTPS pull/push（OT/IT 隔离） | 按需加 WSS |
| **写字楼** | HTTPS pull/push | **加 WSS（远程开门/访客体验）** |
| **产业园区** | HTTPS pull/push | **加 WSS（多栋实时管理）** |

---

## 5. 与当前实现的过渡计划

### 5.1 当前状态

- ✅ NATS 内部总线已运行（publisher/subscriber/simulator）
- ✅ Gateway Simulator 通过 NATS 收发命令
- ✅ HTTPS Gateway API 已有（register/config/events/verify/access-rules）
- ✅ Access rule 缓存包生成器已实现
- ✅ 凭证验证 API 已实现
- ❌ mTLS 设备证书签发/验证未实现
- ❌ WSS 实时通道未实现

### 5.2 过渡步骤

| 阶段 | 任务 | 说明 |
|---|---|---|
| **现在** | 继续用 NATS 开发调试 + HTTPS Gateway API | 不影响现有代码 |
| **MVP 硬件联调** | 香橙派跑 Gateway 程序 → HTTPS :443 连 Cloud | 用已有 `/gateway/*` API |
| **安全加固** | 实现 mTLS：CA 签发 + 设备证书 + 证书轮换 | 关键生产要求 |
| **实时增强** | 加 WSS :443 通道（写字楼/园区客户需要时） | MQTT over WSS 或自定义 WSS |
| **首批部署** | HTTPS-only，按客户要求加 WSS | 保守起步 |

### 5.3 工作量评估（修正）

| 组件 | 工作内容 | 预估 |
|---|---|---|
| HTTPS Gateway 已有 | register/config/events/verify 已可用 | **已完成** |
| mTLS 设备证书 | CA 签发、验证中间件、证书轮换、撤销 | 3-5 天 |
| WSS 实时通道 | WebSocket server + 命令推送 + 断线重连 | 2-3 天 |
| MQTT over WSS（如选用） | broker 部署 + topic ACL + 多租户隔离 + 限流 + 审计 | **5-10 天**（不是 3 天） |
| Gateway 端 HTTPS client | Go 程序跑在香橙派上 | 2-3 天 |

**注意**：MQTT 生产就绪远不止 3 天 — 设备证书、topic ACL、多租户隔离、断线重放、broker HA、审计、限流、证书吊销、OTA、现场网络排障都需要工程投入。

---

## 6. 结论

1. **HTTPS :443 是强制基础通道** — 所有客户（政府/学校/工厂/写字楼）的门禁配置、事件、OTA 都走它
2. **WSS :443 是实时增强通道（可选）** — 需要远程开门/lockdown 实时性的客户加上
3. **NATS 只做云内部总线** — 不暴露给现场 Gateway
4. **mTLS per-device cert 是核心安全模型** — 学 Kisi 的安全架构，不学它的私有传输
5. **MQTT 不是第一阶段主协议** — 如果需要实时通道，WSS 足够；如果要 MQTT，走 MQTT over WSS，但工程量远超 HTTPS-only
6. **先赢保守场景（HTTPS-only），再增强实时体验（+WSS）** — 不要为了技术漂亮把首批客户部署难度拉高

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
