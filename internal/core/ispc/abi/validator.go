// Package abi provides ABI validation and helper functions.
//
// 规范来源：docs/components/core/ispc/abi-and-payload.md
package abi

import (
	"encoding/json"
	"fmt"

	"github.com/weisyn/v1/pkg/types"
)

// stripJSONComments 移除 JSON 文本中的注释（支持 // 行注释 与 /* */ 块注释）。
//
// 用途：Draft JSON 在文档/示例中常包含 // TODO 形式的注释，但这不是严格 JSON。
// 为了兼容“JSONC”输入，这里在反序列化前做一次轻量级清洗。
func stripJSONComments(in string) string {
	b := []byte(in)
	out := make([]byte, 0, len(b))

	inString := false
	escape := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(b); i++ {
		c := b[i]

		// 行注释：跳过直到换行
		if inLineComment {
			if c == '\n' || c == '\r' {
				inLineComment = false
				out = append(out, c)
			}
			continue
		}

		// 块注释：跳过直到 */
		if inBlockComment {
			if c == '*' && i+1 < len(b) && b[i+1] == '/' {
				inBlockComment = false
				i++ // 跳过 '/'
			}
			continue
		}

		// 字符串内：只处理转义与引号
		if inString {
			out = append(out, c)
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		// 非字符串：检测注释起始
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(b) {
			if b[i+1] == '/' {
				inLineComment = true
				i++ // 跳过第二个 '/'
				continue
			}
			if b[i+1] == '*' {
				inBlockComment = true
				i++ // 跳过 '*'
				continue
			}
		}

		out = append(out, c)
	}

	return string(out)
}

// ValidatePayload 验证 Payload JSON 是否符合 WES ABI 规范
//
// 检查：
// 1. 保留字段类型是否正确
// 2. 扩展字段名是否与保留字段冲突
// 3. 字段值格式是否符合规范
func ValidatePayload(payloadJSON string, schema *types.ABISchema) error {
	if schema == nil {
		schema = types.GetDefaultABISchema()
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return fmt.Errorf("invalid JSON payload: %w", err)
	}

	// 检查每个字段
	for fieldName, fieldValue := range payload {
		// 检查是否为保留字段
		if schema.IsReservedField(fieldName) {
			// 验证保留字段类型
			if err := validateReservedField(fieldName, fieldValue, schema); err != nil {
				return fmt.Errorf("reserved field '%s' validation failed: %w", fieldName, err)
			}
		} else {
			// 扩展字段：检查是否与保留字段冲突
			if err := schema.ValidatePayloadField(fieldName); err != nil {
				return fmt.Errorf("extension field '%s' validation failed: %w", fieldName, err)
			}
		}
	}

	return nil
}

// validateReservedField 验证保留字段的值
func validateReservedField(fieldName string, value interface{}, schema *types.ABISchema) error {
	// 查找字段定义
	var fieldDef *types.ReservedField
	for i := range schema.ReservedFields {
		if schema.ReservedFields[i].Name == fieldName {
			fieldDef = &schema.ReservedFields[i]
			break
		}
	}

	if fieldDef == nil {
		return fmt.Errorf("unknown reserved field: %s", fieldName)
	}

	// 类型检查
	switch fieldDef.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field '%s' must be string, got %T", fieldName, value)
		}
	default:
		// 其他类型可以扩展
	}

	return nil
}

// ConvertContractABIToSchema 将 ContractABI 转换为中立 Schema 表示
//
// 用于代码生成或跨语言绑定
func ConvertContractABIToSchema(abi *types.ContractABI) (*types.ABISchema, error) {
	if abi == nil {
		return nil, fmt.Errorf("ABI cannot be nil")
	}

	schema := types.GetDefaultABISchema()
	schema.Version = abi.Version

	// 可以在这里添加 ContractABI -> Schema 的映射逻辑
	// 例如：提取函数签名、参数类型等

	return schema, nil
}

