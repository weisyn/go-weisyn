// 文件说明：
// 本文件用于装配内存池组件的“出站事件”集成（outgoing）。
// 职责：集中管理 mempool 的事件对外发布，将 TxPool/CandidatePool 的本地事件
// 通过事件总线统一命名、统一载荷形式对外发送，作为跨组件事件的标准范本。
package outgoing

import (
	"context"

	candidatepool "github.com/weisyn/v1/internal/core/mempool/candidatepool"
	txpool "github.com/weisyn/v1/internal/core/mempool/txpool"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	mempoolIfaces "github.com/weisyn/v1/pkg/interfaces/mempool"
	"github.com/weisyn/v1/pkg/types"
	"go.uber.org/fx"
)

// txSink 是 TxPool 的事件下沉实现。
// 作用：将交易相关本地事件转换为标准化的事件总线消息。
// 线程安全：事件总线接口自身应保证并发安全；本实现不持有可变共享状态。
type txSink struct {
	eventBus event.EventBus
	logger   log.Logger
}

// OnTxAdded 交易添加事件回调。
// 参数：
// - tx：交易包装器。
// 返回：无。
func (s *txSink) OnTxAdded(tx *txpool.TxWrapper) {
	if s.eventBus != nil {
		s.eventBus.Publish(event.EventType("mempool.tx.added.v1"), tx)
	}
}

// OnTxRemoved 交易移除事件回调。
// 参数：
// - tx：交易包装器。
// 返回：无。
func (s *txSink) OnTxRemoved(tx *txpool.TxWrapper) {
	if s.eventBus != nil {
		s.eventBus.Publish(event.EventType("mempool.tx.removed.v1"), tx)
	}
}

// OnTxConfirmed 交易确认事件回调。
// 参数：
// - tx：交易包装器；
// - h：确认区块高度。
// 返回：无。
func (s *txSink) OnTxConfirmed(tx *txpool.TxWrapper, h uint64) {
	if s.eventBus == nil {
		return
	}
	s.eventBus.Publish(event.EventType("mempool.tx.confirmed.v1"), &struct {
		Tx          *txpool.TxWrapper
		BlockHeight uint64
	}{Tx: tx, BlockHeight: h})
}

// candidateSink 是 CandidatePool 的事件下沉实现。
// 作用：将候选区块相关本地事件转换为标准化的事件总线消息。
// 线程安全：同上。
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
		s.eventBus.Publish(event.EventType("mempool.candidate.added.v1"), c)
	}
}

// OnCandidateRemoved 候选区块移除事件回调。
// 参数：
// - c：候选区块；
// - reason：移除原因。
// 返回：无。
func (s *candidateSink) OnCandidateRemoved(c *types.CandidateBlock, reason string) {
	if s.eventBus != nil {
		s.eventBus.Publish(event.EventType("mempool.candidate.removed.v1"), &struct {
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
		s.eventBus.Publish(event.EventType("mempool.candidate.expired.v1"), c)
	}
}

// OnPoolCleared 候选池清空事件回调。
// 参数：
// - count：清空数量。
// 返回：无。
func (s *candidateSink) OnPoolCleared(count int) {
	if s.eventBus != nil {
		s.eventBus.Publish(event.EventType("mempool.candidate.pool_cleared.v1"), count)
	}
}

// OnCleanupCompleted 清理任务完成事件回调。
// 参数：无。
// 返回：无。
func (s *candidateSink) OnCleanupCompleted() {
	if s.eventBus != nil {
		s.eventBus.Publish(event.EventType("mempool.candidate.cleanup_completed.v1"), struct{}{})
	}
}

// params 定义 Fx 注入参数。
// 字段说明：
// - EventBus：事件总线；
// - Logger：日志；
// - Pool：TxPool 接口（用于注入事件下沉）；
// - Candi：CandidatePool 接口（用于注入事件下沉）。
type params struct {
	fx.In
	EventBus       event.EventBus `optional:"true"`
	Logger         log.Logger     `optional:"true"`
	Lifecycle      fx.Lifecycle
	ExtendedTxPool txpool.ExtendedTxPool
	Candi          mempoolIfaces.CandidatePool `name:"candidate_pool"`
}

// SetupEventOutgoing 装配出站事件下沉。
// 参数：
// - p：包含事件总线、日志、池实例的 Fx 注入参数。
// 返回：
// - error：始终返回 nil。
// 逻辑：
// - 将下沉实现注入到 TxPool 与 CandidatePool；
// - 注册生命周期钩子（当前为空）。
func SetupEventOutgoing(p params) error {
	// 注入 TxPool 事件下沉
	if pool, ok := p.ExtendedTxPool.(*txpool.TxPool); ok {
		pool.SetEventSink(&txSink{eventBus: p.EventBus, logger: p.Logger})
	}
	// 注入 CandidatePool 事件下沉（断言为内部实现）
	if cp, ok := p.Candi.(*candidatepool.CandidatePool); ok {
		cp.SetEventSink(&candidateSink{eventBus: p.EventBus, logger: p.Logger})
	}
	p.Lifecycle.Append(fx.Hook{OnStart: func(ctx context.Context) error { return nil }, OnStop: func(ctx context.Context) error { return nil }})
	return nil
}
