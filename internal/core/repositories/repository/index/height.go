package index

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// 高度索引管理器 - 实现高度到区块哈希的映射
// 严格遵循"区块单一数据源"原则，只存储位置信息

const (
	// 高度索引键前缀
	HeightIndexKeyPrefix = "height:"
	// 高度统计信息键
	HeightStatsKey = "height_stats"
)

// HeightIndex 高度索引管理器
type HeightIndex struct {
	storage storage.BadgerStore // 持久化存储
	logger  log.Logger          // 日志服务
}

// NewHeightIndex 创建高度索引管理器
func NewHeightIndex(storage storage.BadgerStore, logger log.Logger) *HeightIndex {
	return &HeightIndex{
		storage: storage,
		logger:  logger,
	}
}

// ========== 索引管理接口 ==========

// SetHeightMapping 设置高度到区块哈希的映射
// ⚠️ 【写入边界】此方法只能在IndexManager.UpdateBlockIndex中调用
func (hi *HeightIndex) SetHeightMapping(ctx context.Context, tx storage.BadgerTransaction, height uint64, blockHash []byte) error {
	if hi.logger != nil {
		hi.logger.Debugf("设置高度索引映射 - height: %d", height)
	}

	// 验证区块哈希
	if len(blockHash) == 0 {
		return fmt.Errorf("区块哈希不能为空")
	}

	// 存储高度到区块哈希的映射
	key := formatHeightKey(height)
	if err := tx.Set(key, blockHash); err != nil {
		return fmt.Errorf("存储高度映射失败: %w", err)
	}

	// 更新统计信息
	if err := hi.updateStats(tx, height); err != nil {
		return fmt.Errorf("更新高度索引统计失败: %w", err)
	}

	if hi.logger != nil {
		hi.logger.Debugf("成功设置高度索引映射 - height: %d, block_hash: %x", height, blockHash)
	}

	return nil
}

// GetBlockHashByHeight 根据高度获取区块哈希
func (hi *HeightIndex) GetBlockHashByHeight(ctx context.Context, height uint64) ([]byte, error) {
	if hi.logger != nil {
		hi.logger.Debugf("根据高度查询区块哈希 - height: %d", height)
	}

	key := formatHeightKey(height)
	blockHash, err := hi.storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("查询高度索引失败: %w", err)
	}

	// 根据BadgerDB接口的设计，键不存在时返回nil值和nil错误
	if blockHash == nil {
		return nil, fmt.Errorf("指定高度的区块不存在 - height: %d", height)
	}

	if hi.logger != nil {
		hi.logger.Debugf("成功查询区块哈希 - height: %d, block_hash: %x", height, blockHash)
	}

	return blockHash, nil
}

// HasHeight 检查指定高度是否存在
func (hi *HeightIndex) HasHeight(ctx context.Context, height uint64) (bool, error) {
	key := formatHeightKey(height)
	exists, err := hi.storage.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("检查高度存在性失败: %w", err)
	}

	return exists, nil
}

// RemoveHeightMapping 移除高度映射
// ⚠️ 【写入边界】此方法只能在IndexManager.RemoveBlockIndex中调用
func (hi *HeightIndex) RemoveHeightMapping(ctx context.Context, tx storage.BadgerTransaction, height uint64) error {
	if hi.logger != nil {
		hi.logger.Debugf("移除高度索引映射 - height: %d", height)
	}

	key := formatHeightKey(height)
	if err := tx.Delete(key); err != nil {
		return fmt.Errorf("删除高度映射失败: %w", err)
	}

	// 更新统计信息（移除操作）
	if err := hi.updateStatsOnRemoval(ctx, tx, height); err != nil {
		return fmt.Errorf("更新高度索引统计失败: %w", err)
	}

	if hi.logger != nil {
		hi.logger.Debugf("成功移除高度索引映射 - height: %d", height)
	}

	return nil
}

// ========== 范围查询接口 ==========

