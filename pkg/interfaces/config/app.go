// Package config provides application configuration interfaces.
package config

import "github.com/weisyn/v1/pkg/types"

// AppOptions 应用配置选项接口
// 提供获取应用配置的统一接口
type AppOptions interface {
	// GetAppConfig 获取应用配置
	GetAppConfig() *types.AppConfig
}
