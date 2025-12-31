package zkproof

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// ZK证明生成任务定义（异步ZK证明生成优化 - 阶段1）
// ============================================================================
//
// 🎯 **设计目的**：
// 定义ZK证明生成任务的结构和接口，支持异步生成和状态管理。
//
// 🏗️ **实现策略**：
// - 定义任务结构体，包含所有必要的输入和元数据
// - 实现任务序列化，支持持久化
// - 添加任务状态管理
// - 支持任务优先级
//
// ⚠️ **注意**：
// - 任务需要包含完整的证明生成所需信息
// - 任务状态需要支持查询和更新
// - 任务需要支持超时机制
//
// ============================================================================

// ZKProofTask ZK证明生成任务
//
// 🎯 **核心职责**：
// - 封装ZK证明生成所需的所有信息
// - 支持任务状态管理
// - 支持任务优先级
type ZKProofTask struct {
	// 任务ID（唯一标识）
	TaskID string
	
	// 执行上下文ID（关联ExecutionContext）
	ExecutionID string
	
	// 证明生成输入
	Input *interfaces.ZKProofInput
	
	// 执行结果哈希（用于生成证明）
	ExecutionResultHash []byte
	
	// 执行轨迹（用于生成证明）
	ExecutionTrace []*interfaces.HostFunctionCall
	
	// 任务优先级（数字越大优先级越高）
	Priority int
	
	// 任务状态
	Status TaskStatus
	
	// 任务创建时间
	CreatedAt time.Time
	
	// 任务开始时间
	StartedAt time.Time
	
	// 任务完成时间
	CompletedAt time.Time
	
	// 任务超时时间
	TimeoutAt time.Time
	
	// 生成的证明结果（完成时填充）
	ProofResult *transaction.ZKStateProof
	
	// 错误信息（失败时填充）
	Error error
	
	// 重试次数
	RetryCount int
	
	// 最大重试次数
	MaxRetries int
	
	// 任务元数据（扩展字段）
	Metadata map[string]interface{}
}

// TaskStatus 任务状态
type TaskStatus string

const (
	// TaskStatusPending 待处理
	TaskStatusPending TaskStatus = "pending"
	
	// TaskStatusRunning 运行中
	TaskStatusRunning TaskStatus = "running"
	
	// TaskStatusCompleted 已完成
	TaskStatusCompleted TaskStatus = "completed"
	
	// TaskStatusFailed 失败
	TaskStatusFailed TaskStatus = "failed"
	
	// TaskStatusTimeout 超时
	TaskStatusTimeout TaskStatus = "timeout"
	
	// TaskStatusCancelled 已取消
	TaskStatusCancelled TaskStatus = "cancelled"
)

// NewZKProofTask 创建ZK证明生成任务
//
// 📋 **参数**：
//   - taskID: 任务ID
//   - executionID: 执行上下文ID
//   - input: 证明生成输入
//   - executionResultHash: 执行结果哈希
//   - executionTrace: 执行轨迹
//   - priority: 任务优先级（默认0）
//   - timeout: 任务超时时间（默认5分钟）
//
// 📋 **返回值**：
//   - *ZKProofTask: 任务实例
func NewZKProofTask(
	taskID string,
	executionID string,
	input *interfaces.ZKProofInput,
	executionResultHash []byte,
	executionTrace []*interfaces.HostFunctionCall,
	priority int,
	timeout time.Duration,
) *ZKProofTask {
	if timeout <= 0 {
		timeout = 5 * time.Minute // 默认5分钟超时
	}
	
	now := time.Now()
	return &ZKProofTask{
		TaskID:              taskID,
		ExecutionID:         executionID,
		Input:               input,
		ExecutionResultHash: executionResultHash,
		ExecutionTrace:      executionTrace,
		Priority:            priority,
		Status:              TaskStatusPending,
		CreatedAt:           now,
		TimeoutAt:           now.Add(timeout),
		MaxRetries:          3, // 默认最大重试3次
		Metadata:            make(map[string]interface{}),
	}
}

// IsExpired 检查任务是否已过期
func (t *ZKProofTask) IsExpired() bool {
	return time.Now().After(t.TimeoutAt)
}

// CanRetry 检查任务是否可以重试
func (t *ZKProofTask) CanRetry() bool {
	return t.Status == TaskStatusFailed && t.RetryCount < t.MaxRetries
}

// MarkRunning 标记任务为运行中
func (t *ZKProofTask) MarkRunning() {
	t.Status = TaskStatusRunning
	t.StartedAt = time.Now()
}

// MarkCompleted 标记任务为已完成
func (t *ZKProofTask) MarkCompleted(proof *transaction.ZKStateProof) {
	t.Status = TaskStatusCompleted
	t.CompletedAt = time.Now()
	t.ProofResult = proof
}

// MarkFailed 标记任务为失败
func (t *ZKProofTask) MarkFailed(err error) {
	t.Status = TaskStatusFailed
	t.CompletedAt = time.Now()
	t.Error = err
	t.RetryCount++
}

// MarkTimeout 标记任务为超时
func (t *ZKProofTask) MarkTimeout() {
	t.Status = TaskStatusTimeout
	t.CompletedAt = time.Now()
}

// MarkCancelled 标记任务为已取消
func (t *ZKProofTask) MarkCancelled() {
	t.Status = TaskStatusCancelled
	t.CompletedAt = time.Now()
}

