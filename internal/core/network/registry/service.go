package registry

import (
	libhost "github.com/libp2p/go-libp2p/core/host"
)

// Service 协议注册服务（最小化集成器）
// 备注：主要功能已集成在 Facade，该服务保持最小化状态
type Service struct {
	host     libhost.Host
	registry *ProtocolRegistry
	neg      *VersionNegotiator
	comp     *VersionComparator
	wrapper  *HandlerWrapper
}

// New 创建协议注册服务
func New(host libhost.Host) *Service {
	return &Service{
		host:     host,
		registry: NewProtocolRegistry(),
		neg:      NewVersionNegotiator(),
		comp:     NewVersionComparator(),
		wrapper:  NewHandlerWrapper(),
	}
}

// GetComponents 返回内部组件（供 Facade 使用）
func (s *Service) GetComponents() (*ProtocolRegistry, *VersionNegotiator, *VersionComparator, *HandlerWrapper) {
	return s.registry, s.neg, s.comp, s.wrapper
}
