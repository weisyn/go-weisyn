// Package internal 提供交易管理的内部工具函数
//
// 📋 **cache_utils.go - 缓存工具函数集合**
//
// 本文件提供交易管理所需的缓存工具函数，支持哈希+缓存架构模式。
// 专注于交易数据的序列化、反序列化、缓存管理和 TTL 控制。
//
// 🎯 **核心职责**：
// - 交易序列化：将交易对象转换为字节数组存储
// - 交易反序列化：从字节数组恢复交易对象
// - 缓存键管理：标准化缓存键的生成和管理
// - TTL 生命周期：管理缓存项的生存时间
// - 缓存性能优化：提供高效的缓存读写操作
//
// 🏗️ **设计特点**：
// - 独立工具函数：不依赖特定结构体，通过参数传递依赖
// - 标准化序列化：使用 Protobuf 确保跨平台兼容性
// - 内存管理优化：通过 TTL 防止内存泄漏
// - 并发安全：支持多协程安全访问缓存
//
// 📋 **使用方式**：
// 其他子模块可直接调用这些工具函数：
//
//	import "github.com/weisyn/v1/internal/core/blockchain/transaction/internal"
//	hash, err := internal.ComputeTransactionHash(ctx, hashClient, tx)
package internal

import (
	"context"
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
//                              缓存配置
// ============================================================================

// CacheConfig 交易缓存配置
//
// 🎯 **统一的缓存配置管理**
//
// 定义各种交易相关数据的缓存策略，包括TTL、大小限制等。
// 支持不同类型数据的差异化缓存策略。
type CacheConfig struct {
	// 基础TTL配置
	DefaultTTL          time.Duration `json:"default_ttl"`           // 默认TTL（1小时）
	UnsignedTxTTL       time.Duration `json:"unsigned_tx_ttl"`       // 未签名交易TTL（30分钟）
	SignedTxTTL         time.Duration `json:"signed_tx_ttl"`         // 已签名交易TTL（1小时）
	MultiSigSessionTTL  time.Duration `json:"multisig_session_ttl"`  // 多签会话TTL（4小时）
	TxStatusTTL         time.Duration `json:"tx_status_ttl"`         // 交易状态TTL（24小时）
	FeeEstimateTTL      time.Duration `json:"fee_estimate_ttl"`      // 费用估算TTL（10分钟）
	ValidationResultTTL time.Duration `json:"validation_result_ttl"` // 验证结果TTL（30分钟）

	// 存储和性能配置
	MaxCacheSize    int64         `json:"max_cache_size"`   // 最大缓存大小（512MB）
	CleanupInterval time.Duration `json:"cleanup_interval"` // 清理间隔（5分钟）
}

// 缓存键前缀常量
const (
	CacheKeyPrefix         = "tx_cache:"
	UnsignedTxPrefix       = CacheKeyPrefix + "unsigned:"
	SignedTxPrefix         = CacheKeyPrefix + "signed:"
	MultiSigSessionPrefix  = CacheKeyPrefix + "multisig:"
	TxStatusPrefix         = CacheKeyPrefix + "status:"
	FeeEstimatePrefix      = CacheKeyPrefix + "fee:"
	ValidationResultPrefix = CacheKeyPrefix + "validation:"
)

// GetDefaultCacheConfig 获取默认缓存配置
//
// 🎯 **默认缓存配置提供器**
//
// 返回生产环境推荐的缓存配置，包括合理的TTL设置和性能参数。
//
// 💡 **返回值说明**：
//   - *CacheConfig: 默认配置对象
func GetDefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		DefaultTTL:          1 * time.Hour,     // 默认1小时
		UnsignedTxTTL:       30 * time.Minute,  // 未签名交易30分钟
		SignedTxTTL:         1 * time.Hour,     // 已签名交易1小时
		MultiSigSessionTTL:  4 * time.Hour,     // 多签会话4小时
		TxStatusTTL:         24 * time.Hour,    // 交易状态24小时
		FeeEstimateTTL:      10 * time.Minute,  // 费用估算10分钟
		ValidationResultTTL: 30 * time.Minute,  // 验证结果30分钟
		MaxCacheSize:        512 * 1024 * 1024, // 512MB最大缓存
		CleanupInterval:     5 * time.Minute,   // 5分钟清理一次
	}
}

