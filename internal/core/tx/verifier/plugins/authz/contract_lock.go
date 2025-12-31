package authz

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sort"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// ================================================================================================
// 🔒 ContractLock 插件 - 合约锁定条件验证
// ================================================================================================
//
// 🎯 核心原则：验证 ExecutionProof 的格式和约束，不重新执行合约
//
// ⚠️ 架构边界（避免循环依赖）：
// - TX 层【仅】验证 ExecutionProof 的有效性和约束条件
// - TX 层【不】重新执行合约逻辑（避免 TX → ISPC → TX 循环依赖）
// - 合约执行由 ISPC 层在交易构建时完成
// - TX 层信任 execution_result_hash 和 state_transition_proof
//
// 🔍 验证内容：
// 1. resource_address 一致性检查（通用化：合约/模型/其他）
// 2. execution_time_ms 限制检查
// 3. allowed_callers 白名单验证
// 4. execution_result_hash 格式验证（32字节 SHA-256）
// 5. state_transition_proof 存在性验证
// 6. IdentityProof 验证（必需字段，密码学安全保证）
// 7. input_data_hash 格式验证（隐私保护设计）
// 8. parameter_hash 一致性验证（如果设置，使用 input_data_hash）
// 9. output_data_hash 格式验证（隐私保护设计）
// 10. deadline_duration 过期检查（如果设置）
// 11. ContractTokenAsset.contract_address 匹配验证（铸造场景安全验证）
//
// 💡 设计哲学：
// - 确定性验证：基于密码学证明，而非重新计算
// - 性能优化：合约只执行一次（构建时），验证时不重新执行
// - 职责分离：ISPC 层执行，TX 层验证
//
// 📚 参考文档：
// - _docs/architecture/TX_STATE_MACHINE_ARCHITECTURE.md - ContractLock 验证流程
// ================================================================================================

// ================================================================================================
// 🎯 错误定义（ContractLock 专用）
// ================================================================================================

var (
	// ErrInvalidLockingCondition 锁定条件无效
	ErrInvalidLockingCondition = fmt.Errorf("invalid locking condition")

	// ErrInvalidUnlockingProof 解锁证明无效
	ErrInvalidUnlockingProof = fmt.Errorf("invalid unlocking proof")

	// ErrContractAddressMismatch 合约地址不匹配
	ErrContractAddressMismatch = fmt.Errorf("contract address mismatch")

	// ErrExecutionTimeout 执行超时
	ErrExecutionTimeout = fmt.Errorf("execution timeout")

	// ErrCallerNotAllowed 调用者不在白名单中
	ErrCallerNotAllowed = fmt.Errorf("caller not allowed")

	// ErrInvalidExecutionResultHash 执行结果哈希无效
	ErrInvalidExecutionResultHash = fmt.Errorf("invalid execution result hash")

	// ErrMissingStateTransitionProof 缺少状态转换证明
	ErrMissingStateTransitionProof = fmt.Errorf("missing state transition proof")

	// ErrParameterHashMismatch 参数哈希不匹配
	ErrParameterHashMismatch = fmt.Errorf("parameter hash mismatch")

	// ErrMissingTransactionHash 缺少交易哈希（已废弃，transaction_hash 已从 ExecutionProof 移除）
	// ⚠️ 注意：transaction_hash 应该从 Transaction 本身获取，不应该在 ExecutionProof 中
	ErrMissingTransactionHash = fmt.Errorf("missing transaction hash")
)

// ContractLockPlugin 实现合约锁定条件验证
type ContractLockPlugin struct {
	hashManager      crypto.HashManager      // 哈希管理器（用于 parameter_hash 验证）
	signatureManager crypto.SignatureManager // 签名管理器（用于 IdentityProof 验证）
	addressManager   crypto.AddressManager   // 地址管理器（用于 public_key -> address 推导与比对）
}

// NewContractLockPlugin 创建 ContractLock 插件实例
func NewContractLockPlugin(
	hashManager crypto.HashManager,
	signatureManager crypto.SignatureManager,
	addressManager crypto.AddressManager,
) *ContractLockPlugin {
	return &ContractLockPlugin{
		hashManager:      hashManager,
		signatureManager: signatureManager,
		addressManager:   addressManager,
	}
}

// Name 返回插件名称
func (p *ContractLockPlugin) Name() string {
	return "authz.contract_lock"
}

