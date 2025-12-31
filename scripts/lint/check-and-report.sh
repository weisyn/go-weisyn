#!/bin/bash
# 检查并生成修复友好的报告
# 架构：检查（一次性）-> 报告生成（包含代码上下文）-> 修复
# 目的：为修复提供精确定位，即使行号变化也能找到问题

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

# 配置
RAW_OUTPUT="${1:-.lint-raw.json}"      # 原始检查结果（一次性生成）
REPORT_JSON="${2:-.lint-report.json}"  # 修复友好的报告（JSON）
REPORT_MD="${3:-.lint-report.md}"     # 修复友好的报告（Markdown）

echo "🔍 代码质量检查与报告生成"
echo "=========================================="
echo "原始输出: $RAW_OUTPUT"
echo "报告 JSON: $REPORT_JSON"
echo "报告 Markdown: $REPORT_MD"
echo ""

# 检查工具
if [ ! -f "./bin/golangci-lint" ]; then
    echo "❌ golangci-lint 未找到，请先运行: make install-lint-tools"
    exit 1
fi

# 步骤1: 运行检查（一次性）
echo "📊 步骤1: 运行 golangci-lint 检查（这可能需要几分钟）..."
if [ -f "$RAW_OUTPUT" ] && [ "$RAW_OUTPUT" != ".lint-raw.json" ]; then
    echo "⚠️  使用已存在的检查结果: $RAW_OUTPUT"
else
    # 尝试不同的检查方式（处理 go.work 问题）
    echo "   正在检查代码..."
    
    # 方法1: 尝试直接运行（可能因为 go.work 失败）
    ./bin/golangci-lint run --output.json.path "$RAW_OUTPUT" 2>/tmp/lint-errors.txt || true
    
    # 检查是否有问题
    issue_count=$(cat "$RAW_OUTPUT" 2>/dev/null | python3 -c "import json, sys; d=json.load(sys.stdin); print(len(d.get('Issues', [])))" 2>/dev/null || echo "0")
    
    # 如果失败或问题数为0，使用文件列表方式（解决 go.work 问题）
    if [ ! -s "$RAW_OUTPUT" ] || [ "$issue_count" = "0" ]; then
        echo "   使用文件列表方式检查（解决 go.work 问题）..."
        
        # 查找所有需要检查的 Go 文件
        echo '{"Issues":[]}' > "$RAW_OUTPUT"
        
        # 查找所有 .go 文件（排除测试文件和生成文件）
        go_files=$(find . \
            -name "*.go" \
            -not -path "./_archived/*" \
            -not -path "./vendor/*" \
            -not -path "./_docs/*" \
            -not -path "./_sdks/*" \
            -not -path "./docs.backup.*/*" \
            -not -path "./data/*" \
            -not -path "./bin/*" \
            -not -path "./config-temp/*" \
            -not -name "*_test.go" \
            -not -name "*.pb.go" \
            2>/dev/null | head -1000)
        
        file_count=$(echo "$go_files" | grep -c . || echo "0")
        echo "   找到 $file_count 个 Go 文件需要检查"
        
        if [ "$file_count" -gt 0 ]; then
            # 使用 Python 脚本进行批量检查和合并（更可靠）
            echo "   开始批量检查 $file_count 个文件..."
            
            python3 << BATCH_CHECK_SCRIPT
import json
import subprocess
import os
import sys
from pathlib import Path

raw_output = "$RAW_OUTPUT"
go_files_list = """$go_files"""

# 解析文件列表
files = [f.strip() for f in go_files_list.split('\n') if f.strip()]

all_issues = []
total_files = len(files)
processed = 0
batch_size = 50  # 每50个文件显示一次进度

print(f"   总共需要检查 {total_files} 个文件")

for idx, file_path in enumerate(files, 1):
    if not os.path.exists(file_path):
        continue
    
    try:
        # 检查单个文件
        result = subprocess.run(
            ['./bin/golangci-lint', 'run', file_path, '--output.json.path', '/tmp/lint-single.json'],
            capture_output=True,
            timeout=30,
            cwd='.'
        )
        
        # 读取结果
        if os.path.exists('/tmp/lint-single.json'):
            with open('/tmp/lint-single.json', 'r') as f:
                try:
                    data = json.load(f)
                    issues = data.get('Issues', [])
                    all_issues.extend(issues)
                    processed += 1
                except json.JSONDecodeError:
                    pass
            os.remove('/tmp/lint-single.json')
        
        # 显示进度
        if idx % batch_size == 0 or idx == total_files:
            print(f"   进度: {idx}/{total_files} ({idx*100//total_files}%) - 已收集 {len(all_issues)} 个问题")
    
    except subprocess.TimeoutExpired:
        print(f"   ⚠️  文件 {file_path} 检查超时，跳过")
    except Exception as e:
        pass  # 静默忽略错误

