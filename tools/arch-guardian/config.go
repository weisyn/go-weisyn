package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 架构守护工具配置
type Config struct {
	Whitelist    WhitelistConfig    `yaml:"whitelist"`
	Rules        RulesConfig        `yaml:"rules"`
	Reporting    ReportingConfig    `yaml:"reporting"`
	Performance  PerformanceConfig  `yaml:"performance"`
	Integration  IntegrationConfig  `yaml:"integration"`
	Experimental ExperimentalConfig `yaml:"experimental"`
}

// WhitelistConfig 白名单配置
type WhitelistConfig struct {
	Directories  []string `yaml:"directories"`
	FilePatterns []string `yaml:"file_patterns"`
	Files        []string `yaml:"files"`
}

// RulesConfig 规则配置
type RulesConfig struct {
	DirectPublicInterface DirectPublicInterfaceConfig `yaml:"direct_public_interface"`
	CrossModuleDependency CrossModuleDependencyConfig `yaml:"cross_module_dependency"`
	ManagerComplexity     ManagerComplexityConfig     `yaml:"manager_complexity"`
	HardcodedConstant     HardcodedConstantConfig     `yaml:"hardcoded_constant"`
	InterfaceConsistency  InterfaceConsistencyConfig  `yaml:"interface_consistency"`
	NamingConvention      NamingConventionConfig      `yaml:"naming_convention"`
}

// DirectPublicInterfaceConfig 直接实现公共接口规则配置
type DirectPublicInterfaceConfig struct {
	Enabled    bool              `yaml:"enabled"`
	Severity   string            `yaml:"severity"`
	Exceptions []ExceptionConfig `yaml:"exceptions"`
}

// CrossModuleDependencyConfig 跨模块依赖规则配置
type CrossModuleDependencyConfig struct {
	Enabled             bool                      `yaml:"enabled"`
	Severity            string                    `yaml:"severity"`
	AllowedDependencies []AllowedDependencyConfig `yaml:"allowed_dependencies"`
}

// ManagerComplexityConfig Manager复杂度规则配置
type ManagerComplexityConfig struct {
	Enabled    bool                  `yaml:"enabled"`
	Severity   string                `yaml:"severity"`
	Thresholds ComplexityThresholds  `yaml:"thresholds"`
	Exceptions []FileExceptionConfig `yaml:"exceptions"`
}

// HardcodedConstantConfig 硬编码常量规则配置
type HardcodedConstantConfig struct {
	Enabled       bool                 `yaml:"enabled"`
	Severity      string               `yaml:"severity"`
	WasmFunctions []WasmFunctionConfig `yaml:"wasm_functions"`
	Exceptions    []ExceptionConfig    `yaml:"exceptions"`
}

// InterfaceConsistencyConfig 接口一致性规则配置
type InterfaceConsistencyConfig struct {
	Enabled          bool   `yaml:"enabled"`
	Severity         string `yaml:"severity"`
	CheckInheritance bool   `yaml:"check_inheritance"`
	CheckSignatures  bool   `yaml:"check_signatures"`
}

// NamingConventionConfig 命名规范规则配置
type NamingConventionConfig struct {
	Enabled  bool                           `yaml:"enabled"`
	Severity string                         `yaml:"severity"`
	Patterns map[string]NamingPatternConfig `yaml:"patterns"`
}

// ExceptionConfig 例外配置
type ExceptionConfig struct {
	Pattern string `yaml:"pattern"`
	Reason  string `yaml:"reason"`
}

// AllowedDependencyConfig 允许的依赖配置
type AllowedDependencyConfig struct {
	From   string `yaml:"from"`
	To     string `yaml:"to"`
	Reason string `yaml:"reason"`
}

// ComplexityThresholds 复杂度阈值
type ComplexityThresholds struct {
	MaxFileLines            int `yaml:"max_file_lines"`
	MaxMethodStatements     int `yaml:"max_method_statements"`
	MaxCyclomaticComplexity int `yaml:"max_cyclomatic_complexity"`
}

// FileExceptionConfig 文件例外配置
type FileExceptionConfig struct {
	File         string `yaml:"file"`
	Reason       string `yaml:"reason"`
	MaxFileLines int    `yaml:"max_file_lines,omitempty"`
}

// WasmFunctionConfig WASM函数配置
type WasmFunctionConfig struct {
	Name     string `yaml:"name"`
	Constant string `yaml:"constant"`
}

