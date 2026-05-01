# MistyPass vs Kisi 架构对照分析

> 对照日期：2026-05-01
> Kisi 文档来源：docs.kisi.io (System Architecture / API Quick Start / Analytics & Reporting)

---

## 1. 系统架构总览

| 维度 | Kisi | MistyPass | 差距/优势 |
|------|------|-----------|----------|
| **云平台** | Google Cloud Platform，区域故障转移，热备数据库 | 自托管（PostgreSQL + Redis + NATS），Docker Compose 部署 | Kisi 有 GCP 级别 HA；MistyPass 可私有化部署，无供应商锁定 |
| **架构模式** | 云优先（Cloud-first），设备主动连接云端 | 边缘计算优先（Edge-first），网关缓存规则本地决策 | MistyPass 离线能力更强 |
| **Web 防火墙** | GCP WAF | 暂无（Caddy 反代 + 速率限制） | 需补：生产部署建议加 Cloudflare/WAF |
| **高可用** | 热备数据库 + 区域故障转移 | 单实例 + PostgreSQL + Redis | 需补：生产部署需配 PG 主从 + Redis Sentinel |

---

## 2. 硬件设备对比

### Kisi 硬件

| 设备 | 连接方式 | 用途 |
|------|---------|------|
| **Reader Pro 1/2** | PoE (10/100 Mbps) / WiFi 2.4/5 GHz | 读卡器，发起出站连接 |
| **Controller Pro 1/2** | 以太网 / WiFi | 控制器，管理本地读卡器同步 |

- Reader ↔ Controller 通信：**AES 加密 UDP（端口 62435）**
- 设备 ↔ 云端：**自签名固件绑定证书**，仅出站连接

### MistyPass 硬件

| 设备 | 连接方式 | 用途 |
|------|---------|------|
| **Gateway（Orange Pi / RPi）** | 以太网 / WiFi | 读卡器 + 控制器合一 |
| **NFC 读卡器（ACS WalletMate II）** | USB PC/SC | NFC/ISO 14443 读取 |
| **RS485 继电器模块** | RS485 Modbus RTU | 多路锁控制 |
| **GPIO 继电器** | GPIO 直连 | 单路锁控制 |

### 对照分析

| 维度 | Kisi | MistyPass | 评价 |
|------|------|-----------|------|
| 读卡器-控制器分离 | 分离（Reader + Controller 独立设备） | 合一（Gateway 集成读卡+控制） | Kisi 架构更灵活，适合大型部署；MistyPass 更简单，单设备搞定 |
| 读卡器加密通信 | AES 加密 UDP | USB 直连（无需网络加密） | 不同架构，各有优势 |
| 设备证书 | 自签名 + 固件绑定 | TLS SPKI 证书锁定 | MistyPass 用标准 TLS pinning，安全性相当 |
| 多门控制 | 一个 Controller Pro 管多个 Reader | 一个 Gateway + RS485 多路继电器 | 均支持 |
| 锁类型 | 电磁锁、电插锁 | 电磁锁、电插锁、电子锁、螺线管锁 | MistyPass 支持更多类型 |
| 后备电源 | 建议 UPS（文档提及） | 未提及 | 需补：硬件集成指南加 UPS 建议 |

---

## 3. 通信协议对比

### Kisi 端口和协议

| 端口 | 协议 | 用途 |
|------|------|------|
| TCP 31314 | 自有协议 | 设备-服务器主连接 |
| TCP 993 | 自有协议 | 备用连接通道 |
| TCP 443 | HTTPS | 固件更新 + API |
| TCP 80 | HTTP | 固件下载备用 |
| UDP 53 | DNS | 域名解析 |
| UDP 62435 | AES-UDP | Reader ↔ Controller 同步 |

### MistyPass 端口和协议

| 端口 | 协议 | 用途 |
|------|------|------|
| TCP 443 | HTTPS (TLS 1.2+) | API + 网关通信 |
| TCP 4222 | NATS | 实时命令下发（解锁、锁定） |
| TCP 1883 | MQTT（可选） | IoT 事件总线 |
| TCP 6379 | Redis | 会话/缓存 |
| TCP 5432 | PostgreSQL | 数据库 |

### 对照分析

| 维度 | Kisi | MistyPass | 评价 |
|------|------|-----------|------|
| 实时命令 | TCP 31314（自有长连接） | NATS（标准消息队列） | MistyPass 用标准协议，更易调试和扩展 |
| 备用通道 | TCP 993（IMAP 端口，穿透防火墙） | 无备用 | **需补**：可考虑 WebSocket 443 备用 |
| 固件更新 | HTTPS 443 + HTTP 80 备用 | HTTPS 签名 URL | 相当 |
| 防火墙友好 | 设备仅出站连接 | 网关仅出站连接 | 相同策略 |
| 加密通道 | 自签名证书 + 固件绑定 | TLS + SPKI pin + nonce 防重放 | MistyPass 更标准化 |

---

## 4. API 设计对比

### Kisi API

| 维度 | 详情 |
|------|------|
| 认证方式 | API Key |
| Base URL | `https://api.getkisi.com` |
| 文档格式 | 独立文档站 |
| SDK | 未提及 |
| 速率限制 | 有（未公开具体数字） |

### MistyPass API

| 维度 | 详情 |
|------|------|
| 认证方式 | JWT Bearer Token（Access + Refresh） |
| Base URL | `https://{host}/api/v1` |
| 文档格式 | OpenAPI 3.0.3 自动生成（`GET /api/v1/openapi.json`） |
| SDK | 无（TypeScript 客户端在 `web-admin/src/lib/api.ts`） |
| 速率限制 | 登录 10/min/IP，API 600/min/IP，企业 60/min/IP |

