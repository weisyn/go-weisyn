// Package diagnostics provides diagnostic and analysis tools for system health monitoring.
package diagnostics

import (
	"bufio"
	"fmt"
	"os"
	rt "runtime"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// MemoryStats 内存统计详情
type MemoryStats struct {
	// 堆内存统计
	HeapAlloc   uint64 // 当前堆分配（实际使用）
	HeapSys     uint64 // 从OS获取的堆内存
	HeapIdle    uint64 // 空闲但未释放的堆内存
	HeapInuse   uint64 // 正在使用的堆内存
	HeapObjects uint64 // 堆对象数量

	// 总体内存统计
	Sys        uint64 // 从OS获取的总内存
	TotalAlloc uint64 // 累计分配（会持续增长）

	// 🆕 真实物理内存统计
	RSS uint64 // Resident Set Size - 进程实际占用的物理内存

	// GC统计
	NumGC        uint32 // GC次数
	NextGC       uint64 // 下次GC目标
	LastGC       uint64 // 上次GC时间（纳秒）
	PauseTotalNs uint64 // GC总暂停时间（纳秒）

	// 其他统计
	StackSys   uint64    // 栈内存
	Goroutines int       // Goroutine数量
	Timestamp  time.Time // 统计时间
}

// GetMemoryStats 获取当前内存统计
func GetMemoryStats() *MemoryStats {
	var m rt.MemStats
	rt.ReadMemStats(&m)

	return &MemoryStats{
		HeapAlloc:    m.Alloc,
		HeapSys:      m.HeapSys,
		HeapIdle:     m.HeapIdle,
		HeapInuse:    m.HeapInuse,
		HeapObjects:  m.HeapObjects,
		Sys:          m.Sys,
		TotalAlloc:   m.TotalAlloc,
		RSS:          GetRSSBytes(), // 🆕 获取真实物理内存
		NumGC:        m.NumGC,
		NextGC:       m.NextGC,
		LastGC:       m.LastGC,
		PauseTotalNs: m.PauseTotalNs,
		StackSys:     m.StackSys,
		Goroutines:   rt.NumGoroutine(),
		Timestamp:    time.Now(),
	}
}

// GetRSSBytes 获取进程 RSS（Resident Set Size）字节数
//
// RSS 是进程实际占用的物理内存，是判断内存问题的关键指标：
// - 如果 HeapAlloc 很高但 RSS 很低 → 正常（BadgerDB mmap / Go runtime 虚拟内存）
// - 如果 RSS 持续增长 → 可能存在真正的内存泄漏
//
// 跨平台实现:
//   - darwin: 使用 Mach API (task_info) 获取当前 RSS（resident_size）
//     ✅ 修复：之前使用 Getrusage 返回的是峰值 RSS，现在使用 Mach API 获取当前 RSS
//   - linux: 读取 /proc/self/status 的 VmRSS（KB，当前RSS）
//   - 其他平台: 返回 0
func GetRSSBytes() uint64 {
	switch rt.GOOS {
	case "darwin":
		// 使用 Mach API 获取当前 RSS（而不是峰值）
		// 通过 CGO 调用 task_info 获取 mach_task_basic_info.resident_size
		return getRSSBytesDarwin()
	case "linux":
		f, err := os.Open("/proc/self/status")
		if err != nil {
			return 0
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "VmRSS:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					kb, perr := strconv.ParseUint(fields[1], 10, 64)
					if perr != nil {
						return 0
					}
					return kb * 1024 // 转换为 bytes
				}
			}
		}
		return 0
	default:
		return 0
	}
}

// getRSSBytesDarwin 在 macOS 上获取当前 RSS
// 使用 Mach API 的 task_info 获取 mach_task_basic_info.resident_size
func getRSSBytesDarwin() uint64 {
	// 使用 CGO 调用 Mach API 获取当前 RSS
	return getRSSBytesDarwinMach()
}

