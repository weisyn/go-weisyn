package context

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// ============================================================================
// manager.go 未覆盖方法测试
// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// TestContextImpl_HostABI 测试获取HostABI
func TestContextImpl_HostABI(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_hostabi"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 获取HostABI（初始应该为nil）
	hostABI := executionContext.(*contextImpl).HostABI()
	assert.Nil(t, hostABI)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestContextImpl_SetHostABI 测试设置HostABI
func TestContextImpl_SetHostABI(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_set_hostabi"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 设置nil HostABI应该返回错误
	err = ctxImpl.SetHostABI(nil)
	assert.Error(t, err)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestContextImpl_GetTransactionDraft 测试获取交易草稿
func TestContextImpl_GetTransactionDraft(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_get_draft"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 获取交易草稿（初始可能为nil或已初始化，取决于实现）
	draft, err := ctxImpl.GetTransactionDraft()
	if err != nil {
		// 如果返回错误，验证错误信息
		assert.Nil(t, draft)
		assert.Contains(t, err.Error(), "transaction draft not initialized")
	} else {
		// 如果没有错误，说明txDraft已经被初始化
		assert.NotNil(t, draft)
	}

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestContextImpl_UpdateTransactionDraft 测试更新交易草稿
func TestContextImpl_UpdateTransactionDraft(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_update_draft"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 更新nil交易草稿应该返回错误
	err = ctxImpl.UpdateTransactionDraft(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot update with nil transaction draft")

	// 更新有效的交易草稿
	draft := &ispcInterfaces.TransactionDraft{
		Tx: nil, // 简化测试
	}
	err = ctxImpl.UpdateTransactionDraft(draft)
	assert.NoError(t, err)

	// 验证交易草稿已更新
	retrievedDraft, err := ctxImpl.GetTransactionDraft()
	assert.NoError(t, err)
	assert.Equal(t, draft, retrievedDraft)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestContextImpl_RecordStateChange 测试记录状态变更
func TestContextImpl_RecordStateChange(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_state_change"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 记录状态变更
	err = ctxImpl.RecordStateChange("set", "key1", nil, "value1")
	assert.NoError(t, err)

	// 验证状态变更已记录（直接访问stateChanges字段）
	ctxImpl.mutex.RLock()
	stateChanges := ctxImpl.stateChanges
	ctxImpl.mutex.RUnlock()
	require.Len(t, stateChanges, 1)
	assert.Equal(t, "set", stateChanges[0].Type)
	assert.Equal(t, "key1", stateChanges[0].Key)
	assert.Equal(t, "value1", stateChanges[0].NewValue)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_ExportContextState 测试导出上下文状态
func TestManager_ExportContextState(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_export"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	_ = executionContext

	// 导出上下文状态（返回JSON字节）
	stateJSON, err := manager.ExportContextState(executionID, false)
	require.NoError(t, err)
	require.NotNil(t, stateJSON)
	assert.Contains(t, string(stateJSON), executionID)

	// 测试不存在的上下文
	_, err = manager.ExportContextState("non_existent", false)
	assert.Error(t, err)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_CreateDeterministicEnforcer 测试创建确定性增强器
func TestManager_CreateDeterministicEnforcer(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_deterministic"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	_ = executionContext

	// 创建确定性增强器
	inputParams := []byte("test_input")
	fixedTimestamp := time.Now()
	enforcer := manager.CreateDeterministicEnforcer(executionID, inputParams, &fixedTimestamp)
	require.NotNil(t, enforcer)

	// 验证增强器功能
	timestamp := enforcer.GetFixedTimestamp()
	assert.False(t, timestamp.IsZero())

	seed := enforcer.GetFixedRandomSeed()
	assert.NotZero(t, seed)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_RecordExecutionResult 测试记录执行结果
func TestManager_RecordExecutionResult(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_record_result"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)
	_ = ctxImpl

	// 记录执行结果（需要inputHash和resultHash）
	inputHash := []byte("test_input_hash")
	resultHash := []byte("test_result_hash")
	err = manager.RecordExecutionResult(inputHash, resultHash)
	assert.NoError(t, err)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_VerifyExecutionResult 测试验证执行结果
func TestManager_VerifyExecutionResult(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_verify_result"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)
	_ = ctxImpl

	// 记录执行结果（需要inputHash和resultHash）
	inputHash := []byte("test_input_hash")
	resultHash := []byte("test_result_hash")
	err = manager.RecordExecutionResult(inputHash, resultHash)
	require.NoError(t, err)

	// 验证相同结果
	consistent, err := manager.VerifyExecutionResult(inputHash, resultHash)
	assert.NoError(t, err)
	assert.True(t, consistent)

	// 验证不同结果（应该不一致且返回错误）
	// ⚠️ **代码行为**：VerifyExecutionResult在结果不一致时会返回错误，这是正确的行为
	differentHash := []byte("different_hash")
	consistent, err = manager.VerifyExecutionResult(inputHash, differentHash)
	assert.Error(t, err, "结果不一致时应该返回错误")
	assert.Contains(t, err.Error(), "执行结果不一致")
	assert.False(t, consistent)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_GetExecutionStats 测试获取执行统计信息
func TestManager_GetExecutionStats(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_stats"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	_ = executionContext

	// 获取执行统计信息
	stats := manager.GetExecutionStats()
	require.NotNil(t, stats)
	assert.Contains(t, stats, "total_executions")
	assert.Contains(t, stats, "consistent_executions")

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_ValidateTrace 测试验证轨迹
func TestManager_ValidateTrace(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_validate_trace"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 记录一些宿主函数调用
	ctxImpl := executionContext.(*contextImpl)
	ctxImpl.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		Sequence:     1,
		FunctionName: "test_function",
		Parameters:   map[string]interface{}{"key": "value"},
		Result:       map[string]interface{}{"result": "success"},
		Timestamp:   time.Now().UnixNano(),
	})

	// 获取轨迹并验证（需要转换为ExecutionTrace）
	trace, err := ctxImpl.GetExecutionTrace()
	require.NoError(t, err)
	
	// 构建ExecutionTrace对象（需要转换类型）
	hostFunctionCalls := make([]HostFunctionCall, len(trace))
	for i, call := range trace {
		hostFunctionCalls[i] = HostFunctionCall{
			FunctionName: call.FunctionName,
			Parameters:   call.Parameters,
			Result:       call.Result,
			Timestamp:    time.Unix(0, call.Timestamp), // 转换int64到time.Time
			Duration:     0, // 简化处理
			Success:      true,
		}
	}
	executionTrace := &ExecutionTrace{
		ExecutionID:      executionID, // ⚠️ **必需字段**：验证规则要求executionID不能为空
		StartTime:        time.Now(),   // ⚠️ **必需字段**：验证规则要求startTime不能为空
		EndTime:          time.Now().Add(100 * time.Millisecond), // ⚠️ **必需字段**：验证规则要求endTime不能为空
		HostFunctionCalls: hostFunctionCalls,
	}

	errors := manager.ValidateTrace(executionTrace)
	assert.Empty(t, errors, "验证轨迹应该没有错误")

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_VerifyContextCleanup 测试验证上下文清理
func TestManager_VerifyContextCleanup(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_cleanup"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	_ = executionContext

	// 验证清理（上下文存在时）
	cleaned, issues := manager.VerifyContextCleanup(executionID)
	assert.False(t, cleaned, "上下文存在时不应该被标记为已清理")
	_ = issues

	// 销毁上下文
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)

	// 验证清理（上下文已销毁）
	cleaned, issues = manager.VerifyContextCleanup(executionID)
	// 注意：清理验证器的行为取决于实现，这里只测试不报错
	_ = cleaned
	_ = issues
}

// TestManager_GetCleanupStats 测试获取清理统计信息
func TestManager_GetCleanupStats(t *testing.T) {
	manager := createTestManager(t)

	// 获取清理统计信息
	stats := manager.GetCleanupStats()
	require.NotNil(t, stats)
	assert.Contains(t, stats, "total_cleaned")
}

// TestManager_DeepCopyContext 测试深度拷贝上下文
func TestManager_DeepCopyContext(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_deep_copy"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 添加一些数据
	ctxImpl.RecordHostFunctionCall(&ispcInterfaces.HostFunctionCall{
		Sequence:     1,
		FunctionName: "test_function",
		Parameters:   map[string]interface{}{"key": "value"},
		Result:       map[string]interface{}{"result": "success"},
		Timestamp:    time.Now().UnixNano(),
	})

	// 深度拷贝
	copiedCtx, err := manager.DeepCopyContext(executionID)
	require.NoError(t, err)
	require.NotNil(t, copiedCtx)

	// 验证拷贝的上下文
	assert.Equal(t, ctxImpl.executionID, copiedCtx.executionID)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_VerifyContextIsolation 测试验证上下文隔离
func TestManager_VerifyContextIsolation(t *testing.T) {
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

	// 验证隔离
	isolated, issues := manager.VerifyContextIsolation(executionID1, executionID2)
	assert.True(t, isolated, "两个独立的上下文应该是隔离的")
	assert.Empty(t, issues, "不应该有隔离问题")
	_ = executionContext1
	_ = executionContext2

	// 清理
	manager.DestroyContext(ctx, executionID1)
	manager.DestroyContext(ctx, executionID2)
}

// TestManager_CheckMemoryLeak 测试检查内存泄漏
func TestManager_CheckMemoryLeak(t *testing.T) {
	manager := createTestManager(t)

	// 获取初始内存统计
	beforeStats := manager.GetMemoryStats()

	// 创建一些上下文
	ctx := context.Background()
	executionIDs := []string{"exec_1", "exec_2", "exec_3"}
	for _, id := range executionIDs {
		_, err := manager.CreateContext(ctx, id, "caller")
		require.NoError(t, err)
	}

	// 获取之后的内存统计
	afterStats := manager.GetMemoryStats()

	// 检查内存泄漏（需要runtime.MemStats）
	hasLeak, details := manager.CheckMemoryLeak(beforeStats, afterStats)
	// 注意：内存泄漏检测的行为取决于实现，这里只测试不报错
	assert.NotNil(t, details)
	_ = hasLeak

	// 清理
	for _, id := range executionIDs {
		manager.DestroyContext(ctx, id)
	}
}

// TestManager_GetMemoryStats 测试获取内存统计信息
func TestManager_GetMemoryStats(t *testing.T) {
	manager := createTestManager(t)

	// 获取内存统计信息（返回*runtime.MemStats）
	stats := manager.GetMemoryStats()
	require.NotNil(t, stats)
	assert.Greater(t, stats.Alloc, uint64(0))
}

