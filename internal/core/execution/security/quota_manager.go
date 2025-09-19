package security

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// QuotaManager 配额管理器
//
// 职责：
// 1. 管理执行资源配额（时间、内存、指令、资源等）
// 2. 执行前配额检查和预分配
// 3. 执行后配额使用统计和回收
// 4. 超限检测和处理
// 5. 配额策略动态调整
//
// 设计原则：
// - 支持多维度配额管理（全局、引擎、用户、合约）
// - 提供灵活的配额策略配置
// - 实现配额公平性和防滥用机制
type QuotaManager struct {
	// 全局配额池
	globalQuotas map[QuotaType]*QuotaPool

	// 用户配额池（按调用者地址）
	userQuotas map[string]map[QuotaType]*QuotaPool

	// 合约配额池（按合约地址）
	contractQuotas map[string]map[QuotaType]*QuotaPool

	// 引擎配额池（按引擎类型）
	engineQuotas map[types.EngineType]map[QuotaType]*QuotaPool

	// 配额策略配置
	policies *QuotaPolicies

	// 配额使用统计
	usageStats *QuotaUsageStats

	// 审计事件发射器
	auditEmitter AuditEventEmitter

	// config 已移除 - 使用固定的智能配额策略

	// 运行时状态
	mutex             sync.RWMutex
	activeAllocations map[string]*QuotaAllocation // 活跃的配额分配
	limitViolations   []QuotaViolation            // 违限记录
}

// QuotaManagerConfig 已删除 - 使用固定的智能配额策略
// 所有配额策略均为智能默认，无需配置

// QuotaType 配额类型
type QuotaType string

const (
	QuotaTypeExecutionTime QuotaType = "execution_time" // 执行时间配额（毫秒）
	QuotaTypeMemory        QuotaType = "memory"         // 内存配额（字节）
	QuotaTypeResource      QuotaType = "resource"       // 资源配额
	QuotaTypeInstructions  QuotaType = "instructions"   // 指令数配额
	QuotaTypeCPU           QuotaType = "cpu"            // CPU配额（毫秒）
	QuotaTypeNetworkCalls  QuotaType = "network_calls"  // 网络调用次数配额
	QuotaTypeStateOps      QuotaType = "state_ops"      // 状态操作次数配额
	QuotaTypeStorageBytes  QuotaType = "storage_bytes"  // 存储字节数配额
	QuotaTypeRequests      QuotaType = "requests"       // 请求次数配额
)

// QuotaPool 配额池
type QuotaPool struct {
	// 配额类型
	Type QuotaType `json:"type"`

	// 总配额
	Total uint64 `json:"total"`

	// 已使用配额
	Used uint64 `json:"used"`

	// 保留配额
	Reserved uint64 `json:"reserved"`

	// 配额刷新周期（秒）
	RefreshPeriodSec int64 `json:"refresh_period_sec"`

	// 上次刷新时间
	LastRefresh time.Time `json:"last_refresh"`

	// 配额池状态
	Status QuotaPoolStatus `json:"status"`

	// 并发控制
	mutex sync.RWMutex
}

// QuotaPoolStatus 配额池状态
type QuotaPoolStatus string

const (
	QuotaPoolStatusActive    QuotaPoolStatus = "active"
	QuotaPoolStatusWarning   QuotaPoolStatus = "warning"
	QuotaPoolStatusCritical  QuotaPoolStatus = "critical"
	QuotaPoolStatusExhausted QuotaPoolStatus = "exhausted"
	QuotaPoolStatusSuspended QuotaPoolStatus = "suspended"
)

// QuotaPolicies 配额策略配置
type QuotaPolicies struct {
	// 全局配额策略
	Global map[QuotaType]QuotaPolicy `json:"global"`

	// 用户配额策略
	User map[QuotaType]QuotaPolicy `json:"user"`

	// 合约配额策略
	Contract map[QuotaType]QuotaPolicy `json:"contract"`

	// 引擎配额策略
	Engine map[types.EngineType]map[QuotaType]QuotaPolicy `json:"engine"`

	// 配额优先级策略
	Priority QuotaPriorityPolicy `json:"priority"`
}

