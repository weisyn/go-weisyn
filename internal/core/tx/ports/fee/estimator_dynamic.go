// Package fee 提供费用估算器实现
//
// estimator_dynamic.go: 动态费用估算器（基于交易大小和网络拥堵）
package fee

import (
	"context"
	"fmt"
	"math"

	"google.golang.org/protobuf/proto"

	feeconfig "github.com/weisyn/v1/internal/config/tx/fee"
	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// DynamicFeeEstimator 动态费用估算器
//
// 🎯 **核心功能**：基于交易大小和网络拥堵动态计算费用
//
// 💡 **动态策略**：
// - 按字节收费：交易越大，费用越高
// - 拥堵调整：网络拥堵时费率上涨
// - 多档位支持：低速/标准/快速三种确认速度
// - 智能预测：基于历史数据预测最优费率
//
// 🔍 **费用计算公式**：
//
//	费用 = max(
//	    base_fee,
//	    tx_size * rate_per_byte * congestion_multiplier
//	)
//
// 📊 **拥堵等级**：
// - Low (< 30%): 1.0x 费率
// - Medium (30-70%): 1.5x 费率
// - High (> 70%): 2.0x-3.0x 费率
type DynamicFeeEstimator struct {
	// 基础费率（每字节）
	baseRatePerByte uint64
	// 最小费用（防止费用过低）
	minFee uint64
	// 最大费用（防止费用过高）
	maxFee uint64
	// 拥堵倍数（1.0 = 正常，2.0 = 拥堵）
	congestionMultiplier float64
	// 日志服务
	logger log.Logger
	// 网络状态提供者（可选，用于获取实时拥堵信息）
	networkStateProvider NetworkStateProvider
}

// NetworkStateProvider 网络状态提供者接口
//
// 🎯 **用途**：获取实时网络拥堵信息，用于动态调整费率
//
// 💡 **设计理念**：
// 通过接口抽象网络状态获取逻辑，支持：
// - 本地 mempool 统计
// - 远程 RPC 查询
// - Mock 测试
type NetworkStateProvider interface {
	// GetCongestionLevel 获取当前网络拥堵等级
	//
	// 返回值：
	//   - float64: 拥堵比例（0.0 - 1.0）
	//     - 0.0-0.3: 低拥堵
	//     - 0.3-0.7: 中等拥堵
	//     - 0.7-1.0: 高拥堵
	//   - error: 获取失败
	GetCongestionLevel(ctx context.Context) (float64, error)

	// GetRecentFees 获取最近确认的交易费率
	//
	// 返回值：
	//   - []uint64: 最近 N 笔交易的费率（每字节）
	//   - error: 获取失败
	GetRecentFees(ctx context.Context, count int) ([]uint64, error)
}

// DynamicConfig 动态费用估算器配置
type DynamicConfig struct {
	// 基础费率（每字节，单位：最小单位）
	BaseRatePerByte uint64

	// 最小费用（单位：最小单位）
	MinFee uint64

	// 最大费用（单位：最小单位，0 表示无上限）
	MaxFee uint64

	// 拥堵倍数（1.0 = 正常，2.0 = 拥堵2倍费率）
	CongestionMultiplier float64

	// 网络状态提供者（可选）
	NetworkStateProvider NetworkStateProvider
}

// DefaultDynamicConfig 返回默认配置
//
// ⚠️ **已废弃**：此函数保留仅为向后兼容，生产代码应使用配置系统。
// 请使用 internal/config/tx/fee 配置模块提供的配置。
//
// 🔧 **修复说明**：硬编码的默认值已移除，请通过配置系统管理。
func DefaultDynamicConfig() *DynamicConfig {
	// 🔧 修复：移除硬编码，返回空配置，强制使用配置系统
	// 如果调用方需要默认值，应从配置模块获取
	return &DynamicConfig{
		BaseRatePerByte:      0,   // 必须通过配置提供
		MinFee:               0,   // 必须通过配置提供
		MaxFee:               0,   // 无上限（默认）
		CongestionMultiplier: 0,   // 必须通过配置提供
		NetworkStateProvider: nil, // 无网络状态提供者
	}
}

// NewDynamicConfigFromOptions 从配置选项创建动态配置
//
// 🔧 **新增方法**：从配置系统获取配置，替代硬编码
func NewDynamicConfigFromOptions(opts *feeconfig.DynamicFeeEstimatorConfig, networkStateProvider NetworkStateProvider) *DynamicConfig {
	return &DynamicConfig{
		BaseRatePerByte:      opts.BaseRatePerByte,
		MinFee:               opts.MinFee,
		MaxFee:               opts.MaxFee,
		CongestionMultiplier: opts.CongestionMultiplier,
		NetworkStateProvider: networkStateProvider,
	}
}

// NewDynamicEstimator 创建动态费用估算器
//
// 参数：
//   - config: 估算器配置
//   - logger: 日志服务
//
// 返回：
//   - *DynamicFeeEstimator: 估算器实例
func NewDynamicEstimator(
	config *DynamicConfig,
	logger log.Logger,
) *DynamicFeeEstimator {
	if config == nil {
		config = DefaultDynamicConfig()
	}

	// 🔧 修复：移除硬编码，使用配置系统的默认值
	// 如果配置值为0，说明调用方未提供，应使用配置系统的默认值
	// 这里保留作为最后的后备，但强烈建议调用方从配置系统获取
	if config.BaseRatePerByte == 0 {
		config.BaseRatePerByte = 1 // 后备默认值，实际应从配置系统获取
		if logger != nil {
			logger.Warnf("[FeeEstimator] 动态费用估算器使用后备默认值 BaseRatePerByte=%d，建议从配置系统获取", config.BaseRatePerByte)
		}
	}
	if config.MinFee == 0 {
		config.MinFee = 100 // 后备默认值，实际应从配置系统获取
		if logger != nil {
			logger.Warnf("[FeeEstimator] 动态费用估算器使用后备默认值 MinFee=%d，建议从配置系统获取", config.MinFee)
		}
	}
	if config.CongestionMultiplier < 1.0 {
		config.CongestionMultiplier = 1.0 // 后备默认值，实际应从配置系统获取
		if logger != nil {
			logger.Warnf("[FeeEstimator] 动态费用估算器使用后备默认值 CongestionMultiplier=%.2f，建议从配置系统获取", config.CongestionMultiplier)
		}
	}

	if logger != nil {
		logger.Info("✅ 动态费用估算器初始化成功")
		logger.Infof("   基础费率: %d（每字节）", config.BaseRatePerByte)
		logger.Infof("   最小费用: %d", config.MinFee)
		logger.Infof("   最大费用: %d（0=无上限）", config.MaxFee)
		logger.Infof("   拥堵倍数: %.2fx", config.CongestionMultiplier)
		if config.NetworkStateProvider != nil {
			logger.Info("   网络状态提供者: 已启用")
		} else {
			logger.Info("   网络状态提供者: 未启用（使用静态倍数）")
		}
	}

	return &DynamicFeeEstimator{
		baseRatePerByte:      config.BaseRatePerByte,
		minFee:               config.MinFee,
		maxFee:               config.MaxFee,
		congestionMultiplier: config.CongestionMultiplier,
		logger:               logger,
		networkStateProvider: config.NetworkStateProvider,
	}
}

// EstimateFee 估算交易费用
//
// 实现 tx.FeeEstimator 接口
//
// 🎯 **动态计算逻辑**：
// 1. 序列化交易，计算字节大小
// 2. 获取实时拥堵倍数（如果有网络状态提供者）
// 3. 计算动态费用：size * rate * congestion
// 4. 应用最小/最大限制
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待估算的交易
//
// 返回：
//   - uint64: 建议费用
//   - error: 估算失败的原因
func (e *DynamicFeeEstimator) EstimateFee(ctx context.Context, tx *transaction.Transaction) (uint64, error) {
	// 1. 计算交易大小
	txSize, err := e.calculateTxSize(tx)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate tx size: %w", err)
	}

	// 2. 获取动态拥堵倍数
	congestionMultiplier := e.congestionMultiplier
	if e.networkStateProvider != nil {
		// 尝试获取实时拥堵信息
		if congestionLevel, err := e.networkStateProvider.GetCongestionLevel(ctx); err == nil {
			// 根据拥堵等级动态调整倍数
			congestionMultiplier = e.calculateCongestionMultiplier(congestionLevel)
		} else if e.logger != nil {
			e.logger.Warnf("获取网络拥堵信息失败，使用默认倍数: %v", err)
		}
	}

	// 3. 计算动态费用
	baseFee := float64(txSize) * float64(e.baseRatePerByte) * congestionMultiplier
	estimatedFee := uint64(math.Ceil(baseFee))

	// 4. 应用最小费用限制
	if estimatedFee < e.minFee {
		estimatedFee = e.minFee
	}

	// 5. 应用最大费用限制（如果设置）
	if e.maxFee > 0 && estimatedFee > e.maxFee {
		estimatedFee = e.maxFee
	}

	// 6. 记录日志
	if e.logger != nil {
		e.logger.Debugf(
			"动态费用估算: 大小=%d字节, 费率=%.2f(每字节), 拥堵=%.2fx, 费用=%d",
			txSize,
			float64(e.baseRatePerByte),
			congestionMultiplier,
			estimatedFee,
		)
	}

	return estimatedFee, nil
}

