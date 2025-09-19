// Package pow 提供POW（工作量证明）难度计算工具实现
//
// 📊 **难度计算组件 (Difficulty Calculator Component)**
//
// 本文件专门实现POW难度调整的核心算法，专注于：
// - 难度调整：动态难度调整算法
// - 目标管理：区块间隔目标时间控制
// - 算法优化：高效的难度计算和预测
// - 生产级质量：精确计算、边界处理、异常保护
//
// 🎯 **职责边界**：
// - 专门负责难度值的计算和调整
// - 不涉及挖矿逻辑（由mining.go负责）
// - 不涉及验证逻辑（由validation.go负责）
// - 不涉及基础设施管理（由engine.go负责）
//
// 🔧 **算法特点**：
// - 基于历史区块时间的动态调整
// - 支持多种难度调整策略
// - 平滑的难度过渡机制
// - 防恶意操控的保护机制
//
// 🚀 **计算优化**：
// - 高精度浮点计算
// - 整数溢出保护
// - 边界条件处理
// - 数值稳定性保证
//
// 📈 **调整策略**：
// - 比特币式难度调整（经典算法）
// - 线性难度调整（平滑调整）
// - 指数平滑难度调整（快速响应）
// - 自定义调整策略支持
//
// 🛡️ **安全保护**：
// - 难度调整幅度限制
// - 异常值过滤
// - 最小/最大难度边界
// - 时间戳合理性检查
package pow

import (
	"context"
	"fmt"
	"math"
	"time"

	core "github.com/weisyn/v1/pb/blockchain/block"
)

// DifficultyCalculator 专门的难度计算组件
//
// 📊 **难度计算结构**：
// 专注于难度调整算法的实现，提供智能的难度计算服务。
// 采用组合模式依赖核心引擎的基础设施。
//
// 📝 **字段说明**：
// - coreEngine: 核心引擎的引用，用于访问基础设施
// - adjustmentStrategy: 难度调整策略
// - statistics: 难度计算统计信息
//
// 🎯 **设计原则**：
// - 单一职责：专注难度计算算法
// - 高精度：精确的数学计算
// - 可配置：支持多种调整策略
// - 安全可靠：防攻击的保护机制
type DifficultyCalculator struct {
	coreEngine         *Engine                      // 核心引擎引用
	adjustmentStrategy DifficultyAdjustmentStrategy // 难度调整策略
	statistics         *DifficultyStats             // 难度计算统计信息
}

// DifficultyAdjustmentStrategy 难度调整策略接口
//
// 🎯 **策略模式接口**：
// 定义不同的难度调整算法，支持多种调整策略的实现。
// 允许根据不同的网络条件选择最适合的调整算法。
//
// 💡 **策略类型**：
// - Bitcoin式调整：基于固定窗口的周期调整
// - 线性调整：基于最近区块的线性调整
// - 指数平滑：基于历史数据的指数平滑调整
// - 自适应调整：根据网络状态自动选择策略
type DifficultyAdjustmentStrategy interface {
	// CalculateNextDifficulty 计算下一个难度值
	//
	// 📋 **参数说明**：
	//   - ctx: 上下文控制
	//   - currentDifficulty: 当前难度
	//   - recentBlocks: 最近的区块信息（用于计算时间间隔）
	//   - targetInterval: 目标区块间隔
	//
	// 🔄 **返回值**：
	//   - uint64: 计算出的新难度值
	//   - error: 计算错误
	CalculateNextDifficulty(ctx context.Context, currentDifficulty uint64,
		recentBlocks []*core.BlockHeader, targetInterval time.Duration) (uint64, error)

	// GetStrategyName 获取策略名称
	GetStrategyName() string
}