// ValidateDraftJSON 验证 Draft JSON 是否符合 WES ABI 规范
//
// 检查 Draft JSON 的字段名和结构是否符合规范
func ValidateDraftJSON(draftJSON string, schema *types.ABISchema) error {
	if schema == nil {
		schema = types.GetDefaultABISchema()
	}

	var draft map[string]interface{}
	// 兼容 JSONC（含注释）
	clean := stripJSONComments(draftJSON)
	if err := json.Unmarshal([]byte(clean), &draft); err != nil {
		return fmt.Errorf("invalid JSON draft: %w", err)
	}

	// 检查 sign_mode 字段
	if _, ok := draft[schema.DraftJSONFields.SignMode]; !ok {
		return fmt.Errorf("missing required field: %s", schema.DraftJSONFields.SignMode)
	}

	// 检查 outputs 字段
	if outputs, ok := draft["outputs"].([]interface{}); ok {
		for i, output := range outputs {
			if err := validateOutput(output, schema); err != nil {
				return fmt.Errorf("output[%d] validation failed: %w", i, err)
			}
		}
	}

	// 检查 intents 字段（可选）
	if intents, ok := draft["intents"].([]interface{}); ok {
		for i, intent := range intents {
			if err := validateIntent(intent, schema); err != nil {
				return fmt.Errorf("intent[%d] validation failed: %w", i, err)
			}
		}
	}

	return nil
}

// validateOutput 验证输出描述符
func validateOutput(output interface{}, schema *types.ABISchema) error {
	outputMap, ok := output.(map[string]interface{})
	if !ok {
		return fmt.Errorf("output must be an object")
	}

	outputType, ok := outputMap["type"].(string)
	if !ok {
		return fmt.Errorf("output must have 'type' field")
	}

	switch outputType {
	case "asset":
		return validateAssetOutput(outputMap, schema)
	case "resource":
		return validateResourceOutput(outputMap, schema)
	case "state":
		return validateStateOutput(outputMap, schema)
	default:
		return fmt.Errorf("unknown output type: %s", outputType)
	}
}

// validateAssetOutput 验证资产输出
func validateAssetOutput(output map[string]interface{}, schema *types.ABISchema) error {
	fields := schema.DraftJSONFields.OutputDescriptor.AssetOutput

	// 检查必需字段
	if _, ok := output[fields.Owner]; !ok {
		return fmt.Errorf("asset output missing 'owner' field")
	}
	if _, ok := output[fields.Amount]; !ok {
		return fmt.Errorf("asset output missing 'amount' field")
	}
	if _, ok := output[fields.TokenID]; !ok {
		return fmt.Errorf("asset output missing 'token_id' field")
	}

	return nil
}

// validateResourceOutput 验证资源输出
func validateResourceOutput(output map[string]interface{}, schema *types.ABISchema) error {
	fields := schema.DraftJSONFields.OutputDescriptor.ResourceOutput

	// 检查必需字段
	if _, ok := output[fields.ContentHash]; !ok {
		return fmt.Errorf("resource output missing 'content_hash' field")
	}

	return nil
}

// validateStateOutput 验证状态输出
func validateStateOutput(output map[string]interface{}, schema *types.ABISchema) error {
	fields := schema.DraftJSONFields.OutputDescriptor.StateOutput

	// 检查必需字段
	if _, ok := output[fields.StateID]; !ok {
		return fmt.Errorf("state output missing 'state_id' field")
	}
	if _, ok := output[fields.StateVersion]; !ok {
		return fmt.Errorf("state output missing 'state_version' field")
	}
	if _, ok := output[fields.ExecutionResultHash]; !ok {
		return fmt.Errorf("state output missing 'execution_result_hash' field")
	}

	return nil
}

// validateIntent 验证意图描述符
func validateIntent(intent interface{}, schema *types.ABISchema) error {
	intentMap, ok := intent.(map[string]interface{})
	if !ok {
		return fmt.Errorf("intent must be an object")
	}

	intentType, ok := intentMap["type"].(string)
	if !ok {
		return fmt.Errorf("intent must have 'type' field")
	}

	// 检查 params 字段
	params, ok := intentMap["params"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("intent must have 'params' field")
	}

	// 根据意图类型验证参数
	switch intentType {
	case "transfer":
		return validateTransferIntent(params, schema)
	case "stake":
		return validateStakeIntent(params, schema)
	default:
		return fmt.Errorf("unknown intent type: %s", intentType)
	}
}

// validateTransferIntent 验证转账意图
func validateTransferIntent(params map[string]interface{}, schema *types.ABISchema) error {
	requiredFields := []string{"from", "to", "token_id", "amount"}
	for _, field := range requiredFields {
		if _, ok := params[field]; !ok {
			return fmt.Errorf("transfer intent missing required field: %s", field)
		}
	}
	return nil
}

// validateStakeIntent 验证质押意图
func validateStakeIntent(params map[string]interface{}, schema *types.ABISchema) error {
	requiredFields := []string{"staker", "amount", "validator"}
	for _, field := range requiredFields {
		if _, ok := params[field]; !ok {
			return fmt.Errorf("stake intent missing required field: %s", field)
		}
	}
	return nil
}
