package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/ispc/testutil"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// ports_outputs_asset.go 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 AppendAssetOutput, Transfer, TransferEx 的缺陷和BUG
//
// ============================================================================

// TestAppendAssetOutput_NativeCoin 测试追加原生币输出
func TestAppendAssetOutput_NativeCoin(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)

	idx, err := hostABI.AppendAssetOutput(ctx, recipient, amount, nil, nil)

	assert.NoError(t, err, "应该成功追加原生币输出")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
}

// TestAppendAssetOutput_ContractToken 测试追加合约代币输出
func TestAppendAssetOutput_ContractToken(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)
	tokenID := make([]byte, 20)

	idx, err := hostABI.AppendAssetOutput(ctx, recipient, amount, tokenID, nil)

	assert.NoError(t, err, "应该成功追加合约代币输出")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
}

// TestAppendAssetOutput_WithLockingConditions 测试带锁定条件的输出
func TestAppendAssetOutput_WithLockingConditions(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)
	lockingConditions := []*pb.LockingCondition{
		{
			Condition: &pb.LockingCondition_SingleKeyLock{
				SingleKeyLock: &pb.SingleKeyLock{
					KeyRequirement: &pb.SingleKeyLock_RequiredAddressHash{
						RequiredAddressHash: make([]byte, 20),
					},
				},
			},
		},
	}

	idx, err := hostABI.AppendAssetOutput(ctx, recipient, amount, nil, lockingConditions)

	assert.NoError(t, err, "应该成功追加带锁定条件的输出")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
}

// TestAppendAssetOutput_EmptyDraftID 测试空草稿ID
func TestAppendAssetOutput_EmptyDraftID(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "", // 空草稿ID
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)

	idx, err := hostABI.AppendAssetOutput(ctx, recipient, amount, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "获取草稿ID失败", "错误信息应该正确")
}

// TestAppendAssetOutput_LoadDraftFailed 测试加载草稿失败
func TestAppendAssetOutput_LoadDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
	}
	mockDraftService := &mockDraftServiceForPorts{
		loadDraftError: assert.AnError, // 加载草稿失败
	}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)

	idx, err := hostABI.AppendAssetOutput(ctx, recipient, amount, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "加载交易草稿失败", "错误信息应该正确")
}

// TestAppendAssetOutput_AddAssetOutputFailed 测试添加资产输出失败
func TestAppendAssetOutput_AddAssetOutputFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
	}
	mockDraftService := &mockDraftServiceForPorts{
		addAssetOutputError: assert.AnError, // 添加输出失败
	}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)

	idx, err := hostABI.AppendAssetOutput(ctx, recipient, amount, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "追加资产输出失败", "错误信息应该正确")
}

// TestAppendAssetOutput_ContractToken_NilContractAddress 测试合约代币但合约地址为nil
func TestAppendAssetOutput_ContractToken_NilContractAddress(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: nil, // 合约地址为nil
	}
	mockDraftService := &mockDraftServiceForPorts{
		draft: &types.DraftTx{
			DraftID: "draft-123",
			Tx: &pb.Transaction{
				Outputs: []*pb.TxOutput{
					{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_ContractToken{
									ContractToken: &pb.ContractTokenAsset{},
								},
							},
						},
					},
				},
			},
		},
	}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)
	tokenID := make([]byte, 20)

	idx, err := hostABI.AppendAssetOutput(ctx, recipient, amount, tokenID, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "无法获取合约地址", "错误信息应该正确")
}

// TestAppendAssetOutput_ContractToken_SaveDraftFailed 测试合约代币保存草稿失败
func TestAppendAssetOutput_ContractToken_SaveDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: make([]byte, 20),
	}
	mockDraftService := &mockDraftServiceForPorts{
		draft: &types.DraftTx{
			DraftID: "draft-123",
			Tx: &pb.Transaction{
				Outputs: []*pb.TxOutput{
					{
						OutputContent: &pb.TxOutput_Asset{
							Asset: &pb.AssetOutput{
								AssetContent: &pb.AssetOutput_ContractToken{
									ContractToken: &pb.ContractTokenAsset{},
								},
							},
						},
					},
				},
			},
		},
		saveDraftError: assert.AnError, // 保存草稿失败
	}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)
	tokenID := make([]byte, 20)

	idx, err := hostABI.AppendAssetOutput(ctx, recipient, amount, tokenID, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "保存交易草稿失败", "错误信息应该正确")
}

