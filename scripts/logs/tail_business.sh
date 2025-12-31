#!/bin/bash
# 查看业务日志脚本
# 用途：快速查看业务日志（API、合约执行等），过滤掉系统日志（P2P、共识等）

set -e

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 默认日志目录（基于环境变量或默认值）
LOG_DIR="${WES_LOG_DIR:-$PROJECT_ROOT/data/testing/logs}"

# 如果指定了环境，使用对应的日志目录
if [ -n "$1" ]; then
    case "$1" in
        dev|development)
            LOG_DIR="$PROJECT_ROOT/data/development/single/logs"
            ;;
        test|testing)
            LOG_DIR="$PROJECT_ROOT/data/testing/logs"
            ;;
        prod|production)
            LOG_DIR="$PROJECT_ROOT/data/production/logs"
            ;;
        *)
            LOG_DIR="$1"
            ;;
    esac
fi

BUSINESS_LOG="$LOG_DIR/node-business.log"

# 检查文件是否存在
if [ ! -f "$BUSINESS_LOG" ]; then
    echo "❌ 业务日志文件不存在: $BUSINESS_LOG"
    echo ""
    echo "提示："
    echo "  1. 确保节点已启动并启用了多文件日志"
    echo "  2. 检查日志目录路径是否正确"
    echo "  3. 使用环境变量指定日志目录: WES_LOG_DIR=/path/to/logs $0"
    exit 1
fi

echo "📋 查看业务日志: $BUSINESS_LOG"
echo "   按 Ctrl+C 退出"
echo ""

# 使用 tail -f 实时查看日志
# 如果安装了 jq，可以使用 jq 格式化 JSON 日志
if command -v jq &> /dev/null; then
    tail -f "$BUSINESS_LOG" | jq -r '.timestamp + " [" + .level + "] " + .message + (if .module then " [module=" + .module + "]" else "" end)'
else
    tail -f "$BUSINESS_LOG"
fi

