// Package recovery 提供索引恢复管理
//
// 🎯 **核心职责**：
// - 检测和修复所有索引相关的损坏
// - 支持Tip、Height、Hash、TX等多种索引类型
// - 提供选择性修复和全量重建能力
package recovery

import (
	"context"
	"encoding/binary"
	"fmt"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
//                              索引恢复管理器
// ============================================================================

// IndexRecoveryManager 索引恢复管理器
//
// 🎯 **覆盖范围**：
// - tip_inconsistent: BestChain索引不一致
// - index_corrupt_hash_height: hash→height映射损坏
// - index_corrupt_height_index: height→hash映射损坏
// - tx_index_corrupt: 交易索引损坏
// - resource_index_corrupt: 资源索引损坏
type IndexRecoveryManager struct {
	queryService persistence.QueryService
	store        storage.BadgerStore
	hashManager  crypto.HashManager
	logger       logiface.Logger
}

// NewIndexRecoveryManager 创建索引恢复管理器
func NewIndexRecoveryManager(
	queryService persistence.QueryService,
	store storage.BadgerStore,
	hashManager crypto.HashManager,
	logger logiface.Logger,
) *IndexRecoveryManager {
	return &IndexRecoveryManager{
		queryService: queryService,
		store:        store,
		hashManager:  hashManager,
		logger:       logger,
	}
}

// ============================================================================
//                              Tip索引修复
// ============================================================================

// RepairTipByHeight 修复BestChain索引（基于高度）
//
// 🎯 **修复策略**：
// 1. 读取指定高度的区块
// 2. 计算区块hash
// 3. 更新 state:chain:tip
//
// 参数：
//   - ctx: 操作上下文
//   - height: 链尖高度
//
// 返回：
//   - error: 修复失败的错误
func (m *IndexRecoveryManager) RepairTipByHeight(ctx context.Context, height uint64) error {
	if m.logger != nil {
		m.logger.Infof("🔧 修复Tip索引: height=%d", height)
	}

	// 1. 读取区块
	block, err := m.queryService.GetBlockByHeight(ctx, height)
	if err != nil {
		return fmt.Errorf("get block by height failed: %w", err)
	}

	if block == nil || block.Header == nil {
		return fmt.Errorf("block is nil at height %d", height)
	}

	// 2. 计算区块hash
	blockHash, err := m.computeBlockHash(ctx, block)
	if err != nil {
		return fmt.Errorf("compute block hash failed: %w", err)
	}

	// 3. 更新 state:chain:tip
	if err := m.updateChainTip(ctx, height, blockHash); err != nil {
		return fmt.Errorf("update chain tip failed: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("✅ Tip索引修复成功: height=%d hash=%x", height, blockHash[:6])
	}

	return nil
}

// RepairTipIndex 修复BestChain索引（提供hash）
//
// 🎯 **修复策略**：
// 直接使用提供的正确hash更新state:chain:tip
//
// 参数：
//   - ctx: 操作上下文
//   - height: 链尖高度
//   - correctHash: 正确的区块hash
//
// 返回：
//   - error: 修复失败的错误
func (m *IndexRecoveryManager) RepairTipIndex(ctx context.Context, height uint64, correctHash []byte) error {
	if m.logger != nil {
		m.logger.Infof("🔧 修复Tip索引（使用提供的hash）: height=%d hash=%x", height, correctHash[:6])
	}

	if len(correctHash) != 32 {
		return fmt.Errorf("invalid hash length: %d", len(correctHash))
	}

	if err := m.updateChainTip(ctx, height, correctHash); err != nil {
		return fmt.Errorf("update chain tip failed: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("✅ Tip索引修复成功")
	}

	return nil
}

// updateChainTip 更新链尖索引
func (m *IndexRecoveryManager) updateChainTip(ctx context.Context, height uint64, blockHash []byte) error {
	tipKey := []byte("state:chain:tip")
	tipValue := make([]byte, 40) // 8 bytes height + 32 bytes hash

	binary.BigEndian.PutUint64(tipValue[0:8], height)
	copy(tipValue[8:40], blockHash)

	return m.store.Set(ctx, tipKey, tipValue)
}

// ============================================================================
//                              Height索引重建
// ============================================================================

// RebuildHeightIndex 重建height→hash索引
//
// 🎯 **修复策略**：
// 1. 扫描指定范围的区块文件
// 2. 重建 indices:height:{height} → hash 映射
// 3. 重建 indices:hash:{hash} → height 反向映射
//
// 参数：
//   - ctx: 操作上下文
//   - fromHeight: 起始高度
//   - toHeight: 结束高度
//
// 返回：
//   - error: 修复失败的错误
func (m *IndexRecoveryManager) RebuildHeightIndex(ctx context.Context, fromHeight, toHeight uint64) error {
	if m.logger != nil {
		m.logger.Infof("🔧 重建Height索引: [%d..%d]", fromHeight, toHeight)
	}

	for height := fromHeight; height <= toHeight; height++ {
		// 读取区块
		block, err := m.queryService.GetBlockByHeight(ctx, height)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("跳过高度 %d: %v", height, err)
			}
			continue
		}

		if block == nil || block.Header == nil {
			if m.logger != nil {
				m.logger.Warnf("跳过高度 %d: block is nil", height)
			}
			continue
		}

		// 计算区块hash
		blockHash, err := m.computeBlockHash(ctx, block)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("跳过高度 %d: compute hash failed: %v", height, err)
			}
			continue
		}

		// 更新 indices:height:{height}
		heightKey := []byte(fmt.Sprintf("indices:height:%d", height))
		if err := m.store.Set(ctx, heightKey, blockHash); err != nil {
			return fmt.Errorf("set height index failed at %d: %w", height, err)
		}

		// 更新 indices:hash:{hash}
		hashKey := []byte(fmt.Sprintf("indices:hash:%x", blockHash))
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, height)
		if err := m.store.Set(ctx, hashKey, heightBytes); err != nil {
			return fmt.Errorf("set hash index failed at %d: %w", height, err)
		}

		// 定期日志
		if height%1000 == 0 && m.logger != nil {
			m.logger.Infof("进度: %d/%d", height, toHeight)
		}
	}

	if m.logger != nil {
		m.logger.Infof("✅ Height索引重建完成: [%d..%d]", fromHeight, toHeight)
	}

	return nil
}

