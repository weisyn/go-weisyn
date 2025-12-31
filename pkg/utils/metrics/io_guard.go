package metrics

import (
	"sync"
	"time"
)

// IOGuardState 维护磁盘读（FileStore.Load）及基础运行时指标的压力状态
//
// 实现思路：
// - 每次 FileStore.Load 调用时更新一次 QPS 和平均耗时的 EMA（指数滑动平均）
// - 当 EMA QPS 或 EMA 耗时超过阈值时，进入高压状态一段时间（cooldown）
// - 其他模块可以通过 IsIOHighPressure() 查询当前是否处于高压状态

type ioGuardState struct {
	mu sync.Mutex

	lastEventTime time.Time
	emaQPS        float64
	emaLatencySec float64

	// 运行时资源指标（最近一次采样）
	goroutines int
	rssBytes   uint64
	openFDs    int
	fdLimit    uint64

	// 当前压力等级
	level IOPressureLevel

	// 高压 TTL（在 Warning/Critical 下保持一段时间）
	highPressureTTL time.Time

	// 启动时间：用于在启动初期降低 QPS 计算权重，避免误判
	startTime time.Time

	// 🆕 2025-12-18：连续正常计数（用于减速豁免机制）
	// 当连续 N 次检查都为 Normal 时，可以获得一次减速豁免
	consecutiveNormalCount int
}

// IOPressureLevel 表示 IO / 资源压力等级
type IOPressureLevel int

const (
	IOPressureNormal IOPressureLevel = iota
	IOPressureWarning
	IOPressureCritical
)

var (
	defaultAlpha = 0.2 // EMA 平滑因子

	// 默认配置（可通过 SetIOGuardConfig 覆盖）
	//
	// 🆕 2025-12-18 优化：
	// - HighPressureTTL 从 30s 降到 10s，更快恢复
	// - 阈值调整：QPS Warning=200, Critical=400（适应更高吞吐）
	// - Goroutine 阈值上调：Warning=5000, Critical=10000（适应 libp2p 节点）
	defaultIOConfig = IOGuardConfig{
		QPSWarning:         200.0,  // 原 150 -> 200
		QPSCritical:        400.0,  // 原 300 -> 400
		LatWarningSec:      0.05,   // 原 30ms -> 50ms
		LatCriticalSec:     0.1,    // 原 80ms -> 100ms
		HighPressureTTL:    10 * time.Second, // 原 30s -> 10s
		GoroutinesWarning:  5000,   // 原 4000 -> 5000
		GoroutinesCritical: 10000,  // 原 8000 -> 10000
		FDUsageWarning:     0.7,
		FDUsageCritical:    0.9,
	}

	// 当前生效配置（初始为 defaultIOConfig）
	currentIOConfig = defaultIOConfig

	globalIOGuard = &ioGuardState{
		startTime: time.Now(), // 记录启动时间
	} // 全局单例
)

// IOGuardConfig 定义 IOGuard 的动态阈值配置
type IOGuardConfig struct {
	QPSWarning      float64
	QPSCritical     float64
	LatWarningSec   float64
	LatCriticalSec  float64
	HighPressureTTL time.Duration

	GoroutinesWarning  int
	GoroutinesCritical int

	FDUsageWarning  float64
	FDUsageCritical float64
}

// SetIOGuardConfig 覆盖默认 IO 阈值配置（例如从链配置加载）
func SetIOGuardConfig(cfg IOGuardConfig) {
	// 简单防御性：填补空值
	if cfg.QPSWarning <= 0 {
		cfg.QPSWarning = defaultIOConfig.QPSWarning
	}
	if cfg.QPSCritical <= 0 {
		cfg.QPSCritical = defaultIOConfig.QPSCritical
	}
	if cfg.LatWarningSec <= 0 {
		cfg.LatWarningSec = defaultIOConfig.LatWarningSec
	}
	if cfg.LatCriticalSec <= 0 {
		cfg.LatCriticalSec = defaultIOConfig.LatCriticalSec
	}
	if cfg.HighPressureTTL <= 0 {
		cfg.HighPressureTTL = defaultIOConfig.HighPressureTTL
	}
	if cfg.GoroutinesWarning <= 0 {
		cfg.GoroutinesWarning = defaultIOConfig.GoroutinesWarning
	}
	if cfg.GoroutinesCritical <= 0 {
		cfg.GoroutinesCritical = defaultIOConfig.GoroutinesCritical
	}
	if cfg.FDUsageWarning <= 0 {
		cfg.FDUsageWarning = defaultIOConfig.FDUsageWarning
	}
	if cfg.FDUsageCritical <= 0 {
		cfg.FDUsageCritical = defaultIOConfig.FDUsageCritical
	}

	currentIOConfig = cfg
}