// QuotaPolicy 配额策略
type QuotaPolicy struct {
	// 初始配额
	InitialQuota uint64 `json:"initial_quota"`

	// 最大配额
	MaxQuota uint64 `json:"max_quota"`

	// 最小配额
	MinQuota uint64 `json:"min_quota"`

	// 配额刷新周期（秒）
	RefreshPeriodSec int64 `json:"refresh_period_sec"`

	// 配额增长策略
	GrowthStrategy QuotaGrowthStrategy `json:"growth_strategy"`

	// 配额回收策略
	RecycleStrategy QuotaRecycleStrategy `json:"recycle_strategy"`

	// 超限处理策略
	OverlimitStrategy QuotaOverlimitStrategy `json:"overlimit_strategy"`

	// 是否启用突发配额
	EnableBurst bool `json:"enable_burst"`

	// 突发配额大小
	BurstSize uint64 `json:"burst_size"`
}

// QuotaGrowthStrategy 配额增长策略
type QuotaGrowthStrategy string

const (
	GrowthStrategyFixed       QuotaGrowthStrategy = "fixed"       // 固定配额
	GrowthStrategyLinear      QuotaGrowthStrategy = "linear"      // 线性增长
	GrowthStrategyExponential QuotaGrowthStrategy = "exponential" // 指数增长
	GrowthStrategyAdaptive    QuotaGrowthStrategy = "adaptive"    // 自适应增长
)

// QuotaRecycleStrategy 配额回收策略
type QuotaRecycleStrategy string

const (
	RecycleStrategyImmediate QuotaRecycleStrategy = "immediate" // 立即回收
	RecycleStrategyDelayed   QuotaRecycleStrategy = "delayed"   // 延迟回收
	RecycleStrategyPeriodic  QuotaRecycleStrategy = "periodic"  // 周期性回收
)

// QuotaOverlimitStrategy 超限处理策略
type QuotaOverlimitStrategy string

const (
	OverlimitStrategyReject  QuotaOverlimitStrategy = "reject"  // 拒绝执行
	OverlimitStrategyQueue   QuotaOverlimitStrategy = "queue"   // 排队等待
	OverlimitStrategyDegrade QuotaOverlimitStrategy = "degrade" // 降级执行
	OverlimitStrategyBorrow  QuotaOverlimitStrategy = "borrow"  // 借用配额
)

// QuotaPriorityPolicy 配额优先级策略
type QuotaPriorityPolicy struct {
	// 优先级顺序（高到低）
	PriorityOrder []string `json:"priority_order"`

	// 优先级权重
	PriorityWeights map[string]float64 `json:"priority_weights"`

	// 是否启用优先级抢占
	EnablePreemption bool `json:"enable_preemption"`
}

// QuotaAllocation 配额分配
type QuotaAllocation struct {
	// 分配ID
	AllocationID string `json:"allocation_id"`

	// 分配时间
	AllocatedAt time.Time `json:"allocated_at"`

	// 执行参数
	ExecutionParams types.ExecutionParams `json:"execution_params"`

	// 分配的配额
	AllocatedQuotas map[QuotaType]uint64 `json:"allocated_quotas"`

	// 实际使用的配额
	UsedQuotas map[QuotaType]uint64 `json:"used_quotas"`

	// 分配状态
	Status QuotaAllocationStatus `json:"status"`

	// 过期时间
	ExpiresAt time.Time `json:"expires_at"`
}

// QuotaAllocationStatus 配额分配状态
type QuotaAllocationStatus string

const (
	AllocationStatusAllocated QuotaAllocationStatus = "allocated"
	AllocationStatusUsing     QuotaAllocationStatus = "using"
	AllocationStatusCompleted QuotaAllocationStatus = "completed"
	AllocationStatusExpired   QuotaAllocationStatus = "expired"
	AllocationStatusCancelled QuotaAllocationStatus = "cancelled"
)

// QuotaViolation 配额违限记录
type QuotaViolation struct {
	ViolationID   string                 `json:"violation_id"`
	ViolationType string                 `json:"violation_type"`
	QuotaType     QuotaType              `json:"quota_type"`
	Requested     uint64                 `json:"requested"`
	Available     uint64                 `json:"available"`
	Severity      string                 `json:"severity"`
	Context       map[string]interface{} `json:"context"`
	Timestamp     int64                  `json:"timestamp"`
	Action        string                 `json:"action"`
}

