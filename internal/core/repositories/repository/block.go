package repository

import (
	"context"
	"encoding/binary"
	"fmt"

	repositoryConfig "github.com/weisyn/v1/internal/config/repository"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
//                           🏗️ 区块数据操作实现
// ============================================================================

// 存储键前缀定义
const (
	BlockKeyPrefix  = "block:"  // block:<blockHash> -> Block data
	HeightKeyPrefix = "height:" // height:<height> -> blockHash
)

// 区块存储核心组件
type BlockStorage struct {
	storage                storage.BadgerStore
	blockHashServiceClient core.BlockHashServiceClient
	config                 *repositoryConfig.PerformanceConfig // 性能配置
}

// GetBlock 根据区块哈希获取区块
func (bs *BlockStorage) GetBlock(ctx context.Context, blockHash []byte) (*core.Block, error) {
	key := formatBlockKey(blockHash)
	data, err := bs.storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("获取区块数据失败: %w", err)
	}

	if data == nil {
		return nil, fmt.Errorf("区块不存在")
	}

	var block core.Block
	if err := proto.Unmarshal(data, &block); err != nil {
		return nil, fmt.Errorf("反序列化区块数据失败: %w", err)
	}

	return &block, nil
}

// GetBlockByHeight 根据区块高度获取区块
func (bs *BlockStorage) GetBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	// 首先通过高度索引获取区块哈希
	heightKey := formatHeightKey(height)
	blockHashData, err := bs.storage.Get(ctx, heightKey)
	if err != nil {
		return nil, fmt.Errorf("获取高度索引失败: %w", err)
	}

	if blockHashData == nil {
		return nil, fmt.Errorf("指定高度的区块不存在")
	}

	// 然后通过区块哈希获取完整区块
	return bs.GetBlock(ctx, blockHashData)
}

// StoreBlockInTransaction 在事务中存储区块
func (bs *BlockStorage) StoreBlockInTransaction(ctx context.Context, tx storage.BadgerTransaction, block *core.Block) error {
	// 1. 序列化区块数据
	blockData, err := proto.Marshal(block)
	if err != nil {
		return fmt.Errorf("序列化区块失败: %w", err)
	}

	// 2. 计算区块哈希
	blockHash, err := bs.computeBlockHashWithService(ctx, block)
	if err != nil {
		return fmt.Errorf("计算区块哈希失败: %w", err)
	}

	// 3. 存储区块数据
	blockKey := formatBlockKey(blockHash)
	if err := tx.Set(blockKey, blockData); err != nil {
		return fmt.Errorf("存储区块数据失败: %w", err)
	}

	// 4. 存储高度索引
	heightKey := formatHeightKey(block.Header.Height)
	if err := tx.Set(heightKey, blockHash); err != nil {
		return fmt.Errorf("存储高度索引失败: %w", err)
	}

	return nil
}

// computeBlockHashWithService 使用哈希服务计算区块哈希
func (bs *BlockStorage) computeBlockHashWithService(ctx context.Context, block *core.Block) ([]byte, error) {
	req := &core.ComputeBlockHashRequest{
		Block:            block,
		IncludeDebugInfo: false,
	}

	resp, err := bs.blockHashServiceClient.ComputeBlockHash(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("哈希服务调用失败: %w", err)
	}

	if !resp.IsValid {
		return nil, fmt.Errorf("区块结构无效")
	}

	return resp.Hash, nil
}

// 格式化区块存储键
func formatBlockKey(blockHash []byte) []byte {
	key := make([]byte, len(BlockKeyPrefix)+len(blockHash))
	copy(key, BlockKeyPrefix)
	copy(key[len(BlockKeyPrefix):], blockHash)
	return key
}

// 格式化高度索引键
func formatHeightKey(height uint64) []byte {
	key := make([]byte, len(HeightKeyPrefix)+8)
	copy(key, HeightKeyPrefix)
	binary.BigEndian.PutUint64(key[len(HeightKeyPrefix):], height)
	return key
}

// 序列化区块数据
func serializeBlock(block *core.Block) ([]byte, error) {
	return proto.Marshal(block)
}

// 反序列化区块数据
func deserializeBlock(data []byte) (*core.Block, error) {
	block := &core.Block{}
	err := proto.Unmarshal(data, block)
	return block, err
}

// uint64转字节数组
func uint64ToBytes(value uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, value)
	return bytes
}

// 字节数组转uint64
func bytesToUint64(bytes []byte) uint64 {
	return binary.BigEndian.Uint64(bytes)
}

// 验证区块数据完整性
func (m *Manager) validateBlock(block *core.Block) error {
	if block == nil {
		return fmt.Errorf("区块数据为空")
	}
	if block.Header == nil {
		return fmt.Errorf("区块头为空")
	}
	if block.Body == nil {
		return fmt.Errorf("区块体为空")
	}
	if len(block.Header.PreviousHash) != 32 && block.Header.Height != 0 {
		return fmt.Errorf("前一个区块哈希格式错误")
	}
	return nil
}

