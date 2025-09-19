package interfaces

import (
	"context"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/types"
)

// ==================== 宿主能力内部接口 ====================
// 这些接口供execution内部子目录相互调用，不对外暴露

// CapabilityProvider 能力提供者接口
// 由host包实现，供coordinator和manager调用
type CapabilityProvider interface {
	// 注册宿主能力
	RegisterCapability(name string, capability HostCapability) error

	// 取消注册宿主能力
	UnregisterCapability(name string) error

	// 获取宿主能力
	GetCapability(name string) (HostCapability, error)

	// 获取所有可用能力
	GetAvailableCapabilities() []string

	// 检查能力是否可用
	IsCapabilityAvailable(name string) bool

	// 获取能力统计
	GetCapabilityStats(name string) (*CapabilityStats, error)

	// 获取提供者统计
	GetProviderStats() *ProviderStats
}

// HostBinding 宿主绑定接口
// 由host包实现，供coordinator调用
type HostBinding interface {
	// 绑定到引擎
	BindToEngine(engine execution.EngineAdapter) error

	// 解绑引擎
	UnbindEngine(engineType types.EngineType) error

	// 创建执行上下文
	CreateExecutionContext(params types.ExecutionParams) (*ExecutionContext, error)

	// 销毁执行上下文
	DestroyExecutionContext(contextID string) error

	// 获取绑定状态
	GetBindingStatus(engineType types.EngineType) (*BindingStatus, error)

	// 获取绑定统计
	GetBindingStats() *BindingStats
}

// HostCapability 宿主能力接口
// 具体能力的通用接口
type HostCapability interface {
	// 获取能力名称
	GetName() string

	// 获取能力描述
	GetDescription() string

	// 获取能力版本
	GetVersion() string

	// 检查能力是否可用
	IsAvailable() bool

	// 调用能力
	Invoke(ctx context.Context, params CapabilityParams) (*CapabilityResult, error)

	// 获取能力配置
	GetConfiguration() *CapabilityConfiguration

	// 设置能力配置
	SetConfiguration(config *CapabilityConfiguration) error
}

// BlockchainCapability 区块链能力接口
// 由host包实现，提供区块链相关功能
type BlockchainCapability interface {
	HostCapability

	// 获取当前区块信息
	GetCurrentBlock(ctx context.Context) (*BlockInfo, error)

	// 获取指定区块信息
	GetBlock(ctx context.Context, blockHash []byte) (*BlockInfo, error)

	// 获取交易信息
	GetTransaction(ctx context.Context, txHash []byte) (*TransactionInfo, error)

	// 获取账户余额
	GetAccountBalance(ctx context.Context, address string) (*BalanceInfo, error)

	// 验证交易
	ValidateTransaction(ctx context.Context, tx interface{}) (*ValidationResult, error)
}

// StorageCapability 存储能力接口
// 由host包实现，提供数据存储功能
type StorageCapability interface {
	HostCapability

	// 读取数据
	Read(ctx context.Context, key []byte) ([]byte, error)

	// 写入数据
	Write(ctx context.Context, key []byte, value []byte) error

	// 删除数据
	Delete(ctx context.Context, key []byte) error

	// 检查键是否存在
	Exists(ctx context.Context, key []byte) (bool, error)

	// 遍历键值对
	Iterate(ctx context.Context, prefix []byte, callback func(key, value []byte) error) error
}

// CryptoCapability 加密能力接口
// 由host包实现，提供密码学功能
type CryptoCapability interface {
	HostCapability

	// 计算哈希
	Hash(ctx context.Context, data []byte, algorithm string) ([]byte, error)

	// 验证签名
	VerifySignature(ctx context.Context, data, signature, publicKey []byte) (bool, error)

	// 生成随机数
	GenerateRandom(ctx context.Context, length int) ([]byte, error)

	// 加密数据
	Encrypt(ctx context.Context, data, key []byte) ([]byte, error)

	// 解密数据
	Decrypt(ctx context.Context, encrypted, key []byte) ([]byte, error)
}

// LoggingCapability 日志能力接口
// 由host包实现，提供日志记录功能
type LoggingCapability interface {
	HostCapability

	// 记录信息日志
	Info(ctx context.Context, message string, fields map[string]interface{}) error

	// 记录错误日志
	Error(ctx context.Context, message string, err error, fields map[string]interface{}) error

	// 记录调试日志
	Debug(ctx context.Context, message string, fields map[string]interface{}) error

	// 记录警告日志
	Warn(ctx context.Context, message string, fields map[string]interface{}) error
}

// EventCapability 事件能力接口
// 由host包实现，提供事件发布功能
type EventCapability interface {
	HostCapability

	// 发布事件
	PublishEvent(ctx context.Context, event *HostEvent) error

	// 订阅事件
	SubscribeEvent(ctx context.Context, eventType string, callback EventCallback) error

	// 取消订阅
	UnsubscribeEvent(ctx context.Context, eventType string) error

	// 获取事件历史
	GetEventHistory(ctx context.Context, filter EventFilter) ([]*HostEvent, error)
}

