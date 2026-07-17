# OTA Rollout 管理 UI(#5b)Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`). 实现卡片/页面时严守既有 web-admin 模式(mirror 参考见下)+ #5a 的 features/ota 风格。

**Goal:** web-admin `/ota/rollouts`(完整创建表单含调度 + 列表)+ `/ota/rollouts/:rolloutID`(每网关进度监控轮询 + 审批/暂停/恢复/中止),完成整个 Kisi OTA 对标。

**Architecture:** 扩展 `lib/api/ota.ts`(rollout 真实契约)。可测逻辑抽 `features/ota/lib/rollout-utils.ts`(phases 校验/状态→badge/可用操作/调度组装),vitest 单测。react-query hooks(list、detail 带 5s 轮询、action mutations)。组件复用 #5a 模式。

**Tech Stack:** React + TS、shadcn/ui、@tanstack/react-query v5、react-hook-form(`useFieldArray`)+ zod、react-router(`useParams`/`useNavigate`)、react-i18next、vitest。

设计依据:[2026-06-08-ota-rollout-ui-design.md](../specs/2026-06-08-ota-rollout-ui-design.md)

**工作目录:** worktree 根 `/Users/siky/code/MistyPass/.claude/worktrees/distracted-pascal-bf13c4`;前端在 `web-admin/`,**npm 从 `…/distracted-pascal-bf13c4/web-admin/` 跑**;`node_modules` 是已就绪软链(勿 npm install)。提交到分支 `claude/distracted-pascal-bf13c4`。