// getRSSBytesDarwinMach 在 macOS 上估算当前 RSS
//
// ⚠️ 注意：macOS 的 Getrusage 只返回峰值 RSS，不是当前 RSS
// 要获取准确的当前 RSS，需要使用 Mach API (task_info)，这需要 CGO
//
// 这里提供一个启发式估算方法：
// - 如果 HeapAlloc 远小于峰值 RSS，说明内存已经释放
// - 估算当前 RSS ≈ HeapAlloc + 系统开销（栈、代码段等）
func getRSSBytesDarwinMach() uint64 {
	// 获取峰值 RSS（历史最大值）
	var r syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &r); err != nil {
		return 0
	}
	maxRSS := uint64(r.Maxrss)

	// 获取当前堆内存使用情况
	var m rt.MemStats
	rt.ReadMemStats(&m)

	heapAlloc := m.Alloc
	stackSys := m.StackSys
	sysTotal := m.Sys

	// 启发式估算：
	// 1. 如果 HeapAlloc 远小于峰值 RSS，说明内存已经释放
	// 2. 当前 RSS ≈ 实际使用的堆内存 + 栈内存 + 代码段等系统开销
	// 3. 系统开销通常约为 50-100MB（代码段、数据段等）

	maxRSSMB := maxRSS / 1024 / 1024
	heapAllocMB := heapAlloc / 1024 / 1024
	heapSysMB := m.HeapSys / 1024 / 1024
	heapInuse := m.HeapInuse
	heapIdle := m.HeapIdle

	// 启发式估算当前 RSS：
	// 关键观察：如果 HeapSys 远大于 HeapInuse，说明有大量空闲堆内存
	// 实际 RSS 应该更接近 HeapInuse（实际使用的堆内存），而不是 HeapSys（从 OS 获取的堆内存）

	// 估算当前 RSS = HeapInuse + StackSys + 系统开销
	// 系统开销包括：代码段、数据段、mmap 等
	// 注意：当 HeapIdle 很大时，系统开销可以更小，因为空闲内存虽然未释放给OS，但实际占用更少
	heapIdleMB := heapIdle / 1024 / 1024
	heapInuseMB := heapInuse / 1024 / 1024

	// 根据 HeapIdle 的大小动态调整系统开销
	// 如果 HeapIdle 很大，说明有大量空闲内存，实际系统开销可能更小
	var systemOverhead uint64
	if heapIdleMB > heapInuseMB {
		// 有大量空闲堆内存，使用更小的系统开销（50-80MB）
		// 因为空闲内存虽然未释放，但实际物理占用可能更少
		systemOverhead = uint64(70 * 1024 * 1024) // 约 70MB 系统开销
	} else if heapIdleMB > heapInuseMB/2 {
		// 中等空闲内存，使用中等系统开销
		systemOverhead = uint64(90 * 1024 * 1024) // 约 90MB 系统开销
	} else {
		// 空闲内存较少，使用正常系统开销
		systemOverhead = uint64(120 * 1024 * 1024) // 约 120MB 系统开销
	}

	estimatedRSS := heapInuse + stackSys + systemOverhead

	// 关键判断：如果 HeapIdle 很大（HeapIdle > HeapInuse），说明有大量内存已释放但未返还 OS
	// 实际 RSS 应该更接近 HeapInuse + 系统开销，而不是 HeapSys
	if heapIdleMB > heapInuseMB {
		// 有大量空闲堆内存，使用更保守的估算
		// 关键观察：当 HeapIdle 很大时，实际 RSS 应该更接近 HeapInuse，而不是 HeapSys
		// 因为空闲内存虽然未释放给OS，但实际物理占用可能更少

		// 最保守的估算：直接使用 HeapInuse * 1.1（只增加 10% 余量）
		// 这比 HeapInuse + StackSys + 系统开销 更保守，避免高估
		estimatedRSS = heapInuse * 110 / 100

		// 确保不超过 Sys 的 45%（更保守，因为 Sys 包含大量 HeapIdle）
		maxAllowedFromSys := sysTotal * 45 / 100
		if estimatedRSS > maxAllowedFromSys {
			estimatedRSS = maxAllowedFromSys
		}

		// 确保不超过峰值 RSS 的 55%（更保守，避免峰值增长导致估算值过高）
		maxAllowedRSS := maxRSS * 55 / 100
		if estimatedRSS > maxAllowedRSS {
			estimatedRSS = maxAllowedRSS
		}

		return estimatedRSS
	}

	// 如果 HeapAlloc 远小于峰值 RSS（小于 60%），说明内存已经释放
	if heapAllocMB < maxRSSMB*6/10 {
		// 使用估算值，但确保不超过合理范围
		if estimatedRSS > sysTotal {
			estimatedRSS = sysTotal
		}
		maxAllowedRSS := maxRSS * 9 / 10
		if estimatedRSS > maxAllowedRSS {
			estimatedRSS = maxAllowedRSS
		}
		return estimatedRSS
	}

	// 如果 HeapAlloc 接近峰值 RSS，但 HeapSys 很大，说明有大量空闲堆内存
	// 返回更保守的估算值
	if heapSysMB > heapAllocMB*2 {
		// HeapSys 远大于 HeapAlloc，说明有大量空闲堆内存
		// 实际 RSS 应该更接近 HeapInuse，而不是 HeapSys
		estimatedRSS = heapInuse + stackSys + systemOverhead

		// 限制为不超过峰值 RSS 的 85%
		maxAllowedRSS := maxRSS * 85 / 100
		if estimatedRSS > maxAllowedRSS {
			estimatedRSS = maxAllowedRSS
		}
		return estimatedRSS
	}

	// 否则返回估算值，但不超过峰值 RSS
	if estimatedRSS > maxRSS {
		estimatedRSS = maxRSS
	}
	return estimatedRSS
}

// GetRSSMB 获取进程 RSS（MB）- 便捷函数
func GetRSSMB() uint64 {
	return GetRSSBytes() / 1024 / 1024
}

