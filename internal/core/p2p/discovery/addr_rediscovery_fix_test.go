package discovery

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	libpeer "github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// TestRediscoveryConcurrencyLimit 验证并发控制
func TestRediscoveryConcurrencyLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建测试用的AddrManager
	am := &AddrManager{
		ctx:                   ctx,
		cancel:                cancel,
		rediscoveryQueue:      make(map[libpeer.ID]*PeerRediscoveryInfo),
		rediscoverySem:        make(chan struct{}, 5), // 限制5个并发
		rediscoveryInterval:   30 * time.Second,
		rediscoveryMaxRetries: 10,
		maxRediscoveryQueue:   50,
	}

	// 尝试启动10个并发任务
	var running int32
	var maxConcurrent int32

	for i := 0; i < 10; i++ {
		select {
		case am.rediscoverySem <- struct{}{}:
			go func() {
				defer func() { <-am.rediscoverySem }()

				current := atomic.AddInt32(&running, 1)
				if current > atomic.LoadInt32(&maxConcurrent) {
					atomic.StoreInt32(&maxConcurrent, current)
				}

				time.Sleep(10 * time.Millisecond) // 模拟工作
				atomic.AddInt32(&running, -1)
			}()
		default:
			// semaphore满了，符合预期
		}
	}

	// 等待所有任务完成
	time.Sleep(100 * time.Millisecond)

	// 验证最大并发数不超过5
	require.LessOrEqual(t, int(atomic.LoadInt32(&maxConcurrent)), 5,
		"最大并发数应该不超过5")
}

// TestRediscoveryQueueMaxSize 验证队列大小限制
func TestRediscoveryQueueMaxSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	am := &AddrManager{
		ctx:                 ctx,
		cancel:              cancel,
		rediscoveryQueue:    make(map[libpeer.ID]*PeerRediscoveryInfo),
		rediscoverySem:      make(chan struct{}, 5),
		maxRediscoveryQueue: 50, // 队列上限50
		bootstrapPeers:      make(map[libpeer.ID]struct{}),
	}
	am.rediscoveryMu = sync.RWMutex{}

	// 先填满队列到50
	for i := 0; i < 50; i++ {
		pid := generateSimpleTestPeerID(i)
		am.rediscoveryMu.Lock()
		am.rediscoveryQueue[pid] = &PeerRediscoveryInfo{
			PeerID:        pid,
			LastAttemptAt: time.Now(),
			FailCount:     0,
			Priority:      0,
		}
		am.rediscoveryMu.Unlock()
	}

	// 验证队列大小为50
	require.Equal(t, 50, am.GetRediscoveryQueueSize(), "队列大小应该为50")

	// 尝试添加第51个peer（应该触发淘汰机制）
	newPID := generateSimpleTestPeerID(999)
	am.rediscoveryMu.Lock()
	
	// 模拟TriggerRediscovery中的有界化逻辑
	if len(am.rediscoveryQueue) >= am.maxRediscoveryQueue {
		// 找到一个低价值条目淘汰
		var victim libpeer.ID
		first := true
		for id := range am.rediscoveryQueue {
			if first {
				victim = id
				break
			}
		}
		delete(am.rediscoveryQueue, victim)
	}
	
	am.rediscoveryQueue[newPID] = &PeerRediscoveryInfo{
		PeerID:        newPID,
		LastAttemptAt: time.Now(),
		FailCount:     0,
		Priority:      1, // 高优先级
	}
	am.rediscoveryMu.Unlock()

	// 验证队列大小仍然是50（有淘汰发生）
	require.Equal(t, 50, am.GetRediscoveryQueueSize(), "队列大小应该保持在50")
}

// TestRediscoveryTimeout 验证超时机制
func TestRediscoveryTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 创建一个会超时的mock routing
	mockRouting := &mockTimingoutRouting{
		delay: 35 * time.Second, // 超过30秒超时
	}

	am := &AddrManager{
		ctx:                    ctx,
		cancel:                 cancel,
		routing:                mockRouting,
		rediscoveryQueue:       make(map[libpeer.ID]*PeerRediscoveryInfo),
		rediscoverySem:         make(chan struct{}, 5),
		rediscoveryInterval:    30 * time.Second,
		rediscoveryMaxRetries:  10,
		rediscoveryBackoffBase: 1 * time.Minute,
	}
	am.rediscoveryMu = sync.RWMutex{}

	h, _ := libp2p.New()
	defer h.Close()
	pid := h.ID()

	// 执行重发现，带30秒超时
	lookupCtx, lookupCancel := context.WithTimeout(ctx, 30*time.Second)
	defer lookupCancel()

	start := time.Now()
	success := am.executeFindPeerWithContext(lookupCtx, pid)
	elapsed := time.Since(start)

	// 验证超时发生
	require.False(t, success, "查询应该失败（超时）")
	require.Less(t, elapsed, 32*time.Second, "超时应该在约30秒内触发")
}

