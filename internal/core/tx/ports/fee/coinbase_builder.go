package fee

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"

	transaction_pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	txiface "github.com/weisyn/v1/pkg/interfaces/tx"
	"github.com/weisyn/v1/pkg/utils/timeutil"
)

// CoinbaseBuilder Coinbase交易构建器（零增发）
//
// 🎯 **零增发核心组件**
//
// 构建规则:
//   - 无输入
//   - 输出 = 手续费聚合（按Token分组）
//   - 所有输出Owner = minerAddr
//   - 无BlockReward（零增发）
//
// Token输出顺序:
//  1. 原生币（native）优先
//  2. 其他Token按TokenKey字典序排列（确定性）
type CoinbaseBuilder struct{}

// NewCoinbaseBuilder 创建Coinbase构建器
func NewCoinbaseBuilder() *CoinbaseBuilder {
	return &CoinbaseBuilder{}
}

// Build 构建零增发Coinbase交易
//
// 参数:
//
//	aggregatedFees: 聚合的手续费（按Token分组）
//	minerAddr: 矿工地址（20字节）
//	chainID: 链ID
//
// 返回:
//
//	*Transaction: Coinbase交易
//	error: 构建错误
func (b *CoinbaseBuilder) Build(
	aggregatedFees *txiface.AggregatedFees,
	minerAddr []byte,
	chainID []byte,
) (*transaction_pb.Transaction, error) {
	if aggregatedFees == nil {
		return nil, fmt.Errorf("aggregatedFees不能为nil")
	}
	if len(minerAddr) != 20 {
		return nil, fmt.Errorf("矿工地址长度必须为20字节")
	}
	if len(chainID) == 0 {
		return nil, fmt.Errorf("chainID不能为空")
	}

	// 创建基础Coinbase交易
	coinbase := &transaction_pb.Transaction{
		Version:           1,
		Inputs:            []*transaction_pb.TxInput{}, // Coinbase无输入
		Outputs:           []*transaction_pb.TxOutput{},
		Nonce:             0,
		CreationTimestamp: uint64(timeutil.NowUnix()),
		ChainId:           chainID,
	}

	// 1. 原生币优先（始终创建原生币输出，即使金额为0）
	// ✅ 设计理由：
	//    - Coinbase第一个输出的Owner标识了矿工地址
	//    - 即使没有手续费收入，也需要记录是谁挖出了这个区块
	//    - 零增发模式下，金额为0是合法的
	nativeKey := txiface.TokenKey("native")
	nativeAmount := big.NewInt(0) // 默认为0
	if amount, ok := aggregatedFees.ByToken[nativeKey]; ok {
		nativeAmount = amount
	}

	// 🔧 修复：移除硬编码区块奖励，确保零增发原则
	// WES系统采用零增发设计，Coinbase只包含手续费，不包含区块奖励
	// 如果需要测试环境支持区块奖励，应通过配置系统管理（目前不支持）
	// 注意：nativeAmount 只包含手续费，符合零增发原则

	// 创建原生币输出（允许金额为0）
	output, err := b.createFeeOutput(nativeKey, nativeAmount, minerAddr)
	if err != nil {
		return nil, fmt.Errorf("创建原生币输出失败: %w", err)
	}
	coinbase.Outputs = append(coinbase.Outputs, output)

	// 2. 其他Token按字典序排列（仅创建金额>0的输出）
	sortedKeys := b.sortTokenKeys(aggregatedFees.ByToken)
	for _, tokenKey := range sortedKeys {
		// 跳过原生币（已处理）
		if tokenKey == nativeKey {
			continue
		}

		amount := aggregatedFees.ByToken[tokenKey]
		if amount.Sign() > 0 {
			output, err := b.createFeeOutput(tokenKey, amount, minerAddr)
			if err != nil {
				return nil, fmt.Errorf("创建Token输出失败 [%s]: %w", tokenKey, err)
			}
			coinbase.Outputs = append(coinbase.Outputs, output)
		}
	}

	return coinbase, nil
}

// sortTokenKeys 对TokenKey进行字典序排序（确定性）
func (b *CoinbaseBuilder) sortTokenKeys(tokenMap map[txiface.TokenKey]*big.Int) []txiface.TokenKey {
	keys := make([]txiface.TokenKey, 0, len(tokenMap))
	for k := range tokenMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return string(keys[i]) < string(keys[j])
	})
	return keys
}

