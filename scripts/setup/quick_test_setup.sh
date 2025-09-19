#!/bin/bash

# 🎯WES 快速测试设置脚本
# 用于快速启动端到端测试环境

set -e  # 遇到错误立即退出

echo "🎯WES 区块链系统快速测试设置"
echo "=================================="

# 检查是否在正确的目录
if [ ! -f "go.mod" ] || [ ! -d "cmd/node" ]; then
    echo "❌ 错误: 请在项目根目录运行此脚本"
    echo "   当前目录: $(pwd)"
    echo "   期望包含: go.mod, cmd/node/"
    exit 1
fi

echo "✅ 项目目录检查通过"

# Phase 1.1: 环境清理
echo ""
echo "🔧 Phase 1.1: 清理测试环境"
echo "--------------------------"

# 清理数据目录
if [ -d "./badger_data" ]; then
    rm -rf ./badger_data/*
    echo "✅ 清理 BadgerDB 数据"
else
    mkdir -p ./badger_data
    echo "✅ 创建 BadgerDB 数据目录"
fi

# 清理索引
if [ -d "./index" ]; then
    rm -rf ./index/*
    echo "✅ 清理索引数据"
else
    mkdir -p ./index
    echo "✅ 创建索引目录"
fi

# 清理日志
if [ -f "./node.log" ]; then
    rm -f ./node.log
    echo "✅ 清理日志文件"
fi

# 清理PID文件
if [ -f "./node.pid" ]; then
    rm -f ./node.pid
    echo "✅ 清理PID文件"
fi

# Phase 1.2: 编译节点
echo ""
echo "🔧 Phase 1.2: 编译区块链节点"
echo "----------------------------"

echo "📦 编译节点程序..."
if go build -o bin/node cmd/node/main.go; then
    echo "✅ 节点编译成功"
else
    echo "❌ 节点编译失败"
    exit 1
fi

# 检查配置文件
if [ ! -f "config.json" ]; then
    echo "❌ 配置文件 config.json 不存在"
    echo "   请确保配置文件存在并正确配置"
    exit 1
fi
echo "✅ 配置文件检查通过"

# 打开测试文档
echo ""
echo "📋 打开测试跟踪文档"
echo "-------------------"

if command -v code &> /dev/null; then
    echo "📝 使用 VS Code 打开测试文档..."
    code test/END_TO_END_TESTING_PLAN.md
elif command -v open &> /dev/null; then
    echo "📝 使用默认编辑器打开测试文档..."
    open test/END_TO_END_TESTING_PLAN.md
else
    echo "📝 请手动打开测试文档:"
    echo "   test/END_TO_END_TESTING_PLAN.md"
fi

echo ""
echo "🎉 环境设置完成！"
echo "================"
echo ""
echo "📋 下一步操作:"
echo "1. 启动节点: ./bin/node --config config.json"
echo "2. 启动挖矿: curl -X POST http://localhost:8080/api/mining/start \\"
echo "               -H \"Content-Type: application/json\" \\"
echo "               -d '{\"miner_address\": \"0x1234567890abcdef1234567890abcdef12345678\"}'"
echo "3. 按照测试文档 test/END_TO_END_TESTING_PLAN.md 进行完整测试"
echo ""
echo "🎯 重要提示:"
echo "- 测试文档已打开，请按照7个阶段依次执行"
echo "- 记得在每个验证清单中勾选完成状态"
echo "- 遇到问题及时记录到问题追踪表"
echo ""
echo "✨ 祝测试顺利！验证我们的代码质量提升效果！" 