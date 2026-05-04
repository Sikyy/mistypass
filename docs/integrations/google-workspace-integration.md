# Google Workspace 集成方案

> 创建日期：2026-05-03
> 优先级：P1（印尼中大型企业标配）
> API 文档：https://developers.google.com/admin-sdk

---

## 1. 集成范围

| 功能 | Google API | 方向 | 优先级 |
|------|-----------|------|--------|
| 员工目录同步 | Directory API (Admin SDK) | Google → MistyPass | P1 |
| 日历联动 | Calendar API | Google → MistyPass | P2 |
| SSO 登录 | OpenID Connect | Google → MistyPass | ✅ 已完成（OIDC） |

---

## 2. 认证方式

使用 **Service Account** + Domain-Wide Delegation：

1. 在 Google Cloud Console 创建 Service Account
2. 启用 Admin SDK API
3. 在 Google Admin Console 授予域全委托权限
4. 使用 Service Account JSON Key 签署 JWT

```bash
GOOGLE_WORKSPACE_SERVICE_ACCOUNT_KEY=/secrets/gws-service-account.json
GOOGLE_WORKSPACE_DELEGATED_ADMIN=admin@company.com
GOOGLE_WORKSPACE_DOMAIN=company.com
GOOGLE_WORKSPACE_SYNC_ENABLED=true
```

---

## 3. 员工目录同步

```
GET https://admin.googleapis.com/admin/directory/v1/users
?domain=company.com&maxResults=100&orderBy=email

Headers:
  Authorization: Bearer {access_token}
```

### 同步映射

| Google Workspace 字段 | MistyPass 字段 |
|----------------------|----------------|
| primaryEmail | email |
| name.fullName | name |
| id | external_id |
| orgUnitPath | department |
| suspended | status → "inactive" |
| creationTime | created_at |

### 触发条件

- 定时全量同步（每 6 小时）
- Push Notification（Google Directory Push API）注册 webhook

---

## 4. 实现文件

| 文件 | 用途 |
|------|------|
| `api/internal/modules/integration/google_workspace.go` | Directory API 客户端 |
| `api/internal/http/routes_integration_google.go` | Push notification 回调 |
