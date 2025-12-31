#!/usr/bin/env bash
# 统一的测试初始化脚本
# 用途：根据 configs/testing/config.json 中的 test 配置统一管理测试环境
# 所有测试脚本都应该通过此脚本初始化，而不是各自处理清理逻辑

set -eu

# 项目根目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
TEST_CONFIG="${PROJECT_ROOT}/configs/testing/config.json"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[✅]${NC} $1" >&2
}

log_warning() {
    echo -e "${YELLOW}[⚠️]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[❌]${NC} $1" >&2
}

# 读取测试配置
read_test_config() {
    if [[ ! -f "${TEST_CONFIG}" ]]; then
        log_warning "测试配置文件不存在: ${TEST_CONFIG}，使用默认配置"
        echo "true|10|true|true"  # cleanup_on_start|keep_recent_logs|cleanup_wrong_locations|single_node_mode
        return
    fi

    # 使用 jq 解析配置（如果可用）
    if command -v jq &> /dev/null; then
        local cleanup_on_start
        cleanup_on_start=$(jq -r '.test.cleanup_on_start // true' "${TEST_CONFIG}" 2>/dev/null || echo "true")
        
        local keep_recent_logs
        keep_recent_logs=$(jq -r '.test.keep_recent_logs // 10' "${TEST_CONFIG}" 2>/dev/null || echo "10")
        
        local cleanup_wrong_locations
        cleanup_wrong_locations=$(jq -r '.test.cleanup_wrong_locations // true' "${TEST_CONFIG}" 2>/dev/null || echo "true")
        
        local single_node_mode
        single_node_mode=$(jq -r '.test.single_node_mode // true' "${TEST_CONFIG}" 2>/dev/null || echo "true")
        
        echo "${cleanup_on_start}|${keep_recent_logs}|${cleanup_wrong_locations}|${single_node_mode}"
    else
        # 如果没有 jq，使用 grep 和 sed 简单解析
        local cleanup_on_start="true"
        local keep_recent_logs="10"
        local cleanup_wrong_locations="true"
        local single_node_mode="true"
        
        if grep -q '"cleanup_on_start".*false' "${TEST_CONFIG}"; then
            cleanup_on_start="false"
        fi
        if grep -q '"keep_recent_logs".*[0-9]' "${TEST_CONFIG}"; then
            keep_recent_logs=$(grep -o '"keep_recent_logs".*[0-9]' "${TEST_CONFIG}" | grep -o '[0-9]\+' | head -1 || echo "10")
        fi
        if grep -q '"cleanup_wrong_locations".*false' "${TEST_CONFIG}"; then
            cleanup_wrong_locations="false"
        fi
        if grep -q '"single_node_mode".*false' "${TEST_CONFIG}"; then
            single_node_mode="false"
        fi
        
        echo "${cleanup_on_start}|${keep_recent_logs}|${cleanup_wrong_locations}|${single_node_mode}"
    fi
}

# 停止所有相关节点进程
stop_all_nodes() {
    log_info "停止所有相关节点进程..."
    
    local pids

    # 收集当前所有可能的 testing 节点 PID
    collect_testing_pids() {
        local raw
        raw=""
        raw+=" $(pgrep -f 'bin/weisyn-testing --daemon --env testing' 2>/dev/null || true)"
        raw+=" $(pgrep -f 'cmd/weisyn.*--env testing' 2>/dev/null || true)"
        raw+=" $(pgrep -f 'weisyn-testing' 2>/dev/null || true)"
        echo "${raw}" | xargs -n1 2>/dev/null | sort -u | xargs 2>/dev/null || true
    }

    pids="$(collect_testing_pids)"
    
    if [[ -n "${pids}" ]]; then
        log_info "找到运行中的节点进程: ${pids}"
        echo "${pids}" | xargs kill -TERM 2>/dev/null || true
        sleep 2
        
        # 如果还有进程，强制杀死
        pids="$(collect_testing_pids)"
        if [[ -n "${pids}" ]]; then
            log_warning "强制停止残留进程..."
            echo "${pids}" | xargs kill -9 2>/dev/null || true
            sleep 1

            # 第三次检查：如果仍然有残留进程，视为严重错误，直接中止测试
            pids="$(collect_testing_pids)"
            if [[ -n "${pids}" ]]; then
                log_error "仍有 testing 节点进程无法停止: ${pids}"
                log_error "请手动检查并停止这些进程后重试测试脚本"
                exit 1
            fi
        fi
        log_success "节点进程已停止"
    else
        log_info "未找到运行中的节点进程"
    fi
}

