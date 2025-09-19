package engine

import (
	"fmt"
	"math"
	"time"

	types "github.com/weisyn/v1/pkg/types"
)

// ExecutionContextInternal 引擎内部执行上下文
// 负责将对外的 ExecutionParams 映射为引擎可直接使用的参数与限制，并规划内存布局
type ExecutionContextInternal struct {
	// 原始入参
	Params types.ExecutionParams

	// 目标函数
	Function string

	// 编码后的参数（用于直接调用VM）
	EncodedArgs []uint64
	// 参数类型
	ArgTypes []types.ValueType
	// 期望返回类型（供解码使用）
	ReturnTypes []types.ValueType

	// 资源限制
	ExecutionFeeLimit    uint64
	MemoryLimit uint32
	Timeout     time.Duration

	// 内存映射规划（预估/规划阶段，不直接持有VM内存指针）
	MemoryPlan *MemoryMappingPlan

	// 附加元数据
	Metadata map[string]any

	// 本地编解码（为避免包循环，不直接依赖 runtime/encoder）
}

// MemoryMappingPlan 线性内存映射规划（不直接读写内存，仅描述布局计划）
type MemoryMappingPlan struct {
	TotalSize uint32
	Segments  []MemorySegment
}

// MemorySegment 线性内存片段
type MemorySegment struct {
	Offset uint32
	Size   uint32
	Kind   string // string/bytes/args/return/other
	Note   string
}

// BuildContext 从 ExecutionParams 构建内部上下文
func BuildContext(params types.ExecutionParams) (*ExecutionContextInternal, error) {
	if params.Entry == "" {
		return nil, fmt.Errorf("函数入口不能为空")
	}

	ctx := &ExecutionContextInternal{
		Params:      params,
		Function:    params.Entry,
		ExecutionFeeLimit:    params.ExecutionFeeLimit,
		MemoryLimit: params.MemoryLimit,
		Timeout:     time.Duration(params.Timeout) * time.Millisecond,
		Metadata:    make(map[string]any),
	}

	// 解析 Payload（调用方可以选择提前做ABI编码，这里尊重现有调用路径：
	// 若需要从 Context 中传参，可由上层负责拆解后经 MapArgs 注入）
	ctx.MemoryPlan = &MemoryMappingPlan{TotalSize: 0, Segments: make([]MemorySegment, 0)}
	return ctx, nil
}

// MapArgs 映射参数到内部编码格式
// 入参 args 为已解析的高阶类型（int32/int64/float32/float64/string/[]byte/bool等）
func (c *ExecutionContextInternal) MapArgs(args []interface{}) error {
	encoded := make([]uint64, 0, len(args))
	vtypes := make([]types.ValueType, 0, len(args))
	for i, a := range args {
		val, vt, err := encodeParameter(a)
		if err != nil {
			return fmt.Errorf("参数编码失败 arg[%d]: %w", i, err)
		}
		encoded = append(encoded, val)
		vtypes = append(vtypes, vt)
	}
	c.EncodedArgs = encoded
	c.ArgTypes = vtypes

	// 规划字符串/字节数组的内存片段（仅记录长度，实际指针由调用期内存写入时确定）
	var extra uint32
	for i := range args {
		switch args[i].(type) {
		case string:
			s := args[i].(string)
			if len(s) > 0 {
				c.MemoryPlan.Segments = append(c.MemoryPlan.Segments, MemorySegment{
					Offset: 0, // 待运行期确定
					Size:   uint32(len(s)),
					Kind:   "string",
					Note:   fmt.Sprintf("arg[%d]", i),
				})
				extra += uint32(len(s))
			}
		case []byte:
			b := args[i].([]byte)
			if len(b) > 0 {
				c.MemoryPlan.Segments = append(c.MemoryPlan.Segments, MemorySegment{
					Offset: 0,
					Size:   uint32(len(b)),
					Kind:   "bytes",
					Note:   fmt.Sprintf("arg[%d]", i),
				})
				extra += uint32(len(b))
			}
		}
	}
	c.MemoryPlan.TotalSize += extra
	return nil
}

// SetReturnTypes 设置期望返回值类型
func (c *ExecutionContextInternal) SetReturnTypes(vt []types.ValueType) {
	c.ReturnTypes = vt
}

// MapResult 将原始返回寄存器值与返回类型解码为高阶类型
func (c *ExecutionContextInternal) MapResult(raw []uint64) (*DecodingResult, error) {
	if len(raw) != len(c.ReturnTypes) {
		return nil, fmt.Errorf("返回值数量与类型数量不匹配: %d != %d", len(raw), len(c.ReturnTypes))
	}
	decoded := make([]interface{}, 0, len(raw))
	for i, r := range raw {
		v, err := decodeResult(r, c.ReturnTypes[i])
		if err != nil {
			return nil, fmt.Errorf("解码返回值[%d]失败: %w", i, err)
		}
		decoded = append(decoded, v)
	}
	return &DecodingResult{DecodedValues: decoded, ResultTypes: c.ReturnTypes, Metadata: map[string]interface{}{}}, nil
}

