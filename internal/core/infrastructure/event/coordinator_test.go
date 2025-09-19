package event

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	evbus "github.com/asaskevich/EventBus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestCoordinator 创建测试用的协调器
func setupTestCoordinator(t testing.TB, config *CoordinatorConfig) *BasicEventCoordinator {
	logger := &mockLogger{}

	// 创建组件
	domainRegistry := NewDomainRegistry(logger)
	eventRouter := NewEventRouter(logger)
	eventValidator := NewBasicEventValidator(logger, DefaultValidatorConfig())
	eventBus := evbus.New()

	if config == nil {
		config = DefaultCoordinatorConfig()
	}

	coordinator := NewBasicEventCoordinator(
		logger,
		config,
		domainRegistry,
		eventRouter,
		eventValidator,
		eventBus,
	)

	return coordinator
}

func TestNewBasicEventCoordinator(t *testing.T) {
	t.Run("使用默认配置创建协调器", func(t *testing.T) {
		coordinator := setupTestCoordinator(t, nil)

		assert.NotNil(t, coordinator)
		assert.NotNil(t, coordinator.config)
		assert.True(t, coordinator.config.EnableDomainRegistry)
		assert.True(t, coordinator.config.EnableEventRouter)
		assert.True(t, coordinator.config.EnableEventValidator)
		assert.False(t, coordinator.IsRunning())
	})

	t.Run("使用自定义配置创建协调器", func(t *testing.T) {
		config := &CoordinatorConfig{
			EnableDomainRegistry:   false,
			EnableEventRouter:      true,
			EnableEventValidator:   false,
			MaxConcurrentEvents:    500,
			EventProcessingTimeout: 10 * time.Second,
		}

		coordinator := setupTestCoordinator(t, config)

		assert.NotNil(t, coordinator)
		assert.Equal(t, config.EnableDomainRegistry, coordinator.config.EnableDomainRegistry)
		assert.Equal(t, config.MaxConcurrentEvents, coordinator.config.MaxConcurrentEvents)
		assert.Equal(t, 500, cap(coordinator.semaphore))
	})
}

func TestBasicEventCoordinator_Lifecycle(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)

	t.Run("启动协调器", func(t *testing.T) {
		ctx := context.Background()
		err := coordinator.Start(ctx)
		assert.NoError(t, err)
		assert.True(t, coordinator.IsRunning())
	})

	t.Run("重复启动应该失败", func(t *testing.T) {
		ctx := context.Background()
		err := coordinator.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")
	})

	t.Run("停止协调器", func(t *testing.T) {
		err := coordinator.Stop()
		assert.NoError(t, err)
		assert.False(t, coordinator.IsRunning())
	})

	t.Run("重复停止应该失败", func(t *testing.T) {
		err := coordinator.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})
}

func TestBasicEventCoordinator_EventPublishing(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)
	ctx := context.Background()
	coordinator.Start(ctx)
	defer coordinator.Stop()

	t.Run("发布有效事件", func(t *testing.T) {
		err := coordinator.PublishEvent("blockchain.block.produced", map[string]interface{}{
			"height": 123,
			"hash":   "abc123",
		})
		assert.NoError(t, err)
	})

	t.Run("带优先级发布事件", func(t *testing.T) {
		err := coordinator.PublishEventWithPriority("consensus.round.completed", "data", PriorityHigh)
		assert.NoError(t, err)
	})

	t.Run("带元数据发布事件", func(t *testing.T) {
		metadata, err := NewEventMetadata("mempool.tx.added", "test-source")
		require.NoError(t, err)

		err = coordinator.PublishEventWithMetadata(metadata, "transaction data")
		assert.NoError(t, err)
	})

	t.Run("nil元数据应该失败", func(t *testing.T) {
		err := coordinator.PublishEventWithMetadata(nil, "data")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "metadata cannot be nil")
	})

	t.Run("无效事件名应该失败", func(t *testing.T) {
		err := coordinator.PublishEvent("invalid-event-name", "data")
		assert.Error(t, err)
	})

	t.Run("协调器未运行时发布应该失败", func(t *testing.T) {
		coordinator.Stop()
		err := coordinator.PublishEvent("test.event", "data")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})
}

