package builder

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	consensuscfg "github.com/weisyn/v1/internal/config/consensus"
	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo_pb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/constants"
	configiface "github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/utils/timeutil"
	"google.golang.org/protobuf/proto"
)

// IncentiveBuilder 激励交易构建器
//
// 🎯 **零增发激励机制核心组件**
//
// 构建内容:
//  1. Coinbase交易（零增发：仅手续费）
//  2. 赞助领取交易（0-N笔）
//
// 赞助领取流程:
//  1. 扫描赞助池UTXO
//  2. 过滤有效赞助（检查DelegationLock、有效期、白名单）
//  3. 构建领取交易（Input: DelegationProof, Output: 矿工+找零）
//  4. 限制数量（policy.MaxPerBlock）
//
// 🔧 **架构优化**（基于架构分析文档）：
//
//   - DelegateSignature改为可选生成：如果提供了Signer则生成，未提供则不生成
//   - 保持"任意矿工可领取"的灵活性：不强制要求签名
//   - 签名主要用于审计追踪，不是必须的验证项
type IncentiveBuilder struct {
	feeManager txiface.FeeManager
	eutxoQuery persistence.UTXOQuery
	config     configiface.Provider
	signer     txiface.Signer // 可选签名器（nil时不生成签名，用于审计追踪）
	logger     Logger         // 日志记录器（可选）
}

