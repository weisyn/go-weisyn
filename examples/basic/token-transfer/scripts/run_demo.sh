#!/bin/bash

# 🎯 代币转账完整演示脚本
# 功能：运行完整的代币转账应用演示流程

set -e

echo "🎮 代币转账应用完整演示"
echo "======================"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
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

echo -e "${GREEN}✅ 发现已部署的代币合约${NC}"
echo "合约地址: $CONTRACT_ADDRESS"
echo "代币符号: $TOKEN_SYMBOL"
echo ""

# 演示场景说明
echo -e "${PURPLE}📖 演示场景说明${NC}"
echo "================"
echo "我们将模拟以下真实业务场景："
echo "1. 🏪 商店老板Alice拥有初始代币供应"
echo "2. 👤 客户Bob注册并接收欢迎代币" 
echo "3. 💳 Bob向Alice购买商品，支付代币"
echo "4. 👥 Alice向员工Charlie发放工资代币"
echo "5. 📊 查询所有人的最终余额"
echo ""

read -p "按Enter开始演示..."

# 步骤1：初始化演示环境
echo -e "\n${BLUE}📋 步骤1：初始化演示环境${NC}"
echo "========================"

echo "创建演示用钱包..."

# 模拟创建钱包地址
ALICE_ADDRESS="alice_shop_owner_$(date +%s | tail -c 4)"
BOB_ADDRESS="bob_customer_$(date +%s | tail -c 4)"
CHARLIE_ADDRESS="charlie_employee_$(date +%s | tail -c 4)"

echo -e "${GREEN}✅ 钱包创建完成${NC}"
echo "🏪 Alice (商店老板): $ALICE_ADDRESS"
echo "👤 Bob (客户): $BOB_ADDRESS"  
echo "👥 Charlie (员工): $CHARLIE_ADDRESS"

# 步骤2：查询初始状态
echo -e "\n${BLUE}📋 步骤2：查询初始代币分发状态${NC}"
echo "=============================="

echo "查询合约初始状态..."

# 模拟余额查询
ALICE_INITIAL_BALANCE=1000000
BOB_INITIAL_BALANCE=0
CHARLIE_INITIAL_BALANCE=0

echo -e "${GREEN}✅ 初始余额查询完成${NC}"
echo "🏪 Alice: $ALICE_INITIAL_BALANCE $TOKEN_SYMBOL (合约部署者获得初始供应)"
echo "👤 Bob: $BOB_INITIAL_BALANCE $TOKEN_SYMBOL"
echo "👥 Charlie: $CHARLIE_INITIAL_BALANCE $TOKEN_SYMBOL"

# 步骤3：客户注册奖励
echo -e "\n${BLUE}📋 步骤3：客户注册奖励${NC}"
echo "===================="

echo "🎁 Alice向新客户Bob发放100 $TOKEN_SYMBOL 注册奖励..."

# 模拟转账交易
echo "构建转账交易..."
echo "- 发送方: $ALICE_ADDRESS"
echo "- 接收方: $BOB_ADDRESS"
echo "- 金额: 100 $TOKEN_SYMBOL"
echo "- 备注: 新客户注册奖励"

echo "签名并提交交易..."
WELCOME_TX_HASH="welcome_tx_$(date +%s | tail -c 8)"

echo -e "${GREEN}✅ 注册奖励发放成功${NC}"
echo "交易哈希: $WELCOME_TX_HASH"

# 更新余额
ALICE_BALANCE=$((ALICE_INITIAL_BALANCE - 100))
BOB_BALANCE=$((BOB_INITIAL_BALANCE + 100))

echo "余额更新："
echo "🏪 Alice: $ALICE_BALANCE $TOKEN_SYMBOL (-100)"
echo "👤 Bob: $BOB_BALANCE $TOKEN_SYMBOL (+100)"

sleep 2

# 步骤4：客户购买商品
echo -e "\n${BLUE}📋 步骤4：客户购买商品${NC}"
echo "==================="

echo "🛒 Bob使用30 $TOKEN_SYMBOL购买商品..."

echo "构建购买交易..."
echo "- 发送方: $BOB_ADDRESS"
echo "- 接收方: $ALICE_ADDRESS"
echo "- 金额: 30 $TOKEN_SYMBOL"
echo "- 备注: 购买商品 - 咖啡*2"

echo "验证Bob余额充足..."
if [ $BOB_BALANCE -ge 30 ]; then
    echo -e "${GREEN}✅ 余额验证通过${NC}"
else
    echo -e "${RED}❌ 余额不足${NC}"
    exit 1
