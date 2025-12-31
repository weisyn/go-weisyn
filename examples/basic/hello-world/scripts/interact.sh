#!/bin/bash

echo "🎮 与 Hello World 合约交互..."

# 确保在正确的目录
cd "$(dirname "$0")/.."

# 读取合约地址
if [ -f "config/contract_address.txt" ]; then
    CONTRACT_ADDRESS=$(cat config/contract_address.txt)
    echo "📋 使用合约地址: $CONTRACT_ADDRESS"
else
    echo "❌ 找不到合约地址文件，请先部署合约"
    echo "   ./scripts/deploy.sh"
    exit 1
fi

# 检查节点是否运行
if ! curl -s http://localhost:28680/api/v1/info > /dev/null; then
    echo "❌ WES 节点未运行或无法连接"
    exit 1
fi

# 演示用私钥
CALLER_PRIVATE_KEY="0000000000000000000000000000000000000000000000000000000000000001"

echo ""
echo "🎯 开始交互演示..."

# 1. 调用 SayHello 函数
echo ""
echo "📞 1. 调用 SayHello 函数..."
curl -s -X POST http://localhost:28680/api/v1/contract/call \
    -H "Content-Type: application/json" \
    -d '{
        "caller_private_key": "'$CALLER_PRIVATE_KEY'",
        "contract_address": "'$CONTRACT_ADDRESS'",
        "method_name": "SayHello",
        "parameters": {},
        "execution_fee_limit": 100000
    }' | jq .

echo ""
read -p "按回车继续..."

# 2. 查询 GetGreeting 函数（使用 call 接口）
echo ""
echo "🔍 2. 查询 GetGreeting 函数..."
curl -s -X POST http://localhost:28680/api/v1/contract/call \
    -H "Content-Type: application/json" \
    -d '{
        "caller_private_key": "'$CALLER_PRIVATE_KEY'",
        "contract_address": "'$CONTRACT_ADDRESS'",
        "method_name": "GetGreeting",
        "parameters": {},
        "execution_fee_limit": 50000
    }' | jq .

echo ""
read -p "按回车继续..."

# 3. 设置自定义消息
echo ""
echo "📝 3. 设置自定义消息..."
curl -s -X POST http://localhost:28680/api/v1/contract/call \
    -H "Content-Type: application/json" \
    -d '{
        "caller_private_key": "'$CALLER_PRIVATE_KEY'",
        "contract_address": "'$CONTRACT_ADDRESS'",
        "method_name": "SetMessage",
        "parameters": {
            "message": "Hello from WES Example!"
        },
        "execution_fee_limit": 100000
    }' | jq .

echo ""
read -p "按回车继续..."

# 4. 获取自定义消息
echo ""
echo "📖 4. 获取自定义消息..."
curl -s -X POST http://localhost:28680/api/v1/contract/call \
    -H "Content-Type: application/json" \
    -d '{
        "caller_private_key": "'$CALLER_PRIVATE_KEY'",
        "contract_address": "'$CONTRACT_ADDRESS'",
        "method_name": "GetMessage",
        "parameters": {},
        "execution_fee_limit": 50000
    }' | jq .

echo ""
read -p "按回车继续..."

# 5. 获取合约信息
echo ""
echo "ℹ️  5. 获取合约信息..."
curl -s -X POST http://localhost:28680/api/v1/contract/call \
    -H "Content-Type: application/json" \
    -d '{
        "caller_private_key": "'$CALLER_PRIVATE_KEY'",
        "contract_address": "'$CONTRACT_ADDRESS'",
        "method_name": "GetContractInfo",
        "parameters": {},
        "execution_fee_limit": 50000
    }' | jq .

echo ""
echo "🎉 Hello World 合约交互演示完成！"
echo ""
echo "📚 接下来可以学习:"
echo "   - simple-examples/token-transfer/  # 代币转账示例"
echo "   - simple-examples/nft-minting/     # NFT 铸造示例"
echo "   - contracts/staking/               # 质押合约示例"
