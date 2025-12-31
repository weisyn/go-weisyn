package event

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testEvent 测试事件实现
type testEvent struct {
	eventType string
	data      interface{}
}

func (e *testEvent) Type() string {
	return e.eventType
}

func (e *testEvent) Data() interface{} {
	return e.data
}

// testValidationRule 测试验证规则
type testValidationRule struct {
	id           string
	name         string
	description  string
	priority     int
	enabled      bool
	validateFunc func(ctx context.Context, event Event) error
}

func (r *testValidationRule) GetID() string           { return r.id }
func (r *testValidationRule) GetName() string         { return r.name }
func (r *testValidationRule) GetDescription() string  { return r.description }
func (r *testValidationRule) GetPriority() int        { return r.priority }
func (r *testValidationRule) IsEnabled() bool         { return r.enabled }
func (r *testValidationRule) SetEnabled(enabled bool) { r.enabled = enabled }

func (r *testValidationRule) Validate(ctx context.Context, event Event) error {
	if !r.enabled {
		return nil
	}
	if r.validateFunc != nil {
		return r.validateFunc(ctx, event)
	}
	return nil
}

func TestNewBasicEventValidator(t *testing.T) {
	t.Run("使用默认配置创建验证器", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, nil)

		assert.NotNil(t, validator)
		assert.NotNil(t, validator.config)
		assert.True(t, validator.config.EnableNameValidation)
		assert.True(t, validator.config.EnableDataValidation)
		assert.True(t, validator.config.EnableRuleValidation)
		assert.Equal(t, 100, validator.config.MaxConcurrentValidations)
		assert.Equal(t, 5*time.Second, validator.config.ValidationTimeout)
	})

	t.Run("使用自定义配置创建验证器", func(t *testing.T) {
		config := &ValidatorConfig{
			EnableNameValidation:     false,
			EnableDataValidation:     true,
			MaxConcurrentValidations: 50,
			ValidationTimeout:        10 * time.Second,
		}

		validator := NewBasicEventValidator(&mockLogger{}, config)

		assert.NotNil(t, validator)
		assert.Equal(t, config, validator.config)
		assert.False(t, validator.config.EnableNameValidation)
		assert.True(t, validator.config.EnableDataValidation)
		assert.Equal(t, 50, validator.config.MaxConcurrentValidations)
		assert.Equal(t, 10*time.Second, validator.config.ValidationTimeout)
	})
}

func TestBasicEventValidator_ValidateEvent(t *testing.T) {
	validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

	t.Run("验证有效事件", func(t *testing.T) {
		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      map[string]interface{}{"blockHeight": 123},
		}

		err := validator.ValidateEvent(event)
		assert.NoError(t, err)
	})

	t.Run("验证nil事件应该失败", func(t *testing.T) {
		err := validator.ValidateEvent(nil)
		assert.Error(t, err)

		validationErr, ok := err.(*ValidationError)
		assert.True(t, ok)
		assert.Equal(t, "NIL_EVENT", validationErr.Code)
	})

	t.Run("验证无效事件名称", func(t *testing.T) {
		event := &testEvent{
			eventType: "invalid-event-name",
			data:      "some data",
		}

		err := validator.ValidateEvent(event)
		assert.Error(t, err)
	})

	t.Run("验证过大的事件数据", func(t *testing.T) {
		largeData := strings.Repeat("a", MaxEventDataSize+1)
		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      largeData,
		}

		err := validator.ValidateEvent(event)
		assert.Error(t, err)
	})

	t.Run("禁用名称验证", func(t *testing.T) {
		config := DefaultValidatorConfig()
		config.EnableNameValidation = false
		disabledValidator := NewBasicEventValidator(&mockLogger{}, config)

		event := &testEvent{
			eventType: "invalid-event-name",
			data:      "some data",
		}

		err := disabledValidator.ValidateEvent(event)
		assert.NoError(t, err)
	})

	t.Run("禁用数据验证", func(t *testing.T) {
		config := DefaultValidatorConfig()
		config.EnableDataValidation = false
		disabledValidator := NewBasicEventValidator(&mockLogger{}, config)

		largeData := strings.Repeat("a", MaxEventDataSize+1)
		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      largeData,
		}

		err := disabledValidator.ValidateEvent(event)
		assert.NoError(t, err)
	})
}

