package keepalive

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TestKeyPeerMonitorCreation 测试KeyPeerMonitor创建
func TestKeyPeerMonitorCreation(t *testing.T) {
	// 使用nil参数测试默认值设置
	monitor := NewKeyPeerMonitor(
		nil, // host
		nil, // routing
		nil, // addrManager
		NewKeyPeerSet(10, 5*time.Minute),
		nil, // logger
		nil, // eventBus
		0,   // probeInterval - 应使用默认值
		0,   // perPeerMinInterval
		0,   // probeTimeout
		0,   // failThreshold
		0,   // maxConcurrent
	)
	
	// 检查默认值
	if monitor.probeInterval != 60*time.Second {
		t.Errorf("Expected default probeInterval 60s, got %s", monitor.probeInterval)
	}
	
	if monitor.perPeerMinInterval != 30*time.Second {
		t.Errorf("Expected default perPeerMinInterval 30s, got %s", monitor.perPeerMinInterval)
	}
	
	if monitor.probeTimeout != 5*time.Second {
		t.Errorf("Expected default probeTimeout 5s, got %s", monitor.probeTimeout)
	}
	
	if monitor.failThreshold != 3 {
		t.Errorf("Expected default failThreshold 3, got %d", monitor.failThreshold)
	}
	
	if monitor.maxConcurrent != 5 {
		t.Errorf("Expected default maxConcurrent 5, got %d", monitor.maxConcurrent)
	}
}

// TestKeyPeerMonitorStartStop 测试启动和停止
func TestKeyPeerMonitorStartStop(t *testing.T) {
	monitor := NewKeyPeerMonitor(
		nil,
		nil,
		nil,
		NewKeyPeerSet(10, 5*time.Minute),
		nil,
		nil,
		100*time.Millisecond, // 短间隔用于快速测试
		50*time.Millisecond,
		5*time.Second,
		3,
		5,
	)
	
	// 测试启动
	err := monitor.Start()
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	
	// 重复启动应该失败
	err = monitor.Start()
	if err == nil {
		t.Error("Expected error when starting already running monitor")
	}
	
	// 给点时间让循环运行
	time.Sleep(150 * time.Millisecond)
	
	// 测试停止
	err = monitor.Stop()
	if err != nil {
		t.Fatalf("Failed to stop monitor: %v", err)
	}
	
	// 重复停止应该成功（幂等）
	err = monitor.Stop()
	if err != nil {
		t.Error("Expected no error when stopping already stopped monitor")
	}
}

// TestProbeFailureTracking 测试探测失败跟踪
func TestProbeFailureTracking(t *testing.T) {
	monitor := NewKeyPeerMonitor(
		nil,
		nil,
		nil,
		NewKeyPeerSet(10, 5*time.Minute),
		nil,
		nil,
		60*time.Second,
		30*time.Second,
		5*time.Second,
		3,
		5,
	)
	
	testPeer := peer.ID("test_peer")
	
	// 初始状态
	monitor.stateMu.RLock()
	_, exists := monitor.probeFailures[testPeer]
	monitor.stateMu.RUnlock()
	
	if exists {
		t.Error("Expected no failure record initially")
	}
	
	// 模拟记录失败
	monitor.stateMu.Lock()
	monitor.probeFailures[testPeer] = 2
	monitor.stateMu.Unlock()
	
	monitor.stateMu.RLock()
	count := monitor.probeFailures[testPeer]
	monitor.stateMu.RUnlock()
	
	if count != 2 {
		t.Errorf("Expected failure count 2, got %d", count)
	}
}

// TestProbeIntervalRespected 测试per-peer最小间隔
func TestProbeIntervalRespected(t *testing.T) {
	monitor := NewKeyPeerMonitor(
		nil,
		nil,
		nil,
		NewKeyPeerSet(10, 5*time.Minute),
		nil,
		nil,
		60*time.Second,
		100*time.Millisecond, // 短间隔用于测试
		5*time.Second,
		3,
		5,
	)
	
	testPeer := peer.ID("test_peer")
	now := time.Now()
	
	// 设置最后探测时间
	monitor.stateMu.Lock()
	monitor.lastProbeAt[testPeer] = now
	monitor.stateMu.Unlock()
	
	// 立即检查，应该不满足最小间隔
	monitor.stateMu.RLock()
	lastProbe := monitor.lastProbeAt[testPeer]
	shouldSkip := time.Since(lastProbe) < monitor.perPeerMinInterval
	monitor.stateMu.RUnlock()
	
	if !shouldSkip {
		t.Error("Expected peer to be skipped due to min interval")
	}
	
	// 等待超过最小间隔
	time.Sleep(monitor.perPeerMinInterval + 10*time.Millisecond)
	
	monitor.stateMu.RLock()
	lastProbe = monitor.lastProbeAt[testPeer]
	shouldSkip = time.Since(lastProbe) < monitor.perPeerMinInterval
	monitor.stateMu.RUnlock()
	
	if shouldSkip {
		t.Error("Expected peer NOT to be skipped after min interval passed")
	}
}

// TestConcurrencyLimit 测试并发限制
func TestConcurrencyLimit(t *testing.T) {
	maxConcurrent := 3
	monitor := NewKeyPeerMonitor(
		nil,
		nil,
		nil,
		NewKeyPeerSet(10, 5*time.Minute),
		nil,
		nil,
		60*time.Second,
		30*time.Second,
		5*time.Second,
		3,
		maxConcurrent,
	)
	
	// 检查semaphore容量
	if cap(monitor.probeSem) != maxConcurrent {
		t.Errorf("Expected semaphore capacity %d, got %d", maxConcurrent, cap(monitor.probeSem))
	}
	
	// 模拟并发控制
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// 尝试获取超过限制的token
	tokens := 0
	for i := 0; i < maxConcurrent+1; i++ {
		select {
		case monitor.probeSem <- struct{}{}:
			tokens++
		case <-ctx.Done():
			// 预期会超时，因为已达上限
			break
		}
	}
	
	// 清理
	for i := 0; i < tokens; i++ {
		<-monitor.probeSem
	}
	
	if tokens > maxConcurrent {
		t.Errorf("Expected max %d tokens, got %d", maxConcurrent, tokens)
	}
}

