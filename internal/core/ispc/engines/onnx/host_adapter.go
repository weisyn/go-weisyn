//go:build ignore
// +build ignore

package onnx

// ════════════════════════════════════════════════════════════════════════════════════════════════
// ⚠️ **已废弃 (DEPRECATED)** - 本文件为旧架构实现，已被 ISPC HostFunctionProvider 替代
// ════════════════════════════════════════════════════════════════════════════════════════════════
//
// 📋 **废弃原因**：
//   - 架构重构：ONNX 宿主函数现在由 internal/core/ispc/hostabi 统一提供
//   - 接口变更：AppendStateOutput 已改为 AddStateOutput，Log 已改为 LogDebug
//   - 统一设计：ONNX 和 WASM 宿主函数由同一个 HostFunctionProvider 提供
//
// 🔄 **迁移指南**：
//   - 新的实现：internal/core/ispc/hostabi/host_function_provider.go
//   - ONNX 宿主函数：buildONNXHostFunctions（5个最小只读原语）
//   - 获取方式：通过 engines.HostFunctionProvider.GetONNXHostFunctions
//
// 📚 **相关文档**：
//   - pkg/interfaces/ispc/README.md
//   - pkg/interfaces/engines/README.md
//
// ⏰ **计划移除时间**：v2.0 正式发布后
//
// ════════════════════════════════════════════════════════════════════════════════════════════════

import (
	"context"

	// ISPC 接口
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"

	// 日志
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ████████████████████████████████████████████████████████████████████████████████████████████
// HostAdapter - ONNX 引擎的 HostABI 适配层（已废弃）
// ████████████████████████████████████████████████████████████████████████████████████████████
//
// 🎯 **设计目的**：
// 为 ONNX 推理引擎提供宿主能力接口，通过 Go 接口直接调用 HostABI 方法。
// 与 WASM 不同，ONNX 不需要 ABI 编解码（直接 Go 接口调用），只需适配层封装。
//
// 🏗️ **架构位置**：
// - ONNX 推理模型执行期间，通过此适配层访问链上上下文、状态、输出创建等能力
// - HostABI 由 ISPC Coordinator 在执行前注入到 ExecutionContext
// - 适配层从 ExecutionContext 获取 HostABI 并提供便利方法
//
// 🔄 **调用链路**：
// 1. ISPC Coordinator 创建 ExecutionContext 并注入 HostABI
// 2. ONNX Manager 在推理前设置 HostAdapter 的 ExecutionContext
// 3. 推理过程中通过 HostAdapter 调用宿主能力（读上下文、记录输出、发事件）
//
// ⚠️ **使用场景**：
// - 读取链上上下文：高度/时间戳/调用者地址（用于条件推理）
// - 读取资源模型：通过 ResourceOutput 指向的内容哈希加载模型文件
// - 记录推理结果：AppendStateOutput（证据载体），必要时发事件
// - 一般不涉及资产/转账操作（ONNX 场景多为计算/推理/预测）
// - ⚠️ EUTXO模型无全局状态存储，不提供 StateGet/StateExists
//
// ████████████████████████████████████████████████████████████████████████████████████████████

// HostAdapter ONNX 引擎的宿主能力适配器
type HostAdapter struct {
	logger  log.Logger
	execCtx ispcInterfaces.ExecutionContext // 当前执行上下文
}

// NewHostAdapter 创建 ONNX HostABI 适配器
//
// 📋 **参数**：
//   - logger: 日志服务
//
// 🔧 **返回值**：
//   - *HostAdapter: 适配器实例
func NewHostAdapter(logger log.Logger) *HostAdapter {
	return &HostAdapter{
		logger:  logger,
		execCtx: nil, // 在推理前由 ONNX Manager 设置
	}
}

// SetExecutionContext 设置执行上下文
//
// 📋 **参数**：
//   - execCtx: 执行上下文（由 ISPC Coordinator 创建并注入 HostABI）
//
// 🎯 **用途**：ONNX Manager 在推理前调用，提供宿主能力访问路径
func (h *HostAdapter) SetExecutionContext(execCtx ispcInterfaces.ExecutionContext) {
	h.execCtx = execCtx
	if h.logger != nil {
		h.logger.Debug("✅ HostAdapter: ExecutionContext 已设置")
	}
}

// ClearExecutionContext 清理执行上下文
//
// 🎯 **用途**：ONNX Manager 在推理完成后调用，释放资源
func (h *HostAdapter) ClearExecutionContext() {
	h.execCtx = nil
	if h.logger != nil {
		h.logger.Debug("HostAdapter: ExecutionContext 已清理")
	}
}

// ==================== 便利方法（封装 HostABI 调用）====================

// GetBlockHeight 获取当前区块高度
//
// 🎯 **用途**：条件推理（例如：不同高度使用不同模型版本）
func (h *HostAdapter) GetBlockHeight(ctx context.Context) (uint64, error) {
	if h.execCtx == nil {
		return 0, ErrExecutionContextNotSet
	}

	hostABI := h.execCtx.HostABI()
	if hostABI == nil {
		return 0, ErrHostABINotAvailable
	}

	return hostABI.GetBlockHeight(ctx)
}

// GetBlockTimestamp 获取当前区块时间戳
//
// 🎯 **用途**：时序推理（例如：时间相关的预测模型）
func (h *HostAdapter) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	if h.execCtx == nil {
		return 0, ErrExecutionContextNotSet
	}

	hostABI := h.execCtx.HostABI()
	if hostABI == nil {
		return 0, ErrHostABINotAvailable
	}

	return hostABI.GetBlockTimestamp(ctx)
}

