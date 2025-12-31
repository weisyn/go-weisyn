// Package sync 内存监控工具函数
package sync

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// MemorySnapshot 内存快照
//
// 包含关键的内存指标，用于统一的内存监控和日志输出
type MemorySnapshot struct {
	HeapAllocMB uint64 // Go heap 分配（MB）- 可能包含虚拟内存预留
	RSSMB       uint64 // 真实物理内存（MB）- 唯一可信的内存占用指标
	HeapInuseMB uint64 // 正在使用的堆（MB）
	HeapSysMB   uint64 // 从OS获取的堆虚拟内存（MB）
	HeapIdleMB  uint64 // 空闲但未归还OS的堆（MB）
	HeapObjects uint64 // 堆对象数
	NumGC       uint32 // GC 次数
}

// GetMemorySnapshot 获取当前内存快照（支持 macOS 和 Linux）
//
// 返回：
//   - MemorySnapshot: 包含所有关键内存指标的快照
//
// 说明：
//   - HeapAllocMB 在 macOS 上可能包含虚拟内存预留，不代表真实物理内存
//   - RSSMB 是唯一可信的真实物理内存占用指标
//   - 如果无法获取 RSS（如不支持的平台），RSSMB 将为 0
func GetMemorySnapshot() MemorySnapshot {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// 获取 RSS（真实物理内存）
	rssBytes := getRSSBytes()

	return MemorySnapshot{
		HeapAllocMB: m.Alloc / 1024 / 1024,
		RSSMB:       rssBytes / 1024 / 1024,
		HeapInuseMB: m.HeapInuse / 1024 / 1024,
		HeapSysMB:   m.HeapSys / 1024 / 1024,
		HeapIdleMB:  m.HeapIdle / 1024 / 1024,
		HeapObjects: m.HeapObjects,
		NumGC:       m.NumGC,
	}
}

// getRSSBytes 获取进程真实物理内存（RSS）
//
// 返回：
//   - uint64: RSS 字节数
//   - 如果获取失败或不支持，返回 0
//
// 说明：
//   - macOS: 使用 syscall.Getrusage 获取 ru_maxrss（单位：字节）
//     ⚠️ 注意：ru_maxrss 返回的是峰值 RSS（进程运行期间的最大值），不是当前 RSS
//     这意味着即使内存已释放，Maxrss 也不会减少，只会增加
//     因此日志中的 RSS 值可能高于 ps aux 显示的当前 RSS
//   - Linux: 读取 /proc/self/status 获取 VmRSS（单位：KB，当前RSS）
//   - 其他平台：返回 0
func getRSSBytes() uint64 {
	switch runtime.GOOS {
	case "darwin":
		// macOS: 使用 syscall.Getrusage
		// 注意：macOS 的 ru_maxrss 单位是字节，返回的是峰值 RSS（不是当前RSS）
		var rusage syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err != nil {
			return 0
		}
		// macOS 上 ru_maxrss 单位是字节，返回峰值 RSS
		return uint64(rusage.Maxrss)
	case "linux":
		// Linux: 读取 /proc/self/status
		return getRSSBytesFromProc()
	default:
		// 其他平台暂不支持
		return 0
	}
}

// getRSSBytesFromProc 从 /proc/self/status 读取 RSS（Linux）
func getRSSBytesFromProc() uint64 {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			// 格式：VmRSS:    12345 kB
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return 0
				}
				return kb * 1024 // 转换为字节
			}
		}
	}

	return 0
}

// FormatMemoryLog 格式化内存日志消息
//
// 参数：
//   - prefix: 日志前缀（如"🧹 同步开始前内存状态"）
//
// 返回：
//   - string: 格式化的日志消息
//
// 示例输出：
//
//	🧹 同步开始前内存状态: heap_alloc=100635MB rss=325MB heap_inuse=100633MB heap_sys=105473MB (heap_idle=44MB, heap_objects=127272, gc_count=14)
func (s MemorySnapshot) FormatMemoryLog(prefix string) string {
	return fmt.Sprintf("%s: heap_alloc=%dMB rss=%dMB heap_inuse=%dMB heap_sys=%dMB "+
		"(heap_idle=%dMB, heap_objects=%d, gc_count=%d)",
		prefix, s.HeapAllocMB, s.RSSMB, s.HeapInuseMB, s.HeapSysMB,
		s.HeapIdleMB, s.HeapObjects, s.NumGC)
}

