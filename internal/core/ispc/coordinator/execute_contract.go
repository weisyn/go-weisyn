package coordinator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	// 内部模块依赖
	hostabi "github.com/weisyn/v1/internal/core/ispc/hostabi"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"

	// 协议定义
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"

	// 公共接口
	ispcintf "github.com/weisyn/v1/pkg/interfaces/ispc"
	"golang.org/x/crypto/ripemd160"
	"google.golang.org/protobuf/proto"
)

// 说明：为避免跨包类型不一致导致无法读取，这里使用统一的字符串 key 传递 ExecutionContext。
// 注意：key 名称在引擎侧必须保持一致（"execution_context"）。

// ExecuteWASMContract 执行WASM智能合约 (强类型)
//
// 🎯 **核心职责**:
//   - 调度WASM引擎执行合约
//   - 生成零知识证明 (必须成功，否则报错)
//   - 返回WASMExecutionResult (不涉及交易构建/签名/提交)
//
// 📋 **参数**:
//   - ctx: 上下文
//   - contractHash: 合约内容哈希 (用于定位合约资源)
//   - methodName: 要调用的方法名
//   - params: 方法参数 (WASM原生类型 []uint64)
//
// 🔧 **返回值**:
//   - *WASMExecutionResult: 执行产物 (ReturnValues, StateOutputProto, ZKProof等)
//   - error: 执行失败或ZK证明生成失败时的错误
//
// 🌐 **单向依赖**: ISPC → 无
func (m *Manager) ExecuteWASMContract(ctx context.Context, contractHash []byte, methodName string, params []uint64, initParams []byte, callerAddress string) (*ispcintf.WASMExecutionResult, error) {
	// Panic recovery: 确保panic不会导致程序崩溃
	defer func() {
		if r := recover(); r != nil {
			m.logger.Errorf("❌ ExecuteWASMContract panic recovered: %v", r)
		}
	}()

	// 精确计算 initParams 长度（nil 时返回 0）
	initParamsLenForLog := 0
	if initParams != nil {
		initParamsLenForLog = len(initParams)
	}
	m.logger.Debugf("开始执行智能合约: contractHash=%x, methodName=%s, callerAddress=%s, initParamsLen=%d", contractHash, methodName, callerAddress, initParamsLenForLog)

	// 1. 参数验证
	if len(contractHash) == 0 {
		return nil, WrapInvalidContractHashError(contractHash)
	}
	if methodName == "" {
		return nil, WrapInvalidFunctionNameError(methodName)
	}
	if callerAddress == "" {
		return nil, WrapMissingCallerAddressError()
	}

	// 2. 确保确定性执行开始时间（必须在创建执行上下文之前设置）
	// P0: 如果没有设置，从contextManager获取确定性时钟并设置
	var executionStartTime time.Time
	if executionStart := ctx.Value(ContextKeyExecutionStart); executionStart != nil {
		if startTime, ok := executionStart.(time.Time); ok {
			executionStartTime = startTime
		}
	}
	// 如果没有设置，从contextManager获取确定性时钟
	if executionStartTime.IsZero() {
		if m.contextManager == nil {
			return nil, fmt.Errorf("contextManager未初始化，无法获取确定性时钟")
		}
		// 从contextManager获取确定性时钟
		deterministicClock := m.contextManager.GetDeterministicClock()
		if deterministicClock == nil {
			return nil, fmt.Errorf("contextManager的确定性时钟未初始化")
		}
		executionStartTime = deterministicClock.Now()
		// 设置到ctx中，确保后续使用
		ctx = context.WithValue(ctx, ContextKeyExecutionStart, executionStartTime)
	}

	// 3. 创建执行上下文（使用确定性时间生成executionID）
	executionID := fmt.Sprintf("exec_%d", executionStartTime.UnixNano())

	executionContext, err := m.contextManager.CreateContext(ctx, executionID, callerAddress)
	if err != nil {
		return nil, WrapContextCreationFailedError(executionID, err)
	}

	// 3.1 注入合约地址到执行上下文（用于合约代币输出）
	contractAddrBytes, err := m.deriveContractAddress(contractHash)
	if err != nil {
		return nil, err
	}
	if setter, ok := executionContext.(interface{ SetContractAddress([]byte) error }); ok {
		if err := setter.SetContractAddress(contractAddrBytes); err != nil {
			return nil, fmt.Errorf("设置合约地址失败: %w", err)
		}
	} else if m.logger != nil {
		m.logger.Warn("执行上下文不支持设置合约地址接口，可能导致合约代币输出失败")
	}

	// ✅ 设置合约调用参数（initParams）到 ExecutionContext
	// 🎯 **关键修复**：确保客户端传递的参数能够被合约通过 GetContractParams() 获取
	// 📋 **参数状态精确判断**：
	//   - initParams == nil: 客户端未传递 payload 字段（精确：nil）
	//   - initParams != nil && len(initParams) == 0: 客户端传递了空 payload（精确：空切片）
	//   - initParams != nil && len(initParams) > 0: 客户端传递了有效参数（精确：有内容）
	initParamsLen := 0
	initParamsIsNil := initParams == nil
	if !initParamsIsNil {
		initParamsLen = len(initParams)
	}

	// 设置参数到 ExecutionContext（SetInitParams 会处理 nil 情况，将其转换为空切片）
	if err := executionContext.SetInitParams(initParams); err != nil {
		// SetInitParams 的实现不会返回错误（总是返回 nil），但为了防御性编程，仍然检查
		m.logger.Errorf("设置合约调用参数失败: %v（这不应该发生）", err)
		return nil, fmt.Errorf("设置合约调用参数失败: %w", err)
	}

	// 记录参数设置状态（精确记录，用于调试和问题排查）
	if initParamsIsNil {
		m.logger.Debugf("✅ 已设置合约调用参数: nil -> 空切片（客户端未传递 payload）")
	} else if initParamsLen == 0 {
		m.logger.Debugf("✅ 已设置合约调用参数: 0 字节（客户端传递了空 payload）")
	} else {
		m.logger.Debugf("✅ 已设置合约调用参数: %d 字节（客户端传递了有效参数）", initParamsLen)
	}

	// P0: 获取资源限制配置
	resourceLimits := m.getISPCResourceLimits()

	// 确保在所有返回路径上清理执行上下文和完成资源统计
	defer func() {
		// P0: 完成资源使用统计
		if executionContext != nil {
			executionContext.FinalizeResourceUsage()

			// P0: 记录资源使用日志（如果启用）
			if usage := executionContext.GetResourceUsage(); usage != nil {
				m.logResourceUsage(usage)
			}
		}

		// 清理执行上下文
		if destroyErr := m.contextManager.DestroyContext(ctx, executionID); destroyErr != nil {
			m.logger.Debugf("清理执行上下文失败: executionID=%s, error=%v", executionID, destroyErr)
		}
	}()

	// 3. 调用WASM引擎执行 (直接使用 []uint64 参数)
	m.logger.Debug("调用WASM引擎执行合约方法")

	wasmCtx := context.Background()
	wasmCtx, wasmCancel := context.WithTimeout(wasmCtx, 30*time.Second)
	defer wasmCancel()

	// 从原始ctx中提取追踪信息到隔离上下文
	if traceID := ctx.Value(ContextKeyTraceID); traceID != nil {
		wasmCtx = context.WithValue(wasmCtx, ContextKeyTraceID, traceID)
	}

	// ===== 将 ExecutionContext 注入到 context 中传递给 WASM Engine =====
	// 使用 hostabi.WithExecutionContext 确保key类型一致
	wasmCtx = hostabi.WithExecutionContext(wasmCtx, executionContext)

	// ✅ 通过engines.Manager统一调用，符合架构约束：单一入口、引擎内部化
	// 直接使用contractHash []byte，无需转换为string
	result, err := m.engineManager.ExecuteWASM(wasmCtx, contractHash, methodName, params)
	if err != nil {
		return nil, WrapExecutionFailedError(fmt.Sprintf("%x", contractHash), methodName, err)
	}

	m.logger.Debugf("WASM引擎执行成功: result=%v", result)

	// P0: 执行完成同步点 - 刷新轨迹记录队列（如果启用异步轨迹记录）
	if err := m.contextManager.FlushTraceQueue(); err != nil {
		m.logger.Warnf("刷新轨迹记录队列失败: %v", err)
		// 不返回错误，继续执行（轨迹可能已经写入）
	}

	// 4. 提取执行轨迹
	executionTrace, err := m.extractExecutionTrace(ctx, executionContext)
	if err != nil {
		return nil, WrapExecutionTraceExtractionFailedError(executionID, err)
	}

	// 4.1 计算状态快照哈希并写入执行上下文
	stateBeforeHash, stateAfterHash := computeStateSnapshotHashes(executionTrace)
	if snapshotCtx, ok := executionContext.(ispcInterfaces.StateSnapshotProvider); ok {
		snapshotCtx.SetStateSnapshots(stateBeforeHash, stateAfterHash)
	}

	// 5. 计算执行结果哈希
	executionResultHash, err := m.computeExecutionResultHash(result, executionTrace)
	if err != nil {
		return nil, WrapExecutionResultHashComputationFailedError(err)
	}

	// 6. 生成零知识证明（同步或异步）
	var zkProof *pb.ZKStateProof
	var zkProofTaskID string

	if m.asyncZKProofEnabled {
		// 提交异步任务
		taskID, err := m.submitZKProofTask(ctx, executionID, executionResultHash, executionTrace, "contract_execution", 0)
		if err != nil {
			m.logger.Warnf("异步ZK证明任务提交失败，回退到同步生成: %v", err)
			// 回退到同步生成
			zkProof, err = m.generateZKProof(ctx, executionResultHash, executionTrace)
			if err != nil {
				return nil, WrapZKProofGenerationFailedError("contract_execution", err)
			}
		} else {
			zkProofTaskID = taskID
			// 构建ZK证明输入（用于创建pending状态的ZK证明）
			zkInput, err := m.buildZKProofInput(ctx, executionResultHash, executionTrace, "contract_execution")
			if err != nil {
				return nil, WrapZKProofGenerationFailedError("contract_execution", err)
			}
			// 创建pending状态的ZK证明（占位符）
			zkProof = m.createPendingZKProof(zkInput)
			m.logger.Infof("✅ 异步ZK证明任务已提交: taskID=%s, executionID=%s", taskID, executionID)
		}
	} else {
		// 同步生成（向后兼容）
		zkProof, err = m.generateZKProof(ctx, executionResultHash, executionTrace)
		if err != nil {
			return nil, WrapZKProofGenerationFailedError("contract_execution", err)
		}
	}

	if zkProof == nil {
		return nil, WrapZKProofEmptyError()
	}

	// ===== 关键观测日志：输出ZK证明关键信息（Info级别，便于CLI模式观察）=====
	if zkProofTaskID != "" {
		m.logger.Infof("🧩 ZK证明任务已提交（异步）: taskID=%s, circuit=%s v=%d",
			zkProofTaskID, zkProof.CircuitId, zkProof.CircuitVersion)
	} else {
		m.logger.Infof("🧩 ZK证明生成完成（同步）: circuit=%s v=%d constraints=%d proof=%dB vkHash=%x",
			zkProof.CircuitId,
			zkProof.CircuitVersion,
			zkProof.ConstraintCount,
			len(zkProof.Proof),
			zkProof.VerificationKeyHash,
		)
	}

	// 7. 生成状态ID
	stateID, err := m.generateStateID(ctx)
	if err != nil {
		return nil, WrapStateIDGenerationFailedError(err)
	}

	// 8. 构建完整的 pb.StateOutput（包含ZKProof）
	metadata := map[string]string{
		"execution_node": m.getNodeID(),
	}
	// 如果是异步证明，写入 task 信息，供上层“明确拒绝提交 pending 证明的交易”并提示用户轮询。
	if zkProofTaskID != "" {
		metadata["zk_proof_status"] = "pending"
		metadata["zk_proof_task_id"] = zkProofTaskID
	}

	// P0: 使用确定性执行时间（从上下文获取，必须已设置）
	var executionTimeStr string
	if executionStart := ctx.Value(ContextKeyExecutionStart); executionStart != nil {
		if startTime, ok := executionStart.(time.Time); ok {
			executionTimeStr = startTime.Format(time.RFC3339)
		}
	}
	// 如果上下文中没有，尝试从执行上下文获取确定性时间戳
	if executionTimeStr == "" {
		if execCtx, ok := executionContext.(interface{ GetDeterministicTimestamp() time.Time }); ok {
			executionTimeStr = execCtx.GetDeterministicTimestamp().Format(time.RFC3339)
		}
	}
	// 如果仍然没有，这是错误情况（不应该发生）
	if executionTimeStr == "" {
		return nil, fmt.Errorf("无法获取确定性执行时间：executionStartTime未正确设置")
	}
	metadata["execution_time"] = executionTimeStr

	// 直接构建protobuf定义的StateOutput（零转换）
	stateOutput := &pb.StateOutput{
		StateId:             stateID,
		StateVersion:        1,
		ZkProof:             zkProof, // ← 直接包含，必须非nil
		ExecutionResultHash: executionResultHash,
		ParentStateHash:     nil, // 初始状态无父状态，后续可通过状态链追溯
		Metadata:            metadata,
	}

	// ===== 关键观测日志：输出StateOutput关键信息（Info级别）=====
	m.logger.Infof("🧾 StateOutput 构建完成: stateID=%x execResultHash=%x", stateID, executionResultHash)

	// 9. 从ExecutionContext提取业务数据和事件
	returnData, err := executionContext.GetReturnData()
	if err != nil {
		m.logger.Warnf("提取返回数据失败: %v", err)
		returnData = nil
	}

	events, err := executionContext.GetEvents()
	if err != nil {
		m.logger.Warnf("提取事件失败: %v", err)
		events = nil
	}

	// 将ISPC内部的Event类型转换为公共接口的Event类型
	publicEvents := make([]*ispcintf.Event, 0, len(events))
	for _, evt := range events {
		if evt != nil {
			publicEvents = append(publicEvents, &ispcintf.Event{
				Type:      evt.Type,
				Timestamp: evt.Timestamp,
				Data:      evt.Data,
			})
		}
	}

	// 10. 获取合约地址（用于构建ExecutionProof）
	contractAddress := executionContext.GetContractAddress()
	if len(contractAddress) == 0 {
		m.logger.Warnf("合约地址为空，无法填充ExecutionProof.Context.resource_address")
	}

	// 计算合约执行时间（毫秒，用于合约证明）
	executionElapsed := time.Since(executionStartTime)
	executionTimeMs := uint64(executionElapsed.Milliseconds())
	if executionTimeMs == 0 {
		executionTimeMs = 1 // 保底为1ms，避免上下界问题
	}

	// 尝试提取交易草稿（如果合约构建了交易输出）
	var (
		draftTxProto *pb.Transaction
		txDraft      *ispcInterfaces.TransactionDraft
	)
	if ctxDraft, err := executionContext.GetTransactionDraft(); err != nil {
		m.logger.Debugf("获取交易草稿失败: %v", err)
	} else if ctxDraft != nil {
		txDraft = ctxDraft
	}

	// 如果草稿存在且包含交易，评估是否需要追加 ExecutionProof 引用输入
	if txDraft != nil && txDraft.Tx != nil {
		needsExecutionProof := false
		hasExecutionProofInput := false

		for _, input := range txDraft.Tx.GetInputs() {
			if input.UnlockingProof != nil {
				switch proof := input.UnlockingProof.(type) {
				case *pb.TxInput_ExecutionProof:
					if proof.ExecutionProof != nil {
						hasExecutionProofInput = true
						break
					}
				}
			}
		}

		if !hasExecutionProofInput {
			for _, output := range txDraft.Tx.GetOutputs() {
				if asset := output.GetAsset(); asset != nil {
					if asset.GetContractToken() != nil {
						needsExecutionProof = true
						break
					}
				}
			}
		}

		if needsExecutionProof {
			if m.eutxoQuery == nil {
				return nil, fmt.Errorf("queryService未初始化，无法获取合约资源交易")
			}

			// 1. 查询合约部署交易（引用合约UTXO）
			contractDeploymentTxHash, _, _, err := m.eutxoQuery.GetResourceTransaction(ctx, contractHash)
			if err != nil {
				return nil, fmt.Errorf("获取合约资源交易失败: %w", err)
			}
			if len(contractDeploymentTxHash) != 32 {
				return nil, fmt.Errorf("获取到的合约资源交易哈希长度无效: %d", len(contractDeploymentTxHash))
			}

			// 2. 构建 ExecutionProof
			// ⚠️ **注意**：构建基本的 IdentityProof（需要完整的签名和公钥，这里使用占位符）
			// 在实际交易构建流程中，应该从交易签名中获取真实的 signature 和 publicKey
			// 当前实现使用占位符是为了支持 ISPC 层的执行证明构建
			// TODO: 在交易构建阶段，应该从交易签名中提取真实的 signature 和 publicKey 并更新 IdentityProof
			callerIdentity := BuildIdentityProof(
				executionContext,
				nil, // contextHash 将在 BuildExecutionProof 中计算并设置
				nil, // signature 占位符（实际使用中应该提供完整的签名）
				nil, // publicKey 占位符（实际使用中应该提供完整的公钥）
			)

			execProof, err := BuildExecutionProof(
				stateOutput,
				executionContext,
				methodName,
				initParams,
				executionTimeMs,
				pb.ExecutionType_EXECUTION_TYPE_CONTRACT,
				callerIdentity,
			)
			if err != nil {
				return nil, fmt.Errorf("构建ExecutionProof失败: %w", err)
			}

			// 3. 组装引用型输入（is_reference_only = true）
			contractOutpoint := &pb.OutPoint{
				TxId:        contractDeploymentTxHash,
				OutputIndex: 0, // 合约部署交易的 ResourceOutput 默认位于索引 0
			}

			contractInput := &pb.TxInput{
				PreviousOutput:  contractOutpoint,
				IsReferenceOnly: true,
				Sequence:        0,
				UnlockingProof: &pb.TxInput_ExecutionProof{
					ExecutionProof: execProof,
				},
			}

			// 4. 追加到交易草稿输入列表
			txDraft.Tx.Inputs = append(txDraft.Tx.Inputs, contractInput)

			// 4.1 为消费型输入补充 ExecutionProof（用于解锁 ContractLock/ResourceLock）
			for _, input := range txDraft.Tx.Inputs {
				if input == nil || input.IsReferenceOnly {
					continue
				}
				if input.GetExecutionProof() != nil {
					continue
				}
				if clonedProof, ok := proto.Clone(execProof).(*pb.ExecutionProof); ok {
					input.UnlockingProof = &pb.TxInput_ExecutionProof{
						ExecutionProof: clonedProof,
					}
				} else {
					input.UnlockingProof = &pb.TxInput_ExecutionProof{
						ExecutionProof: execProof,
					}
				}
			}

			// 5. 同步更新 TransactionDraft（保持上下文一致）
			txDraft.Outputs = txDraft.Tx.GetOutputs()
			if err := executionContext.UpdateTransactionDraft(txDraft); err != nil {
				m.logger.Warnf("更新执行上下文草稿失败: %v", err)
			} else {
				m.logger.Debugf("已追加引用型输入（ExecutionProof），draftID=%s", txDraft.DraftID)
			}
		}

		if cloned, ok := proto.Clone(txDraft.Tx).(*pb.Transaction); ok {
			draftTxProto = cloned
		} else {
			draftTxProto = txDraft.Tx
		}
	}

	// 11. 构建WASMExecutionResult
	executionResult := &ispcintf.WASMExecutionResult{
		ReturnValues:     result,      // WASM原生返回值
		StateOutput:      stateOutput, // 完整的pb.StateOutput
		DraftTransaction: draftTxProto,
		ReturnData:       returnData,   // 业务返回数据
		Events:           publicEvents, // 事件列表
		ExecutionContext: map[string]interface{}{
			"execution_id":     executionID,
			"contract_hash":    fmt.Sprintf("%x", contractHash),
			"contract_address": contractAddress, // ✅ 新增：合约地址（用于构建ExecutionProof）
			"method_name":      methodName,
			"execution_time":   executionTimeStr, // P0: 使用确定性执行时间
		},
	}

	// P0: 如果使用异步ZK证明生成，添加任务ID到执行上下文
	if zkProofTaskID != "" {
		executionResult.ExecutionContext["zk_proof_task_id"] = zkProofTaskID
		executionResult.ExecutionContext["zk_proof_status"] = "pending"
	}

	m.logger.Debugf("WASM智能合约执行完成: executionID=%s, stateID=%x, returnData=%d字节, events=%d个",
		executionID, stateID, len(returnData), len(publicEvents))

	// P0: 检查资源限制（执行结束后）
	if usage := executionContext.GetResourceUsage(); usage != nil && resourceLimits != nil {
		if err := m.checkResourceLimits(usage, resourceLimits); err != nil {
			return nil, err
		}
	}

	return executionResult, nil
}

