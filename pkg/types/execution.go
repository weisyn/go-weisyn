// Package types provides execution type definitions.
package types

// ==================== 执行通用类型（统一放在 types 层） ====================

// 注意：EngineType 类型已被移除（未使用）
// 如需使用，可从 git 历史中恢复

// ExecutionParams 执行入参（为保持接口稳定性，避免直接耦合 pb）
// 区块链计算层负责将交易/资源等信息整理为统一的执行入参
type ExecutionParams struct {
	// 资源标识（如合约哈希/模型哈希）
	ResourceID []byte
	// 方法/入口（如合约方法名、模型入口）
	Entry string
	// 序列化后的参数负载
	Payload []byte
	// 执行上下文（可扩展键值）
	Context map[string]any
	// 执行费用 限制
	ExecutionFeeLimit uint64
	// 内存限制
	MemoryLimit uint32
	// 超时时间
	Timeout int64
	// 调用者地址
	Caller string
	// 合约地址
	ContractAddr string
}

// ExecutionResult 执行结果（为保持接口稳定性，避免直接耦合 pb）
// 区块链计算层负责将本结果映射为链内统一结果结构

type ExecutionResult struct {
	// 是否成功
	Success bool
	// 返回数据
	ReturnData []byte
	// 执行费用 或资源计量（按具体引擎语义映射）
	Consumed uint64
	// 附加元数据（事件计数、诊断信息等）
	Metadata map[string]any
}

// ==================== WASM类型系统支持 ====================

// 注意：ValueType 和 OptimizationResult 类型已被移除（未使用）
// 如需使用，可从 git 历史中恢复

// ==================== 执行指标系统 ====================

// 注意：ExecutionErrorType, ExecutionMetrics, EngineExecutionStats 类型已被移除（未使用）
// 如需使用，可从 git 历史中恢复
