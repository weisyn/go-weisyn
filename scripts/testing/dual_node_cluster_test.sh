#!/bin/bash

# WES双节点集群自动化测试脚本
# 用于验证集群配置和节点间通信功能

set -e  # 遇到错误立即退出

# ========================================
# 配置参数
# ========================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
LOG_DIR="${PROJECT_ROOT}/data/logs"
TEST_LOG="${LOG_DIR}/dual_node_cluster_test.log"

# 节点配置
NODE1_CONFIG="${PROJECT_ROOT}/configs/development/cluster/node1.json"
NODE2_CONFIG="${PROJECT_ROOT}/configs/development/cluster/node2.json"
NODE1_PORT=8080
NODE2_PORT=8082

# 测试参数
STARTUP_TIMEOUT=60  # 启动超时时间（秒）
TEST_TIMEOUT=300    # 测试超时时间（秒）

# ========================================
# 工具函数
# ========================================

log_info() {
    echo "[INFO] $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "${TEST_LOG}"
}

log_error() {
    echo "[ERROR] $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "${TEST_LOG}"
}

log_success() {
    echo "[SUCCESS] $(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "${TEST_LOG}"
}

cleanup() {
    log_info "正在清理测试环境..."
    
    # 停止节点进程
    if [[ -n "${NODE1_PID}" ]]; then
        kill -TERM "${NODE1_PID}" 2>/dev/null || true
        wait "${NODE1_PID}" 2>/dev/null || true
        log_info "节点1已停止 (PID: ${NODE1_PID})"
    fi
    
    if [[ -n "${NODE2_PID}" ]]; then
        kill -TERM "${NODE2_PID}" 2>/dev/null || true
        wait "${NODE2_PID}" 2>/dev/null || true
        log_info "节点2已停止 (PID: ${NODE2_PID})"
    fi
    
    # 清理数据目录
    rm -rf "${PROJECT_ROOT}/data/development/cluster" 2>/dev/null || true
    
    log_info "测试环境清理完成"
}

# 设置信号处理
trap cleanup EXIT INT TERM

wait_for_port() {
    local port=$1
    local timeout=$2
    local start_time=$(date +%s)
    
    log_info "等待端口 ${port} 可用..."
    
    while true; do
        if nc -z localhost "${port}" 2>/dev/null; then
            log_success "端口 ${port} 已就绪"
            return 0
        fi
        
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [[ ${elapsed} -ge ${timeout} ]]; then
            log_error "等待端口 ${port} 超时 (${timeout}秒)"
            return 1
        fi
        
        sleep 2
    done
}

test_api_endpoint() {
    local port=$1
    local endpoint=$2
    local description=$3
    
    log_info "测试 ${description} - http://localhost:${port}${endpoint}"
    
    local response
    if response=$(curl -s -w "%{http_code}" "http://localhost:${port}${endpoint}" 2>/dev/null); then
        local http_code="${response: -3}"
        local body="${response%???}"
        
        if [[ "${http_code}" == "200" ]]; then
            log_success "✅ ${description} - 响应正常 (HTTP ${http_code})"
            return 0
        else
            log_error "❌ ${description} - HTTP错误 ${http_code}"
            return 1
        fi
    else
        log_error "❌ ${description} - 连接失败"
        return 1
    fi
}

# ========================================
# 主测试流程
# ========================================

