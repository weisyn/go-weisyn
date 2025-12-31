// Package verifier_test 提供 Verifier Hooks 的单元测试
//
// 🧪 **测试覆盖**：
// - AuthZ Hook 测试
// - Conservation Hook 测试
// - Condition Hook 测试
package verifier

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ==================== AuthZ Hook 测试 ====================

// TestNewAuthZHook 测试创建 AuthZ Hook
func TestNewAuthZHook(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	hook := NewAuthZHook(utxoQuery)

	assert.NotNil(t, hook)
	assert.NotNil(t, hook.plugins)
	assert.Empty(t, hook.plugins)
	assert.Equal(t, utxoQuery, hook.eutxoQuery)
}

// TestAuthZHook_Register 测试注册插件
func TestAuthZHook_Register(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	hook := NewAuthZHook(utxoQuery)

	plugin := &MockAuthZPlugin{name: "test-plugin"}
	hook.Register(plugin)

	assert.Len(t, hook.plugins, 1)
	assert.Equal(t, plugin, hook.plugins[0])
}

// TestAuthZHook_Verify_Success 测试验证成功
func TestAuthZHook_Verify_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	hook := NewAuthZHook(utxoQuery)

	// 准备 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建交易
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

	// 注册匹配的插件
	plugin := &MockAuthZPlugin{
		name:    "test-plugin",
		matches: true,
		success: true,
	}
	hook.Register(plugin)

	err := hook.Verify(context.Background(), tx)
	assert.NoError(t, err)
}

// TestAuthZHook_Verify_UTXONotFound 测试 UTXO 不存在
func TestAuthZHook_Verify_UTXONotFound(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	hook := NewAuthZHook(utxoQuery)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	err := hook.Verify(context.Background(), tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "获取 UTXO 失败")
}

// TestAuthZHook_Verify_NoMatchingPlugin 测试无匹配插件
func TestAuthZHook_Verify_NoMatchingPlugin(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	hook := NewAuthZHook(utxoQuery)

	// 准备 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 注册不匹配的插件
	plugin := &MockAuthZPlugin{
		name:    "test-plugin",
		matches: false,
		success: false,
	}
	hook.Register(plugin)

	err := hook.Verify(context.Background(), tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "没有 AuthZ 插件匹配此锁定条件类型")
}

// ==================== Conservation Hook 测试 ====================

// TestNewConservationHook 测试创建 Conservation Hook
func TestNewConservationHook(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	hook := NewConservationHook(utxoQuery)

	assert.NotNil(t, hook)
	assert.NotNil(t, hook.plugins)
	assert.Empty(t, hook.plugins)
	assert.Equal(t, utxoQuery, hook.eutxoQuery)
}

// TestConservationHook_Register 测试注册插件
func TestConservationHook_Register(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	hook := NewConservationHook(utxoQuery)

	plugin := &MockConservationPlugin{name: "test-plugin"}
	hook.Register(plugin)

	assert.Len(t, hook.plugins, 1)
	assert.Equal(t, plugin, hook.plugins[0])
}

// TestConservationHook_Verify_Success 测试验证成功
func TestConservationHook_Verify_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	hook := NewConservationHook(utxoQuery)

	// 准备 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	// 创建交易（输出 < 输入，符合守恒）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 注册验证通过的插件
	plugin := &MockConservationPlugin{
		name:    "test-plugin",
		success: true,
	}
	hook.Register(plugin)

	err := hook.Verify(context.Background(), tx)
	assert.NoError(t, err)
}

// TestConservationHook_Verify_Failure 测试验证失败
func TestConservationHook_Verify_Failure(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	hook := NewConservationHook(utxoQuery)

	// 准备 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil))
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	utxoQuery.AddUTXO(utxo)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "900", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 注册验证失败的插件
	plugin := &MockConservationPlugin{
		name:    "test-plugin",
		success: false,
		err:     "价值不守恒",
	}
	hook.Register(plugin)

	err := hook.Verify(context.Background(), tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "插件 test-plugin 验证失败")
}

// ==================== Condition Hook 测试 ====================

// TestNewConditionHook 测试创建 Condition Hook
func TestNewConditionHook(t *testing.T) {
	hook := NewConditionHook()

	assert.NotNil(t, hook)
	assert.NotNil(t, hook.plugins)
	assert.Empty(t, hook.plugins)
}

// TestConditionHook_Register 测试注册插件
func TestConditionHook_Register(t *testing.T) {
	hook := NewConditionHook()

	plugin := &MockConditionPlugin{name: "test-plugin"}
	hook.Register(plugin)

	assert.Len(t, hook.plugins, 1)
	assert.Equal(t, plugin, hook.plugins[0])
}

// TestConditionHook_Verify_Success 测试验证成功
func TestConditionHook_Verify_Success(t *testing.T) {
	hook := NewConditionHook()

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 注册验证通过的插件
	plugin := &MockConditionPlugin{
		name:    "test-plugin",
		success: true,
	}
	hook.Register(plugin)

	err := hook.Verify(context.Background(), tx, 100, 1000)
	assert.NoError(t, err)
}

// TestConditionHook_Verify_Failure 测试验证失败
func TestConditionHook_Verify_Failure(t *testing.T) {
	hook := NewConditionHook()

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(testutil.RandomAddress(), "1000", testutil.CreateSingleKeyLock(nil)),
		},
	)

	// 注册验证失败的插件
	plugin := &MockConditionPlugin{
		name:    "test-plugin",
		success: false,
		err:     "条件不满足",
	}
	hook.Register(plugin)

	err := hook.Verify(context.Background(), tx, 100, 1000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "插件 test-plugin 验证失败")
}

// ==================== Mock 辅助类型 ====================

// MockAuthZPlugin 模拟 AuthZ 插件（用于测试）
type MockAuthZPlugin struct {
	name    string
	matches bool
	success bool
	err     error
}

func (m *MockAuthZPlugin) Name() string {
	return m.name
}

func (m *MockAuthZPlugin) Match(ctx context.Context, lock *transaction.LockingCondition, proof *transaction.UnlockingProof, tx *transaction.Transaction) (bool, error) {
	if !m.matches {
		return false, nil
	}
	if m.success {
		return true, nil
	}
	if m.err != nil {
		return true, m.err
	}
	return true, assert.AnError
}

// MockConservationPlugin 模拟 Conservation 插件（用于测试）
type MockConservationPlugin struct {
	name    string
	success bool
	err     string
}

func (m *MockConservationPlugin) Name() string {
	return m.name
}

func (m *MockConservationPlugin) Verify(ctx context.Context, tx *transaction.Transaction, utxoFetcher func(*transaction.OutPoint) (*utxopb.UTXO, error)) error {
	if m.success {
		return nil
	}
	if m.err != "" {
		return assert.AnError
	}
	return nil
}

func (m *MockConservationPlugin) Check(ctx context.Context, inputs []*utxopb.UTXO, outputs []*transaction.TxOutput, tx *transaction.Transaction) error {
	if m.success {
		return nil
	}
	if m.err != "" {
		return fmt.Errorf("%s", m.err)
	}
	return nil
}

// MockConditionPlugin 模拟 Condition 插件（用于测试）
type MockConditionPlugin struct {
	name    string
	success bool
	err     string
}

func (m *MockConditionPlugin) Name() string {
	return m.name
}

func (m *MockConditionPlugin) Check(ctx context.Context, tx *transaction.Transaction, blockHeight uint64, blockTime uint64) error {
	if m.success {
		return nil
	}
	if m.err != "" {
		return fmt.Errorf("%s", m.err)
	}
	return nil
}
