// Package testutil 提供 TX 模块测试的辅助工具
//
// 🧪 **测试数据Fixtures**
//
// 本文件提供测试数据的创建函数，用于简化测试代码编写。
// 遵循 docs/system/standards/principles/testing-standards.md 规范。
package testutil

import (
	"crypto/rand"
	"math/big"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== 测试数据 Fixtures ====================

// RandomBytes 生成随机字节数组
func RandomBytes(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

// RandomAddress 生成随机地址（20 字节）
func RandomAddress() []byte {
	return RandomBytes(20)
}

// RandomPublicKey 生成随机公钥（33 字节，压缩格式）
func RandomPublicKey() []byte {
	return RandomBytes(33)
}

// RandomTxID 生成随机交易 ID（32 字节）
func RandomTxID() []byte {
	return RandomBytes(32)
}

// RandomHash 生成随机哈希（32 字节）
func RandomHash() []byte {
	return RandomBytes(32)
}

// CreateOutPoint 创建测试用的 OutPoint
func CreateOutPoint(txid []byte, index uint32) *transaction.OutPoint {
	if txid == nil {
		txid = make([]byte, 32)
		rand.Read(txid)
	}
	return &transaction.OutPoint{
		TxId:        txid,
		OutputIndex: index,
	}
}

// CreateSingleKeyLock 创建测试用的 SingleKeyLock
func CreateSingleKeyLock(publicKey []byte) *transaction.LockingCondition {
	if publicKey == nil {
		publicKey = make([]byte, 33)
		rand.Read(publicKey)
	}
	return &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{
				KeyRequirement: &transaction.SingleKeyLock_RequiredPublicKey{
					RequiredPublicKey: &transaction.PublicKey{
						Value: publicKey,
					},
				},
			},
		},
	}
}

// CreateSingleKeyProof 创建测试用的 SingleKeyProof
func CreateSingleKeyProof(publicKey []byte, signature []byte) *transaction.UnlockingProof {
	if publicKey == nil {
		publicKey = make([]byte, 33)
		rand.Read(publicKey)
	}
	if signature == nil {
		signature = make([]byte, 64)
		rand.Read(signature)
	}
	return &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: &transaction.SingleKeyProof{
				Signature: &transaction.SignatureData{
					Value: signature,
				},
				PublicKey: &transaction.PublicKey{
					Value: publicKey,
				},
			},
		},
	}
}

// CreateMultiKeyLock 创建测试用的 MultiKeyLock
func CreateMultiKeyLock(publicKeys [][]byte, requiredSignatures uint32) *transaction.LockingCondition {
	if publicKeys == nil {
		publicKeys = [][]byte{RandomPublicKey(), RandomPublicKey()}
	}
	if requiredSignatures == 0 {
		requiredSignatures = uint32(len(publicKeys))
	}
	pubKeys := make([]*transaction.PublicKey, len(publicKeys))
	for i, pk := range publicKeys {
		pubKeys[i] = &transaction.PublicKey{Value: pk}
	}
	return &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_MultiKeyLock{
			MultiKeyLock: &transaction.MultiKeyLock{
				AuthorizedKeys:     pubKeys,
				RequiredSignatures: requiredSignatures,
			},
		},
	}
}

// CreateNativeCoinOutput 创建测试用的原生币输出
func CreateNativeCoinOutput(owner []byte, amount string, lock *transaction.LockingCondition) *transaction.TxOutput {
	if owner == nil {
		owner = make([]byte, 20)
		rand.Read(owner)
	}
	if lock == nil {
		lock = CreateSingleKeyLock(nil)
	}
	return &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: []*transaction.LockingCondition{lock},
		OutputContent: &transaction.TxOutput_Asset{
			Asset: &transaction.AssetOutput{
				AssetContent: &transaction.AssetOutput_NativeCoin{
					NativeCoin: &transaction.NativeCoinAsset{
						Amount: amount,
					},
				},
			},
		},
	}
}

