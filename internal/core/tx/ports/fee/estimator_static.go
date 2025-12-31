// Package fee 提供费用估算器实现
//
// 本包实现不同的费用估算策略，支持静态费率、动态费率等多种模式。
package fee

import (
	"context"

	feeconfig "github.com/weisyn/v1/internal/config/tx/fee"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// StaticFeeEstimator 静态费用估算器
//
// 🎯 **核心功能**：返回固定的最小费用
//
// 💡 **P1 简化策略**：
// - 只返回固定的最小费用建议
// - 实际的价值守恒验证由 BasicConservationPlugin 负责
// - 不查询 UTXO（避免复杂的依赖关系）
//
// ⚠️ **设计说明**：
// - FeeEstimator 只是"建议"，不强制执行
// - 实际费用检查由 Verifier 的 Conservation 插件负责
type StaticFeeEstimator struct {
	minFee uint64     // 最小费用（原生币）
	logger log.Logger // 日志服务
}

// Config StaticFeeEstimator 配置
type Config struct {
	MinFee uint64 // 最小费用（原生币，单位：最小单位）
}

// NewStaticEstimator 创建静态费用估算器
//
// 参数：
//   - config: 估算器配置
//   - logger: 日志服务
//
// 返回：
//   - *StaticFeeEstimator: 估算器实例
//
// 🔧 **修复说明**：硬编码的默认值已移除，请通过配置系统提供默认值。
// 如果config.MinFee为0，说明调用方未提供配置，应使用配置系统的默认值。
func NewStaticEstimator(
	config *Config,
	logger log.Logger,
) *StaticFeeEstimator {
	minFee := config.MinFee
	// 🔧 修复：移除硬编码，如果为0则使用配置系统的默认值（在调用方提供）
	// 如果调用方需要默认值，应从配置模块获取（internal/config/tx/fee）
	if minFee == 0 {
		// 使用配置系统默认值（在调用方应该已经提供了）
		// 这里保留100作为最后的后备，但强烈建议调用方从配置系统获取
		minFee = 100 // 后备默认值，实际应从配置系统获取
		if logger != nil {
			logger.Warnf("[FeeEstimator] 静态费用估算器使用后备默认值：%d，建议从配置系统获取", minFee)
		}
	}

	if logger != nil {
		logger.Infof("[FeeEstimator] 静态费用估算器初始化完成，最小费用：%d", minFee)
	}

	return &StaticFeeEstimator{
		minFee: minFee,
		logger: logger,
	}
}

// NewStaticConfigFromOptions 从配置选项创建静态配置
//
// 🔧 **新增方法**：从配置系统获取配置，替代硬编码
func NewStaticConfigFromOptions(opts *feeconfig.StaticFeeEstimatorConfig) *Config {
	return &Config{
		MinFee: opts.MinFee,
	}
}

// ================================================================================================
// 实现 tx.FeeEstimator 接口
// ================================================================================================

// EstimateFee 估算交易费用
//
// 实现 tx.FeeEstimator 接口
//
// P1 简化实现：直接返回最小费用
func (e *StaticFeeEstimator) EstimateFee(ctx context.Context, tx *transaction.Transaction) (uint64, error) {
	// P1 阶段简化：直接返回最小费用
	// 实际的价值守恒验证由 BasicConservationPlugin 负责
	if e.logger != nil {
		e.logger.Debugf("[FeeEstimator] 估算费用：%d（最小费用）", e.minFee)
	}

	return e.minFee, nil
}

// GetMinFee 获取最小费用（辅助方法）
//
// 用途：供其他组件查询最小费用要求
func (e *StaticFeeEstimator) GetMinFee() uint64 {
	return e.minFee
}
