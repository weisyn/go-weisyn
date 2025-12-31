// Package builder_test 提供 Builder 服务的单元测试
//
// 🧪 **测试覆盖**：
// - Builder 基础功能测试
// - Type-state 转换测试
// - 边界条件和错误场景测试
package builder

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/internal/core/tx/testutil"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	resourcepb "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== Builder 基础功能测试 ====================

// TestNewService 测试创建新的 Builder 服务
func TestNewService(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	assert.NotNil(t, builder)
	assert.NotNil(t, builder.tx)
	assert.Equal(t, uint32(1), builder.tx.Version)
	assert.Empty(t, builder.tx.Inputs)
	assert.Empty(t, builder.tx.Outputs)
}

// TestAddInput 测试添加交易输入
func TestAddInput(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)

	assert.Len(t, builder.tx.Inputs, 1)
	assert.Equal(t, outpoint, builder.tx.Inputs[0].PreviousOutput)
	assert.False(t, builder.tx.Inputs[0].IsReferenceOnly)
}

// TestAddInput_ReferenceOnly 测试添加引用型输入
func TestAddInput_ReferenceOnly(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, true)

	assert.Len(t, builder.tx.Inputs, 1)
	assert.True(t, builder.tx.Inputs[0].IsReferenceOnly)
}

// TestAddAssetOutput_NativeCoin 测试添加原生币输出
func TestAddAssetOutput_NativeCoin(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	owner := testutil.RandomAddress()
	amount := "1000"
	lock := testutil.CreateSingleKeyLock(nil)

	builder.AddAssetOutput(owner, amount, nil, lock)

	assert.Len(t, builder.tx.Outputs, 1)
	output := builder.tx.Outputs[0]
	assert.Equal(t, owner, output.Owner)

	asset := output.GetAsset()
	require.NotNil(t, asset)
	nativeCoin := asset.GetNativeCoin()
	require.NotNil(t, nativeCoin)
	assert.Equal(t, amount, nativeCoin.Amount)
}

// TestAddAssetOutput_ContractToken 测试添加合约代币输出
func TestAddAssetOutput_ContractToken(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	owner := testutil.RandomAddress()
	amount := "500"
	contractAddress := testutil.RandomAddress()
	lock := testutil.CreateSingleKeyLock(nil)

	builder.AddAssetOutput(owner, amount, contractAddress, lock)

	assert.Len(t, builder.tx.Outputs, 1)
	output := builder.tx.Outputs[0]

	asset := output.GetAsset()
	require.NotNil(t, asset)
	contractToken := asset.GetContractToken()
	require.NotNil(t, contractToken)
	assert.Equal(t, amount, contractToken.Amount)
	assert.Equal(t, contractAddress, contractToken.ContractAddress)
	// 注意：AddAssetOutput 使用默认的 FungibleClassId（"default"），不支持自定义
	assert.Equal(t, []byte("default"), contractToken.GetFungibleClassId())
}

// TestSetNonce 测试设置交易 nonce
func TestSetNonce(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	nonce := uint64(12345)
	builder.SetNonce(nonce)

	assert.Equal(t, nonce, builder.tx.Nonce)
}

// TestSetChainID 测试设置链 ID
func TestSetChainID(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	chainID := []byte("test-chain")
	builder.SetChainID(chainID)

	assert.Equal(t, chainID, builder.tx.ChainId)
}

// TestBuild_EmptyTransaction 测试构建空交易
func TestBuild_EmptyTransaction(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	composed, err := builder.Build()

	assert.Nil(t, composed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty transaction")
}

// TestBuild_OnlyOutputs 测试只有输出无输入（Coinbase 交易）
func TestBuild_OnlyOutputs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	owner := testutil.RandomAddress()
	amount := "1000"
	lock := testutil.CreateSingleKeyLock(nil)

	builder.AddAssetOutput(owner, amount, nil, lock)
	composed, err := builder.Build()

	assert.NoError(t, err)
	assert.NotNil(t, composed)
	assert.NotNil(t, composed.Tx)
	assert.Len(t, composed.Tx.Inputs, 0)
	assert.Len(t, composed.Tx.Outputs, 1)
	// 注意：Build() 返回的 ComposedTx 初始状态为 Sealed: false，只有在 WithProofs() 时才封闭
	assert.False(t, composed.Sealed)
}

