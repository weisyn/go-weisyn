// Package metrics 内存监控组件
//
// MemoryDoctor 负责周期性采样内存状态，并提供 HTTP 接口查询
package metrics

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	metricsiface "github.com/weisyn/v1/pkg/interfaces/infrastructure/metrics"
	"github.com/weisyn/v1/pkg/utils"
	runtimeutil "github.com/weisyn/v1/pkg/utils/runtime"
	metricsutil "github.com/weisyn/v1/pkg/utils/metrics"
)

// MemoryMonitoringMode 内存监控模式
type MemoryMonitoringMode string

const (
	// MemoryMonitoringModeMinimal 最小模式：只统计 Objects/CacheItems/QueueLength，ApproxBytes 一律为 0
	MemoryMonitoringModeMinimal MemoryMonitoringMode = "minimal"

	// MemoryMonitoringModeHeuristic 启发式模式：对能获取真实统计的模块计算 ApproxBytes（如 block/eutxo 的 proto.Size，mempool 的 calculateTransactionSize），其他为 0
	MemoryMonitoringModeHeuristic MemoryMonitoringMode = "heuristic"

	// MemoryMonitoringModeAccurate 精确模式：所有模块尽可能计算 ApproxBytes（包括基于配置参数的估算，如 WebSocket 缓冲区）
	MemoryMonitoringModeAccurate MemoryMonitoringMode = "accurate"
)

// MemoryDoctorConfig MemoryDoctor 配置
type MemoryDoctorConfig struct {
	// SampleInterval 采样间隔（例如 10s）
	SampleInterval time.Duration

	// WindowSize 保留最近 N 次样本用于趋势判定（例如 30）
	WindowSize int

	// HeapGrowthSoftLimitBytes 某窗口内允许的最大增长（bytes）
	HeapGrowthSoftLimitBytes int64

	// Mode 内存监控模式：minimal / heuristic / accurate
	// - minimal: 只统计对象数，ApproxBytes 一律为 0（适合 dev 环境，减少开销）
	// - heuristic: 对能获取真实统计的模块计算 ApproxBytes（如 proto.Size），其他为 0（默认，适合大多数场景）
	// - accurate: 所有模块尽可能计算 ApproxBytes（包括基于配置的估算，适合 prod 环境）
	Mode MemoryMonitoringMode

	// GoroutineWarnThreshold Goroutine 数量告警阈值（默认 5000）
	// 超过此阈值触发 WARN 级别告警
	GoroutineWarnThreshold int

	// GoroutineCriticalThreshold Goroutine 数量严重告警阈值（默认 10000）
	// 超过此阈值触发 ERROR 级别告警
	GoroutineCriticalThreshold int

	// GoroutineGrowthRateThreshold Goroutine 增长速率告警阈值（每分钟增长数，默认 500）
	// 如果窗口内每分钟增长超过此值，触发增长速率告警
	GoroutineGrowthRateThreshold int
}

// DefaultMemoryDoctorConfig 返回默认配置
func DefaultMemoryDoctorConfig() MemoryDoctorConfig {
	return MemoryDoctorConfig{
		SampleInterval:               10 * time.Second,
		WindowSize:                   30,
		HeapGrowthSoftLimitBytes:     100 * 1024 * 1024,             // 100MB
		Mode:                         MemoryMonitoringModeHeuristic, // 默认启发式模式
		GoroutineWarnThreshold:       5000,                          // 超过 5000 个 Goroutine 触发 WARN
		GoroutineCriticalThreshold:   10000,                         // 超过 10000 个 Goroutine 触发 ERROR
		GoroutineGrowthRateThreshold: 500,                           // 每分钟增长超过 500 个触发告警
	}
}

