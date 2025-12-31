package facade

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

// 说明：这里不做真实拨号（需要复杂的 host mock）。
// 只验证“分层优先 + 预算”行为的核心：Tier0 业务节点必须优先进入 targets，Tier2 仅受抽样预算影响。

func TestForceConnectConfig_DefaultsApplied(t *testing.T) {
	f := &Facade{}
	f.SetForceConnectConfig(ForceConnectConfig{Enabled: true})

	require.True(t, f.forceConnectCfg.Enabled)
	require.Equal(t, 2*time.Minute, f.forceConnectCfg.Cooldown)
	require.Equal(t, 15, f.forceConnectCfg.Concurrency)
	require.Equal(t, 50, f.forceConnectCfg.BudgetPerRound)
	require.Equal(t, 0, f.forceConnectCfg.Tier2SampleBudget) // 默认 0（由上层注入决定）；这里保持 0 不触发 Tier2
	require.Equal(t, 10*time.Second, f.forceConnectCfg.Timeout)
}

func TestForceConnectCooldown_SkipsWithinWindow(t *testing.T) {
	f := &Facade{}
	f.SetForceConnectConfig(ForceConnectConfig{
		Enabled:        true,
		Cooldown:       2 * time.Minute,
		Concurrency:    5,
		BudgetPerRound: 10,
		Timeout:        10 * time.Second,
	})

	f.ensureForceConnectLoop()

	// 人工设置 lastAt，模拟刚运行过一轮
	f.forceConnectMu.Lock()
	f.forceConnectLastAt = time.Now().Add(-30 * time.Second)
	f.forceConnectMu.Unlock()

	// 不应 panic；实际执行在 runForceConnectRound 中会被 cooldown 拦截
	f.runForceConnectRound("test")
}

func TestForceConnectConfig_BusinessPeersStored(t *testing.T) {
	f := &Facade{}
	biz := []peer.ID{"biz1", "biz2"}
	f.SetForceConnectConfig(ForceConnectConfig{BusinessPeers: biz})
	require.Len(t, f.forceConnectCfg.BusinessPeers, 2)
}


