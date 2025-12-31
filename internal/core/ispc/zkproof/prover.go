package zkproof

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"time"

	// 内部接口
	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"

	// 基础设施
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// gnark ZK库
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	gnarklogger "github.com/consensys/gnark/logger"

	// zerolog for gnark logger
	"github.com/rs/zerolog"
)

// ExecutionTraceData 执行轨迹数据（用于ZK证明）
type ExecutionTraceData struct {
	ExecutionID       string                 `json:"execution_id"`
	StartTime         int64                  `json:"start_time"` // Unix时间戳
	EndTime           int64                  `json:"end_time"`   // Unix时间戳
	Duration          int64                  `json:"duration"`   // 纳秒为单位
	HostFunctionCalls []HostFunctionCallData `json:"host_function_calls"`
	StateChanges      []StateChangeData      `json:"state_changes"`
	ExecutionEvents   []ExecutionEventData   `json:"execution_events"`
}

// HostFunctionCallData 宿主函数调用数据
type HostFunctionCallData struct {
	FunctionName string `json:"function_name"`
	ParamCount   int    `json:"param_count"` // 参数数量
	HasResult    bool   `json:"has_result"`  // 是否有返回值
	Success      bool   `json:"success"`     // 是否成功
	Timestamp    int64  `json:"timestamp"`   // Unix时间戳
	Duration     int64  `json:"duration"`    // 纳秒为单位
}

// StateChangeData 状态变更数据
type StateChangeData struct {
	Type      string `json:"type"`      // 变更类型（utxo_create, utxo_spend等）
	Key       string `json:"key"`       // 变更键值
	HasOld    bool   `json:"has_old"`   // 是否有旧值
	HasNew    bool   `json:"has_new"`   // 是否有新值
	Timestamp int64  `json:"timestamp"` // Unix时间戳
}

// ExecutionEventData 执行事件数据
type ExecutionEventData struct {
	EventType string `json:"event_type"` // 事件类型
	Timestamp int64  `json:"timestamp"`  // Unix时间戳
}

// CircuitWitnessData 电路见证数据（电路友好格式）
type CircuitWitnessData struct {
	// 公开输入
	ExecutionID      []byte `json:"execution_id"`       // 执行ID（哈希）
	StartTime        uint64 `json:"start_time"`         // 开始时间
	EndTime          uint64 `json:"end_time"`           // 结束时间
	HostCallCount    uint32 `json:"host_call_count"`    // 宿主函数调用次数
	StateChangeCount uint32 `json:"state_change_count"` // 状态变更次数

	// 私有输入（哈希摘要，用于承诺）
	HostCallsHash    []byte `json:"host_calls_hash"`    // 宿主函数调用哈希
	StateChangesHash []byte `json:"state_changes_hash"` // 状态变更哈希
	ExecutionHash    []byte `json:"execution_hash"`     // 整体执行哈希
}

// Prover ZK证明生成器
//
// 🎯 **专门职责**：负责生成各种类型的零知识证明
// 🏗️ **技术栈**：基于gnark库实现Groth16证明方案
type Prover struct {
	logger         log.Logger
	hashManager    crypto.HashManager
	circuitManager *CircuitManager
	config         *ZKProofManagerConfig
}

// NewProver 创建证明生成器
func NewProver(
	logger log.Logger,
	hashManager crypto.HashManager,
	circuitManager *CircuitManager,
	config *ZKProofManagerConfig,
) *Prover {
	return &Prover{
		logger:         logger,
		hashManager:    hashManager,
		circuitManager: circuitManager,
		config:         config,
	}
}

// GenerateProof 生成零知识证明
func (p *Prover) GenerateProof(ctx context.Context, input *interfaces.ZKProofInput) (*interfaces.ZKProofResult, error) {
	startTime := time.Now()
	p.logger.Debugf("开始生成ZK证明: circuitID=%s", input.CircuitID)

	// ⚠️ **禁用gnark库的日志输出**
	// gnark库会输出大量的调试信息（compiling circuit, parsed circuit inputs等）
	// 这些日志会污染我们的日志系统，所以在执行期间禁用
	// gnark使用zerolog，所以我们创建一个丢弃输出的zerolog.Logger
	oldGnarkLogger := gnarklogger.Logger()
	discardLogger := zerolog.New(io.Discard).Level(zerolog.Disabled)
	gnarklogger.Set(discardLogger)
	defer func() {
		gnarklogger.Set(oldGnarkLogger)
	}()

	// witness将在后面根据电路定义构建

	// 编译电路
	compiledCircuit, provingKey, verifyingKey, err := p.circuitManager.GetTrustedSetup(input.CircuitID, input.CircuitVersion)
	if err != nil {
		return nil, fmt.Errorf("获取可信设置失败: %w", err)
	}

	// 构建证明witness
	realWitness, err := p.buildProofWitness(input)
	if err != nil {
		return nil, fmt.Errorf("构建证明witness失败: %w", err)
	}

	// 生成ZK证明
	proof, err := groth16.Prove(compiledCircuit, provingKey, realWitness)
	if err != nil {
		return nil, fmt.Errorf("生成证明失败: %w", err)
	}

	// 序列化证明
	proofBytes, err := p.serializeProof(proof)
	if err != nil {
		return nil, fmt.Errorf("序列化证明失败: %w", err)
	}

	// 计算验证密钥哈希
	vkHash, err := p.computeVerifyingKeyHash(verifyingKey)
	if err != nil {
		return nil, fmt.Errorf("计算验证密钥哈希失败: %w", err)
	}

	generationTime := time.Since(startTime)
	p.logger.Debugf("ZK证明生成完成: 耗时=%v, 大小=%d字节", generationTime, len(proofBytes))

	return &interfaces.ZKProofResult{
		ProofData:        proofBytes,
		VKHash:           vkHash,
		ConstraintCount:  uint64(compiledCircuit.GetNbConstraints()),
		GenerationTimeMs: uint64(generationTime.Milliseconds()),
		ProofSizeBytes:   uint64(len(proofBytes)),
	}, nil
}

