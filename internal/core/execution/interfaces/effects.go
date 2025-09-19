package interfaces

import (
	"context"
	"time"
)

// ==================== 副作用处理内部接口 ====================
// 这些接口供execution内部子目录相互调用，不对外暴露

// SideEffectProcessor 副作用处理器接口
// 由effects包实现，供coordinator调用
type SideEffectProcessor interface {
	// 处理UTXO副作用
	ProcessUTXOSideEffects(ctx context.Context, effects []UTXOSideEffect) error

	// 处理状态副作用
	ProcessStateSideEffects(ctx context.Context, effects []StateSideEffect) error

	// 处理事件副作用
	ProcessEventSideEffects(ctx context.Context, effects []EventSideEffect) error

	// 批量处理副作用
	ProcessBatch(ctx context.Context, batch *SideEffectBatch) error

	// 回滚副作用
	Rollback(ctx context.Context, transactionID string) error

	// 获取处理统计
	GetProcessingStats() *ProcessingStats
}

// ProcessingStats 副作用处理统计信息
// 提供详细的副作用处理性能指标和统计数据
type ProcessingStats struct {
	// 总体统计
	TotalProcessed      uint64 `json:"total_processed"`      // 总处理数量
	SuccessfulProcessed uint64 `json:"successful_processed"` // 成功处理数量
	FailedProcessed     uint64 `json:"failed_processed"`     // 失败处理数量

	// 分类统计
	UTXOEffectsProcessed  uint64 `json:"utxo_effects_processed"`  // UTXO副作用处理数量
	StateEffectsProcessed uint64 `json:"state_effects_processed"` // 状态副作用处理数量
	EventEffectsProcessed uint64 `json:"event_effects_processed"` // 事件副作用处理数量

	// 性能指标
	AverageProcessingTime float64 `json:"average_processing_time"` // 平均处理时间（毫秒）
	LastProcessedTime     int64   `json:"last_processed_time"`     // 最后处理时间（Unix时间戳）

	// 批处理统计
	TotalBatches     uint64  `json:"total_batches"`      // 总批次数
	AverageBatchSize float64 `json:"average_batch_size"` // 平均批次大小

	// 回滚统计
	TotalRollbacks      uint64 `json:"total_rollbacks"`      // 总回滚次数
	SuccessfulRollbacks uint64 `json:"successful_rollbacks"` // 成功回滚次数
	FailedRollbacks     uint64 `json:"failed_rollbacks"`     // 失败回滚次数
}

// SideEffectArchiver 副作用归档器接口
// 由effects包实现，供coordinator调用
type SideEffectArchiver interface {
	// 归档副作用
	Archive(ctx context.Context, effects *SideEffectBatch) error

	// 查询归档记录
	QueryArchive(ctx context.Context, filter ArchiveFilter) ([]*ArchivedEffect, error)

	// 清理过期归档
	CleanupExpiredArchives(ctx context.Context, retentionDays int) error

	// 获取归档统计
	GetArchiveStats() *ArchiveStats
}

// ==================== 数据结构定义 ====================

