package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	repositoryConfig "github.com/weisyn/v1/internal/config/repository"
	"github.com/weisyn/v1/internal/core/repositories/repository/utxo"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
)

// ============================================================================
//                          📦 Outbox模式实现
// ============================================================================

// OutboxEvent Outbox事件定义
// 用于确保UTXO更新的原子性和可靠性
// 🔧 修复：移除JSON标签，Payload改为专用的强类型字段
type OutboxEvent struct {
	ID          string          `json:"id"`           // 事件唯一ID（保留JSON用于存储）
	Type        OutboxEventType `json:"type"`         // 事件类型
	BlockHeight uint64          `json:"block_height"` // 区块高度
	BlockHash   []byte          `json:"block_hash"`   // 区块哈希
	// 🚨 关键修复：不再使用map[string]interface{}，改为具体类型存储
	BlockData   []byte            `json:"block_data"`   // Block的protobuf序列化数据
	CreatedAt   time.Time         `json:"created_at"`   // 创建时间
	ProcessedAt *time.Time        `json:"processed_at"` // 处理时间（nil表示未处理）
	Attempts    int               `json:"attempts"`     // 尝试次数
	LastError   string            `json:"last_error"`   // 最后错误信息
	Status      OutboxEventStatus `json:"status"`       // 事件状态
}

// OutboxEventType 事件类型
type OutboxEventType string

const (
	EventTypeBlockAdded   OutboxEventType = "block_added"   // 区块添加事件
	EventTypeBlockRemoved OutboxEventType = "block_removed" // 区块移除事件
)

// OutboxEventStatus 事件状态
type OutboxEventStatus string

const (
	EventStatusPending    OutboxEventStatus = "pending"    // 待处理
	EventStatusProcessing OutboxEventStatus = "processing" // 处理中
	EventStatusCompleted  OutboxEventStatus = "completed"  // 已完成
	EventStatusFailed     OutboxEventStatus = "failed"     // 处理失败
)

// Outbox存储键前缀
const (
	OutboxKeyPrefix = "outbox:" // outbox:<event_id> -> OutboxEvent
)

// OutboxManager Outbox管理器
type OutboxManager struct {
	storage storage.BadgerStore
	logger  log.Logger
}

// NewOutboxManager 创建Outbox管理器（使用默认配置）
func NewOutboxManager(storage storage.BadgerStore, logger log.Logger) *OutboxManager {
	return &OutboxManager{
		storage: storage,
		logger:  logger,
	}
}

// NewOutboxManagerWithConfig 创建Outbox管理器（使用配置）
func NewOutboxManagerWithConfig(storage storage.BadgerStore, logger log.Logger, config *repositoryConfig.OutboxConfig) *OutboxManager {
	return &OutboxManager{
		storage: storage,
		logger:  logger,
	}
}

// ========== Outbox事件管理 ==========

// AddBlockAddedEvent 在事务中添加区块添加事件
func (om *OutboxManager) AddBlockAddedEvent(tx storage.BadgerTransaction, block *core.Block, blockHash []byte) error {
	// 🔧 修复：将Block序列化为protobuf字节数据
	blockData, err := proto.Marshal(block)
	if err != nil {
		return fmt.Errorf("序列化Block数据失败: %w", err)
	}

	event := &OutboxEvent{
		ID:          generateEventID(block.Header.Height, blockHash),
		Type:        EventTypeBlockAdded,
		BlockHeight: block.Header.Height,
		BlockHash:   blockHash,
		BlockData:   blockData, // 使用新的protobuf字段
		CreatedAt:   time.Now(),
		Status:      EventStatusPending,
		Attempts:    0,
	}

	return om.storeEvent(tx, event)
}

// AddBlockRemovedEvent 在事务中添加区块移除事件
func (om *OutboxManager) AddBlockRemovedEvent(tx storage.BadgerTransaction, block *core.Block, blockHash []byte) error {
	// 🔧 修复：将Block序列化为protobuf字节数据
	blockData, err := proto.Marshal(block)
	if err != nil {
		return fmt.Errorf("序列化Block数据失败: %w", err)
	}

	event := &OutboxEvent{
		ID:          generateEventID(block.Header.Height, blockHash) + "_removed",
		Type:        EventTypeBlockRemoved,
		BlockHeight: block.Header.Height,
		BlockHash:   blockHash,
		BlockData:   blockData, // 使用新的protobuf字段
		CreatedAt:   time.Now(),
		Status:      EventStatusPending,
		Attempts:    0,
	}

	return om.storeEvent(tx, event)
}

