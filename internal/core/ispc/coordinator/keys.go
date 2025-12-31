// Package coordinator provides key management functionality for ISPC coordination.
package coordinator

// ContextKey ISPC协调器上下文键类型
//
// 🎯 **设计目的**：统一管理上下文键，避免魔法字符串与拼写错误
// 🔒 **类型安全**：使用自定义类型防止与其他字符串键混淆
// 📍 **作用范围**：coordinator包内使用，用于context.WithValue和context.Value操作
type ContextKey string

// ISPC协调器上下文键常量定义
//
// 🌐 **外部继承键**：从外部context继承的标准信息
// 🔧 **ISPC扩展键**：ISPC特有的执行上下文信息
const (
	// ==================== 外部继承键 ====================

	// ContextKeyTraceID 链路追踪ID键
	// 📍 **用途**：用于分布式链路追踪，跨服务传递追踪信息
	// 🔄 **继承**：从外部context.Context中提取并传递给隔离上下文
	ContextKeyTraceID ContextKey = "trace_id"

	// ContextKeyUserID 用户身份ID键
	// 📍 **用途**：标识执行请求的发起用户，用于权限控制和审计
	// 🔄 **继承**：从外部context.Context中提取
	ContextKeyUserID ContextKey = "user_id"

	// ContextKeyRequestID 请求ID键
	// 📍 **用途**：标识单次API请求，用于请求去重和日志关联
	// 🔄 **继承**：从外部context.Context中提取
	ContextKeyRequestID ContextKey = "request_id"

	// ==================== ISPC扩展键 ====================

	// ContextKeyExecutionStart ISPC执行开始时间键
	// 📍 **用途**：记录ISPC执行的开始时间，用于性能监控和超时计算
	// 🕐 **值类型**：time.Time
	// 🔄 **传递**：从manager.go扩展，传递给隔离上下文用于追踪
	ContextKeyExecutionStart ContextKey = "ispc_execution_start"

	// ContextKeyContract 合约地址键
	// 📍 **用途**：标识当前执行的智能合约地址
	// 📝 **值类型**：string
	// 🔄 **传递**：从manager.go扩展，传递给隔离上下文用于日志和监控
	ContextKeyContract ContextKey = "ispc_contract"

	// ContextKeyFunction 函数名称键
	// 📍 **用途**：标识当前执行的合约函数名称
	// 📝 **值类型**：string
	// 🔄 **传递**：从manager.go扩展，用于执行轨迹和错误定位
	ContextKeyFunction ContextKey = "ispc_function"

	// ContextKeyParamsCount 参数数量键
	// 📍 **用途**：记录函数参数的数量，用于执行统计和验证
	// 🔢 **值类型**：int
	// 📊 **用途**：主要用于监控和日志记录，不传递给隔离上下文
	ContextKeyParamsCount ContextKey = "ispc_params_count"

	// ContextKeyHasCallerKey 是否包含调用者私钥键
	// 📍 **用途**：标识当前执行是否提供了调用者私钥（用于签名）
	// ✅ **值类型**：bool
	// 🔐 **安全**：仅标识是否存在，不暴露私钥内容
	ContextKeyHasCallerKey ContextKey = "ispc_has_caller_key"

	// ContextKeyExecutionContext ExecutionContext键
	// 📍 **用途**：将ExecutionContext传递给WASM/ONNX引擎
	// 🔧 **值类型**：*context.ExecutionContext
	// 🔄 **传递**：从coordinator传递给引擎，用于HostABI调用
	ContextKeyExecutionContext ContextKey = "execution_context"
)
