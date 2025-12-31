//go:build android || ios || !cgo
// +build android ios !cgo

package onnx

import (
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/ures"
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
)

// Engine ONNX推理引擎的stub实现（用于不支持的平台）
type Engine struct{}

// NewEngine 创建ONNX推理引擎（stub实现，返回nil）
// 在不支持的平台上，此函数返回nil，表示ONNX引擎不可用
func NewEngine(logger log.Logger, casStorage ures.CASStorage) (ispcInterfaces.InternalONNXEngine, error) {
	if logger != nil {
		logger.Warn("⚠️ 当前平台不支持 ONNX Runtime，ONNX 引擎不可用")
	}
	return nil, nil
}