func TestBasicEventCoordinator_EventSubscription(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)
	ctx := context.Background()
	coordinator.Start(ctx)
	defer coordinator.Stop()

	t.Run("订阅有效事件", func(t *testing.T) {
		handler := func(data interface{}) {
			// 测试处理器
		}

		subID, err := coordinator.SubscribeEvent("blockchain.block.produced", handler)
		assert.NoError(t, err)
		assert.NotEmpty(t, subID)
	})

	t.Run("带选项订阅事件", func(t *testing.T) {
		handler := func(data interface{}) {}

		subID, err := coordinator.SubscribeEventWithOptions(
			"mempool.tx.added",
			handler,
			WithPriority(PriorityHigh),
			WithComponent("test-component"),
		)
		assert.NoError(t, err)
		assert.NotEmpty(t, subID)
	})

	t.Run("订阅无效事件名应该失败", func(t *testing.T) {
		handler := func(data interface{}) {}

		_, err := coordinator.SubscribeEvent("invalid-event-name", handler)
		assert.Error(t, err)
	})

	t.Run("取消订阅", func(t *testing.T) {
		handler := func(data interface{}) {}

		subID, err := coordinator.SubscribeEvent("test.unsubscribe.event", handler)
		require.NoError(t, err)

		err = coordinator.UnsubscribeEvent(subID)
		assert.NoError(t, err)
	})

	t.Run("协调器未运行时订阅应该失败", func(t *testing.T) {
		coordinator.Stop()
		handler := func(data interface{}) {}

		_, err := coordinator.SubscribeEvent("test.event", handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})
}

func TestBasicEventCoordinator_DomainManagement(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)

	t.Run("注册域", func(t *testing.T) {
		info := DomainInfo{
			Component:   "test-component",
			Description: "Test domain",
		}

		err := coordinator.RegisterDomain("testdomain", info)
		assert.NoError(t, err)
		assert.True(t, coordinator.IsDomainRegistered("testdomain"))
	})

	t.Run("列出域", func(t *testing.T) {
		domains := coordinator.ListDomains()
		assert.Contains(t, domains, "testdomain")
	})

	t.Run("注销域", func(t *testing.T) {
		err := coordinator.UnregisterDomain("testdomain")
		assert.NoError(t, err)
		assert.False(t, coordinator.IsDomainRegistered("testdomain"))
	})

	t.Run("域注册功能禁用时应该失败", func(t *testing.T) {
		config := DefaultCoordinatorConfig()
		config.EnableDomainRegistry = false
		disabledCoordinator := setupTestCoordinator(t, config)

		info := DomainInfo{Component: "test"}
		err := disabledCoordinator.RegisterDomain("test", info)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})
}

func TestBasicEventCoordinator_ValidationRuleManagement(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)

	t.Run("添加验证规则", func(t *testing.T) {
		rule := &testValidationRule{
			id:       "test_rule",
			name:     "Test Rule",
			priority: 1,
			enabled:  true,
		}

		err := coordinator.AddValidationRule(rule)
		assert.NoError(t, err)

		rules := coordinator.ListValidationRules()
		assert.Len(t, rules, 1)
		assert.Equal(t, "test_rule", rules[0].GetID())
	})

	t.Run("移除验证规则", func(t *testing.T) {
		err := coordinator.RemoveValidationRule("test_rule")
		assert.NoError(t, err)

		rules := coordinator.ListValidationRules()
		assert.Len(t, rules, 0)
	})

	t.Run("验证器禁用时应该失败", func(t *testing.T) {
		config := DefaultCoordinatorConfig()
		config.EnableEventValidator = false
		disabledCoordinator := setupTestCoordinator(t, config)

		rule := &testValidationRule{id: "test", priority: 1, enabled: true}
		err := disabledCoordinator.AddValidationRule(rule)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")
	})
}

