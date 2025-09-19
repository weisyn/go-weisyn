// Package effects 副作用处理系统
//
// 本包实现了智能合约和AI模型执行过程中产生的副作用处理机制。
// 副作用包括状态变更、事件发射、UTXO操作等，需要与执行逻辑分离
// 以确保执行的纯函数性和可预测性。
//
// # 核心特性：
// - 副作用分类处理：UTXO、状态、事件三大类副作用的专门处理
// - 原子性保障：确保副作用操作的原子性，支持事务回滚
// - 归档管理：提供完整的副作用归档和历史记录功能
// - 一致性验证：多层次的一致性校验机制
// - 可恢复性：完整的回滚和恢复机制
//
// # 设计目标：
// - 数据一致性：确保副作用处理后的数据状态一致
// - 可追溯性：完整记录副作用的产生、处理和结果
// - 高可靠性：提供故障恢复和数据修复机制
// - 性能优化：支持批量处理和并发优化
package effects

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/execution/interfaces"
)

// 注意: interfaces.SideEffectCollection 已移至 interfaces/effects.go

// SideEffectArchiver 副作用归档管理器
//
// # 功能说明：
// SideEffectArchiver是副作用处理系统的核心组件，负责将执行过程中产生的
// 各种副作用进行归档、存储和管理。它提供了完整的副作用生命周期管理，
// 包括收集、验证、存储、查询和回滚等功能。
//
// # 核心职责：
// 1. **归档会话管理**：创建和管理副作用归档会话，每个会话对应一次执行
// 2. **一致性验证**：多层次的副作用一致性校验，确保数据完整性
// 3. **批量存储**：高效的批量存储机制，优化I/O性能
// 4. **回滚支持**：完整的回滚机制，支持事务性操作
// 5. **历史记录**：详细的操作历史和审计轨迹
// 6. **会话监控**：归档会话的状态监控和生命周期管理
//
// # 设计原则：
// - **原子性**：所有副作用操作具有事务性质，要么全部成功要么全部失败
// - **一致性**：多阶段一致性校验，确保数据状态的正确性
// - **隔离性**：不同归档会话之间相互隔离，避免干扰
// - **持久性**：副作用数据持久化存储，支持故障恢复
// - **可追溯性**：完整的操作历史记录，支持审计和调试
//
// # 处理流程：
//  1. 创建归档会话 -> 2. 一致性预校验 -> 3. 批量归档操作 -> 4. 最终一致性校验 -> 5. 会话完成
//     ↓ (失败时)
//  6. 自动回滚 -> 7. 会话清理
//
// # 技术特点：
// - 支持并发归档会话，提高处理效率
// - 分批次处理大量副作用，优化内存使用
// - 校验和机制确保数据完整性
// - 灵活的回滚策略，支持不同类型的副作用回滚
// - 自动会话清理，避免资源泄漏
type SideEffectArchiver struct {
	// ==================== 核心依赖组件 ====================

	// storage 副作用存储接口
	// 负责副作用数据的持久化存储和检索，支持批量操作
	// 提供数据完整性验证和故障恢复功能
	storage interfaces.SideEffectStorage

	// validator 一致性校验器
	// 提供多层次的副作用一致性校验机制
	// 包括UTXO一致性、状态一致性和事件一致性校验
	validator interfaces.SideEffectValidator

	// rollbackManager 回滚策略管理器
	// 管理不同类型副作用的回滚策略和执行流程
	// 支持自定义回滚策略注册和管理
	rollbackManager interfaces.RollbackStrategyManager

	// historyRecorder 操作历史记录器
	// 记录副作用操作的详细审计轨迹和历史信息
	// 用于调试、审计和问题排查
	historyRecorder interfaces.OperationHistoryRecorder

	// ==================== 配置和状态管理 ====================

	// config 归档器配置参数
	// 包括并发控制、批处理大小、超时设置等核心配置
	config *ArchiverConfig

	// mutex 读写锁，保护并发访问安全
	// 用于保护sessions和sequences的并发访问
	mutex sync.RWMutex

	// sessions 活跃的归档会话集合
	// 键为会话 ID，值为归档会话的详细信息
	// 用于跟踪和管理正在进行的归档操作
	sessions map[string]*interfaces.ArchiveSession

	// sequences 序列号生成器
	// 用于生成唯一的会话 ID和操作 ID
	// 保证每个标识符的唯一性和递增性
	sequences map[string]uint64
}

