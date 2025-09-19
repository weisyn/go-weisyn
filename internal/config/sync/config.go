package sync

import "time"

// SyncOptions 同步配置选项
// 整个同步模块的统一配置入口，包含所有区块链同步相关的配置参数
type SyncOptions struct {
	// === 基础同步配置 ===
	Enabled  bool   `json:"enabled"`   // 是否启用同步功能
	SyncMode string `json:"sync_mode"` // 同步模式 (full, fast, light, snapshot)

	// === 并发和性能配置 ===
	Concurrency         int `json:"concurrency"`          // 同步并发数
	SnapshotConcurrency int `json:"snapshot_concurrency"` // 快照下载并发数
	MaxBatchSize        int `json:"max_batch_size"`       // 最大批处理大小

	// === 超时和重试配置 ===
	SyncTimeout    time.Duration `json:"sync_timeout"`    // 同步超时时间
	RequestTimeout time.Duration `json:"request_timeout"` // 请求超时时间
	RetryAttempts  int           `json:"retry_attempts"`  // 重试次数
	RetryDelay     time.Duration `json:"retry_delay"`     // 重试延迟

	// === 区块获取配置 ===
	MaxBlockFetch  int `json:"max_block_fetch"`  // 最大同步区块数
	MaxHeaderFetch int `json:"max_header_fetch"` // 最大头部获取数
	MaxStateFetch  int `json:"max_state_fetch"`  // 最大状态获取数

	// === 节点和网络配置 ===
	MinPeers    int           `json:"min_peers"`    // 最小对等节点数
	MaxPeers    int           `json:"max_peers"`    // 最大对等节点数
	PeerTimeout time.Duration `json:"peer_timeout"` // 节点超时时间

	// === 轻客户端配置 ===
	LightConfirmations int  `json:"light_confirmations"` // 轻客户端确认数
	EnableLightMode    bool `json:"enable_light_mode"`   // 是否启用轻模式

	// === 检查点和进度配置 ===
	CheckpointInterval     int           `json:"checkpoint_interval"`      // 检查点间隔
	ProgressReportInterval time.Duration `json:"progress_report_interval"` // 进度报告间隔
	CompletionThreshold    int           `json:"completion_threshold"`     // 同步完成阈值(百分比)

	// === 优化和高级配置 ===
	EnableForceResync  bool `json:"enable_force_resync"`  // 是否启用强制重新同步
	EnablePeerFilter   bool `json:"enable_peer_filter"`   // 是否启用节点过滤
	EnableStateSync    bool `json:"enable_state_sync"`    // 是否启用状态同步
	EnableSnapshotSync bool `json:"enable_snapshot_sync"` // 是否启用快照同步

	// === 缓存和存储配置 ===
	BlockCacheSize int64 `json:"block_cache_size"` // 区块缓存大小(字节)
	StateCacheSize int64 `json:"state_cache_size"` // 状态缓存大小(字节)
	TempDirSize    int64 `json:"temp_dir_size"`    // 临时目录大小限制(字节)

	// === 验证和安全配置 ===
	EnableFullValidation bool  `json:"enable_full_validation"` // 是否启用完整验证
	SkipVerification     bool  `json:"skip_verification"`      // 是否跳过验证
	TrustedHeight        int64 `json:"trusted_height"`         // 可信高度

	// === 内部配置（不对外暴露） ===
	SyncModes          []string               `json:"-"` // 支持的同步模式
	PeerFilterCriteria map[string]interface{} `json:"-"` // 节点过滤条件
	ValidationLevels   map[string]bool        `json:"-"` // 验证级别配置
}

// Config 同步配置实现
type Config struct {
	options *SyncOptions
}

// New 创建同步配置实现
func New(userConfig interface{}) *Config {
	// 1. 先创建完整的默认配置
	defaultOptions := createDefaultSyncOptions()

	// 2. 暂时不处理用户配置，后续添加
	// TODO: 当有用户配置类型时，在这里进行转换和合并

	return &Config{
		options: defaultOptions,
	}
}

