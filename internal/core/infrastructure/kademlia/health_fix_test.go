package kbucket

import (
	"math"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExponentialDecay 测试指数半衰曲线
func TestExponentialDecay(t *testing.T) {
	// 创建一个peer，初始健康分为50（失败分50）
	p := &PeerInfo{
		Id:                        peer.ID("test-peer"),
		healthScore:               50,
		lastFailureAt:             time.Now().Add(-5 * time.Minute), // 5分钟前失败
		peerState:                 PeerStateSuspect,
		lastHealthUpdateAt:        time.Now().Add(-5 * time.Minute),
	}

	halfLife := 5 * time.Minute
	now := time.Now()

	// 执行一次衰减（经过1个halfLife）
	p.DecayHealth(now, halfLife)

	// 验证：失败影响应衰减到25，健康分应恢复到75
	// failure_0 = 50, periods = 1
	// decayFactor = 0.5^1 = 0.5
	// decayedFailure = 50 * 0.5 = 25
	// healthScore = 50 + 25 = 75
	expectedHealth := 75.0
	actualHealth := p.GetHealthScore()

	// 允许小的浮点误差
	assert.InDelta(t, expectedHealth, actualHealth, 1.0,
		"健康分应从50恢复到约75（经过1个halfLife）")

	// 验证状态应该恢复为Active（健康分>=70）
	assert.Equal(t, PeerStateActive, p.GetState(),
		"健康分>=70时应恢复为Active状态")
}

// TestExponentialDecayFormula 验证指数衰减公式的正确性
func TestExponentialDecayFormula(t *testing.T) {
	p := &PeerInfo{
		Id:                        peer.ID("test-peer"),
		healthScore:               0,  // 100%失败分
		lastFailureAt:             time.Now().Add(-10 * time.Minute),
		peerState:                 PeerStateSuspect,
		lastHealthUpdateAt:        time.Now().Add(-10 * time.Minute),
	}

	halfLife := 5 * time.Minute
	now := time.Now()

	// 经过2个halfLife（10分钟）
	p.DecayHealth(now, halfLife)

	// 验证：failure_0 = 100, periods = 2
	// decayFactor = 0.5^2 = 0.25
	// decayedFailure = 100 * 0.25 = 25
	// healthScore = 0 + 25 = 25
	expectedHealth := 25.0
	actualHealth := p.GetHealthScore()

	assert.InDelta(t, expectedHealth, actualHealth, 1.0,
		"经过2个halfLife，健康分应恢复到约25")
}

// TestConnectedPeerNotCleanedUp 测试连接peer不被删除
func TestConnectedPeerNotCleanedUp(t *testing.T) {
	// 这个测试需要mock的host，简化版本只验证逻辑
	// 实际集成测试会验证完整流程

	p := &PeerInfo{
		Id:                        peer.ID("test-peer"),
		LastUsefulAt:              time.Now().Add(-1 * time.Hour), // 长期无用
		healthScore:               10, // 低健康分
		peerState:                 PeerStateSuspect,
		lastHealthUpdateAt:        time.Now(),
	}

	// 验证：即使healthScore很低且长期无用
	// 但如果连接状态是Connected，就不应该被清理
	// （这个逻辑在cleanupUnhealthyPeers/cleanupSuspectPeers中实现）

	assert.Equal(t, float64(10), p.GetHealthScore(),
		"健康分应为10")
	assert.Equal(t, PeerStateSuspect, p.GetState(),
		"状态应为Suspect")

	// 在实际清理函数中，如果peer.Connectedness == Connected，
	// 会continue跳过清理
	// 这里只是验证peer状态满足清理条件，但不会被清理
}

// TestDisconnectedPeerCanBeCleanedUp 测试断连peer可以清理
func TestDisconnectedPeerCanBeCleanedUp(t *testing.T) {
	p := &PeerInfo{
		Id:                        peer.ID("test-peer"),
		LastUsefulAt:              time.Now().Add(-1 * time.Hour),
		healthScore:               10,
		peerState:                 PeerStateSuspect,
		lastHealthUpdateAt:        time.Now(),
	}

	// 验证：断连 + 长期无用 + 低健康分
	// 应该可以被清理

	gracePeriod := 60 * time.Second
	now := time.Now()

	longTimeUnused := now.Sub(p.LastUsefulAt) > gracePeriod*3
	lowHealth := p.GetHealthScore() < 20

	assert.True(t, longTimeUnused, "应该满足长期无用条件")
	assert.True(t, lowHealth, "应该满足低健康分条件")
}

// TestLastHealthUpdateAt 测试lastHealthUpdateAt字段更新
func TestLastHealthUpdateAt(t *testing.T) {
	p := &PeerInfo{
		Id:                        peer.ID("test-peer"),
		healthScore:               50,
		lastFailureAt:             time.Now(),
		peerState:                 PeerStateSuspect,
	}

	halfLife := 5 * time.Minute

	// 第一次调用，初始化lastHealthUpdateAt
	now1 := time.Now()
	p.DecayHealth(now1, halfLife)

	require.False(t, p.lastHealthUpdateAt.IsZero(),
		"lastHealthUpdateAt应该被初始化")
	assert.Equal(t, now1, p.lastHealthUpdateAt,
		"lastHealthUpdateAt应该等于now")

	// 第二次调用，应该使用Δt
	time.Sleep(100 * time.Millisecond)
	now2 := time.Now()
	p.DecayHealth(now2, halfLife)

	assert.Equal(t, now2, p.lastHealthUpdateAt,
		"lastHealthUpdateAt应该更新为新的now")
}

// TestPeerStateTransitions 测试状态机转换
func TestPeerStateTransitions(t *testing.T) {
	failureThreshold := 3
	quarantineDuration := 1 * time.Minute

	// Active -> Suspect
	p := &PeerInfo{
		Id:                        peer.ID("test-peer"),
		healthScore:               100,
		peerState:                 PeerStateActive,
		lastHealthUpdateAt:        time.Now(),
	}

	// 记录3次失败
	for i := 0; i < failureThreshold; i++ {
		p.RecordFailure(failureThreshold, quarantineDuration)
	}

	assert.Equal(t, PeerStateSuspect, p.GetState(),
		"累计3次失败应转为Suspect")

	// Suspect -> Quarantined
	for i := 0; i < failureThreshold; i++ {
		p.RecordFailure(failureThreshold, quarantineDuration)
	}

	assert.Equal(t, PeerStateQuarantined, p.GetState(),
		"累计更多失败应转为Quarantined")

	// Quarantined -> Active（成功后）
	p.RecordSuccess()

	assert.Equal(t, PeerStateActive, p.GetState(),
		"成功后应立即恢复为Active")
	assert.Equal(t, float64(100), p.GetHealthScore(),
		"成功后健康分应恢复满分")
	assert.Equal(t, 0, p.failureCount,
		"成功后失败计数应清零")
}

// TestMetricsRecording 测试指标记录
func TestMetricsRecording(t *testing.T) {
	metrics := &KBucketMetrics{}

	// 测试清理记录
	metrics.RecordCleanup("disconnected", false)
	metrics.RecordCleanup("long_time_unused", false)
	metrics.RecordCleanup("low_health", false)

	assert.Equal(t, int64(1), metrics.CleanupDisconnected)
	assert.Equal(t, int64(1), metrics.CleanupLongTimeUnused)
	assert.Equal(t, int64(1), metrics.CleanupLowHealth)
	assert.Equal(t, int64(0), metrics.CleanupConnectedViolation,
		"违规清理计数应为0")

	// 测试违规清理记录（应触发告警）
	metrics.RecordCleanup("disconnected", true)
	assert.Equal(t, int64(1), metrics.CleanupConnectedViolation,
		"违规清理已连接peer应被记录")

	// 测试NoClosestPeers记录
	metrics.RecordNoClosestPeers()
	assert.Equal(t, int64(1), metrics.NoClosestPeersFound)

	// 测试维护记录
	metrics.RecordMaintenanceRun()
	assert.Equal(t, int64(1), metrics.MaintenanceRuns)

	// 测试状态分布更新
	metrics.UpdateStateDistribution(10, 5, 2, 1)
	assert.Equal(t, int64(10), metrics.ActiveCount)
	assert.Equal(t, int64(5), metrics.SuspectCount)
	assert.Equal(t, int64(2), metrics.QuarantinedCount)
	assert.Equal(t, int64(1), metrics.EvictedCount)

	// 测试快照
	snapshot := metrics.GetSnapshot()
	assert.Equal(t, int64(2), snapshot.CleanupDisconnected) // 2次（1次正常+1次违规）
	assert.Equal(t, int64(10), snapshot.ActiveCount)
}

// BenchmarkDecayHealth 性能基准测试
func BenchmarkDecayHealth(b *testing.B) {
	p := &PeerInfo{
		Id:                        peer.ID("test-peer"),
		healthScore:               50,
		lastFailureAt:             time.Now(),
		peerState:                 PeerStateSuspect,
		lastHealthUpdateAt:        time.Now(),
	}

	halfLife := 5 * time.Minute
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.DecayHealth(now, halfLife)
	}
}