// RecordFileLoad 在 FileStore.Load 调用结束时上报一次 IO 事件
//
// 参数：
// - duration: 本次 Load 调用耗时
// - hadError: 本次是否发生错误（当前策略对错误不做单独判断，但为未来扩展预留）
func RecordFileLoad(duration time.Duration, hadError bool) {
	globalIOGuard.record(duration)
}

// RecordRuntimeSnapshot 由 MemoryDoctor 调用，记录一次运行时资源快照
func RecordRuntimeSnapshot(goroutines int, rssBytes uint64, openFDs int, fdLimit uint64) {
	globalIOGuard.recordRuntimeSnapshot(goroutines, rssBytes, openFDs, fdLimit)
}

// GetIOPressureLevel 返回当前 IO / 资源压力等级
func GetIOPressureLevel() IOPressureLevel {
	return globalIOGuard.getLevel()
}

// IsIOHighPressure 返回当前是否处于 IO 高压状态（Warning 或 Critical）
func IsIOHighPressure() bool {
	level := globalIOGuard.getLevel()
	return level == IOPressureWarning || level == IOPressureCritical
}

// IOPressureDiagnostic 包含 IO 压力的诊断信息
type IOPressureDiagnostic struct {
	Level       IOPressureLevel
	EMAQPS      float64
	EMALatency  float64 // 秒
	Goroutines  int
	OpenFDs     int
	FDLimit     uint64
	FDUsage     float64
	Triggers    []string // 触发高压的具体原因
}

// GetIOPressureDiagnostic 返回当前 IO 压力的详细诊断信息
//
// 🆕 2025-12-18：用于在挖矿减速时输出具体原因，便于问题定位
func GetIOPressureDiagnostic() IOPressureDiagnostic {
	return globalIOGuard.getDiagnostic()
}

func (g *ioGuardState) getDiagnostic() IOPressureDiagnostic {
	g.mu.Lock()
	defer g.mu.Unlock()

	diag := IOPressureDiagnostic{
		Level:      g.level,
		EMAQPS:     g.emaQPS,
		EMALatency: g.emaLatencySec,
		Goroutines: g.goroutines,
		OpenFDs:    g.openFDs,
		FDLimit:    g.fdLimit,
		Triggers:   make([]string, 0, 4),
	}

	// 计算 FD 使用率
	if g.fdLimit > 0 && g.openFDs > 0 {
		diag.FDUsage = float64(g.openFDs) / float64(g.fdLimit)
	}

	// 确定触发原因
	cfg := currentIOConfig
	if g.emaQPS > cfg.QPSWarning {
		if g.emaQPS > cfg.QPSCritical {
			diag.Triggers = append(diag.Triggers, "QPS_CRITICAL")
		} else {
			diag.Triggers = append(diag.Triggers, "QPS_WARNING")
		}
	}
	if g.emaLatencySec > cfg.LatWarningSec {
		if g.emaLatencySec > cfg.LatCriticalSec {
			diag.Triggers = append(diag.Triggers, "LATENCY_CRITICAL")
		} else {
			diag.Triggers = append(diag.Triggers, "LATENCY_WARNING")
		}
	}
	if g.goroutines > cfg.GoroutinesWarning {
		if g.goroutines > cfg.GoroutinesCritical {
			diag.Triggers = append(diag.Triggers, "GOROUTINE_CRITICAL")
		} else {
			diag.Triggers = append(diag.Triggers, "GOROUTINE_WARNING")
		}
	}
	if diag.FDUsage > cfg.FDUsageWarning {
		if diag.FDUsage > cfg.FDUsageCritical {
			diag.Triggers = append(diag.Triggers, "FD_CRITICAL")
		} else {
			diag.Triggers = append(diag.Triggers, "FD_WARNING")
		}
	}

	return diag
}

// GetRecommendedSlowdownDuration 根据当前压力等级返回建议的减速时间
//
// 🆕 2025-12-18：实现渐进式减速
// - Normal: 0（不减速）
// - Warning: 500ms
// - Critical: 2s
func GetRecommendedSlowdownDuration() time.Duration {
	level := globalIOGuard.getLevel()
	switch level {
	case IOPressureWarning:
		return 500 * time.Millisecond
	case IOPressureCritical:
		return 2 * time.Second
	default:
		return 0
	}
}

// ShouldSlowdown 检查是否应该减速，并返回建议的减速时间
//
// 🆕 2025-12-18：实现连续正常后的减速豁免机制
//
// 策略：
// - 如果连续 3 次检查都为 Normal，可以获得一次 Warning 级别的减速豁免
// - Critical 级别不可豁免
// - 每次豁免后重置计数器
//
// 返回：
// - shouldSlowdown: 是否应该减速
// - duration: 建议的减速时间
// - reason: 减速原因（用于日志）
func ShouldSlowdown() (shouldSlowdown bool, duration time.Duration, reason string) {
	return globalIOGuard.shouldSlowdown()
}

const consecutiveNormalThreshold = 3 // 连续正常 3 次后可以豁免一次 Warning