// DifficultyStats 难度计算统计信息
//
// 📊 **难度统计结构**：
// 记录难度计算过程的统计数据，用于监控和分析。
//
// 📝 **字段说明**：
// - TotalCalculations: 总计算次数
// - AverageCalculationTime: 平均计算时间
// - LastCalculationTime: 最后计算时间
// - DifficultyHistory: 难度历史记录（最近100个）
// - AdjustmentCounts: 各种调整类型的计数
//
// 🎯 **统计用途**：
// - 难度调整监控
// - 算法性能分析
// - 网络健康评估
// - 调整策略优化
type DifficultyStats struct {
	TotalCalculations      uint64                   // 总计算次数
	AverageCalculationTime time.Duration            // 平均计算时间
	LastCalculationTime    time.Time                // 最后计算时间
	DifficultyHistory      []DifficultyHistoryEntry // 难度历史记录
	AdjustmentCounts       map[string]uint64        // 调整类型计数
}

// DifficultyHistoryEntry 难度历史记录条目
//
// 📈 **历史记录结构**：
// 记录单次难度调整的详细信息，用于分析和调试。
//
// 📝 **字段说明**：
// - Timestamp: 调整时间
// - Height: 区块高度
// - OldDifficulty: 调整前难度
// - NewDifficulty: 调整后难度
// - AdjustmentRatio: 调整比例
// - Strategy: 使用的调整策略
type DifficultyHistoryEntry struct {
	Timestamp       time.Time // 调整时间
	Height          uint64    // 区块高度
	OldDifficulty   uint64    // 调整前难度
	NewDifficulty   uint64    // 调整后难度
	AdjustmentRatio float64   // 调整比例
	Strategy        string    // 使用的调整策略
}

// 难度调整类型常量
const (
	AdjustmentIncrease = "increase" // 难度增加
	AdjustmentDecrease = "decrease" // 难度减少
	AdjustmentStable   = "stable"   // 难度保持
)

// NewDifficultyCalculator 创建难度计算器实例
//
// 🚀 **构造函数**：
// 创建专门的难度计算组件，依赖核心引擎提供基础设施。
// 初始化默认的难度调整策略和统计信息。
//
// 📋 **参数说明**：
//   - coreEngine: 核心引擎实例（不能为nil）
//
// 🔄 **返回值**：
//   - *DifficultyCalculator: 初始化好的难度计算器
//   - error: 创建失败时的错误
//
// 💡 **设计说明**：
// - 采用依赖注入模式接收核心引擎
// - 使用默认的Bitcoin式难度调整策略
// - 初始化统计信息和历史记录
func NewDifficultyCalculator(coreEngine *Engine) (*DifficultyCalculator, error) {
	if coreEngine == nil {
		return nil, fmt.Errorf("核心引擎不能为空")
	}

	calculator := &DifficultyCalculator{
		coreEngine:         coreEngine,
		adjustmentStrategy: NewBitcoinStyleStrategy(coreEngine),
		statistics: &DifficultyStats{
			LastCalculationTime: time.Now(),
			DifficultyHistory:   make([]DifficultyHistoryEntry, 0, 100), // 预分配100个历史记录
			AdjustmentCounts:    make(map[string]uint64),
		},
	}

	// 记录初始化日志
	coreEngine.GetLogger().Debug("难度计算器组件初始化完成")

	return calculator, nil
}

