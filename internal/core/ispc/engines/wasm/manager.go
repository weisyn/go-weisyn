package wasm

import (
	"context"
	"fmt"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/ures"
	"github.com/weisyn/v1/pkg/types"

	wasmInterfaces "github.com/weisyn/v1/internal/core/ispc/engines/wasm/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/engines/wasm/abi"
	"github.com/weisyn/v1/internal/core/ispc/engines/wasm/loader"
	"github.com/weisyn/v1/internal/core/ispc/engines/wasm/runtime"
)

// Manager WASM引擎管理器
//
// 🎯 **设计理念**：薄实现，严格遵循WES三层架构 [[memory:9105627]]
// 📋 **架构原则**：Manager只负责依赖注入和接口方法委托，不包含复杂业务逻辑
//
// 实现内部wasmInterfaces.InternalWASMEngine接口，委托所有方法给对应的子组件实现
// 仅做合约执行（即用即消），不包含监控、缓存、统计等越界功能
//
// 🔗 **依赖关系**：
// - repository.ResourceManager：获取WASM合约字节码
// - storage.Provider：存储提供者，用于编译缓存等
// - 各子组件：abi/、loader/、runtime/ 实现具体业务逻辑
// - hostabi.HostFunctionProvider：宿主函数提供者（来自 internal/core/ispc/hostabi/）
type Manager struct {
	// ==================== 基础设施服务 ====================
	logger log.Logger // 日志服务

	// ==================== 资源获取服务 ====================
	resourceManager ures.CASStorage // 资源存储管理器
	storageProvider storage.Provider           // 存储提供者

	// ==================== 子组件实例（委托目标） ====================
	contractLoader wasmInterfaces.ContractLoader // 合约加载器
	runtime        wasmInterfaces.WASMRuntime    // WASM运行时
	abiService     ispcInterfaces.ABIService            // ABI服务

	// Host 子组件实例
	functionProvider ispcInterfaces.HostFunctionProvider // 宿主函数提供者（来自 hostabi）
}

// NewManagerV2 创建WASM引擎管理器（v2.0 简化架构）
//
// ✅ 架构简化：只需要 functionProvider（来自 ISPC）
func NewManagerV2(
	logger log.Logger,
	resourceManager ures.CASStorage,
	storageProvider storage.Provider,
	fileStoreRootPath string,
	functionProvider ispcInterfaces.HostFunctionProvider,
) (*Manager, error) {
	// Fail-Fast：检查必需依赖
	if resourceManager == nil {
		return nil, fmt.Errorf("NewManagerV2: resourceManager 不能为 nil")
	}
	if storageProvider == nil {
		return nil, fmt.Errorf("NewManagerV2: storageProvider 不能为 nil")
	}
	if functionProvider == nil {
		return nil, fmt.Errorf("NewManagerV2: functionProvider 不能为 nil")
	}

	// 创建运行时（使用默认配置）
	config := &runtime.WazeroConfig{
		UseCompiler:             true,
		EnableWASI:              true, // ✅ 启用WASI支持（Go编译的WASM需要）
		ExecutionTimeoutSeconds: 60,
		MaxMemoryPages:          1024, // 64MB
		MaxStackDepth:           1024,
	}
	runtimeInst := runtime.NewWazeroRuntime(logger, config, nil)

	// 创建合约加载器
	contractLoader := loader.NewContractLoader(logger, fileStoreRootPath)

	// 创建ABI服务
	abiService := abi.NewService(logger)

	manager := &Manager{
		logger:           logger,
		resourceManager:  resourceManager,
		storageProvider: storageProvider,
		contractLoader:   contractLoader, // 初始化合约加载器
		runtime:          runtimeInst,
		abiService:       abiService,
		functionProvider: functionProvider,
	}

	if logger != nil {
		logger.Info("✅ WASM引擎管理器初始化完成（v2.0架构）")
	}

	return manager, nil
}

// ⚠️ **架构变更**：旧的 NewManager (v1.0) 已删除
// 旧架构使用的 HostCapabilityProvider、HostCapabilityRegistry 等接口已废弃
// 请使用 NewManagerV2，它只需要 functionProvider（来自 hostabi）

// ============================================================================
//                    ispcInterfaces.WASMEngine 公共接口实现（委托给runtime）
// ============================================================================

