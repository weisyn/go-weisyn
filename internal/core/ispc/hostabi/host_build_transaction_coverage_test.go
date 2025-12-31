package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// host_build_transaction.go 覆盖率提升测试
// ============================================================================
//
// 🎯 **测试目的**：提高覆盖率，发现未覆盖的代码路径中的缺陷和BUG
//
// ============================================================================

// TestApplyDelegationLock_GetDraftError 测试获取Draft错误
func TestApplyDelegationLock_GetDraftError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		getDraftError: assert.AnError,
	}
	ctx := context.Background()
	callerAddress := make([]byte, 20)
	params := &DelegationParams{
		OriginalOwner:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		AllowedDelegates:     []string{"beefdeadbeefdeadbeefdeadbeefdeadbeefdead"},
		AuthorizedOperations: []string{"transfer"},
		ExpiryDurationBlocks: 100,
		MaxValuePerOperation: "1000",
	}

	err := applyDelegationLock(ctx, mockTxAdapter, callerAddress, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "获取Draft失败", "错误信息应该正确")
}

// TestApplyDelegationLock_InvalidOriginalOwnerLength 测试无效原始所有者地址长度
func TestApplyDelegationLock_InvalidOriginalOwnerLength(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	callerAddress := make([]byte, 20)
	params := &DelegationParams{
		OriginalOwner:        "invalid", // 长度不足
		AllowedDelegates:     []string{"beefdeadbeefdeadbeefdeadbeefdeadbeefdead"},
		AuthorizedOperations: []string{"transfer"},
		MaxValuePerOperation: "1000",
	}

	err := applyDelegationLock(ctx, mockTxAdapter, callerAddress, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "original_owner地址长度错误", "错误信息应该正确")
}

// TestApplyDelegationLock_InvalidDelegateLength 测试无效委托者地址长度
func TestApplyDelegationLock_InvalidDelegateLength(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	callerAddress := make([]byte, 20)
	params := &DelegationParams{
		OriginalOwner:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		AllowedDelegates:     []string{"invalid"}, // 长度不足
		AuthorizedOperations: []string{"transfer"},
		MaxValuePerOperation: "1000",
	}

	err := applyDelegationLock(ctx, mockTxAdapter, callerAddress, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "allowed_delegate地址长度错误", "错误信息应该正确")
}

// TestApplyDelegationLock_InvalidMaxValue 测试无效最大金额
func TestApplyDelegationLock_InvalidMaxValue(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	callerAddress := make([]byte, 20)
	params := &DelegationParams{
		OriginalOwner:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		AllowedDelegates:     []string{"beefdeadbeefdeadbeefdeadbeefdeadbeefdead"},
		AuthorizedOperations: []string{"transfer"},
		MaxValuePerOperation: "invalid", // 无效金额
	}

	err := applyDelegationLock(ctx, mockTxAdapter, callerAddress, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "解析max_value_per_operation失败", "错误信息应该正确")
}

