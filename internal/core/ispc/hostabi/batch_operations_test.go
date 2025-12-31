package hostabi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// BatchDraftOperations 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 BatchDraftOperations 的缺陷和BUG
//
// ============================================================================

// TestBatchDraftOperations_BatchAddInputs_Empty 测试空输入列表
func TestBatchDraftOperations_BatchAddInputs_Empty(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	result, err := batchOps.BatchAddInputs(ctx, "draft-123", []BatchInputSpec{})

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 0, result.SuccessCount, "成功数应该为0")
	assert.Equal(t, 0, result.FailureCount, "失败数应该为0")
	assert.Empty(t, result.Indices, "索引列表应该为空")
	assert.Empty(t, result.Errors, "错误列表应该为空")
}

// TestBatchDraftOperations_BatchAddInputs_Success 测试成功批量添加输入
func TestBatchDraftOperations_BatchAddInputs_Success(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{
			addInputErrorAtIndex: -1, // 不返回错误
		},
	}

	ctx := context.Background()
	inputs := []BatchInputSpec{
		{
			Outpoint:        &pb.OutPoint{TxId: make([]byte, 32), OutputIndex: 0},
			IsReferenceOnly: false,
			UnlockingProof:  nil,
		},
		{
			Outpoint:        &pb.OutPoint{TxId: make([]byte, 32), OutputIndex: 1},
			IsReferenceOnly: false,
			UnlockingProof:  nil,
		},
	}

	result, err := batchOps.BatchAddInputs(ctx, "draft-123", inputs)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 2, result.SuccessCount, "成功数应该为2")
	assert.Equal(t, 0, result.FailureCount, "失败数应该为0")
	assert.Len(t, result.Indices, 2, "索引列表应该有2个元素")
	assert.Empty(t, result.Errors, "错误列表应该为空")
}

// TestBatchDraftOperations_BatchAddInputs_LoadDraftFailed 测试加载草稿失败
func TestBatchDraftOperations_BatchAddInputs_LoadDraftFailed(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{
			loadDraftError: assert.AnError,
		},
	}

	ctx := context.Background()
	inputs := []BatchInputSpec{
		{
			Outpoint:        &pb.OutPoint{TxId: make([]byte, 32), OutputIndex: 0},
			IsReferenceOnly: false,
			UnlockingProof:  nil,
		},
	}

	result, err := batchOps.BatchAddInputs(ctx, "draft-123", inputs)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, result, "结果应该为nil")
	assert.Contains(t, err.Error(), "加载草稿失败", "错误信息应该正确")
}

// TestBatchDraftOperations_BatchAddInputs_PartialFailure 测试部分失败
func TestBatchDraftOperations_BatchAddInputs_PartialFailure(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{
			addInputErrorAtIndex: 1, // 第二个输入失败
		},
	}

	ctx := context.Background()
	inputs := []BatchInputSpec{
		{
			Outpoint:        &pb.OutPoint{TxId: make([]byte, 32), OutputIndex: 0},
			IsReferenceOnly: false,
			UnlockingProof:  nil,
		},
		{
			Outpoint:        &pb.OutPoint{TxId: make([]byte, 32), OutputIndex: 1},
			IsReferenceOnly: false,
			UnlockingProof:  nil,
		},
	}

	result, err := batchOps.BatchAddInputs(ctx, "draft-123", inputs)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 1, result.SuccessCount, "成功数应该为1")
	assert.Equal(t, 1, result.FailureCount, "失败数应该为1")
	assert.Len(t, result.Errors, 1, "错误列表应该有1个元素")
	assert.Contains(t, err.Error(), "批量添加输入部分失败", "错误信息应该正确")
}