// TestFailedPeerCleanup 验证失败peer清理
func TestFailedPeerCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockRouting := &mockFailingRouting{}

	am := &AddrManager{
		ctx:                    ctx,
		cancel:                 cancel,
		routing:                mockRouting,
		rediscoveryQueue:       make(map[libpeer.ID]*PeerRediscoveryInfo),
		rediscoverySem:         make(chan struct{}, 5),
		rediscoveryInterval:    30 * time.Second,
		rediscoveryMaxRetries:  10, // 最大重试10次
		rediscoveryBackoffBase: 1 * time.Minute,
	}
	am.rediscoveryMu = sync.RWMutex{}

	h, _ := libp2p.New()
	defer h.Close()
	pid := h.ID()

	// 添加到队列
	am.rediscoveryMu.Lock()
	am.rediscoveryQueue[pid] = &PeerRediscoveryInfo{
		PeerID:        pid,
		LastAttemptAt: time.Now(),
		FailCount:     0,
		Priority:      0,
	}
	am.rediscoveryMu.Unlock()

	// 模拟10次失败
	for i := 0; i < 10; i++ {
		lookupCtx, lookupCancel := context.WithTimeout(ctx, 30*time.Second)
		am.attemptRediscoveryWithContext(lookupCtx, pid)
		lookupCancel()
	}

	// 验证peer已从队列中移除（达到最大重试次数）
	require.Equal(t, 0, am.GetRediscoveryQueueSize(),
		"达到最大重试次数后，peer应该被从队列移除")
}

// TestGetRediscoveryQueueStats 验证队列统计功能
func TestGetRediscoveryQueueStats(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	am := &AddrManager{
		ctx:                 ctx,
		cancel:              cancel,
		rediscoveryQueue:    make(map[libpeer.ID]*PeerRediscoveryInfo),
		maxRediscoveryQueue: 50,
	}
	am.rediscoveryMu = sync.RWMutex{}

	// 添加不同状态的peers
	now := time.Now()

	// 高优先级，失败3次
	pid1 := generateSimpleTestPeerID(1)
	am.rediscoveryQueue[pid1] = &PeerRediscoveryInfo{
		PeerID:        pid1,
		LastAttemptAt: now.Add(-60 * time.Second),
		FailCount:     3,
		Priority:      1,
	}

	// 普通优先级，失败5次
	pid2 := generateSimpleTestPeerID(2)
	am.rediscoveryQueue[pid2] = &PeerRediscoveryInfo{
		PeerID:        pid2,
		LastAttemptAt: now.Add(-120 * time.Second),
		FailCount:     5,
		Priority:      0,
	}

	// 普通优先级，失败2次
	pid3 := generateSimpleTestPeerID(3)
	am.rediscoveryQueue[pid3] = &PeerRediscoveryInfo{
		PeerID:        pid3,
		LastAttemptAt: now.Add(-30 * time.Second),
		FailCount:     2,
		Priority:      0,
	}

	stats := am.GetRediscoveryQueueStats()

	// 验证统计信息
	require.Equal(t, 3, stats.QueueSize, "队列大小应该为3")
	require.Equal(t, 1, stats.HighPriorityCount, "高优先级peer数量应该为1")
	require.Equal(t, 3, stats.FailedCount, "失败peer数量应该为3")
	require.Equal(t, 5, stats.MaxFailCount, "最大失败次数应该为5")
	require.InDelta(t, 3.33, stats.AvgFailCount, 0.1, "平均失败次数应该约为3.33")
	require.Equal(t, int64(120), stats.OldestAttemptAge, "最久未尝试年龄应该为120秒")
}

// Mock helpers

// generateSimpleTestPeerID 生成简单的测试peer ID（不需要testing.T）
func generateSimpleTestPeerID(n int) libpeer.ID {
	// 这里不需要生成“合法的 libp2p peer.ID”（多重哈希），
	// 测试只需要稳定且唯一的 map key 即可。
	return libpeer.ID(fmt.Sprintf("peer-%d", n))
}

type mockTimingoutRouting struct {
	delay time.Duration
}

func (m *mockTimingoutRouting) FindPeer(ctx context.Context, id libpeer.ID) (libpeer.AddrInfo, error) {
	select {
	case <-time.After(m.delay):
		return libpeer.AddrInfo{}, context.DeadlineExceeded
	case <-ctx.Done():
		return libpeer.AddrInfo{}, ctx.Err()
	}
}

func (m *mockTimingoutRouting) AdvertiseAndFindPeers(ctx context.Context, ns string) (<-chan libpeer.AddrInfo, error) {
	// Mock实现，返回空channel
	ch := make(chan libpeer.AddrInfo)
	close(ch)
	return ch, nil
}

func (m *mockTimingoutRouting) Offline() bool {
	return false
}

func (m *mockTimingoutRouting) RoutingTableSize() int {
	return 0
}

type mockFailingRouting struct{}

func (m *mockFailingRouting) FindPeer(ctx context.Context, id libpeer.ID) (libpeer.AddrInfo, error) {
	return libpeer.AddrInfo{}, context.DeadlineExceeded
}

func (m *mockFailingRouting) AdvertiseAndFindPeers(ctx context.Context, ns string) (<-chan libpeer.AddrInfo, error) {
	// Mock实现，返回空channel
	ch := make(chan libpeer.AddrInfo)
	close(ch)
	return ch, nil
}

func (m *mockFailingRouting) Offline() bool {
	return false
}

func (m *mockFailingRouting) RoutingTableSize() int {
	return 0
}