// GenerateStateProof 生成状态证明
//
// 🎯 **核心职责**：生成完全符合 transaction.proto ZKStateProof 规范的证明
//
// 📋 **transaction.proto 规范要求**：
// - proof: bytes - 零知识证明数据（序列化的证明对象）
// - public_inputs: repeated bytes - 公开输入数组（验证时需要的公开参数）
// - proving_scheme: string - 证明方案标识符（"groth16" | "plonk"）
// - curve: string - 椭圆曲线标识符（"bn254" | "bls12-381"）
// - verification_key_hash: bytes - 验证密钥哈希（32字节SHA-256）
// - circuit_id: string - 电路标识符（全局唯一）
// - circuit_version: uint32 - 电路版本号
// - circuit_commitment: optional bytes - 电路承诺（用于额外安全保证）
// - constraint_count: uint64 - 电路约束数量
// - proof_generation_time_ms: optional uint64 - 证明生成时间（毫秒）
// - custom_attributes: map<string, string> - 自定义属性（业务层扩展）
func (p *Prover) GenerateStateProof(ctx context.Context, input *interfaces.ZKProofInput) (*transaction.ZKStateProof, error) {
	startTime := time.Now()

	// 生成基础证明
	result, err := p.GenerateProof(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("生成基础证明失败: %w", err)
	}

	// 获取电路以计算电路承诺
	circuit, err := p.circuitManager.GetCircuit(input.CircuitID, input.CircuitVersion)
	if err != nil {
		return nil, fmt.Errorf("获取电路失败: %w", err)
	}

	// 编译电路以计算电路承诺
	compiledCircuit, err := frontend.Compile(ecc.BN254.ScalarField(), r1cs.NewBuilder, circuit)
	if err != nil {
		return nil, fmt.Errorf("编译电路失败: %w", err)
	}

	// 计算电路承诺（用于防止电路替换攻击）
	circuitCommitment, err := p.computeCircuitCommitment(compiledCircuit)
	if err != nil {
		p.logger.Warnf("计算电路承诺失败，继续生成证明: %v", err)
		// 不返回错误，因为 circuit_commitment 是可选的
	}

	// 确保 public_inputs 是正确的格式（repeated bytes）
	// input.PublicInputs 应该已经是 [][]byte 格式，但我们需要验证
	publicInputs := make([][]byte, 0, len(input.PublicInputs))
	for _, pi := range input.PublicInputs {
		if pi != nil {
			publicInputs = append(publicInputs, pi)
		}
	}

	// 计算证明生成时间
	generationTimeMs := uint64(time.Since(startTime).Milliseconds())

	// 构建完全符合 transaction.proto 规范的 ZKStateProof
	stateProof := &transaction.ZKStateProof{
		// ========== 核心证明数据 ==========
		Proof:        result.ProofData, // 零知识证明数据（序列化的证明对象）
		PublicInputs: publicInputs,     // 公开输入数组（repeated bytes）

		// ========== 证明方案和曲线 ==========
		ProvingScheme:       p.config.DefaultProvingScheme, // "groth16" | "plonk"
		Curve:               p.config.DefaultCurve,         // "bn254" | "bls12-381"
		VerificationKeyHash: result.VKHash,                 // 验证密钥哈希（32字节SHA-256）

		// ========== 电路信息 ==========
		CircuitId:      input.CircuitID,      // 电路标识符（全局唯一）
		CircuitVersion: input.CircuitVersion, // 电路版本号

		// ========== 电路承诺（可选但重要）==========
		CircuitCommitment: circuitCommitment, // 电路承诺（用于防止电路替换攻击）

		// ========== 性能和调试信息 ==========
		ConstraintCount:       result.ConstraintCount, // 电路约束数量
		ProofGenerationTimeMs: &generationTimeMs,      // 证明生成时间（毫秒）

		// ========== 业务扩展字段 ==========
		CustomAttributes: make(map[string]string), // 自定义属性（业务层扩展）
	}

	// 添加自定义属性（如果有）
	// 注意：ZKProofInput 目前没有 CustomAttributes 字段
	// 如果需要自定义属性，可以通过其他方式传递（如 context）

	// 添加默认自定义属性
	stateProof.CustomAttributes["circuit_id"] = input.CircuitID
	stateProof.CustomAttributes["circuit_version"] = fmt.Sprintf("%d", input.CircuitVersion)

	p.logger.Debugf("ZKStateProof生成完成: circuit=%s v=%d, proof=%dB, publicInputs=%d, constraints=%d, time=%dms",
		stateProof.CircuitId, stateProof.CircuitVersion, len(stateProof.Proof),
		len(stateProof.PublicInputs), stateProof.ConstraintCount, generationTimeMs)

	return stateProof, nil
}

// computeCircuitCommitment 计算电路承诺
//
// 🎯 **目的**：计算电路的密码学承诺，用于防止电路替换攻击
// 📋 **方法**：序列化编译后的电路，计算SHA-256哈希
func (p *Prover) computeCircuitCommitment(compiledCircuit constraint.ConstraintSystem) ([]byte, error) {
	// 序列化编译后的电路
	var buf bytes.Buffer
	_, err := compiledCircuit.WriteTo(&buf)
	if err != nil {
		return nil, fmt.Errorf("序列化电路失败: %w", err)
	}

	// 使用HashManager计算SHA-256哈希作为承诺
	hash := p.hashManager.SHA256(buf.Bytes())
	return hash, nil
}

