#!/bin/bash
# 内存监控自动化测试脚本
# 用途：通过 Cursor Agent 会话自动测试内存监控功能
# 使用方法：bash scripts/test_memory_monitoring.sh

set -e  # 遇到错误立即退出

# 配置
PROJECT_DIR="/Users/qinglong/gopath/src/chaincodes/WES/weisyn.git"
API_URL="http://localhost:28680"
TEST_DURATION=300  # 测试时长（秒）
SAMPLE_INTERVAL=10  # 采样间隔（秒）

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查命令是否存在
check_command() {
    if ! command -v "$1" &> /dev/null; then
        log_error "命令 '$1' 未找到，请先安装"
        exit 1
    fi
}

# 检查依赖
log_info "检查依赖..."
check_command "curl"
check_command "jq"
check_command "go"

cd "$PROJECT_DIR" || exit 1

# 步骤 1: 编译节点
log_info "【步骤 1/6】编译节点..."
if [ ! -f "./bin/weisyn-node" ]; then
    log_info "  编译节点二进制..."
    if command -v make &> /dev/null && [ -f "Makefile" ]; then
        make build-node || go build -o bin/weisyn-node ./cmd/node
    else
        go build -o bin/weisyn-node ./cmd/node
    fi
    log_info "  ✓ 节点编译完成"
else
    log_info "  ✓ 节点已编译"
fi

# 步骤 2: 启动节点
log_info "【步骤 2/6】启动节点..."
NODE_PID_FILE="/tmp/weisyn_node_test.pid"
NODE_LOG_FILE="/tmp/weisyn_node_test.log"

# 检查是否已有节点运行
if [ -f "$NODE_PID_FILE" ]; then
    OLD_PID=$(cat "$NODE_PID_FILE")
    if ps -p "$OLD_PID" > /dev/null 2>&1; then
        log_warn "  检测到已有节点运行（PID: $OLD_PID），停止旧节点..."
        kill "$OLD_PID" 2>/dev/null || true
        sleep 2
    fi
    rm -f "$NODE_PID_FILE"
fi

# 启动节点
log_info "  启动节点进程..."
nohup ./bin/weisyn-node --chain private > "$NODE_LOG_FILE" 2>&1 &
NODE_PID=$!
echo "$NODE_PID" > "$NODE_PID_FILE"
log_info "  节点进程已启动（PID: $NODE_PID）"

# 等待节点就绪
log_info "  等待节点就绪..."
for i in {1..12}; do
    if curl -s "$API_URL/api/v1/health" > /dev/null 2>&1; then
        log_info "  ✓ 节点已就绪"
        break
    fi
    if [ $i -eq 12 ]; then
        log_error "  节点启动超时"
        log_error "  请查看日志: $NODE_LOG_FILE"
        kill "$NODE_PID" 2>/dev/null || true
        exit 1
    fi
    echo "    等待中... ($i/12)"
    sleep 5
done

# 步骤 3: 验证内存监控接口
log_info "【步骤 3/6】验证内存监控接口..."
INITIAL_MEMORY="/tmp/memory_initial.json"
if curl -s "$API_URL/api/v1/system/memory" > "$INITIAL_MEMORY"; then
    INITIAL_RSS_MB=$(jq -r '.runtime.rss_mb // 0' "$INITIAL_MEMORY")
    INITIAL_HEAP=$(jq -r '.runtime.heap_alloc // 0' "$INITIAL_MEMORY")
    INITIAL_HEAP_MB=$((INITIAL_HEAP / 1024 / 1024))
    INITIAL_GOROUTINE=$(jq -r '.runtime.num_goroutine // 0' "$INITIAL_MEMORY")
    MODULE_COUNT=$(jq -r '.modules | length' "$INITIAL_MEMORY")
    log_info "  ✓ 内存监控接口可用"
    log_info "    初始真实内存(RSS): ${INITIAL_RSS_MB} MB"
    log_info "    初始堆内存(HeapAlloc): ${INITIAL_HEAP_MB} MB (Go runtime指标，仅作趋势参考)"
    log_info "    Goroutine 数: ${INITIAL_GOROUTINE}"
    log_info "    模块数量: ${MODULE_COUNT}"
