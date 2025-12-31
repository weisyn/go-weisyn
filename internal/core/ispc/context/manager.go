package context

import (
	"context"
	"crypto/sha256"
	"fmt"
	"runtime"
	"sync"
	"time"

	// 公共接口依赖
	"github.com/weisyn/v1/pkg/interfaces/config"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"

	// 基础设施接口依赖
	ispcInterfaces "github.com/weisyn/v1/internal/core/ispc/interfaces"
	infraClock "github.com/weisyn/v1/pkg/interfaces/infrastructure/clock"
	"github.com/weisyn/v1/pkg/types"
	"google.golang.org/protobuf/proto"
)

// ==================== 执行轨迹相关结构体 ====================

// HostFunctionCall 宿主函数调用记录
type HostFunctionCall struct {
	Sequence     uint64        // 序号（调用顺序）
	FunctionName string        // 函数名
	Parameters   interface{}   // 调用参数
	Result       interface{}   // 返回结果
	Timestamp    time.Time     // 调用时间
	Duration     time.Duration // 执行耗时
	Success      bool          // 是否成功
	Error        string        // 错误信息（如果有）
}

// StateChange 状态变更记录
type StateChange struct {
	Type      string      // 变更类型（utxo_create, utxo_spend, storage_set等）
	Key       string      // 变更键值
	OldValue  interface{} // 旧值
	NewValue  interface{} // 新值
	Timestamp time.Time   // 变更时间
}

// ExecutionEvent 执行事件记录
type ExecutionEvent struct {
	EventType string      // 事件类型（contract_call, host_function_call等）
	Data      interface{} // 事件数据
	Timestamp time.Time   // 事件时间
}

// ExecutionTrace 完整的执行轨迹
type ExecutionTrace struct {
	ExecutionID       string             // 执行ID
	StartTime         time.Time          // 开始时间
	EndTime           time.Time          // 结束时间
	HostFunctionCalls []HostFunctionCall // 宿主函数调用列表
	StateChanges      []StateChange      // 状态变更列表
	ExecutionEvents   []ExecutionEvent   // 执行事件列表
	TotalDuration     time.Duration      // 总执行时间
}

// Manager 执行上下文管理器
//
// 🎯 **设计理念**：专注依赖注入和框架性实现
//
// 本管理器负责管理ISPC执行过程中的所有执行上下文，
// 通过依赖注入框架组织所有必要的基础设施服务，
// 为ISPC执行上下文管理提供统一的管理入口。
//
// 🏗️ **架构特点**：
// - 大量依赖公共接口：复用成熟的基础设施服务
// - 上下文生命周期管理：创建、存储、清理执行上下文
// - 框架性实现：专注依赖管理，暂不实现具体业务逻辑
type Manager struct {
	// ==================== 基础设施服务 ====================
	logger         log.Logger       // 日志服务
	configProvider config.Provider  // 配置提供者
	clock          infraClock.Clock // 时钟服务（确定性时间源）

	// ==================== 上下文存储 ====================
	contexts map[string]ispcInterfaces.ExecutionContext // 活跃上下文存储
	mutex    sync.RWMutex                               // 并发安全锁

	// ==================== 配置参数 ====================
	config *ContextManagerConfig

	// P0: 上下文隔离增强
	isolationEnforcer *ContextIsolationEnforcer
	cleanupVerifier   *ContextCleanupVerifier

	// P0: 确定性保证增强
	resultVerifier *ExecutionResultVerifier

	// P0: 轨迹完整性检查器
	traceIntegrityChecker *TraceIntegrityChecker

	// P1: 上下文调试器（日志和调试工具）
	debugger  *ContextDebugger
	debugTool *DebugTool

	// P0: 异步轨迹记录（异步轨迹记录优化）
	traceQueue        *LockFreeQueue   // 无锁队列
	traceWorkerPool   *TraceWorkerPool // 工作线程池
	asyncTraceEnabled bool             // 是否启用异步轨迹记录（默认false，保持向后兼容）
}

// ActiveContextCount 返回当前活跃执行上下文的数量。
// 主要用于内存监控（MemoryReporter）等非核心路径，避免在调用方拍脑袋估算。
func (m *Manager) ActiveContextCount() int64 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return int64(len(m.contexts))
}

// contextImpl 执行上下文的具体实现
//
// 实现 ispcInterfaces.ExecutionContext 接口
type contextImpl struct {
	// 基本信息
	executionID string
	createdAt   time.Time
	expiresAt   time.Time

	// 从外部 context.Context 继承的信息
	hasDeadline bool   // 是否有外部设置的超时
	traceID     string // 链路追踪ID
	userID      string // 用户身份ID
	requestID   string // 请求ID

	// 执行数据
	txDraft *ispcInterfaces.TransactionDraft

	// （已移除）旧版 Services 兼容字段
	// 原字段：services HostRuntimeServices

	// 🔧 引擎无关宿主能力接口（v1.0 新增）
	// 在执行前由 ISPC Coordinator 注入，统一 WASM/ONNX 等执行引擎的宿主能力
	hostABI ispcInterfaces.HostABI

	// 执行轨迹记录（新增）
	hostFunctionCalls []HostFunctionCall // 宿主函数调用记录
	stateChanges      []StateChange      // 状态变更记录
	executionEvents   []ExecutionEvent   // 执行事件记录

	// 业务数据（新增）
	returnData      []byte                  // 业务返回数据（通过set_return_data设置）
	events          []*ispcInterfaces.Event // 事件列表（通过emit_event发射）
	initParams      []byte                  // 合约调用参数（init params，JSON/二进制负载）
	contractAddress []byte                  // 合约地址（v1.0 新增，用于创建合约代币）
	callerAddress   []byte                  // 调用者地址（v1.0 新增，用于权限检查）
	stateBefore     []byte                  // 执行前状态哈希
	stateAfter      []byte                  // 执行后状态哈希

	// 管理器引用（用于访问时钟等服务）
	manager *Manager

	// 同步控制
	mutex sync.RWMutex

	// P1: 执行时间测量相关
	lastCallTime time.Time // 上一个宿主函数调用的时间（用于计算Duration）

	// P0: 资源使用统计
	resourceUsage *types.ResourceUsage // 资源使用统计

	// P0: 确定性保证增强
	deterministicEnforcer *DeterministicEnforcer     // 确定性执行增强器
	randomSource          *DeterministicRandomSource // 确定性随机数源
}

// GetExecutionID 获取执行ID
//
// 📋 **参数**：无
// 🔧 **返回值**：
//   - string: 执行上下文的唯一标识符
//
// 🔒 **并发安全**：使用读锁保护，支持并发读取
// ⚠️ **注意事项**：executionID在上下文创建后不可变
func (c *contextImpl) GetExecutionID() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.executionID
}

