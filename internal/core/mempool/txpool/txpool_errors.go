// Package txpool provides unified error types, error codes, and classification tools for the transaction pool.
// 本文件定义交易池统一错误类型、错误码及分类判断与统计工具，
// 用于将基础验证/存储/网络/配置等错误进行分层与可观测化处理。
package txpool

import "fmt"

// =========================================================================
// 🚨 错误代码定义
// =========================================================================

// TxPoolErrorCode 交易池错误代码
type TxPoolErrorCode int

// 错误代码常量（分层域）
const (
	// ErrCodeInvalidConfig 配置相关错误
	ErrCodeInvalidConfig TxPoolErrorCode = 1000 + iota
	// ErrCodeMissingDependency 缺少依赖错误
	ErrCodeMissingDependency

	// ErrCodeAlreadyRunning 状态相关错误：已运行
	ErrCodeAlreadyRunning
	// ErrCodeNotRunning 状态相关错误：未运行
	ErrCodeNotRunning
	// ErrCodePoolClosed 状态相关错误：池已关闭
	ErrCodePoolClosed

	// ErrCodeInvalidFormat 基础验证错误：无效格式
	ErrCodeInvalidFormat
	// ErrCodeInvalidHash 基础验证错误：无效哈希
	ErrCodeInvalidHash
	// ErrCodeTxTooLarge 基础验证错误：交易过大
	ErrCodeTxTooLarge
	// ErrCodeDuplicateTx 基础验证错误：重复交易
	ErrCodeDuplicateTx
	// ErrCodeMemoryLimit 基础验证错误：内存限制
	ErrCodeMemoryLimit
	// ErrCodeComplianceViolation 基础验证错误：合规性违反
	ErrCodeComplianceViolation

	// ErrCodeTxNotFound 存储相关错误：交易未找到
	ErrCodeTxNotFound
	// ErrCodeTxExists 存储相关错误：交易已存在
	ErrCodeTxExists
	// ErrCodePoolFull 存储相关错误：池已满
	ErrCodePoolFull
	// ErrCodeStorageFailure 存储相关错误：存储失败
	ErrCodeStorageFailure

	// ErrCodeNetworkFailure 网络相关错误：网络失败
	ErrCodeNetworkFailure
	// ErrCodeTimeout 网络相关错误：超时
	ErrCodeTimeout
	// ErrCodeRateLimited 网络相关错误：速率限制
	ErrCodeRateLimited
)

// =========================================================================
// 🚨 错误类型定义
// =========================================================================

// TxPoolError 交易池统一错误类型（携带错误码、消息与底层原因）。
type TxPoolError struct {
	Code    TxPoolErrorCode
	Message string
	Cause   error
}

// Error 实现 error 接口。
func (e *TxPoolError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("TxPool错误[%d]: %s (原因: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("TxPool错误[%d]: %s", e.Code, e.Message)
}

// Unwrap 支持 errors.Unwrap。
func (e *TxPoolError) Unwrap() error { return e.Cause }

// Is 支持 errors.Is 比较（按错误码等价）。
func (e *TxPoolError) Is(target error) bool {
	if targetErr, ok := target.(*TxPoolError); ok {
		return e.Code == targetErr.Code
	}
	return false
}

// =========================================================================
// 🔧 错误构造与包装
// =========================================================================

// NewTxPoolError 创建新的 TxPool 错误。
func NewTxPoolError(code TxPoolErrorCode, message string, cause error) *TxPoolError {
	return &TxPoolError{Code: code, Message: message, Cause: cause}
}

// WrapTxPoolError 包装现有错误为 TxPool 错误。
func WrapTxPoolError(code TxPoolErrorCode, message string, err error) *TxPoolError {
	return &TxPoolError{Code: code, Message: message, Cause: err}
}

// =========================================================================
// 🎯 错误分类判断
// =========================================================================

// IsValidationError 检查是否为验证错误。
func IsValidationError(err error) bool {
	if txErr, ok := err.(*TxPoolError); ok {
		return txErr.Code >= ErrCodeInvalidFormat && txErr.Code <= ErrCodeComplianceViolation
	}
	return false
}

// IsStorageError 检查是否为存储错误。
func IsStorageError(err error) bool {
	if txErr, ok := err.(*TxPoolError); ok {
		return txErr.Code >= ErrCodeTxNotFound && txErr.Code <= ErrCodeStorageFailure
	}
	return false
}

// IsNetworkError 检查是否为网络错误。
func IsNetworkError(err error) bool {
	if txErr, ok := err.(*TxPoolError); ok {
		return txErr.Code >= ErrCodeNetworkFailure && txErr.Code <= ErrCodeRateLimited
	}
	return false
}

// =========================================================================
// 🔄 错误统计
// =========================================================================

// ErrorStats 错误统计信息。
type ErrorStats struct {
	ValidationErrors int64
	StorageErrors    int64
	NetworkErrors    int64
	ConfigErrors     int64
	OtherErrors      int64
}

// RecordError 记录错误到统计。
func (stats *ErrorStats) RecordError(err error) {
	if IsValidationError(err) {
		stats.ValidationErrors++
	} else if IsStorageError(err) {
		stats.StorageErrors++
	} else if IsNetworkError(err) {
		stats.NetworkErrors++
	} else if txErr, ok := err.(*TxPoolError); ok && (txErr.Code == ErrCodeInvalidConfig || txErr.Code == ErrCodeMissingDependency) {
		stats.ConfigErrors++
	} else {
		stats.OtherErrors++
	}
}
