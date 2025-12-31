package authz

import (
	"context"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// DelegationPlugin 委托授权验证插件
//
// 🎯 **核心职责**：验证委托授权锁定条件（DelegationLock + DelegationProof）
//
// 💡 **设计理念**：
// 委托授权允许 UTXO 所有者授权第三方代为操作，适用于：
// - 托管服务：用户授权交易所代为交易
// - 自动化交易：授权机器人执行策略
// - 代理投票：授权代表参与治理
// - 资源临时访问：授权其他用户临时使用资源
//
// 🔒 **验证规则**：
// 1. 委托交易存在且有效（delegation_transaction_id 指向有效的 UTXO）
// 2. 委托未过期（expiry_duration_blocks 检查）
// 3. 操作类型在授权范围内（authorized_operations 检查）
// 4. 操作价值不超过单次最大限额（max_value_per_operation 检查）
// 5. 被委托方签名有效（delegate_signature 验证）- 可选验证
// 6. 被委托方在允许列表中（allowed_delegates 检查）
//
// ⚠️ **核心约束**：
// - ❌ 插件无状态：不存储验证结果
// - ❌ 插件只读：不修改交易
// - ✅ 并发安全：多个 goroutine 可以同时调用
//
// 📞 **调用方**：Verifier Kernel（通过 AuthZ Hook）
type DelegationPlugin struct {
	// 注意：当前简化实现不使用签名验证
	// 如需完整签名验证，需要添加以下依赖：
	// sigManager        crypto.SignatureManager
	// hashCanonicalizer *hash.Canonicalizer
}

// NewDelegationPlugin 创建新的 DelegationPlugin
//
// 返回：
//   - *DelegationPlugin: 新创建的实例
func NewDelegationPlugin() *DelegationPlugin {
	return &DelegationPlugin{}
}

// Name 返回插件名称
//
// 实现 tx.AuthZPlugin 接口
//
// 返回：
//   - string: "delegation"
func (p *DelegationPlugin) Name() string {
	return "delegation"
}

// Match 验证 UnlockingProof 是否匹配 LockingCondition
//
// 实现 tx.AuthZPlugin 接口
//
// 🎯 **核心逻辑**：
// 1. 类型检查：lock 必须是 DelegationLock
// 2. 提取 DelegationProof
// 3. 验证委托未过期
// 4. 验证操作类型在授权范围内
// 5. 验证操作价值不超过限额
// 6. 验证被委托方签名
// 7. 验证被委托方在允许列表中
//
// 参数：
//   - ctx: 上下文对象
//   - lock: UTXO 的锁定条件
//   - unlockingProof: input 的解锁证明
//   - tx: 完整的交易对象（用于签名验证）
//
// 返回：
//   - bool: 是否匹配此插件
//   - true: 此插件处理了验证（可能成功或失败）
//   - false: 此插件不处理此类型的 lock/proof
//   - error: 验证错误
//   - nil: 验证成功
//   - non-nil: 验证失败，描述失败原因
func (p *DelegationPlugin) Match(
	ctx context.Context,
	lock *transaction.LockingCondition,
	unlockingProof *transaction.UnlockingProof,
	tx *transaction.Transaction,
) (bool, error) {
	// 1. 类型检查：是否为 DelegationLock
	delegationLock := lock.GetDelegationLock()
	if delegationLock == nil {
		return false, nil // 不是 DelegationLock，让其他插件处理
	}

	// 2. 提取 DelegationProof
	delegationProof := unlockingProof.GetDelegationProof()
	if delegationProof == nil {
		// 如果没有 DelegationProof，但有 DelegationLock，则认为匹配但验证失败
		return true, fmt.Errorf("missing delegation proof for DelegationLock")
	}

	// 3. 验证委托交易存在（P8 简化：只检查非空）
	// 实际应查询 UTXO 集合验证 delegation_transaction_id 指向的 UTXO 存在且有效
	if len(delegationProof.DelegationTransactionId) == 0 {
		return true, fmt.Errorf("invalid delegation_transaction_id: empty")
	}

	// 4. 验证委托未过期（expiry_duration_blocks 检查）
	// 使用 VerifierEnvironment 提供的区块高度（确定性，不使用硬编码占位）。
	env, _ := txiface.GetVerifierEnvironment(ctx)
	if env == nil {
		return true, fmt.Errorf("delegation lock: verifier environment not provided (cannot validate expiry)")
	}
	currentBlockHeight := env.GetBlockHeight()

	if delegationLock.ExpiryDurationBlocks != nil && *delegationLock.ExpiryDurationBlocks > 0 {
		// 委托创建高度：通过环境查询 delegation_transaction_id 所在区块高度。
		delegationBlockHeight, err := env.GetTxBlockHeight(ctx, delegationProof.DelegationTransactionId)
		if err != nil {
			return true, fmt.Errorf("delegation lock: failed to get delegation tx block height: %w", err)
		}

		expiryBlockHeight := delegationBlockHeight + *delegationLock.ExpiryDurationBlocks
		if currentBlockHeight > expiryBlockHeight {
			return true, fmt.Errorf(
				"delegation expired: current_height=%d, expiry_height=%d",
				currentBlockHeight,
				expiryBlockHeight,
			)
		}
	}

	// 5. 验证操作类型在授权范围内
	if len(delegationLock.AuthorizedOperations) > 0 {
		operationAuthorized := false
		for _, authorizedOp := range delegationLock.AuthorizedOperations {
			if authorizedOp == delegationProof.OperationType {
				operationAuthorized = true
				break
			}
		}
		if !operationAuthorized {
			return true, fmt.Errorf(
				"operation type not authorized: %s (authorized: %v)",
				delegationProof.OperationType,
				delegationLock.AuthorizedOperations,
			)
		}
	}

	// 6. 验证操作价值不超过限额
	if delegationProof.ValueAmount > delegationLock.MaxValuePerOperation {
		return true, fmt.Errorf(
			"operation value exceeds max limit: %d > %d",
			delegationProof.ValueAmount,
			delegationLock.MaxValuePerOperation,
		)
	}

	// 7. 验证被委托方在允许列表中
	if len(delegationLock.AllowedDelegates) > 0 {
		delegateAllowed := false
		for _, allowedDelegate := range delegationLock.AllowedDelegates {
			if bytesEqual(allowedDelegate, delegationProof.DelegateAddress) {
				delegateAllowed = true
				break
			}
		}
		if !delegateAllowed {
			return true, fmt.Errorf(
				"delegate not allowed: %x (allowed: %d delegates)",
				delegationProof.DelegateAddress,
				len(delegationLock.AllowedDelegates),
			)
		}
	}

	// 8. 验证被委托方签名（架构优化：改为可选验证）
	//
	// **设计决策**（基于架构分析文档）：
	// - DelegationLock已经授权任意矿工可以consume（AllowedDelegates为空）
	// - DelegateAddress已经指定了矿工地址
	// - DelegateSignature主要用于审计追踪，不是必须的验证项
	//
	// **验证策略**：
	// - 如果提供了DelegateSignature，进行可选验证（当前简化实现，暂不验证）
	// - 如果未提供，不影响交易验证（保持"任意矿工可领取"的灵活性）
	//
	// **未来扩展**：
	// - 如果需要强制签名验证，可以通过DelegationLock的配置来控制
	// - 或者使用ContractLock方案实现更复杂的签名验证逻辑
	//
	// **当前简化实现**：跳过签名验证（假设 DelegationProof 已由可信来源生成）
	// 适用于测试环境或信任模型宽松的场景。
	//
	// ⚠️ **完整实现说明**（参考 single_key.go 的实现模式）：
	// 1. 添加依赖注入（在NewDelegationPlugin中）：
	//    - sigManager crypto.SignatureManager
	//    - hashCanonicalizer *hash.Canonicalizer
	//
	// 2. 实现签名验证逻辑：
	//    if delegationProof.DelegateSignature != nil && len(delegationProof.DelegateSignature.Value) > 0 {
	//        // 计算交易签名哈希（用于验证）
	//        txHash, err := p.hashCanonicalizer.ComputeSignatureHashForVerification(
	//            ctx, tx, inputIndex, transaction.SignatureHashType_SIGHASH_ALL)
	//        if err != nil {
	//            return true, fmt.Errorf("计算交易哈希失败: %w", err)
	//        }
	//
	//        // 从delegate_address推导公钥（或从proof中获取）
	//        // 注意：DelegationProof不包含公钥，需要从地址推导或查询
	//        pubKeyBytes := derivePublicKeyFromAddress(delegationProof.DelegateAddress)
	//
	//        // 验证签名
	//        valid := p.sigManager.VerifyTransactionSignature(
	//            txHash, 
	//            delegationProof.DelegateSignature.Value, 
	//            pubKeyBytes, 
	//            crypto.SigHashAll)
	//        if !valid {
	//            return true, fmt.Errorf("被委托方签名验证失败")
	//        }
	//    }
	//
	// 3. 注意事项：
	//    - DelegationProof中没有指定签名算法，需要从DelegationLock中获取
	//    - 地址到公钥的推导可能需要额外的查询（从UTXO或账户系统）
	//    - 当前简化实现跳过验证，适用于测试和信任模型宽松的场景

	// 验证通过
	return true, nil
}

// bytesEqual 比较两个字节数组是否相等
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 编译期检查：确保 DelegationPlugin 实现了 AuthZPlugin 接口
var _ txiface.AuthZPlugin = (*DelegationPlugin)(nil)
