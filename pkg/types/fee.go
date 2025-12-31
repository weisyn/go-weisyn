// Package types 提供 WES 系统的公共类型定义
//
// 本文件定义费用相关的公共类型，供接口定义和实现共同使用
package types

import (
	"context"
	"math/big"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
//                           费用相关公共类型
// ============================================================================

// UTXOFetcher UTXO 查询回调函数类型
//
// 🎯 **功能**：从链状态中查询指定 UTXO 的详细信息
// 📋 **用途**：费用计算时需要获取输入 UTXO 的内容来计算差额
// ⚠️ **注意**：实现方需处理 UTXO 不存在的情况，返回明确错误
type UTXOFetcher func(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error)

// TokenKey Token 标识符类型
//
// 🎯 **功能**：统一标识不同类型的 Token（原生币、FT、NFT、SFT）
// 📋 **格式**：
//   - 原生币：空字符串 ""（约定使用空表示原生币，避免与任何合约代币冲突）
//   - FT：合约地址的十六进制字符串 + 类型 + ID，格式 "contract|ft|id"
//   - NFT：合约地址 + Token ID，格式 "contract|nft|id"
//   - SFT：合约地址 + 批次 ID + 实例 ID，格式 "contract|sft|batch|instance"
type TokenKey string

// FeeEstimate 费用估算结果
//
// 🎯 **功能**：提供三档费用估算（保守/标准/快速）
// 📋 **用途**：用户构造交易前的费用参考
//
// 三档费用说明：
//   - Conservative: 保守估算，确保交易被接受，适合不急的交易
//   - Standard: 标准估算，平衡速度和成本，适合大多数场景
//   - Fast: 快速估算，优先处理，适合紧急交易
type FeeEstimate struct {
	Conservative *big.Int // 保守估算
	Standard     *big.Int // 标准估算
	Fast         *big.Int // 快速估算
	TokenKey     TokenKey // 费用 Token 类型
	Mechanism    string   // 使用的费用机制
	Details      string   // 估算详情（可选）
}

// TransactionFee 单个交易的费用信息
//
// 🎯 **功能**：记录交易的实际费用（UTXO 差额）
// 📋 **内容**：按 Token 分类的费用和计算统计
type TransactionFee struct {
	TxID  []byte                // 交易 ID
	Fees  map[TokenKey]*big.Int // 按 Token 分类的费用
	Stats *FeeCalculationStats  // 计算统计信息
}

// FeeCalculationStats 费用计算统计信息
//
// 🎯 **功能**：记录费用计算过程的统计数据
// 📋 **用途**：调试、监控、性能分析
type FeeCalculationStats struct {
	InputCount       int  // 输入数量
	OutputCount      int  // 输出数量
	SuccessfulInputs int  // 成功处理的输入数量
	FailedInputs     int  // 失败的输入数量（UTXO 查询失败）
	TokenTypes       int  // 涉及的 Token 类型数量
	IsAirdrop        bool // 是否为空投交易（无输入）
	IsBurn           bool // 是否为销毁交易（无输出）
	HasZeroFee       bool // 是否为零费用交易
	HasMultiToken    bool // 是否包含多种 Token（已弃用，使用 TokenTypes）
}

// AggregatedFees 聚合费用信息
//
// 🎯 **功能**：汇总多个交易的费用
// 📋 **用途**：Coinbase 构建、费用统计
type AggregatedFees struct {
	ByToken map[TokenKey]*big.Int // 按 Token 分类的总费用
	Stats   *AggregationStats     // 聚合统计信息
}

// AggregationStats 聚合统计信息
//
// 🎯 **功能**：记录费用聚合过程的统计数据
// 📋 **用途**：区块统计、费用分析
type AggregationStats struct {
	TotalTxs       int                   // 总交易数
	ZeroFeeTxs     int                   // 零费用交易数
	TokenTypes     map[TokenKey]int      // 各 Token 类型的交易数
	TotalFeeAmount map[TokenKey]*big.Int // 各 Token 的总费用金额
}

// TransactionAnalysis 交易分析结果
//
// 🎯 **功能**：分析交易类型和费用机制特征
// 📋 **用途**：诊断、调试、监控
type TransactionAnalysis struct {
	Type                 string // 交易类型（正常/空投/销毁）
	Description          string // 类型描述
	FeeMechanism         string // 费用机制
	MechanismDescription string // 机制描述
	InputCount           int    // 输入数量
	OutputCount          int    // 输出数量
	IsValid              bool   // 是否有效
	IsAirdrop            bool   // 是否为空投
	IsBurn               bool   // 是否为销毁
	IsNormal             bool   // 是否为正常交易
}

// SystemStats 费用系统统计信息
//
// 🎯 **功能**：返回费用系统的能力信息
// 📋 **内容**：支持的机制、Token 类型、功能特性
type SystemStats struct {
	ManagerVersion      string   // 管理器版本
	SupportedMechanisms []string // 支持的费用机制
	SupportedTokenTypes []string // 支持的 Token 类型
	Features            []string // 支持的功能特性
}
