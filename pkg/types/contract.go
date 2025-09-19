package types

import "time"

// 合约元数据（链上资源的运行时信息聚合）
// 说明：用于区块链执行层与引擎层之间的合约能力发现与验证。
type ContractMetadata struct {
	// 基础标识
	Address string     `json:"address"` // 合约地址（统一字符串表示）
	Engine  EngineType `json:"engine"`  // 执行引擎类型（wasm/onnx/...）
	Version string     `json:"version"` // 合约版本

	// 代码与结构
	CodeHash []byte   `json:"code_hash"` // 代码哈希（资源唯一性校验）
	CodeSize uint64   `json:"code_size"` // 代码大小（字节）
	Exports  []string `json:"exports"`   // 导出函数清单
	Imports  []string `json:"imports"`   // 导入模块清单

	// 生命周期
	CreatedAt int64 `json:"created_at"` // 创建时间（秒）
	UpdatedAt int64 `json:"updated_at"` // 更新时间（秒）

	// 扩展元数据
	Extra map[string]any `json:"extra"`
}

// ABIParam ABI参数描述
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

// ContractEvent 合约事件签名
type ContractEvent struct {
	Name   string     `json:"name"`   // 事件名
	Params []ABIParam `json:"params"` // 事件参数
}

// ContractABI 合约ABI
type ContractABI struct {
	Version   string             `json:"version"`   // ABI版本
	Functions []ContractFunction `json:"functions"` // 函数清单
	Events    []ContractEvent    `json:"events"`    // 事件清单
	UpdatedAt time.Time          `json:"updated_at"`
}

// FunctionSignature 简化函数签名（便于快速匹配与调度）
type FunctionSignature struct {
	Name        string   `json:"name"`         // 函数名
	ParamTypes  []string `json:"param_types"`  // 参数类型列表
	ReturnTypes []string `json:"return_types"` // 返回类型列表
}
