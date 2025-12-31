#!/bin/bash
# 提取 golangci-lint 问题列表脚本（完整版）
# 用途：基于 lint 输出生成待修复问题列表，提高修复效率
# 支持所有12个检查器，生成 JSON 和 Markdown 两种格式

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

# 配置
JSON_OUTPUT="${1:-.lint-issues.json}"
MD_OUTPUT="${2:-.lint-issues-pending.md}"
ALL_LINTERS="errcheck,govet,ineffassign,staticcheck,unused,gocritic,gosec,misspell,unparam,revive,prealloc,bodyclose"

echo "🔍 提取 golangci-lint 问题列表..."
echo "JSON 输出: $JSON_OUTPUT"
echo "Markdown 输出: $MD_OUTPUT"
echo "检查器: 所有12个检查器"

# 检查工具
if [ ! -f "./bin/golangci-lint" ]; then
    echo "❌ golangci-lint 未找到，请先运行: make install-lint-tools"
    exit 1
fi

# 检查 jq 是否安装（用于处理 JSON）
if ! command -v jq >/dev/null 2>&1; then
    echo "⚠️  jq 未安装，将使用 Python 处理 JSON"
    USE_JQ=false
else
    USE_JQ=true
fi

echo "📊 运行 golangci-lint 检查（这可能需要几分钟）..."

# 运行检查并生成 JSON 输出
./bin/golangci-lint run --out-format json > /tmp/lint-output.json 2>/tmp/lint-errors.txt || true

# 检查是否有错误
if [ -s /tmp/lint-errors.txt ]; then
    echo "⚠️  检查过程中有错误输出，但继续处理..."
    cat /tmp/lint-errors.txt | head -20
fi

# 使用 Python 处理 JSON（更可靠，不依赖 jq）
python3 << 'PYTHON_SCRIPT' > /tmp/lint-stats.txt
import json
import sys
from collections import defaultdict

try:
    with open('/tmp/lint-output.json', 'r') as f:
        data = json.load(f)
    
    # 统计每个检查器的问题数量
    linter_counts = defaultdict(int)
    issues_by_file = defaultdict(lambda: defaultdict(list))
    all_issues = []
    
    for issue in data.get('Issues', []):
        linter = issue.get('FromLinter', 'unknown')
        file_path = issue.get('Pos', {}).get('Filename', 'unknown')
        line = issue.get('Pos', {}).get('Line', 0)
        text = issue.get('Text', '')
        severity = issue.get('Severity', '')
        
        linter_counts[linter] += 1
        
        issue_data = {
            'linter': linter,
            'file': file_path,
            'line': line,
            'text': text,
            'severity': severity
        }
        
        issues_by_file[file_path][linter].append(issue_data)
        all_issues.append(issue_data)
    
    # 输出统计信息
    total = sum(linter_counts.values())
    print(f"TOTAL={total}")
    for linter, count in sorted(linter_counts.items()):
        print(f"{linter.upper()}={count}")
    
    # 保存完整数据到临时文件
    output_data = {
        'generated_at': __import__('datetime').datetime.now().isoformat(),
        'total_issues': total,
        'linter_counts': dict(linter_counts),
        'issues_by_file': {k: dict(v) for k, v in issues_by_file.items()},
        'all_issues': all_issues
    }
    
    with open('/tmp/lint-processed.json', 'w') as f:
        json.dump(output_data, f, indent=2, ensure_ascii=False)
    
except Exception as e:
    print(f"ERROR={str(e)}", file=sys.stderr)
    sys.exit(1)
PYTHON_SCRIPT

if [ $? -ne 0 ]; then
    echo "❌ 处理 JSON 输出失败"
    exit 1
fi

# 读取统计信息
TOTAL_COUNT=0
declare -A LINTER_COUNTS

while IFS='=' read -r key value; do
    if [ "$key" = "TOTAL" ]; then
        TOTAL_COUNT=$value
    else
        LINTER_COUNTS["$key"]=$value
    fi
done < /tmp/lint-stats.txt

echo ""
echo "📊 问题统计:"
echo "  总计: $TOTAL_COUNT 个问题"
for linter in errcheck govet ineffassign staticcheck unused gocritic gosec misspell unparam revive prealloc bodyclose; do
    count=${LINTER_COUNTS[${linter^^}]:-0}
    if [ "$count" -gt 0 ]; then
        printf "  %-15s: %4d 个\n" "$linter" "$count"
    fi
done

# 复制处理后的 JSON 到输出文件
cp /tmp/lint-processed.json "$JSON_OUTPUT"
echo "✅ JSON 格式问题列表已生成: $JSON_OUTPUT"

# 生成 Markdown 格式（按文件分组）
python3 << 'PYTHON_SCRIPT'
import json
from collections import defaultdict

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

with open('/tmp/lint-processed.json', 'r') as f:
    data = json.load(f)

# 按文件分组
files_issues = defaultdict(lambda: defaultdict(list))
for issue in data['all_issues']:
    file_path = issue['file']
    linter = issue['linter']
    files_issues[file_path][linter].append(issue)

# 按优先级和问题数量排序文件
def file_sort_key(item):
    file_path, linters = item
    # 计算优先级分数（优先级1的文件排在前面）
    priority_score = 0
    total_issues = 0
    for linter, issues in linters.items():
        priority, _ = get_priority(linter)
        priority_score += priority * len(issues)
        total_issues += len(issues)
    return (priority_score, -total_issues)  # 负数用于降序

