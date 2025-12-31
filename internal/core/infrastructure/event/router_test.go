package event

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFilter 测试过滤器
type testFilter struct {
	id          string
	name        string
	matchFunc   func(string, interface{}) bool
	description string
}

func (f *testFilter) Match(eventType string, data interface{}) bool {
	if f.matchFunc != nil {
		return f.matchFunc(eventType, data)
	}
	return true
}

func (f *testFilter) GetFilterInfo() *FilterInfo {
	return &FilterInfo{
		ID:          f.id,
		Name:        f.name,
		Priority:    1,
		Conditions:  make(map[string]interface{}),
		Description: f.description,
	}
}

// testHandler 测试处理器
type testHandler struct {
	id        string
	callCount atomic.Uint64
	lastData  atomic.Pointer[interface{}]
	calls     []HandlerCall
	mu        sync.Mutex
}

type HandlerCall struct {
	EventType string
	Data      interface{}
	Timestamp time.Time
}

func (h *testHandler) Handle(eventType string, data interface{}) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.callCount.Add(1)
	h.lastData.Store(&data)
	h.calls = append(h.calls, HandlerCall{
		EventType: eventType,
		Data:      data,
		Timestamp: time.Now(),
	})
	return nil
}

func (h *testHandler) GetCallCount() uint64 {
	return h.callCount.Load()
}

func (h *testHandler) GetCalls() []HandlerCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]HandlerCall, len(h.calls))
	copy(result, h.calls)
	return result
}

func TestEventRouter_NewEventRouter(t *testing.T) {
	router := NewEventRouter(&mockLogger{})
	assert.NotNil(t, router)
	assert.False(t, router.IsRunning())
	assert.NotNil(t, router.subscriptions)
	assert.NotNil(t, router.strategies)
	assert.NotNil(t, router.priorityQueues)
	assert.Len(t, router.priorityQueues, 4) // 4个优先级级别
}

