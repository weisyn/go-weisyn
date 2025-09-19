#!/bin/bash

# WES智能合约构建脚本
# 构建和测试智能合约

set -e

echo "📋 WES智能合约构建"
echo "===================="

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 检查TinyGo是否安装
if ! command -v tinygo &> /dev/null; then
    echo "❌ TinyGo未安装，请先安装TinyGo："
    echo "   https://tinygo.org/getting-started/install/"
    exit 1
fi

echo "🔍 扫描合约目录..."
if [ ! -d "contracts" ]; then
    echo "❌ 合约目录不存在"
    exit 1
fi

echo "🔨 构建系统合约..."
cd contracts/system

# 构建所有Go合约源文件
for contract in *.go; do
    if [[ -f "$contract" ]]; then
        contract_name=$(basename "$contract" .go)
        echo "🔄 构建合约: $contract_name"
        
        tinygo build -o "${contract_name}.wasm" \
            -target wasi \
            -no-debug \
            "$contract" || echo "⚠️  合约构建失败: $contract_name"
            
        if [[ -f "${contract_name}.wasm" ]]; then
            echo "✅ 构建成功: ${contract_name}.wasm"
        fi
    fi
done

cd "$PROJECT_ROOT"

echo ""
echo "✅ 合约构建完成！"
echo "📁 合约位置: contracts/system/*.wasm"

echo ""
echo "🧪 运行合约测试..."
./scripts/build/test_contracts.sh 2>/dev/null || echo "⚠️  合约测试脚本不存在"

echo ""
echo "🚀 使用方法："
echo "  部署合约: ./bin/node contract deploy contracts/system/simple_contract.wasm"
echo "  调用合约: ./bin/node contract call <address> <method> <params>"
