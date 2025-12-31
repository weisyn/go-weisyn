// Package p2p 提供节点运行时状态的实现
//
// 说明：
// - 该包实现 pkg/interfaces/p2p.RuntimeState 抽象接口
// - 节点运行时状态由 P2P 模块管理，因为 is_online 等状态与网络状态密切相关
// - 同步模式、同步状态、挖矿状态等由其他模块通过接口更新
// - P2P 模块负责更新 is_online 状态
package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	p2piface "github.com/weisyn/v1/pkg/interfaces/p2p"
)

// runtimeState 节点运行时状态的默认实现
type runtimeState struct {
	mu sync.RWMutex

	// ========== 核心状态字段 ==========
	syncMode      p2piface.SyncMode
	syncStatus    p2piface.SyncStatus
	isFullySynced bool
	isOnline      bool
	miningEnabled bool

	// ========== 状态更新回调 ==========
	onSyncModeChanged      func(oldMode, newMode p2piface.SyncMode)
	onMiningEnabledChanged func(enabled bool)
	onSyncStatusChanged    func(oldStatus, newStatus p2piface.SyncStatus)

	// ========== 日志记录器 ==========
	logger log.Logger
}

// NewRuntimeState 创建节点运行时状态实例
func NewRuntimeState(logger log.Logger) p2piface.RuntimeState {
	return &runtimeState{
		syncMode:      p2piface.SyncModeFull,      // 默认 full 模式
		syncStatus:    p2piface.SyncStatusSyncing, // 启动时视为同步中
		isFullySynced: false,
		isOnline:      false,
		miningEnabled: false,
		logger:        logger,
	}
}

// ============================================================================
//                           核心状态字段访问器
// ============================================================================

// GetSyncMode 获取同步模式
func (s *runtimeState) GetSyncMode() p2piface.SyncMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.syncMode
}

// SetSyncMode 设置同步模式（带不变式检查）
func (s *runtimeState) SetSyncMode(ctx context.Context, mode p2piface.SyncMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldMode := s.syncMode
	if oldMode == mode {
		return nil // 无需更新
	}

	// 不变式 I6：同步模式切换约束
	// full → light: 必须停止挖矿（如果正在挖矿）
	if oldMode == p2piface.SyncModeFull && mode == p2piface.SyncModeLight {
		if s.miningEnabled {
			if s.logger != nil {
				s.logger.Warnf("切换同步模式 full → light，自动停止挖矿")
			}
			s.miningEnabled = false
			if s.onMiningEnabledChanged != nil {
				s.onMiningEnabledChanged(false)
			}
		}
	}

	// 更新同步模式
	s.syncMode = mode

	if s.logger != nil {
		s.logger.Infof("节点同步模式已更新: %s → %s", oldMode, mode)
	}

	// 触发回调
	if s.onSyncModeChanged != nil {
		s.onSyncModeChanged(oldMode, mode)
	}

	return nil
}

// GetSyncStatus 获取同步状态
func (s *runtimeState) GetSyncStatus() p2piface.SyncStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.syncStatus
}

// SetSyncStatus 设置同步状态
func (s *runtimeState) SetSyncStatus(status p2piface.SyncStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldStatus := s.syncStatus
	if oldStatus == status {
		return // 无需更新
	}

	s.syncStatus = status

	if s.logger != nil {
		s.logger.Debugf("节点同步状态已更新: %s → %s", oldStatus, status)
	}

	// 触发回调
	if s.onSyncStatusChanged != nil {
		s.onSyncStatusChanged(oldStatus, status)
	}
}

// GetIsFullySynced 获取是否已完全同步
func (s *runtimeState) GetIsFullySynced() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isFullySynced
}

// SetIsFullySynced 设置是否已完全同步
func (s *runtimeState) SetIsFullySynced(synced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isFullySynced == synced {
		return // 无需更新
	}

	s.isFullySynced = synced

	if s.logger != nil {
		s.logger.Debugf("节点完全同步状态已更新: %v", synced)
	}
}

// IsOnline 获取是否在线
func (s *runtimeState) IsOnline() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isOnline
}

