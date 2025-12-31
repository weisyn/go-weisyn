#!/usr/bin/env bash
# WES 开发环境停止脚本

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/../" && pwd)
LOG_DIR="$ROOT_DIR/data/development/cluster"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}🛑 停止WES开发环境...${NC}"

# 停止指定PID的进程
stop_process() {
    local name=$1
    local pid_file=$2
    
    if [[ -f "$pid_file" ]]; then
        local pid
        pid=$(cat "$pid_file" 2>/dev/null || true)
        
        if [[ -n "$pid" ]] && ps -p "$pid" >/dev/null 2>&1; then
            echo -e "${YELLOW}停止 $name (PID: $pid)...${NC}"
            kill -TERM "$pid" 2>/dev/null || true
            
            # 等待进程停止
            for i in {1..10}; do
                if ! ps -p "$pid" >/dev/null 2>&1; then
                    echo -e "${GREEN}✅ $name 已停止${NC}"
                    break
                fi
                sleep 0.5
            done
            
            # 如果进程仍在运行，强制杀死
            if ps -p "$pid" >/dev/null 2>&1; then
                echo -e "${RED}🔫 强制停止 $name...${NC}"
                kill -KILL "$pid" 2>/dev/null || true
            fi
        else
            echo -e "${YELLOW}$name 未在运行${NC}"
        fi
        
        rm -f "$pid_file"
    else
        echo -e "${YELLOW}未找到 $name 的PID文件${NC}"
    fi
}

# 停止集群节点
stop_process "节点1" "$LOG_DIR/node1.pid"
stop_process "节点2" "$LOG_DIR/node2.pid"

# 尝试停止可能的单节点进程
echo -e "${YELLOW}检查单节点进程...${NC}"
SINGLE_PIDS=$(pgrep -f "configs/development/single.json" 2>/dev/null || true)
if [[ -n "$SINGLE_PIDS" ]]; then
    echo -e "${YELLOW}停止单节点进程...${NC}"
    kill -TERM $SINGLE_PIDS 2>/dev/null || true
    sleep 1
    # 强制杀死仍在运行的进程
    REMAINING_PIDS=$(pgrep -f "configs/development/single.json" 2>/dev/null || true)
    if [[ -n "$REMAINING_PIDS" ]]; then
        kill -KILL $REMAINING_PIDS 2>/dev/null || true
    fi
    echo -e "${GREEN}✅ 单节点进程已停止${NC}"
else
    echo -e "${YELLOW}未发现单节点进程${NC}"
fi

echo
echo -e "${GREEN}🎉 开发环境停止完成！${NC}"