else
    log_error "  无法获取内存数据"
    kill "$NODE_PID" 2>/dev/null || true
    exit 1
fi

# 步骤 4: 创建测试账户（如果需要）
log_info "【步骤 4/6】准备挖矿..."
# 尝试获取一个账户地址用于挖矿
# 这里使用一个示例地址，实际使用时应该从节点获取或创建
MINER_ADDRESS=""
log_warn "  注意: 需要提供有效的矿工地址才能启动挖矿"
log_warn "  可以跳过此步骤，仅监控节点启动后的内存使用"

# 步骤 5: 启动挖矿（可选）
if [ -n "$MINER_ADDRESS" ]; then
    log_info "【步骤 5/6】启动挖矿..."
    MINING_RESPONSE=$(curl -s -X POST "$API_URL/jsonrpc" \
        -H "Content-Type: application/json" \
        -d "{
            \"jsonrpc\": \"2.0\",
            \"id\": 1,
            \"method\": \"wes_startMining\",
            \"params\": [\"$MINER_ADDRESS\"]
        }")
    
    if echo "$MINING_RESPONSE" | jq -e '.result == true' > /dev/null 2>&1; then
        log_info "  ✓ 挖矿已启动"
    else
        log_warn "  挖矿启动失败，继续测试内存监控..."
        log_warn "  响应: $MINING_RESPONSE"
    fi
else
    log_info "【步骤 5/6】跳过挖矿启动（未提供矿工地址）"
fi

# 步骤 6: 监控内存
log_info "【步骤 6/6】监控内存（持续 ${TEST_DURATION} 秒，采样间隔 ${SAMPLE_INTERVAL} 秒）..."
MEMORY_LOG="/tmp/memory_monitor_$(date +%Y%m%d_%H%M%S).csv"
echo "时间,真实内存RSS(MB),堆分配(MB),堆使用(MB),GC次数,Goroutine数,模块数" > "$MEMORY_LOG"

start_time=$(date +%s)
end_time=$((start_time + TEST_DURATION))
sample_count=0

while [ $(date +%s) -lt $end_time ]; do
    sample_count=$((sample_count + 1))
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    response=$(curl -s "$API_URL/api/v1/system/memory")
    
    if [ $? -eq 0 ]; then
        rss_mb=$(jq -r '.runtime.rss_mb // 0' "$response")
        heap_alloc=$(echo "$response" | jq -r '.runtime.heap_alloc // 0')
        heap_inuse=$(echo "$response" | jq -r '.runtime.heap_inuse // 0')
        num_gc=$(echo "$response" | jq -r '.runtime.num_gc // 0')
        num_goroutine=$(echo "$response" | jq -r '.runtime.num_goroutine // 0')
        module_count=$(echo "$response" | jq -r '.modules | length')
        
        heap_alloc_mb=$((heap_alloc / 1024 / 1024))
        heap_inuse_mb=$((heap_inuse / 1024 / 1024))
        
        echo "$timestamp,$rss_mb,$heap_alloc_mb,$heap_inuse_mb,$num_gc,$num_goroutine,$module_count" >> "$MEMORY_LOG"
        
        elapsed=$((TEST_DURATION - (end_time - $(date +%s))))
        printf "  采样 #%d: 真实内存=%dMB, 堆内存=%dMB, Goroutine=%d, 已用时=%d秒\n" \
            "$sample_count" "$rss_mb" "$heap_alloc_mb" "$num_goroutine" "$elapsed"
    else
        log_warn "  采样失败"
    fi
    
    sleep $SAMPLE_INTERVAL
done

# 获取最终内存快照
log_info "获取最终内存快照..."
FINAL_MEMORY="/tmp/memory_final.json"
curl -s "$API_URL/api/v1/system/memory" > "$FINAL_MEMORY"