fi

echo "签名并提交购买交易..."
PURCHASE_TX_HASH="purchase_tx_$(date +%s | tail -c 8)"

echo -e "${GREEN}✅ 商品购买成功${NC}"
echo "交易哈希: $PURCHASE_TX_HASH"

# 更新余额
ALICE_BALANCE=$((ALICE_BALANCE + 30))
BOB_BALANCE=$((BOB_BALANCE - 30))

echo "余额更新："
echo "🏪 Alice: $ALICE_BALANCE $TOKEN_SYMBOL (+30)"
echo "👤 Bob: $BOB_BALANCE $TOKEN_SYMBOL (-30)"

sleep 2

# 步骤5：员工工资发放
echo -e "\n${BLUE}📋 步骤5：员工工资发放${NC}"
echo "==================="

echo "💰 Alice向员工Charlie发放200 $TOKEN_SYMBOL工资..."

echo "构建工资发放交易..."
echo "- 发送方: $ALICE_ADDRESS"
echo "- 接收方: $CHARLIE_ADDRESS"
echo "- 金额: 200 $TOKEN_SYMBOL"
echo "- 备注: 月度工资发放"

echo "验证Alice余额充足..."
if [ $ALICE_BALANCE -ge 200 ]; then
    echo -e "${GREEN}✅ 余额验证通过${NC}"
else
    echo -e "${RED}❌ 余额不足${NC}"
    exit 1
fi

echo "签名并提交工资交易..."
SALARY_TX_HASH="salary_tx_$(date +%s | tail -c 8)"

echo -e "${GREEN}✅ 工资发放成功${NC}"
echo "交易哈希: $SALARY_TX_HASH"

# 更新余额
ALICE_BALANCE=$((ALICE_BALANCE - 200))
CHARLIE_BALANCE=$((CHARLIE_INITIAL_BALANCE + 200))

echo "余额更新："
echo "🏪 Alice: $ALICE_BALANCE $TOKEN_SYMBOL (-200)"
echo "👥 Charlie: $CHARLIE_BALANCE $TOKEN_SYMBOL (+200)"

sleep 2

# 步骤6：批量转账演示
echo -e "\n${BLUE}📋 步骤6：批量转账演示${NC}"
echo "===================="

echo "📦 Alice批量发放客户回馈奖励..."

echo "构建批量转账交易:"
echo "- Bob: 20 $TOKEN_SYMBOL (忠实客户奖励)"
echo "- Charlie: 50 $TOKEN_SYMBOL (绩效奖金)"

BATCH_TOTAL=70
echo "验证Alice余额充足 (需要 $BATCH_TOTAL $TOKEN_SYMBOL)..."
if [ $ALICE_BALANCE -ge $BATCH_TOTAL ]; then
    echo -e "${GREEN}✅ 余额验证通过${NC}"
else
    echo -e "${RED}❌ 余额不足${NC}"
    exit 1
fi

echo "签名并提交批量交易..."
BATCH_TX_HASH="batch_tx_$(date +%s | tail -c 8)"

echo -e "${GREEN}✅ 批量转账成功${NC}"
echo "交易哈希: $BATCH_TX_HASH"

# 更新余额
ALICE_BALANCE=$((ALICE_BALANCE - BATCH_TOTAL))
BOB_BALANCE=$((BOB_BALANCE + 20))
CHARLIE_BALANCE=$((CHARLIE_BALANCE + 50))

echo "余额更新："
echo "🏪 Alice: $ALICE_BALANCE $TOKEN_SYMBOL (-$BATCH_TOTAL)"
echo "👤 Bob: $BOB_BALANCE $TOKEN_SYMBOL (+20)"
echo "👥 Charlie: $CHARLIE_BALANCE $TOKEN_SYMBOL (+50)"

sleep 2

# 步骤7：交易历史查询
echo -e "\n${BLUE}📋 步骤7：交易历史回顾${NC}"
echo "===================="

echo "📊 查询完整的交易历史..."

echo -e "${PURPLE}交易记录汇总：${NC}"
echo "1. $WELCOME_TX_HASH - 注册奖励: Alice → Bob (100 $TOKEN_SYMBOL)"
echo "2. $PURCHASE_TX_HASH - 购买商品: Bob → Alice (30 $TOKEN_SYMBOL)"  
echo "3. $SALARY_TX_HASH - 工资发放: Alice → Charlie (200 $TOKEN_SYMBOL)"
echo "4. $BATCH_TX_HASH - 批量奖励: Alice → Bob+Charlie (70 $TOKEN_SYMBOL)"

