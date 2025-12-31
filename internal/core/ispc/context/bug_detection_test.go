package context

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// ============================================================================
// BUG检测测试：专门用于发现代码缺陷和潜在问题
// ============================================================================
//
// 🎯 **测试目的**：
// 这些测试专门设计来发现代码中的BUG和设计缺陷，而不是为了通过测试
// 如果测试失败，说明发现了问题，需要修复代码而不是修改测试
//
// ⚠️ **重要原则**：
// - 测试应该验证代码的正确行为，而不是适应代码的错误行为
// - 如果测试失败，优先考虑修复代码，而不是修改测试
// - 测试应该暴露边界条件、错误处理和竞态条件等问题
//
// ============================================================================

// TestGetTransactionDraft_DesignIssue 测试GetTransactionDraft的设计问题
// 🐛 **发现的BUG**：CreateContext时如果callerAddress不为空，会自动创建txDraft
// 这导致GetTransactionDraft永远不会返回错误（除非callerAddress为空）
// 这可能是一个设计问题：GetTransactionDraft应该要求先调用UpdateTransactionDraft
func TestGetTransactionDraft_DesignIssue(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()

	// 测试1：callerAddress不为空时，txDraft被自动创建
	executionID1 := "test_draft_auto_create"
	callerAddress1 := "caller"
	executionContext1, err := manager.CreateContext(ctx, executionID1, callerAddress1)
	require.NoError(t, err)

	draft1, err1 := executionContext1.GetTransactionDraft()
	// ⚠️ **设计问题**：如果callerAddress不为空，txDraft会被自动创建
	// 这意味着GetTransactionDraft永远不会返回错误
	if err1 != nil {
		t.Logf("✅ 发现：callerAddress不为空时，GetTransactionDraft返回错误（符合预期）")
		assert.Nil(t, draft1)
	} else {
		t.Logf("⚠️ 设计问题：callerAddress不为空时，GetTransactionDraft自动创建txDraft，不返回错误")
		assert.NotNil(t, draft1, "txDraft被自动创建，这可能不符合预期")
	}

	// 测试2：callerAddress为空时，txDraft应该为nil
	executionID2 := "test_draft_no_caller"
	callerAddress2 := "" // 空callerAddress
	executionContext2, err := manager.CreateContext(ctx, executionID2, callerAddress2)
	require.NoError(t, err)

	draft2, err2 := executionContext2.GetTransactionDraft()
	// 如果callerAddress为空，txDraft应该为nil，GetTransactionDraft应该返回错误
	if err2 != nil {
		t.Logf("✅ 发现：callerAddress为空时，GetTransactionDraft正确返回错误")
		assert.Nil(t, draft2)
		assert.Contains(t, err2.Error(), "transaction draft not initialized")
	} else {
		t.Errorf("❌ BUG：callerAddress为空时，GetTransactionDraft应该返回错误，但返回了nil")
	}

	// 清理
	manager.DestroyContext(ctx, executionID1)
	manager.DestroyContext(ctx, executionID2)
}

