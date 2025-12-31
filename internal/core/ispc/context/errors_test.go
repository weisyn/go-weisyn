package context

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// errors.go 测试
// ============================================================================

// TestErrorConstants 测试错误常量
func TestErrorConstants(t *testing.T) {
	assert.NotNil(t, ErrContextNotFound)
	assert.NotNil(t, ErrContextExpired)
	assert.NotNil(t, ErrContextCreationFailed)
	assert.NotNil(t, ErrContextDestroyFailed)
	assert.NotNil(t, ErrInvalidContextID)
	assert.NotNil(t, ErrContextAlreadyExists)
	assert.NotNil(t, ErrTransactionDraftNotFound)
	assert.NotNil(t, ErrTransactionDraftCreationFailed)
	assert.NotNil(t, ErrInvalidTransactionDraft)
	assert.NotNil(t, ErrContextStateMismatch)
}

// TestWrapContextNotFoundError 测试包装上下文未找到错误
func TestWrapContextNotFoundError(t *testing.T) {
	contextID := "test_context_id"
	err := WrapContextNotFoundError(contextID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), contextID)
	assert.True(t, errors.Is(err, ErrContextNotFound))
}

// TestWrapContextExpiredError 测试包装上下文已过期错误
func TestWrapContextExpiredError(t *testing.T) {
	contextID := "test_context_id"
	err := WrapContextExpiredError(contextID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), contextID)
	assert.True(t, errors.Is(err, ErrContextExpired))
}

// TestWrapContextCreationFailedError 测试包装上下文创建失败错误
func TestWrapContextCreationFailedError(t *testing.T) {
	contextID := "test_context_id"
	cause := errors.New("underlying error")
	err := WrapContextCreationFailedError(contextID, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), contextID)
	assert.True(t, errors.Is(err, ErrContextCreationFailed))
}

// TestWrapContextDestroyFailedError 测试包装上下文销毁失败错误
func TestWrapContextDestroyFailedError(t *testing.T) {
	contextID := "test_context_id"
	cause := errors.New("underlying error")
	err := WrapContextDestroyFailedError(contextID, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), contextID)
	assert.True(t, errors.Is(err, ErrContextDestroyFailed))
}

// 注意：WrapInvalidContextIDError 函数不存在于 errors.go 中

// TestWrapContextAlreadyExistsError 测试包装上下文已存在错误
func TestWrapContextAlreadyExistsError(t *testing.T) {
	contextID := "existing_context_id"
	err := WrapContextAlreadyExistsError(contextID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), contextID)
	assert.True(t, errors.Is(err, ErrContextAlreadyExists))
}

// TestWrapTransactionDraftNotFoundError 测试包装交易草稿未找到错误
func TestWrapTransactionDraftNotFoundError(t *testing.T) {
	draftID := "test_draft_id"
	err := WrapTransactionDraftNotFoundError(draftID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), draftID)
	assert.True(t, errors.Is(err, ErrTransactionDraftNotFound))
}

// TestWrapTransactionDraftCreationFailedError 测试包装交易草稿创建失败错误
func TestWrapTransactionDraftCreationFailedError(t *testing.T) {
	draftID := "test_draft_id"
	cause := errors.New("underlying error")
	err := WrapTransactionDraftCreationFailedError(draftID, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), draftID)
	assert.True(t, errors.Is(err, ErrTransactionDraftCreationFailed))
}

// TestWrapInvalidTransactionDraftError 测试包装无效交易草稿错误
func TestWrapInvalidTransactionDraftError(t *testing.T) {
	draftID := "invalid_draft_id"
	reason := "invalid reason"
	err := WrapInvalidTransactionDraftError(draftID, reason)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), draftID)
	assert.Contains(t, err.Error(), reason)
	assert.True(t, errors.Is(err, ErrInvalidTransactionDraft))
}

// TestWrapContextStateMismatchError 测试包装上下文状态不匹配错误
func TestWrapContextStateMismatchError(t *testing.T) {
	contextID := "test_context_id"
	expectedState := "expected_state"
	actualState := "actual_state"
	err := WrapContextStateMismatchError(contextID, expectedState, actualState)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), contextID)
	assert.Contains(t, err.Error(), expectedState)
	assert.Contains(t, err.Error(), actualState)
	assert.True(t, errors.Is(err, ErrContextStateMismatch))
}

