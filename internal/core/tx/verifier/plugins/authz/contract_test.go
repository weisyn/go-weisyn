// Package authz_test 提供 ContractPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件（contract.go → contract_test.go）
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// buildTestExecutionProof 构建测试用的 ExecutionProof
//
// 辅助函数：用于测试中快速构建 ExecutionProof
func buildTestExecutionProof(
	resourceAddress []byte,
	methodName string,
	inputDataHash []byte,
	outputDataHash []byte,
	executionTimeMs uint64,
	executionResultHash []byte,
	stateTransitionProof []byte,
) *transaction.ExecutionProof {
	if len(resourceAddress) == 0 {
		resourceAddress = testutil.RandomBytes(20)
	}
	if len(inputDataHash) == 0 {
		inputDataHash = testutil.RandomBytes(32)
	}
	if len(outputDataHash) == 0 {
		outputDataHash = testutil.RandomBytes(32)
	}
	if len(executionResultHash) == 0 {
		executionResultHash = testutil.RandomBytes(32)
	}
	if len(stateTransitionProof) == 0 {
		stateTransitionProof = testutil.RandomBytes(64)
	}
	
	metadata := make(map[string][]byte)
	if methodName != "" {
		metadata["method_name"] = []byte(methodName)
	}
	
	return &transaction.ExecutionProof{
		ExecutionResultHash:  executionResultHash,
		StateTransitionProof: stateTransitionProof,
		ExecutionTimeMs:      executionTimeMs,
		Context: &transaction.ExecutionProof_ExecutionContext{
			CallerIdentity: &transaction.IdentityProof{
				PublicKey:     testutil.RandomBytes(33),
				CallerAddress: testutil.RandomBytes(20),
				Signature:     testutil.RandomBytes(64),
				Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
				Nonce:         testutil.RandomBytes(32),
				Timestamp:     1234567890,
				ContextHash:   testutil.RandomBytes(32),
			},
			ResourceAddress: resourceAddress,
			ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
			InputDataHash:   inputDataHash,
			OutputDataHash:  outputDataHash,
			// ⚠️ **边界原则**：ExecutionProof 不应该包含 Transaction 级别的信息
			// - value_sent：已移除，应该从 Transaction 的 inputs/outputs 中计算
			// - transaction_hash：已移除，应该从 Transaction 本身获取
			// - timestamp：已移除，应该使用 Transaction.creation_timestamp
			Metadata: metadata,
		},
	}
}

// TestNewContractPlugin 测试创建 ContractPlugin
func TestNewContractPlugin(t *testing.T) {
	plugin := NewContractPlugin()

	assert.NotNil(t, plugin)
}

// TestContractPlugin_Name 测试插件名称
func TestContractPlugin_Name(t *testing.T) {
	plugin := NewContractPlugin()

	assert.Equal(t, "contract", plugin.Name())
}

// TestContractPlugin_Match_NotContractLock 测试不匹配其他锁类型
func TestContractPlugin_Match_NotContractLock(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: buildTestExecutionProof(nil, "", nil, nil, 0, nil, nil),
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.False(t, matched)
}

// TestContractPlugin_Match_MissingProof 测试缺少 proof
func TestContractPlugin_Match_MissingProof(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: nil,
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "missing execution proof")
}

// TestContractPlugin_Match_EmptyContractAddress 测试空合约地址
func TestContractPlugin_Match_EmptyContractAddress(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: nil, // 空地址
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: buildTestExecutionProof(
				testutil.RandomBytes(20),
				"test",
				testutil.RandomBytes(32),
				testutil.RandomBytes(32),
				0,
				testutil.RandomBytes(32),
				testutil.RandomBytes(64),
			),
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "invalid contract address")
}

// TestContractPlugin_Match_MethodNameMismatch 测试方法名不匹配
func TestContractPlugin_Match_MethodNameMismatch(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
				RequiredMethod:  "verify",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: buildTestExecutionProof(
				testutil.RandomBytes(20),
				"transfer", // 不同的方法名
				testutil.RandomBytes(32),
				testutil.RandomBytes(32),
				0,
				testutil.RandomBytes(32),
				testutil.RandomBytes(64),
			),
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "method name mismatch")
}

// TestContractPlugin_Match_MissingMethodName 测试缺少方法名
func TestContractPlugin_Match_MissingMethodName(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
				RequiredMethod:  "verify",
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: buildTestExecutionProof(
				testutil.RandomBytes(20),
				"", // 缺少方法名（空字符串）
				testutil.RandomBytes(32),
				testutil.RandomBytes(32),
				0,
				testutil.RandomBytes(32),
				testutil.RandomBytes(64),
			),
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "missing method name")
}

