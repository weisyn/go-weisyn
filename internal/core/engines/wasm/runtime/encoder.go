package runtime

import (
	"encoding/binary"
	"fmt"
	"math"
	"unsafe"

	types "github.com/weisyn/v1/pkg/types"
)

// ParameterEncoder WASM参数编码器
// 负责将Go类型的参数转换为WASM虚拟机可以理解的格式
type ParameterEncoder struct {
	// 字节序
	byteOrder binary.ByteOrder

	// 编码选项
	options *EncodingOptions
}

// EncodingOptions 编码选项
type EncodingOptions struct {
	// 是否启用类型检查
	EnableTypeCheck bool `json:"enableTypeCheck"`

	// 是否使用小端字节序
	LittleEndian bool `json:"littleEndian"`

	// 最大参数数量
	MaxParams int `json:"maxParams"`

	// 最大参数大小（字节）
	MaxParamSize int `json:"maxParamSize"`
}

// ParameterDecoder WASM返回值解码器
// 负责将WASM虚拟机的返回值转换为Go类型
type ParameterDecoder struct {
	// 字节序
	byteOrder binary.ByteOrder

	// 解码选项
	options *DecodingOptions
}

// DecodingOptions 解码选项
type DecodingOptions struct {
	// 是否启用类型检查
	EnableTypeCheck bool `json:"enableTypeCheck"`

	// 是否使用小端字节序
	LittleEndian bool `json:"littleEndian"`

	// 最大返回值数量
	MaxResults int `json:"maxResults"`

	// 最大返回值大小（字节）
	MaxResultSize int `json:"maxResultSize"`
}

// EncodingResult 编码结果
type EncodingResult struct {
	// 编码后的参数
	EncodedParams []uint64 `json:"encodedParams"`

	// 参数类型
	ParamTypes []types.ValueType `json:"paramTypes"`

	// 编码元数据
	Metadata map[string]interface{} `json:"metadata"`
}

// DecodingResult 解码结果
type DecodingResult struct {
	// 解码后的返回值
	DecodedValues []interface{} `json:"decodedValues"`

	// 返回值类型
	ResultTypes []types.ValueType `json:"resultTypes"`

	// 解码元数据
	Metadata map[string]interface{} `json:"metadata"`
}

// NewParameterEncoder 创建参数编码器
func NewParameterEncoder(options *EncodingOptions) *ParameterEncoder {
	if options == nil {
		options = defaultEncodingOptions()
	}

	var byteOrder binary.ByteOrder = binary.BigEndian
	if options.LittleEndian {
		byteOrder = binary.LittleEndian
	}

	return &ParameterEncoder{
		byteOrder: byteOrder,
		options:   options,
	}
}

// NewParameterDecoder 创建参数解码器
func NewParameterDecoder(options *DecodingOptions) *ParameterDecoder {
	if options == nil {
		options = defaultDecodingOptions()
	}

	var byteOrder binary.ByteOrder = binary.BigEndian
	if options.LittleEndian {
		byteOrder = binary.LittleEndian
	}

	return &ParameterDecoder{
		byteOrder: byteOrder,
		options:   options,
	}
}

// EncodeParameters 编码参数
func (pe *ParameterEncoder) EncodeParameters(params []interface{}) (*EncodingResult, error) {
	if len(params) > pe.options.MaxParams {
		return nil, fmt.Errorf("参数数量超过限制: %d > %d", len(params), pe.options.MaxParams)
	}

	result := &EncodingResult{
		EncodedParams: make([]uint64, 0, len(params)),
		ParamTypes:    make([]types.ValueType, 0, len(params)),
		Metadata:      make(map[string]interface{}),
	}

	for i, param := range params {
		encoded, valueType, err := pe.encodeParameter(param)
		if err != nil {
			return nil, fmt.Errorf("编码参数[%d]失败: %w", i, err)
		}

		result.EncodedParams = append(result.EncodedParams, encoded)
		result.ParamTypes = append(result.ParamTypes, valueType)
	}

	// 添加元数据
	result.Metadata["paramCount"] = len(params)
	result.Metadata["byteOrder"] = pe.getByteOrderString()
	result.Metadata["encodingVersion"] = "1.0"

	return result, nil
}

// encodeParameter 编码单个参数
func (pe *ParameterEncoder) encodeParameter(param interface{}) (uint64, types.ValueType, error) {
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
		// 字符串作为指针+长度传递
		return pe.encodeString(v)
	case []byte:
		// 字节数组作为指针+长度传递
		return pe.encodeBytes(v)
	default:
		if pe.options.EnableTypeCheck {
			return 0, "", fmt.Errorf("不支持的参数类型: %T", param)
		}
		// 尝试转换为int64
		if val, ok := pe.tryConvertToInt64(param); ok {
			return uint64(val), types.ValueTypeI64, nil
		}
		return 0, "", fmt.Errorf("无法编码参数类型: %T", param)
	}
}

