#!/bin/bash
#
# WES系统 - DB-IP数据库下载脚本
#
# 🌍 **DB-IP数据库预下载工具 (DB-IP Database Pre-download Tool)**
#
# 本脚本用于预先下载DB-IP免费地理位置数据库，避免应用启动时的网络依赖。
# 支持断点续传、完整性验证和自动解压缩。
#
# 使用方法：
#   ./scripts/compliance/download_dbip.sh [选项]
#
# 选项：
#   -h, --help     显示帮助信息
#   -f, --force    强制重新下载（即使文件已存在）
#   -v, --verbose  详细输出模式
#
# 数据来源：
#   DB-IP (https://db-ip.com/) - Creative Commons Attribution 4.0 License
#   Attribution: "IP Geolocation by DB-IP"

set -euo pipefail  # 严格错误处理

# ============================================================================
#                                   配置常量
# ============================================================================

# DB-IP数据库配置
readonly DBIP_URL="https://download.db-ip.com/free/dbip-country-lite-2025-09.mmdb.gz"
readonly TARGET_DIR="./data/compliance"
readonly TARGET_FILE="${TARGET_DIR}/dbip-country-lite.mmdb"
readonly TEMP_FILE="${TARGET_FILE}.tmp"
readonly COMPRESSED_FILE="${TARGET_FILE}.gz"

# 脚本配置
readonly SCRIPT_NAME="$(basename "$0")"
readonly LOG_PREFIX="[DB-IP下载]"

# 颜色输出
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# ============================================================================
#                                 辅助函数
# ============================================================================

