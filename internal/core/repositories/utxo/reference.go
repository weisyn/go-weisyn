package utxo

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ============================================================================
//                           🔄 UTXO引用管理操作实现
// ============================================================================

// referenceUTXO 引用UTXO（增加引用计数）
//
// 🎯 **系统定位**：ResourceUTXO并发控制核心
// 对ResourceUTXO增加引用计数，防止在被引用期间被消费。
// 这是合约执行、资源访问等操作的并发安全保障。
//
// 实现要点：
// - 类型检查：只对ResourceUTXO有效，其他类型UTXO忽略此操作
// - 原子操作：引用计数的增加必须是原子性的
// - 状态验证：检查UTXO是否可以被引用（状态为AVAILABLE）
// - 并发安全：多个goroutine同时引用同一UTXO的安全处理
// - 上限检查：检查是否超过最大并发引用数限制
func (m *Manager) referenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	if m.logger != nil {
		m.logger.Debugf("引用UTXO实现 - txId: %x, index: %d", outpoint.TxId, outpoint.OutputIndex)
	}

	// 1. 验证OutPoint参数
	if outpoint == nil {
		return fmt.Errorf("OutPoint不能为空")
	}
	if len(outpoint.TxId) != 32 {
		return fmt.Errorf("交易哈希长度错误，期望32字节，实际%d字节", len(outpoint.TxId))
	}

	// 2. 查询目标UTXO
	utxoObj, err := m.getUTXO(ctx, outpoint)
	if err != nil {
		return fmt.Errorf("查询UTXO失败: %w", err)
	}
	if utxoObj == nil {
		return fmt.Errorf("UTXO不存在或已消费")
	}

	// 🔥 修正：支持AssetUTXO和ResourceUTXO两种类型的锁定
	if utxoObj.Category == utxo.UTXOCategory_UTXO_CATEGORY_ASSET {
		// AssetUTXO：简单状态变更（不需要引用计数）
		return m.referenceAssetUTXO(ctx, outpoint, utxoObj)
	} else if utxoObj.Category == utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE {
		// ResourceUTXO：使用引用计数机制（原有逻辑）
		return m.referenceResourceUTXO(ctx, outpoint, utxoObj)
	} else {
		if m.logger != nil {
			m.logger.Debugf("不支持的UTXO类型，跳过引用操作 - category: %s", utxoObj.Category.String())
		}
		return nil
	}

}

// referenceAssetUTXO 引用AssetUTXO（简单状态变更）
//
// 🎯 **AssetUTXO锁定核心实现**
//
// 为AssetUTXO提供简单的状态锁定机制，用于交易提交后防止双花。
// 与ResourceUTXO不同，AssetUTXO不需要引用计数，只需要状态变更。
//
// 实现逻辑：
// - AVAILABLE → REFERENCED（锁定状态）
// - 不使用引用计数机制
// - 交易确认后变为CONSUMED，失败时恢复AVAILABLE
//
// 参数：
//   - ctx: 上下文
//   - outpoint: UTXO位置标识
//   - utxoObj: UTXO对象
//
// 返回：
//   - error: 锁定错误
func (m *Manager) referenceAssetUTXO(ctx context.Context, outpoint *transaction.OutPoint, utxoObj *utxo.UTXO) error {
	// 1. 验证UTXO状态（只能对AVAILABLE状态的UTXO加锁）
	if utxoObj.Status != utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE {
		return fmt.Errorf("AssetUTXO状态不可锁定，当前状态: %s", utxoObj.Status.String())
	}

	// 2. 构建UTXO存储键
	utxoKey := formatUTXOKey(outpoint.TxId, outpoint.OutputIndex)

	// 3. 使用事务进行原子状态更新
	return m.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		// 3.1. 在事务内重新获取UTXO数据（防止并发修改）
		currentData, err := tx.Get(utxoKey)
		if err != nil {
			return fmt.Errorf("事务内获取UTXO数据失败: %w", err)
		}
		if currentData == nil {
			return fmt.Errorf("UTXO在事务执行期间已被删除")
		}

		// 3.2. 反序列化当前UTXO对象
		var currentUTXO utxo.UTXO
		if err := proto.Unmarshal(currentData, &currentUTXO); err != nil {
			return fmt.Errorf("反序列化当前UTXO数据失败: %w", err)
		}

		// 3.3. 再次验证UTXO状态（防止并发期间状态变更）
		if currentUTXO.Status != utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE {
			return fmt.Errorf("UTXO状态在事务期间已变更为: %s", currentUTXO.Status.String())
		}

		// 3.4. 更新状态为REFERENCED
		currentUTXO.Status = utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED

		// 3.5. 序列化更新后的UTXO对象
		updatedData, err := proto.Marshal(&currentUTXO)
		if err != nil {
			return fmt.Errorf("序列化更新的UTXO失败: %w", err)
		}

		// 3.6. 在事务内写入更新的UTXO数据
		if err := tx.Set(utxoKey, updatedData); err != nil {
			return fmt.Errorf("事务内更新UTXO数据失败: %w", err)
		}

		if m.logger != nil {
			m.logger.Debugf("AssetUTXO锁定成功 - txId: %x, index: %d, 状态: AVAILABLE → REFERENCED",
				outpoint.TxId, outpoint.OutputIndex)
		}

		return nil
	})
}