// ExecuteONNXModel 执行ONNX模型推理 (强类型)
//
// 🎯 **核心职责**:
//   - 调度ONNX引擎执行推理
//   - 生成零知识证明 (必须成功，否则报错)
//   - 返回ONNXExecutionResult (不涉及交易构建/签名/提交)
//
// 📋 **参数**:
//   - ctx: 上下文
//   - modelHash: AI模型内容哈希 (用于定位模型资源)
//   - inputs: 输入张量数据 (ONNX原生类型 [][]float64)
//
// 🔧 **返回值**:
//   - *ONNXExecutionResult: 执行产物 (ReturnTensors, StateOutputProto, ZKProof等)
//   - error: 执行失败或ZK证明生成失败时的错误
func (m *Manager) ExecuteONNXModel(ctx context.Context, modelHash []byte, tensorInputs []ispcintf.TensorInput) (*ispcintf.ONNXExecutionResult, error) {
	// 转换为内部接口类型
	internalTensorInputs := make([]ispcInterfaces.TensorInput, len(tensorInputs))
	for i, ti := range tensorInputs {
		internalTensorInputs[i] = ispcInterfaces.TensorInput{
			Name:      ti.Name,
			Data:      ti.Data,
			Int64Data: ti.Int64Data,
			Int32Data: ti.Int32Data,
			Int16Data: ti.Int16Data,
			Uint8Data: ti.Uint8Data,
			Shape:     ti.Shape,
			DataType:  ti.DataType,
		}
	}

	// Panic recovery: 确保panic不会导致程序崩溃
	defer func() {
		if r := recover(); r != nil {
			m.logger.Errorf("❌ ExecuteONNXModel panic recovered: %v", r)
		}
	}()

	m.logger.Debugf("开始执行AI模型推理: modelHash=%x", modelHash)

	// 1. 参数验证
	if len(modelHash) == 0 {
		return nil, WrapInvalidModelHashError(modelHash)
	}
	if len(internalTensorInputs) == 0 {
		return nil, WrapInvalidInputTensorsError(len(internalTensorInputs))
	}

	// 2. 确保确定性执行开始时间（必须在创建执行上下文之前设置）
	// P0: 如果没有设置，从contextManager获取确定性时钟并设置
	var executionStartTime time.Time
	if executionStart := ctx.Value(ContextKeyExecutionStart); executionStart != nil {
		if startTime, ok := executionStart.(time.Time); ok {
			executionStartTime = startTime
		}
	}
	// 如果没有设置，从contextManager获取确定性时钟
	if executionStartTime.IsZero() {
		if m.contextManager == nil {
			return nil, fmt.Errorf("contextManager未初始化，无法获取确定性时钟")
		}
		// 从contextManager获取确定性时钟
		deterministicClock := m.contextManager.GetDeterministicClock()
		if deterministicClock == nil {
			return nil, fmt.Errorf("contextManager的确定性时钟未初始化")
		}
		executionStartTime = deterministicClock.Now()
		// 设置到ctx中，确保后续使用
		ctx = context.WithValue(ctx, ContextKeyExecutionStart, executionStartTime)
	}

	// 3. 创建执行上下文（使用确定性时间生成executionID）
	executionID := fmt.Sprintf("exec_%d", executionStartTime.UnixNano())
	modelAddress := fmt.Sprintf("%x", modelHash)

	executionContext, err := m.contextManager.CreateContext(ctx, executionID, modelAddress)
	if err != nil {
		return nil, WrapContextCreationFailedError(executionID, err)
	}

	// P0: 获取资源限制配置
	resourceLimits := m.getISPCResourceLimits()

	defer func() {
		// P0: 完成资源使用统计
		if executionContext != nil {
			executionContext.FinalizeResourceUsage()

			// P0: 记录资源使用日志（如果启用）
			if usage := executionContext.GetResourceUsage(); usage != nil {
				m.logResourceUsage(usage)
			}
		}

		// 清理执行上下文
		if destroyErr := m.contextManager.DestroyContext(ctx, executionID); destroyErr != nil {
			m.logger.Debugf("清理执行上下文失败: executionID=%s, error=%v", executionID, destroyErr)
		}
	}()

	// 3. 调用ONNX引擎执行推理 (直接使用 [][]float64 参数)
	m.logger.Debug("调用ONNX引擎执行推理")

	onnxCtx, onnxCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer onnxCancel()

	// ✅ 通过engines.Manager统一调用，符合架构约束：单一入口、引擎内部化
	// ExecutionContext已通过context传递（coordinator注入）
	onnxCtx = hostabi.WithExecutionContext(onnxCtx, executionContext)
	
	// Phase 1: 记录执行开始时间（用于 CU 计算）
	// 注意：executionStartTime 已经在前面从 context 或确定性时钟获取
	executionStartTimeForCU := executionStartTime
	
	outputs, err := m.engineManager.ExecuteONNX(onnxCtx, modelHash, internalTensorInputs)
	if err != nil {
		return nil, WrapExecutionFailedError(fmt.Sprintf("%x", modelHash), "onnx_inference", err)
	}

	// Phase 1: 记录执行结束时间（用于 CU 计算）
	executionEndTime := time.Now()
	executionDurationMs := uint64(executionEndTime.Sub(executionStartTimeForCU).Milliseconds())

	// Phase 1: 计算输入大小（字节）
	inputSizeBytes := uint64(0)
	for _, ti := range internalTensorInputs {
		// 估算输入大小：shape 乘积 * 数据类型大小
		elements := uint64(1)
		for _, dim := range ti.Shape {
			elements *= uint64(dim)
		}
		// 根据数据类型估算字节数（简化：float32=4, float64=8, int32=4, int64=8, uint8=1）
		bytesPerElement := uint64(4) // 默认 float32
		if ti.DataType == "float64" || ti.DataType == "int64" {
			bytesPerElement = 8
		} else if ti.DataType == "uint8" {
			bytesPerElement = 1
		}
		inputSizeBytes += elements * bytesPerElement
	}

	// 统计输出张量总元素数和输出大小（用于日志、结果哈希和带宽统计）
	totalValues := 0
	outputSizeBytes := uint64(0)
	for _, out := range outputs {
		totalValues += len(out.Values)
		// Phase 5: 计算输出带宽使用量
		elements := uint64(1)
		for _, dim := range out.Shape {
			elements *= uint64(dim)
		}
		bytesPerElement := uint64(4) // 默认 float32
		if out.DType == "float64" || out.DType == "int64" {
			bytesPerElement = 8
		} else if out.DType == "uint8" {
			bytesPerElement = 1
		}
		outputSizeBytes += elements * bytesPerElement
	}
	m.logger.Debugf("ONNX引擎推理成功: outputs=%d total_values=%d output_size=%d bytes", len(outputs), totalValues, outputSizeBytes)

	// Phase 1: 计算 CU（Compute Units）
	var computeUnits float64
	if m.computeMeter != nil {
		ops := OperationStats{
			StorageOps:         0, // ONNX 模型不涉及存储操作
			CrossContractCalls: 0, // ONNX 模型不涉及跨合约调用
			// Phase 5: 预留多维资源使用字段（当前仅统计，不计费）
			StorageBytes:       0, // 存储使用量（字节）
			BandwidthInBytes:   inputSizeBytes, // 输入带宽使用量
			BandwidthOutBytes:  outputSizeBytes, // 输出带宽使用量
		}
		cu, err := m.computeMeter.CalculateCU(
			ctx,
			ResourceTypeAIModel,
			modelHash,
			inputSizeBytes,
			executionDurationMs,
			ops,
		)
		if err != nil {
			m.logger.Warnf("计算 CU 失败: %v，使用默认值 0", err)
			computeUnits = 0
		} else {
			computeUnits = cu
		}
		m.logger.Debugf("Phase 1: 计算 CU 完成: modelHash=%x, inputSize=%d bytes, execTime=%d ms, CU=%.2f",
			modelHash, inputSizeBytes, executionDurationMs, computeUnits)
	} else {
		m.logger.Warnf("ComputeMeter 未初始化，跳过 CU 计算")
		computeUnits = 0
	}

	// P0: 执行完成同步点 - 刷新轨迹记录队列（如果启用异步轨迹记录）
	if err := m.contextManager.FlushTraceQueue(); err != nil {
		m.logger.Warnf("刷新轨迹记录队列失败: %v", err)
		// 不返回错误，继续执行（轨迹可能已经写入）
	}

	// 4. 提取执行轨迹
	executionTrace, err := m.extractExecutionTrace(ctx, executionContext)
	if err != nil {
		return nil, WrapExecutionTraceExtractionFailedError(executionID, err)
	}

	// 5. 计算执行结果哈希 (使用输出张量数量作为简单特征，后续可扩展为更丰富的摘要)
	resultForHash := []uint64{uint64(len(outputs))}
	executionResultHash, err := m.computeExecutionResultHash(resultForHash, executionTrace)
	if err != nil {
		return nil, WrapExecutionResultHashComputationFailedError(err)
	}

	// 6. 生成零知识证明（同步或异步）
	var zkProof *pb.ZKStateProof
	var zkProofTaskID string

	if m.asyncZKProofEnabled {
		// 提交异步任务
		taskID, err := m.submitZKProofTask(ctx, executionID, executionResultHash, executionTrace, "aimodel_inference", 0)
		if err != nil {
			m.logger.Warnf("异步ZK证明任务提交失败，回退到同步生成: %v", err)
			// 回退到同步生成
			zkProof, err = m.generateZKProof(ctx, executionResultHash, executionTrace)
			if err != nil {
				return nil, WrapZKProofGenerationFailedError("onnx_inference", err)
			}
		} else {
			zkProofTaskID = taskID
			// 构建ZK证明输入（用于创建pending状态的ZK证明）
			zkInput, err := m.buildZKProofInput(ctx, executionResultHash, executionTrace, "aimodel_inference")
			if err != nil {
				return nil, WrapZKProofGenerationFailedError("onnx_inference", err)
			}
			// 创建pending状态的ZK证明（占位符）
			zkProof = m.createPendingZKProof(zkInput)
			m.logger.Infof("✅ 异步ZK证明任务已提交: taskID=%s, executionID=%s", taskID, executionID)
		}
	} else {
		// 同步生成（向后兼容）
		zkProof, err = m.generateZKProof(ctx, executionResultHash, executionTrace)
		if err != nil {
			return nil, WrapZKProofGenerationFailedError("onnx_inference", err)
		}
	}

	if zkProof == nil {
		return nil, WrapZKProofEmptyError()
	}

	// 7. 生成状态ID
	stateID, err := m.generateStateID(ctx)
	if err != nil {
		return nil, WrapStateIDGenerationFailedError(err)
	}

	// 8. 构建完整的 pb.StateOutput（包含ZKProof）
	metadata := map[string]string{
		"execution_node": m.getNodeID(),
		"model_type":     "onnx",
	}

	// P0: 使用确定性执行时间（从上下文获取，必须已设置）
	var executionTimeStr string
	if executionStart := ctx.Value(ContextKeyExecutionStart); executionStart != nil {
		if startTime, ok := executionStart.(time.Time); ok {
			executionTimeStr = startTime.Format(time.RFC3339)
		}
	}
	// 如果上下文中没有，尝试从执行上下文获取确定性时间戳
	if executionTimeStr == "" {
		if execCtx, ok := executionContext.(interface{ GetDeterministicTimestamp() time.Time }); ok {
			executionTimeStr = execCtx.GetDeterministicTimestamp().Format(time.RFC3339)
		}
	}
	// 如果仍然没有，这是错误情况（不应该发生）
	if executionTimeStr == "" {
		return nil, fmt.Errorf("无法获取确定性执行时间：executionStartTime未正确设置")
	}
	metadata["execution_time"] = executionTimeStr

	// Phase 1: 将 CU 写入 metadata（后续在 TX 层构建 ExecutionProof 时读取并写入 ExecutionProof.context.metadata）
	metadata["compute_units"] = fmt.Sprintf("%.2f", computeUnits)
	
	// Phase 5: 预留多维资源使用字段（当前仅统计，不计费）
	metadata["storage_bytes"] = "0" // 存储使用（字节）- 未来扩展
	metadata["bandwidth_in_bytes"] = fmt.Sprintf("%d", inputSizeBytes)  // 输入带宽使用量
	metadata["bandwidth_out_bytes"] = fmt.Sprintf("%d", outputSizeBytes) // 输出带宽使用量

	// 直接构建protobuf定义的StateOutput（零转换）
	stateOutput := &pb.StateOutput{
		StateId:             stateID,
		StateVersion:        1,
		ZkProof:             zkProof, // ← 直接包含，必须非nil
		ExecutionResultHash: executionResultHash,
		ParentStateHash:     nil, // 初始状态无父状态，后续可通过状态链追溯
		Metadata:            metadata,
	}

	// 9. 构建ONNXExecutionResult
	returnTensors := make([][]float64, len(outputs))
	tensorOutputs := make([]ispcintf.ONNXTensorOutput, len(outputs))
	for i, out := range outputs {
		returnTensors[i] = out.Values
		tensorOutputs[i] = ispcintf.ONNXTensorOutput{
			Name:    out.Name,
			DType:   out.DType,
			Shape:   out.Shape,
			Layout:  out.Layout,
			Values:  out.Values,
			RawData: out.RawData,
		}
	}

	executionResult := &ispcintf.ONNXExecutionResult{
		ReturnTensors: returnTensors,
		TensorOutputs: tensorOutputs,
		StateOutput:   stateOutput, // 完整的pb.StateOutput
		ExecutionContext: map[string]interface{}{
			"execution_id":   executionID,
			"model_hash":     fmt.Sprintf("%x", modelHash),
			"execution_time": executionTimeStr, // P0: 使用确定性执行时间
			"compute_units":  computeUnits,     // Phase 1: CU 值
		},
	}

	// P0: 如果使用异步ZK证明生成，添加任务ID到执行上下文
	if zkProofTaskID != "" {
		executionResult.ExecutionContext["zk_proof_task_id"] = zkProofTaskID
		executionResult.ExecutionContext["zk_proof_status"] = "pending"
	}

	// Phase 3: 生成计费计划（如果计费编排器已初始化）
	// 注意：这里不指定 selectedToken，因为执行阶段无法知道用户选择的支付代币
	// 用户选择的支付代币应该在 API 层（CallAIModel）传入，并在费用预估时使用
	if m.billingOrchestrator != nil {
		billingPlan, err := m.billingOrchestrator.GenerateBillingPlan(ctx, modelHash, computeUnits, "")
		if err != nil {
			// 计费计划生成失败不影响执行结果，只记录警告
			m.logger.Warnf("生成计费计划失败: %v（执行结果仍有效）", err)
		} else {
			// 将计费计划添加到执行上下文中（供 TX Builder 使用）
			executionResult.ExecutionContext["billing_plan"] = map[string]interface{}{
				"resource_hash": fmt.Sprintf("%x", billingPlan.ResourceHash),
				"cu":            billingPlan.CU,
				"fee_amount":    billingPlan.FeeAmount.String(),
				"payment_token": billingPlan.PaymentToken,
				"owner_address": fmt.Sprintf("%x", billingPlan.OwnerAddress),
				"billing_mode":  billingPlan.BillingMode.String(),
			}
			m.logger.Debugf("Phase 3: 计费计划已生成: CU=%.2f, Fee=%s %s",
				billingPlan.CU, billingPlan.FeeAmount.String(), billingPlan.PaymentToken)
		}
	} else {
		m.logger.Debugf("计费编排器未初始化，跳过计费计划生成")
	}

	m.logger.Debugf("ONNX模型推理完成: executionID=%s, stateID=%x", executionID, stateID)

	// P0: 检查资源限制（执行结束后）
	if usage := executionContext.GetResourceUsage(); usage != nil && resourceLimits != nil {
		if err := m.checkResourceLimits(usage, resourceLimits); err != nil {
			return nil, err
		}
	}

	return executionResult, nil
}

