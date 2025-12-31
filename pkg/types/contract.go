// Package types provides contract type definitions.
package types

import "time"

// 注意：以下合约相关类型已被移除（未使用）：
// - ContractEvent
// - FunctionSignature
// 如需使用已删除的类型，可从 git 历史中恢复

// ABIParam ABI参数描述（保留，因为 ContractFunction 需要使用）
type ABIParam struct {
	Name string `json:"name"` // 参数名
	Type string `json:"type"` // 参数类型（i32/i64/f32/f64/string/bytes/...）
}

// ContractFunction 合约函数签名
type ContractFunction struct {
	Name       string     `json:"name"`       // 函数名
	Params     []ABIParam `json:"params"`     // 入参
	Returns    []ABIParam `json:"returns"`    // 返回值
	Payable    bool       `json:"payable"`    // 是否可支付
	Deprecated bool       `json:"deprecated"` // 是否废弃
	Notes      string     `json:"notes"`      // 备注
}

// ContractABI 合约ABI
type ContractABI struct {
	Version   string             `json:"version"`   // ABI版本
	Functions []ContractFunction `json:"functions"` // 函数清单
	Events    []struct {
		Name   string     `json:"name"`   // 事件名
		Params []ABIParam `json:"params"` // 事件参数
	} `json:"events"`    // 事件清单
	UpdatedAt time.Time  `json:"updated_at"`
}
