// Package sync 提供 bad-peer 跟踪机制
package sync

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// badPeerTracker 跟踪链身份不匹配的 bad peers
type badPeerTracker struct {
	mu         sync.RWMutex
	badPeers   map[peer.ID]time.Time // peer ID -> 标记时间
	expiryTime time.Duration          // bad peer 标记过期时间
}

var (
	globalBadPeerTracker *badPeerTracker
	badPeerTrackerOnce   sync.Once
)

// getBadPeerTracker 获取全局 bad peer tracker 实例
func getBadPeerTracker() *badPeerTracker {
	badPeerTrackerOnce.Do(func() {
		globalBadPeerTracker = &badPeerTracker{
			badPeers:   make(map[peer.ID]time.Time),
			expiryTime: 1 * time.Hour, // 默认 1 小时后过期
		}
	})
	return globalBadPeerTracker
}

// MarkBadPeer 标记 peer 为 bad-peer（链身份不匹配）
func MarkBadPeer(peerID peer.ID) {
	tracker := getBadPeerTracker()
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	tracker.badPeers[peerID] = time.Now()
}

// IsBadPeer 检查 peer 是否为 bad-peer
func IsBadPeer(peerID peer.ID) bool {
	tracker := getBadPeerTracker()
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	markedTime, exists := tracker.badPeers[peerID]
	if !exists {
		return false
	}

	// 检查是否过期
	if time.Since(markedTime) > tracker.expiryTime {
		// 过期，清理
		tracker.mu.RUnlock()
		tracker.mu.Lock()
		delete(tracker.badPeers, peerID)
		tracker.mu.Unlock()
		tracker.mu.RLock()
		return false
	}

	return true
}

// FilterBadPeers 从 peer 列表中过滤掉 bad peers
func FilterBadPeers(peers []peer.ID) []peer.ID {
	var filtered []peer.ID
	for _, peerID := range peers {
		if !IsBadPeer(peerID) {
			filtered = append(filtered, peerID)
		}
	}
	return filtered
}

// isBadPeerNearExpiry 检查 bad peer 是否即将过期
// 🆕 SYNC-HIGH002修复：紧急模式下放宽过滤条件
func isBadPeerNearExpiry(peerID peer.ID, threshold time.Duration) bool {
	tracker := getBadPeerTracker()
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	markedTime, exists := tracker.badPeers[peerID]
	if !exists {
		return false
	}

	// 计算剩余过期时间
	elapsed := time.Since(markedTime)
	remaining := tracker.expiryTime - elapsed

	// 如果剩余时间小于阈值，则认为即将过期
	return remaining < threshold && remaining > 0
}

// GetBadPeerStats 获取坏节点统计信息
func GetBadPeerStats() (total int, nearExpiry int) {
	tracker := getBadPeerTracker()
	tracker.mu.RLock()
	defer tracker.mu.RUnlock()

	now := time.Now()
	threshold := 10 * time.Minute

	for _, markedTime := range tracker.badPeers {
		elapsed := now.Sub(markedTime)
		if elapsed < tracker.expiryTime {
			total++
			remaining := tracker.expiryTime - elapsed
			if remaining < threshold {
				nearExpiry++
			}
		}
	}
	return
}

