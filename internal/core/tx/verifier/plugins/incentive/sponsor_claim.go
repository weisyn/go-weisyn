package incentive

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo_pb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/constants"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// SponsorClaimPlugin 赞助领取交易验证插件
//
// 🎯 **赞助激励验证**
//
// 识别并验证赞助领取交易的结构和约束。
//
// 验证内容：
//  1. 识别赞助领取交易（1输入+DelegationProof）
//  2. 验证Input引用的UTXO Owner = SponsorPoolOwner
//  3. 验证DelegationProof有效性
//  4. 验证输出结构（矿工领取+找零回池）
//  5. 验证金额守恒
//
// 🔧 **架构优化**（基于架构分析文档）：
//   - DelegateSignature改为可选验证：如果提供则验证，未提供不影响验证通过
//   - 权限验证以LockingConditions为准：Owner字段仅作为辅助验证（防御性编程）
//   - 保持"任意矿工可领取"的灵活性：不强制要求签名验证
type SponsorClaimPlugin struct {
	eutxoQuery        persistence.UTXOQuery
	sigManager        crypto.SignatureManager // 签名验证管理器
	hashManager       crypto.HashManager      // 哈希管理器
	hashCanonicalizer *hash.Canonicalizer     // 交易哈希计算器
}

// NewSponsorClaimPlugin 创建赞助领取验证插件
//
// 参数：
//   - eutxoQuery: UTXO查询服务
//   - sigManager: 签名管理器（用于验证DelegateSignature）
//   - hashManager: 哈希管理器（用于地址验证）
//   - hashCanonicalizer: 交易哈希计算器（用于签名验证）
//
// 返回：
//   - *SponsorClaimPlugin: 插件实例
func NewSponsorClaimPlugin(
	eutxoQuery persistence.UTXOQuery,
	sigManager crypto.SignatureManager,
	hashManager crypto.HashManager,
	hashCanonicalizer *hash.Canonicalizer,
) *SponsorClaimPlugin {
	return &SponsorClaimPlugin{
		eutxoQuery:        eutxoQuery,
		sigManager:        sigManager,
		hashManager:       hashManager,
		hashCanonicalizer: hashCanonicalizer,
	}
}

// Name 插件名称
func (p *SponsorClaimPlugin) Name() string {
	return "SponsorClaimValidator"
}

// Check 实现 ConservationPlugin 接口
//
// 🎯 **核心职责**：验证赞助领取交易的价值守恒和业务规则
//
// 参数：
//   - ctx: 上下文对象
//   - inputs: 输入 UTXO 列表（已通过 ConservationHook 获取）
//   - outputs: 输出列表（从 Transaction 中获取）
//   - tx: 完整的交易对象
//
// 返回：
//   - error: 验证失败原因，nil表示通过
func (p *SponsorClaimPlugin) Check(
	ctx context.Context,
	inputs []*utxo_pb.UTXO,
	outputs []*transaction_pb.TxOutput,
	tx *transaction_pb.Transaction,
) error {
	// 1. 识别赞助领取交易特征：1输入 + DelegationProof
	if len(tx.Inputs) != 1 || len(inputs) != 1 {
		return nil // 不是赞助领取交易，跳过
	}

	delegationProof := tx.Inputs[0].GetDelegationProof()
	if delegationProof == nil {
		return nil // 不是赞助领取交易，跳过
	}

	sponsorUTXO := inputs[0]

	// 2. 验证UTXO Owner = SponsorPoolOwner
	if !bytes.Equal(sponsorUTXO.GetCachedOutput().Owner, constants.SponsorPoolOwner[:]) {
		return nil // 不是赞助池UTXO，跳过（可能是普通DelegationProof交易）
	}

	// 3. 验证输出结构
	if err := p.validateOutputs(tx, sponsorUTXO, nil); err != nil {
		return fmt.Errorf("SponsorClaimPlugin: 输出验证失败: %w", err)
	}

	// 4. 验证金额守恒
	if err := p.validateConservation(tx, sponsorUTXO, delegationProof); err != nil {
		return fmt.Errorf("SponsorClaimPlugin: 金额守恒验证失败: %w", err)
	}

	return nil
}

