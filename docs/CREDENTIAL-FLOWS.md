# Mistyislet 凭证发卡与核验流程

> 更新日期：2026-04-29
> 参考：Kisi Credentials 文档、Apple Wallet Developer 文档、Google Wallet API 文档

本文档描述 Mistyislet 支持的四种凭证类型的发卡（Issuance）和核验（Verification）流程。

---

## 1. 凭证类型总览

| 凭证类型 | 载体 | 发卡方 | 核验方式 | 适用场景 |
|---|---|---|---|---|
| Apple Wallet Pass | iPhone/Apple Watch Wallet | 用户自助注册，Admin 管理 | NFC Tap（Reader Pro） | 正式员工、长期访客 |
| Google Wallet Pass | Android Google Wallet | Admin 发放，用户保存 | NFC Tap（Reader Pro） | 正式员工、长期访客 |
| Physical Card | 实体 HF/DESFire 卡 | Admin 入库 + 分配 | RFID/NFC Reader | 所有人员 |
| Digital Credential | 浏览器链接 / QR Code | Admin 创建分享 | 访问链接点击 / QR 扫描 | 临时访客、短期工 |

---

## 2. Apple Wallet Pass

### 2.1 发卡流程

```
用户（iPhone）                    Mistyislet 后端                   Apple Push / PassKit
    |                                   |                                   |
    |  1. 打开 App / 自助注册页面        |                                   |
    |  ────────────────────────────►    |                                   |
    |                                   |                                   |
    |  2. POST /app/credentials/        |                                   |
    |     apple-pass                    |                                   |
    |     {device_id, device_model}     |                                   |
    |  ────────────────────────────►    |                                   |
    |                                   |                                   |
    |                    3. 验证用户权限（Role Assignment / Group 成员）       |
    |                    4. 生成 pass.json（serial, org, auth_token）        |
    |                    5. 构建 .pkpass bundle:                             |
    |                       - pass.json（门禁信息、NFC payload）              |
    |                       - icon/logo/strip 图片资源                       |
    |                       - manifest.json（文件 SHA256 摘要）               |
    |                       - signature（用 Pass Type Certificate 签名）     |
    |                    6. 记录 device registration + serial → device 映射  |
    |                                   |                                   |
    |  ◄────────────────────────────    |                                   |
    |  7. 返回 .pkpass 文件              |                                   |
    |     用户点击「Add to Wallet」       |                                   |
    |                                   |                                   |
    |  8. iPhone 验证签名，              |                                   |
    |     存入 Wallet                    |                                   |
    |                                   |                                   |
    |                                   |  9. 注册 device push token         |
    |                                   |  ────────────────────────────►    |
    |                                   |                                   |
```

**关键要素：**
- Pass Type ID + Certificate：Apple Developer 后台创建，用于签名 .pkpass
- Auth Token：每个 pass 独立 token，用于后续 device callback 鉴权
- Serial Number：pass 唯一标识，用于更新和撤销
- NFC Payload：写入 pass 的 NFC message，Reader 读取后发给后端核验

### 2.2 核验流程（开门）

```
用户（iPhone/Watch）              Kisi Reader Pro                Mistyislet 后端
    |                                   |                             |
    |  1. 靠近 Reader（NFC 范围内）      |                             |
    |  ────── NFC Tap ──────────►      |                             |
    |                                   |                             |
    |     2. Reader 读取 pass 的         |                             |
    |        NFC payload（含 auth       |                             |
    |        token + serial）            |                             |
    |                                   |                             |
    |                                   |  3. 发送核验请求              |
    |                                   |     POST /verify             |
    |                                   |     {serial, auth_token,     |
    |                                   |      reader_id, timestamp}   |
    |                                   |  ────────────────────────►  |
    |                                   |                             |
    |                                   |     4. 后端验证：             |
    |                                   |        - auth_token 有效     |
    |                                   |        - pass 状态 active    |
    |                                   |        - 用户有门禁权限       |
    |                                   |          （Role Assignment   |
    |                                   |           或 Group 成员）     |
    |                                   |        - 时间窗口检查         |
    |                                   |        - 记录 access event   |
    |                                   |                             |
    |                                   |  ◄────────────────────────  |
    |                                   |  5. 返回 {granted: true}     |
    |                                   |                             |
    |  ◄── 6. 开门 ────────────────    |                             |
    |     （继电器/电锁释放）             |                             |
```

