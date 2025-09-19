package execution

import (
	"encoding/hex"
	"errors"
	"fmt"

	transaction "github.com/weisyn/v1/pb/blockchain/block/transaction"
	"github.com/weisyn/v1/pkg/types"
)

// ParseContractAddress 将字符串地址解析为 types.Address（pb别名）
// 支持格式：
// - weisyn:<hex> （标准前缀 + 十六进制主体）
// - <hex>      （纯十六进制主体）
// 校验：
// - 非空、长度在合理范围内
// - 十六进制可解析
func ParseContractAddress(addr string) (*types.Address, error) {
	if addr == "" {
		return nil, errors.New("address is empty")
	}

	raw := addr
	const prefix = "weisyn:"
	if len(raw) > len(prefix) && raw[:len(prefix)] == prefix {
		raw = raw[len(prefix):]
	}

	// 解析十六进制主体
	bytes, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid address hex: %w", err)
	}
	if len(bytes) == 0 {
		return nil, errors.New("address bytes empty")
	}

	// 组装PB地址，使用proto中定义的Address结构
	pbAddr := &transaction.Address{
		RawHash:     bytes,
		VersionByte: 1,
		AddressType: transaction.Address_P2PKH, // 默认为P2PKH类型
	}
	// 返回指针避免锁复制问题
	return (*types.Address)(pbAddr), nil
}

// 保留：后续扩展与合约相关的通用工具函数

// ExtractFunctionSignature 从ABI中提取给定方法的函数签名
func ExtractFunctionSignature(methodName string, abi *types.ContractABI) (*types.FunctionSignature, error) {
	if abi == nil {
		return nil, errors.New("abi is nil")
	}
	for _, f := range abi.Functions {
		if f.Name == methodName {
			sig := &types.FunctionSignature{
				Name:        f.Name,
				ParamTypes:  make([]string, len(f.Params)),
				ReturnTypes: make([]string, len(f.Returns)),
			}
			for i, p := range f.Params {
				sig.ParamTypes[i] = p.Type
			}
			for i, r := range f.Returns {
				sig.ReturnTypes[i] = r.Type
			}
			return sig, nil
		}
	}
	return nil, fmt.Errorf("method not found in ABI: %s", methodName)
}

// ValidateContractParams 基于ABI校验参数：
// - 若ABI仅包含一个函数，则按该函数校验；
// - 若包含多个函数，则返回错误，需指定方法名使用 ValidateContractParamsForMethod。
func ValidateContractParams(abi *types.ContractABI, params []interface{}) error {
	if abi == nil {
		return errors.New("abi is nil")
	}
	if len(abi.Functions) == 0 {
		return errors.New("abi has no functions")
	}
	if len(abi.Functions) > 1 {
		return errors.New("abi has multiple functions; use ValidateContractParamsForMethod with methodName")
	}
	return validateParamsAgainstFunction(abi.Functions[0], params)
}

// ValidateContractParamsForMethod 基于方法名与ABI校验参数
func ValidateContractParamsForMethod(abi *types.ContractABI, methodName string, params []interface{}) error {
	if abi == nil {
		return errors.New("abi is nil")
	}
	for _, f := range abi.Functions {
		if f.Name == methodName {
			return validateParamsAgainstFunction(f, params)
		}
	}
	return fmt.Errorf("method not found in ABI: %s", methodName)
}

// validateParamsAgainstFunction 校验参数数量与类型匹配
func validateParamsAgainstFunction(f types.ContractFunction, params []interface{}) error {
	if len(params) != len(f.Params) {
		return fmt.Errorf("param count mismatch: want %d, got %d", len(f.Params), len(params))
	}
	for i, p := range f.Params {
		abiType := p.Type
		gotType, ok := inferABIType(params[i])
		if !ok {
			return fmt.Errorf("param %d type not supported: %T", i, params[i])
		}
		if abiType != gotType {
			return fmt.Errorf("param %d type mismatch: want %s, got %s", i, abiType, gotType)
		}
	}
	return nil
}

// inferABIType 将Go值映射为ABI类型字符串
func inferABIType(v interface{}) (string, bool) {
	switch x := v.(type) {
	case int32:
		return "i32", true
	case uint32:
		return "i32", true
	case int64:
		return "i64", true
	case uint64:
		return "i64", true
	case float32:
		return "f32", true
	case float64:
		return "f64", true
	case string:
		return "string", true
	case []byte:
		return "bytes", true
	default:
		// 兼容指针基础类型
		switch px := x.(type) {
		case *int32:
			if px != nil {
				return "i32", true
			}
		case *uint32:
			if px != nil {
				return "i32", true
			}
		case *int64:
			if px != nil {
				return "i64", true
			}
		case *uint64:
			if px != nil {
				return "i64", true
			}
		case *float32:
			if px != nil {
				return "f32", true
			}
		case *float64:
			if px != nil {
				return "f64", true
			}
		case *string:
			if px != nil {
				return "string", true
			}
		case *[]byte:
			if px != nil {
				return "bytes", true
			}
		}
		return "", false
	}
}
