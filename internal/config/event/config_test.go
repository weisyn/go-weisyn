package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew 测试配置创建
func TestNew(t *testing.T) {
	t.Run("创建默认配置", func(t *testing.T) {
		config := New(nil)
		assert.NotNil(t, config)
		assert.NotNil(t, config.options)

		// 验证基础配置
		assert.True(t, config.IsEnabled())
		assert.Equal(t, defaultBufferSize, config.GetBufferSize())
		assert.Equal(t, defaultMaxWorkers, config.GetMaxWorkers())
		assert.Equal(t, defaultMaxSubscribers, config.GetMaxSubscribers())

		// 验证增强配置存在
		assert.NotNil(t, config.GetEnhancedOptions())
		assert.Equal(t, defaultEnhancedEnabled, config.IsEnhancedEnabled())
	})
}

// TestEventOptionsDefaults 测试基础事件配置默认值
func TestEventOptionsDefaults(t *testing.T) {
	options := createDefaultEventOptions()
	require.NotNil(t, options)

	t.Run("基础配置默认值", func(t *testing.T) {
		assert.Equal(t, defaultEnabled, options.Enabled)
		assert.Equal(t, defaultBufferSize, options.BufferSize)
		assert.Equal(t, defaultMaxWorkers, options.MaxWorkers)
		assert.Equal(t, defaultMaxSubscribers, options.MaxSubscribers)
	})

	t.Run("增强配置默认值", func(t *testing.T) {
		require.NotNil(t, options.Enhanced)
		assert.Equal(t, defaultEnhancedEnabled, options.Enhanced.Enabled)

		// 验证子配置存在
		assert.NotNil(t, options.Enhanced.DomainRegistry)
		assert.NotNil(t, options.Enhanced.EventRouter)
		assert.NotNil(t, options.Enhanced.EventValidator)
		assert.NotNil(t, options.Enhanced.EventCoordinator)
	})
}

// TestDomainRegistryOptionsDefaults 测试域注册中心配置默认值
func TestDomainRegistryOptionsDefaults(t *testing.T) {
	options := createDefaultDomainRegistryOptions()
	require.NotNil(t, options)

	assert.Equal(t, defaultDomainRegistryEnabled, options.Enabled)
	assert.Equal(t, defaultStrictDomainCheck, options.StrictDomainCheck)
	assert.Equal(t, defaultWarnCrossDomain, options.WarnCrossDomain)
	assert.Equal(t, defaultAllowUnregisteredDomain, options.AllowUnregisteredDomain)
	assert.Equal(t, defaultMaxDomains, options.MaxDomains)
	assert.Equal(t, defaultDomainTTL, options.DefaultTTL)
}

// TestEventRouterOptionsDefaults 测试事件路由器配置默认值
func TestEventRouterOptionsDefaults(t *testing.T) {
	options := createDefaultEventRouterOptions()
	require.NotNil(t, options)

	assert.Equal(t, defaultEventRouterEnabled, options.Enabled)
	assert.Equal(t, defaultRouteStrategy, options.DefaultStrategy)
	assert.Equal(t, defaultMaxConcurrentRoutes, options.MaxConcurrentRoutes)
	assert.Equal(t, defaultRouteTimeout, options.RouteTimeout)
	assert.Equal(t, defaultEnablePriorityQueue, options.EnablePriorityQueue)
	assert.Equal(t, defaultMaxQueueSize, options.MaxQueueSize)
	assert.Equal(t, defaultRouterWorkerPoolSize, options.WorkerPoolSize)
	assert.Equal(t, defaultEnableRouterMetrics, options.EnableMetrics)
}

// TestEventValidatorOptionsDefaults 测试事件验证器配置默认值
func TestEventValidatorOptionsDefaults(t *testing.T) {
	options := createDefaultEventValidatorOptions()
	require.NotNil(t, options)

	assert.Equal(t, defaultEventValidatorEnabled, options.Enabled)
	assert.Equal(t, defaultValidatorStrictMode, options.StrictMode)
	assert.Equal(t, defaultValidateEventName, options.ValidateEventName)
	assert.Equal(t, defaultValidateEventData, options.ValidateEventData)
	assert.Equal(t, defaultValidationTimeout, options.ValidationTimeout)
	assert.Equal(t, defaultMaxValidationRules, options.MaxValidationRules)
	assert.Equal(t, defaultEnableBatchValidation, options.EnableBatchValidation)
	assert.Equal(t, defaultCacheValidationResults, options.CacheValidationResults)
}

