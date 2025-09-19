package repository

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ============================================================================
//                        🔗 区块链状态管理实现
// ============================================================================

// ChainState 区块链状态管理器
//
// 🎯 **系统定位**：区块链状态管理核心
// 负责维护区块链的全局状态信息，包括最高区块、统计信息等。
//
// 核心职责：
// - 最高区块管理：维护当前链的最新状态
// - 状态持久化：确保状态信息的可靠存储
// - 状态查询：提供快速的状态信息查询
// - 一致性保证：确保状态更新的原子性
type ChainState struct {
	storage storage.BadgerStore
}

// UpdateHighestBlockInTransaction 在事务中更新最高区块信息
func (cs *ChainState) UpdateHighestBlockInTransaction(ctx context.Context, tx storage.BadgerTransaction, block *core.Block, blockHash []byte) error {
	// 1. 更新最高高度
	heightBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(heightBytes, block.Header.Height)
	if err := tx.Set([]byte(ChainLatestHeightKey), heightBytes); err != nil {
		return fmt.Errorf("更新最高高度失败: %w", err)
	}

	// 2. 验证并更新最高区块哈希
	if len(blockHash) == 0 {
		return fmt.Errorf("区块哈希不能为空")
	}
	if err := tx.Set([]byte(ChainLatestHashKey), blockHash); err != nil {
		return fmt.Errorf("更新最高区块哈希失败: %w", err)
	}

	// 3. 更新总区块数（递增）
	totalBlocks, _ := cs.getTotalBlocks(ctx)
	totalBlocksBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(totalBlocksBytes, totalBlocks+1)
	if err := tx.Set([]byte(ChainTotalBlocksKey), totalBlocksBytes); err != nil {
		return fmt.Errorf("更新总区块数失败: %w", err)
	}

	// 4. 更新总交易数
	totalTxs, _ := cs.getTotalTransactions(ctx)
	totalTxsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(totalTxsBytes, totalTxs+uint64(len(block.Body.Transactions)))
	if err := tx.Set([]byte(ChainTotalTxsKey), totalTxsBytes); err != nil {
		return fmt.Errorf("更新总交易数失败: %w", err)
	}

	// 5. 更新最后更新时间
	lastUpdateBytes, _ := time.Now().MarshalBinary()
	if err := tx.Set([]byte(ChainLastUpdateKey), lastUpdateBytes); err != nil {
		return fmt.Errorf("更新最后更新时间失败: %w", err)
	}

	return nil
}

// getTotalBlocks 获取当前总区块数
func (cs *ChainState) getTotalBlocks(ctx context.Context) (uint64, error) {
	data, err := cs.storage.Get(ctx, []byte(ChainTotalBlocksKey))
	if err != nil || data == nil {
		return 0, nil
	}
	return binary.BigEndian.Uint64(data), nil
}

// getTotalTransactions 获取当前总交易数
func (cs *ChainState) getTotalTransactions(ctx context.Context) (uint64, error) {
	data, err := cs.storage.Get(ctx, []byte(ChainTotalTxsKey))
	if err != nil || data == nil {
		return 0, nil
	}
	return binary.BigEndian.Uint64(data), nil
}

// ChainStateInfo 区块链状态信息
type ChainStateInfo struct {
	HighestHeight   uint64    `json:"highest_height"`    // 最高区块高度
	HighestHash     []byte    `json:"highest_hash"`      // 最高区块哈希
	TotalBlocks     uint64    `json:"total_blocks"`      // 总区块数量
	TotalTxs        uint64    `json:"total_txs"`         // 总交易数量
	LastUpdatedTime time.Time `json:"last_updated_time"` // 最后更新时间
	GenesisHash     []byte    `json:"genesis_hash"`      // 创世区块哈希
	GenesisTime     time.Time `json:"genesis_time"`      // 创世区块时间
}