// serializeProof 序列化证明
func (p *Prover) serializeProof(proof groth16.Proof) ([]byte, error) {
	// 使用gnark内置的序列化功能
	var buf bytes.Buffer

	// 使用gnark的WriteTo方法序列化证明
	_, err := proof.WriteTo(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize proof: %w", err)
	}

	serializedProof := buf.Bytes()
	p.logger.Debugf("证明序列化成功: %d 字节", len(serializedProof))

	return serializedProof, nil
}

// computeVerifyingKeyHash 计算验证密钥哈希
func (p *Prover) computeVerifyingKeyHash(vk groth16.VerifyingKey) ([]byte, error) {
	// 序列化验证密钥
	var buf bytes.Buffer
	_, err := vk.WriteTo(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize verifying key: %w", err)
	}

	vkBytes := buf.Bytes()

	// 使用哈希管理器计算哈希
	hash := p.hashManager.SHA256(vkBytes)

	p.logger.Debugf("验证密钥哈希计算成功: %x", hash)
	return hash, nil
}

// buildProofWitness 构建ZK证明的witness
//
// 根据输入数据和电路定义构建完整的witness对象，包括私有和公开输入
//
// 🎯 **关键修复**：支持 contract_execution 和 aimodel_inference 电路
func (p *Prover) buildProofWitness(input *interfaces.ZKProofInput) (witness.Witness, error) {
	p.logger.Debugf("开始构建ZK证明witness: circuitID=%s", input.CircuitID)

	// 🎯 **关键修复**：根据电路ID直接构建对应的witness
	// 使用 frontend.NewWitness 将电路结构体转换为 gnark 的 witness 格式
	switch input.CircuitID {
	case "contract_execution":
		// 合约执行电路：构建包含执行结果哈希、执行轨迹、状态变更的witness
		return p.buildContractExecutionProofWitness(input)

	case "aimodel_inference":
		// AI模型推理电路：构建包含推理结果哈希、模型权重、输入数据的witness
		return p.buildAIModelInferenceProofWitness(input)

	case "execution_proof_circuit":
		// 旧版执行证明电路：保留兼容性
		return p.buildExecutionWitness(input)

	case "state_transition_circuit":
		// 状态转换证明电路：包含状态变更的证明数据
		return p.buildStateTransitionWitness(input)

	case "computation_circuit":
		// 计算证明电路：包含计算结果的正确性证明
		return p.buildComputationWitness(input)

	default:
		// 通用证明电路：基础的witness构建
		return p.buildGenericWitness(input)
	}
}

// buildExecutionWitness 构建执行证明的witness
//
// 🎯 **修复**：直接返回 witness.Witness，使用 GenericExecutionCircuit 构建
func (p *Prover) buildExecutionWitness(input *interfaces.ZKProofInput) (witness.Witness, error) {
	p.logger.Debug("构建执行证明witness")

	// 从PrivateInputs中提取执行轨迹数据
	var executionTrace *ExecutionTraceData
	if input.PrivateInputs != nil {
		if trace, ok := input.PrivateInputs.(*ExecutionTraceData); ok {
			executionTrace = trace
		} else {
			p.logger.Debugf("私有输入不是ExecutionTraceData类型，尝试类型转换")
			executionTrace = p.extractExecutionTraceFromPrivateInputs(input.PrivateInputs)
		}
	}

	if executionTrace == nil {
		return nil, fmt.Errorf("无法从私有输入中提取执行轨迹数据")
	}

	// 将执行轨迹编码为电路友好的格式
	witnessData, err := p.encodeExecutionTraceForCircuit(executionTrace)
	if err != nil {
		return nil, fmt.Errorf("编码执行轨迹失败: %w", err)
	}

	// 构建 GenericExecutionCircuit 并设置数据
	var resultHashVar *big.Int
	if len(input.PublicInputs) > 0 {
		resultHashVar = new(big.Int).SetBytes(input.PublicInputs[0])
	} else {
		// 如果没有公开输入，使用执行哈希作为结果哈希
		resultHashVar = new(big.Int).SetBytes(witnessData.ExecutionHash)
	}

	circuit := &GenericExecutionCircuit{
		ResultHash:    resultHashVar,
		ExecutionData: new(big.Int).SetBytes(witnessData.ExecutionHash),
		AuxiliaryData: new(big.Int).SetBytes(witnessData.HostCallsHash),
	}

	// 🎯 **修复**：使用 frontend.NewWitness 创建 witness
	fullWitness, err := frontend.NewWitness(circuit, ecc.BN254.ScalarField())
	if err != nil {
		return nil, fmt.Errorf("创建执行witness失败: %w", err)
	}

	p.logger.Debugf("执行证明witness构建完成: hostCalls=%d, stateChanges=%d",
		len(executionTrace.HostFunctionCalls), len(executionTrace.StateChanges))
	return fullWitness, nil
}

