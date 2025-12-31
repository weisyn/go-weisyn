#!/bin/bash
# ONNX环境重建+测试脚本
# 用途：彻底清理环境 → 重建项目 → 运行测试
# 注意：追踪日志分析等功能已移除，专注于核心流程

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 日志文件
NODE_LOG="${PROJECT_ROOT}/data/testing/logs/onnx_test_logs/node.log"
BUILD_LOG="/tmp/onnx_build.log"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $*"
}

# 步骤1: 彻底清理所有旧内容
cleanup_all() {
    log_step "步骤 1/5: 彻底清理所有旧内容"
    
    # 1.1 停止所有相关进程
    log_info "停止所有旧进程..."
    pkill -f "weisyn\|testing" 2>/dev/null || true
    sleep 3
    
    # 检查是否还有进程
    if pgrep -f "weisyn\|testing" >/dev/null 2>&1; then
        log_warn "仍有进程在运行，强制杀死..."
        pkill -9 -f "weisyn\|testing" 2>/dev/null || true
        sleep 2
    fi
    
    # 1.2 删除所有旧二进制
    log_info "删除所有旧二进制文件..."
    rm -f "${PROJECT_ROOT}/bin/testing" "${PROJECT_ROOT}/bin/weisyn" "${PROJECT_ROOT}/bin/development" 2>/dev/null || true
    
    # 1.3 清理data/testing目录（区块、数据库、日志等）
    log_info "清理data/testing目录（区块、数据库、日志等）..."
    if [ -d "${PROJECT_ROOT}/data/testing" ]; then
        log_info "删除data/testing目录（大小: $(du -sh "${PROJECT_ROOT}/data/testing" 2>/dev/null | cut -f1 || echo '未知')）..."
        rm -rf "${PROJECT_ROOT}/data/testing" 2>/dev/null || true
    fi
    # 重新创建测试日志目录（归集到 data/testing/logs/onnx_test_logs）
    mkdir -p "${PROJECT_ROOT}/data/testing/logs/onnx_test_logs"
    
    # 1.3.1 清理历史遗留的全局目录（data/files 和 data/logs）
    # 注意：这些是历史遗留的全局目录，测试环境应该使用 data/testing/files 和 data/testing/logs
    log_info "清理历史遗留的全局目录..."
    if [ -d "${PROJECT_ROOT}/data/files" ]; then
        log_info "删除data/files目录（大小: $(du -sh "${PROJECT_ROOT}/data/files" 2>/dev/null | cut -f1 || echo '未知')）..."
        rm -rf "${PROJECT_ROOT}/data/files" 2>/dev/null || true
    fi
    if [ -d "${PROJECT_ROOT}/data/logs" ]; then
        log_info "删除data/logs目录（大小: $(du -sh "${PROJECT_ROOT}/data/logs" 2>/dev/null | cut -f1 || echo '未知')）..."
        rm -rf "${PROJECT_ROOT}/data/logs" 2>/dev/null || true
    fi
    
    # 1.4 删除所有旧日志文件
    log_info "删除所有旧日志文件..."
    rm -f "$BUILD_LOG" 2>/dev/null || true
    
    # 1.5 清理ONNX库文件（强制重新提取）
    log_info "清理ONNX库文件（强制重新提取）..."
    rm -rf ~/.weisyn/libs 2>/dev/null || true
    mkdir -p ~/.weisyn/libs
    
    # 1.6 验证清理完成
    if [ -f "${PROJECT_ROOT}/bin/testing" ]; then
        log_error "❌ 旧二进制文件仍存在！"
        exit 1
    fi
    
    if pgrep -f "weisyn\|testing" >/dev/null 2>&1; then
        log_error "❌ 仍有进程在运行！"
        exit 1
    fi
    
    log_info "✅ 清理完成"
    echo ""
}

# 步骤2: 确保ONNX库文件存在
ensure_onnx_libs() {
    log_step "步骤 2/5: 确保ONNX库文件存在"
    
    bash "${PROJECT_ROOT}/scripts/build/ensure_onnx_libs.sh" || {
        log_error "❌ ONNX库文件准备失败"
        exit 1
    }
    
    log_info "✅ ONNX库文件已就绪"
    echo ""
}

# 步骤3: 重新构建
rebuild() {
    log_step "步骤 3/5: 重新构建二进制文件"
    
    log_info "开始构建..."
    if go build -ldflags "-X main.Environment=testing" -o "${PROJECT_ROOT}/bin/testing" ./cmd/weisyn > "$BUILD_LOG" 2>&1; then
        log_info "✅ 构建成功"
    else
        log_error "❌ 构建失败，查看构建日志:"
        cat "$BUILD_LOG"
        exit 1
    fi
    
    # 验证二进制文件存在
    if [ ! -f "${PROJECT_ROOT}/bin/testing" ]; then
        log_error "❌ 二进制文件不存在！"
        exit 1
    fi
    
    # 显示二进制文件信息
    log_info "二进制文件信息:"
    ls -lh "${PROJECT_ROOT}/bin/testing"
    file "${PROJECT_ROOT}/bin/testing"
    echo ""
}

# 步骤4: 启动节点
start_node() {
    log_step "步骤 4/5: 启动节点"
    
    log_info "启动节点..."
    "${PROJECT_ROOT}/bin/testing" --daemon --env testing > "$NODE_LOG" 2>&1 &
    NODE_PID=$!
    
    log_info "节点PID: $NODE_PID"
    
    # 等待节点启动
    log_info "等待节点启动（最多30秒）..."
    for i in {1..30}; do
        if curl -sf http://localhost:28680/api/v1/health/live >/dev/null 2>&1; then
            log_info "✅ 节点已启动（等待了 ${i} 秒）"
            echo ""
            return 0
        fi
        sleep 1
        echo -n "."
    done
    echo ""
    
    log_error "❌ 节点启动超时"
    log_error "节点日志（最后50行）:"
    tail -50 "$NODE_LOG" 2>/dev/null || true
    exit 1
}

# 步骤5: 运行测试
run_test() {
    local model_name="${1:-}"
    
    log_step "步骤 5/5: 运行ONNX模型测试"
    if [[ -n "${model_name}" ]]; then
        log_info "测试模型: $model_name"
        bash "${PROJECT_ROOT}/scripts/testing/models/onnx_models_test.sh" "$model_name" 2>&1 | tee /tmp/test_output.txt
    else
        log_info "测试所有模型"
        bash "${PROJECT_ROOT}/scripts/testing/models/onnx_models_test.sh" 2>&1 | tee /tmp/test_output.txt
    fi
    
    echo ""
}

# 主函数
main() {
    local model_name="${1:-}"
    
    echo ""
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "ONNX环境重建+测试流程"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    if [[ -n "${model_name}" ]]; then
        log_info "目标模型: ${model_name}"
    else
        log_info "测试模式: 批量测试所有模型"
    fi
    echo ""
    
    cleanup_all
    ensure_onnx_libs
    rebuild
    start_node
    run_test "$model_name"
    
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "✅ 测试流程完成"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    log_info "相关日志文件:"
    log_info "  - 节点日志: $NODE_LOG"
    log_info "  - 构建日志: $BUILD_LOG"
    log_info "  - 测试输出: /tmp/test_output.txt"
    log_info "  - 测试报告: ${PROJECT_ROOT}/data/testing/logs/onnx_test_logs/"
    echo ""
}

# 执行主函数
main "$@"

