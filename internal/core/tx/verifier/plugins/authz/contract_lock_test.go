// Package authz_test 提供 ContractLockPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
)

// ==================== Mock AddressManager（用于 ContractLockPlugin） ====================
//
// 说明：
// - ContractLockPlugin 需要 addressManager 来执行 public_key -> address 推导与比对；
// - 测试侧只需提供确定性返回即可，不需要真实的 Base58Check/RIPEMD160 实现。
type MockAddressManager struct {
	addressToBytesMap map[string][]byte
	err               error
}

func newTestAddressManager() *MockAddressManager {
	m := &MockAddressManager{addressToBytesMap: make(map[string][]byte)}
	// 默认给一个 20 字节地址哈希，满足插件对 caller_address 的比较
	m.addressToBytesMap["Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn"] = make([]byte, 20)
	return m
}

func (m *MockAddressManager) PrivateKeyToAddress(privateKey []byte) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn", nil
}

func (m *MockAddressManager) PublicKeyToAddress(publicKey []byte) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn", nil
}

func (m *MockAddressManager) StringToAddress(addressStr string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return addressStr, nil
}

func (m *MockAddressManager) ValidateAddress(address string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return len(address) > 0, nil
}

func (m *MockAddressManager) AddressToBytes(address string) ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	if b, ok := m.addressToBytesMap[address]; ok {
		return b, nil
	}
	return make([]byte, 20), nil
}

func (m *MockAddressManager) BytesToAddress(addressBytes []byte) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn", nil
}

func (m *MockAddressManager) AddressToHexString(address string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "0000000000000000000000000000000000000000", nil
}

func (m *MockAddressManager) HexStringToAddress(hexStr string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "Cf1Kes6snEUeykiJJgrAtKPNPrAzPdPmSn", nil
}

func (m *MockAddressManager) GetAddressType(address string) (crypto.AddressType, error) {
	if m.err != nil {
		return crypto.AddressTypeInvalid, m.err
	}
	return crypto.AddressTypeBitcoin, nil
}

func (m *MockAddressManager) CompareAddresses(addr1, addr2 string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return addr1 == addr2, nil
}

func (m *MockAddressManager) IsZeroAddress(address string) bool {
	return address == "" || address == "0000000000000000000000000000000000000000"
}

// ==================== ContractLockPlugin 测试 ====================

// TestNewContractLockPlugin 测试创建 ContractLockPlugin
func TestNewContractLockPlugin(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	assert.NotNil(t, plugin)
	assert.Equal(t, "authz.contract_lock", plugin.Name())
}

// prepareExecutionProofForTest 根据插件逻辑计算 ExecutionContext 哈希，确保 IdentityProof 与上下文一致
func prepareExecutionProofForTest(plugin *ContractLockPlugin, execProof *transaction.ExecutionProof) {
	if plugin == nil || execProof == nil || execProof.Context == nil {
		return
	}
	identity := execProof.Context.CallerIdentity
	if identity == nil {
		return
	}
	// 计算 ContextHash
	identity.ContextHash = plugin.computeExecutionContextHash(execProof.Context)
}

// prepareCallerAddressFromPublicKey 从 PublicKey 推导 CallerAddress（用于测试）
func prepareCallerAddressFromPublicKey(addressManager crypto.AddressManager, identity *transaction.IdentityProof) {
	if addressManager == nil || identity == nil || len(identity.PublicKey) == 0 {
		return
	}
	// 从 PublicKey 推导地址
	addrStr, err := addressManager.PublicKeyToAddress(identity.PublicKey)
	if err != nil {
		return
	}
	addrBytes, err := addressManager.AddressToBytes(addrStr)
	if err != nil || len(addrBytes) != 20 {
		return
	}
	identity.CallerAddress = addrBytes
}

// TestContractLockPlugin_Match_NotContractLock 测试不匹配其他锁类型
func TestContractLockPlugin_Match_NotContractLock(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{},
		},
	}
	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
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
					Metadata:        make(map[string][]byte),
				},
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.False(t, matched)
}

// TestContractLockPlugin_Match_MissingProof 测试缺少 proof
func TestContractLockPlugin_Match_MissingProof(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

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
	assert.Contains(t, err.Error(), "ExecutionProof is nil")
}

// TestContractLockPlugin_Match_NilProofContext 测试 proof context 为 nil
func TestContractLockPlugin_Match_NilProofContext(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

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
				Context: nil, // nil context
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "ExecutionProof.Context is nil")
}

