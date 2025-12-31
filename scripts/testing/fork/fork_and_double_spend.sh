#!/bin/bash

# ============================================================================
# 分叉和双花测试脚本
# ============================================================================
# 用途：在一台电脑上启动多个节点，模拟分叉和双花场景
# 作者：WeiSyn Team
# 日期：2024
# ============================================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
NODE_COUNT=3
BASE_PORT=30000
DATA_DIR="data/test_fork"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  分叉和双花测试 - 单机多节点${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 清理旧的测试数据
echo -e "${YELLOW}清理旧的测试数据...${NC}"
rm -rf ${DATA_DIR}
mkdir -p ${DATA_DIR}

# 生成测试配置
echo -e "${YELLOW}生成测试配置...${NC}"
for i in $(seq 1 $NODE_COUNT); do
    node_dir="${DATA_DIR}/node${i}"
    mkdir -p ${node_dir}
    
    # 生成节点配置
    cat > ${node_dir}/config.json <<EOF
{
  "node": {
    "id": "node${i}",
    "name": "测试节点${i}",
    "port": $((BASE_PORT + i - 1)),
    "p2p": {
      "listen_address": "/ip4/127.0.0.1/tcp/$((BASE_PORT + i - 1))",
      "bootstrap_peers": []
    }
  },
  "blockchain": {
    "network": "testnet",
    "consensus": "pow",
    "genesis": {
      "timestamp": $(date +%s),
      "difficulty": 1,
      "reward": 50
    }
  },
  "data": {
    "path": "${node_dir}/blockchain",
    "snapshot_dir": "${node_dir}/snapshots"
  },
  "logging": {
    "level": "debug",
    "file": "${node_dir}/weisyn.log"
  }
}
EOF

    echo -e "${GREEN}✓ 节点${i}配置已生成${NC}"
done

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  测试场景说明${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "1. 启动多个节点"
echo "2. 节点1和节点2同时挖矿，产生分叉"
echo "3. 验证分叉处理逻辑"
echo "4. 验证双花检测逻辑"
echo ""
echo -e "${YELLOW}按任意键开始测试...${NC}"
read -n 1

# 启动节点
echo ""
echo -e "${BLUE}启动节点...${NC}"
for i in $(seq 1 $NODE_COUNT); do
    node_dir="${DATA_DIR}/node${i}"
    log_file="${node_dir}/weisyn.log"
    
    echo -e "${YELLOW}启动节点${i}...${NC}"
    go run cmd/development/main.go \
        > ${log_file} 2>&1 &
    
    echo $! > ${node_dir}/pid.txt
    echo -e "${GREEN}✓ 节点${i}已启动 (PID: $(cat ${node_dir}/pid.txt))${NC}"
    
    # 等待节点启动
    sleep 2
done

echo ""
echo -e "${GREEN}所有节点已启动！${NC}"
echo ""
echo "节点信息："
for i in $(seq 1 $NODE_COUNT); do
    echo "  节点${i}: http://127.0.0.1:$((BASE_PORT + i - 1))"
done
echo ""
echo -e "${YELLOW}按任意键停止所有节点...${NC}"
read -n 1

# 停止所有节点
echo ""
echo -e "${YELLOW}停止所有节点...${NC}"
for i in $(seq 1 $NODE_COUNT); do
    pid_file="${DATA_DIR}/node${i}/pid.txt"
    if [ -f ${pid_file} ]; then
        pid=$(cat ${pid_file})
        kill ${pid} 2>/dev/null || true
        echo -e "${GREEN}✓ 节点${i}已停止${NC}"
    fi
done

echo ""
echo -e "${GREEN}测试完成！${NC}"
echo ""
echo "日志文件位置："
for i in $(seq 1 $NODE_COUNT); do
    echo "  节点${i}: ${DATA_DIR}/node${i}/weisyn.log"
done

