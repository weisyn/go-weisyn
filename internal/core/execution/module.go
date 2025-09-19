package execution

import (
	"go.uber.org/fx"

	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/repository"

	"github.com/weisyn/v1/internal/core/execution/host"

	// migration模块已移除：execution专注于合约/模型执行，不处理数据迁移

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
)

// ModuleInput 执行模块的输入依赖
type ModuleInput struct {
	fx.In

	// 基础设施依赖
	Logger   log.Logger     `optional:"true"`
	EventBus event.EventBus `optional:"true"`

	// 数据层依赖
	Repository repository.RepositoryManager `optional:"false"`

	// 注意：移除对blockchain服务的直接依赖，避免循环依赖
	// execution模块应该是被blockchain模块使用，而不是依赖blockchain模块
	// TransactionService blockchain.TransactionService `optional:"true"`
	// BlockService       blockchain.BlockService       `optional:"true"`
	// ChainService       blockchain.ChainService       `optional:"true"`

	// 执行引擎依赖（通过明确命名获取）
	WASMEngine execution.EngineAdapter `name:"wasm_engine" optional:"true"`
	ONNXEngine execution.EngineAdapter `name:"onnx_engine" optional:"true"`
}

// ModuleOutput 执行模块的输出服务
type ModuleOutput struct {
	fx.Out

	// 核心执行服务
	EngineManager          execution.EngineManager          `name:"execution_engine_manager"`
	HostCapabilityRegistry execution.HostCapabilityRegistry `name:"execution_host_registry"`
	ExecutionCoordinator   execution.ExecutionCoordinator   `name:"execution_coordinator"`

	// 数据迁移服务已移除：execution专注于合约/模型执行协调

	// ABI 服务
	ABIService execiface.ABIService `name:"execution_abi_service"`
}

// Module 执行模块的fx选项
//
// 提供：
// - EngineManager: 引擎管理器，负责多引擎注册和分发
// - HostCapabilityRegistry: 宿主能力注册表，聚合各种宿主能力提供者
// - ExecutionCoordinator: 执行协调器，提供统一的执行入口
//
// 依赖：
// - Logger: 日志记录器（可选）
// - EventBus: 事件总线（可选）
// - Repository: 仓储管理器（必需）
// - WASMEngine: WASM执行引擎适配器（可选）
// - ONNXEngine: ONNX执行引擎适配器（可选）
func Module() fx.Option {
	return fx.Module("blockchain-execution",
		// 提供统一的执行服务
		fx.Provide(ProvideServices),

		// 生命周期管理
		fx.Invoke(func(logger log.Logger) {
			if logger != nil {
				logger.Info("执行模块已启动")
			}
		}),
	)
}

// ProvideServices 提供执行模块服务，遵循标准的fx.In/fx.Out模式
// 参数：ModuleInput（由fx注入，包含所有依赖）
// 返回：ModuleOutput（包含所有提供的服务）
func ProvideServices(input ModuleInput) (ModuleOutput, error) {
	serviceInput := ServiceInput{
		Logger:     input.Logger,
		WASMEngine: input.WASMEngine,
		ONNXEngine: input.ONNXEngine,
	}

	serviceOutput, err := CreateExecutionServices(serviceInput)
	if err != nil {
		return ModuleOutput{}, err
	}

	return ModuleOutput{
		EngineManager:          serviceOutput.EngineManager,
		HostCapabilityRegistry: serviceOutput.HostCapabilityRegistry,
		ExecutionCoordinator:   serviceOutput.ExecutionCoordinator,
		ABIService:             serviceOutput.ABIService,
	}, nil
}

// registerHostProviders 注册各类宿主能力提供者
func registerHostProviders(input ModuleInput, hostRegistry execution.HostCapabilityRegistry) error {
	// 注册IO提供者
	ioProvider := host.NewIOProvider()
	if err := hostRegistry.RegisterProvider(ioProvider); err != nil {
		return err
	}

	// 注册状态提供者
	stateProvider := host.NewStateProvider()
	if err := hostRegistry.RegisterProvider(stateProvider); err != nil {
		return err
	}

	// 事件提供者已移除 - execution模块使用同步操作，不需要事件系统

	// 注册UTXO提供者
	utxoProvider := host.NewUTXOProvider()
	if err := hostRegistry.RegisterProvider(utxoProvider); err != nil {
		return err
	}

	return nil
}

// MigrationComponents 已移除 - execution模块专注于合约/模型执行协调

// 注意：SimpleExecutionCoordinator已删除，统一使用coordinator.DefaultExecutionCoordinator
