#!/bin/bash

# WES项目构建脚本
# 构建所有主要二进制文件

set -e

echo "🔨 WES项目构建"
echo "=================="

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 创建bin目录
mkdir -p bin

echo "📦 构建WES节点程序..."
go build -o ./bin/node ./cmd/node/
echo "✅ 节点程序构建完成: ./bin/node"

echo "💻 构建CLI工具..."
go build -o ./bin/cli ./cmd/cli/ 2>/dev/null || echo "⚠️  CLI工具暂未实现"

echo "🔍 构建区块链浏览器..."
go build -o ./bin/explorer ./cmd/explorer/ 2>/dev/null || echo "⚠️  浏览器暂未实现"

echo ""
echo "✅ 构建完成！"
echo "📋 可用的二进制文件："
ls -la bin/ 2>/dev/null || echo "无二进制文件"

echo ""
echo "🚀 使用方法："
echo "  启动节点: ./bin/node"
echo "  查看帮助: ./bin/node --help"
