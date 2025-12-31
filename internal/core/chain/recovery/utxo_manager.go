package recovery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	blockif "github.com/weisyn/v1/pkg/interfaces/block"
	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	eventiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	logiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	"github.com/weisyn/v1/pkg/types"
	corruptutil "github.com/weisyn/v1/pkg/utils/corruption"
)

// UTXORecoveryManager 是 CHAIN 组件内部的自愈子能力：
// - 监听 corruption.detected(utxo_inconsistent)
// - 选择最近可用快照恢复
// - 按高度重放区块以恢复 UTXO/状态根的一致性
//
// ⚠️ 架构约束：
// - 不作为 internal/core 一级组件
// - 不单独引入 fx module，仅在 chain 模块内部被构造并订阅事件
type UTXORecoveryManager struct {
	queryService  persistence.QueryService
	blockProcessor blockif.BlockProcessor
	utxoSnapshot  eutxo.UTXOSnapshot
	bus           eventiface.EventBus
	logger        logiface.Logger

	mu          sync.Mutex
	inProgress  bool
	lastAttempt time.Time
	throttle    time.Duration
}

func NewUTXORecoveryManager(
	queryService persistence.QueryService,
	blockProcessor blockif.BlockProcessor,
	utxoSnapshot eutxo.UTXOSnapshot,
	bus eventiface.EventBus,
	logger logiface.Logger,
) *UTXORecoveryManager {
	return &UTXORecoveryManager{
		queryService:   queryService,
		blockProcessor: blockProcessor,
		utxoSnapshot:   utxoSnapshot,
		bus:            bus,
		logger:         logger,
		throttle:       60 * time.Second,
	}
}

func (m *UTXORecoveryManager) RegisterSubscriptions(ctx context.Context) {
	if m == nil || m.bus == nil || m.queryService == nil || m.blockProcessor == nil || m.utxoSnapshot == nil {
		return
	}
	_ = m.bus.Subscribe(eventiface.EventTypeCorruptionDetected, func(evCtx context.Context, data interface{}) error {
		evt, ok := data.(types.CorruptionEventData)
		if !ok {
			if p, ok2 := data.(*types.CorruptionEventData); ok2 && p != nil {
				evt = *p
				ok = true
			}
		}
		if !ok {
			return nil
		}
		go m.handle(evCtx, evt)
		return nil
	})
}

func (m *UTXORecoveryManager) handle(ctx context.Context, evt types.CorruptionEventData) {
	if m == nil || m.bus == nil {
		return
	}
	if evt.ErrClass == "" {
		evt.ErrClass = corruptutil.ClassifyErr(fmt.Errorf("%s", evt.Error))
	}
	if evt.ErrClass != "utxo_inconsistent" {
		return
	}

	m.mu.Lock()
	if m.inProgress {
		m.mu.Unlock()
		return
	}
	if !m.lastAttempt.IsZero() && time.Since(m.lastAttempt) < m.throttle {
		m.mu.Unlock()
		m.publishRepairResult("rollback_utxo", "skipped", "skipped(throttled)", "")
		return
	}
	m.inProgress = true
	m.lastAttempt = time.Now()
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.inProgress = false
		m.mu.Unlock()
	}()

	if err := m.recoverFromSnapshotAndReplay(ctx); err != nil {
		m.publishRepairResult("rollback_utxo", "failed", "recover failed", err.Error())
		return
	}
	m.publishRepairResult("rollback_utxo", "success", "recover success", "")
}

func (m *UTXORecoveryManager) recoverFromSnapshotAndReplay(ctx context.Context) error {
	snapshots, err := m.utxoSnapshot.ListSnapshots(ctx)
	if err != nil {
		return fmt.Errorf("list snapshots failed: %w", err)
	}
	if len(snapshots) == 0 {
		return fmt.Errorf("no snapshots available")
	}

	// 选择最高高度的快照（最近可验证点）
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Height < snapshots[j].Height })
	snap := snapshots[len(snapshots)-1]
	if snap == nil {
		return fmt.Errorf("snapshot is nil")
	}

	if m.logger != nil {
		m.logger.Warnf("🩹 [UTXORecovery] 开始恢复快照并重放: snapshot_height=%d snapshot_id=%s", snap.Height, snap.SnapshotID)
	}

	if err := m.utxoSnapshot.RestoreSnapshotAtomic(ctx, snap); err != nil {
		return fmt.Errorf("restore snapshot failed: %w", err)
	}

	tipHeight, _, err := m.queryService.GetHighestBlock(ctx)
	if err != nil {
		return fmt.Errorf("get highest block failed: %w", err)
	}
	if tipHeight <= snap.Height {
		return nil
	}

	// 按高度重放（依赖 BlockProcessor 的在线写路径语义）
	for h := snap.Height + 1; h <= tipHeight; h++ {
		block, err := m.queryService.GetBlockByHeight(ctx, h)
		if err != nil {
			return fmt.Errorf("load block failed: height=%d err=%w", h, err)
		}
		if block == nil {
			return fmt.Errorf("block is nil: height=%d", h)
		}
		if err := m.blockProcessor.ProcessBlock(ctx, block); err != nil {
			return fmt.Errorf("replay block failed: height=%d err=%w", h, err)
		}
	}

	return nil
}

func (m *UTXORecoveryManager) publishRepairResult(action, result, details, errMsg string) {
	if m == nil || m.bus == nil {
		return
	}
	data := types.CorruptionRepairEventData{
		Component: types.CorruptionComponentUTXO,
		Phase:     types.CorruptionPhaseApply,
		Action:    action,
		Result:    result,
		Details:   details,
		Error:     errMsg,
		At:        types.RFC3339Time(time.Now()),
	}

	if result == "success" || result == "skipped" || strings.HasPrefix(details, "skipped(") {
		m.bus.Publish(eventiface.EventTypeCorruptionRepaired, context.Background(), data)
	} else {
		m.bus.Publish(eventiface.EventTypeCorruptionRepairFailed, context.Background(), data)
	}
}


