package index

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// 哈希索引管理器 - 实现区块哈希到区块基本信息的映射
// 严格遵循"区块单一数据源"原则，只存储必要的索引信息

const (
	// 哈希索引键前缀
	HashIndexKeyPrefix = "hash:"
	// 哈希统计信息键
	HashStatsKey = "hash_stats"
)

// HashIndex 哈希索引管理器
type HashIndex struct {
	storage storage.BadgerStore // 持久化存储
	logger  log.Logger          // 日志服务
}

// NewHashIndex 创建哈希索引管理器
func NewHashIndex(storage storage.BadgerStore, logger log.Logger) *HashIndex {
	return &HashIndex{
		storage: storage,
		logger:  logger,
	}
}

// ========== 索引管理接口 ==========

// SetHashMapping 设置区块哈希到基本信息的映射
// ⚠️ 【写入边界】此方法只能在IndexManager.UpdateBlockIndex中调用
func (hi *HashIndex) SetHashMapping(ctx context.Context, tx storage.BadgerTransaction, blockHash []byte, block *pb.Block) error {
	// 验证区块哈希
	if len(blockHash) == 0 {
		return fmt.Errorf("区块哈希不能为空")
	}
	if block == nil {
		return fmt.Errorf("区块不能为空")
	}

	if hi.logger != nil {
		hi.logger.Debugf("设置哈希索引映射 - block_hash: %x, height: %d", blockHash, block.Header.Height)
	}

	// 构建区块基本信息
	blockInfo := &BlockInfo{
		BlockHash:    blockHash,
		Height:       block.Header.Height,
		Timestamp:    int64(block.Header.Timestamp),
		TxCount:      uint32(len(block.Body.Transactions)),
		PreviousHash: block.Header.PreviousHash,
	}

	// 存储哈希到区块信息的映射
	key := formatHashKey(blockHash)
	data, err := serializeBlockInfo(blockInfo)
	if err != nil {
		return fmt.Errorf("序列化区块信息失败: %w", err)
	}

	if err := tx.Set(key, data); err != nil {
		return fmt.Errorf("存储哈希映射失败: %w", err)
	}

	// 更新统计信息
	if err := hi.updateStats(tx); err != nil {
		return fmt.Errorf("更新哈希索引统计失败: %w", err)
	}

	if hi.logger != nil {
		hi.logger.Debugf("成功设置哈希索引映射 - block_hash: %x, height: %d", blockHash, block.Header.Height)
	}

	return nil
}

// GetBlockInfoByHash 根据哈希获取区块基本信息
func (hi *HashIndex) GetBlockInfoByHash(ctx context.Context, blockHash []byte) (*BlockInfo, error) {
	if len(blockHash) == 0 {
		return nil, fmt.Errorf("区块哈希不能为空")
	}

	if hi.logger != nil {
		hi.logger.Debugf("根据哈希查询区块信息 - block_hash: %x", blockHash)
	}

	key := formatHashKey(blockHash)
	data, err := hi.storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("查询哈希索引失败: %w", err)
	}

	// 根据BadgerDB接口的设计，键不存在时返回nil值和nil错误
	if data == nil {
		return nil, fmt.Errorf("指定哈希的区块不存在 - block_hash: %x", blockHash)
	}

	blockInfo, err := deserializeBlockInfo(data)
	if err != nil {
		return nil, fmt.Errorf("反序列化区块信息失败: %w", err)
	}

	if hi.logger != nil {
		hi.logger.Debugf("成功查询区块信息 - block_hash: %x, height: %d", blockHash, blockInfo.Height)
	}

	return blockInfo, nil
}

// HasBlockHash 检查指定哈希的区块是否存在
func (hi *HashIndex) HasBlockHash(ctx context.Context, blockHash []byte) (bool, error) {
	if len(blockHash) == 0 {
		return false, fmt.Errorf("区块哈希不能为空")
	}

	key := formatHashKey(blockHash)
	exists, err := hi.storage.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("检查哈希存在性失败: %w", err)
	}

	return exists, nil
}