# 生成报告
echo ""
log_info "=========================================="
log_info "测试完成，生成报告..."
log_info "=========================================="

# 计算内存增长（使用 RSS 作为真实内存指标）
FINAL_RSS_MB=$(jq -r '.runtime.rss_mb // 0' "$FINAL_MEMORY")
RSS_GROWTH_MB=$((FINAL_RSS_MB - INITIAL_RSS_MB))

FINAL_HEAP=$(jq -r '.runtime.heap_alloc // 0' "$FINAL_MEMORY")
FINAL_HEAP_MB=$((FINAL_HEAP / 1024 / 1024))
HEAP_GROWTH=$((FINAL_HEAP - INITIAL_HEAP))
HEAP_GROWTH_MB=$((HEAP_GROWTH / 1024 / 1024))

echo ""
log_info "【真实内存增长统计（RSS）】"
echo "  - 初始真实内存: ${INITIAL_RSS_MB} MB"
echo "  - 最终真实内存: ${FINAL_RSS_MB} MB"
echo "  - 真实内存增长: ${RSS_GROWTH_MB} MB"
echo ""
log_info "【堆内存趋势（Go runtime指标，仅作趋势参考）】"
echo "  - 初始堆内存: ${INITIAL_HEAP_MB} MB"
echo "  - 最终堆内存: ${FINAL_HEAP_MB} MB"
echo "  - 堆内存变化: ${HEAP_GROWTH_MB} MB"

# 分析模块内存使用
echo ""
log_info "【模块内存使用 Top 5】"
jq -r '.modules | sort_by(.approx_bytes) | reverse | .[0:5] | .[] | "  \(.module): \(.approx_bytes / 1024 / 1024 | floor)MB (对象: \(.objects), 队列: \(.queue_length))"' "$FINAL_MEMORY"

# 生成内存开销清单
REPORT_FILE="/tmp/memory_test_report_$(date +%Y%m%d_%H%M%S).txt"
{
    echo "=========================================="
    echo "内存监控测试报告"
    echo "=========================================="
    echo "生成时间: $(date '+%Y-%m-%d %H:%M:%S')"
    echo "测试时长: ${TEST_DURATION} 秒"
    echo "采样次数: ${sample_count}"
    echo ""
    echo "【真实内存增长统计（RSS）】"
    echo "- 初始真实内存: ${INITIAL_RSS_MB} MB"
    echo "- 最终真实内存: ${FINAL_RSS_MB} MB"
    echo "- 真实内存增长: ${RSS_GROWTH_MB} MB"
    echo ""
    echo "【运行时统计】"
    jq -r '"真实内存(RSS): \(.runtime.rss_mb) MB\n堆分配(HeapAlloc): \(.runtime.heap_alloc / 1024 / 1024 | floor) MB (Go runtime指标，仅作趋势参考)\n堆使用(HeapInuse): \(.runtime.heap_inuse / 1024 / 1024 | floor) MB (Go runtime指标，仅作趋势参考)\nGC 次数: \(.runtime.num_gc)\nGoroutine 数: \(.runtime.num_goroutine)"' "$FINAL_MEMORY"
    echo ""
    echo "【模块内存使用（按内存大小排序）】"
    jq -r '.modules | sort_by(.approx_bytes) | reverse | .[] | "\(.module) (\(.layer)):\n  内存: \(.approx_bytes / 1024 / 1024 | floor) MB\n  对象: \(.objects)\n  队列长度: \(.queue_length)\n"' "$FINAL_MEMORY"
} > "$REPORT_FILE"

log_info "✓ 报告已保存到: $REPORT_FILE"
log_info "✓ 内存监控日志已保存到: $MEMORY_LOG"

# 显示报告摘要
echo ""
log_info "【报告摘要】"
cat "$REPORT_FILE" | head -20

echo ""
log_info "=========================================="
log_info "测试完成"
log_info "=========================================="
log_info "节点仍在运行（PID: $NODE_PID）"
log_info "要停止节点，请运行: kill $NODE_PID"
log_info "或查看完整报告: cat $REPORT_FILE"

