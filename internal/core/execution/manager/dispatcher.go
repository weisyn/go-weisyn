package manager

import (
	"fmt"
	"sync"
	"time"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	types "github.com/weisyn/v1/pkg/types"
)

// Dispatcher 智能执行分发器
//
// # 核心功能：
// - 智能引擎选择：基于入口函数、性能指标自动选择最优引擎
// - 熔断保护：连续失败时自动切换引擎，避免雪崩效应
// - 流量控制：令牌桶限流防止引擎过载，超限时自动回退
// - 动态优化：基于实时执行统计动态调整引擎选择策略
//
// # 支持策略：
// 1. 入口函数映射：根据函数名选择专用引擎（如推理用ONNX）
// 2. 熔断机制：失败阈值保护，冷却期自动恢复
// 3. 限流控制：令牌桶算法，平滑流量峰值
// 4. 动态选择：基于失败率和响应时间的智能选择
//
// # 设计目标：
// - 高可用：多级故障保护，确保服务连续性
// - 高性能：智能路由，最大化执行效率
// - 自适应：基于实时数据动态调优
// - 生产就绪：完整的监控、限流、熔断机制
type Dispatcher struct {
	// ==================== 核心组件 ====================

	// mgr 引擎管理器
	// 提供底层引擎管理和执行能力
	mgr *EngineManager

	// ==================== 路由策略 ====================

	// entryEngineMap 入口函数到引擎类型的映射
	// 示例：{"infer": ONNX, "execute": WASM, "predict": ONNX}
	// 用于根据函数名自动选择最适合的引擎类型
	entryEngineMap map[string]types.EngineType

	// enableDynamic 动态策略开关
	// 启用时基于执行统计动态选择最优引擎
	// 禁用时严格按照入口映射和故障回退执行
	enableDynamic bool

	// ==================== 熔断保护 ====================

	// cbThreshold 熔断触发阈值（连续失败次数）
	// 达到阈值时触发熔断，在冷却期内跳过该引擎
	cbThreshold int

	// cbCooldown 熔断冷却期
	// 熔断后需等待此时间才能重新尝试该引擎
	cbCooldown time.Duration

	// muCB 熔断状态锁
	// 保护熔断相关状态的并发访问安全
	muCB sync.Mutex

	// failCount 引擎失败计数
	// 记录每个引擎的连续失败次数
	failCount map[types.EngineType]int

	// openUntil 熔断结束时间
	// 记录每个引擎的熔断结束时间点
	openUntil map[types.EngineType]time.Time

	// ==================== 流量控制 ====================

	// muRL 限流状态锁
	// 保护令牌桶相关状态的并发访问安全
	muRL sync.Mutex

	// tokens 当前令牌数量
	// 每个引擎类型的可用令牌数
	tokens map[types.EngineType]float64

	// lastRefill 上次令牌补充时间
	// 用于计算令牌补充数量
	lastRefill map[types.EngineType]time.Time

	// capacity 令牌桶容量
	// 每个引擎类型的最大令牌数
	capacity map[types.EngineType]float64

	// refillRate 令牌补充速率（每秒）
	// 控制引擎的最大吞吐量
	refillRate map[types.EngineType]float64
}

// NewDispatcher 创建分发器
func NewDispatcher(mgr *EngineManager) *Dispatcher {
	return &Dispatcher{
		mgr:            mgr,
		entryEngineMap: make(map[string]types.EngineType),
		enableDynamic:  false,
		cbThreshold:    3,
		cbCooldown:     5 * time.Second,
		failCount:      make(map[types.EngineType]int),
		openUntil:      make(map[types.EngineType]time.Time),
		tokens:         make(map[types.EngineType]float64),
		lastRefill:     make(map[types.EngineType]time.Time),
		capacity:       make(map[types.EngineType]float64),
		refillRate:     make(map[types.EngineType]float64),
	}
}

// WithEntryEngineMap 注入入口函数→引擎类型映射
func (d *Dispatcher) WithEntryEngineMap(m map[string]types.EngineType) *Dispatcher {
	for k, v := range m {
		d.entryEngineMap[k] = v
	}
	return d
}

// WithDynamicStrategy 启用/关闭动态策略
func (d *Dispatcher) WithDynamicStrategy(enabled bool) *Dispatcher {
	d.enableDynamic = enabled
	return d
}

// WithCircuitBreakerConfig 配置熔断阈值与冷却期
func (d *Dispatcher) WithCircuitBreakerConfig(threshold int, cooldown time.Duration) *Dispatcher {
	if threshold > 0 {
		d.cbThreshold = threshold
	}
	if cooldown > 0 {
		d.cbCooldown = cooldown
	}
	return d
}

// WithRateLimit 为指定引擎配置令牌桶参数（容量、每秒填充速率）
func (d *Dispatcher) WithRateLimit(engine types.EngineType, capacity, refillPerSec float64) *Dispatcher {
	d.muRL.Lock()
	defer d.muRL.Unlock()
	if capacity <= 0 || refillPerSec <= 0 {
		return d
	}
	d.capacity[engine] = capacity
	d.refillRate[engine] = refillPerSec
	d.tokens[engine] = capacity
	d.lastRefill[engine] = time.Now()
	return d
}

