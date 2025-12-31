#!/bin/bash

# WES 测试网络协调清理脚本 - 增强版
# 支持网络节点发现、协调清理和状态验证
# 🎯 解决测试网络脏数据问题的全面解决方案

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 全局变量
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT_DIR="$(dirname "$0")"
LOG_FILE="${PROJECT_ROOT}/logs/network-cleanup-$(date +%Y%m%d-%H%M%S).log"
BACKUP_DIR="${PROJECT_ROOT}/backup/cleanup-backup-$(date +%Y%m%d-%H%M%S)"
CONFIG_FILE="${PROJECT_ROOT}/configs/development/config.json"
CLEANUP_SESSION_ID="cleanup-session-$(date +%Y%m%d-%H%M%S)"

# 确保日志目录存在
mkdir -p "$(dirname "$LOG_FILE")"

# 日志函数
log() {
    echo -e "$(date '+%Y-%m-%d %H:%M:%S') $1" | tee -a "$LOG_FILE"
}

log_info() {
    log "${BLUE}[INFO]${NC} $1"
}

log_warn() {
    log "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    log "${RED}[ERROR]${NC} $1"
}

log_success() {
    log "${GREEN}[SUCCESS]${NC} $1"
}

# 显示帮助信息
show_help() {
    cat << EOF
${CYAN}WES 测试网络协调清理工具${NC}

${YELLOW}用法:${NC}
  $0 [选项]

${YELLOW}选项:${NC}
  -h, --help           显示此帮助信息
  -f, --force          强制清理，跳过确认
  -b, --backup         清理前创建备份
  -n, --network        执行网络协调清理
  -c, --config FILE    指定配置文件 (默认: configs/development/config.json)
  -s, --session NAME   指定清理会话名称
  --dry-run            仅预览操作，不实际执行
  --discover-only      仅发现网络节点，不执行清理
  --local-only         仅清理本地，不协调网络
  --keep-height N      保留到指定区块高度 (默认: 0=完全清理)
  --api-port PORT      API端口 (默认: 28680)

${YELLOW}示例:${NC}
  $0 --force --backup                # 强制清理并备份
  $0 --network --session test-v1.2   # 协调网络清理，指定会话名
  $0 --dry-run --discover-only        # 预览模式，仅发现节点
  $0 --local-only --keep-height 100   # 仅本地清理，保留100个区块

${YELLOW}清理步骤:${NC}
  1. 发现网络中的节点
  2. 检查网络一致性状态
  3. 协调节点执行清理
  4. 验证清理结果
  5. 重启测试会话

EOF
}

# 解析命令行参数
parse_arguments() {
    FORCE_CLEANUP=false
    CREATE_BACKUP=false
    NETWORK_CLEANUP=false
    DRY_RUN=false
    DISCOVER_ONLY=false
    LOCAL_ONLY=false
    KEEP_HEIGHT=0
    API_PORT=28680
    SESSION_NAME=""

    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -f|--force)
                FORCE_CLEANUP=true
                shift
                ;;
            -b|--backup)
                CREATE_BACKUP=true
                shift
                ;;
            -n|--network)
                NETWORK_CLEANUP=true
                shift
                ;;
            -c|--config)
                CONFIG_FILE="$2"
                shift 2
                ;;
            -s|--session)
                SESSION_NAME="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --discover-only)
                DISCOVER_ONLY=true
                shift
                ;;
            --local-only)
                LOCAL_ONLY=true
                shift
                ;;
            --keep-height)
                KEEP_HEIGHT="$2"
                shift 2
                ;;
            --api-port)
                API_PORT="$2"
                shift 2
                ;;
            *)
                log_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done

    # 设置默认会话名
    if [[ -z "$SESSION_NAME" ]]; then
        SESSION_NAME="$CLEANUP_SESSION_ID"
    fi
}

