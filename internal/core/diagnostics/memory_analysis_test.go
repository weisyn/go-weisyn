package diagnostics

import (
	"testing"
	"time"
)

func TestGetRSSBytes(t *testing.T) {
	rss := GetRSSBytes()
	// RSS 应该大于 0（至少在 darwin 和 linux 上）
	if rss == 0 {
		t.Skip("GetRSSBytes returns 0 on this platform")
	}
	t.Logf("Current RSS: %d bytes (%d MB)", rss, rss/1024/1024)
}

func TestGetRSSMB(t *testing.T) {
	rssMB := GetRSSMB()
	t.Logf("Current RSS: %d MB", rssMB)
}

func TestGetMemoryStats(t *testing.T) {
	stats := GetMemoryStats()

	if stats == nil {
		t.Fatal("GetMemoryStats returned nil")
	}

	// 基本检查
	if stats.HeapAlloc == 0 {
		t.Error("HeapAlloc should not be 0")
	}
	if stats.Goroutines == 0 {
		t.Error("Goroutines should not be 0")
	}
	if stats.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	t.Logf("MemoryStats: HeapAlloc=%d MB, RSS=%d MB, Goroutines=%d",
		stats.HeapAlloc/1024/1024, stats.RSS/1024/1024, stats.Goroutines)
}

func TestMemoryProfile(t *testing.T) {
	profile := MemoryProfile()
	if profile == "" {
		t.Error("MemoryProfile should not return empty string")
	}
	if len(profile) < 100 {
		t.Error("MemoryProfile should return substantial content")
	}
	t.Logf("Profile length: %d characters", len(profile))
}

func TestForceGCAndReport(t *testing.T) {
	before, after, report := ForceGCAndReport()

	if before == nil || after == nil {
		t.Fatal("ForceGCAndReport should return non-nil stats")
	}
	if report == "" {
		t.Error("ForceGCAndReport should return non-empty report")
	}

	// GC 次数应该增加
	if after.NumGC < before.NumGC {
		t.Error("NumGC should increase after GC")
	}

	t.Logf("GC count: %d -> %d", before.NumGC, after.NumGC)
}

func TestCompareMemoryStats(t *testing.T) {
	before := GetMemoryStats()
	time.Sleep(100 * time.Millisecond)
	after := GetMemoryStats()

	report := CompareMemoryStats(before, after)
	if report == "" {
		t.Error("CompareMemoryStats should return non-empty report")
	}
	if len(report) < 100 {
		t.Error("CompareMemoryStats should return substantial content")
	}
}

func TestAutoHeapProfiler(t *testing.T) {
	config := &AutoHeapProfileConfig{
		Enabled:        true,
		RSSThresholdMB: 999999, // 设置一个很高的阈值，确保不会触发
		OutputDir:      t.TempDir(),
		MaxProfiles:    5,
		MinInterval:    1 * time.Second,
	}

	profiler := NewAutoHeapProfiler(config)
	if profiler == nil {
		t.Fatal("NewAutoHeapProfiler should return non-nil")
	}

	// 由于阈值很高，不应该触发 dump
	dumped, filepath, err := profiler.CheckAndDump()
	if err != nil {
		t.Errorf("CheckAndDump should not return error: %v", err)
	}
	if dumped {
		t.Errorf("Should not dump when RSS is below threshold")
	}
	if filepath != "" {
		t.Errorf("Filepath should be empty when not dumped")
	}

	// 测试统计
	stats := profiler.Stats()
	if stats == nil {
		t.Error("Stats should return non-nil")
	}
	if !stats["enabled"].(bool) {
		t.Error("Stats should show enabled=true")
	}
}

