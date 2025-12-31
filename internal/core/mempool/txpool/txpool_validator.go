// 文件说明：
// 本文件定义交易池基础安全验证器（BasicTxValidator）接口与生产实现，
// 负责交易的格式/哈希/大小/重复/内存上限等基础校验，
// 明确不包含签名、余额、UTXO 等业务验证，确保 TxPool 仅承担存储与基础安全职责。
package txpool

import (
	"context"
	"fmt"
	"sync"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"google.golang.org/protobuf/proto"
)

// =========================================================================
// 🛡️ 基础验证器接口定义
// =========================================================================

// BasicTxValidator 基础安全验证器接口。
// 说明：专注网络安全面，避免与业务域耦合。
// 方法要点：
// - ValidateFormat：格式完整性；
// - ValidateHash：与哈希服务结果一致性；
// - ValidateSize：基于配置的大小限制；
// - ValidateDuplicate：重复检测；
// - ValidateMemoryLimit：按字节校验内存上限；
// - UpdateMemoryUsage/GetValidationStats/Reset：运行状态管理。
type BasicTxValidator interface {
	ValidateFormat(tx *transaction.Transaction) error
	ValidateHash(tx *transaction.Transaction, expectedHash []byte) error
	ValidateSize(tx *transaction.Transaction) error
	ValidateDuplicate(txHash []byte) error
	ValidateMemoryLimit(currentUsage, txSize uint64) error

	UpdateMemoryUsage(delta int64) error
	GetValidationStats() ValidationStats
	Reset() error
}

// ValidationStats 验证统计信息。
// 字段：各类验证计数、拒绝计数、平均耗时与最近一次时间戳。
type ValidationStats struct {
	FormatValidations int64
	HashValidations   int64
	SizeValidations   int64
	DuplicateChecks   int64
	MemoryLimitChecks int64

	FormatRejections      int64
	HashRejections        int64
	SizeRejections        int64
	DuplicateRejections   int64
	MemoryLimitRejections int64

	AverageValidationTime time.Duration
	LastValidationTime    time.Time
}

// =========================================================================
// 🏭 生产级基础验证器实现
// =========================================================================

// ProductionBasicValidator 生产级基础验证器。
// 参数说明见构造函数；内部包含并发安全与统计信息。
type ProductionBasicValidator struct {
	maxTxSize      uint64
	maxMemoryUsage uint64

	hashManager crypto.HashManager
	hashService transaction.TransactionHashServiceClient
	logger      log.Logger

	currentMemoryUsage uint64
	duplicateCache     map[string]time.Time
	stats              ValidationStats

	mu sync.RWMutex
}

// NewProductionBasicValidator 创建生产级基础验证器。
// 参数：
// - maxTxSize：最大交易大小（字节）；
// - maxMemoryUsage：交易池允许的最大内存使用（字节）；
// - hashManager：可选哈希管理器（可为 nil）；
// - hashService：统一哈希服务客户端；
// - logger：日志接口。
// 返回：*ProductionBasicValidator。
func NewProductionBasicValidator(
	maxTxSize uint64,
	maxMemoryUsage uint64,
	hashManager crypto.HashManager,
	hashService transaction.TransactionHashServiceClient,
	logger log.Logger,
) *ProductionBasicValidator {
	return &ProductionBasicValidator{
		maxTxSize:          maxTxSize,
		maxMemoryUsage:     maxMemoryUsage,
		hashManager:        hashManager,
		hashService:        hashService,
		logger:             logger,
		currentMemoryUsage: 0,
		duplicateCache:     make(map[string]time.Time),
		stats:              ValidationStats{},
	}
}

// =========================================================================
// 🔍 基础安全验证实现
// =========================================================================

// ValidateFormat 验证交易格式完整性。
//
// 🎯 **验证范围**：
// - ✅ 交易非空
// - ✅ 版本有效
// - ✅ 至少有一个输出
// - ✅ 可序列化
//
// 🔧 **特殊交易处理**：
// 以下交易类型允许 `Inputs = []`（不消费UTXO）：
// - Coinbase交易：矿工奖励，凭空产生资产
// - Genesis交易：创世分配，初始资产分配
// - 免费资源部署：凭空创建资源UTXO
//
// 参数：tx 交易；返回：error 非空表示失败。
func (v *ProductionBasicValidator) ValidateFormat(tx *transaction.Transaction) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	start := time.Now()
	defer func() {
		v.stats.FormatValidations++
		v.stats.LastValidationTime = time.Now()
		v.updateAverageValidationTime(time.Since(start))
	}()
	if tx == nil {
		v.stats.FormatRejections++
		return fmt.Errorf("交易不能为空")
	}
	// Version可以为0（protobuf默认值），不做强制校验
	// ✅ 移除"输入不能为空"的检查
	// 原因：Coinbase/Genesis/免费资源部署等交易允许 Inputs = []
	if len(tx.Outputs) == 0 {
		v.stats.FormatRejections++
		return fmt.Errorf("交易输出不能为空")
	}
	if _, err := proto.Marshal(tx); err != nil {
		v.stats.FormatRejections++
		return fmt.Errorf("交易序列化失败: %w", err)
	}
	if v.logger != nil {
		if len(tx.Inputs) == 0 {
			v.logger.Debug("交易格式验证通过（无输入交易：Coinbase/Genesis/免费资源部署）")
		} else {
			v.logger.Debug("交易格式验证通过")
		}
	}
	return nil
}