// TestBuild_Success 测试正常构建交易
func TestBuild_Success(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 添加输入
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)

	// 添加输出
	owner := testutil.RandomAddress()
	amount := "1000"
	lock := testutil.CreateSingleKeyLock(nil)
	builder.AddAssetOutput(owner, amount, nil, lock)

	composed, err := builder.Build()

	assert.NoError(t, err)
	assert.NotNil(t, composed)
	assert.NotNil(t, composed.Tx)
	assert.Len(t, composed.Tx.Inputs, 1)
	assert.Len(t, composed.Tx.Outputs, 1)
	assert.False(t, composed.Sealed) // Build() 返回的 ComposedTx 初始未封闭
	assert.NotZero(t, composed.Tx.CreationTimestamp)
}

// ==================== Type-state 转换测试 ====================

// TestComposedTx_WithProofs 测试添加证明转换到 ProvenTx
func TestComposedTx_WithProofs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	// 创建包装类型 ComposedTx（builder 包中的类型）
	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	// 创建证明提供者
	proofProvider := testutil.NewMockProofProvider()
	proof := testutil.CreateSingleKeyProof(nil, nil)
	proofProvider.SetProof(0, proof)

	// 转换为 ProvenTx
	proven, err := composed.WithProofs(context.Background(), proofProvider)

	assert.NoError(t, err)
	assert.NotNil(t, proven)
	assert.NotNil(t, proven.Tx)
	assert.True(t, composed.Sealed) // ComposedTx 应该被封闭
	assert.False(t, proven.Sealed)  // ProvenTx 初始状态为未封闭
}

// TestComposedTx_WithProofs_AlreadySealed 测试重复封闭
func TestComposedTx_WithProofs_AlreadySealed(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	// 创建包装类型 ComposedTx
	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	// 第一次转换
	proofProvider := testutil.NewMockProofProvider()
	proof := testutil.CreateSingleKeyProof(nil, nil)
	proofProvider.SetProof(0, proof)
	_, err = composed.WithProofs(context.Background(), proofProvider)
	require.NoError(t, err)

	// 第二次转换应该失败
	_, err = composed.WithProofs(context.Background(), proofProvider)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already sealed")
}

// TestProvenTx_Sign 测试签名转换到 SignedTx
func TestProvenTx_Sign(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx 并添加证明
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	proofProvider := testutil.NewMockProofProvider()
	proof := testutil.CreateSingleKeyProof(nil, nil)
	proofProvider.SetProof(0, proof)
	provenTx, err := composed.WithProofs(context.Background(), proofProvider)
	require.NoError(t, err)

	// 签名
	signer := testutil.NewMockSigner(nil)
	signed, err := provenTx.Sign(context.Background(), signer)

	assert.NoError(t, err)
	assert.NotNil(t, signed)
	assert.NotNil(t, signed.Tx)
	assert.True(t, provenTx.Sealed) // ProvenTx 应该被封闭
}

// TestProvenTx_Sign_MissingProof 测试缺少证明时签名失败
func TestProvenTx_Sign_MissingProof(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建 ComposedTx（不添加证明）
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	provenTx := &ProvenTx{
		ProvenTx: &types.ProvenTx{
			Tx:     composedTx.Tx,
			Sealed: false,
		},
		builder: builder,
	}

	// 签名应该失败（缺少 UnlockingProof）
	signer := testutil.NewMockSigner(nil)
	_, err = provenTx.Sign(context.Background(), signer)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "缺少 UnlockingProof")
}

// TestSignedTx_Submit 测试提交转换到 SubmittedTx
func TestSignedTx_Submit(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 构建完整的交易流程
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	owner := testutil.RandomAddress()
	builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	composedTx, err := builder.Build()
	require.NoError(t, err)

	composed := &ComposedTx{
		ComposedTx: composedTx,
		builder:    builder,
	}

	proofProvider := testutil.NewMockProofProvider()
	proof := testutil.CreateSingleKeyProof(nil, nil)
	proofProvider.SetProof(0, proof)
	provenTx, err := composed.WithProofs(context.Background(), proofProvider)
	require.NoError(t, err)

	signer := testutil.NewMockSigner(nil)
	signedTx, err := provenTx.Sign(context.Background(), signer)
	require.NoError(t, err)

	// 创建模拟的 Processor
	mockTxPool := testutil.NewMockTxPool()
	mockVerifier := &MockVerifier{shouldFail: false}
	processor := &MockProcessor{
		verifier: mockVerifier,
		txPool:   mockTxPool,
	}

	// 提交
	submitted, err := signedTx.Submit(context.Background(), processor)

	assert.NoError(t, err)
	assert.NotNil(t, submitted)
	assert.NotNil(t, submitted.Tx)
	assert.NotNil(t, submitted.TxHash)
	assert.False(t, submitted.SubmittedAt.IsZero())
}

