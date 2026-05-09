# MistyPass App 重构计划

> 参照 Kisi App UX 模式，结合印尼市场需求，制定 Android/iOS 移动端统一改造方案。
> 后端 API 作为 single source of truth，三端（后端 + Android + iOS）同步对齐。

---

## 一、当前问题清单

| # | 问题 | 影响 |
|---|------|------|
| 1 | 无角色标识 — 用户看不到自己是管理员还是住户 | 身份混淆 |
| 2 | 访客凭证无过期时间显示 | 安全风险 + 用户困惑 |
| 3 | 只有 BLE 一种解锁方式 — 无 PIN / QR / NFC | 断网/无 BLE reader 时无法开门 |
| 4 | 语言无法切换 | 印尼用户需要 id-ID / zh-CN / en-US |
| 5 | QR 码在独立 Scanner 页面 — 不属于凭证体系 | 用户不理解用途 |
| 6 | 无环境切换功能 | 调试/联调效率低 |
| 7 | 无摄像头实时画面 | 管理员无法远程确认门口情况 |
| 8 | 管理员看不到管理功能入口 | 角色区分不明确 |

---

## 二、目标架构（参照 Kisi）

### 导航结构重构

```
当前 (5 tabs):
  门列表 | 扫码 | 历史 | 访客 | 我的

改为 (4 tabs):
  门 | 凭证 | 历史 | 我的
       ↑
   合并 QR + 卡 + 访客 pass
```

**底部导航栏：**

| Tab | 图标 | 内容 |
|-----|------|------|
| 门 (Doors) | DoorFront | 门列表 + 收藏置顶 + 一键开锁 |
| 凭证 (Passes) | CreditCard | 卡包样式：我的 QR / NFC 卡 / 访客 pass |
| 历史 (History) | Schedule | 事件日志 + 快照缩略图 |
| 我的 (Profile) | Person | 个人资料 + 设置 + 管理入口 |

---

## 三、各页面改造细节

### 3.1 登录页 (LoginScreen)

**新增功能：**

1. **右上角环境切换按钮** — 三种模式：
   - `生产` → `https://api.mistyislet.com`
   - `测试` → `https://staging-api.mistyislet.com`
   - `本地` → `http://localhost:8081` (开发用)
   - 存储在 DataStore，重启保持
   - UI：小圆点指示当前环境颜色（绿=生产，黄=测试，蓝=本地）

2. **环境切换弹窗：**
   - 显示当前 API 地址
   - 可手动输入自定义 URL
   - 切换后自动清除 token（需重新登录）

**涉及 API：** 无后端改动，纯客户端逻辑。

---

### 3.2 门列表页 (DoorsScreen)

**改进：**

1. **门卡片增强：**
   - 显示门状态图标（在线/离线/告警）
   - 显示最近一次开锁时间
   - 收藏星标（置顶常用门）
   - 长按 → BLE 开锁（保持现有）
   - 单击 → 选择解锁方式弹窗

2. **多种解锁方式：**
   - 云端解锁（现有 `POST /app/access/unlock`）
   - BLE 开锁（现有 challenge-response）
   - PIN 码开锁（新增）
   - QR 码开锁（现有 `POST /app/access/qr-unlock`）

3. **门卡片显示管理员标识：**
   - 如果 `user.role != resident`，在门卡片顶部显示角色徽章

**涉及 API 改动：**

| 方法 | 端点 | 改动 |
|------|------|------|
| GET | `/app/access/my-doors` | 新增返回字段: `last_unlock_at`, `is_favorite` |
| POST | `/app/access/unlock` | 新增 `method` 字段: `"cloud"` / `"pin"` / `"qr"` |
| POST | `/app/access/pin-unlock` | **新增** — PIN 码开锁 |
| PUT | `/app/access/doors/{doorId}/favorite` | **新增** — 收藏/取消收藏 |

---

### 3.3 凭证页 (PassesScreen) — 原 Scanner + Credentials 合并

**卡包样式设计：**