// TestBatchDraftOperations_BatchAddInputs_SaveDraftFailed 测试保存草稿失败
func TestBatchDraftOperations_BatchAddInputs_SaveDraftFailed(t *testing.T) {
	// 创建一个mock，AddInput成功，但SaveDraft失败
	mockService := &mockDraftServiceForBatchOps{
		saveDraftError:      assert.AnError,
		addInputErrorAtIndex: -1, // 不返回错误
	}
	batchOps := &BatchDraftOperations{
		draftService: mockService,
	}

	ctx := context.Background()
	inputs := []BatchInputSpec{
		{
			Outpoint:        &pb.OutPoint{TxId: make([]byte, 32), OutputIndex: 0},
			IsReferenceOnly: false,
			UnlockingProof:  nil,
		},
	}

	result, err := batchOps.BatchAddInputs(ctx, "draft-123", inputs)

	// 当保存失败时，会回滚输入，然后返回"保存草稿失败"的错误
	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, result, "结果应该为nil（保存失败时返回nil）")
	assert.Contains(t, err.Error(), "保存草稿失败", "错误信息应该正确")
}

// TestBatchDraftOperations_BatchAddAssetOutputs_Empty 测试空输出列表
func TestBatchDraftOperations_BatchAddAssetOutputs_Empty(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	result, err := batchOps.BatchAddAssetOutputs(ctx, "draft-123", []BatchAssetOutputSpec{})

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 0, result.SuccessCount, "成功数应该为0")
	assert.Equal(t, 0, result.FailureCount, "失败数应该为0")
}

// TestBatchDraftOperations_BatchAddAssetOutputs_Success 测试成功批量添加资产输出
func TestBatchDraftOperations_BatchAddAssetOutputs_Success(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	outputs := []BatchAssetOutputSpec{
		{
			Owner:             make([]byte, 20),
			Amount:            1000,
			TokenID:           nil,
			LockingConditions: []*pb.LockingCondition{},
		},
		{
			Owner:             make([]byte, 20),
			Amount:            2000,
			TokenID:           nil,
			LockingConditions: []*pb.LockingCondition{},
		},
	}

	result, err := batchOps.BatchAddAssetOutputs(ctx, "draft-123", outputs)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 2, result.SuccessCount, "成功数应该为2")
	assert.Equal(t, 0, result.FailureCount, "失败数应该为0")
}

// TestBatchDraftOperations_BatchAddAssetOutputs_InvalidOwnerLength 测试无效的owner长度
func TestBatchDraftOperations_BatchAddAssetOutputs_InvalidOwnerLength(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	outputs := []BatchAssetOutputSpec{
		{
			Owner:             make([]byte, 19), // 无效长度
			Amount:            1000,
			TokenID:           nil,
			LockingConditions: []*pb.LockingCondition{},
		},
	}

	result, err := batchOps.BatchAddAssetOutputs(ctx, "draft-123", outputs)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 0, result.SuccessCount, "成功数应该为0")
	assert.Equal(t, 1, result.FailureCount, "失败数应该为1")
	assert.Len(t, result.Errors, 1, "错误列表应该有1个元素")
	assert.Contains(t, result.Errors[0].Error(), "owner 地址必须是 20 字节", "错误信息应该正确")
}

// TestBatchDraftOperations_BatchAddResourceOutputs_Empty 测试空资源输出列表
func TestBatchDraftOperations_BatchAddResourceOutputs_Empty(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	result, err := batchOps.BatchAddResourceOutputs(ctx, "draft-123", []BatchResourceOutputSpec{})

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 0, result.SuccessCount, "成功数应该为0")
	assert.Equal(t, 0, result.FailureCount, "失败数应该为0")
}

// TestBatchDraftOperations_BatchAddResourceOutputs_Success 测试成功批量添加资源输出
func TestBatchDraftOperations_BatchAddResourceOutputs_Success(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	outputs := []BatchResourceOutputSpec{
		{
			ContentHash:       make([]byte, 32),
			Category:          "wasm",
			Owner:             make([]byte, 20),
			LockingConditions: []*pb.LockingCondition{},
			Metadata:          []byte("metadata1"),
		},
		{
			ContentHash:       make([]byte, 32),
			Category:          "onnx",
			Owner:             make([]byte, 20),
			LockingConditions: []*pb.LockingCondition{},
			Metadata:          []byte("metadata2"),
		},
	}

	result, err := batchOps.BatchAddResourceOutputs(ctx, "draft-123", outputs)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 2, result.SuccessCount, "成功数应该为2")
	assert.Equal(t, 0, result.FailureCount, "失败数应该为0")
}

