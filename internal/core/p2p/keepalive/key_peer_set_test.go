package keepalive

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TestKeyPeerSetBasic 基本功能测试
func TestKeyPeerSetBasic(t *testing.T) {
	kps := NewKeyPeerSet(10, 5*time.Minute)
	
	// 测试bootstrap设置
	bootstrapPeers := []peer.ID{"peer1", "peer2"}
	kps.SetBootstrapPeers(bootstrapPeers)
	
	allPeers := kps.GetAllKeyPeers()
	if len(allPeers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(allPeers))
	}
}

// TestKeyPeerSetMaxSize maxSize限制测试
func TestKeyPeerSetMaxSize(t *testing.T) {
	kps := NewKeyPeerSet(5, 5*time.Minute)
	
	// 添加超过maxSize的peer
	for i := 0; i < 10; i++ {
		kps.MarkUseful(peer.ID("peer" + string(rune('0'+i))))
	}
	
	allPeers := kps.GetAllKeyPeers()
	if len(allPeers) > 5 {
		t.Errorf("Expected max 5 peers, got %d", len(allPeers))
	}
}

// TestKeyPeerSetRecentlyUsefulExpiry 测试recentlyUseful过期清理
func TestKeyPeerSetRecentlyUsefulExpiry(t *testing.T) {
	usefulWindow := 100 * time.Millisecond
	kps := NewKeyPeerSet(10, usefulWindow)
	
	// 添加useful peer
	kps.MarkUseful("peer1")
	kps.MarkUseful("peer2")
	
	// 立即获取，应该有2个
	allPeers := kps.GetAllKeyPeers()
	if len(allPeers) != 2 {
		t.Errorf("Expected 2 peers before expiry, got %d", len(allPeers))
	}
	
	// 等待过期
	time.Sleep(usefulWindow + 50*time.Millisecond)
	
	// 清理
	kps.Cleanup()
	
	// 再次获取，应该为0
	allPeers = kps.GetAllKeyPeers()
	if len(allPeers) != 0 {
		t.Errorf("Expected 0 peers after expiry, got %d", len(allPeers))
	}
}

// TestKeyPeerSetKBucketCoreUpdate 测试K桶核心节点更新
func TestKeyPeerSetKBucketCoreUpdate(t *testing.T) {
	kps := NewKeyPeerSet(10, 5*time.Minute)
	
	// 第一次更新
	kbucketPeers1 := []peer.ID{"kbpeer1", "kbpeer2", "kbpeer3"}
	kps.UpdateKBucketCore(kbucketPeers1)
	
	allPeers := kps.GetAllKeyPeers()
	if len(allPeers) != 3 {
		t.Errorf("Expected 3 peers after first update, got %d", len(allPeers))
	}
	
	// 第二次更新（替换）
	kbucketPeers2 := []peer.ID{"kbpeer4", "kbpeer5"}
	kps.UpdateKBucketCore(kbucketPeers2)
	
	allPeers = kps.GetAllKeyPeers()
	if len(allPeers) != 2 {
		t.Errorf("Expected 2 peers after second update, got %d", len(allPeers))
	}
}

// TestKeyPeerSetBusinessCritical 测试业务关键节点添加/删除
func TestKeyPeerSetBusinessCritical(t *testing.T) {
	kps := NewKeyPeerSet(10, 5*time.Minute)
	
	// 添加业务关键节点
	kps.AddBusinessCritical("biz1")
	kps.AddBusinessCritical("biz2")
	
	allPeers := kps.GetAllKeyPeers()
	if len(allPeers) != 2 {
		t.Errorf("Expected 2 peers, got %d", len(allPeers))
	}
	
	// 删除一个
	kps.RemoveBusinessCritical("biz1")
	
	allPeers = kps.GetAllKeyPeers()
	if len(allPeers) != 1 {
		t.Errorf("Expected 1 peer after removal, got %d", len(allPeers))
	}
}

// TestKeyPeerSetMergeDedup 测试合并去重逻辑
func TestKeyPeerSetMergeDedup(t *testing.T) {
	kps := NewKeyPeerSet(20, 5*time.Minute)
	
	// 在多个集合中添加同一个peer
	samePeer := peer.ID("same_peer")
	
	kps.SetBootstrapPeers([]peer.ID{samePeer, "peer2"})
	kps.UpdateKBucketCore([]peer.ID{samePeer, "peer3"})
	kps.MarkUseful(samePeer)
	kps.AddBusinessCritical(samePeer)
	
	// 应该去重，只有3个不同的peer
	allPeers := kps.GetAllKeyPeers()
	if len(allPeers) != 3 {
		t.Errorf("Expected 3 unique peers after dedup, got %d", len(allPeers))
	}
	
	// 检查samePeer只出现一次
	count := 0
	for _, p := range allPeers {
		if p == samePeer {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected samePeer to appear once, but appeared %d times", count)
	}
}

// TestKeyPeerSetSize Size方法测试
func TestKeyPeerSetSize(t *testing.T) {
	kps := NewKeyPeerSet(10, 5*time.Minute)
	
	if kps.Size() != 0 {
		t.Errorf("Expected size 0 initially, got %d", kps.Size())
	}
	
	kps.SetBootstrapPeers([]peer.ID{"p1", "p2", "p3"})
	
	if kps.Size() != 3 {
		t.Errorf("Expected size 3, got %d", kps.Size())
	}
}

// TestKeyPeerSetEmptyPeerFiltering 测试空peer过滤
func TestKeyPeerSetEmptyPeerFiltering(t *testing.T) {
	kps := NewKeyPeerSet(10, 5*time.Minute)
	
	// 尝试添加空peer
	kps.SetBootstrapPeers([]peer.ID{"", "peer1", ""})
	kps.MarkUseful("")
	kps.AddBusinessCritical("")
	
	// 应该只有一个有效peer
	allPeers := kps.GetAllKeyPeers()
	if len(allPeers) != 1 {
		t.Errorf("Expected 1 peer (empty peers filtered), got %d", len(allPeers))
	}
}