// buildStateTransitionWitness 构建状态转换证明的witness
//
// 🎯 **修复**：直接返回 witness.Witness，而不是尝试设置到参数中
func (p *Prover) buildStateTransitionWitness(input *interfaces.ZKProofInput) (witness.Witness, error) {
	p.logger.Debug("构建状态转换证明witness")

	// 1. 从PrivateInputs中提取执行轨迹数据
	var executionTrace *ExecutionTraceData
	if input.PrivateInputs != nil {
		if trace, ok := input.PrivateInputs.(*ExecutionTraceData); ok {
			executionTrace = trace
		} else {
			p.logger.Debugf("私有输入不是ExecutionTraceData类型，尝试类型转换")
			executionTrace = p.extractExecutionTraceFromPrivateInputs(input.PrivateInputs)
		}
	}

	if executionTrace == nil {
		return nil, fmt.Errorf("无法从私有输入中提取执行轨迹数据")
	}

	// 2. 构建前状态哈希（执行开始时的状态）
	// 前状态 = 所有输入UTXO的状态哈希
	preStateHash := p.computePreStateHash(executionTrace)

	// 3. 构建后状态哈希（执行结束时的状态）
	// 后状态 = 所有输出UTXO的状态哈希
	postStateHash := p.computePostStateHash(executionTrace)

	// 4. 构建状态变更操作列表哈希
	stateTransitionHash := p.computeStateTransitionHash(executionTrace.StateChanges)

	// 5. 设置公开输入（链上可见的数据）
	if len(input.PublicInputs) >= 2 {
		// PublicInputs[0] = 前状态哈希
		// PublicInputs[1] = 后状态哈希
		preStateVar := new(big.Int).SetBytes(input.PublicInputs[0])

		// 使用GenericExecutionCircuit结构来设置witness
		circuit := &GenericExecutionCircuit{
			ResultHash:    preStateVar, // 使用前状态哈希作为结果哈希
			ExecutionData: new(big.Int).SetBytes(preStateHash),
			AuxiliaryData: new(big.Int).SetBytes(postStateHash),
		}

		// 🎯 **修复**：直接返回创建的 witness，而不是占位代码
		fullWitness, err := frontend.NewWitness(circuit, ecc.BN254.ScalarField())
		if err != nil {
			return nil, fmt.Errorf("创建状态转换witness失败: %w", err)
		}

		p.logger.Debugf("状态转换witness构建完成: preStateHash=%x, postStateHash=%x, transitions=%d, stateTransitionHash=%x",
			preStateHash[:8], postStateHash[:8], len(executionTrace.StateChanges), stateTransitionHash[:8])

		return fullWitness, nil
	}

	return nil, fmt.Errorf("状态转换证明需要至少2个公开输入（前状态哈希和后状态哈希）")
}

// computePreStateHash 计算前状态哈希（执行开始时的状态）
func (p *Prover) computePreStateHash(trace *ExecutionTraceData) []byte {
	var buf bytes.Buffer

	// 基于执行ID和开始时间构建前状态
	buf.WriteString(trace.ExecutionID)
	buf.WriteString("_pre_state")

	// 添加开始时间戳
	startTimeBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		startTimeBytes[i] = byte(trace.StartTime >> (i * 8))
	}
	buf.Write(startTimeBytes)

	return p.hashManager.SHA256(buf.Bytes())
}

// computePostStateHash 计算后状态哈希（执行结束时的状态）
func (p *Prover) computePostStateHash(trace *ExecutionTraceData) []byte {
	var buf bytes.Buffer

	// 基于执行ID和结束时间构建后状态
	buf.WriteString(trace.ExecutionID)
	buf.WriteString("_post_state")

	// 添加结束时间戳
	endTimeBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		endTimeBytes[i] = byte(trace.EndTime >> (i * 8))
	}
	buf.Write(endTimeBytes)

	// 包含状态变更的数量
	buf.WriteByte(byte(len(trace.StateChanges)))

	return p.hashManager.SHA256(buf.Bytes())
}

// computeStateTransitionHash 计算状态变更操作列表哈希
func (p *Prover) computeStateTransitionHash(changes []StateChangeData) []byte {
	if len(changes) == 0 {
		return p.hashManager.SHA256([]byte("no_state_changes"))
	}

	// 序列化所有状态变更
	serialized := p.serializeStateChanges(changes)
	return p.hashManager.SHA256(serialized)
}

// buildComputationWitness 构建计算证明的witness
//
// 🎯 **修复**：直接返回 witness.Witness，而不是尝试设置到参数中
func (p *Prover) buildComputationWitness(input *interfaces.ZKProofInput) (witness.Witness, error) {
	p.logger.Debug("构建计算证明witness")

	// 1. 从PrivateInputs中提取执行轨迹数据
	var executionTrace *ExecutionTraceData
	if input.PrivateInputs != nil {
		if trace, ok := input.PrivateInputs.(*ExecutionTraceData); ok {
			executionTrace = trace
		} else {
			p.logger.Debugf("私有输入不是ExecutionTraceData类型，尝试类型转换")
			executionTrace = p.extractExecutionTraceFromPrivateInputs(input.PrivateInputs)
		}
	}

	if executionTrace == nil {
		return nil, fmt.Errorf("无法从私有输入中提取执行轨迹数据")
	}

	// 2. 构建输入数据哈希（合约初始参数、UTXO数据等）
	inputDataHash := p.computeInputDataHash(executionTrace)

	// 3. 构建计算过程哈希（宿主函数调用序列）
	computationProcessHash := p.computeComputationProcessHash(executionTrace.HostFunctionCalls)

	// 4. 构建输出结果哈希（执行结果、返回数据等）
	outputResultHash := p.computeOutputResultHash(executionTrace)

	// 5. 设置公开输入（链上可见的数据）
	if len(input.PublicInputs) >= 1 {
		// PublicInputs[0] = 输出结果哈希
		resultHashVar := new(big.Int).SetBytes(input.PublicInputs[0])

		// 使用GenericExecutionCircuit结构来设置witness
		circuit := &GenericExecutionCircuit{
			ResultHash:    resultHashVar,
			ExecutionData: new(big.Int).SetBytes(inputDataHash),
			AuxiliaryData: new(big.Int).SetBytes(computationProcessHash),
		}

		// 🎯 **修复**：直接返回创建的 witness，而不是占位代码
		fullWitness, err := frontend.NewWitness(circuit, ecc.BN254.ScalarField())
		if err != nil {
			return nil, fmt.Errorf("创建计算witness失败: %w", err)
		}

		p.logger.Debugf("计算witness构建完成: inputHash=%x, processHash=%x, outputHash=%x, hostCalls=%d",
			inputDataHash[:8], computationProcessHash[:8], outputResultHash[:8], len(executionTrace.HostFunctionCalls))

		return fullWitness, nil
	}

	return nil, fmt.Errorf("计算证明需要至少1个公开输入（输出结果哈希）")
}

