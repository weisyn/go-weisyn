package builder

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo_pb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/constants"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// SponsorUTXOHelper 赞助UTXO辅助工具
//
// 🎯 **核心职责**：基于EUTXO系统提供赞助UTXO的识别、元数据提取和生命周期管理
//
// 💡 **设计理念**：
// - 严格遵循EUTXO原则：所有数据来源于UTXO本身
// - 不创建新的存储结构，通过查询和计算获取信息
// - 提供统一的辅助接口，简化赞助UTXO的使用
type SponsorUTXOHelper struct {
	eutxoQuery persistence.UTXOQuery
}

// NewSponsorUTXOHelper 创建赞助UTXO辅助工具
func NewSponsorUTXOHelper(eutxoQuery persistence.UTXOQuery) *SponsorUTXOHelper {
	return &SponsorUTXOHelper{
		eutxoQuery: eutxoQuery,
	}
}

// SponsorMetadata 赞助UTXO的元数据
//
// **设计说明**（基于架构分析文档）：
// - 元数据通过查询和计算获得，不存储在UTXO中
// - 从DelegationLock配置和UTXO属性推断元数据
// - 用于查询、展示和审计，不参与验证逻辑
type SponsorMetadata struct {
	// 赞助方信息（从UTXO推断）
	SponsorAddress []byte   // 通常无法直接获取，可能为nil
	TokenType      string   // 代币类型（native/contract:xxx:yyy）
	TotalAmount    *big.Int // 总金额（从AssetOutput提取）

	// 限制条件（从DelegationLock提取）
	MaxPerClaim  *big.Int // 单次最大领取金额（DelegationLock.MaxValuePerOperation）
	ExpiryHeight uint64   // 过期高度（从DelegationLock.ExpiryDurationBlocks计算）

	// UTXO信息
	CreationHeight uint64                      // 创建高度（UTXO.block_height）
	CreationTime   uint64                      // 创建时间（UTXO.created_timestamp）
	CurrentStatus  utxo_pb.UTXOLifecycleStatus // 当前状态（UTXO.status）

	// 描述信息（通常无法获取，留空）
	Description string
	Purpose     string
}

// SponsorLifecycleState 赞助UTXO的业务生命周期状态
//
// **设计说明**：
// - 基于UTXO的status和查询接口计算
// - 不同于UTXOLifecycleStatus，这是业务层面的状态
type SponsorLifecycleState string

const (
	// SponsorStateCreated 已创建（刚上链，AVAILABLE状态）
	SponsorStateCreated SponsorLifecycleState = "created"

	// SponsorStateActive 活跃中（AVAILABLE状态，可领取）
	SponsorStateActive SponsorLifecycleState = "active"

	// SponsorStatePartialClaimed 部分领取（有找零回池，UTXO仍存在）
	SponsorStatePartialClaimed SponsorLifecycleState = "partial_claimed"

	// SponsorStateFullyClaimed 全部领取（UTXO已被消费，CONSUMED状态）
	SponsorStateFullyClaimed SponsorLifecycleState = "fully_claimed"

	// SponsorStateExpired 已过期（基于ExpiryHeight计算）
	SponsorStateExpired SponsorLifecycleState = "expired"

	// SponsorStateUnknown 未知状态
	SponsorStateUnknown SponsorLifecycleState = "unknown"
)

// IsSponsorUTXO 判断UTXO是否为赞助UTXO
//
// **判断标准**：
// - Owner = SponsorPoolOwner
// - 有DelegationLock锁定条件
func (h *SponsorUTXOHelper) IsSponsorUTXO(utxo *utxo_pb.UTXO) bool {
	if utxo == nil {
		return false
	}

	output := utxo.GetCachedOutput()
	if output == nil {
		return false
	}

	// 检查Owner
	if !bytes.Equal(output.Owner, constants.SponsorPoolOwner[:]) {
		return false
	}

	// 检查是否有DelegationLock
	for _, lock := range output.LockingConditions {
		if lock.GetDelegationLock() != nil {
			return true
		}
	}

	return false
}

// ExtractMetadata 从UTXO提取赞助元数据
//
// **提取策略**：
// - 从DelegationLock配置提取限制条件
// - 从AssetOutput提取代币类型和金额
// - 从UTXO属性提取创建信息
func (h *SponsorUTXOHelper) ExtractMetadata(utxo *utxo_pb.UTXO) (*SponsorMetadata, error) {
	if !h.IsSponsorUTXO(utxo) {
		return nil, fmt.Errorf("不是赞助UTXO")
	}

	output := utxo.GetCachedOutput()
	if output == nil {
		return nil, fmt.Errorf("UTXO缺少CachedOutput")
	}

	// 提取DelegationLock
	var delegationLock *transaction_pb.DelegationLock
	for _, lock := range output.LockingConditions {
		if dl := lock.GetDelegationLock(); dl != nil {
			delegationLock = dl
			break
		}
	}
	if delegationLock == nil {
		return nil, fmt.Errorf("赞助UTXO缺少DelegationLock")
	}

	// 提取AssetOutput信息
	asset := output.GetAsset()
	if asset == nil {
		return nil, fmt.Errorf("赞助UTXO必须是资产输出")
	}

	// 提取代币类型和金额
	tokenType := h.extractTokenType(asset)
	totalAmount := h.extractAmount(asset)

	// 构建元数据
	metadata := &SponsorMetadata{
		SponsorAddress: nil, // 无法从UTXO直接获取
		TokenType:      tokenType,
		TotalAmount:    totalAmount,
		MaxPerClaim:    big.NewInt(int64(delegationLock.MaxValuePerOperation)),
		CreationHeight: utxo.BlockHeight,
		CreationTime:   utxo.CreatedTimestamp,
		CurrentStatus:  utxo.Status,
		Description:    "", // 无法从UTXO获取
		Purpose:        "", // 无法从UTXO获取
	}

	// 计算过期高度（如果有ExpiryDurationBlocks）
	if delegationLock.ExpiryDurationBlocks != nil {
		metadata.ExpiryHeight = utxo.BlockHeight + *delegationLock.ExpiryDurationBlocks
	}

	return metadata, nil
}

