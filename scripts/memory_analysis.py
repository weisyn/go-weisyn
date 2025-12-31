#!/usr/bin/env python3
"""
WES 内存问题分析工具

用途：分析 WES 节点的内存使用情况，定位潜在的内存问题
功能：
1. 获取内存监控数据
2. 分析各模块的内存使用
3. 识别潜在问题
4. 生成诊断报告
"""

import json
import sys
import urllib.request
import urllib.error
from typing import Dict, List, Any

# 配置
DEFAULT_API_URL = "http://localhost:28680"
MEMORY_ENDPOINT = "/api/v1/system/memory"


def fetch_memory_data(api_url: str) -> Dict[str, Any]:
    """获取内存监控数据"""
    url = f"{api_url}{MEMORY_ENDPOINT}"
    try:
        with urllib.request.urlopen(url, timeout=5) as response:
            return json.loads(response.read().decode())
    except urllib.error.URLError as e:
        print(f"❌ 无法连接到节点: {e}")
        print(f"   请确保节点正在运行，并可通过 {api_url} 访问")
        sys.exit(1)
    except json.JSONDecodeError as e:
        print(f"❌ JSON 解析错误: {e}")
        sys.exit(1)


def format_bytes(bytes_value: int) -> str:
    """格式化字节数为可读格式"""
    for unit in ['B', 'KB', 'MB', 'GB']:
        if bytes_value < 1024.0:
            return f"{bytes_value:.2f} {unit}"
        bytes_value /= 1024.0
    return f"{bytes_value:.2f} TB"


def analyze_memory(data: Dict[str, Any]) -> None:
    """分析内存使用情况"""
    runtime = data.get('runtime', {})
    modules = data.get('modules', [])
    
    # 显示运行时统计
    print("=" * 100)
    print("运行时内存统计")
    print("=" * 100)
    heap_alloc = runtime.get('heap_alloc', 0)
    heap_inuse = runtime.get('heap_inuse', 0)
    num_gc = runtime.get('num_gc', 0)
    num_goroutine = runtime.get('num_goroutine', 0)
    
    print(f"堆分配:     {format_bytes(heap_alloc)} ({heap_alloc:,} bytes)")
    print(f"堆使用:     {format_bytes(heap_inuse)} ({heap_inuse:,} bytes)")
    print(f"GC 次数:    {num_gc:,}")
    print(f"Goroutine:  {num_goroutine:,}")
    print()
    
    if not modules:
        print("⚠️  未找到模块统计数据")
        print("   可能原因：")
        print("     1. 节点刚启动，MemoryDoctor 尚未采样")
        print("     2. 模块未正确注册 MemoryReporter")
        return
    
    # 按内存使用排序
    sorted_modules = sorted(modules, key=lambda x: x.get('approx_bytes', 0), reverse=True)
    
    print("=" * 100)
    print("模块内存使用排名（按 approx_bytes 降序）")
    print("=" * 100)
    print(f"{'模块':<30} {'层级':<20} {'对象数':<15} {'内存':<15} {'缓存':<12} {'队列':<12}")
    print("-" * 100)
    
    total_memory = 0
    for mod in sorted_modules:
        module_name = mod.get('module', 'unknown')
        layer = mod.get('layer', 'unknown')
        objects = mod.get('objects', 0)
        approx_bytes = mod.get('approx_bytes', 0)
        cache_items = mod.get('cache_items', 0)
        queue_length = mod.get('queue_length', 0)
        
        memory_mb = approx_bytes / 1024 / 1024
        total_memory += memory_mb
        
        print(f"{module_name:<30} {layer:<20} {objects:<15,} {format_bytes(approx_bytes):<15} "
              f"{cache_items:<12,} {queue_length:<12,}")
    
    print("-" * 100)
    print(f"{'总计':<30} {'':<20} {'':<15} {format_bytes(int(total_memory * 1024 * 1024)):<15} {'':<12} {'':<12}")
    print()
    
    # 识别潜在问题
    print("=" * 100)
    print("🔍 潜在问题分析")
    print("=" * 100)
    
    issues = []
    warnings = []
    
    for mod in sorted_modules:
        module_name = mod.get('module', 'unknown')
        objects = mod.get('objects', 0)
        approx_bytes = mod.get('approx_bytes', 0)
        cache_items = mod.get('cache_items', 0)
        queue_length = mod.get('queue_length', 0)
        memory_mb = approx_bytes / 1024 / 1024
        
        # 检查内存使用超过 100MB 的模块
        if memory_mb > 100:
            issues.append(f"⚠️  {module_name}: 内存使用较高 ({memory_mb:.2f} MB)")
        
        # 检查对象数量异常
        if objects > 100000:
            warnings.append(f"⚠️  {module_name}: 对象数量异常 ({objects:,})")
        
        # 检查队列长度异常
        if queue_length > 10000:
            warnings.append(f"⚠️  {module_name}: 队列长度异常 ({queue_length:,})")
        
        # 检查缓存条目异常
        if cache_items > 100000:
            warnings.append(f"⚠️  {module_name}: 缓存条目异常 ({cache_items:,})")
    
    if issues:
        print("🚨 发现的问题：")
        for issue in issues:
            print(f"  {issue}")
        print()
    
    if warnings:
        print("⚠️  警告：")
        for warning in warnings:
            print(f"  {warning}")
        print()
    
    if not issues and not warnings:
        print("✅ 未发现明显的内存问题")
        print()
    
    # 提供建议
    print("=" * 100)
    print("💡 建议")
    print("=" * 100)
    print("1. 定期运行此脚本监控内存趋势")
    print("2. 关注内存使用超过 100MB 的模块")
    print("3. 检查队列长度和缓存条目是否异常增长")
    print("4. 使用 MemoryDoctor 的历史数据追踪内存增长趋势")
    print("5. 如果发现内存持续增长，检查是否有内存泄漏")
    print()


def main():
    """主函数"""
    api_url = sys.argv[1] if len(sys.argv) > 1 else DEFAULT_API_URL
    
    print("=" * 100)
    print("WES 内存问题分析工具")
    print("=" * 100)
    print()
    print(f"节点地址: {api_url}")
    print()
    
    # 获取内存数据
    print("正在获取内存监控数据...")
    data = fetch_memory_data(api_url)
    print("✅ 数据获取成功")
    print()
    
    # 分析内存
    analyze_memory(data)


if __name__ == "__main__":
    main()

