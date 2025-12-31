// Package interfaces provides engine manager interfaces for ISPC operations.
package interfaces

import (
	"context"
)

// InternalEngineManager 引擎管理器内部接口
//
// 🎯 **ISPC内部接口**：
// - 统一管理WASM和ONNX引擎
// - coordinator.Manager通过此接口调用引擎
//
// 📋 **设计原则**：
// - 提供统一的执行接口，隐藏引擎实现细节
// - coordinator只依赖此接口，不直接依赖具体引擎
type InternalEngineManager interface {
	// ExecuteWASM 执行WASM合约
	//
	// 参数：
	//   - ctx: 执行上下文（包含ExecutionContext）
	//   - hash: 合约内容哈希（32字节）
	//   - method: 方法名
	//   - params: 函数参数（[]uint64）
	//
	// 返回值：
	//   - []uint64: 执行结果
	//   - error: 执行错误
	ExecuteWASM(
		ctx context.Context,
		hash []byte,
		method string,
		params []uint64,
	) ([]uint64, error)

	// ExecuteONNX 执行ONNX模型推理
	//
	// 参数：
	//   - ctx: 执行上下文（包含ExecutionContext）
	//   - hash: 模型内容哈希（32字节）
	//   - tensorInputs: 张量输入列表（包含数据和形状信息）
	//
	// 返回值：
	//   - []TensorOutput: 推理结果（富张量结构）
	//   - error: 推理错误
	ExecuteONNX(
		ctx context.Context,
		hash []byte,
		tensorInputs []TensorInput,
	) ([]TensorOutput, error)

	// Shutdown 关闭引擎管理器，释放所有资源
	//
	// 🎯 **生命周期管理**：
	// - 关闭WASM引擎
	// - 关闭ONNX引擎
	// - 清理所有占用的资源
	//
	// 📋 **参数**：
	//   - ctx: 关闭上下文（用于控制关闭超时）
	//
	// 🔧 **返回值**：
	//   - error: 关闭过程中的错误（如果有）
	//
	// ⚠️ **注意**：
	//   - 关闭后管理器不能再使用
	//   - 应该等待所有正在执行的请求完成后再关闭
	//   - 建议使用context.Context控制关闭超时
	Shutdown(ctx context.Context) error
}

