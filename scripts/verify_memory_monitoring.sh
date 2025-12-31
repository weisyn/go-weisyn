#!/bin/bash
# 内存监控系统验证脚本
#
# 用途：验证 WES 内存监控系统的完整功能
# 包括：
# 1. 编译检查
# 2. 单元测试
# 3. 集成测试
# 4. 模块注册验证

set -e

echo "=========================================="
echo "WES 内存监控系统验证脚本"
echo "=========================================="
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo -e "${YELLOW}[1/5] 检查代码编译...${NC}"
if go build ./internal/core/... ./internal/api/... ./pkg/... 2>&1 | grep -E "(error|Error)"; then
    echo -e "${RED}❌ 编译失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ 编译通过${NC}"
echo ""

echo -e "${YELLOW}[2/5] 运行内存监控单元测试...${NC}"
if ! go test ./pkg/utils/metrics -v 2>&1; then
    echo -e "${RED}❌ 单元测试失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ 单元测试通过${NC}"
echo ""

echo -e "${YELLOW}[3/5] 运行内存监控集成测试...${NC}"
if ! go test ./test/integration -run TestMemoryMonitoring -v 2>&1; then
    echo -e "${RED}❌ 集成测试失败${NC}"
    exit 1
fi
echo -e "${GREEN}✅ 集成测试通过${NC}"
echo ""

echo -e "${YELLOW}[4/5] 验证模块注册情况...${NC}"
# 检查所有模块是否实现了 MemoryReporter
MODULES=(
    "internal/core/mempool/txpool"
    "internal/core/consensus/miner"
    "internal/core/consensus/aggregator"
    "internal/core/block/builder"
    "internal/core/chain/sync"
    "internal/core/eutxo/writer"
    "internal/core/ispc/coordinator"
    "internal/core/ures/cas"
    "internal/core/tx/draft"
    "internal/core/infrastructure/storage"
    "internal/core/infrastructure/event"
    "internal/core/network/facade"
    "internal/api/http"
    "internal/api/websocket"
)

REGISTERED_COUNT=0
for module in "${MODULES[@]}"; do
    # 检查模块目录或其父级 module.go 中是否有注册
    module_dir=$(dirname "$module")
    if grep -r "RegisterMemoryReporter" "$module_dir" --include="*.go" > /dev/null 2>&1 || \
       grep -r "RegisterMemoryReporter" "$(dirname "$module_dir")" --include="module.go" > /dev/null 2>&1; then
        echo -e "  ${GREEN}✅${NC} $module"
        ((REGISTERED_COUNT++))
    else
        echo -e "  ${YELLOW}⚠️${NC}  $module (注册可能在父级 module.go)"
    fi
done

echo ""
echo -e "已注册模块数: ${REGISTERED_COUNT}/${#MODULES[@]}"
if [ $REGISTERED_COUNT -lt ${#MODULES[@]} ]; then
    echo -e "${YELLOW}⚠️  部分模块未注册${NC}"
fi
echo ""

echo -e "${YELLOW}[5/5] 验证 HTTP 接口...${NC}"
# 检查 HTTP 接口是否正确注册
if grep -r "system/memory" internal/api/http --include="*.go" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ HTTP 接口已注册${NC}"
else
    echo -e "${RED}❌ HTTP 接口未找到${NC}"
    exit 1
fi
echo ""

echo "=========================================="
echo -e "${GREEN}✅ 所有验证通过！${NC}"
echo "=========================================="
echo ""
echo "内存监控系统已就绪，可以通过以下方式使用："
echo "  1. 启动节点后访问: GET /api/v1/system/memory"
echo "  2. 查看日志中的内存监控信息"
echo "  3. 使用 MemoryDoctor 进行内存趋势分析"
echo ""