// TestApplyDelegationLock_WithExpiryAndPolicy 测试带过期时间和策略的委托锁定
func TestApplyDelegationLock_WithExpiryAndPolicy(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		draft: &types.DraftTx{
			DraftID: "draft-123",
			Tx: &pb.Transaction{
				Outputs: []*pb.TxOutput{
					{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	callerAddress := make([]byte, 20)
	params := &DelegationParams{
		OriginalOwner:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		AllowedDelegates:     []string{"beefdeadbeefdeadbeefdeadbeefdeadbeefdead"},
		AuthorizedOperations: []string{"transfer"},
		ExpiryDurationBlocks: 100,
		MaxValuePerOperation: "1000",
		DelegationPolicy:     "test_policy",
	}

	err := applyDelegationLock(ctx, mockTxAdapter, callerAddress, 1, params, 100)

	assert.NoError(t, err, "应该成功应用委托锁定")
	assert.Equal(t, 1, mockTxAdapter.getDraftCallCount, "应该调用GetDraft")
}

// TestApplyDelegationLock_NoAssetOutputs 测试没有Asset输出的情况
func TestApplyDelegationLock_NoAssetOutputs(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		draft: &types.DraftTx{
			DraftID: "draft-123",
			Tx: &pb.Transaction{
				Outputs: []*pb.TxOutput{
					{
						OutputContent: &pb.TxOutput_Resource{
							Resource: &pb.ResourceOutput{},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	callerAddress := make([]byte, 20)
	params := &DelegationParams{
		OriginalOwner:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		AllowedDelegates:     []string{"beefdeadbeefdeadbeefdeadbeefdeadbeefdead"},
		AuthorizedOperations: []string{"transfer"},
		MaxValuePerOperation: "1000",
	}

	err := applyDelegationLock(ctx, mockTxAdapter, callerAddress, 1, params, 100)

	assert.NoError(t, err, "应该成功（没有Asset输出时不会修改）")
}

// TestApplyThresholdLock_GetDraftError 测试获取Draft错误
func TestApplyThresholdLock_GetDraftError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		getDraftError: assert.AnError,
	}
	ctx := context.Background()
	key1 := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	params := &ThresholdParams{
		Threshold:              2,
		TotalParties:           3,
		PartyVerificationKeys:  []string{key1},
		SignatureScheme:       "BLS_THRESHOLD",
		SecurityLevel:          256,
	}

	err := applyThresholdLock(ctx, mockTxAdapter, 1, params)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "获取Draft失败", "错误信息应该正确")
}

// TestApplyThresholdLock_InvalidKeyDecode 测试无效密钥解码
func TestApplyThresholdLock_InvalidKeyDecode(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	params := &ThresholdParams{
		Threshold:              2,
		TotalParties:           3,
		PartyVerificationKeys:  []string{"invalid_hex"}, // 无效十六进制
		SignatureScheme:       "BLS_THRESHOLD",
		SecurityLevel:          256,
	}

	err := applyThresholdLock(ctx, mockTxAdapter, 1, params)

	// decodeHex 对无效输入返回空数组，len(keyBytes) == 0 会触发错误
	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "party_verification_key解码失败", "错误信息应该正确")
}

// TestApplyThresholdLock_WithPolicy 测试带策略的门限锁定
func TestApplyThresholdLock_WithPolicy(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		draft: &types.DraftTx{
			DraftID: "draft-123",
			Tx: &pb.Transaction{
				Outputs: []*pb.TxOutput{
					{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	key1 := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	params := &ThresholdParams{
		Threshold:              2,
		TotalParties:           3,
		PartyVerificationKeys:  []string{key1},
		SignatureScheme:       "BLS_THRESHOLD",
		SecurityLevel:          256,
		ThresholdPolicy:       "test_policy",
	}

	err := applyThresholdLock(ctx, mockTxAdapter, 1, params)

	assert.NoError(t, err, "应该成功应用门限锁定")
}

// TestApplyThresholdLock_NoAssetOutputs 测试没有Asset输出的情况
func TestApplyThresholdLock_NoAssetOutputs(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		draft: &types.DraftTx{
			DraftID: "draft-123",
			Tx: &pb.Transaction{
				Outputs: []*pb.TxOutput{
					{
						OutputContent: &pb.TxOutput_State{
							State: &pb.StateOutput{},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	key1 := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	params := &ThresholdParams{
		Threshold:              2,
		TotalParties:           3,
		PartyVerificationKeys:  []string{key1},
		SignatureScheme:       "BLS_THRESHOLD",
		SecurityLevel:          256,
	}

	err := applyThresholdLock(ctx, mockTxAdapter, 1, params)

	assert.NoError(t, err, "应该成功（没有Asset输出时不会修改）")
}

// TestHandleDelegatedMode_ComputeHashError 测试计算哈希错误
func TestHandleDelegatedMode_ComputeHashError(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		err: assert.AnError,
	}
	ctx := context.Background()
	unsignedTx := &pb.Transaction{}

	receipt, err := handleDelegatedMode(ctx, mockTxHashClient, unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "failed to compute transaction hash", "错误信息应该正确")
}

// TestHandleDelegatedMode_InvalidTransaction 测试无效交易结构
func TestHandleDelegatedMode_InvalidTransaction(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: false, // 交易结构无效
	}
	ctx := context.Background()
	unsignedTx := &pb.Transaction{}

	receipt, err := handleDelegatedMode(ctx, mockTxHashClient, unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "transaction structure is invalid", "错误信息应该正确")
}

// TestHandleThresholdMode_ComputeHashError 测试计算哈希错误
func TestHandleThresholdMode_ComputeHashError(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		err: assert.AnError,
	}
	ctx := context.Background()
	unsignedTx := &pb.Transaction{}

	receipt, err := handleThresholdMode(ctx, mockTxHashClient, unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "failed to compute transaction hash", "错误信息应该正确")
}

// TestHandleThresholdMode_InvalidTransaction 测试无效交易结构
func TestHandleThresholdMode_InvalidTransaction(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: false, // 交易结构无效
	}
	ctx := context.Background()
	unsignedTx := &pb.Transaction{}

	receipt, err := handleThresholdMode(ctx, mockTxHashClient, unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "transaction structure is invalid", "错误信息应该正确")
}

// TestHandlePaymasterMode_ComputeHashError 测试计算哈希错误
func TestHandlePaymasterMode_ComputeHashError(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		err: assert.AnError,
	}
	ctx := context.Background()
	unsignedTx := &pb.Transaction{}

	receipt, err := handlePaymasterMode(ctx, mockTxHashClient, unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "failed to compute transaction hash", "错误信息应该正确")
}

// TestHandlePaymasterMode_InvalidTransaction 测试无效交易结构
func TestHandlePaymasterMode_InvalidTransaction(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: false, // 交易结构无效
	}
	ctx := context.Background()
	unsignedTx := &pb.Transaction{}

	receipt, err := handlePaymasterMode(ctx, mockTxHashClient, unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "transaction structure is invalid", "错误信息应该正确")
}

// TestBuildTxOutputFromSpec_ResourceOutput 测试构建资源输出
func TestBuildTxOutputFromSpec_ResourceOutput(t *testing.T) {
	contentHashHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	metadataJSON := `{
		"content_hash": "` + contentHashHex + `",
		"category": "wasm",
		"mime_type": "application/wasm"
	}`
	spec := &OutputSpec{
		Type:     "resource",
		Owner:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Metadata: []byte(metadataJSON),
	}

	output, err := buildTxOutputFromSpec(spec, nil)

	assert.NoError(t, err, "应该成功构建")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.OutputContent, "应该有输出内容")
	
	// 验证是资源输出
	resourceOutput, ok := output.OutputContent.(*pb.TxOutput_Resource)
	assert.True(t, ok, "应该是资源输出")
	assert.NotNil(t, resourceOutput.Resource, "资源输出应该不为nil")
}

// TestBuildTxOutputFromSpec_StateOutput 测试构建状态输出
func TestBuildTxOutputFromSpec_StateOutput(t *testing.T) {
	executionResultHashHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	stateIDHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	metadataJSON := `{
		"state_id": "` + stateIDHex + `",
		"state_version": 1,
		"execution_result_hash": "` + executionResultHashHex + `"
	}`
	spec := &OutputSpec{
		Type:     "state",
		Owner:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Metadata: []byte(metadataJSON),
	}

	output, err := buildTxOutputFromSpec(spec, nil)

	assert.NoError(t, err, "应该成功构建")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.OutputContent, "应该有输出内容")
	
	// 验证是状态输出
	stateOutput, ok := output.OutputContent.(*pb.TxOutput_State)
	assert.True(t, ok, "应该是状态输出")
	assert.NotNil(t, stateOutput.State, "状态输出应该不为nil")
}

// TestBuildTxOutputFromSpec_NilSpec 测试nil spec
func TestBuildTxOutputFromSpec_NilSpec(t *testing.T) {
	output, err := buildTxOutputFromSpec(nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, output, "输出应该为nil")
	assert.Contains(t, err.Error(), "outputSpec 不能为空", "错误信息应该正确")
}

// TestApplyPaymaster_ParseFeeAmountError 测试解析费用金额错误
func TestApplyPaymaster_ParseFeeAmountError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		sponsorUTXOs: []*utxo.UTXO{
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 0,
				},
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "200",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	params := &PaymasterParams{
		FeeAmount: "invalid_amount", // 无效金额
	}

	err := applyPaymaster(ctx, mockTxAdapter, mockUTXOQuery, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "解析费用金额失败", "错误信息应该正确")
}

// TestApplyPaymaster_NoCachedOutput 测试没有缓存输出的UTXO
func TestApplyPaymaster_NoCachedOutput(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		sponsorUTXOs: []*utxo.UTXO{
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 0,
				},
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				// 没有CachedOutput
			},
		},
	}
	ctx := context.Background()
	params := &PaymasterParams{
		FeeAmount: "100",
	}

	err := applyPaymaster(ctx, mockTxAdapter, mockUTXOQuery, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "金额足够的UTXO", "错误信息应该正确")
}

// TestApplyPaymaster_NoNativeCoin 测试没有原生币的UTXO
func TestApplyPaymaster_NoNativeCoin(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		sponsorUTXOs: []*utxo.UTXO{
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 0,
				},
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_ContractToken{
									ContractToken: &pb.ContractTokenAsset{
										Amount: "200",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	params := &PaymasterParams{
		FeeAmount: "100",
	}

	err := applyPaymaster(ctx, mockTxAdapter, mockUTXOQuery, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "金额足够的UTXO", "错误信息应该正确")
}

// TestApplyPaymaster_InvalidAmountParse 测试UTXO金额解析失败
func TestApplyPaymaster_InvalidAmountParse(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		sponsorUTXOs: []*utxo.UTXO{
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 0,
				},
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "invalid_amount", // 无效金额
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	params := &PaymasterParams{
		FeeAmount: "100",
	}

	err := applyPaymaster(ctx, mockTxAdapter, mockUTXOQuery, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "金额足够的UTXO", "错误信息应该正确")
}

// TestApplyPaymaster_AddInputError 测试添加输入错误
func TestApplyPaymaster_AddInputError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		addCustomInputError: assert.AnError,
	}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		sponsorUTXOs: []*utxo.UTXO{
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 0,
				},
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "200",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	params := &PaymasterParams{
		FeeAmount: "100",
	}

	err := applyPaymaster(ctx, mockTxAdapter, mockUTXOQuery, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "添加赞助池输入失败", "错误信息应该正确")
}

// TestApplyPaymaster_AddOutputError 测试添加输出错误
func TestApplyPaymaster_AddOutputError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		addCustomOutputError: assert.AnError,
	}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		sponsorUTXOs: []*utxo.UTXO{
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 0,
				},
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "200",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	params := &PaymasterParams{
		FeeAmount: "100",
	}

	err := applyPaymaster(ctx, mockTxAdapter, mockUTXOQuery, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "添加费用输出失败", "错误信息应该正确")
}

// TestApplyPaymaster_NilOutpoint 测试nil Outpoint
func TestApplyPaymaster_NilOutpoint(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		sponsorUTXOs: []*utxo.UTXO{
			{
				Outpoint: nil, // nil Outpoint
				Category: utxo.UTXOCategory_UTXO_CATEGORY_ASSET,
				ContentStrategy: &utxo.UTXO_CachedOutput{
					CachedOutput: &pb.TxOutput{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_NativeCoin{
									NativeCoin: &pb.NativeCoinAsset{
										Amount: "200",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ctx := context.Background()
	params := &PaymasterParams{
		FeeAmount: "100",
	}

	err := applyPaymaster(ctx, mockTxAdapter, mockUTXOQuery, 1, params, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "Outpoint为空", "错误信息应该正确")
}

