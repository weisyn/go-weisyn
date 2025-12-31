package fee

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
)

// Calculator 交易费用计算器
//
// 🎯 **零增发费用计算核心**
//
// 计算公式（按Token类型分别计算）:
//
//	Fee(token) = Sum(Inputs(token)) - Sum(Outputs(token))
//
// 支持的Token类型:
//   - 原生币: TokenKey = "native"
//   - 合约Token: TokenKey = "contract:{addr}:{classId}"
//
// 计算流程:
//  1. 遍历交易输入，通过UTXO获取资产信息
//  2. 遍历交易输出，直接提取资产信息
//  3. 按TokenKey分组计算差额
//  4. 验证费用非负（防止恶意交易）
type Calculator struct {
	utxoFetcher txiface.UTXOFetcher
}

// NewCalculator 创建费用计算器
func NewCalculator(utxoFetcher txiface.UTXOFetcher) *Calculator {
	if utxoFetcher == nil {
		panic("UTXOFetcher不能为nil")
	}
	return &Calculator{utxoFetcher: utxoFetcher}
}

// Calculate 计算单笔交易的费用
//
// 参数:
//
//	ctx: 上下文对象
//	tx: 待计算的交易
//
// 返回:
//
//	*AggregatedFees: 按Token分组的费用
//	error: 计算错误
//
// 特殊处理:
//   - Coinbase交易（len(Inputs)==0）返回空费用
//   - 负费用检测（输出>输入）返回错误
func (c *Calculator) Calculate(
	ctx context.Context,
	tx *transaction_pb.Transaction,
) (*txiface.AggregatedFees, error) {
	// Coinbase特殊处理：无输入，费用为0
	if len(tx.Inputs) == 0 {
		return &txiface.AggregatedFees{ByToken: make(map[txiface.TokenKey]*big.Int)}, nil
	}

	// 初始化输入输出累加器
	inputs := make(map[txiface.TokenKey]*big.Int)
	outputs := make(map[txiface.TokenKey]*big.Int)
	authorizedMintContracts := make(map[string]struct{})

	// 1. 遍历交易输入，查询UTXO并提取资产信息
	for i, input := range tx.Inputs {
		// 记录引用型ExecutionProof授权的合约地址（用于判定合法增发）
		var execCtx *transaction_pb.ExecutionProof_ExecutionContext
		if input.UnlockingProof != nil {
			if execProof, ok := input.UnlockingProof.(*transaction_pb.TxInput_ExecutionProof); ok {
				if execProof.ExecutionProof != nil && execProof.ExecutionProof.Context != nil {
					execCtx = execProof.ExecutionProof.Context
				}
			}
		}
		if execCtx != nil && len(execCtx.ResourceAddress) > 0 {
			addrHex := strings.ToLower(hex.EncodeToString(execCtx.ResourceAddress))
			authorizedMintContracts[addrHex] = struct{}{}
		}

		// 🔒 安全-1: 跳过引用型输入（不计入费用）
		if input.IsReferenceOnly {
			continue
		}

		// 查询输入引用的UTXO
		utxo, err := c.utxoFetcher(ctx, input.PreviousOutput)
		if err != nil {
			return nil, fmt.Errorf("输入[%d]: 查询UTXO失败: %w", i, err)
		}
		if utxo == nil {
			return nil, fmt.Errorf("输入[%d]: UTXO不存在", i)
		}

		// 提取资产输出
		assetOutput := utxo.GetAsset()
		if assetOutput == nil {
			// 非资产UTXO（如Resource、State），不计入费用
			continue
		}

		// 提取TokenKey和金额
		tokenKey, amount, err := c.extractTokenInfo(assetOutput)
		if err != nil {
			return nil, fmt.Errorf("输入[%d]: 提取Token信息失败: %w", i, err)
		}

		// 累加输入金额
		if existing, ok := inputs[tokenKey]; ok {
			inputs[tokenKey] = new(big.Int).Add(existing, amount)
		} else {
			inputs[tokenKey] = new(big.Int).Set(amount)
		}
	}

	// 2. 遍历交易输出，提取资产信息
	for i, output := range tx.Outputs {
		assetOutput := output.GetAsset()
		if assetOutput == nil {
			// 非资产输出，不计入费用
			continue
		}

		// 提取TokenKey和金额
		tokenKey, amount, err := c.extractTokenInfo(assetOutput)
		if err != nil {
			return nil, fmt.Errorf("输出[%d]: 提取Token信息失败: %w", i, err)
		}

		// 累加输出金额
		if existing, ok := outputs[tokenKey]; ok {
			outputs[tokenKey] = new(big.Int).Add(existing, amount)
		} else {
			outputs[tokenKey] = new(big.Int).Set(amount)
		}
	}

	// 3. 计算费用差额: Fee = Inputs - Outputs
	fees := &txiface.AggregatedFees{ByToken: make(map[txiface.TokenKey]*big.Int)}

	// 遍历所有输入的Token类型
	for tokenKey, inputAmount := range inputs {
		outputAmount, ok := outputs[tokenKey]
		if !ok {
			outputAmount = big.NewInt(0)
		}

		// 计算差额
		fee := new(big.Int).Sub(inputAmount, outputAmount)

		// 验证费用非负
		if fee.Sign() < 0 {
			return nil, fmt.Errorf("负费用检测: token=%s, 输入=%s, 输出=%s",
				tokenKey, inputAmount.String(), outputAmount.String())
		}

		// 只记录正费用（跳过零费用）
		if fee.Sign() > 0 {
			fees.ByToken[tokenKey] = fee
		}
	}

	// 检查是否有输出但没有对应输入的Token（理论上不应该出现）
	for tokenKey, outputAmount := range outputs {
		if _, ok := inputs[tokenKey]; !ok {
			tokenStr := string(tokenKey)
			if strings.HasPrefix(tokenStr, "contract:") {
				parts := strings.SplitN(tokenStr, ":", 3)
				if len(parts) >= 2 {
					contractHex := strings.ToLower(parts[1])
					if _, authorized := authorizedMintContracts[contractHex]; authorized {
						// 合约ExecutionProof授权的增发，允许输出大于输入
						continue
					}
				}
			}
			return nil, fmt.Errorf("输出Token没有对应输入: token=%s, 输出=%s",
				tokenKey, outputAmount.String())
		}
	}

	return fees, nil
}

