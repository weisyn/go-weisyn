package hostabi

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
	pbresource "github.com/weisyn/v1/pb/blockchain/block/transaction/resource"
	publicispc "github.com/weisyn/v1/pkg/interfaces/ispc"
)

// ============================================================================
// 原语调用追踪工具（文档完善）
// ============================================================================
//
// 🎯 **设计目的**：
// 提供详细的原语调用追踪功能，记录每次调用的详细信息，用于调试和性能分析。
//
// 🏗️ **实现策略**：
// - 记录每次原语调用的详细信息（参数、返回值、调用时间、耗时等）
// - 支持按原语名称过滤追踪记录
// - 支持导出追踪记录为JSON格式
// - 支持设置追踪级别（All、Errors、None）
//
// ⚠️ **注意**：
// - 追踪工具主要用于开发调试阶段
// - 生产环境应该禁用详细追踪（影响性能）
// - 追踪记录会占用内存，需要定期清理
//
// ============================================================================

// TraceLevel 追踪级别
type TraceLevel int

const (
	// TraceLevelNone 不追踪
	TraceLevelNone TraceLevel = iota
	// TraceLevelErrors 只追踪错误
	TraceLevelErrors
	// TraceLevelAll 追踪所有调用
	TraceLevelAll
)

// String 返回追踪级别字符串表示
func (l TraceLevel) String() string {
	switch l {
	case TraceLevelNone:
		return "None"
	case TraceLevelErrors:
		return "Errors"
	case TraceLevelAll:
		return "All"
	default:
		return "Unknown"
	}
}

// PrimitiveCallTrace 原语调用追踪记录
type PrimitiveCallTrace struct {
	PrimitiveName string                 // 原语名称
	CallTime      time.Time              // 调用时间
	Duration      time.Duration          // 调用耗时
	Params        map[string]interface{} // 调用参数（JSON序列化）
	Result        interface{}            // 调用结果（JSON序列化）
	Error         string                 // 错误信息（如果有）
	ExecutionID   string                 // 执行上下文ID（如果有）
	TraceID       string                 // 追踪ID（如果有）
}

// PrimitiveCallTracer 原语调用追踪器
type PrimitiveCallTracer struct {
	traces      []*PrimitiveCallTrace // 追踪记录列表
	maxTraces   int                    // 最大追踪记录数
	traceLevel  TraceLevel             // 追踪级别
	mutex       sync.RWMutex           // 保护追踪记录的并发访问
	enabled     bool                   // 是否启用追踪
}

// NewPrimitiveCallTracer 创建原语调用追踪器
//
// 📋 **参数**：
//   - maxTraces: 最大追踪记录数（0表示无限制）
//   - traceLevel: 追踪级别
func NewPrimitiveCallTracer(maxTraces int, traceLevel TraceLevel) *PrimitiveCallTracer {
	return &PrimitiveCallTracer{
		traces:     make([]*PrimitiveCallTrace, 0),
		maxTraces:  maxTraces,
		traceLevel: traceLevel,
		enabled:    traceLevel != TraceLevelNone,
	}
}