// Services 获取执行期宿主函数服务聚合
//
// 📋 **参数**：无
// 🔧 **返回值**：
//   - pkgInterfaces.HostRuntimeServices: 执行期服务聚合（ChainReader/UTXOReader/TxReader/DraftRecorder）
//
// 🔒 **并发安全**：使用读锁保护，支持并发读取
// 🎯 **用途**：供宿主函数获取读/写能力，不在 Provider 构造期依赖 blockchain/tx
// ⚠️ **注意事项**：
//   - 必须在执行前由 ISPC Coordinator 注入（通过 SetServices 或创建时注入）
//   - 如果未注入会返回 nil，宿主函数应该检查并报错
//   - 这是断环的关键：services 不在 Provider 图中，只在运行时使用
// 已移除 Services()

// SetServices 设置执行期服务聚合（内部方法，供 Manager 使用，旧版，兼容保留）
//
// ⚠️ **弃用提示**：建议使用 SetHostABI() 注入引擎无关的宿主能力接口
//
// 📋 **参数**：
//   - services: pkgInterfaces.HostRuntimeServices - 执行期服务聚合实例
//
// 🔧 **返回值**：
//   - error: 如果 services 为 nil 则返回错误
//
// 🔒 **并发安全**：使用写锁保护，确保原子更新
// 🎯 **用途**：由 ISPC Coordinator 在执行前注入服务
// ⚠️ **注意事项**：
//   - 这是内部方法，不暴露在 ExecutionContext 接口中
//   - 通常在创建上下文后立即调用一次
//   - 不应在执行期间重复调用
// 已移除 SetServices()

// HostABI 获取引擎无关宿主能力接口（v1.0 新增）
//
// 📋 **参数**：无
// 🔧 **返回值**：
//   - ispcInterfaces.HostABI: 引擎无关宿主能力接口
//
// 🔒 **并发安全**：使用读锁保护，支持并发读取
// 🎯 **用途**：供 WASM/ONNX 等执行引擎获取宿主能力，统一业务语义
// ⚠️ **注意事项**：
//   - 必须在执行前由 ISPC Coordinator 注入（通过 SetHostABI）
//   - 如果未注入会返回 nil，宿主函数应该检查并报错
func (c *contextImpl) HostABI() ispcInterfaces.HostABI {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.hostABI
}

// SetHostABI 设置引擎无关宿主能力接口（内部方法，供 Manager 使用）
//
// 📋 **参数**：
//   - hostABI: ispcInterfaces.HostABI - 引擎无关宿主能力接口实例
//
// 🔧 **返回值**：
//   - error: 如果 hostABI 为 nil 则返回错误
//
// 🔒 **并发安全**：使用写锁保护，确保原子更新
// 🎯 **用途**：由 ISPC Coordinator 在执行前注入 HostABI
// ⚠️ **注意事项**：
//   - 这是内部方法，不暴露在 ExecutionContext 接口中
//   - 通常在创建上下文后立即调用一次
//   - 不应在执行期间重复调用
func (c *contextImpl) SetHostABI(hostABI ispcInterfaces.HostABI) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if hostABI == nil {
		return fmt.Errorf("cannot set nil hostABI")
	}
	c.hostABI = hostABI
	return nil
}

// GetCallerAddress 获取调用者地址（v1.0 新增）
//
// 📋 **参数**：无
// 🔧 **返回值**：
//   - []byte: 调用者地址（20字节）
//
// 🔒 **并发安全**：使用读锁保护，支持并发读取
// 🎯 **用途**：供宿主函数获取调用者地址（权限检查、所有权验证）
// ⚠️ **注意事项**：执行上下文初始化时应设置调用者地址
func (c *contextImpl) GetCallerAddress() []byte {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.callerAddress
}

// SetContractAddress 设置合约地址（v1.0 新增）
//
// 📋 **参数**：
//   - address: 20字节合约地址
//
// 🔧 **返回值**：
//   - error: 地址长度无效时返回错误
//
// 🔒 **并发安全**：使用写锁保护，确保原子更新
func (c *contextImpl) SetContractAddress(address []byte) error {
	if len(address) != 20 {
		return fmt.Errorf("contract address must be 20 bytes, got %d", len(address))
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.contractAddress == nil || len(c.contractAddress) != len(address) {
		c.contractAddress = make([]byte, len(address))
	}
	copy(c.contractAddress, address)
	return nil
}

// GetTransactionDraft 获取交易草稿
//
// 📋 **参数**：无
// 🔧 **返回值**：
//   - *ispcInterfaces.TransactionDraft: 当前执行上下文关联的交易草稿
//   - error: 如果草稿未初始化则返回错误
//
// 🔒 **并发安全**：使用读锁保护，支持并发读取
// 🎯 **用途**：供宿主函数获取可修改的交易草稿
// ⚠️ **注意事项**：
//   - 如果CreateContext时callerAddress不为空，会自动创建初始交易草稿
//   - 如果callerAddress为空，需要先调用UpdateTransactionDraft设置草稿
//   - 自动创建的草稿包含空的Transaction对象，需要后续通过UpdateTransactionDraft更新
func (c *contextImpl) GetTransactionDraft() (*ispcInterfaces.TransactionDraft, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.txDraft == nil {
		return nil, fmt.Errorf("transaction draft not initialized for execution ID: %s", c.executionID)
	}
	return c.txDraft, nil
}

// UpdateTransactionDraft 更新交易草稿
//
// 📋 **参数**：
//   - draft: *ispcInterfaces.TransactionDraft - 新的交易草稿对象
//
// 🔧 **返回值**：
//   - error: 如果draft为nil则返回错误，否则返回nil
//
// 🔒 **并发安全**：使用写锁保护，确保原子更新
// 🎯 **用途**：供前置阶段注入交易草稿，供宿主函数动态修改
// ⚠️ **注意事项**：会覆盖现有草稿，调用方需确保传入有效对象
func (c *contextImpl) UpdateTransactionDraft(draft *ispcInterfaces.TransactionDraft) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if draft == nil {
		return fmt.Errorf("cannot update with nil transaction draft")
	}
	c.txDraft = draft
	return nil
}