// getBlock 获取指定哈希的区块
//
// 🎯 **系统定位**：哈希精确查询核心
// 通过区块哈希获取完整区块数据，支持历史数据追溯。
//
// 实现要点：
// - 精确匹配：基于SHA256哈希的精确查询
// - 高性能：直接键值查询，O(1)时间复杂度
// - 完整数据：返回包含所有交易的完整区块
func (m *Manager) getBlock(ctx context.Context, blockHash []byte) (*core.Block, error) {
	if m.logger != nil {
		m.logger.Debugf("查询区块 - blockHash: %x", blockHash)
	}

	// 1. 验证哈希格式（32字节SHA256）
	if len(blockHash) != 32 {
		return nil, fmt.Errorf("无效的区块哈希长度: %d，期望32字节", len(blockHash))
	}

	// 2. 从存储中查询区块数据
	blockKey := formatBlockKey(blockHash)
	blockData, err := m.badgerStore.Get(ctx, blockKey)
	if err != nil {
		return nil, fmt.Errorf("查询区块数据失败: %w", err)
	}
	if blockData == nil {
		return nil, fmt.Errorf("区块不存在")
	}

	// 3. 反序列化区块数据
	block, err := deserializeBlock(blockData)
	if err != nil {
		return nil, fmt.Errorf("反序列化区块失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Debugf("成功查询区块 - height: %d, txCount: %d", block.Header.Height, len(block.Body.Transactions))
	}

	return block, nil
}

// getBlockByHeight 按高度获取区块
//
// 🎯 **系统定位**：高度索引查询核心
// 通过区块高度获取区块数据，支持基于高度的链式验证。
//
// 实现要点：
// - 高度映射：通过HeightIndex进行高效查询
// - 唯一性：每个高度对应唯一区块
// - 完整数据：返回包含所有交易的完整区块
func (m *Manager) getBlockByHeight(ctx context.Context, height uint64) (*core.Block, error) {
	if m.logger != nil {
		m.logger.Debugf("按高度查询区块 - height: %d", height)
	}

	// 1. 通过高度索引获取区块哈希
	heightKey := formatHeightKey(height)
	blockHash, err := m.badgerStore.Get(ctx, heightKey)
	if err != nil {
		return nil, fmt.Errorf("查询高度索引失败: %w", err)
	}
	if blockHash == nil {
		return nil, fmt.Errorf("指定高度的区块不存在: %d", height)
	}

	// 2. 使用区块哈希查询完整区块
	block, err := m.getBlock(ctx, blockHash)
	if err != nil {
		return nil, fmt.Errorf("通过哈希查询区块失败: %w", err)
	}

	return block, nil
}

// getBlockRange 获取区块高度范围
//
// 🎯 **系统定位**：批量区块查询核心
// 获取指定高度范围内的所有区块，支持区块同步、数据分析。
//
// 实现要点：
// - 范围查询：支持指定起始和结束高度的连续查询
// - 批量优化：一次性获取多个区块，减少查询开销
// - 顺序返回：严格按照高度升序返回区块列表
// - 边界处理：自动处理不存在的高度，只返回有效区块
func (m *Manager) getBlockRange(ctx context.Context, startHeight, endHeight uint64) ([]*core.Block, error) {
	if m.logger != nil {
		m.logger.Debugf("查询区块范围 - startHeight: %d, endHeight: %d", startHeight, endHeight)
	}

	// 1. 验证高度范围参数
	if startHeight > endHeight {
		return nil, fmt.Errorf("起始高度不能大于结束高度: start=%d, end=%d", startHeight, endHeight)
	}

	// 从配置中获取最大查询范围
	maxRangeSize := uint64(m.config.Performance.MaxBlockRangeSize)
	rangeSize := endHeight - startHeight + 1
	if rangeSize > maxRangeSize {
		return nil, fmt.Errorf("查询范围过大: %d，最大允许: %d", rangeSize, maxRangeSize)
	}

	// 2. 批量获取指定范围内的区块
	blocks := make([]*core.Block, 0, rangeSize)

	for height := startHeight; height <= endHeight; height++ {
		block, err := m.getBlockByHeight(ctx, height)
		if err != nil {
			// 如果区块不存在，跳过并继续（边界处理）
			if m.logger != nil {
				m.logger.Debugf("跳过不存在的区块 - height: %d, error: %v", height, err)
			}
			continue
		}
		blocks = append(blocks, block)
	}

	if m.logger != nil {
		m.logger.Debugf("成功查询区块范围 - 请求范围: %d-%d, 实际获取: %d个区块",
			startHeight, endHeight, len(blocks))
	}

	return blocks, nil
}

// getHighestBlock 获取最高区块信息
//
// 🎯 **系统定位**：链状态查询核心
// 返回当前区块链的最高区块信息（高度和哈希），为系统管理提供关键信息。
//
// 实现要点：
// - 链状态获取：返回当前链的最新状态信息
// - 轻量级查询：只返回高度和哈希，不返回完整区块数据
// - 实时状态：反映当前链的最新状态
func (m *Manager) getHighestBlock(ctx context.Context) (height uint64, blockHash []byte, err error) {
	if m.logger != nil {
		m.logger.Debug("查询最高区块")
	}

	// 通过链状态管理获取最高区块信息
	state, err := m.getChainState(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("获取链状态失败: %w", err)
	}

	if state.HighestHeight == 0 && state.HighestHash == nil {
		// 链为空的情况
		return 0, nil, nil
	}

	if m.logger != nil {
		m.logger.Debugf("成功查询最高区块 - height: %d, hash: %x", state.HighestHeight, state.HighestHash)
	}

	return state.HighestHeight, state.HighestHash, nil
}