// TestEventCoordinatorOptionsDefaults 测试事件协调器配置默认值
func TestEventCoordinatorOptionsDefaults(t *testing.T) {
	options := createDefaultEventCoordinatorOptions()
	require.NotNil(t, options)

	assert.Equal(t, defaultEventCoordinatorEnabled, options.Enabled)
	assert.Equal(t, defaultMaxConcurrentEvents, options.MaxConcurrentEvents)
	assert.Equal(t, defaultEventTimeout, options.EventTimeout)
	assert.Equal(t, defaultHealthCheckInterval, options.HealthCheckInterval)
	assert.Equal(t, defaultMetricsInterval, options.MetricsInterval)
	assert.Equal(t, defaultEnableCircuitBreaker, options.EnableCircuitBreaker)
	assert.Equal(t, defaultCircuitBreakerThreshold, options.CircuitBreakerThreshold)
	assert.Equal(t, defaultEnableGracefulShutdown, options.EnableGracefulShutdown)
	assert.Equal(t, defaultGracefulShutdownTimeout, options.GracefulShutdownTimeout)
}

// TestConfigAccessors 测试配置访问方法
func TestConfigAccessors(t *testing.T) {
	config := New(nil)

	t.Run("基础配置访问方法", func(t *testing.T) {
		assert.True(t, config.IsEnabled())
		assert.Equal(t, defaultBufferSize, config.GetBufferSize())
		assert.Equal(t, defaultMaxWorkers, config.GetMaxWorkers())
		assert.Equal(t, defaultMaxSubscribers, config.GetMaxSubscribers())

		options := config.GetOptions()
		assert.NotNil(t, options)
		assert.Equal(t, defaultEnabled, options.Enabled)
	})

	t.Run("增强配置访问方法", func(t *testing.T) {
		enhancedOptions := config.GetEnhancedOptions()
		assert.NotNil(t, enhancedOptions)
		assert.Equal(t, defaultEnhancedEnabled, config.IsEnhancedEnabled())

		// 域注册中心配置
		domainOptions := config.GetDomainRegistryOptions()
		assert.NotNil(t, domainOptions)
		assert.Equal(t, defaultDomainRegistryEnabled, config.IsDomainRegistryEnabled())
		assert.Equal(t, defaultStrictDomainCheck, domainOptions.StrictDomainCheck)

		// 事件路由器配置
		routerOptions := config.GetEventRouterOptions()
		assert.NotNil(t, routerOptions)
		assert.Equal(t, defaultEventRouterEnabled, config.IsEventRouterEnabled())
		assert.Equal(t, defaultRouteStrategy, routerOptions.DefaultStrategy)

		// 事件验证器配置
		validatorOptions := config.GetEventValidatorOptions()
		assert.NotNil(t, validatorOptions)
		assert.Equal(t, defaultEventValidatorEnabled, config.IsEventValidatorEnabled())
		assert.Equal(t, defaultValidatorStrictMode, validatorOptions.StrictMode)

		// 事件协调器配置
		coordinatorOptions := config.GetEventCoordinatorOptions()
		assert.NotNil(t, coordinatorOptions)
		assert.Equal(t, defaultEventCoordinatorEnabled, config.IsEventCoordinatorEnabled())
		assert.Equal(t, defaultMaxConcurrentEvents, coordinatorOptions.MaxConcurrentEvents)
	})
}

// TestConfigWithNilEnhanced 测试增强配置为nil的情况
func TestConfigWithNilEnhanced(t *testing.T) {
	config := &Config{
		options: &EventOptions{
			Enabled:    true,
			BufferSize: 1000,
			Enhanced:   nil, // 故意设为nil
		},
	}

	t.Run("增强配置为nil时的访问方法", func(t *testing.T) {
		// 基础配置应该正常工作
		assert.True(t, config.IsEnabled())
		assert.Equal(t, 1000, config.GetBufferSize())

		// 增强配置相关方法应该返回false/nil
		assert.Nil(t, config.GetEnhancedOptions())
		assert.False(t, config.IsEnhancedEnabled())

		assert.Nil(t, config.GetDomainRegistryOptions())
		assert.False(t, config.IsDomainRegistryEnabled())

		assert.Nil(t, config.GetEventRouterOptions())
		assert.False(t, config.IsEventRouterEnabled())

		assert.Nil(t, config.GetEventValidatorOptions())
		assert.False(t, config.IsEventValidatorEnabled())

		assert.Nil(t, config.GetEventCoordinatorOptions())
		assert.False(t, config.IsEventCoordinatorEnabled())
	})
}

// TestConfigWithPartialEnhanced 测试部分增强配置的情况
func TestConfigWithPartialEnhanced(t *testing.T) {
	config := &Config{
		options: &EventOptions{
			Enabled:    true,
			BufferSize: 1000,
			Enhanced: &EnhancedEventOptions{
				Enabled:          true,
				DomainRegistry:   nil, // 部分配置为nil
				EventRouter:      createDefaultEventRouterOptions(),
				EventValidator:   nil,
				EventCoordinator: createDefaultEventCoordinatorOptions(),
			},
		},
	}

	t.Run("部分增强配置的访问方法", func(t *testing.T) {
		// 增强功能总开关应该正常
		assert.True(t, config.IsEnhancedEnabled())

		// 有配置的应该返回正确值
		assert.NotNil(t, config.GetEventRouterOptions())
		assert.Equal(t, defaultEventRouterEnabled, config.IsEventRouterEnabled())

		assert.NotNil(t, config.GetEventCoordinatorOptions())
		assert.Equal(t, defaultEventCoordinatorEnabled, config.IsEventCoordinatorEnabled())

		// 没有配置的应该返回nil/false
		assert.Nil(t, config.GetDomainRegistryOptions())
		assert.False(t, config.IsDomainRegistryEnabled())

		assert.Nil(t, config.GetEventValidatorOptions())
		assert.False(t, config.IsEventValidatorEnabled())
	})
}