// WithLimits 设置资源限制
func (c *ExecutionContextInternal) WithLimits(资源 uint64, mem uint32, timeout time.Duration) *ExecutionContextInternal {
	if 资源 > 0 {
		c.ExecutionFeeLimit = 资源
	}
	if mem > 0 {
		c.MemoryLimit = mem
	}
	if timeout > 0 {
		c.Timeout = timeout
	}
	return c
}

// Validate 对上下文进行基本校验
func (c *ExecutionContextInternal) Validate() error {
	if c.Function == "" {
		return fmt.Errorf("函数入口不能为空")
	}
	if c.ExecutionFeeLimit == 0 {
		return fmt.Errorf("ExecutionFeeLimit 必须大于0")
	}
	if c.MemoryLimit == 0 {
		return fmt.Errorf("MemoryLimit 必须大于0")
	}
	return nil
}

// BuildArgMemoryPlan 基于已知参数估算线性内存需求（供实例调用前分配使用）
func (c *ExecutionContextInternal) BuildArgMemoryPlan(base uint32) *MemoryMappingPlan {
	// 这里仅返回当前规划；实际偏移可在实例化后结合实际内存页进行分配
	plan := *c.MemoryPlan
	for i := range plan.Segments {
		// 将基址写入注释，用于后续快速定位（不直接写Offset，避免误解为已分配）
		plan.Segments[i].Note = fmt.Sprintf("%s@base:%d", plan.Segments[i].Note, base)
	}
	return &plan
}

// BindArgumentsToInstance 将规划的字符串/字节参数写入实例内存，并将 EncodedArgs 中对应参数转为指针或长度
// 说明：对 string/[]byte 参数采用 "指针" 作为值（i32），长度通过紧随的参数或由宿主侧ABI约定获取
func (c *ExecutionContextInternal) BindArgumentsToInstance(inst *Instance, originalArgs []interface{}) error {
	if inst == nil || inst.Memory() == nil {
		return fmt.Errorf("instance or memory not available")
	}
	if len(originalArgs) != len(c.ArgTypes) {
		// 非致命，尽量继续，但建议调用侧保证一致
	}

	// 遍历原始参数，在内存中写入 string/[]byte，并回填 EncodedArgs 为指针
	argIdx := 0
	for i, a := range originalArgs {
		switch v := a.(type) {
		case string:
			ptr, _, err := inst.AllocateAndWriteBytes([]byte(v))
			if err != nil {
				return fmt.Errorf("write string arg[%d]: %w", i, err)
			}
			if argIdx < len(c.EncodedArgs) {
				c.EncodedArgs[argIdx] = uint64(ptr)
			}
		case []byte:
			ptr, _, err := inst.AllocateAndWriteBytes(v)
			if err != nil {
				return fmt.Errorf("write bytes arg[%d]: %w", i, err)
			}
			if argIdx < len(c.EncodedArgs) {
				c.EncodedArgs[argIdx] = uint64(ptr)
			}
		}
		argIdx++
	}
	return nil
}

// DecodingResult 与 runtime/encoder 中的结构保持语义一致（为避免循环依赖，局部定义）
type DecodingResult struct {
	DecodedValues []interface{}
	ResultTypes   []types.ValueType
	Metadata      map[string]interface{}
}

// encodeParameter 本地参数编码（与runtime保持一致的基本类型映射）
func encodeParameter(param interface{}) (uint64, types.ValueType, error) {
	switch v := param.(type) {
	case int32:
		return uint64(uint32(v)), types.ValueTypeI32, nil
	case uint32:
		return uint64(v), types.ValueTypeI32, nil
	case int64:
		return uint64(v), types.ValueTypeI64, nil
	case uint64:
		return v, types.ValueTypeI64, nil
	case float32:
		return uint64(math.Float32bits(v)), types.ValueTypeF32, nil
	case float64:
		return math.Float64bits(v), types.ValueTypeF64, nil
	case bool:
		if v {
			return 1, types.ValueTypeI32, nil
		}
		return 0, types.ValueTypeI32, nil
	case string:
		// 暂以长度承载（真实指针在实例期写入线性内存时确定）
		return uint64(len(v)), types.ValueTypeI32, nil
	case []byte:
		return uint64(len(v)), types.ValueTypeI32, nil
	default:
		// 尝试宽松整型转换
		if val, ok := tryConvertToInt64(param); ok {
			return uint64(val), types.ValueTypeI64, nil
		}
		return 0, "", fmt.Errorf("不支持的参数类型: %T", param)
	}
}

func tryConvertToInt64(param interface{}) (int64, bool) {
	switch v := param.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	default:
		return 0, false
	}
}

// decodeResult 将寄存器值按WASM类型解码为高阶类型
func decodeResult(rawValue uint64, valueType types.ValueType) (interface{}, error) {
	switch valueType {
	case types.ValueTypeI32:
		return int32(uint32(rawValue)), nil
	case types.ValueTypeI64:
		return int64(rawValue), nil
	case types.ValueTypeF32:
		return math.Float32frombits(uint32(rawValue)), nil
	case types.ValueTypeF64:
		return math.Float64frombits(rawValue), nil
	default:
		// 默认为原始值
		return rawValue, nil
	}
}
