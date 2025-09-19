package api

import (
	"github.com/weisyn/v1/internal/api/http"
	"go.uber.org/fx"
)

// Module 返回API模块选项，使其可以被fx框架注册
// 该函数的作用:
// 1. 创建一个名为"api"的fx模块，用于将所有API相关组件组织在一起
// 2. 确保HTTP服务能够被正确注册和初始化
// 3. 为未来可能添加的其他API类型(GraphQL/gRPC/WebSocket)预留扩展空间
func Module() fx.Option {
	return fx.Module("api",
		// 导出HTTP服务模块
		// 这会加载api/http包中定义的所有服务和处理器
		// 包括HTTP服务器的启动、路由注册和请求处理逻辑
		http.Module(),

		// 增加显式调用，确保HTTP服务器被启动
		fx.Invoke(func(server *http.Server) {
			// 通过依赖注入获取HTTP服务器实例
			// fx.Invoke确保该服务器实例会被正确初始化和启动
		}),

		// 🆕 增加内部管理服务器调用，确保内部管理服务器被启动
		// 🚨 重要：此服务器仅供内部开发使用，不对外暴露
		fx.Invoke(func(internalServer *http.InternalManagementServer) {
			// 通过依赖注入获取内部管理服务器实例
			// 该服务器将在不同端口上运行，提供测试网络管理功能
		}),

		// 注意：所有的依赖注入对象已经由其他模块提供
		// 不需要在这里重复提供，以避免依赖冲突
		// 包括：config.Config、interfaces.Logger、interfaces.Blockchain等

		// 以下模块需要时再实现
		// 每个模块将提供不同类型的API服务
		// graphql.Module(),  // GraphQL API服务，提供灵活的查询能力
		// grpc.Module(),     // gRPC API服务，提供高性能RPC调用
		// websocket.Module(), // WebSocket API服务，提供实时推送能力
	)
}
