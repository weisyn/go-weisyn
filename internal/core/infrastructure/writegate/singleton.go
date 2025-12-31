package writegate

import (
	wgif "github.com/weisyn/v1/pkg/interfaces/infrastructure/writegate"
)

// 在 init 中注册默认实例到接口层
func init() {
	wgif.SetDefault(&gateImpl{})
}

