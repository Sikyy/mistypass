#!/bin/bash
# demo-gateway-agent.sh — 测试 Gateway Agent 的完整流程
#
# 前置条件: API 已启动 (PORT=8081, ENABLE_DEMO_USERS=true)
#
# 启动方式：
#   终端 1: cd api && ENABLE_DEMO_USERS=true PORT=8081 go run ./cmd/api
#   终端 2: cd api && go run ./cmd/gateway-agent -api http://localhost:8081 -gateway gw_demo_001 -tenant tenant_demo_jakarta
#
# 然后在终端 2 的 card> 提示符下输入：
#   nfc_uid UID-1001 door_jkt_001    → 应该 ACCESS GRANTED
#   nfc_uid UNKNOWN-999 door_jkt_001 → 应该 ACCESS DENIED
#   rules                             → 查看缓存的 access rules
#
# 如果有硬件：
#   go run ./cmd/gateway-agent -api http://localhost:8081 -gateway gw_demo_001 -tenant tenant_demo_jakarta -relay-gpio 73
#   (GPIO 73 = Orange Pi Zero 3 的 PC9 引脚)
#
# 交叉编译到香橙派：
#   GOOS=linux GOARCH=arm64 go build -o gateway-agent ./cmd/gateway-agent
#   scp gateway-agent orangepi@192.168.1.xxx:~/
#   ssh orangepi@192.168.1.xxx './gateway-agent -api http://192.168.1.100:8081 -gateway gw_demo_001 -tenant tenant_demo_jakarta -relay-gpio 73'

echo "=== Gateway Agent 使用说明 ==="
echo ""
echo "1. 启动 API（另一个终端）:"
echo "   cd api && ENABLE_DEMO_USERS=true PORT=8081 go run ./cmd/api"
echo ""
echo "2. 启动 Agent（本终端）:"
echo "   cd api && go run ./cmd/gateway-agent \\"
echo "     -api http://localhost:8081 \\"
echo "     -gateway gw_demo_001 \\"
echo "     -tenant tenant_demo_jakarta"
echo ""
echo "3. 在 card> 提示符下测试:"
echo "   nfc_uid UID-1001 door_jkt_001    → ACCESS GRANTED (Andri Pratama)"
echo "   nfc_uid UNKNOWN-999 door_jkt_001 → ACCESS DENIED"
echo "   card_number CARD-1001 door_jkt_001 → ACCESS GRANTED"
echo "   rules                             → 查看缓存规则"
echo ""
echo "4. 交叉编译到香橙派:"
echo "   GOOS=linux GOARCH=arm64 go build -o gateway-agent ./cmd/gateway-agent"
echo ""
