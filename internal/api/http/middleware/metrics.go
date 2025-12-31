package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Metrics 指标收集中间件
// 收集API性能指标，用于监控和告警
type Metrics struct {
	logger          *zap.Logger
	requestCounter  *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	requestSize     *prometheus.SummaryVec
	responseSize    *prometheus.SummaryVec
}

// NewMetrics 创建指标中间件
func NewMetrics(logger *zap.Logger) *Metrics {
	m := &Metrics{
		logger: logger,
	}

	// 注册Prometheus指标
	m.requestCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "wes",
			Subsystem: "api",
			Name:      "requests_total",
			Help:      "Total number of API requests",
		},
		[]string{"method", "path", "status"},
	)

	m.requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "wes",
			Subsystem: "api",
			Name:      "request_duration_seconds",
			Help:      "API request duration in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10},
		},
		[]string{"method", "path"},
	)

	m.requestSize = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  "wes",
			Subsystem:  "api",
			Name:       "request_size_bytes",
			Help:       "API request size in bytes",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method", "path"},
	)

	m.responseSize = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace:  "wes",
			Subsystem:  "api",
			Name:       "response_size_bytes",
			Help:       "API response size in bytes",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method", "path"},
	)

	return m
}

// Middleware 返回Gin中间件
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// 记录请求大小
		requestSize := c.Request.ContentLength
		if requestSize > 0 {
			m.requestSize.WithLabelValues(method, path).Observe(float64(requestSize))
		}

		// 处理请求
		c.Next()

		// 收集指标
		duration := time.Since(start)
		status := c.Writer.Status()
		responseSize := c.Writer.Size()

		// 更新Prometheus指标
		m.requestCounter.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
		m.requestDuration.WithLabelValues(method, path).Observe(duration.Seconds())

		if responseSize > 0 {
			m.responseSize.WithLabelValues(method, path).Observe(float64(responseSize))
		}

		// 记录调试日志
		m.logger.Debug("Request metrics collected",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("duration", duration),
			zap.Int64("request_size", requestSize),
			zap.Int("response_size", responseSize),
		)
	}
}
