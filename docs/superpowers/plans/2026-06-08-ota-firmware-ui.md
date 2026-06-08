# OTA 固件管理 UI(#5a)Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. 实现卡片/页面时可参考 frontend-design skill,但**严格对标既有 web-admin 模式**(见下方 mirror 参考)。

**Goal:** web-admin 新建 `/ota`「固件管理」区:固件版本总览(#1)+ 固件上传(multipart,UI 客户端算 sha256)/列表(#2),对标既有 web-admin 模式(shadcn + react-query + react-hook-form + zod + 三语 i18n)。

**Architecture:** 新 `features/ota/` 特性目录(自包含卡片用 hooks 直接查询,页面薄)。`lib/api/ota.ts` 用真实后端契约 + `lib/api/core.ts` 新增 `requestFormData`(multipart)。可测逻辑抽到 `firmware-utils.ts`(sha256/格式化/校验)按 web-admin 惯例单测;API 用 mock-fetch 测(mirror `lib/api.test.ts`)。

**Tech Stack:** React + TS、Vite、shadcn/ui、@tanstack/react-query v5、react-hook-form + zod、react-i18next、vitest。

设计依据:[2026-06-08-ota-firmware-ui-design.md](../specs/2026-06-08-ota-firmware-ui-design.md)

**约定 / mirror 参考(实现者务必读这些现有文件对齐风格):**
- 工作目录前端在 `/Users/siky/code/MistyPass/web-admin/`;命令在 `web-admin/` 下跑(`npm run test:unit`、`npm run lint`、`npm run build`)。
- API 助手:`src/lib/api/core.ts` 的 `request<T>`/`requestItems<T>`/`APIError`/`resolveAuthToken`/`parseAPIErrorDetails`/`clearSession`(import 自 `@/lib/auth`)。API barrel:测试 `import ... from "./api"` → 存在 `src/lib/api.ts` 或 `src/lib/api/index.ts` 桶文件;新函数加进该桶。
- 表单卡 mirror:`src/components/gateways/gateway-serial-inventory-ingest-card.tsx`(shadcn Card/Input/Select/Textarea/Button + react-hook-form + `zodResolver` + `useMemo(()=>z.object({…}),[t])`)。
- 表格卡 mirror:`src/components/gateways/gateway-list-card.tsx`(Table/Badge)。
- API 测试 mirror:`src/lib/api.test.ts`(vitest `describe/it/expect/vi`)。
- query-keys:`src/lib/query-keys.ts` 的 `k(...)`/`ns(...)` 工厂。
- 角色:`viewer.role ∈ {super_admin,tenant_admin,operator,building_admin,resident}`;上传 write 角色 = super_admin/tenant_admin/building_admin。

**真实后端契约(勿用猜测形状):**
- `GET /api/v1/gateways/firmware-summary?tenant_id=` → `{total, reported, versions:[{version,count}]}`
- `GET /api/v1/gateways/firmware?tenant_id=&channel=` → `{items:[{id,tenant_id,version,channel,sha256,signature,size_bytes,uploaded_by,created_at}]}`
- `POST /api/v1/gateways/firmware?tenant_id=`(multipart 表单字段仅 version/channel/sha256/signature/file)→ `GatewayFirmware`

---

## Task 1: API 层(requestFormData + ota.ts + query-keys)

**Files:**
- Modify: `web-admin/src/lib/api/core.ts`(加 `requestFormData`)
- Create: `web-admin/src/lib/api/ota.ts`
- Modify: `web-admin/src/lib/api/index.ts` 或 `src/lib/api.ts`(桶导出 ota.ts;实现者 grep `export .* from "./core"` 找桶文件)
- Modify: `web-admin/src/lib/query-keys.ts`
- Test: `web-admin/src/lib/api/ota.test.ts`

- [ ] **Step 1: 写失败测试**

Create `web-admin/src/lib/api/ota.test.ts`:
```ts
import { afterEach, describe, expect, it, vi } from "vitest"
import { getFirmwareSummary, listFirmware, uploadFirmware } from "./ota"

function mockFetchOnce(body: unknown, ok = true, status = 200) {
  const fn = vi.fn().mockResolvedValue({
    ok,
    status,
    statusText: ok ? "OK" : "Error",
    json: async () => body,
  } as Response)
  vi.stubGlobal("fetch", fn)
  return fn
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe("ota api", () => {
  it("getFirmwareSummary calls the summary endpoint with tenant_id", async () => {
    const fetchFn = mockFetchOnce({ total: 3, reported: 2, versions: [{ version: "1.4.0", count: 2 }] })
    const res = await getFirmwareSummary("tok", "tenant_demo_jakarta")
    expect(res.total).toBe(3)
    const url = String(fetchFn.mock.calls[0][0])
    expect(url).toContain("/api/v1/gateways/firmware-summary")
    expect(url).toContain("tenant_id=tenant_demo_jakarta")
  })

  it("listFirmware passes channel filter and unwraps items", async () => {
    const fetchFn = mockFetchOnce({ items: [{ id: "fw_1", version: "1.4.0", channel: "stable" }] })
    const res = await listFirmware("tok", "tenant_demo_jakarta", "stable")
    expect(res).toHaveLength(1)
    const url = String(fetchFn.mock.calls[0][0])
    expect(url).toContain("/api/v1/gateways/firmware?")
    expect(url).toContain("channel=stable")
  })

  it("uploadFirmware posts multipart FormData with the right fields and no JSON content-type", async () => {
    const fetchFn = mockFetchOnce({ id: "fw_1", version: "1.4.0" })
    const file = new File([new Uint8Array([1, 2, 3])], "gateway-agent")
    await uploadFirmware("tok", "tenant_demo_jakarta", {
      version: "1.4.0", channel: "stable", sha256: "a".repeat(64), signature: "b".repeat(128), file,
    })
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit]
    expect(String(url)).toContain("/api/v1/gateways/firmware?")
    expect(init.method).toBe("POST")
    expect(init.body).toBeInstanceOf(FormData)
    const fd = init.body as FormData
    expect(fd.get("version")).toBe("1.4.0")
    expect(fd.get("sha256")).toBe("a".repeat(64))
    expect(fd.get("signature")).toBe("b".repeat(128))
    expect(fd.get("file")).toBeInstanceOf(File)
    // browser sets multipart Content-Type automatically — we must NOT set application/json
    const headers = new Headers(init.headers)
    expect(headers.get("Content-Type")).toBeNull()
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web-admin && npm run test:unit -- src/lib/api/ota.test.ts`
Expected: FAIL(`Cannot find module './ota'`)。

- [ ] **Step 3: requestFormData(core.ts)**

在 `core.ts` 末尾(`request`/`requestItems` 之后)加(`clearSession` 已 import):
```ts
export async function requestFormData<T>(
  path: string,
  formData: FormData,
  token?: string | undefined,
  options: RequestOptions = {}
): Promise<T> {
  const headers = new Headers()
  const activeToken = resolveAuthToken(token)
  if (activeToken) {
    headers.set("Authorization", `Bearer ${activeToken}`)
  }
  // NO Content-Type: the browser sets multipart/form-data with the boundary.
  const response = await fetch(`${API_BASE_URL}${path}`, { method: "POST", headers, body: formData })

  if (response.status === 401 && activeToken && !options.skipAuthRecovery) {
    const refreshedToken = await refreshAccessTokenForFormData()
    if (refreshedToken) {
      return requestFormData<T>(path, formData, refreshedToken, { skipAuthRecovery: true })
    }
    throw new APIError(401, "Session expired, please sign in again")
  }
  if (!response.ok) {
    const errorDetails = await parseAPIErrorDetails(response)
    if (response.status === 401) {
      clearSession()
    }
    throw new APIError(response.status, errorDetails.message, { code: errorDetails.code, responseStatus: errorDetails.responseStatus })
  }
  if (response.status === 204) {
    return undefined as T
  }
  return (await response.json()) as T
}
```
注:`refreshAccessToken` 当前是文件内私有 fn。若它已可在文件内调用,直接用 `refreshAccessToken()`(不要新建 `refreshAccessTokenForFormData`);把上面 `refreshAccessTokenForFormData()` 替换为现有的 `refreshAccessToken()`。(实现者:读 core.ts 确认 `refreshAccessToken` 名称并直接复用,删掉占位名。)

- [ ] **Step 4: ota.ts**

Create `web-admin/src/lib/api/ota.ts`:
```ts
import { request, requestFormData, requestItems } from "./core"

export type FirmwareVersionCount = { version: string; count: number }
export type FirmwareSummary = { total: number; reported: number; versions: FirmwareVersionCount[] }
export type GatewayFirmware = {
  id: string
  tenant_id: string
  version: string
  channel?: string
  sha256: string
  signature: string
  size_bytes: number
  uploaded_by?: string
  created_at: string
}
export type UploadFirmwareInput = { version: string; channel?: string; sha256: string; signature: string; file: File }

function firmwareQuery(tenantID?: string, channel?: string): string {
  const params = new URLSearchParams()
  if (tenantID && tenantID.trim() !== "") params.set("tenant_id", tenantID.trim())
  if (channel && channel.trim() !== "") params.set("channel", channel.trim())
  const s = params.toString()
  return s ? `?${s}` : ""
}

export async function getFirmwareSummary(token: string | undefined, tenantID?: string): Promise<FirmwareSummary> {
  const res = await request<Partial<FirmwareSummary>>(
    `/api/v1/gateways/firmware-summary${firmwareQuery(tenantID)}`,
    { method: "GET" },
    token
  )
  return { total: res.total ?? 0, reported: res.reported ?? 0, versions: res.versions ?? [] }
}

export async function listFirmware(token: string | undefined, tenantID?: string, channel?: string): Promise<GatewayFirmware[]> {
  return requestItems<GatewayFirmware>(`/api/v1/gateways/firmware${firmwareQuery(tenantID, channel)}`, token)
}

export async function uploadFirmware(
  token: string | undefined,
  tenantID: string | undefined,
  input: UploadFirmwareInput
): Promise<GatewayFirmware> {
  const fd = new FormData()
  fd.set("version", input.version)
  if (input.channel && input.channel.trim() !== "") fd.set("channel", input.channel.trim())
  fd.set("sha256", input.sha256)
  fd.set("signature", input.signature)
  fd.set("file", input.file)
  return requestFormData<GatewayFirmware>(`/api/v1/gateways/firmware${firmwareQuery(tenantID)}`, fd, token)
}
```
桶导出:在 API barrel 文件(`src/lib/api/index.ts` 或 `src/lib/api.ts` —— grep `export .* from "./core"` 定位)加 `export * from "./ota"`(与既有模块导出风格一致)。

- [ ] **Step 5: query-keys.ts**

在 `queryKeys` 对象里加(放近 gateways 相关键或文件末尾分组):
```ts
  // --- OTA / Firmware ---
  firmwareSummary: (...args: string[]) => k("ota-firmware-summary", ...args),
  firmwareList: (...args: string[]) => k("ota-firmware-list", ...args),
```

- [ ] **Step 6: 运行测试 + lint**

Run: `cd web-admin && npm run test:unit -- src/lib/api/ota.test.ts && npm run lint 2>&1 | tail -5`
Expected: 3 测试 PASS;lint 无新错。

- [ ] **Step 7: 提交**

```bash
git add web-admin/src/lib/api/core.ts web-admin/src/lib/api/ota.ts web-admin/src/lib/api.ts web-admin/src/lib/api/index.ts web-admin/src/lib/query-keys.ts web-admin/src/lib/api/ota.test.ts
git commit -m "feat(web): OTA firmware API client + requestFormData"
```
(`git add` 桶文件按实际存在的那个;不存在的路径 git 会忽略报错——只 add 真实改动文件。)

---

## Task 2: firmware-utils(sha256 / 格式化 / 校验)+ 测试

**Files:**
- Create: `web-admin/src/features/ota/lib/firmware-utils.ts`
- Test: `web-admin/src/features/ota/lib/firmware-utils.test.ts`

- [ ] **Step 1: 写失败测试**

Create `web-admin/src/features/ota/lib/firmware-utils.test.ts`:
```ts
import { afterEach, describe, expect, it, vi } from "vitest"
import { computeSha256Hex, formatBytes, isSignatureHex, truncateHex } from "./firmware-utils"

afterEach(() => vi.restoreAllMocks())

describe("firmware-utils", () => {
  it("formatBytes humanizes sizes", () => {
    expect(formatBytes(0)).toBe("0 B")
    expect(formatBytes(1023)).toBe("1023 B")
    expect(formatBytes(1024)).toBe("1.0 KB")
    expect(formatBytes(1024 * 1024 * 5)).toBe("5.0 MB")
    expect(formatBytes(-1)).toBe("—")
  })

  it("truncateHex shortens long hex with ellipsis", () => {
    expect(truncateHex("abcd")).toBe("abcd")
    expect(truncateHex("a".repeat(64))).toContain("…")
    expect(truncateHex("a".repeat(64)).length).toBeLessThan(64)
  })

  it("isSignatureHex requires exactly 128 hex chars", () => {
    expect(isSignatureHex("b".repeat(128))).toBe(true)
    expect(isSignatureHex("  " + "B".repeat(128) + "  ")).toBe(true)
    expect(isSignatureHex("b".repeat(127))).toBe(false)
    expect(isSignatureHex("z".repeat(128))).toBe(false)
  })

  it("computeSha256Hex hex-encodes the platform digest", async () => {
    // mock crypto.subtle.digest to a known 32-byte buffer → assert our hex encoding
    const digest = new Uint8Array(32).fill(0xab).buffer
    const subtle = { digest: vi.fn().mockResolvedValue(digest) }
    vi.stubGlobal("crypto", { subtle })
    const file = new File([new Uint8Array([1, 2, 3])], "fw")
    const hex = await computeSha256Hex(file)
    expect(hex).toBe("ab".repeat(32))
    expect(subtle.digest).toHaveBeenCalledWith("SHA-256", expect.anything())
    vi.unstubAllGlobals()
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web-admin && npm run test:unit -- src/features/ota/lib/firmware-utils.test.ts`
Expected: FAIL(`Cannot find module './firmware-utils'`)。

- [ ] **Step 3: firmware-utils.ts**

Create `web-admin/src/features/ota/lib/firmware-utils.ts`:
```ts
// Pure helpers for the firmware UI (unit-tested per web-admin convention).

export function isCryptoSubtleAvailable(): boolean {
  return typeof crypto !== "undefined" && typeof crypto.subtle !== "undefined" && typeof crypto.subtle.digest === "function"
}

export async function computeSha256Hex(file: File): Promise<string> {
  const buf = await file.arrayBuffer()
  const digest = await crypto.subtle.digest("SHA-256", buf)
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("")
}

export function isSignatureHex(value: string): boolean {
  return /^[0-9a-fA-F]{128}$/.test(value.trim())
}

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "—"
  const units = ["B", "KB", "MB", "GB"]
  let value = bytes
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i += 1
  }
  return `${i === 0 ? Math.round(value) : value.toFixed(1)} ${units[i]}`
}

export function truncateHex(hex: string, head = 8, tail = 6): string {
  const h = hex.trim()
  if (h.length <= head + tail + 1) return h
  return `${h.slice(0, head)}…${h.slice(-tail)}`
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd web-admin && npm run test:unit -- src/features/ota/lib/firmware-utils.test.ts`
Expected: 4 测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add web-admin/src/features/ota/lib/firmware-utils.ts web-admin/src/features/ota/lib/firmware-utils.test.ts
git commit -m "feat(web): firmware-utils (sha256, formatBytes, validation)"
```

---

## Task 3: hooks + 总览卡 + 列表卡(读)

**Files:**
- Create: `web-admin/src/features/ota/hooks/use-firmware.ts`
- Create: `web-admin/src/features/ota/components/firmware-summary-card.tsx`
- Create: `web-admin/src/features/ota/components/firmware-list-card.tsx`

> i18n key 在 Task 5 统一加;本任务卡片用的 `t("ota.firmware.…")` key 先写上,Task 5 补 locale(开发时缺失 key 会回落显示 key 字符串,不报错)。

- [ ] **Step 1: hooks(use-firmware.ts)**

Create `web-admin/src/features/ota/hooks/use-firmware.ts`:
```ts
import { useQuery } from "@tanstack/react-query"
import { getFirmwareSummary, listFirmware } from "@/lib/api/ota"
import { queryKeys } from "@/lib/query-keys"

export function useFirmwareSummary(token: string | undefined, tenantID: string | undefined) {
  return useQuery({
    queryKey: queryKeys.firmwareSummary(tenantID ?? "self"),
    queryFn: () => getFirmwareSummary(token, tenantID),
    staleTime: 30 * 1000,
  })
}

export function useFirmwareList(token: string | undefined, tenantID: string | undefined, channel: string) {
  return useQuery({
    queryKey: queryKeys.firmwareList(tenantID ?? "self", channel || "all"),
    queryFn: () => listFirmware(token, tenantID, channel || undefined),
    staleTime: 30 * 1000,
  })
}
```

- [ ] **Step 2: 总览卡(firmware-summary-card.tsx)**

Create `web-admin/src/features/ota/components/firmware-summary-card.tsx` —— mirror shadcn Card 风格(`gateway-list-card.tsx`):
```tsx
import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useFirmwareSummary } from "../hooks/use-firmware"