// Serialize 序列化任务（用于持久化）
//
// 📋 **返回值**：
//   - []byte: 序列化后的JSON数据
//   - error: 序列化错误
func (t *ZKProofTask) Serialize() ([]byte, error) {
	// 注意：Error字段不能直接序列化，需要转换为字符串
	taskData := struct {
		TaskID              string                              `json:"task_id"`
		ExecutionID         string                              `json:"execution_id"`
		Input               *interfaces.ZKProofInput            `json:"input"`
		ExecutionResultHash []byte                              `json:"execution_result_hash"`
		ExecutionTrace      []*interfaces.HostFunctionCall      `json:"execution_trace"`
		Priority            int                                 `json:"priority"`
		Status              TaskStatus                          `json:"status"`
		CreatedAt           time.Time                           `json:"created_at"`
		StartedAt           time.Time                           `json:"started_at"`
		CompletedAt         time.Time                           `json:"completed_at"`
		TimeoutAt           time.Time                           `json:"timeout_at"`
		ProofResult         *transaction.ZKStateProof           `json:"proof_result"`
		Error               string                              `json:"error,omitempty"`
		RetryCount          int                                 `json:"retry_count"`
		MaxRetries          int                                 `json:"max_retries"`
		Metadata            map[string]interface{}              `json:"metadata"`
	}{
		TaskID:              t.TaskID,
		ExecutionID:         t.ExecutionID,
		Input:               t.Input,
		ExecutionResultHash: t.ExecutionResultHash,
		ExecutionTrace:      t.ExecutionTrace,
		Priority:            t.Priority,
		Status:              t.Status,
		CreatedAt:           t.CreatedAt,
		StartedAt:           t.StartedAt,
		CompletedAt:         t.CompletedAt,
		TimeoutAt:           t.TimeoutAt,
		ProofResult:         t.ProofResult,
		Error:               "",
		RetryCount:          t.RetryCount,
		MaxRetries:          t.MaxRetries,
		Metadata:            t.Metadata,
	}
	
	if t.Error != nil {
		taskData.Error = t.Error.Error()
	}
	
	data, err := json.Marshal(taskData)
	if err != nil {
		return nil, fmt.Errorf("序列化ZKProofTask失败: %w", err)
	}
	return data, nil
}

// Deserialize 反序列化任务（从持久化数据恢复）
//
// 📋 **参数**：
//   - data: 序列化后的JSON数据
//
// 📋 **返回值**：
//   - *ZKProofTask: 任务实例
//   - error: 反序列化错误
func DeserializeZKProofTask(data []byte) (*ZKProofTask, error) {
	var taskData struct {
		TaskID              string                              `json:"task_id"`
		ExecutionID         string                              `json:"execution_id"`
		Input               *interfaces.ZKProofInput            `json:"input"`
		ExecutionResultHash []byte                              `json:"execution_result_hash"`
		ExecutionTrace      []*interfaces.HostFunctionCall      `json:"execution_trace"`
		Priority            int                                 `json:"priority"`
		Status              TaskStatus                          `json:"status"`
		CreatedAt           time.Time                           `json:"created_at"`
		StartedAt           time.Time                           `json:"started_at"`
		CompletedAt         time.Time                           `json:"completed_at"`
		TimeoutAt           time.Time                           `json:"timeout_at"`
		ProofResult         *transaction.ZKStateProof          `json:"proof_result"`
		Error               string                              `json:"error,omitempty"`
		RetryCount          int                                 `json:"retry_count"`
		MaxRetries          int                                 `json:"max_retries"`
		Metadata            map[string]interface{}              `json:"metadata"`
	}
	
	if err := json.Unmarshal(data, &taskData); err != nil {
		return nil, fmt.Errorf("反序列化ZKProofTask失败: %w", err)
	}
	
	task := &ZKProofTask{
		TaskID:              taskData.TaskID,
		ExecutionID:         taskData.ExecutionID,
		Input:               taskData.Input,
		ExecutionResultHash: taskData.ExecutionResultHash,
		ExecutionTrace:      taskData.ExecutionTrace,
		Priority:            taskData.Priority,
		Status:              taskData.Status,
		CreatedAt:           taskData.CreatedAt,
		StartedAt:           taskData.StartedAt,
		CompletedAt:         taskData.CompletedAt,
		TimeoutAt:           taskData.TimeoutAt,
		ProofResult:         taskData.ProofResult,
		RetryCount:          taskData.RetryCount,
		MaxRetries:          taskData.MaxRetries,
		Metadata:            taskData.Metadata,
	}
	
	if taskData.Error != "" {
		task.Error = fmt.Errorf("%s", taskData.Error)
	}
	
	return task, nil
}

// GetDuration 获取任务执行时长
func (t *ZKProofTask) GetDuration() time.Duration {
	if t.StartedAt.IsZero() {
		return 0
	}
	
	endTime := t.CompletedAt
	if endTime.IsZero() {
		endTime = time.Now()
	}
	
	return endTime.Sub(t.StartedAt)
}

// GetWaitTime 获取任务等待时长
func (t *ZKProofTask) GetWaitTime() time.Duration {
	if t.StartedAt.IsZero() {
		return time.Since(t.CreatedAt)
	}
	return t.StartedAt.Sub(t.CreatedAt)
}

