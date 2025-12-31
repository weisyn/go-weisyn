// Package types 定义共识相关的错误类型
package types

import (
	"fmt"
)

// WaiverError 弃权错误
//
// V2 新增：用于表示聚合器节点弃权，而非普通错误
// 网络层会检测此错误类型并构建弃权响应（AggregatorBlockAcceptance.waived=true）
type WaiverError struct {
	Reason      WaiverReason // 弃权原因
	LocalHeight uint64       // 本节点当前高度（用于诊断）
	Height      uint64       // 候选区块高度
}

// WaiverReason 弃权原因枚举
type WaiverReason int32

const (
	WaiverReasonNone WaiverReason = iota
	WaiverReasonHeightTooFarAhead
	WaiverReasonAggregationInProgress
	WaiverReasonReadOnlyMode // 只读模式弃权
)

// Error 实现 error 接口
func (e *WaiverError) Error() string {
	switch e.Reason {
	case WaiverReasonHeightTooFarAhead:
		return fmt.Sprintf("waiver: height too far ahead (candidate=%d local=%d)", e.Height, e.LocalHeight)
	case WaiverReasonAggregationInProgress:
		return fmt.Sprintf("waiver: aggregation in progress (candidate=%d)", e.Height)
	case WaiverReasonReadOnlyMode:
		return fmt.Sprintf("waiver: node in read-only mode (candidate=%d local=%d)", e.Height, e.LocalHeight)
	default:
		return fmt.Sprintf("waiver: unknown reason (candidate=%d)", e.Height)
	}
}

// IsWaiverError 检查错误是否为弃权错误
func IsWaiverError(err error) (*WaiverError, bool) {
	if err == nil {
		return nil, false
	}
	waiverErr, ok := err.(*WaiverError)
	return waiverErr, ok
}
