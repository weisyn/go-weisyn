#!/bin/bash

echo "🚀 部署 Hello World 合约..."

# 确保在正确的目录
cd "$(dirname "$0")/.."

# 检查 WASM 文件是否存在
WASM_FILE="build/hello_world.wasm"
if [ ! -f "$WASM_FILE" ]; then
    echo "❌ 找不到 WASM 文件，请先运行 build.sh"
    echo "   ./scripts/build.sh"
    exit 1
fi

# 检查节点是否运行
if ! curl -s http://localhost:28680/api/v1/info > /dev/null; then
    echo "❌ WES 节点未运行或无法连接"
    echo "   请确保节点在 localhost:28680 运行"
    echo "   启动命令: ./bin/testing"
    exit 1
fi

echo "📁 准备部署文件..."
# 获取 WASM 文件的绝对路径
WASM_PATH=$(cd "$(dirname "$WASM_FILE")" && pwd)/$(basename "$WASM_FILE")
echo "✅ WASM 文件路径: $WASM_PATH"

# 演示用私钥（实际使用时请替换为真实私钥）
DEPLOYER_PRIVATE_KEY="0000000000000000000000000000000000000000000000000000000000000001"

echo "📤 发送部署请求..."
# 部署合约
RESPONSE=$(curl -s -X POST http://localhost:28680/api/v1/contract/deploy \
    -H "Content-Type: application/json" \
    -d '{
        "deployer_private_key": "'$DEPLOYER_PRIVATE_KEY'",
        "contract_file_path": "'$WASM_PATH'",
        "config": {
            "abi_version": "v1",
            "exported_functions": ["SayHello", "GetGreeting", "SetMessage", "GetMessage", "GetContractInfo"]
        },
        "name": "Hello World Contract",
        "description": "WES区块链的第一个入门示例合约"
    }')

echo "$RESPONSE" | jq .

# 检查部署是否成功
SUCCESS=$(echo "$RESPONSE" | jq -r '.success')
CONTENT_HASH=$(echo "$RESPONSE" | jq -r '.data.content_hash // empty')

if [ "$SUCCESS" = "true" ] && [ -n "$CONTENT_HASH" ]; then
    echo "✅ 合约部署交易构建成功！"
    echo "📋 内容哈希（合约地址）: $CONTENT_HASH"
    echo "📏 合约大小: $(echo "$RESPONSE" | jq -r '.data.code_size') bytes"
    echo "📋 交易哈希: $(echo "$RESPONSE" | jq -r '.data.transaction_hash')"
    
    # 保存交易信息
    mkdir -p config
    echo "$CONTENT_HASH" > config/contract_address.txt
    echo "$RESPONSE" > config/deploy_response.json
    echo "💾 部署信息已保存到 config/ 目录"
    
    echo ""
    echo "🎉 合约部署交易已构建完成！"
    echo "📝 说明：WES使用两阶段部署模式"
    echo "   1. ✅ 构建部署交易（已完成）"
    echo "   2. 🔄 签名并提交交易（API会自动处理）"
    echo ""
    echo "🔗 下一步："
    echo "   使用 ./scripts/interact.sh 与合约交互"
    echo "   合约地址（content_hash）已保存到 config/contract_address.txt"
else
    echo "❌ 部署失败，请检查错误信息"
    exit 1
fi