func TestBasicEventValidator_ValidateEventWithContext(t *testing.T) {
	validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

	t.Run("带上下文验证事件", func(t *testing.T) {
		ctx := context.Background()
		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "valid data",
		}

		err := validator.ValidateEventWithContext(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("上下文超时", func(t *testing.T) {
		config := DefaultValidatorConfig()
		config.ValidationTimeout = 1 * time.Millisecond
		validator := NewBasicEventValidator(&mockLogger{}, config)

		// 添加一个慢验证规则
		slowRule := &testValidationRule{
			id:       "slow_rule",
			name:     "Slow Rule",
			priority: 1,
			enabled:  true,
			validateFunc: func(ctx context.Context, event Event) error {
				select {
				case <-time.After(100 * time.Millisecond):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		}

		validator.AddRule(slowRule)

		ctx := context.Background()
		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "test data",
		}

		err := validator.ValidateEventWithContext(ctx, event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "slow_rule failed")
	})
}

func TestBasicEventValidator_ValidateEventWithDomain(t *testing.T) {
	validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
	registry := NewDomainRegistry(&mockLogger{})

	// 注册一个域
	err := registry.RegisterDomain("blockchain", DomainInfo{
		Component:   "blockchain-service",
		Description: "Blockchain events",
	})
	require.NoError(t, err)

	t.Run("验证已注册域的事件", func(t *testing.T) {
		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "test data",
		}

		err := validator.ValidateEventWithDomain(event, registry, true)
		assert.NoError(t, err)
	})

	t.Run("严格模式下验证未注册域的事件应该失败", func(t *testing.T) {
		event := &testEvent{
			eventType: "unregistered.entity.action",
			data:      "test data",
		}

		err := validator.ValidateEventWithDomain(event, registry, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "domain validation failed")
	})

	t.Run("非严格模式下验证未注册域的事件", func(t *testing.T) {
		event := &testEvent{
			eventType: "unregistered.entity.action",
			data:      "test data",
		}

		err := validator.ValidateEventWithDomain(event, registry, false)
		assert.NoError(t, err)
	})

	t.Run("空域注册表应该跳过域验证", func(t *testing.T) {
		event := &testEvent{
			eventType: "any.entity.action",
			data:      "test data",
		}

		err := validator.ValidateEventWithDomain(event, nil, true)
		assert.NoError(t, err)
	})
}

func TestBasicEventValidator_Rules(t *testing.T) {
	t.Run("添加验证规则", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
		rule := &testValidationRule{
			id:          "test_rule_1",
			name:        "Test Rule 1",
			description: "A test validation rule",
			priority:    10,
			enabled:     true,
		}

		err := validator.AddRule(rule)
		assert.NoError(t, err)

		rules := validator.GetRules()
		assert.Len(t, rules, 1)
		assert.Equal(t, "test_rule_1", rules[0].GetID())
	})

	t.Run("添加重复ID的规则应该失败", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
		rule1 := &testValidationRule{id: "duplicate_rule", priority: 1, enabled: true}
		rule2 := &testValidationRule{id: "duplicate_rule", priority: 2, enabled: true}

		err := validator.AddRule(rule1)
		assert.NoError(t, err)

		err = validator.AddRule(rule2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("添加nil规则应该失败", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
		err := validator.AddRule(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("添加空ID规则应该失败", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
		rule := &testValidationRule{
			id:       "",
			priority: 1,
			enabled:  true,
		}

		err := validator.AddRule(rule)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("移除验证规则", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
		rule := &testValidationRule{
			id:       "removable_rule",
			priority: 1,
			enabled:  true,
		}

		err := validator.AddRule(rule)
		require.NoError(t, err)

		rules := validator.GetRules()
		assert.Len(t, rules, 1) // 只有当前添加的一个

		err = validator.RemoveRule("removable_rule")
		assert.NoError(t, err)

		rules = validator.GetRules()
		assert.Len(t, rules, 0)
	})

	t.Run("移除不存在的规则应该失败", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
		err := validator.RemoveRule("nonexistent_rule")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("规则优先级排序", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

		// 添加不同优先级的规则
		rule1 := &testValidationRule{id: "rule_priority_3", priority: 3, enabled: true}
		rule2 := &testValidationRule{id: "rule_priority_1", priority: 1, enabled: true}
		rule3 := &testValidationRule{id: "rule_priority_2", priority: 2, enabled: true}

		validator.AddRule(rule1)
		validator.AddRule(rule2)
		validator.AddRule(rule3)

		rules := validator.GetRules()
		assert.Len(t, rules, 3)

		// 验证按优先级排序（数字越小优先级越高）
		assert.Equal(t, "rule_priority_1", rules[0].GetID())
		assert.Equal(t, "rule_priority_2", rules[1].GetID())
		assert.Equal(t, "rule_priority_3", rules[2].GetID())
	})
}

func TestBasicEventValidator_RuleExecution(t *testing.T) {
	t.Run("执行验证规则", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

		var executionOrder []string
		var mu sync.Mutex

		// 创建记录执行顺序的规则
		rule1 := &testValidationRule{
			id:       "rule_1",
			priority: 1,
			enabled:  true,
			validateFunc: func(ctx context.Context, event Event) error {
				mu.Lock()
				executionOrder = append(executionOrder, "rule_1")
				mu.Unlock()
				return nil
			},
		}

		rule2 := &testValidationRule{
			id:       "rule_2",
			priority: 2,
			enabled:  true,
			validateFunc: func(ctx context.Context, event Event) error {
				mu.Lock()
				executionOrder = append(executionOrder, "rule_2")
				mu.Unlock()
				return nil
			},
		}

		validator.AddRule(rule2) // 先添加优先级低的
		validator.AddRule(rule1) // 后添加优先级高的

		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "test data",
		}

		err := validator.ValidateEvent(event)
		assert.NoError(t, err)

		// 验证执行顺序
		mu.Lock()
		assert.Equal(t, []string{"rule_1", "rule_2"}, executionOrder)
		mu.Unlock()
	})

	t.Run("禁用的规则不应该执行", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

		executed := false
		rule := &testValidationRule{
			id:       "disabled_rule",
			priority: 1,
			enabled:  false,
			validateFunc: func(ctx context.Context, event Event) error {
				executed = true
				return nil
			},
		}

		validator.AddRule(rule)

		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "test data",
		}

		err := validator.ValidateEvent(event)
		assert.NoError(t, err)
		assert.False(t, executed)
	})

	t.Run("规则验证失败", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

		rule := &testValidationRule{
			id:       "failing_rule",
			priority: 1,
			enabled:  true,
			validateFunc: func(ctx context.Context, event Event) error {
				return fmt.Errorf("validation failed")
			},
		}

		validator.AddRule(rule)

		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "test data",
		}

		err := validator.ValidateEvent(event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failing_rule failed")
	})

	t.Run("快速失败模式", func(t *testing.T) {
		config := DefaultValidatorConfig()
		config.FailFast = true
		validator := NewBasicEventValidator(&mockLogger{}, config)

		var executionOrder []string
		var mu sync.Mutex

		rule1 := &testValidationRule{
			id:       "failing_rule",
			priority: 1,
			enabled:  true,
			validateFunc: func(ctx context.Context, event Event) error {
				mu.Lock()
				executionOrder = append(executionOrder, "failing_rule")
				mu.Unlock()
				return fmt.Errorf("first rule failed")
			},
		}

		rule2 := &testValidationRule{
			id:       "second_rule",
			priority: 2,
			enabled:  true,
			validateFunc: func(ctx context.Context, event Event) error {
				mu.Lock()
				executionOrder = append(executionOrder, "second_rule")
				mu.Unlock()
				return nil
			},
		}

		validator.AddRule(rule1)
		validator.AddRule(rule2)

		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "test data",
		}

		err := validator.ValidateEvent(event)
		assert.Error(t, err)

		// 验证只执行了第一个规则
		mu.Lock()
		assert.Equal(t, []string{"failing_rule"}, executionOrder)
		mu.Unlock()
	})
}

