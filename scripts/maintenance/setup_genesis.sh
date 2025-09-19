#!/bin/bash

echo "=== WES 创世配置自动化脚本 ==="
echo ""

# 检查是否在项目根目录
if [ ! -f "config.json" ]; then
    echo "❌ 错误：请在项目根目录运行此脚本"
    exit 1
fi

# 1. 生成新的密钥对
echo "🔐 步骤1: 生成真实的密钥对..."
go run test/generate_genesis_keys.go
if [ $? -ne 0 ]; then
    echo "❌ 密钥生成失败"
    exit 1
fi

echo ""
echo "📋 生成的账户信息:"
jq -r '.[] | "地址: \(.address) | 私钥: \(.private_key)"' test/genesis_keys.json

# 2. 清理环境
echo ""
echo "🧹 步骤2: 清理旧数据..."
rm -rf data/badger/* data/logs/* 2>/dev/null
echo "✅ 环境清理完成"

# 3. 编译节点
echo ""
echo "🔨 步骤3: 编译节点程序..."
go build -o bin/node cmd/node/main.go
if [ $? -ne 0 ]; then
    echo "❌ 编译失败"
    exit 1
fi
echo "✅ 编译完成"

# 4. 启动节点
echo ""
echo "🚀 步骤4: 启动节点..."
echo "节点将在后台启动，日志保存到 node.log"
./bin/node > node.log 2>&1 &
NODE_PID=$!
echo "节点 PID: $NODE_PID"
echo "$NODE_PID" > node.pid

# 5. 等待节点启动
echo ""
echo "⏳ 等待节点启动..."
sleep 8

# 6. 检查节点状态
if kill -0 $NODE_PID 2>/dev/null; then
    echo "✅ 节点启动成功！"
    
    echo ""
    echo "📊 区块链状态:"
    curl -s http://localhost:8089/api/v1/blocks/info | jq .
    
    echo ""
    echo "💰 账户余额 (第一个账户):"
    FIRST_ADDRESS=$(jq -r '.[0].address' test/genesis_keys.json)
    curl -s "http://localhost:8089/api/v1/accounts/$FIRST_ADDRESS/balance" | jq .
    
    echo ""
    echo "🎉 设置完成！"
    echo ""
    echo "📝 使用说明:"
    echo "  - 节点正在后台运行 (PID: $NODE_PID)"
    echo "  - API地址: http://localhost:8089"
    echo "  - 日志文件: node.log"
    echo "  - 私钥文件: test/genesis_keys.json (测试用)"
    echo "  - 停止节点: kill $NODE_PID 或运行 ./scripts/stop_node.sh"
    echo ""
    echo "🔧 API测试命令:"
    echo "  curl http://localhost:8089/api/v1/blocks/info"
    echo "  curl http://localhost:8089/api/v1/accounts/$FIRST_ADDRESS/balance"
    echo "  curl -X POST http://localhost:8089/api/v1/mining/once"
    
else
    echo "❌ 节点启动失败，查看日志:"
    tail -20 node.log
    exit 1
fi 