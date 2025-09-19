#!/bin/bash

# 🎯 余额查询脚本
# 功能：查询指定地址的代币余额和账户信息

set -e

echo "🔍 代币余额查询工具"
echo "=================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT=$(pwd | grep -o '.*weisyn')
if [ -z "$PROJECT_ROOT" ]; then
    echo -e "${RED}❌ 请在WES项目根目录下运行此脚本${NC}"
    exit 1
fi

cd "$PROJECT_ROOT/examples/basic/token-transfer"

# 检查部署信息
if [ ! -f "deployed_contract.json" ]; then
    echo -e "${YELLOW}⚠️  未找到已部署的合约信息${NC}"
    echo "请先运行: ./scripts/deploy_token.sh"
    exit 1
fi

CONTRACT_ADDRESS=$(grep -o '"contract_address": *"[^"]*"' deployed_contract.json | cut -d'"' -f4)
TOKEN_SYMBOL=$(grep -o '"token_symbol": *"[^"]*"' deployed_contract.json | cut -d'"' -f4)
TOKEN_NAME=$(grep -o '"token_name": *"[^"]*"' deployed_contract.json | cut -d'"' -f4)

echo -e "${GREEN}✅ 代币合约信息${NC}"
echo "合约地址: $CONTRACT_ADDRESS"
echo "代币名称: $TOKEN_NAME"
echo "代币符号: $TOKEN_SYMBOL"
echo ""

# 功能选择菜单
show_menu() {
    echo -e "${BLUE}请选择查询功能：${NC}"
    echo "1. 查询单个地址余额"
    echo "2. 查询多个地址余额"
    echo "3. 查询合约总体信息"
    echo "4. 查看演示账户余额"
    echo "5. 退出"
    echo ""
}