// Trace 记录原语调用
//
// 📋 **参数**：
//   - primitiveName: 原语名称
//   - startTime: 调用开始时间
//   - duration: 调用耗时
//   - params: 调用参数
//   - result: 调用结果
//   - err: 调用错误
//   - executionID: 执行上下文ID（可选）
//   - traceID: 追踪ID（可选）
func (t *PrimitiveCallTracer) Trace(
	primitiveName string,
	startTime time.Time,
	duration time.Duration,
	params map[string]interface{},
	result interface{},
	err error,
	executionID string,
	traceID string,
) {
	if !t.enabled {
		return
	}

	// 根据追踪级别决定是否记录
	if t.traceLevel == TraceLevelErrors && err == nil {
		return
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	trace := &PrimitiveCallTrace{
		PrimitiveName: primitiveName,
		CallTime:      startTime,
		Duration:      duration,
		Params:        params,
		Result:        result,
		ExecutionID:   executionID,
		TraceID:       traceID,
	}

	if err != nil {
		trace.Error = err.Error()
	}

	// 添加追踪记录
	t.traces = append(t.traces, trace)

	// 如果超过最大记录数，删除最旧的记录
	if t.maxTraces > 0 && len(t.traces) > t.maxTraces {
		t.traces = t.traces[1:]
	}
}

// GetTraces 获取所有追踪记录
//
// 📋 **参数**：
//   - primitiveName: 原语名称（可选，为空则返回所有记录）
//
// 📋 **返回值**：
//   - []*PrimitiveCallTrace: 追踪记录列表
func (t *PrimitiveCallTracer) GetTraces(primitiveName string) []*PrimitiveCallTrace {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if primitiveName == "" {
		// 返回所有记录的副本
		result := make([]*PrimitiveCallTrace, len(t.traces))
		copy(result, t.traces)
		return result
	}

	// 过滤指定原语的记录
	result := make([]*PrimitiveCallTrace, 0)
	for _, trace := range t.traces {
		if trace.PrimitiveName == primitiveName {
			result = append(result, trace)
		}
	}

	return result
}

// GetTracesByExecutionID 根据执行上下文ID获取追踪记录
//
// 📋 **参数**：
//   - executionID: 执行上下文ID
//
// 📋 **返回值**：
//   - []*PrimitiveCallTrace: 追踪记录列表
func (t *PrimitiveCallTracer) GetTracesByExecutionID(executionID string) []*PrimitiveCallTrace {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	result := make([]*PrimitiveCallTrace, 0)
	for _, trace := range t.traces {
		if trace.ExecutionID == executionID {
			result = append(result, trace)
		}
	}

	return result
}

// GetTracesByTraceID 根据追踪ID获取追踪记录
//
// 📋 **参数**：
//   - traceID: 追踪ID
//
// 📋 **返回值**：
//   - []*PrimitiveCallTrace: 追踪记录列表
func (t *PrimitiveCallTracer) GetTracesByTraceID(traceID string) []*PrimitiveCallTrace {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	result := make([]*PrimitiveCallTrace, 0)
	for _, trace := range t.traces {
		if trace.TraceID == traceID {
			result = append(result, trace)
		}
	}

	return result
}

// GetErrorTraces 获取所有错误追踪记录
//
// 📋 **返回值**：
//   - []*PrimitiveCallTrace: 错误追踪记录列表
func (t *PrimitiveCallTracer) GetErrorTraces() []*PrimitiveCallTrace {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	result := make([]*PrimitiveCallTrace, 0)
	for _, trace := range t.traces {
		if trace.Error != "" {
			result = append(result, trace)
		}
	}

	return result
}

// GetStats 获取追踪统计信息
//
// 📋 **返回值**：
//   - map[string]interface{}: 统计信息
func (t *PrimitiveCallTracer) GetStats() map[string]interface{} {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["total_traces"] = len(t.traces)
	stats["max_traces"] = t.maxTraces
	stats["trace_level"] = t.traceLevel.String()
	stats["enabled"] = t.enabled

	// 按原语统计
	primitiveStats := make(map[string]int)
	errorStats := make(map[string]int)
	totalDuration := time.Duration(0)

	for _, trace := range t.traces {
		primitiveStats[trace.PrimitiveName]++
		if trace.Error != "" {
			errorStats[trace.PrimitiveName]++
		}
		totalDuration += trace.Duration
	}

	stats["primitive_counts"] = primitiveStats
	stats["error_counts"] = errorStats
	if len(t.traces) > 0 {
		stats["avg_duration"] = totalDuration / time.Duration(len(t.traces))
	} else {
		stats["avg_duration"] = time.Duration(0)
	}

	return stats
}

// ExportJSON 导出追踪记录为JSON格式
//
// 📋 **参数**：
//   - primitiveName: 原语名称（可选，为空则导出所有记录）
//
// 📋 **返回值**：
//   - []byte: JSON格式的追踪记录
//   - error: 导出错误
func (t *PrimitiveCallTracer) ExportJSON(primitiveName string) ([]byte, error) {
	traces := t.GetTraces(primitiveName)
	data, err := json.MarshalIndent(traces, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化追踪记录失败: %w", err)
	}
	return data, nil
}

// Clear 清空追踪记录
func (t *PrimitiveCallTracer) Clear() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.traces = make([]*PrimitiveCallTrace, 0)
}

