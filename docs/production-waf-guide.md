# MistyPass 生产环境 WAF 安全指南

> 版本：1.0 | 最后更新：2026-05-03
>
> 本文档覆盖 MistyPass 云端门禁 SaaS 平台的 Web 应用防火墙 (WAF) 配置，
> 包括 WAF 选型、规则配置、白名单策略和 Cloudflare 推荐配置。目标部署区域为印度尼西亚。

---

## 目录

1. [WAF 选型建议](#1-waf-选型建议)
2. [核心规则配置](#2-核心规则配置)
3. [白名单规则](#3-白名单规则)
4. [CORS 配置](#4-cors-配置)
5. [TLS 配置](#5-tls-配置)
6. [DDoS 防护策略](#6-ddos-防护策略)
7. [日志与告警](#7-日志与告警)
8. [Cloudflare 推荐配置](#8-cloudflare-推荐配置印尼区域)

---

## 1. WAF 选型建议

| 方案 | 优势 | 劣势 | 推荐场景 |
|------|------|------|----------|
| **Cloudflare WAF** | 印尼有 Jakarta PoP 节点，延迟 < 5ms；托管规则免运维；与 DDoS/CDN 一体化 | 高级规则需 Enterprise 计划 | **首选方案** — 印尼客户延迟最低 |
| AWS WAF | 与 ALB/CloudFront 深度集成；ap-southeast-1 可用 | 无印尼本地 PoP；规则调试不如 Cloudflare 直观 | 已用 AWS 全家桶的团队 |
| ModSecurity + Caddy/Nginx | 完全自主可控；无月费 | 需自行运维规则更新；印尼无 Anycast 加速 | 预算极低或合规要求自建 |

**结论：推荐 Cloudflare WAF (Pro/Business 计划)**。印尼 Jakarta 有 PoP 节点，对本地客户
延迟最低。下文配置以 Cloudflare 为主，附 ModSecurity 等效规则供参考。

---

## 2. 核心规则配置

### 2.1 OWASP Core Rule Set (CRS) 基线

Cloudflare 托管规则中已包含 OWASP CRS 等效规则，需开启以下规则组：

| 规则组 | Cloudflare 规则 ID | 动作 | 说明 |
|--------|---------------------|------|------|
| SQL Injection | `100001` | Block | 覆盖 SQLi 全部变体 |
| XSS | `100002` | Block | 反射型/存储型 XSS |
| Path Traversal | `100003` | Block | `../` 路径穿越 |
| RFI/LFI | `100004` | Block | 远程/本地文件包含 |
| Command Injection | `100005` | Block | OS 命令注入 |
| Protocol Attack | `100006` | Log | HTTP 协议违规（先观察再拦截） |

**Cloudflare Dashboard 操作路径：**
Security > WAF > Managed Rules > Cloudflare OWASP Core Ruleset > 设为 **High** 敏感度。

### 2.2 SQL Injection / XSS / Path Traversal

Cloudflare 自定义 WAF 规则（补充托管规则覆盖不到的场景）：

```
# 规则 1：阻断 SQL 注入关键字（针对 JSON body）
# Cloudflare Ruleset Engine Expression
(http.request.uri.path contains "/api/v1/" and
 any(http.request.body.form.values[*] contains "UNION SELECT") or
 any(http.request.body.form.values[*] contains "1=1") or
 any(http.request.body.form.values[*] contains "OR 1=1"))
→ Action: Block

# 规则 2：阻断路径穿越
(http.request.uri.path contains ".." or
 http.request.uri.path contains "%2e%2e" or
 http.request.uri.path contains "%252e")
→ Action: Block

# 规则 3：阻断 XSS 反射型攻击
(http.request.uri.query contains "<script" or
 http.request.uri.query contains "javascript:" or
 http.request.uri.query contains "onerror=")
→ Action: Block
```

### 2.3 Rate Limiting（按端点分级）

MistyPass 端点分为四个限速层级：

| 端点类别 | 路径模式 | 限速 | 窗口 | 动作 |
|----------|----------|------|------|------|
| 登录/认证 | `/api/v1/auth/*`, `/api/v1/app/auth/*`, `/oauth2/token` | **10 次/分钟** | 60s | Block 60s |
| 通用 API | `/api/v1/*` (非认证端点) | **100 次/分钟** | 60s | Challenge |
| SCIM 同步 | `/scim/v2/*` | **200 次/分钟** | 60s | Block 60s |
| Webhook 接收 | `/api/v1/integrations/lark/events`, `/api/v1/enterprise/hris-webhook/*` | **240 次/分钟** | 60s | Log |
| Gateway 上报 | `/api/v1/gateway/*` | **300 次/分钟** | 60s | Block 60s |

**Cloudflare Rate Limiting 规则示例：**

```
# 登录端点限速 — 10 req/min per IP
Rule name: rate-limit-auth
Expression:
  (http.request.uri.path matches "^/api/v1/auth/" or
   http.request.uri.path matches "^/api/v1/app/auth/" or
   http.request.uri.path eq "/oauth2/token")
Characteristics: ip.src
Period: 60 seconds
Requests: 10
Action: Block (duration: 60s)
```

```
# SCIM 端点限速 — 200 req/min per IP
Rule name: rate-limit-scim
Expression:
  http.request.uri.path matches "^/scim/v2/"
Characteristics: ip.src
Period: 60 seconds
Requests: 200
Action: Block (duration: 60s)
```

```
# 通用 API 限速 — 100 req/min per IP
Rule name: rate-limit-api
Expression:
  (http.request.uri.path matches "^/api/v1/" and
   not http.request.uri.path matches "^/api/v1/auth/" and
   not http.request.uri.path matches "^/api/v1/app/auth/" and
   not http.request.uri.path matches "^/api/v1/gateway/" and
   not http.request.uri.path matches "^/api/v1/enterprise/hris-webhook/")
Characteristics: ip.src
Period: 60 seconds
Requests: 100
Action: Managed Challenge
```

### 2.4 Bot Protection

Cloudflare Bot Management 配置：

```
# 允许已验证的合法 Bot（SCIM 客户端、Webhook 发送方）
Rule: bot-allow-verified
Expression:
  (cf.bot_management.verified_bot and
   (http.request.uri.path matches "^/scim/v2/" or
    http.request.uri.path matches "^/api/v1/integrations/lark/events"))
Action: Allow

# 对可疑自动化流量发起 Challenge
Rule: bot-challenge-likely
Expression:
  (cf.bot_management.score lt 30 and
   not cf.bot_management.verified_bot and
   http.request.uri.path matches "^/api/v1/")
Action: Managed Challenge

# 阻断明确的恶意 Bot
Rule: bot-block-definite
Expression:
  (cf.bot_management.score lt 5 and
   not cf.bot_management.verified_bot)
Action: Block
```

---

## 3. 白名单规则

### 3.1 SCIM 端点 — Okta / Entra ID

SCIM 客户端（Okta、Microsoft Entra ID）通过固定 IP 段推送用户同步数据：

```
# Cloudflare WAF Exception Rule: scim-idp-allowlist
Rule name: allow-scim-idp-providers
Expression:
  http.request.uri.path matches "^/scim/v2/" and
  (
    # Okta IP 范围（参考 https://help.okta.com/en-us/content/topics/security/ip-address-allow-listing.htm）
    ip.src in {
      100.89.16.0/20
      100.89.32.0/20
      100.89.48.0/20
      100.89.64.0/20
    } or
    # Microsoft Entra ID IP 范围（参考 AzureAD ServiceTag）
    ip.src in {
      20.190.128.0/18
      40.126.0.0/18
      20.99.160.0/20
    }
  )
Action: Skip (skip all remaining WAF rules)
```

> **注意：** Okta 和 Entra ID 的 IP 范围会定期更新。建议通过 Cloudflare API 定期同步：
> - Okta: `https://s3.amazonaws.com/okta-ip-ranges/ip_ranges.json`
> - Entra ID: `https://www.microsoft.com/en-us/download/details.aspx?id=56519`

### 3.2 Webhook 端点 — Lark / Meta

```
# Lark 事件回调白名单
Rule name: allow-lark-webhook
Expression:
  http.request.uri.path eq "/api/v1/integrations/lark/events" and
  ip.src in {
    # Lark/Feishu 服务器 IP（参考飞书开放平台文档）
    124.156.36.0/24
    101.32.179.0/24
    43.159.64.0/24
    # 新加坡节点（印尼客户可能经由此节点）
    43.129.0.0/16
  }
Action: Skip (skip all remaining WAF rules)
```

```
# HRIS Webhook 白名单（按客户 connector 配置动态维护）
Rule name: allow-hris-webhook
Expression:
  http.request.uri.path matches "^/api/v1/enterprise/hris-webhook/" and
  ip.src in {$HRIS_PROVIDER_IP_LIST}
Action: Skip (skip all remaining WAF rules)
```

### 3.3 Gateway Bootstrap — 客户网关 IP

Gateway Agent 从客户现场连回云端，IP 不固定。使用 API Token 认证 + IP 列表双重验证：

```
# 网关设备白名单（通过 Cloudflare IP List 动态管理）
# 1. 创建 IP List: "gateway-agent-ips"
# 2. 通过 API 动态添加客户网关 IP

Rule name: allow-gateway-bootstrap
Expression:
  http.request.uri.path matches "^/api/v1/gateway/" and
  ip.src in $gateway_agent_ips
Action: Skip (skip WAF managed rules, keep rate limiting)
```

> **运维建议：** 新增客户网关时，通过 Cloudflare API 自动添加 IP 到列表：
> ```bash
> curl -X POST "https://api.cloudflare.com/client/v4/accounts/{account_id}/rules/lists/{list_id}/items" \
>   -H "Authorization: Bearer $CF_API_TOKEN" \
>   -H "Content-Type: application/json" \
>   -d '[{"ip":"<customer-gateway-ip>","comment":"客户: XXX 公寓"}]'
> ```

### 3.4 ZKTeco Push 端点 — 设备 IP

```
# ZKTeco / Hikvision 设备推送白名单
Rule name: allow-southbound-devices
Expression:
  http.request.uri.path matches "^/api/v1/gateway/southbound/" and
  ip.src in $southbound_device_ips
Action: Skip (skip all remaining WAF rules)
```

---

## 4. CORS 配置

### 4.1 当前代码行为

MistyPass API 服务器通过 `withCORS` 中间件处理 CORS（见 `router_middleware.go`），
支持以下 header：

- `Access-Control-Allow-Headers: Authorization, Content-Type`
- `Access-Control-Allow-Methods: GET, POST, PUT, PATCH, DELETE, OPTIONS`
- `Access-Control-Expose-Headers: X-Collection-Range, Deprecation, Link, X-MistyPass-Replacement`

### 4.2 生产环境 WAF 层 CORS 加固

在 Cloudflare 通过 Transform Rules 强制覆盖 CORS header（防止源站配置错误）：

```
# Cloudflare Transform Rule: enforce-cors-production
Match: http.response.headers["access-control-allow-origin"][0] eq "*"
Set header:
  Access-Control-Allow-Origin = "https://admin.mistypass.com"
```

**生产环境必须配置的 CORS 源（`CORS_ORIGIN` 环境变量）：**

```
# .env 生产配置
CORS_ORIGIN=https://admin.mistypass.com
```

禁止使用 `*` 通配符。如有多个前端域名，用逗号分隔：

```
CORS_ORIGIN=https://admin.mistypass.com,https://app.mistypass.com
```

---

## 5. TLS 配置

### 5.1 Cloudflare SSL/TLS 设置

```
SSL/TLS 加密模式: Full (Strict)
最低 TLS 版本: TLS 1.2
TLS 1.3: 开启
自动 HTTPS 重写: 开启
Always Use HTTPS: 开启
```

### 5.2 HSTS 配置

通过 Cloudflare 或源站 header 设置：

```
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
```

Cloudflare Dashboard: SSL/TLS > Edge Certificates > HSTS > Enable

| 参数 | 值 | 说明 |
|------|-----|------|
| `max-age` | `31536000` (1 年) | 强制 HTTPS 缓存时间 |
| `includeSubDomains` | `true` | 覆盖所有子域 |
| `preload` | `true` | 提交到浏览器 HSTS preload list |

### 5.3 源站 TLS（Cloudflare → Origin）

```
# Caddy 源站 TLS 配置
mistypass-api:8080 {
    tls /etc/ssl/origin-cert.pem /etc/ssl/origin-key.pem {
        protocols tls1.2 tls1.3
        ciphers TLS_AES_128_GCM_SHA256 TLS_AES_256_GCM_SHA384 TLS_CHACHA20_POLY1305_SHA256
    }
}
```

使用 Cloudflare Origin CA 证书（15 年有效期，仅 Cloudflare 代理可信任）。

---

## 6. DDoS 防护策略

### 6.1 Cloudflare DDoS 防护层级

| 层级 | 防护手段 | 配置 |
|------|----------|------|
| L3/L4 | Cloudflare Spectrum / Magic Transit | 自动（Pro 计划已包含） |
| L7 HTTP Flood | Rate Limiting + Bot Management | 见第 2.3 / 2.4 节 |
| 应用层慢速攻击 | Cloudflare 超级 Bot 战斗模式 | Security > Bots > 开启 |

### 6.2 DDoS 特别防护规则

```
# 针对登录端点的 Credential Stuffing 防护
Rule name: ddos-auth-protect
Expression:
  http.request.uri.path matches "^/api/v1/auth/login$" and
  cf.threat_score gt 10
Action: Managed Challenge

# 大流量异常检测
Rule name: ddos-burst-protect
Expression:
  http.request.uri.path matches "^/api/v1/" and
  cf.threat_score gt 25
Action: Block
```

### 6.3 源站级保护

确保源站 IP 不被直接暴露：

1. 源站 IP 不写入 DNS（只用 Cloudflare Proxy）
2. 源站防火墙只允许 Cloudflare IP 段访问 80/443 端口
3. 使用 Authenticated Origin Pull（mTLS）验证请求来自 Cloudflare

```bash
# iptables: 只允许 Cloudflare IP 访问源站
# Cloudflare IPv4 ranges: https://www.cloudflare.com/ips-v4
for ip in $(curl -s https://www.cloudflare.com/ips-v4); do
  iptables -A INPUT -p tcp --dport 443 -s $ip -j ACCEPT
done
iptables -A INPUT -p tcp --dport 443 -j DROP
```

---

## 7. 日志与告警

### 7.1 Cloudflare 日志推送

使用 Cloudflare Logpush 将 WAF 日志推送到分析平台：

```bash
# 创建 Logpush Job（推送到 GCS 或 S3）
curl -X POST "https://api.cloudflare.com/client/v4/zones/{zone_id}/logpush/jobs" \
  -H "Authorization: Bearer $CF_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mistypass-waf-logs",
    "output_options": {
      "field_names": [
        "ClientIP","ClientRequestHost","ClientRequestMethod",
        "ClientRequestURI","EdgeResponseStatus","WAFAction",
        "WAFRuleID","WAFRuleMessage","BotScore","BotScoreSrc"
      ],
      "timestamp_format": "rfc3339"
    },
    "destination_conf": "gs://mistypass-logs/waf/?project_id=mistypass-prod",
    "dataset": "firewall_events",
    "enabled": true
  }'
```

### 7.2 告警规则

在 Cloudflare Notifications 或 Grafana 中配置：

| 告警名称 | 条件 | 通知渠道 | 严重级别 |
|----------|------|----------|----------|
| WAF 拦截飙升 | Block 事件 > 100/5min | Lark Bot + 邮件 | Warning |
| 登录暴力破解 | Auth rate limit 触发 > 50/5min | Lark Bot + WhatsApp | Critical |
| DDoS 攻击检测 | Challenge 事件 > 1000/5min | Lark Bot + WhatsApp + 短信 | Critical |
| SCIM 同步失败 | SCIM 端点 5xx > 10/5min | Lark Bot | Warning |
| Bot 异常流量 | Bot score < 10 的请求 > 200/5min | Lark Bot | Info |

**Lark Bot 告警集成示例**（复用 MistyPass 已有的 Lark 告警基础设施）：

```bash
# 通过 Cloudflare Worker 转发告警到 MistyPass Lark Bot
curl -X POST "https://api.mistypass.com/api/v1/integrations/lark/bot/alert" \
  -H "Authorization: Bearer $INTERNAL_ALERT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "WAF 告警: 登录暴力破解",
    "severity": "critical",
    "details": "过去 5 分钟内检测到 50+ 次登录限速触发"
  }'
```

---

## 8. Cloudflare 推荐配置（印尼区域）

### 8.1 为什么选择 Cloudflare

- **Jakarta PoP 节点**：印尼用户延迟 < 5ms（RTT），优于 AWS ap-southeast-1（新加坡，约 20-30ms）
- **Surabaya / Denpasar PoP**：覆盖印尼东部用户
- **一体化安全**：WAF + DDoS + CDN + DNS 一站式管理
- **性价比**：Pro 计划 $20/月已包含大部分 WAF 功能

### 8.2 推荐计划

| 功能 | Free | Pro ($20/月) | Business ($200/月) | Enterprise |
|------|------|-------------|-------------------|-----------|
| OWASP 托管规则 | - | 5 条自定义 | 25 条自定义 | 无限 |
| Rate Limiting | 1 条 | 10 条 | 25 条 | 无限 |
| Bot Management | 基础 | Super Bot Fight Mode | 高级 Bot 分析 | 完整 Bot Management |
| DDoS | L3/L4 基础 | L7 高级 | L7 高级 | 自定义 |
| Logpush | - | - | 部分 | 完整 |
| **推荐** | 开发环境 | **MVP 生产** | 规模化生产 | 大客户 |

**MVP 阶段推荐 Pro 计划**，后续客户量增长后升级到 Business。

### 8.3 完整 Cloudflare 配置清单

```yaml
# Cloudflare 配置清单（通过 Terraform 或 Dashboard 配置）

dns:
  - name: admin.mistypass.com     # Web Admin SPA
    type: CNAME
    content: mistypass-origin.example.com
    proxied: true                  # 必须开启 Proxy（橙色云图标）
  - name: api.mistypass.com       # API 服务器
    type: CNAME
    content: mistypass-origin.example.com
    proxied: true

ssl_tls:
  encryption_mode: full_strict
  min_tls_version: "1.2"
  tls_1_3: enabled
  hsts:
    enabled: true
    max_age: 31536000
    include_subdomains: true
    preload: true
  always_use_https: true
  automatic_https_rewrites: true

security:
  security_level: medium           # 印尼地区建议 medium，避免误拦合法用户
  challenge_ttl: 3600
  browser_integrity_check: true
  hotlink_protection: true

waf:
  managed_rules:
    cloudflare_managed:
      sensitivity: high
    owasp_core_ruleset:
      sensitivity: high
      paranoia_level: PL1          # PL1 适合生产环境，PL2 会产生过多误报

  custom_rules:
    # 见第 2 节配置

  rate_limiting:
    # 见第 2.3 节配置

  ip_access_rules:
    # 见第 3 节白名单配置

caching:
  # SPA 静态资源缓存
  page_rules:
    - match: "admin.mistypass.com/assets/*"
      cache_level: cache_everything
      edge_ttl: 86400
    - match: "api.mistypass.com/api/*"
      cache_level: bypass             # API 请求不缓存

network:
  websockets: enabled                 # 如果未来需要实时推送
  http2: enabled
  http3: enabled                      # QUIC — 印尼移动网络受益大
  0rtt: enabled
  ip_geolocation: enabled

firewall:
  # 源站保护 — 只允许 Cloudflare IP 访问源站
  authenticated_origin_pulls: true
```

### 8.4 Terraform 配置参考

```hcl
# cloudflare.tf — WAF Rate Limiting Rules

resource "cloudflare_ruleset" "waf_rate_limit" {
  zone_id = var.cloudflare_zone_id
  name    = "MistyPass WAF Rate Limiting"
  kind    = "zone"
  phase   = "http_ratelimit"

  # 登录端点 — 10 req/min
  rules {
    action = "block"
    action_parameters {
      response {
        status_code = 429
        content_type = "application/json"
        content = "{\"error\":\"rate_limit_exceeded\",\"message\":\"请稍后重试\"}"
      }
    }
    ratelimit {
      characteristics     = ["ip.src"]
      period              = 60
      requests_per_period = 10
      mitigation_timeout  = 60
    }
    expression = "(http.request.uri.path matches \"^/api/v1/auth/\" or http.request.uri.path matches \"^/api/v1/app/auth/\" or http.request.uri.path eq \"/oauth2/token\")"
    description = "Rate limit auth endpoints - 10 req/min"
    enabled = true
  }

  # SCIM — 200 req/min
  rules {
    action = "block"
    action_parameters {
      response {
        status_code = 429
        content_type = "application/json"
        content = "{\"error\":\"rate_limit_exceeded\"}"
      }
    }
    ratelimit {
      characteristics     = ["ip.src"]
      period              = 60
      requests_per_period = 200
      mitigation_timeout  = 60
    }
    expression = "http.request.uri.path matches \"^/scim/v2/\""
    description = "Rate limit SCIM endpoints - 200 req/min"
    enabled = true
  }

  # 通用 API — 100 req/min
  rules {
    action = "managed_challenge"
    ratelimit {
      characteristics     = ["ip.src"]
      period              = 60
      requests_per_period = 100
      mitigation_timeout  = 60
    }
    expression = "(http.request.uri.path matches \"^/api/v1/\" and not http.request.uri.path matches \"^/api/v1/auth/\" and not http.request.uri.path matches \"^/api/v1/gateway/\")"
    description = "Rate limit API endpoints - 100 req/min"
    enabled = true
  }
}

resource "cloudflare_ruleset" "waf_custom_rules" {
  zone_id = var.cloudflare_zone_id
  name    = "MistyPass WAF Custom Rules"
  kind    = "zone"
  phase   = "http_request_firewall_custom"

  # SCIM 白名单
  rules {
    action     = "skip"
    action_parameters {
      ruleset = "current"
    }
    expression = "(http.request.uri.path matches \"^/scim/v2/\" and ip.src in $okta_entra_ips)"
    description = "Allow SCIM from Okta/Entra ID"
    enabled = true
    logging {
      enabled = true
    }
  }

  # Gateway 白名单
  rules {
    action     = "skip"
    action_parameters {
      ruleset = "current"
    }
    expression = "(http.request.uri.path matches \"^/api/v1/gateway/\" and ip.src in $gateway_agent_ips)"
    description = "Allow gateway bootstrap from customer sites"
    enabled = true
  }
}
```

---

## 附录：ModSecurity 等效规则参考

如果使用 Caddy/Nginx + ModSecurity 方案，以下为等效核心规则：

```apache
# /etc/modsecurity/mistypass-custom.conf

# 启用 OWASP CRS
Include /etc/modsecurity/crs/crs-setup.conf
Include /etc/modsecurity/crs/rules/*.conf

# 登录限速 — 10 req/min
SecRule IP:AUTH_COUNTER "@ge 10" \
    "id:90001,phase:1,deny,status:429,\
     msg:'Auth rate limit exceeded'"
SecRule REQUEST_URI "@rx ^/api/v1/auth/" \
    "id:90002,phase:1,pass,nolog,\
     setvar:IP.AUTH_COUNTER=+1,\
     expirevar:IP.AUTH_COUNTER=60"

# SCIM 白名单
SecRule REQUEST_URI "@beginsWith /scim/v2/" \
    "id:90010,phase:1,pass,nolog,\
     chain"
  SecRule REMOTE_ADDR "!@ipMatch 100.89.16.0/20,100.89.32.0/20,20.190.128.0/18,40.126.0.0/18" \
      "deny,status:403,msg:'SCIM request from unknown IP'"

# Gateway 白名单
SecRule REQUEST_URI "@beginsWith /api/v1/gateway/" \
    "id:90020,phase:1,pass,nolog,\
     chain"
  SecRule REMOTE_ADDR "!@ipMatchFromFile /etc/modsecurity/gateway-ips.txt" \
      "deny,status:403,msg:'Gateway request from unknown IP'"
```

---

## 上线检查清单

- [ ] Cloudflare Proxy 已开启（DNS 橙色云图标）
- [ ] SSL/TLS 设为 Full (Strict)
- [ ] 最低 TLS 版本设为 1.2
- [ ] HSTS 已开启（max-age=31536000, includeSubDomains, preload）
- [ ] OWASP 托管规则已开启（High 敏感度）
- [ ] 四级 Rate Limiting 规则已配置
- [ ] SCIM 白名单已添加 Okta/Entra ID IP 段
- [ ] Webhook 白名单已添加 Lark IP 段
- [ ] Gateway IP List 已创建并添加初始客户 IP
- [ ] Bot Management 已开启（Super Bot Fight Mode）
- [ ] Logpush 已配置推送到日志平台
- [ ] 告警规则已配置（WAF 拦截飙升、暴力破解、DDoS）
- [ ] 源站防火墙已限制只允许 Cloudflare IP 访问
- [ ] Authenticated Origin Pull 已开启
- [ ] CORS_ORIGIN 已设为生产域名（非 `*`）
- [ ] 生产环境 `.env` 中 `APP_ENV=production`
- [ ] 上线前进行 WAF 规则 dry-run（日志模式观察 24h，确认无误报后切换为 Block）
