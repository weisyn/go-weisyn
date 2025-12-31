#!/usr/bin/env bash
# 统一交易结构验证工具
# 用途：验证执行型交易（合约/模型调用）的结构是否符合统一协议
# 支持多种验证模式：单个交易、扫描区块、验证交易对、从测试报告读取
#
# 用法：
#   verify_tx_structure.sh <tx_hash>                    # 验证单个交易
#   verify_tx_structure.sh --scan                       # 扫描最近区块
#   verify_tx_structure.sh <deploy_tx> <call_tx>        # 验证部署+调用交易对
#   verify_tx_structure.sh --from-report                # 从最新测试报告读取

set -eu

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
API_URL="http://localhost:28680/jsonrpc"

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✅]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[⚠️]${NC} $1"; }
log_error() { echo -e "${RED}[❌]${NC} $1"; }
log_test() { echo -e "${CYAN}[🧪]${NC} $1"; }

jsonrpc_call() {
    local method="$1"
    local params="$2"
    curl -s -X POST "${API_URL}" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"${method}\",\"params\":${params},\"id\":1}" 2>/dev/null
}

# 验证单个交易的结构
verify_single_tx() {
    local tx_hash="$1"
    local tx_name="${2:-交易}"
    
    log_test "验证 $tx_name 结构: $tx_hash"
    
    # 1. 获取交易详情
    local tx_resp
    tx_resp=$(jsonrpc_call "wes_getTransactionByHash" "[\"$tx_hash\"]")
    
    if echo "$tx_resp" | grep -q '"error"'; then
        log_error "无法获取交易: $(echo "$tx_resp" | jq -r '.error.data // .error.message' 2>/dev/null)"
        return 1
    fi
    
    local status
    status=$(echo "$tx_resp" | jq -r '.result.status // "unknown"' 2>/dev/null)
    local block_height
    block_height=$(echo "$tx_resp" | jq -r '.result.blockHeight // "0x0"' 2>/dev/null)
    
    log_info "   状态: $status"
    log_info "   区块高度: $block_height"
    
    # 2. 检查是否已打包
    if [[ "$status" != "confirmed" ]] && ([[ "$block_height" == "0x0" ]] || [[ "$block_height" == "null" ]]); then
        log_warning "   交易尚未打包进区块（status=$status, blockHeight=$block_height）"
        return 2
    fi
    
    log_success "   交易已打包，区块高度: $block_height"
    
    # 3. 检查交易结构
    local inputs_count
    inputs_count=$(echo "$tx_resp" | jq '[.result.inputs[]?] | length' 2>/dev/null || echo "0")
    local ref_inputs_count
    ref_inputs_count=$(echo "$tx_resp" | jq '[.result.inputs[]? | select(.is_reference_only == true)] | length' 2>/dev/null || echo "0")
    local has_zk_proof
    has_zk_proof=$(echo "$tx_resp" | jq '.result.outputs[]?.state?.zk_proof != null' 2>/dev/null || echo "false")
    
    log_info "   交易结构: inputs=$inputs_count, ref_inputs=$ref_inputs_count, has_zk_proof=$has_zk_proof"
    
    # 4. 验证协议约束
    local errors=0
    
    if [[ "$inputs_count" -lt 1 ]]; then
        log_error "   ❌ 违反协议：执行型交易必须至少包含1个输入"
        errors=$((errors + 1))
    fi
    
    if [[ "$ref_inputs_count" -lt 1 ]]; then
        log_error "   ❌ 违反协议：执行型交易必须至少包含1个 is_reference_only=true 的资源引用输入"
        errors=$((errors + 1))
    fi
    
    if [[ "$has_zk_proof" != "true" ]]; then
        log_error "   ❌ 违反协议：执行型交易必须包含 StateOutput.zk_proof"
        errors=$((errors + 1))
    fi
    
    if [[ $errors -eq 0 ]]; then
        log_success "   ✅ 结构检查通过：满足统一"可执行资源交易"协议"
        
        # 5. 详细检查引用输入
        log_info "   详细检查引用输入..."
        local ref_inputs
        ref_inputs=$(echo "$tx_resp" | jq '[.result.inputs[]? | select(.is_reference_only == true)]' 2>/dev/null)
        local ref_count
        ref_count=$(echo "$ref_inputs" | jq 'length' 2>/dev/null || echo "0")
        
        if [[ "$ref_count" -gt 0 ]]; then
            for i in $(seq 0 $((ref_count - 1))); do
                local prev_tx_id
                prev_tx_id=$(echo "$ref_inputs" | jq -r ".[$i].previous_output.tx_id // empty" 2>/dev/null)
                local output_idx
                output_idx=$(echo "$ref_inputs" | jq -r ".[$i].previous_output.output_index // 0" 2>/dev/null)
                
                if [[ -n "$prev_tx_id" ]]; then
                    log_info "     引用输入[$i]: output_index=$output_idx"
                    
                    # 检查引用的UTXO是否为ResourceOutput
                    local prev_tx_resp
                    prev_tx_resp=$(jsonrpc_call "wes_getTransactionByHash" "[\"$prev_tx_id\"]" 2>/dev/null || echo "{}")
                    local has_resource_output
                    has_resource_output=$(echo "$prev_tx_resp" | jq ".result.outputs[$output_idx]?.resource != null" 2>/dev/null || echo "false")
                    
                    if [[ "$has_resource_output" == "true" ]]; then
                        log_success "     ✅ 引用的UTXO是ResourceOutput（符合协议）"
                    else
                        log_warning "     ⚠️  无法确认引用的UTXO类型（可能需要进一步检查）"
                    fi
                fi
            done
        fi
        
        # 6. 检查解锁证明（合约使用ExecutionProof，模型使用SingleKeyProof）
        log_info "   检查解锁证明..."
        local has_execution_proof
        has_execution_proof=$(echo "$tx_resp" | jq '.result.inputs[]? | select(.is_reference_only == true) | .execution_proof != null' 2>/dev/null || echo "false")
        local has_single_key_proof
        has_single_key_proof=$(echo "$tx_resp" | jq '.result.inputs[]? | select(.is_reference_only == true) | .single_key_proof != null' 2>/dev/null || echo "false")
        
        if [[ "$has_execution_proof" == "true" ]]; then
            log_success "   ✅ 引用输入包含 ExecutionProof（合约调用）"
        elif [[ "$has_single_key_proof" == "true" ]]; then
            log_success "   ✅ 引用输入包含 SingleKeyProof（模型调用）"
        else
            log_warning "   ⚠️  引用输入未检测到解锁证明（可能使用其他方式）"
        fi
        
        return 0
    else
        log_error "   ❌ 结构检查失败：发现 $errors 个协议违反"
        return 1
    fi
}

