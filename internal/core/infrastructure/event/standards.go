// Package event 事件标准定义和工具
package event

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Event naming standards and patterns
// 事件命名标准和模式

const (
	// EventNamePattern 事件名称正则模式: domain.entity.action[.detail...]
	EventNamePattern = `^[a-z][a-z0-9_]{2,31}\.[a-z][a-z0-9_]{0,31}\.[a-z][a-z0-9_]{0,31}(\.[a-z][a-z0-9_]{0,31})*$`

	// DomainNamePattern 域名正则模式
	DomainNamePattern = `^[a-z][a-z0-9_]{2,31}$`

	// EntityNamePattern 实体名正则模式
	EntityNamePattern = `^[a-z][a-z0-9_]{0,31}$`

	// ActionNamePattern 动作名正则模式
	ActionNamePattern = `^[a-z][a-z0-9_]{0,31}$`
)

const (
	// MaxEventNameLength 事件名称最大长度
	MaxEventNameLength = 200

	// MaxDomainNameLength 域名最大长度
	MaxDomainNameLength = 32

	// MaxEntityNameLength 实体名最大长度
	MaxEntityNameLength = 32

	// MaxActionNameLength 动作名最大长度
	MaxActionNameLength = 32

	// MaxEventDataSize 事件数据最大大小（1MB）
	MaxEventDataSize = 1024 * 1024

	// MinEventNameParts 事件名称最少部分数
	MinEventNameParts = 3

	// MaxEventNameParts 事件名称最多部分数
	MaxEventNameParts = 8
)

// Standard event naming separators
// 标准事件命名分隔符

const (
	// EventNameSeparator 事件名称分隔符
	EventNameSeparator = "."

	// VersionSeparator 版本分隔符（在元数据中使用）
	VersionSeparator = "v"
)

// Standard event prefixes for different domains
// 不同域的标准事件前缀（预留，但基础设施不硬编码）

var (
	// 编译后的正则表达式，提高性能
	eventNameRegex  *regexp.Regexp
	domainNameRegex *regexp.Regexp
	entityNameRegex *regexp.Regexp
	actionNameRegex *regexp.Regexp
)

func init() {
	// 编译正则表达式
	eventNameRegex = regexp.MustCompile(EventNamePattern)
	domainNameRegex = regexp.MustCompile(DomainNamePattern)
	entityNameRegex = regexp.MustCompile(EntityNamePattern)
	actionNameRegex = regexp.MustCompile(ActionNamePattern)
}

// EventNameBuilder 事件名称构建器
// 提供便捷的事件名称构建方法，确保命名规范
type EventNameBuilder struct {
	domain string
}

// NewEventNameBuilder 创建事件名称构建器
func NewEventNameBuilder(domain string) (*EventNameBuilder, error) {
	if err := ValidateDomainName(domain); err != nil {
		return nil, fmt.Errorf("invalid domain for builder: %w", err)
	}

	return &EventNameBuilder{domain: domain}, nil
}

// Build 构建标准事件名称: domain.entity.action
func (b *EventNameBuilder) Build(entity, action string) (string, error) {
	if err := ValidateEntityName(entity); err != nil {
		return "", fmt.Errorf("invalid entity name: %w", err)
	}

	if err := ValidateActionName(action); err != nil {
		return "", fmt.Errorf("invalid action name: %w", err)
	}

	eventName := fmt.Sprintf("%s%s%s%s%s",
		b.domain, EventNameSeparator,
		entity, EventNameSeparator,
		action)

	return eventName, nil
}

// BuildWithDetail 构建带详情的事件名称: domain.entity.action.detail
func (b *EventNameBuilder) BuildWithDetail(entity, action string, details ...string) (string, error) {
	baseName, err := b.Build(entity, action)
	if err != nil {
		return "", err
	}

	// 验证详情部分
	for _, detail := range details {
		if err := ValidateActionName(detail); err != nil { // 详情使用与action相同的验证规则
			return "", fmt.Errorf("invalid detail '%s': %w", detail, err)
		}
	}

	if len(details) > 0 {
		eventName := baseName + EventNameSeparator + strings.Join(details, EventNameSeparator)

		// 验证最终事件名称
		if err := ValidateEventName(eventName); err != nil {
			return "", fmt.Errorf("built event name validation failed: %w", err)
		}

		return eventName, nil
	}

	return baseName, nil
}