// ArchiverConfig 归档器配置参数
//
// # 功能说明：
// ArchiverConfig定义了副作用归档系统的所有可配置参数。
// 这些参数控制着归档系统的性能、可靠性和资源使用。
//
// # 配置类别：
// - 性能参数：控制并发度、批处理大小等
// - 可靠性参数：超时设置、自动回滚等
// - 资源管理：历史保留、去重处理等
//
// # 使用场景：
// - 生产环境中根据负载和性能要求调优参数
// - 测试环境中使用更低的超时和更小的批处理大小
// - 调试时启用更详细的历史记录和验证
type ArchiverConfig struct {
	// MaxConcurrentSessions 最大并发归档会话数
	// 控制同时进行的归档会话数量，避免系统过载
	// 建议值：生产环境 10-50，测试环境 1-5
	MaxConcurrentSessions int

	// BatchSize 归档批次大小
	// 每批次处理的副作用数量，影响内存使用和I/O效率
	// 建议值：小文件 100-500，大文件 10-50
	BatchSize int

	// ValidationTimeoutMs 一致性校验超时时间（毫秒）
	// 控制单次一致性校验的最大执行时间
	// 建议值：轻量校验 1000-3000ms，重量校验 5000-10000ms
	ValidationTimeoutMs int64

	// EnableAutoRollback 是否启用自动回滚
	// 当归档操作失败时是否自动执行回滚操作
	// 生产环境建议启用，测试环境可选择性启用
	EnableAutoRollback bool

	// HistoryRetentionDays 历史记录保留天数
	// 控制归档历史和会话信息的保留时间
	// 建议值：生产环境 30-90天，测试环境 7-30天
	HistoryRetentionDays int

	// EnableDeduplication 是否启用副作用去重
	// 对相同的副作用进行去重处理，减少重复存储
	// 适用于高频重复操作的场景
	EnableDeduplication bool

	// ValidationConcurrency 校验并行度
	// 并行执行一致性校验的线程数
	// 建议值：CPU核心数的 1/2 到 1 倍
	ValidationConcurrency int
}

// 注意: interfaces.ArchiveSession, interfaces.ArchiveSessionStatus 等类型已移至 interfaces/effects.go

// 注意: ArchiveOperation, ArchiveOperationType, ArchiveOperationStatus 等类型已移至 interfaces/effects.go

// NewSideEffectArchiver 创建副作用归档管理器
//
// # 功能说明：
// 构造函数，创建一个完全配置的副作用归档管理器实例。
// 所有依赖组件必须在创建时提供，确保归档器的完整功能。
//
// # 参数说明：
//   - storage: 副作用存储接口，负责数据持久化
//   - validator: 一致性校验器，负责数据完整性验证
//   - rollbackManager: 回滚策略管理器，负责回滚操作
//   - historyRecorder: 历史记录器，负责审计轨迹记录
//   - config: 归档器配置参数，nil时使用默认配置
//
// # 返回值：
//   - *SideEffectArchiver: 初始化完成的归档管理器实例
//
// # 初始化状态：
//   - 所有依赖组件已正确注入
//   - 内部状态（sessions、sequences）已初始化为空
//   - 配置参数已设置（使用提供的配置或默认配置）
//   - 归档器处于就绪状态，可以接受归档请求
//
// # 使用示例：
//
//	archiver := NewSideEffectArchiver(
//		storageImpl,
//		validatorImpl,
//		rollbackMgrImpl,
//		historyRecorderImpl,
//		customConfig,
//	)
//
// # 设计考虑：
//   - 依赖注入模式，便于单元测试和模块替换
//   - 配置可选，提供合理的默认值
//   - 创建后即可使用，无需额外初始化步骤
func NewSideEffectArchiver(
	storage interfaces.SideEffectStorage,
	validator interfaces.SideEffectValidator,
	rollbackManager interfaces.RollbackStrategyManager,
	historyRecorder interfaces.OperationHistoryRecorder,
	config *ArchiverConfig,
) *SideEffectArchiver {
	// 如果未提供配置，使用默认配置确保系统正常运行
	if config == nil {
		config = DefaultArchiverConfig()
	}

	// 创建归档器实例，初始化所有字段
	return &SideEffectArchiver{
		storage:         storage,
		validator:       validator,
		rollbackManager: rollbackManager,
		historyRecorder: historyRecorder,
		config:          config,
		// 初始化空的会话和序列号映射
		sessions:  make(map[string]*interfaces.ArchiveSession),
		sequences: make(map[string]uint64),
	}
}

