package p2p

import "time"

// DefaultOptions 返回默认 P2P 配置
func DefaultOptions() *Options {
	return &Options{
		Profile:                           ProfileServer,
		ListenAddrs:                       []string{"/ip4/0.0.0.0/tcp/28683", "/ip4/0.0.0.0/udp/28683/quic-v1"},
		BootstrapPeers:                    []string{},
		EnableDHT:                         true,
		DHTMode:                           "auto",
		EnableMDNS:                        false,
		EnableRelay:                       true,
		EnableRelayService:                false,
		EnableDCUTR:                       true,
		PrivateNetwork:                    false,
		PSKPath:                           "",
		CertificateManagementCABundlePath: "",
		MinPeers:                          4,
		MaxPeers:                          50,
		DiagnosticsEnabled:                false,
		DiagnosticsAddr:                   "127.0.0.1:28686",
		
		// Phase 3: 发现间隔收敛配置（不向后兼容）
		DiscoveryMaxIntervalCap:           2 * time.Minute,
		DHTSteadyIntervalCap:              2 * time.Minute,
		DiscoveryResetMinInterval:         30 * time.Second,
		DiscoveryResetCoolDown:            10 * time.Second,
		
		// Phase 4: 关键peer监控配置（不向后兼容）
		EnableKeyPeerMonitor:              true,
		KeyPeerProbeInterval:              60 * time.Second,
		PerPeerMinProbeInterval:           30 * time.Second,
		ProbeTimeout:                      5 * time.Second,
		ProbeFailThreshold:                3,
		ProbeMaxConcurrent:                5,
		KeyPeerSetMaxSize:                 128,

		// Phase 5: forceConnect（GossipSub 拉活）（不向后兼容）
		BusinessCriticalPeerIDs:       []string{},
		ForceConnectEnabled:           true,
		ForceConnectCooldown:          2 * time.Minute,
		ForceConnectConcurrency:       15,
		ForceConnectBudgetPerRound:    50,
		ForceConnectTier2SampleBudget: 20,
		ForceConnectTimeout:           10 * time.Second,

		// 🆕 Phase 6: 网络超时和健康检查配置（HIGH-003 修复）
		NetworkTimeoutConfig: NetworkTimeoutConfig{
			DialTimeout:           15 * time.Second,
			StreamOpenTimeout:     10 * time.Second,
			StreamReadTimeout:     30 * time.Second,
			StreamWriteTimeout:    30 * time.Second,
			EnableDynamicTimeout:  true,
			MinTimeout:            5 * time.Second,
			MaxTimeout:            60 * time.Second,
			TimeoutIncreaseFactor: 1.5,
			TimeoutDecreaseFactor: 0.9,
			MaxRetries:            3,
			RetryBackoffBase:      1 * time.Second,
			RetryBackoffMax:       30 * time.Second,
			RetryBackoffFactor:    2.0,
		},
		NetworkHealthConfig: NetworkHealthConfig{
			Enabled:                true,
			CheckInterval:          30 * time.Second,
			UnhealthyThreshold:     3,
			HealthyThreshold:       2,
			TimeoutRatioThreshold:  0.3,
			EnableAutoHealing:      true,
			HealingCooldown:        1 * time.Minute,
			MaxHealingAttempts:     5,
			ConnectionCheckEnabled: true,
			ConnectionCheckTimeout: 5 * time.Second,
			MaxIdleConnections:     50,
			IdleConnectionTimeout:  5 * time.Minute,
		},
	}
}