// GetHeightRange 获取高度范围内的所有区块哈希
func (hi *HeightIndex) GetHeightRange(ctx context.Context, startHeight, endHeight uint64) (map[uint64][]byte, error) {
	if startHeight > endHeight {
		return nil, fmt.Errorf("起始高度不能大于结束高度 - start: %d, end: %d", startHeight, endHeight)
	}

	if hi.logger != nil {
		hi.logger.Debugf("查询高度范围 - start: %d, end: %d", startHeight, endHeight)
	}

	result := make(map[uint64][]byte)

	// 逐个查询范围内的高度
	for height := startHeight; height <= endHeight; height++ {
		blockHash, err := hi.GetBlockHashByHeight(ctx, height)
		if err != nil {
			// 如果某个高度不存在，记录警告但继续处理
			if hi.logger != nil {
				hi.logger.Warnf("高度 %d 不存在，跳过", height)
			}
			continue
		}
		result[height] = blockHash
	}

	if hi.logger != nil {
		hi.logger.Debugf("高度范围查询完成 - start: %d, end: %d, found: %d",
			startHeight, endHeight, len(result))
	}

	return result, nil
}

// GetLatestHeights 获取最新的N个区块高度和哈希
func (hi *HeightIndex) GetLatestHeights(ctx context.Context, count uint32) ([]HeightHashPair, error) {
	if count == 0 {
		return []HeightHashPair{}, nil
	}

	if hi.logger != nil {
		hi.logger.Debugf("查询最新区块高度 - count: %d", count)
	}

	// 获取统计信息以确定最大高度
	stats, err := hi.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取高度索引统计失败: %w", err)
	}

	if stats.TotalEntries == 0 {
		return []HeightHashPair{}, nil
	}

	// 计算查询范围
	maxHeight := stats.MaxHeight
	startHeight := maxHeight
	if uint64(count) < maxHeight {
		startHeight = maxHeight - uint64(count) + 1
	} else {
		startHeight = stats.MinHeight
	}

	// 查询范围内的高度
	heightMap, err := hi.GetHeightRange(ctx, startHeight, maxHeight)
	if err != nil {
		return nil, fmt.Errorf("查询高度范围失败: %w", err)
	}

	// 转换为有序列表（从高到低）
	result := make([]HeightHashPair, 0, len(heightMap))
	heights := make([]uint64, 0, len(heightMap))
	for height := range heightMap {
		heights = append(heights, height)
	}

	// 按高度降序排序
	sort.Slice(heights, func(i, j int) bool {
		return heights[i] > heights[j]
	})

	// 构建结果
	for _, height := range heights {
		result = append(result, HeightHashPair{
			Height:    height,
			BlockHash: heightMap[height],
		})
	}

	if hi.logger != nil {
		hi.logger.Debugf("最新区块高度查询完成 - count: %d, found: %d", count, len(result))
	}

	return result, nil
}

// ========== 统计信息接口 ==========

// GetStats 获取高度索引统计信息
func (hi *HeightIndex) GetStats(ctx context.Context) (*HeightIndexStats, error) {
	data, err := hi.storage.Get(ctx, []byte(HeightStatsKey))
	if err != nil {
		return nil, fmt.Errorf("获取高度索引统计失败: %w", err)
	}

	// 如果统计信息不存在，返回默认值
	if data == nil {
		return &HeightIndexStats{
			TotalEntries:  0,
			MinHeight:     0,
			MaxHeight:     0,
			LastUpdatedAt: 0,
		}, nil
	}

	stats, err := deserializeHeightStats(data)
	if err != nil {
		return nil, fmt.Errorf("反序列化高度统计信息失败: %w", err)
	}

	return stats, nil
}

// ========== 索引维护接口 ==========