// TestDefaultValues 测试默认值的合理性
func TestDefaultValues(t *testing.T) {
	t.Run("时间相关默认值", func(t *testing.T) {
		// 验证时间默认值的合理性
		assert.Equal(t, 24*time.Hour, defaultDomainTTL)
		assert.Equal(t, 5*time.Second, defaultRouteTimeout)
		assert.Equal(t, 5*time.Second, defaultValidationTimeout)
		assert.Equal(t, 10*time.Second, defaultEventTimeout)
		assert.Equal(t, 30*time.Second, defaultHealthCheckInterval)
		assert.Equal(t, 10*time.Second, defaultMetricsInterval)
		assert.Equal(t, 30*time.Second, defaultGracefulShutdownTimeout)
	})

	t.Run("数量相关默认值", func(t *testing.T) {
		// 验证数量默认值的合理性
		assert.Equal(t, 1000, defaultBufferSize)
		assert.Equal(t, 10, defaultMaxWorkers)
		assert.Equal(t, 100, defaultMaxSubscribers)
		assert.Equal(t, 100, defaultMaxDomains)
		assert.Equal(t, 10, defaultMaxConcurrentRoutes)
		assert.Equal(t, 1000, defaultMaxQueueSize)
		assert.Equal(t, 5, defaultRouterWorkerPoolSize)
		assert.Equal(t, 100, defaultMaxValidationRules)
		assert.Equal(t, 100, defaultMaxConcurrentEvents)
		assert.Equal(t, 10, defaultCircuitBreakerThreshold)
	})

	t.Run("布尔相关默认值", func(t *testing.T) {
		// 验证布尔默认值的合理性
		assert.True(t, defaultEnabled)
		assert.False(t, defaultEnhancedEnabled)
		assert.False(t, defaultDomainRegistryEnabled)
		assert.False(t, defaultStrictDomainCheck) // 开发友好
		assert.False(t, defaultWarnCrossDomain)
		assert.True(t, defaultAllowUnregisteredDomain) // 向后兼容
		assert.False(t, defaultEventRouterEnabled)
		assert.False(t, defaultEnablePriorityQueue)
		assert.False(t, defaultEnableRouterMetrics)
		assert.False(t, defaultEventValidatorEnabled)
		assert.False(t, defaultValidatorStrictMode) // 开发友好
		assert.True(t, defaultValidateEventName)
		assert.False(t, defaultValidateEventData)
		assert.False(t, defaultEnableBatchValidation)
		assert.False(t, defaultCacheValidationResults)
		assert.False(t, defaultEventCoordinatorEnabled)
		assert.False(t, defaultEnableCircuitBreaker)
		assert.True(t, defaultEnableGracefulShutdown)
	})

	t.Run("字符串相关默认值", func(t *testing.T) {
		assert.Equal(t, "broadcast", defaultRouteStrategy)
	})
}

// TestConfigIntegration 测试配置集成
func TestConfigIntegration(t *testing.T) {
	t.Run("完整配置创建和访问", func(t *testing.T) {
		config := New(nil)

		// 验证完整的配置链
		options := config.GetOptions()
		require.NotNil(t, options)
		require.NotNil(t, options.Enhanced)

		enhanced := options.Enhanced
		require.NotNil(t, enhanced.DomainRegistry)
		require.NotNil(t, enhanced.EventRouter)
		require.NotNil(t, enhanced.EventValidator)
		require.NotNil(t, enhanced.EventCoordinator)

		// 验证配置的一致性
		assert.Equal(t, enhanced.Enabled, config.IsEnhancedEnabled())
		assert.Equal(t, enhanced.DomainRegistry.Enabled, config.IsDomainRegistryEnabled())
		assert.Equal(t, enhanced.EventRouter.Enabled, config.IsEventRouterEnabled())
		assert.Equal(t, enhanced.EventValidator.Enabled, config.IsEventValidatorEnabled())
		assert.Equal(t, enhanced.EventCoordinator.Enabled, config.IsEventCoordinatorEnabled())
	})
}

// BenchmarkConfigCreation 配置创建性能基准测试
func BenchmarkConfigCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		config := New(nil)
		_ = config.GetOptions()
	}
}

// BenchmarkConfigAccess 配置访问性能基准测试
func BenchmarkConfigAccess(b *testing.B) {
	config := New(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.IsEnabled()
		_ = config.IsEnhancedEnabled()
		_ = config.IsDomainRegistryEnabled()
		_ = config.IsEventRouterEnabled()
		_ = config.IsEventValidatorEnabled()
		_ = config.IsEventCoordinatorEnabled()
	}
}
