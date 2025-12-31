package quorum

import consensusconfig "github.com/weisyn/v1/internal/config/consensus"

// NewMinerConfigView 从共识 MinerConfig 构造门闸配置视图。
func NewMinerConfigView(cfg *consensusconfig.MinerConfig) MinerConfigView {
	return minerConfigView{cfg: cfg}
}

type minerConfigView struct {
	cfg *consensusconfig.MinerConfig
}

func (v minerConfigView) GetMinNetworkQuorumTotal() int {
	if v.cfg == nil {
		return 0
	}
	return v.cfg.MinNetworkQuorumTotal
}
func (v minerConfigView) GetAllowSingleNodeMining() bool {
	if v.cfg == nil {
		return false
	}
	return v.cfg.AllowSingleNodeMining
}
func (v minerConfigView) GetNetworkDiscoveryTimeoutSeconds() int {
	if v.cfg == nil {
		return 0
	}
	return v.cfg.NetworkDiscoveryTimeoutSeconds
}
func (v minerConfigView) GetQuorumRecoveryTimeoutSeconds() int {
	if v.cfg == nil {
		return 0
	}
	return v.cfg.QuorumRecoveryTimeoutSeconds
}
func (v minerConfigView) GetMaxHeightSkew() uint64 {
	if v.cfg == nil {
		return 0
	}
	return v.cfg.MaxHeightSkew
}
func (v minerConfigView) GetMaxTipStalenessSeconds() uint64 {
	if v.cfg == nil {
		return 0
	}
	return v.cfg.MaxTipStalenessSeconds
}
func (v minerConfigView) GetEnableTipFreshnessCheck() bool {
	if v.cfg == nil {
		return true
	}
	return v.cfg.EnableTipFreshnessCheck
}

func (v minerConfigView) GetEnableNetworkAlignmentCheck() bool {
	if v.cfg == nil {
		return true
	}
	return v.cfg.EnableNetworkAlignmentCheck
}