sorted_files = sorted(files_issues.items(), key=file_sort_key)

# 生成 Markdown
md_lines = []
md_lines.append("# 待修复问题列表（按文件分组）")
md_lines.append("")
md_lines.append(f"**生成日期**: {data['generated_at']}")
md_lines.append(f"**总问题数**: {data['total_issues']} 个")
md_lines.append(f"**文件数**: {len(files_issues)} 个")
md_lines.append("")
md_lines.append("---")
md_lines.append("")
md_lines.append("## 📊 问题统计（按检查器）")
md_lines.append("")
md_lines.append("| 检查器 | 问题数 | 优先级 |")
md_lines.append("|--------|--------|--------|")

for linter in ['errcheck', 'gosec', 'bodyclose', 'revive', 'staticcheck', 'gocritic', 'govet', 'ineffassign', 'unused', 'unparam', 'prealloc', 'misspell']:
    count = data['linter_counts'].get(linter, 0)
    if count > 0:
        _, priority_name = get_priority(linter)
        md_lines.append(f"| {linter} | {count} | {priority_name} |")

md_lines.append("")
md_lines.append("---")
md_lines.append("")
md_lines.append("## 🔧 使用说明")
md_lines.append("")
md_lines.append("1. **按文件分组修复**: 优先修复同一文件中的多个问题")
md_lines.append("2. **按优先级修复**: 高优先级（errcheck, gosec, bodyclose）→ 中优先级 → 低优先级")
md_lines.append("3. **查看问题详情**: 点击文件路径跳转到具体位置")
md_lines.append("4. **标记完成**: 修复后在对应项前添加 `[x]`")
md_lines.append("")
md_lines.append("---")
md_lines.append("")
md_lines.append("## 📋 待修复问题列表（按文件分组）")
md_lines.append("")

# 按优先级分组输出
for priority_level, priority_name in [(1, '高优先级'), (2, '中优先级'), (3, '低优先级')]:
    priority_files = []
    for file_path, linters in sorted_files:
        has_priority_issue = False
        for linter, issues in linters.items():
            p, _ = get_priority(linter)
            if p == priority_level:
                has_priority_issue = True
                break
        if has_priority_issue:
            priority_files.append((file_path, linters))
    
    if priority_files:
        md_lines.append(f"### {priority_name}问题")
        md_lines.append("")
        
        for file_path, linters in priority_files:
            # 统计该文件的问题
            file_total = sum(len(issues) for issues in linters.values())
            md_lines.append(f"#### `{file_path}` ({file_total} 个问题)")
            md_lines.append("")
            
            # 按检查器分组
            for linter in sorted(linters.keys(), key=lambda x: (get_priority(x)[0], x)):
                issues = linters[linter]
                priority, _ = get_priority(linter)
                if priority == priority_level:
                    md_lines.append(f"**{linter}** ({len(issues)} 个):")
                    md_lines.append("")
                    for issue in sorted(issues, key=lambda x: x['line']):
                        md_lines.append(f"- [ ] 第 {issue['line']} 行: {issue['text']}")
                    md_lines.append("")
        
        md_lines.append("---")
        md_lines.append("")

md_lines.append("## ✅ 修复进度")
md_lines.append("")
md_lines.append(f"- **总问题数**: {data['total_issues']} 个")
md_lines.append("- **已修复**: 0 个")
md_lines.append("- **进度**: 0%")
md_lines.append("")
md_lines.append("---")
md_lines.append("")
md_lines.append("**提示**: 修复问题时，请：")
md_lines.append("1. 修复后在此文件中标记为 `- [x]`")
md_lines.append("2. 更新进度统计")
md_lines.append("3. 运行 `make lint` 验证修复")
md_lines.append("4. 定期运行此脚本更新问题列表")

with open('/tmp/lint-issues.md', 'w', encoding='utf-8') as f:
    f.write('\n'.join(md_lines))
PYTHON_SCRIPT

if [ $? -ne 0 ]; then
    echo "❌ 生成 Markdown 失败"
    exit 1
fi

cp /tmp/lint-issues.md "$MD_OUTPUT"
echo "✅ Markdown 格式问题列表已生成: $MD_OUTPUT"

# 显示文件统计
echo ""
echo "📁 文件统计（问题最多的前10个文件）:"
python3 << 'PYTHON_SCRIPT'
import json
from collections import defaultdict

with open('/tmp/lint-processed.json', 'r') as f:
    data = json.load(f)

file_counts = defaultdict(int)
for issue in data['all_issues']:
    file_counts[issue['file']] += 1

sorted_files = sorted(file_counts.items(), key=lambda x: x[1], reverse=True)
for file_path, count in sorted_files[:10]:
    print(f"  {count:4d} 个 - {file_path}")
PYTHON_SCRIPT

echo ""
echo "✅ 完成！"
echo "📝 查看问题列表:"
echo "   - JSON 格式: $JSON_OUTPUT"
echo "   - Markdown 格式: $MD_OUTPUT"
echo ""
echo "💡 提示: 使用以下命令查看问题最多的文件:"
echo "   python3 -c \"import json; data=json.load(open('$JSON_OUTPUT')); files={}; [files.setdefault(i['file'], []).append(i) for i in data['all_issues']]; sorted_files=sorted(files.items(), key=lambda x: len(x[1]), reverse=True); [print(f'{len(issues):4d} - {file}') for file, issues in sorted_files[:10]]\""
