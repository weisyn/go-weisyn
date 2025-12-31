// Package resourcesvc 提供资源视图服务的 fx 配置
package resourcesvc

import (
	"fmt"

	"go.uber.org/fx"

	"github.com/weisyn/v1/pkg/interfaces/eutxo"
	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/persistence"
	resourcesvciface "github.com/weisyn/v1/pkg/interfaces/resourcesvc"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
)

// ModuleInput 定义 resourcesvc 模块的输入依赖
type ModuleInput struct {
	fx.In

	Logger            log.Logger
	ResourceUTXOQuery eutxo.ResourceUTXOQuery `name:"resource_utxo_query"`
	ResourceQuery     persistence.ResourceQuery `name:"resource_query"` // ✅ 需要匹配 persistence 模块的导出标签
	UTXOQuery         persistence.UTXOQuery    `name:"utxo_query"`    // ✅ 新增：用于查询 UTXO 获取锁定条件（使用 persistence 的 UTXOQuery）
	TxQuery           persistence.TxQuery       `name:"tx_query"`       // ✅ 新增：用于查询交易和区块时间戳（需要匹配 persistence 模块的导出标签）
	BlockQuery        persistence.BlockQuery    `name:"block_query"`    // ✅ 新增：用于通过 blockHash 查询区块（需要匹配 persistence 模块的导出标签）
	BadgerStore       storage.BadgerStore                             // ✅ 新增：用于创建历史查询服务（未命名依赖，从 infrastructure/storage 模块注入）
}

// ModuleOutput 定义 resourcesvc 模块的输出服务
type ModuleOutput struct {
	fx.Out

	ResourceViewService resourcesvciface.Service

	// InternalService 提供对具体实现 *Service 的访问，用于内部控制器（如自动健康检查）
	InternalService *Service
}

// ProvideServices 提供 resourcesvc 模块的所有服务
func ProvideServices(input ModuleInput) (ModuleOutput, error) {
	service, err := NewService(
		input.ResourceUTXOQuery,
		input.ResourceQuery,
		input.UTXOQuery,   // ✅ 新增：传递 UTXOQuery
		input.TxQuery,     // ✅ 新增：传递 TxQuery
		input.BlockQuery,  // ✅ 新增：传递 BlockQuery
		input.BadgerStore, // ✅ 新增：传递 BadgerStore
		input.Logger,
	)
	if err != nil {
		return ModuleOutput{}, err
	}

	// 注册到内存监控系统
	if reporter, ok := service.(metricsiface.MemoryReporter); ok {
		metricsutil.RegisterMemoryReporter(reporter)
		if input.Logger != nil {
			input.Logger.Info("✅ ResourceViewService 已注册到内存监控系统")
		}
	}

	// 将具体实现类型暴露给内部控制器使用
	svcImpl, ok := service.(*Service)
	if !ok {
		return ModuleOutput{}, fmt.Errorf("resourcesvc: unexpected service implementation type")
	}

	return ModuleOutput{
		ResourceViewService: service,
		InternalService:     svcImpl,
	}, nil
}

// Module 返回 resourcesvc 模块的 fx 配置
func Module() fx.Option {
	return fx.Module("resourcesvc",
		fx.Provide(ProvideServices),
		// 启动 ResourceUTXO 自动健康检查控制器
		fx.Invoke(StartAutoResourceHealthController),
	)
}