```
┌─────────────────────────────────┐
│  ◉ 我的通行证                    │
│                                  │
│  ┌─── BLE 移动凭证 ───────────┐ │
│  │  [QR 码动态刷新]            │ │
│  │  有效期: 永久               │ │
│  │  设备: Xiaomi 15            │ │
│  │  状态: ● 已激活             │ │
│  └─────────────────────────────┘ │
│                                  │
│  ┌─── NFC 实体卡 ─────────────┐ │
│  │  卡号: **** 3847            │ │
│  │  绑定时间: 2026-05-01      │ │
│  │  状态: ● 已激活             │ │
│  └─────────────────────────────┘ │
│                                  │
│  ── 我发放的访客凭证 ──          │
│                                  │
│  ┌─── 访客: 张三 ─────────────┐ │
│  │  公司: ABC Corp             │ │
│  │  有效期: 2026-05-05 14:00   │ │
│  │       ~ 2026-05-05 18:00   │ │
│  │  状态: ● 待签到             │ │
│  └─────────────────────────────┘ │
│                                  │
│  [+ 发放访客凭证]                │
└─────────────────────────────────┘
```

**关键改进：**
- 我的 QR 码放在凭证卡片内（动态刷新 BLE token 二维码）
- 访客凭证**必须显示有效期**
- 管理员可发放访客凭证，住户只能查看自己的
- 卡片可左滑删除/吊销

**涉及 API 改动：**

| 方法 | 端点 | 改动 |
|------|------|------|
| GET | `/app/visitor-passes` | 新增返回字段: `valid_from`, `valid_until`, `display_label` |
| POST | `/app/visitor-passes` | 新增必填: `valid_from`, `valid_until`（或 `ttl_hours`） |
| GET | `/app/credentials` | 新增: `device_name`, `activated_at` |
| GET | `/app/me` | 新增: `role_display_label`（如 "建筑管理员"） |

---

### 3.4 历史页 (HistoryScreen)

**改进：**
- 事件卡片附带快照缩略图（如果有关联截图）
- 支持下拉刷新 + 分页加载（已修复后端分页）
- 按日期分组显示
- 点击事件可展开查看完整详情 + 大图

**涉及 API 改动：**

| 方法 | 端点 | 改动 |
|------|------|------|
| GET | `/app/access/logs` | 新增返回字段: `snapshot_urls[]` (事件关联快照) |

---

### 3.5 我的页 (ProfileScreen)

**结构：**

```
┌─────────────────────────────────┐
│  头像  姓名                      │
│  角色: 建筑管理员                 │
│  组织: tenant_demo_jakarta       │
├─────────────────────────────────┤
│  > 语言设置                      │
│  > 通知偏好                      │
│  > 安全 (生物识别/修改密码)       │
│  > 摄像头实时画面                 │ ← 管理员可见
│  > 管理员面板                     │ ← 管理员可见
├─────────────────────────────────┤
│  > 关于 MistyPass               │
│  > 退出登录                      │
└─────────────────────────────────┘
```

**新增功能：**

1. **语言切换：** id-ID / zh-CN / en-US 三语
2. **角色显示：** role_display_label（区分管理员/住户）
3. **摄像头入口（管理员）：** 进入摄像头列表 → 选择 → 实时画面
4. **调试入口（隐藏）：** 连续点击 5 次版本号显示开发者选项

**涉及 API 改动：**

| 方法 | 端点 | 改动 |
|------|------|------|
| PATCH | `/app/me` | **新增** — 更新语言偏好 `{ "language": "zh-CN" }` |
| GET | `/app/me` | 新增: `role_display_label`, `organization_name` |

---

### 3.6 摄像头实时画面 (CameraLiveScreen) — 新增页面

**功能：**
- 摄像头列表（显示名称、关联门点、在线状态）
- 点击进入实时预览（RTSP → HLS 流）
- 手动截图按钮
- 最近快照历史

