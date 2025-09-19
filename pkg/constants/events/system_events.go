// Package events 提供WES系统核心事件类型常量定义
//
// 🎯 **核心事件常量归口管理**
//
// 本文件只定义3个核心组件的跨组件事件类型：
// - blockchain: 区块链状态、分叉检测、交易确认
// - consensus: 共识结果、状态变化
// - mempool: 交易池变化、候选区块管理
//
// 🔧 **设计原则**
// - 简单至上：只保留真正需要跨组件通信的事件
// - 命名规范：domain.category.action 格式
// - 高内聚低耦合：避免不必要的事件依赖
//
// 🏗️ **使用方式**
// ```go
// import "github.com/weisyn/v1/pkg/constants/events"
//
// // 跨组件订阅
// eventBus.Subscribe(events.EventTypeChainReorganized, handler)
//
// // 跨组件发布
// eventBus.Publish(events.EventTypeForkDetected, eventData)
// ```
package events

import (
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
)

// ============================================================================
//                           核心事件类型定义
// ============================================================================

// EventType 全局事件类型别名，兼容标准事件接口
type EventType = event.EventType

// ============================================================================
//                           系统级事件（跨组件）
// ============================================================================

// 系统生命周期事件
const (
	// EventTypeSystemStarted 系统启动完成事件
	// 发布者：main/application启动器
	// 订阅者：所有需要系统启动通知的组件
	EventTypeSystemStarted EventType = "system.lifecycle.started"

	// EventTypeSystemStopping 系统即将停止事件
	// 发布者：main/application关闭器
	// 订阅者：所有需要优雅停止的组件
	EventTypeSystemStopping EventType = "system.lifecycle.stopping"

	// EventTypeSystemStopped 系统已停止事件
	EventTypeSystemStopped EventType = "system.lifecycle.stopped"

	// EventTypeResourceExhausted 资源耗尽事件
	EventTypeResourceExhausted EventType = "system.resource.exhausted"

	// EventTypeStorageSpaceLow 存储空间不足事件
	EventTypeStorageSpaceLow EventType = "system.storage.space_low"
)

// 区块链核心事件（blockchain模块跨组件事件）
const (
	// EventTypeChainReorganized 链重组事件
	// 发布者：blockchain组件
	// 订阅者：consensus（调整聚合状态）、mempool（清理无效交易）、其他依赖链状态的组件
	EventTypeChainReorganized EventType = "blockchain.chain.reorganized"

	// EventTypeForkDetected 分叉检测事件
	// 发布者：blockchain/sync组件
	// 订阅者：consensus（停止当前挖矿）、mempool（暂停交易处理）
	EventTypeForkDetected EventType = "blockchain.fork.detected"

	// EventTypeForkProcessing 分叉处理中事件
	// 发布者：blockchain/fork组件
	// 订阅者：consensus（等待处理完成）
	EventTypeForkProcessing EventType = "blockchain.fork.processing"

	// EventTypeForkCompleted 分叉处理完成事件
	// 发布者：blockchain/fork组件
	// 订阅者：consensus（恢复正常操作）、mempool（重新验证交易）
	EventTypeForkCompleted EventType = "blockchain.fork.completed"

	// EventTypeChainHeightChanged 链高度变化事件
	EventTypeChainHeightChanged EventType = "blockchain.chain.height_changed"

	// 区块事件
	EventTypeBlockProduced  EventType = "blockchain.block.produced"  // 区块生产完成
	EventTypeBlockValidated EventType = "blockchain.block.validated" // 区块验证完成
	EventTypeBlockProcessed EventType = "blockchain.block.processed" // 区块处理完成
	EventTypeBlockConfirmed EventType = "blockchain.block.confirmed" // 区块确认
	EventTypeBlockReverted  EventType = "blockchain.block.reverted"  // 区块回滚
	EventTypeBlockFinalized EventType = "blockchain.block.finalized" // 区块最终确认

	// 链状态事件
	EventTypeChainStateUpdated EventType = "blockchain.chain.state_updated" // 链状态更新

	// 交易事件
	EventTypeTransactionReceived  EventType = "blockchain.transaction.received"  // 交易接收
	EventTypeTransactionValidated EventType = "blockchain.transaction.validated" // 交易验证完成
	EventTypeTransactionExecuted  EventType = "blockchain.transaction.executed"  // 交易执行完成
	EventTypeTransactionFailed    EventType = "blockchain.transaction.failed"    // 交易执行失败
	EventTypeTransactionConfirmed EventType = "blockchain.transaction.confirmed" // 交易确认

	// 同步事件
	EventTypeSyncStarted   EventType = "blockchain.sync.started"   // 同步开始
	EventTypeSyncProgress  EventType = "blockchain.sync.progress"  // 同步进度更新
	EventTypeSyncCompleted EventType = "blockchain.sync.completed" // 同步完成
	EventTypeSyncFailed    EventType = "blockchain.sync.failed"    // 同步失败
)

