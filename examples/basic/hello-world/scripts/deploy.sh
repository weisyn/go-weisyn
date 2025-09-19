#!/bin/bash

echo "🚀 部署 Hello World 合约..."

# 确保在正确的目录
cd "$(dirname "$0")/.."

# 检查 WASM 文件是否存在
if [ ! -f "build/hello_world.wasm" ]; then
    echo "❌ 找不到 WASM 文件，请先运行 build.sh"
    echo "   ./scripts/build.sh"
    exit 1
fi

# 检查节点是否运行
if ! curl -s http://localhost:8080/api/v1/info > /dev/null; then
    echo "❌ WES 节点未运行或无法连接"
    echo "   请确保节点在 localhost:8080 运行"
    echo "   启动命令: ./bin/node --config configs/environments/local/single-node.json"
    exit 1
fi

echo "📁 读取 WASM 文件..."
# 读取 WASM 文件并转换为 hex
WASM_HEX=$(hexdump -ve '1/1 "%.2x"' build/hello_world.wasm)
echo "✅ WASM 文件已读取 (${#WASM_HEX} 字符)"

echo "📤 发送部署请求..."
# 部署合约
RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/contract/deploy \
    -H "Content-Type: application/json" \
    -d '{
        "wasm_code": "'$WASM_HEX'",
        "owner": "CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR",
        "owner_public_key": "02349cb6a770701494eb716d0b430ebcff740a354b2ceaedb4d3a2b4bad2237896",
        "init_params": "",
        "fee_limit": 1000000
    }')

echo "$RESPONSE" | jq .

# 检查部署是否成功
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
TX_HASH=$(echo "$RESPONSE" | jq -r '.data.transaction_hash // empty')

if [ "$SUCCESS" = "true" ] && [ -n "$TX_HASH" ]; then
    echo "✅ 合约部署交易构建成功！"
    echo "📋 交易哈希: $TX_HASH"
    echo "📋 内容哈希: $(echo "$RESPONSE" | jq -r '.data.content_hash')"
    echo "📏 合约大小: $(echo "$RESPONSE" | jq -r '.data.code_size') bytes"
    
    # 保存交易信息
    mkdir -p config
    echo "$TX_HASH" > config/deploy_tx_hash.txt
    echo "$RESPONSE" > config/deploy_response.json
    echo "💾 部署信息已保存"
    
    echo ""
    echo "🎉 合约部署交易已准备就绪！"
    echo "📝 说明：WES使用两阶段部署模式"
    echo "   1. ✅ 构建部署交易（已完成）"
    echo "   2. 🔄 签名并提交交易（需要签名服务）"
    echo ""
    echo "🔗 下一步："
    echo "   在生产环境中，交易会自动签名并提交到区块链"
    echo "   在演示环境中，我们可以继续测试其他功能"
else
    echo "❌ 部署失败，请检查错误信息"
    exit 1
fi
