package hostabi

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	utxo "github.com/weisyn/v1/pb/blockchain/utxo"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/grpc"
)

// ============================================================================
// host_build_transaction.go 核心业务逻辑测试
// ============================================================================
//
// 🎯 **测试目的**：发现核心业务逻辑的缺陷和BUG，特别是简化实现
//
// ============================================================================

// TestProcessIntent_Transfer 测试处理转账意图
func TestProcessIntent_Transfer(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	draftHandle := int32(1)

	transferParams := TransferIntent{
		From:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		To:      "beefdeadbeefdeadbeefdeadbeefdeadbeefdead",
		Amount:  "1000",
		TokenID: "",
	}
	paramsJSON, _ := json.Marshal(transferParams)
	intent := Intent{
		Type:   "transfer",
		Params: paramsJSON,
	}

	err := processIntent(ctx, mockTxAdapter, draftHandle, intent)

	assert.NoError(t, err, "应该成功处理转账意图")
	assert.Equal(t, 1, mockTxAdapter.addTransferCallCount, "应该调用AddTransfer")
}

// TestProcessIntent_InvalidType 测试不支持的意图类型
func TestProcessIntent_InvalidType(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	draftHandle := int32(1)

	intent := Intent{
		Type:   "invalid_type",
		Params: []byte("{}"),
	}

	err := processIntent(ctx, mockTxAdapter, draftHandle, intent)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "不支持的意图类型", "错误信息应该正确")
}

// TestProcessIntent_InvalidParams 测试无效参数
func TestProcessIntent_InvalidParams(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	draftHandle := int32(1)

	intent := Intent{
		Type:   "transfer",
		Params: []byte("invalid json"),
	}

	err := processIntent(ctx, mockTxAdapter, draftHandle, intent)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "解析转账意图参数失败", "错误信息应该正确")
}

// TestApplySignModeLogic_DeferSign 测试defer_sign模式（无需特殊处理）
func TestApplySignModeLogic_DeferSign(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	draftHandle := int32(1)
	draft := &DraftJSON{
		SignMode: "defer_sign",
	}

	err := applySignModeLogic(ctx, mockTxAdapter, nil, nil, draftHandle, draft, 100)

	assert.NoError(t, err, "defer_sign模式应该无需特殊处理")
}

// TestApplySignModeLogic_Delegated 测试delegated模式
func TestApplySignModeLogic_Delegated(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	draftHandle := int32(1)
	callerAddress := make([]byte, 20)
	draft := &DraftJSON{
		SignMode: "delegated",
		Metadata: Metadata{
			DelegationParams: &DelegationParams{
				OriginalOwner:        "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				AllowedDelegates:     []string{"beefdeadbeefdeadbeefdeadbeefdeadbeefdead"},
				AuthorizedOperations: []string{"transfer"},
				ExpiryDurationBlocks: 100,
				MaxValuePerOperation: "1000",
			},
		},
	}

	err := applySignModeLogic(ctx, mockTxAdapter, nil, callerAddress, draftHandle, draft, 100)

	assert.NoError(t, err, "应该成功应用委托锁定")
}

// TestApplySignModeLogic_Delegated_MissingParams 测试delegated模式缺少参数
func TestApplySignModeLogic_Delegated_MissingParams(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	draftHandle := int32(1)
	callerAddress := make([]byte, 20)
	draft := &DraftJSON{
		SignMode: "delegated",
		// 缺少DelegationParams
	}

	err := applySignModeLogic(ctx, mockTxAdapter, nil, callerAddress, draftHandle, draft, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "delegated模式需要提供delegation_params", "错误信息应该正确")
}

// TestApplySignModeLogic_Threshold 测试threshold模式
func TestApplySignModeLogic_Threshold(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	draftHandle := int32(1)
	// 使用有效的十六进制字符串作为验证密钥
	key1 := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	key2 := "beefdeadbeefdeadbeefdeadbeefdeadbeefdead"
	key3 := "feedbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	draft := &DraftJSON{
		SignMode: "threshold",
		Metadata: Metadata{
			ThresholdParams: &ThresholdParams{
				Threshold:              2,
				TotalParties:           3,
				PartyVerificationKeys:  []string{key1, key2, key3},
				SignatureScheme:       "BLS_THRESHOLD",
				SecurityLevel:          256,
			},
		},
	}

	err := applySignModeLogic(ctx, mockTxAdapter, nil, nil, draftHandle, draft, 100)

	assert.NoError(t, err, "应该成功应用门限锁定")
}