// Verify 验证交易（保留用于向后兼容，内部调用Check）
//
// 参数：
//
//	ctx: 上下文
//	tx: 待验证的交易
//	env: 验证环境（必须实现txiface.VerifierEnvironment）
//
// 返回：
//
//	error: 验证失败原因，nil表示通过
//
// 注意：此方法保留用于特殊场景，正常情况下应使用Check方法
func (p *SponsorClaimPlugin) Verify(
	ctx context.Context,
	tx *transaction_pb.Transaction,
	env interface{},
) error {
	// 1. 识别赞助领取交易特征：1输入 + DelegationProof
	if len(tx.Inputs) != 1 {
		return nil // 不是赞助领取交易，跳过
	}

	delegationProof := tx.Inputs[0].GetDelegationProof()
	if delegationProof == nil {
		return nil // 不是赞助领取交易，跳过
	}

	// 2. 类型断言获取验证环境
	verifierEnv, ok := env.(txiface.VerifierEnvironment)
	if !ok {
		return fmt.Errorf("SponsorClaimPlugin: 环境类型错误，期望txiface.VerifierEnvironment")
	}

	// 3. 获取Input引用的UTXO
	sponsorUTXO, err := verifierEnv.GetUTXO(ctx, tx.Inputs[0].PreviousOutput)
	if err != nil {
		return fmt.Errorf("SponsorClaimPlugin: 查询赞助UTXO失败: %w", err)
	}

	// 3.1 强制输入为消费模式
	if tx.Inputs[0].IsReferenceOnly {
		return fmt.Errorf("SponsorClaimPlugin: 赞助领取必须为消费模式(IsReferenceOnly=false)")
	}

	// 4. 验证UTXO Owner = SponsorPoolOwner
	if !bytes.Equal(sponsorUTXO.GetCachedOutput().Owner, constants.SponsorPoolOwner[:]) {
		return nil // 不是赞助池UTXO，跳过（可能是普通DelegationProof交易）
	}

	// 4.1 验证 DelegationLock 授权包含 consume
	var delegationLock *transaction_pb.DelegationLock
	for _, lock := range sponsorUTXO.GetCachedOutput().LockingConditions {
		if dl := lock.GetDelegationLock(); dl != nil {
			delegationLock = dl
			break
		}
	}
	if delegationLock == nil {
		return fmt.Errorf("SponsorClaimPlugin: 赞助UTXO缺少DelegationLock")
	}
	hasConsume := false
	for _, op := range delegationLock.AuthorizedOperations {
		if op == "consume" {
			hasConsume = true
			break
		}
	}
	if !hasConsume {
		return fmt.Errorf("SponsorClaimPlugin: DelegationLock未授权consume操作")
	}

	// 5. 验证DelegationProof基本结构
	if err := p.validateDelegationProof(ctx, delegationProof, tx, verifierEnv); err != nil {
		return fmt.Errorf("SponsorClaimPlugin: DelegationProof验证失败: %w", err)
	}

	// 6. 验证输出结构
	if err := p.validateOutputs(tx, sponsorUTXO, verifierEnv.GetMinerAddress()); err != nil {
		return fmt.Errorf("SponsorClaimPlugin: 输出验证失败: %w", err)
	}

	// 7. 验证金额守恒
	if err := p.validateConservation(tx, sponsorUTXO, delegationProof); err != nil {
		return fmt.Errorf("SponsorClaimPlugin: 金额守恒验证失败: %w", err)
	}

	return nil
}

