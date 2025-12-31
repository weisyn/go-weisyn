package writer

import (
	"context"
	"fmt"
	"sort"

	"github.com/weisyn/v1/internal/core/block/merkle"
	"github.com/weisyn/v1/internal/core/eutxo/writer/eventhelpers"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
	"google.golang.org/protobuf/proto"
)

// UpdateStateRoot 更新状态根
//
// 实现 eutxo.UTXOWriter.UpdateStateRoot
func (s *Service) UpdateStateRoot(ctx context.Context, stateRoot []byte) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "eutxo.UTXOWriter.UpdateStateRoot"); err != nil {
		return err
	}
	// 1. 验证状态根
	if len(stateRoot) != 32 {
		return fmt.Errorf("状态根长度必须为32字节，实际长度: %d", len(stateRoot))
	}

	// 2. 加锁
	s.mu.Lock()
	defer s.mu.Unlock()

	// 3. 存储状态根
	stateRootKey := []byte("utxo_state_root")
	if err := s.storage.Set(ctx, stateRootKey, stateRoot); err != nil {
		return fmt.Errorf("存储状态根失败: %w", err)
	}

	// 4. 发布事件（可选）
	if s.eventBus != nil {
		// P3-10: 发布 UTXOStateRootUpdated 事件
		eventhelpers.PublishUTXOStateRootUpdatedEvent(ctx, s.eventBus, s.logger, stateRoot)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 状态根已更新: %x", stateRoot)
	}

	return nil
}

// UpdateStateRootInTransaction 在事务中更新状态根（原子写入版本）。
//
// 说明：
// - 不加锁：由上层事务边界与写门闸保证互斥
// - 不更新缓存、不发布事件：避免事务未提交时产生外部可见副作用
func (s *Service) UpdateStateRootInTransaction(ctx context.Context, tx storage.BadgerTransaction, stateRoot []byte) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "eutxo.UTXOWriter.UpdateStateRootInTransaction"); err != nil {
		return err
	}
	if len(stateRoot) != 32 {
		return fmt.Errorf("状态根长度必须为32字节，实际长度: %d", len(stateRoot))
	}
	if tx == nil {
		return fmt.Errorf("transaction 不能为空")
	}
	stateRootKey := []byte("utxo_state_root")
	if err := tx.Set(stateRootKey, stateRoot); err != nil {
		return fmt.Errorf("事务内存储状态根失败: %w", err)
	}
	return nil
}

// calculateStateRoot 计算状态根（内部方法）
//
// 🎯 **基于 Merkle 树的状态根计算**
//
// 算法：
// 1. 获取所有 UTXO（通过前缀扫描）
// 2. 计算每个 UTXO 的哈希（序列化后哈希）
// 3. 使用 Merkle 树计算根哈希
//
// 返回：
//   - []byte: 32字节状态根
//   - error: 计算错误
func (s *Service) calculateStateRoot(ctx context.Context) ([]byte, error) {
	// 1. 获取所有 UTXO（通过前缀扫描）
	// 符合 docs/system/designs/storage/data-architecture.md 规范
	utxoPrefix := []byte("utxo:set:")
	utxoMap, err := s.storage.PrefixScan(ctx, utxoPrefix)
	if err != nil {
		return nil, fmt.Errorf("扫描 UTXO 失败: %w", err)
	}

	// 2. 如果没有 UTXO，返回空哈希
	if len(utxoMap) == 0 {
		if s.logger != nil {
			s.logger.Debug("无 UTXO，返回空状态根")
		}
		return make([]byte, 32), nil
	}

	// 3. 计算每个 UTXO 的哈希
	// ⚠️ 注意：PrefixScan 返回的是 map，遍历顺序不确定。
	// 为了保证 StateRoot 在不同节点上可复现，这里需要对 key 做排序后再计算 Merkle Root。
	utxoHashes := make([][]byte, 0, len(utxoMap))

	// 3.1 收集并排序所有 key，保证遍历顺序确定
	keys := make([]string, 0, len(utxoMap))
	for k := range utxoMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 3.2 按照有序 key 计算每个 UTXO 的哈希
	for _, k := range keys {
		utxoData := utxoMap[k]
		// 反序列化 UTXO（用于验证数据完整性）
		utxoObj := &utxo.UTXO{}
		if err := proto.Unmarshal(utxoData, utxoObj); err != nil {
			if s.logger != nil {
				s.logger.Warnf("反序列化 UTXO 失败，跳过: %v", err)
			}
			continue
		}

		// 计算 UTXO 哈希（使用序列化数据）
		utxoHash := s.hasher.SHA256(utxoData)
		utxoHashes = append(utxoHashes, utxoHash)
	}

	// 4. 使用 Merkle 树计算根哈希
	if len(utxoHashes) == 0 {
		return make([]byte, 32), nil
	}

	// 使用 merkle 包计算 Merkle 根
	hasherAdapter := merkle.NewHashManagerAdapter(s.hasher)
	stateRoot, err := buildMerkleTree(hasherAdapter, utxoHashes)
	if err != nil {
		return nil, fmt.Errorf("计算 Merkle 树失败: %w", err)
	}

	// 确保状态根长度为32字节
	if len(stateRoot) != 32 {
		return nil, fmt.Errorf("状态根长度错误: 期望32字节, 得到%d字节", len(stateRoot))
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 状态根计算完成: %x (UTXO数量=%d)", stateRoot[:8], len(utxoHashes))
	}

	return stateRoot, nil
}

// buildMerkleTree 递归构建 Merkle 树
//
// 用于计算 UTXO 状态根的 Merkle 树（使用哈希数组而不是交易列表）
func buildMerkleTree(hasher merkle.Hasher, hashes [][]byte) ([]byte, error) {
	// 基础情况：只有一个节点，返回该节点
	if len(hashes) == 1 {
		return hashes[0], nil
	}

	// 如果节点数为奇数，复制最后一个节点
	if len(hashes)%2 == 1 {
		hashes = append(hashes, hashes[len(hashes)-1])
	}

	// 计算下一层节点
	nextLevel := make([][]byte, 0, len(hashes)/2)
	for i := 0; i < len(hashes); i += 2 {
		// 连接两个子节点的哈希
		combined := append(hashes[i], hashes[i+1]...)

		// 计算父节点哈希
		parentHash, err := hasher.Hash(combined)
		if err != nil {
			return nil, fmt.Errorf("计算父节点哈希失败: %w", err)
		}

		nextLevel = append(nextLevel, parentHash)
	}

	// 递归处理下一层
	return buildMerkleTree(hasher, nextLevel)
}
