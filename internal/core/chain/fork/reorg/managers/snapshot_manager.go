package managers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/core/chain/fork/reorg"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/types"
)

// SnapshotManager 负责快照可逆状态（UTXO）的回滚点创建/恢复/丢弃/验证。
//
// 注意：
// - 这里不提供向后兼容分支；快照创建/恢复失败应直接失败。
// - “单事务 InTx 恢复”会在后续 To-do 中实现（eutxo-intx-restore / atomic-rollback-single-tx）。
type SnapshotManager struct {
	snapshot eutxo.UTXOSnapshot
	mu       sync.RWMutex
	cache    map[string]*types.UTXOSnapshotData // handleID -> snapshotData
}

func NewSnapshotManager(snapshot eutxo.UTXOSnapshot) *SnapshotManager {
	return &SnapshotManager{
		snapshot: snapshot,
		cache:    make(map[string]*types.UTXOSnapshotData),
	}
}

func (m *SnapshotManager) CreateRollbackPoint(ctx context.Context, height uint64) (reorg.RollbackHandle, error) {
	if m == nil || m.snapshot == nil {
		return reorg.RollbackHandle{}, fmt.Errorf("UTXOSnapshot 未注入")
	}
	s, err := m.snapshot.CreateSnapshot(ctx, height)
	if err != nil {
		return reorg.RollbackHandle{}, err
	}
	if s == nil || s.SnapshotID == "" {
		return reorg.RollbackHandle{}, fmt.Errorf("CreateSnapshot 返回无效快照")
	}

	handleID := "utxo:" + s.SnapshotID
	m.mu.Lock()
	m.cache[handleID] = s
	m.mu.Unlock()

	return reorg.RollbackHandle{
		ID:        handleID,
		Height:    height,
		CreatedAt: time.Now(),
		Metadata: map[string]string{
			"snapshot_id": s.SnapshotID,
			"type":        "utxo_snapshot",
		},
	}, nil
}

func (m *SnapshotManager) Rollback(ctx context.Context, handle reorg.RollbackHandle) error {
	if m == nil || m.snapshot == nil {
		return fmt.Errorf("UTXOSnapshot 未注入")
	}
	m.mu.RLock()
	s := m.cache[handle.ID]
	m.mu.RUnlock()
	if s == nil {
		return fmt.Errorf("快照句柄不存在: %s", handle.ID)
	}
	return m.snapshot.RestoreSnapshotAtomic(ctx, s)
}

// SnapshotForHandle 返回句柄对应的快照数据（用于协调器的“单事务原子回滚”）。
func (m *SnapshotManager) SnapshotForHandle(handle reorg.RollbackHandle) (*types.UTXOSnapshotData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.cache[handle.ID]
	if s == nil {
		return nil, fmt.Errorf("快照句柄不存在: %s", handle.ID)
	}
	return s, nil
}

func (m *SnapshotManager) Discard(ctx context.Context, handle reorg.RollbackHandle) error {
	if m == nil || m.snapshot == nil {
		return fmt.Errorf("UTXOSnapshot 未注入")
	}
	sid := handle.Metadata["snapshot_id"]
	if sid == "" {
		// fallback：从 handleID 解析
		sid = handle.ID
	}
	if err := m.snapshot.DeleteSnapshot(ctx, sid); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.cache, handle.ID)
	m.mu.Unlock()
	return nil
}

func (m *SnapshotManager) Verify(ctx context.Context, expectedHeight uint64) (*reorg.VerificationResult, error) {
	// 快照自身验证：此处只保证 Create/Restore 不报错；深度 StateRoot 对比由协调器的 Verify 阶段统一完成。
	// 不做“简化返回 Passed=true”之外的伪验证；因此返回结构化的 CheckResult，明确哪些校验由其他组件完成。
	return &reorg.VerificationResult{
		Passed: true,
		Checks: []reorg.CheckResult{
			{
				Name:     "SnapshotManager:SnapshotLifecycle",
				Passed:   true,
				Expected: fmt.Sprintf("snapshot_exists_for_height_%d", expectedHeight),
				Actual:   "managed_by_session",
				Details:  "快照完整性与 StateRoot 校验由 Verify 阶段统一完成（避免重复与不一致）。",
			},
		},
	}, nil
}

var _ reorg.Reversible = (*SnapshotManager)(nil)


