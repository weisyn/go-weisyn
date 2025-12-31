// Package types provides WASM type definitions.
package types

import (
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// WASM引擎相关类型定义
//
// 🎯 **WASM类型系统支持**
//
// 为WASM引擎提供标准的数据结构定义，
// 支持合约加载、编译、实例化和执行的完整生命周期。

// WASMContract WASM合约结构
//
// 表示从资源存储加载的WASM合约的完整信息
type WASMContract struct {
	// Address 合约地址
	Address string `json:"address"`

	// Hash 合约内容哈希（32字节）
	Hash []byte `json:"hash"`

	// Bytecode WASM字节码
	Bytecode []byte `json:"bytecode"`

	// Metadata 合约元数据
	Metadata map[string]string `json:"metadata"`

	// Size 字节码大小
	Size int64 `json:"size"`
}

// CompiledContract 已编译的WASM合约
//
// 表示经过wazero编译后的WASM模块，可用于实例化
//
// 设计说明：
// - 移除了 ExportedFunctions/ImportedFunctions 字段，因为这些信息：
//  1. wazero.CompiledModule 不直接暴露函数定义
//  2. 标准用法是实例化后通过 api.Module.ExportedFunction(name) 按需查询
//  3. 避免过度设计和错误的工程假设
type CompiledContract struct {
	// Hash 合约内容哈希
	Hash []byte `json:"hash"`

	// Module wazero编译后的模块（运行时特定，interface{}类型）
	Module interface{} `json:"-"`

	// CompiledAt 编译时间戳
	CompiledAt int64 `json:"compiled_at"`
}

// WASMInstance WASM合约实例
//
// 表示基于已编译模块创建的可执行实例
type WASMInstance struct {
	// ID 实例唯一标识符
	ID string `json:"id"`

	// Hash 合约内容哈希
	Hash []byte `json:"hash"`

	// Instance wazero运行时实例（运行时特定，interface{}类型）
	Instance interface{} `json:"-"`

	// Memory WASM线性内存引用（运行时特定，interface{}类型）
	Memory interface{} `json:"-"`

	// CreatedAt 实例创建时间
	CreatedAt int64 `json:"created_at"`

	// Status 实例状态
	Status WASMInstanceStatus `json:"status"`
}

// WASMInstanceStatus WASM实例状态
type WASMInstanceStatus string

const (
	WASMInstanceStatusCreated   WASMInstanceStatus = "created"   // 已创建
	WASMInstanceStatusRunning   WASMInstanceStatus = "running"   // 运行中
	WASMInstanceStatusFinished  WASMInstanceStatus = "finished"  // 已完成
	WASMInstanceStatusFailed    WASMInstanceStatus = "failed"    // 执行失败
	WASMInstanceStatusDestroyed WASMInstanceStatus = "destroyed" // 已销毁
)

// WASMExecutionResult WASM函数执行结果
//
// 标准化的WASM函数调用结果结构
type WASMExecutionResult struct {
	// Results 函数返回值（wazero原生uint64格式）
	Results []uint64 `json:"results"`

	// GasUsed 消耗的Gas（可选）
	GasUsed uint64 `json:"gas_used,omitempty"`

	// Duration 执行时长（毫秒）
	Duration int64 `json:"duration"`

	// Error 执行错误信息
	Error string `json:"error,omitempty"`
}

// ==================== Host ABI DTO 类型 ====================
//
// 说明：这些 DTO 属于 Host ABI/SDK 之间的编解码载体，
// 与共识层交易结构分离，避免污染 pb.blockchain.core.* 协议。

// BatchAssetOutputItemDTO - 批量资产输出单项（Host ABI 专用）
type BatchAssetOutputItemDTO struct {
	Recipient []byte `json:"recipient"`
	Amount    uint64 `json:"amount"`
	TokenID   []byte `json:"token_id"`
	// locking_conditions: protojson 的 LockingCondition 数组（原样传递给主机侧解码）
	LockingConditions [][]byte `json:"locking_conditions"`
}

// BatchAssetOutputsDTO - 批量资产输出集合（Host ABI 专用）
type BatchAssetOutputsDTO struct {
	Items []BatchAssetOutputItemDTO `json:"items"`
}

// LockingConditionListDTO - 锁定条件数组容器（Host ABI 专用）
// 用于 WASM 边界传递，与共识层 proto 隔离
type LockingConditionListDTO struct {
	Conditions []*pb.LockingCondition `json:"conditions"`
}
