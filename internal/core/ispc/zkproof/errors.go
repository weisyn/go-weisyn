// Package zkproof provides error definitions for zero-knowledge proof operations.
package zkproof

import (
	"errors"
	"fmt"
)

// ============================================================================
//                            零知识证明错误定义
// ============================================================================

var (
	// ErrCircuitNotFound 电路未找到错误
	ErrCircuitNotFound = errors.New("circuit not found")

	// ErrCircuitCompilationFailed 电路编译失败错误
	ErrCircuitCompilationFailed = errors.New("circuit compilation failed")

	// ErrProofGenerationFailed 证明生成失败错误
	ErrProofGenerationFailed = errors.New("proof generation failed")

	// ErrProofVerificationFailed 证明验证失败错误
	ErrProofVerificationFailed = errors.New("proof verification failed")

	// ErrInvalidWitness 无效见证错误
	ErrInvalidWitness = errors.New("invalid witness")

	// ErrInvalidPublicInputs 无效公共输入错误
	ErrInvalidPublicInputs = errors.New("invalid public inputs")

	// ErrInvalidProof 无效证明错误
	ErrInvalidProof = errors.New("invalid proof")

	// ErrCircuitManagerNotInitialized 电路管理器未初始化错误
	ErrCircuitManagerNotInitialized = errors.New("circuit manager not initialized")

	// ErrProverNotInitialized 证明器未初始化错误
	ErrProverNotInitialized = errors.New("prover not initialized")

	// ErrVerifierNotInitialized 验证器未初始化错误
	ErrVerifierNotInitialized = errors.New("verifier not initialized")

	// ErrUnsupportedCircuitType 不支持的电路类型错误
	ErrUnsupportedCircuitType = errors.New("unsupported circuit type")

	// ErrCircuitParametersMismatch 电路参数不匹配错误
	ErrCircuitParametersMismatch = errors.New("circuit parameters mismatch")
)

// ============================================================================
//                               错误包装函数
// ============================================================================

// WrapCircuitNotFoundError 包装电路未找到错误
func WrapCircuitNotFoundError(circuitID string) error {
	return fmt.Errorf("%w: circuitID=%s", ErrCircuitNotFound, circuitID)
}

// WrapCircuitCompilationFailedError 包装电路编译失败错误
func WrapCircuitCompilationFailedError(circuitID string, err error) error {
	return fmt.Errorf("%w: circuitID=%s, cause=%v", ErrCircuitCompilationFailed, circuitID, err)
}

// WrapProofGenerationFailedError 包装证明生成失败错误
func WrapProofGenerationFailedError(circuitID string, err error) error {
	return fmt.Errorf("%w: circuitID=%s, cause=%v", ErrProofGenerationFailed, circuitID, err)
}

// WrapProofVerificationFailedError 包装证明验证失败错误
func WrapProofVerificationFailedError(circuitID string, err error) error {
	return fmt.Errorf("%w: circuitID=%s, cause=%v", ErrProofVerificationFailed, circuitID, err)
}

// WrapInvalidWitnessError 包装无效见证错误
func WrapInvalidWitnessError(circuitID, reason string) error {
	return fmt.Errorf("%w: circuitID=%s, reason=%s", ErrInvalidWitness, circuitID, reason)
}

// WrapInvalidPublicInputsError 包装无效公共输入错误
func WrapInvalidPublicInputsError(circuitID string, expected, actual int) error {
	return fmt.Errorf("%w: circuitID=%s, expected=%d, actual=%d", ErrInvalidPublicInputs, circuitID, expected, actual)
}

// WrapInvalidProofError 包装无效证明错误
func WrapInvalidProofError(circuitID, reason string) error {
	return fmt.Errorf("%w: circuitID=%s, reason=%s", ErrInvalidProof, circuitID, reason)
}

// WrapUnsupportedCircuitTypeError 包装不支持的电路类型错误
func WrapUnsupportedCircuitTypeError(circuitType string) error {
	return fmt.Errorf("%w: type=%s", ErrUnsupportedCircuitType, circuitType)
}

// WrapCircuitParametersMismatchError 包装电路参数不匹配错误
func WrapCircuitParametersMismatchError(circuitID, parameter string, expected, actual interface{}) error {
	return fmt.Errorf("%w: circuitID=%s, parameter=%s, expected=%v, actual=%v", ErrCircuitParametersMismatch, circuitID, parameter, expected, actual)
}
