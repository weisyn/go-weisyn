package client

import (
	"context"
	"fmt"
	"time"

	blockchainintf "github.com/weisyn/v1/pkg/interfaces/blockchain"
	consensusintf "github.com/weisyn/v1/pkg/interfaces/consensus"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ClientConfig 客户端配置
type ClientConfig struct {
	// API配置
	APIBaseURL string
	APITimeout time.Duration

	// 直接调用配置
	EnableDirectCall bool

	// 调用策略配置
	DefaultMode     CallMode
	MaxRetries      int
	RetryDelay      time.Duration
	FallbackEnabled bool

	// 性能配置
	PerformanceThreshold time.Duration
	StatsEnabled         bool
}

// ClientFactory 客户端工厂
type ClientFactory struct {
	logger         log.Logger
	config         ClientConfig
	accountService blockchainintf.AccountService
	minerService   consensusintf.MinerService
}

// NewClientFactory 创建客户端工厂
func NewClientFactory(
	logger log.Logger,
	config ClientConfig,
	accountService blockchainintf.AccountService,
	minerService consensusintf.MinerService,
) *ClientFactory {
	return &ClientFactory{
		logger:         logger,
		config:         config,
		accountService: accountService,
		minerService:   minerService,
	}
}

// CreateDualCallClient 创建双重调用客户端
func (cf *ClientFactory) CreateDualCallClient() (DualCallClient, error) {
	cf.logger.Info("创建双重调用客户端")

	// 创建直接调用执行器
	var directExec DirectCallExecutor
	if cf.config.EnableDirectCall && cf.accountService != nil {
		directExec = NewDirectCallExecutor(cf.logger, cf.accountService, cf.minerService)
		cf.logger.Info("直接调用执行器已创建")
	} else {
		directExec = NewMockDirectCallExecutor(cf.logger, false) // 不可用的mock
		cf.logger.Info("直接调用执行器不可用，使用mock实现")
	}

	// 创建API调用执行器
	apiExec := NewAPICallExecutor(cf.logger, cf.config.APIBaseURL)
	cf.logger.Info(fmt.Sprintf("API调用执行器已创建: baseURL=%s", cf.config.APIBaseURL))

	// 创建双重调用客户端
	dualClient := NewDualCallClient(cf.logger, directExec, apiExec)

	// 配置调用策略
	strategy := cf.createCallStrategy()
	dualClient.SetStrategy(strategy)

	cf.logger.Info("双重调用客户端创建完成")
	return dualClient, nil
}

// createCallStrategy 创建调用策略
func (cf *ClientFactory) createCallStrategy() CallStrategy {
	return CallStrategy{
		DefaultMode:          cf.config.DefaultMode,
		DirectCallTimeout:    30 * time.Second,
		APICallTimeout:       cf.config.APITimeout,
		MaxRetries:           cf.config.MaxRetries,
		RetryDelay:           cf.config.RetryDelay,
		PerformanceThreshold: cf.config.PerformanceThreshold,
		FallbackEnabled:      cf.config.FallbackEnabled,
		FallbackMode:         APICall, // 默认降级到API调用
	}
}

// GetDefaultConfig 获取默认配置
func GetDefaultConfig() ClientConfig {
	return ClientConfig{
		APIBaseURL:           "http://localhost:8080",
		APITimeout:           60 * time.Second,
		EnableDirectCall:     true,
		DefaultMode:          AutoCall,
		MaxRetries:           2,
		RetryDelay:           1 * time.Second,
		FallbackEnabled:      true,
		PerformanceThreshold: 5 * time.Second,
		StatsEnabled:         true,
	}
}

// UpdateConfig 更新配置
func (cf *ClientFactory) UpdateConfig(config ClientConfig) {
	cf.config = config
	cf.logger.Info("客户端工厂配置已更新")
}

// GetConfig 获取当前配置
func (cf *ClientFactory) GetConfig() ClientConfig {
	return cf.config
}

// TestConnectivity 测试连接性
func (cf *ClientFactory) TestConnectivity(ctx context.Context) (*ConnectivityResult, error) {
	cf.logger.Info("开始连接性测试")

	result := &ConnectivityResult{
		TestTime: time.Now(),
	}

	// 测试直接调用
	if cf.config.EnableDirectCall {
		directExec := NewDirectCallExecutor(cf.logger, cf.accountService, cf.minerService)

		startTime := time.Now()
		_, err := directExec.Execute(ctx, "health_check", map[string]interface{}{})
		duration := time.Since(startTime)

		result.DirectCall = &CallTestResult{
			Available: err == nil,
			Latency:   duration,
			Error:     err,
		}

		if err == nil {
			cf.logger.Info(fmt.Sprintf("直接调用测试成功: latency=%v", duration))
		} else {
			cf.logger.Info(fmt.Sprintf("直接调用测试失败: error=%v", err))
		}
	} else {
		result.DirectCall = &CallTestResult{
			Available: false,
			Error:     fmt.Errorf("直接调用被禁用"),
		}
	}

	// 测试API调用
	apiExec := NewAPICallExecutor(cf.logger, cf.config.APIBaseURL)

	startTime := time.Now()
	_, err := apiExec.Execute(ctx, "health_check", map[string]interface{}{})
	duration := time.Since(startTime)

	result.APICall = &CallTestResult{
		Available: err == nil,
		Latency:   duration,
		Error:     err,
	}

	if err == nil {
		cf.logger.Info(fmt.Sprintf("API调用测试成功: latency=%v", duration))
	} else {
		cf.logger.Info(fmt.Sprintf("API调用测试失败: error=%v", err))
	}

	// 计算推荐模式
	result.RecommendedMode = cf.calculateRecommendedMode(result)

	cf.logger.Info(fmt.Sprintf("连接性测试完成: recommended_mode=%s", result.RecommendedMode.String()))
	return result, nil
}

// calculateRecommendedMode 计算推荐的调用模式
func (cf *ClientFactory) calculateRecommendedMode(result *ConnectivityResult) CallMode {
	if result.DirectCall.Available && result.APICall.Available {
		// 两种都可用，比较性能
		if result.DirectCall.Latency > 0 && result.APICall.Latency > 0 {
			if result.DirectCall.Latency < result.APICall.Latency*2/3 {
				return DirectCall // 直接调用明显更快
			}
		}
		return AutoCall // 自动选择
	}

	if result.DirectCall.Available {
		return DirectCall
	}

	if result.APICall.Available {
		return APICall
	}

	return AutoCall // 都不可用时，让系统自己处理
}

// ConnectivityResult 连接性测试结果
type ConnectivityResult struct {
	TestTime        time.Time       `json:"test_time"`
	DirectCall      *CallTestResult `json:"direct_call"`
	APICall         *CallTestResult `json:"api_call"`
	RecommendedMode CallMode        `json:"recommended_mode"`
}

// CallTestResult 单次调用测试结果
type CallTestResult struct {
	Available bool          `json:"available"`
	Latency   time.Duration `json:"latency"`
	Error     error         `json:"error,omitempty"`
}

// MockDirectCallExecutor mock直接调用执行器（用于测试）
type mockDirectCallExecutor struct {
	logger    log.Logger
	available bool
	latency   time.Duration
}

// NewMockDirectCallExecutor 创建mock直接调用执行器
func NewMockDirectCallExecutor(logger log.Logger, available bool) DirectCallExecutor {
	return &mockDirectCallExecutor{
		logger:    logger,
		available: available,
		latency:   100 * time.Millisecond,
	}
}

// Execute 执行mock调用
func (m *mockDirectCallExecutor) Execute(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	if !m.available {
		return nil, fmt.Errorf("mock执行器不可用")
	}

	// 模拟延迟
	time.Sleep(m.latency)

	switch operation {
	case "health_check":
		return map[string]interface{}{
			"status": "healthy",
			"mock":   true,
		}, nil
	default:
		return map[string]interface{}{
			"mock":      true,
			"operation": operation,
		}, nil
	}
}

// IsAvailable 检查是否可用
func (m *mockDirectCallExecutor) IsAvailable() bool {
	return m.available
}

// GetLatency 获取延迟
func (m *mockDirectCallExecutor) GetLatency() time.Duration {
	return m.latency
}

// SetAvailable 设置可用状态（用于测试）
func (m *mockDirectCallExecutor) SetAvailable(available bool) {
	m.available = available
}

// SetLatency 设置延迟（用于测试）
func (m *mockDirectCallExecutor) SetLatency(latency time.Duration) {
	m.latency = latency
}
