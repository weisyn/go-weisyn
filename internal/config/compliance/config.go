// Package compliance 提供WES系统的合规配置管理
//
// 🛡️ **合规配置管理 (Compliance Configuration Management)**
//
// 本包提供WES系统合规功能的配置管理，包括：
// - 地理区域限制配置
// - 操作类型限制配置
// - 身份验证提供方配置
// - 网关GeoIP查询配置
// - 合规策略热更新配置
//
// 🎯 **设计原则**
// - 默认允许：系统默认状态为允许所有操作，需显式启用合规控制
// - 配置驱动：所有合规规则通过配置文件定义，支持运行时更新
// - 多层防护：支持身份凭证、GeoIP、P2P地理特征等多重判定信源
// - 操作细分：支持对转账、合约调用等不同操作类型的精细控制
package compliance

import (
	"time"
)

// ComplianceOptions 合规配置选项
//
// 🔧 **合规配置结构 (Compliance Configuration Structure)**
//
// 定义了WES系统合规功能的完整配置选项，包含地理限制、
// 操作限制、身份验证、网关集成等各个层面的配置参数。
type ComplianceOptions struct {
	// ========== 基础控制配置 ==========

	// Enabled 是否启用合规控制
	// true: 启用合规检查，根据配置规则过滤交易
	// false: 禁用合规检查，允许所有交易（默认）
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ========== 地理限制配置 ==========

	// BannedCountries 被禁用的国家列表
	// 使用ISO-3166-1 alpha-2标准国家代码（如"CN","US","JP"）
	// 空列表表示不限制任何国家
	BannedCountries []string `json:"banned_countries" yaml:"banned_countries"`

	// RejectOnUnknownCountry 是否拒绝未知来源地区的请求
	// true: 无法确定来源地区时拒绝请求
	// false: 无法确定来源地区时允许请求（默认）
	RejectOnUnknownCountry bool `json:"reject_on_unknown_country" yaml:"reject_on_unknown_country"`

	// ========== 操作限制配置 ==========

	// BannedOperations 被禁用的操作类型列表
	// 支持的操作类型:
	// - "transfer": 普通转账操作
	// - "contract.*": 所有合约调用
	// - "contract.payments.*": 支付相关合约方法
	// - "contract.specific_address.method_name": 特定合约的特定方法
	BannedOperations []string `json:"banned_operations" yaml:"banned_operations"`

	// ========== DB-IP地理位置配置（系统自包含）==========

	// GeoIP 地理位置查询配置
	GeoIP GeoIPConfig `json:"geoip" yaml:"geoip"`

	// ========== 热更新配置 ==========

	// HotReload 配置热更新功能设置
	HotReload HotReloadConfig `json:"hot_reload" yaml:"hot_reload"`
}

// GeoIPConfig 地理位置查询配置
//
// 🌍 **DB-IP地理位置查询配置 (DB-IP GeoIP Query Configuration)**
//
// 基于DB-IP免费数据库的地理位置查询配置。
// 使用Creative Commons Attribution 4.0协议，需提供attribution链接。
type GeoIPConfig struct {
	// DatabasePath DB-IP数据库文件路径（MMDB格式）
	// 空字符串表示禁用GeoIP地理位置查询功能
	// 默认: "./data/compliance/dbip-country-lite.mmdb"
	DatabasePath string `json:"database_path" yaml:"database_path"`

	// UpdateURL DB-IP数据库下载地址
	// 用于定期更新DB-IP数据库的下载URL（gzip压缩格式）
	// 默认: DB-IP免费版每月更新链接
	UpdateURL string `json:"update_url" yaml:"update_url"`

	// AutoUpdate 自动更新数据库
	// 是否启用定期自动下载和更新DB-IP数据库
	AutoUpdate bool `json:"auto_update" yaml:"auto_update"`

	// UpdateInterval 数据库更新间隔
	// 自动更新的时间间隔（建议每月更新）
	UpdateInterval time.Duration `json:"update_interval" yaml:"update_interval"`

	// CacheTTL GeoIP查询结果缓存时长
	// IP地址到国家代码映射的缓存有效期
	CacheTTL time.Duration `json:"cache_ttl" yaml:"cache_ttl"`

	// Attribution DB-IP attribution要求
	// 根据Creative Commons协议要求显示的attribution信息
	// 默认: "IP Geolocation by DB-IP"
	Attribution string `json:"attribution" yaml:"attribution"`
}

