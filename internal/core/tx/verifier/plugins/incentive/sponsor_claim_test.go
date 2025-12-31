// Package incentive_test 提供 SponsorClaimPlugin 的单元测试
//
// 🧪 **测试规范遵循**：
// - 每个源文件对应一个测试文件
// - 遵循测试规范：docs/system/standards/principles/testing-standards.md
package incentive

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weisyn/v1/internal/core/tx/ports/hash"
	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxopb "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/constants"
)

// ==================== SponsorClaimPlugin 测试 ====================

// createDelegationLock 创建测试用的 DelegationLock
func createDelegationLock(authorizedOps []string) *transaction.LockingCondition {
	return &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_DelegationLock{
			DelegationLock: &transaction.DelegationLock{
				AuthorizedOperations: authorizedOps,
			},
		},
	}
}

// createSingleKeyLockWithAddress 创建使用地址哈希的 SingleKeyLock（用于匹配矿工地址）
func createSingleKeyLockWithAddress(address []byte) *transaction.LockingCondition {
	return &transaction.LockingCondition{
		Condition: &transaction.LockingCondition_SingleKeyLock{
			SingleKeyLock: &transaction.SingleKeyLock{
				KeyRequirement: &transaction.SingleKeyLock_RequiredAddressHash{
					RequiredAddressHash: address,
				},
			},
		},
	}
}

// createDelegationProof 创建测试用的 DelegationProof
func createDelegationProof(operationType string, delegateAddr []byte, valueAmount uint64, signature []byte) *transaction.DelegationProof {
	proof := &transaction.DelegationProof{
		OperationType:  operationType,
		DelegateAddress: delegateAddr,
		ValueAmount:     valueAmount,
	}
	if signature != nil {
		proof.DelegateSignature = &transaction.SignatureData{
			Value: signature,
		}
	}
	return proof
}

// createSponsorUTXO 创建测试用的赞助池 UTXO
func createSponsorUTXO(amount string) (*utxopb.UTXO, *transaction.OutPoint) {
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(
		constants.SponsorPoolOwner[:],
		amount,
		createDelegationLock([]string{"consume"}),
	)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	return utxo, outpoint
}

// TestNewSponsorClaimPlugin 测试创建 SponsorClaimPlugin
func TestNewSponsorClaimPlugin(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	assert.NotNil(t, plugin)
	assert.Equal(t, utxoQuery, plugin.eutxoQuery)
	assert.Equal(t, sigManager, plugin.sigManager)
	assert.Equal(t, hashManager, plugin.hashManager)
	assert.Equal(t, canonicalizer, plugin.hashCanonicalizer)
}

// TestSponsorClaimPlugin_Name 测试插件名称
func TestSponsorClaimPlugin_Name(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	assert.Equal(t, "SponsorClaimValidator", plugin.Name())
}

// TestSponsorClaimPlugin_Check_NonSponsorClaim 测试非赞助领取交易（跳过）
func TestSponsorClaimPlugin_Check_NonSponsorClaim(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	// 创建非赞助领取交易（多个输入）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 1),
				IsReferenceOnly: false,
			},
		},
		nil,
	)

	err := plugin.Check(context.Background(), nil, tx.Outputs, tx)

	assert.NoError(t, err) // 非赞助领取交易应该跳过
}

// TestSponsorClaimPlugin_Check_NoDelegationProof 测试没有 DelegationProof（跳过）
func TestSponsorClaimPlugin_Check_NoDelegationProof(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	// 创建交易（1输入但没有 DelegationProof）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
			},
		},
		nil,
	)

	sponsorUTXO, _ := createSponsorUTXO("1000")
	inputs := []*utxopb.UTXO{sponsorUTXO}

	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 没有 DelegationProof 应该跳过
}

// TestSponsorClaimPlugin_Check_NonSponsorPoolUTXO 测试非赞助池 UTXO（跳过）
func TestSponsorClaimPlugin_Check_NonSponsorPoolUTXO(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	// 创建非赞助池 UTXO
	outpoint := testutil.CreateOutPoint(nil, 0)
	output := testutil.CreateNativeCoinOutput(
		testutil.RandomAddress(), // 不是赞助池地址
		"1000",
		createDelegationLock([]string{"consume"}),
	)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		nil,
	)

	inputs := []*utxopb.UTXO{utxo}

	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err) // 非赞助池 UTXO 应该跳过
}

