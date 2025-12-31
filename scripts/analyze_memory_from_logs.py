#!/usr/bin/env python3
"""
内存日志分析工具

从节点日志中提取 memory_sample 记录，分析内存趋势，并生成报告。

使用方法：
    python3 scripts/analyze_memory_from_logs.py \
        --log ./data/memory-test/public-sync/logs/node-system.log \
        --output ./memory-report.csv

    python3 scripts/analyze_memory_from_logs.py \
        --log ./data/memory-test/public-sync/logs/node-system.log \
        --output ./memory-report.csv \
        --plot memory-trend.png
"""

import argparse
import json
import re
import sys
from datetime import datetime
from pathlib import Path
from typing import List, Dict, Optional

try:
    import matplotlib.pyplot as plt
    HAS_MATPLOTLIB = True
except ImportError:
    HAS_MATPLOTLIB = False
    print("警告: matplotlib 未安装，无法生成图表。使用 --no-plot 跳过图表生成。")


class MemorySample:
    """内存采样数据"""
    def __init__(self, data: Dict):
        self.time = datetime.fromisoformat(data['time'].replace('Z', '+00:00'))
        self.rss_mb = data.get('rss_mb', 0)
        self.rss_bytes = data.get('rss_bytes', 0)
        self.heap_mb = data.get('heap_mb', 0)
        self.heap_alloc_bytes = data.get('heap_alloc_bytes', 0)
        self.heap_inuse_bytes = data.get('heap_inuse_bytes', 0)
        self.gc = data.get('gc', 0)
        self.goroutines = data.get('goroutines', 0)
        self.modules = data.get('modules', [])


def parse_log_line(line: str) -> Optional[MemorySample]:
    """解析日志行，提取 memory_sample 记录"""
    # 查找包含 "memory_sample" 的行
    if '"memory_sample"' not in line and 'memory_sample' not in line:
        return None
    
    try:
        # 尝试解析 JSON（zap 日志格式）
        # 格式示例: {"level":"info","ts":1234567890,"msg":"memory_sample","time":"2025-12-05T16:40:18+08:00","rss_mb":123,...}
        
        # 提取 JSON 部分（从第一个 { 到最后一个 }）
        json_start = line.find('{')
        json_end = line.rfind('}') + 1
        
        if json_start == -1 or json_end == 0:
            return None
        
        json_str = line[json_start:json_end]
        log_entry = json.loads(json_str)
        
        # 检查是否是 memory_sample
        if log_entry.get('msg') != 'memory_sample':
            return None
        
        # 提取内存数据
        sample_data = {
            'time': log_entry.get('time', log_entry.get('ts', '')),
            'rss_mb': log_entry.get('rss_mb', 0),
            'rss_bytes': log_entry.get('rss_bytes', 0),
            'heap_mb': log_entry.get('heap_mb', 0),
            'heap_alloc_bytes': log_entry.get('heap_alloc_bytes', 0),
            'heap_inuse_bytes': log_entry.get('heap_inuse_bytes', 0),
            'gc': log_entry.get('gc', 0),
            'goroutines': log_entry.get('goroutines', 0),
            'modules': log_entry.get('modules', []),
        }
        
        return MemorySample(sample_data)
    except (json.JSONDecodeError, KeyError, ValueError) as e:
        # 如果 JSON 解析失败，尝试正则表达式提取
        return None


def parse_log_file(log_path: Path) -> List[MemorySample]:
    """从日志文件中提取所有 memory_sample 记录"""
    samples = []
    
    if not log_path.exists():
        print(f"错误: 日志文件不存在: {log_path}", file=sys.stderr)
        return samples
    
    print(f"正在解析日志文件: {log_path}")
    
    with open(log_path, 'r', encoding='utf-8', errors='ignore') as f:
        for line_num, line in enumerate(f, 1):
            sample = parse_log_line(line)
            if sample:
                samples.append(sample)
    
    print(f"提取到 {len(samples)} 条内存采样记录")
    return samples


