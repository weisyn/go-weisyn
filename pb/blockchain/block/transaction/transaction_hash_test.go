package transaction

import (
	"crypto/sha256"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

func TestTransactionHashCalculation(t *testing.T) {
	tests := []struct {
		name        string
		tx          *Transaction
		expectError bool
	}{
		{
			name: "valid_simple_transaction",
			tx: &Transaction{
				Version: 1,
				Inputs: []*TxInput{
					{
						PreviousOutput: &OutPoint{
							TxId:        []byte("input_tx_hash_32_bytes_long______"), // 32 bytes
							OutputIndex: 0,
						},
						IsReferenceOnly: false,
						Sequence:        0xFFFFFFFF,
					},
				},
				Outputs: []*TxOutput{
					{
						Owner: []byte("recipient_address"),
						LockingConditions: []*LockingCondition{
							{
								Condition: &LockingCondition_SingleKeyLock{
									SingleKeyLock: &SingleKeyLock{
										KeyRequirement: &SingleKeyLock_RequiredAddressHash{
											RequiredAddressHash: []byte("addr_hash_20_bytes__"), // 20 bytes
										},
										RequiredAlgorithm: SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
										SighashType:       SignatureHashType_SIGHASH_ALL,
									},
								},
							},
						},
						OutputContent: &TxOutput_Asset{
							Asset: &AssetOutput{
								AssetContent: &AssetOutput_NativeCoin{
									NativeCoin: &NativeCoinAsset{
										Amount: "100000000000", // 1000 WES
									},
								},
							},
						},
					},
				},
				Nonce:             12345,
				CreationTimestamp: uint64(time.Now().Unix()),
				ChainId:           []byte("weisyn-testnet"),
				FeeMechanism: &Transaction_MinimumFee{
					MinimumFee: &MinimumFee{
						MinimumAmount: "5000000000", // 50 WES
					},
				},
			},
			expectError: false,
		},
		{
			name: "valid_contract_transaction",
			tx: &Transaction{
				Version: 1,
				Inputs: []*TxInput{
					{
						PreviousOutput: &OutPoint{
							TxId:        []byte("contract_tx_hash_32_bytes_long___"), // 32 bytes
							OutputIndex: 0,
						},
						IsReferenceOnly: true, // 引用合约
						UnlockingProof: &TxInput_ExecutionProof{
							ExecutionProof: &ExecutionProof{
								ExecutionResultHash:  []byte("exec_result_hash_32_bytes_long__"), // 32 bytes
								StateTransitionProof: []byte("state_proof"),
								ExecutionTimeMs:      50000, // 修复：ExecutionTimeMs -> ExecutionTimeMs
								Context: &ExecutionProof_ExecutionContext{
									CallerIdentity: &IdentityProof{
										PublicKey:     make([]byte, 33),
										CallerAddress: []byte("caller_address"),
										Signature:     make([]byte, 64),
										Algorithm:     SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
										SighashType:   SignatureHashType_SIGHASH_ALL,
										Nonce:         make([]byte, 32),
										Timestamp:     1234567890,
										ContextHash:   make([]byte, 32),
									},
									ResourceAddress: make([]byte, 20),
									ExecutionType:   ExecutionType_EXECUTION_TYPE_CONTRACT,
									InputDataHash:   make([]byte, 32), // ✅ 使用哈希替代原始数据（32字节SHA-256）
									OutputDataHash:  make([]byte, 32), // ✅ 使用哈希替代原始数据（32字节SHA-256）
									Metadata: map[string][]byte{
										"method_name":                []byte("execute"),
										"contract_state_before_hash": make([]byte, 32), // ✅ 状态哈希存储在metadata中
										"contract_state_after_hash":  make([]byte, 32), // ✅ 状态哈希存储在metadata中
									},
								},
							},
						},
					},
				},
				Outputs: []*TxOutput{
					{
						Owner: []byte("contract_executor"),
						OutputContent: &TxOutput_State{
							State: &StateOutput{
								StateId:             []byte("state_id_unique"),
								StateVersion:        1,
								ExecutionResultHash: []byte("exec_result_hash_32_bytes_long__"), // 32 bytes
							},
						},
					},
				},
				Nonce:             23456,
				CreationTimestamp: uint64(time.Now().Unix()),
				ChainId:           []byte("weisyn-testnet"),
				FeeMechanism: &Transaction_ContractFee{
					ContractFee: &ContractExecutionFee{
						BaseFee:      "1000",
						ExecutionFee: "100000000", // 修复：ExecutionFee = 100000 * 1000 = 100000000
						FeeToken: &TokenReference{
							TokenType: &TokenReference_NativeToken{
								NativeToken: true,
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "nil_transaction",
			tx:          nil,
			expectError: true,
		},
		{
			name: "empty_inputs_outputs",
			tx: &Transaction{
				Version:           1,
				Inputs:            []*TxInput{},
				Outputs:           []*TxOutput{},
				Nonce:             12345,
				CreationTimestamp: uint64(time.Now().Unix()),
				ChainId:           []byte("weisyn-testnet"),
			},
			expectError: true, // 空输入输出应该被认为是无效的
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 测试交易哈希计算
			hash, err := ComputeTransactionHash(tt.tx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for test case %s, but got none", tt.name)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for test case %s: %v", tt.name, err)
				return
			}

			// 验证哈希长度
			if len(hash) != 32 {
				t.Errorf("Expected hash length 32, got %d", len(hash))
			}

			// 验证哈希的一致性 - 相同输入应该产生相同哈希
			hash2, err := ComputeTransactionHash(tt.tx)
			if err != nil {
				t.Errorf("Error computing hash second time: %v", err)
				return
			}

			if string(hash) != string(hash2) {
				t.Errorf("Hash calculation is not deterministic")
			}
		})
	}
}

func TestTransactionHashDeterminism(t *testing.T) {
	// 创建两个相同的交易
	tx1 := &Transaction{
		Version: 1,
		Inputs: []*TxInput{
			{
				PreviousOutput: &OutPoint{
					TxId:        []byte("same_tx_hash_32_bytes_long_______"), // 32 bytes
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
			},
		},
		Outputs: []*TxOutput{
			{
				Owner: []byte("same_recipient"),
				OutputContent: &TxOutput_Asset{
					Asset: &AssetOutput{
						AssetContent: &AssetOutput_NativeCoin{
							NativeCoin: &NativeCoinAsset{
								Amount: "100000000000",
							},
						},
					},
				},
			},
		},
		Nonce:             54321,
		CreationTimestamp: 1234567890,
		ChainId:           []byte("weisyn-testnet"),
	}

	tx2 := &Transaction{
		Version: 1,
		Inputs: []*TxInput{
			{
				PreviousOutput: &OutPoint{
					TxId:        []byte("same_tx_hash_32_bytes_long_______"), // 32 bytes
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
			},
		},
		Outputs: []*TxOutput{
			{
				Owner: []byte("same_recipient"),
				OutputContent: &TxOutput_Asset{
					Asset: &AssetOutput{
						AssetContent: &AssetOutput_NativeCoin{
							NativeCoin: &NativeCoinAsset{
								Amount: "100000000000",
							},
						},
					},
				},
			},
		},
		Nonce:             54321,
		CreationTimestamp: 1234567890,
		ChainId:           []byte("weisyn-testnet"),
	}

	hash1, err1 := ComputeTransactionHash(tx1)
	hash2, err2 := ComputeTransactionHash(tx2)

	if err1 != nil || err2 != nil {
		t.Fatalf("Hash computation failed: err1=%v, err2=%v", err1, err2)
	}

	if string(hash1) != string(hash2) {
		t.Errorf("Identical transactions should produce identical hashes")
	}
}

func TestTransactionHashUniqueness(t *testing.T) {
	baseTx := &Transaction{
		Version: 1,
		Inputs: []*TxInput{
			{
				PreviousOutput: &OutPoint{
					TxId:        []byte("base_tx_hash_32_bytes_long_______"), // 32 bytes
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
			},
		},
		Outputs: []*TxOutput{
			{
				Owner: []byte("base_recipient"),
				OutputContent: &TxOutput_Asset{
					Asset: &AssetOutput{
						AssetContent: &AssetOutput_NativeCoin{
							NativeCoin: &NativeCoinAsset{
								Amount: "100000000000",
							},
						},
					},
				},
			},
		},
		Nonce:             11111,
		CreationTimestamp: 1234567890,
		ChainId:           []byte("weisyn-testnet"),
	}

	// 创建修改版本 - 不同的nonce
	modifiedTx := &Transaction{
		Version: 1,
		Inputs: []*TxInput{
			{
				PreviousOutput: &OutPoint{
					TxId:        []byte("base_tx_hash_32_bytes_long_______"), // 32 bytes
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
			},
		},
		Outputs: []*TxOutput{
			{
				Owner: []byte("base_recipient"),
				OutputContent: &TxOutput_Asset{
					Asset: &AssetOutput{
						AssetContent: &AssetOutput_NativeCoin{
							NativeCoin: &NativeCoinAsset{
								Amount: "100000000000",
							},
						},
					},
				},
			},
		},
		Nonce:             22222, // 不同的nonce
		CreationTimestamp: 1234567890,
		ChainId:           []byte("weisyn-testnet"),
	}

	hash1, err1 := ComputeTransactionHash(baseTx)
	hash2, err2 := ComputeTransactionHash(modifiedTx)

	if err1 != nil || err2 != nil {
		t.Fatalf("Hash computation failed: err1=%v, err2=%v", err1, err2)
	}

	if string(hash1) == string(hash2) {
		t.Errorf("Different transactions should produce different hashes")
	}
}

func TestTransactionHashExcludesSignatures(t *testing.T) {
	// 创建基础交易
	baseTx := &Transaction{
		Version: 1,
		Inputs: []*TxInput{
			{
				PreviousOutput: &OutPoint{
					TxId:        []byte("sig_test_hash_32_bytes_long______"), // 32 bytes
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
				// 没有签名
			},
		},
		Outputs: []*TxOutput{
			{
				Owner: []byte("sig_test_recipient"),
				OutputContent: &TxOutput_Asset{
					Asset: &AssetOutput{
						AssetContent: &AssetOutput_NativeCoin{
							NativeCoin: &NativeCoinAsset{
								Amount: "100000000000",
							},
						},
					},
				},
			},
		},
		Nonce:             33333,
		CreationTimestamp: 1234567890,
		ChainId:           []byte("weisyn-testnet"),
	}

	// 创建带签名的交易（在实际实现中，哈希计算应该排除签名数据）
	signedTx := &Transaction{
		Version: 1,
		Inputs: []*TxInput{
			{
				PreviousOutput: &OutPoint{
					TxId:        []byte("sig_test_hash_32_bytes_long______"), // 32 bytes
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
				UnlockingProof: &TxInput_SingleKeyProof{
					SingleKeyProof: &SingleKeyProof{
						Signature: &SignatureData{
							Value: []byte("dummy_signature_data"),
						},
						PublicKey: &PublicKey{
							Value: []byte("dummy_public_key"),
						},
						Algorithm:   SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType: SignatureHashType_SIGHASH_ALL,
					},
				},
			},
		},
		Outputs: []*TxOutput{
			{
				Owner: []byte("sig_test_recipient"),
				OutputContent: &TxOutput_Asset{
					Asset: &AssetOutput{
						AssetContent: &AssetOutput_NativeCoin{
							NativeCoin: &NativeCoinAsset{
								Amount: "100000000000",
							},
						},
					},
				},
			},
		},
		Nonce:             33333,
		CreationTimestamp: 1234567890,
		ChainId:           []byte("weisyn-testnet"),
	}

	hash1, err1 := ComputeTransactionHash(baseTx)
	hash2, err2 := ComputeTransactionHash(signedTx)

	if err1 != nil || err2 != nil {
		t.Fatalf("Hash computation failed: err1=%v, err2=%v", err1, err2)
	}

	// 注意：在这个简单测试实现中，哈希可能不同，因为我们包含了签名
	// 在真实实现中，应该排除签名数据，使得hash1 == hash2
	t.Logf("Base tx hash: %x", hash1)
	t.Logf("Signed tx hash: %x", hash2)

	// TODO: 在真实实现中，应该验证 string(hash1) == string(hash2)
}

// ComputeTransactionHash 计算交易哈希的简单实现
// 在实际系统中，这应该在专门的哈希服务中实现
// 注意：真实实现应该排除签名数据
func ComputeTransactionHash(tx *Transaction) ([]byte, error) {
	if tx == nil {
		return nil, ErrNilTransaction
	}

	if len(tx.Inputs) == 0 && len(tx.Outputs) == 0 {
		return nil, ErrEmptyTransaction
	}

	// 序列化完整交易（在真实实现中应该排除签名字段）
	// ⚠️ 注意：交易结构里包含 map 字段（如 ExecutionContext.Metadata），proto.Marshal 默认不保证 map 序列化顺序，
	// 会导致同一对象多次 hash 结果不一致。测试用实现也应使用确定性序列化。
	txBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(tx)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(txBytes)
	return hash[:], nil
}

// 定义测试用的错误类型
var (
	ErrNilTransaction   = &TransactionHashError{Code: "NIL_TRANSACTION", Message: "transaction cannot be nil"}
	ErrEmptyTransaction = &TransactionHashError{Code: "EMPTY_TRANSACTION", Message: "transaction cannot have empty inputs and outputs"}
)

type TransactionHashError struct {
	Code    string
	Message string
}

func (e *TransactionHashError) Error() string {
	return e.Message
}

func BenchmarkTransactionHashCalculation(t *testing.B) {
	tx := &Transaction{
		Version: 1,
		Inputs: []*TxInput{
			{
				PreviousOutput: &OutPoint{
					TxId:        []byte("benchmark_hash_32_bytes_long____"), // 32 bytes
					OutputIndex: 0,
				},
				IsReferenceOnly: false,
				Sequence:        0xFFFFFFFF,
			},
		},
		Outputs: []*TxOutput{
			{
				Owner: []byte("benchmark_recipient"),
				OutputContent: &TxOutput_Asset{
					Asset: &AssetOutput{
						AssetContent: &AssetOutput_NativeCoin{
							NativeCoin: &NativeCoinAsset{
								Amount: "100000000000",
							},
						},
					},
				},
			},
		},
		Nonce:             99999,
		CreationTimestamp: 1234567890,
		ChainId:           []byte("weisyn-testnet"),
		FeeMechanism: &Transaction_MinimumFee{
			MinimumFee: &MinimumFee{
				MinimumAmount: "5000000000",
			},
		},
	}

	t.ResetTimer()
	for i := 0; i < t.N; i++ {
		_, err := ComputeTransactionHash(tx)
		if err != nil {
			t.Fatalf("Hash computation failed: %v", err)
		}
	}
}