// TestSponsorClaimPlugin_Check_Success 测试赞助领取验证成功
func TestSponsorClaimPlugin_Check_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	// 创建交易（1输入+DelegationProof，输出：矿工领取500+找零500回池）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			// Output[0]: 矿工领取 500（使用地址哈希锁定）
			{
				Owner: minerAddr,
				LockingConditions: []*transaction.LockingCondition{
					createSingleKeyLockWithAddress(minerAddr),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "500",
							},
						},
					},
				},
			},
			// Output[1]: 找零 500 回池
			{
				Owner: constants.SponsorPoolOwner[:],
				LockingConditions: []*transaction.LockingCondition{
					createDelegationLock([]string{"consume"}),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "500",
							},
						},
					},
				},
			},
		},
	)

	inputs := []*utxopb.UTXO{sponsorUTXO}

	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.NoError(t, err)
}

// TestSponsorClaimPlugin_Check_InvalidOutputCount 测试无效的输出数量
func TestSponsorClaimPlugin_Check_InvalidOutputCount(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	// 创建交易（3个输出，无效）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(minerAddr, "500", testutil.CreateSingleKeyLock(nil)),
			testutil.CreateNativeCoinOutput(minerAddr, "300", testutil.CreateSingleKeyLock(nil)),
			testutil.CreateNativeCoinOutput(minerAddr, "200", testutil.CreateSingleKeyLock(nil)),
		},
	)

	inputs := []*utxopb.UTXO{sponsorUTXO}

	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "输出验证失败")
}

// TestSponsorClaimPlugin_Check_InvalidMinerAddress 测试无效的矿工地址
func TestSponsorClaimPlugin_Check_InvalidMinerAddress(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	wrongAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	// 创建交易（Output[0] 使用错误的地址）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			// Output[0]: 使用错误的地址
			{
				Owner: wrongAddr,
				LockingConditions: []*transaction.LockingCondition{
					createSingleKeyLockWithAddress(wrongAddr),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "500",
							},
						},
					},
				},
			},
		},
	)

	inputs := []*utxopb.UTXO{sponsorUTXO}

	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "输出验证失败")
}

// TestSponsorClaimPlugin_Check_NonConservation 测试金额不守恒
func TestSponsorClaimPlugin_Check_NonConservation(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 600, nil) // 匹配输出金额

	// 创建交易（输入1000，输出600，不守恒）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			{
				Owner: minerAddr,
				LockingConditions: []*transaction.LockingCondition{
					createSingleKeyLockWithAddress(minerAddr),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "600",
							},
						},
					},
				},
			},
		},
	)

	inputs := []*utxopb.UTXO{sponsorUTXO}

	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "金额守恒验证失败")
}

// TestSponsorClaimPlugin_Check_ValueAmountMismatch 测试 ValueAmount 不匹配
func TestSponsorClaimPlugin_Check_ValueAmountMismatch(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil) // Proof 中指定 500

	// 创建交易（实际领取 600，与 Proof 不一致）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			{
				Owner: minerAddr,
				LockingConditions: []*transaction.LockingCondition{
					createSingleKeyLockWithAddress(minerAddr),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "600",
							},
						},
					},
				},
			},
			{
				Owner: constants.SponsorPoolOwner[:],
				LockingConditions: []*transaction.LockingCondition{
					createDelegationLock([]string{"consume"}),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "400",
							},
						},
					},
				},
			},
		},
	)

	inputs := []*utxopb.UTXO{sponsorUTXO}

	err := plugin.Check(context.Background(), inputs, tx.Outputs, tx)

	// 注意：由于金额守恒（1000 = 600 + 400），但 ValueAmount 不匹配（500 != 600）
	// 应该返回错误，说明领取金额与Proof不一致
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "领取金额与Proof不一致")
	}
}

// TestSponsorClaimPlugin_Verify_Success 测试 Verify 方法成功
func TestSponsorClaimPlugin_Verify_Success(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	utxoQuery.AddUTXO(sponsorUTXO)

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			{
				Owner: minerAddr,
				LockingConditions: []*transaction.LockingCondition{
					createSingleKeyLockWithAddress(minerAddr),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "500",
							},
						},
					},
				},
			},
			{
				Owner: constants.SponsorPoolOwner[:],
				LockingConditions: []*transaction.LockingCondition{
					createDelegationLock([]string{"consume"}),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "500",
							},
						},
					},
				},
			},
		},
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.NoError(t, err)
}

// TestSponsorClaimPlugin_Verify_InvalidEnvironment 测试无效的验证环境
func TestSponsorClaimPlugin_Verify_InvalidEnvironment(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	// 创建有 DelegationProof 的交易（否则会跳过）
	minerAddr := testutil.RandomAddress()
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  testutil.CreateOutPoint(nil, 0),
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		nil,
	)

	// 传入无效的环境类型
	env := "invalid environment"

	err := plugin.Verify(context.Background(), tx, env)

	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "环境类型错误")
	}
}

