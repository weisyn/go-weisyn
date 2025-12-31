package adapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
// SDKAdapter测试
// ============================================================================
//
// 🎯 **测试目的**：发现SDKAdapter的缺陷和BUG
//
// ============================================================================

// mockUnifiedTransactionFacade Mock的UnifiedTransactionFacade
type mockUnifiedTransactionFacade struct {
	composeFunc func(ctx context.Context, intents interface{}) (*types.DraftTx, error)
}

func (m *mockUnifiedTransactionFacade) Compose(ctx context.Context, intents interface{}) (*types.DraftTx, error) {
	if m.composeFunc != nil {
		return m.composeFunc(ctx, intents)
	}
	return &types.DraftTx{}, nil
}

// TestNewSDKAdapter 测试创建SDK适配器
func TestNewSDKAdapter(t *testing.T) {
	facade := &mockUnifiedTransactionFacade{}
	adapter := NewSDKAdapter(facade)

	assert.NotNil(t, adapter, "适配器不应该为nil")
	assert.Equal(t, facade, adapter.facade, "facade应该正确注入")
}

// TestNewSDKAdapter_NilFacade 测试nil facade
// 🐛 **BUG检测**：nil facade可能导致panic
func TestNewSDKAdapter_NilFacade(t *testing.T) {
	adapter := NewSDKAdapter(nil)
	assert.NotNil(t, adapter, "即使facade为nil也应该创建适配器")
	assert.Nil(t, adapter.facade, "facade应该为nil")
}

// TestBuildTransaction_Success 测试构建交易（成功路径）
func TestBuildTransaction_Success(t *testing.T) {
	facade := &mockUnifiedTransactionFacade{
		composeFunc: func(ctx context.Context, intents interface{}) (*types.DraftTx, error) {
			return &types.DraftTx{}, nil
		},
	}
	adapter := NewSDKAdapter(facade)

	draftJSON := []byte(`{
		"outputs": [
			{
				"type": "asset",
				"to": "YWRkcmVzczEyMzQ1Njc4OTA=",
				"amount": 1000
			}
		],
		"intents": [
			{
				"type": "transfer",
				"from": "YWRkcmVzczEyMzQ1Njc4OTA=",
				"to": "YWRkcmVzczk4NzY1NDMyMTA=",
				"amount": 500
			}
		]
	}`)

	ctx := context.Background()
	draft, err := adapter.BuildTransaction(ctx, draftJSON)
	require.NoError(t, err, "成功路径不应该返回错误")
	assert.NotNil(t, draft, "交易草稿不应该为nil")
}

// TestBuildTransaction_InvalidJSON 测试无效JSON
func TestBuildTransaction_InvalidJSON(t *testing.T) {
	facade := &mockUnifiedTransactionFacade{}
	adapter := NewSDKAdapter(facade)

	invalidJSON := []byte(`{"outputs": [`)

	ctx := context.Background()
	draft, err := adapter.BuildTransaction(ctx, invalidJSON)
	assert.Error(t, err, "无效JSON应该返回错误")
	assert.Nil(t, draft, "交易草稿应该为nil")
	assert.Contains(t, err.Error(), "failed to parse SDK draft", "错误信息应该提到解析失败")
}

// TestBuildTransaction_EmptyDraft 测试空draft
// 🐛 **BUG检测**：空draft应该返回错误
func TestBuildTransaction_EmptyDraft(t *testing.T) {
	facade := &mockUnifiedTransactionFacade{}
	adapter := NewSDKAdapter(facade)

	emptyDraft := []byte(`{
		"outputs": [],
		"intents": []
	}`)

	ctx := context.Background()
	draft, err := adapter.BuildTransaction(ctx, emptyDraft)
	assert.Error(t, err, "空draft应该返回错误")
	assert.Nil(t, draft, "交易草稿应该为nil")
	assert.Contains(t, err.Error(), "必须包含至少一个输出或意图", "错误信息应该提到空draft")
}

// TestBuildTransaction_FacadeError 测试Facade返回错误
func TestBuildTransaction_FacadeError(t *testing.T) {
	facade := &mockUnifiedTransactionFacade{
		composeFunc: func(ctx context.Context, intents interface{}) (*types.DraftTx, error) {
			return nil, &dummyError{msg: "insufficient balance for transfer"}
		},
	}
	adapter := NewSDKAdapter(facade)

	draftJSON := []byte(`{
		"outputs": [{"type": "asset", "amount": 1000}],
		"intents": [{"type": "transfer", "amount": 500}]
	}`)

	ctx := context.Background()
	draft, err := adapter.BuildTransaction(ctx, draftJSON)
	assert.Error(t, err, "Facade错误应该被转换")
	assert.Nil(t, draft, "交易草稿应该为nil")
	assert.Contains(t, err.Error(), "余额不足", "错误应该被转换为中文")
}

// TestConvertToTxIntents_NilDraft 测试nil draft
// 🐛 **BUG检测**：nil draft应该返回错误
func TestConvertToTxIntents_NilDraft(t *testing.T) {
	adapter := &SDKAdapter{}

	ctx := context.Background()
	intents, err := adapter.convertToTxIntents(ctx, nil)
	assert.Error(t, err, "nil draft应该返回错误")
	assert.Nil(t, intents, "intents应该为nil")
	assert.Contains(t, err.Error(), "SDK draft不能为空", "错误信息应该提到draft为空")
}