// DefaultArchiverConfig 默认归档器配置
func DefaultArchiverConfig() *ArchiverConfig {
	return &ArchiverConfig{
		MaxConcurrentSessions: 10,
		BatchSize:             100,
		ValidationTimeoutMs:   5000,
		EnableAutoRollback:    true,
		HistoryRetentionDays:  30,
		EnableDeduplication:   true,
		ValidationConcurrency: 4,
	}
}

// ArchiveSideEffects 归档副作用
func (a *SideEffectArchiver) ArchiveSideEffects(ctx context.Context, effects *interfaces.SideEffectCollection, metadata map[string]interface{}) (*interfaces.ArchiveSession, error) {
	// 生成会话ID
	sessionID := a.generateSessionID()

	// 检查并发会话限制
	a.mutex.Lock()
	if len(a.sessions) >= a.config.MaxConcurrentSessions {
		a.mutex.Unlock()
		return nil, fmt.Errorf("maximum concurrent sessions (%d) exceeded", a.config.MaxConcurrentSessions)
	}

	// 创建归档会话
	session := &interfaces.ArchiveSession{
		SessionID:   sessionID,
		StartTime:   time.Now().Unix(),
		Status:      interfaces.ArchiveSessionStatusPending,
		SideEffects: effects,
		Checksum:    a.calculateChecksum(effects),
		Operations:  []interfaces.ArchiveOperation{},
		Metadata:    metadata,
	}

	a.sessions[sessionID] = session
	a.mutex.Unlock()

	// 记录会话开始
	a.historyRecorder.RecordSessionStart(sessionID, effects)

	// 阶段1：一致性校验
	session.Status = interfaces.ArchiveSessionStatusValidating
	if err := a.validateConsistency(ctx, session); err != nil {
		session.Status = interfaces.ArchiveSessionStatusFailed
		a.historyRecorder.RecordSessionError(sessionID, "validation_failed", err)
		return session, fmt.Errorf("consistency validation failed: %w", err)
	}

	// 阶段2：执行归档操作
	session.Status = interfaces.ArchiveSessionStatusArchiving
	if err := a.executeArchiveOperations(ctx, session); err != nil {
		session.Status = interfaces.ArchiveSessionStatusFailed
		a.historyRecorder.RecordSessionError(sessionID, "archive_failed", err)

		// 如果启用自动回滚，执行回滚
		if a.config.EnableAutoRollback {
			if rollbackErr := a.rollbackSession(ctx, session); rollbackErr != nil {
				a.historyRecorder.RecordSessionError(sessionID, "rollback_failed", rollbackErr)
			}
		}

		return session, fmt.Errorf("archive operations failed: %w", err)
	}

	// 阶段3：最终一致性校验
	if err := a.validateFinalConsistency(ctx, session); err != nil {
		session.Status = interfaces.ArchiveSessionStatusFailed
		a.historyRecorder.RecordSessionError(sessionID, "final_validation_failed", err)

		// 自动回滚
		if a.config.EnableAutoRollback {
			if rollbackErr := a.rollbackSession(ctx, session); rollbackErr != nil {
				a.historyRecorder.RecordSessionError(sessionID, "rollback_failed", rollbackErr)
			}
		}

		return session, fmt.Errorf("final consistency validation failed: %w", err)
	}

	// 归档成功
	session.Status = interfaces.ArchiveSessionStatusCompleted
	a.historyRecorder.RecordSessionComplete(sessionID)

	return session, nil
}

