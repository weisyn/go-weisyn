package coordinator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	contextpkg "github.com/weisyn/v1/internal/core/ispc/context"
	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	pb "github.com/weisyn/v1/pb/blockchain/block/transaction"
)

// ExecutionTrace 执行轨迹
type ExecutionTrace struct {
	TraceID            string
	StartTime          time.Time
	EndTime            time.Time
	HostFunctionCalls  []HostFunctionCall
	StateChanges       []StateChange
	OracleInteractions []OracleInteraction
	ExecutionPath      []string
}

// ExecutionResultData 执行结果数据
type ExecutionResultData struct {
	WasmResult        []uint64
	ExecutionTrace    ExecutionTrace
	HostFunctionCalls []HostFunctionCall
	StateChanges      []StateChange
	Timestamp         int64
}

// HostFunctionCall 宿主函数调用记录
type HostFunctionCall struct {
	FunctionName string
	Parameters   []any
	Result       any
	Timestamp    time.Time
}

// StateChange 状态变更记录
type StateChange struct {
	Type      string
	Key       string
	OldValue  any
	NewValue  any
	Timestamp time.Time
}

// OracleInteraction Oracle交互记录
type OracleInteraction struct {
	OracleType string
	Request    any
	Response   any
	Timestamp  time.Time
}

