# MistyPass 云 SaaS 平台开发文档

## 1. 文档目标

本文档用于定义 MistyPass 云 SaaS 平台的产品边界、系统架构、模块拆分、数据模型、接口规划、开发阶段与验收标准，作为后端研发、Web 管理后台、设备接入服务和后续运维的统一参考。

该文档默认服务于 MistyPass 第一阶段目标，即为雅加达写字楼、联合办公和商业地产场景提供可试点、可交付、可扩展的云访问控制平台。

## 2. 平台定位

MistyPass 云平台是整个门禁系统的控制中枢，主要承担以下职责：

- 提供多租户访问控制 SaaS 能力
- 管理组织、建筑、楼层、门和设备
- 管理用户、权限、凭证和访问策略
- 接入边缘网关并处理实时设备状态
- 为移动 App 提供认证、凭证、日志和开门相关服务
- 提供审计日志、告警和后续计费能力

平台本身不直接控制门锁硬件，而是通过边缘网关实现云边协同。

## 3. 开发目标

### 3.1 MVP 目标

在 `2-3` 个月内完成一个可用于 POC 的最小可用平台，满足以下要求：

- 支持多租户隔离
- 支持组织、建筑、楼层、门点管理
- 支持边缘网关注册、在线状态、配置下发
- 支持用户、用户组、时间段权限和临时权限
- 支持 Android App 的登录、凭证同步和 BLE 开门授权
- 支持实时事件日志和基础告警

### 3.2 非目标

以下内容不纳入 MVP：

- 完整计费与订阅体系
- 复杂 BI 报表
- 第三方开放 API 平台
- 全量 Apple Wallet / Google Wallet 发行体系
- 大量第三方企业系统集成

## 4. 平台用户角色

平台应至少支持以下角色：

- `Super Admin`：平台运营方，管理租户、系统配置、设备批次和异常问题
- `Tenant Admin`：租户管理员，管理本组织的建筑、门、用户、权限和设备
- `Building Admin`：楼宇或项目管理员，管理指定建筑范围内的门点、用户和访客
- `Operator`：运营人员，查看日志、处理告警、协助开门
- `Resident/User`：终端用户，仅通过 App 使用，不直接进入 SaaS 后台

角色模型要支持扩展，但 MVP 阶段不应做过于复杂的细粒度策略引擎。

## 5. 功能范围

### 5.1 Phase 0：平台基础能力

- 多租户模型
- 登录认证
- `JWT + Refresh Token`
- `MFA` 预留
- `RBAC`
- `Super Admin` 控制台
- `Tenant Portal`
- 审计日志基础框架
- `MQTT Broker` 集成

### 5.2 Phase 1：MVP 核心能力

- 组织和建筑模型管理
- 门点和设备模型管理
- 边缘网关接入与状态监控
- 权限和凭证管理
- Android App 用户认证与凭证同步
- BLE 开门授权流程
- 实时事件日志
- 基础告警通知
- 简化版访客凭证能力

### 5.3 Phase 2：增强能力

- Wallet 凭证发行
- 更复杂的时间段与区域权限
- 设备 OTA 策略中心
- 计费与订阅
- 统计分析与报表
- 开放 API
- 高级通知策略

## 6. 业务模块拆分

### 6.1 租户与组织模块

负责平台和客户层级管理，包含：

- Tenant 创建与停用
- Organization 管理
- 建筑、楼层、区域模型
- 门点与门组模型

该模块是所有权限和设备绑定的基础。

### 6.2 身份认证与权限模块

负责后台和 App 的身份认证，包含：

- 管理端登录
- App 用户登录
- 访问令牌签发
- 角色权限校验
- 会话管理
- 密码重置与账户冻结

### 6.3 用户与凭证模块

负责终端用户身份与门禁能力绑定，包含：

- 用户资料管理
- 用户组管理
- 凭证状态管理
- 门禁权限分配
- 临时权限与访客权限
- BLE 开门授权数据生成

### 6.4 设备与网关模块

负责边缘网关的全生命周期管理，包含：

- 设备注册
- 设备激活
- 设备认证
- 在线状态
- 固件版本
- 配置下发
- 远程重启
- OTA 升级任务

### 6.5 实时通信模块

负责云端与边缘网关的实时消息交互，包含：

- MQTT Topic 规划
- 设备心跳
- 状态上报
- 事件上报
- 配置指令下发
- 开门请求回执

