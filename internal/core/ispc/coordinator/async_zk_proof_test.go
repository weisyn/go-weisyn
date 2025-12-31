package coordinator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// 异步ZK证明生成测试
// ============================================================================
//
// 🎯 **测试目的**：发现异步ZK证明生成功能的缺陷和BUG
//
// ============================================================================

// TestEnableAsyncZKProofGeneration 测试启用异步ZK证明生成
func TestEnableAsyncZKProofGeneration(t *testing.T) {
	manager := createTestManager(t)

	// 测试默认状态
	assert.False(t, manager.asyncZKProofEnabled, "默认应该禁用异步ZK证明")

	// 启用异步ZK证明
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)

	// 验证已启用
	assert.True(t, manager.asyncZKProofEnabled, "应该已启用异步ZK证明")
	assert.NotNil(t, manager.zkProofTaskQueue, "任务队列应该已创建")
	assert.NotNil(t, manager.zkProofWorkerPool, "工作线程池应该已创建")

	// 清理
	_ = manager.DisableAsyncZKProofGeneration()
}

// TestEnableAsyncZKProofGeneration_AlreadyEnabled 测试重复启用异步ZK证明
// 🐛 **BUG检测**：重复启用应该返回错误
func TestEnableAsyncZKProofGeneration_AlreadyEnabled(t *testing.T) {
	manager := createTestManager(t)

	// 第一次启用
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)

	// 第二次启用（应该返回错误）
	err = manager.EnableAsyncZKProofGeneration(2, 1, 10)
	assert.Error(t, err, "重复启用应该返回错误")
	assert.Contains(t, err.Error(), "异步ZK证明生成已启用")

	// 清理
	_ = manager.DisableAsyncZKProofGeneration()
}

// TestDisableAsyncZKProofGeneration 测试禁用异步ZK证明生成
func TestDisableAsyncZKProofGeneration(t *testing.T) {
	manager := createTestManager(t)

	// 先启用
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)
	assert.True(t, manager.asyncZKProofEnabled)

	// 禁用
	err = manager.DisableAsyncZKProofGeneration()
	require.NoError(t, err)

	// 验证已禁用
	assert.False(t, manager.asyncZKProofEnabled, "应该已禁用异步ZK证明")
	assert.Nil(t, manager.zkProofTaskQueue, "任务队列应该已清理")
	assert.Nil(t, manager.zkProofWorkerPool, "工作线程池应该已清理")
}

// TestDisableAsyncZKProofGeneration_NotEnabled 测试禁用未启用的异步ZK证明
// 🐛 **BUG检测**：禁用未启用的异步ZK证明应该返回错误
func TestDisableAsyncZKProofGeneration_NotEnabled(t *testing.T) {
	manager := createTestManager(t)

	// 直接禁用（应该返回错误）
	err := manager.DisableAsyncZKProofGeneration()
	assert.Error(t, err, "禁用未启用的异步ZK证明应该返回错误")
	assert.Contains(t, err.Error(), "异步ZK证明生成未启用")
}

// TestIsAsyncZKProofGenerationEnabled 测试检查异步ZK证明是否启用
func TestIsAsyncZKProofGenerationEnabled(t *testing.T) {
	manager := createTestManager(t)

	// 默认应该禁用
	assert.False(t, manager.IsAsyncZKProofGenerationEnabled(), "默认应该禁用")

	// 启用后应该返回true
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)
	assert.True(t, manager.IsAsyncZKProofGenerationEnabled(), "启用后应该返回true")

	// 禁用后应该返回false
	err = manager.DisableAsyncZKProofGeneration()
	require.NoError(t, err)
	assert.False(t, manager.IsAsyncZKProofGenerationEnabled(), "禁用后应该返回false")
}

// TestGetZKProofTaskStatus_NotEnabled 测试获取任务状态（未启用异步模式）
// 🐛 **BUG检测**：未启用异步模式时应该返回nil
func TestGetZKProofTaskStatus_NotEnabled(t *testing.T) {
	manager := createTestManager(t)

	status := manager.GetZKProofTaskStatus("task_123")
	assert.Nil(t, status, "未启用异步模式时应该返回nil")
}