// TestSponsorClaimPlugin_Verify_ReferenceOnlyInput 测试引用型输入（应该失败）
func TestSponsorClaimPlugin_Verify_ReferenceOnlyInput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	utxoQuery.AddUTXO(sponsorUTXO)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: true, // 引用型输入
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须为消费模式")
}

// TestSponsorClaimPlugin_Verify_NoDelegationLock 测试缺少 DelegationLock
func TestSponsorClaimPlugin_Verify_NoDelegationLock(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	// 创建没有 DelegationLock 的 UTXO
	output := testutil.CreateNativeCoinOutput(
		constants.SponsorPoolOwner[:],
		"1000",
		testutil.CreateSingleKeyLock(nil), // 使用 SingleKeyLock 而不是 DelegationLock
	)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	utxoQuery.AddUTXO(utxo)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "缺少DelegationLock")
}

// TestSponsorClaimPlugin_Verify_NoConsumeAuthorization 测试未授权 consume 操作
func TestSponsorClaimPlugin_Verify_NoConsumeAuthorization(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	// 创建未授权 consume 的 DelegationLock
	output := testutil.CreateNativeCoinOutput(
		constants.SponsorPoolOwner[:],
		"1000",
		createDelegationLock([]string{"transfer"}), // 只授权 transfer，不授权 consume
	)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	utxoQuery.AddUTXO(utxo)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未授权consume操作")
}

// TestSponsorClaimPlugin_Verify_InvalidOperationType 测试无效的操作类型
func TestSponsorClaimPlugin_Verify_InvalidOperationType(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("transfer", minerAddr, 500, nil) // 使用 transfer 而不是 consume

	utxoQuery.AddUTXO(sponsorUTXO)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须使用consume操作")
}

// TestSponsorClaimPlugin_Verify_InvalidDelegateAddress 测试无效的 DelegateAddress
func TestSponsorClaimPlugin_Verify_InvalidDelegateAddress(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	wrongAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", wrongAddr, 500, nil) // 使用错误的地址

	utxoQuery.AddUTXO(sponsorUTXO)

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须是矿工地址")
}

// TestSponsorClaimPlugin_Verify_UTXONotFound 测试 UTXO 不存在
func TestSponsorClaimPlugin_Verify_UTXONotFound(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	// 不添加 UTXO 到 utxoQuery

	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		nil,
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "查询赞助UTXO失败")
}

// TestSponsorClaimPlugin_Verify_NoChangeOutput 测试没有找零输出
func TestSponsorClaimPlugin_Verify_NoChangeOutput(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 1000, nil) // 全部领取

	utxoQuery.AddUTXO(sponsorUTXO)

	// 创建交易（只有1个输出，全部领取）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			{
				Owner: minerAddr,
				LockingConditions: []*transaction.LockingCondition{
					createSingleKeyLockWithAddress(minerAddr),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "1000",
							},
						},
					},
				},
			},
		},
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.NoError(t, err) // 没有找零输出是合法的（全部领取）
}

// TestSponsorClaimPlugin_Verify_ChangeOutputInvalidOwner 测试找零输出 Owner 错误
func TestSponsorClaimPlugin_Verify_ChangeOutputInvalidOwner(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	utxoQuery.AddUTXO(sponsorUTXO)

	// 创建交易（找零输出的 Owner 不是赞助池地址）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			testutil.CreateNativeCoinOutput(minerAddr, "500", testutil.CreateSingleKeyLock(nil)),
			{
				Owner: testutil.RandomAddress(), // 错误的 Owner
				LockingConditions: []*transaction.LockingCondition{
					createDelegationLock([]string{"consume"}),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "500",
							},
						},
					},
				},
			},
		},
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "输出验证失败")
}

// TestSponsorClaimPlugin_Verify_ChangeOutputNoDelegationLock 测试找零输出缺少 DelegationLock
func TestSponsorClaimPlugin_Verify_ChangeOutputNoDelegationLock(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	sponsorUTXO, outpoint := createSponsorUTXO("1000")
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	utxoQuery.AddUTXO(sponsorUTXO)

	// 创建交易（找零输出缺少 DelegationLock）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			{
				Owner: minerAddr,
				LockingConditions: []*transaction.LockingCondition{
					createSingleKeyLockWithAddress(minerAddr),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "500",
							},
						},
					},
				},
			},
			{
				Owner: constants.SponsorPoolOwner[:],
				LockingConditions: []*transaction.LockingCondition{
					testutil.CreateSingleKeyLock(nil), // 使用 SingleKeyLock 而不是 DelegationLock
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_NativeCoin{
							NativeCoin: &transaction.NativeCoinAsset{
								Amount: "500",
							},
						},
					},
				},
			},
		},
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "输出验证失败")
}

