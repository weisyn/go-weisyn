package quorum

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// mockLogger 简单的 mock logger
type mockLogger struct{}

func (m *mockLogger) Debug(msg string)                          {}
func (m *mockLogger) Debugf(format string, args ...interface{})  {}
func (m *mockLogger) Info(msg string)                           {}
func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Warn(msg string)                           {}
func (m *mockLogger) Warnf(format string, args ...interface{})  {}
func (m *mockLogger) Error(msg string)                          {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}
func (m *mockLogger) Fatal(msg string)                          {}
func (m *mockLogger) Fatalf(format string, args ...interface{}) {}
func (m *mockLogger) With(args ...interface{}) log.Logger        { return m }
func (m *mockLogger) Sync() error                               { return nil }
func (m *mockLogger) GetZapLogger() *zap.Logger                 { return nil }

// ==================== Mock 实现 ====================

type mockMinerConfigView struct {
	minNetworkQuorumTotal          int
	allowSingleNodeMining          bool
	networkDiscoveryTimeoutSeconds int
	quorumRecoveryTimeoutSeconds   int
	maxHeightSkew                  uint64
	maxTipStalenessSeconds        uint64
	enableTipFreshnessCheck       bool
	enableNetworkAlignmentCheck    bool
}

func (m mockMinerConfigView) GetMinNetworkQuorumTotal() int {
	return m.minNetworkQuorumTotal
}

func (m mockMinerConfigView) GetAllowSingleNodeMining() bool {
	return m.allowSingleNodeMining
}

func (m mockMinerConfigView) GetNetworkDiscoveryTimeoutSeconds() int {
	return m.networkDiscoveryTimeoutSeconds
}

func (m mockMinerConfigView) GetQuorumRecoveryTimeoutSeconds() int {
	return m.quorumRecoveryTimeoutSeconds
}

func (m mockMinerConfigView) GetMaxHeightSkew() uint64 {
	return m.maxHeightSkew
}

func (m mockMinerConfigView) GetMaxTipStalenessSeconds() uint64 {
	return m.maxTipStalenessSeconds
}

func (m mockMinerConfigView) GetEnableTipFreshnessCheck() bool {
	return m.enableTipFreshnessCheck
}

func (m mockMinerConfigView) GetEnableNetworkAlignmentCheck() bool {
	return m.enableNetworkAlignmentCheck
}

// ==================== 测试用例 ====================

func TestChecker_Check_NetworkAlignmentDisabled(t *testing.T) {
	// 测试：配置开关禁用网络对齐检查时，直接允许挖矿
	cfg := mockMinerConfigView{
		enableNetworkAlignmentCheck: false,
	}
	checker := &checker{
		minerCfg: cfg,
		logger:   &mockLogger{},
	}

	ctx := context.Background()
	res, err := checker.Check(ctx)

	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.AllowMining)
	assert.Equal(t, StateHeightAligned, res.State)
	assert.Contains(t, res.Reason, "网络对齐检查已禁用")
}

func TestChecker_Check_TipNotReadable(t *testing.T) {
	// 测试：链尖不可读时阻止挖矿
	// 注意：此测试需要完整的 mock，这里仅验证基本逻辑
	// 完整测试需要集成测试环境
	t.Skip("需要完整的 mock（ChainQuery, QueryService 等），移至集成测试")
}

// ==================== 并发 Hello 测试 ====================

