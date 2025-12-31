#!/bin/bash

# 🎮 WES质押应用演示脚本

echo "🚀 启动WES质押应用演示"
echo "========================"
echo ""

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ 错误: 未找到Go环境，请先安装Go"
    exit 1
fi

# 切换到示例目录
cd "$(dirname "$0")/.."

echo "📁 当前目录: $(pwd)"
echo ""

# 检查文件是否存在
if [ ! -f "staking_example.go" ]; then
    echo "❌ 错误: 未找到 staking_example.go 文件"
    exit 1
fi

echo "✅ 文件检查通过"
echo ""

# 运行演示
echo "🎯 运行质押演示..."
echo "=================="
echo ""

go run staking_example.go

echo ""
echo "🎉 演示完成！"
echo ""
echo "📚 学习提示:"
echo "• 查看源码: cat staking_example.go"
echo "• 了解更多: 参考 README.md"
echo "• 实际部署: 需要连接真实的WES网络"