// RecordHostFunctionCall 记录宿主函数调用（v2.0 更新签名）
//
// 📋 **参数**：
//   - call: 宿主函数调用记录
//
// 🔒 **并发安全**：使用写锁保护（同步模式）或无锁入队（异步模式）
// 🎯 **用途**：记录执行轨迹用于ZK证明生成
//
// ⚠️ **注意**：
// - 如果启用了异步轨迹记录，则使用无锁队列异步记录
// - 如果未启用异步轨迹记录，则使用同步记录（保持向后兼容）
func (c *contextImpl) RecordHostFunctionCall(call *ispcInterfaces.HostFunctionCall) {
	if call == nil {
		return
	}

	// P0: 异步轨迹记录（如果启用）
	if c.manager.asyncTraceEnabled && c.manager.traceQueue != nil {
		// 计算执行时间（Duration）- 需要加锁读取lastCallTime
		currentTime := c.manager.clock.Now()
		var duration time.Duration

		c.mutex.RLock()
		lastCallTime := c.lastCallTime
		c.mutex.RUnlock()

		if lastCallTime.IsZero() {
			// 第一次调用，Duration为0或从createdAt开始计算
			if !c.createdAt.IsZero() {
				duration = currentTime.Sub(c.createdAt)
			}
		} else {
			// 计算与上一个调用的时间差
			duration = currentTime.Sub(lastCallTime)
		}

		// 更新lastCallTime需要加锁
		c.mutex.Lock()
		c.lastCallTime = currentTime
		c.mutex.Unlock()

		// 转换为内部类型
		internalCall := HostFunctionCall{
			Sequence:     call.Sequence, // 保存Sequence
			FunctionName: call.FunctionName,
			Parameters:   call.Parameters,
			Result:       call.Result,
			Timestamp:    currentTime,
			Duration:     duration,
			Success:      true,
			Error:        "",
		}

		// 创建轨迹记录
		record := &TraceRecord{
			RecordType:       "host_function_call",
			HostFunctionCall: &internalCall,
			ExecutionID:      c.executionID,
		}

		// 异步入队（无锁）
		c.manager.traceQueue.Enqueue(record)

		// P0: 更新资源使用统计（异步模式下也需要加锁）
		c.mutex.Lock()
		if c.resourceUsage != nil {
			c.resourceUsage.HostFunctionCalls++
		}
		c.mutex.Unlock()

		return
	}

	// 同步记录（向后兼容）
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// P1: 计算执行时间（Duration）
	currentTime := c.manager.clock.Now()
	var duration time.Duration
	if c.lastCallTime.IsZero() {
		// 第一次调用，Duration为0或从createdAt开始计算
		if !c.createdAt.IsZero() {
			duration = currentTime.Sub(c.createdAt)
		}
	} else {
		// 计算与上一个调用的时间差
		duration = currentTime.Sub(c.lastCallTime)
	}
	c.lastCallTime = currentTime

	// 转换为内部类型并添加到调用记录列表
	internalCall := HostFunctionCall{
		Sequence:     call.Sequence, // 保存Sequence
		FunctionName: call.FunctionName,
		Parameters:   call.Parameters,
		Result:       call.Result,
		Timestamp:    currentTime,
		Duration:     duration, // P1: 已实现执行时间测量
		Success:      true,     // 默认成功
		Error:        "",
	}

	c.hostFunctionCalls = append(c.hostFunctionCalls, internalCall)

	// P0: 更新资源使用统计
	if c.resourceUsage != nil {
		c.resourceUsage.HostFunctionCalls++
		// 更新内存使用（使用runtime.MemStats）
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		c.resourceUsage.UpdatePeakMemory(m.Alloc)
	}

	// 记录执行事件
	event := ExecutionEvent{
		EventType: "host_function_call",
		Data: map[string]interface{}{
			"function_name": call.FunctionName,
			"sequence":      call.Sequence,
		},
		Timestamp: time.Unix(0, call.Timestamp),
	}
	c.executionEvents = append(c.executionEvents, event)
}

// GetExecutionTrace 获取执行轨迹（v2.0 更新签名）
//
// 📋 **参数**：无
// 🔧 **返回值**：
//   - []*ispcInterfaces.HostFunctionCall: 宿主函数调用列表
//   - error: 获取过程中的错误信息
//
// 🔒 **并发安全**：使用读锁保护
// 🎯 **用途**：供ZK证明生成器获取宿主函数调用轨迹
func (c *contextImpl) GetExecutionTrace() ([]*ispcInterfaces.HostFunctionCall, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// 转换 HostFunctionCall 到接口定义的类型
	trace := make([]*ispcInterfaces.HostFunctionCall, 0, len(c.hostFunctionCalls))
	for _, call := range c.hostFunctionCalls {
		// 转换 Parameters 和 Result 到 map[string]interface{}
		var params map[string]interface{}
		if call.Parameters != nil {
			if p, ok := call.Parameters.(map[string]interface{}); ok {
				params = p
			} else {
				params = map[string]interface{}{"value": call.Parameters}
			}
		}

		var result map[string]interface{}
		if call.Result != nil {
			if r, ok := call.Result.(map[string]interface{}); ok {
				result = r
			} else {
				result = map[string]interface{}{"value": call.Result}
			}
		}

		trace = append(trace, &ispcInterfaces.HostFunctionCall{
			Sequence:     call.Sequence, // 使用call的Sequence，如果为0则使用索引作为后备
			FunctionName: call.FunctionName,
			Parameters:   params,
			Result:       result,
			Timestamp:    call.Timestamp.UnixNano(),
		})
	}

	return trace, nil
}

// RecordTraceRecords 批量记录轨迹记录（异步轨迹记录优化）
//
// 📋 **参数**：
//   - records: 轨迹记录列表（包含host_function_call、state_change、execution_event）
//
// 📋 **返回值**：
//   - error: 写入失败时的错误信息
//
// 🔒 **并发安全**：使用写锁保护
// 🎯 **用途**：供TraceWorker批量写入轨迹记录，提升性能
func (c *contextImpl) RecordTraceRecords(records []ispcInterfaces.TraceRecord) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, record := range records {
		switch record.RecordType {
		case "host_function_call":
			if record.HostFunctionCall != nil {
				// 转换为内部类型
				internalCall := HostFunctionCall{
					Sequence:     record.HostFunctionCall.Sequence, // 保存Sequence
					FunctionName: record.HostFunctionCall.FunctionName,
					Parameters:   record.HostFunctionCall.Parameters,
					Result:       record.HostFunctionCall.Result,
					Timestamp:    time.Unix(0, record.HostFunctionCall.Timestamp),
					Duration:     0, // 异步记录时Duration已在入队时计算
					Success:      true,
					Error:        "",
				}
				c.hostFunctionCalls = append(c.hostFunctionCalls, internalCall)

				// 更新资源使用统计
				if c.resourceUsage != nil {
					c.resourceUsage.HostFunctionCalls++
				}
			}
		case "state_change":
			if record.StateChange != nil {
				// 转换为内部类型
				internalChange := StateChange{
					Type:      record.StateChange.Type,
					Key:       record.StateChange.Key,
					OldValue:  record.StateChange.OldValue,
					NewValue:  record.StateChange.NewValue,
					Timestamp: time.Unix(0, record.StateChange.Timestamp),
				}
				c.stateChanges = append(c.stateChanges, internalChange)

				// 更新资源使用统计
				if c.resourceUsage != nil {
					c.resourceUsage.StateChanges++
				}
			}
		case "execution_event":
			if record.ExecutionEvent != nil {
				// 转换为内部类型
				internalEvent := ExecutionEvent{
					EventType: record.ExecutionEvent.EventType,
					Data:      record.ExecutionEvent.Data,
					Timestamp: time.Unix(0, record.ExecutionEvent.Timestamp),
				}
				c.executionEvents = append(c.executionEvents, internalEvent)
			}
		}
	}

	return nil
}