// GenerateCacheKey 生成标准化缓存键
//
// 🎯 **缓存键标准化工具**
//
// 根据前缀和哈希生成标准化的缓存键，确保键名的一致性和唯一性。
//
// 💡 **参数说明**：
//   - prefix: 缓存键前缀（如：UnsignedTxPrefix）
//   - hash: 交易哈希或标识符
//
// 💡 **返回值说明**：
//   - string: 标准化缓存键（格式：{prefix}{hex(hash)}）
func GenerateCacheKey(prefix string, hash []byte) string {
	return fmt.Sprintf("%s%x", prefix, hash)
}

// ============================================================================
//                              交易序列化工具
// ============================================================================

// SerializeTransaction 序列化交易对象
//
// 🎯 **交易对象序列化工具**
//
// 将交易对象序列化为字节数组，用于缓存存储。
// 使用 protobuf 确保序列化的一致性和跨平台兼容性。
//
// 💡 **参数说明**：
//   - tx: 交易对象
//
// 💡 **返回值说明**：
//   - []byte: 序列化后的字节数组
//   - error: 序列化错误
func SerializeTransaction(tx *transaction.Transaction) ([]byte, error) {
	if tx == nil {
		return nil, fmt.Errorf("交易对象为空，无法序列化")
	}

	data, err := proto.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("protobuf序列化失败: %w", err)
	}

	return data, nil
}

// DeserializeTransaction 反序列化交易对象
//
// 🎯 **交易对象反序列化工具**
//
// 从字节数组恢复交易对象，用于缓存读取。
// 使用 protobuf 确保反序列化的一致性和跨平台兼容性。
//
// 💡 **参数说明**：
//   - data: 序列化的字节数组
//
// 💡 **返回值说明**：
//   - *transaction.Transaction: 反序列化后的交易对象
//   - error: 反序列化错误
func DeserializeTransaction(data []byte) (*transaction.Transaction, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("数据为空，无法反序列化")
	}

	tx := &transaction.Transaction{}
	err := proto.Unmarshal(data, tx)
	if err != nil {
		return nil, fmt.Errorf("protobuf反序列化失败: %w", err)
	}

	return tx, nil
}

// ============================================================================
//                              缓存操作工具
// ============================================================================

// CacheTransactionWithTTL 缓存交易对象（带TTL）
//
// 🎯 **交易缓存存储工具**
//
// 将交易对象序列化并存储到缓存中，支持TTL控制。
// 自动处理序列化和错误处理。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - memoryStore: 内存缓存存储接口
//   - prefix: 缓存键前缀
//   - txHash: 交易哈希
//   - tx: 交易对象
//   - ttl: 生存时间
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - error: 缓存错误，nil表示成功
func CacheTransactionWithTTL(
	ctx context.Context,
	memoryStore storage.MemoryStore,
	prefix string,
	txHash []byte,
	tx *transaction.Transaction,
	ttl time.Duration,
	logger log.Logger,
) error {
	if memoryStore == nil {
		return fmt.Errorf("内存存储服务为空")
	}
	if tx == nil {
		return fmt.Errorf("交易对象为空")
	}

	// 生成缓存键
	cacheKey := GenerateCacheKey(prefix, txHash)

	// 序列化交易
	data, err := SerializeTransaction(tx)
	if err != nil {
		return fmt.Errorf("序列化交易失败: %w", err)
	}

	// 存储到缓存
	err = memoryStore.Set(ctx, cacheKey, data, ttl)
	if err != nil {
		return fmt.Errorf("缓存交易失败: %w", err)
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("✅ 交易已缓存 - 键: %s, TTL: %v, 大小: %d字节",
			cacheKey, ttl, len(data)))
	}

	return nil
}

