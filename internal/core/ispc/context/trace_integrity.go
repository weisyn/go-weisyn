package context

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// TraceIntegrityChecker 轨迹完整性检查器
//
// 🎯 **完整性保证**：
// - 轨迹记录验证：验证记录的轨迹是否符合预期格式和完整性
// - 轨迹完整性检查：检查轨迹是否完整（时间顺序、调用顺序等）
// - 轨迹回放机制：能够回放轨迹用于调试
type TraceIntegrityChecker struct {
	// 轨迹验证规则
	validationRules []TraceValidationRule

	// 轨迹完整性检查配置
	checkConfig *TraceIntegrityCheckConfig

	// 轨迹回放记录（用于调试）
	replayRecords    []TraceReplayRecord
	replayMutex      sync.RWMutex
	maxReplayRecords int
}

// TraceValidationRule 轨迹验证规则
type TraceValidationRule struct {
	Name        string
	Description string
	Validate    func(trace *ExecutionTrace) error
}

// TraceIntegrityCheckConfig 轨迹完整性检查配置
type TraceIntegrityCheckConfig struct {
	// 检查时间顺序
	CheckTimeOrder bool
	// 检查调用顺序
	CheckCallOrder bool
	// 检查状态变更一致性
	CheckStateConsistency bool
	// 检查轨迹哈希
	CheckTraceHash bool
	// 允许的最大时间间隔（用于检测异常）
	MaxTimeGap time.Duration
}

// DefaultTraceIntegrityCheckConfig 默认轨迹完整性检查配置
func DefaultTraceIntegrityCheckConfig() *TraceIntegrityCheckConfig {
	return &TraceIntegrityCheckConfig{
		CheckTimeOrder:        true,
		CheckCallOrder:        true,
		CheckStateConsistency: true,
		CheckTraceHash:        true,
		MaxTimeGap:            1 * time.Hour, // 最大允许1小时的时间间隔
	}
}

// TraceReplayRecord 轨迹回放记录
type TraceReplayRecord struct {
	ExecutionID string
	Trace       *ExecutionTrace
	RecordedAt  time.Time
	ReplayCount int
}

// NewTraceIntegrityChecker 创建轨迹完整性检查器
func NewTraceIntegrityChecker(config *TraceIntegrityCheckConfig) *TraceIntegrityChecker {
	if config == nil {
		config = DefaultTraceIntegrityCheckConfig()
	}

	checker := &TraceIntegrityChecker{
		validationRules:  make([]TraceValidationRule, 0),
		checkConfig:      config,
		replayRecords:    make([]TraceReplayRecord, 0),
		maxReplayRecords: 100, // 最多保存100条回放记录
	}

	// 注册默认验证规则
	checker.registerDefaultRules()

	return checker
}

// registerDefaultRules 注册默认验证规则
func (c *TraceIntegrityChecker) registerDefaultRules() {
	// 规则1：检查执行ID是否存在
	c.validationRules = append(c.validationRules, TraceValidationRule{
		Name:        "execution_id_check",
		Description: "检查执行ID是否存在",
		Validate: func(trace *ExecutionTrace) error {
			if trace.ExecutionID == "" {
				return fmt.Errorf("执行ID为空")
			}
			return nil
		},
	})

	// 规则2：检查时间范围是否有效
	c.validationRules = append(c.validationRules, TraceValidationRule{
		Name:        "time_range_check",
		Description: "检查时间范围是否有效",
		Validate: func(trace *ExecutionTrace) error {
			if trace.StartTime.IsZero() {
				return fmt.Errorf("开始时间为空")
			}
			if trace.EndTime.IsZero() {
				return fmt.Errorf("结束时间为空")
			}
			if trace.EndTime.Before(trace.StartTime) {
				return fmt.Errorf("结束时间早于开始时间")
			}
			return nil
		},
	})

	// 规则3：检查总执行时间是否合理
	c.validationRules = append(c.validationRules, TraceValidationRule{
		Name:        "duration_check",
		Description: "检查总执行时间是否合理",
		Validate: func(trace *ExecutionTrace) error {
			actualDuration := trace.EndTime.Sub(trace.StartTime)
			if trace.TotalDuration != 0 && actualDuration != trace.TotalDuration {
				// 允许小的误差（1秒）
				diff := actualDuration - trace.TotalDuration
				if diff < 0 {
					diff = -diff
				}
				if diff > 1*time.Second {
					return fmt.Errorf("总执行时间不匹配: 实际=%v, 记录=%v", actualDuration, trace.TotalDuration)
				}
			}
			return nil
		},
	})
}

// RegisterValidationRule 注册自定义验证规则
func (c *TraceIntegrityChecker) RegisterValidationRule(rule TraceValidationRule) {
	c.validationRules = append(c.validationRules, rule)
}