// extractExecutionTrace 提取执行轨迹
//
// 从执行上下文中提取完整的执行轨迹，包括宿主函数调用、状态变更等信息（确定性实现）
func (m *Manager) extractExecutionTrace(ctx context.Context, executionContext interface{}) (*ExecutionTrace, error) {
	// 从上下文获取确定性的执行开始时间
	var startTime time.Time
	if executionStart := ctx.Value(ContextKeyExecutionStart); executionStart != nil {
		if st, ok := executionStart.(time.Time); ok {
			startTime = st
		}
	} else {
		startTime = time.Time{}
	}

	// 生成确定性的轨迹ID（基于执行开始时间而非当前时间）
	traceID := fmt.Sprintf("trace_%d", startTime.UnixNano())

	// 尝试从执行上下文中提取真实轨迹
	if execCtx, ok := executionContext.(interface{ GetExecutionTrace() (any, error) }); ok {
		rawTrace, err := execCtx.GetExecutionTrace()
		if err != nil {
			m.logger.Debugf("从执行上下文获取轨迹失败，使用默认轨迹: %v", err)
		} else {
			// 如果执行上下文返回的是我们定义的ExecutionTrace结构
			if contextTrace, ok := rawTrace.(*contextpkg.ExecutionTrace); ok {
				// 转换context包中的结构到coordinator包中的结构
				// 使用上下文中的实际EndTime（已由确定性时钟计算），而不是占位的100ms
				trace := &ExecutionTrace{
					TraceID:            traceID,                // 使用确定性的ID
					StartTime:          contextTrace.StartTime, // 使用上下文的实际开始时间
					EndTime:            contextTrace.EndTime,   // 使用上下文的确定性结束时间
					HostFunctionCalls:  convertHostFunctionCalls(contextTrace.HostFunctionCalls),
					StateChanges:       convertStateChanges(contextTrace.StateChanges),
					OracleInteractions: []OracleInteraction{}, // 暂时为空
					ExecutionPath:      []string{"contract_call"},
				}

				m.logger.Debugf("提取到真实执行轨迹: duration=%v, hostCalls=%d, stateChanges=%d",
					trace.EndTime.Sub(trace.StartTime), len(trace.HostFunctionCalls), len(trace.StateChanges))
				return trace, nil
			}
		}
	}

	// 如果无法从执行上下文提取轨迹，构建基础执行轨迹
	// 使用startTime作为结束时间（表示瞬时执行，无法获取真实执行时间）
	trace := &ExecutionTrace{
		TraceID:            traceID,
		StartTime:          startTime,
		EndTime:            startTime, // 使用开始时间作为结束时间（瞬时执行）
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	return trace, nil
}

// computeExecutionResultHash 计算执行结果哈希
//
// 将执行结果和轨迹进行标准化序列化，然后计算SHA-256哈希（确定性实现）
func (m *Manager) computeExecutionResultHash(result []uint64, trace *ExecutionTrace) ([]byte, error) {
	// 构建执行结果数据（移除非确定性时间戳）
	resultData := &ExecutionResultData{
		WasmResult:        result,
		ExecutionTrace:    *trace,
		HostFunctionCalls: trace.HostFunctionCalls,
		StateChanges:      trace.StateChanges,
		// 使用轨迹的开始时间作为确定性时间戳，而非当前时间
		Timestamp: trace.StartTime.Unix(),
	}

	// 序列化为规范化字节数组
	canonicalBytes, err := m.canonicalizeExecutionResult(resultData)
	if err != nil {
		return nil, fmt.Errorf("序列化执行结果失败: %w", err)
	}

	// 计算SHA-256哈希
	// P1: 使用公共接口 HashManager 而不是直接使用 crypto/sha256
	if m.hashManager == nil {
		return nil, fmt.Errorf("hashManager未初始化，无法计算执行结果哈希")
	}
	hash := m.hashManager.SHA256(canonicalBytes)
	return hash, nil
}

// generateZKProof 生成零知识证明
//
// 基于执行结果哈希和轨迹生成ZK证明
func (m *Manager) generateZKProof(ctx context.Context, executionResultHash []byte, trace *ExecutionTrace) (*pb.ZKStateProof, error) {
	// 构建公开输入
	publicInputs := [][]byte{
		executionResultHash,
	}

	// 从上下文中提取合约信息
	if contractAddr := ctx.Value(ContextKeyContract); contractAddr != nil {
		if addr, ok := contractAddr.(string); ok {
			publicInputs = append(publicInputs, []byte(addr))
		}
	}

	if functionName := ctx.Value(ContextKeyFunction); functionName != nil {
		if name, ok := functionName.(string); ok {
			publicInputs = append(publicInputs, []byte(name))
		}
	}

	// 构建ZK证明输入
	// 🎯 **电路ID规范**：使用基础名，版本号单独指定
	// 🎯 **私有输入编码**：使用确定性哈希而不是原始字符串

	// 计算execution_trace哈希（确定性编码）
	traceBytes, err := m.serializeExecutionTraceForZK(trace)
	if err != nil {
		return nil, fmt.Errorf("序列化execution_trace失败: %w", err)
	}
	// P1: 使用公共接口 HashManager 而不是直接使用 crypto/sha256
	if m.hashManager == nil {
		return nil, fmt.Errorf("hashManager未初始化，无法计算execution_trace哈希")
	}
	traceHash := m.hashManager.SHA256(traceBytes)

	// 计算state_diff哈希（确定性编码）
	stateBytes, err := m.serializeStateChangesForZK(trace.StateChanges)
	if err != nil {
		return nil, fmt.Errorf("序列化state_diff失败: %w", err)
	}
	// P1: 使用公共接口 HashManager 而不是直接使用 crypto/sha256
	stateDiffHash := m.hashManager.SHA256(stateBytes)

	zkInput := &interfaces.ZKProofInput{
		PublicInputs: publicInputs,
		PrivateInputs: map[string]any{
			"execution_trace": traceHash,     // 32字节SHA256哈希（来自HashManager）
			"state_diff":      stateDiffHash, // 32字节SHA256哈希（来自HashManager）
		},
		CircuitID:      "contract_execution", // 基础名（不含.v1后缀）
		CircuitVersion: 1,                    // 版本号独立指定
	}

	// 调用ZK证明管理器生成证明
	m.logger.Debugf("🔐 开始生成 ZK 证明: circuitID=%s, version=%d", zkInput.CircuitID, zkInput.CircuitVersion)
	zkProof, err := m.zkproofManager.GenerateStateProof(ctx, zkInput)
	if err != nil {
		m.logger.Errorf("❌ ZK 证明生成失败: %v", err)
		return nil, fmt.Errorf("生成ZK证明失败: %w", err)
	}

	// 打印 ZK 证明生成结果
	m.printZKProofResult(zkInput.CircuitID, zkInput.CircuitVersion, zkProof)

	return zkProof, nil
}

// generateStateID 生成状态ID
//
// 基于执行上下文生成确定性的状态ID
func (m *Manager) generateStateID(ctx context.Context) ([]byte, error) {
	// 构建确定性输入：基于合约信息和执行开始时间
	var stateIDInputs [][]byte

	// 添加合约地址（确定性）
	if contractAddr := ctx.Value(ContextKeyContract); contractAddr != nil {
		if addr, ok := contractAddr.(string); ok {
			stateIDInputs = append(stateIDInputs, []byte(addr))
		}
	}

	// 添加函数名（确定性）
	if functionName := ctx.Value(ContextKeyFunction); functionName != nil {
		if name, ok := functionName.(string); ok {
			stateIDInputs = append(stateIDInputs, []byte(name))
		}
	}

	// 添加执行开始时间（确定性，来自上下文而非当前时间）
	if executionStart := ctx.Value(ContextKeyExecutionStart); executionStart != nil {
		if startTime, ok := executionStart.(time.Time); ok {
			// 使用执行开始时间的纳秒时间戳（确定性）
			timestampBytes := make([]byte, 8)
			timestamp := uint64(startTime.UnixNano())
			for i := 7; i >= 0; i-- {
				timestampBytes[i] = byte(timestamp >> (8 * (7 - i)))
			}
			stateIDInputs = append(stateIDInputs, timestampBytes)
		}
	}

	// 添加参数数量（确定性）
	if paramsCount := ctx.Value(ContextKeyParamsCount); paramsCount != nil {
		if count, ok := paramsCount.(int); ok {
			countBytes := make([]byte, 4)
			for i := 3; i >= 0; i-- {
				countBytes[i] = byte(count >> (8 * (3 - i)))
			}
			stateIDInputs = append(stateIDInputs, countBytes)
		}
	}

	// 拼接所有输入并计算SHA-256
	var allBytes []byte
	for _, input := range stateIDInputs {
		allBytes = append(allBytes, input...)
	}

	// 计算SHA-256哈希作为状态ID
	// P1: 使用公共接口 HashManager 而不是直接使用 crypto/sha256
	if m.hashManager == nil {
		return nil, fmt.Errorf("hashManager未初始化，无法生成状态ID")
	}
	hash := m.hashManager.SHA256(allBytes)
	return hash, nil
}

// getNodeID 获取节点ID
//
// 返回当前执行节点的标识符，优先从环境变量获取，否则使用默认值
func (m *Manager) getNodeID() string {
	// 尝试从环境变量获取节点ID
	if nodeID := os.Getenv("WEISYN_NODE_ID"); nodeID != "" {
		m.logger.Debugf("从环境变量获取节点ID: %s", nodeID)
		return nodeID
	}

	if nodeID := os.Getenv("NODE_ID"); nodeID != "" {
		m.logger.Debugf("从环境变量获取节点ID: %s", nodeID)
		return nodeID
	}

	// P1: 配置提供者集成（节点ID从libp2p PeerID获取，不通过配置）
	// 注意：NodeOptions中没有NodeID字段，节点ID应该从libp2p的PeerID中获取
	// 当前保持使用环境变量和默认值的方式，这是合理的实现

	// 如果环境变量中没有设置，使用默认值
	// 在生产环境中，应该确保设置了正确的节点ID
	m.logger.Debugf("未找到节点ID环境变量，使用默认值")
	return "weisyn-node-default"
}

// canonicalizeExecutionResult 序列化执行结果
//
// 将执行结果数据序列化为规范化字节数组，使用确定性的JSON序列化
func (m *Manager) canonicalizeExecutionResult(data *ExecutionResultData) ([]byte, error) {
	m.logger.Debug("开始规范化序列化执行结果")

	// 构建规范化的数据结构
	canonical := map[string]any{
		"version":   1, // 版本号
		"timestamp": data.Timestamp,
	}

	// 序列化WASM执行结果
	if len(data.WasmResult) > 0 {
		canonical["wasm_result"] = data.WasmResult
	}

	// 序列化执行轨迹
	if traceData, err := m.serializeExecutionTrace(&data.ExecutionTrace); err == nil {
		canonical["execution_trace"] = traceData
	} else {
		m.logger.Debugf("序列化执行轨迹失败: %v", err)
		return nil, fmt.Errorf("failed to serialize execution trace: %w", err)
	}

	// 序列化宿主函数调用（去重并排序以确保确定性）
	if len(data.HostFunctionCalls) > 0 {
		hostCalls, err := m.serializeHostFunctionCalls(data.HostFunctionCalls)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize host function calls: %w", err)
		}
		canonical["host_function_calls"] = hostCalls
	}

	// 序列化状态变更（排序以确保确定性）
	if len(data.StateChanges) > 0 {
		stateChanges, err := m.serializeStateChanges(data.StateChanges)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize state changes: %w", err)
		}
		canonical["state_changes"] = stateChanges
	}

	// 使用确定性JSON序列化
	return m.deterministicJSONMarshal(canonical)
}

