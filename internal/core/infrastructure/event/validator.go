// Package event 事件验证器实现
package event

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// EventValidator 事件验证器接口
type EventValidator interface {
	// ValidateEvent 验证事件（包括名称、数据和元数据）
	ValidateEvent(event Event) error

	// ValidateEventWithContext 带上下文的事件验证
	ValidateEventWithContext(ctx context.Context, event Event) error

	// ValidateEventName 验证事件名称
	ValidateEventName(eventName string) error

	// ValidateEventData 验证事件数据
	ValidateEventData(data interface{}) error

	// ValidateEventWithDomain 验证事件并检查域
	ValidateEventWithDomain(event Event, domainRegistry *DomainRegistry, strictMode bool) error

	// AddRule 添加自定义验证规则
	AddRule(rule ValidationRule) error

	// RemoveRule 移除验证规则
	RemoveRule(ruleID string) error

	// GetRules 获取所有验证规则
	GetRules() []ValidationRule

	// BatchValidate 批量验证事件
	BatchValidate(events []Event) []ValidationResult

	// GetStatistics 获取验证统计信息
	GetStatistics() *ValidatorStatistics
}

// Event 事件接口（基础定义，需要与pkg/interfaces保持一致）
type Event interface {
	Type() string
	Data() interface{}
}

// ValidationRule 验证规则接口
type ValidationRule interface {
	// GetID 获取规则ID
	GetID() string

	// GetName 获取规则名称
	GetName() string

	// GetDescription 获取规则描述
	GetDescription() string

	// Validate 执行验证
	Validate(ctx context.Context, event Event) error

	// GetPriority 获取优先级（数字越小优先级越高）
	GetPriority() int

	// IsEnabled 检查规则是否启用
	IsEnabled() bool

	// SetEnabled 设置规则启用状态
	SetEnabled(enabled bool)
}

// ValidationResult 验证结果
type ValidationResult struct {
	EventType string            `json:"event_type"`
	Valid     bool              `json:"valid"`
	Errors    []ValidationError `json:"errors,omitempty"`
	Warnings  []string          `json:"warnings,omitempty"`
	Duration  time.Duration     `json:"duration"`
	RuleID    string            `json:"rule_id,omitempty"`
}

// ValidatorStatistics 验证器统计信息
type ValidatorStatistics struct {
	TotalValidations   uint64            `json:"total_validations"`
	SuccessValidations uint64            `json:"success_validations"`
	FailedValidations  uint64            `json:"failed_validations"`
	AverageLatency     time.Duration     `json:"average_latency"`
	RuleStatistics     map[string]uint64 `json:"rule_statistics"`
	LastValidation     *time.Time        `json:"last_validation,omitempty"`
}

// BasicEventValidator 基础事件验证器实现
type BasicEventValidator struct {
	mu        sync.RWMutex
	rules     map[string]ValidationRule // ruleID -> rule
	ruleOrder []string                  // 规则执行顺序
	stats     *ValidatorStatistics
	logger    log.Logger

	// 配置
	config *ValidatorConfig
}

// ValidatorConfig 验证器配置
type ValidatorConfig struct {
	// 基础验证开关
	EnableNameValidation bool `json:"enable_name_validation"`
	EnableDataValidation bool `json:"enable_data_validation"`
	EnableRuleValidation bool `json:"enable_rule_validation"`

	// 性能配置
	MaxConcurrentValidations int           `json:"max_concurrent_validations"`
	ValidationTimeout        time.Duration `json:"validation_timeout"`
	EnableBatchValidation    bool          `json:"enable_batch_validation"`
	BatchSize                int           `json:"batch_size"`

	// 域验证配置
	StrictDomainCheck      bool `json:"strict_domain_check"`
	RequireDomainExistence bool `json:"require_domain_existence"`

	// 缓存配置
	EnableValidationCache bool          `json:"enable_validation_cache"`
	CacheSize             int           `json:"cache_size"`
	CacheTTL              time.Duration `json:"cache_ttl"`

	// 错误处理
	FailFast          bool `json:"fail_fast"`           // 遇到第一个错误立即返回
	CollectWarnings   bool `json:"collect_warnings"`    // 收集警告信息
	IgnoreMinorErrors bool `json:"ignore_minor_errors"` // 忽略轻微错误
}

// DefaultValidatorConfig 默认验证器配置
func DefaultValidatorConfig() *ValidatorConfig {
	return &ValidatorConfig{
		EnableNameValidation:     true,
		EnableDataValidation:     true,
		EnableRuleValidation:     true,
		MaxConcurrentValidations: 100,
		ValidationTimeout:        5 * time.Second,
		EnableBatchValidation:    true,
		BatchSize:                50,
		StrictDomainCheck:        false,
		RequireDomainExistence:   false,
		EnableValidationCache:    true,
		CacheSize:                1000,
		CacheTTL:                 10 * time.Minute,
		FailFast:                 false,
		CollectWarnings:          true,
		IgnoreMinorErrors:        false,
	}
}