// ==================== 数据结构定义 ====================

// CapabilityStats 能力统计
type CapabilityStats struct {
	Name                  string           `json:"name"`
	TotalInvocations      int64            `json:"total_invocations"`
	SuccessfulInvocations int64            `json:"successful_invocations"`
	FailedInvocations     int64            `json:"failed_invocations"`
	AverageLatency        time.Duration    `json:"average_latency"`
	LastInvocationTime    time.Time        `json:"last_invocation_time"`
	ErrorsByType          map[string]int64 `json:"errors_by_type"`
	ResourceUsage         ResourceUsage    `json:"resource_usage"`
}

// ProviderStats 提供者统计
type ProviderStats struct {
	TotalCapabilities     int                        `json:"total_capabilities"`
	AvailableCapabilities int                        `json:"available_capabilities"`
	CapabilityStats       map[string]CapabilityStats `json:"capability_stats"`
	TotalInvocations      int64                      `json:"total_invocations"`
	LastRegistrationTime  time.Time                  `json:"last_registration_time"`
}

// ExecutionContext 执行上下文
type ExecutionContext struct {
	ContextID      string                 `json:"context_id"`
	EngineType     types.EngineType       `json:"engine_type"`
	CreatedAt      time.Time              `json:"created_at"`
	ExpiresAt      time.Time              `json:"expires_at"`
	Capabilities   []string               `json:"capabilities"`
	Permissions    []Permission           `json:"permissions"`
	ResourceLimits ResourceLimits         `json:"resource_limits"`
	Metadata       map[string]interface{} `json:"metadata"`
}

