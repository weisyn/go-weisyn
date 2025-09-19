package badger

import (
	"github.com/weisyn/v1/pkg/utils"
)

// BadgerDB存储默认配置值
// 这些默认值基于BadgerDB的最佳实践和区块链存储需求

// getDefaultPath 获取默认数据库路径（使用路径解析工具）
// 原因：统一的数据目录便于管理和备份，确保路径解析正确
func getDefaultPath() string {
	return utils.ResolveDataPath("./data/badger")
}

const (
	// === 基础配置 ===

	// defaultSyncWrites 默认启用同步写入
	// 原因：区块链数据需要强一致性，同步写入确保数据安全性
	// 虽然性能略有损失，但数据完整性更重要
	defaultSyncWrites = true

	// === 性能配置 ===

	// defaultMemTableSize 默认内存表大小为64MB
	// 原因：64MB提供良好的读写性能，适合区块链的数据访问模式
	// 平衡内存使用和I/O性能
	defaultMemTableSize = 64 << 20 // 64MB

	// === 维护配置 ===

	// defaultEnableAutoCompaction 默认启用自动压缩
	// 原因：自动压缩减少磁盘占用，提高查询性能
	// 对于区块链这种写多读少的场景很重要
	defaultEnableAutoCompaction = true
)
