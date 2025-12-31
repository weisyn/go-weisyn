// Package writer 实现交易索引写入逻辑
//
// 📇 **交易索引写入 (Transaction Index Writing)**
//
// 本文件实现交易索引的写入逻辑，从区块中提取交易并创建索引。
//
// 🎯 **核心职责**：
// - 从区块中提取所有交易
// - 计算每笔交易的哈希
// - 写入交易索引（只存储索引，不重复存储交易数据）
//
// ⚠️ **关键原则**：
// - 只存储索引，不重复存储交易数据（交易已被区块包含）
// - 索引格式：txHash → (blockHeight, txIndex)
package writer

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
	core "github.com/weisyn/v1/pb/blockchain/block"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// writeTransactionIndices 更新交易索引
//
// 🎯 **核心职责**：
// 从区块中提取交易，创建交易索引。
//
// 📋 **处理流程**：
// 1. 计算区块哈希（用于交易索引）
// 2. 遍历区块中的所有交易
// 3. 计算每笔交易的哈希
	// 4. 写入交易索引（indices:tx:{txHash} → blockHeight(8字节) + blockHash(32字节) + txIndex(4字节)）
//
// ⚠️ **关键原则**：
// - 只存储索引，不重复存储交易数据
// - 交易数据可以从区块中提取
// - 索引格式：txHash → (blockHeight(8字节) + blockHash(32字节) + txIndex(4字节))
func (s *Service) writeTransactionIndices(ctx context.Context, tx storage.BadgerTransaction, block *core.Block) error {
	if s.blockHashClient == nil {
		return fmt.Errorf("blockHashClient 未初始化")
	}
	if s.txHashClient == nil {
		return fmt.Errorf("txHashClient 未初始化")
	}

	// 1. 计算区块哈希（用于交易索引）
	blockReq := &core.ComputeBlockHashRequest{
		Block: block,
	}
	blockResp, err := s.blockHashClient.ComputeBlockHash(ctx, blockReq)
	if err != nil {
		return fmt.Errorf("计算区块哈希失败: %w", err)
	}

	if !blockResp.IsValid {
		return fmt.Errorf("区块结构无效")
	}

	blockHash := blockResp.Hash

	// 2. 遍历区块中的所有交易
	transactions := block.Body.Transactions
	if transactions == nil {
		// 如果没有交易，直接返回
		return nil
	}

	for i, txProto := range transactions {
		// 3. 计算交易哈希（使用 gRPC 服务）
		txReq := &transaction.ComputeHashRequest{
			Transaction: txProto,
		}
		txResp, err := s.txHashClient.ComputeHash(ctx, txReq)
		if err != nil {
			return fmt.Errorf("计算交易哈希失败（交易 %d）: %w", i, err)
		}

		if !txResp.IsValid {
			return fmt.Errorf("交易 %d 结构无效", i)
		}

		txHash := txResp.Hash

		// 4. 编码交易索引值：blockHeight(8字节) + blockHash(32字节) + txIndex(4字节)
		indexValue := make([]byte, 8+32+4)
		// 编码高度（前8字节）
		copy(indexValue[0:8], uint64ToBytes(block.Header.Height))
		// 编码区块哈希（中间32字节）
		copy(indexValue[8:40], blockHash)
		// 编码交易索引（后4字节）
		binary.BigEndian.PutUint32(indexValue[40:44], uint32(i))

		// 5. 写入交易索引（indices:tx:{txHash} → indexValue）
		// ✅ 修复 P0-1：键格式必须与查询一致，添加 "indices:" 前缀
		txKey := fmt.Sprintf("indices:tx:%x", txHash)
		if err := tx.Set([]byte(txKey), indexValue); err != nil {
			return fmt.Errorf("写入交易索引失败（交易 %d）: %w", i, err)
		}
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 交易索引已更新: height=%d, txCount=%d",
			block.Header.Height, len(transactions))
	}

	return nil
}

// deleteBlockTransactionIndices 删除区块的交易索引（用于分叉处理）
//
// 🎯 **核心职责**：
// 在分叉处理时，删除原主链区块的交易索引，确保索引一致性。
//
// 📋 **处理流程**：
// 1. 遍历区块中的所有交易
// 2. 计算每笔交易的哈希
// 3. 删除对应的交易索引（indices:tx:{txHash}）
//
// ⚠️ **关键原则**：
// - 只在分叉处理时调用，用于清理原主链的交易索引
// - 不删除区块数据本身（区块保留用于历史查询）
// - 不影响 UTXO（UTXO 由 UTXOSnapshot 处理）
func (s *Service) deleteBlockTransactionIndices(ctx context.Context, tx storage.BadgerTransaction, block *core.Block) error {
	if s.txHashClient == nil {
		return fmt.Errorf("txHashClient 未初始化")
	}

	// 1. 遍历区块中的所有交易
	transactions := block.Body.Transactions
	if transactions == nil {
		// 如果没有交易，直接返回
		return nil
	}

	for i, txProto := range transactions {
		// 2. 计算交易哈希（使用 gRPC 服务）
		txReq := &transaction.ComputeHashRequest{
			Transaction: txProto,
		}
		txResp, err := s.txHashClient.ComputeHash(ctx, txReq)
		if err != nil {
			return fmt.Errorf("计算交易哈希失败（交易 %d）: %w", i, err)
		}

		if !txResp.IsValid {
			return fmt.Errorf("交易 %d 结构无效", i)
		}

		txHash := txResp.Hash

		// 3. 删除交易索引（indices:tx:{txHash}）
		txKey := fmt.Sprintf("indices:tx:%x", txHash)
		if err := tx.Delete([]byte(txKey)); err != nil {
			return fmt.Errorf("删除交易索引失败（交易 %d）: %w", i, err)
		}
	}

	if s.logger != nil {
		s.logger.Debugf("✅ 交易索引已删除: height=%d, txCount=%d",
			block.Header.Height, len(transactions))
	}

	return nil
}

// DeleteBlockTransactionIndices 实现 DataWriter 接口（删除区块的交易索引）
//
// ✅ 修复 P0-3：分叉处理时删除原主链的交易索引
func (s *Service) DeleteBlockTransactionIndices(ctx context.Context, block *core.Block) error {
	if err := writegate.Default().AssertWriteAllowed(ctx, "persistence.DataWriter.DeleteBlockTransactionIndices"); err != nil {
		return err
	}
	// 在事务中删除交易索引
	return s.storage.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		return s.deleteBlockTransactionIndices(ctx, tx, block)
	})
}

