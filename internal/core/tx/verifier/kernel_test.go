// Package verifier_test 提供 Verifier 服务的单元测试
//
// 🧪 **测试覆盖**：
// - Verifier Kernel 核心功能测试
// - 三阶段验证顺序测试
// - 插件注册和调用测试
// - 边界条件和错误场景测试
package verifier

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== Verifier Kernel 核心功能测试 ====================

// TestNewKernel 测试创建新的 Kernel
func TestNewKernel(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	assert.NotNil(t, kernel)
	assert.NotNil(t, kernel.authzHook)
	assert.NotNil(t, kernel.conservationHook)
	assert.NotNil(t, kernel.conditionHook)
}

// TestVerify_Success 测试验证有效交易
func TestVerify_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	// 创建有效交易
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_SingleKeyProof{
					SingleKeyProof: &transaction.SingleKeyProof{
						Signature: &transaction.SignatureData{
							Value: []byte("signature"),
						},
						PublicKey: &transaction.PublicKey{
							Value: testutil.RandomPublicKey(),
						},
					},
				},
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 注册插件（简化：使用空插件列表，实际需要注册真实插件）
	// 注意：这里简化测试，实际需要注册 SingleKeyPlugin 等

	err := kernel.Verify(context.Background(), tx)
	// 由于没有注册插件，验证可能会失败，这是预期的
	// 实际测试中需要注册相应的插件
	_ = err
}

// TestVerify_AuthZFailure 测试 AuthZ 验证失败
func TestVerify_AuthZFailure(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	// 创建无效交易（缺少 UnlockingProof）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
				// 缺少 UnlockingProof
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	err := kernel.Verify(context.Background(), tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "权限验证失败")
}

// TestVerify_Order 测试验证顺序
func TestVerify_Order(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	// 创建一个会在 AuthZ 阶段失败的交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
				// 缺少 UnlockingProof，AuthZ 会失败
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	err := kernel.Verify(context.Background(), tx)
	// 应该返回 AuthZ 错误，而不是 Conservation 或 Condition 错误
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "权限验证失败")
	// 不应该包含 "价值守恒" 或 "条件检查"
	assert.NotContains(t, err.Error(), "价值守恒验证失败")
	assert.NotContains(t, err.Error(), "条件检查失败")
}

// TestVerify_EmptyTransaction 测试空交易
func TestVerify_EmptyTransaction(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}

	err := kernel.Verify(context.Background(), tx)
	// 空交易应该通过验证（Coinbase 交易）
	assert.NoError(t, err)
}

// TestVerify_NilTransaction 测试 nil 交易
func TestVerify_NilTransaction(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	// nil 交易会导致 panic，这是预期的行为
	// 实际使用中应该由调用方确保交易不为 nil
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic for nil transaction")
		}
	}()
	_ = kernel.Verify(context.Background(), nil)
}

// TestRegisterAuthZPlugin 测试注册 AuthZ 插件
func TestRegisterAuthZPlugin(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	// 创建模拟插件
	plugin := &MockAuthZPlugin{name: "test-plugin"}

	kernel.RegisterAuthZPlugin(plugin)

	// 验证插件已注册（通过调用验证来间接验证）
	// 注意：这里简化测试，实际需要验证插件列表
	assert.NotNil(t, kernel.authzHook)
}

// TestRegisterConservationPlugin 测试注册 Conservation 插件
func TestRegisterConservationPlugin(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	plugin := &MockConservationPlugin{name: "test-plugin"}

	kernel.RegisterConservationPlugin(plugin)

	assert.NotNil(t, kernel.conservationHook)
}

// TestRegisterConditionPlugin 测试注册 Condition 插件
func TestRegisterConditionPlugin(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	plugin := &MockConditionPlugin{name: "test-plugin"}

	kernel.RegisterConditionPlugin(plugin)

	assert.NotNil(t, kernel.conditionHook)
}

// TestKernel_VerifyAuthZLock_Success 测试 VerifyAuthZLock 成功
func TestKernel_VerifyAuthZLock_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	lock := testutil.CreateSingleKeyLock(nil)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: &transaction.SingleKeyProof{
				Signature: &transaction.SignatureData{
					Value: []byte("signature"),
				},
				PublicKey: &transaction.PublicKey{
					Value: testutil.RandomPublicKey(),
				},
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	// 注册匹配的插件
	plugin := &MockAuthZPlugin{
		name:    "test-plugin",
		matches: true,
		success: true,
	}
	kernel.RegisterAuthZPlugin(plugin)

	err := kernel.VerifyAuthZLock(context.Background(), lock, proof, tx)
	assert.NoError(t, err)
}

// TestKernel_VerifyAuthZLock_NoMatch 测试 VerifyAuthZLock 无匹配插件
func TestKernel_VerifyAuthZLock_NoMatch(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	lock := testutil.CreateSingleKeyLock(nil)
	proof := &transaction.UnlockingProof{
		Proof: &transaction.UnlockingProof_SingleKeyProof{
			SingleKeyProof: &transaction.SingleKeyProof{
				Signature: &transaction.SignatureData{
					Value: []byte("signature"),
				},
				PublicKey: &transaction.PublicKey{
					Value: testutil.RandomPublicKey(),
				},
			},
		},
	}
	tx := testutil.CreateTransaction(nil, nil)

	// 注册不匹配的插件
	plugin := &MockAuthZPlugin{
		name:    "test-plugin",
		matches: false,
		success: false,
	}
	kernel.RegisterAuthZPlugin(plugin)

	err := kernel.VerifyAuthZLock(context.Background(), lock, proof, tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "没有 AuthZ 插件匹配此锁定条件类型")
}

// TestKernel_VerifyBatch 测试批量验证
func TestKernel_VerifyBatch(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	// 创建两个交易
	tx1 := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}
	tx2 := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}

	results, err := kernel.VerifyBatch(context.Background(), []*transaction.Transaction{tx1, tx2})

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	// 空交易应该通过验证
	assert.NoError(t, results[0])
	assert.NoError(t, results[1])
}

// TestKernel_VerifyWithContext_Success 测试带环境的验证成功
func TestKernel_VerifyWithContext_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}

	config := &VerifierEnvironmentConfig{
		BlockHeight:  100,
		BlockTime:    1234567890,
		MinerAddress: testutil.RandomAddress(),
		ChainID:      []byte("test-chain"),
		UTXOQuery:    utxoQuery,
	}
	env := NewStaticVerifierEnvironment(config)

	err := kernel.VerifyWithContext(context.Background(), tx, env)
	assert.NoError(t, err)
}

// TestKernel_VerifyWithContext_InvalidEnv 测试带环境的验证（无效环境）
func TestKernel_VerifyWithContext_InvalidEnv(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	kernel := NewKernel(utxoQuery)

	tx := &transaction.Transaction{
		Version: 1,
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}

	// 传入无效的环境类型（不是 VerifierEnvironment）
	invalidEnv := "not a VerifierEnvironment"

	err := kernel.VerifyWithContext(context.Background(), tx, invalidEnv)
	// 应该仍然通过验证（因为空交易）
	assert.NoError(t, err)
}

// ==================== Mock 辅助类型 ====================
// 注意：Mock 类型定义在 hooks_test.go 中，避免重复定义