// computeInputDataHash 计算输入数据哈希（合约初始参数、UTXO数据等）
func (p *Prover) computeInputDataHash(trace *ExecutionTraceData) []byte {
	var buf bytes.Buffer

	// 基于执行ID构建输入数据哈希
	buf.WriteString(trace.ExecutionID)
	buf.WriteString("_input_data")

	// 添加开始时间戳（作为输入数据的一部分）
	startTimeBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		startTimeBytes[i] = byte(trace.StartTime >> (i * 8))
	}
	buf.Write(startTimeBytes)

	return p.hashManager.SHA256(buf.Bytes())
}

// computeComputationProcessHash 计算计算过程哈希（宿主函数调用序列）
func (p *Prover) computeComputationProcessHash(calls []HostFunctionCallData) []byte {
	if len(calls) == 0 {
		return p.hashManager.SHA256([]byte("no_host_calls"))
	}

	// 序列化所有宿主函数调用
	serialized := p.serializeHostFunctionCalls(calls)
	return p.hashManager.SHA256(serialized)
}

// computeOutputResultHash 计算输出结果哈希（执行结果、返回数据等）
func (p *Prover) computeOutputResultHash(trace *ExecutionTraceData) []byte {
	var buf bytes.Buffer

	// 基于执行ID构建输出结果哈希
	buf.WriteString(trace.ExecutionID)
	buf.WriteString("_output_result")

	// 添加结束时间戳
	endTimeBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		endTimeBytes[i] = byte(trace.EndTime >> (i * 8))
	}
	buf.Write(endTimeBytes)

	// 包含宿主函数调用数量（作为输出的一部分）
	buf.WriteByte(byte(len(trace.HostFunctionCalls)))

	return p.hashManager.SHA256(buf.Bytes())
}

// buildGenericWitness 构建通用证明的witness
//
// 🎯 **修复**：直接返回 witness.Witness，使用 GenericExecutionCircuit 构建
func (p *Prover) buildGenericWitness(input *interfaces.ZKProofInput) (witness.Witness, error) {
	p.logger.Debug("构建通用证明witness")

	// 构建 GenericExecutionCircuit
	var resultHashVar *big.Int
	if len(input.PublicInputs) > 0 {
		resultHashVar = new(big.Int).SetBytes(input.PublicInputs[0])
	} else {
		// 如果没有公开输入，使用零值
		resultHashVar = big.NewInt(0)
	}

	// 构建执行数据和辅助数据
	var executionDataVar *big.Int
	var auxiliaryDataVar *big.Int

	if input.PrivateInputs != nil {
		// 尝试从私有输入中提取数据
		if trace, ok := input.PrivateInputs.(*ExecutionTraceData); ok {
			witnessData, err := p.encodeExecutionTraceForCircuit(trace)
			if err == nil {
				executionDataVar = new(big.Int).SetBytes(witnessData.ExecutionHash)
				auxiliaryDataVar = new(big.Int).SetBytes(witnessData.HostCallsHash)
			} else {
				// 如果编码失败，使用默认值
				executionDataVar = big.NewInt(0)
				auxiliaryDataVar = big.NewInt(0)
			}
		} else {
			// 如果私有输入不是 ExecutionTraceData，使用默认值
			executionDataVar = big.NewInt(0)
			auxiliaryDataVar = big.NewInt(0)
		}
	} else {
		// 如果没有私有输入，使用默认值
		executionDataVar = big.NewInt(0)
		auxiliaryDataVar = big.NewInt(0)
	}

	circuit := &GenericExecutionCircuit{
		ResultHash:    resultHashVar,
		ExecutionData: executionDataVar,
		AuxiliaryData: auxiliaryDataVar,
	}

	// 🎯 **修复**：使用 frontend.NewWitness 创建 witness
	fullWitness, err := frontend.NewWitness(circuit, ecc.BN254.ScalarField())
	if err != nil {
		return nil, fmt.Errorf("创建通用witness失败: %w", err)
	}

	p.logger.Debugf("通用证明witness构建完成: resultHash=%s", resultHashVar.String())
	return fullWitness, nil
}

// ==================== ZK Witness 构建辅助方法 ====================

// extractExecutionTraceFromPrivateInputs 从私有输入中提取执行轨迹数据
func (p *Prover) extractExecutionTraceFromPrivateInputs(privateInputs interface{}) *ExecutionTraceData {
	p.logger.Debug("尝试从私有输入中提取执行轨迹数据")

	// 尝试各种可能的类型转换
	switch v := privateInputs.(type) {
	case *ExecutionTraceData:
		return v
	case map[string]interface{}:
		// 从map中构建ExecutionTraceData
		return p.buildExecutionTraceFromMap(v)
	case []byte:
		// 尝试从JSON字节数组解析
		return p.parseExecutionTraceFromJSON(v)
	default:
		p.logger.Debugf("未知的私有输入类型: %T", privateInputs)
		return nil
	}
}

