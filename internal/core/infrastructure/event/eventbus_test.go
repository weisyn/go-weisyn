package event

import (
	"sync"
	"testing"
	"time"

	eventconfig "github.com/weisyn/v1/internal/config/event"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/event"
)

// 简单的测试函数，用于验证事件处理
func TestEventBus(t *testing.T) {
	// 使用默认配置创建事件总线
	config := eventconfig.New(nil) // 使用默认配置
	eventBus := New(config)

	// 测试同步事件处理
	var receivedData string
	var wg sync.WaitGroup
	wg.Add(1)

	// 定义处理函数
	handler := func(data string) {
		receivedData = data
		wg.Done()
	}

	// 订阅事件
	err := eventBus.Subscribe(event.EventType("test-event"), handler)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// 发布事件
	eventBus.Publish(event.EventType("test-event"), "hello world")

	// 等待处理完成
	wg.Wait()

	// 验证结果
	if receivedData != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", receivedData)
	}

	// 测试异步事件处理
	var asyncData string
	var asyncWg sync.WaitGroup
	asyncWg.Add(1)

	// 定义异步处理函数
	asyncHandler := func(data string) {
		// 模拟耗时操作
		time.Sleep(100 * time.Millisecond)
		asyncData = data
		asyncWg.Done()
	}

	// 订阅异步事件
	err = eventBus.SubscribeAsync(event.EventType("async-event"), asyncHandler, false)
	if err != nil {
		t.Fatalf("Failed to subscribe async: %v", err)
	}

	// 发布事件
	eventBus.Publish(event.EventType("async-event"), "async data")

	// 等待所有异步处理完成
	eventBus.WaitAsync()
	asyncWg.Wait()

	// 验证结果
	if asyncData != "async data" {
		t.Errorf("Expected 'async data', got '%s'", asyncData)
	}

	// 测试取消订阅
	err = eventBus.Unsubscribe(event.EventType("test-event"), handler)
	if err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	// 验证取消后不再接收事件
	receivedData = ""
	eventBus.Publish(event.EventType("test-event"), "should not receive")

	// 由于已取消订阅，receivedData应该保持为空
	if receivedData != "" {
		t.Errorf("Expected empty string after unsubscribe, got '%s'", receivedData)
	}
}
