#!/bin/bash
# 场景 1：公链同步测试
# 目的：观察长时间同步 + 收/发区块 + GossipSub 流量下的内存曲线
#
# 运行时长建议：
#   - 快速验证：10-30 分钟
#   - 稳定性测试：2-24 小时
#
# 使用方法：
#   chmod +x scripts/memory_scenarios/public_sync.sh
#   ./scripts/memory_scenarios/public_sync.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DATA_DIR="${PROJECT_ROOT}/data/memory-test/public-sync"
LOG_DIR="${DATA_DIR}/logs"

echo "=========================================="
echo "场景 1：公链同步内存测试"
echo "=========================================="
echo "数据目录: ${DATA_DIR}"
echo "日志目录: ${LOG_DIR}"
echo ""
echo "⚠️  注意："
echo "  - 此场景会连接到 WES 主网，需要网络连接"
echo "  - 主要压力在：P2P、Sync、Block/Tx 存储索引等"
echo "  - 预期：RSS 在启动 10-30 分钟后趋于稳定"
echo ""

# 清理旧数据（可选）
if [ "${1}" = "--clean" ]; then
    echo "清理旧数据..."
    rm -rf "${DATA_DIR}"
    echo "✅ 清理完成"
    echo ""
fi

# 创建目录
mkdir -p "${LOG_DIR}"

# 设置环境变量：日志只写入文件，不刷屏
export WES_CLI_MODE=true

# 启动节点
echo "启动节点（公链模式）..."
echo "  日志文件: ${LOG_DIR}/node-system.log"
echo "  按 Ctrl+C 停止节点"
echo ""

cd "${PROJECT_ROOT}"

# 使用 go run 或编译后的二进制
if [ -f "./bin/weisyn-node" ]; then
    ./bin/weisyn-node --chain public --data-dir "${DATA_DIR}" 2>&1 | tee "${LOG_DIR}/node-console.log"
else
    go run ./cmd/node --chain public --data-dir "${DATA_DIR}" 2>&1 | tee "${LOG_DIR}/node-console.log"
fi