// RecordStateChange 记录状态变更
//
// 📋 **参数**：
//   - changeType: 变更类型（如"utxo_create", "utxo_spend", "storage_set"等）
//   - key: 变更键值
//   - oldValue: 旧值
//   - newValue: 新值
//
// 🔧 **返回值**：
//   - error: 记录过程中的错误信息
//
// 🔒 **并发安全**：使用写锁保护
// 🎯 **用途**：记录状态变更用于执行轨迹和ZK证明生成
func (c *contextImpl) RecordStateChange(changeType string, key string, oldValue interface{}, newValue interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 创建状态变更记录
	change := StateChange{
		Type:      changeType,
		Key:       key,
		OldValue:  oldValue,
		NewValue:  newValue,
		Timestamp: c.manager.clock.Now(),
	}

	// 添加到状态变更列表
	c.stateChanges = append(c.stateChanges, change)

	// P0: 更新资源使用统计
	if c.resourceUsage != nil {
		c.resourceUsage.StateChanges++
	}

	// 记录执行事件
	event := ExecutionEvent{
		EventType: "state_change",
		Data: map[string]interface{}{
			"change_type": changeType,
			"key":         key,
			"timestamp":   change.Timestamp,
		},
		Timestamp: change.Timestamp,
	}
	c.executionEvents = append(c.executionEvents, event)

	return nil
}

// GetExecutionHandle 获取执行句柄
//
// 📋 **参数**：无
// 🔧 **返回值**：
//   - *ispcInterfaces.ExecutionHandle: 当前执行的句柄对象
//   - error: 如果句柄未初始化则返回错误
//
// 🔒 **并发安全**：使用读锁保护，支持并发读取
// 🎯 **用途**：供后置阶段获取前置阶段的执行结果
// ⚠️ **注意事项**：必须先调用SetExecutionHandle设置句柄
// 已移除 ExecutionHandle 相关方法，遵循最小可用同步路径

// GetResourceUsage 获取资源使用统计
//
// 📋 **参数**：无
// 🔧 **返回值**：
//   - *types.ResourceUsage: 资源使用统计（如果未启用则返回nil）
//
// 🔒 **并发安全**：使用读锁保护
// 🎯 **用途**：供coordinator获取资源使用统计，用于性能分析和问题诊断
func (c *contextImpl) GetResourceUsage() *types.ResourceUsage {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.resourceUsage == nil {
		return nil
	}

	// 返回副本，防止外部修改
	usage := *c.resourceUsage
	return &usage
}

// FinalizeResourceUsage 完成资源使用统计
//
// 📋 **参数**：无
// 🔧 **返回值**：无
//
// 🔒 **并发安全**：使用写锁保护
// 🎯 **用途**：在执行结束时调用，完成资源使用统计的计算
func (c *contextImpl) FinalizeResourceUsage() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.resourceUsage == nil {
		return
	}

	// 设置结束时间
	c.resourceUsage.EndTime = c.manager.clock.Now()

	// 计算执行轨迹大小
	traceSize := uint64(0)
	for _, call := range c.hostFunctionCalls {
		// 估算每个调用的内存占用（简化计算）
		traceSize += uint64(len(call.FunctionName)) + 100 // 基础开销
	}
	c.resourceUsage.UpdateTraceSize(traceSize)

	// 完成统计
	c.resourceUsage.Finalize()
}

// ContextManagerConfig 执行上下文管理器配置
//
// 🎯 **配置项说明**：
// 定义上下文管理器的各项配置参数，包括资源限制、超时设置、清理策略等。
type ContextManagerConfig struct {
	// 上下文超时配置
	DefaultTimeoutMs   int64 // 默认超时时间（毫秒）
	MaxContextLifetime int64 // 最大生存时间（毫秒）

	// 资源限制
	MaxConcurrentContexts int    // 最大并发上下文数
	MaxMemoryPerContext   uint64 // 每个上下文最大内存（字节）

	// 清理配置
	CleanupIntervalMs int64 // 清理间隔（毫秒）
	StateRetentionMs  int64 // 状态保留时间（毫秒）
}

// NewManager 创建执行上下文管理器
//
// 🎯 **依赖注入构造器**：
// 本构造器专注于依赖注入的框架性实现，接收所有必要的基础设施服务。
//
// 📋 **参数说明**：
//   - logger: 日志服务，来自基础设施
//   - configProvider: 配置提供者，来自基础设施
//   - clockService: 时钟服务，来自基础设施
//
// 🔧 **返回值**：
//   - *Manager: 完整初始化的上下文管理器实例
//
// ⚠️ **注意事项**：
// 当前为框架性实现，专注依赖注入结构，具体业务逻辑待后续实现。
func NewManager(
	logger log.Logger,
	configProvider config.Provider,
	clockService infraClock.Clock,
) *Manager {
	// 默认配置
	config := &ContextManagerConfig{
		DefaultTimeoutMs:      30000,     // 30秒
		MaxContextLifetime:    300000,    // 5分钟
		MaxConcurrentContexts: 100,       // 最多100个并发上下文
		MaxMemoryPerContext:   104857600, // 100MB
		CleanupIntervalMs:     60000,     // 1分钟清理一次
		StateRetentionMs:      600000,    // 状态保留10分钟
	}

	manager := &Manager{
		logger:         logger,
		configProvider: configProvider,
		clock:          clockService,
		contexts:       make(map[string]ispcInterfaces.ExecutionContext),
		mutex:          sync.RWMutex{},
		config:         config,
		// P0: 初始化上下文隔离增强器
		isolationEnforcer: NewContextIsolationEnforcer(time.Duration(config.MaxContextLifetime) * time.Millisecond),
		cleanupVerifier:   NewContextCleanupVerifier(),
		// P0: 初始化执行结果一致性验证器
		resultVerifier: NewExecutionResultVerifier(),
		// P0: 初始化轨迹完整性检查器
		traceIntegrityChecker: NewTraceIntegrityChecker(nil),
		// P1: 初始化上下文调试器（默认关闭，可通过SetDebugMode启用）
		debugger: NewContextDebugger(logger, DebugModeOff),
		// P1: 初始化调试工具（稍后设置manager引用）
		debugTool: nil,
		// P0: 异步轨迹记录（默认禁用，保持向后兼容）
		traceQueue:        nil,
		traceWorkerPool:   nil,
		asyncTraceEnabled: false,
	}

	// 设置调试工具的manager引用
	manager.debugTool = NewDebugTool(manager, logger)

	// P0: 初始化异步轨迹记录（可选，默认禁用）
	// 如果需要启用异步轨迹记录，调用 EnableAsyncTraceRecording()

	// 启动后台清理任务（委托给内部函数）
	manager.startCleanupTask()

	return manager
}

// ==================== ExecutionContextManager接口实现 ====================

// CreateContext 创建执行上下文（公共接口实现）
//
// 📋 **参数**：
//   - ctx: context.Context - 外部调用上下文，用于继承超时、链路追踪等信息
//   - request: interface{} - 执行请求对象，需为*ispcInterfaces.ExecutionRequest类型
//
// 🔧 **返回值**：
//   - ispcInterfaces.ExecutionContext: 新创建的执行上下文实例
//   - error: 创建失败时的错误信息
//
// 🔒 **并发安全**：委托给内部实现，使用写锁保护contexts映射
// 🎯 **用途**：为每次ISPC执行创建独立的执行环境
// ⚠️ **薄实现**：直接委托给createContextInternal处理复杂逻辑
func (m *Manager) CreateContext(ctx context.Context, executionID string, callerAddress string) (ispcInterfaces.ExecutionContext, error) {
	return m.createContextInternal(ctx, executionID, callerAddress)
}

