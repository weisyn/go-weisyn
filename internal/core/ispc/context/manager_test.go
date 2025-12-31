package context

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// Manager 核心功能测试
// ============================================================================
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
//
// ============================================================================

// createTestManager 创建测试用的Manager
//
// ✅ **重构说明**：使用testutil包中的统一Mock对象，遵循测试规范
func createTestManager(t *testing.T) *Manager {
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()
	clock := testutil.NewTestClock()

	return NewManager(logger, configProvider, clock)
}

// TestNewManager 测试创建Manager
func TestNewManager(t *testing.T) {
	logger := testutil.NewTestLogger()
	configProvider := testutil.NewTestConfigProvider()
	clock := testutil.NewTestClock()

	manager := NewManager(logger, configProvider, clock)
	require.NotNil(t, manager)
	require.NotNil(t, manager.logger)
	require.NotNil(t, manager.configProvider)
	require.NotNil(t, manager.clock)
	require.NotNil(t, manager.contexts)
	require.NotNil(t, manager.config)
	require.NotNil(t, manager.isolationEnforcer)
	require.NotNil(t, manager.cleanupVerifier)
	require.NotNil(t, manager.resultVerifier)
	require.NotNil(t, manager.traceIntegrityChecker)
	require.NotNil(t, manager.debugger)
	require.NotNil(t, manager.debugTool)
}

// TestManager_GetContext 测试获取执行上下文
func TestManager_GetContext(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_1"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取上下文
	retrievedContext, err := manager.GetContext(executionID)
	require.NoError(t, err)
	require.NotNil(t, retrievedContext)
	assert.Equal(t, executionContext, retrievedContext)

	// 获取不存在的上下文
	_, err = manager.GetContext("non_existent")
	require.Error(t, err)

	// 清理
	err = manager.DestroyContext(ctx, executionID)
	require.NoError(t, err)
}

// TestManager_ListContexts 测试列出所有上下文
func TestManager_ListContexts(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()

	// 创建多个上下文
	executionIDs := []string{"exec_1", "exec_2", "exec_3"}
	for _, id := range executionIDs {
		_, err := manager.CreateContext(ctx, id, "caller")
		require.NoError(t, err)
	}

	// 列出上下文
	contexts := manager.ListContexts()
	require.GreaterOrEqual(t, len(contexts), len(executionIDs))

	// 验证所有创建的上下文都在列表中
	foundIDs := make(map[string]bool)
	for _, execCtxID := range contexts {
		foundIDs[execCtxID] = true
	}

	for _, id := range executionIDs {
		assert.True(t, foundIDs[id], "上下文 %s 应该在列表中", id)
	}

	// 清理
	for _, id := range executionIDs {
		manager.DestroyContext(ctx, id)
	}
}

// TestManager_GetStats 测试获取统计信息
func TestManager_GetStats(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()

	// 创建一些上下文
	for i := 0; i < 3; i++ {
		_, err := manager.CreateContext(ctx, "exec_"+string(rune(i+'0')), "caller")
		require.NoError(t, err)
	}

	// 获取统计信息
	stats := manager.GetStats()
	require.NotNil(t, stats)
	// ⚠️ **代码行为**：GetStats返回的是"active_context_count"，不是"active_contexts"
	require.Contains(t, stats, "active_context_count")
	assert.GreaterOrEqual(t, stats["active_context_count"].(int), 3)

	// 清理
	for i := 0; i < 3; i++ {
		manager.DestroyContext(ctx, "exec_"+string(rune(i+'0')))
	}
}

// TestManager_GetDebugger 测试获取调试器
func TestManager_GetDebugger(t *testing.T) {
	manager := createTestManager(t)

	debugger := manager.GetDebugger()
	require.NotNil(t, debugger)
}

// TestManager_GetDebugTool 测试获取调试工具
func TestManager_GetDebugTool(t *testing.T) {
	manager := createTestManager(t)

	debugTool := manager.GetDebugTool()
	require.NotNil(t, debugTool)
}

// TestManager_SetDebugMode 测试设置调试模式
func TestManager_SetDebugMode(t *testing.T) {
	manager := createTestManager(t)

	// 设置调试模式
	manager.SetDebugMode(DebugModeVerbose)

	// 验证调试模式已设置
	debugger := manager.GetDebugger()
	require.NotNil(t, debugger)
	assert.Equal(t, DebugModeVerbose, debugger.GetDebugMode())
}