### 6.6 事件与审计模块

负责记录业务与安全事件，包含：

- 开门记录
- 拒绝记录
- 设备告警
- 设备上下线
- 配置变更日志
- 管理员操作审计

### 6.7 通知与告警模块

MVP 阶段提供基础能力：

- 设备离线告警
- 非法开门告警
- 异常重试告警
- 邮件或站内消息通知

## 7. 技术架构建议

### 7.1 架构原则

- 云边解耦
- 多租户安全优先
- 核心链路可观测
- 支持离线补偿
- 先单体模块化，后按瓶颈拆服务

### 7.2 MVP 推荐架构

MVP 阶段建议采用模块化单体架构，而不是一开始就拆成大量微服务。

推荐结构如下：

- `API Gateway / BFF`
- `Core Backend`：业务主服务
- `Device Service`：网关接入与消息处理
- `Auth Service`：认证与令牌
- `MQTT Broker`
- `PostgreSQL`
- `Redis`
- `Object Storage`：固件包、导出文件

说明：

- `Core Backend` 与 `Device Service` 可以在代码层分模块、部署层同服务
- 待设备量和日志量上来后，再把事件处理和设备通信拆出来

### 7.3 建议技术栈

- 后端语言：`Go`
- Web 框架：`Gin` 或 `Chi`
- 数据访问：`GORM`（快速交付）或 `sqlc`（强类型 SQL）
- 数据库：`PostgreSQL`
- 缓存与临时状态：`Redis`
- 消息层：`EMQX`
- 实时推送：`WebSocket`
- 对象存储：兼容 `S3`
- 后台前端：`React + TypeScript + shadcn/ui + Tailwind CSS`
- 容器化：`Docker`
- 部署：`Kubernetes` 或轻量云主机集群

之所以优先推荐 `Go`，是因为它在设备接入、并发处理、内存占用和可部署性上更适合门禁云边协同场景，且在 MVP 阶段同样能保持清晰模块边界。

## 8. 逻辑架构

### 8.1 SaaS 后台访问链路

1. 管理员通过 Web 后台登录
2. 后端完成租户身份识别与角色校验
3. 管理员配置建筑、门点、用户和权限
4. 平台将相关配置转换为设备可消费的数据
5. 指令通过 MQTT 下发到指定边缘网关

### 8.2 App 开门授权链路

1. App 用户登录获取访问令牌
2. App 拉取自己的凭证、门权限和时间段数据
3. App 到达门点附近，通过 BLE 与网关建立通信
4. 网关校验本地缓存权限或向云端请求补充校验
5. 网关执行开门并上报事件
6. 云端写入事件日志并推送给管理后台

### 8.3 设备心跳与配置链路

1. 网关启动后使用设备证书或预共享密钥接入 MQTT
2. 周期性上报在线状态、固件版本、输入输出状态
3. 云端检测设备心跳并更新在线状态
4. 管理端修改配置后，平台向指定设备下发配置命令
5. 设备执行后返回确认结果

## 9. 核心数据模型

### 9.1 租户侧核心对象

- `tenant`
- `organization`
- `building`
- `floor`
- `area`
- `door`
- `door_group`

### 9.2 用户侧核心对象

- `user`
- `user_group`
- `credential`
- `access_policy`
- `schedule`
- `temporary_access`
- `visitor_pass`

### 9.3 设备侧核心对象

- `gateway`
- `gateway_binding`
- `gateway_config`
- `firmware_release`
- `ota_job`
- `device_heartbeat`

### 9.4 事件侧核心对象

- `access_event`
- `device_event`
- `alarm_event`
- `audit_log`
- `notification_task`

### 9.5 数据建模原则

- 所有业务表必须具备 `tenant_id`
- 审计和事件表要包含 `source`, `actor`, `door_id`, `gateway_id`
- 权限表需要支持时间段、生效时间和失效时间
- 设备事件与业务事件分表，避免日志表失控

## 10. API 设计范围

### 10.1 管理后台 API

- 认证登录
- 租户与组织管理
- 建筑、楼层、门点管理
- 用户与用户组管理
- 权限与时间表管理
- 网关注册、绑定、状态查询
- 事件日志查询
- 告警查询

### 10.2 App API

