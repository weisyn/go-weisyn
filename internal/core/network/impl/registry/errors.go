package registry

import "errors"

// errors.go
// 协议注册相关错误模型与错误码归纳（方法框架）：
// - 采用哨兵错误作为对外分类的基础（不暴露内部细节）
// - 具体错误细节在实现中通过包装 errors.Join 或 fmt.Errorf 扩展

var (
	// ErrAlreadyRegistered 当对同一协议重复注册时返回
	ErrAlreadyRegistered = errors.New("protocol already registered")

	// ErrNotFound 当查询/注销不存在的协议时返回
	ErrNotFound = errors.New("protocol not found")

	// ErrInvalidProtocolID 当协议ID格式非法时返回
	ErrInvalidProtocolID = errors.New("invalid protocol id")

	// ErrIncompatibleVersion 当版本不兼容时返回
	ErrIncompatibleVersion = errors.New("incompatible protocol version")
)
