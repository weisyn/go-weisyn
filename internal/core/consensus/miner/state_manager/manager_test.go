// Package state_manager_test 提供状态管理器的单元测试
//
// 🧪 **测试覆盖**：
// - 状态获取/设置的基本功能测试
// - 状态转换验证测试
// - 并发安全测试
// - 边界条件测试
package state_manager

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
	"github.com/weisyn/v1/pkg/types"
)

// mockLogger 测试用的模拟日志器，实现完整的Logger接口
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
func (m *mockLogger) GetZapLogger() *zap.Logger                 { return nil }

// newTestStateManager 创建用于测试的状态管理器实例
func newTestStateManager() interfaces.MinerStateManager {
	return NewMinerStateService(&mockLogger{})
}

// TestNewMinerStateService 测试状态管理器的创建
func TestNewMinerStateService(t *testing.T) {
	manager := newTestStateManager()
	if manager == nil {
		t.Fatal("状态管理器创建失败")
	}

	// 验证初始状态
	initialState := manager.GetMinerState()
	if initialState != types.MinerStateIdle {
		t.Errorf("期望初始状态为 %v，实际为 %v", types.MinerStateIdle, initialState)
	}
}

// TestGetMinerState 测试状态获取功能
func TestGetMinerState(t *testing.T) {
	manager := newTestStateManager()

	// 测试初始状态获取
	state := manager.GetMinerState()
	if state != types.MinerStateIdle {
		t.Errorf("期望状态为 %v，实际为 %v", types.MinerStateIdle, state)
	}

	// 测试多次获取的一致性
	for i := 0; i < 10; i++ {
		if manager.GetMinerState() != types.MinerStateIdle {
			t.Errorf("第 %d 次获取状态不一致", i+1)
		}
	}
}

// TestSetMinerState 测试状态设置功能
func TestSetMinerState(t *testing.T) {
	manager := newTestStateManager()

	// 测试合法状态转换：Idle -> Mining
	err := manager.SetMinerState(types.MinerStateActive)
	if err != nil {
		t.Errorf("合法状态转换失败：%v", err)
	}

	// 验证状态已更新
	if manager.GetMinerState() != types.MinerStateActive {
		t.Errorf("状态未正确更新")
	}

	// 测试进一步的合法转换：Mining -> Paused
	err = manager.SetMinerState(types.MinerStatePaused)
	if err != nil {
		t.Errorf("合法状态转换失败：%v", err)
	}

	if manager.GetMinerState() != types.MinerStatePaused {
		t.Errorf("状态未正确更新")
	}
}

// TestSetMinerState_InvalidTransitions 测试非法状态转换
func TestSetMinerState_InvalidTransitions(t *testing.T) {
	manager := newTestStateManager()

	// 测试非法转换：Idle -> Paused
	err := manager.SetMinerState(types.MinerStatePaused)
	if err == nil {
		t.Error("应该拒绝非法状态转换 Idle -> Paused")
	}

	// 验证状态未改变
	if manager.GetMinerState() != types.MinerStateIdle {
		t.Error("非法转换后状态不应改变")
	}

	// 测试非法转换：Idle -> Stopping
	err = manager.SetMinerState(types.MinerStateStopping)
	if err == nil {
		t.Error("应该拒绝非法状态转换 Idle -> Stopping")
	}
}

// TestValidateStateTransition 测试状态转换验证
func TestValidateStateTransition(t *testing.T) {
	manager := newTestStateManager()

	// 测试合法转换
	testCases := []struct {
		from     interfaces.MinerInternalState
		to       interfaces.MinerInternalState
		expected bool
		name     string
	}{
		// 合法转换
		{types.MinerStateIdle, types.MinerStateActive, true, "Idle -> Mining"},
		{types.MinerStateActive, types.MinerStatePaused, true, "Mining -> Paused"},
		{types.MinerStateActive, types.MinerStateStopping, true, "Mining -> Stopping"},
		{types.MinerStatePaused, types.MinerStateActive, true, "Paused -> Mining"},
		{types.MinerStatePaused, types.MinerStateStopping, true, "Paused -> Stopping"},
		{types.MinerStateStopping, types.MinerStateIdle, true, "Stopping -> Idle"},

		// 相同状态转换（幂等性）
		{types.MinerStateIdle, types.MinerStateIdle, true, "Idle -> Idle"},
		{types.MinerStateActive, types.MinerStateActive, true, "Mining -> Mining"},
		{types.MinerStatePaused, types.MinerStatePaused, true, "Paused -> Paused"},
		{types.MinerStateStopping, types.MinerStateStopping, true, "Stopping -> Stopping"},

		// 非法转换
		{types.MinerStateIdle, types.MinerStatePaused, false, "Idle -> Paused (非法)"},
		{types.MinerStateIdle, types.MinerStateStopping, false, "Idle -> Stopping (非法)"},
		{types.MinerStateActive, types.MinerStateIdle, false, "Mining -> Idle (非法)"},
		{types.MinerStatePaused, types.MinerStateIdle, false, "Paused -> Idle (非法)"},
		{types.MinerStateStopping, types.MinerStateActive, false, "Stopping -> Mining (非法)"},
		{types.MinerStateStopping, types.MinerStatePaused, false, "Stopping -> Paused (非法)"},
	}

	for _, tc := range testCases {
		result := manager.ValidateStateTransition(tc.from, tc.to)
		if result != tc.expected {
			t.Errorf("转换验证 %s: 期望 %v，实际 %v", tc.name, tc.expected, result)
		}
	}
}