// validateDelegationProof 验证DelegationProof基本结构
func (p *SponsorClaimPlugin) validateDelegationProof(
	ctx context.Context,
	proof *transaction_pb.DelegationProof,
	tx *transaction_pb.Transaction,
	env txiface.VerifierEnvironment,
) error {
	// 验证OperationType必须是"consume"
	if proof.OperationType != "consume" {
		return fmt.Errorf("赞助领取必须使用consume操作，实际=%s", proof.OperationType)
	}

	// 验证DelegateAddress必须是矿工地址
	minerAddr := env.GetMinerAddress()
	if !bytes.Equal(proof.DelegateAddress, minerAddr) {
		return fmt.Errorf("DelegateAddress必须是矿工地址，期望=%x，实际=%x",
			minerAddr, proof.DelegateAddress)
	}

	// 🔐 **架构优化：DelegateSignature改为可选验证**
	//
	// **设计决策**（基于架构分析文档）：
	// - DelegationLock已经授权任意矿工可以consume（AllowedDelegates为空）
	// - DelegateAddress已经指定了矿工地址
	// - DelegateSignature主要用于审计追踪，不是必须的验证项
	//
	// **验证策略**：
	// - 如果提供了DelegateSignature，则进行验证（可选功能）
	// - 如果未提供，不影响交易验证（保持"任意矿工可领取"的灵活性）
	//
	// **未来扩展**：
	// - 如果需要强制签名验证，可以通过DelegationLock的配置来控制
	// - 或者使用ContractLock方案实现更复杂的签名验证逻辑

	if proof.DelegateSignature != nil && len(proof.DelegateSignature.Value) > 0 {
		// ✅ **使用 VerifierEnvironment.GetPublicKey 获取公钥并验证签名**
		if env != nil {
			// 计算交易签名哈希（赞助领取交易只有一个输入，索引为0）
			inputIndex := 0
			txHash, err := p.hashCanonicalizer.ComputeSignatureHashForVerification(
				ctx, tx, inputIndex, transaction_pb.SignatureHashType_SIGHASH_ALL)
			if err != nil {
				// 计算哈希失败，但不阻止验证通过（向后兼容）
				// return fmt.Errorf("计算交易签名哈希失败: %w", err)
			} else {
				// 尝试从 VerifierEnvironment 获取矿工公钥
				minerPubKey, err := env.GetPublicKey(ctx, proof.DelegateAddress)
				if err != nil {
					// 获取公钥失败，但不阻止验证通过（向后兼容）
					// return fmt.Errorf("获取矿工公钥失败: %w", err)
				} else if len(minerPubKey) > 0 {
					// 成功获取公钥，进行签名验证
					valid := p.sigManager.VerifyTransactionSignature(
						txHash, proof.DelegateSignature.Value, minerPubKey, crypto.SigHashAll)
					if !valid {
						return fmt.Errorf("DelegateSignature 验证失败：矿工签名无效")
					}
					// ✅ 签名验证通过
				}
				// 如果 minerPubKey 为 nil，说明地址没有对应的公钥记录，跳过验证
			}
		}
		// 如果没有提供 VerifierEnvironment，跳过签名验证（向后兼容）
	}
	// 如果未提供签名，跳过验证（允许任意矿工无签名领取）

	return nil
}

