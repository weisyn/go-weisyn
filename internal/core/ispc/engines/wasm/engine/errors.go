// Package engine provides error definitions for WASM engine operations.
package engine

import (
	"errors"
	"fmt"
)

// ============================================================================
//                               WASM引擎错误定义
// ============================================================================

var (
	// ErrEngineNotInitialized 引擎未初始化错误
	ErrEngineNotInitialized = errors.New("WASM engine not initialized")

	// ErrContractLoadFailed 合约加载失败错误
	ErrContractLoadFailed = errors.New("contract loading failed")

	// ErrContractCompileFailed 合约编译失败错误
	ErrContractCompileFailed = errors.New("contract compilation failed")

	// ErrInstanceCreationFailed 实例创建失败错误
	ErrInstanceCreationFailed = errors.New("WASM instance creation failed")

	// ErrFunctionNotFound 函数未找到错误
	ErrFunctionNotFound = errors.New("function not found in WASM module")

	// ErrFunctionExecutionFailed 函数执行失败错误
	ErrFunctionExecutionFailed = errors.New("function execution failed")

	// ErrInvalidParameters 参数无效错误
	ErrInvalidParameters = errors.New("invalid parameters for function call")

	// ErrHostFunctionRegistrationFailed 宿主函数注册失败错误
	ErrHostFunctionRegistrationFailed = errors.New("host function registration failed")

	// ErrMemoryAccessFailed 内存访问失败错误
	ErrMemoryAccessFailed = errors.New("WASM memory access failed")

	// ErrEngineShutdown 引擎关闭错误
	ErrEngineShutdown = errors.New("WASM engine shutdown failed")

	// ErrInvalidWASMBytes WASM字节码无效错误
	ErrInvalidWASMBytes = errors.New("invalid WASM bytecode")

	// ErrResourceExhausted 资源耗尽错误
	ErrResourceExhausted = errors.New("WASM execution resource exhausted")

	// ErrTimeout 执行超时错误
	ErrTimeout = errors.New("WASM execution timeout")
)

// ============================================================================
//                               错误包装函数
// ============================================================================

// WrapContractLoadFailedError 包装合约加载失败错误
func WrapContractLoadFailedError(contractAddress string, err error) error {
	return fmt.Errorf("%w: address=%s, cause=%v", ErrContractLoadFailed, contractAddress, err)
}

// WrapContractCompileFailedError 包装合约编译失败错误
func WrapContractCompileFailedError(contractAddress string, err error) error {
	return fmt.Errorf("%w: address=%s, cause=%v", ErrContractCompileFailed, contractAddress, err)
}

// WrapInstanceCreationFailedError 包装实例创建失败错误
func WrapInstanceCreationFailedError(contractAddress string, err error) error {
	return fmt.Errorf("%w: address=%s, cause=%v", ErrInstanceCreationFailed, contractAddress, err)
}

// WrapFunctionNotFoundError 包装函数未找到错误
func WrapFunctionNotFoundError(contractAddress, functionName string) error {
	return fmt.Errorf("%w: address=%s, function=%s", ErrFunctionNotFound, contractAddress, functionName)
}

// WrapFunctionExecutionFailedError 包装函数执行失败错误
func WrapFunctionExecutionFailedError(contractAddress, functionName string, err error) error {
	return fmt.Errorf("%w: address=%s, function=%s, cause=%v", ErrFunctionExecutionFailed, contractAddress, functionName, err)
}

// WrapInvalidParametersError 包装参数无效错误
func WrapInvalidParametersError(functionName string, expectedCount, actualCount int) error {
	return fmt.Errorf("%w: function=%s, expected=%d, actual=%d", ErrInvalidParameters, functionName, expectedCount, actualCount)
}

// WrapHostFunctionRegistrationFailedError 包装宿主函数注册失败错误
func WrapHostFunctionRegistrationFailedError(functionName string, err error) error {
	return fmt.Errorf("%w: function=%s, cause=%v", ErrHostFunctionRegistrationFailed, functionName, err)
}

// WrapMemoryAccessFailedError 包装内存访问失败错误
func WrapMemoryAccessFailedError(operation string, address uint32, err error) error {
	return fmt.Errorf("%w: operation=%s, address=0x%x, cause=%v", ErrMemoryAccessFailed, operation, address, err)
}

// WrapInvalidWASMBytesError 包装WASM字节码无效错误
func WrapInvalidWASMBytesError(size int, err error) error {
	return fmt.Errorf("%w: size=%d bytes, cause=%v", ErrInvalidWASMBytes, size, err)
}

// WrapResourceExhaustedError 包装资源耗尽错误
func WrapResourceExhaustedError(resource string, limit interface{}) error {
	return fmt.Errorf("%w: resource=%s, limit=%v", ErrResourceExhausted, resource, limit)
}

// WrapTimeoutError 包装执行超时错误
func WrapTimeoutError(functionName string, timeoutMs int) error {
	return fmt.Errorf("%w: function=%s, timeout=%dms", ErrTimeout, functionName, timeoutMs)
}
