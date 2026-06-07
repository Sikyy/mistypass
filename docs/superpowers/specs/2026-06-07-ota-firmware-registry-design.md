# OTA 固件制品仓库 — 设计文档

> 日期：2026-06-07
> 状态：设计已确认,待出实施计划
> 上层目标:**全面对标 Kisi OTA**。本文是子项 **#2**(#1 版本可见性已完成)。

---

## 1. 背景

OTA 签名(PR #145)已做齐安全核心;#1 给了舰队版本可见性。但发布固件仍要**管理员自己托管二进制、手动把 URL/sha256/signature 贴进 OTA 任务**。#2 把固件**收进平台**:上传一次签名固件 → 平台存储 + 版本目录 + 分发;建 OTA 任务时引用某版本,服务端自动填 sha256/signature/url。

### 复用的现有基建(已核实)
- `routes_uploads.go`:HMAC-SHA256 签名上传/下载 URL,存储是**本地 FS**(`s.cfg.UploadStorageDir`,`{id[:2]}/{id}` 分片);签名密钥 `s.cfg.UploadSigningKey`。
- 现有签名下载端点已做"公开 + HMAC 校验"范式(企业安全审计加固过)——固件下载照此。
- OTA 任务现状:`firmware_url`/`firmware_sha256`/`firmware_signature` 由管理员手填(`CreateOTATask`,`service.go`)。

---

## 2. 已确认的关键决策
| 决策 | 选定 |
|---|---|
| 仓库模型 | **版本 + 可选通道标签**(`channel` 可空;#2 只存不 act,给 #3 金丝雀 / #4 自动跟通道留维度) |
| 分发机制 | **config/pull 时生成短时 HMAC 签名 URL**(复用 `UploadSigningKey`),agent 照常 plain GET;每轮重生 → 离线网关回来也不过期 |
| 上传流程 | **专用固件 multipart 上传端点**(一次传二进制+元数据),复用存储目录+签名密钥,不套用 user/purpose 绑定的两步签名上传流 |
| 隔离 | firmware **按租户隔离**(与 gateways/OTA 任务一致) |

> Ed25519 签名仍是真正的完整性/真实性锚(agent 验签后才刷)。签名下载 URL 只是**访问控制** + 防 URL 篡改,不影响更新安全。

---

## 3. 数据模型 — 固件记录
放新文件 `api/internal/modules/gateway/firmware.go`(不撑大 service.go):
```go
type GatewayFirmware struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Version    string    `json:"version"`
	Channel    string    `json:"channel,omitempty"`     // stable/beta/...，可空
	SHA256     string    `json:"sha256"`                 // lowercase hex
	Signature  string    `json:"signature"`              // Ed25519 hex (格式校验，验签在 agent)
	StorageKey string    `json:"-"`                      // 本地 FS 相对 key（不外泄）
	SizeBytes  int64     `json:"size_bytes"`
	UploadedBy string    `json:"uploaded_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
```

---

## 4. 组件设计(6 个单元,纯后端)

### 4.1 仓库存储(`gateway/firmware.go`)
- `CreateFirmware(in CreateFirmwareInput) (GatewayFirmware, error)` — 校验必填(tenant/version/sha256/signature 格式 + StorageKey)、生成 id、写记录、`persistLocked()`。
- `ListFirmware(tenantID, channel string) []GatewayFirmware` — 租户范围(channel 非空则过滤),按 CreatedAt 倒序。
- `GetFirmware(tenantID, id string) (GatewayFirmware, error)` — 租户隔离(用于建任务/列表),不存在 → `ErrGatewayFirmwareNotFound`。
- `GetFirmwareByID(id string) (GatewayFirmware, error)` — **无租户**,仅供签名下载端点用。隔离由"建任务时 `GetFirmware(tenant,id)` 校验 + 签名 URL 只能由持密钥的服务端生成 + id 不可猜"三重保证,故下载按 id 取安全。
- 二进制存储:复用 `UploadStorageDir` 下的固件子目录(分片),StorageKey = 安全 id(无路径穿越)。

### 4.2 上传端点 `POST /api/v1/gateways/firmware`(admin,multipart)
- 角色:`super_admin`/`tenant_admin`/`building_admin`。
- 表单:`version`、`channel`(可空)、`sha256`、`signature` + 文件部分。
- 服务端:读文件(带大小上限,如 512 MiB)→ 算 `sha256(bytes)` 与传入比对(不符 → 400,防上传损坏)→ 校验 signature 是 128-hex Ed25519 格式 → 写二进制到 `UploadStorageDir` → `CreateFirmware` → 201 返回记录。
- 未配 `UploadStorageDir`/`UploadSigningKey` → 503。

### 4.3 列表端点 `GET /api/v1/gateways/firmware`(admin)
- 角色同上 + `operator`。`?channel=` 可选过滤。返回 `{items: [...]}`(不含 StorageKey)。

### 4.4 建 OTA 任务引用仓库(`service.go` `CreateOTATask`)
- 新增可选入参 `firmwareID`;OTA 任务结构加 `FirmwareID string`。
- 给了 `firmware_id` → `GetFirmware` 取记录,用其 `SHA256`/`Signature` 填任务(`FirmwareURL` 留空,config/pull 动态填),记 `FirmwareID`。
- 未给 `firmware_id` → 保持原手动路径(`firmware_url`+`sha256`+`signature` 必填,外部托管,**向后兼容**)。
- HTTP 层 `createGatewayOTATask` 请求体加可选 `firmware_id`;二选一(给了 firmware_id 时 url/sha/sig 由仓库填)。

### 4.5 config/pull 动态填签名 URL(`router_handlers_gateway.go`)
- 现有 `pending_ota_tasks` 填充处:对每个 `FirmwareID != ""` 的 pending 任务,用 `UploadSigningKey` 生成短时签名 URL 设为该任务的 `FirmwareURL`:
  - canonical message = `firmwareID + "|" + expiryUnix`;`sig = hex(HMAC_SHA256(UploadSigningKey, message))`;URL = `<base>/api/v1/gateway/firmware/{id}?exp=<unix>&sig=<sig>`。
  - 过期窗口宽松(如 10 分钟);每次 config/pull 重生,故离线网关回来也能拿到新鲜 URL。

### 4.6 固件下载端点 `GET /api/v1/gateway/firmware/{id}`(公开 + HMAC)
- **非**设备 token 鉴权(agent plain GET);校验 `exp`(未过期)+ `sig`(HMAC 重算相等,`hmac.Equal` 常数时间)→ `GetFirmwareByID(id)`(无租户,见 §4.1 隔离论证)→ serve 存储二进制(`Content-Type: application/octet-stream`,带 `Content-Length`)。
- 路径安全:用记录里的 `StorageKey` 拼绝对路径,**绝不**用 URL 里的 `{id}` 直接拼文件路径(防穿越);`{id}` 仅用于查记录。

---

## 5. 数据流
```
admin POST /gateways/firmware（签名固件 + 元数据） → 仓库存二进制+记录
admin POST /gateways/{id}/ota/tasks {firmware_id} → 任务带 sha256+signature（自仓库）+ FirmwareID
agent pullConfig → 服务端为该任务生成新鲜 HMAC 签名 URL 填 firmware_url
agent plain GET 签名 URL → 下载端点验 HMAC+exp → serve → agent 验 sha256+Ed25519（任务里的）→ 刷
```

---

## 6. 错误处理 / 边界
| 情况 | 行为 |
|---|---|
| 上传 sha256 与字节不符 | 400(防上传损坏) |
| signature 非 128-hex | 400(格式) |
| `UploadStorageDir`/`UploadSigningKey` 未配 | 503 |
| `firmware_id` 不存在/跨租户 | `CreateOTATask` 返回 404/not-found |
| 下载签名过期/篡改 | 403 |
| 文件超大 | 上传按 `MaxBytesReader` 截断 → 400 |
| 手动路径(无 firmware_id) | 原样保留,向后兼容 |

---

## 7. 测试
- **firmware store**(`firmware_test.go`):Create/List/Get + 租户隔离 + channel 过滤。
- **upload handler**:sha256 比对(符/不符)、signature 格式、存储落盘、503 未配。
- **CreateOTATask**:给 firmware_id → 任务带仓库的 sha/sig + FirmwareID;不存在的 firmware_id → 错误。
- **config/pull**:带 FirmwareID 的 pending 任务 → 响应里 firmware_url 是有效签名 URL(能被下载端点验过)。
- **download handler**:合法签名 → 200 + 字节;过期/坏签名 → 403;路径穿越尝试无效(用 StorageKey)。

---

## 8. 改动文件
**新增**
- `api/internal/modules/gateway/firmware.go` — `GatewayFirmware` + store 方法 + 错误
- `api/internal/modules/gateway/firmware_test.go`
- `api/internal/http/routes_gateway_firmware.go` — 上传/列表/下载 handler + 签名 URL 助手
- 相应 http 测试

**修改**
- `api/internal/modules/gateway/service.go` — `CreateOTATask` 加 `firmwareID`;OTA 任务加 `FirmwareID`
- `api/internal/http/routes_gateway_management.go` — `createGatewayOTATask` 请求体加 `firmware_id`
- `api/internal/http/router_handlers_gateway.go` — config/pull 对 registry 任务填签名 URL
- `api/internal/http/router.go` — 注册 3 路由(上传/列表/下载)

---

## 9. 不做(YAGNI / 留后续子项)
UI(#5)、按通道 act(#3 金丝雀 / #4 自动跟通道)、delta/分片下载、device-type 维度(单一 gateway-agent 产品)、S3/R2 后端(复用本地 FS)。

## 10. 工作量
约 1–1.5 天(纯后端,含 multipart 上传 + HMAC 签名 URL + config/pull 集成)。