// createFeeOutput 创建单个Token的费用输出
//
// 参数:
//
//	tokenKey: Token唯一标识
//	amount: Token金额
//	minerAddr: 矿工地址
//
// 返回:
//
//	*TxOutput: 费用输出
//	error: 构建错误
func (b *CoinbaseBuilder) createFeeOutput(
	tokenKey txiface.TokenKey,
	amount *big.Int,
	minerAddr []byte,
) (*transaction_pb.TxOutput, error) {
	output := &transaction_pb.TxOutput{
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
	}

	// 根据TokenKey类型创建资产输出
	if tokenKey == "native" {
		// 原生币
		output.OutputContent = &transaction_pb.TxOutput_Asset{
			Asset: &transaction_pb.AssetOutput{
				AssetContent: &transaction_pb.AssetOutput_NativeCoin{
					NativeCoin: &transaction_pb.NativeCoinAsset{
						Amount: amount.String(),
					},
				},
			},
		}
	} else if strings.HasPrefix(string(tokenKey), "contract:") {
		// 合约Token: contract:{addr}:{type}:{id}
		// 支持格式:
		//   contract:{addr}:{classId}              -> 同质化Token
		//   contract:{addr}:nft:{uniqueId}         -> NFT
		//   contract:{addr}:sft:{batchId}:{instId} -> 半同质化Token
		assetOutput, err := b.parseContractToken(string(tokenKey), amount)
		if err != nil {
			return nil, fmt.Errorf("解析合约Token失败: %w", err)
		}
		output.OutputContent = &transaction_pb.TxOutput_Asset{
			Asset: assetOutput,
		}
	} else {
		return nil, fmt.Errorf("未知的TokenKey格式: %s", tokenKey)
	}

	return output, nil
}

// parseContractToken 解析合约Token字符串并构建AssetOutput
//
// TokenKey格式:
//   - contract:{addr}:{classId}              -> FT
//   - contract:{addr}:nft:{uniqueId}         -> NFT
//   - contract:{addr}:sft:{batchId}:{instId} -> SFT
func (b *CoinbaseBuilder) parseContractToken(tokenKeyStr string, amount *big.Int) (*transaction_pb.AssetOutput, error) {
	parts := strings.Split(tokenKeyStr, ":")
	if len(parts) < 3 || parts[0] != "contract" {
		return nil, fmt.Errorf("无效的合约Token格式: %s", tokenKeyStr)
	}

	// 解析合约地址（十六进制）
	contractAddr, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("解析合约地址失败: %w", err)
	}

	// 根据类型解析Token标识符
	if len(parts) == 3 {
		// contract:{addr}:{classId} -> 同质化Token (FT)
		classId, err := hex.DecodeString(parts[2])
		if err != nil {
			return nil, fmt.Errorf("解析FungibleClassId失败: %w", err)
		}
		return &transaction_pb.AssetOutput{
			AssetContent: &transaction_pb.AssetOutput_ContractToken{
				ContractToken: &transaction_pb.ContractTokenAsset{
					ContractAddress: contractAddr,
					TokenIdentifier: &transaction_pb.ContractTokenAsset_FungibleClassId{
						FungibleClassId: classId,
					},
					Amount: amount.String(),
				},
			},
		}, nil
	} else if len(parts) == 4 && parts[2] == "nft" {
		// contract:{addr}:nft:{uniqueId} -> NFT
		uniqueId, err := hex.DecodeString(parts[3])
		if err != nil {
			return nil, fmt.Errorf("解析NftUniqueId失败: %w", err)
		}
		return &transaction_pb.AssetOutput{
			AssetContent: &transaction_pb.AssetOutput_ContractToken{
				ContractToken: &transaction_pb.ContractTokenAsset{
					ContractAddress: contractAddr,
					TokenIdentifier: &transaction_pb.ContractTokenAsset_NftUniqueId{
						NftUniqueId: uniqueId,
					},
					Amount: amount.String(),
				},
			},
		}, nil
	} else if len(parts) == 5 && parts[2] == "sft" {
		// contract:{addr}:sft:{batchId}:{instanceId} -> 半同质化Token
		batchId, err := hex.DecodeString(parts[3])
		if err != nil {
			return nil, fmt.Errorf("解析SFT BatchId失败: %w", err)
		}
		// InstanceId是uint64，需要从十六进制字符串解析
		var instanceId uint64
		_, err = fmt.Sscanf(parts[4], "%x", &instanceId)
		if err != nil {
			return nil, fmt.Errorf("解析SFT InstanceId失败: %w", err)
		}
		return &transaction_pb.AssetOutput{
			AssetContent: &transaction_pb.AssetOutput_ContractToken{
				ContractToken: &transaction_pb.ContractTokenAsset{
					ContractAddress: contractAddr,
					TokenIdentifier: &transaction_pb.ContractTokenAsset_SemiFungibleId{
						SemiFungibleId: &transaction_pb.SemiFungibleId{
							BatchId:    batchId,
							InstanceId: instanceId,
						},
					},
					Amount: amount.String(),
				},
			},
		}, nil
	}

	return nil, fmt.Errorf("不支持的合约Token格式: %s", tokenKeyStr)
}
