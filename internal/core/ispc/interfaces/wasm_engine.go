package interfaces

import (
	"context"
)

// InternalWASMEngine WASM引擎内部接口（ISPC统一接口层）
//
// 🎯 **ISPC内部接口**：
// - 仅供ISPC模块内部使用
// - engines/wasm/Engine实现此接口
// - coordinator.Manager通过此接口调用WASM引擎
//
// 📋 **设计原则**：
// - 只暴露执行必需的公共方法
// - 简化接口，避免继承过多依赖
type InternalWASMEngine interface {
	// CallFunction 执行WASM合约函数
	//
	// 📋 **参数说明**：
	//   - ctx: 执行上下文（包含ExecutionContext）
	//   - contractHash: 合约内容哈希（32字节）
	//   - method: 方法名
	//   - params: 函数参数（[]uint64）
	//
	// 🔧 **返回值**：
	//   - []uint64: 执行结果
	//   - error: 执行错误
	CallFunction(
		ctx context.Context,
		contractHash []byte,
		method string,
		params []uint64,
	) ([]uint64, error)

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
	Close() error
}
