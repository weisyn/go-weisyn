// Package event 事件域注册中心实现
package event

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// DomainRegistry 事件域注册中心
// 负责管理所有组件的事件域注册，防止域名冲突，提供域验证服务
type DomainRegistry struct {
	mu      sync.RWMutex
	domains map[string]*DomainInfo // domain -> info映射
	routes  map[string][]string    // eventPattern -> subscriberIDs映射
	logger  log.Logger             // 日志记录器
}

// DomainInfo 域信息结构
type DomainInfo struct {
	Name         string    `json:"name"`          // 域名称，如"blockchain"
	Component    string    `json:"component"`     // 组件标识
	Description  string    `json:"description"`   // 域描述
	EventTypes   []string  `json:"event_types"`   // 该域支持的事件类型（可选）
	RegisteredAt time.Time `json:"registered_at"` // 注册时间
	Active       bool      `json:"active"`        // 是否活跃
}

// NewDomainRegistry 创建新的域注册中心
func NewDomainRegistry(logger log.Logger) *DomainRegistry {
	var componentLogger log.Logger
	if logger != nil {
		componentLogger = logger.With("component", "domain_registry")
	}

	return &DomainRegistry{
		domains: make(map[string]*DomainInfo),
		routes:  make(map[string][]string),
		logger:  componentLogger,
	}
}

// RegisterDomain 注册事件域
// 组件启动时调用此方法注册其事件域
func (r *DomainRegistry) RegisterDomain(domain string, info DomainInfo) error {
	if domain == "" {
		return fmt.Errorf("domain name cannot be empty")
	}

	// 验证域名格式
	if err := r.validateDomainName(domain); err != nil {
		return fmt.Errorf("invalid domain name %s: %w", domain, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已注册
	if existing, exists := r.domains[domain]; exists {
		return fmt.Errorf("domain %s already registered by component %s at %v",
			domain, existing.Component, existing.RegisteredAt)
	}

	// 设置注册信息
	domainInfo := &DomainInfo{
		Name:         domain,
		Component:    info.Component,
		Description:  info.Description,
		EventTypes:   info.EventTypes,
		RegisteredAt: time.Now(),
		Active:       true,
	}

	r.domains[domain] = domainInfo

	r.logger.Infof("注册事件域: domain=%s, component=%s, description=%s",
		domain, info.Component, info.Description)

	return nil
}

// UnregisterDomain 注销事件域
func (r *DomainRegistry) UnregisterDomain(domain string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.domains[domain]; !exists {
		return fmt.Errorf("domain %s not found", domain)
	}

	delete(r.domains, domain)

	r.logger.Infof("注销事件域: domain=%s", domain)
	return nil
}

// IsDomainRegistered 检查域是否已注册
func (r *DomainRegistry) IsDomainRegistered(domain string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.domains[domain]
	return exists && info.Active
}

// GetDomainInfo 获取域信息
func (r *DomainRegistry) GetDomainInfo(domain string) *DomainInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if info, exists := r.domains[domain]; exists {
		// 返回副本，避免外部修改
		return &DomainInfo{
			Name:         info.Name,
			Component:    info.Component,
			Description:  info.Description,
			EventTypes:   append([]string{}, info.EventTypes...),
			RegisteredAt: info.RegisteredAt,
			Active:       info.Active,
		}
	}
	return nil
}

// ListDomains 列出所有已注册的域
func (r *DomainRegistry) ListDomains() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	domains := make([]string, 0, len(r.domains))
	for domain, info := range r.domains {
		if info.Active {
			domains = append(domains, domain)
		}
	}
	return domains
}

// GetAllDomainInfos 获取所有域的详细信息
func (r *DomainRegistry) GetAllDomainInfos() map[string]*DomainInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*DomainInfo)
	for domain, info := range r.domains {
		if info.Active {
			result[domain] = &DomainInfo{
				Name:         info.Name,
				Component:    info.Component,
				Description:  info.Description,
				EventTypes:   append([]string{}, info.EventTypes...),
				RegisteredAt: info.RegisteredAt,
				Active:       info.Active,
			}
		}
	}
	return result
}

// ValidateEventName 验证事件名称是否符合规范
func (r *DomainRegistry) ValidateEventName(eventName string) error {
	if eventName == "" {
		return fmt.Errorf("event name cannot be empty")
	}

	// 验证事件名称格式: domain.entity.action
	parts := strings.Split(eventName, ".")
	if len(parts) < 3 {
		return fmt.Errorf("event name must follow format: domain.entity.action, got: %s", eventName)
	}

	domain := parts[0]

	// 验证域名格式
	if err := r.validateDomainName(domain); err != nil {
		return fmt.Errorf("invalid domain in event name %s: %w", eventName, err)
	}

	// 验证实体和动作部分
	for i := 1; i < len(parts); i++ {
		if err := r.validateEventNamePart(parts[i]); err != nil {
			return fmt.Errorf("invalid part '%s' in event name %s: %w", parts[i], eventName, err)
		}
	}

	return nil
}

