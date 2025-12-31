//go:build !android && !ios && cgo
// +build !android,!ios,cgo

// Package onnx provides error definitions for the ONNX inference engine.
package onnx

import (
	"errors"
	"fmt"
)

// 预定义错误
var (
	// ErrModelNotFound ONNX模型未找到
	ErrModelNotFound = errors.New("ONNX模型未找到")
	// ErrInvalidModelFormat 无效的ONNX模型格式
	ErrInvalidModelFormat = errors.New("无效的ONNX模型格式")
	// ErrInvalidInput 无效的输入张量
	ErrInvalidInput = errors.New("无效的输入张量")
	// ErrInferenceFailed ONNX推理失败
	ErrInferenceFailed = errors.New("ONNX推理失败")
	// ErrSessionCreation ONNX会话创建失败
	ErrSessionCreation = errors.New("ONNX会话创建失败")
	// ErrTensorConversion 张量格式转换失败
	ErrTensorConversion = errors.New("张量格式转换失败")
	// ErrInvalidModelAddress 无效的模型地址
	ErrInvalidModelAddress = errors.New("无效的模型地址")
)

// ONNXError ONNX专用错误
type ONNXError struct {
	Op      string // 操作名称
	Model   string // 模型地址
	Err     error  // 原始错误
}

// Error 实现error接口
func (e *ONNXError) Error() string {
	return fmt.Sprintf("ONNX错误[%s][模型:%s]: %v", e.Op, e.Model, e.Err)
}

// Unwrap 支持errors.Is/As
func (e *ONNXError) Unwrap() error {
	return e.Err
}

// WrapError 包装错误
func WrapError(op, model string, err error) error {
	if err == nil {
		return nil
	}

	return &ONNXError{
		Op:    op,
		Model: model,
		Err:   err,
	}
}