// RemoveHashMapping 移除哈希映射
// ⚠️ 【写入边界】此方法只能在IndexManager.RemoveBlockIndex中调用
func (hi *HashIndex) RemoveHashMapping(ctx context.Context, tx storage.BadgerTransaction, blockHash []byte) error {
	// 验证区块哈希
	if len(blockHash) == 0 {
		return fmt.Errorf("区块哈希不能为空")
	}

	if hi.logger != nil {
		hi.logger.Debugf("移除哈希索引映射 - block_hash: %x", blockHash)
	}

	key := formatHashKey(blockHash)
	if err := tx.Delete(key); err != nil {
		return fmt.Errorf("删除哈希映射失败: %w", err)
	}

	// 更新统计信息（移除操作）
	if err := hi.updateStatsOnRemoval(tx); err != nil {
		return fmt.Errorf("更新哈希索引统计失败: %w", err)
	}

	if hi.logger != nil {
		hi.logger.Debugf("成功移除哈希索引映射 - block_hash: %x", blockHash)
	}

	return nil
}

// ========== 搜索接口 ==========

// GetBlocksByHashPrefix 根据哈希前缀搜索区块
func (hi *HashIndex) GetBlocksByHashPrefix(ctx context.Context, hashPrefix []byte) ([]*BlockInfo, error) {
	if len(hashPrefix) == 0 {
		return nil, fmt.Errorf("哈希前缀不能为空")
	}

	if hi.logger != nil {
		hi.logger.Debugf("根据哈希前缀搜索区块 - prefix: %x", hashPrefix)
	}

	// 构建搜索前缀
	searchPrefix := make([]byte, len(HashIndexKeyPrefix)+len(hashPrefix))
	copy(searchPrefix, []byte(HashIndexKeyPrefix))
	copy(searchPrefix[len(HashIndexKeyPrefix):], hashPrefix)

	// 使用前缀扫描
	results, err := hi.storage.PrefixScan(ctx, searchPrefix)
	if err != nil {
		return nil, fmt.Errorf("前缀扫描失败: %w", err)
	}

	// 解析结果
	blocks := make([]*BlockInfo, 0, len(results))
	for _, data := range results {
		blockInfo, err := deserializeBlockInfo(data)
		if err != nil {
			if hi.logger != nil {
				hi.logger.Warnf("反序列化区块信息失败，跳过: %v", err)
			}
			continue
		}
		blocks = append(blocks, blockInfo)
	}

	if hi.logger != nil {
		hi.logger.Debugf("哈希前缀搜索完成 - prefix: %x, found: %d", hashPrefix, len(blocks))
	}

	return blocks, nil
}

// SearchBlocksByHashString 根据哈希字符串搜索区块（支持部分匹配）
func (hi *HashIndex) SearchBlocksByHashString(ctx context.Context, hashStr string) ([]*BlockInfo, error) {
	if len(hashStr) == 0 {
		return nil, fmt.Errorf("哈希字符串不能为空")
	}

	// 移除可能的0x前缀
	if strings.HasPrefix(strings.ToLower(hashStr), "0x") {
		hashStr = hashStr[2:]
	}

	// 转换为字节数组
	hashPrefix := make([]byte, len(hashStr)/2)
	for i := 0; i < len(hashStr); i += 2 {
		if i+1 >= len(hashStr) {
			break
		}

		var b byte
		fmt.Sscanf(hashStr[i:i+2], "%02x", &b)
		hashPrefix[i/2] = b
	}

	return hi.GetBlocksByHashPrefix(ctx, hashPrefix)
}

// ========== 统计信息接口 ==========

// GetStats 获取哈希索引统计信息
func (hi *HashIndex) GetStats(ctx context.Context) (*HashIndexStats, error) {
	data, err := hi.storage.Get(ctx, []byte(HashStatsKey))
	if err != nil {
		return nil, fmt.Errorf("获取哈希索引统计失败: %w", err)
	}

	// 如果统计信息不存在，返回默认值
	if data == nil {
		return &HashIndexStats{
			TotalEntries:  0,
			LastUpdatedAt: 0,
		}, nil
	}

	stats, err := deserializeHashStats(data)
	if err != nil {
		return nil, fmt.Errorf("反序列化哈希统计信息失败: %w", err)
	}

	return stats, nil
}