// ValidateConsistency 验证高度索引一致性
func (hi *HeightIndex) ValidateConsistency(ctx context.Context) error {
	if hi.logger != nil {
		hi.logger.Debugf("开始验证高度索引一致性")
	}

	// 获取统计信息
	stats, err := hi.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("获取统计信息失败: %w", err)
	}

	if stats.TotalEntries == 0 {
		if hi.logger != nil {
			hi.logger.Debugf("高度索引为空，验证通过")
		}
		return nil
	}

	// 验证连续性（检查是否有缺失的高度）
	missingHeights := []uint64{}
	for height := stats.MinHeight; height <= stats.MaxHeight; height++ {
		exists, err := hi.HasHeight(ctx, height)
		if err != nil {
			return fmt.Errorf("检查高度 %d 存在性失败: %w", height, err)
		}
		if !exists {
			missingHeights = append(missingHeights, height)
		}
	}

	if len(missingHeights) > 0 {
		return fmt.Errorf("发现缺失的高度: %v", missingHeights)
	}

	if hi.logger != nil {
		hi.logger.Debugf("高度索引一致性验证通过 - total: %d, range: %d-%d",
			stats.TotalEntries, stats.MinHeight, stats.MaxHeight)
	}

	return nil
}

// RepairIndex 修复高度索引
func (hi *HeightIndex) RepairIndex(ctx context.Context, blockStorage BlockStorageInterface) error {
	if hi.logger != nil {
		hi.logger.Debugf("开始修复高度索引")
	}

	// 实现基本的高度索引修复逻辑
	// 注意：完整的修复需要访问区块存储来重建索引

	// 1. 清理现有的统计信息（强制重新计算）
	statsKey := []byte(HeightIndexKeyPrefix + "stats")
	if err := hi.storage.Delete(ctx, statsKey); err != nil {
		hi.logger.Warnf("清理统计信息失败: %v", err)
	}

	// 2. 重新计算统计信息
	// 注意：这里只是清理，完整的重建需要从区块存储扫描
	stats := &HeightIndexStats{
		TotalEntries:  0,
		MinHeight:     0,
		MaxHeight:     0,
		LastUpdatedAt: time.Now().Unix(),
	}

	// 保存新的统计信息
	if data, err := serializeHeightStats(stats); err == nil {
		hi.storage.Set(ctx, statsKey, data)
	}

	if hi.logger != nil {
		hi.logger.Debugf("高度索引修复完成 - 统计信息已重置")
	}

	return nil
}

// ========== 内部方法 ==========

// updateStats 更新统计信息
func (hi *HeightIndex) updateStats(tx storage.BadgerTransaction, height uint64) error {
	// 获取当前统计信息（使用事务）
	data, err := tx.Get([]byte(HeightStatsKey))
	if err != nil {
		return fmt.Errorf("获取当前统计信息失败: %w", err)
	}

	var stats *HeightIndexStats
	if data == nil {
		// 首次创建统计信息
		stats = &HeightIndexStats{
			TotalEntries:  1,
			MinHeight:     height,
			MaxHeight:     height,
			LastUpdatedAt: time.Now().Unix(),
		}
	} else {
		stats, err = deserializeHeightStats(data)
		if err != nil {
			return fmt.Errorf("反序列化统计信息失败: %w", err)
		}

		// 更新统计信息
		stats.TotalEntries++
		if height < stats.MinHeight {
			stats.MinHeight = height
		}
		if height > stats.MaxHeight {
			stats.MaxHeight = height
		}
		stats.LastUpdatedAt = time.Now().Unix()
	}

	// 序列化并存储
	updatedData, err := serializeHeightStats(stats)
	if err != nil {
		return fmt.Errorf("序列化统计信息失败: %w", err)
	}

	if err := tx.Set([]byte(HeightStatsKey), updatedData); err != nil {
		return fmt.Errorf("存储统计信息失败: %w", err)
	}

	return nil
}

