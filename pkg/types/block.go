package types

// 保留区块相关注释与已注释结构，实际同步状态已统一至 pkg/types/system_sync.go

// ================================================================================================
// 🎯 第一部分：区块查询和统计类型
// ================================================================================================

// ⚠️ **非业务性过度设计 - 已注释**
// 以下类型为复杂区块统计查询功能，不被 pkg/interfaces/blockchain 直接使用
// 如需要时可取消注释

/*
// BlockInfo - 区块信息摘要
// 业务语义：面向用户的区块信息视图
type BlockInfo struct {
	// 基础信息
	Height    uint64     `json:"height"`    // 区块高度
	Hash      *core.Hash `json:"hash"`      // 区块哈希
	Timestamp time.Time  `json:"timestamp"` // 区块时间
	Size      uint64     `json:"size"`      // 区块大小

	// 交易信息
	TransactionCount uint32 `json:"transaction_count"` // 交易数量
	TotalValue       uint64 `json:"total_value"`       // 总价值

	// 挖矿信息
	Miner      *core.Address `json:"miner"`      // 矿工地址
	Difficulty uint64        `json:"difficulty"` // 挖矿难度
	Nonce      uint64        `json:"nonce"`      // 随机数

	// 状态信息
	Status        string `json:"status"`        // 区块状态
	Confirmations uint32 `json:"confirmations"` // 确认数
}

// BlockQuery - 区块查询条件
type BlockQuery struct {
	// 高度范围
	StartHeight uint64 `json:"start_height,omitempty"` // 起始高度
	EndHeight   uint64 `json:"end_height,omitempty"`   // 结束高度

	// 时间范围
	StartTime time.Time `json:"start_time,omitempty"` // 起始时间
	EndTime   time.Time `json:"end_time,omitempty"`   // 结束时间

	// 矿工过滤
	MinerAddress *core.Address `json:"miner_address,omitempty"` // 矿工地址

	// 分页
	Limit  uint32 `json:"limit,omitempty"`  // 限制数量
	Offset uint32 `json:"offset,omitempty"` // 偏移量

	// 排序
	SortBy        string `json:"sort_by,omitempty"`        // 排序字段
	SortDirection string `json:"sort_direction,omitempty"` // 排序方向
}
*/

// ================================================================================================
// 🎯 第二部分：区块统计类型
// ================================================================================================

// ⚠️ **非业务性过度设计 - 已注释**
// 以下类型为区块统计分析功能，不被 pkg/interfaces/blockchain 直接使用
// 如需要时可取消注释

/*
// BlockStats - 区块统计信息
type BlockStats struct {
	// 基础统计
	TotalBlocks      uint64    `json:"total_blocks"`       // 总区块数
	AverageBlockTime float64   `json:"average_block_time"` // 平均出块时间
	LastBlockTime    time.Time `json:"last_block_time"`    // 最后区块时间

	// 难度统计
	CurrentDifficulty uint64  `json:"current_difficulty"` // 当前难度
	DifficultyChange  float64 `json:"difficulty_change"`  // 难度变化百分比

	// 交易统计
	TotalTransactions     uint64  `json:"total_transactions"`      // 总交易数
	TransactionsPerBlock  float64 `json:"transactions_per_block"`  // 每区块平均交易数
	TransactionsPerSecond float64 `json:"transactions_per_second"` // 每秒交易数

	// 价值统计
	TotalValue         uint64 `json:"total_value"`         // 总价值
	AverageBlockValue  uint64 `json:"average_block_value"` // 平均区块价值
	AverageTransaction uint64 `json:"average_transaction"` // 平均交易价值

	// 网络统计
	HashRate        uint64 `json:"hash_rate"`        // 网络算力
	NetworkSecurity string `json:"network_security"` // 网络安全级别
}
*/

// ================================================================================================
// 🎯 第三部分：链级状态类型
// ================================================================================================

// ⚠️ **非业务性过度设计 - 已注释**
// 以下类型为复杂链状态信息，不被 pkg/interfaces/blockchain 直接使用
// 如需要时可取消注释

/*
// ChainInfo - 区块链信息
type ChainInfo struct {
	// 链基础信息
	ChainID       string     `json:"chain_id"`        // 链标识
	NetworkType   string     `json:"network_type"`    // 网络类型
	GenesisTime   time.Time  `json:"genesis_time"`    // 创世时间
	CurrentHeight uint64     `json:"current_height"`  // 当前高度
	BestBlockHash *core.Hash `json:"best_block_hash"` // 最佳区块哈希
	PreviousHash  *core.Hash `json:"previous_hash"`   // 前一区块哈希
	StateRoot     *core.Hash `json:"state_root"`      // 状态根

	// 网络状态
	PeerCount       uint32    `json:"peer_count"`        // 节点数量
	IsSync          bool      `json:"is_sync"`           // 是否同步
	SyncProgress    float64   `json:"sync_progress"`     // 同步进度
	LastSyncTime    time.Time `json:"last_sync_time"`    // 最后同步时间
	NetworkHashRate uint64    `json:"network_hash_rate"` // 网络算力

	// 版本信息
	ProtocolVersion string `json:"protocol_version"` // 协议版本
	SoftwareVersion string `json:"software_version"` // 软件版本
	DatabaseVersion string `json:"database_version"` // 数据库版本

	// 性能指标
	AverageBlockTime    float64 `json:"average_block_time"`    // 平均出块时间
	AverageBlockSize    uint64  `json:"average_block_size"`    // 平均区块大小
	TransactionPoolSize uint32  `json:"transaction_pool_size"` // 交易池大小
}
*/

