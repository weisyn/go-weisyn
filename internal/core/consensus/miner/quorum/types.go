package quorum

import (
	"time"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

// NetworkQuorumState 表示节点“网络确认/法定人数”状态（V2）。
type NetworkQuorumState string

const (
	StateNotStarted     NetworkQuorumState = "NotStarted"
	StateDiscovering    NetworkQuorumState = "Discovering"
	StateQuorumPending  NetworkQuorumState = "QuorumPending"
	StateQuorumReached  NetworkQuorumState = "QuorumReached"
	StateHeightAligned  NetworkQuorumState = "HeightAligned"
	StateHeightConflict NetworkQuorumState = "HeightConflict"
	StateIsolated       NetworkQuorumState = "Isolated"
)

// MinerConfigView 提供门闸检查所需的配置视图（避免依赖具体配置装配层）。
type MinerConfigView interface {
	GetMinNetworkQuorumTotal() int
	GetAllowSingleNodeMining() bool
	GetNetworkDiscoveryTimeoutSeconds() int
	GetQuorumRecoveryTimeoutSeconds() int
	GetMaxHeightSkew() uint64
	GetMaxTipStalenessSeconds() uint64
	GetEnableTipFreshnessCheck() bool
	GetEnableNetworkAlignmentCheck() bool
}

// ChainTipPrerequisite 链尖前置条件（V2）。
type ChainTipPrerequisite struct {
	TipReadable            bool
	TipTimestamp           uint64
	TipAge                 time.Duration
	TipFresh               bool
	TipHealthyForHandshake bool // 重命名：明确包含新鲜度检查的语义
}

// Metrics 为门闸决策提供最小观测口径（不对外暴露 API 时仍便于日志/错误信息）。
type Metrics struct {
	// peers 口径
	DiscoveredPeers int
	ConnectedPeers  int
	QualifiedPeers  int // connected && SyncHelloV2 可用 && 同链

	// quorum（含本机）
	RequiredQuorumTotal int
	CurrentQuorumTotal  int
	QuorumReached       bool

	// height
	LocalHeight      uint64
	PeerHeights      map[peer.ID]uint64
	MedianPeerHeight uint64
	HeightSkew       int64 // local - median

	// time
	DiscoveryStartedAt time.Time
	QuorumReachedAt    time.Time
}

// Result 是一次 Check 的决策结果。
type Result struct {
	State           NetworkQuorumState
	AllowMining     bool
	Reason          string
	SuggestedAction string

	Metrics  Metrics
	ChainTip ChainTipPrerequisite
}


