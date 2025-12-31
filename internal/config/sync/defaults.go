// Package sync provides default configuration values for blockchain synchronization.
package sync

import "time"

// 同步配置默认值
const (
	// === 基础同步配置 ===

	// defaultSyncMode 默认同步模式为"fast"
	// 原因：快速同步模式平衡了同步速度和资源消耗
	defaultSyncMode = "fast"

	// defaultEnabled 默认启用同步功能
	// 原因：区块链节点必须与网络保持同步才能正常运行
	defaultEnabled = true

	// === 并发和性能配置 ===

	// defaultConcurrency 默认同步并发数设为10
	// 原因：10个并发任务能有效利用网络带宽和处理能力
	defaultConcurrency = 10

	// defaultSnapshotConcurrency 默认快照下载并发数设为5
	// 原因：快照下载通常是大文件传输，5个并发足够且不会过载网络
	defaultSnapshotConcurrency = 5

	// defaultMaxBatchSize 默认最大批处理大小设为100
	// 原因：100个区块的批次在网络传输和处理效率之间取得平衡
	defaultMaxBatchSize = 100

	// === 超时和重试配置 ===

	// defaultSyncTimeout 默认同步超时时间设为120秒
	// 原因：120秒足够处理网络延迟和大区块的同步
	defaultSyncTimeout = 120 * time.Second

	// defaultRequestTimeout 默认请求超时时间设为30秒
	// 原因：30秒适合单个请求的超时，覆盖网络延迟和处理时间
	defaultRequestTimeout = 30 * time.Second

	// defaultRetryAttempts 默认重试次数设为3
	// 原因：3次重试能应对大多数临时网络故障
	defaultRetryAttempts = 3

	// defaultRetryDelay 默认重试延迟设为5秒
	// 原因：5秒延迟避免立即重试导致的重复失败
	defaultRetryDelay = 5 * time.Second

	// === 区块获取配置 ===

	// defaultMaxBlockFetch 默认最大同步区块数设为256
	// 原因：256个区块是合理的批次大小，不会造成过大的内存压力
	defaultMaxBlockFetch = 256

	// defaultMaxHeaderFetch 默认最大头部获取数设为512
	// 原因：区块头部比完整区块小得多，可以获取更多数量
	defaultMaxHeaderFetch = 512

	// defaultMaxStateFetch 默认最大状态获取数设为64
	// 原因：状态数据通常较大，64个状态节点是合理的批次
	defaultMaxStateFetch = 64

	// === 节点和网络配置 ===

	// defaultMinPeers 默认最小对等节点数设为5
	// 原因：5个节点提供足够的同步源和冗余
	defaultMinPeers = 5

	// defaultMaxPeers 默认最大对等节点数设为50
	// 原因：50个节点提供良好的同步性能，不会过度消耗资源
	defaultMaxPeers = 50

	// defaultPeerTimeout 默认节点超时时间设为60秒
	// 原因：60秒足够检测节点是否响应
	defaultPeerTimeout = 60 * time.Second

	// === 轻客户端配置 ===

	// defaultLightConfirmations 默认轻客户端确认数设为64
	// 原因：64个确认提供足够的安全性
	defaultLightConfirmations = 64

	// defaultEnableLightMode 默认禁用轻模式
	// 原因：完整节点提供更好的安全性和功能
	defaultEnableLightMode = false

	// === 检查点和进度配置 ===

	// defaultCheckpointInterval 默认检查点间隔设为1000个区块
	// 原因：1000个区块的间隔平衡了检查点频率和存储开销
	defaultCheckpointInterval = 1000

	// defaultProgressReportInterval 默认进度报告间隔设为10秒
	// 原因：10秒间隔提供及时的同步进度反馈
	defaultProgressReportInterval = 10 * time.Second

	// defaultCompletionThreshold 默认同步完成阈值设为98%
	// 原因：98%的阈值认为同步基本完成，允许小幅差异
	defaultCompletionThreshold = 98

	// === 优化和高级配置 ===

	// defaultEnableForceResync 默认禁用强制重新同步
	// 原因：强制重新同步是昂贵的操作，只在必要时使用
	defaultEnableForceResync = false

	// defaultEnablePeerFilter 默认启用节点过滤
	// 原因：过滤低质量节点提高同步效率
	defaultEnablePeerFilter = true

	// defaultEnableStateSync 默认启用状态同步
	// 原因：状态同步是快速同步的重要组成部分
	defaultEnableStateSync = true

	// defaultEnableSnapshotSync 默认启用快照同步
	// 原因：快照同步能极大加速初始同步过程
	defaultEnableSnapshotSync = true

	// === 缓存和存储配置 ===

	// defaultBlockCacheSize 默认区块缓存大小设为100MB
	// 原因：100MB缓存能存储大量区块，减少重复获取
	defaultBlockCacheSize = 100 * 1024 * 1024

	// defaultStateCacheSize 默认状态缓存大小设为50MB
	// 原因：状态缓存提高状态查询性能
	defaultStateCacheSize = 50 * 1024 * 1024

	// defaultTempDirSize 默认临时目录大小限制设为1GB
	// 原因：同步过程可能需要大量临时存储
	defaultTempDirSize = 1024 * 1024 * 1024

	// === 验证和安全配置 ===

	// defaultEnableFullValidation 默认启用完整验证
	// 原因：完整验证确保区块链数据的完整性和正确性
	defaultEnableFullValidation = true

	// defaultSkipVerification 默认不跳过验证
	// 原因：跳过验证虽然能加速同步，但会降低安全性
	defaultSkipVerification = false

	// defaultTrustedHeight 默认可信高度设为0（包含在默认配置中）
	// 原因：从创世块开始同步提供最高安全性
	defaultTrustedHeight = 0

	// === 资源管理配置 ===

	// defaultMemoryPressureThreshold 默认内存压力阈值设为500MB
	// 原因：500MB是合理的内存压力阈值，超过此值将触发同步优化
	defaultMemoryPressureThreshold = 500 * 1024 * 1024 // 500MB
)

// 默认支持的同步模式
var defaultSyncModes = []string{
	"full",     // 完整同步模式
	"fast",     // 快速同步模式
	"light",    // 轻量同步模式
	"snapshot", // 快照同步模式
}

// 默认节点过滤条件
var defaultPeerFilterCriteria = map[string]interface{}{
	"min_version":        "1.0.0", // 最小版本要求
	"max_latency_ms":     1000,    // 最大延迟(毫秒)
	"min_bandwidth_kbps": 100,     // 最小带宽(KB/s)
	"blacklist_enabled":  true,    // 启用黑名单
	"whitelist_enabled":  false,   // 禁用白名单
}

// 默认验证级别配置
var defaultValidationLevels = map[string]bool{
	"signature_verification":    true, // 签名验证
	"merkle_proof_verification": true, // 默克尔证明验证
	"state_root_verification":   true, // 状态根验证
	"transaction_execution":     true, // 交易执行验证
	"consensus_rules":           true, // 共识规则验证
}