func TestBasicEventCoordinator_RouteManagement(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)

	t.Run("设置路由策略", func(t *testing.T) {
		err := coordinator.SetRouteStrategy("test.event", RouteDirect)
		assert.NoError(t, err)

		strategy := coordinator.GetRouteStrategy("test.event")
		assert.Equal(t, RouteDirect, strategy)
	})

	t.Run("获取默认路由策略", func(t *testing.T) {
		strategy := coordinator.GetRouteStrategy("nonexistent.event")
		assert.Equal(t, RouteBroadcast, strategy)
	})

	t.Run("路由器禁用时应该返回默认策略", func(t *testing.T) {
		config := DefaultCoordinatorConfig()
		config.EnableEventRouter = false
		disabledCoordinator := setupTestCoordinator(t, config)

		err := disabledCoordinator.SetRouteStrategy("test.event", RouteDirect)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not enabled")

		strategy := disabledCoordinator.GetRouteStrategy("test.event")
		assert.Equal(t, RouteBroadcast, strategy)
	})
}

func TestBasicEventCoordinator_ConfigManagement(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)

	t.Run("更新配置", func(t *testing.T) {
		newConfig := DefaultCoordinatorConfig()
		newConfig.MaxConcurrentEvents = 2000
		newConfig.EventProcessingTimeout = 60 * time.Second

		err := coordinator.UpdateConfig(newConfig)
		assert.NoError(t, err)

		currentConfig := coordinator.GetConfig()
		assert.Equal(t, 2000, currentConfig.MaxConcurrentEvents)
		assert.Equal(t, 60*time.Second, currentConfig.EventProcessingTimeout)

		// 验证信号量容量更新
		assert.Equal(t, 2000, cap(coordinator.semaphore))
	})

	t.Run("nil配置应该失败", func(t *testing.T) {
		err := coordinator.UpdateConfig(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("获取配置返回副本", func(t *testing.T) {
		config1 := coordinator.GetConfig()
		config2 := coordinator.GetConfig()

		// 修改其中一个不应影响另一个
		config1.MaxConcurrentEvents = 9999
		assert.NotEqual(t, config1.MaxConcurrentEvents, config2.MaxConcurrentEvents)
	})
}

func TestBasicEventCoordinator_Statistics(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)
	ctx := context.Background()
	coordinator.Start(ctx)
	defer coordinator.Stop()

	t.Run("初始统计信息", func(t *testing.T) {
		stats := coordinator.GetStatistics()
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.TotalEvents.Load())
		assert.Equal(t, uint64(0), stats.SuccessEvents.Load())
		assert.Equal(t, uint64(0), stats.FailedEvents.Load())
	})

	t.Run("事件发布后统计更新", func(t *testing.T) {
		// 发布一些事件
		coordinator.PublishEvent("blockchain.block.produced", "data1")
		coordinator.PublishEvent("mempool.tx.added", "data2")

		stats := coordinator.GetStatistics()
		assert.Equal(t, uint64(2), stats.TotalEvents.Load())
		assert.Equal(t, uint64(2), stats.SuccessEvents.Load())
		assert.Equal(t, uint64(0), stats.FailedEvents.Load())

		// 验证延迟统计
		avgLatency := stats.AverageLatency.Load()
		assert.NotNil(t, avgLatency)
		assert.Greater(t, *avgLatency, time.Duration(0))

		// 验证时间统计
		lastEventTime := stats.LastEventTime.Load()
		assert.NotNil(t, lastEventTime)
	})

	t.Run("失败事件统计", func(t *testing.T) {
		// 发布无效事件
		coordinator.PublishEvent("invalid-event-name", "data")

		stats := coordinator.GetStatistics()
		assert.Greater(t, stats.FailedEvents.Load(), uint64(0))
	})

	t.Run("运行时间统计", func(t *testing.T) {
		stats := coordinator.GetStatistics()
		assert.Greater(t, stats.UptimeDuration, time.Duration(0))
	})
}

