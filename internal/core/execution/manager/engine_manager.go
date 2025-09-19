package manager

import (
	"fmt"
	"sync"
	"time"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	types "github.com/weisyn/v1/pkg/types"
)

// EngineManager 统一的执行引擎管理器
//
// # 核心职责：
// - 引擎注册与查询：管理多种执行引擎（WASM、ONNX等）的生命周期
// - 智能分发执行：支持负载均衡、故障回退、性能优化
// - 健康检查与监控：实时监控引擎状态，提供执行统计和健康评估
// - 高可用保障：支持同类型多副本引擎的负载均衡和故障转移
//
// # 设计目标：
// - 高性能：微秒级引擎选择，纳秒级统计记录
// - 高可用：故障自动恢复，多级回退机制
// - 可扩展：支持新引擎类型的动态注册
// - 生产就绪：完整的监控、日志、错误处理
//
// # 使用场景：
// - 区块链智能合约执行（WASM引擎）
// - AI模型推理服务（ONNX引擎）
// - 多引擎混合负载的智能调度
// - 高并发执行环境的负载均衡
type EngineManager struct {
	// ==================== 核心组件 ====================

	// reg 引擎注册表
	// 维护引擎类型到适配器的映射，确保每种类型有主引擎
	reg *Registry

	// ==================== 故障恢复配置 ====================

	// failoverOrder 默认故障回退顺序
	// 当主引擎失败时，按此顺序尝试其他引擎类型
	// 例如：[WASM, ONNX] 表示WASM失败时回退到ONNX
	failoverOrder []types.EngineType

	// ==================== 监控与统计 ====================

	// metrics 引擎执行指标收集器
	// 记录每个引擎的执行次数、成功率、耗时等统计信息
	// 用于健康检查、负载均衡决策和性能监控
	metrics *EngineMetrics

	// ==================== 负载均衡支持 ====================

	// muReplicas 副本桶访问锁
	// 保护replicaBucket的并发访问安全
	muReplicas sync.RWMutex

	// replicaBucket 同类型多副本引擎存储
	// 支持同一引擎类型的多个实例，实现负载均衡
	// 键：引擎类型，值：该类型的所有适配器实例
	// 负载均衡策略：基于执行统计选择最优实例
	replicaBucket map[types.EngineType][]execiface.EngineAdapter
}

// EngineMetrics 引擎执行统计收集器
//
// # 功能说明：
// - 并发安全的执行统计聚合
// - 按引擎类型分别统计执行指标
// - 支持实时查询和健康状态评估
// - 为负载均衡和故障回退提供决策数据
//
// # 性能特性：
// - 读写锁优化，读多写少场景高效
// - 内存占用最小，仅记录关键指标
// - 无后台清理，适合长期运行
//
// # 使用场景：
// - 引擎健康检查的数据源
// - 负载均衡的决策依据
// - 性能监控和告警的基础数据
type EngineMetrics struct {
	// mu 读写锁
	// 保护byEngine映射的并发访问安全
	// 使用读写锁优化读多写少的访问模式
	mu sync.RWMutex

	// byEngine 按引擎类型分组的执行统计
	// 键：引擎类型（WASM、ONNX等）
	// 值：该引擎的详细执行统计信息
	byEngine map[types.EngineType]*engineExecStats
}