// referenceResourceUTXO 引用ResourceUTXO（引用计数机制）
//
// 🎯 **ResourceUTXO引用计数核心实现**
//
// 对ResourceUTXO增加引用计数，支持并发访问控制。
// 这是原有的ResourceUTXO引用逻辑，保持不变。
//
// 参数：
//   - ctx: 上下文
//   - outpoint: UTXO位置标识
//   - utxoObj: UTXO对象
//
// 返回：
//   - error: 引用错误
func (m *Manager) referenceResourceUTXO(ctx context.Context, outpoint *transaction.OutPoint, utxoObj *utxo.UTXO) error {
	// 1. 验证UTXO状态（必须是AVAILABLE或REFERENCED）
	if utxoObj.Status == utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED {
		return fmt.Errorf("UTXO已被消费，无法引用")
	}

	// 2. 获取ResourceUTXO约束
	resourceConstraints := utxoObj.GetResourceConstraints()
	if resourceConstraints == nil {
		// 如果没有资源约束，创建默认约束
		resourceConstraints = &utxo.ResourceUTXOConstraints{
			ReferenceCount: 0,
		}
		utxoObj.TypeSpecificConstraints = &utxo.UTXO_ResourceConstraints{
			ResourceConstraints: resourceConstraints,
		}
	}

	// 3. 检查最大并发引用数限制（如果设置）
	if resourceConstraints.MaxConcurrentReferences != nil &&
		*resourceConstraints.MaxConcurrentReferences > 0 &&
		resourceConstraints.ReferenceCount >= *resourceConstraints.MaxConcurrentReferences {
		return fmt.Errorf("已达到最大并发引用数限制: %d", *resourceConstraints.MaxConcurrentReferences)
	}

	// 4. 原子性更新引用计数和状态
	err := m.atomicUpdateReferenceCount(ctx, outpoint, utxoObj, 1)
	if err != nil {
		return fmt.Errorf("原子更新引用计数失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("ResourceUTXO引用成功 - txId: %x, index: %d, 新引用计数: %d",
			outpoint.TxId, outpoint.OutputIndex, resourceConstraints.ReferenceCount+1)
	}

	return nil
}

// unreferenceUTXO 解除UTXO引用（减少引用计数）
//
// 🎯 **系统定位**：ResourceUTXO引用完成后的清理
// 对ResourceUTXO减少引用计数，当引用计数归零时允许被消费。
// 这是合约执行、资源访问完成后的必要清理操作。
//
// 实现要点：
// - 类型检查：只对ResourceUTXO有效，其他类型UTXO忽略此操作
// - 原子操作：引用计数的减少必须是原子性的
// - 状态管理：当引用计数归零时，状态从REFERENCED回到AVAILABLE
// - 并发安全：多个goroutine同时解除引用的安全处理
// - 边界检查：防止引用计数减少到负数
func (m *Manager) unreferenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	if m.logger != nil {
		m.logger.Debugf("解除UTXO引用实现 - txId: %x, index: %d", outpoint.TxId, outpoint.OutputIndex)
	}

	// 1. 验证OutPoint参数
	if outpoint == nil {
		return fmt.Errorf("OutPoint不能为空")
	}
	if len(outpoint.TxId) != 32 {
		return fmt.Errorf("交易哈希长度错误，期望32字节，实际%d字节", len(outpoint.TxId))
	}

	// 2. 查询目标UTXO
	utxoObj, err := m.getUTXO(ctx, outpoint)
	if err != nil {
		return fmt.Errorf("查询UTXO失败: %w", err)
	}
	if utxoObj == nil {
		return fmt.Errorf("UTXO不存在或已消费")
	}

	// 3. 验证UTXO类型（只有ResourceUTXO需要引用计数）
	if utxoObj.Category != utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE {
		if m.logger != nil {
			m.logger.Debugf("非ResourceUTXO类型，跳过解除引用操作 - category: %s", utxoObj.Category.String())
		}
		return nil // 非ResourceUTXO直接返回成功
	}

	// 4. 获取ResourceUTXO约束
	resourceConstraints := utxoObj.GetResourceConstraints()
	if resourceConstraints == nil {
		return fmt.Errorf("ResourceUTXO缺少必要的约束信息")
	}

	// 5. 验证当前引用计数（必须 > 0）
	if resourceConstraints.ReferenceCount == 0 {
		if m.logger != nil {
			m.logger.Warnf("UTXO引用计数已为0，无需解除引用 - txId: %x, index: %d", outpoint.TxId, outpoint.OutputIndex)
		}
		return nil // 引用计数已为0，直接返回成功
	}

	// 6. 原子性更新引用计数和状态
	err = m.atomicUpdateReferenceCount(ctx, outpoint, utxoObj, -1)
	if err != nil {
		return fmt.Errorf("原子更新引用计数失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("UTXO解除引用成功 - txId: %x, index: %d, 新引用计数: %d",
			outpoint.TxId, outpoint.OutputIndex, resourceConstraints.ReferenceCount-1)
	}

	return nil
}