// Match 验证 ExecutionProof 是否匹配 ContractLock
//
// 实现 tx.AuthZPlugin 接口
//
// 🔍 验证流程：
// 1. 提取 lock 和 ExecutionProof
// 2. 验证 resource_address 一致性（通用化：合约/模型/其他）
// 3. 验证 execution_time_ms 限制
// 4. 验证 allowed_callers 白名单（如果设置）
// 5. 验证 execution_result_hash 格式
// 6. 验证 state_transition_proof 存在性
// 7. 验证 IdentityProof（必需字段，密码学安全保证）
// 8. 验证 input_data_hash 格式（隐私保护设计）
// 9. 验证 parameter_hash 一致性（如果设置，使用 input_data_hash）
// 10. 验证 output_data_hash 格式（隐私保护设计）
// 11. 验证 deadline_duration 过期（如果设置）
// 12. 验证 ContractTokenAsset.contract_address 匹配（铸造场景安全验证）
//
// 参数：
//   - ctx: 上下文对象
//   - lock: UTXO 的锁定条件
//   - unlockingProof: input 的解锁证明
//   - tx: 完整的交易对象
//
// 返回：
//   - bool: 是否匹配此插件
//     • true: 此插件处理了验证（可能成功或失败）
//     • false: 此插件不处理此类型的 lock/proof
//   - error: 验证错误
//     • nil: 验证成功（仅当第一个返回值为 true 时）
//     • non-nil: 验证失败，描述失败原因
func (p *ContractLockPlugin) Match(
	ctx context.Context,
	lockingCondition *transaction.LockingCondition,
	unlockingProof *transaction.UnlockingProof,
	tx *transaction.Transaction,
) (bool, error) {
	// 1. 类型检查：是否为 ContractLock
	lock := lockingCondition.GetContractLock()
	if lock == nil {
		return false, nil // 不是 ContractLock，让其他插件处理
	}

	// 2. 提取 ExecutionProof
	execProof := unlockingProof.GetExecutionProof()
	if execProof == nil {
		return true, fmt.Errorf("%w: ExecutionProof is nil", ErrInvalidUnlockingProof)
	}
	
	execCtx := execProof.Context
	if execCtx == nil {
		return true, fmt.Errorf("%w: ExecutionProof.Context is nil", ErrInvalidUnlockingProof)
	}

	// 3. 验证 resource_address 一致性（通用化：合约/模型/其他）
	// ✅ **更新**：使用 resource_address 替代 contract_address
	// 验证：lock.ContractAddress == execCtx.ResourceAddress
	if len(lock.ContractAddress) > 0 {
		if len(execCtx.ResourceAddress) == 0 {
			return true, fmt.Errorf(
				"%w: resource_address missing in ExecutionProof.Context",
				ErrContractAddressMismatch,
			)
		}
		if !bytes.Equal(lock.ContractAddress, execCtx.ResourceAddress) {
			return true, fmt.Errorf(
				"%w: expected %x, got %x",
				ErrContractAddressMismatch,
				lock.ContractAddress,
				execCtx.ResourceAddress,
			)
		}
	}

	// 4. 验证 execution_time_ms 限制
	if lock.MaxExecutionTimeMs > 0 && execProof.ExecutionTimeMs > lock.MaxExecutionTimeMs {
		return true, fmt.Errorf(
			"%w: execution time %d ms exceeds limit %d ms",
			ErrExecutionTimeout,
			execProof.ExecutionTimeMs,
			lock.MaxExecutionTimeMs,
		)
	}

	// 5. 验证 allowed_callers 白名单（如果设置）
	// ✅ **更新**：从 IdentityProof 中获取 caller_address
	if len(lock.AllowedCallers) > 0 {
		callerAddress := execCtx.CallerIdentity.GetCallerAddress()
		if len(callerAddress) == 0 {
			return true, fmt.Errorf(
				"%w: caller_address missing in IdentityProof",
				ErrCallerNotAllowed,
			)
		}
		if !containsCaller(lock.AllowedCallers, callerAddress) {
			return true, fmt.Errorf(
				"%w: caller %x not in allowed list",
				ErrCallerNotAllowed,
				callerAddress,
			)
		}
	}

	// 6. 验证 execution_result_hash 格式（32字节 SHA-256）
	if len(execProof.ExecutionResultHash) != 32 {
		return true, fmt.Errorf(
			"%w: invalid execution_result_hash length: got %d, want 32",
			ErrInvalidExecutionResultHash,
			len(execProof.ExecutionResultHash),
		)
	}

	// 7. 验证 state_transition_proof 存在性
	if len(execProof.StateTransitionProof) == 0 {
		return true, fmt.Errorf("%w: state_transition_proof is empty", ErrMissingStateTransitionProof)
	}

	// 8. 验证 IdentityProof（必需字段）
	// ✅ **更新**：IdentityProof 现在是必需字段，不再是可选的
	if execCtx.CallerIdentity == nil {
		return true, fmt.Errorf(
			"%w: caller_identity is required (cryptographic security guarantee)",
			ErrInvalidUnlockingProof,
		)
	}
	if err := p.verifyIdentityProof(ctx, execCtx.CallerIdentity, execCtx); err != nil {
		return true, fmt.Errorf("identity proof verification failed: %w", err)
	}

	// 9. 验证 input_data_hash 格式（隐私保护设计）
	// ✅ **更新**：使用 input_data_hash 替代 InputParameters
	if len(execCtx.InputDataHash) != 32 {
		return true, fmt.Errorf(
			"%w: invalid input_data_hash length: got %d, want 32",
			ErrParameterHashMismatch,
			len(execCtx.InputDataHash),
		)
	}

	// 9.1 验证 parameter_hash 一致性（如果设置，使用 input_data_hash）
	if len(lock.ParameterHash) > 0 {
		// ✅ **更新**：使用 input_data_hash 替代原始参数
		if !bytes.Equal(lock.ParameterHash, execCtx.InputDataHash) {
			return true, fmt.Errorf(
				"%w: parameter_hash mismatch: expected %x, got %x",
				ErrParameterHashMismatch,
				lock.ParameterHash,
				execCtx.InputDataHash,
			)
		}
	}

	// 9.2 验证 output_data_hash 格式（隐私保护设计）
	if len(execCtx.OutputDataHash) != 32 {
		return true, fmt.Errorf(
			"%w: invalid output_data_hash length: got %d, want 32",
			ErrInvalidExecutionResultHash,
			len(execCtx.OutputDataHash),
		)
	}

	// 10. 验证 deadline_duration 过期（如果设置）
	if lock.DeadlineDurationSeconds != nil && *lock.DeadlineDurationSeconds > 0 {
		// deadline 语义：以 Transaction.creation_timestamp 为起点，deadline_duration_seconds 为窗口长度；
		// 当前区块时间（VerifierEnvironment.GetBlockTime）必须落在窗口内。
		//
		// 说明：这里不使用墙钟，完全由验证环境提供“确定性区块时间”。
		env, _ := txiface.GetVerifierEnvironment(ctx)
		if env == nil {
			return true, fmt.Errorf("deadline 验证需要 VerifierEnvironment，但未提供")
		}
		if tx == nil || tx.CreationTimestamp == 0 {
			return true, fmt.Errorf("deadline 验证需要 Transaction.creation_timestamp，但为空")
		}
		now := env.GetBlockTime()
		expiry := tx.CreationTimestamp + uint64(*lock.DeadlineDurationSeconds)
		if now > expiry {
			return true, fmt.Errorf("deadline 已过期: now=%d expiry=%d creation_ts=%d window=%ds",
				now, expiry, tx.CreationTimestamp, *lock.DeadlineDurationSeconds)
		}
	}

	// 11. ✅ 验证输出中的 ContractTokenAsset.contract_address 是否匹配执行资源地址
	// 🎯 **目的**：防止合约A创建合约B的代币（铸造场景安全验证）
	// 如果交易输出包含 ContractTokenAsset，必须验证其 contract_address 匹配 execCtx.ResourceAddress
	if len(execCtx.ResourceAddress) > 0 {
		for _, output := range tx.Outputs {
			if asset := output.GetAsset(); asset != nil {
				if contractToken := asset.GetContractToken(); contractToken != nil {
					// 验证 contract_address 是否匹配
					if len(contractToken.ContractAddress) == 0 {
						return true, fmt.Errorf(
							"%w: ContractTokenAsset.contract_address is empty in output",
							ErrContractAddressMismatch,
						)
					}
					if !bytes.Equal(contractToken.ContractAddress, execCtx.ResourceAddress) {
						return true, fmt.Errorf(
							"%w: ContractTokenAsset.contract_address mismatch in output: expected %x, got %x",
							ErrContractAddressMismatch,
							execCtx.ResourceAddress,
							contractToken.ContractAddress,
						)
					}
				}
			}
		}
	}

	// ✅ 所有验证通过
	return true, nil
}