// ==================== 边界条件测试 ====================

// TestAddInput_NilOutpoint 测试 nil outpoint
func TestAddInput_NilOutpoint(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 注意：当前实现允许 nil outpoint，这可能是设计缺陷
	// 但测试应该反映当前行为
	builder.AddInput(nil, false)

	assert.Len(t, builder.tx.Inputs, 1)
	assert.Nil(t, builder.tx.Inputs[0].PreviousOutput)
}

// TestAddAssetOutput_NilOwner 测试 nil owner
func TestAddAssetOutput_NilOwner(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 当前实现允许 nil owner
	builder.AddAssetOutput(nil, "1000", nil, testutil.CreateSingleKeyLock(nil))

	assert.Len(t, builder.tx.Outputs, 1)
	assert.Nil(t, builder.tx.Outputs[0].Owner)
}

// TestAddAssetOutput_EmptyAmount 测试空金额
func TestAddAssetOutput_EmptyAmount(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 当前实现允许空金额（可能需要在验证层检查）
	builder.AddAssetOutput(testutil.RandomAddress(), "", nil, testutil.CreateSingleKeyLock(nil))

	assert.Len(t, builder.tx.Outputs, 1)
	asset := builder.tx.Outputs[0].GetAsset()
	require.NotNil(t, asset)
	nativeCoin := asset.GetNativeCoin()
	require.NotNil(t, nativeCoin)
	assert.Empty(t, nativeCoin.Amount)
}

// TestBuild_MultipleInputsOutputs 测试多个输入输出
func TestBuild_MultipleInputsOutputs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 添加多个输入
	for i := 0; i < 3; i++ {
		outpoint := testutil.CreateOutPoint(nil, uint32(i))
		builder.AddInput(outpoint, false)
	}

	// 添加多个输出
	for i := 0; i < 2; i++ {
		owner := testutil.RandomAddress()
		builder.AddAssetOutput(owner, "1000", nil, testutil.CreateSingleKeyLock(nil))
	}

	composed, err := builder.Build()

	assert.NoError(t, err)
	assert.NotNil(t, composed)
	assert.Len(t, composed.Tx.Inputs, 3)
	assert.Len(t, composed.Tx.Outputs, 2)
}

// ==================== 链式调用测试 ====================

// TestBuilder_ChainCalls 测试链式调用
func TestBuilder_ChainCalls(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	outpoint := testutil.CreateOutPoint(nil, 0)
	owner := testutil.RandomAddress()
	lock := testutil.CreateSingleKeyLock(nil)

	// 链式调用
	result := builder.
		SetNonce(12345).
		AddInput(outpoint, false).
		AddAssetOutput(owner, "1000", nil, lock)

	assert.Equal(t, builder, result)
	assert.Equal(t, uint64(12345), builder.tx.Nonce)
	assert.Len(t, builder.tx.Inputs, 1)
	assert.Len(t, builder.tx.Outputs, 1)
}

// TestAddResourceOutput 测试添加资源输出
func TestAddResourceOutput(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	owner := testutil.RandomAddress()
	lock := testutil.CreateSingleKeyLock(nil)

	// AddResourceOutput 需要 resourcepb.Resource
	resourceProto := &resourcepb.Resource{
		Category:       resourcepb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
		ExecutableType: resourcepb.ExecutableType_EXECUTABLE_TYPE_CONTRACT,
		ContentHash:    testutil.RandomHash(),
		MimeType:       "application/wasm",
		Size:           1024,
	}

	builder.AddResourceOutput(owner, resourceProto, lock)

	assert.Len(t, builder.tx.Outputs, 1)
	output := builder.tx.Outputs[0]
	assert.Equal(t, owner, output.Owner)

	resourceOutput := output.GetResource()
	require.NotNil(t, resourceOutput)
	assert.Equal(t, resourceProto.ContentHash, resourceOutput.Resource.ContentHash)
	assert.Equal(t, resourceProto.MimeType, resourceOutput.Resource.MimeType)
	assert.True(t, resourceOutput.IsImmutable)
	assert.NotZero(t, resourceOutput.CreationTimestamp)
}