// SetTraceLevel 设置追踪级别
//
// 📋 **参数**：
//   - level: 追踪级别
func (t *PrimitiveCallTracer) SetTraceLevel(level TraceLevel) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.traceLevel = level
	t.enabled = level != TraceLevelNone
}

// Enable 启用追踪
func (t *PrimitiveCallTracer) Enable() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.enabled = true
}

// Disable 禁用追踪
func (t *PrimitiveCallTracer) Disable() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.enabled = false
}

// IsEnabled 检查是否启用追踪
func (t *PrimitiveCallTracer) IsEnabled() bool {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.enabled
}

// ============================================================================
// 追踪包装器（用于包装HostABI实现）
// ============================================================================

// HostRuntimePortsWithTracer 带追踪功能的HostABI实现包装器
type HostRuntimePortsWithTracer struct {
	publicispc.HostABI
	tracer *PrimitiveCallTracer
}

// NewHostRuntimePortsWithTracer 创建带追踪功能的HostABI包装器
//
// 📋 **参数**：
//   - hostABI: HostABI接口实例
//   - tracer: 原语调用追踪器
func NewHostRuntimePortsWithTracer(hostABI publicispc.HostABI, tracer *PrimitiveCallTracer) *HostRuntimePortsWithTracer {
	return &HostRuntimePortsWithTracer{
		HostABI: hostABI,
		tracer:  tracer,
	}
}

// GetTracer 获取追踪器实例
func (w *HostRuntimePortsWithTracer) GetTracer() *PrimitiveCallTracer {
	return w.tracer
}

// traceCall 追踪原语调用的辅助方法
func (w *HostRuntimePortsWithTracer) traceCall(
	primitiveName string,
	startTime time.Time,
	params map[string]interface{},
	result interface{},
	err error,
) {
	if w.tracer == nil || !w.tracer.IsEnabled() {
		return
	}

	duration := time.Since(startTime)

	// 尝试从context中获取executionID和traceID
	executionID := ""
	traceID := ""

	w.tracer.Trace(primitiveName, startTime, duration, params, result, err, executionID, traceID)
}

// ════════════════════════════════════════════════════════════════════════════════════════
// 类别 A：确定性区块视图（4个）
// ════════════════════════════════════════════════════════════════════════════════════════

// GetBlockHeight 包装GetBlockHeight方法（带追踪）
func (w *HostRuntimePortsWithTracer) GetBlockHeight(ctx context.Context) (uint64, error) {
	startTime := time.Now()
	params := make(map[string]interface{})

	result, err := w.HostABI.GetBlockHeight(ctx)

	w.traceCall("GetBlockHeight", startTime, params, result, err)

	return result, err
}

// GetBlockTimestamp 包装GetBlockTimestamp方法（带追踪）
func (w *HostRuntimePortsWithTracer) GetBlockTimestamp(ctx context.Context) (uint64, error) {
	startTime := time.Now()
	params := make(map[string]interface{})

	result, err := w.HostABI.GetBlockTimestamp(ctx)

	w.traceCall("GetBlockTimestamp", startTime, params, result, err)

	return result, err
}

// GetBlockHash 包装GetBlockHash方法（带追踪）
func (w *HostRuntimePortsWithTracer) GetBlockHash(ctx context.Context, height uint64) ([]byte, error) {
	startTime := time.Now()
	params := map[string]interface{}{
		"height": height,
	}

	result, err := w.HostABI.GetBlockHash(ctx, height)

	w.traceCall("GetBlockHash", startTime, params, result, err)

	return result, err
}

