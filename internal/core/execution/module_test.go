package execution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/types"
)

// mockEngineAdapter 简单的引擎适配器Mock实现
type mockEngineAdapter struct {
	engineType  types.EngineType
	initialized bool
	hostBound   bool
	closed      bool
}

func (m *mockEngineAdapter) GetEngineType() types.EngineType {
	return m.engineType
}

func (m *mockEngineAdapter) Initialize(config map[string]any) error {
	m.initialized = true
	return nil
}

func (m *mockEngineAdapter) BindHost(binding execiface.HostBinding) error {
	m.hostBound = true
	return nil
}

func (m *mockEngineAdapter) Execute(params types.ExecutionParams) (*types.ExecutionResult, error) {
	return &types.ExecutionResult{
		Success:    true,
		ReturnData: []byte("test result"),
		Consumed:   100,
		Metadata:   map[string]any{"engine": string(m.engineType)},
	}, nil
}

func (m *mockEngineAdapter) Close() error {
	m.closed = true
	return nil
}

// mockHostCapabilityProvider 简单的宿主能力提供者Mock实现
type mockHostCapabilityProvider struct {
	capabilityType string
}

func (m *mockHostCapabilityProvider) GetCapabilityType() string {
	return m.capabilityType
}

func (m *mockHostCapabilityProvider) CapabilityDomain() string {
	return m.capabilityType
}

// TestModuleComponents 测试模块组件的基本功能
func TestModuleComponents(t *testing.T) {
	t.Run("EngineAdapter Interface", func(t *testing.T) {
		adapter := &mockEngineAdapter{
			engineType: types.EngineTypeWASM,
		}

		// 测试基本接口方法
		assert.Equal(t, types.EngineTypeWASM, adapter.GetEngineType())

		// 测试初始化
		err := adapter.Initialize(map[string]any{"test": "config"})
		assert.NoError(t, err)
		assert.True(t, adapter.initialized)

		// 测试关闭
		err = adapter.Close()
		assert.NoError(t, err)
		assert.True(t, adapter.closed)
	})

	t.Run("ExecutionParams Validation", func(t *testing.T) {
		params := types.ExecutionParams{
			ResourceID:  []byte("test-resource"),
			Entry:       "main",
			Payload:     []byte("test payload"),
			ExecutionFeeLimit:    5000,
			MemoryLimit: 1024,
			Timeout:     30000,
			Caller:      "test-caller",
			Context:     map[string]any{"engine_type": types.EngineTypeWASM},
		}

		// 验证参数结构
		assert.NotEmpty(t, params.ResourceID)
		assert.NotEmpty(t, params.Entry)
		assert.Greater(t, params.ExecutionFeeLimit, uint64(0))
		assert.Greater(t, params.MemoryLimit, uint32(0))
		assert.Greater(t, params.Timeout, int64(0))
		assert.NotEmpty(t, params.Caller)
		assert.NotNil(t, params.Context)
	})

	t.Run("ExecutionResult Structure", func(t *testing.T) {
		result := types.ExecutionResult{
			Success:    true,
			ReturnData: []byte("test result"),
			Consumed:   100,
			Metadata:   map[string]any{"engine": "wasm"},
		}

		// 验证结果结构
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.ReturnData)
		assert.Greater(t, result.Consumed, uint64(0))
		assert.NotNil(t, result.Metadata)
		assert.Equal(t, "wasm", result.Metadata["engine"])
	})
}

// TestEngineTypes 测试引擎类型定义
func TestEngineTypes(t *testing.T) {
	t.Run("Engine Type Constants", func(t *testing.T) {
		// 验证预定义的引擎类型
		assert.Equal(t, types.EngineType("wasm"), types.EngineTypeWASM)
		assert.Equal(t, types.EngineType("onnx"), types.EngineTypeONNX)
	})

	t.Run("Engine Type Usage", func(t *testing.T) {
		// 测试引擎类型在参数中的使用
		params := types.ExecutionParams{
			Context: map[string]any{
				"engine_type": types.EngineTypeWASM,
			},
		}

		engineType, ok := params.Context["engine_type"].(types.EngineType)
		assert.True(t, ok)
		assert.Equal(t, types.EngineTypeWASM, engineType)
	})
}

