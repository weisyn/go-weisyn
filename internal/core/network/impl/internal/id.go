package internal

// id.go
// 能力：生成 correlationId/traceId 等与消息相关的无业务标识。
// 依赖/公共接口：
// - 若项目已有统一 ID/UUID 服务，应优先依赖公共接口；若无，则本模块仅作最小随机 ID 适配器。
// 目的：
// - 保证 Network 层的请求-响应关联、链路追踪（与日志/事件配合）
// 非目标：
// - 不引入全局 ID 规范，遵循项目级公共 ID 接口（若存在）

// IDGenerator ID 生成器（方法框架）
type IDGenerator struct{}

// NewIDGenerator 创建 ID 生成器
func NewIDGenerator() *IDGenerator { return &IDGenerator{} }

// NewCorrelationID 生成新的关联ID（方法框架）
func (g *IDGenerator) NewCorrelationID() string { return "" }

// NewTraceID 生成新的追踪ID（方法框架）
func (g *IDGenerator) NewTraceID() string { return "" }