// ==================== 轨迹结构转换函数 ====================

// convertHostFunctionCalls 转换宿主函数调用记录
//
// 将context包中的HostFunctionCall转换为coordinator包中的HostFunctionCall
func convertHostFunctionCalls(contextCalls []contextpkg.HostFunctionCall) []HostFunctionCall {
	var coordinatorCalls []HostFunctionCall
	for _, call := range contextCalls {
		coordinatorCall := HostFunctionCall{
			FunctionName: call.FunctionName,
			Parameters:   []any{call.Parameters}, // 包装成切片
			Result:       call.Result,
			Timestamp:    call.Timestamp,
		}
		coordinatorCalls = append(coordinatorCalls, coordinatorCall)
	}
	return coordinatorCalls
}

// convertStateChanges 转换状态变更记录
//
// 将context包中的StateChange转换为coordinator包中的StateChange
func convertStateChanges(contextChanges []contextpkg.StateChange) []StateChange {
	var coordinatorChanges []StateChange
	for _, change := range contextChanges {
		coordinatorChange := StateChange{
			Type:      change.Type,
			Key:       change.Key,
			OldValue:  change.OldValue,
			NewValue:  change.NewValue,
			Timestamp: change.Timestamp,
		}
		coordinatorChanges = append(coordinatorChanges, coordinatorChange)
	}
	return coordinatorChanges
}

// ==================== 序列化辅助函数 ====================

// serializeExecutionTrace 序列化执行轨迹
func (m *Manager) serializeExecutionTrace(trace *ExecutionTrace) (map[string]any, error) {
	return map[string]any{
		"trace_id":       trace.TraceID,
		"start_time":     trace.StartTime.Unix(),
		"end_time":       trace.EndTime.Unix(),
		"duration":       trace.EndTime.Sub(trace.StartTime).Nanoseconds(),
		"path_count":     len(trace.ExecutionPath),
		"execution_path": trace.ExecutionPath,
	}, nil
}

