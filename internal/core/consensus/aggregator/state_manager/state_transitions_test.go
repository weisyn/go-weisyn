package state_manager

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
	"go.uber.org/zap"
)

// TestIdleToIdleTransition 测试 Idle -> Idle 自转换应被视为幂等成功
func TestIdleToIdleTransition(t *testing.T) {
	logger := &mockLogger{}
	manager := newStateTransitionManager(logger)

	// 初始状态应该是 Idle
	assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())

	// 尝试 Idle -> Idle 自转换应该成功（幂等）
	err := manager.transitionTo(types.AggregationStateIdle)
	assert.NoError(t, err, "Idle -> Idle 自转换不应返回错误（幂等）")

	// 状态应该保持不变
	assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())
}

// TestTransitionToIdleIfNeeded 测试幂等转换方法（兼容性测试，新代码应使用 EnsureIdle）
// Deprecated: 此测试验证旧 API，新测试应使用 TestEnsureState
func TestTransitionToIdleIfNeeded(t *testing.T) {
	logger := &mockLogger{}
	manager := newStateTransitionManager(logger)

	t.Run("从Idle状态调用应该成功（幂等）", func(t *testing.T) {
		assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())

		err := manager.transitionToIdleIfNeeded()
		assert.NoError(t, err, "幂等操作不应返回错误")
		assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())
	})

	t.Run("从Listening状态调用应该成功转换", func(t *testing.T) {
		// 先转换到 Listening
		err := manager.transitionTo(types.AggregationStateListening)
		assert.NoError(t, err)
		assert.Equal(t, types.AggregationStateListening, manager.getCurrentState())

		// 调用幂等方法应该成功转换到 Idle
		err = manager.transitionToIdleIfNeeded()
		assert.NoError(t, err)
		assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())
	})

	t.Run("从非法中间状态调用应该失败", func(t *testing.T) {
		// 重新创建 manager 以重置状态
		manager = newStateTransitionManager(logger)
		
		// 转换到 Collecting
		_ = manager.transitionTo(types.AggregationStateListening)
		_ = manager.transitionTo(types.AggregationStateCollecting)
		assert.Equal(t, types.AggregationStateCollecting, manager.getCurrentState())

		// 从 Collecting 直接转 Idle 是非法的
		err := manager.transitionToIdleIfNeeded()
		assert.Error(t, err, "Collecting -> Idle 转换应该失败")
	})
}

// TestValidStateTransitions 测试所有合法的状态转换
func TestValidStateTransitions(t *testing.T) {
	logger := &mockLogger{}

	transitions := []struct {
		from  types.AggregationState
		to    types.AggregationState
		valid bool
		path  []types.AggregationState // 到达 from 状态的路径
	}{
		{types.AggregationStateIdle, types.AggregationStateListening, true, nil},
		{types.AggregationStateIdle, types.AggregationStateError, true, nil},
		{types.AggregationStateIdle, types.AggregationStateIdle, true, nil}, // 自转换幂等
		{types.AggregationStateListening, types.AggregationStateIdle, true, []types.AggregationState{types.AggregationStateListening}},
		{types.AggregationStateDistributing, types.AggregationStateIdle, true, []types.AggregationState{
			types.AggregationStateListening,
			types.AggregationStateCollecting,
			types.AggregationStateEvaluating,
			types.AggregationStateSelecting,
			types.AggregationStateDistributing,
		}},
		{types.AggregationStateError, types.AggregationStateIdle, true, []types.AggregationState{types.AggregationStateError}},
	}

	for _, tt := range transitions {
		t.Run(fmt.Sprintf("%s -> %s", tt.from.String(), tt.to.String()), func(t *testing.T) {
			// 重置到初始状态
			manager := newStateTransitionManager(logger)

			// 如果有路径，按路径转换到目标起始状态
			if tt.path != nil {
				for _, state := range tt.path {
					err := manager.transitionTo(state)
					assert.NoError(t, err, "路径转换应该成功")
				}
				assert.Equal(t, tt.from, manager.getCurrentState())
			}

			// 执行转换
			err := manager.transitionTo(tt.to)
			if tt.valid {
				assert.NoError(t, err, "合法转换不应返回错误")
				assert.Equal(t, tt.to, manager.getCurrentState())
			} else {
				assert.Error(t, err, "非法转换应该返回错误")
			}
		})
	}
}

