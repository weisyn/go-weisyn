package stream

import (
	"time"

	iface "github.com/weisyn/v1/pkg/interfaces/network"
)

// options.go
// 传输选项解析与默认值处理

// DefaultTransportOptions 默认传输选项
var DefaultTransportOptions = &iface.TransportOptions{
	ConnectTimeout: 10 * time.Second,
	WriteTimeout:   5 * time.Second,
	ReadTimeout:    10 * time.Second,
	MaxRetries:     3,
	RetryDelay:     time.Second,
	BackoffFactor:  2.0,
}

// ResolveTransportOptions 解析最终传输选项（填充默认值）
func ResolveTransportOptions(in *iface.TransportOptions) *iface.TransportOptions {
	if in == nil {
		return DefaultTransportOptions
	}
	result := *DefaultTransportOptions // 复制默认值
	// 覆盖非零值
	if in.ConnectTimeout > 0 {
		result.ConnectTimeout = in.ConnectTimeout
	}
	if in.WriteTimeout > 0 {
		result.WriteTimeout = in.WriteTimeout
	}
	if in.ReadTimeout > 0 {
		result.ReadTimeout = in.ReadTimeout
	}
	if in.MaxRetries >= 0 {
		result.MaxRetries = in.MaxRetries
	}
	if in.RetryDelay > 0 {
		result.RetryDelay = in.RetryDelay
	}
	if in.BackoffFactor > 0 {
		result.BackoffFactor = in.BackoffFactor
	}
	return &result
}