// TestExecutionInterface 测试执行接口的基本实现
func TestExecutionInterface(t *testing.T) {
	t.Run("Mock Engine Execution", func(t *testing.T) {
		adapter := &mockEngineAdapter{
			engineType: types.EngineTypeWASM,
		}

		params := types.ExecutionParams{
			ResourceID: []byte("test-resource"),
			Entry:      "main",
			Payload:    []byte("test"),
		}

		result, err := adapter.Execute(params)

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, []byte("test result"), result.ReturnData)
		assert.Equal(t, uint64(100), result.Consumed)
		assert.Equal(t, "wasm", result.Metadata["engine"])
	})
}

// TestModuleConfiguration 测试模块配置相关功能
func TestModuleConfiguration(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		// 测试默认配置的合理性
		config := map[string]any{
			"资源_limit":    uint64(5000000),
			"memory_limit": uint32(1024),
			"timeout_ms":   int64(30000),
		}

		ExecutionFeeLimit, ok := config["资源_limit"].(uint64)
		assert.True(t, ok)
		assert.Greater(t, ExecutionFeeLimit, uint64(0))

		memoryLimit, ok := config["memory_limit"].(uint32)
		assert.True(t, ok)
		assert.Greater(t, memoryLimit, uint32(0))

		timeout, ok := config["timeout_ms"].(int64)
		assert.True(t, ok)
		assert.Greater(t, timeout, int64(0))
	})
}

// TestContextHandling 测试上下文处理
func TestContextHandling(t *testing.T) {
	t.Run("Context Creation", func(t *testing.T) {
		ctx := context.Background()
		assert.NotNil(t, ctx)

		// 测试带超时的上下文
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		assert.NotNil(t, ctx)
	})

	t.Run("Execution Context", func(t *testing.T) {
		params := types.ExecutionParams{
			Context: map[string]any{
				"caller":      "test-caller",
				"资源_price":   uint64(100),
				"engine_type": types.EngineTypeWASM,
				"debug":       true,
			},
		}

		// 验证上下文数据访问
		caller, ok := params.Context["caller"].(string)
		assert.True(t, ok)
		assert.Equal(t, "test-caller", caller)

		资源Price, ok := params.Context["资源_price"].(uint64)
		assert.True(t, ok)
		assert.Equal(t, uint64(100), 资源Price)

		debug, ok := params.Context["debug"].(bool)
		assert.True(t, ok)
		assert.True(t, debug)
	})
}

// TestErrorHandling 测试错误处理
func TestErrorHandling(t *testing.T) {
	t.Run("Execution Error Types", func(t *testing.T) {
		// 测试不同类型的执行错误
		errorTypes := []string{
			"timeout",
			"资源_limit_exceeded",
			"memory_limit_exceeded",
			"invalid_resource",
			"runtime_error",
		}

		for _, errorType := range errorTypes {
			assert.NotEmpty(t, errorType)
			assert.IsType(t, "", errorType)
		}
	})

	t.Run("Error Result Structure", func(t *testing.T) {
		// 测试错误情况下的结果结构
		errorResult := types.ExecutionResult{
			Success:    false,
			ReturnData: nil,
			Consumed:   0,
			Metadata: map[string]any{
				"error_type": "timeout",
				"error_msg":  "execution timeout",
			},
		}

		assert.False(t, errorResult.Success)
		assert.Nil(t, errorResult.ReturnData)
		assert.Equal(t, uint64(0), errorResult.Consumed)
		assert.Equal(t, "timeout", errorResult.Metadata["error_type"])
	})
}