// ValidateHash 验证交易哈希正确性（与哈希服务结果一致）。
// 参数：tx 交易；expectedHash 期望哈希；返回：error。
func (v *ProductionBasicValidator) ValidateHash(tx *transaction.Transaction, expectedHash []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	start := time.Now()
	defer func() { v.stats.HashValidations++; v.updateAverageValidationTime(time.Since(start)) }()
	if len(expectedHash) == 0 {
		v.stats.HashRejections++
		return fmt.Errorf("期望哈希不能为空")
	}
	req := &transaction.ComputeHashRequest{Transaction: tx, IncludeDebugInfo: false}
	resp, err := v.hashService.ComputeHash(context.Background(), req)
	if err != nil {
		v.stats.HashRejections++
		return fmt.Errorf("计算交易哈希失败: %w", err)
	}
	if !resp.IsValid {
		v.stats.HashRejections++
		return fmt.Errorf("交易哈希计算无效")
	}
	if len(resp.Hash) != len(expectedHash) {
		v.stats.HashRejections++
		return fmt.Errorf("哈希长度不匹配: 计算=%d, 期望=%d", len(resp.Hash), len(expectedHash))
	}
	for i := range resp.Hash {
		if resp.Hash[i] != expectedHash[i] {
			v.stats.HashRejections++
			return fmt.Errorf("哈希值不匹配")
		}
	}
	v.logger.Debug("交易哈希验证通过")
	return nil
}

// ValidateSize 验证交易大小限制。
// 参数：tx 交易；返回：error 超限时错误。
func (v *ProductionBasicValidator) ValidateSize(tx *transaction.Transaction) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	start := time.Now()
	defer func() { v.stats.SizeValidations++; v.updateAverageValidationTime(time.Since(start)) }()
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		v.stats.SizeRejections++
		return fmt.Errorf("无法计算交易大小: %w", err)
	}
	txSize := uint64(len(txBytes))
	if txSize > v.maxTxSize {
		v.stats.SizeRejections++
		return fmt.Errorf("交易大小超限: %d > %d 字节", txSize, v.maxTxSize)
	}
	v.logger.Debug("交易大小验证通过")
	return nil
}

// ValidateDuplicate 检测重复交易。
// 参数：txHash 交易哈希；返回：error 重复时错误。
func (v *ProductionBasicValidator) ValidateDuplicate(txHash []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	start := time.Now()
	defer func() { v.stats.DuplicateChecks++; v.updateAverageValidationTime(time.Since(start)) }()
	hashStr := fmt.Sprintf("%x", txHash)
	if lastSeen, exists := v.duplicateCache[hashStr]; exists {
		v.stats.DuplicateRejections++
		return fmt.Errorf("重复交易检测: 上次见于 %v", lastSeen)
	}
	v.duplicateCache[hashStr] = time.Now()
	v.cleanupExpiredDuplicates()
	v.logger.Debug("重复交易检查通过")
	return nil
}

// ValidateMemoryLimit 验证内存使用限制。
// 参数：currentUsage 当前使用（字节）；txSize 本次新增（字节）。返回：error 超限时报错。
func (v *ProductionBasicValidator) ValidateMemoryLimit(currentUsage, txSize uint64) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	start := time.Now()
	defer func() { v.stats.MemoryLimitChecks++; v.updateAverageValidationTime(time.Since(start)) }()
	projectedUsage := currentUsage + txSize
	if projectedUsage > v.maxMemoryUsage {
		v.stats.MemoryLimitRejections++
		return fmt.Errorf("内存使用将超限: %d + %d > %d 字节", currentUsage, txSize, v.maxMemoryUsage)
	}
	v.logger.Debug("内存限制验证通过")
	return nil
}

// =========================================================================
// 🔧 验证器管理方法
// =========================================================================

// UpdateMemoryUsage 更新内存使用量（可与池侧计数配合使用）。
func (v *ProductionBasicValidator) UpdateMemoryUsage(delta int64) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	newUsage := int64(v.currentMemoryUsage) + delta
	if newUsage < 0 {
		newUsage = 0
	}
	v.currentMemoryUsage = uint64(newUsage)
	return nil
}

// GetValidationStats 获取验证统计信息（线程安全快照）。
func (v *ProductionBasicValidator) GetValidationStats() ValidationStats {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.stats
}

// Reset 重置验证器状态（清空缓存与统计）。
func (v *ProductionBasicValidator) Reset() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.currentMemoryUsage = 0
	v.duplicateCache = make(map[string]time.Time)
	v.stats = ValidationStats{}
	v.logger.Info("基础验证器已重置")
	return nil
}

// =========================================================================
// 🔧 内部辅助方法
// =========================================================================

// updateAverageValidationTime 更新平均验证时间。
func (v *ProductionBasicValidator) updateAverageValidationTime(duration time.Duration) {
	totalValidations := v.stats.FormatValidations + v.stats.HashValidations + v.stats.SizeValidations + v.stats.DuplicateChecks + v.stats.MemoryLimitChecks
	if totalValidations > 0 {
		currentTotal := v.stats.AverageValidationTime * time.Duration(totalValidations-1)
		v.stats.AverageValidationTime = (currentTotal + duration) / time.Duration(totalValidations)
	} else {
		v.stats.AverageValidationTime = duration
	}
}

// cleanupExpiredDuplicates 清理过期的重复交易缓存（>5分钟）。
func (v *ProductionBasicValidator) cleanupExpiredDuplicates() {
	expireTime := time.Now().Add(-5 * time.Minute)
	for hash, timestamp := range v.duplicateCache {
		if timestamp.Before(expireTime) {
			delete(v.duplicateCache, hash)
		}
	}
}
