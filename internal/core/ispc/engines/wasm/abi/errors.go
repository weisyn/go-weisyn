// Package abi provides error definitions for WASM ABI operations.
package abi

import (
	"errors"
	"fmt"
)

// ============================================================================
//                               ABI服务错误定义
// ============================================================================

var (
	// ErrContractNotFound 合约未找到错误
	ErrContractNotFound = errors.New("contract not found")

	// ErrABINotRegistered ABI未注册错误
	ErrABINotRegistered = errors.New("ABI not registered for this contract")

	// ErrMethodNotFound 方法未找到错误
	ErrMethodNotFound = errors.New("method not found in ABI")

	// ErrInvalidParameters 参数无效错误
	ErrInvalidParameters = errors.New("invalid parameters for method")

	// ErrEncodingFailed 编码失败错误
	ErrEncodingFailed = errors.New("parameter encoding failed")

	// ErrDecodingFailed 解码失败错误
	ErrDecodingFailed = errors.New("result decoding failed")

	// ErrInvalidABI ABI格式无效错误
	ErrInvalidABI = errors.New("invalid ABI format")

	// ErrTypeConversion 类型转换错误
	ErrTypeConversion = errors.New("type conversion failed")
)

// ============================================================================
//                               错误包装函数
// ============================================================================

// WrapContractNotFoundError 包装合约未找到错误
func WrapContractNotFoundError(contractID string) error {
	return fmt.Errorf("%w: contractID=%s", ErrContractNotFound, contractID)
}

// WrapABINotRegisteredError 包装ABI未注册错误
func WrapABINotRegisteredError(contractID string) error {
	return fmt.Errorf("%w: contractID=%s", ErrABINotRegistered, contractID)
}

// WrapMethodNotFoundError 包装方法未找到错误
func WrapMethodNotFoundError(contractID, methodName string) error {
	return fmt.Errorf("%w: contractID=%s, method=%s", ErrMethodNotFound, contractID, methodName)
}

// WrapInvalidParametersError 包装参数无效错误
func WrapInvalidParametersError(contractID, methodName string, paramCount int) error {
	return fmt.Errorf("%w: contractID=%s, method=%s, paramCount=%d", ErrInvalidParameters, contractID, methodName, paramCount)
}

// WrapEncodingFailedError 包装编码失败错误
func WrapEncodingFailedError(contractID, methodName string, err error) error {
	return fmt.Errorf("%w: contractID=%s, method=%s, cause=%v", ErrEncodingFailed, contractID, methodName, err)
}

// WrapDecodingFailedError 包装解码失败错误
func WrapDecodingFailedError(contractID, methodName string, err error) error {
	return fmt.Errorf("%w: contractID=%s, method=%s, cause=%v", ErrDecodingFailed, contractID, methodName, err)
}

// WrapInvalidABIError 包装ABI格式无效错误
func WrapInvalidABIError(contractID string, err error) error {
	return fmt.Errorf("%w: contractID=%s, cause=%v", ErrInvalidABI, contractID, err)
}

// WrapTypeConversionError 包装类型转换错误
func WrapTypeConversionError(expectedType, actualType string) error {
	return fmt.Errorf("%w: expected=%s, actual=%s", ErrTypeConversion, expectedType, actualType)
}
