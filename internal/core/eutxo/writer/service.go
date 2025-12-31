// Package writer 实现 UTXO 写入服务
//
// 🎯 **核心职责**：
// - UTXO 创建和删除
// - 引用计数管理
// - 状态根更新
// - 性能指标收集
//
// 🏗️ **设计理念**：
// - 直接操作 Storage：不依赖 repository
// - 缓存优化：使用缓存提升性能
// - 并发安全：使用 RWMutex 保护
// - 事件驱动：发布 UTXO 变更事件
//
// 详细设计说明请参考：internal/core/eutxo/TECHNICAL_DESIGN.md
package writer

import (
	"fmt"
	"sync"

	"github.com/weisyn/v1/internal/core/eutxo/interfaces"
	"github.com/weisyn/v1/internal/core/eutxo/shared"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
)

// Service UTXO 写入服务
//
// 🎯 **核心职责**：
// - 实现 InternalUTXOWriter 接口
// - 管理 UTXO 的创建、删除、引用计数
// - 维护状态根
// - 提供性能指标
//
// 💡 **并发安全**：
// - mu: 保护 UTXO 数据操作（读写锁）
// - metricsMu: 保护性能指标更新（互斥锁）
// - 读操作：使用 RLock，允许并发读
// - 写操作：使用 Lock，独占访问
type Service struct {
	// ==================== 依赖注入 ====================

	// storage 存储服务（必需）
	storage storage.BadgerStore

	// hasher 哈希服务（必需，用于计算状态根）
	hasher crypto.HashManager

	// eventBus 事件总线（可选）
	eventBus event.EventBus

	// logger 日志记录器（可选）
	logger log.Logger

	// ==================== 内部组件 ====================

	// cache 缓存管理器
	cache *shared.Cache

	// indexManager 索引管理器
	indexManager *shared.IndexManager

	// ==================== 状态与并发保护 ====================

	// mu 并发保护（读写锁）
	mu sync.RWMutex
}

// NewService 创建 UTXO 写入服务
//
// 🎯 **创建流程**：
// 1. 验证必需依赖
// 2. 初始化缓存（容量 1000）
// 3. 初始化索引管理器
// 4. 初始化性能指标
// 5. 返回服务实例
//
// 参数：
//   - storage: 存储服务（必需）
//   - hasher: 哈希服务（必需）
//   - eventBus: 事件总线（可选）
//   - logger: 日志记录器（可选）
//
// 返回：
//   - interfaces.InternalUTXOWriter: UTXO 写入服务实例
//   - error: 创建错误，nil 表示成功
func NewService(
	storage storage.BadgerStore,
	hasher crypto.HashManager,
	eventBus event.EventBus,
	logger log.Logger,
) (interfaces.InternalUTXOWriter, error) {
	// 验证必需依赖
	if storage == nil {
		return nil, fmt.Errorf("storage 不能为空")
	}
	if hasher == nil {
		return nil, fmt.Errorf("hasher 不能为空")
	}

	// 创建服务实例
	s := &Service{
		storage:      storage,
		hasher:       hasher,
		eventBus:     eventBus,
		logger:       logger,
		cache:        shared.NewCache(1000), // 缓存 1000 个 UTXO
		indexManager: shared.NewIndexManager(storage, logger),
	}

	if logger != nil {
		logger.Info("✅ UTXOWriter 服务已创建")
	}

	return s, nil
}

// 编译时检查接口实现
var _ interfaces.InternalUTXOWriter = (*Service)(nil)

// ============================================================================
// 内存监控接口实现（MemoryReporter）
// ============================================================================

// ModuleName 返回模块名称（实现 MemoryReporter 接口）
func (s *Service) ModuleName() string {
	return "eutxo"
}

// CollectMemoryStats 收集 EUTXO 模块的内存统计信息（实现 MemoryReporter 接口）
//
// 映射规则（根据 memory-standards.md）：
// - Objects: 内存中的 UTXO 条数（例如最近高度窗口、热区 state）
// - ApproxBytes: UTXO 集 estimated bytes
// - CacheItems: UTXO 读缓存条数
// - QueueLength: 无队列
func (s *Service) CollectMemoryStats() metricsiface.ModuleMemoryStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 统计缓存中的 UTXO 数量
	cacheItems := int64(0)
	if s.cache != nil {
		cacheItems = int64(s.cache.Size())
	}
	objects := cacheItems // 缓存中的 UTXO 数量

	// 根据内存监控模式决定是否计算 ApproxBytes
	var approxBytes int64 = 0
	mode := metricsutil.GetMemoryMonitoringMode()
	if mode != "minimal" {
		// heuristic 和 accurate 模式：使用缓存内部维护的平均 UTXO 序列化大小（基于 proto.Size 的滚动统计）
		if s.cache != nil && cacheItems > 0 {
			if avg := s.cache.AvgEntrySizeBytes(); avg > 0 {
				approxBytes = cacheItems * avg
			}
		}
	}

	return metricsiface.ModuleMemoryStats{
		Module:      "eutxo",
		Layer:       "L4-CoreBusiness",
		Objects:     objects,
		ApproxBytes: approxBytes,
		CacheItems:  cacheItems,
		QueueLength: 0,
	}
}

// ShrinkCache 主动裁剪 UTXO 缓存（供 MemoryDoctor 调用）
func (s *Service) ShrinkCache(targetSize int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cache == nil {
		return
	}
	if targetSize <= 0 {
		targetSize = 1
	}
	if s.logger != nil {
		s.logger.Warnf("MemoryDoctor 触发 EUTXO Writer 缓存收缩: targetSize=%d (current=%d)",
			targetSize, s.cache.Size())
	}

	s.cache.Shrink(targetSize)
}