// Permission 权限
type Permission struct {
	Type       PermissionType `json:"type"`
	Resource   string         `json:"resource"`
	Action     string         `json:"action"`
	Conditions []string       `json:"conditions"`
	Granted    bool           `json:"granted"`
	GrantedAt  time.Time      `json:"granted_at"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
}

// PermissionType 权限类型
type PermissionType string

const (
	PermissionTypeRead    PermissionType = "read"
	PermissionTypeWrite   PermissionType = "write"
	PermissionTypeExecute PermissionType = "execute"
	PermissionTypeAdmin   PermissionType = "admin"
)

// ResourceLimits 资源限制
type ResourceLimits struct {
	MaxMemory    uint64        `json:"max_memory"`
	MaxCPUTime   time.Duration `json:"max_cpu_time"`
	MaxResource  uint64        `json:"max_resource"`
	MaxFileSize  uint64        `json:"max_file_size"`
	MaxNetworkIO uint64        `json:"max_network_io"`
	MaxStorageIO uint64        `json:"max_storage_io"`
}

// BindingStatus 绑定状态
type BindingStatus struct {
	EngineType        types.EngineType       `json:"engine_type"`
	IsBound           bool                   `json:"is_bound"`
	BoundAt           time.Time              `json:"bound_at"`
	LastHeartbeat     time.Time              `json:"last_heartbeat"`
	BoundCapabilities []string               `json:"bound_capabilities"`
	Status            BindingStatusType      `json:"status"`
	ErrorMessage      string                 `json:"error_message,omitempty"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// BindingStatusType 绑定状态类型
type BindingStatusType string

const (
	BindingStatusActive      BindingStatusType = "active"
	BindingStatusInactive    BindingStatusType = "inactive"
	BindingStatusError       BindingStatusType = "error"
	BindingStatusMaintenance BindingStatusType = "maintenance"
)

// BindingStats 绑定统计
type BindingStats struct {
	TotalBindings        int                      `json:"total_bindings"`
	ActiveBindings       int                      `json:"active_bindings"`
	BindingsByEngine     map[string]int           `json:"bindings_by_engine"`
	BindingsByStatus     map[string]int           `json:"bindings_by_status"`
	AverageBindingTime   time.Duration            `json:"average_binding_time"`
	LastBindingTime      time.Time                `json:"last_binding_time"`
	BindingStatusDetails map[string]BindingStatus `json:"binding_status_details"`
}

// CapabilityParams 能力参数
type CapabilityParams struct {
	Method     string                 `json:"method"`
	Arguments  []interface{}          `json:"arguments"`
	Context    map[string]interface{} `json:"context"`
	Options    map[string]interface{} `json:"options"`
	Timeout    time.Duration          `json:"timeout"`
	RetryCount int                    `json:"retry_count"`
}

// CapabilityResult 能力结果
type CapabilityResult struct {
	Success       bool                   `json:"success"`
	Result        interface{}            `json:"result"`
	Error         error                  `json:"error"`
	Metadata      map[string]interface{} `json:"metadata"`
	Duration      time.Duration          `json:"duration"`
	UsedResources ResourceUsage          `json:"used_resources"`
}

// CapabilityConfiguration 能力配置
type CapabilityConfiguration struct {
	Enabled        bool                   `json:"enabled"`
	MaxConcurrency int                    `json:"max_concurrency"`
	Timeout        time.Duration          `json:"timeout"`
	RetryPolicy    RetryPolicy            `json:"retry_policy"`
	ResourceLimits ResourceLimits         `json:"resource_limits"`
	CachePolicy    CachePolicy            `json:"cache_policy"`
	SecurityPolicy SecurityPolicy         `json:"security_policy"`
	CustomSettings map[string]interface{} `json:"custom_settings"`
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	Enabled      bool          `json:"enabled"`
	MaxRetries   int           `json:"max_retries"`
	InitialDelay time.Duration `json:"initial_delay"`
	MaxDelay     time.Duration `json:"max_delay"`
	Multiplier   float64       `json:"multiplier"`
	Jitter       bool          `json:"jitter"`
}

// CachePolicy 缓存策略
type CachePolicy struct {
	Enabled  bool          `json:"enabled"`
	TTL      time.Duration `json:"ttl"`
	MaxSize  int           `json:"max_size"`
	Strategy string        `json:"strategy"`
	CacheKey string        `json:"cache_key"`
}

// SecurityPolicy 安全策略
type SecurityPolicy struct {
	RequiredPermissions []Permission `json:"required_permissions"`
	AllowedCallers      []string     `json:"allowed_callers"`
	RateLimits          []RateLimit  `json:"rate_limits"`
	IPWhitelist         []string     `json:"ip_whitelist"`
	IPBlacklist         []string     `json:"ip_blacklist"`
}

// RateLimit 速率限制
type RateLimit struct {
	Type           string        `json:"type"`
	RequestsPerSec int           `json:"requests_per_sec"`
	BurstSize      int           `json:"burst_size"`
	TimeWindow     time.Duration `json:"time_window"`
}

// BlockInfo 区块信息
type BlockInfo struct {
	Hash              []byte    `json:"hash"`
	Height            uint64    `json:"height"`
	Timestamp         time.Time `json:"timestamp"`
	ParentHash        []byte    `json:"parent_hash"`
	StateRoot         []byte    `json:"state_root"`
	TxRoot            []byte    `json:"tx_root"`
	Transactions      [][]byte  `json:"transactions"`
	Size              uint64    `json:"size"`
	ResourceUsed      uint64    `json:"resource_used"`
	ExecutionFeeLimit uint64    `json:"execution_fee_limit"`
}

// TransactionInfo 交易信息
type TransactionInfo struct {
	Hash              []byte    `json:"hash"`
	BlockHash         []byte    `json:"block_hash"`
	BlockHeight       uint64    `json:"block_height"`
	Index             uint32    `json:"index"`
	From              string    `json:"from"`
	To                string    `json:"to"`
	Value             uint64    `json:"value"`
	ExecutionFeeLimit uint64    `json:"execution_fee_limit"`
	ResourceUsed      uint64    `json:"resource_used"`
	ResourcePrice     uint64    `json:"resource_price"`
	Nonce             uint64    `json:"nonce"`
	Data              []byte    `json:"data"`
	Status            TxStatus  `json:"status"`
	Timestamp         time.Time `json:"timestamp"`
}

// TxStatus 交易状态
type TxStatus string

const (
	TxStatusPending   TxStatus = "pending"
	TxStatusConfirmed TxStatus = "confirmed"
	TxStatusFailed    TxStatus = "failed"
)

// BalanceInfo 余额信息
type BalanceInfo struct {
	Address     string                 `json:"address"`
	Balances    map[string]uint64      `json:"balances"`
	Nonce       uint64                 `json:"nonce"`
	LastUpdated time.Time              `json:"last_updated"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// ValidationResult 验证结果
type ValidationResult struct {
	Valid            bool     `json:"valid"`
	Errors           []string `json:"errors"`
	Warnings         []string `json:"warnings"`
	ResourceEstimate uint64   `json:"resource_estimate"`
	Status           string   `json:"status"`
}

// HostEvent 宿主事件
type HostEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Tags      []string               `json:"tags"`
	Priority  EventPriority          `json:"priority"`
	TTL       time.Duration          `json:"ttl"`
}

// EventPriority 事件优先级
type EventPriority string

const (
	EventPriorityLow      EventPriority = "low"
	EventPriorityNormal   EventPriority = "normal"
	EventPriorityHigh     EventPriority = "high"
	EventPriorityCritical EventPriority = "critical"
)

// EventCallback 事件回调
type EventCallback func(event *HostEvent) error

// EventFilter 事件过滤器
type EventFilter struct {
	EventTypes []string       `json:"event_types"`
	Sources    []string       `json:"sources"`
	StartTime  *time.Time     `json:"start_time"`
	EndTime    *time.Time     `json:"end_time"`
	Tags       []string       `json:"tags"`
	Priority   *EventPriority `json:"priority"`
	Limit      int            `json:"limit"`
	Offset     int            `json:"offset"`
}
