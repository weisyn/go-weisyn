#!/bin/bash
# 按模块过滤日志脚本
# 用途：从日志文件中过滤特定模块的日志

set -e

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# 默认日志目录
LOG_DIR="${WES_LOG_DIR:-$PROJECT_ROOT/data/testing/logs}"

# 解析参数
MODULE=""
LOG_FILE=""
FOLLOW=false
ENV=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--module)
            MODULE="$2"
            shift 2
            ;;
        -f|--file)
            LOG_FILE="$2"
            shift 2
            ;;
        -F|--follow)
            FOLLOW=true
            shift
            ;;
        -e|--env)
            ENV="$2"
            shift 2
            ;;
        -h|--help)
            echo "用法: $0 [选项]"
            echo ""
            echo "选项："
            echo "  -m, --module MODULE    要过滤的模块名（如：api, p2p, consensus）"
            echo "  -f, --file FILE        日志文件路径（默认：自动检测）"
            echo "  -F, --follow           实时跟踪日志（类似 tail -f）"
            echo "  -e, --env ENV          环境（dev/test/prod，用于自动检测日志目录）"
            echo "  -h, --help             显示帮助信息"
            echo ""
            echo "示例："
            echo "  $0 -m api -F                    # 实时查看 API 模块日志"
            echo "  $0 -m p2p -e dev               # 查看开发环境的 P2P 模块日志"
            echo "  $0 -m contract -f /path/to/log # 从指定文件查看合约模块日志"
            exit 0
            ;;
        *)
            echo "未知参数: $1"
            echo "使用 -h 或 --help 查看帮助"
            exit 1
            ;;
    esac
done

# 如果没有指定模块，报错
if [ -z "$MODULE" ]; then
    echo "❌ 错误：必须指定模块名（使用 -m 或 --module）"
    echo "使用 -h 或 --help 查看帮助"
    exit 1
fi

# 根据环境设置日志目录
if [ -n "$ENV" ]; then
    case "$ENV" in
        dev|development)
            LOG_DIR="$PROJECT_ROOT/data/development/single/logs"
            ;;
        test|testing)
            LOG_DIR="$PROJECT_ROOT/data/testing/logs"
            ;;
        prod|production)
            LOG_DIR="$PROJECT_ROOT/data/production/logs"
            ;;
    esac
fi

# 如果没有指定日志文件，根据模块类型自动选择
if [ -z "$LOG_FILE" ]; then
    # 系统模块使用 system.log，业务模块使用 business.log
    case "$MODULE" in
        p2p|consensus|storage|network|sync|infra|system)
            LOG_FILE="$LOG_DIR/node-system.log"
            ;;
        api|executor|contract|workbench|tx|business|app)
            LOG_FILE="$LOG_DIR/node-business.log"
            ;;
        *)
            # 未知模块，尝试两个文件
            if [ -f "$LOG_DIR/node-business.log" ]; then
                LOG_FILE="$LOG_DIR/node-business.log"
            elif [ -f "$LOG_DIR/node-system.log" ]; then
                LOG_FILE="$LOG_DIR/node-system.log"
            else
                LOG_FILE="$LOG_DIR/weisyn.log"  # 回退到单文件模式
            fi
            ;;
    esac
fi

# 检查文件是否存在
if [ ! -f "$LOG_FILE" ]; then
    echo "❌ 日志文件不存在: $LOG_FILE"
    exit 1
fi

echo "📋 过滤模块 '$MODULE' 的日志"
echo "   日志文件: $LOG_FILE"
if [ "$FOLLOW" = true ]; then
    echo "   模式: 实时跟踪"
fi
echo ""

# 使用 jq 过滤 JSON 日志（如果可用）
if command -v jq &> /dev/null; then
    if [ "$FOLLOW" = true ]; then
        tail -f "$LOG_FILE" | jq -r --arg module "$MODULE" 'select(.module == $module) | .timestamp + " [" + .level + "] " + .message'
    else
        jq -r --arg module "$MODULE" 'select(.module == $module) | .timestamp + " [" + .level + "] " + .message' "$LOG_FILE"
    fi
else
    # 回退：使用 grep 过滤（适用于 JSON 格式）
    if [ "$FOLLOW" = true ]; then
        tail -f "$LOG_FILE" | grep "\"module\":\"$MODULE\""
    else
        grep "\"module\":\"$MODULE\"" "$LOG_FILE"
    fi
fi

