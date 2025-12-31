package writer

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/weisyn/v1/internal/core/eutxo/writer/eventhelpers"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
)

// ReferenceUTXO 引用 UTXO（增加引用计数）
//
// 实现 eutxo.UTXOWriter.ReferenceUTXO
func (s *Service) ReferenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "eutxo.UTXOWriter.ReferenceUTXO"); err != nil {
		return err
	}
	// 1. 验证 OutPoint
	if outpoint == nil || outpoint.TxId == nil {
		return fmt.Errorf("无效的 OutPoint")
	}

	// 2. 加锁
	s.mu.Lock()
	defer s.mu.Unlock()

	// 3. 获取当前引用计数
	refCount, err := s.getReferenceCount(ctx, outpoint)
	if err != nil {
		return fmt.Errorf("获取引用计数失败: %w", err)
	}

	// 4. 增加引用计数
	refCount++

	// 5. 存储引用计数
	if err := s.storeReferenceCount(ctx, outpoint, refCount); err != nil {
		return fmt.Errorf("存储引用计数失败: %w", err)
	}

	// 6. 发布事件（可选）
	if s.eventBus != nil {
		// P3-10: 发布 UTXOReferenced 事件
		eventhelpers.PublishUTXOReferencedEvent(ctx, s.eventBus, s.logger, outpoint, refCount)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ UTXO 引用计数增加: %s, count=%d", buildUTXOKey(outpoint), refCount)
	}

	return nil
}

// UnreferenceUTXO 解除 UTXO 引用（减少引用计数）
//
// 实现 eutxo.UTXOWriter.UnreferenceUTXO
func (s *Service) UnreferenceUTXO(ctx context.Context, outpoint *transaction.OutPoint) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "eutxo.UTXOWriter.UnreferenceUTXO"); err != nil {
		return err
	}
	// 1. 验证 OutPoint
	if outpoint == nil || outpoint.TxId == nil {
		return fmt.Errorf("无效的 OutPoint")
	}

	// 2. 加锁
	s.mu.Lock()
	defer s.mu.Unlock()

	// 3. 获取当前引用计数
	refCount, err := s.getReferenceCount(ctx, outpoint)
	if err != nil {
		return fmt.Errorf("获取引用计数失败: %w", err)
	}

	// 4. 检查引用计数
	if refCount == 0 {
		return fmt.Errorf("引用计数已为0，无法减少")
	}

	// 5. 减少引用计数
	refCount--

	// 6. 存储引用计数
	if err := s.storeReferenceCount(ctx, outpoint, refCount); err != nil {
		return fmt.Errorf("存储引用计数失败: %w", err)
	}

	// 7. 发布事件（可选）
	if s.eventBus != nil {
		// P3-10: 发布 UTXOUnreferenced 事件
		eventhelpers.PublishUTXOUnreferencedEvent(ctx, s.eventBus, s.logger, outpoint, refCount)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ UTXO 引用计数减少: %s, count=%d", buildUTXOKey(outpoint), refCount)
	}

	return nil
}

// getReferenceCount 获取引用计数（内部方法）
func (s *Service) getReferenceCount(ctx context.Context, outpoint *transaction.OutPoint) (uint64, error) {
	refKey := buildReferenceKey(outpoint)

	data, err := s.storage.Get(ctx, []byte(refKey))
	if err != nil {
		// 如果不存在，返回 0
		return 0, nil
	}

	// 如果数据为空或 nil，返回 0
	if data == nil || len(data) == 0 {
		return 0, nil
	}

	if len(data) != 8 {
		return 0, fmt.Errorf("引用计数数据长度错误: 期望8字节，实际%d字节", len(data))
	}

	refCount := binary.BigEndian.Uint64(data)
	return refCount, nil
}

// storeReferenceCount 存储引用计数（内部方法）
func (s *Service) storeReferenceCount(ctx context.Context, outpoint *transaction.OutPoint, refCount uint64) error {
	refKey := buildReferenceKey(outpoint)

	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, refCount)

	return s.storage.Set(ctx, []byte(refKey), data)
}

// buildReferenceKey 构造引用计数存储键
//
// 格式：ref:<txhash>:<index>
func buildReferenceKey(outpoint *transaction.OutPoint) string {
	return fmt.Sprintf("ref:%x:%d", outpoint.TxId, outpoint.OutputIndex)
}