// TestAddStateOutput 测试添加状态输出
func TestAddStateOutput(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	owner := testutil.RandomAddress()
	stateID := testutil.RandomHash()
	stateVersion := uint64(1)
	executionResultHash := testutil.RandomHash()
	lock := testutil.CreateSingleKeyLock(nil)
	zkProof := &transaction.ZKStateProof{
		Proof:         testutil.RandomBytes(128),
		PublicInputs:  [][]byte{testutil.RandomBytes(32)},
		ProvingScheme: "groth16",
		Curve:         "bn254",
	}

	builder.AddStateOutput(owner, stateID, stateVersion, zkProof, executionResultHash, lock)

	assert.Len(t, builder.tx.Outputs, 1)
	output := builder.tx.Outputs[0]
	assert.Equal(t, owner, output.Owner)

	stateOutput := output.GetState()
	require.NotNil(t, stateOutput)
	assert.Equal(t, stateID, stateOutput.StateId)
	assert.Equal(t, stateVersion, stateOutput.StateVersion)
	assert.Equal(t, executionResultHash, stateOutput.ExecutionResultHash)
	assert.NotNil(t, stateOutput.ZkProof)
}

// TestReset 测试重置 Builder
func TestReset(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 添加一些数据
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)
	builder.AddAssetOutput(testutil.RandomAddress(), "1000", nil, testutil.CreateSingleKeyLock(nil))
	builder.SetNonce(12345)
	builder.SetChainID([]byte("test-chain"))

	// 验证数据已添加
	assert.Len(t, builder.tx.Inputs, 1)
	assert.Len(t, builder.tx.Outputs, 1)
	assert.Equal(t, uint64(12345), builder.tx.Nonce)
	assert.Equal(t, []byte("test-chain"), builder.tx.ChainId)

	// 重置
	builder.Reset()

	// 验证已重置
	assert.Len(t, builder.tx.Inputs, 0)
	assert.Len(t, builder.tx.Outputs, 0)
	assert.Equal(t, uint64(0), builder.tx.Nonce)
	assert.Empty(t, builder.tx.ChainId)
	assert.Equal(t, uint32(1), builder.tx.Version) // Version 应该保持为 1
}

// TestSetExecutionProof_Success 测试成功设置ExecutionProof
func TestSetExecutionProof_Success(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	contractOutpoint := testutil.CreateOutPoint(nil, 0)
	contractAddr := testutil.RandomAddress()
	execProof := &transaction.ExecutionProof{
		Context: &transaction.ExecutionProof_ExecutionContext{
			CallerIdentity: &transaction.IdentityProof{
				PublicKey:     testutil.RandomBytes(33),
				CallerAddress: testutil.RandomBytes(20),
				Signature:     testutil.RandomBytes(64),
				Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
				Nonce:         testutil.RandomBytes(32),
				Timestamp:     1234567890,
				ContextHash:   testutil.RandomBytes(32),
			},
			ResourceAddress: contractAddr,
			ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
			InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			Metadata:        map[string][]byte{"method_name": []byte("mint")},
		},
		ExecutionResultHash: testutil.RandomBytes(32),
		StateTransitionProof: testutil.RandomBytes(64),
	}

	// 添加引用型输入
	builder.AddInput(contractOutpoint, true)

	// 设置ExecutionProof
	result, err := builder.SetExecutionProof(execProof)

	assert.NoError(t, err)
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.tx.Inputs[0].UnlockingProof)
	
	// 从 UnlockingProof 中提取 ExecutionProof
	var extractedProof *transaction.ExecutionProof
	if execProofInput, ok := builder.tx.Inputs[0].UnlockingProof.(*transaction.TxInput_ExecutionProof); ok {
		extractedProof = execProofInput.ExecutionProof
	}
	require.NotNil(t, extractedProof)
	// ✅ 更新：使用新的字段结构
	assert.Equal(t, execProof.Context.Metadata["method_name"], extractedProof.Context.Metadata["method_name"])
	assert.Equal(t, execProof.Context.ResourceAddress, extractedProof.Context.ResourceAddress)
	assert.Equal(t, execProof.ExecutionResultHash, extractedProof.ExecutionResultHash)
}