// TestContractPlugin_Match_ExecutionTimeExceedsLimit 测试执行时间超过限制
func TestContractPlugin_Match_ExecutionTimeExceedsLimit(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress:   testutil.RandomBytes(20),
				MaxExecutionTimeMs: 1000,
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: buildTestExecutionProof(
				testutil.RandomBytes(20),
				"test",
				testutil.RandomBytes(32),
				testutil.RandomBytes(32),
				2000, // 超过限制
				testutil.RandomBytes(32),
				testutil.RandomBytes(64),
			),
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "execution time exceeds limit")
}

// TestContractPlugin_Match_MissingExecutionResultHash 测试缺少执行结果哈希
func TestContractPlugin_Match_MissingExecutionResultHash(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				ExecutionResultHash:  nil, // 缺少执行结果哈希
				StateTransitionProof: testutil.RandomBytes(64),
				ExecutionTimeMs:      0,
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     testutil.RandomBytes(33),
						CallerAddress: testutil.RandomBytes(20),
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: testutil.RandomBytes(20),
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32),
					OutputDataHash:  testutil.RandomBytes(32),
					// ⚠️ **边界原则**：value_sent 已移除，应该从 Transaction 的 inputs/outputs 中计算
					// ⚠️ **边界原则**：ExecutionProof 不应该包含 Transaction 级别的信息
					// - transaction_hash：已移除，应该从 Transaction 本身获取
					// - timestamp：已移除，应该使用 Transaction.creation_timestamp
					Metadata: map[string][]byte{"method_name": []byte("test")},
				},
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "missing execution result hash")
}

// TestContractPlugin_Match_MissingStateTransitionProof 测试缺少状态转换证明
func TestContractPlugin_Match_MissingStateTransitionProof(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: nil, // 缺少状态转换证明
				ExecutionTimeMs:      0,
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     testutil.RandomBytes(33),
						CallerAddress: testutil.RandomBytes(20),
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: testutil.RandomBytes(20),
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32),
					OutputDataHash:  testutil.RandomBytes(32),
					// ⚠️ **边界原则**：value_sent 已移除，应该从 Transaction 的 inputs/outputs 中计算
					// ⚠️ **边界原则**：ExecutionProof 不应该包含 Transaction 级别的信息
					// - transaction_hash：已移除，应该从 Transaction 本身获取
					// - timestamp：已移除，应该使用 Transaction.creation_timestamp
					Metadata: map[string][]byte{"method_name": []byte("test")},
				},
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "missing state transition proof")
}

// TestContractPlugin_Match_MissingInputDataHash 测试缺少输入数据哈希
func TestContractPlugin_Match_MissingInputDataHash(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
				ExecutionTimeMs:      0,
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     testutil.RandomBytes(33),
						CallerAddress: testutil.RandomBytes(20),
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: testutil.RandomBytes(20),
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   nil, // 缺少输入数据哈希
					OutputDataHash:  testutil.RandomBytes(32),
					// ⚠️ **边界原则**：value_sent 已移除，应该从 Transaction 的 inputs/outputs 中计算
					// ⚠️ **边界原则**：ExecutionProof 不应该包含 Transaction 级别的信息
					// - transaction_hash：已移除，应该从 Transaction 本身获取
					// - timestamp：已移除，应该使用 Transaction.creation_timestamp
					Metadata: map[string][]byte{"method_name": []byte("test")},
				},
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "input_data_hash")
}

// TestContractPlugin_Match_Success 测试成功匹配
func TestContractPlugin_Match_Success(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
				RequiredMethod:  "verify",
				MaxExecutionTimeMs: 5000,
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: buildTestExecutionProof(
				testutil.RandomBytes(20),
				"verify",
				testutil.RandomBytes(32),
				testutil.RandomBytes(32),
				1000,
				testutil.RandomBytes(32),
				testutil.RandomBytes(64),
			),
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestContractPlugin_Match_Success_NoRequiredMethod 测试成功匹配（无必需方法）
func TestContractPlugin_Match_Success_NoRequiredMethod(t *testing.T) {
	plugin := NewContractPlugin()

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
				RequiredMethod:  "", // 无必需方法
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: buildTestExecutionProof(
				testutil.RandomBytes(20),
				"any_method",
				testutil.RandomBytes(32),
				testutil.RandomBytes(32),
				0,
				testutil.RandomBytes(32),
				testutil.RandomBytes(64),
			),
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