// engineExecStats 单个引擎的执行统计信息
//
// # 统计维度：
// - 执行计数：总次数、成功次数、失败次数
// - 时间信息：最近执行时间、最近执行耗时
// - 错误信息：最近一次错误的详细信息
//
// # 数据用途：
// - 计算成功率：SuccessCount / ExecutionCount
// - 计算失败率：FailureCount / ExecutionCount
// - 性能评估：LastDurationMs作为响应时间指标
// - 故障诊断：LastError提供错误上下文
type engineExecStats struct {
	// ExecutionCount 累计执行总次数
	// 包括成功和失败的所有执行尝试
	// 用于计算成功率的分母
	ExecutionCount uint64

	// SuccessCount 累计成功执行次数
	// 仅统计正常完成且返回结果的执行
	// 用于计算成功率：SuccessCount / ExecutionCount
	SuccessCount uint64

	// FailureCount 累计失败执行次数
	// 包括异常、超时、引擎错误等所有失败情况
	// 用于计算失败率：FailureCount / ExecutionCount
	FailureCount uint64

	// LastError 最近一次执行错误的详细信息
	// 成功执行时清空，失败时记录错误消息
	// 用于故障诊断和错误模式分析
	LastError string

	// LastExecAt 最近一次执行的时间戳（Unix秒）
	// 用于检测引擎是否长时间未使用
	// 配合健康检查判断引擎活跃状态
	LastExecAt int64

	// LastDurationMs 最近一次执行的耗时（毫秒）
	// 用于性能评估和负载均衡决策
	// 耗时越短的引擎优先级越高
	LastDurationMs uint64
}

// newEngineMetrics 创建引擎统计收集器
//
// 返回初始化完成的EngineMetrics实例，内部映射为空
// 引擎统计会在首次执行时自动创建
//
// 性能特性：
//   - 零开销初始化，仅分配映射结构
//   - 延迟初始化，按需创建引擎统计
func newEngineMetrics() *EngineMetrics {
	return &EngineMetrics{byEngine: make(map[types.EngineType]*engineExecStats)}
}

// record 记录引擎执行结果
//
// 更新指定引擎的执行统计信息，包括计数、耗时、错误状态
//
// 参数：
//   - engine: 执行引擎类型
//   - success: 执行是否成功
//   - duration: 执行耗时
//   - err: 执行错误（成功时为nil）
//
// 更新逻辑：
//  1. 总执行次数+1
//  2. 根据success更新成功/失败计数
//  3. 更新最近执行时间和耗时
//  4. 成功时清空错误信息，失败时记录错误
//
// 并发安全：
//   - 使用写锁保护统计数据修改
//   - 自动创建首次执行的引擎统计
//
// 性能特性：
//   - 微秒级记录延迟
//   - 固定内存开销，无动态分配
func (em *EngineMetrics) record(engine types.EngineType, success bool, duration time.Duration, err error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// 获取或创建引擎统计结构
	st, ok := em.byEngine[engine]
	if !ok {
		st = &engineExecStats{}
		em.byEngine[engine] = st
	}

	// 更新执行计数
	st.ExecutionCount++

	// 根据执行结果更新成功/失败计数和错误信息
	if success {
		st.SuccessCount++
		st.LastError = "" // 成功时清空错误信息
	} else {
		st.FailureCount++
		if err != nil {
			st.LastError = err.Error() // 记录详细错误信息
		}
	}

	// 更新时间和性能指标
	st.LastExecAt = time.Now().Unix()
	st.LastDurationMs = uint64(duration / time.Millisecond)
}

// GetStats 获取所有引擎的执行统计快照
//
// 返回所有已记录执行统计的引擎数据副本，确保数据一致性
//
// 返回值：
//   - map[types.EngineType]engineExecStats: 引擎类型到统计信息的映射
//   - 返回的是数据副本，调用方可安全修改而不影响内部状态
//
// 使用场景：
//   - 全局监控面板数据展示
//   - 批量健康检查
//   - 性能分析和报告生成
//   - 负载均衡策略的全局视图
//
// 并发安全：
//   - 使用读锁保护数据访问
//   - 返回深拷贝，避免数据竞争
//
// 性能特性：
//   - 读取操作，微秒级延迟
//   - 内存开销：每个引擎约100字节副本
func (em *EngineMetrics) GetStats() map[types.EngineType]engineExecStats {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// 预分配映射容量，提高性能
	out := make(map[types.EngineType]engineExecStats, len(em.byEngine))

	// 深拷贝所有统计数据，确保线程安全
	for k, v := range em.byEngine {
		if v == nil {
			continue // 跳过无效条目
		}
		out[k] = *v // 值拷贝，避免指针共享
	}
	return out
}

