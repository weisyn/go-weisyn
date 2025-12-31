// Package security provides rate limiting functionality for network security.
package security

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter 速率限制器，用于防止DDoS攻击
type RateLimiter struct {
	mu sync.RWMutex

	// peerConnections 按 peerID 统计的连接数
	peerConnections map[string]int

	// ipConnections 按 IP 统计的连接数
	ipConnections map[string]int

	// maxConnections 全局最大连接数（按 *唯一 peer* 统计）
	maxConnections int

	// maxPerIP 单个 IP 允许的最大连接数
	maxPerIP int

	cleanupInterval time.Duration
	stopCh          chan struct{}
}

// NewRateLimiter 创建新的速率限制器
func NewRateLimiter(maxConnections, maxPerIP int) *RateLimiter {
	rl := &RateLimiter{
		peerConnections: make(map[string]int),
		ipConnections:   make(map[string]int),
		maxConnections:  maxConnections,
		maxPerIP:        maxPerIP,
		cleanupInterval: 1 * time.Minute,
		stopCh:          make(chan struct{}),
	}

	// 启动清理协程
	go rl.cleanup()

	return rl
}

// CheckConnection 检查连接是否允许
func (rl *RateLimiter) CheckConnection(peerID, ip string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// 检查总连接数（按唯一 peer 统计）
	totalPeers := 0
	for _, count := range rl.peerConnections {
		totalPeers += count
	}

	if totalPeers >= rl.maxConnections {
		return fmt.Errorf("连接数已达上限: %d/%d", totalPeers, rl.maxConnections)
	}

	// 检查单IP连接数
	if rl.ipConnections[ip] >= rl.maxPerIP {
		return fmt.Errorf("IP连接数已达上限: %d/%d", rl.ipConnections[ip], rl.maxPerIP)
	}

	// 允许连接：分别更新 peer 和 IP 计数
	rl.peerConnections[peerID]++
	rl.ipConnections[ip]++

	return nil
}

// RemoveConnection 移除连接
func (rl *RateLimiter) RemoveConnection(peerID, ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if count := rl.peerConnections[peerID]; count > 0 {
		rl.peerConnections[peerID]--
		if rl.peerConnections[peerID] == 0 {
			delete(rl.peerConnections, peerID)
		}
	}

	if count := rl.ipConnections[ip]; count > 0 {
		rl.ipConnections[ip]--
		if rl.ipConnections[ip] == 0 {
			delete(rl.ipConnections, ip)
		}
	}
}

// GetConnectionCount 获取连接数
func (rl *RateLimiter) GetConnectionCount() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	total := 0
	for _, count := range rl.peerConnections {
		total += count
	}

	return total
}

// GetIPConnectionCount 获取IP连接数
func (rl *RateLimiter) GetIPConnectionCount(ip string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return rl.ipConnections[ip]
}

// cleanup 定期清理过期连接
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			// 清理计数为0的条目
			for key, count := range rl.peerConnections {
				if count == 0 {
					delete(rl.peerConnections, key)
				}
			}
			for key, count := range rl.ipConnections {
				if count == 0 {
					delete(rl.ipConnections, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Stop 停止速率限制器
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}