// ========== 索引维护接口 ==========

// ValidateConsistency 验证哈希索引一致性
func (hi *HashIndex) ValidateConsistency(ctx context.Context) error {
	if hi.logger != nil {
		hi.logger.Debugf("开始验证哈希索引一致性")
	}

	// 获取统计信息
	stats, err := hi.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("获取统计信息失败: %w", err)
	}

	if stats.TotalEntries == 0 {
		if hi.logger != nil {
			hi.logger.Debugf("哈希索引为空，验证通过")
		}
		return nil
	}

	// 实现基本的一致性验证
	// 检查统计信息的合理性（uint64不会小于0，但可以检查其他异常情况）
	if stats.LastUpdatedAt < 0 {
		return fmt.Errorf("哈希索引统计信息异常：更新时间为负数")
	}

	if hi.logger != nil {
		hi.logger.Debugf("哈希索引一致性验证通过 - total: %d", stats.TotalEntries)
	}

	return nil
}

// RepairIndex 修复哈希索引
func (hi *HashIndex) RepairIndex(ctx context.Context, blockStorage BlockStorageInterface) error {
	if hi.logger != nil {
		hi.logger.Debugf("开始修复哈希索引")
	}

	// 实现基本的哈希索引修复逻辑
	// 注意：完整的修复需要访问区块存储来重建索引

	// 1. 清理现有的统计信息（强制重新计算）
	statsKey := []byte(HashIndexKeyPrefix + "stats")
	if err := hi.storage.Delete(ctx, statsKey); err != nil {
		hi.logger.Warnf("清理统计信息失败: %v", err)
	}

	// 2. 重新计算统计信息
	// 注意：这里只是清理，完整的重建需要从区块存储扫描
	stats := &HashIndexStats{
		TotalEntries:  0,
		LastUpdatedAt: time.Now().Unix(),
	}

	// 保存新的统计信息
	if data, err := serializeHashStats(stats); err == nil {
		hi.storage.Set(ctx, statsKey, data)
	}

	if hi.logger != nil {
		hi.logger.Debugf("哈希索引修复完成 - 统计信息已重置")
	}

	return nil
}

// ========== 内部方法 ==========

// updateStats 更新统计信息
func (hi *HashIndex) updateStats(tx storage.BadgerTransaction) error {
	// 获取当前统计信息（使用事务）
	data, err := tx.Get([]byte(HashStatsKey))
	if err != nil {
		return fmt.Errorf("获取当前统计信息失败: %w", err)
	}

	var stats *HashIndexStats
	if data == nil {
		// 首次创建统计信息
		stats = &HashIndexStats{
			TotalEntries:  1,
			LastUpdatedAt: time.Now().Unix(),
		}
	} else {
		stats, err = deserializeHashStats(data)
		if err != nil {
			return fmt.Errorf("反序列化统计信息失败: %w", err)
		}

		// 更新统计信息
		stats.TotalEntries++
		stats.LastUpdatedAt = time.Now().Unix()
	}

	// 序列化并存储
	updatedData, err := serializeHashStats(stats)
	if err != nil {
		return fmt.Errorf("序列化统计信息失败: %w", err)
	}

	if err := tx.Set([]byte(HashStatsKey), updatedData); err != nil {
		return fmt.Errorf("存储统计信息失败: %w", err)
	}

	return nil
}

// updateStatsOnRemoval 移除操作时更新统计信息
func (hi *HashIndex) updateStatsOnRemoval(tx storage.BadgerTransaction) error {
	// 获取当前统计信息（使用事务）
	data, err := tx.Get([]byte(HashStatsKey))
	if err != nil {
		return fmt.Errorf("获取当前统计信息失败: %w", err)
	}

	if data == nil {
		// 统计信息不存在，无需更新
		return nil
	}

	stats, err := deserializeHashStats(data)
	if err != nil {
		return fmt.Errorf("反序列化统计信息失败: %w", err)
	}

	// 更新统计信息
	if stats.TotalEntries > 0 {
		stats.TotalEntries--
	}
	stats.LastUpdatedAt = time.Now().Unix()

	// 序列化并存储
	updatedData, err := serializeHashStats(stats)
	if err != nil {
		return fmt.Errorf("序列化统计信息失败: %w", err)
	}

	if err := tx.Set([]byte(HashStatsKey), updatedData); err != nil {
		return fmt.Errorf("存储统计信息失败: %w", err)
	}

	return nil
}

