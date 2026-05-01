# Kisi vs MistyPass 差距分析

> 更新日期：2026-05-01
> 基于 Kisi API Bundled References (OpenAPI 3.1.0) + docs.kisi.io + kisi-image/ 截图

---

## 1. 完全覆盖的 Kisi 资源（~20 个）

Places, Locks, Users, Groups, Teams, Roles, Shares, Cards, Card Assignments, Event Sets, Events, Reports, Scheduled Reports, Integrations, Alert Policies, Floors, Group Links, Group Locks, Schedules, Holiday Calendars, Signed Upload URLs, Invites, Team Memberships, Guests

---

## 2. MistyPass 比 Kisi 多的功能（25+）

| 模块 | 说明 |
|---|---|
| 多租户架构 | Tenants CRUD + topology |
| Areas（子区域） | 楼层下的区域划分 |
| WebAuthn/Passkey | 无密码登录（Kisi 没有） |
| MFA 恢复码 | TOTP + 一次性恢复码 |
| 登录会话管理 | 查看/撤销活跃 session |
| 密码重置 | Token 自助重置流程 |
| SSO 联邦 | OIDC + SAML IdP 集成 |
| Enterprise 域名映射 | 邮箱域名 → 租户路由 |
| Enterprise HRIS | Talenta 等 HR 系统 webhook + DLQ |
| Enterprise JIT 审批 | 即时用户供给 + 审批流 |
| Enterprise 同步引擎 | Sync jobs/requests/workers + alerts |
| 告警系统 | 实时告警 + SSE 推送 |
| 告警策略 | 条件表达式 + 多渠道通知 |
| 审计日志 HMAC 链 | 防篡改审计 + Webhook 投递 |
| 事件流 SSE | 实时事件/告警推送 |
| 状态回放 | Event sourcing + checkpoint |
| Wallet 全生命周期 | Google/Apple Pass + 物理卡 + 任务队列 |
| Gateway Bootstrap 协议 | 注册/激活/配置同步/OTA |
| Gateway 序列号库存 | 硬件资产管理 |
| 移动端 App API | 居民端 BLE/QR 解锁 |
| 访客通行证 | Visitor Passes CRUD |
| 临时访问 | Temporary Access 独立资源 |
| 组织高级操作 | 审计导出/Webhook 轮换/禁用 |

---

## 3. Kisi 有但 MistyPass 完全缺失的（剩余 3 类）

| 缺失模块 | Kisi 端点数 | 影响 | 建议优先级 | 状态 |
|---|---|---|---|---|
| ~~Elevators~~ | ~~5~~ | ~~电梯门禁~~ | | ✅ done |
| ~~Elevator Stops~~ | ~~7~~ | ~~电梯楼层管控~~ | | ✅ done |
| ~~Group Elevator Stops~~ | ~~3~~ | ~~组-电梯楼层绑定~~ | | ✅ done |
| ~~Group Terminals~~ | ~~3~~ | ~~终端按组分配~~ | | ✅ done |
| ~~Presences~~ | ~~1~~ | ~~在场追踪~~ | | ✅ done |
| ~~CSV Card Imports~~ | ~~3~~ | ~~批量导卡~~ | | ✅ done |
| Cameras / Video | 6 | 摄像头集成（Rhombus 等） | P3 长期 | 待做 |
| Controller I/O (Inputs/Relays/Wiegands + Connections) | 18 | 控制器硬件接线 | P2 硬件 | 待做 |
| Wireless Locks | 1 | 无线锁列表 | P2 硬件 | 待做 |

---

## 4. 部分覆盖区域（已全部补齐 ✅）

| 资源 | 补齐端点 | 前端 | 状态 |
|---|---|---|---|
| Locks | favorite, unfavorite | ★ 按钮 | ✅ done |
| Places | favorite, unfavorite | ★ 按钮 | ✅ done |
| Members | POST, GET/{id}, PATCH/{id}, DELETE/{id} | 已有 CRUD UI | ✅ done |
| Reports | POST (create), DELETE/{id} | 已有列表/下载 | ✅ done |
| Group Zones | POST, GET/{id}, DELETE/{id} | 已有管理 UI | ✅ done |
| Readers | GET/{id}, PATCH/{id}, reset_tamper | Reset Tamper 按钮 | ✅ done |
| Controllers | GET/{id}, PATCH/{id} | 已有管理 UI | ✅ done |
| Terminals | POST, PUT/{id}, DELETE/{id} | 已有管理 UI | ✅ done |
| Cards | PATCH/{id}, DELETE/{id} | 已有编辑/删除 | ✅ done |
| Card Assignments | PATCH/{id}, DELETE/{id}, activate, deactivate | 已有管理 UI | ✅ done |

---

## 5. 剩余待做

### 依赖硬件/第三方
1. Controller I/O 全套（Inputs/Relays/Wiegands + Connections，18 个端点）— 依赖硬件
2. Wireless Locks（1 个端点）— 依赖硬件
3. Cameras / Video（6 个端点）— 依赖第三方集成

### 平台能力
4. Organization dashboard/transfers/certificates — 复杂
