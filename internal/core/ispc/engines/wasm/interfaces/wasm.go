package interfaces

import (
	"context"

	"github.com/weisyn/v1/pkg/types"
)

// ============================================================================
//                           WASM引擎总接口（WASM引擎内部接口）
// ============================================================================

// InternalWASMEngine WASM引擎内部总接口（已迁移到ispc/interfaces）
//
// ⚠️ **已废弃**：此接口已迁移到 `internal/core/ispc/interfaces.InternalWASMEngine`
// 请使用 `ispcInterfaces.InternalWASMEngine` 替代
//
// 本文件保留以下接口供WASM引擎内部使用：
// - ContractLoader
// - WASMRuntime
// 这些接口是WASM引擎的内部实现细节，不需要在ISPC层面暴露。
//
// ⚠️ **架构变更**：旧的宿主函数接口（HostCapabilityProvider、HostCapabilityRegistry等）
// 已废弃，功能已迁移到 internal/core/ispc/hostabi/。

// ============================================================================
//                           纯内部接口定义
// ============================================================================

// ContractLoader 合约加载器接口
//
// 📋 **对应实现**：internal/core/engines/wasm/loader/
// 📋 **接口性质**：纯内部接口，无对应公共接口
//
// ⚠️ **架构说明**：
//   - 合约ID = 32字节SHA-256哈希（contentHash）
//   - 表示为：64位十六进制字符串（无0x前缀）
//   - LoadContract已包含格式验证，无需单独的地址验证方法
type ContractLoader interface {
	// LoadContract 加载指定contentHash的WASM合约
	// contractAddress: 64位十六进制字符串（32字节contentHash）
	LoadContract(ctx context.Context, contractAddress string) (*types.WASMContract, error)
}

// WASMRuntime WASM运行时接口
//
// 📋 **对应实现**：internal/core/engines/wasm/runtime/
// 📋 **接口性质**：纯内部接口，无对应公共接口
type WASMRuntime interface {
	// CompileContract 编译WASM合约字节码
	CompileContract(ctx context.Context, wasmBytes []byte) (*types.CompiledContract, error)

	// CreateInstance 基于编译模块创建执行实例
	CreateInstance(ctx context.Context, compiled *types.CompiledContract) (*types.WASMInstance, error)

	// ExecuteFunction 执行WASM实例中的指定函数
	ExecuteFunction(ctx context.Context, instance *types.WASMInstance, functionName string, params []uint64) ([]uint64, error)

	// DestroyInstance 销毁WASM实例，释放相关资源
	DestroyInstance(ctx context.Context, instance *types.WASMInstance) error

	// RegisterHostFunctions 注册宿主函数到WASM运行时
	RegisterHostFunctions(functions map[string]interface{}) error

	// Close 关闭运行时，释放所有相关资源
	Close() error
}

// ============================================================================
//                           宿主函数内部接口（已废弃）
// ============================================================================
//
// ⚠️ **架构变更**：以下接口已废弃，功能已迁移到 internal/core/ispc/hostabi/
//
// 新的架构：
// - 使用 ispcInterfaces.HostFunctionProvider 接口（定义在 internal/core/ispc/interfaces/host_function_provider.go）
// - 实现位于 internal/core/ispc/hostabi/host_function_provider.go
// - WASM 适配器位于 internal/core/ispc/hostabi/adapter/wasm_adapter.go
//
// 旧接口（已删除）：
// - HostCapabilityProvider
// - HostCapabilityRegistry
// - HostStandardInterface
// - HostBinding
