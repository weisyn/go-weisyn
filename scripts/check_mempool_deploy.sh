#!/bin/bash

# 检查内存池中的部署交易
# 用法: ./scripts/check_mempool_deploy.sh

set -e

NODE_URL="${NODE_URL:-http://localhost:28680}"

echo "🔍 检查内存池中的部署交易..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# 1. 查询交易池状态
echo ""
echo "📊 交易池状态:"
STATUS=$(curl -s "${NODE_URL}/api/v1/txpool/status" | jq .)
echo "$STATUS" | jq .

# 2. 查询交易池内容
echo ""
echo "📋 交易池内容:"
CONTENT=$(curl -s "${NODE_URL}/api/v1/txpool/content" | jq .)
echo "$CONTENT" | jq .

# 3. 检查是否有部署交易
DEPLOY_COUNT=$(echo "$CONTENT" | jq -r '.deploy_count // 0')
TOTAL=$(echo "$CONTENT" | jq -r '.total // 0')

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$DEPLOY_COUNT" -gt 0 ]; then
    echo "✅ 发现 $DEPLOY_COUNT 个部署交易（总共 $TOTAL 个待处理交易）"
    echo ""
    echo "📦 部署交易详情:"
    echo "$CONTENT" | jq -r '.deploy_txs[] | "  - 交易哈希: \(.tx_hash)\n    类型: \(.type)\n    输入数: \(.numInputs)\n    输出数: \(.numOutputs)"'
else
    if [ "$TOTAL" -gt 0 ]; then
        echo "ℹ️  内存池中有 $TOTAL 个待处理交易，但没有部署交易"
    else
        echo "ℹ️  内存池当前为空，没有待处理交易"
    fi
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

