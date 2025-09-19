package env

import (
	"context"

	"github.com/weisyn/v1/pkg/types"
)

// CoordinatorAdapter 协调器适配器
//
// # 核心功能：
// - 将Advisor适配为ExecutionCoordinator期望的EnvAdvisor接口
// - 提供类型转换和接口桥接功能
// - 实现适配器模式，解耦两个组件的接口依赖
// - 支持现有代码的无缝集成和向后兼容
//
// # 设计目标：
// - 接口适配：桥接不同组件间的接口差异
// - 解耦合：避免组件间的直接依赖
// - 易集成：无需修改现有代码即可集成
// - 类型安全：编译时确保接口契约正确性
//
// # 设计模式：
// - 适配器模式：将一个接口转换为另一个接口
// - 组合模式：通过组合实现功能扩展
// - 装饰器模式：在不改变原有逻辑的基础上增加新功能
//
// # 使用场景：
// - ExecutionCoordinator与Advisor的集成
// - 不同版本接口的兼容性处理
// - 测试环境中的Mock和Stub实现
// - 渐进式重构中的过渡方案
type CoordinatorAdapter struct {
	// advisor 底层的环境顾问实例
	// 提供实际的算法逻辑和数据处理能力
	// nil时所有方法返回nil，实现优雅降级
	advisor *Advisor
}

// NewCoordinatorAdapter 创建协调器适配器
//
// 构造函数，创建适配器实例并绑定底层Advisor
//
// 参数：
//   - advisor: 底层环境顾问实例，提供实际的算法能力
//
// 返回值：
//   - *CoordinatorAdapter: 新创建的适配器实例
//
// 功能说明：
//   - 简单包装，保持底层Advisor的原始能力
//   - 支持nil输入，实现优雅降级
//   - 无状态转换，直接传递调用
//
// 使用示例：
//
//	advisor := NewAdvisor(txService, blockService, engineManager)
//	adapter := NewCoordinatorAdapter(advisor)
//	coordinator.SetEnvAdvisor(adapter)
//
// 设计考虑：
//   - 直接引用，避免不必要的拷贝
//   - 简单包装，保持性能和透明度
func NewCoordinatorAdapter(advisor *Advisor) *CoordinatorAdapter {
	return &CoordinatorAdapter{
		advisor: advisor,
	}
}

// AdviseResourceLimits 实现EnvAdvisor接口 - 基于历史数据和ML模型提供资源限制建议
func (ca *CoordinatorAdapter) AdviseResourceLimits(ctx context.Context, contractAddr string, function string) (*CoordinatorResourceAdvice, error) {
	if ca.advisor == nil {
		return nil, nil
	}

	// 调用底层Advisor的AdviseResourceLimits方法
	advice, err := ca.advisor.AdviseResourceLimits(ctx, contractAddr, function)
	if err != nil {
		return nil, err
	}

	if advice == nil {
		return nil, nil
	}

	// 转换为协调器期望的类型
	return &CoordinatorResourceAdvice{
		ExecutionFeeLimit: advice.ExecutionFeeLimit,
		MemoryLimit:       advice.MemoryLimit,
		Concurrency:       advice.Concurrency,
		TimeoutMs:         advice.TimeoutMs,
		Rationale:         advice.Rationale,
	}, nil
}

// PredictExecutionCost 实现EnvAdvisor接口 - 预测执行成本（资源和时间）
func (ca *CoordinatorAdapter) PredictExecutionCost(ctx context.Context, params types.ExecutionParams) (*CoordinatorCostPrediction, error) {
	if ca.advisor == nil {
		return nil, nil
	}

	// 调用底层Advisor的PredictExecutionCost方法
	prediction, err := ca.advisor.PredictExecutionCost(ctx, params)
	if err != nil {
		return nil, err
	}

	if prediction == nil {
		return nil, nil
	}

	// 转换为协调器期望的类型
	return &CoordinatorCostPrediction{
		ExpectedResource: prediction.ExpectedResource,
		ExpectedTimeMs:   prediction.ExpectedTimeMs,
		ConfidencePct:    prediction.ConfidencePct,
		ModelVersion:     prediction.ModelVersion,
	}, nil
}

// AnalyzePerformanceHistory 实现EnvAdvisor接口 - 分析合约的历史性能
func (ca *CoordinatorAdapter) AnalyzePerformanceHistory(ctx context.Context, contractAddr string) (*CoordinatorPerformanceAnalysis, error) {
	if ca.advisor == nil {
		return nil, nil
	}

	// 调用底层Advisor的AnalyzePerformanceHistory方法
	analysis, err := ca.advisor.AnalyzePerformanceHistory(ctx, contractAddr)
	if err != nil {
		return nil, err
	}

	if analysis == nil {
		return nil, nil
	}

	// 转换为协调器期望的类型
	return &CoordinatorPerformanceAnalysis{
		AvgTimeMs:    analysis.AvgTimeMs,
		P95TimeMs:    analysis.P95TimeMs,
		FailureRate:  analysis.FailureRate,
		SampleCount:  analysis.SampleCount,
		LastObserved: analysis.LastObserved,
	}, nil
}

// CoordinatorResourceAdvice 协调器期望的资源建议格式
type CoordinatorResourceAdvice struct {
	ExecutionFeeLimit uint64
	MemoryLimit       uint32
	Concurrency       uint32
	TimeoutMs         int64
	Rationale         string
}

// CoordinatorCostPrediction 协调器期望的成本预测格式
type CoordinatorCostPrediction struct {
	ExpectedResource uint64
	ExpectedTimeMs   uint64
	ConfidencePct    float32
	ModelVersion     string
}

// CoordinatorPerformanceAnalysis 协调器期望的性能分析格式
type CoordinatorPerformanceAnalysis struct {
	AvgTimeMs    uint64
	P95TimeMs    uint64
	FailureRate  float32
	SampleCount  uint64
	LastObserved int64
}
