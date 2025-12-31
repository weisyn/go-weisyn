// Package processor 实现区块处理服务
package processor

import (
	"context"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// executeBlock 执行区块处理
//
// 🎯 **区块执行流程**：
// 1. 执行并验证所有交易（业务验证，包括 ZK / 资源 / 引用UTXO 等）
// 2. 存储区块数据（通过 DataWriter，原子性更新区块/索引/UTXO/链状态）
// 3. 更新引用计数与状态根（通过 UTXOWriter）
// 4. 清理交易池
//
// 参数：
//   - ctx: 上下文
//   - block: 待执行区块
//
// 返回：
//   - error: 执行错误
func (s *Service) executeBlock(ctx context.Context, block *core.Block) error {
	if s.logger != nil {
		s.logger.Debugf("开始执行区块，高度: %d, 交易数: %d",
			block.Header.Height, len(block.Body.Transactions))
	}

	// 1. 验证所有交易执行结果（executeTransactions，S1）
	// ✅ **职责分离**：UTXO 变更由 DataWriter 处理，验证由 executeTransactions 处理
	// ❌ **不重新执行智能合约**（合约已在 TX 层执行）
	if err := s.executeTransactions(ctx, block); err != nil {
		return fmt.Errorf("交易验证失败: %w", err)
	}

	// 2. 存储区块数据（通过 DataWriter，会自动处理 UTXO 变更，S2-S5）
	//    只有在业务验证全部通过后才写入，确保对外语义上的“原子性”
	if err := s.storeBlock(ctx, block); err != nil {
		return fmt.Errorf("存储区块失败: %w", err)
	}

	// 3. ✅ 架构修复：处理引用计数管理（业务逻辑，应在 DataWriter 写入后处理）
	if err := s.processReferenceCounts(ctx, block); err != nil {
		// 引用计数管理失败不影响区块处理，只记录警告
		if s.logger != nil {
			s.logger.Warnf("⚠️ 引用计数管理失败: %v", err)
		}
	}

	// 4. ✅ 架构修复：更新状态根（业务逻辑，应在 UTXO 变更后处理）
	if err := s.updateStateRoot(ctx, block); err != nil {
		// 状态根更新失败不影响区块处理，但这通常意味着 UTXO/状态不一致，需要自愈链路介入
		h := block.Header.Height
		s.publishCorruptionDetected(ctx, types.CorruptionPhaseApply, types.CorruptionSeverityWarning, &h, "", "utxo:state_root", err)

		if s.logger != nil {
			s.logger.Warnf("⚠️ 状态根更新失败: %v", err)
		}
	}

	// 5. 清理交易池
	if err := s.cleanMempool(ctx, block); err != nil {
		// 清理失败不影响区块处理，只记录警告
		if s.logger != nil {
			s.logger.Warnf("清理交易池失败: %v", err)
		}
	}

	return nil
}

// processReferenceCounts 处理引用计数管理
//
// 🎯 **核心职责**：
// 在 DataWriter 写入区块后，扫描区块中的交易，识别引用型输入和被消费的引用交易，
// 然后通过 eutxo.UTXOWriter 处理引用计数。
//
// ✅ **架构修复**：
// 引用计数管理是业务逻辑，应该在业务层（BlockProcessor）处理，而不是在基础设施层（Persistence）。
//
// 📋 **处理流程**：
// 1. 扫描区块中的所有交易
// 2. 识别引用型输入（is_reference_only=true），增加引用计数
// 3. 识别被消费的引用交易，减少引用计数
func (s *Service) processReferenceCounts(ctx context.Context, block *core.Block) error {
	if s.utxoWriter == nil {
		// utxoWriter 不可用，跳过引用计数管理
		if s.logger != nil {
			s.logger.Debug("⚠️ utxoWriter 不可用，跳过引用计数管理")
		}
		return nil
	}

	if block == nil || block.Body == nil || len(block.Body.Transactions) == 0 {
		return nil
	}

	// 彻底迭代：
	// - 引用型输入（is_reference_only）是“只读依赖”，不形成跨区块锁定语义；
	// - 链上持久化的一致性由 DataWriter 在同一 Badger 事务内保证（区块/UTXO/索引）；
	// - 引用计数（ref:*）不应作为共识门闸，因此不在区块处理阶段做持久化写入，避免出现“写一半导致计数残留”的自运行问题。
	//
	// 结论：这里保持 no-op，仅保留函数以兼容现有调用链与日志语义。
	if s.logger != nil {
		s.logger.Debug("引用计数管理（持久化）已在彻底迭代中禁用：reference_only 仅作为验证语义，不落 ref:*")
	}
	return nil
}

// updateStateRoot 更新状态根
//
// 🎯 **核心职责**：
// 在 UTXO 变更完成后，重新计算状态根并更新到 EUTXO 模块。
//
// ✅ **架构修复**：
// 状态根更新是业务逻辑，应该在业务层（BlockProcessor）处理，而不是在基础设施层（Persistence）。
//
// 📋 **处理流程**：
// 1. 使用 UTXOQuery 计算当前状态根
// 2. 通过 eutxo.UTXOWriter.UpdateStateRoot 更新状态根
func (s *Service) updateStateRoot(ctx context.Context, block *core.Block) error {
	if s.utxoWriter == nil {
		// utxoWriter 不可用，跳过状态根更新
		if s.logger != nil {
			s.logger.Debug("⚠️ utxoWriter 不可用，跳过状态根更新")
		}
		return nil
	}

	if s.utxoQuery == nil {
		// utxoQuery 不可用，跳过状态根更新
		if s.logger != nil {
			s.logger.Debug("⚠️ utxoQuery 不可用，跳过状态根更新")
		}
		return nil
	}

	// 1. 计算新的状态根
	stateRoot, err := s.utxoQuery.GetCurrentStateRoot(ctx)
	if err != nil {
		return fmt.Errorf("计算状态根失败: %w", err)
	}

	// 2. 验证状态根长度
	if len(stateRoot) != 32 {
		return fmt.Errorf("状态根长度错误: 期望32字节, 得到%d字节", len(stateRoot))
	}

	// 3. 更新状态根到 EUTXO 模块
	if err := s.utxoWriter.UpdateStateRoot(ctx, stateRoot); err != nil {
		return fmt.Errorf("更新 EUTXO 状态根失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ EUTXO 状态根已更新: %x", stateRoot[:minHelper(8, len(stateRoot))])
	}

	return nil
}

// calculateTxHash 计算交易哈希
func (s *Service) calculateTxHash(ctx context.Context, txProto *transaction.Transaction) ([]byte, error) {
	if txProto == nil {
		return nil, fmt.Errorf("交易为空")
	}
	if s.txHashClient == nil {
		return nil, fmt.Errorf("txHashClient 未初始化")
	}

	req := &transaction.ComputeHashRequest{
		Transaction: txProto,
	}
	resp, err := s.txHashClient.ComputeHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用交易哈希服务失败: %w", err)
	}

	if !resp.IsValid {
		return nil, fmt.Errorf("交易结构无效")
	}

	return resp.Hash, nil
}

