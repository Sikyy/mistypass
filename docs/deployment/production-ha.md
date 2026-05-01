# Production High-Availability Deployment

> Deployment guide for running MistyPass in a production HA configuration.
>
> Last updated: 2026-05-01

---

## Architecture Overview

A production MistyPass deployment consists of five layers:

1. **Load Balancer** -- TLS termination and request routing
2. **API Servers** -- stateless Go binaries (2+ instances)
3. **PostgreSQL** -- primary-replica with connection pooling
4. **Redis** -- session store and cache (Sentinel for HA)
5. **NATS** -- real-time gateway messaging (JetStream cluster)

All components should be deployed across at least 3 nodes for quorum-based failover.

---

## Recommended Topology (3-Node Minimum)

```
                        Internet
                           |
                    [ Load Balancer ]
                    (Caddy / Nginx)
                     TLS termination
                     /            \
               +--------+    +--------+
               | API-1  |    | API-2  |
               | :8080  |    | :8080  |
               +--------+    +--------+
                  |    \        /    |
                  |     \      /     |
    +-------------+------+----+-----+-----------+
    |             |            |                 |
+--------+  +---------+  +---------+  +---------+
|  PG    |  |  PG     |  |  Redis  |  |  NATS   |
|Primary |  |Replica  |  |Sentinel |  | Cluster |
|+ PgB.  |  |+ PgB.   |  | (3x)   |  |  (3x)   |
+--------+  +---------+  +---------+  +---------+

Node 1: API-1, PG Primary, PgBouncer, Redis-1, Sentinel-1, NATS-1
Node 2: API-2, PG Replica, PgBouncer, Redis-2, Sentinel-2, NATS-2
Node 3: PgBouncer (witness), Redis-3, Sentinel-3, NATS-3
```

For larger deployments, separate database and messaging nodes from API nodes.

---

## PostgreSQL HA

### Components

| Component | Role |
|-----------|------|
| PG Primary | Read-write, single leader |
| PG Replica | Streaming replication, read-only standby |
| PgBouncer | Connection pooling (one per PG instance) |
| Patroni or repmgr | Automated failover orchestration |

### Setup with Patroni

1. Install Patroni on each PG node. Patroni uses etcd/Consul/ZooKeeper for leader election.
2. Configure `postgresql.conf` for streaming replication:
   ```
   wal_level = replica
   max_wal_senders = 5
   wal_keep_size = 1GB
   synchronous_commit = on
   ```
3. Set up PgBouncer on each node pointing to the local PG instance:
   ```ini
   [databases]
   mistypass = host=127.0.0.1 port=5432 dbname=mistypass
   [pgbouncer]
   pool_mode = transaction
   max_client_conn = 200
   default_pool_size = 20
   ```
4. API servers connect to PgBouncer (port 6432), not PG directly.

### Failover Behavior

- Patroni detects primary failure within ~30 seconds.
- The replica is promoted to primary automatically.
- PgBouncer connections are briefly interrupted during switchover (< 5 s with proper health checks).
- API servers retry failed queries; stateless design means no session loss.

---

## Redis HA

### Redis Sentinel (recommended)

Deploy 3 Redis instances with 3 Sentinel processes for automatic failover.

```
# sentinel.conf (each node)
sentinel monitor mistypass-redis <primary-ip> 6379 2
sentinel down-after-milliseconds mistypass-redis 5000
sentinel failover-timeout mistypass-redis 10000
sentinel parallel-syncs mistypass-redis 1
```

The quorum of 2 (out of 3 Sentinels) triggers automatic failover. API servers use Sentinel-aware Redis clients to discover the current primary.

### What MistyPass stores in Redis

| Key pattern | Purpose | TTL |
|-------------|---------|-----|
| `session:<id>` | User sessions | 24 h |
| `rate:<ip>` | Rate limit counters | 60 s |
| `nonce:<val>` | Request nonce dedup | 300 s |

Loss of Redis data causes session invalidation (users re-login) but no data loss. PostgreSQL is the source of truth.

---

## NATS HA

### 3-Node JetStream Cluster

NATS provides real-time communication between the cloud API and gateways. JetStream adds persistence for event delivery guarantees.