// 编译期检查：确保 ContractLockPlugin 实现了 tx.AuthZPlugin 接口
var _ txiface.AuthZPlugin = (*ContractLockPlugin)(nil)

// ================================================================================================
// 🔧 辅助函数
// ================================================================================================

// containsCaller 检查调用者是否在允许列表中
func containsCaller(allowedCallers []string, callerAddress []byte) bool {
	callerStr := string(callerAddress)
	for _, allowed := range allowedCallers {
		if allowed == callerStr {
			return true
		}
	}
	return false
}

// verifyIdentityProof 验证 IdentityProof
//
// 🎯 **验证流程**：
// 1. 验证基础字段完整性（public_key、caller_address、signature、context_hash、nonce）
// 2. ⚠️ **安全修复**：先验证 context_hash 是否匹配实际的 ExecutionContext（完整性验证）
//    - 这是关键的安全检查：签名是对 context_hash 的签名，所以必须先验证 context_hash 的正确性
// 3. 验证 signature 是否匹配 context_hash（使用 public_key）
// 4. 验证 caller_address 是否从 public_key 推导（确保一致性）
// 5. 验证 nonce 是否未被使用（防重放攻击，需要查询nonce数据库）
// 6. 验证 timestamp 是否在有效期内（时效性验证）
//
// 参数：
//   - ctx: 上下文对象
//   - identityProof: 身份证明
//   - executionContext: 执行上下文
//
// 返回：
//   - error: 验证失败时的错误
func (p *ContractLockPlugin) verifyIdentityProof(
	ctx context.Context,
	identityProof *transaction.IdentityProof,
	executionContext *transaction.ExecutionProof_ExecutionContext,
) error {
	// 1. 验证基础字段完整性
	if len(identityProof.PublicKey) == 0 {
		return fmt.Errorf("identity proof: public_key is empty")
	}
	if len(identityProof.CallerAddress) == 0 {
		return fmt.Errorf("identity proof: caller_address is empty")
	}
	if len(identityProof.CallerAddress) != 20 {
		return fmt.Errorf("identity proof: invalid caller_address length: got %d, want 20", len(identityProof.CallerAddress))
	}
	if len(identityProof.Signature) == 0 {
		return fmt.Errorf("identity proof: signature is empty")
	}
	if len(identityProof.ContextHash) != 32 {
		return fmt.Errorf("identity proof: invalid context_hash length: got %d, want 32", len(identityProof.ContextHash))
	}
	if len(identityProof.Nonce) != 32 {
		return fmt.Errorf("identity proof: invalid nonce length: got %d, want 32", len(identityProof.Nonce))
	}

	// 2. ⚠️ **安全修复**：先验证 context_hash 是否匹配实际的 ExecutionContext
	// 这是关键的安全检查：签名是对 context_hash 的签名，所以必须先验证 context_hash 的正确性
	// 如果 context_hash 不匹配，签名验证也会失败，但先验证 context_hash 可以提供更清晰的错误信息
	contextHash := p.computeExecutionContextHash(executionContext)
	if !bytes.Equal(contextHash, identityProof.ContextHash) {
		return fmt.Errorf("identity proof: context_hash mismatch: expected %x, got %x",
			contextHash, identityProof.ContextHash)
	}

	// 3. 验证 signature 是否匹配 context_hash
	// ⚠️ **安全修复**：在验证 context_hash 之后验证签名，确保逻辑正确
	if p.signatureManager == nil {
		return fmt.Errorf("identity proof: signature manager not available")
	}

	// 使用签名管理器验证签名
	// 注意：这里需要根据算法类型选择合适的验证方法
	// 目前简化处理，假设使用 ECDSA_SECP256K1
	valid := p.signatureManager.Verify(
		identityProof.ContextHash,
		identityProof.Signature,
		identityProof.PublicKey,
	)
	if !valid {
		return fmt.Errorf("identity proof: signature verification failed")
	}

	// 4. 验证 caller_address 是否从 public_key 推导
	if p.addressManager == nil {
		return fmt.Errorf("identity proof: address manager not available")
	}
	addrStr, err := p.addressManager.PublicKeyToAddress(identityProof.PublicKey)
	if err != nil {
		return fmt.Errorf("identity proof: derive address from public_key failed: %w", err)
	}
	derivedAddrBytes, err := p.addressManager.AddressToBytes(addrStr)
	if err != nil {
		return fmt.Errorf("identity proof: convert derived address to bytes failed: %w", err)
	}
	if len(derivedAddrBytes) != 20 {
		return fmt.Errorf("identity proof: derived address length invalid: got %d, want 20", len(derivedAddrBytes))
	}
	if !bytes.Equal(derivedAddrBytes, identityProof.CallerAddress) {
		return fmt.Errorf("identity proof: caller_address mismatch (derived=%x got=%x)", derivedAddrBytes, identityProof.CallerAddress)
	}

	// 5. 验证 nonce 是否未被使用
	// 说明：
	// - 交易级别的防重放由 Condition/NoncePlugin 对 tx.nonce + 账户 nonce 完成（确定性、可验证）。
	// - IdentityProof.nonce 当前仅做格式要求（32字节），不在此处做“是否已使用”的全局查询，
	//   以避免在 AuthZ 阶段引入额外写依赖/外部状态。

	// 6. 验证 timestamp 是否在有效期内（5分钟内）
	// 规则：
	// - 使用 VerifierEnvironment 的区块时间（确定性）做窗口校验
	// - 允许小幅漂移（防止不同节点打包/预验证的微小时间差）
	env, _ := txiface.GetVerifierEnvironment(ctx)
	if env != nil {
		const maxSkewSec = uint64(300)   // 5分钟窗口
		const maxFutureSec = uint64(60)  // 最多超前 60s
		now := env.GetBlockTime()
		if identityProof.Timestamp == 0 {
			return fmt.Errorf("identity proof: timestamp is empty")
		}
		if identityProof.Timestamp > now+maxFutureSec {
			return fmt.Errorf("identity proof: timestamp too far in future: ts=%d now=%d", identityProof.Timestamp, now)
		}
		if identityProof.Timestamp+maxSkewSec < now {
			return fmt.Errorf("identity proof: timestamp expired: ts=%d now=%d window=%ds", identityProof.Timestamp, now, maxSkewSec)
		}
	}

	return nil
}

