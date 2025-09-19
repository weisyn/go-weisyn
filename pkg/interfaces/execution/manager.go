package execution

import (
	// 仅引用类型，不引入实现
	types "github.com/weisyn/v1/pkg/types"
)

// 执行引擎管理器抽象（由区块链计算层实现）
//
// 设计目标：
// 1) 统一管理多个引擎（WASM/ONNX/…）的注册、查询与分发；
// 2) 对外提供统一执行入口，内部路由到具体引擎；
// 3) 不依赖引擎实现细节，仅面向 EngineAdapter 抽象；
// 4) 通过 fx 分组注册各引擎适配器后统一装配。

type EngineManager interface {
	// RegisterEngine 注册一个执行引擎适配器
	RegisterEngine(adapter EngineAdapter) error

	// GetEngine 根据类型获取引擎
	GetEngine(t types.EngineType) (EngineAdapter, bool)

	// ListEngines 列出所有已注册的引擎类型
	ListEngines() []types.EngineType

	// Execute 通过引擎类型统一分发执行
	Execute(t types.EngineType, params types.ExecutionParams) (*types.ExecutionResult, error)
}
