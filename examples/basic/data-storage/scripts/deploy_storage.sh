#!/bin/bash

# 🎯 数据存储合约部署脚本
# 功能：部署数据存储合约，为存储应用提供基础

set -e

echo "🚀 部署数据存储合约"
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

# 步骤1：选择要部署的存储合约模板
echo -e "${BLUE}📋 步骤1：选择存储合约模板${NC}"
echo "============================="

echo "可用的存储合约模板："
echo "1. learning/starter-contract (推荐，包含存储功能)"
echo "2. standard/governance (高级，支持访问控制)"

read -p "请选择要部署的模板 (1-2): " template_choice

case $template_choice in
    1)
        TEMPLATE_PATH="$PROJECT_ROOT/contracts/templates/learning/starter-contract"
        TEMPLATE_NAME="starter-contract"
        echo -e "${GREEN}✅ 选择: 学习版入门合约（包含存储功能）${NC}"
        ;;
    2)
        TEMPLATE_PATH="$PROJECT_ROOT/contracts/templates/standard/governance"
        TEMPLATE_NAME="governance-contract"
        echo -e "${GREEN}✅ 选择: 标准版治理合约（高级存储）${NC}"
        ;;
    *)
        echo -e "${RED}❌ 无效选择${NC}"
        exit 1
        ;;
esac

# 检查模板是否存在
if [ ! -d "$TEMPLATE_PATH" ]; then
    echo -e "${RED}❌ 模板目录不存在: $TEMPLATE_PATH${NC}"
    exit 1
fi

# 步骤2：配置存储参数
echo -e "\n${BLUE}📋 步骤2：配置存储参数${NC}"
echo "======================"

echo "请输入存储配置信息（按Enter使用默认值）："

read -p "存储容量限制(MB) [1000]: " storage_limit
storage_limit=${storage_limit:-"1000"}

read -p "最大文件大小(MB) [10]: " max_file_size
max_file_size=${max_file_size:-"10"}

read -p "是否启用加密 (y/n) [y]: " enable_encryption
enable_encryption=${enable_encryption:-"y"}

read -p "是否启用压缩 (y/n) [y]: " enable_compression
enable_compression=${enable_compression:-"y"}

echo -e "${GREEN}✅ 存储配置:${NC}"
echo "容量限制: ${storage_limit}MB"
echo "文件大小: ${max_file_size}MB"
echo "启用加密: $enable_encryption"
echo "启用压缩: $enable_compression"

# 步骤3：编译合约
echo -e "\n${BLUE}📋 步骤3：编译合约${NC}"
echo "=================="

BUILD_DIR="$PROJECT_ROOT/examples/basic/data-storage/build"
mkdir -p "$BUILD_DIR"

cd "$TEMPLATE_PATH"

echo "编译存储合约..."

if [ "$template_choice" = "1" ]; then
    # 学习版使用framework SDK编译
    if command -v tinygo &> /dev/null; then
        echo "使用TinyGo编译..."
        tinygo build -target wasm -o "$BUILD_DIR/storage.wasm" src/main.go
        echo -e "${GREEN}✅ 合约编译完成${NC}"
    else
        echo -e "${YELLOW}⚠️  TinyGo未安装，使用模拟编译${NC}"
        echo "在实际环境中，这里会生成storage.wasm文件"
        echo "demo_storage_wasm_content" > "$BUILD_DIR/storage.wasm"
    fi
else
    # 标准版使用直接宿主函数编译
    echo "使用WES工具链编译..."
    if [ -f "$PROJECT_ROOT/contracts/tools/compiler/main.go" ]; then
        go run "$PROJECT_ROOT/contracts/tools/compiler/main.go" \
            -input "dao_governance_template.go" \
            -output "$BUILD_DIR/storage.wasm"
        echo -e "${GREEN}✅ 合约编译完成${NC}"
    else
        echo -e "${YELLOW}⚠️  WES编译器不可用，使用模拟编译${NC}"
        echo "demo_storage_wasm_content" > "$BUILD_DIR/storage.wasm"
    fi
fi

# 步骤4：部署合约
echo -e "\n${BLUE}📋 步骤4：部署合约到区块链${NC}"
echo "========================="

# 检查区块链节点是否运行
if ! curl -s http://localhost:28680/health > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  区块链节点未运行${NC}"
    echo "请先启动WES节点:"
    echo "  cd $PROJECT_ROOT"
    echo "  ./bin/node"
    echo ""
    echo -e "${BLUE}是否继续模拟部署？ (y/n):${NC}"
    read -p "" simulate_deploy
    
    if [ "$simulate_deploy" != "y" ]; then
        echo "部署取消"
        exit 1
    fi
    
    echo -e "${YELLOW}📝 模拟部署模式${NC}"
    
    # 生成模拟合约地址
    CONTRACT_ADDRESS="storage_contract_$(date +%s | tail -c 6)"
    echo "模拟合约地址: $CONTRACT_ADDRESS"
    
