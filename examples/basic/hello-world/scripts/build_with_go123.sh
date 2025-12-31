#!/bin/bash

echo "🔨 构建 Hello World 合约（使用 Go 1.24.2）..."

# 确保在正确的目录
cd "$(dirname "$0")/.."

# 创建输出目录
mkdir -p build

# 设置环境变量
export PATH="/Users/qinglong/go/bin:/Users/qinglong/tinygo/bin:$PATH"
export GOTOOLCHAIN=local

# 创建临时的 go wrapper
TEMP_GO_WRAPPER="/tmp/go_wrapper_$$"
cat > "$TEMP_GO_WRAPPER" << 'EOF'
#!/bin/bash
exec /Users/qinglong/go/bin/go1.24.2 "$@"
EOF
chmod +x "$TEMP_GO_WRAPPER"

# 临时替换 PATH 中的 go 命令
export PATH="/tmp:$PATH"
ln -sf "$TEMP_GO_WRAPPER" /tmp/go

echo "📋 环境信息:"
go version
/Users/qinglong/tinygo/bin/tinygo version
echo ""

# 使用 TinyGo 编译为 WASM
echo "📦 编译合约代码..."
/Users/qinglong/tinygo/bin/tinygo build -o build/hello_world.wasm -target wasi src/hello_world.go

if [ $? -eq 0 ]; then
    echo "✅ 构建成功: build/hello_world.wasm"
    echo "📊 文件大小: $(wc -c < build/hello_world.wasm) bytes"
    ls -lh build/hello_world.wasm
else
    echo "❌ 构建失败"
    # 清理临时文件
    rm -f /tmp/go "$TEMP_GO_WRAPPER"
    exit 1
fi

# 清理临时文件
rm -f /tmp/go "$TEMP_GO_WRAPPER"

echo ""
echo "🎉 构建完成！"