// RollbackSession 回滚归档会话
func (a *SideEffectArchiver) RollbackSession(ctx context.Context, sessionID string) error {
	a.mutex.RLock()
	session, exists := a.sessions[sessionID]
	a.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("archive session %s not found", sessionID)
	}

	return a.rollbackSession(ctx, session)
}

// rollbackSession 内部回滚实现
func (a *SideEffectArchiver) rollbackSession(ctx context.Context, session *interfaces.ArchiveSession) error {
	// 记录回滚开始
	a.historyRecorder.RecordRollbackStart(session.SessionID)

	// 逆序处理已执行的操作
	for i := len(session.Operations) - 1; i >= 0; i-- {
		operation := &session.Operations[i]
		if operation.Status != interfaces.ArchiveOperationStatusExecuted {
			continue // 跳过未执行的操作
		}

		// 获取回滚策略
		strategy, err := a.rollbackManager.GetStrategy(operation.OperationType)
		if err != nil {
			return fmt.Errorf("failed to get rollback strategy for operation %s: %w", operation.OperationID, err)
		}

		// 执行回滚
		if err := strategy.Rollback(ctx, operation); err != nil {
			a.historyRecorder.RecordRollbackError(session.SessionID, operation.OperationID, err)
			return fmt.Errorf("failed to rollback operation %s: %w", operation.OperationID, err)
		}

		operation.Status = interfaces.ArchiveOperationStatusRolledBack
		a.historyRecorder.RecordOperationRollback(session.SessionID, operation.OperationID)
	}

	session.Status = interfaces.ArchiveSessionStatusRolledBack
	a.historyRecorder.RecordRollbackComplete(session.SessionID)

	return nil
}

// GetSessionStatus 获取会话状态
func (a *SideEffectArchiver) GetSessionStatus(sessionID string) (*interfaces.ArchiveSession, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	session, exists := a.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("archive session %s not found", sessionID)
	}

	// 返回会话副本
	sessionCopy := *session
	return &sessionCopy, nil
}

// CleanupExpiredSessions 清理过期会话
func (a *SideEffectArchiver) CleanupExpiredSessions() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	cutoffTime := time.Now().AddDate(0, 0, -a.config.HistoryRetentionDays).Unix()
	for sessionID, session := range a.sessions {
		if session.StartTime < cutoffTime &&
			(session.Status == interfaces.ArchiveSessionStatusCompleted || session.Status == interfaces.ArchiveSessionStatusFailed || session.Status == interfaces.ArchiveSessionStatusRolledBack) {
			delete(a.sessions, sessionID)
		}
	}
}

// generateSessionID 生成唯一会话ID
func (a *SideEffectArchiver) generateSessionID() string {
	timestamp := time.Now().UnixNano()
	sequence := a.getNextSequence("session")
	return fmt.Sprintf("archive_%d_%d", timestamp, sequence)
}

// getNextSequence 获取下一个序列号
func (a *SideEffectArchiver) getNextSequence(key string) uint64 {
	a.sequences[key]++
	return a.sequences[key]
}

