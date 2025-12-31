#!/bin/bash

# WES 测试环境清理脚本 - 纯净模式
# 清理所有测试数据，提供全新的测试环境

set -e

echo "🧹 WES 测试环境清理 - 纯净模式"
echo "=================================="

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$PROJECT_ROOT"

# 确认操作
read -p "⚠️ 这将删除所有测试数据，是否继续？(y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "操作已取消"
    exit 0
fi

echo "📋 开始清理测试数据..."

# 停止所有可能运行的节点
echo "🛑 停止运行中的节点..."
pkill -f "bin/node" 2>/dev/null || true
sleep 2

# 清理数据目录
echo "🗑️ 清理数据目录..."
rm -rf data/badger* || true
rm -rf data/logs/* || true
rm -rf data/p2p/* || true
rm -rf data_node2/ || true
rm -rf data/dht* || true

# 清理临时文件
echo "🧽 清理临时文件..."
rm -f *.log || true
rm -f *.pid || true
rm -f node.log || true
rm -f /tmp/weisyn_* || true

# 清理测试生成的文件
echo "📂 清理测试生成文件..."
rm -rf test_data/ || true
rm -rf tmp_test/ || true
rm -f test/reports/*.html || true

# 清理构建产物
echo "🔨 清理构建产物..."
rm -f bin/node || true
rm -f bin/cli || true
rm -f bin/explorer || true

echo ""
echo "✅ 环境清理完成！"
echo ""
echo "📋 清理内容："
echo "  - 区块链数据库"
echo "  - 节点日志文件"
echo "  - P2P网络数据" 
echo "  - DHT存储数据"
echo "  - 临时文件"
echo "  - 测试报告"
echo "  - 构建产物"
echo ""
echo "🚀 现在可以开始全新的测试了！"
echo ""
echo "下一步操作："
echo "  1. 构建项目: ./scripts/build.sh"
echo "  2. 运行测试: ./test/scripts/automation/run-e2e-tests.sh"
echo "  3. 启动节点: ./bin/node --config configs_new/environments/local/single-node.json"