// GetPendingEvents 获取待处理的事件
func (om *OutboxManager) GetPendingEvents(ctx context.Context) ([]*OutboxEvent, error) {
	var events []*OutboxEvent

	// 使用前缀扫描获取所有outbox事件
	prefix := []byte(OutboxKeyPrefix)
	results, err := om.storage.PrefixScan(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("扫描outbox事件失败: %w", err)
	}

	// 解析事件并筛选待处理的事件
	for _, value := range results {
		var event OutboxEvent
		if err := json.Unmarshal(value, &event); err != nil {
			if om.logger != nil {
				om.logger.Warnf("反序列化outbox事件失败: %v", err)
			}
			continue // 跳过损坏的事件
		}

		// 只返回待处理的事件
		if event.Status == EventStatusPending {
			events = append(events, &event)
		}
	}

	return events, nil
}

// MarkEventProcessing 标记事件为处理中
func (om *OutboxManager) MarkEventProcessing(ctx context.Context, eventID string) error {
	return om.updateEventStatus(ctx, eventID, EventStatusProcessing)
}

// MarkEventCompleted 标记事件为已完成
func (om *OutboxManager) MarkEventCompleted(ctx context.Context, eventID string) error {
	return om.updateEventStatus(ctx, eventID, EventStatusCompleted)
}

// MarkEventFailed 标记事件为失败
func (om *OutboxManager) MarkEventFailed(ctx context.Context, eventID string, errorMsg string) error {
	key := formatOutboxKey(eventID)

	// 获取现有事件
	data, err := om.storage.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("获取事件失败: %w", err)
	}

	var event OutboxEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("反序列化事件失败: %w", err)
	}

	// 更新事件状态
	event.Status = EventStatusFailed
	event.Attempts++
	event.LastError = errorMsg

	// 存储更新后的事件
	updatedData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化事件失败: %w", err)
	}

	return om.storage.Set(ctx, key, updatedData)
}

// ========== 内部辅助方法 ==========

// storeEvent 存储事件到outbox
func (om *OutboxManager) storeEvent(tx storage.BadgerTransaction, event *OutboxEvent) error {
	key := formatOutboxKey(event.ID)

	// 🔧 修复：检查BlockData是否为有效的protobuf数据
	if len(event.BlockData) > 0 {
		// 验证protobuf数据的完整性
		var testBlock core.Block
		if err := proto.Unmarshal(event.BlockData, &testBlock); err != nil {
			if om.logger != nil {
				om.logger.Warnf("⚠️ 检测到JSON格式的block数据，跳过处理以避免protobuf oneof字段反序列化错误")
			}
			// 清空损坏的BlockData，避免后续处理错误
			event.BlockData = nil
		}
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化outbox事件失败: %w", err)
	}

	return tx.Set(key, data)
}

// updateEventStatus 更新事件状态
func (om *OutboxManager) updateEventStatus(ctx context.Context, eventID string, status OutboxEventStatus) error {
	key := formatOutboxKey(eventID)

	// 获取现有事件
	data, err := om.storage.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("获取事件失败: %w", err)
	}

	var event OutboxEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("反序列化事件失败: %w", err)
	}

	// 更新状态
	event.Status = status
	if status == EventStatusCompleted {
		now := time.Now()
		event.ProcessedAt = &now
	}

	// 存储更新后的事件
	updatedData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化事件失败: %w", err)
	}

	return om.storage.Set(ctx, key, updatedData)
}

// formatOutboxKey 格式化outbox存储键
func formatOutboxKey(eventID string) []byte {
	key := make([]byte, len(OutboxKeyPrefix)+len(eventID))
	copy(key, OutboxKeyPrefix)
	copy(key[len(OutboxKeyPrefix):], eventID)
	return key
}

// generateEventID 生成事件ID
func generateEventID(height uint64, blockHash []byte) string {
	return fmt.Sprintf("%d_%x", height, blockHash[:8]) // 使用高度和哈希前8字节
}

// ========== Outbox事件处理器 ==========

// OutboxProcessor Outbox事件处理器
type OutboxProcessor struct {
	outboxManager *OutboxManager
	utxoClient    *utxo.UTXOService
	logger        log.Logger
	maxRetries    int
	retryDelay    time.Duration
}