def calculate_hourly_growth(samples: List[MemorySample]) -> List[Dict]:
    """计算每小时的内存增长"""
    if len(samples) < 2:
        return []
    
    hourly_stats = []
    current_hour_start = samples[0].time.replace(minute=0, second=0, microsecond=0)
    hour_samples = []
    
    for sample in samples:
        sample_hour = sample.time.replace(minute=0, second=0, microsecond=0)
        
        if sample_hour != current_hour_start:
            # 计算上一小时的增长
            if len(hour_samples) >= 2:
                first = hour_samples[0]
                last = hour_samples[-1]
                
                hourly_stats.append({
                    'hour': current_hour_start,
                    'rss_start_mb': first.rss_mb,
                    'rss_end_mb': last.rss_mb,
                    'rss_growth_mb': last.rss_mb - first.rss_mb,
                    'rss_growth_percent': ((last.rss_mb - first.rss_mb) / first.rss_mb * 100) if first.rss_mb > 0 else 0,
                    'heap_growth_mb': (last.heap_alloc_bytes - first.heap_alloc_bytes) / 1024 / 1024,
                    'gc_growth': last.gc - first.gc,
                    'goroutines_growth': last.goroutines - first.goroutines,
                    'sample_count': len(hour_samples),
                })
            
            # 开始新的一小时
            current_hour_start = sample_hour
            hour_samples = [sample]
        else:
            hour_samples.append(sample)
    
    # 处理最后一小时
    if len(hour_samples) >= 2:
        first = hour_samples[0]
        last = hour_samples[-1]
        
        hourly_stats.append({
            'hour': current_hour_start,
            'rss_start_mb': first.rss_mb,
            'rss_end_mb': last.rss_mb,
            'rss_growth_mb': last.rss_mb - first.rss_mb,
            'rss_growth_percent': ((last.rss_mb - first.rss_mb) / first.rss_mb * 100) if first.rss_mb > 0 else 0,
            'heap_growth_mb': (last.heap_alloc_bytes - first.heap_alloc_bytes) / 1024 / 1024,
            'gc_growth': last.gc - first.gc,
            'goroutines_growth': last.goroutines - first.goroutines,
            'sample_count': len(hour_samples),
        })
    
    return hourly_stats


def generate_csv_report(samples: List[MemorySample], output_path: Path):
    """生成 CSV 报告"""
    import csv
    
    with open(output_path, 'w', newline='', encoding='utf-8') as f:
        writer = csv.writer(f)
        
        # 写入表头
        writer.writerow([
            '时间', 'RSS(MB)', 'RSS(Bytes)', 'Heap(MB)', 'HeapAlloc(Bytes)',
            'HeapInuse(Bytes)', 'GC次数', 'Goroutines', '模块数'
        ])
        
        # 写入数据
        for sample in samples:
            writer.writerow([
                sample.time.isoformat(),
                sample.rss_mb,
                sample.rss_bytes,
                sample.heap_mb,
                sample.heap_alloc_bytes,
                sample.heap_inuse_bytes,
                sample.gc,
                sample.goroutines,
                len(sample.modules),
            ])
    
    print(f"CSV 报告已生成: {output_path}")


def generate_summary_report(samples: List[MemorySample], hourly_stats: List[Dict], output_path: Optional[Path] = None):
    """生成文本摘要报告"""
    if len(samples) < 2:
        print("警告: 采样数据不足，无法生成报告")
        return
    
    first = samples[0]
    last = samples[-1]
    duration = (last.time - first.time).total_seconds() / 3600  # 小时
    
    rss_growth = last.rss_mb - first.rss_mb
    rss_growth_percent = (rss_growth / first.rss_mb * 100) if first.rss_mb > 0 else 0
    rss_growth_per_hour = rss_growth / duration if duration > 0 else 0
    
    heap_growth_mb = (last.heap_alloc_bytes - first.heap_alloc_bytes) / 1024 / 1024
    gc_growth = last.gc - first.gc
    goroutines_growth = last.goroutines - first.goroutines
    
    report_lines = [
        "=" * 60,
        "内存测试分析报告",
        "=" * 60,
        "",
        f"测试时长: {duration:.2f} 小时",
        f"采样次数: {len(samples)}",
        f"采样间隔: {duration * 3600 / len(samples):.1f} 秒",
        "",
        "【总体变化】",
        f"  RSS: {first.rss_mb} MB → {last.rss_mb} MB (增长: {rss_growth:+d} MB, {rss_growth_percent:+.2f}%)",
        f"  平均每小时增长: {rss_growth_per_hour:+.2f} MB/h",
        f"  Heap: {first.heap_mb:.1f} MB → {last.heap_mb:.1f} MB (增长: {heap_growth_mb:+.1f} MB)",
        f"  GC: {first.gc} → {last.gc} (增长: {gc_growth:+d})",
        f"  Goroutines: {first.goroutines} → {last.goroutines} (增长: {goroutines_growth:+d})",
        "",
    ]
    
    # 评估结果
    if rss_growth_per_hour < 20 and rss_growth_percent < 2:
        status = "✅ 正常"
        status_desc = "内存增长在正常范围内"
    elif rss_growth_per_hour < 50 and rss_growth_percent < 5:
        status = "⚠️  可疑"
        status_desc = "内存增长略高，建议继续观察"
    else:
        status = "❌ 异常"
        status_desc = "内存增长异常，可能存在泄漏"
    
    report_lines.extend([
        "【评估结果】",
        f"  状态: {status}",
        f"  说明: {status_desc}",
        "",
    ])
    
    # 每小时统计
    if hourly_stats:
        report_lines.extend([
            "【每小时增长统计】",
            f"{'时间':<20} {'RSS起始':<12} {'RSS结束':<12} {'增长(MB)':<12} {'增长(%)':<10} {'GC增长':<10} {'Goroutines增长':<15}",
            "-" * 100,
        ])
        
        for stat in hourly_stats:
            report_lines.append(
                f"{stat['hour'].strftime('%Y-%m-%d %H:00'):<20} "
                f"{stat['rss_start_mb']:<12} "
                f"{stat['rss_end_mb']:<12} "
                f"{stat['rss_growth_mb']:+12.1f} "
                f"{stat['rss_growth_percent']:+10.2f}% "
                f"{stat['gc_growth']:+10} "
                f"{stat['goroutines_growth']:+15}"
            )
        
        report_lines.append("")
    
    report_lines.append("=" * 60)
    
    report_text = "\n".join(report_lines)
    
    if output_path:
        with open(output_path, 'w', encoding='utf-8') as f:
            f.write(report_text)
        print(f"摘要报告已生成: {output_path}")
    else:
        print(report_text)


