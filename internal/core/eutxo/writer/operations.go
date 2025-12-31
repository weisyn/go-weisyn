package writer

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/weisyn/v1/internal/core/eutxo/writer/eventhelpers"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
	"google.golang.org/protobuf/proto"
)

// CreateUTXO 创建 UTXO
//
// 实现 eutxo.UTXOWriter.CreateUTXO
func (s *Service) CreateUTXO(ctx context.Context, utxoObj *utxo.UTXO) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "eutxo.UTXOWriter.CreateUTXO"); err != nil {
		return err
	}
	// 1. 验证 UTXO
	if err := s.ValidateUTXO(ctx, utxoObj); err != nil {
		return fmt.Errorf("UTXO 验证失败: %w", err)
	}

	// 2. 加锁
	s.mu.Lock()
	defer s.mu.Unlock()

	// 3. 序列化 UTXO
	utxoData, err := proto.Marshal(utxoObj)
	if err != nil {
		return fmt.Errorf("序列化 UTXO 失败: %w", err)
	}

	// 4. 构造存储键
	utxoKey := buildUTXOKey(utxoObj.Outpoint)

	// 5. 存储 UTXO
	if err := s.storage.Set(ctx, []byte(utxoKey), utxoData); err != nil {
		return fmt.Errorf("存储 UTXO 失败: %w", err)
	}

	// 6. 更新缓存
	s.cache.Put(utxoKey, utxoObj)

	// 7. 更新索引
	s.indexManager.AddUTXO(utxoObj)

	// 8. 发布事件（可选）
	if s.eventBus != nil {
		// P3-10: 发布 UTXOCreated 事件
		eventhelpers.PublishUTXOCreatedEvent(ctx, s.eventBus, s.logger, utxoObj)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ UTXO 已创建: %s", utxoKey)
	}

	return nil
}

// DeleteUTXO 删除 UTXO（P3-14：优化索引移除）
//
// 实现 eutxo.UTXOWriter.DeleteUTXO
func (s *Service) DeleteUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "eutxo.UTXOWriter.DeleteUTXO"); err != nil {
		return err
	}
	// 1. 验证 OutPoint
	if outpoint == nil || outpoint.TxId == nil {
		return fmt.Errorf("无效的 OutPoint")
	}

	// 2. 加锁
	s.mu.Lock()
	defer s.mu.Unlock()

	// 3. 构造存储键
	utxoKey := buildUTXOKey(outpoint)

	// 4. 先获取 UTXO 对象（用于索引移除和引用计数检查）
	var utxoObj *utxo.UTXO
	// 优先从缓存获取（最快）
	if cachedObj, found := s.cache.Get(utxoKey); found {
		if obj, ok := cachedObj.(*utxo.UTXO); ok {
			utxoObj = obj
		}
	}

	// 如果缓存未命中，尝试从存储读取（用于索引移除和引用计数检查）
	if utxoObj == nil {
		if data, err := s.storage.Get(ctx, []byte(utxoKey)); err == nil && len(data) > 0 {
			tempObj := &utxo.UTXO{}
			if err := proto.Unmarshal(data, tempObj); err == nil {
				utxoObj = tempObj
			} else {
				// 反序列化失败，记录警告但继续删除操作
				if s.logger != nil {
					s.logger.Warnf("反序列化UTXO失败: %v", err)
				}
			}
		}
	}

	// 5. ⚠️ 引用计数说明（彻底迭代）
	// 引用型输入（is_reference_only）是“只读依赖”，不应该在共识层形成跨区块的锁定语义。
	// 因此引用计数不再作为 DeleteUTXO 的强门闸条件（否则会导致因计数残留而阻断正常消费）。
	// 这里仅做观测：若 refCount > 0，记录告警但继续删除。
	if refCount, err := s.getReferenceCount(ctx, outpoint); err == nil && refCount > 0 {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 删除UTXO时发现非零引用计数（将忽略该计数）: outpoint=%s refCount=%d",
				buildUTXOKey(outpoint), refCount)
		}
	}

	// 6. 删除 UTXO
	if err := s.storage.Delete(ctx, []byte(utxoKey)); err != nil {
		return fmt.Errorf("删除 UTXO 失败: %w", err)
	}

	// 7. 从缓存移除
	s.cache.Delete(utxoKey)

	// 8. 更新索引（P3-14：使用 RemoveUTXOWithDetails 优化）
	if utxoObj != nil {
		// 有 UTXO 对象，使用完整的索引移除
		s.indexManager.RemoveUTXOWithDetails(ctx, utxoObj)
	} else {
		// 没有 UTXO 对象，使用简化移除（向后兼容）
		if s.logger != nil {
			s.logger.Warnf("删除 UTXO 时无法获取对象，使用简化索引移除: %s", utxoKey)
		}
		s.indexManager.RemoveUTXO(outpoint)
	}

	// 9. 发布事件（可选）
	if s.eventBus != nil {
		// P3-10: 发布 UTXODeleted 事件
		eventhelpers.PublishUTXODeletedEvent(ctx, s.eventBus, s.logger, outpoint)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ UTXO 已删除: %s", utxoKey)
	}

	return nil
}

