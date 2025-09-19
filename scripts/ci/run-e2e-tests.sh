#!/bin/bash

# WES E2E测试自动化脚本
# 自动启动节点、运行测试、生成报告

set -e

echo "🚀 WES E2E测试自动化执行"
echo "=========================="

PROJECT_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$PROJECT_ROOT"

# 配置参数
NODE_CONFIG="configs_new/environments/local/single-node.json"
TEST_MODE="${1:-clean}"  # clean 或 persistent
TIMEOUT="${2:-300}"      # 测试超时时间（秒）

echo "📋 测试配置："
echo "  - 模式: $TEST_MODE"
echo "  - 超时: ${TIMEOUT}秒"
echo "  - 配置: $NODE_CONFIG"
echo ""

# 函数：清理环境
cleanup() {
    echo "🧹 清理测试环境..."
    pkill -f "bin/node" 2>/dev/null || true
    sleep 2
}

# 函数：启动节点
start_node() {
    echo "🔥 启动WES节点..."
    
    # 确保二进制文件存在
    if [ ! -f "bin/node" ]; then
        echo "📦 构建节点程序..."
        ./scripts/build.sh
    fi
    
    # 检查配置文件
    if [ ! -f "$NODE_CONFIG" ]; then
        echo "❌ 配置文件不存在: $NODE_CONFIG"
        exit 1
    fi
    
    # 启动节点
    ./bin/node --config "$NODE_CONFIG" > test_new/logs/node.log 2>&1 &
    NODE_PID=$!
    echo "📍 节点进程ID: $NODE_PID"
    
    # 等待节点启动
    echo "⏳ 等待节点启动..."
    for i in {1..30}; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            echo "✅ 节点启动成功！"
            return 0
        fi
        sleep 2
        echo "   等待中... (${i}/30)"
    done
    
    echo "❌ 节点启动超时"
    cleanup
    exit 1
}

# 函数：运行测试
run_tests() {
    echo "🧪 运行E2E测试..."
    
    # 创建测试报告目录
    mkdir -p test_new/docs/reports
    
    local test_dir=""
    case "$TEST_MODE" in
        "clean")
            test_dir="test_new/e2e/clean"
            ;;
        "persistent")
            test_dir="test_new/e2e/persistent"
            ;;
        *)
            test_dir="test_new/e2e/scenarios"
            ;;
    esac
    
    echo "📂 测试目录: $test_dir"
    
    # 运行Go测试
    if [ -d "$test_dir" ] && [ "$(ls -A $test_dir/*.go 2>/dev/null)" ]; then
        echo "🔬 运行Go测试..."
        timeout $TIMEOUT go test -v "$test_dir"/*.go > "test_new/docs/reports/e2e-$(date +%Y%m%d-%H%M%S).log" 2>&1
        echo "✅ Go测试完成"
    else
        echo "⚠️ 没有找到Go测试文件，跳过"
    fi
    
    # 运行脚本测试
    if [ -f "test_new/scripts/automation/e2e_dht_persist.sh" ]; then
        echo "🔧 运行脚本测试..."
        timeout $TIMEOUT ./test_new/scripts/automation/e2e_dht_persist.sh
        echo "✅ 脚本测试完成"
    fi
}

# 函数：生成报告
generate_report() {
    echo "📊 生成测试报告..."
    
    local report_file="test_new/docs/reports/e2e-summary-$(date +%Y%m%d-%H%M%S).md"
    
    cat > "$report_file" << EOF
# WES E2E测试报告

**执行时间**: $(date)  
**测试模式**: $TEST_MODE  
**配置文件**: $NODE_CONFIG  

## 测试结果

$(if [ $? -eq 0 ]; then echo "✅ 测试通过"; else echo "❌ 测试失败"; fi)

## 环境信息

- 节点版本: $(./bin/node --version 2>/dev/null || echo "未知")
- 测试超时: ${TIMEOUT}秒
- 日志文件: test_new/logs/node.log

## 详细日志

详见: test_new/docs/reports/

EOF
    
    echo "📄 报告生成: $report_file"
}

# 主流程
main() {
    # 设置trap确保清理
    trap cleanup EXIT
    
    echo "🏁 开始E2E测试流程..."
    
    # 根据模式清理环境
    if [ "$TEST_MODE" = "clean" ]; then
        echo "🧹 清理模式：清空所有数据"
        ./test_new/scripts/cleanup/clean-environment.sh <<< "y"
    else
        echo "📊 继承模式：保留现有数据"
        cleanup  # 只停止进程，不删除数据
    fi
    
    # 创建日志目录
    mkdir -p test_new/logs
    
    # 启动节点
    start_node
    
    # 运行测试
    run_tests
    
    # 生成报告
    generate_report
    
    echo ""
    echo "🎉 E2E测试完成！"
    echo ""
    echo "📋 查看结果："
    echo "  - 节点日志: test_new/logs/node.log"
    echo "  - 测试报告: test_new/docs/reports/"
    echo "  - 节点状态: curl http://localhost:8080/health"
}

# 检查参数
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo "用法: $0 [模式] [超时时间]"
    echo ""
    echo "模式:"
    echo "  clean      - 纯净模式（清空所有数据）"
    echo "  persistent - 继承模式（保留现有数据）"
    echo ""
    echo "示例:"
    echo "  $0 clean 300      # 纯净模式，300秒超时"
    echo "  $0 persistent     # 继承模式，默认超时"
    exit 0
fi

# 执行主流程
main