# 清理测试数据（根据配置）
# ⚠️ 注意：此函数已废弃，数据清理应由 Go 程序逻辑执行
# 保留此函数仅用于日志管理，数据清理由节点启动时的 CleanupHistoricalData() 执行
cleanup_test_data() {
    local cleanup_on_start="$1"
    local keep_recent_logs="$2"
    local cleanup_wrong_locations="$3"
    
    # ⚠️ 重要：数据清理已由 Go 程序逻辑执行（cmd/common/startup.go:CleanupHistoricalData）
    # 脚本不再负责清理数据目录，只负责日志管理
    log_info "数据清理由节点启动时的 Go 程序逻辑执行（CleanupHistoricalData）"
    
    # 只清理旧的测试日志（根据配置保留数量）
    # 日志统一归集在 data/testing/logs/onnx_test_logs 下
    local log_dir="${PROJECT_ROOT}/data/testing/logs/onnx_test_logs"
    if [[ -d "${log_dir}" ]]; then
        local log_count
        log_count=$(ls -1 "${log_dir}"/test_report_*.txt 2>/dev/null | wc -l | tr -d ' ')
        if [[ ${log_count} -gt ${keep_recent_logs} ]]; then
            log_info "清理旧测试日志（保留最近${keep_recent_logs}个，当前有${log_count}个）..."
            ls -t "${log_dir}"/test_report_*.txt 2>/dev/null | tail -n +$((keep_recent_logs + 1)) | xargs rm -f 2>/dev/null || true
            log_success "已清理旧测试日志"
        fi
    else
        mkdir -p "${log_dir}"
        log_info "创建测试日志目录"
    fi
    
    # ⚠️ 注意：错误位置的数据目录清理也应由 Go 程序逻辑执行
    # 脚本不再负责清理数据目录，只负责日志管理
    if [[ "${cleanup_wrong_locations}" == "true" ]]; then
        log_info "错误位置的数据目录清理应由 Go 程序逻辑执行（CleanupHistoricalData）"
    fi
}

# 主函数：初始化测试环境
init_test_environment() {
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "统一测试环境初始化（基于 configs/testing/config.json）"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # 读取测试配置
    local config
    config=$(read_test_config)
    IFS='|' read -r cleanup_on_start keep_recent_logs cleanup_wrong_locations single_node_mode <<< "${config}"
    
    log_info "测试配置:"
    log_info "  - cleanup_on_start: ${cleanup_on_start}"
    log_info "  - keep_recent_logs: ${keep_recent_logs}"
    log_info "  - cleanup_wrong_locations: ${cleanup_wrong_locations}"
    log_info "  - single_node_mode: ${single_node_mode}"
    log_info ""
    
    # 1. 停止所有相关节点进程
    stop_all_nodes
    
    # 2. 清理测试数据（根据配置）
    cleanup_test_data "${cleanup_on_start}" "${keep_recent_logs}" "${cleanup_wrong_locations}"
    
    # 3. 等待进程完全停止
    sleep 2
    
    log_success "测试环境初始化完成"
    log_info ""
    
    # 导出配置供调用方使用
    export TEST_CLEANUP_ON_START="${cleanup_on_start}"
    export TEST_KEEP_RECENT_LOGS="${keep_recent_logs}"
    export TEST_CLEANUP_WRONG_LOCATIONS="${cleanup_wrong_locations}"
    export TEST_SINGLE_NODE_MODE="${single_node_mode}"
}

# 如果直接运行此脚本，执行初始化
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    init_test_environment
fi

