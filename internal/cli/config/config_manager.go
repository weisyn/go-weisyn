// Package config 提供CLI的配置管理功能
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/weisyn/v1/internal/cli/ui"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ConfigScope 配置范围
type ConfigScope string

const (
	// GlobalScope 全局配置
	GlobalScope ConfigScope = "global"
	// UserScope 用户配置
	UserScope ConfigScope = "user"
	// SessionScope 会话配置
	SessionScope ConfigScope = "session"
	// LocalScope 本地配置
	LocalScope ConfigScope = "local"
)

// ConfigFormat 配置格式
type ConfigFormat string

const (
	// JSONFormat JSON格式
	JSONFormat ConfigFormat = "json"
	// YAMLFormat YAML格式
	YAMLFormat ConfigFormat = "yaml"
	// TOMLFormat TOML格式
	TOMLFormat ConfigFormat = "toml"
)

// ConfigValue 配置值接口
type ConfigValue interface {
	// GetValue 获取值
	GetValue() interface{}
	// SetValue 设置值
	SetValue(value interface{}) error
	// GetType 获取类型
	GetType() reflect.Type
	// Validate 验证值
	Validate() error
	// GetDefault 获取默认值
	GetDefault() interface{}
}

// ConfigEntry 配置项
type ConfigEntry struct {
	Key         string                 // 配置键
	Value       interface{}            // 配置值
	Type        string                 // 值类型
	Description string                 // 描述
	Category    string                 // 分类
	Scope       ConfigScope            // 作用范围
	Default     interface{}            // 默认值
	Constraints map[string]interface{} // 约束条件
	Tags        []string               // 标签
	CreatedAt   time.Time              // 创建时间
	UpdatedAt   time.Time              // 更新时间
	UpdatedBy   string                 // 更新者
}

// ConfigSection 配置分组
type ConfigSection struct {
	Name        string                    // 分组名称
	Description string                    // 描述
	Entries     map[string]*ConfigEntry   // 配置项
	SubSections map[string]*ConfigSection // 子分组
	Order       int                       // 排序
	Metadata    map[string]interface{}    // 元数据
}

// ConfigChangeEvent 配置变更事件
type ConfigChangeEvent struct {
	Key       string      // 配置键
	OldValue  interface{} // 旧值
	NewValue  interface{} // 新值
	Scope     ConfigScope // 作用范围
	Timestamp time.Time   // 时间戳
	Source    string      // 变更源
}

// ConfigValidator 配置验证器
type ConfigValidator interface {
	// Validate 验证配置值
	Validate(key string, value interface{}) error
	// GetConstraints 获取约束条件
	GetConstraints(key string) map[string]interface{}
}

// ConfigListener 配置监听器
type ConfigListener interface {
	// OnConfigChanged 配置变更回调
	OnConfigChanged(event ConfigChangeEvent) error
}

