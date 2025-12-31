#!/bin/bash

# WES 架构守护检查脚本
# 用于自动化检测架构违规行为

set -e

echo "🛡️  WES 架构守护检查开始..."

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查结果统计
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0

# 检查函数
check_rule() {
    local rule_name="$1"
    local check_command="$2"
    local success_message="$3"
    local failure_message="$4"
    
    echo -n "🔍 检查 $rule_name... "
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    if eval "$check_command" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ $success_message${NC}"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
        return 0
    else
        echo -e "${RED}❌ $failure_message${NC}"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
        return 1
    fi
}

# 检查函数（允许失败但显示详情）
check_rule_with_details() {
    local rule_name="$1"
    local check_command="$2"
    local success_message="$3"
    local failure_message="$4"
    
    echo -n "🔍 检查 $rule_name... "
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    local result
    result=$(eval "$check_command" 2>&1)
    
    if [ -z "$result" ]; then
        echo -e "${GREEN}✅ $success_message${NC}"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
        return 0
    else
        echo -e "${RED}❌ $failure_message${NC}"
        echo -e "${YELLOW}详细信息：${NC}"
        echo "$result" | head -10  # 只显示前10行
        if [ $(echo "$result" | wc -l) -gt 10 ]; then
            echo "... (还有更多，请检查完整输出)"
        fi
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
        return 1
    fi
}

echo "📋 开始执行架构规则检查..."

# ============================================================================
# 规则 1：检查是否有直接实现公共接口的情况
# ============================================================================

echo -e "\n${YELLOW}=== 规则 1：三层架构分离检查 ===${NC}"

check_rule_with_details \
    "直接实现公共接口检测" \
    "find internal/core -name '*.go' -exec grep -l 'pkg/interfaces/' {} \; | xargs -I {} grep -L 'internal/core/.*/interfaces' {} | grep -v 'internal/core/.*/interfaces/'" \
    "无直接实现公共接口的违规" \
    "发现直接实现公共接口的文件"

# ============================================================================
# 规则 2：检查跨模块依赖
# ============================================================================

echo -e "\n${YELLOW}=== 规则 2：模块边界检查 ===${NC}"

check_rule_with_details \
    "engines 模块依赖 execution 检测" \
    "find internal/core/engines -name '*.go' -exec grep -l 'github.com/weisyn/v1/internal/core/ispc' {} \;" \
    "engines 模块未违规依赖 execution" \
    "engines 模块违规依赖 execution"

check_rule_with_details \
    "execution 模块依赖具体 engines 实现检测" \
    "find internal/core/ispc -name '*.go' -exec grep -l 'github.com/weisyn/v1/internal/core/engines' {} \; | grep -v interfaces" \
    "execution 模块未违规依赖具体 engines 实现" \
    "execution 模块违规依赖具体 engines 实现"

# ============================================================================
# 规则 3：Manager 复杂度检查
# ============================================================================

echo -e "\n${YELLOW}=== 规则 3：Manager 薄实现检查 ===${NC}"

check_rule_with_details \
    "Manager 文件复杂度检测" \
    "find internal/core -name 'manager.go' -exec sh -c 'wc -l < \"$1\" | awk \"\\$1 > 200 {print \\\"$1:\\\" \\$1 \\\" lines\\\"}\"' _ {} \;" \
    "所有 Manager 文件复杂度合规 (≤200行)" \
    "发现复杂度过高的 Manager 文件"

# ============================================================================
# 规则 4：接口定义与实现位置一致性检查
# ============================================================================

echo -e "\n${YELLOW}=== 规则 4：接口定义与实现位置一致性检查 ===${NC}"

# 检查 engines 接口是否在对应位置实现
check_rule \
    "engines 接口实现位置检查" \
    "[ -d internal/core/engines ] && [ -f pkg/interfaces/engines/wasm.go ]" \
    "engines 接口定义与实现位置一致" \
    "engines 接口定义与实现位置不一致"

# 检查 execution 接口是否在对应位置实现  
check_rule \
    "execution 接口实现位置检查" \
    "[ -d internal/core/ispc ] && [ -f pkg/interfaces/ispc/coordinator.go ]" \
    "execution 接口定义与实现位置一致" \
    "execution 接口定义与实现位置不一致"

# ============================================================================
# 规则 5：内部接口继承公共接口检查
# ============================================================================

echo -e "\n${YELLOW}=== 规则 5：内部接口继承检查 ===${NC}"

check_rule_with_details \
    "内部接口继承公共接口检测" \
    "find internal/core/*/interfaces -name '*.go' -exec grep -L 'pkg/interfaces/' {} \;" \
    "所有内部接口都正确继承公共接口" \
    "发现未继承公共接口的内部接口"

# ============================================================================
# 规则 6：禁用词检查
# ============================================================================

echo -e "\n${YELLOW}=== 规则 6：代码规范检查 ===${NC}"

# 检查是否使用了弃用的 _* 目录
check_rule_with_details \
    "弃用目录引用检测" \
    "find internal/core -name '*.go' -exec grep -l 'internal/core/_' {} \;" \
    "未发现对弃用目录的引用" \
    "发现对弃用目录的引用"

# 检查是否有硬编码的字符串（应使用常量）
check_rule_with_details \
    "WASM 函数名硬编码检测" \
    "find internal/core/engines/wasm -name '*.go' -exec grep -n '\"get_caller\"\\|\"query_utxo_balance\"\\|\"execute_utxo_transfer\"' {} \; | grep -v 'const\\|var\\|//'" \
    "未发现 WASM 函数名硬编码" \
    "发现 WASM 函数名硬编码，应使用 wasm_abi.go 中的常量"

# ============================================================================
# 结果统计
# ============================================================================

echo -e "\n${YELLOW}=== 检查结果统计 ===${NC}"
echo "总检查项: $TOTAL_CHECKS"
echo -e "通过: ${GREEN}$PASSED_CHECKS${NC}"
echo -e "失败: ${RED}$FAILED_CHECKS${NC}"

if [ $FAILED_CHECKS -eq 0 ]; then
    echo -e "\n${GREEN}🎉 所有架构检查通过！${NC}"
    exit 0
else
    echo -e "\n${RED}⚠️  发现 $FAILED_CHECKS 个架构问题，请修复后重试${NC}"
    echo -e "\n${YELLOW}💡 修复建议：${NC}"
    echo "1. 查看上述详细错误信息"
    echo "2. 参考 docs/architecture/ARCHITECTURE_RULES.md"
    echo "3. 使用 'make arch-fix' 尝试自动修复部分问题"
    exit 1
fi