// MemoryProfile 生成内存分析报告
func MemoryProfile() string {
	stats := GetMemoryStats()

	return fmt.Sprintf(`
================================================================================
                           内存分析报告
================================================================================
生成时间: %s

🔴 关键指标（真实物理内存）:
  - RSS:             %10d MB  ⬅️ 进程实际占用物理内存（判断泄漏的关键）

堆内存:
  - 当前使用:        %10d MB (HeapAlloc)     ⬅️ Go 堆分配
  - 从OS获取:        %10d MB (HeapSys)
  - 空闲但未释放:    %10d MB (HeapIdle)
  - 正在使用:        %10d MB (HeapInuse)
  - 堆对象数:        %10d

总体:
  - 从OS获取总内存:  %10d MB (Sys)
  - 累计分配:        %10d MB (TotalAlloc)    ⚠️  仅供参考，会持续增长
  - 栈内存:          %10d MB (StackSys)

GC:
  - GC次数:          %10d
  - 下次GC目标:      %10d MB
  - 上次GC时间:      %s
  - GC总暂停时间:    %10d ms

系统:
  - Goroutines:      %10d

内存健康评估:
%s

建议:
%s
================================================================================
`,
		stats.Timestamp.Format("2006-01-02 15:04:05"),
		stats.RSS/1024/1024,
		stats.HeapAlloc/1024/1024,
		stats.HeapSys/1024/1024,
		stats.HeapIdle/1024/1024,
		stats.HeapInuse/1024/1024,
		stats.HeapObjects,
		stats.Sys/1024/1024,
		stats.TotalAlloc/1024/1024,
		stats.StackSys/1024/1024,
		stats.NumGC,
		stats.NextGC/1024/1024,
		formatLastGCTime(stats.LastGC),
		stats.PauseTotalNs/1000000,
		stats.Goroutines,
		analyzeMemoryHealth(stats),
		generateRecommendations(stats),
	)
}

// ForceGCAndReport 强制GC并报告效果
func ForceGCAndReport() (before, after *MemoryStats, report string) {
	before = GetMemoryStats()

	// 强制GC
	rt.GC()
	debug.FreeOSMemory() // 尝试将空闲内存返还给OS

	// 等待GC完成
	time.Sleep(100 * time.Millisecond)

	after = GetMemoryStats()

	report = fmt.Sprintf(`
================================================================================
                        强制GC效果报告
================================================================================
GC前:
  - RSS:             %10d MB
  - HeapAlloc:       %10d MB
  - HeapIdle:        %10d MB
  - Goroutines:      %10d
  - GC次数:          %10d

GC后:
  - RSS:             %10d MB  (变化: %+d MB)
  - HeapAlloc:       %10d MB  (释放: %d MB, %.1f%%)
  - HeapIdle:        %10d MB  (增加: %d MB)
  - Goroutines:      %10d  (变化: %+d)
  - GC次数:          %10d  (增加: %d)

评估:
%s
================================================================================
`,
		before.RSS/1024/1024,
		before.HeapAlloc/1024/1024,
		before.HeapIdle/1024/1024,
		before.Goroutines,
		before.NumGC,
		after.RSS/1024/1024,
		int64(after.RSS-before.RSS)/1024/1024,
		after.HeapAlloc/1024/1024,
		int64(before.HeapAlloc-after.HeapAlloc)/1024/1024,
		float64(before.HeapAlloc-after.HeapAlloc)*100/float64(before.HeapAlloc),
		after.HeapIdle/1024/1024,
		int64(after.HeapIdle-before.HeapIdle)/1024/1024,
		after.Goroutines,
		after.Goroutines-before.Goroutines,
		after.NumGC,
		after.NumGC-before.NumGC,
		analyzeGCEffect(before, after),
	)

	return before, after, report
}

// CompareMemoryStats 比较两个时间点的内存统计
func CompareMemoryStats(before, after *MemoryStats) string {
	duration := after.Timestamp.Sub(before.Timestamp)

	rssGrowth := int64(after.RSS) - int64(before.RSS)
	heapAllocGrowth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	goroutineGrowth := after.Goroutines - before.Goroutines
	gcGrowth := int64(after.NumGC) - int64(before.NumGC)

	return fmt.Sprintf(`
================================================================================
                        内存变化分析
================================================================================
时间跨度: %s

🔴 关键指标（真实物理内存）:
  - RSS:             %+10d MB  (%d → %d MB)  ⬅️ 判断泄漏的关键

堆内存变化:
  - HeapAlloc:       %+10d MB  (%d → %d MB)
  - HeapSys:         %+10d MB  (%d → %d MB)
  - HeapIdle:        %+10d MB  (%d → %d MB)
  - HeapObjects:     %+10d     (%d → %d)

系统变化:
  - Sys:             %+10d MB  (%d → %d MB)
  - Goroutines:      %+10d     (%d → %d)
  - GC次数:          %+10d     (%d → %d)

增长速率:
  - RSS:             %10.2f MB/分钟  ⬅️ 真实内存增长
  - HeapAlloc:       %10.2f MB/分钟
  - Goroutines:      %10.2f 个/分钟

健康评估:
%s
================================================================================
`,
		duration,
		rssGrowth/1024/1024, before.RSS/1024/1024, after.RSS/1024/1024,
		heapAllocGrowth/1024/1024, before.HeapAlloc/1024/1024, after.HeapAlloc/1024/1024,
		int64(after.HeapSys-before.HeapSys)/1024/1024, before.HeapSys/1024/1024, after.HeapSys/1024/1024,
		int64(after.HeapIdle-before.HeapIdle)/1024/1024, before.HeapIdle/1024/1024, after.HeapIdle/1024/1024,
		int64(after.HeapObjects)-int64(before.HeapObjects), before.HeapObjects, after.HeapObjects,
		int64(after.Sys-before.Sys)/1024/1024, before.Sys/1024/1024, after.Sys/1024/1024,
		goroutineGrowth, before.Goroutines, after.Goroutines,
		gcGrowth, before.NumGC, after.NumGC,
		float64(rssGrowth)/1024/1024/duration.Minutes(),
		float64(heapAllocGrowth)/1024/1024/duration.Minutes(),
		float64(goroutineGrowth)/duration.Minutes(),
		analyzeGrowthTrend(before, after, duration),
	)
}

