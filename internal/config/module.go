// Package config 提供应用配置管理功能
package config

import (
	blockchainconfig "github.com/weisyn/v1/internal/config/blockchain"
	"github.com/weisyn/v1/internal/config/compliance"
	"github.com/weisyn/v1/internal/config/consensus"
	"github.com/weisyn/v1/internal/config/repository"
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/types"
	"go.uber.org/fx"
)

// ConfigParams 定义配置模块的依赖参数
type ConfigParams struct {
	fx.In

	// 应用配置选项
	AppOptions config.AppOptions `optional:"true"`
}

// ConfigOutput 定义配置模块的输出结构
type ConfigOutput struct {
	fx.Out

	// 配置提供者
	Provider config.Provider
}

// Module 返回配置模块
func Module() fx.Option {
	return fx.Module("config",
		fx.Provide(
			ProvideConfigServices,
			// 提供具体的配置类型用于依赖注入
			func(provider config.Provider) *blockchainconfig.BlockchainOptions {
				return provider.GetBlockchain()
			},
			func(provider config.Provider) *consensus.ConsensusOptions {
				return provider.GetConsensus()
			},
			func(provider config.Provider) *repository.RepositoryOptions {
				return provider.GetRepository()
			},
			func(provider config.Provider) *compliance.ComplianceOptions {
				return provider.GetCompliance()
			},
		),
	)
}

// ProvideConfigServices 提供配置服务
func ProvideConfigServices(params ConfigParams) (ConfigOutput, error) {
	// 从应用配置选项获取用户配置
	var appConfig *types.AppConfig
	if params.AppOptions != nil {
		appConfig = params.AppOptions.GetAppConfig()
	}

	// 创建配置提供者
	provider := NewProvider(appConfig)

	return ConfigOutput{
		Provider: provider,
	}, nil
}
