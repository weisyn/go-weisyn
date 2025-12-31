package compliance

import (
	"time"
)

// ============================================================================
//                            🚫 不可绕过的安全配置
// ============================================================================

// IMMUTABLE_BANNED_COUNTRIES 系统级禁用国家清单（用户配置无法覆盖）
//
// 🌍 **禁用国家分析与依据**
//
// 该清单基于以下权威机构的制裁和风险评估：
// - 联合国安理会制裁决议
// - 美国财政部外国资产控制办公室(OFAC)
// - 金融行动特别工作组(FATF)高风险清单
// - 各国数字资产监管政策
var IMMUTABLE_BANNED_COUNTRIES = []string{
	// === 联合国安理会全面制裁 ===
	"KP", // 朝鲜 - 联合国全面制裁，禁止所有金融服务
	"IR", // 伊朗 - 核计划相关制裁，金融交易严格受限
	"SY", // 叙利亚 - 人道主义危机相关制裁

	// === 美国OFAC重点制裁 ===
	"US", // 美国 - 未注册数字资产服务商面临严格监管
	"CU", // 古巴 - 美国长期经济制裁
	"VE", // 委内瑞拉 - 政府及相关实体制裁
	"MM", // 缅甸 - 军政府相关制裁

	// === FATF高风险司法管辖区 ===
	"AF", // 阿富汗 - 政治不稳定，监管机制缺失
	"LB", // 黎巴嫩 - 金融系统危机，洗钱风险极高
	"YE", // 也门 - 战争状态，监管执行真空
	"LY", // 利比亚 - 政治分裂，监管执行力薄弱

	// === 数字资产监管严格地区 ===
	"CN", // 中国 - 数字货币交易全面禁止
	"BD", // 孟加拉国 - 加密货币交易被认定为非法
	"NP", // 尼泊尔 - 加密货币使用被禁止

	// === 其他高风险地区 ===
	"SO", // 索马里 - 持续的政治不稳定和监管缺失
	"SD", // 苏丹 - 国际制裁和政治动荡
	"ER", // 厄立特里亚 - 专制政权，国际制裁
}

// IMMUTABLE_BANNED_OPERATIONS 系统级禁用操作清单（用户配置无法覆盖）
//
// ⚠️ **高风险操作分析**
//
// 基于反洗钱(AML)、反恐融资(CTF)和监管合规要求识别的高风险操作类型
var IMMUTABLE_BANNED_OPERATIONS = []string{
	// === 基础资金转移类 ===
	"transfer", // 普通转账 - 最基础的价值转移，监管重点关注

	// === 支付合约类 ===
	"contract.payments.send",      // 单笔合约支付 - 可能规避传统金融监管
	"contract.payments.batch",     // 批量合约支付 - 常用于资金分拆逃避监控
	"contract.payments.scheduled", // 定时支付 - 可能用于自动化可疑交易
	"contract.payments.recurring", // 循环支付 - 可能掩盖持续的非法资金流动

	// === 治理参与类 ===
	"contract.governance.voting",   // 治理投票 - 可能影响系统关键规则
	"contract.governance.proposal", // 治理提案 - 可能提出绕过合规的提案
	"contract.governance.execute",  // 治理执行 - 可能执行有害的治理决定

	// === 隐私增强类（高风险）===
	"contract.mixer.*",     // 混币相关 - 显著增强交易隐私，规避追踪
	"contract.privacy.*",   // 隐私保护 - 可能完全规避审计和监管追踪
	"contract.tumbler.*",   // 翻滚器 - 专门用于混淆资金来源
	"contract.anonymity.*", // 匿名化 - 完全隐匿交易参与方身份

	// === 系统管理类（超高风险）===
	"contract.admin.*",     // 管理权限操作 - 系统级别权限，可能被滥用
	"contract.upgrade.*",   // 合约升级 - 可能通过升级绕过现有限制
	"contract.emergency.*", // 应急操作 - 可能被用作后门机制

	// === 跨链和桥接类 ===
	"contract.bridge.*",     // 跨链桥接 - 可能用于跨司法管辖区转移资产
	"contract.crosschain.*", // 跨链操作 - 增加监管复杂性和追踪难度
	"contract.atomic.*",     // 原子交换 - 可能用于规避集中式交易所监管

	// === 借贷和DeFi类 ===
	"contract.lending.flash", // 闪电贷 - 常被用于套利和操纵市场
	"contract.derivatives.*", // 衍生品交易 - 高风险金融工具
	"contract.leveraged.*",   // 杠杆交易 - 高风险投资操作
	"contract.liquidation.*", // 强制清算 - 可能涉及资产强制转移
}

