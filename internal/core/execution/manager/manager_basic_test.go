package manager

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	execiface "github.com/weisyn/v1/pkg/interfaces/execution"
	"github.com/weisyn/v1/pkg/types"
)

// TestRegistry 测试注册表功能
func TestRegistry(t *testing.T) {
	t.Run("NewRegistry Creation", func(t *testing.T) {
		registry := NewRegistry()

		assert.NotNil(t, registry)
		assert.NotNil(t, registry.engines)
		assert.Empty(t, registry.engines)
	})

	t.Run("Registry Engine Management", func(t *testing.T) {
		registry := NewRegistry()

		// 创建mock引擎
		mockEngine := &mockEngineForTest{
			engineType: types.EngineTypeWASM,
		}

		// 测试注册引擎
		err := registry.Register(mockEngine)
		assert.NoError(t, err)

		// 测试获取引擎
		adapter, found := registry.Get(types.EngineTypeWASM)
		assert.True(t, found)
		assert.Equal(t, mockEngine, adapter)

		// 测试获取不存在的引擎
		_, found = registry.Get(types.EngineTypeONNX)
		assert.False(t, found)

		// 测试列出引擎
		engines := registry.List()
		assert.Len(t, engines, 1)
		assert.Contains(t, engines, types.EngineTypeWASM)
	})

	t.Run("Registry Duplicate Registration", func(t *testing.T) {
		registry := NewRegistry()

		mockEngine1 := &mockEngineForTest{engineType: types.EngineTypeWASM}
		mockEngine2 := &mockEngineForTest{engineType: types.EngineTypeWASM}

		// 第一次注册成功
		err1 := registry.Register(mockEngine1)
		assert.NoError(t, err1)

		// 重复注册同类型引擎应该失败
		err2 := registry.Register(mockEngine2)
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "already registered")
	})
}

// TestEngineMetrics 测试引擎指标功能
func TestEngineMetrics(t *testing.T) {
	t.Run("Metrics Creation", func(t *testing.T) {
		metrics := newEngineMetrics()

		assert.NotNil(t, metrics)
		assert.NotNil(t, metrics.byEngine)
		assert.Empty(t, metrics.byEngine)
	})

	t.Run("Metrics Recording", func(t *testing.T) {
		metrics := newEngineMetrics()

		// 记录成功执行
		metrics.record(types.EngineTypeWASM, true, 100000000, nil) // 100ms in nanoseconds

		stats := metrics.GetStats()
		wasmStats, exists := stats[types.EngineTypeWASM]
		assert.True(t, exists)
		assert.Equal(t, uint64(1), wasmStats.ExecutionCount)
		assert.Equal(t, uint64(1), wasmStats.SuccessCount)
		assert.Equal(t, uint64(0), wasmStats.FailureCount)
		assert.Equal(t, uint64(100), wasmStats.LastDurationMs)
		assert.Empty(t, wasmStats.LastError)

		// 记录失败执行
		testErr := assert.AnError
		metrics.record(types.EngineTypeWASM, false, 200000000, testErr) // 200ms

		stats = metrics.GetStats()
		wasmStats, exists = stats[types.EngineTypeWASM]
		assert.True(t, exists)
		assert.Equal(t, uint64(2), wasmStats.ExecutionCount)
		assert.Equal(t, uint64(1), wasmStats.SuccessCount)
		assert.Equal(t, uint64(1), wasmStats.FailureCount)
		assert.Equal(t, uint64(200), wasmStats.LastDurationMs)
		assert.Equal(t, testErr.Error(), wasmStats.LastError)
	})

	t.Run("Metrics for Non-existent Engine", func(t *testing.T) {
		metrics := newEngineMetrics()

		// 获取不存在引擎的统计
		stats := metrics.GetStats()
		_, exists := stats[types.EngineTypeONNX]
		assert.False(t, exists, "Non-existent engine should not have stats")
	})
}

// TestDispatcher 测试调度器基本功能
func TestDispatcher(t *testing.T) {
	t.Run("Dispatcher Creation", func(t *testing.T) {
		// 创建一个简单的引擎管理器用于Dispatcher
		registry := NewRegistry()
		engineManager := NewEngineManager(registry)

		dispatcher := NewDispatcher(engineManager)

		assert.NotNil(t, dispatcher)
		// 由于Dispatcher的内部实现可能比较复杂，这里只测试基本创建
	})
}