**涉及 API：**

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/app/cameras` | **新增** — 列出用户可见的摄像头 |
| GET | `/app/cameras/{id}/video-link` | **新增** — 获取 HLS/RTSP 播放地址 |
| POST | `/app/cameras/{id}/snapshot` | **新增** — 手动截图 |

---

### 3.7 PIN 码开锁 — 新增功能

**场景：** BLE reader 不可用 / 手机没电时的 fallback 方式。

**流程：**
1. 用户在门详情选择「PIN 码开锁」
2. 弹出数字键盘
3. 输入 6 位动态 PIN
4. 后端验证 PIN 后触发开锁

**涉及 API：**

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/app/access/pin-code` | **新增** — 获取当前有效的动态 PIN（TOTP，30s 轮换） |
| POST | `/app/access/pin-unlock` | **新增** — PIN 验证开锁 |

---

## 四、后端 API 改动汇总

### 4.1 新增端点

| # | 方法 | 路径 | 说明 |
|---|------|------|------|
| 1 | POST | `/api/v1/app/access/pin-unlock` | PIN 码开锁 |
| 2 | GET | `/api/v1/app/access/pin-code` | 获取当前动态 PIN |
| 3 | PUT | `/api/v1/app/access/doors/{doorId}/favorite` | 收藏门 |
| 4 | PATCH | `/api/v1/app/me` | 更新个人偏好（语言等） |
| 5 | GET | `/api/v1/app/cameras` | 移动端摄像头列表 |
| 6 | GET | `/api/v1/app/cameras/{id}/video-link` | 获取播放地址 |
| 7 | POST | `/api/v1/app/cameras/{id}/snapshot` | 移动端手动截图 |

### 4.2 现有端点字段扩展

| 端点 | 新增字段 |
|------|----------|
| `GET /app/me` | `role_display_label`, `organization_name` |
| `GET /app/access/my-doors` | `last_unlock_at`, `is_favorite` |
| `GET /app/access/logs` | `snapshot_urls` (数组) |
| `GET /app/credentials` | `device_name`, `activated_at` |
| `GET /app/visitor-passes` | `valid_from`, `valid_until`, `display_label` |
| `POST /app/visitor-passes` | 接受 `valid_from`, `valid_until` 或 `ttl_hours` |

### 4.3 无改动端点（保持现有）

- `POST /app/auth/login` ✅
- `POST /app/auth/refresh` ✅
- `POST /app/access/unlock` ✅
- `POST /app/access/qr-unlock` ✅
- `GET /app/access/ble-token` ✅
- `POST /app/credentials/register` ✅
- `GET /app/credentials/mobile` ✅
- `DELETE /app/credentials/mobile/{id}` ✅
- `POST /app/credentials/mobile/{id}/refresh` ✅
- `POST /app/devices/register` ✅

---

## 五、三端联调协议

### 对齐原则

1. **后端先行** — 所有 API 改动先在后端实现并通过集成测试
2. **Android 跟进** — 后端 API 稳定后 Android 接入
3. **iOS 后续** — iOS 开发时直接对齐最终 API，不需要再改后端
4. **向后兼容** — 新增字段均为可选（nullable），旧版 App 不会崩溃

### 版本控制

```
API 响应新增 header:
  X-API-Version: 2026.05.1

客户端请求新增 header:
  X-Client-Version: android/1.2.0
  X-Client-Version: ios/1.0.0
```

### 联调顺序

```
Phase A (本周): 后端新增 7 个端点 + 字段扩展
Phase B (下周): Android 重构导航 + 凭证页 + 环境切换
Phase C (后续): iOS 开发直接对齐 Phase A 的 API
```

---

## 六、优先级排序

| 优先级 | 任务 | 工作量 | 依赖 |
|--------|------|--------|------|
| P0 | 登录页环境切换 | 小 | 无 |
| P0 | 凭证页合并（QR + 卡 + 访客） | 中 | 无 |
| P0 | 访客凭证显示有效期 | 小 | 后端字段扩展 |
| P0 | 角色标识显示 | 小 | `GET /app/me` 扩展 |
| P1 | 语言切换 | 中 | `PATCH /app/me` |
| P1 | PIN 码开锁 | 中 | 新增 2 个端点 |
| P1 | 门收藏 + 排序 | 小 | 新增 1 个端点 |
| P2 | 摄像头实时画面 | 大 | HLS 播放器 + 新增 3 个端点 |
| P2 | 事件关联快照 | 中 | 后端字段扩展 |
| P2 | 管理员面板入口 | 中 | 角色判断逻辑 |