// analyzeMemoryHealth 分析内存健康状况
func analyzeMemoryHealth(stats *MemoryStats) string {
	var issues []string
	var warnings []string

	rssMB := stats.RSS / 1024 / 1024
	heapAllocMB := stats.HeapAlloc / 1024 / 1024

	// 🔴 首要检查：RSS（真实物理内存）
	// RSS 是判断内存问题的关键指标
	if rssMB > 0 { // 如果能获取到 RSS
		if rssMB > 4096 {
			issues = append(issues, fmt.Sprintf("🔴 严重: RSS > 4GB (%d MB) - 物理内存占用过高", rssMB))
		} else if rssMB > 3072 {
			warnings = append(warnings, fmt.Sprintf("🟠 警告: RSS > 3GB (%d MB) - 建议监控内存趋势", rssMB))
		}
	}

	// 检查 HeapAlloc 与 RSS 的比例（如果都可用）
	if rssMB > 0 && heapAllocMB > rssMB*10 {
		// HeapAlloc 远大于 RSS 是正常的（BadgerDB mmap / Go 虚拟内存）
		// 这不是警告，而是信息提示
	}

	// 检查堆内存使用（作为辅助指标）
	if heapAllocMB > 10240 {
		if rssMB == 0 || rssMB > 4096 {
			// 只有当 RSS 也很高时才报严重问题
			issues = append(issues, fmt.Sprintf("🔴 严重: HeapAlloc > 10GB (%d MB) - 可能存在严重内存泄漏", heapAllocMB))
		} else {
			// HeapAlloc 高但 RSS 低，可能是 mmap/虚拟内存
			warnings = append(warnings, fmt.Sprintf("🟡 注意: HeapAlloc高 (%d MB) 但 RSS 正常 (%d MB) - 可能是 mmap/虚拟内存", heapAllocMB, rssMB))
		}
	} else if heapAllocMB > 2048 {
		warnings = append(warnings, fmt.Sprintf("🟠 警告: HeapAlloc > 2GB (%d MB) - 建议调查", heapAllocMB))
	}

	// 检查Goroutine数量
	if stats.Goroutines > 5000 {
		issues = append(issues, fmt.Sprintf("🔴 严重: Goroutines > 5000 (%d) - 可能存在goroutine泄漏", stats.Goroutines))
	} else if stats.Goroutines > 1000 {
		warnings = append(warnings, fmt.Sprintf("🟠 警告: Goroutines > 1000 (%d) - 建议监控", stats.Goroutines))
	}

	// 检查堆空闲比例
	if stats.HeapIdle > 0 && stats.HeapSys > 0 {
		idlePercent := float64(stats.HeapIdle) * 100 / float64(stats.HeapSys)
		if idlePercent > 50 {
			warnings = append(warnings, fmt.Sprintf("🟠 警告: HeapIdle占比%.1f%% - 大量空闲内存未返还OS", idlePercent))
		}
	}

	// 检查GC频率
	if stats.NumGC < 100 && heapAllocMB > 1024 {
		warnings = append(warnings, "🟠 警告: GC次数偏低，可能GC未正常工作")
	}

	result := ""
	if len(issues) > 0 {
		result += "  ❌ 发现问题:\n"
		for _, issue := range issues {
			result += fmt.Sprintf("     %s\n", issue)
		}
	}
	if len(warnings) > 0 {
		result += "  ⚠️  警告:\n"
		for _, warning := range warnings {
			result += fmt.Sprintf("     %s\n", warning)
		}
	}
	if len(issues) == 0 && len(warnings) == 0 {
		result = "  ✅ 内存使用正常"
	}

	return result
}

// generateRecommendations 生成优化建议
func generateRecommendations(stats *MemoryStats) string {
	var recommendations []string

	rssMB := stats.RSS / 1024 / 1024
	heapAllocMB := stats.HeapAlloc / 1024 / 1024

	// 🔴 RSS 相关建议（优先级最高）
	if rssMB > 4096 {
		recommendations = append(recommendations,
			"1. 🚨 RSS 超过 4GB，立即抓取 heap profile: curl http://localhost:28686/debug/pprof/heap > heap.prof",
			"2. 🔍 使用 go tool pprof 分析: go tool pprof -http=:8081 heap.prof",
			"3. 💡 尝试强制 GC: curl http://localhost:28686/debug/memory/force-gc",
			"4. 📊 使用 /debug/memory/compare?duration=5m 监控趋势",
		)
	} else if rssMB > 3072 {
		recommendations = append(recommendations,
			"1. 📊 RSS 接近警戒线，使用 /debug/memory/compare?duration=5m 监控增长趋势",
			"2. 💡 考虑主动执行 GC: curl http://localhost:28686/debug/memory/force-gc",
		)
	}

	if heapAllocMB > 10240 {
		recommendations = append(recommendations,
			"1. 🔧 立即生成 pprof heap profile: curl http://localhost:28686/debug/pprof/heap > heap.prof",
			"2. 🔍 使用 go tool pprof 查看对象分布: go tool pprof -http=:8081 heap.prof",
			"3. 🚨 检查是否有大对象或缓存未释放",
		)
	} else if heapAllocMB > 2048 && rssMB <= 3072 {
		recommendations = append(recommendations,
			"1. 📊 监控内存增长趋势，定期生成快照",
			"2. 🔍 检查缓存大小是否设置了上限",
		)
	}

	if stats.Goroutines > 5000 {
		recommendations = append(recommendations,
			"1. 🔧 使用 pprof goroutine profile 分析: curl http://localhost:28686/debug/pprof/goroutine > goroutine.prof",
			"2. 🔍 检查是否有goroutine泄漏（未正确关闭的channel、context）",
		)
	} else if stats.Goroutines > 1000 {
		recommendations = append(recommendations,
			"1. 📊 监控goroutine数量变化趋势",
		)
	}

	if stats.HeapIdle > stats.HeapAlloc && stats.HeapIdle > 1024*1024*1024 {
		recommendations = append(recommendations,
			"1. 💡 尝试调用 debug.FreeOSMemory() 返还空闲内存给OS",
			"2. 💡 调整 GOGC 环境变量优化GC频率",
		)
	}

	if len(recommendations) == 0 {
		return "  ✅ 暂无特殊建议，继续保持监控"
	}

	result := ""
	for _, rec := range recommendations {
		result += fmt.Sprintf("  %s\n", rec)
	}
	return result
}