// NamingPatternConfig 命名模式配置
type NamingPatternConfig struct {
	Pattern string `yaml:"pattern"`
	Message string `yaml:"message"`
}

// ReportingConfig 报告配置
type ReportingConfig struct {
	Format            string `yaml:"format"`
	Verbosity         string `yaml:"verbosity"`
	ShowSuggestions   bool   `yaml:"show_suggestions"`
	GenerateFixScript bool   `yaml:"generate_fix_script"`
	OutputFile        string `yaml:"output_file"`
	GroupBySeverity   bool   `yaml:"group_by_severity"`
	ShowStatistics    bool   `yaml:"show_statistics"`
}

// PerformanceConfig 性能配置
type PerformanceConfig struct {
	MaxConcurrentFiles int    `yaml:"max_concurrent_files"`
	MaxMemoryMB        int    `yaml:"max_memory_mb"`
	TimeoutSeconds     int    `yaml:"timeout_seconds"`
	EnableCache        bool   `yaml:"enable_cache"`
	CacheDir           string `yaml:"cache_dir"`
}

// IntegrationConfig 集成配置
type IntegrationConfig struct {
	Git GitConfig `yaml:"git"`
	CI  CIConfig  `yaml:"ci"`
	IDE IDEConfig `yaml:"ide"`
}

// GitConfig Git集成配置
type GitConfig struct {
	Enabled               bool   `yaml:"enabled"`
	CheckChangedFilesOnly bool   `yaml:"check_changed_files_only"`
	BaseBranch            string `yaml:"base_branch"`
}

// CIConfig CI/CD集成配置
type CIConfig struct {
	Enabled         bool `yaml:"enabled"`
	FailureExitCode int  `yaml:"failure_exit_code"`
	CommentOnPR     bool `yaml:"comment_on_pr"`
}

// IDEConfig IDE集成配置
type IDEConfig struct {
	Enabled       bool `yaml:"enabled"`
	LSPServerPort int  `yaml:"lsp_server_port"`
	RealTimeCheck bool `yaml:"real_time_check"`
}

// ExperimentalConfig 实验性功能配置
type ExperimentalConfig struct {
	MLDetection        MLDetectionConfig        `yaml:"ml_detection"`
	AutoFixSuggestions AutoFixSuggestionsConfig `yaml:"auto_fix_suggestions"`
	DebtAssessment     DebtAssessmentConfig     `yaml:"debt_assessment"`
	TrendAnalysis      TrendAnalysisConfig      `yaml:"trend_analysis"`
}

// MLDetectionConfig 机器学习检测配置
type MLDetectionConfig struct {
	Enabled   bool   `yaml:"enabled"`
	ModelPath string `yaml:"model_path"`
}

