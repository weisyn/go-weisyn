#!/bin/bash
# 增量更新报告（只检查修改的文件）
# 用途：文件修改后，只检查修改的文件，更新报告

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

REPORT_JSON="${1:-.lint-report.json}"

if [ ! -f "$REPORT_JSON" ]; then
    echo "❌ 报告文件不存在: $REPORT_JSON"
    echo "💡 请先运行: ./scripts/lint/check-and-report.sh"
    exit 1
fi

echo "🔄 增量更新报告（检查修改的文件）..."
echo ""

# 获取修改的文件
MODIFIED_FILES=$(git diff --name-only --diff-filter=ACM HEAD 2>/dev/null | grep '\.go$' || echo "")

if [ -z "$MODIFIED_FILES" ]; then
    echo "⚠️  没有检测到修改的 Go 文件"
    echo "💡 提示: 确保文件已 git add 或使用 git diff 检测修改"
    exit 0
fi

echo "📝 检测到修改的文件:"
echo "$MODIFIED_FILES" | while read -r file; do
    echo "  - $file"
done
echo ""

# 检查修改的文件
echo "🔍 检查修改的文件..."
./bin/golangci-lint run --out-format json $(echo "$MODIFIED_FILES" | tr '\n' ' ') > /tmp/lint-update.json 2>/dev/null || true

# 更新报告
python3 << 'PYTHON_SCRIPT'
import json
import os
from collections import defaultdict
from datetime import datetime

def extract_code_context(file_path, line_num, context_lines=3):
    """提取代码上下文"""
    try:
        if not os.path.exists(file_path):
            return None, None
        
        with open(file_path, 'r', encoding='utf-8') as f:
            lines = f.readlines()
        
        if line_num < 1 or line_num > len(lines):
            return None, None
        
        start_line = max(0, line_num - context_lines - 1)
        end_line = min(len(lines), line_num + context_lines)
        
        context_before = lines[start_line:line_num-1]
        problem_line = lines[line_num-1] if line_num <= len(lines) else ""
        context_after = lines[line_num:end_line]
        
        context = {
            'start_line': start_line + 1,
            'problem_line': line_num,
            'end_line': end_line,
            'before': [line.rstrip() for line in context_before],
            'line': problem_line.rstrip(),
            'after': [line.rstrip() for line in context_after]
        }
        
        code_snippet = problem_line.strip()[:100]
        return context, code_snippet
    except Exception:
        return None, None

def get_priority(linter):
    PRIORITY_HIGH = ['errcheck', 'gosec', 'bodyclose']
    PRIORITY_MEDIUM = ['revive', 'staticcheck', 'gocritic', 'govet', 'ineffassign']
    if linter in PRIORITY_HIGH:
        return 1, '高'
    elif linter in PRIORITY_MEDIUM:
        return 2, '中'
    else:
        return 3, '低'

try:
    # 读取现有报告
    with open('$REPORT_JSON', 'r') as f:
        report_data = json.load(f)
    
    # 读取新的检查结果
    with open('/tmp/lint-update.json', 'r') as f:
        new_data = json.load(f)
    
    # 获取修改的文件列表
    modified_files = set()
    for issue in new_data.get('Issues', []):
        file_path = issue.get('Pos', {}).get('Filename', '')
        if file_path:
            modified_files.add(file_path)
    
    print(f"   更新 {len(modified_files)} 个文件的问题...")
    
    # 从现有报告中移除这些文件的问题
    issues_by_file = defaultdict(list)
    for file_path, issues in report_data.get('issues_by_file', {}).items():
        if file_path not in modified_files:
            issues_by_file[file_path] = issues
    
    # 添加新检查的问题
    for issue in new_data.get('Issues', []):
        linter = issue.get('FromLinter', 'unknown')
        file_path = issue.get('Pos', {}).get('Filename', 'unknown')
        line = issue.get('Pos', {}).get('Line', 0)
        text = issue.get('Text', '')
        severity = issue.get('Severity', '')
        
        context, code_snippet = extract_code_context(file_path, line)
        
        issue_data = {
            'id': f"{file_path}:{line}:{linter}",
            'linter': linter,
            'file': file_path,
            'line': line,
            'text': text,
            'severity': severity,
            'code_context': context,
            'code_snippet': code_snippet,
            'priority': get_priority(linter)[0],
            'priority_name': get_priority(linter)[1]
        }
        
        issues_by_file[file_path].append(issue_data)
    
    # 重新统计
    all_issues = []
    for issues in issues_by_file.values():
        all_issues.extend(issues)
    
    linter_counts = defaultdict(int)
    for issue in all_issues:
        linter_counts[issue['linter']] += 1
    
    # 更新报告
    report_data['generated_at'] = datetime.now().isoformat()
    report_data['total_issues'] = len(all_issues)
    report_data['files_count'] = len(issues_by_file)
    report_data['linter_counts'] = dict(linter_counts)
    report_data['issues_by_file'] = {
        file_path: sorted(issues, key=lambda x: x['line'])
        for file_path, issues in issues_by_file.items()
    }
    report_data['all_issues'] = all_issues
    
    # 保存更新后的报告
    with open('$REPORT_JSON', 'w', encoding='utf-8') as f:
        json.dump(report_data, f, indent=2, ensure_ascii=False)
    
    print(f"✅ 报告已更新: $REPORT_JSON")
    print(f"   总问题数: {len(all_issues)} 个")
    print(f"   涉及文件: {len(issues_by_file)} 个")

except Exception as e:
    print(f"❌ 错误: {e}")
    import traceback
    traceback.print_exc()
    exit(1)
PYTHON_SCRIPT

echo ""
echo "💡 提示: 运行完整报告生成: ./scripts/lint/check-and-report.sh"