// DestroyContext 销毁执行上下文（公共接口实现）
//
// 📋 **参数**：
//   - ctx: context.Context - 外部调用上下文（当前未使用，为接口兼容性保留）
//   - executionID: string - 要销毁的执行上下文ID
//
// 🔧 **返回值**：
//   - error: 销毁失败时的错误信息，幂等设计下通常返回nil
//
// 🔒 **并发安全**：委托给内部实现，使用写锁保护contexts映射
// 🎯 **用途**：清理执行完成或异常的上下文，释放内存资源
// ⚠️ **幂等设计**：重复调用不会报错，确保清理的可靠性
// ⚠️ **薄实现**：直接委托给destroyContextInternal处理复杂逻辑
func (m *Manager) DestroyContext(ctx context.Context, executionID string) error {
	return m.destroyContextInternal(ctx, executionID)
}

// GetContext 获取执行上下文（公共接口实现）
//
// 📋 **参数**：
//   - executionID: string - 执行上下文的唯一标识符
//
// 🔧 **返回值**：
//   - ispcInterfaces.ExecutionContext: 找到的执行上下文实例
//   - error: 未找到或已过期时的错误信息
//
// 🔒 **并发安全**：委托给内部实现，使用读锁保护contexts映射
// 🎯 **用途**：供后置阶段获取前置阶段创建的执行上下文
// ⚠️ **过期检查**：会自动检查上下文是否过期
// ⚠️ **薄实现**：直接委托给getContextInternal处理复杂逻辑
func (m *Manager) GetContext(executionID string) (ispcInterfaces.ExecutionContext, error) {
	return m.getContextInternal(executionID)
}

// ListContexts 列出所有活跃的执行上下文ID
//
// 📋 **返回值**：
//   - []string: 所有活跃的执行上下文ID列表
//
// 🔒 **并发安全**：使用读锁保护contexts映射
// 🎯 **用途**：供调试工具列出所有上下文
func (m *Manager) ListContexts() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	executionIDs := make([]string, 0, len(m.contexts))
	for executionID := range m.contexts {
		executionIDs = append(executionIDs, executionID)
	}
	return executionIDs
}

// GetStats 获取管理器统计信息
//
// 📋 **返回值**：
//   - map[string]interface{}: 统计信息（活跃上下文数、异步轨迹记录状态等）
//
// 🔒 **并发安全**：使用读锁保护contexts映射
// 🎯 **用途**：供调试工具显示统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mutex.RLock()
	activeContextCount := len(m.contexts)
	m.mutex.RUnlock()

	stats := map[string]interface{}{
		"active_context_count": activeContextCount,
		"async_trace_enabled":  m.asyncTraceEnabled,
	}

	// 添加异步轨迹记录统计（如果启用）
	if m.asyncTraceEnabled && m.traceWorkerPool != nil {
		workerStats := m.traceWorkerPool.GetStats()
		stats["async_trace"] = workerStats
	}

	// 添加清理验证统计
	if m.cleanupVerifier != nil {
		cleanupStats := m.cleanupVerifier.GetCleanupStats()
		stats["cleanup"] = cleanupStats
	}

	// 添加执行结果验证统计
	if m.resultVerifier != nil {
		executionStats := m.resultVerifier.GetExecutionStats()
		stats["execution"] = executionStats
	}

	return stats
}

// ==================== P1: 日志和调试增强方法 ====================

// GetDebugger 获取上下文调试器
//
// 🎯 **调试工具**：
// - 提供上下文生命周期日志记录
// - 支持设置调试模式（Off、Basic、Verbose）
//
// 📋 **返回值**：
//   - *ContextDebugger: 上下文调试器实例
func (m *Manager) GetDebugger() *ContextDebugger {
	return m.debugger
}

// GetDebugTool 获取调试工具
//
// 🎯 **调试工具**：
// - 提供调试命令执行功能
// - 支持上下文状态导出
//
// 📋 **返回值**：
//   - *DebugTool: 调试工具实例
func (m *Manager) GetDebugTool() *DebugTool {
	return m.debugTool
}

// SetDebugMode 设置调试模式
//
// 📋 **参数**：
//   - mode: 调试模式（DebugModeOff、DebugModeBasic、DebugModeVerbose）
func (m *Manager) SetDebugMode(mode DebugMode) {
	if m.debugger != nil {
		m.debugger.SetDebugMode(mode)
	}
}

// ExportContextState 导出上下文状态（便捷方法）
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//   - includeStackTrace: 是否包含堆栈跟踪
//
// 📋 **返回值**：
//   - []byte: JSON格式的状态快照
//   - error: 导出错误
func (m *Manager) ExportContextState(executionID string, includeStackTrace bool) ([]byte, error) {
	ctx, err := m.GetContext(executionID)
	if err != nil {
		return nil, fmt.Errorf("获取上下文失败: %w", err)
	}
	return ExportContextStateJSON(ctx, includeStackTrace)
}

// ==================== P0: 异步轨迹记录管理方法 ====================

// EnableAsyncTraceRecording 启用异步轨迹记录
//
// 🎯 **异步轨迹记录**：
// - 创建无锁队列和工作线程池
// - 启动工作线程池
// - 后续的轨迹记录将使用异步模式
//
// 📋 **参数**：
//   - workerCount: 工作线程数量（默认2）
//   - batchSize: 批量大小（默认100）
//   - batchTimeout: 批量超时（默认100ms）
//   - maxRetries: 最大重试次数（默认3）
//   - retryDelay: 重试延迟（默认10ms）
func (m *Manager) EnableAsyncTraceRecording(workerCount int, batchSize int, batchTimeout time.Duration, maxRetries int, retryDelay time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.asyncTraceEnabled {
		// 幂等性：如果已启用，直接返回成功
		return nil
	}

	// 创建无锁队列
	m.traceQueue = NewLockFreeQueue()

	// 创建工作线程池
	m.traceWorkerPool = NewTraceWorkerPool(
		m.traceQueue,
		workerCount,
		batchSize,
		batchTimeout,
		maxRetries,
		retryDelay,
		m.logger,
	)

	// 启动工作线程池
	m.traceWorkerPool.Start()

	m.asyncTraceEnabled = true

	if m.logger != nil {
		m.logger.Infof("✅ 异步轨迹记录已启用: workerCount=%d, batchSize=%d, batchTimeout=%v, maxRetries=%d, retryDelay=%v", workerCount, batchSize, batchTimeout, maxRetries, retryDelay)
	}

	return nil
}

// DisableAsyncTraceRecording 禁用异步轨迹记录
//
// 🎯 **禁用异步轨迹记录**：
// - 刷新队列，确保所有记录都已写入
// - 停止工作线程池
// - 后续的轨迹记录将使用同步模式
func (m *Manager) DisableAsyncTraceRecording() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.asyncTraceEnabled {
		// 幂等性：如果未启用，直接返回成功
		return nil
	}

	// 刷新队列
	if m.traceWorkerPool != nil {
		m.traceWorkerPool.Flush()
		m.traceWorkerPool.Stop()
		m.traceWorkerPool = nil
	}

	m.traceQueue = nil
	m.asyncTraceEnabled = false

	if m.logger != nil {
		m.logger.Infof("✅ 异步轨迹记录已禁用")
	}

	return nil
}