// AutoFixSuggestionsConfig 自动修复建议配置
type AutoFixSuggestionsConfig struct {
	Enabled             bool    `yaml:"enabled"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
}

// DebtAssessmentConfig 架构债务评估配置
type DebtAssessmentConfig struct {
	Enabled       bool `yaml:"enabled"`
	DebtThreshold int  `yaml:"debt_threshold"`
}

// TrendAnalysisConfig 趋势分析配置
type TrendAnalysisConfig struct {
	Enabled     bool `yaml:"enabled"`
	HistoryDays int  `yaml:"history_days"`
}

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	// 如果没有指定配置文件，使用默认配置
	if configPath == "" {
		return getDefaultConfig(), nil
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// 解析YAML配置
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// getDefaultConfig 获取默认配置
func getDefaultConfig() *Config {
	return &Config{
		Whitelist: WhitelistConfig{
			Directories: []string{
				"internal/core/ispc/engines/wasm/interfaces", // WASM引擎内部接口
				"internal/core/ispc/interfaces",              // ISPC内部接口
				"internal/core/execution/interfaces",
				"internal/core/blockchain/interfaces",
				"tools",
				"scripts",
				"cmd",
				"examples",
				"docs",
			},
			FilePatterns: []string{
				"*_test.go",
				"*_mock.go",
				"integration_*",
				"benchmark_*",
			},
			Files: []string{
				"internal/core/ispc/module.go", // ISPC模块（包含engines）
				"internal/core/execution/module.go",
				"internal/core/blockchain/module.go",
			},
		},
		Rules: RulesConfig{
			DirectPublicInterface: DirectPublicInterfaceConfig{
				Enabled:  true,
				Severity: "ERROR",
			},
			CrossModuleDependency: CrossModuleDependencyConfig{
				Enabled:  true,
				Severity: "ERROR",
			},
			ManagerComplexity: ManagerComplexityConfig{
				Enabled:  true,
				Severity: "WARNING",
				Thresholds: ComplexityThresholds{
					MaxFileLines:            200,
					MaxMethodStatements:     20,
					MaxCyclomaticComplexity: 10,
				},
			},
			HardcodedConstant: HardcodedConstantConfig{
				Enabled:  true,
				Severity: "WARNING",
			},
			InterfaceConsistency: InterfaceConsistencyConfig{
				Enabled:          true,
				Severity:         "WARNING",
				CheckInheritance: true,
				CheckSignatures:  true,
			},
			NamingConvention: NamingConventionConfig{
				Enabled:  true,
				Severity: "INFO",
			},
		},
		Reporting: ReportingConfig{
			Format:            "console",
			Verbosity:         "normal",
			ShowSuggestions:   true,
			GenerateFixScript: true,
			GroupBySeverity:   true,
			ShowStatistics:    true,
		},
		Performance: PerformanceConfig{
			MaxConcurrentFiles: 10,
			MaxMemoryMB:        512,
			TimeoutSeconds:     300,
			EnableCache:        true,
			CacheDir:           ".arch-guardian-cache",
		},
	}
}

// IsWhitelisted 检查文件是否在白名单中
func (c *Config) IsWhitelisted(filePath string) bool {
	// 检查目录白名单
	for _, dir := range c.Whitelist.Directories {
		if strings.Contains(filePath, dir) {
			return true
		}
	}

	// 检查文件模式白名单
	fileName := filepath.Base(filePath)
	for _, pattern := range c.Whitelist.FilePatterns {
		if matched, _ := filepath.Match(pattern, fileName); matched {
			return true
		}
	}

	// 检查具体文件白名单
	for _, file := range c.Whitelist.Files {
		if strings.Contains(filePath, file) {
			return true
		}
	}

	return false
}

// IsRuleEnabled 检查规则是否启用
func (c *Config) IsRuleEnabled(ruleName string) bool {
	switch ruleName {
	case "DirectPublicInterface":
		return c.Rules.DirectPublicInterface.Enabled
	case "CrossModuleDependency":
		return c.Rules.CrossModuleDependency.Enabled
	case "ManagerComplexity":
		return c.Rules.ManagerComplexity.Enabled
	case "HardcodedConstant":
		return c.Rules.HardcodedConstant.Enabled
	case "InterfaceConsistency":
		return c.Rules.InterfaceConsistency.Enabled
	case "NamingConvention":
		return c.Rules.NamingConvention.Enabled
	default:
		return false
	}
}

// GetRuleSeverity 获取规则严重程度
func (c *Config) GetRuleSeverity(ruleName string) string {
	switch ruleName {
	case "DirectPublicInterface":
		return c.Rules.DirectPublicInterface.Severity
	case "CrossModuleDependency":
		return c.Rules.CrossModuleDependency.Severity
	case "ManagerComplexity":
		return c.Rules.ManagerComplexity.Severity
	case "HardcodedConstant":
		return c.Rules.HardcodedConstant.Severity
	case "InterfaceConsistency":
		return c.Rules.InterfaceConsistency.Severity
	case "NamingConvention":
		return c.Rules.NamingConvention.Severity
	default:
		return "INFO"
	}
}

// IsExceptionMatch 检查是否匹配例外规则
func (c *Config) IsExceptionMatch(ruleName, filePath string) (bool, string) {
	var exceptions []ExceptionConfig

	switch ruleName {
	case "DirectPublicInterface":
		exceptions = c.Rules.DirectPublicInterface.Exceptions
	case "HardcodedConstant":
		exceptions = c.Rules.HardcodedConstant.Exceptions
	}

	for _, exception := range exceptions {
		if matched, _ := regexp.MatchString(exception.Pattern, filePath); matched {
			return true, exception.Reason
		}
	}

	return false, ""
}

// GetComplexityThresholds 获取复杂度阈值
func (c *Config) GetComplexityThresholds(filePath string) ComplexityThresholds {
	// 检查文件特定的例外配置
	for _, exception := range c.Rules.ManagerComplexity.Exceptions {
		if strings.Contains(filePath, exception.File) {
			thresholds := c.Rules.ManagerComplexity.Thresholds
			if exception.MaxFileLines > 0 {
				thresholds.MaxFileLines = exception.MaxFileLines
			}
			return thresholds
		}
	}

	return c.Rules.ManagerComplexity.Thresholds
}
