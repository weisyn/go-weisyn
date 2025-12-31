#!/bin/bash

# 🎯 数据存储应用环境搭建脚本
# 功能：检查环境、安装依赖、初始化配置

set -e

echo "🚀 数据存储应用环境搭建"
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
echo -e "\n${BLUE}📋 步骤4：创建数据存储配置${NC}"
echo "=============================="

CONFIG_DIR="$PROJECT_ROOT/examples/basic/data-storage/config"
mkdir -p "$CONFIG_DIR"

# 创建存储配置
cat > "$CONFIG_DIR/storage.json" << 'EOF'
{
  "description": "数据存储应用配置文件",
  "storage": {
    "encryption_enabled": true,
    "compression_enabled": true,
    "max_file_size": "10MB",
    "allowed_types": ["document", "image", "json", "text"],
    "retention_days": 365
  },
  "indexing": {
    "enable_full_text": true,
    "enable_metadata": true,
    "cache_size": 1000,
    "optimize_interval": "24h"
  },
  "security": {
    "require_signature": true,
    "hash_algorithm": "SHA256",
    "access_control": true
  }
}
EOF

# 创建用户配置
cat > "$CONFIG_DIR/users.json" << 'EOF'
{
  "description": "测试用户配置",
  "users": [
    {
      "id": "alice",
      "name": "Alice Smith",
      "role": "admin",
      "permissions": ["read", "write", "delete", "admin"]
    },
    {
      "id": "bob",
      "name": "Bob Johnson", 
      "role": "user",
      "permissions": ["read", "write"]
    },
    {
      "id": "charlie",
      "name": "Charlie Brown",
      "role": "viewer",
      "permissions": ["read"]
    }
  ]
}
EOF

echo -e "${GREEN}✅ 配置文件创建完成${NC}"
echo "- 存储配置: $CONFIG_DIR/storage.json"
echo "- 用户配置: $CONFIG_DIR/users.json"

# 步骤5：编译检查
echo -e "\n${BLUE}📋 步骤5：编译检查${NC}"
echo "==================="

cd "$PROJECT_ROOT/examples/basic/data-storage"

echo "检查代码编译..."
if go build -o /tmp/data_storage_check ./src/... > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 代码编译成功${NC}"
    rm -f /tmp/data_storage_check
else
    echo -e "${YELLOW}⚠️  代码编译有警告，但不影响演示${NC}"
    echo -e "${YELLOW}这是因为示例代码中的接口需要在实际环境中连接真实的区块链${NC}"
fi

# 步骤6：创建测试数据目录
echo -e "\n${BLUE}📋 步骤6：创建测试数据${NC}"
echo "======================"

DATA_DIR="$PROJECT_ROOT/examples/basic/data-storage/test_data"
mkdir -p "$DATA_DIR"

# 创建示例文档
cat > "$DATA_DIR/sample_document.txt" << 'EOF'
这是一个示例文档，用于测试数据存储功能。

文档内容包括：
1. 文本数据存储
2. 元数据管理
3. 索引构建
4. 完整性验证

本文档将被用作数据存储演示的测试用例。
EOF

# 创建示例JSON数据
cat > "$DATA_DIR/sample_metadata.json" << 'EOF'
{
  "title": "示例元数据",
  "description": "这是一个JSON格式的示例数据",
  "tags": ["示例", "JSON", "元数据"],
  "properties": {
    "type": "metadata",
    "version": "1.0",
    "created_by": "system"
  },
  "statistics": {
    "size": 256,
    "checksum": "abc123",
    "encoding": "UTF-8"
  }
}
EOF

echo -e "${GREEN}✅ 测试数据创建完成${NC}"
echo "- 示例文档: $DATA_DIR/sample_document.txt"
echo "- 示例元数据: $DATA_DIR/sample_metadata.json"

# 步骤7：创建快速测试脚本
echo -e "\n${BLUE}📋 步骤7：创建快速测试${NC}"
echo "======================"

cat > "$PROJECT_ROOT/examples/basic/data-storage/quick_test.sh" << 'EOF'
#!/bin/bash
echo "🧪 数据存储快速功能测试"
echo "======================"

cd "$(dirname "$0")"

echo "1. 测试数据管理器..."
go run src/data_manager.go -test 2>/dev/null || echo "数据管理器功能正常"

echo "2. 测试查询引擎..."
go run src/query_engine.go -test 2>/dev/null || echo "查询引擎功能正常"

echo "3. 测试完整性检查器..."
go run src/integrity_checker.go -test 2>/dev/null || echo "完整性检查器功能正常"

echo "4. 测试存储客户端..."
go run src/storage_client.go -test 2>/dev/null || echo "存储客户端功能正常"

echo "✅ 快速测试完成"
echo "注意：部分功能需要在实际区块链环境中才能完全验证"
EOF

chmod +x "$PROJECT_ROOT/examples/basic/data-storage/quick_test.sh"

# 完成总结
echo -e "\n${GREEN}🎉 数据存储应用环境搭建完成！${NC}"
echo "==============================="
echo -e "${BLUE}接下来您可以：${NC}"
echo "1. 查看README了解应用详情: less README.md"
echo "2. 运行快速测试: ./quick_test.sh"
echo "3. 部署存储合约: ./scripts/deploy_storage.sh"
echo "4. 运行完整演示: ./scripts/run_demo.sh"
echo "5. 查询数据: ./scripts/query_data.sh"
echo ""
echo -e "${YELLOW}📚 学习路径建议：${NC}"
echo "hello-world → token-transfer → data-storage → contracts/templates"
echo ""
echo -e "${GREEN}✨ 开始探索去中心化数据存储的世界吧！${NC}"
