package event

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// mockLogger 模拟日志记录器
type mockLogger struct{}

func (m *mockLogger) Debug(msg string)                          {}
func (m *mockLogger) Debugf(format string, args ...interface{}) {}
func (m *mockLogger) Info(msg string)                           {}
func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Warn(msg string)                           {}
func (m *mockLogger) Warnf(format string, args ...interface{})  {}
func (m *mockLogger) Error(msg string)                          {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}
func (m *mockLogger) Fatal(msg string)                          {}
func (m *mockLogger) Fatalf(format string, args ...interface{}) {}
func (m *mockLogger) With(args ...interface{}) log.Logger {
	return &mockLogger{}
}
func (m *mockLogger) Sync() error {
	return nil
}
func (m *mockLogger) GetZapLogger() *zap.Logger {
	return nil
}

func TestDomainRegistry_RegisterDomain(t *testing.T) {
	registry := NewDomainRegistry(&mockLogger{})

	t.Run("正常注册域", func(t *testing.T) {
		info := DomainInfo{
			Component:   "test-component",
			Description: "测试域",
			EventTypes:  []string{"entity.created", "entity.updated"},
		}

		err := registry.RegisterDomain("test", info)
		assert.NoError(t, err)
		assert.True(t, registry.IsDomainRegistered("test"))

		// 验证域信息
		domainInfo := registry.GetDomainInfo("test")
		require.NotNil(t, domainInfo)
		assert.Equal(t, "test", domainInfo.Name)
		assert.Equal(t, "test-component", domainInfo.Component)
		assert.Equal(t, "测试域", domainInfo.Description)
		assert.Equal(t, []string{"entity.created", "entity.updated"}, domainInfo.EventTypes)
		assert.True(t, domainInfo.Active)
		assert.False(t, domainInfo.RegisteredAt.IsZero())
	})

	t.Run("重复注册域应该失败", func(t *testing.T) {
		info := DomainInfo{
			Component:   "another-component",
			Description: "另一个测试域",
		}

		err := registry.RegisterDomain("test", info)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("空域名应该失败", func(t *testing.T) {
		info := DomainInfo{
			Component: "test-component",
		}

		err := registry.RegisterDomain("", info)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("无效域名格式应该失败", func(t *testing.T) {
		invalidDomains := []string{
			"Test",                         // 大写字母
			"test-domain",                  // 连字符
			"123test",                      // 数字开头
			"a",                            // 太短
			"a" + string(make([]byte, 32)), // 太长
			"test.domain",                  // 包含点
		}

		for _, domain := range invalidDomains {
			err := registry.RegisterDomain(domain, DomainInfo{Component: "test"})
			assert.Error(t, err, "Domain %s should be invalid", domain)
		}
	})

	t.Run("有效域名格式应该成功", func(t *testing.T) {
		validDomains := []string{
			"blockchain",
			"mempool",
			"consensus",
			"test123",
			"test_domain",
			"abc",
		}

		for _, domain := range validDomains {
			err := registry.RegisterDomain(domain, DomainInfo{Component: "test"})
			assert.NoError(t, err, "Domain %s should be valid", domain)
		}
	})
}

func TestDomainRegistry_UnregisterDomain(t *testing.T) {
	registry := NewDomainRegistry(&mockLogger{})

	// 先注册一个域
	err := registry.RegisterDomain("test", DomainInfo{Component: "test-component"})
	require.NoError(t, err)

	t.Run("正常注销域", func(t *testing.T) {
		err := registry.UnregisterDomain("test")
		assert.NoError(t, err)
		assert.False(t, registry.IsDomainRegistered("test"))
		assert.Nil(t, registry.GetDomainInfo("test"))
	})

	t.Run("注销不存在的域应该失败", func(t *testing.T) {
		err := registry.UnregisterDomain("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestDomainRegistry_ListDomains(t *testing.T) {
	registry := NewDomainRegistry(&mockLogger{})

	// 注册多个域
	domains := []string{"blockchain", "mempool", "consensus"}
	for _, domain := range domains {
		err := registry.RegisterDomain(domain, DomainInfo{Component: domain + "-component"})
		require.NoError(t, err)
	}

	// 测试列出所有域
	listedDomains := registry.ListDomains()
	assert.Len(t, listedDomains, 3)

	// 验证所有域都在列表中
	domainSet := make(map[string]bool)
	for _, domain := range listedDomains {
		domainSet[domain] = true
	}
	for _, expected := range domains {
		assert.True(t, domainSet[expected], "Domain %s should be in the list", expected)
	}
}

func TestDomainRegistry_ValidateEventName(t *testing.T) {
	registry := NewDomainRegistry(&mockLogger{})

	t.Run("有效事件名", func(t *testing.T) {
		validNames := []string{
			"blockchain.block.produced",
			"mempool.tx.added",
			"consensus.round.completed",
			"network.peer.connected",
			"system.startup.completed",
			"blockchain.block.confirmed.final",
		}

		for _, name := range validNames {
			err := registry.ValidateEventName(name)
			assert.NoError(t, err, "Event name %s should be valid", name)
		}
	})

	t.Run("无效事件名格式", func(t *testing.T) {
		invalidNames := []string{
			"",                               // 空名称
			"blockchain",                     // 缺少部分
			"blockchain.block",               // 缺少动作
			"Blockchain.block.produced",      // 大写字母
			"blockchain.Block.produced",      // 大写字母
			"blockchain.block.Produced",      // 大写字母
			"blockchain-test.block.produced", // 域名包含连字符
			"blockchain.block-test.produced", // 实体包含连字符
		}

		for _, name := range invalidNames {
			err := registry.ValidateEventName(name)
			assert.Error(t, err, "Event name %s should be invalid", name)
		}
	})
}

func TestDomainRegistry_ValidateEventNameWithDomainCheck(t *testing.T) {
	registry := NewDomainRegistry(&mockLogger{})

	// 注册一个域
	err := registry.RegisterDomain("blockchain", DomainInfo{Component: "blockchain-component"})
	require.NoError(t, err)

	t.Run("严格模式下已注册域的事件名应该有效", func(t *testing.T) {
		err := registry.ValidateEventNameWithDomainCheck("blockchain.block.produced", true)
		assert.NoError(t, err)
	})

	t.Run("严格模式下未注册域的事件名应该无效", func(t *testing.T) {
		err := registry.ValidateEventNameWithDomainCheck("unregistered.block.produced", true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})

	t.Run("非严格模式下未注册域的事件名应该有效", func(t *testing.T) {
		err := registry.ValidateEventNameWithDomainCheck("unregistered.block.produced", false)
		assert.NoError(t, err)
	})
}

func TestDomainRegistry_ExtractDomain(t *testing.T) {
	registry := NewDomainRegistry(&mockLogger{})

	testCases := []struct {
		eventName      string
		expectedDomain string
	}{
		{"blockchain.block.produced", "blockchain"},
		{"mempool.tx.added", "mempool"},
		{"consensus.round.completed", "consensus"},
		{"single", "single"},
		{"", ""},
	}

	for _, tc := range testCases {
		domain := registry.ExtractDomain(tc.eventName)
		assert.Equal(t, tc.expectedDomain, domain, "Event name: %s", tc.eventName)
	}
}

func TestDomainRegistry_EventRoutes(t *testing.T) {
	registry := NewDomainRegistry(&mockLogger{})

	eventPattern := "blockchain.block.*"
	subscriberIDs := []string{"subscriber1", "subscriber2", "subscriber3"}

	t.Run("添加事件路由", func(t *testing.T) {
		for _, id := range subscriberIDs {
			registry.AddEventRoute(eventPattern, id)
		}

		routes := registry.GetEventRoutes(eventPattern)
		assert.Len(t, routes, 3)

		// 验证所有订阅者都在路由中
		routeSet := make(map[string]bool)
		for _, route := range routes {
			routeSet[route] = true
		}
		for _, id := range subscriberIDs {
			assert.True(t, routeSet[id], "Subscriber %s should be in routes", id)
		}
	})

	t.Run("避免重复添加", func(t *testing.T) {
		// 重复添加相同的订阅者
		registry.AddEventRoute(eventPattern, subscriberIDs[0])
		routes := registry.GetEventRoutes(eventPattern)
		assert.Len(t, routes, 3, "Should not add duplicate subscriber")
	})

	t.Run("移除事件路由", func(t *testing.T) {
		registry.RemoveEventRoute(eventPattern, subscriberIDs[1])
		routes := registry.GetEventRoutes(eventPattern)
		assert.Len(t, routes, 2)

		// 验证被移除的订阅者不在路由中
		for _, route := range routes {
			assert.NotEqual(t, subscriberIDs[1], route)
		}
	})

	t.Run("移除所有订阅者后删除路由", func(t *testing.T) {
		// 移除剩余的订阅者
		registry.RemoveEventRoute(eventPattern, subscriberIDs[0])
		registry.RemoveEventRoute(eventPattern, subscriberIDs[2])

		routes := registry.GetEventRoutes(eventPattern)
		assert.Nil(t, routes, "Route should be deleted when no subscribers left")
	})
}

func TestDomainRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewDomainRegistry(&mockLogger{})
	numGoroutines := 10
	numOperations := 100

	t.Run("并发注册不同域", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				domain := fmt.Sprintf("domain%d", idx)
				info := DomainInfo{
					Component:   fmt.Sprintf("component%d", idx),
					Description: fmt.Sprintf("Test domain %d", idx),
				}
				errors[idx] = registry.RegisterDomain(domain, info)
			}(i)
		}

		wg.Wait()

		// 检查所有注册都成功
		for i, err := range errors {
			assert.NoError(t, err, "Registration %d should succeed", i)
		}

		// 验证所有域都注册成功
		for i := 0; i < numGoroutines; i++ {
			domain := fmt.Sprintf("domain%d", i)
			assert.True(t, registry.IsDomainRegistered(domain))
		}
	})

	t.Run("并发读写操作", func(t *testing.T) {
		// 先注册一些域
		for i := 0; i < 5; i++ {
			domain := fmt.Sprintf("testdomain%d", i)
			err := registry.RegisterDomain(domain, DomainInfo{Component: "test"})
			require.NoError(t, err)
		}

		var wg sync.WaitGroup

		// 启动读操作
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					registry.ListDomains()
					registry.IsDomainRegistered("testdomain0")
					registry.GetDomainInfo("testdomain1")
				}
			}()
		}

		// 启动写操作
		for i := 0; i < numGoroutines/2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < numOperations/10; j++ {
					eventPattern := fmt.Sprintf("test.event.%d_%d", idx, j)
					subscriberID := fmt.Sprintf("subscriber_%d_%d", idx, j)
					registry.AddEventRoute(eventPattern, subscriberID)
					registry.GetEventRoutes(eventPattern)
					registry.RemoveEventRoute(eventPattern, subscriberID)
				}
			}(i)
		}

		wg.Wait()
		// 如果没有竞态条件，测试应该正常完成
	})
}