# 查询单个地址余额
query_single_balance() {
    echo -e "\n${BLUE}📋 查询单个地址余额${NC}"
    echo "==================="
    
    read -p "请输入要查询的地址: " address
    
    if [ -z "$address" ]; then
        echo -e "${RED}❌ 地址不能为空${NC}"
        return
    fi
    
    echo "查询地址: $address"
    echo "正在查询余额..."
    
    # 模拟余额查询
    # 实际实现会调用区块链节点API
    BALANCE=$(( RANDOM % 1000000 ))
    
    echo -e "${GREEN}✅ 查询完成${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📍 地址: $address"
    echo "💰 余额: $BALANCE $TOKEN_SYMBOL"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# 查询多个地址余额
query_multiple_balances() {
    echo -e "\n${BLUE}📋 查询多个地址余额${NC}"
    echo "==================="
    
    echo "请输入要查询的地址（每行一个，空行结束）："
    addresses=()
    
    while true; do
        read -p "地址 $(( ${#addresses[@]} + 1 )): " address
        if [ -z "$address" ]; then
            break
        fi
        addresses+=("$address")
    done
    
    if [ ${#addresses[@]} -eq 0 ]; then
        echo -e "${RED}❌ 未输入任何地址${NC}"
        return
    fi
    
    echo -e "${GREEN}✅ 开始批量查询 ${#addresses[@]} 个地址${NC}"
    echo ""
    
    total_balance=0
    
    for i in "${!addresses[@]}"; do
        address="${addresses[$i]}"
        echo "正在查询地址 $(($i + 1))/${#addresses[@]}: $address"
        
        # 模拟余额查询
        balance=$(( RANDOM % 1000000 ))
        total_balance=$(( total_balance + balance ))
        
        echo -e "${GREEN}✅ 余额: $balance $TOKEN_SYMBOL${NC}"
        echo ""
    done
    
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📊 批量查询汇总"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    for i in "${!addresses[@]}"; do
        balance=$(( RANDOM % 1000000 ))
        echo "地址 $(($i + 1)): ${addresses[$i]::20}... | $balance $TOKEN_SYMBOL"
    done
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "💰 总计: $total_balance $TOKEN_SYMBOL"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# 查询合约总体信息
query_contract_info() {
    echo -e "\n${BLUE}📋 查询合约总体信息${NC}"
    echo "==================="
    
    echo "正在查询合约状态..."
    
    # 模拟合约信息查询
    TOTAL_SUPPLY=1000000
    TOTAL_HOLDERS=$(( RANDOM % 1000 + 100 ))
    TOTAL_TRANSACTIONS=$(( RANDOM % 10000 + 1000 ))
    
    echo -e "${GREEN}✅ 合约信息查询完成${NC}"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📋 $TOKEN_NAME ($TOKEN_SYMBOL) 合约信息"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🏠 合约地址: $CONTRACT_ADDRESS"
    echo "💰 总供应量: $TOTAL_SUPPLY $TOKEN_SYMBOL"
    echo "👥 持有者数量: $TOTAL_HOLDERS"
    echo "📊 总交易数: $TOTAL_TRANSACTIONS"
    echo "📅 部署时间: $(grep -o '"deployed_at": *"[^"]*"' deployed_contract.json | cut -d'"' -f4)"
    echo "🏷️  使用模板: $(grep -o '"template_used": *"[^"]*"' deployed_contract.json | cut -d'"' -f4)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # 显示网络统计信息
    echo ""
    echo -e "${BLUE}📊 网络统计信息${NC}"
    echo "==============="
    echo "⛽ 平均执行费用价格: 1 gwei"
    echo "⏱️  平均确认时间: 2-3秒"
    echo "📈 24小时交易量: $(( RANDOM % 50000 + 10000 )) $TOKEN_SYMBOL"
    echo "🔄 24小时转账次数: $(( RANDOM % 500 + 100 ))"
}

# 查看演示账户余额
query_demo_accounts() {
    echo -e "\n${BLUE}📋 演示账户余额${NC}"
    echo "================"
    
    # 检查是否存在演示报告
    LATEST_REPORT=$(ls -t demo_report_*.json 2>/dev/null | head -1)
    
    if [ -z "$LATEST_REPORT" ]; then
        echo -e "${YELLOW}⚠️  未找到演示报告${NC}"
        echo "请先运行: ./scripts/run_demo.sh"
        echo ""
        echo "或者查看配置中的演示账户："
        
        if [ -f "config/wallets.json" ]; then
            echo -e "${GREEN}✅ 配置文件中的演示钱包${NC}"
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
            
            # 简单解析JSON文件
            grep -o '"name": *"[^"]*"' config/wallets.json | while read -r name_line; do
                name=$(echo "$name_line" | cut -d'"' -f4)
                echo "👤 $name: (需要运行演示后查看余额)"
            done
            echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        fi
        return
    fi
    
    echo -e "${GREEN}✅ 发现最新演示报告: $LATEST_REPORT${NC}"
    echo ""
    
    # 解析演示报告
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📊 演示账户最终余额"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # 提取Alice信息
    ALICE_ROLE=$(grep -A 10 '"alice"' "$LATEST_REPORT" | grep '"role"' | cut -d'"' -f4)
    ALICE_ADDRESS=$(grep -A 10 '"alice"' "$LATEST_REPORT" | grep '"address"' | cut -d'"' -f4)
    ALICE_BALANCE=$(grep -A 10 '"alice"' "$LATEST_REPORT" | grep '"final_balance"' | grep -o '[0-9]*')
    
    echo "🏪 Alice ($ALICE_ROLE)"
    echo "   地址: $ALICE_ADDRESS"
    echo "   余额: $ALICE_BALANCE $TOKEN_SYMBOL"
    echo ""
    
    # 提取Bob信息  
    BOB_ROLE=$(grep -A 10 '"bob"' "$LATEST_REPORT" | grep '"role"' | cut -d'"' -f4)
    BOB_ADDRESS=$(grep -A 10 '"bob"' "$LATEST_REPORT" | grep '"address"' | cut -d'"' -f4)
    BOB_BALANCE=$(grep -A 10 '"bob"' "$LATEST_REPORT" | grep '"final_balance"' | grep -o '[0-9]*')
    
    echo "👤 Bob ($BOB_ROLE)"
    echo "   地址: $BOB_ADDRESS"
    echo "   余额: $BOB_BALANCE $TOKEN_SYMBOL"
    echo ""
    
    # 提取Charlie信息
    CHARLIE_ROLE=$(grep -A 10 '"charlie"' "$LATEST_REPORT" | grep '"role"' | cut -d'"' -f4)
    CHARLIE_ADDRESS=$(grep -A 10 '"charlie"' "$LATEST_REPORT" | grep '"address"' | cut -d'"' -f4)
    CHARLIE_BALANCE=$(grep -A 10 '"charlie"' "$LATEST_REPORT" | grep '"final_balance"' | grep -o '[0-9]*')
    
    echo "👥 Charlie ($CHARLIE_ROLE)"
    echo "   地址: $CHARLIE_ADDRESS"
    echo "   余额: $CHARLIE_BALANCE $TOKEN_SYMBOL"
    echo ""
    
    # 计算总计
    TOTAL=$(( ALICE_BALANCE + BOB_BALANCE + CHARLIE_BALANCE ))
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "💰 总计: $TOTAL $TOKEN_SYMBOL"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    
    # 显示最近交易
    echo ""
    echo -e "${BLUE}📊 演示中的交易记录${NC}"
    echo "==================="
    
    grep -A 30 '"transactions"' "$LATEST_REPORT" | grep '"type"' | while read -r tx_line; do
        tx_type=$(echo "$tx_line" | cut -d'"' -f4)
        echo "• $tx_type"
    done
}

# 主循环
while true; do
    show_menu
    read -p "请选择 (1-5): " choice
    
    case $choice in
        1)
            query_single_balance
            ;;
        2)
            query_multiple_balances
            ;;
        3)
            query_contract_info
            ;;
        4)
            query_demo_accounts
            ;;
        5)
            echo -e "${GREEN}👋 感谢使用余额查询工具！${NC}"
            exit 0
            ;;
        *)
            echo -e "${RED}❌ 无效选择，请输入1-5${NC}"
            ;;
    esac
    
    echo ""
    read -p "按Enter继续..."
    echo ""
done