// TestBatchDraftOperations_BatchAddResourceOutputs_InvalidContentHashLength 测试无效的contentHash长度
func TestBatchDraftOperations_BatchAddResourceOutputs_InvalidContentHashLength(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	outputs := []BatchResourceOutputSpec{
		{
			ContentHash:       make([]byte, 31), // 无效长度
			Category:          "wasm",
			Owner:             make([]byte, 20),
			LockingConditions: []*pb.LockingCondition{},
			Metadata:          []byte("metadata"),
		},
	}

	result, err := batchOps.BatchAddResourceOutputs(ctx, "draft-123", outputs)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 0, result.SuccessCount, "成功数应该为0")
	assert.Equal(t, 1, result.FailureCount, "失败数应该为1")
	assert.Contains(t, result.Errors[0].Error(), "contentHash 必须是 32 字节", "错误信息应该正确")
}

// TestBatchDraftOperations_BatchAddResourceOutputs_InvalidOwnerLength 测试无效的owner长度
func TestBatchDraftOperations_BatchAddResourceOutputs_InvalidOwnerLength(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	outputs := []BatchResourceOutputSpec{
		{
			ContentHash:       make([]byte, 32),
			Category:          "wasm",
			Owner:             make([]byte, 19), // 无效长度
			LockingConditions: []*pb.LockingCondition{},
			Metadata:          []byte("metadata"),
		},
	}

	result, err := batchOps.BatchAddResourceOutputs(ctx, "draft-123", outputs)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 0, result.SuccessCount, "成功数应该为0")
	assert.Equal(t, 1, result.FailureCount, "失败数应该为1")
	assert.Contains(t, result.Errors[0].Error(), "owner 地址必须是 20 字节", "错误信息应该正确")
}

// TestBatchDraftOperations_BatchAddStateOutputs_Empty 测试空状态输出列表
func TestBatchDraftOperations_BatchAddStateOutputs_Empty(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	result, err := batchOps.BatchAddStateOutputs(ctx, "draft-123", []BatchStateOutputSpec{})

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 0, result.SuccessCount, "成功数应该为0")
	assert.Equal(t, 0, result.FailureCount, "失败数应该为0")
}

// TestBatchDraftOperations_BatchAddStateOutputs_Success 测试成功批量添加状态输出
func TestBatchDraftOperations_BatchAddStateOutputs_Success(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	outputs := []BatchStateOutputSpec{
		{
			StateID:             []byte("state1"),
			StateVersion:        1,
			ExecutionResultHash: make([]byte, 32),
			PublicInputs:        []byte("inputs1"),
			ParentStateHash:     []byte("parent1"),
		},
		{
			StateID:             []byte("state2"),
			StateVersion:        2,
			ExecutionResultHash: make([]byte, 32),
			PublicInputs:        []byte("inputs2"),
			ParentStateHash:     []byte("parent2"),
		},
	}

	result, err := batchOps.BatchAddStateOutputs(ctx, "draft-123", outputs)

	assert.NoError(t, err, "应该成功")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 2, result.SuccessCount, "成功数应该为2")
	assert.Equal(t, 0, result.FailureCount, "失败数应该为0")
}

// TestBatchDraftOperations_BatchAddStateOutputs_EmptyStateID 测试空的stateID
func TestBatchDraftOperations_BatchAddStateOutputs_EmptyStateID(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	outputs := []BatchStateOutputSpec{
		{
			StateID:             []byte{}, // 空stateID
			StateVersion:        1,
			ExecutionResultHash: make([]byte, 32),
			PublicInputs:        []byte("inputs"),
			ParentStateHash:     []byte("parent"),
		},
	}

	result, err := batchOps.BatchAddStateOutputs(ctx, "draft-123", outputs)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 0, result.SuccessCount, "成功数应该为0")
	assert.Equal(t, 1, result.FailureCount, "失败数应该为1")
	assert.Contains(t, result.Errors[0].Error(), "stateID 不能为空", "错误信息应该正确")
}

