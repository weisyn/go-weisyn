package zkproof

import (
	"context"
	"hash"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	// 内部接口
	"github.com/weisyn/v1/internal/core/ispc/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/crypto"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
)

// ============================================================================
// ZK证明生成性能基准测试
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
//   - 运行所有基准测试：`go test -bench=. ./internal/core/ispc/zkproof`
//   - 运行特定测试：`go test -bench=BenchmarkProofGeneration ./internal/core/ispc/zkproof`
//   - 生成CPU分析：`go test -bench=. -cpuprofile=cpu.prof ./internal/core/ispc/zkproof`
//   - 查看分析结果：`go tool pprof cpu.prof`
//
// ⚠️ **限制**：
//   - 由于ZK证明生成需要真实的电路和gnark库，某些测试可能需要跳过
//   - 当前主要测试关键路径的性能
// ============================================================================

// mockLogger Mock的日志记录器
type mockBenchmarkLogger struct{}

func (m *mockBenchmarkLogger) Debug(msg string)                          {}
func (m *mockBenchmarkLogger) Debugf(format string, args ...interface{}) {}
func (m *mockBenchmarkLogger) Info(msg string)                           {}
func (m *mockBenchmarkLogger) Infof(format string, args ...interface{})  {}
func (m *mockBenchmarkLogger) Warn(msg string)                           {}
func (m *mockBenchmarkLogger) Warnf(format string, args ...interface{})  {}
func (m *mockBenchmarkLogger) Error(msg string)                          {}
func (m *mockBenchmarkLogger) Errorf(format string, args ...interface{}) {}
func (m *mockBenchmarkLogger) Fatal(msg string)                          {}
func (m *mockBenchmarkLogger) Fatalf(format string, args ...interface{}) {}
func (m *mockBenchmarkLogger) With(args ...interface{}) log.Logger       { return m }
func (m *mockBenchmarkLogger) Sync() error                               { return nil }
func (m *mockBenchmarkLogger) GetZapLogger() *zap.Logger                 { return zap.NewNop() }

// mockHashManager Mock的哈希管理器
type mockBenchmarkHashManager struct{}

func (m *mockBenchmarkHashManager) SHA256(data []byte) []byte {
	// 简单的Mock实现，返回固定长度的哈希
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	return hash
}

func (m *mockBenchmarkHashManager) SHA3_256(data []byte) []byte {
	return m.SHA256(data)
}

func (m *mockBenchmarkHashManager) Keccak256(data []byte) []byte {
	return m.SHA256(data)
}

func (m *mockBenchmarkHashManager) Blake2b_256(data []byte) []byte {
	return m.SHA256(data)
}

func (m *mockBenchmarkHashManager) RIPEMD160(data []byte) []byte {
	hash := make([]byte, 20)
	for i := range hash {
		hash[i] = byte(i)
	}
	return hash
}

func (m *mockBenchmarkHashManager) DoubleSHA256(data []byte) []byte {
	// 双重SHA256：SHA256(SHA256(data))
	first := m.SHA256(data)
	return m.SHA256(first)
}

func (m *mockBenchmarkHashManager) NewSHA256Hasher() hash.Hash {
	return &mockHasher{}
}

func (m *mockBenchmarkHashManager) NewRIPEMD160Hasher() hash.Hash {
	return &mockHasher{}
}

// mockHasher Mock的hash.Hash实现
type mockHasher struct {
	data []byte
}

func (m *mockHasher) Write(p []byte) (n int, err error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

func (m *mockHasher) Sum(b []byte) []byte {
	hash := make([]byte, 32)
	for i := range hash {
		hash[i] = byte(i)
	}
	return append(b, hash...)
}

func (m *mockHasher) Reset() {
	m.data = nil
}

func (m *mockHasher) Size() int {
	return 32
}

func (m *mockHasher) BlockSize() int {
	return 64
}

// setupBenchmarkProver 创建用于基准测试的Prover实例
func setupBenchmarkProver(b *testing.B) (*Prover, crypto.HashManager) {
	logger := &mockBenchmarkLogger{}
	hashManager := &mockBenchmarkHashManager{}
	config := &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
		MaxConcurrentProofs:  4,
		ProofTimeoutSeconds:  300,
		CircuitCacheSize:     100,
		EnableParallelSetup:  true,
	}

	// 注意：CircuitManager需要真实的电路，这里使用nil（某些测试会跳过）
	circuitManager := NewCircuitManager(logger, config)

	return NewProver(logger, hashManager, circuitManager, config), hashManager
}

// createMockZKProofInput 创建Mock的ZK证明输入
func createMockZKProofInput() *interfaces.ZKProofInput {
	return &interfaces.ZKProofInput{
		CircuitID:      "contract_execution",
		CircuitVersion: 1,
		PublicInputs: [][]byte{
			[]byte("execution_result_hash_12345678901234567890123456789012"),
		},
		PrivateInputs: map[string]interface{}{
			"execution_trace": []byte("mock_execution_trace_data"),
			"state_diff":      []byte("mock_state_diff_data"),
		},
	}
}

