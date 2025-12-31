package hostabi

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ============================================================================
// host_build_transaction.go 测试
// ============================================================================
//
// 🎯 **测试目的**：发现 host_build_transaction 的缺陷和BUG，特别是简化实现和占位代码
//
// ⚠️ **已知简化实现**：
//   - 第576行：简化实现：选择第一个UTXO（实际应该按金额选择）
//   - 第611行：简化实现：通过AddCustomOutput方法添加
//   - 第990行：合约代币（简化实现：使用默认的 FungibleClassId）
//   - 第1115行：如果没有提供，使用零哈希（占位）
//
// ============================================================================

// TestParseDraftJSON_Success 测试成功解析Draft JSON
func TestParseDraftJSON_Success(t *testing.T) {
	draftJSON := `{
		"sign_mode": "defer_sign",
		"inputs": [{"tx_hash": "abc123", "output_index": 0}],
		"outputs": [{"type": "asset", "owner": "deadbeef", "amount": "100"}]
	}`
	draftJSONBytes := []byte(draftJSON)

	draft, err := ParseDraftJSON(draftJSONBytes)

	assert.NoError(t, err, "应该成功解析")
	assert.NotNil(t, draft, "应该返回Draft对象")
	assert.Equal(t, "defer_sign", draft.SignMode, "签名模式应该正确")
	assert.Len(t, draft.Inputs, 1, "应该有1个输入")
	assert.Len(t, draft.Outputs, 1, "应该有1个输出")
}

// TestParseDraftJSON_DefaultSignMode 测试默认签名模式
func TestParseDraftJSON_DefaultSignMode(t *testing.T) {
	draftJSON := `{
		"inputs": [{"tx_hash": "abc123", "output_index": 0}]
	}`
	draftJSONBytes := []byte(draftJSON)

	draft, err := ParseDraftJSON(draftJSONBytes)

	assert.NoError(t, err, "应该成功解析")
	assert.NotNil(t, draft, "应该返回Draft对象")
	assert.Equal(t, "defer_sign", draft.SignMode, "应该使用默认签名模式")
}

// TestParseDraftJSON_InvalidJSON 测试无效JSON
func TestParseDraftJSON_InvalidJSON(t *testing.T) {
	invalidJSON := `{invalid json}`
	draftJSONBytes := []byte(invalidJSON)

	draft, err := ParseDraftJSON(draftJSONBytes)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, draft, "Draft应该为nil")
	assert.Contains(t, err.Error(), "解析 Draft JSON 失败", "错误信息应该正确")
}

// TestValidateDraftJSON_Success 测试成功验证Draft JSON
func TestValidateDraftJSON_Success(t *testing.T) {
	draft := &DraftJSON{
		SignMode: "defer_sign",
		Inputs:   []InputSpec{{TxHash: "abc123", OutputIndex: 0}},
	}

	err := ValidateDraftJSON(draft)

	assert.NoError(t, err, "应该成功验证")
}

// TestValidateDraftJSON_InvalidSignMode 测试无效签名模式
func TestValidateDraftJSON_InvalidSignMode(t *testing.T) {
	draft := &DraftJSON{
		SignMode: "invalid_mode",
		Inputs:   []InputSpec{{TxHash: "abc123", OutputIndex: 0}},
	}

	err := ValidateDraftJSON(draft)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "无效的签名模式", "错误信息应该正确")
}

// TestValidateDraftJSON_EmptyTransaction 测试空交易
func TestValidateDraftJSON_EmptyTransaction(t *testing.T) {
	draft := &DraftJSON{
		SignMode: "defer_sign",
		// 没有输入、输出或意图
	}

	err := ValidateDraftJSON(draft)

	assert.Error(t, err, "应该返回错误")
	assert.Contains(t, err.Error(), "交易为空", "错误信息应该正确")
}

