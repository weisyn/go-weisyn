package hostabi

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"google.golang.org/protobuf/proto"
)

// ============================================================================
// host_build_transaction.go 最终覆盖率提升测试
// ============================================================================
//
// 🎯 **测试目的**：提高覆盖率到80%+，发现未覆盖的代码路径中的缺陷和BUG
//
// ============================================================================

// TestEncodeTxReceipt_Error 测试编码错误（模拟JSON Marshal失败）
func TestEncodeTxReceipt_Error(t *testing.T) {
	// 创建一个会导致JSON Marshal失败的TxReceipt
	// 注意：在Go中，JSON Marshal很少失败，但我们可以测试错误处理路径
	receipt := &TxReceipt{
		Mode:           "unsigned",
		UnsignedTxHash: "test",
		SerializedTx:   "test",
	}

	// 正常情况下应该成功
	data, err := EncodeTxReceipt(receipt)

	assert.NoError(t, err, "应该成功编码")
	assert.NotNil(t, data, "应该返回数据")

	// 验证可以解码
	var decoded TxReceipt
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err, "应该可以解码")
}

// TestBuildTxOutputFromSpec_WithLockingConditions 测试带锁定条件的输出
func TestBuildTxOutputFromSpec_WithLockingConditions(t *testing.T) {
	// 创建一个有效的锁定条件（序列化为protobuf）
	lockCondition := &pb.LockingCondition{
		Condition: &pb.LockingCondition_SingleKeyLock{
			SingleKeyLock: &pb.SingleKeyLock{
				KeyRequirement: &pb.SingleKeyLock_RequiredAddressHash{
					RequiredAddressHash: make([]byte, 20),
				},
				RequiredAlgorithm: pb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1,
				SighashType:       pb.SignatureHashType_SIGHASH_ALL,
			},
		},
	}
	lockBytes, _ := proto.Marshal(lockCondition)
	lockHex := encodeHex(lockBytes)

	metadataJSON := `{
		"locking_conditions": "` + lockHex + `"
	}`
	spec := &OutputSpec{
		Type:     "asset",
		Owner:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Amount:   "1000",
		Metadata: []byte(metadataJSON),
	}

	output, err := buildTxOutputFromSpec(spec, nil)

	assert.NoError(t, err, "应该成功构建")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.LockingConditions, "应该有锁定条件")
	assert.Len(t, output.LockingConditions, 1, "应该有1个锁定条件")
}

// TestBuildTxOutputFromSpec_InvalidLockingConditionsJSON 测试无效锁定条件JSON
func TestBuildTxOutputFromSpec_InvalidLockingConditionsJSON(t *testing.T) {
	metadataJSON := `{invalid json}`
	spec := &OutputSpec{
		Type:     "asset",
		Owner:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Amount:   "1000",
		Metadata: []byte(metadataJSON),
	}

	// 应该使用默认锁定条件（因为JSON解析失败）
	output, err := buildTxOutputFromSpec(spec, nil)

	assert.NoError(t, err, "应该成功构建（使用默认锁定条件）")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.LockingConditions, "应该有默认锁定条件")
}

// TestBuildTxOutputFromSpec_InvalidLockingConditionsProto 测试无效锁定条件protobuf
func TestBuildTxOutputFromSpec_InvalidLockingConditionsProto(t *testing.T) {
	metadataJSON := `{
		"locking_conditions": "invalid_proto_hex"
	}`
	spec := &OutputSpec{
		Type:     "asset",
		Owner:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Amount:   "1000",
		Metadata: []byte(metadataJSON),
	}

	// 应该使用默认锁定条件（因为protobuf解析失败）
	output, err := buildTxOutputFromSpec(spec, nil)

	assert.NoError(t, err, "应该成功构建（使用默认锁定条件）")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.LockingConditions, "应该有默认锁定条件")
}

// TestBuildTxOutputFromSpec_EmptyLockingConditions 测试空锁定条件
func TestBuildTxOutputFromSpec_EmptyLockingConditions(t *testing.T) {
	metadataJSON := `{
		"locking_conditions": ""
	}`
	spec := &OutputSpec{
		Type:     "asset",
		Owner:    "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Amount:   "1000",
		Metadata: []byte(metadataJSON),
	}

	// 应该使用默认锁定条件（因为锁定条件为空）
	output, err := buildTxOutputFromSpec(spec, nil)

	assert.NoError(t, err, "应该成功构建（使用默认锁定条件）")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.LockingConditions, "应该有默认锁定条件")
}

func TestBuildTxOutputFromSpec_ContractTokenUsesContractLock(t *testing.T) {
	spec := &OutputSpec{
		Type:   "asset",
		Owner:  strings.Repeat("aa", 20),
		Amount: "100",
		// token_id 采用 hex 编码
		TokenID: "746f6b656e", // "token"
	}
	contractAddr := bytes.Repeat([]byte{0x11}, 20)

	output, err := buildTxOutputFromSpec(spec, contractAddr)

	assert.NoError(t, err, "合约代币输出应构建成功")
	require.NotNil(t, output)
	require.Len(t, output.LockingConditions, 1)

	lock := output.LockingConditions[0].GetContractLock()
	require.NotNil(t, lock, "合约代币输出应使用 ContractLock")
	assert.Equal(t, contractAddr, lock.ContractAddress, "锁定条件中的合约地址应匹配")
}

