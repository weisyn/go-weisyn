package diagnostics

import (
	"fmt"
	"net/http"
	"runtime"
	"time"
)

// HandleMemoryProfile HTTP 处理器：返回详细的内存分析报告
func HandleMemoryProfile(w http.ResponseWriter, r *http.Request) {
	profile := MemoryProfile()
	
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(profile))
}

// HandleForceGC HTTP 处理器：强制GC并返回效果报告
func HandleForceGC(w http.ResponseWriter, r *http.Request) {
	_, _, report := ForceGCAndReport()
	
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(report))
}

// HandleMemoryCompare HTTP 处理器：监控一段时间的内存变化
func HandleMemoryCompare(w http.ResponseWriter, r *http.Request) {
	// 解析时间间隔参数（默认30秒）
	durationStr := r.URL.Query().Get("duration")
	duration := 30 * time.Second
	if durationStr != "" {
		if d, err := time.ParseDuration(durationStr); err == nil {
			duration = d
		}
	}

	// 第一次快照
	before := GetMemoryStats()
	
	// 等待指定时间
	time.Sleep(duration)
	
	// 第二次快照
	after := GetMemoryStats()
	
	// 生成对比报告
	report := CompareMemoryStats(before, after)
	
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(report))
}

// HandleMemoryJSON HTTP 处理器：返回JSON格式的内存统计
func HandleMemoryJSON(w http.ResponseWriter, r *http.Request) {
	stats := GetMemoryStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{
	"timestamp": "%s",
	"rss_mb": %d,
	"heap_alloc_mb": %d,
	"heap_sys_mb": %d,
	"heap_idle_mb": %d,
	"heap_inuse_mb": %d,
	"heap_objects": %d,
	"sys_mb": %d,
	"total_alloc_mb": %d,
	"stack_sys_mb": %d,
	"num_gc": %d,
	"next_gc_mb": %d,
	"goroutines": %d,
	"gc_cpu_fraction": %.6f
}`,
		stats.Timestamp.Format(time.RFC3339),
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
		stats.Goroutines,
		getGCCPUFraction(),
	)
}

// getGCCPUFraction 获取GC CPU占用比例
func getGCCPUFraction() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.GCCPUFraction
}

// HandleRSSTrend HTTP 处理器：返回 RSS 趋势分析报告
// 注意：需要通过 RegisterMemoryHandlersWithGuard 注册才能使用
func HandleRSSTrend(tracker *RSSTracker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if tracker == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "RSS 趋势追踪器未初始化")
			return
		}
		report := tracker.GenerateReport()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(report))
	}
}

// HandleMemoryGuardStatus HTTP 处理器：返回 MemoryGuard 状态报告
func HandleMemoryGuardStatus(guard *MemoryGuard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if guard == nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "MemoryGuard 未初始化")
			return
		}
		report := guard.GenerateReport()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(report))
	}
}

// HandleMemoryMitigate HTTP 处理器：主动触发内存缓解
func HandleMemoryMitigate(w http.ResponseWriter, r *http.Request) {
	aggressive := r.URL.Query().Get("aggressive") == "true"
	beforeMB, afterMB := MitigateMemoryPressure(aggressive)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `
================================================================================
                        内存缓解执行报告
================================================================================
执行时间: %s
缓解模式: %s

执行前 RSS: %d MB
执行后 RSS: %d MB
释放内存:   %+d MB

说明:
- 普通模式: 执行 GC
- 强力模式: 执行 GC + 返还内存给 OS

使用方法:
- 普通模式: curl http://localhost:28686/debug/memory/mitigate
- 强力模式: curl http://localhost:28686/debug/memory/mitigate?aggressive=true
================================================================================
`,
		time.Now().Format("2006-01-02 15:04:05"),
		map[bool]string{true: "强力模式", false: "普通模式"}[aggressive],
		beforeMB,
		afterMB,
		int64(afterMB)-int64(beforeMB),
	)
}

// RegisterMemoryHandlers 注册所有内存诊断处理器到给定的 ServeMux
func RegisterMemoryHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/debug/memory/profile", HandleMemoryProfile)
	mux.HandleFunc("/debug/memory/force-gc", HandleForceGC)
	mux.HandleFunc("/debug/memory/compare", HandleMemoryCompare)
	mux.HandleFunc("/debug/memory/json", HandleMemoryJSON)
	mux.HandleFunc("/debug/memory/mitigate", HandleMemoryMitigate)

	// 添加帮助端点
	mux.HandleFunc("/debug/memory/help", handleMemoryHelp)
}

// RegisterMemoryHandlersWithGuard 注册所有内存诊断处理器，包括 MemoryGuard 相关端点
func RegisterMemoryHandlersWithGuard(mux *http.ServeMux, guard *MemoryGuard) {
	// 基础端点
	mux.HandleFunc("/debug/memory/profile", HandleMemoryProfile)
	mux.HandleFunc("/debug/memory/force-gc", HandleForceGC)
	mux.HandleFunc("/debug/memory/compare", HandleMemoryCompare)
	mux.HandleFunc("/debug/memory/json", HandleMemoryJSON)
	mux.HandleFunc("/debug/memory/mitigate", HandleMemoryMitigate)

	// MemoryGuard 相关端点
	if guard != nil {
		mux.HandleFunc("/debug/memory/guard", HandleMemoryGuardStatus(guard))
		mux.HandleFunc("/debug/memory/rss-trend", HandleRSSTrend(guard.GetRSSTracker()))
	}

	// 添加帮助端点
	mux.HandleFunc("/debug/memory/help", handleMemoryHelp)
}

// handleMemoryHelp 返回帮助信息
func handleMemoryHelp(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `
内存诊断端点使用说明:
======================

基础端点:
---------

1. /debug/memory/profile
   - 返回详细的内存分析报告（文本格式）
   - 包括 RSS、堆内存、GC 统计、Goroutine 数量等
   - 示例: curl http://localhost:28686/debug/memory/profile

2. /debug/memory/json
   - 返回 JSON 格式的内存统计（方便程序解析）
   - 🆕 包含 RSS 字段
   - 示例: curl http://localhost:28686/debug/memory/json

3. /debug/memory/force-gc
   - 强制执行 GC 并返回效果报告
   - 显示 GC 前后的内存变化（包括 RSS）
   - 示例: curl http://localhost:28686/debug/memory/force-gc

4. /debug/memory/compare?duration=30s
   - 监控指定时间段内的内存变化
   - 🆕 包含 RSS 趋势分析
   - 示例: curl "http://localhost:28686/debug/memory/compare?duration=1m"

5. /debug/memory/mitigate[?aggressive=true]
   - 🆕 主动触发内存缓解
   - 普通模式: 执行 GC
   - 强力模式: 执行 GC + 返还内存给 OS
   - 示例: curl http://localhost:28686/debug/memory/mitigate?aggressive=true

MemoryGuard 端点（需启用 MemoryGuard）:
---------------------------------------

6. /debug/memory/guard
   - 🆕 返回 MemoryGuard 状态报告
   - 包括运行状态、触发统计、健康评估等
   - 示例: curl http://localhost:28686/debug/memory/guard

7. /debug/memory/rss-trend
   - 🆕 返回 RSS 趋势分析报告
   - 包括增长率、预测值、到达阈值时间等
   - 示例: curl http://localhost:28686/debug/memory/rss-trend

pprof 端点:
-----------

8. /debug/pprof/heap
   - 生成 heap profile（可用 go tool pprof 分析）
   - 示例: curl http://localhost:28686/debug/pprof/heap > heap.prof
   - 分析: go tool pprof -http=:8081 heap.prof

9. /debug/pprof/goroutine
   - 生成 goroutine profile
   - 示例: curl http://localhost:28686/debug/pprof/goroutine > goroutine.prof
   - 分析: go tool pprof -http=:8081 goroutine.prof

故障排查建议:
==============

如果怀疑内存泄漏:
1. 先查看 /debug/memory/profile 确认当前状态
2. 检查 /debug/memory/guard 查看 MemoryGuard 触发情况
3. 使用 /debug/memory/rss-trend 查看 RSS 增长趋势
4. 执行 /debug/memory/mitigate?aggressive=true 尝试缓解
5. 如果确认泄漏，生成 heap profile 深入分析

MemoryGuard 自动保护机制:
=========================

MemoryGuard 会自动监控内存使用并采取保护措施：
- 软限制（默认 3GB）: 触发 GC
- 硬限制（默认 4GB）: 清理缓存 + 强力 GC + 自动保存 heap profile

配置选项（config.json）:
{
  "memory_monitoring": {
    "memory_guard": {
      "enabled": true,
      "soft_limit_mb": 3072,
      "hard_limit_mb": 4096,
      "auto_profile": true,
      "profile_output_dir": "data/pprof",
      "check_interval_seconds": 30
    }
  }
}

`)
}

