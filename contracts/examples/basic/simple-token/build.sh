#!/bin/bash

# Simple Token 合约编译脚本
# 将 Go 合约编译为 WASM 文件

set -e

echo "🔨 编译 Simple Token 合约..."

# 检查 TinyGo 是否安装
if ! command -v tinygo &> /dev/null; then
    echo "❌ TinyGo 未安装，请先安装 TinyGo："
    echo "   https://tinygo.org/getting-started/install/"
    exit 1
fi

# 编译合约
tinygo build -o simple-token.wasm \
    -target=wasi \
    -scheduler=none \
    -no-debug \
    -opt=2 \
    ./src/main.go

if [ -f "simple-token.wasm" ]; then
    echo "✅ 编译成功！"
    echo "📁 输出文件: simple-token.wasm"
    ls -lh simple-token.wasm
else
    echo "❌ 编译失败"
    exit 1
fi