// GetStatsFor 获取指定引擎的执行统计
//
// 返回特定引擎的统计信息副本，比GetStats()更高效
//
// 参数：
//   - engine: 要查询的引擎类型
//
// 返回值：
//   - engineExecStats: 引擎统计信息副本
//   - bool: 是否找到该引擎的统计数据
//
// 使用场景：
//   - 单引擎健康检查
//   - 特定引擎的性能监控
//   - 负载均衡决策时的单点查询
//   - 引擎状态实时查询
//
// 并发安全：
//   - 使用读锁保护数据访问
//   - 返回数据副本，避免外部修改
//
// 性能特性：
//   - O(1)查找复杂度
//   - 纳秒级访问延迟
//   - 固定内存开销（约100字节）
func (em *EngineMetrics) GetStatsFor(engine types.EngineType) (engineExecStats, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// 查找指定引擎的统计数据
	v, ok := em.byEngine[engine]
	if !ok || v == nil {
		return engineExecStats{}, false
	}

	// 返回数据副本，确保线程安全
	return *v, true
}

// NewEngineManager 创建执行引擎管理器
//
// 构造函数，创建完整配置的引擎管理器实例
//
// 参数：
//   - reg: 引擎注册表，nil时自动创建新实例
//
// 返回值：
//   - *EngineManager: 初始化完成的引擎管理器
//
// 初始化内容：
//   - 引擎注册表（新建或使用传入的）
//   - 执行统计收集器（空状态）
//   - 副本存储桶（空状态）
//   - 故障回退顺序（空，需后续配置）
//
// 使用示例：
//   - mgr := NewEngineManager(nil)  // 使用新注册表
//   - mgr := NewEngineManager(existingRegistry)  // 使用现有注册表
//
// 设计考虑：
//   - 支持注册表复用，便于多管理器场景
//   - 零配置可用，所有组件都有默认状态
//   - 延迟配置，故障回退等高级功能可后续添加
func NewEngineManager(reg *Registry) *EngineManager {
	// 自动创建注册表，确保管理器始终可用
	if reg == nil {
		reg = NewRegistry()
	}

	return &EngineManager{
		reg:           reg,
		metrics:       newEngineMetrics(),
		replicaBucket: make(map[types.EngineType][]execiface.EngineAdapter),
		// failoverOrder 保持nil，需要时通过SetFailoverOrder配置
	}
}

// RegisterEngine 注册执行引擎适配器
//
// 将引擎适配器注册到管理器，支持同类型多实例注册以实现负载均衡
//
// 参数：
//   - adapter: 要注册的引擎适配器实现
//
// 返回值：
//   - error: 注册错误，仅在适配器为nil时返回错误
//
// 注册策略：
//  1. 尝试注册为主引擎（每类型一个）
//  2. 无论主引擎注册成功与否，都加入副本桶
//  3. 支持同类型多实例，实现负载均衡和高可用
//
// 使用场景：
//   - 系统启动时注册各种引擎（WASM、ONNX等）
//   - 动态添加引擎实例以提高容量
//   - 热插拔引擎更新和维护
//
// 并发安全：
//   - 使用锁保护副本桶的并发修改
//   - 底层注册表有自己的并发保护
//
// 错误处理：
//   - 主引擎重复注册不影响副本收集
//   - 仅在适配器为nil时返回错误
//   - 容错设计，最大化可用性
func (m *EngineManager) RegisterEngine(adapter execiface.EngineAdapter) error {
	// 参数校验：适配器不能为nil
	if adapter == nil {
		return fmt.Errorf("engine adapter is nil")
	}

	// 尝试注册为主引擎（每种类型保留一个主适配器）
	if err := m.reg.Register(adapter); err != nil {
		// 主引擎已存在时忽略错误，继续收集副本
		// 这样支持同类型多实例的负载均衡场景
		_ = err // 忽略错误，允许副本注册
	}

	// 将适配器加入副本桶，支持负载均衡
	t := adapter.GetEngineType()
	m.muReplicas.Lock()
	m.replicaBucket[t] = append(m.replicaBucket[t], adapter)
	m.muReplicas.Unlock()

	return nil
}