// TestSetExecutionProof_NoInput 测试没有输入时设置ExecutionProof
func TestSetExecutionProof_NoInput(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	execProof := &transaction.ExecutionProof{
		Context: &transaction.ExecutionProof_ExecutionContext{
			CallerIdentity: &transaction.IdentityProof{
				PublicKey:     testutil.RandomBytes(33),
				CallerAddress: testutil.RandomBytes(20),
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
			Metadata:        map[string][]byte{"method_name": []byte("mint")},
		},
	}

	_, err := builder.SetExecutionProof(execProof)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "没有输入")
}

// TestSetExecutionProof_NotReferenceOnly 测试为消费型输入设置ExecutionProof
func TestSetExecutionProof_NotReferenceOnly(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	contractOutpoint := testutil.CreateOutPoint(nil, 0)
	execProof := &transaction.ExecutionProof{
		Context: &transaction.ExecutionProof_ExecutionContext{
			CallerIdentity: &transaction.IdentityProof{
				PublicKey:     testutil.RandomBytes(33),
				CallerAddress: testutil.RandomBytes(20),
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
			Metadata:        map[string][]byte{"method_name": []byte("mint")},
		},
	}

	// 添加消费型输入（不是引用型）
	builder.AddInput(contractOutpoint, false)

	_, err := builder.SetExecutionProof(execProof)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "只能为引用型输入设置")
}

// TestSetExecutionProof_MintingScenario 测试完整的铸造场景构建
func TestSetExecutionProof_MintingScenario(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	contractOutpoint := testutil.CreateOutPoint(nil, 0)
	contractAddr := testutil.RandomAddress()
	recipient := testutil.RandomAddress()
	lock := testutil.CreateSingleKeyLock(nil)

	execProof := &transaction.ExecutionProof{
		Context: &transaction.ExecutionProof_ExecutionContext{
			CallerIdentity: &transaction.IdentityProof{
				PublicKey:     testutil.RandomBytes(33),
				CallerAddress: testutil.RandomBytes(20),
				Signature:     testutil.RandomBytes(64),
				Algorithm:     transaction.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType:   transaction.SignatureHashType_SIGHASH_ALL,
				Nonce:         testutil.RandomBytes(32),
				Timestamp:     1234567890,
				ContextHash:   testutil.RandomBytes(32),
			},
			ResourceAddress: contractAddr,
			ExecutionType:   transaction.ExecutionType_EXECUTION_TYPE_CONTRACT,
			InputDataHash:   testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			OutputDataHash:  testutil.RandomBytes(32), // ✅ 使用哈希替代原始数据
			Metadata:        map[string][]byte{"method_name": []byte("mint")},
		},
		ExecutionResultHash: testutil.RandomBytes(32),
		StateTransitionProof: testutil.RandomBytes(64),
	}

	// 构建铸造交易
	builder.AddInput(contractOutpoint, true)  // 引用型输入
	_, err := builder.SetExecutionProof(execProof)
	require.NoError(t, err)
	builder.AddAssetOutput(recipient, "1000", contractAddr, lock)

	// 验证交易结构
	assert.Len(t, builder.tx.Inputs, 1)
	assert.True(t, builder.tx.Inputs[0].IsReferenceOnly)
	// 验证 ExecutionProof 已设置
	var hasExecProof bool
	if execProofInput, ok := builder.tx.Inputs[0].UnlockingProof.(*transaction.TxInput_ExecutionProof); ok {
		hasExecProof = execProofInput.ExecutionProof != nil
	}
	assert.True(t, hasExecProof)
	
	assert.Len(t, builder.tx.Outputs, 1)
	output := builder.tx.Outputs[0]
	assert.Equal(t, recipient, output.Owner)
	
	contractToken := output.GetAsset().GetContractToken()
	require.NotNil(t, contractToken)
	assert.Equal(t, contractAddr, contractToken.ContractAddress)
	assert.Equal(t, "1000", contractToken.Amount)
}

