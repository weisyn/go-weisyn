package event

import (
	"testing"
	"time"

	"github.com/weisyn/v1/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestEnhancedInterfaces 测试增强接口的定义
func TestEnhancedInterfaces(t *testing.T) {
	t.Run("接口类型验证", func(t *testing.T) {
		// 验证接口类型定义正确
		var enhancedBus EnhancedEventBus
		var domainRegistry DomainRegistry
		var eventRouter EventRouter
		var eventValidator EventValidator
		var coordinator EventCoordinator

		// 这些应该都是接口类型
		assert.Nil(t, enhancedBus)
		assert.Nil(t, domainRegistry)
		assert.Nil(t, eventRouter)
		assert.Nil(t, eventValidator)
		assert.Nil(t, coordinator)
	})
}

// TestDomainInfo 测试域信息结构
func TestDomainInfo(t *testing.T) {
	t.Run("创建域信息", func(t *testing.T) {
		info := NewDomainInfo("test_domain", "test_component", "测试域")

		assert.Equal(t, "test_domain", info.Name)
		assert.Equal(t, "test_component", info.Component)
		assert.Equal(t, "测试域", info.Description)
		assert.True(t, info.Active)
		assert.NotZero(t, info.RegisteredAt)
		assert.Empty(t, info.EventTypes)
	})

	t.Run("域信息JSON序列化", func(t *testing.T) {
		info := DomainInfo{
			Name:         "blockchain",
			Component:    "blockchain_module",
			Description:  "区块链事件域",
			EventTypes:   []string{"block.produced", "block.confirmed"},
			RegisteredAt: time.Now(),
			Active:       true,
		}

		// 验证结构体字段标签
		assert.Contains(t, info.Name, "blockchain")
		assert.Contains(t, info.Component, "blockchain_module")
		assert.Len(t, info.EventTypes, 2)
	})
}

// TestRouteStrategy 测试路由策略常量
func TestRouteStrategy(t *testing.T) {
	t.Run("路由策略常量", func(t *testing.T) {
		assert.Equal(t, RouteStrategy("direct"), RouteStrategyDirect)
		assert.Equal(t, RouteStrategy("broadcast"), RouteStrategyBroadcast)
		assert.Equal(t, RouteStrategy("round_robin"), RouteStrategyRoundRobin)
		assert.Equal(t, RouteStrategy("priority"), RouteStrategyPriority)
		assert.Equal(t, RouteStrategy("filter"), RouteStrategyFilter)
	})
}

// TestPriorityConstants 测试优先级常量
func TestPriorityConstants(t *testing.T) {
	t.Run("优先级常量值", func(t *testing.T) {
		assert.Equal(t, Priority(4), PriorityCritical)
		assert.Equal(t, Priority(3), PriorityHigh)
		assert.Equal(t, Priority(2), PriorityNormal)
		assert.Equal(t, Priority(1), PriorityLow)

		// 验证优先级排序
		assert.True(t, PriorityCritical > PriorityHigh)
		assert.True(t, PriorityHigh > PriorityNormal)
		assert.True(t, PriorityNormal > PriorityLow)
	})
}

// TestSubscriptionOptions 测试订阅选项
func TestSubscriptionOptions(t *testing.T) {
	t.Run("订阅选项构造", func(t *testing.T) {
		config := &SubscriptionConfig{}

		// 测试优先级选项
		WithPriority(PriorityHigh)(config)
		assert.Equal(t, PriorityHigh, config.Priority)

		// 测试组件选项
		WithComponent("test_component")(config)
		assert.Equal(t, "test_component", config.Component)

		// 测试元数据选项
		metadata := map[string]interface{}{"key": "value"}
		WithMetadata(metadata)(config)
		assert.Equal(t, metadata, config.Metadata)
	})

	t.Run("组合订阅选项", func(t *testing.T) {
		config := &SubscriptionConfig{}

		// 应用多个选项
		options := []SubscriptionOption{
			WithPriority(PriorityCritical),
			WithComponent("blockchain"),
			WithMetadata(map[string]interface{}{"domain": "blockchain"}),
		}

		for _, opt := range options {
			opt(config)
		}

		assert.Equal(t, PriorityCritical, config.Priority)
		assert.Equal(t, "blockchain", config.Component)
		assert.Equal(t, "blockchain", config.Metadata["domain"])
	})
}

// TestPublishOptions 测试发布选项
func TestPublishOptions(t *testing.T) {
	t.Run("发布选项构造", func(t *testing.T) {
		config := &PublishConfig{}

		// 测试各种发布选项
		WithPublishPriority(PriorityHigh)(config)
		assert.Equal(t, PriorityHigh, config.Priority)

		WithPublishComponent("test_publisher")(config)
		assert.Equal(t, "test_publisher", config.Component)

		WithAsync(true)(config)
		assert.True(t, config.Async)

		timeout := 30 * time.Second
		WithTimeout(timeout)(config)
		assert.Equal(t, timeout, config.Timeout)

		WithRetry(3)(config)
		assert.Equal(t, 3, config.RetryCount)

		metadata := map[string]interface{}{"source": "test"}
		WithPublishMetadata(metadata)(config)
		assert.Equal(t, metadata, config.Metadata)
	})
}

// TestEventData 测试事件数据结构
func TestEventData(t *testing.T) {
	t.Run("创建基础事件数据", func(t *testing.T) {
		data := NewEventData("test.event", "test_payload")

		assert.Equal(t, "test.event", data.Type)
		assert.Equal(t, "test_payload", data.Data)
		assert.NotNil(t, data.Metadata)
		assert.Empty(t, data.Metadata)
	})

	t.Run("创建带元数据的事件数据", func(t *testing.T) {
		metadata := map[string]interface{}{
			"source":    "test_component",
			"timestamp": time.Now().Unix(),
		}

		data := NewEventDataWithMetadata("test.event", "test_payload", metadata)

		assert.Equal(t, "test.event", data.Type)
		assert.Equal(t, "test_payload", data.Data)
		assert.Equal(t, metadata, data.Metadata)
		assert.Equal(t, "test_component", data.Metadata["source"])
	})
}

// ❌ **已删除：TestHealthStatus - 健康状态测试**
//
// 🚨 **删除原因**：
// 测试代码引用了已删除的监控结构体和类型：
// - HealthLevel 类型已删除（healthy/warning/critical/unknown等级别）
// - HealthStatus 结构体已删除（健康状态监控）
// - 相关常量已删除（HealthHealthy/HealthWarning/HealthCritical/HealthUnknown）
//
// 🎯 **符合项目偏好**：
// 删除健康状态测试符合"接口不暴露指标"原则，自治系统不需要对外暴露健康监控

// ❌ **已删除：TestStatisticsStructures - 统计结构体测试**
//
// 🚨 **删除原因**：
// 测试代码引用了所有已删除的监控统计结构体：
// - EventStatistics - 事件统计信息（9个字段的详细统计）
// - RegistryStatistics - 注册中心统计信息（6个统计字段）
// - RouterStatistics - 路由器统计信息（8个详细统计）
// - ValidatorStatistics - 验证器统计信息（7个统计字段）
// - CoordinatorStatistics - 协调器统计信息（9个复杂统计字段）
//
// 🎯 **清理内容**：
// 删除了所有统计结构体测试，包括：
// - 50+个监控字段的创建和验证测试
// - 各种统计计算和聚合功能的测试
// - 健康状态和性能指标的测试
// - 时间追踪和分类统计的测试
//
// 🔧 **符合项目偏好**：
// 删除统计测试完全符合"接口不暴露指标"原则：
// 1. 统计监控增加系统复杂度而无实际价值
// 2. 自治系统应该内部处理问题，不需要外部统计
// 3. 这些统计数据没有明确的消费者和使用场景

// MockValidationRule 模拟验证规则用于测试
type MockValidationRule struct {
	mock.Mock
}

func (m *MockValidationRule) GetID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockValidationRule) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockValidationRule) Validate(event Event) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockValidationRule) GetDescription() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockValidationRule) IsEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

