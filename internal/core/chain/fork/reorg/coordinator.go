package reorg

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
)

// Coordinator 是生产级 REORG 协调器（将流程收口为 Begin/Execute/Commit/Abort）。
//
// 说明：
// - 本实现不提供"向后兼容"路径；所有 REORG 必须走 Coordinator。
// - 单事务原子回滚与深度验证会在后续 todos 中逐步强化，但接口与阶段语义在此处固定。
type Coordinator struct {
	logger       log.Logger
	queryService persistence.QueryService
	blockProc    block.BlockProcessor

	// managers
	snapshotMgr Reversible
	indexMgr    Reversible

	// verifier：暂以函数形式注入，后续替换为严格版 RollbackValidator（deep-verification-impl）
	verifyFn func(ctx context.Context, expectedHeight uint64) (*VerificationResult, error)

	// atomicRollbackFn：严格原子化 Phase2（单事务完成 index 删除 + UTXO 恢复 + tip/root 更新）
	atomicRollbackFn func(ctx context.Context, session *ReorgSession) error

	// abort hook：进入只读/停写（后续 todo 替换为全局 write gate）
	enterReadOnlyFn func(ctx context.Context, reason error)

	// event publisher：用于发布 REORG 阶段事件和补偿事件
	eventPublisher *EventPublisher
}

type Options struct {
	Logger          log.Logger
	QueryService    persistence.QueryService
	BlockProcessor  block.BlockProcessor
	SnapshotManager Reversible
	IndexManager    Reversible
	VerifyFn        func(ctx context.Context, expectedHeight uint64) (*VerificationResult, error)
	AtomicRollbackFn func(ctx context.Context, session *ReorgSession) error
	EnterReadOnlyFn func(ctx context.Context, reason error)
	EventPublisher  *EventPublisher
}

func NewCoordinator(opts Options) (*Coordinator, error) {
	if opts.QueryService == nil {
		return nil, fmt.Errorf("QueryService 不能为空")
	}
	if opts.BlockProcessor == nil {
		return nil, fmt.Errorf("BlockProcessor 不能为空")
	}
	if opts.SnapshotManager == nil {
		return nil, fmt.Errorf("SnapshotManager 不能为空")
	}
	if opts.IndexManager == nil {
		return nil, fmt.Errorf("IndexManager 不能为空")
	}
	if opts.VerifyFn == nil {
		return nil, fmt.Errorf("VerifyFn 不能为空")
	}
	return &Coordinator{
		logger:           opts.Logger,
		queryService:     opts.QueryService,
		blockProc:        opts.BlockProcessor,
		snapshotMgr:      opts.SnapshotManager,
		indexMgr:         opts.IndexManager,
		verifyFn:         opts.VerifyFn,
		atomicRollbackFn: opts.AtomicRollbackFn,
		enterReadOnlyFn:  opts.EnterReadOnlyFn,
		eventPublisher:   opts.EventPublisher,
	}, nil
}

