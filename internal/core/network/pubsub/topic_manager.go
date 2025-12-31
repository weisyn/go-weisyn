// Package pubsub provides topic management functionality for publish-subscribe messaging.
package pubsub

import (
	"sync"
	"time"
)

// topic_manager.go
// Topic 订阅/退订与生命周期管理
// - 维护主题与订阅处理器的映射
// - 提供订阅、退订、查询等方法签名

// TopicInfo 主题信息
type TopicInfo struct {
	Topic        string
	SubscribedAt time.Time
	HandlerCount int
}

// TopicManager 主题管理器（并发安全）
type TopicManager struct {
	mu     sync.RWMutex
	topics map[string]*TopicInfo // 主题 -> 订阅信息
}

// NewTopicManager 创建主题管理器
func NewTopicManager() *TopicManager {
	return &TopicManager{
		topics: make(map[string]*TopicInfo),
	}
}

// Subscribe 订阅主题
func (m *TopicManager) Subscribe(topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if info, exists := m.topics[topic]; exists {
		info.HandlerCount++
	} else {
		m.topics[topic] = &TopicInfo{
			Topic:        topic,
			SubscribedAt: time.Now(),
			HandlerCount: 1,
		}
	}
	return nil
}

// Unsubscribe 退订主题
func (m *TopicManager) Unsubscribe(topic string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if info, exists := m.topics[topic]; exists {
		info.HandlerCount--
		if info.HandlerCount <= 0 {
			delete(m.topics, topic)
		}
	}
	return nil
}

// IsSubscribed 查询是否已订阅
func (m *TopicManager) IsSubscribed(topic string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.topics[topic]
	return exists
}

// ListTopics 返回所有订阅主题的快照
func (m *TopicManager) ListTopics() []TopicInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]TopicInfo, 0, len(m.topics))
	for _, info := range m.topics {
		result = append(result, *info) // 复制值
	}
	return result
}

// GetTopicInfo 获取指定主题信息
func (m *TopicManager) GetTopicInfo(topic string) (*TopicInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if info, exists := m.topics[topic]; exists {
		cp := *info
		return &cp, true
	}
	return nil, false
}
