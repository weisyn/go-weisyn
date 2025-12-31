#!/bin/bash

# ================================================================================
# WES 跨平台发布构建脚本
# ================================================================================
# 
# 功能说明：
#   - 支持 Linux/macOS/Windows 多操作系统
#   - 支持 amd64/arm64 多芯片架构
#   - 自动版本号嵌入
#   - 自动打包压缩
#   - 生成校验和文件
#
# 使用方法：
#   ./scripts/build/release-build.sh [选项]
#
# 选项：
#   -v, --version VERSION   指定版本号 (默认从 git tag 读取)
#   -p, --platform PLATFORM 指定平台 (darwin, linux, windows)
#   -a, --arch ARCH         指定架构 (amd64, arm64)
#   -o, --output DIR        输出目录 (默认 dist/)
#   -c, --component COMP    指定组件 (node, cli, launcher, all)
#   --all                   构建所有平台
#   --clean                 构建前清理
#   --no-compress          不压缩打包
#   -h, --help             显示帮助
#
# ================================================================================

set -e

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 配置变量
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

# 项目信息
PROJECT_NAME="weisyn"
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# 默认配置
OUTPUT_DIR="${PROJECT_ROOT}/dist"
BUILD_ALL=false
CLEAN_FIRST=false
COMPRESS=true
COMPONENT="all"

# 支持的平台和架构
SUPPORTED_PLATFORMS=("darwin" "linux" "windows")
SUPPORTED_ARCHS=("amd64" "arm64")

# 版本信息 (从 git 获取或手动指定)
VERSION=""
GIT_COMMIT=$(git -C "$PROJECT_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG=$(git -C "$PROJECT_ROOT" describe --tags --always 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_DATE=$(date -u +"%Y%m%d")

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 颜色输出
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${PURPLE}[STEP]${NC} $1"; }

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 帮助信息
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

show_help() {
    cat << EOF
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
🚀 WES 跨平台发布构建脚本
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

用法: $0 [选项]

选项:
  -v, --version VERSION   指定版本号 (默认: 从 git tag 获取)
  -p, --platform PLATFORM 指定目标平台:
                           darwin  - macOS
                           linux   - Linux
                           windows - Windows
  -a, --arch ARCH         指定目标架构:
                           amd64 - x86_64 (Intel/AMD)
                           arm64 - ARM64 (Apple Silicon/ARM)
  -c, --component COMP    指定构建组件:
                           node     - 仅构建节点程序（weisyn-node）
                           cli      - 仅构建 CLI 工具（weisyn-cli）
                           launcher - 仅构建启动器（weisyn，可视化/TUI 外壳）
                           all      - 构建所有组件 (默认)
  -o, --output DIR        输出目录 (默认: dist/)
  --all                   构建所有平台和架构
  --clean                 构建前清理输出目录
  --no-compress          不进行压缩打包
  -h, --help             显示此帮助信息

示例:
  # 构建当前平台
  $0

  # 构建指定版本
  $0 -v 1.0.0

  # 构建 Linux AMD64
  $0 -p linux -a amd64

  # 构建所有平台
  $0 --all -v 1.2.0

  # 仅构建节点程序
  $0 -c node -p darwin -a arm64

命名规范:
  ${PROJECT_NAME}-{component}-{version}-{os}-{arch}[.exe]

  示例:
    weisyn-node-v1.0.0-darwin-arm64
    weisyn-cli-v1.0.0-linux-amd64
    weisyn-node-v1.0.0-windows-amd64.exe

压缩包命名:
  ${PROJECT_NAME}-{version}-{os}-{arch}.tar.gz     (Linux/macOS)
  ${PROJECT_NAME}-{version}-{os}-{arch}.zip        (Windows)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
EOF
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 参数解析
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

TARGET_PLATFORM=""
TARGET_ARCH=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -p|--platform)
            TARGET_PLATFORM="$2"
            shift 2
            ;;
        -a|--arch)
            TARGET_ARCH="$2"
            shift 2
            ;;
        -c|--component)
            COMPONENT="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --all)
            BUILD_ALL=true
            shift
            ;;
        --clean)
            CLEAN_FIRST=true
            shift
            ;;
        --no-compress)
            COMPRESS=false
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            log_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 版本号处理
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

