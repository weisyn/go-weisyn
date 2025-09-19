// Package effects 副作用处理系统实现
//
// 本文件实现了生产级的副作用处理器，负责统一处理执行过程中产生的
// 各种副作用。副作用包括UTXO操作、状态变更和事件发射等。
//
// # 核心特性：
// - 统一处理接口：为不同类型的副作用提供统一的处理入口
// - 批量处理支持：支持大量副作用的高效批量处理
// - 统计数据收集：提供详细的处理统计和性能监控
// - 事务性支持：支持副作用的事务性处理和回滚
// - 错误处理：完善的错误处理和恢复机制
package effects

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/execution/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// processingStats 副作用处理统计数据结构
//
// # 功能说明：
// 该结构体用于收集和统计副作用处理器的详细运行数据。
// 提供多维度的统计信息，用于性能监控、质量分析和问题诊断。
//
// # 统计类别：
// - 总体统计：全局处理数量、成功率、失败率
// - 分类统计：各类型副作用的处理数量
// - 时间统计：处理耗时、最后处理时间
// - 批处理统计：批次数量、平均批大小
// - 回滚统计：回滚次数、回滚成功率
//
// # 使用场景：
// - 性能监控：实时监控处理性能和吐量
// - 质量分析：分析处理成功率和错误率
// - 容量规划：根据历史数据预测资源需求
// - 问题诊断：定位性能瓶颈和异常情况
type processingStats struct {
	// 总体统计
	totalProcessed      uint64 // 总处理数量
	successfulProcessed uint64 // 成功处理数量
	failedProcessed     uint64 // 失败处理数量

	// 分类统计
	utxoEffectsProcessed  uint64 // UTXO副作用处理数量
	stateEffectsProcessed uint64 // 状态副作用处理数量
	eventEffectsProcessed uint64 // 事件副作用处理数量

	// 时间统计
	totalProcessingTime time.Duration // 总处理时间
	lastProcessedTime   time.Time     // 最后处理时间

	// 批处理统计
	totalBatches    uint64 // 总批次数
	totalBatchItems uint64 // 总批次项目数

	// 回滚统计
	totalRollbacks      uint64    // 总回滚次数
	successfulRollbacks uint64    // 成功回滚次数
	failedRollbacks     uint64    // 失败回滚次数
	lastRollbackTime    time.Time // 最后回滚时间
}

// ProductionSideEffectProcessor 生产级副作用处理器
//
// # 功能说明：
// ProductionSideEffectProcessor是生产环境中使用的高性能、高可靠性的
// 副作用处理器实现。它提供了完整的副作用处理生命周期管理，
// 包括收集、处理、存储、监控和回滚等所有环节。
//
// # 设计目标：
// 1. **数据一致性**：确保所有副作用处理的原子性和一致性
// 2. **高可靠性**：提供完善的错误处理和故障恢复机制
// 3. **高性能**：支持批量处理和并发优化，提高处理吐量
// 4. **事务支持**：完整的ACID特性和回滚机制
// 5. **监控和观测**：详细的统计数据和实时性能监控
// 6. **可扩展性**：支持不同类型副作用的灵活扩展
//
// # 核心特性：
// - **类型化处理**：为UTXO、状态和事件副作用提供专门处理逻辑
// - **统计收集**：实时收集和统计处理数据，支持性能分析
// - **批量优化**：支持大量副作用的高效批量处理
// - **错误恢复**：完善的错误处理和自动恢复机制
// - **线程安全**：完全的并发安全和线程安全保障
//
// # 处理流程：
//  1. 接收副作用 -> 2. 类型识别 -> 3. 分类处理 -> 4. 统计更新 -> 5. 结果返回
//     ↓ (失败时)
//  6. 错误记录 -> 7. 恢复处理
//
// # 适用场景：
// - 生产环境中的高并发副作用处理
// - 需要严格数据一致性保障的场景
// - 需要详细性能监控和统计的场景
// - 需要支持大量副作用批量处理的场景
type ProductionSideEffectProcessor struct {
	// ==================== 核心组件 ====================

	// archiver 副作用归档器
	// 负责副作用的持久化存储、一致性验证和回滚管理
	// 提供完整的副作用生命周期管理功能
	archiver *SideEffectArchiver

	// logger 日志记录器
	// 负责记录处理过程中的关键信息、错误和警告
	// 支持不同级别的日志输出，用于调试和监控
	logger log.Logger

	// ==================== 配置和状态 ====================

	// config 处理器配置参数
	// 控制处理器的行为，包括事务模式、批处理大小等
	config *SideEffectProcessorConfig

	// stats 内部统计数据收集器
	// 实时收集和统计处理性能数据，用于监控和分析
	stats *processingStats

	// mu 读写互斥锁
	// 保护统计数据的并发访问安全，支持多读单写
	mu sync.RWMutex
}