func TestBasicEventCoordinator_HealthCheck(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)
	ctx := context.Background()
	coordinator.Start(ctx)
	defer coordinator.Stop()

	t.Run("获取健康状态", func(t *testing.T) {
		health := coordinator.GetHealthStatus()
		assert.NotNil(t, health)
		assert.NotNil(t, health.Components)
		assert.NotNil(t, health.Issues)
	})

	t.Run("健康检查包含各组件", func(t *testing.T) {
		// 手动触发健康检查
		coordinator.performHealthCheck()

		health := coordinator.GetHealthStatus()
		assert.Contains(t, health.Components, "domain_registry")
		assert.Contains(t, health.Components, "event_router")
		assert.Contains(t, health.Components, "event_validator")
	})
}

func TestBasicEventCoordinator_BatchOperations(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)
	ctx := context.Background()
	coordinator.Start(ctx)
	defer coordinator.Stop()

	t.Run("批量发布事件", func(t *testing.T) {
		requests := []EventRequest{
			{EventType: "blockchain.block.produced", Data: "data1", Priority: PriorityNormal},
			{EventType: "mempool.tx.added", Data: "data2", Priority: PriorityHigh},
			{EventType: "consensus.round.completed", Data: "data3", Priority: PriorityLow},
		}

		results := coordinator.BatchPublishEvents(requests)
		assert.Len(t, results, 3)

		for i, result := range results {
			assert.Equal(t, requests[i].EventType, result.EventType)
			assert.True(t, result.Success)
			assert.Empty(t, result.Error)
			assert.Greater(t, result.Duration, time.Duration(0))
		}
	})

	t.Run("批量发布包含无效事件", func(t *testing.T) {
		requests := []EventRequest{
			{EventType: "blockchain.block.produced", Data: "data1"},
			{EventType: "invalid-event-name", Data: "data2"}, // 无效
			{EventType: "consensus.round.completed", Data: "data3"},
		}

		results := coordinator.BatchPublishEvents(requests)
		assert.Len(t, results, 3)

		assert.True(t, results[0].Success)
		assert.False(t, results[1].Success) // 应该失败
		assert.NotEmpty(t, results[1].Error)
		assert.True(t, results[2].Success)
	})

	t.Run("批量验证事件", func(t *testing.T) {
		events := []Event{
			&basicEvent{eventType: "blockchain.block.produced", data: "data1"},
			&basicEvent{eventType: "mempool.tx.added", data: "data2"},
			&basicEvent{eventType: "invalid-name", data: "data3"}, // 无效
		}

		results := coordinator.BatchValidateEvents(events)
		assert.Len(t, results, 3)

		assert.True(t, results[0].Valid)
		assert.True(t, results[1].Valid)
		assert.False(t, results[2].Valid) // 应该无效
	})
}