// ValidateEventNameWithDomainCheck 验证事件名称并检查域是否已注册
func (r *DomainRegistry) ValidateEventNameWithDomainCheck(eventName string, strictMode bool) error {
	// 首先验证格式
	if err := r.ValidateEventName(eventName); err != nil {
		return err
	}

	// 如果启用严格模式，检查域是否已注册
	if strictMode {
		domain := r.ExtractDomain(eventName)
		if !r.IsDomainRegistered(domain) {
			return fmt.Errorf("domain %s in event %s is not registered", domain, eventName)
		}
	}

	return nil
}

// ExtractDomain 从事件名称中提取域名
func (r *DomainRegistry) ExtractDomain(eventName string) string {
	parts := strings.Split(eventName, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// validateDomainName 验证域名格式
func (r *DomainRegistry) validateDomainName(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain name cannot be empty")
	}

	// 域名规则：小写字母、数字、连字符，3-32字符
	domainPattern := `^[a-z][a-z0-9_]{2,31}$`
	matched, err := regexp.MatchString(domainPattern, domain)
	if err != nil {
		return fmt.Errorf("regex error: %w", err)
	}
	if !matched {
		return fmt.Errorf("domain name must be 3-32 characters, start with letter, contain only lowercase letters, numbers and underscores")
	}

	return nil
}

// validateEventNamePart 验证事件名称的各个部分
func (r *DomainRegistry) validateEventNamePart(part string) error {
	if part == "" {
		return fmt.Errorf("event name part cannot be empty")
	}

	// 事件名称部分规则：小写字母、数字、下划线，1-32字符
	partPattern := `^[a-z][a-z0-9_]{0,31}$`
	matched, err := regexp.MatchString(partPattern, part)
	if err != nil {
		return fmt.Errorf("regex error: %w", err)
	}
	if !matched {
		return fmt.Errorf("event name part must be 1-32 characters, start with letter, contain only lowercase letters, numbers and underscores")
	}

	return nil
}

// AddEventRoute 添加事件路由映射
func (r *DomainRegistry) AddEventRoute(eventPattern string, subscriberID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.routes[eventPattern] == nil {
		r.routes[eventPattern] = make([]string, 0)
	}

	// 避免重复添加
	for _, existing := range r.routes[eventPattern] {
		if existing == subscriberID {
			return
		}
	}

	r.routes[eventPattern] = append(r.routes[eventPattern], subscriberID)
}

// RemoveEventRoute 移除事件路由映射
func (r *DomainRegistry) RemoveEventRoute(eventPattern string, subscriberID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	subscribers := r.routes[eventPattern]
	for i, id := range subscribers {
		if id == subscriberID {
			// 移除该订阅者
			r.routes[eventPattern] = append(subscribers[:i], subscribers[i+1:]...)
			break
		}
	}

	// 如果没有订阅者了，删除该路由
	if len(r.routes[eventPattern]) == 0 {
		delete(r.routes, eventPattern)
	}
}

// GetEventRoutes 获取事件的路由信息
func (r *DomainRegistry) GetEventRoutes(eventPattern string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if routes, exists := r.routes[eventPattern]; exists {
		// 返回副本
		return append([]string{}, routes...)
	}
	return nil
}

// GetStatistics 获取注册中心统计信息
func (r *DomainRegistry) GetStatistics() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	activeDomains := 0
	totalEventTypes := 0
	componentCount := make(map[string]int)

	for _, info := range r.domains {
		if info.Active {
			activeDomains++
			totalEventTypes += len(info.EventTypes)
			componentCount[info.Component]++
		}
	}

	return map[string]interface{}{
		"active_domains":    activeDomains,
		"total_event_types": totalEventTypes,
		"total_routes":      len(r.routes),
		"component_count":   componentCount,
		"last_updated":      time.Now(),
	}
}

// IsValidEventName 辅助函数：检查事件名称是否有效
func IsValidEventName(eventName string) bool {
	registry := &DomainRegistry{}
	return registry.ValidateEventName(eventName) == nil
}

// IsValidDomainName 辅助函数：检查域名是否有效
func IsValidDomainName(domain string) bool {
	registry := &DomainRegistry{}
	return registry.validateDomainName(domain) == nil
}