### 对照分析

| 维度 | Kisi | MistyPass | 评价 |
|------|------|-----------|------|
| 认证安全性 | API Key（长期有效） | JWT + Refresh Token + MFA + WebAuthn | MistyPass 显著更安全 |
| API 文档 | 独立站 + Swagger | 内嵌 OpenAPI 3.0.3 | 相当 |
| 快速开始体验 | 5 步指引 | Demo 账号 + 一键登录 | **需补**：写一份 Quick Start 文档 |
| 分页/过滤 | 有（具体未公开） | 有（offset/limit + X-Collection-Range） | 相当 |
| Webhook | 有 | 有（签名 + 轮换） | 相当 |

---

## 5. 分析和报表对比

### Kisi Insights（6 种报表）

| 报表 | 说明 | MistyPass 对应 |
|------|------|---------------|
| **Hardware Summary** | 设备概览（Device ID、型号、状态、MAC 地址） | `GET /gateways` + `GET /readers` + `GET /controllers`（已有） |
| **Network Visualization** | 网络拓扑、连接状态、设备依赖 | **缺失** — 需前端可视化 |
| **Time Tracking** | 首次解锁、末次解锁、在场时长 | `GET /presences`（已有基础版） |
| **Unlock Statistics & Trends** | 独立用户解锁数、总解锁数、失败次数，35 天趋势 + 6 个月对比 | `GET /analytics/access-summary`（已有，支持自定义时间范围） |
| **User Presence Reports** | 出勤模式、用户名、邮箱、按星期解锁频率 | `GET /analytics/door-activity`（已有 hourly_distribution） |
| **Weekly Place Analytics** | 每日使用模式、用户热力图、凭证类型分布、Top 门、失败解锁、设备在线率 | `GET /analytics/access-summary` + `GET /analytics/door-activity`（部分覆盖） |

### 对照分析

| 维度 | Kisi | MistyPass | 评价 |
|------|------|-----------|------|
| 报表种类 | 6 种固定报表 | 3 个 API 端点（灵活查询） | API 更灵活但缺少预制报表 UI |
| 自动邮件推送 | 支持（最长 180 天间隔） | **缺失** | 需补：定时报表邮件 |
| PDF 导出 | 支持 | **缺失** | 需补：报表 PDF 生成 |
| 数据范围 | 35 天趋势 + 6 个月对比 | 无限制（取决于事件存储） | MistyPass 更灵活 |
| 权限控制 | 组织管理员 / Manager / Observer | RBAC 4 级角色均可访问 | 相当 |
| 自定义报表 | 需联系支持 | API 灵活查询 | MistyPass 更开放 |
| 网络拓扑可视化 | 有 | **缺失** | 需前端组件 |

---

## 6. 安全架构对比

| 维度 | Kisi | MistyPass | 评价 |
|------|------|-----------|------|
| 设备证书 | 自签名 + 固件绑定 | TLS SPKI 证书锁定 | 相当（不同实现，同等安全级别） |
| 出站连接策略 | 设备仅出站 | 网关仅出站 | 相同 |
| WiFi 安全 | WPA2/WPA3-PSK（不支持 802.1X） | WPA2/WPA3（由系统管理） | 相同 |
| API 认证 | API Key | JWT + MFA + WebAuthn + SSO | MistyPass 显著更强 |
| 数据加密 | 未公开细节 | AES-256-GCM（HKDF 密钥派生 + 版本化轮换） | MistyPass 明确且可审计 |
| 审计日志 | 有 | HMAC 链式完整性校验 | MistyPass 更强（防篡改） |
| 合规文档 | getkisi.com/security（需 NDA 看架构图） | 开源架构文档 | MistyPass 更透明 |

---

## 7. 总结：需补齐的差距

### 高优先级（影响用户体验）

| # | 差距 | 说明 | 工作量 |
|---|------|------|--------|
| 1 | **Quick Start 文档** | Kisi 有 5 步快速开始指引，MistyPass 缺少 | 0.5 天 |
| 2 | **报表自动邮件推送** | Kisi 支持定时发送报表邮件 | 2 天 |
| 3 | **报表 PDF 导出** | Kisi 支持 PDF 下载 | 1-2 天 |
| 4 | **网络拓扑可视化** | Kisi 有网络设备依赖关系图 | 2-3 天（前端） |

### 中优先级（架构完善）

| # | 差距 | 说明 | 工作量 |
|---|------|------|--------|
| 5 | **备用通信通道** | Kisi 用 TCP 993 穿透防火墙，MistyPass 仅 HTTPS+NATS | 1 天 |
| 6 | **UPS 电源指南** | Kisi 文档有 UPS 建议 | 0.5 天（文档） |
| 7 | **生产高可用部署指南** | Kisi 有 GCP 级 HA，MistyPass 需文档化部署方案 | 1 天（文档） |

### MistyPass 优势（Kisi 不具备）

| # | 优势 | 说明 |
|---|------|------|
| 1 | **私有化部署** | 可完全自托管，无供应商锁定 |
| 2 | **边缘计算** | 网关本地缓存规则，离线决策能力更强 |
| 3 | **标准协议** | NATS/MQTT 替代自有协议，易于集成和调试 |
| 4 | **多因素认证** | JWT + MFA + WebAuthn + SSO（Kisi 仅 API Key） |
| 5 | **HMAC 链式审计** | 审计日志防篡改（Kisi 未公开此级别） |
| 6 | **密钥版本化轮换** | AES-256-GCM + HKDF + 自动重加密 |
| 7 | **开放 API 分析** | 无限时间范围查询，无需联系支持定制报表 |
| 8 | **硬件灵活性** | 支持 GPIO/RS485/USB 多种接口，不绑定专用硬件 |