// createDefaultSyncOptions 创建默认同步配置
func createDefaultSyncOptions() *SyncOptions {
	return &SyncOptions{
		// 基础同步配置
		Enabled:  defaultEnabled,
		SyncMode: defaultSyncMode,

		// 并发和性能配置
		Concurrency:         defaultConcurrency,
		SnapshotConcurrency: defaultSnapshotConcurrency,
		MaxBatchSize:        defaultMaxBatchSize,

		// 超时和重试配置
		SyncTimeout:    defaultSyncTimeout,
		RequestTimeout: defaultRequestTimeout,
		RetryAttempts:  defaultRetryAttempts,
		RetryDelay:     defaultRetryDelay,

		// 区块获取配置
		MaxBlockFetch:  defaultMaxBlockFetch,
		MaxHeaderFetch: defaultMaxHeaderFetch,
		MaxStateFetch:  defaultMaxStateFetch,

		// 节点和网络配置
		MinPeers:    defaultMinPeers,
		MaxPeers:    defaultMaxPeers,
		PeerTimeout: defaultPeerTimeout,

		// 轻客户端配置
		LightConfirmations: defaultLightConfirmations,
		EnableLightMode:    defaultEnableLightMode,

		// 检查点和进度配置
		CheckpointInterval:     defaultCheckpointInterval,
		ProgressReportInterval: defaultProgressReportInterval,
		CompletionThreshold:    defaultCompletionThreshold,

		// 优化和高级配置
		EnableForceResync:  defaultEnableForceResync,
		EnablePeerFilter:   defaultEnablePeerFilter,
		EnableStateSync:    defaultEnableStateSync,
		EnableSnapshotSync: defaultEnableSnapshotSync,

		// 缓存和存储配置
		BlockCacheSize: defaultBlockCacheSize,
		StateCacheSize: defaultStateCacheSize,
		TempDirSize:    defaultTempDirSize,

		// 验证和安全配置
		EnableFullValidation: defaultEnableFullValidation,
		SkipVerification:     defaultSkipVerification,
		TrustedHeight:        defaultTrustedHeight,

		// 内部配置
		SyncModes:          append([]string{}, defaultSyncModes...),     // 复制切片
		PeerFilterCriteria: copyInterfaceMap(defaultPeerFilterCriteria), // 复制映射
		ValidationLevels:   copyBoolMap(defaultValidationLevels),        // 复制映射
	}
}