// 网络核心事件（network模块基础事件）
const (
	// EventTypeNetworkQualityChanged 网络质量变化事件
	// 发布者：network组件
	// 订阅者：consensus（调整超时策略）、blockchain（调整同步策略）
	EventTypeNetworkQualityChanged EventType = "network.quality.changed"

	// EventTypeNetworkPartitioned 网络分区检测事件
	// 发布者：network组件
	// 订阅者：consensus（进入安全模式）
	EventTypeNetworkPartitioned EventType = "network.partition.detected"

	// EventTypeNetworkRecovered 网络分区恢复事件
	EventTypeNetworkRecovered EventType = "network.partition.recovered"
)

// 共识核心事件（consensus模块跨组件事件）
const (
	// EventTypeConsensusResultBroadcast 共识结果广播事件
	// 发布者：consensus/aggregator组件
	// 订阅者：blockchain（应用共识结果）、mempool（更新交易状态）
	EventTypeConsensusResultBroadcast EventType = "consensus.result.broadcast"

	// EventTypeConsensusStateChanged 共识状态变化事件
	// 发布者：consensus组件
	// 订阅者：监控组件、状态依赖的其他组件
	EventTypeConsensusStateChanged EventType = "consensus.state.changed"
)

// 内存池事件（mempool模块事件）
const (
	// ========== 交易池事件 ==========

	// EventTypeTxAdded 交易添加到池事件
	// 发布者：mempool组件（txpool）
	// 订阅者：consensus（通知有新交易）、network（广播交易）
	EventTypeTxAdded EventType = "mempool.tx.added"

	// EventTypeTxRemoved 交易从池移除事件
	// 发布者：mempool组件（txpool）
	// 订阅者：network（停止广播）、监控组件
	EventTypeTxRemoved EventType = "mempool.tx.removed"

	// EventTypeTxConfirmed 交易确认事件
	// 发布者：mempool组件（txpool）
	// 订阅者：监控组件、用户接口
	EventTypeTxConfirmed EventType = "mempool.tx.confirmed"

	// 交易池管理事件
	EventTypeTxExpired     EventType = "mempool.tx.expired"
	EventTypeTxPoolFull    EventType = "mempool.tx.pool_full"
	EventTypeTxPoolCleared EventType = "mempool.tx.pool_cleared"

	// ========== 候选区块池事件 ==========

	// EventTypeCandidateAdded 候选区块添加事件
	// 发布者：mempool组件（candidatepool）
	// 订阅者：consensus（处理候选区块）
	EventTypeCandidateAdded EventType = "mempool.candidate.added"

	// EventTypeCandidateRemoved 候选区块移除事件
	// 发布者：mempool组件（candidatepool）
	// 订阅者：consensus（更新处理状态）
	EventTypeCandidateRemoved EventType = "mempool.candidate.removed"

	// EventTypeCandidateExpired 候选区块过期事件
	// 发布者：mempool组件（candidatepool）
	// 订阅者：consensus（清理过期候选）
	EventTypeCandidateExpired EventType = "mempool.candidate.expired"

	// 候选区块池管理事件
	EventTypeCandidatePoolFull EventType = "mempool.candidate.pool_full"

	// EventTypeCandidatePoolCleared 候选区块池清理事件
	// 发布者：mempool组件（candidatepool）
	// 订阅者：consensus（重置处理状态）
	EventTypeCandidatePoolCleared EventType = "mempool.candidate.pool_cleared"

	// EventTypeCandidateCleanupCompleted 候选区块清理完成事件
	// 发布者：mempool组件（candidatepool）
	// 订阅者：监控组件
	EventTypeCandidateCleanupCompleted EventType = "mempool.candidate.cleanup_completed"

	// ========== 内存池生命周期事件 ==========

	EventTypeMempoolStarted EventType = "mempool.lifecycle.started"

	EventTypeMempoolStopped EventType = "mempool.lifecycle.stopped"

	EventTypeMempoolSizeChanged EventType = "mempool.stats.size_changed"

	// EventTypeMempoolPressureHigh 内存池压力高事件
	EventTypeMempoolPressureHigh EventType = "mempool.performance.pressure_high"
)

