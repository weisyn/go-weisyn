package interfaces

import (
	"context"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 引擎管理内部接口 ====================
// 这些接口供execution内部子目录相互调用，不对外暴露

// EngineRegistry 引擎注册表接口
// 由manager包实现，供module和coordinator调用
type EngineRegistry interface {
	// 注册引擎
	Register(engine execution.EngineAdapter) error

	// 取消注册引擎
	Unregister(engineType types.EngineType) error

	// 获取引擎
	GetEngine(engineType types.EngineType) (execution.EngineAdapter, error)

	// 获取所有支持的引擎类型
	GetSupportedEngines() []types.EngineType

	// 检查引擎是否可用
	IsEngineAvailable(engineType types.EngineType) bool

	// 获取引擎健康状态
	GetEngineHealth(engineType types.EngineType) (*EngineHealth, error)

	// 获取引擎统计
	GetEngineStats(engineType types.EngineType) (*EngineStats, error)

	// 获取注册表统计
	GetRegistryStats() *RegistryStats
}

// Dispatcher 执行分发器接口
// 由manager包实现，供coordinator调用
type Dispatcher interface {
	// 分发执行请求
	Dispatch(ctx context.Context, params types.ExecutionParams) (*types.ExecutionResult, error)

	// 批量分发
	BatchDispatch(ctx context.Context, requests []DispatchRequest) ([]*DispatchResult, error)

	// 取消执行
	Cancel(ctx context.Context, executionID string) error

	// 获取执行状态
	GetExecutionStatus(executionID string) (*ExecutionStatus, error)

	// 配置分发策略
	ConfigureStrategy(strategy DispatchStrategy) error

	// 获取负载均衡状态
	GetLoadBalanceStatus() *LoadBalanceStatus

	// 获取分发统计
	GetDispatchStats() *DispatchStats
}

// CircuitBreaker 熔断器接口
// 由manager包实现，供dispatcher调用
type CircuitBreaker interface {
	// 尝试执行
	TryExecute(ctx context.Context, operation func() error) error

	// 获取熔断器状态
	GetState() CircuitBreakerState

	// 手动打开熔断器
	ForceOpen() error

	// 手动关闭熔断器
	ForceClose() error

	// 重置熔断器
	Reset() error

	// 获取熔断统计
	GetStats() *CircuitBreakerStats
}

// RateLimiter 限流器接口
// 由manager包实现，供dispatcher调用
type RateLimiter interface {
	// 检查是否允许执行
	Allow(ctx context.Context, key string) bool

	// 等待执行许可
	Wait(ctx context.Context, key string) error

	// 获取剩余令牌数
	GetTokens(key string) int64

	// 设置限流规则
	SetRateLimit(key string, rule RateLimitRule) error

	// 删除限流规则
	RemoveRateLimit(key string) error

	// 获取限流统计
	GetRateLimitStats() *RateLimitStats
}

// ==================== 数据结构定义 ====================

// EngineHealth 引擎健康状态
type EngineHealth struct {
	Status         HealthStatus           `json:"status"`
	LastCheckTime  time.Time              `json:"last_check_time"`
	Uptime         time.Duration          `json:"uptime"`
	ErrorCount     int64                  `json:"error_count"`
	SuccessCount   int64                  `json:"success_count"`
	AverageLatency time.Duration          `json:"average_latency"`
	ResourceUsage  ResourceUsage          `json:"resource_usage"`
	HealthDetails  map[string]interface{} `json:"health_details"`
}

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	MemoryUsage     uint64  `json:"memory_usage"`
	CPUUsage        float64 `json:"cpu_usage"`
	GoroutineCount  int     `json:"goroutine_count"`
	FileDescriptors int     `json:"file_descriptors"`
}

// EngineStats 引擎统计
type EngineStats struct {
	TotalExecutions             int64            `json:"total_executions"`
	SuccessfulExecutions        int64            `json:"successful_executions"`
	FailedExecutions            int64            `json:"failed_executions"`
	AverageExecutionTime        time.Duration    `json:"average_execution_time"`
	TotalResourceConsumed       uint64           `json:"total_resource_consumed"`
	AverageResourcePerExecution uint64           `json:"average_resource_per_execution"`
	PeakMemoryUsage             uint32           `json:"peak_memory_usage"`
	LastExecutionTime           time.Time        `json:"last_execution_time"`
	ErrorsByType                map[string]int64 `json:"errors_by_type"`
}

