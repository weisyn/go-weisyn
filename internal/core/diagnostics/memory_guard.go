// Package diagnostics provides diagnostic and analysis tools for system health monitoring.
package diagnostics

import (
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
)

// ============================================================================
//                       内存保护守护程序
// ============================================================================

// MemoryGuardConfig 内存保护配置
type MemoryGuardConfig struct {
	// Enabled 是否启用内存保护（默认 true）
	Enabled bool

	// SoftLimitMB 软限制（MB）
	// 超过此限制时触发 GC
	SoftLimitMB uint64

	// HardLimitMB 硬限制（MB）
	// 超过此限制时强制清理缓存 + GC
	HardLimitMB uint64

	// AutoProfile 是否自动保存 heap profile（当 RSS 超过 HardLimit 时）
	AutoProfile bool

	// ProfileOutputDir heap profile 输出目录
	ProfileOutputDir string

	// CheckInterval 检查间隔
	CheckInterval time.Duration
}

// DefaultMemoryGuardConfig 返回默认配置
func DefaultMemoryGuardConfig() *MemoryGuardConfig {
	return &MemoryGuardConfig{
		Enabled:          true,
		SoftLimitMB:      3072, // 3GB
		HardLimitMB:      4096, // 4GB
		AutoProfile:      true,
		ProfileOutputDir: "data/pprof",
		CheckInterval:    30 * time.Second,
	}
}

// MemoryGuardStats 内存保护统计信息
type MemoryGuardStats struct {
	// 运行状态
	Running   bool      `json:"running"`
	StartTime time.Time `json:"start_time"`
	Uptime    string    `json:"uptime"`

	// 配置信息
	SoftLimitMB uint64 `json:"soft_limit_mb"`
	HardLimitMB uint64 `json:"hard_limit_mb"`

	// 当前状态
	CurrentRSSMB uint64 `json:"current_rss_mb"`
	PressureLevel string `json:"pressure_level"` // none, soft, hard

	// 触发统计
	SoftTriggerCount int       `json:"soft_trigger_count"`
	HardTriggerCount int       `json:"hard_trigger_count"`
	LastSoftTrigger  time.Time `json:"last_soft_trigger,omitempty"`
	LastHardTrigger  time.Time `json:"last_hard_trigger,omitempty"`

	// GC 统计
	GCCount       int `json:"gc_count"`
	CacheCleared  int `json:"cache_cleared"`
	ProfilesSaved int `json:"profiles_saved"`
}

// CacheCleaner 缓存清理器接口
// 各模块可以实现此接口，注册到 MemoryGuard 中
type CacheCleaner interface {
	// Name 返回清理器名称（用于日志）
	Name() string
	// ClearCache 清理缓存，返回释放的估计字节数
	ClearCache() (freedBytes uint64)
}

// Logger 日志接口
type MemoryGuardLogger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// MemoryGuard 内存保护守护程序
type MemoryGuard struct {
	config *MemoryGuardConfig
	logger MemoryGuardLogger

	// 状态
	running   bool
	startTime time.Time
	mu        sync.RWMutex

	// 统计
	stats MemoryGuardStats

	// 子组件
	rssTracker     *RSSTracker
	heapProfiler   *AutoHeapProfiler
	cacheCleaners  []CacheCleaner

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMemoryGuard 创建内存保护守护程序
func NewMemoryGuard(config *MemoryGuardConfig, logger MemoryGuardLogger) *MemoryGuard {
	if config == nil {
		config = DefaultMemoryGuardConfig()
	}

	// 创建 RSS 趋势追踪器
	rssTracker := NewRSSTracker(120, config.SoftLimitMB, config.HardLimitMB)

	// 创建自动 heap profiler
	heapProfiler := NewAutoHeapProfiler(&AutoHeapProfileConfig{
		Enabled:        config.AutoProfile,
		RSSThresholdMB: config.HardLimitMB,
		OutputDir:      config.ProfileOutputDir,
		MaxProfiles:    10,
		MinInterval:    5 * time.Minute,
	})

	return &MemoryGuard{
		config:       config,
		logger:       logger,
		rssTracker:   rssTracker,
		heapProfiler: heapProfiler,
		cacheCleaners: make([]CacheCleaner, 0),
	}
}

// RegisterCacheCleaner 注册缓存清理器
func (g *MemoryGuard) RegisterCacheCleaner(cleaner CacheCleaner) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cacheCleaners = append(g.cacheCleaners, cleaner)
	if g.logger != nil {
		g.logger.Infof("[MemoryGuard] 注册缓存清理器: %s", cleaner.Name())
	}
}

