// Package execution 提供执行服务工厂实现
package execution

import (
	"github.com/weisyn/v1/internal/core/execution/abi"
	"github.com/weisyn/v1/internal/core/execution/coordinator"
	"github.com/weisyn/v1/internal/core/execution/env"
	"github.com/weisyn/v1/internal/core/execution/host"
	"github.com/weisyn/v1/internal/core/execution/manager"
	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ServiceInput 定义执行服务工厂的输入参数
type ServiceInput struct {
	// 基础设施依赖
	Logger log.Logger `optional:"true"`

	// 引擎适配器
	WASMEngine execution.EngineAdapter `name:"wasm_engine" optional:"true"`
	ONNXEngine execution.EngineAdapter `name:"onnx_engine" optional:"true"`
}

// ServiceOutput 定义执行服务工厂的输出结果
type ServiceOutput struct {
	EngineManager          execution.EngineManager
	HostCapabilityRegistry execution.HostCapabilityRegistry
	ExecutionCoordinator   execution.ExecutionCoordinator
	ABIService             execution.ABIService
}

// CreateExecutionServices 创建执行服务
//
// 🏭 **执行服务工厂**：
// 该函数负责创建执行模块的所有服务，处理引擎注册和协调器初始化。
// 将复杂的服务创建逻辑从module.go中分离出来，保持module.go的薄实现。
//
// 参数：
//   - input: 服务创建所需的输入参数
//
// 返回：
//   - ServiceOutput: 创建的服务实例集合
//   - error: 创建过程中的错误
func CreateExecutionServices(input ServiceInput) (ServiceOutput, error) {
	// 1. 创建引擎注册表并注册明确的引擎
	registry := manager.NewRegistry()

	// 注册WASM引擎（如果可用）
	if input.WASMEngine != nil {
		if err := registry.Register(input.WASMEngine); err != nil {
			if input.Logger != nil {
				input.Logger.Error("注册WASM引擎失败: " + err.Error())
			}
			return ServiceOutput{}, err
		}
		if input.Logger != nil {
			input.Logger.Info("成功注册WASM引擎")
		}
	}

	// 注册ONNX引擎（如果可用）
	if input.ONNXEngine != nil {
		if err := registry.Register(input.ONNXEngine); err != nil {
			if input.Logger != nil {
				input.Logger.Error("注册ONNX引擎失败: " + err.Error())
			}
			return ServiceOutput{}, err
		}
		if input.Logger != nil {
			input.Logger.Info("成功注册ONNX引擎")
		}
	}

	// 2. 创建引擎管理器
	engineManager := manager.NewEngineManager(registry)

	// 3. 创建宿主能力注册表
	hostRegistry := host.NewHostCapabilityRegistryWrapper(input.Logger)

	// 4. 监控组件已在coordinator中默认使用NoOp实现
	// 保持execution模块的轻量化，无需额外配置

	// 5. 创建环境顾问（暂时使用nil，避免循环依赖）
	// 注意：环境顾问将在blockchain模块中创建并注入到execution中
	var envAdvisor *env.CoordinatorAdapter = nil

	// 6. 创建执行分发器
	dispatcher := manager.NewExecutionDispatcher(registry, input.Logger)

	// 7. 数据迁移服务已移除 - execution模块专注于合约/模型执行

	// 8. 注册宿主能力提供者（简化实现）
	// 基本的IO提供者注册在module.go中处理

	// 9. 创建执行协调器（使用默认NoOp监控实现）
	execCoordinator := coordinator.NewExecutionCoordinatorSimple(engineManager, hostRegistry, envAdvisor, dispatcher, input.Logger)

	// 10. 创建 ABI 服务（生产依赖）
	abiService := abi.NewABIManager(nil)

	if input.Logger != nil {
		input.Logger.Info("✅ 执行模块所有服务初始化完成")
	}

	return ServiceOutput{
		EngineManager:          engineManager,
		HostCapabilityRegistry: hostRegistry,
		ExecutionCoordinator:   execCoordinator,
		ABIService:             abiService,
	}, nil
}

// 宿主能力提供者注册逻辑已移至module.go中的registerHostProviders函数
