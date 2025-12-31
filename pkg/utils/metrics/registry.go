// Package metrics 提供统一的内存监控指标注册和收集工具
//
// 📋 **内存监控工具层 (Memory Metrics Utility Layer)**
//
// 本包提供全局的内存上报器注册和收集功能，供所有模块使用。
// 遵循架构约束：internal/core/* 模块通过此工具包实现跨组件协作。
//
// 🎯 **设计原则**：
// - 全局注册器：单机进程全局的内存上报器注册表
// - 线程安全：使用读写锁保护并发访问
// - 架构约束：internal/core/* 模块通过此工具包协作，不直接相互调用
//
package metrics

import (
	"sync"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
)

var (
	// mu 保护 reporters 切片的读写锁
	mu sync.RWMutex

	// reporters 全局注册的内存上报器列表（单机进程全局）
	reporters []metrics.MemoryReporter

	// memoryMonitoringMode 全局内存监控模式（由 MemoryDoctor 设置）
	memoryMonitoringMode string
	modeMu              sync.RWMutex
)

// RegisterMemoryReporter 注册一个内存上报器
//
// 参数：
//   - r: 实现了 MemoryReporter 接口的模块实例
//
// 说明：
//   - 线程安全：使用读写锁保护
//   - 建议在模块的 fx module.go 中，实例化完主要服务后调用
//   - 可以多次调用注册多个模块
//   - 如果 r 为 nil，则忽略
func RegisterMemoryReporter(r metrics.MemoryReporter) {
	if r == nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	reporters = append(reporters, r)
}

// ForEachReporter 遍历所有已注册的 MemoryReporter
//
// 仅供内部基础设施（如 MemoryDoctor）使用，用于在检测到内存/缓存压力时
// 对特定模块执行诸如 ShrinkCache 等自救操作。
func ForEachReporter(fn func(metrics.MemoryReporter)) {
	if fn == nil {
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	for _, r := range reporters {
		fn(r)
	}
}

// CollectAllModuleStats 收集所有已注册模块的内存统计信息
//
// 返回：
//   - []ModuleMemoryStats: 所有模块的内存统计信息切片
//
// 说明：
//   - 线程安全：使用读锁保护
//   - 返回的切片顺序与注册顺序一致
//   - 如果某个模块的 CollectMemoryStats() 发生 panic，不会影响其他模块
func CollectAllModuleStats() []metrics.ModuleMemoryStats {
	mu.RLock()
	defer mu.RUnlock()

	stats := make([]metrics.ModuleMemoryStats, 0, len(reporters))
	for _, r := range reporters {
		// 捕获 panic，避免单个模块的错误影响整体收集
		func() {
			defer func() {
				if r := recover(); r != nil {
					// 如果发生 panic，跳过该模块
					// 在实际使用中，可以通过日志记录错误
				}
			}()
			stats = append(stats, r.CollectMemoryStats())
		}()
	}

	return stats
}

// GetRegisteredReportersCount 返回已注册的上报器数量（用于调试和监控）
func GetRegisteredReportersCount() int {
	mu.RLock()
	defer mu.RUnlock()
	return len(reporters)
}

// ClearAllMemoryReporters 清空所有已注册的上报器（主要用于测试）
func ClearAllMemoryReporters() {
	mu.Lock()
	defer mu.Unlock()
	reporters = nil
}

// SetMemoryMonitoringMode 设置全局内存监控模式（由 MemoryDoctor 调用）
//
// 参数：
//   - mode: 监控模式（"minimal" / "heuristic" / "accurate"）
//
// 说明：
//   - 线程安全：使用读写锁保护
//   - 各模块的 CollectMemoryStats() 可以通过 GetMemoryMonitoringMode() 查询当前模式
func SetMemoryMonitoringMode(mode string) {
	modeMu.Lock()
	defer modeMu.Unlock()
	memoryMonitoringMode = mode
}

// GetMemoryMonitoringMode 获取当前内存监控模式
//
// 返回：
//   - string: 监控模式（"minimal" / "heuristic" / "accurate"），如果未设置则返回 "heuristic"
//
// 说明：
//   - 线程安全：使用读锁保护
//   - 各模块可以在 CollectMemoryStats() 中调用此函数，根据模式决定是否计算 ApproxBytes
func GetMemoryMonitoringMode() string {
	modeMu.RLock()
	defer modeMu.RUnlock()
	if memoryMonitoringMode == "" {
		return "heuristic" // 默认值
	}
	return memoryMonitoringMode
}

