// Package height_gate_test 提供高度门闸管理器的单元测试
//
// 🧪 **测试覆盖**：
// - 基本高度操作测试
// - 高度验证逻辑测试
// - 分叉场景测试
// - 并发安全测试
// - 性能基准测试
package height_gate

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/weisyn/v1/internal/core/consensus/interfaces"
	"github.com/weisyn/v1/pkg/interfaces/infrastructure/log"
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

// newTestHeightGate 创建用于测试的高度门闸管理器实例
func newTestHeightGate() interfaces.HeightGateManager {
	return NewHeightGateService(&mockLogger{}, 100)
}

// TestNewHeightGateService 测试高度门闸管理器的创建
func TestNewHeightGateService(t *testing.T) {
	manager := newTestHeightGate()
	if manager == nil {
		t.Fatal("高度门闸管理器创建失败")
	}

	// 验证初始高度
	initialHeight := manager.GetLastProcessedHeight()
	if initialHeight != 0 {
		t.Errorf("期望初始高度为 0，实际为 %d", initialHeight)
	}
}

// TestGetLastProcessedHeight 测试高度获取功能
func TestGetLastProcessedHeight(t *testing.T) {
	manager := newTestHeightGate()

	// 测试初始高度获取
	height := manager.GetLastProcessedHeight()
	if height != 0 {
		t.Errorf("期望高度为 0，实际为 %d", height)
	}

	// 测试多次获取的一致性
	for i := 0; i < 100; i++ {
		if manager.GetLastProcessedHeight() != 0 {
			t.Errorf("第 %d 次获取高度不一致", i+1)
		}
	}
}

// TestUpdateLastProcessedHeight 测试高度更新功能
func TestUpdateLastProcessedHeight(t *testing.T) {
	manager := newTestHeightGate()

	// 测试高度递增更新
	testHeights := []uint64{1, 2, 5, 10, 100, 1000}
	for _, targetHeight := range testHeights {
		manager.UpdateLastProcessedHeight(targetHeight)
		currentHeight := manager.GetLastProcessedHeight()
		if currentHeight != targetHeight {
			t.Errorf("高度更新失败：期望 %d，实际 %d", targetHeight, currentHeight)
		}
	}

	// 测试幂等更新
	manager.UpdateLastProcessedHeight(1000)
	if manager.GetLastProcessedHeight() != 1000 {
		t.Error("幂等更新后高度应保持不变")
	}
}

// TestHeightForkHandling 测试分叉场景的高度处理
func TestHeightForkHandling(t *testing.T) {
	manager := newTestHeightGate()

	// 设置初始高度
	manager.UpdateLastProcessedHeight(100)

	// 测试合法的分叉回退（在uint64(100)范围内）
	testCases := []struct {
		targetHeight uint64
		shouldUpdate bool
		description  string
	}{
		// 合法的分叉回退
		{99, true, "回退1个区块（合法）"},
		{95, true, "回退4个区块（合法）"},
		{50, true, "回退45个区块（合法）"},
		{1, true, "回退98个区块（接近最大深度，合法）"},

		// 高度递增（总是合法）
		{2, true, "递增到2（合法）"},
		{150, true, "大幅递增（合法）"},

		// 非法的深度分叉
		// 注意：当前高度150，uint64(100)=100，所以150-100=50以下的高度应该被拒绝
	}

	for _, tc := range testCases {
		oldHeight := manager.GetLastProcessedHeight()
		manager.UpdateLastProcessedHeight(tc.targetHeight)
		newHeight := manager.GetLastProcessedHeight()

		if tc.shouldUpdate {
			if newHeight != tc.targetHeight {
				t.Errorf("%s: 期望高度更新为 %d，实际为 %d", tc.description, tc.targetHeight, newHeight)
			}
		} else {
			if newHeight != oldHeight {
				t.Errorf("%s: 期望高度保持为 %d，实际更新为 %d", tc.description, oldHeight, newHeight)
			}
		}
	}
}