// ValidateTrace 验证轨迹记录
//
// 🎯 **轨迹记录验证**：
// - 验证轨迹是否符合预期格式
// - 验证轨迹的完整性
// - 返回所有验证错误
func (c *TraceIntegrityChecker) ValidateTrace(trace *ExecutionTrace) []error {
	if trace == nil {
		return []error{fmt.Errorf("轨迹为空")}
	}

	var errors []error

	// 执行所有验证规则
	for _, rule := range c.validationRules {
		if err := rule.Validate(trace); err != nil {
			errors = append(errors, fmt.Errorf("验证规则[%s]失败: %w", rule.Name, err))
		}
	}

	return errors
}

// CheckIntegrity 检查轨迹完整性
//
// 🎯 **轨迹完整性检查**：
// - 检查时间顺序
// - 检查调用顺序
// - 检查状态变更一致性
// - 检查轨迹哈希
func (c *TraceIntegrityChecker) CheckIntegrity(trace *ExecutionTrace) (*IntegrityCheckResult, error) {
	if trace == nil {
		return nil, fmt.Errorf("轨迹为空")
	}

	result := &IntegrityCheckResult{
		IsValid:          true,
		Issues:           make([]string, 0),
		HostCallCount:    len(trace.HostFunctionCalls),
		StateChangeCount: len(trace.StateChanges),
		EventCount:       len(trace.ExecutionEvents),
	}

	// 1. 检查时间顺序
	if c.checkConfig.CheckTimeOrder {
		if err := c.checkTimeOrder(trace); err != nil {
			result.IsValid = false
			result.Issues = append(result.Issues, fmt.Sprintf("时间顺序检查失败: %v", err))
		} else {
			result.TimeOrderValid = true
		}
	}

	// 2. 检查调用顺序
	if c.checkConfig.CheckCallOrder {
		if err := c.checkCallOrder(trace); err != nil {
			result.IsValid = false
			result.Issues = append(result.Issues, fmt.Sprintf("调用顺序检查失败: %v", err))
		} else {
			result.CallOrderValid = true
		}
	}

	// 3. 检查状态变更一致性
	if c.checkConfig.CheckStateConsistency {
		if err := c.checkStateConsistency(trace); err != nil {
			result.IsValid = false
			result.Issues = append(result.Issues, fmt.Sprintf("状态一致性检查失败: %v", err))
		} else {
			result.StateConsistent = true
		}
	}

	// 4. 检查轨迹哈希
	if c.checkConfig.CheckTraceHash {
		expectedHash := c.computeTraceHash(trace)
		result.TraceHash = expectedHash
		// 如果轨迹有哈希字段，进行比较
		// 注意：当前ExecutionTrace结构中没有哈希字段，这里仅计算并记录
	}

	return result, nil
}

// IntegrityCheckResult 完整性检查结果
type IntegrityCheckResult struct {
	IsValid          bool
	Issues           []string
	HostCallCount    int
	StateChangeCount int
	EventCount       int
	TraceHash        []byte
	TimeOrderValid   bool
	CallOrderValid   bool
	StateConsistent  bool
}

// checkTimeOrder 检查时间顺序
func (c *TraceIntegrityChecker) checkTimeOrder(trace *ExecutionTrace) error {
	// 检查宿主函数调用的时间顺序
	for i := 1; i < len(trace.HostFunctionCalls); i++ {
		prev := trace.HostFunctionCalls[i-1]
		curr := trace.HostFunctionCalls[i]

		if curr.Timestamp.Before(prev.Timestamp) {
			return fmt.Errorf("宿主函数调用时间顺序错误: 调用[%d]时间(%v)早于调用[%d]时间(%v)",
				i, curr.Timestamp, i-1, prev.Timestamp)
		}

		// 检查时间间隔是否异常
		gap := curr.Timestamp.Sub(prev.Timestamp)
		if gap > c.checkConfig.MaxTimeGap {
			return fmt.Errorf("宿主函数调用时间间隔异常: 调用[%d]与调用[%d]间隔=%v, 超过最大允许间隔=%v",
				i, i-1, gap, c.checkConfig.MaxTimeGap)
		}
	}

	// 检查状态变更的时间顺序
	for i := 1; i < len(trace.StateChanges); i++ {
		prev := trace.StateChanges[i-1]
		curr := trace.StateChanges[i]

		if curr.Timestamp.Before(prev.Timestamp) {
			return fmt.Errorf("状态变更时间顺序错误: 变更[%d]时间(%v)早于变更[%d]时间(%v)",
				i, curr.Timestamp, i-1, prev.Timestamp)
		}
	}

	// 检查所有操作是否在开始时间和结束时间之间
	for _, call := range trace.HostFunctionCalls {
		if call.Timestamp.Before(trace.StartTime) || call.Timestamp.After(trace.EndTime) {
			return fmt.Errorf("宿主函数调用时间超出执行时间范围: 调用时间=%v, 执行范围=[%v, %v]",
				call.Timestamp, trace.StartTime, trace.EndTime)
		}
	}

	for _, change := range trace.StateChanges {
		if change.Timestamp.Before(trace.StartTime) || change.Timestamp.After(trace.EndTime) {
			return fmt.Errorf("状态变更时间超出执行时间范围: 变更时间=%v, 执行范围=[%v, %v]",
				change.Timestamp, trace.StartTime, trace.EndTime)
		}
	}

	return nil
}