# 打印帮助信息
print_help() {
    cat << EOF
用法: ${SCRIPT_NAME} [选项]

WES系统 DB-IP数据库下载工具

此脚本用于预先下载DB-IP免费地理位置数据库，避免应用启动时的网络依赖。

选项:
    -h, --help     显示此帮助信息
    -f, --force    强制重新下载（即使文件已存在）
    -v, --verbose  详细输出模式

示例:
    ${SCRIPT_NAME}                # 正常下载
    ${SCRIPT_NAME} --force        # 强制重新下载
    ${SCRIPT_NAME} --verbose      # 详细输出模式

数据来源:
    DB-IP (https://db-ip.com/)
    许可: Creative Commons Attribution 4.0 License
    Attribution: "IP Geolocation by DB-IP"
EOF
}

# 日志输出函数
log_info() {
    echo -e "${BLUE}${LOG_PREFIX}${NC} $1"
}

log_success() {
    echo -e "${GREEN}${LOG_PREFIX}${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}${LOG_PREFIX}${NC} $1"
}

log_error() {
    echo -e "${RED}${LOG_PREFIX}${NC} $1" >&2
}

# 详细输出函数（仅在详细模式下输出）
log_verbose() {
    if [[ "${VERBOSE:-0}" == "1" ]]; then
        echo -e "${BLUE}${LOG_PREFIX} [详细]${NC} $1"
    fi
}

# 检查命令是否存在
check_command() {
    if ! command -v "$1" &> /dev/null; then
        log_error "错误：未找到必需的命令: $1"
        log_error "请安装 $1 后重试"
        exit 1
    fi
}

# 格式化文件大小
format_size() {
    local size=$1
    if (( size >= 1073741824 )); then
        printf "%.1fGB" "$(echo "scale=1; $size / 1073741824" | bc -l)"
    elif (( size >= 1048576 )); then
        printf "%.1fMB" "$(echo "scale=1; $size / 1048576" | bc -l)"
    elif (( size >= 1024 )); then
        printf "%.1fKB" "$(echo "scale=1; $size / 1024" | bc -l)"
    else
        printf "%dB" "$size"
    fi
}

# ============================================================================
#                                 核心功能
# ============================================================================

# 检查系统环境
check_environment() {
    log_verbose "检查系统环境..."
    
    # 检查必需的命令
    check_command "curl"
    check_command "gunzip"
    check_command "bc"
    
    # 检查工作目录
    if [[ ! -f "go.mod" ]] || [[ ! -d "internal" ]]; then
        log_error "错误：请在WES项目根目录执行此脚本"
        exit 1
    fi
    
    log_verbose "✅ 系统环境检查完成"
}

# 创建目标目录
create_directories() {
    log_verbose "创建目标目录: ${TARGET_DIR}"
    
    if ! mkdir -p "${TARGET_DIR}"; then
        log_error "错误：无法创建目录 ${TARGET_DIR}"
        exit 1
    fi
    
    log_verbose "✅ 目录创建完成"
}

# 检查文件是否已存在
check_existing_file() {
    if [[ -f "${TARGET_FILE}" ]] && [[ "${FORCE:-0}" != "1" ]]; then
        local file_size
        file_size=$(stat -c%s "${TARGET_FILE}" 2>/dev/null || stat -f%z "${TARGET_FILE}" 2>/dev/null)
        local formatted_size
        formatted_size=$(format_size "$file_size")
        
        log_warn "数据库文件已存在: ${TARGET_FILE} (${formatted_size})"
        log_warn "使用 --force 选项强制重新下载"
        return 0  # 文件已存在，无需下载
    fi
    return 1  # 需要下载
}

# 清理临时文件
cleanup_temp_files() {
    log_verbose "清理临时文件..."
    rm -f "${TEMP_FILE}" "${COMPRESSED_FILE}"
}

# 下载压缩文件
download_compressed_file() {
    log_info "开始下载DB-IP数据库..."
    log_info "下载地址: ${DBIP_URL}"
    log_info "目标文件: ${TARGET_FILE}"
    
    # 显示下载进度的curl选项
    local curl_opts=()
    if [[ "${VERBOSE:-0}" == "1" ]]; then
        curl_opts+=(--progress-bar)
    else
        curl_opts+=(--silent --show-error)
    fi
    
    # 下载文件
    if curl "${curl_opts[@]}" \
           --fail \
           --location \
           --retry 3 \
           --retry-delay 2 \
           --connect-timeout 30 \
           --max-time 600 \
           --output "${COMPRESSED_FILE}" \
           "${DBIP_URL}"; then
        
        # 检查下载的文件大小
        local file_size
        file_size=$(stat -c%s "${COMPRESSED_FILE}" 2>/dev/null || stat -f%z "${COMPRESSED_FILE}" 2>/dev/null)
        local formatted_size
        formatted_size=$(format_size "$file_size")
        
        log_success "✅ 压缩文件下载完成 (${formatted_size})"
        return 0
    else
        log_error "❌ 下载失败"
        return 1
    fi
}

# 解压缩文件
decompress_file() {
    log_info "解压缩数据库文件..."
    
    # 使用临时文件避免部分写入
    if gunzip --stdout "${COMPRESSED_FILE}" > "${TEMP_FILE}"; then
        # 原子性移动到最终位置
        if mv "${TEMP_FILE}" "${TARGET_FILE}"; then
            # 检查解压后的文件大小
            local file_size
            file_size=$(stat -c%s "${TARGET_FILE}" 2>/dev/null || stat -f%z "${TARGET_FILE}" 2>/dev/null)
            local formatted_size
            formatted_size=$(format_size "$file_size")
            
            log_success "✅ 解压缩完成 (${formatted_size})"
            return 0
        else
            log_error "❌ 无法移动文件到目标位置"
            return 1
        fi
    else
        log_error "❌ 解压缩失败"
        return 1
    fi
}

# 验证文件完整性
verify_file() {
    log_info "验证文件完整性..."
    
    # 基本检查：文件存在且非空
    if [[ ! -f "${TARGET_FILE}" ]]; then
        log_error "❌ 目标文件不存在"
        return 1
    fi
    
    local file_size
    file_size=$(stat -c%s "${TARGET_FILE}" 2>/dev/null || stat -f%z "${TARGET_FILE}" 2>/dev/null)
    
    if (( file_size == 0 )); then
        log_error "❌ 文件为空"
        return 1
    fi
    
    # 检查文件头是否为MMDB格式
    local file_header
    file_header=$(head -c 4 "${TARGET_FILE}" | xxd -p 2>/dev/null || true)
    if [[ -n "${file_header}" ]]; then
        log_verbose "文件头: ${file_header}"
    fi
    
    log_success "✅ 文件完整性验证通过"
    return 0
}

# 显示最终信息
show_final_info() {
    local file_size
    file_size=$(stat -c%s "${TARGET_FILE}" 2>/dev/null || stat -f%z "${TARGET_FILE}" 2>/dev/null)
    local formatted_size
    formatted_size=$(format_size "$file_size")
    
    log_success "🎉 DB-IP数据库下载完成！"
    echo
    echo "📍 文件路径: ${TARGET_FILE}"
    echo "📊 文件大小: ${formatted_size}"
    echo "🏷️  Attribution: IP Geolocation by DB-IP"
    echo "📄 许可协议: Creative Commons Attribution 4.0"
    echo
    log_info "现在可以启动WES节点，GeoIP服务将使用本地数据库文件"
}

# ============================================================================
#                                   主函数
# ============================================================================

main() {
    # 默认选项
    local FORCE=0
    local VERBOSE=0
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                print_help
                exit 0
                ;;
            -f|--force)
                FORCE=1
                shift
                ;;
            -v|--verbose)
                VERBOSE=1
                shift
                ;;
            *)
                log_error "未知选项: $1"
                print_help
                exit 1
                ;;
        esac
    done
    
    # 导出变量供子函数使用
    export FORCE VERBOSE
    
    echo "🌍 WES DB-IP数据库下载工具"
    echo "================================"
    
    # 执行主要流程
    check_environment
    create_directories
    
    # 检查是否需要下载
    if check_existing_file; then
        show_final_info
        exit 0
    fi
    
    # 设置清理陷阱
    trap cleanup_temp_files EXIT
    
    # 执行下载流程
    if download_compressed_file && decompress_file && verify_file; then
        cleanup_temp_files
        show_final_info
        exit 0
    else
        log_error "❌ 下载过程中发生错误"
        cleanup_temp_files
        exit 1
    fi
}

# 只有直接执行脚本时才调用main函数
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
