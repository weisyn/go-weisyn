package types

// TxStatus 表示交易在池中的状态（从 pkg/interfaces/mempool 迁移）
type TxStatus int

const (
	TxStatusUnknown   TxStatus = iota // 未知状态
	TxStatusPending                   // 等待处理(已验证但未打包)
	TxStatusIncluded                  // 已包含在池中(等待验证)
	TxStatusConfirmed                 // 已确认(已打包进区块)
	TxStatusRejected                  // 被拒绝(验证失败)
	TxStatusExpired                   // 已过期(超过生存时间)
)

// String 返回TxStatus的字符串表示
func (s TxStatus) String() string {
	switch s {
	case TxStatusUnknown:
		return "Unknown"
	case TxStatusPending:
		return "Pending"
	case TxStatusIncluded:
		return "Included"
	case TxStatusConfirmed:
		return "Confirmed"
	case TxStatusRejected:
		return "Rejected"
	case TxStatusExpired:
		return "Expired"
	default:
		return "Invalid"
	}
}
