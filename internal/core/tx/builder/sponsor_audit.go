package builder

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo_pb "github.com/weisyn/v1/pb/blockchain/utxo"
	cryptoface "github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"google.golang.org/protobuf/proto"
)

// SponsorAuditService 赞助UTXO审计服务
//
// 🎯 **核心职责**：提供赞助UTXO的审计和追踪功能
//
// 💡 **设计理念**：
// - 基于EUTXO原则：通过查询区块链历史获取审计信息
// - 不创建新的存储结构，通过查询接口聚合数据
// - 提供统一的审计查询接口
type SponsorAuditService struct {
	eutxoQuery  persistence.UTXOQuery
	txQuery     persistence.TxQuery    // 交易查询接口（用于查询历史）
	chainQuery  persistence.ChainQuery // 链状态查询接口（用于获取当前区块高度）
	hashManager cryptoface.HashManager // 哈希管理器（用于计算交易哈希）
	helper      *SponsorUTXOHelper     // 使用SponsorUTXOHelper辅助
}

// NewSponsorAuditService 创建赞助审计服务
func NewSponsorAuditService(
	eutxoQuery persistence.UTXOQuery,
	txQuery persistence.TxQuery,
	chainQuery persistence.ChainQuery,
	hashManager cryptoface.HashManager,
) *SponsorAuditService {
	return &SponsorAuditService{
		eutxoQuery:  eutxoQuery,
		txQuery:     txQuery,
		chainQuery:  chainQuery,
		hashManager: hashManager,
		helper:      NewSponsorUTXOHelper(eutxoQuery),
	}
}

// ClaimRecord 领取记录
//
// **设计说明**（基于架构分析文档）：
// - 通过查询交易历史获取领取记录
// - 不单独存储，而是通过查询接口聚合
type ClaimRecord struct {
	SponsorUTXOId []byte   // 赞助UTXO的OutPoint（txId + outputIndex）
	MinerAddress  []byte   // 矿工地址
	ClaimAmount   *big.Int // 领取金额
	ClaimTime     uint64   // 领取时间（区块时间戳）
	BlockHeight   uint64   // 区块高度
	TransactionId []byte   // 交易ID
	ChangeAmount  *big.Int // 找零金额（如果有）
}

// SponsorStats 赞助统计信息
type SponsorStats struct {
	TotalSponsors       int      // 总赞助数
	TotalAmount         *big.Int // 总金额
	TotalClaimed        *big.Int // 已领取金额
	TotalRemaining      *big.Int // 剩余金额
	ActiveSponsors      int      // 活跃赞助数（未过期）
	ExpiredSponsors     int      // 已过期赞助数
	FullyClaimedCount   int      // 全部领取数
	PartialClaimedCount int      // 部分领取数
}

// GetSponsorClaimHistory 查询赞助UTXO的领取历史
//
// **查询策略**：
// - 通过查询交易历史，找出所有引用该UTXO的消费交易
// - 解析DelegationProof获取领取信息
//
// **参数**：
//   - ctx: 上下文对象
//   - sponsorUTXOId: 赞助UTXO的OutPoint（txId + outputIndex）
//
// **返回**：
//   - []*ClaimRecord: 领取记录列表
//   - error: 查询错误
func (s *SponsorAuditService) GetSponsorClaimHistory(
	ctx context.Context,
	sponsorUTXOId *transaction_pb.OutPoint,
) ([]*ClaimRecord, error) {
	if sponsorUTXOId == nil {
		return nil, fmt.Errorf("sponsorUTXOId不能为空")
	}

	// 📝 **查询策略说明**：
	// 完整实现需要扩展TxQuery接口，添加"查询引用特定UTXO的交易"方法。
	// 当前实现为基础框架，返回空列表。
	//
	// **未来扩展**：
	// 1. 在TxQuery接口中添加：GetTransactionsByInputUTXO(ctx, outpoint) ([]*Transaction, error)
	// 2. 查询所有引用该UTXO的交易
	// 3. 过滤出赞助领取交易（有DelegationProof，且DelegateAddress匹配）
	// 4. 解析DelegationProof获取领取信息
	// 5. 从区块信息获取BlockHeight和ClaimTime
	// 6. 构建ClaimRecord列表

	// 当前简化实现：返回空列表
	// 需要扩展TxQuery接口支持"查询引用特定UTXO的交易"
	return []*ClaimRecord{}, nil
}

// GetMinerClaimHistory 查询矿工的领取历史
//
// **查询策略**：
// - 查询所有赞助池UTXO
// - 查询每个UTXO的领取历史
// - 过滤出指定矿工的领取记录
//
// **参数**：
//   - ctx: 上下文对象
//   - minerAddr: 矿工地址
//
// **返回**：
//   - []*ClaimRecord: 领取记录列表
//   - error: 查询错误
func (s *SponsorAuditService) GetMinerClaimHistory(
	ctx context.Context,
	minerAddr []byte,
) ([]*ClaimRecord, error) {
	if len(minerAddr) == 0 {
		return nil, fmt.Errorf("minerAddr不能为空")
	}

	// 1. 查询所有赞助池UTXO
	sponsorUTXOs, err := s.eutxoQuery.GetSponsorPoolUTXOs(ctx, false) // 包含已消费的
	if err != nil {
		return nil, fmt.Errorf("查询赞助池UTXO失败: %w", err)
	}

	// 2. 查询每个UTXO的领取历史（简化实现）
	var allClaims []*ClaimRecord
	for _, utxo := range sponsorUTXOs {
		outpoint := utxo.Outpoint
		claims, err := s.GetSponsorClaimHistory(ctx, outpoint)
		if err != nil {
			continue // 单个UTXO查询失败，继续下一个
		}

		// 3. 过滤出指定矿工的记录
		for _, claim := range claims {
			if bytes.Equal(claim.MinerAddress, minerAddr) {
				allClaims = append(allClaims, claim)
			}
		}
	}

	return allClaims, nil
}