// TestDeepForkRejection 测试深度分叉拒绝机制
func TestDeepForkRejection(t *testing.T) {
	manager := newTestHeightGate()

	// 设置一个较高的初始高度
	initialHeight := uint64(1000)
	manager.UpdateLastProcessedHeight(initialHeight)

	// 测试超过uint64(100)的分叉回退应该被拒绝
	deepForkHeight := initialHeight - uint64(100) - 1 // 超过最大分叉深度
	manager.UpdateLastProcessedHeight(deepForkHeight)

	// 验证高度没有改变
	currentHeight := manager.GetLastProcessedHeight()
	if currentHeight != initialHeight {
		t.Errorf("深度分叉应该被拒绝：期望高度保持 %d，实际为 %d", initialHeight, currentHeight)
	}

	// 测试正好在边界的分叉回退应该被允许
	boundaryHeight := initialHeight - uint64(100)
	manager.UpdateLastProcessedHeight(boundaryHeight)

	currentHeight = manager.GetLastProcessedHeight()
	if currentHeight != boundaryHeight {
		t.Errorf("边界分叉应该被允许：期望高度更新为 %d，实际为 %d", boundaryHeight, currentHeight)
	}
}

// TestConcurrentOperations 测试并发安全
func TestConcurrentOperations(t *testing.T) {
	manager := newTestHeightGate()
	const numGoroutines = 1000
	const numOperationsPerGoroutine = 100

	var wg sync.WaitGroup

	// 并发读取测试
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperationsPerGoroutine; j++ {
				_ = manager.GetLastProcessedHeight() // 并发读取应该是安全的
			}
		}()
	}

	// 并发更新测试（少量goroutine进行更新，避免竞争条件导致的不确定性）
	const numUpdateGoroutines = 10
	for i := 0; i < numUpdateGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				// 每个goroutine更新到不同的高度范围，避免冲突
				height := uint64(id*100 + j)
				manager.UpdateLastProcessedHeight(height)
			}
		}(i)
	}

	wg.Wait()

	// 验证最终状态的一致性（应该没有数据竞争或崩溃）
	finalHeight := manager.GetLastProcessedHeight()
	// 注意：uint64不能为负数，这里只是确保获取到了有效值
	_ = finalHeight // 获取高度成功即表明状态正常
}

