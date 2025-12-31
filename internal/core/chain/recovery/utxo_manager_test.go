package recovery

import (
	"context"
	"fmt"
	"testing"

	blocktestutil "github.com/weisyn/v1/internal/core/block/testutil"
	core "github.com/weisyn/v1/pb/blockchain/block"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/types"
	"github.com/stretchr/testify/require"
)

type mockUTXOSnapshot struct {
	snapshots []*types.UTXOSnapshotData
	restored  *types.UTXOSnapshotData
}

func (m *mockUTXOSnapshot) CreateSnapshot(ctx context.Context, height uint64) (*types.UTXOSnapshotData, error) {
	return &types.UTXOSnapshotData{SnapshotID: "created", Height: height, StateRoot: make([]byte, 32)}, nil
}
func (m *mockUTXOSnapshot) BuildClearPlan(ctx context.Context) (*eutxo.UTXOClearPlan, error) {
	return &eutxo.UTXOClearPlan{}, nil
}
func (m *mockUTXOSnapshot) LoadSnapshotPayload(ctx context.Context, snapshot *types.UTXOSnapshotData) (*eutxo.UTXOSnapshotPayload, error) {
	_ = ctx
	_ = snapshot
	return &eutxo.UTXOSnapshotPayload{Version: 2, Utxos: nil}, nil
}
func (m *mockUTXOSnapshot) RestoreSnapshotInTransaction(ctx context.Context, tx storage.BadgerTransaction, snapshot *types.UTXOSnapshotData, payload *eutxo.UTXOSnapshotPayload, clearPlan *eutxo.UTXOClearPlan) error {
	_ = tx
	_ = payload
	_ = clearPlan
	m.restored = snapshot
	return nil
}
func (m *mockUTXOSnapshot) RestoreSnapshotAtomic(ctx context.Context, snapshot *types.UTXOSnapshotData) error {
	m.restored = snapshot
	return nil
}
func (m *mockUTXOSnapshot) RestoreSnapshotWithBatching(ctx context.Context, snapshot *types.UTXOSnapshotData, payload *eutxo.UTXOSnapshotPayload, clearPlan *eutxo.UTXOClearPlan) error {
	_ = payload
	_ = clearPlan
	m.restored = snapshot
	return nil
}
func (m *mockUTXOSnapshot) DeleteSnapshot(ctx context.Context, snapshotID string) error { return nil }
func (m *mockUTXOSnapshot) ListSnapshots(ctx context.Context) ([]*types.UTXOSnapshotData, error) {
	return m.snapshots, nil
}

type stubBlockProcessor struct {
	processed []uint64
}

func (s *stubBlockProcessor) ProcessBlock(ctx context.Context, block *core.Block) error {
	s.processed = append(s.processed, block.Header.Height)
	return nil
}

func TestUTXORecoveryManager_ReplayFromLatestSnapshot(t *testing.T) {
	ctx := context.Background()
	query := blocktestutil.NewMockQueryService()

	// 添加 1..5 高度的区块
	for h := uint64(1); h <= 5; h++ {
		b := &core.Block{
			Header: &core.BlockHeader{Height: h, Timestamp: 1},
			Body:   &core.BlockBody{Transactions: nil},
		}
		// MockQueryService 以 hash 为 key；这里只要能被 GetBlockByHeight 遍历到即可
		query.SetBlock([]byte(fmt.Sprintf("block-%d", h)), b)
	}

	snap := &mockUTXOSnapshot{
		snapshots: []*types.UTXOSnapshotData{
			{SnapshotID: "s2", Height: 2, StateRoot: make([]byte, 32)},
		},
	}
	bp := &stubBlockProcessor{}
	bus := blocktestutil.NewMockEventBus()

	m := NewUTXORecoveryManager(query, bp, snap, bus, nil)
	err := m.recoverFromSnapshotAndReplay(ctx)
	require.NoError(t, err)
	require.NotNil(t, snap.restored)
	require.Equal(t, uint64(2), snap.restored.Height)
	require.Equal(t, []uint64{3, 4, 5}, bp.processed)
}

func TestUTXORecoveryManager_NoSnapshots_ReturnsError(t *testing.T) {
	ctx := context.Background()
	query := blocktestutil.NewMockQueryService()
	bp := &stubBlockProcessor{}
	bus := blocktestutil.NewMockEventBus()

	m := NewUTXORecoveryManager(query, bp, &mockUTXOSnapshot{snapshots: nil}, bus, nil)
	err := m.recoverFromSnapshotAndReplay(ctx)
	require.Error(t, err)
}