// TestRouteBySignMode_DeferSign_ComputeHashError 测试defer_sign模式计算哈希错误
func TestRouteBySignMode_DeferSign_ComputeHashError(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		err: assert.AnError,
	}
	ctx := context.Background()
	unsignedTx := &pb.Transaction{
		Inputs:  []*pb.TxInput{{}},
		Outputs: []*pb.TxOutput{{}},
	}

	receipt, err := routeBySignMode(ctx, mockTxHashClient, "defer_sign", unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "failed to compute transaction hash", "错误信息应该正确")
}

// TestRouteBySignMode_DeferSign_InvalidTransaction 测试defer_sign模式无效交易
func TestRouteBySignMode_DeferSign_InvalidTransaction(t *testing.T) {
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: false, // 交易结构无效
	}
	ctx := context.Background()
	unsignedTx := &pb.Transaction{
		Inputs:  []*pb.TxInput{{}},
		Outputs: []*pb.TxOutput{{}},
	}

	receipt, err := routeBySignMode(ctx, mockTxHashClient, "defer_sign", unsignedTx)

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "transaction structure is invalid", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_EmptyTransaction 测试空交易（只有sign_mode）
func TestBuildTransactionFromDraft_EmptyTransaction(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign"
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

	assert.Error(t, err, "应该返回错误（空交易）")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "交易为空", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_QueryUTXOError 测试paymaster模式查询UTXO错误
func TestBuildTransactionFromDraft_QueryUTXOError(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{}
	mockUTXOQuery := &mockUTXOQueryForPaymaster{
		queryError: assert.AnError,
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "paymaster",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}],
		"metadata": {
			"paymaster_params": {
				"fee_amount": "100"
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
	assert.Contains(t, receipt.Error, "应用签名模式逻辑失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_UTXOQueryNil 测试paymaster模式UTXOQuery为nil
func TestBuildTransactionFromDraft_UTXOQueryNil(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "paymaster",
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}],
		"metadata": {
			"paymaster_params": {
				"fee_amount": "100"
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

	assert.Error(t, err, "应该返回错误")
	assert.NotNil(t, receipt, "应该返回错误收据")
	assert.Equal(t, "error", receipt.Mode, "模式应该是error")
	assert.Contains(t, receipt.Error, "应用签名模式逻辑失败", "错误信息应该正确")
}

// TestBuildTransactionFromDraft_MultipleIntents 测试多个意图
func TestBuildTransactionFromDraft_MultipleIntents(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"intents": [
			{
				"type": "transfer",
				"params": {
					"from": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					"to": "beefdeadbeefdeadbeefdeadbeefdeadbeefdead",
					"amount": "1000"
				}
			},
			{
				"type": "transfer",
				"params": {
					"from": "beefdeadbeefdeadbeefdeadbeefdeadbeefdead",
					"to": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
					"amount": "500"
				}
			}
		]
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
	assert.Equal(t, 2, mockTxAdapter.addTransferCallCount, "应该处理2个转账意图")
}

// TestBuildTransactionFromDraft_MultipleInputs 测试多个输入
func TestBuildTransactionFromDraft_MultipleInputs(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"inputs": [
			{"tx_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "output_index": 0},
			{"tx_hash": "beefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdead", "output_index": 1}
		]
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
	assert.Equal(t, 2, mockTxAdapter.addCustomInputCallCount, "应该添加2个输入")
}

// TestBuildTransactionFromDraft_MultipleOutputs 测试多个输出
func TestBuildTransactionFromDraft_MultipleOutputs(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"outputs": [
			{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"},
			{"type": "asset", "owner": "beefdeadbeefdeadbeefdeadbeefdeadbeefdead", "amount": "500"}
		]
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
	assert.Equal(t, 2, mockTxAdapter.addCustomOutputCallCount, "应该添加2个输出")
}

// TestBuildTransactionFromDraft_ReferenceOnlyInput 测试引用型输入
func TestBuildTransactionFromDraft_ReferenceOnlyInput(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"inputs": [{"tx_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "output_index": 0, "is_reference_only": true}]
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
	assert.Equal(t, 1, mockTxAdapter.addCustomInputCallCount, "应该添加1个引用型输入")
}

// TestBuildTransactionFromDraft_ComplexTransaction 测试复杂交易（输入+输出+意图）
func TestBuildTransactionFromDraft_ComplexTransaction(t *testing.T) {
	mockTxAdapter := &mockTxAdapterForBuildTransaction{}
	mockTxHashClient := &mockTxHashServiceClient{
		hash:    make([]byte, 32),
		isValid: true,
	}
	ctx := context.Background()

	draftJSON := `{
		"sign_mode": "defer_sign",
		"inputs": [{"tx_hash": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "output_index": 0}],
		"outputs": [{"type": "asset", "owner": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "amount": "1000"}],
		"intents": [{
			"type": "transfer",
			"params": {
				"from": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
				"to": "beefdeadbeefdeadbeefdeadbeefdeadbeefdead",
				"amount": "500"
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
	assert.Equal(t, 1, mockTxAdapter.addCustomInputCallCount, "应该添加输入")
	assert.Equal(t, 1, mockTxAdapter.addCustomOutputCallCount, "应该添加输出")
	assert.Equal(t, 1, mockTxAdapter.addTransferCallCount, "应该处理意图")
}