- 登录与刷新令牌
- 获取当前用户资料
- 获取我的凭证
- 获取我的门权限列表
- 获取 BLE 开门授权信息
- 获取我的开门记录
- 获取访客邀请码或临时凭证

### 10.3 网关 API / MQTT 命令

- 设备注册
- 设备激活
- 心跳上报
- 状态上报
- 配置下发
- 权限同步
- OTA 任务下发
- 事件回传

### 10.4 MVP API 清单（`/api/v1`）

以下清单用于前后端联调和接口冻结，默认所有业务接口都基于 `tenant_id` 做隔离。

管理后台 API（Admin Portal）：

- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /me`
- `GET /tenants`
- `POST /tenants`
- `PATCH /tenants/{tenantId}/status`
- `GET /buildings`
- `POST /buildings`
- `GET /floors`
- `POST /floors`
- `GET /doors`
- `POST /doors`
- `GET /door-groups`
- `POST /door-groups`
- `GET /users`
- `POST /users`
- `GET /user-groups`
- `POST /user-groups`
- `GET /access-policies`
- `POST /access-policies`
- `POST /temporary-access`
- `POST /visitor-passes`
- `GET /gateways`
- `POST /gateways/register`
- `POST /gateways/{gatewayId}/bind-door`
- `POST /gateways/{gatewayId}/config/publish`
- `POST /gateways/{gatewayId}/reboot`
- `GET /events/access`
- `GET /events/device`
- `GET /alarms`
- `GET /audit-logs`

App API（Resident App）：

- `POST /app/auth/login`
- `POST /app/auth/refresh`
- `GET /app/me`
- `GET /app/credentials`
- `GET /app/access/doors`
- `GET /app/access/ble-token`
- `GET /app/access/logs`
- `POST /app/visitor-passes`

网关 HTTP API（Bootstrap）：

- `POST /gateway/register`
- `POST /gateway/activate`
- `POST /gateway/heartbeat`
- `POST /gateway/status`
- `POST /gateway/events/access`
- `POST /gateway/events/device`

MQTT 命令面（与第 11 章 Topic 对应）：

- `command.sync_access_policy`
- `command.sync_credential`
- `command.update_config`
- `command.reboot`
- `command.start_ota`
- `ack.command_result`
- `event.access_result`
- `event.device_alarm`

## 11. MQTT Topic 建议

MVP 阶段建议按租户和设备维度规划 Topic：

- `mistypass/tenant/{tenantId}/gateway/{gatewayId}/heartbeat`
- `mistypass/tenant/{tenantId}/gateway/{gatewayId}/status`
- `mistypass/tenant/{tenantId}/gateway/{gatewayId}/event`
- `mistypass/tenant/{tenantId}/gateway/{gatewayId}/command`
- `mistypass/tenant/{tenantId}/gateway/{gatewayId}/config`
- `mistypass/tenant/{tenantId}/gateway/{gatewayId}/ota`

如果后期设备量显著增加，再进一步细分租户隔离 Topic 或引入消息编排层。

## 12. 权限引擎设计要求

权限引擎至少需要支持以下条件：

- 用户对门点的访问授权
- 基于用户组的批量授权
- 时间段限制
- 生效与失效时间
- 临时授权
- 访客授权

MVP 阶段不建议支持过于复杂的布尔规则组合，否则会明显拖慢交付。

## 13. 安全要求

### 13.1 平台安全

- 全量 API 强制 `HTTPS/TLS`
- 租户数据隔离
- 管理端权限最小化
- 密码安全存储
- 支持管理员 `MFA`
- 敏感操作写审计日志

### 13.2 设备安全

- 网关唯一身份标识
- 设备接入认证
- 配置下发签名校验
- OTA 包校验
- 指令幂等机制

### 13.3 App 安全配合

- 短期访问令牌
- 刷新令牌轮换
- 设备绑定与风控预留
- 本地凭证加密缓存

## 14. 可观测性要求

平台上线前至少要具备以下观测能力：

- API 请求日志
- 设备在线率
- MQTT 消息成功率
- 开门成功率
- 告警数量趋势
- 设备心跳延迟
- 异常堆栈收集

建议配套：

- 日志：`Loki` 或 `ELK`
- 指标：`Prometheus + Grafana`
- 异常：`Sentry`

## 15. 开发阶段建议

### 15.1 第一阶段：基础骨架

- 初始化后端项目
- 建立认证体系
- 建立租户模型
- 建立 Web 后台基础框架
- 接入 PostgreSQL、Redis、MQTT

### 15.2 第二阶段：核心业务模型

- 组织、建筑、楼层、门点模型
- 用户、用户组、权限模型
- 网关模型与设备状态
- 事件日志模型

### 15.3 第三阶段：设备与 App 闭环

- 网关接入认证
- 权限同步机制
- BLE 开门授权接口
- 实时事件写入
- App 数据接口联调

### 15.4 第四阶段：运维与交付能力

- 基础告警
- OTA 任务
- 审计日志
- 数据导出
- 部署脚本和灰度流程

## 16. 验收标准

MVP 阶段的平台验收至少满足以下要求：

- 支持至少 `3` 个租户隔离运行
- 支持每个租户独立维护建筑、门点、用户和权限
- 支持网关稳定在线与断线恢复
- 支持 Android App 登录、凭证同步和 BLE 开门闭环
- 支持开门事件实时写入和查询
- 支持基础配置下发与状态回执
- 支持基本审计与管理员操作追踪

## 17. 风险与注意事项

- 过早拆微服务会放大开发和部署复杂度
- 多租户隔离如果设计不严，会成为后期无法补救的问题
- 设备消息模型如果定义混乱，后续很难做兼容升级
- 事件表如果不做冷热分层，后期查询性能会迅速恶化
- 后台功能如果先做太多非核心页面，会拖慢主链路闭环

## 18. 下一步输出建议

在这份开发文档基础上，建议继续补齐以下材料：

1. 云平台详细 ER 图
2. MQTT 消息协议文档
3. 后台 Web 管理端页面清单
4. POC 阶段部署架构图

## 19. 后端模块与目录结构建议（Go）

建议采用模块化单体，按 `domain/application/infrastructure` 分层，目录如下：

```text
mistypass/
  cmd/
    api/
      main.go
  internal/
    platform/
      config/
      logger/
      db/
      cache/
      mq/
      http/
      auth/
    modules/
      tenant/
        domain/
        application/
        infrastructure/
        delivery/http/
      iam/
        domain/
        application/
        infrastructure/
        delivery/http/
      access/
        domain/
        application/
        infrastructure/
        delivery/http/
      gateway/
        domain/
        application/
        infrastructure/
        delivery/http/
        delivery/mqtt/
      event/
        domain/
        application/
        infrastructure/
        delivery/http/
      alarm/
        domain/
        application/
        infrastructure/
        delivery/http/
    jobs/
      heartbeat_checker/
      notification_dispatcher/
      ota_scheduler/
  migrations/
  deploy/
    docker/
    k8s/
  docs/
    openapi/