// ============================================================================
//                              TX索引重建
// ============================================================================

// RebuildTxIndex 重建交易索引
//
// 🎯 **修复策略**：
// 1. 扫描指定范围的区块
// 2. 重建 indices:tx:{txHash} → (height + txIndex) 映射
//
// 参数：
//   - ctx: 操作上下文
//   - fromHeight: 起始高度
//   - toHeight: 结束高度
//
// 返回：
//   - error: 修复失败的错误
func (m *IndexRecoveryManager) RebuildTxIndex(ctx context.Context, fromHeight, toHeight uint64) error {
	if m.logger != nil {
		m.logger.Infof("🔧 重建TX索引: [%d..%d]", fromHeight, toHeight)
	}

	for height := fromHeight; height <= toHeight; height++ {
		// 读取区块
		block, err := m.queryService.GetBlockByHeight(ctx, height)
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("跳过高度 %d: %v", height, err)
			}
			continue
		}

		if block == nil || block.Body == nil || len(block.Body.Transactions) == 0 {
			continue
		}

		// 遍历交易，重建TX索引
		for txIndex, tx := range block.Body.Transactions {
			if tx == nil {
				if m.logger != nil {
					m.logger.Warnf("区块 %d 的交易 %d 为空，跳过", height, txIndex)
				}
				continue
			}

			// 序列化交易
			txBytes, err := proto.Marshal(tx)
			if err != nil {
				if m.logger != nil {
					m.logger.Warnf("序列化交易失败 (height=%d, txIndex=%d): %v", height, txIndex, err)
				}
				continue
			}

			// 计算交易hash
			txHash, err := m.computeTxHash(ctx, txBytes)
			if err != nil {
				if m.logger != nil {
					m.logger.Warnf("计算交易hash失败 (height=%d, txIndex=%d): %v", height, txIndex, err)
				}
				continue
			}

			// 写入TX索引: indices:tx:{txHash} → {blockHash, txIndex}
			if err := m.writeTxIndex(ctx, txHash, block, uint32(txIndex)); err != nil {
				if m.logger != nil {
					m.logger.Warnf("写入TX索引失败 (height=%d, txIndex=%d): %v", height, txIndex, err)
				}
				continue
			}
		}

		// 定期日志
		if height%1000 == 0 && m.logger != nil {
			m.logger.Infof("进度: %d/%d", height, toHeight)
		}
	}

	if m.logger != nil {
		m.logger.Infof("✅ TX索引重建完成: [%d..%d]", fromHeight, toHeight)
	}

	return nil
}

// ============================================================================
//                              全量索引重建
// ============================================================================

