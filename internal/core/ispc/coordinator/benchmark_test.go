package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	// 内部模块依赖
	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
// 性能基准测试
// ============================================================================
//
// 🎯 **目的**：
//   - 用于开发阶段的性能分析和优化
//   - 性能回归测试
//   - 识别性能瓶颈
//
// 📋 **注意**：
//   - 这些是开发工具，不是生产监控
//   - 基准测试需要Mock依赖，避免真实执行
//   - 使用`go test -bench=. -benchmem`运行
//   - 使用`go test -bench=. -cpuprofile=cpu.prof`生成性能分析文件
//
// 🔧 **使用方法**：
//   - 运行所有基准测试：`go test -bench=. ./internal/core/ispc/coordinator`
//   - 运行特定测试：`go test -bench=BenchmarkExtractExecutionTrace ./internal/core/ispc/coordinator`
//   - 生成CPU分析：`go test -bench=. -cpuprofile=cpu.prof ./internal/core/ispc/coordinator`
//   - 查看分析结果：`go tool pprof cpu.prof`
//
// ⚠️ **限制**：
//   - 由于Manager需要具体类型（*ctxmgr.Manager、*zkproof.Manager），
//     完整流程的基准测试需要真实的依赖设置
//   - 当前仅测试关键路径的辅助函数，这些函数可以直接测试
// ============================================================================

// mockLogger Mock的日志记录器
type mockLogger struct{}

func (m *mockLogger) Debug(msg string)                          {}
func (m *mockLogger) Debugf(format string, args ...interface{}) {}
func (m *mockLogger) Info(msg string)                           {}
func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Warn(msg string)                           {}
func (m *mockLogger) Warnf(format string, args ...interface{})  {}
func (m *mockLogger) Error(msg string)                          {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}
func (m *mockLogger) Fatal(msg string)                          {}
func (m *mockLogger) Fatalf(format string, args ...interface{}) {}
func (m *mockLogger) With(args ...interface{}) log.Logger       { return m }
func (m *mockLogger) Sync() error                               { return nil }
func (m *mockLogger) GetZapLogger() *zap.Logger                 { return zap.NewNop() }

// setupBenchmarkManager 创建用于基准测试的Manager实例
//
// 注意：仅用于测试关键路径的辅助函数（extractExecutionTrace、computeExecutionResultHash、generateStateID）
// 这些函数只需要logger，不需要完整的依赖
func setupBenchmarkManager(b *testing.B) *Manager {
	logger := &mockLogger{}

	// 创建最小化的Manager，仅用于测试辅助函数
	manager := &Manager{
		logger: logger,
		// 注意：contextManager、zkproofManager、engineManager需要具体类型
		// 对于辅助函数的基准测试，这些可以为nil（如果函数不使用它们）
	}

	return manager
}

// mockExecutionContext Mock的执行上下文
// 用于测试extractExecutionTrace函数
type mockExecutionContext struct{}

func (m *mockExecutionContext) GetExecutionID() string {
	return "mock_execution_id"
}

func (m *mockExecutionContext) GetDraftID() string {
	return "mock_draft_id"
}

func (m *mockExecutionContext) GetBlockHeight() uint64 {
	return 100
}

func (m *mockExecutionContext) GetBlockTimestamp() uint64 {
	return uint64(time.Now().Unix())
}

func (m *mockExecutionContext) GetChainID() []byte {
	return []byte("test_chain_id")
}

func (m *mockExecutionContext) GetTransactionID() []byte {
	return []byte("mock_transaction_id_12345678901234567890123456789012")
}

func (m *mockExecutionContext) HostABI() interfaces.HostABI {
	return nil
}

func (m *mockExecutionContext) SetHostABI(hostABI interfaces.HostABI) error {
	return nil
}

func (m *mockExecutionContext) GetCallerAddress() []byte {
	return []byte("mock_caller_address")
}

func (m *mockExecutionContext) GetTransactionDraft() (*interfaces.TransactionDraft, error) {
	return nil, nil
}

func (m *mockExecutionContext) UpdateTransactionDraft(draft *interfaces.TransactionDraft) error {
	return nil
}

func (m *mockExecutionContext) RecordHostFunctionCall(call *interfaces.HostFunctionCall) {
	// Mock实现：不记录
}

func (m *mockExecutionContext) GetExecutionTrace() ([]*interfaces.HostFunctionCall, error) {
	return []*interfaces.HostFunctionCall{}, nil
}

func (m *mockExecutionContext) SetReturnData(data []byte) error {
	return nil
}

func (m *mockExecutionContext) GetReturnData() ([]byte, error) {
	return []byte("test return data"), nil
}

func (m *mockExecutionContext) AddEvent(event *interfaces.Event) error {
	return nil
}