// TestSponsorClaimPlugin_Verify_ContractToken 测试合约代币
func TestSponsorClaimPlugin_Verify_ContractToken(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	contractAddr := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	// 创建合约代币 UTXO
	output := &transaction.TxOutput{
		Owner: constants.SponsorPoolOwner[:],
		LockingConditions: []*transaction.LockingCondition{
			createDelegationLock([]string{"consume"}),
		},
		OutputContent: &transaction.TxOutput_Asset{
			Asset: &transaction.AssetOutput{
				AssetContent: &transaction.AssetOutput_ContractToken{
					ContractToken: &transaction.ContractTokenAsset{
						ContractAddress: contractAddr,
						TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
							FungibleClassId: []byte("token"),
						},
						Amount: "1000",
					},
				},
			},
		},
	}
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	utxoQuery.AddUTXO(utxo)

	// 创建交易
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			// Output[0]: 矿工领取 500（使用地址哈希锁定）
			{
				Owner: minerAddr,
				LockingConditions: []*transaction.LockingCondition{
					createSingleKeyLockWithAddress(minerAddr),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_ContractToken{
							ContractToken: &transaction.ContractTokenAsset{
								ContractAddress: contractAddr,
								TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
									FungibleClassId: []byte("token"),
								},
								Amount: "500",
							},
						},
					},
				},
			},
			// Output[1]: 找零 500 回池
			{
				Owner: constants.SponsorPoolOwner[:],
				LockingConditions: []*transaction.LockingCondition{
					createDelegationLock([]string{"consume"}),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_ContractToken{
							ContractToken: &transaction.ContractTokenAsset{
								ContractAddress: contractAddr,
								TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
									FungibleClassId: []byte("token"),
								},
								Amount: "500",
							},
						},
					},
				},
			},
		},
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.NoError(t, err)
}

// TestSponsorClaimPlugin_Verify_AssetTypeMismatch 测试资产类型不匹配
func TestSponsorClaimPlugin_Verify_AssetTypeMismatch(t *testing.T) {
	utxoQuery := testutil.NewMockUTXOQuery()
	sigManager := testutil.NewTestSignatureManager()
	hashManager := testutil.NewTestHashManager()
	canonicalizer := hash.NewCanonicalizer(nil)

	plugin := NewSponsorClaimPlugin(utxoQuery, sigManager, hashManager, canonicalizer)

	minerAddr := testutil.RandomAddress()
	contractAddr := testutil.RandomAddress()
	outpoint := testutil.CreateOutPoint(nil, 0)
	// 创建原生币 UTXO
	output := testutil.CreateNativeCoinOutput(
		constants.SponsorPoolOwner[:],
		"1000",
		createDelegationLock([]string{"consume"}),
	)
	utxo := testutil.CreateUTXO(outpoint, output, utxopb.UTXOLifecycleStatus_UTXO_LIFECYCLE_AVAILABLE)
	delegationProof := createDelegationProof("consume", minerAddr, 500, nil)

	utxoQuery.AddUTXO(utxo)

	// 创建交易（输出使用合约代币，类型不匹配）
	tx := testutil.CreateTransaction(
		[]*transaction.TxInput{
			{
				PreviousOutput:  outpoint,
				IsReferenceOnly: false,
				UnlockingProof: &transaction.TxInput_DelegationProof{
					DelegationProof: delegationProof,
				},
			},
		},
		[]*transaction.TxOutput{
			{
				Owner: minerAddr,
				LockingConditions: []*transaction.LockingCondition{
					createSingleKeyLockWithAddress(minerAddr),
				},
				OutputContent: &transaction.TxOutput_Asset{
					Asset: &transaction.AssetOutput{
						AssetContent: &transaction.AssetOutput_ContractToken{
							ContractToken: &transaction.ContractTokenAsset{
								ContractAddress: contractAddr,
								TokenIdentifier: &transaction.ContractTokenAsset_FungibleClassId{
									FungibleClassId: []byte("token"),
								},
								Amount: "500",
							},
						},
					},
				},
			},
		},
	)

	env := &MockVerifierEnvironment{
		minerAddress: minerAddr,
		utxoQuery:    utxoQuery,
	}

	err := plugin.Verify(context.Background(), tx, env)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "金额守恒验证失败")
}