---

## 七、UI 设计规范

### 颜色系统

| 用途 | 颜色 | 说明 |
|------|------|------|
| Brand | #4F55FF | 主色调（按钮、链接） |
| Success | #35A853 | 开锁成功、在线状态 |
| Warning | #F1C27A | 离线、待确认 |
| Danger | #E74C3C | 拒绝、告警、过期 |
| Surface | #F7F7F8 | 背景色 |
| Card | #FFFFFF | 卡片背景 |

### 卡片样式

- 圆角: 12dp
- 阴影: elevation 2dp
- 内边距: 16dp
- 状态指示: 左侧 4dp 色带

### 环境指示器颜色

- 生产: 绿色 (#35A853)
- 测试: 黄色 (#F1C27A)
- 本地: 蓝色 (#4F55FF)

---

## 八、文件影响清单

### Android 新增文件

```
ui/passes/PassesScreen.kt          ← 凭证卡包页（替代 Scanner + Credentials）
ui/passes/PassesViewModel.kt
ui/passes/VisitorPassCard.kt
ui/passes/MyCredentialCard.kt
ui/camera/CameraListScreen.kt      ← 摄像头列表
ui/camera/CameraLiveScreen.kt      ← 实时画面播放
ui/camera/CameraViewModel.kt
ui/doors/PinUnlockSheet.kt         ← PIN 码输入弹窗
ui/doors/UnlockMethodSheet.kt      ← 解锁方式选择弹窗
ui/login/EnvironmentSwitcher.kt    ← 环境切换组件
ui/settings/LanguageScreen.kt      ← 语言设置页
data/api/CameraApi.kt              ← 摄像头 Retrofit 接口
data/repository/CameraRepository.kt
domain/model/CameraFeed.kt
```

### Android 修改文件

```
ui/navigation/AppNavigation.kt     ← 4 tabs, 移除 Scanner tab
ui/login/LoginScreen.kt            ← 加环境切换按钮
ui/doors/DoorsScreen.kt            ← 多解锁方式 + 收藏
ui/doors/DoorsViewModel.kt         ← 收藏逻辑 + PIN
ui/history/HistoryScreen.kt        ← 快照缩略图
ui/history/HistoryViewModel.kt
ui/profile/ProfileScreen.kt        ← 角色显示 + 新入口
ui/profile/ProfileViewModel.kt     ← 语言切换
ui/visitors/ (整体移到 passes/)     ← 合并到凭证页
core/network/ApiClientModule.kt    ← 动态 baseUrl
domain/model/ApiModels.kt          ← UserInfo 加 role_display_label
domain/model/VisitorPass.kt        ← 加 valid_from/valid_until
app/build.gradle.kts               ← 移除 hardcoded baseUrl, 改为运行时配置
```

### 后端新增/修改文件

```
api/internal/http/routes_app_access.go   ← pin-unlock, favorite, pin-code
api/internal/http/routes_app_cameras.go  ← 新文件，移动端摄像头 API
api/internal/http/router.go              ← 注册新路由
api/internal/modules/access/service.go   ← PIN 码生成/验证逻辑
```

---

## 九、Kisi 视频分析补充（2026-05-05）

> 基于 Kisi 官方教学视频逐帧分析所得 UX 洞察

### 9.1 门列表 Grid 布局（P0 必改）

Kisi 门列表使用 **2列 Grid**，非 List。每个门卡片包含：
- 锁图标 + 状态文字（LOCKED / UNLOCKED / FAILED）
- 门名称（截断用省略号）
- 方向标注后缀：`(Incoming)` / `(Outgoing)`

**门状态颜色系统：**

| 状态 | 图标 | 颜色 | 可操作 |
|------|------|------|--------|
| 已锁定 (LOCKED) | 🔒 | Brand 蓝 | ✅ 可开门 |
| 已开启 (UNLOCKED) | 🔓 | Success 绿 | ✅ 可关门 |
| 异常 (FAILED) | ⚠️ | Danger 红 | ❌ 不可操作 |
| 离线 | 🔒 | 灰色 | ❌ 不可操作 |

### 9.2 Favorites / All Doors 双 Tab

```
   收藏   │   全部
          ────────   ← 下划线指示
```

- 收藏 Tab 显示星标门（快速访问）
- 全部 Tab 显示所有有权限的门
- 搜索框在 Tab 之上

### 9.3 距离限制反馈（Toast 式）

```
用户点击开门 → 距离太远时：
  底部 Snackbar："您距离该门禁过远，请靠近后再试"
  → 1.5s 自动消失
  → 不阻断操作、不弹全屏 Modal
```

**Toast 文案规范：**
- 开门成功：`"✅ 已开门"` (绿色, 1.5s)
- 距离过远：`"您距离该门禁过远，请靠近后再试"`
- 开门失败：`"⚠️ 开门失败，请重试"`
- 设备离线：`"该设备当前离线"`

### 9.4 权限分阶段申请策略

不一次性弹出所有权限，按使用时机分阶段：

| 时机 | Android 权限 | iOS 权限 |
|------|-------------|----------|
| 首次进入主界面 | 附近设备 (Nearby Devices) | 蓝牙 (Bluetooth) |
| 进入第一个地点 | 位置 (Location) | — |
| 开启 BLE 感应 | 电池优化不受限 | — |

**关键原则：** 在系统权限弹窗前，先用自定义页面解释用途。iOS 蓝牙弹窗只能弹一次。

### 9.5 BLE 感应开门设置页（对应 Kisi Hand Wave）

```
┌────────────────────────────────────┐
│  靠近感应开门                       │
│                                     │
│  将手机放在口袋中，靠近读头时        │
│  自动开门，无需掏出手机操作。        │
│                                     │
│        [动态示意图]                  │
│                                     │
│  开启感应开门  ────────── [Toggle]   │
│                                     │
│  ── 所需权限 ──                     │
│  ● 附近设备     已开启 ✅            │
│  ● 位置权限     已开启 ✅            │
│  ● 电池优化     受限 ⚠️ [前往设置]   │
└────────────────────────────────────┘
```

### 9.6 与 Kisi 不同之处（需额外设计）

| 功能 | Kisi | 我方 MistyPass |
|------|------|----------------|
| 登录方式 | Magic Link（无密码） | 密码 + SSO（后续可加 Magic Link） |
| 凭据类型 | 仅移动凭据 | BLE + NFC + DESFire 实体卡 |
| 离线模式 | 未提及 | Controller 72h 缓存，需提示用户 |
| 访客通行 | 未展示 | 二维码 + 有效期卡片式展示 |
| 组织切换 | iOS 有 Organizations 页 | 需做（印尼多园区常见） |
| 手机丢失 | 未展示 | 需突出快速自助吊销入口 |
| 多地点 | 三级导航（组织→地点→门） | 先单地点，后续扩展 |

### 9.7 更新后的 Android 文件影响

新增：
```
ui/doors/DoorGridCard.kt           ← 2列 Grid 门卡片组件
ui/doors/DoorStatusColors.kt       ← 门状态颜色系统
ui/doors/DistanceToast.kt          ← 距离限制 Toast 组件
ui/onboarding/PermissionScreen.kt  ← 分阶段权限引导页
ui/ble/BLEProximitySettings.kt     ← 感应开门设置页（带权限检查）
```

---

*最后更新: 2026-05-05*
*关联 PR: #49 (camera admin + event-snapshot)*
*参考: /Users/siky/Downloads/Kisi_App_UX_Analysis.md*