// TestManager_GetDeterministicClock 测试获取确定性时钟
func TestManager_GetDeterministicClock(t *testing.T) {
	manager := createTestManager(t)

	clock := manager.GetDeterministicClock()
	require.NotNil(t, clock)
}

// TestManager_GetDeterministicTimestamp 测试获取确定性时间戳
func TestManager_GetDeterministicTimestamp(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_timestamp"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取确定性时间戳（通过Manager的GetDeterministicClock）
	clock := manager.GetDeterministicClock()
	require.NotNil(t, clock)
	timestamp := clock.Now()
	require.False(t, timestamp.IsZero())

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_GetDeterministicRandomSource 测试获取确定性随机源
func TestManager_GetDeterministicRandomSource(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_random"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取确定性随机源（通过类型断言到contextImpl）
	// 注意：GetDeterministicRandomSource 是 contextImpl 的方法，不在接口中
	if ctxImpl, ok := executionContext.(*contextImpl); ok {
		randomSource := ctxImpl.GetDeterministicRandomSource()
		require.NotNil(t, randomSource)
	} else {
		t.Skip("无法获取确定性随机源：executionContext 不是 *contextImpl 类型")
	}

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_GetExecutionID 测试获取执行ID（通过contextImpl）
func TestManager_GetExecutionID(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_id"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取执行ID
	retrievedID := executionContext.GetExecutionID()
	assert.Equal(t, executionID, retrievedID)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_GetCallerAddress 测试获取调用者地址
func TestManager_GetCallerAddress(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_caller"
	// 注意：callerAddress 会被转换为20字节的地址
	callerAddress := "test_caller_address"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取调用者地址（返回20字节的地址）
	retrievedAddress := executionContext.GetCallerAddress()
	// callerAddress 会被转换为20字节的地址（hex解码或填充）
	// 如果callerAddress是hex字符串，会被解码；否则会被填充或截断到20字节
	assert.NotNil(t, retrievedAddress)
	if len(retrievedAddress) > 0 {
		assert.Equal(t, 20, len(retrievedAddress), "调用者地址应该是20字节")
	}

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_GetBlockHeight 测试获取区块高度
func TestManager_GetBlockHeight(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_block"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取区块高度
	height := executionContext.GetBlockHeight()
	assert.GreaterOrEqual(t, height, uint64(0))

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_GetBlockTimestamp 测试获取区块时间戳
func TestManager_GetBlockTimestamp(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_block_ts"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取区块时间戳
	timestamp := executionContext.GetBlockTimestamp()
	assert.GreaterOrEqual(t, timestamp, uint64(0))

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_GetChainID 测试获取链ID
func TestManager_GetChainID(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_chain"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取链ID
	chainID := executionContext.GetChainID()
	require.NotNil(t, chainID)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_GetDraftID 测试获取Draft ID
func TestManager_GetDraftID(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_draft"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取Draft ID
	draftID := executionContext.GetDraftID()
	require.NotEmpty(t, draftID)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_SetReturnData 测试设置返回数据
func TestManager_SetReturnData(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_return"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 设置返回数据
	returnData := []byte("test_return_data")
	err = executionContext.SetReturnData(returnData)
	require.NoError(t, err)

	// 获取返回数据
	retrievedData, err := executionContext.GetReturnData()
	require.NoError(t, err)
	assert.Equal(t, returnData, retrievedData)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_AddEvent 测试添加事件
func TestManager_AddEvent(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_event"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 添加事件
	event := &ispcInterfaces.Event{
		Type:      "test_event",
		Timestamp: time.Now().Unix(),
		Data:      map[string]interface{}{"key": "value"},
	}
	err = executionContext.AddEvent(event)
	require.NoError(t, err)

	// 获取事件
	events, err := executionContext.GetEvents()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(events), 1)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_GetResourceUsage 测试获取资源使用情况
func TestManager_GetResourceUsage(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_resource"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 获取资源使用情况
	resourceUsage := executionContext.GetResourceUsage()
	require.NotNil(t, resourceUsage)

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestManager_FinalizeResourceUsage 测试完成资源使用统计
func TestManager_FinalizeResourceUsage(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_execution_finalize"
	callerAddress := "test_caller"

	// 创建上下文
	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	require.NotNil(t, executionContext)

	// 完成资源使用统计
	executionContext.FinalizeResourceUsage()

	// 清理
	manager.DestroyContext(ctx, executionID)
}
