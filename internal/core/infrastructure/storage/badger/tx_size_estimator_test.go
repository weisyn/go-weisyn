package badger

import (
	"testing"
)

// TestTxSizeEstimator_BasicOperations 测试基本操作
func TestTxSizeEstimator_BasicOperations(t *testing.T) {
	est := NewTxSizeEstimator(1000)

	// 测试初始状态
	if est.GetCurrentSize() != 0 {
		t.Errorf("初始大小应该为0, 实际为 %d", est.GetCurrentSize())
	}

	// 测试写入
	est.AddWrite(10, 100)
	expected := uint64(130) // 10 + 100 + 20 (开销)
	if est.GetCurrentSize() != expected {
		t.Errorf("写入后大小应该为 %d, 实际为 %d", expected, est.GetCurrentSize())
	}

	// 测试删除
	est.AddDelete(50)
	expected += 60 // 50 + 10 (开销)
	if est.GetCurrentSize() != expected {
		t.Errorf("删除后大小应该为 %d, 实际为 %d", expected, est.GetCurrentSize())
	}

	// 测试重置
	est.Reset()
	if est.GetCurrentSize() != 0 {
		t.Errorf("重置后大小应该为0, 实际为 %d", est.GetCurrentSize())
	}
}

// TestTxSizeEstimator_NearLimit 测试接近限制检测
func TestTxSizeEstimator_NearLimit(t *testing.T) {
	est := NewTxSizeEstimator(1000)

	// 不接近限制
	est.AddWrite(10, 100)
	if est.IsNearLimit() {
		t.Error("当前大小远低于限制，IsNearLimit 应该返回 false")
	}

	// 接近限制（超过80%）
	est.AddWrite(100, 700) // 总计 130 + 820 = 950
	if !est.IsNearLimit() {
		t.Error("当前大小超过80%，IsNearLimit 应该返回 true")
	}

	// 测试使用百分比
	percent := est.GetUsagePercent()
	if percent < 80 || percent > 100 {
		t.Errorf("使用百分比应该在 80-100 之间, 实际为 %.2f", percent)
	}
}

// TestTxSizeEstimator_GetRemainingSize 测试剩余空间计算
func TestTxSizeEstimator_GetRemainingSize(t *testing.T) {
	est := NewTxSizeEstimator(1000)

	// 初始剩余空间
	if est.GetRemainingSize() != 1000 {
		t.Errorf("初始剩余空间应该为1000, 实际为 %d", est.GetRemainingSize())
	}

	// 写入后剩余空间
	est.AddWrite(10, 100)
	remaining := est.GetRemainingSize()
	expected := uint64(870) // 1000 - 130
	if remaining != expected {
		t.Errorf("剩余空间应该为 %d, 实际为 %d", expected, remaining)
	}

	// 超过限制时剩余空间为0
	est.AddWrite(500, 500)
	if est.GetRemainingSize() != 0 {
		t.Errorf("超过限制时剩余空间应该为0, 实际为 %d", est.GetRemainingSize())
	}
}

// TestTxSizeEstimator_DefaultMaxSize 测试默认最大值
func TestTxSizeEstimator_DefaultMaxSize(t *testing.T) {
	est := NewTxSizeEstimator(0)

	expectedMaxSize := uint64(10 * 1024 * 1024) // 10MB
	if est.GetMaxSize() != expectedMaxSize {
		t.Errorf("默认最大值应该为 %d, 实际为 %d", expectedMaxSize, est.GetMaxSize())
	}
}

// TestTxSizeEstimator_MultipleOperations 测试多次操作
func TestTxSizeEstimator_MultipleOperations(t *testing.T) {
	est := NewTxSizeEstimator(10000)

	// 模拟批量写入
	for i := 0; i < 100; i++ {
		est.AddWrite(20, 80) // 每次 120 字节
	}

	expected := uint64(100 * 120) // 12000
	if est.GetCurrentSize() != expected {
		t.Errorf("100次写入后大小应该为 %d, 实际为 %d", expected, est.GetCurrentSize())
	}

	// 应该超过限制
	if !est.IsNearLimit() {
		t.Error("大小超过限制，IsNearLimit 应该返回 true")
	}
}

// TestTxSizeEstimator_Concurrent 测试并发安全性
func TestTxSizeEstimator_Concurrent(t *testing.T) {
	est := NewTxSizeEstimator(100000)

	// 启动多个 goroutine 并发写入
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				est.AddWrite(10, 20)
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证总大小（10个goroutine * 100次 * 50字节）
	expected := uint64(10 * 100 * 50) // 50000
	if est.GetCurrentSize() != expected {
		t.Errorf("并发写入后大小应该为 %d, 实际为 %d", expected, est.GetCurrentSize())
	}
}

// TestTxSizeEstimator_ThresholdCalculation 测试阈值计算
func TestTxSizeEstimator_ThresholdCalculation(t *testing.T) {
	est := NewTxSizeEstimator(1000)

	// 测试79%（不接近限制）
	est.AddWrite(10, 769) // 799字节
	if est.IsNearLimit() {
		t.Errorf("79.9%% 不应该接近限制, 当前: %.1f%%", est.GetUsagePercent())
	}

	// 测试80%（正好达到阈值）
	est.AddWrite(1, 0) // 再加21字节，达到820
	if !est.IsNearLimit() {
		t.Errorf("80%%以上应该接近限制, 当前: %.1f%%", est.GetUsagePercent())
	}
}

