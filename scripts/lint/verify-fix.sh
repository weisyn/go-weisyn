#!/bin/bash
# 验证修复脚本
# 用途：验证特定文件或检查器的问题是否已修复

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

show_help() {
    cat << EOF
用法: $0 [选项] [文件路径]

验证 lint 问题是否已修复

选项:
  -h, --help              显示帮助信息
  -f, --file PATH         验证特定文件
  -l, --linter NAME       验证特定检查器
  -a, --all               验证所有问题（重新运行检查）

示例:
  $0 -f internal/core/chain/manager.go    # 验证特定文件
  $0 -l errcheck                          # 验证 errcheck 问题
  $0 -a                                   # 重新检查所有问题

EOF
}

VERIFY_FILE=""
VERIFY_LINTER=""
VERIFY_ALL=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -f|--file)
            VERIFY_FILE="$2"
            shift 2
            ;;
        -l|--linter)
            VERIFY_LINTER="$2"
            shift 2
            ;;
        -a|--all)
            VERIFY_ALL=true
            shift
            ;;
        *)
            VERIFY_FILE="$1"
            shift
            ;;
    esac
done

if [ "$VERIFY_ALL" = true ]; then
    echo "🔄 重新运行完整检查..."
    ./bin/golangci-lint run --out-format json > /tmp/lint-verify.json 2>/dev/null || true
    
    python3 << 'PYTHON_SCRIPT'
import json

try:
    with open('/tmp/lint-verify.json', 'r') as f:
        data = json.load(f)
    
    issues = data.get('Issues', [])
    total = len(issues)
    
    print(f"📊 当前问题统计:")
    print(f"总问题数: {total} 个")
    
    if total == 0:
        print("✅ 恭喜！所有问题已修复！")
    else:
        # 按检查器统计
        linter_counts = {}
        for issue in issues:
            linter = issue.get('FromLinter', 'unknown')
            linter_counts[linter] = linter_counts.get(linter, 0) + 1
        
        print("\n按检查器统计:")
        for linter, count in sorted(linter_counts.items(), key=lambda x: x[1], reverse=True):
            print(f"  {linter}: {count} 个")
        
        print(f"\n💡 还有 {total} 个问题需要修复")

except Exception as e:
    print(f"❌ 错误: {e}")
    exit(1)
PYTHON_SCRIPT
    
    exit 0
fi

if [ -z "$VERIFY_FILE" ] && [ -z "$VERIFY_LINTER" ]; then
    echo "❌ 请指定要验证的文件或检查器"
    show_help
    exit 1
fi

if [ ! -f ".lint-report.json" ]; then
    echo "❌ 报告文件不存在: .lint-report.json"
    echo "💡 请先运行: make lint-check"
    exit 1
fi

echo "🔍 验证修复情况..."

if [ -n "$VERIFY_FILE" ]; then
    echo "验证文件: $VERIFY_FILE"
    ./bin/golangci-lint run "$VERIFY_FILE" 2>&1 | head -50
    
    python3 << PYTHON_SCRIPT
import json

try:
    with open('.lint-report.json', 'r') as f:
        data = json.load(f)
    
    # 支持新旧格式
    if 'all_issues' in data:
        all_issues = data['all_issues']
    else:
        all_issues = []
        for file_path, issues in data.get('issues_by_file', {}).items():
            all_issues.extend(issues)
    
    file_issues = [i for i in all_issues if '$VERIFY_FILE' in i['file']]
    
    if file_issues:
        print(f"\n📋 问题列表中该文件有 {len(file_issues)} 个问题:")
        for issue in file_issues[:10]:
            print(f"  - 第 {issue['line']} 行 [{issue['linter']}]: {issue['text']}")
        if len(file_issues) > 10:
            print(f"  ... 还有 {len(file_issues) - 10} 个问题")
    else:
        print(f"\n✅ 问题列表中该文件没有问题")

except Exception as e:
    print(f"❌ 错误: {e}")
PYTHON_SCRIPT

elif [ -n "$VERIFY_LINTER" ]; then
    echo "验证检查器: $VERIFY_LINTER"
    ./bin/golangci-lint run --disable-all --enable "$VERIFY_LINTER" 2>&1 | head -50
    
    python3 << PYTHON_SCRIPT
import json

try:
    with open('.lint-report.json', 'r') as f:
        data = json.load(f)
    
    # 支持新旧格式
    if 'all_issues' in data:
        all_issues = data['all_issues']
    else:
        all_issues = []
        for file_path, issues in data.get('issues_by_file', {}).items():
            all_issues.extend(issues)
    
    linter_issues = [i for i in all_issues if i['linter'] == '$VERIFY_LINTER']
    
    print(f"\n📋 问题列表中该检查器有 {len(linter_issues)} 个问题")
    
    if linter_issues:
        # 按文件分组
        files = {}
        for issue in linter_issues:
            file_path = issue['file']
            if file_path not in files:
                files[file_path] = []
            files[file_path].append(issue)
        
        print(f"\n涉及 {len(files)} 个文件:")
        for file_path, issues in sorted(files.items(), key=lambda x: len(x[1]), reverse=True)[:10]:
            print(f"  {file_path}: {len(issues)} 个")
        if len(files) > 10:
            print(f"  ... 还有 {len(files) - 10} 个文件")

except Exception as e:
    print(f"❌ 错误: {e}")
PYTHON_SCRIPT
fi

echo ""
echo "💡 提示: 运行完整检查: ./scripts/lint/verify-fix.sh -a"