// 存储键定义
const (
	ChainLatestHeightKey = "chain:latest_height" // 最高区块高度
	ChainLatestHashKey   = "chain:latest_hash"   // 最高区块哈希
	ChainTotalBlocksKey  = "chain:total_blocks"  // 总区块数量
	ChainTotalTxsKey     = "chain:total_txs"     // 总交易数量
	ChainLastUpdateKey   = "chain:last_update"   // 最后更新时间
	ChainGenesisHashKey  = "chain:genesis_hash"  // 创世区块哈希
	ChainGenesisTimeKey  = "chain:genesis_time"  // 创世区块时间
	ChainInitializedKey  = "chain:initialized"   // 链初始化标志
)

// ============================================================================
//                           状态更新方法
// ============================================================================

// updateChainState 更新区块链状态
//
// 🎯 **系统定位**：状态同步更新核心
// 在事务中原子性更新所有相关的链状态信息
func (m *Manager) updateChainState(ctx context.Context, tx storage.BadgerTransaction, block *core.Block) error {
	height := block.Header.Height
	blockHash, err := m.blockStorage.computeBlockHashWithService(ctx, block)
	if err != nil {
		return fmt.Errorf("计算区块哈希失败: %w", err)
	}

	// 检查是否为创世区块
	if height == 0 {
		return m.initializeGenesisState(tx, block, blockHash)
	}

	return m.updateRegularBlockState(tx, block, blockHash)
}

// initializeGenesisState 初始化创世区块状态
func (m *Manager) initializeGenesisState(tx storage.BadgerTransaction, block *core.Block, blockHash []byte) error {
	now := time.Now()

	// 设置创世区块信息
	if err := tx.Set([]byte(ChainGenesisHashKey), blockHash); err != nil {
		return fmt.Errorf("设置创世区块哈希失败: %w", err)
	}

	genesisTime := time.Unix(int64(block.Header.Timestamp), 0)
	if err := tx.Set([]byte(ChainGenesisTimeKey), timeToBytes(genesisTime)); err != nil {
		return fmt.Errorf("设置创世区块时间失败: %w", err)
	}

	// 初始化统计信息
	if err := tx.Set([]byte(ChainLatestHeightKey), uint64ToBytes(0)); err != nil {
		return err
	}
	if err := tx.Set([]byte(ChainLatestHashKey), blockHash); err != nil {
		return err
	}
	if err := tx.Set([]byte(ChainTotalBlocksKey), uint64ToBytes(1)); err != nil {
		return err
	}
	if err := tx.Set([]byte(ChainTotalTxsKey), uint64ToBytes(uint64(len(block.Body.Transactions)))); err != nil {
		return err
	}
	if err := tx.Set([]byte(ChainLastUpdateKey), timeToBytes(now)); err != nil {
		return err
	}

	// 设置初始化完成标志
	if err := tx.Set([]byte(ChainInitializedKey), []byte("true")); err != nil {
		return err
	}

	return nil
}

// updateRegularBlockState 更新常规区块状态
func (m *Manager) updateRegularBlockState(tx storage.BadgerTransaction, block *core.Block, blockHash []byte) error {
	height := block.Header.Height

	// 检查是否需要更新最高高度
	currentHeightBytes, err := tx.Get([]byte(ChainLatestHeightKey))
	if err == nil && currentHeightBytes != nil {
		currentHeight := bytesToUint64(currentHeightBytes)
		if height <= currentHeight {
			// 不需要更新
			return nil
		}
	}

	now := time.Now()

	// 更新最高区块信息
	if err := tx.Set([]byte(ChainLatestHeightKey), uint64ToBytes(height)); err != nil {
		return fmt.Errorf("更新最高高度失败: %w", err)
	}
	if err := tx.Set([]byte(ChainLatestHashKey), blockHash); err != nil {
		return fmt.Errorf("更新最高区块哈希失败: %w", err)
	}

	// 更新总区块数量
	totalBlocksBytes, err := tx.Get([]byte(ChainTotalBlocksKey))
	var totalBlocks uint64 = 1
	if err == nil && totalBlocksBytes != nil {
		totalBlocks = bytesToUint64(totalBlocksBytes) + 1
	}
	if err := tx.Set([]byte(ChainTotalBlocksKey), uint64ToBytes(totalBlocks)); err != nil {
		return fmt.Errorf("更新总区块数量失败: %w", err)
	}

	// 更新总交易数量
	totalTxsBytes, err := tx.Get([]byte(ChainTotalTxsKey))
	var totalTxs uint64 = uint64(len(block.Body.Transactions))
	if err == nil && totalTxsBytes != nil {
		totalTxs = bytesToUint64(totalTxsBytes) + uint64(len(block.Body.Transactions))
	}
	if err := tx.Set([]byte(ChainTotalTxsKey), uint64ToBytes(totalTxs)); err != nil {
		return fmt.Errorf("更新总交易数量失败: %w", err)
	}

	// 更新最后更新时间
	if err := tx.Set([]byte(ChainLastUpdateKey), timeToBytes(now)); err != nil {
		return fmt.Errorf("更新最后更新时间失败: %w", err)
	}

	return nil
}

