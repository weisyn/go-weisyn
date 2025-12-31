// height_history.go - 网络高度历史记录系统
// 负责记录和追踪网络高度的变化历史，用于诊断和分析
package sync

import (
	"sync"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

// ======================= 网络高度历史记录（SYNC-104修复） =======================
//
// 背景：
// - 同步过程中会从多个节点查询网络高度，不同节点可能返回不同的高度。
// - 记录高度历史有助于：
//   1. 诊断高度不一致问题
//   2. 追踪网络高度的变化趋势
//   3. 识别异常高度报告
//
// 功能：
// - 记录每次高度查询/观察的结果
// - 保留最近的高度历史（默认50条）
// - 提供查询接口供诊断使用

// NetworkHeightRecord 网络高度记录
type NetworkHeightRecord struct {
	Height     uint64    `json:"height"`      // 观察到的高度
	SourcePeer peer.ID   `json:"source_peer"` // 提供高度的节点
	Timestamp  time.Time `json:"timestamp"`   // 观察时间
	Stage      string    `json:"stage"`       // 观察阶段：height_query/hello/blocks
}

var (
	heightHistoryMu     sync.RWMutex
	heightHistory       []NetworkHeightRecord
	maxHeightHistory    = 50
)

// recordNetworkHeight 记录一次网络高度观察
//
// 参数：
//   - height: 观察到的高度
//   - sourcePeer: 提供高度的节点
//   - stage: 观察阶段（height_query/hello/blocks）
func recordNetworkHeight(height uint64, sourcePeer peer.ID, stage string) {
	heightHistoryMu.Lock()
	defer heightHistoryMu.Unlock()

	record := NetworkHeightRecord{
		Height:     height,
		SourcePeer: sourcePeer,
		Timestamp:  time.Now(),
		Stage:      stage,
	}

	heightHistory = append(heightHistory, record)
	if len(heightHistory) > maxHeightHistory {
		heightHistory = heightHistory[1:]
	}
}

// GetNetworkHeightHistory 获取网络高度历史
//
// 返回值：
//   - []NetworkHeightRecord: 高度历史列表（按时间顺序）
func GetNetworkHeightHistory() []NetworkHeightRecord {
	heightHistoryMu.RLock()
	defer heightHistoryMu.RUnlock()
	result := make([]NetworkHeightRecord, len(heightHistory))
	copy(result, heightHistory)
	return result
}

// GetLatestObservedHeight 获取最近观察到的最高高度
//
// 返回值：
//   - uint64: 最高高度
//   - peer.ID: 提供该高度的节点
//   - bool: 是否有历史记录
func GetLatestObservedHeight() (uint64, peer.ID, bool) {
	heightHistoryMu.RLock()
	defer heightHistoryMu.RUnlock()

	if len(heightHistory) == 0 {
		return 0, "", false
	}

	// 查找最高高度
	maxHeight := uint64(0)
	var maxPeer peer.ID
	for _, r := range heightHistory {
		if r.Height > maxHeight {
			maxHeight = r.Height
			maxPeer = r.SourcePeer
		}
	}

	return maxHeight, maxPeer, true
}

// GetHeightStatistics 获取高度统计信息（最近N分钟内）
//
// 参数：
//   - duration: 时间窗口
//
// 返回值：
//   - minHeight: 最小高度
//   - maxHeight: 最大高度
//   - avgHeight: 平均高度
//   - count: 观察次数
func GetHeightStatistics(duration time.Duration) (minHeight, maxHeight, avgHeight uint64, count int) {
	heightHistoryMu.RLock()
	defer heightHistoryMu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var heights []uint64
	for _, r := range heightHistory {
		if r.Timestamp.After(cutoff) {
			heights = append(heights, r.Height)
		}
	}

	if len(heights) == 0 {
		return 0, 0, 0, 0
	}

	minHeight = heights[0]
	maxHeight = heights[0]
	sum := uint64(0)
	for _, h := range heights {
		if h < minHeight {
			minHeight = h
		}
		if h > maxHeight {
			maxHeight = h
		}
		sum += h
	}

	avgHeight = sum / uint64(len(heights))
	count = len(heights)
	return
}

// ClearNetworkHeightHistory 清空网络高度历史（用于测试或管理）
func ClearNetworkHeightHistory() {
	heightHistoryMu.Lock()
	defer heightHistoryMu.Unlock()
	heightHistory = nil
}