// QuotaUsageStats 配额使用统计
type QuotaUsageStats struct {
	// 全局统计
	GlobalStats map[QuotaType]*QuotaTypeStat `json:"global_stats"`

	// 用户统计
	UserStats map[string]map[QuotaType]*QuotaTypeStat `json:"user_stats"`

	// 合约统计
	ContractStats map[string]map[QuotaType]*QuotaTypeStat `json:"contract_stats"`

	// 引擎统计
	EngineStats map[types.EngineType]map[QuotaType]*QuotaTypeStat `json:"engine_stats"`

	// 统计更新时间
	LastUpdated time.Time `json:"last_updated"`

	// 并发控制
	mutex sync.RWMutex
}

// QuotaTypeStat 配额类型统计
type QuotaTypeStat struct {
	TotalAllocated uint64    `json:"total_allocated"`
	TotalUsed      uint64    `json:"total_used"`
	TotalWasted    uint64    `json:"total_wasted"`
	PeakUsage      uint64    `json:"peak_usage"`
	AverageUsage   float64   `json:"average_usage"`
	RequestCount   uint64    `json:"request_count"`
	ViolationCount uint64    `json:"violation_count"`
	LastUsed       time.Time `json:"last_used"`
}

// NewQuotaManager 创建配额管理器
func NewQuotaManager(
	policies *QuotaPolicies,
	auditEmitter AuditEventEmitter,
) *QuotaManager {
	// 使用更大的默认配额以支持合约执行

	if policies == nil {
		policies = DefaultQuotaPolicies()
	}

	// 强制增加执行时间配额以支持合约执行
	policies.Global[QuotaTypeExecutionTime] = QuotaPolicy{
		InitialQuota:      1000000,  // 增加到100万毫秒
		MaxQuota:          10000000, // 增加到1000万毫秒
		MinQuota:          1000,
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}

	// 强制增加内存配额以支持合约执行
	policies.Global[QuotaTypeMemory] = QuotaPolicy{
		InitialQuota:      200000000,  // 增加到200MB
		MaxQuota:          2000000000, // 增加到2GB
		MinQuota:          1048576,    // 1MB
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}

	// 强制增加资源配额以支持合约执行
	policies.Global[QuotaTypeResource] = QuotaPolicy{
		InitialQuota:      10000000,  // 增加到1000万资源
		MaxQuota:          100000000, // 增加到1亿资源
		MinQuota:          10000,     // 1万资源
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}

	// 强制增加所有其他配额类型以支持合约执行
	policies.Global[QuotaTypeInstructions] = QuotaPolicy{
		InitialQuota:      100000000,  // 1亿指令
		MaxQuota:          1000000000, // 10亿指令
		MinQuota:          10000,      // 1万指令
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}

	policies.Global[QuotaTypeCPU] = QuotaPolicy{
		InitialQuota:      1000000,  // 1000秒CPU时间
		MaxQuota:          10000000, // 10000秒CPU时间
		MinQuota:          1000,     // 1秒
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}

	policies.Global[QuotaTypeNetworkCalls] = QuotaPolicy{
		InitialQuota:      100000,  // 10万次网络调用
		MaxQuota:          1000000, // 100万次
		MinQuota:          100,     // 100次
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}

	policies.Global[QuotaTypeStateOps] = QuotaPolicy{
		InitialQuota:      1000000,  // 100万次状态操作
		MaxQuota:          10000000, // 1000万次
		MinQuota:          1000,     // 1000次
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}

	policies.Global[QuotaTypeStorageBytes] = QuotaPolicy{
		InitialQuota:      100000000,  // 100MB存储
		MaxQuota:          1000000000, // 1GB存储
		MinQuota:          1048576,    // 1MB
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}

	policies.Global[QuotaTypeRequests] = QuotaPolicy{
		InitialQuota:      100000,  // 10万次请求
		MaxQuota:          1000000, // 100万次
		MinQuota:          100,     // 100次
		RefreshPeriodSec:  3600,
		GrowthStrategy:    GrowthStrategyFixed,
		RecycleStrategy:   RecycleStrategyImmediate,
		OverlimitStrategy: OverlimitStrategyReject,
	}

	qm := &QuotaManager{
		globalQuotas:   make(map[QuotaType]*QuotaPool),
		userQuotas:     make(map[string]map[QuotaType]*QuotaPool),
		contractQuotas: make(map[string]map[QuotaType]*QuotaPool),
		engineQuotas:   make(map[types.EngineType]map[QuotaType]*QuotaPool),
		policies:       policies,
		usageStats:     NewQuotaUsageStats(),
		auditEmitter:   auditEmitter,
		// config已移除，使用固定的智能配额策略
		activeAllocations: make(map[string]*QuotaAllocation),
		limitViolations:   make([]QuotaViolation, 0, 1000), // 固定智能默认值
	}

	// 初始化全局配额池
	qm.initializeGlobalQuotas()

	return qm
}