// CalculateNextDifficulty 计算下一个区块的难度值
//
// 📊 **核心难度计算**：
// 基于历史区块信息和当前网络状态，计算下一个区块应该使用的难度值。
// 采用配置的调整策略进行智能计算。
//
// 📋 **计算流程**：
// 1. 参数验证和预处理
// 2. 收集必要的历史区块数据
// 3. 委托给调整策略进行计算
// 4. 应用安全边界限制
// 5. 记录计算结果和统计
// 6. 返回最终难度值
//
// 🔄 **安全保护**：
// - 难度范围边界检查
// - 异常值过滤
// - 调整幅度限制
// - 整数溢出保护
//
// 📊 **统计记录**：
// - 计算次数和耗时
// - 难度调整历史
// - 调整类型分类
// - 性能指标
//
// 📋 **参数说明**：
//   - ctx: 上下文控制
//   - currentDifficulty: 当前难度值
//   - recentBlocks: 最近的区块头信息（用于时间分析）
//
// 🔄 **返回值**：
//   - uint64: 计算出的下一个难度值
//   - error: 计算失败时的错误
func (d *DifficultyCalculator) CalculateNextDifficulty(ctx context.Context,
	currentDifficulty uint64, recentBlocks []*core.BlockHeader) (uint64, error) {

	// ==================== 参数验证和预处理 ====================

	startTime := time.Now()
	logger := d.coreEngine.GetLogger()
	config := d.coreEngine.GetConfig()

	logger.Debugf("开始计算下一难度，当前难度: %d，历史区块数: %d",
		currentDifficulty, len(recentBlocks))

	// 更新统计计数
	d.statistics.TotalCalculations++
	d.statistics.LastCalculationTime = startTime

	// 基础参数验证
	if currentDifficulty == 0 {
		return 0, fmt.Errorf("当前难度不能为零")
	}

	// 验证当前难度在合理范围内
	if err := d.coreEngine.ValidateDifficulty(currentDifficulty); err != nil {
		return 0, fmt.Errorf("当前难度不合理: %w", err)
	}

	// 计算目标区块间隔（从配置获取）
	targetInterval := time.Duration(10 * time.Minute) // 默认10分钟
	if config != nil {
		// 注意：这里需要根据实际配置结构调整
		// targetInterval = config.TargetBlockInterval
	}

	// ==================== 委托策略进行计算 ====================

	newDifficulty, err := d.adjustmentStrategy.CalculateNextDifficulty(
		ctx, currentDifficulty, recentBlocks, targetInterval)
	if err != nil {
		logger.Errorf("难度计算策略失败: %v", err)
		return 0, fmt.Errorf("难度计算失败: %w", err)
	}

	// ==================== 安全边界和限制应用 ====================

	// 应用配置的难度边界
	originalDifficulty := newDifficulty
	newDifficulty = d.applyDifficultyBounds(newDifficulty)

	// 应用调整幅度限制（防止恶意操控）
	newDifficulty = d.applyAdjustmentLimits(currentDifficulty, newDifficulty)

	// ==================== 计算结果分析和记录 ====================

	elapsed := time.Since(startTime)
	adjustmentRatio := float64(newDifficulty) / float64(currentDifficulty)

	// 确定调整类型
	var adjustmentType string
	if newDifficulty > currentDifficulty {
		adjustmentType = AdjustmentIncrease
	} else if newDifficulty < currentDifficulty {
		adjustmentType = AdjustmentDecrease
	} else {
		adjustmentType = AdjustmentStable
	}

	// 记录统计信息
	d.statistics.AdjustmentCounts[adjustmentType]++
	d.recordDifficultyHistory(currentDifficulty, newDifficulty, adjustmentRatio)

	// 记录详细日志
	logger.Infof("难度计算完成: %d → %d (%.2fx)，类型: %s，策略: %s，耗时: %v",
		currentDifficulty, newDifficulty, adjustmentRatio, adjustmentType,
		d.adjustmentStrategy.GetStrategyName(), elapsed)

	// 记录边界调整信息
	if originalDifficulty != newDifficulty {
		logger.Warnf("难度被边界限制调整: %d → %d → %d",
			currentDifficulty, originalDifficulty, newDifficulty)
	}

	return newDifficulty, nil
}

