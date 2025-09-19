#!/bin/bash

# ==================== WES智能合约简化编译工具 ====================
#
# 🎯 工具作用：提供友好的智能合约编译体验，适合初学者使用
# 💡 特点：详细的进度提示、错误诊断、跨平台支持
# 🎨 设计理念：让编译过程透明易懂，遇到问题有清晰的解决方案
#
# 📚 使用方法：
#   ./simple-build.sh [选项]
#   选项：
#     --target <目标>     指定编译目标（默认：wasi）
#     --optimize         启用优化编译
#     --verbose          显示详细编译信息
#     --help             显示帮助信息
#
# ==================== 颜色和样式定义 ====================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m'

# 输出函数
print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${WHITE}$1${NC}"
    echo -e "${BLUE}================================${NC}"
    echo ""
}

print_step() {
    echo -e "${CYAN}📍 $1${NC}"
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

print_info() {
    echo -e "${PURPLE}💡 $1${NC}"
}

print_progress() {
    echo -e "${CYAN}🔸 $1${NC}"
}

# ==================== 帮助信息 ====================

show_help() {
    print_header "🔨 WES智能合约编译工具帮助"
    
    echo -e "${WHITE}使用方法：${NC}"
    echo "  ./simple-build.sh [选项]"
    echo ""
    echo -e "${WHITE}选项：${NC}"
    echo "  --target <目标>    指定编译目标（默认：wasi）"
    echo "  --optimize        启用优化编译"
    echo "  --verbose         显示详细编译信息"
    echo "  --help            显示本帮助信息"
    echo ""
    echo -e "${WHITE}示例：${NC}"
    echo "  ./simple-build.sh                    # 基础编译"
    echo "  ./simple-build.sh --optimize         # 优化编译"
    echo "  ./simple-build.sh --verbose          # 详细信息编译"
    echo ""
    echo -e "${WHITE}支持的目标平台：${NC}"
    echo "  wasi              WebAssembly System Interface (推荐)"
    echo "  wasm              纯WebAssembly"
    echo ""
}

# ==================== 参数解析 ====================

TARGET="wasi"
OPTIMIZE=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --target)
            TARGET="$2"
            shift 2
            ;;
        --optimize)
            OPTIMIZE=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            print_error "未知选项: $1"
            echo "使用 --help 查看帮助信息"
            exit 1
            ;;
    esac
done

# ==================== 编译开始 ====================

clear
print_header "🔨 WES智能合约编译器"

echo -e "${WHITE}这个工具将帮你编译智能合约到WebAssembly格式${NC}"
echo -e "${CYAN}🎯 编译目标: $TARGET${NC}"
if [[ $OPTIMIZE == true ]]; then
    echo -e "${CYAN}⚡ 优化模式: 启用${NC}"
fi
if [[ $VERBOSE == true ]]; then
    echo -e "${CYAN}📊 详细模式: 启用${NC}"
fi
echo ""

# ==================== 环境检查 ====================

print_step "检查编译环境..."

# 检查操作系统
OS=$(uname -s)
ARCH=$(uname -m)
print_info "检测到系统: $OS ($ARCH)"

# 检查TinyGo是否安装
if ! command -v tinygo &> /dev/null; then
    print_error "TinyGo编译器未安装"
    echo ""
    echo -e "${WHITE}📝 安装方法：${NC}"
    
    case $OS in
        "Darwin")
            echo "   brew tap tinygo-org/tools"
            echo "   brew install tinygo"
            ;;
        "Linux")
            echo "   # Ubuntu/Debian:"
            echo "   wget https://github.com/tinygo-org/tinygo/releases/download/v0.30.0/tinygo_0.30.0_amd64.deb"
            echo "   sudo dpkg -i tinygo_0.30.0_amd64.deb"
            echo ""
            echo "   # 或使用包管理器:"
            echo "   sudo apt install tinygo"
            ;;
        *)
            echo "   请访问: https://tinygo.org/getting-started/install/"
            ;;
    esac
    
    echo ""
    print_warning "请先安装TinyGo编译器后再运行此工具"
    exit 1