// buildExecutionTraceFromMap 从map构建ExecutionTraceData
func (p *Prover) buildExecutionTraceFromMap(data map[string]interface{}) *ExecutionTraceData {
	trace := &ExecutionTraceData{}

	if id, ok := data["execution_id"].(string); ok {
		trace.ExecutionID = id
	}

	if startTime, ok := data["start_time"].(int64); ok {
		trace.StartTime = startTime
	}

	if endTime, ok := data["end_time"].(int64); ok {
		trace.EndTime = endTime
	}

	if duration, ok := data["duration"].(int64); ok {
		trace.Duration = duration
	}

	// 提取宿主函数调用数据
	if hostCallsRaw, ok := data["host_function_calls"]; ok {
		if hostCallsArray, ok := hostCallsRaw.([]interface{}); ok {
			trace.HostFunctionCalls = make([]HostFunctionCallData, 0, len(hostCallsArray))
			for _, callRaw := range hostCallsArray {
				if callMap, ok := callRaw.(map[string]interface{}); ok {
					call := HostFunctionCallData{}
					if fn, ok := callMap["function_name"].(string); ok {
						call.FunctionName = fn
					}
					if paramCount, ok := callMap["param_count"].(float64); ok {
						call.ParamCount = int(paramCount)
					}
					if hasResult, ok := callMap["has_result"].(bool); ok {
						call.HasResult = hasResult
					}
					if success, ok := callMap["success"].(bool); ok {
						call.Success = success
					}
					if timestamp, ok := callMap["timestamp"].(float64); ok {
						call.Timestamp = int64(timestamp)
					}
					if duration, ok := callMap["duration"].(float64); ok {
						call.Duration = int64(duration)
					}
					trace.HostFunctionCalls = append(trace.HostFunctionCalls, call)
				}
			}
		}
	}

	// 提取状态变更数据
	if stateChangesRaw, ok := data["state_changes"]; ok {
		if stateChangesArray, ok := stateChangesRaw.([]interface{}); ok {
			trace.StateChanges = make([]StateChangeData, 0, len(stateChangesArray))
			for _, changeRaw := range stateChangesArray {
				if changeMap, ok := changeRaw.(map[string]interface{}); ok {
					change := StateChangeData{}
					if changeType, ok := changeMap["type"].(string); ok {
						change.Type = changeType
					}
					if key, ok := changeMap["key"].(string); ok {
						change.Key = key
					}
					if hasOld, ok := changeMap["has_old"].(bool); ok {
						change.HasOld = hasOld
					}
					if hasNew, ok := changeMap["has_new"].(bool); ok {
						change.HasNew = hasNew
					}
					if timestamp, ok := changeMap["timestamp"].(float64); ok {
						change.Timestamp = int64(timestamp)
					}
					trace.StateChanges = append(trace.StateChanges, change)
				}
			}
		}
	}

	// 提取执行事件数据
	if eventsRaw, ok := data["execution_events"]; ok {
		if eventsArray, ok := eventsRaw.([]interface{}); ok {
			trace.ExecutionEvents = make([]ExecutionEventData, 0, len(eventsArray))
			for _, eventRaw := range eventsArray {
				if eventMap, ok := eventRaw.(map[string]interface{}); ok {
					event := ExecutionEventData{}
					if eventType, ok := eventMap["event_type"].(string); ok {
						event.EventType = eventType
					}
					if timestamp, ok := eventMap["timestamp"].(float64); ok {
						event.Timestamp = int64(timestamp)
					}
					trace.ExecutionEvents = append(trace.ExecutionEvents, event)
				}
			}
		}
	}

	p.logger.Debugf("从map构建执行轨迹: executionID=%s, duration=%d, hostCalls=%d, stateChanges=%d",
		trace.ExecutionID, trace.Duration, len(trace.HostFunctionCalls), len(trace.StateChanges))
	return trace
}

// parseExecutionTraceFromJSON 从JSON解析ExecutionTraceData
func (p *Prover) parseExecutionTraceFromJSON(jsonData []byte) *ExecutionTraceData {
	p.logger.Debug("从JSON解析执行轨迹")

	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		p.logger.Debugf("JSON解析失败: %v", err)
		return nil
	}

	// 复用buildExecutionTraceFromMap方法
	return p.buildExecutionTraceFromMap(data)
}

// encodeExecutionTraceForCircuit 将执行轨迹编码为电路友好的格式
func (p *Prover) encodeExecutionTraceForCircuit(trace *ExecutionTraceData) (*CircuitWitnessData, error) {
	p.logger.Debug("编码执行轨迹为电路友好格式")

	witnessData := &CircuitWitnessData{
		ExecutionID:      p.hashManager.SHA256([]byte(trace.ExecutionID)),
		StartTime:        uint64(trace.StartTime),
		EndTime:          uint64(trace.EndTime),
		HostCallCount:    uint32(len(trace.HostFunctionCalls)),
		StateChangeCount: uint32(len(trace.StateChanges)),
	}

	// 计算宿主函数调用哈希
	if len(trace.HostFunctionCalls) > 0 {
		hostCallsData := p.serializeHostFunctionCalls(trace.HostFunctionCalls)
		witnessData.HostCallsHash = p.hashManager.SHA256(hostCallsData)
	}

	// 计算状态变更哈希
	if len(trace.StateChanges) > 0 {
		stateChangesData := p.serializeStateChanges(trace.StateChanges)
		witnessData.StateChangesHash = p.hashManager.SHA256(stateChangesData)
	}

	// 计算整体执行哈希（承诺）
	witnessData.ExecutionHash = p.computeExecutionCommitment(witnessData)

	p.logger.Debugf("执行轨迹编码完成: hostCalls=%d, stateChanges=%d",
		witnessData.HostCallCount, witnessData.StateChangeCount)
	return witnessData, nil
}

