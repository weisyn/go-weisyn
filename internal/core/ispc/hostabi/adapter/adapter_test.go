package adapter

import (
	"testing"
)

// TestSDKAdapter_ParseSDKDraft 测试解析SDK draft
func TestSDKAdapter_ParseSDKDraft(t *testing.T) {
	adapter := &SDKAdapter{}

	// 测试用例1: 有效的JSON
	validJSON := `{
		"outputs": [
			{
				"type": "asset",
				"to": "YWRkcmVzczEyMzQ1Njc4OTA=",
				"token_id": "",
				"amount": 1000
			}
		],
		"intents": [
			{
				"type": "transfer",
				"from": "YWRkcmVzczEyMzQ1Njc4OTA=",
				"to": "YWRkcmVzczk4NzY1NDMyMTA=",
				"token_id": "",
				"amount": 500
			}
		]
	}`

	draft, err := adapter.parseSDKDraft([]byte(validJSON))
	if err != nil {
		t.Fatalf("failed to parse valid JSON: %v", err)
	}

	if len(draft.Outputs) != 1 {
		t.Errorf("expected 1 output, got %d", len(draft.Outputs))
	}

	if len(draft.Intents) != 1 {
		t.Errorf("expected 1 intent, got %d", len(draft.Intents))
	}

	// 测试用例2: 无效的JSON
	invalidJSON := `{"outputs": [`

	_, err = adapter.parseSDKDraft([]byte(invalidJSON))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestSDKAdapter_ConvertError 测试错误转换
func TestSDKAdapter_ConvertError(t *testing.T) {
	adapter := &SDKAdapter{}

	testCases := []struct {
		name     string
		input    error
		contains string
	}{
		{
			name:     "insufficient balance",
			input:    &dummyError{msg: "insufficient balance for transfer"},
			contains: "余额不足",
		},
		{
			name:     "invalid parameter",
			input:    &dummyError{msg: "invalid parameter provided"},
			contains: "参数无效",
		},
		{
			name:     "permission denied",
			input:    &dummyError{msg: "permission denied for user"},
			contains: "权限不足",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := adapter.convertError(tc.input)
			if result == nil {
				t.Fatal("expected non-nil error")
			}

			if !contains(result.Error(), tc.contains) {
				t.Errorf("expected error to contain %q, got %q", tc.contains, result.Error())
			}
		})
	}
}

// TestDecodeBase64 测试base64解码
func TestDecodeBase64(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []byte
		wantErr  bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "valid base64",
			input:    "SGVsbG8=",
			expected: []byte("Hello"),
			wantErr:  false,
		},
		{
			name:     "valid base64 with padding",
			input:    "SGVsbG8=",
			expected: []byte("Hello"),
			wantErr:  false,
		},
		{
			name:     "invalid base64",
			input:    "Hello!", // 无效的base64字符
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := decodeBase64(tc.input)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result) != len(tc.expected) {
				t.Errorf("expected length %d, got %d", len(tc.expected), len(result))
			}

			for i := range tc.expected {
				if i < len(result) && result[i] != tc.expected[i] {
					t.Errorf("byte %d: expected %d, got %d", i, tc.expected[i], result[i])
				}
			}
		})
	}
}

// dummyError 用于测试的错误类型
type dummyError struct {
	msg string
}

func (e *dummyError) Error() string {
	return e.msg
}