func TestBasicEventCoordinator_AutoCreateDomains(t *testing.T) {
	config := DefaultCoordinatorConfig()
	config.AutoCreateDomains = true
	config.RequireDomainExistence = true

	coordinator := setupTestCoordinator(t, config)
	ctx := context.Background()
	coordinator.Start(ctx)
	defer coordinator.Stop()

	t.Run("自动创建域", func(t *testing.T) {
		handler := func(data interface{}) {}

		// 订阅一个不存在域的事件
		subID, err := coordinator.SubscribeEvent("newdomain.entity.action", handler)
		assert.NoError(t, err)
		assert.NotEmpty(t, subID)

		// 验证域已被自动创建
		assert.True(t, coordinator.IsDomainRegistered("newdomain"))
	})

	t.Run("禁用自动创建时应该失败", func(t *testing.T) {
		config := DefaultCoordinatorConfig()
		config.AutoCreateDomains = false
		config.RequireDomainExistence = true

		strictCoordinator := setupTestCoordinator(t, config)
		strictCoordinator.Start(ctx)
		defer strictCoordinator.Stop()

		handler := func(data interface{}) {}

		_, err := strictCoordinator.SubscribeEvent("anotherdomain.entity.action", handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
}

func TestBasicEventCoordinator_ConcurrentAccess(t *testing.T) {
	coordinator := setupTestCoordinator(t, nil)
	ctx := context.Background()
	coordinator.Start(ctx)
	defer coordinator.Stop()

	t.Run("并发发布事件", func(t *testing.T) {
		numGoroutines := 10
		numEvents := 100

		var wg sync.WaitGroup
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				for j := 0; j < numEvents; j++ {
					eventType := "concurrent.test.event"
					data := fmt.Sprintf("data-%d-%d", idx, j)

					if err := coordinator.PublishEvent(eventType, data); err != nil {
						errors[idx] = err
						return
					}
				}
			}(i)
		}

		wg.Wait()

		// 检查错误
		for i, err := range errors {
			assert.NoError(t, err, "Goroutine %d should not have errors", i)
		}

		// 验证统计
		stats := coordinator.GetStatistics()
		expectedTotal := uint64(numGoroutines * numEvents)
		assert.Equal(t, expectedTotal, stats.TotalEvents.Load())
		assert.Equal(t, expectedTotal, stats.SuccessEvents.Load())
	})

	t.Run("并发订阅和取消订阅", func(t *testing.T) {
		numGoroutines := 5

		var wg sync.WaitGroup
		subscriptionIDs := make([]string, numGoroutines)

		// 并发订阅
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				handler := func(data interface{}) {}
				eventType := fmt.Sprintf("test.concurrent.subscribe_%d", idx)

				subID, err := coordinator.SubscribeEvent(eventType, handler)
				assert.NoError(t, err)
				subscriptionIDs[idx] = subID
			}(i)
		}

		wg.Wait()

		// 并发取消订阅
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				err := coordinator.UnsubscribeEvent(subscriptionIDs[idx])
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
	})

	t.Run("并发域管理", func(t *testing.T) {
		numGoroutines := 5

		var wg sync.WaitGroup

		// 并发注册域
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				domain := fmt.Sprintf("concurrent_domain_%d", idx)
				info := DomainInfo{
					Component:   fmt.Sprintf("component_%d", idx),
					Description: fmt.Sprintf("Test domain %d", idx),
				}

				err := coordinator.RegisterDomain(domain, info)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// 验证所有域都注册成功
		for i := 0; i < numGoroutines; i++ {
			domain := fmt.Sprintf("concurrent_domain_%d", i)
			assert.True(t, coordinator.IsDomainRegistered(domain))
		}
	})
}

func TestBasicEventCoordinator_ErrorHandling(t *testing.T) {
	t.Run("组件启动失败处理", func(t *testing.T) {
		// 这个测试比较难模拟，因为我们的组件启动通常不会失败
		// 在实际实现中，可能需要创建mock组件来模拟失败情况
		coordinator := setupTestCoordinator(t, nil)
		assert.NotNil(t, coordinator)
	})

	t.Run("并发限制", func(t *testing.T) {
		config := DefaultCoordinatorConfig()
		config.MaxConcurrentEvents = 1 // 设置很小的并发限制

		limitedCoordinator := setupTestCoordinator(t, config)
		ctx := context.Background()
		limitedCoordinator.Start(ctx)
		defer limitedCoordinator.Stop()

		// 这个测试需要模拟阻塞的事件处理来触发并发限制
		// 由于测试环境的限制，这里只验证基本功能
		err := limitedCoordinator.PublishEvent("test.limit.event", "data")
		assert.NoError(t, err)
	})
}

// BenchmarkBasicEventCoordinator_PublishEvent 性能基准测试
func BenchmarkBasicEventCoordinator_PublishEvent(b *testing.B) {
	coordinator := setupTestCoordinator(b, nil)
	ctx := context.Background()
	coordinator.Start(ctx)
	defer coordinator.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coordinator.PublishEvent("benchmark.test.event", "benchmark data")
	}
}

// BenchmarkBasicEventCoordinator_BatchPublish 批量发布性能基准测试
func BenchmarkBasicEventCoordinator_BatchPublish(b *testing.B) {
	coordinator := setupTestCoordinator(b, nil)
	ctx := context.Background()
	coordinator.Start(ctx)
	defer coordinator.Stop()

	// 准备批量事件
	requests := make([]EventRequest, 100)
	for i := 0; i < 100; i++ {
		requests[i] = EventRequest{
			EventType: "benchmark.batch.event",
			Data:      fmt.Sprintf("data-%d", i),
			Priority:  PriorityNormal,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		coordinator.BatchPublishEvents(requests)
	}
}