// SetIsOnline 设置是否在线（由 P2P 模块调用）
func (s *runtimeState) SetIsOnline(online bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isOnline == online {
		return // 无需更新
	}

	s.isOnline = online

	if s.logger != nil {
		s.logger.Debugf("节点在线状态已更新: %v", online)
	}
}

// IsMiningEnabled 获取是否开启挖矿
func (s *runtimeState) IsMiningEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.miningEnabled
}

// SetMiningEnabled 设置是否开启挖矿（带不变式检查）
func (s *runtimeState) SetMiningEnabled(ctx context.Context, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.miningEnabled == enabled {
		return nil // 无需更新
	}

	// 不变式 I4：挖矿前置条件
	// ✅ V2：挖矿门闸由共识层统一执行（网络法定人数 + 高度一致性 + 链尖前置）。
	// RuntimeState 只做“轻节点不能挖矿”的硬约束；不再在此处做网络/同步判定，避免重复与口径不一致。
	if enabled {
		if s.syncMode != p2piface.SyncModeFull &&
			s.syncMode != p2piface.SyncModeArchive &&
			s.syncMode != p2piface.SyncModePruned {
			return ErrMiningNotAllowedForLightNode
		}
	}

	// 更新挖矿状态
	s.miningEnabled = enabled

	if s.logger != nil {
		if enabled {
			s.logger.Infof("节点挖矿已开启")
		} else {
			s.logger.Infof("节点挖矿已关闭")
		}
	}

	// 触发回调
	if s.onMiningEnabledChanged != nil {
		s.onMiningEnabledChanged(enabled)
	}

	return nil
}

// ============================================================================
//                           派生状态计算
// ============================================================================

// IsConsensusEligible 判断是否具备共识资格
//
// 不变式 I1：共识资格不变式
// 只有 full/archive/pruned 模式的节点可以参与共识，且必须已完全同步。
//
// 设计说明：
// - 早期实现要求节点同时「在线」才能参与共识（isOnline==true）；
// - 但在单节点 / 无上游网络场景下，即使没有任何外部连接，节点仍然需要具备本地共识与聚合能力；
// - 特别是 Aggregator 在本地自举链时，必须允许“只有自己一个节点”的共识路径。
// - 因此，这里仅根据同步模式 + 完全同步状态做 gating，不再强制要求 isOnline。
func (s *runtimeState) IsConsensusEligible() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 检查同步模式
	if s.syncMode != p2piface.SyncModeFull &&
		s.syncMode != p2piface.SyncModeArchive &&
		s.syncMode != p2piface.SyncModePruned {
		return false
	}

	// 检查是否已完全同步
	if !s.isFullySynced {
		return false
	}

	return true
}

// IsVoterInRound 判断当前轮次是否参与投票
//
// 不变式 I2：投票义务不变式
// 所有具备共识资格的节点都必须参与投票
func (s *runtimeState) IsVoterInRound() bool {
	return s.IsConsensusEligible()
}

// IsProposerCandidate 判断当前轮次是否可作为出块候选者
//
// 只有具备共识资格且开启挖矿的节点才能成为出块候选
func (s *runtimeState) IsProposerCandidate() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 出块候选的最小硬约束：不能是 light；是否“完全同步/在线”不作为硬门槛
	// （允许单节点/孤岛持续 PoW，后续通过 fork-choice/reorg 吸收分叉）
	if s.syncMode != p2piface.SyncModeFull &&
		s.syncMode != p2piface.SyncModeArchive &&
		s.syncMode != p2piface.SyncModePruned {
		return false
	}
	return s.miningEnabled
}

// ============================================================================
//                           状态快照
// ============================================================================

// GetSnapshot 获取状态快照（用于 API 查询）
func (s *runtimeState) GetSnapshot() p2piface.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return p2piface.Snapshot{
		SyncMode:            s.syncMode,
		SyncStatus:          s.syncStatus,
		IsFullySynced:       s.isFullySynced,
		IsOnline:            s.isOnline,
		MiningEnabled:       s.miningEnabled,
		IsConsensusEligible: s.IsConsensusEligible(),
		IsVoterInRound:      s.IsVoterInRound(),
		IsProposerCandidate: s.IsProposerCandidate(),
	}
}

