package execution

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"

	"github.com/weisyn/v1/pkg/types"
)

// ConvertToWASMParams 将通用参数转换为WASM引擎可接受的参数集合
// 允许的目标类型：int32、int64、float32、float64、string、[]byte
// 失败将返回详细错误，便于调用方定位问题。
func ConvertToWASMParams(params []interface{}) ([]interface{}, error) {
	if params == nil {
		return []interface{}{}, nil
	}
	out := make([]interface{}, len(params))
	for i, v := range params {
		vv, err := toAllowedScalar(v)
		if err != nil {
			return nil, fmt.Errorf("param[%d] invalid for WASM: %w", i, err)
		}
		out[i] = vv
	}
	return out, nil
}

// ConvertToONNXParams 将通用参数转换为ONNX引擎输入集合
// 说明：此处仅做标量校验与转换，张量/多维数组的构造在ONNX引擎适配层完成。
func ConvertToONNXParams(params []interface{}) ([]interface{}, error) {
	if params == nil {
		return []interface{}{}, nil
	}
	out := make([]interface{}, len(params))
	for i, v := range params {
		vv, err := toAllowedScalar(v)
		if err != nil {
			return nil, fmt.Errorf("param[%d] invalid for ONNX: %w", i, err)
		}
		out[i] = vv
	}
	return out, nil
}

// ConvertFromEngineResult 统一转换引擎返回值为上层可消费的类型
// 当前支持：[]byte、string、int32、int64、float32、float64
func ConvertFromEngineResult(result interface{}, engineType types.EngineType) (interface{}, error) {
	if result == nil {
		return nil, nil
	}
	switch r := result.(type) {
	case []byte, string, int32, int64, float32, float64:
		return r, nil
	default:
		return nil, fmt.Errorf("unsupported engine result type %T from %s", result, engineType)
	}
}

// toAllowedScalar 将任意输入转换为受支持的标量类型
func toAllowedScalar(v interface{}) (interface{}, error) {
	switch x := v.(type) {
	case int32:
		return x, nil
	case uint32:
		// WASM i32 无符号到有符号的安全映射（溢出由调用者保证）
		return int32(x), nil
	case int64:
		return x, nil
	case uint64:
		// 安全映射到有符号，调用侧应保证不超界
		return int64(x), nil
	case int:
		return int64(x), nil
	case uint:
		return int64(x), nil
	case float32:
		return x, nil
	case float64:
		return x, nil
	case string:
		return x, nil
	case []byte:
		return x, nil
	default:
		// 指针基础类型解引用
		switch px := x.(type) {
		case *int32:
			if px != nil {
				return *px, nil
			}
		case *uint32:
			if px != nil {
				return int32(*px), nil
			}
		case *int64:
			if px != nil {
				return *px, nil
			}
		case *uint64:
			if px != nil {
				return int64(*px), nil
			}
		case *int:
			if px != nil {
				return int64(*px), nil
			}
		case *uint:
			if px != nil {
				return int64(*px), nil
			}
		case *float32:
			if px != nil {
				return *px, nil
			}
		case *float64:
			if px != nil {
				return *px, nil
			}
		case *string:
			if px != nil {
				return *px, nil
			}
		case *[]byte:
			if px != nil {
				return *px, nil
			}
		}
		return nil, errors.New("unsupported param type")
	}
}

// ==================== 基础类型显式映射（WASM友好） ====================

// DetermineValueType 判定Go值对应的WASM值类型（仅标量）
func DetermineValueType(v interface{}) (types.ValueType, error) {
	switch v.(type) {
	case int32, uint32:
		return types.ValueTypeI32, nil
	case int64, uint64, int, uint:
		return types.ValueTypeI64, nil
	case float32:
		return types.ValueTypeF32, nil
	case float64:
		return types.ValueTypeF64, nil
	case string, []byte:
		// 字节与字符串需要走线性内存映射，不判定为数值值类型
		return "", fmt.Errorf("non-scalar type requires memory mapping")
	default:
		return "", fmt.Errorf("unsupported type %T", v)
	}
}

// ToWASMValue 将Go标量编码为适配运行时的uint64及其类型
// 说明：
// - i32: 低32位有效
// - i64: 64位整型
// - f32/f64: 使用IEEE754位模式编码
func ToWASMValue(v interface{}) (uint64, types.ValueType, error) {
	switch x := v.(type) {
	case int32:
		return uint64(uint32(x)), types.ValueTypeI32, nil
	case uint32:
		return uint64(x), types.ValueTypeI32, nil
	case int64:
		return uint64(x), types.ValueTypeI64, nil
	case uint64:
		return x, types.ValueTypeI64, nil
	case int:
		return uint64(int64(x)), types.ValueTypeI64, nil
	case uint:
		return uint64(x), types.ValueTypeI64, nil
	case float32:
		bits := math.Float32bits(x)
		return uint64(bits), types.ValueTypeF32, nil
	case float64:
		bits := math.Float64bits(x)
		return bits, types.ValueTypeF64, nil
	default:
		return 0, "", fmt.Errorf("unsupported scalar type %T", v)
	}
}

// FromWASMValue 将uint64+类型还原为Go标量
func FromWASMValue(val uint64, vt types.ValueType) (interface{}, error) {
	switch vt {
	case types.ValueTypeI32:
		return int32(uint32(val)), nil
	case types.ValueTypeI64:
		return int64(val), nil
	case types.ValueTypeF32:
		return math.Float32frombits(uint32(val)), nil
	case types.ValueTypeF64:
		return math.Float64frombits(val), nil
	default:
		return nil, fmt.Errorf("unsupported wasm value type: %s", vt)
	}
}

// ==================== 复杂类型（struct/array）序列化支持 ====================

// EncodeComplexParam 将复杂类型（struct/array/slice/map）序列化为JSON字节
// 返回的schemaHint用于标注编码方案（当前固定为"json"）
func EncodeComplexParam(v interface{}) ([]byte, string, error) {
	if v == nil {
		return nil, "json", nil
	}
	k := reflect.ValueOf(v).Kind()
	switch k {
	case reflect.Struct, reflect.Map, reflect.Slice, reflect.Array:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, "", fmt.Errorf("encode complex param failed: %w", err)
		}
		return b, "json", nil
	default:
		return nil, "", fmt.Errorf("not a complex type: %T", v)
	}
}

// DecodeComplexParam 将JSON字节反序列化到out（out必须为指针）
func DecodeComplexParam(data []byte, out interface{}) error {
	if out == nil {
		return fmt.Errorf("out is nil")
	}
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("out must be a non-nil pointer")
	}
	if len(data) == 0 {
		// 空数据视为零值
		rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode complex param failed: %w", err)
	}
	return nil
}
