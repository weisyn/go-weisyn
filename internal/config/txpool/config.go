package txpool

import "time"

// TxPoolOptions 交易池配置选项
type TxPoolOptions struct {
	// 基础池配置
	MaxSize    int    `json:"max_size"`
	PriceLimit uint64 `json:"price_limit"`
	PriceBump  uint64 `json:"price_bump"`

	// 生存周期配置
	Lifetime   time.Duration `json:"lifetime"`
	KeepLocals bool          `json:"keep_locals"`

	// 挖矿配置
	Mining MiningOptions `json:"mining"`

	// 性能和监控配置
	MetricsEnabled  bool          `json:"metrics_enabled"`
	MetricsInterval time.Duration `json:"metrics_interval"`

	// 资源限制配置
	MemoryLimit uint64 `json:"memory_limit"`
	MaxTxSize   uint64 `json:"max_tx_size"`
}

// MiningOptions 挖矿配置选项
type MiningOptions struct {
	MaxTransactionsForMining uint32 `json:"max_transactions_for_mining"` // 挖矿时最大交易数量
	MaxBlockSizeForMining    uint64 `json:"max_block_size_for_mining"`   // 挖矿时区块大小限制（字节）
}

// Config 交易池配置实现
type Config struct {
	options *TxPoolOptions
}

// New 创建交易池配置实现
func New(userConfig interface{}) *Config {
	defaultOptions := createDefaultTxPoolOptions()
	return &Config{
		options: defaultOptions,
	}
}

// createDefaultTxPoolOptions 创建默认交易池配置
func createDefaultTxPoolOptions() *TxPoolOptions {
	return &TxPoolOptions{
		MaxSize:    defaultMaxSize,
		PriceLimit: defaultPriceLimit,
		PriceBump:  defaultPriceBump,
		Lifetime:   defaultLifetime,
		KeepLocals: defaultKeepLocals,
		Mining: MiningOptions{
			MaxTransactionsForMining: defaultMaxTransactionsForMining,
			MaxBlockSizeForMining:    defaultMaxBlockSizeForMining,
		},
		MetricsEnabled:  defaultMetricsEnabled,
		MetricsInterval: defaultMetricsInterval,
		MemoryLimit:     defaultMemoryLimit,
		MaxTxSize:       defaultMaxTxSize,
	}
}

// GetOptions 获取完整的交易池配置选项
func (c *Config) GetOptions() *TxPoolOptions {
	return c.options
}

// GetMaxSize 获取交易池最大容量
func (c *Config) GetMaxSize() int {
	return c.options.MaxSize
}

// GetPriceLimit 获取最低交易价格
func (c *Config) GetPriceLimit() uint64 {
	return c.options.PriceLimit
}

// GetPriceBump 获取价格提升百分比
func (c *Config) GetPriceBump() uint64 {
	return c.options.PriceBump
}

// GetLifetime 获取交易生存时间
func (c *Config) GetLifetime() time.Duration {
	return c.options.Lifetime
}

// IsKeepLocals 是否保留本地交易
func (c *Config) IsKeepLocals() bool {
	return c.options.KeepLocals
}

// IsMetricsEnabled 是否启用性能指标收集
func (c *Config) IsMetricsEnabled() bool {
	return c.options.MetricsEnabled
}

// GetMemoryLimit 获取内存限制
func (c *Config) GetMemoryLimit() uint64 {
	return c.options.MemoryLimit
}

// GetMiningOptions 获取挖矿配置选项
func (c *Config) GetMiningOptions() *MiningOptions {
	return &c.options.Mining
}

// GetMaxTransactionsForMining 获取挖矿时最大交易数量
func (c *Config) GetMaxTransactionsForMining() uint32 {
	return c.options.Mining.MaxTransactionsForMining
}

// GetMaxBlockSizeForMining 获取挖矿时区块大小限制
func (c *Config) GetMaxBlockSizeForMining() uint64 {
	return c.options.Mining.MaxBlockSizeForMining
}