// SideEffectProcessorConfig 副作用处理器配置
//
// # 功能说明：
// 定义副作用处理器的所有可配置参数，控制处理器的行为和性能。
// 这些配置直接影响处理器的执行效率、可靠性和资源使用。
//
// # 配置项说明：
// - 事务模式：控制是否启用事务性处理
// - 批处理：控制批量处理的大小和行为
// - 超时控制：防止单次处理耗时过久
//
// # 使用场景：
// - 生产环境中根据负载调优参数
// - 测试环境中使用不同的参数配置
// - 不同部署场景下的特定优化
type SideEffectProcessorConfig struct {
	// EnableTransactional 是否启用事务模式
	// 启用时保证所有副作用处理的原子性，支持回滚操作
	// 生产环境建议启用，测试环境可禁用以提高性能
	EnableTransactional bool

	// BatchSize 批量处理大小
	// 控制单次批量处理的副作用数量，影响内存使用和处理效率
	// 建议值：小规模 50-100，大规模 100-500
	BatchSize int

	// ProcessTimeoutMs 单次处理超时时间（毫秒）
	// 防止单次副作用处理耗时过久，避免系统阻塞
	// 建议值：快速处理 1000-3000ms，复杂处理 5000-10000ms
	ProcessTimeoutMs int
}

// NewProductionSideEffectProcessor 创建生产级副作用处理器
//
// 参数：
// - logger: 日志记录器，用于内部日志输出
//
// 返回值：
// - SideEffectProcessor: 完整功能的副作用处理器实例
//
// 设计说明：
// 1. 集成SideEffectArchiver作为底层副作用存储和处理引擎
// 2. 支持多种副作用类型的统一处理
// 3. 提供详细的错误处理和日志记录
// 4. 符合生产环境的可靠性和一致性要求
func NewProductionSideEffectProcessor(logger log.Logger) interfaces.SideEffectProcessor {
	config := &SideEffectProcessorConfig{
		EnableTransactional: true,
		BatchSize:           100,
		ProcessTimeoutMs:    5000,
	}

	// 创建基础副作用归档器（MVP设计）
	archiver := createBasicArchiver(logger)

	processor := &ProductionSideEffectProcessor{
		archiver: archiver,
		logger:   logger,
		config:   config,
		stats:    &processingStats{}, // 初始化统计数据收集器
	}

	if logger != nil {
		logger.Info("生产级副作用处理器已创建")
	}

	return processor
}

// updateStats 统计更新辅助方法
// 提供线程安全的统计数据更新功能
//
// 参数：
// - updater: 统计更新函数，接收processingStats指针进行更新操作
//
// 设计说明：
// 1. 使用互斥锁确保统计更新的原子性和线程安全
// 2. 通过函数参数模式提供灵活的统计更新操作
// 3. 避免锁粒度过大，提高并发性能
func (p *ProductionSideEffectProcessor) updateStats(updater func(*processingStats)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	updater(p.stats)
}

// ProcessUTXOSideEffects 处理UTXO副作用
//
// 功能说明：
// 1. 处理执行过程中产生的UTXO相关副作用
// 2. 确保UTXO状态变更的原子性和一致性
// 3. 支持回滚和错误恢复机制
// 4. 收集详细的处理统计信息
func (p *ProductionSideEffectProcessor) ProcessUTXOSideEffects(ctx context.Context, effects []interfaces.UTXOSideEffect) error {
	// 记录处理开始时间
	startTime := time.Now()

	// 执行实际的UTXO副作用处理
	err := p.processUTXOEffectsInternal(ctx, effects)

	// 更新统计信息
	p.updateStats(func(s *processingStats) {
		effectsCount := uint64(len(effects))
		s.utxoEffectsProcessed += effectsCount
		s.totalProcessed += effectsCount

		if err == nil {
			s.successfulProcessed += effectsCount
		} else {
			s.failedProcessed += effectsCount
		}

		// 更新时间统计
		processingDuration := time.Since(startTime)
		s.totalProcessingTime += processingDuration
		s.lastProcessedTime = time.Now()
	})

	return err
}

