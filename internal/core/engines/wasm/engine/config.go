package engine

import (
	"fmt"
	"time"
)

// EngineConfig 引擎配置
// 控制WASM引擎底层运行时行为，与适配器Config协同工作
type EngineConfig struct {
	// ========== 基础配置 ==========

	// Name 引擎实例名称
	Name string `json:"name" yaml:"name"`

	// Version 引擎版本
	Version string `json:"version" yaml:"version"`

	// LogLevel 日志级别
	LogLevel string `json:"logLevel" yaml:"logLevel"`

	// ========== 内存配置 ==========

	// MaxLinearMemoryPages 最大线性内存页数（64KB/页）
	MaxLinearMemoryPages uint32 `json:"maxLinearMemoryPages" yaml:"maxLinearMemoryPages"`

	// InitialMemoryPages 初始内存页数
	InitialMemoryPages uint32 `json:"initialMemoryPages" yaml:"initialMemoryPages"`

	// EnableMemoryGrowth 是否允许内存增长
	EnableMemoryGrowth bool `json:"enableMemoryGrowth" yaml:"enableMemoryGrowth"`

	// ========== 编译配置 ==========

	// EnableAOT 是否启用AOT编译
	EnableAOT bool `json:"enableAOT" yaml:"enableAOT"`

	// EnableJIT 是否启用JIT编译
	EnableJIT bool `json:"enableJIT" yaml:"enableJIT"`

	// OptimizationLevel 优化级别 (0-3)
	OptimizationLevel int `json:"optimizationLevel" yaml:"optimizationLevel"`

	// CompileTimeout 编译超时时间
	CompileTimeout time.Duration `json:"compileTimeout" yaml:"compileTimeout"`

	// ========== 运行时配置 ==========

	// EnableWASI 是否启用WASI支持
	EnableWASI bool `json:"enableWASI" yaml:"enableWASI"`

	// EnableMultiValue 是否启用多值返回
	EnableMultiValue bool `json:"enableMultiValue" yaml:"enableMultiValue"`

	// EnableBulkMemoryOps 是否启用批量内存操作
	EnableBulkMemoryOps bool `json:"enableBulkMemoryOps" yaml:"enableBulkMemoryOps"`

	// EnableReferenceTypes 是否启用引用类型
	EnableReferenceTypes bool `json:"enableReferenceTypes" yaml:"enableReferenceTypes"`

	// ========== 性能配置 ==========

	// CacheSize 编译缓存大小
	CacheSize int `json:"cacheSize" yaml:"cacheSize"`

	// CacheTTL 缓存生存时间
	CacheTTL time.Duration `json:"cacheTTL" yaml:"cacheTTL"`

	// InstancePoolSize 实例池大小
	InstancePoolSize int `json:"instancePoolSize" yaml:"instancePoolSize"`

	// ========== 安全配置 ==========

	// EnableSandbox 是否启用沙箱
	EnableSandbox bool `json:"enableSandbox" yaml:"enableSandbox"`

	// MaxModuleSize 最大模块大小（字节）
	MaxModuleSize int64 `json:"maxModuleSize" yaml:"maxModuleSize"`

	// ExecutionTimeout 执行超时时间
	ExecutionTimeout time.Duration `json:"executionTimeout" yaml:"executionTimeout"`

	// ========== 调试配置 ==========

	// EnableDebug 是否启用调试模式
	EnableDebug bool `json:"enableDebug" yaml:"enableDebug"`

	// EnableProfiling 是否启用性能分析
	EnableProfiling bool `json:"enableProfiling" yaml:"enableProfiling"`

	// EnableMetrics 是否启用指标收集
	EnableMetrics bool `json:"enableMetrics" yaml:"enableMetrics"`
}

// ConfigManager 配置管理器
// 负责配置的加载、验证、更新和持久化
type ConfigManager struct {
	// 当前配置
	config *EngineConfig

	// 配置版本
	version string

	// 最后更新时间
	lastUpdated time.Time

	// 更新回调函数
	updateCallbacks []ConfigUpdateCallback
}

// ConfigUpdateCallback 配置更新回调函数类型
type ConfigUpdateCallback func(oldConfig, newConfig *EngineConfig) error

// ConfigValidationResult 配置验证结果
type ConfigValidationResult struct {
	// 是否有效
	Valid bool `json:"valid"`

	// 错误列表
	Errors []string `json:"errors"`

	// 警告列表
	Warnings []string `json:"warnings"`

	// 建议列表
	Suggestions []string `json:"suggestions"`
}