// Logger 日志接口（简化版，避免引入完整的日志框架依赖）
type Logger interface {
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// NewIncentiveBuilder 创建激励交易构建器
//
// 参数:
//
//	feeManager: 费用管理器
//	utxoManager: UTXO管理器
//	config: 配置提供者
//	signer: 签名器（可选，nil时不生成签名）
//
// 设计说明:
//   - signer参数可选，支持向后兼容
//   - 如果提供了signer，将生成真实的DelegationProof签名（用于审计追踪）
//   - 如果signer为nil，不生成签名（DelegateSignature保持为nil），验证端会接受
//   - 签名主要用于审计追踪，不是必须的验证项
func NewIncentiveBuilder(
	feeManager txiface.FeeManager,
	eutxoQuery persistence.UTXOQuery,
	config configiface.Provider,
	signer txiface.Signer,
) *IncentiveBuilder {
	if feeManager == nil {
		panic("feeManager不能为nil")
	}
	if eutxoQuery == nil {
		panic("eutxoQuery不能为nil")
	}
	if config == nil {
		panic("config不能为nil")
	}
	// signer可以为nil（向后兼容）
	return &IncentiveBuilder{
		feeManager: feeManager,
		eutxoQuery: eutxoQuery,
		config:     config,
		signer:     signer,
	}
}

// 确保实现接口
var _ txiface.IncentiveTxBuilder = (*IncentiveBuilder)(nil)

// BuildIncentiveTransactions 实现 txiface.IncentiveTxBuilder
//
// 参数:
//
//	ctx: 上下文对象
//	candidateTxs: 候选交易列表（用于计算手续费）
//	minerAddr: 矿工地址（20字节）
//	chainID: 链ID
//	blockHeight: 当前区块高度（用于检查赞助有效期）
//
// 返回:
//
//	[]*Transaction: 激励交易列表（Coinbase + 赞助领取）
//	error: 构建错误
func (b *IncentiveBuilder) BuildIncentiveTransactions(
	ctx context.Context,
	candidateTxs []*transaction_pb.Transaction,
	minerAddr []byte,
	chainID []byte,
	blockHeight uint64,
) ([]*transaction_pb.Transaction, error) {
	var result []*transaction_pb.Transaction

	// 1. 构建Coinbase（零增发）
	coinbase, err := b.buildCoinbase(ctx, candidateTxs, minerAddr, chainID)
	if err != nil {
		return nil, fmt.Errorf("构建Coinbase失败: %w", err)
	}
	result = append(result, coinbase)

	// 2. 构建赞助领取交易（可选）
	sponsorCfg := b.getSponsorIncentiveConfig()
	if sponsorCfg != nil && sponsorCfg.Enabled {
		sponsorTxs, err := b.buildSponsorClaimTransactions(ctx, minerAddr, chainID, blockHeight, sponsorCfg)
		if err != nil {
			// 赞助领取失败不应阻塞区块生成，记录警告后继续
			if b.logger != nil {
				b.logger.Warnf("构建赞助领取交易失败: %v", err)
			} else {
				fmt.Printf("WARN: 构建赞助领取交易失败: %v\n", err)
			}
		} else {
			result = append(result, sponsorTxs...)
		}
	}

	return result, nil
}

// buildCoinbase 构建Coinbase交易
func (b *IncentiveBuilder) buildCoinbase(
	ctx context.Context,
	candidateTxs []*transaction_pb.Transaction,
	minerAddr []byte,
	chainID []byte,
) (*transaction_pb.Transaction, error) {
	// 计算所有交易的费用
	var allFees []*txiface.AggregatedFees
	for _, tx := range candidateTxs {
		fee, err := b.feeManager.CalculateTransactionFee(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("计算交易费用失败: %w", err)
		}
		allFees = append(allFees, fee)
	}

	// 聚合费用
	aggregated := b.feeManager.AggregateFees(allFees)

	// 构建Coinbase
	return b.feeManager.BuildCoinbase(aggregated, minerAddr, chainID)
}

// buildSponsorClaimTransactions 构建赞助领取交易列表
func (b *IncentiveBuilder) buildSponsorClaimTransactions(
	ctx context.Context,
	minerAddr []byte,
	chainID []byte,
	blockHeight uint64,
	policy *consensuscfg.SponsorIncentiveConfig,
) ([]*transaction_pb.Transaction, error) {
	// 1. 扫描赞助池UTXO
	sponsorUTXOs, err := b.eutxoQuery.GetSponsorPoolUTXOs(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("扫描赞助池失败: %w", err)
	}

	if len(sponsorUTXOs) == 0 {
		// 没有可用赞助，正常情况
		return nil, nil
	}

	// 2. 过滤有效赞助
	validSponsors := b.filterValidSponsors(sponsorUTXOs, blockHeight, policy)

	if len(validSponsors) == 0 {
		return nil, nil
	}

	// 3. 限制数量
	maxCount := int(policy.MaxPerBlock)
	if maxCount > 0 && len(validSponsors) > maxCount {
		validSponsors = validSponsors[:maxCount]
	}

	// 4. 构建领取交易
	var claimTxs []*transaction_pb.Transaction
	for _, sponsor := range validSponsors {
		claimTx, err := b.buildSingleSponsorClaimTx(ctx, sponsor, minerAddr, chainID, policy)
		if err != nil {
			// 单个赞助构建失败，记录警告后继续
			fmt.Printf("WARN: 构建赞助领取交易失败 [%x:%d]: %v\n",
				sponsor.Outpoint.TxId, sponsor.Outpoint.OutputIndex, err)
			continue
		}
		claimTxs = append(claimTxs, claimTx)
	}

	return claimTxs, nil
}

// filterValidSponsors 过滤有效的赞助UTXO
//
// 过滤条件:
//  1. 必须有DelegationLock
//  2. AuthorizedOperations包含"consume"
//  3. AllowedDelegates为空（任意矿工可领取）
//  4. 未过期: currentHeight <= creationHeight + expiryDuration
//  5. Token在白名单中（policy.AcceptedTokens）
//  6. 金额 >= 最低金额（policy.MinAmountPerSponsor）
func (b *IncentiveBuilder) filterValidSponsors(
	sponsors []*utxo_pb.UTXO,
	currentHeight uint64,
	policy *consensuscfg.SponsorIncentiveConfig,
) []*utxo_pb.UTXO {
	var valid []*utxo_pb.UTXO

	for _, sponsor := range sponsors {
		// 检查是否有 CachedOutput
		output := sponsor.GetCachedOutput()
		if output == nil || output.GetAsset() == nil {
			continue
		}

		// 检查 DelegationLock
		delegationLock := b.extractDelegationLock(output)
		if delegationLock == nil {
			continue
		}

		// 检查授权操作包含 "consume"
		if !b.hasOperation(delegationLock.AuthorizedOperations, "consume") {
			continue
		}

		// 检查 AllowedDelegates 为空（任意矿工）
		if len(delegationLock.AllowedDelegates) > 0 {
			continue
		}

		// 检查未过期
		if delegationLock.ExpiryDurationBlocks != nil && *delegationLock.ExpiryDurationBlocks > 0 {
			expiryHeight := sponsor.BlockHeight + *delegationLock.ExpiryDurationBlocks
			if currentHeight > expiryHeight {
				continue
			}
		}

		// 检查Token白名单并获取MinAmount（中优先级-2）
		tokenKey := b.extractTokenKey(output.GetAsset())
		minAmount, accepted := b.getTokenMinAmount(tokenKey, policy.AcceptedTokens)
		if !accepted {
			continue
		}

		// 检查金额 >= MinAmount（中优先级-2）
		if minAmount > 0 {
			amount := b.extractAmount(output.GetAsset())
			if amount.Cmp(big.NewInt(int64(minAmount))) < 0 {
				continue // 金额低于最低要求
			}
		}

		valid = append(valid, sponsor)
	}

	return valid
}

// buildSingleSponsorClaimTx 构建单个赞助领取交易
func (b *IncentiveBuilder) buildSingleSponsorClaimTx(
	ctx context.Context,
	sponsor *utxo_pb.UTXO,
	minerAddr []byte,
	chainID []byte,
	policy *consensuscfg.SponsorIncentiveConfig,
) (*transaction_pb.Transaction, error) {
	output := sponsor.GetCachedOutput()
	asset := output.GetAsset()
	delegationLock := b.extractDelegationLock(output)

	// 计算领取金额（不超过DelegationLock限制和Policy限制）
	totalAmount := b.extractAmount(asset)
	maxPerOperation := big.NewInt(int64(delegationLock.MaxValuePerOperation))
	policyMax := big.NewInt(int64(policy.MaxAmountPerSponsor))

	claimAmount := new(big.Int).Set(totalAmount)
	if maxPerOperation.Sign() > 0 && claimAmount.Cmp(maxPerOperation) > 0 {
		claimAmount.Set(maxPerOperation)
	}
	if policyMax.Sign() > 0 && claimAmount.Cmp(policyMax) > 0 {
		claimAmount.Set(policyMax)
	}

	// 计算找零
	changeAmount := new(big.Int).Sub(totalAmount, claimAmount)

	// 构建输出
	outputs := []*transaction_pb.TxOutput{
		// 输出1: 矿工领取
		{
			Owner: minerAddr,
			LockingConditions: []*transaction_pb.LockingCondition{
				{
					Condition: &transaction_pb.LockingCondition_SingleKeyLock{
						SingleKeyLock: &transaction_pb.SingleKeyLock{
							KeyRequirement: &transaction_pb.SingleKeyLock_RequiredAddressHash{
								RequiredAddressHash: minerAddr,
							},
							RequiredAlgorithm: transaction_pb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
							SighashType:       transaction_pb.SignatureHashType_SIGHASH_ALL,
						},
					},
				},
			},
			OutputContent: &transaction_pb.TxOutput_Asset{
				Asset: b.cloneAssetWithAmount(asset, claimAmount),
			},
		},
	}

	// 如果有找零，创建找零输出（返回赞助池）
	if changeAmount.Sign() > 0 {
		// 复制原有的DelegationLock
		clonedLock := proto.Clone(delegationLock).(*transaction_pb.DelegationLock)
		outputs = append(outputs, &transaction_pb.TxOutput{
			Owner: constants.SponsorPoolOwner[:],
			LockingConditions: []*transaction_pb.LockingCondition{
				{
					Condition: &transaction_pb.LockingCondition_DelegationLock{
						DelegationLock: clonedLock,
					},
				},
			},
			OutputContent: &transaction_pb.TxOutput_Asset{
				Asset: b.cloneAssetWithAmount(asset, changeAmount),
			},
		})
	}

	// 构建 DelegationProof
	// 🔒 缺陷-1: 检查big.Int是否超过uint64范围
	var valueAmount uint64
	if claimAmount.IsUint64() {
		valueAmount = claimAmount.Uint64()
	} else {
		// 超过uint64最大值，返回错误（不应构建这样的交易）
		// 💡 **架构改进建议**：当前使用uint64存储金额，精度有限（最多~184.4亿 WES，即2^64-1 BaseUnit）。
		// 未来如需支持更大金额或更高精度，需要进行以下改动：
		// 1. 修改 pb/blockchain/block/transaction/value.proto 中的 ValueAmount.amount 字段
		//    从 uint64 改为 string 类型
		// 2. 重新生成 protobuf 代码（make proto 或 buf generate）
		// 3. 修改所有使用 ValueAmount 的代码，使用 big.Int 进行解析和计算
		// 4. 更新验证插件中的金额计算逻辑（conservation插件）
		return nil, fmt.Errorf("领取金额超过uint64最大值: %s", claimAmount.String())
	}

	delegationProof := &transaction_pb.DelegationProof{
		DelegationTransactionId: sponsor.Outpoint.TxId,
		DelegationOutputIndex:   sponsor.Outpoint.OutputIndex,
		OperationType:           "consume",
		ValueAmount:             valueAmount,
		DelegateAddress:         minerAddr,
	}

	// 🔐 **架构优化：DelegateSignature改为可选生成**
	//
	// **设计决策**（基于架构分析文档）：
	// - DelegationLock已经授权任意矿工可以consume（AllowedDelegates为空）
	// - DelegateAddress已经指定了矿工地址
	// - DelegateSignature主要用于审计追踪，不是必须的验证项
	//
	// **生成策略**：
	// - 如果提供了Signer，生成真实签名（可选功能）
	// - 如果未提供Signer，不生成签名（nil），验证端会接受
	// - 保持"任意矿工可领取"的灵活性
	//
	// **向后兼容**：
	// - 旧的占位符签名逻辑已移除，不再生成占位符
	// - 验证端已支持签名可选，不会因缺少签名而失败

	if b.signer != nil {
		// 提供了Signer，生成可选签名（用于审计追踪）
		signature, err := b.signDelegationProof(ctx, delegationProof, sponsor, minerAddr, claimAmount, changeAmount)
		if err != nil {
			// 签名生成失败不应该阻止交易构建，记录警告后继续
			if b.logger != nil {
				b.logger.Warnf("生成DelegateSignature失败（不影响交易构建）: %v", err)
			}
			// 不设置DelegateSignature，保持为nil
		} else {
			delegationProof.DelegateSignature = signature
		}
	}
	// 如果未提供Signer，DelegateSignature保持为nil（验证端会接受）

	// 构建交易
	tx := &transaction_pb.Transaction{
		Version: 1,
		Inputs: []*transaction_pb.TxInput{
			{
				PreviousOutput:  sponsor.Outpoint,
				IsReferenceOnly: false, // 消费模式
				UnlockingProof: &transaction_pb.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		Outputs:           outputs,
		Nonce:             uint64(time.Now().UnixNano()),
		CreationTimestamp: uint64(timeutil.NowUnix()),
		ChainId:           chainID,
	}

	return tx, nil
}

// 辅助方法

func (b *IncentiveBuilder) getSponsorIncentiveConfig() *consensuscfg.SponsorIncentiveConfig {
	consensusCfg := b.config.GetConsensus()
	if consensusCfg == nil {
		return nil
	}
	return &consensusCfg.Miner.SponsorIncentive
}

func (b *IncentiveBuilder) extractDelegationLock(output *transaction_pb.TxOutput) *transaction_pb.DelegationLock {
	for _, lock := range output.LockingConditions {
		if dl := lock.GetDelegationLock(); dl != nil {
			return dl
		}
	}
	return nil
}

// GetSponsorUTXOHelper 获取赞助UTXO辅助工具
//
// **用途**：提供赞助UTXO的元数据提取和生命周期管理功能
func (b *IncentiveBuilder) GetSponsorUTXOHelper() *SponsorUTXOHelper {
	return NewSponsorUTXOHelper(b.eutxoQuery)
}

func (b *IncentiveBuilder) hasOperation(operations []string, target string) bool {
	for _, op := range operations {
		if op == target {
			return true
		}
	}
	return false
}

func (b *IncentiveBuilder) extractTokenKey(asset *transaction_pb.AssetOutput) txiface.TokenKey {
	if asset.GetNativeCoin() != nil {
		return txiface.TokenKey("native")
	}
	if ct := asset.GetContractToken(); ct != nil {
		if fungibleClassId := ct.GetFungibleClassId(); fungibleClassId != nil {
			return txiface.TokenKey(fmt.Sprintf("contract:%x:%x", ct.ContractAddress, fungibleClassId))
		}
		if nftUniqueId := ct.GetNftUniqueId(); nftUniqueId != nil {
			return txiface.TokenKey(fmt.Sprintf("contract:%x:nft:%x", ct.ContractAddress, nftUniqueId))
		}
		if sfId := ct.GetSemiFungibleId(); sfId != nil {
			return txiface.TokenKey(fmt.Sprintf("contract:%x:sft:%x:%x", ct.ContractAddress, sfId.BatchId, sfId.InstanceId))
		}
	}
	return txiface.TokenKey("unknown")
}

func (b *IncentiveBuilder) extractAmount(asset *transaction_pb.AssetOutput) *big.Int {
	var amountStr string
	if nc := asset.GetNativeCoin(); nc != nil {
		amountStr = nc.Amount
	} else if ct := asset.GetContractToken(); ct != nil {
		amountStr = ct.Amount
	}
	amount, _ := new(big.Int).SetString(amountStr, 10)
	return amount
}

func (b *IncentiveBuilder) isTokenAcceptedInPolicy(tokenKey txiface.TokenKey, acceptedTokens []consensuscfg.TokenFilterConfig) bool {
	if len(acceptedTokens) == 0 {
		// 空白名单表示接受所有Token
		return true
	}
	tokenStr := string(tokenKey)
	for _, tokenFilter := range acceptedTokens {
		if tokenFilter.AssetID == tokenStr {
			return true
		}
	}
	return false
}

// getTokenMinAmount 获取Token的最低金额要求（中优先级-2）
//
// 参数:
//
//	tokenKey: Token标识
//	acceptedTokens: 白名单配置
//
// 返回:
//
//	minAmount: 最低金额要求（0表示无要求）
//	accepted: Token是否在白名单中
func (b *IncentiveBuilder) getTokenMinAmount(tokenKey txiface.TokenKey, acceptedTokens []consensuscfg.TokenFilterConfig) (uint64, bool) {
	if len(acceptedTokens) == 0 {
		// 空白名单表示接受所有Token，无最低金额要求
		return 0, true
	}
	tokenStr := string(tokenKey)
	for _, tokenFilter := range acceptedTokens {
		if tokenFilter.AssetID == tokenStr {
			return tokenFilter.MinAmount, true
		}
	}
	return 0, false // Token不在白名单中
}

func (b *IncentiveBuilder) cloneAssetWithAmount(original *transaction_pb.AssetOutput, newAmount *big.Int) *transaction_pb.AssetOutput {
	if nc := original.GetNativeCoin(); nc != nil {
		return &transaction_pb.AssetOutput{
			AssetContent: &transaction_pb.AssetOutput_NativeCoin{
				NativeCoin: &transaction_pb.NativeCoinAsset{
					Amount: newAmount.String(),
				},
			},
		}
	}
	if ct := original.GetContractToken(); ct != nil {
		cloned := proto.Clone(ct).(*transaction_pb.ContractTokenAsset)
		cloned.Amount = newAmount.String()
		return &transaction_pb.AssetOutput{
			AssetContent: &transaction_pb.AssetOutput_ContractToken{
				ContractToken: cloned,
			},
		}
	}
	return nil
}

// signDelegationProof 为DelegationProof生成可选签名
//
// 🔐 **架构优化：签名改为可选生成**
//
// **设计决策**（基于架构分析文档）：
// - 签名主要用于审计追踪，不是必须的验证项
// - 保持"任意矿工可领取"的灵活性
//
// **签名内容**：
//
//	对 DelegationProof 的核心字段进行哈希，然后使用矿工私钥签名。
//	这可用于审计追踪，证明矿工确实领取了赞助。
//
// **注意**：
//
//	此方法仅在Signer不为nil时被调用，如果Signer为nil，则不会生成签名。
//
// 参数：
//
//	ctx: 上下文
//	proof: DelegationProof对象（待签名）
//	sponsor: 赞助UTXO
//	minerAddr: 矿工地址
//	claimAmount: 领取金额
//	changeAmount: 找零金额
//
// 返回：
//
//	*SignatureData: 签名数据（如果生成成功）
//	error: 签名错误
func (b *IncentiveBuilder) signDelegationProof(
	ctx context.Context,
	proof *transaction_pb.DelegationProof,
	sponsor *utxo_pb.UTXO,
	minerAddr []byte,
	claimAmount *big.Int,
	changeAmount *big.Int,
) (*transaction_pb.SignatureData, error) {
	// 构建待签名的数据（DelegationProof的规范序列化）
	signData, err := b.buildDelegationProofSignData(proof, sponsor, minerAddr, claimAmount, changeAmount)
	if err != nil {
		return nil, fmt.Errorf("构建签名数据失败: %w", err)
	}

	// 🔐 **P0-3修复：使用真实的密码学签名**
	//
	// **实现说明**：
	// - 使用Signer接口的SignBytes方法对数据进行签名
	// - 如果签名失败，返回错误（不应该继续使用占位符）
	//
	// **注意**：signData应该是已经规范化处理的待签名数据（通常是哈希值）
	signature, err := b.signer.SignBytes(ctx, signData)
	if err != nil {
		return nil, fmt.Errorf("签名DelegationProof失败: %w", err)
	}

	return &transaction_pb.SignatureData{
		Value: signature,
	}, nil
}

// buildDelegationProofSignData 构建DelegationProof的签名数据
//
// 签名数据包括：
//   - DelegationTransactionId
//   - DelegationOutputIndex
//   - OperationType
//   - ValueAmount
//   - DelegateAddress
//   - 赞助UTXO的OutPoint
//   - 领取金额和找零金额
//
// 返回：
//
//	[]byte: 规范化的签名数据（哈希输入）
//	error: 构建错误
func (b *IncentiveBuilder) buildDelegationProofSignData(
	proof *transaction_pb.DelegationProof,
	sponsor *utxo_pb.UTXO,
	minerAddr []byte,
	claimAmount *big.Int,
	changeAmount *big.Int,
) ([]byte, error) {
	// 使用简单的规范序列化（实际生产中应使用更严格的序列化）
	data := []byte{}

	// 添加各个字段（使用固定宽度编码以避免截断）
	data = append(data, proof.DelegationTransactionId...)
	// 🔒 精度-2: 8字节大端编码输出索引（uint32 → uint64）
	idx := make([]byte, 8)
	binary.BigEndian.PutUint64(idx, uint64(proof.DelegationOutputIndex))
	data = append(data, idx...)
	data = append(data, []byte(proof.OperationType)...)
	data = append(data, []byte(fmt.Sprintf("%d", proof.ValueAmount))...)
	data = append(data, minerAddr...)

	// 添加赞助UTXO信息
	if sponsor.Outpoint != nil {
		data = append(data, sponsor.Outpoint.TxId...)
		data = append(data, byte(sponsor.Outpoint.OutputIndex))
	}

	// 添加金额信息
	data = append(data, []byte(claimAmount.String())...)
	data = append(data, []byte(changeAmount.String())...)

	return data, nil
}
