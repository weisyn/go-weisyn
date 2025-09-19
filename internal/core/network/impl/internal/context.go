package internal

// context.go
// 关联上下文键、元数据传递（实施阶段补充）

import "context"

// ContextKey 上下文键类型（方法框架）
type ContextKey string

// 预定义上下文键（方法框架）
const (
	ContextKeyCorrelationID ContextKey = "correlation_id"
	ContextKeyTraceID       ContextKey = "trace_id"
)

// WithCorrelationID 在上下文中设置关联ID
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ContextKeyCorrelationID, id)
}

// CorrelationIDFrom 从上下文获取关联ID
func CorrelationIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyCorrelationID).(string)
	return v, ok
}