# 保存结果
result_data = {"Issues": all_issues}
with open(raw_output, 'w') as f:
    json.dump(result_data, f)

print(f"   ✅ 检查完成！共检查 {processed} 个文件，发现 {len(all_issues)} 个问题")
BATCH_CHECK_SCRIPT
        else
            echo "   ⚠️  未找到需要检查的 Go 文件"
        fi
    fi
    
    if [ -s /tmp/lint-errors.txt ] || [ -s /tmp/lint-errors2.txt ]; then
        echo "⚠️  检查过程中有错误输出，但继续处理..."
        cat /tmp/lint-errors.txt /tmp/lint-errors2.txt 2>/dev/null | head -20
    fi
    
    echo "✅ 检查完成，结果已保存到: $RAW_OUTPUT"
fi

# 步骤2: 生成修复友好的报告（包含代码上下文）
echo ""
echo "📝 步骤2: 生成修复友好的报告（提取代码上下文）..."

# 将变量写入临时文件供 Python 使用
echo "$RAW_OUTPUT" > /tmp/lint-raw-output-path.txt
echo "$REPORT_JSON" > /tmp/lint-report-json-path.txt
echo "$REPORT_MD" > /tmp/lint-report-md-path.txt

python3 << 'PYTHON_SCRIPT'
import json
import os
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

def extract_code_context(file_path, line_num, context_lines=3):
    """提取代码上下文（问题行前后几行）"""
    try:
        if not os.path.exists(file_path):
            return None, None
        
        with open(file_path, 'r', encoding='utf-8') as f:
            lines = f.readlines()
        
        if line_num < 1 or line_num > len(lines):
            return None, None
        
        # 提取上下文（前后各 context_lines 行）
        start_line = max(0, line_num - context_lines - 1)
        end_line = min(len(lines), line_num + context_lines)
        
        context_before = lines[start_line:line_num-1]
        problem_line = lines[line_num-1] if line_num <= len(lines) else ""
        context_after = lines[line_num:end_line]
        
        # 构建上下文
        context = {
            'start_line': start_line + 1,
            'problem_line': line_num,
            'end_line': end_line,
            'before': [line.rstrip() for line in context_before],
            'line': problem_line.rstrip(),
            'after': [line.rstrip() for line in context_after]
        }
        
        # 生成代码片段（用于定位）
        code_snippet = problem_line.strip()[:100]  # 限制长度
        
        return context, code_snippet
    except Exception as e:
        return None, None