// ============================================================================
//                           状态查询方法
// ============================================================================

// getChainState 获取完整的区块链状态信息
//
// 🎯 **系统定位**：状态信息查询核心
// 返回区块链的完整状态信息，用于监控和管理
func (m *Manager) getChainState(ctx context.Context) (*ChainStateInfo, error) {
	if m.logger != nil {
		m.logger.Debug("查询区块链状态信息")
	}

	// 检查链是否已初始化
	initialized, err := m.badgerStore.Get(ctx, []byte(ChainInitializedKey))
	if err != nil || initialized == nil {
		return &ChainStateInfo{}, nil // 返回空状态
	}

	state := &ChainStateInfo{}

	// 获取最高区块信息
	heightBytes, err := m.badgerStore.Get(ctx, []byte(ChainLatestHeightKey))
	if err == nil && heightBytes != nil {
		state.HighestHeight = bytesToUint64(heightBytes)
	}

	state.HighestHash, err = m.badgerStore.Get(ctx, []byte(ChainLatestHashKey))
	if err != nil {
		return nil, fmt.Errorf("获取最高区块哈希失败: %w", err)
	}

	// 获取统计信息
	totalBlocksBytes, err := m.badgerStore.Get(ctx, []byte(ChainTotalBlocksKey))
	if err == nil && totalBlocksBytes != nil {
		state.TotalBlocks = bytesToUint64(totalBlocksBytes)
	}

	totalTxsBytes, err := m.badgerStore.Get(ctx, []byte(ChainTotalTxsKey))
	if err == nil && totalTxsBytes != nil {
		state.TotalTxs = bytesToUint64(totalTxsBytes)
	}

	// 获取时间信息
	lastUpdateBytes, err := m.badgerStore.Get(ctx, []byte(ChainLastUpdateKey))
	if err == nil && lastUpdateBytes != nil {
		state.LastUpdatedTime = bytesToTime(lastUpdateBytes)
	}

	// 获取创世区块信息
	state.GenesisHash, err = m.badgerStore.Get(ctx, []byte(ChainGenesisHashKey))
	if err != nil && state.TotalBlocks > 0 {
		return nil, fmt.Errorf("获取创世区块哈希失败: %w", err)
	}

	genesisTimeBytes, err := m.badgerStore.Get(ctx, []byte(ChainGenesisTimeKey))
	if err == nil && genesisTimeBytes != nil {
		state.GenesisTime = bytesToTime(genesisTimeBytes)
	}

	if m.logger != nil {
		m.logger.Debugf("查询链状态完成 - height: %d, totalBlocks: %d, totalTxs: %d",
			state.HighestHeight, state.TotalBlocks, state.TotalTxs)
	}

	return state, nil
}

// isChainInitialized 检查区块链是否已初始化
func (m *Manager) isChainInitialized(ctx context.Context) (bool, error) {
	initialized, err := m.badgerStore.Get(ctx, []byte(ChainInitializedKey))
	if err != nil {
		return false, err
	}
	return initialized != nil, nil
}

// ============================================================================
//                           辅助函数
// ============================================================================

// timeToBytes 时间转字节数组
func timeToBytes(t time.Time) []byte {
	return uint64ToBytes(uint64(t.Unix()))
}

