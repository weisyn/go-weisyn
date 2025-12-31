// Package coordinator provides error definitions for ISPC coordination operations.
package coordinator

import (
	"errors"
	"fmt"
)

// ============================================================================
//                            执行协调器错误定义
// ============================================================================

var (
	// ErrInvalidContractAddress 无效合约地址错误
	ErrInvalidContractAddress = errors.New("invalid contract address")

	// ErrInvalidContractHash 无效合约哈希错误
	ErrInvalidContractHash = errors.New("invalid contract hash")

	// ErrInvalidFunctionName 无效函数名错误
	ErrInvalidFunctionName = errors.New("invalid function name")

	// ErrInvalidParameters 无效参数错误
	ErrInvalidParameters = errors.New("invalid function parameters")

	// ErrInvalidPrivateKey 无效私钥错误
	ErrInvalidPrivateKey = errors.New("invalid private key")

	// ErrMissingContractAddress 缺少合约地址错误
	ErrMissingContractAddress = errors.New("missing contract address")

	// ErrMissingFunctionName 缺少函数名错误
	ErrMissingFunctionName = errors.New("missing function name")

	// ErrMissingCallerAddress 缺少调用者地址错误
	ErrMissingCallerAddress = errors.New("missing caller address")

	// ErrExecutionFailed 执行失败错误
	ErrExecutionFailed = errors.New("contract execution failed")

	// ErrTransactionBuildFailed 交易构建失败错误
	ErrTransactionBuildFailed = errors.New("transaction build failed")

	// ErrTransactionSealFailed 交易封装失败错误
	ErrTransactionSealFailed = errors.New("transaction seal failed")

	// ErrExecutionTimeout 执行超时错误
	ErrExecutionTimeout = errors.New("execution timeout")

	// ErrResourceExhausted 资源耗尽错误
	ErrResourceExhausted = errors.New("execution resource exhausted")

	// ErrPreStageValidationFailed 预执行阶段验证失败错误
	ErrPreStageValidationFailed = errors.New("pre-stage validation failed")

	// ErrPostStageProcessingFailed 后执行阶段处理失败错误
	ErrPostStageProcessingFailed = errors.New("post-stage processing failed")

	// ErrKeyGenerationFailed 密钥生成失败错误
	ErrKeyGenerationFailed = errors.New("key generation failed")

	// ErrContextCreationFailed 执行上下文创建失败错误
	ErrContextCreationFailed = errors.New("execution context creation failed")

	// ErrRuntimeDependenciesMissing 运行时依赖缺失错误
	ErrRuntimeDependenciesMissing = errors.New("runtime dependencies missing")

	// ErrExecutionTraceExtractionFailed 执行轨迹提取失败错误
	ErrExecutionTraceExtractionFailed = errors.New("execution trace extraction failed")

	// ErrExecutionResultHashComputationFailed 执行结果哈希计算失败错误
	ErrExecutionResultHashComputationFailed = errors.New("execution result hash computation failed")

	// ErrZKProofGenerationFailed ZK证明生成失败错误
	ErrZKProofGenerationFailed = errors.New("zk proof generation failed")

	// ErrZKProofEmpty ZK证明为空错误
	ErrZKProofEmpty = errors.New("zk proof is empty")

	// ErrStateIDGenerationFailed 状态ID生成失败错误
	ErrStateIDGenerationFailed = errors.New("state id generation failed")

	// ErrInvalidModelHash 无效模型哈希错误
	ErrInvalidModelHash = errors.New("invalid model hash")

	// ErrInvalidInputTensors 无效输入张量错误
	ErrInvalidInputTensors = errors.New("invalid input tensors")
)

// ============================================================================
//                               错误包装函数
// ============================================================================

// WrapInvalidContractAddressError 包装无效合约地址错误
func WrapInvalidContractAddressError(address string) error {
	return fmt.Errorf("%w: address=%s", ErrInvalidContractAddress, address)
}

// WrapInvalidFunctionNameError 包装无效函数名错误
func WrapInvalidFunctionNameError(functionName string) error {
	return fmt.Errorf("%w: function=%s", ErrInvalidFunctionName, functionName)
}

// WrapInvalidParametersError 包装无效参数错误
func WrapInvalidParametersError(functionName string, reason string) error {
	return fmt.Errorf("%w: function=%s, reason=%s", ErrInvalidParameters, functionName, reason)
}