// serializeHostFunctionCalls 序列化宿主函数调用数据
func (p *Prover) serializeHostFunctionCalls(calls []HostFunctionCallData) []byte {
	var buf bytes.Buffer

	for _, call := range calls {
		// 写入函数名（哈希）
		nameHash := p.hashManager.SHA256([]byte(call.FunctionName))
		buf.Write(nameHash)

		// 写入统计信息（小端序）
		buf.Write([]byte{
			byte(call.ParamCount),
			byte(boolToByte(call.HasResult)),
			byte(boolToByte(call.Success)),
			0, // 填充字节
		})

		// 写入时间戳（8字节小端序）
		timestampBytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			timestampBytes[i] = byte(call.Timestamp >> (i * 8))
		}
		buf.Write(timestampBytes)
	}

	return buf.Bytes()
}

// serializeStateChanges 序列化状态变更数据
func (p *Prover) serializeStateChanges(changes []StateChangeData) []byte {
	var buf bytes.Buffer

	for _, change := range changes {
		// 写入变更类型（哈希）
		typeHash := p.hashManager.SHA256([]byte(change.Type))
		buf.Write(typeHash)

		// 写入键（哈希）
		keyHash := p.hashManager.SHA256([]byte(change.Key))
		buf.Write(keyHash)

		// 写入标志位
		buf.Write([]byte{
			byte(boolToByte(change.HasOld)),
			byte(boolToByte(change.HasNew)),
			0, 0, // 填充字节
		})

		// 写入时间戳（8字节小端序）
		timestampBytes := make([]byte, 8)
		for i := 0; i < 8; i++ {
			timestampBytes[i] = byte(change.Timestamp >> (i * 8))
		}
		buf.Write(timestampBytes)
	}

	return buf.Bytes()
}

// computeExecutionCommitment 计算执行承诺（整体哈希）
func (p *Prover) computeExecutionCommitment(data *CircuitWitnessData) []byte {
	var buf bytes.Buffer

	// 连接所有关键数据
	buf.Write(data.ExecutionID)

	// 写入时间信息（8字节小端序）
	for _, val := range []uint64{data.StartTime, data.EndTime} {
		for i := 0; i < 8; i++ {
			buf.WriteByte(byte(val >> (i * 8)))
		}
	}

	// 写入计数信息（4字节小端序）
	for _, val := range []uint32{data.HostCallCount, data.StateChangeCount} {
		for i := 0; i < 4; i++ {
			buf.WriteByte(byte(val >> (i * 8)))
		}
	}

	buf.Write(data.HostCallsHash)
	buf.Write(data.StateChangesHash)

	return p.hashManager.SHA256(buf.Bytes())
}

// boolToByte 将布尔值转换为字节
func boolToByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// buildContractExecutionProofWitness 构建合约执行电路的proof witness
//
// 🎯 **关键修复**：使用 frontend.NewWitness 将电路结构体转换为 gnark witness
func (p *Prover) buildContractExecutionProofWitness(input *interfaces.ZKProofInput) (witness.Witness, error) {
	p.logger.Debug("构建合约执行proof witness")

	// 构建电路实例并填充数据
	contractCircuit := &ContractExecutionCircuit{}

	// 设置公开输入：执行结果哈希
	if len(input.PublicInputs) > 0 {
		// 将字节数组转换为 big.Int
		executionResultHash := new(big.Int).SetBytes(input.PublicInputs[0])
		contractCircuit.ExecutionResultHash = executionResultHash
		p.logger.Debugf("设置执行结果哈希: %s", executionResultHash.String())
	} else {
		return nil, fmt.Errorf("缺少公开输入：执行结果哈希")
	}

	// 设置私有输入：执行轨迹和状态变更
	//
	// 🎯 **关键修复**：去除默认值，改为强制要求有效输入
	if privateData, ok := input.PrivateInputs.(map[string]interface{}); ok {
		// 执行轨迹
		if traceData, exists := privateData["execution_trace"]; exists {
			p.logger.Debugf("设置执行轨迹: %v (type=%T)", traceData, traceData)
			switch v := traceData.(type) {
			case []byte:
				if len(v) == 0 {
					return nil, fmt.Errorf("execution_trace 字节数组为空")
				}
				contractCircuit.ExecutionTrace = new(big.Int).SetBytes(v)
			case string:
				if v == "" {
					return nil, fmt.Errorf("execution_trace 字符串为空")
				}
				// 将字符串转为字节（确定性编码）
				contractCircuit.ExecutionTrace = new(big.Int).SetBytes([]byte(v))
			case *big.Int:
				if v == nil || v.Sign() == 0 {
					return nil, fmt.Errorf("execution_trace big.Int 无效")
				}
				contractCircuit.ExecutionTrace = v
			default:
				// ❌ 修复：不再使用默认值，改为返回错误
				return nil, fmt.Errorf("execution_trace 类型不支持: %T", traceData)
			}
			p.logger.Debug("✅ ExecutionTrace 设置成功")
		} else {
			// ❌ 修复：不再使用默认值，改为返回错误
			return nil, fmt.Errorf("缺少私有输入: execution_trace")
		}

		// 状态变更
		if stateDiff, exists := privateData["state_diff"]; exists {
			p.logger.Debugf("设置状态变更: %v (type=%T)", stateDiff, stateDiff)
			switch v := stateDiff.(type) {
			case []byte:
				if len(v) == 0 {
					return nil, fmt.Errorf("state_diff 字节数组为空")
				}
				contractCircuit.StateDiff = new(big.Int).SetBytes(v)
			case string:
				if v == "" {
					return nil, fmt.Errorf("state_diff 字符串为空")
				}
				// 将字符串转为字节（确定性编码）
				contractCircuit.StateDiff = new(big.Int).SetBytes([]byte(v))
			case *big.Int:
				if v == nil || v.Sign() == 0 {
					return nil, fmt.Errorf("state_diff big.Int 无效")
				}
				contractCircuit.StateDiff = v
			default:
				// ❌ 修复：不再使用默认值，改为返回错误
				return nil, fmt.Errorf("state_diff 类型不支持: %T", stateDiff)
			}
			p.logger.Debug("✅ StateDiff 设置成功")
		} else {
			// ❌ 修复：不再使用默认值，改为返回错误
			return nil, fmt.Errorf("缺少私有输入: state_diff")
		}
	} else {
		// ❌ 修复：不再使用默认值，改为返回错误
		return nil, fmt.Errorf("私有输入格式错误: 期望 map[string]interface{}, 实际 %T", input.PrivateInputs)
	}

	// 🎯 **关键步骤**：使用 frontend.NewWitness 创建正确的witness
	fullWitness, err := frontend.NewWitness(contractCircuit, ecc.BN254.ScalarField())
	if err != nil {
		return nil, fmt.Errorf("创建witness失败: %w", err)
	}

	p.logger.Debugf("合约执行witness构建成功: resultHash=%s", contractCircuit.ExecutionResultHash)
	return fullWitness, nil
}