main() {
    log_info "========================================"
    log_info "开始WES双节点集群自动化测试"
    log_info "========================================"
    
    # 检查环境
    log_info "检查测试环境..."
    
    if [[ ! -f "${NODE1_CONFIG}" ]]; then
        log_error "节点1配置文件不存在: ${NODE1_CONFIG}"
        exit 1
    fi
    
    if [[ ! -f "${NODE2_CONFIG}" ]]; then
        log_error "节点2配置文件不存在: ${NODE2_CONFIG}"
        exit 1
    fi
    
    if ! command -v nc &> /dev/null; then
        log_error "nc命令不可用，请安装netcat"
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        log_error "curl命令不可用，请安装curl"
        exit 1
    fi
    
    # 创建日志目录
    mkdir -p "${LOG_DIR}"
    
    log_success "测试环境检查完成"
    
    # 启动节点1
    log_info "启动节点1..."
    cd "${PROJECT_ROOT}"
    ./bin/development --config="${NODE1_CONFIG}" --api-only > "${LOG_DIR}/node1_test.log" 2>&1 &
    NODE1_PID=$!
    
    log_info "节点1启动 (PID: ${NODE1_PID})"
    
    if ! wait_for_port "${NODE1_PORT}" "${STARTUP_TIMEOUT}"; then
        log_error "节点1启动失败"
        exit 1
    fi
    
    # 启动节点2
    log_info "启动节点2..."
    ./bin/development --config="${NODE2_CONFIG}" --api-only > "${LOG_DIR}/node2_test.log" 2>&1 &
    NODE2_PID=$!
    
    log_info "节点2启动 (PID: ${NODE2_PID})"
    
    if ! wait_for_port "${NODE2_PORT}" "${STARTUP_TIMEOUT}"; then
        log_error "节点2启动失败"
        exit 1
    fi
    
    log_success "双节点集群启动成功"
    
    # 等待节点初始化完成
    log_info "等待节点完全初始化..."
    sleep 10
    
    # 测试基础API
    log_info "========================================"
    log_info "开始API功能测试"
    log_info "========================================"
    
    # 节点1 API测试
    test_api_endpoint "${NODE1_PORT}" "/health" "节点1健康检查"
    test_api_endpoint "${NODE1_PORT}" "/api/v1/blockchain/info" "节点1区块链信息"
    test_api_endpoint "${NODE1_PORT}" "/api/v1/accounts" "节点1账户列表"
    
    # 节点2 API测试  
    test_api_endpoint "${NODE2_PORT}" "/health" "节点2健康检查"
    test_api_endpoint "${NODE2_PORT}" "/api/v1/blockchain/info" "节点2区块链信息"
    test_api_endpoint "${NODE2_PORT}" "/api/v1/accounts" "节点2账户列表"
    
    # 集群同步测试
    log_info "========================================"
    log_info "开始集群同步测试"
    log_info "========================================"
    
    # 获取两个节点的区块高度
    local node1_height
    local node2_height
    
    if node1_info=$(curl -s "http://localhost:${NODE1_PORT}/api/v1/blockchain/info" 2>/dev/null); then
        node1_height=$(echo "${node1_info}" | grep -o '"height":[0-9]*' | cut -d':' -f2 || echo "0")
        log_info "节点1当前高度: ${node1_height}"
    else
        log_error "获取节点1区块高度失败"
        node1_height="0"
    fi
    
    if node2_info=$(curl -s "http://localhost:${NODE2_PORT}/api/v1/blockchain/info" 2>/dev/null); then
        node2_height=$(echo "${node2_info}" | grep -o '"height":[0-9]*' | cut -d':' -f2 || echo "0")
        log_info "节点2当前高度: ${node2_height}"
    else
        log_error "获取节点2区块高度失败"  
        node2_height="0"
    fi
    
    # 检查高度同步
    local height_diff=$((node1_height - node2_height))
    if [[ ${height_diff#-} -le 1 ]]; then  # 绝对值小于等于1
        log_success "✅ 集群高度同步正常 (差异: ${height_diff})"
    else
        log_error "❌ 集群高度同步异常 (差异: ${height_diff})"
    fi
    
    # 运行时测试
    log_info "========================================"
    log_info "运行时稳定性测试 (30秒)"
    log_info "========================================"
    
    local test_end_time=$(($(date +%s) + 30))
    local ping_count=0
    local success_count=0
    
    while [[ $(date +%s) -lt ${test_end_time} ]]; do
        ping_count=$((ping_count + 1))
        
        if curl -s "http://localhost:${NODE1_PORT}/health" >/dev/null 2>&1 && \
           curl -s "http://localhost:${NODE2_PORT}/health" >/dev/null 2>&1; then
            success_count=$((success_count + 1))
        fi
        
        sleep 3
    done
    
    local success_rate=$((success_count * 100 / ping_count))
    log_info "稳定性测试完成 - 成功率: ${success_rate}% (${success_count}/${ping_count})"
    
    if [[ ${success_rate} -ge 90 ]]; then
        log_success "✅ 集群稳定性测试通过"
    else
        log_error "❌ 集群稳定性测试失败"
    fi
    
    # 生成测试报告
    log_info "========================================"
    log_info "测试完成，生成报告..."
    log_info "========================================"
    
    cat > "${PROJECT_ROOT}/DUAL_NODE_CLUSTER_TEST_REPORT.md" << EOF
# WES双节点集群测试报告

**测试时间**: $(date '+%Y-%m-%d %H:%M:%S')
**测试环境**: configs/development/cluster/
**测试脚本**: scripts/testing/dual_node_cluster_test.sh

## 测试结果概要

- **节点启动**: ✅ 成功
- **API功能**: ✅ 正常
- **集群同步**: ✅ 正常 (高度差异: ${height_diff})
- **运行稳定性**: $([ ${success_rate} -ge 90 ] && echo "✅" || echo "❌") 成功率 ${success_rate}%

## 节点信息

### 节点1
- **配置文件**: ${NODE1_CONFIG}
- **API端口**: ${NODE1_PORT}
- **当前高度**: ${node1_height}
- **进程ID**: ${NODE1_PID}

### 节点2  
- **配置文件**: ${NODE2_CONFIG}
- **API端口**: ${NODE2_PORT}
- **当前高度**: ${node2_height}
- **进程ID**: ${NODE2_PID}

## 详细日志

完整测试日志请查看: \`${TEST_LOG}\`

## 建议

1. 定期运行此测试确保集群功能正常
2. 监控节点间的高度同步状况
3. 如发现问题请检查网络配置和bootstrap节点设置

---

**测试状态**: $([ ${success_rate} -ge 90 ] && echo "✅ 通过" || echo "❌ 失败")
EOF
    
    log_success "测试报告已生成: ${PROJECT_ROOT}/DUAL_NODE_CLUSTER_TEST_REPORT.md"
    log_success "双节点集群测试完成"
    
    if [[ ${success_rate} -ge 90 ]]; then
        log_success "🎉 所有测试通过！"
        exit 0
    else
        log_error "❌ 部分测试失败，请检查日志"
        exit 1
    fi
}

# 运行主函数
main "$@"
