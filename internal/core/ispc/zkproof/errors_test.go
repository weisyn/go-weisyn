package zkproof

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// ============================================================================
// errors.go 测试
// ============================================================================

// TestWrapCircuitNotFoundError 测试包装电路未找到错误
func TestWrapCircuitNotFoundError(t *testing.T) {
	circuitID := "test_circuit"
	err := WrapCircuitNotFoundError(circuitID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "circuit not found")
	require.Contains(t, err.Error(), circuitID)
	require.True(t, errors.Is(err, ErrCircuitNotFound))
}

// TestWrapCircuitCompilationFailedError 测试包装电路编译失败错误
func TestWrapCircuitCompilationFailedError(t *testing.T) {
	circuitID := "test_circuit"
	cause := errors.New("compilation error")
	err := WrapCircuitCompilationFailedError(circuitID, cause)
	require.Error(t, err)
	require.Contains(t, err.Error(), "circuit compilation failed")
	require.Contains(t, err.Error(), circuitID)
	require.Contains(t, err.Error(), cause.Error())
	require.True(t, errors.Is(err, ErrCircuitCompilationFailed))
}

// TestWrapProofGenerationFailedError 测试包装证明生成失败错误
func TestWrapProofGenerationFailedError(t *testing.T) {
	circuitID := "test_circuit"
	cause := errors.New("generation error")
	err := WrapProofGenerationFailedError(circuitID, cause)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proof generation failed")
	require.Contains(t, err.Error(), circuitID)
	require.Contains(t, err.Error(), cause.Error())
	require.True(t, errors.Is(err, ErrProofGenerationFailed))
}

// TestWrapProofVerificationFailedError 测试包装证明验证失败错误
func TestWrapProofVerificationFailedError(t *testing.T) {
	circuitID := "test_circuit"
	cause := errors.New("verification error")
	err := WrapProofVerificationFailedError(circuitID, cause)
	require.Error(t, err)
	require.Contains(t, err.Error(), "proof verification failed")
	require.Contains(t, err.Error(), circuitID)
	require.Contains(t, err.Error(), cause.Error())
	require.True(t, errors.Is(err, ErrProofVerificationFailed))
}

// TestWrapInvalidWitnessError 测试包装无效见证错误
func TestWrapInvalidWitnessError(t *testing.T) {
	circuitID := "test_circuit"
	reason := "witness format invalid"
	err := WrapInvalidWitnessError(circuitID, reason)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid witness")
	require.Contains(t, err.Error(), circuitID)
	require.Contains(t, err.Error(), reason)
	require.True(t, errors.Is(err, ErrInvalidWitness))
}

// TestWrapInvalidPublicInputsError 测试包装无效公共输入错误
func TestWrapInvalidPublicInputsError(t *testing.T) {
	circuitID := "test_circuit"
	expected := 5
	actual := 3
	err := WrapInvalidPublicInputsError(circuitID, expected, actual)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid public inputs")
	require.Contains(t, err.Error(), circuitID)
	require.Contains(t, err.Error(), "expected=5")
	require.Contains(t, err.Error(), "actual=3")
	require.True(t, errors.Is(err, ErrInvalidPublicInputs))
}

// TestWrapInvalidProofError 测试包装无效证明错误
func TestWrapInvalidProofError(t *testing.T) {
	circuitID := "test_circuit"
	reason := "proof format invalid"
	err := WrapInvalidProofError(circuitID, reason)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid proof")
	require.Contains(t, err.Error(), circuitID)
	require.Contains(t, err.Error(), reason)
	require.True(t, errors.Is(err, ErrInvalidProof))
}

// TestWrapUnsupportedCircuitTypeError 测试包装不支持的电路类型错误
func TestWrapUnsupportedCircuitTypeError(t *testing.T) {
	circuitType := "unsupported_type"
	err := WrapUnsupportedCircuitTypeError(circuitType)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported circuit type")
	require.Contains(t, err.Error(), circuitType)
	require.True(t, errors.Is(err, ErrUnsupportedCircuitType))
}

// TestWrapCircuitParametersMismatchError 测试包装电路参数不匹配错误
func TestWrapCircuitParametersMismatchError(t *testing.T) {
	circuitID := "test_circuit"
	parameter := "depth"
	expected := 10
	actual := 5
	err := WrapCircuitParametersMismatchError(circuitID, parameter, expected, actual)
	require.Error(t, err)
	require.Contains(t, err.Error(), "circuit parameters mismatch")
	require.Contains(t, err.Error(), circuitID)
	require.Contains(t, err.Error(), parameter)
	require.Contains(t, err.Error(), "expected=10")
	require.Contains(t, err.Error(), "actual=5")
	require.True(t, errors.Is(err, ErrCircuitParametersMismatch))
}

// TestErrorConstants 测试错误常量
func TestErrorConstants(t *testing.T) {
	require.NotNil(t, ErrCircuitNotFound)
	require.NotNil(t, ErrCircuitCompilationFailed)
	require.NotNil(t, ErrProofGenerationFailed)
	require.NotNil(t, ErrProofVerificationFailed)
	require.NotNil(t, ErrInvalidWitness)
	require.NotNil(t, ErrInvalidPublicInputs)
	require.NotNil(t, ErrInvalidProof)
	require.NotNil(t, ErrCircuitManagerNotInitialized)
	require.NotNil(t, ErrProverNotInitialized)
	require.NotNil(t, ErrVerifierNotInitialized)
	require.NotNil(t, ErrUnsupportedCircuitType)
	require.NotNil(t, ErrCircuitParametersMismatch)
}

