package zkproof

import (
	"context"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/stretchr/testify/require"
	"github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// ============================================================================
// prover.go 测试（补充）
// ============================================================================

// TestProver_computeCircuitCommitment 测试计算电路承诺
func TestProver_computeCircuitCommitment(t *testing.T) {
	prover := createTestProver(t)

	circuit := &simpleTestCircuit{}
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)

	commitment, err := prover.computeCircuitCommitment(compiledCircuit)
	require.NoError(t, err)
	require.NotNil(t, commitment)
	require.Equal(t, 32, len(commitment)) // SHA-256 哈希长度
}

// TestProver_serializeProof 测试序列化证明
func TestProver_serializeProof(t *testing.T) {
	prover := createTestProver(t)

	// 创建测试电路和证明
	circuit := &simpleTestCircuit{}
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)

	pk, _, err := groth16.Setup(compiledCircuit)
	require.NoError(t, err)

	witness, err := frontend.NewWitness(&simpleTestCircuit{X: 42, Y: 42}, ecc.BN254.ScalarField())
	require.NoError(t, err)

	proof, err := groth16.Prove(compiledCircuit, pk, witness)
	require.NoError(t, err)

	// 序列化证明
	proofBytes, err := prover.serializeProof(proof)
	require.NoError(t, err)
	require.NotEmpty(t, proofBytes)
}

// TestProver_computeVerifyingKeyHash 测试计算验证密钥哈希
func TestProver_computeVerifyingKeyHash(t *testing.T) {
	prover := createTestProver(t)

	// 创建测试电路和验证密钥
	circuit := &simpleTestCircuit{}
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	require.NoError(t, err)

	_, vk, err := groth16.Setup(compiledCircuit)
	require.NoError(t, err)

	// 计算验证密钥哈希
	vkHash, err := prover.computeVerifyingKeyHash(vk)
	require.NoError(t, err)
	require.NotNil(t, vkHash)
	require.Equal(t, 32, len(vkHash)) // SHA-256 哈希长度
}

// TestProver_GenerateStateProof 测试生成状态证明
func TestProver_GenerateStateProof(t *testing.T) {
	prover := createTestProver(t)

	ctx := context.Background()
	// 提供有效的私有输入（合约执行电路需要 execution_trace 和 state_diff）
	input := &interfaces.ZKProofInput{
		CircuitID:      "contract_execution",
		CircuitVersion: 1,
		PublicInputs:   [][]byte{[]byte("test_hash")},
		PrivateInputs: map[string]interface{}{
			"execution_trace": []byte("trace_data"),
			"state_diff":      []byte("state_diff_data"),
		},
	}

	stateProof, err := prover.GenerateStateProof(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, stateProof)
	require.Equal(t, "contract_execution", stateProof.CircuitId)
	require.Equal(t, uint32(1), stateProof.CircuitVersion)
	require.NotEmpty(t, stateProof.Proof)
	require.NotEmpty(t, stateProof.PublicInputs)
	require.NotEmpty(t, stateProof.VerificationKeyHash)
	require.NotNil(t, stateProof.CustomAttributes)
	require.Equal(t, "contract_execution", stateProof.CustomAttributes["circuit_id"])
}

// TestProver_GenerateStateProof_InvalidCircuit 测试无效电路的生成状态证明
func TestProver_GenerateStateProof_InvalidCircuit(t *testing.T) {
	prover := createTestProver(t)

	ctx := context.Background()
	input := &interfaces.ZKProofInput{
		CircuitID:      "nonexistent_circuit",
		CircuitVersion: 1,
		PublicInputs:   [][]byte{[]byte("test")},
	}

	_, err := prover.GenerateStateProof(ctx, input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "获取可信设置失败")
}

// TestProver_boolToByte 测试布尔值转字节
func TestProver_boolToByte(t *testing.T) {
	require.Equal(t, byte(1), boolToByte(true))
	require.Equal(t, byte(0), boolToByte(false))
}

// TestProver_computePreStateHash 测试计算前状态哈希
func TestProver_computePreStateHash(t *testing.T) {
	prover := createTestProver(t)

	trace := &ExecutionTraceData{
		ExecutionID: "test_execution",
		StartTime:   1234567890,
	}

	hash := prover.computePreStateHash(trace)
	require.NotNil(t, hash)
	require.Equal(t, 32, len(hash)) // SHA-256 哈希长度
}

// TestProver_computePostStateHash 测试计算后状态哈希
func TestProver_computePostStateHash(t *testing.T) {
	prover := createTestProver(t)

	trace := &ExecutionTraceData{
		ExecutionID: "test_execution",
		EndTime:     1234567890,
		StateChanges: []StateChangeData{
			{Type: "utxo_create", Key: "key1"},
			{Type: "utxo_spend", Key: "key2"},
		},
	}

	hash := prover.computePostStateHash(trace)
	require.NotNil(t, hash)
	require.Equal(t, 32, len(hash)) // SHA-256 哈希长度
}