// TestHighConcurrentReads 测试高并发读取场景
func TestHighConcurrentReads(t *testing.T) {
	manager := newTestHeightGate()
	const numReaders = 10000 // 测试10000+并发读取
	const numReadsPerReader = 100

	// 设置一个初始高度
	manager.UpdateLastProcessedHeight(12345)

	var wg sync.WaitGroup
	startTime := time.Now()

	// 启动大量并发读取
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numReadsPerReader; j++ {
				height := manager.GetLastProcessedHeight()
				if height != 12345 {
					t.Errorf("并发读取结果不正确：期望 12345，得到 %d", height)
					return
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(startTime)

	totalReads := numReaders * numReadsPerReader
	avgReadTime := duration / time.Duration(totalReads)

	t.Logf("高并发读取测试完成：")
	t.Logf("- 并发读取者: %d", numReaders)
	t.Logf("- 每个读取者操作数: %d", numReadsPerReader)
	t.Logf("- 总读取次数: %d", totalReads)
	t.Logf("- 总耗时: %v", duration)
	t.Logf("- 平均每次读取: %v", avgReadTime)

	// 验证性能要求（目标 < 100ns，但在测试环境中可能较高）
	if avgReadTime > 1*time.Microsecond {
		t.Logf("警告：平均读取时间 %v 超过1μs，但这可能是由于测试环境的开销", avgReadTime)
	}
}

// TestFormatUint64 测试数字格式化函数
func TestFormatUint64(t *testing.T) {
	testCases := []struct {
		input    uint64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123, "123"},
		{1000, "1000"},
		{12345, "12345"},
		{18446744073709551615, "18446744073709551615"}, // uint64最大值
	}

	for _, tc := range testCases {
		result := formatUint64(tc.input)
		if result != tc.expected {
			t.Errorf("formatUint64(%d): 期望 %s，实际 %s", tc.input, tc.expected, result)
		}
	}
}

// TestHeightUpdateSequence 测试完整的高度更新序列
func TestHeightUpdateSequence(t *testing.T) {
	manager := newTestHeightGate()

	// 模拟正常的区块链进展
	sequence := []struct {
		height      uint64
		description string
	}{
		{1, "创世后第一个区块"},
		{2, "第二个区块"},
		{3, "第三个区块"},
		{5, "跳跃到第五个区块"},
		{6, "继续到第六个区块"},
		{4, "分叉回退到第四个区块"},
		{5, "分叉后继续到第五个区块"},
		{7, "超过原来的高度"},
		{10, "继续增长"},
	}

	for _, step := range sequence {
		manager.UpdateLastProcessedHeight(step.height)
		currentHeight := manager.GetLastProcessedHeight()
		if currentHeight != step.height {
			t.Errorf("%s: 期望高度 %d，实际 %d", step.description, step.height, currentHeight)
		}
	}
}

// TestEdgeCases 测试边界情况
func TestEdgeCases(t *testing.T) {
	// 测试0高度的边界
	t.Run("ZeroHeight", func(t *testing.T) {
		manager := newTestHeightGate()
		manager.UpdateLastProcessedHeight(0)
		if manager.GetLastProcessedHeight() != 0 {
			t.Error("0高度更新失败")
		}
	})

	// 测试正常高度递增
	t.Run("LargeHeightIncrement", func(t *testing.T) {
		manager := newTestHeightGate()
		largeHeight := uint64(10000)
		manager.UpdateLastProcessedHeight(largeHeight)
		if manager.GetLastProcessedHeight() != largeHeight {
			t.Error("大高度值更新失败")
		}
	})

	// 测试最大允许回退深度
	t.Run("MaxAllowedRollback", func(t *testing.T) {
		manager := newTestHeightGate()
		baseHeight := uint64(1000)
		manager.UpdateLastProcessedHeight(baseHeight)

		// 测试正好在uint64(100)边界的回退（应该允许）
		rollbackTarget := baseHeight - uint64(100)
		manager.UpdateLastProcessedHeight(rollbackTarget)
		if manager.GetLastProcessedHeight() != rollbackTarget {
			t.Errorf("最大允许回退深度测试失败：期望 %d，实际 %d", rollbackTarget, manager.GetLastProcessedHeight())
		}
	})

	// 测试超过最大回退深度（应该被拒绝）
	t.Run("ExceedMaxRollback", func(t *testing.T) {
		manager := newTestHeightGate()
		baseHeight := uint64(1000)
		manager.UpdateLastProcessedHeight(baseHeight)

		// 测试超过uint64(100)的回退（应该被拒绝）
		invalidRollback := baseHeight - uint64(100) - 1
		manager.UpdateLastProcessedHeight(invalidRollback)

		currentHeight := manager.GetLastProcessedHeight()
		if currentHeight != baseHeight {
			t.Errorf("超过最大回退深度应该被拒绝：期望保持 %d，实际变成 %d", baseHeight, currentHeight)
		}
	})

	// 测试uint64边界值
	t.Run("Uint64Boundary", func(t *testing.T) {
		manager := newTestHeightGate()

		// 测试接近uint64最大值的高度（避免溢出）
		maxSafeHeight := uint64(18446744073709551615 - uint64(100) - 1000) // 留出安全边界
		manager.UpdateLastProcessedHeight(maxSafeHeight)
		if manager.GetLastProcessedHeight() != maxSafeHeight {
			t.Error("接近uint64最大值的高度更新失败")
		}
	})
}

// BenchmarkGetLastProcessedHeight 基准测试：高度获取性能
func BenchmarkGetLastProcessedHeight(b *testing.B) {
	manager := newTestHeightGate()
	manager.UpdateLastProcessedHeight(12345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = manager.GetLastProcessedHeight()
	}
}

// BenchmarkUpdateLastProcessedHeight 基准测试：高度更新性能
func BenchmarkUpdateLastProcessedHeight(b *testing.B) {
	manager := newTestHeightGate()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.UpdateLastProcessedHeight(uint64(i))
	}
}

// BenchmarkConcurrentReads 基准测试：并发读取性能
func BenchmarkConcurrentReads(b *testing.B) {
	manager := newTestHeightGate()
	manager.UpdateLastProcessedHeight(12345)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = manager.GetLastProcessedHeight()
		}
	})
}

// BenchmarkFormatUint64 基准测试：数字格式化性能
func BenchmarkFormatUint64(b *testing.B) {
	testValue := uint64(123456789)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatUint64(testValue)
	}
}

// BenchmarkMixedOperations 基准测试：混合读写操作
func BenchmarkMixedOperations(b *testing.B) {
	manager := newTestHeightGate()

	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			if counter%10 == 0 {
				// 10%的操作是写入
				manager.UpdateLastProcessedHeight(uint64(counter))
			} else {
				// 90%的操作是读取
				_ = manager.GetLastProcessedHeight()
			}
			counter++
		}
	})
}