// extractTokenInfo 从AssetOutput提取TokenKey和金额
//
// 参数:
//
//	assetOutput: 资产输出
//
// 返回:
//
//	TokenKey: Token唯一标识
//	*big.Int: 金额
//	error: 提取错误
func (c *Calculator) extractTokenInfo(assetOutput *transaction_pb.AssetOutput) (txiface.TokenKey, *big.Int, error) {
	// 检查原生币
	if nativeCoin := assetOutput.GetNativeCoin(); nativeCoin != nil {
		amount, ok := new(big.Int).SetString(nativeCoin.Amount, 10)
		if !ok {
			return "", nil, fmt.Errorf("原生币金额格式错误: %s", nativeCoin.Amount)
		}
		return txiface.TokenKey("native"), amount, nil
	}

	// 检查合约Token
	if contractToken := assetOutput.GetContractToken(); contractToken != nil {
		// 解析金额
		amount, ok := new(big.Int).SetString(contractToken.Amount, 10)
		if !ok {
			return "", nil, fmt.Errorf("合约Token金额格式错误: %s", contractToken.Amount)
		}

		// 构造TokenKey: contract:{addr}:{classId}
		var tokenKey txiface.TokenKey

		if fungibleClassId := contractToken.GetFungibleClassId(); fungibleClassId != nil {
			// 同质化Token
			tokenKey = txiface.TokenKey(fmt.Sprintf("contract:%x:%x",
				contractToken.ContractAddress, fungibleClassId))
		} else if nftUniqueId := contractToken.GetNftUniqueId(); nftUniqueId != nil {
			// NFT
			tokenKey = txiface.TokenKey(fmt.Sprintf("contract:%x:nft:%x",
				contractToken.ContractAddress, nftUniqueId))
		} else if sfId := contractToken.GetSemiFungibleId(); sfId != nil {
			// 半同质化Token (InstanceId是uint64)
			tokenKey = txiface.TokenKey(fmt.Sprintf("contract:%x:sft:%x:%x",
				contractToken.ContractAddress, sfId.BatchId, sfId.InstanceId))
		} else {
			return "", nil, fmt.Errorf("合约Token缺少标识符")
		}

		return tokenKey, amount, nil
	}

	return "", nil, fmt.Errorf("未知的资产类型")
}
