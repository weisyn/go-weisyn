// 文件说明：
// 本文件定义候选区块池的事件下沉接口 CandidateEventSink 及其默认空实现。
// 设计要点：候选池内部仅依赖该接口进行事件发布，真正的对外事件派发由
// integration/event/outgoing 层实现并注入，确保组件与事件总线解耦。
package candidatepool

import (
	"github.com/weisyn/v1/pkg/types"
)

// CandidateEventSink 候选区块池对外的事件下沉接口。
// 说明：由 integration 层提供实现，把本地事件转发到统一 EventBus。
// 若未注入则采用 Noop 实现，不对外发布事件。
type CandidateEventSink interface {
	// OnCandidateAdded 候选区块加入时触发。
	OnCandidateAdded(candidate *types.CandidateBlock)
	// OnCandidateRemoved 候选区块被移除时触发（reason 提供移除原因）。
	OnCandidateRemoved(candidate *types.CandidateBlock, reason string)
	// OnCandidateExpired 候选区块过期时触发。
	OnCandidateExpired(candidate *types.CandidateBlock)
	// OnPoolCleared 候选池清空时触发（count 为清理数量）。
	OnPoolCleared(count int)
	// OnCleanupCompleted 定期清理任务完成时触发。
	OnCleanupCompleted()
}

// NoopCandidateEventSink 默认空实现（不进行任何发布）。
type NoopCandidateEventSink struct{}

func (NoopCandidateEventSink) OnCandidateAdded(candidate *types.CandidateBlock)                  {}
func (NoopCandidateEventSink) OnCandidateRemoved(candidate *types.CandidateBlock, reason string) {}
func (NoopCandidateEventSink) OnCandidateExpired(candidate *types.CandidateBlock)                {}
func (NoopCandidateEventSink) OnPoolCleared(count int)                                           {}
func (NoopCandidateEventSink) OnCleanupCompleted()                                               {}