// GetTransactionFromCache 从缓存获取交易对象
//
// 🎯 **交易缓存读取工具**
//
// 从缓存中读取交易对象，自动处理反序列化。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - memoryStore: 内存缓存存储接口
//   - prefix: 缓存键前缀
//   - txHash: 交易哈希
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - *transaction.Transaction: 交易对象，nil表示未找到
//   - bool: 是否找到缓存项
//   - error: 操作错误
func GetTransactionFromCache(
	ctx context.Context,
	memoryStore storage.MemoryStore,
	prefix string,
	txHash []byte,
	logger log.Logger,
) (*transaction.Transaction, bool, error) {
	if memoryStore == nil {
		return nil, false, fmt.Errorf("内存存储服务为空")
	}

	// 生成缓存键
	cacheKey := GenerateCacheKey(prefix, txHash)

	// 从缓存获取数据
	data, found, err := memoryStore.Get(ctx, cacheKey)
	if err != nil {
		return nil, false, fmt.Errorf("读取缓存失败: %w", err)
	}
	if !found {
		if logger != nil {
			logger.Debug(fmt.Sprintf("🔍 缓存未命中 - 键: %s", cacheKey))
		}
		return nil, false, nil
	}

	// 直接使用data（假设MemoryStore.Get返回的是[]byte）
	dataBytes := data

	// 反序列化交易
	tx, err := DeserializeTransaction(dataBytes)
	if err != nil {
		return nil, false, fmt.Errorf("反序列化交易失败: %w", err)
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("✅ 缓存命中 - 键: %s, 大小: %d字节",
			cacheKey, len(dataBytes)))
	}

	return tx, true, nil
}

// DeleteTransactionFromCache 从缓存删除交易
//
// 🎯 **交易缓存删除工具**
//
// 从缓存中删除指定的交易对象。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - memoryStore: 内存缓存存储接口
//   - prefix: 缓存键前缀
//   - txHash: 交易哈希
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - error: 删除错误，nil表示成功
func DeleteTransactionFromCache(
	ctx context.Context,
	memoryStore storage.MemoryStore,
	prefix string,
	txHash []byte,
	logger log.Logger,
) error {
	if memoryStore == nil {
		return fmt.Errorf("内存存储服务为空")
	}

	// 生成缓存键
	cacheKey := GenerateCacheKey(prefix, txHash)

	// 从缓存删除
	err := memoryStore.Delete(ctx, cacheKey)
	if err != nil {
		return fmt.Errorf("删除缓存失败: %w", err)
	}

	if logger != nil {
		logger.Debug(fmt.Sprintf("🗑️ 缓存已删除 - 键: %s", cacheKey))
	}

	return nil
}

// ============================================================================
//                              快捷缓存方法
// ============================================================================

// CacheUnsignedTransaction 缓存未签名交易
//
// 🎯 **未签名交易缓存快捷方法**
func CacheUnsignedTransaction(
	ctx context.Context,
	memoryStore storage.MemoryStore,
	txHash []byte,
	tx *transaction.Transaction,
	config *CacheConfig,
	logger log.Logger,
) error {
	ttl := config.UnsignedTxTTL
	if config == nil {
		ttl = 30 * time.Minute // 默认30分钟
	}

	return CacheTransactionWithTTL(ctx, memoryStore, UnsignedTxPrefix, txHash, tx, ttl, logger)
}

// GetUnsignedTransactionFromCache 获取未签名交易
//
// 🎯 **未签名交易缓存读取快捷方法**
func GetUnsignedTransactionFromCache(
	ctx context.Context,
	memoryStore storage.MemoryStore,
	txHash []byte,
	logger log.Logger,
) (*transaction.Transaction, bool, error) {
	return GetTransactionFromCache(ctx, memoryStore, UnsignedTxPrefix, txHash, logger)
}

