package gc

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// GC 运行次数
	gcRunsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "weisyn",
		Subsystem: "chain_gc",
		Name:      "runs_total",
		Help:      "Total number of block file GC runs",
	})

	// 扫描文件总数
	gcScannedFilesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "weisyn",
		Subsystem: "chain_gc",
		Name:      "scanned_files_total",
		Help:      "Total number of files scanned by block file GC",
	})

	// 删除文件总数
	gcDeletedFilesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "weisyn",
		Subsystem: "chain_gc",
		Name:      "deleted_files_total",
		Help:      "Total number of files deleted by block file GC",
	})

	// 回收字节总数
	gcReclaimedBytesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "weisyn",
		Subsystem: "chain_gc",
		Name:      "reclaimed_bytes_total",
		Help:      "Total bytes reclaimed by block file GC",
	})

	// GC 运行耗时
	gcDurationSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "weisyn",
		Subsystem: "chain_gc",
		Name:      "duration_seconds",
		Help:      "Duration of block file GC runs in seconds",
		Buckets:   prometheus.DefBuckets, // [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
	})

	// 当前运行状态
	gcRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "weisyn",
		Subsystem: "chain_gc",
		Name:      "running",
		Help:      "Whether block file GC is currently running (1 = running, 0 = idle)",
	})

	// 不可达文件数（最近一次 GC）
	gcUnreachableFilesLast = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "weisyn",
		Subsystem: "chain_gc",
		Name:      "unreachable_files_last",
		Help:      "Number of unreachable files found in the last GC run",
	})

	// GC 错误次数
	gcErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "weisyn",
		Subsystem: "chain_gc",
		Name:      "errors_total",
		Help:      "Total number of errors encountered during block file GC",
	})
)

// updateMetrics 更新 Prometheus 指标
func (gc *BlockFileGC) updateMetrics(result *GCRunResult, duration float64, err error) {
	// 增加运行次数
	gcRunsTotal.Inc()

	if err != nil {
		// 记录错误
		gcErrorsTotal.Inc()
		return
	}

	// 更新计数器
	gcScannedFilesTotal.Add(float64(result.ScannedFiles))
	gcDeletedFilesTotal.Add(float64(result.DeletedFiles))
	gcReclaimedBytesTotal.Add(float64(result.ReclaimedBytes))

	// 更新耗时
	gcDurationSeconds.Observe(duration)

	// 更新不可达文件数
	gcUnreachableFilesLast.Set(float64(result.UnreachableFiles))
}

// setRunningStatus 设置运行状态
func (gc *BlockFileGC) setRunningStatus(running bool) {
	if running {
		gcRunning.Set(1)
	} else {
		gcRunning.Set(0)
	}
}