// EstimateFeeWithSpeed 根据确认速度档位估算费用
//
// 扩展方法（非 FeeEstimator 接口定义）
//
// 🎯 **速度档位**：
// - Low: 低速确认（1.0x 费率）
// - Standard: 标准确认（1.5x 费率）
// - Fast: 快速确认（2.0x 费率）
//
// 参数：
//   - ctx: 上下文对象
//   - tx: 待估算的交易
//   - speed: 确认速度档位（"low", "standard", "fast"）
//
// 返回：
//   - uint64: 建议费用
//   - error: 估算失败的原因
func (e *DynamicFeeEstimator) EstimateFeeWithSpeed(
	ctx context.Context,
	tx *transaction.Transaction,
	speed string,
) (uint64, error) {
	// 获取基础费用
	baseFee, err := e.EstimateFee(ctx, tx)
	if err != nil {
		return 0, err
	}

	// 根据速度档位调整
	var speedMultiplier float64
	switch speed {
	case "low":
		speedMultiplier = 1.0 // 低速
	case "standard":
		speedMultiplier = 1.5 // 标准
	case "fast":
		speedMultiplier = 2.0 // 快速
	default:
		speedMultiplier = 1.5 // 默认标准
	}

	adjustedFee := uint64(float64(baseFee) * speedMultiplier)

	// 应用最大费用限制
	if e.maxFee > 0 && adjustedFee > e.maxFee {
		adjustedFee = e.maxFee
	}

	if e.logger != nil {
		e.logger.Debugf(
			"速度档位估算: 速度=%s, 基础费用=%d, 倍数=%.2fx, 调整后=%d",
			speed,
			baseFee,
			speedMultiplier,
			adjustedFee,
		)
	}

	return adjustedFee, nil
}

