// Package writer 实现链状态更新逻辑
//
// ⛓️ **链状态更新 (Chain State Update)**
//
// 本文件实现链状态的更新逻辑，包括链尖和状态根更新。
//
// 🎯 **核心职责**：
// - 更新链尖（state:chain:tip）
// - 更新状态根（state:chain:root）
//
// ⚠️ **关键原则**：
// - 链尖格式：height(8字节) + blockHash(32字节)
// - 状态根格式：stateRoot(32字节)
package writer

import (
	"context"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// writeChainState 更新链状态
//
// 🎯 **核心职责**：
// 更新链的当前状态，包括链尖和状态根。
//
// 📋 **处理流程**：
// 1. 计算区块哈希（BlockHeader 不包含 Hash 字段，需要计算）
// 2. 更新链尖（state:chain:tip）
//   - 值格式：height(8字节) + blockHash(32字节)
//
// 3. 更新状态根（state:chain:root）
//   - 值格式：stateRoot(32字节)
//   - 从区块头的 StateRoot 获取
//
// ⚠️ **注意事项**：
// - 链尖必须原子性更新
// - 区块哈希需要计算（BlockHeader 不包含 Hash 字段）
// - 状态根从区块头的 StateRoot 获取
func (s *Service) writeChainState(ctx context.Context, tx storage.BadgerTransaction, block *core.Block) error {
	if s.blockHashClient == nil {
		return fmt.Errorf("blockHashClient 未初始化")
	}

	// 1. 计算区块哈希（BlockHeader 不包含 Hash 字段，需要计算，使用 gRPC 服务）
	req := &core.ComputeBlockHashRequest{
		Block: block,
	}
	resp, err := s.blockHashClient.ComputeBlockHash(ctx, req)
	if err != nil {
		return fmt.Errorf("调用区块哈希服务失败: %w", err)
	}

	if !resp.IsValid {
		return fmt.Errorf("区块结构无效")
	}

	blockHash := resp.Hash

	// 2. 更新链尖（state:chain:tip）
	// 值格式：height(8字节) + blockHash(32字节)
	tipKey := []byte("state:chain:tip")
	tipValue := make([]byte, 8+32)
	copy(tipValue[0:8], uint64ToBytes(block.Header.Height))
	copy(tipValue[8:40], blockHash)
	if err := tx.Set(tipKey, tipValue); err != nil {
		return fmt.Errorf("更新链尖失败: %w", err)
	}

	// 3. 更新状态根（state:chain:root）
	// ⚠️ **状态根更新策略**：
	// - 优先使用区块头中的 StateRoot（如果存在）
	// - 如果区块头中的 StateRoot 为空，状态根会在事务提交后通过 updateStateRootAfterUTXOChanges 更新
	// - 状态根反映当前所有 UTXO 的状态，应该在 UTXO 变更后立即更新
	if len(block.Header.StateRoot) > 0 && len(block.Header.StateRoot) == 32 {
		stateRootKey := []byte("state:chain:root")
		if err := tx.Set(stateRootKey, block.Header.StateRoot); err != nil {
			return fmt.Errorf("更新状态根失败: %w", err)
		}
	} else {
		// 区块头中的 StateRoot 为空或无效，状态根会在事务提交后更新
		if s.logger != nil {
			s.logger.Debug("⚠️ 区块头中的 StateRoot 为空或无效，将在事务提交后更新")
		}
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 链状态已更新: height=%d, hash=%x",
			block.Header.Height, blockHash[:8])
	}

	return nil
}
