#!/bin/bash
# 统计 WES 项目代码行数
# 使用 scc 工具：https://github.com/boyter/scc
#
# 安装 scc：
#   brew install scc
#   或
#   go install github.com/boyter/scc/v3@latest
#
# 使用方法：
#   ./scripts/count-loc.sh          # 只统计 Go 代码
#   ./scripts/count-loc.sh --all    # 统计所有代码（Go + Proto + Shell 等）
#   ./scripts/count-loc.sh --detail # 显示详细统计

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 检查 scc 是否安装
if ! command -v scc &> /dev/null; then
    echo -e "${RED}错误: scc 未安装${NC}"
    echo ""
    echo "请先安装 scc："
    echo -e "  ${CYAN}brew install scc${NC}"
    echo "  或"
    echo -e "  ${CYAN}go install github.com/boyter/scc/v3@latest${NC}"
    exit 1
fi

# 切换到项目根目录
cd "$(dirname "$0")/.."

# 默认只统计 Go
MODE="go"
DETAIL=false

# 解析参数
for arg in "$@"; do
    case $arg in
        --all)
            MODE="all"
            ;;
        --detail)
            DETAIL=true
            ;;
        -h|--help)
            echo "WES 代码行数统计工具"
            echo ""
            echo "使用方法："
            echo "  ./scripts/count-loc.sh          # 只统计 Go 代码"
            echo "  ./scripts/count-loc.sh --all    # 统计所有代码"
            echo "  ./scripts/count-loc.sh --detail # 显示详细统计"
            exit 0
            ;;
    esac
done

# 排除的目录
EXCLUDE_DIRS=".git,.github,vendor,node_modules,_dev,data,testdata,mocks"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}📊 WES 代码行数统计${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

if [ "$DETAIL" = true ]; then
    # 详细模式：显示完整 scc 输出
    if [ "$MODE" = "go" ]; then
        echo -e "${YELLOW}▶ Go 代码详细统计${NC}"
        scc --exclude-dir "$EXCLUDE_DIRS" \
            --include-ext go \
            --no-cocomo \
            .
    else
        echo -e "${YELLOW}▶ 所有代码详细统计${NC}"
        scc --exclude-dir "$EXCLUDE_DIRS" \
            --no-cocomo \
            .
    fi
else
    # 简洁模式
    if [ "$MODE" = "go" ]; then
        echo -e "${YELLOW}▶ 统计 Go 代码...${NC}"
        echo ""
        
        # 使用 scc 的标准表格输出，解析 Go 行
        # 格式: Language | Files | Lines | Blanks | Comments | Code | Complexity
        RESULT=$(scc --exclude-dir "$EXCLUDE_DIRS" \
            --include-ext go \
            --no-cocomo \
            . 2>/dev/null | grep "^Go ")
        
        if [ -n "$RESULT" ]; then
            # 解析标准输出格式
            FILES=$(echo "$RESULT" | awk '{print $2}')
            LINES=$(echo "$RESULT" | awk '{print $3}')
            BLANKS=$(echo "$RESULT" | awk '{print $4}')
            COMMENTS=$(echo "$RESULT" | awk '{print $5}')
            CODE=$(echo "$RESULT" | awk '{print $6}')
            
            # 计算万行
            LINES_WAN=$(echo "scale=1; $LINES / 10000" | bc)
            CODE_WAN=$(echo "scale=1; $CODE / 10000" | bc)
            
            echo -e "  ${GREEN}Go 文件数:${NC}   $FILES 个"
            echo -e "  ${GREEN}总行数:${NC}     $LINES 行 (约 ${CYAN}${LINES_WAN} 万行${NC})"
            echo -e "  ${GREEN}代码行:${NC}     $CODE 行 (约 ${CYAN}${CODE_WAN} 万行${NC})"
            echo -e "  ${GREEN}注释行:${NC}     $COMMENTS 行"
            echo -e "  ${GREEN}空行:${NC}       $BLANKS 行"
            
            # 输出适合放入 README 的格式
            echo ""
            echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
            echo -e "${YELLOW}📝 README 徽章格式:${NC}"
            echo ""
            echo "<sub>📊 代码规模：${LINES_WAN} 万行 Go 代码</sub>"
        else
            echo -e "${RED}统计失败${NC}"
            exit 1
        fi
    else
        echo -e "${YELLOW}▶ 统计所有代码...${NC}"
        echo ""
        
        # 获取总计行（从标准表格输出解析 Total 行）
        TOTAL_RESULT=$(scc --exclude-dir "$EXCLUDE_DIRS" \
            --no-cocomo \
            . 2>/dev/null | grep "^Total ")
        
        if [ -n "$TOTAL_RESULT" ]; then
            TOTAL_FILES=$(echo "$TOTAL_RESULT" | awk '{print $2}')
            TOTAL_LINES=$(echo "$TOTAL_RESULT" | awk '{print $3}')
            TOTAL_BLANKS=$(echo "$TOTAL_RESULT" | awk '{print $4}')
            TOTAL_COMMENTS=$(echo "$TOTAL_RESULT" | awk '{print $5}')
            TOTAL_CODE=$(echo "$TOTAL_RESULT" | awk '{print $6}')
            
            TOTAL_WAN=$(echo "scale=1; $TOTAL_LINES / 10000" | bc)
            CODE_WAN=$(echo "scale=1; $TOTAL_CODE / 10000" | bc)
            
            echo -e "  ${GREEN}总文件数:${NC}   $TOTAL_FILES 个"
            echo -e "  ${GREEN}总行数:${NC}     $TOTAL_LINES 行 (约 ${CYAN}${TOTAL_WAN} 万行${NC})"
            echo -e "  ${GREEN}代码行:${NC}     $TOTAL_CODE 行 (约 ${CYAN}${CODE_WAN} 万行${NC})"
            echo -e "  ${GREEN}注释行:${NC}     $TOTAL_COMMENTS 行"
            echo -e "  ${GREEN}空行:${NC}       $TOTAL_BLANKS 行"
        fi
        
        echo ""
        echo -e "${YELLOW}▶ 各语言统计:${NC}"
        scc --exclude-dir "$EXCLUDE_DIRS" \
            --no-cocomo \
            . 2>/dev/null | head -25
    fi
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

