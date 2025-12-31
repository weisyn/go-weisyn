#!/bin/bash
# 场景 3：高并发交易压测
# 目的：专门压 TxPool / Mempool / Executor / UTXO 索引等模块
#
# 运行时长建议：
#   - 快速验证：5-10 分钟
#   - 压力测试：30-60 分钟
#
# 前置条件：
#   1. 节点已启动（私链或公链）
#   2. 至少有两个账户（from 和 to）
#   3. from 账户有足够余额
#
# 使用方法：
#   chmod +x scripts/memory_scenarios/tx_stress.sh
#   ./scripts/memory_scenarios/tx_stress.sh <from_address> <to_address> [tps] [duration_minutes]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

FROM_ADDR="${1}"
TO_ADDR="${2}"
TPS="${3:-10}"          # 默认 10 TPS
DURATION_MIN="${4:-10}" # 默认 10 分钟

if [ -z "${FROM_ADDR}" ] || [ -z "${TO_ADDR}" ]; then
    echo "用法: $0 <from_address> <to_address> [tps] [duration_minutes]"
    echo ""
    echo "示例:"
    echo "  $0 CUQ3g6P5WmFN289pPn7AAhnQ3T2cZRv2BR CWb1owGnpUaB2JoQPhohpa81Cz9aiqikZG 20 30"
    echo ""
    echo "参数说明："
    echo "  from_address: 发送方地址"
    echo "  to_address:   接收方地址"
    echo "  tps:          每秒交易数（默认 10）"
    echo "  duration_minutes: 压测时长（分钟，默认 10）"
    exit 1
fi

echo "=========================================="
echo "场景 3：高并发交易压测"
echo "=========================================="
echo "发送方: ${FROM_ADDR}"
echo "接收方: ${TO_ADDR}"
echo "TPS: ${TPS}"
echo "时长: ${DURATION_MIN} 分钟"
echo ""
echo "⚠️  注意："
echo "  - 确保节点已启动并运行"
echo "  - 确保发送方账户有足够余额"
echo "  - 此脚本会持续发送交易，请监控节点内存"
echo ""

cd "${PROJECT_ROOT}"

# 计算总交易数
TOTAL_TXS=$((TPS * DURATION_MIN * 60))
INTERVAL_MS=$((1000 / TPS))

echo "开始压测..."
echo "  总交易数: ${TOTAL_TXS}"
echo "  发送间隔: ${INTERVAL_MS}ms"
echo "  按 Ctrl+C 提前停止"
echo ""

SUCCESS=0
FAILED=0
START_TIME=$(date +%s)

# 简单版：循环发送交易（低并发演示）
# 注意：实际生产环境建议使用 Go 程序实现真正的并发压测
for i in $(seq 1 ${TOTAL_TXS}); do
    # 构建并发送交易
    if [ -f "./bin/weisyn-cli" ]; then
        ./bin/weisyn-cli tx build transfer "${TO_ADDR}" 1wes --from "${FROM_ADDR}" > /dev/null 2>&1 && \
        ./bin/weisyn-cli tx send > /dev/null 2>&1 && \
        ((SUCCESS++)) || ((FAILED++))
    else
        go run ./cmd/cli tx build transfer "${TO_ADDR}" 1wes --from "${FROM_ADDR}" > /dev/null 2>&1 && \
        go run ./cmd/cli tx send > /dev/null 2>&1 && \
        ((SUCCESS++)) || ((FAILED++))
    fi
    
    # 显示进度
    if [ $((i % TPS)) -eq 0 ]; then
        ELAPSED=$(($(date +%s) - START_TIME))
        echo "[$(date +%H:%M:%S)] 已发送: ${i}/${TOTAL_TXS} (成功: ${SUCCESS}, 失败: ${FAILED}, 耗时: ${ELAPSED}s)"
    fi
    
    # 控制发送速率（使用 awk 替代 bc，兼容性更好）
    sleep $(awk "BEGIN {printf \"%.3f\", ${INTERVAL_MS}/1000}")
done

END_TIME=$(date +%s)
TOTAL_TIME=$((END_TIME - START_TIME))

echo ""
echo "=========================================="
echo "压测完成"
echo "=========================================="
echo "总交易数: ${TOTAL_TXS}"
echo "成功: ${SUCCESS}"
echo "失败: ${FAILED}"
echo "总耗时: ${TOTAL_TIME} 秒"
echo "实际 TPS: $(awk "BEGIN {printf \"%.2f\", ${SUCCESS}/${TOTAL_TIME}}")"
echo ""
echo "⚠️  请检查节点日志中的 memory_sample 记录，分析内存增长趋势"