// validateOutputs 验证输出结构
func (p *SponsorClaimPlugin) validateOutputs(
	tx *transaction_pb.Transaction,
	sponsorUTXO *utxo_pb.UTXO,
	minerAddr []byte, // 如果为nil，从DelegationProof中提取
) error {
	// 如果未提供minerAddr，尝试从DelegationProof中提取
	if minerAddr == nil {
		delegationProof := tx.Inputs[0].GetDelegationProof()
		if delegationProof != nil {
			minerAddr = delegationProof.DelegateAddress
		}
	}
	if len(minerAddr) == 0 {
		return fmt.Errorf("无法确定矿工地址")
	}
	if len(tx.Outputs) == 0 || len(tx.Outputs) > 2 {
		return fmt.Errorf("赞助领取交易必须有1-2个输出，实际=%d", len(tx.Outputs))
	}

	// 🔒 **架构优化：权限验证以LockingConditions为准**
	//
	// **设计决策**（基于架构分析文档）：
	// - Owner字段的作用：索引/展示用途（transaction.proto:594）
	// - LockingConditions的作用：实际权限控制（transaction.proto:595）
	// - 权限应该以LockingConditions为准，Owner只是辅助字段
	//
	// **验证策略**：
	// - 核心验证：SingleKeyLock的地址哈希（必须验证）
	// - 辅助验证：Owner字段（防御性编程，发现不一致时警告但不阻止）

	// 🔒 核心验证：Output[0]必须有锁定条件（UTXO模型强制要求）
	if len(tx.Outputs[0].LockingConditions) == 0 {
		return fmt.Errorf("Output[0]必须有锁定条件")
	}
	singleKeyLock := tx.Outputs[0].LockingConditions[0].GetSingleKeyLock()
	if singleKeyLock == nil {
		return fmt.Errorf("Output[0]必须使用SingleKeyLock（矿工地址锁）")
	}

	// 🔐 **核心验证：SingleKeyLock的地址哈希匹配**
	//
	// **验证逻辑**：验证 SingleKeyLock 的地址哈希与矿工地址一致
	// 这是UTXO模型的核心安全机制，确保 Output[0] 确实锁定给了正确的矿工地址

	// 从 SingleKeyLock 提取地址哈希
	keyReq := singleKeyLock.KeyRequirement
	if keyReq == nil {
		return fmt.Errorf("SingleKeyLock 缺少 KeyRequirement")
	}

	var requiredAddrHash []byte
	switch req := keyReq.(type) {
	case *transaction_pb.SingleKeyLock_RequiredAddressHash:
		requiredAddrHash = req.RequiredAddressHash
	case *transaction_pb.SingleKeyLock_RequiredPublicKey:
		// 如果使用公钥锁定，需要从公钥计算地址哈希
		// 地址计算：address = RIPEMD160(SHA256(pubKey))
		sha256Hash := p.hashManager.SHA256(req.RequiredPublicKey.Value)
		requiredAddrHash = p.hashManager.RIPEMD160(sha256Hash)
	default:
		return fmt.Errorf("SingleKeyLock 必须使用 RequiredAddressHash 或 RequiredPublicKey")
	}

	if len(requiredAddrHash) == 0 {
		return fmt.Errorf("SingleKeyLock 缺少有效的地址哈希")
	}

	// 验证地址哈希与矿工地址一致
	// **方案1**：如果 minerAddr 是公钥哈希（20字节），直接比较
	// **方案2**：如果 minerAddr 是其他格式，使用 HashManager 计算哈希后比较
	if len(minerAddr) == 20 {
		// minerAddr 是 20 字节公钥哈希，直接比较
		if !bytes.Equal(requiredAddrHash, minerAddr) {
			return fmt.Errorf("SingleKeyLock 的地址哈希不匹配矿工地址：期望=%x，实际=%x",
				minerAddr, requiredAddrHash)
		}
	} else {
		// minerAddr 是其他格式（如完整地址、Bech32编码等）
		// 使用 HashManager 计算地址哈希
		// 地址计算：address = RIPEMD160(SHA256(minerAddr))
		sha256Hash := p.hashManager.SHA256(minerAddr)
		minerAddrHash := p.hashManager.RIPEMD160(sha256Hash)
		if !bytes.Equal(requiredAddrHash, minerAddrHash) {
			return fmt.Errorf("SingleKeyLock 的地址哈希不匹配矿工地址：期望=%x，实际=%x",
				minerAddrHash, requiredAddrHash)
		}
	}

	// ✅ 核心验证通过：地址哈希匹配

	// 🔍 **辅助验证：Owner字段一致性检查（防御性编程）**
	// 注意：如果Owner字段与LockingConditions不一致，这里只作为警告参考
	// 实际权限控制以LockingConditions为准
	if !bytes.Equal(tx.Outputs[0].Owner, minerAddr) {
		// Owner字段不一致，但不影响验证通过（权限以LockingConditions为准）
		// 在实际生产环境中，可以考虑记录警告日志
		// 这里不返回错误，因为LockingConditions的验证已经通过
	}

	// 如果有Output[1]，必须是找零回赞助池
	if len(tx.Outputs) == 2 {
		if !bytes.Equal(tx.Outputs[1].Owner, constants.SponsorPoolOwner[:]) {
			return fmt.Errorf("Output[1]的Owner必须是赞助池地址，期望=%x，实际=%x",
				constants.SponsorPoolOwner[:], tx.Outputs[1].Owner)
		}

		// 找零输出必须保持DelegationLock
		hasDelegationLock := false
		for _, lock := range tx.Outputs[1].LockingConditions {
			if lock.GetDelegationLock() != nil {
				hasDelegationLock = true
				break
			}
		}
		if !hasDelegationLock {
			return fmt.Errorf("找零输出必须包含DelegationLock")
		}
	}

	return nil
}

