// Package pubsub provides error definitions for publish-subscribe messaging operations.
package pubsub

import "errors"

// errors.go
// 发布-订阅错误模型与错误码归纳（方法框架）：
// - 用哨兵错误支持上层分类

var (
	// ErrNotSubscribed 未订阅错误
	ErrNotSubscribed = errors.New("not subscribed")
	// ErrValidateFail 验证失败错误
	ErrValidateFail = errors.New("validate failed")
	// ErrEncodeFail 编码失败错误
	ErrEncodeFail = errors.New("encode failed")
)
