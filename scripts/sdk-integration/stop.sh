#!/usr/bin/env bash
# SDK 集成测试环境停止脚本
# 用途：停止 SDK 集成测试环境的 WES 节点

set -euo pipefail

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

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

# 停止节点
stop_node() {
    local pid_file="${PROJECT_ROOT}/data/testing/sdk-integration/node.pid"
    
    # 从 PID 文件读取
    if [[ -f "${pid_file}" ]]; then
        local pid
        pid=$(cat "${pid_file}")
        
        if kill -0 "${pid}" 2>/dev/null; then
            log_info "停止节点进程 (PID: ${pid})..."
            kill -TERM "${pid}" 2>/dev/null || true
            sleep 2
            
            # 如果还在运行，强制停止
            if kill -0 "${pid}" 2>/dev/null; then
                log_warning "强制停止节点进程..."
                kill -9 "${pid}" 2>/dev/null || true
                sleep 1
            fi
            
            log_success "节点已停止"
        else
            log_warning "PID 文件存在但进程不存在: ${pid}"
        fi
        
        # 删除 PID 文件
        rm -f "${pid_file}"
    else
        log_warning "PID 文件不存在: ${pid_file}"
    fi
    
    # 通过进程名查找并停止
    local pids
    pids=$(pgrep -f 'weisyn.*sdk-integration' 2>/dev/null || true)
    
    if [[ -n "${pids}" ]]; then
        log_info "发现相关进程: ${pids}"
        echo "${pids}" | xargs kill -TERM 2>/dev/null || true
        sleep 2
        
        # 强制停止
        pids=$(pgrep -f 'weisyn.*sdk-integration' 2>/dev/null || true)
        if [[ -n "${pids}" ]]; then
            log_warning "强制停止残留进程..."
            echo "${pids}" | xargs kill -9 2>/dev/null || true
        fi
        
        log_success "相关进程已停止"
    fi
    
    # 通过端口查找并停止（备用方法）
    if lsof -Pi :28680 -sTCP:LISTEN -t >/dev/null 2>&1; then
        local port_pid
        port_pid=$(lsof -Pi :28680 -sTCP:LISTEN -t)
        log_warning "端口 28680 仍被占用，进程 PID: ${port_pid}"
        log_info "是否停止占用端口的进程？(y/N)"
        read -p "" -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            kill -TERM "${port_pid}" 2>/dev/null || true
            sleep 2
            if kill -0 "${port_pid}" 2>/dev/null; then
                kill -9 "${port_pid}" 2>/dev/null || true
            fi
            log_success "端口占用进程已停止"
        fi
    fi
}

# 清理环境变量（可选）
cleanup_env_vars() {
    log_info "清理环境变量..."
    
    unset WES_ENDPOINT_HTTP
    unset WES_ENDPOINT_WS
    unset WES_TEST_PRIVKEY_MINER
    unset WES_TEST_PRIVKEY_USER_A
    unset WES_TEST_PRIVKEY_USER_B
    
    log_success "环境变量已清理"
}

# 主函数
main() {
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "SDK 集成测试环境停止"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info ""
    
    stop_node
    
    log_info ""
    log_info "是否清理环境变量？(y/N)"
    read -p "" -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cleanup_env_vars
    fi
    
    log_info ""
    log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_success "SDK 集成测试环境已停止"
    log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# 执行主函数
main "$@"