// serializeHostFunctionCalls 序列化宿主函数调用列表（确定性排序）
func (m *Manager) serializeHostFunctionCalls(calls []HostFunctionCall) ([]map[string]any, error) {
	serializedCalls := make([]map[string]any, len(calls))

	for i, call := range calls {
		serializedCalls[i] = map[string]any{
			"function_name": call.FunctionName,
			"timestamp":     call.Timestamp.Unix(),
			"param_count":   len(call.Parameters),
			"has_result":    call.Result != nil,
		}
	}

	// 按函数名和时间戳排序，确保确定性
	sort.Slice(serializedCalls, func(i, j int) bool {
		nameI := serializedCalls[i]["function_name"].(string)
		nameJ := serializedCalls[j]["function_name"].(string)
		if nameI != nameJ {
			return nameI < nameJ
		}
		timeI := serializedCalls[i]["timestamp"].(int64)
		timeJ := serializedCalls[j]["timestamp"].(int64)
		return timeI < timeJ
	})

	return serializedCalls, nil
}

// serializeStateChanges 序列化状态变更列表（确定性排序）
func (m *Manager) serializeStateChanges(changes []StateChange) ([]map[string]any, error) {
	serializedChanges := make([]map[string]any, len(changes))

	for i, change := range changes {
		serializedChanges[i] = map[string]any{
			"type":      change.Type,
			"key":       change.Key,
			"timestamp": change.Timestamp.Unix(),
			"has_old":   change.OldValue != nil,
			"has_new":   change.NewValue != nil,
		}
	}

	// 按类型、键和时间戳排序，确保确定性
	sort.Slice(serializedChanges, func(i, j int) bool {
		typeI := serializedChanges[i]["type"].(string)
		typeJ := serializedChanges[j]["type"].(string)
		if typeI != typeJ {
			return typeI < typeJ
		}
		keyI := serializedChanges[i]["key"].(string)
		keyJ := serializedChanges[j]["key"].(string)
		if keyI != keyJ {
			return keyI < keyJ
		}
		timeI := serializedChanges[i]["timestamp"].(int64)
		timeJ := serializedChanges[j]["timestamp"].(int64)
		return timeI < timeJ
	})

	return serializedChanges, nil
}

// deterministicJSONMarshal 确定性JSON序列化
//
// 🎯 **确定性保证**：
// - 顶层键按字母顺序排序
// - 递归处理嵌套的 map[string]any，确保所有层级的键都排序
// - slice 保持原有顺序（slice 本身是有序的）
//
// ⚠️ **注意**：
// - 对于嵌套的 map[string]any，会递归排序其键
// - 对于 slice，保持顺序不变（slice 本身有序）
// - 对于基本类型（int, string, bool等），直接序列化
func (m *Manager) deterministicJSONMarshal(data map[string]any) ([]byte, error) {
	// 递归规范化数据，确保所有嵌套 map 的键都排序
	normalized := m.normalizeMapForDeterministicJSON(data)

	// 使用bytes.Buffer来构建确定性的JSON
	var buf bytes.Buffer

	// 对键进行排序以确保确定性
	keys := make([]string, 0, len(normalized))
	for k := range normalized {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 手动构建JSON对象
	buf.WriteString("{")
	for i, key := range keys {
		if i > 0 {
			buf.WriteString(",")
		}
		// 写入键
		keyBytes, err := json.Marshal(key)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal key: %w", err)
		}
		buf.Write(keyBytes)
		buf.WriteString(":")

		// 写入值（值已经规范化，可以直接序列化）
		valueBytes, err := json.Marshal(normalized[key])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal value for key %s: %w", key, err)
		}
		buf.Write(valueBytes)
	}
	buf.WriteString("}")

	return buf.Bytes(), nil
}

// normalizeMapForDeterministicJSON 规范化 map 以确保确定性序列化
//
// 🎯 **递归处理**：
// - 对于 map[string]any，递归排序其键并规范化值
// - 对于 []any，递归规范化每个元素
// - 对于基本类型，直接返回
func (m *Manager) normalizeMapForDeterministicJSON(data map[string]any) map[string]any {
	normalized := make(map[string]any)

	// 收集所有键并排序
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 按排序后的键顺序处理值
	for _, key := range keys {
		value := data[key]
		normalized[key] = m.normalizeValueForDeterministicJSON(value)
	}

	return normalized
}

// normalizeValueForDeterministicJSON 规范化值以确保确定性序列化
//
// 🎯 **类型处理**：
// - map[string]any: 递归规范化
// - []any: 递归规范化每个元素
// - []map[string]any: 递归规范化每个 map
// - 基本类型: 直接返回
func (m *Manager) normalizeValueForDeterministicJSON(value any) any {
	switch v := value.(type) {
	case map[string]any:
		// 递归规范化嵌套的 map
		return m.normalizeMapForDeterministicJSON(v)
	case []any:
		// 规范化 slice 中的每个元素
		normalized := make([]any, len(v))
		for i, elem := range v {
			normalized[i] = m.normalizeValueForDeterministicJSON(elem)
		}
		return normalized
	case []map[string]any:
		// 规范化 []map[string]any（例如 host_function_calls, state_changes）
		normalized := make([]map[string]any, len(v))
		for i, elem := range v {
			normalized[i] = m.normalizeMapForDeterministicJSON(elem)
		}
		return normalized
	default:
		// 基本类型（int, string, bool, []uint64等）直接返回
		return value
	}
}