// HeapSample 堆内存采样数据
//
// ⚠️ 重要说明（2025-12-18 更新）：
//
// HeapAlloc / HeapSys 等指标包含了 mmap 区域的虚拟地址空间统计（如 BadgerDB value log mmap），
// 可能导致这些值虚高（例如 100GB+），但实际物理内存（RSS）正常（例如 2GB）。
//
// 因此：
// - **判断内存压力应该使用 RSS（物理内存），而非 HeapAlloc（虚拟内存）**
// - HeapAlloc 仅作为诊断参考，不应作为告警依据
//
// 典型场景：
// - BadgerDB 使用 mmap 将 value log 文件（可达 GB 级）映射到虚拟地址空间
// - Go runtime.MemStats.HeapAlloc 统计包含了这部分虚拟地址
// - 但物理内存（RSS）只在实际访问时才分配（按需分页）
// - 所以会出现 "HeapAlloc=100GB, RSS=2GB" 的正常现象
type HeapSample struct {
	Time         time.Time                        `json:"time"`
	HeapAlloc    uint64                           `json:"heap_alloc"`    // 当前堆分配（bytes）- ⚠️ 包含 mmap 虚拟地址，可能虚高
	HeapInuse    uint64                           `json:"heap_inuse"`    // 当前堆使用（bytes）- ⚠️ 包含 mmap 虚拟地址，可能虚高
	HeapSys      uint64                           `json:"heap_sys"`      // Go 堆保留虚拟内存（bytes）- ⚠️ 包含 mmap，可能虚高
	StackInuse   uint64                           `json:"stack_inuse"`   // goroutine 栈占用（bytes）
	MSpanInuse   uint64                           `json:"mspan_inuse"`   // mspan 元数据占用（bytes）
	MCacheInuse  uint64                           `json:"mcache_inuse"`  // mcache 元数据占用（bytes）
	Sys          uint64                           `json:"sys"`           // Go runtime 申请的总虚拟内存（bytes）- ⚠️ 包含 mmap
	RSSBytes     uint64                           `json:"rss_bytes"`     // 进程真实物理内存（RSS，bytes）- ✅ 判断内存压力的主要指标
	RSSMB        uint64                           `json:"rss_mb"`        // 进程真实物理内存（RSS，MB）- ✅ 判断内存压力的主要指标
	NumGC        uint32                           `json:"num_gc"`        // GC 次数
	NumGoroutine int                              `json:"num_goroutine"` // Goroutine 数量
	OpenFDs      int                              `json:"open_fds"`      // 当前打开的文件描述符数量（估算）
	FDLimit      uint64                           `json:"fd_limit"`      // 进程文件描述符软上限
	Modules      []metricsiface.ModuleMemoryStats `json:"modules"`       // 各模块内存统计
}

// MemoryDoctor 内存监控组件
//
// 职责：
// - 周期性采样内存状态（runtime.MemStats + 各模块统计）
// - 保留历史样本用于趋势分析
// - 提供当前内存状态查询接口
type MemoryDoctor struct {
	cfg     MemoryDoctorConfig
	logger  *zap.Logger
	history []HeapSample
	mu      sync.RWMutex

	// 限频动作
	lastHeapDumpAt  time.Time
	lastFreeOSAt    time.Time
	lastVlogCheckAt time.Time // 🆕 2025-12-18: BadgerDB vlog 大小检查限频
}

// GetMode 返回当前内存监控模式
func (d *MemoryDoctor) GetMode() MemoryMonitoringMode {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.cfg.Mode == "" {
		return MemoryMonitoringModeHeuristic // 默认值
	}
	return d.cfg.Mode
}

// NewMemoryDoctor 创建新的 MemoryDoctor 实例
func NewMemoryDoctor(cfg MemoryDoctorConfig, logger *zap.Logger) *MemoryDoctor {
	if cfg.SampleInterval == 0 {
		cfg.SampleInterval = 10 * time.Second
	}
	if cfg.WindowSize == 0 {
		cfg.WindowSize = 30
	}
	if cfg.HeapGrowthSoftLimitBytes == 0 {
		cfg.HeapGrowthSoftLimitBytes = 100 * 1024 * 1024 // 100MB
	}
	if cfg.GoroutineWarnThreshold == 0 {
		cfg.GoroutineWarnThreshold = 5000
	}
	if cfg.GoroutineCriticalThreshold == 0 {
		cfg.GoroutineCriticalThreshold = 10000
	}
	if cfg.GoroutineGrowthRateThreshold == 0 {
		cfg.GoroutineGrowthRateThreshold = 500
	}

	return &MemoryDoctor{
		cfg:     cfg,
		logger:  logger,
		history: make([]HeapSample, 0, cfg.WindowSize),
	}
}