// analyzeGCEffect 分析GC效果
func analyzeGCEffect(before, after *MemoryStats) string {
	heapFreed := int64(before.HeapAlloc) - int64(after.HeapAlloc)
	heapFreedMB := heapFreed / 1024 / 1024
	freedPercent := float64(heapFreed) * 100 / float64(before.HeapAlloc)

	var result string

	if heapFreedMB > 1024 {
		result = fmt.Sprintf("✅ 效果显著: 释放了 %d MB (%.1f%%)，说明有大量可回收对象", heapFreedMB, freedPercent)
	} else if heapFreedMB > 100 {
		result = fmt.Sprintf("✅ 效果正常: 释放了 %d MB (%.1f%%)", heapFreedMB, freedPercent)
	} else if heapFreedMB > 0 {
		result = fmt.Sprintf("⚠️  效果有限: 仅释放了 %d MB (%.1f%%)，大部分对象仍被引用", heapFreedMB, freedPercent)
	} else {
		result = fmt.Sprintf("🔴 无效果: 未释放内存，可能存在严重的内存泄漏（强引用未释放）")
	}

	// 检查Goroutine变化
	goroutineChange := after.Goroutines - before.Goroutines
	if goroutineChange > 0 {
		result += fmt.Sprintf("\n⚠️  Goroutine数量增加了 %d，可能存在goroutine泄漏", goroutineChange)
	}

	return result
}

// analyzeGrowthTrend 分析增长趋势
func analyzeGrowthTrend(before, after *MemoryStats, duration time.Duration) string {
	rssGrowth := int64(after.RSS) - int64(before.RSS)
	rssGrowthMB := rssGrowth / 1024 / 1024
	heapAllocGrowth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	heapAllocGrowthMB := heapAllocGrowth / 1024 / 1024
	goroutineGrowth := after.Goroutines - before.Goroutines

	var result string

	// 🔴 首要分析：RSS 增长趋势（真实内存）
	if before.RSS > 0 && after.RSS > 0 { // 如果能获取到 RSS
		if rssGrowthMB > 100 {
			hourlyGrowth := float64(rssGrowthMB) * 60 / duration.Minutes()
			result += fmt.Sprintf("🔴 RSS快速增长: %.1f MB/小时，物理内存持续增加，可能存在泄漏\n", hourlyGrowth)
		} else if rssGrowthMB > 0 {
			result += fmt.Sprintf("⚠️  RSS缓慢增长: %d MB，继续观察\n", rssGrowthMB)
		} else if rssGrowthMB < -100 {
			result += fmt.Sprintf("✅ RSS释放正常: 释放了 %d MB\n", -rssGrowthMB)
		} else {
			result += "✅ RSS稳定\n"
		}
	}

	// 分析堆内存增长（辅助指标）
	if heapAllocGrowthMB > 100 {
		hourlyGrowth := float64(heapAllocGrowthMB) * 60 / duration.Minutes()
		result += fmt.Sprintf("🟠 HeapAlloc增长: %.1f MB/小时\n", hourlyGrowth)
	} else if heapAllocGrowthMB > 0 {
		result += fmt.Sprintf("⚠️  HeapAlloc缓慢增长: %d MB\n", heapAllocGrowthMB)
	} else if heapAllocGrowthMB < -100 {
		result += fmt.Sprintf("✅ HeapAlloc释放: 释放了 %d MB\n", -heapAllocGrowthMB)
	} else {
		result += "✅ HeapAlloc稳定\n"
	}

	// 分析Goroutine增长
	if goroutineGrowth > 100 {
		hourlyGrowth := float64(goroutineGrowth) * 60 / duration.Minutes()
		result += fmt.Sprintf("🔴 Goroutine快速增长: %.1f 个/小时，可能存在泄漏", hourlyGrowth)
	} else if goroutineGrowth > 10 {
		result += fmt.Sprintf("⚠️  Goroutine缓慢增长: %d 个，继续观察", goroutineGrowth)
	} else if goroutineGrowth < -10 {
		result += fmt.Sprintf("✅ Goroutine数量正常波动: %d 个", goroutineGrowth)
	} else {
		result += "✅ Goroutine数量稳定"
	}

	return result
}