func (c *Coordinator) BeginReorg(ctx context.Context, fromHeight, forkHeight, toHeight uint64) (*ReorgSession, error) {
	if forkHeight > fromHeight {
		return nil, &ReorgError{Class: ErrClassPrepare, Phase: PhasePrepare, Err: fmt.Errorf("forkHeight(%d) > fromHeight(%d)", forkHeight, fromHeight)}
	}
	if toHeight <= forkHeight {
		return nil, &ReorgError{Class: ErrClassPrepare, Phase: PhasePrepare, Err: fmt.Errorf("toHeight(%d) <= forkHeight(%d)", toHeight, forkHeight)}
	}

	sid := fmt.Sprintf("reorg:%d:%d:%d:%d", fromHeight, forkHeight, toHeight, time.Now().UnixNano())
	session := &ReorgSession{
		ID:         sid,
		FromHeight: fromHeight,
		ForkHeight: forkHeight,
		ToHeight:   toHeight,
		CreatedAt:  time.Now(),
		Handles:    make(map[string]RollbackHandle),
	}

	// 发布 Prepare 阶段开始事件
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseStarted(ctx, session, PhasePrepare)
	}

	prepareStart := time.Now()

	// Prepare: 创建回滚点（recovery + rollback）
	recovery, err := c.snapshotMgr.CreateRollbackPoint(ctx, fromHeight)
	if err != nil {
		// ✅ 容错策略：检测UTXO损坏导致的快照创建失败
		if strings.Contains(err.Error(), "BlockHeight为0") || strings.Contains(err.Error(), "BlockHeight=0") {
			if c.logger != nil {
				c.logger.Warnf("⚠️ 检测到损坏UTXO导致快照创建失败 (recovery_point, height=%d)", fromHeight)
				c.logger.Warnf("   快照创建时应已自动修复，请稍后重试REORG")
				c.logger.Warnf("   如果问题持续，请检查UTXO数据完整性")
			}
		}
		return nil, &ReorgError{Class: ErrClassPrepare, Phase: PhasePrepare, Err: fmt.Errorf("create recovery rollback point failed: %w", err)}
	}

	rollback, err := c.snapshotMgr.CreateRollbackPoint(ctx, forkHeight)
	if err != nil {
		// ✅ 容错策略：检测UTXO损坏导致的快照创建失败
		if strings.Contains(err.Error(), "BlockHeight为0") || strings.Contains(err.Error(), "BlockHeight=0") {
			if c.logger != nil {
				c.logger.Warnf("⚠️ 检测到损坏UTXO导致快照创建失败 (rollback_point, height=%d)", forkHeight)
				c.logger.Warnf("   快照创建时应已自动修复，请稍后重试REORG")
				c.logger.Warnf("   如果问题持续，请检查UTXO数据完整性")
			}
		}
		return nil, &ReorgError{Class: ErrClassPrepare, Phase: PhasePrepare, Err: fmt.Errorf("create rollback rollback point failed: %w", err)}
	}
	indexHandle, err := c.indexMgr.CreateRollbackPoint(ctx, forkHeight)
	if err != nil {
		return nil, &ReorgError{Class: ErrClassPrepare, Phase: PhasePrepare, Err: fmt.Errorf("create index rollback point failed: %w", err)}
	}
	session.Handles["utxo_recovery"] = recovery
	session.Handles["utxo_rollback"] = rollback
	session.Handles["index_rollback"] = indexHandle

	// 发布 Prepare 阶段完成事件
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseCompleted(ctx, session, PhasePrepare, time.Since(prepareStart))
	}

	if c.logger != nil {
		c.logger.Warnf("🔁 REORG Begin: id=%s from=%d fork=%d to=%d", session.ID, fromHeight, forkHeight, toHeight)
	}
	return session, nil
}