def plot_memory_trend(samples: List[MemorySample], output_path: Path):
    """生成内存趋势图"""
    if not HAS_MATPLOTLIB:
        print("警告: matplotlib 未安装，跳过图表生成")
        return
    
    if len(samples) < 2:
        print("警告: 采样数据不足，无法生成图表")
        return
    
    times = [s.time for s in samples]
    rss_mb = [s.rss_mb for s in samples]
    heap_mb = [s.heap_mb for s in samples]
    goroutines = [s.goroutines for s in samples]
    
    fig, axes = plt.subplots(3, 1, figsize=(12, 10))
    
    # RSS 趋势
    axes[0].plot(times, rss_mb, 'b-', linewidth=1.5, label='RSS (MB)')
    axes[0].set_ylabel('RSS (MB)', fontsize=10)
    axes[0].set_title('内存趋势分析', fontsize=12, fontweight='bold')
    axes[0].grid(True, alpha=0.3)
    axes[0].legend()
    
    # Heap 趋势
    axes[1].plot(times, heap_mb, 'g-', linewidth=1.5, label='Heap (MB)')
    axes[1].set_ylabel('Heap (MB)', fontsize=10)
    axes[1].grid(True, alpha=0.3)
    axes[1].legend()
    
    # Goroutines 趋势
    axes[2].plot(times, goroutines, 'r-', linewidth=1.5, label='Goroutines')
    axes[2].set_ylabel('Goroutines', fontsize=10)
    axes[2].set_xlabel('时间', fontsize=10)
    axes[2].grid(True, alpha=0.3)
    axes[2].legend()
    
    plt.tight_layout()
    plt.savefig(output_path, dpi=150, bbox_inches='tight')
    print(f"趋势图已生成: {output_path}")


def main():
    parser = argparse.ArgumentParser(
        description='从节点日志中提取并分析内存采样数据',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    
    parser.add_argument(
        '--log',
        type=Path,
        required=True,
        help='节点日志文件路径（node-system.log 或 node-business.log）'
    )
    
    parser.add_argument(
        '--output',
        type=Path,
        required=True,
        help='CSV 输出文件路径'
    )
    
    parser.add_argument(
        '--summary',
        type=Path,
        help='文本摘要报告输出路径（可选）'
    )
    
    parser.add_argument(
        '--plot',
        type=Path,
        help='趋势图输出路径（可选，需要 matplotlib）'
    )
    
    parser.add_argument(
        '--no-plot',
        action='store_true',
        help='跳过图表生成（即使 matplotlib 可用）'
    )
    
    args = parser.parse_args()
    
    # 解析日志
    samples = parse_log_file(args.log)
    
    if not samples:
        print("错误: 未找到任何 memory_sample 记录", file=sys.stderr)
        sys.exit(1)
    
    # 生成 CSV 报告
    generate_csv_report(samples, args.output)
    
    # 计算每小时增长
    hourly_stats = calculate_hourly_growth(samples)
    
    # 生成摘要报告
    if args.summary:
        generate_summary_report(samples, hourly_stats, args.summary)
    else:
        generate_summary_report(samples, hourly_stats)
    
    # 生成趋势图
    if args.plot and not args.no_plot:
        plot_memory_trend(samples, args.plot)


if __name__ == '__main__':
    main()

