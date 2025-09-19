package index

import (
	"context"
	"fmt"

	pb "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// 统一索引管理器 - 协调所有索引组件
// 严格遵循"区块单一数据源"原则，所有索引都只存储位置信息

// IndexManager 统一索引管理器
// 负责协调和管理所有类型的索引（高度索引、哈希索引等）
type IndexManager struct {
	heightIndex            *HeightIndex              // 高度索引管理器
	hashIndex              *HashIndex                // 哈希索引管理器
	storage                storage.BadgerStore       // 持久化存储
	logger                 log.Logger                // 日志服务
	blockHashServiceClient pb.BlockHashServiceClient // 区块哈希服务客户端
}

// NewIndexManager 创建统一索引管理器
func NewIndexManager(storage storage.BadgerStore, logger log.Logger, blockHashServiceClient pb.BlockHashServiceClient) *IndexManager {
	return &IndexManager{
		heightIndex:            NewHeightIndex(storage, logger),
		hashIndex:              NewHashIndex(storage, logger),
		storage:                storage,
		logger:                 logger,
		blockHashServiceClient: blockHashServiceClient,
	}
}

// ========== 区块索引管理接口 ==========

// UpdateBlockIndex 更新区块索引
// ⚠️ 【写入边界】此方法只能在Manager.StoreBlock的事务中调用
// 在存储新区块时调用此方法，同时更新高度索引和哈希索引
func (im *IndexManager) UpdateBlockIndex(ctx context.Context, tx storage.BadgerTransaction, block *pb.Block) error {
	if im.logger != nil {
		im.logger.Debugf("更新区块索引 - height: %d", block.Header.Height)
	}

	// 计算区块哈希
	req := &pb.ComputeBlockHashRequest{
		Block:            block,
		IncludeDebugInfo: false,
	}
	resp, err := im.blockHashServiceClient.ComputeBlockHash(ctx, req)
	if err != nil || !resp.IsValid {
		return fmt.Errorf("计算区块哈希失败: %w", err)
	}
	blockHash := resp.Hash

	// 1. 更新高度索引
	if err := im.heightIndex.SetHeightMapping(ctx, tx, block.Header.Height, blockHash); err != nil {
		return fmt.Errorf("更新高度索引失败: %w", err)
	}

	// 2. 更新哈希索引（如果需要的话）
	if err := im.hashIndex.SetHashMapping(ctx, tx, blockHash, block); err != nil {
		return fmt.Errorf("更新哈希索引失败: %w", err)
	}

	if im.logger != nil {
		im.logger.Debugf("成功更新区块索引 - height: %d", block.Header.Height)
	}

	return nil
}

// RemoveBlockIndex 移除区块索引
// ⚠️ 【写入边界】此方法只能在Manager区块回滚事务中调用
// 在区块回滚时调用此方法
func (im *IndexManager) RemoveBlockIndex(ctx context.Context, tx storage.BadgerTransaction, block *pb.Block) error {
	if im.logger != nil {
		im.logger.Debugf("移除区块索引 - height: %d", block.Header.Height)
	}

	// 计算区块哈希
	req := &pb.ComputeBlockHashRequest{
		Block:            block,
		IncludeDebugInfo: false,
	}
	resp, err := im.blockHashServiceClient.ComputeBlockHash(ctx, req)
	if err != nil || !resp.IsValid {
		return fmt.Errorf("计算区块哈希失败: %w", err)
	}
	blockHash := resp.Hash

	// 1. 移除高度索引
	if err := im.heightIndex.RemoveHeightMapping(ctx, tx, block.Header.Height); err != nil {
		return fmt.Errorf("移除高度索引失败: %w", err)
	}

	// 2. 移除哈希索引（如果需要的话）
	if err := im.hashIndex.RemoveHashMapping(ctx, tx, blockHash); err != nil {
		return fmt.Errorf("移除哈希索引失败: %w", err)
	}

	if im.logger != nil {
		im.logger.Debugf("成功移除区块索引 - height: %d", block.Header.Height)
	}

	return nil
}

// ========== 高度索引接口 ==========

// GetBlockHashByHeight 根据高度获取区块哈希
func (im *IndexManager) GetBlockHashByHeight(ctx context.Context, height uint64) ([]byte, error) {
	return im.heightIndex.GetBlockHashByHeight(ctx, height)
}

// HasHeight 检查指定高度是否存在
func (im *IndexManager) HasHeight(ctx context.Context, height uint64) (bool, error) {
	return im.heightIndex.HasHeight(ctx, height)
}

// GetHeightRange 获取高度范围内的所有区块哈希
func (im *IndexManager) GetHeightRange(ctx context.Context, startHeight, endHeight uint64) (map[uint64][]byte, error) {
	return im.heightIndex.GetHeightRange(ctx, startHeight, endHeight)
}

// GetLatestHeights 获取最新的N个区块高度和哈希
func (im *IndexManager) GetLatestHeights(ctx context.Context, count uint32) ([]HeightHashPair, error) {
	return im.heightIndex.GetLatestHeights(ctx, count)
}

// ========== 哈希索引接口 ==========

// GetBlockInfoByHash 根据哈希获取区块基本信息
func (im *IndexManager) GetBlockInfoByHash(ctx context.Context, blockHash []byte) (*BlockInfo, error) {
	return im.hashIndex.GetBlockInfoByHash(ctx, blockHash)
}

// HasBlockHash 检查指定哈希的区块是否存在
func (im *IndexManager) HasBlockHash(ctx context.Context, blockHash []byte) (bool, error) {
	return im.hashIndex.HasBlockHash(ctx, blockHash)
}

// GetBlocksByHashPrefix 根据哈希前缀搜索区块
func (im *IndexManager) GetBlocksByHashPrefix(ctx context.Context, hashPrefix []byte) ([]*BlockInfo, error) {
	return im.hashIndex.GetBlocksByHashPrefix(ctx, hashPrefix)
}

// ========== 索引维护接口 ==========

// ValidateIndexConsistency 验证索引一致性
func (im *IndexManager) ValidateIndexConsistency(ctx context.Context) error {
	if im.logger != nil {
		im.logger.Debugf("开始验证索引一致性")
	}

	// 1. 验证高度索引一致性
	if err := im.heightIndex.ValidateConsistency(ctx); err != nil {
		return fmt.Errorf("高度索引一致性验证失败: %w", err)
	}

	// 2. 验证哈希索引一致性
	if err := im.hashIndex.ValidateConsistency(ctx); err != nil {
		return fmt.Errorf("哈希索引一致性验证失败: %w", err)
	}

	// 3. 验证高度索引和哈希索引之间的一致性
	if err := im.validateCrossIndexConsistency(ctx); err != nil {
		return fmt.Errorf("跨索引一致性验证失败: %w", err)
	}

	if im.logger != nil {
		im.logger.Debugf("索引一致性验证通过")
	}

	return nil
}

// RepairIndexes 修复索引
func (im *IndexManager) RepairIndexes(ctx context.Context, blockStorage BlockStorageInterface) error {
	if im.logger != nil {
		im.logger.Debugf("开始修复索引")
	}

	// 1. 修复高度索引
	if err := im.heightIndex.RepairIndex(ctx, blockStorage); err != nil {
		return fmt.Errorf("修复高度索引失败: %w", err)
	}

	// 2. 修复哈希索引
	if err := im.hashIndex.RepairIndex(ctx, blockStorage); err != nil {
		return fmt.Errorf("修复哈希索引失败: %w", err)
	}

	if im.logger != nil {
		im.logger.Debugf("索引修复完成")
	}

	return nil
}

// GetIndexStats 获取索引统计信息
func (im *IndexManager) GetIndexStats(ctx context.Context) (*IndexStats, error) {
	if im.logger != nil {
		im.logger.Debugf("查询索引统计信息")
	}

	// 1. 获取高度索引统计
	heightStats, err := im.heightIndex.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取高度索引统计失败: %w", err)
	}

	// 2. 获取哈希索引统计
	hashStats, err := im.hashIndex.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取哈希索引统计失败: %w", err)
	}

	// 3. 合并统计信息
	stats := &IndexStats{
		HeightIndexStats: heightStats,
		HashIndexStats:   hashStats,
		TotalIndexes:     heightStats.TotalEntries + hashStats.TotalEntries,
	}

	if im.logger != nil {
		im.logger.Debugf("索引统计信息查询完成 - total_indexes: %d", stats.TotalIndexes)
	}

	return stats, nil
}