// COMPLIANCE_CONFIG_METADATA 合规配置元数据
var COMPLIANCE_CONFIG_METADATA = struct {
	Version         string
	LastUpdateDate  string
	SanctionsSource string
	UpdatedBy       string
	NextReviewDate  string
}{
	Version:         "1.0.0",
	LastUpdateDate:  "2024-01-15",
	SanctionsSource: "UN/OFAC/FATF-2024-Q1",
	UpdatedBy:       "WES Compliance Team",
	NextReviewDate:  "2024-04-15", // 季度审查
}

// ============================================================================
//                          🛡️ 环境感知安全配置系统
// ============================================================================

// 环境类型定义
const (
	EnvDevelopment = "development"
	EnvTesting     = "testing"
	EnvProduction  = "production"
)

// 🔧 **环境感知合规控制策略**
//
// 合规系统根据运行环境自动决定启用策略：
// - Development/Testing: 自动禁用，便于开发调试
// - Production: 强制启用，确保生产安全
//
// 此设计确保：
// 1. 开发者无需手动配置即可正常开发
// 2. 生产环境安全规则不可被用户配置绕过
// 3. 系统级安全控制与用户配置完全分离
func isComplianceEnabledByEnvironment(networkType string) bool {
	switch networkType {
	case "development":
		return false // 开发环境：禁用合规，便于调试
	case "testnet", "testing":
		return false // 测试环境：禁用合规，便于测试
	case "mainnet", "production":
		return true // 生产环境：强制启用合规
	default:
		// 未知环境类型：安全优先，启用合规
		return true
	}
}

// 🌍 **环境感知GeoIP自动更新策略**
//
// 根据运行环境自动决定DB-IP数据库自动更新策略：
// - Development/Testing: 禁用自动更新，避免网络依赖导致启动失败
// - Production: 启用自动更新，确保地理位置数据的准确性
//
// 此设计确保：
// 1. 开发环境避免因网络问题导致启动失败
// 2. 生产环境保持地理位置数据的时效性
// 3. 降级处理确保辅助功能不阻塞核心业务
func getGeoIPAutoUpdateByEnvironment(networkType string) bool {
	switch networkType {
	case "development":
		return false // 开发环境：禁用自动更新，避免网络依赖
	case "testnet", "testing":
		return false // 测试环境：禁用自动更新，专注测试逻辑
	case "mainnet", "production":
		return true // 生产环境：启用自动更新，确保数据准确性
	default:
		// 未知环境类型：保守策略，禁用自动更新
		return false
	}
}

// ============================================================================
//                              🔧 默认配置常量
// ============================================================================

// 基础配置默认值（非安全相关，可由用户配置覆盖）
const (
	// === 地理限制默认值 ===
	defaultRejectOnUnknownCountry = true // 默认拒绝未知来源地区（安全优先）

	// === DB-IP GeoIP默认值 ===
	defaultGeoIPDatabasePath   = "./data/compliance/dbip-country-lite.mmdb"                          // DB-IP数据库路径
	defaultGeoIPUpdateURL      = "https://download.db-ip.com/free/dbip-country-lite-2025-09.mmdb.gz" // DB-IP下载URL
	defaultGeoIPCacheTTL       = 4 * time.Hour                                                       // GeoIP查询结果缓存时长
	defaultGeoIPAutoUpdate     = true                                                                // 自动更新数据库
	defaultGeoIPUpdateInterval = 24 * 30 * time.Hour                                                 // 每月更新一次
	defaultGeoIPAttribution    = "IP Geolocation by DB-IP"                                           // CC协议required attribution

	// === 热更新默认值 ===
	defaultHotReloadEnabled    = false            // 默认禁用配置热更新
	defaultConfigCheckInterval = 30 * time.Second // 配置文件变更检查间隔
	defaultPolicyUpdateTimeout = 5 * time.Second  // 策略更新超时
)

