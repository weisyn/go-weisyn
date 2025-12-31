// Package abi provides ABI validation and helper functions tests.
package abi

import (
	"encoding/json"
	"testing"

	"github.com/weisyn/v1/pkg/types"
)

func TestValidatePayload(t *testing.T) {
	schema := types.GetDefaultABISchema()

	tests := []struct {
		name    string
		payload map[string]interface{}
		wantErr bool
	}{
		{
			name: "规范示例 - 所有保留字段",
			// 遵循 IDENTIFIER_REPRESENTATION_GUIDE.md 规范：
			// - from/to: 账户地址，使用 Base58Check 格式（不带 0x）
			// - token_id: 代币标识，使用纯 hex 字符串（不带 0x）
			payload: map[string]interface{}{
				"from":     "CJ89RzBaa2SrLRUbGFY2SFfsu6UMAgqfNZ", // Base58Check 地址
				"to":       "CY8JpYU6CLAwg3M9yuQM8v1aCJWnSjVEwW", // Base58Check 地址
				"amount":   "1000000",
				"token_id": "0000000000000000000000000000000000000000000000000000000000000000", // 纯 hex（不带 0x）
			},
			wantErr: false,
		},
		{
			name: "扩展字段不冲突",
			// 遵循 WES 地址规范，使用 Base58Check 格式
			payload: map[string]interface{}{
				"from":         "CJ89RzBaa2SrLRUbGFY2SFfsu6UMAgqfNZ", // Base58Check 地址
				"custom_field": "value",
			},
			wantErr: false,
		},
		{
			name: "无效 JSON",
			payload: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payloadJSON string
			if tt.payload != nil {
				jsonBytes, err := json.Marshal(tt.payload)
				if err != nil {
					if !tt.wantErr {
						t.Fatalf("failed to marshal payload: %v", err)
					}
					return
				}
				payloadJSON = string(jsonBytes)
			} else {
				payloadJSON = "invalid json"
			}

			err := ValidatePayload(payloadJSON, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePayload() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDraftJSON(t *testing.T) {
	schema := types.GetDefaultABISchema()

	tests := []struct {
		name      string
		draftJSON string
		wantErr   bool
	}{
		{
			name: "正确的 State Output",
			draftJSON: `{
				"sign_mode": "defer_sign",
				"outputs": [{
					"type": "state",
					"state_id": "base64...",
					"state_version": 1,
					"execution_result_hash": "base64..."
				}]
			}`,
			wantErr: false,
		},
		{
			name: "正确的 Asset Output",
			// 遵循 WES 地址规范：owner 使用 Base58Check，token_id 使用纯 hex
			draftJSON: `{
				"sign_mode": "defer_sign",
				"outputs": [{
					"type": "asset",
					"owner": "CJ89RzBaa2SrLRUbGFY2SFfsu6UMAgqfNZ",
					"amount": "1000",
					"token_id": "0000000000000000000000000000000000000000000000000000000000000000"
				}]
			}`,
			wantErr: false,
		},
		{
			name: "缺少 sign_mode",
			draftJSON: `{
				"outputs": []
			}`,
			wantErr: true,
		},
		{
			name: "无效 JSON",
			draftJSON: "invalid json",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDraftJSON(tt.draftJSON, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDraftJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvertContractABIToSchema(t *testing.T) {
	tests := []struct {
		name    string
		abi     *types.ContractABI
		wantErr bool
	}{
		{
			name: "有效的 ABI",
			abi: &types.ContractABI{
				Version:   "1.0",
				Functions: []types.ContractFunction{},
				Events:    []struct {
					Name   string           `json:"name"`
					Params []types.ABIParam `json:"params"`
				}{},
			},
			wantErr: false,
		},
		{
			name:    "nil ABI",
			abi:     nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := ConvertContractABIToSchema(tt.abi)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertContractABIToSchema() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && schema == nil {
				t.Error("ConvertContractABIToSchema() returned nil schema without error")
			}
		})
	}
}

