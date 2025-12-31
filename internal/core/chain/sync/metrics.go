package sync

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/weisyn/v1/pkg/types"
)

// 同步系统 Prometheus 指标
//
// 设计原则：
// - 仅暴露少量高价值指标，避免噪音；
// - 不在热路径做复杂计算，更新开销尽量常数级；
// - 使用默认 Registry，方便通过 /metrics 统一抓取。

var (
	syncMetricsOnce sync.Once

	syncLocalHeightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "sync",
		Name:      "local_height",
		Help:      "Current local blockchain height observed by SystemSyncService.",
	})

	syncNetworkHeightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "sync",
		Name:      "network_height",
		Help:      "Current estimated network height observed by SystemSyncService.",
	})

	syncGapGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "sync",
		Name:      "height_gap",
		Help:      "Gap between network_height and local_height (network - local, floored at 0).",
	})

	syncLastSuccessGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "sync",
		Name:      "last_success_unix",
		Help:      "Unix timestamp of last successful sync check (as seen by SystemSyncService).",
	})

	syncStatusGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "wes",
			Subsystem: "sync",
			Name:      "status",
			Help:      "Current sync status (1 for the active status label, 0 for others).",
		},
		[]string{"status"},
	)

	// P3-005: 同步速率指标（blocks per second）
	syncBlocksPerSecondGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "wes",
		Subsystem: "sync",
		Name:      "blocks_per_second",
		Help:      "Current sync speed in blocks per second (smoothed EMA).",
	})

	// 用于计算同步速率的内部状态
	syncSpeedMu          sync.Mutex
	syncSpeedLastHeight  uint64
	syncSpeedLastTime    time.Time
	syncSpeedEMA         float64 // 指数移动平均
	syncSpeedEMAAlpha    = 0.3   // EMA 平滑因子
)

// initSyncMetrics 在首次使用时注册同步指标。
func initSyncMetrics() {
	syncMetricsOnce.Do(func() {
		prometheus.MustRegister(
			syncLocalHeightGauge,
			syncNetworkHeightGauge,
			syncGapGauge,
			syncLastSuccessGauge,
			syncStatusGauge,
			syncBlocksPerSecondGauge,
		)
	})
}

// observeSyncMetrics 根据当前 SystemSyncStatus 更新 Prometheus 指标。
//
// 注意：
// - 该函数应只在同步状态查询的慢路径调用，不放在热写入路径；
// - 调用方应保证 status 非 nil。
func observeSyncMetrics(status *types.SystemSyncStatus) {
	if status == nil {
		return
	}

	initSyncMetrics()

	syncLocalHeightGauge.Set(float64(status.CurrentHeight))
	syncNetworkHeightGauge.Set(float64(status.NetworkHeight))

	var gap float64
	if status.NetworkHeight > status.CurrentHeight {
		gap = float64(status.NetworkHeight - status.CurrentHeight)
	}
	syncGapGauge.Set(gap)

	// LastSyncTime 是 RFC3339Time，底层为 time.Time 的别名结构体
	var ts time.Time
	if t := time.Time(status.LastSyncTime); !t.IsZero() {
		ts = t
	} else {
		ts = time.Now()
	}
	syncLastSuccessGauge.Set(float64(ts.Unix()))

	// 将当前 status 标记为 1，其它已知状态标记为 0
	for _, s := range []types.SystemSyncStatusType{
		types.SyncStatusIdle,
		types.SyncStatusSyncing,
		types.SyncStatusSynced,
		types.SyncStatusError,
		types.SyncStatusBootstrapping,
		types.SyncStatusDegraded,
	} {
		value := 0.0
		if s == status.Status {
			value = 1.0
		}
		syncStatusGauge.WithLabelValues(s.String()).Set(value)
	}

	// P3-005: 更新同步速率
	updateSyncSpeed(status.CurrentHeight)
}

// updateSyncSpeed 更新同步速率指标（使用 EMA 平滑）
//
// 计算逻辑：
// 1. 比较当前高度与上次记录的高度差
// 2. 除以时间差得到瞬时速率
// 3. 使用 EMA 平滑避免抖动
func updateSyncSpeed(currentHeight uint64) {
	initSyncMetrics()

	syncSpeedMu.Lock()
	defer syncSpeedMu.Unlock()

	now := time.Now()

	// 首次调用，初始化基准
	if syncSpeedLastTime.IsZero() {
		syncSpeedLastHeight = currentHeight
		syncSpeedLastTime = now
		return
	}

	elapsed := now.Sub(syncSpeedLastTime).Seconds()
	if elapsed < 1.0 {
		// 间隔太短，跳过本次计算
		return
	}

	// 计算瞬时速率
	var instantSpeed float64
	if currentHeight > syncSpeedLastHeight {
		instantSpeed = float64(currentHeight-syncSpeedLastHeight) / elapsed
	}

	// EMA 平滑
	if syncSpeedEMA == 0 {
		syncSpeedEMA = instantSpeed
	} else {
		syncSpeedEMA = syncSpeedEMAAlpha*instantSpeed + (1-syncSpeedEMAAlpha)*syncSpeedEMA
	}

	// 更新指标
	syncBlocksPerSecondGauge.Set(syncSpeedEMA)

	// 更新基准
	syncSpeedLastHeight = currentHeight
	syncSpeedLastTime = now
}

// GetSyncSpeed 获取当前同步速率（供外部查询）
func GetSyncSpeed() float64 {
	syncSpeedMu.Lock()
	defer syncSpeedMu.Unlock()
	return syncSpeedEMA
}