// Dispatch 分发执行
func (d *Dispatcher) Dispatch(t types.EngineType, params types.ExecutionParams) (*types.ExecutionResult, error) {
	if d.mgr == nil {
		return nil, fmt.Errorf("engine manager is nil")
	}
	// 优先按入口函数策略选择引擎类型
	selected := t
	if params.Entry != "" {
		if mapped, ok := d.entryEngineMap[params.Entry]; ok {
			selected = mapped
		}
	}

	// 动态策略：如开启则依据指标选择更优引擎类型
	if d.enableDynamic {
		if dyn, ok := d.selectByMetrics(selected); ok {
			selected = dyn
		}
	}

	// 熔断检查
	if d.isCircuitOpen(selected) {
		// 熔断打开时使用默认回退顺序
		return d.mgr.ExecuteWithDefaultFailover(selected, params)
	}

	// 限流检查
	if !d.consumeToken(selected) {
		// 超限时回退
		return d.mgr.ExecuteWithDefaultFailover(selected, params)
	}

	// 正常执行
	res, err := d.mgr.executeWithEngine(selected, params)
	if err != nil {
		d.onFailure(selected)
		return nil, err
	}
	d.onSuccess(selected)
	return res, nil
}

// selectByMetrics 基于 EngineManager 指标选择更优引擎类型
// 策略：优先失败率低者；失败率相同则选择最近耗时短者；如无统计或不可比则返回false
func (d *Dispatcher) selectByMetrics(defaultEngine types.EngineType) (types.EngineType, bool) {
	if d.mgr == nil {
		return defaultEngine, false
	}
	stats := d.mgr.GetMetrics()
	if len(stats) == 0 {
		return defaultEngine, false
	}
	best := defaultEngine
	bestSet := false
	bestFailureRate := 0.0
	bestDurationMs := uint64(0)

	for et, st := range stats {
		// 仅考虑未熔断的引擎
		if d.isCircuitOpen(et) {
			continue
		}
		if st.ExecutionCount == 0 {
			continue
		}
		failureRate := float64(st.FailureCount) / float64(st.ExecutionCount)
		dur := st.LastDurationMs
		if !bestSet || failureRate < bestFailureRate || (failureRate == bestFailureRate && dur < bestDurationMs) {
			best = et
			bestFailureRate = failureRate
			bestDurationMs = dur
			bestSet = true
		}
	}
	if bestSet {
		return best, true
	}
	return defaultEngine, false
}

// ======== 熔断实现 ========

func (d *Dispatcher) isCircuitOpen(e types.EngineType) bool {
	d.muCB.Lock()
	defer d.muCB.Unlock()
	until, ok := d.openUntil[e]
	if ok && time.Now().Before(until) {
		return true
	}
	return false
}

func (d *Dispatcher) onFailure(e types.EngineType) {
	d.muCB.Lock()
	defer d.muCB.Unlock()
	d.failCount[e]++
	if d.failCount[e] >= d.cbThreshold {
		d.openUntil[e] = time.Now().Add(d.cbCooldown)
		d.failCount[e] = 0
	}
}

func (d *Dispatcher) onSuccess(e types.EngineType) {
	d.muCB.Lock()
	defer d.muCB.Unlock()
	d.failCount[e] = 0
	delete(d.openUntil, e)
}

// ======== 令牌桶实现 ========

func (d *Dispatcher) consumeToken(e types.EngineType) bool {
	d.muRL.Lock()
	defer d.muRL.Unlock()
	cap, okCap := d.capacity[e]
	rate, okRate := d.refillRate[e]
	if !okCap || !okRate {
		return true // 未配置限流
	}
	now := time.Now()
	last := d.lastRefill[e]
	elapsed := now.Sub(last).Seconds()
	if elapsed > 0 {
		d.tokens[e] = minFloat(cap, d.tokens[e]+elapsed*rate)
		d.lastRefill[e] = now
	}
	if d.tokens[e] >= 1.0 {
		d.tokens[e] -= 1.0
		return true
	}
	return false
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

var _ execiface.EngineAdapter // 保持对接口层编译依赖（仅说明用途）

// NewExecutionDispatcher 创建配置好的执行分发器
// 从module.go迁移而来，用于fx依赖注入
func NewExecutionDispatcher(
	registry *Registry,
	logger log.Logger,
) *Dispatcher {
	// 创建引擎管理器
	engineManager := NewEngineManager(registry)

	// 创建分发器并配置熔断/限流参数
	dispatcher := NewDispatcher(engineManager).
		WithCircuitBreakerConfig(3, 5*time.Second). // 3次失败后熔断5秒
		WithRateLimit(types.EngineTypeWASM, 10, 2). // WASM: 容量10，每秒补充2个token
		WithRateLimit(types.EngineTypeONNX, 5, 1).  // ONNX: 容量5，每秒补充1个token
		WithDynamicStrategy(true)                   // 启用动态引擎选择

	// 配置引擎映射策略（基于函数入口选择引擎）
	entryEngineMap := map[string]types.EngineType{
		"execute":  types.EngineTypeWASM, // 通用执行使用WASM
		"infer":    types.EngineTypeONNX, // 推理使用ONNX
		"predict":  types.EngineTypeONNX, // 预测使用ONNX
		"contract": types.EngineTypeWASM, // 合约执行使用WASM
	}
	dispatcher.WithEntryEngineMap(entryEngineMap)

	if logger != nil {
		logger.Info("执行分发器已创建，启用熔断/限流/智能调度功能")
	}

	return dispatcher
}