// computeExecutionContextHash 计算 ExecutionContext 的哈希
//
// 🎯 **计算内容**：包含所有非敏感字段的哈希
// - input_data_hash
// - output_data_hash
// - resource_address
// - execution_type
// - metadata（不包括敏感原始数据）
//
// ⚠️ **边界原则**：不包含 value_sent、transaction_hash 和 timestamp
//
// 参数：
//   - executionContext: 执行上下文
//
// 返回：
//   - []byte: 32字节SHA-256哈希
func (p *ContractLockPlugin) computeExecutionContextHash(
	executionContext *transaction.ExecutionProof_ExecutionContext,
) []byte {
	// 构建用于哈希的数据
	var buf bytes.Buffer

	// 添加所有非敏感字段（按照设计文档的要求）
	// ⚠️ **安全修复**：只添加32字节的哈希，确保一致性
	if len(executionContext.InputDataHash) == 32 {
		buf.Write(executionContext.InputDataHash)
	}
	// ⚠️ **安全修复**：如果 InputDataHash 不是32字节，不添加（避免哈希不一致）
	
	if len(executionContext.OutputDataHash) == 32 {
		buf.Write(executionContext.OutputDataHash)
	}
	// ⚠️ **安全修复**：如果 OutputDataHash 不是32字节，不添加（避免哈希不一致）
	
	// ⚠️ **安全修复**：验证 ResourceAddress 长度，确保哈希一致性
	if len(executionContext.ResourceAddress) != 20 {
		// 如果长度不正确，使用空字节数组填充（防御性编程）
		// 注意：验证逻辑中已经检查了长度，这里只是防御性检查
		emptyAddr := make([]byte, 20)
		buf.Write(emptyAddr)
	} else {
		buf.Write(executionContext.ResourceAddress)
	}

	// 添加 execution_type（4字节）
	execTypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(execTypeBytes, uint32(executionContext.ExecutionType))
	buf.Write(execTypeBytes)

	// ⚠️ **边界原则**：不包含 value_sent、transaction_hash 和 timestamp
	// - value_sent：应该从 Transaction 的 inputs/outputs 中计算
	// - transaction_hash：应该从 Transaction 本身获取
	// - timestamp：应该使用 Transaction.creation_timestamp
	// - IdentityProof.timestamp：保留，用于 IdentityProof 的时效性验证（独立于 TX timestamp）

	// 添加 metadata（排序后添加，确保确定性）
	if len(executionContext.Metadata) > 0 {
		keys := make([]string, 0, len(executionContext.Metadata))
		for k := range executionContext.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			buf.WriteString(k)
			buf.Write(executionContext.Metadata[k])
		}
	}

	// 计算SHA-256哈希
	// ⚠️ **注意**：使用 hashManager.SHA256，与 execution_helpers.go 中的 sha256.Sum256 应该产生相同结果
	// hashManager.SHA256 的实现也是使用 sha256.Sum256，所以是一致的
	return p.hashManager.SHA256(buf.Bytes())
}