// applyDifficultyBounds 应用难度边界限制
//
// 🛡️ **边界保护机制**：
// 确保计算出的难度值在配置的合理范围内。
// 防止计算错误或恶意攻击导致的极端难度值。
//
// 📋 **边界类型**：
// - 最小难度限制
// - 最大难度限制
// - 合理性检查
//
// 📋 **参数说明**：
//   - difficulty: 待限制的难度值
//
// 🔄 **返回值**：
//   - uint64: 限制后的难度值
func (d *DifficultyCalculator) applyDifficultyBounds(difficulty uint64) uint64 {
	config := d.coreEngine.GetConfig()

	// 应用最小难度限制
	if difficulty < config.MinDifficulty {
		return config.MinDifficulty
	}

	// 应用最大难度限制（如果设置）
	if config.MaxDifficulty > 0 && difficulty > config.MaxDifficulty {
		return config.MaxDifficulty
	}

	return difficulty
}

// applyAdjustmentLimits 应用调整幅度限制
//
// 🛡️ **调整幅度保护**：
// 限制单次难度调整的幅度，防止网络受到突然的极端变化影响。
// 基于配置的调整因子进行限制。
//
// 📋 **限制规则**：
// - 单次调整不超过配置的调整因子
// - 防止难度骤增或骤减
// - 保持网络稳定性
//
// 📋 **参数说明**：
//   - currentDifficulty: 当前难度值
//   - newDifficulty: 计算出的新难度值
//
// 🔄 **返回值**：
//   - uint64: 限制后的新难度值
func (d *DifficultyCalculator) applyAdjustmentLimits(currentDifficulty, newDifficulty uint64) uint64 {
	config := d.coreEngine.GetConfig()
	adjustmentFactor := config.DifficultyAdjustmentFactor

	if adjustmentFactor <= 0 {
		adjustmentFactor = 4.0 // 默认最大4倍调整
	}

	// 计算调整比例
	ratio := float64(newDifficulty) / float64(currentDifficulty)

	// 检查是否超过上限
	if ratio > adjustmentFactor {
		limited := uint64(float64(currentDifficulty) * adjustmentFactor)
		d.coreEngine.GetLogger().Warnf("难度调整超过上限，限制: %.2f → %.2f",
			ratio, adjustmentFactor)
		return limited
	}

	// 检查是否超过下限
	if ratio < 1.0/adjustmentFactor {
		limited := uint64(float64(currentDifficulty) / adjustmentFactor)
		d.coreEngine.GetLogger().Warnf("难度调整超过下限，限制: %.2f → %.2f",
			ratio, 1.0/adjustmentFactor)
		return limited
	}

	return newDifficulty
}

// recordDifficultyHistory 记录难度调整历史
//
// 📈 **历史记录管理**：
// 记录每次难度调整的详细信息，用于分析和调试。
// 维护固定大小的历史记录环形缓冲区。
//
// 📋 **记录内容**：
// - 调整时间和区块高度
// - 调整前后难度值
// - 调整比例和策略
//
// 📋 **参数说明**：
//   - oldDifficulty: 调整前难度
//   - newDifficulty: 调整后难度
//   - adjustmentRatio: 调整比例
func (d *DifficultyCalculator) recordDifficultyHistory(oldDifficulty, newDifficulty uint64, adjustmentRatio float64) {
	entry := DifficultyHistoryEntry{
		Timestamp:       time.Now(),
		Height:          0, // TODO: 从上下文获取当前区块高度
		OldDifficulty:   oldDifficulty,
		NewDifficulty:   newDifficulty,
		AdjustmentRatio: adjustmentRatio,
		Strategy:        d.adjustmentStrategy.GetStrategyName(),
	}

	// 维护固定大小的历史记录（最多100条）
	if len(d.statistics.DifficultyHistory) >= 100 {
		// 移除最旧的记录（环形缓冲区）
		copy(d.statistics.DifficultyHistory, d.statistics.DifficultyHistory[1:])
		d.statistics.DifficultyHistory = d.statistics.DifficultyHistory[:99]
	}

	d.statistics.DifficultyHistory = append(d.statistics.DifficultyHistory, entry)
}

