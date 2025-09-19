package router

import "errors"

// errors.go
// 路由模块错误模型与错误码归纳（方法框架）：
// - 用哨兵错误支持上层分支

var (
	ErrNoRouteAvailable = errors.New("no route available")
	ErrDuplicateMessage = errors.New("duplicate message")
)
