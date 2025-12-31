package api

import (
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// APIOptions API服务配置选项
// 整个API模块的统一配置入口，包含所有API服务的配置
type APIOptions struct {
	// HTTP API配置
	HTTP HTTPConfig `json:"http"`

	// gRPC API配置
	GRPC GRPCConfig `json:"grpc"`

	// WebSocket配置
	WebSocket WebSocketConfig `json:"websocket"`
}

// HTTPConfig HTTP API配置
type HTTPConfig struct {
	// 基础配置
	Enabled bool   `json:"enabled"` // 是否启用HTTP服务（总开关）
	Host    string `json:"host"`    // 监听地址
	Port    int    `json:"port"`    // 监听端口

	// 协议细粒度开关（v0.0.2+）
	EnableREST      bool `json:"enable_rest"`      // 是否启用REST端点（/api/v1/*）
	EnableJSONRPC   bool `json:"enable_jsonrpc"`   // 是否启用JSON-RPC（/jsonrpc）
	EnableWebSocket bool `json:"enable_websocket"` // 是否启用WebSocket（/ws）

	// 超时配置
	Timeout      time.Duration `json:"timeout"`       // 请求超时时间
	ReadTimeout  time.Duration `json:"read_timeout"`  // 读取超时时间
	WriteTimeout time.Duration `json:"write_timeout"` // 写入超时时间

	// CORS配置
	CORSEnabled bool     `json:"cors_enabled"` // 是否启用CORS
	CORSOrigins []string `json:"cors_origins"` // 允许的CORS源

	// 限流和安全
	RateLimitRequestsPerMinute int `json:"rate_limit_requests_per_minute"` // 每分钟最大请求数
	MaxRequestSize             int `json:"max_request_size"`               // 最大请求大小(字节)
}

// GRPCConfig gRPC API配置
type GRPCConfig struct {
	// 基础配置
	Enabled bool   `json:"enabled"` // 是否启用gRPC
	Host    string `json:"host"`    // 监听地址
	Port    int    `json:"port"`    // 监听端口

	// 消息配置
	MaxMessageSize int `json:"max_message_size"` // 最大消息大小(字节)

	// 连接保活配置
	KeepaliveTime    time.Duration `json:"keepalive_time"`    // 保活时间
	KeepaliveTimeout time.Duration `json:"keepalive_timeout"` // 保活超时
}

// WebSocketConfig WebSocket配置
type WebSocketConfig struct {
	// 基础配置
	Enabled bool   `json:"enabled"` // 是否启用WebSocket
	Host    string `json:"host"`    // 监听地址
	Port    int    `json:"port"`    // 监听端口

	// 连接限制
	MaxConnections int `json:"max_connections"` // 最大连接数

	// 缓冲区配置
	ReadBufferSize  int `json:"read_buffer_size"`  // 读缓冲区大小(字节)
	WriteBufferSize int `json:"write_buffer_size"` // 写缓冲区大小(字节)
}

// Config API配置实现
type Config struct {
	options *APIOptions
}

// New 创建API配置实现
func New(userConfig *types.UserAPIConfig) *Config {
	// 1. 先创建完整的默认配置
	defaultOptions := createDefaultAPIOptions()

	// 2. 如果有用户配置，则转换并覆盖默认配置
	if userConfig != nil {
		convertAndMergeUserConfig(defaultOptions, userConfig)
	}

	return &Config{
		options: defaultOptions,
	}
}

// createDefaultAPIOptions 创建默认API配置
func createDefaultAPIOptions() *APIOptions {
	return &APIOptions{
		HTTP: HTTPConfig{
			Enabled:                    defaultHTTPEnabled,
			Host:                       defaultHTTPHost,
			Port:                       defaultHTTPPort,
			EnableREST:                 defaultHTTPEnableREST,
			EnableJSONRPC:              defaultHTTPEnableJSONRPC,
			EnableWebSocket:            defaultHTTPEnableWebSocket,
			Timeout:                    defaultHTTPTimeout,
			ReadTimeout:                defaultHTTPReadTimeout,
			WriteTimeout:               defaultHTTPWriteTimeout,
			CORSEnabled:                defaultCORSEnabled,
			CORSOrigins:                append([]string{}, defaultCORSOrigins...), // 复制切片
			RateLimitRequestsPerMinute: defaultRateLimitRPM,
			MaxRequestSize:             defaultMaxRequestSize,
		},
		GRPC: GRPCConfig{
			Enabled:          defaultGRPCEnabled,
			Host:             defaultGRPCHost,
			Port:             defaultGRPCPort,
			MaxMessageSize:   defaultGRPCMaxMessageSize,
			KeepaliveTime:    defaultGRPCKeepaliveTime,
			KeepaliveTimeout: defaultGRPCKeepaliveTimeout,
		},
		WebSocket: WebSocketConfig{
			Enabled:         defaultWebSocketEnabled,
			Host:            defaultWebSocketHost,
			Port:            defaultWebSocketPort,
			MaxConnections:  defaultWebSocketMaxConnections,
			ReadBufferSize:  defaultWebSocketReadBufferSize,
			WriteBufferSize: defaultWebSocketWriteBufferSize,
		},
	}
}

// convertAndMergeUserConfig 将用户配置转换并合并到默认配置中
// 使用指针类型来准确区分"未设置"和"设置为零值"
func convertAndMergeUserConfig(defaultOpts *APIOptions, userConfig *types.UserAPIConfig) {
	// === HTTP API配置 ===

	// HTTPEnabled: 指针类型，用户未设置时为nil，设置为false时为&false
	if userConfig.HTTPEnabled != nil {
		// 用户明确设置了HTTP API开关（无论true还是false都是用户的明确意图）
		defaultOpts.HTTP.Enabled = *userConfig.HTTPEnabled
	}
	// 如果userConfig.HTTPEnabled == nil，表示用户未设置，保持默认值

	// HTTPHost字段不在JSON配置中暴露，使用defaults.go中的默认值

	// HTTPPort: 指针类型，用户未设置时为nil，设置为0时为&0
	if userConfig.HTTPPort != nil {
		// 用户明确设置了HTTP端口（即使设置为0，也可能是用户想要禁用HTTP的意图）
		defaultOpts.HTTP.Port = *userConfig.HTTPPort
	}
	// 如果userConfig.HTTPPort == nil，表示用户未设置，保持默认值

	// 协议细粒度开关（v0.0.2+）
	if userConfig.HTTPEnableREST != nil {
		defaultOpts.HTTP.EnableREST = *userConfig.HTTPEnableREST
	}
	if userConfig.HTTPEnableJSONRPC != nil {
		defaultOpts.HTTP.EnableJSONRPC = *userConfig.HTTPEnableJSONRPC
	}
	if userConfig.HTTPEnableWebSocket != nil {
		defaultOpts.HTTP.EnableWebSocket = *userConfig.HTTPEnableWebSocket
	}

	// 兼容性处理：websocket_enabled 映射到 http_enable_websocket
	if userConfig.WebSocketEnabled != nil && userConfig.HTTPEnableWebSocket == nil {
		defaultOpts.HTTP.EnableWebSocket = *userConfig.WebSocketEnabled
	}

	// CORS 配置
	if userConfig.HTTPCorsEnabled != nil {
		defaultOpts.HTTP.CORSEnabled = *userConfig.HTTPCorsEnabled
	}
	if len(userConfig.HTTPCorsOrigins) > 0 {
		defaultOpts.HTTP.CORSOrigins = userConfig.HTTPCorsOrigins
	}

	// === gRPC API配置 ===

	// GRPCEnabled: 指针类型，用户未设置时为nil，设置为false时为&false
	if userConfig.GRPCEnabled != nil {
		// 用户明确设置了gRPC API开关
		defaultOpts.GRPC.Enabled = *userConfig.GRPCEnabled
	}
	// 如果userConfig.GRPCEnabled == nil，表示用户未设置，保持默认值

	// GRPCHost字段不在JSON配置中暴露，使用defaults.go中的默认值

	// GRPCPort: 指针类型，用户未设置时为nil，设置为0时为&0
	if userConfig.GRPCPort != nil {
		// 用户明确设置了gRPC端口
		defaultOpts.GRPC.Port = *userConfig.GRPCPort
	}
	// 如果userConfig.GRPCPort == nil，表示用户未设置，保持默认值

	// === WebSocket配置 ===

	// WebSocketEnabled: 指针类型，用户未设置时为nil，设置为false时为&false
	if userConfig.WebSocketEnabled != nil {
		// 用户明确设置了WebSocket开关
		defaultOpts.WebSocket.Enabled = *userConfig.WebSocketEnabled
	}
	// 如果userConfig.WebSocketEnabled == nil，表示用户未设置，保持默认值

	// WebSocketHost字段不在JSON配置中暴露，使用defaults.go中的默认值

	// WebSocketPort: 指针类型，用户未设置时为nil，设置为0时为&0
	if userConfig.WebSocketPort != nil {
		// 用户明确设置了WebSocket端口
		defaultOpts.WebSocket.Port = *userConfig.WebSocketPort
	}
	// 如果userConfig.WebSocketPort == nil，表示用户未设置，保持默认值

	// === 安全和限流配置使用默认值 ===
	// EnableMiningAPI字段处理
	if userConfig.EnableMiningAPI != nil {
		// 暂时记录，待API层支持挖矿API功能时启用
	}
}

// GetOptions 获取完整的API配置选项
func (c *Config) GetOptions() *APIOptions {
	return c.options
}

// GetHTTPConfig 获取HTTP配置
func (c *Config) GetHTTPConfig() *HTTPConfig {
	return &c.options.HTTP
}

// GetGRPCConfig 获取gRPC配置
func (c *Config) GetGRPCConfig() *GRPCConfig {
	return &c.options.GRPC
}

// GetWebSocketConfig 获取WebSocket配置
func (c *Config) GetWebSocketConfig() *WebSocketConfig {
	return &c.options.WebSocket
}