// TestContractLockPlugin_Match_ExecutionTimeExceedsLimit 测试执行时间超过限制
func TestContractLockPlugin_Match_ExecutionTimeExceedsLimit(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress:    testutil.RandomBytes(20),
				MaxExecutionTimeMs: 1000,
			},
		},
	}
	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: lock.GetContractLock().ContractAddress, // ✅ 匹配的合约地址
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionTimeMs:      2000, // 超过限制
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "execution time")
}

// TestContractLockPlugin_Match_CallerNotAllowed 测试调用者不在允许列表中
func TestContractLockPlugin_Match_CallerNotAllowed(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	allowedCaller := testutil.RandomAddress()
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
				AllowedCallers:  []string{string(allowedCaller)},
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     testutil.RandomBytes(33),
						CallerAddress: testutil.RandomAddress(), // 不同的调用者
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: lock.GetContractLock().ContractAddress, // ✅ 匹配的合约地址
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "caller")
}

// TestContractLockPlugin_Match_InvalidExecutionResultHashLength 测试执行结果哈希长度无效
func TestContractLockPlugin_Match_InvalidExecutionResultHashLength(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
			},
		},
	}
	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: lock.GetContractLock().ContractAddress, // ✅ 匹配的合约地址
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionResultHash:  testutil.RandomBytes(16), // 长度不是32
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "execution_result_hash length")
}

// TestContractLockPlugin_Match_MissingStateTransitionProof 测试缺少状态转换证明
func TestContractLockPlugin_Match_MissingStateTransitionProof(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
			},
		},
	}
	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: lock.GetContractLock().ContractAddress, // ✅ 匹配的合约地址
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: nil, // 缺少状态转换证明
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "state_transition_proof")
}

// TestContractLockPlugin_Match_ParameterHashMismatch 测试参数哈希不匹配
func TestContractLockPlugin_Match_ParameterHashMismatch(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	expectedParamHash := testutil.RandomBytes(32)
	differentParamHash := testutil.RandomBytes(32) // 不同的哈希
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
				ParameterHash:   expectedParamHash,
			},
		},
	}
	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: lock.GetContractLock().ContractAddress, // ✅ 匹配的合约地址
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   differentParamHash,       // ❌ 不匹配的哈希
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	// 应该失败，因为参数哈希不匹配
	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "parameter_hash")
}

// TestContractLockPlugin_Match_Success 测试成功匹配
func TestContractLockPlugin_Match_Success(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
			},
		},
	}
	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: lock.GetContractLock().ContractAddress, // ✅ 匹配的合约地址
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestContractLockPlugin_Match_Success_WithAllowedCaller 测试成功匹配（有允许的调用者）
func TestContractLockPlugin_Match_Success_WithAllowedCaller(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	contractAddr := testutil.RandomBytes(20)
	publicKey := testutil.RandomBytes(33)
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: contractAddr,
			},
		},
	}
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: lock.GetContractLock().ContractAddress,
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	// 设置允许的调用者列表
	lock.GetContractLock().AllowedCallers = []string{string(proof.GetExecutionProof().Context.CallerIdentity.CallerAddress)}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())

	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestContainsCaller 测试 containsCaller 辅助函数
func TestContainsCaller(t *testing.T) {
	caller1 := []byte{1, 2, 3}
	caller2 := []byte{4, 5, 6}
	caller3 := []byte{7, 8, 9}
	allowedCallers := []string{string(caller1), string(caller2)}

	assert.True(t, containsCaller(allowedCallers, caller1))
	assert.True(t, containsCaller(allowedCallers, caller2))
	assert.False(t, containsCaller(allowedCallers, caller3))
	assert.False(t, containsCaller(nil, caller1))
	assert.False(t, containsCaller(allowedCallers, nil))
}

// TestContractLockPlugin_Match_ContractAddressMismatch 测试合约地址不匹配
func TestContractLockPlugin_Match_ContractAddressMismatch(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	lockAddress := testutil.RandomBytes(20)
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: lockAddress,
			},
		},
	}
	// 使用不同的合约地址
	differentAddress := testutil.RandomBytes(20)
	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: differentAddress, // ❌ 不匹配的合约地址
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "contract address mismatch")
}

// TestContractLockPlugin_Match_MissingContractAddress 测试缺少合约地址
func TestContractLockPlugin_Match_MissingContractAddress(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: testutil.RandomBytes(20),
			},
		},
	}
	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
					},
					// ❌ 缺少 ResourceAddress
					ExecutionType:  transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash: testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:       map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "resource_address missing")
}

