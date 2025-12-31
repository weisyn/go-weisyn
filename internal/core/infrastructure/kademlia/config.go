package kbucket

import (
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/kademlia"
)

// ProvideKBucketConfig 提供K桶配置
// 遵循项目架构规范，配置集中管理而非组件内部管理 [[memory:4989457]]
func ProvideKBucketConfig() kademlia.KBucketConfig {
	return GetDefaultKBucketConfig()
}

// GetDefaultKBucketConfig 获取默认配置
func GetDefaultKBucketConfig() kademlia.KBucketConfig {
	return &defaultKBucketConfig{}
}

// defaultKBucketConfig 默认K桶配置实现
type defaultKBucketConfig struct{}

// GetBucketSize 返回桶大小
func (c *defaultKBucketConfig) GetBucketSize() int {
	return 20 // Kademlia标准桶大小
}

// GetMaxLatency 返回最大延迟
func (c *defaultKBucketConfig) GetMaxLatency() time.Duration {
	return 10 * time.Second
}

// GetUsefulnessGracePeriod 返回有用性宽限期
func (c *defaultKBucketConfig) GetUsefulnessGracePeriod() time.Duration {
	return 60 * time.Second // 更保守的阈值，降低误删风险
}

// GetRefreshInterval 返回刷新间隔
func (c *defaultKBucketConfig) GetRefreshInterval() time.Duration {
	return 1 * time.Hour
}

// GetMaxReplacementCacheSize 返回最大替换缓存大小
func (c *defaultKBucketConfig) GetMaxReplacementCacheSize() int {
	return 5
}

// GetMaxPeersPerCpl 返回每个公共前缀长度的最大节点数
func (c *defaultKBucketConfig) GetMaxPeersPerCpl() int {
	return 20
}

// IsDiversityFilterEnabled 是否启用多样性过滤
func (c *defaultKBucketConfig) IsDiversityFilterEnabled() bool {
	return true
}

// GetFailureThreshold 返回失败阈值（触发Suspect状态）
func (c *defaultKBucketConfig) GetFailureThreshold() int {
	return 3 // 2分钟窗口内累计3次失败
}

// GetQuarantineDuration 返回隔离时长
func (c *defaultKBucketConfig) GetQuarantineDuration() time.Duration {
	return 1 * time.Minute // 隔离1分钟（加快清理速度）
}

// GetMinPeersPerBucket 返回每个桶的最小节点数（避免被掏空）
func (c *defaultKBucketConfig) GetMinPeersPerBucket() int {
	return 2 // 每桶至少保留2个节点，增强桶稳定性
}

// GetProbeInterval 返回探测间隔（用于探测隔离节点）
func (c *defaultKBucketConfig) GetProbeInterval() time.Duration {
	return 30 * time.Second // 每30秒探测一次
}

// GetHealthDecayHalfLife 返回健康分衰减半衰期
func (c *defaultKBucketConfig) GetHealthDecayHalfLife() time.Duration {
	return 5 * time.Minute // 5分钟半衰期
}

// GetMaintainInterval 返回维护协程的运行间隔
func (c *defaultKBucketConfig) GetMaintainInterval() time.Duration {
	return 30 * time.Second // 每30秒运行一次维护，降低维护频率
}

// GetCleanupGracePeriod 返回清理宽限期（断连/长期无用节点进入待清理/待探测前的最小保留时间）
// P0-010：3分钟过短，网络抖动/临时断连会触发误清理，导致 K 桶被逐步掏空。
func (c *defaultKBucketConfig) GetCleanupGracePeriod() time.Duration {
	return 10 * time.Minute
}

// GetLowHealthThreshold 返回低健康分阈值
// P0-010：从 20 降到 10，更保守，减少误判。
func (c *defaultKBucketConfig) GetLowHealthThreshold() float64 {
	return 10
}

// GetAddrProtectionGracePeriod 返回地址保护宽限期
// P0-010：为仍有地址（在 peerstore 中）的 peer 提供更长的保护窗口。
// 这类 peer 即使暂时不可用，只要地址还在，就有较高概率能重新连接。
func (c *defaultKBucketConfig) GetAddrProtectionGracePeriod() time.Duration {
	return 30 * time.Minute
}

// === Phase 2：清理前探测机制配置 ===

// IsPreCleanupProbeEnabled 是否启用清理前探测
func (c *defaultKBucketConfig) IsPreCleanupProbeEnabled() bool {
	return true // 生产环境默认开启
}

// GetProbeTimeout 获取探测超时时间
func (c *defaultKBucketConfig) GetProbeTimeout() time.Duration {
	return 5 * time.Second // 5秒超时
}

// GetProbeFailThreshold 获取探测失败阈值
func (c *defaultKBucketConfig) GetProbeFailThreshold() int {
	return 3 // 连续3次失败才删除
}

// GetProbeIntervalMin 获取最小探测间隔
func (c *defaultKBucketConfig) GetProbeIntervalMin() time.Duration {
	return 30 * time.Second // 避免频繁探测
}

// GetProbeMaxConcurrent 获取最大并发探测数
func (c *defaultKBucketConfig) GetProbeMaxConcurrent() int {
	return 5 // 最多5个并发探测
}

// applyPlatformDefaults 应用平台特定的默认值
func applyPlatformDefaults(config kademlia.KBucketConfig) kademlia.KBucketConfig {
	// 根据运行平台调整配置参数
	// 例如：移动设备可能需要更小的桶大小和更长的刷新间隔

	// 这里可以根据runtime.GOOS等进行平台特定优化
	return config
}

// validateKBucketConfig 验证K桶配置的有效性
func validateKBucketConfig(config kademlia.KBucketConfig) error {
	if config.GetBucketSize() <= 0 {
		return ErrInvalidBucketSize
	}

	if config.GetMaxLatency() <= 0 {
		return ErrInvalidMaxLatency
	}

	if config.GetUsefulnessGracePeriod() <= 0 {
		return ErrInvalidGracePeriod
	}

	return nil
}
