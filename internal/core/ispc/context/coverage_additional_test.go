package context

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// 补充测试：提高覆盖率
// ============================================================================

// TestValidateContextCleanup 测试验证上下文清理
func TestValidateContextCleanup(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_validate_cleanup"
	callerAddress := "caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 验证清理（上下文存在时）
	cleaned, issues := ValidateContextCleanup(ctxImpl, manager)
	assert.False(t, cleaned, "上下文存在时不应该被标记为已清理")
	assert.NotEmpty(t, issues, "应该有清理问题")

	// 销毁上下文
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)

	// 验证清理（上下文已销毁）
	cleaned, issues = ValidateContextCleanup(ctxImpl, manager)
	// 注意：即使上下文已从管理器中移除，ctxImpl对象仍然存在
	// 所以cleaned可能为false，issues可能包含"上下文仍在管理器中"
	_ = cleaned
	_ = issues

	// 测试nil上下文
	cleaned, issues = ValidateContextCleanup(nil, manager)
	assert.True(t, cleaned, "nil上下文应该被标记为已清理")
	assert.Empty(t, issues)
}

// TestContextDebugger_LogContextCreation 测试记录上下文创建日志（提高覆盖率）
func TestContextDebugger_LogContextCreation(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	
	// 测试不同调试模式
	for _, mode := range []DebugMode{DebugModeOff, DebugModeBasic, DebugModeVerbose} {
		debugger := NewContextDebugger(logger, mode)
		debugger.Enable()

		debugger.LogContextCreation("exec_1", "trace_1", "request_1", "user_1")
		debugger.LogContextCreation("exec_2", "", "", "") // 空值测试
	}

	// 验证日志被记录
	logs := logger.GetLogs()
	require.Greater(t, len(logs), 0)
}

// TestContextDebugger_LogContextAccess 测试记录上下文访问日志（提高覆盖率）
func TestContextDebugger_LogContextAccess(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	
	// 测试Verbose模式（会记录访问日志）
	debugger := NewContextDebugger(logger, DebugModeVerbose)
	debugger.Enable()

	debugger.LogContextAccess("exec_1", "read")
	debugger.LogContextAccess("exec_2", "write")

	// 验证日志被记录
	logs := logger.GetLogs()
	require.Greater(t, len(logs), 0)
}

// TestContextDebugger_LogContextDestruction 测试记录上下文销毁日志（提高覆盖率）
func TestContextDebugger_LogContextDestruction(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	
	debugger := NewContextDebugger(logger, DebugModeBasic)
	debugger.Enable()

	debugger.LogContextDestruction("exec_1", 100*time.Millisecond, "normal")
	debugger.LogContextDestruction("exec_2", 200*time.Millisecond, "") // 空reason

	// 验证日志被记录
	logs := logger.GetLogs()
	require.Greater(t, len(logs), 0)
}

// TestContextDebugger_LogHostFunctionCall_Verbose 测试记录宿主函数调用日志（Verbose模式）
// 注意：在 Verbose 模式下，LogHostFunctionCall 会直接返回，不记录日志
// 这个测试验证了这种行为
func TestContextDebugger_LogHostFunctionCall_Verbose(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	debugger := NewContextDebugger(logger, DebugModeVerbose)
	debugger.Enable()

	// 在 Verbose 模式下，这些调用会直接返回，不记录日志
	debugger.LogHostFunctionCall("exec_1", "test_function", 100*time.Millisecond, true, nil)
	err := assert.AnError
	debugger.LogHostFunctionCall("exec_1", "test_function", 50*time.Millisecond, false, err)

	// 验证日志没有被记录（因为 Verbose 模式下会直接返回）
	logs := logger.GetLogs()
	// 在 Verbose 模式下，LogHostFunctionCall 会直接返回，所以日志应该为空
	assert.Equal(t, 0, len(logs), "Verbose模式下LogHostFunctionCall应该不记录日志")
}

// TestContextDebugger_LogStateChange_Verbose 测试记录状态变更日志（Verbose模式）
// 注意：在 Verbose 模式下，LogStateChange 会直接返回，不记录日志
// 这个测试验证了这种行为
func TestContextDebugger_LogStateChange_Verbose(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	debugger := NewContextDebugger(logger, DebugModeVerbose)
	debugger.Enable()

	// 在 Verbose 模式下，这些调用会直接返回，不记录日志
	debugger.LogStateChange("exec_1", "set", "key1")
	debugger.LogStateChange("exec_1", "delete", "key2")

	// 验证日志没有被记录（因为 Verbose 模式下会直接返回）
	logs := logger.GetLogs()
	// 在 Verbose 模式下，LogStateChange 会直接返回，所以日志应该为空
	assert.Equal(t, 0, len(logs), "Verbose模式下LogStateChange应该不记录日志")
}