// NewConfigManager 创建配置管理器
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		config:          DefaultEngineConfig(),
		version:         "1.0.0",
		lastUpdated:     time.Now(),
		updateCallbacks: make([]ConfigUpdateCallback, 0),
	}
}

// GetConfig 获取当前配置
func (cm *ConfigManager) GetConfig() *EngineConfig {
	return cm.config
}

// UpdateConfig 更新配置
func (cm *ConfigManager) UpdateConfig(newConfig *EngineConfig) error {
	// 验证配置
	if result := cm.ValidateConfig(newConfig); !result.Valid {
		return fmt.Errorf("配置验证失败: %v", result.Errors)
	}

	oldConfig := cm.config

	// 触发更新回调
	for _, callback := range cm.updateCallbacks {
		if err := callback(oldConfig, newConfig); err != nil {
			return fmt.Errorf("配置更新回调失败: %w", err)
		}
	}

	// 更新配置
	cm.config = newConfig
	cm.lastUpdated = time.Now()

	return nil
}

// ValidateConfig 验证配置
func (cm *ConfigManager) ValidateConfig(config *EngineConfig) *ConfigValidationResult {
	result := &ConfigValidationResult{
		Valid:       true,
		Errors:      make([]string, 0),
		Warnings:    make([]string, 0),
		Suggestions: make([]string, 0),
	}

	// 验证内存配置
	if config.MaxLinearMemoryPages == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "最大线性内存页数不能为0")
	}

	if config.MaxLinearMemoryPages > 65536 { // 4GB限制
		result.Valid = false
		result.Errors = append(result.Errors, "最大线性内存页数过大（超过4GB）")
	}

	if config.InitialMemoryPages > config.MaxLinearMemoryPages {
		result.Valid = false
		result.Errors = append(result.Errors, "初始内存页数不能超过最大内存页数")
	}

	// 验证编译配置
	if config.OptimizationLevel < 0 || config.OptimizationLevel > 3 {
		result.Valid = false
		result.Errors = append(result.Errors, "优化级别必须在0-3之间")
	}

	if config.CompileTimeout <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "编译超时时间必须大于0")
	}

	// 验证性能配置
	if config.CacheSize < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "缓存大小不能为负数")
	}

	if config.InstancePoolSize <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "实例池大小必须大于0")
	}

	// 验证安全配置
	if config.MaxModuleSize <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "最大模块大小必须大于0")
	}

	if config.ExecutionTimeout <= 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "执行超时时间必须大于0")
	}

	// 添加警告和建议
	cm.addWarningsAndSuggestions(config, result)

	return result
}

// addWarningsAndSuggestions 添加警告和建议
func (cm *ConfigManager) addWarningsAndSuggestions(config *EngineConfig, result *ConfigValidationResult) {
	// 性能相关的警告
	if config.MaxLinearMemoryPages > 16384 { // 超过1GB
		result.Warnings = append(result.Warnings, "内存限制较高，注意资源消耗")
	}

	if !config.EnableAOT && !config.EnableJIT {
		result.Warnings = append(result.Warnings, "未启用AOT或JIT，性能可能受影响")
		result.Suggestions = append(result.Suggestions, "建议启用JIT以提高性能")
	}

	if config.CacheSize == 0 {
		result.Warnings = append(result.Warnings, "未启用编译缓存，可能影响性能")
		result.Suggestions = append(result.Suggestions, "建议设置适当的缓存大小")
	}

	// 安全相关的建议
	if !config.EnableSandbox {
		result.Warnings = append(result.Warnings, "未启用沙箱，存在安全风险")
		result.Suggestions = append(result.Suggestions, "强烈建议启用沙箱模式")
	}

	if config.ExecutionTimeout > 60*time.Second {
		result.Warnings = append(result.Warnings, "执行超时时间较长，可能影响响应性")
	}

	// 优化建议
	if config.OptimizationLevel == 0 {
		result.Suggestions = append(result.Suggestions, "建议启用适当的优化级别以提高性能")
	}
}

// RegisterUpdateCallback 注册配置更新回调
func (cm *ConfigManager) RegisterUpdateCallback(callback ConfigUpdateCallback) {
	cm.updateCallbacks = append(cm.updateCallbacks, callback)
}

// GetVersion 获取配置版本
func (cm *ConfigManager) GetVersion() string {
	return cm.version
}

// GetLastUpdated 获取最后更新时间
func (cm *ConfigManager) GetLastUpdated() time.Time {
	return cm.lastUpdated
}

