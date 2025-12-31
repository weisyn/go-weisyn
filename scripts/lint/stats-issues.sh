#!/bin/bash
# 统计 lint 问题脚本
# 用途：生成详细的问题统计报告

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

python3 << 'PYTHON_SCRIPT'
import json
from collections import defaultdict
from datetime import datetime

# 优先级定义
PRIORITY_HIGH = ['errcheck', 'gosec', 'bodyclose']
PRIORITY_MEDIUM = ['revive', 'staticcheck', 'gocritic', 'govet', 'ineffassign']
PRIORITY_LOW = ['unused', 'unparam', 'prealloc', 'misspell']

def get_priority(linter):
    if linter in PRIORITY_HIGH:
        return 1, '高'
    elif linter in PRIORITY_MEDIUM:
        return 2, '中'
    else:
        return 3, '低'

try:
    with open('$JSON_FILE', 'r') as f:
        data = json.load(f)
    
    # 支持新旧格式
    if 'all_issues' in data:
        issues = data['all_issues']
    else:
        # 兼容旧格式
        issues = []
        for file_path, file_issues in data.get('issues_by_file', {}).items():
            issues.extend(file_issues)
    total = len(issues)
    
    print("=" * 80)
    print("📊 Lint 问题统计报告")
    print("=" * 80)
    print(f"生成时间: {data.get('generated_at', '未知')}")
    print(f"总问题数: {total} 个")
    print()
    
    # 按优先级统计
    print("📈 按优先级统计:")
    print("-" * 80)
    priority_counts = defaultdict(int)
    for issue in issues:
        _, priority_name = get_priority(issue['linter'])
        priority_counts[priority_name] += 1
    
    for priority in ['高', '中', '低']:
        count = priority_counts[priority]
        percentage = (count / total * 100) if total > 0 else 0
        bar = "█" * int(percentage / 2)
        print(f"{priority:4s}优先级: {count:4d} 个 ({percentage:5.1f}%) {bar}")
    print()
    
    # 按检查器统计
    print("🔍 按检查器统计:")
    print("-" * 80)
    linter_counts = defaultdict(int)
    for issue in issues:
        linter_counts[issue['linter']] += 1
    
    # 按优先级和数量排序
    linter_list = []
    for linter, count in linter_counts.items():
        priority, priority_name = get_priority(linter)
        linter_list.append((priority, count, linter, priority_name))
    
    linter_list.sort(key=lambda x: (x[0], -x[1]))
    
    print(f"{'检查器':<20} {'数量':<8} {'百分比':<10} {'优先级':<8} {'进度条'}")
    print("-" * 80)
    for _, count, linter, priority_name in linter_list:
        percentage = (count / total * 100) if total > 0 else 0
        bar = "█" * int(percentage / 2)
        print(f"{linter:<20} {count:<8} {percentage:>6.1f}%   {priority_name:<8} {bar}")
    print()
    
    # 按文件统计
    print("📁 问题最多的前20个文件:")
    print("-" * 80)
    file_counts = defaultdict(list)
    for issue in issues:
        file_counts[issue['file']].append(issue)
    
    sorted_files = sorted(file_counts.items(), key=lambda x: len(x[1]), reverse=True)
    print(f"{'排名':<6} {'问题数':<8} {'文件路径'}")
    print("-" * 80)
    for i, (file_path, file_issues) in enumerate(sorted_files[:20], 1):
        # 按检查器分组
        linters = defaultdict(int)
        for issue in file_issues:
            linters[issue['linter']] += 1
        linter_summary = ", ".join([f"{k}({v})" for k, v in sorted(linters.items(), key=lambda x: x[1], reverse=True)[:3]])
        if len(linters) > 3:
            linter_summary += "..."
        
        print(f"{i:<6} {len(file_issues):<8} {file_path}")
        print(f"{'':6} {'':8} └─ {linter_summary}")
    print()
    
    # 按目录统计
    print("📂 按目录统计（前15个）:")
    print("-" * 80)
    dir_counts = defaultdict(int)
    for issue in issues:
        # 提取目录（去掉文件名）
        dir_path = "/".join(issue['file'].split("/")[:-1])
        if not dir_path:
            dir_path = "."
        dir_counts[dir_path] += 1
    
    sorted_dirs = sorted(dir_counts.items(), key=lambda x: x[1], reverse=True)
    print(f"{'问题数':<8} {'目录路径'}")
    print("-" * 80)
    for dir_path, count in sorted_dirs[:15]:
        percentage = (count / total * 100) if total > 0 else 0
        print(f"{count:<8} {dir_path} ({percentage:.1f}%)")
    print()
    
    # 修复建议
    print("💡 修复建议:")
    print("-" * 80)
    high_count = priority_counts['高']
    if high_count > 0:
        print(f"1. 优先修复高优先级问题 ({high_count} 个): errcheck, gosec, bodyclose")
    
    # 找出问题最多的文件
    if sorted_files:
        top_file, top_issues = sorted_files[0]
        print(f"2. 优先修复问题最多的文件: {top_file} ({len(top_issues)} 个问题)")
    
    # 找出问题最多的目录
    if sorted_dirs:
        top_dir, top_count = sorted_dirs[0]
        print(f"3. 优先修复问题最多的目录: {top_dir} ({top_count} 个问题)")
    
    print()
    print("=" * 80)

except Exception as e:
    print(f"❌ 错误: {e}")
    import traceback
    traceback.print_exc()
    exit(1)
PYTHON_SCRIPT

