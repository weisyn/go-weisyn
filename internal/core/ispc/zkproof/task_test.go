package zkproof

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// task.go 测试
// ============================================================================

// TestNewZKProofTask 测试创建ZK证明任务
func TestNewZKProofTask(t *testing.T) {
	taskID := "test_task_1"
	executionID := "test_execution_1"
	input := &interfaces.ZKProofInput{
		CircuitID:      "contract_execution",
		CircuitVersion: 1,
		PublicInputs:   [][]byte{[]byte("test")},
	}
	executionResultHash := []byte("hash")
	executionTrace := []*interfaces.HostFunctionCall{}
	priority := 10
	timeout := 5 * time.Minute

	task := NewZKProofTask(taskID, executionID, input, executionResultHash, executionTrace, priority, timeout)
	require.NotNil(t, task)
	require.Equal(t, taskID, task.TaskID)
	require.Equal(t, executionID, task.ExecutionID)
	require.Equal(t, input, task.Input)
	require.Equal(t, executionResultHash, task.ExecutionResultHash)
	require.Equal(t, priority, task.Priority)
	require.Equal(t, TaskStatusPending, task.Status)
	require.Equal(t, 3, task.MaxRetries)
	require.NotNil(t, task.Metadata)
}

// TestNewZKProofTask_DefaultTimeout 测试默认超时时间
func TestNewZKProofTask_DefaultTimeout(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, 0)
	require.NotNil(t, task)
	require.True(t, task.TimeoutAt.After(time.Now()))
	require.Equal(t, 5*time.Minute, task.TimeoutAt.Sub(task.CreatedAt))
}

// TestZKProofTask_IsExpired 测试任务是否过期
func TestZKProofTask_IsExpired(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, time.Minute)
	
	// 刚创建的任务不应该过期
	require.False(t, task.IsExpired())
	
	// 设置超时时间为过去
	task.TimeoutAt = time.Now().Add(-time.Minute)
	require.True(t, task.IsExpired())
}

// TestZKProofTask_CanRetry 测试任务是否可以重试
func TestZKProofTask_CanRetry(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, time.Minute)
	
	// 待处理状态不能重试
	require.False(t, task.CanRetry())
	
	// 失败状态且未达到最大重试次数可以重试
	task.Status = TaskStatusFailed
	task.RetryCount = 0
	task.MaxRetries = 3
	require.True(t, task.CanRetry())
	
	// 达到最大重试次数不能重试
	task.RetryCount = 3
	require.False(t, task.CanRetry())
	
	// 已完成状态不能重试
	task.Status = TaskStatusCompleted
	require.False(t, task.CanRetry())
}

// TestZKProofTask_MarkRunning 测试标记任务为运行中
func TestZKProofTask_MarkRunning(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, time.Minute)
	
	task.MarkRunning()
	require.Equal(t, TaskStatusRunning, task.Status)
	require.False(t, task.StartedAt.IsZero())
}

// TestZKProofTask_MarkCompleted 测试标记任务为已完成
func TestZKProofTask_MarkCompleted(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, time.Minute)
	proof := &transaction.ZKStateProof{
		CircuitId: "test_circuit",
	}
	
	task.MarkCompleted(proof)
	require.Equal(t, TaskStatusCompleted, task.Status)
	require.False(t, task.CompletedAt.IsZero())
	require.Equal(t, proof, task.ProofResult)
}

// TestZKProofTask_MarkFailed 测试标记任务为失败
func TestZKProofTask_MarkFailed(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, time.Minute)
	err := errors.New("test error")
	
	initialRetryCount := task.RetryCount
	task.MarkFailed(err)
	require.Equal(t, TaskStatusFailed, task.Status)
	require.False(t, task.CompletedAt.IsZero())
	require.Equal(t, err, task.Error)
	require.Equal(t, initialRetryCount+1, task.RetryCount)
}