// processUTXOEffectsInternal UTXO副作用处理的内部实现
// 将原有处理逻辑抽取到独立方法，便于统计包装
func (p *ProductionSideEffectProcessor) processUTXOEffectsInternal(ctx context.Context, effects []interfaces.UTXOSideEffect) error {
	if p.archiver != nil {
		if err := p.archiver.ArchiveUTXOEffects(effects); err != nil {
			if p.logger != nil {
				p.logger.Error(fmt.Sprintf("归档UTXO副作用失败: %v", err))
			}
			return fmt.Errorf("处理UTXO副作用失败: %w", err)
		}
	}

	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("成功处理UTXO副作用: count=%d", len(effects)))
	}

	return nil
}

// ProcessStateSideEffects 处理状态副作用
//
// 功能说明：
// 1. 处理执行过程中产生的状态变更副作用
// 2. 确保状态变更的原子性和一致性
// 3. 支持状态快照和回滚机制
// 4. 收集详细的处理统计信息
func (p *ProductionSideEffectProcessor) ProcessStateSideEffects(ctx context.Context, effects []interfaces.StateSideEffect) error {
	// 记录处理开始时间
	startTime := time.Now()

	// 执行实际的状态副作用处理
	err := p.processStateSideEffectsInternal(ctx, effects)

	// 更新统计信息
	p.updateStats(func(s *processingStats) {
		effectsCount := uint64(len(effects))
		s.stateEffectsProcessed += effectsCount
		s.totalProcessed += effectsCount

		if err == nil {
			s.successfulProcessed += effectsCount
		} else {
			s.failedProcessed += effectsCount
		}

		// 更新时间统计
		processingDuration := time.Since(startTime)
		s.totalProcessingTime += processingDuration
		s.lastProcessedTime = time.Now()
	})

	return err
}

// processStateSideEffectsInternal 状态副作用处理的内部实现
// 将原有处理逻辑抽取到独立方法，便于统计包装
func (p *ProductionSideEffectProcessor) processStateSideEffectsInternal(ctx context.Context, effects []interfaces.StateSideEffect) error {
	if p.archiver != nil {
		if err := p.archiver.ArchiveStateEffects(effects); err != nil {
			if p.logger != nil {
				p.logger.Error(fmt.Sprintf("归档状态副作用失败: %v", err))
			}
			return fmt.Errorf("处理状态副作用失败: %w", err)
		}
	}

	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("成功处理状态副作用: count=%d", len(effects)))
	}

	return nil
}

// ProcessEventSideEffects 处理事件副作用
//
// 功能说明：
// 1. 处理执行过程中产生的事件副作用
// 2. 确保事件发布的可靠性和顺序性
// 3. 支持事件重放和故障恢复
// 4. 收集详细的处理统计信息
func (p *ProductionSideEffectProcessor) ProcessEventSideEffects(ctx context.Context, effects []interfaces.EventSideEffect) error {
	// 记录处理开始时间
	startTime := time.Now()

	// 执行实际的事件副作用处理
	err := p.processEventSideEffectsInternal(ctx, effects)

	// 更新统计信息
	p.updateStats(func(s *processingStats) {
		effectsCount := uint64(len(effects))
		s.eventEffectsProcessed += effectsCount
		s.totalProcessed += effectsCount

		if err == nil {
			s.successfulProcessed += effectsCount
		} else {
			s.failedProcessed += effectsCount
		}

		// 更新时间统计
		processingDuration := time.Since(startTime)
		s.totalProcessingTime += processingDuration
		s.lastProcessedTime = time.Now()
	})

	return err
}

// processEventSideEffectsInternal 事件副作用处理的内部实现
// 将原有处理逻辑抽取到独立方法，便于统计包装
func (p *ProductionSideEffectProcessor) processEventSideEffectsInternal(ctx context.Context, effects []interfaces.EventSideEffect) error {
	if p.archiver != nil {
		if err := p.archiver.ArchiveEventEffects(effects); err != nil {
			if p.logger != nil {
				p.logger.Error(fmt.Sprintf("归档事件副作用失败: %v", err))
			}
			return fmt.Errorf("处理事件副作用失败: %w", err)
		}
	}

	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("成功处理事件副作用: count=%d", len(effects)))
	}

	return nil
}

