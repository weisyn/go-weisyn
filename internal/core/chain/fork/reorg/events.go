package reorg

import (
	"context"
	"time"

	eventconstants "github.com/weisyn/v1/pkg/constants/events"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/types"
)

// EventPublisher 封装 REORG 阶段事件发布逻辑
type EventPublisher struct {
	eventBus event.EventBus
}

// NewEventPublisher 创建事件发布器
func NewEventPublisher(eventBus event.EventBus) *EventPublisher {
	return &EventPublisher{eventBus: eventBus}
}

// PublishPhaseStarted 发布阶段开始事件
func (p *EventPublisher) PublishPhaseStarted(ctx context.Context, session *ReorgSession, phase Phase) {
	if p.eventBus == nil {
		return
	}

	eventData := &types.ReorgPhaseEventData{
		SessionID:  session.ID,
		Phase:      string(phase),
		Status:     "started",
		FromHeight: session.FromHeight,
		ForkHeight: session.ForkHeight,
		ToHeight:   session.ToHeight,
		Timestamp:  time.Now().UnixMilli(),
	}

	var eventType eventconstants.EventType
	switch phase {
	case PhasePrepare:
		eventType = eventconstants.EventTypeReorgPrepareStarted
	case PhaseRollback:
		eventType = eventconstants.EventTypeReorgRollbackStarted
	case PhaseReplay:
		eventType = eventconstants.EventTypeReorgReplayStarted
	case PhaseVerify:
		eventType = eventconstants.EventTypeReorgVerifyStarted
	case PhaseCommit:
		eventType = eventconstants.EventTypeReorgCommitStarted
	default:
		return
	}

	p.eventBus.Publish(eventType, ctx, eventData)
}

// PublishPhaseCompleted 发布阶段完成事件
func (p *EventPublisher) PublishPhaseCompleted(ctx context.Context, session *ReorgSession, phase Phase, duration time.Duration) {
	if p.eventBus == nil {
		return
	}

	eventData := &types.ReorgPhaseEventData{
		SessionID:  session.ID,
		Phase:      string(phase),
		Status:     "completed",
		FromHeight: session.FromHeight,
		ForkHeight: session.ForkHeight,
		ToHeight:   session.ToHeight,
		Timestamp:  time.Now().UnixMilli(),
		Duration:   duration.Milliseconds(),
	}

	var eventType eventconstants.EventType
	switch phase {
	case PhasePrepare:
		eventType = eventconstants.EventTypeReorgPrepareCompleted
	case PhaseRollback:
		eventType = eventconstants.EventTypeReorgRollbackCompleted
	case PhaseReplay:
		eventType = eventconstants.EventTypeReorgReplayCompleted
	case PhaseVerify:
		eventType = eventconstants.EventTypeReorgVerifyCompleted
	case PhaseCommit:
		eventType = eventconstants.EventTypeReorgCommitCompleted
	default:
		return
	}

	p.eventBus.Publish(eventType, ctx, eventData)
}

// PublishForkCompleted 发布分叉处理完成事件（兼容现有事件）
func (p *EventPublisher) PublishForkCompleted(ctx context.Context, session *ReorgSession, duration time.Duration) {
	if p.eventBus == nil {
		return
	}

	eventData := &types.ForkCompletedEventData{
		ProcessID:      session.ID,
		Resolution:     "remote_adopted",
		CompletedAt:    time.Now().UnixMilli(),
		Duration:       duration.Milliseconds(),
		FinalHeight:    session.ToHeight,
		RevertedBlocks: int(session.FromHeight - session.ForkHeight),
		AppliedBlocks:  int(session.ToHeight - session.ForkHeight),
		Success:        true,
		ChainSwitched:  true,
		ProcessingTime: duration.Milliseconds(),
	}

	p.eventBus.Publish(eventconstants.EventTypeForkCompleted, ctx, eventData)
}

// PublishForkFailed 发布分叉处理失败事件
func (p *EventPublisher) PublishForkFailed(ctx context.Context, session *ReorgSession, reorgErr *ReorgError, duration time.Duration) {
	if p.eventBus == nil {
		return
	}

	eventData := &types.ForkFailedEventData{
		ProcessID:  session.ID,
		FailedAt:   time.Now().UnixMilli(),
		Duration:   duration.Milliseconds(),
		FailPhase:  string(reorgErr.Phase),
		ErrorClass: string(reorgErr.Class),
		Error:      reorgErr.Err.Error(),
		FromHeight: session.FromHeight,
		ForkHeight: session.ForkHeight,
		ToHeight:   session.ToHeight,
		// Recoverable 和 ReadOnlyMode 由调用方决定
	}

	p.eventBus.Publish(eventconstants.EventTypeForkFailed, ctx, eventData)
}

// PublishReorgAborted 发布 REORG 中止事件
func (p *EventPublisher) PublishReorgAborted(ctx context.Context, session *ReorgSession, abortReason error, failPhase Phase, abortSuccess bool, abortError error) {
	if p.eventBus == nil {
		return
	}

	eventData := &types.ReorgAbortedEventData{
		SessionID:    session.ID,
		AbortReason:  abortReason.Error(),
		FailPhase:    string(failPhase),
		FromHeight:   session.FromHeight,
		ForkHeight:   session.ForkHeight,
		ToHeight:     session.ToHeight,
		AbortedAt:    time.Now().UnixMilli(),
		RecoveryMode: "rollback_to_origin",
		Success:      abortSuccess,
	}

	if !abortSuccess && abortError != nil {
		eventData.Error = abortError.Error()
		eventData.RecoveryMode = "enter_readonly"
	}

	p.eventBus.Publish(eventconstants.EventTypeReorgAborted, ctx, eventData)
}

// PublishReorgCompensation 发布 REORG 补偿事件
func (p *EventPublisher) PublishReorgCompensation(ctx context.Context, session *ReorgSession, utxoRestored, indicesRolledBack int, success bool, err error) {
	if p.eventBus == nil {
		return
	}

	eventData := &types.ReorgCompensationEventData{
		SessionID:        session.ID,
		CompensationType: "full_rollback",
		FromHeight:       session.FromHeight,
		RestoredHeight:   session.FromHeight,
		CompletedAt:      time.Now().UnixMilli(),
		Success:          success,
		AffectedModules:  []string{"utxo", "index", "chain_state"},
		UTXORestored:     utxoRestored,
		IndicesRolledBack: indicesRolledBack,
	}

	if !success && err != nil {
		eventData.Error = err.Error()
	}

	p.eventBus.Publish(eventconstants.EventTypeReorgCompensation, ctx, eventData)
}

