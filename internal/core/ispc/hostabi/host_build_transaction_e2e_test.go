package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
)

// ============================================================================
// BuildTransactionFromDraft 端到端测试
// ============================================================================
//
// 🎯 **测试目的**：发现 BuildTransactionFromDraft 的缺陷和BUG
//
// ============================================================================

// TestBuildTransactionFromDraft_DeferSign_Success 测试defer_sign模式成功构建
func TestBuildTransactionFromDraft_DeferSign_Success(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"inputs": [{"tx_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "output_index": 0}],
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.NoError(t, err, "应该成功构建交易")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "unsigned", receipt.Mode, "模式应该正确")
	assert.NotEmpty(t, receipt.UnsignedTxHash, "应该包含交易哈希")
	assert.Equal(t, 1, mockTxAdapter.beginTransactionCallCount, "应该调用BeginTransaction")
	assert.Equal(t, 1, mockTxAdapter.addCustomInputCallCount, "应该添加输入")
	assert.Equal(t, 1, mockTxAdapter.addCustomOutputCallCount, "应该添加输出")
	assert.Equal(t, 1, mockTxAdapter.finalizeTransactionCallCount, "应该Finalize交易")
}

// TestBuildTransactionFromDraft_Delegated_Success 测试delegated模式成功构建
func TestBuildTransactionFromDraft_Delegated_Success(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()
	callerAddress := make([]byte, 20)

	draftJSON := `{
		"sign_mode": "delegated",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}],
		"metadata": {
			"delegation_params": {
				"original_owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				"allowed_delegates": ["beefdeadbeefdeadbeefdeadbeefdeadbeefdead"],
				"authorized_operations": ["transfer"],
				"expiry_duration_blocks": 100,
				"max_value_per_operation": "1000"
			}
		}
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil,           // eutxoQuery
		callerAddress, // callerAddress
		nil,           // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.NoError(t, err, "应该成功构建交易")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "delegated", receipt.Mode, "模式应该正确")
}

// TestBuildTransactionFromDraft_Threshold_Success 测试threshold模式成功构建
func TestBuildTransactionFromDraft_Threshold_Success(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()

	key1 := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	key2 := "beefdeadbeefdeadbeefdeadbeefdeadbeefdead"
	key3 := "feedbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	draftJSON := `{
		"sign_mode": "threshold",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}],
		"metadata": {
			"threshold_params": {
				"threshold": 2,
				"total_parties": 3,
				"party_verification_keys": ["` + key1 + `", "` + key2 + `", "` + key3 + `"],
				"signature_scheme": "BLS_THRESHOLD",
				"security_level": 256
			}
		}
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.NoError(t, err, "应该成功构建交易")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "threshold", receipt.Mode, "模式应该正确")
}

