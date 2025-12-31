// Package p2p 定义 P2P 节点运行时状态的公共接口与类型
//
// 设计原则：
// - 节点运行时状态由 P2P 模块管理，因为 is_online 等状态与网络状态密切相关
// - 仅暴露抽象接口与轻量级类型，不依赖 internal 实现包
// - 具体实现由 internal/core/p2p 包提供
package p2p

import (
	"context"
	"time"
)

// SyncMode 同步模式
type SyncMode string

const (
	SyncModeFull    SyncMode = "full"    // 完整同步区块 + 全状态
	SyncModeLight   SyncMode = "light"   // 只同步区块头 + SPV 验证
	SyncModeArchive SyncMode = "archive" // 完整同步 + 保留完整历史（无裁剪）
	SyncModePruned  SyncMode = "pruned"  // 只保留最近 N 高度的完整数据 + 全 header
)

// SyncStatus 同步状态
type SyncStatus string

const (
	SyncStatusSyncing SyncStatus = "syncing" // 正在同步中
	SyncStatusSynced  SyncStatus = "synced"  // 已同步到最新
	SyncStatusLagging SyncStatus = "lagging" // 落后于网络（延迟 > 阈值）
	SyncStatusError   SyncStatus = "error"   // 同步出错
)

// Snapshot 运行时状态快照（用于 API 查询）
type Snapshot struct {
	SyncMode            SyncMode   `json:"sync_mode"`
	SyncStatus          SyncStatus `json:"sync_status"`
	IsFullySynced       bool       `json:"is_fully_synced"`
	IsOnline            bool       `json:"is_online"`
	MiningEnabled       bool       `json:"mining_enabled"`
	IsConsensusEligible bool       `json:"is_consensus_eligible"`
	IsVoterInRound      bool       `json:"is_voter_in_round"`
	IsProposerCandidate bool       `json:"is_proposer_candidate"`
}

// RuntimeState 节点运行时状态接口（状态机 + 不变式）
//
// 说明：
// - 节点运行时状态由 P2P 模块管理，因为 is_online 等状态与网络状态密切相关
// - 同步模式、同步状态、挖矿状态等由其他模块通过接口更新
// - P2P 模块负责更新 is_online 状态
type RuntimeState interface {
	// 核心状态字段访问器
	GetSyncMode() SyncMode
	SetSyncMode(ctx context.Context, mode SyncMode) error
	GetSyncStatus() SyncStatus
	SetSyncStatus(status SyncStatus)
	GetIsFullySynced() bool
	SetIsFullySynced(synced bool)
	IsOnline() bool
	SetIsOnline(online bool) // P2P 模块调用此方法更新在线状态
	IsMiningEnabled() bool
	SetMiningEnabled(ctx context.Context, enabled bool) error

	// 派生状态计算
	IsConsensusEligible() bool
	IsVoterInRound() bool
	IsProposerCandidate() bool

	// 状态快照
	GetSnapshot() Snapshot

	// 回调注册
	SetOnSyncModeChanged(callback func(oldMode, newMode SyncMode))
	SetOnMiningEnabledChanged(callback func(enabled bool))
	SetOnSyncStatusChanged(callback func(oldStatus, newStatus SyncStatus))

	// 状态更新辅助函数
	UpdateSyncStatusFromSyncService(
		currentHeight uint64,
		networkLatestHeight uint64,
		syncLagThreshold uint64,
		isSyncing bool,
	)
	StartPeriodicSyncStatusUpdate(
		ctx context.Context,
		getCurrentHeight func() uint64,
		getNetworkLatestHeight func() uint64,
		syncLagThreshold uint64,
		updateInterval time.Duration,
	)
}
