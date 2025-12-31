#!/bin/bash
# 配置文件加载集成测试脚本
# 用于测试新的配置模型（Environment × ChainMode）

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BIN_DIR="${PROJECT_ROOT}/bin"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查二进制文件是否存在
if [ ! -f "${BIN_DIR}/weisyn" ]; then
    log_error "二进制文件不存在: ${BIN_DIR}/weisyn"
    log_info "请先编译: go build -o ${BIN_DIR}/weisyn ./cmd/weisyn"
    exit 1
fi

log_info "开始配置文件加载测试..."

# 测试 1: dev-private 配置
log_info "测试 1: dev-private 配置..."
if "${BIN_DIR}/weisyn" --config "${PROJECT_ROOT}/configs/networks/dev-private-local.json" --version > /dev/null 2>&1; then
    log_success "dev-private 配置加载成功"
else
    log_error "dev-private 配置加载失败"
    exit 1
fi

# 测试 2: prod-public 配置
log_info "测试 2: prod-public 配置..."
if "${BIN_DIR}/weisyn" --config "${PROJECT_ROOT}/configs/networks/prod-public-mainnet.json" --version > /dev/null 2>&1; then
    log_success "prod-public 配置加载成功"
else
    log_error "prod-public 配置加载失败"
    exit 1
fi

# 测试 3: test-consortium 配置
log_info "测试 3: test-consortium 配置..."
if "${BIN_DIR}/weisyn" --config "${PROJECT_ROOT}/configs/networks/test-consortium-demo.json" --version > /dev/null 2>&1; then
    log_success "test-consortium 配置加载成功"
else
    log_error "test-consortium 配置加载失败"
    exit 1
fi

# 测试 4: --profile 参数（如果文件存在）
log_info "测试 4: --profile 参数..."
if [ -f "${PROJECT_ROOT}/configs/networks/dev-private-local.json" ]; then
    if "${BIN_DIR}/weisyn" --profile dev-private-local --version > /dev/null 2>&1; then
        log_success "--profile dev-private-local 配置加载成功"
    else
        log_error "--profile dev-private-local 配置加载失败"
        exit 1
    fi
else
    log_info "跳过 --profile 测试（配置文件不存在）"
fi

# 测试 5: 向后兼容 --env 参数
log_info "测试 5: 向后兼容 --env 参数..."
if "${BIN_DIR}/weisyn" --env testing --version > /dev/null 2>&1; then
    log_success "--env testing 参数工作正常（向后兼容）"
else
    log_error "--env testing 参数失败"
    exit 1
fi

# 测试 6: 无效配置文件应失败
log_info "测试 6: 无效配置文件应失败..."
if "${BIN_DIR}/weisyn" --config "/nonexistent/config.json" --version > /dev/null 2>&1; then
    log_error "无效配置文件应该失败，但没有失败"
    exit 1
else
    log_success "无效配置文件正确失败"
fi

log_success "所有配置文件加载测试通过！"