// GetChainID 包装GetChainID方法（带追踪）
func (w *HostRuntimePortsWithTracer) GetChainID(ctx context.Context) ([]byte, error) {
	startTime := time.Now()
	params := make(map[string]interface{})

	result, err := w.HostABI.GetChainID(ctx)

	w.traceCall("GetChainID", startTime, params, result, err)

	return result, err
}

// ════════════════════════════════════════════════════════════════════════════════════════
// 类别 B：执行上下文（3个）
// ════════════════════════════════════════════════════════════════════════════════════════

// GetCaller 包装GetCaller方法（带追踪）
func (w *HostRuntimePortsWithTracer) GetCaller(ctx context.Context) ([]byte, error) {
	startTime := time.Now()
	params := make(map[string]interface{})

	result, err := w.HostABI.GetCaller(ctx)

	w.traceCall("GetCaller", startTime, params, result, err)

	return result, err
}

// GetContractAddress 包装GetContractAddress方法（带追踪）
func (w *HostRuntimePortsWithTracer) GetContractAddress(ctx context.Context) ([]byte, error) {
	startTime := time.Now()
	params := make(map[string]interface{})

	result, err := w.HostABI.GetContractAddress(ctx)

	w.traceCall("GetContractAddress", startTime, params, result, err)

	return result, err
}

// GetTransactionID 包装GetTransactionID方法（带追踪）
func (w *HostRuntimePortsWithTracer) GetTransactionID(ctx context.Context) ([]byte, error) {
	startTime := time.Now()
	params := make(map[string]interface{})

	result, err := w.HostABI.GetTransactionID(ctx)

	w.traceCall("GetTransactionID", startTime, params, result, err)

	return result, err
}

// ════════════════════════════════════════════════════════════════════════════════════════
// 类别 C：UTXO查询（2个）
// ════════════════════════════════════════════════════════════════════════════════════════

// UTXOLookup 包装UTXOLookup方法（带追踪）
func (w *HostRuntimePortsWithTracer) UTXOLookup(ctx context.Context, outpoint *pb.OutPoint) (*pb.TxOutput, error) {
	startTime := time.Now()
	params := map[string]interface{}{
		"outpoint": outpoint,
	}

	result, err := w.HostABI.UTXOLookup(ctx, outpoint)

	w.traceCall("UTXOLookup", startTime, params, result, err)

	return result, err
}

// UTXOExists 包装UTXOExists方法（带追踪）
func (w *HostRuntimePortsWithTracer) UTXOExists(ctx context.Context, outpoint *pb.OutPoint) (bool, error) {
	startTime := time.Now()
	params := map[string]interface{}{
		"outpoint": outpoint,
	}

	result, err := w.HostABI.UTXOExists(ctx, outpoint)

	w.traceCall("UTXOExists", startTime, params, result, err)

	return result, err
}

// ════════════════════════════════════════════════════════════════════════════════════════
// 类别 D：资源查询（2个）
// ════════════════════════════════════════════════════════════════════════════════════════

// ResourceLookup 包装ResourceLookup方法（带追踪）
func (w *HostRuntimePortsWithTracer) ResourceLookup(ctx context.Context, contentHash []byte) (*pbresource.Resource, error) {
	startTime := time.Now()
	params := map[string]interface{}{
		"contentHash": contentHash,
	}

	result, err := w.HostABI.ResourceLookup(ctx, contentHash)

	w.traceCall("ResourceLookup", startTime, params, result, err)

	return result, err
}

// ResourceExists 包装ResourceExists方法（带追踪）
func (w *HostRuntimePortsWithTracer) ResourceExists(ctx context.Context, contentHash []byte) (bool, error) {
	startTime := time.Now()
	params := map[string]interface{}{
		"contentHash": contentHash,
	}

	result, err := w.HostABI.ResourceExists(ctx, contentHash)

	w.traceCall("ResourceExists", startTime, params, result, err)

	return result, err
}

// ════════════════════════════════════════════════════════════════════════════════════════
// 类别 E：交易草稿构建（4个）
// ════════════════════════════════════════════════════════════════════════════════════════

