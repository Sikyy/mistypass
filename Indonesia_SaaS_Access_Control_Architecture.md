# 印尼云 SaaS 门禁平台
## 完整技术架构与市场落地方案

> **文档版本：** 评估版 · 2025  
> **适用读者：** 技术评估、商业评估、投资人参考  
> **核心立场：** 不依赖 Apple/Google 支付生态，基于印尼市场实际约束，构建完全自主可控的云原生门禁平台

---

## 目录

1. [执行摘要](#一执行摘要)
2. [背景：主流方案在印尼的困境](#二背景主流方案在印尼的困境)
3. [印尼市场深度分析](#三印尼市场深度分析)
4. [核心设计决策](#四核心设计决策)
5. [整体平台架构](#五整体平台架构)
6. [凭据分层体系](#六凭据分层体系)
7. [核心组件详解](#七核心组件详解)
8. [安全架构](#八安全架构)
9. [凭据生命周期](#九凭据生命周期)
10. [设备集成网关](#十设备集成网关)
11. [核心 API 设计](#十一核心-api-设计)
12. [方案综合评估](#十二方案综合评估)
13. [客户分层与切入策略](#十三客户分层与切入策略)
14. [开发路线](#十四开发路线)
15. [附录](#附录)

---

## 一、执行摘要

| 维度 | 核心结论 |
|---|---|
| 市场规模 | 印尼门禁市场 2024 年 2.37 亿美元，CAGR 9.6%，2033 年预计达 5.41 亿美元 |
| 核心机会 | 海康/ZKTeco 存量硬件庞大但无云管理，市场空白明显 |
| 凭据安全 | BLE + Android Keystore，私钥永不离开硬件，安全等级接近 HID Seos |
| 凭据策略 | 三层凭据：BLE 移动凭据（高安全）+ DESFire 实体卡（中高安全）+ 动态二维码（访客） |
| 核心差异 | 不依赖 Apple/Google 生态，完全平台自主，兼容存量硬件，无需客户换设备 |
| 离线能力 | Controller 本地缓存，断网最长 72 小时正常开门 |
| 吊销速度 | Controller 在线时，管理员操作后 < 5 秒完全生效 |
| MVP 周期 | 3–4 个月：云端核心 + BLE 开门 + DESFire 卡管理 + 海康/ZKTeco 集成 |

---

## 二、背景：主流方案在印尼的困境

### 2.1 行业主流移动凭据协议

当前门禁行业认可度最高的移动凭据方案有三类：

**Apple VAS / ECP2（Apple Wallet）**
苹果在 iPhone 上通过 Secure Enclave 硬件芯片存储门禁凭据，安全等级极高。读头需通过 Apple ECP 2.0 认证，凭据通过 Apple 服务器的 server-to-server 接口下发至用户 Apple Wallet。这是目前体验最好、安全等级最高的移动门禁方案之一。

**Google Smart Tap（Google Wallet）**
谷歌的等价方案，通过手机内 Titan M 或 SE 芯片存储凭据，依托 Google 的 TSM（Trusted Service Manager）基础设施完成凭据下发和管理。

**HID Mobile Access（Seos 协议）**
HID 自研 Seos 协议，通过 BLE 或 NFC 传输，凭据存储在 HID 自有安全模块中，需配套 HID 认证读头（如 HID Signo），是企业门禁市场最受认可的移动凭据方案，有 CC EAL 安全认证背书。

### 2.2 三种方案在印尼的共同困境

| 方案 | 安全等级 | 印尼可用性 | 核心障碍 |
|---|---|---|---|
| Apple Wallet（VAS/ECP2） | ⭐⭐⭐⭐⭐ | ❌ | Apple Pay 在印尼不可用；ECP2 读头需苹果独立认证，授权周期以年计，费用高昂；即使完成认证，读头成本远超印尼市场接受范围 |
| Google Smart Tap | ⭐⭐⭐⭐⭐ | ⚠️ | Google Pay 在印尼支持受限；凭据下发强依赖 Google TSM 基础设施，平台自主性差；凭据吊销需经 Google 中转，实时性无法保证 |
| MIFARE2Go（NXP） | ⭐⭐⭐⭐⭐ | ❌ | 虽然安全等级高，但凭据下发与 Google Wallet 基础设施深度绑定，本质上仍受 Google 生态约束 |
| HID Seos 移动版 | ⭐⭐⭐⭐⭐ | ⚠️ | 技术上可用，但 HID 读头（如 Signo）价格高，完全绑定 HID 硬件生态；License 授权费用高；印尼市场主流是海康/ZKTeco，HID 装机量极少 |
| 裸 NFC HCE | ⭐⭐ | ✅ | 无生态依赖，但凭据存普通内存，静态凭据可被截获重放；即使加 Android Keystore，TEE 与主 CPU 共享硅片，理论上存在提取风险；iOS NFC HCE 历史上受限。**不建议作为主力方案** |

### 2.3 关于商用 BLE 读头的关键认知

这是选型过程中最容易踩的坑，需要明确说清楚：

**市面上所有主流高安全 BLE 读头，BLE 频道均锁定自家凭据生态。**

| 读头品牌 | BLE 是否开放给第三方 | NFC 是否支持 DESFire | OSDP 是否开放 |
|---|---|---|---|
| HID Signo | ❌ 仅支持 HID Mobile Access/Seos | ✅ 支持 DESFire EV1/EV2/EV3 | ✅ 开放标准 |
| Suprema BioEntry | ❌ 仅支持 BioStar 2 Mobile | ✅ 支持 DESFire | ✅ 开放标准 |
| LEGIC 读头 | ❌ 仅支持 LEGIC 协议 | ✅ 支持 DESFire | ✅ 开放标准 |
| Allegion | ❌ 仅支持自家生态 | ✅ 支持 DESFire | ✅ 开放标准 |

**结论：读头对外开放的是 OSDP（有线协议），不是 BLE（无线凭据频道）。** 若要实现自定义 BLE 凭据，BLE 必须在 Controller 侧而非读头侧实现。

Kisi 的做法是自研读头（Kisi Reader Pro），读头本身 IP 直连云端，手机 BLE 直接与 Kisi 自研读头通信，Controller 只负责控制继电器开门——这本质上就是"自研读头"路线，是 Phase 4 的目标，不是 Phase 1 的可行路径。

### 2.4 选型结论

基于以上约束，本平台采用以下技术路线：

- **不依赖 Apple/Google/HID 任何生态**
- **BLE 凭据在 Controller 侧实现**（手机与 Controller 通信，Controller 控制现有读头）
- **以 DESFire 实体卡作为高安全备选凭据**（兼容所有主流读头）
- **云端完全自主控制认证与吊销**
- **前期集成海康/ZKTeco 存量硬件，后期自研读头**

---

## 三、印尼市场深度分析

### 3.1 市场规模与增长

印尼门禁市场 2024 年市场规模 2.37 亿美元，预计以 9.6% 的年复合增长率增长，2033 年达到 5.41 亿美元。核心驱动因素：

- 印尼移动用户达 2.66 亿，移动渗透率 112%，移动化安全管理需求迫切
- 目前已有 380 万部手机被用作数字钥匙，市场对移动凭据的接受度已充分验证
- 62,000 套楼宇管理系统已与门禁系统集成，平台化管理需求明确
- 78,500 家组织将安全性列为访问管理的首要优先级
- 2024 年新进入市场的企业达 35 家，市场活跃度高但本土 SaaS 方案极为稀缺

### 3.2 存量硬件格局

印尼门禁硬件市场高度集中于两个品牌：

**海康威视（Hikvision）**：门禁 + 摄像 + 闸机 + 对讲机全产品线，是印尼写字楼、工厂、园区的绝对主流选择。海康提供 ISAPI（局域网 REST 接口）和 ISUP 5.0（云端透传）两种开放集成方式，第三方平台接入成本相对可控。

**ZKTeco**：中小企业和工厂场景的标配，覆盖指纹机、人脸机、RFID 读头等多种形态。ZKTeco Push SDK 支持设备主动向第三方服务器推送数据，集成方式简单，是印尼中小企业门禁市场占有率最高的品牌。

**Honeywell / Bosch / ASSA ABLOY**：在金融、医院、高端写字楼等高合规场景有一定份额，但总量远低于海康和 ZKTeco。

**关键机会**：绝大多数海康/ZKTeco 用户目前仍在用本地管理软件，数据靠 U 盘导出，无云端管理能力，无移动凭据，无实时审计。这是平台切入的核心机会点。

### 3.3 设备与网络特性

**手机设备分布：**

| 维度 | 数据 | 对方案的影响 |
|---|---|---|
| 安卓占有率 | 95%+ | Android 绝对优先，iOS 次之 |
| 主流品牌 | 三星、OPPO、vivo、小米、realme | 中端机为主，均支持 BLE 5.0 |
| BLE 支持率 | 2018 年后机型几乎全部支持 | BLE 方案设备覆盖最广 |
| NFC 支持率 | 主流中端机均支持 | NFC 可作辅助，覆盖率略低 |
| StrongBox 支持率 | 旗舰/高端机型支持，中低端降级至 TEE | 低端机通过缩短 TTL 补偿 |
| 基础款手机占比 | 工厂/保安/临时工场景较高 | 必须保留实体卡 Tier 2 凭据 |

**网络环境：**

印尼城市 4G 整体稳定，但存在明显的网络分层：

- **写字楼/商业区**：WiFi + 4G 覆盖好，联网鉴权无障碍
- **工业园区/工厂**：网络质量参差不齐，部分区域信号弱
- **地下停车场/地下室**：弱网或无网常态，离线能力是刚需
- **群岛地区/农村**：网络覆盖差，远程管理需要离线优先设计

**结论：离线能力不是加分项，是印尼市场的基础要求。**

### 3.4 市场痛点与机会

**中小企业（50-500 人）：**
- 痛点：ZKTeco 本地管理，数据靠 U 盘导出，员工离职手动删卡，多地点无法统一管理，无实时审计
- 机会：接入现有 ZKTeco 设备 + 云端统一管理，无需换硬件，部署阻力极低

**中型企业/写字楼（500-5000 人）：**
- 痛点：海康门禁与 HR 系统割裂，员工入离职无自动联动，访客管理靠纸质登记，多地点权限管理混乱
- 机会：海康集成 + SCIM 联动 HR + 访客数字化，移动凭据替换实体卡

**大型企业/银行/数据中心：**
- 痛点：合规审计要求严格，无完整操作日志，实体卡丢失风险，多因子认证缺失
- 机会：高安全 BLE 凭据 + DESFire 卡 + 完整审计链 + 多因子认证

---

## 四、核心设计决策

### 4.1 不做 NFC HCE 作为主力方案

NFC HCE 的根本安全局限在于凭据存储位置：

```
DESFire 实体卡 / Apple SE / HID Seos：
  密钥在专用硬件安全芯片（SE）里
  物理上不可能提取，攻击面为零

HCE + Android Keystore（TEE）：
  密钥在 CPU 内的可信执行环境（TEE）
  TEE 与主 CPU 共享硅片
  正常情况无法提取，但 root 设备 + 专用工具存在理论提取风险
  这不是"加固"能完全解决的，是架构天花板
```

补充说明：即使通过 Root 检测、极短 TTL、挑战响应等手段加固，HCE 的安全等级仍低于硬件 SE 方案。在印尼这个价格敏感市场，客户可能不理解技术细节，但一旦出现安全事件，平台信誉将受到根本性损害。

**决策：HCE 可以作为 Phase 2 的低优先级辅助功能（明确标注安全等级较低），但不作为主力凭据方案。**

### 4.2 BLE 在 Controller 侧实现，而非读头侧

如前所述，市面主流读头的 BLE 均锁定自家生态，第三方无法接入。因此 BLE 凭据的正确实现位置是 Controller，而非读头。

```
错误思路：手机 BLE → 第三方读头（BLE 锁定，走不通）

正确思路：
手机 BLE → 你的 Controller（含 BLE 模块）
                    ↓ Wiegand / OSDP
               现成读头（只做 NFC/RFID 感应和信号输出）
                    ↓
                  开门
```

Controller 内嵌 BLE 的硬件开发复杂度远低于自研读头，Phase 1 用树莓派 CM4 + nRF52840 模组即可出工程样机，不需要开模具，不需要射频认证。

### 4.3 三层凭据覆盖印尼市场分层现实

强迫所有用户用手机 BLE 开门，会丢掉印尼大量工厂、保安、临时工客户。三层凭据是适配印尼市场分层现实的必要设计：

- **Tier 1（BLE 移动凭据）**：高安全，覆盖白领/管理层
- **Tier 2（DESFire 实体卡）**：中高安全，覆盖工厂工人/无合适手机用户/访客
- **Tier 3（动态二维码）**：低安全，覆盖临时访客/快递/一次性通行

平台的核心价值是**云端统一管理**，三种凭据在同一平台统一管控，而不是强制用户使用特定凭据形态。

---

## 五、整体平台架构

### 5.1 设计原则

| 原则 | 具体说明 |
|---|---|
| 平台完全独立 | 不依赖 Apple Wallet、Google Wallet、HID 或任何第三方支付/钱包生态 |
| 安全分层 | 三层凭据对应三个安全等级，覆盖印尼各类客户场景 |
| 离线优先 | Controller 本地缓存白名单，断网 72 小时不影响正常开门 |
| 吊销实时 | 云端即时发起，Controller 在线 < 5 秒完全生效 |
| 存量兼容 | 接入海康/ZKTeco 存量设备，客户无需换任何硬件 |
| 开放生态 | 北向 Webhook + REST + SCIM，向第三方 SI 和客户自研系统开放 |
| 自主演进 | 硬件路线从"适配存量"到"自研读头"，平台控制权始终在自己手中 |

### 5.2 整体架构图

```
                    ┌──────────────────────────────────────────────────────┐
                    │                  你的 SaaS 云平台                      │
                    │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────┐ │
                    │  │ 凭据服务  │ │ 权限引擎  │ │ 审计日志  │ │  KMS   │ │
                    │  └──────────┘ └──────────┘ └──────────┘ └────────┘ │
                    │  ┌─────────────────────┐  ┌───────────────────────┐ │
                    │  │    南向设备网关        │  │  北向开放 API          │ │
                    │  │ 海康/ZKTeco/TTLock   │  │  REST/Webhook/SCIM   │ │
                    │  └─────────────────────┘  └───────────────────────┘ │
                    └─────────────────────┬────────────────────────────────┘
                                          │ TLS 1.3 双向认证
                    ┌─────────────────────┼──────────────────┐
                    ▼                     ▼                  ▼
             ┌─────────────┐    ┌──────────────────┐  ┌────────────────┐
             │  管理员       │    │   你的 Controller  │  │   用户手机 App  │
             │  Web 控制台   │    │  ┌─────────────┐ │  │               │
             └─────────────┘    │  │  BLE 模块    │ │  │  BLE（主力）   │
                                │  │  nRF52840   │◄├──┤  NFC（辅助）   │
                                │  └─────────────┘ │  │  App 内开门    │
                                │  ┌─────────────┐ │  └────────────────┘
                                │  │ 本地白名单   │ │
                                │  │ 72h 离线缓存│ │  ┌────────────────┐
                                │  └─────────────┘ │  │  DESFire 实体卡 │
                                └────────┬──────────┘  └───────┬────────┘
                                         │ Wiegand / OSDP      │ NFC 直接刷卡
                                ┌────────▼──────────────────────▼────────┐
                                │         存量读头（海康/ZKTeco/任意品牌） │
                                │         支持 Wiegand 或 OSDP 输出       │
                                └────────────────────┬───────────────────┘
                                                     │
                                              电控锁 / 门禁控制器
```

---

## 六、凭据分层体系

这是整个方案最重要的设计决策，决定了平台能否覆盖印尼市场的分层现实。

### 6.1 三层凭据概览

```
┌────────────────────────────────────────────────────────────────┐
│  Tier 1：BLE 移动凭据（高安全）                                    │
│  ──────────────────────────────────────────────────────────── │
│  适用场景：白领、管理层、IT 人员、高安全要求场所                      │
│  安全机制：手机 Keystore 私钥签名 + Controller 双向认证              │
│  私钥存储：Android Keystore / iOS Secure Enclave（硬件隔离）         │
│  印尼覆盖：2018 年后主流安卓机型均支持                               │
├────────────────────────────────────────────────────────────────┤
│  Tier 2：DESFire EV2/EV3 实体卡（中高安全）                        │
│  ──────────────────────────────────────────────────────────── │
│  适用场景：工厂工人、保安、无合适手机用户、访客卡、来宾卡               │
│  安全机制：AES-256 加密 + 随机 UID + 双向认证                       │
│  硬件兼容：兼容所有支持 DESFire 的主流读头（海康/Suprema/HID 等）     │
│  印尼覆盖：普遍兼容，无手机要求                                      │
├────────────────────────────────────────────────────────────────┤
│  Tier 3：动态二维码（低安全，访客专用）                              │
│  ──────────────────────────────────────────────────────────── │
│  适用场景：临时访客、快递员、外包工人、一次性通行                      │
│  安全机制：HTTPS 短效 token，有效期可配置（5 分钟至 24 小时）          │
│  使用门槛：无需安装 App，点击短链即可使用                             │
│  局限性：安全等级低，不应用于常规员工                                 │
└────────────────────────────────────────────────────────────────┘
```

### 6.2 三层凭据安全等级横向对比

| 维度 | Tier 1 BLE | Tier 2 DESFire 卡 | Tier 3 二维码 |
|---|---|---|---|
| 密钥存储位置 | 手机硬件（Keystore/SE） | 卡片硬件 SE | 云端 token |
| 克隆难度 | 极难 | 极难 | 低（截图可复用，需短有效期） |
| 遗失吊销速度 | < 5 秒（云端即时） | < 5 秒（云端即时） | token 自动过期 |
| 离线开门 | ✅ Controller 缓存 | ✅ Controller 缓存 | ❌ 需联网验证 |
| 防重放攻击 | ✅ 每次唯一 Nonce | ✅ 原生支持 | ⚠️ 依赖 token 有效期 |
| 设备要求 | 支持 BLE 的安卓/iOS | 无 | 任何有摄像头的设备 |
| 适合人群 | 正式员工 | 正式员工/访客 | 临时访客 |

### 6.3 凭据与读头兼容矩阵

```
                    海康读头    ZKTeco    HID Signo   Suprema
Tier 1 BLE       │    -         -          -            -      │
（经 Controller）  │ ✅ OSDP   ✅ OSDP   ✅ OSDP     ✅ OSDP  │
                  │ （Wiegand 输出，读头不感知 BLE 存在）        │
──────────────────┼───────────────────────────────────────────┤
Tier 2 DESFire    │  ✅直接刷  ✅直接刷   ✅直接刷    ✅直接刷  │
（NFC 直接刷）     │                                           │
──────────────────┼───────────────────────────────────────────┤
Tier 3 QR 码      │ 需摄像头   需摄像头   ❌无摄像头   部分型号  │
                  │ 或二维码读头（可独立部署）                   │
```

---

## 七、核心组件详解

### 7.1 SaaS 云平台

云平台是整个方案的控制核心，负责凭据全生命周期管理、权限配置、审计日志，以及南北两个方向的网关服务。

**核心模块：**

| 模块 | 职责 | 关键设计点 |
|---|---|---|
| 凭据服务 | 签发、更新、吊销用户凭据证书；维护用户公钥与设备绑定关系 | PKI 体系，私钥从不上传；凭据携带 TTL |
| 权限引擎 | 管理「用户 × 门 × 时间段」三维权限矩阵 | 支持动态实时变更；变更推送至 Controller < 5 秒 |
| 密钥管理（KMS） | Root CA 主密钥托管于 HSM，不可导出；Controller 和用户证书由 KMS 签发 | AWS KMS 或 GCP Cloud KMS |
| 审计日志 | 记录所有开门事件、权限变更、吊销操作 | 不可篡改；支持实时查询与导出；满足印尼 PDPA 合规 |
| 推送服务 | WebSocket 实时推送权限变更和吊销至 Controller；FCM 推送凭据更新至手机 | 推送失败自动重试；离线 Controller 下次联网补偿 |
| 南向网关 | 与海康/ZKTeco/TTLock/ONVIF 等设备通信 | 详见第十章 |
| 北向 API | REST + Webhook + SCIM 开放给第三方 | 详见第十一章 |

**技术栈建议：**

```
后端语言：  Go（低延迟认证关键路径）+ Node.js（管理控制台 BFF）
数据库：    PostgreSQL（权限/用户数据）+ Redis（实时状态/黑名单/Token 缓存）
消息队列：  NATS 或 RabbitMQ（Controller 实时推送解耦）
KMS：      AWS KMS 或 GCP Cloud KMS（主密钥托管）
推送：      Firebase Cloud Messaging（手机端凭据通知）
基础设施：  GCP asia-southeast2（雅加达，延迟最低）或 AWS ap-southeast-1（新加坡）
CDN：      Cloudflare（印尼节点覆盖好）
容器：      Kubernetes（弹性伸缩）
```

**优缺点评估：**

| 优点 | 缺点 / 风险 |
|---|---|
| 完全自主控制认证与吊销链路，不受任何第三方平台规则约束 | 需自行承担安全运营责任，无第三方认证背书（CC EAL / FIPS） |
| 实时吊销能力优于 Apple/Google Wallet（无需经过第三方 TSM） | PKI 和 KMS 体系建设有一定工程复杂度，需要密码学工程能力 |
| 可灵活定制权限模型，支持印尼各类客户场景 | 初期安全运营成本高于直接使用 Apple/Google 生态 |
| 南向接入海康/ZKTeco，立即获得印尼存量市场覆盖 | 多设备协议兼容（ISAPI/Push SDK/OSDP）测试工作量大 |

---

### 7.2 Controller（现场边缘设备）

Controller 是整个方案离线能力和 BLE 凭据的核心实现载体。部署于客户现场，叠加在客户现有门禁控制器之上，无需替换原有硬件。

**核心职责：**

- 内嵌 BLE 模块，接受手机 BLE 凭据，完成双向认证
- 本地缓存 50,000+ 条用户凭据（公钥 + 权限 + 时间规则），断网独立鉴权
- 通过 Wiegand / OSDP 控制下游读头和门禁控制器
- 联网状态下与云端实时同步权限变更（< 5 秒）
- 离线事件本地队列，联网后批量上传

**分阶段硬件方案：**

**Phase 1：工程样机（快速出货，验证可行性）**

```
主控板：   树莓派 CM4（4GB RAM + 32GB eMMC）或 Rockchip RK3568 工业板
BLE 模块： Nordic nRF52840 USB dongle 或 UART 模组（$5-10）
操作系统： Linux（Debian / Yocto）
本地存储： SQLite（凭据缓存）
网络：     以太网 PoE（IEEE 802.3af）
输出接口： GPIO（Wiegand）+ RS-485（OSDP）
外壳：     ODM 工业塑料外壳，无需开模
成本估算： $30-60 BOM（批量后可压低）
```

**Phase 2：定制 PCB（降成本，量产）**

```
主控：     Nordic nRF9160（集成 LTE-M/NB-IoT）或 ESP32-C6（WiFi+BLE 二合一）
安全芯片： ATECC608（或集成 ARM TrustZone）
存储：     外挂 Flash（凭据数据 AES-256 加密）
接口：     PoE + RS-485 + GPIO
BOM 成本： $20-35（规模量产后）
认证：     CE / FCC / SRRC（印尼进口需要）
```

**关键参数：**

```
本地缓存容量：   50,000+ 条用户凭据
云端同步延迟：   联网状态下权限变更 < 5 秒推送并生效
离线工作时长：   最长 72 小时本地独立鉴权
BLE 认证耗时：   < 300ms（与物理刷卡体感相当）
本地存储加密：   AES-256（per-device key，设备证书生成时派生）
固件安全：      Secure Boot + OTA RSA-2048 签名验证
通信加密：      TLS 1.3 + 设备证书双向认证
防拆保护：      防拆开关，触发后立即上报云端告警
调试接口：      出厂前永久禁用（防物理攻击）
```

**优缺点评估：**

| 优点 | 缺点 / 风险 |
|---|---|
| 叠加部署，客户现有控制器完全保留，部署阻力极低 | 前期需要自研或采购 Controller 硬件，有一定研发成本 |
| 72 小时离线能力覆盖印尼弱网场景 | Controller 本身有单点风险，高可用场景需考虑双机热备 |
| BLE 在 Controller 侧实现，完全绕开读头厂商生态锁定 | 硬件 SKU 管理和供应链引入额外运营复杂度 |
| 本地鉴权延迟极低，不依赖云端往返 | 硬件认证（CE/FCC）周期 2-4 个月，需提前规划 |
| 云端 OTA 更新固件，安全策略可远程升级 | 若 Controller 被物理盗取，需确保本地数据加密足够强 |
| Phase 1 可用树莓派快速验证，无需等待定制硬件 | — |

---

### 7.3 手机 App（用户凭据层）

**凭据存储与安全：**

```
Android（主力平台）：
  密钥存储：  Android Keystore（TEE 硬件隔离）
  高端机型：  StrongBox（独立安全芯片，等同 SE）
  低端机型：  TEE（与主 CPU 共享硅片，但仍是硬件隔离）
  密钥生成：  设备本地生成 EC P-256 密钥对，私钥永不导出
  设备证明：  Device Attestation Certificate 上传云端，云端验证密钥在真实硬件中生成

iOS（Phase 2 支持）：
  密钥存储：  Secure Enclave（苹果自研硬件安全芯片）
  BLE 开门：  完全支持（BLE 对第三方 App 完全开放）
  NFC 读写：  NFC Tag 读取完全支持；NFC HCE 历史上受限（EU DMA 后有改善）
```

**支持的开门方式：**

| 开门方式 | 底层协议 | 安全等级 | 适用场景 | Phase |
|---|---|---|---|---|
| BLE 感应开门 | BLE 5.0 + 自研 GATT | 高 | 靠近 Controller 自动感应，主力方案 | Phase 1 |
| App 内一键开门 | HTTPS → 云端 → Controller | 高 | 远程开门、临时授权 | Phase 1 |
| 动态二维码 | HTTPS 短效 token | 低 | 访客临时通行，无需 App | Phase 1 |
| NFC 刷卡（HCE） | NFC HCE ISO 14443-4 | 中 | 靠近读头轻触，辅助方案 | Phase 2 |

**安全加固措施：**

```
Root / Jailbreak 检测：  Google Play Integrity API（检测到 Root 拒绝凭据下发）
证书 Pinning：           云端公钥在 App 内固化，防中间人攻击
生物识别锁：              高安全场景强制 Face ID / 指纹解锁后再传输凭据
凭据 TTL：               所有凭据携带有效期，断网场景下自动失效
低端机降级：              不支持 StrongBox 的设备使用 TEE + 缩短 TTL（8 小时）
```

**优缺点评估：**

| 优点 | 缺点 / 风险 |
|---|---|
| Android Keystore 私钥硬件隔离，安全等级接近 SE | 需要用户安装 App，相比 Wallet 方案多一步操作摩擦 |
| BLE 在印尼安卓设备覆盖率极高（95%+ 设备支持） | 部分国产安卓机型 BLE 后台保活限制，需针对主流机型专项适配 |
| iOS 和 Android 均通过 BLE 完全支持，不依赖 NFC HCE | 低端机型 StrongBox 不可用，需降级处理 |
| App 内开门提供远程控制能力，是 Wallet 方案没有的功能 | 印尼工厂/保安等群体对 App 使用习惯不稳定，Tier 2 实体卡必须保留 |

---

### 7.4 读头硬件策略

**Phase 1-3：利用存量读头，不自研**

客户现有的海康/ZKTeco/HID 读头通过 Controller 的 Wiegand/OSDP 接口控制。Controller 向读头发送"开门"信号，读头不参与 BLE 认证逻辑，只作为执行件。

同时，读头的 NFC/RFID 功能保留，用于扫描 Tier 2 DESFire 实体卡，卡数据通过 Wiegand/OSDP 上报 Controller，Controller 再鉴权。

**Phase 4：自研智能 IP 读头（对标 Kisi Reader Pro）**

```
目标形态：
  - 读头自带以太网（PoE 供电）
  - 读头内嵌 BLE 5.0 模块，接受手机凭据
  - 读头内嵌 NFC，支持 DESFire 实体卡直接刷
  - 读头与云端直接通信（IP 直连），Controller 仅负责继电器控制
  - 读头内置安全芯片，存储读头私钥（用于双向认证）

时机：平台规模和现金流稳定后启动，预计 Phase 3 完成后
价值：掌控硬件毛利，产品差异化，形成完整垂直整合形态
```

---

## 八、安全架构

### 8.1 整体信任链

```
云端 Root CA（HSM 托管，私钥不可导出）
    │
    ├── Controller 设备证书（出厂烧录，全局唯一）
    │       └── 读头设备证书（OSDP Secure Channel，Phase 4）
    │
    └── 用户凭据证书
            ↑  手机 Keystore 本地生成 EC P-256 密钥对
            ↑  公钥 + Device Attestation Certificate 上传云端
            ↑  云端验证 Attestation 后签发证书，下发至手机
            ↑  凭据同时同步至对应 Controller 本地白名单
```

### 8.2 BLE 双向认证握手（详细时序）

每次开门均使用一次性随机数（Nonce），防止重放攻击。双向认证确保手机和 Controller 互相验证身份，防止伪造 Controller 攻击。

```
T+0ms    Controller 广播 BLE Beacon
         └── 包含：Controller_ID（明文）+ Nonce_R（每次唯一随机数，32字节）

T+10ms   手机 App 扫描到 Beacon，建立 BLE 连接（GATT）

T+15ms   [Phase 1：Controller 向手机发送身份证明]
         Controller → 手机：{ Sign(Nonce_P_request, Controller 私钥), 读头证书 }
         手机验证 Controller 证书链 ✅（防伪造 Controller 攻击）

T+20ms   [Phase 2：手机向 Controller 发送凭据]
         手机调用 Android Keystore Sign：
           data = Nonce_R + User_ID + Timestamp + Controller_ID
           key  = 设备私钥（永不离开 Keystore）
         手机 → Controller：{ User_ID, Timestamp, Signature, 凭据证书 }

T+35ms   Controller 本地鉴权：
         ① 验证凭据证书链（云端 CA 签发）✅
         ② 验证手机签名（用用户公钥）✅
         ③ 查本地白名单：User_ID 是否有此门权限 ✅
         ④ 验证时间规则：当前时间是否在授权时段 ✅
         ⑤ 验证 Timestamp：防止重放（允许 ±30 秒时钟偏差）✅

T+50ms   Controller 触发 Wiegand/OSDP 开门信号

T+55ms   BLE 通知手机：认证结果（成功/失败）

T+60ms   开门完成（门锁响应）

T+异步   开门事件上传云端审计日志
         - Controller 在线：实时上传
         - Controller 离线：本地队列，联网后批量同步

目标总耗时：< 300ms（与物理刷卡体感相当）
```

### 8.3 DESFire 卡认证流程（Tier 2）

```
用户刷卡（DESFire EV2/EV3）
    │ NFC，ISO 14443-4
    ▼
读头（海康/ZKTeco/任意 DESFire 兼容读头）
    │ DESFire 三步 AES-256 认证（读头侧执行）
    │   1. 读头 → 卡：GetChallenge
    │   2. 卡 → 读头：RndA + RndB_encrypted
    │   3. 读头 → 卡：RndA' 验证
    │ 认证通过后读取卡内 UserID
    │ Wiegand / OSDP 输出 UserID
    ▼
Controller
    │ 查本地白名单：UserID 是否有权限
    ▼
开门
```

**注意：** DESFire 卡的密钥（DiversifiedKey）由你的云端 KMS 派生并写入，读头不存储主密钥，只存储应用密钥。卡片丢失时，云端标记 UserID 失效，即使找到卡也无法使用。

### 8.4 安全等级横向对比

| 安全维度 | Apple Wallet ECP2 | HID Seos 移动版 | 本方案 Tier 1 BLE | Tier 2 DESFire 卡 |
|---|---|---|---|---|
| 凭据存储位置 | Apple Secure Enclave | HID 硬件 SE | Android Keystore（TEE/StrongBox） | 卡片硬件 SE |
| 私钥可导出 | ❌ 不可 | ❌ 不可 | ❌ 不可 | ❌ 不可 |
| 双向认证 | ✅ 原生 | ✅ 原生 | ✅ 自行实现 | ✅ DESFire 原生 |
| 防重放攻击 | ✅ 原生 | ✅ 原生 | ✅ 每次唯一 Nonce | ✅ 原生 |
| 凭据克隆难度 | 极难 | 极难 | 极难 | 极难 |
| 实时吊销自主性 | ❌ 依赖 Apple TSM | ❌ 依赖 HID TSM | ✅ 完全自主 < 5 秒 | ✅ 完全自主 < 5 秒 |
| 离线开门 | ✅ 卡片/SE 本地 | ✅ 本地 | ✅ Controller 缓存 | ✅ Controller 缓存 |
| 印尼可用性 | ❌ | ✅ 但成本高 | ✅ 无障碍 | ✅ 无障碍 |
| 第三方安全认证 | ✅ Apple 认证 | ✅ CC EAL | ❌ 暂无 | ✅ NXP FIPS |
| 平台自主权 | ❌ 苹果规则 | ❌ HID 规则 | ✅ 完全自主 | ✅ 完全自主 |

### 8.5 云端安全加固

```
速率限制：     同一 User_ID 开门频率异常（如 1 分钟内 >10 次）触发告警并临时锁定
异常检测：     同一凭据在两个相距 >100km 的地点短时间内同时出现 → 自动锁定账户
地理围栏：     可选，印尼境外管理操作强制二次验证（防账号被境外攻击者盗用）
HSM：         Root CA 主密钥存储于硬件安全模块，无任何软件导出路径
日志不可篡改：  审计日志写入后加密签名，任何修改可被检测
定期轮换：     Controller 设备证书每 12 个月自动轮换，手机凭据 TTL ≤ 24 小时
```

---

## 九、凭据生命周期

### 9.1 BLE 移动凭据下发（Tier 1 Provisioning）

```
① 管理员在 SaaS 控制台创建用户账号，分配门权限（门 + 时间段）

② 系统自动向用户发送激活邀请（短信 / 邮件 / 企业 App 推送）

③ 用户打开 App，完成首次激活：
   a. App 调用 Android Keystore.generateKeyPair(EC_P256, StrongBox/TEE)
   b. EC P-256 密钥对在 Keystore 内部生成，私钥永不可被 App 读取
   c. 生成 Device Attestation Certificate 证明密钥在真实硬件中生成
   d. 公钥 + Attestation 证书 + 设备指纹 上传云端

④ 云端处理注册请求：
   a. 验证 Device Attestation 证书链（确认密钥确实在 Keystore 硬件中生成）
   b. 验证设备未被 Root（结合 Play Integrity API 结果）
   c. 用 KMS 签发用户凭据证书（含 User_ID + 权限摘要 + 有效期）
   d. 将凭据证书推送至用户 App
   e. 同时将用户公钥 + 完整权限规则 同步至对应 Controller 本地缓存

⑤ 用户激活完成，可以通过 BLE 开门
```

### 9.2 DESFire 实体卡发行（Tier 2 Provisioning）

```
① 管理员在控制台发起制卡请求，指定用户和权限

② 云端 KMS 为该卡派生 DiversifiedKey（基于卡 UID + 主密钥派生，每卡唯一）

③ 管理员使用授权的 NFC 写卡设备（连接平台）将 AID + DiversifiedKey + UserID 写入卡片

④ 卡片信息同步至对应 Controller 白名单（UserID + 权限 + 时间规则）

⑤ 卡片发放给用户，即可直接刷卡开门

注意：主密钥始终在 KMS 中，DiversifiedKey 通过安全通道下发至写卡设备，
      写卡完成后 DiversifiedKey 不在任何客户端长期存储
```

### 9.3 日常开门（Runtime）

```
BLE 开门（在线，Controller 联网）：
手机 BLE → Controller 双向认证 → 本地鉴权通过 → Wiegand/OSDP 开门信号
                                                      └→ 开门事件实时上传云端

BLE 开门（离线，Controller 断网）：
手机 BLE → Controller 双向认证 → 本地鉴权通过 → Wiegand/OSDP 开门信号
                                                      └→ 事件写本地队列
                                                      └→ 联网后批量上传云端

DeSFire 卡刷卡（任意网络状态）：
DeSFire 卡 NFC → 读头 AES 认证 → Wiegand 输出 UserID → Controller 鉴权 → 开门
```

### 9.4 权限变更

```
管理员修改权限 → 云端立即更新
    │
    ├── Controller 在线：
    │   WebSocket 推送变更指令 → Controller 收到后 < 5 秒更新本地缓存
    │   用户下次 tap 即使用新权限
    │
    └── Controller 离线：
        变更记录在云端排队
        Controller 重新联网时自动拉取全量最新权限（冲突以云端为准）
```

### 9.5 凭据吊销（Revocation）

```
触发场景：
  - 员工离职（HR 系统联动或手动操作）
  - 手机丢失（用户自助在 App/Web 申报）
  - 账号异常（系统自动检测触发）
  - 卡片丢失（管理员手动操作）

BLE 移动凭据吊销流程：
  管理员点击吊销 → 云端立即标记凭据失效 → 写入黑名单缓存（Redis）
      │
      ├── Controller 在线：
      │   WebSocket 推送吊销指令
      │   Controller 从白名单删除该 User_ID
      │   该手机下次 BLE tap 立即被拒绝（< 5 秒生效）
      │
      └── Controller 离线（最坏情况）：
          凭据 TTL 到期自动失效（最长等待 24 小时）
          Controller 重新联网后立即同步吊销状态

手机端处理：
  App 下次联网收到吊销通知
  本地凭据证书删除
  Android Keystore 密钥对销毁（KeyPairGenerator.generateKeyPair 前先 deleteEntry）

DeSFire 实体卡吊销：
  云端标记 UserID 失效 → Controller 白名单删除
  物理卡无法被克隆，标记失效后刷卡立即被拒绝
  不需要物理回收卡片（当然回收更好）
```

---

## 十、设备集成网关

### 10.1 架构关系

```
第三方系统（HR / BMS / SI 自研 / 客户 ERP）
                 │  北向 API（Northbound）
                 ▼
        ┌──────────────────┐
        │   你的 SaaS 平台   │
        └──────────────────┘
                 │  南向网关（Southbound）
                 ▼
  海康 / ZKTeco / TTLock / ONVIF / 大华...
```

### 10.2 南向网关优先级

#### 🥇 P1 — 海康威视（Hikvision）

**优先原因：** 印尼写字楼、工厂、园区装机量最大，覆盖门禁、摄像、闸机、对讲全产品线。

海康提供三层集成方式，需同时支持前两层以覆盖不同客户部署形态：

| 集成层级 | 协议 | 适用场景 | 优先级 |
|---|---|---|---|
| 设备直连 | ISAPI（HTTP REST，1500+ 接口） | 设备与平台同局域网，或设备有固定公网 IP | 必做 |
| 云端透传 | ISUP 5.0（海康私有协议，支持 NAT 穿透） | 设备在远端，通过海康云通道接入第三方平台 | 必做 |
| 平台级对接 | HikCentral OpenAPI | 客户已部署 HikCentral 管理平台 | 二期 |

**ISAPI 核心能力：**

```
门禁控制：  GET  /ISAPI/AccessControl/door/capabilities
           PUT  /ISAPI/AccessControl/RemoteControl/door/{doorNo}（远程开门）
           GET  /ISAPI/AccessControl/AcsEvent（刷卡记录）
用户管理：  POST /ISAPI/AccessControl/UserInfo/Record（创建用户）
           DELETE /ISAPI/AccessControl/UserInfo/Delete（删除用户）
卡片管理：  POST /ISAPI/AccessControl/CardInfo/Record（发卡）
事件订阅：  ISUP 5.0 长连接事件推送（实时事件流）
```

**优缺点：**

| 优点 | 缺点 / 风险 |
|---|---|
| 印尼装机量最大，接入即覆盖大部分存量市场 | ISAPI 完整文档需签署 TPP 合作协议方可获取 |
| 设备功能丰富，可联动摄像头、报警、对讲 | 不同固件版本、不同型号 ISAPI 实现存在细节差异，测试工作量大 |
| ISUP 5.0 支持 NAT 穿透，覆盖无固定 IP 场景 | 对海康生态的依赖一定程度引入供应商风险 |
| 开放 API 超过 1500 个，功能覆盖全面 | ISAPI 使用 XML/JSON 混合格式，解析实现有一定复杂度 |

---

#### 🥈 P2 — ZKTeco

**优先原因：** 印尼中小企业门禁市场占有率极高，工厂、学校、中小写字楼标配，覆盖指纹、人脸、RFID 多形态设备。

ZKTeco Push SDK 是设备主动向平台推送数据的模式，配置简单，实时性好，集成成本最低。

**Push SDK 核心机制：**

```
配置阶段：
  在 ZKTeco 设备管理界面填写你的服务器 IP:Port
  选择推送协议（ADMS/PushSDK）

运行时：
  设备刷卡 → 设备主动 TCP 推送 → 你的接收服务
  不需要轮询，实时性与 WebSocket 相当

推送数据格式（示例）：
  {
    "sn": "设备序列号",
    "table": "ATTLOG",
    "data": "UserID\tTimestamp\tStatus\tVerifyType"
  }

反向控制（你 → 设备）：
  POST 到设备本地 HTTP 接口（需设备在同局域网或有公网 IP）
  支持：发卡、删卡、修改用户信息、远程开门
```

**优缺点：**

| 优点 | 缺点 / 风险 |
|---|---|
| Push 模式不需轮询，集成简单，实时性好 | Push SDK 为私有协议，不同设备型号间存在细微差异 |
| 覆盖指纹、人脸、RFID 多种认证形态 | 设备网络安全性较弱，需在平台侧做数据校验防篡改 |
| 中小企业客户基数大，拓客成本低 | 反向控制（平台→设备）需要设备有可访问 IP，NAT 场景有局限 |
| 设备价格低，客户购置成本低 | ZKTeco 设备权限管理粒度较粗，时间段规则不如高端系统灵活 |

---

#### 🥉 P3 — TTLock

**优先原因：** IoT 智能门锁场景（公寓、联合办公、酒店客房）在印尼增长很快，TTLock 是市场主流的智能门锁云平台之一，REST API 接入成本最低。

**集成模式：**

```
TTLock 云平台（统一管理旗下所有智能锁）
    │  TTLock Open API（OAuth 2.0 + REST）
    ▼
你的 SaaS 平台
    │  调用 TTLock API 发送开锁指令、查询记录
    ▼
TTLock 云 → 下发至具体锁设备（BLE/WiFi）
```

**核心 API：**

```
POST /v3/lock/unlock         远程开锁
POST /v3/keyboardPwd/add     添加密码
GET  /v3/lockRecord/list     查询开锁记录
POST /v3/lock/delete         删除权限
```

**优缺点：**

| 优点 | 缺点 / 风险 |
|---|---|
| REST API 标准，接入周期短，文档完善 | 平台与锁之间通信经 TTLock 云中转，依赖其稳定性 |
| 覆盖大量低功耗智能锁场景（无需布线） | 若 TTLock 平台故障，你的平台连带受影响，需设计降级预案 |
| 公寓/联办场景增长快，新客户获取场景多样 | 安全等级相对低，不适合高安全场景 |

---

#### P4 — ONVIF + 大华

**ONVIF（优先实现 Profile A）：**

ONVIF Profile A 定义了门禁系统的开放标准接口，一次实现兼容百余品牌，是最高效的扩展路径。

```
ONVIF Profile A：门禁设备标准接口（读卡记录、权限管理、远程开门）
ONVIF Profile S：摄像头视频流标准接口（视频联动、录像查询）
```

**大华（Dahua）DSS API：**

大华是摄像市场全球第二，门禁业务快速扩张，其 DSS 平台提供开放 API，集成方式类似海康 ISAPI。

**优缺点：**

| 优点 | 缺点 / 风险 |
|---|---|
| ONVIF 一次实现，兼容大量品牌，后续扩展成本低 | ONVIF 各厂商实现一致性差，仍需逐品牌测试 |
| 大华在摄像市场份额大，门禁增长快 | ONVIF Profile A 在门禁领域采用率仍低于摄像领域 |

---

### 10.3 北向 API 优先级

#### 🥇 P1 — Webhook + REST API（基础能力）

这是 SI（系统集成商）和客户自研系统接入的第一需求，也是平台开放生态的基础。

**事件 Webhook（平台 → 第三方）：**

```json
// 开门事件示例
{
  "event_type": "ACCESS_GRANTED",
  "timestamp": "2025-03-01T08:30:00+07:00",
  "user_id": "user_123",
  "door_id": "door_456",
  "credential_type": "BLE",
  "controller_id": "ctrl_789",
  "location": "Jakarta HQ - Floor 3"
}
```

**设备控制 REST API（第三方 → 平台）：**

```
POST /v1/doors/{id}/unlock          远程开门
POST /v1/users                      创建用户并触发凭据下发
PUT  /v1/users/{id}/permissions     修改权限
DELETE /v1/users/{id}/credentials   吊销凭据
GET  /v1/audit-log                  查询审计日志
```

**优缺点：**

| 优点 | 缺点 / 风险 |
|---|---|
| 开发成本低，SI 接入门槛低，生态快速扩展 | 需做好 API 认证（OAuth 2.0）和速率限制，防止滥用 |
| Webhook 推送模式降低第三方轮询压力 | Webhook 在客户网络不稳定时可能丢事件，需实现重试和幂等机制 |

---

#### 🥈 P2 — SCIM 2.0 身份同步

对接企业 AD / Okta / 钉钉 / 飞书 / Workday，员工入职自动发凭据，离职自动吊销。这是打企业大客户的关键差异化能力。

```
企业 IdP（Okta / 飞书 / 钉钉）
        │  SCIM 2.0 协议
        ▼
你的 SaaS 平台 SCIM Endpoint
        │
        ├── 新建用户 → 自动触发凭据下发流程
        ├── 用户属性变更 → 同步更新权限
        └── 删除用户 → 自动吊销所有凭据
```

**优缺点：**

| 优点 | 缺点 / 风险 |
|---|---|
| 接一遍 SCIM，兼容几乎所有主流企业 HR/IdP 系统 | SCIM 2.0 规范较复杂，各 IdP 实现存在差异，需逐一测试 |
| 大幅降低企业客户管理运营成本，是大客户销售的核心卖点 | 身份同步涉及敏感数据，需符合印尼 PDPA 法规（UU PDP No.27/2022） |

---

#### 🥉 P3 — BACnet / BMS 集成

与楼宇管理系统（Schneider Electric / Johnson Controls / Honeywell BMS）打通，联动电梯、灯控、HVAC，适用于大型物业客户。

**优缺点：**

| 优点 | 缺点 / 风险 |
|---|---|
| 打开大型地产、物业管理公司客户 | BACnet over IP 实现复杂，开发周期长 |
| 门禁 + 楼控联动（进门自动开灯/空调）提升产品价值感 | 楼宇 BMS 采购决策链长，销售周期慢（3-12 个月） |

---

## 十一、核心 API 设计

### 11.1 Controller ↔ 云端

```
POST /v1/controller/register
  - Body: { device_cert, public_key, firmware_version, capabilities }
  - 返回: { controller_id, session_token, sync_url }
  - 用途: Controller 首次注册，上传设备证书，云端验证后建立信任关系

GET  /v1/controller/sync?since={timestamp}
  - 返回: { credentials[], access_rules[], revoked_ids[], timestamp }
  - 用途: 拉取全量（首次）或增量（后续）权限数据

POST /v1/controller/events
  - Body: [{ event_id, user_id, door_id, result, timestamp, credential_type }]
  - 用途: 批量上传离线期间积累的开门事件日志

WSS  /v1/controller/realtime
  - 服务器推送消息类型：
    PERMISSION_UPDATE  权限变更（新增/修改）
    CREDENTIAL_REVOKE  凭据吊销
    DOOR_UNLOCK        远程开门指令
    CONFIG_UPDATE      Controller 配置变更
    FIRMWARE_OTA       固件更新通知
```

### 11.2 手机 App ↔ 云端

```
POST /v1/device/register
  - Body: { public_key, attestation_cert, device_fingerprint, play_integrity_token }
  - 返回: { credential_cert, user_id, valid_until }
  - 用途: 上传公钥和设备证明，获取凭据证书

GET  /v1/credentials
  - 返回: [{ door_id, door_name, valid_from, valid_until, time_rules }]
  - 用途: 获取当前用户的凭据列表（含门权限详情）

POST /v1/credentials/{id}/revoke
  - Body: { reason: "lost_device" }
  - 用途: 用户自助注销（手机丢失场景）

GET  /v1/access-log?page={n}&per_page={n}
  - 返回: [{ door_name, timestamp, result, credential_type }]
  - 用途: 查看本人开门记录

POST /v1/doors/{id}/unlock
  - Body: { credential_cert, signature, timestamp }
  - 用途: App 内远程开门（经云端转发至 Controller）
```

### 11.3 管理员控制台 ↔ 云端

```
POST   /v1/users                           创建用户，触发凭据下发流程
GET    /v1/users?page={n}&search={q}       查询用户列表
PUT    /v1/users/{id}/permissions          分配/修改门权限（支持批量）
DELETE /v1/users/{id}/credentials          吊销用户所有凭据
POST   /v1/users/{id}/cards                为用户发行 DeSFire 实体卡

GET    /v1/doors                           查询门列表和状态
POST   /v1/doors/{id}/unlock               管理员远程开门
GET    /v1/doors/{id}/access-log           查询特定门的开门记录

GET    /v1/audit-log?from={ts}&to={ts}     审计日志查询（支持时间、用户、门等过滤）
GET    /v1/controllers/{id}/status         查询 Controller 在线状态和健康信息
POST   /v1/visitors                        创建访客，生成动态二维码
```

### 11.4 南向设备网关（平台内部接口）

```
// 海康威视
POST /internal/gateway/hikvision/{device_id}/door/unlock
GET  /internal/gateway/hikvision/{device_id}/events/stream
POST /internal/gateway/hikvision/{device_id}/users/sync

// ZKTeco
POST /internal/gateway/zkteco/push/receive      接收设备 Push 数据
POST /internal/gateway/zkteco/{device_id}/users/sync

// TTLock
POST /internal/gateway/ttlock/{lock_id}/unlock
GET  /internal/gateway/ttlock/{lock_id}/records

// ONVIF
POST /internal/gateway/onvif/{device_id}/door/unlock
GET  /internal/gateway/onvif/{device_id}/access-events
```

---

## 十二、方案综合评估

### 12.1 与主流方案横向对比

| 维度 | Apple Wallet ECP2 | HID Seos 移动版 | 本方案（BLE + Controller） |
|---|---|---|---|
| **安全等级** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **凭据存储** | Apple Secure Enclave | HID 硬件 SE | Android Keystore TEE/StrongBox |
| **离线开门** | ✅ SE 本地验证 | ✅ SE 本地验证 | ✅ Controller 本地缓存 |
| **吊销自主性** | ❌ 依赖 Apple 基础设施 | ❌ 依赖 HID TSM | ✅ 完全自主，< 5 秒 |
| **印尼可用性** | ❌ Apple Pay 不可用 | ✅ 可用但成本高 | ✅ 无障碍 |
| **读头成本** | 高（需 ECP2 认证，$100+） | 高（HID 认证读头） | 低（现成 Wiegand 读头） |
| **平台自主权** | ❌ 受苹果规则约束 | ❌ 受 HID 规则约束 | ✅ 完全独立 |
| **存量硬件兼容** | ❌ 需 ECP2 读头 | ❌ 需 HID 读头 | ✅ 兼容所有 Wiegand/OSDP 设备 |
| **第三方安全认证** | ✅ Apple 官方 | ✅ CC EAL | ❌ 暂无（商业场景够用） |
| **多凭据支持** | ⚠️ 仅 Apple Wallet | ⚠️ 仅 Seos | ✅ BLE + DeSFire 卡 + QR |

### 12.2 本方案核心优势

**对比 Apple/Google Wallet：**
- 不受印尼生态限制，立即可以上线，无需等待任何平台授权
- 吊销完全自主，毫秒级触发，无需经过任何第三方基础设施
- 多凭据体系覆盖印尼分层市场（白领用 BLE，工人用 DeSFire 卡）
- 兼容客户现有海康/ZKTeco 设备，无硬件替换成本

**对比 HID Seos：**
- 无需支付 HID 高额 License 授权费
- 不绑定 HID 硬件读头生态，读头可自由选型
- 云端集成更灵活，可深度定制权限模型
- 印尼市场 HID 装机量少，从海康/ZKTeco 存量切入更有效率

### 12.3 主要风险与应对措施

| 风险 | 严重程度 | 应对措施 |
|---|---|---|
| 无第三方安全认证（CC EAL / FIPS） | 中 | 面向商业客户初期够用；政府/金融场景可后续申请 ISO 27001 或引入 HID 联合方案 |
| Controller Phase 1 用树莓派成本较高 | 中 | 仅用于工程验证；Phase 2 定制 PCB 将 BOM 压至 $20-35 |
| Android 生态碎片化（BLE 后台保活） | 中 | 建立主流机型测试白名单（三星/OPPO/vivo/小米/realme），针对性适配 BLE 保活策略 |
| 海康 ISAPI 文档获取需签署 TPP 协议 | 低 | 提前申请海康 TPP 合作伙伴资质，流程有既定路径 |
| 依赖 TTLock 云端稳定性 | 低 | 设计降级方案：TTLock 不可用时提示用实体卡或 BLE 备用 |
| Controller 被物理盗取 | 低 | 本地存储 AES-256 加密（per-device key）；防拆开关联动云端告警；设备证书吊销机制 |
| 印尼 PDPA 合规要求 | 低 | 数据本地化（优先使用 GCP 雅加达节点）；用户数据最小化收集；隐私政策合规设计 |

---

## 十三、客户分层与切入策略

### 13.1 客户分层

**第一层：中小企业（50–500 人）— 最快切入，最低阻力**

```
典型客户：  中小型办公室、诊所、零售连锁、中小工厂
当前痛点：  ZKTeco 本地管理，U 盘导出数据
            员工离职手动删卡，经常忘记
            多个地点权限无法统一管理
            无移动开门能力

你的切入点：
  第一步：接入现有 ZKTeco → 云端统一管理（不换任何硬件）
  第二步：上线 DeSFire 卡管理（统一卡片发行和吊销）
  第三步：推广 BLE 手机开门（选择性推广，非强制）

定价参考：  按门/月订阅（印尼市场建议 $8–15/门/月）
销售策略：  SI 渠道销售，合作当地系统集成商
```

**第二层：中型企业 / 写字楼（500–5000 人）— 主力市场**

```
典型客户：  外资企业印尼分支、大型写字楼、工业园区、学校
当前痛点：  海康门禁与 HR 系统割裂
            访客管理靠纸质登记
            多地点权限管理混乱
            无实时审计日志

你的切入点：
  核心卖点：海康集成 + SCIM 联动 HR（员工入离职自动联动）
  差异化：  访客数字化（二维码 + 系统记录）
  升级路径：移动凭据替换部分实体卡场景

定价参考：  $15–30/门/月（含 SCIM 和访客管理模块）
销售策略：  直销 + 方案演示，决策周期 1–3 个月
```

**第三层：大型企业 / 银行 / 数据中心 — 高安全场景**

```
典型客户：  银行分行、数据中心、政府机关、大型制造业
当前痛点：  合规审计要求严格，需完整不可篡改日志
            实体卡管理混乱，丢卡风险高
            多因子认证能力缺失
            现有系统无法实时管理和远程吊销

你的切入点：
  核心卖点：完整审计链 + BLE 高安全凭据 + 多因子认证
  加分项：  双向认证防伪造读头 + 实时吊销 + 异常检测告警
  长期价值：与 BMS 集成（楼控联动）

定价参考：  $30–60/门/月（定制化，含 SLA 承诺）
销售策略：  直销 + 技术方案投标，决策周期 3–12 个月
```

### 13.2 印尼本地化运营要点

**合规：**
- 印尼个人数据保护法（UU PDP No.27/2022）2024 年正式生效，用户数据需告知同意，用户有删除权
- 优先使用 GCP 雅加达节点实现数据本地化，降低合规风险
- 访客数据保留策略需符合当地法规（建议最长保留 90 天）

**本地化：**
- App 界面支持印尼语（Bahasa Indonesia）
- 短信通知优先使用本地运营商（Telkomsel / Indosat），送达率高于国际 SMS
- 客服支持印尼语，配合当地工作时区（WIB UTC+7）

**渠道：**
- 印尼安防市场高度依赖 SI（System Integrator）渠道，直接触达终端客户效率低
- 优先建立雅加达、泗水、万隆三大城市的 SI 合作伙伴网络
- 提供 SI 专属后台、白标方案和返佣机制

---

## 十四、开发路线

### Phase 1 — MVP（3–4 个月）

**目标：** 核心平台跑通，覆盖印尼 70%+ 存量门禁硬件，验证商业模式。

```
云端平台：
  ✅ 用户管理（创建/编辑/删除）
  ✅ 权限管理（门 × 用户 × 时间段三维矩阵）
  ✅ 凭据签发与吊销（BLE 移动凭据 + DeSFire 卡）
  ✅ 审计日志（不可篡改，支持查询导出）
  ✅ KMS 集成（AWS KMS 或 GCP Cloud KMS）
  ✅ 管理员 Web 控制台（基础版）

Android App：
  ✅ BLE 开门（主力方案，基于 Keystore 私钥签名）
  ✅ App 内远程开门
  ✅ 凭据下发和本地存储
  ✅ Root 检测（Play Integrity API）

Controller：
  ✅ 工程样机（树莓派 CM4 + nRF52840 BLE 模块）
  ✅ BLE 双向认证握手
  ✅ 本地白名单缓存（SQLite）
  ✅ Wiegand 26/34 输出
  ✅ OSDP v2 输出
  ✅ 72 小时离线能力
  ✅ 云端 WebSocket 实时同步

凭据体系：
  ✅ Tier 1：BLE 移动凭据
  ✅ Tier 2：DeSFire EV2/EV3 实体卡管理
  ✅ Tier 3：动态二维码访客通行

南向网关：
  ✅ 海康威视 ISAPI 集成（局域网直连场景）
  ✅ 海康威视 ISUP 5.0 集成（云端透传场景）
  ✅ ZKTeco Push SDK 集成

北向 API：
  ✅ Webhook 事件推送
  ✅ REST API 基础版（用户/门/权限/审计日志）
```

**Phase 1 完成后能力：** 覆盖印尼 70%+ 门禁存量设备，支持移动和实体卡双凭据，可面向第一批客户上线。

---

### Phase 2 — 功能扩展（3–4 个月）

**目标：** 提升产品完整度，打开企业市场，覆盖更多设备类型。

```
Controller：
  ✅ 定制 PCB（nRF52840 核心板，BOM 成本 $20-35）
  ✅ Controller 防拆告警
  ✅ OTA 固件更新（RSA 签名验证）
  ✅ OSDP Secure Channel（加密通信）

iOS App：
  ✅ iOS BLE 开门（BLE 对第三方 App 完全开放）
  ✅ iOS Secure Enclave 密钥存储

南向网关：
  ✅ TTLock REST API 集成（IoT 智能锁）
  ✅ ONVIF Profile A/S 集成

北向 API：
  ✅ SCIM 2.0 身份同步（对接钉钉/飞书/Okta/Workday）
  ✅ Webhook 重试和幂等机制

功能增强：
  ✅ 访客管理系统（二维码 + 预约 + 记录）
  ✅ 实时告警（异常开门、设备离线、凭据异常）
  ✅ 多因子认证（BLE + 指纹/PIN 组合）
  ✅ 多地点统一管理（企业多分支场景）
  ✅ 数据报表（出入统计、异常分析）
```

---

### Phase 3 — 平台规模化（持续迭代）

```
南向网关：
  ✅ 大华 DSS API 集成
  ✅ Suprema BioStar 2 集成（高安全生物识别场景）
  ✅ BACnet over IP（BMS 楼控联动）
  ✅ HikCentral OpenAPI（海康平台级对接）

功能增强：
  ✅ 梯控集成（电梯楼层权限联动）
  ✅ 闸机联动（园区/停车场闸机统一管控）
  ✅ 人脸识别联动（ZKTeco/海康终端已具备此能力）
  ✅ 巡逻管理（安保路线规划和打卡）

商业体系：
  ✅ 多租户 SaaS 计费体系（按门/按用户/按功能模块分级）
  ✅ SI 合作伙伴后台（白标/返佣/配额管理）
  ✅ API 商业化（开放 API 付费调用）
```

---

### Phase 4 — 自研智能读头（Phase 3 稳定后启动）

**战略意义：** 掌控硬件毛利率，产品形态从"纯软件 SaaS"升级为"硬件 + 软件 + 云端"垂直整合，对标 Kisi 完整产品形态。

```
自研智能读头目标规格：
  NFC：    ISO 14443-A/B，13.56MHz，DeSFire EV3 原生支持
  BLE：    BLE 5.2，自研 GATT Protocol（与现有 Controller 协议兼容）
  网络：   PoE（IEEE 802.3af）+ 以太网直连云端
  安全芯片：内置独立硬件 SE，存储读头私钥（用于双向认证）
  接口：   OSDP v2（向下兼容客户现有控制器）
  防护：   IP65（防尘防水，适合印尼热带气候）
  认证：   CE / FCC / SRRC / MUI（印尼清真认证，部分客户要求）
  OTA：    RSA 签名固件更新，远程管理

架构演进：
  Phase 1-3：手机 BLE → Controller → Wiegand → 现成读头
  Phase 4：  手机 BLE/NFC → 自研 IP 读头（直连云端）→ 继电器
             Controller 角色简化为"继电器控制器"
```

> **硬件节奏说明：** 前三期通过 API 接入存量硬件，以最低成本验证商业模式和积累客户；待平台现金流稳定、对印尼市场硬件需求充分了解后，再投入自研读头。避免过早进入硬件研发导致资金和周期风险。

---

## 附录

### A. Controller 本地数据库结构

```sql
-- 用户凭据表（BLE 移动凭据 + DeSFire 卡）
CREATE TABLE credentials (
    user_id         TEXT NOT NULL,
    credential_type TEXT NOT NULL,      -- 'BLE' | 'DESFIRE' | 'QR'
    public_key      BLOB,               -- BLE 凭据：用户手机公钥（EC P-256）
    card_uid        TEXT,               -- DeSFire：卡片 UID
    valid_from      INTEGER NOT NULL,   -- Unix timestamp
    valid_until     INTEGER NOT NULL,   -- Unix timestamp（TTL）
    revoked         BOOLEAN DEFAULT 0,
    updated_at      INTEGER NOT NULL,
    PRIMARY KEY (user_id, credential_type)
);

-- 门权限表
CREATE TABLE access_rules (
    user_id         TEXT NOT NULL,
    door_id         TEXT NOT NULL,
    time_rules      TEXT NOT NULL,      -- JSON: [{"days":[1,2,3,4,5],"start":"0800","end":"2000"}]
    anti_passback   BOOLEAN DEFAULT 0,  -- 反潜回（进出匹配）
    PRIMARY KEY (user_id, door_id)
);

-- 本地事件队列（待上传云端）
CREATE TABLE event_queue (
    event_id        TEXT PRIMARY KEY,
    user_id         TEXT,
    door_id         TEXT,
    result          TEXT NOT NULL,      -- 'GRANTED' | 'DENIED' | 'TIMEOUT'
    deny_reason     TEXT,               -- 'NOT_IN_WHITELIST' | 'TIME_RULE' | 'REVOKED' | 'BAD_SIG'
    credential_type TEXT,
    timestamp       INTEGER NOT NULL,
    uploaded        BOOLEAN DEFAULT 0,
    retry_count     INTEGER DEFAULT 0
);

-- Controller 配置（云端下发）
CREATE TABLE controller_config (
    key             TEXT PRIMARY KEY,
    value           TEXT NOT NULL,
    updated_at      INTEGER NOT NULL
);
-- 配置示例：
-- { key: "offline_ttl_hours", value: "72" }
-- { key: "ble_tx_power", value: "medium" }
-- { key: "anti_replay_window_sec", value: "30" }
```

**同步策略：**

```
全量同步：  Controller 首次联网 / 每日凌晨 02:00（印尼时间）
增量同步：  云端变更后通过 WebSocket 实时推送（毫秒级）
冲突解决：  云端数据始终优先（以 updated_at 时间戳为准）
存储容量：  支持 50,000 条凭据本地存储（典型企业完全够用）
加密方式：  数据库文件 AES-256 加密（per-device key，设备证书生成时派生）
```

---

### B. BLE GATT Profile 定义

```
Primary Service UUID：自定义 128-bit UUID（避免与标准 UUID 冲突）

Characteristics：
┌─────────────────────┬──────────┬───────────────────────────────────────────┐
│ Characteristic      │ 属性      │ 用途                                       │
├─────────────────────┼──────────┼───────────────────────────────────────────┤
│ CONTROLLER_IDENTITY │ Read     │ Controller 公钥证书（手机验证 Controller 身份）│
│ CHALLENGE           │ Read     │ Controller 发送随机 Nonce_R（32 字节）       │
│ AUTH_REQUEST        │ Write    │ 手机写入 { User_ID, Nonce_P, Timestamp }    │
│ AUTH_RESPONSE       │ Write    │ 手机写入签名后的响应（Keystore 私钥签名）      │
│ AUTH_RESULT         │ Notify   │ Controller 通知认证结果（GRANTED/DENIED）    │
└─────────────────────┴──────────┴───────────────────────────────────────────┘

安全约束：
  - 所有特征值传输均在 BLE 加密连接（LE Secure Connections）上进行
  - 每次连接使用全新随机 Nonce，防止重放攻击
  - Controller 证书由云端 Root CA 签发，手机侧验证完整证书链
  - 握手超时：500ms 内未完成则断开连接并记录告警
```

---

### C. 安全等级视觉对比

```
凭据安全等级（由高至低）：

Apple Wallet (ECP2)    ████████████████████  硬件 SE（Secure Enclave）+ Apple 生态认证
HID Seos 移动版        ████████████████████  硬件 SE + CC EAL 认证
DeSFire EV3 实体卡     ██████████████████░░  卡片硬件 SE + NXP FIPS 认证
本方案 BLE（StrongBox）████████████████░░░░  Android StrongBox + 双向认证（自实现）
本方案 BLE（TEE）       ██████████████░░░░░░  Android TEE + 双向认证（自实现）
NFC HCE + 静态凭据     ████████░░░░░░░░░░░░  软件实现，安全天花板明显
动态二维码             █████░░░░░░░░░░░░░░░  云端 Token，适合低安全临时访客

本方案覆盖区间：DeSFire 卡（高）+ BLE StrongBox（中高）+ BLE TEE（中）+ QR（低）
通过三层凭据体系，覆盖印尼市场从工厂工人到企业白领的全部安全需求层次
```

---

### D. 术语说明

| 术语 | 说明 |
|---|---|
| VAS | Value Added Services，GSMA 定义的 NFC 移动凭据传输标准，Apple/Google 各自实现私有版本 |
| ECP2 | Enhanced Contactless Polling v2，苹果私有 NFC 轮询协议，读头需通过苹果独立认证 |
| HCE | Host Card Emulation，手机通过软件模拟 NFC 智能卡，凭据存在 CPU/TEE 侧（非专用 SE） |
| SE | Secure Element，专用硬件安全芯片，密钥物理隔离不可导出，苹果称 Secure Enclave |
| TEE | Trusted Execution Environment，CPU 内的可信执行环境，安全级别略低于独立 SE |
| StrongBox | Android 上的独立硬件安全芯片（部分高端机型支持），安全等级等同独立 SE |
| Seos | HID Global 自研的移动门禁凭据协议，基于 SIO（Secure Identity Object）数据模型 |
| SIO | Secure Identity Object，HID Seos 的凭据数据容器格式，经签名和加密 |
| OSDP | Open Supervised Device Protocol v2，读头与控制器间的双向加密通信标准 |
| Wiegand | 读头与控制器间的传统单向协议，无加密，仍是大量存量设备的主流接口 |
| ISAPI | Hikvision 设备开放 HTTP RESTful 接口协议（超过 1500 个接口） |
| ISUP | Hikvision 设备接入第三方云平台的协议，支持 NAT 穿透 |
| SCIM | System for Cross-domain Identity Management，企业身份同步标准协议（v2.0） |
| DeSFire | NXP MIFARE DeSFire，高安全 NFC 智能卡标准，使用 AES-256 加密 |
| TSM | Trusted Service Manager，管理手机 SE 内应用的云端可信服务，Apple/Google/HID 均有各自实现 |
| TTL | Time to Live，凭据有效期，到期须重新从云端获取 |
| PDPA | Personal Data Protection Act，印尼个人数据保护法（UU PDP No.27/2022） |
| BACnet | Building Automation and Control Networks，楼宇自动化标准协议 |
| PKI | Public Key Infrastructure，公钥基础设施，支撑数字证书签发和验证体系 |
| KMS | Key Management Service，密钥管理服务，主密钥托管于 HSM 硬件安全模块 |
| nRF52840 | Nordic Semiconductor 推出的 BLE 5.0 SoC，门禁行业主流 BLE 芯片，支持 ARM TrustZone |
