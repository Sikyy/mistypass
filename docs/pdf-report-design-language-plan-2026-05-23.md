# PDF 报表设计语言适配计划

> 日期：2026-05-23
> 状态：已开始执行，首批模板与邮件样式已落地
> 基线：PR #93 squash commit `2af7400`

## 1. 目标

把 PDF 报表从默认紫色 SaaS 模板改成 Mistyislet 官网设计语言：

- 使用 obsidian / graphite / mist / smoke / teal / moss / brass / copper 色系。
- PDF 采用“深色品牌 masthead + warm paper 数据正文”，兼顾品牌识别和 A4 可读性。
- 六类报表、定时报表邮件和 preview 工具统一视觉。
- 保持 Gotenberg HTML -> PDF 渲染链路不变。

## 2. 已完成

- [x] `api/internal/pdfgen/templates/base.html` 重做 PDF 全局样式。
- [x] PDF masthead 改为 Mistyislet 深色品牌头部。
- [x] 表格、KPI 卡片、状态 pill、页脚统一为官网色系。
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
- `api/internal/http/routes_report_schedule.go`
- `api/internal/http/routes_report_schedule_test.go`
- `api/cmd/preview-templates/main.go`
- `web-admin/src/features/reports/pages/report-schedule-page.tsx`

## 4. 待办

- [ ] 用 Gotenberg 生成六类 PDF，做人工截图审查。
- [ ] 若生产 Gotenberg 容器不能访问 CDN，把 Chart.js 和 matrix plugin vendored 到本地 embed asset。
- [ ] 视业务需要，把 mobile app report export placeholder 接到统一 PDF export 链路。
- [ ] 若客户明确需要深色 PDF 全主题，再单独增加 `dark_report` 模式。

## 5. 验收清单

- [x] HTML 输出包含 Mistyislet 品牌 token。
- [x] HTML 输出不包含旧紫色 token：`#5046E5`、`#E8E7FB`、`#8B5CF6`、`#EC4899`、`80,70,229`。
- [x] 新旧 report schedule type 能映射到 PDF renderer 支持的 report type。
- [ ] 六类 PDF 经 Gotenberg 渲染后图表非空、表格不溢出、页眉页脚正常。
- [ ] 定时报表邮件在真实 Resend 环境中展示正常。
