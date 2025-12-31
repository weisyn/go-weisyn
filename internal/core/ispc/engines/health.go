package engines

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// EngineHealthStatus 引擎健康状态
type EngineHealthStatus string

const (
	EngineHealthHealthy   EngineHealthStatus = "healthy"   // 健康
	EngineHealthDegraded  EngineHealthStatus = "degraded"  // 降级（有错误但可用）
	EngineHealthUnhealthy EngineHealthStatus = "unhealthy" // 不健康（不可用）
)

// EngineErrorStats 引擎错误统计
type EngineErrorStats struct {
	TotalErrors       uint64            // 总错误数
	ErrorByType       map[string]uint64 // 按错误类型统计
	LastErrorTime     time.Time         // 最后错误时间
	LastError         error             // 最后错误
	ConsecutiveErrors uint64            // 连续错误数
	mutex             sync.RWMutex      // 保护统计数据的并发访问
}

// NewEngineErrorStats 创建引擎错误统计
func NewEngineErrorStats() *EngineErrorStats {
	return &EngineErrorStats{
		ErrorByType: make(map[string]uint64),
	}
}

// RecordError 记录错误
func (s *EngineErrorStats) RecordError(err error) {
	if err == nil {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	atomic.AddUint64(&s.TotalErrors, 1)
	atomic.AddUint64(&s.ConsecutiveErrors, 1)

	// 记录错误类型
	errorType := getErrorType(err)
	s.ErrorByType[errorType]++

	s.LastErrorTime = time.Now()
	s.LastError = err
}

// RecordSuccess 记录成功（重置连续错误计数）
func (s *EngineErrorStats) RecordSuccess() {
	atomic.StoreUint64(&s.ConsecutiveErrors, 0)
}

// GetStats 获取统计信息（线程安全）
func (s *EngineErrorStats) GetStats() (totalErrors uint64, errorByType map[string]uint64, lastErrorTime time.Time, consecutiveErrors uint64, lastError error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	totalErrors = atomic.LoadUint64(&s.TotalErrors)
	consecutiveErrors = atomic.LoadUint64(&s.ConsecutiveErrors)

	// 深拷贝错误类型统计
	errorByType = make(map[string]uint64)
	for k, v := range s.ErrorByType {
		errorByType[k] = v
	}

	lastErrorTime = s.LastErrorTime
	lastError = s.LastError

	return totalErrors, errorByType, lastErrorTime, consecutiveErrors, lastError
}

// getErrorType 获取错误类型（用于分类统计）
func getErrorType(err error) string {
	if err == nil {
		return "unknown"
	}

	errMsg := err.Error()

	// 错误类型分类
	switch {
	case containsAny(errMsg, "timeout", "deadline exceeded"):
		return "timeout"
	case containsAny(errMsg, "connection", "network", "refused"):
		return "network"
	case containsAny(errMsg, "not found", "missing"):
		return "not_found"
	case containsAny(errMsg, "invalid", "malformed"):
		return "invalid_input"
	case containsAny(errMsg, "resource", "exhausted", "out of"):
		return "resource_exhausted"
	case containsAny(errMsg, "permission", "unauthorized", "forbidden"):
		return "permission"
	case containsAny(errMsg, "compile", "compilation"):
		return "compilation"
	case containsAny(errMsg, "runtime", "execution"):
		return "runtime"
	default:
		return "unknown"
	}
}

// containsAny 检查字符串是否包含任一子串
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// EngineHealth 引擎健康信息
type EngineHealth struct {
	Status      EngineHealthStatus // 健康状态
	LastCheck   time.Time          // 最后检查时间
	ErrorStats  *EngineErrorStats  // 错误统计
	IsAvailable bool               // 是否可用
}

// HealthCheckConfig 健康检查配置
//
// 🎯 **设计原则**：
// - 参考 onnxruntime_go 的错误处理模式
// - 错误应该被记录和统计，但不应该阻止后续请求
// - 健康检查应该用于监控和告警，而不是阻止执行
type HealthCheckConfig struct {
	// 是否启用健康检查（禁用时，即使连续错误也不会标记为不可用）
	Enabled bool
	// 连续错误阈值（超过此值标记为不健康）
	UnhealthyThreshold uint64
	// 降级阈值（超过此值但未达到不健康阈值时标记为降级）
	DegradedThreshold uint64
}

// updateWASMHealthStatus 更新WASM引擎健康状态
//
// 🎯 **健康状态判断**：
// - Healthy: 连续错误数 < 3
// - Degraded: 连续错误数 >= 3 且 < 10
// - Unhealthy: 连续错误数 >= 10
func updateWASMHealthStatus(health *EngineHealth, config HealthCheckConfig, logger log.Logger) {
	_, _, _, consecutiveErrors, _ := health.ErrorStats.GetStats()

	if consecutiveErrors >= config.UnhealthyThreshold {
		health.Status = EngineHealthUnhealthy
		if config.Enabled {
			health.IsAvailable = false
			if logger != nil {
				logger.Warnf("⚠️ WASM引擎状态：不健康（连续错误数: %d），已标记为不可用", consecutiveErrors)
			}
		} else {
			health.IsAvailable = true
			if logger != nil {
				logger.Warnf("⚠️ WASM引擎状态：不健康（连续错误数: %d），但保持可用（健康检查已禁用）", consecutiveErrors)
			}
		}
	} else if consecutiveErrors >= config.DegradedThreshold {
		health.Status = EngineHealthDegraded
		health.IsAvailable = true // 降级状态仍可用
		if logger != nil {
			logger.Warnf("⚠️ WASM引擎状态：降级（连续错误数: %d）", consecutiveErrors)
		}
	} else {
		health.Status = EngineHealthHealthy
		health.IsAvailable = true
	}

	health.LastCheck = time.Now()
}

// updateONNXHealthStatus 更新ONNX引擎健康状态
//
// 🎯 **健康状态判断**（参考 onnxruntime_go 的错误处理模式）：
// - Healthy: 连续错误数 < DegradedThreshold
// - Degraded: 连续错误数 >= DegradedThreshold 且 < UnhealthyThreshold
// - Unhealthy: 连续错误数 >= UnhealthyThreshold
//
// 📝 **设计原则**（参考 onnxruntime_go）：
// - 错误应该被记录和统计，用于监控和告警
// - 健康检查不应该阻止后续请求的执行
// - 即使引擎标记为不健康，仍然允许执行（通过配置控制）
// - 这样可以避免健康检查机制阻止测试或调试过程
func updateONNXHealthStatus(health *EngineHealth, config HealthCheckConfig, logger log.Logger) {
	_, _, _, consecutiveErrors, _ := health.ErrorStats.GetStats()

	// 根据配置决定是否启用健康检查阻止机制
	// 参考 onnxruntime_go：错误应该被记录，但不应该阻止执行
	if !config.Enabled {
		// 健康检查禁用：仅更新状态，不阻止执行
		if consecutiveErrors >= config.UnhealthyThreshold {
			health.Status = EngineHealthUnhealthy
			health.IsAvailable = true // 保持可用，仅用于监控
			if logger != nil {
				logger.Warnf("⚠️ ONNX引擎状态：不健康（连续错误数: %d），但保持可用（健康检查已禁用）", consecutiveErrors)
			}
		} else if consecutiveErrors >= config.DegradedThreshold {
			health.Status = EngineHealthDegraded
			health.IsAvailable = true
			if logger != nil {
				logger.Warnf("⚠️ ONNX引擎状态：降级（连续错误数: %d）", consecutiveErrors)
			}
		} else {
			health.Status = EngineHealthHealthy
			health.IsAvailable = true
		}
	} else {
		// 健康检查启用：根据错误数决定是否可用
		if consecutiveErrors >= config.UnhealthyThreshold {
			health.Status = EngineHealthUnhealthy
			health.IsAvailable = false // 标记为不可用
			if logger != nil {
				logger.Warnf("⚠️ ONNX引擎状态：不健康（连续错误数: %d），已标记为不可用", consecutiveErrors)
			}
		} else if consecutiveErrors >= config.DegradedThreshold {
			health.Status = EngineHealthDegraded
			health.IsAvailable = true // 降级状态仍可用
			if logger != nil {
				logger.Warnf("⚠️ ONNX引擎状态：降级（连续错误数: %d）", consecutiveErrors)
			}
		} else {
			health.Status = EngineHealthHealthy
			health.IsAvailable = true
		}
	}

	health.LastCheck = time.Now()
}