func TestRSSTracker(t *testing.T) {
	tracker := NewRSSTracker(10, 3072, 4096)
	if tracker == nil {
		t.Fatal("NewRSSTracker should return non-nil")
	}

	// 添加几个样本
	for i := 0; i < 5; i++ {
		tracker.AddSample()
		time.Sleep(10 * time.Millisecond)
	}

	samples := tracker.GetSamples()
	if len(samples) != 5 {
		t.Errorf("Expected 5 samples, got %d", len(samples))
	}

	// 测试分析
	report := tracker.AnalyzeGrowth()
	if report == nil {
		t.Fatal("AnalyzeGrowth should return non-nil")
	}
	if report.SampleCount != 5 {
		t.Errorf("Expected SampleCount=5, got %d", report.SampleCount)
	}
	if report.HealthLevel == "" {
		t.Error("HealthLevel should not be empty")
	}

	t.Logf("RSS Growth Report: HealthLevel=%s, GrowthPerHour=%.2f MB/h",
		report.HealthLevel, report.RSSGrowthPerHour)
}

func TestRSSTracker_LRUEviction(t *testing.T) {
	// 创建只保留 5 个样本的追踪器
	tracker := NewRSSTracker(5, 3072, 4096)

	// 添加 10 个样本
	for i := 0; i < 10; i++ {
		tracker.AddSample()
	}

	samples := tracker.GetSamples()
	if len(samples) != 5 {
		t.Errorf("Expected 5 samples after LRU eviction, got %d", len(samples))
	}
}

func TestRSSTracker_GenerateReport(t *testing.T) {
	tracker := NewRSSTracker(10, 3072, 4096)

	// 添加几个样本
	for i := 0; i < 3; i++ {
		tracker.AddSample()
		time.Sleep(10 * time.Millisecond)
	}

	report := tracker.GenerateReport()
	if report == "" {
		t.Error("GenerateReport should return non-empty string")
	}
	if len(report) < 100 {
		t.Error("GenerateReport should return substantial content")
	}
}

func TestRSSTracker_Clear(t *testing.T) {
	tracker := NewRSSTracker(10, 3072, 4096)

	// 添加样本
	tracker.AddSample()
	tracker.AddSample()

	if len(tracker.GetSamples()) != 2 {
		t.Error("Should have 2 samples before clear")
	}

	tracker.Clear()

	if len(tracker.GetSamples()) != 0 {
		t.Error("Should have 0 samples after clear")
	}
}

func TestAnalyzeMemoryHealth(t *testing.T) {
	tests := []struct {
		name     string
		stats    *MemoryStats
		wantIssue bool
	}{
		{
			name: "healthy",
			stats: &MemoryStats{
				RSS:        1024 * 1024 * 1024, // 1GB
				HeapAlloc:  512 * 1024 * 1024,  // 512MB
				Goroutines: 100,
			},
			wantIssue: false,
		},
		{
			name: "high RSS",
			stats: &MemoryStats{
				RSS:        5 * 1024 * 1024 * 1024, // 5GB
				HeapAlloc:  512 * 1024 * 1024,      // 512MB
				Goroutines: 100,
			},
			wantIssue: true,
		},
		{
			name: "many goroutines",
			stats: &MemoryStats{
				RSS:        1024 * 1024 * 1024, // 1GB
				HeapAlloc:  512 * 1024 * 1024,  // 512MB
				Goroutines: 6000,
			},
			wantIssue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzeMemoryHealth(tt.stats)
			hasIssue := len(result) > 0 && (result != "  ✅ 内存使用正常")
			if hasIssue != tt.wantIssue {
				t.Errorf("analyzeMemoryHealth() hasIssue=%v, want %v", hasIssue, tt.wantIssue)
			}
		})
	}
}

func TestCheckMemoryPressure(t *testing.T) {
	level := CheckMemoryPressure(3072, 4096)
	// 当前环境应该在正常范围内
	if level != "none" && level != "soft" && level != "hard" {
		t.Errorf("CheckMemoryPressure returned invalid level: %s", level)
	}
	t.Logf("Current memory pressure level: %s", level)
}

func TestMitigateMemoryPressure(t *testing.T) {
	beforeMB, afterMB := MitigateMemoryPressure(false)
	t.Logf("MitigateMemoryPressure (normal): %d MB -> %d MB", beforeMB, afterMB)

	beforeMB, afterMB = MitigateMemoryPressure(true)
	t.Logf("MitigateMemoryPressure (aggressive): %d MB -> %d MB", beforeMB, afterMB)
}