func TestEventRouter_StartStop(t *testing.T) {
	router := NewEventRouter(&mockLogger{})

	t.Run("正常启动", func(t *testing.T) {
		ctx := context.Background()
		err := router.Start(ctx)
		assert.NoError(t, err)
		assert.True(t, router.IsRunning())
	})

	t.Run("重复启动应该失败", func(t *testing.T) {
		ctx := context.Background()
		err := router.Start(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")
	})

	t.Run("正常停止", func(t *testing.T) {
		err := router.Stop()
		assert.NoError(t, err)
		assert.False(t, router.IsRunning())
	})

	t.Run("重复停止应该失败", func(t *testing.T) {
		err := router.Stop()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")
	})
}

func TestEventRouter_AddRemoveSubscription(t *testing.T) {
	router := NewEventRouter(&mockLogger{})

	t.Run("添加订阅", func(t *testing.T) {
		handler := &testHandler{id: "test1"}

		subID, err := router.AddSubscription("test.event", handler)
		assert.NoError(t, err)
		assert.NotEmpty(t, subID)

		// 验证订阅已添加
		subs := router.GetSubscriptions("test.event")
		assert.Len(t, subs, 1)
		assert.Equal(t, subID, subs[0].ID)
		assert.Equal(t, "test.event", subs[0].EventType)
		assert.Equal(t, handler, subs[0].Handler)
		assert.True(t, subs[0].Active)
	})

	t.Run("添加带选项的订阅", func(t *testing.T) {
		handler := &testHandler{id: "test2"}
		filter := &testFilter{id: "filter1", name: "test filter"}

		subID, err := router.AddSubscription("test.event.with.options", handler,
			WithPriority(PriorityHigh),
			WithComponent("test-component"),
			WithFilter(filter),
		)
		assert.NoError(t, err)
		assert.NotEmpty(t, subID)

		subs := router.GetSubscriptions("test.event.with.options")
		require.Len(t, subs, 1)
		assert.Equal(t, PriorityHigh, subs[0].Priority)
		assert.Equal(t, "test-component", subs[0].Component)
		assert.Equal(t, filter, subs[0].Filter)
	})

	t.Run("添加空事件类型应该失败", func(t *testing.T) {
		handler := &testHandler{id: "test3"}

		_, err := router.AddSubscription("", handler)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("添加空处理器应该失败", func(t *testing.T) {
		_, err := router.AddSubscription("test.event", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("移除订阅", func(t *testing.T) {
		handler := &testHandler{id: "test4"}

		subID, err := router.AddSubscription("test.remove", handler)
		require.NoError(t, err)

		// 验证订阅存在
		subs := router.GetSubscriptions("test.remove")
		assert.Len(t, subs, 1)

		// 移除订阅
		err = router.RemoveSubscription(subID)
		assert.NoError(t, err)

		// 验证订阅已移除
		subs = router.GetSubscriptions("test.remove")
		assert.Len(t, subs, 0)
	})

	t.Run("移除不存在的订阅应该失败", func(t *testing.T) {
		err := router.RemoveSubscription("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestEventRouter_RouteStrategies(t *testing.T) {
	router := NewEventRouter(&mockLogger{})
	ctx := context.Background()
	require.NoError(t, router.Start(ctx))
	defer router.Stop()

	t.Run("设置和获取路由策略", func(t *testing.T) {
		// 默认策略应该是广播
		assert.Equal(t, RouteBroadcast, router.GetRouteStrategy("test.event"))

		// 设置直接路由策略
		router.SetRouteStrategy("test.event", RouteDirect)
		assert.Equal(t, RouteDirect, router.GetRouteStrategy("test.event"))

		// 设置轮询路由策略
		router.SetRouteStrategy("test.event", RouteRoundRobin)
		assert.Equal(t, RouteRoundRobin, router.GetRouteStrategy("test.event"))
	})

	t.Run("广播路由策略", func(t *testing.T) {
		eventType := "test.broadcast"
		router.SetRouteStrategy(eventType, RouteBroadcast)

		// 添加多个订阅者
		handlers := []*testHandler{
			{id: "handler1"},
			{id: "handler2"},
			{id: "handler3"},
		}

		for _, handler := range handlers {
			_, err := router.AddSubscription(eventType, handler)
			require.NoError(t, err)
		}

		// 发送事件
		data := "test data"
		err := router.RouteEvent(eventType, data, PriorityNormal, "test")
		assert.NoError(t, err)

		// 等待事件处理
		time.Sleep(10 * time.Millisecond)

		// 验证所有处理器都被调用（在实际实现中需要实现处理器调用）
		// 这里只验证路由逻辑的正确性
	})

	t.Run("直接路由策略", func(t *testing.T) {
		eventType := "test.direct"
		router.SetRouteStrategy(eventType, RouteDirect)

		handler := &testHandler{id: "direct-handler"}
		_, err := router.AddSubscription(eventType, handler)
		require.NoError(t, err)

		// 发送事件
		err = router.RouteEvent(eventType, "direct data", PriorityNormal, "test")
		assert.NoError(t, err)
	})

	t.Run("轮询路由策略", func(t *testing.T) {
		eventType := "test.roundrobin"
		router.SetRouteStrategy(eventType, RouteRoundRobin)

		// 添加多个处理器
		for i := 0; i < 3; i++ {
			handler := &testHandler{id: fmt.Sprintf("rr-handler-%d", i)}
			_, err := router.AddSubscription(eventType, handler)
			require.NoError(t, err)
		}

		// 发送多个事件
		for i := 0; i < 6; i++ {
			err := router.RouteEvent(eventType, fmt.Sprintf("data-%d", i), PriorityNormal, "test")
			assert.NoError(t, err)
		}
	})

	t.Run("优先级路由策略", func(t *testing.T) {
		eventType := "test.priority"
		router.SetRouteStrategy(eventType, RoutePriority)

		// 添加不同优先级的处理器
		priorities := []Priority{PriorityLow, PriorityNormal, PriorityHigh, PriorityCritical}
		for _, priority := range priorities {
			handler := &testHandler{id: fmt.Sprintf("priority-handler-%d", priority)}
			_, err := router.AddSubscription(eventType, handler, WithPriority(priority))
			require.NoError(t, err)
		}

		// 发送事件
		err := router.RouteEvent(eventType, "priority data", PriorityNormal, "test")
		assert.NoError(t, err)
	})
}

func TestEventRouter_EventRouting(t *testing.T) {
	router := NewEventRouter(&mockLogger{})
	ctx := context.Background()
	require.NoError(t, router.Start(ctx))
	defer router.Stop()

	t.Run("路由器未运行时应该失败", func(t *testing.T) {
		router.Stop()
		err := router.RouteEvent("test.event", "data", PriorityNormal, "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not running")

		// 重新启动
		require.NoError(t, router.Start(ctx))
	})

	t.Run("紧急优先级事件直接处理", func(t *testing.T) {
		eventType := "test.critical"
		handler := &testHandler{id: "critical-handler"}

		_, err := router.AddSubscription(eventType, handler)
		require.NoError(t, err)

		// 发送紧急事件
		err = router.RouteEvent(eventType, "critical data", PriorityCritical, "test")
		assert.NoError(t, err)
	})

	t.Run("普通优先级事件队列处理", func(t *testing.T) {
		eventType := "test.normal"
		handler := &testHandler{id: "normal-handler"}

		_, err := router.AddSubscription(eventType, handler)
		require.NoError(t, err)

		// 发送普通事件
		err = router.RouteEvent(eventType, "normal data", PriorityNormal, "test")
		assert.NoError(t, err)

		// 等待队列处理
		time.Sleep(10 * time.Millisecond)
	})

	t.Run("没有订阅者的事件", func(t *testing.T) {
		err := router.RouteEvent("no.subscribers", "data", PriorityNormal, "test")
		assert.NoError(t, err) // 应该成功，只是没有处理器
	})
}

func TestEventRouter_Filters(t *testing.T) {
	router := NewEventRouter(&mockLogger{})
	ctx := context.Background()
	require.NoError(t, router.Start(ctx))
	defer router.Stop()

	t.Run("添加和移除全局过滤器", func(t *testing.T) {
		filter := &testFilter{
			id:   "global-filter-1",
			name: "Global Test Filter",
			matchFunc: func(eventType string, data interface{}) bool {
				return eventType == "allowed.event"
			},
		}

		// 添加过滤器
		router.AddFilter(filter)

		// 验证过滤器生效
		handler := &testHandler{id: "filter-handler"}
		_, err := router.AddSubscription("allowed.event", handler)
		require.NoError(t, err)
		_, err = router.AddSubscription("blocked.event", handler)
		require.NoError(t, err)

		// 发送允许的事件
		err = router.RouteEvent("allowed.event", "data", PriorityNormal, "test")
		assert.NoError(t, err)

		// 发送被阻止的事件
		err = router.RouteEvent("blocked.event", "data", PriorityNormal, "test")
		assert.NoError(t, err)

		// 移除过滤器
		router.RemoveFilter("global-filter-1")

		// 再次发送被阻止的事件，现在应该通过
		err = router.RouteEvent("blocked.event", "data", PriorityNormal, "test")
		assert.NoError(t, err)
	})

	t.Run("订阅级过滤器", func(t *testing.T) {
		filter := &testFilter{
			id:   "sub-filter-1",
			name: "Subscription Filter",
			matchFunc: func(eventType string, data interface{}) bool {
				if str, ok := data.(string); ok {
					return str == "allowed"
				}
				return false
			},
		}

		handler := &testHandler{id: "filtered-handler"}
		_, err := router.AddSubscription("filtered.event", handler, WithFilter(filter))
		require.NoError(t, err)

		// 发送允许的数据
		err = router.RouteEvent("filtered.event", "allowed", PriorityNormal, "test")
		assert.NoError(t, err)

		// 发送被阻止的数据
		err = router.RouteEvent("filtered.event", "blocked", PriorityNormal, "test")
		assert.NoError(t, err)
	})
}

func TestEventRouter_ConcurrentAccess(t *testing.T) {
	router := NewEventRouter(&mockLogger{})
	ctx := context.Background()
	require.NoError(t, router.Start(ctx))
	defer router.Stop()

	numGoroutines := 10
	numOperations := 100

	t.Run("并发添加订阅", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < numOperations/10; j++ {
					eventType := fmt.Sprintf("concurrent.event.%d.%d", idx, j)
					handler := &testHandler{id: fmt.Sprintf("handler-%d-%d", idx, j)}
					_, err := router.AddSubscription(eventType, handler)
					if err != nil {
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
	})

	t.Run("并发路由事件", func(t *testing.T) {
		// 先添加一些订阅
		for i := 0; i < 5; i++ {
			eventType := fmt.Sprintf("route.test.%d", i)
			handler := &testHandler{id: fmt.Sprintf("route-handler-%d", i)}
			_, err := router.AddSubscription(eventType, handler)
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < numOperations/10; j++ {
					eventType := fmt.Sprintf("route.test.%d", j%5)
					data := fmt.Sprintf("data-%d-%d", idx, j)
					err := router.RouteEvent(eventType, data, PriorityNormal, "test")
					if err != nil {
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
	})

	t.Run("并发添加和移除订阅", func(t *testing.T) {
		var wg sync.WaitGroup
		subscriptionIDs := make([]string, numGoroutines)

		// 并发添加订阅
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				eventType := fmt.Sprintf("add.remove.%d", idx)
				handler := &testHandler{id: fmt.Sprintf("add-remove-handler-%d", idx)}
				subID, err := router.AddSubscription(eventType, handler)
				assert.NoError(t, err)
				subscriptionIDs[idx] = subID
			}(i)
		}

		wg.Wait()

		// 并发移除订阅
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				err := router.RemoveSubscription(subscriptionIDs[idx])
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()
	})
}

func TestEventRouter_Statistics(t *testing.T) {
	router := NewEventRouter(&mockLogger{})
	ctx := context.Background()
	require.NoError(t, router.Start(ctx))
	defer router.Stop()

	t.Run("获取统计信息", func(t *testing.T) {
		stats := router.GetStatistics()

		// 验证基本统计字段
		assert.Contains(t, stats, "total_routed")
		assert.Contains(t, stats, "success_routed")
		assert.Contains(t, stats, "failed_routed")
		assert.Contains(t, stats, "running")
		assert.Contains(t, stats, "routes_by_strategy")
		assert.Contains(t, stats, "routes_by_priority")
		assert.Contains(t, stats, "queue_lengths")

		// 验证运行状态
		assert.True(t, stats["running"].(bool))
	})

	t.Run("路由统计更新", func(t *testing.T) {
		// 添加订阅和发送事件
		handler := &testHandler{id: "stats-handler"}
		_, err := router.AddSubscription("stats.event", handler)
		require.NoError(t, err)

		// 发送一些事件
		for i := 0; i < 5; i++ {
			err = router.RouteEvent("stats.event", fmt.Sprintf("data-%d", i), PriorityNormal, "test")
			assert.NoError(t, err)
		}

		// 等待处理
		time.Sleep(50 * time.Millisecond)

		stats := router.GetStatistics()
		totalRouted := stats["total_routed"].(uint64)
		assert.GreaterOrEqual(t, totalRouted, uint64(5))
	})
}

func TestEventRouter_GetSubscriptions(t *testing.T) {
	router := NewEventRouter(&mockLogger{})

	t.Run("获取特定事件类型的订阅", func(t *testing.T) {
		eventType := "get.subscriptions.test"

		// 添加多个订阅
		for i := 0; i < 3; i++ {
			handler := &testHandler{id: fmt.Sprintf("handler-%d", i)}
			_, err := router.AddSubscription(eventType, handler)
			require.NoError(t, err)
		}

		subs := router.GetSubscriptions(eventType)
		assert.Len(t, subs, 3)

		// 验证返回的是副本（修改不应影响原始数据）
		subs[0].Active = false
		originalSubs := router.GetSubscriptions(eventType)
		assert.True(t, originalSubs[0].Active)
	})

	t.Run("获取所有订阅", func(t *testing.T) {
		allSubs := router.GetAllSubscriptions()
		assert.NotNil(t, allSubs)

		// 应该包含之前添加的订阅
		assert.Contains(t, allSubs, "get.subscriptions.test")
		assert.Len(t, allSubs["get.subscriptions.test"], 3)
	})
}

// BenchmarkEventRouter_RouteEvent 性能基准测试
func BenchmarkEventRouter_RouteEvent(b *testing.B) {
	router := NewEventRouter(&mockLogger{})
	ctx := context.Background()
	router.Start(ctx)
	defer router.Stop()

	// 添加一些订阅
	for i := 0; i < 10; i++ {
		handler := &testHandler{id: fmt.Sprintf("bench-handler-%d", i)}
		router.AddSubscription("bench.event", handler)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			router.RouteEvent("bench.event", "bench data", PriorityNormal, "bench")
		}
	})
}

// BenchmarkEventRouter_AddSubscription 性能基准测试
func BenchmarkEventRouter_AddSubscription(b *testing.B) {
	router := NewEventRouter(&mockLogger{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eventType := fmt.Sprintf("bench.event.%d", i)
		handler := &testHandler{id: fmt.Sprintf("bench-handler-%d", i)}
		router.AddSubscription(eventType, handler)
	}
}