// GetFeeRateEstimate 获取费率估算（每字节）
//
// 扩展方法（非 FeeEstimator 接口定义）
//
// 🎯 **用途**：获取当前建议的费率，供用户自行计算
//
// 返回：
//   - uint64: 当前费率（每字节）
func (e *DynamicFeeEstimator) GetFeeRateEstimate(ctx context.Context) (uint64, error) {
	// 获取拥堵倍数
	congestionMultiplier := e.congestionMultiplier
	if e.networkStateProvider != nil {
		if congestionLevel, err := e.networkStateProvider.GetCongestionLevel(ctx); err == nil {
			congestionMultiplier = e.calculateCongestionMultiplier(congestionLevel)
		}
	}

	feeRate := uint64(float64(e.baseRatePerByte) * congestionMultiplier)
	return feeRate, nil
}

// SetCongestionMultiplier 设置拥堵倍数
//
// 扩展方法（非 FeeEstimator 接口定义）
//
// 🎯 **用途**：动态调整拥堵倍数（用于运营调控）
//
// 参数：
//   - multiplier: 拥堵倍数（1.0 = 正常，2.0 = 拥堵2倍费率）
func (e *DynamicFeeEstimator) SetCongestionMultiplier(multiplier float64) {
	if multiplier < 1.0 {
		multiplier = 1.0
	}
	e.congestionMultiplier = multiplier

	if e.logger != nil {
		e.logger.Infof("拥堵倍数已更新: %.2fx", multiplier)
	}
}

