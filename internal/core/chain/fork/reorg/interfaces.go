package reorg

import (
	"context"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// Reversible 定义可逆组件的统一接口（严格对齐设计讨论文档）。
//
// 说明：
// - 不提供“向后兼容”接口；所有可逆组件必须实现此接口。
// - 事务内原子回滚能力会在后续 To-do 中通过 InTx 扩展实现（不在此阶段简化）。
type Reversible interface {
	CreateRollbackPoint(ctx context.Context, height uint64) (RollbackHandle, error)
	Rollback(ctx context.Context, handle RollbackHandle) error
	Discard(ctx context.Context, handle RollbackHandle) error
	Verify(ctx context.Context, expectedHeight uint64) (*VerificationResult, error)
}

// ReorgCoordinator 协调整个 REORG 过程（Begin/Execute/Commit/Abort）。
type ReorgCoordinator interface {
	BeginReorg(ctx context.Context, fromHeight, forkHeight, toHeight uint64) (*ReorgSession, error)
	ExecuteReorg(ctx context.Context, session *ReorgSession, provider BlockProvider) error
	CommitReorg(ctx context.Context, session *ReorgSession) error
	AbortReorg(ctx context.Context, session *ReorgSession, reason error) error
}

// BlockProvider 提供重放阶段所需的新链区块。
// - 必须严格按 height 返回对应区块
// - 必须保证返回的区块 header.height == height
// - 不允许缺块（缺块视为 reorg 失败）
type BlockProvider func(height uint64) (block *core.Block, ok bool)