// TestContractLockPlugin_Match_ContractAddressMatch 测试合约地址匹配成功
func TestContractLockPlugin_Match_ContractAddressMatch(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	lockAddress := testutil.RandomBytes(20)
	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: lockAddress,
			},
		},
	}
	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: lockAddress, // ✅ 匹配的合约地址
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("test")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())
	tx := testutil.CreateTransaction(nil, nil)

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestContractLockPlugin_Match_OutputContractTokenAddressMismatch 测试输出中 ContractTokenAsset.contract_address 不匹配
func TestContractLockPlugin_Match_OutputContractTokenAddressMismatch(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	contractAddress := testutil.RandomBytes(20)
	wrongContractAddress := testutil.RandomBytes(20)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: contractAddress,
			},
		},
	}

	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: contractAddress,
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("mint")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())

	tx := &transaction.Transaction{
		Outputs: []*transaction.TxOutput{
			{
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_ContractToken{
							ContractToken: &transaction.ContractTokenAsset{
								ContractAddress: wrongContractAddress,
								TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
									FungibleClassId: []byte("token123"),
								},
								Amount: "1000",
							},
						},
					},
				},
			},
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "ContractTokenAsset.contract_address mismatch")
}

// TestContractLockPlugin_Match_OutputContractTokenAddressMatch 测试输出中 ContractTokenAsset.contract_address 匹配成功
func TestContractLockPlugin_Match_OutputContractTokenAddressMatch(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	contractAddress := testutil.RandomBytes(20)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: contractAddress,
			},
		},
	}

	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: contractAddress,
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("mint")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())

	tx := &transaction.Transaction{
		Outputs: []*transaction.TxOutput{
			{
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_ContractToken{
							ContractToken: &transaction.ContractTokenAsset{
								ContractAddress: contractAddress,
								TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
									FungibleClassId: []byte("token123"),
								},
								Amount: "1000",
							},
						},
					},
				},
			},
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.NoError(t, err)
	assert.True(t, matched)
}

// TestContractLockPlugin_Match_OutputContractTokenEmptyAddress 测试输出中 ContractTokenAsset.contract_address 为空
func TestContractLockPlugin_Match_OutputContractTokenEmptyAddress(t *testing.T) {
	mockHashMgr := &testutil.MockHashManager{}
	mockSigMgr := &testutil.MockSignatureManager{}
	mockAddrMgr := newTestAddressManager()
	plugin := NewContractLockPlugin(mockHashMgr, mockSigMgr, mockAddrMgr)

	contractAddress := testutil.RandomBytes(20)

	lock := &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_ContractLock{
			ContractLock: &transaction.ContractLock{
				ContractAddress: contractAddress,
			},
		},
	}

	publicKey := testutil.RandomBytes(33)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_ExecutionProof{
			ExecutionProof: &transaction.ExecutionProof{
				Context: &transaction.ExecutionProof_ExecutionContext{
					CallerIdentity: &transaction.IdentityProof{
						PublicKey:     publicKey,
						CallerAddress: nil, // 将在下面从 PublicKey 推导
						Signature:     testutil.RandomBytes(64),
						Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
						SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
						Nonce:         testutil.RandomBytes(32),
						Timestamp:     1234567890,
						ContextHash:   testutil.RandomBytes(32),
					},
					ResourceAddress: contractAddress,
					ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
					InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
					Metadata:        map[string][]byte{"method_name": []byte("mint")},
				},
				ExecutionResultHash:  testutil.RandomBytes(32),
				StateTransitionProof: testutil.RandomBytes(64),
			},
		},
	}
	if proof.GetExecutionProof() != nil && proof.GetExecutionProof().Context != nil && proof.GetExecutionProof().Context.CallerIdentity != nil {
		prepareCallerAddressFromPublicKey(mockAddrMgr, proof.GetExecutionProof().Context.CallerIdentity)
	}
	prepareExecutionProofForTest(plugin, proof.GetExecutionProof())

	tx := &transaction.Transaction{
		Outputs: []*transaction.TxOutput{
			{
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_ContractToken{
							ContractToken: &transaction.ContractTokenAsset{
								ContractAddress: nil,
								TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
									FungibleClassId: []byte("token123"),
								},
								Amount: "1000",
							},
						},
					},
				},
			},
		},
	}

	matched, err := plugin.Match(context.Background(), lock, proof, tx)

	assert.Error(t, err)
	assert.True(t, matched)
	assert.Contains(t, err.Error(), "ContractTokenAsset.contract_address is empty")
}