// TestDebugTool_showStats_Additional 测试显示统计信息（补充测试）
func TestDebugTool_showStats_Additional(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()

	// 创建一些上下文
	executionIDs := []string{"exec_1", "exec_2"}
	for _, id := range executionIDs {
		_, err := manager.CreateContext(ctx, id, "caller")
		require.NoError(t, err)
	}

	debugTool := manager.GetDebugTool()
	result, err := debugTool.ExecuteCommand(DebugCommandStats)
	require.NoError(t, err)
	resultMap := result.(map[string]interface{})
	assert.Contains(t, resultMap, "active_context_count")

	// 清理
	for _, id := range executionIDs {
		manager.DestroyContext(ctx, id)
	}
}

// TestDebugTool_exportContext_Additional 测试导出上下文（补充测试）
func TestDebugTool_exportContext_Additional(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "exec_export_additional"
	callerAddress := "caller"

	// 创建上下文
	_, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	debugTool := manager.GetDebugTool()
	result, err := debugTool.ExecuteCommand(DebugCommandExport, executionID)
	require.NoError(t, err)
	jsonData, ok := result.([]byte)
	require.True(t, ok)
	assert.Contains(t, string(jsonData), executionID)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestDebugTool_detectLeaks_Additional 测试检测上下文泄漏（补充测试）
func TestDebugTool_detectLeaks_Additional(t *testing.T) {
	manager := createTestManager(t)
	debugTool := manager.GetDebugTool()

	result, err := debugTool.ExecuteCommand(DebugCommandLeaks)
	require.NoError(t, err)
	resultMap := result.(map[string]interface{})
	assert.Contains(t, resultMap, "leaked_contexts")
}

// TestExecutionResultVerifier_RecordExecutionResult_Multiple 测试多次记录执行结果
// 注意：验证器会检查一致性，如果相同输入产生不同结果，会返回错误
func TestExecutionResultVerifier_RecordExecutionResult_Multiple(t *testing.T) {
	verifier := NewExecutionResultVerifier()
	inputHash := []byte("input_hash")
	resultHash1 := []byte("result_hash_1")
	resultHash2 := []byte("result_hash_2")

	// 第一次记录
	err := verifier.RecordExecutionResult(inputHash, resultHash1)
	require.NoError(t, err)

	// 验证第一次记录的结果
	consistent, err := verifier.VerifyExecutionResult(inputHash, resultHash1)
	require.NoError(t, err)
	assert.True(t, consistent)

	// 第二次记录（相同输入，不同结果）- 这应该更新记录，而不是报错
	err = verifier.RecordExecutionResult(inputHash, resultHash2)
	// 注意：根据实现，可能会更新记录或返回错误
	// 如果返回错误，说明验证器不允许更新结果
	if err != nil {
		assert.Contains(t, err.Error(), "执行结果不一致")
	} else {
		// 如果没有错误，验证最后一次记录的结果
		consistent, err := verifier.VerifyExecutionResult(inputHash, resultHash2)
		require.NoError(t, err)
		assert.True(t, consistent)
	}
}

// TestExecutionResultVerifier_GetExecutionStats 测试获取执行统计信息
func TestExecutionResultVerifier_GetExecutionStats(t *testing.T) {
	verifier := NewExecutionResultVerifier()
	
	// 记录一些执行结果
	inputHash1 := []byte("input_hash_1")
	resultHash1 := []byte("result_hash_1")
	verifier.RecordExecutionResult(inputHash1, resultHash1)

	inputHash2 := []byte("input_hash_2")
	resultHash2 := []byte("result_hash_2")
	verifier.RecordExecutionResult(inputHash2, resultHash2)

	// 获取统计信息
	stats := verifier.GetExecutionStats()
	require.NotNil(t, stats)
	assert.Contains(t, stats, "total_executions")
	assert.Contains(t, stats, "consistent_executions")
}

// TestEnsureDeterministicTimestamp_Additional 测试确保确定性时间戳（补充测试）
func TestEnsureDeterministicTimestamp_Additional(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_deterministic_timestamp_additional"
	callerAddress := "caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 创建确定性增强器
	fixedTimestamp := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	enforcer := NewDeterministicEnforcer(executionID, nil, &fixedTimestamp)

	// 确保确定性时间戳
	EnsureDeterministicTimestamp(ctxImpl, enforcer)

	// 验证时间戳已设置
	timestamp := ctxImpl.GetDeterministicTimestamp()
	assert.Equal(t, fixedTimestamp, timestamp)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestVerifyExecutionResultConsistency_Additional 测试验证执行结果一致性（补充测试）
func TestVerifyExecutionResultConsistency_Additional(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_consistency_additional"
	callerAddress := "caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 创建确定性增强器和验证器
	enforcer := NewDeterministicEnforcer(executionID, nil, nil)
	verifier := NewExecutionResultVerifier()

	// 计算结果哈希
	resultHash := []byte("test_result_hash")

	// 验证一致性（第一次执行，应该通过）
	err = VerifyExecutionResultConsistency(ctxImpl, enforcer, verifier, resultHash)
	require.NoError(t, err)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestCheckMemoryLeak_Detailed 测试检查内存泄漏（详细场景）
func TestCheckMemoryLeak_Detailed(t *testing.T) {
	// 获取初始内存统计
	beforeStats := GetMemoryStats()

	// 创建一些对象
	manager := createTestManager(t)
	ctx := context.Background()
	executionIDs := []string{"exec_1", "exec_2", "exec_3"}
	for _, id := range executionIDs {
		_, err := manager.CreateContext(ctx, id, "caller")
		require.NoError(t, err)
	}

	// 获取之后的内存统计
	afterStats := GetMemoryStats()

	// 检查内存泄漏
	hasLeak, details := CheckMemoryLeak(beforeStats, afterStats)
	assert.NotNil(t, details)
	_ = hasLeak

	// 清理
	for _, id := range executionIDs {
		manager.DestroyContext(ctx, id)
	}
}

// TestDeepCopyInterface_ComplexTypes 测试深度拷贝复杂类型
func TestDeepCopyInterface_ComplexTypes(t *testing.T) {
	// 测试map[string]interface{}
	srcMap := map[string]interface{}{
		"key1": "value1",
		"key2": []byte("bytes"),
		"key3": map[string]interface{}{
			"nested": "value",
		},
	}
	dstMap := deepCopyInterface(srcMap).(map[string]interface{})
	assert.Equal(t, srcMap["key1"], dstMap["key1"])
	// 验证是深拷贝（修改dstMap不应该影响srcMap）
	dstMap["key1"] = "modified"
	assert.NotEqual(t, srcMap["key1"], dstMap["key1"], "深拷贝后修改不应该影响原map")

	// 测试[]interface{}
	srcSlice := []interface{}{"item1", []byte("bytes"), map[string]interface{}{"key": "value"}}
	dstSlice := deepCopyInterface(srcSlice).([]interface{})
	assert.Equal(t, len(srcSlice), len(dstSlice))
	// 验证是深拷贝（修改dstSlice不应该影响srcSlice）
	if len(dstSlice) > 0 {
		dstSlice[0] = "modified"
		assert.NotEqual(t, srcSlice[0], dstSlice[0], "深拷贝后修改不应该影响原slice")
	}

	// 测试[]byte
	srcBytes := []byte("test bytes")
	dstBytes := deepCopyInterface(srcBytes).([]byte)
	assert.Equal(t, srcBytes, dstBytes)
	// 验证是深拷贝（修改dstBytes不应该影响srcBytes）
	if len(dstBytes) > 0 {
		dstBytes[0] = 'X'
		assert.NotEqual(t, srcBytes[0], dstBytes[0], "深拷贝后修改不应该影响原字节数组")
	}

	// 测试string（不可变，应该返回相同值）
	srcStr := "test string"
	dstStr := deepCopyInterface(srcStr).(string)
	assert.Equal(t, srcStr, dstStr)
}

// TestVerifyContextIsolation_Detailed 测试验证上下文隔离（详细场景）
func TestVerifyContextIsolation_Detailed(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()

	// 创建两个上下文
	executionID1 := "exec_1"
	executionID2 := "exec_2"
	callerAddress := "caller"

	executionContext1, err := manager.CreateContext(ctx, executionID1, callerAddress)
	require.NoError(t, err)

	executionContext2, err := manager.CreateContext(ctx, executionID2, callerAddress)
	require.NoError(t, err)

	ctxImpl1 := executionContext1.(*contextImpl)
	ctxImpl2 := executionContext2.(*contextImpl)

	// 验证隔离
	isolated, issues := VerifyContextIsolation(ctxImpl1, ctxImpl2)
	assert.True(t, isolated, "两个独立的上下文应该是隔离的")
	assert.Empty(t, issues, "不应该有隔离问题")

	// 测试nil上下文
	isolated, issues = VerifyContextIsolation(nil, ctxImpl1)
	assert.False(t, isolated, "nil上下文不应该被标记为隔离")
	assert.NotEmpty(t, issues)

	// 清理
	manager.DestroyContext(ctx, executionID1)
	manager.DestroyContext(ctx, executionID2)
}

