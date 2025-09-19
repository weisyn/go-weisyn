package abi

import (
	"encoding/json"

	iface "github.com/weisyn/v1/internal/core/execution/interfaces"
	typespkg "github.com/weisyn/v1/pkg/types"
)

// defaultEncoder 默认编码器的非导出实现。
//
// 实现 interfaces.Encoder 接口，提供基于JSON的ABI编码功能。
// 该实现为简化版本，适用于开发和测试环境。生产环境可根据需要
// 替换为更高效的编码实现（如Protobuf、MessagePack等）。
//
// 设计特点：
//   - 基于JSON编码，具有良好的可读性和调试性
//   - 支持任意类型的参数编码
//   - 保持函数签名信息，便于调试
//   - 错误处理简单直接
type defaultEncoder struct{}

// newDefaultEncoder 创建默认编码器实例。
//
// 返回值：
//   - iface.Encoder: 编码器接口实例
func newDefaultEncoder() iface.Encoder {
	return &defaultEncoder{}
}

// EncodeFunctionCall 编码函数调用为字节序列。
//
// 将函数定义和参数组合编码为JSON格式的字节序列，包含函数名和参数信息。
// 编码结果可用于网络传输或存储。
//
// 参数：
//   - fn: 合约函数定义，包含函数名、参数类型等信息
//   - args: 函数调用参数列表，按ABI定义顺序排列
//
// 返回值：
//   - []byte: 编码后的字节序列
//   - error: 编码过程中的错误，nil表示成功
//
// 编码格式：
//
//	{"function": "函数名", "args": [参数列表]}
func (e *defaultEncoder) EncodeFunctionCall(fn *typespkg.ContractFunction, args []interface{}) ([]byte, error) {
	data := map[string]interface{}{
		"function": fn.Name,
		"args":     args,
	}
	return json.Marshal(data)
}

// EncodeParameters 编码参数列表为字节序列。
//
// 将参数定义和实际值组合编码为JSON格式，用于参数的独立编码场景。
//
// 参数：
//   - params: ABI参数定义列表，描述参数的类型和名称
//   - args: 实际参数值列表，与参数定义一一对应
//
// 返回值：
//   - []byte: 编码后的字节序列
//   - error: 编码过程中的错误，nil表示成功
//
// 编码格式：
//
//	{"params": [参数定义], "args": [参数值]}
func (e *defaultEncoder) EncodeParameters(params []typespkg.ABIParam, args []interface{}) ([]byte, error) {
	data := map[string]interface{}{
		"params": params,
		"args":   args,
	}
	return json.Marshal(data)
}

// EncodeValue 编码单个值为字节序列。
//
// 将单个值根据指定的类型编码为JSON格式，用于简单值的编码场景。
//
// 参数：
//   - paramType: 参数类型字符串（当前实现中未使用，但保留接口兼容性）
//   - value: 待编码的值，可以是任意JSON可序列化的类型
//
// 返回值：
//   - []byte: 编码后的字节序列
//   - error: 编码过程中的错误，nil表示成功
func (e *defaultEncoder) EncodeValue(_ string, value interface{}) ([]byte, error) {
	return json.Marshal(value)
}

// defaultDecoder 默认解码器的非导出实现。
//
// 实现 interfaces.Decoder 接口，提供基于JSON的ABI解码功能。
// 与defaultEncoder配对使用，支持编码数据的完整解码。
//
// 设计特点：
//   - 基于JSON解码，与编码器对称
//   - 支持任意类型的结果解码
//   - 容错性较好，支持基本的类型推断
//   - 返回通用接口类型，便于上层处理
type defaultDecoder struct{}

// newDefaultDecoder 创建默认解码器实例。
//
// 返回值：
//   - iface.Decoder: 解码器接口实例
func newDefaultDecoder() iface.Decoder {
	return &defaultDecoder{}
}

// DecodeFunctionResult 解码函数返回值。
//
// 将字节序列解码为函数返回值列表，支持多返回值的函数。
//
// 参数：
//   - fn: 合约函数定义（当前实现中未使用，但保留接口兼容性）
//   - data: 待解码的字节序列，应为JSON格式的数组
//
// 返回值：
//   - []interface{}: 解码后的返回值列表
//   - error: 解码过程中的错误，nil表示成功
//
// 支持的格式：
//   - JSON数组：[value1, value2, ...]
//   - 自动推断基本类型（数字、字符串、布尔值等）
func (d *defaultDecoder) DecodeFunctionResult(_ *typespkg.ContractFunction, data []byte) ([]interface{}, error) {
	var result []interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DecodeParameters 解码参数列表。
//
// 将字节序列解码为参数值列表，用于参数的独立解码场景。
//
// 参数：
//   - params: ABI参数定义列表（当前实现中未使用，但保留接口兼容性）
//   - data: 待解码的字节序列，应为JSON格式的数组
//
// 返回值：
//   - []interface{}: 解码后的参数值列表
//   - error: 解码过程中的错误，nil表示成功
func (d *defaultDecoder) DecodeParameters(_ []typespkg.ABIParam, data []byte) ([]interface{}, error) {
	var result []interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DecodeValue 解码单个值。
//
// 将字节序列解码为单个值，用于简单值的解码场景。
//
// 参数：
//   - paramType: 参数类型字符串（当前实现中未使用，但保留接口兼容性）
//   - data: 待解码的字节序列，应为有效的JSON格式
//
// 返回值：
//   - interface{}: 解码后的值，类型由JSON内容决定
//   - error: 解码过程中的错误，nil表示成功
//
// 支持的类型：
//   - 基本类型：number、string、boolean、null
//   - 复合类型：array、object
func (d *defaultDecoder) DecodeValue(_ string, data []byte) (interface{}, error) {
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