// updateStatsOnRemoval 移除操作时更新统计信息
func (hi *HeightIndex) updateStatsOnRemoval(ctx context.Context, tx storage.BadgerTransaction, height uint64) error {
	// 获取当前统计信息（使用事务）
	data, err := tx.Get([]byte(HeightStatsKey))
	if err != nil {
		return fmt.Errorf("获取当前统计信息失败: %w", err)
	}

	if data == nil {
		// 统计信息不存在，无需更新
		return nil
	}

	stats, err := deserializeHeightStats(data)
	if err != nil {
		return fmt.Errorf("反序列化统计信息失败: %w", err)
	}

	// 更新统计信息
	if stats.TotalEntries > 0 {
		stats.TotalEntries--
	}
	stats.LastUpdatedAt = time.Now().Unix()

	// 如果是边界高度，需要重新计算边界
	if height == stats.MinHeight || height == stats.MaxHeight {
		if hi.logger != nil {
			hi.logger.Warnf("删除了边界高度 %d，重新计算最小/最大高度", height)
		}

		// 重新计算边界值
		if err := hi.recalculateBoundaries(ctx, stats); err != nil {
			return fmt.Errorf("重新计算边界失败: %w", err)
		}
	}

	// 序列化并存储
	updatedData, err := serializeHeightStats(stats)
	if err != nil {
		return fmt.Errorf("序列化统计信息失败: %w", err)
	}

	if err := tx.Set([]byte(HeightStatsKey), updatedData); err != nil {
		return fmt.Errorf("存储统计信息失败: %w", err)
	}

	return nil
}

// recalculateBoundaries 重新计算高度边界值
func (hi *HeightIndex) recalculateBoundaries(ctx context.Context, stats *HeightIndexStats) error {
	if stats.TotalEntries == 0 {
		// 如果没有条目，重置边界
		stats.MinHeight = 0
		stats.MaxHeight = 0
		return nil
	}

	var newMinHeight, newMaxHeight uint64
	var foundFirst bool

	// 使用前缀扫描获取所有高度索引
	prefix := []byte(HeightIndexKeyPrefix)
	results, err := hi.storage.PrefixScan(ctx, prefix)
	if err != nil {
		return fmt.Errorf("扫描高度索引失败: %w", err)
	}

	// 遍历所有高度索引条目
	for keyStr := range results {
		key := []byte(keyStr)

		// 解析高度值（跳过前缀）
		if len(key) < len(HeightIndexKeyPrefix)+8 {
			continue // 跳过无效键
		}

		heightBytes := key[len(HeightIndexKeyPrefix):]
		height := uint64(heightBytes[0])<<56 | uint64(heightBytes[1])<<48 |
			uint64(heightBytes[2])<<40 | uint64(heightBytes[3])<<32 |
			uint64(heightBytes[4])<<24 | uint64(heightBytes[5])<<16 |
			uint64(heightBytes[6])<<8 | uint64(heightBytes[7])

		if !foundFirst {
			newMinHeight = height
			newMaxHeight = height
			foundFirst = true
		} else {
			if height < newMinHeight {
				newMinHeight = height
			}
			if height > newMaxHeight {
				newMaxHeight = height
			}
		}
	}

	if foundFirst {
		stats.MinHeight = newMinHeight
		stats.MaxHeight = newMaxHeight
		if hi.logger != nil {
			hi.logger.Debugf("重新计算边界完成 - min: %d, max: %d", newMinHeight, newMaxHeight)
		}
	} else {
		// 没有找到任何条目，重置为0
		stats.MinHeight = 0
		stats.MaxHeight = 0
		stats.TotalEntries = 0
	}

	return nil
}

// ========== 辅助函数 ==========

// formatHeightKey 格式化高度索引键
func formatHeightKey(height uint64) []byte {
	key := make([]byte, len(HeightIndexKeyPrefix)+8)
	copy(key, []byte(HeightIndexKeyPrefix))

	// 将高度转换为大端序字节数组（保证排序正确）
	offset := len(HeightIndexKeyPrefix)
	key[offset] = byte(height >> 56)
	key[offset+1] = byte(height >> 48)
	key[offset+2] = byte(height >> 40)
	key[offset+3] = byte(height >> 32)
	key[offset+4] = byte(height >> 24)
	key[offset+5] = byte(height >> 16)
	key[offset+6] = byte(height >> 8)
	key[offset+7] = byte(height)

	return key
}

