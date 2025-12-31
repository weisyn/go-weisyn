#!/usr/bin/env bash
# 诊断交易打包问题
# 用途：检查为什么执行型交易没有被打包进区块

set -eu

API_URL="http://localhost:28680/jsonrpc"
MINER_ADDR="CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR"

log_info() { echo -e "\033[0;34m[INFO]\033[0m $1"; }
log_success() { echo -e "\033[0;32m[✅]\033[0m $1"; }
log_warning() { echo -e "\033[1;33m[⚠️]\033[0m $1"; }
log_error() { echo -e "\033[0;31m[❌]\033[0m $1"; }

jsonrpc_call() {
    local method="$1"
    local params="$2"
    curl -s -X POST "${API_URL}" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"${method}\",\"params\":${params},\"id\":1}" 2>/dev/null
}

diagnose_tx() {
    local tx_hash="$1"
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "诊断交易: $tx_hash"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # 1. 检查交易是否存在
    local tx_resp
    tx_resp=$(jsonrpc_call "wes_getTransactionByHash" "[\"$tx_hash\"]")
    
    if echo "$tx_resp" | grep -q '"error"'; then
        log_error "交易不存在或查询失败"
        echo "$tx_resp" | jq '.' 2>/dev/null
        return 1
    fi
    
    local status
    status=$(echo "$tx_resp" | jq -r '.result.status // "unknown"' 2>/dev/null)
    local block_height
    block_height=$(echo "$tx_resp" | jq -r '.result.blockHeight // "0x0"' 2>/dev/null)
    
    log_info "交易状态: $status"
    log_info "区块高度: $block_height"
    
    # 2. 检查交易结构
    local inputs_count
    inputs_count=$(echo "$tx_resp" | jq '[.result.inputs[]?] | length' 2>/dev/null || echo "0")
    local ref_inputs_count
    ref_inputs_count=$(echo "$tx_resp" | jq '[.result.inputs[]? | select(.is_reference_only == true)] | length' 2>/dev/null || echo "0")
    local has_zk_proof
    has_zk_proof=$(echo "$tx_resp" | jq '.result.outputs[]?.state?.zk_proof != null' 2>/dev/null || echo "false")
    
    log_info "交易结构: inputs=$inputs_count, ref_inputs=$ref_inputs_count, has_zk_proof=$has_zk_proof"
    
    # 3. 检查是否是执行型交易
    if [[ "$has_zk_proof" == "true" ]]; then
        log_info "这是执行型交易（包含 ZKStateProof）"
        
        if [[ "$inputs_count" -lt 1 ]]; then
            log_error "❌ 交易缺少输入（可能被 ExecResourceInvariantPlugin 拒绝）"
        fi
        
        if [[ "$ref_inputs_count" -lt 1 ]]; then
            log_error "❌ 交易缺少引用型输入（可能被 ExecResourceInvariantPlugin 拒绝）"
        fi
    fi
    
    # 4. 检查交易收据
    local receipt
    receipt=$(jsonrpc_call "wes_getTransactionReceipt" "[\"$tx_hash\"]")
    local receipt_block
    receipt_block=$(echo "$receipt" | jq -r '.result.blockHeight // "0x0"' 2>/dev/null)
    
    if [[ "$receipt_block" != "0x0" ]] && [[ "$receipt_block" != "null" ]]; then
        log_success "交易已打包，区块高度: $receipt_block"
        return 0
    fi
    
    log_warning "交易尚未打包"
    
    # 5. 检查当前区块高度和挖矿状态
    local current_height
    current_height=$(jsonrpc_call "wes_blockNumber" "[]" | jq -r '.result' 2>/dev/null)
    log_info "当前区块高度: $current_height"
    
    local mining_status
    mining_status=$(jsonrpc_call "wes_getMiningStatus" "[]" | jq -r '.result.status // "unknown"' 2>/dev/null)
    log_info "挖矿状态: $mining_status"
    
    # 6. 检查交易池（如果API支持）
    log_info "检查交易池状态..."
    local pool_status
    pool_status=$(jsonrpc_call "wes_txpool_status" "[]" 2>/dev/null)
    if ! echo "$pool_status" | grep -q '"error"'; then
        echo "$pool_status" | jq '.' 2>/dev/null
    else
        log_warning "无法获取交易池状态"
    fi
    
    return 1
}

main() {
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🔍 交易打包问题诊断"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # 诊断之前的调用交易
    if [[ $# -ge 1 ]]; then
        diagnose_tx "$1"
    else
        log_info "用法: $0 <tx_hash>"
        log_info "示例: $0 459da1415cb0aae970389ac78ed165d8a2309bdcb60b217b3d7566b18165310c"
    fi
    
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

main "$@"