// TestTransfer_Success 测试基础转账成功
func TestTransfer_Success(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	from := make([]byte, 20)
	to := make([]byte, 20)
	amount := uint64(1000)

	err := hostABI.Transfer(ctx, from, to, amount, nil)

	assert.NoError(t, err, "应该成功转账")
}

// TestTransfer_WithTokenID 测试代币转账
func TestTransfer_WithTokenID(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	from := make([]byte, 20)
	to := make([]byte, 20)
	amount := uint64(1000)
	tokenID := make([]byte, 20)

	err := hostABI.Transfer(ctx, from, to, amount, tokenID)

	assert.NoError(t, err, "应该成功转账代币")
}

// TestTransfer_AppendAssetOutputFailed 测试转账时追加输出失败
func TestTransfer_AppendAssetOutputFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "", // 空草稿ID导致失败
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	from := make([]byte, 20)
	to := make([]byte, 20)
	amount := uint64(1000)

	err := hostABI.Transfer(ctx, from, to, amount, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "创建转账输出失败", "错误信息应该正确")
}

// TestTransferEx_Success 测试扩展转账成功
func TestTransferEx_Success(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	from := make([]byte, 20)
	to := make([]byte, 20)
	amount := uint64(1000)
	lockingConditions := []*pb.LockingCondition{
		{
			Condition: &pb.LockingCondition_SingleKeyLock{
				SingleKeyLock: &pb.SingleKeyLock{
					KeyRequirement: &pb.SingleKeyLock_RequiredAddressHash{
						RequiredAddressHash: make([]byte, 20),
					},
				},
			},
		},
	}

	err := hostABI.TransferEx(ctx, from, to, amount, nil, lockingConditions)

	assert.NoError(t, err, "应该成功执行扩展转账")
}

// TestTransferEx_WithLockingConditions 测试带锁定条件的扩展转账
func TestTransferEx_WithLockingConditions(t *testing.T) {
	hostABI := createTestHostRuntimePortsForPorts(t)
	ctx := context.Background()
	from := make([]byte, 20)
	to := make([]byte, 20)
	amount := uint64(1000)
	lockingConditions := []*pb.LockingCondition{
		{
			Condition: &pb.LockingCondition_SingleKeyLock{
				SingleKeyLock: &pb.SingleKeyLock{
					KeyRequirement: &pb.SingleKeyLock_RequiredAddressHash{
						RequiredAddressHash: make([]byte, 20),
					},
				},
			},
		},
	}

	err := hostABI.TransferEx(ctx, from, to, amount, nil, lockingConditions)

	assert.NoError(t, err, "应该成功执行带高度锁的转账")
}

// TestTransferEx_AppendAssetOutputFailed 测试扩展转账时追加输出失败
func TestTransferEx_AppendAssetOutputFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "", // 空草稿ID导致失败
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	from := make([]byte, 20)
	to := make([]byte, 20)
	amount := uint64(1000)
	lockingConditions := []*pb.LockingCondition{}

	err := hostABI.TransferEx(ctx, from, to, amount, nil, lockingConditions)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "创建转账输出失败", "错误信息应该正确")
}

// ============================================================================
// 辅助函数
// ============================================================================

// createTestHostRuntimePortsForPorts 创建用于测试ports的HostRuntimePorts
func createTestHostRuntimePortsForPorts(t *testing.T) *HostRuntimePorts {
	t.Helper()

	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: make([]byte, 20),
	}
	mockDraftService := &mockDraftServiceForPorts{
		draft: &types.DraftTx{
			DraftID: "draft-123",
			Tx:      &pb.Transaction{},
		},
	}

	return createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
}

