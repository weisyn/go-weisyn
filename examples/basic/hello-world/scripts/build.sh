#!/bin/bash

echo "🔨 构建 Hello World 合约..."

# 确保在正确的目录
cd "$(dirname "$0")/.."

# 创建输出目录
mkdir -p build

# 检查 TinyGo 是否安装
if ! command -v tinygo &> /dev/null; then
    echo "❌ TinyGo 未安装，请先安装 TinyGo"
    echo "   macOS: brew install tinygo"
    echo "   其他: https://tinygo.org/getting-started/install/"
    exit 1
fi

# 使用 TinyGo 编译为 WASM
echo "📦 编译合约代码..."
tinygo build -o build/hello_world.wasm -target wasi src/hello_world.go

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
