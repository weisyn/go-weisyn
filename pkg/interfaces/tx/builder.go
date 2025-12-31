// Package tx 提供交易处理的公共接口定义
//
// 📋 **builder.go - 交易构建接口**
//
// 本文件定义交易构建器的公共接口，包括通用交易构建和激励交易构建。
package tx

import (
	"context"

	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                         通用交易构建接口（骨架）
// ============================================================================

// TxBuilder 通用交易构建器接口（骨架定义）
//
// 🎯 **核心职责**: 构建普通交易（转账、合约调用等）
//
// ⚠️ **状态**: 接口骨架，完整实现在后续迭代中完成
//
// 设计理念:
//   - Type-state模式保证构建顺序
//   - 纯装配器，不做UTXO选择、费用估算等业务逻辑
//   - Draft模式支持渐进式构建（ISPC场景）
//
// 实现位置:
//   - internal/core/tx/builder/ (待实现)
type TxBuilder interface {
	// SetNonce 设置交易Nonce
	SetNonce(nonce uint64) TxBuilder

	// AddInput 添加交易输入
	//
	// 参数:
	//   outpoint: 引用的UTXO OutPoint
	//   isReferenceOnly: 是否仅引用（不消费）
	AddInput(outpoint *transaction_pb.OutPoint, isReferenceOnly bool) TxBuilder

	// AddAssetOutput 添加资产输出
	//
	// 参数:
	//   toAddress: 接收方地址
	//   amount: 金额（字符串格式，支持大数）
	//   contractAddress: 合约地址（nil表示原生币）
	//   lockingCondition: 锁定条件
	AddAssetOutput(
		toAddress []byte,
		amount string,
		contractAddress []byte,
		lockingCondition *transaction_pb.LockingCondition,
	) TxBuilder

	// Build 构建交易
	//
	// 返回:
	//   *types.ComposedTx: 组装完成的交易（Type-state模式）
	//   error: 构建错误
	Build() (*types.ComposedTx, error)

	// TODO: 其他方法待补充
	// - AddContractOutput()
	// - SetChainID()
	// - SetCreationTimestamp()
	// 等等...
}

// ============================================================================
//                         激励交易构建接口
// ============================================================================

// IncentiveTxBuilder 激励交易构建器接口
	//
// 🎯 **核心职责**: 构建矿工激励交易（Coinbase + 赞助领取）
	//
// 设计理念:
//   - 零增发Coinbase: 仅聚合手续费
//   - 赞助激励: 可选的项目方代币激励
//   - 共识内部: 这些交易不经过TxPool，直接插入区块
	//
// 激励交易结构:
//   Block.Transactions = [Coinbase, SponsorClaim1, SponsorClaim2, ..., NormalTx1, NormalTx2, ...]
//                         └────────── 激励区 ──────────┘
	//
// 调用链:
//   Miner Incentive Collector
//   → IncentiveTxBuilder.BuildIncentiveTransactions()
//   → [Coinbase, ClaimTxs...]
	//
// 实现位置:
//   - internal/core/tx/builder/incentive.go
type IncentiveTxBuilder interface {
	// BuildIncentiveTransactions 构建激励交易（Coinbase + 赞助领取）
	//
	// 🎯 **矿工激励交易构建核心方法**
	//
	// 构建内容:
	//   1. Coinbase交易（零增发：仅聚合手续费）
	//      - 无输入
	//      - 输出 = 聚合的手续费（按Token分组）
	//      - 所有输出Owner = minerAddr
	//
	//   2. 赞助领取交易（0-N笔，根据策略和可用性）
	//      - 扫描赞助池UTXO（Owner = SponsorPoolOwner）
	//      - 过滤有效的赞助（DelegationLock检查）
	//      - 构建领取交易（consume + 找零）
	//
	// 参数:
	//   ctx: 上下文对象
	//   candidateTxs: 候选交易列表（用于计算手续费）
	//   minerAddr: 矿工地址（激励接收方）
	//   chainID: 链ID
	//   blockHeight: 当前区块高度（用于DelegationLock有效期检查）
	//
	// 返回:
	//   []*Transaction: 激励交易列表 [Coinbase, ClaimTx1, ClaimTx2, ...]
	//   error: 构建错误
	//
	// 约束:
	//   - Coinbase必须是第一笔
	//   - 赞助领取交易数量受策略限制（MaxPerBlock）
	//   - 赞助领取失败不应阻塞Coinbase构建
	//
	// 使用场景:
	//   矿工在创建候选区块时，调用此方法获取激励交易，
	//   然后将激励交易放在区块首部，普通交易放在后面。
	//
	// 示例:
	//
	//	incentiveTxs, err := builder.BuildIncentiveTransactions(
	//	    ctx,
	//	    candidateTxs,      // [tx1, tx2, tx3]
	//	    minerAddr,         // 0x1234...
	//	    chainID,           // [0x01, 0x00, ...]
	//	    blockHeight,       // 100000
	//	)
	//	// incentiveTxs = [Coinbase, ClaimTx1, ClaimTx2]
	//	// 最终区块交易 = [Coinbase, ClaimTx1, ClaimTx2, tx1, tx2, tx3]
	BuildIncentiveTransactions(
		ctx context.Context,
		candidateTxs []*transaction_pb.Transaction,
		minerAddr []byte,
		chainID []byte,
		blockHeight uint64,
	) ([]*transaction_pb.Transaction, error)
}

// ============================================================================
//                         辅助数据结构
// ============================================================================

// SponsorClaim 赞助领取信息
//
// 用于内部传递赞助池UTXO的过滤结果。
type SponsorClaim struct {
	OutPoint       *transaction_pb.OutPoint // 赞助池UTXO的OutPoint
	AssetOutput    *transaction_pb.AssetOutput // 资产输出
	DelegationLock *transaction_pb.DelegationLock // 委托锁定条件
	AvailableAmount uint64 // 可领取金额
	ExpiryHeight   uint64 // 过期高度（创建高度 + 有效期）
}
