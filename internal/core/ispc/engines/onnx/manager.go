//go:build !android && !ios && cgo
// +build !android,!ios,cgo

package onnx

import (
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/ures"
)

// Manager ONNX引擎管理器
//
// 🎯 **设计理念**：薄实现，严格遵循WES三层架构
// 📋 **架构原则**：Manager只负责依赖注入和接口方法实现，不包含复杂业务逻辑
//
// 实现pkg/interfaces/ispcInterfaces.ONNXEngine接口
// 所有复杂业务逻辑委托给Engine处理
type Manager struct {
	logger log.Logger     // 日志服务
	engine *Engine        // 核心推理引擎
	casStorage ures.CASStorage // 内容寻址存储（用于加载模型文件）
}

// NewManager 创建ONNX引擎管理器
//
// 🎯 **依赖注入构造器**：接收必要的依赖服务
// 📋 **薄实现原则**：只做依赖管理，不实现具体业务逻辑
func NewManager(logger log.Logger, casStorage ures.CASStorage) (*Manager, error) {
	// 创建核心引擎
	engine, err := NewEngine(logger, casStorage)
	if err != nil {
		return nil, err
	}

	return &Manager{
		logger:     logger,
		engine:     engine,
		casStorage: casStorage,
	}, nil
}

// CallModel 方法已移除，请直接使用engine.CallModel
// 此Manager已废弃，所有功能已迁移到Engine

// Shutdown 关闭引擎
func (m *Manager) Shutdown() error {
	if m.engine != nil {
		return m.engine.Shutdown()
	}
	return nil
}