// TestConvertToTxIntents_ValidDraft 测试有效的draft
func TestConvertToTxIntents_ValidDraft(t *testing.T) {
	adapter := &SDKAdapter{}

	sdkDraft := &SDKDraft{
		Outputs: []SDKOutput{
			{Type: "asset", Amount: 1000},
		},
		Intents: []SDKIntent{
			{Type: "transfer", Amount: 500},
		},
	}

	ctx := context.Background()
	intents, err := adapter.convertToTxIntents(ctx, sdkDraft)
	require.NoError(t, err, "有效draft不应该返回错误")
	assert.NotNil(t, intents, "intents不应该为nil")
	
	// 验证返回的是SDKDraft
	returnedDraft, ok := intents.(*SDKDraft)
	assert.True(t, ok, "返回的应该是*SDKDraft类型")
	assert.Equal(t, sdkDraft, returnedDraft, "返回的draft应该与输入相同")
}

// TestConvertError 测试错误转换
func TestConvertError(t *testing.T) {
	adapter := &SDKAdapter{}

	testCases := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:     "insufficient balance",
			input:    &dummyError{msg: "insufficient balance for transfer"},
			expected: "余额不足",
		},
		{
			name:     "invalid parameter",
			input:    &dummyError{msg: "invalid parameter provided"},
			expected: "参数无效",
		},
		{
			name:     "invalid state",
			input:    &dummyError{msg: "invalid state"},
			expected: "状态无效",
		},
		{
			name:     "not found",
			input:    &dummyError{msg: "not found"},
			expected: "未找到",
		},
		{
			name:     "permission denied",
			input:    &dummyError{msg: "permission denied"},
			expected: "权限不足",
		},
		{
			name:     "unknown error",
			input:    &dummyError{msg: "unknown error"},
			expected: "unknown error", // 未知错误应该原样返回
		},
		{
			name:     "nil error",
			input:    nil,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := adapter.convertError(tc.input)
			if tc.input == nil {
				assert.Nil(t, result, "nil错误应该返回nil")
			} else {
				assert.NotNil(t, result, "非nil错误应该返回非nil")
				if tc.expected != "" {
					assert.Contains(t, result.Error(), tc.expected, "错误信息应该包含预期内容")
				}
			}
		})
	}
}

// TestParseSDKDraft_ValidJSON 测试解析有效JSON
func TestParseSDKDraft_ValidJSON(t *testing.T) {
	adapter := &SDKAdapter{}

	validJSON := []byte(`{
		"outputs": [
			{
				"type": "asset",
				"to": "YWRkcmVzczEyMzQ1Njc4OTA=",
				"token_id": "dG9rZW5faWQ=",
				"amount": 1000,
				"state_id": "c3RhdGVfaWQ=",
				"version": 1,
				"exec_hash": "ZXhlY19oYXNo",
				"resource": "cmVzb3VyY2U="
			}
		],
		"intents": [
			{
				"type": "transfer",
				"from": "YWRkcmVzczEyMzQ1Njc4OTA=",
				"to": "YWRkcmVzczk4NzY1NDMyMTA=",
				"token_id": "dG9rZW5faWQ=",
				"amount": 500,
				"staker": "c3Rha2Vy",
				"validator": "dmFsaWRhdG9y"
			}
		]
	}`)

	draft, err := adapter.parseSDKDraft(validJSON)
	require.NoError(t, err, "有效JSON不应该返回错误")
	assert.NotNil(t, draft, "draft不应该为nil")
	assert.Equal(t, 1, len(draft.Outputs), "应该有1个输出")
	assert.Equal(t, 1, len(draft.Intents), "应该有1个意图")
	
	// 验证输出字段
	output := draft.Outputs[0]
	assert.Equal(t, "asset", output.Type)
	assert.Equal(t, "YWRkcmVzczEyMzQ1Njc4OTA=", output.To)
	assert.Equal(t, "dG9rZW5faWQ=", output.TokenID)
	assert.Equal(t, uint64(1000), output.Amount)
	
	// 验证意图字段
	intent := draft.Intents[0]
	assert.Equal(t, "transfer", intent.Type)
	assert.Equal(t, "YWRkcmVzczEyMzQ1Njc4OTA=", intent.From)
	assert.Equal(t, "YWRkcmVzczk4NzY1NDMyMTA=", intent.To)
	assert.Equal(t, uint64(500), intent.Amount)
}

// TestParseSDKDraft_InvalidJSON 测试解析无效JSON
func TestParseSDKDraft_InvalidJSON(t *testing.T) {
	adapter := &SDKAdapter{}

	invalidJSON := []byte(`{"outputs": [`)

	draft, err := adapter.parseSDKDraft(invalidJSON)
	assert.Error(t, err, "无效JSON应该返回错误")
	assert.Nil(t, draft, "draft应该为nil")
	assert.Contains(t, err.Error(), "invalid JSON", "错误信息应该提到无效JSON")
}

// TestParseSDKDraft_EmptyJSON 测试空JSON
func TestParseSDKDraft_EmptyJSON(t *testing.T) {
	adapter := &SDKAdapter{}

	emptyJSON := []byte(`{}`)

	draft, err := adapter.parseSDKDraft(emptyJSON)
	require.NoError(t, err, "空JSON不应该返回错误")
	assert.NotNil(t, draft, "draft不应该为nil")
	assert.Equal(t, 0, len(draft.Outputs), "输出应该为空")
	assert.Equal(t, 0, len(draft.Intents), "意图应该为空")
}