try:
    # 读取文件路径
    with open('/tmp/lint-raw-output-path.txt', 'r') as f:
        raw_output_file = f.read().strip()
    with open('/tmp/lint-report-json-path.txt', 'r') as f:
        report_json_file = f.read().strip()
    with open('/tmp/lint-report-md-path.txt', 'r') as f:
        report_md_file = f.read().strip()
    
    # 读取原始检查结果
    with open(raw_output_file, 'r') as f:
        raw_data = json.load(f)
    
    issues = raw_data.get('Issues', [])
    total = len(issues)
    
    print(f"   找到 {total} 个问题")
    print(f"   正在提取代码上下文...")
    
    # 处理每个问题，提取代码上下文
    processed_issues = []
    issues_by_file = defaultdict(list)
    
    for idx, issue in enumerate(issues):
        if (idx + 1) % 100 == 0:
            print(f"   处理进度: {idx + 1}/{total}")
        
        linter = issue.get('FromLinter', 'unknown')
        file_path = issue.get('Pos', {}).get('Filename', 'unknown')
        line = issue.get('Pos', {}).get('Line', 0)
        text = issue.get('Text', '')
        severity = issue.get('Severity', '')
        
        # 提取代码上下文
        context, code_snippet = extract_code_context(file_path, line)
        
        issue_data = {
            'id': f"{file_path}:{line}:{linter}",  # 唯一标识
            'linter': linter,
            'file': file_path,
            'line': line,
            'text': text,
            'severity': severity,
            'code_context': context,  # 代码上下文
            'code_snippet': code_snippet,  # 代码片段（用于定位）
            'priority': get_priority(linter)[0],
            'priority_name': get_priority(linter)[1]
        }
        
        processed_issues.append(issue_data)
        issues_by_file[file_path].append(issue_data)
    
    # 统计信息
    linter_counts = defaultdict(int)
    for issue in processed_issues:
        linter_counts[issue['linter']] += 1
    
    # 生成报告数据
    report_data = {
        'generated_at': datetime.now().isoformat(),
        'raw_file': raw_output_file,
        'total_issues': total,
        'files_count': len(issues_by_file),
        'linter_counts': dict(linter_counts),
        'issues_by_file': {
            file_path: sorted(issues, key=lambda x: x['line'])
            for file_path, issues in issues_by_file.items()
        },
        'all_issues': processed_issues
    }
    
    # 保存 JSON 报告
    with open(report_json_file, 'w', encoding='utf-8') as f:
        json.dump(report_data, f, indent=2, ensure_ascii=False)
    
    print(f"✅ JSON 报告已生成: {report_json_file}")
    
    # 生成 Markdown 报告
    md_lines = []
    md_lines.append("# 代码质量检查报告（修复友好版）")
    md_lines.append("")
    md_lines.append(f"**生成时间**: {report_data['generated_at']}")
    md_lines.append(f"**总问题数**: {total} 个")
    md_lines.append(f"**涉及文件**: {len(issues_by_file)} 个")
    md_lines.append("")
    md_lines.append("---")
    md_lines.append("")
    md_lines.append("## 📊 问题统计")
    md_lines.append("")
    md_lines.append("### 按检查器统计")
    md_lines.append("")
    md_lines.append("| 检查器 | 问题数 | 优先级 |")
    md_lines.append("|--------|--------|--------|")
    for linter, count in sorted(linter_counts.items(), key=lambda x: x[1], reverse=True):
        _, priority_name = get_priority(linter)
        md_lines.append(f"| {linter} | {count} | {priority_name} |")
    
    md_lines.append("")
    md_lines.append("### 按优先级统计")
    md_lines.append("")
    priority_counts = defaultdict(int)
    for issue in processed_issues:
        priority_counts[issue['priority_name']] += 1
    
    # 可视化进度条
    for priority in ['高', '中', '低']:
        count = priority_counts[priority]
        percentage = (count / total * 100) if total > 0 else 0
        # 生成进度条（每2%一个字符，最多50个字符）
        bar_length = int(percentage / 2) if percentage > 0 else 0
        bar = "█" * bar_length + "░" * (50 - bar_length)
        md_lines.append(f"- **{priority}优先级**: {count:4d} 个 ({percentage:5.1f}%) {bar}")
    
    md_lines.append("")
    md_lines.append("### 按检查器统计（前10个）")
    md_lines.append("")
    md_lines.append("| 排名 | 检查器 | 问题数 | 百分比 | 优先级 | 进度条 |")
    md_lines.append("|------|--------|--------|--------|--------|--------|")
    
    # 按数量排序
    sorted_linters = sorted(linter_counts.items(), key=lambda x: x[1], reverse=True)
    for rank, (linter, count) in enumerate(sorted_linters[:10], 1):
        _, priority_name = get_priority(linter)
        percentage = (count / total * 100) if total > 0 else 0
        bar_length = int(percentage / 2) if percentage > 0 else 0
        bar = "█" * min(bar_length, 50) + "░" * max(0, 50 - bar_length)
        md_lines.append(f"| {rank} | {linter} | {count:4d} | {percentage:5.1f}% | {priority_name} | {bar} |")
    
    md_lines.append("")
    md_lines.append("---")
    md_lines.append("")
    md_lines.append("## 📋 问题详情（按文件分组）")
    md_lines.append("")
    md_lines.append("> 💡 **提示**: 每个问题都包含代码上下文，即使行号变化也能准确定位")
    md_lines.append("")
    
    # 添加总体进度可视化
    if total > 0:
        md_lines.append("### 📈 修复进度概览")
        md_lines.append("")
        md_lines.append("```")
        md_lines.append(f"总问题数: {total}")
        md_lines.append("")
        for priority in ['高', '中', '低']:
            count = priority_counts[priority]
            percentage = (count / total * 100) if total > 0 else 0
            bar_length = int(percentage / 2)
            bar = "█" * bar_length + "░" * (50 - bar_length)
            md_lines.append(f"{priority:2s}优先级: {bar} {count:4d} ({percentage:5.1f}%)")
        md_lines.append("```")
        md_lines.append("")
    
    # 按优先级和文件分组
    for priority_level, priority_name in [(1, '高优先级'), (2, '中优先级'), (3, '低优先级')]:
        priority_files = []
        for file_path, file_issues in issues_by_file.items():
            has_priority_issue = any(i['priority'] == priority_level for i in file_issues)
            if has_priority_issue:
                priority_files.append((file_path, file_issues))
        
        if priority_files:
            md_lines.append(f"### {priority_name}问题")
            md_lines.append("")
            
            # 按问题数量排序
            priority_files.sort(key=lambda x: len(x[1]), reverse=True)
            
            for file_path, file_issues in priority_files:
                # 统计该文件的问题
                file_total = len(file_issues)
                priority_issues = [i for i in file_issues if i['priority'] == priority_level]
                
                md_lines.append(f"#### 📄 `{file_path}` ({len(priority_issues)} 个{priority_name}问题，共 {file_total} 个)")
                md_lines.append("")
                
                # 按行号排序
                for issue in sorted(priority_issues, key=lambda x: x['line']):
                    md_lines.append(f"**问题 #{issue['line']}** [{issue['linter']}]")
                    md_lines.append("")
                    md_lines.append(f"> {issue['text']}")
                    md_lines.append("")
                    
                    # 显示代码上下文
                    if issue['code_context']:
                        ctx = issue['code_context']
                        md_lines.append("```go")
                        # 显示上下文
                        for i, line in enumerate(ctx['before'], start=ctx['start_line']):
                            md_lines.append(f"{i:4d} | {line}")
                        # 问题行（标记）
                        md_lines.append(f"{ctx['problem_line']:4d} | {ctx['line']}  // ⚠️ 问题位置")
                        for i, line in enumerate(ctx['after'], start=ctx['problem_line'] + 1):
                            md_lines.append(f"{i:4d} | {line}")
                        md_lines.append("```")
                        md_lines.append("")
                    
                    md_lines.append("---")
                    md_lines.append("")
            
            md_lines.append("")
    
    md_lines.append("## ✅ 修复建议")
    md_lines.append("")
    md_lines.append("1. **按文件修复**: 优先修复问题数量多的文件，一次修复文件中的所有问题")
    md_lines.append("2. **按优先级修复**: 先修复高优先级问题（errcheck, gosec, bodyclose）")
    md_lines.append("3. **使用代码上下文**: 即使行号变化，也能通过代码片段准确定位问题")
    md_lines.append("4. **验证修复**: 修复后运行 `make lint-verify FILE=path/to/file.go` 验证")
    md_lines.append("")
    
    with open(report_md_file, 'w', encoding='utf-8') as f:
        f.write('\n'.join(md_lines))
    
    print(f"✅ Markdown 报告已生成: {report_md_file}")
    
    # 显示文件统计（带可视化）
    print("")
    print("📁 问题最多的前10个文件:")
    print("=" * 80)
    file_counts = [(file_path, len(issues)) for file_path, issues in issues_by_file.items()]
    file_counts.sort(key=lambda x: x[1], reverse=True)
    
    if file_counts:
        max_count = file_counts[0][1]
        for rank, (file_path, count) in enumerate(file_counts[:10], 1):
            # 生成进度条
            bar_length = int((count / max_count) * 50) if max_count > 0 else 0
            bar = "█" * bar_length + "░" * (50 - bar_length)
            percentage = (count / total * 100) if total > 0 else 0
            print(f"  {rank:2d}. {count:4d} 个 ({percentage:5.1f}%) {bar} {file_path}")
    else:
        print("  (无)")
    print("=" * 80)
    
    # 显示检查器统计（可视化）
    print("")
    print("🔍 按检查器统计（前10个）:")
    print("=" * 80)
    sorted_linters = sorted(linter_counts.items(), key=lambda x: x[1], reverse=True)
    max_linter_count = sorted_linters[0][1] if sorted_linters else 1
    
    for rank, (linter, count) in enumerate(sorted_linters[:10], 1):
        _, priority_name = get_priority(linter)
        percentage = (count / total * 100) if total > 0 else 0
        bar_length = int((count / max_linter_count) * 50) if max_linter_count > 0 else 0
        bar = "█" * bar_length + "░" * (50 - bar_length)
        print(f"  {rank:2d}. {linter:15s} {count:4d} 个 ({percentage:5.1f}%) [{priority_name:2s}] {bar}")
    print("=" * 80)

except Exception as e:
    print(f"❌ 错误: {e}")
    import traceback
    traceback.print_exc()
    exit(1)
PYTHON_SCRIPT

echo ""
echo "✅ 完成！"
echo ""
echo "📝 报告文件:"
echo "   - JSON 格式: $REPORT_JSON"
echo "   - Markdown 格式: $REPORT_MD"
echo ""
echo "💡 使用建议:"
echo "   1. 查看 Markdown 报告: cat $REPORT_MD"
echo "   2. 按文件修复问题（报告已按文件分组）"
echo "   3. 使用代码上下文精确定位问题（即使行号变化）"