// ============================================================================
// 基准测试：关键路径组件
// ============================================================================

// BenchmarkWitnessBuilding 基准测试：Witness构建
//
// 测试构建证明witness的性能
func BenchmarkWitnessBuilding(b *testing.B) {
	prover, _ := setupBenchmarkProver(b)
	input := createMockZKProofInput()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := prover.buildProofWitness(input)
		if err != nil {
			b.Fatalf("构建witness失败: %v", err)
		}
	}
}

// BenchmarkProofSerialization 基准测试：证明序列化
//
// 测试证明序列化的性能
func BenchmarkProofSerialization(b *testing.B) {
	_, _ = setupBenchmarkProver(b)

	// 创建Mock的证明对象（简化版本）
	// 注意：这里使用简化的测试数据，实际序列化需要真实的gnark证明对象
	b.Skip("需要真实的gnark证明对象，当前跳过")
}

// BenchmarkVerifyingKeyHash 基准测试：验证密钥哈希计算
//
// 测试计算验证密钥哈希的性能
func BenchmarkVerifyingKeyHash(b *testing.B) {
	prover, _ := setupBenchmarkProver(b)
	input := createMockZKProofInput()

	// 获取电路（如果失败则跳过）
	_, err := prover.circuitManager.GetCircuit(input.CircuitID, input.CircuitVersion)
	if err != nil {
		b.Skipf("无法获取电路: %v", err)
		return
	}

	// 编译电路
	// 注意：这里需要真实的gnark编译，如果失败则跳过
	b.Skip("需要真实的gnark电路编译，当前跳过")
}

// ============================================================================
// 基准测试：完整证明生成流程（需要真实依赖）
// ============================================================================

// BenchmarkProofGeneration 基准测试：完整证明生成
//
// ⚠️ **注意**：此测试需要真实的电路和gnark库
// 当前使用Mock实现，仅用于测试关键路径的性能
func BenchmarkProofGeneration(b *testing.B) {
	// 跳过需要真实依赖的测试
	b.Skip("需要真实的电路和gnark库，当前跳过")

	prover, _ := setupBenchmarkProver(b)
	ctx := context.Background()
	input := createMockZKProofInput()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := prover.GenerateProof(ctx, input)
		require.NoError(b, err)
	}
}

// BenchmarkStateProofGeneration 基准测试：状态证明生成
//
// ⚠️ **注意**：此测试需要真实的电路和gnark库
func BenchmarkStateProofGeneration(b *testing.B) {
	b.Skip("需要真实的电路和gnark库，当前跳过")

	prover, _ := setupBenchmarkProver(b)
	ctx := context.Background()
	input := createMockZKProofInput()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := prover.GenerateStateProof(ctx, input)
		require.NoError(b, err)
	}
}

// BenchmarkProofGenerationWithRetry 基准测试：带重试机制的证明生成
//
// ⚠️ **注意**：此测试需要真实的电路和gnark库
func BenchmarkProofGenerationWithRetry(b *testing.B) {
	b.Skip("需要真实的电路和gnark库，当前跳过")

	prover, hashManager := setupBenchmarkProver(b)
	logger := &mockBenchmarkLogger{}
	validator := NewValidator(logger, prover.circuitManager, prover.config, hashManager)
	reliabilityEnforcer := NewProofReliabilityEnforcer(logger, prover, validator, nil)
	ctx := context.Background()
	input := createMockZKProofInput()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := reliabilityEnforcer.GenerateProofWithRetry(ctx, input)
		require.NoError(b, err)
	}
}

// ============================================================================
// 基准测试：证明验证
// ============================================================================

// BenchmarkProofVerification 基准测试：证明验证
//
// ⚠️ **注意**：此测试需要真实的证明和验证密钥
func BenchmarkProofVerification(b *testing.B) {
	_, hashManager := setupBenchmarkProver(b)
	_ = NewValidator(&mockBenchmarkLogger{}, nil, &ZKProofManagerConfig{
		DefaultProvingScheme: "groth16",
		DefaultCurve:         "bn254",
	}, hashManager)

	b.Skip("需要真实的证明和验证密钥，当前跳过")
}

// ============================================================================
// 基准测试：性能对比工具
// ============================================================================

// BenchmarkProofGenerationComparison 基准测试：证明生成性能对比
//
// 🎯 **用途**：对比不同电路或配置的性能差异
func BenchmarkProofGenerationComparison(b *testing.B) {
	b.Skip("性能对比测试，需要多个配置对比")
}

// ============================================================================
// 基准测试：关键路径耗时统计
// ============================================================================