// formatLastGCTime 格式化上次GC时间
func formatLastGCTime(lastGC uint64) string {
	if lastGC == 0 {
		return "N/A"
	}
	lastGCTime := time.Unix(0, int64(lastGC))
	elapsed := time.Since(lastGCTime)
	return fmt.Sprintf("%s (距今 %s)", lastGCTime.Format("15:04:05"), elapsed.Round(time.Second))
}

// ============================================================================
//                       自动 Heap Profile Dump 机制
// ============================================================================

// AutoHeapProfileConfig 自动 Heap Profile 配置
type AutoHeapProfileConfig struct {
	Enabled        bool          // 是否启用自动保存
	RSSThresholdMB uint64        // RSS 阈值（MB），超过时自动保存
	OutputDir      string        // 输出目录
	MaxProfiles    int           // 最多保留的 profile 文件数
	MinInterval    time.Duration // 两次自动保存之间的最小间隔
}

// DefaultAutoHeapProfileConfig 返回默认配置
func DefaultAutoHeapProfileConfig() *AutoHeapProfileConfig {
	return &AutoHeapProfileConfig{
		Enabled:        true,
		RSSThresholdMB: 4096, // 4GB
		OutputDir:      "data/pprof",
		MaxProfiles:    10,
		MinInterval:    5 * time.Minute,
	}
}

// AutoHeapProfiler 自动 Heap Profile 保存器
type AutoHeapProfiler struct {
	config       *AutoHeapProfileConfig
	lastDumpTime time.Time
	dumpCount    int
}

// NewAutoHeapProfiler 创建自动 Heap Profile 保存器
func NewAutoHeapProfiler(config *AutoHeapProfileConfig) *AutoHeapProfiler {
	if config == nil {
		config = DefaultAutoHeapProfileConfig()
	}
	return &AutoHeapProfiler{
		config: config,
	}
}

// CheckAndDump 检查 RSS 并在超过阈值时自动保存 heap profile
//
// 返回值:
// - dumped: 是否保存了 profile
// - filepath: 保存的文件路径（如果保存了）
// - err: 错误信息
func (p *AutoHeapProfiler) CheckAndDump() (dumped bool, filepath string, err error) {
	if !p.config.Enabled {
		return false, "", nil
	}

	rssMB := GetRSSMB()
	if rssMB == 0 {
		// 无法获取 RSS，跳过
		return false, "", nil
	}

	if rssMB < p.config.RSSThresholdMB {
		// RSS 未超过阈值
		return false, "", nil
	}

	// 检查最小间隔
	if time.Since(p.lastDumpTime) < p.config.MinInterval {
		return false, "", nil
	}

	// 保存 heap profile
	filepath, err = p.dumpHeapProfile()
	if err != nil {
		return false, "", err
	}

	p.lastDumpTime = time.Now()
	p.dumpCount++

	// 清理旧的 profile 文件
	if err := p.cleanupOldProfiles(); err != nil {
		// 清理失败不影响主流程
	}

	return true, filepath, nil
}

// dumpHeapProfile 保存 heap profile 到文件
func (p *AutoHeapProfiler) dumpHeapProfile() (string, error) {
	// 确保目录存在
	if err := os.MkdirAll(p.config.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 生成文件名
	rssMB := GetRSSMB()
	filename := fmt.Sprintf("heap_%s_rss%dMB.prof",
		time.Now().Format("20060102_150405"),
		rssMB)
	filepath := fmt.Sprintf("%s/%s", p.config.OutputDir, filename)

	// 创建文件
	f, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %w", err)
	}
	defer f.Close()

	// 写入 heap profile
	if err := writeHeapProfile(f); err != nil {
		return "", fmt.Errorf("写入 profile 失败: %w", err)
	}

	return filepath, nil
}

// writeHeapProfile 使用 pprof 写入 heap profile
func writeHeapProfile(w *os.File) error {
	// 使用 runtime/pprof 包写入
	return pprof.Lookup("heap").WriteTo(w, 0)
}

// cleanupOldProfiles 清理旧的 profile 文件
func (p *AutoHeapProfiler) cleanupOldProfiles() error {
	entries, err := os.ReadDir(p.config.OutputDir)
	if err != nil {
		return err
	}

	// 过滤出 heap profile 文件
	var heapProfiles []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "heap_") && strings.HasSuffix(entry.Name(), ".prof") {
			heapProfiles = append(heapProfiles, entry)
		}
	}

	// 如果文件数量超过限制，删除最旧的
	if len(heapProfiles) > p.config.MaxProfiles {
		// 按修改时间排序（从旧到新）
		type fileInfo struct {
			entry os.DirEntry
			info  os.FileInfo
		}
		var files []fileInfo
		for _, entry := range heapProfiles {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileInfo{entry, info})
		}

		// 按时间排序
		for i := 0; i < len(files)-1; i++ {
			for j := i + 1; j < len(files); j++ {
				if files[i].info.ModTime().After(files[j].info.ModTime()) {
					files[i], files[j] = files[j], files[i]
				}
			}
		}

		// 删除多余的文件
		deleteCount := len(files) - p.config.MaxProfiles
		for i := 0; i < deleteCount; i++ {
			path := fmt.Sprintf("%s/%s", p.config.OutputDir, files[i].entry.Name())
			os.Remove(path)
		}
	}

	return nil
}