// Start 启动内存保护守护程序
func (g *MemoryGuard) Start(ctx context.Context) error {
	g.mu.Lock()
	if g.running {
		g.mu.Unlock()
		return fmt.Errorf("MemoryGuard 已在运行")
	}

	if !g.config.Enabled {
		g.mu.Unlock()
		if g.logger != nil {
			g.logger.Infof("[MemoryGuard] 已禁用，跳过启动")
		}
		return nil
	}

	g.ctx, g.cancel = context.WithCancel(ctx)
	g.running = true
	g.startTime = time.Now()
	g.stats = MemoryGuardStats{
		Running:     true,
		StartTime:   g.startTime,
		SoftLimitMB: g.config.SoftLimitMB,
		HardLimitMB: g.config.HardLimitMB,
	}
	g.mu.Unlock()

	g.wg.Add(1)
	go g.monitorLoop()

	if g.logger != nil {
		g.logger.Infof("[MemoryGuard] 启动成功 (soft_limit=%dMB, hard_limit=%dMB, interval=%s)",
			g.config.SoftLimitMB, g.config.HardLimitMB, g.config.CheckInterval)
	}

	return nil
}

// Stop 停止内存保护守护程序
func (g *MemoryGuard) Stop() error {
	g.mu.Lock()
	if !g.running {
		g.mu.Unlock()
		return nil
	}
	g.running = false
	g.mu.Unlock()

	if g.cancel != nil {
		g.cancel()
	}
	g.wg.Wait()

	if g.logger != nil {
		g.logger.Infof("[MemoryGuard] 已停止")
	}

	return nil
}

// monitorLoop 监控循环
func (g *MemoryGuard) monitorLoop() {
	defer g.wg.Done()

	ticker := time.NewTicker(g.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			g.checkAndMitigate()
		}
	}
}

// checkAndMitigate 检查内存压力并采取缓解措施
func (g *MemoryGuard) checkAndMitigate() {
	// 获取当前内存统计
	stats := GetMemoryStats()
	rssMB := stats.RSS / 1024 / 1024

	// 添加到趋势追踪器
	g.rssTracker.AddSampleWithStats(stats)

	// 更新当前状态
	g.mu.Lock()
	g.stats.CurrentRSSMB = rssMB
	g.mu.Unlock()

	// 检查压力等级
	if rssMB >= g.config.HardLimitMB {
		g.handleHardPressure(stats)
	} else if rssMB >= g.config.SoftLimitMB {
		g.handleSoftPressure(stats)
	} else {
		g.mu.Lock()
		g.stats.PressureLevel = "none"
		g.mu.Unlock()
	}
}

// handleSoftPressure 处理软限制压力
func (g *MemoryGuard) handleSoftPressure(stats *MemoryStats) {
	g.mu.Lock()
	g.stats.PressureLevel = "soft"
	g.stats.SoftTriggerCount++
	g.stats.LastSoftTrigger = time.Now()
	g.mu.Unlock()

	if g.logger != nil {
		g.logger.Warnf("[MemoryGuard] ⚠️ 软限制触发 (RSS=%dMB > %dMB): 执行 GC",
			stats.RSS/1024/1024, g.config.SoftLimitMB)
	}

	// 执行 GC
	beforeHeap := stats.HeapAlloc
	runtime.GC()
	afterStats := GetMemoryStats()

	g.mu.Lock()
	g.stats.GCCount++
	g.mu.Unlock()

	if g.logger != nil {
		freed := int64(beforeHeap-afterStats.HeapAlloc) / 1024 / 1024
		g.logger.Infof("[MemoryGuard] GC 完成: HeapAlloc %dMB → %dMB (释放 %dMB)",
			beforeHeap/1024/1024, afterStats.HeapAlloc/1024/1024, freed)
	}
}

// handleHardPressure 处理硬限制压力
func (g *MemoryGuard) handleHardPressure(stats *MemoryStats) {
	g.mu.Lock()
	g.stats.PressureLevel = "hard"
	g.stats.HardTriggerCount++
	g.stats.LastHardTrigger = time.Now()
	g.mu.Unlock()

	if g.logger != nil {
		g.logger.Errorf("[MemoryGuard] 🔴 硬限制触发 (RSS=%dMB > %dMB): 执行强力缓解",
			stats.RSS/1024/1024, g.config.HardLimitMB)
	}

	// 1. 自动保存 heap profile（如果启用）
	if g.config.AutoProfile {
		dumped, filepath, err := g.heapProfiler.CheckAndDump()
		if err != nil {
			if g.logger != nil {
				g.logger.Errorf("[MemoryGuard] 保存 heap profile 失败: %v", err)
			}
		} else if dumped {
			g.mu.Lock()
			g.stats.ProfilesSaved++
			g.mu.Unlock()
			if g.logger != nil {
				g.logger.Infof("[MemoryGuard] 📁 已保存 heap profile: %s", filepath)
			}
		}
	}

	// 2. 清理所有注册的缓存
	g.clearAllCaches()

	// 3. 强制 GC + 返还内存给 OS
	beforeHeap := stats.HeapAlloc
	runtime.GC()
	debug.FreeOSMemory()

	g.mu.Lock()
	g.stats.GCCount++
	g.mu.Unlock()

	// 等待 GC 完成
	time.Sleep(100 * time.Millisecond)

	afterStats := GetMemoryStats()
	if g.logger != nil {
		freed := int64(beforeHeap-afterStats.HeapAlloc) / 1024 / 1024
		g.logger.Infof("[MemoryGuard] 强力 GC 完成: HeapAlloc %dMB → %dMB (释放 %dMB), RSS %dMB → %dMB",
			beforeHeap/1024/1024, afterStats.HeapAlloc/1024/1024, freed,
			stats.RSS/1024/1024, afterStats.RSS/1024/1024)
	}
}

