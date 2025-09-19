#!/usr/bin/env bash
# WES双节点交易测试自动化脚本
# 用途：自动化执行双节点交易测试流程，验证P2P网络下的交易处理

set -euo pipefail

# 脚本配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_LOG_DIR="$ROOT_DIR/data/testing/dual_node"
DATE=$(date +"%Y%m%d_%H%M%S")
TEST_LOG="$TEST_LOG_DIR/test_$DATE.log"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

# 测试账户配置
ACCOUNT1_ADDRESS="CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR"
ACCOUNT1_PRIVKEY="ae009e242a7317826396eafca13e4142aca5d8adbaf438682fa4779dc6e16323"
ACCOUNT2_ADDRESS="CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG"
ACCOUNT2_PRIVKEY="e913d55e6487714c900fbfa2cc79dc6072f3da0486dcc5c4eba3555f00014598"

# API端点配置
NODE1_API="http://localhost:8080"
NODE2_API="http://localhost:8082"
TRANSFER_AMOUNT="0.3"

# 初始化测试环境
init_test_env() {
    echo -e "${BLUE}🧪 WES双节点交易测试 - 初始化环境${NC}"
    echo "=============================================="
    
    # 创建测试日志目录
    mkdir -p "$TEST_LOG_DIR"
    
    # 初始化日志文件
    {
        echo "# WES双节点交易测试日志"
        echo "# 测试时间: $(date)"
        echo "# 测试脚本: $0"
        echo "# 测试目标: 验证双节点间交易创建、传播、挖矿和同步"
        echo "=============================================="
    } > "$TEST_LOG"
    
    echo -e "${GREEN}✅ 测试环境初始化完成${NC}"
    echo "测试日志: $TEST_LOG"
    echo
}

# 日志记录函数
log_info() {
    local message="$1"
    echo -e "${BLUE}[INFO]${NC} $message"
    echo "[$(date '+%H:%M:%S')] [INFO] $message" >> "$TEST_LOG"
}

log_success() {
    local message="$1"
    echo -e "${GREEN}[SUCCESS]${NC} $message"
    echo "[$(date '+%H:%M:%S')] [SUCCESS] $message" >> "$TEST_LOG"
}

log_error() {
    local message="$1"
    echo -e "${RED}[ERROR]${NC} $message"
    echo "[$(date '+%H:%M:%S')] [ERROR] $message" >> "$TEST_LOG"
}

log_warning() {
    local message="$1"
    echo -e "${YELLOW}[WARNING]${NC} $message"
    echo "[$(date '+%H:%M:%S')] [WARNING] $message" >> "$TEST_LOG"
}

# 检查API可用性
check_api_availability() {
    log_info "检查节点API可用性..."
    
    local node1_status=0
    local node2_status=0
    
    # 检查节点1
    if curl -sf "$NODE1_API/api/v1/health" >/dev/null 2>&1; then
        log_success "节点1 API可用 ($NODE1_API)"
    else
        log_error "节点1 API不可用 ($NODE1_API)"
        node1_status=1
    fi
    
    # 检查节点2  
    if curl -sf "$NODE2_API/api/v1/health" >/dev/null 2>&1; then
        log_success "节点2 API可用 ($NODE2_API)"
    else
        log_error "节点2 API不可用 ($NODE2_API)"
        node2_status=1
    fi
    
    if [[ $node1_status -ne 0 ]] || [[ $node2_status -ne 0 ]]; then
        log_error "节点API检查失败，请确保双节点集群已启动"
        echo "启动命令: ./scripts/deploy/start_development.sh"
        exit 1
    fi
    
    echo
}

# 查询账户余额
query_balance() {
    local node_api="$1"
    local address="$2" 
    local node_name="$3"
    
    local response
    if ! response=$(curl -sf "$node_api/api/v1/accounts/$address/balance" 2>>"$TEST_LOG"); then
        log_error "$node_name - 余额查询失败"
        return 1
    fi
    
    # 提取余额信息（简化版，实际可能需要jq）
    log_info "$node_name - $address 余额查询成功"
    echo "$response" >> "$TEST_LOG"
    echo "$response"
}