// TestZKProofTask_MarkTimeout 测试标记任务为超时
func TestZKProofTask_MarkTimeout(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, time.Minute)
	
	task.MarkTimeout()
	require.Equal(t, TaskStatusTimeout, task.Status)
	require.False(t, task.CompletedAt.IsZero())
}

// TestZKProofTask_MarkCancelled 测试标记任务为已取消
func TestZKProofTask_MarkCancelled(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, time.Minute)
	
	task.MarkCancelled()
	require.Equal(t, TaskStatusCancelled, task.Status)
	require.False(t, task.CompletedAt.IsZero())
}

// TestZKProofTask_Serialize 测试任务序列化
func TestZKProofTask_Serialize(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{
		CircuitID:      "test_circuit",
		CircuitVersion: 1,
	}, []byte("hash"), nil, 10, time.Minute)
	task.Error = errors.New("test error")
	
	data, err := task.Serialize()
	require.NoError(t, err)
	require.NotEmpty(t, data)
	
	// 验证可以反序列化
	deserializedTask, err := DeserializeZKProofTask(data)
	require.NoError(t, err)
	require.Equal(t, task.TaskID, deserializedTask.TaskID)
	require.Equal(t, task.ExecutionID, deserializedTask.ExecutionID)
	require.Equal(t, task.Status, deserializedTask.Status)
}

// TestDeserializeZKProofTask 测试任务反序列化
func TestDeserializeZKProofTask(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{
		CircuitID:      "test_circuit",
		CircuitVersion: 1,
	}, []byte("hash"), nil, 10, time.Minute)
	
	data, err := task.Serialize()
	require.NoError(t, err)
	
	deserializedTask, err := DeserializeZKProofTask(data)
	require.NoError(t, err)
	require.Equal(t, task.TaskID, deserializedTask.TaskID)
	require.Equal(t, task.ExecutionID, deserializedTask.ExecutionID)
	require.Equal(t, task.Priority, deserializedTask.Priority)
	require.Equal(t, task.Status, deserializedTask.Status)
}

// TestDeserializeZKProofTask_InvalidJSON 测试无效JSON反序列化
func TestDeserializeZKProofTask_InvalidJSON(t *testing.T) {
	_, err := DeserializeZKProofTask([]byte("invalid json"))
	require.Error(t, err)
}

// TestZKProofTask_GetDuration 测试获取任务执行时长
func TestZKProofTask_GetDuration(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, time.Minute)
	
	// 未开始的任务
	require.Equal(t, time.Duration(0), task.GetDuration())
	
	// 已开始但未完成的任务
	task.MarkRunning()
	time.Sleep(10 * time.Millisecond)
	duration := task.GetDuration()
	require.Greater(t, duration, time.Duration(0))
	
	// 已完成的任务
	task.MarkCompleted(nil)
	completedDuration := task.GetDuration()
	require.GreaterOrEqual(t, completedDuration, duration)
}

// TestZKProofTask_GetWaitTime 测试获取任务等待时长
func TestZKProofTask_GetWaitTime(t *testing.T) {
	task := NewZKProofTask("task1", "exec1", &interfaces.ZKProofInput{}, []byte("hash"), nil, 0, time.Minute)
	
	// 未开始的任务
	waitTime := task.GetWaitTime()
	require.Greater(t, waitTime, time.Duration(0))
	
	// 已开始的任务
	task.MarkRunning()
	waitTime = task.GetWaitTime()
	require.GreaterOrEqual(t, waitTime, time.Duration(0))
}

// TestTaskStatus_Constants 测试任务状态常量
func TestTaskStatus_Constants(t *testing.T) {
	require.Equal(t, TaskStatus("pending"), TaskStatusPending)
	require.Equal(t, TaskStatus("running"), TaskStatusRunning)
	require.Equal(t, TaskStatus("completed"), TaskStatusCompleted)
	require.Equal(t, TaskStatus("failed"), TaskStatusFailed)
	require.Equal(t, TaskStatus("timeout"), TaskStatusTimeout)
	require.Equal(t, TaskStatus("cancelled"), TaskStatusCancelled)
}