// storeBlock 存储区块数据（P3-6：完整区块存储）
//
// 🎯 **存储策略**：
// 1. 序列化完整区块数据
// 2. 存储区块数据（block:data:{hash}）
// 3. 存储区块索引（block:height:{height} -> hash）
// 4. 存储区块哈希索引（block:hash:{hash} -> height）
// 5. 更新链尖状态
func (s *Service) storeBlock(ctx context.Context, block *core.Block) error {
	// 1. 计算区块哈希
	if s.blockHashClient == nil {
		// 区块哈希是链一致性的根基：不允许“临时值/占位符”回退。
		return fmt.Errorf("blockHashClient 未初始化：拒绝存储区块（height=%d）", block.Header.Height)
	}

	blockHash, err := s.calculateBlockHash(ctx, block.Header)
	if err != nil {
		return fmt.Errorf("计算区块哈希失败：拒绝存储区块（height=%d）: %w", block.Header.Height, err)
	}
	if len(blockHash) == 0 {
		return fmt.Errorf("区块哈希为空：拒绝存储区块（height=%d）", block.Header.Height)
	}

	// 2. 存储区块数据（通过 DataWriter，内部会处理所有索引和状态更新）
	// DataWriter.WriteBlock 会自动处理：
	// - 存储区块数据
	// - 更新区块索引（高度索引、哈希索引）
	// - 更新链尖状态
	// - 更新交易索引
	// - 处理 UTXO 变更
	if err := s.dataWriter.WriteBlock(ctx, block); err != nil {
		return fmt.Errorf("存储区块数据失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 区块已存储: height=%d, hash=%x",
			block.Header.Height, blockHash[:8])
	}

	return nil
}

// cleanMempool 清理交易池
//
// 🎯 **移除已处理的交易**
//
// 从交易池中移除区块中已处理的交易
func (s *Service) cleanMempool(ctx context.Context, block *core.Block) error {
	if s.mempool == nil {
		// 内存池不可用，跳过清理
		return nil
	}

	if block == nil || block.Body == nil || len(block.Body.Transactions) == 0 {
		// 没有交易需要清理
		return nil
	}

	// 计算所有交易的哈希
	txIDs := make([][]byte, 0, len(block.Body.Transactions))
	for _, tx := range block.Body.Transactions {
		if s.txHashClient == nil {
			if s.logger != nil {
				s.logger.Warnf("txHashClient 未初始化，跳过交易哈希计算")
			}
			continue
		}

		// 使用 gRPC 服务计算交易哈希
		req := &transaction.ComputeHashRequest{
			Transaction: tx,
		}
		resp, err := s.txHashClient.ComputeHash(ctx, req)
		if err != nil {
			if s.logger != nil {
				s.logger.Warnf("计算交易哈希失败，跳过清理: %v", err)
			}
			continue
		}

		if !resp.IsValid {
			if s.logger != nil {
				s.logger.Warnf("交易结构无效，跳过清理")
			}
			continue
		}

		txIDs = append(txIDs, resp.Hash)
	}

	// 确认交易（从交易池移除）
	if len(txIDs) > 0 {
		if err := s.mempool.ConfirmTransactions(txIDs, block.Header.Height); err != nil {
			return fmt.Errorf("确认交易失败: %w", err)
		}

		if s.logger != nil {
			s.logger.Debugf("✅ 已从交易池移除 %d 个已处理交易", len(txIDs))
		}
	}

	return nil
}

// uint64ToBytes uint64转字节
func uint64ToBytes(n uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(n >> 56)
	b[1] = byte(n >> 48)
	b[2] = byte(n >> 40)
	b[3] = byte(n >> 32)
	b[4] = byte(n >> 24)
	b[5] = byte(n >> 16)
	b[6] = byte(n >> 8)
	b[7] = byte(n)
	return b
}

// minHelper 返回两个整数中的较小值
func minHelper(a, b int) int {
	if a < b {
		return a
	}
	return b
}