func (m *mockExecutionContext) GetEvents() ([]*interfaces.Event, error) {
	return []*interfaces.Event{}, nil
}

func (m *mockExecutionContext) SetInitParams(params []byte) error {
	return nil
}

func (m *mockExecutionContext) GetInitParams() ([]byte, error) {
	return []byte{}, nil
}

func (m *mockExecutionContext) GetContractAddress() []byte {
	return []byte("mock_contract_address")
}

// ============================================================================
// 基准测试：WASM合约执行
// ============================================================================

// BenchmarkExecuteWASMContract 基准测试：WASM合约执行
//
// ⚠️ **注意**：此测试需要完整的Manager设置，包括运行时依赖
// 当前使用Mock实现，仅用于测试关键路径的性能
// 实际使用时需要设置真实的依赖
func BenchmarkExecuteWASMContract(b *testing.B) {
	// 跳过需要运行时依赖的测试
	b.Skip("需要完整的运行时依赖设置，当前仅测试关键路径函数")

	// 以下代码在跳过测试时不会执行，保留以供将来实现时参考
	// manager := setupBenchmarkManager(b)
	// ctx := context.Background()
	// contractHash := []byte("test_contract_hash_12345678901234567890123456789012")
	// methodName := "test_method"
	// params := []uint64{1, 2, 3}
	// initParams := []byte{}
	// callerAddress := "test_caller_address"
	// b.ResetTimer()
	// b.ReportAllocs()
	// for i := 0; i < b.N; i++ {
	// 	_, err := manager.ExecuteWASMContract(ctx, contractHash, methodName, params, initParams, callerAddress)
	// 	require.NoError(b, err)
	// }
}

// BenchmarkExecuteWASMContract_Parallel 并行基准测试：WASM合约执行
//
// ⚠️ **注意**：此测试需要完整的Manager设置
func BenchmarkExecuteWASMContract_Parallel(b *testing.B) {
	b.Skip("需要完整的运行时依赖设置，当前仅测试关键路径函数")
}

// ============================================================================
// 基准测试：ONNX模型推理
// ============================================================================

// BenchmarkExecuteONNXModel 基准测试：ONNX模型推理
//
// ⚠️ **注意**：此测试需要完整的Manager设置，包括运行时依赖
// 当前使用Mock实现，仅用于测试关键路径的性能
func BenchmarkExecuteONNXModel(b *testing.B) {
	b.Skip("需要完整的运行时依赖设置，当前仅测试关键路径函数")
}

// BenchmarkExecuteONNXModel_Parallel 并行基准测试：ONNX模型推理
//
// ⚠️ **注意**：此测试需要完整的Manager设置
func BenchmarkExecuteONNXModel_Parallel(b *testing.B) {
	b.Skip("需要完整的运行时依赖设置，当前仅测试关键路径函数")
}

// ============================================================================
// 基准测试：关键路径组件
// ============================================================================

// BenchmarkExtractExecutionTrace 基准测试：执行轨迹提取
//
// 测试执行轨迹提取的性能
func BenchmarkExtractExecutionTrace(b *testing.B) {
	manager := setupBenchmarkManager(b)
	ctx := context.Background()

	executionContext := &mockExecutionContext{}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := manager.extractExecutionTrace(ctx, executionContext)
		require.NoError(b, err)
	}
}

// BenchmarkComputeExecutionResultHash 基准测试：执行结果哈希计算
//
// 测试执行结果哈希计算的性能
func BenchmarkComputeExecutionResultHash(b *testing.B) {
	manager := setupBenchmarkManager(b)

	result := []uint64{1, 2, 3, 4, 5}
	trace := &ExecutionTrace{
		TraceID:            "test_trace_id",
		StartTime:          time.Now(),
		EndTime:            time.Now().Add(10 * time.Millisecond),
		HostFunctionCalls:  []HostFunctionCall{},
		StateChanges:       []StateChange{},
		OracleInteractions: []OracleInteraction{},
		ExecutionPath:      []string{"contract_call"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := manager.computeExecutionResultHash(result, trace)
		require.NoError(b, err)
	}
}

// BenchmarkGenerateStateID 基准测试：状态ID生成
//
// 测试状态ID生成的性能
func BenchmarkGenerateStateID(b *testing.B) {
	manager := setupBenchmarkManager(b)
	ctx := context.Background()

	// 设置上下文值
	ctx = context.WithValue(ctx, ContextKeyContract, "test_contract")
	ctx = context.WithValue(ctx, ContextKeyFunction, "test_function")
	ctx = context.WithValue(ctx, ContextKeyExecutionStart, time.Now())
	ctx = context.WithValue(ctx, ContextKeyParamsCount, 3)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := manager.generateStateID(ctx)
		require.NoError(b, err)
	}
}
