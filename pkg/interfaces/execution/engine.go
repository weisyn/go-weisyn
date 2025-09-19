package execution

import (
	// 仅引用类型，不引入实现
	types "github.com/weisyn/v1/pkg/types"
)

// 执行引擎适配器抽象（如 WASM、ONNX）
//
// 设计目标：
// 1) 为区块链计算层提供统一的执行能力抽象；
// 2) 由各具体执行环境（WASM/ONNX）实现本接口；
// 3) 不依赖具体区块链实现，避免循环依赖；
// 4) 便于通过 fx 进行分组注册与统一装配。
//
// 注意：本文件仅定义接口与类型，不包含任何实现。

type EngineAdapter interface {
	// GetEngineType 返回引擎类型（如 wasm、onnx），用于管理与分发
	GetEngineType() types.EngineType

	// Initialize 初始化引擎所需的内部资源（如运行时、缓存等）
	// 仅限引擎内部依赖，不应耦合区块链实现
	Initialize(config map[string]any) error

	// BindHost 绑定宿主函数接口，由区块链计算层提供标准宿主接口的绑定
	BindHost(binding HostBinding) error

	// Execute 执行资源（合约/模型等），返回统一的执行结果
	Execute(params types.ExecutionParams) (*types.ExecutionResult, error)

	// Close 关闭引擎并释放资源
	Close() error
}