// DefaultQuotaManagerConfig 已删除 - 不再需要配置函数
// 所有配额策略均为智能默认，无需配置

// DefaultQuotaPolicies 默认配额策略
func DefaultQuotaPolicies() *QuotaPolicies {

	return &QuotaPolicies{
		Global: map[QuotaType]QuotaPolicy{
			QuotaTypeExecutionTime: {
				InitialQuota:      1000000,  // 100万毫秒（1000秒）
				MaxQuota:          10000000, // 1000万毫秒（10000秒）
				MinQuota:          1000,     // 1秒
				RefreshPeriodSec:  3600,
				GrowthStrategy:    GrowthStrategyFixed,
				RecycleStrategy:   RecycleStrategyImmediate,
				OverlimitStrategy: OverlimitStrategyReject,
			},
			QuotaTypeMemory: {
				InitialQuota:      536870912,  // 🔧 强制修复：512MB内存配额
				MaxQuota:          1073741824, // 1GB
				MinQuota:          1048576,    // 1MB
				RefreshPeriodSec:  3600,
				GrowthStrategy:    GrowthStrategyFixed,
				RecycleStrategy:   RecycleStrategyImmediate,
				OverlimitStrategy: OverlimitStrategyReject,
			},
			QuotaTypeResource: {
				InitialQuota:      1000000,  // 100万资源
				MaxQuota:          10000000, // 1000万资源
				MinQuota:          10000,    // 1万资源
				RefreshPeriodSec:  3600,
				GrowthStrategy:    GrowthStrategyFixed,
				RecycleStrategy:   RecycleStrategyImmediate,
				OverlimitStrategy: OverlimitStrategyReject,
			},
		},
		User:     map[QuotaType]QuotaPolicy{},
		Contract: map[QuotaType]QuotaPolicy{},
		Engine:   map[types.EngineType]map[QuotaType]QuotaPolicy{},
		Priority: QuotaPriorityPolicy{
			PriorityOrder: []string{"global", "engine", "contract", "user"},
			PriorityWeights: map[string]float64{
				"global":   1.0,
				"engine":   0.8,
				"contract": 0.6,
				"user":     0.4,
			},
			EnablePreemption: false,
		},
	}
}

// NewQuotaUsageStats 创建配额使用统计
func NewQuotaUsageStats() *QuotaUsageStats {
	return &QuotaUsageStats{
		GlobalStats:   make(map[QuotaType]*QuotaTypeStat),
		UserStats:     make(map[string]map[QuotaType]*QuotaTypeStat),
		ContractStats: make(map[string]map[QuotaType]*QuotaTypeStat),
		EngineStats:   make(map[types.EngineType]map[QuotaType]*QuotaTypeStat),
		LastUpdated:   time.Now(),
	}
}