func TestBasicEventValidator_BatchValidate(t *testing.T) {
	validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

	t.Run("批量验证事件", func(t *testing.T) {
		events := []Event{
			&testEvent{eventType: "blockchain.block.produced", data: "data1"},
			&testEvent{eventType: "mempool.tx.added", data: "data2"},
			&testEvent{eventType: "consensus.round.completed", data: "data3"},
		}

		results := validator.BatchValidate(events)
		assert.Len(t, results, 3)

		for i, result := range results {
			assert.Equal(t, events[i].Type(), result.EventType)
			assert.True(t, result.Valid)
			assert.Empty(t, result.Errors)
			assert.Greater(t, result.Duration, time.Duration(0))
		}
	})

	t.Run("批量验证包含无效事件", func(t *testing.T) {
		events := []Event{
			&testEvent{eventType: "blockchain.block.produced", data: "data1"},
			&testEvent{eventType: "invalid-event-name", data: "data2"}, // 无效
			&testEvent{eventType: "consensus.round.completed", data: "data3"},
		}

		results := validator.BatchValidate(events)
		assert.Len(t, results, 3)

		assert.True(t, results[0].Valid)
		assert.False(t, results[1].Valid) // 应该是无效的
		assert.NotEmpty(t, results[1].Errors)
		assert.True(t, results[2].Valid)
	})

	t.Run("禁用批量验证", func(t *testing.T) {
		config := DefaultValidatorConfig()
		config.EnableBatchValidation = false
		validator := NewBasicEventValidator(&mockLogger{}, config)

		events := []Event{
			&testEvent{eventType: "blockchain.block.produced", data: "data1"},
		}

		results := validator.BatchValidate(events)
		assert.Nil(t, results)
	})

	t.Run("空事件列表", func(t *testing.T) {
		results := validator.BatchValidate([]Event{})
		assert.Nil(t, results)
	})
}