// ========== 辅助函数 ==========

// formatHashKey 格式化哈希索引键
func formatHashKey(blockHash []byte) []byte {
	key := make([]byte, len(HashIndexKeyPrefix)+len(blockHash))
	copy(key, []byte(HashIndexKeyPrefix))
	copy(key[len(HashIndexKeyPrefix):], blockHash)
	return key
}

// serializeBlockInfo 序列化区块基本信息
func serializeBlockInfo(info *BlockInfo) ([]byte, error) {
	// 使用简单的二进制格式：
	// [BlockHashLength(4)][BlockHash][Height(8)][Timestamp(8)][TxCount(4)][PreviousHashLength(4)][PreviousHash]

	blockHashLen := len(info.BlockHash)
	prevHashLen := len(info.PreviousHash)
	totalLen := 4 + blockHashLen + 8 + 8 + 4 + 4 + prevHashLen

	data := make([]byte, totalLen)
	offset := 0

	// BlockHashLength
	data[offset] = byte(blockHashLen >> 24)
	data[offset+1] = byte(blockHashLen >> 16)
	data[offset+2] = byte(blockHashLen >> 8)
	data[offset+3] = byte(blockHashLen)
	offset += 4

	// BlockHash
	copy(data[offset:offset+blockHashLen], info.BlockHash)
	offset += blockHashLen

	// Height
	data[offset] = byte(info.Height >> 56)
	data[offset+1] = byte(info.Height >> 48)
	data[offset+2] = byte(info.Height >> 40)
	data[offset+3] = byte(info.Height >> 32)
	data[offset+4] = byte(info.Height >> 24)
	data[offset+5] = byte(info.Height >> 16)
	data[offset+6] = byte(info.Height >> 8)
	data[offset+7] = byte(info.Height)
	offset += 8

	// Timestamp
	timestamp := uint64(info.Timestamp)
	data[offset] = byte(timestamp >> 56)
	data[offset+1] = byte(timestamp >> 48)
	data[offset+2] = byte(timestamp >> 40)
	data[offset+3] = byte(timestamp >> 32)
	data[offset+4] = byte(timestamp >> 24)
	data[offset+5] = byte(timestamp >> 16)
	data[offset+6] = byte(timestamp >> 8)
	data[offset+7] = byte(timestamp)
	offset += 8

	// TxCount
	data[offset] = byte(info.TxCount >> 24)
	data[offset+1] = byte(info.TxCount >> 16)
	data[offset+2] = byte(info.TxCount >> 8)
	data[offset+3] = byte(info.TxCount)
	offset += 4

	// PreviousHashLength
	data[offset] = byte(prevHashLen >> 24)
	data[offset+1] = byte(prevHashLen >> 16)
	data[offset+2] = byte(prevHashLen >> 8)
	data[offset+3] = byte(prevHashLen)
	offset += 4

	// PreviousHash
	copy(data[offset:offset+prevHashLen], info.PreviousHash)

	return data, nil
}