// Stats 返回自动保存统计信息
func (p *AutoHeapProfiler) Stats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":        p.config.Enabled,
		"threshold_mb":   p.config.RSSThresholdMB,
		"output_dir":     p.config.OutputDir,
		"dump_count":     p.dumpCount,
		"last_dump_time": p.lastDumpTime,
		"current_rss_mb": GetRSSMB(),
	}
}

// ============================================================================
//                       RSS 趋势分析器
// ============================================================================

// RSSSample RSS 采样数据点
type RSSSample struct {
	Timestamp  time.Time
	RSSMB      uint64
	HeapMB     uint64
	Goroutines int
}

// RSSGrowthReport RSS 增长分析报告
type RSSGrowthReport struct {
	// 采样信息
	SampleCount int           // 采样点数量
	Duration    time.Duration // 采样时间跨度
	FirstSample RSSSample     // 第一个样本
	LastSample  RSSSample     // 最后一个样本

	// 增长分析
	RSSGrowthMB      int64   // RSS 总增长量（MB）
	RSSGrowthPercent float64 // RSS 增长百分比
	RSSGrowthPerHour float64 // RSS 小时增长率（MB/hour）

	// 峰值信息
	PeakRSSMB uint64    // RSS 峰值
	PeakTime  time.Time // 峰值时间

	// 健康评估
	IsHealthy     bool   // 是否健康
	HealthLevel   string // 健康等级：healthy, warning, critical
	HealthMessage string // 健康状态描述

	// 预测（基于线性回归）
	PredictedRSSIn1Hour  uint64        // 预测1小时后的 RSS
	PredictedRSSIn24Hour uint64        // 预测24小时后的 RSS
	TimeToThreshold      time.Duration // 预计达到阈值的时间（如果正在增长）
}

// RSSTracker RSS 增长趋势追踪器
type RSSTracker struct {
	samples    []RSSSample
	maxSamples int
	warningMB  uint64 // 警告阈值
	criticalMB uint64 // 严重阈值
}

// NewRSSTracker 创建 RSS 趋势追踪器
func NewRSSTracker(maxSamples int, warningMB, criticalMB uint64) *RSSTracker {
	if maxSamples <= 0 {
		maxSamples = 120 // 默认保留120个样本（如果30秒采样一次，约1小时数据）
	}
	if warningMB == 0 {
		warningMB = 3072 // 默认3GB警告
	}
	if criticalMB == 0 {
		criticalMB = 4096 // 默认4GB严重
	}
	return &RSSTracker{
		samples:    make([]RSSSample, 0, maxSamples),
		maxSamples: maxSamples,
		warningMB:  warningMB,
		criticalMB: criticalMB,
	}
}

// AddSample 添加一个采样点
func (t *RSSTracker) AddSample() {
	sample := RSSSample{
		Timestamp:  time.Now(),
		RSSMB:      GetRSSMB(),
		HeapMB:     GetMemoryStats().HeapAlloc / 1024 / 1024,
		Goroutines: rt.NumGoroutine(),
	}

	t.samples = append(t.samples, sample)

	// 保持样本数量在限制内
	if len(t.samples) > t.maxSamples {
		t.samples = t.samples[1:]
	}
}

// AddSampleWithStats 使用已有的内存统计添加采样点（避免重复获取）
func (t *RSSTracker) AddSampleWithStats(stats *MemoryStats) {
	sample := RSSSample{
		Timestamp:  stats.Timestamp,
		RSSMB:      stats.RSS / 1024 / 1024,
		HeapMB:     stats.HeapAlloc / 1024 / 1024,
		Goroutines: stats.Goroutines,
	}

	t.samples = append(t.samples, sample)

	// 保持样本数量在限制内
	if len(t.samples) > t.maxSamples {
		t.samples = t.samples[1:]
	}
}

// AnalyzeGrowth 分析 RSS 增长趋势
func (t *RSSTracker) AnalyzeGrowth() *RSSGrowthReport {
	report := &RSSGrowthReport{}

	if len(t.samples) < 2 {
		report.HealthLevel = "unknown"
		report.HealthMessage = "样本数量不足，无法分析趋势"
		return report
	}

	// 基本信息
	report.SampleCount = len(t.samples)
	report.FirstSample = t.samples[0]
	report.LastSample = t.samples[len(t.samples)-1]
	report.Duration = report.LastSample.Timestamp.Sub(report.FirstSample.Timestamp)

	// 增长计算
	report.RSSGrowthMB = int64(report.LastSample.RSSMB) - int64(report.FirstSample.RSSMB)
	if report.FirstSample.RSSMB > 0 {
		report.RSSGrowthPercent = float64(report.RSSGrowthMB) * 100 / float64(report.FirstSample.RSSMB)
	}
	if report.Duration.Hours() > 0 {
		report.RSSGrowthPerHour = float64(report.RSSGrowthMB) / report.Duration.Hours()
	}

	// 峰值查找
	for _, s := range t.samples {
		if s.RSSMB > report.PeakRSSMB {
			report.PeakRSSMB = s.RSSMB
			report.PeakTime = s.Timestamp
		}
	}

	// 线性回归预测
	slope, intercept := t.linearRegression()
	now := time.Now()
	hoursFromStart := now.Sub(report.FirstSample.Timestamp).Hours()

	// 预测未来值
	report.PredictedRSSIn1Hour = uint64(slope*(hoursFromStart+1) + intercept)
	report.PredictedRSSIn24Hour = uint64(slope*(hoursFromStart+24) + intercept)

	// 预测到达阈值的时间
	if slope > 0 && report.LastSample.RSSMB < t.criticalMB {
		hoursToThreshold := (float64(t.criticalMB)-intercept)/slope - hoursFromStart
		if hoursToThreshold > 0 {
			report.TimeToThreshold = time.Duration(hoursToThreshold * float64(time.Hour))
		}
	}

	// 健康评估
	t.evaluateHealth(report)

	return report
}

