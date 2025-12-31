// Package eventhelpers 提供 UTXO 事件发布帮助函数
//
// 🎯 **UTXO 事件发布**
//
// 本文件提供了便捷的 UTXO 事件发布函数，用于发布 UTXO 相关的各种事件。
package eventhelpers

import (
	"context"
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// UTXO 事件类型常量
const (
	EventTypeUTXOCreated           event.EventType = "utxo.created"
	EventTypeUTXODeleted           event.EventType = "utxo.deleted"
	EventTypeUTXOStateRootUpdated  event.EventType = "utxo.state_root.updated"
	EventTypeUTXOReferenced        event.EventType = "utxo.referenced"
	EventTypeUTXOUnreferenced      event.EventType = "utxo.unreferenced"
)

// PublishUTXOCreatedEvent 发布 UTXO 创建事件
//
// 参数：
//   - ctx: 上下文
//   - eventBus: 事件总线
//   - logger: 日志记录器
//   - utxoObj: UTXO 对象
//
// 返回：
//   - error: 发布错误
func PublishUTXOCreatedEvent(
	ctx context.Context,
	eventBus event.EventBus,
	logger log.Logger,
	utxoObj *utxo.UTXO,
) error {
	if eventBus == nil {
		return nil
	}

	// 构造事件数据
	eventData := &types.UTXOStateChangedEventData{
		UTXOHash:    fmt.Sprintf("%x:%d", utxoObj.Outpoint.TxId, utxoObj.Outpoint.OutputIndex),
		Operation:   "created",
		TxHash:      fmt.Sprintf("%x", utxoObj.Outpoint.TxId),
		BlockHeight: 0, // UTXO创建时可能还没有区块高度
		Timestamp:   time.Now().Unix(),
	}

	// 发布事件
	eventBus.Publish(EventTypeUTXOCreated, eventData)

	if logger != nil {
		logger.Debugf("✅ 已发布 UTXO 创建事件: %s", eventData.UTXOHash)
	}

	return nil
}

// PublishUTXODeletedEvent 发布 UTXO 删除事件
//
// 参数：
//   - ctx: 上下文
//   - eventBus: 事件总线
//   - logger: 日志记录器
//   - outpoint: OutPoint
//
// 返回：
//   - error: 发布错误
func PublishUTXODeletedEvent(
	ctx context.Context,
	eventBus event.EventBus,
	logger log.Logger,
	outpoint *transaction.OutPoint,
) error {
	if eventBus == nil {
		return nil
	}

	// 构造事件数据
	eventData := &types.UTXOStateChangedEventData{
		UTXOHash:    fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex),
		Operation:   "deleted",
		TxHash:      fmt.Sprintf("%x", outpoint.TxId),
		BlockHeight: 0, // UTXO删除时可能还没有区块高度
		Timestamp:   time.Now().Unix(),
	}

	// 发布事件
	eventBus.Publish(EventTypeUTXODeleted, eventData)

	if logger != nil {
		logger.Debugf("✅ 已发布 UTXO 删除事件: %s", eventData.UTXOHash)
	}

	return nil
}

// PublishUTXOStateRootUpdatedEvent 发布 UTXO 状态根更新事件
//
// 参数：
//   - ctx: 上下文
//   - eventBus: 事件总线
//   - logger: 日志记录器
//   - stateRoot: 状态根（32字节）
//
// 返回：
//   - error: 发布错误
func PublishUTXOStateRootUpdatedEvent(
	ctx context.Context,
	eventBus event.EventBus,
	logger log.Logger,
	stateRoot []byte,
) error {
	if eventBus == nil {
		return nil
	}

	// 构造事件数据（使用简单的map结构，因为UTXOStateChangedEventData不适合状态根更新）
	eventData := map[string]interface{}{
		"state_root": fmt.Sprintf("%x", stateRoot),
		"timestamp":  time.Now().Unix(),
	}

	// 发布事件
	eventBus.Publish(EventTypeUTXOStateRootUpdated, eventData)

	if logger != nil {
		logger.Debugf("✅ 已发布 UTXO 状态根更新事件: %x", stateRoot)
	}

	return nil
}

// PublishUTXOReferencedEvent 发布 UTXO 引用事件
//
// 参数：
//   - ctx: 上下文
//   - eventBus: 事件总线
//   - logger: 日志记录器
//   - outpoint: OutPoint
//   - refCount: 引用计数
//
// 返回：
//   - error: 发布错误
func PublishUTXOReferencedEvent(
	ctx context.Context,
	eventBus event.EventBus,
	logger log.Logger,
	outpoint *transaction.OutPoint,
	refCount uint64,
) error {
	if eventBus == nil {
		return nil
	}

	// 构造事件数据
	eventData := map[string]interface{}{
		"utxo_hash":  fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex),
		"tx_hash":    fmt.Sprintf("%x", outpoint.TxId),
		"ref_count":  refCount,
		"operation":  "referenced",
		"timestamp":  time.Now().Unix(),
	}

	// 发布事件
	eventBus.Publish(EventTypeUTXOReferenced, eventData)

	if logger != nil {
		logger.Debugf("✅ 已发布 UTXO 引用事件: %x:%d, ref_count=%d", outpoint.TxId, outpoint.OutputIndex, refCount)
	}

	return nil
}

// PublishUTXOUnreferencedEvent 发布 UTXO 解除引用事件
//
// 参数：
//   - ctx: 上下文
//   - eventBus: 事件总线
//   - logger: 日志记录器
//   - outpoint: OutPoint
//   - refCount: 引用计数
//
// 返回：
//   - error: 发布错误
func PublishUTXOUnreferencedEvent(
	ctx context.Context,
	eventBus event.EventBus,
	logger log.Logger,
	outpoint *transaction.OutPoint,
	refCount uint64,
) error {
	if eventBus == nil {
		return nil
	}

	// 构造事件数据
	eventData := map[string]interface{}{
		"utxo_hash":  fmt.Sprintf("%x:%d", outpoint.TxId, outpoint.OutputIndex),
		"tx_hash":    fmt.Sprintf("%x", outpoint.TxId),
		"ref_count":  refCount,
		"operation":  "unreferenced",
		"timestamp":  time.Now().Unix(),
	}

	// 发布事件
	eventBus.Publish(EventTypeUTXOUnreferenced, eventData)

	if logger != nil {
		logger.Debugf("✅ 已发布 UTXO 解除引用事件: %x:%d, ref_count=%d", outpoint.TxId, outpoint.OutputIndex, refCount)
	}

	return nil
}