```

模块职责边界建议：

- `tenant`：租户、组织、建筑、楼层、区域、门点主数据
- `iam`：后台账号、角色、会话、令牌、密码策略、MFA 预留
- `access`：用户、用户组、凭证、策略、时间段、临时与访客权限
- `gateway`：设备注册、激活、认证、配置、OTA、远程指令
- `event`：开门事件、设备事件、事件查询与归档
- `alarm`：告警规则、告警状态机、通知任务

跨模块约束：

- `event` 模块只接收领域事件，不反向依赖业务模块细节
- `gateway` 与 `access` 通过应用层接口交互，避免直接访问彼此存储层
- 所有仓储查询必须显式带 `tenant_id`

## 20. 前端管理台结构建议（React + shadcn/ui）

建议使用 `Vite + React + TypeScript + React Router + TanStack Query + shadcn/ui`：

```text
web-admin/
  src/
    app/
      router.tsx
      providers.tsx
    pages/
      auth/
      dashboard/
      tenants/
      buildings/
      doors/
      users/
      access/
      gateways/
      events/
      alarms/
      audit/
    features/
      tenant/
      building/
      door/
      user/
      policy/
      gateway/
      event/
      alarm/
    components/
      layout/
      data-table/
      form/
      status-badge/
    lib/
      api-client/
      auth/
      rbac/
      utils/
```

UI 风格落地建议：

- 组件基线采用 `shadcn/ui`，统一 `Button/Input/Dialog/Table/Form` 交互规范
- 所有列表页统一支持：关键字检索、状态筛选、分页、导出（MVP 可先 CSV）
- 权限敏感操作统一二次确认与审计埋点（例如：远程重启、策略发布）