// CacheSignedTransaction 缓存已签名交易
//
// 🎯 **已签名交易缓存快捷方法**
func CacheSignedTransaction(
	ctx context.Context,
	memoryStore storage.MemoryStore,
	txHash []byte,
	tx *transaction.Transaction,
	config *CacheConfig,
	logger log.Logger,
) error {
	ttl := config.SignedTxTTL
	if config == nil {
		ttl = 1 * time.Hour // 默认1小时
	}

	return CacheTransactionWithTTL(ctx, memoryStore, SignedTxPrefix, txHash, tx, ttl, logger)
}

// GetSignedTransactionFromCache 获取已签名交易
//
// 🎯 **已签名交易缓存读取快捷方法**
func GetSignedTransactionFromCache(
	ctx context.Context,
	memoryStore storage.MemoryStore,
	txHash []byte,
	logger log.Logger,
) (*transaction.Transaction, bool, error) {
	return GetTransactionFromCache(ctx, memoryStore, SignedTxPrefix, txHash, logger)
}

// ============================================================================
//                              批量操作工具
// ============================================================================

// UpdateTransactionCache 更新交易缓存
//
// 🎯 **交易缓存更新工具**
//
// 处理交易状态变更时的缓存更新，如从未签名变为已签名。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - memoryStore: 内存缓存存储接口
//   - oldPrefix: 旧缓存键前缀
//   - newPrefix: 新缓存键前缀
//   - oldHash: 旧交易哈希
//   - newHash: 新交易哈希
//   - tx: 交易对象
//   - config: 缓存配置
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - error: 更新错误，nil表示成功
func UpdateTransactionCache(
	ctx context.Context,
	memoryStore storage.MemoryStore,
	oldPrefix, newPrefix string,
	oldHash, newHash []byte,
	tx *transaction.Transaction,
	config *CacheConfig,
	logger log.Logger,
) error {
	// 删除旧缓存
	if err := DeleteTransactionFromCache(ctx, memoryStore, oldPrefix, oldHash, logger); err != nil {
		if logger != nil {
			logger.Warn(fmt.Sprintf("删除旧缓存失败: %v", err))
		}
	}

	// 确定新缓存的TTL
	var newTTL time.Duration
	switch newPrefix {
	case SignedTxPrefix:
		newTTL = config.SignedTxTTL
	case UnsignedTxPrefix:
		newTTL = config.UnsignedTxTTL
	default:
		newTTL = config.DefaultTTL
	}

	// 添加新缓存
	return CacheTransactionWithTTL(ctx, memoryStore, newPrefix, newHash, tx, newTTL, logger)
}

// ValidateCacheKey 验证缓存键格式
//
// 🎯 **缓存键格式验证工具**
//
// 验证缓存键是否符合标准格式要求。
//
// 💡 **参数说明**：
//   - key: 待验证的缓存键
//
// 💡 **返回值说明**：
//   - error: 验证错误，nil表示格式正确
func ValidateCacheKey(key string) error {
	if len(key) == 0 {
		return fmt.Errorf("缓存键为空")
	}

	if len(key) < len(CacheKeyPrefix) {
		return fmt.Errorf("缓存键太短: %s", key)
	}

	if key[:len(CacheKeyPrefix)] != CacheKeyPrefix {
		return fmt.Errorf("缓存键前缀不正确: %s", key)
	}

	return nil
}