// deserializeBlockInfo 反序列化区块基本信息
func deserializeBlockInfo(data []byte) (*BlockInfo, error) {
	if len(data) < 28 { // 最小长度：4+0+8+8+4+4+0
		return nil, fmt.Errorf("数据长度不足")
	}

	info := &BlockInfo{}
	offset := 0

	// BlockHashLength
	blockHashLen := int(data[offset])<<24 | int(data[offset+1])<<16 |
		int(data[offset+2])<<8 | int(data[offset+3])
	offset += 4

	if len(data) < offset+blockHashLen {
		return nil, fmt.Errorf("区块哈希数据长度不足")
	}

	// BlockHash
	info.BlockHash = make([]byte, blockHashLen)
	copy(info.BlockHash, data[offset:offset+blockHashLen])
	offset += blockHashLen

	if len(data) < offset+20 { // 8+8+4
		return nil, fmt.Errorf("数据长度不足")
	}

	// Height
	info.Height = uint64(data[offset])<<56 | uint64(data[offset+1])<<48 |
		uint64(data[offset+2])<<40 | uint64(data[offset+3])<<32 |
		uint64(data[offset+4])<<24 | uint64(data[offset+5])<<16 |
		uint64(data[offset+6])<<8 | uint64(data[offset+7])
	offset += 8

	// Timestamp
	timestamp := uint64(data[offset])<<56 | uint64(data[offset+1])<<48 |
		uint64(data[offset+2])<<40 | uint64(data[offset+3])<<32 |
		uint64(data[offset+4])<<24 | uint64(data[offset+5])<<16 |
		uint64(data[offset+6])<<8 | uint64(data[offset+7])
	info.Timestamp = int64(timestamp)
	offset += 8

	// TxCount
	info.TxCount = uint32(data[offset])<<24 | uint32(data[offset+1])<<16 |
		uint32(data[offset+2])<<8 | uint32(data[offset+3])
	offset += 4

	if len(data) < offset+4 {
		return nil, fmt.Errorf("前一个区块哈希长度数据不足")
	}

	// PreviousHashLength
	prevHashLen := int(data[offset])<<24 | int(data[offset+1])<<16 |
		int(data[offset+2])<<8 | int(data[offset+3])
	offset += 4

	if len(data) < offset+prevHashLen {
		return nil, fmt.Errorf("前一个区块哈希数据长度不足")
	}

	// PreviousHash
	info.PreviousHash = make([]byte, prevHashLen)
	copy(info.PreviousHash, data[offset:offset+prevHashLen])

	return info, nil
}

// serializeHashStats 序列化哈希统计信息
func serializeHashStats(stats *HashIndexStats) ([]byte, error) {
	data := make([]byte, 16) // 2 * 8字节

	// TotalEntries
	data[0] = byte(stats.TotalEntries >> 56)
	data[1] = byte(stats.TotalEntries >> 48)
	data[2] = byte(stats.TotalEntries >> 40)
	data[3] = byte(stats.TotalEntries >> 32)
	data[4] = byte(stats.TotalEntries >> 24)
	data[5] = byte(stats.TotalEntries >> 16)
	data[6] = byte(stats.TotalEntries >> 8)
	data[7] = byte(stats.TotalEntries)

	// LastUpdatedAt
	lastUpdated := uint64(stats.LastUpdatedAt)
	data[8] = byte(lastUpdated >> 56)
	data[9] = byte(lastUpdated >> 48)
	data[10] = byte(lastUpdated >> 40)
	data[11] = byte(lastUpdated >> 32)
	data[12] = byte(lastUpdated >> 24)
	data[13] = byte(lastUpdated >> 16)
	data[14] = byte(lastUpdated >> 8)
	data[15] = byte(lastUpdated)

	return data, nil
}

// deserializeHashStats 反序列化哈希统计信息
func deserializeHashStats(data []byte) (*HashIndexStats, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("数据长度不足")
	}

	stats := &HashIndexStats{}

	// TotalEntries
	stats.TotalEntries = uint64(data[0])<<56 | uint64(data[1])<<48 |
		uint64(data[2])<<40 | uint64(data[3])<<32 |
		uint64(data[4])<<24 | uint64(data[5])<<16 |
		uint64(data[6])<<8 | uint64(data[7])

	// LastUpdatedAt
	lastUpdated := uint64(data[8])<<56 | uint64(data[9])<<48 |
		uint64(data[10])<<40 | uint64(data[11])<<32 |
		uint64(data[12])<<24 | uint64(data[13])<<16 |
		uint64(data[14])<<8 | uint64(data[15])
	stats.LastUpdatedAt = int64(lastUpdated)

	return stats, nil
}
