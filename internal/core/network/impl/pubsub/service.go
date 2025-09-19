package pubsub

// service.go
// PubSub 服务封装（最小化，主要功能已集成在 Facade）
// 备注：由于 Facade 已提供主要 PubSub 能力，该服务保持最小实现即可

// Service PubSub服务（最小实现）
type Service struct {
	m   *TopicManager
	p   *Publisher
	enc *Encoder
	val *Validator
}

// New 创建 PubSub 服务
func New() *Service {
	return &Service{
		m:   NewTopicManager(),
		p:   NewPublisher(),
		enc: NewEncoder(),
		val: NewValidator(),
	}
}

// GetComponents 返回内部组件（供 Facade 使用）
func (s *Service) GetComponents() (*TopicManager, *Publisher, *Encoder, *Validator) {
	return s.m, s.p, s.enc, s.val
}
