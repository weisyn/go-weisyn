#!/bin/bash

# ============================================================================
# 真实环境分叉测试脚本
# ============================================================================
# 用途：在真实节点上测试分叉和双花功能
# ============================================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  真实环境分叉测试${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 配置
DATA_DIR="data/test_real_fork_$(date +%s)"
LOG_FILE="${DATA_DIR}/weisyn.log"
PID_FILE="${DATA_DIR}/weisyn.pid"

# 创建数据目录
mkdir -p ${DATA_DIR}

echo -e "${YELLOW}步骤1: 启动测试节点...${NC}"

# 启动节点（使用development模式）
go run cmd/development/main.go \
    > ${LOG_FILE} 2>&1 &

NODE_PID=$!
echo ${NODE_PID} > ${PID_FILE}

echo -e "${GREEN}✓ 节点已启动 (PID: ${NODE_PID})${NC}"
echo ""

# 等待节点启动
echo -e "${YELLOW}步骤2: 等待节点就绪...${NC}"
sleep 5

# 检查节点是否运行
if ! kill -0 ${NODE_PID} 2>/dev/null; then
    echo -e "${RED}✗ 节点启动失败！${NC}"
    echo -e "${RED}查看日志: ${LOG_FILE}${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 节点运行中${NC}"
echo ""

# 测试分叉检测
echo -e "${YELLOW}步骤3: 测试分叉检测...${NC}"
echo "监控日志中的分叉检测信息..."
echo ""

# 监控日志（10秒） - 使用macOS兼容的方式
(tail -f ${LOG_FILE} | grep -i "fork\|双花\|分叉" &) &
TAIL_PID=$!
sleep 10
kill ${TAIL_PID} 2>/dev/null || true

echo ""
echo -e "${GREEN}✓ 分叉检测监控完成${NC}"
echo ""

# 停止节点
echo -e "${YELLOW}步骤4: 停止节点...${NC}"
kill ${NODE_PID} 2>/dev/null || true
sleep 2

echo -e "${GREEN}✓ 节点已停止${NC}"
echo ""

# 分析日志
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  测试结果分析${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

echo "日志文件: ${LOG_FILE}"
echo ""

echo "分叉相关日志:"
grep -i "fork\|分叉" ${LOG_FILE} | head -10 || echo "无分叉检测日志"
echo ""

echo "双花相关日志:"
grep -i "double\|双花" ${LOG_FILE} | head -10 || echo "无双花检测日志"
echo ""

echo "UTXO相关日志:"
grep -i "utxo\|快照\|回滚\|重放" ${LOG_FILE} | head -10 || echo "无UTXO操作日志"
echo ""

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  测试完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

echo "清理测试数据:"
echo "  rm -rf ${DATA_DIR}"
echo ""

echo -e "${YELLOW}按任意键清理测试数据...${NC}"
read -n 1

rm -rf ${DATA_DIR}
echo -e "${GREEN}✓ 测试数据已清理${NC}"

