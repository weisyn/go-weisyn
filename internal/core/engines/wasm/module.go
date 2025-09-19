package wasm

import (
	"go.uber.org/fx"

	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// WASMModuleInput WASM模块的输入依赖
type WASMModuleInput struct {
	fx.In

	// 基础设施依赖
	Logger           log.Logger       `optional:"true"` // 日志记录器（可选）
	MetricsCollector MetricsCollector `optional:"true"` // 统一指标收集器（可选）
}

// WASMModuleOutput WASM模块的输出服务
type WASMModuleOutput struct {
	fx.Out

	// 执行引擎适配器（通过明确命名提供给execution模块）
	WASMEngineAdapter execution.EngineAdapter `name:"wasm_engine"`
}

// ProvideWASMAdapter 提供WASM引擎适配器
func ProvideWASMAdapter(input WASMModuleInput) WASMModuleOutput {
	// 使用完整的WASM适配器（自动创建所需的底层组件）
	adapter := NewAdapterWithDefaults(input.MetricsCollector, input.Logger)

	if input.Logger != nil {
		input.Logger.Info("WASM引擎适配器已创建（完整版本，包含VM、缓存、验证器、优化器、实例池）")
	}

	return WASMModuleOutput{
		WASMEngineAdapter: adapter,
	}
}

// Module WASM 引擎 fx 模块
//
// 提供WASM引擎适配器，通过明确命名("wasm_engine")供执行层使用
// 这样区块链执行层可以通过名称明确获取WASM引擎实例
func Module() fx.Option {
	return fx.Module("engine-wasm",
		fx.Provide(ProvideWASMAdapter),
	)
}
