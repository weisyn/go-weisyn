#!/bin/bash

# Hello World 合约编译脚本
# 将 Go 合约编译为 WASM 文件

set -e

echo "🔨 编译 Hello World 合约..."

# 检查 TinyGo 是否安装
if ! command -v tinygo &> /dev/null; then
    echo "❌ TinyGo 未安装，请先安装 TinyGo："
    echo "   https://tinygo.org/getting-started/install/"
    exit 1
fi

# 检查 main.go 文件是否存在
if [ ! -f "main.go" ]; then
    echo "❌ main.go 文件不存在"
    exit 1
fi

# 检查 go.mod 文件是否存在
if [ -f "go.mod" ]; then
    echo "📦 检查并下载依赖..."
    # 使用 go mod download 下载依赖（如果 go 命令可用）
    if command -v go &> /dev/null; then
        # 设置 GOSUMDB 以避免校验和数据库问题
        export GOSUMDB=${GOSUMDB:-sum.golang.org}
        go mod download 2>/dev/null || echo "⚠️  依赖下载失败，但将继续编译..."
    else
        echo "⚠️  Go 命令未找到，跳过依赖检查"
    fi
else
    echo "⚠️  go.mod 文件不存在，跳过依赖检查"
fi

# 设置 GOSUMDB 环境变量（如果未设置）
export GOSUMDB=${GOSUMDB:-sum.golang.org}

# 编译合约
echo "🔧 开始编译..."
tinygo build -o hello-world.wasm \
    -target=wasi \
    -scheduler=none \
    -no-debug \
    -opt=2 \
    main.go

if [ -f "hello-world.wasm" ]; then
    echo "✅ 编译成功！"
    echo "📁 输出文件: hello-world.wasm"
    ls -lh hello-world.wasm
    
    # 验证 WASM 文件（如果 wasm-validate 可用）
    if command -v wasm-validate &> /dev/null; then
        echo "🔍 验证 WASM 文件..."
        if wasm-validate hello-world.wasm 2>/dev/null; then
            echo "✅ WASM 文件验证通过"
        else
            echo "⚠️  WASM 文件验证失败，但文件已生成"
        fi
    else
        echo "💡 提示: 安装 wasm-validate 可以验证 WASM 文件"
        echo "   macOS: brew install binaryen"
    fi
else
    echo "❌ 编译失败"
    exit 1
fi
