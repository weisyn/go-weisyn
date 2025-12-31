// sync_state.go - 同步状态管理
// 负责管理全局同步状态，防止并发同步冲突
package sync

import (
	"context"
	"sync"
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
//                              同步状态管理
// ============================================================================

// activeSyncContext 活跃同步任务上下文
type activeSyncContext struct {
	RequestID       string             // 同步请求ID
	StartTime       time.Time          // 开始时间
	TargetHeight    uint64             // 目标高度
	SourcePeerID    peer.ID            // 数据源节点ID
	CancelFunc      context.CancelFunc // 取消函数
	ProcessedBlocks uint64             // 已处理区块数
}

// syncedPeerRecord 已同步的节点记录
type syncedPeerRecord struct {
	PeerID       peer.ID   // 节点ID
	LastSyncAt   time.Time // 最后同步时间
	SyncedHeight uint64    // 同步时的高度
	IsConsistent bool      // 高度是否一致
}

var (
	// 全局同步状态保护
	activeSyncMutex sync.RWMutex
	activeSyncTask  *activeSyncContext

	// 已同步节点缓存（避免重复同步同一节点）
	syncedPeersMutex sync.RWMutex
	syncedPeersCache map[peer.ID]*syncedPeerRecord
)

func init() {
	syncedPeersCache = make(map[peer.ID]*syncedPeerRecord)
}

// tryAcquireSyncLock 尝试获取同步锁并设置占位标志
// 🎯 确保同时只有一个同步任务在运行，避免资源竞争
//
// 📝 **修复并发窗口期问题**：
// 获取锁成功时立即设置占位符，避免多个并发请求同时通过检查。
// 后续必须调用 setActiveSyncTask 或 releaseSyncLock 来完善或释放状态。
func tryAcquireSyncLock(requestID string, logger log.Logger) bool {
	activeSyncMutex.Lock()
	defer activeSyncMutex.Unlock()

	// 检查是否已有活跃的同步任务
	if activeSyncTask != nil {
		if logger != nil {
			elapsed := time.Since(activeSyncTask.StartTime)
			// 频繁触发是常态（订阅/定时/候选验证都会触发），这里避免刷屏：
			// - 短时间冲突：debug
			// - 长时间占用：warn（可能卡住/网络异常）
			if elapsed > 30*time.Second {
				logger.Warnf("⚠️ 同步任务冲突(长时间占用): 当前活跃任务=%s, 目标高度=%d, 运行时长=%s",
				activeSyncTask.RequestID,
				activeSyncTask.TargetHeight,
					elapsed.String())
			} else {
				logger.Debugf("⏩ skip: sync already running: request=%s active=%s elapsed=%s",
					requestID, activeSyncTask.RequestID, elapsed.String())
			}
		}
		return false
	}

	// 立即设置占位符，防止并发窗口期
	activeSyncTask = &activeSyncContext{
		RequestID: requestID,
		StartTime: time.Now(),
		// 其他字段后续通过 setActiveSyncTask 完善
	}

	if logger != nil {
		logger.Infof("✅ 同步锁获取成功: RequestID=%s", requestID)
	}

	return true
}

func hasActiveSyncTask() bool {
	activeSyncMutex.RLock()
	defer activeSyncMutex.RUnlock()
	return activeSyncTask != nil
}

// releaseSyncLock 释放同步锁
func releaseSyncLock(logger log.Logger) {
	activeSyncMutex.Lock()
	defer activeSyncMutex.Unlock()

	if activeSyncTask != nil {
		if logger != nil {
			logger.Infof("释放同步锁: RequestID=%s, 处理区块数=%d, 运行时长=%s",
				activeSyncTask.RequestID,
				activeSyncTask.ProcessedBlocks,
				time.Since(activeSyncTask.StartTime).String())
		}
	}

	activeSyncTask = nil
}

// setActiveSyncTask 设置活跃同步任务
//
// 🎯 **更新占位符为完整任务**：
// 用完整的任务信息更新之前通过 tryAcquireSyncLock 设置的占位符。
// 如果没有占位符，则直接设置新任务。
func setActiveSyncTask(task *activeSyncContext) {
	activeSyncMutex.Lock()
	defer activeSyncMutex.Unlock()

	if activeSyncTask != nil && activeSyncTask.RequestID == task.RequestID {
		// 更新占位符为完整信息，保持相同的 RequestID 和 StartTime
		task.StartTime = activeSyncTask.StartTime
	}

	activeSyncTask = task
}

// updateSyncProgress 更新同步进度
func updateSyncProgress(processedBlocks uint64) {
	activeSyncMutex.Lock()
	defer activeSyncMutex.Unlock()

	if activeSyncTask != nil {
		activeSyncTask.ProcessedBlocks += processedBlocks
	}
}

// ============================================================================
//                           节点同步状态缓存管理
// ============================================================================

// checkIfPeerRecentlySynced 检查节点是否最近已同步过
// 避免对同一节点重复进行同步请求，提高效率
func checkIfPeerRecentlySynced(peerID peer.ID, currentHeight uint64, syncCacheExpiry time.Duration) bool {
	syncedPeersMutex.RLock()
	defer syncedPeersMutex.RUnlock()

	record, exists := syncedPeersCache[peerID]
	if !exists {
		return false
	}

	// 检查缓存是否过期
	if time.Since(record.LastSyncAt) > syncCacheExpiry {
		return false
	}

	// 如果之前同步时高度一致，且当前高度没有变化，则认为不需要重复同步
	if record.IsConsistent && record.SyncedHeight == currentHeight {
		return true
	}

	return false
}

// recordPeerSyncResult 记录节点同步结果
func recordPeerSyncResult(peerID peer.ID, localHeight, remoteHeight uint64) {
	syncedPeersMutex.Lock()
	defer syncedPeersMutex.Unlock()

	syncedPeersCache[peerID] = &syncedPeerRecord{
		PeerID:       peerID,
		LastSyncAt:   time.Now(),
		SyncedHeight: localHeight,
		IsConsistent: localHeight == remoteHeight,
	}
}

// cleanupExpiredPeerRecords 清理过期的节点记录
func cleanupExpiredPeerRecords(expiry time.Duration) {
	syncedPeersMutex.Lock()
	defer syncedPeersMutex.Unlock()

	now := time.Now()
	for peerID, record := range syncedPeersCache {
		if now.Sub(record.LastSyncAt) > expiry {
			delete(syncedPeersCache, peerID)
		}
	}
}
