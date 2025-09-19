// Package utxo UTXO索引管理实现
//
// 🗂️ **UTXO索引管理器 (UTXO Index Manager)**
//
// 本文件实现UTXO的高效索引管理：
// - 地址索引：支持按地址快速查询UTXO列表
// - 类别索引：支持按UTXO类型进行分类查询
// - 状态索引：支持按UTXO状态进行过滤查询
// - 批量操作：支持区块处理时的批量索引更新
//
// 🎯 **核心功能**
// - 高效索引：基于BadgerDB的前缀索引机制
// - 批量优化：支持批量索引创建和更新操作
// - 一致性维护：确保索引与UTXO数据的一致性
// - 查询加速：显著提升地址和类别查询性能
//
// 🏗️ **设计原则**
// - 索引分离：索引操作与UTXO数据操作解耦
// - 性能优先：优化批量操作和查询性能
// - 数据一致：严格保证索引与数据的一致性
// - 简约设计：遵循WES极简设计原则
package utxo

import (
	"context"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"

	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ============================================================================
//                              索引管理器定义
// ============================================================================

// IndexManager UTXO索引管理器
//
// 🎯 **索引管理核心**
//
// 负责管理UTXO的各种索引，包括地址索引、类别索引等。
// 为UTXO查询操作提供高效的索引支撑，显著提升查询性能。
//
// 架构特点：
// - 统一管理：集中管理所有类型的UTXO索引
// - 批量优化：支持区块处理时的批量索引操作
// - 一致性保障：确保索引与UTXO数据的强一致性
// - 性能导向：基于BadgerDB优化的索引实现
type IndexManager struct {
	// 核心依赖
	logger      log.Logger          // 日志服务
	badgerStore storage.BadgerStore // 持久化存储
}

// ============================================================================
//                              构造函数
// ============================================================================

// NewIndexManager 创建UTXO索引管理器实例
//
// 🏗️ **构造器模式**
//
// 参数：
//   - logger: 日志服务
//   - badgerStore: 持久化存储
//
// 返回：
//   - *IndexManager: 索引管理器实例
//   - error: 创建错误
func NewIndexManager(logger log.Logger, badgerStore storage.BadgerStore) (*IndexManager, error) {
	if badgerStore == nil {
		return nil, fmt.Errorf("badger store 不能为空")
	}

	manager := &IndexManager{
		logger:      logger,
		badgerStore: badgerStore,
	}

	if logger != nil {
		logger.Debug("UTXO索引管理器初始化完成")
	}

	return manager, nil
}

// ============================================================================
//                           🔧 地址索引管理
// ============================================================================

// CreateAddressIndex 创建地址索引
//
// 🎯 **地址索引核心功能**：
// 为指定地址的UTXO创建索引条目，支持高效的按地址查询操作。
// 索引键格式: utxo:addr:{address}:{txHash}:{outputIndex} -> 1
//
// 参数：
//   - ctx: 上下文
//   - tx: 数据库事务（确保与UTXO创建的原子性）
//   - address: 所有者地址
//   - txHash: 交易哈希
//   - outputIndex: 输出索引
//
// 返回：
//   - error: 创建错误
func (im *IndexManager) CreateAddressIndex(ctx context.Context, tx storage.BadgerTransaction, address []byte, txHash []byte, outputIndex uint32) error {
	if im.logger != nil {
		im.logger.Debugf("创建地址索引 - address: %x, txHash: %x, index: %d", address, txHash, outputIndex)
	}

	// 1. 验证参数
	if len(address) != 20 {
		return fmt.Errorf("地址长度错误，期望20字节，实际%d字节", len(address))
	}
	if len(txHash) != 32 {
		return fmt.Errorf("交易哈希长度错误，期望32字节，实际%d字节", len(txHash))
	}

	// 2. 构建地址索引键
	indexKey := formatAddressIndexKey(address, txHash, outputIndex)

	// 3. 写入索引数据（值为空，我们只需要键存在）
	if err := tx.Set(indexKey, []byte{1}); err != nil {
		return fmt.Errorf("创建地址索引失败: %w", err)
	}

	return nil
}

// DeleteAddressIndex 删除地址索引
//
// 🎯 **地址索引清理功能**：
// 当UTXO被消费时，删除对应的地址索引条目。
//
// 参数：
//   - ctx: 上下文
//   - tx: 数据库事务
//   - address: 所有者地址
//   - txHash: 交易哈希
//   - outputIndex: 输出索引
//
// 返回：
//   - error: 删除错误
func (im *IndexManager) DeleteAddressIndex(ctx context.Context, tx storage.BadgerTransaction, address []byte, txHash []byte, outputIndex uint32) error {
	if im.logger != nil {
		im.logger.Debugf("删除地址索引 - address: %x, txHash: %x, index: %d", address, txHash, outputIndex)
	}

	// 构建地址索引键
	indexKey := formatAddressIndexKey(address, txHash, outputIndex)

	// 删除索引条目
	if err := tx.Delete(indexKey); err != nil {
		return fmt.Errorf("删除地址索引失败: %w", err)
	}

	return nil
}

// ============================================================================
//                           🏷️ 类别索引管理
// ============================================================================

// CreateCategoryIndex 创建类别索引
//
// 🎯 **类别索引核心功能**：
// 为指定类型的UTXO创建类别索引，支持按UTXO类型的高效查询。
// 索引键格式: utxo:cat:{category}:{txHash}:{outputIndex} -> 1
//
// 参数：
//   - ctx: 上下文
//   - tx: 数据库事务
//   - category: UTXO类别
//   - txHash: 交易哈希
//   - outputIndex: 输出索引
//
// 返回：
//   - error: 创建错误
func (im *IndexManager) CreateCategoryIndex(ctx context.Context, tx storage.BadgerTransaction, category utxo.UTXOCategory, txHash []byte, outputIndex uint32) error {
	if im.logger != nil {
		im.logger.Debugf("创建类别索引 - category: %s, txHash: %x, index: %d", category.String(), txHash, outputIndex)
	}

	// 1. 验证参数
	if len(txHash) != 32 {
		return fmt.Errorf("交易哈希长度错误，期望32字节，实际%d字节", len(txHash))
	}

	// 2. 构建类别索引键
	indexKey := im.formatCategoryIndexKey(category, txHash, outputIndex)

	// 3. 写入索引数据
	if err := tx.Set(indexKey, []byte{1}); err != nil {
		return fmt.Errorf("创建类别索引失败: %w", err)
	}

	return nil
}

// DeleteCategoryIndex 删除类别索引
//
// 🎯 **类别索引清理功能**：
// 当UTXO被消费时，删除对应的类别索引条目。
//
// 参数：
//   - ctx: 上下文
//   - tx: 数据库事务
//   - category: UTXO类别
//   - txHash: 交易哈希
//   - outputIndex: 输出索引
//
// 返回：
//   - error: 删除错误
func (im *IndexManager) DeleteCategoryIndex(ctx context.Context, tx storage.BadgerTransaction, category utxo.UTXOCategory, txHash []byte, outputIndex uint32) error {
	if im.logger != nil {
		im.logger.Debugf("删除类别索引 - category: %s, txHash: %x, index: %d", category.String(), txHash, outputIndex)
	}

	// 构建类别索引键
	indexKey := im.formatCategoryIndexKey(category, txHash, outputIndex)

	// 删除索引条目
	if err := tx.Delete(indexKey); err != nil {
		return fmt.Errorf("删除类别索引失败: %w", err)
	}

	return nil
}

// ============================================================================
//                           📊 状态索引管理
// ============================================================================

// CreateStatusIndex 创建状态索引
//
// 🎯 **状态索引核心功能**：
// 为指定状态的UTXO创建状态索引，支持按UTXO状态的快速过滤查询。
// 索引键格式: utxo:status:{status}:{txHash}:{outputIndex} -> 1
//
// 参数：
//   - ctx: 上下文
//   - tx: 数据库事务
//   - status: UTXO状态
//   - txHash: 交易哈希
//   - outputIndex: 输出索引
//
// 返回：
//   - error: 创建错误
func (im *IndexManager) CreateStatusIndex(ctx context.Context, tx storage.BadgerTransaction, status utxo.UTXOLifecycleStatus, txHash []byte, outputIndex uint32) error {
	if im.logger != nil {
		im.logger.Debugf("创建状态索引 - status: %s, txHash: %x, index: %d", status.String(), txHash, outputIndex)
	}

	// 构建状态索引键
	indexKey := im.formatStatusIndexKey(status, txHash, outputIndex)

	// 写入索引数据
	if err := tx.Set(indexKey, []byte{1}); err != nil {
		return fmt.Errorf("创建状态索引失败: %w", err)
	}

	return nil
}

// UpdateStatusIndex 更新状态索引
//
// 🎯 **状态索引更新功能**：
// 当UTXO状态发生变化时，更新对应的状态索引。
// 删除旧状态索引，创建新状态索引。
//
// 参数：
//   - ctx: 上下文
//   - tx: 数据库事务
//   - oldStatus: 原状态
//   - newStatus: 新状态
//   - txHash: 交易哈希
//   - outputIndex: 输出索引
//
// 返回：
//   - error: 更新错误
func (im *IndexManager) UpdateStatusIndex(ctx context.Context, tx storage.BadgerTransaction, oldStatus, newStatus utxo.UTXOLifecycleStatus, txHash []byte, outputIndex uint32) error {
	if im.logger != nil {
		im.logger.Debugf("更新状态索引 - oldStatus: %s, newStatus: %s, txHash: %x, index: %d",
			oldStatus.String(), newStatus.String(), txHash, outputIndex)
	}

	// 如果状态没有变化，跳过更新
	if oldStatus == newStatus {
		return nil
	}

	// 1. 删除旧状态索引
	oldIndexKey := im.formatStatusIndexKey(oldStatus, txHash, outputIndex)
	if err := tx.Delete(oldIndexKey); err != nil {
		return fmt.Errorf("删除旧状态索引失败: %w", err)
	}

	// 2. 创建新状态索引
	newIndexKey := im.formatStatusIndexKey(newStatus, txHash, outputIndex)
	if err := tx.Set(newIndexKey, []byte{1}); err != nil {
		return fmt.Errorf("创建新状态索引失败: %w", err)
	}

	return nil
}

// ============================================================================
//                           🔧 索引键格式化辅助方法
// ============================================================================

// formatCategoryIndexKey 格式化类别索引键
// 格式: utxo:cat:{category}:{txHash}:{outputIndex}
func (im *IndexManager) formatCategoryIndexKey(category utxo.UTXOCategory, txHash []byte, outputIndex uint32) []byte {
	categoryStr := category.String()
	keySize := len(UTXOCategoryPrefix) + len(categoryStr) + 1 + len(txHash) + 4

	key := make([]byte, keySize)
	offset := 0

	// 添加前缀
	copy(key[offset:], UTXOCategoryPrefix)
	offset += len(UTXOCategoryPrefix)

	// 添加类别字符串
	copy(key[offset:], categoryStr)
	offset += len(categoryStr)

	// 添加分隔符
	key[offset] = ':'
	offset++

	// 添加交易哈希
	copy(key[offset:], txHash)
	offset += len(txHash)

	// 添加输出索引（大端序）
	key[offset] = byte(outputIndex >> 24)
	key[offset+1] = byte(outputIndex >> 16)
	key[offset+2] = byte(outputIndex >> 8)
	key[offset+3] = byte(outputIndex)

	return key
}

// formatStatusIndexKey 格式化状态索引键
// 格式: utxo:status:{status}:{txHash}:{outputIndex}
func (im *IndexManager) formatStatusIndexKey(status utxo.UTXOLifecycleStatus, txHash []byte, outputIndex uint32) []byte {
	statusStr := status.String()
	keySize := len(UTXOStatusPrefix) + len(statusStr) + 1 + len(txHash) + 4

	key := make([]byte, keySize)
	offset := 0

	// 添加前缀
	copy(key[offset:], UTXOStatusPrefix)
	offset += len(UTXOStatusPrefix)

	// 添加状态字符串
	copy(key[offset:], statusStr)
	offset += len(statusStr)

	// 添加分隔符
	key[offset] = ':'
	offset++

	// 添加交易哈希
	copy(key[offset:], txHash)
	offset += len(txHash)

	// 添加输出索引（大端序）
	key[offset] = byte(outputIndex >> 24)
	key[offset+1] = byte(outputIndex >> 16)
	key[offset+2] = byte(outputIndex >> 8)
	key[offset+3] = byte(outputIndex)

	return key
}

// ============================================================================
//                           📈 索引统计和维护
// ============================================================================

// GetIndexStats 获取索引统计信息
//
// 🎯 **索引状态监控**：
// 获取各类型索引的统计信息，用于监控和性能调优。
//
// 参数：
//   - ctx: 上下文
//
// 返回：
//   - *IndexStats: 索引统计信息
//   - error: 查询错误
func (im *IndexManager) GetIndexStats(ctx context.Context) (*IndexStats, error) {
	if im.logger != nil {
		im.logger.Debug("获取UTXO索引统计信息")
	}

	stats := &IndexStats{}

	// 统计地址索引数量
	addressIndexMap, err := im.badgerStore.PrefixScan(ctx, []byte(UTXOAddressPrefix))
	if err != nil {
		return nil, fmt.Errorf("扫描地址索引失败: %w", err)
	}
	stats.AddressIndexCount = len(addressIndexMap)

	// 统计类别索引数量
	categoryIndexMap, err := im.badgerStore.PrefixScan(ctx, []byte(UTXOCategoryPrefix))
	if err != nil {
		return nil, fmt.Errorf("扫描类别索引失败: %w", err)
	}
	stats.CategoryIndexCount = len(categoryIndexMap)

	if im.logger != nil {
		im.logger.Debugf("索引统计 - 地址索引: %d, 类别索引: %d", stats.AddressIndexCount, stats.CategoryIndexCount)
	}

	return stats, nil
}

// ============================================================================
//                           📋 索引统计数据结构
// ============================================================================

// IndexStats UTXO索引统计信息
//
// 🎯 **索引监控数据**：
// 提供各类型索引的统计信息，用于性能监控和优化决策。
type IndexStats struct {
	AddressIndexCount  int `json:"address_index_count"`  // 地址索引数量
	CategoryIndexCount int `json:"category_index_count"` // 类别索引数量
}

// ============================================================================
//                           🔧 索引键前缀常量
// ============================================================================

// 状态索引键前缀定义
const (
	UTXOStatusPrefix = "utxo:status:" // 状态索引键前缀: utxo:status:{status}:{txHash}:{outputIndex}
)