// ConfigManager 配置管理器接口
type ConfigManager interface {
	// 基本操作
	Get(key string) (interface{}, error)
	GetWithScope(key string, scope ConfigScope) (interface{}, error)
	Set(key string, value interface{}) error
	SetWithScope(key string, value interface{}, scope ConfigScope) error
	Delete(key string) error

	// 类型化获取
	GetString(key string) (string, error)
	GetInt(key string) (int, error)
	GetBool(key string) (bool, error)
	GetFloat(key string) (float64, error)
	GetStringSlice(key string) ([]string, error)

	// 配置文件操作
	Load(filePath string) error
	LoadFromDir(dirPath string) error
	Save(filePath string) error
	SaveWithFormat(filePath string, format ConfigFormat) error

	// 配置结构操作
	GetSection(sectionName string) (*ConfigSection, error)
	SetSection(sectionName string, section *ConfigSection) error
	ListSections() []string

	// 默认配置
	SetDefault(key string, value interface{}) error
	ResetToDefault(key string) error
	ResetAllToDefault() error

	// 验证和监听
	RegisterValidator(validator ConfigValidator) error
	RegisterListener(listener ConfigListener) error
	UnregisterListener(listener ConfigListener) error

	// 导入导出
	Export() (map[string]interface{}, error)
	Import(data map[string]interface{}) error

	// 配置管理
	ListKeys() []string
	ListKeysByScope(scope ConfigScope) []string
	GetConfigInfo(key string) (*ConfigEntry, error)

	// 生命周期
	Initialize(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// configManager 配置管理器实现
type configManager struct {
	logger log.Logger
	ui     ui.Components

	// 配置存储
	sections   map[string]*ConfigSection
	entries    map[string]*ConfigEntry
	defaults   map[string]interface{}
	sectionsMu sync.RWMutex

	// 验证和监听
	validators  []ConfigValidator
	listeners   []ConfigListener
	listenersMu sync.RWMutex

	// 配置文件
	configDir   string
	configFiles map[ConfigScope]string
	format      ConfigFormat

	// 会话状态
	sessionData map[string]interface{}
	sessionMu   sync.RWMutex

	// 配置源优先级
	scopePriority []ConfigScope
}

// NewConfigManager 创建配置管理器
func NewConfigManager(logger log.Logger, uiComponents ui.Components, configDir string) ConfigManager {
	if configDir == "" {
		homeDir, _ := os.UserHomeDir()
		configDir = filepath.Join(homeDir, ".weisyn_cli", "config")
	}

	cm := &configManager{
		logger:     logger,
		ui:         uiComponents,
		sections:   make(map[string]*ConfigSection),
		entries:    make(map[string]*ConfigEntry),
		defaults:   make(map[string]interface{}),
		validators: make([]ConfigValidator, 0),
		listeners:  make([]ConfigListener, 0),
		configDir:  configDir,
		configFiles: map[ConfigScope]string{
			GlobalScope: filepath.Join(configDir, "global.json"),
			UserScope:   filepath.Join(configDir, "user.json"),
			LocalScope:  filepath.Join(configDir, "local.json"),
		},
		format:        JSONFormat,
		sessionData:   make(map[string]interface{}),
		scopePriority: []ConfigScope{SessionScope, LocalScope, UserScope, GlobalScope},
	}

	// 初始化默认配置
	cm.initializeDefaults()

	return cm
}

// Initialize 初始化配置管理器
func (cm *configManager) Initialize(ctx context.Context) error {
	cm.logger.Info("初始化配置管理器")

	// 确保配置目录存在
	if err := os.MkdirAll(cm.configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	// 加载现有配置文件
	for scope, filePath := range cm.configFiles {
		if _, err := os.Stat(filePath); err == nil {
			if err := cm.loadScopeConfig(scope, filePath); err != nil {
				cm.logger.Error(fmt.Sprintf("加载%s配置失败: %v", scope, err))
			} else {
				cm.logger.Info(fmt.Sprintf("加载%s配置成功: %s", scope, filePath))
			}
		}
	}

	// 应用默认配置
	cm.applyDefaults()

	cm.logger.Info("配置管理器初始化完成")
	return nil
}

// Shutdown 关闭配置管理器
func (cm *configManager) Shutdown(ctx context.Context) error {
	cm.logger.Info("关闭配置管理器")

	// 保存所有配置
	for scope, filePath := range cm.configFiles {
		if err := cm.saveScopeConfig(scope, filePath); err != nil {
			cm.logger.Error(fmt.Sprintf("保存%s配置失败: %v", scope, err))
		}
	}

	cm.logger.Info("配置管理器已关闭")
	return nil
}

// Get 获取配置值
func (cm *configManager) Get(key string) (interface{}, error) {
	cm.sectionsMu.RLock()
	defer cm.sectionsMu.RUnlock()

	// 按优先级查找配置值
	for _, scope := range cm.scopePriority {
		if value, exists := cm.getValueFromScope(key, scope); exists {
			return value, nil
		}
	}

	// 查找默认值
	if defaultValue, exists := cm.defaults[key]; exists {
		return defaultValue, nil
	}

	return nil, fmt.Errorf("配置项不存在: %s", key)
}

// GetWithScope 从指定范围获取配置值
func (cm *configManager) GetWithScope(key string, scope ConfigScope) (interface{}, error) {
	cm.sectionsMu.RLock()
	defer cm.sectionsMu.RUnlock()

	if value, exists := cm.getValueFromScope(key, scope); exists {
		return value, nil
	}

	return nil, fmt.Errorf("配置项不存在: %s (scope: %s)", key, scope)
}

// Set 设置配置值
func (cm *configManager) Set(key string, value interface{}) error {
	return cm.SetWithScope(key, value, UserScope)
}

// SetWithScope 在指定范围设置配置值
func (cm *configManager) SetWithScope(key string, value interface{}, scope ConfigScope) error {
	// 验证配置值
	if err := cm.validateValue(key, value); err != nil {
		return fmt.Errorf("配置值验证失败: %v", err)
	}

	cm.sectionsMu.Lock()
	defer cm.sectionsMu.Unlock()

	// 获取旧值（用于事件）
	oldValue, _ := cm.getValueFromScope(key, scope)

	// 设置新值
	if err := cm.setValueInScope(key, value, scope); err != nil {
		return err
	}

	// 触发变更事件
	event := ConfigChangeEvent{
		Key:       key,
		OldValue:  oldValue,
		NewValue:  value,
		Scope:     scope,
		Timestamp: time.Now(),
		Source:    "config_manager",
	}

	cm.notifyListeners(event)

	cm.logger.Info(fmt.Sprintf("配置已更新: key=%s, scope=%s", key, scope))
	return nil
}

// Delete 删除配置项
func (cm *configManager) Delete(key string) error {
	cm.sectionsMu.Lock()
	defer cm.sectionsMu.Unlock()

	// 从所有范围中删除
	found := false
	for _, scope := range cm.scopePriority {
		if _, exists := cm.getValueFromScope(key, scope); exists {
			cm.deleteValueFromScope(key, scope)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("配置项不存在: %s", key)
	}

	cm.logger.Info(fmt.Sprintf("配置已删除: key=%s", key))
	return nil
}

// 类型化获取方法

// GetString 获取字符串配置
func (cm *configManager) GetString(key string) (string, error) {
	value, err := cm.Get(key)
	if err != nil {
		return "", err
	}

	if str, ok := value.(string); ok {
		return str, nil
	}

	return fmt.Sprintf("%v", value), nil
}

// GetInt 获取整数配置
func (cm *configManager) GetInt(key string) (int, error) {
	value, err := cm.Get(key)
	if err != nil {
		return 0, err
	}

	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("配置值不是整数类型: %s", key)
	}
}

// GetBool 获取布尔配置
func (cm *configManager) GetBool(key string) (bool, error) {
	value, err := cm.Get(key)
	if err != nil {
		return false, err
	}

	if b, ok := value.(bool); ok {
		return b, nil
	}

	return false, fmt.Errorf("配置值不是布尔类型: %s", key)
}

// GetFloat 获取浮点数配置
func (cm *configManager) GetFloat(key string) (float64, error) {
	value, err := cm.Get(key)
	if err != nil {
		return 0, err
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("配置值不是数字类型: %s", key)
	}
}

// GetStringSlice 获取字符串数组配置
func (cm *configManager) GetStringSlice(key string) ([]string, error) {
	value, err := cm.Get(key)
	if err != nil {
		return nil, err
	}

	if slice, ok := value.([]string); ok {
		return slice, nil
	}

	if slice, ok := value.([]interface{}); ok {
		result := make([]string, len(slice))
		for i, item := range slice {
			result[i] = fmt.Sprintf("%v", item)
		}
		return result, nil
	}

	return nil, fmt.Errorf("配置值不是字符串数组类型: %s", key)
}

// getValueFromScope 从指定范围获取值
func (cm *configManager) getValueFromScope(key string, scope ConfigScope) (interface{}, bool) {
	switch scope {
	case SessionScope:
		cm.sessionMu.RLock()
		value, exists := cm.sessionData[key]
		cm.sessionMu.RUnlock()
		return value, exists
	default:
		// 从配置项中查找
		if entry, exists := cm.entries[key]; exists && entry.Scope == scope {
			return entry.Value, true
		}
		return nil, false
	}
}

// setValueInScope 在指定范围设置值
func (cm *configManager) setValueInScope(key string, value interface{}, scope ConfigScope) error {
	switch scope {
	case SessionScope:
		cm.sessionMu.Lock()
		cm.sessionData[key] = value
		cm.sessionMu.Unlock()
		return nil
	default:
		// 创建或更新配置项
		entry := &ConfigEntry{
			Key:       key,
			Value:     value,
			Type:      reflect.TypeOf(value).String(),
			Scope:     scope,
			UpdatedAt: time.Now(),
			UpdatedBy: "config_manager",
		}

		if existing, exists := cm.entries[key]; exists {
			entry.CreatedAt = existing.CreatedAt
			entry.Description = existing.Description
			entry.Category = existing.Category
			entry.Default = existing.Default
			entry.Constraints = existing.Constraints
			entry.Tags = existing.Tags
		} else {
			entry.CreatedAt = time.Now()
		}

		cm.entries[key] = entry
		return nil
	}
}

// deleteValueFromScope 从指定范围删除值
func (cm *configManager) deleteValueFromScope(key string, scope ConfigScope) {
	switch scope {
	case SessionScope:
		cm.sessionMu.Lock()
		delete(cm.sessionData, key)
		cm.sessionMu.Unlock()
	default:
		if entry, exists := cm.entries[key]; exists && entry.Scope == scope {
			delete(cm.entries, key)
		}
	}
}

// validateValue 验证配置值
func (cm *configManager) validateValue(key string, value interface{}) error {
	// 使用注册的验证器
	for _, validator := range cm.validators {
		if err := validator.Validate(key, value); err != nil {
			return err
		}
	}

	// 检查配置项约束
	if entry, exists := cm.entries[key]; exists && entry.Constraints != nil {
		return cm.validateConstraints(key, value, entry.Constraints)
	}

	return nil
}

// validateConstraints 验证约束条件
func (cm *configManager) validateConstraints(key string, value interface{}, constraints map[string]interface{}) error {
	// 类型检查
	if expectedType, exists := constraints["type"]; exists {
		actualType := reflect.TypeOf(value).String()
		if actualType != expectedType {
			return fmt.Errorf("配置%s类型错误: 期望%s, 实际%s", key, expectedType, actualType)
		}
	}

	// 范围检查
	if minVal, exists := constraints["min"]; exists {
		if !cm.compareValue(value, minVal, ">=") {
			return fmt.Errorf("配置%s值过小: 最小值%v", key, minVal)
		}
	}

	if maxVal, exists := constraints["max"]; exists {
		if !cm.compareValue(value, maxVal, "<=") {
			return fmt.Errorf("配置%s值过大: 最大值%v", key, maxVal)
		}
	}

	// 枚举检查
	if allowedValues, exists := constraints["enum"]; exists {
		if allowed, ok := allowedValues.([]interface{}); ok {
			found := false
			for _, allowedValue := range allowed {
				if reflect.DeepEqual(value, allowedValue) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("配置%s值不在允许范围内: %v", key, allowedValues)
			}
		}
	}

	return nil
}

// compareValue 比较值
func (cm *configManager) compareValue(value, target interface{}, operator string) bool {
	// 简化实现，只处理数字比较
	v1, ok1 := value.(float64)
	v2, ok2 := target.(float64)

	if !ok1 || !ok2 {
		return true // 无法比较，默认通过
	}

	switch operator {
	case ">=":
		return v1 >= v2
	case "<=":
		return v1 <= v2
	case ">":
		return v1 > v2
	case "<":
		return v1 < v2
	case "==":
		return v1 == v2
	default:
		return true
	}
}

// notifyListeners 通知监听器
func (cm *configManager) notifyListeners(event ConfigChangeEvent) {
	cm.listenersMu.RLock()
	listeners := make([]ConfigListener, len(cm.listeners))
	copy(listeners, cm.listeners)
	cm.listenersMu.RUnlock()

	for _, listener := range listeners {
		if err := listener.OnConfigChanged(event); err != nil {
			cm.logger.Error(fmt.Sprintf("配置监听器处理失败: %v", err))
		}
	}
}

// initializeDefaults 初始化默认配置
func (cm *configManager) initializeDefaults() {
	// UI相关默认配置
	cm.defaults["ui.theme"] = "default"
	cm.defaults["ui.language"] = "zh-CN"
	cm.defaults["ui.show_hints"] = true
	cm.defaults["ui.animation_enabled"] = true
	cm.defaults["ui.page_size"] = 20

	// 网络相关默认配置
	cm.defaults["network.api_url"] = "http://localhost:8080"
	cm.defaults["network.timeout"] = 30
	cm.defaults["network.max_retries"] = 3
	cm.defaults["network.enable_tls"] = false

	// 安全相关默认配置
	cm.defaults["security.require_confirmation"] = true
	cm.defaults["security.session_timeout"] = 1800
	cm.defaults["security.mask_sensitive_data"] = true
	cm.defaults["security.audit_logging"] = true

	// 系统相关默认配置
	cm.defaults["system.log_level"] = "info"
	cm.defaults["system.auto_save"] = true
	cm.defaults["system.backup_enabled"] = true
	cm.defaults["system.cleanup_interval"] = 3600

	// 钱包相关默认配置
	cm.defaults["wallet.auto_lock_timeout"] = 600
	cm.defaults["wallet.backup_on_create"] = true
	cm.defaults["wallet.encryption_enabled"] = true

	cm.logger.Info(fmt.Sprintf("初始化了 %d 个默认配置项", len(cm.defaults)))
}

// applyDefaults 应用默认配置
func (cm *configManager) applyDefaults() {
	for key, value := range cm.defaults {
		// 只在配置不存在时应用默认值
		if _, err := cm.Get(key); err != nil {
			cm.setValueInScope(key, value, GlobalScope)
		}
	}
}

// loadScopeConfig 加载指定范围的配置
func (cm *configManager) loadScopeConfig(scope ConfigScope, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 将配置加载到对应范围
	for key, value := range config {
		cm.setValueInScope(key, value, scope)
	}

	return nil
}

// saveScopeConfig 保存指定范围的配置
func (cm *configManager) saveScopeConfig(scope ConfigScope, filePath string) error {
	config := make(map[string]interface{})

	// 收集该范围的所有配置
	for key, entry := range cm.entries {
		if entry.Scope == scope {
			config[key] = entry.Value
		}
	}

	// 如果没有配置项，不创建文件
	if len(config) == 0 {
		return nil
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// 其他方法的实现将在下个文件中继续...
