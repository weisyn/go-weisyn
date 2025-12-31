// Package ures 提供 URES 模块的 fx 依赖注入配置
//
// 🎯 **模块职责**：
// - CASStorage 服务提供
// - ResourceWriter 服务提供
// - 生命周期管理
//
// 📦 **导出服务**：
// - uresif.CASStorage (name: "cas_storage")
// - uresif.ResourceWriter (name: "resource_writer")
// - interfaces.InternalCASStorage (未命名，内部使用)
// - interfaces.InternalResourceWriter (未命名，内部使用)
//
// 🔗 **依赖关系**：
// - 输入：FileStore, HashManager, Logger
// - 输出：CASStorage, ResourceWriter
package ures

import (
	"context"

	// fx 框架
	"go.uber.org/fx"

	// 公共接口
	uresif "github.com/weisyn/v1/pkg/interfaces/ures"

	// 基础设施接口
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"

	// 内部实现
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
	"github.com/weisyn/v1/internal/core/ures/cas"
	"github.com/weisyn/v1/internal/core/ures/interfaces"
	"github.com/weisyn/v1/internal/core/ures/writer"
)

// ============================================================================
//                              模块输入依赖
// ============================================================================

// ModuleInput 定义 ures 模块的输入依赖
//
// 🎯 **依赖组织**：
// 本结构体使用fx.In标签，通过依赖注入自动提供所有必需的组件依赖。
type ModuleInput struct {
	fx.In

	// ========== 基础设施组件 ==========
	Logger log.Logger `optional:"true"` // 日志记录器

	// ========== 存储组件 ==========
	FileStore storage.FileStore `optional:"false"` // 文件存储

	// ========== 密码学组件 ==========
	HashManager crypto.HashManager `optional:"false"` // 哈希管理器
}

// ============================================================================
//                              模块输出服务
// ============================================================================

// ModuleOutput 定义 ures 模块的输出服务
//
// 🎯 **服务导出说明**：
// 本结构体使用fx.Out标签，将模块内部创建的公共服务接口统一导出，供其他模块使用。
type ModuleOutput struct {
	fx.Out

	// 核心服务导出（命名依赖）
	CASStorage     uresif.CASStorage     `name:"cas_storage"`    // CAS存储服务
	ResourceWriter uresif.ResourceWriter `name:"resource_writer"` // 资源写入服务

	// 内部接口导出（未命名，供内部使用）
	InternalCASStorage    interfaces.InternalCASStorage    // 内部CAS存储服务
	InternalResourceWriter interfaces.InternalResourceWriter // 内部资源写入服务
}

// ============================================================================
//                              模块定义
// ============================================================================

// ProvideServices 提供 ures 模块的所有服务
//
// 🎯 **服务创建**：
// 本函数负责创建 ures 模块的所有服务实例，并通过 ModuleOutput 统一导出。
func ProvideServices(input ModuleInput) (ModuleOutput, error) {
	// 🎯 为 URES 模块添加 module 字段，日志将路由到 node-business.log
	var uresLogger log.Logger
	if input.Logger != nil {
		uresLogger = input.Logger.With("module", "ures")
	}
	
	// 创建 CASStorage 服务
	casStorage, err := cas.NewService(input.FileStore, input.HashManager, uresLogger)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 创建 ResourceWriter 服务
	resourceWriter, err := writer.NewService(casStorage, input.HashManager, uresLogger)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 类型断言为公共接口
	var publicCASStorage uresif.CASStorage = casStorage
	var publicResourceWriter uresif.ResourceWriter = resourceWriter

	// 注册 URES CASStorage 到内存监控系统
	if reporter, ok := casStorage.(metricsiface.MemoryReporter); ok {
		metricsutil.RegisterMemoryReporter(reporter)
		if uresLogger != nil {
			uresLogger.Info("✅ URES CASStorage 已注册到内存监控系统")
		}
	}

	return ModuleOutput{
		CASStorage:            publicCASStorage,
		ResourceWriter:        publicResourceWriter,
		InternalCASStorage:    casStorage,
		InternalResourceWriter: resourceWriter,
	}, nil
}

// Module 提供 URES 模块的 fx 依赖注入
//
// 🎯 **核心功能**：
// - 提供 CASStorage 服务 ✅
// - 提供 ResourceWriter 服务 ✅
// - 管理生命周期 ✅
//
// 🔗 **依赖关系**：
// - 输入：FileStore, HashManager, Logger
// - 输出：CASStorage, ResourceWriter
//
// 📋 **导出服务**：
// - uresif.CASStorage (name: "cas_storage") ✅
// - uresif.ResourceWriter (name: "resource_writer") ✅
// - interfaces.InternalCASStorage (未命名，内部使用) ✅
// - interfaces.InternalResourceWriter (未命名，内部使用) ✅
//
// 使用示例：
//
//	app := fx.New(
//	    storage.Module(),
//	    crypto.Module(),
//	    ures.Module(),  // ✅ 添加 URES 模块
//	)
func Module() fx.Option {
	return fx.Module("ures",
		// ====================================================================
		//                           服务提供
		// ====================================================================

		fx.Provide(
			// 提供所有服务（通过 ModuleOutput 统一导出）
			// fx 会自动展开 ModuleOutput 结构体（因为它有 fx.Out）
			// 所有带 name tag 的字段会注册为命名依赖
			// 所有未命名的字段会注册为未命名依赖
			// 注意：统一使用命名依赖，确保一致性
			ProvideServices,
		),

		// ====================================================================
		//                         生命周期管理
		// ====================================================================

		fx.Invoke(
			func(lc fx.Lifecycle, logger log.Logger) {
				// 🎯 为 URES 模块添加 module 字段
				var uresLogger log.Logger
				if logger != nil {
					uresLogger = logger.With("module", "ures")
				}
				
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						if uresLogger != nil {
							uresLogger.Info("🚀 URES 模块正在启动...")
						}
						return nil
					},
					OnStop: func(ctx context.Context) error {
						if uresLogger != nil {
							uresLogger.Info("🛑 URES 模块正在停止...")
						}
						return nil
					},
				})
			},
		),

		// 模块加载日志
		fx.Invoke(
			func(logger log.Logger) {
				if logger != nil {
					// 🎯 为 URES 模块添加 module 字段
					uresLogger := logger.With("module", "ures")
					uresLogger.Info("✅ URES 模块已加载 (CASStorage, ResourceWriter)")
				}
			},
		),
	)
}
