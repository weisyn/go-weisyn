package stream

import (
	libhost "github.com/libp2p/go-libp2p/core/host"
)

// Service 流式传输服务（最小实现）
// 备注：目前主要功能已集成在 Facade 中，该服务保持最小化状态
type Service struct {
	host libhost.Host
	sem  *Semaphore // 背压控制信号量
}

// New 创建流式传输服务
func New(host libhost.Host) *Service {
	return &Service{
		host: host,
		sem:  NewSemaphore(100), // 默认并发数100
	}
}

// GetSemaphore 返回背压控制信号量（供 Facade 使用）
func (s *Service) GetSemaphore() *Semaphore {
	return s.sem
}

// SetConcurrencyLimit 设置并发数上限
func (s *Service) SetConcurrencyLimit(limit int) {
	s.sem = NewSemaphore(limit)
}