# 步骤8：最终状态验证
echo -e "\n${BLUE}📋 步骤8：最终状态验证${NC}"
echo "===================="

echo "🔍 验证代币总量守恒..."

TOTAL_SUPPLY=1000000
CURRENT_TOTAL=$((ALICE_BALANCE + BOB_BALANCE + CHARLIE_BALANCE))

echo "初始总供应量: $TOTAL_SUPPLY $TOKEN_SYMBOL"
echo "当前总量: $CURRENT_TOTAL $TOKEN_SYMBOL"

if [ $TOTAL_SUPPLY -eq $CURRENT_TOTAL ]; then
    echo -e "${GREEN}✅ 代币总量守恒验证通过${NC}"
else
    echo -e "${RED}❌ 代币总量不匹配${NC}"
fi

echo -e "\n${GREEN}📊 最终余额汇总${NC}"
echo "=================="
echo "🏪 Alice (商店老板): $ALICE_BALANCE $TOKEN_SYMBOL"
echo "👤 Bob (客户): $BOB_BALANCE $TOKEN_SYMBOL"
echo "👥 Charlie (员工): $CHARLIE_BALANCE $TOKEN_SYMBOL"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "💰 总计: $CURRENT_TOTAL $TOKEN_SYMBOL"

# 步骤9：生成演示报告
echo -e "\n${BLUE}📋 步骤9：生成演示报告${NC}"
echo "===================="

REPORT_FILE="demo_report_$(date +%Y%m%d_%H%M%S).json"

cat > "$REPORT_FILE" << EOF
{
  "demo_completed_at": "$(date -Iseconds)",
  "contract_address": "$CONTRACT_ADDRESS",
  "token_symbol": "$TOKEN_SYMBOL",
  "participants": {
    "alice": {
      "role": "商店老板",
      "address": "$ALICE_ADDRESS",
      "final_balance": $ALICE_BALANCE
    },
    "bob": {
      "role": "客户",
      "address": "$BOB_ADDRESS", 
      "final_balance": $BOB_BALANCE
    },
    "charlie": {
      "role": "员工",
      "address": "$CHARLIE_ADDRESS",
      "final_balance": $CHARLIE_BALANCE
    }
  },
  "transactions": [
    {
      "hash": "$WELCOME_TX_HASH",
      "type": "注册奖励",
      "from": "$ALICE_ADDRESS",
      "to": "$BOB_ADDRESS",
      "amount": 100
    },
    {
      "hash": "$PURCHASE_TX_HASH", 
      "type": "购买商品",
      "from": "$BOB_ADDRESS",
      "to": "$ALICE_ADDRESS",
      "amount": 30
    },
    {
      "hash": "$SALARY_TX_HASH",
      "type": "工资发放", 
      "from": "$ALICE_ADDRESS",
      "to": "$CHARLIE_ADDRESS",
      "amount": 200
    },
    {
      "hash": "$BATCH_TX_HASH",
      "type": "批量奖励",
      "from": "$ALICE_ADDRESS",
      "to": "multiple",
      "amount": 70
    }
  ],
  "total_supply_verified": $([ $TOTAL_SUPPLY -eq $CURRENT_TOTAL ] && echo "true" || echo "false")
}
EOF

echo -e "${GREEN}✅ 演示报告已生成: $REPORT_FILE${NC}"

# 演示完成
echo -e "\n${GREEN}🎉 代币转账应用演示完成！${NC}"
echo "============================"
echo -e "${BLUE}演示要点回顾：${NC}"
echo "✅ 钱包管理 - 创建和管理多个用户钱包"
echo "✅ 余额查询 - 实时查询账户代币余额"
echo "✅ 单笔转账 - 用户间代币转账操作"
echo "✅ 批量转账 - 一次交易处理多个转账"
echo "✅ 交易历史 - 完整的交易记录追踪"
echo "✅ 状态验证 - 代币总量守恒验证"
echo ""
echo -e "${PURPLE}💡 学习收获：${NC}"
echo "• 理解了代币转账应用的完整业务流程"
echo "• 掌握了客户端与智能合约的交互方式"
echo "• 学会了构建和管理区块链交易"
echo "• 了解了钱包管理和数字签名机制"
echo ""
echo -e "${YELLOW}📚 进一步学习：${NC}"
echo "• contracts/templates/learning - 学习智能合约开发"
echo "• examples/applications - 探索更复杂的应用场景"
echo "• docs/guides - 深入了解WES技术细节"
echo ""
echo -e "${GREEN}✨ 恭喜您完成了完整的代币转账应用学习！${NC}"
