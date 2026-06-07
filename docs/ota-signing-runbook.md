# OTA 固件签名运维手册

> 日期：2026-06-07
> 能力状态：CONTRACT_READY（签名/验签/自更新软件完成且单测通过；待 Orange Pi 真机冒烟验证，见 §5）

## 信任模型(必读)
- 信任锚 = agent 内固定的公钥(`--ota-pubkey`)。
- **私钥离线托管**:只在你本地/一台不对外的机器,绝不复制到 API/staging(Mac mini)。被攻破的服务器没有私钥 → 伪造不出能过验签的固件。
- 验签公钥固定在 agent,绝不与固件一起动态下发。

## 1. 一次性:生成密钥对
```bash
cd api && go run ./cmd/ota-sign gen-key --out-priv ota-priv.pem --out-pub ota-pub.hex
```
- `ota-priv.pem` 离线保管(密码管理器/离线盘),`chmod 600`。
- `ota-pub.hex` 填进 agent 的 `--ota-pubkey`。

## 2. agent 端固定公钥(systemd)
部署 `docs/deployment/gateway-agent.service`(已含 `Restart=always` + `ExecStartPre` 守护 —— 自动回滚的前提),把其中 `REPLACE_*` 占位(`--ota-pubkey` 填 `ota-pub.hex` 内容、`--gateway`、`--tenant`)改成实际值。
再把 `docs/deployment/mistypass-ota-guard.sh` 装到 `/usr/local/bin/mistypass-ota-guard.sh` 并 `chmod +x`。安装步骤见 service 文件头部注释。

## 3. 发布一次签名更新
```bash
# 构建目标平台二进制(版本号经 ldflags 注入,用于防降级;不带版本会是 "dev" 导致防降级失效)
cd api && make gateway-agent-release VERSION=1.4.0
# 离线签名(私钥不离开本机)
go run ./cmd/ota-sign sign --key ota-priv.pem --version 1.4.0 --in gateway-agent-1.4.0 \
  --gateway gw_demo_001 --tenant tenant_demo_jakarta \
  --url https://cdn.example.com/firmware/gw_demo_001/1.4.0
# 上传 gateway-agent-1.4.0 到上面的 --url(任意静态托管;签名而非 TLS 是完整性锚)
# 用打印出的 JSON 创建任务:
curl -X POST https://api.example.com/api/v1/gateways/gw_demo_001/ota/tasks \
  -H "Authorization: Bearer <admin-token>" -H "Content-Type: application/json" \
  -d @task.json
```
服务端会拒绝缺少 `firmware_sha256` / `firmware_signature` 的任务(400)。

## 4. 密钥轮换
`--ota-pubkey` 支持逗号分隔多把:先把新公钥追加进去部署(agent 接受新旧两把)→ 之后改用新私钥签名 → 全部网关切换后,移除旧公钥。

## 5. 真机验证(Orange Pi)
- **正常路径**:发布 version 高于当前的签名更新 → agent 日志出现 `OTA installed; exiting...` → systemd 重启 → 下次 pull 成功后 `OTA confirmed healthy` → 后台任务状态变 `succeeded`。
- **强制回滚**:用一个**起不来**的二进制(如 `printf '#!/bin/sh\nexit 1' > bad; ` 包装,或截断的二进制)走同样流程签名发布 → 观察 `ExecStartPre` 守护在 3 次启动后 `rolled back`,旧二进制恢复运行;后台任务状态最终为 `failed`,error_message 记录回滚原因。
- **验签失败**:把 `--url` 指向被篡改 1 字节的二进制 → agent `verify:` 失败 → `report failed`,**二进制原封不动**继续跑旧版。
