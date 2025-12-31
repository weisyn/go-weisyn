#!/bin/bash

# ============================================================================
# 真实环境多节点分叉测试脚本
# ============================================================================
# 用途：启动多个真实节点，模拟分叉场景
# ============================================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  真实环境多节点分叉测试${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 配置
NODE_COUNT=2
BASE_PORT=30000
BASE_DIR="data/test_multi_node_$(date +%s)"

# 创建基础目录
mkdir -p ${BASE_DIR}

echo -e "${YELLOW}步骤1: 准备测试环境...${NC}"
echo "节点数量: ${NODE_COUNT}"
echo "数据目录: ${BASE_DIR}"
echo ""

# 为每个节点创建配置
for i in $(seq 1 ${NODE_COUNT}); do
    NODE_DIR="${BASE_DIR}/node${i}"
    mkdir -p ${NODE_DIR}
    
    cat > ${NODE_DIR}/config.json <<EOF
{
  "node": {
    "id": "node${i}",
    "name": "测试节点${i}",
    "mode": "development"
  },
  "network": {
    "port": $((BASE_PORT + i - 1)),
    "bootstrap_peers": []
  },
  "blockchain": {
    "network": "testnet",
    "consensus": "pow"
  },
  "data": {
    "path": "${NODE_DIR}/blockchain"
  },
  "logging": {
    "level": "debug",
    "file": "${NODE_DIR}/weisyn.log"
  }
}
EOF

    echo -e "${GREEN}✓ 节点${i}配置已创建${NC}"
done

echo ""
echo -e "${YELLOW}步骤2: 启动所有节点...${NC}"

# 启动所有节点
for i in $(seq 1 ${NODE_COUNT}); do
    NODE_DIR="${BASE_DIR}/node${i}"
    
    go run cmd/development/main.go \
        > ${NODE_DIR}/weisyn.log 2>&1 &
    
    PID=$!
    echo ${PID} > ${NODE_DIR}/weisyn.pid
    
    echo -e "${GREEN}✓ 节点${i}已启动 (PID: ${PID})${NC}"
    sleep 2
done

echo ""
echo -e "${GREEN}所有节点已启动！${NC}"
echo ""

# 显示节点信息
echo -e "${BLUE}节点信息:${NC}"
for i in $(seq 1 ${NODE_COUNT}); do
    echo "  节点${i}: http://127.0.0.1:$((BASE_PORT + i - 1))"
    echo "  数据: ${BASE_DIR}/node${i}"
    echo "  日志: ${BASE_DIR}/node${i}/weisyn.log"
    echo ""
done

echo ""
echo -e "${YELLOW}步骤3: 监控节点运行...${NC}"
echo "监控时间: 30秒"
echo "查看分叉检测日志..."
echo ""

# 监控所有节点的日志
for i in $(seq 1 $NODE_COUNT); do
    echo -e "${BLUE}节点${i}日志:${NC}"
    (tail -f ${BASE_DIR}/node${i}/weisyn.log | grep -i "fork\|分叉\|双花" &) &
    TAIL_PID=$!
    sleep 5
    kill ${TAIL_PID} 2>/dev/null || true
    echo ""
done

echo ""
echo -e "${YELLOW}步骤4: 分析测试结果...${NC}"
echo ""

# 分析每个节点的日志
for i in $(seq 1 $NODE_COUNT); do
    echo -e "${BLUE}节点${i}分析:${NC}"
    LOG_FILE="${BASE_DIR}/node${i}/weisyn.log"
    
    FORK_COUNT=$(grep -i "fork\|分叉" ${LOG_FILE} | wc -l || echo 0)
    DOUBLE_SPEND_COUNT=$(grep -i "double\|双花" ${LOG_FILE} | wc -l || echo 0)
    UTXO_COUNT=$(grep -i "utxo\|快照\|回滚" ${LOG_FILE} | wc -l || echo 0)
    
    echo "  分叉检测次数: ${FORK_COUNT}"
    echo "  双花检测次数: ${DOUBLE_SPEND_COUNT}"
    echo "  UTXO操作次数: ${UTXO_COUNT}"
    echo ""
done

echo ""
echo -e "${YELLOW}步骤5: 停止所有节点...${NC}"

# 停止所有节点
for i in $(seq 1 $NODE_COUNT); do
    PID_FILE="${BASE_DIR}/node${i}/weisyn.pid"
    if [ -f ${PID_FILE} ]; then
        PID=$(cat ${PID_FILE})
        kill ${PID} 2>/dev/null || true
        echo -e "${GREEN}✓ 节点${i}已停止${NC}"
    fi
done

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  测试完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

echo "测试数据位置: ${BASE_DIR}"
echo ""
echo "查看详细日志:"
for i in $(seq 1 $NODE_COUNT); do
    echo "  cat ${BASE_DIR}/node${i}/weisyn.log"
done
echo ""

echo -e "${YELLOW}是否清理测试数据？(y/n)${NC}"
read -n 1 answer
echo ""

if [ "$answer" = "y" ]; then
    rm -rf ${BASE_DIR}
    echo -e "${GREEN}✓ 测试数据已清理${NC}"
else
    echo -e "${YELLOW}测试数据保留在: ${BASE_DIR}${NC}"
fi