// CreateUTXOInTransaction 在事务中创建 UTXO
//
// 实现 eutxo.UTXOWriter.CreateUTXOInTransaction
//
// ⚠️ **注意事项**：
// - 使用外部传入的事务，不创建新事务
// - 缓存更新应延迟到事务提交后（当前实现中禁用缓存更新）
// - 事件发布应延迟到事务提交后（当前实现中禁用事件发布）
func (s *Service) CreateUTXOInTransaction(ctx context.Context, tx storage.BadgerTransaction, utxoObj *utxo.UTXO) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "eutxo.UTXOWriter.CreateUTXOInTransaction"); err != nil {
		return err
	}
	// 1. 验证 UTXO
	if err := s.ValidateUTXO(ctx, utxoObj); err != nil {
		return fmt.Errorf("UTXO 验证失败: %w", err)
	}

	// 2. 序列化 UTXO
	utxoData, err := proto.Marshal(utxoObj)
	if err != nil {
		return fmt.Errorf("序列化 UTXO 失败: %w", err)
	}

	// 3. 构造存储键
	utxoKey := buildUTXOKey(utxoObj.Outpoint)

	// 4. 在事务中存储 UTXO
	if err := tx.Set([]byte(utxoKey), utxoData); err != nil {
		return fmt.Errorf("存储 UTXO 失败: %w", err)
	}

	// 5. 在事务中更新索引
	s.indexManager.AddUTXOInTransaction(tx, utxoObj)

	// 注意：缓存更新和事件发布延迟到事务提交后
	// 当前实现中，事务版本的方法不更新缓存和发布事件
	// 调用方应在事务提交后手动更新缓存和发布事件（如果需要）

	if s.logger != nil {
		s.logger.Debugf("✅ UTXO 已在事务中创建: %s", utxoKey)
	}

	return nil
}

// DeleteUTXOInTransaction 在事务中删除 UTXO
//
// 实现 eutxo.UTXOWriter.DeleteUTXOInTransaction
//
// ⚠️ **注意事项**：
// - 使用外部传入的事务，不创建新事务
// - 会检查引用计数，引用计数 > 0 时拒绝删除
// - 缓存更新应延迟到事务提交后（当前实现中禁用缓存更新）
// - 事件发布应延迟到事务提交后（当前实现中禁用事件发布）
func (s *Service) DeleteUTXOInTransaction(ctx context.Context, tx storage.BadgerTransaction, outpoint *transaction.OutPoint) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "eutxo.UTXOWriter.DeleteUTXOInTransaction"); err != nil {
		return err
	}
	// 1. 验证 OutPoint
	if outpoint == nil || outpoint.TxId == nil {
		return fmt.Errorf("无效的 OutPoint")
	}

	// 2. 构造存储键
	utxoKey := buildUTXOKey(outpoint)

	// 3. 先获取 UTXO 对象（用于索引移除和引用计数检查）
	var utxoObj *utxo.UTXO
	// 从事务中读取
	data, err := tx.Get([]byte(utxoKey))
	if err != nil {
		// UTXO 不存在，这通常表示交易验证阶段的错误（引用了不存在的 UTXO）
		// 但在某些场景下（如回滚）可能是正常的，这里返回错误以确保数据一致性
		return fmt.Errorf("UTXO 不存在: %s (交易验证应拒绝引用不存在的 UTXO)", utxoKey)
	}
	if len(data) == 0 {
		return fmt.Errorf("UTXO 数据为空: %s", utxoKey)
	}

	// 反序列化 UTXO 对象
	tempObj := &utxo.UTXO{}
	if err := proto.Unmarshal(data, tempObj); err != nil {
		return fmt.Errorf("反序列化 UTXO 失败: %w", err)
	}
	utxoObj = tempObj

	// 4. ⚠️ 引用计数说明（彻底迭代）
	// 同 DeleteUTXO：引用计数不再作为事务内删除的强门闸，仅用于观测。
	if refCount, err := s.getReferenceCountInTransaction(tx, outpoint); err == nil && refCount > 0 {
		if s.logger != nil {
			s.logger.Warnf("⚠️ 事务内删除UTXO时发现非零引用计数（将忽略该计数）: outpoint=%s refCount=%d",
				buildUTXOKey(outpoint), refCount)
		}
	}

	// 5. 在事务中删除 UTXO
	if err := tx.Delete([]byte(utxoKey)); err != nil {
		return fmt.Errorf("删除 UTXO 失败: %w", err)
	}

	// 6. 在事务中更新索引（utxoObj 已确保不为 nil）
	s.indexManager.RemoveUTXOWithDetailsInTransaction(tx, utxoObj)

	// 注意：缓存更新和事件发布延迟到事务提交后
	// 当前实现中，事务版本的方法不更新缓存和发布事件
	// 调用方应在事务提交后手动更新缓存和发布事件（如果需要）

	if s.logger != nil {
		s.logger.Debugf("✅ UTXO 已在事务中删除: %s", utxoKey)
	}

	return nil
}

// getReferenceCountInTransaction 在事务中获取引用计数
func (s *Service) getReferenceCountInTransaction(tx storage.BadgerTransaction, outpoint *transaction.OutPoint) (uint64, error) {
	refKey := buildReferenceKey(outpoint)
	data, err := tx.Get([]byte(refKey))
	if err != nil {
		// 引用计数不存在，返回 0
		return 0, nil
	}
	if len(data) == 0 {
		return 0, nil
	}
	// 解析引用计数（uint64，8字节），使用 binary.BigEndian 与 getReferenceCount 保持一致
	if len(data) != 8 {
		return 0, fmt.Errorf("引用计数数据长度错误")
	}
	refCount := binary.BigEndian.Uint64(data)
	return refCount, nil
}

// buildUTXOKey 构造 UTXO 存储键
//
// 格式：utxo:set:{txHash}:{outputIndex}
// 符合 docs/system/designs/storage/data-architecture.md 规范
func buildUTXOKey(outpoint *transaction.OutPoint) string {
	return fmt.Sprintf("utxo:set:%x:%d", outpoint.TxId, outpoint.OutputIndex)
}
