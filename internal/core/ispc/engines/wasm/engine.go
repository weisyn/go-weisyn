package wasm

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/storage"
	"github.com/weisyn/v1/pkg/interfaces/ures"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/engines/wasm/loader"
	"github.com/weisyn/v1/internal/core/ispc/engines/wasm/runtime"
)

// Engine WASM引擎核心实现
//
// 🎯 **设计理念**：实现InternalWASMEngine接口，负责WASM合约执行
// 📋 **架构原则**：整合manager.go和engine/service.go的功能，实现统一接口
//
// 🔗 **依赖关系**：
// - loader.ContractLoader：合约加载
// - runtime.WazeroRuntime：WASM运行时
// - hostabi.HostFunctionProvider：宿主函数提供（通过GetWASMHostFunctions获取）
type Engine struct {
	logger          log.Logger
	resourceManager ures.CASStorage
	storageProvider storage.Provider

	contractLoader *loader.ContractLoader
	runtime        *runtime.WazeroRuntime
	hostProvider   ispcInterfaces.HostFunctionProvider // 宿主函数提供者（通过内部接口暴露）
}

// NewEngine 创建WASM引擎实例
func NewEngine(
	logger log.Logger,
	resourceManager ures.CASStorage,
	storageProvider storage.Provider,
	fileStoreRootPath string,
	hostProvider ispcInterfaces.HostFunctionProvider,
) (*Engine, error) {
	if resourceManager == nil {
		return nil, fmt.Errorf("resourceManager cannot be nil")
	}
	if storageProvider == nil {
		return nil, fmt.Errorf("storageProvider cannot be nil")
	}
	if hostProvider == nil {
		return nil, fmt.Errorf("hostProvider cannot be nil")
	}

	// 创建运行时（使用默认配置）
	config := &runtime.WazeroConfig{
		UseCompiler:             true,
		EnableWASI:              true,
		ExecutionTimeoutSeconds: 60,
		MaxMemoryPages:          1024, // 64MB
		MaxStackDepth:           1024,
	}
	runtimeInst := runtime.NewWazeroRuntime(logger, config, nil)

	// 创建合约加载器
	contractLoader := loader.NewContractLoader(logger, fileStoreRootPath)

	return &Engine{
		logger:          logger,
		resourceManager: resourceManager,
		storageProvider: storageProvider,
		contractLoader:  contractLoader,
		runtime:         runtimeInst,
		hostProvider:   hostProvider,
	}, nil
}

// CallFunction 执行WASM合约函数
//
// 实现InternalWASMEngine接口
func (e *Engine) CallFunction(
	ctx context.Context,
	contractHash []byte,
	methodName string,
	params []uint64,
) ([]uint64, error) {
	// 将hash转换为hex string（供loader使用）
	contractAddress := hex.EncodeToString(contractHash)

	if e.logger != nil {
		e.logger.Debugf("开始执行WASM合约: %s.%s", contractAddress, methodName)
	}

	// 1. 加载合约（委托给contractLoader）
	contract, err := e.contractLoader.LoadContract(ctx, contractAddress)
	if err != nil {
		return nil, fmt.Errorf("加载合约失败: %w", err)
	}

	// 2. 编译合约（委托给runtime）
	compiled, err := e.runtime.CompileContract(ctx, contract.Bytecode)
	if err != nil {
		return nil, fmt.Errorf("编译合约失败: %w", err)
	}

	// 3. 注册宿主函数（必须在实例化之前！）
	// ExecutionContext应该已经通过context传递（由coordinator注入）
	executionID := fmt.Sprintf("execution_%s", contractAddress)
	hostFunctions, err := e.hostProvider.GetWASMHostFunctions(ctx, executionID)
	if err != nil {
		return nil, fmt.Errorf("获取宿主函数失败: %w", err)
	}
	if err := e.runtime.RegisterHostFunctions(hostFunctions); err != nil {
		return nil, fmt.Errorf("注册宿主函数失败: %w", err)
	}

	// 4. 创建实例（委托给runtime）
	instance, err := e.runtime.CreateInstance(ctx, compiled)
	if err != nil {
		return nil, fmt.Errorf("创建实例失败: %w", err)
	}
	defer func() {
		// 确保实例被销毁（即用即消）
		if destroyErr := e.runtime.DestroyInstance(ctx, instance); destroyErr != nil {
			if e.logger != nil {
				e.logger.Error("销毁实例失败")
			}
		}
	}()

	// 5. 执行函数（委托给runtime）
	results, err := e.runtime.ExecuteFunction(ctx, instance, methodName, params)
	if err != nil {
		if e.logger != nil {
			e.logger.Errorf("WASM执行失败: method=%s, error=%v", methodName, err)
		}
		return nil, fmt.Errorf("执行函数失败: %w", err)
	}

	if e.logger != nil {
		e.logger.Debugf("WASM合约执行完成: %s.%s", contractAddress, methodName)
	}

	return results, nil
}

// 确保Engine实现InternalWASMEngine接口
var _ ispcInterfaces.InternalWASMEngine = (*Engine)(nil)

// Close 关闭引擎，释放资源
func (e *Engine) Close() error {
	if e.runtime != nil {
		return e.runtime.Close()
	}
	return nil
}

