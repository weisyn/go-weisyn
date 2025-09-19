package onnx

import (
	"fmt"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	types "github.com/weisyn/v1/pkg/types"
)

// ONNX 引擎适配器（占位骨架）
// 说明：
// - 实现 pkg/interfaces/execution.EngineAdapter 的方法签名；
// - 仅占位，不包含任何执行逻辑；
// - 依赖注入由 module.go 负责（后续补齐）。

type Adapter struct {
	logger log.Logger
}

func (a *Adapter) GetEngineType() types.EngineType { return types.EngineTypeONNX }

func (a *Adapter) Initialize(config map[string]any) error {
	if a.logger != nil {
		a.logger.Info("ONNX引擎初始化（占位实现）")
	}
	return nil
}

func (a *Adapter) BindHost(binding execiface.HostBinding) error {
	if a.logger != nil {
		a.logger.Info("ONNX引擎宿主绑定（占位实现）")
	}
	return nil
}

func (a *Adapter) Execute(params types.ExecutionParams) (*types.ExecutionResult, error) {
	if a.logger != nil {
		a.logger.Warn("ONNX引擎执行请求被拒绝：尚未实现")
	}
	return nil, fmt.Errorf("ONNX引擎尚未实现 - ONNX engine not implemented yet")
}

func (a *Adapter) Close() error {
	if a.logger != nil {
		a.logger.Info("ONNX引擎关闭（占位实现）")
	}
	return nil
}
