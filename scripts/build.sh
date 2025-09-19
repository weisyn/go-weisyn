#!/bin/bash

# WES区块链构建脚本
# 自动注入版本信息到二进制文件中

set -e

# 脚本配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION_PACKAGE="github.com/weisyn/v1/internal/app/version"

# 默认构建配置
DEFAULT_VERSION="v0.0.1"
DEFAULT_BUILD_ENV="development"
DEFAULT_OUTPUT_DIR="$PROJECT_ROOT/dist"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 输出函数
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# 帮助信息
show_help() {
    cat << EOF
WES区块链构建脚本

用法: $0 [选项]

选项:
  -v, --version VERSION      设置版本号 (默认: $DEFAULT_VERSION)
  -e, --env ENVIRONMENT     设置构建环境 [development|testing|production] (默认: $DEFAULT_BUILD_ENV)
  -o, --output DIR          设置输出目录 (默认: $DEFAULT_OUTPUT_DIR)
  -t, --target TARGET       设置构建目标 [wesd|all] (默认: wesd)
  --dry-run                仅显示构建信息，不执行构建
  -h, --help               显示帮助信息

环境变量:
  WES_VERSION              覆盖版本号
  WES_BUILD_ENV           覆盖构建环境
  WES_BUILD_USER          覆盖构建用户
  
示例:
  # 开发构建
  $0 -v v1.0.0 -e development
  
  # 生产构建
  $0 -v v1.0.0 -e production -o ./release
  
  # 测试构建
  $0 -v v1.0.0 -e testing
  
  # 跨平台构建
  $0 -v v1.0.0 -e production -t all

产物命名规则:
  wesd_{version}+{timestamp}_{env}_{os}_{arch}
  例如: wesd_v1.0.0+20240101120000_production_linux_amd64
EOF
}

# 解析命令行参数
VERSION="$DEFAULT_VERSION"
BUILD_ENV="$DEFAULT_BUILD_ENV"
OUTPUT_DIR="$DEFAULT_OUTPUT_DIR"
TARGET="wesd"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -e|--env)
            BUILD_ENV="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        -t|--target)
            TARGET="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            error "未知选项: $1"
            ;;
    esac
done

# 环境变量覆盖
[[ -n "$WES_VERSION" ]] && VERSION="$WES_VERSION"
[[ -n "$WES_BUILD_ENV" ]] && BUILD_ENV="$WES_BUILD_ENV"

# 验证构建环境
case "$BUILD_ENV" in
    development|testing|production)
        ;;
    *)
        error "无效的构建环境: $BUILD_ENV (支持: development, testing, production)"
        ;;
esac

# 收集构建信息
info "收集构建信息..."

# 版本信息
BUILD_VERSION="$VERSION"

# 构建标识符生成（用于产物命名）
BUILD_IDENTIFIER=$(date +"%Y%m%d%H%M%S") # 时间戳作为标识符

# 构建信息
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_USER="${WES_BUILD_USER:-$(whoami)}"
BUILD_HOST="$(hostname)"

# Go信息
GO_VERSION=$(go version | awk '{print $3}')

# 构建目标配置
declare -A BUILD_TARGETS
case "$TARGET" in
    wesd)
        BUILD_TARGETS[wesd]="cmd/wesd/main.go"
        ;;
    all)
        BUILD_TARGETS[wesd]="cmd/wesd/main.go"
        # 可以添加其他二进制目标
        # BUILD_TARGETS[wesd-cli]="cmd/wesd-cli/main.go"
        ;;
    *)
        error "未知构建目标: $TARGET"
        ;;
esac

# 平台配置（用于跨平台构建）
declare -a PLATFORMS=(
    "linux/amd64"
    "linux/arm64" 
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# 构建LDFLAGS
LDFLAGS=(
    "-X '${VERSION_PACKAGE}.Version=${BUILD_VERSION}'"
    "-X '${VERSION_PACKAGE}.BuildTime=${BUILD_TIME}'"
    "-X '${VERSION_PACKAGE}.BuildUser=${BUILD_USER}'"
    "-X '${VERSION_PACKAGE}.BuildHost=${BUILD_HOST}'"
    "-X '${VERSION_PACKAGE}.BuildEnv=${BUILD_ENV}'"
)

# 拼接LDFLAGS
LDFLAGS_STR=$(IFS=' '; echo "${LDFLAGS[*]}")

# 显示构建信息
info "构建配置:"
echo "  版本号:       $BUILD_VERSION"
echo "  构建标识符:   $BUILD_IDENTIFIER"
echo "  构建时间:     $BUILD_TIME"
echo "  构建用户:     $BUILD_USER"
echo "  构建主机:     $BUILD_HOST"
echo "  构建环境:     $BUILD_ENV"
echo "  Go版本:       $GO_VERSION"
echo "  输出目录:     $OUTPUT_DIR"
echo "  构建目标:     $TARGET"

if [[ "$DRY_RUN" == "true" ]]; then
    info "干运行模式，跳过实际构建"
    exit 0
fi

# 创建输出目录
mkdir -p "$OUTPUT_DIR"

# 执行构建
info "开始构建..."

cd "$PROJECT_ROOT"

for binary in "${!BUILD_TARGETS[@]}"; do
    main_file="${BUILD_TARGETS[$binary]}"
    
    info "构建 $binary ..."
    
    if [[ "$TARGET" == "all" ]]; then
        # 跨平台构建
        for platform in "${PLATFORMS[@]}"; do
            IFS='/' read -r os arch <<< "$platform"
            
            # 构建产物文件名
            output_name="${binary}_${BUILD_VERSION}+${BUILD_IDENTIFIER}_${BUILD_ENV}_${os}_${arch}"
            [[ "$os" == "windows" ]] && output_name+=".exe"
            
            output_path="$OUTPUT_DIR/$output_name"
            
            info "  构建 $os/$arch -> $output_name"
            
            GOOS="$os" GOARCH="$arch" go build \
                -ldflags="$LDFLAGS_STR" \
                -o "$output_path" \
                "$main_file"
                
            if [[ $? -eq 0 ]]; then
                success "    ✓ $output_name 构建成功 ($(du -h "$output_path" | cut -f1))"
            else
                error "构建失败: $os/$arch"
            fi
        done
    else
        # 本地平台构建
        os=$(go env GOOS)
        arch=$(go env GOARCH)
        
        output_name="${binary}_${BUILD_VERSION}+${BUILD_IDENTIFIER}_${BUILD_ENV}_${os}_${arch}"
        [[ "$os" == "windows" ]] && output_name+=".exe"
        
        output_path="$OUTPUT_DIR/$output_name"
        
        info "  构建 $os/$arch -> $output_name"
        
        go build \
            -ldflags="$LDFLAGS_STR" \
            -o "$output_path" \
            "$main_file"
            
        if [[ $? -eq 0 ]]; then
            success "    ✓ $output_name 构建成功 ($(du -h "$output_path" | cut -f1))"
        else
            error "构建失败"
        fi
    fi
done

success "所有构建任务完成!"
info "构建产物位于: $OUTPUT_DIR"

# 显示产物列表
if [[ -d "$OUTPUT_DIR" ]]; then
    info "产物列表:"
    ls -la "$OUTPUT_DIR" | grep "^-" | awk '{printf "  %s (%s)\n", $9, $5}'
fi
