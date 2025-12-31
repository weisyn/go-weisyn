// Package types 账户相关业务抽象数据结构
//
// 🎯 **设计理念**
// 本文件定义面向用户的账户抽象数据结构，提供用户友好的账户概念，
// 隐藏底层UTXO技术细节，实现"账户视角"的业务语义。
//
// 📊 **核心概念**
// - **账户抽象**：将分散的UTXO聚合为统一的账户余额
// - **业务语义**：使用账户、余额、转账等用户熟悉的概念
// - **技术隐藏**：内部使用UTXO但对外完全隐藏技术细节
//
// 🏗️ **架构分层**
// - **pb层**：标准化的UTXO数据结构（pb.blockchain.utxo）
// - **types层**：业务友好的账户抽象（BalanceInfo, AccountInfo）
// - **interface层**：面向外部组件的AccountService接口
package types

import (
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ================================================================================================
// 🎯 第一部分：业务抽象类型
// ================================================================================================

/**
 * BalanceInfo - 账户余额信息
 *
 * 🎯 **业务语义**：用户账户余额的完整视图，包含可用、锁定、待确认余额
 *
 * 📝 **使用场景**：
 * • 钱包余额显示
 * • 交易前余额验证
 * • 余额变动追踪
 * • 锁定余额管理
 *
 * 💡 **关键设计**：
 * - Total = Available + Locked（修正：pending不参与总余额计算）
 * - Available：可立即使用的余额
 * - Locked：被时间锁、多签等条件锁定的余额
 * - Pending：在内存池中等待确认的余额变动（仅作参考，不影响总额）
 */
type BalanceInfo struct {
	// 核心标识
	Address *transaction.Address `json:"address"`  // 账户地址
	TokenID []byte               `json:"token_id"` // 代币ID（空=原生WES）

	// 余额分类（wei原始值，精确计算用）
	Available uint64 `json:"available"` // 可用余额（wei）
	Locked    uint64 `json:"locked"`    // 锁定余额（wei）
	Pending   uint64 `json:"pending"`   // 待确认余额（wei）
	Total     uint64 `json:"total"`     // 总余额（wei）

	// 格式化余额（WES单位，用户友好显示）
	AvailableFormatted string `json:"available_formatted"` // 可用余额（WES）
	LockedFormatted    string `json:"locked_formatted"`    // 锁定余额（WES）
	PendingFormatted   string `json:"pending_formatted"`   // 待确认余额（WES）
	TotalFormatted     string `json:"total_formatted"`     // 总余额（WES）

	// 统计信息
	UTXOCount uint32 `json:"utxo_count"` // UTXO数量

	// 元信息
	LastUpdated  time.Time `json:"last_updated"`  // 最后更新时间
	UpdateHeight uint64    `json:"update_height"` // 更新区块高度
}

/**
 * AccountInfo - 账户基础信息
 *
 * 🎯 **业务语义**：账户的核心状态信息
 * 📊 **数据组成**：地址、余额、UTXO统计、时间信息、nonce状态
 * 🎯 **使用场景**：
 * • API查询账户信息
 * • 钱包显示账户概览
 * • 交易构建时获取账户状态
 */
type AccountInfo struct {
	Address      *transaction.Address `json:"address"`       // 账户地址
	Balances     []*BalanceInfo       `json:"balances"`      // 各代币余额
	TotalUTXOs   uint32               `json:"total_utxos"`   // 总UTXO数量
	LastActivity time.Time            `json:"last_activity"` // 最后活动时间
	CreatedTime  time.Time            `json:"created_time"`  // 创建时间
	Nonce        uint64               `json:"nonce"`         // 🎯 账户nonce（交易序号）
}

// ================================================================================================
// 📊 第三部分：锁定余额详情
// ================================================================================================

/**
 * LockedBalanceEntry - 锁定余额条目
 *
 * 🎯 **业务语义**：单笔锁定余额的详细信息
 */
type LockedBalanceEntry struct {
	// 基础信息
	TxID        []byte               `json:"tx_id"`        // 锁定交易ID
	OutputIndex uint32               `json:"output_index"` // 输出索引
	Amount      uint64               `json:"amount"`       // 锁定金额
	TokenID     []byte               `json:"token_id"`     // 代币ID
	Address     *transaction.Address `json:"address"`      // 地址

	// 锁定详情
	LockType        string `json:"lock_type"`        // 锁定类型
	LockReason      string `json:"lock_reason"`      // 锁定原因
	UnlockHeight    uint64 `json:"unlock_height"`    // 解锁区块高度
	UnlockTimestamp uint64 `json:"unlock_timestamp"` // 解锁时间戳

	// 状态信息
	IsActive      bool      `json:"is_active"`      // 是否激活
	CreatedAt     time.Time `json:"created_at"`     // 创建时间
	EstimatedTime time.Time `json:"estimated_time"` // 预计解锁时间
}

/**
 * PendingBalanceEntry - 待确认余额条目
 *
 * 🎯 **业务语义**：内存池中影响余额的交易信息
 */
type PendingBalanceEntry struct {
	// 基础信息
	TxID       []byte               `json:"tx_id"`       // 交易ID
	Address    *transaction.Address `json:"address"`     // 地址
	TokenID    []byte               `json:"token_id"`    // 代币ID
	Amount     int64                `json:"amount"`      // 金额变动（正数=收入，负数=支出）
	ChangeType string               `json:"change_type"` // 变动类型

	// 状态信息
	Status        string    `json:"status"`         // 状态（pending/confirmed/failed）
	SubmittedAt   time.Time `json:"submitted_at"`   // 提交时间
	Confirmations uint32    `json:"confirmations"`  // 当前确认数
	RequiredConfs uint32    `json:"required_confs"` // 需要确认数

	// 费用信息
	Fee               uint64 `json:"fee"`                 // 交易费用
	ExecutionFeeUsed  uint64 `json:"execution_fee_used"`  // 消耗执行费用
	ExecutionFeePrice uint64 `json:"execution_fee_price"` // 执行费用价格
}

/**
 * EffectiveBalanceInfo - 有效余额信息
 *
 * 🎯 **业务语义**：用户真正可动用的余额计算结果，解决审查报告中用户期望的余额实时扣减问题
 *
 * 📝 **使用场景**：
 * • 钱包显示"可动用余额"
 * • 转账前余额验证
 * • 实时余额状态跟踪
 * • 解决矿工地址、找零等混淆问题
 *
 * 💡 **核心计算公式**：
 * SpendableAmount = ConfirmedAvailable - PendingOut + PendingIn
 * 其中：
 * - ConfirmedAvailable：已确认的可用余额
 * - PendingOut：待确认的支出金额（绝对值）
 * - PendingIn：待确认的收入金额
 */
type EffectiveBalanceInfo struct {
	// 核心标识
	Address *transaction.Address `json:"address"`  // 账户地址
	TokenID []byte               `json:"token_id"` // 代币ID（空=原生WES）

	// 核心计算结果
	SpendableAmount uint64 `json:"spendable_amount"` // 可动用余额（最终结果）

	// 计算过程明细（透明化计算过程，便于用户理解和调试）
	ConfirmedAvailable uint64 `json:"confirmed_available"` // 已确认可用余额
	PendingOut         uint64 `json:"pending_out"`         // 待确认支出（正数）
	PendingIn          uint64 `json:"pending_in"`          // 待确认收入（正数）

	// 状态统计
	PendingTxCount    uint32 `json:"pending_tx_count"`     // 待确认交易数
	PendingOutTxCount uint32 `json:"pending_out_tx_count"` // 待确认支出交易数
	PendingInTxCount  uint32 `json:"pending_in_tx_count"`  // 待确认收入交易数

	// 元信息
	LastUpdated       time.Time `json:"last_updated"`       // 最后更新时间
	UpdateHeight      uint64    `json:"update_height"`      // 更新区块高度
	CalculationMethod string    `json:"calculation_method"` // 计算方法标识

	// 调试信息（可选，用于问题诊断）
	DebugInfo *EffectiveBalanceDebugInfo `json:"debug_info,omitempty"`
}

/**
 * EffectiveBalanceDebugInfo - 有效余额计算调试信息
 *
 * 🎯 **业务语义**：用于诊断余额计算问题，特别是审查报告中提到的地址混淆等情况
 */
type EffectiveBalanceDebugInfo struct {
	// 地址分析
	IsMinerAddress         bool   `json:"is_miner_address"`          // 是否为矿工地址
	LastMiningRewardHeight uint64 `json:"last_mining_reward_height"` // 最后挖矿奖励高度

	// UTXO状态统计
	AvailableUTXOCount  uint32 `json:"available_utxo_count"`  // 可用UTXO数量
	ReferencedUTXOCount uint32 `json:"referenced_utxo_count"` // 被引用UTXO数量
	LockedUTXOCount     uint32 `json:"locked_utxo_count"`     // 锁定UTXO数量

	// Pending交易分析
	PendingTransactionIds [][]byte `json:"pending_transaction_ids"` // 相关待确认交易ID列表
	FastConfirmationCount uint32   `json:"fast_confirmation_count"` // 快速确认交易数

	// 计算时间戳
	CalculatedAt      time.Time `json:"calculated_at"`       // 计算时间
	UTXOQueryDuration int64     `json:"utxo_query_duration"` // UTXO查询耗时（毫秒）
	TxPoolQueryTime   int64     `json:"txpool_query_time"`   // 交易池查询耗时（毫秒）
}

// ================================================================================================
// 📈 第四部分：地址交易历史和统计（已移除）
// ================================================================================================
// 以下类型已被移除（过度设计，未被实际使用）：
// - AddressTransactionHistory
// - TransactionSummary
// - AddressTxStats
// 如需使用，可从 git 历史中恢复

// ================================================================================================
// 🎯 第五部分：多签钱包配置（已移除）
// ================================================================================================
// 以下类型已被移除（过度设计，未被实际使用）：
// - MultiSigWalletConfig
// 如需使用，可从 git 历史中恢复

// ================================================================================================
// 📊 第六部分：UTXO优化分析（已移除）
// ================================================================================================
// 以下类型已被移除（过度设计，未被实际使用）：
// - UTXOOptimizationAnalysis
// 如需使用，可从 git 历史中恢复

// ================================================================================================
// 🔧 第七部分：工具函数
// ================================================================================================

// NewBalanceInfo 创建新的余额信息
// 这是一个纯构造函数，不包含业务逻辑，符合 types 包的设计原则
func NewBalanceInfo(address *transaction.Address, tokenID []byte) *BalanceInfo {
	return &BalanceInfo{
		Address:      address,
		TokenID:      tokenID,
		Available:    0,
		Locked:       0,
		Pending:      0,
		Total:        0,
		UTXOCount:    0,
		LastUpdated:  time.Now(),
		UpdateHeight: 0,
	}
}

// 注意：以下业务逻辑方法已移除，应移到业务层（internal/core/account）：
// - UpdateBalance() - 余额更新逻辑
// - IsEmpty() - 余额检查逻辑
// - GetSpendable() - 可花费余额计算
// - HasSufficientBalance() - 余额充足性检查
//
// 这些方法应该在 internal/core/account/service.go 或类似位置实现
