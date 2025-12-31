// Package txpool provides default configuration values for transaction pool.
package txpool

import "time"

// 交易池配置默认值
// 这些默认值基于区块链交易池管理的最佳实践和性能考虑
const (
	// === 基础池配置 ===

	// defaultMaxSize 默认交易池最大容量设为10000
	// 原因：10000个交易能满足大多数网络的交易负载
	// 平衡内存使用和交易处理能力，避免交易池过满影响性能
	defaultMaxSize = 10000

	// defaultPriceLimit 默认最低交易价格设为1
	// 原因：设置最低价格防止垃圾交易和拒绝服务攻击
	// 1单位的最低价格是合理的门槛，不会阻碍正常交易
	defaultPriceLimit = 1

	// defaultPriceBump 默认价格提升百分比设为10%
	// 原因：10%的价格提升确保替换交易有足够的经济激励
	// 防止频繁低价替换，保护网络免受垃圾交易干扰
	defaultPriceBump = 10

	// === 生存周期配置 ===

	// defaultLifetime 默认交易生存时间设为1小时
	// 原因：1小时足够交易被打包，过期交易应被清理以释放空间
	// 平衡交易有效性和池空间管理
	defaultLifetime = 1 * time.Hour

	// defaultKeepLocals 默认保留本地交易
	// 原因：本地交易通常是用户发起的重要交易，应优先保留
	// 保护用户交易不被网络拥塞时清理
	defaultKeepLocals = true

	// === 性能和监控配置 ===

	// defaultMetricsEnabled 默认启用性能指标收集
	// 原因：性能指标对于交易池优化和问题诊断很重要
	// 监控池的健康状态和处理性能
	defaultMetricsEnabled = true

	// defaultMetricsInterval 默认指标收集间隔设为30秒
	// 原因：30秒间隔提供足够的监控精度
	// 平衡监控详细度和系统开销
	defaultMetricsInterval = 30 * time.Second

	// === 挖矿配置 ===

	// defaultMaxTransactionsForMining 默认挖矿时最大交易数量设为1000
	// 原因：1000个交易能够充分利用区块空间，同时保证打包效率
	// 平衡区块大小和处理性能，避免区块过于臃肿
	defaultMaxTransactionsForMining = 1000

	// defaultMaxBlockSizeForMining 默认挖矿时区块大小限制设为1MB
	// 原因：1MB区块大小是经过验证的合理大小，平衡网络传输和容量
	// 兼容主流区块链网络的区块大小限制，确保网络兼容性
	defaultMaxBlockSizeForMining = 1024 * 1024 // 1MB

	// === 资源限制配置 ===

	// defaultMemoryLimit 默认内存限制设为512MB
	// 原因：512MB能存储大量交易，同时控制内存使用
	// 防止交易池消耗过多系统内存
	defaultMemoryLimit = 512 * 1024 * 1024

	// defaultMaxTxSize 默认最大交易大小设为128KB
	// 原因：128KB能容纳复杂的智能合约交易
	// 控制单个交易的大小，防止超大交易影响网络
	defaultMaxTxSize = 128 * 1024
)

// 默认交易排序策略
var defaultSortingStrategies = []string{
	"执行费用_price",   // 按执行费用价格排序
	"fee_per_执行费用", // 按单位执行费用费用排序
	"total_fee",    // 按总费用排序
	"arrival_time", // 按到达时间排序
}

// 默认验证级别配置
var defaultValidationLevels = map[string]bool{
	"signature_verification":    true, // 签名验证
	"nonce_verification":        true, // Nonce验证
	"balance_verification":      true, // 余额验证
	"执行费用_limit_verification":   true, // 执行费用限制验证
	"smart_contract_validation": true, // 智能合约验证
}

// 默认性能阈值配置
var defaultPerformanceThresholds = map[string]interface{}{
	"max_insertion_time_ms":  100,  // 最大插入时间(毫秒)
	"max_removal_time_ms":    50,   // 最大移除时间(毫秒)
	"max_query_time_ms":      10,   // 最大查询时间(毫秒)
	"memory_usage_warning":   0.8,  // 内存使用警告阈值
	"memory_usage_critical":  0.95, // 内存使用严重阈值
	"pool_fullness_warning":  0.8,  // 池满度警告阈值
	"pool_fullness_critical": 0.95, // 池满度严重阈值
}

// 默认费用策略配置
var defaultFeeStrategies = map[string]interface{}{
	"min_tip_cap":         1,       // 最小小费上限
	"max_fee_cap":         1000000, // 最大费用上限
	"base_fee_multiplier": 1.5,     // 基础费用倍数
	"priority_fee_ratio":  0.1,     // 优先费用比例
}
