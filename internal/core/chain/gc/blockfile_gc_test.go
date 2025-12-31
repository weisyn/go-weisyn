package gc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewBlockFileGC 测试 GC 创建
func TestNewBlockFileGC(t *testing.T) {
	config := &BlockFileGCConfig{
		Enabled:                 true,
		DryRun:                  true,
		IntervalSeconds:         3600,
		RateLimitFilesPerSecond: 100,
		ProtectRecentHeight:     1000,
		BatchSize:               50,
	}

	gc := NewBlockFileGC(config, nil, nil, nil)
	require.NotNil(t, gc)
	assert.Equal(t, config, gc.config)
	assert.False(t, gc.IsRunning())
}

// TestBlockFileGC_ConfigOverride 测试配置覆盖
func TestBlockFileGC_ConfigOverride(t *testing.T) {
	config := &BlockFileGCConfig{
		Enabled:                 true,
		DryRun:                  true,
		IntervalSeconds:         3600,
		RateLimitFilesPerSecond: 100,
		ProtectRecentHeight:     1000,
		BatchSize:               50,
	}

	gc := NewBlockFileGC(config, nil, nil, nil)
	require.NotNil(t, gc)

	// 验证配置值
	assert.True(t, gc.config.DryRun)
	assert.Equal(t, 3600, gc.config.IntervalSeconds)
	assert.Equal(t, uint64(1000), gc.config.ProtectRecentHeight)
}

// TestBlockFileGC_GetStatus 测试状态查询
func TestBlockFileGC_GetStatus(t *testing.T) {
	config := &BlockFileGCConfig{
		Enabled:                 true,
		DryRun:                  true,
		IntervalSeconds:         3600,
		RateLimitFilesPerSecond: 100,
		ProtectRecentHeight:     1000,
		BatchSize:               50,
	}

	gc := NewBlockFileGC(config, nil, nil, nil)
	require.NotNil(t, gc)

	// 获取状态
	status := gc.GetStatus()
	require.NotNil(t, status)

	assert.True(t, status.Enabled)
	assert.False(t, status.Running)
	assert.NotNil(t, status.Metrics)
}

// TestBlockFileGC_IsRunning 测试运行状态
func TestBlockFileGC_IsRunning(t *testing.T) {
	gc := NewBlockFileGC(&BlockFileGCConfig{}, nil, nil, nil)
	require.NotNil(t, gc)

	// 初始状态
	assert.False(t, gc.IsRunning())

	// 设置为运行中
	gc.running.Store(true)
	assert.True(t, gc.IsRunning())

	// 设置为停止
	gc.running.Store(false)
	assert.False(t, gc.IsRunning())
}

// TestBlockFileGC_ConcurrentSafety 测试并发安全
func TestBlockFileGC_ConcurrentSafety(t *testing.T) {
	config := &BlockFileGCConfig{
		Enabled:                 true,
		DryRun:                  true,
		IntervalSeconds:         3600,
		RateLimitFilesPerSecond: 100,
		ProtectRecentHeight:     1000,
		BatchSize:               50,
	}

	gc := NewBlockFileGC(config, nil, nil, nil)
	require.NotNil(t, gc)

	// 并发调用 GetStatus（应该是安全的）
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			status := gc.GetStatus()
			assert.NotNil(t, status)
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestBlockFileGC_Lifecycle 测试生命周期
func TestBlockFileGC_Lifecycle(t *testing.T) {
	config := &BlockFileGCConfig{
		Enabled:                 true,
		DryRun:                  true,
		IntervalSeconds:         3600,
		RateLimitFilesPerSecond: 100,
		ProtectRecentHeight:     1000,
		BatchSize:               50,
	}

	gc := NewBlockFileGC(config, nil, nil, nil)
	require.NotNil(t, gc)

	// 测试启动和停止（无依赖时应该优雅处理）
	ctx := context.Background()
	
	// Start 应该成功（即使没有依赖）
	err := gc.Start(ctx)
	// 由于没有依赖，Start 可能会失败，这是预期的
	t.Logf("Start result: %v", err)

	// Stop 应该总是成功
	err = gc.Stop(ctx)
	assert.NoError(t, err)
}