// CheckQuota 检查配额是否充足
func (qm *QuotaManager) CheckQuota(ctx context.Context, params types.ExecutionParams) (*QuotaAllocation, error) {
	// 配额管理始终启用（自运行节点的资源保护）
	// if !qm.config.EnableQuotaManagement { return nil, nil }

	// 固定智能超时：3秒，平衡检查效率与系统响应
	timeout := 3 * time.Second
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 提取配额需求
	requirements, err := qm.extractQuotaRequirements(params)
	if err != nil {
		return nil, fmt.Errorf("failed to extract quota requirements: %w", err)
	}

	// 创建配额分配
	allocation := &QuotaAllocation{
		AllocationID:    qm.generateAllocationID(),
		AllocatedAt:     time.Now(),
		ExecutionParams: params,
		AllocatedQuotas: make(map[QuotaType]uint64),
		UsedQuotas:      make(map[QuotaType]uint64),
		Status:          AllocationStatusAllocated,
		ExpiresAt:       time.Now().Add(time.Duration(params.Timeout) * time.Millisecond),
	}

	// 按优先级检查各级配额
	for _, quotaType := range []QuotaType{QuotaTypeExecutionTime, QuotaTypeMemory, QuotaTypeResource} {
		if required, exists := requirements[quotaType]; exists {
			if err := qm.checkAndAllocateQuota(checkCtx, allocation, quotaType, required); err != nil {
				// 回滚已分配的配额
				qm.rollbackAllocation(allocation)

				// 记录违限
				violation := QuotaViolation{
					ViolationID:   qm.generateViolationID(),
					ViolationType: "quota_exceeded",
					QuotaType:     quotaType,
					Requested:     required,
					Available:     qm.getAvailableQuota(quotaType, params),
					Severity:      "high",
					Context: map[string]interface{}{
						"allocation_id": allocation.AllocationID,
						"caller":        params.Caller,
						"contract_addr": params.ContractAddr,
					},
					Timestamp: time.Now().Unix(),
					Action:    "execution_rejected",
				}
				qm.recordViolation(violation)

				return nil, fmt.Errorf("quota check failed for %s: %w", quotaType, err)
			}
		}
	}

	// 注册活跃分配
	qm.mutex.Lock()
	qm.activeAllocations[allocation.AllocationID] = allocation
	qm.mutex.Unlock()

	// 发射配额分配审计事件
	qm.auditEmitter.EmitSecurityEvent(SecurityAuditEvent{
		EventType: "quota_allocated",
		Severity:  "low",
		Timestamp: time.Now(),
		Caller:    params.Caller,
		Action:    "quota_allocation",
		Result:    "success",
	})

	return allocation, nil
}

// ConsumeQuota 消费配额
func (qm *QuotaManager) ConsumeQuota(allocationID string, quotaType QuotaType, amount uint64) error {
	qm.mutex.RLock()
	allocation, exists := qm.activeAllocations[allocationID]
	qm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("allocation %s not found", allocationID)
	}

	// 检查是否超出已分配的配额
	allocated, ok := allocation.AllocatedQuotas[quotaType]
	if !ok {
		return fmt.Errorf("quota type %s not allocated", quotaType)
	}

	currentUsed := allocation.UsedQuotas[quotaType]
	if currentUsed+amount > allocated {
		return fmt.Errorf("quota consumption exceeds allocated amount: used=%d + request=%d > allocated=%d",
			currentUsed, amount, allocated)
	}

	// 更新使用量
	allocation.UsedQuotas[quotaType] = currentUsed + amount
	allocation.Status = AllocationStatusUsing

	// 更新统计
	qm.updateUsageStats(allocation.ExecutionParams, quotaType, amount)

	return nil
}

// ReleaseQuota 释放配额
func (qm *QuotaManager) ReleaseQuota(allocationID string) error {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	allocation, exists := qm.activeAllocations[allocationID]
	if !exists {
		return fmt.Errorf("allocation %s not found", allocationID)
	}

	// 回收未使用的配额
	for quotaType, allocated := range allocation.AllocatedQuotas {
		used := allocation.UsedQuotas[quotaType]
		if used < allocated {
			unused := allocated - used
			qm.recycleQuota(quotaType, unused, allocation.ExecutionParams)
		}
	}

	allocation.Status = AllocationStatusCompleted
	delete(qm.activeAllocations, allocationID)

	// 发射配额释放审计事件
	qm.auditEmitter.EmitSecurityEvent(SecurityAuditEvent{
		EventType: "quota_released",
		Severity:  "low",
		Timestamp: time.Now(),
		Caller:    allocationID,
		Action:    "quota_release",
		Result:    "success",
	})

	return nil
}

// 内部辅助方法