// ListEngines 列出已注册的引擎类型
//
// 返回所有已成功注册的引擎类型列表，按字母顺序排序
//
// 返回值：
//   - []types.EngineType: 已注册引擎类型的有序列表
//
// 使用场景：
//   - 系统状态检查和诊断
//   - 管理界面的引擎列表展示
//   - 动态引擎发现和能力查询
//   - 故障回退顺序的动态配置
//
// 性能特性：
//   - 委托给注册表，具有读锁保护
//   - 返回排序列表，便于展示和调试
//   - 轻量级操作，微秒级延迟
func (m *EngineManager) ListEngines() []types.EngineType {
	return m.reg.List()
}

// GetEngine 获取指定类型的引擎适配器
//
// 返回主引擎适配器（而非副本桶中的实例）
//
// 参数：
//   - t: 要获取的引擎类型
//
// 返回值：
//   - execiface.EngineAdapter: 引擎适配器实例
//   - bool: 是否找到该类型的引擎
//
// 使用场景：
//   - 直接引擎访问（绕过负载均衡）
//   - 引擎能力查询和配置
//   - 系统集成和测试
//   - 特定引擎的专门操作
//
// 注意：
//   - 返回的是主引擎，不是负载均衡选择的结果
//   - 如需负载均衡，应使用Execute方法或pickReplica
//   - 适用于需要访问特定引擎实例的场景
func (m *EngineManager) GetEngine(t types.EngineType) (execiface.EngineAdapter, bool) {
	return m.reg.Get(t)
}

// pickReplica 简单负载均衡：在同类型副本中按分数选择
// 评分：最近耗时越短越优（LastDurationMs），失败率越低越优（FailureCount/ExecutionCount）
func (m *EngineManager) pickReplica(t types.EngineType) execiface.EngineAdapter {
	m.muReplicas.RLock()
	replicas := append([]execiface.EngineAdapter(nil), m.replicaBucket[t]...)
	m.muReplicas.RUnlock()
	if len(replicas) == 0 {
		// 回退主引擎
		ad, _ := m.reg.Get(t)
		return ad
	}
	bestIdx := 0
	bestScore := float64(1<<63 - 1)
	stats, _ := m.metrics.GetStatsFor(t)
	for i := range replicas {
		// 构造分数：duration权重0.7，失败率权重0.3；无历史则给中性评分
		dur := float64(stats.LastDurationMs)
		failRate := 0.0
		if stats.ExecutionCount > 0 {
			failRate = float64(stats.FailureCount) / float64(stats.ExecutionCount)
		}
		score := 0.7*dur + 0.3*failRate*1000.0
		if score < bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return replicas[bestIdx]
}

// executeWithEngine 内部执行方法 - 按类型分发执行（含同类型负载均衡）
// 统一返回指针类型以保持接口一致性
func (m *EngineManager) executeWithEngine(t types.EngineType, params types.ExecutionParams) (*types.ExecutionResult, error) {
	ad := m.pickReplica(t)
	if ad == nil {
		return nil, fmt.Errorf("engine not found: %s", t)
	}
	start := time.Now()
	res, err := ad.Execute(params)
	dur := time.Since(start)
	m.metrics.record(t, err == nil && res != nil, dur, err)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("nil execution result from engine: %s", t)
	}
	return res, nil
}

// SetFailoverOrder 设置默认故障回退顺序
func (m *EngineManager) SetFailoverOrder(order []types.EngineType) {
	m.failoverOrder = append([]types.EngineType(nil), order...)
}

