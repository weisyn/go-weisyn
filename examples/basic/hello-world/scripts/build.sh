#!/bin/bash

# ==================== WES Hello World 合约构建脚本 ====================
#
# 🎯 功能：将 Go 合约代码编译为 WebAssembly (WASM) 格式
#
# 📋 环境要求：
#   - TinyGo 0.34.0（brew install tinygo 或访问 https://tinygo.org）
#   - Go 1.19 ~ 1.23（本合约使用 Go 1.23，独立于主项目的 Go 1.25）
#
# ⚠️ 重要说明：
#   - 本合约使用独立的 go.mod（Go 1.23）以兼容 TinyGo 0.34.0
#   - 如果系统 Go 版本是 1.25，TinyGo 会报错，需安装 Go 1.23
#   - 推荐路径：~/go/bin/go1.23.4 或 /usr/local/go1.23
#
# 🔧 使用方法：
#   bash scripts/build.sh
#   或：TINYGO_PATH=/path/to/tinygo bash scripts/build.sh
#
# ==================== 脚本开始 ====================

echo "🔨 构建 Hello World 合约..."

# 确保在正确的目录
cd "$(dirname "$0")/.."

# 创建输出目录
mkdir -p build

# 检查 TinyGo 是否安装（支持自定义路径）
TINYGO_CMD="${TINYGO_PATH:-tinygo}"
if ! command -v $TINYGO_CMD &> /dev/null; then
    echo "❌ TinyGo 未找到，请先安装 TinyGo"
    echo ""
    echo "📥 安装方法："
    echo "   macOS:   brew install tinygo"
    echo "   Linux:   https://tinygo.org/getting-started/install/linux/"
    echo "   Windows: https://tinygo.org/getting-started/install/windows/"
    echo ""
    echo "💡 或者设置 TINYGO_PATH 环境变量指向 tinygo 可执行文件"
    echo "   export TINYGO_PATH=/path/to/tinygo"
    exit 1
fi

# 检查 Go 版本兼容性
GO_VERSION=$(go version 2>/dev/null | awk '{print $3}' | sed 's/go//' || echo "未知")
TINYGO_VERSION=$($TINYGO_CMD version 2>/dev/null | awk '{print $3}' || echo "未知")

echo "📋 环境信息:"
echo "   系统 Go 版本: $GO_VERSION"
echo "   TinyGo 版本: $TINYGO_VERSION"
echo "   TinyGo 路径: $(command -v $TINYGO_CMD)"
echo ""

# 检查 Go 版本是否兼容（1.19~1.23）
MAJOR_VERSION=$(echo "$GO_VERSION" | cut -d. -f1)
MINOR_VERSION=$(echo "$GO_VERSION" | cut -d. -f2)

if [[ "$MAJOR_VERSION" == "1" ]] && [[ "$MINOR_VERSION" -gt 23 ]]; then
    echo "⚠️  警告：系统 Go 版本 ($GO_VERSION) 高于 TinyGo 支持的最高版本 (1.23)"
    echo ""
    echo "🔧 解决方案："
    echo "   1. 安装 Go 1.23："
    echo "      go install golang.org/dl/go1.23.4@latest"
    echo "      ~/go/bin/go1.23.4 download"
    echo ""
    echo "   2. 创建临时 wrapper（高级用户）："
    echo "      export PATH=\"~/go/bin:\$PATH\""
    echo "      ln -sf ~/go/bin/go1.23.4 /tmp/go"
    echo "      export PATH=\"/tmp:\$PATH\""
    echo ""
    echo "   3. 如果 TinyGo 仍能正常工作，可忽略此警告"
    echo ""
fi

# 设置环境变量以使用本地 go.mod
export GOTOOLCHAIN=local

# 检查是否需要wasm-opt
if ! command -v wasm-opt &> /dev/null; then
    echo "⚠️  wasm-opt 未找到，编译可能会失败"
    echo "   建议安装: brew install binaryen"
    echo "   或手动下载: https://github.com/WebAssembly/binaryen/releases"
    echo ""
fi

# 使用 TinyGo 编译为 WASM
echo "📦 编译合约代码..."
tinygo build -o build/hello_world.wasm -target wasi -tags tinygo src/hello_world.go

if [ $? -eq 0 ]; then
    echo "✅ 构建成功: build/hello_world.wasm"
    echo "📊 文件大小: $(wc -c < build/hello_world.wasm) bytes"
    
    # 验证 WASM 文件
    if command -v wasm-validate &> /dev/null; then
        wasm-validate build/hello_world.wasm
        if [ $? -eq 0 ]; then
            echo "✅ WASM 文件验证通过"
        else
            echo "⚠️  WASM 文件验证失败，但可能仍可使用"
        fi
    fi
else
    echo "❌ 构建失败"
    exit 1
fi

echo ""
echo "🎉 构建完成！接下来可以运行:"
echo "   ./scripts/deploy.sh  # 部署合约"
echo "   ./scripts/interact.sh # 与合约交互"
