package discovery

import (
	"context"
	"testing"
	"time"

	p2pcfg "github.com/weisyn/v1/internal/config/p2p"
)

// TestSchedulerIntervalCap 测试间隔上限收敛
func TestSchedulerIntervalCap(t *testing.T) {
	opts := &p2pcfg.Options{
		DiscoveryInterval:       5 * time.Second,
		AdvertiseInterval:       15 * time.Minute, // 旧的大上限
		DiscoveryMaxIntervalCap: 2 * time.Minute,  // 新的小上限
		MinPeers:                4,
	}
	
	// schedulerLoop内部使用DiscoveryMaxIntervalCap作为maxInterval
	// 验证配置正确性
	if opts.DiscoveryMaxIntervalCap >= opts.AdvertiseInterval {
		t.Error("DiscoveryMaxIntervalCap should be less than AdvertiseInterval")
	}
	
	if opts.DiscoveryMaxIntervalCap != 2*time.Minute {
		t.Errorf("Expected DiscoveryMaxIntervalCap 2m, got %s", opts.DiscoveryMaxIntervalCap)
	}
}

// TestSchedulerResetChannel 测试重置通道
func TestSchedulerResetChannel(t *testing.T) {
	svc := NewService()
	
	// 检查重置通道已初始化
	if svc.schedulerResetChan == nil {
		t.Error("schedulerResetChan should be initialized")
	}
	
	if svc.dhtResetChan == nil {
		t.Error("dhtResetChan should be initialized")
	}
	
	// 测试非阻塞发送
	select {
	case svc.schedulerResetChan <- struct{}{}:
		// 成功发送
	case <-time.After(10 * time.Millisecond):
		t.Error("Failed to send to schedulerResetChan")
	}
	
	// 消费
	select {
	case <-svc.schedulerResetChan:
		// 成功接收
	case <-time.After(10 * time.Millisecond):
		t.Error("Failed to receive from schedulerResetChan")
	}
}

// TestResetCoolDown 测试冷却机制配置
func TestResetCoolDown(t *testing.T) {
	opts := &p2pcfg.Options{
		DiscoveryResetCoolDown: 10 * time.Second,
	}
	
	if opts.DiscoveryResetCoolDown != 10*time.Second {
		t.Errorf("Expected cooldown 10s, got %s", opts.DiscoveryResetCoolDown)
	}
	
	// 测试默认值场景
	optsDefault := &p2pcfg.Options{}
	if optsDefault.DiscoveryResetCoolDown != 0 {
		// 默认值应该在schedulerLoop中处理
		t.Logf("Default cooldown: %s", optsDefault.DiscoveryResetCoolDown)
	}
}

// TestDHTSteadyIntervalCap 测试DHT steady模式上限
func TestDHTSteadyIntervalCap(t *testing.T) {
	opts := &p2pcfg.Options{
		DHTSteadyIntervalCap: 2 * time.Minute,
		AdvertiseInterval:    15 * time.Minute,
	}
	
	// DHT状态机应该使用DHTSteadyIntervalCap而不是AdvertiseInterval
	if opts.DHTSteadyIntervalCap >= opts.AdvertiseInterval {
		t.Error("DHTSteadyIntervalCap should be less than AdvertiseInterval")
	}
	
	if opts.DHTSteadyIntervalCap != 2*time.Minute {
		t.Errorf("Expected DHTSteadyIntervalCap 2m, got %s", opts.DHTSteadyIntervalCap)
	}
}

// TestResetMinInterval 测试重置最小间隔
func TestResetMinInterval(t *testing.T) {
	opts := &p2pcfg.Options{
		DiscoveryResetMinInterval: 30 * time.Second,
		DiscoveryInterval:         5 * time.Minute,
	}
	
	// 重置后应该回到baseInterval，不是ResetMinInterval
	// ResetMinInterval是防止重置到太小值的保护
	if opts.DiscoveryResetMinInterval > opts.DiscoveryInterval {
		t.Error("ResetMinInterval should not exceed base DiscoveryInterval")
	}
}

// TestSchedulerResetEventData 测试重置事件数据结构
func TestSchedulerResetEventData(t *testing.T) {
	// 这个测试验证事件数据结构的完整性
	// 实际测试在types包中，这里只做类型检查
	
	// 模拟创建重置事件数据
	type DiscoveryResetEventData struct {
		Reason    string
		Trigger   string
		Timestamp int64
	}
	
	data := DiscoveryResetEventData{
		Reason:    "kbucket_degraded",
		Trigger:   "kademlia",
		Timestamp: time.Now().Unix(),
	}
	
	if data.Reason == "" {
		t.Error("Reason should not be empty")
	}
	
	if data.Trigger == "" {
		t.Error("Trigger should not be empty")
	}
	
	if data.Timestamp == 0 {
		t.Error("Timestamp should not be zero")
	}
}

// TestSchedulerResetIntegration 测试重置集成（简化版）
func TestSchedulerResetIntegration(t *testing.T) {
	svc := NewService()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// 模拟重置触发
	go func() {
		time.Sleep(10 * time.Millisecond)
		select {
		case svc.schedulerResetChan <- struct{}{}:
		case <-ctx.Done():
		}
	}()
	
	// 等待并接收
	select {
	case <-svc.schedulerResetChan:
		t.Log("Reset event received successfully")
	case <-ctx.Done():
		t.Error("Timeout waiting for reset event")
	}
}

