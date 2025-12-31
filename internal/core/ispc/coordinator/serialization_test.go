package coordinator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// 序列化函数测试
// ============================================================================
//
// 🎯 **测试目的**：发现序列化函数的缺陷和BUG
//
// ============================================================================

// TestSerializeHostFunctionCalls 测试序列化宿主函数调用
func TestSerializeHostFunctionCalls(t *testing.T) {
	manager := createTestManager(t)

	calls := []HostFunctionCall{
		{
			FunctionName: "function_a",
			Parameters:   []interface{}{"param1", 123},
			Result:       "result1",
			Timestamp:    time.Now(),
		},
		{
			FunctionName: "function_b",
			Parameters:   []interface{}{"param2", 456},
			Result:       "result2",
			Timestamp:    time.Now().Add(1 * time.Second),
		},
	}

	serialized, err := manager.serializeHostFunctionCalls(calls)
	require.NoError(t, err)
	assert.Equal(t, 2, len(serialized), "应该序列化2个调用")

	// 验证排序（应该按函数名和时间戳排序）
	assert.Equal(t, "function_a", serialized[0]["function_name"])
	assert.Equal(t, "function_b", serialized[1]["function_name"])
}

// TestSerializeHostFunctionCalls_Empty 测试空列表
func TestSerializeHostFunctionCalls_Empty(t *testing.T) {
	manager := createTestManager(t)

	serialized, err := manager.serializeHostFunctionCalls([]HostFunctionCall{})
	require.NoError(t, err)
	assert.Equal(t, 0, len(serialized), "空列表应该返回空结果")
}

// TestSerializeHostFunctionCalls_SameFunctionName 测试相同函数名的情况
func TestSerializeHostFunctionCalls_SameFunctionName(t *testing.T) {
	manager := createTestManager(t)

	baseTime := time.Now()
	calls := []HostFunctionCall{
		{
			FunctionName: "function_a",
			Parameters:   []interface{}{"param1"},
			Result:       "result1",
			Timestamp:    baseTime.Add(2 * time.Second),
		},
		{
			FunctionName: "function_a",
			Parameters:   []interface{}{"param2"},
			Result:       "result2",
			Timestamp:    baseTime.Add(1 * time.Second),
		},
	}

	serialized, err := manager.serializeHostFunctionCalls(calls)
	require.NoError(t, err)
	assert.Equal(t, 2, len(serialized), "应该序列化2个调用")

	// 验证排序（相同函数名应该按时间戳排序）
	time1 := serialized[0]["timestamp"].(int64)
	time2 := serialized[1]["timestamp"].(int64)
	assert.True(t, time1 < time2, "应该按时间戳排序")
}

// TestSerializeStateChanges 测试序列化状态变更
func TestSerializeStateChanges(t *testing.T) {
	manager := createTestManager(t)

	changes := []StateChange{
		{
			Type:      "update",
			Key:       "key_a",
			OldValue:  "old_value_a",
			NewValue:  "new_value_a",
			Timestamp: time.Now(),
		},
		{
			Type:      "create",
			Key:       "key_b",
			OldValue:  nil,
			NewValue:  "new_value_b",
			Timestamp: time.Now().Add(1 * time.Second),
		},
	}

	serialized, err := manager.serializeStateChanges(changes)
	require.NoError(t, err)
	assert.Equal(t, 2, len(serialized), "应该序列化2个变更")

	// 验证排序（应该按类型、键和时间戳排序）
	assert.Equal(t, "create", serialized[0]["type"], "create应该在update之前")
	assert.Equal(t, "update", serialized[1]["type"])
}

// TestSerializeStateChanges_Empty 测试空列表
func TestSerializeStateChanges_Empty(t *testing.T) {
	manager := createTestManager(t)

	serialized, err := manager.serializeStateChanges([]StateChange{})
	require.NoError(t, err)
	assert.Equal(t, 0, len(serialized), "空列表应该返回空结果")
}

// TestSerializeStateChanges_SameTypeAndKey 测试相同类型和键的情况
func TestSerializeStateChanges_SameTypeAndKey(t *testing.T) {
	manager := createTestManager(t)

	baseTime := time.Now()
	changes := []StateChange{
		{
			Type:      "update",
			Key:       "key_a",
			OldValue:  "old_value_1",
			NewValue:  "new_value_1",
			Timestamp: baseTime.Add(2 * time.Second),
		},
		{
			Type:      "update",
			Key:       "key_a",
			OldValue:  "old_value_2",
			NewValue:  "new_value_2",
			Timestamp: baseTime.Add(1 * time.Second),
		},
	}

	serialized, err := manager.serializeStateChanges(changes)
	require.NoError(t, err)
	assert.Equal(t, 2, len(serialized), "应该序列化2个变更")

	// 验证排序（相同类型和键应该按时间戳排序）
	time1 := serialized[0]["timestamp"].(int64)
	time2 := serialized[1]["timestamp"].(int64)
	assert.True(t, time1 < time2, "应该按时间戳排序")
}

// TestSerializeExecutionTraceForZK 测试序列化执行轨迹用于ZK证明
func TestSerializeExecutionTraceForZK(t *testing.T) {
	manager := createTestManager(t)

	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now().Add(10 * time.Millisecond),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	serialized, err := manager.serializeExecutionTraceForZK(trace)
	require.NoError(t, err)
	assert.NotNil(t, serialized, "序列化结果不应该为nil")
	assert.Greater(t, len(serialized), 0, "序列化结果不应该为空")
}

// TestSerializeExecutionTraceForZK_WithCalls 测试包含调用的情况
func TestSerializeExecutionTraceForZK_WithCalls(t *testing.T) {
	manager := createTestManager(t)

	trace := &ExecutionTrace{
		TraceID:   "test_trace_id",
		StartTime: time.Now(),
		EndTime:   time.Now().Add(10 * time.Millisecond),
		HostFunctionCalls: []HostFunctionCall{
			{
				FunctionName: "test_function",
				Parameters:   []interface{}{"param1"},
				Result:       "result1",
				Timestamp:    time.Now(),
			},
		},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	serialized, err := manager.serializeExecutionTraceForZK(trace)
	require.NoError(t, err)
	assert.NotNil(t, serialized, "序列化结果不应该为nil")
	assert.Greater(t, len(serialized), 0, "序列化结果不应该为空")
}

// TestSerializeStateChangesForZK 测试序列化状态变更用于ZK证明
func TestSerializeStateChangesForZK(t *testing.T) {
	manager := createTestManager(t)

	changes := []StateChange{
		{
			Type:      "update",
			Key:       "key_a",
			OldValue:  "old_value",
			NewValue:  "new_value",
			Timestamp: time.Now(),
		},
	}

	serialized, err := manager.serializeStateChangesForZK(changes)
	require.NoError(t, err)
	assert.NotNil(t, serialized, "序列化结果不应该为nil")
	assert.Greater(t, len(serialized), 0, "序列化结果不应该为空")
}

// TestSerializeStateChangesForZK_Empty 测试空列表
func TestSerializeStateChangesForZK_Empty(t *testing.T) {
	manager := createTestManager(t)

	serialized, err := manager.serializeStateChangesForZK([]StateChange{})
	require.NoError(t, err)
	assert.NotNil(t, serialized, "序列化结果不应该为nil")
	// 空列表可能返回空字节数组或包含空数组标记的字节数组
}