func TestBasicEventValidator_Statistics(t *testing.T) {
	t.Run("初始统计信息", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
		stats := validator.GetStatistics()
		assert.NotNil(t, stats)
		assert.Equal(t, uint64(0), stats.TotalValidations)
		assert.Equal(t, uint64(0), stats.SuccessValidations)
		assert.Equal(t, uint64(0), stats.FailedValidations)
		assert.Equal(t, time.Duration(0), stats.AverageLatency)
		assert.NotNil(t, stats.RuleStatistics)
		assert.Nil(t, stats.LastValidation)
	})

	t.Run("验证后统计信息更新", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "test data",
		}

		err := validator.ValidateEvent(event)
		assert.NoError(t, err)

		stats := validator.GetStatistics()
		assert.Equal(t, uint64(1), stats.TotalValidations)
		assert.Equal(t, uint64(1), stats.SuccessValidations)
		assert.Equal(t, uint64(0), stats.FailedValidations)
		assert.GreaterOrEqual(t, stats.AverageLatency, time.Duration(0))
		assert.NotNil(t, stats.LastValidation)
	})

	t.Run("失败验证统计", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
		// 先执行一次成功验证
		validEvent := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "test data",
		}
		validator.ValidateEvent(validEvent)

		invalidEvent := &testEvent{
			eventType: "invalid-event-name",
			data:      "test data",
		}

		err := validator.ValidateEvent(invalidEvent)
		assert.Error(t, err)

		stats := validator.GetStatistics()
		assert.Equal(t, uint64(2), stats.TotalValidations) // 前面有一次成功
		assert.Equal(t, uint64(1), stats.SuccessValidations)
		assert.Equal(t, uint64(1), stats.FailedValidations)
	})

	t.Run("规则统计", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

		rule := &testValidationRule{
			id:       "counting_rule",
			priority: 1,
			enabled:  true,
			validateFunc: func(ctx context.Context, event Event) error {
				return fmt.Errorf("always fails")
			},
		}

		validator.AddRule(rule)

		event := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "test data",
		}

		// 执行几次验证
		for i := 0; i < 3; i++ {
			validator.ValidateEvent(event)
		}

		stats := validator.GetStatistics()
		assert.Equal(t, uint64(3), stats.RuleStatistics["counting_rule"])
	})
}