### 2.3 Admin 管理

```
Admin                             Mistyislet 后端                   Apple Push
    |                                   |                              |
    |  暂停 pass                         |                              |
    |  PATCH /cards/:id/deactivate      |                              |
    |  ────────────────────────────►    |                              |
    |                                   |  更新 pass 状态 = suspended   |
    |                                   |  推送 pass 更新通知            |
    |                                   |  ──────────────────────────► |
    |                                   |                              |
    |  撤销 pass                         |                              |
    |  POST /cards/:id/revoke           |                              |
    |  ────────────────────────────►    |                              |
    |                                   |  更新 pass 状态 = revoked     |
    |                                   |  推送 pass 移除通知            |
    |                                   |  ──────────────────────────► |
    |                                   |                              |
    |                                   |  用户 Wallet 中 pass          |
    |                                   |  显示为不可用或自动移除         |
```

---

## 3. Google Wallet Pass

### 3.1 发卡流程

```
Admin                         Mistyislet 后端                 Google Wallet API
    |                                |                               |
    |  1. POST /wallet/passes/issue  |                               |
    |     {template_id, target_id}   |                               |
    |  ──────────────────────────►  |                               |
    |                                |                               |
    |              2. 验证 template 和 target 权限                     |
    |              3. 创建 Pass Class（如尚不存在）                     |
    |                                |                               |
    |                                |  4. POST /walletobjects/       |
    |                                |     genericObject              |
    |                                |     {classId, id, nfc_payload, |
    |                                |      barcode, holder_info}     |
    |                                |  ──────────────────────────►  |
    |                                |                               |
    |                                |  ◄──────────────────────────  |
    |                                |  5. 返回 object resource       |
    |                                |                               |
    |              6. 生成 Save Link（JWT 签名）:                      |
    |                 https://pay.google.com/gp/v/save/{jwt}          |
    |              7. 记录 PassInstance + save_link                    |
    |                                |                               |
    |  ◄──────────────────────────  |                               |
    |  8. 返回 {pass, save_link}     |                               |
    |                                |                               |
    |  9. 通过邮件/短信发送 save_link  |                               |
    |     给用户                      |                               |
    |                                |                               |
```

**用户保存：**
```
用户（Android）                  Google Wallet
    |                                |
    |  1. 点击 save_link              |
    |  ──────────────────────────►  |
    |                                |
    |  2. Google Wallet 展示 pass     |
    |     预览并确认保存               |
    |                                |
    |  3. Pass 存入 Google Wallet     |
    |     含 NFC payload              |
    |                                |
```

### 3.2 核验流程（开门）

与 Apple Wallet 核验流程相同：
1. 用户手机靠近 Reader（NFC）
2. Reader 读取 NFC payload（auth token + pass ID）
3. Reader 发送核验请求到后端
4. 后端验证 token、pass 状态、权限、时间窗口
5. 返回 granted/denied
6. Reader 控制门锁

### 3.3 Pass 更新

```
Admin                         Mistyislet 后端                 Google Wallet API
    |                                |                               |
    |  PATCH /cards/:id/deactivate   |                               |
    |  ──────────────────────────►  |                               |
    |                                |                               |
    |                                |  PATCH /walletobjects/{id}    |
    |                                |  {state: "INACTIVE"}          |
    |                                |  ──────────────────────────► |
    |                                |                               |
    |                                |  用户 Google Wallet 中         |
    |                                |  pass 自动更新为不可用          |
```

---

## 4. Physical Card（实体卡）

### 4.1 发卡流程

