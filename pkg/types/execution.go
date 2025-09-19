package types

// HeaderAdvice 表示对区块头可选字段的建议（从 pkg/interfaces/execution 迁移）
type HeaderAdvice struct {
	WasmRuntimeVersion    *string
	WasmFeatures          *uint32
	OnnxRuntimeVersion    *string
	OnnxFeatures          *uint32
	ExecutionFeeUsedTotal *uint64
	ExecutionFeeLimit     *uint64
}

// ==================== 执行通用类型（统一放在 types 层） ====================

// EngineType 引擎类型标识（如 wasm、onnx）。建议使用小写短名以便配置与日志输出
// 说明：由 interfaces 层的接口方法返回/接收，避免在接口层重复定义类型。

type EngineType string

const (
	EngineTypeWASM EngineType = "wasm"
	EngineTypeONNX EngineType = "onnx"
)

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

// ValueType WASM值类型
type ValueType string

const (
	ValueTypeI32 ValueType = "i32"
	ValueTypeI64 ValueType = "i64"
	ValueTypeF32 ValueType = "f32"
	ValueTypeF64 ValueType = "f64"
)

// OptimizationResult 编译器优化结果
type OptimizationResult struct {
	// 是否优化成功
	Success bool `json:"success"`

	// 原始大小
	OriginalSize uint64 `json:"originalSize"`

	// 优化后大小
	OptimizedSize uint64 `json:"optimizedSize"`

	// 改进幅度（百分比）
	Improvement float64 `json:"improvement"`

	// 优化后字节码
	OptimizedBytecode []byte `json:"optimizedBytecode"`

	// 应用的优化过程
	AppliedPasses []string `json:"appliedPasses"`

	// 元数据
	Metadata map[string]any `json:"metadata"`
}

// ==================== 执行指标系统 ====================

// ExecutionErrorType 执行错误类型
type ExecutionErrorType string

// ExecutionMetrics 执行指标信息
type ExecutionMetrics struct {
	// 总执行次数
	TotalExecutions uint64 `json:"total_executions"`

	// 成功执行次数
	SuccessfulExecutions uint64 `json:"successful_executions"`

	// 失败执行次数
	FailedExecutions uint64 `json:"failed_executions"`

	// 平均执行时间（毫秒）
	AverageExecutionTimeMs float64 `json:"average_execution_time_ms"`

	// 总执行费用消耗
	TotalResourceConsumed uint64 `json:"total_resource_consumed"`

	// 按引擎类型分类的统计
	EngineStats map[EngineType]EngineExecutionStats `json:"engine_stats"`
}

// EngineExecutionStats 单个引擎的执行统计
type EngineExecutionStats struct {
	// 执行次数
	ExecutionCount uint64 `json:"execution_count"`

	// 成功次数
	SuccessCount uint64 `json:"success_count"`

	// 失败次数
	FailureCount uint64 `json:"failure_count"`

	// 平均执行时间（毫秒）
	AvgExecutionTimeMs float64 `json:"avg_execution_time_ms"`

	// 平均执行费用消耗
	AvgExecutionFeeConsumed float64 `json:"avg_execution_fee_consumed"`

	// 最后执行时间戳
	LastExecutionTimestamp int64 `json:"last_execution_timestamp"`
}
