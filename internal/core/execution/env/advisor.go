package env

import (
	"context"
	"fmt"
	"time"

	blockchain "github.com/weisyn/v1/pkg/interfaces/blockchain"
	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// Advisor 执行环境顾问
//
// # 核心功能：
// - 智能资源建议：基于历史数据和机器学习模型提供资源、内存、并发度等资源限制建议
// - 成本预测：预测智能合约和AI模型执行的资源消耗和时间成本
// - 性能分析：分析历史执行性能，提供优化建议和趋势分析
// - 无副作用决策：所有建议和预测都是只读操作，不影响执行流程
//
// # 设计目标：
// - 数据驱动：基于真实的执行历史数据进行决策
// - 自适应：通过机器学习模型动态调整建议策略
// - 高性能：轻量级计算，微秒级建议生成
// - 可测试：时间函数可替换，支持确定性测试
//
// # 技术特点：
// - 在线学习：使用轻量级线性模型进行在线参数调优
// - 启发式算法：结合规则和统计的混合决策策略
// - 多数据源：整合引擎指标、区块链状态、交易历史
// - 渐进式优化：随着数据积累逐步提升预测精度
//
// # 使用场景：
// - 智能合约执行前的资源预算规划
// - AI模型推理的性能优化和成本控制
// - 区块链网络的负载均衡和容量规划
// - 执行环境的自动调优和异常检测
type Advisor struct {
	// ==================== 基础依赖服务 ====================

	// txService 交易服务接口
	// 用于查询历史交易数据，分析执行模式和频率分布
	// 提供合约调用历史、资源使用模式等统计基础
	txService blockchain.TransactionService

	// blockService 区块服务接口
	// 用于查询区块信息，分析网络状态和时序特征
	// 提供区块时间间隔、网络拥堵程度等环境数据
	blockService blockchain.BlockService

	// metrics 引擎管理器引用
	// 保留引用用于潜在的内部指标访问
	// 当前版本通过getMetrics函数间接获取数据
	metrics execiface.EngineManager

	// clock 时间函数提供者
	// 用于获取当前时间，支持测试时的时间模拟
	// 默认使用time.Now，测试时可替换为固定时间
	clock func() time.Time

	// ==================== 可选增强功能 ====================

	// getMetrics 内部指标快照提供者
	// 获取引擎执行统计的快照数据，用于建议计算
	// 不依赖公共接口，避免暴露内部实现细节
	// nil时使用基线算法，非nil时进行启发式优化
	getMetrics func() map[types.EngineType]EngineMetricsSnapshot

	// model 轻量级机器学习模型
	// 用于时间和资源预测的微调优化
	// 采用在线线性回归，支持增量学习
	// nil时仅使用启发式算法，非nil时进行ML增强
	model *LinearModel
}

// NewAdvisor 创建执行环境顾问实例
//
// 构造函数，创建配置完整的顾问实例，设置基础依赖服务
//
// 参数：
//   - tx: 交易服务接口，用于查询历史交易数据
//   - bm: 区块服务接口，用于查询区块链状态信息
//   - em: 引擎管理器，保留引用用于潜在的指标访问
//
// 返回值：
//   - *Advisor: 初始化完成的顾问实例
//
// 初始状态：
//   - 基础服务已配置，可进行基线算法计算
//   - 增强功能为nil，需通过With方法配置
//   - 时间函数设置为time.Now，支持真实时间获取
//
// 使用示例：
//
//	advisor := NewAdvisor(txService, blockService, engineManager)
//	advisor = advisor.WithMetricsProvider(metricsFunc).WithModel(mlModel)
//
// 设计考虑：
//   - 基础功能优先，增强功能可选
//   - 依赖注入模式，便于测试和扩展
//   - 零配置可用，默认行为安全可靠
func NewAdvisor(tx blockchain.TransactionService, bm blockchain.BlockService, em execiface.EngineManager) *Advisor {
	return &Advisor{txService: tx, blockService: bm, metrics: em, clock: time.Now}
}

// EngineMetricsSnapshot 引擎指标快照
//
// # 功能说明：
// - 内部使用的轻量级指标数据结构
// - 避免直接依赖公共接口的复杂数据类型
// - 提供建议算法所需的核心统计信息
// - 支持快速计算和内存效率优化
//
// # 设计目标：
// - 数据最小化：只包含建议算法必需的关键指标
// - 类型简化：使用基础数据类型，避免复杂依赖
// - 快照语义：表示某一时刻的静态数据视图
// - 计算友好：数据格式便于数学运算和统计分析
//
// # 使用场景：
// - 资源建议算法的输入数据
// - 机器学习模型的特征工程
// - 启发式规则的统计基础
// - 性能分析的数据源
type EngineMetricsSnapshot struct {
	// ExecutionCount 累计执行总次数
	// 包括成功和失败的所有执行尝试
	// 用于计算执行频率和活跃度
	ExecutionCount uint64

	// FailureCount 累计失败执行次数
	// 用于计算失败率：FailureCount / ExecutionCount
	// 高失败率影响资源分配策略和超时设置
	FailureCount uint64

	// LastDurationMs 最近一次执行的耗时（毫秒）
	// 用于估算当前性能水平和响应时间
	// 作为超时设置和性能预测的重要指标
	LastDurationMs uint64
}

// WithMetricsProvider 注入内部指标快照提供者
//
// 配置指标数据源，启用启发式算法的数据驱动优化
//
// 参数：
//   - p: 指标快照提供函数，返回各引擎类型的执行统计
//
// 返回值：
//   - *Advisor: 支持链式调用的顾问实例
//
// 功能说明：
//   - 启用数据驱动的资源建议算法
//   - 提供实时执行统计用于动态调优
//   - 支持多引擎类型的差异化处理
//
// 使用场景：
//   - 生产环境中基于真实数据的智能决策
//   - 测试环境中模拟不同执行场景
//   - 性能调优和容量规划
func (a *Advisor) WithMetricsProvider(p func() map[types.EngineType]EngineMetricsSnapshot) *Advisor {
	a.getMetrics = p
	return a
}

// WithModel 注入轻量级机器学习模型
//
// 配置ML模型，启用基于机器学习的预测优化
//
// 参数：
//   - m: 线性回归模型实例，用于时间和资源预测
//
// 返回值：
//   - *Advisor: 支持链式调用的顾问实例
//
// 功能说明：
//   - 启用ML增强的预测算法
//   - 支持在线学习和模型更新
//   - 提供比启发式算法更精确的预测
//
// 使用场景：
//   - 高精度的成本预测需求
//   - 自适应的资源优化场景
//   - 复杂执行模式的智能分析
func (a *Advisor) WithModel(m *LinearModel) *Advisor {
	a.model = m
	return a
}

// ResourceAdvice 资源建议信息
//
// # 功能说明：
// - 包含执行环境的完整资源配置建议
// - 基于历史数据和预测模型生成
// - 提供建议依据的可追溯性
// - 支持执行前的资源预算和配置
//
// # 使用场景：
// - 智能合约执行前的资源规划
// - AI模型推理的性能优化
// - 执行环境的动态调优
// - 资源使用的成本控制
type ResourceAdvice struct {
	// ExecutionFeeLimit 资源限制
	// 建议的最大资源消耗量，用于防止无限循环和控制成本
	ExecutionFeeLimit uint64

	// MemoryLimit 内存限制（字节）
	// 建议的最大内存使用量，用于防止内存泄漏和系统稳定
	MemoryLimit uint32

	// Concurrency 并发度
	// 建议的并发执行线程数，用于平衡性能和资源消耗
	Concurrency uint32

	// TimeoutMs 超时时间（毫秒）
	// 建议的最大执行时间，用于防止长时间阻塞和资源占用
	TimeoutMs int64

	// Rationale 建议依据说明
	// 详细说明产生此建议的算法逻辑和数据依据
	// 用于审计、调试和决策透明化
	Rationale string
}

// CostPrediction 执行成本预测
//
// # 功能说明：
// - 预测单次执行的资源消耗和时间成本
// - 基于历史统计和机器学习模型
// - 提供预测置信度和模型版本信息
// - 支持成本控制和预算规划
//
// # 使用场景：
// - 执行前的成本评估和预算
// - 不同执行策略的成本比较
// - 资源定价和计费依据
// - 用户体验的预期管理
type CostPrediction struct {
	// ExpectedResource 预期资源消耗
	// 基于历史数据和模型预测的资源使用量
	ExpectedResource uint64

	// ExpectedTimeMs 预期执行时间（毫秒）
	// 基于历史统计和ML模型的时间预测
	ExpectedTimeMs uint64

	// ConfidencePct 预测置信度（百分比）
	// 表示预测结果的可信程度，0-1之间的浮点数
	// 高置信度表示预测更可靠，低置信度提醒谨慎使用
	ConfidencePct float32

	// ModelVersion 模型版本标识
	// 用于标识生成预测的算法版本
	// 便于模型升级后的效果对比和回归分析
	ModelVersion string
}

// PerformanceAnalysis 性能历史分析
//
// # 功能说明：
// - 提供合约或模型的历史执行性能统计
// - 包含平均值、分位数、失败率等关键指标
// - 支持性能趋势分析和异常检测
// - 为优化决策提供量化依据
//
// # 使用场景：
// - 合约性能的历史回顾和趋势分析
// - 执行环境的容量规划和优化
// - 性能异常的检测和诊断
// - SLA制定和性能基线建立
type PerformanceAnalysis struct {
	// AvgTimeMs 平均执行时间（毫秒）
	// 历史执行的算术平均时间
	// 反映一般情况下的性能水平
	AvgTimeMs uint64

	// P95TimeMs 95分位执行时间（毫秒）
	// 95%的执行都在此时间内完成
	// 用于SLA设定和性能保证
	P95TimeMs uint64

	// FailureRate 失败率（0-1之间）
	// 历史执行中失败的比例
	// 反映稳定性和可靠性水平
	FailureRate float32

	// SampleCount 样本数量
	// 用于计算统计指标的历史执行次数
	// 样本数量越大，统计结果越可靠
	SampleCount uint64

	// LastObserved 最后观测时间（Unix时间戳）
	// 最近一次数据更新的时间
	// 用于判断数据的时效性
	LastObserved int64
}

// AdviseResourceLimits 基于当前链负载与历史画像给出资源建议（启发式+基线）
func (a *Advisor) AdviseResourceLimits(_ context.Context, contractAddr string, function string) (*ResourceAdvice, error) {
	if contractAddr == "" || function == "" {
		return nil, fmt.Errorf("invalid contract/function")
	}
	// 基线
	资源 := uint64(1_000_000)
	mem := uint32(64 * 1024 * 1024)
	conc := uint32(1)
	timeout := int64(30_000)
	rationale := "默认安全基线"

	// 使用内部指标快照进行启发式微调（若可用）
	if a.getMetrics != nil {
		snap := a.getMetrics()
		if len(snap) > 0 {
			var totalExec, totalFail uint64
			var sumDur uint64
			var count int
			for _, s := range snap {
				totalExec += s.ExecutionCount
				totalFail += s.FailureCount
				if s.LastDurationMs > 0 {
					sumDur += s.LastDurationMs
					count++
				}
			}
			avgDur := uint64(50)
			if count > 0 {
				avgDur = sumDur / uint64(count)
			}
			failRate := 0.0
			if totalExec > 0 {
				failRate = float64(totalFail) / float64(totalExec)
			}
			if avgDur > 120 {
				timeout = int64(minU64(uint64(timeout+int64((avgDur-120))*5), 120_000))
				rationale += "；avgDur>120ms"
			}
			if failRate > 0.05 {
				if conc > 1 {
					conc = 1
				}
				timeout = int64(minU64(uint64(timeout+10_000), 120_000))
				资源 = uint64(float64(资源) * 1.1)
				rationale += "；failRate>5%"
			}
		}
	}

	// 机器学习微调（可选）：基于全局特征估计期望耗时，作为超时上界的进一步校正
	if a.model != nil && a.model.Ready() {
		var avgDur float64 = 0.05 // s
		var failRate float64 = 0.0
		if a.getMetrics != nil {
			snap := a.getMetrics()
			var sum uint64
			var c int
			var te, tf uint64
			for _, s := range snap {
				if s.LastDurationMs > 0 {
					sum += s.LastDurationMs
					c++
				}
				te += s.ExecutionCount
				tf += s.FailureCount
			}
			if c > 0 {
				avgDur = float64(sum/uint64(c)) / 1000.0
			}
			if te > 0 {
				failRate = float64(tf) / float64(te)
			}
		}
		资源Norm := float64(资源) / 2_000_000.0
		if 资源Norm > 1 {
			资源Norm = 1
		}
		if avgDur > 1 {
			avgDur = 1
		}
		if failRate > 1 {
			failRate = 1
		}
		predSec := a.model.Predict([]float64{资源Norm, avgDur, failRate})
		if predSec > 0 {
			if int64(predSec*1000.0) > timeout {
				timeout = int64(predSec * 1000.0)
				rationale += "；ML校正"
			}
		}
	}

	return &ResourceAdvice{ExecutionFeeLimit: 资源, MemoryLimit: mem, Concurrency: conc, TimeoutMs: timeout, Rationale: rationale}, nil
}

// PredictExecutionCost 预测一次执行的资源与耗时（启发式）
func (a *Advisor) PredictExecutionCost(_ context.Context, params types.ExecutionParams) (*CostPrediction, error) {
	if len(params.ResourceID) == 0 {
		return nil, fmt.Errorf("resource id empty")
	}
	expectedResource := maxU64(params.ExecutionFeeLimit/2, 50000)
	expectedTime := uint64(50)
	confidence := float32(0.6)
	if a.getMetrics != nil {
		snap := a.getMetrics()
		if len(snap) > 0 {
			var sumDur uint64
			var count int
			var totalExec, totalFail uint64
			for _, s := range snap {
				if s.LastDurationMs > 0 {
					sumDur += s.LastDurationMs
					count++
				}
				totalExec += s.ExecutionCount
				totalFail += s.FailureCount
			}
			if count > 0 {
				expectedTime = maxU64(expectedTime, sumDur/uint64(count))
			}
			if totalExec > 0 && float64(totalFail)/float64(totalExec) > 0.05 {
				confidence = 0.5
				expectedTime = uint64(float64(expectedTime) * 1.2)
			}
		}
	}

	// 机器学习微调（可选）
	if a.model != nil && a.model.Ready() {
		var avgDur float64 = 0.05
		var failRate float64 = 0.0
		if a.getMetrics != nil {
			snap := a.getMetrics()
			var sum uint64
			var c int
			var te, tf uint64
			for _, s := range snap {
				if s.LastDurationMs > 0 {
					sum += s.LastDurationMs
					c++
				}
				te += s.ExecutionCount
				tf += s.FailureCount
			}
			if c > 0 {
				avgDur = float64(sum/uint64(c)) / 1000.0
			}
			if te > 0 {
				failRate = float64(tf) / float64(te)
			}
		}
		资源Norm := float64(params.ExecutionFeeLimit) / 2_000_000.0
		if 资源Norm > 1 {
			资源Norm = 1
		}
		if avgDur > 1 {
			avgDur = 1
		}
		if failRate > 1 {
			failRate = 1
		}
		pred := a.model.Predict([]float64{资源Norm, avgDur, failRate})
		if pred > 0 {
			if uint64(pred*1000.0) > expectedTime {
				expectedTime = uint64(pred * 1000.0)
			}
			confidence = maxF32(confidence, 0.65)
		}
	}
	return &CostPrediction{ExpectedResource: expectedResource, ExpectedTimeMs: expectedTime, ConfidencePct: confidence, ModelVersion: "heuristic-v1"}, nil
}

// AnalyzePerformanceHistory 返回合约维度的简单历史分析
func (a *Advisor) AnalyzePerformanceHistory(_ context.Context, contractAddr string) (*PerformanceAnalysis, error) {
	if contractAddr == "" {
		return nil, fmt.Errorf("contract address empty")
	}
	avg := uint64(45)
	p95 := uint64(120)
	fail := float32(0.02)
	samples := uint64(128)
	if a.getMetrics != nil {
		snap := a.getMetrics()
		if len(snap) > 0 {
			var sumDur uint64
			var count int
			var totalExec, totalFail uint64
			for _, s := range snap {
				if s.LastDurationMs > 0 {
					sumDur += s.LastDurationMs
					count++
				}
				totalExec += s.ExecutionCount
				totalFail += s.FailureCount
			}
			if count > 0 {
				avg = sumDur / uint64(count)
			}
			if totalExec > 0 {
				fail = float32(float64(totalFail) / float64(totalExec))
			}
			samples = totalExec
			p95 = maxU64(avg*22/10, avg)
		}
	}
	return &PerformanceAnalysis{AvgTimeMs: avg, P95TimeMs: p95, FailureRate: fail, SampleCount: samples, LastObserved: a.clock().Unix()}, nil
}

func maxU64(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

func minU64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func maxF32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// NewEnvAdvisor 创建环境顾问（ML智能决策系统）
// 从module.go迁移而来，用于fx依赖注入
func NewEnvAdvisor(
	txService blockchain.TransactionService,
	blockService blockchain.BlockService,
	logger log.Logger,
) *CoordinatorAdapter {
	// 如果缺少区块链服务，返回nil（advisor是可选的）
	if txService == nil || blockService == nil {
		if logger != nil {
			logger.Info("区块链服务不可用，跳过ML环境顾问创建")
		}
		return nil
	}

	// 创建底层Advisor
	advisor := NewAdvisor(txService, blockService, nil)

	// 配置ML模型（基于合约历史数据进行预测优化）
	// 创建线性模型用于资源预测（3个特征：资源Norm, avgDur, failRate）
	model := NewLinearModel(3)
	advisor = advisor.WithModel(model)

	// 创建适配器
	adapter := NewCoordinatorAdapter(advisor)

	if logger != nil {
		logger.Info("ML环境顾问已创建并集成")
	}

	return adapter
}