// NewBasicEventValidator 创建基础事件验证器
func NewBasicEventValidator(logger log.Logger, config *ValidatorConfig) *BasicEventValidator {
	if config == nil {
		config = DefaultValidatorConfig()
	}

	var componentLogger log.Logger
	if logger != nil {
		componentLogger = logger.With("component", "event_validator")
	}

	return &BasicEventValidator{
		rules:     make(map[string]ValidationRule),
		ruleOrder: make([]string, 0),
		stats: &ValidatorStatistics{
			RuleStatistics: make(map[string]uint64),
		},
		logger: componentLogger,
		config: config,
	}
}

// ValidateEvent 验证事件
func (v *BasicEventValidator) ValidateEvent(event Event) error {
	return v.ValidateEventWithContext(context.Background(), event)
}

// ValidateEventWithContext 带上下文的事件验证
func (v *BasicEventValidator) ValidateEventWithContext(ctx context.Context, event Event) error {
	if event == nil {
		return NewValidationError("event cannot be nil", "NIL_EVENT")
	}

	startTime := time.Now()
	var validationSuccess bool = true
	defer func() {
		v.updateStatistics(time.Since(startTime), validationSuccess)
	}()

	// 创建验证上下文
	validationCtx, cancel := context.WithTimeout(ctx, v.config.ValidationTimeout)
	defer cancel()

	var errors []error

	// 1. 验证事件名称
	if v.config.EnableNameValidation {
		if err := v.ValidateEventName(event.Type()); err != nil {
			if v.config.FailFast {
				return err
			}
			errors = append(errors, err)
		}
	}

	// 2. 验证事件数据
	if v.config.EnableDataValidation {
		if err := v.ValidateEventData(event.Data()); err != nil {
			if v.config.FailFast {
				return err
			}
			errors = append(errors, err)
		}
	}

	// 3. 执行自定义验证规则
	if v.config.EnableRuleValidation {
		ruleErrors := v.executeValidationRules(validationCtx, event)
		if len(ruleErrors) > 0 {
			if v.config.FailFast && len(ruleErrors) > 0 {
				return ruleErrors[0]
			}
			errors = append(errors, ruleErrors...)
		}
	}

	// 汇总错误
	if len(errors) > 0 {
		validationSuccess = false
		return v.combineErrors(errors)
	}

	v.logger.Debugf("事件验证成功: type=%s", event.Type())
	return nil
}

// ValidateEventName 验证事件名称
func (v *BasicEventValidator) ValidateEventName(eventName string) error {
	return ValidateEventName(eventName)
}

// ValidateEventData 验证事件数据
func (v *BasicEventValidator) ValidateEventData(data interface{}) error {
	return ValidateEventData(data)
}

// ValidateEventWithDomain 验证事件并检查域
func (v *BasicEventValidator) ValidateEventWithDomain(event Event, domainRegistry *DomainRegistry, strictMode bool) error {
	// 首先进行基础验证
	if err := v.ValidateEvent(event); err != nil {
		return err
	}

	// 域验证
	if domainRegistry != nil {
		eventName := event.Type()
		if err := domainRegistry.ValidateEventNameWithDomainCheck(eventName, strictMode); err != nil {
			return fmt.Errorf("domain validation failed: %w", err)
		}
	}

	return nil
}

// executeValidationRules 执行验证规则
func (v *BasicEventValidator) executeValidationRules(ctx context.Context, event Event) []error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var errors []error

	// 按优先级顺序执行规则
	for _, ruleID := range v.ruleOrder {
		rule, exists := v.rules[ruleID]
		if !exists || !rule.IsEnabled() {
			continue
		}

		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			errors = append(errors, fmt.Errorf("validation timeout for rule %s", ruleID))
			return errors
		default:
		}

		// 执行规则验证
		if err := rule.Validate(ctx, event); err != nil {
			v.stats.RuleStatistics[ruleID]++
			errors = append(errors, fmt.Errorf("rule %s failed: %w", ruleID, err))

			if v.config.FailFast {
				return errors
			}
		}
	}

	return errors
}

// AddRule 添加自定义验证规则
func (v *BasicEventValidator) AddRule(rule ValidationRule) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	ruleID := rule.GetID()
	if ruleID == "" {
		return fmt.Errorf("rule ID cannot be empty")
	}

	// 检查规则是否已存在
	if _, exists := v.rules[ruleID]; exists {
		return fmt.Errorf("rule with ID %s already exists", ruleID)
	}

	// 添加规则
	v.rules[ruleID] = rule

	// 根据优先级插入到正确位置
	v.insertRuleByPriority(ruleID, rule.GetPriority())

	// 初始化统计
	v.stats.RuleStatistics[ruleID] = 0

	v.logger.Infof("添加验证规则: id=%s, name=%s, priority=%d",
		ruleID, rule.GetName(), rule.GetPriority())

	return nil
}