// initializeGlobalQuotas 初始化全局配额池
func (qm *QuotaManager) initializeGlobalQuotas() {
	for quotaType, policy := range qm.policies.Global {
		pool := &QuotaPool{
			Type:             quotaType,
			Total:            policy.InitialQuota,
			Used:             0,
			Reserved:         0,
			RefreshPeriodSec: policy.RefreshPeriodSec,
			LastRefresh:      time.Now(),
			Status:           QuotaPoolStatusActive,
		}
		qm.globalQuotas[quotaType] = pool
	}
}

// extractQuotaRequirements 提取配额需求
func (qm *QuotaManager) extractQuotaRequirements(params types.ExecutionParams) (map[QuotaType]uint64, error) {
	requirements := make(map[QuotaType]uint64)

	// 执行时间配额
	requirements[QuotaTypeExecutionTime] = uint64(params.Timeout)

	// 内存配额
	requirements[QuotaTypeMemory] = uint64(params.MemoryLimit)

	// 资源配额
	requirements[QuotaTypeResource] = params.ExecutionFeeLimit

	return requirements, nil
}

// checkAndAllocateQuota 检查并分配配额
func (qm *QuotaManager) checkAndAllocateQuota(ctx context.Context, allocation *QuotaAllocation, quotaType QuotaType, required uint64) error {
	// 检查全局配额
	if err := qm.checkGlobalQuota(quotaType, required); err != nil {
		return fmt.Errorf("global quota check failed: %w", err)
	}

	// 检查其他级别配额（用户、合约、引擎）
	// 用户级配额检查（自运行节点始终启用，防止滥用）
	if true { // 原：qm.config.EnableUserQuotas
		if err := qm.checkUserQuota(allocation.ExecutionParams.Caller, quotaType, required); err != nil {
			return fmt.Errorf("user quota check failed: %w", err)
		}
	}

	// 合约级配额检查（自运行节点始终启用，防止恶意合约）
	if true { // 原：qm.config.EnableContractQuotas
		if err := qm.checkContractQuota(allocation.ExecutionParams.ContractAddr, quotaType, required); err != nil {
			return fmt.Errorf("contract quota check failed: %w", err)
		}
	}

	// 引擎级配额检查（自运行节点始终启用，保证引擎公平）
	if true { // 原：qm.config.EnableEngineQuotas
		engineType, _ := qm.extractEngineType(allocation.ExecutionParams)
		if err := qm.checkEngineQuota(engineType, quotaType, required); err != nil {
			return fmt.Errorf("engine quota check failed: %w", err)
		}
	}

	// 分配配额
	qm.allocateQuota(quotaType, required, allocation.ExecutionParams)
	allocation.AllocatedQuotas[quotaType] = required
	allocation.UsedQuotas[quotaType] = 0

	return nil
}

// checkGlobalQuota 检查全局配额
func (qm *QuotaManager) checkGlobalQuota(quotaType QuotaType, required uint64) error {
	pool, exists := qm.globalQuotas[quotaType]
	if !exists {
		return fmt.Errorf("global quota pool for %s not found", quotaType)
	}

	pool.mutex.RLock()
	available := pool.Total - pool.Used - pool.Reserved
	pool.mutex.RUnlock()

	if required > available {
		return fmt.Errorf("insufficient global quota: required=%d, available=%d", required, available)
	}

	return nil
}

// checkUserQuota 检查用户配额
// 在MVP简化版本中，用户级配额检查被简化为全局配额检查
// 所有用户共享全局配额池，避免复杂的用户级配额管理
func (qm *QuotaManager) checkUserQuota(userAddr string, quotaType QuotaType, required uint64) error {
	// MVP简化策略：用户配额检查委托给全局配额检查
	// 这确保了基础资源保护，同时避免了复杂的用户级配额追踪
	// 注意：userAddr参数保留用于日志记录和未来扩展，当前版本中未使用
	_ = userAddr // 标记参数已知但未使用，避免编译器警告
	return qm.checkGlobalQuota(quotaType, required)
}