// TestConcurrentStateOperations 测试并发安全
func TestConcurrentStateOperations(t *testing.T) {
	manager := newTestStateManager()
	const numGoroutines = 100
	const numOperations = 10

	var wg sync.WaitGroup

	// 启动多个并发goroutine执行状态操作
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// 并发读取状态
				_ = manager.GetMinerState()

				// 并发验证状态转换
				_ = manager.ValidateStateTransition(types.MinerStateIdle, types.MinerStateActive)
			}
		}(i)
	}

	// 同时有一个goroutine进行状态更新
	wg.Add(1)
	go func() {
		defer wg.Done()

		// 执行一系列状态转换
		transitions := []interfaces.MinerInternalState{
			types.MinerStateActive, types.MinerStatePaused, types.MinerStateActive, types.MinerStateStopping, types.MinerStateIdle,
		}

		for _, state := range transitions {
			_ = manager.SetMinerState(state)
			time.Sleep(1 * time.Millisecond) // 短暂延迟模拟真实场景
		}
	}()

	wg.Wait()

	// 验证最终状态的一致性
	finalState := manager.GetMinerState()
	if finalState != types.MinerStateIdle {
		t.Errorf("并发测试后期望最终状态为 %v，实际为 %v", types.MinerStateIdle, finalState)
	}
}

// TestStateTransitionFlow 测试完整状态转换流程
func TestStateTransitionFlow(t *testing.T) {
	manager := newTestStateManager()

	// 完整的状态转换流程：Idle -> Mining -> Paused -> Mining -> Stopping -> Idle
	transitionFlow := []struct {
		targetState interfaces.MinerInternalState
		expectError bool
		description string
	}{
		{types.MinerStateActive, false, "启动挖矿"},
		{types.MinerStatePaused, false, "暂停挖矿"},
		{types.MinerStateActive, false, "恢复挖矿"},
		{types.MinerStateStopping, false, "停止挖矿"},
		{types.MinerStateIdle, false, "回到空闲状态"},
	}

	for i, step := range transitionFlow {
		err := manager.SetMinerState(step.targetState)

		if step.expectError && err == nil {
			t.Errorf("步骤 %d (%s): 期望错误但没有发生", i+1, step.description)
		}

		if !step.expectError && err != nil {
			t.Errorf("步骤 %d (%s): 意外的错误: %v", i+1, step.description, err)
		}

		if err == nil {
			currentState := manager.GetMinerState()
			if currentState != step.targetState {
				t.Errorf("步骤 %d (%s): 期望状态 %v，实际状态 %v",
					i+1, step.description, step.targetState, currentState)
			}
		}
	}
}

// TestStateTransitionIdempotency 测试状态转换的幂等性
func TestStateTransitionIdempotency(t *testing.T) {
	manager := newTestStateManager()

	// 测试相同状态的多次设置
	states := []interfaces.MinerInternalState{
		types.MinerStateIdle, types.MinerStateActive, types.MinerStatePaused, types.MinerStateStopping,
	}

	for _, state := range states {
		// 首先转换到目标状态（可能需要中间步骤）
		switch state {
		case types.MinerStateActive:
			_ = manager.SetMinerState(types.MinerStateActive)
		case types.MinerStatePaused:
			_ = manager.SetMinerState(types.MinerStateActive)
			_ = manager.SetMinerState(types.MinerStatePaused)
		case types.MinerStateStopping:
			_ = manager.SetMinerState(types.MinerStateActive)
			_ = manager.SetMinerState(types.MinerStateStopping)
		}

		// 测试同一状态的重复设置（幂等性）
		for i := 0; i < 5; i++ {
			err := manager.SetMinerState(state)
			if err != nil {
				t.Errorf("状态 %v 的幂等设置失败: %v", state, err)
			}

			currentState := manager.GetMinerState()
			if currentState != state {
				t.Errorf("幂等设置后状态不一致: 期望 %v，实际 %v", state, currentState)
			}
		}
	}
}

// BenchmarkGetMinerState 基准测试：状态获取性能
func BenchmarkGetMinerState(b *testing.B) {
	manager := newTestStateManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GetMinerState()
	}
}

// BenchmarkSetMinerState 基准测试：状态设置性能
func BenchmarkSetMinerState(b *testing.B) {
	manager := newTestStateManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 在Mining和Paused之间切换（都是合法转换）
		if i%2 == 0 {
			_ = manager.SetMinerState(types.MinerStateActive)
		} else {
			_ = manager.SetMinerState(types.MinerStatePaused)
		}
	}
}

// BenchmarkValidateStateTransition 基准测试：状态转换验证性能
func BenchmarkValidateStateTransition(b *testing.B) {
	manager := newTestStateManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.ValidateStateTransition(types.MinerStateIdle, types.MinerStateActive)
	}
}
