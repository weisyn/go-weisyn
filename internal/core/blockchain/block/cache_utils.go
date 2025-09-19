// Package block 提供区块管理的核心实现
//
// 📋 **cache_utils.go - 缓存工具实现**
//
// 本文件提供区块管理所需的缓存工具方法，支持哈希+缓存架构模式。
// 专注于区块数据的序列化、反序列化、缓存管理和 TTL 控制。
//
// 🎯 **核心职责**：
// - 区块序列化：将区块对象转换为字节数组存储
// - 区块反序列化：从字节数组恢复区块对象
// - 缓存键管理：标准化缓存键的生成和管理
// - TTL 生命周期：管理缓存项的生存时间
// - 缓存性能优化：提供高效的缓存读写操作
//
// 🏗️ **架构特点**：
// - 哈希+缓存模式：支持轻量级哈希和复杂对象缓存
// - 标准化序列化：使用 Protobuf 确保跨平台兼容性
// - 内存管理优化：通过 TTL 防止内存泄漏
// - 并发安全：支持多协程安全访问缓存
//
// 详细设计文档：internal/core/blockchain/block/README.md
package block

import (
	"context"
	"fmt"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"google.golang.org/protobuf/proto"
)

// ==================== 缓存键管理 ====================

// CacheKeyPrefix 定义缓存键前缀常量
const (
	// 候选区块缓存前缀
	CandidateBlockPrefix = "candidate_block:"
)

// CacheConfig 缓存配置结构
type CacheConfig struct {
	// 候选区块 TTL（矿工创建的候选区块）
	CandidateBlockTTL time.Duration

	// 最大缓存大小（字节）
	MaxCacheSize int64
}

// getDefaultCacheConfig 获取默认缓存配置
//
// 🎯 **缓存配置管理**
//
// 提供区块管理的默认缓存配置，专注于候选区块缓存。
//
// 配置策略：
// - 候选区块：较短 TTL，因为挖矿完成后不再需要
// - 内存限制：防止缓存占用过多内存
//
// 返回值：
//
//	*CacheConfig: 默认配置对象
func (m *Manager) getDefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		CandidateBlockTTL: 5 * time.Minute,   // 候选区块5分钟TTL
		MaxCacheSize:      512 * 1024 * 1024, // 512MB最大缓存
	}
}

// generateCacheKey 生成标准化缓存键
//
// 🎯 **缓存键标准化**
//
// 根据前缀和哈希生成标准化的缓存键，确保键的唯一性和可读性。
//
// 键格式：{prefix}{hash_hex}
// 示例：candidate_block:abc123def456...
//
// 参数：
//
//	prefix: 缓存键前缀
//	hash: 哈希字节数组
//
// 返回值：
//
//	string: 标准化的缓存键
func (m *Manager) generateCacheKey(prefix string, hash []byte) string {
	return fmt.Sprintf("%s%x", prefix, hash)
}

// ==================== 区块序列化工具 ====================

// serializeBlock 序列化区块为字节数组
//
// 🎯 **区块数据序列化**
//
// 将区块对象序列化为字节数组，用于缓存存储。
// 使用 Protobuf 序列化确保数据的紧凑性和跨平台兼容性。
//
// 序列化特点：
// - 使用标准 Protobuf 序列化
// - 保持数据完整性
// - 优化存储空间
// - 确保跨平台兼容性
//
// 参数：
//
//	block: 待序列化的区块对象
//
// 返回值：
//
//	[]byte: 序列化后的字节数组
//	error: 序列化过程中的错误，nil 表示成功
func (m *Manager) serializeBlock(block *core.Block) ([]byte, error) {
	if block == nil {
		return nil, fmt.Errorf("区块对象不能为空")
	}

	if block.Header == nil {
		return nil, fmt.Errorf("区块头不能为空")
	}

	if m.logger != nil {
		m.logger.Debugf("序列化区块，高度: %d", block.Header.Height)
	}

	// 使用 Protobuf 序列化
	data, err := proto.Marshal(block)
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("区块序列化失败: %v", err)
		}
		return nil, fmt.Errorf("区块序列化失败: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("序列化结果为空")
	}

	if m.logger != nil {
		m.logger.Debugf("区块序列化成功，数据大小: %d 字节", len(data))
	}

	return data, nil
}

