package writer

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
)

// ValidateUTXO 验证 UTXO 对象的有效性
//
// 实现 interfaces.InternalUTXOWriter.ValidateUTXO
func (s *Service) ValidateUTXO(ctx context.Context, utxoObj *utxo.UTXO) error {
	// 1. 验证 UTXO 对象不为空
	if utxoObj == nil {
		return fmt.Errorf("UTXO 对象不能为空")
	}

	// 2. 验证 OutPoint
	if err := s.validateOutPoint(utxoObj.Outpoint); err != nil {
		return fmt.Errorf("无效的 OutPoint: %w", err)
	}

	// 3. 验证 Output (如果有缓存)
	if cachedOutput := utxoObj.GetCachedOutput(); cachedOutput != nil {
		if err := s.validateOutput(cachedOutput); err != nil {
			return fmt.Errorf("无效的 Output: %w", err)
		}
	}

	// 4. 验证区块高度
	//
	// 约束：
	// - 正常区块的 UTXO 高度必须 >= 1
	// - 特殊场景允许高度为 0：
	//   a) 创世块 UTXO 创建（通过 context 标记 "genesis_utxo_allowed"）
	//   b) 快照恢复模式（通过 context 标记 "snapshot_restore_mode"）
	if utxoObj.BlockHeight == 0 {
		// 检查是否为允许的特殊场景
		isGenesisContext := ctx.Value("genesis_utxo_allowed")
		isSnapshotContext := ctx.Value("snapshot_restore_mode")
		
		if isGenesisContext == nil && isSnapshotContext == nil {
			// 非特殊场景，拒绝高度为 0 的 UTXO
			return fmt.Errorf("区块高度不能为0（非创世或快照恢复场景）")
		}
		
		// 特殊场景允许通过，但不记录日志以避免噪音
		// 创世块和快照恢复是正常的系统操作
	}

	return nil
}

// validateOutPoint 验证 OutPoint
func (s *Service) validateOutPoint(outpoint *transaction.OutPoint) error {
	if outpoint == nil {
		return fmt.Errorf("OutPoint 不能为空")
	}

	if outpoint.TxId == nil || len(outpoint.TxId) != 32 {
		return fmt.Errorf("交易哈希长度必须为32字节")
	}

	// 索引可以是任意值，包括 0

	return nil
}

// validateOutput 验证 Output（P3-23：完整的输出验证逻辑）
func (s *Service) validateOutput(output *transaction.TxOutput) error {
	if output == nil {
		return fmt.Errorf("Output 不能为空")
	}

	// 1. 验证锁定条件（相当于 ScriptPubKey 验证）
	if len(output.LockingConditions) == 0 {
		return fmt.Errorf("输出必须至少包含一个锁定条件")
	}
	for i, condition := range output.LockingConditions {
		if condition == nil {
			return fmt.Errorf("锁定条件%d不能为空", i)
		}
		// 验证锁定条件类型（至少有一个有效的锁定条件）
		if condition.GetSingleKeyLock() == nil &&
			condition.GetMultiKeyLock() == nil &&
			condition.GetContractLock() == nil &&
			condition.GetDelegationLock() == nil &&
			condition.GetThresholdLock() == nil &&
			condition.GetTimeLock() == nil &&
			condition.GetHeightLock() == nil {
			return fmt.Errorf("锁定条件%d无效：未指定任何锁定类型", i)
		}
	}

	// 2. 验证输出类型（Asset/Resource/State）- 必须有且仅有一个类型
	outputTypeCount := 0
	if output.GetAsset() != nil {
		outputTypeCount++
	}
	if output.GetState() != nil {
		outputTypeCount++
	}
	if output.GetResource() != nil {
		outputTypeCount++
	}

	if outputTypeCount == 0 {
		return fmt.Errorf("输出必须指定一个输出类型（Asset/Resource/State）")
	}
	if outputTypeCount > 1 {
		return fmt.Errorf("输出只能指定一个输出类型，当前指定了%d个", outputTypeCount)
	}

	// 3. 根据输出类型进行详细验证
	if asset := output.GetAsset(); asset != nil {
		if err := s.validateAssetOutput(asset); err != nil {
			return fmt.Errorf("资产输出验证失败: %w", err)
		}
	} else if state := output.GetState(); state != nil {
		if err := s.validateStateOutput(state); err != nil {
			return fmt.Errorf("状态输出验证失败: %w", err)
		}
	} else if resource := output.GetResource(); resource != nil {
		if err := s.validateResourceOutput(resource); err != nil {
			return fmt.Errorf("资源输出验证失败: %w", err)
		}
	}

	return nil
}

