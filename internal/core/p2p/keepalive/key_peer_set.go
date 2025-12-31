package keepalive

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// KeyPeerSet 关键peer集合
// 包含需要主动探测/保活的peer：bootstrap + K桶核心 + 最近有用 + 业务关键
type KeyPeerSet struct {
	mu               sync.RWMutex
	bootstrap        map[peer.ID]struct{} // 配置的bootstrap节点
	kbucketCore      map[peer.ID]struct{} // K桶Active+Suspect节点
	recentlyUseful   map[peer.ID]time.Time // 最近有用的peer及其时间
	businessCritical map[peer.ID]struct{} // 业务层标记的关键peer
	
	maxSize          int                  // 最大集合大小
	usefulWindow     time.Duration        // "最近有用"的时间窗口
}

// NewKeyPeerSet 创建KeyPeerSet
func NewKeyPeerSet(maxSize int, usefulWindow time.Duration) *KeyPeerSet {
	if maxSize <= 0 {
		maxSize = 128
	}
	if usefulWindow <= 0 {
		usefulWindow = 10 * time.Minute
	}
	
	return &KeyPeerSet{
		bootstrap:        make(map[peer.ID]struct{}),
		kbucketCore:      make(map[peer.ID]struct{}),
		recentlyUseful:   make(map[peer.ID]time.Time),
		businessCritical: make(map[peer.ID]struct{}),
		maxSize:          maxSize,
		usefulWindow:     usefulWindow,
	}
}

// SetBootstrapPeers 设置bootstrap节点列表
func (kps *KeyPeerSet) SetBootstrapPeers(peers []peer.ID) {
	kps.mu.Lock()
	defer kps.mu.Unlock()
	
	kps.bootstrap = make(map[peer.ID]struct{}, len(peers))
	for _, p := range peers {
		if p != "" {
			kps.bootstrap[p] = struct{}{}
		}
	}
}

// UpdateKBucketCore 更新K桶核心节点（Active+Suspect）
func (kps *KeyPeerSet) UpdateKBucketCore(peers []peer.ID) {
	kps.mu.Lock()
	defer kps.mu.Unlock()
	
	kps.kbucketCore = make(map[peer.ID]struct{}, len(peers))
	for _, p := range peers {
		if p != "" {
			kps.kbucketCore[p] = struct{}{}
		}
	}
}

// MarkUseful 标记peer为"最近有用"
func (kps *KeyPeerSet) MarkUseful(p peer.ID) {
	if p == "" {
		return
	}
	
	kps.mu.Lock()
	defer kps.mu.Unlock()
	
	kps.recentlyUseful[p] = time.Now()
}

// AddBusinessCritical 添加业务关键peer
func (kps *KeyPeerSet) AddBusinessCritical(p peer.ID) {
	if p == "" {
		return
	}
	
	kps.mu.Lock()
	defer kps.mu.Unlock()
	
	kps.businessCritical[p] = struct{}{}
}

// RemoveBusinessCritical 移除业务关键peer
func (kps *KeyPeerSet) RemoveBusinessCritical(p peer.ID) {
	kps.mu.Lock()
	defer kps.mu.Unlock()
	
	delete(kps.businessCritical, p)
}

// GetAllKeyPeers 获取所有关键peer（合并去重，按maxSize限制）
func (kps *KeyPeerSet) GetAllKeyPeers() []peer.ID {
	kps.mu.RLock()
	defer kps.mu.RUnlock()
	
	now := time.Now()
	merged := make(map[peer.ID]struct{})
	
	// 1. Bootstrap节点（最高优先级）
	for p := range kps.bootstrap {
		merged[p] = struct{}{}
	}
	
	// 2. K桶核心节点
	for p := range kps.kbucketCore {
		merged[p] = struct{}{}
	}
	
	// 3. 业务关键节点
	for p := range kps.businessCritical {
		merged[p] = struct{}{}
	}
	
	// 4. 最近有用节点（过滤过期）
	for p, t := range kps.recentlyUseful {
		if now.Sub(t) <= kps.usefulWindow {
			merged[p] = struct{}{}
		}
	}
	
	// 转换为切片
	result := make([]peer.ID, 0, len(merged))
	for p := range merged {
		result = append(result, p)
	}
	
	// 限制大小（优先保留bootstrap和业务关键，然后K桶核心，最后recently useful）
	if len(result) > kps.maxSize {
		// 简化策略：截断
		result = result[:kps.maxSize]
	}
	
	return result
}

// Size 返回当前关键peer集合大小
func (kps *KeyPeerSet) Size() int {
	return len(kps.GetAllKeyPeers())
}

// Cleanup 清理过期的"最近有用"记录
func (kps *KeyPeerSet) Cleanup() {
	kps.mu.Lock()
	defer kps.mu.Unlock()
	
	now := time.Now()
	for p, t := range kps.recentlyUseful {
		if now.Sub(t) > kps.usefulWindow {
			delete(kps.recentlyUseful, p)
		}
	}
}

