package coordinator

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contextpkg "github.com/weisyn/v1/internal/core/ispc/context"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// 执行辅助函数测试
// ============================================================================
//
// 🎯 **测试目的**：发现执行辅助函数的缺陷和BUG
//
// ============================================================================

// TestExtractExecutionTrace 测试提取执行轨迹
func TestExtractExecutionTrace(t *testing.T) {
	manager := createTestManager(t)

	ctx := context.Background()
	executionStartTime := time.Now()
	ctx = context.WithValue(ctx, ContextKeyExecutionStart, executionStartTime)

	// 创建Mock执行上下文
	mockExecCtx := &mockExecutionContextWithTrace{
		trace: &contextpkg.ExecutionTrace{
			ExecutionID: "exec_123",
			StartTime:   executionStartTime,
			EndTime:     executionStartTime.Add(10 * time.Millisecond),
			HostFunctionCalls: []contextpkg.HostFunctionCall{
				{
					FunctionName: "test_function",
					Parameters:   map[string]interface{}{"param1": "value1"},
					Result:       map[string]interface{}{"result": "success"},
					Timestamp:    executionStartTime,
				},
			},
			StateChanges: []contextpkg.StateChange{
				{
					Type:      "update",
					Key:       "test_key",
					OldValue:  "old_value",
					NewValue:  "new_value",
					Timestamp: executionStartTime,
				},
			},
		},
	}

	trace, err := manager.extractExecutionTrace(ctx, mockExecCtx)
	require.NoError(t, err)
	assert.NotNil(t, trace)
	expectedTraceID := fmt.Sprintf("trace_%d", executionStartTime.UnixNano())
	assert.Equal(t, expectedTraceID, trace.TraceID)
	assert.Equal(t, executionStartTime, trace.StartTime)
	assert.Equal(t, executionStartTime.Add(10*time.Millisecond), trace.EndTime)
	assert.Equal(t, 1, len(trace.HostFunctionCalls))
	assert.Equal(t, 1, len(trace.StateChanges))
}

// TestExtractExecutionTrace_NoContextTrace 测试无法从执行上下文提取轨迹的情况
func TestExtractExecutionTrace_NoContextTrace(t *testing.T) {
	manager := createTestManager(t)

	ctx := context.Background()
	executionStartTime := time.Now()
	ctx = context.WithValue(ctx, ContextKeyExecutionStart, executionStartTime)

	// 创建不提供轨迹的执行上下文
	mockExecCtx := &mockExecutionContextWithoutTrace{}

	trace, err := manager.extractExecutionTrace(ctx, mockExecCtx)
	require.NoError(t, err)
	assert.NotNil(t, trace)
	assert.Contains(t, trace.TraceID, "trace_", "轨迹ID应该包含trace_前缀")
	assert.Equal(t, executionStartTime, trace.StartTime)
	assert.Equal(t, executionStartTime, trace.EndTime, "应该使用开始时间作为结束时间")
	assert.Equal(t, 0, len(trace.HostFunctionCalls))
	assert.Equal(t, 0, len(trace.StateChanges))
}

// TestExtractExecutionTrace_NoExecutionStart 测试没有执行开始时间的情况
func TestExtractExecutionTrace_NoExecutionStart(t *testing.T) {
	manager := createTestManager(t)

	ctx := context.Background()
	mockExecCtx := &mockExecutionContextWithoutTrace{}

	trace, err := manager.extractExecutionTrace(ctx, mockExecCtx)
	require.NoError(t, err)
	assert.NotNil(t, trace)
	assert.Contains(t, trace.TraceID, "trace_", "轨迹ID应该包含trace_前缀")
	assert.True(t, trace.StartTime.IsZero(), "开始时间应该为零值")
}

// TestComputeExecutionResultHash 测试计算执行结果哈希
func TestComputeExecutionResultHash(t *testing.T) {
	manager := createTestManager(t)

	// 设置hashManager
	manager.hashManager = testutil.NewTestHashManager()

	result := []uint64{1, 2, 3, 4, 5}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now().Add(10 * time.Millisecond),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	hash, err := manager.computeExecutionResultHash(result, trace)
	require.NoError(t, err)
	assert.NotNil(t, hash)
	assert.Equal(t, 32, len(hash), "SHA256哈希应该是32字节")
}

// TestComputeExecutionResultHash_NilHashManager 测试nil hashManager
// 🐛 **BUG检测**：nil hashManager应该返回错误
func TestComputeExecutionResultHash_NilHashManager(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = nil // nil hashManager

	result := []uint64{1, 2, 3}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now(),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	hash, err := manager.computeExecutionResultHash(result, trace)
	assert.Error(t, err, "nil hashManager应该返回错误")
	assert.Nil(t, hash, "哈希应该为nil")
	assert.Contains(t, err.Error(), "hashManager未初始化", "错误信息应该提到hashManager")
}

func TestComputeStateSnapshotHashes(t *testing.T) {
	trace := &ExecutionTrace{
		StateChanges: []StateChange{
			{
				Key:      "balance",
				OldValue: map[string]any{"alice": 10},
				NewValue: map[string]any{"alice": 5},
			},
			{
				Key:      "supply",
				OldValue: 100,
				NewValue: 95,
			},
		},
	}

	before, after := computeStateSnapshotHashes(trace)
	require.Len(t, before, 32)
	require.Len(t, after, 32)

	if bytes.Equal(before, after) {
		t.Fatalf("before/after hashes should differ when state changes differ")
	}
}

// TestGenerateStateID 测试生成状态ID
func TestGenerateStateID(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	ctx := context.Background()
	executionStartTime := time.Now()
	ctx = context.WithValue(ctx, ContextKeyContract, "test_contract")
	ctx = context.WithValue(ctx, ContextKeyFunction, "test_function")
	ctx = context.WithValue(ctx, ContextKeyExecutionStart, executionStartTime)
	ctx = context.WithValue(ctx, ContextKeyParamsCount, 3)

	stateID, err := manager.generateStateID(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stateID)
	assert.Greater(t, len(stateID), 0, "状态ID应该不为空")
}

// TestGenerateStateID_NoContextValues 测试没有上下文值的情况
func TestGenerateStateID_NoContextValues(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	ctx := context.Background()
	stateID, err := manager.generateStateID(ctx)
	require.NoError(t, err)
	assert.NotNil(t, stateID, "即使没有上下文值也应该生成状态ID")
}

// TestGetNodeID 测试获取节点ID
func TestGetNodeID(t *testing.T) {
	manager := createTestManager(t)

	nodeID := manager.getNodeID()
	// 可能从环境变量获取，也可能返回默认值
	assert.NotEmpty(t, nodeID, "节点ID不应该为空")
}

// ============================================================================
// Mock对象定义
// ============================================================================

// mockExecutionContextWithTrace Mock的执行上下文（提供轨迹）
type mockExecutionContextWithTrace struct {
	trace *contextpkg.ExecutionTrace
}

func (m *mockExecutionContextWithTrace) GetExecutionTrace() (interface{}, error) {
	return m.trace, nil
}

// mockExecutionContextWithoutTrace Mock的执行上下文（不提供轨迹）
type mockExecutionContextWithoutTrace struct{}

func (m *mockExecutionContextWithoutTrace) GetExecutionTrace() (interface{}, error) {
	return nil, assert.AnError
}
