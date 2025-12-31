// Package types provides HTTP error type definitions.
package types

// ErrorResponse 统一错误响应格式
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Code      string      `json:"code"`                // 错误码
	Message   string      `json:"message"`             // 错误消息
	Details   interface{} `json:"details,omitempty"`   // 详细信息
	RequestID string      `json:"requestId,omitempty"` // 请求ID
	Timestamp string      `json:"timestamp,omitempty"` // 时间戳
}

// 区块链特有错误码常量
const (
	// 通用错误码（400-499）
	ErrInvalidArgument   = "INVALID_ARGUMENT"
	ErrUnauthenticated   = "UNAUTHENTICATED"
	ErrPermissionDenied  = "PERMISSION_DENIED"
	ErrNotFound          = "NOT_FOUND"
	ErrRateLimitExceeded = "RATE_LIMIT_EXCEEDED"

	// 链/区块错误码（1000-1099）
	ErrChainSyncing      = "CHAIN_SYNCING"
	ErrChainReorganized  = "CHAIN_REORGANIZED"
	ErrBlockNotFound     = "BLOCK_NOT_FOUND"
	ErrInvalidBlockParam = "INVALID_BLOCK_PARAM"
	ErrBlockTooOld       = "BLOCK_TOO_OLD"

	// 交易错误码（2000-2099）
	ErrTxInvalidSignature  = "TX_INVALID_SIGNATURE"
	ErrTxFeeTooLow         = "TX_FEE_TOO_LOW"
	ErrTxAlreadyKnown      = "TX_ALREADY_KNOWN"
	ErrTxConflicts         = "TX_CONFLICTS"
	ErrTxPolicyRejected    = "TX_POLICY_REJECTED"
	ErrTxInsufficientFunds = "TX_INSUFFICIENT_FUNDS"
	ErrTxNonceError        = "TX_NONCE_ERROR"
	ErrTxTooLarge          = "TX_TOO_LARGE"

	// Mempool错误码（3000-3099）
	ErrMempoolFull    = "MEMPOOL_FULL"
	ErrMempoolEvicted = "MEMPOOL_EVICTED"

	// 执行/合约错误码（4000-4099）
	ErrExecutionReverted = "EXECUTION_REVERTED"
	ErrOutOfGas          = "OUT_OF_GAS"
	ErrContractNotFound  = "CONTRACT_NOT_FOUND"

	// 服务器错误码（500-599）
	ErrInternal           = "INTERNAL"
	ErrServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// NewErrorResponse 创建错误响应
func NewErrorResponse(code, message string, details interface{}) *ErrorResponse {
	return &ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// WithRequestID 添加请求ID
func (e *ErrorResponse) WithRequestID(requestID string) *ErrorResponse {
	e.Error.RequestID = requestID
	return e
}

// WithTimestamp 添加时间戳
func (e *ErrorResponse) WithTimestamp(timestamp string) *ErrorResponse {
	e.Error.Timestamp = timestamp
	return e
}

// 常用错误构造函数

// ErrBlockNotFoundResponse 区块不存在错误
func ErrBlockNotFoundResponse(heightOrHash interface{}) *ErrorResponse {
	return NewErrorResponse(
		ErrBlockNotFound,
		"Block not found",
		map[string]interface{}{
			"heightOrHash": heightOrHash,
		},
	)
}

// ErrTxFeeTooLowResponse 交易费过低错误
func ErrTxFeeTooLowResponse(provided, required string) *ErrorResponse {
	return NewErrorResponse(
		ErrTxFeeTooLow,
		"Transaction fee too low",
		map[string]interface{}{
			"providedFeeRate":    provided,
			"minRequiredFeeRate": required,
			"unit":               "sat/byte",
			"policyHint":         "请增加交易费率或使用 wes_estimateFee 获取建议费率",
		},
	)
}

// ErrInvalidSignatureResponse 无效签名错误
func ErrInvalidSignatureResponse(reason string) *ErrorResponse {
	return NewErrorResponse(
		ErrTxInvalidSignature,
		"Invalid transaction signature",
		map[string]interface{}{
			"reason": reason,
		},
	)
}

// ErrChainSyncingResponse 节点同步中错误
func ErrChainSyncingResponse() *ErrorResponse {
	return NewErrorResponse(
		ErrChainSyncing,
		"Node is syncing, please try again later",
		nil,
	)
}