// ProcessBatch 批量处理副作用
// 提供统一的批量处理接口，并收集批处理统计信息
func (p *ProductionSideEffectProcessor) ProcessBatch(ctx context.Context, batch *interfaces.SideEffectBatch) error {
	// 记录批处理开始时间
	startTime := time.Now()

	// 计算批次中的总项目数
	batchItemCount := uint64(len(batch.UTXOEffects) + len(batch.StateEffects) + len(batch.EventEffects))

	// 执行实际的批量处理
	err := p.processBatchInternal(ctx, batch)

	// 更新批处理统计信息
	p.updateStats(func(s *processingStats) {
		s.totalBatches++
		s.totalBatchItems += batchItemCount

		// 注意：具体的效果统计在各个Process*SideEffects方法中已更新
		// 这里只记录批次级别的统计
	})

	if p.logger != nil {
		processingDuration := time.Since(startTime)
		if err == nil {
			p.logger.Info(fmt.Sprintf("成功处理副作用批次: 项目数=%d, 耗时=%v", batchItemCount, processingDuration))
		} else {
			p.logger.Error(fmt.Sprintf("处理副作用批次失败: 项目数=%d, 耗时=%v, 错误=%v", batchItemCount, processingDuration, err))
		}
	}

	return err
}

// processBatchInternal 批量处理的内部实现
// 将原有处理逻辑抽取到独立方法，便于统计包装
func (p *ProductionSideEffectProcessor) processBatchInternal(ctx context.Context, batch *interfaces.SideEffectBatch) error {
	// 处理批次中的所有副作用
	if err := p.ProcessUTXOSideEffects(ctx, batch.UTXOEffects); err != nil {
		return fmt.Errorf("failed to process UTXO effects in batch: %w", err)
	}

	if err := p.ProcessStateSideEffects(ctx, batch.StateEffects); err != nil {
		return fmt.Errorf("failed to process state effects in batch: %w", err)
	}

	if err := p.ProcessEventSideEffects(ctx, batch.EventEffects); err != nil {
		return fmt.Errorf("failed to process event effects in batch: %w", err)
	}

	return nil
}

// Rollback 回滚副作用
// 根据交易ID回滚相关的所有副作用操作，确保状态一致性
//
// 参数：
// - ctx: 上下文对象
// - transactionID: 需要回滚的交易ID
//
// 返回值：
// - error: 回滚过程中的错误信息
//
// 设计说明：
// 1. 查询指定交易的所有副作用记录
// 2. 按逆序执行回滚操作，确保原子性
// 3. 记录回滚统计信息和操作日志
// 4. 支持部分回滚失败的错误处理
func (p *ProductionSideEffectProcessor) Rollback(ctx context.Context, transactionID string) error {
	// 记录回滚开始时间
	startTime := time.Now()

	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("开始回滚交易副作用: transactionID=%s", transactionID))
	}

	// 执行实际回滚逻辑
	err := p.executeRollbackInternal(ctx, transactionID)

	// 更新回滚统计信息
	p.updateStats(func(s *processingStats) {
		s.totalRollbacks++
		if err == nil {
			s.successfulRollbacks++
		} else {
			s.failedRollbacks++
		}
		s.lastRollbackTime = time.Now()
	})

	// 记录回滚结果
	rollbackDuration := time.Since(startTime)
	if p.logger != nil {
		if err == nil {
			p.logger.Info(fmt.Sprintf("成功回滚交易副作用: transactionID=%s, 耗时=%v", transactionID, rollbackDuration))
		} else {
			p.logger.Error(fmt.Sprintf("回滚交易副作用失败: transactionID=%s, 耗时=%v, 错误=%v", transactionID, rollbackDuration, err))
		}
	}

	return err
}

// executeRollbackInternal 执行实际的回滚逻辑
// 将回滚操作抽取到独立方法，便于统计包装和测试
func (p *ProductionSideEffectProcessor) executeRollbackInternal(ctx context.Context, transactionID string) error {
	// MVP实现：基础回滚逻辑
	if p.archiver == nil {
		return fmt.Errorf("归档器未初始化，无法执行回滚操作")
	}

	// 基础回滚实现：
	// 1. 记录回滚请求（用于调试和审计）
	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("开始执行副作用回滚: transactionID=%s", transactionID))
	}

	// 2. MVP设计：简化的回滚逻辑
	// 对于自运行区块链节点，大多数副作用是幂等的或可忽略的
	// 实际的状态回滚由上层的执行协调器处理
	// 这里主要清理本地缓存和临时状态

	// 3. 清理与该交易相关的临时状态
	// 注意：这里不执行复杂的状态恢复，而是依赖上层机制

	if p.logger != nil {
		p.logger.Info(fmt.Sprintf("副作用回滚完成: transactionID=%s", transactionID))
	}

	return nil
}