# 扫描最近区块，查找执行型交易
scan_recent_blocks() {
    log_test "扫描最近区块，查找执行型交易..."
    
    local found_txs=0
    local latest_hex
    latest_hex=$(jsonrpc_call "wes_blockNumber" "[]" | jq -r '.result' 2>/dev/null)
    log_info "当前区块高度: $latest_hex"
    
    local latest_dec
    latest_dec=$(( $(echo "$latest_hex" | sed 's/0x//' | tr '[:lower:]' '[:upper:]' | xargs -I {} echo "ibase=16; {}" | bc 2>/dev/null || echo "0") ))
    
    # 检查最近5个区块
    for i in {0..4}; do
        local check_height=$((latest_dec - i))
        if [[ $check_height -lt 0 ]]; then
            continue
        fi
        
        local check_hex
        check_hex=$(printf "0x%x" $check_height)
        
        local block_resp
        block_resp=$(jsonrpc_call "wes_getBlockByHeight" "[\"$check_hex\"]")
        
        if echo "$block_resp" | grep -q '"error"'; then
            continue
        fi
        
        local tx_hashes
        tx_hashes=$(echo "$block_resp" | jq -r '.result.transactions[]?.hash // .result.transactions[]? | select(type=="string")' 2>/dev/null | grep -v "^null$" | head -10)
        
        if [[ -z "$tx_hashes" ]]; then
            continue
        fi
        
        while IFS= read -r tx_hash; do
            if [[ -z "$tx_hash" ]] || [[ "$tx_hash" == "null" ]]; then
                continue
            fi
            
            # 检查是否是执行型交易（有StateOutput和ZKProof）
            local tx_check
            tx_check=$(jsonrpc_call "wes_getTransactionByHash" "[\"$tx_hash\"]")
            local has_state_zk
            has_state_zk=$(echo "$tx_check" | jq '.result.outputs[]?.state?.zk_proof != null' 2>/dev/null || echo "false")
            
            if [[ "$has_state_zk" == "true" ]]; then
                echo ""
                verify_single_tx "$tx_hash" "区块 $check_hex 中的执行型交易"
                found_txs=$((found_txs + 1))
            fi
        done <<< "$tx_hashes"
    done
    
    echo ""
    if [[ $found_txs -eq 0 ]]; then
        log_warning "未找到已打包的执行型交易（可能交易尚未被打包）"
        log_info "建议："
        log_info "1. 触发挖矿确保交易被打包"
        log_info "2. 等待几个区块后重新运行此脚本"
    else
        log_success "找到 $found_txs 个执行型交易，已全部验证"
    fi
}

