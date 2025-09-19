package app

import (
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/types"
)

// Option 应用程序选项函数类型
type Option func(*options)

// options 应用程序选项
// 实现config.AppOptions接口
type options struct {
	// 配置文件路径
	configFilePath string

	// 用户配置
	appConfig *types.AppConfig

	// CLI支持开关
	enableCLI bool

	// API支持开关 (默认启用)
	enableAPI bool
}

// 编译时校验options是否实现了config.AppOptions接口
var _ config.AppOptions = (*options)(nil)

// WithConfigFile 设置配置文件路径
func WithConfigFile(configPath string) Option {
	return func(o *options) {
		o.configFilePath = configPath
	}
}

// WithNode 设置节点网络配置选项
func WithNode(userNodeConfig *types.UserNodeConfig) Option {
	return func(o *options) {
		if o.appConfig == nil {
			o.appConfig = &types.AppConfig{}
		}
		o.appConfig.Node = userNodeConfig
	}
}

// WithCLI 启用CLI模块
func WithCLI() Option {
	return func(o *options) {
		o.enableCLI = true
	}
}

// WithAPI 启用API模块
func WithAPI() Option {
	return func(o *options) {
		o.enableAPI = true
	}
}

// WithoutAPI 禁用API模块
func WithoutAPI() Option {
	return func(o *options) {
		o.enableAPI = false
	}
}

// newOptions 创建选项
func newOptions(opts ...Option) *options {
	options := &options{
		// 创建默认的空AppConfig
		appConfig: &types.AppConfig{},
		// API默认启用，CLI默认禁用
		enableAPI: true,
		enableCLI: false,
	}

	// 应用自定义选项
	for _, opt := range opts {
		opt(options)
	}

	return options
}

// GetAppConfig 返回应用程序配置
// 实现config.AppOptions接口的新方法
func (o *options) GetAppConfig() *types.AppConfig {
	return o.appConfig
}
