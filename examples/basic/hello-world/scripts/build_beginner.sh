#!/bin/bash

# ==================== WES智能合约构建助手 ====================
#
# 🎯 这个脚本的作用：
# 将你写的Go代码转换成区块链能理解的WASM格式
#
# 💡 什么是WASM？
# WebAssembly (WASM) 是一种在区块链上运行程序的标准格式
# 就像将中文翻译成英文，让不同的人都能理解

set -e  # 遇到错误立即停止

# 🎨 颜色定义 - 让输出更好看
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 📢 打印带颜色的消息
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_step() {
    echo -e "${BLUE}🔸 $1${NC}"
}

# 🎊 欢迎信息
echo -e "${BLUE}"
echo "======================================"
echo "🔨 WES智能合约构建助手"
echo "======================================"
echo -e "${NC}"
print_info "这个脚本将帮助您编译Go代码为区块链合约"
echo ""

# 📍 步骤1: 检查操作系统
print_step "检查您的操作系统..."
OS=$(uname -s)
case $OS in
    Darwin*)
        OS_NAME="macOS"
        INSTALL_CMD="brew install tinygo"
        ;;
    Linux*)
        OS_NAME="Linux"
        INSTALL_CMD="详见: https://tinygo.org/getting-started/install/linux/"
        ;;
    MINGW*|CYGWIN*|MSYS*)
        OS_NAME="Windows"
        INSTALL_CMD="详见: https://tinygo.org/getting-started/install/windows/"
        ;;
    *)
        OS_NAME="未知系统"
        INSTALL_CMD="请访问 https://tinygo.org/"
        ;;
esac
print_info "检测到系统: $OS_NAME"

# 📍 步骤2: 确保在正确的目录
print_step "定位到项目目录..."
cd "$(dirname "$0")/.."
print_info "当前工作目录: $(pwd)"

# 📍 步骤3: 检查TinyGo是否安装
print_step "检查TinyGo编译器..."
if ! command -v tinygo &> /dev/null; then
    print_error "TinyGo编译器未安装!"
    echo ""
    print_info "TinyGo是将Go代码编译为WASM的专用编译器"
    print_info "就像需要烤箱才能烤面包一样，我们需要TinyGo来编译合约"
    echo ""
    print_info "安装方法 ($OS_NAME):"
    echo "  $INSTALL_CMD"
    echo ""
    print_info "安装完成后，请重新运行此脚本"
    exit 1
else
    TINYGO_VERSION=$(tinygo version 2>/dev/null || echo "版本信息获取失败")
    print_success "TinyGo已安装: $TINYGO_VERSION"
fi

# 📍 步骤4: 检查源代码文件
print_step "检查源代码文件..."
if [ ! -f "src/hello_world.go" ]; then
    print_error "找不到源代码文件: src/hello_world.go"
    print_info "请确保文件存在且路径正确"
    print_info "当前目录: $(pwd)"
    print_info "期望的文件结构:"
    echo "  $(basename $(pwd))/"
    echo "  ├── src/"
    echo "  │   └── hello_world.go"
    echo "  └── scripts/"
    echo "      └── build.sh (当前脚本)"
    exit 1
else
    FILE_SIZE=$(wc -l < "src/hello_world.go")
    print_success "源代码文件存在 ($FILE_SIZE 行代码)"
fi

# 📍 步骤5: 创建输出目录
print_step "准备输出目录..."
mkdir -p build
print_success "输出目录准备完成: build/"

# 📍 步骤6: 开始编译
print_step "开始编译智能合约..."
print_info "将 Go 代码编译为 WASM 格式..."
echo ""

# 显示编译命令 - 让用户知道底层发生了什么
echo "🔧 执行编译命令:"
echo "  tinygo build -o build/hello_world.wasm -target wasm src/hello_world.go"
echo ""

# 执行编译
if tinygo build -o build/hello_world.wasm -target wasm src/hello_world.go; then
    echo ""
    print_success "编译成功! 🎉"
    
    # 📊 显示编译结果信息
    if [ -f "build/hello_world.wasm" ]; then
        # 获取文件大小 - 兼容不同操作系统
        if [[ "$OS" == "Darwin"* ]]; then
            FILE_SIZE=$(stat -f%z build/hello_world.wasm)
        elif [[ "$OS" == "Linux"* ]]; then
            FILE_SIZE=$(stat -c%s build/hello_world.wasm)
        else
            FILE_SIZE=$(wc -c < build/hello_world.wasm 2>/dev/null || echo "未知")
        fi
        
        print_info "编译输出信息:"
        echo "  📁 输出文件: build/hello_world.wasm"
        echo "  📏 文件大小: $FILE_SIZE bytes"
        echo "  🎯 目标格式: WebAssembly (WASM)"
        
        echo ""
        print_success "您的智能合约编译完成!"
        echo ""
        print_info "🚀 下一步操作:"
        echo "  1. 部署合约: ./scripts/deploy.sh"
        echo "  2. 测试合约: ./scripts/interact.sh"
        echo "  3. 查看合约: ls -la build/"
        
        echo ""
        print_info "💡 小贴士:"
        echo "  WASM文件就是您的智能合约，可以部署到WES区块链上"
        echo "  任何人都可以调用您的合约中的函数"
    fi
else
    echo ""
    print_error "编译失败! 😞"
    echo ""
    print_info "🔍 可能的解决方案:"
    echo "  1. 检查Go代码语法是否正确"
    echo "  2. 确保导入的包路径正确"
    echo "  3. 检查TinyGo版本是否兼容"
    echo "  4. 查看上方的错误信息获取详细原因"
    echo ""
    print_info "🆘 需要帮助？"
    echo "  查看故障排除指南: CONCEPTS.md"
    echo "  查看完整文档: BEGINNER_README.md"
    exit 1
fi