// FullIndexRebuild 全量索引重建
//
// 🎯 **修复策略**：
// - 从genesis到指定高度重建所有索引
// - 包括Height索引、Hash索引、TX索引
//
// 参数：
//   - ctx: 操作上下文
//   - maxHeight: 最大高度
//
// 返回：
//   - error: 修复失败的错误
func (m *IndexRecoveryManager) FullIndexRebuild(ctx context.Context, maxHeight uint64) error {
	if m.logger != nil {
		m.logger.Infof("🔧 全量索引重建: [0..%d]", maxHeight)
	}

	// 重建Height索引
	if err := m.RebuildHeightIndex(ctx, 0, maxHeight); err != nil {
		return fmt.Errorf("rebuild height index failed: %w", err)
	}

	// 重建TX索引
	if err := m.RebuildTxIndex(ctx, 0, maxHeight); err != nil {
		return fmt.Errorf("rebuild tx index failed: %w", err)
	}

	// 更新链尖
	if err := m.RepairTipByHeight(ctx, maxHeight); err != nil {
		return fmt.Errorf("repair tip failed: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("✅ 全量索引重建完成")
	}

	return nil
}

// ============================================================================
//                              辅助方法
// ============================================================================

// computeBlockHash 计算区块hash
func (m *IndexRecoveryManager) computeBlockHash(ctx context.Context, block *core.Block) ([]byte, error) {
	if m.hashManager == nil {
		return nil, fmt.Errorf("hash manager not initialized")
	}

	if block == nil || block.Header == nil {
		return nil, fmt.Errorf("block or header is nil")
	}

	// 序列化区块头
	headerBytes, err := proto.Marshal(block.Header)
	if err != nil {
		return nil, fmt.Errorf("serialize block header failed: %w", err)
	}

	// 使用DoubleSHA256计算区块hash（比特币风格）
	blockHash := m.hashManager.DoubleSHA256(headerBytes)

	if len(blockHash) != 32 {
		return nil, fmt.Errorf("invalid block hash length: %d", len(blockHash))
	}

	return blockHash, nil
}

// computeTxHash 计算交易hash
//
// 参数：
//   - ctx: 上下文
//   - txBytes: 交易序列化后的字节数组
//
// 返回：
//   - []byte: 交易hash（32字节）
//   - error: 计算错误
func (m *IndexRecoveryManager) computeTxHash(ctx context.Context, txBytes []byte) ([]byte, error) {
	if m.hashManager == nil {
		return nil, fmt.Errorf("hash manager not initialized")
	}

	if len(txBytes) == 0 {
		return nil, fmt.Errorf("transaction bytes is empty")
	}

	// 使用SHA256计算交易hash
	txHash := m.hashManager.SHA256(txBytes)

	if len(txHash) != 32 {
		return nil, fmt.Errorf("invalid tx hash length: %d", len(txHash))
	}

	return txHash, nil
}

// writeTxIndex 写入交易索引
//
// 参数：
//   - ctx: 上下文
//   - txHash: 交易hash（32字节）
//   - block: 区块对象（用于获取高度和计算区块hash）
//   - txIndex: 交易在区块中的索引
//
// 返回：
//   - error: 写入错误
func (m *IndexRecoveryManager) writeTxIndex(ctx context.Context, txHash []byte, block *core.Block, txIndex uint32) error {
	if m.store == nil {
		return fmt.Errorf("store not initialized")
	}

	if len(txHash) != 32 {
		return fmt.Errorf("invalid tx hash length: %d", len(txHash))
	}

	// 计算区块hash
	blockHash, err := m.computeBlockHash(ctx, block)
	if err != nil {
		return fmt.Errorf("compute block hash failed: %w", err)
	}

	// 编码交易索引值：blockHeight(8字节) + blockHash(32字节) + txIndex(4字节)
	indexValue := make([]byte, 44)
	// 编码高度（前8字节）
	binary.BigEndian.PutUint64(indexValue[0:8], block.Header.Height)
	// 编码区块哈希（中间32字节）
	copy(indexValue[8:40], blockHash)
	// 编码交易索引（后4字节）
	binary.BigEndian.PutUint32(indexValue[40:44], txIndex)

	// 写入交易索引（indices:tx:{txHash} → indexValue）
	txKey := []byte(fmt.Sprintf("indices:tx:%x", txHash))
	if err := m.store.Set(ctx, txKey, indexValue); err != nil {
		return fmt.Errorf("set tx index failed: %w", err)
	}

	return nil
}