# 验证初始余额一致性
verify_initial_balance() {
    log_info "步骤1: 验证初始余额一致性"
    echo "----------------------------------------"
    
    log_info "查询节点1 - Account1余额..."
    local n1_balance1
    n1_balance1=$(query_balance "$NODE1_API" "$ACCOUNT1_ADDRESS" "节点1")
    
    log_info "查询节点2 - Account1余额..."
    local n2_balance1  
    n2_balance1=$(query_balance "$NODE2_API" "$ACCOUNT1_ADDRESS" "节点2")
    
    log_info "查询节点1 - Account2余额..."
    local n1_balance2
    n1_balance2=$(query_balance "$NODE1_API" "$ACCOUNT2_ADDRESS" "节点1")
    
    log_info "查询节点2 - Account2余额..."
    local n2_balance2
    n2_balance2=$(query_balance "$NODE2_API" "$ACCOUNT2_ADDRESS" "节点2") 
    
    # 简化验证：检查返回是否包含success: true
    if echo "$n1_balance1" | grep -q '"success": true' && 
       echo "$n2_balance1" | grep -q '"success": true' &&
       echo "$n1_balance2" | grep -q '"success": true' &&
       echo "$n2_balance2" | grep -q '"success": true'; then
        log_success "初始余额查询成功，需要人工验证一致性"
        echo -e "${YELLOW}请验证以下两个节点的余额数据是否一致:${NC}"
        echo "节点1 - Account1: $(echo "$n1_balance1" | head -1)"
        echo "节点2 - Account1: $(echo "$n2_balance1" | head -1)"
    else
        log_error "初始余额查询失败"
        return 1
    fi
    
    echo
    return 0
}

# 创建并签名交易
create_and_sign_transaction() {
    log_info "步骤2: 创建并签名转账交易"
    echo "----------------------------------------"
    
    # 创建转账交易
    log_info "创建转账交易: $ACCOUNT1_ADDRESS → $ACCOUNT2_ADDRESS ($TRANSFER_AMOUNT WES)"
    local create_payload="{
        \"sender_private_key\": \"$ACCOUNT1_PRIVKEY\",
        \"to_address\": \"$ACCOUNT2_ADDRESS\",
        \"amount\": \"$TRANSFER_AMOUNT\",
        \"token_id\": \"\",
        \"memo\": \"双节点测试转账-$(date +%H:%M:%S)\",
        \"options\": {}
    }"
    
    local create_response
    if ! create_response=$(curl -sf -X POST "$NODE1_API/api/v1/transactions/transfer" \
        -H "Content-Type: application/json" \
        -d "$create_payload" 2>>"$TEST_LOG"); then
        log_error "交易创建失败"
        return 1
    fi
    
    log_success "交易创建成功"
    echo "$create_response" >> "$TEST_LOG"
    
    # 提取transaction_hash（需要手动处理或使用jq）
    # 这里提供一个简化的提示
    echo -e "${YELLOW}请从以下响应中复制transaction_hash:${NC}"
    echo "$create_response"
    echo
    
    # 等待用户输入
    read -p "请粘贴transaction_hash: " TX_HASH
    if [[ -z "$TX_HASH" ]]; then
        log_error "未提供交易哈希"
        return 1
    fi
    
    log_info "收到交易哈希: $TX_HASH"
    
    # 签名交易
    log_info "对交易进行数字签名..."
    local sign_payload="{
        \"transaction_hash\": \"$TX_HASH\",
        \"private_key\": \"$ACCOUNT1_PRIVKEY\"
    }"
    
    local sign_response
    if ! sign_response=$(curl -sf -X POST "$NODE1_API/api/v1/transactions/sign" \
        -H "Content-Type: application/json" \
        -d "$sign_payload" 2>>"$TEST_LOG"); then
        log_error "交易签名失败"
        return 1
    fi
    
    log_success "交易签名成功"
    echo "$sign_response" >> "$TEST_LOG"
    
    echo -e "${YELLOW}请从以下响应中复制signed_tx_hash:${NC}"
    echo "$sign_response"
    echo
    
    # 等待用户输入签名后哈希
    read -p "请粘贴signed_tx_hash: " SIGNED_HASH
    if [[ -z "$SIGNED_HASH" ]]; then
        log_error "未提供签名后交易哈希"
        return 1
    fi
    
    log_info "收到签名后交易哈希: $SIGNED_HASH"
    echo "SIGNED_TX_HASH=$SIGNED_HASH" >> "$TEST_LOG"
    
    # 导出变量供后续步骤使用
    export SIGNED_TX_HASH="$SIGNED_HASH"
    
    echo
    return 0
}