// getRSSBytes 获取进程真实物理内存（RSS）
//
// 返回：
//   - uint64: RSS 字节数
//   - 如果获取失败，返回 0
//
// 说明：
//   - macOS: 使用 syscall.Getrusage 获取 ru_maxrss（单位：字节）
//     ⚠️ 注意：ru_maxrss 返回的是峰值 RSS（进程运行期间的最大值），不是当前 RSS
//     这意味着即使内存已释放，Maxrss 也不会减少，只会增加
//     因此日志中的 RSS 值可能高于 ps aux 显示的当前 RSS
//   - Linux: 读取 /proc/self/status 获取 VmRSS（单位：KB，当前RSS）
//   - 其他平台：返回 0
func getRSSBytes() uint64 {
	switch runtime.GOOS {
	case "darwin":
		// macOS: 使用 syscall.Getrusage
		// 注意：macOS 的 ru_maxrss 单位是字节，返回的是峰值 RSS（不是当前RSS）
		var rusage syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err != nil {
			return 0
		}
		// macOS 上 ru_maxrss 单位是字节，返回峰值 RSS
		return uint64(rusage.Maxrss)
	case "linux":
		// Linux: 读取 /proc/self/status
		return getRSSBytesFromProc()
	default:
		// 其他平台暂不支持
		return 0
	}
}

// getRSSBytesFromProc 从 /proc/self/status 读取 RSS（Linux）
func getRSSBytesFromProc() uint64 {
	file, err := os.Open("/proc/self/status")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			// 格式：VmRSS:    12345 kB
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return 0
				}
				return kb * 1024 // 转换为字节
			}
		}
	}

	return 0
}

// getOpenFDInfo 获取当前进程打开的 FD 数量及软上限
func getOpenFDInfo() (count int, limit uint64) {
	// 获取 rlimit
	var rl syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rl); err == nil {
		limit = rl.Cur
	}

	// 统计 /proc/self/fd 或 /dev/fd 下的条目数
	// 在 Linux 上优先使用 /proc/self/fd，macOS 上使用 /dev/fd
	dirs := []string{"/proc/self/fd", "/dev/fd"}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err == nil {
			// 去掉 "." / ".." 等特殊项（ReadDir 一般不会返回这两项）
			return len(entries), limit
		}
	}

	return 0, limit
}

// Start 启动 MemoryDoctor 的采样循环
//
// 参数：
//   - ctx: 上下文，用于控制生命周期
//
// 说明：
//   - 在独立的 goroutine 中运行
//   - 当 ctx.Done() 时自动停止
func (d *MemoryDoctor) Start(ctx context.Context) {
	ticker := time.NewTicker(d.cfg.SampleInterval)
	defer ticker.Stop()

	if d.logger != nil {
		d.logger.Info("MemoryDoctor 启动",
			zap.Duration("sample_interval", d.cfg.SampleInterval),
			zap.Int("window_size", d.cfg.WindowSize))
	}

	for {
		select {
		case <-ctx.Done():
			if d.logger != nil {
				d.logger.Info("MemoryDoctor 停止")
			}
			return
		case <-ticker.C:
			d.SampleOnce()
		}
	}
}