// bytesToTime 字节数组转时间
func bytesToTime(bytes []byte) time.Time {
	return time.Unix(int64(bytesToUint64(bytes)), 0)
}

// validateChainConsistency 验证区块链状态一致性
//
// 🎯 **系统定位**：状态一致性验证核心
// 验证区块链状态信息的一致性，用于健康检查
func (m *Manager) validateChainConsistency(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Debug("验证区块链状态一致性")
	}

	initialized, err := m.isChainInitialized(ctx)
	if err != nil {
		return fmt.Errorf("检查初始化状态失败: %w", err)
	}
	if !initialized {
		return nil // 未初始化的链不需要验证
	}

	state, err := m.getChainState(ctx)
	if err != nil {
		return fmt.Errorf("获取链状态失败: %w", err)
	}

	// 验证最高区块是否存在
	if state.HighestHash != nil {
		block, err := m.getBlock(ctx, state.HighestHash)
		if err != nil {
			return fmt.Errorf("验证最高区块失败: %w", err)
		}
		if block.Header.Height != state.HighestHeight {
			return fmt.Errorf("最高区块高度不一致: 状态=%d, 区块=%d",
				state.HighestHeight, block.Header.Height)
		}
	}

	if m.logger != nil {
		m.logger.Debug("区块链状态一致性验证通过")
	}

	return nil
}

// repairChainState 修复区块链状态
//
// 🎯 **系统定位**：状态修复核心
// 从区块数据重建区块链状态信息
func (m *Manager) repairChainState(ctx context.Context) error {
	if m.logger != nil {
		m.logger.Debug("开始修复区块链状态")
	}

	// 获取最高区块
	height, blockHash, err := m.getHighestBlock(ctx)
	if err != nil {
		return fmt.Errorf("获取最高区块失败: %w", err)
	}
	if height == 0 && blockHash == nil {
		// 空链，无需修复
		return nil
	}

	// 在事务中修复状态
	return m.badgerStore.RunInTransaction(ctx, func(tx storage.BadgerTransaction) error {
		// 重新计算统计信息
		var totalBlocks uint64 = 0
		var totalTxs uint64 = 0
		var genesisHash []byte
		var genesisTime time.Time

		// 遍历所有区块重新计算
		for h := uint64(0); h <= height; h++ {
			block, err := m.getBlockByHeight(ctx, h)
			if err != nil {
				continue // 跳过不存在的区块
			}

			totalBlocks++
			totalTxs += uint64(len(block.Body.Transactions))

			if h == 0 {
				genesisHash, err = m.blockStorage.computeBlockHashWithService(ctx, block)
				if err != nil {
					return fmt.Errorf("计算创世区块哈希失败: %w", err)
				}
				genesisTime = time.Unix(int64(block.Header.Timestamp), 0)
			}
		}

		// 更新所有状态
		now := time.Now()

		if err := tx.Set([]byte(ChainLatestHeightKey), uint64ToBytes(height)); err != nil {
			return err
		}
		if err := tx.Set([]byte(ChainLatestHashKey), blockHash); err != nil {
			return err
		}
		if err := tx.Set([]byte(ChainTotalBlocksKey), uint64ToBytes(totalBlocks)); err != nil {
			return err
		}
		if err := tx.Set([]byte(ChainTotalTxsKey), uint64ToBytes(totalTxs)); err != nil {
			return err
		}
		if err := tx.Set([]byte(ChainLastUpdateKey), timeToBytes(now)); err != nil {
			return err
		}

		if genesisHash != nil {
			if err := tx.Set([]byte(ChainGenesisHashKey), genesisHash); err != nil {
				return err
			}
			if err := tx.Set([]byte(ChainGenesisTimeKey), timeToBytes(genesisTime)); err != nil {
				return err
			}
		}

		if err := tx.Set([]byte(ChainInitializedKey), []byte("true")); err != nil {
			return err
		}

		if m.logger != nil {
			m.logger.Debugf("区块链状态修复完成 - totalBlocks: %d, totalTxs: %d", totalBlocks, totalTxs)
		}

		return nil
	})
}