// TestProver_computeStateTransitionHash 测试计算状态变更哈希
func TestProver_computeStateTransitionHash(t *testing.T) {
	prover := createTestProver(t)

	// 测试空状态变更
	hash := prover.computeStateTransitionHash([]StateChangeData{})
	require.NotNil(t, hash)
	require.Equal(t, 32, len(hash))

	// 测试有状态变更
	changes := []StateChangeData{
		{Type: "utxo_create", Key: "key1", Timestamp: 1234567890},
		{Type: "utxo_spend", Key: "key2", Timestamp: 1234567891},
	}
	hash = prover.computeStateTransitionHash(changes)
	require.NotNil(t, hash)
	require.Equal(t, 32, len(hash))
}

// TestProver_computeInputDataHash 测试计算输入数据哈希
func TestProver_computeInputDataHash(t *testing.T) {
	prover := createTestProver(t)

	trace := &ExecutionTraceData{
		ExecutionID: "test_execution",
		StartTime:   1234567890,
	}

	hash := prover.computeInputDataHash(trace)
	require.NotNil(t, hash)
	require.Equal(t, 32, len(hash)) // SHA-256 哈希长度
}

// TestProver_computeComputationProcessHash 测试计算计算过程哈希
func TestProver_computeComputationProcessHash(t *testing.T) {
	prover := createTestProver(t)

	// 测试空调用列表
	hash := prover.computeComputationProcessHash([]HostFunctionCallData{})
	require.NotNil(t, hash)
	require.Equal(t, 32, len(hash))

	// 测试有调用列表
	calls := []HostFunctionCallData{
		{FunctionName: "func1", ParamCount: 2, HasResult: true, Success: true, Timestamp: 1234567890},
		{FunctionName: "func2", ParamCount: 1, HasResult: false, Success: true, Timestamp: 1234567891},
	}
	hash = prover.computeComputationProcessHash(calls)
	require.NotNil(t, hash)
	require.Equal(t, 32, len(hash))
}

// TestProver_computeOutputResultHash 测试计算输出结果哈希
func TestProver_computeOutputResultHash(t *testing.T) {
	prover := createTestProver(t)

	trace := &ExecutionTraceData{
		ExecutionID: "test_execution",
		EndTime:     1234567890,
		HostFunctionCalls: []HostFunctionCallData{
			{FunctionName: "func1"},
			{FunctionName: "func2"},
		},
	}

	hash := prover.computeOutputResultHash(trace)
	require.NotNil(t, hash)
	require.Equal(t, 32, len(hash)) // SHA-256 哈希长度
}

// TestProver_buildGenericWitness 测试构建通用witness
func TestProver_buildGenericWitness(t *testing.T) {
	prover := createTestProver(t)

	input := &interfaces.ZKProofInput{
		CircuitID:    "generic_circuit",
		PublicInputs: [][]byte{[]byte("input1"), []byte("input2")},
		PrivateInputs: map[string]interface{}{
			"data": "test",
		},
	}

	witness, err := prover.buildGenericWitness(input)
	require.NoError(t, err)
	require.NotNil(t, witness)
}

// TestProver_buildGenericWitness_WithExecutionTrace 测试使用ExecutionTraceData构建通用witness
func TestProver_buildGenericWitness_WithExecutionTrace(t *testing.T) {
	prover := createTestProver(t)

	input := &interfaces.ZKProofInput{
		CircuitID:    "generic_circuit",
		PublicInputs: [][]byte{[]byte("input1")},
		PrivateInputs: &ExecutionTraceData{
			ExecutionID: "test_execution",
			StartTime:   1234567890,
			EndTime:     1234567900,
			HostFunctionCalls: []HostFunctionCallData{
				{FunctionName: "func1"},
			},
		},
	}

	witness, err := prover.buildGenericWitness(input)
	require.NoError(t, err)
	require.NotNil(t, witness)
}

// TestProver_buildExecutionWitness 测试构建执行witness
func TestProver_buildExecutionWitness(t *testing.T) {
	prover := createTestProver(t)

	input := &interfaces.ZKProofInput{
		CircuitID:    "execution_proof_circuit",
		PublicInputs: [][]byte{[]byte("result_hash")},
		PrivateInputs: &ExecutionTraceData{
			ExecutionID: "test_execution",
			StartTime:   1234567890,
			EndTime:     1234567900,
			Duration:    10000000000,
			HostFunctionCalls: []HostFunctionCallData{
				{FunctionName: "func1", Success: true},
			},
			StateChanges: []StateChangeData{
				{Type: "utxo_create", Key: "key1"},
			},
		},
	}

	witness, err := prover.buildExecutionWitness(input)
	require.NoError(t, err)
	require.NotNil(t, witness)
}