// serializeExecutionTraceForZK 序列化执行轨迹用于ZK证明（确定性编码）
//
// 🎯 **确定性保证**：
// - 固定字段顺序
// - 固定编码格式（大端序）
// - 不包含非确定性时间戳
func (m *Manager) serializeExecutionTraceForZK(trace *ExecutionTrace) ([]byte, error) {
	var buf bytes.Buffer

	// 1. 写入TraceID（字符串转字节）
	buf.WriteString(trace.TraceID)

	// 2. 写入StartTime（Unix时间戳，8字节大端序）
	startTimestamp := uint64(trace.StartTime.Unix())
	binary.Write(&buf, binary.BigEndian, startTimestamp)

	// 3. 写入EndTime（Unix时间戳，8字节大端序）
	endTimestamp := uint64(trace.EndTime.Unix())
	binary.Write(&buf, binary.BigEndian, endTimestamp)

	// 4. 写入HostFunctionCalls计数（4字节大端序）
	hostCallCount := uint32(len(trace.HostFunctionCalls))
	binary.Write(&buf, binary.BigEndian, hostCallCount)

	// 5. 写入StateChanges计数（4字节大端序）
	stateChangeCount := uint32(len(trace.StateChanges))
	binary.Write(&buf, binary.BigEndian, stateChangeCount)

	// 6. 写入ExecutionPath长度和内容（确定性编码）
	pathCount := uint32(len(trace.ExecutionPath))
	binary.Write(&buf, binary.BigEndian, pathCount)
	for _, path := range trace.ExecutionPath {
		pathLen := uint32(len(path))
		binary.Write(&buf, binary.BigEndian, pathLen)
		buf.WriteString(path)
	}

	return buf.Bytes(), nil
}

