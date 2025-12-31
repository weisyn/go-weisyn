package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// ports_outputs_resource.go 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 AppendResourceOutput 的缺陷和BUG
//
// ============================================================================

// TestAppendResourceOutput_Success 测试成功追加资源输出
func TestAppendResourceOutput_Success(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	contentHash := make([]byte, 32)
	category := "wasm"
	owner := make([]byte, 20)
	metadata := []byte("test metadata")

	idx, err := hostABI.AppendResourceOutput(ctx, contentHash, category, owner, nil, metadata)

	assert.NoError(t, err, "应该成功追加资源输出")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
}

// TestAppendResourceOutput_WithLockingConditions 测试带锁定条件的资源输出
func TestAppendResourceOutput_WithLockingConditions(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	contentHash := make([]byte, 32)
	category := "onnx"
	owner := make([]byte, 20)
	lockingConditions := []*pb.LockingCondition{
		{
			Condition: &pb.LockingCondition_SingleKeyLock{
				SingleKeyLock: &pb.SingleKeyLock{
					KeyRequirement: &pb.SingleKeyLock_RequiredAddressHash{
						RequiredAddressHash: make([]byte, 20),
					},
				},
			},
		},
	}
	metadata := []byte("test metadata")

	idx, err := hostABI.AppendResourceOutput(ctx, contentHash, category, owner, lockingConditions, metadata)

	assert.NoError(t, err, "应该成功追加带锁定条件的资源输出")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
}

// TestAppendResourceOutput_EmptyDraftID 测试空草稿ID
func TestAppendResourceOutput_EmptyDraftID(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "", // 空草稿ID
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	contentHash := make([]byte, 32)
	category := "wasm"
	owner := make([]byte, 20)

	idx, err := hostABI.AppendResourceOutput(ctx, contentHash, category, owner, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "获取草稿ID失败", "错误信息应该正确")
}

// TestAppendResourceOutput_LoadDraftFailed 测试加载草稿失败
func TestAppendResourceOutput_LoadDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
	}
	mockDraftService := &mockDraftServiceForPorts{
		loadDraftError: assert.AnError, // 加载草稿失败
	}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	contentHash := make([]byte, 32)
	category := "wasm"
	owner := make([]byte, 20)

	idx, err := hostABI.AppendResourceOutput(ctx, contentHash, category, owner, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "加载交易草稿失败", "错误信息应该正确")
}

// TestAppendResourceOutput_AddResourceOutputFailed 测试添加资源输出失败
func TestAppendResourceOutput_AddResourceOutputFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
	}
	mockDraftService := &mockDraftServiceForPorts{
		addResourceOutputError: assert.AnError, // 添加输出失败
	}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	contentHash := make([]byte, 32)
	category := "wasm"
	owner := make([]byte, 20)

	idx, err := hostABI.AppendResourceOutput(ctx, contentHash, category, owner, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "追加资源输出失败", "错误信息应该正确")
}

// TestAppendResourceOutput_DifferentCategories 测试不同类别的资源输出
func TestAppendResourceOutput_DifferentCategories(t *testing.T) {
	categories := []string{"wasm", "onnx", "document", "static"}
	ctx := context.Background()
	contentHash := make([]byte, 32)
	owner := make([]byte, 20)

	for _, category := range categories {
		t.Run(category, func(t *testing.T) {
			// 每个子测试使用独立的hostABI实例，确保索引从0开始
			hostABI := createTestHostRuntimePortsForPorts(t)
			idx, err := hostABI.AppendResourceOutput(ctx, contentHash, category, owner, nil, nil)
			assert.NoError(t, err, "应该成功追加%s资源输出", category)
			assert.Equal(t, uint32(0), idx, "应该返回输出索引")
		})
	}
}


