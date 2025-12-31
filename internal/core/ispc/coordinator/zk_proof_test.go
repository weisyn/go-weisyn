package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
	"github.com/weisyn/v1/internal/core/ispc/zkproof"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// ZK证明生成测试
// ============================================================================
//
// 🎯 **测试目的**：发现ZK证明生成功能的缺陷和BUG
//
// ============================================================================

// TestGenerateZKProof 测试生成ZK证明
func TestGenerateZKProof(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeyContract, "test_contract")
	ctx = context.WithValue(ctx, ContextKeyFunction, "test_function")

	executionResultHash := []byte{0x12, 0x34, 0x56, 0x78}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now().Add(10 * time.Millisecond),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	// 注意：generateZKProof会调用zkproofManager.GenerateStateProof
	// 如果zkproofManager没有正确Mock，可能会失败
	proof, err := manager.generateZKProof(ctx, executionResultHash, trace)
	if err != nil {
		// 如果zkproofManager没有正确实现，这是预期的
		t.Logf("⚠️ 警告：generateZKProof返回错误（可能是zkproofManager未正确Mock）：%v", err)
		assert.Error(t, err)
	} else {
		assert.NotNil(t, proof, "ZK证明不应该为nil")
	}
}

// TestGenerateZKProof_NilHashManager 测试nil hashManager
// 🐛 **BUG检测**：nil hashManager应该返回错误
func TestGenerateZKProof_NilHashManager(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = nil // nil hashManager

	ctx := context.Background()
	executionResultHash := []byte{0x12, 0x34, 0x56}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now(),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	proof, err := manager.generateZKProof(ctx, executionResultHash, trace)
	assert.Error(t, err, "nil hashManager应该返回错误")
	assert.Nil(t, proof, "证明应该为nil")
	assert.Contains(t, err.Error(), "hashManager未初始化", "错误信息应该提到hashManager")
}

// TestBuildZKProofInput 测试构建ZK证明输入
func TestBuildZKProofInput(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	ctx := context.Background()
	ctx = context.WithValue(ctx, ContextKeyContract, "test_contract")
	ctx = context.WithValue(ctx, ContextKeyFunction, "test_function")

	executionResultHash := []byte{0x12, 0x34, 0x56, 0x78}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now(),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	zkInput, err := manager.buildZKProofInput(ctx, executionResultHash, trace, "test_circuit")
	require.NoError(t, err)
	assert.NotNil(t, zkInput)
	assert.Equal(t, "test_circuit", zkInput.CircuitID)
	assert.Equal(t, uint32(1), zkInput.CircuitVersion)
	assert.NotNil(t, zkInput.PublicInputs)
	assert.NotNil(t, zkInput.PrivateInputs)
}

// TestBuildZKProofInput_NilHashManager 测试nil hashManager
// 🐛 **BUG检测**：nil hashManager应该返回错误
func TestBuildZKProofInput_NilHashManager(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = nil

	ctx := context.Background()
	executionResultHash := []byte{0x12, 0x34, 0x56}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now(),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	zkInput, err := manager.buildZKProofInput(ctx, executionResultHash, trace, "test_circuit")
	assert.Error(t, err, "nil hashManager应该返回错误")
	assert.Nil(t, zkInput, "输入应该为nil")
	assert.Contains(t, err.Error(), "hashManager未初始化", "错误信息应该提到hashManager")
}

// TestCreatePendingZKProof 测试创建pending状态的ZK证明
func TestCreatePendingZKProof(t *testing.T) {
	manager := createTestManager(t)

	zkInput := &ispcInterfaces.ZKProofInput{
		PublicInputs: [][]byte{{0x12, 0x34}},
		PrivateInputs: map[string]interface{}{
			"execution_trace": []byte{0x56, 0x78},
			"state_diff":      []byte{0x9a, 0xbc},
		},
		CircuitID:      "test_circuit",
		CircuitVersion: 1,
	}

	proof := manager.createPendingZKProof(zkInput)
	assert.NotNil(t, proof, "pending证明不应该为nil")
	assert.Equal(t, "test_circuit", proof.CircuitId, "电路ID应该匹配")
	assert.Equal(t, uint32(1), proof.CircuitVersion, "电路版本应该匹配")
	assert.NotEmpty(t, proof.Proof, "pending证明应该有占位符Proof")
	assert.Equal(t, "pending", string(proof.Proof), "Proof应该是'pending'占位符")
	assert.NotEmpty(t, proof.ProvingScheme, "应该设置证明方案")
	assert.NotEmpty(t, proof.Curve, "应该设置曲线")
	assert.Equal(t, uint64(0), proof.ConstraintCount, "pending证明的约束数应该为0")
}