```hcl
# nats-server.conf (each node)
server_name: nats-1
listen: 0.0.0.0:4222

cluster {
  name: mistypass
  listen: 0.0.0.0:6222
  routes: [
    nats-route://nats-1:6222
    nats-route://nats-2:6222
    nats-route://nats-3:6222
  ]
}

jetstream {
  store_dir: /data/nats/jetstream
  max_mem: 256MB
  max_file: 2GB
}
```

- JetStream streams replicate across all 3 nodes (R3).
- Gateway subjects (`mistypass.gateway.{gw_id}.*`) are distributed across the cluster.
- If one NATS node goes down, gateways reconnect to surviving nodes automatically.

---

## API HA

### Stateless Design

MistyPass API servers are fully stateless. All state lives in PostgreSQL (persistent data), Redis (sessions/cache), and NATS (real-time messaging). Any API instance can serve any request.

### Load Balancer Configuration (Caddy example)

```
mistypass.example.com {
    reverse_proxy api-1:8080 api-2:8080 {
        lb_policy       round_robin
        health_uri      /healthz
        health_interval 5s
        health_timeout  2s
    }
}
```

### Health Check Endpoint

The API exposes `GET /healthz` which returns:
- **200** when the instance can reach PostgreSQL, Redis, and NATS
- **503** when any dependency is unreachable

The load balancer should remove unhealthy instances from the pool within two failed health checks.

### Scaling

Add more API instances behind the load balancer as traffic grows. No coordination is required between instances.

---

## TLS

### Option 1: Caddy (recommended)

Caddy handles automatic HTTPS via Let's Encrypt with zero configuration beyond the domain name. It manages certificate issuance, renewal, and OCSP stapling automatically.

### Option 2: Nginx + certbot

```bash
certbot certonly --webroot -w /var/www/html -d mistypass.example.com
```

Configure Nginx to use the generated certificates and set up a cron job for renewal:
```
0 3 * * * certbot renew --quiet --post-hook "systemctl reload nginx"
```

### Internal TLS

For inter-service communication (API to PostgreSQL, Redis, NATS), use mTLS or deploy within a private network/VPC with no public exposure.

---

## Monitoring

### Prometheus Metrics

The API exposes a Prometheus-compatible metrics endpoint at `GET /metrics`. Key metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Total HTTP requests by method, path, status |
| `http_request_duration_seconds` | Histogram | Request latency distribution |
| `gateway_online_total` | Gauge | Number of connected gateways |
| `access_decisions_total` | Counter | Access grant/deny counts |
| `pg_pool_active_connections` | Gauge | Active database connections |

### Grafana Dashboards

Create dashboards for:
1. **API Overview** -- request rate, error rate, p50/p95/p99 latency
2. **Gateway Fleet** -- online count, heartbeat age, offline gateways
3. **Database** -- connection pool usage, replication lag, query latency
4. **Access Events** -- grants/denies per minute, top doors, top users

### Alerting Rules (examples)

```yaml
groups:
  - name: mistypass
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
      - alert: GatewayOffline
        expr: time() - gateway_last_heartbeat_timestamp > 300
        for: 5m
      - alert: ReplicationLag
        expr: pg_replication_lag_seconds > 10
        for: 2m
```

---

## Backup Strategy

### PostgreSQL

| Method | Frequency | Retention | Purpose |
|--------|-----------|-----------|---------|
| `pg_dump` | Daily at 02:00 UTC | 30 days | Logical backup, easy restore |
| WAL archiving | Continuous | 7 days | Point-in-time recovery (PITR) |
| `pg_basebackup` | Weekly | 4 weeks | Full physical backup |

```bash
# Daily logical backup
pg_dump -Fc mistypass | gzip > /backups/pg/mistypass_$(date +%Y%m%d).dump.gz
```

For PITR, configure WAL archiving to ship WAL files to object storage (S3, GCS, MinIO).

### Redis

- Enable RDB snapshots: `save 900 1` (snapshot every 15 min if at least 1 key changed)
- Enable AOF for durability: `appendonly yes`, `appendfsync everysec`
- Copy RDB snapshots to off-host storage daily

Redis data is ephemeral (sessions, rate limits). Full data loss is recoverable -- users simply re-authenticate.

### NATS JetStream

- JetStream data is replicated across 3 nodes (R3), providing built-in redundancy.
- For disaster recovery, back up the JetStream store directory (`/data/nats/jetstream`) daily.
- NATS streams can be exported and re-imported using `nats stream backup` / `nats stream restore`.

### Backup Verification

Test restores monthly. A backup that has never been restored is not a backup.