**mirror 参考:** API → `src/lib/api/ota.ts`(#5a)+ `core.ts`;表单卡 → #5a `firmware-upload-card.tsx` + `gateway-serial-inventory-ingest-card.tsx`;表格卡 → #5a `firmware-list-card.tsx`;详情路由页 → `src/features/users/pages/user-detail-page.tsx`(`/users/:userID`,接 `{token, viewer}` + `useParams`);布尔 → `@/components/ui/switch`(`Switch`);**无 Checkbox 组件** → 网关多选用原生 `<input type="checkbox" className="…">`。Badge variants:default/secondary/success/warning/destructive/outline。

**真实后端契约:** §2 of the spec. tenant 走 query(tenant-scoped 省略);写角色 super_admin/tenant_admin/building_admin,读 +operator;状态冲突 409。

---

## Task 1: API 扩展(rollout 函数 + 类型 + query-keys)

**Files:** Modify `web-admin/src/lib/api/ota.ts`、`web-admin/src/lib/query-keys.ts`;Test `web-admin/src/lib/api/ota.test.ts`(追加)

- [ ] **Step 1: 追加失败测试**(在 ota.test.ts 末尾,复用文件顶部已有的 `mockFetchOnce`/`afterEach`):
```ts
import { createRollout, getRolloutDetail, listRollouts, rolloutAction } from "./ota"

describe("rollout api", () => {
  it("createRollout POSTs the body with tenant_id query", async () => {
    const fetchFn = mockFetchOnce({ id: "rollout_1", state: "active" })
    await createRollout("tok", "tenant_demo_jakarta", {
      firmware_id: "fw_1", target: { kind: "all" }, phases: [{ percentage: 100, requires_approval: false }],
    })
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit]
    expect(String(url)).toContain("/api/v1/gateways/rollouts?")
    expect(String(url)).toContain("tenant_id=tenant_demo_jakarta")
    expect(init.method).toBe("POST")
    expect(JSON.parse(String(init.body)).firmware_id).toBe("fw_1")
  })
  it("listRollouts unwraps items", async () => {
    mockFetchOnce({ items: [{ id: "rollout_1" }] })
    expect(await listRollouts("tok", "t")).toHaveLength(1)
  })
  it("getRolloutDetail returns {rollout, gateways}", async () => {
    mockFetchOnce({ rollout: { id: "rollout_1", state: "active" }, gateways: [{ gateway_id: "gw1", phase: 0, ota_status: "queued" }] })
    const res = await getRolloutDetail("tok", "t", "rollout_1")
    expect(res.rollout.id).toBe("rollout_1")
    expect(res.gateways).toHaveLength(1)
  })
  it("rolloutAction POSTs to the action path", async () => {
    const fetchFn = mockFetchOnce({ id: "rollout_1", state: "paused" })
    await rolloutAction("tok", "t", "rollout_1", "pause")
    const [url, init] = fetchFn.mock.calls[0] as [string, RequestInit]
    expect(String(url)).toContain("/api/v1/gateways/rollouts/rollout_1/pause")
    expect(init.method).toBe("POST")
  })
})
```

- [ ] **Step 2: 确认失败** — `cd web-admin && npm run test:unit -- src/lib/api/ota.test.ts` → FAIL(createRollout 等 undefined)。

- [ ] **Step 3: ota.ts 追加类型 + 函数**(`firmwareQuery` 已在文件,复用——它在只传 tenantID 时生成 `?tenant_id=`):
```ts
export type RolloutTarget = { kind: "all" | "building" | "gateways"; building_id?: string; gateway_ids?: string[] }
export type RolloutPhase = { percentage: number; requires_approval: boolean }
export type RolloutSchedule = { start_at?: string; window_start?: string; window_end?: string; timezone?: string }
export type GatewayRollout = {
  id: string; tenant_id: string; firmware_id: string; firmware_version: string
  target: RolloutTarget; phases: RolloutPhase[]; failure_threshold_pct: number
  state: string; current_phase: number; schedule?: RolloutSchedule
  created_by?: string; updated_by?: string; created_at: string; updated_at: string
}
export type RolloutGatewayStatus = { gateway_id: string; phase: number; ota_status: string; current_firmware_version?: string }
export type CreateRolloutInput = { firmware_id: string; target: RolloutTarget; phases: RolloutPhase[]; failure_threshold_pct?: number; schedule?: RolloutSchedule }
export type RolloutActionName = "approve" | "pause" | "resume" | "abort"

export async function createRollout(token: string | undefined, tenantID: string | undefined, input: CreateRolloutInput): Promise<GatewayRollout> {
  return request<GatewayRollout>(`/api/v1/gateways/rollouts${firmwareQuery(tenantID)}`, { method: "POST", body: JSON.stringify(input) }, token)
}
export async function listRollouts(token: string | undefined, tenantID?: string): Promise<GatewayRollout[]> {
  return requestItems<GatewayRollout>(`/api/v1/gateways/rollouts${firmwareQuery(tenantID)}`, token)
}
export async function getRolloutDetail(token: string | undefined, tenantID: string | undefined, id: string): Promise<{ rollout: GatewayRollout; gateways: RolloutGatewayStatus[] }> {
  return request<{ rollout: GatewayRollout; gateways: RolloutGatewayStatus[] }>(`/api/v1/gateways/rollouts/${encodeURIComponent(id)}${firmwareQuery(tenantID)}`, { method: "GET" }, token)
}
export async function rolloutAction(token: string | undefined, tenantID: string | undefined, id: string, action: RolloutActionName): Promise<GatewayRollout> {
  return request<GatewayRollout>(`/api/v1/gateways/rollouts/${encodeURIComponent(id)}/${action}${firmwareQuery(tenantID)}`, { method: "POST" }, token)
}
```
(`firmwareQuery` 仅在传 channel 时加 channel;只传 tenantID 时输出 `?tenant_id=…`。barrel 已 `export * from "./ota"`,新符号自动导出。)

- [ ] **Step 4: query-keys.ts** — 加:
```ts
  rolloutList: ns("ota-rollout-list"),
  rolloutDetail: ns("ota-rollout-detail"),
```

- [ ] **Step 5: 测试 + 提交** — `cd web-admin && npm run test:unit -- src/lib/api/ota.test.ts`(全 PASS)。
```bash
git add web-admin/src/lib/api/ota.ts web-admin/src/lib/api/ota.test.ts web-admin/src/lib/query-keys.ts
git commit -m "feat(web): rollout API client functions"
```

---

## Task 2: rollout-utils + 测试

**Files:** Create `web-admin/src/features/ota/lib/rollout-utils.ts` + `.test.ts`

- [ ] **Step 1: 失败测试** — Create `web-admin/src/features/ota/lib/rollout-utils.test.ts`:
```ts
import { describe, expect, it } from "vitest"
import { availableRolloutActions, buildSchedulePayload, buildingOptions, isHHMM, rolloutStateBadgeVariant, targetSummary, validatePhases } from "./rollout-utils"

describe("rollout-utils", () => {
  it("validatePhases", () => {
    expect(validatePhases([{ percentage: 100, requires_approval: false }])).toBe(true)
    expect(validatePhases([{ percentage: 10, requires_approval: false }, { percentage: 50, requires_approval: false }, { percentage: 100, requires_approval: true }])).toBe(true)
    expect(validatePhases([])).toBe(false)
    expect(validatePhases([{ percentage: 50, requires_approval: false }])).toBe(false) // last != 100
    expect(validatePhases([{ percentage: 50, requires_approval: false }, { percentage: 50, requires_approval: false }, { percentage: 100, requires_approval: false }])).toBe(false) // not increasing
    expect(validatePhases([{ percentage: 0, requires_approval: false }, { percentage: 100, requires_approval: false }])).toBe(false)
  })
  it("availableRolloutActions mirrors backend guards", () => {
    expect(availableRolloutActions("awaiting_approval")).toEqual(["approve", "abort"])
    expect(availableRolloutActions("active")).toEqual(["pause", "abort"])
    expect(availableRolloutActions("paused")).toEqual(["resume", "abort"])
    expect(availableRolloutActions("scheduled")).toEqual(["abort"])
    expect(availableRolloutActions("completed")).toEqual([])
    expect(availableRolloutActions("failed")).toEqual([])
  })
  it("rolloutStateBadgeVariant", () => {
    expect(rolloutStateBadgeVariant("completed")).toBe("success")
    expect(rolloutStateBadgeVariant("failed")).toBe("destructive")
    expect(rolloutStateBadgeVariant("active")).toBe("default")
    expect(rolloutStateBadgeVariant("scheduled")).toBe("outline")
  })
  it("buildingOptions dedups + sorts", () => {
    expect(buildingOptions([{ building_id: "b2" }, { building_id: "b1" }, { building_id: "b1" }, { building_id: "" }])).toEqual(["b1", "b2"])
  })
  it("isHHMM", () => {
    expect(isHHMM("02:00")).toBe(true)
    expect(isHHMM("23:59")).toBe(true)
    expect(isHHMM("24:00")).toBe(false)
    expect(isHHMM("2:00")).toBe(false)
  })
  it("buildSchedulePayload", () => {
    expect(buildSchedulePayload({})).toBeUndefined()
    const p = buildSchedulePayload({ windowStart: "02:00", windowEnd: "05:00", timezone: "Asia/Jakarta" })
    expect(p).toEqual({ window_start: "02:00", window_end: "05:00", timezone: "Asia/Jakarta" })
    const p2 = buildSchedulePayload({ startAtLocal: "2026-06-08T02:00" })
    expect(typeof p2?.start_at).toBe("string")
    expect(p2?.start_at).toContain("T")
  })
  it("targetSummary", () => {
    expect(targetSummary({ kind: "all" })).toContain("All")
    expect(targetSummary({ kind: "gateways", gateway_ids: ["a", "b"] })).toContain("2")
  })
})
```

- [ ] **Step 2: 确认失败** — `cd web-admin && npm run test:unit -- src/features/ota/lib/rollout-utils.test.ts` → FAIL。

- [ ] **Step 3: rollout-utils.ts** — Create:
```ts
import type { RolloutPhase, RolloutSchedule, RolloutTarget } from "@/lib/api/ota"

export function validatePhases(phases: RolloutPhase[]): boolean {
  if (phases.length === 0) return false
  let prev = 0
  for (const p of phases) {
    if (!Number.isInteger(p.percentage) || p.percentage < 1 || p.percentage > 100 || p.percentage <= prev) return false
    prev = p.percentage
  }
  return prev === 100
}

export type RolloutActionName = "approve" | "pause" | "resume" | "abort"
export function availableRolloutActions(state: string): RolloutActionName[] {
  switch (state) {
    case "awaiting_approval": return ["approve", "abort"]
    case "active": return ["pause", "abort"]
    case "paused": return ["resume", "abort"]
    case "scheduled":
    case "pending": return ["abort"]
    default: return []
  }
}

export function rolloutStateBadgeVariant(state: string): string {
  switch (state) {
    case "active": return "default"
    case "completed": return "success"
    case "failed": return "destructive"
    case "paused":
    case "awaiting_approval": return "warning"
    case "scheduled": return "outline"
    default: return "secondary"
  }
}

export function buildingOptions(gateways: Array<{ building_id?: string }>): string[] {
  const set = new Set<string>()
  for (const g of gateways) {
    if (g.building_id && g.building_id.trim() !== "") set.add(g.building_id.trim())
  }
  return Array.from(set).sort()
}

export function isHHMM(s: string): boolean {
  return /^([01][0-9]|2[0-3]):[0-5][0-9]$/.test(s.trim())
}

export function buildSchedulePayload(input: { startAtLocal?: string; windowStart?: string; windowEnd?: string; timezone?: string }): RolloutSchedule | undefined {
  const sch: RolloutSchedule = {}
  if (input.startAtLocal && input.startAtLocal.trim() !== "") sch.start_at = new Date(input.startAtLocal).toISOString()
  if (input.windowStart && input.windowStart.trim() !== "") sch.window_start = input.windowStart.trim()
  if (input.windowEnd && input.windowEnd.trim() !== "") sch.window_end = input.windowEnd.trim()
  if (input.timezone && input.timezone.trim() !== "") sch.timezone = input.timezone.trim()
  return Object.keys(sch).length > 0 ? sch : undefined
}

export function targetSummary(target: RolloutTarget): string {
  if (target.kind === "all") return "All gateways"
  if (target.kind === "building") return `Building ${target.building_id ?? "—"}`
  return `${target.gateway_ids?.length ?? 0} gateways`
}
```

- [ ] **Step 4: 测试通过 + 提交** — `cd web-admin && npm run test:unit -- src/features/ota/lib/rollout-utils.test.ts`(全 PASS)。
```bash
git add web-admin/src/features/ota/lib/rollout-utils.ts web-admin/src/features/ota/lib/rollout-utils.test.ts
git commit -m "feat(web): rollout-utils (phases/state/actions/schedule)"
```

---

## Task 3: hooks(list / detail 轮询 / actions)

**Files:** Create `web-admin/src/features/ota/hooks/use-rollouts.ts`

- [ ] **Step 1: 实现 hooks** — Create:
```ts
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createRollout, getRolloutDetail, listRollouts, rolloutAction, type CreateRolloutInput, type RolloutActionName } from "@/lib/api/ota"
import { queryKeys } from "@/lib/query-keys"

export function useRolloutList(token: string | undefined, tenantID: string | undefined) {
  return useQuery({
    queryKey: queryKeys.rolloutList._key(tenantID ?? "self"),
    queryFn: () => listRollouts(token, tenantID),
    staleTime: 15 * 1000,
  })
}

const POLLING_STATES = ["active", "scheduled", "awaiting_approval", "paused"]
export function useRolloutDetail(token: string | undefined, tenantID: string | undefined, id: string) {
  return useQuery({
    queryKey: queryKeys.rolloutDetail._key(tenantID ?? "self", id),
    queryFn: () => getRolloutDetail(token, tenantID, id),
    refetchInterval: (query) => {
      const state = query.state.data?.rollout.state
      return state && POLLING_STATES.includes(state) ? 5000 : false
    },
  })
}

export function useCreateRolloutMutation(token: string | undefined, tenantID: string | undefined) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateRolloutInput) => createRollout(token, tenantID, input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: queryKeys.rolloutList._base }),
  })
}

export function useRolloutActionMutation(token: string | undefined, tenantID: string | undefined, id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (action: RolloutActionName) => rolloutAction(token, tenantID, id, action),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: queryKeys.rolloutDetail._base })
      void qc.invalidateQueries({ queryKey: queryKeys.rolloutList._base })
    },
  })
}
```

- [ ] **Step 2: 类型检查 + 提交** — `cd web-admin && npx tsc --noEmit 2>&1 | grep use-rollouts | head`(无错;若 `refetchInterval` 的 query 参数类型在 v5 不符,改用 `(query) => ...` 的实际签名——读 react-query 版本调整,本质是 state.data?.rollout.state)。
```bash
git add web-admin/src/features/ota/hooks/use-rollouts.ts
git commit -m "feat(web): rollout hooks (list, polling detail, actions)"
```

---

## Task 4: 列表卡 + Rollouts 页 + 路由 + 导航

**Files:** Create `rollout-list-card.tsx`、`pages/rollouts-page.tsx`;Modify `routes.tsx`、`navigation.ts`

- [ ] **Step 1: 列表卡** — Create `web-admin/src/features/ota/components/rollout-list-card.tsx`(mirror firmware-list-card):
```tsx
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { useRolloutList } from "../hooks/use-rollouts"
import { rolloutStateBadgeVariant, targetSummary } from "../lib/rollout-utils"

export function RolloutListCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t, i18n } = useTranslation()
  const navigate = useNavigate()
  const query = useRolloutList(token, tenantID)
  const items = query.data ?? []
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.rollout.list.title")}</CardTitle>
        <CardDescription>{t("ota.rollout.list.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {query.isPending ? (
          <p className="text-sm text-muted-foreground">{t("ota.rollout.list.loading")}</p>
        ) : query.error ? (
          <p className="text-sm text-destructive">{query.error instanceof Error ? query.error.message : t("ota.rollout.errors.generic")}</p>
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t("ota.rollout.list.empty")}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("ota.rollout.list.colFirmware")}</TableHead>
                <TableHead>{t("ota.rollout.list.colTarget")}</TableHead>
                <TableHead>{t("ota.rollout.list.colState")}</TableHead>
                <TableHead>{t("ota.rollout.list.colPhase")}</TableHead>
                <TableHead>{t("ota.rollout.list.colCreated")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((r) => (
                <TableRow key={r.id} className="cursor-pointer" onClick={() => navigate(`/ota/rollouts/${r.id}`)}>
                  <TableCell>{r.firmware_version}</TableCell>
                  <TableCell>{targetSummary(r.target)}</TableCell>
                  <TableCell><Badge variant={rolloutStateBadgeVariant(r.state) as never}>{t(`ota.rollout.state.${r.state}`)}</Badge></TableCell>
                  <TableCell>{r.current_phase + 1}/{r.phases.length}</TableCell>
                  <TableCell>{new Date(r.created_at).toLocaleString(i18n.language)}</TableCell>
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
(`Badge variant` 接受的 union 若不含 success/warning,读 `ui/badge.tsx` 确认 variant 名;`as never` 仅为绕过窄类型——若 badge variant 是字符串 union,直接传字符串即可,去掉 `as never`。)

- [ ] **Step 2: Rollouts 页** — Create `web-admin/src/features/ota/pages/rollouts-page.tsx`(mirror firmware-page;创建卡在 Task 5 加,先放列表):
```tsx
import { useTranslation } from "react-i18next"
import type { CurrentUser } from "@/lib/api"
import { RolloutListCard } from "../components/rollout-list-card"
import { RolloutCreateCard } from "../components/rollout-create-card"

const WRITE_ROLES: CurrentUser["role"][] = ["super_admin", "tenant_admin", "building_admin"]

export function RolloutsPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const { t } = useTranslation()
  const tenantID = viewer.role === "super_admin" ? undefined : viewer.tenant_id
  const canWrite = WRITE_ROLES.includes(viewer.role)
  return (
    <div className="space-y-6">
      <div className="mp-page-hero">
        <div className="relative z-10 flex flex-col gap-5">
          <div className="max-w-3xl space-y-2"><h1 className="mp-page-title">{t("ota.rollout.pageTitle")}</h1></div>
        </div>
      </div>
      <div className="space-y-4">
        {canWrite ? <RolloutCreateCard token={token} tenantID={tenantID} /> : null}
        <RolloutListCard token={token} tenantID={tenantID} />
      </div>
    </div>
  )
}
```
(本任务先不 import RolloutCreateCard —— 注释掉该行 + 那个 import,Task 5 再加。或先建一个最小占位 RolloutCreateCard 返回 null,Task 5 实装。为避免编译错误,**本任务先注释 `{canWrite ? ... : null}` 那行与其 import**,Task 5 取消注释并实装。)

- [ ] **Step 3: 路由 + 导航** — `routes.tsx` 加 lazy + Route(mirror firmware 页):
```tsx
const RolloutsPage = lazy(() => import("@/features/ota/pages/rollouts-page").then((m) => ({ default: m.RolloutsPage })))
// near /ota:
<Route path="/ota/rollouts" element={<RolloutsPage token={token} viewer={viewer} />} />
```
`navigation.ts` 在 firmware 入口旁加 `{ label: t("ota.rollout.nav.rollouts"), icon: <复用一个 lucide icon 如 RocketIcon/SendIcon>, to: "/ota/rollouts" }`;`/ota/rollouts` 加进 `mistyisletPreviewRoutePrefixes`(若存在该数组)。

- [ ] **Step 4: 类型检查 + 提交** — `cd web-admin && npx tsc --noEmit 2>&1 | grep -E "features/ota" | head`(无错)。
```bash
git add web-admin/src/features/ota/components/rollout-list-card.tsx web-admin/src/features/ota/pages/rollouts-page.tsx web-admin/src/features/mistyislet-shell/routes.tsx web-admin/src/features/mistyislet-shell/navigation.ts
git commit -m "feat(web): rollout list card + page + route + nav"
```

---

## Task 5: 创建卡(完整表单:固件 + target + phases + 阈值 + 调度)

**Files:** Create `web-admin/src/features/ota/components/rollout-create-card.tsx`;Modify `rollouts-page.tsx`(取消注释接入)

- [ ] **Step 1: 创建卡** — Create `rollout-create-card.tsx`。结构(mirror firmware-upload-card + 用 `useFieldArray`、`Switch`、原生 checkbox):
```tsx
import { zodResolver } from "@hookform/resolvers/zod"
import { useMemo, useState } from "react"
import { Controller, useFieldArray, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { z } from "zod"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { listFirmware, type CreateRolloutInput, type RolloutTarget } from "@/lib/api/ota"
import { listGateways } from "@/lib/api/gateways"
import { useQuery } from "@tanstack/react-query"
import { queryKeys } from "@/lib/query-keys"
import { useCreateRolloutMutation } from "../hooks/use-rollouts"
import { buildSchedulePayload, buildingOptions, isHHMM, validatePhases } from "../lib/rollout-utils"

type FormValues = {
  firmware_id: string
  targetKind: "all" | "building" | "gateways"
  building_id?: string
  gateway_ids: string[]
  phases: { percentage: number; requires_approval: boolean }[]
  failure_threshold_pct: number
  startAtLocal?: string; windowStart?: string; windowEnd?: string; timezone?: string
}

export function RolloutCreateCard({ token, tenantID }: { token: string | undefined; tenantID: string | undefined }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [serverError, setServerError] = useState("")
  const firmwareQ = useQuery({ queryKey: queryKeys.firmwareList._key(tenantID ?? "self", "all"), queryFn: () => listFirmware(token, tenantID), staleTime: 30000 })
  const gatewaysQ = useQuery({ queryKey: ["ota-rollout-gateways", tenantID ?? "self"], queryFn: () => listGateways(token), staleTime: 30000 })
  const gateways = gatewaysQ.data ?? []
  const buildings = useMemo(() => buildingOptions(gateways), [gateways])
  const create = useCreateRolloutMutation(token, tenantID)

  const schema = useMemo(() => z.object({
    firmware_id: z.string().min(1, t("ota.rollout.create.validation.firmwareRequired")),
    targetKind: z.enum(["all", "building", "gateways"]),
    building_id: z.string().optional(),
    gateway_ids: z.array(z.string()),
    phases: z.array(z.object({ percentage: z.coerce.number(), requires_approval: z.boolean() })).refine(validatePhases, t("ota.rollout.create.validation.phasesInvalid")),
    failure_threshold_pct: z.coerce.number().min(0).max(100),
    startAtLocal: z.string().optional(),
    windowStart: z.string().optional().refine((v) => !v || isHHMM(v), t("ota.rollout.create.validation.hhmm")),
    windowEnd: z.string().optional().refine((v) => !v || isHHMM(v), t("ota.rollout.create.validation.hhmm")),
    timezone: z.string().optional(),
  }).refine((d) => d.targetKind !== "building" || !!d.building_id, { message: t("ota.rollout.create.validation.buildingRequired"), path: ["building_id"] })
    .refine((d) => d.targetKind !== "gateways" || d.gateway_ids.length > 0, { message: t("ota.rollout.create.validation.gatewaysRequired"), path: ["gateway_ids"] }), [t])

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { firmware_id: "", targetKind: "all", building_id: "", gateway_ids: [], phases: [{ percentage: 100, requires_approval: false }], failure_threshold_pct: 20, startAtLocal: "", windowStart: "", windowEnd: "", timezone: "" },
  })
  const phases = useFieldArray({ control: form.control, name: "phases" })
  const kind = form.watch("targetKind")
  const tzOptions = useMemo<string[]>(() => { try { return (Intl as unknown as { supportedValuesOf?: (k: string) => string[] }).supportedValuesOf?.("timeZone") ?? [] } catch { return [] } }, [])

  function onSubmit(v: FormValues) {
    const target: RolloutTarget = v.targetKind === "all" ? { kind: "all" }
      : v.targetKind === "building" ? { kind: "building", building_id: v.building_id }
      : { kind: "gateways", gateway_ids: v.gateway_ids }
    const input: CreateRolloutInput = {
      firmware_id: v.firmware_id, target, phases: v.phases,
      failure_threshold_pct: v.failure_threshold_pct,
      schedule: buildSchedulePayload({ startAtLocal: v.startAtLocal, windowStart: v.windowStart, windowEnd: v.windowEnd, timezone: v.timezone }),
    }
    create.mutate(input, {
      onSuccess: (r) => navigate(`/ota/rollouts/${r.id}`),
      onError: (e) => setServerError(e instanceof Error ? e.message : t("ota.rollout.errors.generic")),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("ota.rollout.create.title")}</CardTitle>
        <CardDescription>{t("ota.rollout.create.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)}>
          {/* firmware select */}
          <Controller control={form.control} name="firmware_id" render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger><SelectValue placeholder={t("ota.rollout.create.firmwarePlaceholder")} /></SelectTrigger>
              <SelectContent>
                {(firmwareQ.data ?? []).map((fw) => (<SelectItem key={fw.id} value={fw.id}>{fw.version}{fw.channel ? ` · ${fw.channel}` : ""}</SelectItem>))}
              </SelectContent>
            </Select>
          )} />
          {form.formState.errors.firmware_id && <p className="text-sm text-destructive">{form.formState.errors.firmware_id.message}</p>}

          {/* target kind */}
          <Controller control={form.control} name="targetKind" render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("ota.rollout.create.targetAll")}</SelectItem>
                <SelectItem value="building">{t("ota.rollout.create.targetBuilding")}</SelectItem>
                <SelectItem value="gateways">{t("ota.rollout.create.targetGateways")}</SelectItem>
              </SelectContent>
            </Select>
          )} />
          {kind === "building" && (
            <Controller control={form.control} name="building_id" render={({ field }) => (
              <Select value={field.value ?? ""} onValueChange={field.onChange}>
                <SelectTrigger><SelectValue placeholder={t("ota.rollout.create.buildingPlaceholder")} /></SelectTrigger>
                <SelectContent>{buildings.map((b) => <SelectItem key={b} value={b}>{b}</SelectItem>)}</SelectContent>
              </Select>
            )} />
          )}
          {kind === "gateways" && (
            <Controller control={form.control} name="gateway_ids" render={({ field }) => (
              <div className="max-h-48 overflow-auto rounded border p-2 space-y-1">
                {gateways.map((gw) => (
                  <label key={gw.id} className="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={field.value.includes(gw.id)}
                      onChange={(e) => field.onChange(e.target.checked ? [...field.value, gw.id] : field.value.filter((x: string) => x !== gw.id))} />
                    <span>{gw.id}{gw.current_firmware_version ? ` (${gw.current_firmware_version})` : ""}</span>
                  </label>
                ))}
              </div>
            )} />
          )}
          {(form.formState.errors.building_id || form.formState.errors.gateway_ids) && <p className="text-sm text-destructive">{String(form.formState.errors.building_id?.message ?? form.formState.errors.gateway_ids?.message)}</p>}

          {/* phases */}
          <div className="space-y-2">
            <p className="text-sm font-medium">{t("ota.rollout.create.phases")}</p>
            {phases.fields.map((f, i) => (
              <div key={f.id} className="flex items-center gap-2">
                <Input type="number" className="w-24" {...form.register(`phases.${i}.percentage`, { valueAsNumber: true })} />
                <Controller control={form.control} name={`phases.${i}.requires_approval`} render={({ field }) => (
                  <label className="flex items-center gap-2 text-sm"><Switch checked={field.value} onCheckedChange={field.onChange} />{t("ota.rollout.create.requiresApproval")}</label>
                )} />
                {phases.fields.length > 1 && <Button type="button" variant="outline" size="sm" onClick={() => phases.remove(i)}>{t("ota.rollout.create.removePhase")}</Button>}
              </div>
            ))}
            <Button type="button" variant="outline" size="sm" onClick={() => phases.append({ percentage: 100, requires_approval: false })}>{t("ota.rollout.create.addPhase")}</Button>
            {form.formState.errors.phases && <p className="text-sm text-destructive">{String(form.formState.errors.phases.message ?? t("ota.rollout.create.validation.phasesInvalid"))}</p>}
          </div>

          {/* failure threshold */}
          <div>
            <label className="text-sm">{t("ota.rollout.create.failureThreshold")}</label>
            <Input type="number" className="w-24" {...form.register("failure_threshold_pct", { valueAsNumber: true })} />
          </div>

          {/* schedule (optional) */}
          <details className="rounded border p-2">
            <summary className="text-sm cursor-pointer">{t("ota.rollout.create.scheduleSection")}</summary>
            <div className="mt-2 space-y-2">
              <Input type="datetime-local" {...form.register("startAtLocal")} />
              <div className="flex gap-2">
                <Input placeholder="HH:MM" {...form.register("windowStart")} />
                <Input placeholder="HH:MM" {...form.register("windowEnd")} />
              </div>
              <Controller control={form.control} name="timezone" render={({ field }) => (
                <Select value={field.value ?? ""} onValueChange={field.onChange}>
                  <SelectTrigger><SelectValue placeholder={t("ota.rollout.create.timezonePlaceholder")} /></SelectTrigger>
                  <SelectContent>{tzOptions.slice(0, 400).map((z) => <SelectItem key={z} value={z}>{z}</SelectItem>)}</SelectContent>
                </Select>
              )} />
              {(form.formState.errors.windowStart || form.formState.errors.windowEnd) && <p className="text-sm text-destructive">{t("ota.rollout.create.validation.hhmm")}</p>}
            </div>
          </details>

          {serverError && <p className="text-sm text-destructive">{serverError}</p>}
          <Button type="submit" disabled={create.isPending}>{create.isPending ? t("ota.rollout.create.submitting") : t("ota.rollout.create.submit")}</Button>
        </form>
      </CardContent>
    </Card>
  )
}
```
(若 `Switch` 的 prop 不是 `onCheckedChange`/`checked`,读 `ui/switch.tsx` 改正。若 `listGateways` 不在 `@/lib/api/gateways`,grep 实际导出位置。`Select` 的 value 不能为空字符串触发 placeholder——shadcn Select 用受控 value;空串 ok。)

- [ ] **Step 2: 接入 rollouts-page** — 取消 Task 4 注释的 `RolloutCreateCard` import + `{canWrite ? <RolloutCreateCard .../> : null}` 行。

- [ ] **Step 3: 类型检查 + 提交** — `cd web-admin && npx tsc --noEmit 2>&1 | grep -E "features/ota" | head`(无错)+ `npm run lint 2>&1 | grep -E "rollout-create" | head`(无新错)。
```bash
git add web-admin/src/features/ota/components/rollout-create-card.tsx web-admin/src/features/ota/pages/rollouts-page.tsx
git commit -m "feat(web): rollout create card (firmware/target/phases/schedule)"
```

---

## Task 6: 详情页(监控轮询 + 每网关进度 + 操作)

**Files:** Create `components/rollout-detail.tsx`、`pages/rollout-detail-page.tsx`;Modify `routes.tsx`

- [ ] **Step 1: 详情主体 rollout-detail.tsx** — Create:
```tsx
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import type { CurrentUser } from "@/lib/api"
import type { RolloutActionName } from "@/lib/api/ota"
import { useRolloutActionMutation, useRolloutDetail } from "../hooks/use-rollouts"
import { availableRolloutActions, rolloutStateBadgeVariant, targetSummary } from "../lib/rollout-utils"