// serializeHeightStats 序列化高度统计信息
func serializeHeightStats(stats *HeightIndexStats) ([]byte, error) {
	// 使用简单的二进制格式
	data := make([]byte, 32) // 4 * 8字节

	// TotalEntries
	data[0] = byte(stats.TotalEntries >> 56)
	data[1] = byte(stats.TotalEntries >> 48)
	data[2] = byte(stats.TotalEntries >> 40)
	data[3] = byte(stats.TotalEntries >> 32)
	data[4] = byte(stats.TotalEntries >> 24)
	data[5] = byte(stats.TotalEntries >> 16)
	data[6] = byte(stats.TotalEntries >> 8)
	data[7] = byte(stats.TotalEntries)

	// MinHeight
	data[8] = byte(stats.MinHeight >> 56)
	data[9] = byte(stats.MinHeight >> 48)
	data[10] = byte(stats.MinHeight >> 40)
	data[11] = byte(stats.MinHeight >> 32)
	data[12] = byte(stats.MinHeight >> 24)
	data[13] = byte(stats.MinHeight >> 16)
	data[14] = byte(stats.MinHeight >> 8)
	data[15] = byte(stats.MinHeight)

	// MaxHeight
	data[16] = byte(stats.MaxHeight >> 56)
	data[17] = byte(stats.MaxHeight >> 48)
	data[18] = byte(stats.MaxHeight >> 40)
	data[19] = byte(stats.MaxHeight >> 32)
	data[20] = byte(stats.MaxHeight >> 24)
	data[21] = byte(stats.MaxHeight >> 16)
	data[22] = byte(stats.MaxHeight >> 8)
	data[23] = byte(stats.MaxHeight)

	// LastUpdatedAt
	lastUpdated := uint64(stats.LastUpdatedAt)
	data[24] = byte(lastUpdated >> 56)
	data[25] = byte(lastUpdated >> 48)
	data[26] = byte(lastUpdated >> 40)
	data[27] = byte(lastUpdated >> 32)
	data[28] = byte(lastUpdated >> 24)
	data[29] = byte(lastUpdated >> 16)
	data[30] = byte(lastUpdated >> 8)
	data[31] = byte(lastUpdated)

	return data, nil
}

// deserializeHeightStats 反序列化高度统计信息
func deserializeHeightStats(data []byte) (*HeightIndexStats, error) {
	if len(data) < 32 {
		return nil, fmt.Errorf("数据长度不足")
	}

	stats := &HeightIndexStats{}

	// TotalEntries
	stats.TotalEntries = uint64(data[0])<<56 | uint64(data[1])<<48 |
		uint64(data[2])<<40 | uint64(data[3])<<32 |
		uint64(data[4])<<24 | uint64(data[5])<<16 |
		uint64(data[6])<<8 | uint64(data[7])

	// MinHeight
	stats.MinHeight = uint64(data[8])<<56 | uint64(data[9])<<48 |
		uint64(data[10])<<40 | uint64(data[11])<<32 |
		uint64(data[12])<<24 | uint64(data[13])<<16 |
		uint64(data[14])<<8 | uint64(data[15])

	// MaxHeight
	stats.MaxHeight = uint64(data[16])<<56 | uint64(data[17])<<48 |
		uint64(data[18])<<40 | uint64(data[19])<<32 |
		uint64(data[20])<<24 | uint64(data[21])<<16 |
		uint64(data[22])<<8 | uint64(data[23])

	// LastUpdatedAt
	lastUpdated := uint64(data[24])<<56 | uint64(data[25])<<48 |
		uint64(data[26])<<40 | uint64(data[27])<<32 |
		uint64(data[28])<<24 | uint64(data[29])<<16 |
		uint64(data[30])<<8 | uint64(data[31])
	stats.LastUpdatedAt = int64(lastUpdated)

	return stats, nil
}