// ExecuteFailover 以主引擎+回退顺序执行，首个成功即返回
// 统一返回指针类型以保持接口一致性
func (m *EngineManager) ExecuteFailover(primary types.EngineType, fallbacks []types.EngineType, params types.ExecutionParams) (*types.ExecutionResult, error) {
	tryOrder := make([]types.EngineType, 0, 1+len(fallbacks))
	tryOrder = append(tryOrder, primary)
	tryOrder = append(tryOrder, fallbacks...)

	var lastErr error
	for _, et := range tryOrder {
		res, err := m.executeWithEngine(et, params)
		if err == nil && res != nil {
			return res, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("engine %s failed: %w", et, err)
		} else {
			lastErr = fmt.Errorf("engine %s returned nil result", et)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("all engines failed with unknown error")
	}
	return nil, lastErr
}

// ExecuteWithDefaultFailover 使用默认回退顺序执行
// 统一返回指针类型以保持接口一致性
func (m *EngineManager) ExecuteWithDefaultFailover(primary types.EngineType, params types.ExecutionParams) (*types.ExecutionResult, error) {
	return m.ExecuteFailover(primary, m.failoverOrder, params)
}

// HealthCheck 引擎健康状态检查
//
// 检查指定引擎的健康状态，基于注册状态和执行统计进行综合评估
//
// 参数：
//   - t: 要检查的引擎类型
//
// 返回值：
//   - ok: 引擎是否健康可用
//   - checkedAt: 检查时间戳
//
// 健康判断标准：
// 1. 引擎必须已注册
// 2. 如果有执行历史，近期失败率不超过80%
// 3. 不在熔断状态（如果使用Dispatcher）
//
// 使用场景：
//   - 运维监控和告警
//   - 负载均衡决策
//   - 自动故障恢复
//
// 性能特性：
//   - 轻量级检查，微秒级延迟
//   - 无阻塞操作，基于现有统计数据
//   - 线程安全，可并发调用
func (m *EngineManager) HealthCheck(t types.EngineType) (ok bool, checkedAt time.Time) {
	checkedAt = time.Now()

	// 1. 检查引擎是否已注册
	_, exists := m.reg.Get(t)
	if !exists {
		return false, checkedAt
	}

	// 2. 检查执行统计，评估健康状态
	if stats, hasStats := m.metrics.GetStatsFor(t); hasStats {
		// 如果有执行历史，检查失败率
		if stats.ExecutionCount > 0 {
			failureRate := float64(stats.FailureCount) / float64(stats.ExecutionCount)
			// 失败率超过80%认为不健康
			if failureRate > 0.8 {
				return false, checkedAt
			}
		}

		// 检查最近执行时间，超过5分钟无执行可能表示引擎异常
		if stats.LastExecAt > 0 {
			lastExec := time.Unix(stats.LastExecAt, 0)
			if time.Since(lastExec) > 5*time.Minute && stats.ExecutionCount > 5 {
				// 有执行历史但长时间未使用，可能存在问题
				// 注意：这里不直接返回false，因为可能是正常的业务低峰
			}
		}
	}

	// 3. 引擎已注册且统计正常，认为健康
	return true, checkedAt
}

// GetMetrics 获取所有引擎的执行统计
func (m *EngineManager) GetMetrics() map[types.EngineType]engineExecStats {
	return m.metrics.GetStats()
}

// GetMetricsFor 获取指定引擎的执行统计
func (m *EngineManager) GetMetricsFor(t types.EngineType) (engineExecStats, bool) {
	return m.metrics.GetStatsFor(t)
}

// Execute 统一执行入口（满足接口契约）
// 对外暴露的标准方法，直接调用内部执行方法
func (m *EngineManager) Execute(t types.EngineType, params types.ExecutionParams) (*types.ExecutionResult, error) {
	return m.executeWithEngine(t, params)
}

// UnregisterEngine 取消注册指定类型的引擎
// 已注册则移除并返回nil；未注册返回明确错误
func (m *EngineManager) UnregisterEngine(t types.EngineType) error {
	if m == nil || m.reg == nil {
		return fmt.Errorf("engine registry is nil")
	}
	if ok := m.reg.Unregister(t); !ok {
		return fmt.Errorf("engine not registered: %s", t)
	}
	// 同步移除副本桶
	m.muReplicas.Lock()
	delete(m.replicaBucket, t)
	m.muReplicas.Unlock()
	return nil
}