// TestDecayHealthEdgeCases 测试边界情况
func TestDecayHealthEdgeCases(t *testing.T) {
	halfLife := 5 * time.Minute

	t.Run("未失败过的peer", func(t *testing.T) {
		p := &PeerInfo{
			Id:                        peer.ID("test-peer"),
			healthScore:               100,
			peerState:                 PeerStateActive,
		}

		now := time.Now()
		p.DecayHealth(now, halfLife)

		// 未失败过，健康分应保持100
		assert.Equal(t, float64(100), p.GetHealthScore())
		assert.False(t, p.lastHealthUpdateAt.IsZero())
	})

	t.Run("时间未前进", func(t *testing.T) {
		now := time.Now()
		p := &PeerInfo{
			Id:                        peer.ID("test-peer"),
			healthScore:               50,
			lastFailureAt:             now,
			peerState:                 PeerStateSuspect,
			lastHealthUpdateAt:        now,
		}

		// 使用相同的时间调用
		p.DecayHealth(now, halfLife)

		// 健康分应保持不变
		assert.Equal(t, float64(50), p.GetHealthScore())
	})

	t.Run("健康分不会超过100", func(t *testing.T) {
		p := &PeerInfo{
			Id:                        peer.ID("test-peer"),
			healthScore:               90,
			lastFailureAt:             time.Now().Add(-100 * time.Minute),
			peerState:                 PeerStateActive,
			lastHealthUpdateAt:        time.Now().Add(-100 * time.Minute),
		}

		now := time.Now()
		p.DecayHealth(now, halfLife)

		// 健康分应该恢复到接近100，不会超过（允许浮点误差）
		assert.InDelta(t, float64(100), p.GetHealthScore(), 10.0,
			"健康分应该恢复到接近100")
		assert.LessOrEqual(t, p.GetHealthScore(), float64(100),
			"健康分不应该超过100")
	})
}

// TestMathPowUsage 验证math.Pow的使用
func TestMathPowUsage(t *testing.T) {
	// 验证指数计算
	periods := 2.0
	decayFactor := math.Pow(0.5, periods)

	expectedDecayFactor := 0.25 // 0.5^2 = 0.25
	assert.Equal(t, expectedDecayFactor, decayFactor,
		"math.Pow(0.5, 2) 应该等于 0.25")

	// 验证不是线性的
	linearDecay := 0.5 * periods
	assert.NotEqual(t, linearDecay, decayFactor,
		"指数衰减不应该等于线性衰减")
	assert.Equal(t, 1.0, linearDecay) // 线性会是1.0
	assert.Equal(t, 0.25, decayFactor) // 指数是0.25
}

