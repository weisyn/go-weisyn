#!/usr/bin/env bash
# WES双节点快速健康检查脚本
# 用途：快速验证双节点集群的基础状态和连通性

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 节点配置
NODE1_API="http://localhost:8080"
NODE2_API="http://localhost:8082"
ACCOUNT1_ADDRESS="CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR"
ACCOUNT2_ADDRESS="CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG"

echo -e "${BLUE}🔍 WES双节点快速健康检查${NC}"
echo "=================================="
echo

# 检查函数
check_api() {
    local api_url="$1"
    local node_name="$2"
    
    if curl -sf "$api_url/api/v1/health" >/dev/null 2>&1; then
        echo -e "${GREEN}✅ $node_name API可用${NC} ($api_url)"
        return 0
    else
        echo -e "${RED}❌ $node_name API不可用${NC} ($api_url)"
        return 1
    fi
}

check_balance() {
    local api_url="$1"
    local address="$2"
    local node_name="$3"
    local account_name="$4"
    
    local response
    if response=$(curl -sf "$api_url/api/v1/accounts/$address/balance" 2>/dev/null); then
        # 简单提取available字段（假设格式相对稳定）
        local available=$(echo "$response" | grep -o '"available": [0-9]*' | grep -o '[0-9]*' || echo "unknown")
        local locked=$(echo "$response" | grep -o '"locked": [0-9]*' | grep -o '[0-9]*' || echo "unknown")
        local total=$(echo "$response" | grep -o '"total": [0-9]*' | grep -o '[0-9]*' || echo "unknown")
        
        echo -e "${GREEN}✅ $node_name - $account_name${NC}"
        echo "   Available: $available, Locked: $locked, Total: $total"
        return 0
    else
        echo -e "${RED}❌ $node_name - $account_name 余额查询失败${NC}"
        return 1
    fi
}

check_processes() {
    echo -e "${BLUE}📊 检查节点进程${NC}"
    local node_processes=$(ps aux | grep -E "bin/node|node.*config.*json" | grep -v grep | wc -l)
    if [[ $node_processes -ge 2 ]]; then
        echo -e "${GREEN}✅ 检测到 $node_processes 个节点进程${NC}"
        ps aux | grep -E "bin/node|node.*config.*json" | grep -v grep | while read line; do
            echo "   $line"
        done
    else
        echo -e "${YELLOW}⚠️  仅检测到 $node_processes 个节点进程${NC}"
        echo "   预期应该有2个节点进程运行"
    fi
    echo
}

check_ports() {
    echo -e "${BLUE}🔌 检查端口占用${NC}"
    local ports=(8080 8082 9090 9091 4001 4002)
    for port in "${ports[@]}"; do
        if lsof -i ":$port" >/dev/null 2>&1; then
            echo -e "${GREEN}✅ 端口 $port 被占用${NC}"
        else
            echo -e "${YELLOW}⚠️  端口 $port 未被占用${NC}"
        fi
    done
    echo
}

# 主检查流程
main() {
    # 检查进程
    check_processes
    
    # 检查端口
    check_ports
    
    # 检查API可用性
    echo -e "${BLUE}🌐 检查API可用性${NC}"
    local node1_ok=0
    local node2_ok=0
    
    if ! check_api "$NODE1_API" "节点1"; then
        node1_ok=1
    fi
    
    if ! check_api "$NODE2_API" "节点2"; then
        node2_ok=1
    fi
    
    echo
    
    # 如果API不可用，提供启动建议
    if [[ $node1_ok -ne 0 ]] || [[ $node2_ok -ne 0 ]]; then
        echo -e "${YELLOW}💡 启动建议${NC}"
        echo "如果节点未启动，请使用以下命令启动双节点集群:"
        echo "   ./scripts/deploy/start_development.sh"
        echo "   选择: 2) 双节点集群模式"
        echo
        echo "或者使用P2P专用脚本:"
        echo "   ./scripts/p2p/local_dual_node.sh"
        echo
        return 1
    fi
    
    # 检查余额一致性
    echo -e "${BLUE}💰 检查账户余额一致性${NC}"
    
    echo "Account1 ($ACCOUNT1_ADDRESS):"
    check_balance "$NODE1_API" "$ACCOUNT1_ADDRESS" "节点1" "Account1"
    check_balance "$NODE2_API" "$ACCOUNT1_ADDRESS" "节点2" "Account1"
    echo
    
    echo "Account2 ($ACCOUNT2_ADDRESS):"
    check_balance "$NODE1_API" "$ACCOUNT2_ADDRESS" "节点1" "Account2"  
    check_balance "$NODE2_API" "$ACCOUNT2_ADDRESS" "节点2" "Account2"
    echo
    
    # 总结
    echo -e "${GREEN}🎉 双节点健康检查完成！${NC}"
    echo
    echo -e "${BLUE}📋 下一步操作建议：${NC}"
    echo "1. 如果所有检查都通过，可以开始交易测试"
    echo "2. 使用完整测试脚本: ./scripts/testing/dual_node_transaction_test.sh"
    echo "3. 或者参考测试计划: DUAL_NODE_TRANSACTION_TEST_PLAN.md"
    echo "4. 手动执行API测试命令验证交易流程"
    echo
}

# 脚本入口点
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
