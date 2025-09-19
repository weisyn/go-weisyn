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
if ! curl -s http://localhost:8080/api/v1/info > /dev/null; then
    echo "❌ WES 节点未运行或无法连接"
    exit 1
fi

echo ""
echo "🎯 开始交互演示..."

# 1. 调用 SayHello 函数
echo ""
echo "📞 1. 调用 SayHello 函数..."
curl -s -X POST http://localhost:8080/api/v1/contract/call \
    -H "Content-Type: application/json" \
    -d '{
        "contract_address": "'$CONTRACT_ADDRESS'",
        "function_name": "SayHello",
        "params": {},
        "caller": "CUser123456789",
        "fee_limit": 100000
    }' | jq .

echo ""
read -p "按回车继续..."

# 2. 查询 GetGreeting 函数
echo ""
echo "🔍 2. 查询 GetGreeting 函数..."
curl -s -X POST http://localhost:8080/api/v1/contract/query \
    -H "Content-Type: application/json" \
    -d '{
        "contract_address": "'$CONTRACT_ADDRESS'",
        "function_name": "GetGreeting",
        "params": {}
    }' | jq .

echo ""
read -p "按回车继续..."

# 3. 设置自定义消息
echo ""
echo "📝 3. 设置自定义消息..."
curl -s -X POST http://localhost:8080/api/v1/contract/call \
    -H "Content-Type: application/json" \
    -d '{
        "contract_address": "'$CONTRACT_ADDRESS'",
        "function_name": "SetMessage",
        "params": {
            "message": "Hello from WES Example!"
        },
        "caller": "CUser123456789",
        "fee_limit": 100000
    }' | jq .

echo ""
read -p "按回车继续..."

# 4. 获取自定义消息
echo ""
echo "📖 4. 获取自定义消息..."
curl -s -X POST http://localhost:8080/api/v1/contract/query \
    -H "Content-Type: application/json" \
    -d '{
        "contract_address": "'$CONTRACT_ADDRESS'",
        "function_name": "GetMessage",
        "params": {}
    }' | jq .

echo ""
read -p "按回车继续..."

# 5. 获取合约信息
echo ""
echo "ℹ️  5. 获取合约信息..."
curl -s -X POST http://localhost:8080/api/v1/contract/query \
    -H "Content-Type: application/json" \
    -d '{
        "contract_address": "'$CONTRACT_ADDRESS'",
        "function_name": "GetContractInfo",
        "params": {}
    }' | jq .

echo ""
echo "🎉 Hello World 合约交互演示完成！"
echo ""
echo "📚 接下来可以学习:"
echo "   - simple-examples/token-transfer/  # 代币转账示例"
echo "   - simple-examples/nft-minting/     # NFT 铸造示例"
echo "   - contracts/staking/               # 质押合约示例"
