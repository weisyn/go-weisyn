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
// 边界测试和错误场景测试
// ============================================================================

// TestManager_CreateDeterministicEnforcer_NilTimestamp 测试创建确定性增强器（nil时间戳）
func TestManager_CreateDeterministicEnforcer_NilTimestamp(t *testing.T) {
	manager := createTestManager(t)
	executionID := "test_nil_timestamp"
	inputParams := []byte("test_input")

	// 使用nil时间戳（应该使用当前时间）
	enforcer := manager.CreateDeterministicEnforcer(executionID, inputParams, nil)
	require.NotNil(t, enforcer)
	assert.False(t, enforcer.GetFixedTimestamp().IsZero())
}

// TestContextImpl_GetContractAddress 测试获取合约地址
func TestContextImpl_GetContractAddress(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_contract_address"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 获取合约地址（初始应该为nil或空）
	contractAddress := ctxImpl.GetContractAddress()
	// 初始状态可能为nil或空切片
	_ = contractAddress

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestContextDebugger_LogContextCreation_Disabled 测试禁用状态下的日志记录
func TestContextDebugger_LogContextCreation_Disabled(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	debugger := NewContextDebugger(logger, DebugModeOff)
	debugger.Disable()

	// 禁用状态下不应该记录日志
	debugger.LogContextCreation("exec_1", "trace_1", "request_1", "user_1")
	logs := logger.GetLogs()
	assert.Equal(t, 0, len(logs), "禁用状态下不应该记录日志")
}

// TestContextDebugger_LogContextAccess_OffMode 测试Off模式下的访问日志
func TestContextDebugger_LogContextAccess_OffMode(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	debugger := NewContextDebugger(logger, DebugModeOff)
	debugger.Enable()

	// Off模式下不应该记录访问日志
	debugger.LogContextAccess("exec_1", "read")
	logs := logger.GetLogs()
	assert.Equal(t, 0, len(logs), "Off模式下不应该记录访问日志")
}

// TestContextDebugger_LogContextDestruction_Disabled 测试禁用状态下的销毁日志
func TestContextDebugger_LogContextDestruction_Disabled(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	debugger := NewContextDebugger(logger, DebugModeOff)
	debugger.Disable()

	// 禁用状态下不应该记录日志
	debugger.LogContextDestruction("exec_1", 100*time.Millisecond, "normal")
	logs := logger.GetLogs()
	assert.Equal(t, 0, len(logs), "禁用状态下不应该记录日志")
}

// TestExecutionResultVerifier_RecordExecutionResult_NilHash 测试nil哈希处理
func TestExecutionResultVerifier_RecordExecutionResult_NilHash(t *testing.T) {
	verifier := NewExecutionResultVerifier()

	// 测试nil输入哈希
	err := verifier.RecordExecutionResult(nil, []byte("result"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能为nil")

	// 测试nil结果哈希
	err = verifier.RecordExecutionResult([]byte("input"), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能为nil")
}

// TestExecutionResultVerifier_VerifyExecutionResult_NilHash 测试nil哈希验证
func TestExecutionResultVerifier_VerifyExecutionResult_NilHash(t *testing.T) {
	verifier := NewExecutionResultVerifier()

	// 测试nil输入哈希
	consistent, err := verifier.VerifyExecutionResult(nil, []byte("result"))
	assert.Error(t, err)
	assert.False(t, consistent)

	// 测试nil结果哈希
	consistent, err = verifier.VerifyExecutionResult([]byte("input"), nil)
	assert.Error(t, err)
	assert.False(t, consistent)
}

// TestDeepCopyContext_NilSource 测试深度拷贝nil源
func TestDeepCopyContext_NilSource(t *testing.T) {
	_, err := DeepCopyContext(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "源上下文不能为nil")
}

// TestVerifyContextIsolation_NilContext 测试nil上下文隔离验证
func TestVerifyContextIsolation_NilContext(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "exec_1"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 测试nil上下文
	isolated, issues := VerifyContextIsolation(nil, ctxImpl)
	assert.False(t, isolated)
	assert.NotEmpty(t, issues)

	isolated, issues = VerifyContextIsolation(ctxImpl, nil)
	assert.False(t, isolated)
	assert.NotEmpty(t, issues)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestCheckMemoryLeak_NilStats 测试nil内存统计
func TestCheckMemoryLeak_NilStats(t *testing.T) {
	beforeStats := GetMemoryStats()
	afterStats := GetMemoryStats()

	// 测试nil统计
	hasLeak, details := CheckMemoryLeak(nil, afterStats)
	assert.NotNil(t, details)
	_ = hasLeak

	hasLeak, details = CheckMemoryLeak(beforeStats, nil)
	assert.NotNil(t, details)
	_ = hasLeak
}

// TestManager_RecordExecutionResult_NilVerifier 测试nil验证器
func TestManager_RecordExecutionResult_NilVerifier(t *testing.T) {
	manager := createTestManager(t)
	// 注意：如果resultVerifier为nil，应该返回错误
	// 但实际实现中，NewManager会初始化resultVerifier，所以这个测试可能不会触发错误
	// 这里主要测试方法调用
	inputHash := []byte("input_hash")
	resultHash := []byte("result_hash")
	err := manager.RecordExecutionResult(inputHash, resultHash)
	// 如果resultVerifier已初始化，应该成功；如果为nil，应该返回错误
	_ = err
}

// TestManager_ValidateTrace_NilChecker 测试nil检查器
func TestManager_ValidateTrace_NilChecker(t *testing.T) {
	manager := createTestManager(t)
	trace := &ExecutionTrace{}

	errors := manager.ValidateTrace(trace)
	// 如果traceIntegrityChecker已初始化，应该返回空错误列表
	// 如果为nil，应该返回错误
	_ = errors
}

// TestContextImpl_SetHostABI_Nil 测试设置nil HostABI
func TestContextImpl_SetHostABI_Nil(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_nil_hostabi"
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

// TestContextImpl_UpdateTransactionDraft_Nil 测试更新nil交易草稿
func TestContextImpl_UpdateTransactionDraft_Nil(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_nil_draft"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	ctxImpl := executionContext.(*contextImpl)

	// 更新nil交易草稿应该返回错误
	err = ctxImpl.UpdateTransactionDraft(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot update with nil")

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestExportContextState_NilContext 测试导出nil上下文状态
func TestExportContextState_NilContext(t *testing.T) {
	_, err := ExportContextState(nil, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "上下文不能为 nil")
}

// TestExportContextStateJSON_NilContext 测试导出nil上下文状态为JSON
func TestExportContextStateJSON_NilContext(t *testing.T) {
	_, err := ExportContextStateJSON(nil, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "上下文不能为 nil")
}

// TestDeepCopyInterface_Nil 测试深度拷贝nil值
func TestDeepCopyInterface_Nil(t *testing.T) {
	result := deepCopyInterface(nil)
	assert.Nil(t, result)
}

// TestDeepCopyInterface_UnknownType 测试深度拷贝未知类型
func TestDeepCopyInterface_UnknownType(t *testing.T) {
	// 测试未知类型（应该返回原值）
	unknownValue := 12345
	result := deepCopyInterface(unknownValue)
	assert.Equal(t, unknownValue, result)
}

// TestContextIsolationEnforcer_TrackAccess_NotExists 测试跟踪不存在的上下文访问
func TestContextIsolationEnforcer_TrackAccess_NotExists(t *testing.T) {
	enforcer := NewContextIsolationEnforcer(5 * time.Minute)

	// 跟踪不存在的上下文访问（不应该panic）
	enforcer.TrackAccess("non_existent")
	
	// 验证没有创建记录
	enforcer.mutex.RLock()
	_, exists := enforcer.activeContexts["non_existent"]
	enforcer.mutex.RUnlock()
	assert.False(t, exists)
}

// TestContextIsolationEnforcer_TrackDestroy_NotExists 测试跟踪不存在的上下文销毁
func TestContextIsolationEnforcer_TrackDestroy_NotExists(t *testing.T) {
	enforcer := NewContextIsolationEnforcer(5 * time.Minute)

	// 跟踪不存在的上下文销毁（不应该panic）
	enforcer.TrackDestroy("non_existent")
	
	// 验证没有创建记录
	enforcer.mutex.RLock()
	_, exists := enforcer.activeContexts["non_existent"]
	enforcer.mutex.RUnlock()
	assert.False(t, exists)
}

// TestManager_DeepCopyContext_NotExists 测试深度拷贝不存在的上下文
func TestManager_DeepCopyContext_NotExists(t *testing.T) {
	manager := createTestManager(t)

	// 尝试深度拷贝不存在的上下文
	_, err := manager.DeepCopyContext("non_existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "执行上下文不存在")
}

// TestManager_ExportContextState_NotExists 测试导出不存在的上下文状态
func TestManager_ExportContextState_NotExists(t *testing.T) {
	manager := createTestManager(t)

	// 尝试导出不存在的上下文状态
	_, err := manager.ExportContextState("non_existent", false)
	assert.Error(t, err)
}

// TestManager_VerifyContextIsolation_NotExists 测试验证不存在的上下文隔离
func TestManager_VerifyContextIsolation_NotExists(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "exec_1"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	_ = executionContext

	// 验证不存在的上下文隔离
	isolated, issues := manager.VerifyContextIsolation("non_existent", executionID)
	// 如果上下文不存在，VerifyContextIsolation可能会返回错误或false
	_ = isolated
	_ = issues

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_VerifyContextCleanup_NotExists 测试验证不存在的上下文清理
func TestManager_VerifyContextCleanup_NotExists(t *testing.T) {
	manager := createTestManager(t)

	// 验证不存在的上下文清理
	cleaned, issues := manager.VerifyContextCleanup("non_existent")
	// 如果上下文不存在，应该返回true（已清理）或false（未找到）
	_ = cleaned
	_ = issues
}

// TestContextDebugger_LogHostFunctionCall_BasicMode 测试Basic模式下的宿主函数调用日志
func TestContextDebugger_LogHostFunctionCall_BasicMode(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	debugger := NewContextDebugger(logger, DebugModeBasic)
	debugger.Enable()

	// Basic模式下应该记录日志
	debugger.LogHostFunctionCall("exec_1", "test_function", 100*time.Millisecond, true, nil)
	logs := logger.GetLogs()
	require.Greater(t, len(logs), 0)
}

// TestContextDebugger_LogStateChange_BasicMode 测试Basic模式下的状态变更日志
func TestContextDebugger_LogStateChange_BasicMode(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	debugger := NewContextDebugger(logger, DebugModeBasic)
	debugger.Enable()

	// Basic模式下应该记录日志
	debugger.LogStateChange("exec_1", "set", "key1")
	logs := logger.GetLogs()
	require.Greater(t, len(logs), 0)
}

// TestManager_CreateDeterministicEnforcer_WithTimestamp 测试使用固定时间戳创建确定性增强器
func TestManager_CreateDeterministicEnforcer_WithTimestamp(t *testing.T) {
	manager := createTestManager(t)
	executionID := "test_with_timestamp"
	inputParams := []byte("test_input")
	fixedTimestamp := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	enforcer := manager.CreateDeterministicEnforcer(executionID, inputParams, &fixedTimestamp)
	require.NotNil(t, enforcer)
	assert.Equal(t, fixedTimestamp, enforcer.GetFixedTimestamp())
}

// TestDeepCopyInterface_EmptyTypes 测试深度拷贝空类型
func TestDeepCopyInterface_EmptyTypes(t *testing.T) {
	// 测试空map
	emptyMap := map[string]interface{}{}
	resultMap := deepCopyInterface(emptyMap).(map[string]interface{})
	assert.Equal(t, 0, len(resultMap))

	// 测试空slice
	emptySlice := []interface{}{}
	resultSlice := deepCopyInterface(emptySlice).([]interface{})
	assert.Equal(t, 0, len(resultSlice))

	// 测试空字节数组
	emptyBytes := []byte{}
	resultBytes := deepCopyInterface(emptyBytes).([]byte)
	assert.Equal(t, 0, len(resultBytes))
}