// SampleOnce 执行一次内存采样（公开方法，供外部调用）
//
// 🎯 **使用场景**：
// - 启动时立即采样，无需等待SampleInterval
// - 健康检查或手动触发采样
func (d *MemoryDoctor) SampleOnce() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// 收集所有模块的内存统计
	modStats := metricsutil.CollectAllModuleStats()

	d.mu.Lock()

	// 获取进程真实物理内存（RSS）
	rssBytes := getRSSBytes()
	rssMB := rssBytes / 1024 / 1024

	// 获取 FD 使用情况
	openFDs, fdLimit := getOpenFDInfo()

	s := HeapSample{
		Time:         time.Now(),
		HeapAlloc:    ms.HeapAlloc,
		HeapInuse:    ms.HeapInuse,
		HeapSys:      ms.HeapSys,
		StackInuse:   ms.StackInuse,
		MSpanInuse:   ms.MSpanInuse,
		MCacheInuse:  ms.MCacheInuse,
		Sys:          ms.Sys,
		RSSBytes:     rssBytes,
		RSSMB:        rssMB,
		NumGC:        ms.NumGC,
		NumGoroutine: runtime.NumGoroutine(),
		OpenFDs:      openFDs,
		FDLimit:      fdLimit,
		Modules:      modStats,
	}

	d.history = append(d.history, s)

	// 保持窗口大小
	if len(d.history) > d.cfg.WindowSize {
		d.history = d.history[len(d.history)-d.cfg.WindowSize:]
	}

	// 检测异常趋势（用于驱动主动自救）
	bad := d.detectBadTrendLocked()

	d.mu.Unlock()

	// 输出统一的结构化日志（便于后续分析和监控）
	// 格式：memory_sample，包含所有关键内存指标
	if d.logger != nil {
		d.logger.Info("memory_sample",
			zap.Time("time", s.Time),
			zap.Uint64("rss_mb", s.RSSMB),
			zap.Uint64("rss_bytes", s.RSSBytes),
			zap.Uint64("heap_mb", s.HeapAlloc/1024/1024),
			zap.Uint64("heap_alloc_bytes", s.HeapAlloc),
			zap.Uint64("heap_inuse_bytes", s.HeapInuse),
			zap.Uint64("heap_sys_bytes", s.HeapSys),
			zap.Uint64("stack_inuse_bytes", s.StackInuse),
			zap.Uint64("mspan_inuse_bytes", s.MSpanInuse),
			zap.Uint64("mcache_inuse_bytes", s.MCacheInuse),
			zap.Uint64("sys_bytes", s.Sys),
			zap.Uint32("gc", s.NumGC),
			zap.Int("goroutines", s.NumGoroutine),
			zap.Int("modules_count", len(s.Modules)),
			zap.Any("modules", s.Modules),
		)
	}

	if bad != nil && d.logger != nil {
		// 获取 top 3 模块的内存占用（用于诊断）
		topModules := d.getTopModulesForLog(s.Modules, 3)

		d.logger.Warn("内存趋势警告",
			zap.String("reason", bad.Reason),
			zap.Uint64("rss_mb", s.RSSMB),
			zap.Uint64("heap_alloc", bad.HeapAlloc),
			zap.Int64("growth_bytes", bad.GrowthBytes),
			zap.Int("num_goroutine", s.NumGoroutine),
			zap.Int("open_fds", s.OpenFDs),
			zap.Any("top_modules", topModules))
	}

	// 🆕 Goroutine 数量告警检查（P0 紧急修复：Goroutine 泄漏排查）
	goroutineAlert := d.checkGoroutineCount(s.NumGoroutine)
	if goroutineAlert != nil && d.logger != nil {
		if goroutineAlert.Level == "critical" {
			d.logger.Error("goroutine_count_critical",
				zap.Int("count", goroutineAlert.Count),
				zap.Int("threshold", goroutineAlert.Threshold),
				zap.String("action", "立即排查 Goroutine 泄漏，访问 /api/v1/system/diagnostics/pprof/goroutine?debug=2 获取堆栈"),
			)
		} else if goroutineAlert.GrowthRate > 0 {
			d.logger.Warn("goroutine_growth_rate_high",
				zap.Int("count", goroutineAlert.Count),
				zap.Float64("growth_rate_per_min", goroutineAlert.GrowthRate),
				zap.Int("growth_threshold", goroutineAlert.GrowthThreshold),
				zap.String("action", "Goroutine 数量快速增长，可能存在泄漏"),
			)
		} else {
			d.logger.Warn("goroutine_count_high",
				zap.Int("count", goroutineAlert.Count),
				zap.Int("threshold", goroutineAlert.Threshold),
				zap.String("action", "Goroutine 数量偏高，建议排查是否有泄漏"),
			)
		}
	}

	// 将运行时快照同步给 IOGuard，用于综合判断压力等级
	metricsutil.RecordRuntimeSnapshot(
		int(s.NumGoroutine),
		s.RSSBytes,
		s.OpenFDs,
		s.FDLimit,
	)

	// 根据内存与 IO 压力，尝试触发各模块的缓存收缩
	d.applyCacheShrink(s, bad != nil)

	// ✅ 高压自动诊断：当 RSS 接近 cgroup 上限时，限频落盘 heap profile，并尝试释放 OS 内存
	d.maybeDumpHeapAndFreeOS(s)

	// 🆕 2025-12-18：监控 BadgerDB vlog 文件大小（mmap 虚拟地址占用来源）
	d.checkBadgerVlogSize()
}

