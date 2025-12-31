package types

import "time"

// KBucketLastAdd 记录最近一次“尝试将 peer 加入 K桶”的结果与原因，用于线上诊断。
type KBucketLastAdd struct {
	PeerID string      `json:"peer_id"`
	At     RFC3339Time `json:"at"`

	// Result: added | already_exists | rejected | error | bucket_full
	Result string `json:"result"`

	// Reason: weisyn_proto | not_wes | chain_mismatch | wes_check_error | bucket_full | unknown
	Reason string `json:"reason"`

	Error string `json:"error,omitempty"`
}

// NewKBucketLastAdd 构造并规范化时间字段
func NewKBucketLastAdd(peerID string, at time.Time, result, reason, err string) KBucketLastAdd {
	return KBucketLastAdd{
		PeerID: peerID,
		At:     RFC3339Time(at),
		Result: result,
		Reason: reason,
		Error:  err,
	}
}

// KBucketSummary 是 K桶路由表的高信号摘要信息，便于快速判断“空桶风险”。
type KBucketSummary struct {
	TotalPeers   int             `json:"total_peers"`
	HealthyPeers int             `json:"healthy_peers"`
	LastAdd      *KBucketLastAdd `json:"last_add,omitempty"`
}