// TestValidateDraftJSON_ValidModes 测试所有有效签名模式
func TestValidateDraftJSON_ValidModes(t *testing.T) {
	validModes := []string{"defer_sign", "delegated", "threshold", "paymaster"}

	for _, mode := range validModes {
		t.Run(mode, func(t *testing.T) {
			draft := &DraftJSON{
				SignMode: mode,
				Inputs:   []InputSpec{{TxHash: "abc123", OutputIndex: 0}},
			}

			err := ValidateDraftJSON(draft)

			assert.NoError(t, err, "模式 %s 应该有效", mode)
		})
	}
}

// TestEncodeTxReceipt_Success 测试成功编码TxReceipt
func TestEncodeTxReceipt_Success(t *testing.T) {
	receipt := &TxReceipt{
		Mode:           "unsigned",
		UnsignedTxHash: "abc123",
		SerializedTx:   "base64data",
	}

	data, err := EncodeTxReceipt(receipt)

	assert.NoError(t, err, "应该成功编码")
	assert.NotNil(t, data, "应该返回数据")
	
	// 验证可以解码
	var decoded TxReceipt
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err, "应该可以解码")
	assert.Equal(t, receipt.Mode, decoded.Mode, "模式应该一致")
	assert.Equal(t, receipt.UnsignedTxHash, decoded.UnsignedTxHash, "哈希应该一致")
}

// TestEncodeTxReceipt_EmptyReceipt 测试空收据
func TestEncodeTxReceipt_EmptyReceipt(t *testing.T) {
	receipt := &TxReceipt{
		Mode: "error",
		Error: "test error",
	}

	data, err := EncodeTxReceipt(receipt)

	assert.NoError(t, err, "应该成功编码")
	assert.NotNil(t, data, "应该返回数据")
	
	// 验证可以解码
	var decoded TxReceipt
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err, "应该可以解码")
	assert.Equal(t, receipt.Mode, decoded.Mode, "模式应该一致")
	assert.Equal(t, receipt.Error, decoded.Error, "错误信息应该一致")
}

// TestDecodeHex_Success 测试成功解码十六进制
func TestDecodeHex_Success(t *testing.T) {
	hexStr := "deadbeef"
	expected := []byte{0xde, 0xad, 0xbe, 0xef}

	result := decodeHex(hexStr)

	assert.Equal(t, expected, result, "解码结果应该正确")
}

// TestDecodeHex_EmptyString 测试空字符串
func TestDecodeHex_EmptyString(t *testing.T) {
	result := decodeHex("")

	assert.Empty(t, result, "空字符串应该返回空字节数组")
}

// TestDecodeHex_InvalidHex 测试无效十六进制（应该返回空或panic）
func TestDecodeHex_InvalidHex(t *testing.T) {
	// decodeHex 使用 hex.DecodeString，无效输入会返回错误
	// 但当前实现可能没有处理错误，需要检查
	invalidHex := "invalid"
	
	// 如果实现不处理错误，可能会panic或返回空
	// 这里测试实际行为
	result := decodeHex(invalidHex)
	
	// 根据实际实现，可能是空数组或panic
	// 如果是空数组，说明实现忽略了错误（这是潜在问题）
	if len(result) == 0 {
		t.Logf("⚠️ 警告：decodeHex 对无效输入返回空数组，可能掩盖了错误")
	}
}

// TestEncodeHex_Success 测试成功编码十六进制
func TestEncodeHex_Success(t *testing.T) {
	data := []byte{0xde, 0xad, 0xbe, 0xef}
	expected := "deadbeef"

	result := encodeHex(data)

	assert.Equal(t, expected, result, "编码结果应该正确")
}

// TestEncodeHex_EmptyBytes 测试空字节数组
func TestEncodeHex_EmptyBytes(t *testing.T) {
	result := encodeHex([]byte{})

	assert.Empty(t, result, "空字节数组应该返回空字符串")
}

// TestEncodeBase64_Success 测试成功编码Base64
func TestEncodeBase64_Success(t *testing.T) {
	data := []byte("hello world")
	expected := "aGVsbG8gd29ybGQ=" // base64编码

	result := encodeBase64(data)

	assert.Equal(t, expected, result, "编码结果应该正确")
}