// NewOutboxProcessor 创建Outbox事件处理器（使用默认配置）
func NewOutboxProcessor(outboxManager *OutboxManager, utxoClient *utxo.UTXOService, logger log.Logger) *OutboxProcessor {
	return &OutboxProcessor{
		outboxManager: outboxManager,
		utxoClient:    utxoClient,
		logger:        logger,
		maxRetries:    3,               // 最大重试次数
		retryDelay:    time.Second * 2, // 重试延迟
	}
}

// NewOutboxProcessorWithConfig 创建Outbox事件处理器（使用配置）
func NewOutboxProcessorWithConfig(outboxManager *OutboxManager, utxoClient *utxo.UTXOService, logger log.Logger, config *repositoryConfig.OutboxConfig) *OutboxProcessor {
	return &OutboxProcessor{
		outboxManager: outboxManager,
		utxoClient:    utxoClient,
		logger:        logger,
		maxRetries:    config.MaxRetries,
		retryDelay:    config.RetryDelay,
	}
}

// ProcessEvents 处理待处理的事件
func (op *OutboxProcessor) ProcessEvents(ctx context.Context) error {
	events, err := op.outboxManager.GetPendingEvents(ctx)
	if err != nil {
		return fmt.Errorf("获取待处理事件失败: %w", err)
	}

	if len(events) == 0 {
		return nil // 没有待处理事件
	}

	if op.logger != nil {
		op.logger.Debugf("开始处理outbox事件 - count: %d", len(events))
	}

	for _, event := range events {
		if err := op.processEvent(ctx, event); err != nil && op.logger != nil {
			op.logger.Errorf("处理outbox事件失败 - eventID: %s, error: %v", event.ID, err)
		}
	}

	return nil
}

// processEvent 处理单个事件
func (op *OutboxProcessor) processEvent(ctx context.Context, event *OutboxEvent) error {
	// 检查重试次数
	if event.Attempts >= op.maxRetries {
		if op.logger != nil {
			op.logger.Warnf("事件处理失败次数过多，跳过 - eventID: %s, attempts: %d", event.ID, event.Attempts)
		}
		return nil
	}

	// 标记为处理中
	if err := op.outboxManager.MarkEventProcessing(ctx, event.ID); err != nil {
		return fmt.Errorf("标记事件处理中失败: %w", err)
	}

	// 根据事件类型处理
	var err error
	switch event.Type {
	case EventTypeBlockAdded:
		err = op.processBlockAddedEvent(ctx, event)
	case EventTypeBlockRemoved:
		err = op.processBlockRemovedEvent(ctx, event)
	default:
		err = fmt.Errorf("未知的事件类型: %s", event.Type)
	}

	// 更新事件状态
	if err != nil {
		if markErr := op.outboxManager.MarkEventFailed(ctx, event.ID, err.Error()); markErr != nil && op.logger != nil {
			op.logger.Errorf("标记事件失败状态失败: %v", markErr)
		}
		return err
	}

	// 标记为已完成
	if err := op.outboxManager.MarkEventCompleted(ctx, event.ID); err != nil && op.logger != nil {
		op.logger.Errorf("标记事件完成状态失败: %v", err)
	}

	return nil
}

// processBlockAddedEvent 处理区块添加事件
func (op *OutboxProcessor) processBlockAddedEvent(ctx context.Context, event *OutboxEvent) error {
	// 🔧 修复：直接从新的BlockData字段获取protobuf数据
	if len(event.BlockData) == 0 {
		return fmt.Errorf("事件中缺少block数据")
	}

	// 直接从protobuf字节数据反序列化Block
	var block core.Block
	if err := proto.Unmarshal(event.BlockData, &block); err != nil {
		return fmt.Errorf("反序列化Block数据失败: %w", err)
	}

	// 通知UTXO系统
	if op.utxoClient != nil {
		return op.utxoClient.NotifyBlockAdded(ctx, &block)
	}

	return nil
}

// processBlockRemovedEvent 处理区块移除事件
func (op *OutboxProcessor) processBlockRemovedEvent(ctx context.Context, event *OutboxEvent) error {
	// 🔧 修复：直接从新的BlockData字段获取protobuf数据
	if len(event.BlockData) == 0 {
		return fmt.Errorf("事件中缺少block数据")
	}

	// 直接从protobuf字节数据反序列化Block
	var block core.Block
	if err := proto.Unmarshal(event.BlockData, &block); err != nil {
		return fmt.Errorf("反序列化Block数据失败: %w", err)
	}

	// 通知UTXO系统
	if op.utxoClient != nil {
		return op.utxoClient.NotifyBlockRemoved(ctx, &block)
	}

	return nil
}
