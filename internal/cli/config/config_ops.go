// Package config 提供CLI的配置管理功能 - 操作实现
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Load 加载配置文件
func (cm *configManager) Load(filePath string) error {
	cm.logger.Info(fmt.Sprintf("加载配置文件: %s", filePath))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	cm.sectionsMu.Lock()
	defer cm.sectionsMu.Unlock()

	// 导入配置
	for key, value := range config {
		cm.setValueInScope(key, value, UserScope)
	}

	cm.logger.Info(fmt.Sprintf("成功加载 %d 个配置项", len(config)))
	return nil
}

// LoadFromDir 从目录加载配置文件
func (cm *configManager) LoadFromDir(dirPath string) error {
	cm.logger.Info(fmt.Sprintf("从目录加载配置: %s", dirPath))

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("读取配置目录失败: %v", err)
	}

	loadedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".json") {
			filePath := filepath.Join(dirPath, name)
			if err := cm.Load(filePath); err != nil {
				cm.logger.Error(fmt.Sprintf("加载配置文件 %s 失败: %v", name, err))
			} else {
				loadedCount++
			}
		}
	}

	cm.logger.Info(fmt.Sprintf("成功从目录加载 %d 个配置文件", loadedCount))
	return nil
}

// Save 保存配置到文件
func (cm *configManager) Save(filePath string) error {
	return cm.SaveWithFormat(filePath, JSONFormat)
}

// SaveWithFormat 以指定格式保存配置
func (cm *configManager) SaveWithFormat(filePath string, format ConfigFormat) error {
	cm.logger.Info(fmt.Sprintf("保存配置到文件: %s (格式: %s)", filePath, format))

	// 导出所有配置
	config, err := cm.Export()
	if err != nil {
		return fmt.Errorf("导出配置失败: %v", err)
	}

	var data []byte
	switch format {
	case JSONFormat:
		data, err = json.MarshalIndent(config, "", "  ")
	default:
		return fmt.Errorf("不支持的配置格式: %s", format)
	}

	if err != nil {
		return fmt.Errorf("序列化配置失败: %v", err)
	}

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}

	cm.logger.Info("配置保存成功")
	return nil
}

// GetSection 获取配置分组
func (cm *configManager) GetSection(sectionName string) (*ConfigSection, error) {
	cm.sectionsMu.RLock()
	defer cm.sectionsMu.RUnlock()

	if section, exists := cm.sections[sectionName]; exists {
		return section, nil
	}

	// 如果分组不存在，创建一个空的分组
	section := &ConfigSection{
		Name:        sectionName,
		Description: fmt.Sprintf("配置分组: %s", sectionName),
		Entries:     make(map[string]*ConfigEntry),
		SubSections: make(map[string]*ConfigSection),
		Metadata:    make(map[string]interface{}),
	}

	return section, nil
}

// SetSection 设置配置分组
func (cm *configManager) SetSection(sectionName string, section *ConfigSection) error {
	cm.sectionsMu.Lock()
	defer cm.sectionsMu.Unlock()

	cm.sections[sectionName] = section

	// 将分组中的配置项同步到全局配置
	for key, entry := range section.Entries {
		fullKey := fmt.Sprintf("%s.%s", sectionName, key)
		cm.entries[fullKey] = entry
	}

	cm.logger.Info(fmt.Sprintf("设置配置分组: %s", sectionName))
	return nil
}

// ListSections 列出所有配置分组
func (cm *configManager) ListSections() []string {
	cm.sectionsMu.RLock()
	defer cm.sectionsMu.RUnlock()

	sections := make([]string, 0, len(cm.sections))
	for name := range cm.sections {
		sections = append(sections, name)
	}

	return sections
}

// SetDefault 设置默认配置值
func (cm *configManager) SetDefault(key string, value interface{}) error {
	cm.sectionsMu.Lock()
	defer cm.sectionsMu.Unlock()

	cm.defaults[key] = value

	// 如果当前没有配置值，应用默认值
	if _, err := cm.Get(key); err != nil {
		cm.setValueInScope(key, value, GlobalScope)
	}

	cm.logger.Info(fmt.Sprintf("设置默认配置: key=%s", key))
	return nil
}

// ResetToDefault 重置配置到默认值
func (cm *configManager) ResetToDefault(key string) error {
	defaultValue, exists := cm.defaults[key]
	if !exists {
		return fmt.Errorf("配置项没有默认值: %s", key)
	}

	return cm.Set(key, defaultValue)
}