// mockEngineForTest 测试用的简单引擎实现
type mockEngineForTest struct {
	engineType   types.EngineType
	initialized  bool
	hostBound    bool
	executeCount int
}

func (m *mockEngineForTest) GetEngineType() types.EngineType {
	return m.engineType
}

func (m *mockEngineForTest) Initialize(config map[string]any) error {
	m.initialized = true
	return nil
}

func (m *mockEngineForTest) BindHost(binding execiface.HostBinding) error {
	m.hostBound = true
	return nil
}

func (m *mockEngineForTest) Execute(params types.ExecutionParams) (*types.ExecutionResult, error) {
	m.executeCount++
	return &types.ExecutionResult{
		Success:    true,
		ReturnData: []byte("test result"),
		Consumed:   uint64(m.executeCount * 100),
		Metadata: map[string]any{
			"engine":     string(m.engineType),
			"call_count": m.executeCount,
		},
	}, nil
}

func (m *mockEngineForTest) Close() error {
	return nil
}

// TestEngineRegistration 测试引擎注册流程
func TestEngineRegistration(t *testing.T) {
	t.Run("Multiple Engine Types", func(t *testing.T) {
		registry := NewRegistry()

		// 注册多种类型的引擎
		wasmEngine := &mockEngineForTest{engineType: types.EngineTypeWASM}
		onnxEngine := &mockEngineForTest{engineType: types.EngineTypeONNX}

		err1 := registry.Register(wasmEngine)
		err2 := registry.Register(onnxEngine)

		assert.NoError(t, err1)
		assert.NoError(t, err2)

		// 验证都已注册
		engines := registry.List()
		assert.Len(t, engines, 2)
		assert.Contains(t, engines, types.EngineTypeWASM)
		assert.Contains(t, engines, types.EngineTypeONNX)
	})

	t.Run("Engine Lifecycle", func(t *testing.T) {
		mockEngine := &mockEngineForTest{engineType: types.EngineTypeWASM}

		// 测试初始化
		err := mockEngine.Initialize(map[string]any{"test": "config"})
		assert.NoError(t, err)
		assert.True(t, mockEngine.initialized)

		// 测试执行
		params := types.ExecutionParams{
			ResourceID: []byte("test"),
			Entry:      "main",
		}

		result, err := mockEngine.Execute(params)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Success)
		assert.Equal(t, 1, mockEngine.executeCount)

		// 测试关闭
		err = mockEngine.Close()
		assert.NoError(t, err)
	})
}

// TestConcurrentAccess 测试并发访问安全性
func TestConcurrentAccess(t *testing.T) {
	t.Run("Concurrent Registry Access", func(t *testing.T) {
		registry := NewRegistry()

		// 先注册一个引擎
		mockEngine := &mockEngineForTest{engineType: types.EngineTypeWASM}
		err := registry.Register(mockEngine)
		require.NoError(t, err)

		// 并发访问测试
		done := make(chan bool, 10)

		// 启动多个goroutine并发访问注册表
		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()

				// 并发读取
				_, found := registry.Get(types.EngineTypeWASM)
				assert.True(t, found)

				engines := registry.List()
				assert.Contains(t, engines, types.EngineTypeWASM)
			}()
		}

		// 等待所有goroutine完成
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("Concurrent Metrics Access", func(t *testing.T) {
		metrics := newEngineMetrics()
		done := make(chan bool, 10)

		// 并发记录指标
		for i := 0; i < 10; i++ {
			go func(index int) {
				defer func() { done <- true }()

				// 并发记录
				metrics.record(types.EngineTypeWASM, index%2 == 0, time.Duration(index*1000000), nil)

				// 并发读取
				stats := metrics.GetStats()
				assert.NotNil(t, stats)
			}(i)
		}

		// 等待所有goroutine完成
		for i := 0; i < 10; i++ {
			<-done
		}

		// 验证最终统计
		finalStatsMap := metrics.GetStats()
		finalStats, exists := finalStatsMap[types.EngineTypeWASM]
		assert.True(t, exists)
		assert.Equal(t, uint64(10), finalStats.ExecutionCount)
		assert.Equal(t, uint64(5), finalStats.SuccessCount)
		assert.Equal(t, uint64(5), finalStats.FailureCount)
	})
}