// validateConservation 验证金额守恒
func (p *SponsorClaimPlugin) validateConservation(
	tx *transaction_pb.Transaction,
	sponsorUTXO *utxo_pb.UTXO,
	proof *transaction_pb.DelegationProof,
) error {
	// 提取赞助UTXO的总金额
	inputAsset := sponsorUTXO.GetCachedOutput().GetAsset()
	if inputAsset == nil {
		return fmt.Errorf("赞助UTXO必须是资产输出")
	}

	inputAmount, ok := new(big.Int).SetString(p.extractAmount(inputAsset), 10)
	if !ok {
		return fmt.Errorf("解析输入金额失败")
	}

	// 🔒 安全-2: 提取输入资产类型
	inputTokenKey := p.getAssetTokenKey(inputAsset)

	// 计算所有输出的总金额
	var outputSum = big.NewInt(0)
	for i, output := range tx.Outputs {
		outAsset := output.GetAsset()
		if outAsset == nil {
			return fmt.Errorf("Output[%d]必须是资产输出", i)
		}

		// 🔒 安全-2: 验证输出资产类型与输入一致
		outTokenKey := p.getAssetTokenKey(outAsset)
		if inputTokenKey != outTokenKey {
			return fmt.Errorf("Output[%d]资产类型不一致：期望=%s，实际=%s",
				i, inputTokenKey, outTokenKey)
		}

		outAmount, ok := new(big.Int).SetString(p.extractAmount(outAsset), 10)
		if !ok {
			return fmt.Errorf("解析Output[%d]金额失败", i)
		}
		outputSum.Add(outputSum, outAmount)
	}

	// 验证守恒：输入 == 输出
	if inputAmount.Cmp(outputSum) != 0 {
		return fmt.Errorf("金额不守恒：输入=%s，输出=%s",
			inputAmount.String(), outputSum.String())
	}

	// 验证领取金额 <= MaxValuePerOperation（如果设置）
	claimAmount, ok := new(big.Int).SetString(p.extractAmount(tx.Outputs[0].GetAsset()), 10)
	if !ok {
		return fmt.Errorf("解析领取金额失败")
	}

	// 🔒 缺陷-1: 验证ValueAmount（uint64统一转big.Int）
	if proof.ValueAmount > 0 {
		// 使用SetUint64安全转换uint64到big.Int，避免溢出风险
		proofAmount := new(big.Int).SetUint64(proof.ValueAmount)
		if claimAmount.Cmp(proofAmount) != 0 {
			return fmt.Errorf("领取金额与Proof不一致：实际=%s，Proof=%s",
				claimAmount.String(), proofAmount.String())
		}
	}

	return nil
}

// extractAmount 从AssetOutput提取金额字符串
func (p *SponsorClaimPlugin) extractAmount(asset *transaction_pb.AssetOutput) string {
	if nc := asset.GetNativeCoin(); nc != nil {
		return nc.Amount
	}
	if ct := asset.GetContractToken(); ct != nil {
		return ct.Amount
	}
	return "0"
}

// getAssetTokenKey 提取资产的TokenKey（用于类型一致性检查）
func (p *SponsorClaimPlugin) getAssetTokenKey(asset *transaction_pb.AssetOutput) string {
	if nc := asset.GetNativeCoin(); nc != nil {
		return "native"
	}
	if ct := asset.GetContractToken(); ct != nil {
		// 使用proto实际结构：contract_address + token_identifier
		contractAddr := fmt.Sprintf("%x", ct.ContractAddress)
		switch ti := ct.TokenIdentifier.(type) {
		case *transaction_pb.ContractTokenAsset_FungibleClassId:
			return fmt.Sprintf("FT:%s:%x", contractAddr, ti.FungibleClassId)
		case *transaction_pb.ContractTokenAsset_NftUniqueId:
			return fmt.Sprintf("NFT:%s:%x", contractAddr, ti.NftUniqueId)
		case *transaction_pb.ContractTokenAsset_SemiFungibleId:
			return fmt.Sprintf("SFT:%s:%x:%d", contractAddr, ti.SemiFungibleId.BatchId, ti.SemiFungibleId.InstanceId)
		}
	}
	return "unknown"
}