func TestDomainRegistry_GetStatistics(t *testing.T) {
	registry := NewDomainRegistry(&mockLogger{})

	// 注册几个域
	domains := []struct {
		name       string
		component  string
		eventTypes []string
	}{
		{"blockchain", "blockchain-component", []string{"block.produced", "block.confirmed"}},
		{"mempool", "mempool-component", []string{"tx.added", "tx.removed", "tx.confirmed"}},
		{"consensus", "consensus-component", []string{"round.started", "round.completed"}},
	}

	for _, domain := range domains {
		err := registry.RegisterDomain(domain.name, DomainInfo{
			Component:  domain.component,
			EventTypes: domain.eventTypes,
		})
		require.NoError(t, err)
	}

	// 添加一些路由
	registry.AddEventRoute("blockchain.*", "subscriber1")
	registry.AddEventRoute("mempool.*", "subscriber2")

	stats := registry.GetStatistics()

	assert.Equal(t, 3, stats["active_domains"])
	assert.Equal(t, 7, stats["total_event_types"]) // 2+3+2
	assert.Equal(t, 2, stats["total_routes"])

	componentCount := stats["component_count"].(map[string]int)
	assert.Equal(t, 1, componentCount["blockchain-component"])
	assert.Equal(t, 1, componentCount["mempool-component"])
	assert.Equal(t, 1, componentCount["consensus-component"])

	assert.NotNil(t, stats["last_updated"])
}