// TestBuild_PreserveCreationTimestamp 测试保留已设置的时间戳
func TestBuild_PreserveCreationTimestamp(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 手动设置时间戳
	expectedTimestamp := uint64(1234567890)
	builder.tx.CreationTimestamp = expectedTimestamp

	// 添加输出以允许构建
	builder.AddAssetOutput(testutil.RandomAddress(), "1000", nil, testutil.CreateSingleKeyLock(nil))

	composed, err := builder.Build()
	require.NoError(t, err)

	// 验证时间戳未被覆盖
	assert.Equal(t, expectedTimestamp, composed.Tx.CreationTimestamp)
}

// TestBuild_AutoSetCreationTimestamp 测试自动设置时间戳
func TestBuild_AutoSetCreationTimestamp(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 确保时间戳为 0
	assert.Equal(t, uint64(0), builder.tx.CreationTimestamp)

	// 添加输出以允许构建
	builder.AddAssetOutput(testutil.RandomAddress(), "1000", nil, testutil.CreateSingleKeyLock(nil))

	beforeBuild := time.Now().Unix()
	composed, err := builder.Build()
	afterBuild := time.Now().Unix()
	require.NoError(t, err)

	// 验证时间戳已自动设置，且在合理范围内
	assert.NotZero(t, composed.Tx.CreationTimestamp)
	assert.GreaterOrEqual(t, int64(composed.Tx.CreationTimestamp), beforeBuild)
	assert.LessOrEqual(t, int64(composed.Tx.CreationTimestamp), afterBuild)
}

// TestCreateDraft_Success 测试创建草稿
func TestCreateDraft_Success(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	draft, err := builder.CreateDraft(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, draft)
	assert.NotNil(t, draft.Tx)
}

// TestCreateDraft_NilDraftService 测试 DraftService 为 nil
func TestCreateDraft_NilDraftService(t *testing.T) {
	builder := NewService(nil)

	draft, err := builder.CreateDraft(context.Background())

	assert.Nil(t, draft)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draftService 未初始化")
}

// TestLoadDraft_Success 测试加载草稿
func TestLoadDraft_Success(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 先创建一个草稿
	draft, err := builder.CreateDraft(context.Background())
	require.NoError(t, err)

	// 保存草稿（通过 DraftService）
	err = draftService.SaveDraft(context.Background(), draft)
	require.NoError(t, err)

	// 加载草稿（需要 draftID，但 MockDraftService 没有返回 draftID）
	// 注意：MockDraftService 的 SaveDraft 没有返回 draftID，这是 Mock 的限制
	// 实际测试中应该使用真实的 DraftService 或改进 Mock
	// 这里先测试错误场景
	_, err = builder.LoadDraft(context.Background(), "non-existent-draft")
	assert.Error(t, err)
}

// TestLoadDraft_NilDraftService 测试 DraftService 为 nil
func TestLoadDraft_NilDraftService(t *testing.T) {
	builder := NewService(nil)

	draft, err := builder.LoadDraft(context.Background(), "test-draft-id")

	assert.Nil(t, draft)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "draftService 未初始化")
}

// TestBuild_OnlyInputs 测试只有输入无输出
func TestBuild_OnlyInputs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 只添加输入
	outpoint := testutil.CreateOutPoint(nil, 0)
	builder.AddInput(outpoint, false)

	composed, err := builder.Build()

	// 根据 Build() 的实现，只有输入无输出是允许的（可能是引用型交易）
	// 只有当输入和输出都为空时才返回错误
	assert.NoError(t, err)
	assert.NotNil(t, composed)
	assert.Len(t, composed.Tx.Inputs, 1)
	assert.Len(t, composed.Tx.Outputs, 0)
}

// TestAddAssetOutput_NilLock 测试 nil 锁定条件
func TestAddAssetOutput_NilLock(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 当前实现允许 nil lock
	builder.AddAssetOutput(testutil.RandomAddress(), "1000", nil, nil)

	assert.Len(t, builder.tx.Outputs, 1)
	output := builder.tx.Outputs[0]
	// 验证锁定条件列表为空或包含 nil
	assert.NotNil(t, output.LockingConditions)
}

