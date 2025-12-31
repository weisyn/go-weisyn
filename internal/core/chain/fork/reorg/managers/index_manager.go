package managers

import (
	"context"
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/core/chain/fork/reorg"
)

// IndexRollbackFn 回滚索引到指定高度（不包含 UTXO 恢复——由 SnapshotManager 负责）。
type IndexRollbackFn func(ctx context.Context, height uint64) error

// IndexManager 负责索引可逆状态（区块/交易/资源/历史索引）的回滚。
//
// 注意：
// - 在本 To-do 阶段，IndexManager 复用现有实现（RollbackToHeight）完成索引删除与 tip 更新。
// - 后续 todo（rollback-plan-refactor/atomic-rollback-single-tx）会把此处演进为“计划预收集+协调器单事务执行”。
type IndexManager struct {
	rollback IndexRollbackFn
}

func NewIndexManager(rollback IndexRollbackFn) *IndexManager {
	return &IndexManager{rollback: rollback}
}

func (m *IndexManager) CreateRollbackPoint(ctx context.Context, height uint64) (reorg.RollbackHandle, error) {
	if m == nil || m.rollback == nil {
		return reorg.RollbackHandle{}, fmt.Errorf("IndexRollbackFn 未注入")
	}
	return reorg.RollbackHandle{
		ID:        fmt.Sprintf("index:%d:%d", height, time.Now().UnixNano()),
		Height:    height,
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			"type": "index",
		},
	}, nil
}

func (m *IndexManager) Rollback(ctx context.Context, handle reorg.RollbackHandle) error {
	if m == nil || m.rollback == nil {
		return fmt.Errorf("IndexRollbackFn 未注入")
	}
	return m.rollback(ctx, handle.Height)
}

func (m *IndexManager) Discard(ctx context.Context, handle reorg.RollbackHandle) error {
	// 索引回滚点为逻辑句柄，无需资源释放；后续引入“计划缓存”后在此释放。
	return nil
}

func (m *IndexManager) Verify(ctx context.Context, expectedHeight uint64) (*reorg.VerificationResult, error) {
	// 索引深度验证由协调器 Verify 阶段统一完成（不在此阶段重复扫描）。
	return &reorg.VerificationResult{
		Passed: true,
		Checks: []reorg.CheckResult{
			{
				Name:     "IndexManager:Delegated",
				Passed:   true,
				Expected: fmt.Sprintf("rolled_back_to_%d", expectedHeight),
				Actual:   "delegated_to_coordinator_verify",
				Details:  "索引连续性/可达性验证在 Coordinator.Verify 阶段执行（严格版）。",
			},
		},
	}, nil
}

var _ reorg.Reversible = (*IndexManager)(nil)


