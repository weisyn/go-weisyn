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
// debug_tool.go 测试
// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// TestDebugMode_String 测试调试模式字符串表示
func TestDebugMode_String(t *testing.T) {
	assert.Equal(t, "off", DebugModeOff.String())
	assert.Equal(t, "basic", DebugModeBasic.String())
	assert.Equal(t, "verbose", DebugModeVerbose.String())
	assert.Equal(t, "unknown", DebugMode(999).String())
}

// TestContextDebugger_LogHostFunctionCall 测试记录宿主函数调用日志
func TestContextDebugger_LogHostFunctionCall(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	// ⚠️ **代码行为**：LogHostFunctionCall在Verbose模式下会直接返回，不记录日志
	// 所以应该使用Basic模式来测试日志记录功能
	debugger := NewContextDebugger(logger, DebugModeBasic)
	debugger.Enable()

	// 记录成功的宿主函数调用
	debugger.LogHostFunctionCall("exec_1", "test_function", 100*time.Millisecond, true, nil)

	// 记录失败的宿主函数调用
	err := assert.AnError
	debugger.LogHostFunctionCall("exec_1", "test_function", 50*time.Millisecond, false, err)

	// 验证日志被记录
	logs := logger.GetLogs()
	require.Greater(t, len(logs), 0)
}

// TestContextDebugger_LogStateChange 测试记录状态变更日志
func TestContextDebugger_LogStateChange(t *testing.T) {
	logger := testutil.NewTestBehavioralLogger()
	// 注意：LogStateChange 在 Verbose 模式下会直接返回，所以使用 Basic 模式
	debugger := NewContextDebugger(logger, DebugModeBasic)
	debugger.Enable()

	// 记录状态变更
	debugger.LogStateChange("exec_1", "set", "key1")
	debugger.LogStateChange("exec_1", "delete", "key2")

	// 验证日志被记录
	logs := logger.GetLogs()
	require.Greater(t, len(logs), 0)
}

// TestContextDebugger_EnableDisableIsEnabled 测试启用/禁用调试
func TestContextDebugger_EnableDisableIsEnabled(t *testing.T) {
	logger := testutil.NewTestLogger()
	debugger := NewContextDebugger(logger, DebugModeOff)

	// 初始状态应该是禁用
	assert.False(t, debugger.IsEnabled())

	// 启用调试
	debugger.Enable()
	assert.True(t, debugger.IsEnabled())

	// 禁用调试
	debugger.Disable()
	assert.False(t, debugger.IsEnabled())
}

// TestExportContextState 测试导出上下文状态
func TestExportContextState(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_export"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 导出状态（不包含堆栈跟踪）
	snapshot, err := ExportContextState(executionContext, false)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Equal(t, executionID, snapshot.ExecutionID)
	assert.NotZero(t, snapshot.CreatedAt)

	// 导出状态（包含堆栈跟踪）
	snapshot2, err := ExportContextState(executionContext, true)
	require.NoError(t, err)
	require.NotNil(t, snapshot2)
	assert.NotEmpty(t, snapshot2.StackTrace)

	// 测试导出nil上下文
	_, err = ExportContextState(nil, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "上下文不能为 nil")

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestExportContextStateJSON 测试导出上下文状态为JSON
func TestExportContextStateJSON(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_export_json"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 导出为JSON
	jsonData, err := ExportContextStateJSON(executionContext, false)
	require.NoError(t, err)
	require.NotEmpty(t, jsonData)
	assert.Contains(t, string(jsonData), executionID)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestDebugTool_ExecuteCommand 测试执行调试命令
func TestDebugTool_ExecuteCommand(t *testing.T) {
	manager := createTestManager(t)
	debugTool := manager.GetDebugTool()
	require.NotNil(t, debugTool)

	// 测试 list 命令
	result, err := debugTool.ExecuteCommand(DebugCommandList)
	require.NoError(t, err)
	resultMap, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, resultMap, "execution_ids")

	// 测试 stats 命令
	result, err = debugTool.ExecuteCommand(DebugCommandStats)
	require.NoError(t, err)
	resultMap, ok = result.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, resultMap, "active_context_count")

	// 测试无效命令
	_, err = debugTool.ExecuteCommand(DebugCommand("invalid"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未知的调试命令")
}

// TestDebugTool_listContexts 测试列出上下文
func TestDebugTool_listContexts(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()

	// 创建几个上下文
	executionIDs := []string{"exec_1", "exec_2"}
	for _, id := range executionIDs {
		_, err := manager.CreateContext(ctx, id, "caller")
		require.NoError(t, err)
	}

	debugTool := manager.GetDebugTool()
	result, err := debugTool.ExecuteCommand(DebugCommandList)
	require.NoError(t, err)
	resultMap := result.(map[string]interface{})
	executionIDsList := resultMap["execution_ids"].([]string)
	
	// 验证结果包含创建的上下文ID
	foundIDs := make(map[string]bool)
	for _, id := range executionIDsList {
		foundIDs[id] = true
	}
	for _, id := range executionIDs {
		assert.True(t, foundIDs[id], "上下文 %s 应该在列表中", id)
	}

	// 清理
	for _, id := range executionIDs {
		manager.DestroyContext(ctx, id)
	}
}

// TestDebugTool_showStats 测试显示统计信息
func TestDebugTool_showStats(t *testing.T) {
	manager := createTestManager(t)
	debugTool := manager.GetDebugTool()

	result, err := debugTool.ExecuteCommand(DebugCommandStats)
	require.NoError(t, err)
	resultMap := result.(map[string]interface{})
	assert.Contains(t, resultMap, "active_context_count")
}

// TestDebugTool_showContext 测试显示指定上下文
func TestDebugTool_showContext(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "exec_show"
	callerAddress := "caller"

	// 创建上下文
	_, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	debugTool := manager.GetDebugTool()
	result, err := debugTool.ExecuteCommand(DebugCommandShow, executionID)
	require.NoError(t, err)
	snapshot := result.(*ContextStateSnapshot)
	assert.Equal(t, executionID, snapshot.ExecutionID)

	// 测试不存在的上下文
	_, err = debugTool.ExecuteCommand(DebugCommandShow, "non_existent")
	require.Error(t, err)

	// 测试缺少参数
	_, err = debugTool.ExecuteCommand(DebugCommandShow)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "show命令需要executionID参数")

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestDebugTool_exportContext 测试导出上下文
func TestDebugTool_exportContext(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "exec_export"
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

	// 测试缺少参数
	_, err = debugTool.ExecuteCommand(DebugCommandExport)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "export命令需要executionID参数")

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestDebugTool_detectLeaks 测试检测上下文泄漏
func TestDebugTool_detectLeaks(t *testing.T) {
	manager := createTestManager(t)
	debugTool := manager.GetDebugTool()

	result, err := debugTool.ExecuteCommand(DebugCommandLeaks)
	require.NoError(t, err)
	resultMap := result.(map[string]interface{})
	assert.Contains(t, resultMap, "leaked_contexts")
}

