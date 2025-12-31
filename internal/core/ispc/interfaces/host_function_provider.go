package interfaces

import "context"

// HostFunctionProvider 定义 ISPC 引擎在执行期可见的宿主函数提供能力。
//
// 🎯 设计原则：
//   - 只暴露执行期能力（GetXxxHostFunctions / 缓存查看 / 清理）
//   - 不暴露 SetChainQuery / SetTxAdapter 等装配期方法，这些只在 module 组装根中使用
//
// 对比关系：
//   - p2p/host.Runtime 通过 BandwidthProvider / ResourceManagerInspector 暴露能力
//   - ispc/hostabi.HostFunctionProvider 通过本接口对 engines / coordinator 暴露能力
type HostFunctionProvider interface {
	// GetWASMHostFunctions 为一次 WASM 执行构建宿主函数映射。
	//
	// 要求：
	//   - ctx 必须已经通过 hostabi.WithExecutionContext 注入 ExecutionContext
	//   - executionID 仅用于日志和调试，不参与语义
	GetWASMHostFunctions(ctx context.Context, executionID string) (map[string]interface{}, error)

	// GetONNXHostFunctions 为一次 ONNX 推理构建只读宿主函数映射。
	//
	// 设计约束：
	//   - 只提供只读查询能力，不允许任何状态写入
	GetONNXHostFunctions(ctx context.Context, executionID string) (map[string]interface{}, error)

	// GetCacheStats 返回内部原语调用缓存的统计信息（如果未启用缓存可返回 nil）。
	GetCacheStats() map[string]interface{}

	// ClearCache 清空内部原语调用缓存。
	ClearCache()
}


