package execution

import (
	"context"

	"github.com/weisyn/v1/pkg/types"
)

// ExecutionCoordinator 定义执行协调器的公共接口
//
// 设计目标：
// 1. 提供统一的执行协调入口，屏蔽内部实现复杂性
// 2. 支持依赖注入框架（如fx）进行模块装配
// 3. 保持接口与实现分离，便于单元测试和Mock
// 4. 遵循依赖倒置原则，不依赖具体实现细节
type ExecutionCoordinator interface {
	// Execute 执行资源（合约/模型等）的统一入口
	//
	// 参数：
	// - ctx：执行上下文，用于控制超时和取消
	// - params：标准化执行参数，包含资源ID、入口、负载等信息
	//
	// 返回值：
	// - types.ExecutionResult：标准化执行结果
	// - error：执行过程中的错误
	Execute(ctx context.Context, params types.ExecutionParams) (types.ExecutionResult, error)

	// GetSupportedEngines 返回当前支持的引擎类型列表
	//
	// 返回值：
	// - []types.EngineType：支持的引擎类型数组
	GetSupportedEngines() []types.EngineType

	// GetExecutionMetrics 获取执行相关的指标信息
	//
	// 返回值：
	// - ExecutionMetrics：包含执行统计、性能指标等信息
	GetExecutionMetrics() types.ExecutionMetrics
}