// CallFunction 执行WASM合约函数
//
// 🎯 **核心功能**：执行WASM智能合约的导出函数
// 📋 **委托实现**：组合调用loader、runtime、hostProvider等子组件
//
// ⚠️ **标识协议对齐**（参考 IDENTIFIER_AND_NAMESPACE_PROTOCOL_SPEC.md）：
// - contractAddress 参数语义：资源实例标识（ResourceInstanceId）或资源代码标识（ResourceCodeId）
// - 当前实现：接受 64 位 hex 字符串（32 字节 contentHash = ResourceCodeId）
// - 未来扩展：可支持 ResourceInstanceId（OutPoint 编码）用于多实例场景
func (m *Manager) CallFunction(ctx context.Context, contractAddress string, functionName string, params []uint64, callerPrivateKey ...string) ([]uint64, error) {
	// 1. 加载合约（委托给contractLoader）
	// contractAddress: 64位hex字符串，表示资源内容哈希（ResourceCodeId）
	// ⚠️ 注意：此参数不是"账户地址"，而是"资源标识"，属于对象标识命名空间，而非地址命名空间
	contract, err := m.contractLoader.LoadContract(ctx, contractAddress)
	if err != nil {
		return nil, fmt.Errorf("加载合约失败: %w", err)
	}

	// 2. 编译合约（委托给runtime）
	compiled, err := m.runtime.CompileContract(ctx, contract.Bytecode)
	if err != nil {
		return nil, fmt.Errorf("编译合约失败: %w", err)
	}

	// 3. 注册宿主函数（必须在实例化之前！）
	// 注意：ExecutionContext已通过ctx传递给GetWASMHostFunctions，由HostFunctionProvider自行提取
	//
	// 🎯 **关键修复**：将宿主函数注册移到实例化之前
	// 原因：WASM 模块在实例化时需要解析所有导入的模块和函数
	//      如果 env 模块尚未注册，实例化会失败并报错：
	//      "module[env] not instantiated"
	hostFunctions, err := m.functionProvider.GetWASMHostFunctions(ctx, "execution_"+contractAddress)
	if err != nil {
		return nil, fmt.Errorf("获取宿主函数失败: %w", err)
	}
	if err := m.runtime.RegisterHostFunctions(hostFunctions); err != nil {
		return nil, fmt.Errorf("注册宿主函数失败: %w", err)
	}

	// 4. 创建实例（委托给runtime）
	// 此时 env 模块已注册，实例化可以正确解析导入的宿主函数
	instance, err := m.runtime.CreateInstance(ctx, compiled)
	if err != nil {
		return nil, fmt.Errorf("创建实例失败: %w", err)
	}
	defer func() {
		// 确保实例被销毁（即用即消）
		if destroyErr := m.runtime.DestroyInstance(ctx, instance); destroyErr != nil {
			m.logger.Error("销毁实例失败")
		}
	}()

	// 5. 执行函数（委托给runtime）
	m.logger.Debugf("🔧 开始执行 WASM 函数: %s, 参数=%v", functionName, params)
	results, err := m.runtime.ExecuteFunction(ctx, instance, functionName, params)
	if err != nil {
		m.logger.Errorf("❌ WASM 执行失败: function=%s, error=%v", functionName, err)
		return nil, fmt.Errorf("执行函数失败: %w", err)
	}

	// 打印 WASM 执行结果（第三方包调用后）
	m.printWASMExecutionResult(contractAddress, functionName, params, results)

	return results, nil
}

// RegisterHostFunctions 注册宿主函数到WASM引擎
//
// 🎯 **委托实现**：直接委托给runtime处理
func (m *Manager) RegisterHostFunctions(functions map[string]interface{}) error {
	return m.runtime.RegisterHostFunctions(functions)
}

// ============================================================================
//                    ispcInterfaces.ABIService 公共接口实现（委托给abiService）
// ============================================================================

// RegisterABI 注册合约ABI
//
// 🎯 **委托实现**：委托给abiService处理
func (m *Manager) RegisterABI(contractID string, abi *types.ContractABI) error {
	return m.abiService.RegisterABI(contractID, abi)
}

// EncodeParameters 编码函数参数
//
// 🎯 **委托实现**：委托给abiService处理
func (m *Manager) EncodeParameters(contractID, method string, args []interface{}) ([]byte, error) {
	return m.abiService.EncodeParameters(contractID, method, args)
}

// DecodeResult 解码函数返回值
//
// 🎯 **委托实现**：委托给abiService处理
func (m *Manager) DecodeResult(contractID, method string, data []byte) ([]interface{}, error) {
	return m.abiService.DecodeResult(contractID, method, data)
}