// serializeStateChangesForZK 序列化状态变更用于ZK证明（确定性编码）
//
// 🎯 **确定性保证**：
// - 固定字段顺序
// - 固定编码格式（大端序）
// - 按Type+Key排序（确保多次调用结果一致）
func (m *Manager) serializeStateChangesForZK(changes []StateChange) ([]byte, error) {
	var buf bytes.Buffer

	// 1. 写入状态变更数量（4字节大端序）
	changeCount := uint32(len(changes))
	binary.Write(&buf, binary.BigEndian, changeCount)

	// 2. 排序（确保确定性）
	sortedChanges := make([]StateChange, len(changes))
	copy(sortedChanges, changes)
	sort.Slice(sortedChanges, func(i, j int) bool {
		if sortedChanges[i].Type != sortedChanges[j].Type {
			return sortedChanges[i].Type < sortedChanges[j].Type
		}
		return sortedChanges[i].Key < sortedChanges[j].Key
	})

	// 3. 写入每个状态变更
	for _, change := range sortedChanges {
		// Type（字符串长度+内容）
		typeLen := uint32(len(change.Type))
		binary.Write(&buf, binary.BigEndian, typeLen)
		buf.WriteString(change.Type)

		// Key（字符串长度+内容）
		keyLen := uint32(len(change.Key))
		binary.Write(&buf, binary.BigEndian, keyLen)
		buf.WriteString(change.Key)

		// Timestamp（Unix时间戳，8字节大端序）
		timestamp := uint64(change.Timestamp.Unix())
		binary.Write(&buf, binary.BigEndian, timestamp)

		// OldValue存在标志（1字节）
		if change.OldValue != nil {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}

		// NewValue存在标志（1字节）
		if change.NewValue != nil {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	}

	return buf.Bytes(), nil
}

// printZKProofResult 打印 ZK 证明生成结果
//
// 🎯 **调试用途**：
//   - 在 ZK 证明生成完成后，打印证明信息
//   - 帮助观察 ZK 证明系统的运行状态
//
// 📋 **打印内容**：
//   - 电路ID、版本
//   - 证明长度
//   - 约束数量
//   - 证明方案和曲线
func (m *Manager) printZKProofResult(circuitID string, version uint32, proof *pb.ZKStateProof) {
	m.logger.Info("========== 🔐 ZK 证明生成结果 ==========")
	m.logger.Infof("电路ID: %s", circuitID)
	m.logger.Infof("电路版本: v%d", version)
	m.logger.Infof("证明长度: %d 字节", len(proof.Proof))
	m.logger.Infof("公开输入数量: %d", len(proof.PublicInputs))
	m.logger.Infof("约束数量: %d", proof.ConstraintCount)
	m.logger.Infof("证明方案: %s", proof.ProvingScheme)
	m.logger.Infof("曲线: %s", proof.Curve)
	m.logger.Infof("验证密钥哈希: %x", proof.VerificationKeyHash)
	m.logger.Info("✅ ZK 证明生成成功")
	m.logger.Info("======================================")
}

// BuildIdentityProof 构建 IdentityProof（调用者身份证明）
//
// ⚠️ **注意**：这是一个辅助函数，用于从 executionContext 构建基本的 IdentityProof
// 完整的 IdentityProof 需要调用者提供签名，这通常在交易构建阶段完成
//
// 📋 **字段赋值说明**：
// ✅ **真实业务赋值**：
//   - CallerAddress: 从 executionContext.GetCallerAddress() 获取（真实值）
//   - Algorithm: 默认 ECDSA_SECP256K1（合理默认值）
//   - SighashType: 默认 SIGHASH_ALL（合理默认值）
//   - Timestamp: 从 executionContext.GetBlockTimestamp() 或当前时间获取（真实值）
//   - ContextHash: 在 BuildExecutionProof 中计算并设置（真实值）
//
// ⚠️ **占位符（需要在交易构建阶段提供真实值）**：
//   - PublicKey: 如果未提供，创建33字节占位符（实际使用中必须提供真实公钥）
//   - Signature: 如果未提供，创建64字节占位符（实际使用中必须提供真实签名）
//   - ContextHash: 如果未提供，创建32字节占位符（BuildExecutionProof 会重新计算）
//
// 🔄 **待实现功能**：
//   - Nonce: 当前随机生成，实际使用中应该从 nonce 服务获取唯一的 nonce（TODO）
//
// 参数：
//   - executionContext: 执行上下文
//   - contextHash: ExecutionContext 的哈希（用于签名验证）
//   - signature: 调用者的签名（可选，如果为空则创建占位符）
//   - publicKey: 调用者的公钥（可选）
//
// 返回：
//   - *pb.IdentityProof: 构建的 IdentityProof
func BuildIdentityProof(
	executionContext interfaces.ExecutionContext,
	contextHash []byte,
	signature []byte,
	publicKey []byte,
) *pb.IdentityProof {
	// 获取调用者地址
	callerAddress := executionContext.GetCallerAddress()

	// ⚠️ 重要：此处不再生成任何“占位符”字节数组。
	// 原因：
	// - 33/64/32 字节的全零占位符会让上层误以为字段已齐备；
	// - 交易验证（ContractLockPlugin.verifyIdentityProof）要求 public_key/signature/nonce 非空且长度正确；
	// - 若此处填充全零，占位值会沿链路传播，导致问题定位困难。
	//
	// 约束：
	// - publicKey/signature/nonce 应由“交易构建/签名阶段”提供真实值；
	// - contextHash 会在 BuildExecutionProof 内部计算并写回 identityProof.ContextHash（如需要）。
	if len(contextHash) != 32 {
		contextHash = nil
	}

	// 获取时间戳
	var timestamp uint64
	if blockTimestamp := executionContext.GetBlockTimestamp(); blockTimestamp > 0 {
		timestamp = blockTimestamp
	} else {
		timestamp = uint64(time.Now().Unix())
	}

	return &pb.IdentityProof{
		PublicKey:     publicKey,
		CallerAddress: callerAddress,
		Signature:     signature,
		Algorithm:     pb.SignatureAlgorithm_SIGNATURE_ALGORITHM_ECDSA_SECP256K1, // 默认算法
		SighashType:   pb.SignatureHashType_SIGHASH_ALL,                          // 默认哈希类型
		Nonce:         nil, // 必须由交易构建/签名阶段填充真实 nonce（32字节）
		Timestamp:     timestamp,
		ContextHash:   contextHash,
	}
}

// BuildExecutionProof 构建 ExecutionProof（通用ISPC执行证明）
//
// ✅ **完整实现**：按照设计文档实现通用化设计
//   - 使用 ExecutionProof 替代 ContractProof（通用化）
//   - caller_identity 为必需字段（密码学安全保证）
//   - 使用 resource_address 替代 contract_address（通用化）
//   - 使用 ExecutionType 枚举标识执行类型
//   - 合约特定字段（method_name、call_depth）存储在 metadata 中
//
// 📋 **字段赋值说明**：
// ✅ **真实业务赋值**：
//   - ExecutionResultHash: 从 stateOutput.ExecutionResultHash 获取（真实值）
//   - StateTransitionProof: 从 stateOutput.ZkProof.Proof 获取（真实值）
//   - ExecutionTimeMs: 从参数传入（真实值）
//   - Context.CallerIdentity: 从参数传入（必需字段）
//   - Context.ResourceAddress: 从 executionContext.GetContractAddress() 获取（真实值）
//   - Context.ExecutionType: 从参数传入（真实值）
//   - Context.InputDataHash: 从 inputParameters 计算 SHA-256 哈希（真实值）
//   - Context.OutputDataHash: 从 returnData 或 executionResultHash 计算 SHA-256 哈希（真实值）
//   - Context.Metadata["method_name"]: 从 methodName 参数设置（真实值）
//   - Context.CallerIdentity.ContextHash: 在函数内部计算并设置（真实值）
//
// ⚠️ **未实现的字段（需要扩展接口）**：
//   - Context.Metadata["contract_state_before_hash"]: 需要扩展 ExecutionContext 接口添加 GetStateBefore() 方法
//   - Context.Metadata["contract_state_after_hash"]: 需要扩展 ExecutionContext 接口添加 GetStateAfter() 方法
//
// ⚠️ **占位符字段（需要在交易构建阶段提供真实值）**：
//   - Context.CallerIdentity.PublicKey: 如果 BuildIdentityProof 时未提供，使用占位符
//   - Context.CallerIdentity.Signature: 如果 BuildIdentityProof 时未提供，使用占位符
//   - Context.CallerIdentity.Nonce: 当前随机生成，应该从 nonce 服务获取（TODO）
//
// 参数：
//   - stateOutput: 执行状态输出
//   - executionContext: 执行上下文
//   - methodName: 方法名（合约特定，存储在metadata中）
//   - inputParameters: 输入参数（用于计算哈希）
//   - executionTimeMs: 执行时间（毫秒）
//   - executionType: 执行类型（WASM合约、ONNX模型等）
//   - callerIdentity: 调用者身份证明（必需）
//
// 返回：
//   - *pb.ExecutionProof: 构建的 ExecutionProof
//   - error: 构建失败时的错误
func BuildExecutionProof(
	stateOutput *pb.StateOutput,
	executionContext interfaces.ExecutionContext,
	methodName string,
	inputParameters []byte,
	executionTimeMs uint64,
	executionType pb.ExecutionType,
	callerIdentity *pb.IdentityProof,
) (*pb.ExecutionProof, error) {
	if stateOutput == nil {
		return nil, fmt.Errorf("stateOutput cannot be nil")
	}
	if executionContext == nil {
		return nil, fmt.Errorf("executionContext cannot be nil")
	}
	if callerIdentity == nil {
		return nil, fmt.Errorf("callerIdentity cannot be nil (required for cryptographic security)")
	}

	// 获取资源地址（通用：合约/模型/其他）
	resourceAddress := executionContext.GetContractAddress() // 使用GetContractAddress获取，但作为通用resource_address
	if len(resourceAddress) == 0 {
		return nil, fmt.Errorf("resource address is empty in execution context")
	}
	if len(resourceAddress) != 20 {
		return nil, fmt.Errorf("invalid resource address length: expected 20 bytes, got %d", len(resourceAddress))
	}

	// ========== 隐私保护设计：计算输入/输出数据哈希 ==========
	// 规范化输入参数：
	// - 如果 nil 或为空，使用 JSON 空数组 "[]"
	//   （Contract 验证插件要求字段存在，且调用无参方法时也需要一个占位值）
	normalizedParams := inputParameters
	if len(normalizedParams) == 0 {
		normalizedParams = []byte("[]")
	}

	// 计算输入数据哈希（32字节SHA-256，保护隐私）
	inputDataHash := sha256.Sum256(normalizedParams)

	// 计算输出数据哈希（从executionContext获取returnData并计算哈希）
	var outputDataHash [32]byte
	returnData, err := executionContext.GetReturnData()
	if err == nil && len(returnData) > 0 {
		// 如果有返回数据，计算哈希
		outputDataHash = sha256.Sum256(returnData)
	} else if len(stateOutput.ExecutionResultHash) == 32 {
		// 如果没有返回数据，使用execution_result_hash作为fallback
		// 注意：stateOutput 已经在函数开头检查过非nil，这里不需要再检查
		copy(outputDataHash[:], stateOutput.ExecutionResultHash)
	} else {
		// 如果都没有，使用空哈希
		outputDataHash = sha256.Sum256([]byte(""))
	}

	// ========== 构建 ExecutionContext（通用设计）==========
	// ⚠️ **边界原则**：ExecutionProof 不应该包含 Transaction 级别的信息
	// - value_sent：应该从 Transaction 的 inputs/outputs 中计算（不在这里设置）
	// - transaction_hash：应该从 Transaction 本身获取（不在这里设置）
	// - timestamp：应该使用 Transaction.creation_timestamp（不在这里设置）
	// - IdentityProof.timestamp：保留，用于 IdentityProof 的时效性验证（独立于 TX timestamp）
	execCtx := &pb.ExecutionProof_ExecutionContext{
		// ========== 身份和资源信息（通用，必需）==========
		CallerIdentity:  callerIdentity,  // ✅ 调用者身份证明（必需）
		ResourceAddress: resourceAddress, // ✅ 资源地址（通用）
		ExecutionType:   executionType,   // ✅ 执行类型（通用）

		// ========== 执行信息（通用，隐私保护）==========
		InputDataHash:  inputDataHash[:],  // ✅ 使用哈希替代原始数据
		OutputDataHash: outputDataHash[:], // ✅ 使用哈希替代原始数据

		// ========== 元数据扩展 ==========
		Metadata: make(map[string][]byte), // 初始化metadata map
	}

	// ========== 填充metadata中的扩展信息 ==========
	// 1. 合约特定字段（存储在metadata中，保持通用性）
	if len(methodName) > 0 {
		execCtx.Metadata["method_name"] = []byte(methodName)
	}
	// call_depth 如果需要，可以从executionContext获取或默认为0
	// execCtx.Metadata["call_depth"] = []byte(fmt.Sprintf("%d", callDepth))

	// 2. 状态哈希（如果executionContext支持获取状态）
	if snapshotProvider, ok := executionContext.(interfaces.StateSnapshotProvider); ok {
		if stateBefore := snapshotProvider.GetStateBefore(); len(stateBefore) > 0 {
			execCtx.Metadata["contract_state_before_hash"] = normalizeStateHash(stateBefore)
		}
		if stateAfter := snapshotProvider.GetStateAfter(); len(stateAfter) > 0 {
			execCtx.Metadata["contract_state_after_hash"] = normalizeStateHash(stateAfter)
		}
	}

	// ========== 计算并设置 IdentityProof 的 context_hash ==========
	// ✅ **关键修复**：在构建完 ExecutionContext 后，计算 context_hash 并更新 IdentityProof
	// context_hash 用于 IdentityProof 的签名验证，必须包含 ExecutionContext 的所有非敏感字段
	contextHash := computeExecutionContextHash(execCtx)
	if execCtx.CallerIdentity != nil {
		execCtx.CallerIdentity.ContextHash = contextHash
	}

	// 从 ZKProof 中提取 state_transition_proof
	var stateTransitionProof []byte
	if stateOutput.ZkProof != nil {
		stateTransitionProof = stateOutput.ZkProof.Proof
	}
	if len(stateTransitionProof) == 0 {
		return nil, fmt.Errorf("state_transition_proof is empty")
	}

	// 构建 ExecutionProof
	executionProof := &pb.ExecutionProof{
		ExecutionResultHash:  stateOutput.ExecutionResultHash,
		StateTransitionProof: stateTransitionProof,
		ExecutionTimeMs:      executionTimeMs,
		Context:              execCtx,
	}

	return executionProof, nil
}

// computeExecutionContextHash 计算 ExecutionContext 的哈希
//
// 用于 IdentityProof 的 context_hash 字段
func computeExecutionContextHash(execCtx *pb.ExecutionProof_ExecutionContext) []byte {
	var buf bytes.Buffer

	// 添加所有非敏感字段（按照设计文档的要求）
	// ⚠️ **安全修复**：只添加32字节的哈希，确保一致性
	if len(execCtx.InputDataHash) == 32 {
		buf.Write(execCtx.InputDataHash)
	}
	// ⚠️ **安全修复**：如果 InputDataHash 不是32字节，不添加（避免哈希不一致）

	if len(execCtx.OutputDataHash) == 32 {
		buf.Write(execCtx.OutputDataHash)
	}
	// ⚠️ **安全修复**：如果 OutputDataHash 不是32字节，不添加（避免哈希不一致）

	// ⚠️ **安全修复**：验证 ResourceAddress 长度，确保哈希一致性
	if len(execCtx.ResourceAddress) != 20 {
		// 如果长度不正确，使用空字节数组填充（防御性编程）
		// 注意：BuildExecutionProof 中已经验证了长度，这里只是防御性检查
		emptyAddr := make([]byte, 20)
		buf.Write(emptyAddr)
	} else {
		buf.Write(execCtx.ResourceAddress)
	}

	// 添加 execution_type（4字节）
	execTypeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(execTypeBytes, uint32(execCtx.ExecutionType))
	buf.Write(execTypeBytes)

	// ⚠️ **边界原则**：不包含 value_sent、transaction_hash 和 timestamp
	// - value_sent：应该从 Transaction 的 inputs/outputs 中计算
	// - transaction_hash：应该从 Transaction 本身获取
	// - timestamp：应该使用 Transaction.creation_timestamp
	// - IdentityProof.timestamp：保留，用于 IdentityProof 的时效性验证（独立于 TX timestamp）

	// 添加 metadata（排序后添加，确保确定性）
	if len(execCtx.Metadata) > 0 {
		keys := make([]string, 0, len(execCtx.Metadata))
		for k := range execCtx.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			buf.WriteString(k)
			buf.Write(execCtx.Metadata[k])
		}
	}

	// 计算SHA-256哈希
	// ⚠️ **注意**：这里使用标准库 sha256.Sum256，与 contract_lock.go 中的 hashManager.SHA256 应该产生相同结果
	// hashManager.SHA256 的实现也是使用 sha256.Sum256，所以是一致的
	hash := sha256.Sum256(buf.Bytes())
	return hash[:]
}

func normalizeStateHash(value []byte) []byte {
	if len(value) == 32 {
		return append([]byte(nil), value...)
	}
	if len(value) == 0 {
		return nil
	}
	hash := sha256.Sum256(value)
	return hash[:]
}

func computeStateSnapshotHashes(trace *ExecutionTrace) ([]byte, []byte) {
	if trace == nil || len(trace.StateChanges) == 0 {
		return nil, nil
	}

	var beforeBuf, afterBuf bytes.Buffer

	for _, change := range trace.StateChanges {
		beforeBuf.WriteString(change.Key)
		if change.OldValue != nil {
			if data, err := json.Marshal(change.OldValue); err == nil {
				beforeBuf.Write(data)
			}
		}

		afterBuf.WriteString(change.Key)
		if change.NewValue != nil {
			if data, err := json.Marshal(change.NewValue); err == nil {
				afterBuf.Write(data)
			}
		}
	}

	beforeHash := sha256.Sum256(beforeBuf.Bytes())
	afterHash := sha256.Sum256(afterBuf.Bytes())
	return beforeHash[:], afterHash[:]
}