// TestIdempotentTransitionConcurrency 测试并发场景下的幂等转换
func TestIdempotentTransitionConcurrency(t *testing.T) {
	logger := &mockLogger{}
	manager := newStateTransitionManager(logger)

	// 转换到 Listening 状态
	err := manager.transitionTo(types.AggregationStateListening)
	assert.NoError(t, err)

	// 模拟多个 goroutine 同时尝试转换到 Idle
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = manager.transitionToIdleIfNeeded()
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 最终状态应该是 Idle
	assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())
}

// TestStateTransitionFromAllStates 测试从所有状态转换到 Idle 的场景
func TestStateTransitionFromAllStates(t *testing.T) {
	logger := &mockLogger{}

	testCases := []struct {
		name          string
		currentState  types.AggregationState
		setupPath     []types.AggregationState
		shouldSucceed bool
	}{
		{"从Idle", types.AggregationStateIdle, nil, true},
		{"从Listening", types.AggregationStateListening, []types.AggregationState{types.AggregationStateListening}, true},
		{"从Collecting", types.AggregationStateCollecting, []types.AggregationState{types.AggregationStateListening, types.AggregationStateCollecting}, false},
		{"从Evaluating", types.AggregationStateEvaluating, []types.AggregationState{
			types.AggregationStateListening,
			types.AggregationStateCollecting,
			types.AggregationStateEvaluating,
		}, false},
		{"从Selecting", types.AggregationStateSelecting, []types.AggregationState{
			types.AggregationStateListening,
			types.AggregationStateCollecting,
			types.AggregationStateEvaluating,
			types.AggregationStateSelecting,
		}, false},
		{"从Distributing", types.AggregationStateDistributing, []types.AggregationState{
			types.AggregationStateListening,
			types.AggregationStateCollecting,
			types.AggregationStateEvaluating,
			types.AggregationStateSelecting,
			types.AggregationStateDistributing,
		}, true},
		{"从Paused", types.AggregationStatePaused, []types.AggregationState{types.AggregationStateListening, types.AggregationStatePaused}, true},
		{"从Error", types.AggregationStateError, []types.AggregationState{types.AggregationStateError}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := newStateTransitionManager(logger)

			// 按路径转换到目标状态
			if tc.setupPath != nil {
				for _, state := range tc.setupPath {
					err := manager.transitionTo(state)
					assert.NoError(t, err, "路径转换应该成功")
				}
			}

			// 确认当前状态
			assert.Equal(t, tc.currentState, manager.getCurrentState())

			// 尝试使用幂等方法转换到 Idle
			err := manager.transitionToIdleIfNeeded()

			if tc.shouldSucceed {
				assert.NoError(t, err, "从 %s 转换到 Idle 应该成功", tc.currentState.String())
				assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())
			} else {
				assert.Error(t, err, "从 %s 转换到 Idle 应该失败", tc.currentState.String())
			}
		})
	}
}

// mockLogger 简单的日志模拟
type mockLogger struct{}

func (m *mockLogger) Debug(msg string)                            {}
func (m *mockLogger) Debugf(format string, args ...interface{})   {}
func (m *mockLogger) Info(msg string)                             {}
func (m *mockLogger) Infof(format string, args ...interface{})    {}
func (m *mockLogger) Warn(msg string)                             {}
func (m *mockLogger) Warnf(format string, args ...interface{})    {}
func (m *mockLogger) Error(msg string)                            {}
func (m *mockLogger) Errorf(format string, args ...interface{})   {}
func (m *mockLogger) Fatal(msg string)                            {}
func (m *mockLogger) Fatalf(format string, args ...interface{})   {}
func (m *mockLogger) With(args ...interface{}) log.Logger         { return m }
func (m *mockLogger) Sync() error                                 { return nil }
func (m *mockLogger) GetZapLogger() *zap.Logger                   { return nil }