// ============================================================================
//                           事件数据结构引用
// ============================================================================

// 事件数据结构统一定义在 pkg/types/event.go 中：
// - ChainReorganizedEventData
// - ForkDetectedEventData
// - ForkProcessingEventData
// - ForkCompletedEventData
// - NetworkQualityChangedEventData
// - ConsensusResultEventData
// - ConsensusStateChangedEventData
// - TransactionReceivedEventData
// - BlockProcessedEventData
// - MempoolEventData
//
// 使用示例：
// ```go
// import (
//     "github.com/weisyn/v1/pkg/constants/events"
//     "github.com/weisyn/v1/pkg/types"
// )
//
// // 发布事件
// eventData := &types.ChainReorganizedEventData{...}
// eventBus.Publish(events.EventTypeChainReorganized, eventData)
// ```

// ============================================================================
//                           核心事件类型列表
// ============================================================================

// SystemEvents 系统级事件列表（所有跨组件事件）
var SystemEvents = []EventType{
	// 系统生命周期
	EventTypeSystemStarted,
	EventTypeSystemStopping,
	EventTypeSystemStopped,

	// 区块链核心事件
	EventTypeChainReorganized,
	EventTypeForkDetected,
	EventTypeForkProcessing,
	EventTypeForkCompleted,
	EventTypeChainHeightChanged,
	EventTypeBlockProduced,
	EventTypeBlockProcessed,
	EventTypeTransactionConfirmed,

	// 网络核心事件
	EventTypeNetworkQualityChanged,
	EventTypeNetworkPartitioned,
	EventTypeNetworkRecovered,

	// 共识核心事件
	EventTypeConsensusResultBroadcast,
	EventTypeConsensusStateChanged,

	// 内存池核心事件
	EventTypeTxAdded,
	EventTypeTxRemoved,
	EventTypeCandidateAdded,
	EventTypeCandidateRemoved,
	EventTypeMempoolSizeChanged,
}

// GetEventCategory 获取事件分类
// 帮助组件判断事件的重要性和处理优先级
func GetEventCategory(eventType EventType) string {
	switch eventType {
	case EventTypeSystemStarted, EventTypeSystemStopping, EventTypeSystemStopped:
		return "system_lifecycle"
	case EventTypeChainReorganized, EventTypeForkDetected, EventTypeForkProcessing, EventTypeForkCompleted:
		return "blockchain_fork"
	case EventTypeChainHeightChanged, EventTypeBlockProduced, EventTypeBlockProcessed:
		return "blockchain_state"
	case EventTypeTransactionReceived, EventTypeTransactionValidated, EventTypeTransactionConfirmed:
		return "blockchain_transaction"
	case EventTypeNetworkQualityChanged, EventTypeNetworkPartitioned, EventTypeNetworkRecovered:
		return "network_topology"
	case EventTypeConsensusResultBroadcast, EventTypeConsensusStateChanged:
		return "consensus_coordination"
	case EventTypeTxAdded, EventTypeTxRemoved, EventTypeTxConfirmed:
		return "mempool_transaction"
	case EventTypeCandidateAdded, EventTypeCandidateRemoved, EventTypeCandidatePoolCleared:
		return "mempool_candidate"
	case EventTypeMempoolSizeChanged, EventTypeMempoolPressureHigh:
		return "mempool_management"
	default:
		return "unknown"
	}
}

// IsSystemCriticalEvent 判断是否为系统关键事件
// 关键事件需要优先处理，确保系统安全
func IsSystemCriticalEvent(eventType EventType) bool {
	criticalEvents := []EventType{
		// 系统级关键事件
		EventTypeSystemStopping,

		// 区块链关键事件
		EventTypeChainReorganized,
		EventTypeForkDetected,

		// 网络关键事件
		EventTypeNetworkPartitioned,

		// 共识关键事件
		EventTypeConsensusResultBroadcast,

		// 内存池关键事件
		EventTypeMempoolPressureHigh,
		EventTypeTxPoolFull,
		EventTypeCandidatePoolFull,

		// 资源关键事件
		EventTypeResourceExhausted,
		EventTypeStorageSpaceLow,
	}

	for _, critical := range criticalEvents {
		if eventType == critical {
			return true
		}
	}
	return false
}
