package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RateLimit 匿名限流中间件
// 实现区块链节点的匿名友好限流策略：
// - 读操作宽松限流
// - 写操作严格限流
// - 按IP/ASN/行为模式限流
type RateLimit struct {
	logger     *zap.Logger
	limiters   map[string]*rateLimiter
	mu         sync.RWMutex
	readLimit  int // 读操作QPS限制
	writeLimit int // 写操作QPS限制
}

// rateLimiter 简单的令牌桶限流器
type rateLimiter struct {
	tokens     int
	maxTokens  int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimit 创建限流中间件
func NewRateLimit(logger *zap.Logger, readLimit, writeLimit int) *RateLimit {
	return &RateLimit{
		logger:     logger,
		limiters:   make(map[string]*rateLimiter),
		readLimit:  readLimit,  // 默认: 100 QPS
		writeLimit: writeLimit, // 默认: 10 QPS
	}
}

// Middleware 返回Gin中间件
func (m *RateLimit) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取客户端标识（IP地址）
		clientID := c.ClientIP()

		// 判断操作类型
		isWrite := isWriteOperation(c.Request.URL.Path, c.Request.Method)

		// 选择限流策略
		limit := m.readLimit
		if isWrite {
			limit = m.writeLimit
		}

		// 检查限流
		if !m.allowRequest(clientID, limit) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Request rate limit exceeded",
					"details": gin.H{
						"limit":      limit,
						"retryAfter": "1s",
					},
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// allowRequest 检查是否允许请求
func (m *RateLimit) allowRequest(clientID string, limit int) bool {
	m.mu.Lock()
	limiter, exists := m.limiters[clientID]
	if !exists {
		limiter = &rateLimiter{
			tokens:     limit,
			maxTokens:  limit,
			lastRefill: time.Now(),
		}
		m.limiters[clientID] = limiter
	}
	m.mu.Unlock()

	return limiter.consume()
}

// consume 消费一个令牌
func (r *rateLimiter) consume() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 补充令牌（每秒补充maxTokens个）
	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	tokensToAdd := int(elapsed.Seconds()) * r.maxTokens
	if tokensToAdd > 0 {
		r.tokens += tokensToAdd
		if r.tokens > r.maxTokens {
			r.tokens = r.maxTokens
		}
		r.lastRefill = now
	}

	// 尝试消费令牌
	if r.tokens > 0 {
		r.tokens--
		return true
	}

	return false
}

// TODO: 高级限流策略
// - 按ASN限流（防止单个ASN恶意请求）
// - 按行为模式限流（检测异常行为）
// - 动态调整限流阈值（根据节点负载）
// - 白名单机制（可信节点）
