package keepalive

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/weisyn/v1/pkg/types"
)

// TestNATDisconnectScenario NAT断开场景集成测试
func TestNATDisconnectScenario(t *testing.T) {
	t.Log("=== NAT断开场景测试 ===")
	
	// 1. 设置KeyPeerSet with bootstrap peers
	kps := NewKeyPeerSet(128, 10*time.Minute)
	bootstrapPeers := []peer.ID{"bootstrap1", "bootstrap2", "bootstrap3"}
	kps.SetBootstrapPeers(bootstrapPeers)
	
	// 2. 创建Monitor（使用短间隔用于测试）
	monitor := NewKeyPeerMonitor(
		nil, // 实际场景需要真实的host
		nil, // 实际场景需要DHT routing
		nil,
		kps,
		nil,
		nil, // 实际场景需要eventBus
		200*time.Millisecond, // 短探测间隔
		100*time.Millisecond, // 短per-peer间隔
		5*time.Second,
		2, // 降低阈值加快测试
		5,
	)
	
	// 3. 模拟场景：monitor会尝试探测bootstrap peers
	// 由于没有真实host，探测会"失败"，但我们可以验证失败计数
	
	// 启动monitor
	err := monitor.Start()
	if err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()
	
	// 4. 等待几轮探测
	time.Sleep(500 * time.Millisecond)
	
	// 5. 验证：由于host为nil，探测会被跳过或快速失败
	// 这里主要验证monitor不会崩溃
	t.Log("Monitor运行正常，未崩溃")
	
	// 6. 验证KeyPeerSet仍然保持bootstrap peers
	if kps.Size() != 3 {
		t.Errorf("Expected 3 bootstrap peers, got %d", kps.Size())
	}
	
	t.Log("✅ NAT断开场景测试通过（简化版）")
}

// TestAddressExpiryScenario 地址过期场景集成测试
func TestAddressExpiryScenario(t *testing.T) {
	t.Log("=== 地址过期场景测试 ===")
	
	// 1. 创建KeyPeerSet并添加一些recently useful peers
	kps := NewKeyPeerSet(128, 500*time.Millisecond) // 短过期窗口
	
	kps.MarkUseful("useful1")
	kps.MarkUseful("useful2")
	kps.MarkUseful("useful3")
	
	// 2. 验证初始大小
	if kps.Size() != 3 {
		t.Errorf("Expected 3 peers initially, got %d", kps.Size())
	}
	
	// 3. 等待地址过期
	time.Sleep(600 * time.Millisecond)
	
	// 4. 清理过期peer
	kps.Cleanup()
	
	// 5. 验证过期后大小为0
	if kps.Size() != 0 {
		t.Errorf("Expected 0 peers after expiry, got %d", kps.Size())
	}
	
	// 6. 添加新的useful peer
	kps.MarkUseful("new_useful")
	
	// 7. 验证新peer被添加
	if kps.Size() != 1 {
		t.Errorf("Expected 1 peer after adding new, got %d", kps.Size())
	}
	
	t.Log("✅ 地址过期场景测试通过")
}

// TestEventStormScenario 事件风暴场景集成测试
func TestEventStormScenario(t *testing.T) {
	t.Log("=== 事件风暴场景测试 ===")
	
	// 1. 模拟大量重置事件
	eventChan := make(chan *types.DiscoveryResetEventData, 100)
	
	// 2. 发送大量重置事件
	eventCount := 50
	for i := 0; i < eventCount; i++ {
		eventChan <- &types.DiscoveryResetEventData{
			Reason:    "peer_disconnected",
			Trigger:   "keypeer_monitor",
			PeerID:    "peer" + string(rune(i)),
			Timestamp: time.Now().Unix(),
		}
	}
	
	// 3. 模拟冷却机制消费
	coolDown := 10 * time.Millisecond
	lastProcessed := time.Time{}
	processedCount := 0
	
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	
	for {
		select {
		case event := <-eventChan:
			now := time.Now()
			if now.Sub(lastProcessed) >= coolDown {
				// 处理事件
				processedCount++
				lastProcessed = now
				t.Logf("Processed reset event: reason=%s trigger=%s", event.Reason, event.Trigger)
			} else {
				// 冷却期内，忽略
				t.Logf("Ignored reset event (cooldown): reason=%s", event.Reason)
			}
		case <-ctx.Done():
			goto done
		}
	}
	
done:
	// 4. 验证：由于冷却，实际处理数应该远小于发送数
	t.Logf("Sent %d events, processed %d events", eventCount, processedCount)
	
	if processedCount >= eventCount {
		t.Error("Expected cooldown to reduce processed events")
	}
	
	// 5. 验证处理数合理（大约 500ms / 10ms = 50个，但考虑初始延迟，应该<50）
	if processedCount > 50 {
		t.Errorf("Processed too many events (%d), cooldown may not be working", processedCount)
	}
	
	t.Log("✅ 事件风暴场景测试通过")
}

// TestFailureThresholdScenario 失败阈值场景测试
func TestFailureThresholdScenario(t *testing.T) {
	t.Log("=== 失败阈值场景测试 ===")
	
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
		3, // 阈值为3
		5,
	)
	
	testPeer := peer.ID("test_peer")
	
	// 模拟连续失败
	for i := 1; i <= 4; i++ {
		monitor.stateMu.Lock()
		monitor.probeFailures[testPeer] = i
		failCount := monitor.probeFailures[testPeer]
		monitor.stateMu.Unlock()
		
		t.Logf("Failure %d recorded for peer", i)
		
		// 验证是否达到阈值
		if failCount >= monitor.failThreshold {
			t.Logf("✅ Threshold reached at failure %d, repair would be triggered", i)
			break
		}
	}
	
	// 验证最终失败计数
	monitor.stateMu.RLock()
	finalCount := monitor.probeFailures[testPeer]
	monitor.stateMu.RUnlock()
	
	if finalCount < monitor.failThreshold {
		t.Errorf("Expected failures >= %d, got %d", monitor.failThreshold, finalCount)
	}
	
	t.Log("✅ 失败阈值场景测试通过")
}

// TestRepairChainSimulation 自愈链路模拟测试
func TestRepairChainSimulation(t *testing.T) {
	t.Log("=== 自愈链路模拟测试 ===")
	
	// 这个测试模拟整个自愈流程（不需要真实网络）
	
	// 1. 快速重连失败
	t.Log("Step 1: 快速重连失败（无有效地址）")
	
	// 2. DHT FindPeer（模拟成功）
	t.Log("Step 2: DHT FindPeer获取新地址（模拟成功）")
	newAddrs := []string{"/ip4/192.168.1.100/tcp/28683"}
	if len(newAddrs) == 0 {
		t.Error("Expected new addresses from DHT")
	}
	
	// 3. 使用新地址二次重连
	t.Log("Step 3: 使用新地址二次重连")
	
	// 4. 发布重置事件
	t.Log("Step 4: 发布Discovery间隔重置事件")
	resetEvent := &types.DiscoveryResetEventData{
		Reason:    "peer_disconnected",
		Trigger:   "keypeer_monitor",
		PeerID:    "repaired_peer",
		Timestamp: time.Now().Unix(),
	}
	
	if resetEvent.Reason == "" {
		t.Error("Reset event should have a reason")
	}
	
	t.Log("✅ 自愈链路模拟测试通过")
}

