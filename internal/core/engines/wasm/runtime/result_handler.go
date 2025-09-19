package runtime

import (
	"encoding/json"
	"fmt"
	"time"

	types "github.com/weisyn/v1/pkg/types"
)

// ResultHandler WASM执行结果处理器
// 负责处理和规范化WASM执行的返回值、消耗资源和元数据
type ResultHandler struct {
	// 编码器和解码器
	encoder *ParameterEncoder
	decoder *ParameterDecoder

	// 配置选项
	options *ResultHandlerOptions
}

// ResultHandlerOptions 结果处理器选项
type ResultHandlerOptions struct {
	// 是否启用严格模式验证
	StrictMode bool `json:"strictMode"`

	// 最大返回数据大小（字节）
	MaxReturnDataSize int `json:"maxReturnDataSize"`

	// 最大元数据条目数
	MaxMetadataEntries int `json:"maxMetadataEntries"`

	// 是否自动类型转换
	AutoTypeConversion bool `json:"autoTypeConversion"`

	// 是否收集详细统计
	CollectDetailedStats bool `json:"collectDetailedStats"`
}

// ExecutionContext 执行上下文
// 包含执行过程中的状态和资源使用信息
type ExecutionContext struct {
	// 执行标识
	ExecutionID string `json:"executionId"`

	// 开始时间
	StartTime time.Time `json:"startTime"`

	// 结束时间
	EndTime time.Time `json:"endTime"`

	// 资源相关
	ExecutionFeeLimit uint64 `json:"ExecutionFeeLimit"`
	ResourceUsed      uint64 `json:"ResourceUsed"`

	// 内存相关
	MemoryLimit uint32 `json:"memoryLimit"`
	MemoryUsed  uint32 `json:"memoryUsed"`
	PeakMemory  uint32 `json:"peakMemory"`

	// 执行相关
	InstructionCount uint64 `json:"instructionCount"`
	FunctionCalls    uint64 `json:"functionCalls"`
	HostCalls        uint64 `json:"hostCalls"`

	// 错误信息
	Error *WASMError `json:"error,omitempty"`

	// 警告信息
	Warnings []string `json:"warnings,omitempty"`

	// 调试信息
	DebugInfo map[string]interface{} `json:"debugInfo,omitempty"`
}

