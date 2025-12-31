#!/bin/bash
# 查询和过滤 lint 问题脚本
# 用途：从 .lint-issues.json 中查询、过滤和统计问题

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

JSON_FILE="${1:-.lint-report.json}"

if [ ! -f "$JSON_FILE" ]; then
    echo "❌ 报告文件不存在: $JSON_FILE"
    echo "💡 请先运行: make lint-check"
    exit 1
fi

# 显示帮助信息
show_help() {
    cat << EOF
用法: $0 [选项] [JSON文件]

查询和过滤 lint 问题

选项:
  -h, --help              显示帮助信息
  -l, --linter NAME       按检查器过滤（如: errcheck, gosec）
  -f, --file PATH         按文件过滤（支持部分匹配）
  -p, --priority LEVEL    按优先级过滤（high/medium/low）
  -t, --top N             显示前 N 个问题最多的文件（默认: 10）
  -s, --stats             显示统计信息
  -c, --count             只显示数量
  --format FORMAT         输出格式（table/json/markdown，默认: table）

示例:
  $0 -s                          # 显示统计信息
  $0 -l errcheck                 # 显示所有 errcheck 问题
  $0 -f internal/core            # 显示 internal/core 目录下的问题
  $0 -p high                     # 显示高优先级问题
  $0 -t 20                       # 显示问题最多的前20个文件
  $0 -l errcheck --format json   # JSON 格式输出

EOF
}

# 解析参数
LINTER=""
FILE_FILTER=""
PRIORITY=""
TOP_N=10
SHOW_STATS=false
SHOW_COUNT=false
FORMAT="table"

# 先检查是否有位置参数（JSON文件路径）
if [[ $# -gt 0 ]] && [[ ! "$1" =~ ^- ]]; then
    JSON_FILE="$1"
    shift
fi

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -l|--linter)
            LINTER="$2"
            shift 2
            ;;
        -f|--file)
            FILE_FILTER="$2"
            shift 2
            ;;
        -p|--priority)
            PRIORITY="$2"
            shift 2
            ;;
        -t|--top)
            TOP_N="$2"
            shift 2
            ;;
        -s|--stats)
            SHOW_STATS=true
            shift
            ;;
        -c|--count)
            SHOW_COUNT=true
            shift
            ;;
        --format)
            FORMAT="$2"
            shift 2
            ;;
        *)
            # 如果还有未处理的参数，可能是 JSON 文件路径
            if [ -z "$JSON_FILE" ] || [ "$JSON_FILE" = ".lint-report.json" ]; then
                JSON_FILE="$1"
            fi
            shift
            ;;
    esac
done

# Python 处理脚本
python3 << PYTHON_SCRIPT
import json
import sys
from collections import defaultdict

# 优先级定义
PRIORITY_HIGH = ['errcheck', 'gosec', 'bodyclose']
PRIORITY_MEDIUM = ['revive', 'staticcheck', 'gocritic', 'govet', 'ineffassign']
PRIORITY_LOW = ['unused', 'unparam', 'prealloc', 'misspell']

def get_priority(linter):
    if linter in PRIORITY_HIGH:
        return 1, 'high'
    elif linter in PRIORITY_MEDIUM:
        return 2, 'medium'
    else:
        return 3, 'low'

try:
    with open('$JSON_FILE', 'r') as f:
        data = json.load(f)
    
    # 支持新旧格式
    if 'all_issues' in data:
        all_issues = data['all_issues']
    else:
        # 兼容旧格式
        all_issues = []
        for file_path, issues in data.get('issues_by_file', {}).items():
            all_issues.extend(issues)
    
    # 过滤问题
    filtered_issues = []
    for issue in all_issues:
        # 按检查器过滤
        if '$LINTER' and issue['linter'] != '$LINTER':
            continue
        
        # 按文件过滤
        if '$FILE_FILTER' and '$FILE_FILTER' not in issue['file']:
            continue
        
        # 按优先级过滤
        if '$PRIORITY':
            priority, _ = get_priority(issue['linter'])
            priority_map = {'high': 1, 'medium': 2, 'low': 3}
            if priority != priority_map.get('$PRIORITY', 0):
                continue
        
        filtered_issues.append(issue)
    
    # 显示统计信息
    if $SHOW_STATS:
        print("📊 问题统计")
        print("=" * 60)
        print(f"总问题数: {data['total_issues']} 个")
        print(f"过滤后: {len(filtered_issues)} 个")
        print()
        print("按检查器统计:")
        linter_counts = defaultdict(int)
        for issue in filtered_issues:
            linter_counts[issue['linter']] += 1
        for linter, count in sorted(linter_counts.items(), key=lambda x: x[1], reverse=True):
            priority, priority_name = get_priority(linter)
            print(f"  {linter:15s} {count:4d} 个 (优先级: {priority_name})")
        sys.exit(0)
    
    # 只显示数量
    if $SHOW_COUNT:
        print(len(filtered_issues))
        sys.exit(0)
    
    # 显示前 N 个文件
    if not filtered_issues and not '$LINTER' and not '$FILE_FILTER' and not '$PRIORITY':
        print("📁 问题最多的前 $TOP_N 个文件:")
        print("=" * 60)
        file_counts = defaultdict(list)
        for issue in data['all_issues']:
            file_counts[issue['file']].append(issue)
        
        sorted_files = sorted(file_counts.items(), key=lambda x: len(x[1]), reverse=True)
        for i, (file_path, issues) in enumerate(sorted_files[:int('$TOP_N')], 1):
            print(f"{i:2d}. {len(issues):4d} 个 - {file_path}")
        sys.exit(0)
    
    # 显示过滤后的问题
    if '$FORMAT' == 'json':
        output = {
            'total': len(filtered_issues),
            'issues': filtered_issues
        }
        print(json.dumps(output, indent=2, ensure_ascii=False))
    elif '$FORMAT' == 'markdown':
        print("# 过滤后的问题列表")
        print()
        print(f"**总数**: {len(filtered_issues)} 个")
        print()
        # 按文件分组
        files_issues = defaultdict(list)
        for issue in filtered_issues:
            files_issues[issue['file']].append(issue)
        
        for file_path in sorted(files_issues.keys()):
            issues = files_issues[file_path]
            print(f"## {file_path} ({len(issues)} 个)")
            print()
            for issue in sorted(issues, key=lambda x: x['line']):
                print(f"- 第 {issue['line']} 行 [{issue['linter']}]: {issue['text']}")
            print()
    else:  # table format
        if filtered_issues:
            print(f"📋 找到 {len(filtered_issues)} 个问题")
            print("=" * 100)
            print(f"{'文件':<50} {'行':<6} {'检查器':<15} {'问题描述'}")
            print("=" * 100)
            for issue in sorted(filtered_issues, key=lambda x: (x['file'], x['line'])):
                file_path = issue['file']
                if len(file_path) > 47:
                    file_path = "..." + file_path[-44:]
                print(f"{file_path:<50} {issue['line']:<6} {issue['linter']:<15} {issue['text']}")
        else:
            print("✅ 没有找到匹配的问题")

except Exception as e:
    print(f"❌ 错误: {e}", file=sys.stderr)
    sys.exit(1)
PYTHON_SCRIPT

