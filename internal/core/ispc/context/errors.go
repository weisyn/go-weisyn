// Package context provides error definitions for ISPC execution context operations.
package context

import (
	"errors"
	"fmt"
)

// ============================================================================
//                            执行上下文错误定义
// ============================================================================

var (
	// ErrContextNotFound 执行上下文未找到错误
	ErrContextNotFound = errors.New("execution context not found")

	// ErrContextExpired 执行上下文已过期错误
	ErrContextExpired = errors.New("execution context expired")

	// ErrContextCreationFailed 执行上下文创建失败错误
	ErrContextCreationFailed = errors.New("execution context creation failed")

	// ErrContextDestroyFailed 执行上下文销毁失败错误
	ErrContextDestroyFailed = errors.New("execution context destroy failed")

	// ErrInvalidContextID 无效的上下文ID错误
	ErrInvalidContextID = errors.New("invalid context ID")

	// ErrContextAlreadyExists 上下文已存在错误
	ErrContextAlreadyExists = errors.New("execution context already exists")

	// ErrTransactionDraftNotFound 交易草稿未找到错误
	ErrTransactionDraftNotFound = errors.New("transaction draft not found")

	// ErrTransactionDraftCreationFailed 交易草稿创建失败错误
	ErrTransactionDraftCreationFailed = errors.New("transaction draft creation failed")

	// ErrInvalidTransactionDraft 无效的交易草稿错误
	ErrInvalidTransactionDraft = errors.New("invalid transaction draft")

	// ErrContextStateMismatch 上下文状态不匹配错误
	ErrContextStateMismatch = errors.New("context state mismatch")
)

// ============================================================================
//                               错误包装函数
// ============================================================================

// WrapContextNotFoundError 包装上下文未找到错误
func WrapContextNotFoundError(contextID string) error {
	return fmt.Errorf("%w: contextID=%s", ErrContextNotFound, contextID)
}

// WrapContextExpiredError 包装上下文已过期错误
func WrapContextExpiredError(contextID string) error {
	return fmt.Errorf("%w: contextID=%s", ErrContextExpired, contextID)
}

// WrapContextCreationFailedError 包装上下文创建失败错误
func WrapContextCreationFailedError(contextID string, err error) error {
	return fmt.Errorf("%w: contextID=%s, cause=%v", ErrContextCreationFailed, contextID, err)
}

// WrapContextDestroyFailedError 包装上下文销毁失败错误
func WrapContextDestroyFailedError(contextID string, err error) error {
	return fmt.Errorf("%w: contextID=%s, cause=%v", ErrContextDestroyFailed, contextID, err)
}

// WrapContextAlreadyExistsError 包装上下文已存在错误
func WrapContextAlreadyExistsError(contextID string) error {
	return fmt.Errorf("%w: contextID=%s", ErrContextAlreadyExists, contextID)
}

// WrapTransactionDraftNotFoundError 包装交易草稿未找到错误
func WrapTransactionDraftNotFoundError(draftID string) error {
	return fmt.Errorf("%w: draftID=%s", ErrTransactionDraftNotFound, draftID)
}

// WrapTransactionDraftCreationFailedError 包装交易草稿创建失败错误
func WrapTransactionDraftCreationFailedError(draftID string, err error) error {
	return fmt.Errorf("%w: draftID=%s, cause=%v", ErrTransactionDraftCreationFailed, draftID, err)
}

// WrapInvalidTransactionDraftError 包装无效交易草稿错误
func WrapInvalidTransactionDraftError(draftID, reason string) error {
	return fmt.Errorf("%w: draftID=%s, reason=%s", ErrInvalidTransactionDraft, draftID, reason)
}

// WrapContextStateMismatchError 包装上下文状态不匹配错误
func WrapContextStateMismatchError(contextID, expected, actual string) error {
	return fmt.Errorf("%w: contextID=%s, expected=%s, actual=%s", ErrContextStateMismatch, contextID, expected, actual)
}
