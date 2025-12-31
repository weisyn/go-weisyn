package coordinator

import (
	"context"
	"fmt"
	"time"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/zkproof"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ==================== P0: 异步ZK证明生成管理方法 ====================

// EnableAsyncZKProofGeneration 启用异步ZK证明生成
//
// 🎯 **异步ZK证明生成**：
// - 创建任务队列和工作线程池
// - 启动工作线程池
// - 后续的ZK证明生成将使用异步模式
//
// 📋 **参数**：
//   - workerCount: 工作线程数量（默认2）
//   - minWorkers: 最小工作线程数量（默认1）
//   - maxWorkers: 最大工作线程数量（默认10）
func (m *Manager) EnableAsyncZKProofGeneration(workerCount int, minWorkers int, maxWorkers int) error {
	if m.asyncZKProofEnabled {
		return fmt.Errorf("异步ZK证明生成已启用")
	}

	// 创建任务队列
	m.zkProofTaskQueue = zkproof.NewZKProofTaskQueue(m.logger)
	m.zkProofTaskQueue.Start()

	// 创建回调函数
	callback := func(task *zkproof.ZKProofTask, proof *transaction.ZKStateProof, err error) {
		m.handleZKProofCallback(task, proof, err)
	}

	// 创建工作线程池
	m.zkProofWorkerPool = zkproof.NewZKProofWorkerPool(
		m.zkProofTaskQueue,
		m.zkproofManager,
		callback,
		workerCount,
		minWorkers,
		maxWorkers,
		m.logger,
	)

	// 启动工作线程池
	m.zkProofWorkerPool.Start()

	m.asyncZKProofEnabled = true

	if m.logger != nil {
		m.logger.Infof("✅ 异步ZK证明生成已启用: workerCount=%d, minWorkers=%d, maxWorkers=%d", workerCount, minWorkers, maxWorkers)
	}

	return nil
}

// DisableAsyncZKProofGeneration 禁用异步ZK证明生成
//
// 🎯 **禁用异步ZK证明生成**：
// - 停止工作线程池
// - 停止任务队列
// - 后续的ZK证明生成将使用同步模式
func (m *Manager) DisableAsyncZKProofGeneration() error {
	if !m.asyncZKProofEnabled {
		return fmt.Errorf("异步ZK证明生成未启用")
	}

	// 停止工作线程池
	if m.zkProofWorkerPool != nil {
		m.zkProofWorkerPool.Stop()
		m.zkProofWorkerPool = nil
	}

	// 停止任务队列
	if m.zkProofTaskQueue != nil {
		m.zkProofTaskQueue.Stop()
		m.zkProofTaskQueue = nil
	}

	m.asyncZKProofEnabled = false

	if m.logger != nil {
		m.logger.Infof("✅ 异步ZK证明生成已禁用")
	}

	return nil
}

// GetZKProofTaskStatus 获取ZK证明任务状态
//
// 📋 **参数**：
//   - taskID: 任务ID
//
// 📋 **返回值**：
//   - *zkproof.ZKProofTask: 任务实例（如果不存在返回nil）
func (m *Manager) GetZKProofTaskStatus(taskID string) *zkproof.ZKProofTask {
	if !m.asyncZKProofEnabled || m.zkProofTaskQueue == nil {
		return nil
	}

	m.zkProofTaskMutex.RLock()
	defer m.zkProofTaskMutex.RUnlock()

	return m.zkProofTaskStore[taskID]
}

// GetZKProofTaskStats 获取ZK证明任务统计信息
//
// 📋 **返回值**：
//   - map[string]interface{}: 统计信息（队列统计和工作线程池统计）
func (m *Manager) GetZKProofTaskStats() map[string]interface{} {
	if !m.asyncZKProofEnabled {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	stats := make(map[string]interface{})
	stats["enabled"] = true

	if m.zkProofTaskQueue != nil {
		stats["queue"] = m.zkProofTaskQueue.GetStats()
	}

	if m.zkProofWorkerPool != nil {
		stats["worker_pool"] = m.zkProofWorkerPool.GetStats()
	}

	m.zkProofTaskMutex.RLock()
	stats["total_tasks"] = len(m.zkProofTaskStore)
	m.zkProofTaskMutex.RUnlock()

	return stats
}

// IsAsyncZKProofGenerationEnabled 检查是否启用异步ZK证明生成
func (m *Manager) IsAsyncZKProofGenerationEnabled() bool {
	return m.asyncZKProofEnabled
}

// handleZKProofCallback 处理ZK证明生成完成回调
//
// 🎯 **回调处理**：
// - 更新任务状态
// - 记录日志（包含StateOutput相关信息）
// - 更新任务存储中的证明结果
//
// ⚠️ **设计说明**：
// - StateOutput 在执行完成时已经构建并返回给调用方
// - 异步 ZK 证明生成主要用于性能优化，实际的 StateOutput 在返回时包含 pending 状态的证明
// - 如果需要在交易提交前等待证明完成，调用方应该通过 zk_proof_task_id 查询任务状态
// - 本回调主要用于：
//  1. 更新任务状态和证明结果（供查询使用）
//  2. 记录日志和监控信息
//  3. 通知相关组件（如果需要）
func (m *Manager) handleZKProofCallback(task *zkproof.ZKProofTask, proof *transaction.ZKStateProof, err error) {
	if err != nil {
		m.logger.Errorf("ZK证明生成失败: taskID=%s, executionID=%s, error=%v", task.TaskID, task.ExecutionID, err)

		// 更新任务状态为失败
		m.zkProofTaskMutex.Lock()
		if storedTask, exists := m.zkProofTaskStore[task.TaskID]; exists {
			storedTask.MarkFailed(err)
		}
		m.zkProofTaskMutex.Unlock()
	} else {
		m.logger.Infof("✅ ZK证明生成完成: taskID=%s, executionID=%s, circuit=%s, proofSize=%d字节",
			task.TaskID, task.ExecutionID, proof.CircuitId, len(proof.Proof))

		// 更新任务状态和证明结果
		m.zkProofTaskMutex.Lock()
		if storedTask, exists := m.zkProofTaskStore[task.TaskID]; exists {
			storedTask.MarkCompleted(proof)
			// 记录StateOutput相关信息（用于日志和监控）
			if storedTask.ExecutionID != "" {
				m.logger.Infof("📋 StateOutput关联信息: executionID=%s, stateID可通过executionID查询, zkProof已更新到任务存储",
					storedTask.ExecutionID)
			}
		}
		m.zkProofTaskMutex.Unlock()

		// 注意：StateOutput 在执行完成时已经构建并返回给调用方
		// 如果需要在交易提交前使用完整的 ZK 证明，调用方应该：
		// 1. 通过 GetZKProofTaskStatus(taskID) 查询任务状态
		// 2. 等待任务完成（通过轮询或回调）
		// 3. 使用任务中的 ProofResult 更新 StateOutput（如果需要）
		//
		// 当前设计：异步证明生成主要用于性能优化和监控，实际的 StateOutput 在返回时
		// 包含 pending 状态的证明，调用方可以根据需要决定是否等待证明完成
	}
}

// submitZKProofTask 提交ZK证明生成任务（异步）
//
// 📋 **参数**：
//   - ctx: 上下文
//   - executionID: 执行上下文ID
//   - executionResultHash: 执行结果哈希
//   - executionTrace: 执行轨迹（coordinator包中的ExecutionTrace类型）
//   - circuitID: 电路ID
//   - priority: 任务优先级（默认0）
//
// 📋 **返回值**：
//   - string: 任务ID
//   - error: 提交错误
func (m *Manager) submitZKProofTask(
	ctx context.Context,
	executionID string,
	executionResultHash []byte,
	executionTrace *ExecutionTrace,
	circuitID string,
	priority int,
) (string, error) {
	if !m.asyncZKProofEnabled || m.zkProofTaskQueue == nil {
		return "", fmt.Errorf("异步ZK证明生成未启用")
	}

	// 构建ZK证明输入
	zkInput, err := m.buildZKProofInput(ctx, executionResultHash, executionTrace, circuitID)
	if err != nil {
		return "", fmt.Errorf("构建ZK证明输入失败: %w", err)
	}

	// 转换ExecutionTrace到interfaces.HostFunctionCall列表
	hostFunctionCalls := make([]*ispcInterfaces.HostFunctionCall, 0, len(executionTrace.HostFunctionCalls))
	for _, call := range executionTrace.HostFunctionCalls {
		// 转换Parameters和Result到map[string]interface{}
		var params map[string]interface{}
		if len(call.Parameters) > 0 {
			// Parameters是[]any类型，转换为map
			params = map[string]interface{}{
				"parameters": call.Parameters,
			}
		}

		var result map[string]interface{}
		if call.Result != nil {
			// Result是any类型，转换为map
			result = map[string]interface{}{
				"result": call.Result,
			}
		}

		hostFunctionCalls = append(hostFunctionCalls, &ispcInterfaces.HostFunctionCall{
			FunctionName: call.FunctionName,
			Parameters:   params,
			Result:       result,
			Timestamp:    call.Timestamp.UnixNano(),
		})
	}

	// 生成任务ID（使用确定性方式：executionID + 序列号）
	// P1: 使用确定性方式生成taskID，避免使用time.Now()
	// 使用executionID和任务存储中的任务数量生成确定性ID
	m.zkProofTaskMutex.RLock()
	taskSequence := len(m.zkProofTaskStore)
	m.zkProofTaskMutex.RUnlock()
	taskID := fmt.Sprintf("zkproof_%s_%d", executionID, taskSequence)

	// 创建任务
	task := zkproof.NewZKProofTask(
		taskID,
		executionID,
		zkInput,
		executionResultHash,
		hostFunctionCalls,
		priority,
		5*time.Minute, // 默认5分钟超时
	)

	// 入队
	if err := m.zkProofTaskQueue.Enqueue(task); err != nil {
		return "", fmt.Errorf("任务入队失败: %w", err)
	}

	// 存储任务
	m.zkProofTaskMutex.Lock()
	m.zkProofTaskStore[taskID] = task
	m.zkProofTaskMutex.Unlock()

	if m.logger != nil {
		m.logger.Debugf("ZK证明任务已提交: taskID=%s, executionID=%s, priority=%d", taskID, executionID, priority)
	}

	return taskID, nil
}

// buildZKProofInput 构建ZK证明输入
//
// 📋 **参数**：
//   - ctx: 上下文
//   - executionResultHash: 执行结果哈希
//   - executionTrace: 执行轨迹
//   - circuitID: 电路ID
//
// 📋 **返回值**：
//   - *ispcInterfaces.ZKProofInput: ZK证明输入
//   - error: 构建错误
func (m *Manager) buildZKProofInput(
	ctx context.Context,
	executionResultHash []byte,
	executionTrace *ExecutionTrace,
	circuitID string,
) (*ispcInterfaces.ZKProofInput, error) {
	// 构建公开输入
	publicInputs := [][]byte{
		executionResultHash,
	}

	// 从上下文中提取合约信息
	if contractAddr := ctx.Value(ContextKeyContract); contractAddr != nil {
		if addr, ok := contractAddr.(string); ok {
			publicInputs = append(publicInputs, []byte(addr))
		}
	}

	if functionName := ctx.Value(ContextKeyFunction); functionName != nil {
		if name, ok := functionName.(string); ok {
			publicInputs = append(publicInputs, []byte(name))
		}
	}

	// 计算execution_trace哈希（确定性编码）
	traceBytes, err := m.serializeExecutionTraceForZK(executionTrace)
	if err != nil {
		return nil, fmt.Errorf("序列化execution_trace失败: %w", err)
	}
	// P1: 使用公共接口 HashManager 而不是直接使用 crypto/sha256
	if m.hashManager == nil {
		return nil, fmt.Errorf("hashManager未初始化，无法计算execution_trace哈希")
	}
	traceHash := m.hashManager.SHA256(traceBytes)

	// 计算state_diff哈希（确定性编码）
	stateBytes, err := m.serializeStateChangesForZK(executionTrace.StateChanges)
	if err != nil {
		return nil, fmt.Errorf("序列化state_diff失败: %w", err)
	}
	// P1: 使用公共接口 HashManager 而不是直接使用 crypto/sha256
	stateDiffHash := m.hashManager.SHA256(stateBytes)

	zkInput := &ispcInterfaces.ZKProofInput{
		PublicInputs: publicInputs,
		PrivateInputs: map[string]any{
			"execution_trace": traceHash,     // 32字节SHA256哈希（来自HashManager）
			"state_diff":      stateDiffHash, // 32字节SHA256哈希（来自HashManager）
		},
		CircuitID:      circuitID, // 基础名（不含.v1后缀）
		CircuitVersion: 1,         // 版本号独立指定
	}

	return zkInput, nil
}

// createPendingZKProof 创建pending状态的ZK证明（占位符）
//
// 📋 **参数**：
//   - input: ZK证明输入
//
// 📋 **返回值**：
//   - *transaction.ZKStateProof: pending状态的ZK证明
func (m *Manager) createPendingZKProof(input *ispcInterfaces.ZKProofInput) *transaction.ZKStateProof {
	// 创建pending状态的ZK证明（占位符）
	// 注意：这是一个临时占位符，实际的证明将在异步生成完成后更新
	// 从zkproofManager获取默认配置（如果可用）
	var defaultScheme, defaultCurve string
	if m.zkproofManager != nil {
		defaultScheme = m.zkproofManager.GetDefaultProvingScheme()
		defaultCurve = m.zkproofManager.GetDefaultCurve()
	} else {
		// 回退到硬编码默认值（向后兼容）
		defaultScheme = "groth16"
		defaultCurve = "bn254"
	}

	return &transaction.ZKStateProof{
		// ⚠️ 重要：pending 状态的 proof 使用显式占位符（"pending"），便于上层/序列化/调试识别。
		// 真实 proof 生成完成前，不允许进入交易验证/入池/进块流程：
		// - tx/verifier/plugins/condition/exec_resource_invariants.go 会拒绝
		//   (constraint_count==0 或 proof_len==0) 的 pending 证明。
		Proof:               []byte("pending"),
		PublicInputs:        input.PublicInputs,
		ProvingScheme:       defaultScheme,
		Curve:               defaultCurve,
		VerificationKeyHash: []byte{}, // 空，待生成
		CircuitId:           input.CircuitID,
		CircuitVersion:      input.CircuitVersion,
		ConstraintCount:     0, // 0表示pending
	}
}
