# MistyPass Quick Start Guide / 快速上手指南

> Get MistyPass running locally in 10 minutes.
>
> 10 分钟在本地跑通 MistyPass。

---

## Prerequisites / 前置条件

| Tool | Version | Note |
|------|---------|------|
| Docker & Docker Compose | 24+ | Infrastructure (Postgres, Redis, NATS, EMQX) / 基础设施 |
| Go | 1.22+ | API server / API 服务器 |
| Node.js | 20+ | Admin UI (uses pnpm) / 管理后台 |
| pnpm | 9+ | `corepack enable && corepack prepare pnpm@latest --activate` |
| curl | any | Testing API calls / 测试 API 调用 |

---

## Step 1: Start Infrastructure / 启动基础设施

```bash
git clone https://github.com/your-org/MistyPass.git
cd MistyPass

# Start Postgres, PgBouncer, Redis, NATS, EMQX
# 启动 Postgres, PgBouncer, Redis, NATS, EMQX
docker compose up -d postgres pgbouncer redis nats emqx
```

Wait for health checks to pass / 等待健康检查通过:

```bash
docker compose ps
```

You should see `postgres` and `redis` as **healthy**.

| Service / 服务 | Port / 端口 | Purpose / 用途 |
|---|---|---|
| Postgres | 5432 | Primary database / 主数据库 |
| PgBouncer | 6432 | Connection pooling / 连接池 |
| Redis | 6379 | Caching, rate limiting / 缓存与限流 |
| NATS | 4222 | Event bus (JetStream) / 事件总线 |
| EMQX | 1883 | MQTT broker for gateways / 网关 MQTT |

---

## Step 2: Start the API / 启动 API 服务器

**Option A: Run directly with Go / 直接用 Go 运行**

```bash
cd api

export APP_ENV=development
export ENABLE_DEMO_USERS=true
export PORT=8080
export CORS_ORIGIN=http://localhost:5173
export DATABASE_URL="postgres://postgres:mistypass-dev-postgres-local-only-20260424@localhost:5432/mistypass?sslmode=disable"
export DATABASE_AUTO_MIGRATE=true
export GATEWAY_BOOTSTRAP_TOKEN=mistypass-dev-bootstrap-local-only-20260424
export REDIS_ADDR=localhost:6379
export REDIS_PASSWORD=mistypass-dev-redis-local-only-20260424
export REDIS_DB=0
export REDIS_KEY_PREFIX=mistypass
export NATS_ENABLED=true
export NATS_SERVER_URL=nats://localhost:4222
export NATS_SUBJECT_PREFIX=mistypass

go run ./cmd/server
```

**Option B: Run via Docker Compose / 用 Docker Compose 运行**

```bash
ENABLE_DEMO_USERS=true docker compose up -d api
```

Verify the API is running / 验证 API 已启动:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

---

## Step 3: Start the Admin UI / 启动管理后台

```bash
cd web-admin
pnpm install
pnpm run dev
```

Open **http://localhost:5173** in your browser / 在浏览器打开。

---

## Step 4: Log In and Explore / 登录并探索

### Demo Account / 演示账号

| Field / 字段 | Value / 值 |
|---|---|
| Email | `organization.admin@mistypass.local` |
| Password / 密码 | `admin123` |

Log in at **http://localhost:5173**. As an organization admin (`tenant_admin`) you have full access to:

以 `tenant_admin` 角色登录后，你可以操作:

- **Places / 场所** --- buildings, floors, areas / 楼宇、楼层、区域
- **Doors & Locks / 门与锁** --- create, unlock, lockdown / 新增、开锁、锁定
- **Gateways / 网关** --- register hardware, bind to doors / 注册硬件、绑定到门
- **Users & Groups / 用户与组** --- invite members, assign access / 邀请成员、分配权限
- **Access Policies / 访问策略** --- time-based schedules, holiday calendars / 按时间表、假日日历
- **Audit Logs / 审计日志** --- full history of all operations / 所有操作的完整记录

### Role Hierarchy / 角色层级

| Role | Description / 说明 |
|---|---|
| `super_admin` | Platform-level, manages all tenants / 平台级，管理所有租户 |
| `tenant_admin` | Organization-level, full access / 组织级，完全权限 |
| `building_admin` | Scoped to specific buildings / 限定到特定楼宇 |
| `operator` | Read-only operations view / 运维只读视图 |
| `resident` | End-user, mobile app access / 终端用户，移动端 |

---

## Step 5: Make Your First API Call / 发起第一个 API 调用

### 5.1 Login / 登录

```bash
# Login and save the token
# 登录并保存 token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "organization.admin@mistypass.local",
    "password": "admin123"
  }' | jq -r '.token')

echo $TOKEN
```

### 5.2 Get Current User / 获取当前用户

```bash
curl -s http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer $TOKEN" | jq
```

### 5.3 Create a Building / 创建楼宇

```bash
curl -s -X POST http://localhost:8080/api/v1/buildings \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "HQ Tower",
    "address": "100 Main St",
    "timezone": "Asia/Shanghai"
  }' | jq
```

### 5.4 List Places / 列出场所

```bash
curl -s http://localhost:8080/api/v1/places \
  -H "Authorization: Bearer $TOKEN" | jq
```

Save a place ID for the next step / 记下一个 place ID 用于下一步:

```bash
PLACE_ID=$(curl -s http://localhost:8080/api/v1/places \
  -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')
```

### 5.5 Create a Lock (Door) / 创建门锁

```bash
curl -s -X POST http://localhost:8080/api/v1/locks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"Main Entrance\",
    \"place_id\": \"$PLACE_ID\"
  }" | jq
```