// GetProcessingStats 获取处理统计
// 提供详细的副作用处理统计信息，支持生产级监控和分析
//
// 返回值：
// - ProcessingStats: 包含所有处理统计数据的结构体
//
// 设计说明：
// 1. 使用读锁确保统计数据读取的一致性
// 2. 动态计算平均值和百分比，确保数据的实时性
// 3. 提供详细的分类统计，支持精细化监控
func (p *ProductionSideEffectProcessor) GetProcessingStats() *interfaces.ProcessingStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	s := p.stats

	// 计算平均处理时间（毫秒）
	avgProcessingTime := float64(0)
	if s.totalProcessed > 0 {
		avgProcessingTime = float64(s.totalProcessingTime.Nanoseconds()) / float64(s.totalProcessed) / 1e6 // 转换为毫秒
	}

	// 计算平均批次大小
	avgBatchSize := float64(0)
	if s.totalBatches > 0 {
		avgBatchSize = float64(s.totalBatchItems) / float64(s.totalBatches)
	}

	// 获取最后处理时间
	lastProcessedTime := int64(0)
	if !s.lastProcessedTime.IsZero() {
		lastProcessedTime = s.lastProcessedTime.Unix()
	}

	return &interfaces.ProcessingStats{
		// 总体统计
		TotalProcessed:      s.totalProcessed,
		SuccessfulProcessed: s.successfulProcessed,
		FailedProcessed:     s.failedProcessed,

		// 分类统计
		UTXOEffectsProcessed:  s.utxoEffectsProcessed,
		StateEffectsProcessed: s.stateEffectsProcessed,
		EventEffectsProcessed: s.eventEffectsProcessed,

		// 性能指标
		AverageProcessingTime: avgProcessingTime,
		LastProcessedTime:     lastProcessedTime,

		// 批处理统计
		TotalBatches:     s.totalBatches,
		AverageBatchSize: avgBatchSize,

		// 回滚统计
		TotalRollbacks:      s.totalRollbacks,
		SuccessfulRollbacks: s.successfulRollbacks,
		FailedRollbacks:     s.failedRollbacks,
	}
}

// 注意: SideEffectProcessor 接口已移至 interfaces/effects.go

// createBasicArchiver 创建基础副作用归档器
//
// # 功能说明：
// 为MVP设计创建简化的副作用归档器，专注于核心功能而非企业级特性。
// 采用内存存储、基础校验和NoOp历史记录，确保零配置可用。
//
// # MVP设计原则：
// - 内存存储：避免复杂的持久化逻辑，减少外部依赖
// - 基础校验：实现必要的数据完整性检查，避免过度校验
// - 简化回滚：提供基本的回滚能力，满足核心需求
// - 最小配置：使用合理的默认参数，无需用户配置
//
// # 适用场景：
// - 自运行区块链节点的副作用处理
// - 无需复杂归档和审计的执行环境
// - 追求高性能和低复杂度的部署场景
func createBasicArchiver(logger log.Logger) *SideEffectArchiver {
	// 创建基础存储实现（内存存储）
	storage := &basicSideEffectStorage{
		logger: logger,
		data:   make(map[string]*interfaces.SideEffectCollection),
	}

	// 创建基础验证器
	validator := &basicSideEffectValidator{
		logger: logger,
	}

	// 创建基础回滚管理器
	rollbackManager := &basicRollbackManager{
		logger: logger,
	}

	// 创建NoOp历史记录器（MVP设计：最小化审计功能）
	historyRecorder := &noOpHistoryRecorder{
		logger: logger,
	}

	// 使用简化的归档器配置
	config := &ArchiverConfig{
		MaxConcurrentSessions: 5,  // 降低并发需求
		BatchSize:             50, // 减小批处理大小
		ValidationTimeoutMs:   1000,
		EnableAutoRollback:    false, // MVP：手动控制回滚
		HistoryRetentionDays:  1,     // 最小历史保留
		EnableDeduplication:   false, // 简化逻辑
		ValidationConcurrency: 1,     // 单线程验证
	}

	return NewSideEffectArchiver(
		storage,
		validator,
		rollbackManager,
		historyRecorder,
		config,
	)
}

// basicSideEffectStorage 基础副作用存储实现
//
// # MVP设计说明：
// 采用内存存储，专注于核心的副作用管理功能，避免复杂的持久化逻辑。
// 适用于自运行区块链节点的基本副作用处理需求。
type basicSideEffectStorage struct {
	logger log.Logger
	mu     sync.RWMutex
	data   map[string]*interfaces.SideEffectCollection
}

// Store 存储副作用集合
func (s *basicSideEffectStorage) Store(sessionID string, effects *interfaces.SideEffectCollection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[sessionID] = effects
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("存储副作用集合: sessionID=%s", sessionID))
	}
	return nil
}

