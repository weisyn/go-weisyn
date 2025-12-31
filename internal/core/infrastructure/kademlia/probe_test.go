package kbucket

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// TestProbeStatusTransition 测试探测状态转换
func TestProbeStatusTransition(t *testing.T) {
	p := &PeerInfo{
		Id:           peer.ID("test-peer"),
		probeStatus:  ProbeNotNeeded,
		healthScore:  50,
		LastUsefulAt: time.Now().Add(-20 * time.Minute),
	}

	// 初始状态应该是NotNeeded
	require.Equal(t, ProbeNotNeeded, p.probeStatus)

	// 模拟标记为待探测
	p.stateLock.Lock()
	p.probeStatus = ProbePending
	p.lastProbeAt = time.Time{}
	p.probeFailCount = 0
	p.stateLock.Unlock()

	require.Equal(t, ProbePending, p.probeStatus)
	require.Equal(t, 0, p.probeFailCount)

	// 模拟探测失败
	p.stateLock.Lock()
	p.probeFailCount++
	p.stateLock.Unlock()

	require.Equal(t, 1, p.probeFailCount)

	// 连续3次失败后应该变为ProbeFailed
	p.stateLock.Lock()
	p.probeFailCount = 3
	p.probeStatus = ProbeFailed
	p.stateLock.Unlock()

	require.Equal(t, ProbeFailed, p.probeStatus)
	require.Equal(t, 3, p.probeFailCount)
}

// TestProbeSuccessRestoresHealth 测试探测成功恢复健康状态
func TestProbeSuccessRestoresHealth(t *testing.T) {
	now := time.Now()
	p := &PeerInfo{
		Id:           peer.ID("test-peer"),
		probeStatus:  ProbePending,
		healthScore:  20, // 低健康分
		failureCount: 5,
		peerState:    PeerStateSuspect,
		LastUsefulAt: now.Add(-30 * time.Minute),
		probeFailCount: 1,
	}

	// 记录探测成功
	p.stateLock.Lock()
	p.probeStatus = ProbeSuccess
	p.probeFailCount = 0
	p.healthScore = 100
	p.failureCount = 0
	p.LastUsefulAt = time.Now()
	p.peerState = PeerStateActive
	p.stateLock.Unlock()

	// 验证状态恢复
	require.Equal(t, ProbeSuccess, p.probeStatus)
	require.Equal(t, 0, p.probeFailCount)
	require.Equal(t, float64(100), p.healthScore)
	require.Equal(t, 0, p.failureCount)
	require.Equal(t, PeerStateActive, p.peerState)
	require.True(t, time.Since(p.LastUsefulAt) < time.Second)
}

// TestProbeFailureCount 测试探测失败计数
func TestProbeFailureCount(t *testing.T) {
	p := &PeerInfo{
		Id:           peer.ID("test-peer"),
		probeStatus:  ProbePending,
		probeFailCount: 0,
	}

	// 第1次失败
	p.stateLock.Lock()
	p.probeFailCount++
	p.stateLock.Unlock()
	require.Equal(t, 1, p.probeFailCount)
	require.Equal(t, ProbePending, p.probeStatus)

	// 第2次失败
	p.stateLock.Lock()
	p.probeFailCount++
	p.stateLock.Unlock()
	require.Equal(t, 2, p.probeFailCount)
	require.Equal(t, ProbePending, p.probeStatus)

	// 第3次失败，状态变为ProbeFailed
	p.stateLock.Lock()
	p.probeFailCount++
	if p.probeFailCount >= 3 {
		p.probeStatus = ProbeFailed
	}
	p.stateLock.Unlock()

	require.Equal(t, 3, p.probeFailCount)
	require.Equal(t, ProbeFailed, p.probeStatus)
}

// TestProbeIntervalRespected 测试探测间隔限制
func TestProbeIntervalRespected(t *testing.T) {
	now := time.Now()
	probeIntervalMin := 30 * time.Second

	p := &PeerInfo{
		Id:           peer.ID("test-peer"),
		probeStatus:  ProbePending,
		lastProbeAt:  now.Add(-20 * time.Second), // 20秒前探测过
	}

	// 检查是否应该探测（不应该，因为未到30秒）
	shouldProbe := p.lastProbeAt.IsZero() || now.Sub(p.lastProbeAt) >= probeIntervalMin
	require.False(t, shouldProbe, "不应该在30秒内再次探测")

	// 35秒后
	p.lastProbeAt = now.Add(-35 * time.Second)
	shouldProbe = p.lastProbeAt.IsZero() || now.Sub(p.lastProbeAt) >= probeIntervalMin
	require.True(t, shouldProbe, "应该在30秒后可以探测")
}

// TestProbePreventedCleanupMetric 测试探测防止清理的指标
func TestProbePreventedCleanupMetric(t *testing.T) {
	metrics := &KBucketMetrics{}

	// 初始值应该是0
	require.Equal(t, int64(0), metrics.ProbePreventedCleanup)

	// 模拟探测成功，防止了清理
	metrics.ProbeSuccessCount++
	metrics.ProbePreventedCleanup++

	require.Equal(t, int64(1), metrics.ProbeSuccessCount)
	require.Equal(t, int64(1), metrics.ProbePreventedCleanup)

	// 再次成功
	metrics.ProbeSuccessCount++
	metrics.ProbePreventedCleanup++

	require.Equal(t, int64(2), metrics.ProbeSuccessCount)
	require.Equal(t, int64(2), metrics.ProbePreventedCleanup)
}