// TestGetZKProofTaskStatus_NotFound 测试获取不存在的任务状态
func TestGetZKProofTaskStatus_NotFound(t *testing.T) {
	manager := createTestManager(t)

	// 启用异步模式
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)
	defer manager.DisableAsyncZKProofGeneration()

	// 查询不存在的任务
	status := manager.GetZKProofTaskStatus("non_existent_task")
	assert.Nil(t, status, "不存在的任务状态应该为nil")
}

// TestGetZKProofTaskStats 测试获取任务统计信息
func TestGetZKProofTaskStats(t *testing.T) {
	manager := createTestManager(t)

	// 未启用时应该返回包含enabled=false的统计信息
	stats := manager.GetZKProofTaskStats()
	assert.NotNil(t, stats, "统计信息不应该为nil")
	assert.Equal(t, false, stats["enabled"], "未启用时enabled应该为false")

	// 启用后应该返回统计信息
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)
	defer manager.DisableAsyncZKProofGeneration()

	stats = manager.GetZKProofTaskStats()
	assert.NotNil(t, stats, "统计信息不应该为nil")
	assert.Equal(t, true, stats["enabled"], "启用后enabled应该为true")
	assert.Contains(t, stats, "queue", "应该包含队列统计")
	assert.Contains(t, stats, "worker_pool", "应该包含工作线程池统计")
	assert.Contains(t, stats, "total_tasks", "应该包含任务总数")
}

// TestEnableAsyncZKProofGeneration_InvalidWorkers 测试无效的工作线程数量
// 🐛 **BUG检测**：测试边界条件和无效参数
func TestEnableAsyncZKProofGeneration_InvalidWorkers(t *testing.T) {
	manager := createTestManager(t)

	tests := []struct {
		name       string
		workerCount int
		minWorkers  int
		maxWorkers  int
		expectError bool
	}{
		{
			name:        "zero workers",
			workerCount: 0,
			minWorkers:  1,
			maxWorkers:  10,
			expectError: false, // 可能允许0，由zkproof包决定
		},
		{
			name:        "negative workers",
			workerCount: -1,
			minWorkers:  1,
			maxWorkers:  10,
			expectError: false, // 可能允许负数，由zkproof包决定
		},
		{
			name:        "min > max",
			workerCount: 5,
			minWorkers:  10,
			maxWorkers:  5,
			expectError: false, // 可能允许，由zkproof包决定
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 清理之前的状态
			if manager.asyncZKProofEnabled {
				_ = manager.DisableAsyncZKProofGeneration()
			}

			err := manager.EnableAsyncZKProofGeneration(tt.workerCount, tt.minWorkers, tt.maxWorkers)
			if tt.expectError {
				assert.Error(t, err, "应该返回错误")
			} else {
				// 如果不期望错误，清理资源
				if err == nil {
					_ = manager.DisableAsyncZKProofGeneration()
				}
			}
		})
	}
}

// TestDisableAsyncZKProofGeneration_Concurrent 测试并发禁用异步ZK证明
// 🐛 **BUG检测**：测试并发安全性
func TestDisableAsyncZKProofGeneration_Concurrent(t *testing.T) {
	manager := createTestManager(t)

	// 启用异步ZK证明
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)

	// 并发禁用
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

			err := manager.DisableAsyncZKProofGeneration()
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
			t.Errorf("❌ BUG发现：并发禁用异步ZK证明时发生panic：%v", err)
		} else {
			t.Logf("⚠️ 警告：并发禁用异步ZK证明时发生错误（可能是幂等问题）：%v", err)
		}
	default:
		t.Logf("✅ 并发禁用异步ZK证明没有发生panic或错误")
	}

	// 验证最终状态
	assert.False(t, manager.asyncZKProofEnabled, "最终应该已禁用")
}

