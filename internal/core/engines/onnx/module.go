package onnx

// 本文件为 ONNX 执行引擎模块的 fx 装配骨架。
//
// 目标：
// 1) 提供 EngineAdapter 实例，并以 fx 分组方式导出供区块链执行层统一注册；
// 2) 仅依赖 pkg/interfaces/execution 的抽象，不依赖区块链实现；
// 3) 暂不包含任何具体执行逻辑。

import (
	"go.uber.org/fx"

	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ONNXModuleInput ONNX模块的输入依赖
type ONNXModuleInput struct {
	fx.In

	// 基础设施依赖
	Logger log.Logger `optional:"true"` // 日志记录器（可选）
}

// ONNXModuleOutput ONNX模块的输出服务
type ONNXModuleOutput struct {
	fx.Out

	// 执行引擎适配器（通过明确命名提供给execution模块）
	ONNXEngineAdapter execution.EngineAdapter `name:"onnx_engine"`
}

// ProvideONNXAdapter 提供ONNX引擎适配器
func ProvideONNXAdapter(input ONNXModuleInput) ONNXModuleOutput {
	// 创建ONNX适配器实例
	adapter := &Adapter{
		logger: input.Logger,
	}

	if input.Logger != nil {
		input.Logger.Info("ONNX引擎适配器已创建（占位实现）")
	}

	return ONNXModuleOutput{
		ONNXEngineAdapter: adapter,
	}
}

// Module ONNX 引擎 fx 模块
func Module() fx.Option {
	return fx.Module("engine-onnx",
		fx.Provide(ProvideONNXAdapter),
	)
}