// FlushTraceQueue 刷新轨迹记录队列
//
// 🎯 **执行完成同步点**：
// - 刷新队列，确保所有记录都已写入ExecutionContext
// - 用于执行完成时确保轨迹完整性
func (m *Manager) FlushTraceQueue() error {
	if !m.asyncTraceEnabled || m.traceWorkerPool == nil {
		return nil // 未启用异步轨迹记录，无需刷新
	}

	// 刷新队列
	m.traceWorkerPool.Flush()

	if m.logger != nil {
		m.logger.Debugf("✅ 轨迹记录队列已刷新")
	}

	return nil
}

// GetTraceQueueStats 获取轨迹记录队列统计信息
//
// 📋 **返回值**：
//   - map[string]interface{}: 统计信息（队列统计和工作线程池统计）
func (m *Manager) GetTraceQueueStats() map[string]interface{} {
	if !m.asyncTraceEnabled {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	stats := make(map[string]interface{})
	stats["enabled"] = true

	if m.traceQueue != nil {
		stats["queue"] = m.traceQueue.GetStats()
	}

	if m.traceWorkerPool != nil {
		stats["worker_pool"] = m.traceWorkerPool.GetStats()
	}

	return stats
}

// IsAsyncTraceRecordingEnabled 检查是否启用异步轨迹记录
func (m *Manager) IsAsyncTraceRecordingEnabled() bool {
	return m.asyncTraceEnabled
}

// ==================== P0: 上下文隔离增强方法 ====================

// DetectContextLeaks 检测上下文泄漏
//
// 🎯 **泄漏检测**：
// - 检测超过最大生存时间仍未销毁的上下文
// - 检测访问次数异常高的上下文（可能的内存泄漏）
//
// 📋 **返回值**：
//   - leakedContexts: 泄漏的上下文列表
//   - err: 检测过程中的错误
func (m *Manager) DetectContextLeaks() (leakedContexts []string, err error) {
	if m.isolationEnforcer == nil {
		return nil, fmt.Errorf("隔离增强器未初始化")
	}
	return m.isolationEnforcer.DetectLeaks()
}

// VerifyContextCleanup 验证上下文清理
//
// 🎯 **清理验证**：
// - 检查上下文是否已从管理器中移除
// - 检查清理记录
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//
// 🔧 **返回值**：
//   - cleaned: 是否已清理
//   - issues: 清理问题列表
func (m *Manager) VerifyContextCleanup(executionID string) (cleaned bool, issues []string) {
	// 检查1：上下文是否仍在管理器中
	m.mutex.RLock()
	_, exists := m.contexts[executionID]
	m.mutex.RUnlock()

	if exists {
		return false, []string{"上下文仍在管理器中，未清理"}
	}

	// 检查2：检查清理记录
	if m.cleanupVerifier != nil {
		cleaned, record := m.cleanupVerifier.VerifyCleanup(executionID)
		if !cleaned {
			return false, []string{fmt.Sprintf("清理记录不存在或清理失败: %v", record)}
		}
	}

	return true, []string{}
}

// GetCleanupStats 获取清理统计信息
func (m *Manager) GetCleanupStats() map[string]interface{} {
	if m.cleanupVerifier == nil {
		return map[string]interface{}{
			"error": "清理验证器未初始化",
		}
	}
	return m.cleanupVerifier.GetCleanupStats()
}

// DeepCopyContext 深度拷贝执行上下文
//
// 🎯 **深度拷贝**：
// - 拷贝所有基本字段
// - 拷贝所有切片和映射（深拷贝）
// - 不拷贝管理器引用（避免循环引用）
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//
// 🔧 **返回值**：
//   - *contextImpl: 深度拷贝的上下文副本
//   - error: 拷贝过程中的错误
func (m *Manager) DeepCopyContext(executionID string) (*contextImpl, error) {
	m.mutex.RLock()
	context, exists := m.contexts[executionID]
	m.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("执行上下文不存在: %s", executionID)
	}

	contextImpl, ok := context.(*contextImpl)
	if !ok {
		return nil, fmt.Errorf("上下文类型错误")
	}

	return DeepCopyContext(contextImpl)
}

// VerifyContextIsolation 验证两个上下文的隔离性
//
// 🎯 **隔离验证**：
// - 检查两个上下文是否完全独立
// - 检查是否有共享的可变状态
//
// 📋 **参数**：
//   - executionID1: 第一个上下文ID
//   - executionID2: 第二个上下文ID
//
// 🔧 **返回值**：
//   - isolated: 是否隔离
//   - issues: 隔离问题列表
func (m *Manager) VerifyContextIsolation(executionID1, executionID2 string) (isolated bool, issues []string) {
	m.mutex.RLock()
	ctx1, exists1 := m.contexts[executionID1]
	ctx2, exists2 := m.contexts[executionID2]
	m.mutex.RUnlock()

	if !exists1 {
		return false, []string{fmt.Sprintf("上下文1不存在: %s", executionID1)}
	}
	if !exists2 {
		return false, []string{fmt.Sprintf("上下文2不存在: %s", executionID2)}
	}

	ctx1Impl, ok1 := ctx1.(*contextImpl)
	ctx2Impl, ok2 := ctx2.(*contextImpl)

	if !ok1 || !ok2 {
		return false, []string{"上下文类型错误"}
	}

	return VerifyContextIsolation(ctx1Impl, ctx2Impl)
}

// CheckMemoryLeak 检查内存泄漏
//
// 🎯 **内存泄漏检测**：
// - 使用runtime.MemStats检测内存增长
// - 检测goroutine泄漏
//
// 📋 **返回值**：
//   - hasLeak: 是否检测到泄漏
//   - details: 泄漏详情
func (m *Manager) CheckMemoryLeak(beforeStats, afterStats *runtime.MemStats) (hasLeak bool, details map[string]interface{}) {
	return CheckMemoryLeak(beforeStats, afterStats)
}

// GetMemoryStats 获取当前内存统计
func (m *Manager) GetMemoryStats() *runtime.MemStats {
	return GetMemoryStats()
}

// ==================== P0: 确定性保证增强方法 ====================

// CreateDeterministicEnforcer 创建确定性执行增强器
//
// 🎯 **确定性增强**：
// - 为执行上下文创建确定性增强器
// - 固定时间戳和随机数种子
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//   - inputParams: 执行输入参数
//   - fixedTimestamp: 固定时间戳（可选，如果为nil使用当前时间）
//
// 🔧 **返回值**：
//   - *DeterministicEnforcer: 确定性增强器实例
func (m *Manager) CreateDeterministicEnforcer(executionID string, inputParams []byte, fixedTimestamp *time.Time) *DeterministicEnforcer {
	if fixedTimestamp == nil {
		now := m.clock.Now()
		fixedTimestamp = &now
	}
	return NewDeterministicEnforcer(executionID, inputParams, fixedTimestamp)
}