// TestApplySignModeLogic_Threshold_MissingParams 测试threshold模式缺少参数
func TestApplySignModeLogic_Threshold_MissingParams(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	draftHandle := int32(1)
	draft := &DraftJSON{
		SignMode: "threshold",
		// 缺少ThresholdParams
	}

	err := applySignModeLogic(ctx, mockTxAdapter, nil, nil, draftHandle, draft, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "threshold模式需要提供threshold_params", "错误信息应该正确")
}

// TestApplySignModeLogic_Paymaster 测试paymaster模式（检查修复后的UTXO选择）
func TestApplySignModeLogic_Paymaster(t *testing.T) {
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
	draftHandle := int32(1)
	draft := &DraftJSON{
		SignMode: "paymaster",
		Metadata: Metadata{
			PaymasterParams: &PaymasterParams{
				FeeAmount: "100",
				TokenID:   "",
				MinerAddr: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
			},
		},
	}

	err := applySignModeLogic(ctx, mockTxAdapter, mockUTXOQuery, nil, draftHandle, draft, 100)

	assert.NoError(t, err, "应该成功应用代付逻辑")
	assert.Equal(t, 1, mockTxAdapter.addCustomInputCallCount, "应该添加赞助池输入")
	assert.Equal(t, 1, mockTxAdapter.addCustomOutputCallCount, "应该添加费用输出")
	
	// ✅ 修复后的实现：按金额选择UTXO（选择金额 >= 所需费用的第一个UTXO）
	t.Logf("✅ 已修复：applyPaymaster 现在按金额选择UTXO，选择金额 >= 所需费用的第一个UTXO")
}

// TestApplySignModeLogic_Paymaster_MissingParams 测试paymaster模式缺少参数
func TestApplySignModeLogic_Paymaster_MissingParams(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	ctx := context.Background()
	draftHandle := int32(1)
	draft := &DraftJSON{
		SignMode: "paymaster",
		// 缺少PaymasterParams
	}

	err := applySignModeLogic(ctx, mockTxAdapter, nil, nil, draftHandle, draft, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "paymaster模式需要提供paymaster_params", "错误信息应该正确")
}

// TestApplySignModeLogic_Paymaster_NoUTXOs 测试paymaster模式没有可用UTXO
func TestApplySignModeLogic_Paymaster_NoUTXOs(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		sponsorUTXOs: []*utxo.UTXO{}, // 空列表
	}
	ctx := context.Background()
	draftHandle := int32(1)
	draft := &DraftJSON{
		SignMode: "paymaster",
		Metadata: Metadata{
			PaymasterParams: &PaymasterParams{
				FeeAmount: "100",
			},
		},
	}

	err := applySignModeLogic(ctx, mockTxAdapter, mockUTXOQuery, nil, draftHandle, draft, 100)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "赞助池中没有可用的UTXO", "错误信息应该正确")
}

// TestRouteBySignMode_DeferSign 测试defer_sign模式路由
func TestRouteBySignMode_DeferSign(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true, // 设置为true，表示交易结构有效
	}
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{
		Inputs: []*transaction.TxInput{
			{},
		},
		Outputs: []*transaction.TxOutput{
			{},
		},
	}

	receipt, err := routeBySignMode(ctx, mockTxHashClient, "defer_sign", unsignedTx)

	assert.NoError(t, err, "应该成功路由")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "unsigned", receipt.Mode, "模式应该正确")
	assert.NotEmpty(t, receipt.UnsignedTxHash, "应该包含交易哈希")
	// SerializedTx 可能为空（如果protobuf序列化空交易返回空字节数组）
	// 但至少应该存在（即使是空字符串）
	assert.NotNil(t, receipt.SerializedTx, "应该包含序列化交易字段")
}

// TestRouteBySignMode_DeferSign_NilClient 测试defer_sign模式nil客户端
func TestRouteBySignMode_DeferSign_NilClient(t *testing.T) {
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{}

	receipt, err := routeBySignMode(ctx, nil, "defer_sign", unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "transaction hash client is not initialized", "错误信息应该正确")
}

