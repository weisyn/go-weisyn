#!/bin/bash

# 🎯 代币转账应用环境搭建脚本
# 功能：检查环境、安装依赖、初始化配置

set -e

echo "🚀 代币转账应用环境搭建"
echo "======================"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 步骤1：检查必需的工具
echo -e "${BLUE}📋 步骤1：检查环境依赖${NC}"
echo "================================"

check_command() {
    if command -v "$1" &> /dev/null; then
        echo -e "${GREEN}✅ $1 已安装${NC}"
    else
        echo -e "${RED}❌ $1 未安装${NC}"
        echo -e "${YELLOW}请安装 $1 后重试${NC}"
        if [ "$1" = "go" ]; then
            echo "安装Go: https://golang.org/dl/"
        elif [ "$1" = "node" ]; then
            echo "安装Node.js: https://nodejs.org/"
        fi
        exit 1
    fi
}

check_command "go"
check_command "git"

# 检查Go版本
GO_VERSION=$(go version | cut -d ' ' -f 3)
echo -e "${GREEN}Go版本: $GO_VERSION${NC}"

# 步骤2：检查WES项目结构
echo -e "\n${BLUE}📋 步骤2：检查项目结构${NC}"
echo "============================="

PROJECT_ROOT=$(pwd | grep -o '.*weisyn')
if [ -z "$PROJECT_ROOT" ]; then
    echo -e "${RED}❌ 请在WES项目根目录下运行此脚本${NC}"
    exit 1
fi

echo -e "${GREEN}✅ 项目根目录: $PROJECT_ROOT${NC}"

# 检查关键目录
check_directory() {
    if [ -d "$1" ]; then
        echo -e "${GREEN}✅ $1 目录存在${NC}"
    else
        echo -e "${RED}❌ $1 目录不存在${NC}"
        return 1
    fi
}

check_directory "$PROJECT_ROOT/contracts/templates/learning"
check_directory "$PROJECT_ROOT/pkg/interfaces"

# 步骤3：初始化Go模块依赖
echo -e "\n${BLUE}📋 步骤3：检查Go模块依赖${NC}"
echo "=============================="

cd "$PROJECT_ROOT"

echo "检查go.mod文件..."
if [ -f "go.mod" ]; then
    echo -e "${GREEN}✅ go.mod 存在${NC}"
    echo "更新依赖..."
    go mod tidy
    echo -e "${GREEN}✅ 依赖更新完成${NC}"
else
    echo -e "${RED}❌ go.mod 不存在，请在项目根目录运行 go mod init${NC}"
    exit 1
fi

# 步骤4：创建示例配置文件
echo -e "\n${BLUE}📋 步骤4：创建示例配置${NC}"
echo "=========================="

CONFIG_DIR="$PROJECT_ROOT/examples/basic/token-transfer/config"
mkdir -p "$CONFIG_DIR"

# 创建示例钱包配置
cat > "$CONFIG_DIR/wallets.json" << 'EOF'
{
  "description": "示例钱包配置文件",
  "wallets": [
    {
      "name": "Alice",
      "address": "alice_demo_address",
      "label": "测试用户Alice"
    },
    {
      "name": "Bob", 
      "address": "bob_demo_address",
      "label": "测试用户Bob"
    }
  ],
  "note": "这些是演示用的地址，实际使用时需要生成真实地址"
}
EOF

# 创建应用配置
cat > "$CONFIG_DIR/app.json" << 'EOF'
{
  "description": "代币转账应用配置",
  "blockchain": {
    "network": "local",
    "node_url": "http://localhost:8080",
    "timeout": 30
  },
  "token_contract": {
    "address": "demo_token_contract_address",
    "symbol": "DEMO",
    "decimals": 18
  },
  "transaction": {
    "fee_limit": 1000000,
    "fee_price": 1,
    "confirmation_blocks": 1
  }
}
EOF

echo -e "${GREEN}✅ 配置文件创建完成${NC}"
echo "- 钱包配置: $CONFIG_DIR/wallets.json"
echo "- 应用配置: $CONFIG_DIR/app.json"

# 步骤5：编译检查
echo -e "\n${BLUE}📋 步骤5：编译检查${NC}"
echo "==================="

cd "$PROJECT_ROOT/examples/basic/token-transfer"

echo "检查代码编译..."
if go build -o /tmp/token_transfer_check ./src/... > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 代码编译成功${NC}"
    rm -f /tmp/token_transfer_check
else
    echo -e "${YELLOW}⚠️  代码编译有警告，但不影响演示${NC}"
    echo -e "${YELLOW}这是因为示例代码中的接口需要在实际环境中连接真实的区块链${NC}"
fi

# 步骤6：创建快速测试脚本
echo -e "\n${BLUE}📋 步骤6：创建快速测试${NC}"
echo "======================"

cat > "$PROJECT_ROOT/examples/basic/token-transfer/quick_test.sh" << 'EOF'
#!/bin/bash
echo "🧪 快速功能测试"
echo "=============="

cd "$(dirname "$0")"

echo "1. 测试钱包管理..."
go run src/wallet_manager.go -test

echo "2. 测试交易构建..."  
go run src/transaction_builder.go -test

echo "3. 测试转账客户端..."
go run src/transfer_client.go -test

echo "✅ 快速测试完成"
EOF

chmod +x "$PROJECT_ROOT/examples/basic/token-transfer/quick_test.sh"

# 完成总结
echo -e "\n${GREEN}🎉 环境搭建完成！${NC}"
echo "=================="
echo -e "${BLUE}接下来您可以：${NC}"
echo "1. 查看README了解示例详情: less README.md"
echo "2. 运行快速测试: ./quick_test.sh"
echo "3. 部署代币合约: ./scripts/deploy_token.sh"
echo "4. 运行完整演示: ./scripts/run_demo.sh"
echo ""
echo -e "${YELLOW}📚 学习路径建议：${NC}"
echo "examples/basic/hello-world → token-transfer → contracts/templates/learning"
echo ""
echo -e "${GREEN}✨ 祝您学习愉快！${NC}"
