package interfaces

import (
	"github.com/weisyn/v1/pkg/interfaces/ispc"
)

// ISPCCoordinator ISPC执行协调器内部接口
//
// 🎯 **内部接口设计**
//
// 继承 pkg/interfaces/ispc.ISPCCoordinator 公共接口，
// 并添加 ISPC 特有的内部能力，遵循 WES 三层架构原则
//
// 📋 **设计原则**：
// - 继承公共接口，保持 API 一致性
// - 可添加内部特有能力，支持复杂执行协调场景
// - 严格遵循三层架构：公共接口 → 内部接口 → 具体实现
//
// 🔧 **使用说明**：
// - 仅供 ISPC 模块内部使用，不对外暴露
// - Manager 实现此接口，而不是直接实现公共接口
// - 通过 fx.As 导出为公共接口服务
//
// 🏗️ **架构关系**：
//
//	pkg/interfaces/ispc.ISPCCoordinator (公共接口)
//	         ↑ 继承
//	internal/core/ispc/interfaces.ISPCCoordinator (内部接口)
//	         ↑ 实现
//	internal/core/ispc/coordinator.Manager (具体实现)
type ISPCCoordinator interface {
	// 继承公共接口
	ispc.ISPCCoordinator

	// ==================== 内部专用方法 (如有需要可添加) ====================
	// 例如：
	// - 获取执行上下文详情（调试用）
	// - 性能指标采集
	// - 内部状态管理
	// GetExecutionMetrics(ctx context.Context, executionID string) (*ExecutionMetrics, error)
}

// 📝 **注意**：
// - StateOutputProto、WASMExecutionResult、ONNXExecutionResult 已定义在 pkg/interfaces/ispc
// - 内部接口仅继承公共接口，不重复定义类型
// - TX 层通过 pkg/interfaces/ispc 调用，不直接依赖此内部接口