// deserializeBlock 反序列化字节数组为区块
//
// 🎯 **区块数据反序列化**
//
// 将字节数组反序列化为区块对象，从缓存中恢复区块数据。
// 确保反序列化后的区块对象完整和有效。
//
// 反序列化特点：
// - 使用标准 Protobuf 反序列化
// - 验证数据完整性
// - 处理兼容性问题
// - 优化内存使用
//
// 参数：
//
//	data: 序列化的字节数组
//
// 返回值：
//
//	*core.Block: 反序列化后的区块对象
//	error: 反序列化过程中的错误，nil 表示成功
func (m *Manager) deserializeBlock(data []byte) (*core.Block, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("无法反序列化空数据")
	}

	if m.logger != nil {
		m.logger.Debugf("反序列化区块，数据大小: %d 字节", len(data))
	}

	// 创建区块对象
	block := &core.Block{}

	// 使用 Protobuf 反序列化
	err := proto.Unmarshal(data, block)
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("区块反序列化失败: %v", err)
		}
		return nil, fmt.Errorf("区块反序列化失败: %w", err)
	}

	// 验证基本字段
	if block.Header == nil {
		return nil, fmt.Errorf("反序列化的区块缺少区块头")
	}

	if block.Body == nil {
		return nil, fmt.Errorf("反序列化的区块缺少区块体")
	}

	if m.logger != nil {
		m.logger.Debugf("区块反序列化成功，高度: %d", block.Header.Height)
	}

	return block, nil
}

// ==================== 候选区块缓存操作 ====================

// storeCandidateBlock 存储候选区块到缓存并返回区块哈希
//
// 🎯 **候选区块缓存存储**
//
// 将创建的候选区块存储到内存缓存中，供后续挖矿使用。
// 在内部计算区块哈希，使用哈希作为缓存键，并返回哈希值。
//
// 存储策略：
// - 内部计算区块哈希
// - 使用区块哈希作为缓存键
// - 设置较短的 TTL（候选区块有时效性）
// - 序列化后存储以节省内存
// - 支持并发安全操作
//
// 参数：
//
//	ctx: 上下文对象
//	block: 候选区块对象
//
// 返回值：
//
//	[]byte: 计算出的区块哈希
//	error: 存储过程中的错误，nil 表示存储成功
func (m *Manager) storeCandidateBlock(ctx context.Context, block *core.Block) ([]byte, error) {
	if m.logger != nil {
		m.logger.Debugf("存储候选区块到缓存，高度: %d", block.Header.Height)
	}

	// 1. 计算区块哈希
	hashResponse, err := m.blockHashServiceClient.ComputeBlockHash(ctx, &core.ComputeBlockHashRequest{
		Block: block,
	})
	if err != nil {
		return nil, fmt.Errorf("计算区块哈希失败: %w", err)
	}

	blockHash := hashResponse.Hash
	if len(blockHash) != 32 {
		return nil, fmt.Errorf("区块哈希长度异常，期望32字节，实际: %d", len(blockHash))
	}

	// 2. 序列化区块
	blockData, err := m.serializeBlock(block)
	if err != nil {
		return nil, fmt.Errorf("序列化候选区块失败: %w", err)
	}

	// 3. 生成缓存键
	cacheKey := m.generateCacheKey(CandidateBlockPrefix, blockHash)

	// 4. 获取配置
	config := m.getDefaultCacheConfig()

	// 5. 存储到缓存（设置TTL）
	err = m.cacheStore.Set(ctx, cacheKey, blockData, config.CandidateBlockTTL)
	if err != nil {
		if m.logger != nil {
			m.logger.Errorf("存储候选区块到缓存失败: %v", err)
		}
		return nil, fmt.Errorf("存储候选区块到缓存失败: %w", err)
	}

	if m.logger != nil {
		m.logger.Infof("候选区块缓存存储成功，哈希: %x, 高度: %d, 缓存键: %s, TTL: %v",
			blockHash, block.Header.Height, cacheKey, config.CandidateBlockTTL)
	}

	return blockHash, nil
}

// ==================== 序列化工具方法 ====================

// ==================== 文件结束 ====================
//
// 本文件专注于区块缓存的核心功能：
// 1. 区块序列化/反序列化
// 2. 候选区块存储（含哈希计算）
// 3. 缓存键管理和配置
//
// 遵循单一职责原则，只保留blockchain组件必需的功能