// clearAllCaches 清理所有注册的缓存
func (g *MemoryGuard) clearAllCaches() {
	g.mu.RLock()
	cleaners := make([]CacheCleaner, len(g.cacheCleaners))
	copy(cleaners, g.cacheCleaners)
	g.mu.RUnlock()

	if len(cleaners) == 0 {
		return
	}

	if g.logger != nil {
		g.logger.Infof("[MemoryGuard] 开始清理 %d 个缓存...", len(cleaners))
	}

	var totalFreed uint64
	for _, cleaner := range cleaners {
		freed := cleaner.ClearCache()
		totalFreed += freed
		if g.logger != nil && freed > 0 {
			g.logger.Debugf("[MemoryGuard] 清理 %s: 释放 %d MB", cleaner.Name(), freed/1024/1024)
		}
	}

	g.mu.Lock()
	g.stats.CacheCleared++
	g.mu.Unlock()

	if g.logger != nil {
		g.logger.Infof("[MemoryGuard] 缓存清理完成: 估计释放 %d MB", totalFreed/1024/1024)
	}
}

// Stats 返回当前统计信息
func (g *MemoryGuard) Stats() MemoryGuardStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := g.stats
	if g.running {
		stats.Uptime = time.Since(g.startTime).Round(time.Second).String()
	}
	stats.CurrentRSSMB = GetRSSMB()

	return stats
}

// GetRSSTracker 返回 RSS 趋势追踪器
func (g *MemoryGuard) GetRSSTracker() *RSSTracker {
	return g.rssTracker
}

// GenerateReport 生成详细报告
func (g *MemoryGuard) GenerateReport() string {
	stats := g.Stats()
	rssReport := g.rssTracker.AnalyzeGrowth()

	return fmt.Sprintf(`
================================================================================
                       MemoryGuard 状态报告
================================================================================
生成时间: %s

运行状态:
  - 运行中:         %v
  - 启动时间:       %s
  - 运行时长:       %s

配置:
  - 软限制:         %d MB
  - 硬限制:         %d MB
  - 自动Profile:    %v

当前状态:
  - 当前 RSS:       %d MB
  - 压力等级:       %s

触发统计:
  - 软限制触发:     %d 次 (最后: %s)
  - 硬限制触发:     %d 次 (最后: %s)

操作统计:
  - GC 执行次数:    %d
  - 缓存清理次数:   %d
  - Profile 保存:   %d

趋势分析:
  - 健康等级:       %s
  - 状态:           %s
  - 小时增长率:     %.1f MB/h
================================================================================
`,
		time.Now().Format("2006-01-02 15:04:05"),
		stats.Running,
		stats.StartTime.Format("2006-01-02 15:04:05"),
		stats.Uptime,
		stats.SoftLimitMB,
		stats.HardLimitMB,
		g.config.AutoProfile,
		stats.CurrentRSSMB,
		stats.PressureLevel,
		stats.SoftTriggerCount, formatTimeOrNA(stats.LastSoftTrigger),
		stats.HardTriggerCount, formatTimeOrNA(stats.LastHardTrigger),
		stats.GCCount,
		stats.CacheCleared,
		stats.ProfilesSaved,
		rssReport.HealthLevel,
		rssReport.HealthMessage,
		rssReport.RSSGrowthPerHour,
	)
}

// formatTimeOrNA 格式化时间或返回 N/A
func formatTimeOrNA(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("15:04:05")
}

// ============================================================================
//                       便捷函数
// ============================================================================

// CheckMemoryPressure 快速检查内存压力（无需创建 MemoryGuard 实例）
// 返回压力等级: "none", "soft", "hard"
func CheckMemoryPressure(softLimitMB, hardLimitMB uint64) string {
	rssMB := GetRSSMB()
	if rssMB >= hardLimitMB {
		return "hard"
	}
	if rssMB >= softLimitMB {
		return "soft"
	}
	return "none"
}

// MitigateMemoryPressure 快速缓解内存压力（无需创建 MemoryGuard 实例）
// 返回缓解前后的 RSS（MB）
func MitigateMemoryPressure(aggressive bool) (beforeMB, afterMB uint64) {
	beforeMB = GetRSSMB()

	runtime.GC()
	if aggressive {
		debug.FreeOSMemory()
		time.Sleep(100 * time.Millisecond)
	}

	afterMB = GetRSSMB()
	return beforeMB, afterMB
}