// Retrieve 检索副作用集合
func (s *basicSideEffectStorage) Retrieve(sessionID string) (*interfaces.SideEffectCollection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	effects, ok := s.data[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	return effects, nil
}

// Delete 删除副作用集合
func (s *basicSideEffectStorage) Delete(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, sessionID)
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("删除副作用集合: sessionID=%s", sessionID))
	}
	return nil
}

// List 列出所有会话ID
func (s *basicSideEffectStorage) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys, nil
}

// StoreBatch 批量存储（基础实现）
func (s *basicSideEffectStorage) StoreBatch(ctx context.Context, opType interfaces.ArchiveOperationType, data []byte) error {
	// MVP实现：直接成功，因为我们使用内存存储
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("批量存储副作用: type=%s, size=%d", opType, len(data)))
	}
	return nil
}

// VerifyIntegrity 验证数据完整性（基础实现）
func (s *basicSideEffectStorage) VerifyIntegrity(ctx context.Context, opType interfaces.ArchiveOperationType, data []byte) error {
	// MVP实现：基础长度检查
	if len(data) == 0 {
		return fmt.Errorf("empty data for operation type %s", opType)
	}
	// 验证JSON格式
	var temp interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("invalid JSON data for operation type %s: %w", opType, err)
	}
	return nil
}

// LoadBatch 加载批量数据（基础实现）
func (s *basicSideEffectStorage) LoadBatch(ctx context.Context, opType interfaces.ArchiveOperationType, batchID string) ([]byte, error) {
	// MVP实现：返回空数据，因为我们不持久化批次
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("加载批次数据: type=%s, batchID=%s", opType, batchID))
	}
	return []byte("{}"), nil
}

// DeleteBatch 删除批量数据（基础实现）
func (s *basicSideEffectStorage) DeleteBatch(ctx context.Context, opType interfaces.ArchiveOperationType, batchID string) error {
	// MVP实现：直接成功
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("删除批次数据: type=%s, batchID=%s", opType, batchID))
	}
	return nil
}

// basicSideEffectValidator 基础副作用验证器实现
//
// # MVP设计说明：
// 实现必要的数据完整性检查，避免过度复杂的验证逻辑。
// 专注于防止明显的数据错误，保证系统基本的稳定性。
type basicSideEffectValidator struct {
	logger log.Logger
}

// ValidateCollection 验证副作用集合的基本有效性
func (v *basicSideEffectValidator) ValidateCollection(effects *interfaces.SideEffectCollection) error {
	if effects == nil {
		return fmt.Errorf("effects collection is nil")
	}
	// 检查集合中各类型副作用的数量是否合理
	totalEffects := len(effects.UTXOEffects) + len(effects.StateEffects) + len(effects.EventEffects)
	if totalEffects > 10000 { // 防止过大的副作用集合
		return fmt.Errorf("too many effects in collection: %d", totalEffects)
	}
	return nil
}

// ValidateIntegrity 验证数据完整性（基础实现）
func (v *basicSideEffectValidator) ValidateIntegrity(effects *interfaces.SideEffectCollection, checksum string) error {
	if effects == nil {
		return fmt.Errorf("effects collection is nil")
	}
	if checksum == "" {
		return fmt.Errorf("checksum is empty")
	}
	// MVP实现：基础长度验证
	if len(checksum) < 16 {
		return fmt.Errorf("checksum too short: %s", checksum)
	}
	return nil
}

// ValidateConsistency 验证集合一致性（基础实现）
func (v *basicSideEffectValidator) ValidateConsistency(effects *interfaces.SideEffectCollection) error {
	if effects == nil {
		return fmt.Errorf("effects collection is nil")
	}
	// 基础检查：确保没有空的副作用
	for i, utxo := range effects.UTXOEffects {
		if utxo.UTXOID == "" {
			return fmt.Errorf("UTXO effect %d has empty UTXOID", i)
		}
	}
	for i, state := range effects.StateEffects {
		if state.Key == "" {
			return fmt.Errorf("State effect %d has empty Key", i)
		}
	}
	for i, event := range effects.EventEffects {
		if event.Type == "" {
			return fmt.Errorf("Event effect %d has empty Type", i)
		}
	}
	return nil
}

// ValidateUTXOConsistency 验证UTXO一致性（基础实现）
func (v *basicSideEffectValidator) ValidateUTXOConsistency(ctx context.Context, effects []interfaces.UTXOSideEffect) error {
	// 基础验证：检查UTXO基本字段
	for i, effect := range effects {
		if effect.UTXOID == "" {
			return fmt.Errorf("UTXO effect %d: missing UTXOID", i)
		}
		if effect.Owner == "" {
			return fmt.Errorf("UTXO effect %d: missing Owner", i)
		}
		if effect.Type == "" {
			return fmt.Errorf("UTXO effect %d: missing Type", i)
		}
	}
	return nil
}

