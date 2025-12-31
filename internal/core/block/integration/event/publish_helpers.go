// Package event 提供 Block 模块的事件集成
//
// 🎯 **事件发布帮助函数**
//
// 本文件提供了便捷的事件发布函数，用于在区块处理完成后发布事件。
package event

import (
	"context"

	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"google.golang.org/protobuf/proto"
)

// PublishBlockProcessedEvent 发布区块处理完成事件
//
// 🎯 **出站事件**：EventTypeBlockProcessed
//
// 用途：
// - 通知 Chain 模块更新链尖状态
// - 通知其他订阅者区块已成功处理
//
// 参数：
//   - ctx: 上下文
//   - eventBus: 事件总线
//   - logger: 日志记录器
//   - block: 已处理的区块
//   - blockHash: 区块哈希（32字节）
//
// 返回：
//   - error: 发布错误
func PublishBlockProcessedEvent(
	ctx context.Context,
	eventBus event.EventBus,
	logger log.Logger,
	block *core.Block,
	blockHash []byte,
) error {
	if eventBus == nil {
		if logger != nil {
			logger.Debug("EventBus不可用，跳过事件发布")
		}
		return nil
	}

	// 检查区块是否为 nil
	if block == nil {
		if logger != nil {
			logger.Warn("区块为nil，跳过事件发布")
		}
		return nil
	}

	// 检查区块头和区块体是否为 nil
	if block.Header == nil || block.Body == nil {
		if logger != nil {
			logger.Warn("区块头或区块体为nil，跳过事件发布")
		}
		return nil
	}

	// 创建事件数据
	eventData := &types.BlockProcessedEventData{
		Height:           block.Header.Height,
		Hash:             string(blockHash), // 使用实际区块哈希
		ParentHash:       string(block.Header.PreviousHash),
		StateRoot:        string(block.Header.StateRoot),
		TxCount:          len(block.Body.Transactions),
		TransactionCount: len(block.Body.Transactions),
		Timestamp:        int64(block.Header.Timestamp),
		Size:             int64(proto.Size(block)), // 计算区块大小
	}

	// 发布事件（EventBus.Publish 无返回值）
	// ⚠️ 注意：订阅者期望 (ctx context.Context, data interface{}) 两个参数
	eventBus.Publish(eventconstants.EventTypeBlockProcessed, ctx, eventData)

	if logger != nil {
		logger.Debugf("✅ 已发布BlockProcessed事件，高度: %d",
			block.Header.Height)
	}

	return nil
}

// PublishForkDetectedEvent 发布分叉检测事件
//
// 🎯 **出站事件**：EventTypeForkDetected
//
// 用途：
// - 通知 Chain/Fork 模块处理分叉
//
// 参数：
//   - ctx: 上下文
//   - eventBus: 事件总线
//   - logger: 日志记录器
//   - forkBlock: 分叉区块
//   - localBlockHash: 本地区块哈希（32字节）
//   - forkBlockHash: 分叉区块哈希（32字节）
//
// 返回：
//   - error: 发布错误
func PublishForkDetectedEvent(
	ctx context.Context,
	eventBus event.EventBus,
	logger log.Logger,
	forkBlock *core.Block,
	localBlockHash []byte,
	forkBlockHash []byte,
) error {
	if eventBus == nil {
		if logger != nil {
			logger.Debug("EventBus不可用，跳过事件发布")
		}
		return nil
	}

	// 检查分叉区块是否为 nil
	if forkBlock == nil {
		if logger != nil {
			logger.Warn("分叉区块为nil，跳过事件发布")
		}
		return nil
	}

	// 检查区块头是否为 nil
	if forkBlock.Header == nil {
		if logger != nil {
			logger.Warn("分叉区块头为nil，跳过事件发布")
		}
		return nil
	}

	// 创建事件数据
	eventData := &types.ForkDetectedEventData{
		Height:         forkBlock.Header.Height,
		ForkHeight:     forkBlock.Header.Height,
		LocalBlockHash: string(localBlockHash), // 使用实际本地区块哈希
		ForkBlockHash:  string(forkBlockHash), // 使用实际分叉区块哈希
		ConflictType:   "block_hash",
		ForkType:       "block_hash",
		Source:         "validation",
		DetectedAt:     int64(forkBlock.Header.Timestamp),
		Message:        "检测到分叉区块",
	}

	// 发布事件（EventBus.Publish 无返回值）
	// ⚠️ 注意：订阅者期望 (ctx context.Context, data interface{}) 两个参数
	eventBus.Publish(eventconstants.EventTypeForkDetected, ctx, eventData)

	if logger != nil {
		logger.Warnf("⚠️ 已发布ForkDetected事件，分叉区块高度: %d",
			forkBlock.Header.Height)
	}

	return nil
}