// HotReloadConfig 配置热更新功能设置
//
// 🔄 **热更新配置 (Hot Reload Configuration)**
//
// 配置合规策略的动态更新机制，支持无重启更新合规规则。
type HotReloadConfig struct {
	// Enabled 是否启用配置热更新
	// true: 监听配置文件变更并自动重载
	// false: 需要重启服务才能应用配置变更（默认）
	Enabled bool `json:"enabled" yaml:"enabled"`

	// ConfigCheckInterval 配置文件变更检查间隔
	// 定期检查配置文件修改时间的间隔
	ConfigCheckInterval time.Duration `json:"config_check_interval" yaml:"config_check_interval"`

	// PolicyUpdateTimeout 策略更新操作超时时间
	// 应用新配置策略的最大处理时间
	PolicyUpdateTimeout time.Duration `json:"policy_update_timeout" yaml:"policy_update_timeout"`
}

// Config 合规配置管理器
//
// 🔧 **配置管理器 (Configuration Manager)**
//
// 负责合规配置的加载、验证、合并和访问。
type Config struct {
	options *ComplianceOptions // 配置选项实例
}

// New 创建合规配置实例
//
// 📝 **环境感知配置初始化流程 (Environment-Aware Configuration Initialization)**
//
// 创建合规配置管理器实例，处理环境感知和用户配置覆盖：
// 1. 根据网络类型自动决定合规启用状态（系统级决策）
// 2. 创建包含所有默认值的配置选项
// 3. 应用用户提供的配置覆盖默认值（仅限非安全相关参数）
// 4. 验证配置的有效性和一致性
// 5. 返回最终的配置管理器实例
//
// 参数:
// - userConfig: 用户提供的配置数据，可以是*types.UserComplianceConfig或nil
// - networkType: 网络类型 ("development"/"testnet"/"mainnet")，用于环境感知安全控制
//
// 返回:
// - *Config: 配置管理器实例
func New(userConfig interface{}, networkType string) *Config {
	// 创建完全自包含的合规配置
	// 用户配置被忽略，系统完全自包含，无需用户干预
	defaultOptions := createDefaultComplianceOptions(networkType)

	// 验证和调整内置配置
	validateAndAdjustConfig(defaultOptions)

	return &Config{
		options: defaultOptions,
	}
}

// GetOptions 获取配置选项
//
// 📊 **配置选项访问器 (Configuration Options Accessor)**
//
// 返回当前的合规配置选项，供其他模块使用。
//
// 返回:
// - *ComplianceOptions: 合规配置选项
func (c *Config) GetOptions() *ComplianceOptions {
	return c.options
}

// ============================================================================
//                           ⚙️ 配置处理辅助函数
// ============================================================================

// validateAndAdjustConfig 验证和调整配置
//
// ✅ **配置验证器 (Configuration Validator)**
//
// 验证配置的有效性并进行必要的调整。
func validateAndAdjustConfig(config *ComplianceOptions) {
	// 验证国家代码格式（ISO-3166-1 alpha-2）
	config.BannedCountries = validateCountryCodes(config.BannedCountries)

	// 验证操作类型格式
	config.BannedOperations = validateOperationTypes(config.BannedOperations)

	// 验证GeoIP配置的合理性
	if config.GeoIP.CacheTTL < 10*time.Minute {
		config.GeoIP.CacheTTL = time.Hour // 最小1小时缓存
	}
}

// validateCountryCodes 验证国家代码格式
func validateCountryCodes(codes []string) []string {
	var validCodes []string
	for _, code := range codes {
		// ISO-3166-1 alpha-2标准：2个大写字母
		if len(code) == 2 {
			validCodes = append(validCodes, code)
		}
	}
	return validCodes
}

// validateOperationTypes 验证操作类型格式
func validateOperationTypes(operations []string) []string {
	var validOperations []string
	validPatterns := map[string]bool{
		"transfer":              true,
		"contract.*":            true,
		"contract.payments.*":   true,
		"contract.governance.*": true,
		"contract.staking.*":    true,
	}

	for _, op := range operations {
		// 检查是否为预定义的有效模式
		if validPatterns[op] {
			validOperations = append(validOperations, op)
		} else {
			// 检查是否为特定合约地址+方法的格式
			// 格式: contract.{address}.{method}
			// 简化验证：包含"contract."前缀即认为有效
			if len(op) > 9 && op[:9] == "contract." {
				validOperations = append(validOperations, op)
			}
		}
	}
	return validOperations
}

// convertToStringSlice 将interface{}切片转换为字符串切片
func convertToStringSlice(slice []interface{}) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}
	return result
}