# 检查依赖
check_dependencies() {
    log_info "检查依赖工具..."

    local missing_deps=()

    if ! command -v curl &> /dev/null; then
        missing_deps+=("curl")
    fi

    if ! command -v jq &> /dev/null; then
        missing_deps+=("jq")
    fi

    if ! command -v netstat &> /dev/null && ! command -v ss &> /dev/null; then
        missing_deps+=("netstat 或 ss")
    fi

    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        log_error "缺少必要依赖: ${missing_deps[*]}"
        log_error "请安装这些工具后重新运行"
        exit 1
    fi

    log_success "依赖检查通过"
}

# 检查节点API可达性
check_api_reachable() {
    local host="$1"
    local port="$2"
    local timeout="${3:-3}"

    curl -s --connect-timeout "$timeout" "http://${host}:${port}/health" > /dev/null 2>&1
}

# 发现网络节点
discover_network_nodes() {
    log_info "发现网络节点..."

    local discovered_nodes=()
    local api_base="http://localhost:${API_PORT}"

    # 检查本地API是否可达
    if check_api_reachable "localhost" "$API_PORT"; then
        log_info "本地节点API可达: ${api_base}"
        
        # 尝试通过内部管理API发现节点
        local response
        if response=$(curl -s "${api_base}/internal/test-network/nodes/discover" 2>/dev/null); then
            log_info "通过内部API发现节点:"
            echo "$response" | jq -r '.data.nodes[].peer_id' 2>/dev/null || true
        else
            log_warn "内部API不可用，使用传统方法发现节点"
        fi
    else
        log_warn "本地API不可达，尝试其他端口..."
        
        # 尝试其他常用端口
        for port in 28681 8082 8083 28682 9091; do
            if check_api_reachable "localhost" "$port"; then
                log_info "发现节点在端口 $port"
                discovered_nodes+=("localhost:$port")
            fi
        done
    fi

    # 扫描网络中的其他节点（基于配置文件）
    if [[ -f "$CONFIG_FILE" ]]; then
        log_info "从配置文件扫描引导节点: $CONFIG_FILE"
        # TODO: 解析配置文件中的bootstrap nodes
    fi

    # 输出发现结果
    if [[ ${#discovered_nodes[@]} -gt 0 ]]; then
        log_success "发现 ${#discovered_nodes[@]} 个网络节点:"
        for node in "${discovered_nodes[@]}"; do
            log_info "  - $node"
        done
    else
        log_warn "未发现其他网络节点"
    fi

    echo "${discovered_nodes[@]}"
}

# 检查网络一致性
check_network_consistency() {
    log_info "检查网络数据一致性..."

    local api_base="http://localhost:${API_PORT}"
    
    if ! check_api_reachable "localhost" "$API_PORT"; then
        log_error "无法连接到本地API，跳过一致性检查"
        return 1
    fi

    # 调用内部管理API进行一致性检查
    local response
    if response=$(curl -s "${api_base}/internal/test-network/consistency-check?depth=10" 2>/dev/null); then
        local inconsistencies
        inconsistencies=$(echo "$response" | jq -r '.data.inconsistencies | length' 2>/dev/null || echo "0")
        
        if [[ "$inconsistencies" -gt 0 ]]; then
            log_warn "发现 $inconsistencies 个数据一致性问题"
            echo "$response" | jq -r '.data.inconsistencies[].description' 2>/dev/null || true
            return 1
        else
            log_success "网络数据一致性检查通过"
            return 0
        fi
    else
        log_warn "一致性检查API调用失败"
        return 1
    fi
}

# 创建备份
create_backup() {
    if [[ "$CREATE_BACKUP" != "true" ]]; then
        return 0
    fi

    log_info "创建数据备份..."
    
    mkdir -p "$BACKUP_DIR"
    
    # 备份数据目录
    if [[ -d "${PROJECT_ROOT}/data" ]]; then
        log_info "备份数据目录..."
        cp -r "${PROJECT_ROOT}/data" "${BACKUP_DIR}/"
        log_success "数据目录已备份到: ${BACKUP_DIR}/data"
    fi
    
    # 备份配置文件
    if [[ -d "${PROJECT_ROOT}/configs" ]]; then
        log_info "备份配置文件..."
        cp -r "${PROJECT_ROOT}/configs" "${BACKUP_DIR}/"
        log_success "配置文件已备份到: ${BACKUP_DIR}/configs"
    fi

    # 创建备份清单
    cat > "${BACKUP_DIR}/backup-info.txt" << EOF
备份信息
================
备份时间: $(date)
会话ID: $SESSION_NAME
项目路径: $PROJECT_ROOT
备份原因: 测试网络清理
EOF

    log_success "备份完成: $BACKUP_DIR"
}

# 停止本地节点
stop_local_nodes() {
    log_info "停止本地节点进程..."

    # 查找并终止节点进程
    local pids
    pids=$(pgrep -f "bin/node" 2>/dev/null || true)
    
    if [[ -n "$pids" ]]; then
        log_info "发现运行中的节点进程: $pids"
        
        if [[ "$FORCE_CLEANUP" == "true" ]] || [[ "$DRY_RUN" == "true" ]]; then
            if [[ "$DRY_RUN" != "true" ]]; then
                kill $pids
                sleep 3
                # 强制终止仍在运行的进程
                if pgrep -f "bin/node" > /dev/null; then
                    log_warn "强制终止顽固进程..."
                    pkill -9 -f "bin/node" || true
                fi
            fi
            log_success "节点进程已停止"
        else
            log_warn "发现运行中的节点，请手动停止或使用 --force 参数"
            return 1
        fi
    else
        log_info "没有发现运行中的节点进程"
    fi
}

# 清理本地数据
cleanup_local_data() {
    log_info "清理本地数据..."

    local cleaned_items=()

    # 清理数据目录
    for dir in "data/badger" "data/logs" "data/p2p" "data/dht" "data_node2" "tmp"; do
        local full_path="${PROJECT_ROOT}/$dir"
        if [[ -d "$full_path" ]]; then
            if [[ "$DRY_RUN" == "true" ]]; then
                log_info "[预览] 将清理目录: $full_path"
            else
                rm -rf "$full_path"
                log_success "已清理目录: $full_path"
            fi
            cleaned_items+=("$dir")
        fi
    done

    # 清理临时文件
    for pattern in "*.log" "*.pid" "node.log" "/tmp/weisyn_*"; do
        if [[ "$DRY_RUN" == "true" ]]; then
            log_info "[预览] 将清理文件模式: $pattern"
        else
            rm -f $pattern 2>/dev/null || true
        fi
    done

    # 清理测试相关文件
    for dir in "test_data" "tmp_test" "test/reports"; do
        local full_path="${PROJECT_ROOT}/$dir"
        if [[ -d "$full_path" ]]; then
            if [[ "$DRY_RUN" == "true" ]]; then
                log_info "[预览] 将清理测试目录: $full_path"
            else
                rm -rf "$full_path"
                log_success "已清理测试目录: $full_path"
            fi
        fi
    done

    if [[ ${#cleaned_items[@]} -gt 0 ]]; then
        log_success "本地清理完成，清理了 ${#cleaned_items[@]} 个项目"
    else
        log_info "没有需要清理的本地数据"
    fi
}

# 协调网络清理
coordinate_network_cleanup() {
    if [[ "$NETWORK_CLEANUP" != "true" ]] || [[ "$LOCAL_ONLY" == "true" ]]; then
        return 0
    fi

    log_info "执行网络协调清理..."

    local api_base="http://localhost:${API_PORT}"
    
    if ! check_api_reachable "localhost" "$API_PORT"; then
        log_error "无法连接到本地API，跳过网络协调清理"
        return 1
    fi

    # 构建重置请求
    local reset_request=$(cat << EOF
{
    "reset_id": "$SESSION_NAME",
    "reset_height": $KEEP_HEIGHT,
    "reset_reason": "测试网络协调清理",
    "force": $FORCE_CLEANUP
}
EOF
)

    log_info "广播网络重置消息..."
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "[预览] 将发送网络重置请求:"
        echo "$reset_request" | jq . 2>/dev/null || echo "$reset_request"
    else
        local response
        if response=$(curl -s -X POST \
            -H "Content-Type: application/json" \
            -d "$reset_request" \
            "${api_base}/internal/test-network/broadcast-reset" 2>/dev/null); then
            
            local success
            success=$(echo "$response" | jq -r '.success' 2>/dev/null || echo "false")
            
            if [[ "$success" == "true" ]]; then
                log_success "网络重置消息广播成功"
                
                # 显示广播统计
                local stats
                stats=$(echo "$response" | jq -r '.data.broadcast_stats' 2>/dev/null || echo "{}")
                if [[ "$stats" != "{}" ]]; then
                    local success_count failed_count
                    success_count=$(echo "$stats" | jq -r '.success' 2>/dev/null || echo "0")
                    failed_count=$(echo "$stats" | jq -r '.failed' 2>/dev/null || echo "0")
                    log_info "广播统计: 成功 $success_count，失败 $failed_count"
                fi
            else
                local error_msg
                error_msg=$(echo "$response" | jq -r '.error' 2>/dev/null || echo "未知错误")
                log_error "网络重置消息广播失败: $error_msg"
                return 1
            fi
        else
            log_error "网络重置API调用失败"
            return 1
        fi
    fi
}

# 验证清理结果
verify_cleanup_result() {
    log_info "验证清理结果..."

    local verification_passed=true

    # 检查进程状态
    if pgrep -f "bin/node" > /dev/null; then
        log_warn "仍有节点进程在运行"
        verification_passed=false
    else
        log_success "没有节点进程运行"
    fi

    # 检查数据目录
    local remaining_data=()
    for dir in "data/badger" "data/logs" "data/p2p" "data/dht"; do
        local full_path="${PROJECT_ROOT}/$dir"
        if [[ -d "$full_path" ]] && [[ -n "$(ls -A "$full_path" 2>/dev/null)" ]]; then
            remaining_data+=("$dir")
        fi
    done

    if [[ ${#remaining_data[@]} -gt 0 ]]; then
        log_warn "仍有数据目录包含文件: ${remaining_data[*]}"
        verification_passed=false
    else
        log_success "数据目录清理完成"
    fi

    # 网络状态验证
    if [[ "$NETWORK_CLEANUP" == "true" ]] && [[ "$LOCAL_ONLY" != "true" ]]; then
        sleep 5  # 等待网络状态稳定
        if check_network_consistency; then
            log_success "网络一致性验证通过"
        else
            log_warn "网络一致性验证失败"
            verification_passed=false
        fi
    fi

    if [[ "$verification_passed" == "true" ]]; then
        log_success "清理结果验证通过"
        return 0
    else
        log_warn "清理结果验证存在问题"
        return 1
    fi
}

# 生成清理报告
generate_cleanup_report() {
    local report_file="${PROJECT_ROOT}/logs/cleanup-report-$(date +%Y%m%d-%H%M%S).md"
    
    cat > "$report_file" << EOF
# WES 测试网络清理报告

## 基本信息
- **会话ID**: $SESSION_NAME
- **执行时间**: $(date)
- **执行模式**: $(if [[ "$DRY_RUN" == "true" ]]; then echo "预览模式"; else echo "实际执行"; fi)
- **清理类型**: $(if [[ "$NETWORK_CLEANUP" == "true" ]]; then echo "网络协调清理"; else echo "本地清理"; fi)

## 执行参数
- 强制清理: $FORCE_CLEANUP
- 创建备份: $CREATE_BACKUP
- 保留高度: $KEEP_HEIGHT
- API端口: $API_PORT

## 清理结果
$(if [[ "$CREATE_BACKUP" == "true" ]]; then echo "- ✅ 备份已创建: $BACKUP_DIR"; fi)
- ✅ 本地数据已清理
$(if [[ "$NETWORK_CLEANUP" == "true" ]]; then echo "- ✅ 网络重置消息已广播"; fi)

## 后续步骤
1. 重新构建项目: \`./scripts/build.sh\`
2. 启动节点: \`./bin/node --config configs/development/config.json\`
3. 验证网络状态: 检查节点连接和区块同步

## 日志文件
- 详细日志: $LOG_FILE
$(if [[ "$CREATE_BACKUP" == "true" ]]; then echo "- 备份信息: $BACKUP_DIR/backup-info.txt"; fi)

---
报告生成时间: $(date)
EOF

    log_success "清理报告已生成: $report_file"
}

# 主执行函数
main() {
    cd "$PROJECT_ROOT"
    
    # 显示标题
    echo -e "${CYAN}"
    echo "████████████████████████████████████████████████████████"
    echo "       🧹 WES 测试网络协调清理工具 v2.0"
    echo "████████████████████████████████████████████████████████"
    echo -e "${NC}"
    
    log_info "开始执行测试网络清理..."
    log_info "会话ID: $SESSION_NAME"
    log_info "项目根目录: $PROJECT_ROOT"
    
    # 检查依赖
    check_dependencies
    
    # 仅发现模式
    if [[ "$DISCOVER_ONLY" == "true" ]]; then
        discover_network_nodes
        log_success "节点发现完成"
        exit 0
    fi
    
    # 发现网络节点
    local network_nodes
    network_nodes=$(discover_network_nodes)
    
    # 检查网络一致性
    if [[ "$NETWORK_CLEANUP" == "true" ]] && [[ "$LOCAL_ONLY" != "true" ]]; then
        check_network_consistency || log_warn "网络存在一致性问题，建议执行清理"
    fi
    
    # 确认执行（非强制模式）
    if [[ "$FORCE_CLEANUP" != "true" ]] && [[ "$DRY_RUN" != "true" ]]; then
        echo
        log_warn "即将执行以下操作:"
        echo -e "  ${YELLOW}•${NC} 停止本地节点进程"
        echo -e "  ${YELLOW}•${NC} 清理本地数据目录"
        if [[ "$CREATE_BACKUP" == "true" ]]; then
            echo -e "  ${YELLOW}•${NC} 创建数据备份"
        fi
        if [[ "$NETWORK_CLEANUP" == "true" ]]; then
            echo -e "  ${YELLOW}•${NC} 协调网络节点清理"
        fi
        echo
        read -p "$(echo -e ${YELLOW}确认继续执行？[y/N]: ${NC})" -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "操作已取消"
            exit 0
        fi
    fi
    
    # 执行清理步骤
    create_backup
    stop_local_nodes
    cleanup_local_data
    coordinate_network_cleanup
    
    # 等待清理完成
    if [[ "$DRY_RUN" != "true" ]]; then
        log_info "等待清理操作完成..."
        sleep 3
    fi
    
    # 验证结果
    if [[ "$DRY_RUN" != "true" ]]; then
        verify_cleanup_result
    fi
    
    # 生成报告
    generate_cleanup_report
    
    # 完成提示
    echo
    log_success "测试网络清理完成！"
    echo
    if [[ "$DRY_RUN" == "true" ]]; then
        echo -e "${CYAN}这是预览模式，没有实际执行清理操作${NC}"
    else
        echo -e "${GREEN}网络已重置为干净状态，可以开始新的测试${NC}"
    fi
    echo
    echo -e "${YELLOW}下一步操作:${NC}"
    echo -e "  1. 重新构建: ${CYAN}./scripts/build.sh${NC}"
    echo -e "  2. 启动节点: ${CYAN}./bin/node --config configs/development/config.json${NC}"
    echo -e "  3. 验证状态: ${CYAN}curl http://localhost:28680/health${NC}"
    echo
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    parse_arguments "$@"
    main
fi