// checkCallOrder 检查调用顺序
func (c *TraceIntegrityChecker) checkCallOrder(trace *ExecutionTrace) error {
	// 检查宿主函数调用的顺序是否合理
	// 确保Sequence顺序与索引顺序一致，并且Sequence是递增的

	if len(trace.HostFunctionCalls) <= 1 {
		return nil // 0个或1个调用，无需检查顺序
	}

	for i := 1; i < len(trace.HostFunctionCalls); i++ {
		prev := trace.HostFunctionCalls[i-1]
		curr := trace.HostFunctionCalls[i]

		// 1. 检查Sequence顺序：Sequence必须严格递增
		if curr.Sequence <= prev.Sequence {
			return fmt.Errorf("调用顺序错误: 调用[%d]的Sequence(%d)应该大于调用[%d]的Sequence(%d)",
				i, curr.Sequence, i-1, prev.Sequence)
		}

		// 2. 检查时间戳与Sequence的一致性
		// 使用Equal方法比较时间戳（处理时区等情况）
		isEqual := curr.Timestamp.Equal(prev.Timestamp)
		if isEqual {
			// 时间戳相同的情况下，Sequence必须严格递增（已在上面检查）
			// 相同时间戳的调用应该按照Sequence顺序排列
			// 这是合理的，因为同一时刻可能有多个调用（例如并发调用）
		} else if curr.Timestamp.Before(prev.Timestamp) {
			// 时间戳顺序错误，但Sequence顺序正确，这是异常情况
			// 可能是时间戳设置错误，但Sequence是正确的
			// 这里返回错误，因为时间戳和Sequence应该保持一致
			// 注意：虽然checkTimeOrder也会检查时间戳顺序，但这里从调用顺序角度检查更严格
			return fmt.Errorf("调用顺序不一致: 调用[%d]的时间戳(%v)早于调用[%d]的时间戳(%v)，但Sequence(%d)大于Sequence(%d)，时间戳与Sequence不一致",
				i, curr.Timestamp, i-1, prev.Timestamp, curr.Sequence, prev.Sequence)
		}
		// 如果时间戳递增，Sequence也递增，这是正常情况，无需额外检查
	}

	return nil
}

// checkStateConsistency 检查状态变更一致性
func (c *TraceIntegrityChecker) checkStateConsistency(trace *ExecutionTrace) error {
	// 检查状态变更的一致性
	// 例如：创建UTXO后不能立即删除，必须先创建后使用等

	stateMap := make(map[string]*StateChange) // key -> 最新的状态变更

	for i := range trace.StateChanges {
		change := &trace.StateChanges[i]
		prevChange, exists := stateMap[change.Key]

		if exists {
			// 检查状态变更的合理性
			// 例如：如果之前是"create"，现在不能是"create"（重复创建）
			if prevChange.Type == "utxo_create" && change.Type == "utxo_create" {
				return fmt.Errorf("状态变更不一致: 键[%s]重复创建", change.Key)
			}
			if prevChange.Type == "utxo_spend" && change.Type == "utxo_spend" {
				return fmt.Errorf("状态变更不一致: 键[%s]重复花费", change.Key)
			}
		}

		stateMap[change.Key] = change
	}

	return nil
}

// computeTraceHash 计算轨迹哈希
func (c *TraceIntegrityChecker) computeTraceHash(trace *ExecutionTrace) []byte {
	// 序列化轨迹数据
	traceData, err := json.Marshal(trace)
	if err != nil {
		// 如果序列化失败，使用简化方法
		return c.computeTraceHashSimple(trace)
	}

	// 计算SHA-256哈希
	hash := sha256.Sum256(traceData)
	return hash[:]
}