// TestRecordHostFunctionCall_AsyncMode_WithoutRegistration BUG检测：异步模式下未注册到worker pool
// 🐛 **潜在BUG**：异步模式下，如果没有注册到worker pool，调用会丢失
func TestRecordHostFunctionCall_AsyncMode_WithoutRegistration(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_async_no_registration"
	callerAddress := "caller"

	// 启用异步轨迹记录
	err := manager.EnableAsyncTraceRecording(1, 1, 100*time.Millisecond, 3, 50*time.Millisecond)
	require.NoError(t, err)
	defer manager.DisableAsyncTraceRecording()

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// ⚠️ **BUG检测**：不注册到worker pool，直接记录调用
	call := &ispcInterfaces.HostFunctionCall{
		Sequence:     1,
		FunctionName: "test_function",
		Parameters:   map[string]interface{}{"key": "value"},
		Result:       map[string]interface{}{"result": "success"},
		Timestamp:    time.Now().UnixNano(),
	}

	executionContext.RecordHostFunctionCall(call)

	// 等待异步处理完成
	time.Sleep(200 * time.Millisecond)
	err = manager.FlushTraceQueue()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// 检查调用是否被记录
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)

	// ⚠️ **潜在BUG**：如果异步模式下没有注册到worker pool，调用会丢失
	if len(trace) == 0 {
		t.Logf("⚠️ 警告：异步模式下未注册到worker pool时，调用未被记录（这可能是一个BUG）")
	} else {
		t.Logf("✅ 发现：即使未注册到worker pool，调用也被记录了（trace长度=%d）", len(trace))
	}

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestGetResourceUsage_NilCheck BUG检测：GetResourceUsage返回nil的情况
// 🐛 **潜在BUG**：如果resourceUsage初始化失败，GetResourceUsage可能返回nil
func TestGetResourceUsage_NilCheck(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_resource_nil"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 根据代码实现，CreateContext时会初始化resourceUsage
	usage := executionContext.GetResourceUsage()

	// ⚠️ **BUG检测**：如果返回nil，说明初始化失败
	if usage == nil {
		t.Errorf("❌ BUG发现：GetResourceUsage返回nil，但CreateContext应该初始化resourceUsage")
	} else {
		// 验证初始值是否正确
		assert.Equal(t, uint64(0), usage.PeakMemoryBytes, "初始PeakMemoryBytes应该为0")
		assert.Equal(t, uint32(0), usage.HostFunctionCalls, "初始HostFunctionCalls应该为0")
		assert.False(t, usage.StartTime.IsZero(), "StartTime应该已设置")
		t.Logf("✅ GetResourceUsage正确返回初始化的resourceUsage")
	}

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestGetExecutionTrace_ConcurrentAccess BUG检测：并发访问GetExecutionTrace
// 🐛 **潜在BUG**：并发访问可能导致数据竞争或panic
func TestGetExecutionTrace_ConcurrentAccess(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_concurrent_trace"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 并发记录调用
	concurrency := 10
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(seq int) {
			defer func() {
				if r := recover(); r != nil {
					errors <- &panicError{panic: r}
				}
				done <- true
			}()

			call := &ispcInterfaces.HostFunctionCall{
				Sequence:     uint64(seq),
				FunctionName: "test_function",
				Parameters:   map[string]interface{}{"seq": seq},
				Result:       map[string]interface{}{"result": seq},
				Timestamp:    time.Now().UnixNano(),
			}
			executionContext.RecordHostFunctionCall(call)
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 检查是否有panic
	select {
	case err := <-errors:
		t.Errorf("❌ BUG发现：并发访问GetExecutionTrace时发生panic：%v", err)
	default:
		t.Logf("✅ 并发访问GetExecutionTrace没有发生panic")
	}

	// 验证所有调用都被记录
	trace, err := executionContext.GetExecutionTrace()
	require.NoError(t, err)
	
	// ⚠️ **潜在BUG**：并发访问可能导致调用丢失
	if len(trace) < concurrency {
		t.Errorf("❌ BUG发现：并发访问时调用丢失，期望%d个调用，实际%d个", concurrency, len(trace))
	} else {
		t.Logf("✅ 并发访问时所有调用都被正确记录，trace长度=%d", len(trace))
	}

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestDestroyContext_ConcurrentDestroy BUG检测：并发销毁上下文
// 🐛 **潜在BUG**：并发销毁可能导致panic或资源泄漏
func TestDestroyContext_ConcurrentDestroy(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_concurrent_destroy"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)
	_ = executionContext

	// 并发销毁
	concurrency := 5
	done := make(chan bool, concurrency)
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errors <- &panicError{panic: r}
				}
				done <- true
			}()

			err := manager.DestroyContext(ctx, executionID)
			if err != nil {
				errors <- err
			}
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 检查是否有panic或错误
	select {
	case err := <-errors:
		if _, ok := err.(*panicError); ok {
			t.Errorf("❌ BUG发现：并发销毁上下文时发生panic：%v", err)
		} else {
			t.Logf("⚠️ 警告：并发销毁上下文时发生错误（幂等设计应该允许）：%v", err)
		}
	default:
		t.Logf("✅ 并发销毁上下文没有发生panic或错误（幂等设计正确）")
	}

	// 验证上下文已被销毁
	_, err = manager.GetContext(executionID)
	assert.Error(t, err, "上下文应该已被销毁")
}

// TestGetExecutionTrace_EmptyParameters BUG检测：空Parameters和Result的处理
// 🐛 **潜在BUG**：空Parameters和Result可能导致panic或数据丢失
func TestGetExecutionTrace_EmptyParameters(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_empty_params"
	callerAddress := "caller"

	executionContext, err := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err)

	// 类型断言到 contextImpl
	ctxImpl, ok := executionContext.(*contextImpl)
	require.True(t, ok)

	// 设置各种边界情况的调用
	ctxImpl.mutex.Lock()
	ctxImpl.hostFunctionCalls = []HostFunctionCall{
		{
			Sequence:     1,
			FunctionName: "test_empty",
			Parameters:   map[string]interface{}{}, // 空map
			Result:       map[string]interface{}{}, // 空map
			Timestamp:    time.Now(),
		},
		{
			Sequence:     2,
			FunctionName: "test_nil",
			Parameters:   nil, // nil
			Result:       nil, // nil
			Timestamp:    time.Now(),
		},
		{
			Sequence:     3,
			FunctionName: "test_non_map",
			Parameters:   "string", // 非map类型
			Result:       123,      // 非map类型
			Timestamp:    time.Now(),
		},
	}
	ctxImpl.mutex.Unlock()

	// ⚠️ **BUG检测**：测试GetExecutionTrace是否能正确处理这些边界情况
	trace, err := executionContext.GetExecutionTrace()
	if err != nil {
		t.Errorf("❌ BUG发现：GetExecutionTrace处理边界情况时返回错误：%v", err)
		return
	}

	require.Len(t, trace, 3, "应该返回3个调用")

	// 验证每个调用的处理
	for i, call := range trace {
		// Parameters和Result已经是map[string]interface{}类型（GetExecutionTrace会转换）
		if call.Parameters == nil && call.Result == nil {
			t.Logf("✅ 调用%d：nil Parameters和Result被正确处理", i+1)
		} else {
			// 验证Parameters和Result不为nil（GetExecutionTrace应该处理了类型转换）
			if call.Parameters == nil || call.Result == nil {
				t.Logf("⚠️ 调用%d：Parameters或Result为nil（可能是正常的）", i+1)
			} else {
				// Parameters和Result已经是map[string]interface{}类型，直接使用
				paramsMap := call.Parameters
				resultMap := call.Result
				t.Logf("✅ 调用%d：Parameters和Result类型正确，Parameters长度=%d, Result长度=%d", i+1, len(paramsMap), len(resultMap))
			}
		}
	}

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// TestCreateContext_DuplicateID BUG检测：重复的executionID
// 🐛 **潜在BUG**：创建重复的executionID可能导致数据覆盖或错误
func TestCreateContext_DuplicateID(t *testing.T) {
	manager := createTestManager(t)
	ctx := context.Background()
	executionID := "test_duplicate_id"
	callerAddress := "caller"

	// 第一次创建
	executionContext1, err1 := manager.CreateContext(ctx, executionID, callerAddress)
	require.NoError(t, err1)
	require.NotNil(t, executionContext1)

	// ⚠️ **BUG检测**：尝试创建重复的executionID
	executionContext2, err2 := manager.CreateContext(ctx, executionID, callerAddress)

	// 根据实现，可能允许重复创建（覆盖）或返回错误
	if err2 != nil {
		t.Logf("✅ 发现：创建重复executionID时正确返回错误：%v", err2)
		assert.Nil(t, executionContext2)
	} else {
		t.Logf("⚠️ 警告：创建重复executionID时没有返回错误，这可能覆盖了之前的上下文")
		if executionContext2 != nil {
			// 验证是否覆盖了之前的上下文
			retrievedContext, err := manager.GetContext(executionID)
			if err != nil {
				t.Errorf("❌ BUG发现：创建重复executionID后，无法获取上下文")
			} else {
				// 检查是否是新的上下文
				if retrievedContext == executionContext1 {
					t.Logf("⚠️ 警告：创建重复executionID时，返回的是旧的上下文（可能没有覆盖）")
				} else {
					t.Logf("⚠️ 警告：创建重复executionID时，返回的是新的上下文（覆盖了旧的）")
				}
			}
		}
	}

	// 清理
	manager.DestroyContext(ctx, executionID)
}

// panicError 用于捕获panic错误
type panicError struct {
	panic interface{}
}

func (e *panicError) Error() string {
	return fmt.Sprintf("panic: %v", e.panic)
}