// GetWASMHostFunctions 获取WASM宿主函数集合
//
// 🎯 **委托实现**：直接委托给functionProvider处理
func (m *Manager) GetWASMHostFunctions(ctx context.Context, executionID string) (map[string]interface{}, error) {
	return m.functionProvider.GetWASMHostFunctions(ctx, executionID)
}

// GetONNXHostFunctions 获取ONNX宿主函数集合
//
// 🎯 **委托实现**：直接委托给functionProvider处理
func (m *Manager) GetONNXHostFunctions(ctx context.Context, executionID string) (map[string]interface{}, error) {
	return m.functionProvider.GetONNXHostFunctions(ctx, executionID)
}

// ============================================================================
//                    ContractLoader 内部接口实现（委托给contractLoader）
// ============================================================================

// LoadContract 加载WASM合约（根据contentHash）
//
// 🎯 **委托实现**：委托给contractLoader处理
//
// 📋 **参数说明**：
//   - contractAddress: 64位十六进制字符串（32字节SHA-256哈希）
//   - 语义：资源内容哈希（ResourceCodeId），属于对象标识命名空间
//   - ⚠️ 注意：此参数不是"账户地址"（Address），而是"资源标识"（ResourceCodeId）
//
// ⚠️ **标识协议对齐**：
// - 参数名虽为 "Address"，但实际语义是 ResourceCodeId（内容哈希）
// - 未来可扩展支持 ResourceInstanceId（OutPoint 编码）用于多实例场景
func (m *Manager) LoadContract(ctx context.Context, contractAddress string) (*types.WASMContract, error) {
	return m.contractLoader.LoadContract(ctx, contractAddress)
}

// ============================================================================
//                    WASMRuntime 内部接口实现（委托给runtime）
// ============================================================================

// CompileContract 编译WASM合约字节码
//
// 🎯 **委托实现**：委托给runtime处理
func (m *Manager) CompileContract(ctx context.Context, wasmBytes []byte) (*types.CompiledContract, error) {
	return m.runtime.CompileContract(ctx, wasmBytes)
}

// CreateInstance 基于编译模块创建执行实例
//
// 🎯 **委托实现**：委托给runtime处理
func (m *Manager) CreateInstance(ctx context.Context, compiled *types.CompiledContract) (*types.WASMInstance, error) {
	return m.runtime.CreateInstance(ctx, compiled)
}

// ExecuteFunction 执行WASM实例中的指定函数
//
// 🎯 **委托实现**：委托给runtime处理
func (m *Manager) ExecuteFunction(ctx context.Context, instance *types.WASMInstance, functionName string, params []uint64) ([]uint64, error) {
	return m.runtime.ExecuteFunction(ctx, instance, functionName, params)
}

// DestroyInstance 销毁WASM实例，释放相关资源
//
// 🎯 **委托实现**：委托给runtime处理
func (m *Manager) DestroyInstance(ctx context.Context, instance *types.WASMInstance) error {
	return m.runtime.DestroyInstance(ctx, instance)
}

// Close 关闭运行时，释放所有相关资源
//
// 🎯 **委托实现**：委托给runtime处理
func (m *Manager) Close() error {
	return m.runtime.Close()
}

// ⚠️ **架构变更**：旧的宿主函数接口方法已删除
// 这些方法依赖于已废弃的 HostCapabilityProvider、HostCapabilityRegistry 等接口
// 新的架构使用 hostabi.HostFunctionProvider，通过 GetWASMHostFunctions() 直接提供宿主函数映射

// printWASMExecutionResult 打印 WASM 执行结果（wazero 调用后）
//
// 🎯 **调试用途**：
//   - 在 wazero 执行完成后，打印执行结果
//   - 帮助观察 WASM 引擎的执行状态
//
// 📋 **打印内容**：
//   - 合约地址、函数名
//   - 输入参数
//   - 返回值（wazero 原生 []uint64）
func (m *Manager) printWASMExecutionResult(contractAddr, functionName string, params, results []uint64) {
	m.logger.Info("========== 🔧 WASM 执行结果（wazero）==========")
	m.logger.Infof("合约地址: %s", contractAddr)
	m.logger.Infof("调用函数: %s", functionName)
	m.logger.Infof("输入参数: %v", params)
	m.logger.Infof("返回值: %v", results)

	// 解析状态码（通常第一个返回值是状态码）
	if len(results) > 0 {
		statusCode := results[0]
		if statusCode == 0 {
			m.logger.Infof("✅ 执行状态: SUCCESS (0)")
		} else {
			m.logger.Warnf("⚠️ 执行状态: ERROR (%d)", statusCode)
		}
	}

	m.logger.Info("=============================================")
}
