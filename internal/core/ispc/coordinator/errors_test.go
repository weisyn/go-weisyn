package coordinator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// 错误定义和包装函数测试
// ============================================================================
//
// 🎯 **测试目的**：验证错误定义和包装函数的正确性
//
// ============================================================================

// TestErrorConstants 测试错误常量定义
func TestErrorConstants(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrInvalidContractAddress", ErrInvalidContractAddress},
		{"ErrInvalidContractHash", ErrInvalidContractHash},
		{"ErrInvalidFunctionName", ErrInvalidFunctionName},
		{"ErrInvalidParameters", ErrInvalidParameters},
		{"ErrInvalidPrivateKey", ErrInvalidPrivateKey},
		{"ErrMissingContractAddress", ErrMissingContractAddress},
		{"ErrMissingFunctionName", ErrMissingFunctionName},
		{"ErrMissingCallerAddress", ErrMissingCallerAddress},
		{"ErrExecutionFailed", ErrExecutionFailed},
		{"ErrTransactionBuildFailed", ErrTransactionBuildFailed},
		{"ErrTransactionSealFailed", ErrTransactionSealFailed},
		{"ErrExecutionTimeout", ErrExecutionTimeout},
		{"ErrResourceExhausted", ErrResourceExhausted},
		{"ErrPreStageValidationFailed", ErrPreStageValidationFailed},
		{"ErrPostStageProcessingFailed", ErrPostStageProcessingFailed},
		{"ErrKeyGenerationFailed", ErrKeyGenerationFailed},
		{"ErrContextCreationFailed", ErrContextCreationFailed},
		{"ErrRuntimeDependenciesMissing", ErrRuntimeDependenciesMissing},
		{"ErrExecutionTraceExtractionFailed", ErrExecutionTraceExtractionFailed},
		{"ErrExecutionResultHashComputationFailed", ErrExecutionResultHashComputationFailed},
		{"ErrZKProofGenerationFailed", ErrZKProofGenerationFailed},
		{"ErrZKProofEmpty", ErrZKProofEmpty},
		{"ErrStateIDGenerationFailed", ErrStateIDGenerationFailed},
		{"ErrInvalidModelHash", ErrInvalidModelHash},
		{"ErrInvalidInputTensors", ErrInvalidInputTensors},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.err, "错误常量应该不为nil")
			assert.Error(t, tt.err, "错误常量应该是一个error")
		})
	}
}

// TestWrapInvalidContractAddressError 测试包装无效合约地址错误
func TestWrapInvalidContractAddressError(t *testing.T) {
	address := "invalid_address"
	err := WrapInvalidContractAddressError(address)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contract address")
	assert.Contains(t, err.Error(), address)
	assert.True(t, errors.Is(err, ErrInvalidContractAddress), "应该包装原始错误")
}

// TestWrapInvalidFunctionNameError 测试包装无效函数名错误
func TestWrapInvalidFunctionNameError(t *testing.T) {
	functionName := "invalid_function"
	err := WrapInvalidFunctionNameError(functionName)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid function name")
	assert.Contains(t, err.Error(), functionName)
	assert.True(t, errors.Is(err, ErrInvalidFunctionName), "应该包装原始错误")
}

// TestWrapInvalidParametersError 测试包装无效参数错误
func TestWrapInvalidParametersError(t *testing.T) {
	functionName := "test_function"
	reason := "invalid type"
	err := WrapInvalidParametersError(functionName, reason)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid function parameters")
	assert.Contains(t, err.Error(), functionName)
	assert.Contains(t, err.Error(), reason)
	assert.True(t, errors.Is(err, ErrInvalidParameters), "应该包装原始错误")
}

