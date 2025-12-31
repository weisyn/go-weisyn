#!/usr/bin/env bash
# SDK 集成测试环境启动脚本
# 用途：启动 WES 节点，配置为 SDK 集成测试专用环境

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

# 检查依赖
check_dependencies() {
    log_info "检查依赖..."
    
    if ! command -v go &> /dev/null; then
        log_error "Go 未安装，请先安装 Go"
        exit 1
    fi
    
    log_success "依赖检查通过"
}

# 停止已运行的节点
stop_existing_nodes() {
    log_info "检查并停止已运行的节点..."
    
    local pids
    pids=$(pgrep -f 'weisyn.*--env testing' 2>/dev/null || true)
    
    if [[ -n "${pids}" ]]; then
        log_warning "发现运行中的节点进程: ${pids}"
        echo "${pids}" | xargs kill -TERM 2>/dev/null || true
        sleep 2
        
        # 强制停止
        pids=$(pgrep -f 'weisyn.*--env testing' 2>/dev/null || true)
        if [[ -n "${pids}" ]]; then
            log_warning "强制停止残留进程..."
            echo "${pids}" | xargs kill -9 2>/dev/null || true
            sleep 1
        fi
        
        log_success "已停止现有节点"
    else
        log_info "未发现运行中的节点"
    fi
}

# 检查端口是否被占用
check_ports() {
    log_info "检查端口占用情况..."
    
    local http_port=28680
    local ws_port=28681
    
    if lsof -Pi :${http_port} -sTCP:LISTEN -t >/dev/null 2>&1; then
        log_error "端口 ${http_port} 已被占用"
        log_info "请运行以下命令查看占用进程："
        log_info "  lsof -i :${http_port}"
        exit 1
    fi
    
    if lsof -Pi :${ws_port} -sTCP:LISTEN -t >/dev/null 2>&1; then
        log_error "端口 ${ws_port} 已被占用"
        log_info "请运行以下命令查看占用进程："
        log_info "  lsof -i :${ws_port}"
        exit 1
    fi
    
    log_success "端口检查通过"
}

# 创建 SDK 集成测试配置
create_sdk_config() {
    log_info "创建 SDK 集成测试配置..."
    
    local config_dir="${PROJECT_ROOT}/configs/testing"
    local sdk_config="${config_dir}/sdk-integration.json"
    
    # 如果配置文件已存在，询问是否覆盖
    if [[ -f "${sdk_config}" ]]; then
        log_warning "配置文件已存在: ${sdk_config}"
        read -p "是否覆盖现有配置？(y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "使用现有配置"
            return
        fi
    fi
    
    # 基于 testing/config.json 创建 SDK 集成测试配置
    # 主要修改：固定端口 28680 (HTTP) 和 28681 (WebSocket)
    cat > "${sdk_config}" << 'EOF'
{
  "_comment": "SDK 集成测试专用环境配置",
  "_environment": "testing",
  "_version": "0.0.1",
  
  "network": {
    "chain_id": 20001,
    "network_name": "WES_sdk_integration"
  },
  
  "genesis": {
    "timestamp": 1704067200,
    "accounts": [
      {
        "name": "SDK-Test-Miner",
        "private_key": "ae009e242a7317826396eafca13e4142aca5d8adbaf438682fa4779dc6e16323",
        "address": "CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR",
        "initial_balance": "1000000000000000000000"
      },
      {
        "name": "SDK-Test-User-A",
        "private_key": "e913d55e6487714c900fbfa2cc79dc6072f3da0486dcc5c4eba3555f00014598",
        "address": "CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG",
        "initial_balance": "1000000000000000000"
      },
      {
        "name": "SDK-Test-User-B",
        "private_key": "f913d55e6487714c900fbfa2cc79dc6072f3da0486dcc5c4eba3555f00014599",
        "address": "CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZH",
        "initial_balance": "0"
      }
    ]
  },
  
  "api": {
    "_comment": "SDK 集成测试固定端口配置",
    "http_enabled": true,
    "http_port": 28680,
    "http_enable_rest": true,
    "http_enable_jsonrpc": true,
    "http_enable_websocket": true,
    "grpc_enabled": false,
    "grpc_port": 50051,
    "enable_mining_api": true
  },
  
  "mining": {
    "_comment": "单节点开发模式 - 仅用于 SDK 集成测试",
    "target_block_time": "5s",
    "enable_aggregator": false,
    "max_mining_threads": 1
  },

  "blockchain": {
    "block": {
      "min_block_interval": 2
    }
  },
  
  "node": {
    "listen_addresses": [
      "/ip4/127.0.0.1/tcp/28683"
    ],
    "host": {
      "identity": {
        "key_file": "./p2p/identity.key"
      }
    },
    "bootstrap_peers": [],
    "enable_mdns": false,
    "enable_dht": false,
    "enable_nat_port": false,
    "enable_dcutr": false,
    "enable_auto_relay": false
  },
  
  "log": {
    "level": "info",
    "enable_multi_file": true,
    "system_log_file": "sdk-integration-system.log",
    "business_log_file": "sdk-integration-business.log"
  },
  
  "storage": {
    "data_root": "./data/testing/sdk-integration"
  },

  "test": {
    "cleanup_on_start": true,
    "keep_recent_logs": 10,
    "cleanup_wrong_locations": true,
    "single_node_mode": true
  },

  "signer": {
    "type": "local",
    "local": {
      "private_key_hex": "",
      "environment": "testing"
    }
  }
}
EOF
    
    log_success "配置文件已创建: ${sdk_config}"
}