// validateAssetOutput 验证资产输出（P3-23：资产 ID 验证）
func (s *Service) validateAssetOutput(asset *transaction.AssetOutput) error {
	if asset == nil {
		return fmt.Errorf("资产输出不能为空")
	}

	// 验证资产类型（NativeCoin 或 ContractToken）
	assetTypeCount := 0
	if asset.GetNativeCoin() != nil {
		assetTypeCount++
	}
	if asset.GetContractToken() != nil {
		assetTypeCount++
	}

	if assetTypeCount == 0 {
		return fmt.Errorf("资产输出必须指定一个资产类型（NativeCoin 或 ContractToken）")
	}
	if assetTypeCount > 1 {
		return fmt.Errorf("资产输出只能指定一个资产类型，当前指定了%d个", assetTypeCount)
	}

	// 验证原生代币
	if nativeCoin := asset.GetNativeCoin(); nativeCoin != nil {
		if nativeCoin.Amount == "" || nativeCoin.Amount == "0" {
			return fmt.Errorf("原生代币金额不能为空或0")
		}
		// 可以添加金额格式验证（如必须为正数、格式正确等）
	}

	// 验证合约代币（P3-23：资产 ID 验证）
	if contractToken := asset.GetContractToken(); contractToken != nil {
		// 验证合约地址
		if len(contractToken.ContractAddress) == 0 {
			return fmt.Errorf("合约代币必须指定合约地址")
		}
		if len(contractToken.ContractAddress) != 20 {
			return fmt.Errorf("合约地址长度必须为20字节，实际长度: %d", len(contractToken.ContractAddress))
		}

		// 验证代币标识符（资产 ID）
		tokenIdCount := 0
		if contractToken.GetFungibleClassId() != nil {
			tokenIdCount++
		}
		if contractToken.GetNftUniqueId() != nil {
			tokenIdCount++
		}
		if contractToken.GetSemiFungibleId() != nil {
			tokenIdCount++
		}

		if tokenIdCount == 0 {
			return fmt.Errorf("合约代币必须指定一个代币标识符（FungibleClassId/NftUniqueId/SemiFungibleId）")
		}
		if tokenIdCount > 1 {
			return fmt.Errorf("合约代币只能指定一个代币标识符，当前指定了%d个", tokenIdCount)
		}

		// 验证代币标识符内容
		if fungibleClassId := contractToken.GetFungibleClassId(); fungibleClassId != nil {
			if len(fungibleClassId) == 0 {
				return fmt.Errorf("同质化代币类别ID不能为空")
			}
		} else if nftUniqueId := contractToken.GetNftUniqueId(); nftUniqueId != nil {
			if len(nftUniqueId) == 0 {
				return fmt.Errorf("NFT唯一标识符不能为空")
			}
		} else if semiFungibleId := contractToken.GetSemiFungibleId(); semiFungibleId != nil {
			if len(semiFungibleId.BatchId) == 0 {
				return fmt.Errorf("半同质化代币批次ID不能为空")
			}
		}

		// 验证数量
		if contractToken.Amount == "" || contractToken.Amount == "0" {
			return fmt.Errorf("合约代币数量不能为空或0")
		}
	}

	return nil
}

// validateStateOutput 验证状态输出（P3-23：输出类型验证）
func (s *Service) validateStateOutput(state *transaction.StateOutput) error {
	if state == nil {
		return fmt.Errorf("状态输出不能为空")
	}

	// 验证状态ID
	if len(state.StateId) == 0 {
		return fmt.Errorf("状态ID不能为空")
	}

	// 验证零知识证明
	if state.ZkProof == nil {
		return fmt.Errorf("状态输出必须包含零知识证明")
	}

	return nil
}

// validateResourceOutput 验证资源输出（P3-23：输出类型验证）
func (s *Service) validateResourceOutput(resource *transaction.ResourceOutput) error {
	if resource == nil {
		return fmt.Errorf("资源输出不能为空")
	}

	// 验证资源定义
	if resource.Resource == nil {
		return fmt.Errorf("资源输出必须包含资源定义")
	}

	// 验证资源哈希（通过 resource.Resource.ContentHash 获取）
	if len(resource.Resource.ContentHash) == 0 {
		return fmt.Errorf("资源哈希不能为空")
	}
	if len(resource.Resource.ContentHash) != 32 {
		return fmt.Errorf("资源哈希长度必须为32字节，实际长度: %d", len(resource.Resource.ContentHash))
	}

	return nil
}