// createDefaultComplianceOptions 创建默认的合规配置选项
//
// 🔧 **环境感知安全配置生成器 (Environment-Aware Security Configuration Generator)**
//
// 根据运行环境和国际制裁清单，自动生成合规配置：
// - 开发/测试环境：自动禁用合规，确保开发便利性
// - 生产环境：强制启用合规，确保生产安全性
//
// 采用"安全优先，硬编码核心规则，环境感知"的策略，确保：
// 1. 关键合规限制不可被用户配置绕过
// 2. 环境差异由系统自动处理，无需用户干预
// 3. 开发和生产环境的安全策略完全分离
//
// 参数：
// - networkType: 网络类型 ("development"/"testnet"/"mainnet")
//
// 返回：
// - *ComplianceOptions: 包含系统级安全规则的完整合规配置
func createDefaultComplianceOptions(networkType string) *ComplianceOptions {
	// 🛡️ 根据环境自动决定合规启用状态（系统级决策，用户无法覆盖）
	complianceEnabled := isComplianceEnabledByEnvironment(networkType)

	// 🌍 根据环境决定DB-IP自动更新策略（开发环境避免网络依赖）
	geoipAutoUpdate := getGeoIPAutoUpdateByEnvironment(networkType)

	return &ComplianceOptions{
		// ========== 系统级安全控制（环境感知）==========
		Enabled:                complianceEnabled,             // 系统根据环境自动决定
		RejectOnUnknownCountry: defaultRejectOnUnknownCountry, // 安全优先策略

		// ========== 系统级强制限制（不可绕过）==========
		BannedCountries:  append([]string{}, IMMUTABLE_BANNED_COUNTRIES...),  // 复制硬编码国家清单
		BannedOperations: append([]string{}, IMMUTABLE_BANNED_OPERATIONS...), // 复制硬编码操作清单

		// ========== DB-IP地理位置服务配置（系统自包含）==========
		GeoIP: GeoIPConfig{
			DatabasePath:   defaultGeoIPDatabasePath,   // DB-IP数据库路径
			UpdateURL:      defaultGeoIPUpdateURL,      // DB-IP免费版下载URL
			AutoUpdate:     geoipAutoUpdate,            // 根据环境决定是否自动更新
			UpdateInterval: defaultGeoIPUpdateInterval, // 每月更新间隔
			CacheTTL:       defaultGeoIPCacheTTL,       // 4小时缓存
			Attribution:    defaultGeoIPAttribution,    // CC协议required attribution
		},

		// ========== 热重载配置 ==========
		HotReload: HotReloadConfig{
			Enabled:             defaultHotReloadEnabled,    // 默认禁用（安全考虑）
			ConfigCheckInterval: defaultConfigCheckInterval, // 30秒检查间隔
			PolicyUpdateTimeout: defaultPolicyUpdateTimeout, // 5秒更新超时
		},
	}
}

// ============================================================================
//                              🔒 安全检查工具函数
// ============================================================================

// IsImmutableBannedCountry 检查是否为系统级禁用国家
//
// 🔒 **不可绕过检查 (Immutable Security Check)**
//
// 检查指定国家是否在系统级禁用清单中，此类限制用户配置无法覆盖。
//
// 参数：
// - country: ISO-3166-1 alpha-2 国家代码
//
// 返回：
// - bool: true表示是系统级禁用国家
func IsImmutableBannedCountry(country string) bool {
	for _, banned := range IMMUTABLE_BANNED_COUNTRIES {
		if banned == country {
			return true
		}
	}
	return false
}

// IsImmutableBannedOperation 检查是否为系统级禁用操作
//
// 🔒 **不可绕过检查 (Immutable Security Check)**
//
// 检查指定操作是否在系统级禁用清单中，此类限制用户配置无法覆盖。
// 支持通配符匹配（如 "contract.payments.*"）。
//
// 参数：
// - operation: 操作类型字符串
//
// 返回：
// - bool: true表示是系统级禁用操作
func IsImmutableBannedOperation(operation string) bool {
	for _, banned := range IMMUTABLE_BANNED_OPERATIONS {
		if banned == operation {
			return true
		}
		// 支持通配符匹配
		if len(banned) > 1 && banned[len(banned)-1] == '*' {
			prefix := banned[:len(banned)-1]
			if len(operation) >= len(prefix) && operation[:len(prefix)] == prefix {
				return true
			}
		}
	}
	return false
}

// GetComplianceMetadata 获取合规配置元数据
//
// 📋 **配置追踪信息 (Configuration Metadata)**
//
// 返回合规配置的版本、更新时间等元数据，用于审计和维护。
//
// 返回：
// - map[string]interface{}: 包含版本、更新时间等信息的映射
func GetComplianceMetadata() map[string]interface{} {
	return map[string]interface{}{
		"version":           COMPLIANCE_CONFIG_METADATA.Version,
		"last_update_date":  COMPLIANCE_CONFIG_METADATA.LastUpdateDate,
		"sanctions_source":  COMPLIANCE_CONFIG_METADATA.SanctionsSource,
		"updated_by":        COMPLIANCE_CONFIG_METADATA.UpdatedBy,
		"next_review_date":  COMPLIANCE_CONFIG_METADATA.NextReviewDate,
		"banned_countries":  len(IMMUTABLE_BANNED_COUNTRIES),
		"banned_operations": len(IMMUTABLE_BANNED_OPERATIONS),
		"countries_list":    IMMUTABLE_BANNED_COUNTRIES,
		"operations_list":   IMMUTABLE_BANNED_OPERATIONS,
	}
}
