package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	infralog "github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"go.uber.org/zap"
)

// Logger 日志中间件
// 记录所有API请求的详细信息（复用系统统一日志接口）
type Logger struct {
	logger infralog.Logger
}

// NewLogger 创建日志中间件（使用统一日志接口）
func NewLogger(logger infralog.Logger) *Logger {
	return &Logger{logger: logger}
}

// Middleware 返回Gin中间件
func (m *Logger) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// 获取请求ID
		requestID := GetRequestID(c)

		// 处理请求
		c.Next()

		// 计算延迟
		latency := time.Since(start)

		// 使用统一日志接口获取底层zap记录器（结构化日志）
		zl := m.logger.GetZapLogger()
		if zl != nil {
			fields := []zap.Field{
				zap.String("request_id", requestID),
				zap.String("method", c.Request.Method),
				zap.String("path", path),
				zap.String("query", query),
				zap.Int("status", c.Writer.Status()),
				zap.Duration("latency", latency),
				zap.String("client_ip", c.ClientIP()),
				zap.String("user_agent", c.Request.UserAgent()),
			}
			if len(c.Errors) > 0 {
				fields = append(fields, zap.String("errors", c.Errors.String()))
			}
			switch {
			case c.Writer.Status() >= 500:
				zl.Error("HTTP request", fields...)
			case c.Writer.Status() >= 400:
				zl.Warn("HTTP request", fields...)
			default:
				zl.Info("HTTP request", fields...)
			}
			return
		}

		// 回退：若无底层zap，可使用文本日志（不建议，但保证不崩）
		msg := fmt.Sprintf("HTTP request | id=%s method=%s path=%s?%s status=%d latency=%s ip=%s ua=%s",
			requestID, c.Request.Method, path, query, c.Writer.Status(), latency.String(), c.ClientIP(), c.Request.UserAgent())
		switch {
		case c.Writer.Status() >= 500:
			m.logger.Error(msg)
		case c.Writer.Status() >= 400:
			m.logger.Warn(msg)
		default:
			m.logger.Info(msg)
		}
	}
}