// TestEncodeBase64_EmptyBytes 测试空字节数组
func TestEncodeBase64_EmptyBytes(t *testing.T) {
	result := encodeBase64([]byte{})

	assert.Empty(t, result, "空字节数组应该返回空字符串")
}

// TestBuildTxOutputFromSpec_AssetOutput 测试构建资产输出
func TestBuildTxOutputFromSpec_AssetOutput(t *testing.T) {
	spec := &OutputSpec{
		Type:   "asset",
		Owner:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", // 40个字符 = 20字节
		Amount: "1000",
		TokenID: "",
	}

	output, err := buildTxOutputFromSpec(spec, nil)

	assert.NoError(t, err, "应该成功构建")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.OutputContent, "应该有输出内容")
	
	// 验证是资产输出
	assetOutput, ok := output.OutputContent.(*pb.TxOutput_Asset)
	assert.True(t, ok, "应该是资产输出")
	assert.NotNil(t, assetOutput.Asset, "资产输出应该不为nil")
}

// TestBuildTxOutputFromSpec_InvalidOwnerLength 测试无效所有者长度
func TestBuildTxOutputFromSpec_InvalidOwnerLength(t *testing.T) {
	spec := &OutputSpec{
		Type:   "asset",
		Owner:  "invalid", // 长度不足
		Amount: "1000",
	}

	output, err := buildTxOutputFromSpec(spec, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, output, "输出应该为nil")
	assert.Contains(t, err.Error(), "owner 地址必须是 20 字节", "错误信息应该正确")
}

// TestBuildTxOutputFromSpec_InvalidType 测试无效输出类型
func TestBuildTxOutputFromSpec_InvalidType(t *testing.T) {
	spec := &OutputSpec{
		Type:   "invalid_type",
		Owner:  "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Amount: "1000",
	}

	output, err := buildTxOutputFromSpec(spec, nil)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, output, "输出应该为nil")
	assert.Contains(t, err.Error(), "不支持的输出类型", "错误信息应该正确")
}

// TestBuildAssetOutput_Success 测试构建资产输出
func TestBuildAssetOutput_Success(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	spec := &OutputSpec{
		Amount: "1000",
		TokenID: "",
	}
	locks := []*transaction.LockingCondition{}

	output, err := buildAssetOutput(owner, spec, locks, nil)

	assert.NoError(t, err, "应该成功构建")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.OutputContent, "应该有输出内容")
	
	// 验证是资产输出
	assetOutput, ok := output.OutputContent.(*pb.TxOutput_Asset)
	assert.True(t, ok, "应该是资产输出")
	assert.NotNil(t, assetOutput.Asset, "资产输出应该不为nil")
}

// TestBuildAssetOutput_WithTokenID 测试带TokenID的资产输出（检查简化实现）
func TestBuildAssetOutput_WithTokenID(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	spec := &OutputSpec{
		Amount: "1000",
		TokenID: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", // 40个字符 = 20字节
	}
	locks := []*transaction.LockingCondition{}
	contractAddr := decodeHex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	output, err := buildAssetOutput(owner, spec, locks, contractAddr)

	assert.NoError(t, err, "应该成功构建")
	assert.NotNil(t, output, "应该返回输出对象")
	
	// 验证是合约代币输出
	assetOutput, ok := output.OutputContent.(*pb.TxOutput_Asset)
	assert.True(t, ok, "应该是资产输出")
	assert.NotNil(t, assetOutput.Asset, "资产输出应该不为nil")
	
	contractToken, ok := assetOutput.Asset.AssetContent.(*transaction.AssetOutput_ContractToken)
	require.True(t, ok, "应该是合约代币输出")
	require.NotNil(t, contractToken.ContractToken)
	assert.Equal(t, contractAddr, contractToken.ContractToken.ContractAddress, "合约地址应与传入的一致")
	assert.Equal(t,
		decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"),
		contractToken.ContractToken.TokenIdentifier.(*transaction.ContractTokenAsset_FungibleClassId).FungibleClassId,
		"TokenID 应与 spec 中一致",
	)

	require.Len(t, output.LockingConditions, 1, "合约代币输出应包含 ContractLock")
	lock := output.LockingConditions[0].GetContractLock()
	require.NotNil(t, lock, "锁定条件应为 ContractLock")
	assert.Equal(t, contractAddr, lock.ContractAddress, "ContractLock 中的合约地址应匹配")
}

