# Lark (飞书) 集成方案

> 创建日期：2026-05-03
> 优先级：P1（工厂客户高度可能使用 Lark）
> API 文档：https://open.larksuite.com/document

---

## 1. 集成范围

| 功能 | Lark API | 方向 | 优先级 |
|------|---------|------|--------|
| Bot 消息通知 | Custom Bot Webhook | MistyPass → Lark | P1 |
| 员工目录同步 | Contact API v3 | Lark → MistyPass | P1 |
| 事件订阅 | Event Subscription | Lark → MistyPass | P1 |
| 审批流（访客） | Approval API v4 | 双向 | P2 |

---

## 2. 认证方式

Lark Open Platform 使用 **tenant_access_token**（企业自建应用）：

```
POST https://open.larksuite.com/open-apis/auth/v3/tenant_access_token/internal
{
  "app_id": "cli_xxx",
  "app_secret": "xxx"
}

Response:
{
  "tenant_access_token": "t-xxx",
  "expire": 7200
}
```

Token 有效期 2 小时，需自动刷新。

---

## 3. Bot 消息通知

### 3.1 Custom Bot Webhook（最简方案）

在 Lark 群组中添加自定义机器人，获取 Webhook URL：
```
POST https://open.larksuite.com/open-apis/bot/v2/hook/{webhook_id}
{
  "msg_type": "interactive",
  "card": { ... }
}
```

**触发场景：**
- 异常拒绝告警 → 安保群
- 门未关超时 → 安保群
- 访客到达 → 被访人私聊
- 员工 BLE 凭据到期提醒 → 员工私聊

### 3.2 消息卡片格式

```json
{
  "msg_type": "interactive",
  "card": {
    "header": {
      "title": { "tag": "plain_text", "content": "🚪 Access Alert" },
      "template": "red"
    },
    "elements": [
      {
        "tag": "div",
        "text": {
          "tag": "lark_md",
          "content": "**Door:** Main Entrance\n**Event:** Access Denied\n**User:** John Doe\n**Time:** 2026-05-03 14:30:00 WIB"
        }
      }
    ]
  }
}
```

---

## 4. 员工目录同步

### 4.1 全量同步

```
GET https://open.larksuite.com/open-apis/contact/v3/users
?department_id=0&page_size=50

Headers:
  Authorization: Bearer {tenant_access_token}
```

返回员工列表，包含 user_id、name、email、mobile、department 等。

### 4.2 增量同步（Event Subscription）

订阅事件：
- `contact.user.created_v3` — 新员工入职
- `contact.user.deleted_v3` — 员工离职
- `contact.user.updated_v3` — 信息变更

MistyPass 响应：
- 入职 → 创建 access_user + 发送凭据注册邀请
- 离职 → 吊销所有 BLE 凭据 + 停用门禁权限
- 变更 → 更新用户信息

---

## 5. 事件订阅（Lark → MistyPass）

Lark 推送事件到 MistyPass 注册的回调 URL：

```
POST /api/v1/integrations/lark/events
Content-Type: application/json

{
  "schema": "2.0",
  "header": {
    "event_id": "xxx",
    "event_type": "contact.user.created_v3",
    "create_time": "1234567890",
    "token": "verification_token",
    "app_id": "cli_xxx",
    "tenant_key": "xxx"
  },
  "event": { ... }
}
```

**安全验证：**
- 首次订阅时 Lark 发送 challenge 验证
- 后续事件通过 Encrypt Key 签名验证

---

## 6. 配置环境变量

```bash
# Lark 应用凭据
LARK_APP_ID=cli_xxxxxxxxxx
LARK_APP_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxx

# Lark Bot Webhook（每个群组一个）
LARK_WEBHOOK_SECURITY=security_group_webhook_id
LARK_WEBHOOK_OPS=ops_group_webhook_id

# Lark 事件订阅
LARK_ENCRYPT_KEY=xxxxxxxxxx
LARK_VERIFICATION_TOKEN=xxxxxxxxxx

# 同步策略
LARK_SYNC_ENABLED=true
LARK_SYNC_INTERVAL=300  # 全量同步间隔（秒），默认 5 分钟
```

---

## 7. 实现文件清单

| 文件 | 用途 |
|------|------|
| `api/internal/modules/integration/lark_client.go` | Lark API 客户端（token 管理、HTTP 封装） |
| `api/internal/modules/integration/lark_bot.go` | Bot 消息发送（Webhook + 卡片模板） |
| `api/internal/modules/integration/lark_contact.go` | 员工目录同步（全量 + 增量） |
| `api/internal/http/routes_integration_lark.go` | 事件回调端点 + 管理端点 |
