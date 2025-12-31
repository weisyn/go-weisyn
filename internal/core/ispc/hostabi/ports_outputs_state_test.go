package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// ports_outputs_state.go 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 AppendStateOutput 的缺陷和BUG
//
// ============================================================================

// TestAppendStateOutput_Success 测试成功追加状态输出
func TestAppendStateOutput_Success(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	stateID := make([]byte, 20)
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 32)
	publicInputs := make([]byte, 64)
	parentStateHash := make([]byte, 32)

	idx, err := hostABI.AppendStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)

	assert.NoError(t, err, "应该成功追加状态输出")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
}

// TestAppendStateOutput_EmptyPublicInputs 测试空公开输入
func TestAppendStateOutput_EmptyPublicInputs(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	stateID := make([]byte, 20)
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 32)
	publicInputs := []byte{} // 空公开输入
	parentStateHash := make([]byte, 32)

	idx, err := hostABI.AppendStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)

	assert.NoError(t, err, "应该成功追加状态输出（空公开输入）")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
}

// TestAppendStateOutput_NilParentStateHash 测试nil父状态哈希
func TestAppendStateOutput_NilParentStateHash(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	stateID := make([]byte, 20)
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 32)
	publicInputs := make([]byte, 64)
	parentStateHash := []byte(nil) // nil父状态哈希

	idx, err := hostABI.AppendStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)

	assert.NoError(t, err, "应该成功追加状态输出（nil父状态哈希）")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
}

// TestAppendStateOutput_EmptyDraftID 测试空草稿ID
func TestAppendStateOutput_EmptyDraftID(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "", // 空草稿ID
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	stateID := make([]byte, 20)
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 32)

	idx, err := hostABI.AppendStateOutput(ctx, stateID, stateVersion, executionResultHash, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "获取草稿ID失败", "错误信息应该正确")
}

// TestAppendStateOutput_LoadDraftFailed 测试加载草稿失败
func TestAppendStateOutput_LoadDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
	}
	mockDraftService := &mockDraftServiceForPorts{
		loadDraftError: assert.AnError, // 加载草稿失败
	}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	stateID := make([]byte, 20)
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 32)

	idx, err := hostABI.AppendStateOutput(ctx, stateID, stateVersion, executionResultHash, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "加载交易草稿失败", "错误信息应该正确")
}

// TestAppendStateOutput_AddStateOutputFailed 测试添加状态输出失败
func TestAppendStateOutput_AddStateOutputFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
	}
	mockDraftService := &mockDraftServiceForPorts{
		addStateOutputError: assert.AnError, // 添加输出失败
	}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	stateID := make([]byte, 20)
	stateVersion := uint64(1)
	executionResultHash := make([]byte, 32)

	idx, err := hostABI.AppendStateOutput(ctx, stateID, stateVersion, executionResultHash, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "追加状态输出失败", "错误信息应该正确")
}