// RecordExecutionResult 记录执行结果（用于一致性验证）
//
// 🎯 **结果记录**：
// - 记录输入哈希和结果哈希的映射
// - 用于后续的一致性验证
//
// 📋 **参数**：
//   - inputHash: 执行输入哈希
//   - resultHash: 执行结果哈希
//
// 🔧 **返回值**：
//   - error: 记录过程中的错误
func (m *Manager) RecordExecutionResult(inputHash, resultHash []byte) error {
	if m.resultVerifier == nil {
		return fmt.Errorf("执行结果验证器未初始化")
	}
	return m.resultVerifier.RecordExecutionResult(inputHash, resultHash)
}

// VerifyExecutionResult 验证执行结果一致性
//
// 🎯 **一致性验证**：
// - 比较当前执行结果与历史执行结果
// - 确保相同输入产生相同输出
//
// 📋 **参数**：
//   - inputHash: 执行输入哈希
//   - resultHash: 执行结果哈希
//
// 🔧 **返回值**：
//   - consistent: 是否一致
//   - err: 验证过程中的错误
func (m *Manager) VerifyExecutionResult(inputHash, resultHash []byte) (consistent bool, err error) {
	if m.resultVerifier == nil {
		return true, nil // 如果未启用验证，返回一致
	}
	return m.resultVerifier.VerifyExecutionResult(inputHash, resultHash)
}

// GetExecutionStats 获取执行统计信息
func (m *Manager) GetExecutionStats() map[string]interface{} {
	if m.resultVerifier == nil {
		return map[string]interface{}{
			"error": "执行结果验证器未初始化",
		}
	}
	return m.resultVerifier.GetExecutionStats()
}

// ==================== P0: 轨迹完整性保证方法 ====================

// ValidateTrace 验证轨迹记录
//
// 🎯 **轨迹记录验证**：
// - 验证轨迹是否符合预期格式
// - 验证轨迹的完整性
func (m *Manager) ValidateTrace(trace *ExecutionTrace) []error {
	if m.traceIntegrityChecker == nil {
		return []error{fmt.Errorf("轨迹完整性检查器未初始化")}
	}
	return m.traceIntegrityChecker.ValidateTrace(trace)
}

// CheckTraceIntegrity 检查轨迹完整性
//
// 🎯 **轨迹完整性检查**：
// - 检查时间顺序
// - 检查调用顺序
// - 检查状态变更一致性
// - 检查轨迹哈希
func (m *Manager) CheckTraceIntegrity(trace *ExecutionTrace) (*IntegrityCheckResult, error) {
	if m.traceIntegrityChecker == nil {
		return nil, fmt.Errorf("轨迹完整性检查器未初始化")
	}
	return m.traceIntegrityChecker.CheckIntegrity(trace)
}

// RecordTraceForReplay 记录轨迹用于回放
//
// 🎯 **轨迹回放机制**：
// - 记录轨迹用于后续回放
// - 用于调试和问题排查
func (m *Manager) RecordTraceForReplay(executionID string, trace *ExecutionTrace) {
	if m.traceIntegrityChecker != nil {
		m.traceIntegrityChecker.RecordTraceForReplay(executionID, trace)
	}
}

// ReplayTrace 回放轨迹
//
// 🎯 **轨迹回放**：
// - 按照时间顺序回放轨迹
// - 用于调试和问题排查
func (m *Manager) ReplayTrace(executionID string, handler TraceReplayHandler) error {
	if m.traceIntegrityChecker == nil {
		return fmt.Errorf("轨迹完整性检查器未初始化")
	}
	return m.traceIntegrityChecker.ReplayTrace(executionID, handler)
}

// GetReplayRecords 获取回放记录列表
func (m *Manager) GetReplayRecords() []TraceReplayRecord {
	if m.traceIntegrityChecker == nil {
		return nil
	}
	return m.traceIntegrityChecker.GetReplayRecords()
}

// ClearReplayRecords 清空回放记录
func (m *Manager) ClearReplayRecords() {
	if m.traceIntegrityChecker != nil {
		m.traceIntegrityChecker.ClearReplayRecords()
	}
}

// RegisterTraceValidationRule 注册自定义轨迹验证规则
func (m *Manager) RegisterTraceValidationRule(rule TraceValidationRule) {
	if m.traceIntegrityChecker != nil {
		m.traceIntegrityChecker.RegisterValidationRule(rule)
	}
}

// ==================== 后置阶段支持方法（内部使用） ====================

// GetCurrentTime 获取当前确定性时间
//
// 🎯 **用途**：为其他模块提供统一的确定性时钟访问
// 🔒 **并发安全**：时钟服务本身是线程安全的
func (m *Manager) GetCurrentTime() time.Time {
	return m.clock.Now()
}

// ==================== 业务数据管理方法 ====================

// SetReturnData 设置业务返回数据
func (c *contextImpl) SetReturnData(data []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.returnData = make([]byte, len(data))
	copy(c.returnData, data)

	if c.manager != nil && c.manager.logger != nil {
		c.manager.logger.Infof("🔧 [ExecutionContext %s] SetReturnData: %d 字节", c.executionID, len(data))
	}

	return nil
}

// GetReturnData 获取业务返回数据
func (c *contextImpl) GetReturnData() ([]byte, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.manager != nil && c.manager.logger != nil {
		c.manager.logger.Infof("🔍 [ExecutionContext %s] GetReturnData 被调用: returnData长度=%d", c.executionID, len(c.returnData))
	}

	if c.returnData == nil {
		if c.manager != nil && c.manager.logger != nil {
			c.manager.logger.Warnf("🔍 [ExecutionContext %s] GetReturnData: returnData为nil", c.executionID)
		}
		return nil, nil
	}

	// 返回副本，防止外部修改
	result := make([]byte, len(c.returnData))
	copy(result, c.returnData)

	if c.manager != nil && c.manager.logger != nil {
		c.manager.logger.Infof("🔍 [ExecutionContext %s] GetReturnData 返回: %d 字节", c.executionID, len(result))
	}

	return result, nil
}