// ProcessedResult 处理后的执行结果
type ProcessedResult struct {
	// 基本信息
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`

	// 返回数据
	ReturnData []byte `json:"returnData,omitempty"`

	// 解析后的返回值
	ParsedValues []interface{} `json:"parsedValues,omitempty"`

	// 资源消耗
	Consumed *ResourceConsumption `json:"consumed"`

	// 执行元数据
	Metadata map[string]interface{} `json:"metadata"`

	// 性能指标
	Performance *PerformanceMetrics `json:"performance,omitempty"`

	// 错误信息（如果有）
	Error *WASMError `json:"error,omitempty"`
}

// ResourceConsumption 资源消耗信息
type ResourceConsumption struct {
	// 资源消耗
	Resource uint64 `json:"Resource"`

	// 内存使用（字节）
	Memory uint32 `json:"memory"`

	// 执行时间（毫秒）
	ExecutionTime int64 `json:"executionTime"`

	// 指令数量
	Instructions uint64 `json:"instructions"`

	// 函数调用次数
	FunctionCalls uint64 `json:"functionCalls"`

	// 宿主函数调用次数
	HostCalls uint64 `json:"hostCalls"`

	// CPU使用率（百分比）
	CPUUsage float64 `json:"cpuUsage,omitempty"`
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	// 执行阶段耗时
	CompilationTime   time.Duration `json:"compilationTime"`
	InstantiationTime time.Duration `json:"instantiationTime"`
	ExecutionTime     time.Duration `json:"executionTime"`
	CleanupTime       time.Duration `json:"cleanupTime"`

	// 吞吐量指标
	InstructionsPerSecond uint64  `json:"instructionsPerSecond"`
	ResourcePerSecond     uint64  `json:"ResourcePerSecond"`
	MemoryEfficiency      float64 `json:"memoryEfficiency"`

	// 缓存命中率
	ModuleCacheHitRate   float64 `json:"moduleCacheHitRate"`
	InstanceCacheHitRate float64 `json:"instanceCacheHitRate"`

	// 资源利用率
	ResourceUtilization float64 `json:"ResourceUtilization"`
	MemoryUtilization   float64 `json:"memoryUtilization"`
}

// NewResultHandler 创建结果处理器
func NewResultHandler(options *ResultHandlerOptions) *ResultHandler {
	if options == nil {
		options = defaultResultHandlerOptions()
	}

	return &ResultHandler{
		encoder: NewParameterEncoder(nil),
		decoder: NewParameterDecoder(nil),
		options: options,
	}
}

// ProcessExecutionResult 处理执行结果
func (rh *ResultHandler) ProcessExecutionResult(
	ctx *ExecutionContext,
	rawResult interface{},
	returnData []byte,
	expectedTypes []types.ValueType,
) (*ProcessedResult, error) {
	result := &ProcessedResult{
		Success:  ctx.Error == nil,
		Metadata: make(map[string]interface{}),
	}

	// 处理错误情况
	if ctx.Error != nil {
		result.Error = ctx.Error
		result.Message = ctx.Error.Message
		result.Success = false
	}

	// 处理返回数据
	if err := rh.processReturnData(result, returnData, expectedTypes); err != nil {
		return nil, fmt.Errorf("处理返回数据失败: %w", err)
	}

	// 计算资源消耗
	result.Consumed = rh.calculateResourceConsumption(ctx)

	// 生成元数据
	rh.generateMetadata(result, ctx)

	// 计算性能指标
	if rh.options.CollectDetailedStats {
		result.Performance = rh.calculatePerformanceMetrics(ctx)
	}

	// 验证结果
	if rh.options.StrictMode {
		if err := rh.validateResult(result); err != nil {
			return nil, fmt.Errorf("结果验证失败: %w", err)
		}
	}

	return result, nil
}

// processReturnData 处理返回数据
func (rh *ResultHandler) processReturnData(
	result *ProcessedResult,
	returnData []byte,
	expectedTypes []types.ValueType,
) error {
	// 设置原始返回数据
	if len(returnData) > 0 {
		if len(returnData) > rh.options.MaxReturnDataSize {
			return fmt.Errorf("返回数据大小超过限制: %d > %d",
				len(returnData), rh.options.MaxReturnDataSize)
		}
		result.ReturnData = returnData
	}

	// 解析返回值
	if len(expectedTypes) > 0 {
		parsedValues := make([]interface{}, 0, len(expectedTypes))

		for i, valueType := range expectedTypes {
			// 根据类型解析返回数据的相应部分
			var value interface{}
			var err error

			if len(returnData) > 0 {
				// 从返回数据中解析
				value, err = rh.decoder.DecodeReturnData(returnData, valueType)
				if err != nil && rh.options.AutoTypeConversion {
					// 尝试自动类型转换
					value, err = rh.attemptTypeConversion(returnData, valueType)
				}
				if err != nil {
					return fmt.Errorf("解析返回值[%d]失败: %w", i, err)
				}
			} else {
				// 返回默认值
				value = rh.getDefaultValue(valueType)
			}

			parsedValues = append(parsedValues, value)
		}

		result.ParsedValues = parsedValues
	}

	return nil
}

// attemptTypeConversion 尝试类型转换
func (rh *ResultHandler) attemptTypeConversion(data []byte, targetType types.ValueType) (interface{}, error) {
	if len(data) == 0 {
		return rh.getDefaultValue(targetType), nil
	}

	// 尝试将数据解释为不同类型
	switch targetType {
	case types.ValueTypeI32:
		if len(data) >= 4 {
			return int32(rh.decoder.byteOrder.Uint32(data[:4])), nil
		}
		if len(data) >= 1 {
			return int32(data[0]), nil
		}

	case types.ValueTypeI64:
		if len(data) >= 8 {
			return int64(rh.decoder.byteOrder.Uint64(data[:8])), nil
		}
		if len(data) >= 4 {
			return int64(rh.decoder.byteOrder.Uint32(data[:4])), nil
		}

	case types.ValueTypeF32:
		if len(data) >= 4 {
			bits := rh.decoder.byteOrder.Uint32(data[:4])
			return float32(bits), nil
		}

	case types.ValueTypeF64:
		if len(data) >= 8 {
			bits := rh.decoder.byteOrder.Uint64(data[:8])
			return float64(bits), nil
		}
	}

	return nil, fmt.Errorf("无法转换数据到类型 %s", targetType)
}

// getDefaultValue 获取类型默认值
func (rh *ResultHandler) getDefaultValue(valueType types.ValueType) interface{} {
	switch valueType {
	case types.ValueTypeI32:
		return int32(0)
	case types.ValueTypeI64:
		return int64(0)
	case types.ValueTypeF32:
		return float32(0.0)
	case types.ValueTypeF64:
		return float64(0.0)
	default:
		return nil
	}
}

// calculateResourceConsumption 计算资源消耗
func (rh *ResultHandler) calculateResourceConsumption(ctx *ExecutionContext) *ResourceConsumption {
	executionTime := ctx.EndTime.Sub(ctx.StartTime)

	consumption := &ResourceConsumption{
		Resource:      ctx.ResourceUsed,
		Memory:        ctx.MemoryUsed,
		ExecutionTime: executionTime.Milliseconds(),
		Instructions:  ctx.InstructionCount,
		FunctionCalls: ctx.FunctionCalls,
		HostCalls:     ctx.HostCalls,
	}

	// 计算CPU使用率（简化估算）
	if executionTime > 0 {
		consumption.CPUUsage = float64(ctx.InstructionCount) / float64(executionTime.Microseconds()) * 100
		if consumption.CPUUsage > 100 {
			consumption.CPUUsage = 100
		}
	}

	return consumption
}

// generateMetadata 生成元数据
func (rh *ResultHandler) generateMetadata(result *ProcessedResult, ctx *ExecutionContext) {
	metadata := result.Metadata

	// 基本信息
	metadata["executionId"] = ctx.ExecutionID
	metadata["startTime"] = ctx.StartTime.Format(time.RFC3339)
	metadata["endTime"] = ctx.EndTime.Format(time.RFC3339)
	metadata["duration"] = ctx.EndTime.Sub(ctx.StartTime).String()

	// 资源信息
	metadata["ResourceUtilization"] = float64(ctx.ResourceUsed) / float64(ctx.ExecutionFeeLimit) * 100
	metadata["memoryUtilization"] = float64(ctx.MemoryUsed) / float64(ctx.MemoryLimit) * 100
	metadata["peakMemory"] = ctx.PeakMemory

	// 执行统计
	metadata["instructionCount"] = ctx.InstructionCount
	metadata["functionCalls"] = ctx.FunctionCalls
	metadata["hostCalls"] = ctx.HostCalls

	// 警告信息
	if len(ctx.Warnings) > 0 {
		metadata["warnings"] = ctx.Warnings
	}

	// 调试信息
	if len(ctx.DebugInfo) > 0 {
		metadata["debug"] = ctx.DebugInfo
	}

	// 处理器信息
	metadata["resultHandler"] = map[string]interface{}{
		"version":     "1.0",
		"strictMode":  rh.options.StrictMode,
		"processTime": time.Now().Format(time.RFC3339),
	}

	// 限制元数据条目数量
	if len(metadata) > rh.options.MaxMetadataEntries {
		// 保留最重要的元数据
		essential := map[string]interface{}{
			"executionId":       metadata["executionId"],
			"duration":          metadata["duration"],
			"资源Utilization":     metadata["资源Utilization"],
			"memoryUtilization": metadata["memoryUtilization"],
			"instructionCount":  metadata["instructionCount"],
		}
		result.Metadata = essential
	}
}

// calculatePerformanceMetrics 计算性能指标
func (rh *ResultHandler) calculatePerformanceMetrics(ctx *ExecutionContext) *PerformanceMetrics {
	totalTime := ctx.EndTime.Sub(ctx.StartTime)

	metrics := &PerformanceMetrics{
		ExecutionTime: totalTime,
	}

	// 计算吞吐量指标
	if totalTime > 0 {
		seconds := totalTime.Seconds()
		metrics.InstructionsPerSecond = uint64(float64(ctx.InstructionCount) / seconds)
		metrics.ResourcePerSecond = uint64(float64(ctx.ResourceUsed) / seconds)
	}

	// 计算内存效率
	if ctx.MemoryLimit > 0 {
		metrics.MemoryEfficiency = float64(ctx.MemoryUsed) / float64(ctx.MemoryLimit)
	}

	// 计算资源利用率
	if ctx.ExecutionFeeLimit > 0 {
		metrics.ResourceUtilization = float64(ctx.ResourceUsed) / float64(ctx.ExecutionFeeLimit)
	}

	if ctx.MemoryLimit > 0 {
		metrics.MemoryUtilization = float64(ctx.PeakMemory) / float64(ctx.MemoryLimit)
	}

	return metrics
}

// validateResult 验证结果
func (rh *ResultHandler) validateResult(result *ProcessedResult) error {
	// 验证必要字段
	if result.Consumed == nil {
		return fmt.Errorf("缺少资源消耗信息")
	}

	if result.Metadata == nil {
		return fmt.Errorf("缺少元数据信息")
	}

	// 验证返回数据大小
	if len(result.ReturnData) > rh.options.MaxReturnDataSize {
		return fmt.Errorf("返回数据大小超过限制")
	}

	// 验证元数据条目数量
	if len(result.Metadata) > rh.options.MaxMetadataEntries {
		return fmt.Errorf("元数据条目数量超过限制")
	}

	// 验证数据一致性
	if result.Success && result.Error != nil {
		return fmt.Errorf("成功状态与错误信息不一致")
	}

	if !result.Success && result.Error == nil {
		return fmt.Errorf("失败状态缺少错误信息")
	}

	return nil
}

// ConvertToExecutionResult 转换为标准执行结果
func (rh *ResultHandler) ConvertToExecutionResult(processed *ProcessedResult) *types.ExecutionResult {
	result := &types.ExecutionResult{
		Success:    processed.Success,
		ReturnData: processed.ReturnData,
		Consumed:   processed.Consumed.Resource,
		Metadata:   processed.Metadata,
	}

	// 添加额外的消耗信息到元数据
	if processed.Consumed != nil {
		result.Metadata["consumption"] = map[string]interface{}{
			"resource":      processed.Consumed.Resource,
			"memory":        processed.Consumed.Memory,
			"executionTime": processed.Consumed.ExecutionTime,
			"instructions":  processed.Consumed.Instructions,
			"functionCalls": processed.Consumed.FunctionCalls,
			"hostCalls":     processed.Consumed.HostCalls,
		}
	}

	// 添加性能指标到元数据
	if processed.Performance != nil {
		perfData, _ := json.Marshal(processed.Performance)
		var perfMap map[string]interface{}
		json.Unmarshal(perfData, &perfMap)
		result.Metadata["performance"] = perfMap
	}

	// 添加解析后的值到元数据
	if len(processed.ParsedValues) > 0 {
		result.Metadata["parsedValues"] = processed.ParsedValues
	}

	return result
}

// CreateExecutionContext 创建执行上下文
func CreateExecutionContext(executionID string, ExecutionFeeLimit uint64, memoryLimit uint32) *ExecutionContext {
	return &ExecutionContext{
		ExecutionID:       executionID,
		StartTime:         time.Now(),
		ExecutionFeeLimit: ExecutionFeeLimit,
		MemoryLimit:       memoryLimit,
		DebugInfo:         make(map[string]interface{}),
	}
}

// UpdateExecutionContext 更新执行上下文
func (ctx *ExecutionContext) UpdateExecutionContext(resourceUsed uint64, memoryUsed uint32, instructionCount uint64) {
	ctx.ResourceUsed = resourceUsed
	ctx.MemoryUsed = memoryUsed
	ctx.InstructionCount = instructionCount

	// 更新峰值内存
	if memoryUsed > ctx.PeakMemory {
		ctx.PeakMemory = memoryUsed
	}
}

// FinishExecution 完成执行
func (ctx *ExecutionContext) FinishExecution(err *WASMError) {
	ctx.EndTime = time.Now()
	ctx.Error = err
}

// AddWarning 添加警告
func (ctx *ExecutionContext) AddWarning(warning string) {
	ctx.Warnings = append(ctx.Warnings, warning)
}

// AddDebugInfo 添加调试信息
func (ctx *ExecutionContext) AddDebugInfo(key string, value interface{}) {
	if ctx.DebugInfo == nil {
		ctx.DebugInfo = make(map[string]interface{})
	}
	ctx.DebugInfo[key] = value
}

// defaultResultHandlerOptions 默认结果处理器选项
func defaultResultHandlerOptions() *ResultHandlerOptions {
	return &ResultHandlerOptions{
		StrictMode:           true,
		MaxReturnDataSize:    1024 * 1024, // 1MB
		MaxMetadataEntries:   50,
		AutoTypeConversion:   true,
		CollectDetailedStats: true,
	}
}