// encodeString 编码字符串
func (pe *ParameterEncoder) encodeString(s string) (uint64, types.ValueType, error) {
	bytes := []byte(s)
	if len(bytes) > pe.options.MaxParamSize {
		return 0, "", fmt.Errorf("字符串大小超过限制: %d > %d", len(bytes), pe.options.MaxParamSize)
	}

	// 简化实现：返回长度作为值
	// 实际实现中需要在WASM内存中分配空间并返回指针
	return uint64(len(bytes)), types.ValueTypeI32, nil
}

// encodeBytes 编码字节数组
func (pe *ParameterEncoder) encodeBytes(bytes []byte) (uint64, types.ValueType, error) {
	if len(bytes) > pe.options.MaxParamSize {
		return 0, "", fmt.Errorf("字节数组大小超过限制: %d > %d", len(bytes), pe.options.MaxParamSize)
	}

	// 简化实现：返回长度作为值
	// 实际实现中需要在WASM内存中分配空间并返回指针
	return uint64(len(bytes)), types.ValueTypeI32, nil
}

// tryConvertToInt64 尝试转换为int64
func (pe *ParameterEncoder) tryConvertToInt64(param interface{}) (int64, bool) {
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
	case uintptr:
		return int64(v), true
	default:
		return 0, false
	}
}

// getByteOrderString 获取字节序字符串
func (pe *ParameterEncoder) getByteOrderString() string {
	if pe.byteOrder == binary.LittleEndian {
		return "little"
	}
	return "big"
}

// DecodeResults 解码返回值
func (pd *ParameterDecoder) DecodeResults(rawResults []uint64, resultTypes []types.ValueType) (*DecodingResult, error) {
	if len(rawResults) != len(resultTypes) {
		return nil, fmt.Errorf("返回值数量与类型数量不匹配: %d != %d", len(rawResults), len(resultTypes))
	}

	if len(rawResults) > pd.options.MaxResults {
		return nil, fmt.Errorf("返回值数量超过限制: %d > %d", len(rawResults), pd.options.MaxResults)
	}

	result := &DecodingResult{
		DecodedValues: make([]interface{}, 0, len(rawResults)),
		ResultTypes:   resultTypes,
		Metadata:      make(map[string]interface{}),
	}

	for i, rawValue := range rawResults {
		decoded, err := pd.decodeResult(rawValue, resultTypes[i])
		if err != nil {
			return nil, fmt.Errorf("解码返回值[%d]失败: %w", i, err)
		}

		result.DecodedValues = append(result.DecodedValues, decoded)
	}

	// 添加元数据
	result.Metadata["resultCount"] = len(rawResults)
	result.Metadata["byteOrder"] = pd.getByteOrderString()
	result.Metadata["decodingVersion"] = "1.0"

	return result, nil
}

// decodeResult 解码单个返回值
func (pd *ParameterDecoder) decodeResult(rawValue uint64, valueType types.ValueType) (interface{}, error) {
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
		if pd.options.EnableTypeCheck {
			return nil, fmt.Errorf("不支持的返回值类型: %s", valueType)
		}
		// 默认返回原始值
		return rawValue, nil
	}
}

// getByteOrderString 获取字节序字符串
func (pd *ParameterDecoder) getByteOrderString() string {
	if pd.byteOrder == binary.LittleEndian {
		return "little"
	}
	return "big"
}

// DecodeReturnData 解码ReturnData
func (pd *ParameterDecoder) DecodeReturnData(returnData []byte, expectedType types.ValueType) (interface{}, error) {
	if len(returnData) == 0 {
		return nil, nil
	}

	switch expectedType {
	case types.ValueTypeI32:
		if len(returnData) < 4 {
			return nil, fmt.Errorf("返回数据长度不足，期望4字节，实际%d字节", len(returnData))
		}
		return int32(pd.byteOrder.Uint32(returnData[:4])), nil

	case types.ValueTypeI64:
		if len(returnData) < 8 {
			return nil, fmt.Errorf("返回数据长度不足，期望8字节，实际%d字节", len(returnData))
		}
		return int64(pd.byteOrder.Uint64(returnData[:8])), nil

	case types.ValueTypeF32:
		if len(returnData) < 4 {
			return nil, fmt.Errorf("返回数据长度不足，期望4字节，实际%d字节", len(returnData))
		}
		bits := pd.byteOrder.Uint32(returnData[:4])
		return math.Float32frombits(bits), nil

	case types.ValueTypeF64:
		if len(returnData) < 8 {
			return nil, fmt.Errorf("返回数据长度不足，期望8字节，实际%d字节", len(returnData))
		}
		bits := pd.byteOrder.Uint64(returnData[:8])
		return math.Float64frombits(bits), nil

	default:
		// 默认返回字节数组
		return returnData, nil
	}
}