// createHostRuntimePortsWithMocks 使用指定的mock对象创建HostRuntimePorts
func createHostRuntimePortsWithMocks(t *testing.T, mockExecCtx *mockExecutionContextForPorts, mockDraftService *mockDraftServiceForPorts) *HostRuntimePorts {
	t.Helper()

	logger := testutil.NewTestLogger()
	mockChainQuery := &mockChainQueryForHostABI{}
	mockUTXOQuery := &mockUTXOQueryForHostABI{}
	mockCASStorage := &mockCASStorageForHostABI{}
	mockTxQuery := &mockTxQueryForHostABI{}
	mockResourceQuery := &mockResourceQueryForHostABI{}
	mockHashManager := testutil.NewTestHashManager()

	hostABI, err := NewHostRuntimePorts(
		logger,
		mockChainQuery,
		&mockBlockQueryForHostABI{},
		mockUTXOQuery,
		mockCASStorage,
		mockTxQuery,
		mockResourceQuery,
		mockDraftService,
		mockHashManager,
		mockExecCtx,
	)
	require.NoError(t, err, "应该成功创建HostRuntimePorts")

	return hostABI.(*HostRuntimePorts)
}

// mockExecutionContextForPorts Mock的ExecutionContext（用于ports测试）
type mockExecutionContextForPorts struct {
	draftID                    string
	contractAddress            []byte
	transactionDraft           *ispcInterfaces.TransactionDraft
	getTransactionDraftError   error
	updateTransactionDraftError error
}

func (m *mockExecutionContextForPorts) GetDraftID() string { return m.draftID }
func (m *mockExecutionContextForPorts) GetContractAddress() []byte { return m.contractAddress }
func (m *mockExecutionContextForPorts) GetExecutionID() string { return "exec-123" }
func (m *mockExecutionContextForPorts) GetCallerAddress() []byte { return make([]byte, 20) }
func (m *mockExecutionContextForPorts) GetBlockHeight() uint64 { return 100 }
func (m *mockExecutionContextForPorts) GetBlockTimestamp() uint64 { return 1234567890 }
func (m *mockExecutionContextForPorts) GetChainID() []byte { return []byte("test-chain") }
func (m *mockExecutionContextForPorts) GetTransactionID() []byte { return make([]byte, 32) }
func (m *mockExecutionContextForPorts) HostABI() ispcInterfaces.HostABI { return nil }
func (m *mockExecutionContextForPorts) SetHostABI(hostABI ispcInterfaces.HostABI) error { return nil }
func (m *mockExecutionContextForPorts) GetTransactionDraft() (*ispcInterfaces.TransactionDraft, error) {
	if m.getTransactionDraftError != nil {
		return nil, m.getTransactionDraftError
	}
	if m.transactionDraft == nil {
		return &ispcInterfaces.TransactionDraft{
			Tx: &pb.Transaction{},
		}, nil
	}
	return m.transactionDraft, nil
}
func (m *mockExecutionContextForPorts) UpdateTransactionDraft(draft *ispcInterfaces.TransactionDraft) error {
	if m.updateTransactionDraftError != nil {
		return m.updateTransactionDraftError
	}
	m.transactionDraft = draft
	return nil
}
func (m *mockExecutionContextForPorts) RecordHostFunctionCall(call *ispcInterfaces.HostFunctionCall) {}
func (m *mockExecutionContextForPorts) GetExecutionTrace() ([]*ispcInterfaces.HostFunctionCall, error) { return nil, nil }
func (m *mockExecutionContextForPorts) RecordStateChange(key string, oldValue interface{}, newValue interface{}) error { return nil }
func (m *mockExecutionContextForPorts) RecordTraceRecords(records []ispcInterfaces.TraceRecord) error { return nil }
func (m *mockExecutionContextForPorts) GetResourceUsage() *types.ResourceUsage { return &types.ResourceUsage{} }
func (m *mockExecutionContextForPorts) FinalizeResourceUsage() {}
func (m *mockExecutionContextForPorts) SetReturnData(data []byte) error { return nil }
func (m *mockExecutionContextForPorts) GetReturnData() ([]byte, error) { return nil, nil }
func (m *mockExecutionContextForPorts) AddEvent(event *ispcInterfaces.Event) error { return nil }
func (m *mockExecutionContextForPorts) GetEvents() ([]*ispcInterfaces.Event, error) { return nil, nil }
func (m *mockExecutionContextForPorts) SetInitParams(params []byte) error { return nil }
func (m *mockExecutionContextForPorts) GetInitParams() ([]byte, error) { return nil, nil }

