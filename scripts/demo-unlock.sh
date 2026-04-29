#!/bin/bash
# demo-unlock.sh — 演示完整的解锁流程
# 前置条件: NATS 已运行, API 已启动(带 NATS_ENABLED=true), Gateway Simulator 已启动
#
# 快速启动方式（3 个终端）:
#
#   终端 1: docker compose up nats -d && NATS_ENABLED=true ENABLE_DEMO_USERS=true go run ./cmd/api
#   终端 2: go run ./cmd/gateway-simulator -gateway gw_demo_001
#   终端 3: bash scripts/demo-unlock.sh
#
# 或者用 docker compose 全部启动:
#   docker compose up -d
#   go run ./cmd/gateway-simulator -gateway gw_demo_001  (单独跑 simulator)
#   bash scripts/demo-unlock.sh

set -e

API="${MISTYPASS_API:-http://127.0.0.1:8081}"

echo "=== MistyPass 解锁流程演示 ==="
echo ""

# 1. 登录获取 token
echo "1. 登录..."
LOGIN_RESPONSE=$(curl -sS -X POST "$API/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"organization.admin@mistypass.local","password":"admin123"}')

TOKEN=$(echo "$LOGIN_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('access_token',''))" 2>/dev/null || echo "")

if [ -z "$TOKEN" ]; then
  echo "   登录失败: $LOGIN_RESPONSE"
  exit 1
fi
echo "   Token: ${TOKEN:0:20}..."

# 2. 查看可用门点
echo ""
echo "2. 查看门点列表..."
LOCKS=$(curl -sS "$API/api/v1/locks?tenant_id=tenant_demo_jakarta&limit=3" \
  -H "Authorization: Bearer $TOKEN")
echo "   $LOCKS" | python3 -c "
import sys, json
data = json.load(sys.stdin)
items = data.get('items', [])
for item in items:
    print(f'   - {item[\"id\"]}: {item[\"name\"]}')
if not items:
    print('   (无门点)')
" 2>/dev/null || echo "   $LOCKS"

# 3. 解锁第一扇门
LOCK_ID=$(echo "$LOCKS" | python3 -c "import sys,json; items=json.load(sys.stdin).get('items',[]); print(items[0]['id'] if items else '')" 2>/dev/null || echo "")

if [ -z "$LOCK_ID" ]; then
  echo ""
  echo "没有可用的门点。请先创建 Place 和 Door。"
  exit 1
fi

echo ""
echo "3. 解锁门 $LOCK_ID ..."
UNLOCK_RESPONSE=$(curl -sS -X POST "$API/api/v1/locks/$LOCK_ID/unlock?tenant_id=tenant_demo_jakarta" \
  -H "Authorization: Bearer $TOKEN")
echo "   响应: $UNLOCK_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f'   Action: {data.get(\"action\")}')
print(f'   Status: {data.get(\"status\")}')
print(f'   Lock:   {data.get(\"lock_id\")}')
" 2>/dev/null || echo "   $UNLOCK_RESPONSE"

# 4. 凭证验证 — 模拟 NFC 刷卡
echo ""
echo "4. NFC 刷卡验证 (UID-1001 → door_jkt_001)..."
VERIFY_RESPONSE=$(curl -sS -X POST "$API/api/v1/verify-credential" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "gateway_id": "gw_demo_001",
    "reader_id": "gdv_demo_001",
    "lock_id": "door_jkt_001",
    "tenant_id": "tenant_demo_jakarta",
    "credential_type": "nfc_uid",
    "credential_data": "UID-1001"
  }')
echo "$VERIFY_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f'   Decision: {data.get(\"decision\")}')
print(f'   Reason:   {data.get(\"reason\")}')
print(f'   User:     {data.get(\"user_name\",\"\")} ({data.get(\"user_email\",\"\")})')
print(f'   Group:    {data.get(\"group_name\",\"\")}')
print(f'   Lock:     {data.get(\"lock_id\")}')
" 2>/dev/null || echo "   $VERIFY_RESPONSE"

# 5. 凭证验证 — 未知卡号
echo ""
echo "5. 未知 NFC 卡验证 (UNKNOWN-999)..."
DENY_RESPONSE=$(curl -sS -X POST "$API/api/v1/verify-credential" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "gateway_id": "gw_demo_001",
    "lock_id": "door_jkt_001",
    "tenant_id": "tenant_demo_jakarta",
    "credential_type": "nfc_uid",
    "credential_data": "UNKNOWN-999"
  }')
echo "$DENY_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f'   Decision: {data.get(\"decision\")}')
print(f'   Reason:   {data.get(\"reason\")}')
" 2>/dev/null || echo "   $DENY_RESPONSE"

# 6. Lockdown
echo ""
echo "6. Place lockdown (building_demo_001)..."
LOCKDOWN_RESPONSE=$(curl -sS -X POST "$API/api/v1/places/building_demo_001/lock_down?tenant_id=tenant_demo_jakarta" \
  -H "Authorization: Bearer $TOKEN")
echo "   响应: $LOCKDOWN_RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(f'   Action:     {data.get(\"action\")}')
print(f'   Status:     {data.get(\"status\")}')
print(f'   Lock count: {data.get(\"lock_count\")}')
" 2>/dev/null || echo "   $LOCKDOWN_RESPONSE"

# 7. Cancel lockdown
echo ""
echo "7. 取消 lockdown..."
CANCEL_RESPONSE=$(curl -sS -X POST "$API/api/v1/places/building_demo_001/cancel_lockdown?tenant_id=tenant_demo_jakarta" \
  -H "Authorization: Bearer $TOKEN")
echo "   Status: $(echo "$CANCEL_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('status',''))" 2>/dev/null)"

echo ""
echo "=== 演示完成 ==="
echo ""
echo "如果 Gateway Simulator 正在运行，你应该在 simulator 终端看到:"
echo "  >>> UNLOCKING door $LOCK_ID (手动解锁)"
echo "  >>> UNLOCKING door door_jkt_001 (NFC 刷卡自动解锁)"
echo "  >>> LOCKDOWN / CANCELLED ..."