// computeTraceHashSimple 计算轨迹哈希（简化方法）
func (c *TraceIntegrityChecker) computeTraceHashSimple(trace *ExecutionTrace) []byte {
	h := sha256.New()

	// 添加执行ID
	h.Write([]byte(trace.ExecutionID))

	// 添加时间戳
	startBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(startBytes, uint64(trace.StartTime.UnixNano()))
	h.Write(startBytes)

	endBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(endBytes, uint64(trace.EndTime.UnixNano()))
	h.Write(endBytes)

	// 添加宿主函数调用数量
	countBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(countBytes, uint32(len(trace.HostFunctionCalls)))
	h.Write(countBytes)

	// 添加状态变更数量
	binary.BigEndian.PutUint32(countBytes, uint32(len(trace.StateChanges)))
	h.Write(countBytes)

	// 添加每个宿主函数调用的函数名和时间戳
	for _, call := range trace.HostFunctionCalls {
		h.Write([]byte(call.FunctionName))
		tsBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(tsBytes, uint64(call.Timestamp.UnixNano()))
		h.Write(tsBytes)
	}

	// 添加每个状态变更的类型和键
	for _, change := range trace.StateChanges {
		h.Write([]byte(change.Type))
		h.Write([]byte(change.Key))
		tsBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(tsBytes, uint64(change.Timestamp.UnixNano()))
		h.Write(tsBytes)
	}

	return h.Sum(nil)
}

// RecordTraceForReplay 记录轨迹用于回放
//
// 🎯 **轨迹回放机制**：
// - 记录轨迹用于后续回放
// - 用于调试和问题排查
func (c *TraceIntegrityChecker) RecordTraceForReplay(executionID string, trace *ExecutionTrace) {
	c.replayMutex.Lock()
	defer c.replayMutex.Unlock()

	replayRecord := TraceReplayRecord{
		ExecutionID: executionID,
		Trace:       trace,
		RecordedAt:  time.Now(),
		ReplayCount: 0,
	}

	c.replayRecords = append(c.replayRecords, replayRecord)

	// 限制回放记录数量（FIFO）
	if len(c.replayRecords) > c.maxReplayRecords {
		c.replayRecords = c.replayRecords[1:]
	}
}

// ReplayTrace 回放轨迹
//
// 🎯 **轨迹回放**：
// - 按照时间顺序回放轨迹
// - 用于调试和问题排查
func (c *TraceIntegrityChecker) ReplayTrace(executionID string, handler TraceReplayHandler) error {
	c.replayMutex.RLock()
	defer c.replayMutex.RUnlock()

	// 查找对应的轨迹记录
	var targetRecord *TraceReplayRecord
	for i := range c.replayRecords {
		if c.replayRecords[i].ExecutionID == executionID {
			targetRecord = &c.replayRecords[i]
			break
		}
	}

	if targetRecord == nil {
		return fmt.Errorf("未找到执行ID[%s]的轨迹记录", executionID)
	}

	trace := targetRecord.Trace

	// 按照时间顺序排序所有操作
	operations := make([]TraceOperation, 0)

	// 添加宿主函数调用
	for _, call := range trace.HostFunctionCalls {
		operations = append(operations, TraceOperation{
			Type:      "host_function_call",
			Timestamp: call.Timestamp,
			Data:      call,
		})
	}

	// 添加状态变更
	for _, change := range trace.StateChanges {
		operations = append(operations, TraceOperation{
			Type:      "state_change",
			Timestamp: change.Timestamp,
			Data:      change,
		})
	}

	// 添加执行事件
	for _, event := range trace.ExecutionEvents {
		operations = append(operations, TraceOperation{
			Type:      "execution_event",
			Timestamp: event.Timestamp,
			Data:      event,
		})
	}

	// 按时间戳排序
	sort.Slice(operations, func(i, j int) bool {
		return operations[i].Timestamp.Before(operations[j].Timestamp)
	})

	// 回放操作
	for _, op := range operations {
		if err := handler.HandleOperation(op); err != nil {
			return fmt.Errorf("回放操作失败: %w", err)
		}
	}

	// 更新回放计数
	c.replayMutex.RUnlock()
	c.replayMutex.Lock()
	targetRecord.ReplayCount++
	c.replayMutex.Unlock()
	c.replayMutex.RLock()

	return nil
}

// TraceOperation 轨迹操作（用于回放）
type TraceOperation struct {
	Type      string
	Timestamp time.Time
	Data      interface{}
}

// TraceReplayHandler 轨迹回放处理器接口
type TraceReplayHandler interface {
	HandleOperation(op TraceOperation) error
}

// GetReplayRecords 获取回放记录列表
func (c *TraceIntegrityChecker) GetReplayRecords() []TraceReplayRecord {
	c.replayMutex.RLock()
	defer c.replayMutex.RUnlock()

	result := make([]TraceReplayRecord, len(c.replayRecords))
	copy(result, c.replayRecords)
	return result
}

// ClearReplayRecords 清空回放记录
func (c *TraceIntegrityChecker) ClearReplayRecords() {
	c.replayMutex.Lock()
	defer c.replayMutex.Unlock()

	c.replayRecords = make([]TraceReplayRecord, 0)
}
