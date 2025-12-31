// diagnostics.go - 同步进度可观测性诊断系统
// 负责提供同步状态的实时诊断信息
package sync

import (
	"sync"
	"time"
)

// ======================= 同步诊断数据结构（SYNC-202修复） =======================
//
// 背景：
// - 同步过程涉及多个阶段、多个节点、多种状态，缺乏统一的可观测性。
// - 需要一个统一的诊断接口来提供：
//   1. 当前同步状态和进度
//   2. 网络高度和本地高度信息
//   3. 失败历史和高度历史
//   4. 可用节点和问题节点信息
//
// 功能：
// - 提供GetSyncDiagnostics接口，返回完整的同步诊断信息
// - 在同步关键位置更新诊断状态
// - 支持实时查询和监控

// SyncDiagnostics 同步诊断信息
type SyncDiagnostics struct {
	// 网络高度信息
	CurrentNetworkHeight     uint64    `json:"current_network_height"`
	NetworkHeightSourcePeer  string    `json:"network_height_source_peer"`
	NetworkHeightQueriedAt   time.Time `json:"network_height_queried_at"`

	// 本地同步信息
	CurrentLocalHeight    uint64 `json:"current_local_height"`
	CurrentSyncStage      string `json:"current_sync_stage"` // idle/stage1/stage1.5/stage2/stage3/completed
	CurrentDataSourcePeer string `json:"current_data_source_peer"`

	// 同步进度
	BlocksFetched   uint64  `json:"blocks_fetched"`
	BlocksProcessed uint64  `json:"blocks_processed"`
	SyncProgress    float64 `json:"sync_progress"` // 0.0 - 1.0

	// 失败历史
	RecentFailures []SyncFailureReason `json:"recent_failures"`

	// 高度历史
	HeightHistory []NetworkHeightRecord `json:"height_history"`

	// 节点信息
	AvailablePeers int `json:"available_peers"`
	LowHeightPeers int `json:"low_height_peers"`
	BadPeers       int `json:"bad_peers"`

	// 时间戳
	LastUpdated time.Time `json:"last_updated"`
}

var (
	currentSyncDiagnostics SyncDiagnostics
	syncDiagnosticsMu      sync.RWMutex
)

// UpdateSyncDiagnostics 更新同步诊断信息
//
// 参数：
//   - update: 更新函数，接收当前诊断信息的指针
func UpdateSyncDiagnostics(update func(*SyncDiagnostics)) {
	syncDiagnosticsMu.Lock()
	defer syncDiagnosticsMu.Unlock()
	update(&currentSyncDiagnostics)
	currentSyncDiagnostics.LastUpdated = time.Now()
}

// GetSyncDiagnostics 获取完整的同步诊断信息
//
// 返回值：
//   - SyncDiagnostics: 当前的同步诊断信息
func GetSyncDiagnostics() SyncDiagnostics {
	syncDiagnosticsMu.RLock()
	defer syncDiagnosticsMu.RUnlock()

	// 填充动态数据
	diag := currentSyncDiagnostics
	diag.RecentFailures = GetSyncFailureHistory()
	diag.HeightHistory = GetNetworkHeightHistory()

	// 计算同步进度
	if diag.CurrentNetworkHeight > 0 {
		diag.SyncProgress = float64(diag.CurrentLocalHeight) / float64(diag.CurrentNetworkHeight)
		if diag.SyncProgress > 1.0 {
			diag.SyncProgress = 1.0
		}
	}

	// 统计节点信息
	diag.LowHeightPeers = countLowHeightPeers()
	diag.BadPeers = countBadPeers()

	return diag
}

// countLowHeightPeers 统计低高度节点数量
func countLowHeightPeers() int {
	lowHeightPeersMu.RLock()
	defer lowHeightPeersMu.RUnlock()

	count := 0
	now := time.Now()
	for _, info := range lowHeightPeers {
		if now.Sub(info.RecordedAt) <= lowHeightPeerTTL {
			count++
		}
	}
	return count
}

// countBadPeers 统计坏节点数量
func countBadPeers() int {
	tracker := getBadPeerTracker()
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, ts := range tracker.badPeers {
		if now.Sub(ts) <= tracker.expiryTime {
			count++
		}
	}
	return count
}

// ResetSyncDiagnostics 重置同步诊断信息（用于测试）
func ResetSyncDiagnostics() {
	syncDiagnosticsMu.Lock()
	defer syncDiagnosticsMu.Unlock()
	currentSyncDiagnostics = SyncDiagnostics{}
}