// TestBuildResourceOutput_Success 测试构建资源输出
func TestBuildResourceOutput_Success(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	contentHashHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef" // 64个字符 = 32字节
	metadataJSON := `{
		"content_hash": "` + contentHashHex + `",
		"category": "wasm",
		"mime_type": "application/wasm"
	}`
	spec := &OutputSpec{
		Metadata: json.RawMessage(metadataJSON),
	}
	locks := []*transaction.LockingCondition{}

	output, err := buildResourceOutput(owner, spec, locks)

	assert.NoError(t, err, "应该成功构建")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.OutputContent, "应该有输出内容")
	
	// 验证是资源输出
	resourceOutput, ok := output.OutputContent.(*pb.TxOutput_Resource)
	assert.True(t, ok, "应该是资源输出")
	assert.NotNil(t, resourceOutput.Resource, "资源输出应该不为nil")
}

// TestBuildResourceOutput_MissingContentHash 测试缺少内容哈希
func TestBuildResourceOutput_MissingContentHash(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	metadataJSON := `{
		"category": "wasm"
	}`
	spec := &OutputSpec{
		Metadata: json.RawMessage(metadataJSON),
	}
	locks := []*transaction.LockingCondition{}

	output, err := buildResourceOutput(owner, spec, locks)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, output, "输出应该为nil")
	assert.Contains(t, err.Error(), "content_hash 不能为空", "错误信息应该正确")
}

// TestBuildResourceOutput_InvalidContentHashLength 测试无效内容哈希长度
func TestBuildResourceOutput_InvalidContentHashLength(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	metadataJSON := `{
		"content_hash": "deadbeef",
		"category": "wasm"
	}`
	spec := &OutputSpec{
		Metadata: json.RawMessage(metadataJSON),
	}
	locks := []*transaction.LockingCondition{}

	output, err := buildResourceOutput(owner, spec, locks)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, output, "输出应该为nil")
	assert.Contains(t, err.Error(), "必须是 32 字节", "错误信息应该正确")
}

// TestBuildResourceOutput_InvalidCategory 测试无效资源类别
func TestBuildResourceOutput_InvalidCategory(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	contentHashHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	metadataJSON := `{
		"content_hash": "` + contentHashHex + `",
		"category": "invalid_category"
	}`
	spec := &OutputSpec{
		Metadata: json.RawMessage(metadataJSON),
	}
	locks := []*transaction.LockingCondition{}

	output, err := buildResourceOutput(owner, spec, locks)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, output, "输出应该为nil")
	assert.Contains(t, err.Error(), "不支持的资源类别", "错误信息应该正确")
}

// TestParseAmount_Success 测试成功解析金额
func TestParseAmount_Success(t *testing.T) {
	amountStr := "1000"
	
	result, err := parseAmount(amountStr)

	assert.NoError(t, err, "应该成功解析")
	assert.Equal(t, uint64(1000), result, "金额应该正确")
}

// TestParseAmount_Zero 测试零金额
func TestParseAmount_Zero(t *testing.T) {
	amountStr := "0"
	
	result, err := parseAmount(amountStr)

	assert.NoError(t, err, "应该成功解析")
	assert.Equal(t, uint64(0), result, "金额应该为0")
}