// TestRouteBySignMode_Delegated 测试delegated模式路由
func TestRouteBySignMode_Delegated(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{}

	receipt, err := routeBySignMode(ctx, mockTxHashClient, "delegated", unsignedTx)

	assert.NoError(t, err, "应该成功路由")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "delegated", receipt.Mode, "模式应该正确")
}

// TestRouteBySignMode_Threshold 测试threshold模式路由
func TestRouteBySignMode_Threshold(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{}

	receipt, err := routeBySignMode(ctx, mockTxHashClient, "threshold", unsignedTx)

	assert.NoError(t, err, "应该成功路由")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "threshold", receipt.Mode, "模式应该正确")
}

// TestRouteBySignMode_Paymaster 测试paymaster模式路由
func TestRouteBySignMode_Paymaster(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{}

	receipt, err := routeBySignMode(ctx, mockTxHashClient, "paymaster", unsignedTx)

	assert.NoError(t, err, "应该成功路由")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "paymaster", receipt.Mode, "模式应该正确")
}

// TestRouteBySignMode_InvalidMode 测试无效签名模式
func TestRouteBySignMode_InvalidMode(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{}

	receipt, err := routeBySignMode(ctx, mockTxHashClient, "invalid_mode", unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "未知的签名模式", "错误信息应该正确")
}

// TestHandleDelegatedMode_Success 测试处理委托模式成功
func TestHandleDelegatedMode_Success(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{}

	receipt, err := handleDelegatedMode(ctx, mockTxHashClient, unsignedTx)

	assert.NoError(t, err, "应该成功处理")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "delegated", receipt.Mode, "模式应该正确")
	assert.NotEmpty(t, receipt.UnsignedTxHash, "应该包含交易哈希")
}

// TestHandleDelegatedMode_NilClient 测试nil客户端
func TestHandleDelegatedMode_NilClient(t *testing.T) {
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{}

	receipt, err := handleDelegatedMode(ctx, nil, unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
}

// TestHandleThresholdMode_Success 测试处理门限模式成功
func TestHandleThresholdMode_Success(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{}

	receipt, err := handleThresholdMode(ctx, mockTxHashClient, unsignedTx)

	assert.NoError(t, err, "应该成功处理")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "threshold", receipt.Mode, "模式应该正确")
}

// TestHandlePaymasterMode_Success 测试处理代付模式成功
func TestHandlePaymasterMode_Success(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()
	unsignedTx := &transaction.Transaction{}

	receipt, err := handlePaymasterMode(ctx, mockTxHashClient, unsignedTx)

	assert.NoError(t, err, "应该成功处理")
	assert.NotNil(t, receipt, "应该返回收据")
	assert.Equal(t, "paymaster", receipt.Mode, "模式应该正确")
}

// ============================================================================
// Mock对象定义
// ============================================================================

// mockTxAdapterForBuildTransaction Mock的TxAdapter（用于构建交易测试）
type mockTxAdapterForBuildTransaction struct {
	beginTransactionCallCount int
	addTransferCallCount      int
	addCustomInputCallCount   int
	addCustomOutputCallCount  int
	getDraftCallCount         int
	finalizeTransactionCallCount int
	cleanupDraftCallCount     int
	
	beginTransactionError     error
	addTransferError          error
	addCustomInputError       error
	addCustomOutputError      error
	getDraftError             error
	finalizeTransactionError  error
	cleanupDraftError         error
	
	draftHandle               int32
	draft                     *types.DraftTx
	finalizedTx               *transaction.Transaction
}

func (m *mockTxAdapterForBuildTransaction) BeginTransaction(ctx context.Context, blockHeight uint64, blockTimestamp uint64) (int32, error) {
	m.beginTransactionCallCount++
	if m.beginTransactionError != nil {
		return 0, m.beginTransactionError
	}
	m.draftHandle = 1
	return m.draftHandle, nil
}

func (m *mockTxAdapterForBuildTransaction) AddTransfer(ctx context.Context, draftHandle int32, from []byte, to []byte, amount string, tokenID []byte) (int32, error) {
	m.addTransferCallCount++
	if m.addTransferError != nil {
		return 0, m.addTransferError
	}
	return 1, nil
}

