#!/bin/bash

# WES 全部合约构建脚本

set -e

echo "🚀 构建WES全部智能合约..."
echo ""

# 构建统计
TOTAL_CONTRACTS=3
BUILT_CONTRACTS=0
FAILED_CONTRACTS=0

# 构建Token合约
echo "📊 [1/3] 构建Token合约..."
cd contracts/token
if ./build.sh > /dev/null 2>&1; then
    echo "✅ Token合约构建成功"
    BUILT_CONTRACTS=$((BUILT_CONTRACTS + 1))
else
    echo "❌ Token合约构建失败"
    FAILED_CONTRACTS=$((FAILED_CONTRACTS + 1))
fi
cd ../../

echo ""

# 构建RWA合约
echo "🏠 [2/3] 构建RWA合约..."
cd contracts/rwa
if ./build.sh > /dev/null 2>&1; then
    echo "✅ RWA合约构建成功"
    BUILT_CONTRACTS=$((BUILT_CONTRACTS + 1))
else
    echo "❌ RWA合约构建失败"
    FAILED_CONTRACTS=$((FAILED_CONTRACTS + 1))
fi
cd ../../

echo ""

# 构建NFT合约
echo "🎨 [3/3] 构建NFT合约..."
cd contracts/nft
if ./build.sh > /dev/null 2>&1; then
    echo "✅ NFT合约构建成功"
    BUILT_CONTRACTS=$((BUILT_CONTRACTS + 1))
else
    echo "❌ NFT合约构建失败"
    FAILED_CONTRACTS=$((FAILED_CONTRACTS + 1))
fi
cd ../../

echo ""
echo "========================================"
echo "🎉 构建完成统计报告"
echo "========================================"
echo "📊 总合约数量: $TOTAL_CONTRACTS"
echo "✅ 成功构建: $BUILT_CONTRACTS"
echo "❌ 构建失败: $FAILED_CONTRACTS"
echo ""

if [ $FAILED_CONTRACTS -eq 0 ]; then
    echo "🎊 所有合约构建成功！"
    echo ""
    echo "📁 生成的文件："
    echo "   • contracts/token/build/weisyn_token.wasm"
    echo "   • contracts/rwa/build/real_world_asset.wasm"
    echo "   • contracts/nft/build/non_fungible_token.wasm"
    echo ""
    echo "📋 合约信息："
    ls -la contracts/*/build/*.wasm | while read -r line; do
        size=$(echo $line | awk '{print $5}')
        file=$(echo $line | awk '{print $9}')
        echo "   • $file ($size 字节)"
    done
    echo ""
    echo "🚀 下一步："
    echo "   1. 启动WES节点: go run cmd/node/main.go --config configs/config.json"
    echo "   2. 部署合约到区块链"
    echo "   3. 开始构建你的Web3应用！"
else
    echo "⚠️  部分合约构建失败，请检查错误信息"
    exit 1
fi