// TxAddInput 包装TxAddInput方法（带追踪）
func (w *HostRuntimePortsWithTracer) TxAddInput(ctx context.Context, outpoint *pb.OutPoint, isReferenceOnly bool, unlockingProof *pb.UnlockingProof) (uint32, error) {
	startTime := time.Now()
	params := map[string]interface{}{
		"outpoint":         outpoint,
		"isReferenceOnly":  isReferenceOnly,
		"unlockingProof":   unlockingProof,
	}

	result, err := w.HostABI.TxAddInput(ctx, outpoint, isReferenceOnly, unlockingProof)

	w.traceCall("TxAddInput", startTime, params, result, err)

	return result, err
}

// TxAddAssetOutput 包装TxAddAssetOutput方法（带追踪）
func (w *HostRuntimePortsWithTracer) TxAddAssetOutput(ctx context.Context, owner []byte, amount uint64, tokenID []byte, lockingConditions []*pb.LockingCondition) (uint32, error) {
	startTime := time.Now()
	params := map[string]interface{}{
		"owner":            owner,
		"amount":           amount,
		"tokenID":          tokenID,
		"lockingConditions": lockingConditions,
	}

	result, err := w.HostABI.TxAddAssetOutput(ctx, owner, amount, tokenID, lockingConditions)

	w.traceCall("TxAddAssetOutput", startTime, params, result, err)

	return result, err
}

// TxAddResourceOutput 包装TxAddResourceOutput方法（带追踪）
func (w *HostRuntimePortsWithTracer) TxAddResourceOutput(ctx context.Context, contentHash []byte, category string, owner []byte, lockingConditions []*pb.LockingCondition, metadata []byte) (uint32, error) {
	startTime := time.Now()
	params := map[string]interface{}{
		"contentHash":      contentHash,
		"category":         category,
		"owner":            owner,
		"lockingConditions": lockingConditions,
		"metadata":         metadata,
	}

	result, err := w.HostABI.TxAddResourceOutput(ctx, contentHash, category, owner, lockingConditions, metadata)

	w.traceCall("TxAddResourceOutput", startTime, params, result, err)

	return result, err
}

// TxAddStateOutput 包装TxAddStateOutput方法（带追踪）
func (w *HostRuntimePortsWithTracer) TxAddStateOutput(ctx context.Context, stateID []byte, stateVersion uint64, executionResultHash []byte, publicInputs []byte, parentStateHash []byte) (uint32, error) {
	startTime := time.Now()
	params := map[string]interface{}{
		"stateID":            stateID,
		"stateVersion":       stateVersion,
		"executionResultHash": executionResultHash,
		"publicInputs":       publicInputs,
		"parentStateHash":    parentStateHash,
	}

	result, err := w.HostABI.TxAddStateOutput(ctx, stateID, stateVersion, executionResultHash, publicInputs, parentStateHash)

	w.traceCall("TxAddStateOutput", startTime, params, result, err)

	return result, err
}

// ════════════════════════════════════════════════════════════════════════════════════════
// 类别 G：执行追踪（2个）
// ════════════════════════════════════════════════════════════════════════════════════════

// EmitEvent 包装EmitEvent方法（带追踪）
func (w *HostRuntimePortsWithTracer) EmitEvent(ctx context.Context, eventType string, eventData []byte) error {
	startTime := time.Now()
	params := map[string]interface{}{
		"eventType": eventType,
		"eventData": eventData,
	}

	err := w.HostABI.EmitEvent(ctx, eventType, eventData)

	w.traceCall("EmitEvent", startTime, params, nil, err)

	return err
}

// LogDebug 包装LogDebug方法（带追踪）
func (w *HostRuntimePortsWithTracer) LogDebug(ctx context.Context, message string) error {
	startTime := time.Now()
	params := map[string]interface{}{
		"message": message,
	}

	err := w.HostABI.LogDebug(ctx, message)

	w.traceCall("LogDebug", startTime, params, nil, err)

	return err
}

