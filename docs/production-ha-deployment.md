# MistyPass 生产环境高可用 (HA) 部署指南

> 版本：1.0 | 最后更新：2026-05-03
>
> 本文档覆盖 MistyPass 全组件的生产级高可用部署方案，包括架构设计、配置片段、
> Kubernetes 清单和运维流程。目标区域为印度尼西亚（GCP asia-southeast2 / AWS ap-southeast-1）。

---

## 目录

1. [架构总览](#1-架构总览)
2. [PostgreSQL 高可用](#2-postgresql-高可用)
3. [Redis 高可用](#3-redis-高可用)
4. [NATS 高可用](#4-nats-高可用)
5. [EMQX 高可用](#5-emqx-高可用)
6. [API 服务器](#6-api-服务器)
7. [Gateway Agent 边缘部署](#7-gateway-agent-边缘部署)
8. [TLS 与证书管理](#8-tls-与证书管理)
9. [可观测性](#9-可观测性)
10. [备份与恢复](#10-备份与恢复)
11. [印度尼西亚区域部署](#11-印度尼西亚区域部署)
12. [Docker Compose 生产配置](#12-docker-compose-生产配置)
13. [Systemd 单元文件](#13-systemd-单元文件)
14. [Kubernetes 清单](#14-kubernetes-清单)
15. [上线检查清单](#15-上线检查清单)

---

## 1. 架构总览

MistyPass 生产部署由六个核心层组成：

| 层 | 组件 | 角色 |
|---|------|------|
| 入口层 | Caddy / Nginx / 云 LB | TLS 终止、反向代理、限流 |
| 应用层 | Go API 服务器 (`:8080`) x 2+ | 无状态 REST API，所有状态存储在 PG/Redis |
| 数据层 | PostgreSQL 16 + PGBouncer | 持久化存储，主-从流复制 |
| 缓存层 | Redis 7 (Sentinel x 3) | 会话、速率限制、临时缓存 |
| 消息层 | NATS 2.10 (JetStream x 3) | 内部消息总线、事件持久化 |
| MQTT 层 | EMQX 5.10 (集群 x 2+) | Gateway 设备 MQTT 通信 |

### 架构拓扑图

```
                              互联网
                                |
                         [ 云负载均衡 ]
                         TLS 终止 / WAF
                        /              \
                 +----------+    +----------+
                 |  API-1   |    |  API-2   |
                 | :8080    |    | :8080    |
                 +----------+    +----------+
                    |   \          /   |
                    |    \        /    |
     +--------------+-----+------+----+---------------+
     |              |            |                     |
+---------+  +-----------+  +----------+  +---------+ +---------+
|   PG    |  |    PG     |  |  Redis   |  |  NATS   | |  EMQX   |
| Primary |  |  Replica  |  | Sentinel |  | Cluster | | Cluster |
|+PGBouncer| |+PGBouncer |  |  (3 节点) |  | (3 节点) | | (2+节点) |
+---------+  +-----------+  +----------+  +---------+ +---------+
                                                          |
                                                    [ MQTT/TLS ]
                                                          |
                                                  +---------------+
                                                  | Gateway Agent |
                                                  |  (边缘设备)    |
                                                  | Orange Pi 等   |
                                                  +---------------+
                                                          |
                                                   [ 门禁读卡器 ]
                                                   [ 继电器/门锁 ]
```

### 最小生产部署（3 节点）

```
节点 1: API-1, PG Primary, PGBouncer, Redis-1, Sentinel-1, NATS-1, EMQX-1
节点 2: API-2, PG Replica, PGBouncer, Redis-2, Sentinel-2, NATS-2, EMQX-2
节点 3: PGBouncer (见证), Redis-3, Sentinel-3, NATS-3
```

大规模部署时应将数据库节点、消息中间件节点和应用节点分离。

### 数据流

```
用户/管理端 → HTTPS → API → PGBouncer → PostgreSQL
                      ↕          ↕
                    Redis      NATS ← JetStream 持久化
                      ↕          ↕
                    EMQX ←→ Gateway Agent ←→ 门禁硬件
```

---

## 2. PostgreSQL 高可用

### 2.1 方案选择

| 方案 | 适用场景 | 运维成本 |
|------|---------|---------|
| Patroni + 流复制 | 自建服务器 / 裸机 / VM | 高 |
| GCP Cloud SQL (推荐) | GCP 部署 | 低 |
| AWS RDS for PostgreSQL | AWS 部署 | 低 |
| Supabase | 早期 MVP | 低 |

**推荐**：印度尼西亚市场首选 GCP Cloud SQL (asia-southeast2 雅加达区域) 或自建 Patroni。

### 2.2 自建：Patroni + 流复制

#### postgresql.conf (主库)

```ini
# 复制设置
wal_level = replica
max_wal_senders = 10
wal_keep_size = 2GB
synchronous_commit = on
synchronous_standby_names = 'ANY 1 (pg_replica_1)'

# 性能调优 (8GB RAM 服务器)
shared_buffers = 2GB
effective_cache_size = 6GB
maintenance_work_mem = 512MB
work_mem = 16MB
max_connections = 200
wal_buffers = 64MB
checkpoint_completion_target = 0.9
random_page_cost = 1.1              # SSD
effective_io_concurrency = 200      # SSD

# 日志
log_min_duration_statement = 500    # 慢查询阈值 (ms)
log_checkpoints = on
log_connections = on
log_disconnections = on
log_lock_waits = on
```

#### pg_hba.conf (复制授权)

```
# 允许从库流复制连接
host    replication     replicator      10.0.0.0/8      scram-sha-256
host    replication     replicator      172.16.0.0/12   scram-sha-256
```

#### PGBouncer 配置

每个 PG 节点部署一个 PGBouncer 实例，API 服务器连接 PGBouncer (端口 6432) 而非直连 PG。

```ini
; pgbouncer.ini
[databases]
mistypass = host=127.0.0.1 port=5432 dbname=mistypass auth_user=pgbouncer

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
auth_type = scram-sha-256
auth_file = /etc/pgbouncer/userlist.txt

; 连接池设置
pool_mode = transaction
max_client_conn = 500
default_pool_size = 30
min_pool_size = 5
reserve_pool_size = 5
reserve_pool_timeout = 3

; 超时
server_idle_timeout = 600
server_lifetime = 3600
client_idle_timeout = 0
query_timeout = 30
query_wait_timeout = 120

; 日志
log_connections = 1
log_disconnections = 1
log_pooler_errors = 1
stats_period = 60

; TLS (内部通信)
; server_tls_sslmode = require
; server_tls_ca_file = /etc/pgbouncer/ca.crt
```

#### 故障转移行为

- Patroni 检测主库故障约 30 秒内完成。
- 从库自动提升为主库。
- PGBouncer 连接短暂中断 (< 5 秒)。
- API 服务器重试失败查询；无状态设计意味着无会话丢失。

### 2.3 托管服务：GCP Cloud SQL

```bash
# 创建高可用 Cloud SQL 实例 (雅加达区域)
gcloud sql instances create mistypass-db \
  --database-version=POSTGRES_16 \
  --tier=db-custom-4-16384 \
  --region=asia-southeast2 \
  --availability-type=REGIONAL \
  --storage-type=SSD \
  --storage-size=50GB \
  --storage-auto-increase \
  --backup-start-time=02:00 \
  --enable-point-in-time-recovery \
  --retained-backups-count=30 \
  --maintenance-window-day=SUN \
  --maintenance-window-hour=4 \
  --database-flags=\
log_min_duration_statement=500,\
max_connections=200,\
shared_buffers=4194304

# 创建数据库和用户
gcloud sql databases create mistypass --instance=mistypass-db
gcloud sql users create mistypass_app --instance=mistypass-db \
  --password='<STRONG_PASSWORD>'
```

使用 Cloud SQL 时，PGBouncer 仍然建议在应用层部署以管理连接池。可使用 Cloud SQL Auth Proxy 或直接配置 PGBouncer 指向 Cloud SQL 的私有 IP。

---

## 3. Redis 高可用

### 3.1 Redis Sentinel (自建)

部署 3 个 Redis 实例 + 3 个 Sentinel 进程实现自动故障转移。

#### redis.conf (每个节点)

```conf
# 通用配置
bind 0.0.0.0
port 6379
requirepass <REDIS_PASSWORD>
masterauth <REDIS_PASSWORD>

# 持久化
appendonly yes
appendfsync everysec
save 900 1
save 300 10
save 60 10000

# 内存
maxmemory 1gb
maxmemory-policy allkeys-lru

# 安全
rename-command FLUSHDB ""
rename-command FLUSHALL ""
rename-command DEBUG ""

# 慢日志
slowlog-log-slower-than 10000
slowlog-max-len 128
```

#### sentinel.conf (每个 Sentinel 节点)

```conf
port 26379
sentinel monitor mistypass-redis <PRIMARY_IP> 6379 2
sentinel down-after-milliseconds mistypass-redis 5000
sentinel failover-timeout mistypass-redis 10000
sentinel parallel-syncs mistypass-redis 1
sentinel auth-pass mistypass-redis <REDIS_PASSWORD>

# Sentinel 自身密码 (可选但推荐)
requirepass <SENTINEL_PASSWORD>
```

**仲裁机制**：2/3 Sentinel 同意后触发自动故障转移。API 服务器使用 Sentinel 感知的 Redis 客户端自动发现当前主节点。

#### MistyPass 在 Redis 中存储的数据

| 键模式 | 用途 | TTL |
|--------|------|-----|
| `mistypass:session:<id>` | 用户会话 | 24 小时 |
| `mistypass:rate:<ip>` | 速率限制计数器 | 60 秒 |
| `mistypass:nonce:<val>` | 请求去重 nonce | 300 秒 |

**重要**：Redis 数据丢失仅导致用户需要重新登录，不会造成数据丢失。PostgreSQL 是唯一权威数据源。

### 3.2 托管服务

| 服务 | 区域 | 备注 |
|------|------|------|
| GCP Memorystore for Redis | asia-southeast2 | 标准版支持自动故障转移 |
| AWS ElastiCache for Redis | ap-southeast-1 | Multi-AZ 部署 |

```bash
# GCP Memorystore 创建
gcloud redis instances create mistypass-redis \
  --size=1 \
  --region=asia-southeast2 \
  --tier=STANDARD_HA \
  --redis-version=redis_7_0 \
  --redis-config=maxmemory-policy=allkeys-lru
```

---

## 4. NATS 高可用

### 4.1 三节点 JetStream 集群

NATS 提供 API 与 Gateway 之间的实时通信。JetStream 为事件投递提供持久化保证。

#### nats-server.conf (节点 1)

```hcl
server_name: nats-1
listen: 0.0.0.0:4222
http_port: 8222

# 授权
authorization {
  token: "<NATS_AUTH_TOKEN>"
}

# 集群配置
cluster {
  name: mistypass
  listen: 0.0.0.0:6222
  routes: [
    nats-route://nats-1.mistypass.internal:6222
    nats-route://nats-2.mistypass.internal:6222
    nats-route://nats-3.mistypass.internal:6222
  ]
}

# JetStream 持久化
jetstream {
  store_dir: /data/nats/jetstream
  max_mem: 512MB
  max_file: 10GB
}

# TLS (生产环境必须启用)
tls {
  cert_file: /etc/nats/tls/server.crt
  key_file: /etc/nats/tls/server.key
  ca_file: /etc/nats/tls/ca.crt
  verify: true
}

# 日志
logfile: /var/log/nats/nats-server.log
logfile_size_limit: 100MB
debug: false
trace: false
```

节点 2 和节点 3 只需更改 `server_name` 为 `nats-2`、`nats-3`。

#### JetStream Stream 配置

```bash
# 创建 Gateway 事件流 (R3 = 3 副本)
nats stream add MISTYPASS_GATEWAY_EVENTS \
  --subjects="mistypass.gateway.>" \
  --storage=file \
  --replicas=3 \
  --retention=limits \
  --max-age=7d \
  --max-bytes=5GB \
  --discard=old \
  --dupe-window=2m

# 创建消费者
nats consumer add MISTYPASS_GATEWAY_EVENTS api-processor \
  --deliver=all \
  --ack=explicit \
  --max-deliver=5 \
  --ack-wait=30s \
  --filter="mistypass.gateway.*.events"
```

#### 容错特性

- JetStream 流在 3 个节点间复制 (R3)。
- Gateway 主题 (`mistypass.gateway.{gw_id}.*`) 分布在集群中。
- 任一 NATS 节点宕机后，Gateway 自动重连到存活节点。
- Raft 共识确保 Leader 选举在秒级完成。

---

## 5. EMQX 高可用

### 5.1 EMQX 集群 (2+ 节点)

EMQX 负责 Gateway 设备的 MQTT 通信，包括遥测上报、命令下发和实时凭证验证。

#### emqx.conf (节点 1)

```hocon
node {
  name = "emqx@emqx-1.mistypass.internal"
  cookie = "<EMQX_CLUSTER_COOKIE>"
}

cluster {
  name = mistypass
  discovery_strategy = static
  static {
    seeds = ["emqx@emqx-1.mistypass.internal", "emqx@emqx-2.mistypass.internal"]
  }
}

# MQTT 监听器
listeners.tcp.default {
  bind = "0.0.0.0:1883"
  max_connections = 10000
}

listeners.ssl.default {
  bind = "0.0.0.0:8883"
  max_connections = 10000
  ssl_options {
    certfile = "/etc/emqx/certs/server.crt"
    keyfile = "/etc/emqx/certs/server.key"
    cacertfile = "/etc/emqx/certs/ca.crt"
    verify = verify_peer
    fail_if_no_peer_cert = true
  }
}

# WebSocket (管理端用)
listeners.ws.default {
  bind = "0.0.0.0:8083"
}

listeners.wss.default {
  bind = "0.0.0.0:8084"
  ssl_options {
    certfile = "/etc/emqx/certs/server.crt"
    keyfile = "/etc/emqx/certs/server.key"
  }
}

# Dashboard
dashboard {
  listeners.http {
    bind = "0.0.0.0:18083"
  }
  default_username = "admin"
  default_password = "<EMQX_DASHBOARD_PASSWORD>"
}

# 认证 (使用内置数据库或对接 PostgreSQL)
authentication = [
  {
    mechanism = password_based
    backend = built_in_database
    password_hash_algorithm {
      name = bcrypt
    }
  }
]

# 授权 (ACL)
authorization {
  no_match = deny
  sources = [
    {
      type = built_in_database
      enable = true
    }
  ]
}

# 会话持久化
durable_sessions {
  enable = true
}
```

#### EMQX 集群 DNS 发现 (Kubernetes 环境替代方案)

```hocon
cluster {
  name = mistypass
  discovery_strategy = dns
  dns {
    name = "emqx-headless.mistypass.svc.cluster.local"
    record_type = srv
  }
}
```

#### MQTT 主题设计

| 主题 | 方向 | 用途 |
|------|------|------|
| `mistypass/{tenant_id}/{gw_id}/telemetry` | Gateway -> Cloud | 心跳、状态上报 |
| `mistypass/{tenant_id}/{gw_id}/events` | Gateway -> Cloud | 门禁事件上报 |
| `mistypass/{tenant_id}/{gw_id}/commands` | Cloud -> Gateway | 远程开门、配置下发 |
| `mistypass/{tenant_id}/{gw_id}/ota` | Cloud -> Gateway | 固件更新通知 |

---

## 6. API 服务器

### 6.1 无状态设计

MistyPass API 服务器完全无状态。所有状态存储在：

- **PostgreSQL**：持久化数据（用户、门禁规则、审计日志）
- **Redis**：会话、速率限制、临时缓存
- **NATS**：实时消息和事件流

任何 API 实例均可处理任何请求，无需实例间协调。

### 6.2 健康检查端点

API 暴露 `GET /healthz` 端点：

```json
// 200 OK — 所有依赖正常
{"status": "ok", "time": "2026-05-03T10:00:00Z"}
```

- **200**：实例可达 PostgreSQL、Redis、NATS
- **503**：任何依赖不可达

负载均衡器应在连续 2 次健康检查失败后将实例从池中移除。

### 6.3 负载均衡配置

#### Caddy (推荐)

```
mistypass.example.com {
    encode zstd gzip

    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        X-Content-Type-Options "nosniff"
        X-Frame-Options "DENY"
        Referrer-Policy "strict-origin-when-cross-origin"
    }

    reverse_proxy api-1:8080 api-2:8080 {
        lb_policy       round_robin
        health_uri      /healthz
        health_interval 5s
        health_timeout  2s
    }
}
```

#### Nginx

```nginx
upstream mistypass_api {
    least_conn;
    server api-1:8080 max_fails=2 fail_timeout=10s;
    server api-2:8080 max_fails=2 fail_timeout=10s;

    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name mistypass.example.com;

    ssl_certificate     /etc/nginx/ssl/fullchain.pem;
    ssl_certificate_key /etc/nginx/ssl/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    location / {
        proxy_pass http://mistypass_api;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_connect_timeout 5s;
        proxy_read_timeout 30s;
        proxy_send_timeout 10s;
    }

    location /healthz {
        proxy_pass http://mistypass_api;
        access_log off;
    }
}
```

### 6.4 生产环境变量

```bash
# 必须设置的环境变量 (APP_ENV=production 时强制校验)
APP_ENV=production
PORT=8080
JWT_SECRET=<最少 32 字符的强随机字符串>
GATEWAY_BOOTSTRAP_TOKEN=<强随机字符串>
HRIS_VAULT_MASTER_KEY=<32 字节 hex 编码密钥>

# 数据库 (通过 PGBouncer)
DATABASE_URL=postgres://mistypass_app:<PASSWORD>@pgbouncer:6432/mistypass?sslmode=require
DATABASE_AUTO_MIGRATE=false          # 生产环境禁用自动迁移

# Redis
REDIS_ADDR=redis-sentinel-1:26379,redis-sentinel-2:26379,redis-sentinel-3:26379
REDIS_PASSWORD=<REDIS_PASSWORD>
REDIS_DB=0
REDIS_KEY_PREFIX=mistypass

# NATS
NATS_ENABLED=true
NATS_SERVER_URL=nats://nats-1:4222,nats://nats-2:4222,nats://nats-3:4222
NATS_SUBJECT_PREFIX=mistypass

# MQTT (EMQX)
MQTT_ENABLED=true
MQTT_BROKER_URL=tcp://emqx-1:1883
MQTT_TOPIC_PREFIX=mistypass

# OpenTelemetry
OTEL_ENABLED=true
OTEL_SERVICE_NAME=mistypass-api
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
OTEL_TRACE_SAMPLE_RATIO=0.1         # 生产环境采样率 10%

# CORS
CORS_ORIGIN=https://admin.mistypass.com

# 时区
TZ=Asia/Jakarta
DEFAULT_TIMEZONE=Asia/Jakarta
```

### 6.5 水平扩展

增加更多 API 实例只需在负载均衡器后面添加节点。无需实例间协调。推荐基于 CPU/内存自动扩缩：

- 目标 CPU 利用率：70%
- 最小实例数：2
- 最大实例数：10

---

## 7. Gateway Agent 边缘部署

### 7.1 边缘架构

Gateway Agent 运行在物理设备 (Orange Pi 等 ARM64 Linux 设备) 上，具备以下特性：

- **离线优先**：本地缓存门禁规则，断网时仍可做出访问决策
- **规则缓存 TTL**：默认 24 小时，超过后拒绝所有访问 (`-rules-cache-ttl`)
- **自动重连**：与云端断开后自动重连 HTTPS 和 NATS
- **设备令牌持久化**：首次注册后将设备令牌保存到本地文件
- **TLS 证书锁定**：支持 SPKI SHA256 证书锁定 (`-tls-pin-sha256`)

### 7.2 systemd 服务

```ini
# /etc/systemd/system/mistypass-gateway.service
[Unit]
Description=MistyPass Gateway Agent
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=300
StartLimitBurst=5

[Service]
Type=simple
User=mistypass
Group=mistypass
ExecStart=/usr/local/bin/gateway-agent \
    -api https://api.mistypass.com \
    -gateway ${GATEWAY_ID} \
    -tenant ${TENANT_ID} \
    -token-file /var/lib/mistypass/device-token \
    -tls-pin-sha256 ${TLS_PIN_SHA256} \
    -relay-gpio 73 \
    -unlock-duration 5s \
    -poll 30s \
    -heartbeat 30s \
    -rules-cache-ttl 24h
EnvironmentFile=/etc/mistypass/gateway.env
Restart=always
RestartSec=5
TimeoutStopSec=10

# 安全加固
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=/var/lib/mistypass
PrivateTmp=yes
ProtectHome=yes

# 看门狗
WatchdogSec=120

[Install]
WantedBy=multi-user.target
```

#### gateway.env

```bash
GATEWAY_ID=gw_jkt_building_a_001
TENANT_ID=tenant_jakarta_office
TLS_PIN_SHA256=<云端证书 SPKI SHA256 哈希值>
```

### 7.3 离线行为矩阵

| 场景 | 行为 |
|------|------|
| 网络断开 < 24h | 使用本地缓存规则继续放行 |
| 网络断开 > 24h (缓存过期) | 拒绝所有访问 |
| 云端 API 不可达 | 排队事件，连接恢复后批量上报 |
| NATS 断开 | 退回 HTTPS 轮询模式 |
| 设备重启 | 从 token-file 恢复身份，无需重新注册 |

### 7.4 固件更新

交叉编译 Gateway Agent 二进制：

```bash
GOOS=linux GOARCH=arm64 go build -o gateway-agent ./cmd/gateway-agent
```

通过 OTA 或 SSH 推送更新，然后重启服务：

```bash
sudo systemctl restart mistypass-gateway
```

---

## 8. TLS 与证书管理

### 8.1 外部 TLS (客户端到负载均衡)

#### 方案 A：Caddy 自动 HTTPS (推荐)

Caddy 自动处理 Let's Encrypt 证书的签发、续期和 OCSP Stapling，零配置：

```
mistypass.example.com {
    reverse_proxy api:8080
}
```

#### 方案 B：Cloudflare

1. 将域名 DNS 托管到 Cloudflare
2. 启用 "Full (Strict)" SSL 模式
3. 源站安装 Cloudflare Origin CA 证书（15 年有效）
4. 开启 "Always Use HTTPS" 和 "Automatic HTTPS Rewrites"

#### 方案 C：certbot + Nginx

```bash
# 签发证书
certbot certonly --webroot -w /var/www/html -d mistypass.example.com

# 自动续期 (cron)
0 3 * * * certbot renew --quiet --post-hook "systemctl reload nginx"
```

### 8.2 内部 TLS (服务间通信)

生产环境中各服务之间的通信应加密：

| 通信路径 | 推荐方案 |
|---------|---------|
| API <-> PostgreSQL | `sslmode=require` 或 `verify-full` |
| API <-> Redis | Redis 6+ 原生 TLS |
| API <-> NATS | NATS TLS + 客户端证书 |
| API <-> EMQX | MQTT over TLS (端口 8883) |
| Gateway <-> Cloud API | HTTPS + 证书锁定 |

对于 Kubernetes 环境，建议使用 cert-manager 自动管理内部证书：

```yaml
# cert-manager ClusterIssuer
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: mistypass-ca
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: mistypass-internal-tls
  namespace: mistypass
spec:
  secretName: mistypass-internal-tls
  issuerRef:
    name: mistypass-ca
    kind: ClusterIssuer
  dnsNames:
    - "*.mistypass.svc.cluster.local"
  duration: 8760h   # 1 年
  renewBefore: 720h  # 30 天前续期
```

### 8.3 安全标头

项目 Caddyfile 已配置以下安全标头 (保持一致)：

```
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
Content-Security-Policy: default-src 'self'; ...
Permissions-Policy: camera=(), microphone=(), ...
```

---

## 9. 可观测性

### 9.1 OpenTelemetry (项目已集成)

MistyPass API 已内置 OpenTelemetry 支持。生产环境配置：

```bash
OTEL_ENABLED=true
OTEL_SERVICE_NAME=mistypass-api
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
OTEL_EXPORTER_OTLP_INSECURE=false
OTEL_TRACE_SAMPLE_RATIO=0.1    # 生产环境 10% 采样
OTEL_EXPORT_TIMEOUT=5s
```

#### OTel Collector 配置

```yaml
# otel-collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 5s
    send_batch_size: 512
  memory_limiter:
    check_interval: 1s
    limit_mib: 512

exporters:
  # Prometheus 指标
  prometheus:
    endpoint: 0.0.0.0:8889
    namespace: mistypass
  # Jaeger 链路追踪
  otlp/jaeger:
    endpoint: jaeger:4317
    tls:
      insecure: true
  # 日志 (Loki)
  loki:
    endpoint: http://loki:3100/loki/api/v1/push

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/jaeger]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [prometheus]
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [loki]
```

### 9.2 Prometheus 指标

API 暴露 `GET /metrics` 端点 (Prometheus 兼容)。关键指标：

| 指标 | 类型 | 说明 |
|------|------|------|
| `http_requests_total` | Counter | HTTP 请求总数 (按方法/路径/状态码) |
| `http_request_duration_seconds` | Histogram | 请求延迟分布 |
| `gateway_online_total` | Gauge | 在线 Gateway 数量 |
| `access_decisions_total` | Counter | 门禁决策计数 (允许/拒绝) |
| `pg_pool_active_connections` | Gauge | 数据库连接池活跃连接数 |

#### Prometheus 采集配置

```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: mistypass-api
    static_configs:
      - targets:
          - api-1:8080
          - api-2:8080
    metrics_path: /metrics

  - job_name: nats
    static_configs:
      - targets:
          - nats-1:8222
          - nats-2:8222
          - nats-3:8222
    metrics_path: /varz

  - job_name: pgbouncer
    static_configs:
      - targets:
          - pgbouncer-exporter:9127

  - job_name: redis
    static_configs:
      - targets:
          - redis-exporter:9121

  - job_name: emqx
    static_configs:
      - targets:
          - emqx-1:18083
          - emqx-2:18083
    metrics_path: /api/v5/prometheus/stats
```

### 9.3 Grafana 仪表盘

建议创建以下仪表盘：

1. **API 总览** — 请求速率、错误率、p50/p95/p99 延迟
2. **Gateway 设备群** — 在线数量、心跳时间、离线 Gateway 列表
3. **数据库** — 连接池使用率、复制延迟、查询延迟
4. **门禁事件** — 每分钟放行/拒绝数、热门门锁、热门用户

### 9.4 告警规则

```yaml
# alerting-rules.yaml
groups:
  - name: mistypass-critical
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "API 5xx 错误率过高"
          description: "5 分钟内 5xx 错误率超过 5%"

      - alert: GatewayOffline
        expr: time() - gateway_last_heartbeat_timestamp > 300
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Gateway 离线"
          description: "Gateway {{ $labels.gateway_id }} 超过 5 分钟未上报心跳"

      - alert: ReplicationLag
        expr: pg_replication_lag_seconds > 10
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "PostgreSQL 复制延迟过高"

      - alert: RedisDown
        expr: redis_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Redis 实例不可达"

      - alert: NATSNoLeader
        expr: nats_jetstream_server_is_leader == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "NATS JetStream 无 Leader"

      - alert: HighConnectionPoolUsage
        expr: pg_pool_active_connections / pg_pool_max_connections > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "PGBouncer 连接池使用率超过 80%"

      - alert: DiskSpaceLow
        expr: node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"} < 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "磁盘剩余空间不足 10%"
```

---

## 10. 备份与恢复

### 10.1 PostgreSQL 备份

| 方法 | 频率 | 保留 | 用途 |
|------|------|------|------|
| `pg_dump` | 每日 02:00 UTC | 30 天 | 逻辑备份，便于恢复 |
| WAL 归档 | 持续 | 7 天 | 时间点恢复 (PITR) |
| `pg_basebackup` | 每周 | 4 周 | 完整物理备份 |

#### 每日逻辑备份脚本

```bash
#!/bin/bash
# /opt/mistypass/scripts/pg-backup.sh
set -euo pipefail

BACKUP_DIR=/backups/pg
DATE=$(date +%Y%m%d_%H%M%S)
RETENTION_DAYS=30

mkdir -p "$BACKUP_DIR"

# 创建压缩备份
pg_dump -Fc -h pgbouncer -p 6432 -U mistypass_app mistypass \
  | gzip > "${BACKUP_DIR}/mistypass_${DATE}.dump.gz"

# 上传到对象存储 (GCS)
gsutil cp "${BACKUP_DIR}/mistypass_${DATE}.dump.gz" \
  gs://mistypass-backups/pg/mistypass_${DATE}.dump.gz

# 清理过期本地备份
find "$BACKUP_DIR" -name "*.dump.gz" -mtime +${RETENTION_DAYS} -delete

echo "[$(date)] 备份完成: mistypass_${DATE}.dump.gz"
```

#### WAL 归档配置 (postgresql.conf)

```ini
archive_mode = on
archive_command = 'gsutil cp %p gs://mistypass-backups/wal/%f'
archive_timeout = 300       # 最长 5 分钟归档一次
```

#### 恢复流程

```bash
# 从逻辑备份恢复
gunzip -c mistypass_20260503.dump.gz | pg_restore -d mistypass -Fc --clean

# PITR 恢复 (恢复到指定时间点)
# 1. 停止 PostgreSQL
# 2. 从 pg_basebackup 还原数据目录
# 3. 配置 recovery.conf / recovery.signal:
restore_command = 'gsutil cp gs://mistypass-backups/wal/%f %p'
recovery_target_time = '2026-05-03 10:00:00+07'
recovery_target_action = 'promote'
# 4. 启动 PostgreSQL
```

### 10.2 Redis 备份

```conf
# redis.conf 持久化策略
save 900 1         # 15 分钟内至少 1 个 key 变更时快照
save 300 10        # 5 分钟内至少 10 个 key 变更
save 60 10000      # 1 分钟内至少 10000 个 key 变更
appendonly yes
appendfsync everysec
```

每日将 RDB 快照复制到对象存储：

```bash
#!/bin/bash
# /opt/mistypass/scripts/redis-backup.sh
REDIS_DATA=/var/lib/redis
gsutil cp "${REDIS_DATA}/dump.rdb" \
  "gs://mistypass-backups/redis/dump_$(date +%Y%m%d).rdb"
```

**重要**：Redis 数据完全丢失是可恢复的 — 用户只需重新登录。PostgreSQL 是唯一权威数据源。

### 10.3 NATS JetStream 备份

```bash
# 导出 stream 数据
nats stream backup MISTYPASS_GATEWAY_EVENTS /backups/nats/gateway-events-$(date +%Y%m%d)

# 上传到对象存储
gsutil -m rsync -r /backups/nats/ gs://mistypass-backups/nats/
```

JetStream 数据本身已在 3 节点间复制 (R3)，提供内置冗余。

### 10.4 备份验证

**每月进行恢复演练**。从未恢复测试过的备份不是备份。

```bash
# 验证流程
# 1. 在隔离环境中恢复 PostgreSQL 备份
# 2. 运行 API 健康检查
# 3. 验证关键数据完整性 (租户数、门禁规则数)
# 4. 记录恢复时间 (RTO) 和数据完整性 (RPO)
```

---

## 11. 印度尼西亚区域部署

### 11.1 区域选择

| 云厂商 | 区域 | 位置 | 延迟 (雅加达) |
|--------|------|------|-------------|
| GCP | asia-southeast2 | 雅加达 | < 5 ms |
| AWS | ap-southeast-1 | 新加坡 | 15-30 ms |
| AWS | ap-southeast-3 | 雅加达 | < 5 ms |
| 阿里云 | ap-southeast-5 | 雅加达 | < 5 ms |

**首选**：GCP asia-southeast2 (雅加达)。直接覆盖印尼本土，满足 UU PDP (印尼个人数据保护法) 数据本地化要求。

### 11.2 网络延迟考量

```
用户 (雅加达) → GCP asia-southeast2:  < 5ms   (理想)
用户 (雅加达) → AWS ap-southeast-1:   15-30ms (可接受)
Gateway (楼宇) → Cloud API:            取决于本地 ISP，通常 10-50ms
Gateway (离线模式):                     0ms (本地决策)
```

关键原则：
- Gateway 设备的门禁决策始终在本地完成，不依赖云端延迟
- 云端延迟仅影响管理操作 (配置同步、事件上报、仪表盘查看)
- 印尼 ISP 质量参差不齐，Gateway 的离线能力是核心要求

### 11.3 合规要求 (UU PDP)

- 租户的个人数据 (姓名、手机号、门禁记录) 必须存储在印尼境内
- 日志和审计数据保留至少 5 年
- 数据跨境传输需要数据主体同意
- 详见 `docs/compliance-uu-pdp-indonesia.md`

### 11.4 印尼特定基础设施建议

| 方面 | 建议 |
|------|------|
| CDN | Cloudflare (雅加达 PoP) 或 GCP Cloud CDN |
| DNS | Cloudflare DNS (低延迟全球解析) |
| 短信/WhatsApp | 项目已集成 Meta WhatsApp Business API |
| 支付 | Midtrans / Xendit (印尼本土支付网关) |
| UPS | 所有 Gateway 设备必须配备 UPS (详见 `docs/deployment/ups-power-guide.md`) |

---

## 12. Docker Compose 生产配置

> 适用于单机或小规模部署。大规模部署应使用 Kubernetes (见第 14 章)。

```yaml
# docker-compose.production.yml
# 用法: docker compose -f docker-compose.production.yml up -d

services:
  # ============================================================
  # 反向代理 + TLS
  # ============================================================
  caddy:
    image: caddy:2.10
    container_name: mistypass-caddy
    restart: always
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./deploy/caddy/Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    environment:
      MISTYPASS_DOMAIN: ${MISTYPASS_DOMAIN}
    depends_on:
      api:
        condition: service_healthy
    networks:
      - frontend
      - backend

  # ============================================================
  # API 服务器 (可以 scale 多个实例)
  # ============================================================
  api:
    image: mistypass-api:${IMAGE_TAG:-latest}
    restart: always
    deploy:
      replicas: 2
      resources:
        limits:
          cpus: "2.0"
          memory: 1G
        reservations:
          cpus: "0.5"
          memory: 256M
    environment:
      APP_ENV: production
      PORT: "8080"
      JWT_SECRET: ${JWT_SECRET}
      GATEWAY_BOOTSTRAP_TOKEN: ${GATEWAY_BOOTSTRAP_TOKEN}
      HRIS_VAULT_MASTER_KEY: ${HRIS_VAULT_MASTER_KEY}
      DATABASE_URL: "postgres://mistypass_app:${POSTGRES_PASSWORD}@pgbouncer:6432/mistypass?sslmode=disable"
      DATABASE_AUTO_MIGRATE: "false"
      REDIS_ADDR: "redis:6379"
      REDIS_PASSWORD: ${REDIS_PASSWORD}
      REDIS_DB: "0"
      REDIS_KEY_PREFIX: mistypass
      NATS_ENABLED: "true"
      NATS_SERVER_URL: "nats://nats:4222"
      NATS_SUBJECT_PREFIX: mistypass
      MQTT_ENABLED: "true"
      MQTT_BROKER_URL: "tcp://emqx:1883"
      MQTT_TOPIC_PREFIX: mistypass
      OTEL_ENABLED: "true"
      OTEL_SERVICE_NAME: mistypass-api
      OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector:4317"
      OTEL_TRACE_SAMPLE_RATIO: "0.1"
      CORS_ORIGIN: "https://${MISTYPASS_DOMAIN}"
      TZ: Asia/Jakarta
      DEFAULT_TIMEZONE: Asia/Jakarta
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/healthz"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 10s
    depends_on:
      pgbouncer:
        condition: service_started
      redis:
        condition: service_healthy
      nats:
        condition: service_started
      emqx:
        condition: service_started
    networks:
      - backend

  # ============================================================
  # PostgreSQL 主库
  # ============================================================
  postgres:
    image: postgres:16
    restart: always
    environment:
      POSTGRES_DB: mistypass
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./deploy/postgres/postgresql.conf:/etc/postgresql/postgresql.conf:ro
    command: postgres -c config_file=/etc/postgresql/postgresql.conf
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d mistypass"]
      interval: 10s
      timeout: 5s
      retries: 5
    deploy:
      resources:
        limits:
          cpus: "4.0"
          memory: 8G
    networks:
      - backend

  # ============================================================
  # PGBouncer 连接池
  # ============================================================
  pgbouncer:
    image: edoburu/pgbouncer:latest
    restart: always
    environment:
      DB_USER: postgres
      DB_PASSWORD: ${POSTGRES_PASSWORD}
      DB_HOST: postgres
      DB_PORT: "5432"
      DB_NAME: mistypass
      POOL_MODE: transaction
      MAX_CLIENT_CONN: "500"
      DEFAULT_POOL_SIZE: "30"
    depends_on:
      postgres:
        condition: service_healthy
    networks:
      - backend

  # ============================================================
  # Redis + 持久化
  # ============================================================
  redis:
    image: redis:7
    restart: always
    command:
      - redis-server
      - --appendonly
      - "yes"
      - --appendfsync
      - everysec
      - --requirepass
      - ${REDIS_PASSWORD}
      - --maxmemory
      - 1gb
      - --maxmemory-policy
      - allkeys-lru
      - --save
      - "900 1"
      - --save
      - "300 10"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "--no-auth-warning", "-a", "${REDIS_PASSWORD}", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5
    deploy:
      resources:
        limits:
          cpus: "1.0"
          memory: 1.5G
    networks:
      - backend

  # ============================================================
  # EMQX MQTT Broker
  # ============================================================
  emqx:
    image: emqx/emqx:5.10.1
    restart: always
    environment:
      EMQX_DASHBOARD__DEFAULT_USERNAME: ${EMQX_DASHBOARD_USERNAME:-admin}
      EMQX_DASHBOARD__DEFAULT_PASSWORD: ${EMQX_DASHBOARD_PASSWORD}
    volumes:
      - emqx_data:/opt/emqx/data
      - emqx_log:/opt/emqx/log
    deploy:
      resources:
        limits:
          cpus: "2.0"
          memory: 2G
    networks:
      - backend

  # ============================================================
  # NATS + JetStream
  # ============================================================
  nats:
    image: nats:2.10-alpine
    restart: always
    command:
      - -js
      - --store_dir=/data/nats/jetstream
      - -m
      - "8222"
    volumes:
      - nats_data:/data/nats
    deploy:
      resources:
        limits:
          cpus: "1.0"
          memory: 1G
    networks:
      - backend

  # ============================================================
  # OpenTelemetry Collector
  # ============================================================
  otel-collector:
    image: otel/opentelemetry-collector-contrib:0.100.0
    restart: always
    volumes:
      - ./deploy/otel/otel-collector-config.yaml:/etc/otelcol-contrib/config.yaml:ro
    networks:
      - backend

volumes:
  caddy_data:
  caddy_config:
  postgres_data:
  redis_data:
  emqx_data:
  emqx_log:
  nats_data:

networks:
  frontend:
  backend:
    internal: true
```

---

## 13. Systemd 单元文件

适用于裸机 / VM 部署场景。

### 13.1 API 服务器

```ini
# /etc/systemd/system/mistypass-api.service
[Unit]
Description=MistyPass API Server
After=network-online.target postgresql.service redis.service
Wants=network-online.target
Requires=postgresql.service redis.service

[Service]
Type=simple
User=mistypass
Group=mistypass
EnvironmentFile=/etc/mistypass/api.env
ExecStart=/usr/local/bin/mistypass-api
Restart=always
RestartSec=5
TimeoutStopSec=30

# 资源限制
LimitNOFILE=65536
LimitNPROC=4096

# 安全加固
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ReadWritePaths=/var/lib/mistypass /var/log/mistypass

[Install]
WantedBy=multi-user.target
```

### 13.2 PGBouncer

```ini
# /etc/systemd/system/pgbouncer.service
[Unit]
Description=PGBouncer Connection Pooler
After=postgresql.service
Requires=postgresql.service

[Service]
Type=simple
User=pgbouncer
Group=pgbouncer
ExecStart=/usr/bin/pgbouncer /etc/pgbouncer/pgbouncer.ini
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=3
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

### 13.3 NATS

```ini
# /etc/systemd/system/nats.service
[Unit]
Description=NATS Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=nats
Group=nats
ExecStart=/usr/local/bin/nats-server -c /etc/nats/nats-server.conf
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=3
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

---

## 14. Kubernetes 清单

适用于中大规模部署。以下为核心资源清单。

### 14.1 Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: mistypass
  labels:
    app.kubernetes.io/part-of: mistypass
```

### 14.2 API Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mistypass-api
  namespace: mistypass
  labels:
    app: mistypass-api
spec:
  replicas: 2
  selector:
    matchLabels:
      app: mistypass-api
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
  template:
    metadata:
      labels:
        app: mistypass-api
    spec:
      serviceAccountName: mistypass-api
      terminationGracePeriodSeconds: 30
      containers:
        - name: api
          image: registry.example.com/mistypass-api:latest
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
          envFrom:
            - secretRef:
                name: mistypass-api-secrets
            - configMapRef:
                name: mistypass-api-config
          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 3
            periodSeconds: 5
            failureThreshold: 2
          resources:
            requests:
              cpu: 500m
              memory: 256Mi
            limits:
              cpu: "2"
              memory: 1Gi
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: DoNotSchedule
          labelSelector:
            matchLabels:
              app: mistypass-api
---
apiVersion: v1
kind: Service
metadata:
  name: mistypass-api
  namespace: mistypass
spec:
  selector:
    app: mistypass-api
  ports:
    - name: http
      port: 8080
      targetPort: http
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: mistypass-api
  namespace: mistypass
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: mistypass-api
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

### 14.3 ConfigMap 与 Secret

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mistypass-api-config
  namespace: mistypass
data:
  APP_ENV: production
  PORT: "8080"
  DATABASE_AUTO_MIGRATE: "false"
  REDIS_DB: "0"
  REDIS_KEY_PREFIX: mistypass
  NATS_ENABLED: "true"
  NATS_SUBJECT_PREFIX: mistypass
  MQTT_ENABLED: "true"
  MQTT_TOPIC_PREFIX: mistypass
  OTEL_ENABLED: "true"
  OTEL_SERVICE_NAME: mistypass-api
  OTEL_TRACE_SAMPLE_RATIO: "0.1"
  TZ: Asia/Jakarta
  DEFAULT_TIMEZONE: Asia/Jakarta
---
apiVersion: v1
kind: Secret
metadata:
  name: mistypass-api-secrets
  namespace: mistypass
type: Opaque
stringData:
  JWT_SECRET: "<CHANGE_ME>"
  GATEWAY_BOOTSTRAP_TOKEN: "<CHANGE_ME>"
  HRIS_VAULT_MASTER_KEY: "<CHANGE_ME>"
  DATABASE_URL: "postgres://mistypass_app:<PW>@pgbouncer:6432/mistypass?sslmode=require"
  REDIS_ADDR: "redis-sentinel:26379"
  REDIS_PASSWORD: "<CHANGE_ME>"
  NATS_SERVER_URL: "nats://nats:4222"
  MQTT_BROKER_URL: "tcp://emqx:1883"
  OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-collector:4317"
  CORS_ORIGIN: "https://admin.mistypass.com"
```

### 14.4 Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mistypass-api
  namespace: mistypass
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/proxy-body-size: "10m"
    nginx.ingress.kubernetes.io/rate-limit-connections: "50"
    nginx.ingress.kubernetes.io/rate-limit-rps: "100"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - api.mistypass.com
        - admin.mistypass.com
      secretName: mistypass-tls
  rules:
    - host: api.mistypass.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: mistypass-api
                port:
                  number: 8080
```

### 14.5 NATS StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: nats
  namespace: mistypass
spec:
  serviceName: nats-headless
  replicas: 3
  selector:
    matchLabels:
      app: nats
  template:
    metadata:
      labels:
        app: nats
    spec:
      containers:
        - name: nats
          image: nats:2.10-alpine
          ports:
            - containerPort: 4222
              name: client
            - containerPort: 6222
              name: cluster
            - containerPort: 8222
              name: monitor
          args:
            - -js
            - --cluster_name=mistypass
            - --cluster=nats://0.0.0.0:6222
            - --routes=nats://nats-0.nats-headless:6222,nats://nats-1.nats-headless:6222,nats://nats-2.nats-headless:6222
            - --store_dir=/data/jetstream
            - -m
            - "8222"
          volumeMounts:
            - name: nats-data
              mountPath: /data
          resources:
            requests:
              cpu: 250m
              memory: 256Mi
            limits:
              cpu: "1"
              memory: 1Gi
  volumeClaimTemplates:
    - metadata:
        name: nats-data
      spec:
        accessModes: ["ReadWriteOnce"]
        storageClassName: ssd
        resources:
          requests:
            storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: nats
  namespace: mistypass
spec:
  selector:
    app: nats
  ports:
    - name: client
      port: 4222
---
apiVersion: v1
kind: Service
metadata:
  name: nats-headless
  namespace: mistypass
spec:
  clusterIP: None
  selector:
    app: nats
  ports:
    - name: client
      port: 4222
    - name: cluster
      port: 6222
    - name: monitor
      port: 8222
```

### 14.6 EMQX StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: emqx
  namespace: mistypass
spec:
  serviceName: emqx-headless
  replicas: 2
  selector:
    matchLabels:
      app: emqx
  template:
    metadata:
      labels:
        app: emqx
    spec:
      containers:
        - name: emqx
          image: emqx/emqx:5.10.1
          ports:
            - containerPort: 1883
              name: mqtt
            - containerPort: 8883
              name: mqtts
            - containerPort: 8083
              name: ws
            - containerPort: 18083
              name: dashboard
          env:
            - name: EMQX_NAME
              value: emqx
            - name: EMQX_CLUSTER__DISCOVERY_STRATEGY
              value: dns
            - name: EMQX_CLUSTER__DNS__NAME
              value: emqx-headless.mistypass.svc.cluster.local
            - name: EMQX_CLUSTER__DNS__RECORD_TYPE
              value: srv
            - name: EMQX_DASHBOARD__DEFAULT_USERNAME
              valueFrom:
                secretKeyRef:
                  name: emqx-secrets
                  key: dashboard-username
            - name: EMQX_DASHBOARD__DEFAULT_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: emqx-secrets
                  key: dashboard-password
          volumeMounts:
            - name: emqx-data
              mountPath: /opt/emqx/data
          resources:
            requests:
              cpu: 500m
              memory: 512Mi
            limits:
              cpu: "2"
              memory: 2Gi
  volumeClaimTemplates:
    - metadata:
        name: emqx-data
      spec:
        accessModes: ["ReadWriteOnce"]
        storageClassName: ssd
        resources:
          requests:
            storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: emqx
  namespace: mistypass
spec:
  selector:
    app: emqx
  ports:
    - name: mqtt
      port: 1883
    - name: mqtts
      port: 8883
    - name: ws
      port: 8083
    - name: dashboard
      port: 18083
---
apiVersion: v1
kind: Service
metadata:
  name: emqx-headless
  namespace: mistypass
spec:
  clusterIP: None
  selector:
    app: emqx
  ports:
    - name: mqtt
      port: 1883
    - name: cluster-ekka
      port: 4370
    - name: cluster-rpc
      port: 5369
```

---

## 15. 上线检查清单

### 安全

- [ ] `APP_ENV=production` 已设置
- [ ] `JWT_SECRET` 使用 32+ 字符强随机字符串
- [ ] `GATEWAY_BOOTSTRAP_TOKEN` 已更换为生产值
- [ ] `HRIS_VAULT_MASTER_KEY` 已配置
- [ ] `DATABASE_AUTO_MIGRATE=false` (使用迁移工具)
- [ ] 所有密码/密钥不在代码仓库中
- [ ] TLS 已启用 (外部 HTTPS + 内部 mTLS/VPC)
- [ ] CORS_ORIGIN 限制为生产域名
- [ ] Redis `rename-command` 已禁用危险命令
- [ ] PGBouncer `auth_type=scram-sha-256`

### 高可用

- [ ] PostgreSQL 主-从复制已配置并验证
- [ ] Redis Sentinel (3 节点) 已部署
- [ ] NATS JetStream 集群 (3 节点) 已部署
- [ ] EMQX 集群 (2+ 节点) 已部署
- [ ] API 服务器至少 2 个实例
- [ ] 负载均衡器健康检查已配置 (`/healthz`)

### 备份

- [ ] PostgreSQL 每日备份 + WAL 归档已启用
- [ ] Redis RDB 快照定期备份
- [ ] 备份文件上传到对象存储 (GCS/S3)
- [ ] 恢复演练已完成至少一次

### 监控

- [ ] OpenTelemetry 已启用
- [ ] Prometheus 已采集所有组件指标
- [ ] Grafana 仪表盘已创建
- [ ] 告警规则已配置 (错误率、Gateway 离线、复制延迟)
- [ ] 值班通知渠道已设置 (WhatsApp/Lark/邮件)

### 网络

- [ ] 所有内部服务不暴露公网端口
- [ ] VPC / 私有网络已配置
- [ ] 防火墙规则已设置
- [ ] DNS 已配置 (A/CNAME 记录)
- [ ] CDN 已配置 (可选)

### Gateway 设备

- [ ] Gateway Agent 二进制已交叉编译 (ARM64)
- [ ] systemd 服务已配置
- [ ] UPS 已安装 (参见 `docs/deployment/ups-power-guide.md`)
- [ ] 离线模式已测试 (断网 > 24h 拒绝所有)
- [ ] TLS 证书锁定已配置
- [ ] 设备令牌文件路径权限正确

---

> **注意**：本指南基于 MistyPass 当前架构编写。随着系统演进，请定期更新此文档。
> 如有疑问，请联系基础设施团队。