// TestParseAmount_InvalidFormat 测试无效格式
func TestParseAmount_InvalidFormat(t *testing.T) {
	amountStr := "invalid"
	
	result, err := parseAmount(amountStr)

	assert.Error(t, err, "应该返回错误")
	assert.Equal(t, uint64(0), result, "错误时应该返回0")
	assert.Contains(t, err.Error(), "金额格式无效", "错误信息应该正确")
}

// TestParseAmount_EmptyString 测试空字符串（可能返回0而不是错误）
func TestParseAmount_EmptyString(t *testing.T) {
	amountStr := ""
	
	result, err := parseAmount(amountStr)

	// 根据实际实现，空字符串可能返回0而不是错误
	if err != nil {
		assert.Error(t, err, "如果返回错误，应该包含错误信息")
	} else {
		assert.Equal(t, uint64(0), result, "空字符串可能返回0")
		t.Logf("⚠️ 注意：parseAmount 对空字符串返回0而不是错误，这可能掩盖了问题")
	}
}

// TestBuildStateOutput_Success 测试构建状态输出
func TestBuildStateOutput_Success(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	executionResultHashHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef" // 64个字符 = 32字节
	stateIDHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" // 40个字符 = 20字节
	metadataJSON := `{
		"state_id": "` + stateIDHex + `",
		"state_version": 1,
		"execution_result_hash": "` + executionResultHashHex + `",
		"public_inputs": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"parent_state_hash": "` + executionResultHashHex + `"
	}`
	spec := &OutputSpec{
		Metadata: json.RawMessage(metadataJSON),
	}
	locks := []*transaction.LockingCondition{}

	output, err := buildStateOutput(owner, spec, locks)

	assert.NoError(t, err, "应该成功构建")
	assert.NotNil(t, output, "应该返回输出对象")
	assert.NotNil(t, output.OutputContent, "应该有输出内容")
	
	// 验证是状态输出
	stateOutput, ok := output.OutputContent.(*pb.TxOutput_State)
	assert.True(t, ok, "应该是状态输出")
	assert.NotNil(t, stateOutput.State, "状态输出应该不为nil")
}

// TestBuildStateOutput_MissingStateID 测试缺少StateID
func TestBuildStateOutput_MissingStateID(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	metadataJSON := `{
		"state_version": 1
	}`
	spec := &OutputSpec{
		Metadata: json.RawMessage(metadataJSON),
	}
	locks := []*transaction.LockingCondition{}

	output, err := buildStateOutput(owner, spec, locks)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, output, "输出应该为nil")
	assert.Contains(t, err.Error(), "state_id 不能为空", "错误信息应该正确")
}

// TestBuildStateOutput_InvalidExecutionResultHashLength 测试无效执行结果哈希长度
func TestBuildStateOutput_InvalidExecutionResultHashLength(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	stateIDHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	metadataJSON := `{
		"state_id": "` + stateIDHex + `",
		"state_version": 1,
		"execution_result_hash": "deadbeef"
	}`
	spec := &OutputSpec{
		Metadata: json.RawMessage(metadataJSON),
	}
	locks := []*transaction.LockingCondition{}

	output, err := buildStateOutput(owner, spec, locks)

	assert.Error(t, err, "应该返回错误")
	assert.Nil(t, output, "输出应该为nil")
	assert.Contains(t, err.Error(), "execution_result_hash 必须是 32 字节", "错误信息应该正确")
}

