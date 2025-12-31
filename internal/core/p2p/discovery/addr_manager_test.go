package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p2pinterfaces "github.com/weisyn/v1/internal/core/p2p/interfaces"
)

// TestAddrManager_AddDHTAddr 测试添加DHT地址
func TestAddrManager_AddDHTAddr(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器
	cfg := AddrManagerConfig{
		TTL:                  DefaultAddrTTL,
		MaxConcurrentLookups: 10,
		LookupTimeout:        30 * time.Second,
		RefreshInterval:      10 * time.Millisecond,
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	am := NewAddrManager(h, nil, cfg, nil)
	defer am.Stop()

	// 生成测试peer和地址
	testPeerID := generateTestPeerID(t)
	testAddr := generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4001")

	// 添加DHT地址
	am.AddDHTAddr(testPeerID, []ma.Multiaddr{testAddr})

	// 验证地址已添加到peerstore
	addrs := am.peerstore.Addrs(testPeerID)
	assert.Equal(t, 1, len(addrs))
	assert.True(t, addrs[0].Equal(testAddr))

	// 验证刷新时间已记录
	am.mu.RLock()
	_, exists := am.lastRefreshAt[testPeerID]
	am.mu.RUnlock()
	assert.True(t, exists)
}

func TestDefaultAddrTTL_P009(t *testing.T) {
	// P0-009 回归保护：DefaultAddrTTL.DHT 不应再是 30min（过短会导致地址过期 -> addrs=0 -> 网络孤岛）
	assert.Equal(t, 2*time.Hour, DefaultAddrTTL.DHT)
}

// TestAddrManager_AddConnectedAddr 测试连接成功升级TTL
func TestAddrManager_AddConnectedAddr(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器
	cfg := AddrManagerConfig{
		TTL:                  DefaultAddrTTL,
		MaxConcurrentLookups: 10,
		LookupTimeout:        30 * time.Second,
		RefreshInterval:      10 * time.Millisecond,
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	am := NewAddrManager(h, nil, cfg, nil)
	defer am.Stop()

	// 生成测试peer和地址
	testPeerID := generateTestPeerID(t)
	testAddr := generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4001")

	// 先添加DHT地址
	am.AddDHTAddr(testPeerID, []ma.Multiaddr{testAddr})

	// 再升级为连接地址
	am.AddConnectedAddr(testPeerID, []ma.Multiaddr{testAddr})

	// 验证地址仍然存在
	addrs := am.peerstore.Addrs(testPeerID)
	assert.Equal(t, 1, len(addrs))
}

// TestAddrManager_MarkAddrFailed 测试失败降级TTL
func TestAddrManager_MarkAddrFailed(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器
	cfg := AddrManagerConfig{
		TTL:                  DefaultAddrTTL,
		MaxConcurrentLookups: 10,
		LookupTimeout:        30 * time.Second,
		RefreshInterval:      10 * time.Millisecond,
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	am := NewAddrManager(h, nil, cfg, nil)
	defer am.Stop()

	// 生成测试peer和地址
	testPeerID := generateTestPeerID(t)
	testAddr := generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4001")

	// 添加地址
	am.AddDHTAddr(testPeerID, []ma.Multiaddr{testAddr})

	// 标记失败
	am.MarkAddrFailed(testPeerID)

	// 验证地址仍然存在（只是TTL降低）
	addrs := am.peerstore.Addrs(testPeerID)
	assert.Equal(t, 1, len(addrs))
}

// TestAddrManager_GetAddrs_TriggersLookup 测试无地址时触发查询
func TestAddrManager_GetAddrs_TriggersLookup(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器
	cfg := AddrManagerConfig{
		TTL:                  DefaultAddrTTL,
		MaxConcurrentLookups: 10,
		LookupTimeout:        30 * time.Second,
		RefreshInterval:      10 * time.Millisecond,
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	// 使用阻塞 routing，确保 triggerAddrLookup 的 goroutine 在断言前不会快速退出并清理 pending 标记
	am := NewAddrManager(h, blockingRouting{}, cfg, nil)
	am.Start()
	defer am.Stop()

	// 生成测试peer（无地址）
	testPeerID := generateTestPeerID(t)

	// 获取地址（应该为空，但会触发查询）
	addrs := am.GetAddrs(testPeerID)
	assert.Equal(t, 0, len(addrs))

	// 验证查询已标记为pending
	am.mu.RLock()
	isPending := am.pendingLookups[testPeerID]
	am.mu.RUnlock()
	assert.True(t, isPending)

	// 等待一小段时间让异步查询完成
	time.Sleep(100 * time.Millisecond)
}

// blockingRouting 用于测试：FindPeer 会一直阻塞直到 ctx.Done()，
// 从而使 pendingLookups 在短时间内保持为 true，避免测试竞争条件。
type blockingRouting struct{}

var _ p2pinterfaces.RendezvousRouting = (*blockingRouting)(nil)

func (blockingRouting) AdvertiseAndFindPeers(ctx context.Context, ns string) (<-chan libpeer.AddrInfo, error) {
	ch := make(chan libpeer.AddrInfo)
	close(ch)
	return ch, nil
}

func (blockingRouting) FindPeer(ctx context.Context, id libpeer.ID) (libpeer.AddrInfo, error) {
	<-ctx.Done()
	return libpeer.AddrInfo{}, ctx.Err()
}

func (blockingRouting) RoutingTableSize() int { return 0 }
func (blockingRouting) Offline() bool         { return false }

// TestAddrManager_RefreshLoop 测试刷新循环
func TestAddrManager_RefreshLoop(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器（使用较短的TTL用于测试）
	cfg := AddrManagerConfig{
		TTL: AddrTTL{
			DHT:       2 * time.Second,
			Connected: 24 * time.Hour,
			Bootstrap: 0,
			Failed:    5 * time.Minute,
		},
		MaxConcurrentLookups: 10,
		LookupTimeout:        30 * time.Second,
		RefreshInterval:      10 * time.Millisecond,
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	am := NewAddrManager(h, nil, cfg, nil)
	am.Start()
	defer am.Stop()

	// 添加一个地址
	testPeerID := generateTestPeerID(t)
	testAddr := generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4001")
	am.AddDHTAddr(testPeerID, []ma.Multiaddr{testAddr})

	// 等待超过刷新阈值
	time.Sleep(3 * time.Second)

	// 手动触发刷新检查
	shouldRefresh := am.shouldRefresh(testPeerID)
	assert.True(t, shouldRefresh)
}

// 辅助函数：生成测试peer ID
func generateTestPeerID(t *testing.T) libpeer.ID {
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()
	return h.ID()
}

// 辅助函数：生成测试multiaddr
func generateTestMultiaddr(t *testing.T, addrStr string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(addrStr)
	require.NoError(t, err)
	return addr
}

// ====================
// 🆕 P1 修复相关测试
// ====================

// TestAddrManager_MaxTrackedPeers 测试最大跟踪 peer 数限制
func TestAddrManager_MaxTrackedPeers(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器，设置较小的最大跟踪数用于测试
	cfg := AddrManagerConfig{
		TTL:                  DefaultAddrTTL,
		MaxConcurrentLookups: 5,
		LookupTimeout:        15 * time.Second,
		RefreshInterval:      10 * time.Millisecond,
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	am := NewAddrManager(h, nil, cfg, nil)
	
	// 手动设置更小的限制用于测试
	am.maxTrackedPeers = 10
	am.maxAddrsPerPeer = 3
	
	defer am.Stop()

	// 添加超过限制的 peer
	for i := 0; i < 15; i++ {
		testPeerID := generateTestPeerID(t)
		testAddr := generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4001")
		am.AddDHTAddr(testPeerID, []ma.Multiaddr{testAddr})
		time.Sleep(10 * time.Millisecond) // 稍微延迟确保时间戳不同
	}

	// 触发有界化检查
	am.enforceBounds()

	// 验证 peer 数量被限制
	peers := am.peerstore.Peers()
	// 减去自身 peer
	peerCount := len(peers) - 1
	assert.LessOrEqual(t, peerCount, am.maxTrackedPeers, "peer count should be <= maxTrackedPeers")
}

// TestAddrManager_MaxAddrsPerPeer 测试每个 peer 最大地址数限制
func TestAddrManager_MaxAddrsPerPeer(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器
	cfg := AddrManagerConfig{
		TTL:                  DefaultAddrTTL,
		MaxConcurrentLookups: 5,
		LookupTimeout:        15 * time.Second,
		RefreshInterval:      10 * time.Millisecond,
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	am := NewAddrManager(h, nil, cfg, nil)
	
	// 设置较小的最大地址数用于测试
	am.maxAddrsPerPeer = 3
	
	defer am.Stop()

	// 添加多个地址（超过限制）
	addrs := []ma.Multiaddr{
		generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4001"),
		generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4002"),
		generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4003"),
		generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4004"),
		generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4005"),
	}

	// 通过 capAddrs 限制地址数
	cappedAddrs := am.capAddrs(addrs)
	
	// 验证地址数被限制
	assert.LessOrEqual(t, len(cappedAddrs), am.maxAddrsPerPeer, "address count should be <= maxAddrsPerPeer")
}

// TestAddrManager_MaxRediscoveryQueue 测试重发现队列最大限制
func TestAddrManager_MaxRediscoveryQueue(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器
	cfg := AddrManagerConfig{
		TTL:                  DefaultAddrTTL,
		MaxConcurrentLookups: 5,
		LookupTimeout:        15 * time.Second,
		RefreshInterval:      1 * time.Minute, // 长间隔避免自动处理
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	am := NewAddrManager(h, nil, cfg, nil)
	
	// 设置较小的队列限制用于测试
	am.maxRediscoveryQueue = 5
	
	defer am.Stop()

	// 添加超过限制的重发现任务
	for i := 0; i < 10; i++ {
		peerID := generateTestPeerID(t)
		am.TriggerRediscovery(peerID, false)
	}

	// 验证队列大小被限制
	queueSize := am.GetRediscoveryQueueSize()
	assert.LessOrEqual(t, queueSize, am.maxRediscoveryQueue, "rediscovery queue should be <= maxRediscoveryQueue")
}

// TestAddrManager_MemoryStats 测试内存统计功能
func TestAddrManager_MemoryStats(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器
	cfg := AddrManagerConfig{
		TTL:                  DefaultAddrTTL,
		MaxConcurrentLookups: 5,
		LookupTimeout:        15 * time.Second,
		RefreshInterval:      10 * time.Millisecond,
		RefreshThreshold:     5 * time.Millisecond,
		EnablePersistence:    false,
	}
	am := NewAddrManager(h, nil, cfg, nil)
	defer am.Stop()

	// 添加一些 peer
	for i := 0; i < 5; i++ {
		testPeerID := generateTestPeerID(t)
		testAddr := generateTestMultiaddr(t, "/ip4/127.0.0.1/tcp/4001")
		am.AddDHTAddr(testPeerID, []ma.Multiaddr{testAddr})
	}

	// 获取内存统计
	stats := am.CollectMemoryStats()

	// 验证模块名称
	assert.Equal(t, "p2p.addr_manager", stats.Module)
	assert.Equal(t, "L2-Infrastructure", stats.Layer)
	
	// 验证有对象计数
	assert.GreaterOrEqual(t, stats.Objects, int64(5), "should have at least 5 peers")
	
	// 验证有内存估算
	assert.Greater(t, stats.ApproxBytes, int64(0), "should have non-zero approx bytes")
}

// TestAddrManager_RediscoveryBackoff 测试重发现退避机制
func TestAddrManager_RediscoveryBackoff(t *testing.T) {
	// 创建测试host
	h, err := libp2p.New()
	require.NoError(t, err)
	defer h.Close()

	// 创建地址管理器
	cfg := AddrManagerConfig{
		TTL:                       DefaultAddrTTL,
		MaxConcurrentLookups:      5,
		LookupTimeout:             15 * time.Second,
		RefreshInterval:           1 * time.Minute,
		RefreshThreshold:          5 * time.Millisecond,
		EnablePersistence:         false,
		RediscoveryInterval:       30 * time.Second,
		RediscoveryMaxRetries:     3,
		RediscoveryBackoffBase:    30 * time.Second,
	}
	am := NewAddrManager(h, nil, cfg, nil)
	defer am.Stop()

	// 测试退避计算
	backoff0 := am.calculateBackoff(0)
	backoff1 := am.calculateBackoff(1)
	backoff2 := am.calculateBackoff(2)

	// 验证退避时间指数增长
	assert.Equal(t, 30*time.Second, backoff0, "backoff(0) should be base time")
	assert.Equal(t, 60*time.Second, backoff1, "backoff(1) should be 2x base")
	assert.Equal(t, 120*time.Second, backoff2, "backoff(2) should be 4x base")

	// 验证退避时间上限
	backoff10 := am.calculateBackoff(10)
	assert.LessOrEqual(t, backoff10, 10*time.Minute, "backoff should be capped at 10 minutes")
}

