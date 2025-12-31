package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// ports_token_lifecycle.go 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 AppendContractTokenOutput, AppendBurnIntent, AppendApproveIntent 的缺陷和BUG
//
// ============================================================================

// TestAppendContractTokenOutput_WithTokenUniqueID 测试使用tokenUniqueID创建NFT输出
func TestAppendContractTokenOutput_WithTokenUniqueID(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: make([]byte, 20),
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx: &pb.Transaction{},
		},
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1) // NFT通常数量为1
	tokenUniqueID := make([]byte, 20)
	tokenUniqueID[0] = 0x01

	idx, err := hostABI.AppendContractTokenOutput(ctx, recipient, amount, nil, tokenUniqueID, nil)

	assert.NoError(t, err, "应该成功追加NFT输出")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
	
	// ✅ 验证 contract_address 已正确设置
	draft := mockExecCtx.transactionDraft
	require.NotNil(t, draft)
	require.Len(t, draft.Outputs, 1)
	output := draft.Outputs[0]
	contractToken := output.GetAsset().GetContractToken()
	require.NotNil(t, contractToken)
	assert.Equal(t, mockExecCtx.contractAddress, contractToken.ContractAddress, "contract_address 应该匹配执行合约的地址")
	assert.Equal(t, tokenUniqueID, contractToken.GetNftUniqueId(), "token_identifier 应该正确设置")
}

// TestAppendContractTokenOutput_WithTokenClassID 测试使用tokenClassID创建FT/SFT输出
func TestAppendContractTokenOutput_WithTokenClassID(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: make([]byte, 20),
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx: &pb.Transaction{},
		},
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)
	tokenClassID := make([]byte, 20)
	tokenClassID[0] = 0x02

	idx, err := hostABI.AppendContractTokenOutput(ctx, recipient, amount, tokenClassID, nil, nil)

	assert.NoError(t, err, "应该成功追加FT/SFT输出")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
	
	// ✅ 验证 contract_address 已正确设置
	draft := mockExecCtx.transactionDraft
	require.NotNil(t, draft)
	require.Len(t, draft.Outputs, 1)
	output := draft.Outputs[0]
	contractToken := output.GetAsset().GetContractToken()
	require.NotNil(t, contractToken)
	assert.Equal(t, mockExecCtx.contractAddress, contractToken.ContractAddress, "contract_address 应该匹配执行合约的地址")
	assert.Equal(t, tokenClassID, contractToken.GetFungibleClassId(), "token_identifier 应该正确设置")
}

// TestAppendContractTokenOutput_WithLockingConditions 测试带锁定条件的代币输出
func TestAppendContractTokenOutput_WithLockingConditions(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: make([]byte, 20),
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx: &pb.Transaction{},
		},
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)
	tokenClassID := make([]byte, 20)
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

	idx, err := hostABI.AppendContractTokenOutput(ctx, recipient, amount, tokenClassID, nil, lockingConditions)

	assert.NoError(t, err, "应该成功追加带锁定条件的代币输出")
	assert.Equal(t, uint32(0), idx, "应该返回输出索引")
}

// TestAppendContractTokenOutput_GetTransactionDraftFailed 测试获取交易草稿失败
func TestAppendContractTokenOutput_GetTransactionDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: make([]byte, 20),
		getTransactionDraftError: assert.AnError, // 获取草稿失败
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)
	tokenClassID := make([]byte, 20)

	idx, err := hostABI.AppendContractTokenOutput(ctx, recipient, amount, tokenClassID, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "获取交易草稿失败", "错误信息应该正确")
}

// TestAppendContractTokenOutput_NilContractAddress 测试nil合约地址
func TestAppendContractTokenOutput_NilContractAddress(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: nil, // nil合约地址
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx: &pb.Transaction{},
		},
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)
	tokenClassID := make([]byte, 20)

	idx, err := hostABI.AppendContractTokenOutput(ctx, recipient, amount, tokenClassID, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "无法获取合约地址", "错误信息应该正确")
}

// TestAppendContractTokenOutput_BothNil 测试tokenClassID和tokenUniqueID都为nil
func TestAppendContractTokenOutput_BothNil(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: make([]byte, 20),
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx: &pb.Transaction{},
		},
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)

	idx, err := hostABI.AppendContractTokenOutput(ctx, recipient, amount, nil, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "tokenClassID 和 tokenUniqueID 不能同时为 nil", "错误信息应该正确")
}