// calculateChecksum 计算副作用集合的校验和
func (a *SideEffectArchiver) calculateChecksum(effects *interfaces.SideEffectCollection) string {
	data, _ := json.Marshal(effects)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// validateConsistency 校验一致性
func (a *SideEffectArchiver) validateConsistency(ctx context.Context, session *interfaces.ArchiveSession) error {
	// 设置校验超时
	timeout := time.Duration(a.config.ValidationTimeoutMs) * time.Millisecond
	validationCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 校验UTXO副作用一致性
	if len(session.SideEffects.UTXOEffects) > 0 {
		if err := a.validator.ValidateUTXOConsistency(validationCtx, session.SideEffects.UTXOEffects); err != nil {
			return fmt.Errorf("UTXO consistency validation failed: %w", err)
		}
	}

	// 校验状态副作用一致性
	if len(session.SideEffects.StateEffects) > 0 {
		if err := a.validator.ValidateStateConsistency(validationCtx, session.SideEffects.StateEffects); err != nil {
			return fmt.Errorf("state consistency validation failed: %w", err)
		}
	}

	// 校验事件副作用一致性
	if len(session.SideEffects.EventEffects) > 0 {
		if err := a.validator.ValidateEventConsistency(validationCtx, session.SideEffects.EventEffects); err != nil {
			return fmt.Errorf("event consistency validation failed: %w", err)
		}
	}

	return nil
}

// executeArchiveOperations 执行归档操作
func (a *SideEffectArchiver) executeArchiveOperations(ctx context.Context, session *interfaces.ArchiveSession) error {
	// 按批次处理UTXO副作用
	if err := a.archiveInBatches(ctx, session, session.SideEffects.UTXOEffects, interfaces.ArchiveOperationTypeUTXO); err != nil {
		return fmt.Errorf("failed to archive UTXO effects: %w", err)
	}

	// 按批次处理状态副作用
	if err := a.archiveInBatches(ctx, session, session.SideEffects.StateEffects, interfaces.ArchiveOperationTypeState); err != nil {
		return fmt.Errorf("failed to archive state effects: %w", err)
	}

	// 按批次处理事件副作用
	if err := a.archiveInBatches(ctx, session, session.SideEffects.EventEffects, interfaces.ArchiveOperationTypeEvent); err != nil {
		return fmt.Errorf("failed to archive event effects: %w", err)
	}

	return nil
}

// archiveInBatches 按批次归档副作用
func (a *SideEffectArchiver) archiveInBatches(ctx context.Context, session *interfaces.ArchiveSession, effects interface{}, opType interfaces.ArchiveOperationType) error {
	var effectsSlice []interface{}

	// 将具体类型转换为interface{}切片
	switch v := effects.(type) {
	case []interfaces.UTXOSideEffect:
		for _, effect := range v {
			effectsSlice = append(effectsSlice, effect)
		}
	case []interfaces.StateSideEffect:
		for _, effect := range v {
			effectsSlice = append(effectsSlice, effect)
		}
	case []interfaces.EventSideEffect:
		for _, effect := range v {
			effectsSlice = append(effectsSlice, effect)
		}
	default:
		return fmt.Errorf("unsupported effect type: %T", effects)
	}

	// 按批次处理
	batchSize := a.config.BatchSize
	for i := 0; i < len(effectsSlice); i += batchSize {
		end := i + batchSize
		if end > len(effectsSlice) {
			end = len(effectsSlice)
		}
		batch := effectsSlice[i:end]

		// 序列化批次数据
		batchData, err := json.Marshal(batch)
		if err != nil {
			return fmt.Errorf("failed to serialize batch: %w", err)
		}

		// 创建归档操作
		operation := interfaces.ArchiveOperation{
			OperationID:   fmt.Sprintf("%s_%s_%d", session.SessionID, opType, len(session.Operations)),
			OperationType: opType,
			Timestamp:     time.Now().Unix(),
			Data:          batchData,
			Status:        interfaces.ArchiveOperationStatusPending,
		}

		// 执行存储操作
		if err := a.storage.StoreBatch(ctx, opType, batchData); err != nil {
			operation.Status = interfaces.ArchiveOperationStatusFailed
			operation.ErrorMessage = err.Error()
			session.Operations = append(session.Operations, operation)
			return fmt.Errorf("failed to store batch: %w", err)
		}

		operation.Status = interfaces.ArchiveOperationStatusExecuted
		session.Operations = append(session.Operations, operation)
	}

	return nil
}

// validateFinalConsistency 最终一致性校验
func (a *SideEffectArchiver) validateFinalConsistency(ctx context.Context, session *interfaces.ArchiveSession) error {
	// 重新计算校验和并比较
	currentChecksum := a.calculateChecksum(session.SideEffects)
	if currentChecksum != session.Checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", session.Checksum, currentChecksum)
	}

	// 验证存储的数据完整性
	for _, operation := range session.Operations {
		if operation.Status != interfaces.ArchiveOperationStatusExecuted {
			continue
		}

		if err := a.storage.VerifyIntegrity(ctx, operation.OperationType, operation.Data); err != nil {
			return fmt.Errorf("integrity verification failed for operation %s: %w", operation.OperationID, err)
		}
	}

	return nil
}