// EncodeForMemory 编码数据到WASM内存
func (pe *ParameterEncoder) EncodeForMemory(data interface{}) ([]byte, error) {
	switch v := data.(type) {
	case int32:
		buf := make([]byte, 4)
		pe.byteOrder.PutUint32(buf, uint32(v))
		return buf, nil

	case uint32:
		buf := make([]byte, 4)
		pe.byteOrder.PutUint32(buf, v)
		return buf, nil

	case int64:
		buf := make([]byte, 8)
		pe.byteOrder.PutUint64(buf, uint64(v))
		return buf, nil

	case uint64:
		buf := make([]byte, 8)
		pe.byteOrder.PutUint64(buf, v)
		return buf, nil

	case float32:
		buf := make([]byte, 4)
		pe.byteOrder.PutUint32(buf, math.Float32bits(v))
		return buf, nil

	case float64:
		buf := make([]byte, 8)
		pe.byteOrder.PutUint64(buf, math.Float64bits(v))
		return buf, nil

	case string:
		return []byte(v), nil

	case []byte:
		return v, nil

	default:
		return nil, fmt.Errorf("不支持的数据类型: %T", data)
	}
}

// defaultEncodingOptions 默认编码选项
func defaultEncodingOptions() *EncodingOptions {
	return &EncodingOptions{
		EnableTypeCheck: true,
		LittleEndian:    true, // WASM使用小端字节序
		MaxParams:       16,
		MaxParamSize:    1024 * 1024, // 1MB
	}
}

// defaultDecodingOptions 默认解码选项
func defaultDecodingOptions() *DecodingOptions {
	return &DecodingOptions{
		EnableTypeCheck: true,
		LittleEndian:    true, // WASM使用小端字节序
		MaxResults:      16,
		MaxResultSize:   1024 * 1024, // 1MB
	}
}

// MemoryHelper 内存操作辅助工具
type MemoryHelper struct {
	encoder *ParameterEncoder
	decoder *ParameterDecoder
}

// NewMemoryHelper 创建内存辅助工具
func NewMemoryHelper() *MemoryHelper {
	return &MemoryHelper{
		encoder: NewParameterEncoder(nil),
		decoder: NewParameterDecoder(nil),
	}
}

// WriteToMemory 写入数据到WASM内存
func (mh *MemoryHelper) WriteToMemory(memory []byte, offset uint32, data interface{}) error {
	encoded, err := mh.encoder.EncodeForMemory(data)
	if err != nil {
		return fmt.Errorf("编码数据失败: %w", err)
	}

	if int(offset)+len(encoded) > len(memory) {
		return fmt.Errorf("内存越界: offset=%d, size=%d, memory=%d", offset, len(encoded), len(memory))
	}

	copy(memory[offset:], encoded)
	return nil
}

// ReadFromMemory 从WASM内存读取数据
func (mh *MemoryHelper) ReadFromMemory(memory []byte, offset uint32, size uint32, valueType types.ValueType) (interface{}, error) {
	if int(offset)+int(size) > len(memory) {
		return nil, fmt.Errorf("内存越界: offset=%d, size=%d, memory=%d", offset, size, len(memory))
	}

	data := memory[offset : offset+size]
	return mh.decoder.DecodeReturnData(data, valueType)
}

// GetPointerValue 获取指针值（用于unsafe操作）
func (mh *MemoryHelper) GetPointerValue(ptr interface{}) (uint64, error) {
	switch v := ptr.(type) {
	case *int32:
		return uint64(uintptr(unsafe.Pointer(v))), nil
	case *int64:
		return uint64(uintptr(unsafe.Pointer(v))), nil
	case *float32:
		return uint64(uintptr(unsafe.Pointer(v))), nil
	case *float64:
		return uint64(uintptr(unsafe.Pointer(v))), nil
	case *string:
		return uint64(uintptr(unsafe.Pointer(v))), nil
	case *[]byte:
		return uint64(uintptr(unsafe.Pointer(v))), nil
	default:
		return 0, fmt.Errorf("不支持的指针类型: %T", ptr)
	}
}
