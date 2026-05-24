# PDF 报表设计语言适配计划

> 日期：2026-05-23
> 状态：模板、邮件样式、Web Admin 报表类型与 Gotenberg 实渲染验证已落地
> 能力状态：CONTRACT_READY（模板、类型映射与 Gotenberg 渲染链路已验证；真实邮件 Provider 展示仍待联调）
> 基线：PR #93 squash commit `2af7400`

## 1. 目标

把 PDF 报表从默认紫色 SaaS 模板改成 Mistyislet 官网设计语言：

- 使用 obsidian / graphite / mist / smoke / teal / moss / brass / copper 色系。
- PDF 采用官网同源的深色沉浸式基底、hero 图、noise 纹理、半透明面板和 teal 细节信号。
- 六类报表、定时报表邮件和 preview 工具统一视觉。
- 保持 Gotenberg HTML -> PDF 渲染链路不变。

## 2. 已完成

- [x] `api/internal/pdfgen/templates/base.html` 重做 PDF 全局样式。
- [x] PDF masthead 接入官网 `framer-hero.png`，形成更接近官网首屏的深色视觉锚点。
- [x] PDF 背景接入官网 `framer-noise.png`，保留低强度颗粒质感。
- [x] 表格、KPI 卡片、状态 pill、页脚统一为官网深色 soft-border / translucent panel 语言。
- [x] 六类模板移除旧紫色 chart token。
- [x] 热力图改为 teal alpha scale。
- [x] 图表 palette 改为 teal / moss / brass / copper / smoke。
- [x] PDF logo asset 替换为官网白色透明 Mistyislet mark。
- [x] 定时报表邮件正文替换为品牌化 HTML。
- [x] `report-schedules` 兼容旧 report type，并保存为新的 PDF report type。
- [x] Web Admin report schedule 下拉项改为 PDF renderer 实际支持的六类报表。
- [x] preview templates 首页改为 Mistyislet 风格。
- [x] 新增测试，防止旧紫色 token 回归。

## 3. 本轮文件

- `api/internal/pdfgen/templates/base.html`
- `api/internal/pdfgen/templates/weekly_analytics.html`
- `api/internal/pdfgen/templates/events.html`
- `api/internal/pdfgen/templates/unlock_stats.html`
- `api/internal/pdfgen/templates/user_presence.html`
- `api/internal/pdfgen/templates/incidents.html`
- `api/internal/pdfgen/templates/hardware.html`
- `api/internal/pdfgen/renderer.go`
- `api/internal/pdfgen/assets/logo.png`
- `api/internal/pdfgen/assets/hero.png`
- `api/internal/pdfgen/assets/noise.png`
- `api/internal/http/routes_report_schedule.go`
- `api/internal/http/routes_report_schedule_test.go`
- `api/cmd/preview-templates/main.go`
- `web-admin/src/features/reports/pages/report-schedule-page.tsx`

## 4. 待办

- [ ] 若生产 Gotenberg 容器不能访问 CDN，把 Chart.js 和 matrix plugin vendored 到本地 embed asset。
- [ ] 视业务需要，把 mobile app report export placeholder 接到统一 PDF export 链路。
- [ ] 如后续需要控制 PDF 体积，可将 hero 图增加专用压缩版本。

## 5. 验证记录

### 2026-05-23

- [x] `go test ./internal/pdfgen ./internal/http`
- [x] `npm run build`
- [x] `git diff --check`
- [x] preview templates 六类 HTML 路由返回 200。
- [x] preview HTML 未检出旧紫色 token。
- [x] 记录当时的 Gotenberg 阻塞原因：本机 Docker/OrbStack socket 不可用，`localhost:3000` 是官网/Next.js 服务而不是 Gotenberg。

### 2026-05-24

- [x] 电脑重启后确认 OrbStack/Docker 已恢复，`mistypass` 主服务容器运行中。
- [x] 启动临时 Gotenberg：`mistypass-gotenberg-test`，`127.0.0.1:3010/health` 返回 200。
- [x] 使用最终深色官网风格模板渲染六类 PDF，全部返回 200。
- [x] `pdftotext` 可抽取六类 PDF 标题和 Mistyislet 品牌文本。
- [x] `qlmanage` 为六类 PDF 生成缩略图成功。
- [x] 抽查 `weekly_analytics`、`events`、`hardware` 缩略图，确认图表非空、事件表格未被挤到第二页、硬件图表无明显右侧裁切。
- [x] 将 Gotenberg 默认 `waitDelay` 从 `500ms` 提高到 `4000ms`，降低 Chart.js 图表偶发空白风险。

## 6. 验收清单

- [x] HTML 输出包含 Mistyislet 品牌 token。
- [x] HTML 输出不包含旧紫色 token：`#5046E5`、`#E8E7FB`、`#8B5CF6`、`#EC4899`、`80,70,229`。
- [x] 新旧 report schedule type 能映射到 PDF renderer 支持的 report type。
- [x] 六类 PDF 经 Gotenberg 渲染后图表非空、表格不溢出、页眉页脚正常。
- [ ] 定时报表邮件在真实 Resend 环境中展示正常。