// GetLifecycleState 获取赞助UTXO的生命周期状态
//
// **状态计算逻辑**：
// - Created: AVAILABLE状态，刚创建
// - Active: AVAILABLE状态，未过期
// - PartialClaimed: AVAILABLE状态，但金额可能已部分领取（需要通过查询历史计算）
// - FullyClaimed: CONSUMED状态
// - Expired: 基于ExpiryHeight计算
func (h *SponsorUTXOHelper) GetLifecycleState(
	ctx context.Context,
	utxo *utxo_pb.UTXO,
	currentHeight uint64,
) (SponsorLifecycleState, error) {
	if !h.IsSponsorUTXO(utxo) {
		return SponsorStateUnknown, fmt.Errorf("不是赞助UTXO")
	}

	metadata, err := h.ExtractMetadata(utxo)
	if err != nil {
		return SponsorStateUnknown, fmt.Errorf("提取元数据失败: %w", err)
	}

	// 检查是否已消费
	if utxo.Status == utxo_pb.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED {
		return SponsorStateFullyClaimed, nil
	}

	// 检查是否过期
	if metadata.ExpiryHeight > 0 && currentHeight > metadata.ExpiryHeight {
		return SponsorStateExpired, nil
	}

	// 检查是否刚创建（可选：可以根据创建时间判断）
	if utxo.Status == utxo_pb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE {
		// 尝试判断是否部分领取：通过UTXO当前金额判断
		// 注意：完整实现需要查询历史交易，当前通过UTXO当前金额判断
		cachedOutput := utxo.GetCachedOutput()
		if cachedOutput != nil {
			assetOutput := cachedOutput.GetAsset()
			if assetOutput != nil {
				currentAmount := h.extractAmount(assetOutput)
				if currentAmount != nil && currentAmount.Cmp(metadata.TotalAmount) < 0 && currentAmount.Sign() > 0 {
					// 当前金额 < 总金额 且 > 0，可能已部分领取
					// 但更准确的判断需要查询历史交易（需要扩展TxQuery接口）
					return SponsorStatePartialClaimed, nil
				}
			}
		}
		return SponsorStateActive, nil
	}

	return SponsorStateUnknown, nil
}

// ValidateSponsorUTXO 验证赞助UTXO是否符合标准结构
//
// **验证内容**：
// - Owner = SponsorPoolOwner
// - 有DelegationLock
// - DelegationLock授权consume操作
// - 是AssetOutput
func (h *SponsorUTXOHelper) ValidateSponsorUTXO(utxo *utxo_pb.UTXO) error {
	if !h.IsSponsorUTXO(utxo) {
		return fmt.Errorf("不是赞助UTXO")
	}

	output := utxo.GetCachedOutput()
	if output == nil {
		return fmt.Errorf("UTXO缺少CachedOutput")
	}

	// 验证Owner
	if !bytes.Equal(output.Owner, constants.SponsorPoolOwner[:]) {
		return fmt.Errorf("Owner必须是SponsorPoolOwner")
	}

	// 验证有DelegationLock
	var delegationLock *transaction_pb.DelegationLock
	for _, lock := range output.LockingConditions {
		if dl := lock.GetDelegationLock(); dl != nil {
			delegationLock = dl
			break
		}
	}
	if delegationLock == nil {
		return fmt.Errorf("缺少DelegationLock")
	}

	// 验证授权consume操作
	hasConsume := false
	for _, op := range delegationLock.AuthorizedOperations {
		if op == "consume" {
			hasConsume = true
			break
		}
	}
	if !hasConsume {
		return fmt.Errorf("DelegationLock未授权consume操作")
	}

	// 验证是AssetOutput
	if output.GetAsset() == nil {
		return fmt.Errorf("必须是AssetOutput")
	}

	return nil
}

// 辅助方法

func (h *SponsorUTXOHelper) extractTokenType(asset *transaction_pb.AssetOutput) string {
	if nc := asset.GetNativeCoin(); nc != nil {
		return "native"
	}
	if ct := asset.GetContractToken(); ct != nil {
		contractAddr := fmt.Sprintf("%x", ct.ContractAddress)
		switch ti := ct.TokenIdentifier.(type) {
		case *transaction_pb.ContractTokenAsset_FungibleClassId:
			return fmt.Sprintf("contract:%s:%x", contractAddr, ti.FungibleClassId)
		case *transaction_pb.ContractTokenAsset_NftUniqueId:
			return fmt.Sprintf("contract:%s:nft:%x", contractAddr, ti.NftUniqueId)
		case *transaction_pb.ContractTokenAsset_SemiFungibleId:
			return fmt.Sprintf("contract:%s:sft:%x:%d", contractAddr, ti.SemiFungibleId.BatchId, ti.SemiFungibleId.InstanceId)
		}
	}
	return "unknown"
}

func (h *SponsorUTXOHelper) extractAmount(asset *transaction_pb.AssetOutput) *big.Int {
	var amountStr string
	if nc := asset.GetNativeCoin(); nc != nil {
		amountStr = nc.Amount
	} else if ct := asset.GetContractToken(); ct != nil {
		amountStr = ct.Amount
	} else {
		return big.NewInt(0)
	}

	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		return big.NewInt(0)
	}
	return amount
}