func TestIsValidEventName(t *testing.T) {
	validNames := []string{
		"blockchain.block.produced",
		"mempool.tx.added",
	}

	invalidNames := []string{
		"",
		"invalid",
		"Invalid.block.produced",
	}

	for _, name := range validNames {
		assert.True(t, IsValidEventName(name), "Event name %s should be valid", name)
	}

	for _, name := range invalidNames {
		assert.False(t, IsValidEventName(name), "Event name %s should be invalid", name)
	}
}

func TestIsValidDomainName(t *testing.T) {
	validDomains := []string{
		"blockchain",
		"mempool",
		"test123",
		"test_domain",
	}

	invalidDomains := []string{
		"",
		"Test",
		"test-domain",
		"123test",
		"a",
	}

	for _, domain := range validDomains {
		assert.True(t, IsValidDomainName(domain), "Domain %s should be valid", domain)
	}

	for _, domain := range invalidDomains {
		assert.False(t, IsValidDomainName(domain), "Domain %s should be invalid", domain)
	}
}

// BenchmarkDomainRegistry_RegisterDomain 性能基准测试
func BenchmarkDomainRegistry_RegisterDomain(b *testing.B) {
	registry := NewDomainRegistry(&mockLogger{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domain := fmt.Sprintf("domain%d", i)
		info := DomainInfo{
			Component:   "test-component",
			Description: "Benchmark test domain",
		}
		registry.RegisterDomain(domain, info)
	}
}

// BenchmarkDomainRegistry_IsDomainRegistered 性能基准测试
func BenchmarkDomainRegistry_IsDomainRegistered(b *testing.B) {
	registry := NewDomainRegistry(&mockLogger{})

	// 预先注册一些域
	for i := 0; i < 1000; i++ {
		domain := fmt.Sprintf("domain%d", i)
		registry.RegisterDomain(domain, DomainInfo{Component: "test"})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domain := fmt.Sprintf("domain%d", i%1000)
		registry.IsDomainRegistered(domain)
	}
}

// BenchmarkDomainRegistry_ValidateEventName 性能基准测试
func BenchmarkDomainRegistry_ValidateEventName(b *testing.B) {
	registry := NewDomainRegistry(&mockLogger{})
	eventName := "blockchain.block.produced"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.ValidateEventName(eventName)
	}
}