func (d *MemoryDoctor) maybeDumpHeapAndFreeOS(s HeapSample) {
	limit, ok, err := runtimeutil.GetCgroupMemoryLimitBytes()
	if err != nil || !ok || limit == 0 {
		return
	}
	rss := s.RSSBytes
	if rss == 0 {
		return
	}
	// 触发阈值：85% 先 dump，90% 再 FreeOSMemory
	dumpThresh := uint64(float64(limit) * 0.85)
	freeThresh := uint64(float64(limit) * 0.90)
	now := time.Now()

	if rss >= dumpThresh {
		// dump 限频：10分钟一次
		if d.lastHeapDumpAt.IsZero() || now.Sub(d.lastHeapDumpAt) >= 10*time.Minute {
			if path, dumpErr := d.dumpHeapProfileLocked(now); dumpErr != nil {
				if d.logger != nil {
					d.logger.Warn("heap_profile_dump_failed", zap.Error(dumpErr))
				}
			} else if d.logger != nil {
				d.logger.Warn("heap_profile_dumped",
					zap.String("path", path),
					zap.Uint64("rss_mb", s.RSSMB),
					zap.Uint64("cgroup_limit_mb", limit/1024/1024),
				)
			}
			d.lastHeapDumpAt = now
		}
	}

	if rss >= freeThresh {
		// free 限频：2分钟一次
		if d.lastFreeOSAt.IsZero() || now.Sub(d.lastFreeOSAt) >= 2*time.Minute {
			debug.FreeOSMemory()
			d.lastFreeOSAt = now
			if d.logger != nil {
				d.logger.Warn("free_os_memory_triggered",
					zap.Uint64("rss_mb", s.RSSMB),
					zap.Uint64("cgroup_limit_mb", limit/1024/1024),
				)
			}
		}
	}
}

func (d *MemoryDoctor) dumpHeapProfileLocked(now time.Time) (string, error) {
	// 统一落盘到 data/pprof（容器内通常会挂载 data volume）
	dir := utils.ResolveDataPath("./data/pprof")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	filename := now.Format("20060102-150405") + "-heap.pprof"
	path := dir + string(os.PathSeparator) + filename

	// GC 一次，降低噪声（避免把短命对象也算进去）
	runtime.GC()

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return "", err
	}
	_ = f.Sync()
	return path, nil
}

// BadTrend 异常趋势信息
type BadTrend struct {
	Reason      string // 异常原因
	HeapAlloc   uint64 // 当前堆分配
	GrowthBytes int64  // 增长字节数
}

// GoroutineAlert Goroutine 告警信息
type GoroutineAlert struct {
	Level           string  // "warn" 或 "critical"
	Count           int     // 当前 Goroutine 数量
	Threshold       int     // 触发的阈值
	GrowthRate      float64 // 每分钟增长速率（如果有）
	GrowthThreshold int     // 增长速率阈值
}

// detectBadTrendLocked 检测异常趋势（需要在持有锁的情况下调用）
//
// 检测规则：
//   - 🆕 2025-12-18 修复：基于 RSS（物理内存）而非 heap_alloc（虚拟内存）
//   - 原因：BadgerDB 使用 mmap 导致 heap_alloc 虚高（~100GB），但实际物理内存正常
//   - 如果窗口内 RSS 增长超过 HeapGrowthSoftLimitBytes（沿用旧配置名，实际检测 RSS）
//   - 如果某个模块的 ApproxBytes / Objects 在窗口内涨幅超过阈值
//
// 返回：
//   - *BadTrend: 如果检测到异常趋势，返回详细信息；否则返回 nil
func (d *MemoryDoctor) detectBadTrendLocked() *BadTrend {
	if len(d.history) < 2 {
		return nil
	}

	first := d.history[0]
	last := d.history[len(d.history)-1]

	// 🆕 修复：检测 RSS（物理内存）增长，而非 heap_alloc（虚拟内存）
	//
	// 原因：BadgerDB 使用 mmap 将 value log 文件映射到虚拟地址空间，
	// 导致 heap_alloc 虚高（可达 100GB+），但实际物理内存（RSS）正常。
	// Go 的 runtime.MemStats.HeapAlloc 包含了 mmap 区域的虚拟地址统计，
	// 因此不应该用 heap_alloc 判断内存压力，应该用 RSS。
	rssGrowth := int64(last.RSSBytes) - int64(first.RSSBytes)
	if rssGrowth > d.cfg.HeapGrowthSoftLimitBytes {
		// RSS 增长超过阈值（100MB），认为异常
		return &BadTrend{
			Reason:      "物理内存(RSS)增长超过阈值",
			HeapAlloc:   last.HeapAlloc,  // 保留 HeapAlloc 用于诊断参考
			GrowthBytes: rssGrowth,        // 实际是 RSS 增长量
		}
	}

	return nil
}