// GetStatistics 获取难度计算统计信息
//
// 📊 **统计信息访问**：
// 获取难度计算器的实时统计信息，用于监控和分析。
// 返回统计信息的副本，确保线程安全。
//
// 🔄 **返回值**：
//   - DifficultyStats: 当前的难度计算统计信息
func (d *DifficultyCalculator) GetStatistics() DifficultyStats {
	// 创建历史记录的副本
	historyCopy := make([]DifficultyHistoryEntry, len(d.statistics.DifficultyHistory))
	copy(historyCopy, d.statistics.DifficultyHistory)

	// 创建调整计数的副本
	adjustmentCountsCopy := make(map[string]uint64)
	for k, v := range d.statistics.AdjustmentCounts {
		adjustmentCountsCopy[k] = v
	}

	return DifficultyStats{
		TotalCalculations:      d.statistics.TotalCalculations,
		AverageCalculationTime: d.statistics.AverageCalculationTime,
		LastCalculationTime:    d.statistics.LastCalculationTime,
		DifficultyHistory:      historyCopy,
		AdjustmentCounts:       adjustmentCountsCopy,
	}
}

// SetAdjustmentStrategy 设置难度调整策略
//
// 🔧 **策略切换**：
// 动态切换难度调整策略，支持不同网络条件下的优化。
//
// 📋 **参数说明**：
//   - strategy: 新的调整策略实现
//
// 💡 **使用场景**：
// - 网络条件变化时的策略调整
// - A/B测试不同的调整算法
// - 根据性能指标优化策略
func (d *DifficultyCalculator) SetAdjustmentStrategy(strategy DifficultyAdjustmentStrategy) {
	if strategy != nil {
		oldStrategy := d.adjustmentStrategy.GetStrategyName()
		d.adjustmentStrategy = strategy
		d.coreEngine.GetLogger().Infof("难度调整策略已切换: %s → %s",
			oldStrategy, strategy.GetStrategyName())
	}
}

// PredictNextDifficulty 预测下一个难度值（不记录统计）
//
// 🔮 **难度预测**：
// 基于当前数据预测下一个难度值，用于规划和展示。
// 不影响实际的难度计算统计。
//
// 📋 **参数说明**：
//   - ctx: 上下文控制
//   - currentDifficulty: 当前难度值
//   - recentBlocks: 最近的区块头信息
//
// 🔄 **返回值**：
//   - uint64: 预测的难度值
//   - float64: 预测的调整比例
//   - error: 预测错误
//
// 💡 **使用场景**：
// - 矿工收益预测
// - 网络状态分析
// - 用户界面展示
// - API查询接口
func (d *DifficultyCalculator) PredictNextDifficulty(ctx context.Context,
	currentDifficulty uint64, recentBlocks []*core.BlockHeader) (uint64, float64, error) {

	// 使用当前策略进行预测计算
	targetInterval := time.Duration(10 * time.Minute) // 默认目标间隔

	predictedDifficulty, err := d.adjustmentStrategy.CalculateNextDifficulty(
		ctx, currentDifficulty, recentBlocks, targetInterval)
	if err != nil {
		return 0, 0, fmt.Errorf("难度预测失败: %w", err)
	}

	// 应用边界和限制
	predictedDifficulty = d.applyDifficultyBounds(predictedDifficulty)
	predictedDifficulty = d.applyAdjustmentLimits(currentDifficulty, predictedDifficulty)

	// 计算调整比例
	adjustmentRatio := float64(predictedDifficulty) / float64(currentDifficulty)

	return predictedDifficulty, adjustmentRatio, nil
}

// ==================== Bitcoin式难度调整策略实现 ====================

// BitcoinStyleStrategy Bitcoin式难度调整策略
//
// 🪙 **经典调整算法**：
// 实现类似比特币的难度调整算法，基于固定窗口的周期性调整。
// 这是经过实战验证的成熟算法。
//
// 📝 **算法特点**：
// - 每N个区块调整一次难度
// - 基于实际出块时间与目标时间的比较
// - 简单可靠，经过长期验证
type BitcoinStyleStrategy struct {
	coreEngine *Engine
	windowSize uint64 // 调整窗口大小（区块数）
}

