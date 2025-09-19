#!/usr/bin/env bash
# WES 开发环境启动脚本 - 支持单节点和集群模式

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/../" && pwd)
BIN="$ROOT_DIR/bin/node"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 WES 开发环境启动器${NC}"
echo

# 构建节点程序
echo -e "${YELLOW}📦 构建节点程序...${NC}"
go build -o "$BIN" ./cmd/node
echo -e "${GREEN}✅ 构建完成${NC}"
echo

# 显示启动选项
echo -e "${BLUE}请选择启动模式:${NC}"
echo "1) 单节点开发模式 (端口: 8080)"
echo "2) 双节点集群模式 (端口: 8080, 8082)"
echo "3) 退出"
echo

read -p "请输入选择 (1-3): " choice

case $choice in
  1)
    echo -e "${GREEN}🔥 启动单节点开发模式...${NC}"
    CFG="$ROOT_DIR/configs/development/single.json"
    
    echo "配置文件: $CFG"
    echo "数据目录: ./data/development/single/"
    echo "HTTP API: http://localhost:8080"
    echo "WebSocket: ws://localhost:8081"
    echo "gRPC: localhost:9090"
    echo
    
    echo -e "${YELLOW}按 Ctrl+C 停止节点${NC}"
    echo "----------------------------------------"
    exec "$BIN" --config "$CFG"
    ;;
    
  2)
    echo -e "${GREEN}🔥 启动双节点集群模式...${NC}"
    CFG1="$ROOT_DIR/configs/development/node1.json"
    CFG2="$ROOT_DIR/configs/development/node2.json"
    LOG_DIR="$ROOT_DIR/data/development/cluster"
    
    mkdir -p "$LOG_DIR/node1/logs" "$LOG_DIR/node2/logs"
    
    echo "节点1配置: $CFG1"
    echo "节点2配置: $CFG2"
    echo "节点1 API: http://localhost:8080"
    echo "节点2 API: http://localhost:8082"
    echo
    
    echo -e "${YELLOW}启动节点1...${NC}"
    nohup "$BIN" --config "$CFG1" > "$LOG_DIR/node1/logs/output.log" 2>&1 & 
    NODE1_PID=$!
    echo "节点1 PID: $NODE1_PID"
    
    echo -e "${YELLOW}启动节点2...${NC}"
    nohup "$BIN" --config "$CFG2" > "$LOG_DIR/node2/logs/output.log" 2>&1 &
    NODE2_PID=$!
    echo "节点2 PID: $NODE2_PID"
    
    echo
    echo -e "${GREEN}✅ 双节点集群启动完成！${NC}"
    echo "节点1 PID: $NODE1_PID (http://localhost:8080)"
    echo "节点2 PID: $NODE2_PID (http://localhost:8082)"
    echo
    echo "查看日志:"
    echo "  节点1: tail -f $LOG_DIR/node1/logs/output.log"
    echo "  节点2: tail -f $LOG_DIR/node2/logs/output.log"
    echo
    echo "停止集群:"
    echo "  kill $NODE1_PID $NODE2_PID"
    echo
    
    # 保存PID以便后续停止
    echo "$NODE1_PID" > "$LOG_DIR/node1.pid"
    echo "$NODE2_PID" > "$LOG_DIR/node2.pid"
    echo -e "${BLUE}PID已保存，可使用 ./scripts/stop_development.sh 停止集群${NC}"
    ;;
    
  3)
    echo -e "${YELLOW}👋 退出${NC}"
    exit 0
    ;;
    
  *)
    echo -e "${RED}❌ 无效选择${NC}"
    exit 1
    ;;
esac