// CreateContractTokenOutput 创建测试用的合约代币输出
func CreateContractTokenOutput(
	owner []byte,
	amount string,
	contractAddress []byte,
	classID []byte,
	lock *transaction.LockingCondition,
) *transaction.TxOutput {
	if owner == nil {
		owner = make([]byte, 20)
		rand.Read(owner)
	}
	if contractAddress == nil {
		contractAddress = make([]byte, 20)
		rand.Read(contractAddress)
	}
	if classID == nil {
		classID = []byte("default")
	}
	if lock == nil {
		lock = &transaction.LockingCondition{
			Condition: &transaction.LockingCondition_ContractLock{
				ContractLock: &transaction.ContractLock{
					ContractAddress: append([]byte(nil), contractAddress...),
				},
			},
		}
	}
	return &transaction.TxOutput{
		Owner:             owner,
		LockingConditions: []*transaction.LockingCondition{lock},
		OutputContent: &transaction.TxOutput_Asset{
			Asset: &transaction.AssetOutput{
				AssetContent: &transaction.AssetOutput_ContractToken{
					ContractToken: &transaction.ContractTokenAsset{
						ContractAddress: contractAddress,
						TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
							FungibleClassId: classID,
						},
						Amount: amount,
					},
				},
			},
		},
	}
}

// CreateUTXO 创建测试用的 UTXO
func CreateUTXO(
	outpoint *transaction.OutPoint,
	output *transaction.TxOutput,
	status utxopb.UTXOLifecycleStatus,
) *utxopb.UTXO {
	if outpoint == nil {
		outpoint = CreateOutPoint(nil, 0)
	}
	if output == nil {
		output = CreateNativeCoinOutput(nil, "1000", nil)
	}
	if status == 0 {
		status = utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE
	}
	return &utxopb.UTXO{
		Outpoint:     outpoint,
		Category:     utxopb.UTXOCategory_UTXO_CATEGORY_ASSET,
		Status:       status,
		OwnerAddress: extractOwnerFromOutput(output), // 从 output 提取 owner
		ContentStrategy: &utxopb.UTXO_CachedOutput{
			CachedOutput: output,
		},
	}
}

// extractOwnerFromOutput 从 TxOutput 中提取 owner 地址
func extractOwnerFromOutput(output *transaction.TxOutput) []byte {
	if output != nil && len(output.Owner) > 0 {
		return output.Owner
	}
	return RandomAddress()
}

// CreateTransaction 创建测试用的交易
func CreateTransaction(
	inputs []*transaction.TxInput,
	outputs []*transaction.TxOutput,
) *transaction.Transaction {
	return &transaction.Transaction{
		Version:           1,
		Inputs:            inputs,
		Outputs:           outputs,
		CreationTimestamp: uint64(0),
	}
}

// ==================== 金额计算辅助函数 ====================

// BigIntFromString 从字符串创建 big.Int（用于测试）
func BigIntFromString(s string) *big.Int {
	val, _ := new(big.Int).SetString(s, 10)
	return val
}

// BigIntToString 将 big.Int 转换为字符串（用于测试）
func BigIntToString(val *big.Int) string {
	return val.String()
}

// AmountAdd 金额相加（用于测试）
func AmountAdd(a, b string) string {
	valA, _ := new(big.Int).SetString(a, 10)
	valB, _ := new(big.Int).SetString(b, 10)
	return new(big.Int).Add(valA, valB).String()
}

// AmountSub 金额相减（用于测试）
func AmountSub(a, b string) string {
	valA, _ := new(big.Int).SetString(a, 10)
	valB, _ := new(big.Int).SetString(b, 10)
	return new(big.Int).Sub(valA, valB).String()
}

// AmountCmp 金额比较（用于测试）
// 返回：-1 (a < b), 0 (a == b), 1 (a > b)
func AmountCmp(a, b string) int {
	valA, _ := new(big.Int).SetString(a, 10)
	valB, _ := new(big.Int).SetString(b, 10)
	return valA.Cmp(valB)
}