// checkGoroutineCount 检查 Goroutine 数量并生成告警
//
// 检测规则：
//   - 超过 GoroutineCriticalThreshold（默认 10000）触发 critical 告警
//   - 超过 GoroutineWarnThreshold（默认 5000）触发 warn 告警
//   - 窗口内每分钟增长超过 GoroutineGrowthRateThreshold（默认 500）触发增长速率告警
//
// 返回：
//   - *GoroutineAlert: 如果检测到异常，返回告警信息；否则返回 nil
func (d *MemoryDoctor) checkGoroutineCount(count int) *GoroutineAlert {
	// 检查绝对数量阈值
	if count >= d.cfg.GoroutineCriticalThreshold {
		return &GoroutineAlert{
			Level:     "critical",
			Count:     count,
			Threshold: d.cfg.GoroutineCriticalThreshold,
		}
	}

	if count >= d.cfg.GoroutineWarnThreshold {
		return &GoroutineAlert{
			Level:     "warn",
			Count:     count,
			Threshold: d.cfg.GoroutineWarnThreshold,
		}
	}

	// 检查增长速率（需要至少 2 个样本）
	d.mu.RLock()
	historyLen := len(d.history)
	var growthRate float64
	if historyLen >= 2 {
		first := d.history[0]
		last := d.history[historyLen-1]
		duration := last.Time.Sub(first.Time)
		if duration > 0 {
			goroutineDiff := last.NumGoroutine - first.NumGoroutine
			// 计算每分钟增长速率
			growthRate = float64(goroutineDiff) / duration.Minutes()
		}
	}
	d.mu.RUnlock()

	// 如果增长速率超过阈值，即使绝对数量未超标也告警
	if growthRate > float64(d.cfg.GoroutineGrowthRateThreshold) {
		return &GoroutineAlert{
			Level:           "warn",
			Count:           count,
			Threshold:       d.cfg.GoroutineWarnThreshold,
			GrowthRate:      growthRate,
			GrowthThreshold: d.cfg.GoroutineGrowthRateThreshold,
		}
	}

	return nil
}

// applyCacheShrink 根据当前样本和趋势，尝试触发各模块的缓存收缩
func (d *MemoryDoctor) applyCacheShrink(s HeapSample, hasBadTrend bool) {
	if len(s.Modules) == 0 {
		return
	}

	level := metricsutil.GetIOPressureLevel()

	// 将模块统计转为 map，便于按名称查找
	statsByModule := make(map[string]metricsiface.ModuleMemoryStats, len(s.Modules))
	for _, m := range s.Modules {
		statsByModule[m.Module] = m
	}

	metricsutil.ForEachReporter(func(r metricsiface.MemoryReporter) {
		name := r.ModuleName()
		stat, ok := statsByModule[name]
		if !ok || stat.CacheItems <= 0 {
			return
		}

		// 只关注缓存条目较多的模块
		if stat.CacheItems < 100 {
			return
		}

		shrinker, ok := r.(interface{ ShrinkCache(targetSize int) })
		if !ok {
			return
		}

		var factor float64 = 1.0

		// 根据压力等级与趋势决定缩减比例
		switch level {
		case metricsutil.IOPressureCritical:
			// Critical：更激进，直接减半
			factor = 0.5
		case metricsutil.IOPressureWarning:
			// Warning：温和缩减
			factor = 0.8
		default:
			// IO 正常但内存趋势异常时，做一次轻量缩减
			if hasBadTrend {
				factor = 0.9
			} else {
				// 无明显压力，不动
				return
			}
		}

		target := int(float64(stat.CacheItems) * factor)
		if target <= 0 {
			target = 1
		}

		shrinker.ShrinkCache(target)
	})
}