// deriveContractAddress 根据合约内容哈希推导20字节合约地址
func (m *Manager) deriveContractAddress(contractHash []byte) ([]byte, error) {
	if len(contractHash) == 0 {
		return nil, WrapInvalidContractHashError(contractHash)
	}

	// 优先使用 hashManager 提供的算法，确保与系统一致
	if m.hashManager != nil {
		sha := m.hashManager.SHA256(contractHash)
		if len(sha) > 0 {
			addr := m.hashManager.RIPEMD160(sha)
			if len(addr) == 20 {
				return addr, nil
			}
			if len(addr) > 0 && m.logger != nil {
				m.logger.Warnf("hashManager.RIPEMD160 返回长度 %d，期望20字节，回退到内置算法", len(addr))
			}
		}
	}

	// 回退到内置的 Hash160 实现 (SHA256 → RIPEMD160)
	sha := sha256.Sum256(contractHash)
	r := ripemd160.New()
	if _, err := r.Write(sha[:]); err != nil {
		return nil, fmt.Errorf("计算合约地址失败: %w", err)
	}
	addr := r.Sum(nil)
	if len(addr) != 20 {
		return nil, fmt.Errorf("计算合约地址失败: 结果长度为 %d", len(addr))
	}
	return addr, nil
}

// ==================== 辅助方法 ====================
// （原 parseParams、serializeResult、serializeInferenceOutputs 已删除，因为现在直接使用强类型）
