package fee

// getDefaultEstimatorType 获取默认估算器类型
//
// 🎯 **默认值策略**：
// - 默认使用静态费用估算器（static），简单可靠
// - 动态费用估算器需要在配置中显式启用
func getDefaultEstimatorType() string {
	return "static"
}

// getDefaultStaticConfig 获取默认静态费用估算器配置
func getDefaultStaticConfig() StaticFeeEstimatorConfig {
	return StaticFeeEstimatorConfig{
		MinFee: 100, // 默认最小费用：100（最小单位）
	}
}

// getDefaultDynamicConfig 获取默认动态费用估算器配置
//
// 🎯 **默认值策略**：
// - 基础费率：每字节1个最小单位
// - 最小费用：100个最小单位
// - 最大费用：0（无上限）
// - 拥堵倍数：1.0（正常费率）
func getDefaultDynamicConfig() DynamicFeeEstimatorConfig {
	return DynamicFeeEstimatorConfig{
		BaseRatePerByte:      1,   // 每字节 1 个最小单位
		MinFee:               100, // 最小 100
		MaxFee:               0,   // 无上限
		CongestionMultiplier: 1.0, // 正常费率
	}
}