// AddEvent 添加事件
func (c *contextImpl) AddEvent(event *ispcInterfaces.Event) error {
	if event == nil {
		return fmt.Errorf("事件不能为空")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// 添加时间戳（如果未设置）
	if event.Timestamp == 0 && c.manager != nil && c.manager.clock != nil {
		event.Timestamp = c.manager.clock.Now().Unix()
	}

	c.events = append(c.events, event)

	if c.manager != nil && c.manager.logger != nil {
		c.manager.logger.Debugf("[ExecutionContext] 添加事件: type=%s", event.Type)
	}

	return nil
}

// GetEvents 获取所有事件
func (c *contextImpl) GetEvents() ([]*ispcInterfaces.Event, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if len(c.events) == 0 {
		return nil, nil
	}

	// 返回副本，防止外部修改
	result := make([]*ispcInterfaces.Event, len(c.events))
	copy(result, c.events)

	return result, nil
}

// ==================== 合约调用参数管理 ====================

// SetInitParams 设置合约调用参数
func (c *contextImpl) SetInitParams(params []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if params == nil {
		c.initParams = []byte{}
	} else {
		c.initParams = make([]byte, len(params))
		copy(c.initParams, params)
	}

	if c.manager != nil && c.manager.logger != nil {
		c.manager.logger.Debugf("[ExecutionContext] 设置合约调用参数: %d 字节", len(c.initParams))
	}

	return nil
}

func cloneBytes(src []byte) []byte {
	if len(src) == 0 {
		return nil
	}
	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

// GetInitParams 获取合约调用参数
func (c *contextImpl) GetInitParams() ([]byte, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if len(c.initParams) == 0 {
		return []byte{}, nil
	}

	// 返回副本，防止外部修改
	result := make([]byte, len(c.initParams))
	copy(result, c.initParams)

	return result, nil
}

// GetContractAddress 获取当前执行的合约地址
//
// 🎯 **用途**：供宿主函数获取合约地址（v1.0 新增）
//   - 用于创建 ContractTokenAsset 时填充 contract_address 字段
//
// 📋 **返回**：
//   - []byte: 合约地址（20字节）
//
// 🔒 **并发安全**：使用读锁保护
func (c *contextImpl) GetContractAddress() []byte {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if len(c.contractAddress) == 0 {
		return nil
	}

	// 返回副本，防止外部修改
	result := make([]byte, len(c.contractAddress))
	copy(result, c.contractAddress)

	return result
}

// SetStateSnapshots 设置执行前/后的状态快照哈希
func (c *contextImpl) SetStateSnapshots(stateBefore []byte, stateAfter []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.stateBefore = cloneBytes(stateBefore)
	c.stateAfter = cloneBytes(stateAfter)
}

// GetStateBefore 返回执行前状态哈希
func (c *contextImpl) GetStateBefore() []byte {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return cloneBytes(c.stateBefore)
}

// GetStateAfter 返回执行后状态哈希
func (c *contextImpl) GetStateAfter() []byte {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return cloneBytes(c.stateAfter)
}

// ==================== v2.0 新增：确定性区块视图 ====================

// GetBlockHeight 获取执行时的区块高度（v2.0 新增）
func (c *contextImpl) GetBlockHeight() uint64 {
	// 从 HostABI 获取真实区块高度
	if c.hostABI != nil {
		if height, err := c.hostABI.GetBlockHeight(context.Background()); err == nil {
			return height
		}
	}
	// 如果HostABI未注入或查询失败，返回0
	return 0
}

// GetBlockTimestamp 获取执行时的区块时间戳（v2.0 新增）
func (c *contextImpl) GetBlockTimestamp() uint64 {
	// P0: 使用确定性增强器的固定时间戳
	if c.deterministicEnforcer != nil {
		return uint64(c.deterministicEnforcer.GetFixedTimestamp().Unix())
	}

	// 回退到管理器时钟
	if c.manager != nil && c.manager.clock != nil {
		return uint64(c.manager.clock.Now().Unix())
	}
	return uint64(time.Now().Unix())
}

// GetChainID 获取链标识（v2.0 新增）
func (c *contextImpl) GetChainID() []byte {
	// P1: 从 configProvider 获取真实 ChainID
	if c.manager != nil && c.manager.configProvider != nil {
		blockchainConfig := c.manager.configProvider.GetBlockchain()
		if blockchainConfig != nil {
			// ChainID是uint64，转换为字符串格式（兼容原有接口）
			chainIDStr := fmt.Sprintf("%d", blockchainConfig.ChainID)
			return []byte(chainIDStr)
		}
	}

	// 如果无法从配置获取，返回默认值（向后兼容）
	return []byte("weisyn-testnet")
}

// GetTransactionID 获取当前交易ID（v2.0 新增）
//
// 🎯 **实现**：
// - 如果交易草稿存在且包含交易对象，计算真实的交易哈希（SHA-256）
// - 如果交易草稿不存在或交易对象为空，返回空切片
//
// 📋 **返回**：
//   - []byte: 交易ID（32字节哈希），如果无法计算则返回空切片
//
// ⚠️ **注意**：
// - 使用Protobuf确定性序列化确保跨平台一致性
// - 哈希计算基于交易的核心字段（排除签名字段）
func (c *contextImpl) GetTransactionID() []byte {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// 如果交易草稿不存在或交易对象为空，返回空切片
	if c.txDraft == nil || c.txDraft.Tx == nil {
		return nil
	}

	// 使用Protobuf确定性序列化交易对象
	mo := proto.MarshalOptions{Deterministic: true}
	txBytes, err := mo.Marshal(c.txDraft.Tx)
	if err != nil {
		// 序列化失败，返回空切片（不应该发生，但为了安全）
		if c.manager != nil && c.manager.logger != nil {
			c.manager.logger.Warnf("GetTransactionID: 序列化交易失败 executionID=%s, error=%v", c.executionID, err)
		}
		return nil
	}

	// 计算SHA-256哈希（32字节）
	hash := sha256.Sum256(txBytes)
	return hash[:]
}

// GetDraftID 获取交易草稿ID（v2.0 新增）
func (c *contextImpl) GetDraftID() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.txDraft != nil {
		return c.txDraft.DraftID
	}
	return ""
}

// ==================== P0: 确定性保证方法 ====================

// GetDeterministicClock 获取确定性时钟
//
// 🎯 **确定性时钟**：
// - 返回管理器使用的确定性时钟
// - 用于生成确定性时间戳
//
// 📋 **返回值**：
//   - infraClock.Clock: 确定性时钟实例
func (m *Manager) GetDeterministicClock() infraClock.Clock {
	return m.clock
}

// 🎯 **时间戳固定**：
// - 返回执行期间固定的时间戳
// - 确保相同输入产生相同的时间相关结果
func (c *contextImpl) GetDeterministicTimestamp() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.deterministicEnforcer != nil {
		return c.deterministicEnforcer.GetFixedTimestamp()
	}

	// 回退到createdAt
	return c.createdAt
}

// GetDeterministicRandomSource 获取确定性随机数源
//
// 🎯 **随机数种子固定**：
// - 基于executionID和输入参数生成确定性种子
// - 确保相同输入产生相同的随机数序列
func (c *contextImpl) GetDeterministicRandomSource() *DeterministicRandomSource {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.randomSource == nil && c.deterministicEnforcer != nil {
		seed := c.deterministicEnforcer.GetFixedRandomSeed()
		c.randomSource = NewDeterministicRandomSource(seed)
	}

	return c.randomSource
}

// SetExecutionResultHash 设置执行结果哈希（用于一致性验证）
func (c *contextImpl) SetExecutionResultHash(resultHash []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.deterministicEnforcer != nil {
		c.deterministicEnforcer.SetExecutionResultHash(resultHash)
	}
}

// ==================== Manager 薄实现原则 ====================
// 内部处理逻辑已委托给 internal_ops.go 中的函数
// Manager 只保留接口方法的实现