// ============================================================================
//                           回调注册
// ============================================================================

// SetOnSyncModeChanged 设置同步模式变更回调
func (s *runtimeState) SetOnSyncModeChanged(callback func(oldMode, newMode p2piface.SyncMode)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSyncModeChanged = callback
}

// SetOnMiningEnabledChanged 设置挖矿开关变更回调
func (s *runtimeState) SetOnMiningEnabledChanged(callback func(enabled bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onMiningEnabledChanged = callback
}

// SetOnSyncStatusChanged 设置同步状态变更回调
func (s *runtimeState) SetOnSyncStatusChanged(callback func(oldStatus, newStatus p2piface.SyncStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSyncStatusChanged = callback
}

// ============================================================================
//                           错误定义
// ============================================================================

var (
	ErrMiningNotAllowedForLightNode = &StateError{
		Code:    "MINING_NOT_ALLOWED_FOR_LIGHT_NODE",
		Message: "轻节点不能开启挖矿",
	}
)

// StateError 状态错误
type StateError struct {
	Code    string
	Message string
}

func (e *StateError) Error() string {
	return e.Message
}

// ============================================================================
//                           状态更新辅助函数
// ============================================================================

// UpdateSyncStatusFromSyncService 从同步服务更新同步状态
//
// 此函数由同步服务调用，用于实时更新同步状态
func (s *runtimeState) UpdateSyncStatusFromSyncService(
	currentHeight uint64,
	networkLatestHeight uint64,
	syncLagThreshold uint64,
	isSyncing bool,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 计算是否已完全同步
	isFullySynced := currentHeight >= networkLatestHeight

	// 更新完全同步状态
	if s.isFullySynced != isFullySynced {
		s.isFullySynced = isFullySynced
		if s.logger != nil {
			s.logger.Debugf("节点完全同步状态已更新: %v (current=%d, network=%d)", isFullySynced, currentHeight, networkLatestHeight)
		}
	}

	// 更新同步状态
	var newStatus p2piface.SyncStatus
	if isSyncing {
		if currentHeight < networkLatestHeight {
			lag := networkLatestHeight - currentHeight
			if lag > syncLagThreshold {
				newStatus = p2piface.SyncStatusLagging
			} else {
				newStatus = p2piface.SyncStatusSyncing
			}
		} else {
			if isFullySynced {
				newStatus = p2piface.SyncStatusSynced
			} else {
				newStatus = p2piface.SyncStatusLagging
			}
		}
	} else {
		if isFullySynced {
			newStatus = p2piface.SyncStatusSynced
		} else {
			newStatus = p2piface.SyncStatusLagging
		}
	}

	if s.syncStatus != newStatus {
		oldStatus := s.syncStatus
		s.syncStatus = newStatus
		if s.logger != nil {
			s.logger.Debugf("节点同步状态已更新: %s → %s (current=%d, network=%d)", oldStatus, newStatus, currentHeight, networkLatestHeight)
		}
		if s.onSyncStatusChanged != nil {
			s.onSyncStatusChanged(oldStatus, newStatus)
		}
	}
}

// StartPeriodicSyncStatusUpdate 启动周期性同步状态更新
//
// 此函数启动一个后台 goroutine，周期性从同步服务获取状态并更新
func (s *runtimeState) StartPeriodicSyncStatusUpdate(
	ctx context.Context,
	getCurrentHeight func() uint64,
	getNetworkLatestHeight func() uint64,
	syncLagThreshold uint64,
	updateInterval time.Duration,
) {
	go func() {
		ticker := time.NewTicker(updateInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentHeight := getCurrentHeight()
				networkLatestHeight := getNetworkLatestHeight()
				isSyncing := s.GetSyncStatus() == p2piface.SyncStatusSyncing

				s.UpdateSyncStatusFromSyncService(
					currentHeight,
					networkLatestHeight,
					syncLagThreshold,
					isSyncing,
				)
			}
		}
	}()
}
