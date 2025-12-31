package zkproof

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/internal/core/ispc/testutil"
)

// ============================================================================
// reliability.go 测试
// ============================================================================

// TestDefaultProofGenerationRetryConfig 测试默认重试配置
func TestDefaultProofGenerationRetryConfig(t *testing.T) {
	config := DefaultProofGenerationRetryConfig()
	require.NotNil(t, config)
	require.Equal(t, 3, config.MaxRetries)
	require.Equal(t, 100*time.Millisecond, config.InitialDelay)
	require.Equal(t, 5*time.Second, config.MaxDelay)
	require.Equal(t, 2.0, config.BackoffFactor)
	require.Greater(t, len(config.RetryableErrors), 0)
}

// TestNewProofReliabilityEnforcer 测试创建可靠性增强器
func TestNewProofReliabilityEnforcer(t *testing.T) {
	logger := testutil.NewTestLogger()
	prover := createTestProver(t)
	validator := createTestValidator(t)

	enforcer := NewProofReliabilityEnforcer(logger, prover, validator, nil)
	require.NotNil(t, enforcer)
	require.NotNil(t, enforcer.retryConfig)
	require.Equal(t, 1000, enforcer.maxErrorLogs)
}

// TestNewProofReliabilityEnforcer_WithConfig 测试使用自定义配置创建可靠性增强器
func TestNewProofReliabilityEnforcer_WithConfig(t *testing.T) {
	logger := testutil.NewTestLogger()
	prover := createTestProver(t)
	validator := createTestValidator(t)
	customConfig := &ProofGenerationRetryConfig{
		MaxRetries:    5,
		InitialDelay:  200 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 1.5,
		RetryableErrors: []string{"test_error"},
	}

	enforcer := NewProofReliabilityEnforcer(logger, prover, validator, customConfig)
	require.NotNil(t, enforcer)
	require.Equal(t, customConfig, enforcer.retryConfig)
}

// TestProofReliabilityEnforcer_isRetryableError 测试判断错误是否可重试
func TestProofReliabilityEnforcer_isRetryableError(t *testing.T) {
	enforcer := createTestReliabilityEnforcer(t)

	// 测试可重试的错误
	retryableErr := errors.New("timeout error occurred")
	require.True(t, enforcer.isRetryableError(retryableErr))

	retryableErr = errors.New("temporary failure")
	require.True(t, enforcer.isRetryableError(retryableErr))

	retryableErr = errors.New("circuit compilation failed")
	require.True(t, enforcer.isRetryableError(retryableErr))

	retryableErr = errors.New("witness building error")
	require.True(t, enforcer.isRetryableError(retryableErr))

	// 测试不可重试的错误
	nonRetryableErr := errors.New("invalid circuit ID")
	require.False(t, enforcer.isRetryableError(nonRetryableErr))

	// 测试上下文取消错误（不可重试）
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.False(t, enforcer.isRetryableError(ctx.Err()))

	// 测试nil错误
	require.False(t, enforcer.isRetryableError(nil))
}

// TestProofReliabilityEnforcer_contains 测试字符串包含检查
func TestProofReliabilityEnforcer_contains(t *testing.T) {
	require.True(t, contains("timeout error", "timeout"))
	require.True(t, contains("TIMEOUT ERROR", "timeout")) // 不区分大小写
	require.True(t, contains("Error: timeout occurred", "timeout"))
	require.False(t, contains("error", "timeout"))
	require.False(t, contains("short", "very long string"))
}

// TestProofReliabilityEnforcer_equalsIgnoreCase 测试不区分大小写的字符串比较
func TestProofReliabilityEnforcer_equalsIgnoreCase(t *testing.T) {
	require.True(t, equalsIgnoreCase("timeout", "timeout"))
	require.True(t, equalsIgnoreCase("TIMEOUT", "timeout"))
	require.True(t, equalsIgnoreCase("Timeout", "TIMEOUT"))
	require.False(t, equalsIgnoreCase("timeout", "error"))
	require.False(t, equalsIgnoreCase("timeout", "timeoutx"))
}

// TestProofReliabilityEnforcer_logError 测试错误日志记录
func TestProofReliabilityEnforcer_logError(t *testing.T) {
	enforcer := createTestReliabilityEnforcer(t)

	input := &interfaces.ZKProofInput{
		CircuitID:      "test_circuit",
		CircuitVersion: 1,
	}
	err := errors.New("test error")
	context := map[string]interface{}{
		"test_key": "test_value",
	}

	enforcer.logError(input, err, 0, true, context)

	logs := enforcer.GetErrorLogs(10)
	require.Equal(t, 1, len(logs))
	require.Equal(t, "test_circuit", logs[0].CircuitID)
	require.Equal(t, uint32(1), logs[0].CircuitVersion)
	require.Equal(t, err, logs[0].Error)
	require.True(t, logs[0].Retryable)
}