// RemoveRule 移除验证规则
func (v *BasicEventValidator) RemoveRule(ruleID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if _, exists := v.rules[ruleID]; !exists {
		return fmt.Errorf("rule with ID %s not found", ruleID)
	}

	// 移除规则
	delete(v.rules, ruleID)

	// 从执行顺序中移除
	for i, id := range v.ruleOrder {
		if id == ruleID {
			v.ruleOrder = append(v.ruleOrder[:i], v.ruleOrder[i+1:]...)
			break
		}
	}

	// 清理统计
	delete(v.stats.RuleStatistics, ruleID)

	v.logger.Infof("移除验证规则: id=%s", ruleID)
	return nil
}

// GetRules 获取所有验证规则
func (v *BasicEventValidator) GetRules() []ValidationRule {
	v.mu.RLock()
	defer v.mu.RUnlock()

	rules := make([]ValidationRule, 0, len(v.rules))
	for _, ruleID := range v.ruleOrder {
		if rule, exists := v.rules[ruleID]; exists {
			rules = append(rules, rule)
		}
	}

	return rules
}

// BatchValidate 批量验证事件
func (v *BasicEventValidator) BatchValidate(events []Event) []ValidationResult {
	if !v.config.EnableBatchValidation || len(events) == 0 {
		return nil
	}

	results := make([]ValidationResult, len(events))

	// 根据配置决定是否并发处理
	if v.config.MaxConcurrentValidations > 1 && len(events) > v.config.BatchSize {
		v.batchValidateConcurrent(events, results)
	} else {
		v.batchValidateSequential(events, results)
	}

	return results
}

// batchValidateSequential 顺序批量验证
func (v *BasicEventValidator) batchValidateSequential(events []Event, results []ValidationResult) {
	for i, event := range events {
		startTime := time.Now()
		err := v.ValidateEvent(event)
		duration := time.Since(startTime)

		results[i] = ValidationResult{
			EventType: event.Type(),
			Valid:     err == nil,
			Duration:  duration,
		}

		if err != nil {
			if validationErr, ok := err.(*ValidationError); ok {
				results[i].Errors = []ValidationError{*validationErr}
			} else {
				results[i].Errors = []ValidationError{{Message: err.Error(), Code: "VALIDATION_ERROR"}}
			}
		}
	}
}

// batchValidateConcurrent 并发批量验证
func (v *BasicEventValidator) batchValidateConcurrent(events []Event, results []ValidationResult) {
	semaphore := make(chan struct{}, v.config.MaxConcurrentValidations)
	var wg sync.WaitGroup

	for i, event := range events {
		wg.Add(1)
		go func(idx int, evt Event) {
			defer wg.Done()

			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			startTime := time.Now()
			err := v.ValidateEvent(evt)
			duration := time.Since(startTime)

			results[idx] = ValidationResult{
				EventType: evt.Type(),
				Valid:     err == nil,
				Duration:  duration,
			}

			if err != nil {
				if validationErr, ok := err.(*ValidationError); ok {
					results[idx].Errors = []ValidationError{*validationErr}
				} else {
					results[idx].Errors = []ValidationError{{Message: err.Error(), Code: "VALIDATION_ERROR"}}
				}
			}
		}(i, event)
	}

	wg.Wait()
}

// GetStatistics 获取验证统计信息
func (v *BasicEventValidator) GetStatistics() *ValidatorStatistics {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// 返回统计信息的副本
	stats := &ValidatorStatistics{
		TotalValidations:   v.stats.TotalValidations,
		SuccessValidations: v.stats.SuccessValidations,
		FailedValidations:  v.stats.FailedValidations,
		AverageLatency:     v.stats.AverageLatency,
		RuleStatistics:     make(map[string]uint64),
	}

	// 复制规则统计
	for ruleID, count := range v.stats.RuleStatistics {
		stats.RuleStatistics[ruleID] = count
	}

	if v.stats.LastValidation != nil {
		lastValidation := *v.stats.LastValidation
		stats.LastValidation = &lastValidation
	}

	return stats
}

// insertRuleByPriority 按优先级插入规则
func (v *BasicEventValidator) insertRuleByPriority(ruleID string, priority int) {
	// 找到插入位置（优先级数字越小，越靠前）
	insertIndex := len(v.ruleOrder)
	for i, existingRuleID := range v.ruleOrder {
		if existingRule, exists := v.rules[existingRuleID]; exists {
			if priority < existingRule.GetPriority() {
				insertIndex = i
				break
			}
		}
	}

	// 插入到指定位置
	v.ruleOrder = append(v.ruleOrder, "")
	copy(v.ruleOrder[insertIndex+1:], v.ruleOrder[insertIndex:])
	v.ruleOrder[insertIndex] = ruleID
}