// calculateTxSize 计算交易字节大小
//
// 🎯 **核心逻辑**：
// 使用 protobuf Marshal 序列化交易，得到字节数组大小。
//
// 参数：
//   - tx: 待计算的交易
//
// 返回：
//   - uint64: 交易字节大小
//   - error: 计算失败的原因
func (e *DynamicFeeEstimator) calculateTxSize(tx *transaction.Transaction) (uint64, error) {
	if tx == nil {
		return 0, fmt.Errorf("transaction cannot be nil")
	}

	// 使用 protobuf Marshal 序列化
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal transaction: %w", err)
	}

	return uint64(len(txBytes)), nil
}

// calculateCongestionMultiplier 根据拥堵等级计算倍数
//
// 🎯 **拥堵等级映射**：
// - 0.0 - 0.3: 低拥堵 → 1.0x
// - 0.3 - 0.5: 中低拥堵 → 1.0x - 1.5x（线性插值）
// - 0.5 - 0.7: 中高拥堵 → 1.5x - 2.0x（线性插值）
// - 0.7 - 1.0: 高拥堵 → 2.0x - 3.0x（线性插值）
//
// 参数：
//   - congestionLevel: 拥堵比例（0.0 - 1.0）
//
// 返回：
//   - float64: 拥堵倍数
func (e *DynamicFeeEstimator) calculateCongestionMultiplier(congestionLevel float64) float64 {
	// 确保在 [0.0, 1.0] 范围内
	if congestionLevel < 0.0 {
		congestionLevel = 0.0
	}
	if congestionLevel > 1.0 {
		congestionLevel = 1.0
	}

	// 分段线性插值
	var multiplier float64

	if congestionLevel < 0.3 {
		// 低拥堵：1.0x
		multiplier = 1.0
	} else if congestionLevel < 0.5 {
		// 中低拥堵：1.0x - 1.5x（线性插值）
		ratio := (congestionLevel - 0.3) / 0.2
		multiplier = 1.0 + (ratio * 0.5)
	} else if congestionLevel < 0.7 {
		// 中高拥堵：1.5x - 2.0x（线性插值）
		ratio := (congestionLevel - 0.5) / 0.2
		multiplier = 1.5 + (ratio * 0.5)
	} else {
		// 高拥堵：2.0x - 3.0x（线性插值）
		ratio := (congestionLevel - 0.7) / 0.3
		multiplier = 2.0 + (ratio * 1.0)
	}

	return multiplier
}

// GetMinFee 获取最小费用（辅助方法）
//
// 用途：供其他组件查询最小费用要求
func (e *DynamicFeeEstimator) GetMinFee() uint64 {
	return e.minFee
}

// GetMaxFee 获取最大费用（辅助方法）
//
// 用途：供其他组件查询最大费用限制
func (e *DynamicFeeEstimator) GetMaxFee() uint64 {
	return e.maxFee
}