fi

# 检查TinyGo版本
TINYGO_VERSION=$(tinygo version 2>/dev/null | head -n1)
print_success "TinyGo已安装: $TINYGO_VERSION"

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    print_warning "Go编译器未安装（TinyGo需要Go支持）"
    echo ""
    echo -e "${WHITE}📝 安装方法：${NC}"
    echo "   访问: https://golang.org/dl/"
    echo "   或使用包管理器安装"
else
    GO_VERSION=$(go version 2>/dev/null | awk '{print $3}')
    print_success "Go已安装: $GO_VERSION"
fi

echo ""

# ==================== 项目检查 ====================

print_step "检查项目结构..."

# 查找源代码文件
if [[ -f "src/main.go" ]]; then
    SOURCE_FILE="src/main.go"
    print_success "找到源代码: $SOURCE_FILE"
elif [[ -f "main.go" ]]; then
    SOURCE_FILE="main.go"
    print_success "找到源代码: $SOURCE_FILE"
else
    print_error "未找到源代码文件"
    echo ""
    echo -e "${WHITE}期望的文件位置：${NC}"
    echo "   src/main.go  (推荐结构)"
    echo "   main.go      (简单结构)"
    echo ""
    exit 1
fi

# 检查源代码基本语法
print_progress "检查源代码语法..."
if go build -o /dev/null "$SOURCE_FILE" 2>/dev/null; then
    print_success "源代码语法检查通过"
else
    print_warning "源代码可能存在语法错误"
    echo ""
    echo -e "${WHITE}🔍 语法检查结果：${NC}"
    go build -o /dev/null "$SOURCE_FILE"
    echo ""
    print_info "提示：修复语法错误后重新编译"
fi

# 创建build目录
if [[ ! -d "build" ]]; then
    mkdir -p build
    print_info "创建build目录"
else
    print_info "使用现有build目录"
fi

echo ""

# ==================== 编译过程 ====================

print_step "开始编译合约..."

# 构建编译命令
COMPILE_CMD="tinygo build"
COMPILE_CMD+=" -target $TARGET"
COMPILE_CMD+=" -o build/main.wasm"

# 添加优化选项
if [[ $OPTIMIZE == true ]]; then
    COMPILE_CMD+=" -opt=2"
    print_info "启用优化编译（可能需要更长时间）"
fi

# 添加其他有用的选项
COMPILE_CMD+=" -no-debug"  # 移除调试信息，减小文件大小
COMPILE_CMD+=" $SOURCE_FILE"

# 显示编译命令
if [[ $VERBOSE == true ]]; then
    echo -e "${WHITE}🔧 编译命令：${NC}"
    echo "   $COMPILE_CMD"
    echo ""
fi

print_progress "正在编译..."

# 执行编译
START_TIME=$(date +%s)

if [[ $VERBOSE == true ]]; then
    # 详细模式：显示编译输出
    eval $COMPILE_CMD
    COMPILE_EXIT_CODE=$?
else
    # 静默模式：捕获输出
    COMPILE_OUTPUT=$(eval $COMPILE_CMD 2>&1)
    COMPILE_EXIT_CODE=$?
fi

END_TIME=$(date +%s)
COMPILE_TIME=$((END_TIME - START_TIME))

echo ""

# ==================== 编译结果 ====================

