# OTA 固件管理 UI — 设计文档(#5a)

> 日期：2026-06-08
> 状态：设计已确认,待出实施计划
> 上层目标:**全面对标 Kisi OTA**。本文是子项 **#5** 管理 UI 的第一块 **#5a 固件 UI**(后端 #1–#4 已完成)。
> 纯前端(web-admin React/TS);对标既有 web-admin 模式。

---

## 1. 背景与范围

OTA 后端(签名 #145 + 版本可见性 #1 + 固件仓库 #2 + 灰度 #3 + 调度 #4)已全部完成,但管理员只能走 API。#5 = 管理 UI,**拆两块**:**#5a 固件 UI**(本文)+ #5b rollout UI(后续)。

**#5a 范围**:新建独立 `/ota`「固件管理」区,含
- **固件版本总览**(#1):固件版本在舰队的分布。
- **固件仓库**(#2):上传签名固件(multipart)+ 列表。

### 已确认的关键决策
| 决策 | 选定 |
|---|---|
| 放置 | 新建独立 `/ota` 区(不堆进已 1650 行的 gateways 页) |
| 上传 sha256 | UI 客户端用 `crypto.subtle.digest('SHA-256')` 从文件算(secure context),管理员不手贴 |
| 签名 | **永远离线生成**(ota-sign,私钥绝不进浏览器);UI 只接收**粘贴**的 signature |

### 复用的现有 web-admin 模式(已核实)
- 栈:shadcn/ui + Tailwind + `@tanstack/react-query` v5 + react-hook-form + zod + react-i18next(en-US/id-ID/zh-CN)。
- API:`lib/api/core.ts` 的 `request<T>`/`requestItems<T>`(Bearer token,401 自动刷新,`API_BASE_URL`);tenant 经 `tenant_id` query(tenant-scoped 用户可省,服务端按 token 推断;super_admin 跨租户时传)。**目前无 multipart 上传** → 需加 `requestFormData`。
- 路由:React Router v7,lazy page,角色门控(`viewer.role`)。
- i18n:`useTranslation()` + `t('ota.firmware.…')`,三 locale 文件。
- 测试:**vitest + @testing-library**。
- web-admin 里 firmware/ota/rollout **零代码**(全新地)。

---

## 2. 真实后端契约(我建的,非猜测)
| 用途 | 调用 |
|---|---|
| 版本总览(#1) | `GET /api/v1/gateways/firmware-summary?tenant_id=` → `{total:int, reported:int, versions:[{version:string, count:int}]}` |
| 固件列表(#2) | `GET /api/v1/gateways/firmware?tenant_id=&channel=` → `{items: GatewayFirmware[]}` |
| 固件上传(#2) | `POST /api/v1/gateways/firmware?tenant_id=`(tenant 同走 **query**;multipart **表单字段仅** `version`/`channel`/`sha256`/`signature`/`file`,**无 tenant_id 表单字段**)→ `GatewayFirmware` |

`GatewayFirmware = {id, tenant_id, version, channel?, sha256, signature, size_bytes, uploaded_by?, created_at}`。上传错误:sha256 不符→400、signature 非 128-hex→400、缺 version→400、未配存储→503。角色:上传 = super_admin/tenant_admin/building_admin;读 = + operator。

---

## 3. 架构 — 新增结构
`web-admin/src/features/ota/`:
- `pages/firmware-page.tsx` — 固件管理页(组合三卡;按角色门控上传卡)
- `components/firmware-summary-card.tsx` — 版本总览
- `components/firmware-upload-card.tsx` — multipart 上传表单
- `components/firmware-list-card.tsx` — 版本列表表格
- `hooks/use-firmware.ts` — react-query hooks(`useFirmwareSummary`、`useFirmwareList`)

`web-admin/src/lib/`:
- `api/ota.ts` — `getFirmwareSummary`/`listFirmware`/`uploadFirmware`(用真实契约)
- `api/core.ts` — 加 `requestFormData<T>(path, formData, token?)`(不设 Content-Type 让浏览器自带 boundary;Bearer;401 刷新)
- `query-keys.ts` — 加 `firmwareSummary`/`firmwareList` keys

`web-admin/src/`:
- `features/mistyislet-shell/routes.tsx` — 注册 `/ota` lazy 路由 + 角色门控
- 导航(sidebar)加「固件管理」入口
- `locales/{en-US,id-ID,zh-CN}.json` — 加 `ota` 命名空间

---

## 4. 组件设计

### 4.1 `firmware-page.tsx`
- 接 `{ token, viewer }`(同 gateways page 模式);解析 effective tenant(super_admin 可选租户,tenant-scoped 用 token 租户)。
- 垂直卡片栈:总览卡 → 上传卡(仅 write 角色)→ 列表卡。
- 角色门控:`canUpload = ['super_admin','tenant_admin','building_admin'].includes(viewer.role)`。

### 4.2 `firmware-summary-card.tsx`
- `useFirmwareSummary(token, tenantID)` → 展示 `total` 网关、`reported` 数、版本分布(`versions` 每项 version + count,用 Badge/进度条列表)。
- 加载(skeleton)/空(无网关)/错误态。

### 4.3 `firmware-upload-card.tsx`(write 角色)
- react-hook-form + zod。字段:
  - `version`(必填,trim,如 `1.4.0`)
  - `channel`(可选,文本/下拉建议 `stable`/`beta`)
  - `signature`(必填,textarea;zod 校验 128-hex)
  - `file`(必填,file input)
- 提交:读 `file` 的 ArrayBuffer → `crypto.subtle.digest('SHA-256', buf)` → hex sha256 → `FormData{version,channel,sha256,signature,file}` → `uploadFirmware`。
- 成功:`form.reset()` + `queryClient.invalidateQueries` 列表 & 总览。失败:显后端错误消息(sha 不符/格式/503)。
- `crypto.subtle` 不可用(非 secure context)时:禁用提交 + 提示「需 HTTPS」。

### 4.4 `firmware-list-card.tsx`
- `useFirmwareList(token, tenantID, channel)` → Table(version / channel / sha256[截断+title 全量] / size_bytes[人类可读] / uploaded_by / created_at[本地化])。
- channel 过滤(Select:全部 / stable / beta / 自定义);加载/空/错误态。

---

## 5. 数据流
```
进 /ota → useFirmwareSummary + useFirmwareList(staleTime 30s)
上传成功 → invalidate firmwareList + firmwareSummary → 两卡刷新
channel 过滤变化 → useFirmwareList 重查(query key 含 channel)
```

---

## 6. 错误 / 边界
| 情况 | UI |
|---|---|
| 查询失败 | 卡片内错误条(消息来自 APIError) |
| 上传 sha256 不符/格式/503 | 表单下错误消息(后端文案) |
| crypto.subtle 不可用 | 禁用上传 + 「需 HTTPS/secure context」提示 |
| 空列表/无网关 | 友好空态 |
| 非 write 角色 | 不渲染上传卡(只读) |

---

## 7. i18n
`ota.firmware.*` 三语:页标题、总览(total/reported/版本)、上传(字段标签/占位/校验/成功/错误/「需 HTTPS」)、列表(表头/空态/channel 过滤项)。三 locale 文件同步加。

---

## 8. 测试(vitest + @testing-library)
- **`api/ota.ts`**:mock `fetch` —— 验 `getFirmwareSummary`/`listFirmware` 的 URL + tenant/channel query;`uploadFirmware` 组的 FormData 字段(version/channel/sha256/signature/file)+ 不设 Content-Type。
- **upload card**:填表(含选一个假 File)→ 提交 → 断言 `crypto.subtle.digest` 被调 + `uploadFirmware` 收到算出的 sha256;后端错误 → 显消息;非 secure context → 禁用。(`crypto.subtle` 在测试里 mock。)
- **list card**:渲染 items → 表格行;channel 过滤切换 → 重查。
- **summary card**:渲染分布;空态。
- **page**:write 角色见上传卡;非 write 角色不见。

---

## 9. 改动文件
**新增**:`features/ota/pages/firmware-page.tsx`、`features/ota/components/{firmware-summary,firmware-upload,firmware-list}-card.tsx`、`features/ota/hooks/use-firmware.ts`、`lib/api/ota.ts` + 各自 `.test.ts(x)`。
**修改**:`lib/api/core.ts`(+`requestFormData`)、`lib/query-keys.ts`、`features/mistyislet-shell/routes.tsx`、sidebar 导航、`locales/{en-US,id-ID,zh-CN}.json`。

---

## 10. 安全
- 签名**离线生成**(ota-sign);**私钥绝不进浏览器**;UI 只接收粘贴的 signature。
- sha256 客户端算只为 UX + 服务端仍校验 `sha256(bytes)==declared`(防上传损坏);真正完整性锚是 agent 端 Ed25519 验签。
- 上传走既有 Bearer + 401 刷新;tenant 服务端按 token 强制(UI 传的 tenant_id 仅 super_admin 跨租户)。

---

## 11. 不做(YAGNI / 留后续)
rollout UI(#5b)、per-gateway 当前版本表(已在 gateways 页)、固件删除/编辑(后端无端点)、浏览器内生成签名(私钥离线)、固件二进制预览/下载(管理端不需要)。

## 12. 工作量
约 1–1.5 天(纯前端;3 卡 + 上传 multipart + sha256 客户端 + API + 路由 + 三语 i18n + vitest)。
