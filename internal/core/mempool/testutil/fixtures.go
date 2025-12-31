// Package testutil 测试数据Fixtures
package testutil

import (
	"fmt"
	"time"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// CreateTestTransaction 创建测试用的交易
//
// 参数：
//   - txID: 交易ID（用于生成唯一的交易）
//   - inputs: 交易输入列表（可选，默认创建一个输入）
//   - outputs: 交易输出列表（可选，默认创建一个输出）
//
// 返回：
//   - *transaction.Transaction: 测试交易
func CreateTestTransaction(txID int, inputs []*transaction.TxInput, outputs []*transaction.TxOutput) *transaction.Transaction {
	// 生成唯一的交易ID字节
	txIDBytes := make([]byte, 32)
	txIDBytes[0] = byte(txID)
	txIDBytes[1] = byte(txID >> 8)
	txIDBytes[2] = byte(txID >> 16)
	txIDBytes[3] = byte(txID >> 24)

	// 如果没有提供输入，创建默认输入
	if inputs == nil || len(inputs) == 0 {
		inputs = []*transaction.TxInput{
			{
				PreviousOutput: &transaction.OutPoint{
					TxId:        txIDBytes,
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
			},
		}
	}

	// 如果没有提供输出，创建默认输出
	if outputs == nil || len(outputs) == 0 {
		outputs = []*transaction.TxOutput{
			{
				Owner: []byte(fmt.Sprintf("recipient_%d", txID)),
				LockingConditions: []*transaction.LockingCondition{
					{
						Condition: &transaction.LockingCondition_SingleKeyLock{
							SingleKeyLock: &transaction.SingleKeyLock{
								KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
									RequiredAddressHash: []byte(fmt.Sprintf("addr_hash_%d", txID)),
								},
								RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
								SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
							},
						},
					},
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "100000000000", // 1000 WES
							},
						},
					},
				},
			},
		}
	}

	return &transaction.Transaction{
		Version:           1,
		Inputs:            inputs,
		Outputs:           outputs,
		Nonce:             uint64(txID),
		CreationTimestamp: uint64(time.Now().Unix()),
		ChainId:           []byte("weisyn-testnet"),
		FeeMechanism: &transaction.Transaction_MinimumFee{
			MinimumFee: &transaction.MinimumFee{
				MinimumAmount: "5000000000", // 50 WES
			},
		},
	}
}

// CreateSimpleTestTransaction 创建简单的测试交易
//
// 参数：
//   - txID: 交易ID
//
// 返回：
//   - *transaction.Transaction: 测试交易
func CreateSimpleTestTransaction(txID int) *transaction.Transaction {
	return CreateTestTransaction(txID, nil, nil)
}

// CreateTestTransactionWithAmount 创建指定金额的测试交易
//
// 参数：
//   - txID: 交易ID
//   - amount: 金额（字符串格式）
//
// 返回：
//   - *transaction.Transaction: 测试交易
func CreateTestTransactionWithAmount(txID int, amount string) *transaction.Transaction {
	outputs := []*transaction.TxOutput{
		{
			Owner: []byte(fmt.Sprintf("recipient_%d", txID)),
			LockingConditions: []*transaction.LockingCondition{
				{
					Condition: &transaction.LockingCondition_SingleKeyLock{
						SingleKeyLock: &transaction.SingleKeyLock{
							KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
								RequiredAddressHash: []byte(fmt.Sprintf("addr_hash_%d", txID)),
							},
							RequiredAlgorithm: transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
							SighashType:       transaction.SignatureHashType_SIGHASH_ALL,
						},
					},
				},
			},
			OutputContent: &transaction.TxOutput_Asset{
				Asset: &transaction.AssetOutput{
					AssetContent: &transaction.AssetOutput_NativeCoin{
						NativeCoin: &transaction.NativeCoinAsset{
							Amount: amount,
						},
					},
				},
			},
		},
	}
	return CreateTestTransaction(txID, nil, outputs)
}

