# 企业接入设计规范（Domain + IdP + Employee Sync）

当前能力状态：

- `CONTRACT_READY`：企业域名识别、IdP 配置校验、`/enterprise/auth/start`、`/enterprise/auth/exchange`、`/enterprise/auth/logout`、OIDC/SAML callback（含 OIDC `code -> id_token` 交换）、员工同步补偿，以及 `sync_mode=jit` 的目录缺失自动建档 + 停用阻断 + 本地会话权限对齐链路已具备联调条件。
- `SKELETON_ONLY`：SCIM/HRIS 深属性映射第二阶段仍在迭代（已落地审批门禁开关与停用撤权第二阶段联动）。

## 1. 适用范围

- 适用于企业域名识别、企业登录接入、员工目录同步、权限自动分配。
- 作为后续实现、联调和测试的统一规范基线。

## 2. 命名规范

- API 路径统一前缀：`/api/v1/enterprise/*`
- 数据表统一前缀：`enterprise_*`
- ID 前缀约定：
  - `dm_`：domain mapping
  - `idp_`：idp config
  - `emp_`：enterprise employee
  - `syn_`：sync job

## 3. 字段规范

- `tenant_id`：必填，除 `super_admin` 外必须与 token 中租户一致。
- `domain`：小写、无前缀 `@`、禁止 URL 格式。
- `provider`：`oidc | saml`
- `status`：统一 `active | inactive`（同步任务例外）。
- 所有时间字段使用 UTC `RFC3339`。

## 4. API 规范

- 成功返回：
  - 读：`200`
  - 创建：`201`
  - 异步提交：`202`
- 失败返回：
  - 参数错误：`400`
  - 未认证：`401`
  - 权限不足：`403`
  - 资源不存在：`404`
  - 冲突：`409`
- 错误体统一：`{"error":"<message>"}`。

## 5. 安全规范

- 严格 RBAC 与租户隔离；跨租户请求统一返回 `tenant scope forbidden`。
- 不保存明文密钥，仅存 `secret_ref` 或外部引用。
- 企业登录交换必须校验：
  - 邮箱域名归属租户
  - IdP 配置启用状态
  - provider 一致性
- 生产环境必须启用审计日志与 request_id 追踪。

## 6. 同步规范

- 员工同步支持 `manual | scheduled | jit` 三模式。
- 幂等键优先级：`tenant_id + external_id`，缺失时回退 `tenant_id + email`。
- 同步结果必须返回统计：
  - `created`
  - `updated`
  - `deactivated`
  - `rejected`
- enterprise -> access 联动写入需携带稳定 identity（`sync_source + sync_ref`），其中 `sync_ref` 采用 `external_id` 优先、`email` 回退。
- 拒绝记录需可追溯到字段原因（后续版本补充 `error_detail` 表）。

## 7. 权限模板规范（MVP）

- `department` 到 `access_role`：
  - `security/satpam/guard` -> `operator`
  - `facility/engineering/building` -> `building_admin`
  - 其他 -> `resident`
- `location` 到 `building_id`：
  - `jakarta/jkt/sudirman` -> Jakarta building
  - `factory/pabrik/bandung/bekasi` -> Factory building

## 8. 可观测性规范

- 关键指标：
  - 域名识别成功率
  - 登录交换成功率
  - 员工同步成功率
  - 同步拒绝率
- 告警阈值（建议）：
  - 登录交换失败率 > 5%（5 分钟窗口）
  - 同步任务失败连续 3 次

## 9. 测试规范

- 单元测试：
  - 域名标准化与匹配
  - provider/status 校验
  - 权限模板计算
- 集成测试：
  - `tenant_admin` 跨租户阻断
  - 同步任务统计准确性
  - 身份交换 provider mismatch 拒绝

## 10. 版本演进规范

- 任何 breaking change 需提升 API 次版本并记录迁移说明。
- 表结构变更必须包含可回滚 SQL。
- 对外字段新增遵循向后兼容原则。
- JIT 深属性映射与撤权联动收口以 `docs/enterprise/jit-deep-attribute-mapping-draft.md` 为当前草案基线。
