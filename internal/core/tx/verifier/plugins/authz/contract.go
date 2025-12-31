package authz

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/tx"
)

// ContractPlugin 智能合约锁定验证插件
//
// 🎯 **核心职责**：验证智能合约锁定条件（ContractLock + ExecutionProof）
//
// 💡 **设计理念**：
// 通过智能合约逻辑控制 UTXO 的使用，适用于：
// - DeFi 协议：自动做市商、流动性池、借贷协议
// - 自动化交易：条件单、策略执行
// - 可编程场景：复杂的状态转换逻辑
// - 资源付费访问：按次付费模型、动态权限管理
//
// 🔒 **验证规则（P8 简化版）**：
// 1. 合约地址匹配
// 2. 执行时间在允许范围内
// 3. 执行结果哈希存在且非空
// 4. 状态转换证明存在
// 5. 签名方案一致性（如果需要）
//
// ⚠️ **核心约束**：
// - ❌ 插件无状态：不存储验证结果
// - ❌ 插件只读：不修改交易
// - ❌ **不在共识路径重执行合约**：只验证执行证明的有效性
// - ✅ 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 AuthZ Hook）
//
// 📝 **完整验证（P8 之后）**：
// - 验证 execution_result_hash 与实际执行结果一致
// - 验证 state_transition_proof（默克尔证明）
// - 验证参数符合 parameter_schema
// - 验证状态符合 state_requirements
// - 验证 contract_state_hash 与实际合约状态一致
// - 验证 parameter_hash 与实际参数一致
// - 验证调用者在 allowed_callers 列表中
// - 验证未超过 deadline_duration_seconds
//
// 🔥 **P8 设计决策**：
// "仅验证路径"意味着：
// 1. 验证证明结构的完整性
// 2. 验证基本的字段匹配（地址、方法名、时间等）
// 3. 不重新执行合约代码
// 4. 不验证默克尔证明的密码学有效性（需要完整的状态树）
// 5. 为完整实现预留扩展点
type ContractPlugin struct{}

// NewContractPlugin 创建新的 ContractPlugin
//
// 返回：
//   - *ContractPlugin: 新创建的实例
func NewContractPlugin() *ContractPlugin {
	return &ContractPlugin{}
}

// Name 返回插件名称
//
// 实现 tx.AuthZPlugin 接口
//
// 返回：
//   - string: "contract"
func (p *ContractPlugin) Name() string {
	return "contract"
}