// copyInterfaceMap 复制interface{}映射
func copyInterfaceMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyBoolMap 复制bool映射
func copyBoolMap(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// GetOptions 获取完整的同步配置选项
func (c *Config) GetOptions() *SyncOptions {
	return c.options
}

// === 基础同步配置访问方法 ===

// IsEnabled 是否启用同步功能
func (c *Config) IsEnabled() bool {
	return c.options.Enabled
}

// GetSyncMode 获取同步模式
func (c *Config) GetSyncMode() string {
	return c.options.SyncMode
}

// === 并发和性能配置访问方法 ===

// GetConcurrency 获取同步并发数
func (c *Config) GetConcurrency() int {
	return c.options.Concurrency
}

// GetSnapshotConcurrency 获取快照下载并发数
func (c *Config) GetSnapshotConcurrency() int {
	return c.options.SnapshotConcurrency
}

// GetMaxBatchSize 获取最大批处理大小
func (c *Config) GetMaxBatchSize() int {
	return c.options.MaxBatchSize
}

// === 超时和重试配置访问方法 ===

// GetSyncTimeout 获取同步超时时间
func (c *Config) GetSyncTimeout() time.Duration {
	return c.options.SyncTimeout
}

// GetRequestTimeout 获取请求超时时间
func (c *Config) GetRequestTimeout() time.Duration {
	return c.options.RequestTimeout
}

// GetRetryAttempts 获取重试次数
func (c *Config) GetRetryAttempts() int {
	return c.options.RetryAttempts
}

// GetRetryDelay 获取重试延迟
func (c *Config) GetRetryDelay() time.Duration {
	return c.options.RetryDelay
}

// === 区块获取配置访问方法 ===

// GetMaxBlockFetch 获取最大同步区块数
func (c *Config) GetMaxBlockFetch() int {
	return c.options.MaxBlockFetch
}

// GetMaxHeaderFetch 获取最大头部获取数
func (c *Config) GetMaxHeaderFetch() int {
	return c.options.MaxHeaderFetch
}

// GetMaxStateFetch 获取最大状态获取数
func (c *Config) GetMaxStateFetch() int {
	return c.options.MaxStateFetch
}

// === 节点和网络配置访问方法 ===

// GetMinPeers 获取最小对等节点数
func (c *Config) GetMinPeers() int {
	return c.options.MinPeers
}

// GetMaxPeers 获取最大对等节点数
func (c *Config) GetMaxPeers() int {
	return c.options.MaxPeers
}

// GetPeerTimeout 获取节点超时时间
func (c *Config) GetPeerTimeout() time.Duration {
	return c.options.PeerTimeout
}

// === 轻客户端配置访问方法 ===

// GetLightConfirmations 获取轻客户端确认数
func (c *Config) GetLightConfirmations() int {
	return c.options.LightConfirmations
}

// IsLightModeEnabled 是否启用轻模式
func (c *Config) IsLightModeEnabled() bool {
	return c.options.EnableLightMode
}

// === 检查点和进度配置访问方法 ===

// GetCheckpointInterval 获取检查点间隔
func (c *Config) GetCheckpointInterval() int {
	return c.options.CheckpointInterval
}

// GetProgressReportInterval 获取进度报告间隔
func (c *Config) GetProgressReportInterval() time.Duration {
	return c.options.ProgressReportInterval
}

// GetCompletionThreshold 获取同步完成阈值
func (c *Config) GetCompletionThreshold() int {
	return c.options.CompletionThreshold
}

// === 优化和高级配置访问方法 ===

// IsForceResyncEnabled 是否启用强制重新同步
func (c *Config) IsForceResyncEnabled() bool {
	return c.options.EnableForceResync
}

// IsPeerFilterEnabled 是否启用节点过滤
func (c *Config) IsPeerFilterEnabled() bool {
	return c.options.EnablePeerFilter
}

// IsStateSyncEnabled 是否启用状态同步
func (c *Config) IsStateSyncEnabled() bool {
	return c.options.EnableStateSync
}

// IsSnapshotSyncEnabled 是否启用快照同步
func (c *Config) IsSnapshotSyncEnabled() bool {
	return c.options.EnableSnapshotSync
}

// === 缓存和存储配置访问方法 ===

// GetBlockCacheSize 获取区块缓存大小
func (c *Config) GetBlockCacheSize() int64 {
	return c.options.BlockCacheSize
}

// GetStateCacheSize 获取状态缓存大小
func (c *Config) GetStateCacheSize() int64 {
	return c.options.StateCacheSize
}

// GetTempDirSize 获取临时目录大小限制
func (c *Config) GetTempDirSize() int64 {
	return c.options.TempDirSize
}

// === 验证和安全配置访问方法 ===

// IsFullValidationEnabled 是否启用完整验证
func (c *Config) IsFullValidationEnabled() bool {
	return c.options.EnableFullValidation
}

// IsVerificationSkipped 是否跳过验证
func (c *Config) IsVerificationSkipped() bool {
	return c.options.SkipVerification
}

// GetTrustedHeight 获取可信高度
func (c *Config) GetTrustedHeight() int64 {
	return c.options.TrustedHeight
}

// === 同步模式管理方法 ===

// GetSupportedSyncModes 获取支持的同步模式
func (c *Config) GetSupportedSyncModes() []string {
	return append([]string{}, c.options.SyncModes...) // 返回副本
}

// IsSyncModeSupported 检查同步模式是否被支持
func (c *Config) IsSyncModeSupported(mode string) bool {
	for _, supportedMode := range c.options.SyncModes {
		if supportedMode == mode {
			return true
		}
	}
	return false
}

// === 节点过滤管理方法 ===

// GetPeerFilterCriterion 获取节点过滤条件
func (c *Config) GetPeerFilterCriterion(key string) interface{} {
	return c.options.PeerFilterCriteria[key]
}

// GetAllPeerFilterCriteria 获取所有节点过滤条件
func (c *Config) GetAllPeerFilterCriteria() map[string]interface{} {
	return copyInterfaceMap(c.options.PeerFilterCriteria) // 返回副本
}

// === 验证级别管理方法 ===

// IsValidationLevelEnabled 检查验证级别是否启用
func (c *Config) IsValidationLevelEnabled(level string) bool {
	if enabled, exists := c.options.ValidationLevels[level]; exists {
		return enabled
	}
	return false
}

// GetAllValidationLevels 获取所有验证级别配置
func (c *Config) GetAllValidationLevels() map[string]bool {
	return copyBoolMap(c.options.ValidationLevels) // 返回副本
}