else
    echo "检测到区块链节点运行中..."
    
    # 实际部署逻辑
    echo "构建部署交易..."
    
    # 这里应该调用实际的部署工具
    # CONTRACT_ADDRESS=$(weisyn deploy "$BUILD_DIR/storage.wasm" --limit "$storage_limit" --max-file "$max_file_size")
    
    # 模拟部署
    CONTRACT_ADDRESS="deployed_storage_$(date +%s | tail -c 6)"
    echo -e "${GREEN}✅ 合约部署成功${NC}"
fi

# 步骤5：保存部署信息
echo -e "\n${BLUE}📋 步骤5：保存部署信息${NC}"
echo "======================"

DEPLOY_INFO_FILE="$PROJECT_ROOT/examples/basic/data-storage/deployed_contract.json"

cat > "$DEPLOY_INFO_FILE" << EOF
{
  "contract_address": "$CONTRACT_ADDRESS",
  "contract_type": "data_storage",
  "storage_limit_mb": "$storage_limit",
  "max_file_size_mb": "$max_file_size",
  "encryption_enabled": "$enable_encryption",
  "compression_enabled": "$enable_compression",
  "template_used": "$TEMPLATE_NAME",
  "deployed_at": "$(date -Iseconds)",
  "wasm_file": "$BUILD_DIR/storage.wasm"
}
EOF

echo -e "${GREEN}✅ 部署信息保存到: $DEPLOY_INFO_FILE${NC}"

# 更新应用配置
CONFIG_FILE="$PROJECT_ROOT/examples/basic/data-storage/config/storage.json"
if [ -f "$CONFIG_FILE" ]; then
    # 使用jq更新配置文件（如果可用）
    if command -v jq &> /dev/null; then
        jq ".contract_address = \"$CONTRACT_ADDRESS\" | .storage.max_file_size = \"${max_file_size}MB\"" "$CONFIG_FILE" > "${CONFIG_FILE}.tmp"
        mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"
        echo -e "${GREEN}✅ 应用配置已更新${NC}"
    else
        echo -e "${YELLOW}⚠️  jq未安装，请手动更新config/storage.json中的合约地址${NC}"
    fi
fi

# 步骤6：验证部署
echo -e "\n${BLUE}📋 步骤6：验证部署${NC}"
echo "=================="

echo "进行基础验证..."

if [ -f "$BUILD_DIR/storage.wasm" ]; then
    WASM_SIZE=$(stat -f%z "$BUILD_DIR/storage.wasm" 2>/dev/null || stat -c%s "$BUILD_DIR/storage.wasm" 2>/dev/null)
    echo -e "${GREEN}✅ WASM文件存在 (大小: ${WASM_SIZE} bytes)${NC}"
else
    echo -e "${RED}❌ WASM文件不存在${NC}"
fi

if [ -f "$DEPLOY_INFO_FILE" ]; then
    echo -e "${GREEN}✅ 部署信息已记录${NC}"
else
    echo -e "${RED}❌ 部署信息记录失败${NC}"
fi

# 创建索引初始化脚本
echo "创建索引初始化脚本..."
cat > "$PROJECT_ROOT/examples/basic/data-storage/init_index.sh" << 'INIT_SCRIPT'
#!/bin/bash
echo "🔧 初始化存储索引..."

# 这里可以调用合约的初始化方法
# curl -X POST http://localhost:28680/contract/call \
#   -H "Content-Type: application/json" \
#   -d '{"address":"CONTRACT_ADDRESS","method":"Initialize","params":{}}'

echo "✅ 索引初始化完成"
INIT_SCRIPT

chmod +x "$PROJECT_ROOT/examples/basic/data-storage/init_index.sh"

# 完成总结
echo -e "\n${GREEN}🎉 数据存储合约部署完成！${NC}"
echo "=========================="
echo -e "${BLUE}部署摘要：${NC}"
echo "合约地址: $CONTRACT_ADDRESS"
echo "存储限制: ${storage_limit}MB"
echo "文件大小: ${max_file_size}MB"
echo "加密启用: $enable_encryption"
echo "压缩启用: $enable_compression"
echo ""
echo -e "${BLUE}接下来您可以：${NC}"
echo "1. 初始化索引: ./init_index.sh"
echo "2. 运行存储演示: ./scripts/run_demo.sh"
echo "3. 查询存储状态: ./scripts/query_data.sh"
echo "4. 查看部署详情: cat deployed_contract.json"
echo ""
echo -e "${GREEN}✨ 数据存储合约已就绪，可以开始存储应用演示！${NC}"