if [[ $COMPILE_EXIT_CODE -eq 0 ]]; then
    print_success "编译成功！🎉"
    
    # 文件信息
    if [[ -f "build/main.wasm" ]]; then
        FILE_SIZE=$(ls -lh build/main.wasm | awk '{print $5}')
        FILE_SIZE_BYTES=$(ls -l build/main.wasm | awk '{print $5}')
        
        echo ""
        echo -e "${WHITE}📊 编译结果：${NC}"
        echo -e "${GREEN}   📁 输出文件: build/main.wasm${NC}"
        echo -e "${GREEN}   📏 文件大小: $FILE_SIZE ($FILE_SIZE_BYTES bytes)${NC}"
        echo -e "${GREEN}   ⏱️  编译耗时: ${COMPILE_TIME}秒${NC}"
        echo -e "${GREEN}   🎯 目标平台: $TARGET${NC}"
        
        # 文件大小建议
        if [[ $FILE_SIZE_BYTES -gt 1048576 ]]; then  # > 1MB
            print_warning "文件较大，考虑启用优化编译"
            echo "   使用: ./simple-build.sh --optimize"
        elif [[ $FILE_SIZE_BYTES -lt 10240 ]]; then  # < 10KB
            print_info "文件大小很小，编译高效！"
        fi
    fi
    
else
    print_error "编译失败"
    
    # 显示错误信息
    if [[ $VERBOSE == false && -n "$COMPILE_OUTPUT" ]]; then
        echo ""
        echo -e "${WHITE}🔍 错误详情：${NC}"
        echo "$COMPILE_OUTPUT"
    fi
    
    # 常见错误的解决建议
    echo ""
    echo -e "${WHITE}🛠️  常见问题解决方案：${NC}"
    
    if echo "$COMPILE_OUTPUT" | grep -q "undefined"; then
        echo -e "${CYAN}• 未定义的函数/变量：检查import语句和函数名${NC}"
    fi
    
    if echo "$COMPILE_OUTPUT" | grep -q "syntax error"; then
        echo -e "${CYAN}• 语法错误：检查括号、分号等语法元素${NC}"
    fi
    
    if echo "$COMPILE_OUTPUT" | grep -q "import"; then
        echo -e "${CYAN}• 导入错误：确认import路径正确${NC}"
        echo -e "${CYAN}  正确示例：import \"github.com/weisyn/v1/contracts/sdk/go/framework\"${NC}"
    fi
    
    if echo "$COMPILE_OUTPUT" | grep -q "wasm"; then
        echo -e "${CYAN}• WASM目标错误：尝试使用不同的目标平台${NC}"
        echo -e "${CYAN}  命令：./simple-build.sh --target wasm${NC}"
    fi
    
    echo -e "${CYAN}• 获取帮助：运行 ./simple-build.sh --help${NC}"
    echo -e "${CYAN}• 详细信息：运行 ./simple-build.sh --verbose${NC}"
    
    exit 1
fi

echo ""

# ==================== 后续步骤 ====================

print_header "🚀 下一步操作"

echo -e "${WHITE}编译完成后你可以：${NC}"
echo ""
echo -e "${GREEN}1. 🧪 测试合约：${NC}"
echo "   ./test.sh                    # 运行测试脚本"
echo ""
echo -e "${GREEN}2. 🚀 部署合约：${NC}"
echo "   ./deploy.sh testnet          # 部署到测试网"
echo "   ./deploy.sh mainnet          # 部署到主网"
echo ""
echo -e "${GREEN}3. 🔍 验证合约：${NC}"
echo "   ../../../tools/beginner/verify.sh build/main.wasm"
echo ""
echo -e "${GREEN}4. 📊 分析合约：${NC}"
echo "   wasm-objdump -x build/main.wasm  # 查看WASM结构"
echo "   hexdump -C build/main.wasm | head  # 查看二进制内容"

echo ""
echo -e "${WHITE}💡 编译技巧：${NC}"
echo -e "${CYAN}• 开发阶段：使用普通编译获得快速反馈${NC}"
echo -e "${CYAN}• 生产部署：使用 --optimize 获得最小文件${NC}"
echo -e "${CYAN}• 调试问题：使用 --verbose 查看详细信息${NC}"
echo -e "${CYAN}• 跨平台：使用不同 --target 适配不同环境${NC}"

echo ""
print_success "编译工具使用完成！"

echo ""
echo -e "${BLUE}================================${NC}"
echo -e "${WHITE}     WES智能合约编译完成！     ${NC}"
echo -e "${BLUE}================================${NC}"