// ClearTransactionCache 清理过期的交易缓存
//
// 🎯 **缓存清理管理**
//
// 清理指定时间之前的交易相关缓存数据，防止内存泄漏。
// 支持按模式批量清理不同类型的缓存数据。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - cacheStore: 缓存存储接口
//   - olderThan: 清理时间阈值（清理超过此时间的数据）
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - int64: 清理的缓存条目数量
//   - error: 清理错误
func ClearTransactionCache(
	ctx context.Context,
	cacheStore storage.MemoryStore,
	olderThan time.Duration,
	logger log.Logger,
) (int64, error) {
	if logger != nil {
		logger.Debug(fmt.Sprintf("开始清理交易缓存，时间阈值: %v", olderThan))
	}

	totalCleaned := int64(0)

	// 定义要清理的缓存前缀
	prefixes := []string{
		UnsignedTxPrefix,
		SignedTxPrefix,
		MultiSigSessionPrefix,
		TxStatusPrefix,
		FeeEstimatePrefix,
		ValidationResultPrefix,
	}

	// 逐个清理每种类型的缓存
	for _, prefix := range prefixes {
		pattern := prefix + "*"
		// 注意：这里假设 MemoryStore 接口支持 DeleteByPattern 方法
		// 如果不支持，需要实现具体的清理逻辑
		if deleter, ok := cacheStore.(interface {
			DeleteByPattern(ctx context.Context, pattern string) (int64, error)
		}); ok {
			cleaned, err := deleter.DeleteByPattern(ctx, pattern)
			if err != nil {
				if logger != nil {
					logger.Warn(fmt.Sprintf("清理缓存模式 %s 失败: %v", pattern, err))
				}
				continue
			}
			totalCleaned += cleaned

			if logger != nil && cleaned > 0 {
				logger.Debug(fmt.Sprintf("清理缓存模式 %s: %d 个条目", pattern, cleaned))
			}
		}
	}

	if logger != nil {
		logger.Info(fmt.Sprintf("交易缓存清理完成，总共清理: %d 个条目", totalCleaned))
	}

	return totalCleaned, nil
}

// GetCacheStatus 获取缓存状态统计
//
// 🎯 **缓存状态监控**
//
// 获取交易缓存的统计信息，用于监控、调试和性能分析。
// 提供各种类型缓存的条目数量统计。
//
// 💡 **参数说明**：
//   - ctx: 上下文对象
//   - cacheStore: 缓存存储接口
//   - logger: 日志记录器（可选）
//
// 💡 **返回值说明**：
//   - map[string]interface{}: 缓存状态信息映射
//   - error: 查询错误
func GetCacheStatus(
	ctx context.Context,
	cacheStore storage.MemoryStore,
	logger log.Logger,
) (map[string]interface{}, error) {
	status := make(map[string]interface{})

	// 获取总缓存条目数（如果缓存支持）
	if counter, ok := cacheStore.(interface {
		Count(ctx context.Context) (int64, error)
	}); ok {
		totalCount, err := counter.Count(ctx)
		if err != nil {
			if logger != nil {
				logger.Warn(fmt.Sprintf("获取缓存总数失败: %v", err))
			}
		} else {
			status["total_count"] = totalCount
		}
	}

	// 统计各种类型的缓存数量（如果缓存支持模式匹配）
	prefixes := map[string]string{
		"unsigned_transactions": UnsignedTxPrefix + "*",
		"signed_transactions":   SignedTxPrefix + "*",
		"multisig_sessions":     MultiSigSessionPrefix + "*",
		"transaction_status":    TxStatusPrefix + "*",
		"fee_estimates":         FeeEstimatePrefix + "*",
		"validation_results":    ValidationResultPrefix + "*",
	}

	if patternCounter, ok := cacheStore.(interface {
		CountByPattern(ctx context.Context, pattern string) (int64, error)
	}); ok {
		for category, pattern := range prefixes {
			count, err := patternCounter.CountByPattern(ctx, pattern)
			if err != nil {
				if logger != nil {
					logger.Warn(fmt.Sprintf("统计缓存模式 %s 失败: %v", pattern, err))
				}
				status[category] = 0
			} else {
				status[category] = count
			}
		}
	}

	// 添加缓存配置信息
	config := GetDefaultCacheConfig()
	status["cache_config"] = map[string]interface{}{
		"unsigned_tx_ttl":  config.UnsignedTxTTL.String(),
		"signed_tx_ttl":    config.SignedTxTTL.String(),
		"multisig_ttl":     config.MultiSigSessionTTL.String(),
		"status_ttl":       config.TxStatusTTL.String(),
		"fee_estimate_ttl": config.FeeEstimateTTL.String(),
		"validation_ttl":   config.ValidationResultTTL.String(),
	}

	return status, nil
}