// TestEnsureState 测试确保状态方法（新 API）
func TestEnsureState(t *testing.T) {
	logger := &mockLogger{}
	manager := newStateTransitionManager(logger)

	t.Run("确保Idle状态_当前已是Idle_应该幂等成功", func(t *testing.T) {
		assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())

		err := manager.ensureIdle()
		assert.NoError(t, err, "幂等调用不应返回错误")
		assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())
	})

	t.Run("确保Idle状态_当前是Listening_应该成功转换", func(t *testing.T) {
		// 转换到 Listening
		_ = manager.transitionTo(types.AggregationStateListening)
		assert.Equal(t, types.AggregationStateListening, manager.getCurrentState())

		// 确保 Idle
		err := manager.ensureIdle()
		assert.NoError(t, err)
		assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())
	})

	t.Run("确保Idle状态_当前是Collecting_应该失败", func(t *testing.T) {
		// 重置
		manager = newStateTransitionManager(logger)
		_ = manager.transitionTo(types.AggregationStateListening)
		_ = manager.transitionTo(types.AggregationStateCollecting)

		// Collecting 无法直接到 Idle，应该失败
		err := manager.ensureIdle()
		assert.Error(t, err, "非法路径应该失败")
	})

	t.Run("确保任意状态_当前已是目标状态_应该幂等成功", func(t *testing.T) {
		manager = newStateTransitionManager(logger)
		_ = manager.transitionTo(types.AggregationStateListening)

		err := manager.ensureState(types.AggregationStateListening)
		assert.NoError(t, err, "幂等调用不应返回错误")
		assert.Equal(t, types.AggregationStateListening, manager.getCurrentState())
	})
}

// TestTransitionVsEnsure 测试转换语义 vs 确保语义的区别
func TestTransitionVsEnsure(t *testing.T) {
	logger := &mockLogger{}
	manager := newStateTransitionManager(logger)

	t.Run("TransitionTo允许Idle到Idle自转换（幂等）", func(t *testing.T) {
		err := manager.transitionTo(types.AggregationStateIdle)
		assert.NoError(t, err, "TransitionTo 自转换应幂等成功")
		assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())
	})

	t.Run("EnsureIdle允许幂等调用", func(t *testing.T) {
		err := manager.ensureIdle()
		assert.NoError(t, err, "EnsureIdle 允许幂等调用")
		assert.Equal(t, types.AggregationStateIdle, manager.getCurrentState())
	})

	t.Run("多次EnsureIdle应该都成功", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			err := manager.ensureIdle()
			assert.NoError(t, err, "第 %d 次 EnsureIdle 应该成功", i+1)
		}
	})
}

// TestEnsureStateSemantics 测试确保状态的语义正确性
func TestEnsureStateSemantics(t *testing.T) {
	logger := &mockLogger{}

	scenarios := []struct {
		name          string
		initialState  types.AggregationState
		targetState   types.AggregationState
		setupPath     []types.AggregationState
		shouldSucceed bool
		description   string
	}{
		{
			name:          "确保Idle_从Idle",
			initialState:  types.AggregationStateIdle,
			targetState:   types.AggregationStateIdle,
			setupPath:     nil,
			shouldSucceed: true,
			description:   "幂等操作，应该成功",
		},
		{
			name:          "确保Idle_从Listening",
			initialState:  types.AggregationStateListening,
			targetState:   types.AggregationStateIdle,
			setupPath:     []types.AggregationState{types.AggregationStateListening},
			shouldSucceed: true,
			description:   "Listening -> Idle 是合法转换",
		},
		{
			name:         "确保Idle_从Distributing",
			initialState: types.AggregationStateDistributing,
			targetState:  types.AggregationStateIdle,
			setupPath: []types.AggregationState{
				types.AggregationStateListening,
				types.AggregationStateCollecting,
				types.AggregationStateEvaluating,
				types.AggregationStateSelecting,
				types.AggregationStateDistributing,
			},
			shouldSucceed: true,
			description:   "Distributing -> Idle 是合法转换",
		},
		{
			name:         "确保Idle_从Collecting",
			initialState: types.AggregationStateCollecting,
			targetState:  types.AggregationStateIdle,
			setupPath: []types.AggregationState{
				types.AggregationStateListening,
				types.AggregationStateCollecting,
			},
			shouldSucceed: false,
			description:   "Collecting -> Idle 是非法转换",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			manager := newStateTransitionManager(logger)

			// 设置初始状态
			if sc.setupPath != nil {
				for _, state := range sc.setupPath {
					_ = manager.transitionTo(state)
				}
			}

			assert.Equal(t, sc.initialState, manager.getCurrentState())

			// 执行确保状态
			err := manager.ensureState(sc.targetState)

			if sc.shouldSucceed {
				assert.NoError(t, err, sc.description)
				assert.Equal(t, sc.targetState, manager.getCurrentState())
			} else {
				assert.Error(t, err, sc.description)
			}
		})
	}
}

