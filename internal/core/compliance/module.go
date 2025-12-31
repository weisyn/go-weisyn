// Package compliance 提供WES系统的合规服务实现
package compliance

import (
	"go.uber.org/fx"

	"github.com/weisyn/v1/internal/config/compliance"
	complianceIfaces "github.com/weisyn/v1/pkg/interfaces/compliance"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ModuleInput 定义合规模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
//
// 📋 **依赖分类**：
// - 合规配置：ComplianceOptions合规策略配置
// - 基础设施：Logger日志服务
//
// ⚠️ **可选性控制**：
// - optional:"false" - 必需依赖，缺失时启动失败
// - optional:"true"  - 可选依赖，允许为nil，模块内需要nil检查
type ModuleInput struct {
	fx.In

	// 合规配置
	Config *compliance.ComplianceOptions `optional:"false"`

	// 基础设施组件
	Logger log.Logger `optional:"true"`
}

// ModuleOutput 定义合规模块的输出服务
//
// 🎯 **服务导出**：
// 本结构体使用fx.Out标签，将合规模块的主要服务导出，供其他模块使用。
// 合规模块采用简化设计，只暴露核心的Policy接口，内部依赖自行管理。
//
// 📋 **导出服务**：
//   - Policy: 合规策略决策服务，提供交易和操作的合规检查功能
//     内部自动集成身份验证和地理位置查询能力
type ModuleOutput struct {
	fx.Out

	// 合规策略服务（统一入口）
	Policy complianceIfaces.Policy `name:"compliance_policy"`
}

// Module 构建并返回合规模块的fx配置
//
// 🎯 **模块构建器**：
// 本函数是合规模块的主要入口点，负责构建完整的fx模块配置。
// 通过fx.Module组织所有合规服务的依赖注入配置，确保服务的正确创建和生命周期管理。
//
// 🏗️ **构建流程**：
// 1. 创建身份凭证验证服务：IdentityRegistry
// 2. 创建地理位置查询服务：GeoIPService（简单实现）
// 3. 创建合规策略决策服务：Policy（主服务）
// 4. 聚合输出服务：将所有服务包装为ModuleOutput统一导出
// 5. 注册初始化回调：模块加载完成后的日志记录
//
// 📋 **服务创建顺序**：
// - IdentityRegistry: 身份凭证验证，独立服务
// - GeoIPService: 地理位置查询，独立服务
// - Policy: 合规策略决策，依赖前两个服务
//
// 🔧 **使用方式**：
//
//	app := fx.New(
//	    compliance.Module(),
//	    // 其他模块...
//	)
//
// ⚠️ **依赖要求**：
// 使用此模块前需要确保合规配置已正确提供。
func Module() fx.Option {
	return fx.Module("compliance",
		fx.Provide(
			// 合规策略服务（使用工厂函数创建）
			fx.Annotate(
				func(input ModuleInput) (complianceIfaces.Policy, error) {
					// 🎯 为合规模块添加 module 字段，日志将路由到 node-system.log
					var complianceLogger log.Logger
					if input.Logger != nil {
						complianceLogger = input.Logger.With("module", "compliance")
					}
					return CreateCompliancePolicy(input.Config, complianceLogger)
				},
				fx.As(new(complianceIfaces.Policy)),
			),

			// 模块输出聚合
			func(policy complianceIfaces.Policy) ModuleOutput {
				return ModuleOutput{
					Policy: policy,
				}
			},
		),

		fx.Invoke(
			func(logger log.Logger) {
				if logger != nil {
					// 🎯 为合规模块添加 module 字段
					complianceLogger := logger.With("module", "compliance")
					complianceLogger.Info("合规模块已加载")
				}
			},
		),
	)
}