// WrapExecutionFailedError 包装执行失败错误
func WrapExecutionFailedError(contractAddress, functionName string, err error) error {
	return fmt.Errorf("%w: contract=%s, function=%s, cause=%w", ErrExecutionFailed, contractAddress, functionName, err)
}

// WrapTransactionBuildFailedError 包装交易构建失败错误
func WrapTransactionBuildFailedError(contractAddress, functionName string, err error) error {
	return fmt.Errorf("%w: contract=%s, function=%s, cause=%w", ErrTransactionBuildFailed, contractAddress, functionName, err)
}

// WrapTransactionSealFailedError 包装交易封装失败错误
func WrapTransactionSealFailedError(txHash string, err error) error {
	return fmt.Errorf("%w: txHash=%s, cause=%w", ErrTransactionSealFailed, txHash, err)
}

// WrapExecutionTimeoutError 包装执行超时错误
func WrapExecutionTimeoutError(contractAddress, functionName string, timeoutMs int) error {
	return fmt.Errorf("%w: contract=%s, function=%s, timeout=%dms", ErrExecutionTimeout, contractAddress, functionName, timeoutMs)
}

// WrapResourceExhaustedError 包装资源耗尽错误
func WrapResourceExhaustedError(resource string, limit interface{}) error {
	return fmt.Errorf("%w: resource=%s, limit=%v", ErrResourceExhausted, resource, limit)
}

// WrapPreStageValidationFailedError 包装预执行阶段验证失败错误
func WrapPreStageValidationFailedError(stage, reason string) error {
	return fmt.Errorf("%w: stage=%s, reason=%s", ErrPreStageValidationFailed, stage, reason)
}

// WrapPostStageProcessingFailedError 包装后执行阶段处理失败错误
func WrapPostStageProcessingFailedError(stage string, err error) error {
	return fmt.Errorf("%w: stage=%s, cause=%v", ErrPostStageProcessingFailed, stage, err)
}

// WrapInvalidContractHashError 包装无效合约哈希错误
func WrapInvalidContractHashError(hash []byte) error {
	return fmt.Errorf("%w: hash=%x", ErrInvalidContractHash, hash)
}

// WrapMissingCallerAddressError 包装缺少调用者地址错误
func WrapMissingCallerAddressError() error {
	return ErrMissingCallerAddress
}

// WrapContextCreationFailedError 包装执行上下文创建失败错误
func WrapContextCreationFailedError(executionID string, err error) error {
	return fmt.Errorf("%w: executionID=%s, cause=%w", ErrContextCreationFailed, executionID, err)
}

// WrapRuntimeDependenciesMissingError 包装运行时依赖缺失错误
func WrapRuntimeDependenciesMissingError() error {
	return ErrRuntimeDependenciesMissing
}

// WrapExecutionTraceExtractionFailedError 包装执行轨迹提取失败错误
func WrapExecutionTraceExtractionFailedError(executionID string, err error) error {
	return fmt.Errorf("%w: executionID=%s, cause=%w", ErrExecutionTraceExtractionFailed, executionID, err)
}

// WrapExecutionResultHashComputationFailedError 包装执行结果哈希计算失败错误
func WrapExecutionResultHashComputationFailedError(err error) error {
	return fmt.Errorf("%w: cause=%w", ErrExecutionResultHashComputationFailed, err)
}

// WrapZKProofGenerationFailedError 包装ZK证明生成失败错误
func WrapZKProofGenerationFailedError(circuitID string, err error) error {
	return fmt.Errorf("%w: circuitID=%s, cause=%w", ErrZKProofGenerationFailed, circuitID, err)
}

// WrapZKProofEmptyError 包装ZK证明为空错误
func WrapZKProofEmptyError() error {
	return ErrZKProofEmpty
}

// WrapStateIDGenerationFailedError 包装状态ID生成失败错误
func WrapStateIDGenerationFailedError(err error) error {
	return fmt.Errorf("%w: cause=%w", ErrStateIDGenerationFailed, err)
}

// WrapInvalidModelHashError 包装无效模型哈希错误
func WrapInvalidModelHashError(hash []byte) error {
	return fmt.Errorf("%w: hash=%x", ErrInvalidModelHash, hash)
}

// WrapInvalidInputTensorsError 包装无效输入张量错误
func WrapInvalidInputTensorsError(tensorCount int) error {
	return fmt.Errorf("%w: tensorCount=%d", ErrInvalidInputTensors, tensorCount)
}