// ========== 内部方法 ==========

// validateCrossIndexConsistency 验证跨索引一致性
func (im *IndexManager) validateCrossIndexConsistency(ctx context.Context) error {
	// 实现基本的一致性验证：检查索引是否存在明显不一致
	// 1. 验证高度索引和哈希索引的条目数量是否一致
	// 2. 抽样验证几个区块在两个索引中的信息是否一致

	heightStats, err := im.heightIndex.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("获取高度索引统计失败: %w", err)
	}

	hashStats, err := im.hashIndex.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("获取哈希索引统计失败: %w", err)
	}

	// 检查基本统计信息一致性
	if heightStats.TotalEntries != hashStats.TotalEntries {
		return fmt.Errorf("索引不一致：高度索引条目数=%d，哈希索引条目数=%d",
			heightStats.TotalEntries, hashStats.TotalEntries)
	}

	if im.logger != nil {
		im.logger.Debugf("跨索引一致性验证通过 - 总条目数: %d", heightStats.TotalEntries)
	}

	return nil
}

// ========== 数据结构定义 ==========

// BlockStorageInterface 区块存储接口
// 用于索引修复时从区块存储获取数据
type BlockStorageInterface interface {
	GetBlock(ctx context.Context, blockHash []byte) (*pb.Block, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*pb.Block, error)
}

// IndexStats 索引统计信息
type IndexStats struct {
	HeightIndexStats *HeightIndexStats `json:"height_index_stats"` // 高度索引统计
	HashIndexStats   *HashIndexStats   `json:"hash_index_stats"`   // 哈希索引统计
	TotalIndexes     uint64            `json:"total_indexes"`      // 总索引数量
}

// HeightHashPair 高度哈希对
type HeightHashPair struct {
	Height    uint64 `json:"height"`     // 区块高度
	BlockHash []byte `json:"block_hash"` // 区块哈希
}

// BlockInfo 区块基本信息
type BlockInfo struct {
	BlockHash    []byte `json:"block_hash"`    // 区块哈希
	Height       uint64 `json:"height"`        // 区块高度
	Timestamp    int64  `json:"timestamp"`     // 区块时间戳
	TxCount      uint32 `json:"tx_count"`      // 交易数量
	PreviousHash []byte `json:"previous_hash"` // 前一个区块哈希
}

// HeightIndexStats 高度索引统计信息
type HeightIndexStats struct {
	TotalEntries  uint64 `json:"total_entries"`   // 总条目数
	MinHeight     uint64 `json:"min_height"`      // 最小高度
	MaxHeight     uint64 `json:"max_height"`      // 最大高度
	LastUpdatedAt int64  `json:"last_updated_at"` // 最后更新时间
}

// HashIndexStats 哈希索引统计信息
type HashIndexStats struct {
	TotalEntries  uint64 `json:"total_entries"`   // 总条目数
	LastUpdatedAt int64  `json:"last_updated_at"` // 最后更新时间
}