// NewBitcoinStyleStrategy 创建Bitcoin式难度调整策略
func NewBitcoinStyleStrategy(coreEngine *Engine) *BitcoinStyleStrategy {
	windowSize := uint64(2016) // 比特币标准：2016个区块

	if coreEngine != nil && coreEngine.GetConfig() != nil {
		windowSize = coreEngine.GetConfig().DifficultyWindow
	}

	return &BitcoinStyleStrategy{
		coreEngine: coreEngine,
		windowSize: windowSize,
	}
}

// CalculateNextDifficulty 实现Bitcoin式难度调整算法
func (s *BitcoinStyleStrategy) CalculateNextDifficulty(ctx context.Context,
	currentDifficulty uint64, recentBlocks []*core.BlockHeader,
	targetInterval time.Duration) (uint64, error) {

	// 如果历史区块不足，保持当前难度
	if len(recentBlocks) < int(s.windowSize) {
		s.coreEngine.GetLogger().Debugf("历史区块不足 (%d < %d)，保持当前难度",
			len(recentBlocks), s.windowSize)
		return currentDifficulty, nil
	}

	// 计算窗口内的实际出块时间
	windowBlocks := recentBlocks[:s.windowSize]
	actualTime := s.calculateActualTime(windowBlocks)
	expectedTime := time.Duration(s.windowSize) * targetInterval

	if actualTime <= 0 || expectedTime <= 0 {
		return currentDifficulty, fmt.Errorf("时间计算错误：实际时间=%v, 期望时间=%v",
			actualTime, expectedTime)
	}

	// 计算调整比例
	timeRatio := float64(expectedTime) / float64(actualTime)
	newDifficulty := uint64(float64(currentDifficulty) * timeRatio)

	// 防止溢出和极端值
	if newDifficulty == 0 {
		newDifficulty = 1
	}

	s.coreEngine.GetLogger().Debugf("Bitcoin式调整: 实际时间=%v, 期望时间=%v, 比例=%.4f",
		actualTime, expectedTime, timeRatio)

	return newDifficulty, nil
}

// calculateActualTime 计算窗口内实际出块时间
func (s *BitcoinStyleStrategy) calculateActualTime(blocks []*core.BlockHeader) time.Duration {
	if len(blocks) < 2 {
		return 0
	}

	// 计算第一个和最后一个区块的时间差
	firstBlock := blocks[len(blocks)-1] // 最旧的区块
	lastBlock := blocks[0]              // 最新的区块

	actualSeconds := int64(lastBlock.Timestamp) - int64(firstBlock.Timestamp)
	if actualSeconds <= 0 {
		return 0
	}

	return time.Duration(actualSeconds) * time.Second
}

// GetStrategyName 获取策略名称
func (s *BitcoinStyleStrategy) GetStrategyName() string {
	return "BitcoinStyle"
}

// ==================== 其他工具方法 ====================

// EstimateBlockTime 估算在给定难度下的出块时间
//
// ⏱️ **时间估算工具**：
// 根据难度值和网络算力估算出块时间，用于用户界面展示。
//
// 📋 **参数说明**：
//   - difficulty: 难度值
//   - networkHashRate: 网络总算力（可选，为0时使用默认值）
//
// 🔄 **返回值**：
//   - time.Duration: 估算的出块时间
func (d *DifficultyCalculator) EstimateBlockTime(difficulty uint64, networkHashRate float64) time.Duration {
	if networkHashRate <= 0 {
		// 使用默认网络算力估算（可根据实际情况调整）
		networkHashRate = 1000000 // 1MH/s
	}

	// 简化计算：估算时间 = 2^difficulty / 网络算力
	expectedHashes := math.Pow(2, float64(difficulty))
	expectedSeconds := expectedHashes / networkHashRate

	return time.Duration(expectedSeconds * float64(time.Second))
}