// GetCaller 获取调用者地址
//
// 🎯 **用途**：权限相关推理（例如：根据调用者身份选择模型）
func (h *HostAdapter) GetCaller(ctx context.Context) ([]byte, error) {
	if h.execCtx == nil {
		return nil, ErrExecutionContextNotSet
	}

	hostABI := h.execCtx.HostABI()
	if hostABI == nil {
		return nil, ErrHostABINotAvailable
	}

	return hostABI.GetCaller(ctx)
}

// AppendStateOutput 追加状态输出
//
// 🎯 **用途**：记录推理结果（证据载体）
//
// 📋 **参数**：
//   - stateID: 状态标识符（例如："inference_result"）
//   - stateVersion: 状态版本号（递增）
//   - executionResultHash: 推理结果哈希（摘要）
//   - publicInputs: ZK 公开输入（推理输入摘要，可选）
//   - parentStateHash: 父状态哈希（可选，用于状态链）
//
// 🎯 **典型使用**：
//   - 图像分类：记录分类结果及置信度
//   - 异常检测：记录检测到的异常及概率
//   - 预测模型：记录预测值及不确定性
func (h *HostAdapter) AppendStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	if h.execCtx == nil {
		return 0, ErrExecutionContextNotSet
	}

	hostABI := h.execCtx.HostABI()
	if hostABI == nil {
		return 0, ErrHostABINotAvailable
	}

	return hostABI.AppendStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)
}

// EmitEvent 发射事件
//
// 🎯 **用途**：记录推理事件（供链外索引/监听）
//
// 📋 **参数**：
//   - eventType: 事件类型（例如："inference_completed"）
//   - eventData: 事件数据（JSON/Protobuf）
//
// 🎯 **典型使用**：
//   - 推理完成事件：包含输入摘要、输出摘要、耗时等
//   - 异常事件：包含异常类型、异常详情等
func (h *HostAdapter) EmitEvent(ctx context.Context, eventType string, eventData []byte) error {
	if h.execCtx == nil {
		return ErrExecutionContextNotSet
	}

	hostABI := h.execCtx.HostABI()
	if hostABI == nil {
		return ErrHostABINotAvailable
	}

	return hostABI.EmitEvent(ctx, eventType, eventData)
}

// Log 记录日志
//
// 🎯 **用途**：调试输出（不进 ExecutionTrace，仅供开发期使用）
func (h *HostAdapter) Log(ctx context.Context, level string, message string) error {
	if h.execCtx == nil {
		return ErrExecutionContextNotSet
	}

	hostABI := h.execCtx.HostABI()
	if hostABI == nil {
		return ErrHostABINotAvailable
	}

	return hostABI.Log(ctx, level, message)
}

// ==================== 错误定义 ====================

var (
	// ErrExecutionContextNotSet 执行上下文未设置
	ErrExecutionContextNotSet = NewONNXError("execution context not set")

	// ErrHostABINotAvailable HostABI 不可用
	ErrHostABINotAvailable = NewONNXError("host ABI not available")
)

// ONNXError ONNX 引擎错误
type ONNXError struct {
	message string
}

// NewONNXError 创建 ONNX 错误
func NewONNXError(message string) *ONNXError {
	return &ONNXError{message: message}
}

// Error 实现 error 接口
func (e *ONNXError) Error() string {
	return e.message
}

// ████████████████████████████████████████████████████████████████████████████████████████████
// 实现说明
// ████████████████████████████████████████████████████████████████████████████████████████████
//
// 🎯 **当前状态（v1.0）**：
// - 提供基本的宿主能力封装（读上下文、状态、输出创建、事件发射）
// - ONNX 推理模型可通过 HostAdapter 访问链上能力
// - 与 WASM 共享同一套 HostABI 语义，无需重复实现
//
// ⚠️ **ONNX 特殊性**：
// - ONNX 推理通常不涉及资产/转账操作（仅计算/推理/预测）
// - 主要使用场景：
//   1. 读取链上上下文（高度/时间戳）→ 条件推理
//   2. 读取状态（超参数/配置）→ 配置推理行为
//   3. 记录推理结果（StateOutput）→ 证据链
//   4. 发射事件（Event）→ 链外监听/索引
//
// ⚠️ **待完善（v1.1）**：
// - 资源模型加载：通过 ResourceOutput 指向的内容哈希加载 ONNX 模型文件
// - 批量推理：一次性处理多个输入，生成多个 StateOutput
// - 推理缓存：缓存常用模型，避免重复加载
//
// 🔧 **集成步骤**：
// 1. 在 ONNX Manager 中持有 HostAdapter 实例
// 2. 推理前调用 HostAdapter.SetExecutionContext(execCtx)
// 3. 推理过程中通过 HostAdapter 调用宿主能力
// 4. 推理后调用 HostAdapter.ClearExecutionContext()
//
// ████████████████████████████████████████████████████████████████████████████████████████████