// DefaultEngineConfig 默认引擎配置
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		// 基础配置
		Name:     "wasm-engine",
		Version:  "0.0.1",
		LogLevel: "info",

		// 内存配置
		MaxLinearMemoryPages: 1024, // 64MB
		InitialMemoryPages:   16,   // 1MB
		EnableMemoryGrowth:   true,

		// 编译配置
		EnableAOT:         true,
		EnableJIT:         true,
		OptimizationLevel: 2,
		CompileTimeout:    10 * time.Second,

		// 运行时配置
		EnableWASI:           true,
		EnableMultiValue:     true,
		EnableBulkMemoryOps:  true,
		EnableReferenceTypes: false,

		// 性能配置
		CacheSize:        100,
		CacheTTL:         time.Hour,
		InstancePoolSize: 10,

		// 安全配置
		EnableSandbox:    true,
		MaxModuleSize:    10 * 1024 * 1024, // 10MB
		ExecutionTimeout: 30 * time.Second,

		// 调试配置
		EnableDebug:     false,
		EnableProfiling: false,
		EnableMetrics:   true,
	}
}

// Clone 克隆配置
func (ec *EngineConfig) Clone() *EngineConfig {
	clone := *ec
	return &clone
}

// Merge 合并配置
func (ec *EngineConfig) Merge(other *EngineConfig) *EngineConfig {
	merged := ec.Clone()

	// 非零值覆盖
	if other.Name != "" {
		merged.Name = other.Name
	}
	if other.Version != "" {
		merged.Version = other.Version
	}
	if other.LogLevel != "" {
		merged.LogLevel = other.LogLevel
	}
	if other.MaxLinearMemoryPages != 0 {
		merged.MaxLinearMemoryPages = other.MaxLinearMemoryPages
	}
	if other.InitialMemoryPages != 0 {
		merged.InitialMemoryPages = other.InitialMemoryPages
	}
	if other.OptimizationLevel != 0 {
		merged.OptimizationLevel = other.OptimizationLevel
	}
	if other.CompileTimeout != 0 {
		merged.CompileTimeout = other.CompileTimeout
	}
	if other.CacheSize != 0 {
		merged.CacheSize = other.CacheSize
	}
	if other.CacheTTL != 0 {
		merged.CacheTTL = other.CacheTTL
	}
	if other.InstancePoolSize != 0 {
		merged.InstancePoolSize = other.InstancePoolSize
	}
	if other.MaxModuleSize != 0 {
		merged.MaxModuleSize = other.MaxModuleSize
	}
	if other.ExecutionTimeout != 0 {
		merged.ExecutionTimeout = other.ExecutionTimeout
	}

	// 布尔值直接覆盖
	merged.EnableMemoryGrowth = other.EnableMemoryGrowth
	merged.EnableAOT = other.EnableAOT
	merged.EnableJIT = other.EnableJIT
	merged.EnableWASI = other.EnableWASI
	merged.EnableMultiValue = other.EnableMultiValue
	merged.EnableBulkMemoryOps = other.EnableBulkMemoryOps
	merged.EnableReferenceTypes = other.EnableReferenceTypes
	merged.EnableSandbox = other.EnableSandbox
	merged.EnableDebug = other.EnableDebug
	merged.EnableProfiling = other.EnableProfiling
	merged.EnableMetrics = other.EnableMetrics

	return merged
}

// ToMap 转换为映射
func (ec *EngineConfig) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"name":                 ec.Name,
		"version":              ec.Version,
		"logLevel":             ec.LogLevel,
		"maxLinearMemoryPages": ec.MaxLinearMemoryPages,
		"initialMemoryPages":   ec.InitialMemoryPages,
		"enableMemoryGrowth":   ec.EnableMemoryGrowth,
		"enableAOT":            ec.EnableAOT,
		"enableJIT":            ec.EnableJIT,
		"optimizationLevel":    ec.OptimizationLevel,
		"compileTimeout":       ec.CompileTimeout,
		"enableWASI":           ec.EnableWASI,
		"enableMultiValue":     ec.EnableMultiValue,
		"enableBulkMemoryOps":  ec.EnableBulkMemoryOps,
		"enableReferenceTypes": ec.EnableReferenceTypes,
		"cacheSize":            ec.CacheSize,
		"cacheTTL":             ec.CacheTTL,
		"instancePoolSize":     ec.InstancePoolSize,
		"enableSandbox":        ec.EnableSandbox,
		"maxModuleSize":        ec.MaxModuleSize,
		"executionTimeout":     ec.ExecutionTimeout,
		"enableDebug":          ec.EnableDebug,
		"enableProfiling":      ec.EnableProfiling,
		"enableMetrics":        ec.EnableMetrics,
	}
}