// GetDomain 获取构建器的域名
func (b *EventNameBuilder) GetDomain() string {
	return b.domain
}

// EventNameParser 事件名称解析器
// 提供事件名称的解析和提取功能
type EventNameParser struct{}

// NewEventNameParser 创建事件名称解析器
func NewEventNameParser() *EventNameParser {
	return &EventNameParser{}
}

// Parse 解析事件名称，返回各个部分
func (p *EventNameParser) Parse(eventName string) (*ParsedEventName, error) {
	if err := ValidateEventName(eventName); err != nil {
		return nil, fmt.Errorf("invalid event name for parsing: %w", err)
	}

	parts := strings.Split(eventName, EventNameSeparator)
	if len(parts) < MinEventNameParts {
		return nil, fmt.Errorf("event name must have at least %d parts", MinEventNameParts)
	}

	parsed := &ParsedEventName{
		FullName: eventName,
		Domain:   parts[0],
		Entity:   parts[1],
		Action:   parts[2],
		Details:  make([]string, 0),
	}

	// 提取详情部分
	if len(parts) > MinEventNameParts {
		parsed.Details = parts[MinEventNameParts:]
	}

	return parsed, nil
}

// ExtractDomain 从事件名称中提取域名
func (p *EventNameParser) ExtractDomain(eventName string) string {
	parts := strings.Split(eventName, EventNameSeparator)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// ExtractEntity 从事件名称中提取实体名
func (p *EventNameParser) ExtractEntity(eventName string) string {
	parts := strings.Split(eventName, EventNameSeparator)
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

// ExtractAction 从事件名称中提取动作名
func (p *EventNameParser) ExtractAction(eventName string) string {
	parts := strings.Split(eventName, EventNameSeparator)
	if len(parts) > 2 {
		return parts[2]
	}
	return ""
}

// ParsedEventName 解析后的事件名称结构
type ParsedEventName struct {
	FullName string   `json:"full_name"` // 完整事件名称
	Domain   string   `json:"domain"`    // 域名
	Entity   string   `json:"entity"`    // 实体名
	Action   string   `json:"action"`    // 动作名
	Details  []string `json:"details"`   // 详情部分（可选）
}

// String 返回完整事件名称
func (p *ParsedEventName) String() string {
	return p.FullName
}

// GetBaseName 获取基础名称（不包含详情）
func (p *ParsedEventName) GetBaseName() string {
	return fmt.Sprintf("%s%s%s%s%s",
		p.Domain, EventNameSeparator,
		p.Entity, EventNameSeparator,
		p.Action)
}

// HasDetails 检查是否有详情部分
func (p *ParsedEventName) HasDetails() bool {
	return len(p.Details) > 0
}

// GetDetailString 获取详情字符串
func (p *ParsedEventName) GetDetailString() string {
	if len(p.Details) == 0 {
		return ""
	}
	return strings.Join(p.Details, EventNameSeparator)
}

// EventMetadata 事件元数据标准结构
type EventMetadata struct {
	// 基础信息
	ID        string    `json:"id"`        // 事件唯一ID
	Name      string    `json:"name"`      // 事件名称
	Version   string    `json:"version"`   // 事件版本
	Timestamp time.Time `json:"timestamp"` // 事件时间戳

	// 来源信息
	Source    string `json:"source"`    // 事件源
	Component string `json:"component"` // 产生事件的组件
	Node      string `json:"node"`      // 节点标识

	// 事件分类
	Domain   string `json:"domain"`   // 事件域
	Entity   string `json:"entity"`   // 实体类型
	Action   string `json:"action"`   // 动作类型
	Category string `json:"category"` // 事件分类（可选）

	// 处理信息
	Priority   Priority `json:"priority"`   // 事件优先级
	TTL        int64    `json:"ttl"`        // 生存时间（秒）
	Retryable  bool     `json:"retryable"`  // 是否可重试
	Idempotent bool     `json:"idempotent"` // 是否幂等

	// 关联信息
	CorrelationID string            `json:"correlation_id"` // 关联ID
	CausationID   string            `json:"causation_id"`   // 因果ID
	TraceID       string            `json:"trace_id"`       // 追踪ID
	Tags          []string          `json:"tags"`           // 标签
	Labels        map[string]string `json:"labels"`         // 标签键值对

	// 扩展字段
	Context    map[string]interface{} `json:"context"`    // 上下文信息
	Properties map[string]interface{} `json:"properties"` // 自定义属性
}

// NewEventMetadata 创建标准事件元数据
func NewEventMetadata(eventName, source string) (*EventMetadata, error) {
	parser := NewEventNameParser()
	parsed, err := parser.Parse(eventName)
	if err != nil {
		return nil, fmt.Errorf("invalid event name for metadata: %w", err)
	}

	return &EventMetadata{
		ID:         generateEventID(),
		Name:       eventName,
		Version:    "1.0",
		Timestamp:  time.Now(),
		Source:     source,
		Domain:     parsed.Domain,
		Entity:     parsed.Entity,
		Action:     parsed.Action,
		Priority:   PriorityNormal,
		TTL:        3600, // 默认1小时
		Retryable:  true,
		Idempotent: false,
		Tags:       make([]string, 0),
		Labels:     make(map[string]string),
		Context:    make(map[string]interface{}),
		Properties: make(map[string]interface{}),
	}, nil
}

// Validation functions
// 验证函数

// ValidateEventName 验证事件名称格式
func ValidateEventName(eventName string) error {
	if eventName == "" {
		return NewValidationError("event name cannot be empty", "EMPTY_EVENT_NAME")
	}

	if len(eventName) > MaxEventNameLength {
		return NewValidationError(
			fmt.Sprintf("event name too long: %d > %d", len(eventName), MaxEventNameLength),
			"EVENT_NAME_TOO_LONG")
	}

	// 先验证部分数量（避免正则表达式掩盖部分数量问题）
	parts := strings.Split(eventName, EventNameSeparator)
	if len(parts) < MinEventNameParts {
		return NewValidationError(
			fmt.Sprintf("event name must have at least %d parts", MinEventNameParts),
			"INSUFFICIENT_EVENT_NAME_PARTS")
	}

	if len(parts) > MaxEventNameParts {
		return NewValidationError(
			fmt.Sprintf("event name cannot have more than %d parts", MaxEventNameParts),
			"TOO_MANY_EVENT_NAME_PARTS")
	}

	// 再验证格式
	if !eventNameRegex.MatchString(eventName) {
		return NewValidationError(
			fmt.Sprintf("event name '%s' does not match pattern: %s", eventName, EventNamePattern),
			"INVALID_EVENT_NAME_FORMAT")
	}

	return nil
}

// ValidateDomainName 验证域名格式
func ValidateDomainName(domain string) error {
	if domain == "" {
		return NewValidationError("domain name cannot be empty", "EMPTY_DOMAIN_NAME")
	}

	if len(domain) > MaxDomainNameLength {
		return NewValidationError(
			fmt.Sprintf("domain name too long: %d > %d", len(domain), MaxDomainNameLength),
			"DOMAIN_NAME_TOO_LONG")
	}

	if !domainNameRegex.MatchString(domain) {
		return NewValidationError(
			fmt.Sprintf("domain name '%s' does not match pattern: %s", domain, DomainNamePattern),
			"INVALID_DOMAIN_NAME_FORMAT")
	}

	return nil
}

// ValidateEntityName 验证实体名格式
func ValidateEntityName(entity string) error {
	if entity == "" {
		return NewValidationError("entity name cannot be empty", "EMPTY_ENTITY_NAME")
	}

	if len(entity) > MaxEntityNameLength {
		return NewValidationError(
			fmt.Sprintf("entity name too long: %d > %d", len(entity), MaxEntityNameLength),
			"ENTITY_NAME_TOO_LONG")
	}

	if !entityNameRegex.MatchString(entity) {
		return NewValidationError(
			fmt.Sprintf("entity name '%s' does not match pattern: %s", entity, EntityNamePattern),
			"INVALID_ENTITY_NAME_FORMAT")
	}

	return nil
}

// ValidateActionName 验证动作名格式
func ValidateActionName(action string) error {
	if action == "" {
		return NewValidationError("action name cannot be empty", "EMPTY_ACTION_NAME")
	}

	if len(action) > MaxActionNameLength {
		return NewValidationError(
			fmt.Sprintf("action name too long: %d > %d", len(action), MaxActionNameLength),
			"ACTION_NAME_TOO_LONG")
	}

	if !actionNameRegex.MatchString(action) {
		return NewValidationError(
			fmt.Sprintf("action name '%s' does not match pattern: %s", action, ActionNamePattern),
			"INVALID_ACTION_NAME_FORMAT")
	}

	return nil
}

// ValidateEventData 验证事件数据
func ValidateEventData(data interface{}) error {
	if data == nil {
		return nil // 允许空数据
	}

	// 估算数据大小（简单实现）
	dataStr := fmt.Sprintf("%+v", data)
	if len(dataStr) > MaxEventDataSize {
		return NewValidationError(
			fmt.Sprintf("event data too large: %d > %d bytes", len(dataStr), MaxEventDataSize),
			"EVENT_DATA_TOO_LARGE")
	}

	return nil
}

// ValidationError 验证错误类型
type ValidationError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// Error 实现error接口
func (e *ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewValidationError 创建验证错误
func NewValidationError(message, code string) *ValidationError {
	return &ValidationError{
		Message: message,
		Code:    code,
	}
}

// IsValidationError 检查是否为验证错误
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// Utility functions
// 工具函数

// ExtractDomainFromEventName 从事件名称中快速提取域名
func ExtractDomainFromEventName(eventName string) string {
	if idx := strings.Index(eventName, EventNameSeparator); idx > 0 {
		return eventName[:idx]
	}
	return eventName
}

// ExtractEntityFromEventName 从事件名称中快速提取实体名
func ExtractEntityFromEventName(eventName string) string {
	parts := strings.SplitN(eventName, EventNameSeparator, 3)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// ExtractActionFromEventName 从事件名称中快速提取动作名
func ExtractActionFromEventName(eventName string) string {
	parts := strings.SplitN(eventName, EventNameSeparator, 4)
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}

// NormalizeEventName 标准化事件名称（转换为小写，移除额外空格）
func NormalizeEventName(eventName string) string {
	return strings.ToLower(strings.TrimSpace(eventName))
}

// IsStandardEventName 检查是否为标准格式的事件名称
func IsStandardEventName(eventName string) bool {
	return ValidateEventName(eventName) == nil
}

// GetEventNamePattern 获取事件名称正则模式（用于外部验证）
func GetEventNamePattern() string {
	return EventNamePattern
}

// GetEventNameInfo 获取事件名称的详细信息
func GetEventNameInfo(eventName string) map[string]interface{} {
	info := map[string]interface{}{
		"valid":      IsStandardEventName(eventName),
		"length":     len(eventName),
		"max_length": MaxEventNameLength,
	}

	if parts := strings.Split(eventName, EventNameSeparator); len(parts) >= MinEventNameParts {
		info["parts"] = len(parts)
		info["domain"] = parts[0]
		if len(parts) > 1 {
			info["entity"] = parts[1]
		}
		if len(parts) > 2 {
			info["action"] = parts[2]
		}
		if len(parts) > 3 {
			info["details"] = parts[3:]
		}
	}

	return info
}

// generateEventID 生成事件ID（简单实现）
func generateEventID() string {
	return fmt.Sprintf("evt_%d", time.Now().UnixNano())
}
