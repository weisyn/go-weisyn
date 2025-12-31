// Package security provides message rate limiting functionality for network security.
package security

import (
	"fmt"
	"sync"
	"time"
)

// MessageRateLimiter 消息速率限制器，用于防止消息洪水攻击
type MessageRateLimiter struct {
	mu              sync.RWMutex
	messageCount    map[string][]time.Time
	maxMessages     int
	timeWindow      time.Duration
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

// NewMessageRateLimiter 创建新的消息速率限制器
func NewMessageRateLimiter(maxMessages int, timeWindow time.Duration) *MessageRateLimiter {
	mrl := &MessageRateLimiter{
		messageCount:    make(map[string][]time.Time),
		maxMessages:     maxMessages,
		timeWindow:      timeWindow,
		cleanupInterval: 1 * time.Minute,
		stopCh:          make(chan struct{}),
	}

	// 启动清理协程
	go mrl.cleanup()

	return mrl
}

// CheckMessage 检查消息是否允许发送
func (mrl *MessageRateLimiter) CheckMessage(peerID string) error {
	mrl.mu.Lock()
	defer mrl.mu.Unlock()

	now := time.Now()

	// 清理过期记录
	if times, exists := mrl.messageCount[peerID]; exists {
		validTimes := make([]time.Time, 0)
		for _, t := range times {
			if now.Sub(t) < mrl.timeWindow {
				validTimes = append(validTimes, t)
			}
		}
		mrl.messageCount[peerID] = validTimes
	}

	// 检查消息数量
	if len(mrl.messageCount[peerID]) >= mrl.maxMessages {
		return fmt.Errorf("消息速率超限: %d/%d 条/%v",
			len(mrl.messageCount[peerID]), mrl.maxMessages, mrl.timeWindow)
	}

	// 记录消息
	mrl.messageCount[peerID] = append(mrl.messageCount[peerID], now)

	return nil
}

// GetMessageCount 获取消息数量
func (mrl *MessageRateLimiter) GetMessageCount(peerID string) int {
	mrl.mu.RLock()
	defer mrl.mu.RUnlock()

	return len(mrl.messageCount[peerID])
}

// Reset 重置消息计数
func (mrl *MessageRateLimiter) Reset(peerID string) {
	mrl.mu.Lock()
	defer mrl.mu.Unlock()

	delete(mrl.messageCount, peerID)
}

// ResetAll 重置所有消息计数
func (mrl *MessageRateLimiter) ResetAll() {
	mrl.mu.Lock()
	defer mrl.mu.Unlock()

	mrl.messageCount = make(map[string][]time.Time)
}

// cleanup 定期清理过期记录
func (mrl *MessageRateLimiter) cleanup() {
	ticker := time.NewTicker(mrl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mrl.mu.Lock()
			now := time.Now()
			for peerID, times := range mrl.messageCount {
				validTimes := make([]time.Time, 0)
				for _, t := range times {
					if now.Sub(t) < mrl.timeWindow {
						validTimes = append(validTimes, t)
					}
				}
				if len(validTimes) == 0 {
					delete(mrl.messageCount, peerID)
				} else {
					mrl.messageCount[peerID] = validTimes
				}
			}
			mrl.mu.Unlock()
		case <-mrl.stopCh:
			return
		}
	}
}

// Stop 停止消息速率限制器
func (mrl *MessageRateLimiter) Stop() {
	close(mrl.stopCh)
}
