package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID 请求ID中间件
// 为每个请求生成唯一追踪ID
type RequestID struct{}

// NewRequestID 创建请求ID中间件
func NewRequestID() *RequestID {
	return &RequestID{}
}

// Middleware 返回Gin中间件
func (m *RequestID) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 尝试从请求头获取已有的RequestID
		requestID := c.GetHeader("X-Request-ID")

		// 如果没有，生成新的
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// 注入上下文
		c.Set("request_id", requestID)

		// 设置响应头
		c.Header("X-Request-ID", requestID)

		c.Next()
	}
}

// GetRequestID 从上下文或请求头获取请求ID（与 RequestID 中间件配合）
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok2 := v.(string); ok2 && s != "" {
			return s
		}
	}
	if h := c.GetHeader("X-Request-ID"); h != "" {
		return h
	}
	return ""
}
