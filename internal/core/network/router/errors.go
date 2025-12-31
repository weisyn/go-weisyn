// Package router provides error definitions for network routing operations.
package router

import "errors"

// errors.go
// 路由模块错误模型与错误码归纳（方法框架）：
// - 用哨兵错误支持上层分支

var (
	// ErrNoRouteAvailable 无可用路由错误
	ErrNoRouteAvailable = errors.New("no route available")
	// ErrDuplicateMessage 重复消息错误
	ErrDuplicateMessage = errors.New("duplicate message")
)