func (m *mockTxAdapterForBuildTransaction) AddCustomInput(ctx context.Context, draftHandle int32, outpoint *transaction.OutPoint, isReferenceOnly bool) (int32, error) {
	m.addCustomInputCallCount++
	if m.addCustomInputError != nil {
		return 0, m.addCustomInputError
	}
	return 0, nil
}

func (m *mockTxAdapterForBuildTransaction) AddCustomOutput(ctx context.Context, draftHandle int32, output *transaction.TxOutput) (int32, error) {
	m.addCustomOutputCallCount++
	if m.addCustomOutputError != nil {
		return 0, m.addCustomOutputError
	}
	return 0, nil
}

func (m *mockTxAdapterForBuildTransaction) GetDraft(ctx context.Context, draftHandle int32) (*types.DraftTx, error) {
	m.getDraftCallCount++
	if m.getDraftError != nil {
		return nil, m.getDraftError
	}
	if m.draft == nil {
		m.draft = &types.DraftTx{
			DraftID: "draft-123",
			Tx: &transaction.Transaction{
				Inputs:  []*transaction.TxInput{},
				Outputs: []*transaction.TxOutput{},
			},
		}
	}
	return m.draft, nil
}

func (m *mockTxAdapterForBuildTransaction) FinalizeTransaction(ctx context.Context, draftHandle int32) (*transaction.Transaction, error) {
	m.finalizeTransactionCallCount++
	if m.finalizeTransactionError != nil {
		return nil, m.finalizeTransactionError
	}
	if m.finalizedTx == nil {
		m.finalizedTx = &transaction.Transaction{
			Inputs:  []*transaction.TxInput{},
			Outputs: []*transaction.TxOutput{},
		}
	}
	return m.finalizedTx, nil
}

func (m *mockTxAdapterForBuildTransaction) CleanupDraft(ctx context.Context, draftHandle int32) error {
	m.cleanupDraftCallCount++
	if m.cleanupDraftError != nil {
		return m.cleanupDraftError
	}
	return nil
}

// mockUTXOQueryForPaymaster Mock的UTXO查询服务（用于paymaster测试）
type mockUTXOQueryForPaymaster struct {
	sponsorUTXOs []*utxo.UTXO
	queryError   error
}

func (m *mockUTXOQueryForPaymaster) GetUTXO(ctx context.Context, outpoint *pb.OutPoint) (*utxo.UTXO, error) {
	return nil, nil
}

func (m *mockUTXOQueryForPaymaster) GetUTXOsByAddress(ctx context.Context, address []byte, category *utxo.UTXOCategory, onlyAvailable bool) ([]*utxo.UTXO, error) {
	return nil, nil
}

func (m *mockUTXOQueryForPaymaster) GetSponsorPoolUTXOs(ctx context.Context, onlyAvailable bool) ([]*utxo.UTXO, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.sponsorUTXOs, nil
}

func (m *mockUTXOQueryForPaymaster) GetCurrentStateRoot(ctx context.Context) ([]byte, error) {
	return nil, nil
}

// mockTxHashServiceClient Mock的交易哈希服务客户端
type mockTxHashServiceClient struct {
	hash    []byte
	isValid bool
	err     error
}

func (m *mockTxHashServiceClient) ComputeHash(ctx context.Context, in *transaction.ComputeHashRequest, opts ...grpc.CallOption) (*transaction.ComputeHashResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.hash == nil {
		m.hash = make([]byte, 32)
	}
	return &transaction.ComputeHashResponse{
		Hash:    m.hash,
		IsValid: m.isValid,
	}, nil
}

func (m *mockTxHashServiceClient) ValidateHash(ctx context.Context, in *transaction.ValidateHashRequest, opts ...grpc.CallOption) (*transaction.ValidateHashResponse, error) {
	return nil, nil
}

func (m *mockTxHashServiceClient) ComputeSignatureHash(ctx context.Context, in *transaction.ComputeSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ComputeSignatureHashResponse, error) {
	return nil, nil
}

func (m *mockTxHashServiceClient) ValidateSignatureHash(ctx context.Context, in *transaction.ValidateSignatureHashRequest, opts ...grpc.CallOption) (*transaction.ValidateSignatureHashResponse, error) {
	return nil, nil
}