// TestBuildTransactionFromDraft_Paymaster_Success 测试paymaster模式成功构建（检查修复后的UTXO选择）
func TestBuildTransactionFromDraft_Paymaster_Success(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
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
										Amount: "200", // 金额足够支付费用100
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

	draftJSON := `{
		"sign_mode": "paymaster",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}],
		"metadata": {
			"paymaster_params": {
				"fee_amount": "100",
				"token_id": "",
				"miner_addr": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
			}
		}
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		mockUTXOQuery, // eutxoQuery
		nil,           // callerAddress
		nil,           // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.NoError(t, err, "应该成功构建交易")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "paymaster", receipt.Mode, "模式应该正确")
	assert.Equal(t, 1, mockTxAdapter.addCustomInputCallCount, "应该添加赞助池输入")
	assert.Equal(t, 2, mockTxAdapter.addCustomOutputCallCount, "应该添加费用输出和业务输出")
}

// TestBuildTransactionFromDraft_WithIntents 测试带意图的交易构建
func TestBuildTransactionFromDraft_WithIntents(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"intents": [{
			"type": "transfer",
			"params": {
				"from": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				"to": "beefdeadbeefdeadbeefdeadbeefdeadbeefdead",
				"amount": "1000",
				"token_id": ""
			}
		}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.NoError(t, err, "应该成功构建交易")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, 1, mockTxAdapter.addTransferCallCount, "应该处理转账意图")
}

// TestBuildTransactionFromDraft_ParseError 测试解析错误
func TestBuildTransactionFromDraft_ParseError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	invalidJSON := `{invalid json}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(invalidJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "解析 Draft JSON 失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_ValidateError 测试验证错误
func TestBuildTransactionFromDraft_ValidateError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "invalid_mode",
		"inputs": [{"tx_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "output_index": 0}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "验证 Draft JSON 失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_BeginTransactionError 测试创建Draft错误
func TestBuildTransactionFromDraft_BeginTransactionError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		beginTransactionError: assert.AnError,
	}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"inputs": [{"tx_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "output_index": 0}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "创建 Draft 失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_ProcessIntentError 测试处理意图错误
func TestBuildTransactionFromDraft_ProcessIntentError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		addTransferError: assert.AnError,
	}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"intents": [{
			"type": "transfer",
			"params": {
				"from": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				"to": "beefdeadbeefdeadbeefdeadbeefdeadbeefdead",
				"amount": "1000"
			}
		}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "处理意图失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_AddInputError 测试添加输入错误
func TestBuildTransactionFromDraft_AddInputError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		addCustomInputError: assert.AnError,
	}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"inputs": [{"tx_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "output_index": 0}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "添加输入失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_BuildOutputError 测试构建输出错误
func TestBuildTransactionFromDraft_BuildOutputError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"outputs": [{"type": "asset", "owner": "invalid", "amount": "1000"}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "构建输出失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_AddOutputError 测试添加输出错误
func TestBuildTransactionFromDraft_AddOutputError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		addCustomOutputError: assert.AnError,
	}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "添加输出失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_ApplySignModeLogicError 测试应用签名模式逻辑错误
func TestBuildTransactionFromDraft_ApplySignModeLogicError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	// 缺少delegation_params
	draftJSON := `{
		"sign_mode": "delegated",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil,              // eutxoQuery
		nil,              // callerAddress
		make([]byte, 20), // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "应用签名模式逻辑失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_FinalizeError 测试Finalize错误
func TestBuildTransactionFromDraft_FinalizeError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{
		finalizeTransactionError: assert.AnError,
	}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "完成交易构建失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_RouteError 测试路由错误
func TestBuildTransactionFromDraft_RouteError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		err: assert.AnError, // 计算哈希失败
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}]
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		nil, // eutxoQuery
		nil, // callerAddress
		nil, // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	// routeBySignMode 会返回错误收据，但错误信息在 receipt.Error 中
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
}

// TestBuildTransactionFromDraft_Paymaster_InsufficientUTXO 测试paymaster模式UTXO金额不足
func TestBuildTransactionFromDraft_Paymaster_InsufficientUTXO(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{}
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
										Amount: "50", // 金额不足支付费用100
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

	draftJSON := `{
		"sign_mode": "paymaster",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}],
		"metadata": {
			"paymaster_params": {
				"fee_amount": "100",
				"token_id": ""
			}
		}
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		mockUTXOQuery, // eutxoQuery
		nil,           // callerAddress
		nil,           // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "金额足够的UTXO", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_Paymaster_NoAssetUTXO 测试paymaster模式没有Asset类型UTXO
func TestBuildTransactionFromDraft_Paymaster_NoAssetUTXO(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		sponsorUTXOs: []*utxo.UTXO{
			{
				Outpoint: &pb.OutPoint{
					TxId:        make([]byte, 32),
					OutputIndex: 0,
				},
				Category: utxo.UTXOCategory_UTXO_CATEGORY_RESOURCE, // 非Asset类型
			},
		},
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "paymaster",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}],
		"metadata": {
			"paymaster_params": {
				"fee_amount": "100",
				"token_id": ""
			}
		}
	}`

	receipt, err := BuildTransactionFromDraft(
		ctx,
		mockTxAdapter,
		mockTxHashClient,
		mockUTXOQuery, // eutxoQuery
		nil,           // callerAddress
		nil,           // contractAddress
		[]byte(draftJSON),
		100,
		1000,
	)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "金额足够的UTXO", "错误信息应该正确")
}