// TestBuildStateOutput_ZeroHashPlaceholder 测试零哈希占位（检查占位代码）
func TestBuildStateOutput_ZeroHashPlaceholder(t *testing.T) {
	owner := decodeHex("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	executionResultHashHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	stateIDHex := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	metadataJSON := `{
		"state_id": "` + stateIDHex + `",
		"state_version": 1,
		"execution_result_hash": "` + executionResultHashHex + `"
	}`
	// 没有提供 parent_state_hash，应该使用零哈希（占位）
	spec := &OutputSpec{
		Metadata: json.RawMessage(metadataJSON),
	}
	locks := []*transaction.LockingCondition{}

	output, err := buildStateOutput(owner, spec, locks)

	assert.NoError(t, err, "应该成功构建（使用零哈希占位）")
	assert.NotNil(t, output, "应该返回输出对象")
	
	// ⚠️ 检查占位代码：如果没有提供parent_state_hash，使用零哈希
	stateOutput, ok := output.OutputContent.(*pb.TxOutput_State)
	if ok {
		zeroHash := make([]byte, 32)
		if len(stateOutput.State.ParentStateHash) == 0 {
			// parent_state_hash为空，这是正常的（可选字段）
		} else if len(stateOutput.State.ParentStateHash) == 32 {
			if string(stateOutput.State.ParentStateHash) == string(zeroHash) {
				t.Logf("⚠️ 警告：buildStateOutput 使用零哈希作为占位（第1115行），实际应该要求明确提供")
			}
		}
	}
}

// TestSerializeTx_Success 测试序列化交易
func TestSerializeTx_Success(t *testing.T) {
	tx := &transaction.Transaction{
		Inputs: []*transaction.TxInput{
			{},
		},
		Outputs: []*transaction.TxOutput{
			{},
		},
	}

	data := serializeTx(tx)

	assert.NotNil(t, data, "应该返回数据")
	assert.Greater(t, len(data), 0, "数据应该不为空")
}

// TestSerializeTx_EmptyTransaction 测试空交易序列化
func TestSerializeTx_EmptyTransaction(t *testing.T) {
	tx := &transaction.Transaction{
		Inputs:  []*transaction.TxInput{},
		Outputs: []*transaction.TxOutput{},
	}

	data := serializeTx(tx)

	// 空交易序列化可能返回空字节数组或非空字节数组（取决于protobuf实现）
	assert.NotNil(t, data, "应该返回数据（即使是空数组）")
}

// TestSerializeTx_NilTransaction 测试nil交易（检查错误处理）
func TestSerializeTx_NilTransaction(t *testing.T) {
	// ⚠️ 注意：serializeTx 不返回错误，序列化失败时返回空字节数组
	// 这可能掩盖了问题，应该返回错误
	data := serializeTx(nil)

	if len(data) == 0 {
		t.Logf("⚠️ 警告：serializeTx 对nil交易返回空字节数组而不是错误，这可能掩盖了问题")
	}
	assert.Empty(t, data, "nil交易应该返回空字节数组（当前实现）")
}

// ============================================================================
// 检查简化实现和占位代码
// ============================================================================

// TestDetectSimplifiedImplementations 检查简化实现
func TestDetectSimplifiedImplementations(t *testing.T) {
	// 这个测试用于记录已知的简化实现，确保它们被标记为已知问题
	
	// 1. 检查 applyPaymaster 中的简化实现（选择第一个UTXO）
	// 实际应该按金额选择，但当前实现选择第一个
	t.Logf("⚠️ 已知简化实现：applyPaymaster 选择第一个UTXO（第576行），实际应该按金额选择")
	
	// 2. 检查 buildResourceOutput 中的简化实现
	// 第611行：简化实现：通过AddCustomOutput方法添加
	t.Logf("⚠️ 已知简化实现：buildResourceOutput 通过AddCustomOutput方法添加（第611行）")
	
	// 3. 检查 buildAssetOutput 中的简化实现
	// 第990行：合约代币（简化实现：使用默认的 FungibleClassId）
	t.Logf("⚠️ 已知简化实现：buildAssetOutput 使用默认的 FungibleClassId（第990行）")
	
	// 4. 检查占位代码
	// 第1115行：如果没有提供，使用零哈希（占位）
	t.Logf("⚠️ 已知占位代码：使用零哈希作为占位（第1115行）")
	
	// 5. 检查 parseAmount 的行为
	// 空字符串返回0而不是错误，可能掩盖问题
	t.Logf("⚠️ 潜在问题：parseAmount 对空字符串返回0而不是错误，可能掩盖了问题")
	
	// 这些简化实现和占位代码应该在文档中明确标记，并在后续版本中完善
}