func (c *Coordinator) ExecuteReorg(ctx context.Context, session *ReorgSession, provider BlockProvider) error {
	if session == nil {
		return &ReorgError{Class: ErrClassPrepare, Phase: PhasePrepare, Err: fmt.Errorf("session 不能为空")}
	}
	if provider == nil {
		return &ReorgError{Class: ErrClassPrepare, Phase: PhasePrepare, Err: fmt.Errorf("provider 不能为空")}
	}

	overallStart := time.Now()

	// Phase Rollback：严格原子化（优先单事务）；否则退化为"索引回滚 -> UTXO 回滚"（仅用于过渡）
	if c.logger != nil {
		c.logger.Warnf("🔁 REORG Rollback: id=%s fork=%d", session.ID, session.ForkHeight)
	}
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseStarted(ctx, session, PhaseRollback)
	}
	rollbackStart := time.Now()
	if c.atomicRollbackFn != nil {
		if err := c.atomicRollbackFn(ctx, session); err != nil {
			c.abortToReadOnly(ctx, session, &ReorgError{Class: ErrClassRollback, Phase: PhaseRollback, Err: err})
			return &ReorgError{Class: ErrClassRollback, Phase: PhaseRollback, Err: err}
		}
	} else {
		if err := c.indexMgr.Rollback(ctx, session.Handles["index_rollback"]); err != nil {
			c.abortToReadOnly(ctx, session, &ReorgError{Class: ErrClassRollback, Phase: PhaseRollback, Err: err})
			return &ReorgError{Class: ErrClassRollback, Phase: PhaseRollback, Err: err}
		}
		if err := c.snapshotMgr.Rollback(ctx, session.Handles["utxo_rollback"]); err != nil {
			c.abortToReadOnly(ctx, session, &ReorgError{Class: ErrClassRollback, Phase: PhaseRollback, Err: err})
			return &ReorgError{Class: ErrClassRollback, Phase: PhaseRollback, Err: err}
		}
	}
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseCompleted(ctx, session, PhaseRollback, time.Since(rollbackStart))
	}

	// Phase Replay：逐块重放 forkHeight+1..toHeight
	if c.logger != nil {
		c.logger.Warnf("🔁 REORG Replay: id=%s range=%d..%d", session.ID, session.ForkHeight+1, session.ToHeight)
	}
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseStarted(ctx, session, PhaseReplay)
	}
	replayStart := time.Now()
	for h := session.ForkHeight + 1; h <= session.ToHeight; h++ {
		blk, ok := provider(h)
		if !ok || blk == nil || blk.Header == nil || blk.Header.Height != h {
			err := fmt.Errorf("provider 缺失/无效区块: height=%d", h)
			c.abortToReadOnly(ctx, session, &ReorgError{Class: ErrClassReplay, Phase: PhaseReplay, Err: err})
			return &ReorgError{Class: ErrClassReplay, Phase: PhaseReplay, Err: err}
		}
		ctxWithReorg := context.WithValue(ctx, "reorg_mode", true)
		if err := c.blockProc.ProcessBlock(ctxWithReorg, blk); err != nil {
			c.abortToReadOnly(ctx, session, &ReorgError{Class: ErrClassReplay, Phase: PhaseReplay, Err: fmt.Errorf("process block failed height=%d: %w", h, err)})
			return &ReorgError{Class: ErrClassReplay, Phase: PhaseReplay, Err: fmt.Errorf("process block failed height=%d: %w", h, err)}
		}
	}
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseCompleted(ctx, session, PhaseReplay, time.Since(replayStart))
	}

	// Phase Verify：严格验证（由注入 verifyFn 实现）
	if c.logger != nil {
		c.logger.Warnf("🔁 REORG Verify: id=%s tip=%d", session.ID, session.ToHeight)
	}
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseStarted(ctx, session, PhaseVerify)
	}
	verifyStart := time.Now()
	res, err := c.verifyFn(ctx, session.ToHeight)
	if err != nil || res == nil || !res.Passed {
		if err == nil {
			err = fmt.Errorf("verification failed")
		}
		c.abortToReadOnly(ctx, session, &ReorgError{Class: ErrClassVerify, Phase: PhaseVerify, Err: err})
		return &ReorgError{Class: ErrClassVerify, Phase: PhaseVerify, Err: err}
	}
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseCompleted(ctx, session, PhaseVerify, time.Since(verifyStart))
	}

	// Phase Commit：丢弃回滚点
	if c.logger != nil {
		c.logger.Warnf("🔁 REORG Commit: id=%s", session.ID)
	}
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseStarted(ctx, session, PhaseCommit)
	}
	commitStart := time.Now()
	if err := c.CommitReorg(ctx, session); err != nil {
		c.abortToReadOnly(ctx, session, &ReorgError{Class: ErrClassCommit, Phase: PhaseCommit, Err: err})
		return err
	}
	if c.eventPublisher != nil {
		c.eventPublisher.PublishPhaseCompleted(ctx, session, PhaseCommit, time.Since(commitStart))
	}

	// 发布整体 ForkCompleted 事件（兼容现有订阅者）
	if c.eventPublisher != nil {
		c.eventPublisher.PublishForkCompleted(ctx, session, time.Since(overallStart))
	}

	if c.logger != nil {
		c.logger.Warnf("✅ REORG Done: id=%s new_tip=%d", session.ID, session.ToHeight)
	}
	return nil
}

