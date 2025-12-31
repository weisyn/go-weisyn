#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/../../" && pwd)
BIN="$ROOT_DIR/bin/node"
LOG_DIR="$ROOT_DIR/data/logs"
CFG1="$ROOT_DIR/configs/development/node1.json"
CFG2="$ROOT_DIR/configs/development/node2.json"

mkdir -p "$LOG_DIR"

echo "[P2P] Building node binary..."
go build -o "$BIN" ./cmd/node

# 默认关闭公共引导，避免出网；如需外网引导，可导出为 true
export WES_P2P_ENABLE_DEFAULT_BOOTSTRAPS=${WES_P2P_ENABLE_DEFAULT_BOOTSTRAPS:-false}

echo "[P2P] Starting node-1 with $CFG1"
nohup "$BIN" --config "$CFG1" \
  > "$LOG_DIR/node1.out" 2>&1 & echo $! > "$LOG_DIR/node1.pid"

echo "[P2P] Starting node-2 with $CFG2"
nohup "$BIN" --config "$CFG2" \
  > "$LOG_DIR/node2.out" 2>&1 & echo $! > "$LOG_DIR/node2.pid"

echo "[P2P] Started. PIDs: node1=$(cat "$LOG_DIR/node1.pid"), node2=$(cat "$LOG_DIR/node2.pid")"
echo "[P2P] Diagnostics: node1 http://127.0.0.1:28680, node2 http://127.0.0.1:28681"
echo "[P2P] Tail logs: tail -f $LOG_DIR/node1.out $LOG_DIR/node2.out"