// BenchmarkProofGenerationTiming 基准测试：证明生成各阶段耗时统计
//
// 🎯 **用途**：分析证明生成各阶段的耗时分布
func BenchmarkProofGenerationTiming(b *testing.B) {
	b.Skip("需要真实的电路和gnark库，当前跳过")

	prover, _ := setupBenchmarkProver(b)
	ctx := context.Background()
	input := createMockZKProofInput()

	// 记录各阶段耗时
	var circuitCompileTime time.Duration
	var witnessBuildTime time.Duration
	var proofGenTime time.Duration
	var serializationTime time.Duration

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = time.Now()

		// 1. 电路编译
		circuitStart := time.Now()
		_, _ = prover.circuitManager.GetCircuit(input.CircuitID, input.CircuitVersion)
		circuitCompileTime += time.Since(circuitStart)

		// 2. Witness构建
		witnessStart := time.Now()
		// witness, err := prover.buildProofWitness(input, circuit)
		witnessBuildTime += time.Since(witnessStart)

		// 3. 证明生成
		proofStart := time.Now()
		_, err := prover.GenerateProof(ctx, input)
		proofGenTime += time.Since(proofStart)

		if err != nil {
			b.Fatalf("证明生成失败: %v", err)
		}

		_ = serializationTime
	}

	// 输出各阶段平均耗时
	b.Logf("平均电路编译耗时: %v", circuitCompileTime/time.Duration(b.N))
	b.Logf("平均Witness构建耗时: %v", witnessBuildTime/time.Duration(b.N))
	b.Logf("平均证明生成耗时: %v", proofGenTime/time.Duration(b.N))
	b.Logf("平均序列化耗时: %v", serializationTime/time.Duration(b.N))
}

// ============================================================================
// 基准测试：内存分配分析
// ============================================================================

// BenchmarkProofGenerationMemory 基准测试：证明生成内存分配分析
//
// 🎯 **用途**：分析证明生成过程中的内存分配情况
func BenchmarkProofGenerationMemory(b *testing.B) {
	b.Skip("需要真实的电路和gnark库，当前跳过")

	prover, _ := setupBenchmarkProver(b)
	ctx := context.Background()
	input := createMockZKProofInput()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := prover.GenerateProof(ctx, input)
		require.NoError(b, err)
	}
}

// ============================================================================
// 基准测试：并发性能
// ============================================================================

// BenchmarkProofGenerationParallel 并行基准测试：证明生成
//
// 🎯 **用途**：测试并发证明生成的性能
func BenchmarkProofGenerationParallel(b *testing.B) {
	b.Skip("需要真实的电路和gnark库，当前跳过")

	prover, _ := setupBenchmarkProver(b)
	ctx := context.Background()
	input := createMockZKProofInput()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := prover.GenerateProof(ctx, input)
			require.NoError(b, err)
		}
	})
}

// ============================================================================
// 基准测试：不同电路类型的性能对比
// ============================================================================

// BenchmarkContractExecutionCircuit 基准测试：合约执行电路
func BenchmarkContractExecutionCircuit(b *testing.B) {
	b.Skip("需要真实的电路和gnark库，当前跳过")

	prover, _ := setupBenchmarkProver(b)
	ctx := context.Background()
	input := &interfaces.ZKProofInput{
		CircuitID:      "contract_execution",
		CircuitVersion: 1,
		PublicInputs: [][]byte{
			[]byte("execution_result_hash_12345678901234567890123456789012"),
		},
		PrivateInputs: map[string]interface{}{
			"execution_trace": []byte("mock_execution_trace_data"),
			"state_diff":      []byte("mock_state_diff_data"),
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := prover.GenerateProof(ctx, input)
		require.NoError(b, err)
	}
}

// BenchmarkAIModelInferenceCircuit 基准测试：AI模型推理电路
func BenchmarkAIModelInferenceCircuit(b *testing.B) {
	b.Skip("需要真实的电路和gnark库，当前跳过")

	prover, _ := setupBenchmarkProver(b)
	ctx := context.Background()
	input := &interfaces.ZKProofInput{
		CircuitID:      "aimodel_inference",
		CircuitVersion: 1,
		PublicInputs: [][]byte{
			[]byte("inference_result_hash_12345678901234567890123456789012"),
		},
		PrivateInputs: map[string]interface{}{
			"model_weights": []byte("mock_model_weights_data"),
			"input_data":    []byte("mock_input_data"),
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := prover.GenerateProof(ctx, input)
		require.NoError(b, err)
	}
}

// ============================================================================
// 基准测试：性能回归测试辅助函数
// ============================================================================

// compareBenchmarkResults 比较基准测试结果
//
// 🎯 **用途**：用于性能回归测试，比较当前结果与历史结果
func compareBenchmarkResults(current, baseline map[string]float64) map[string]float64 {
	comparison := make(map[string]float64)

	for key, currentValue := range current {
		if baselineValue, exists := baseline[key]; exists {
			// 计算性能变化百分比（正值表示变慢，负值表示变快）
			changePercent := ((currentValue - baselineValue) / baselineValue) * 100
			comparison[key] = changePercent
		}
	}

	return comparison
}

// recordBenchmarkBaseline 记录基准测试基线
//
// 🎯 **用途**：记录当前性能作为基线，用于后续回归测试
func recordBenchmarkBaseline(results map[string]float64) {
	// 这里可以将结果保存到文件或数据库中
	// 用于后续的性能回归测试
}
