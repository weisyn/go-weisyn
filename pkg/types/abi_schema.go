// Package types provides ABI schema definitions for code generation and validation.
package types

import "fmt"

// ABISchema ABI Schema 定义
// 
// 用于代码生成和校验的结构化描述，与 ContractABI 映射
// 规范来源：docs/components/core/ispc/abi-and-payload.md
type ABISchema struct {
	// Version Schema 版本
	Version string `json:"version"`
	
	// ReservedFields Payload 保留字段列表
	ReservedFields []ReservedField `json:"reserved_fields"`
	
	// DraftJSONFields Draft JSON 字段定义
	DraftJSONFields DraftJSONSchema `json:"draft_json_fields"`
	
	// ParameterTypes 支持的参数类型
	ParameterTypes []ParameterType `json:"parameter_types"`
}

// ReservedField Payload 保留字段定义
type ReservedField struct {
	Name        string `json:"name"`         // 字段名（如 "from", "to", "amount", "token_id"）
	Type        string `json:"type"`         // 字段类型（如 "string"）
	Description string `json:"description"`  // 字段说明
	Required    bool   `json:"required"`     // 是否必需
}

// DraftJSONSchema Draft JSON 结构定义
type DraftJSONSchema struct {
	// SignMode 签名模式字段
	SignMode string `json:"sign_mode"`
	
	// InputDescriptor 输入描述符字段
	InputDescriptor InputDescriptorSchema `json:"input_descriptor"`
	
	// OutputDescriptor 输出描述符字段
	OutputDescriptor OutputDescriptorSchema `json:"output_descriptor"`
	
	// IntentDescriptor 意图描述符字段
	IntentDescriptor IntentDescriptorSchema `json:"intent_descriptor"`
}

// InputDescriptorSchema 输入描述符 Schema
type InputDescriptorSchema struct {
	Outpoint         OutpointSchema `json:"outpoint"`
	IsReferenceOnly string          `json:"is_reference_only"`
	UnlockingProof  string          `json:"unlocking_proof"`
}

// OutpointSchema Outpoint Schema
type OutpointSchema struct {
	TxHash string `json:"tx_hash"`
	Index  string `json:"index"`
}

// OutputDescriptorSchema 输出描述符 Schema
type OutputDescriptorSchema struct {
	// AssetOutput 资产输出字段
	AssetOutput AssetOutputSchema `json:"asset_output"`
	
	// ResourceOutput 资源输出字段
	ResourceOutput ResourceOutputSchema `json:"resource_output"`
	
	// StateOutput 状态输出字段
	StateOutput StateOutputSchema `json:"state_output"`
}

// AssetOutputSchema 资产输出 Schema
type AssetOutputSchema struct {
	Type     string `json:"type"`      // "asset"
	Owner    string `json:"owner"`     // 所有者地址（十六进制）
	Amount   string `json:"amount"`    // 金额（大整数字符串）
	TokenID  string `json:"token_id"`  // 代币ID（十六进制）
	Metadata string `json:"metadata"`  // 元数据（JSON 对象）
}

// ResourceOutputSchema 资源输出 Schema
type ResourceOutputSchema struct {
	Type        string `json:"type"`         // "resource"
	ContentHash string `json:"content_hash"` // 内容哈希（十六进制）
	Category    string `json:"category"`     // 资源类别（如 "wasm_contract"）
	Owner       string `json:"owner"`        // 所有者地址（十六进制）
	Metadata    string `json:"metadata"`     // 元数据（JSON 对象）
}

// StateOutputSchema 状态输出 Schema
type StateOutputSchema struct {
	Type                 string `json:"type"`                   // "state"
	StateID              string `json:"state_id"`                // 状态ID（Base64）
	StateVersion         string `json:"state_version"`           // 状态版本号（数字）
	ExecutionResultHash   string `json:"execution_result_hash"`  // 执行结果哈希（Base64）
	PublicInputs         string `json:"public_inputs"`           // 公开输入（Base64，可选）
	ParentStateHash      string `json:"parent_state_hash"`       // 父状态哈希（Base64，可选）
}

// IntentDescriptorSchema 意图描述符 Schema
type IntentDescriptorSchema struct {
	Type   string            `json:"type"`   // 意图类型（如 "transfer", "stake"）
	Params map[string]string `json:"params"` // 参数映射（字段名 -> 类型）
}

