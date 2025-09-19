package security

import (
	"context"
	"testing"
	"time"

	"github.com/weisyn/v1/pkg/types"
)

// TestExecutionSecurity_BasicFunctionality 测试ExecutionSecurity基础功能
func TestExecutionSecurity_BasicFunctionality(t *testing.T) {
	// 创建简化的安全保护器
	es := NewExecutionSecurity()

	// 测试基本配置
	if es.maxExecutionTime != 30*time.Second {
		t.Errorf("Expected max execution time 30s, got %v", es.maxExecutionTime)
	}

	if es.maxMemoryUsage != 67108864 {
		t.Errorf("Expected max memory 64MB, got %d", es.maxMemoryUsage)
	}

	if es.maxExecutionFeeLimit != 1000000 {
		t.Errorf("Expected max 资源 1M, got %d", es.maxExecutionFeeLimit)
	}
}

// TestExecutionSecurity_ValidateExecution 测试执行验证
func TestExecutionSecurity_ValidateExecution(t *testing.T) {
	es := NewExecutionSecurity()

	// 测试正常参数
	validParams := types.ExecutionParams{
		ResourceID:   []byte("test_contract"),
		ContractAddr: "0x123",
		ExecutionFeeLimit:     100000,
		MemoryLimit:  1024 * 1024, // 1MB
		Timeout:      5000,        // 5秒
		Context:      map[string]any{"engine_type": "wasm"},
	}

	if err := es.ValidateExecution(validParams); err != nil {
		t.Errorf("Valid params should pass validation, got error: %v", err)
	}

	// 测试超时限制
	invalidTimeoutParams := validParams
	invalidTimeoutParams.Timeout = 60000 // 60秒，超过30秒限制

	if err := es.ValidateExecution(invalidTimeoutParams); err == nil {
		t.Error("Invalid timeout should fail validation")
	}

	// 测试内存限制
	invalidMemoryParams := validParams
	invalidMemoryParams.MemoryLimit = 128 * 1024 * 1024 // 128MB，超过64MB限制

	if err := es.ValidateExecution(invalidMemoryParams); err == nil {
		t.Error("Invalid memory limit should fail validation")
	}

	// 测试资源限制
	invalid资源Params := validParams
	invalid资源Params.ExecutionFeeLimit = 2000000 // 200万，超过100万限制

	if err := es.ValidateExecution(invalid资源Params); err == nil {
		t.Error("Invalid 资源 limit should fail validation")
	}
}

// TestExecutionSecurity_HostCallValidation 测试宿主函数验证
func TestExecutionSecurity_HostCallValidation(t *testing.T) {
	es := NewExecutionSecurity()

	// 测试允许的函数
	allowedFunctions := []string{
		"blockchain.getBlockHeight",
		"storage.get",
		"crypto.hash",
		"log.info",
	}

	// 为每个函数提供正确的参数
	testCases := map[string][]interface{}{
		"blockchain.getBlockHeight": {},          // 无参数
		"storage.get":               {"key"},     // 1个参数
		"crypto.hash":               {"data"},    // 1个参数
		"log.info":                  {"message"}, // 1个参数
	}

	for _, funcName := range allowedFunctions {
		params := testCases[funcName]
		if err := es.ValidateHostCall(funcName, params); err != nil {
			t.Errorf("Allowed function %s should pass validation, got error: %v", funcName, err)
		}
	}

	// 测试不允许的函数
	disallowedFunctions := []string{
		"system.exec",
		"file.write",
		"network.request",
	}

	for _, funcName := range disallowedFunctions {
		if err := es.ValidateHostCall(funcName, []interface{}{}); err == nil {
			t.Errorf("Disallowed function %s should fail validation", funcName)
		}
	}
}

// TestExecutionSecurity_ResourceLimits 测试资源限制
func TestExecutionSecurity_ResourceLimits(t *testing.T) {
	es := NewExecutionSecurity()

	ctx := context.Background()
	limitedCtx, cancel := es.ApplyResourceLimits(ctx)
	defer cancel()

	// 检查上下文是否有超时
	if _, hasDeadline := limitedCtx.Deadline(); !hasDeadline {
		t.Error("Resource limited context should have deadline")
	}

	// 检查上下文值
	if maxMem := limitedCtx.Value("max_memory"); maxMem != es.maxMemoryUsage {
		t.Errorf("Expected max memory in context: %d, got: %v", es.maxMemoryUsage, maxMem)
	}

	if max资源 := limitedCtx.Value("max_资源"); max资源 != es.maxExecutionFeeLimit {
		t.Errorf("Expected max 资源 in context: %d, got: %v", es.maxExecutionFeeLimit, max资源)
	}
}

// TestDefaultConstructors 测试默认构造函数
func TestDefaultConstructors(t *testing.T) {
	// 测试简化的SecurityIntegrator
	si := NewDefaultSecurityIntegrator()
	if si == nil {
		t.Error("NewDefaultSecurityIntegrator should not return nil")
	}

	// 测试基础验证功能
	params := types.ExecutionParams{
		ResourceID:   []byte("test"),
		ContractAddr: "0x123",
		ExecutionFeeLimit:     100000,
		MemoryLimit:  1024 * 1024,
		Timeout:      5000,
		Entry:        "main",
		Context:      map[string]any{"engine_type": "wasm"}, // 设置引擎类型
	}

	if err := si.ValidateExecution(context.Background(), params); err != nil {
		t.Errorf("Default SecurityIntegrator validation failed: %v", err)
	}

	// 测试简化的QuotaManager
	qm := NewDefaultQuotaManager()
	if qm == nil {
		t.Error("NewDefaultQuotaManager should not return nil")
	}

	if _, err := qm.CheckQuota(context.Background(), params); err != nil {
		t.Errorf("Default QuotaManager check failed: %v", err)
	}
}

// BenchmarkExecutionSecurity_ValidateExecution 性能基准测试
func BenchmarkExecutionSecurity_ValidateExecution(b *testing.B) {
	es := NewExecutionSecurity()
	params := types.ExecutionParams{
		ResourceID:   []byte("test_contract"),
		ContractAddr: "0x123",
		ExecutionFeeLimit:     100000,
		MemoryLimit:  1024 * 1024,
		Timeout:      5000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = es.ValidateExecution(params)
	}
}

// BenchmarkExecutionSecurity_HostCallValidation 宿主函数验证性能基准
func BenchmarkExecutionSecurity_HostCallValidation(b *testing.B) {
	es := NewExecutionSecurity()
	funcName := "blockchain.getBlockHeight"
	params := []interface{}{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = es.ValidateHostCall(funcName, params)
	}
}
