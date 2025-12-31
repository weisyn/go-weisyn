// Package main provides ABI conformance testing tool.
//
// 跨仓库 ABI 一致性测试工具
// 规范来源：docs/components/core/ispc/abi-and-payload.md
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/weisyn/v1/internal/core/ispc/abi"
	"github.com/weisyn/v1/pkg/types"
)

func main() {
	fmt.Println("🔍 WES ABI Conformance Checker")
	fmt.Println("规范来源：docs/components/core/ispc/abi-and-payload.md")
	fmt.Println()

	schema := types.GetDefaultABISchema()

	// 检查项
	checks := []struct {
		name string
		fn   func(*types.ABISchema) error
	}{
		{"Payload 字段名检查", checkPayloadFieldNames},
		{"Draft JSON 字段名检查", checkDraftJSONFieldNames},
		{"保留字段冲突检查", checkReservedFieldConflicts},
	}

	allPassed := true
	for _, check := range checks {
		fmt.Printf("检查：%s\n", check.name)
		if err := check.fn(schema); err != nil {
			fmt.Printf("  ❌ 失败：%v\n", err)
			allPassed = false
		} else {
			fmt.Printf("  ✅ 通过\n")
		}
	}

	// 可选：扫描 SDK fixtures（如果存在）
	if len(os.Args) > 1 && os.Args[1] == "--scan-fixtures" {
		fmt.Println("\n扫描 SDK fixtures...")
		if err := scanSDKFixtures(); err != nil {
			fmt.Printf("  ⚠️  扫描完成（部分路径可能不存在）: %v\n", err)
		}
	}

	fmt.Println()
	if allPassed {
		fmt.Println("✅ 所有检查通过")
		os.Exit(0)
	} else {
		fmt.Println("❌ 部分检查失败")
		os.Exit(1)
	}
}

// checkPayloadFieldNames 检查 Payload 字段名是否符合规范
func checkPayloadFieldNames(schema *types.ABISchema) error {
	// 测试用例：规范示例
	testCases := []struct {
		name    string
		payload map[string]interface{}
		wantErr bool
	}{
		{
			name: "规范示例 - 所有保留字段",
			// 遵循 WES 地址规范：地址使用 Base58Check，哈希使用纯 hex
			payload: map[string]interface{}{
				"from":     "CJ89RzBaa2SrLRUbGFY2SFfsu6UMAgqfNZ", // Base58Check 地址
				"to":       "CY8JpYU6CLAwg3M9yuQM8v1aCJWnSjVEwW", // Base58Check 地址
				"amount":   "1000000",
				"token_id": "0000000000000000000000000000000000000000000000000000000000000000", // 纯 hex
			},
			wantErr: false,
		},
		{
			name: "扩展字段使用 tokenID（驼峰）- 允许但不推荐",
			payload: map[string]interface{}{
				"tokenID": "0000000000000000000000000000000000000000000000000000000000000000", // 扩展字段可以使用任意名称
			},
			wantErr: false, // 扩展字段名不与保留字段冲突，所以不报错
		},
	}

	for _, tc := range testCases {
		payloadJSON, err := json.Marshal(tc.payload)
		if err != nil {
			return fmt.Errorf("测试用例 '%s' JSON 序列化失败: %v", tc.name, err)
		}

		err = abi.ValidatePayload(string(payloadJSON), schema)

		if tc.wantErr && err == nil {
			return fmt.Errorf("测试用例 '%s' 应该失败但没有失败", tc.name)
		}
		if !tc.wantErr && err != nil {
			return fmt.Errorf("测试用例 '%s' 不应该失败但失败了: %v", tc.name, err)
		}
	}

	return nil
}