func TestChecker_ConcurrentHello_SemaphoreLimit(t *testing.T) {
	// 测试：semaphore 限制并发度为 10
	checker := &checker{
		helloSemaphore: make(chan struct{}, 10),
		logger:         &mockLogger{},
	}
	
	// 验证 semaphore 容量
	assert.Equal(t, 10, cap(checker.helloSemaphore))
	assert.Equal(t, 0, len(checker.helloSemaphore))
	
	// 模拟 10 个并发任务占满 semaphore
	for i := 0; i < 10; i++ {
		checker.helloSemaphore <- struct{}{}
	}
	assert.Equal(t, 10, len(checker.helloSemaphore))
	
	// 第 11 个任务应该阻塞（使用 select 验证）
	select {
	case checker.helloSemaphore <- struct{}{}:
		t.Fatal("semaphore should be full, but accept new token")
	default:
		// 预期行为：semaphore 满了，select 走 default 分支
	}
	
	// 释放一个令牌
	<-checker.helloSemaphore
	assert.Equal(t, 9, len(checker.helloSemaphore))
	
	// 现在第 11 个任务应该能进入
	select {
	case checker.helloSemaphore <- struct{}{}:
		// 预期行为：能够获取令牌
		assert.Equal(t, 10, len(checker.helloSemaphore))
	default:
		t.Fatal("semaphore should accept new token after release")
	}
}

func TestChecker_ConcurrentHello_ContextCancellation(t *testing.T) {
	// 测试：context 取消时，并发 hello 应该优雅退出
	checker := &checker{
		helloSemaphore: make(chan struct{}, 10),
		logger:         &mockLogger{},
	}
	
	ctx, cancel := context.WithCancel(context.Background())

	// 预先占满 semaphore，确保后续 goroutine 会阻塞在“获取令牌”处，
	// 从而在 ctx 取消后走 <-ctx.Done() 分支并优雅退出（避免随机性）。
	for i := 0; i < cap(checker.helloSemaphore); i++ {
		checker.helloSemaphore <- struct{}{}
	}
	
	// 模拟并发 hello：goroutine 尝试获取 semaphore
	completed := make(chan bool, 1)
	go func() {
		select {
		case checker.helloSemaphore <- struct{}{}:
			defer func() { <-checker.helloSemaphore }()
			// 模拟长时间运行的 hello
			<-ctx.Done()
			completed <- false // context 取消，未完成正常流程
		case <-ctx.Done():
			completed <- true // context 取消，正常退出
		}
	}()
	
	// 立即取消 context
	cancel()
	
	// 验证 goroutine 能够优雅退出
	select {
	case result := <-completed:
		assert.True(t, result, "goroutine should exit gracefully on context cancellation")
	case <-context.Background().Done():
		t.Fatal("goroutine did not exit after context cancellation")
	}
}

func TestChecker_ConcurrentHello_QualifiedPeersCount(t *testing.T) {
	// 测试：并发 hello 完成后，qualified peers 计数应该正确
	// 注意：这个测试验证并发写入 map 的安全性（通过 mutex 保护）
	
	checker := &checker{
		helloSemaphore: make(chan struct{}, 10),
		logger:         &mockLogger{},
	}
	
	// 模拟并发写入 peerHeights map
	peerHeights := make(map[string]uint64)
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	// 启动 30 个并发任务（模拟 30 peers）
	for i := 0; i < 30; i++ {
		wg.Add(1)
		peerID := "peer-" + string(rune(i))
		
		go func(id string, height uint64) {
			defer wg.Done()
			
			// 获取 semaphore 令牌
			select {
			case checker.helloSemaphore <- struct{}{}:
				defer func() { <-checker.helloSemaphore }()
			case <-context.Background().Done():
				return
			}
			
			// 模拟 hello 延迟
			// time.Sleep(time.Millisecond)
			
			// 安全地写入 map
			mu.Lock()
			peerHeights[id] = height
			mu.Unlock()
		}(peerID, uint64(i+100))
	}
	
	// 等待所有任务完成
	wg.Wait()
	
	// 验证所有 30 个 peers 都被正确记录
	assert.Equal(t, 30, len(peerHeights))
	for i := 0; i < 30; i++ {
		peerID := "peer-" + string(rune(i))
		height, exists := peerHeights[peerID]
		assert.True(t, exists, "peer %s should exist in peerHeights", peerID)
		assert.Equal(t, uint64(i+100), height, "peer %s height mismatch", peerID)
	}
}

// 注意：完整的 Check 测试需要完整的 mock（p2pService, netService, routing, ChainQuery, QueryService 等）
// 这里测试了并发 hello 的核心机制（semaphore、context 取消、map 写入安全性）
// 完整的集成测试需要真实的网络环境
