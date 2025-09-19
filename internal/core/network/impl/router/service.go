package router

// service.go
// Router 服务（最小化，基础去重/限流已迁移到 PubSub Validator）
// 备注：目前主要功能已集成在 Facade 与 PubSub 中，该服务保持最小化状态

// Service Router服务（最小实现）
type Service struct {
	// 占位：未来可扩展复杂路由策略（如负载均衡、地理优先等）
}

// New 创建 Router 服务
func New() *Service {
	return &Service{}
}

// IsMinimal 返回是否为最小实现（用于诊断）
func (s *Service) IsMinimal() bool {
	return true
}