```
Admin                         Mistyislet 后端                 卡片/读卡器
    |                                |                            |
    |  库存管理阶段                    |                            |
    |                                |                            |
    |  1a. 手动入库                   |                            |
    |  POST /physical-card-inventory |                            |
    |  {card_number, uid, vendor_id} |                            |
    |  ──────────────────────────►  |                            |
    |                                |                            |
    |  1b. 或 读卡器扫描入库           |                            |
    |  POST /physical-card-inventory |   扫描卡片 UID              |
    |       /scan                    |  ◄────────────────────────|
    |  ──────────────────────────►  |                            |
    |                                |                            |
    |  1c. 或 CSV 批量导入            |                            |
    |  POST /physical-card-inventory |                            |
    |       /import-csv              |                            |
    |  ──────────────────────────►  |                            |
    |                                |                            |
    |  分配阶段                       |  库存状态: available         |
    |                                |                            |
    |  2. 创建发卡任务                 |                            |
    |  POST /physical-card-tasks     |                            |
    |  {pass_id, task_type: "issue", |                            |
    |   card_number, inventory_id}   |                            |
    |  ──────────────────────────►  |                            |
    |                                |  库存状态: reserved → issued |
    |                                |                            |
    |  3. Admin 将实体卡交给用户       |                            |
    |                                |                            |
    |  4. 更新任务状态为 completed     |                            |
    |  PATCH /physical-card-tasks    |                            |
    |        /{taskID}/status        |                            |
    |  ──────────────────────────►  |                            |
```

### 4.2 核验流程（开门）

```
用户（持实体卡）                Reader（RFID/NFC）            Mistyislet 后端
    |                                |                            |
    |  1. 刷卡                       |                            |
    |  ────── RFID/NFC ──────►      |                            |
    |                                |                            |
    |     2. Reader 读取卡片 UID      |                            |
    |                                |                            |
    |                                |  3. 发送核验请求            |
    |                                |     POST /verify            |
    |                                |     {uid, reader_id}        |
    |                                |  ──────────────────────►   |
    |                                |                            |
    |                                |     4. 后端验证：            |
    |                                |        - UID → card_assignment
    |                                |        - card 状态 active   |
    |                                |        - 用户权限检查        |
    |                                |        - 时间窗口检查        |
    |                                |        - 记录 access event  |
    |                                |                            |
    |                                |  ◄──────────────────────   |
    |                                |  5. 返回 {granted: true}    |
    |                                |                            |
    |  ◄── 6. 开门 ────────────────|                            |
```

---

## 5. Digital Credential（访问链接 / QR Code）

### 5.1 发卡流程

```
Admin                         Mistyislet 后端                 通知渠道
    |                                |                            |
    |  1. 创建 Share / Group Link     |                            |
    |  POST /shares 或               |                            |
    |  POST /group_links             |                            |
    |  {group_id, email, valid_until,|                            |
    |   delivery_method}             |                            |
    |  ──────────────────────────►  |                            |
    |                                |                            |
    |              2. 生成 access token                            |
    |              3. 生成访问链接：                                 |
    |                 https://app.mistyislet.com/                  |
    |                   access-link/{token}                        |
    |              4. 如有 QR，生成 QR Code                         |
    |                 （token 编码到 QR 图片）                       |
    |                                |                            |
    |  ◄──────────────────────────  |                            |
    |  5. 返回 {share, access_link}   |                            |
    |                                |                            |
    |  6. 通过邮件/短信/WhatsApp       |                            |
    |     发送链接给访客               |  ──────────────────────►  |
    |                                |                            |
```

### 5.2 核验流程

**方式 A：访问链接（浏览器）**