// ResetAllToDefault 重置所有配置到默认值
func (cm *configManager) ResetAllToDefault() error {
	cm.logger.Info("重置所有配置到默认值")

	errorCount := 0
	for key, defaultValue := range cm.defaults {
		if err := cm.Set(key, defaultValue); err != nil {
			cm.logger.Error(fmt.Sprintf("重置配置失败: key=%s, error=%v", key, err))
			errorCount++
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("重置过程中发生 %d 个错误", errorCount)
	}

	cm.logger.Info("所有配置已重置到默认值")
	return nil
}

// RegisterValidator 注册配置验证器
func (cm *configManager) RegisterValidator(validator ConfigValidator) error {
	cm.listenersMu.Lock()
	defer cm.listenersMu.Unlock()

	cm.validators = append(cm.validators, validator)

	cm.logger.Info("注册配置验证器")
	return nil
}

// RegisterListener 注册配置监听器
func (cm *configManager) RegisterListener(listener ConfigListener) error {
	cm.listenersMu.Lock()
	defer cm.listenersMu.Unlock()

	cm.listeners = append(cm.listeners, listener)

	cm.logger.Info("注册配置监听器")
	return nil
}

// UnregisterListener 注销配置监听器
func (cm *configManager) UnregisterListener(listener ConfigListener) error {
	cm.listenersMu.Lock()
	defer cm.listenersMu.Unlock()

	for i, l := range cm.listeners {
		if l == listener {
			cm.listeners = append(cm.listeners[:i], cm.listeners[i+1:]...)
			cm.logger.Info("注销配置监听器")
			return nil
		}
	}

	return fmt.Errorf("监听器未找到")
}

// Export 导出所有配置
func (cm *configManager) Export() (map[string]interface{}, error) {
	cm.sectionsMu.RLock()
	defer cm.sectionsMu.RUnlock()

	config := make(map[string]interface{})

	// 导出所有配置项
	for key, entry := range cm.entries {
		config[key] = entry.Value
	}

	// 包含会话数据
	cm.sessionMu.RLock()
	for key, value := range cm.sessionData {
		config[key] = value
	}
	cm.sessionMu.RUnlock()

	cm.logger.Info(fmt.Sprintf("导出了 %d 个配置项", len(config)))
	return config, nil
}

// Import 导入配置数据
func (cm *configManager) Import(data map[string]interface{}) error {
	cm.logger.Info(fmt.Sprintf("导入 %d 个配置项", len(data)))

	cm.sectionsMu.Lock()
	defer cm.sectionsMu.Unlock()

	errorCount := 0
	for key, value := range data {
		if err := cm.setValueInScope(key, value, UserScope); err != nil {
			cm.logger.Error(fmt.Sprintf("导入配置失败: key=%s, error=%v", key, err))
			errorCount++
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("导入过程中发生 %d 个错误", errorCount)
	}

	cm.logger.Info("配置导入完成")
	return nil
}

// ListKeys 列出所有配置键
func (cm *configManager) ListKeys() []string {
	cm.sectionsMu.RLock()
	defer cm.sectionsMu.RUnlock()

	keys := make([]string, 0, len(cm.entries))
	for key := range cm.entries {
		keys = append(keys, key)
	}

	// 包含会话数据的键
	cm.sessionMu.RLock()
	for key := range cm.sessionData {
		keys = append(keys, key)
	}
	cm.sessionMu.RUnlock()

	return keys
}

// ListKeysByScope 按范围列出配置键
func (cm *configManager) ListKeysByScope(scope ConfigScope) []string {
	cm.sectionsMu.RLock()
	defer cm.sectionsMu.RUnlock()

	keys := make([]string, 0)

	switch scope {
	case SessionScope:
		cm.sessionMu.RLock()
		for key := range cm.sessionData {
			keys = append(keys, key)
		}
		cm.sessionMu.RUnlock()
	default:
		for key, entry := range cm.entries {
			if entry.Scope == scope {
				keys = append(keys, key)
			}
		}
	}

	return keys
}

// GetConfigInfo 获取配置项信息
func (cm *configManager) GetConfigInfo(key string) (*ConfigEntry, error) {
	cm.sectionsMu.RLock()
	defer cm.sectionsMu.RUnlock()

	if entry, exists := cm.entries[key]; exists {
		// 创建副本以避免并发修改
		entryCopy := *entry
		return &entryCopy, nil
	}

	// 检查是否是会话数据
	cm.sessionMu.RLock()
	if value, exists := cm.sessionData[key]; exists {
		cm.sessionMu.RUnlock()

		// 创建临时配置项信息
		return &ConfigEntry{
			Key:       key,
			Value:     value,
			Type:      fmt.Sprintf("%T", value),
			Scope:     SessionScope,
			UpdatedAt: cm.getSessionUpdateTime(key),
		}, nil
	}
	cm.sessionMu.RUnlock()

	return nil, fmt.Errorf("配置项不存在: %s", key)
}

// getSessionUpdateTime 获取会话数据的更新时间（简化实现）
func (cm *configManager) getSessionUpdateTime(key string) time.Time {
	// 在实际实现中，可以维护一个单独的时间戳映射
	return time.Now()
}
