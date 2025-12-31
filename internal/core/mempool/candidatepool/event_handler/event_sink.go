// Package event_handler 候选区块池事件发布下沉
//
// 本文件实现候选区块池的事件发布下沉（Event Sink），负责将 CandidatePool 的内部事件
// 转换为标准化的事件总线消息并发布。
//
// 职责：
// - 实现 CandidateEventSink 接口
// - 将本地事件转换为全局事件常量并发布到事件总线
// - 确保事件发布的类型安全和标准化
package event_handler

import (
	candidatepool "github.com/weisyn/v1/internal/core/mempool/candidatepool"
	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/types"
)

// candidateSink 是 CandidatePool 的事件下沉实现。
// 作用：将候选区块相关本地事件转换为标准化的事件总线消息。
// 线程安全：事件总线接口自身应保证并发安全；本实现不持有可变共享状态。
type candidateSink struct {
	eventBus event.EventBus
	logger   log.Logger
}

// OnCandidateAdded 候选区块添加事件回调。
// 参数：
// - c：候选区块。
// 返回：无。
func (s *candidateSink) OnCandidateAdded(c *types.CandidateBlock) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventconstants.EventTypeCandidateAdded, c)
	}
}

// OnCandidateRemoved 候选区块移除事件回调。
// 参数：
// - c：候选区块；
// - reason：移除原因。
// 返回：无。
func (s *candidateSink) OnCandidateRemoved(c *types.CandidateBlock, reason string) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventconstants.EventTypeCandidateRemoved, &struct {
			Candidate *types.CandidateBlock
			Reason    string
		}{c, reason})
	}
}

// OnCandidateExpired 候选区块过期事件回调。
// 参数：
// - c：候选区块。
// 返回：无。
func (s *candidateSink) OnCandidateExpired(c *types.CandidateBlock) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventconstants.EventTypeCandidateExpired, c)
	}
}

// OnPoolCleared 候选池清空事件回调。
// 参数：
// - count：清空数量。
// 返回：无。
func (s *candidateSink) OnPoolCleared(count int) {
	if s.eventBus != nil {
		s.eventBus.Publish(eventconstants.EventTypeCandidatePoolCleared, count)
	}
}

// OnCleanupCompleted 清理任务完成事件回调。
// 参数：无。
// 返回：无。
func (s *candidateSink) OnCleanupCompleted() {
	if s.eventBus != nil {
		s.eventBus.Publish(eventconstants.EventTypeCandidateCleanupCompleted, struct{}{})
	}
}

// SetupCandidatePoolEventSink 设置候选区块池事件发布下沉。
// 将事件发布实现注入到 CandidatePool 中，使它们能够发布事件到事件总线。
//
// 参数：
// - eventBus：事件总线接口（可选，nil 时事件发布将被禁用）
// - logger：日志接口（可选）
// - candidatePool：候选区块池接口
//
// 说明：
// - 如果 eventBus 为 nil，事件发布将被禁用（池会使用 Noop 实现）
// - 使用类型断言确保类型安全
func SetupCandidatePoolEventSink(
	eventBus event.EventBus,
	logger log.Logger,
	candidatePool mempoolIfaces.CandidatePool,
) {
	// 注入 CandidatePool 事件下沉
	if cp, ok := candidatePool.(*candidatepool.CandidatePool); ok {
		cp.SetEventSink(&candidateSink{eventBus: eventBus, logger: logger})
		if logger != nil {
			logger.Debug("✅ CandidatePool 事件发布下沉已配置")
		}
	}
}

// 编译期检查：确保 candidateSink 实现了 CandidateEventSink 接口
var _ candidatepool.CandidateEventSink = (*candidateSink)(nil)