// RegistryStats 注册表统计
type RegistryStats struct {
	TotalEngines         int              `json:"total_engines"`
	AvailableEngines     int              `json:"available_engines"`
	EnginesByStatus      map[string]int   `json:"engines_by_status"`
	TotalRegistrations   int64            `json:"total_registrations"`
	LastRegistrationTime time.Time        `json:"last_registration_time"`
	EngineDistribution   map[string]int64 `json:"engine_distribution"`
}

// DispatchRequest 分发请求
type DispatchRequest struct {
	RequestID string                 `json:"request_id"`
	Params    types.ExecutionParams  `json:"params"`
	Priority  int                    `json:"priority"`
	Timeout   time.Duration          `json:"timeout"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// DispatchResult 分发结果
type DispatchResult struct {
	RequestID  string                 `json:"request_id"`
	Result     *types.ExecutionResult `json:"result"`
	Error      error                  `json:"error"`
	Duration   time.Duration          `json:"duration"`
	EngineUsed types.EngineType       `json:"engine_used"`
	Metadata   map[string]interface{} `json:"metadata"`
}

// ExecutionStatus 执行状态
type ExecutionStatus struct {
	ExecutionID   string                 `json:"execution_id"`
	Status        ExecutionStatusType    `json:"status"`
	EngineType    types.EngineType       `json:"engine_type"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       *time.Time             `json:"end_time,omitempty"`
	Progress      float64                `json:"progress"`
	ResourceUsage ResourceUsage          `json:"resource_usage"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// ExecutionStatusType 执行状态类型
type ExecutionStatusType string

const (
	ExecutionStatusPending   ExecutionStatusType = "pending"
	ExecutionStatusRunning   ExecutionStatusType = "running"
	ExecutionStatusCompleted ExecutionStatusType = "completed"
	ExecutionStatusFailed    ExecutionStatusType = "failed"
	ExecutionStatusCancelled ExecutionStatusType = "cancelled"
	ExecutionStatusTimeout   ExecutionStatusType = "timeout"
)

// DispatchStrategy 分发策略
type DispatchStrategy struct {
	Type                 StrategyType           `json:"type"`
	LoadBalancingMode    LoadBalancingMode      `json:"load_balancing_mode"`
	CircuitBreakerConfig CircuitBreakerConfig   `json:"circuit_breaker_config"`
	RateLimitConfig      RateLimitConfig        `json:"rate_limit_config"`
	RetryConfig          RetryConfig            `json:"retry_config"`
	TimeoutConfig        TimeoutConfig          `json:"timeout_config"`
	Metadata             map[string]interface{} `json:"metadata"`
}

// StrategyType 策略类型
type StrategyType string

const (
	StrategyTypeRoundRobin     StrategyType = "round_robin"
	StrategyTypeWeightedRound  StrategyType = "weighted_round"
	StrategyTypeLeastLoad      StrategyType = "least_load"
	StrategyTypeRandom         StrategyType = "random"
	StrategyTypeConsistentHash StrategyType = "consistent_hash"
)

// LoadBalancingMode 负载均衡模式
type LoadBalancingMode string

const (
	LoadBalancingModeStatic   LoadBalancingMode = "static"
	LoadBalancingModeDynamic  LoadBalancingMode = "dynamic"
	LoadBalancingModeAdaptive LoadBalancingMode = "adaptive"
)

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failure_threshold"`
	SuccessThreshold int           `json:"success_threshold"`
	Timeout          time.Duration `json:"timeout"`
	HalfOpenMaxCalls int           `json:"half_open_max_calls"`
	ResetTimeout     time.Duration `json:"reset_timeout"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled         bool                     `json:"enabled"`
	GlobalRateLimit RateLimitRule            `json:"global_rate_limit"`
	PerUserLimit    RateLimitRule            `json:"per_user_limit"`
	PerEngineLimit  map[string]RateLimitRule `json:"per_engine_limit"`
	BurstAllowed    bool                     `json:"burst_allowed"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	Enabled         bool          `json:"enabled"`
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	Multiplier      float64       `json:"multiplier"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	DefaultTimeout  time.Duration            `json:"default_timeout"`
	EngineTimeouts  map[string]time.Duration `json:"engine_timeouts"`
	ContextTimeout  time.Duration            `json:"context_timeout"`
	GracefulTimeout time.Duration            `json:"graceful_timeout"`
}

// LoadBalanceStatus 负载均衡状态
type LoadBalanceStatus struct {
	CurrentStrategy   StrategyType          `json:"current_strategy"`
	EngineLoads       map[string]EngineLoad `json:"engine_loads"`
	TotalRequests     int64                 `json:"total_requests"`
	ActiveConnections int                   `json:"active_connections"`
	LastBalanceTime   time.Time             `json:"last_balance_time"`
	BalanceEfficiency float64               `json:"balance_efficiency"`
}

// EngineLoad 引擎负载
type EngineLoad struct {
	ActiveRequests int           `json:"active_requests"`
	QueueLength    int           `json:"queue_length"`
	CPUUsage       float64       `json:"cpu_usage"`
	MemoryUsage    uint64        `json:"memory_usage"`
	ResponseTime   time.Duration `json:"response_time"`
	SuccessRate    float64       `json:"success_rate"`
	LoadScore      float64       `json:"load_score"`
}

// DispatchStats 分发统计
type DispatchStats struct {
	TotalDispatches      int64            `json:"total_dispatches"`
	SuccessfulDispatches int64            `json:"successful_dispatches"`
	FailedDispatches     int64            `json:"failed_dispatches"`
	AverageDispatchTime  time.Duration    `json:"average_dispatch_time"`
	DispatchesByEngine   map[string]int64 `json:"dispatches_by_engine"`
	DispatchesByStatus   map[string]int64 `json:"dispatches_by_status"`
	CurrentQueueLength   int              `json:"current_queue_length"`
	PeakQueueLength      int              `json:"peak_queue_length"`
	LastDispatchTime     time.Time        `json:"last_dispatch_time"`
}

// CircuitBreakerState 熔断器状态
type CircuitBreakerState string

const (
	CircuitBreakerStateClosed   CircuitBreakerState = "closed"
	CircuitBreakerStateHalfOpen CircuitBreakerState = "half_open"
	CircuitBreakerStateOpen     CircuitBreakerState = "open"
)

// CircuitBreakerStats 熔断器统计
type CircuitBreakerStats struct {
	State                CircuitBreakerState `json:"state"`
	FailureCount         int64               `json:"failure_count"`
	SuccessCount         int64               `json:"success_count"`
	LastFailureTime      time.Time           `json:"last_failure_time"`
	LastSuccessTime      time.Time           `json:"last_success_time"`
	StateChangeCount     int64               `json:"state_change_count"`
	LastStateChange      time.Time           `json:"last_state_change"`
	ConsecutiveFailures  int                 `json:"consecutive_failures"`
	ConsecutiveSuccesses int                 `json:"consecutive_successes"`
}

// RateLimitRule 限流规则
type RateLimitRule struct {
	RequestsPerSecond int           `json:"requests_per_second"`
	BurstSize         int           `json:"burst_size"`
	TimeWindow        time.Duration `json:"time_window"`
	Enabled           bool          `json:"enabled"`
}

// RateLimitStats 限流统计
type RateLimitStats struct {
	TotalRequests      int64                    `json:"total_requests"`
	AllowedRequests    int64                    `json:"allowed_requests"`
	RejectedRequests   int64                    `json:"rejected_requests"`
	RateLimitsByKey    map[string]KeyLimitStats `json:"rate_limits_by_key"`
	LastResetTime      time.Time                `json:"last_reset_time"`
	CurrentWindowStart time.Time                `json:"current_window_start"`
}

// KeyLimitStats 按键限流统计
type KeyLimitStats struct {
	Key              string        `json:"key"`
	RequestsInWindow int64         `json:"requests_in_window"`
	RemainingTokens  int64         `json:"remaining_tokens"`
	LastRequestTime  time.Time     `json:"last_request_time"`
	WindowStartTime  time.Time     `json:"window_start_time"`
	RuleApplied      RateLimitRule `json:"rule_applied"`
}
