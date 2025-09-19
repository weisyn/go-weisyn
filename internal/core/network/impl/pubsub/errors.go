package pubsub

import "errors"

// errors.go
// 发布-订阅错误模型与错误码归纳（方法框架）：
// - 用哨兵错误支持上层分类

var (
	ErrNotSubscribed = errors.New("not subscribed")
	ErrValidateFail  = errors.New("validate failed")
	ErrEncodeFail    = errors.New("encode failed")
)