get_version() {
    if [[ -n "$VERSION" ]]; then
        # 使用指定版本
        echo "$VERSION"
    elif [[ "$GIT_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
        # 从 git tag 获取语义化版本
        echo "$GIT_TAG"
    else
        # 使用开发版本: dev-{日期}-{commit}
        echo "dev-${BUILD_DATE}-${GIT_COMMIT}"
    fi
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 构建函数
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

build_binary() {
    local component=$1
    local os=$2
    local arch=$3
    local version=$4
    
    local cmd_path=""
    local output_name=""
    local ext=""
    
    # 确定命令路径
    case $component in
        node)
            cmd_path="./cmd/node"
            ;;
        cli)
            cmd_path="./cmd/cli"
            ;;
        launcher)
            cmd_path="./cmd/weisyn"
            ;;
        *)
            log_error "未知组件: $component"
            return 1
            ;;
    esac
    
    # Windows 需要 .exe 后缀
    if [[ "$os" == "windows" ]]; then
        ext=".exe"
    fi
    
    # 构建输出文件名
    output_name="${PROJECT_NAME}-${component}-${version}-${os}-${arch}${ext}"
    local output_path="${OUTPUT_DIR}/${os}-${arch}/${output_name}"
    
    log_step "构建 ${component} for ${os}/${arch}..."
    
    # 创建输出目录
    mkdir -p "$(dirname "$output_path")"
    
    # 构建 LDFLAGS
    local ldflags="-s -w"
    ldflags="${ldflags} -X main.Version=${version}"
    ldflags="${ldflags} -X main.GitCommit=${GIT_COMMIT}"
    ldflags="${ldflags} -X main.BuildTime=${BUILD_TIME}"
    ldflags="${ldflags} -X main.GoVersion=$(go version | awk '{print $3}')"
    ldflags="${ldflags} -X main.Platform=${os}/${arch}"
    
    # 设置环境变量并构建
    # 注意：某些依赖可能需要 CGO
    local cgo_enabled=0
    
    # 如果是 darwin 本地构建，可能需要启用 CGO（如有 ONNX 依赖）
    if [[ "$os" == "darwin" ]] && [[ "$(uname)" == "Darwin" ]]; then
        cgo_enabled=1
    fi
    
    # 对于 Linux 和 Windows 交叉编译，通常禁用 CGO
    # 如果项目有特殊 CGO 依赖，需要配置相应的交叉编译工具链
    
    GOOS=$os GOARCH=$arch CGO_ENABLED=$cgo_enabled \
        go build -ldflags "$ldflags" -o "$output_path" "$cmd_path" 2>&1
    
    if [[ -f "$output_path" ]]; then
        chmod +x "$output_path" 2>/dev/null || true
        local size=$(du -h "$output_path" | cut -f1)
        log_success "✅ ${output_name} ($size)"
        echo "$output_path"
    else
        log_error "❌ 构建失败: ${output_name}"
        return 1
    fi
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 压缩打包函数
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

compress_package() {
    local os=$1
    local arch=$2
    local version=$3
    
    local source_dir="${OUTPUT_DIR}/${os}-${arch}"
    local archive_name="${PROJECT_NAME}-${version}-${os}-${arch}"
    
    if [[ ! -d "$source_dir" ]]; then
        log_warn "目录不存在，跳过压缩: $source_dir"
        return 0
    fi
    
    cd "${OUTPUT_DIR}"
    
    if [[ "$os" == "windows" ]]; then
        # Windows 使用 zip
        local archive="${archive_name}.zip"
        log_step "打包 ${archive}..."
        zip -rq "$archive" "${os}-${arch}/"
        log_success "✅ ${archive}"
    else
        # Linux/macOS 使用 tar.gz
        local archive="${archive_name}.tar.gz"
        log_step "打包 ${archive}..."
        tar -czf "$archive" "${os}-${arch}/"
        log_success "✅ ${archive}"
    fi
    
    cd "$PROJECT_ROOT"
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 生成校验和
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

generate_checksums() {
    local version=$1
    
    log_step "生成校验和文件..."
    
    cd "${OUTPUT_DIR}"
    
    # 生成 SHA256 校验和
    local checksum_file="checksums-${version}.txt"
    
    # 清空或创建校验和文件
    > "$checksum_file"
    
    # 计算所有文件的校验和
    for file in *.tar.gz *.zip; do
        if [[ -f "$file" ]]; then
            if command -v shasum &> /dev/null; then
                shasum -a 256 "$file" >> "$checksum_file"
            elif command -v sha256sum &> /dev/null; then
                sha256sum "$file" >> "$checksum_file"
            fi
        fi
    done
    
    if [[ -s "$checksum_file" ]]; then
        log_success "✅ ${checksum_file}"
    fi
    
    cd "$PROJECT_ROOT"
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 构建单个平台
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

build_platform() {
    local os=$1
    local arch=$2
    local version=$3
    
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "🎯 构建平台: ${os}/${arch}"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # 根据组件选择构建
    case $COMPONENT in
        node)
            build_binary "node" "$os" "$arch" "$version"
            ;;
        cli)
            build_binary "cli" "$os" "$arch" "$version"
            ;;
        launcher)
            build_binary "launcher" "$os" "$arch" "$version"
            ;;
        all)
            build_binary "node" "$os" "$arch" "$version"
            build_binary "cli" "$os" "$arch" "$version"
            build_binary "launcher" "$os" "$arch" "$version"
            ;;
    esac
    
    # 压缩打包
    if [[ "$COMPRESS" == true ]]; then
        compress_package "$os" "$arch" "$version"
    fi
}

# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# 主函数
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

main() {
    # 切换到项目根目录
    cd "$PROJECT_ROOT"
    
    # 获取版本号
    local version=$(get_version)
    
    # 显示构建信息
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🚀 WES 发布构建"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "  📦 项目:     ${PROJECT_NAME}"
    echo "  🏷️  版本:     ${version}"
    echo "  📝 提交:     ${GIT_COMMIT}"
    echo "  🕐 时间:     ${BUILD_TIME}"
    echo "  📂 输出:     ${OUTPUT_DIR}"
    echo "  🔧 组件:     ${COMPONENT}"
    echo ""
    
    # 清理输出目录
    if [[ "$CLEAN_FIRST" == true ]]; then
        log_step "清理输出目录..."
        rm -rf "${OUTPUT_DIR}"
    fi
    
    # 创建输出目录
    mkdir -p "${OUTPUT_DIR}"
    
    # 确保依赖
    log_step "检查依赖..."
    if [[ -f "scripts/build/ensure_onnx_libs.sh" ]]; then
        bash scripts/build/ensure_onnx_libs.sh 2>/dev/null || true
    fi
    
    # 构建
    if [[ "$BUILD_ALL" == true ]]; then
        # 构建所有平台
        for os in "${SUPPORTED_PLATFORMS[@]}"; do
            for arch in "${SUPPORTED_ARCHS[@]}"; do
                # 跳过不常用的组合
                if [[ "$os" == "windows" && "$arch" == "arm64" ]]; then
                    log_warn "跳过 windows/arm64 (不常用)"
                    continue
                fi
                build_platform "$os" "$arch" "$version" || true
            done
        done
    elif [[ -n "$TARGET_PLATFORM" && -n "$TARGET_ARCH" ]]; then
        # 构建指定平台
        build_platform "$TARGET_PLATFORM" "$TARGET_ARCH" "$version"
    elif [[ -n "$TARGET_PLATFORM" ]]; then
        # 构建指定平台的所有架构
        for arch in "${SUPPORTED_ARCHS[@]}"; do
            build_platform "$TARGET_PLATFORM" "$arch" "$version" || true
        done
    elif [[ -n "$TARGET_ARCH" ]]; then
        # 构建所有平台的指定架构
        for os in "${SUPPORTED_PLATFORMS[@]}"; do
            build_platform "$os" "$TARGET_ARCH" "$version" || true
        done
    else
        # 构建当前平台
        local current_os=$(uname | tr '[:upper:]' '[:lower:]')
        local current_arch=$(uname -m)
        
        # 标准化架构名称
        case $current_arch in
            x86_64) current_arch="amd64" ;;
            aarch64|arm64) current_arch="arm64" ;;
        esac
        
        build_platform "$current_os" "$current_arch" "$version"
    fi
    
    # 生成校验和
    if [[ "$COMPRESS" == true ]]; then
        generate_checksums "$version"
    fi
    
    # 显示构建结果
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "✅ 构建完成"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "📦 输出目录: ${OUTPUT_DIR}"
    echo ""
    
    # 列出构建产物
    if [[ -d "${OUTPUT_DIR}" ]]; then
        echo "📋 构建产物:"
        find "${OUTPUT_DIR}" -type f \( -name "*.tar.gz" -o -name "*.zip" -o -name "checksums-*.txt" \) | sort | while read -r file; do
            local size=$(du -h "$file" | cut -f1)
            echo "   $(basename "$file") ($size)"
        done
        echo ""
        
        echo "📁 二进制文件:"
        find "${OUTPUT_DIR}" -type f -executable ! -name "*.sh" | sort | while read -r file; do
            local size=$(du -h "$file" | cut -f1)
            echo "   ${file#${OUTPUT_DIR}/} ($size)"
        done
    fi
    
    echo ""
}

# 运行主函数
main "$@"

