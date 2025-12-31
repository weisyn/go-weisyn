package engine

import (
	"context"
	"encoding/hex"
	"fmt"

	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	wasmInterfaces "github.com/weisyn/v1/internal/core/ispc/engines/wasm/interfaces"
)

// Service WASM引擎实现
//
// 🎯 **设计理念**：实现 ispcInterfaces.InternalWASMEngine 接口
// 📋 **架构原则**：专门负责WASM合约执行的核心逻辑
//
// 📋 **对应接口**：internal/core/ispc/interfaces.InternalWASMEngine
// 📋 **职责范围**：合约执行的完整流程协调（加载→编译→实例化→执行→销毁）
//
// 🔗 **依赖关系**：
// - wasmInterfaces.ContractLoader：合约加载
// - wasmInterfaces.WASMRuntime：运行时管理
// - ispcInterfaces.HostFunctionProvider：宿主函数提供
type Service struct {
	// ==================== 基础设施服务 ====================
	logger log.Logger // 日志服务

	// ==================== 子组件依赖 ====================
	contractLoader wasmInterfaces.ContractLoader // 合约加载器
	runtime        wasmInterfaces.WASMRuntime    // WASM运行时
	hostProvider   ispcInterfaces.HostFunctionProvider  // 宿主函数提供者
}

// 确保Service实现ispcInterfaces.InternalWASMEngine接口
var _ ispcInterfaces.InternalWASMEngine = (*Service)(nil)

// NewService 创建WASM引擎服务
//
// 🎯 **构造器模式**：通过依赖注入创建引擎实例
//
// 📋 **参数说明**：
//   - logger: 日志服务
//   - contractLoader: 合约加载器
//   - runtime: WASM运行时
//   - hostProvider: 宿主函数提供者
func NewService(
	logger log.Logger,
	contractLoader wasmInterfaces.ContractLoader,
	runtime wasmInterfaces.WASMRuntime,
	hostProvider ispcInterfaces.HostFunctionProvider,
) *Service {
	return &Service{
		logger:         logger,
		contractLoader: contractLoader,
		runtime:        runtime,
		hostProvider:   hostProvider,
	}
}

// ============================================================================
//                    ispcInterfaces.InternalWASMEngine 接口实现
// ============================================================================

// CallFunction 执行WASM合约函数
//
// 🎯 **核心功能**：执行WASM智能合约的导出函数
// 📋 **执行流程**：加载→编译→实例化→注册宿主函数→执行→销毁（即用即消）
//
// 📋 **参数说明**：
//   - ctx: 执行上下文
//   - contractHash: 合约内容哈希（32字节）
//   - method: 要调用的方法名
//   - params: 函数参数（[]uint64）
//
// 📋 **返回值**：
//   - []uint64: 函数执行结果
//   - error: 执行错误
func (s *Service) CallFunction(
	ctx context.Context,
	contractHash []byte,
	method string,
	params []uint64,
) ([]uint64, error) {
	// 将hash转换为hex string（供loader使用）
	contractAddress := hex.EncodeToString(contractHash)
	
	if s.logger != nil {
		s.logger.Debugf("开始执行WASM合约: %s.%s", contractAddress, method)
	}

	// 1. 加载合约（委托给contractLoader）
	contract, err := s.contractLoader.LoadContract(ctx, contractAddress)
	if err != nil {
		return nil, fmt.Errorf("加载合约失败: %w", err)
	}

	// 2. 编译合约（委托给runtime）
	compiled, err := s.runtime.CompileContract(ctx, contract.Bytecode)
	if err != nil {
		return nil, fmt.Errorf("编译合约失败: %w", err)
	}

	// 3. 注册宿主函数（必须在实例化之前！）
	//
	// 🎯 **关键修复**：将宿主函数注册移到实例化之前
	// 原因：WASM 模块在实例化时需要解析所有导入的模块和函数
	//      如果 env 模块尚未注册，实例化会失败并报错：
	//      "module[env] not instantiated"
	hostFunctions, err := s.hostProvider.GetWASMHostFunctions(ctx, "execution_"+contractAddress)
	if err != nil {
		return nil, fmt.Errorf("获取宿主函数失败: %w", err)
	}
	if err := s.runtime.RegisterHostFunctions(hostFunctions); err != nil {
		return nil, fmt.Errorf("注册宿主函数失败: %w", err)
	}

	// 4. 创建实例（委托给runtime）
	// 此时 env 模块已注册，实例化可以正确解析导入的宿主函数
	instance, err := s.runtime.CreateInstance(ctx, compiled)
	if err != nil {
		return nil, fmt.Errorf("创建实例失败: %w", err)
	}
	defer func() {
		// 确保实例被销毁（即用即消原则）
		if destroyErr := s.runtime.DestroyInstance(ctx, instance); destroyErr != nil {
			if s.logger != nil {
				s.logger.Error("销毁实例失败")
			}
		}
	}()

	// 5. 执行函数（委托给runtime）
	results, err := s.runtime.ExecuteFunction(ctx, instance, method, params)
	if err != nil {
		return nil, fmt.Errorf("执行函数失败: %w", err)
	}

	if s.logger != nil {
		s.logger.Debugf("WASM合约执行完成: %s.%s", contractAddress, method)
	}

	return results, nil
}

// Close 关闭引擎，释放资源
//
// 🎯 **生命周期管理**：
// - 关闭WASM运行时
// - 清理编译缓存
// - 释放所有占用的资源
//
// 🔧 **返回值**：
//   - error: 关闭过程中的错误（如果有）
//
// ⚠️ **注意**：关闭后引擎不能再使用
func (s *Service) Close() error {
	if s.runtime != nil {
		return s.runtime.Close()
	}
	return nil
}