// GetSponsorStatistics 统计赞助信息
//
// **统计策略**：
// - 查询所有赞助池UTXO
// - 计算统计信息
//
// **参数**：
//   - ctx: 上下文对象
//
// **返回**：
//   - *SponsorStats: 统计信息
//   - error: 查询错误
func (s *SponsorAuditService) GetSponsorStatistics(
	ctx context.Context,
) (*SponsorStats, error) {
	// 1. 查询所有赞助池UTXO
	sponsorUTXOs, err := s.eutxoQuery.GetSponsorPoolUTXOs(ctx, false) // 包含已消费的
	if err != nil {
		return nil, fmt.Errorf("查询赞助池UTXO失败: %w", err)
	}

	stats := &SponsorStats{
		TotalSponsors:       len(sponsorUTXOs),
		TotalAmount:         big.NewInt(0),
		TotalClaimed:        big.NewInt(0),
		TotalRemaining:      big.NewInt(0),
		ActiveSponsors:      0,
		ExpiredSponsors:     0,
		FullyClaimedCount:   0,
		PartialClaimedCount: 0,
	}

	// 2. 遍历UTXO计算统计
	for _, utxo := range sponsorUTXOs {
		metadata, err := s.helper.ExtractMetadata(utxo)
		if err != nil {
			continue // 提取失败，跳过
		}

		// 累计总金额
		stats.TotalAmount.Add(stats.TotalAmount, metadata.TotalAmount)

		// 判断状态
		if utxo.Status == utxo_pb.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED {
			stats.FullyClaimedCount++
			stats.TotalClaimed.Add(stats.TotalClaimed, metadata.TotalAmount)
		} else {
			stats.TotalRemaining.Add(stats.TotalRemaining, metadata.TotalAmount)

			// 判断是否部分领取：查询领取历史并累加金额
			claimHistory, err := s.GetSponsorClaimHistory(ctx, utxo.Outpoint)
			if err == nil && len(claimHistory) > 0 {
				// 累加所有领取金额
				totalClaimed := big.NewInt(0)
				for _, claim := range claimHistory {
					if claim.ClaimAmount != nil {
						totalClaimed.Add(totalClaimed, claim.ClaimAmount)
					}
				}
				// 如果累计领取金额 > 0 且 < 总金额，则为部分领取
				if totalClaimed.Sign() > 0 && totalClaimed.Cmp(metadata.TotalAmount) < 0 {
					stats.PartialClaimedCount++
					stats.TotalClaimed.Add(stats.TotalClaimed, totalClaimed)
				} else if totalClaimed.Cmp(metadata.TotalAmount) >= 0 {
					// 累计领取金额 >= 总金额，应该已被消费，但状态可能未更新
					stats.FullyClaimedCount++
					stats.TotalClaimed.Add(stats.TotalClaimed, metadata.TotalAmount)
				}
			}
		}

		// 判断是否过期：获取当前区块高度并比较
		currentHeight, err := s.chainQuery.GetCurrentHeight(ctx)
		if err == nil && metadata.ExpiryHeight > 0 {
			if currentHeight > metadata.ExpiryHeight {
				stats.ExpiredSponsors++
			} else {
				stats.ActiveSponsors++
			}
		} else if metadata.ExpiryHeight == 0 {
			// 没有过期高度，视为活跃
			stats.ActiveSponsors++
		}
	}

	return stats, nil
}

// 辅助方法：解析领取交易
//
// **用途**：从交易中提取领取信息
func (s *SponsorAuditService) parseClaimTransaction(
	tx *transaction_pb.Transaction,
	sponsorUTXOId *transaction_pb.OutPoint,
) (*ClaimRecord, error) {
	// 1. 检查是否为赞助领取交易
	if len(tx.Inputs) != 1 {
		return nil, fmt.Errorf("不是赞助领取交易")
	}

	delegationProof := tx.Inputs[0].GetDelegationProof()
	if delegationProof == nil {
		return nil, fmt.Errorf("缺少DelegationProof")
	}

	// 2. 提取领取信息
	claimAmount := big.NewInt(int64(delegationProof.ValueAmount))
	minerAddr := delegationProof.DelegateAddress

	// 3. 计算找零金额（如果有Output[1]）
	var changeAmount *big.Int
	if len(tx.Outputs) == 2 {
		changeAsset := tx.Outputs[1].GetAsset()
		if changeAsset != nil {
			changeAmount = s.helper.extractAmount(changeAsset)
		}
	}

	// 4. 计算交易ID（使用确定性序列化 + SHA256）
	txBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("序列化交易失败: %w", err)
	}
	// ✅ 使用 HashManager 接口进行哈希计算（符合架构规范）
	transactionId := s.hashManager.SHA256(txBytes)

	// 5. 构建领取记录
	// 注意：BlockHeight和ClaimTime需要从区块信息获取
	record := &ClaimRecord{
		SponsorUTXOId: append(sponsorUTXOId.TxId, byte(sponsorUTXOId.OutputIndex)),
		MinerAddress:  minerAddr,
		ClaimAmount:   claimAmount,
		ChangeAmount:  changeAmount,
		TransactionId: transactionId,
		// BlockHeight和ClaimTime需要从区块查询获取（调用方负责设置）
	}

	return record, nil
}