# 提交交易并验证余额锁定
submit_and_verify_lock() {
    log_info "步骤3: 提交交易并验证余额锁定"
    echo "----------------------------------------"
    
    if [[ -z "${SIGNED_TX_HASH:-}" ]]; then
        log_error "未找到签名后交易哈希，请先执行交易创建和签名"
        return 1
    fi
    
    # 提交交易
    log_info "提交交易到节点1内存池..."
    local submit_payload="{\"signed_tx_hash\": \"$SIGNED_TX_HASH\"}"
    
    local submit_response
    if ! submit_response=$(curl -sf -X POST "$NODE1_API/api/v1/transactions/submit" \
        -H "Content-Type: application/json" \
        -d "$submit_payload" 2>>"$TEST_LOG"); then
        log_error "交易提交失败"
        return 1
    fi
    
    log_success "交易提交成功"
    echo "$submit_response" >> "$TEST_LOG"
    echo "$submit_response"
    
    # 等待余额锁定生效
    log_info "等待3秒让余额锁定生效..."
    sleep 3
    
    # 验证余额锁定
    log_info "验证Account1余额锁定状态..."
    local locked_balance
    locked_balance=$(query_balance "$NODE1_API" "$ACCOUNT1_ADDRESS" "节点1(锁定后)")
    
    echo -e "${YELLOW}预期结果: available应减少$TRANSFER_AMOUNT，locked应增加$TRANSFER_AMOUNT，total保持不变${NC}"
    echo -e "${BLUE}实际结果:${NC} $locked_balance"
    
    echo
    return 0
}

# 验证网络传播
verify_network_propagation() {
    log_info "步骤4: 验证交易网络传播"
    echo "----------------------------------------"
    
    # 等待交易传播
    log_info "等待10秒让交易传播到节点2..."
    sleep 10
    
    # 查询节点2的Account1余额状态
    log_info "检查节点2的Account1余额状态..."
    local n2_balance
    n2_balance=$(query_balance "$NODE2_API" "$ACCOUNT1_ADDRESS" "节点2(传播后)")
    
    echo -e "${YELLOW}验证要点: 节点2也应该显示相同的锁定状态${NC}"
    echo -e "${BLUE}节点2余额状态:${NC} $n2_balance"
    
    # 检查传播日志
    log_info "检查节点2的传播相关日志..."
    if [[ -f "$ROOT_DIR/data/logs/cluster-node2.log" ]]; then
        local propagation_logs
        propagation_logs=$(tail -50 "$ROOT_DIR/data/logs/cluster-node2.log" | grep -iE "(transaction|tx|broadcast|mempool)" | tail -10 || echo "未找到明确的传播日志")
        log_info "节点2传播相关日志:"
        echo "$propagation_logs"
        echo "$propagation_logs" >> "$TEST_LOG"
    else
        log_warning "未找到节点2日志文件"
    fi
    
    echo
    return 0
}

