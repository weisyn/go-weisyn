#!/bin/bash
# 检查进程的mmap映射和RSS使用情况
# 用于对比M1和Intel芯片上的内存使用差异

if [ $# -eq 0 ]; then
    echo "用法: $0 <进程名或PID>"
    echo "示例: $0 weisyn-node"
    echo "示例: $0 12345"
    exit 1
fi

TARGET=$1

# 获取PID
if [[ "$TARGET" =~ ^[0-9]+$ ]]; then
    PID=$TARGET
else
    PID=$(pgrep -f "$TARGET" | head -1)
fi

if [ -z "$PID" ]; then
    echo "错误: 未找到进程 $TARGET"
    exit 1
fi

echo "=========================================="
echo "进程内存使用分析 (PID: $PID)"
echo "=========================================="
echo ""

# 1. 基本RSS信息
echo "=== 1. 基本RSS信息 ==="
ps aux | grep "^[^ ]* *$PID " | awk '{printf "RSS: %.2f MB\n", $6/1024}'
echo ""

# 2. 系统架构
echo "=== 2. 系统架构 ==="
uname -m
sysctl -n machdep.cpu.brand_string 2>/dev/null || echo "无法获取CPU信息"
echo ""

# 3. mmap映射统计（macOS）
if [[ "$(uname)" == "Darwin" ]]; then
    echo "=== 3. mmap映射统计 (vmmap) ==="
    echo ""
    
    echo "--- REGION SUMMARY ---"
    vmmap -summary $PID 2>/dev/null | grep -A 30 "REGION TYPE" | head -40
    echo ""
    
    echo "--- vlog文件的mmap映射 ---"
    vmmap $PID 2>/dev/null | grep "\.vlog" -A 3 | head -30
    echo ""
    
    echo "--- 所有Mapped File的内存使用 ---"
    vmmap $PID 2>/dev/null | grep -i "mapped file" | awk '{
        # 提取内存大小（格式: "    Mapped file     1234K"）
        for(i=1; i<=NF; i++) {
            if ($i ~ /[0-9]+K/ || $i ~ /[0-9]+M/) {
                size = $i
                gsub(/K/, "*1024", size)
                gsub(/M/, "*1024*1024", size)
                # 简单计算（这里只是显示，不实际计算）
                print "  " $0
            }
        }
    }'
    echo ""
fi

# 4. 详细内存映射（如果可用）
echo "=== 4. 关键内存区域 ==="
if [[ "$(uname)" == "Darwin" ]]; then
    vmmap $PID 2>/dev/null | grep -E "(REGION TYPE|TOTAL|\.vlog)" | head -20
fi
echo ""

echo "=========================================="
echo "分析完成"
echo ""
echo "对比要点："
echo "1. RSS大小（物理内存占用）"
echo "2. mmap映射的总大小（虚拟地址空间）"
echo "3. vlog文件的实际RSS占用（物理内存中的mmap页面）"
echo "=========================================="