export function FirmwareSummaryCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t } = useTranslation()
  const query = useFirmwareSummary(token, tenantID)
  const summary = query.data
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.firmware.summary.title")}</CardTitle>
        <CardDescription>{t("ota.firmware.summary.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {query.isPending ? (
          <p className="text-sm text-muted-foreground">{t("ota.firmware.summary.loading")}</p>
        ) : query.error ? (
          <p className="text-sm text-destructive">{query.error instanceof Error ? query.error.message : t("ota.firmware.errors.generic")}</p>
        ) : !summary || summary.total === 0 ? (
          <p className="text-sm text-muted-foreground">{t("ota.firmware.summary.empty")}</p>
        ) : (
          <div className="space-y-3">
            <p className="text-sm">
              {t("ota.firmware.summary.fleet", { total: summary.total, reported: summary.reported })}
            </p>
            <div className="flex flex-wrap gap-2">
              {summary.versions.map((v) => (
                <Badge key={v.version} variant="outline">
                  {v.version} · {v.count}
                </Badge>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 3: 列表卡(firmware-list-card.tsx)**

Create `web-admin/src/features/ota/components/firmware-list-card.tsx` —— mirror `gateway-list-card.tsx` 的 Table:
```tsx
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useFirmwareList } from "../hooks/use-firmware"
import { formatBytes, truncateHex } from "../lib/firmware-utils"

export function FirmwareListCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t } = useTranslation()
  const [channel, setChannel] = useState("")
  const query = useFirmwareList(token, tenantID, channel)
  const items = query.data ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.firmware.list.title")}</CardTitle>
        <CardDescription>{t("ota.firmware.list.description")}</CardDescription>
        <Select value={channel || "all"} onValueChange={(v) => setChannel(v === "all" ? "" : v)}>
          <SelectTrigger className="w-40"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("ota.firmware.list.channelAll")}</SelectItem>
            <SelectItem value="stable">stable</SelectItem>
            <SelectItem value="beta">beta</SelectItem>
          </SelectContent>
        </Select>
      </CardHeader>
      <CardContent>
        {query.isPending ? (
          <p className="text-sm text-muted-foreground">{t("ota.firmware.list.loading")}</p>
        ) : query.error ? (
          <p className="text-sm text-destructive">{query.error instanceof Error ? query.error.message : t("ota.firmware.errors.generic")}</p>
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("ota.firmware.list.empty")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("ota.firmware.list.colVersion")}</TableHead>
                <TableHead>{t("ota.firmware.list.colChannel")}</TableHead>
                <TableHead>{t("ota.firmware.list.colSha")}</TableHead>
                <TableHead>{t("ota.firmware.list.colSize")}</TableHead>
                <TableHead>{t("ota.firmware.list.colUploadedBy")}</TableHead>
                <TableHead>{t("ota.firmware.list.colCreated")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((fw) => (
                <TableRow key={fw.id}>
                  <TableCell>{fw.version}</TableCell>
                  <TableCell>{fw.channel ?? "—"}</TableCell>
                  <TableCell><span title={fw.sha256} className="font-mono text-xs">{truncateHex(fw.sha256)}</span></TableCell>
                  <TableCell>{formatBytes(fw.size_bytes)}</TableCell>
                  <TableCell>{fw.uploaded_by ?? "—"}</TableCell>
                  <TableCell>{new Date(fw.created_at).toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 4: 类型检查 + lint**

Run: `cd web-admin && npx tsc --noEmit 2>&1 | grep -E "features/ota" | head && npm run lint 2>&1 | tail -5`
Expected: ota 目录无类型错;lint 无新错。(若 Badge/Table/Select 的子组件名不符,读 `src/components/ui/{badge,table,select}.tsx` 改正导出名。)

- [ ] **Step 5: 提交**

```bash
git add web-admin/src/features/ota/hooks/use-firmware.ts web-admin/src/features/ota/components/firmware-summary-card.tsx web-admin/src/features/ota/components/firmware-list-card.tsx
git commit -m "feat(web): firmware summary + list cards"
```

---

## Task 4: 上传卡(表单 + 客户端 sha256 + mutation)

**Files:**
- Create: `web-admin/src/features/ota/components/firmware-upload-card.tsx`

- [ ] **Step 1: 实现上传卡** —— mirror `gateway-serial-inventory-ingest-card.tsx` 的表单结构

Create `web-admin/src/features/ota/components/firmware-upload-card.tsx`:
```tsx
import { zodResolver } from "@hookform/resolvers/zod"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { useMemo, useState } from "react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { uploadFirmware } from "@/lib/api/ota"
import { queryKeys } from "@/lib/query-keys"
import { computeSha256Hex, isCryptoSubtleAvailable, isSignatureHex } from "../lib/firmware-utils"

type UploadFormValues = { version: string; channel?: string; signature: string; file: FileList }

export function FirmwareUploadCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [serverError, setServerError] = useState("")
  const cryptoOk = isCryptoSubtleAvailable()

  const schema = useMemo(
    () =>
      z.object({
        version: z.string().trim().min(1, t("ota.firmware.upload.validation.versionRequired")).max(64, t("ota.firmware.upload.validation.versionMax")),
        channel: z.string().trim().max(32).optional().or(z.literal("")),
        signature: z.string().trim().refine(isSignatureHex, t("ota.firmware.upload.validation.signatureHex")),
        file: z.custom<FileList>((v) => v instanceof FileList && v.length > 0, t("ota.firmware.upload.validation.fileRequired")),
      }),
    [t]
  )
  const form = useForm<UploadFormValues>({ resolver: zodResolver(schema), defaultValues: { version: "", channel: "", signature: "", file: undefined as unknown as FileList } })

  const mutation = useMutation({
    mutationFn: async (values: UploadFormValues) => {
      const file = values.file[0]
      const sha256 = await computeSha256Hex(file)
      return uploadFirmware(token, tenantID, { version: values.version.trim(), channel: values.channel?.trim() || undefined, sha256, signature: values.signature.trim(), file })
    },
    onSuccess: () => {
      setServerError("")
      form.reset()
      void queryClient.invalidateQueries({ queryKey: queryKeys.firmwareList._base })
      void queryClient.invalidateQueries({ queryKey: queryKeys.firmwareSummary._base })
    },
    onError: (err) => setServerError(err instanceof Error ? err.message : t("ota.firmware.errors.generic")),
  })

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.firmware.upload.title")}</CardTitle>
        <CardDescription>{t("ota.firmware.upload.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {!cryptoOk ? (
          <p className="text-sm text-destructive">{t("ota.firmware.upload.cryptoUnavailable")}</p>
        ) : (
          <form className="space-y-3" onSubmit={form.handleSubmit((v) => mutation.mutate(v))}>
            <div>
              <Input placeholder={t("ota.firmware.upload.versionPlaceholder")} {...form.register("version")} />
              {form.formState.errors.version && <p className="text-sm text-destructive">{form.formState.errors.version.message}</p>}
            </div>
            <Input placeholder={t("ota.firmware.upload.channelPlaceholder")} {...form.register("channel")} />
            <div>
              <Textarea rows={3} placeholder={t("ota.firmware.upload.signaturePlaceholder")} {...form.register("signature")} />
              {form.formState.errors.signature && <p className="text-sm text-destructive">{form.formState.errors.signature.message}</p>}
            </div>
            <div>
              <Controller
                control={form.control}
                name="file"
                render={({ field }) => (
                  <Input type="file" onChange={(e) => field.onChange(e.target.files ?? undefined)} />
                )}
              />
              {form.formState.errors.file && <p className="text-sm text-destructive">{String(form.formState.errors.file.message)}</p>}
            </div>
            {serverError && <p className="text-sm text-destructive">{serverError}</p>}
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("ota.firmware.upload.submitting") : t("ota.firmware.upload.submit")}
            </Button>
          </form>
        )}
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 2: 类型检查 + lint**

Run: `cd web-admin && npx tsc --noEmit 2>&1 | grep -E "firmware-upload" | head && npm run lint 2>&1 | tail -5`
Expected: 无类型错;lint 无新错。(`Input type="file"` 的受控写法若与 ui/input 冲突,按 mirror 卡片的 file 处理方式调整。)

- [ ] **Step 3: 提交**

```bash
git add web-admin/src/features/ota/components/firmware-upload-card.tsx
git commit -m "feat(web): firmware upload card (client sha256 + multipart)"
```

---

## Task 5: 页面 + 路由 + 导航 + 三语 i18n

**Files:**
- Create: `web-admin/src/features/ota/pages/firmware-page.tsx`
- Modify: `web-admin/src/features/mistyislet-shell/routes.tsx`(注册 `/ota`)
- Modify: 侧边导航文件(实现者 grep `"/gateways"` 在 nav/sidebar 找到导航定义)
- Modify: `web-admin/src/locales/{en-US,id-ID,zh-CN}.json`(加 `ota` 命名空间)

- [ ] **Step 1: firmware-page.tsx**

Create `web-admin/src/features/ota/pages/firmware-page.tsx`:
```tsx
import { useTranslation } from "react-i18next"
import type { CurrentUser } from "@/lib/api/core"
import { FirmwareSummaryCard } from "../components/firmware-summary-card"
import { FirmwareUploadCard } from "../components/firmware-upload-card"
import { FirmwareListCard } from "../components/firmware-list-card"

const UPLOAD_ROLES = ["super_admin", "tenant_admin", "building_admin"]

export function FirmwarePage({ token, viewer }: { token: string | undefined; viewer: CurrentUser }) {
  const { t } = useTranslation()
  // tenant-scoped 用户用 token 的租户(传 undefined → 服务端按 token 推断);super_admin 可后续加租户选择(#5b/后续)。
  const tenantID = viewer.role === "super_admin" ? undefined : viewer.tenant_id
  const canUpload = UPLOAD_ROLES.includes(viewer.role)
  return (
    <div className="space-y-4 p-4">
      <h1 className="text-xl font-semibold">{t("ota.firmware.pageTitle")}</h1>
      <FirmwareSummaryCard token={token} tenantID={tenantID} />
      {canUpload ? <FirmwareUploadCard token={token} tenantID={tenantID} /> : null}
      <FirmwareListCard token={token} tenantID={tenantID} />
    </div>
  )
}
```
(`CurrentUser` 类型实际位置以 grep 为准:`git grep "export type CurrentUser" web-admin/src` —— 在 `lib/api/core.ts`。`viewer` 传入的具体类型按 routes.tsx 里 gateways page 接的 `viewer` prop 对齐。)

- [ ] **Step 2: 注册路由(routes.tsx)**

mirror gateways 的 lazy + Route(读 `routes.tsx` 里 `GatewaysPage` 的注册):
```tsx
const FirmwarePage = lazy(() =>
  import("@/features/ota/pages/firmware-page").then((m) => ({ default: m.FirmwarePage }))
)
// 在 <Routes> 里(gateways Route 附近):
<Route path="/ota" element={<FirmwarePage token={token} viewer={viewer} />} />
```
(若该 console 路由组对非 resident 角色统一可见即可;读 routes.tsx 看 gateways Route 是否包了角色 guard,有则照搬 write/read 角色门控。)

- [ ] **Step 3: 侧边导航加入口**

`git grep -n '"/gateways"' web-admin/src` 找到 nav 定义(label + path + icon + 角色),照样加一条 `{ path: "/ota", labelKey: "ota.nav.firmware", roles: [...] }`(字段名/结构以实际 nav 定义为准;角色 = 读角色集)。

- [ ] **Step 4: 三语 i18n**

在 `web-admin/src/locales/en-US.json`、`id-ID.json`、`zh-CN.json` 各加顶层 `ota` 命名空间(三语翻译对应)。en-US 示例(id/zh 同结构翻译):
```json
"ota": {
  "nav": { "firmware": "Firmware" },
  "firmware": {
    "pageTitle": "Firmware Management",
    "summary": {
      "title": "Fleet Firmware Versions",
      "description": "Firmware version distribution across this tenant's gateways.",
      "loading": "Loading…",
      "empty": "No gateways have reported a firmware version yet.",
      "fleet": "{{reported}} of {{total}} gateways have reported a version."
    },
    "upload": {
      "title": "Upload Firmware",
      "description": "Upload a signed firmware binary. Sign offline with ota-sign; the private key never enters the browser.",
      "versionPlaceholder": "Version (e.g. 1.4.0)",
      "channelPlaceholder": "Channel (optional: stable / beta)",
      "signaturePlaceholder": "Ed25519 signature (hex, from ota-sign)",
      "submit": "Upload",
      "submitting": "Uploading…",
      "cryptoUnavailable": "Firmware upload requires a secure context (HTTPS).",
      "validation": {
        "versionRequired": "Version is required",
        "versionMax": "Version too long",
        "signatureHex": "Signature must be 128 hex characters (Ed25519)",
        "fileRequired": "Select a firmware file"
      }
    },
    "list": {
      "title": "Firmware Registry",
      "description": "Uploaded firmware versions for this tenant.",
      "loading": "Loading…",
      "empty": "No firmware uploaded yet.",
      "channelAll": "All channels",
      "colVersion": "Version", "colChannel": "Channel", "colSha": "SHA-256",
      "colSize": "Size", "colUploadedBy": "Uploaded by", "colCreated": "Created"
    },
    "errors": { "generic": "Something went wrong." }
  }
}
```
id-ID / zh-CN 用相应语言翻译(zh-CN 例:`"pageTitle": "固件管理"`、`"upload.title": "上传固件"` 等)。**三个文件的键结构必须一致。**

- [ ] **Step 5: 类型检查 + lint + 构建 + 全测**

Run: `cd web-admin && npx tsc --noEmit 2>&1 | tail -5 && npm run lint 2>&1 | tail -5 && npm run test:unit 2>&1 | tail -5 && npm run build 2>&1 | tail -5`
Expected: tsc 无错;lint 无新错;全测 PASS;build 成功。

- [ ] **Step 6: 提交**

```bash
git add web-admin/src/features/ota/pages/firmware-page.tsx web-admin/src/features/mistyislet-shell/routes.tsx web-admin/src/locales/en-US.json web-admin/src/locales/id-ID.json web-admin/src/locales/zh-CN.json
git commit -m "feat(web): firmware management page + route + nav + i18n"
```

---

## 自检(Self-Review)

**1. Spec 覆盖**
- §3 结构(features/ota/ 页+3卡+hooks+lib;lib/api/ota.ts;requestFormData;query-keys)→ Task 1/2/3/4/5 ✓
- §2 真实契约(summary/list/upload)→ Task 1 ota.ts ✓
- §4.2 总览卡 → Task 3 ✓;§4.3 上传卡(客户端 sha256 + 离线签名 + secure context 门)→ Task 4 ✓;§4.4 列表卡(channel 过滤)→ Task 3 ✓
- §4.1 页面 + 角色门控上传 → Task 5 ✓
- §6 错误/边界(查询失败、上传错误、crypto 不可用禁用、空态、非 write 不见上传)→ Task 3/4/5 ✓
- §7 三语 i18n → Task 5 ✓;§8 测试(ota.ts mock fetch、firmware-utils)→ Task 1/2 ✓(按 web-admin 惯例:逻辑/API 单测,非 .test.tsx 渲染测)

**2. 占位符扫描**:无 TODO/TBD。两处"实现者 grep 定位"(API barrel 桶文件、nav 定义、refreshAccessToken 名)是因 web-admin 具体文件位置需现场确认 —— 已给明确 grep 命令 + 期望,非占位。

**3. 类型一致性**:`FirmwareSummary`/`GatewayFirmware`/`UploadFirmwareInput` 字段全程一致;`getFirmwareSummary`/`listFirmware`/`uploadFirmware`/`requestFormData` 签名一致;`computeSha256Hex`/`isSignatureHex`/`formatBytes`/`truncateHex`/`isCryptoSubtleAvailable` 一致;`queryKeys.firmwareSummary`/`firmwareList` 一致;i18n key 在卡片与 locale 一致。

**4. 关键风险**:(a) crypto.subtle 需 secure context → 上传卡门控 + 提示;(b) 私钥绝不进浏览器,签名仅粘贴;(c) requestFormData 不设 Content-Type(测试断言);(d) tenant 走 query,tenant-scoped 用户传 undefined → 服务端按 token;(e) 三 locale 文件键结构必须同步。

---

## 执行交接(建议 Subagent-Driven)
前端按既有模式落地,建议 **superpowers:subagent-driven-development**;实现卡片时可叠加 frontend-design skill 但严守 web-admin 既有风格。