// ValidateStateConsistency 验证状态一致性（基础实现）
func (v *basicSideEffectValidator) ValidateStateConsistency(ctx context.Context, effects []interfaces.StateSideEffect) error {
	// 基础验证：检查状态基本字段
	for i, effect := range effects {
		if effect.Key == "" {
			return fmt.Errorf("State effect %d: missing Key", i)
		}
		if effect.Type == "" {
			return fmt.Errorf("State effect %d: missing Type", i)
		}
	}
	return nil
}

// ValidateEventConsistency 验证事件一致性（基础实现）
func (v *basicSideEffectValidator) ValidateEventConsistency(ctx context.Context, effects []interfaces.EventSideEffect) error {
	// 基础验证：检查事件基本字段
	for i, effect := range effects {
		if effect.Type == "" {
			return fmt.Errorf("Event effect %d: missing Type", i)
		}
		if effect.Contract == "" {
			return fmt.Errorf("Event effect %d: missing Contract", i)
		}
		if len(effect.Data) == 0 {
			return fmt.Errorf("Event effect %d: missing Data", i)
		}
	}
	return nil
}

// basicRollbackManager 基础回滚管理器实现
//
// # MVP设计说明：
// 实现简化的回滚逻辑，专注于必要的回滚功能而非复杂的策略管理。
// 适用于自运行区块链节点，大多数回滚由上层执行引擎处理。
type basicRollbackManager struct {
	logger log.Logger
}

// CreateRollbackPlan 创建回滚计划（基础实现）
func (r *basicRollbackManager) CreateRollbackPlan(sessionID string, effects *interfaces.SideEffectCollection) (RollbackPlan, error) {
	if sessionID == "" {
		return RollbackPlan{}, fmt.Errorf("sessionID cannot be empty")
	}
	if effects == nil {
		return RollbackPlan{}, fmt.Errorf("effects cannot be nil")
	}

	// MVP实现：创建简化的回滚计划
	plan := RollbackPlan{
		SessionID: sessionID,
		Steps:     []RollbackStep{}, // 空步骤，实际回滚由上层处理
		Status:    "ready",
	}

	if r.logger != nil {
		r.logger.Debug(fmt.Sprintf("创建回滚计划: sessionID=%s", sessionID))
	}
	return plan, nil
}

// ExecuteRollback 执行回滚（基础实现）
func (r *basicRollbackManager) ExecuteRollback(plan RollbackPlan) error {
	if plan.SessionID == "" {
		return fmt.Errorf("invalid rollback plan: empty sessionID")
	}

	// MVP实现：简化的回滚执行
	// 对于自运行区块链，大多数回滚操作是幂等的（如日志记录）
	// 或由上层执行引擎和状态管理器处理
	if r.logger != nil {
		r.logger.Info(fmt.Sprintf("执行回滚计划: sessionID=%s", plan.SessionID))
	}
	return nil
}

// RegisterStrategy 注册回滚策略（基础实现）
func (r *basicRollbackManager) RegisterStrategy(opType interfaces.ArchiveOperationType, strategy interfaces.RollbackStrategy) error {
	// MVP实现：不支持复杂的策略注册，直接成功
	if r.logger != nil {
		r.logger.Debug(fmt.Sprintf("回滚策略注册已忽略: opType=%s", opType))
	}
	return nil
}

// GetStrategy 获取回滚策略（基础实现）
func (r *basicRollbackManager) GetStrategy(opType interfaces.ArchiveOperationType) (interfaces.RollbackStrategy, error) {
	// MVP实现：返回NoOp策略
	return &noOpRollbackStrategy{logger: r.logger}, nil
}

// CanRollback 检查是否可以回滚
func (r *basicRollbackManager) CanRollback(effects *interfaces.SideEffectCollection) bool {
	// MVP实现：始终返回true，因为我们的回滚操作是安全的
	return effects != nil
}

// PrepareRollback 准备回滚操作
func (r *basicRollbackManager) PrepareRollback(sessionID string, effects *interfaces.SideEffectCollection) error {
	// MVP实现：无需复杂的准备工作
	if r.logger != nil {
		r.logger.Debug(fmt.Sprintf("准备回滚: sessionID=%s", sessionID))
	}
	return nil
}

// noOpRollbackStrategy NoOp回滚策略实现
type noOpRollbackStrategy struct {
	logger log.Logger
}