// TestBatchDraftOperations_BatchAddStateOutputs_InvalidExecutionResultHashLength 测试无效的executionResultHash长度
func TestBatchDraftOperations_BatchAddStateOutputs_InvalidExecutionResultHashLength(t *testing.T) {
	batchOps := &BatchDraftOperations{
		draftService: &mockDraftServiceForBatchOps{},
	}

	ctx := context.Background()
	outputs := []BatchStateOutputSpec{
		{
			StateID:             []byte("state1"),
			StateVersion:        1,
			ExecutionResultHash: make([]byte, 31), // 无效长度
			PublicInputs:        []byte("inputs"),
			ParentStateHash:     []byte("parent"),
		},
	}

	result, err := batchOps.BatchAddStateOutputs(ctx, "draft-123", outputs)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, result, "应该返回结果")
	assert.Equal(t, 0, result.SuccessCount, "成功数应该为0")
	assert.Equal(t, 1, result.FailureCount, "失败数应该为1")
	assert.Contains(t, result.Errors[0].Error(), "executionResultHash 必须是 32 字节", "错误信息应该正确")
}

// ============================================================================
// Mock对象定义
// ============================================================================

// mockDraftServiceForBatchOps Mock的交易草稿服务（用于批量操作测试）
type mockDraftServiceForBatchOps struct {
	loadDraftError        error
	saveDraftError        error
	addInputErrorAtIndex  int // 在第几个输入时返回错误（-1表示不返回错误）
	addInputCallCount     int
	addInputShouldFail    bool // 如果为true，所有AddInput调用都失败
}

func (m *mockDraftServiceForBatchOps) CreateDraft(ctx context.Context) (*types.DraftTx, error) { return nil, nil }
func (m *mockDraftServiceForBatchOps) LoadDraft(ctx context.Context, draftID string) (*types.DraftTx, error) {
	if m.loadDraftError != nil {
		return nil, m.loadDraftError
	}
	return &types.DraftTx{
		DraftID: draftID,
		Tx:      &pb.Transaction{},
	}, nil
}
func (m *mockDraftServiceForBatchOps) SaveDraft(ctx context.Context, draft *types.DraftTx) error {
	if m.saveDraftError != nil {
		return m.saveDraftError
	}
	return nil
}
func (m *mockDraftServiceForBatchOps) GetDraftByID(ctx context.Context, draftID string) (*types.DraftTx, error) { return nil, nil }
func (m *mockDraftServiceForBatchOps) ValidateDraft(ctx context.Context, draft *types.DraftTx) error { return nil }
func (m *mockDraftServiceForBatchOps) SealDraft(ctx context.Context, draft *types.DraftTx) (*types.ComposedTx, error) { return nil, nil }
func (m *mockDraftServiceForBatchOps) DeleteDraft(ctx context.Context, draftID string) error { return nil }
func (m *mockDraftServiceForBatchOps) AddInput(ctx context.Context, draft *types.DraftTx, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	m.addInputCallCount++
	if m.addInputShouldFail {
		return 0, assert.AnError
	}
	if m.addInputErrorAtIndex >= 0 && m.addInputCallCount == m.addInputErrorAtIndex+1 {
		return 0, assert.AnError
	}
	// 模拟添加输入
	draft.Tx.Inputs = append(draft.Tx.Inputs, &pb.TxInput{})
	return uint32(len(draft.Tx.Inputs) - 1), nil
}
func (m *mockDraftServiceForBatchOps) AddAssetOutput(ctx context.Context, draft *types.DraftTx, owner []byte, amount string, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	// 模拟添加输出
	draft.Tx.Outputs = append(draft.Tx.Outputs, &pb.TxOutput{})
	return uint32(len(draft.Tx.Outputs) - 1), nil
}
func (m *mockDraftServiceForBatchOps) AddResourceOutput(ctx context.Context, draft *types.DraftTx, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	// 模拟添加输出
	draft.Tx.Outputs = append(draft.Tx.Outputs, &pb.TxOutput{})
	return uint32(len(draft.Tx.Outputs) - 1), nil
}
func (m *mockDraftServiceForBatchOps) AddStateOutput(ctx context.Context, draft *types.DraftTx, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	// 模拟添加输出
	draft.Tx.Outputs = append(draft.Tx.Outputs, &pb.TxOutput{})
	return uint32(len(draft.Tx.Outputs) - 1), nil
}

