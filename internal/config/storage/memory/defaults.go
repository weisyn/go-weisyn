package memory

import "time"

// 内存存储默认配置值
// 这些默认值基于内存缓存的最佳实践
const (
	// === 基础配置 ===

	// defaultMaxMemory 默认最大内存使用量为256MB
	// 原因：256MB适合大多数应用场景，平衡性能和内存占用
	defaultMaxMemory = 256 << 20 // 256MB

	// defaultMaxEntries 默认最大条目数为100000
	// 原因：10万条目能满足大多数缓存需求
	defaultMaxEntries = 100000

	// defaultDefaultTTL 默认TTL为1小时
	// 原因：1小时平衡了缓存命中率和数据新鲜度
	defaultDefaultTTL = time.Hour

	// === 清理配置 ===

	// defaultCleanupInterval 默认清理间隔为10分钟
	// 原因：10分钟间隔及时清理过期数据，不会过于频繁
	defaultCleanupInterval = 10 * time.Minute
)