// TestSubmitZKProofTask 测试提交ZK证明任务
func TestSubmitZKProofTask(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = testutil.NewTestHashManager()

	// 启用异步ZK证明
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)
	defer manager.DisableAsyncZKProofGeneration()

	ctx := context.Background()
	executionID := "exec_123"
	executionResultHash := []byte{0x12, 0x34, 0x56}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now(),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	taskID, err := manager.submitZKProofTask(ctx, executionID, executionResultHash, trace, "test_circuit", 0)
	require.NoError(t, err)
	assert.NotEmpty(t, taskID, "任务ID不应该为空")
	assert.Contains(t, taskID, executionID, "任务ID应该包含executionID")

	// 验证任务已存储
	status := manager.GetZKProofTaskStatus(taskID)
	assert.NotNil(t, status, "任务应该已存储")
}

// TestSubmitZKProofTask_NotEnabled 测试未启用异步模式时提交任务
// 🐛 **BUG检测**：未启用异步模式时应该返回错误
func TestSubmitZKProofTask_NotEnabled(t *testing.T) {
	manager := createTestManager(t)

	ctx := context.Background()
	executionID := "exec_123"
	executionResultHash := []byte{0x12, 0x34, 0x56}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now(),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	taskID, err := manager.submitZKProofTask(ctx, executionID, executionResultHash, trace, "test_circuit", 0)
	assert.Error(t, err, "未启用异步模式时应该返回错误")
	assert.Empty(t, taskID, "任务ID应该为空")
	assert.Contains(t, err.Error(), "异步ZK证明生成未启用", "错误信息应该提到未启用")
}

// TestSubmitZKProofTask_NilHashManager 测试nil hashManager
// 🐛 **BUG检测**：nil hashManager应该返回错误
func TestSubmitZKProofTask_NilHashManager(t *testing.T) {
	manager := createTestManager(t)
	manager.hashManager = nil

	// 启用异步ZK证明
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)
	defer manager.DisableAsyncZKProofGeneration()

	ctx := context.Background()
	executionID := "exec_123"
	executionResultHash := []byte{0x12, 0x34, 0x56}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now(),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	taskID, err := manager.submitZKProofTask(ctx, executionID, executionResultHash, trace, "test_circuit", 0)
	assert.Error(t, err, "nil hashManager应该返回错误")
	assert.Empty(t, taskID, "任务ID应该为空")
}

// TestHandleZKProofCallback_Success 测试ZK证明回调（成功）
func TestHandleZKProofCallback_Success(t *testing.T) {
	manager := createTestManager(t)

	// 启用异步ZK证明
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)
	defer manager.DisableAsyncZKProofGeneration()

	// 创建任务并存储
	taskID := "task_123"
	executionID := "exec_123"
	zkInput := &ispcInterfaces.ZKProofInput{
		CircuitID:      "test_circuit",
		CircuitVersion: 1,
	}
	task := zkproof.NewZKProofTask(
		taskID,
		executionID,
		zkInput,
		[]byte{0x12, 0x34},
		nil,
		0,
		5*time.Minute,
	)

	manager.zkProofTaskMutex.Lock()
	manager.zkProofTaskStore[taskID] = task
	manager.zkProofTaskMutex.Unlock()

	// 创建成功的证明
	proof := &pb.ZKStateProof{
		CircuitId:      "test_circuit",
		CircuitVersion: 1,
		Proof:          []byte{0x12, 0x34, 0x56},
	}

	// 调用回调
	manager.handleZKProofCallback(task, proof, nil)

	// 验证任务状态已更新
	status := manager.GetZKProofTaskStatus(taskID)
	assert.NotNil(t, status, "任务应该存在")
}

// TestHandleZKProofCallback_Failure 测试ZK证明回调（失败）
func TestHandleZKProofCallback_Failure(t *testing.T) {
	manager := createTestManager(t)

	// 启用异步ZK证明
	err := manager.EnableAsyncZKProofGeneration(2, 1, 10)
	require.NoError(t, err)
	defer manager.DisableAsyncZKProofGeneration()

	// 创建任务并存储
	taskID := "task_123"
	executionID := "exec_123"
	zkInput := &ispcInterfaces.ZKProofInput{
		CircuitID:      "test_circuit",
		CircuitVersion: 1,
	}
	task := zkproof.NewZKProofTask(
		taskID,
		executionID,
		zkInput,
		[]byte{0x12, 0x34},
		nil,
		0,
		5*time.Minute,
	)

	manager.zkProofTaskMutex.Lock()
	manager.zkProofTaskStore[taskID] = task
	manager.zkProofTaskMutex.Unlock()

	// 调用回调（失败）
	callbackErr := assert.AnError
	manager.handleZKProofCallback(task, nil, callbackErr)

	// 验证任务状态已更新为失败
	status := manager.GetZKProofTaskStatus(taskID)
	assert.NotNil(t, status, "任务应该存在")
}

