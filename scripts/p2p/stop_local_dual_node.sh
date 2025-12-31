#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/../../" && pwd)
LOG_DIR="$ROOT_DIR/data/logs"

stop_one() {
  local name=$1
  local pid_file="$LOG_DIR/${name}.pid"
  if [[ -f "$pid_file" ]]; then
    local pid
    pid=$(cat "$pid_file" || true)
    if [[ -n "${pid}" ]] && ps -p "$pid" >/dev/null 2>&1; then
      echo "[P2P] Stopping ${name} (PID ${pid})..."
      kill -TERM "$pid" || true
      # 等待最多 5 秒
      for i in {1..10}; do
        if ! ps -p "$pid" >/dev/null 2>&1; then break; fi
        sleep 0.5
      done
      if ps -p "$pid" >/dev/null 2>&1; then
        echo "[P2P] Force killing ${name} (PID ${pid})..."
        kill -KILL "$pid" || true
      fi
    else
      echo "[P2P] ${name} not running"
    fi
    rm -f "$pid_file"
  else
    echo "[P2P] PID file not found for ${name} ($pid_file)"
  fi
}

stop_one node1
stop_one node2

echo "[P2P] Stopped local dual nodes."