// TestAppendContractTokenOutput_UpdateTransactionDraftFailed 测试更新交易草稿失败
func TestAppendContractTokenOutput_UpdateTransactionDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID:         "draft-123",
		contractAddress: make([]byte, 20),
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx: &pb.Transaction{},
		},
		updateTransactionDraftError: assert.AnError, // 更新草稿失败
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	recipient := make([]byte, 20)
	amount := uint64(1000)
	tokenClassID := make([]byte, 20)

	idx, err := hostABI.AppendContractTokenOutput(ctx, recipient, amount, tokenClassID, nil, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint32(0), idx, "索引应该为0")
	assert.Contains(t, err.Error(), "更新交易草稿失败", "错误信息应该正确")
}

// TestAppendBurnIntent_Success 测试成功追加销毁意图
func TestAppendBurnIntent_Success(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx:          &pb.Transaction{},
			BurnIntents: []*ispcInterfaces.TokenBurnIntent{},
		},
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	tokenID := make([]byte, 20)
	amount := uint64(1000)
	burnProof := make([]byte, 32)

	err := hostABI.AppendBurnIntent(ctx, tokenID, amount, burnProof)

	assert.NoError(t, err, "应该成功追加销毁意图")
}

// TestAppendBurnIntent_GetTransactionDraftFailed 测试获取交易草稿失败
func TestAppendBurnIntent_GetTransactionDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
		getTransactionDraftError: assert.AnError, // 获取草稿失败
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	tokenID := make([]byte, 20)
	amount := uint64(1000)

	err := hostABI.AppendBurnIntent(ctx, tokenID, amount, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "获取交易草稿失败", "错误信息应该正确")
}

// TestAppendBurnIntent_UpdateTransactionDraftFailed 测试更新交易草稿失败
func TestAppendBurnIntent_UpdateTransactionDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx:          &pb.Transaction{},
			BurnIntents: []*ispcInterfaces.TokenBurnIntent{},
		},
		updateTransactionDraftError: assert.AnError, // 更新草稿失败
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	tokenID := make([]byte, 20)
	amount := uint64(1000)

	err := hostABI.AppendBurnIntent(ctx, tokenID, amount, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "更新交易草稿失败", "错误信息应该正确")
}

// TestAppendApproveIntent_Success 测试成功追加授权意图
func TestAppendApproveIntent_Success(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx:             &pb.Transaction{},
			ApproveIntents: []*ispcInterfaces.TokenApproveIntent{},
		},
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	tokenID := make([]byte, 20)
	spender := make([]byte, 20)
	amount := uint64(1000)
	expiry := uint64(1700000000)

	err := hostABI.AppendApproveIntent(ctx, tokenID, spender, amount, expiry)

	assert.NoError(t, err, "应该成功追加授权意图")
}

// TestAppendApproveIntent_Permanent 测试永久授权（expiry=0）
func TestAppendApproveIntent_Permanent(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx:             &pb.Transaction{},
			ApproveIntents: []*ispcInterfaces.TokenApproveIntent{},
		},
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	tokenID := make([]byte, 20)
	spender := make([]byte, 20)
	amount := uint64(1000)
	expiry := uint64(0) // 永久授权

	err := hostABI.AppendApproveIntent(ctx, tokenID, spender, amount, expiry)

	assert.NoError(t, err, "应该成功追加永久授权意图")
}

// TestAppendApproveIntent_GetTransactionDraftFailed 测试获取交易草稿失败
func TestAppendApproveIntent_GetTransactionDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
		getTransactionDraftError: assert.AnError, // 获取草稿失败
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	tokenID := make([]byte, 20)
	spender := make([]byte, 20)
	amount := uint64(1000)

	err := hostABI.AppendApproveIntent(ctx, tokenID, spender, amount, 0)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "获取交易草稿失败", "错误信息应该正确")
}

// TestAppendApproveIntent_UpdateTransactionDraftFailed 测试更新交易草稿失败
func TestAppendApproveIntent_UpdateTransactionDraftFailed(t *testing.T) {
	mockExecCtx := &mockExecutionContextForPorts{
		draftID: "draft-123",
		transactionDraft: &ispcInterfaces.TransactionDraft{
			Tx:             &pb.Transaction{},
			ApproveIntents: []*ispcInterfaces.TokenApproveIntent{},
		},
		updateTransactionDraftError: assert.AnError, // 更新草稿失败
	}
	mockDraftService := &mockDraftServiceForPorts{}
	hostABI := createHostRuntimePortsWithMocks(t, mockExecCtx, mockDraftService)
	ctx := context.Background()
	tokenID := make([]byte, 20)
	spender := make([]byte, 20)
	amount := uint64(1000)

	err := hostABI.AppendApproveIntent(ctx, tokenID, spender, amount, 0)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "更新交易草稿失败", "错误信息应该正确")
}