// TestProver_buildStateTransitionWitness 测试构建状态转换witness
func TestProver_buildStateTransitionWitness(t *testing.T) {
	prover := createTestProver(t)

	input := &interfaces.ZKProofInput{
		CircuitID:    "state_transition_circuit",
		PublicInputs: [][]byte{[]byte("pre_state"), []byte("post_state")},
		PrivateInputs: &ExecutionTraceData{
			ExecutionID: "test_execution",
			StartTime:   1234567890,
			EndTime:     1234567900,
			StateChanges: []StateChangeData{
				{Type: "utxo_create", Key: "key1"},
			},
		},
	}

	witness, err := prover.buildStateTransitionWitness(input)
	require.NoError(t, err)
	require.NotNil(t, witness)
}

// TestProver_buildStateTransitionWitness_InsufficientInputs 测试构建状态转换witness（输入不足）
func TestProver_buildStateTransitionWitness_InsufficientInputs(t *testing.T) {
	prover := createTestProver(t)

	input := &interfaces.ZKProofInput{
		CircuitID:    "state_transition_circuit",
		PublicInputs: [][]byte{[]byte("pre_state")}, // 只有1个输入，需要2个
		PrivateInputs: &ExecutionTraceData{
			ExecutionID: "test_execution",
		},
	}

	_, err := prover.buildStateTransitionWitness(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "至少2个公开输入")
}

// TestProver_buildComputationWitness 测试构建计算witness
func TestProver_buildComputationWitness(t *testing.T) {
	prover := createTestProver(t)

	input := &interfaces.ZKProofInput{
		CircuitID:    "computation_circuit",
		PublicInputs: [][]byte{[]byte("output_hash")},
		PrivateInputs: &ExecutionTraceData{
			ExecutionID: "test_execution",
			StartTime:   1234567890,
			EndTime:     1234567900,
			HostFunctionCalls: []HostFunctionCallData{
				{FunctionName: "func1", Success: true},
			},
		},
	}

	witness, err := prover.buildComputationWitness(input)
	require.NoError(t, err)
	require.NotNil(t, witness)
}

// TestProver_buildComputationWitness_InsufficientInputs 测试构建计算witness（输入不足）
func TestProver_buildComputationWitness_InsufficientInputs(t *testing.T) {
	prover := createTestProver(t)

	input := &interfaces.ZKProofInput{
		CircuitID:    "computation_circuit",
		PublicInputs: [][]byte{}, // 没有输入
		PrivateInputs: &ExecutionTraceData{
			ExecutionID: "test_execution",
		},
	}

	_, err := prover.buildComputationWitness(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "至少1个公开输入")
}

// TestProver_buildAIModelInferenceProofWitness 测试构建AI模型推理proof witness
func TestProver_buildAIModelInferenceProofWitness(t *testing.T) {
	prover := createTestProver(t)

	input := &interfaces.ZKProofInput{
		CircuitID:    "aimodel_inference",
		PublicInputs: [][]byte{[]byte("inference_result_hash")},
		PrivateInputs: map[string]interface{}{
			"model_weights": []byte("weights_data"),
			"input_data":    []byte("input_data"),
		},
	}

	witness, err := prover.buildAIModelInferenceProofWitness(input)
	require.NoError(t, err)
	require.NotNil(t, witness)
}

// TestProver_buildAIModelInferenceProofWitness_MissingInputs 测试构建AI模型推理proof witness（缺少输入）
func TestProver_buildAIModelInferenceProofWitness_MissingInputs(t *testing.T) {
	prover := createTestProver(t)

	input := &interfaces.ZKProofInput{
		CircuitID:    "aimodel_inference",
		PublicInputs: [][]byte{},
	}

	_, err := prover.buildAIModelInferenceProofWitness(input)
	require.Error(t, err)
	require.Contains(t, err.Error(), "缺少公开输入")
}