func (g *ioGuardState) shouldSlowdown() (shouldSlowdown bool, duration time.Duration, reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	level := g.level

	// 检查 TTL 是否过期
	if !g.highPressureTTL.IsZero() && time.Now().Before(g.highPressureTTL) {
		// TTL 未过期，使用当前等级
	} else {
		// TTL 过期，重新评估
		g.updateLevelLocked(time.Now())
		level = g.level
	}

	switch level {
	case IOPressureNormal:
		// 正常状态：累计连续正常计数
		g.consecutiveNormalCount++
		return false, 0, ""

	case IOPressureWarning:
		// Warning 级别：检查是否有豁免资格
		if g.consecutiveNormalCount >= consecutiveNormalThreshold {
			// 消耗豁免资格
			g.consecutiveNormalCount = 0
			return false, 0, "exempt_by_consecutive_normal"
		}
		// 无豁免资格，需要减速
		g.consecutiveNormalCount = 0
		return true, 500 * time.Millisecond, "io_pressure_warning"

	case IOPressureCritical:
		// Critical 级别：不可豁免
		g.consecutiveNormalCount = 0
		return true, 2 * time.Second, "io_pressure_critical"

	default:
		return false, 0, ""
	}
}

// --- 内部实现 ---

func (g *ioGuardState) record(duration time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	// 计算瞬时 QPS（基于两次调用间隔的近似值）
	var instQPS float64
	if !g.lastEventTime.IsZero() {
		delta := now.Sub(g.lastEventTime).Seconds()
		if delta > 0 {
			instQPS = 1.0 / delta
		}
	}

	// ⚠️ **启动初期保护**：
	// - 节点启动后前 30 秒内，降低 QPS 计算权重，避免启动初期连续快速调用导致误判
	// - 使用更小的 alpha 值（0.05 vs 0.2），让 EMA 更平滑
	startupGracePeriod := 30 * time.Second
	alpha := defaultAlpha
	if time.Since(g.startTime) < startupGracePeriod {
		alpha = 0.05 // 启动初期使用更小的平滑因子
	}

	// 更新 EMA QPS
	if instQPS > 0 {
		g.emaQPS = alpha*instQPS + (1-alpha)*g.emaQPS
	}

	// 更新 EMA 耗时
	lat := duration.Seconds()
	if lat > 0 {
		g.emaLatencySec = alpha*lat + (1-alpha)*g.emaLatencySec
	}

	g.lastEventTime = now

	g.updateLevelLocked(now)
}

// recordRuntimeSnapshot 更新运行时资源统计（由 MemoryDoctor 调用）
func (g *ioGuardState) recordRuntimeSnapshot(goroutines int, rssBytes uint64, openFDs int, fdLimit uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.goroutines = goroutines
	g.rssBytes = rssBytes
	g.openFDs = openFDs
	g.fdLimit = fdLimit

	g.updateLevelLocked(time.Now())
}

// getLevel 返回当前压力等级（考虑 TTL）
func (g *ioGuardState) getLevel() IOPressureLevel {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 如果 TTL 还没过期，直接返回当前等级
	if !g.highPressureTTL.IsZero() && time.Now().Before(g.highPressureTTL) {
		return g.level
	}

	// 否则根据当前指标重新评估
	g.updateLevelLocked(time.Now())
	return g.level
}

// updateLevelLocked 在持有锁的情况下，根据 EMA + 运行时指标更新压力等级
func (g *ioGuardState) updateLevelLocked(now time.Time) {
	level := IOPressureNormal

	// 1. 基于 QPS / 延迟的压力
	if g.emaQPS > currentIOConfig.QPSWarning || g.emaLatencySec > currentIOConfig.LatWarningSec {
		level = IOPressureWarning
	}
	if g.emaQPS > currentIOConfig.QPSCritical || g.emaLatencySec > currentIOConfig.LatCriticalSec {
		level = IOPressureCritical
	}

	// 2. 基于 Goroutine 数的压力
	if g.goroutines > currentIOConfig.GoroutinesWarning {
		if level < IOPressureWarning {
			level = IOPressureWarning
		}
	}
	if g.goroutines > currentIOConfig.GoroutinesCritical {
		level = IOPressureCritical
	}

	// 3. 基于 FD 使用率的压力
	if g.fdLimit > 0 && g.openFDs > 0 {
		usage := float64(g.openFDs) / float64(g.fdLimit)
		if usage > currentIOConfig.FDUsageWarning && level < IOPressureWarning {
			level = IOPressureWarning
		}
		if usage > currentIOConfig.FDUsageCritical {
			level = IOPressureCritical
		}
	}

	g.level = level

	// 如果进入 Warning 或 Critical，则更新 TTL
	if level == IOPressureWarning || level == IOPressureCritical {
		g.highPressureTTL = now.Add(currentIOConfig.HighPressureTTL)
	} else {
		// 正常状态清空 TTL
		g.highPressureTTL = time.Time{}
	}
}


