#!/bin/bash

# WES项目依赖检查脚本
# 检查开发和构建所需的所有依赖

set -e

echo "🔍 WES项目依赖检查"
echo "===================="

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 检查结果统计
PASS=0
FAIL=0
WARN=0

check_command() {
    local cmd=$1
    local name=$2
    local required=$3
    
    if command -v "$cmd" &> /dev/null; then
        version=$($cmd version 2>/dev/null || $cmd --version 2>/dev/null || echo "未知")
        echo "✅ $name: $version"
        ((PASS++))
    else
        if [[ "$required" == "required" ]]; then
            echo "❌ $name: 未安装 (必需)"
            ((FAIL++))
        else
            echo "⚠️  $name: 未安装 (可选)"
            ((WARN++))
        fi
    fi
}

echo "📋 核心依赖检查:"
check_command "go" "Go语言" "required"
check_command "git" "Git版本控制" "required"

echo ""
echo "📋 构建工具检查:"
check_command "make" "Make构建工具" "optional"
check_command "tinygo" "TinyGo (合约编译)" "optional"

echo ""
echo "📋 开发工具检查:"
check_command "curl" "网络工具curl" "required"
check_command "jq" "JSON处理工具" "optional"
check_command "protoc" "Protocol Buffer编译器" "optional"

echo ""
echo "📋 测试工具检查:"
check_command "docker" "Docker容器" "optional"
check_command "docker-compose" "Docker Compose" "optional"

echo ""
echo "📋 Go模块依赖检查:"
if [[ -f "go.mod" ]]; then
    echo "✅ go.mod 文件存在"
    echo "🔄 检查依赖完整性..."
    if go mod verify &>/dev/null; then
        echo "✅ Go模块依赖验证成功"
        ((PASS++))
    else
        echo "❌ Go模块依赖验证失败"
        echo "💡 尝试运行: go mod tidy && go mod download"
        ((FAIL++))
    fi
else
    echo "❌ go.mod 文件不存在"
    ((FAIL++))
fi

echo ""
echo "📋 项目文件检查:"
required_dirs=("cmd" "internal" "pkg" "configs")
for dir in "${required_dirs[@]}"; do
    if [[ -d "$dir" ]]; then
        echo "✅ 目录 $dir/ 存在"
        ((PASS++))
    else
        echo "❌ 目录 $dir/ 不存在"
        ((FAIL++))
    fi
done

echo ""
echo "📊 检查结果统计:"
echo "✅ 通过: $PASS"
echo "❌ 失败: $FAIL"  
echo "⚠️  警告: $WARN"

if [[ $FAIL -eq 0 ]]; then
    echo ""
    echo "🎉 所有必需依赖检查通过！"
    echo "🚀 可以开始构建项目了:"
    echo "   ./scripts/build/build.sh"
    exit 0
else
    echo ""
    echo "💡 请先安装缺失的必需依赖，然后重新运行检查。"
    
    echo ""
    echo "📖 安装指南:"
    echo "  Go语言: https://golang.org/doc/install"
    echo "  TinyGo:  https://tinygo.org/getting-started/install/"
    echo "  Docker:  https://docs.docker.com/get-docker/"
    
    exit 1
fi