// GetCurrentStats 获取当前内存状态（用于 HTTP 接口）
//
// 返回：
//   - HeapSample: 最新的内存采样数据
func (d *MemoryDoctor) GetCurrentStats() HeapSample {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.history) == 0 {
		// 如果没有历史数据，立即采样一次
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		modStats := metricsutil.CollectAllModuleStats()

		// 获取进程真实物理内存（RSS）
		rssBytes := getRSSBytes()
		rssMB := rssBytes / 1024 / 1024

		return HeapSample{
			Time:         time.Now(),
			HeapAlloc:    ms.HeapAlloc,
			HeapInuse:    ms.HeapInuse,
			RSSBytes:     rssBytes,
			RSSMB:        rssMB,
			NumGC:        ms.NumGC,
			NumGoroutine: runtime.NumGoroutine(),
			Modules:      modStats,
		}
	}

	return d.history[len(d.history)-1]
}

// GetHistory 获取历史采样数据（用于趋势分析）
//
// 返回：
//   - []HeapSample: 历史采样数据切片（按时间顺序）
func (d *MemoryDoctor) GetHistory() []HeapSample {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 返回副本，避免外部修改
	result := make([]HeapSample, len(d.history))
	copy(result, d.history)
	return result
}

// getTopModulesForLog 获取 Top N 模块用于日志输出
func (d *MemoryDoctor) getTopModulesForLog(modules []metricsiface.ModuleMemoryStats, topN int) []map[string]interface{} {
	// 按 approx_bytes 降序排序
	sorted := make([]metricsiface.ModuleMemoryStats, len(modules))
	copy(sorted, modules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ApproxBytes > sorted[j].ApproxBytes
	})

	// 取前 topN 个
	result := make([]map[string]interface{}, 0)
	for i := 0; i < topN && i < len(sorted); i++ {
		result = append(result, map[string]interface{}{
			"module":       sorted[i].Module,
			"approx_bytes": sorted[i].ApproxBytes,
			"objects":      sorted[i].Objects,
		})
	}

	return result
}

// StartMemoryOptimization 启动定期内存优化循环
//
// 🆕 P2 修复：定期强制释放 RSS 内存给操作系统
//
// 功能：
// - 每 10 分钟执行一次 GC 和 debug.FreeOSMemory()
// - 强制释放 Go runtime 持有但不再使用的内存给操作系统
// - 解决 RSS 内存持续增长但 GC 后不释放的问题
//
// 参数：
//   - ctx: 上下文，用于控制生命周期
//
// 说明：
//   - 在独立的 goroutine 中运行
//   - 当 ctx.Done() 时自动停止
func (d *MemoryDoctor) StartMemoryOptimization(ctx context.Context) {
	// 优化间隔：10 分钟
	const optimizationInterval = 10 * time.Minute

	ticker := time.NewTicker(optimizationInterval)
	defer ticker.Stop()

	if d.logger != nil {
		d.logger.Info("MemoryDoctor 内存优化循环启动",
			zap.Duration("interval", optimizationInterval))
	}

	for {
		select {
		case <-ctx.Done():
			if d.logger != nil {
				d.logger.Info("MemoryDoctor 内存优化循环停止")
			}
			return
		case <-ticker.C:
			d.optimizeMemory()
		}
	}
}

