package quorum

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// MiningQuorumState 挖矿门闸状态（Gauge）
	// 状态值映射：
	// - 0: NotStarted
	// - 1: Discovering
	// - 2: QuorumPending
	// - 3: QuorumReached
	// - 4: HeightAligned
	// - 5: HeightConflict
	// - 6: Isolated
	MiningQuorumState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "weisyn_mining_quorum_state",
			Help: "挖矿门闸状态（0=NotStarted, 1=Discovering, 2=QuorumPending, 3=QuorumReached, 4=HeightAligned, 5=HeightConflict, 6=Isolated）",
		},
		[]string{"node_id"},
	)

	// MiningQuorumPeers 挖矿门闸 peer 指标（Gauge）
	MiningQuorumPeers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "weisyn_mining_quorum_peers",
			Help: "挖矿门闸 peer 数量（discovered/connected/qualified/required/current）",
		},
		[]string{"node_id", "type"}, // type: discovered, connected, qualified, required, current
	)

	// MiningQuorumHeightSkew 挖矿门闸高度偏差（Gauge）
	MiningQuorumHeightSkew = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "weisyn_mining_quorum_height_skew",
			Help: "挖矿门闸高度偏差（local_height - median_peer_height）",
		},
		[]string{"node_id"},
	)

	// MiningQuorumCheckTotal 挖矿门闸检查次数（Counter）
	MiningQuorumCheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "weisyn_mining_quorum_check_total",
			Help: "挖矿门闸检查总次数",
		},
		[]string{"node_id", "result"}, // result: allowed, blocked
	)

	// MiningQuorumTipAge 链尖年龄（Gauge，秒）
	MiningQuorumTipAge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "weisyn_mining_quorum_tip_age_seconds",
			Help: "链尖年龄（秒）",
		},
		[]string{"node_id"},
	)
)

func init() {
	// 注册所有指标
	prometheus.MustRegister(MiningQuorumState)
	prometheus.MustRegister(MiningQuorumPeers)
	prometheus.MustRegister(MiningQuorumHeightSkew)
	prometheus.MustRegister(MiningQuorumCheckTotal)
	prometheus.MustRegister(MiningQuorumTipAge)
}

// stateToValue 将 NetworkQuorumState 转换为数值（用于 Prometheus Gauge）
func stateToValue(state NetworkQuorumState) float64 {
	switch state {
	case StateNotStarted:
		return 0
	case StateDiscovering:
		return 1
	case StateQuorumPending:
		return 2
	case StateQuorumReached:
		return 3
	case StateHeightAligned:
		return 4
	case StateHeightConflict:
		return 5
	case StateIsolated:
		return 6
	default:
		return -1
	}
}

// updateMetrics 更新 Prometheus 指标
func updateMetrics(nodeID string, res *Result) {
	if res == nil {
		return
	}

	// 更新状态
	MiningQuorumState.WithLabelValues(nodeID).Set(stateToValue(res.State))

	// 更新 peer 指标
	MiningQuorumPeers.WithLabelValues(nodeID, "discovered").Set(float64(res.Metrics.DiscoveredPeers))
	MiningQuorumPeers.WithLabelValues(nodeID, "connected").Set(float64(res.Metrics.ConnectedPeers))
	MiningQuorumPeers.WithLabelValues(nodeID, "qualified").Set(float64(res.Metrics.QualifiedPeers))
	MiningQuorumPeers.WithLabelValues(nodeID, "required").Set(float64(res.Metrics.RequiredQuorumTotal))
	MiningQuorumPeers.WithLabelValues(nodeID, "current").Set(float64(res.Metrics.CurrentQuorumTotal))

	// 更新高度偏差
	MiningQuorumHeightSkew.WithLabelValues(nodeID).Set(float64(res.Metrics.HeightSkew))

	// 更新检查次数
	resultLabel := "blocked"
	if res.AllowMining {
		resultLabel = "allowed"
	}
	MiningQuorumCheckTotal.WithLabelValues(nodeID, resultLabel).Inc()

	// 更新链尖年龄
	tipAgeSeconds := float64(res.ChainTip.TipAge.Seconds())
	MiningQuorumTipAge.WithLabelValues(nodeID).Set(tipAgeSeconds)
}