// buildAIModelInferenceProofWitness 构建AI模型推理电路的proof witness
//
// 🎯 **关键修复**：使用 frontend.NewWitness 将电路结构体转换为 gnark witness
func (p *Prover) buildAIModelInferenceProofWitness(input *interfaces.ZKProofInput) (witness.Witness, error) {
	p.logger.Debug("构建AI模型推理proof witness")

	// 构建电路实例并填充数据
	inferenceCircuit := &AIModelInferenceCircuit{}

	// 设置公开输入：推理结果哈希
	if len(input.PublicInputs) > 0 {
		inferenceResultHash := new(big.Int).SetBytes(input.PublicInputs[0])
		inferenceCircuit.InferenceResultHash = inferenceResultHash
		p.logger.Debugf("设置推理结果哈希: %s", inferenceResultHash.String())
	} else {
		return nil, fmt.Errorf("缺少公开输入：推理结果哈希")
	}

	// 设置私有输入：模型权重和输入数据
	// 🎯 **关键修复**：去除默认值，改为强制要求有效输入
	if privateData, ok := input.PrivateInputs.(map[string]interface{}); ok {
		// 模型权重
		if modelWeights, exists := privateData["model_weights"]; exists {
			p.logger.Debugf("设置模型权重: %v (type=%T)", modelWeights, modelWeights)
			switch v := modelWeights.(type) {
			case []byte:
				if len(v) == 0 {
					return nil, fmt.Errorf("model_weights 字节数组为空")
				}
				inferenceCircuit.ModelWeights = new(big.Int).SetBytes(v)
			case string:
				if v == "" {
					return nil, fmt.Errorf("model_weights 字符串为空")
				}
				inferenceCircuit.ModelWeights = new(big.Int).SetBytes([]byte(v))
			case *big.Int:
				if v == nil || v.Sign() == 0 {
					return nil, fmt.Errorf("model_weights big.Int 无效")
				}
				inferenceCircuit.ModelWeights = v
			default:
				return nil, fmt.Errorf("model_weights 类型不支持: %T", modelWeights)
			}
			p.logger.Debug("✅ ModelWeights 设置成功")
		} else {
			return nil, fmt.Errorf("缺少私有输入: model_weights")
		}

		// 输入数据
		if inputData, exists := privateData["input_data"]; exists {
			p.logger.Debugf("设置输入数据: %v (type=%T)", inputData, inputData)
			switch v := inputData.(type) {
			case []byte:
				if len(v) == 0 {
					return nil, fmt.Errorf("input_data 字节数组为空")
				}
				inferenceCircuit.InputData = new(big.Int).SetBytes(v)
			case string:
				if v == "" {
					return nil, fmt.Errorf("input_data 字符串为空")
				}
				inferenceCircuit.InputData = new(big.Int).SetBytes([]byte(v))
			case *big.Int:
				if v == nil || v.Sign() == 0 {
					return nil, fmt.Errorf("input_data big.Int 无效")
				}
				inferenceCircuit.InputData = v
			default:
				return nil, fmt.Errorf("input_data 类型不支持: %T", inputData)
			}
			p.logger.Debug("✅ InputData 设置成功")
		} else {
			return nil, fmt.Errorf("缺少私有输入: input_data")
		}
	} else {
		return nil, fmt.Errorf("私有输入格式错误: 期望 map[string]interface{}, 实际 %T", input.PrivateInputs)
	}

	// 🎯 **关键步骤**：使用 frontend.NewWitness 创建正确的witness
	fullWitness, err := frontend.NewWitness(inferenceCircuit, ecc.BN254.ScalarField())
	if err != nil {
		return nil, fmt.Errorf("创建witness失败: %w", err)
	}

	p.logger.Debugf("AI模型推理witness构建成功: resultHash=%s", inferenceCircuit.InferenceResultHash)
	return fullWitness, nil
}