// combineErrors 合并多个错误
func (v *BasicEventValidator) combineErrors(errors []error) error {
	if len(errors) == 0 {
		return nil
	}

	if len(errors) == 1 {
		return errors[0]
	}

	// 创建组合错误消息
	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Error())
	}

	return NewValidationError(
		fmt.Sprintf("multiple validation errors: [%s]", joinStrings(messages, "; ")),
		"MULTIPLE_VALIDATION_ERRORS")
}

// updateStatistics 更新验证统计信息
func (v *BasicEventValidator) updateStatistics(duration time.Duration, success bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.stats.TotalValidations++
	if success {
		v.stats.SuccessValidations++
	} else {
		v.stats.FailedValidations++
	}

	// 更新平均延迟（简单移动平均）
	if v.stats.TotalValidations == 1 {
		v.stats.AverageLatency = duration
	} else {
		alpha := 0.1 // 平滑因子
		v.stats.AverageLatency = time.Duration(
			float64(v.stats.AverageLatency)*(1-alpha) + float64(duration)*alpha)
	}

	now := time.Now()
	v.stats.LastValidation = &now
}

// joinStrings 简单的字符串连接函数
func joinStrings(strs []string, separator string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}

	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += separator + strs[i]
	}
	return result
}

// 预定义的验证规则

// BasicNameFormatRule 基础名称格式验证规则
type BasicNameFormatRule struct {
	id          string
	name        string
	description string
	priority    int
	enabled     bool
}

// NewBasicNameFormatRule 创建基础名称格式验证规则
func NewBasicNameFormatRule() *BasicNameFormatRule {
	return &BasicNameFormatRule{
		id:          "basic_name_format",
		name:        "Basic Event Name Format",
		description: "Validates basic event name format (domain.entity.action)",
		priority:    1,
		enabled:     true,
	}
}

func (r *BasicNameFormatRule) GetID() string           { return r.id }
func (r *BasicNameFormatRule) GetName() string         { return r.name }
func (r *BasicNameFormatRule) GetDescription() string  { return r.description }
func (r *BasicNameFormatRule) GetPriority() int        { return r.priority }
func (r *BasicNameFormatRule) IsEnabled() bool         { return r.enabled }
func (r *BasicNameFormatRule) SetEnabled(enabled bool) { r.enabled = enabled }

func (r *BasicNameFormatRule) Validate(ctx context.Context, event Event) error {
	if !r.enabled {
		return nil
	}

	return ValidateEventName(event.Type())
}

// DataSizeRule 数据大小验证规则
type DataSizeRule struct {
	id          string
	name        string
	description string
	priority    int
	enabled     bool
	maxSize     int
}

// NewDataSizeRule 创建数据大小验证规则
func NewDataSizeRule(maxSize int) *DataSizeRule {
	return &DataSizeRule{
		id:          "data_size_limit",
		name:        "Event Data Size Limit",
		description: fmt.Sprintf("Validates event data size does not exceed %d bytes", maxSize),
		priority:    2,
		enabled:     true,
		maxSize:     maxSize,
	}
}

func (r *DataSizeRule) GetID() string           { return r.id }
func (r *DataSizeRule) GetName() string         { return r.name }
func (r *DataSizeRule) GetDescription() string  { return r.description }
func (r *DataSizeRule) GetPriority() int        { return r.priority }
func (r *DataSizeRule) IsEnabled() bool         { return r.enabled }
func (r *DataSizeRule) SetEnabled(enabled bool) { r.enabled = enabled }

func (r *DataSizeRule) Validate(ctx context.Context, event Event) error {
	if !r.enabled {
		return nil
	}

	data := event.Data()
	if data == nil {
		return nil
	}

	// 估算数据大小
	dataStr := fmt.Sprintf("%+v", data)
	if len(dataStr) > r.maxSize {
		return NewValidationError(
			fmt.Sprintf("event data size %d exceeds limit %d", len(dataStr), r.maxSize),
			"DATA_SIZE_EXCEEDED")
	}

	return nil
}

// 工厂函数

// NewEventValidatorWithDefaultRules 创建带默认规则的事件验证器
func NewEventValidatorWithDefaultRules(logger log.Logger, config *ValidatorConfig) (*BasicEventValidator, error) {
	validator := NewBasicEventValidator(logger, config)

	// 添加默认验证规则
	if err := validator.AddRule(NewBasicNameFormatRule()); err != nil {
		return nil, fmt.Errorf("failed to add basic name format rule: %w", err)
	}

	if err := validator.AddRule(NewDataSizeRule(MaxEventDataSize)); err != nil {
		return nil, fmt.Errorf("failed to add data size rule: %w", err)
	}

	return validator, nil
}