// ==================== 接口定义已移至 interfaces/effects.go ====================

// ArchiveUTXOEffects 归档UTXO副作用
func (a *SideEffectArchiver) ArchiveUTXOEffects(effects []interfaces.UTXOSideEffect) error {
	if a.storage == nil {
		return fmt.Errorf("storage is not available for archiving")
	}

	if len(effects) == 0 {
		return nil // 没有副作用需要归档
	}

	// 创建归档批次
	archiveBatch := &UTXOArchiveBatch{
		Timestamp: time.Now(),
		Effects:   effects,
		BatchID:   a.generateBatchID(),
	}

	// 序列化批次数据
	data, err := json.Marshal(archiveBatch)
	if err != nil {
		return fmt.Errorf("failed to marshal UTXO archive batch: %w", err)
	}

	// 存储到storage
	return a.storage.StoreBatch(context.Background(), interfaces.ArchiveOperationTypeUTXO, data)
}

// ArchiveStateEffects 归档状态副作用
func (a *SideEffectArchiver) ArchiveStateEffects(effects []interfaces.StateSideEffect) error {
	if a.storage == nil {
		return fmt.Errorf("storage is not available for archiving")
	}

	if len(effects) == 0 {
		return nil // 没有副作用需要归档
	}

	// 创建归档批次
	archiveBatch := &StateArchiveBatch{
		Timestamp: time.Now(),
		Effects:   effects,
		BatchID:   a.generateBatchID(),
	}

	// 序列化批次数据
	data, err := json.Marshal(archiveBatch)
	if err != nil {
		return fmt.Errorf("failed to marshal state archive batch: %w", err)
	}

	// 存储到storage
	return a.storage.StoreBatch(context.Background(), interfaces.ArchiveOperationTypeState, data)
}

// ArchiveEventEffects 归档事件副作用
func (a *SideEffectArchiver) ArchiveEventEffects(effects []interfaces.EventSideEffect) error {
	if a.storage == nil {
		return fmt.Errorf("storage is not available for archiving")
	}

	if len(effects) == 0 {
		return nil // 没有副作用需要归档
	}

	// 创建归档批次
	archiveBatch := &EventArchiveBatch{
		Timestamp: time.Now(),
		Effects:   effects,
		BatchID:   a.generateBatchID(),
	}

	// 序列化批次数据
	data, err := json.Marshal(archiveBatch)
	if err != nil {
		return fmt.Errorf("failed to marshal event archive batch: %w", err)
	}

	// 存储到storage
	return a.storage.StoreBatch(context.Background(), interfaces.ArchiveOperationTypeEvent, data)
}

// generateBatchID 生成唯一的批次ID
func (a *SideEffectArchiver) generateBatchID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// 归档批次数据结构
type UTXOArchiveBatch struct {
	Timestamp time.Time                   `json:"timestamp"`
	Effects   []interfaces.UTXOSideEffect `json:"effects"`
	BatchID   string                      `json:"batch_id"`
}

type StateArchiveBatch struct {
	Timestamp time.Time                    `json:"timestamp"`
	Effects   []interfaces.StateSideEffect `json:"effects"`
	BatchID   string                       `json:"batch_id"`
}

type EventArchiveBatch struct {
	Timestamp time.Time                    `json:"timestamp"`
	Effects   []interfaces.EventSideEffect `json:"effects"`
	BatchID   string                       `json:"batch_id"`
}
