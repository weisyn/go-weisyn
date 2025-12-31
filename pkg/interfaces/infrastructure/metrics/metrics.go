// Package metrics 提供统一的内存监控指标接口定义
//
// 📋 **内存监控接口层 (Memory Metrics Interface Layer)**
//
// 本包定义了 WES 区块链系统的统一内存监控接口，供所有核心模块实现。
// 接口定义遵循架构约束：internal/core/* 模块通过此接口实现跨组件协作。
//
// 🎯 **设计原则**：
// - 接口定义与实现分离：接口在此定义，实现在 internal/core/infrastructure/metrics
// - 跨模块协作：所有 internal/core/* 模块通过实现 MemoryReporter 接口上报内存状态
// - 架构约束：internal/core/* 模块不得直接调用其他 internal/core/* 模块
//
// 📦 **使用方式**：
// 1. 模块实现 MemoryReporter 接口
// 2. 通过 pkg/utils/metrics.RegisterMemoryReporter(...) 注册
// 3. 通过 pkg/utils/metrics.CollectAllModuleStats() 收集所有模块的内存统计
//
package metrics

// ModuleMemoryStats 模块"自己认账"的逻辑内存状态
//
// 每个模块通过实现 MemoryReporter 接口，自行上报其内存使用情况。
// 不追求绝对精确，关键是能反映内存使用的趋势和相对大小。
type ModuleMemoryStats struct {
	Module      string `json:"module"`       // 模块名称：mempool.txpool / consensus.pow / block.manager ...
	Layer       string `json:"layer"`        // 架构层级：L3-Coordination / L4-CoreBusiness / L2-Infrastructure 等
	Objects     int64  `json:"objects"`      // 主要对象数：tx 数量 / block 数量 / 连接数 ...
	ApproxBytes int64  `json:"approx_bytes"` // 模块自己估算 bytes（不追求绝对精确，关键是趋势）
	CacheItems  int64  `json:"cache_items"`  // 缓存条目（如 block cache、UTXO cache）
	QueueLength int64  `json:"queue_length"` // 队列 / channel / pending 列表长度
}

// MemoryReporter 每个核心模块需要实现的内存上报接口
//
// 实现此接口的模块需要：
// 1. 返回模块名称（用于标识）
// 2. 实现 CollectMemoryStats() 方法，返回当前模块的内存统计
//
// 注意：此接口定义在 pkg/interfaces/infrastructure/metrics，供所有 internal/core/* 模块实现。
// 实现此接口的模块应通过 pkg/utils/metrics.RegisterMemoryReporter() 注册。
type MemoryReporter interface {
	// ModuleName 返回模块名称
	ModuleName() string

	// CollectMemoryStats 收集当前模块的内存统计信息
	CollectMemoryStats() ModuleMemoryStats
}

