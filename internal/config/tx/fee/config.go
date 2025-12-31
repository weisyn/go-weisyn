package fee

// FeeEstimatorOptions 费用估算器配置选项
//
// 🎯 **配置职责**：管理交易费用估算相关的所有配置
//
// 📋 **估算器类型**：
// - static: 静态费用估算器（固定费率）
// - dynamic: 动态费用估算器（根据网络状态调整）
type FeeEstimatorOptions struct {
	// 估算器类型（static, dynamic）
	Type string `json:"type"`

	// 静态费用估算器配置
	Static StaticFeeEstimatorConfig `json:"static"`

	// 动态费用估算器配置
	Dynamic DynamicFeeEstimatorConfig `json:"dynamic"`
}

// StaticFeeEstimatorConfig 静态费用估算器配置
type StaticFeeEstimatorConfig struct {
	// 最小费用（最小单位）
	MinFee uint64 `json:"min_fee"`
}

// DynamicFeeEstimatorConfig 动态费用估算器配置
type DynamicFeeEstimatorConfig struct {
	// 基础费率（每字节的最小单位数）
	BaseRatePerByte uint64 `json:"base_rate_per_byte"`

	// 最小费用（最小单位）
	MinFee uint64 `json:"min_fee"`

	// 最大费用（最小单位，0表示无上限）
	MaxFee uint64 `json:"max_fee"`

	// 拥堵倍数（1.0 = 正常，2.0 = 拥堵2倍费率）
	CongestionMultiplier float64 `json:"congestion_multiplier"`
}

// UserFeeEstimatorConfig 用户费用估算器配置（从configs/*/config.json加载）
//
// 📋 **配置来源**：用户配置文件（可选，通常不暴露给用户）
type UserFeeEstimatorConfig struct {
	// 估算器类型（static, dynamic）
	Type string `json:"type"`

	// 静态费用估算器配置
	Static *StaticFeeEstimatorConfig `json:"static,omitempty"`

	// 动态费用估算器配置
	Dynamic *DynamicFeeEstimatorConfig `json:"dynamic,omitempty"`
}

// New 创建费用估算器配置选项
//
// 参数：
//   - userConfig: 用户配置（从configs/*/config.json加载，可为nil）
//
// 返回：
//   - *FeeEstimatorOptions: 费用估算器配置选项
func New(userConfig *UserFeeEstimatorConfig) *FeeEstimatorOptions {
	opts := &FeeEstimatorOptions{
		Type:    getDefaultEstimatorType(),
		Static:  getDefaultStaticConfig(),
		Dynamic: getDefaultDynamicConfig(),
	}

	// 应用用户配置
	if userConfig != nil {
		applyUserConfig(opts, userConfig)
	}

	return opts
}

// applyUserConfig 应用用户配置
func applyUserConfig(opts *FeeEstimatorOptions, userConfig *UserFeeEstimatorConfig) {
	// 应用估算器类型
	if userConfig.Type != "" {
		opts.Type = userConfig.Type
	}

	// 应用静态费用估算器配置
	if userConfig.Static != nil {
		if userConfig.Static.MinFee > 0 {
			opts.Static.MinFee = userConfig.Static.MinFee
		}
	}

	// 应用动态费用估算器配置
	if userConfig.Dynamic != nil {
		if userConfig.Dynamic.BaseRatePerByte > 0 {
			opts.Dynamic.BaseRatePerByte = userConfig.Dynamic.BaseRatePerByte
		}
		if userConfig.Dynamic.MinFee > 0 {
			opts.Dynamic.MinFee = userConfig.Dynamic.MinFee
		}
		if userConfig.Dynamic.MaxFee > 0 {
			opts.Dynamic.MaxFee = userConfig.Dynamic.MaxFee
		}
		if userConfig.Dynamic.CongestionMultiplier > 0 {
			opts.Dynamic.CongestionMultiplier = userConfig.Dynamic.CongestionMultiplier
		}
	}
}

// GetStaticConfig 获取静态费用估算器配置
func (o *FeeEstimatorOptions) GetStaticConfig() *StaticFeeEstimatorConfig {
	return &o.Static
}

// GetDynamicConfig 获取动态费用估算器配置
func (o *FeeEstimatorOptions) GetDynamicConfig() *DynamicFeeEstimatorConfig {
	return &o.Dynamic
}