# 挖矿验证指导
mining_verification_guide() {
    log_info "步骤5: 挖矿和区块同步验证指导"
    echo "=========================================="
    
    echo -e "${PURPLE}挖矿验证阶段需要手动监控：${NC}"
    echo
    echo -e "${YELLOW}1. 监控挖矿进度:${NC}"
    echo "   tail -f $ROOT_DIR/data/logs/cluster-node2.log | grep -iE '(mining|block|nonce)'"
    echo
    echo -e "${YELLOW}2. 查询区块高度:${NC}"
    echo "   # 节点1: curl $NODE1_API/api/v1/blockchain/height"
    echo "   # 节点2: curl $NODE2_API/api/v1/blockchain/height"
    echo 
    echo -e "${YELLOW}3. 验证最终余额:${NC}"
    echo "   当新区块产生后，检查:"
    echo "   - Account1: available=0.7, locked=0, total=0.7"
    echo "   - Account2: available=1.3, locked=0, total=1.3"
    echo "   - 两节点余额数据完全一致"
    echo
    echo -e "${YELLOW}4. 手动验证命令:${NC}"
    echo "   curl $NODE1_API/api/v1/accounts/$ACCOUNT1_ADDRESS/balance"
    echo "   curl $NODE2_API/api/v1/accounts/$ACCOUNT1_ADDRESS/balance"
    echo "   curl $NODE1_API/api/v1/accounts/$ACCOUNT2_ADDRESS/balance" 
    echo "   curl $NODE2_API/api/v1/accounts/$ACCOUNT2_ADDRESS/balance"
    echo
    
    log_info "测试脚本的自动化部分已完成，请按上述指导手动验证挖矿和最终余额"
    echo
}

# 生成测试报告
generate_test_report() {
    local report_file="$TEST_LOG_DIR/test_report_$DATE.md"
    
    log_info "生成测试报告: $report_file"
    
    cat > "$report_file" << EOF
# WES双节点交易测试报告

**测试时间**: $(date)
**测试脚本**: $0
**测试日志**: $TEST_LOG

## 测试概览

本次测试验证了WES双节点环境下的完整交易流程，包括：
- 双节点API可用性
- 初始余额一致性
- 交易创建和签名
- 交易提交和余额锁定
- P2P网络传播
- 挖矿和区块同步（需手动验证）

## 测试配置

- **节点1**: $NODE1_API (交易请求方)
- **节点2**: $NODE2_API (矿工节点)
- **测试账户1**: $ACCOUNT1_ADDRESS
- **测试账户2**: $ACCOUNT2_ADDRESS
- **转账金额**: $TRANSFER_AMOUNT WES

## 自动化测试结果

$(grep -E "\[SUCCESS\]|\[ERROR\]" "$TEST_LOG" | sed 's/^/- /')

## 详细执行日志

\`\`\`
$(cat "$TEST_LOG")
\`\`\`

## 手动验证项目

### 挖矿验证
- [ ] 节点2成功挖出包含测试交易的区块
- [ ] 新区块在两节点间同步
- [ ] 区块高度一致

### 最终余额验证
- [ ] Account1余额: available=0.7, locked=0, total=0.7
- [ ] Account2余额: available=1.3, locked=0, total=1.3  
- [ ] 两节点余额数据完全一致

## 测试结论

自动化测试部分: **[待填写]**
手动验证部分: **[待填写]**

总体结论: **[待填写]**

---
*报告生成时间: $(date)*
EOF
    
    log_success "测试报告已生成: $report_file"
}

# 主测试流程
main() {
    # 初始化测试环境
    init_test_env
    
    # 检查API可用性
    check_api_availability
    
    # 执行测试步骤
    if verify_initial_balance && \
       create_and_sign_transaction && \
       submit_and_verify_lock && \
       verify_network_propagation; then
        
        log_success "自动化测试步骤执行完成"
        
        # 显示挖矿验证指导
        mining_verification_guide
        
        # 生成测试报告
        generate_test_report
        
        echo -e "${GREEN}🎉 双节点交易测试脚本执行完成！${NC}"
        echo -e "${BLUE}📋 请查看测试报告: $TEST_LOG_DIR/test_report_$DATE.md${NC}"
        echo -e "${YELLOW}⚠️  请继续手动验证挖矿和最终余额部分${NC}"
        
    else
        log_error "测试执行过程中出现错误，请检查日志"
        echo -e "${RED}❌ 测试失败，详细信息请查看: $TEST_LOG${NC}"
        return 1
    fi
}

# 脚本入口点
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
