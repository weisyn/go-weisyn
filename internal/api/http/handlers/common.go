// Package handlers provides HTTP API handlers for theWES blockchain
package handlers

// ==================== 📋 标准API响应结构 ====================

// StandardAPIResponse 标准API响应格式
// ✅ 统一所有handler的响应格式，提供一致的用户体验
type StandardAPIResponse struct {
	Success bool        `json:"success"`           // 操作是否成功
	Data    interface{} `json:"data,omitempty"`    // 响应数据（成功时）
	Message string      `json:"message,omitempty"` // 成功消息或简要说明
	Error   *APIError   `json:"error,omitempty"`   // 错误信息（失败时）
}

// APIError 标准错误结构
type APIError struct {
	Code    string `json:"code"`              // 错误代码（用于程序化处理）
	Message string `json:"message"`           // 用户友好的错误消息
	Details string `json:"details,omitempty"` // 详细错误信息（调试用）
}

// ==================== 🎯 通用错误代码常量 ====================

// 请求相关错误
const (
	ErrorCodeInvalidRequest   = "INVALID_REQUEST"
	ErrorCodeInvalidParameter = "INVALID_PARAMETER"
	ErrorCodeMissingParameter = "MISSING_PARAMETER"
	ErrorCodeInvalidJSON      = "INVALID_JSON"
)

// 地址和身份相关错误
const (
	ErrorCodeInvalidAddress    = "INVALID_ADDRESS"
	ErrorCodeInvalidPublicKey  = "INVALID_PUBLIC_KEY"
	ErrorCodeInvalidPrivateKey = "INVALID_PRIVATE_KEY"
)

// 数据格式相关错误
const (
	ErrorCodeInvalidAmount    = "INVALID_AMOUNT"
	ErrorCodeInvalidHash      = "INVALID_HASH"
	ErrorCodeInvalidTokenID   = "INVALID_TOKEN_ID"
	ErrorCodeInvalidHeight    = "INVALID_HEIGHT"
	ErrorCodeInvalidTimestamp = "INVALID_TIMESTAMP"
)

// 业务逻辑相关错误
const (
	ErrorCodeTransactionNotFound = "TRANSACTION_NOT_FOUND"
	ErrorCodeInsufficientBalance = "INSUFFICIENT_BALANCE"
	ErrorCodeBlockNotFound       = "BLOCK_NOT_FOUND"
	ErrorCodeAccountNotFound     = "ACCOUNT_NOT_FOUND"
	ErrorCodeSessionNotFound     = "SESSION_NOT_FOUND"
)

// 系统相关错误
const (
	ErrorCodeNetworkError       = "NETWORK_ERROR"
	ErrorCodeInternalError      = "INTERNAL_ERROR"
	ErrorCodeTimeout            = "TIMEOUT"
	ErrorCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// 挖矿相关错误
const (
	ErrorCodeMiningNotStarted     = "MINING_NOT_STARTED"
	ErrorCodeMiningAlreadyRunning = "MINING_ALREADY_RUNNING"
	ErrorCodeMiningFailed         = "MINING_FAILED"
)