func TestPreDefinedRules(t *testing.T) {
	t.Run("BasicNameFormatRule", func(t *testing.T) {
		rule := NewBasicNameFormatRule()

		assert.Equal(t, "basic_name_format", rule.GetID())
		assert.Equal(t, "Basic Event Name Format", rule.GetName())
		assert.NotEmpty(t, rule.GetDescription())
		assert.Equal(t, 1, rule.GetPriority())
		assert.True(t, rule.IsEnabled())

		// 测试验证
		ctx := context.Background()

		validEvent := &testEvent{eventType: "blockchain.block.produced"}
		err := rule.Validate(ctx, validEvent)
		assert.NoError(t, err)

		invalidEvent := &testEvent{eventType: "invalid-name"}
		err = rule.Validate(ctx, invalidEvent)
		assert.Error(t, err)

		// 测试禁用规则
		rule.SetEnabled(false)
		err = rule.Validate(ctx, invalidEvent)
		assert.NoError(t, err)
	})

	t.Run("DataSizeRule", func(t *testing.T) {
		maxSize := 100
		rule := NewDataSizeRule(maxSize)

		assert.Equal(t, "data_size_limit", rule.GetID())
		assert.Equal(t, "Event Data Size Limit", rule.GetName())
		assert.Contains(t, rule.GetDescription(), "100 bytes")
		assert.Equal(t, 2, rule.GetPriority())
		assert.True(t, rule.IsEnabled())

		// 测试验证
		ctx := context.Background()

		smallEvent := &testEvent{
			eventType: "blockchain.block.produced",
			data:      "small data",
		}
		err := rule.Validate(ctx, smallEvent)
		assert.NoError(t, err)

		largeEvent := &testEvent{
			eventType: "blockchain.block.produced",
			data:      strings.Repeat("a", maxSize+1),
		}
		err = rule.Validate(ctx, largeEvent)
		assert.Error(t, err)

		validationErr, ok := err.(*ValidationError)
		assert.True(t, ok)
		assert.Equal(t, "DATA_SIZE_EXCEEDED", validationErr.Code)

		// 测试nil数据
		nilEvent := &testEvent{
			eventType: "blockchain.block.produced",
			data:      nil,
		}
		err = rule.Validate(ctx, nilEvent)
		assert.NoError(t, err)
	})
}

func TestNewEventValidatorWithDefaultRules(t *testing.T) {
	t.Run("创建带默认规则的验证器", func(t *testing.T) {
		validator, err := NewEventValidatorWithDefaultRules(&mockLogger{}, DefaultValidatorConfig())
		assert.NoError(t, err)
		assert.NotNil(t, validator)

		rules := validator.GetRules()
		assert.Len(t, rules, 2) // 应该有两个默认规则

		// 验证规则ID
		ruleIDs := make([]string, len(rules))
		for i, rule := range rules {
			ruleIDs[i] = rule.GetID()
		}
		assert.Contains(t, ruleIDs, "basic_name_format")
		assert.Contains(t, ruleIDs, "data_size_limit")
	})
}

func TestBasicEventValidator_ConcurrentAccess(t *testing.T) {
	t.Run("并发验证事件", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

		numGoroutines := 10
		numValidations := 100

		var wg sync.WaitGroup
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				for j := 0; j < numValidations; j++ {
					event := &testEvent{
						eventType: "blockchain.block.produced",
						data:      fmt.Sprintf("data-%d-%d", idx, j),
					}

					if err := validator.ValidateEvent(event); err != nil {
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

		// 验证统计信息
		stats := validator.GetStatistics()
		expectedTotal := uint64(numGoroutines * numValidations)
		assert.Equal(t, expectedTotal, stats.TotalValidations)
		assert.Equal(t, expectedTotal, stats.SuccessValidations)
		assert.Equal(t, uint64(0), stats.FailedValidations)
	})

	t.Run("并发添加和移除规则", func(t *testing.T) {
		validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

		numGoroutines := 5
		var wg sync.WaitGroup

		// 并发添加规则
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				rule := &testValidationRule{
					id:       fmt.Sprintf("concurrent_rule_%d", idx),
					priority: idx,
					enabled:  true,
				}

				err := validator.AddRule(rule)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// 验证所有规则都添加成功
		rules := validator.GetRules()
		assert.Len(t, rules, numGoroutines)

		// 并发移除规则
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ruleID := fmt.Sprintf("concurrent_rule_%d", idx)
				err := validator.RemoveRule(ruleID)
				assert.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// 验证所有规则都移除成功
		rules = validator.GetRules()
		assert.Len(t, rules, 0)
	})
}

// BenchmarkBasicEventValidator_ValidateEvent 性能基准测试
func BenchmarkBasicEventValidator_ValidateEvent(b *testing.B) {
	validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())
	event := &testEvent{
		eventType: "blockchain.block.produced",
		data:      map[string]interface{}{"height": 123, "hash": "abc123"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateEvent(event)
	}
}

// BenchmarkBasicEventValidator_BatchValidate 批量验证性能基准测试
func BenchmarkBasicEventValidator_BatchValidate(b *testing.B) {
	validator := NewBasicEventValidator(&mockLogger{}, DefaultValidatorConfig())

	events := make([]Event, 100)
	for i := 0; i < 100; i++ {
		events[i] = &testEvent{
			eventType: "blockchain.block.produced",
			data:      fmt.Sprintf("data-%d", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.BatchValidate(events)
	}
}