// optimizeMemory 执行一次内存优化
func (d *MemoryDoctor) optimizeMemory() {
	// 获取优化前的 RSS
	beforeRSS := getRSSBytes()
	beforeRSSMB := beforeRSS / 1024 / 1024

	// 获取优化前的 heap
	var beforeMS runtime.MemStats
	runtime.ReadMemStats(&beforeMS)

	// 1. 执行 GC
	runtime.GC()

	// 2. 强制释放内存给操作系统
	debug.FreeOSMemory()

	// 获取优化后的指标
	afterRSS := getRSSBytes()
	afterRSSMB := afterRSS / 1024 / 1024

	var afterMS runtime.MemStats
	runtime.ReadMemStats(&afterMS)

	// 计算释放量
	freedRSS := int64(0)
	if beforeRSS > afterRSS {
		freedRSS = int64(beforeRSS - afterRSS)
	}
	freedHeap := int64(0)
	if beforeMS.HeapAlloc > afterMS.HeapAlloc {
		freedHeap = int64(beforeMS.HeapAlloc - afterMS.HeapAlloc)
	}

	// 记录日志
	if d.logger != nil {
		d.logger.Info("memory_optimization_done",
			zap.Uint64("before_rss_mb", beforeRSSMB),
			zap.Uint64("after_rss_mb", afterRSSMB),
			zap.Int64("freed_rss_mb", freedRSS/1024/1024),
			zap.Uint64("before_heap_mb", beforeMS.HeapAlloc/1024/1024),
			zap.Uint64("after_heap_mb", afterMS.HeapAlloc/1024/1024),
			zap.Int64("freed_heap_mb", freedHeap/1024/1024),
			zap.Int("goroutines", runtime.NumGoroutine()),
		)
	}
}

// checkBadgerVlogSize 检查 BadgerDB vlog 文件总大小并告警
//
// 🆕 2025-12-18：监控 BadgerDB vlog 文件大小（mmap 虚拟地址占用来源）
//
// 问题：BadgerDB 使用 mmap 将 value log 文件映射到虚拟地址空间，
// 导致 runtime.MemStats.HeapAlloc 虚高。vlog 文件过大会占用过多虚拟地址空间。
//
// 告警规则：
// - vlog 总大小 > 10GB: ERROR 级别
// - vlog 总大小 > 5GB: WARN 级别
// - 限频：每 10 分钟最多告警一次
func (d *MemoryDoctor) checkBadgerVlogSize() {
	// 限频检查：每 10 分钟最多检查一次
	now := time.Now()
	if !d.lastVlogCheckAt.IsZero() && now.Sub(d.lastVlogCheckAt) < 10*time.Minute {
		return
	}
	d.lastVlogCheckAt = now

	// 获取 BadgerDB 数据目录
	// 通常在 data/<instance>/badger/ 或 data/badger/
	dataDir := utils.ResolveDataPath("./data")
	
	// 搜索所有可能的 badger 目录
	badgerDirs := []string{
		dataDir + "/badger",
		dataDir + "/test/test-public-WES_public_testnet_demo_2024/badger",
		// 可以根据实际情况添加更多路径
	}

	for _, badgerDir := range badgerDirs {
		totalSize, vlogCount, err := d.getBadgerVlogSize(badgerDir)
		if err != nil {
			continue // 目录不存在或无法访问，跳过
		}

		totalSizeMB := totalSize / 1024 / 1024
		totalSizeGB := totalSize / 1024 / 1024 / 1024

		if d.logger != nil {
			if totalSizeGB > 10 {
				// vlog > 10GB，严重告警
				d.logger.Error("badger_vlog_size_critical",
					zap.String("dir", badgerDir),
					zap.Uint64("total_size_gb", totalSizeGB),
					zap.Int("vlog_count", vlogCount),
					zap.String("action", "BadgerDB vlog 文件过大，可能导致虚拟地址空间占用过高，建议手动压缩或清理旧数据"),
				)
			} else if totalSizeGB > 5 {
				// vlog > 5GB，警告
				d.logger.Warn("badger_vlog_size_high",
					zap.String("dir", badgerDir),
					zap.Uint64("total_size_mb", totalSizeMB),
					zap.Int("vlog_count", vlogCount),
					zap.String("action", "BadgerDB vlog 文件偏大，建议关注"),
				)
			} else {
				// vlog <= 5GB，正常，仅 DEBUG 记录
				d.logger.Debug("badger_vlog_size_normal",
					zap.String("dir", badgerDir),
					zap.Uint64("total_size_mb", totalSizeMB),
					zap.Int("vlog_count", vlogCount),
				)
			}
		}
	}
}

// getBadgerVlogSize 获取指定目录下所有 *.vlog 文件的总大小
func (d *MemoryDoctor) getBadgerVlogSize(dir string) (totalSize uint64, count int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// 检查是否是 vlog 文件（如 000002.vlog, 000003.vlog）
		if !strings.HasSuffix(entry.Name(), ".vlog") {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		totalSize += uint64(info.Size())
		count++
	}

	return totalSize, count, nil
}