// 同步状态定义已统一至 pkg/types/system_sync.go

// ================================================================================================
// 🎯 第四部分：创世区块和分叉管理
// ================================================================================================

// ⚠️ **非业务性过度设计 - 已注释**
// 以下类型为创世配置和分叉管理功能，不被 pkg/interfaces/blockchain 直接使用
// 这些属于底层配置和共识层处理的技术细节，如需要时可取消注释

/*
// GenesisConfig - 创世区块配置
type GenesisConfig struct {
	// 基础配置
	ChainID           string    `json:"chain_id"`           // 链标识
	NetworkType       string    `json:"network_type"`       // 网络类型
	GenesisTime       time.Time `json:"genesis_time"`       // 创世时间
	InitialDifficulty uint64    `json:"initial_difficulty"` // 初始难度

	// 预分配资金
	Allocations []*GenesisAllocation `json:"allocations"`  // 预分配
	TotalSupply uint64               `json:"total_supply"` // 总供应量

	// 共识参数
	BlockTime        uint32 `json:"block_time"`        // 目标出块时间(秒)
	MaxBlockSize     uint64 `json:"max_block_size"`    // 最大区块大小
	DifficultyAdjust uint32 `json:"difficulty_adjust"` // 难度调整周期

	// 系统参数
	MinTxFee      uint64 `json:"min_tx_fee"`       // 最小交易费
	MaxTxPerBlock uint32 `json:"max_tx_per_block"` // 每区块最大交易数
}

// GenesisAllocation - 创世预分配
type GenesisAllocation struct {
	Address     *core.Address `json:"address"`     // 接收地址
	Amount      uint64        `json:"amount"`      // 分配金额
	Description string        `json:"description"` // 描述
	LockPeriod  uint64        `json:"lock_period"` // 锁定期(区块数)
}

// ForkInfo - 分叉信息
type ForkInfo struct {
	// 分叉基础信息
	ForkHeight     uint64     `json:"fork_height"`     // 分叉高度
	CommonAncestor *core.Hash `json:"common_ancestor"` // 共同祖先
	MainChainTip   *core.Hash `json:"main_chain_tip"`  // 主链顶端
	ForkChainTip   *core.Hash `json:"fork_chain_tip"`  // 分叉链顶端

	// 分叉状态
	ForkLength uint32    `json:"fork_length"` // 分叉长度
	IsActive   bool      `json:"is_active"`   // 是否活跃
	DetectedAt time.Time `json:"detected_at"` // 检测时间

	// 分叉原因
	ForkReason   string `json:"fork_reason"`   // 分叉原因
	ConflictType string `json:"conflict_type"` // 冲突类型
}
*/

// ================================================================================================
// 🎯 第五部分：工具函数
// ================================================================================================

// ⚠️ **非业务性过度设计 - 已注释**
// 以下工具函数依赖已注释的类型，不被 pkg/interfaces/blockchain 直接使用
// 如需要时可取消注释

/*
// NewBlockInfo 创建新的区块信息
func NewBlockInfo(height uint64, hash *core.Hash) *BlockInfo {
	return &BlockInfo{
		Height:           height,
		Hash:             hash,
		Timestamp:        time.Now(),
		TransactionCount: 0,
		TotalValue:       0,
		Status:           "pending",
		Confirmations:    0,
	}
}

// IsConfirmed 检查区块是否已确认
func (b *BlockInfo) IsConfirmed(minConfirmations uint32) bool {
	return b.Confirmations >= minConfirmations
}

// GetAge 获取区块年龄
func (b *BlockInfo) GetAge() time.Duration {
	return time.Since(b.Timestamp)
}
*/

// 原 NewSyncStatus/UpdateProgress 已移除，请使用 system_sync 的定义与逻辑。

// ⚠️ **非业务性过度设计 - 已注释**
// 以下类型为额外的区块位置信息，不被 pkg/interfaces/blockchain 直接使用
// 如需要时可取消注释

/*
// BlockLocation 区块位置信息
type BlockLocation struct {
	Hash          *core.Hash `json:"hash"`          // 区块哈希
	Height        uint64     `json:"height"`        // 区块高度
	ChainTip      bool       `json:"chain_tip"`     // 是否是链顶
	Confirmations uint32     `json:"confirmations"` // 确认数
}
*/