// checkContractQuota 检查合约配额
// 在MVP简化版本中，合约级配额检查被简化为全局配额检查
// 所有合约共享全局配额池，避免复杂的合约级配额管理
func (qm *QuotaManager) checkContractQuota(contractAddr string, quotaType QuotaType, required uint64) error {
	// MVP简化策略：合约配额检查委托给全局配额检查
	// 这确保了基础资源保护，同时避免了复杂的合约级配额追踪
	// 注意：contractAddr参数保留用于日志记录和未来扩展，当前版本中未使用
	_ = contractAddr // 标记参数已知但未使用，避免编译器警告
	return qm.checkGlobalQuota(quotaType, required)
}

// checkEngineQuota 检查引擎配额
// 在MVP简化版本中，引擎级配额检查被简化为全局配额检查
// 所有引擎共享全局配额池，避免复杂的引擎级配额管理
func (qm *QuotaManager) checkEngineQuota(engineType types.EngineType, quotaType QuotaType, required uint64) error {
	// MVP简化策略：引擎配额检查委托给全局配额检查
	// 这确保了基础资源保护，同时避免了复杂的引擎级配额追踪
	// 所有引擎类型（WASM、ONNX等）使用统一的资源限制
	// 注意：engineType参数保留用于日志记录和未来扩展，当前版本中未使用
	_ = engineType // 标记参数已知但未使用，避免编译器警告
	return qm.checkGlobalQuota(quotaType, required)
}

// allocateQuota 分配配额
func (qm *QuotaManager) allocateQuota(quotaType QuotaType, amount uint64, params types.ExecutionParams) {
	// 从全局配额池分配
	// 注意：params参数保留用于日志记录和未来扩展，当前版本中未使用
	_ = params // 标记参数已知但未使用，避免编译器警告

	if pool, exists := qm.globalQuotas[quotaType]; exists {
		pool.mutex.Lock()
		pool.Used += amount
		qm.updatePoolStatus(pool)
		pool.mutex.Unlock()
	}
}

// recycleQuota 回收配额
func (qm *QuotaManager) recycleQuota(quotaType QuotaType, amount uint64, params types.ExecutionParams) {
	// 回收到全局配额池
	// 注意：params参数保留用于日志记录和未来扩展，当前版本中未使用
	_ = params // 标记参数已知但未使用，避免编译器警告

	if pool, exists := qm.globalQuotas[quotaType]; exists {
		pool.mutex.Lock()
		if pool.Used >= amount {
			pool.Used -= amount
		}
		qm.updatePoolStatus(pool)
		pool.mutex.Unlock()
	}
}

// updatePoolStatus 更新配额池状态
func (qm *QuotaManager) updatePoolStatus(pool *QuotaPool) {
	usagePercent := float64(pool.Used+pool.Reserved) / float64(pool.Total) * 100

	// 智能阈值算法：95%严重，80%警告
	if usagePercent >= 95.0 { // 原：qm.config.CriticalThresholdPercent
		pool.Status = QuotaPoolStatusCritical
	} else if usagePercent >= 80.0 { // 原：qm.config.WarningThresholdPercent
		pool.Status = QuotaPoolStatusWarning
	} else {
		pool.Status = QuotaPoolStatusActive
	}

	// 发射配额状态变化事件
	if pool.Status != QuotaPoolStatusActive {
		qm.auditEmitter.EmitSecurityEvent(SecurityAuditEvent{
			EventType: "quota_threshold_exceeded",
			Severity:  "high",
			Timestamp: time.Now(),
			Caller:    "system",
			Action:    "quota_monitoring",
			Result:    "threshold_exceeded",
		})
	}
}

// getAvailableQuota 获取可用配额
func (qm *QuotaManager) getAvailableQuota(quotaType QuotaType, params types.ExecutionParams) uint64 {
	// 注意：params参数保留用于日志记录和未来扩展，当前版本中未使用
	_ = params // 标记参数已知但未使用，避免编译器警告

	if pool, exists := qm.globalQuotas[quotaType]; exists {
		pool.mutex.RLock()
		available := pool.Total - pool.Used - pool.Reserved
		pool.mutex.RUnlock()
		return available
	}
	return 0
}

// rollbackAllocation 回滚配额分配
func (qm *QuotaManager) rollbackAllocation(allocation *QuotaAllocation) {
	for quotaType, amount := range allocation.AllocatedQuotas {
		qm.recycleQuota(quotaType, amount, allocation.ExecutionParams)
	}
	allocation.Status = AllocationStatusCancelled
}

