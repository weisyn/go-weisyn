#!/bin/bash

# ============================================================================
# 测试脚本验证工具
# ============================================================================
# 用途：验证所有测试脚本的语法和基本功能
# ============================================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  测试脚本验证工具${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}/../.."

# 要验证的脚本列表（包含所有测试脚本）
SCRIPTS=(
    "scripts/testing/fork/fork_scenarios.sh"
    "scripts/testing/fork/real_fork.sh"
    "scripts/testing/fork/real_multi_node.sh"
    "scripts/testing/fork/fork_and_double_spend.sh"
    "scripts/testing/models/onnx_models_test.sh"
    "scripts/testing/models/clean_and_test_onnx.sh"
)

# 验证计数
TOTAL=0
PASSED=0
FAILED=0

echo -e "${YELLOW}步骤1: 语法检查${NC}"
echo ""

for script in "${SCRIPTS[@]}"; do
    TOTAL=$((TOTAL + 1))
    echo -n "检查 $(basename ${script})... "
    
    if bash -n "${script}" 2>/dev/null; then
        echo -e "${GREEN}✓ 通过${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}✗ 失败${NC}"
        FAILED=$((FAILED + 1))
        bash -n "${script}"
    fi
done

echo ""
echo -e "${YELLOW}步骤2: 权限检查${NC}"
echo ""

for script in "${SCRIPTS[@]}"; do
    echo -n "检查 $(basename ${script})... "
    
    if [ -x "${script}" ]; then
        echo -e "${GREEN}✓ 可执行${NC}"
    else
        echo -e "${YELLOW}! 不可执行,正在修复...${NC}"
        chmod +x "${script}"
        echo -e "${GREEN}✓ 已修复${NC}"
    fi
done

echo ""
echo -e "${YELLOW}步骤3: 依赖检查${NC}"
echo ""

# 检查Go环境
echo -n "检查 Go 环境... "
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    echo -e "${GREEN}✓ ${GO_VERSION}${NC}"
else
    echo -e "${RED}✗ 未安装${NC}"
fi

# 检查必要的命令
COMMANDS=("tail" "grep" "kill" "ps" "mkdir" "rm" "cat" "lsof")
for cmd in "${COMMANDS[@]}"; do
    echo -n "检查 ${cmd} 命令... "
    if command -v ${cmd} &> /dev/null; then
        echo -e "${GREEN}✓ 已安装${NC}"
    else
        echo -e "${RED}✗ 未安装${NC}"
    fi
done

echo ""
echo -e "${YELLOW}步骤4: 端口检查${NC}"
echo ""

PORTS=(28680 30000 30001 30002)
for port in "${PORTS[@]}"; do
    echo -n "检查端口 ${port}... "
    if lsof -i :${port} &> /dev/null; then
        echo -e "${YELLOW}! 已被占用${NC}"
        lsof -i :${port} | grep LISTEN
    else
        echo -e "${GREEN}✓ 可用${NC}"
    fi
done

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  验证结果汇总${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo "语法检查:"
echo "  总数: ${TOTAL}"
echo -e "  ${GREEN}通过: ${PASSED}${NC}"
if [ ${FAILED} -gt 0 ]; then
    echo -e "  ${RED}失败: ${FAILED}${NC}"
else
    echo -e "  ${GREEN}失败: 0${NC}"
fi
echo ""

if [ ${FAILED} -eq 0 ]; then
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  所有脚本验证通过！✓${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "可以使用以下脚本:"
    for script in "${SCRIPTS[@]}"; do
        echo "  bash ${script}"
    done
else
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}  部分脚本验证失败！✗${NC}"
    echo -e "${RED}========================================${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}主要测试脚本:${NC}"
echo "1. 分叉场景说明: bash scripts/testing/fork/fork_scenarios.sh"
echo "2. 单节点测试:   bash scripts/testing/fork/real_fork.sh"
echo "3. 多节点测试:   bash scripts/testing/fork/real_multi_node.sh"
echo "4. 综合测试:     bash scripts/testing/fork/fork_and_double_spend.sh"
echo "5. ONNX模型测试: bash scripts/testing/models/onnx_models_test.sh"
echo ""


