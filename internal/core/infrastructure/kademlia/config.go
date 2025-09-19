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
	return 60 * time.Second
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