// checkDraftJSONFieldNames 检查 Draft JSON 字段名是否符合规范
func checkDraftJSONFieldNames(schema *types.ABISchema) error {
	// 测试用例：State Output 字段名
	testCases := []struct {
		name        string
		draftJSON   string
		wantErr     bool
		description string
	}{
		{
			name: "正确的 State Output 字段名",
			draftJSON: `{
				"sign_mode": "defer_sign",
				"outputs": [{
					"type": "state",
					"state_id": "base64...",
					"state_version": 1,
					"execution_result_hash": "base64..."
				}]
			}`,
			wantErr:     false,
			description: "使用 state_version 和 execution_result_hash",
		},
		{
			name: "错误：使用 version 和 exec_hash",
			draftJSON: `{
				"sign_mode": "defer_sign",
				"outputs": [{
					"type": "state",
					"state_id": "base64...",
					"version": 1,
					"exec_hash": "base64..."
				}]
			}`,
			wantErr:     true,
			description: "应该使用 state_version 和 execution_result_hash",
		},
	}

	for _, tc := range testCases {
		err := abi.ValidateDraftJSON(tc.draftJSON, schema)

		if tc.wantErr && err == nil {
			return fmt.Errorf("测试用例 '%s' 应该失败但没有失败: %s", tc.name, tc.description)
		}
		if !tc.wantErr && err != nil {
			// 对于错误用例，如果验证失败是预期的，则继续
			// 对于正确用例，如果验证失败则返回错误
			return fmt.Errorf("测试用例 '%s' 不应该失败但失败了: %v", tc.name, err)
		}
	}

	return nil
}

// checkReservedFieldConflicts 检查保留字段冲突
func checkReservedFieldConflicts(schema *types.ABISchema) error {
	// 测试用例：扩展字段与保留字段冲突
	testCases := []struct {
		name    string
		payload map[string]interface{}
		wantErr bool
	}{
		{
			name: "扩展字段与保留字段冲突",
			payload: map[string]interface{}{
				"from":        "0x1234...", // 保留字段
				"custom_from": "value",     // 扩展字段（不冲突）
			},
			wantErr: false,
		},
		{
			name: "方法参数与保留字段冲突",
			payload: map[string]interface{}{
				"from": "0x1234...", // 保留字段
			},
			wantErr: false, // 保留字段本身不冲突
		},
	}

	for _, tc := range testCases {
		payloadJSON, err := json.Marshal(tc.payload)
		if err != nil {
			return fmt.Errorf("测试用例 '%s' JSON 序列化失败: %v", tc.name, err)
		}

		err = abi.ValidatePayload(string(payloadJSON), schema)

		if tc.wantErr && err == nil {
			return fmt.Errorf("测试用例 '%s' 应该失败但没有失败", tc.name)
		}
		if !tc.wantErr && err != nil {
			return fmt.Errorf("测试用例 '%s' 不应该失败但失败了: %v", tc.name, err)
		}
	}

	return nil
}

// scanSDKFixtures 扫描 SDK 测试用例目录
func scanSDKFixtures() error {
	sdkPaths := []string{
		"../../sdk/client-sdk-go.git/tests/fixtures",
		"../../sdk/client-sdk-js.git/tests/fixtures",
		"../../sdk/contract-sdk-go.git/tests/fixtures",
		"../../sdk/contract-sdk-js.git/tests/fixtures",
	}

	foundCount := 0
	for _, path := range sdkPaths {
		if err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(p, ".json") {
				fmt.Printf("  发现测试用例：%s\n", p)
				foundCount++
			}
			return nil
		}); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	if foundCount == 0 {
		fmt.Println("  未发现测试用例（路径可能不存在）")
	} else {
		fmt.Printf("  共发现 %d 个测试用例文件\n", foundCount)
	}

	return nil
}

// validateBase64Encoding 验证 Base64 编码
func validateBase64Encoding(encoded string, expectedJSON string) error {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("Base64 解码失败: %w", err)
	}

	if string(decoded) != expectedJSON {
		return fmt.Errorf("Base64 解码结果不匹配")
	}

	return nil
}
