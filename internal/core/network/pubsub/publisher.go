// Package pubsub provides publisher functionality for publish-subscribe messaging.
package pubsub

import (
	"context"
	"sync"
	"time"
)

// publisher.go
// 发布路径（批量/压缩阈值）与发布策略
// - 提供批量聚合与压缩阈值判断入口
// - 与 Internal/encoding、compress 配合

// PublishStats 发布统计（仅内部使用）
type PublishStats struct {
	TotalMessages  uint64
	FailedMessages uint64
	LastPublishAt  time.Time
}

// Publisher 发布器（最小实现）
type Publisher struct {
	mu    sync.RWMutex
	stats map[string]*PublishStats // 每主题统计
}

// NewPublisher 创建发布器
func NewPublisher() *Publisher {
	return &Publisher{
		stats: make(map[string]*PublishStats),
	}
}

// Publish 执行发布（最小实现）
func (p *Publisher) Publish(topic string, _payload []byte) error {
	// 更新统计
	p.updateStats(topic, true)

	// 占位：实际发布逻辑由上层 Facade 处理
	// 这里仅做内部统计与策略判断
	return nil
}

// PublishWithContext 带上下文的发布（可扩展）
func (p *Publisher) PublishWithContext(ctx context.Context, topic string, payload []byte) error {
	select {
	case <-ctx.Done():
		p.updateStats(topic, false)
		return ctx.Err()
	default:
		return p.Publish(topic, payload)
	}
}

// GetStats 获取发布统计（仅内部使用）
func (p *Publisher) GetStats(topic string) *PublishStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if stats, ok := p.stats[topic]; ok {
		cp := *stats
		return &cp
	}
	return nil
}

// updateStats 更新统计（内部方法）
func (p *Publisher) updateStats(topic string, success bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.stats[topic]; !ok {
		p.stats[topic] = &PublishStats{}
	}
	s := p.stats[topic]
	s.TotalMessages++
	if !success {
		s.FailedMessages++
	}
	s.LastPublishAt = time.Now()
}