// recordViolation 记录配额违限
func (qm *QuotaManager) recordViolation(violation QuotaViolation) {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	// 添加到违限日志
	// 智能日志管理：固定保留1000条违限记录
	if len(qm.limitViolations) >= 1000 { // 原：qm.config.ViolationLogSize
		qm.limitViolations = qm.limitViolations[1:]
	}
	qm.limitViolations = append(qm.limitViolations, violation)

	// 发射审计事件
	qm.auditEmitter.EmitSecurityEvent(SecurityAuditEvent{
		EventType: "quota_violation",
		Severity:  "critical",
		Timestamp: time.Now(),
		Caller:    "system",
		Action:    "quota_violation",
		Result:    "denied",
	})
}

// updateUsageStats 更新使用统计
func (qm *QuotaManager) updateUsageStats(params types.ExecutionParams, quotaType QuotaType, amount uint64) {
	qm.usageStats.mutex.Lock()
	defer qm.usageStats.mutex.Unlock()

	// 更新全局统计
	if stat, exists := qm.usageStats.GlobalStats[quotaType]; exists {
		stat.TotalUsed += amount
		stat.RequestCount++
		if amount > stat.PeakUsage {
			stat.PeakUsage = amount
		}
		stat.AverageUsage = float64(stat.TotalUsed) / float64(stat.RequestCount)
		stat.LastUsed = time.Now()
	} else {
		qm.usageStats.GlobalStats[quotaType] = &QuotaTypeStat{
			TotalUsed:    amount,
			PeakUsage:    amount,
			AverageUsage: float64(amount),
			RequestCount: 1,
			LastUsed:     time.Now(),
		}
	}

	qm.usageStats.LastUpdated = time.Now()
}

// extractEngineType 从执行参数中提取引擎类型
func (qm *QuotaManager) extractEngineType(params types.ExecutionParams) (types.EngineType, error) {
	if engineTypeVal, exists := params.Context["engine_type"]; exists {
		if engineTypeStr, ok := engineTypeVal.(string); ok {
			return types.EngineType(engineTypeStr), nil
		}
	}
	return types.EngineTypeWASM, nil // 默认WASM
}

// generateAllocationID 生成分配ID
func (qm *QuotaManager) generateAllocationID() string {
	return fmt.Sprintf("quota_alloc_%d", time.Now().UnixNano())
}

// generateViolationID 生成违限ID
func (qm *QuotaManager) generateViolationID() string {
	return fmt.Sprintf("quota_violation_%d", time.Now().UnixNano())
}

// GetQuotaStats 获取配额统计信息
func (qm *QuotaManager) GetQuotaStats() *QuotaUsageStats {
	qm.usageStats.mutex.RLock()
	defer qm.usageStats.mutex.RUnlock()

	// 返回统计数据的副本（避免锁拷贝问题）
	return &QuotaUsageStats{
		GlobalStats:   qm.usageStats.GlobalStats,
		UserStats:     qm.usageStats.UserStats,
		ContractStats: qm.usageStats.ContractStats,
		EngineStats:   qm.usageStats.EngineStats,
		LastUpdated:   qm.usageStats.LastUpdated,
	}
}

// GetActiveAllocations 获取活跃的配额分配
func (qm *QuotaManager) GetActiveAllocations() map[string]*QuotaAllocation {
	qm.mutex.RLock()
	defer qm.mutex.RUnlock()

	// 返回分配数据的副本
	allocationsCopy := make(map[string]*QuotaAllocation)
	for id, allocation := range qm.activeAllocations {
		allocCopy := *allocation
		allocationsCopy[id] = &allocCopy
	}
	return allocationsCopy
}

// CleanupExpiredAllocations 清理过期的配额分配
func (qm *QuotaManager) CleanupExpiredAllocations() {
	qm.mutex.Lock()
	defer qm.mutex.Unlock()

	now := time.Now()
	for id, allocation := range qm.activeAllocations {
		if now.After(allocation.ExpiresAt) {
			// 回收过期分配的配额
			qm.rollbackAllocation(allocation)
			allocation.Status = AllocationStatusExpired
			delete(qm.activeAllocations, id)
		}
	}
}