Save the lock ID / 记下锁 ID:

```bash
LOCK_ID=$(curl -s http://localhost:8080/api/v1/locks \
  -H "Authorization: Bearer $TOKEN" | jq -r '.[0].id')
```

### 5.6 Unlock a Lock / 远程开锁

```bash
curl -s -X POST "http://localhost:8080/api/v1/locks/$LOCK_ID/unlock" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" | jq
```

### 5.7 View the OpenAPI Spec / 查看 OpenAPI 文档

```bash
curl -s http://localhost:8080/api/v1/openapi.json | jq '.info'
```

---

## Step 6: Set Up a Gateway / 注册网关

Gateways are the hardware controllers that connect physical locks to the cloud.

网关是连接物理门锁与云端的硬件控制器。

### 6.1 Register a Gateway (Bootstrap Flow) / 注册网关 (引导流程)

The gateway uses a bootstrap token to self-register with the cloud.

网关使用引导令牌向云端自注册。

```bash
# Gateway calls this endpoint on first boot
# 网关首次启动时调用此接口
curl -s -X POST http://localhost:8080/api/v1/gateway/register \
  -H "Content-Type: application/json" \
  -H "X-Bootstrap-Token: mistypass-dev-bootstrap-local-only-20260424" \
  -d '{
    "serial_number": "GW-TEST-001",
    "firmware_version": "1.0.0",
    "hardware_model": "mistypass-gw-v1"
  }' | jq
```

### 6.2 Activate the Gateway / 激活网关

```bash
GATEWAY_TOKEN="<token from register response>"

curl -s -X POST http://localhost:8080/api/v1/gateway/activate \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $GATEWAY_TOKEN" \
  -d '{
    "serial_number": "GW-TEST-001"
  }' | jq
```

### 6.3 Bind Gateway to a Door / 将网关绑定到门

```bash
GATEWAY_ID="<gateway ID from registration>"

curl -s -X POST "http://localhost:8080/api/v1/gateways/$GATEWAY_ID/bind-door" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"door_id\": \"$LOCK_ID\"
  }" | jq
```

### 6.4 Gateway Heartbeat / 网关心跳

```bash
# Gateway sends periodic heartbeats
# 网关定期发送心跳
curl -s -X POST http://localhost:8080/api/v1/gateway/heartbeat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $GATEWAY_TOKEN" \
  -d '{
    "serial_number": "GW-TEST-001",
    "uptime_seconds": 3600,
    "firmware_version": "1.0.0"
  }' | jq
```

---

## Environment Variables Reference / 环境变量参考

| Variable | Default | Description / 说明 |
|---|---|---|
| `APP_ENV` | `development` | Environment mode / 运行环境 |
| `PORT` | `8080` | API listen port / API 监听端口 |
| `CORS_ORIGIN` | `http://localhost:5173` | Allowed CORS origin / 允许的跨域来源 |
| `ENABLE_DEMO_USERS` | `false` | Seed demo accounts on startup / 启动时创建演示账号 |
| `DATABASE_URL` | --- | PostgreSQL connection string / PostgreSQL 连接串 |
| `DATABASE_AUTO_MIGRATE` | `true` | Auto-run schema migrations / 自动执行数据库迁移 |
| `GATEWAY_BOOTSTRAP_TOKEN` | --- | Shared secret for gateway self-registration / 网关自注册共享密钥 |
| `REDIS_ADDR` | `localhost:6379` | Redis address / Redis 地址 |
| `REDIS_PASSWORD` | --- | Redis password / Redis 密码 |
| `REDIS_DB` | `0` | Redis database number / Redis 数据库编号 |
| `REDIS_KEY_PREFIX` | `mistypass` | Redis key namespace / Redis 键命名空间 |
| `NATS_ENABLED` | `true` | Enable NATS event bus / 启用 NATS 事件总线 |
| `NATS_SERVER_URL` | `nats://localhost:4222` | NATS server URL / NATS 服务器地址 |
| `NATS_SUBJECT_PREFIX` | `mistypass` | NATS subject namespace / NATS 主题命名空间 |
| `POSTGRES_PASSWORD` | (dev default) | Postgres password for Docker / Docker 用 Postgres 密码 |
| `REDIS_PASSWORD` | (dev default) | Redis password for Docker / Docker 用 Redis 密码 |
| `EMQX_DASHBOARD_USERNAME` | `admin` | EMQX dashboard login / EMQX 控制台用户名 |
| `EMQX_DASHBOARD_PASSWORD` | (dev default) | EMQX dashboard password / EMQX 控制台密码 |

---

## Troubleshooting / 常见问题

**Port already in use / 端口被占用**

```bash
# Check what's using port 8080
lsof -i :8080
```

**Database connection refused / 数据库连接被拒绝**

Make sure Postgres is healthy before starting the API:
确保 API 启动前 Postgres 已健康:

```bash
docker compose ps postgres
# Should show "healthy"
```

**Reset everything / 重置全部数据**

```bash
docker compose down -v   # removes volumes (all data)
docker compose up -d postgres pgbouncer redis nats emqx
```

---

## What's Next / 接下来

- Read the [Gateway Cloud Protocol](architecture/gateway-cloud-protocol.md) to understand hardware communication
- Browse the OpenAPI spec at `http://localhost:8080/api/v1/openapi.json`
- See the [Roadmap](NEXT-ROADMAP.md) for upcoming features

---

> **MistyPass** --- easier, safer, more efficient door access.
>
> **MistyPass** --- 让开门更方便、更安全、更高效。
