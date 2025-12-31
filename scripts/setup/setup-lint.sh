#!/bin/bash
# WES 项目 golangci-lint 快速安装和使用脚本

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "🔍 WES 项目代码检查工具设置"
echo "================================"
echo ""

# 检查 golangci-lint 是否已安装
if command -v golangci-lint >/dev/null 2>&1; then
    echo "✅ golangci-lint 已安装"
    golangci-lint --version
    echo ""
    INSTALLED=true
else
    echo "❌ golangci-lint 未安装"
    echo ""
    INSTALLED=false
    
    # 尝试安装
    if command -v brew >/dev/null 2>&1; then
        echo "📦 检测到 Homebrew，使用 Homebrew 安装..."
        echo "   运行: brew install golangci-lint"
        echo ""
        read -p "是否现在安装？(y/n) " -n 1 -r
        echo ""
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            brew install golangci-lint
            INSTALLED=true
        fi
    else
        echo "📦 安装选项："
        echo "   1. Homebrew: brew install golangci-lint"
        echo "   2. 官方脚本: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$(go env GOPATH)/bin latest"
        echo "   3. Go 安装: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
        echo ""
    fi
fi

# 如果已安装，运行检查
if [ "$INSTALLED" = true ]; then
    echo ""
    echo "🚀 开始运行代码检查..."
    echo "================================"
    echo ""
    
    cd "$PROJECT_ROOT"
    
    # 运行检查
    golangci-lint run --timeout=5m
    
    echo ""
    echo "✅ 检查完成！"
    echo ""
    echo "💡 提示："
    echo "   - 运行 'make lint' 进行代码检查"
    echo "   - 运行 'make lint-fix' 自动修复可修复的问题"
    echo "   - 查看 docs/GOLANGCI_LINT_USAGE.md 了解更多用法"
else
    echo ""
    echo "⚠️  请先安装 golangci-lint，然后重新运行此脚本"
    echo "   或直接运行: make lint"
fi