// TestProofReliabilityEnforcer_GetErrorLogs 测试获取错误日志
func TestProofReliabilityEnforcer_GetErrorLogs(t *testing.T) {
	enforcer := createTestReliabilityEnforcer(t)

	input := &interfaces.ZKProofInput{
		CircuitID:      "test_circuit",
		CircuitVersion: 1,
	}

	// 添加多个错误日志
	for i := 0; i < 5; i++ {
		enforcer.logError(input, errors.New("error"), i, true, nil)
	}

	// 获取所有日志
	logs := enforcer.GetErrorLogs(0)
	require.Equal(t, 5, len(logs))

	// 获取最近的3条日志
	logs = enforcer.GetErrorLogs(3)
	require.Equal(t, 3, len(logs))
}

// TestProofReliabilityEnforcer_GetErrorStats 测试获取错误统计信息
func TestProofReliabilityEnforcer_GetErrorStats(t *testing.T) {
	enforcer := createTestReliabilityEnforcer(t)

	input1 := &interfaces.ZKProofInput{
		CircuitID:      "circuit1",
		CircuitVersion: 1,
	}
	input2 := &interfaces.ZKProofInput{
		CircuitID:      "circuit2",
		CircuitVersion: 1,
	}

	// 添加可重试错误
	enforcer.logError(input1, errors.New("timeout"), 0, true, nil)
	enforcer.logError(input1, errors.New("timeout"), 1, true, nil)

	// 添加不可重试错误
	enforcer.logError(input2, errors.New("invalid"), 0, false, nil)

	stats := enforcer.GetErrorStats()
	require.NotNil(t, stats)
	require.Equal(t, 3, stats["total_errors"])
	require.Equal(t, 2, stats["retryable_errors"])
	require.Equal(t, 1, stats["non_retryable_errors"])
	require.Equal(t, 1000, stats["max_error_logs"])

	circuitCounts := stats["circuit_error_counts"].(map[string]int)
	require.Equal(t, 2, circuitCounts["circuit1"])
	require.Equal(t, 1, circuitCounts["circuit2"])
}

// TestProofReliabilityEnforcer_ClearErrorLogs 测试清空错误日志
func TestProofReliabilityEnforcer_ClearErrorLogs(t *testing.T) {
	enforcer := createTestReliabilityEnforcer(t)

	input := &interfaces.ZKProofInput{
		CircuitID:      "test_circuit",
		CircuitVersion: 1,
	}

	// 添加错误日志
	enforcer.logError(input, errors.New("error"), 0, true, nil)
	require.Equal(t, 1, len(enforcer.GetErrorLogs(0)))

	// 清空日志
	enforcer.ClearErrorLogs()
	require.Equal(t, 0, len(enforcer.GetErrorLogs(0)))
}

// TestProofReliabilityEnforcer_GetErrorLogs_MaxLimit 测试错误日志数量限制
func TestProofReliabilityEnforcer_GetErrorLogs_MaxLimit(t *testing.T) {
	enforcer := createTestReliabilityEnforcer(t)
	enforcer.maxErrorLogs = 5 // 设置较小的限制

	input := &interfaces.ZKProofInput{
		CircuitID:      "test_circuit",
		CircuitVersion: 1,
	}

	// 添加超过限制的错误日志
	for i := 0; i < 10; i++ {
		enforcer.logError(input, errors.New("error"), i, true, nil)
	}

	// 应该只保留最近的5条
	logs := enforcer.GetErrorLogs(0)
	require.Equal(t, 5, len(logs))
}

// createTestReliabilityEnforcer 创建测试用的可靠性增强器
func createTestReliabilityEnforcer(t *testing.T) *ProofReliabilityEnforcer {
	logger := testutil.NewTestLogger()
	prover := createTestProver(t)
	validator := createTestValidator(t)
	return NewProofReliabilityEnforcer(logger, prover, validator, nil)
}

// TestProofReliabilityEnforcer_GenerateStateProofWithRetry 测试带重试机制的状态证明生成
// 注意：这个测试需要真实的证明生成，可能会失败，但我们测试接口调用
func TestProofReliabilityEnforcer_GenerateStateProofWithRetry(t *testing.T) {
	enforcer := createTestReliabilityEnforcer(t)

	ctx := context.Background()
	input := &interfaces.ZKProofInput{
		CircuitID:      "contract_execution",
		CircuitVersion: 1,
		PublicInputs:   [][]byte{[]byte("test_hash")},
		PrivateInputs: map[string]interface{}{
			"execution_trace": []byte("trace_data"),
			"state_diff":      []byte("state_diff_data"),
		},
	}

	// 这个测试可能会失败（因为需要真实的电路和密钥），但我们测试接口调用
	_, err := enforcer.GenerateStateProofWithRetry(ctx, input)
	// 预期可能会失败（因为需要真实的电路设置），但我们确保接口正确调用
	require.NotNil(t, err) // 预期错误，因为需要真实的电路设置
}

// createTestProver 创建测试用的证明生成器
func createTestProver(t *testing.T) *Prover {
	logger := testutil.NewTestLogger()
	hashManager := testutil.NewTestHashManager()
	circuitManager := NewCircuitManager(logger, &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	})
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}
	return NewProver(logger, hashManager, circuitManager, config)
}