// TestProver_extractExecutionTraceFromPrivateInputs 测试从私有输入中提取执行轨迹
func TestProver_extractExecutionTraceFromPrivateInputs(t *testing.T) {
	prover := createTestProver(t)

	// 测试直接传入 ExecutionTraceData
	trace := &ExecutionTraceData{
		ExecutionID: "test_execution",
		StartTime:   1234567890,
	}
	result := prover.extractExecutionTraceFromPrivateInputs(trace)
	require.NotNil(t, result)
	require.Equal(t, trace, result)

	// 测试从 map 提取
	mapData := map[string]interface{}{
		"execution_id": "test_execution",
		"start_time":   int64(1234567890),
		"end_time":     int64(1234567900),
		"duration":     int64(10000000000),
		"host_function_calls": []interface{}{
			map[string]interface{}{
				"function_name": "func1",
				"param_count":   float64(2),
				"has_result":    true,
				"success":       true,
				"timestamp":     float64(1234567890),
				"duration":      float64(1000),
			},
		},
		"state_changes": []interface{}{
			map[string]interface{}{
				"type":      "utxo_create",
				"key":       "key1",
				"has_old":   false,
				"has_new":   true,
				"timestamp": float64(1234567890),
			},
		},
	}
	result = prover.extractExecutionTraceFromPrivateInputs(mapData)
	require.NotNil(t, result)
	require.Equal(t, "test_execution", result.ExecutionID)

	// 测试未知类型
	result = prover.extractExecutionTraceFromPrivateInputs("unknown_type")
	require.Nil(t, result)
}

// TestProver_buildExecutionTraceFromMap 测试从map构建执行轨迹
func TestProver_buildExecutionTraceFromMap(t *testing.T) {
	prover := createTestProver(t)

	mapData := map[string]interface{}{
		"execution_id": "test_execution",
		"start_time":   int64(1234567890),
		"end_time":     int64(1234567900),
		"duration":     int64(10000000000),
		"host_function_calls": []interface{}{
			map[string]interface{}{
				"function_name": "func1",
				"param_count":   float64(2),
				"has_result":    true,
				"success":       true,
				"timestamp":     float64(1234567890),
				"duration":      float64(1000),
			},
		},
		"state_changes": []interface{}{
			map[string]interface{}{
				"type":      "utxo_create",
				"key":       "key1",
				"has_old":   false,
				"has_new":   true,
				"timestamp": float64(1234567890),
			},
		},
	}

	trace := prover.buildExecutionTraceFromMap(mapData)
	require.NotNil(t, trace)
	require.Equal(t, "test_execution", trace.ExecutionID)
	require.Equal(t, int64(1234567890), trace.StartTime)
	require.Equal(t, int64(1234567900), trace.EndTime)
	require.Equal(t, 1, len(trace.HostFunctionCalls))
	require.Equal(t, 1, len(trace.StateChanges))
}

// TestProver_parseExecutionTraceFromJSON 测试从JSON解析执行轨迹
func TestProver_parseExecutionTraceFromJSON(t *testing.T) {
	prover := createTestProver(t)

	jsonData := []byte(`{
		"execution_id": "test_execution",
		"start_time": 1234567890,
		"end_time": 1234567900,
		"duration": 10000000000,
		"host_function_calls": [
			{
				"function_name": "func1",
				"param_count": 2,
				"has_result": true,
				"success": true,
				"timestamp": 1234567890,
				"duration": 1000
			}
		],
		"state_changes": [
			{
				"type": "utxo_create",
				"key": "key1",
				"has_old": false,
				"has_new": true,
				"timestamp": 1234567890
			}
		]
	}`)

	trace := prover.parseExecutionTraceFromJSON(jsonData)
	require.NotNil(t, trace)
	require.Equal(t, "test_execution", trace.ExecutionID)
	require.Equal(t, 1, len(trace.HostFunctionCalls))
	require.Equal(t, 1, len(trace.StateChanges))

	// 测试无效JSON
	invalidJSON := []byte("invalid json")
	trace = prover.parseExecutionTraceFromJSON(invalidJSON)
	require.Nil(t, trace)
}

// TestProver_encodeExecutionTraceForCircuit 测试编码执行轨迹为电路友好格式
func TestProver_encodeExecutionTraceForCircuit(t *testing.T) {
	prover := createTestProver(t)

	trace := &ExecutionTraceData{
		ExecutionID: "test_execution",
		StartTime:   1234567890,
		EndTime:     1234567900,
		HostFunctionCalls: []HostFunctionCallData{
			{FunctionName: "func1", Success: true},
		},
		StateChanges: []StateChangeData{
			{Type: "utxo_create", Key: "key1"},
		},
	}

	witnessData, err := prover.encodeExecutionTraceForCircuit(trace)
	require.NoError(t, err)
	require.NotNil(t, witnessData)
	require.Equal(t, uint64(1234567890), witnessData.StartTime)
	require.Equal(t, uint64(1234567900), witnessData.EndTime)
	require.Equal(t, uint32(1), witnessData.HostCallCount)
	require.Equal(t, uint32(1), witnessData.StateChangeCount)
	require.NotNil(t, witnessData.ExecutionID)
	require.NotNil(t, witnessData.HostCallsHash)
	require.NotNil(t, witnessData.StateChangesHash)
	require.NotNil(t, witnessData.ExecutionHash)
}