// mockDraftServiceForPorts Mock的TransactionDraftService（用于ports测试）
type mockDraftServiceForPorts struct {
	draft                  *types.DraftTx
	loadDraftError         error
	addAssetOutputError    error
	addResourceOutputError error
	addStateOutputError    error
	saveDraftError         error
	addAssetOutputIndex    uint32
}

func (m *mockDraftServiceForPorts) CreateDraft(ctx context.Context) (*types.DraftTx, error) {
	return m.draft, nil
}

func (m *mockDraftServiceForPorts) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	if m.loadDraftError != nil {
		return nil, m.loadDraftError
	}
	if m.draft == nil {
		return &types.DraftTx{
			DraftID: draftID,
			Tx:      &pb.Transaction{},
		}, nil
	}
	return m.draft, nil
}

func (m *mockDraftServiceForPorts) SaveDraft(ctx context.Context, draft *types.DraftTx) error {
	if m.saveDraftError != nil {
		return m.saveDraftError
	}
	m.draft = draft
	return nil
}

func (m *mockDraftServiceForPorts) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) {
	return m.draft, nil
}

func (m *mockDraftServiceForPorts) ValidateDraft(ctx context.Context, draft *types.DraftTx) error {
	return nil
}

func (m *mockDraftServiceForPorts) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) {
	return nil, nil
}

func (m *mockDraftServiceForPorts) DeleteDraft(ctx context.Context, draftID string) error {
	return nil
}

func (m *mockDraftServiceForPorts) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	return 0, nil
}

func (m *mockDraftServiceForPorts) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	if m.addAssetOutputError != nil {
		return 0, m.addAssetOutputError
	}
	if draft.Tx == nil {
		draft.Tx = &pb.Transaction{}
	}
	draft.Tx.Outputs = append(draft.Tx.Outputs, &pb.TxOutput{
		Owner: owner,
		OutputContent: &pb.TxOutput_Asset{
			Asset: func() *pb.AssetOutput {
				if tokenID == nil {
					return &pb.AssetOutput{
						AssetContent: &pb.AssetOutput_NativeCoin{
							NativeCoin: &pb.NativeCoinAsset{
								Amount: amount,
							},
						},
					}
				}
				return &pb.AssetOutput{
					AssetContent: &pb.AssetOutput_ContractToken{
						ContractToken: &pb.ContractTokenAsset{
							Amount: amount,
						},
					},
				}
			}(),
		},
	})
	idx := uint32(len(draft.Tx.Outputs) - 1)
	m.addAssetOutputIndex = idx
	return idx, nil
}

func (m *mockDraftServiceForPorts) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	if m.addResourceOutputError != nil {
		return 0, m.addResourceOutputError
	}
	if draft.Tx == nil {
		draft.Tx = &pb.Transaction{}
	}
	draft.Tx.Outputs = append(draft.Tx.Outputs, &pb.TxOutput{
		Owner: owner,
		OutputContent: &pb.TxOutput_Resource{
			Resource: &pb.ResourceOutput{
				Resource: &pbresource.Resource{
					ContentHash: contentHash,
					Category:    pbresource.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
				},
			},
		},
	})
	return uint32(len(draft.Tx.Outputs) - 1), nil
}

func (m *mockDraftServiceForPorts) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	if m.addStateOutputError != nil {
		return 0, m.addStateOutputError
	}
	if draft.Tx == nil {
		draft.Tx = &pb.Transaction{}
	}
	draft.Tx.Outputs = append(draft.Tx.Outputs, &pb.TxOutput{
		OutputContent: &pb.TxOutput_State{
			State: &pb.StateOutput{
				StateId:             stateID,
				StateVersion:        stateVersion,
				ExecutionResultHash: executionResultHash,
			},
		},
	})
	return uint32(len(draft.Tx.Outputs) - 1), nil
}

