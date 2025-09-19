// 事件类型常量定义

package event

// 这个包提供了事件总线的实现，业务模块可以导入此包并使用这些常量
// 但业务特定的事件类型应该在各自的业务模块中定义，而不是在基础设施层

// EventType 事件类型
type EventType string

// 全局事件类型定义 - 只保留基础的系统事件类型
const (
	// 系统事件
	SystemStarted EventType = "system:started"
	SystemStopped EventType = "system:stopped"
)

// 注意: 业务特定的事件类型(如块事件、交易事件等)应该由相应的业务模块定义
// 例如，区块相关事件应该在区块链模块中定义
// 交易相关事件应该在交易处理模块中定义
// 这样可以避免基础设施层和业务逻辑的耦合