# 启动节点
start_node() {
    log_info "启动 WES 节点..."
    
    local config_file="${PROJECT_ROOT}/configs/testing/sdk-integration.json"
    local log_file="${PROJECT_ROOT}/data/testing/sdk-integration/node.log"
    local pid_file="${PROJECT_ROOT}/data/testing/sdk-integration/node.pid"
    
    # 确保日志目录存在
    mkdir -p "$(dirname "${log_file}")"
    
    # 启动节点（后台运行）
    log_info "节点配置: ${config_file}"
    log_info "HTTP API: http://127.0.0.1:28680"
    log_info "WebSocket: ws://127.0.0.1:28681"
    log_info "日志文件: ${log_file}"
    
    # 使用 go run 启动（开发环境推荐）
    cd "${PROJECT_ROOT}"
    nohup go run cmd/weisyn/main.go --env testing --config "${config_file}" --daemon > "${log_file}" 2>&1 &
    local node_pid=$!
    
    # 保存 PID
    echo "${node_pid}" > "${pid_file}"
    
    log_success "节点已启动，PID: ${node_pid}"
    
    # 等待节点启动
    log_info "等待节点启动..."
    sleep 5
    
    # 验证节点是否运行
    if ! kill -0 "${node_pid}" 2>/dev/null; then
        log_error "节点启动失败，请查看日志: ${log_file}"
        exit 1
    fi
    
    # 检查健康状态
    local max_attempts=30
    local attempt=0
    while [[ ${attempt} -lt ${max_attempts} ]]; do
        if curl -s http://127.0.0.1:28680/health > /dev/null 2>&1; then
            log_success "节点健康检查通过"
            break
        fi
        attempt=$((attempt + 1))
        sleep 1
    done
    
    if [[ ${attempt} -eq ${max_attempts} ]]; then
        log_error "节点健康检查失败，请查看日志: ${log_file}"
        exit 1
    fi
}

# 导出环境变量
export_env_vars() {
    log_info "导出环境变量..."
    
    # 从配置文件读取账户私钥（实际应该从环境变量或密钥管理服务获取）
    # 这里使用示例私钥，实际使用时应该从安全的地方读取
    export WES_ENDPOINT_HTTP="http://127.0.0.1:28680"
    export WES_ENDPOINT_WS="ws://127.0.0.1:28681"
    
    # 预置账户私钥（示例，实际应该从安全存储读取）
    export WES_TEST_PRIVKEY_MINER="ae009e242a7317826396eafca13e4142aca5d8adbaf438682fa4779dc6e16323"
    export WES_TEST_PRIVKEY_USER_A="e913d55e6487714c900fbfa2cc79dc6072f3da0486dcc5c4eba3555f00014598"
    export WES_TEST_PRIVKEY_USER_B="f913d55e6487714c900fbfa2cc79dc6072f3da0486dcc5c4eba3555f00014599"
    
    log_success "环境变量已导出"
    log_info ""
    log_info "环境变量："
    log_info "  WES_ENDPOINT_HTTP=${WES_ENDPOINT_HTTP}"
    log_info "  WES_ENDPOINT_WS=${WES_ENDPOINT_WS}"
    log_info "  WES_TEST_PRIVKEY_MINER=***"
    log_info "  WES_TEST_PRIVKEY_USER_A=***"
    log_info "  WES_TEST_PRIVKEY_USER_B=***"
}

# 主函数
main() {
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info "SDK 集成测试环境启动"
    log_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info ""
    
    check_dependencies
    stop_existing_nodes
    check_ports
    create_sdk_config
    start_node
    export_env_vars
    
    log_info ""
    log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_success "SDK 集成测试环境启动完成！"
    log_success "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_info ""
    log_info "节点信息："
    log_info "  HTTP API: http://127.0.0.1:28680"
    log_info "  WebSocket: ws://127.0.0.1:28681"
    log_info ""
    log_info "停止节点："
    log_info "  ./scripts/sdk-integration/stop.sh"
    log_info ""
    log_info "查看日志："
    log_info "  tail -f data/testing/sdk-integration/node.log"
    log_info ""
}

# 执行主函数
main "$@"