// ============================================================================
//                           🔧 原子性引用计数更新
// ============================================================================

// atomicUpdateReferenceCount 原子性更新UTXO引用计数
//
// 🎯 **原子性保障**：使用BadgerStore事务机制确保引用计数更新的原子性
// 这是ResourceUTXO并发控制的核心实现，必须保证在高并发场景下的数据一致性。
//
// 参数：
//   - ctx: 上下文
//   - outpoint: UTXO位置标识
//   - utxoObj: UTXO对象（调用前已验证）
//   - delta: 引用计数变更量（+1为引用，-1为解除引用）
//
// 返回：
//   - error: 更新错误
func (m *Manager) atomicUpdateReferenceCount(ctx context.Context, outpoint *transaction.OutPoint, utxoObj *utxo.UTXO, delta int64) error {
	// 构建UTXO存储键
	utxoKey := formatUTXOKey(outpoint.TxId, outpoint.OutputIndex)

	// 使用BadgerStore事务进行原子性更新
	return m.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		// 1. 在事务内重新获取UTXO数据（防止并发修改）
		currentData, err := tx.Get(utxoKey)
		if err != nil {
			return fmt.Errorf("事务内获取UTXO数据失败: %w", err)
		}
		if currentData == nil {
			return fmt.Errorf("UTXO在事务执行期间已被删除")
		}

		// 2. 反序列化当前UTXO对象
		var currentUTXO utxo.UTXO
		if err := proto.Unmarshal(currentData, &currentUTXO); err != nil {
			return fmt.Errorf("反序列化当前UTXO数据失败: %w", err)
		}

		// 3. 再次验证UTXO类型和状态（防止并发期间状态变更）
		if currentUTXO.Category != utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE {
			return fmt.Errorf("UTXO类型在事务期间已变更")
		}
		if currentUTXO.Status == utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_CONSUMED {
			return fmt.Errorf("UTXO在事务期间已被消费")
		}

		// 4. 获取或创建ResourceUTXO约束
		resourceConstraints := currentUTXO.GetResourceConstraints()
		if resourceConstraints == nil {
			resourceConstraints = &utxo.ResourceUTXOConstraints{
				ReferenceCount: 0,
			}
			currentUTXO.TypeSpecificConstraints = &utxo.UTXO_ResourceConstraints{
				ResourceConstraints: resourceConstraints,
			}
		}

		// 5. 计算新的引用计数
		newReferenceCount := int64(resourceConstraints.ReferenceCount) + delta
		if newReferenceCount < 0 {
			return fmt.Errorf("引用计数不能为负数，当前计数: %d, 变更量: %d", resourceConstraints.ReferenceCount, delta)
		}

		// 6. 检查并发引用数限制（仅在增加引用时检查）
		if delta > 0 {
			if resourceConstraints.MaxConcurrentReferences != nil &&
				*resourceConstraints.MaxConcurrentReferences > 0 &&
				uint64(newReferenceCount) > *resourceConstraints.MaxConcurrentReferences {
				return fmt.Errorf("超过最大并发引用数限制: %d", *resourceConstraints.MaxConcurrentReferences)
			}
		}

		// 7. 更新引用计数
		resourceConstraints.ReferenceCount = uint64(newReferenceCount)

		// 8. 更新历史总引用次数统计（可选）
		if delta > 0 {
			if resourceConstraints.TotalReferenceCount == nil {
				zero := uint64(0)
				resourceConstraints.TotalReferenceCount = &zero
			}
			*resourceConstraints.TotalReferenceCount++
		}

		// 9. 根据引用计数更新UTXO状态
		if resourceConstraints.ReferenceCount > 0 {
			// 有引用时，状态为REFERENCED
			currentUTXO.Status = utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_REFERENCED
		} else {
			// 无引用时，状态为AVAILABLE
			currentUTXO.Status = utxo.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE
		}

		// 10. 序列化更新后的UTXO对象
		updatedData, err := proto.Marshal(&currentUTXO)
		if err != nil {
			return fmt.Errorf("序列化更新的UTXO失败: %w", err)
		}

		// 11. 在事务内写入更新的UTXO数据
		if err := tx.Set(utxoKey, updatedData); err != nil {
			return fmt.Errorf("事务内更新UTXO数据失败: %w", err)
		}

		if m.logger != nil {
			m.logger.Debugf("原子更新引用计数完成 - txId: %x, index: %d, 原计数: %d, 新计数: %d, 状态: %s",
				outpoint.TxId, outpoint.OutputIndex,
				resourceConstraints.ReferenceCount-uint64(delta), resourceConstraints.ReferenceCount,
				currentUTXO.Status.String())
		}

		return nil
	})
}