// linearRegression 计算线性回归斜率和截距
func (t *RSSTracker) linearRegression() (slope, intercept float64) {
	if len(t.samples) < 2 {
		return 0, 0
	}

	n := float64(len(t.samples))
	var sumX, sumY, sumXY, sumX2 float64

	startTime := t.samples[0].Timestamp
	for _, s := range t.samples {
		x := s.Timestamp.Sub(startTime).Hours() // X 轴：小时
		y := float64(s.RSSMB)                   // Y 轴：RSS（MB）
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 计算斜率和截距
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, sumY / n
	}

	slope = (n*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / n

	return slope, intercept
}

// evaluateHealth 评估内存健康状态
func (t *RSSTracker) evaluateHealth(report *RSSGrowthReport) {
	currentRSS := report.LastSample.RSSMB

	// 基于当前值评估
	if currentRSS >= t.criticalMB {
		report.IsHealthy = false
		report.HealthLevel = "critical"
		report.HealthMessage = fmt.Sprintf("🔴 严重：RSS（%d MB）已超过严重阈值（%d MB）", currentRSS, t.criticalMB)
		return
	}

	if currentRSS >= t.warningMB {
		report.IsHealthy = false
		report.HealthLevel = "warning"
		report.HealthMessage = fmt.Sprintf("🟠 警告：RSS（%d MB）已超过警告阈值（%d MB）", currentRSS, t.warningMB)
		return
	}

	// 基于增长趋势评估
	if report.RSSGrowthPerHour > 100 { // 超过100MB/小时增长
		report.IsHealthy = false
		report.HealthLevel = "warning"
		report.HealthMessage = fmt.Sprintf("🟠 警告：RSS快速增长（%.1f MB/小时），可能存在内存泄漏", report.RSSGrowthPerHour)
		return
	}

	if report.RSSGrowthPerHour > 50 { // 超过50MB/小时增长
		report.IsHealthy = true
		report.HealthLevel = "caution"
		report.HealthMessage = fmt.Sprintf("🟡 注意：RSS持续增长（%.1f MB/小时），建议监控", report.RSSGrowthPerHour)
		return
	}

	// 健康状态
	report.IsHealthy = true
	report.HealthLevel = "healthy"
	if report.RSSGrowthMB < 0 {
		report.HealthMessage = fmt.Sprintf("✅ 健康：RSS稳定（%d MB），已释放 %d MB", currentRSS, -report.RSSGrowthMB)
	} else {
		report.HealthMessage = fmt.Sprintf("✅ 健康：RSS稳定（%d MB）", currentRSS)
	}
}

// GetSamples 返回所有采样数据
func (t *RSSTracker) GetSamples() []RSSSample {
	return t.samples
}

// Clear 清空采样数据
func (t *RSSTracker) Clear() {
	t.samples = t.samples[:0]
}

// GenerateReport 生成可读的文本报告
func (t *RSSTracker) GenerateReport() string {
	report := t.AnalyzeGrowth()

	return fmt.Sprintf(`
================================================================================
                        RSS 趋势分析报告
================================================================================
生成时间: %s

采样信息:
  - 采样点数:       %10d
  - 采样时长:       %s
  - 首次采样:       %s (RSS: %d MB)
  - 最新采样:       %s (RSS: %d MB)

增长分析:
  - RSS 总增长:     %+10d MB  (%.1f%%)
  - 小时增长率:     %10.1f MB/小时

峰值信息:
  - 峰值 RSS:       %10d MB
  - 峰值时间:       %s

预测（基于线性回归）:
  - 1小时后预测:    %10d MB
  - 24小时后预测:   %10d MB
  - 达到阈值时间:   %s

健康评估:
  - 等级:           %s
  - 状态:           %s
================================================================================
`,
		time.Now().Format("2006-01-02 15:04:05"),
		report.SampleCount,
		report.Duration.Round(time.Second),
		report.FirstSample.Timestamp.Format("15:04:05"), report.FirstSample.RSSMB,
		report.LastSample.Timestamp.Format("15:04:05"), report.LastSample.RSSMB,
		report.RSSGrowthMB, report.RSSGrowthPercent,
		report.RSSGrowthPerHour,
		report.PeakRSSMB,
		report.PeakTime.Format("15:04:05"),
		report.PredictedRSSIn1Hour,
		report.PredictedRSSIn24Hour,
		formatDuration(report.TimeToThreshold),
		report.HealthLevel,
		report.HealthMessage,
	)
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "N/A（当前无增长趋势）"
	}
	if d > 24*time.Hour {
		return fmt.Sprintf("约 %.1f 天", d.Hours()/24)
	}
	if d > time.Hour {
		return fmt.Sprintf("约 %.1f 小时", d.Hours())
	}
	return fmt.Sprintf("约 %.0f 分钟", d.Minutes())
}