// TestWrapExecutionFailedError 测试包装执行失败错误
func TestWrapExecutionFailedError(t *testing.T) {
	contractAddress := "0x1234"
	functionName := "test_function"
	cause := errors.New("execution error")
	err := WrapExecutionFailedError(contractAddress, functionName, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contract execution failed")
	assert.Contains(t, err.Error(), contractAddress)
	assert.Contains(t, err.Error(), functionName)
	assert.True(t, errors.Is(err, ErrExecutionFailed), "应该包装原始错误")
	assert.True(t, errors.Is(err, cause), "应该包装原因错误")
}

// TestWrapTransactionBuildFailedError 测试包装交易构建失败错误
func TestWrapTransactionBuildFailedError(t *testing.T) {
	contractAddress := "0x1234"
	functionName := "test_function"
	cause := errors.New("build error")
	err := WrapTransactionBuildFailedError(contractAddress, functionName, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction build failed")
	assert.Contains(t, err.Error(), contractAddress)
	assert.Contains(t, err.Error(), functionName)
	assert.True(t, errors.Is(err, ErrTransactionBuildFailed), "应该包装原始错误")
	assert.True(t, errors.Is(err, cause), "应该包装原因错误")
}

// TestWrapTransactionSealFailedError 测试包装交易封装失败错误
func TestWrapTransactionSealFailedError(t *testing.T) {
	txHash := "0xabcd"
	cause := errors.New("seal error")
	err := WrapTransactionSealFailedError(txHash, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction seal failed")
	assert.Contains(t, err.Error(), txHash)
	assert.True(t, errors.Is(err, ErrTransactionSealFailed), "应该包装原始错误")
	assert.True(t, errors.Is(err, cause), "应该包装原因错误")
}

// TestWrapExecutionTimeoutError 测试包装执行超时错误
func TestWrapExecutionTimeoutError(t *testing.T) {
	contractAddress := "0x1234"
	functionName := "test_function"
	timeoutMs := 5000
	err := WrapExecutionTimeoutError(contractAddress, functionName, timeoutMs)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execution timeout")
	assert.Contains(t, err.Error(), contractAddress)
	assert.Contains(t, err.Error(), functionName)
	assert.Contains(t, err.Error(), "5000")
	assert.True(t, errors.Is(err, ErrExecutionTimeout), "应该包装原始错误")
}

// TestWrapResourceExhaustedError 测试包装资源耗尽错误
func TestWrapResourceExhaustedError(t *testing.T) {
	resource := "memory"
	limit := 1024
	err := WrapResourceExhaustedError(resource, limit)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execution resource exhausted")
	assert.Contains(t, err.Error(), resource)
	assert.Contains(t, err.Error(), "1024")
	assert.True(t, errors.Is(err, ErrResourceExhausted), "应该包装原始错误")
}

// TestWrapPreStageValidationFailedError 测试包装预执行阶段验证失败错误
func TestWrapPreStageValidationFailedError(t *testing.T) {
	stage := "pre_execution"
	reason := "validation failed"
	err := WrapPreStageValidationFailedError(stage, reason)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pre-stage validation failed")
	assert.Contains(t, err.Error(), stage)
	assert.Contains(t, err.Error(), reason)
	assert.True(t, errors.Is(err, ErrPreStageValidationFailed), "应该包装原始错误")
}

// TestWrapPostStageProcessingFailedError 测试包装后执行阶段处理失败错误
func TestWrapPostStageProcessingFailedError(t *testing.T) {
	stage := "post_execution"
	cause := errors.New("processing error")
	err := WrapPostStageProcessingFailedError(stage, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "post-stage processing failed")
	assert.Contains(t, err.Error(), stage)
	assert.True(t, errors.Is(err, ErrPostStageProcessingFailed), "应该包装原始错误")
	// 注意：WrapPostStageProcessingFailedError 使用 %v 而不是 %w，所以不会包装原因错误
	assert.Contains(t, err.Error(), "processing error", "错误信息应该包含原因")
}

// TestWrapInvalidContractHashError 测试包装无效合约哈希错误
func TestWrapInvalidContractHashError(t *testing.T) {
	hash := []byte{0x12, 0x34, 0x56}
	err := WrapInvalidContractHashError(hash)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid contract hash")
	assert.Contains(t, err.Error(), "123456")
	assert.True(t, errors.Is(err, ErrInvalidContractHash), "应该包装原始错误")
}

// TestWrapMissingCallerAddressError 测试包装缺少调用者地址错误
func TestWrapMissingCallerAddressError(t *testing.T) {
	err := WrapMissingCallerAddressError()

	assert.Error(t, err)
	assert.Equal(t, ErrMissingCallerAddress, err, "应该返回原始错误")
}

// TestWrapContextCreationFailedError 测试包装执行上下文创建失败错误
func TestWrapContextCreationFailedError(t *testing.T) {
	executionID := "exec_123"
	cause := errors.New("creation error")
	err := WrapContextCreationFailedError(executionID, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execution context creation failed")
	assert.Contains(t, err.Error(), executionID)
	assert.True(t, errors.Is(err, ErrContextCreationFailed), "应该包装原始错误")
	assert.True(t, errors.Is(err, cause), "应该包装原因错误")
}

// TestWrapRuntimeDependenciesMissingError 测试包装运行时依赖缺失错误
func TestWrapRuntimeDependenciesMissingError(t *testing.T) {
	err := WrapRuntimeDependenciesMissingError()

	assert.Error(t, err)
	assert.Equal(t, ErrRuntimeDependenciesMissing, err, "应该返回原始错误")
}

// TestWrapExecutionTraceExtractionFailedError 测试包装执行轨迹提取失败错误
func TestWrapExecutionTraceExtractionFailedError(t *testing.T) {
	executionID := "exec_123"
	cause := errors.New("extraction error")
	err := WrapExecutionTraceExtractionFailedError(executionID, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execution trace extraction failed")
	assert.Contains(t, err.Error(), executionID)
	assert.True(t, errors.Is(err, ErrExecutionTraceExtractionFailed), "应该包装原始错误")
	assert.True(t, errors.Is(err, cause), "应该包装原因错误")
}

// TestWrapExecutionResultHashComputationFailedError 测试包装执行结果哈希计算失败错误
func TestWrapExecutionResultHashComputationFailedError(t *testing.T) {
	cause := errors.New("computation error")
	err := WrapExecutionResultHashComputationFailedError(cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execution result hash computation failed")
	assert.True(t, errors.Is(err, ErrExecutionResultHashComputationFailed), "应该包装原始错误")
	assert.True(t, errors.Is(err, cause), "应该包装原因错误")
}

// TestWrapZKProofGenerationFailedError 测试包装ZK证明生成失败错误
func TestWrapZKProofGenerationFailedError(t *testing.T) {
	circuitID := "circuit_123"
	cause := errors.New("generation error")
	err := WrapZKProofGenerationFailedError(circuitID, cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "zk proof generation failed")
	assert.Contains(t, err.Error(), circuitID)
	assert.True(t, errors.Is(err, ErrZKProofGenerationFailed), "应该包装原始错误")
	assert.True(t, errors.Is(err, cause), "应该包装原因错误")
}

// TestWrapZKProofEmptyError 测试包装ZK证明为空错误
func TestWrapZKProofEmptyError(t *testing.T) {
	err := WrapZKProofEmptyError()

	assert.Error(t, err)
	assert.Equal(t, ErrZKProofEmpty, err, "应该返回原始错误")
}

// TestWrapStateIDGenerationFailedError 测试包装状态ID生成失败错误
func TestWrapStateIDGenerationFailedError(t *testing.T) {
	cause := errors.New("generation error")
	err := WrapStateIDGenerationFailedError(cause)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state id generation failed")
	assert.True(t, errors.Is(err, ErrStateIDGenerationFailed), "应该包装原始错误")
	assert.True(t, errors.Is(err, cause), "应该包装原因错误")
}

// TestWrapInvalidModelHashError 测试包装无效模型哈希错误
func TestWrapInvalidModelHashError(t *testing.T) {
	hash := []byte{0xab, 0xcd, 0xef}
	err := WrapInvalidModelHashError(hash)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid model hash")
	assert.Contains(t, err.Error(), "abcdef")
	assert.True(t, errors.Is(err, ErrInvalidModelHash), "应该包装原始错误")
}

// TestWrapInvalidInputTensorsError 测试包装无效输入张量错误
func TestWrapInvalidInputTensorsError(t *testing.T) {
	tensorCount := 5
	err := WrapInvalidInputTensorsError(tensorCount)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid input tensors")
	assert.Contains(t, err.Error(), "5")
	assert.True(t, errors.Is(err, ErrInvalidInputTensors), "应该包装原始错误")
}