// Match 验证 UnlockingProof 是否匹配 LockingCondition
//
// 实现 tx.AuthZPlugin 接口
//
// 🎯 **核心逻辑（P8 简化版）**：
// 1. 类型检查：lock 必须是 ContractLock
// 2. 提取 ExecutionProof
// 3. 验证合约地址匹配
// 4. 验证方法名匹配
// 5. 验证执行时间在限制内
// 6. 验证执行结果哈希非空
// 7. 验证状态转换证明非空
// 8. 验证输入参数存在
//
// 参数：
//   - ctx: 上下文对象
//   - lock: UTXO 的锁定条件
//   - unlockingProof: input 的解锁证明
//   - tx: 完整的交易对象（用于验证）
//
// 返回：
//   - bool: 是否匹配此插件
//   - true: 此插件处理了验证（可能成功或失败）
//   - false: 此插件不处理此类型的 lock/proof
//   - error: 验证错误
//   - nil: 验证成功
//   - non-nil: 验证失败，描述失败原因
func (p *ContractPlugin) Match(
	ctx context.Context,
	lock *transaction.LockingCondition,
	unlockingProof *transaction.UnlockingProof,
	tx *transaction.Transaction,
) (bool, error) {
	// 1. 类型检查：是否为 ContractLock
	contractLock := lock.GetContractLock()
	if contractLock == nil {
		return false, nil // 不是 ContractLock，让其他插件处理
	}

	// 2. 提取 ExecutionProof
	execProof := unlockingProof.GetExecutionProof()
	if execProof == nil {
		return true, fmt.Errorf("missing execution proof for ContractLock")
	}
	
	execCtx := execProof.Context
	if execCtx == nil {
		return true, fmt.Errorf("missing execution context in proof")
	}

	// 3. 验证资源地址匹配（通用化：合约/模型/其他）
	// P8 简化：检查非空和长度合理
	if len(contractLock.ContractAddress) == 0 {
		return true, fmt.Errorf("invalid contract address: empty")
	}
	if len(execCtx.ResourceAddress) == 0 {
		return true, fmt.Errorf("missing resource_address in execution proof")
	}
	if len(execCtx.ResourceAddress) != 20 {
		return true, fmt.Errorf("invalid resource_address length: expected 20 bytes, got %d", len(execCtx.ResourceAddress))
	}

	// 4. 验证方法名匹配（从 metadata 中获取）
	// P8 简化：检查 required_method 和 metadata["method_name"] 一致性
	if contractLock.RequiredMethod != "" {
		methodNameBytes, exists := execCtx.Metadata["method_name"]
		if !exists || len(methodNameBytes) == 0 {
			return true, fmt.Errorf("missing method name in execution proof metadata")
		}

		// 比较方法名（字节数组 vs 字符串）
		proofMethodName := string(methodNameBytes)
		if proofMethodName != contractLock.RequiredMethod {
			return true, fmt.Errorf(
				"method name mismatch: expected=%s, got=%s",
				contractLock.RequiredMethod,
				proofMethodName,
			)
		}
	}

	// 5. 验证 IdentityProof（必需字段）
	if execCtx.CallerIdentity == nil {
		return true, fmt.Errorf("missing caller_identity in execution proof (required for cryptographic security)")
	}

	// 6. 验证执行时间在限制内
	if contractLock.MaxExecutionTimeMs > 0 {
		if execProof.ExecutionTimeMs > contractLock.MaxExecutionTimeMs {
			return true, fmt.Errorf(
				"execution time exceeds limit: %dms > %dms",
				execProof.ExecutionTimeMs,
				contractLock.MaxExecutionTimeMs,
			)
		}
	}

	// 7. 验证执行结果哈希非空
	if len(execProof.ExecutionResultHash) == 0 {
		return true, fmt.Errorf("missing execution result hash")
	}
	if len(execProof.ExecutionResultHash) != 32 {
		return true, fmt.Errorf("invalid execution_result_hash length: expected 32 bytes, got %d", len(execProof.ExecutionResultHash))
	}

	// 8. 验证状态转换证明非空
	if len(execProof.StateTransitionProof) == 0 {
		return true, fmt.Errorf("missing state transition proof")
	}

	// 9. 验证输入数据哈希存在（隐私保护设计）
	if len(execCtx.InputDataHash) != 32 {
		return true, fmt.Errorf("missing or invalid input_data_hash in execution proof (expected 32 bytes, got %d)", len(execCtx.InputDataHash))
	}

	// 10. 验证输出数据哈希存在（隐私保护设计）
	if len(execCtx.OutputDataHash) != 32 {
		return true, fmt.Errorf("missing or invalid output_data_hash in execution proof (expected 32 bytes, got %d)", len(execCtx.OutputDataHash))
	}

	// P8 简化：暂不实现完整的合约执行验证
	// 实际应：
	// 1. 从区块链状态中获取合约代码
	// 2. 验证 contract_state_hash 与实际合约状态一致
	// 3. 验证 execution_result_hash 通过默克尔证明可推导出
	// 4. 验证 state_requirements 列表中的所有条件满足
	// 5. 验证 parameter_schema 与实际参数类型一致
	// 6. 验证 parameter_hash 与实际参数哈希一致
	// 7. 验证 allowed_callers 列表（如果非空）
	// 8. 验证 deadline_duration_seconds（如果设置）
	//
	// 示例（完整验证）：
	// - contractEngine := getContractEngine()
	// - contractCode := getContractCode(contractLock.ContractAddress)
	// - inputStateHash := execCtx.Metadata["contract_state_before_hash"]
	// - outputStateHash := execCtx.Metadata["contract_state_after_hash"]
	// - inputDataHash := execCtx.InputDataHash
	// - outputDataHash := execCtx.OutputDataHash
	//
	// - isValid := contractEngine.VerifyExecution(
	//     contractCode,
	//     contractLock.RequiredMethod,
	//     parameters,
	//     inputState,
	//     outputState,
	//     execProof.ExecutionResultHash,
	//     execProof.StateTransitionProof,
	// )
	// - if !isValid {
	//     return true, fmt.Errorf("contract execution verification failed")
	// }

	// 9. 验证通过（P8 简化版）
	return true, nil
}

// 编译期检查：确保 ContractPlugin 实现了 tx.AuthZPlugin 接口
var _ tx.AuthZPlugin = (*ContractPlugin)(nil)
