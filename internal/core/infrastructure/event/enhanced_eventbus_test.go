package event

import (
	"context"
	"testing"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// enhancedTestEvent 增强测试事件实现
type enhancedTestEvent struct {
	eventType event.EventType
	data      interface{}
}

func (e *enhancedTestEvent) Type() event.EventType {
	return e.eventType
}

func (e *enhancedTestEvent) Data() interface{} {
	return e.data
}

// enhancedTestEventHandler 增强测试事件处理器
func enhancedTestEventHandler(e event.Event) error {
	return nil
}

// setupEnhancedEventBus 创建测试用的增强事件总线
func setupEnhancedEventBus(t testing.TB) *EnhancedEventBus {
	logger := &mockLogger{}
	config := DefaultEnhancedEventBusConfig()

	enhanced, err := NewEnhanced(logger, config)
	require.NoError(t, err)
	require.NotNil(t, enhanced)

	return enhanced
}

func TestNewEnhanced(t *testing.T) {
	t.Run("创建增强事件总线", func(t *testing.T) {
		enhanced := setupEnhancedEventBus(t)

		assert.NotNil(t, enhanced)
		assert.NotNil(t, enhanced.EventBus)
		assert.NotNil(t, enhanced.coordinator)
		assert.NotNil(t, enhanced.domainRegistry)
		assert.NotNil(t, enhanced.eventRouter)
		assert.NotNil(t, enhanced.eventValidator)
		assert.False(t, enhanced.IsStarted())
	})

	t.Run("使用默认配置", func(t *testing.T) {
		logger := &mockLogger{}
		enhanced, err := NewEnhanced(logger, nil)

		assert.NoError(t, err)
		assert.NotNil(t, enhanced)
		assert.NotNil(t, enhanced.enhancedConfig)
		assert.True(t, enhanced.enhancedConfig.EnableDomainRegistry)
		assert.True(t, enhanced.enhancedConfig.EnableSmartRouting)
		assert.True(t, enhanced.enhancedConfig.EnableEventValidation)
	})
}

func TestEnhancedEventBus_Lifecycle(t *testing.T) {
	enhanced := setupEnhancedEventBus(t)

	t.Run("启动增强事件总线", func(t *testing.T) {
		ctx := context.Background()
		err := enhanced.Start(ctx)
		assert.NoError(t, err)
		assert.True(t, enhanced.IsStarted())
	})

	t.Run("重复启动应该失败", func(t *testing.T) {
		ctx := context.Background()
		err := enhanced.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already started")
	})

	t.Run("停止增强事件总线", func(t *testing.T) {
		ctx := context.Background()
		err := enhanced.Stop(ctx)
		assert.NoError(t, err)
		assert.False(t, enhanced.IsStarted())
	})

	t.Run("重复停止应该失败", func(t *testing.T) {
		ctx := context.Background()
		err := enhanced.Stop(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not started")
	})
}

func TestEnhancedEventBus_EventPublishing(t *testing.T) {
	enhanced := setupEnhancedEventBus(t)
	ctx := context.Background()
	enhanced.Start(ctx)
	defer enhanced.Stop(ctx)

	t.Run("发布Event接口事件", func(t *testing.T) {
		event := &enhancedTestEvent{
			eventType: "blockchain.block.produced",
			data:      map[string]interface{}{"height": 123},
		}

		// 这个方法不返回错误，只是记录日志
		enhanced.PublishEvent(event)

		// 验证统计信息更新
		stats := enhanced.GetStatistics()
		assert.NotNil(t, stats)
	})

	t.Run("带优先级发布事件", func(t *testing.T) {
		err := enhanced.PublishEventWithPriority("mempool.tx.added", "transaction", PriorityHigh)
		assert.NoError(t, err)
	})

	t.Run("带元数据发布事件", func(t *testing.T) {
		metadata, err := NewEventMetadata("consensus.round.completed", "test-source")
		require.NoError(t, err)

		err = enhanced.PublishEventWithMetadata(metadata, "consensus data")
		assert.NoError(t, err)
	})

	t.Run("批量发布事件", func(t *testing.T) {
		events := []EventRequest{
			{EventType: "blockchain.block.produced", Data: "data1", Priority: PriorityNormal},
			{EventType: "mempool.tx.added", Data: "data2", Priority: PriorityHigh},
		}

		results := enhanced.BatchPublishEvents(events)
		assert.Len(t, results, 2)

		for _, result := range results {
			assert.True(t, result.Success, "Event should be published successfully")
			assert.Empty(t, result.Error)
		}
	})

	t.Run("未启动时发布应该失败", func(t *testing.T) {
		enhanced.Stop(ctx)

		err := enhanced.PublishEventWithPriority("test.event", "data", PriorityNormal)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not started")
	})
}

func TestEnhancedEventBus_EventSubscription(t *testing.T) {
	enhanced := setupEnhancedEventBus(t)
	ctx := context.Background()
	enhanced.Start(ctx)
	defer enhanced.Stop(ctx)

	t.Run("订阅事件", func(t *testing.T) {
		subID, err := enhanced.SubscribeEvent("blockchain.block.produced", enhancedTestEventHandler)
		assert.NoError(t, err)
		assert.NotEmpty(t, subID)
	})

	t.Run("带选项订阅事件", func(t *testing.T) {
		subID, err := enhanced.SubscribeEventWithOptions("mempool.tx.added", enhancedTestEventHandler)
		assert.NoError(t, err)
		assert.NotEmpty(t, subID)
	})

	t.Run("订阅域事件", func(t *testing.T) {
		// 先注册域
		domainInfo := DomainInfo{
			Component:   "test-component",
			Description: "Test domain",
		}
		err := enhanced.RegisterDomain("testdomain", domainInfo)
		require.NoError(t, err)

		// 订阅域事件
		handler := func(eventType string, data interface{}) error {
			return nil
		}

		subID, err := enhanced.SubscribeDomainEvents("testdomain", handler)
		assert.NoError(t, err)
		assert.NotEmpty(t, subID)
	})

	t.Run("取消订阅", func(t *testing.T) {
		subID, err := enhanced.SubscribeEvent("test.unsubscribe.event", enhancedTestEventHandler)
		require.NoError(t, err)

		err = enhanced.UnsubscribeEvent(subID)
		assert.NoError(t, err)
	})
}

func TestEnhancedEventBus_DomainManagement(t *testing.T) {
	enhanced := setupEnhancedEventBus(t)

	t.Run("注册域", func(t *testing.T) {
		domainInfo := DomainInfo{
			Component:   "test-component",
			Description: "Test domain for unit tests",
		}

		err := enhanced.RegisterDomain("testdomain", domainInfo)
		assert.NoError(t, err)
		assert.True(t, enhanced.IsDomainRegistered("testdomain"))
	})

	t.Run("列出域", func(t *testing.T) {
		domains := enhanced.ListDomains()
		assert.Contains(t, domains, "testdomain")
	})

	t.Run("获取域信息", func(t *testing.T) {
		info, err := enhanced.GetDomainInfo("testdomain")
		assert.NoError(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, "testdomain", info.Name)
	})

	t.Run("注销域", func(t *testing.T) {
		err := enhanced.UnregisterDomain("testdomain")
		assert.NoError(t, err)
		assert.False(t, enhanced.IsDomainRegistered("testdomain"))
	})

	t.Run("域注册功能禁用时应该失败", func(t *testing.T) {
		config := DefaultEnhancedEventBusConfig()
		config.EnableDomainRegistry = false

		logger := &mockLogger{}
		disabledEnhanced, err := NewEnhanced(logger, config)
		require.NoError(t, err)

		domainInfo := DomainInfo{Component: "test"}
		err = disabledEnhanced.RegisterDomain("test", domainInfo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})
}

func TestEnhancedEventBus_RouteManagement(t *testing.T) {
	enhanced := setupEnhancedEventBus(t)

	t.Run("设置路由策略", func(t *testing.T) {
		err := enhanced.SetRouteStrategy("test.route.event", RouteDirect)
		assert.NoError(t, err)

		strategy := enhanced.GetRouteStrategy("test.route.event")
		assert.Equal(t, RouteDirect, strategy)
	})

	t.Run("获取默认路由策略", func(t *testing.T) {
		strategy := enhanced.GetRouteStrategy("nonexistent.event")
		assert.Equal(t, RouteBroadcast, strategy)
	})
}

func TestEnhancedEventBus_ValidationManagement(t *testing.T) {
	enhanced := setupEnhancedEventBus(t)
	
	// 启动event bus以初始化context
	ctx := context.Background()
	enhanced.Start(ctx)
	defer enhanced.Stop(ctx)

	t.Run("添加验证规则", func(t *testing.T) {
		rule := &testValidationRule{
			id:       "test_rule",
			name:     "Test Rule",
			priority: 1,
			enabled:  true,
		}

		err := enhanced.AddValidationRule(rule)
		assert.NoError(t, err)
	})

	t.Run("验证事件", func(t *testing.T) {
		event := &enhancedTestEvent{
			eventType: "blockchain.block.produced",
			data:      "valid data",
		}

		err := enhanced.ValidateEvent(event)
		assert.NoError(t, err)
	})

	t.Run("移除验证规则", func(t *testing.T) {
		err := enhanced.RemoveValidationRule("test_rule")
		assert.NoError(t, err)
	})
}

func TestEnhancedEventBus_StatisticsAndMonitoring(t *testing.T) {
	enhanced := setupEnhancedEventBus(t)
	ctx := context.Background()
	enhanced.Start(ctx)
	defer enhanced.Stop(ctx)

	t.Run("获取统计信息", func(t *testing.T) {
		stats := enhanced.GetStatistics()
		assert.NotNil(t, stats)
		assert.NotNil(t, stats.DomainStats)
		assert.NotNil(t, stats.RouteStats)
		assert.NotNil(t, stats.ValidationStats)
		assert.NotNil(t, stats.PerformanceStats)
		assert.NotNil(t, stats.ErrorStats)
	})

	t.Run("获取健康状态", func(t *testing.T) {
		health := enhanced.GetHealthStatus()
		assert.NotNil(t, health)
		assert.NotNil(t, health.Components)
	})

	t.Run("事件发布后统计更新", func(t *testing.T) {
		// 发布几个事件
		enhanced.PublishEventWithPriority("blockchain.block.produced", "data1", PriorityNormal)
		enhanced.PublishEventWithPriority("mempool.tx.added", "data2", PriorityHigh)

		// 验证域统计
		stats := enhanced.GetStatistics()
		blockchainStats := stats.DomainStats["blockchain"]
		mempoolStats := stats.DomainStats["mempool"]

		if blockchainStats != nil {
			assert.Greater(t, blockchainStats.EventsPublished.Load(), uint64(0))
		}
		if mempoolStats != nil {
			assert.Greater(t, mempoolStats.EventsPublished.Load(), uint64(0))
		}
	})
}

func TestEnhancedEventBus_ConfigManagement(t *testing.T) {
	enhanced := setupEnhancedEventBus(t)

	t.Run("获取配置", func(t *testing.T) {
		config := enhanced.GetEnhancedConfig()
		assert.NotNil(t, config)
		assert.True(t, config.EnableDomainRegistry)
		assert.True(t, config.EnableSmartRouting)
		assert.True(t, config.EnableEventValidation)
	})

	t.Run("更新配置", func(t *testing.T) {
		newConfig := DefaultEnhancedEventBusConfig()
		newConfig.EnableDomainRegistry = false
		newConfig.EventBatchSize = 200

		err := enhanced.UpdateEnhancedConfig(newConfig)
		assert.NoError(t, err)

		currentConfig := enhanced.GetEnhancedConfig()
		assert.False(t, currentConfig.EnableDomainRegistry)
		assert.Equal(t, 200, currentConfig.EventBatchSize)
	})

	t.Run("nil配置应该失败", func(t *testing.T) {
		err := enhanced.UpdateEnhancedConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestEnhancedEventBus_ErrorHandling(t *testing.T) {
	enhanced := setupEnhancedEventBus(t)

	t.Run("未启动时操作应该失败", func(t *testing.T) {
		// 事件发布
		err := enhanced.PublishEventWithPriority("test.event", "data", PriorityNormal)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not started")

		// 事件订阅
		_, err = enhanced.SubscribeEvent("test.event", enhancedTestEventHandler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not started")

		// 域事件订阅
		domainHandler := func(eventType string, data interface{}) error { return nil }
		_, err = enhanced.SubscribeDomainEvents("test", domainHandler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not started")
	})

	t.Run("订阅未注册域应该失败", func(t *testing.T) {
		ctx := context.Background()
		enhanced.Start(ctx)
		defer enhanced.Stop(ctx)

		handler := func(eventType string, data interface{}) error { return nil }
		_, err := enhanced.SubscribeDomainEvents("nonexistent", handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
}

// BenchmarkEnhancedEventBus_PublishEvent 性能基准测试
func BenchmarkEnhancedEventBus_PublishEvent(b *testing.B) {
	enhanced := setupEnhancedEventBus(b)
	ctx := context.Background()
	enhanced.Start(ctx)
	defer enhanced.Stop(ctx)

	event := &enhancedTestEvent{
		eventType: "benchmark.test.event",
		data:      "benchmark data",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enhanced.PublishEvent(event)
	}
}

// BenchmarkEnhancedEventBus_PublishEventWithPriority 带优先级发布性能基准测试
func BenchmarkEnhancedEventBus_PublishEventWithPriority(b *testing.B) {
	enhanced := setupEnhancedEventBus(b)
	ctx := context.Background()
	enhanced.Start(ctx)
	defer enhanced.Stop(ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enhanced.PublishEventWithPriority("benchmark.priority.event", "data", PriorityNormal)
	}
}

// BenchmarkEnhancedEventBus_BatchPublish 批量发布性能基准测试
func BenchmarkEnhancedEventBus_BatchPublish(b *testing.B) {
	enhanced := setupEnhancedEventBus(b)
	ctx := context.Background()
	enhanced.Start(ctx)
	defer enhanced.Stop(ctx)

	// 准备批量事件
	events := make([]EventRequest, 50)
	for i := 0; i < 50; i++ {
		events[i] = EventRequest{
			EventType: "benchmark.batch.event",
			Data:      "data",
			Priority:  PriorityNormal,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		enhanced.BatchPublishEvents(events)
	}
}