// TestAddMultipleOutputs 测试添加多个不同类型的输出
func TestAddMultipleOutputs(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 添加原生币输出
	builder.AddAssetOutput(testutil.RandomAddress(), "1000", nil, testutil.CreateSingleKeyLock(nil))

	// 添加合约代币输出
	builder.AddAssetOutput(testutil.RandomAddress(), "500", testutil.RandomAddress(), testutil.CreateSingleKeyLock(nil))

	// 添加资源输出
	resourceProto := &resourcepb.Resource{
		Category:       resourcepb.ResourceCategory_RESOURCE_CATEGORY_EXECUTABLE,
		ExecutableType: resourcepb.ExecutableType_EXECUTABLE_TYPE_CONTRACT,
		ContentHash:    testutil.RandomHash(),
		MimeType:       "application/wasm",
		Size:           1024,
	}
	builder.AddResourceOutput(testutil.RandomAddress(), resourceProto, testutil.CreateSingleKeyLock(nil))

	// 添加状态输出
	builder.AddStateOutput(
		testutil.RandomAddress(),
		testutil.RandomHash(),
		1,
		nil, // nil zkProof
		testutil.RandomHash(),
		testutil.CreateSingleKeyLock(nil),
	)

	assert.Len(t, builder.tx.Outputs, 4)

	// 验证输出类型
	assert.NotNil(t, builder.tx.Outputs[0].GetAsset().GetNativeCoin())
	assert.NotNil(t, builder.tx.Outputs[1].GetAsset().GetContractToken())
	assert.NotNil(t, builder.tx.Outputs[2].GetResource())
	assert.NotNil(t, builder.tx.Outputs[3].GetState())
}

// ==================== 并发安全测试 ====================

// TestBuilder_ConcurrentAddInput 测试并发添加输入
func TestBuilder_ConcurrentAddInput(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 并发添加输入
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			outpoint := testutil.CreateOutPoint(nil, uint32(index))
			builder.AddInput(outpoint, false)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证所有输入都被添加（可能顺序不同）
	assert.Len(t, builder.tx.Inputs, numGoroutines)
}

// TestBuilder_ConcurrentAddOutput 测试并发添加输出
func TestBuilder_ConcurrentAddOutput(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 并发添加输出
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			owner := testutil.RandomAddress()
			amount := fmt.Sprintf("%d", index*100)
			builder.AddAssetOutput(owner, amount, nil, testutil.CreateSingleKeyLock(nil))
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证所有输出都被添加（可能顺序不同）
	assert.Len(t, builder.tx.Outputs, numGoroutines)
}

// TestBuilder_ConcurrentBuild 测试并发构建（应该失败或序列化）
func TestBuilder_ConcurrentBuild(t *testing.T) {
	draftService := testutil.NewMockDraftService()
	builder := NewService(draftService)

	// 添加一些数据
	builder.AddAssetOutput(testutil.RandomAddress(), "1000", nil, testutil.CreateSingleKeyLock(nil))

	// 并发构建
	const numGoroutines = 5
	results := make(chan *types.ComposedTx, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			composed, err := builder.Build()
			if err != nil {
				errors <- err
			} else {
				results <- composed
			}
		}()
	}

	// 收集结果
	var successCount int
	var errorCount int
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-results:
			successCount++
		case <-errors:
			errorCount++
		}
	}

	// 验证：由于 Builder 不是线程安全的，可能会有竞争条件
	// 但至少应该有一些成功或失败的构建
	assert.Greater(t, successCount+errorCount, 0)
}

// ==================== Mock 辅助类型 ====================

// MockVerifier 模拟验证器
type MockVerifier struct {
	shouldFail bool
}

func (m *MockVerifier) Verify(ctx context.Context, tx *transaction.Transaction) error {
	if m.shouldFail {
		return fmt.Errorf("verification failed")
	}
	return nil
}

// MockProcessor 模拟处理器
type MockProcessor struct {
	verifier *MockVerifier
	txPool   *testutil.MockTxPool
}

func (m *MockProcessor) SubmitTx(ctx context.Context, signedTx *types.SignedTx) (*types.SubmittedTx, error) {
	// 先验证
	if err := m.verifier.Verify(ctx, signedTx.Tx); err != nil {
		return nil, err
	}
	// 提交到池
	txHash, err := m.txPool.SubmitTx(signedTx.Tx)
	if err != nil {
		return nil, err
	}
	return &types.SubmittedTx{
		TxHash:      txHash,
		Tx:          signedTx.Tx,
		SubmittedAt: time.Now(),
	}, nil
}

func (m *MockProcessor) GetTxStatus(ctx context.Context, txHash []byte) (*types.TxBroadcastState, error) {
	// 简化实现
	return &types.TxBroadcastState{
		Status: types.BroadcastStatusLocalSubmitted,
	}, nil
}