# 验证部署+调用交易对
verify_tx_pair() {
    local deploy_tx="$1"
    local call_tx="$2"
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "验证部署+调用交易对"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    log_info "部署交易: $deploy_tx"
    log_info "调用交易: $call_tx"
    echo ""
    
    # 验证部署交易
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_test "步骤 1: 检查部署交易"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    local deploy_resp
    deploy_resp=$(jsonrpc_call "wes_getTransactionByHash" "[\"$deploy_tx\"]")
    
    if echo "$deploy_resp" | grep -q '"error"'; then
        log_error "查询部署交易失败"
        return 1
    fi
    
    local deploy_block_height
    deploy_block_height=$(echo "$deploy_resp" | jq -r '.result.blockHeight // "0x0"' 2>/dev/null)
    log_info "部署交易区块高度: $deploy_block_height"
    
    # 查找资源输出
    local resource_output_idx=-1
    local resource_content_hash=""
    local outputs
    outputs=$(echo "$deploy_resp" | jq -r '.result.outputs // []' 2>/dev/null)
    local output_count
    output_count=$(echo "$outputs" | jq 'length' 2>/dev/null || echo "0")
    
    for i in $(seq 0 $((output_count - 1))); do
        local output
        output=$(echo "$outputs" | jq -r ".[$i]" 2>/dev/null)
        if echo "$output" | grep -q '"resource"'; then
            resource_output_idx=$i
            resource_content_hash=$(echo "$output" | jq -r '.resource.content_hash // .resource.resource.content_hash // empty' 2>/dev/null)
            log_success "找到资源输出 [索引 $i]: content_hash=$resource_content_hash"
            break
        fi
    done
    
    if [[ $resource_output_idx -lt 0 ]]; then
        log_warning "未找到资源输出"
    fi
    
    echo ""
    
    # 验证调用交易
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    log_test "步骤 2: 检查调用交易（验证引用不消费）"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    verify_single_tx "$call_tx" "调用交易"
    
    # 检查调用交易是否引用了部署交易的输出
    local call_resp
    call_resp=$(jsonrpc_call "wes_getTransactionByHash" "[\"$call_tx\"]")
    local call_inputs
    call_inputs=$(echo "$call_resp" | jq -r '.result.inputs // []' 2>/dev/null)
    local found_reference=false
    
    for i in $(seq 0 $((output_count - 1))); do
        local input
        input=$(echo "$call_inputs" | jq -r ".[$i]" 2>/dev/null)
        local prev_tx_id
        prev_tx_id=$(echo "$input" | jq -r '.previous_output.tx_id // empty' 2>/dev/null)
        local is_reference_only
        is_reference_only=$(echo "$input" | jq -r '.is_reference_only // false' 2>/dev/null)
        
        if [[ "$prev_tx_id" == "$deploy_tx" ]] && [[ "$is_reference_only" == "true" ]]; then
            found_reference=true
            log_success "✅ 调用交易引用了部署交易的资源输出（只读引用，不消费）"
            break
        fi
    done
    
    if [[ "$found_reference" == "false" ]]; then
        log_warning "⚠️  未找到对部署交易的引用"
    fi
    
    echo ""
}

# 从测试报告读取交易哈希
read_from_report() {
    local report_dir="${PROJECT_ROOT}/data/testing/logs"
    local latest_report
    
    # 查找最新的合约测试报告
    latest_report=$(find "${report_dir}/contract_test_logs" -name "contract_test_*.txt" -type f 2>/dev/null | sort -r | head -1)
    
    if [[ -z "$latest_report" ]] || [[ ! -f "$latest_report" ]]; then
        log_error "未找到测试报告"
        log_info "请确保测试报告存在: ${report_dir}/contract_test_logs/"
        return 1
    fi
    
    log_info "从测试报告读取: $latest_report"
    
    local deploy_tx call_tx
    deploy_tx=$(grep "部署交易:" "$latest_report" | tail -1 | awk '{print $2}' || echo "")
    call_tx=$(grep "调用交易:" "$latest_report" | tail -1 | awk '{print $2}' || echo "")
    
    if [[ -z "$deploy_tx" ]] || [[ -z "$call_tx" ]]; then
        log_error "无法从测试报告中提取交易哈希"
        return 1
    fi
    
    verify_tx_pair "$deploy_tx" "$call_tx"
}

# 主函数
main() {
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🔍 统一交易结构验证工具"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    
    # 检查节点
    local node_check
    node_check=$(jsonrpc_call "wes_blockNumber" "[]" | jq -r '.result // empty' 2>/dev/null)
    if [[ -z "$node_check" ]] || [[ "$node_check" == "null" ]]; then
        log_error "节点未运行，请先启动节点"
        exit 1
    fi
    
    # 根据参数选择模式
    case "${1:-}" in
        --scan)
            scan_recent_blocks
            ;;
        --from-report)
            read_from_report
            ;;
        "")
            log_error "用法: $0 <tx_hash> | --scan | <deploy_tx> <call_tx> | --from-report"
            log_info "示例:"
            log_info "  $0 0x1234...                    # 验证单个交易"
            log_info "  $0 --scan                       # 扫描最近区块"
            log_info "  $0 0x1234... 0x5678...         # 验证部署+调用交易对"
            log_info "  $0 --from-report                # 从最新测试报告读取"
            exit 1
            ;;
        *)
            if [[ $# -eq 1 ]]; then
                # 单个交易验证
                verify_single_tx "$1"
            elif [[ $# -eq 2 ]]; then
                # 交易对验证
                verify_tx_pair "$1" "$2"
            else
                log_error "参数错误"
                exit 1
            fi
            ;;
    esac
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

main "$@"