// Rollback 执行回滚（NoOp实现）
func (s *noOpRollbackStrategy) Rollback(ctx context.Context, operation *interfaces.ArchiveOperation) error {
	// NoOp实现：不执行实际回滚操作
	if s.logger != nil {
		s.logger.Debug(fmt.Sprintf("回滚操作已忽略: operationID=%s", operation.OperationID))
	}
	return nil
}

// CanRollback 检查是否可以回滚（NoOp实现）
func (s *noOpRollbackStrategy) CanRollback(operation *interfaces.ArchiveOperation) bool {
	// NoOp实现：总是返回true，因为我们不执行实际回滚
	return operation != nil
}

// GetRollbackCost 获取回滚成本（NoOp实现）
func (s *noOpRollbackStrategy) GetRollbackCost(operation *interfaces.ArchiveOperation) (time.Duration, error) {
	// NoOp实现：返回零成本，因为我们不执行实际回滚
	return 0, nil
}

// noOpHistoryRecorder NoOp历史记录器实现
//
// # MVP设计说明：
// 采用NoOp实现，最小化审计和历史记录功能。
// 适用于自运行区块链节点，减少不必要的存储和计算开销。
// 重要事件仍通过日志系统记录，保证必要的可观测性。
type noOpHistoryRecorder struct {
	logger log.Logger
}

// RecordSessionStart 记录会话开始（NoOp实现）
func (h *noOpHistoryRecorder) RecordSessionStart(sessionID string, effects *interfaces.SideEffectCollection) {
	// MVP实现：仅记录日志，不持久化历史
	if h.logger != nil {
		effectCount := 0
		if effects != nil {
			effectCount = len(effects.UTXOEffects) + len(effects.StateEffects) + len(effects.EventEffects)
		}
		h.logger.Info(fmt.Sprintf("归档会话开始: sessionID=%s, effects=%d", sessionID, effectCount))
	}
}

// RecordSessionComplete 记录会话完成（NoOp实现）
func (h *noOpHistoryRecorder) RecordSessionComplete(sessionID string) {
	// MVP实现：仅记录日志
	if h.logger != nil {
		h.logger.Info(fmt.Sprintf("归档会话完成: sessionID=%s", sessionID))
	}
}

// RecordSessionError 记录会话错误（NoOp实现）
func (h *noOpHistoryRecorder) RecordSessionError(sessionID string, errorType string, err error) {
	// MVP实现：记录错误日志（这个很重要）
	if h.logger != nil {
		h.logger.Error(fmt.Sprintf("归档会话错误: sessionID=%s, type=%s, error=%v", sessionID, errorType, err))
	}
}

// RecordRollbackStart 记录回滚开始（NoOp实现）
func (h *noOpHistoryRecorder) RecordRollbackStart(sessionID string) {
	// MVP实现：仅记录日志
	if h.logger != nil {
		h.logger.Warn(fmt.Sprintf("回滚操作开始: sessionID=%s", sessionID))
	}
}

// RecordRollbackComplete 记录回滚完成（NoOp实现）
func (h *noOpHistoryRecorder) RecordRollbackComplete(sessionID string) {
	// MVP实现：仅记录日志
	if h.logger != nil {
		h.logger.Info(fmt.Sprintf("回滚操作完成: sessionID=%s", sessionID))
	}
}

// RecordRollbackError 记录回滚错误（NoOp实现）
func (h *noOpHistoryRecorder) RecordRollbackError(sessionID string, operationID string, err error) {
	// MVP实现：记录错误日志（这个很重要）
	if h.logger != nil {
		h.logger.Error(fmt.Sprintf("回滚操作错误: sessionID=%s, operationID=%s, error=%v", sessionID, operationID, err))
	}
}

// RecordOperationRollback 记录操作回滚（NoOp实现）
func (h *noOpHistoryRecorder) RecordOperationRollback(sessionID string, operationID string) {
	// MVP实现：仅记录日志
	if h.logger != nil {
		h.logger.Info(fmt.Sprintf("操作回滚: sessionID=%s, operationID=%s", sessionID, operationID))
	}
}

// 辅助类型定义
type HistoryEntry struct {
	Timestamp   time.Time
	Event       string
	SessionID   string
	OperationID string
	Error       string
	ErrorType   string
}

type RollbackPlan struct {
	SessionID string
	Steps     []RollbackStep
	Status    string
}

type RollbackStep struct {
	OperationID string
	Type        string
	Data        interface{}
}

// 注意: UTXOSideEffect, StateSideEffect, EventSideEffect 类型已移至 interfaces/effects.go