// ParameterType 参数类型定义
type ParameterType struct {
	Name        string `json:"name"`        // 类型名（如 "i32", "i64", "string", "bytes"）
	WASMMapping string `json:"wasm_mapping"` // WASM 映射（如 "i32", "(i32, i32)"）
	JSONType    string `json:"json_type"`   // JSON 类型（如 "number", "string"）
	Description string `json:"description"` // 类型说明
}

// GetDefaultABISchema 获取默认 ABI Schema
// 
// 基于 abi-and-payload.md 规范定义的默认 Schema
func GetDefaultABISchema() *ABISchema {
	return &ABISchema{
		Version: "1.0",
		ReservedFields: []ReservedField{
			{
				Name:        "from",
				Type:        "string",
				Description: "调用者地址（20字节十六进制）",
				Required:    false,
			},
			{
				Name:        "to",
				Type:        "string",
				Description: "接收者地址（20字节十六进制）",
				Required:    false,
			},
			{
				Name:        "amount",
				Type:        "string",
				Description: "转账金额（大整数字符串）",
				Required:    false,
			},
			{
				Name:        "token_id",
				Type:        "string",
				Description: "代币ID（32字节十六进制）",
				Required:    false,
			},
		},
		DraftJSONFields: DraftJSONSchema{
			SignMode: "sign_mode",
			InputDescriptor: InputDescriptorSchema{
				Outpoint: OutpointSchema{
					TxHash: "tx_hash",
					Index:  "index",
				},
				IsReferenceOnly: "is_reference_only",
				UnlockingProof:  "unlocking_proof",
			},
			OutputDescriptor: OutputDescriptorSchema{
				AssetOutput: AssetOutputSchema{
					Type:     "type",
					Owner:    "owner",
					Amount:   "amount",
					TokenID:  "token_id",
					Metadata: "metadata",
				},
				ResourceOutput: ResourceOutputSchema{
					Type:        "type",
					ContentHash: "content_hash",
					Category:    "category",
					Owner:       "owner",
					Metadata:    "metadata",
				},
				StateOutput: StateOutputSchema{
					Type:                "type",
					StateID:             "state_id",
					StateVersion:        "state_version",
					ExecutionResultHash: "execution_result_hash",
					PublicInputs:        "public_inputs",
					ParentStateHash:     "parent_state_hash",
				},
			},
			IntentDescriptor: IntentDescriptorSchema{
				Type:   "type",
				Params: map[string]string{
					"from":      "string",
					"to":        "string",
					"token_id":  "string",
					"amount":    "string",
				},
			},
		},
		ParameterTypes: []ParameterType{
			{Name: "i32", WASMMapping: "i32", JSONType: "number", Description: "32位有符号整数"},
			{Name: "i64", WASMMapping: "i64", JSONType: "number", Description: "64位有符号整数"},
			{Name: "u32", WASMMapping: "i32", JSONType: "number", Description: "32位无符号整数"},
			{Name: "u64", WASMMapping: "i64", JSONType: "number", Description: "64位无符号整数"},
			{Name: "f32", WASMMapping: "f32", JSONType: "number", Description: "32位浮点数"},
			{Name: "f64", WASMMapping: "f64", JSONType: "number", Description: "64位浮点数"},
			{Name: "string", WASMMapping: "(i32, i32)", JSONType: "string", Description: "UTF-8 字符串"},
			{Name: "bytes", WASMMapping: "(i32, i32)", JSONType: "string", Description: "字节数组（Base64 编码）"},
		},
	}
}

// IsReservedField 检查字段名是否为保留字段
func (s *ABISchema) IsReservedField(fieldName string) bool {
	for _, rf := range s.ReservedFields {
		if rf.Name == fieldName {
			return true
		}
	}
	return false
}

// ValidatePayloadField 验证 Payload 字段名是否合法
// 
// 扩展字段名不得与保留字段冲突
func (s *ABISchema) ValidatePayloadField(fieldName string) error {
	if s.IsReservedField(fieldName) {
		return &FieldConflictError{
			FieldName: fieldName,
			Reason:    "conflicts with reserved field",
		}
	}
	return nil
}

// FieldConflictError 字段冲突错误
type FieldConflictError struct {
	FieldName string
	Reason    string
}

func (e *FieldConflictError) Error() string {
	return fmt.Sprintf("field '%s' conflicts with reserved field: %s", e.FieldName, e.Reason)
}