func (c *Coordinator) CommitReorg(ctx context.Context, session *ReorgSession) error {
	if session == nil {
		return &ReorgError{Class: ErrClassCommit, Phase: PhaseCommit, Err: fmt.Errorf("session 不能为空")}
	}
	// 丢弃回滚点：rollback+recovery（严格要求释放资源，避免泄漏）
	if err := c.snapshotMgr.Discard(ctx, session.Handles["utxo_recovery"]); err != nil {
		return &ReorgError{Class: ErrClassCommit, Phase: PhaseCommit, Err: fmt.Errorf("discard utxo_recovery failed: %w", err)}
	}
	if err := c.snapshotMgr.Discard(ctx, session.Handles["utxo_rollback"]); err != nil {
		return &ReorgError{Class: ErrClassCommit, Phase: PhaseCommit, Err: fmt.Errorf("discard utxo_rollback failed: %w", err)}
	}
	if err := c.indexMgr.Discard(ctx, session.Handles["index_rollback"]); err != nil {
		return &ReorgError{Class: ErrClassCommit, Phase: PhaseCommit, Err: fmt.Errorf("discard index_rollback failed: %w", err)}
	}
	return nil
}

func (c *Coordinator) AbortReorg(ctx context.Context, session *ReorgSession, reason error) error {
	if session == nil {
		return &ReorgError{Class: ErrClassAbort, Phase: PhaseCommit, Err: fmt.Errorf("session 不能为空")}
	}
	// 回滚到 recovery（严格：索引回滚到 fromHeight + UTXO 恢复）
	if err := c.indexMgr.Rollback(ctx, RollbackHandle{Height: session.FromHeight}); err != nil {
		return &ReorgError{Class: ErrClassAbort, Phase: PhaseRollback, Err: err}
	}
	if err := c.snapshotMgr.Rollback(ctx, session.Handles["utxo_recovery"]); err != nil {
		return &ReorgError{Class: ErrClassAbort, Phase: PhaseRollback, Err: err}
	}
	_ = c.CommitReorg(ctx, session)
	_ = reason
	return nil
}

func (c *Coordinator) abortToReadOnly(ctx context.Context, session *ReorgSession, err error) {
	// 提取 ReorgError 信息以便发布事件
	var reorgErr *ReorgError
	if re, ok := err.(*ReorgError); ok {
		reorgErr = re
	} else {
		reorgErr = &ReorgError{Class: ErrClassUnknown, Phase: PhasePrepare, Err: err}
	}

	// 发布 ForkFailed 事件
	if c.eventPublisher != nil {
		c.eventPublisher.PublishForkFailed(ctx, session, reorgErr, time.Since(session.CreatedAt))
	}

	// 尝试 Abort；失败则进入只读（后续 todo 将升级为"全局写门闸"硬停写）。
	abortErr := c.AbortReorg(ctx, session, err)
	
	// 发布 ReorgAborted 事件
	if c.eventPublisher != nil {
		c.eventPublisher.PublishReorgAborted(ctx, session, err, reorgErr.Phase, abortErr == nil, abortErr)
	}

	if abortErr != nil {
		if c.enterReadOnlyFn != nil {
			c.enterReadOnlyFn(ctx, fmt.Errorf("abort_failed: %v; original=%v", abortErr, err))
		}
		return
	}

	// Abort 成功，发布补偿事件
	if c.eventPublisher != nil {
		// 注意：这里的统计信息（utxoRestored, indicesRolledBack）需要从实际操作中获取
		// 暂时使用估算值：回滚的区块数
		utxoCount := int(session.FromHeight - session.ForkHeight)
		indexCount := int(session.FromHeight - session.ForkHeight)
		c.eventPublisher.PublishReorgCompensation(ctx, session, utxoCount, indexCount, true, nil)
	}

	// Abort 成功，仍然返回原错误（由调用方决定是否只读）；此处不强制只读。
}

// Ensure Coordinator implements ReorgCoordinator
var _ ReorgCoordinator = (*Coordinator)(nil)

// helper: compile-time reference to core.Block to avoid unused import in some builds
var _ = (*core.Block)(nil)