```
访客（浏览器）                   Mistyislet 后端                门锁控制器
    |                                |                            |
    |  1. 点击访问链接                |                            |
    |  GET /access-link/{token}      |                            |
    |  ──────────────────────────►  |                            |
    |                                |                            |
    |              2. 验证 token 有效性                              |
    |                 - token 存在且未过期                           |
    |                 - 时间窗口内                                   |
    |                 - 写回 last_used_at / claimed_at              |
    |                                |                            |
    |  ◄──────────────────────────  |                            |
    |  3. 展示可开启的门列表          |                            |
    |                                |                            |
    |  4. 访客点击某扇门               |                            |
    |  POST /access-link/{token}/    |                            |
    |       unlock                   |                            |
    |  ──────────────────────────►  |                            |
    |                                |                            |
    |              5. 验证权限 + 记录 event                         |
    |                                |  6. 发送开门指令             |
    |                                |  ──────────────────────►  |
    |                                |                            |
    |  ◄──────────────────────────  |                            |
    |  7. 返回 {unlocked: true}       |                            |
```

**方式 B：QR Code（终端扫描）**

```
访客（持 QR Code）              Kisi Terminal                 Mistyislet 后端
    |                                |                            |
    |  1. 向终端展示 QR Code          |                            |
    |  ────── 摄像头扫描 ──────►     |                            |
    |                                |                            |
    |     2. Terminal 解码 QR          |                            |
    |        获取 token               |                            |
    |                                |                            |
    |                                |  3. POST /group_links/verify|
    |                                |     {token, terminal_id}    |
    |                                |  ──────────────────────►   |
    |                                |                            |
    |                                |     4. 验证 token + 权限    |
    |                                |        写回 claimed_at      |
    |                                |        记录 audit event     |
    |                                |                            |
    |                                |  ◄──────────────────────   |
    |                                |  5. 返回 {valid: true,      |
    |                                |          group_id}          |
    |                                |                            |
    |  ◄── 6. 开门 ────────────────|                            |
```

---

## 6. 凭证生命周期对比

```
               创建          分配          激活          暂停         撤销/删除
               ──────        ──────        ──────        ──────       ──────

Apple Pass     自助注册       自动          NFC Tap       Admin 暂停    Admin 撤销
               enrollment    (立即可用)     首次使用       → push 更新   → push 移除

Google Wallet  Admin 发放     Save Link     用户保存       Admin 暂停    Admin 撤销
               issue pass    → 邮件通知     到 Wallet      → API 更新    → API 移除

Physical Card  入库           Task assign   交付卡片       Admin 冻结    丢失报告
               inventory     → reserved     → issued       → frozen      → scrapped

Digital Cred   创建 Share     链接发送       访客首次       Admin 删除    过期自动
               / Group Link  → 邮件/短信    claim          share/link    失效
```

---

## 7. 数据流对比

| 步骤 | Apple Pass | Google Wallet | Physical Card | Digital Credential |
|---|---|---|---|---|
| **后端存储** | PassInstance + device reg | PassInstance + Google object ID | PassInstance + inventory item + task | Share / GroupLink |
| **用户侧存储** | iPhone Wallet .pkpass | Google Wallet pass object | 实体卡片 | 浏览器链接 / QR 图片 |
| **核验通道** | NFC → Reader → 后端 | NFC → Reader → 后端 | RFID/NFC → Reader → 后端 | HTTPS → 后端 → 控制器 |
| **离线能力** | 支持（NFC payload 本地） | 支持（NFC payload 本地） | 支持（卡片 UID 本地） | QR 离线可扫，链接需网络 |
| **更新推送** | APNs push → 重新下载 pass | Google API → 自动更新 | 无推送，物理回收 | 无推送，链接即时生效 |

---

## 8. Mistyislet 当前实现状态

| 能力 | Apple Pass | Google Wallet | Physical Card | Digital Credential |
|---|---|---|---|---|
| 发卡/创建 | 自助 enrollment 记录 | Save link 生成 | 完整 inventory + task | Share + Group Link |
| .pkpass / pass object | mock stub | mock stub | N/A | N/A |
| NFC payload | 待实现 | 待实现 | 依赖 Reader firmware | N/A |
| 核验 | 待 Reader 对接 | 待 Reader 对接 | 待 Reader 对接 | token verify 已落地 |
| Admin 管理 | activate/deactivate/revoke | activate/deactivate/revoke | status 治理已完整 | CRUD + claim 审计 |
| 推送更新 | mock stub | mock stub | N/A | N/A |
