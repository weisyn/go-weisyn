package types

// ==================== 业务状态枚举（保留） ====================

// Status 统一状态表示
type Status int

const (
	StatusUnknown Status = iota
	StatusPending
	StatusProcessing
	StatusConfirmed
	StatusFailed
	StatusRejected
)

// String 返回状态的字符串表示
func (s Status) String() string {
	switch s {
	case StatusUnknown:
		return "unknown"
	case StatusPending:
		return "pending"
	case StatusProcessing:
		return "processing"
	case StatusConfirmed:
		return "confirmed"
	case StatusFailed:
		return "failed"
	case StatusRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

// ==================== 时间戳类型（已移至common.go） ====================
//
// 注意：Timestamp类型已统一定义在common.go中
// 使用 type Timestamp time.Time 以提供完整的时间操作功能