// TestValidationRule 测试验证规则接口
func TestValidationRule(t *testing.T) {
	t.Run("模拟验证规则", func(t *testing.T) {
		mockRule := new(MockValidationRule)

		// 设置期望的调用和返回值
		mockRule.On("GetID").Return("test_rule_001")
		mockRule.On("GetName").Return("测试验证规则")
		mockRule.On("GetDescription").Return("用于测试的验证规则")
		mockRule.On("IsEnabled").Return(true)

		// 验证接口实现
		var rule ValidationRule = mockRule

		assert.Equal(t, "test_rule_001", rule.GetID())
		assert.Equal(t, "测试验证规则", rule.GetName())
		assert.Equal(t, "用于测试的验证规则", rule.GetDescription())
		assert.True(t, rule.IsEnabled())

		// 验证所有期望的调用都被执行
		mockRule.AssertExpectations(t)
	})
}

// TestEventTypeConstants 测试事件类型常量
func TestEventTypeConstants(t *testing.T) {
	t.Run("系统事件常量", func(t *testing.T) {
		assert.Equal(t, EventType("system.startup"), EventTypeSystemStartup)
		assert.Equal(t, EventType("system.shutdown"), EventTypeSystemShutdown)
		assert.Equal(t, EventType("system.error"), EventTypeSystemError)
	})

	t.Run("网络事件常量", func(t *testing.T) {
		assert.Equal(t, EventType("network.peer.connected"), EventTypeNetworkPeerConnected)
		assert.Equal(t, EventType("network.peer.disconnected"), EventTypeNetworkPeerDisconnected)
		assert.Equal(t, EventType("network.message.received"), EventTypeNetworkMessageReceived)
		assert.Equal(t, EventType("network.message.sent"), EventTypeNetworkMessageSent)
		assert.Equal(t, EventType("network.quality.changed"), EventTypeNetworkQualityChanged)
	})

	t.Run("区块链事件常量", func(t *testing.T) {
		assert.Equal(t, EventType("blockchain.block.produced"), EventTypeBlockProduced)
		assert.Equal(t, EventType("blockchain.block.validated"), EventTypeBlockValidated)
		assert.Equal(t, EventType("blockchain.block.processed"), EventTypeBlockProcessed)
		assert.Equal(t, EventType("blockchain.block.confirmed"), EventTypeBlockConfirmed)
		assert.Equal(t, EventType("blockchain.block.reverted"), EventTypeBlockReverted)
		assert.Equal(t, EventType("blockchain.block.finalized"), EventTypeBlockFinalized)
	})

	t.Run("链状态事件常量", func(t *testing.T) {
		assert.Equal(t, EventType("blockchain.chain.height_changed"), EventTypeChainHeightChanged)
		assert.Equal(t, EventType("blockchain.chain.state_updated"), EventTypeChainStateUpdated)
		assert.Equal(t, EventType("blockchain.chain.reorganized"), EventTypeChainReorganized)
	})
}

// TestCompatibilityTypes 测试兼容性类型别名
func TestCompatibilityTypes(t *testing.T) {
	t.Run("类型别名验证", func(t *testing.T) {
		// 验证类型别名正确定义
		var eventType EventType = types.EventType("test")
		var protocolType ProtocolType = types.ProtocolType("test_protocol")
		var subscriptionID SubscriptionID = types.SubscriptionID("sub_001")
		var priority Priority = types.Priority(2)

		assert.Equal(t, "test", string(eventType))
		assert.Equal(t, "test_protocol", string(protocolType))
		assert.Equal(t, "sub_001", string(subscriptionID))
		assert.Equal(t, types.Priority(2), priority)
	})
}