// UTXOSideEffect UTXO副作用
type UTXOSideEffect struct {
	Type      UTXOEffectType         `json:"type"`
	UTXOID    string                 `json:"utxo_id"`
	Amount    uint64                 `json:"amount"`
	Owner     string                 `json:"owner"`
	TokenType string                 `json:"token_type"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// UTXOEffectType UTXO副作用类型
type UTXOEffectType string

const (
	UTXOEffectCreate  UTXOEffectType = "create"
	UTXOEffectConsume UTXOEffectType = "consume"
	UTXOEffectUpdate  UTXOEffectType = "update"
)

// StateSideEffect 状态副作用
type StateSideEffect struct {
	Type     StateEffectType        `json:"type"`
	Key      string                 `json:"key"`
	OldValue []byte                 `json:"old_value"`
	NewValue []byte                 `json:"new_value"`
	Contract string                 `json:"contract"`
	Metadata map[string]interface{} `json:"metadata"`
}

// StateEffectType 状态副作用类型
type StateEffectType string

const (
	StateEffectSet    StateEffectType = "set"
	StateEffectDelete StateEffectType = "delete"
	StateEffectUpdate StateEffectType = "update"
)

// EventSideEffect 事件副作用
type EventSideEffect struct {
	Type      EventEffectType        `json:"type"`
	EventName string                 `json:"event_name"`
	Contract  string                 `json:"contract"`
	Data      map[string]interface{} `json:"data"`
	Indexed   []string               `json:"indexed"`
	Timestamp int64                  `json:"timestamp"`
}

// EventEffectType 事件副作用类型
type EventEffectType string

const (
	EventEffectEmit   EventEffectType = "emit"
	EventEffectLog    EventEffectType = "log"
	EventEffectNotify EventEffectType = "notify"
)

// SideEffectBatch 副作用批次
type SideEffectBatch struct {
	TransactionID    string            `json:"transaction_id"`
	BlockHeight      uint64            `json:"block_height"`
	Timestamp        int64             `json:"timestamp"`
	UTXOEffects      []UTXOSideEffect  `json:"utxo_effects"`
	StateEffects     []StateSideEffect `json:"state_effects"`
	EventEffects     []EventSideEffect `json:"event_effects"`
	ResourceConsumed uint64            `json:"resource_consumed"`
	Success          bool              `json:"success"`
	ErrorMessage     string            `json:"error_message,omitempty"`
}

// ArchiveFilter 归档过滤器
type ArchiveFilter struct {
	TransactionID string  `json:"transaction_id,omitempty"`
	BlockHeight   *uint64 `json:"block_height,omitempty"`
	StartTime     *int64  `json:"start_time,omitempty"`
	EndTime       *int64  `json:"end_time,omitempty"`
	Contract      string  `json:"contract,omitempty"`
	EffectType    string  `json:"effect_type,omitempty"`
	Limit         int     `json:"limit,omitempty"`
	Offset        int     `json:"offset,omitempty"`
}

// ArchivedEffect 归档的副作用
type ArchivedEffect struct {
	ID            string                 `json:"id"`
	TransactionID string                 `json:"transaction_id"`
	BlockHeight   uint64                 `json:"block_height"`
	Timestamp     int64                  `json:"timestamp"`
	EffectType    string                 `json:"effect_type"`
	EffectData    map[string]interface{} `json:"effect_data"`
	ArchivedAt    int64                  `json:"archived_at"`
}

// ArchiveStats 归档统计
type ArchiveStats struct {
	TotalArchived     int64            `json:"total_archived"`
	ArchivedByType    map[string]int64 `json:"archived_by_type"`
	OldestArchiveTime int64            `json:"oldest_archive_time"`
	NewestArchiveTime int64            `json:"newest_archive_time"`
	TotalStorageSize  int64            `json:"total_storage_size_bytes"`
}

// ==================== 归档相关接口 ====================

// SideEffectStorage 副作用存储接口
// 由effects包实现，供SideEffectArchiver调用
type SideEffectStorage interface {
	StoreBatch(ctx context.Context, opType ArchiveOperationType, data []byte) error
	VerifyIntegrity(ctx context.Context, opType ArchiveOperationType, data []byte) error
	LoadBatch(ctx context.Context, opType ArchiveOperationType, batchID string) ([]byte, error)
	DeleteBatch(ctx context.Context, opType ArchiveOperationType, batchID string) error
}

// SideEffectValidator 副作用校验器接口
// 由effects包实现，供SideEffectArchiver调用
type SideEffectValidator interface {
	ValidateUTXOConsistency(ctx context.Context, effects []UTXOSideEffect) error
	ValidateStateConsistency(ctx context.Context, effects []StateSideEffect) error
	ValidateEventConsistency(ctx context.Context, effects []EventSideEffect) error
}

// RollbackStrategyManager 回滚策略管理器接口
// 由effects包实现，供SideEffectArchiver调用
type RollbackStrategyManager interface {
	GetStrategy(opType ArchiveOperationType) (RollbackStrategy, error)
	RegisterStrategy(opType ArchiveOperationType, strategy RollbackStrategy) error
}

// RollbackStrategy 回滚策略接口
// 由effects包实现，供RollbackStrategyManager调用
type RollbackStrategy interface {
	Rollback(ctx context.Context, operation *ArchiveOperation) error
	CanRollback(operation *ArchiveOperation) bool
	GetRollbackCost(operation *ArchiveOperation) (time.Duration, error)
}

// OperationHistoryRecorder 操作历史记录器接口
// 由effects包实现，供SideEffectArchiver调用
type OperationHistoryRecorder interface {
	RecordSessionStart(sessionID string, effects *SideEffectCollection)
	RecordSessionComplete(sessionID string)
	RecordSessionError(sessionID string, errorType string, err error)
	RecordRollbackStart(sessionID string)
	RecordRollbackComplete(sessionID string)
	RecordRollbackError(sessionID string, operationID string, err error)
	RecordOperationRollback(sessionID string, operationID string)
}

// ArchiveOperationType 归档操作类型
type ArchiveOperationType string

const (
	ArchiveOperationTypeUTXO  ArchiveOperationType = "utxo"
	ArchiveOperationTypeState ArchiveOperationType = "state"
	ArchiveOperationTypeEvent ArchiveOperationType = "event"
)

// ArchiveOperation 归档操作
type ArchiveOperation struct {
	OperationID   string                 `json:"operation_id"`
	OperationType ArchiveOperationType   `json:"operation_type"`
	Timestamp     int64                  `json:"timestamp"`
	Status        ArchiveOperationStatus `json:"status"`
	Data          []byte                 `json:"data"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
}

// ArchiveOperationStatus 归档操作状态
type ArchiveOperationStatus string

const (
	ArchiveOperationStatusPending    ArchiveOperationStatus = "pending"
	ArchiveOperationStatusExecuted   ArchiveOperationStatus = "executed"
	ArchiveOperationStatusFailed     ArchiveOperationStatus = "failed"
	ArchiveOperationStatusRolledBack ArchiveOperationStatus = "rolled_back"
)

// SideEffectCollection 副作用集合
type SideEffectCollection struct {
	UTXOEffects  []UTXOSideEffect  `json:"utxo_effects"`
	StateEffects []StateSideEffect `json:"state_effects"`
	EventEffects []EventSideEffect `json:"event_effects"`
}

// ArchiveSession 归档会话
type ArchiveSession struct {
	SessionID   string                 `json:"session_id"`
	Status      ArchiveSessionStatus   `json:"status"`
	StartTime   int64                  `json:"start_time"`
	EndTime     int64                  `json:"end_time"`
	SideEffects *SideEffectCollection  `json:"side_effects"`
	Checksum    string                 `json:"checksum"`
	Operations  []ArchiveOperation     `json:"operations"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ArchiveSessionStatus 归档会话状态
type ArchiveSessionStatus string

const (
	ArchiveSessionStatusPending    ArchiveSessionStatus = "pending"
	ArchiveSessionStatusValidating ArchiveSessionStatus = "validating"
	ArchiveSessionStatusArchiving  ArchiveSessionStatus = "archiving"
	ArchiveSessionStatusActive     ArchiveSessionStatus = "active"
	ArchiveSessionStatusCompleted  ArchiveSessionStatus = "completed"
	ArchiveSessionStatusFailed     ArchiveSessionStatus = "failed"
	ArchiveSessionStatusRolledBack ArchiveSessionStatus = "rolled_back"
)