const WRITE_ROLES: CurrentUser["role"][] = ["super_admin", "tenant_admin", "building_admin"]

export function RolloutDetail({ token, tenantID, viewer, id }: { token: string | undefined; tenantID: string | undefined; viewer: CurrentUser; id: string }) {
  const { t, i18n } = useTranslation()
  const query = useRolloutDetail(token, tenantID, id)
  const action = useRolloutActionMutation(token, tenantID, id)
  const [confirmAbort, setConfirmAbort] = useState(false)
  const canWrite = WRITE_ROLES.includes(viewer.role)

  if (query.isPending) return <p className="text-sm text-muted-foreground">{t("ota.rollout.detail.loading")}</p>
  if (query.error || !query.data) return <p className="text-sm text-destructive">{query.error instanceof Error ? query.error.message : t("ota.rollout.errors.generic")}</p>
  const { rollout, gateways } = query.data
  const actions = canWrite ? availableRolloutActions(rollout.state) : []

  const run = (a: RolloutActionName) => action.mutate(a)

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader><CardTitle className="flex items-center gap-3">{t("ota.rollout.detail.title")} <Badge variant={rolloutStateBadgeVariant(rollout.state) as never}>{t(`ota.rollout.state.${rollout.state}`)}</Badge></CardTitle></CardHeader>
        <CardContent className="space-y-2 text-sm">
          <p>{t("ota.rollout.detail.firmware")}: {rollout.firmware_version}</p>
          <p>{t("ota.rollout.detail.target")}: {targetSummary(rollout.target)}</p>
          <p>{t("ota.rollout.detail.phase")}: {rollout.current_phase + 1}/{rollout.phases.length} ({rollout.phases.map((p) => `${p.percentage}%${p.requires_approval ? "*" : ""}`).join(" → ")})</p>
          <p>{t("ota.rollout.detail.threshold")}: {rollout.failure_threshold_pct}%</p>
          {rollout.schedule && <p>{t("ota.rollout.detail.schedule")}: {JSON.stringify(rollout.schedule)}</p>}
          <div className="flex gap-2 pt-2">
            {actions.map((a) => a === "abort" ? (
              <Button key={a} variant="destructive" size="sm" disabled={action.isPending} onClick={() => setConfirmAbort(true)}>{t("ota.rollout.detail.actions.abort")}</Button>
            ) : (
              <Button key={a} variant="outline" size="sm" disabled={action.isPending} onClick={() => run(a)}>{t(`ota.rollout.detail.actions.${a}`)}</Button>
            ))}
          </div>
          {action.error && <p className="text-sm text-destructive">{action.error instanceof Error ? action.error.message : t("ota.rollout.errors.generic")}</p>}
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>{t("ota.rollout.detail.gatewaysTitle")}</CardTitle></CardHeader>
        <CardContent>
          <Table>
            <TableHeader><TableRow>
              <TableHead>{t("ota.rollout.detail.colGateway")}</TableHead>
              <TableHead>{t("ota.rollout.detail.colPhase")}</TableHead>
              <TableHead>{t("ota.rollout.detail.colStatus")}</TableHead>
              <TableHead>{t("ota.rollout.detail.colVersion")}</TableHead>
            </TableRow></TableHeader>
            <TableBody>
              {gateways.map((g) => (
                <TableRow key={g.gateway_id}>
                  <TableCell>{g.gateway_id}</TableCell>
                  <TableCell>{g.phase < 0 ? "—" : g.phase + 1}</TableCell>
                  <TableCell><Badge variant="outline">{g.ota_status}</Badge></TableCell>
                  <TableCell>{g.current_firmware_version ?? "—"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Dialog open={confirmAbort} onOpenChange={setConfirmAbort}>
        <DialogContent>
          <DialogHeader><DialogTitle>{t("ota.rollout.detail.confirmAbortTitle")}</DialogTitle><DialogDescription>{t("ota.rollout.detail.confirmAbortBody")}</DialogDescription></DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmAbort(false)}>{t("ota.rollout.detail.cancel")}</Button>
            <Button variant="destructive" onClick={() => { setConfirmAbort(false); run("abort") }}>{t("ota.rollout.detail.actions.abort")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <p className="text-xs text-muted-foreground">{new Date(rollout.updated_at).toLocaleString(i18n.language)}</p>
    </div>
  )
}
```
(`Dialog` 子组件名以 `ui/dialog.tsx` 实际为准;`Badge variant ... as never` 同列表卡处理。)

- [ ] **Step 2: 详情路由页 rollout-detail-page.tsx** — Create(mirror user-detail-page 取 param):
```tsx
import { useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import type { CurrentUser } from "@/lib/api"
import { RolloutDetail } from "../components/rollout-detail"

export function RolloutDetailPage({ token, viewer }: { token: string; viewer: CurrentUser }) {
  const { t } = useTranslation()
  const { rolloutID } = useParams<{ rolloutID: string }>()
  const tenantID = viewer.role === "super_admin" ? undefined : viewer.tenant_id
  if (!rolloutID) return <p className="text-sm text-destructive">{t("ota.rollout.errors.generic")}</p>
  return (
    <div className="space-y-6">
      <div className="mp-page-hero"><div className="relative z-10 flex flex-col gap-5"><div className="max-w-3xl space-y-2"><h1 className="mp-page-title">{t("ota.rollout.detail.pageTitle")}</h1></div></div></div>
      <RolloutDetail token={token} tenantID={tenantID} viewer={viewer} id={rolloutID} />
    </div>
  )
}
```

- [ ] **Step 3: 路由** — `routes.tsx` 加(在 `/ota/rollouts` 之后,param 路由放后面):
```tsx
const RolloutDetailPage = lazy(() => import("@/features/ota/pages/rollout-detail-page").then((m) => ({ default: m.RolloutDetailPage })))
<Route path="/ota/rollouts/:rolloutID" element={<RolloutDetailPage token={token} viewer={viewer} />} />
```

- [ ] **Step 4: 类型检查 + 提交** — `cd web-admin && npx tsc --noEmit 2>&1 | grep -E "features/ota" | head`(无错)。
```bash
git add web-admin/src/features/ota/components/rollout-detail.tsx web-admin/src/features/ota/pages/rollout-detail-page.tsx web-admin/src/features/mistyislet-shell/routes.tsx
git commit -m "feat(web): rollout detail page (monitor + per-gateway + actions)"
```

---

## Task 7: 三语 i18n + 全量门

**Files:** Modify `locales/{en-US,id-ID,zh-CN}.json`

- [ ] **Step 1: i18n** — 在三 locale 文件的 `ota` 命名空间下加 `rollout` 子树(键结构三语一致)。en-US:
```json
"rollout": {
  "pageTitle": "Rollouts", "nav": { "rollouts": "Rollouts" },
  "create": { "title": "Create Rollout", "description": "Roll out a firmware version to a set of gateways in phases.",
    "firmwarePlaceholder": "Select firmware version", "targetAll": "All gateways", "targetBuilding": "By building", "targetGateways": "Specific gateways",
    "buildingPlaceholder": "Select building", "phases": "Phases (cumulative %, last must be 100)", "requiresApproval": "Requires approval", "addPhase": "Add phase", "removePhase": "Remove",
    "failureThreshold": "Failure threshold %", "scheduleSection": "Schedule (optional)", "timezonePlaceholder": "Timezone", "submit": "Create", "submitting": "Creating…",
    "validation": { "firmwareRequired": "Select a firmware version", "phasesInvalid": "Phases must be strictly increasing percentages ending at 100", "buildingRequired": "Select a building", "gatewaysRequired": "Select at least one gateway", "hhmm": "Use HH:MM (24h)" } },
  "list": { "title": "Rollouts", "description": "Firmware rollouts for this tenant.", "loading": "Loading…", "empty": "No rollouts yet.", "colFirmware": "Firmware", "colTarget": "Target", "colState": "State", "colPhase": "Phase", "colCreated": "Created" },
  "detail": { "pageTitle": "Rollout", "title": "Rollout", "loading": "Loading…", "firmware": "Firmware", "target": "Target", "phase": "Phase", "threshold": "Failure threshold", "schedule": "Schedule",
    "gatewaysTitle": "Per-gateway progress", "colGateway": "Gateway", "colPhase": "Phase", "colStatus": "Status", "colVersion": "Current version",
    "actions": { "approve": "Approve", "pause": "Pause", "resume": "Resume", "abort": "Abort" }, "confirmAbortTitle": "Abort rollout?", "confirmAbortBody": "This stops the rollout. Gateways already updated keep the new firmware.", "cancel": "Cancel" },
  "state": { "pending": "Pending", "active": "Active", "awaiting_approval": "Awaiting approval", "paused": "Paused", "completed": "Completed", "failed": "Failed", "scheduled": "Scheduled" },
  "errors": { "generic": "Something went wrong." }
}
```
zh-CN(例:pageTitle="发布"、create.title="创建发布"、state.active="进行中"、state.awaiting_approval="待审批"、detail.actions.abort="中止" 等)+ id-ID 各自翻译;**键结构与 en-US 完全一致**。

- [ ] **Step 2: 全量门** — Run:
```
cd web-admin && npx tsc --noEmit 2>&1 | tail -8 && npm run test:unit 2>&1 | tail -6 && npm run build 2>&1 | tail -8 && npm run lint 2>&1 | grep -E "features/ota" | head
```
Expected: tsc **零错**;全测 PASS;**build 成功**;ota 无新 lint 错。

- [ ] **Step 3: i18n 集成检查** — 确认组件用到的每个 `ota.rollout.*` key 都在三 locale 定义(无缺失→无裸 key 渲染):
```
cd web-admin && node -e 'const fs=require("fs");const g=(o,p)=>p.split(".").reduce((a,k)=>a&&a[k],o);const u=require("child_process").execSync("git grep -ohE \"t\\(\`?\\\"?ota\\.rollout[^\\\"`)]*\" src/features/ota || true").toString();console.log("(手动核对 en-US/id-ID/zh-CN 的 ota.rollout 键集一致 + 组件 t() 调用都有定义)")'
```
(简单做法:`node` 读三 JSON,比较 `ota.rollout` 的扁平键集是否完全相同;再 grep 组件里的 `t("ota.rollout...")` 确认都在 en-US 里。)

- [ ] **Step 4: 提交**
```bash
git add web-admin/src/locales/en-US.json web-admin/src/locales/id-ID.json web-admin/src/locales/zh-CN.json
git commit -m "feat(web): rollout UI i18n (en/id/zh)"
```

---

## 自检(Self-Review)

**1. Spec 覆盖**:§2 契约→Task1;§4 rollout-utils→Task2;hooks(含轮询)→Task3;§5.2 列表+页+路由+nav→Task4;§5.1 完整创建表单(固件/target/phases/阈值/调度)→Task5;§5.3 详情(监控轮询+每网关表+操作+abort确认)→Task6;§7 三语 i18n + 门→Task7。
**2. 占位符**:无 TODO/TBD;"grep 定位/读 ui 改正"是现场确认指令(给了命令)。
**3. 类型一致**:`GatewayRollout`/`RolloutTarget`/`RolloutPhase`/`RolloutSchedule`/`CreateRolloutInput`/`RolloutActionName`/`RolloutGatewayStatus` 全程一致;`createRollout`/`listRollouts`/`getRolloutDetail`/`rolloutAction` 与 hooks 一致;`validatePhases`/`availableRolloutActions`/`rolloutStateBadgeVariant`/`buildingOptions`/`isHHMM`/`buildSchedulePayload`/`targetSummary` 在 utils 与组件一致;query keys `rolloutList`/`rolloutDetail` 一致;i18n key 组件↔locale 一致。
**4. 关键风险**:(a) Badge variant 窄类型→`as never` 或读 badge.tsx 用字符串;(b) Switch/Dialog/Select 子组件名以实际 ui/ 为准;(c) refetchInterval v5 函数签名;(d) Task4 先注释创建卡接入避免编译错,Task5 取消;(e) Intl.supportedValuesOf 回退;(f) 三 locale 键必须同步。

---

## 执行交接(建议 Subagent-Driven)
最大一块 UI;建议 **superpowers:subagent-driven-development**,实现卡片可叠加 frontend-design 但严守 web-admin 风格。
