#!/bin/bash
# 内存分析脚本
# 用于诊断节点内存使用情况

set -e

# 默认配置
DIAGNOSTICS_PORT=${DIAGNOSTICS_PORT:-6060}
HOST=${HOST:-localhost}
BASE_URL="http://${HOST}:${DIAGNOSTICS_PORT}"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_header() {
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

# 检查节点是否运行
check_node_running() {
    if ! curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/debug/memory/json" > /dev/null 2>&1; then
        print_error "节点未运行或诊断端口 ${DIAGNOSTICS_PORT} 不可访问"
        echo "请确保:"
        echo "  1. 节点正在运行"
        echo "  2. 诊断端口已启用（默认 6060）"
        echo "  3. 防火墙允许访问该端口"
        exit 1
    fi
}

# 显示使用帮助
show_help() {
    cat << EOF
内存分析工具

用法: $0 [命令] [选项]

命令:
  profile       显示详细的内存分析报告（默认）
  json          显示JSON格式的内存统计
  force-gc      强制执行GC并显示效果
  compare       监控一段时间的内存变化
  heap          生成heap profile（需要go tool pprof）
  goroutine     生成goroutine profile（需要go tool pprof）
  continuous    持续监控内存使用（每30秒一次）
  help          显示此帮助信息

选项:
  --host HOST           节点主机（默认: localhost）
  --port PORT           诊断端口（默认: 6060）
  --duration DURATION   监控时长（用于compare命令，默认: 30s）
  --output FILE         输出到文件而不是屏幕

示例:
  $0 profile                                    # 查看内存分析报告
  $0 json                                       # 查看JSON格式的内存统计
  $0 force-gc                                   # 强制GC
  $0 compare --duration 1m                      # 监控1分钟的内存变化
  $0 heap                                       # 生成heap profile并使用pprof分析
  $0 continuous                                 # 持续监控内存
  $0 profile --host 192.168.1.100 --port 7070  # 分析远程节点

环境变量:
  DIAGNOSTICS_PORT      诊断端口（默认: 6060）
  HOST                  节点主机（默认: localhost）

EOF
}

# 显示内存分析报告
cmd_profile() {
    print_header "内存分析报告"
    curl -s "${BASE_URL}/debug/memory/profile"
}

# 显示JSON格式的内存统计
cmd_json() {
    print_header "内存统计 (JSON)"
    curl -s "${BASE_URL}/debug/memory/json" | python3 -m json.tool 2>/dev/null || curl -s "${BASE_URL}/debug/memory/json"
}

# 强制GC
cmd_force_gc() {
    print_header "强制GC效果分析"
    print_warning "正在执行GC，请稍候..."
    curl -s "${BASE_URL}/debug/memory/force-gc"
}

# 内存对比分析
cmd_compare() {
    local duration="${1:-30s}"
    print_header "内存变化分析 (监控时长: ${duration})"
    print_warning "正在监控，请稍候..."
    curl -s "${BASE_URL}/debug/memory/compare?duration=${duration}"
}

# 生成heap profile
cmd_heap() {
    local output_file="${1:-heap_$(date +%Y%m%d_%H%M%S).prof}"
    print_header "生成 Heap Profile"
    
    if ! command -v go &> /dev/null; then
        print_error "未找到 go 命令，请安装 Go"
        exit 1
    fi
    
    print_warning "正在下载 heap profile..."
    curl -s "${BASE_URL}/debug/pprof/heap" > "${output_file}"
    print_success "已保存到: ${output_file}"
    
    echo ""
    print_warning "正在启动 pprof 分析工具（浏览器）..."
    go tool pprof -http=:8081 "${output_file}" &
    local pprof_pid=$!
    
    echo ""
    print_success "pprof 已启动，请在浏览器中访问: http://localhost:8081"
    echo "按 Ctrl+C 退出"
    wait $pprof_pid
}

# 生成goroutine profile
cmd_goroutine() {
    local output_file="${1:-goroutine_$(date +%Y%m%d_%H%M%S).prof}"
    print_header "生成 Goroutine Profile"
    
    if ! command -v go &> /dev/null; then
        print_error "未找到 go 命令，请安装 Go"
        exit 1
    fi
    
    print_warning "正在下载 goroutine profile..."
    curl -s "${BASE_URL}/debug/pprof/goroutine" > "${output_file}"
    print_success "已保存到: ${output_file}"
    
    echo ""
    print_warning "正在启动 pprof 分析工具（浏览器）..."
    go tool pprof -http=:8081 "${output_file}" &
    local pprof_pid=$!
    
    echo ""
    print_success "pprof 已启动，请在浏览器中访问: http://localhost:8081"
    echo "按 Ctrl+C 退出"
    wait $pprof_pid
}

# 持续监控
cmd_continuous() {
    print_header "持续内存监控"
    echo "每30秒更新一次，按 Ctrl+C 退出"
    echo ""
    
    while true; do
        echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC}"
        curl -s "${BASE_URL}/debug/memory/json" | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(f\"  HeapAlloc:   {data['heap_alloc_mb']:>6} MB\")
print(f\"  HeapSys:     {data['heap_sys_mb']:>6} MB\")
print(f\"  HeapIdle:    {data['heap_idle_mb']:>6} MB\")
print(f\"  Goroutines:  {data['goroutines']:>6}\")
print(f\"  GC Count:    {data['num_gc']:>6}\")
" 2>/dev/null || {
            # 如果没有python3，使用简单的格式
            curl -s "${BASE_URL}/debug/memory/json"
        }
        echo ""
        sleep 30
    done
}

# 解析命令行参数
COMMAND="${1:-profile}"
shift 2>/dev/null || true

DURATION="30s"
OUTPUT_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --host)
            HOST="$2"
            BASE_URL="http://${HOST}:${DIAGNOSTICS_PORT}"
            shift 2
            ;;
        --port)
            DIAGNOSTICS_PORT="$2"
            BASE_URL="http://${HOST}:${DIAGNOSTICS_PORT}"
            shift 2
            ;;
        --duration)
            DURATION="$2"
            shift 2
            ;;
        --output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        *)
            print_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 检查节点运行状态
if [[ "$COMMAND" != "help" ]]; then
    check_node_running
fi

# 执行命令
case "$COMMAND" in
    profile)
        if [[ -n "$OUTPUT_FILE" ]]; then
            cmd_profile > "$OUTPUT_FILE"
            print_success "已保存到: $OUTPUT_FILE"
        else
            cmd_profile
        fi
        ;;
    json)
        if [[ -n "$OUTPUT_FILE" ]]; then
            cmd_json > "$OUTPUT_FILE"
            print_success "已保存到: $OUTPUT_FILE"
        else
            cmd_json
        fi
        ;;
    force-gc|gc)
        cmd_force_gc
        ;;
    compare)
        cmd_compare "$DURATION"
        ;;
    heap)
        cmd_heap "$OUTPUT_FILE"
        ;;
    goroutine)
        cmd_goroutine "$OUTPUT_FILE"
        ;;
    continuous|watch|monitor)
        cmd_continuous
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "未知命令: $COMMAND"
        show_help
        exit 1
        ;;
esac