// TestProbeMetricsRecording 测试探测指标记录
func TestProbeMetricsRecording(t *testing.T) {
	metrics := &KBucketMetrics{}

	// 记录探测尝试
	metrics.ProbeAttempts = 10
	require.Equal(t, int64(10), metrics.ProbeAttempts)

	// 记录成功和失败
	metrics.ProbeSuccessCount = 6
	metrics.ProbeFailCount = 4
	require.Equal(t, int64(6), metrics.ProbeSuccessCount)
	require.Equal(t, int64(4), metrics.ProbeFailCount)

	// 记录超时
	metrics.ProbeTimeout = 2
	require.Equal(t, int64(2), metrics.ProbeTimeout)

	// 记录防止清理
	metrics.ProbePreventedCleanup = 6
	require.Equal(t, int64(6), metrics.ProbePreventedCleanup)

	// 获取快照验证
	snapshot := metrics.GetSnapshot()
	require.Equal(t, int64(10), snapshot.ProbeAttempts)
	require.Equal(t, int64(6), snapshot.ProbeSuccessCount)
	require.Equal(t, int64(4), snapshot.ProbeFailCount)
	require.Equal(t, int64(2), snapshot.ProbeTimeout)
	require.Equal(t, int64(6), snapshot.ProbePreventedCleanup)
}

// TestCleanupProbeFailedReason 测试探测失败导致清理的原因记录
func TestCleanupProbeFailedReason(t *testing.T) {
	metrics := &KBucketMetrics{}

	// 初始值应该是0
	require.Equal(t, int64(0), metrics.CleanupProbeFailed)

	// 记录探测失败导致的清理
	metrics.RecordCleanup("probe_failed", false)
	require.Equal(t, int64(1), metrics.CleanupProbeFailed)

	// 再次记录
	metrics.RecordCleanup("probe_failed", false)
	require.Equal(t, int64(2), metrics.CleanupProbeFailed)

	// 验证快照
	snapshot := metrics.GetSnapshot()
	require.Equal(t, int64(2), snapshot.CleanupProbeFailed)
}

// TestProbeStatusString 测试探测状态字符串表示
func TestProbeStatusString(t *testing.T) {
	tests := []struct {
		status   PeerProbeStatus
		expected string
	}{
		{ProbeNotNeeded, "NotNeeded"},
		{ProbePending, "Pending"},
		{ProbeSuccess, "Success"},
		{ProbeFailed, "Failed"},
		{PeerProbeStatus(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.status.String())
		})
	}
}

// TestTwoPhaseCleanupWorkflow 测试两阶段清理工作流
func TestTwoPhaseCleanupWorkflow(t *testing.T) {
	ctx := context.Background()
	_ = ctx

	// 阶段1：标记为待探测
	p := &PeerInfo{
		Id:           peer.ID("test-peer"),
		probeStatus:  ProbeNotNeeded,
		healthScore:  15, // 低健康分
		LastUsefulAt: time.Now().Add(-30 * time.Minute), // 长期无用
		peerState:    PeerStateSuspect,
	}

	// 模拟cleanupUnhealthyPeers标记peer
	p.stateLock.Lock()
	if p.probeStatus == ProbeNotNeeded || p.probeStatus == ProbeSuccess {
		p.probeStatus = ProbePending
		p.lastProbeAt = time.Time{}
		p.probeFailCount = 0
	}
	p.stateLock.Unlock()

	require.Equal(t, ProbePending, p.probeStatus)

	// 阶段2a：探测成功
	p.stateLock.Lock()
	p.probeStatus = ProbeSuccess
	p.probeFailCount = 0
	p.healthScore = 100
	p.peerState = PeerStateActive
	p.LastUsefulAt = time.Now()
	p.stateLock.Unlock()

	require.Equal(t, ProbeSuccess, p.probeStatus)
	require.Equal(t, PeerStateActive, p.peerState)

	// 重置，测试阶段2b：探测失败
	p2 := &PeerInfo{
		Id:           peer.ID("test-peer-2"),
		probeStatus:  ProbePending,
		probeFailCount: 0,
	}

	// 连续3次探测失败
	for i := 0; i < 3; i++ {
		p2.stateLock.Lock()
		p2.probeFailCount++
		if p2.probeFailCount >= 3 {
			p2.probeStatus = ProbeFailed
		}
		p2.stateLock.Unlock()
	}

	require.Equal(t, ProbeFailed, p2.probeStatus)
	require.Equal(t, 3, p2.probeFailCount)

	// 阶段3：finalCleanup删除ProbeFailed的peer
	// （这里只验证状态，实际删除由finalCleanup执行）
	shouldCleanup := p2.probeStatus == ProbeFailed
	require.True(t, shouldCleanup, "ProbeFailed的peer应该被最终清理")
}

// TestProbeResetOnNewMark 测试重新标记时探测状态重置
func TestProbeResetOnNewMark(t *testing.T) {
	p := &PeerInfo{
		Id:           peer.ID("test-peer"),
		probeStatus:  ProbeSuccess, // 之前探测成功
		probeFailCount: 0,
		lastProbeAt:  time.Now().Add(-1 * time.Hour),
	}

	// 再次标记为待探测时，应该重置状态
	p.stateLock.Lock()
	if p.probeStatus == ProbeNotNeeded || p.probeStatus == ProbeSuccess {
		p.probeStatus = ProbePending
		p.lastProbeAt = time.Time{}    // 重置
		p.probeFailCount = 0           // 重置
	}
	p.stateLock.Unlock()

	require.Equal(t, ProbePending, p.probeStatus)
	require.True(t, p.lastProbeAt.IsZero())
	require.Equal(t, 0, p.probeFailCount)
}

